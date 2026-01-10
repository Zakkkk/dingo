package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/MadAppGang/dingo/pkg/build"
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
	cmd     *exec.Cmd
	cancel  context.CancelFunc
	mu      sync.Mutex
	running bool
	doneCh  chan struct{}
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

// DependencyTracker manages file dependencies for watch mode
type DependencyTracker struct {
	workspaceRoot string
	mainFiles     []string // User-specified .dingo files

	// Computed dependencies
	allDingoFiles map[string]bool // All .dingo files to watch
	allGoFiles    map[string]bool // All .go files to watch

	mu sync.RWMutex
}

// NewDependencyTracker creates a dependency tracker
func NewDependencyTracker(workspaceRoot string, mainFiles []string) *DependencyTracker {
	return &DependencyTracker{
		workspaceRoot: workspaceRoot,
		mainFiles:     mainFiles,
		allDingoFiles: make(map[string]bool),
		allGoFiles:    make(map[string]bool),
	}
}

// Discover scans import statements and builds dependency set
func (dt *DependencyTracker) Discover() error {
	dt.mu.Lock()
	defer dt.mu.Unlock()

	// Reset maps
	dt.allDingoFiles = make(map[string]bool)
	dt.allGoFiles = make(map[string]bool)

	// Add main files to watch set
	for _, mainFile := range dt.mainFiles {
		absPath, err := filepath.Abs(mainFile)
		if err != nil {
			return fmt.Errorf("failed to get absolute path for %s: %w", mainFile, err)
		}
		dt.allDingoFiles[absPath] = true
	}

	// Build packages list for dependency graph
	packages := make([]build.Package, 0)
	dirMap := make(map[string][]string) // Dir -> dingo files

	// Group files by directory
	for _, mainFile := range dt.mainFiles {
		absPath, _ := filepath.Abs(mainFile)
		dir := filepath.Dir(absPath)
		dirMap[dir] = append(dirMap[dir], filepath.Base(absPath))
	}

	// Create packages
	for dir, files := range dirMap {
		relDir, err := filepath.Rel(dt.workspaceRoot, dir)
		if err != nil {
			relDir = dir
		}
		packages = append(packages, build.Package{
			Path:       relDir,
			DingoFiles: files,
		})
	}

	// Build dependency graph (only if workspace root has go.mod)
	if _, err := os.Stat(filepath.Join(dt.workspaceRoot, "go.mod")); err == nil {
		// Dependency graph analysis would go here
		// For now, we watch the directories containing the main files
		// and their parent directories (simple approach)
	}

	// Also watch pure .go files in the same directories
	for dir := range dirMap {
		goFiles, err := filepath.Glob(filepath.Join(dir, "*.go"))
		if err == nil {
			for _, goFile := range goFiles {
				absPath, _ := filepath.Abs(goFile)
				dt.allGoFiles[absPath] = true
			}
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
	depCount := wr.depTracker.Count()
	if wr.ui != nil {
		wr.ui.SetState(mascot.StateIdle, "Watching...", fmt.Sprintf("%d dependencies", depCount))
	} else {
		wr.log("%s Watching %d directories (%d dependencies)",
			wr.styles.event.Render("👀"),
			len(watchDirs),
			depCount)
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

			// Transition to idle after celebration
			time.AfterFunc(2*time.Second, func() {
				wr.ui.SetState(mascot.StateIdle, "Watching...", fmt.Sprintf("%d dependencies", wr.depTracker.Count()))
			})
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

	// Main event loop
	for {
		select {
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

					// Return to idle after celebration
					time.AfterFunc(2*time.Second, func() {
						wr.ui.SetState(mascot.StateIdle, "Watching...", fmt.Sprintf("%d dependencies", wr.depTracker.Count()))
					})
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
	dirSet := make(map[string]struct{})

	// Add directories containing .dingo files
	for _, f := range wr.dingoFiles {
		absPath, err := filepath.Abs(f)
		if err != nil {
			return nil, err
		}
		dir := filepath.Dir(absPath)
		if !wr.isExcludedDir(dir) {
			dirSet[dir] = struct{}{}
		}
	}

	// For each dingo file directory, also watch parent directory if it contains pkg/ or internal/
	// This helps catch related Go file changes
	for dir := range dirSet {
		parent := filepath.Dir(dir)
		if parent != dir && parent != "." && !wr.isExcludedDir(parent) {
			dirSet[parent] = struct{}{}
		}
	}

	dirs := make([]string, 0, len(dirSet))
	for d := range dirSet {
		dirs = append(dirs, d)
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

	// Start the process from shadow directory
	if err := wr.pm.StartFromDir(result.ShadowDir, result.GeneratedFiles, wr.programArgs); err != nil {
		return nil, err
	}

	return result.GeneratedFiles, nil
}

func (wr *WatchRunner) buildAndRunInPlace() ([]string, error) {
	// Transpile .dingo files in-place
	var goFiles []string

	for _, dingoFile := range wr.dingoFiles {
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

	// Start the process
	if err := wr.pm.Start(goFiles, wr.programArgs); err != nil {
		return nil, err
	}

	return goFiles, nil
}
