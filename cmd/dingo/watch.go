package main

import (
	"bufio"
	"context"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/MadAppGang/dingo/pkg/config"
	"github.com/MadAppGang/dingo/pkg/shadow"
	"github.com/MadAppGang/dingo/pkg/transpiler"
	"github.com/MadAppGang/dingo/pkg/ui"
	"github.com/MadAppGang/dingo/pkg/ui/mascot"
	"github.com/MadAppGang/dingo/pkg/version"
	"github.com/charmbracelet/lipgloss"
	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"
)

// transpileForWatch transpiles a .dingo file and returns the Go code
func transpileForWatch(src []byte, filename string) ([]byte, error) {
	result, err := transpiler.PureASTTranspileWithMappings(src, filename, true)
	if err != nil {
		return nil, err
	}
	return result.GoCode, nil
}

// watchDebounce is the debounce duration for file changes
const watchDebounce = 500 * time.Millisecond

// gracefulShutdownTimeout is how long to wait for interrupt before kill
const gracefulShutdownTimeout = 2 * time.Second

// ProcessManager handles the running child process (cross-platform)
type ProcessManager struct {
	cmd      *exec.Cmd
	cancel   context.CancelFunc
	mu       sync.Mutex
	running  bool
	doneCh   chan struct{}
	exitCh   chan error  // Sends exit error (nil = clean exit, non-nil = crash)
	exitErr  error       // Last exit error
	exitMsg  string      // Last error message from stderr
	stopping bool        // True if we intentionally stopped the process
	stderr   *stderrCapture // Captures stderr for error messages
}

// stderrCapture captures stderr while also writing to os.Stderr
type stderrCapture struct {
	buf   []byte
	mu    sync.Mutex
	limit int // Max bytes to keep
}

func newStderrCapture(limit int) *stderrCapture {
	return &stderrCapture{limit: limit}
}

func (s *stderrCapture) Write(p []byte) (n int, err error) {
	// Write to actual stderr
	n, err = os.Stderr.Write(p)

	// Capture last N bytes
	s.mu.Lock()
	s.buf = append(s.buf, p...)
	if len(s.buf) > s.limit {
		s.buf = s.buf[len(s.buf)-s.limit:]
	}
	s.mu.Unlock()

	return n, err
}

// CrashInfo contains parsed information about a crash
type CrashInfo struct {
	Message  string // e.g., "panic: called MustOk on an Err value"
	File     string // e.g., "main.go"
	Line     int    // e.g., 274
	FullPath string // Full path to the file
}

func (s *stderrCapture) ParseCrash() *CrashInfo {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.buf) == 0 {
		return nil
	}

	lines := strings.Split(string(s.buf), "\n")
	info := &CrashInfo{}

	// Look for panic message and location
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Capture panic message
		if strings.HasPrefix(trimmed, "panic:") {
			info.Message = trimmed
			continue
		}
		if strings.HasPrefix(trimmed, "fatal error:") {
			info.Message = trimmed
			continue
		}

		// Look for file:line location in stack trace (lines starting with whitespace containing .go:)
		if info.Message != "" && info.FullPath == "" && strings.HasPrefix(line, "\t") {
			// Skip internal dgo package frames, find user code
			if strings.Contains(trimmed, ".go:") && !strings.Contains(trimmed, "/dgo/") {
				// Extract full path and line number
				// Format: /path/to/file.go:123 +0x...
				pathEnd := strings.Index(trimmed, " ")
				if pathEnd < 0 {
					pathEnd = len(trimmed)
				}
				fullLoc := trimmed[:pathEnd]

				// Parse path and line number
				if colonIdx := strings.LastIndex(fullLoc, ":"); colonIdx > 0 {
					info.FullPath = fullLoc[:colonIdx]
					if lineNum, err := strconv.Atoi(fullLoc[colonIdx+1:]); err == nil {
						info.Line = lineNum
					}
					// Extract just filename
					if slashIdx := strings.LastIndex(info.FullPath, "/"); slashIdx >= 0 {
						info.File = info.FullPath[slashIdx+1:]
					} else {
						info.File = info.FullPath
					}
				}
			}
		}

		// Also check for regular error messages
		if info.Message == "" {
			if strings.HasPrefix(trimmed, "Error:") || strings.HasPrefix(trimmed, "error:") {
				info.Message = trimmed
			}
		}

		// Stop after finding both
		if info.Message != "" && info.FullPath != "" {
			break
		}

		// Don't look too far
		if i > 20 {
			break
		}
	}

	if info.Message == "" {
		return nil
	}
	return info
}

func (s *stderrCapture) LastError() string {
	info := s.ParseCrash()
	if info == nil {
		return ""
	}
	if info.File != "" && info.Line > 0 {
		return fmt.Sprintf("%s at %s:%d", info.Message, info.File, info.Line)
	}
	return info.Message
}

