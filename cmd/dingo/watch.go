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

	"github.com/charmbracelet/lipgloss"
	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"
	"github.com/MadAppGang/dingo/pkg/config"
	"github.com/MadAppGang/dingo/pkg/shadow"
	"github.com/MadAppGang/dingo/pkg/transpiler"
	"github.com/MadAppGang/dingo/pkg/version"
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
	cmd       *exec.Cmd
	cancel    context.CancelFunc
	mu        sync.Mutex
	running   bool
	doneCh    chan struct{}
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
	opts, err := parseWatchArgs(args)
	if err != nil {
		return err
	}

	// Load config
	cfg, err := config.Load(nil)
	if err != nil {
		cfg = config.DefaultConfig()
	}

	// Resolve package paths to .dingo files
	if err := resolveDingoFiles(opts); err != nil {
		return err
	}

	if len(opts.DingoFiles) == 0 {
		return fmt.Errorf("no .dingo files found")
	}

	wr := &WatchRunner{
		dingoFiles:     opts.DingoFiles,
		programArgs:    opts.GoArgs, // GoArgs contains program args after --
		cfg:            cfg,
		pm:             &ProcessManager{},
		styles:         newWatchStyles(),
		generatedFiles: make(map[string]bool),
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

func parseWatchArgs(args []string) (*CompileOptions, error) {
	opts := &CompileOptions{}
	var programArgs []string
	inProgramArgs := false

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

	return opts, nil
}

func (wr *WatchRunner) Run() error {
	// Print header
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7D56F4"))
	versionStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6C7086"))
	fmt.Printf("%s %s\n\n", titleStyle.Render("🐕 Dingo Watch"), versionStyle.Render("v"+version.Version))

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

	wr.log("%s Watching %d directories for .dingo and .go files",
		wr.styles.event.Render("👀"),
		len(watchDirs))

	// Initial build and run
	_, err = wr.buildAndRun()
	if err != nil {
		wr.log("%s Initial build failed: %v", wr.styles.error.Render("✗"), err)
		// Continue watching, don't exit
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
			wr.log("%s Stopping...", wr.styles.event.Render("⏹"))
			wr.pm.Stop()
			return nil

		case event, ok := <-wr.watcher.Events:
			if !ok {
				return nil
			}

			// Filter for .dingo and .go files only
			if !wr.isWatchedFile(event.Name) {
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
			// Perform rebuild synchronously in main loop
			wr.log("%s Change detected: %s",
				wr.styles.event.Render("📝"),
				wr.styles.file.Render(filepath.Base(changedFile)))
			wr.log("%s Rebuilding...", wr.styles.event.Render("🔄"))

			// Stop current process
			wr.pm.Stop()

			// Rebuild and restart
			_, err := wr.buildAndRun()
			if err != nil {
				wr.log("%s Build failed: %v", wr.styles.error.Render("✗"), err)
			} else {
				wr.log("%s Restarted", wr.styles.success.Render("✓"))
			}

		case err, ok := <-wr.watcher.Errors:
			if !ok {
				return nil
			}
			wr.log("%s Watcher error: %v", wr.styles.error.Render("⚠"), err)
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
		dirSet[filepath.Dir(absPath)] = struct{}{}
	}

	// For each dingo file directory, also watch parent directory if it contains pkg/ or internal/
	// This helps catch related Go file changes
	for dir := range dirSet {
		parent := filepath.Dir(dir)
		if parent != dir && parent != "." {
			dirSet[parent] = struct{}{}
		}
	}

	dirs := make([]string, 0, len(dirSet))
	for d := range dirSet {
		dirs = append(dirs, d)
	}
	return dirs, nil
}

func (wr *WatchRunner) isWatchedFile(path string) bool {
	ext := filepath.Ext(path)
	if ext != ".dingo" && ext != ".go" {
		return false
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