// GetSourceContext reads source lines around a crash location
func GetSourceContext(filePath string, line int, contextLines int) string {
	if filePath == "" || line <= 0 {
		return ""
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return ""
	}

	lines := strings.Split(string(content), "\n")
	if line > len(lines) {
		return ""
	}

	// Calculate range
	startLine := line - contextLines
	if startLine < 1 {
		startLine = 1
	}
	endLine := line + contextLines
	if endLine > len(lines) {
		endLine = len(lines)
	}

	var result strings.Builder
	for i := startLine; i <= endLine; i++ {
		lineContent := lines[i-1]
		// Truncate long lines
		if len(lineContent) > 60 {
			lineContent = lineContent[:57] + "..."
		}
		// Mark the crash line
		if i == line {
			result.WriteString(fmt.Sprintf("  → %4d │ %s\n", i, lineContent))
		} else {
			result.WriteString(fmt.Sprintf("    %4d │ %s\n", i, lineContent))
		}
	}

	return result.String()
}

func (s *stderrCapture) Reset() {
	s.mu.Lock()
	s.buf = nil
	s.mu.Unlock()
}

// Start starts the process with the given command and arguments
func (pm *ProcessManager) Start(goFiles []string, programArgs []string) error {
	return pm.StartFromDir("", goFiles, programArgs)
}

// StartFromDir starts the process from a specific directory
func (pm *ProcessManager) StartFromDir(dir string, goFiles []string, programArgs []string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Create cancellable context for this process
	ctx, cancel := context.WithCancel(context.Background())
	pm.cancel = cancel
	pm.doneCh = make(chan struct{})

	// Convert absolute paths to relative paths if running from a specific directory
	var relFiles []string
	if dir != "" {
		for _, goFile := range goFiles {
			relPath, err := filepath.Rel(dir, goFile)
			if err == nil {
				relFiles = append(relFiles, relPath)
			} else {
				relFiles = append(relFiles, goFile)
			}
		}
	} else {
		relFiles = goFiles
	}

	args := []string{"run"}
	args = append(args, relFiles...)
	if len(programArgs) > 0 {
		args = append(args, programArgs...)
	}

	pm.cmd = exec.CommandContext(ctx, "go", args...)
	if dir != "" {
		pm.cmd.Dir = dir
	}
	pm.cmd.Stdout = os.Stdout
	pm.cmd.Stderr = os.Stderr
	pm.cmd.Stdin = os.Stdin

	if err := pm.cmd.Start(); err != nil {
		cancel()
		return fmt.Errorf("failed to start process: %w", err)
	}

	pm.running = true

	// Monitor the process in the background
	go func() {
		_ = pm.cmd.Wait()
		pm.mu.Lock()
		pm.running = false
		pm.mu.Unlock()
		close(pm.doneCh)
	}()

	return nil
}

// Stop stops the running process with graceful shutdown
func (pm *ProcessManager) Stop() {
	pm.mu.Lock()
	if !pm.running || pm.cmd == nil || pm.cmd.Process == nil {
		pm.mu.Unlock()
		return
	}
	pm.stopping = true // Mark as intentional stop
	doneCh := pm.doneCh
	cancel := pm.cancel
	process := pm.cmd.Process
	pm.mu.Unlock()

	// First try to interrupt the process gracefully
	// On Unix this sends SIGINT, on Windows it calls TerminateProcess
	_ = interruptProcess(process)

	// Wait for graceful shutdown or timeout
	select {
	case <-doneCh:
		// Process exited gracefully
		return
	case <-time.After(gracefulShutdownTimeout):
		// Force kill via context cancellation
		if cancel != nil {
			cancel()
		}
		// Also try direct kill as fallback
		_ = process.Kill()
	}

	// Wait for the process to actually exit (with a short timeout)
	select {
	case <-doneCh:
		return
	case <-time.After(500 * time.Millisecond):
		// Process didn't exit in time, continue anyway
		return
	}
}

// StartPackage starts the process using "go run ." from a package directory
// This allows Go to properly resolve local module imports
func (pm *ProcessManager) StartPackage(pkgDir string, programArgs []string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Create cancellable context for this process
	ctx, cancel := context.WithCancel(context.Background())
	pm.cancel = cancel
	pm.doneCh = make(chan struct{})
	pm.exitCh = make(chan error, 1)
	pm.exitErr = nil
	pm.exitMsg = ""
	pm.stopping = false
	pm.stderr = newStderrCapture(8192) // Capture last 8KB of stderr

	// Build args: go run . [programArgs...]
	args := []string{"run", "."}
	if len(programArgs) > 0 {
		args = append(args, programArgs...)
	}

	pm.cmd = exec.CommandContext(ctx, "go", args...)
	pm.cmd.Dir = pkgDir
	pm.cmd.Stdout = os.Stdout
	pm.cmd.Stderr = pm.stderr // Capture stderr while still displaying
	pm.cmd.Stdin = os.Stdin

	if err := pm.cmd.Start(); err != nil {
		cancel()
		return fmt.Errorf("failed to start process: %w", err)
	}

	pm.running = true

	// Monitor the process in the background
	go func() {
		err := pm.cmd.Wait()
		pm.mu.Lock()
		pm.running = false
		pm.exitErr = err
		if err != nil && pm.stderr != nil {
			pm.exitMsg = pm.stderr.LastError()
		}
		stopping := pm.stopping
		exitCh := pm.exitCh
		pm.mu.Unlock()
		close(pm.doneCh)

		// Only notify if we didn't intentionally stop and there was an error
		if !stopping && err != nil && exitCh != nil {
			select {
			case exitCh <- err:
			default:
			}
		}
	}()

	return nil
}

// LastErrorMessage returns the last error message captured from stderr
func (pm *ProcessManager) LastErrorMessage() string {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	return pm.exitMsg
}

// ParseCrash returns parsed crash information from stderr
func (pm *ProcessManager) ParseCrash() *CrashInfo {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if pm.stderr == nil {
		return nil
	}
	return pm.stderr.ParseCrash()
}

// IsRunning returns true if the process is currently running
func (pm *ProcessManager) IsRunning() bool {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	return pm.running
}

// ExitCh returns a channel that receives the exit error when the process crashes
func (pm *ProcessManager) ExitCh() <-chan error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	return pm.exitCh
}

// DependencyTracker manages file dependencies for watch mode
type DependencyTracker struct {
	workspaceRoot string
	mainFiles     []string // User-specified .dingo files
	modulePath    string   // Module path from go.mod

	// Computed dependencies
	allDingoFiles map[string]bool // All .dingo files to watch
	allGoFiles    map[string]bool // All .go files to watch
	allDirs       map[string]bool // All directories containing watched files

	mu sync.RWMutex
}

// NewDependencyTracker creates a dependency tracker
func NewDependencyTracker(workspaceRoot string, mainFiles []string) *DependencyTracker {
	return &DependencyTracker{
		workspaceRoot: workspaceRoot,
		mainFiles:     mainFiles,
		allDingoFiles: make(map[string]bool),
		allGoFiles:    make(map[string]bool),
		allDirs:       make(map[string]bool),
	}
}

// readModulePath reads the module path from go.mod
func (dt *DependencyTracker) readModulePath() error {
	goModPath := filepath.Join(dt.workspaceRoot, "go.mod")
	file, err := os.Open(goModPath)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	moduleRegex := regexp.MustCompile(`^module\s+(.+)$`)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if matches := moduleRegex.FindStringSubmatch(line); len(matches) == 2 {
			dt.modulePath = matches[1]
			return nil
		}
	}
	return fmt.Errorf("module path not found in go.mod")
}

// extractImportsFromDingo extracts import paths from a .dingo file
func (dt *DependencyTracker) extractImportsFromDingo(filePath string) ([]string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var imports []string
	lines := strings.Split(string(content), "\n")
	inImportBlock := false

	// Match single import: import "path" or import ( block
	singleImportRegex := regexp.MustCompile(`^\s*import\s+"([^"]+)"`)
	importPathRegex := regexp.MustCompile(`^\s*"([^"]+)"`)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check for import block start
		if strings.HasPrefix(trimmed, "import (") {
			inImportBlock = true
			continue
		}

		// Check for import block end
		if inImportBlock && trimmed == ")" {
			inImportBlock = false
			continue
		}

		// Inside import block
		if inImportBlock {
			if matches := importPathRegex.FindStringSubmatch(line); len(matches) == 2 {
				imports = append(imports, matches[1])
			}
			continue
		}

		// Single import statement
		if matches := singleImportRegex.FindStringSubmatch(line); len(matches) == 2 {
			imports = append(imports, matches[1])
		}
	}

	return imports, nil
}

// extractImportsFromGo extracts import paths from a .go file using go/parser
func (dt *DependencyTracker) extractImportsFromGo(filePath string) ([]string, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, parser.ImportsOnly)
	if err != nil {
		return nil, err
	}

	var imports []string
	for _, imp := range node.Imports {
		// Remove quotes from import path
		path := strings.Trim(imp.Path.Value, `"`)
		imports = append(imports, path)
	}
	return imports, nil
}

// isLocalImport checks if an import path is local to this module
func (dt *DependencyTracker) isLocalImport(importPath string) bool {
	return strings.HasPrefix(importPath, dt.modulePath)
}

// resolveImportToDir converts a local import path to an absolute directory
func (dt *DependencyTracker) resolveImportToDir(importPath string) string {
	// Strip module path prefix to get relative path
	relPath := strings.TrimPrefix(importPath, dt.modulePath)
	relPath = strings.TrimPrefix(relPath, "/")
	return filepath.Join(dt.workspaceRoot, relPath)
}

// discoverDir scans a directory for .dingo and .go files and their imports
func (dt *DependencyTracker) discoverDir(dir string, visited map[string]bool) error {
	if visited[dir] {
		return nil
	}
	visited[dir] = true

	// Check if directory exists
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil
	}

	dt.allDirs[dir] = true

	// Find all .dingo files
	dingoFiles, _ := filepath.Glob(filepath.Join(dir, "*.dingo"))
	for _, f := range dingoFiles {
		absPath, _ := filepath.Abs(f)
		dt.allDingoFiles[absPath] = true

		// Extract imports from this file
		imports, err := dt.extractImportsFromDingo(absPath)
		if err != nil {
			continue
		}

		// Recursively discover local imports
		for _, imp := range imports {
			if dt.isLocalImport(imp) {
				impDir := dt.resolveImportToDir(imp)
				if err := dt.discoverDir(impDir, visited); err != nil {
					continue
				}
			}
		}
	}

	// Find all .go files
	goFiles, _ := filepath.Glob(filepath.Join(dir, "*.go"))
	for _, f := range goFiles {
		absPath, _ := filepath.Abs(f)
		// Skip test files
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		dt.allGoFiles[absPath] = true

		// Extract imports from this file
		imports, err := dt.extractImportsFromGo(absPath)
		if err != nil {
			continue
		}

		// Recursively discover local imports
		for _, imp := range imports {
			if dt.isLocalImport(imp) {
				impDir := dt.resolveImportToDir(imp)
				if err := dt.discoverDir(impDir, visited); err != nil {
					continue
				}
			}
		}
	}

	return nil
}

// Discover scans import statements and builds dependency set
func (dt *DependencyTracker) Discover() error {
	dt.mu.Lock()
	defer dt.mu.Unlock()

	// Reset maps
	dt.allDingoFiles = make(map[string]bool)
	dt.allGoFiles = make(map[string]bool)
	dt.allDirs = make(map[string]bool)

	// Read module path from go.mod
	if err := dt.readModulePath(); err != nil {
		// If no go.mod, fall back to simple directory watching
		dt.modulePath = ""
	}

	// Track visited directories to avoid cycles
	visited := make(map[string]bool)

	// Start discovery from directories containing main files
	for _, mainFile := range dt.mainFiles {
		absPath, err := filepath.Abs(mainFile)
		if err != nil {
			return fmt.Errorf("failed to get absolute path for %s: %w", mainFile, err)
		}

		dir := filepath.Dir(absPath)
		if err := dt.discoverDir(dir, visited); err != nil {
			return err
		}
	}

	return nil
}

// IsWatched checks if a file path should trigger rebuild
func (dt *DependencyTracker) IsWatched(path string) bool {
	dt.mu.RLock()
	defer dt.mu.RUnlock()

	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	return dt.allDingoFiles[absPath] || dt.allGoFiles[absPath]
}

// RefreshDependencies re-scans dependencies (after successful build)
func (dt *DependencyTracker) RefreshDependencies() error {
	return dt.Discover()
}

// Count returns total number of watched files
func (dt *DependencyTracker) Count() int {
	dt.mu.RLock()
	defer dt.mu.RUnlock()
	return len(dt.allDingoFiles) + len(dt.allGoFiles)
}

// DirCount returns number of directories being watched
func (dt *DependencyTracker) DirCount() int {
	dt.mu.RLock()
	defer dt.mu.RUnlock()
	return len(dt.allDirs)
}

// Summary returns a human-readable summary of watched files
func (dt *DependencyTracker) Summary() string {
	dt.mu.RLock()
	defer dt.mu.RUnlock()
	return fmt.Sprintf("%d files in %d dirs", len(dt.allDingoFiles)+len(dt.allGoFiles), len(dt.allDirs))
}

// GetWatchPaths returns all paths to watch
func (dt *DependencyTracker) GetWatchPaths() []string {
	dt.mu.RLock()
	defer dt.mu.RUnlock()

	paths := make([]string, 0, dt.Count())
	for path := range dt.allDingoFiles {
		paths = append(paths, path)
	}
	for path := range dt.allGoFiles {
		paths = append(paths, path)
	}
	return paths
}

// GetDingoFiles returns all .dingo files to transpile
func (dt *DependencyTracker) GetDingoFiles() []string {
	dt.mu.RLock()
	defer dt.mu.RUnlock()

	files := make([]string, 0, len(dt.allDingoFiles))
	for path := range dt.allDingoFiles {
		files = append(files, path)
	}
	return files
}

// WatchRunner manages the watch loop
type WatchRunner struct {
	dingoFiles     []string
	programArgs    []string
	cfg            *config.Config
	pm             *ProcessManager
	watcher        *fsnotify.Watcher
	styles         watchStyles
	generatedFiles map[string]bool // Set of generated .go files to ignore
	mu             sync.Mutex      // Protects generatedFiles and rebuild operations
	workspaceRoot  string          // Path to go.mod
	shadowBuilder  *shadow.Builder // Shadow build manager

	// NEW: UI and dependency tracking
	ui         *ui.WatchUI
	depTracker *DependencyTracker
	noMascot   bool // Disable UI
}

type watchStyles struct {
	timestamp lipgloss.Style
	event     lipgloss.Style
	file      lipgloss.Style
	success   lipgloss.Style
	error     lipgloss.Style
}

func newWatchStyles() watchStyles {
	return watchStyles{
		timestamp: lipgloss.NewStyle().Foreground(lipgloss.Color("#6C7086")),
		event:     lipgloss.NewStyle().Foreground(lipgloss.Color("#7D56F4")).Bold(true),
		file:      lipgloss.NewStyle().Foreground(lipgloss.Color("#CDD6F4")),
		success:   lipgloss.NewStyle().Foreground(lipgloss.Color("#5AF78E")),
		error:     lipgloss.NewStyle().Foreground(lipgloss.Color("#F38BA8")),
	}
}

func (wr *WatchRunner) timestamp() string {
	return wr.styles.timestamp.Render(fmt.Sprintf("[%s]", time.Now().Format("15:04:05")))
}

func (wr *WatchRunner) log(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("%s %s\n", wr.timestamp(), msg)
}

func watchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watch [flags] [packages/files] [-- program args]",
		Short: "Watch for changes and auto-rebuild/restart (like dingo run with hot reload)",
		Long: `Watch transpiles and runs a Dingo program, then watches for file changes.
When .dingo or .go files change, it automatically rebuilds and restarts the program.

Features:
- 500ms debounce to avoid rapid rebuilds during save-all operations
- Graceful shutdown: interrupt first, then force kill after 2 seconds
- Timestamped output for rebuild events
- Passes stdin/stdout/stderr to the running program

Examples:
  dingo watch main.dingo                    # Watch and run single file
  dingo watch ./cmd/myapp                   # Watch package directory
  dingo watch main.dingo -- --port 8080     # Pass args to program`,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Handle --help manually since we disabled flag parsing
			for _, arg := range args {
				if arg == "-h" || arg == "--help" || arg == "-help" {
					return cmd.Help()
				}
			}
			return runWatch(args)
		},
	}
	return cmd
}

func runWatch(args []string) error {
	// Parse arguments similar to run command
	opts, noMascot, err := parseWatchArgs(args)
	if err != nil {
		return err
	}

	// Load config
	cfg, err := config.Load(nil)
	if err != nil {
		cfg = config.DefaultConfig()
	}

	// If no paths specified on CLI, use config paths
	if len(opts.DingoFiles) == 0 && len(opts.PackagePaths) == 0 {
		if len(cfg.Watch.Paths) > 0 {
			opts.PackagePaths = cfg.Watch.Paths
		}
	}

	// Resolve package paths to .dingo files
	if err := resolveDingoFiles(opts); err != nil {
		return err
	}

	if len(opts.DingoFiles) == 0 {
		return fmt.Errorf("no .dingo files found\n\nSpecify paths to watch:\n  dingo watch ./cmd/api\n  dingo watch main.dingo\n\nOr configure in dingo.toml:\n  [watch]\n  paths = [\"./cmd/api\"]")
	}

	// Verify all files are in the same directory (Go build requirement)
	if len(opts.DingoFiles) > 1 {
		firstDir := filepath.Dir(opts.DingoFiles[0])
		for _, f := range opts.DingoFiles[1:] {
			if filepath.Dir(f) != firstDir {
				return fmt.Errorf("all .dingo files must be in the same directory for 'go run'\n\nFound files in multiple directories:\n  %s\n  %s\n\nSpecify a single package: dingo watch ./cmd/api",
					opts.DingoFiles[0], f)
			}
		}
	}

	wr := &WatchRunner{
		dingoFiles:     opts.DingoFiles,
		programArgs:    opts.GoArgs, // GoArgs contains program args after --
		cfg:            cfg,
		pm:             &ProcessManager{},
		styles:         newWatchStyles(),
		generatedFiles: make(map[string]bool),
		noMascot:       noMascot,
	}

	// Set up shadow builder if shadow mode is enabled
	if cfg.Build.Shadow {
		// Find workspace root
		workspaceRoot, err := shadow.FindWorkspaceRoot(opts.DingoFiles[0])
		if err != nil {
			return fmt.Errorf("failed to find workspace root: %w", err)
		}

		// Determine shadow directory name
		shadowDir := cfg.Build.OutDir
		if shadowDir == "" {
			shadowDir = "build"
		}

		wr.workspaceRoot = workspaceRoot
		wr.shadowBuilder = shadow.NewBuilder(workspaceRoot, shadowDir, cfg)
	}

	return wr.Run()
}

func parseWatchArgs(args []string) (*CompileOptions, bool, error) {
	opts := &CompileOptions{}
	var programArgs []string
	inProgramArgs := false
	noMascot := false

	for i := 0; i < len(args); i++ {
		arg := args[i]

		// Everything after -- goes to the program
		if arg == "--" {
			inProgramArgs = true
			continue
		}

		if inProgramArgs {
			programArgs = append(programArgs, arg)
			continue
		}

		// Check for --no-mascot flag
		if arg == "--no-mascot" {
			noMascot = true
			continue
		}

		// Detect .dingo files
		if strings.HasSuffix(arg, ".dingo") {
			opts.DingoFiles = append(opts.DingoFiles, arg)
			continue
		}

		// Assume directories are package paths
		if !strings.HasPrefix(arg, "-") {
			opts.PackagePaths = append(opts.PackagePaths, arg)
		}
	}

	// Store program args in GoArgs
	opts.GoArgs = programArgs

	return opts, noMascot, nil
}

func (wr *WatchRunner) Run() error {
	// Initialize UI (if not disabled)
	if !wr.noMascot {
		wr.ui = ui.NewWatchUI()
		wr.ui.Start()
		defer wr.ui.Stop()
	} else {
		// Print simple header for non-UI mode
		titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7D56F4"))
		versionStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6C7086"))
		fmt.Printf("%s %s\n\n", titleStyle.Render("🐕 Dingo Watch"), versionStyle.Render("v"+version.Version))
	}

	// Determine workspace root
	var workspaceRoot string
	if wr.shadowBuilder != nil {
		workspaceRoot = wr.workspaceRoot
	} else {
		// Find workspace root from first dingo file
		var err error
		workspaceRoot, err = shadow.FindWorkspaceRoot(wr.dingoFiles[0])
		if err != nil {
			workspaceRoot = filepath.Dir(wr.dingoFiles[0])
		}
	}

	// Initialize dependency tracker
	wr.depTracker = NewDependencyTracker(workspaceRoot, wr.dingoFiles)
	if err := wr.depTracker.Discover(); err != nil {
		return fmt.Errorf("failed to discover dependencies: %w", err)
	}

	// Add startup event
	if wr.ui != nil {
		wr.ui.SetState(mascot.StateIdle, "Initializing...", "")
		wr.ui.AddEvent(ui.BuildEvent{
			Timestamp: time.Now(),
			EventType: ui.EventStartup,
			Message:   "Dingo Watch started",
		})
	}

	// Collect all directories to watch
	watchDirs, err := wr.collectWatchDirs()
	if err != nil {
		return err
	}

	// Set up file watcher
	wr.watcher, err = fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create watcher: %w", err)
	}
	defer wr.watcher.Close()

	// Add directories to watcher
	for _, dir := range watchDirs {
		if err := wr.watcher.Add(dir); err != nil {
			return fmt.Errorf("failed to watch %s: %w", dir, err)
		}
	}

	// Update status
	if wr.ui != nil {
		wr.ui.SetState(mascot.StateIdle, "Watching...", wr.depTracker.Summary())
	} else {
		wr.log("%s Watching %s",
			wr.styles.event.Render("👀"),
			wr.depTracker.Summary())
	}

	// Timer for transitioning to idle state (can be cancelled on crash)
	var idleTimer *time.Timer
	var idleTimerMu sync.Mutex

	// Helper to schedule transition to idle state
	scheduleIdleTransition := func() {
		if wr.ui == nil {
			return
		}
		idleTimerMu.Lock()
		defer idleTimerMu.Unlock()
		if idleTimer != nil {
			idleTimer.Stop()
		}
		idleTimer = time.AfterFunc(2*time.Second, func() {
			// Only transition to idle if process is still running
			if wr.pm.IsRunning() {
				wr.ui.SetState(mascot.StateIdle, "Watching...", wr.depTracker.Summary())
			}
		})
	}

	// Helper to cancel idle transition (called on crash)
	cancelIdleTransition := func() {
		idleTimerMu.Lock()
		defer idleTimerMu.Unlock()
		if idleTimer != nil {
			idleTimer.Stop()
			idleTimer = nil
		}
	}

	// Initial build and run
	if wr.ui != nil {
		wr.ui.SetState(mascot.StateCompiling, "Building...", "Initial build")
	}

	start := time.Now()
	_, err = wr.buildAndRun()
	duration := time.Since(start)

	if err != nil {
		if wr.ui != nil {
			wr.ui.SetState(mascot.StateFailed, "Build Failed", err.Error())
			wr.ui.AddEvent(ui.BuildEvent{
				Timestamp: time.Now(),
				EventType: ui.EventBuildFailed,
				Message:   fmt.Sprintf("Initial build failed: %v", err),
				Duration:  duration,
			})
		} else {
			wr.log("%s Initial build failed: %v", wr.styles.error.Render("✗"), err)
		}
		// Continue watching, don't exit
	} else {
		if wr.ui != nil {
			wr.ui.SetState(mascot.StateSuccess, "Running", "Build successful")
			wr.ui.AddEvent(ui.BuildEvent{
				Timestamp: time.Now(),
				EventType: ui.EventBuildSuccess,
				Message:   "Initial build successful",
				Duration:  duration,
			})

			// Refresh dependencies after successful build
			_ = wr.depTracker.RefreshDependencies()

			// Transition to idle after celebration (will be cancelled if process crashes)
			scheduleIdleTransition()
		} else {
			wr.log("%s Initial build successful", wr.styles.success.Render("✓"))
		}
	}

	// Set up signal handling (cross-platform)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)

	// Channel for triggering rebuilds (with file name for display)
	rebuildCh := make(chan string, 1)

	// Debounce timer and last changed file
	var debounceTimer *time.Timer
	var pendingFile string

	// Helper to get exit channel (may be nil)
	getExitCh := func() <-chan error {
		return wr.pm.ExitCh()
	}

	// Main event loop
	for {
		exitCh := getExitCh()
		select {
		case exitErr := <-exitCh:
			// Process crashed unexpectedly
			if exitErr != nil {
				// Cancel any pending transition to idle state
				cancelIdleTransition()

				// Parse crash info from stderr
				crashInfo := wr.pm.ParseCrash()

				// Get the actual error message from stderr (e.g., "panic: ...")
				errMsg := wr.pm.LastErrorMessage()
				if errMsg == "" {
					// Fallback to exit status if no stderr message
					if exitError, ok := exitErr.(*exec.ExitError); ok {
						errMsg = fmt.Sprintf("exit status %d", exitError.ExitCode())
					} else {
						errMsg = exitErr.Error()
					}
				}

				// Build detailed crash message with source context
				var detailedMsg string
				if crashInfo != nil && crashInfo.FullPath != "" && crashInfo.Line > 0 {
					sourceContext := GetSourceContext(crashInfo.FullPath, crashInfo.Line, 5)
					if sourceContext != "" {
						detailedMsg = fmt.Sprintf("%s\n\n%s", errMsg, sourceContext)
					} else {
						detailedMsg = errMsg
					}
				} else {
					detailedMsg = errMsg
				}

				if wr.ui != nil {
					wr.ui.SetState(mascot.StateFailed, "Crashed", errMsg)
					wr.ui.AddEvent(ui.BuildEvent{
						Timestamp: time.Now(),
						EventType: ui.EventBuildFailed,
						Message:   fmt.Sprintf("Program crashed: %s", detailedMsg),
					})
				} else {
					wr.log("%s Program crashed: %s", wr.styles.error.Render("✗"), detailedMsg)
				}
			}

		case <-sigCh:
			if wr.ui != nil {
				wr.ui.SetState(mascot.StateIdle, "Stopping...", "")
				wr.ui.AddEvent(ui.BuildEvent{
					Timestamp: time.Now(),
					EventType: ui.EventShutdown,
					Message:   "Watch stopped by user",
				})
			} else {
				wr.log("%s Stopping...", wr.styles.event.Render("⏹"))
			}
			wr.pm.Stop()
			return nil

		case event, ok := <-wr.watcher.Events:
			if !ok {
				return nil
			}

			// Check if file is in dependency set or is a watched file
			if !wr.depTracker.IsWatched(event.Name) && !wr.isWatchedFile(event.Name) {
				continue
			}

			// Only react to write/create events
			if event.Op&(fsnotify.Write|fsnotify.Create) == 0 {
				continue
			}

			// Debounce: reset timer on each event
			if debounceTimer != nil {
				debounceTimer.Stop()
			}

			pendingFile = event.Name
			debounceTimer = time.AfterFunc(watchDebounce, func() {
				// Non-blocking send to rebuild channel
				select {
				case rebuildCh <- pendingFile:
				default:
					// Already have a pending rebuild, skip
				}
			})

		case changedFile := <-rebuildCh:
			// Add file changed event
			if wr.ui != nil {
				wr.ui.AddEvent(ui.BuildEvent{
					Timestamp: time.Now(),
					EventType: ui.EventFileChanged,
					Message:   filepath.Base(changedFile),
					FilePath:  changedFile,
				})
				wr.ui.SetState(mascot.StateCompiling, "Rebuilding...", filepath.Base(changedFile))
			} else {
				wr.log("%s Change detected: %s",
					wr.styles.event.Render("📝"),
					wr.styles.file.Render(filepath.Base(changedFile)))
				wr.log("%s Rebuilding...", wr.styles.event.Render("🔄"))
			}

			// Stop current process
			wr.pm.Stop()

			// Rebuild and restart
			start := time.Now()
			_, err := wr.buildAndRun()
			duration := time.Since(start)

			if err != nil {
				if wr.ui != nil {
					wr.ui.SetState(mascot.StateFailed, "Build Failed", err.Error())
					wr.ui.AddEvent(ui.BuildEvent{
						Timestamp: time.Now(),
						EventType: ui.EventBuildFailed,
						Message:   fmt.Sprintf("Build failed: %v", err),
						Duration:  duration,
					})
				} else {
					wr.log("%s Build failed: %v", wr.styles.error.Render("✗"), err)
				}
			} else {
				if wr.ui != nil {
					wr.ui.SetState(mascot.StateSuccess, "Running", "Build successful")
					wr.ui.AddEvent(ui.BuildEvent{
						Timestamp: time.Now(),
						EventType: ui.EventBuildSuccess,
						Message:   "Build successful",
						Duration:  duration,
					})
					wr.ui.AddEvent(ui.BuildEvent{
						Timestamp: time.Now(),
						EventType: ui.EventRestart,
						Message:   "Program restarted",
					})

					// Refresh dependencies
					_ = wr.depTracker.RefreshDependencies()

					// Return to idle after celebration (will be cancelled if process crashes)
					scheduleIdleTransition()
				} else {
					wr.log("%s Restarted", wr.styles.success.Render("✓"))
				}
			}

		case err, ok := <-wr.watcher.Errors:
			if !ok {
				return nil
			}
			if wr.ui != nil {
				// Don't add watcher errors to history, just log them
			} else {
				wr.log("%s Watcher error: %v", wr.styles.error.Render("⚠"), err)
			}
		}
	}
}

func (wr *WatchRunner) collectWatchDirs() ([]string, error) {
	// Use directories discovered by dependency tracker
	wr.depTracker.mu.RLock()
	defer wr.depTracker.mu.RUnlock()

	dirs := make([]string, 0, len(wr.depTracker.allDirs))
	for dir := range wr.depTracker.allDirs {
		if !wr.isExcludedDir(dir) {
			dirs = append(dirs, dir)
		}
	}
	return dirs, nil
}

func (wr *WatchRunner) isExcludedDir(dir string) bool {
	baseName := filepath.Base(dir)
	for _, pattern := range wr.cfg.Watch.Exclude {
		// Skip glob patterns (those are for files)
		if strings.HasPrefix(pattern, "*") {
			continue
		}
		// Check if directory name matches exclude pattern
		if baseName == pattern {
			return true
		}
	}
	return false
}

func (wr *WatchRunner) isWatchedFile(path string) bool {
	ext := filepath.Ext(path)
	if ext != ".dingo" && ext != ".go" {
		return false
	}

	// Ignore .dmap files
	if ext == ".dmap" {
		return false
	}

	// Check exclude patterns from config
	baseName := filepath.Base(path)
	for _, pattern := range wr.cfg.Watch.Exclude {
		// Check if path contains excluded directory
		if !strings.HasPrefix(pattern, "*") && strings.Contains(path, pattern+string(filepath.Separator)) {
			return false
		}
		// Check if file matches glob pattern
		if matched, _ := filepath.Match(pattern, baseName); matched {
			return false
		}
	}

	// Ignore generated .go files (they correspond to our .dingo files)
	absPath, err := filepath.Abs(path)
	if err == nil {
		wr.mu.Lock()
		isGenerated := wr.generatedFiles[absPath]
		wr.mu.Unlock()
		if isGenerated {
			return false
		}
	}

	return true
}

func (wr *WatchRunner) buildAndRun() ([]string, error) {
	// Use shadow build if configured
	if wr.shadowBuilder != nil {
		return wr.buildAndRunShadow()
	}
	return wr.buildAndRunInPlace()
}

func (wr *WatchRunner) buildAndRunShadow() ([]string, error) {
	// Build shadow directory (transpile + copy)
	result, err := wr.shadowBuilder.Build(wr.dingoFiles)
	if err != nil {
		return nil, err
	}

	// Track generated files to avoid triggering rebuilds
	wr.mu.Lock()
	for _, goFile := range result.GeneratedFiles {
		wr.generatedFiles[goFile] = true
	}
	wr.mu.Unlock()

	// Determine the main package directory within the shadow dir
	// Find the relative path of the main file from workspace root
	var mainPkgDir string
	for _, mainFile := range wr.dingoFiles {
		absMainFile, err := filepath.Abs(mainFile)
		if err != nil {
			continue
		}
		relPath, err := filepath.Rel(wr.workspaceRoot, absMainFile)
		if err != nil {
			continue
		}
		// The main package dir in shadow is: shadowDir + dirname(relPath)
		mainPkgDir = filepath.Join(result.ShadowDir, filepath.Dir(relPath))
		break
	}

	if mainPkgDir == "" {
		mainPkgDir = result.ShadowDir
	}

	// Start the process using "go run ." from the main package directory
	// This allows Go to properly resolve local imports within the shadow dir
	if err := wr.pm.StartPackage(mainPkgDir, wr.programArgs); err != nil {
		return nil, err
	}

	return result.GeneratedFiles, nil
}

func (wr *WatchRunner) buildAndRunInPlace() ([]string, error) {
	// Transpile ALL discovered .dingo files (including dependencies)
	var goFiles []string
	var mainPkgDir string

	// Get all .dingo files from dependency tracker
	allDingoFiles := wr.depTracker.GetDingoFiles()

	for _, dingoFile := range allDingoFiles {
		// Read source
		src, err := os.ReadFile(dingoFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", dingoFile, err)
		}

		// Transpile using pkg/transpiler
		result, err := transpileForWatch(src, dingoFile)
		if err != nil {
			return nil, fmt.Errorf("transpilation error in %s: %w", dingoFile, err)
		}

		// Compute output path (in-place)
		goFile := strings.TrimSuffix(dingoFile, ".dingo") + ".go"

		// Track this as a generated file (to ignore changes to it)
		absGoFile, err := filepath.Abs(goFile)
		if err == nil {
			wr.mu.Lock()
			wr.generatedFiles[absGoFile] = true
			wr.mu.Unlock()
		}

		// Write .go file
		if err := os.WriteFile(goFile, result, 0644); err != nil {
			return nil, fmt.Errorf("failed to write %s: %w", goFile, err)
		}

		goFiles = append(goFiles, goFile)
	}

	// Determine the main package directory from original main files
	for _, mainFile := range wr.dingoFiles {
		absPath, err := filepath.Abs(mainFile)
		if err == nil {
			mainPkgDir = filepath.Dir(absPath)
			break
		}
	}

	// Start the process using "go run ." from the main package directory
	// This allows Go to properly resolve local imports
	if err := wr.pm.StartPackage(mainPkgDir, wr.programArgs); err != nil {
		return nil, err
	}

	return goFiles, nil
}
