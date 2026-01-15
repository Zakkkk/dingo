package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/MadAppGang/dingo/pkg/config"
	"github.com/MadAppGang/dingo/pkg/shadow"
	"github.com/MadAppGang/dingo/pkg/sourcemap/dmap"
	"github.com/MadAppGang/dingo/pkg/transpiler"
	"github.com/MadAppGang/dingo/pkg/ui"
	"github.com/MadAppGang/dingo/pkg/ui/mascot"
	"github.com/MadAppGang/dingo/pkg/version"
)

// errAlreadyPrinted is a sentinel error indicating the error was already printed.
// Used to prevent double-printing errors (once in command, once in main).
var errAlreadyPrinted = errors.New("error already printed")

// isTerminal returns true if stdout is connected to a terminal.
// Used to auto-disable mascot animation in CI, agents, and piped output.
func isTerminal() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

// CompileOptions holds parsed compile command options
type CompileOptions struct {
	// Verbose prints the go build command before execution
	Verbose bool

	// NoMascot disables mascot animation
	NoMascot bool

	// Debug enables debug mode: emits //line directives for Delve debugging
	// and adds -gcflags=-N -l to disable optimizations.
	// Can also be enabled via DINGO_DEBUG=1 environment variable.
	Debug bool

	// DingoFiles are .dingo source files to transpile first
	DingoFiles []string

	// GoFiles are .go files passed through directly
	GoFiles []string

	// PackagePaths are Go package paths (./cmd/myapp)
	PackagePaths []string

	// OutDir from config (where to place .go files)
	OutDir string

	// GoArgs are all arguments to pass to go build (flags + sources)
	GoArgs []string

	// OutputPath is the binary output path (-o flag value)
	OutputPath string
}

func goBuildCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "build [flags] [packages/files]",
		Short: "Compile Dingo sources to a binary (like go build)",
		Long: `Build transpiles .dingo files to .go and invokes go build.

All go build flags are supported and passed through directly.

Dingo-specific flags:
  --debug      Enable debug mode: emits //line directives in generated Go code
               for Delve source mapping, and adds -gcflags=-N -l to disable
               compiler optimizations. Use for debugging only.
               Can also be enabled via DINGO_DEBUG=1 environment variable.
  --verbose    Print the go build command before execution
  --no-mascot  Disable mascot animation during build

Examples:
  dingo build main.dingo                    # Compile single file
  dingo build -o myapp main.dingo           # With output name
  dingo build ./cmd/myapp                   # Package mode
  dingo build --verbose -race ./...         # Verbose with race detector
  dingo build -ldflags="-s -w" main.dingo   # With linker flags
  dingo build --debug main.dingo            # Debug build for Delve`,
		DisableFlagParsing: true, // Take raw args for go build passthrough
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGoCommand(args, "build")
		},
	}
	return cmd
}

func goRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run [flags] [packages/files] [-- program args]",
		Short: "Compile and run Dingo program (like go run)",
		Long: `Run transpiles .dingo files to .go and invokes go run.

All go run flags are supported and passed through directly.
Arguments after -- are passed to the program.

Dingo-specific flags:
  --debug      Enable debug mode: emits //line directives and disables
               optimizations. Useful for debugging with Delve.
  --verbose    Print the go run command before execution

Examples:
  dingo run main.dingo                    # Run single file
  dingo run ./cmd/myapp                   # Run package
  dingo run --verbose main.dingo          # Show go run command
  dingo run main.dingo -- --port 8080     # Pass args to program
  dingo run -race main.dingo              # Run with race detector
  dingo run --debug main.dingo            # Debug run for Delve`,
		DisableFlagParsing: true, // Take raw args for go run passthrough
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGoCommand(args, "run")
		},
	}
	return cmd
}

// runGoCommand orchestrates the full workflow for both build and run commands
func runGoCommand(args []string, goCmd string) error {
	// Parse arguments
	opts, err := parseCompileArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return errAlreadyPrinted
	}

	// For 'run' command, always disable mascot animation
	// The running program needs full access to stdin/stdout
	if goCmd == "run" {
		opts.NoMascot = true
	}

	// Auto-detect non-interactive environment (CI, agents, pipes)
	// Disable mascot if stdout is not a TTY
	if !isTerminal() {
		opts.NoMascot = true
	}

	// Load config for outdir setting
	cfg, err := config.Load(nil)
	if err != nil {
		// Silently use defaults - this is normal if no dingo.toml exists
		cfg = config.DefaultConfig()
	}

	// Resolve package paths to .dingo files
	if err := resolveDingoFiles(opts); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return errAlreadyPrinted
	}

	// Validate we have something to compile
	if len(opts.DingoFiles) == 0 && len(opts.GoFiles) == 0 && len(opts.PackagePaths) == 0 {
		fmt.Fprintf(os.Stderr, "Error: no source files or packages specified\n")
		return errAlreadyPrinted
	}

	// Create build UI for mascot animation (unless --no-mascot)
	var buildUI *ui.SimpleBuildUI
	var deferredError error // Error to print after mascot stops (deferred functions run LIFO)
	if !opts.NoMascot {
		buildUI = ui.NewSimpleBuildUI()
		buildUI.SuppressSpinnerMascot = true // Prevent runWithSpinnerCompile from printing its own mascot
		buildUI.Start()
		// Deferred functions run in LIFO order (Last In, First Out).
		// We want: Stop() runs first (prints mascot), then error prints after.
		// So defer error printing SECOND (runs after), defer Stop() FIRST (runs before).
		defer func() {
			if deferredError != nil {
				fmt.Fprintf(os.Stderr, "\nError: %v\n", deferredError)
			}
		}()
		defer buildUI.Stop()

		// Print header
		titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7D56F4"))
		versionStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6C7086"))
		fmt.Printf("%s %s\n\n", titleStyle.Render("🐕 Dingo"), versionStyle.Render("v"+version.Version))

		// Note: actual file count will be shown during shadow build discovery
	}

	// Check if shadow build is enabled (default: true)
	var buildErr error
	if cfg.Build.Shadow {
		buildErr = runWithShadowBuild(opts, cfg, buildUI, goCmd)
	} else {
		// Fall back to in-place generation
		buildErr = runWithInPlaceBuild(opts, cfg, buildUI, goCmd)
	}

	// Handle error printing: if buildUI is active, use deferred printing so error shows after mascot
	if buildErr != nil && buildErr != errAlreadyPrinted && buildUI != nil {
		deferredError = buildErr
		return errAlreadyPrinted
	}
	return buildErr
}

// runWithInPlaceBuild uses in-place generation (legacy mode)
func runWithInPlaceBuild(opts *CompileOptions, cfg *config.Config, buildUI *ui.SimpleBuildUI, goCmd string) error {
	// Force in-place generation
	cfg.Build.OutDir = ""
	opts.OutDir = ""

	// Step 1: Transpile .dingo files with UI
	generatedFiles, err := transpileDingoFilesWithUI(opts, buildUI)
	if err != nil {
		if buildUI != nil {
			buildUI.SetStatus(mascot.StateFailed, "Transpile failed!", "")
			return err // Return actual error, caller handles printing after Stop()
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return errAlreadyPrinted
	}

	// Step 2: Invoke go build or go run (with timing and UI)
	var goDuration time.Duration
	var goErr error

	if buildUI != nil {
		buildUI.SetStatus(mascot.StateCompiling, "Running go "+goCmd+"...", "")
		goErr = runWithSpinnerCompile("Go "+goCmd, buildUI, func() error {
			start := time.Now()
			err := invokeGoToolSilent(opts, generatedFiles, goCmd)
			goDuration = time.Since(start)
			return err
		})
	} else {
		start := time.Now()
		goErr = invokeGoTool(opts, generatedFiles, goCmd)
		goDuration = time.Since(start)
	}

	if goErr != nil {
		if buildUI != nil {
			buildUI.SetStatus(mascot.StateFailed, "Go "+goCmd+" failed!", "")
			return goErr // Return actual error, caller handles printing after Stop()
		}
		return goErr
	}

	// Show Go step completion
	if buildUI != nil {
		successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#5AF78E"))
		timeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6C7086")).Italic(true)
		fmt.Printf("  %s Go %s    Done %s\n\n",
			successStyle.Render("✓"),
			goCmd,
			timeStyle.Render("("+formatDuration(goDuration)+")"))
	}

	// Success!
	if buildUI != nil {
		if goCmd == "build" {
			buildUI.SetStatus(mascot.StateSuccess, "Build successful!", fmt.Sprintf("%d file(s) compiled", len(generatedFiles)))
		} else {
			buildUI.SetStatus(mascot.StateSuccess, "Run complete!", "")
		}
	}

	return nil
}

// runWithShadowBuild uses the shadow build system for compilation
func runWithShadowBuild(opts *CompileOptions, cfg *config.Config, buildUI *ui.SimpleBuildUI, goCmd string) error {
	// Find workspace root
	var workspaceRoot string
	var err error

	if len(opts.DingoFiles) > 0 {
		workspaceRoot, err = shadow.FindWorkspaceRoot(opts.DingoFiles[0])
	} else if len(opts.PackagePaths) > 0 {
		workspaceRoot, err = shadow.FindWorkspaceRoot(opts.PackagePaths[0])
	} else {
		workspaceRoot, err = shadow.FindWorkspaceRoot(".")
	}
	if err != nil {
		return fmt.Errorf("failed to find workspace root: %w", err)
	}

	// Determine shadow directory name
	shadowDir := cfg.Build.OutDir
	if shadowDir == "" {
		shadowDir = "build"
	}

	// Create shadow builder
	builder := shadow.NewBuilder(workspaceRoot, shadowDir, cfg)
	builder.Debug = opts.Debug     // Pass debug flag for //line directive emission
	builder.Verbose = opts.Verbose // Pass verbose flag for progress output

	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#5AF78E"))
	timeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6C7086")).Italic(true)
	fileStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#CDD6F4"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6C7086"))

	// Set up progress callback with text-only progress display
	// (no mascot here - the final mascot is shown by SimpleBuildUI.Stop())
	const maxVisibleFiles = 5

	// File entry with timing info
	type fileEntry struct {
		name     string
		duration time.Duration
		done     bool
	}

	// Shared state for animation goroutine
	var progressMu sync.Mutex
	var progressCurrent, progressTotal int
	var recentFiles []fileEntry
	var currentFile string
	var lastFileTime time.Time
	var animationStarted bool
	var animationDone chan struct{}
	var animationExited chan struct{}
	var lastLineCount int // Track lines printed for reliable clearing

	// Spinner characters for current file
	spinnerChars := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	spinnerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#F7DC6F")).Bold(true)
	currentFileStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#F7DC6F"))

	// Loading mascot frames for animation
	loadingFrames := [][]string{
		mascot.FrameLoading1,
		mascot.FrameLoading2,
		mascot.FrameLoading3,
		mascot.FrameLoading4,
	}
	mascotStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#7D56F4"))

	// drawProgress renders mascot + progress side by side
	//
	// Uses relative cursor movement (move up N lines) instead of save/restore.
	// This survives terminal scrolling because we track how many lines we printed
	// and move up by that amount before clearing and redrawing.
	drawProgress := func(frameIdx int, firstDraw bool) {
		progressMu.Lock()
		current := progressCurrent
		total := progressTotal
		files := make([]fileEntry, len(recentFiles))
		copy(files, recentFiles)
		currFile := currentFile
		prevLines := lastLineCount
		progressMu.Unlock()

		if total == 0 {
			return
		}

		// Clear previous output by moving up N lines, then clearing to end of screen
		// This is more robust than save/restore which breaks on terminal scroll
		if !firstDraw && prevLines > 0 {
			fmt.Printf("\033[%dA", prevLines) // Move cursor up N lines
			fmt.Print("\033[0J")              // Clear from cursor to end of screen
		}

		// Get current mascot frame
		mascotFrame := loadingFrames[frameIdx%len(loadingFrames)]

		// Build progress lines (right side)
		var progressLines []string

		// Progress bar with percentage
		progressWidth := 30
		filled := (current * progressWidth) / total
		percentage := (current * 100) / total
		bar := strings.Repeat("█", filled) + strings.Repeat("░", progressWidth-filled)
		progressLines = append(progressLines, fmt.Sprintf("%s %s %d/%d %s",
			dimStyle.Render("["+bar+"]"),
			dimStyle.Render("Transpiling"),
			current, total,
			dimStyle.Render(fmt.Sprintf("(%d%%)", percentage))))

		// Empty line for spacing
		progressLines = append(progressLines, "")

		// Completed files with timing (max 5)
		for _, f := range files {
			timeStr := dimStyle.Render(fmt.Sprintf("(%s)", formatDuration(f.duration)))
			progressLines = append(progressLines, fmt.Sprintf("  %s %s %s", successStyle.Render("✓"), fileStyle.Render(f.name), timeStr))
		}

		// Current file being processed with spinner
		if currFile != "" {
			spinner := spinnerChars[frameIdx%len(spinnerChars)]
			progressLines = append(progressLines, fmt.Sprintf("  %s %s%s", spinnerStyle.Render(spinner), currentFileStyle.Render(currFile), dimStyle.Render(" ...")))
		}

		// Render mascot + progress side by side
		lineCount := 0
		maxLines := len(mascotFrame)
		if len(progressLines) > maxLines {
			maxLines = len(progressLines)
		}

		for i := 0; i < maxLines; i++ {
			mascotLine := ""
			if i < len(mascotFrame) {
				mascotLine = mascotStyle.Render(mascotFrame[i])
			} else {
				mascotLine = strings.Repeat(" ", 22) // Mascot width padding
			}

			progressLine := ""
			if i < len(progressLines) {
				progressLine = "  " + progressLines[i]
			}

			fmt.Printf("%s%s\n", mascotLine, progressLine)
			lineCount++
		}

		// Update line count for next iteration (under lock)
		progressMu.Lock()
		lastLineCount = lineCount
		progressMu.Unlock()
	}

	if buildUI != nil {
		builder.OnProgress = func(current, total int, file string) {
			now := time.Now()
			progressMu.Lock()

			// Start animation on first callback
			if !animationStarted {
				animationStarted = true
				animationDone = make(chan struct{})
				animationExited = make(chan struct{})
				lastFileTime = now

				// Print header
				if total == 1 {
					fmt.Println("Building 1 file")
				} else {
					fmt.Printf("Building %d files\n", total)
				}
				fmt.Println()

				// No cursor save needed - we use relative line count tracking
				// which survives terminal scrolling

				// Start animation goroutine
				go func() {
					ticker := time.NewTicker(80 * time.Millisecond)
					defer ticker.Stop()
					frameIdx := 0
					firstDraw := true
					for {
						select {
						case <-animationDone:
							// Signal that we've exited
							close(animationExited)
							return
						case <-ticker.C:
							drawProgress(frameIdx, firstDraw)
							firstDraw = false
							frameIdx++
						}
					}
				}()
			}

			// Calculate duration for the previous file
			fileDuration := now.Sub(lastFileTime)
			lastFileTime = now

			// Add completed file with timing (skip first callback which has no previous file)
			if currentFile != "" {
				recentFiles = append(recentFiles, fileEntry{
					name:     currentFile,
					duration: fileDuration,
					done:     true,
				})
				if len(recentFiles) > maxVisibleFiles {
					recentFiles = recentFiles[1:]
				}
			}

			// Update current file
			currentFile = file
			progressCurrent = current
			progressTotal = total
			progressMu.Unlock()
		}
	}

	// Step 1: Build shadow directory
	var result *shadow.BuildResult
	var shadowDuration time.Duration

	if buildUI != nil {
		buildUI.SetStatus(mascot.StateCompiling, "Transpiling...", "please wait")
		start := time.Now()
		result, err = builder.Build(opts.DingoFiles)
		shadowDuration = time.Since(start)
	} else {
		start := time.Now()
		result, err = builder.Build(opts.DingoFiles)
		shadowDuration = time.Since(start)
	}

	// Stop animation goroutine and wait for it to actually exit
	if animationDone != nil {
		close(animationDone)
		<-animationExited // Wait for goroutine to confirm exit
	}

	if err != nil {
		// Clear animation area before showing error
		if animationStarted && lastLineCount > 0 {
			fmt.Printf("\033[%dA", lastLineCount) // Move cursor up N lines
			fmt.Print("\033[0J")                  // Clear from cursor to end of screen
		}
		// Show sad mascot - error will be printed by caller after mascot stops
		if buildUI != nil {
			buildUI.SetStatus(mascot.StateFailed, "Build failed!", "")
			return err // Return actual error, caller handles printing after Stop()
		}
		// No buildUI - print error directly
		fmt.Fprintf(os.Stderr, "\nError: %v\n", err)
		return errAlreadyPrinted
	}

	if buildUI != nil && animationStarted {
		// Clear animation area using relative line movement
		// This survives terminal scrolling unlike cursor save/restore
		if lastLineCount > 0 {
			fmt.Printf("\033[%dA", lastLineCount) // Move cursor up N lines
			fmt.Print("\033[0J")                  // Clear from cursor to end of screen
		}

		// Show completion summary with transpiled vs skipped distinction
		if result.TranspiledCount > 0 {
			fmt.Printf("  %s Transpile   %d files %s\n",
				successStyle.Render("✓"),
				result.TranspiledCount,
				timeStyle.Render("("+formatDuration(shadowDuration)+")"))
		} else {
			fmt.Printf("  %s Transpile   %d files up to date\n",
				successStyle.Render("✓"),
				result.SkippedCount)
		}
		fmt.Println()
	} else if buildUI == nil && opts.Verbose {
		// In no-mascot verbose mode, show what was transpiled
		if result.TranspiledCount > 0 {
			fmt.Printf("Transpiled %d file(s) in %s\n", result.TranspiledCount, formatDuration(shadowDuration))
		} else if result.SkippedCount > 0 {
			fmt.Printf("All %d file(s) up to date\n", result.SkippedCount)
		}
	}

	// Step 2: Run go build/run from shadow directory
	var goDuration time.Duration
	var goErr error

	if buildUI != nil {
		buildUI.SetStatus(mascot.StateCompiling, "Running go "+goCmd+"...", "")
		goErr = runWithSpinnerCompile("Go "+goCmd, buildUI, func() error {
			start := time.Now()
			err := invokeGoToolFromShadow(opts, result, goCmd, workspaceRoot)
			goDuration = time.Since(start)
			return err
		})
	} else {
		start := time.Now()
		goErr = invokeGoToolFromShadow(opts, result, goCmd, workspaceRoot)
		goDuration = time.Since(start)
	}

	if goErr != nil {
		if buildUI != nil {
			buildUI.SetStatus(mascot.StateFailed, "Go "+goCmd+" failed!", "")
			return goErr // Return actual error, caller handles printing after Stop()
		}
		return goErr
	}

	// Show Go step completion
	if buildUI != nil {
		fmt.Printf("  %s Go %s    Done %s\n\n",
			successStyle.Render("✓"),
			goCmd,
			timeStyle.Render("("+formatDuration(goDuration)+")"))
	}

	// Success!
	if buildUI != nil {
		if goCmd == "build" {
			var detail string
			if result.TranspiledCount > 0 {
				detail = fmt.Sprintf("%d file(s) transpiled", result.TranspiledCount)
			} else {
				detail = fmt.Sprintf("%d file(s) up to date", result.SkippedCount)
			}
			buildUI.SetStatus(mascot.StateSuccess, "Build successful!", detail)
		} else {
			buildUI.SetStatus(mascot.StateSuccess, "Run complete!", "")
		}
	}

	return nil
}

// invokeGoToolFromShadow executes go build or go run from the shadow directory
func invokeGoToolFromShadow(opts *CompileOptions, result *shadow.BuildResult, goCmd string, workspaceRoot string) error {
	// Build argument list
	args := []string{goCmd}

	// Add debug flags: disable optimizations and inlining for better debugging
	// -N: disable optimizations
	// -l: disable inlining
	if opts.Debug {
		args = append(args, "-gcflags=all=-N -l")
	}

	// Add -o flag with path relative to shadow (output goes to workspace root)
	if opts.OutputPath != "" && goCmd == "build" {
		// Make output path absolute relative to workspace root
		outputPath := opts.OutputPath
		if !filepath.IsAbs(outputPath) {
			outputPath = filepath.Join(workspaceRoot, outputPath)
		}
		args = append(args, "-o", outputPath)
	} else if goCmd == "build" {
		// Default output name: binary goes to workspace root
		// Get the package name or main file name
		var binaryName string
		if len(opts.PackagePaths) > 0 {
			binaryName = filepath.Base(opts.PackagePaths[0])
		} else if len(opts.DingoFiles) > 0 {
			binaryName = strings.TrimSuffix(filepath.Base(opts.DingoFiles[0]), ".dingo")
		} else {
			binaryName = "main"
		}
		args = append(args, "-o", filepath.Join(workspaceRoot, binaryName))
	}

	// Add Go flags from GoArgs (excluding package paths which we handle differently)
	for _, arg := range opts.GoArgs {
		// Skip package paths - we'll specify them relative to shadow
		if !strings.HasPrefix(arg, "-") && !strings.HasSuffix(arg, ".go") {
			continue
		}
		args = append(args, arg)
	}

	// For file mode, add only the .go files corresponding to the specified .dingo files
	if len(opts.PackagePaths) == 0 && len(opts.DingoFiles) > 0 {
		// Build only the files that correspond to the originally specified .dingo files
		for _, dingoFile := range opts.DingoFiles {
			// Get relative path from workspace root
			absDingo, _ := filepath.Abs(dingoFile)
			relDingo, _ := filepath.Rel(workspaceRoot, absDingo)
			relGo := strings.TrimSuffix(relDingo, ".dingo") + ".go"
			args = append(args, relGo)
		}
	} else if len(opts.PackagePaths) > 0 {
		// Package mode - preserve the original package path
		// Convert workspace-relative path to shadow-relative
		for _, pkgPath := range opts.PackagePaths {
			// Get relative path from workspace root
			absPath, _ := filepath.Abs(pkgPath)
			relPath, err := filepath.Rel(workspaceRoot, absPath)
			if err != nil || relPath == "." {
				// Can't determine or it's the workspace root - use "."
				args = append(args, ".")
			} else {
				// Use relative path (e.g., ./cmd/api)
				args = append(args, "./"+relPath)
			}
		}
	}

	// Verbose: print the command
	if opts.Verbose {
		fmt.Printf("+ cd %s && go %s\n", result.ShadowDir, strings.Join(args, " "))
	}

	cmd := exec.Command("go", args...)
	cmd.Dir = result.ShadowDir // Run from shadow directory
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	return cmd.Run()
}

// parseCompileArgs separates dingo options from go build args
func parseCompileArgs(args []string) (*CompileOptions, error) {
	opts := &CompileOptions{}

	// Check DINGO_DEBUG environment variable
	if os.Getenv("DINGO_DEBUG") == "1" {
		opts.Debug = true
	}

	for i := 0; i < len(args); i++ {
		arg := args[i]

		// Dingo-specific: --verbose (only long form to avoid -v collision with go build)
		if arg == "--verbose" {
			opts.Verbose = true
			continue
		}

		// Dingo-specific: --no-mascot (disable mascot animation)
		if arg == "--no-mascot" {
			opts.NoMascot = true
			continue
		}

		// Dingo-specific: --debug (enable debug mode with //line directives)
		if arg == "--debug" {
			opts.Debug = true
			continue
		}

		// Extract -o flag and its value
		if arg == "-o" {
			// Next arg is the output path
			if i+1 >= len(args) {
				return nil, fmt.Errorf("-o flag requires an argument")
			}
			opts.OutputPath = args[i+1]
			i++ // Skip the next arg since we consumed it
			continue
		}

		// Handle -o=path format
		if strings.HasPrefix(arg, "-o=") {
			opts.OutputPath = strings.TrimPrefix(arg, "-o=")
			continue
		}

		// Detect .dingo files
		if strings.HasSuffix(arg, ".dingo") {
			opts.DingoFiles = append(opts.DingoFiles, arg)
			continue
		}

		// Everything else goes to Go
		opts.GoArgs = append(opts.GoArgs, arg)
	}

	// Classify non-flag args in GoArgs as package paths
	for _, arg := range opts.GoArgs {
		if !strings.HasPrefix(arg, "-") && !strings.HasSuffix(arg, ".go") {
			opts.PackagePaths = append(opts.PackagePaths, arg)
		}
	}

	// Validate not mixing file and package modes
	hasFiles := len(opts.DingoFiles) > 0
	hasPackages := len(opts.PackagePaths) > 0
	if hasFiles && hasPackages {
		return nil, fmt.Errorf("cannot mix file mode (.dingo files) and package mode (directories): use one or the other")
	}

	return opts, nil
}

// resolveDingoFiles finds all .dingo files from package paths
func resolveDingoFiles(opts *CompileOptions) error {
	for _, pkgPath := range opts.PackagePaths {
		// Check for workspace patterns
		if strings.Contains(pkgPath, "...") {
			return fmt.Errorf("recursive patterns like './...' are not yet supported\nUse 'dingo go ./...' for transpile-only, or specify individual packages")
		}

		// Resolve to absolute path
		absPath, err := filepath.Abs(pkgPath)
		if err != nil {
			return fmt.Errorf("failed to resolve %s: %w", pkgPath, err)
		}

		// Check if it's a directory
		info, err := os.Stat(absPath)
		if err != nil {
			return fmt.Errorf("failed to stat %s: %w", pkgPath, err)
		}

		if info.IsDir() {
			// Scan for .dingo files (recursively)
			files, err := scanDirForDingo(absPath)
			if err != nil {
				return err
			}

			// Warn if no .dingo files found
			if len(files) == 0 {
				fmt.Fprintf(os.Stderr, "Warning: No .dingo files found in %s\n", pkgPath)
			}

			opts.DingoFiles = append(opts.DingoFiles, files...)
		}
	}
	return nil
}

// scanDirForDingo finds all .dingo files in a directory recursively
func scanDirForDingo(dir string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".dingo") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk directory %s: %w", dir, err)
	}
	return files, nil
}

// transpileDingoFilesWithUI transpiles .dingo files with optional UI feedback and spinner animation
func transpileDingoFilesWithUI(opts *CompileOptions, buildUI *ui.SimpleBuildUI) ([]string, error) {
	var generatedFiles []string

	// Load dingo config for output path calculation
	cfg, err := config.Load(nil)
	if err != nil {
		cfg = config.DefaultConfig()
	}

	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#5AF78E"))
	timeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6C7086")).Italic(true)
	inputStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#CDD6F4"))
	arrowStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6C7086"))
	outputStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#5AF78E"))

	for _, dingoFile := range opts.DingoFiles {
		// Determine output path using config
		goFile := computeOutputPath(dingoFile, opts.OutDir, cfg)

		// Print file header if UI enabled
		if buildUI != nil {
			fmt.Printf("  %s %s %s\n\n",
				inputStyle.Render(dingoFile),
				arrowStyle.Render("→"),
				outputStyle.Render(goFile))
		}

		// Step 1: Read source (with spinner if UI enabled)
		var src []byte
		var readDuration time.Duration
		var readErr error

		if buildUI != nil {
			readErr = runWithSpinnerCompile("Read", buildUI, func() error {
				start := time.Now()
				var err error
				src, err = os.ReadFile(dingoFile)
				readDuration = time.Since(start)
				return err
			})
		} else {
			start := time.Now()
			src, readErr = os.ReadFile(dingoFile)
			readDuration = time.Since(start)
		}

		if readErr != nil {
			return nil, fmt.Errorf("failed to read %s: %w", dingoFile, readErr)
		}
		if buildUI != nil {
			fmt.Printf("  %s Read        Done %s\n",
				successStyle.Render("✓"),
				timeStyle.Render("("+formatDuration(readDuration)+")"))
		}

		// Step 2: Transpile using existing pipeline (with spinner - this is the long one!)
		var result transpiler.TranspileResult
		var transpileDuration time.Duration
		var transpileErr error

		// Configure transpile options (debug mode emits //line directives for Delve)
		transpileOpts := transpiler.TranspileOptions{
			InferTypes: true,
			Debug:      opts.Debug,
		}

		if buildUI != nil {
			buildUI.SetStatus(mascot.StateCompiling, "Transpiling...", filepath.Base(dingoFile))
			transpileErr = runWithSpinnerCompile("Transpile", buildUI, func() error {
				start := time.Now()
				var err error
				result, err = transpiler.PureASTTranspileWithMappingsOpts(src, dingoFile, transpileOpts)
				transpileDuration = time.Since(start)
				return err
			})
		} else {
			start := time.Now()
			result, transpileErr = transpiler.PureASTTranspileWithMappingsOpts(src, dingoFile, transpileOpts)
			transpileDuration = time.Since(start)
		}

		if transpileErr != nil {
			return nil, fmt.Errorf("transpilation error in %s: %w", dingoFile, transpileErr)
		}

		// Extract Go code and mappings from transpilation result
		goSource := result.GoCode
		lineMappings := result.LineMappings
		columnMappings := result.ColumnMappings

		if buildUI != nil {
			fmt.Printf("  %s Transpile   Done %s\n",
				successStyle.Render("✓"),
				timeStyle.Render("("+formatDuration(transpileDuration)+")"))
		}

		// Ensure output directory exists
		if dir := filepath.Dir(goFile); dir != "." {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
			}
		}

		// Step 3: Write output files (with spinner if UI enabled)
		var writeDuration time.Duration
		var writeErr error

		if buildUI != nil {
			writeErr = runWithSpinnerCompile("Write", buildUI, func() error {
				start := time.Now()

				// Write .go file
				err := os.WriteFile(goFile, goSource, 0644)
				if err != nil {
					return err
				}

				// Write .dmap file alongside .go file in output directory
				dmapPath, dmapPathErr := calculateDmapPath(dingoFile, cfg)
				if dmapPathErr == nil {
					writer := dmap.NewWriter(src, goSource)
					// Write v3 format with column mappings
					if dmapErr := writer.WriteFile(dmapPath, lineMappings, columnMappings); dmapErr != nil {
						// .dmap write failure is non-fatal - warn but don't fail build
						fmt.Printf("\n  ⚠ Warning: Failed to write source map: %v\n", dmapErr)
					}
				}

				writeDuration = time.Since(start)
				return nil
			})
		} else {
			start := time.Now()

			// Write .go file
			writeErr = os.WriteFile(goFile, goSource, 0644)
			if writeErr == nil {
				// Write .dmap file alongside .go file in output directory
				dmapPath, dmapPathErr := calculateDmapPath(dingoFile, cfg)
				if dmapPathErr == nil {
					writer := dmap.NewWriter(src, goSource)
					// Write v3 format with column mappings
					if dmapErr := writer.WriteFile(dmapPath, lineMappings, columnMappings); dmapErr != nil {
						// .dmap write failure is non-fatal - warn but don't fail build
						fmt.Printf("\n  ⚠ Warning: Failed to write source map: %v\n", dmapErr)
					}
				}
			}

			writeDuration = time.Since(start)
		}

		if writeErr != nil {
			return nil, fmt.Errorf("failed to write %s: %w", goFile, writeErr)
		}
		if buildUI != nil {
			fmt.Printf("  %s Write       Done %s\n",
				successStyle.Render("✓"),
				timeStyle.Render("("+formatDuration(writeDuration)+")"))
			fmt.Printf("    %s\n\n", timeStyle.Render(fmt.Sprintf("%d bytes written", len(goSource))))
		}

		generatedFiles = append(generatedFiles, goFile)
	}

	return generatedFiles, nil
}

// runWithSpinnerCompile runs a function while showing animated mascot spinner.
// If buildUI is provided with SuppressSpinnerMascot=true, this function
// skips the mascot and runs fn silently to prevent duplicate displays.
func runWithSpinnerCompile(stepName string, buildUI *ui.SimpleBuildUI, fn func() error) error {
	// Skip mascot if buildUI is suppressing it
	if buildUI != nil && buildUI.SuppressSpinnerMascot {
		return fn()
	}

	done := make(chan error, 1)
	go func() {
		done <- fn()
	}()

	frameIdx := 0
	ticker := time.NewTicker(120 * time.Millisecond)
	defer ticker.Stop()

	// Mascot frames with spinner eyes
	mascotFrames := [][]string{
		{
			"     ▄▀▄    ▄▀▄       ",
			"     █  ▀▀▀▀▀  █      ",
			"     █  ◜   ◜  █      ",
			"     ▀▄   ▲   ▄▀      ",
			"       ▀▄▄▄▄▄▀        ",
			"      ▄█▀   ▀█▄       ",
			"     ██  ███  ██      ",
			"     ▀█▄▄▀ ▀▄▄█▀      ",
		},
		{
			"     ▄▀▄    ▄▀▄       ",
			"     █  ▀▀▀▀▀  █      ",
			"     █  ◝   ◝  █      ",
			"     ▀▄   ▲   ▄▀      ",
			"       ▀▄▄▄▄▄▀        ",
			"      ▄█▀   ▀█▄       ",
			"     ██  ███  ██      ",
			"     ▀█▄▄▀ ▀▄▄█▀      ",
		},
		{
			"     ▄▀▄    ▄▀▄       ",
			"     █  ▀▀▀▀▀  █      ",
			"     █  ◞   ◞  █      ",
			"     ▀▄   ▲   ▄▀      ",
			"       ▀▄▄▄▄▄▀        ",
			"      ▄█▀   ▀█▄       ",
			"     ██  ███  ██      ",
			"     ▀█▄▄▀ ▀▄▄█▀      ",
		},
		{
			"     ▄▀▄    ▄▀▄       ",
			"     █  ▀▀▀▀▀  █      ",
			"     █  ◟   ◟  █      ",
			"     ▀▄   ▲   ▄▀      ",
			"       ▀▄▄▄▄▄▀        ",
			"      ▄█▀   ▀█▄       ",
			"     ██  ███  ██      ",
			"     ▀█▄▄▀ ▀▄▄█▀      ",
		},
	}

	mascotColor := lipgloss.NewStyle().Foreground(lipgloss.Color("#7D56F4"))
	statusStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7D56F4"))
	detailStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6C7086")).Italic(true)
	separatorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#45475A"))

	// Status text to show next to mascot
	statusLines := []string{
		"",
		statusStyle.Render(stepName + "..."),
		detailStyle.Render("please wait"),
	}

	// lastLineCount tracks lines printed for reliable clearing (robust against scrolling)
	lastLineCount := 0

	// Draw mascot frame
	drawMascotFrame := func(frame []string) {
		// Clear previous output if we've drawn before
		if lastLineCount > 0 {
			fmt.Printf("\033[%dA", lastLineCount) // Move cursor up N lines
			fmt.Print("\033[0J")                  // Clear from cursor to end of screen
		}

		lineCount := 0

		// Separator
		fmt.Println(separatorStyle.Render("────────────────────────────────────────────────────────────"))
		lineCount++

		maxLines := len(frame)
		for i := 0; i < maxLines; i++ {
			mascotLine := mascotColor.Render(frame[i])
			statusLine := ""
			if i < len(statusLines) {
				statusLine = "  " + statusLines[i]
			}
			fmt.Printf("%s%s\n", mascotLine, statusLine)
			lineCount++
		}

		lastLineCount = lineCount
	}

	// Hide cursor
	fmt.Print("\033[?25l")

	// Draw first frame
	drawMascotFrame(mascotFrames[frameIdx])

	for {
		select {
		case err := <-done:
			// Clear animation one last time
			if lastLineCount > 0 {
				fmt.Printf("\033[%dA", lastLineCount) // Move cursor up N lines
				fmt.Print("\033[0J")                  // Clear from cursor to end of screen
			}
			fmt.Print("\033[?25h") // Show cursor
			return err
		case <-ticker.C:
			frameIdx = (frameIdx + 1) % len(mascotFrames)
			drawMascotFrame(mascotFrames[frameIdx])
		}
	}
}

// computeOutputPath determines where to write the .go file.
// Uses the unified output path calculation from the transpiler package.
// When outDir is empty, uses the configured output directory (default: build/).
func computeOutputPath(dingoFile, outDir string, cfg *config.Config) string {
	// If explicit outDir is set, use custom logic (backwards compatibility)
	if outDir != "" {
		base := strings.TrimSuffix(dingoFile, ".dingo") + ".go"

		// OutDir set: mirror structure in output directory
		// e.g., cmd/app/main.dingo -> <outDir>/cmd/app/main.go
		// Use absolute paths for reliability
		absDingo, err := filepath.Abs(dingoFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Cannot get absolute path for %s: %v\nUsing flat structure in output directory.\n", dingoFile, err)
			return filepath.Join(outDir, filepath.Base(base))
		}

		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Cannot get working directory: %v\nUsing flat structure in output directory.\n", err)
			return filepath.Join(outDir, filepath.Base(base))
		}

		// Compute relative path from cwd
		rel, err := filepath.Rel(cwd, absDingo)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Cannot compute relative path for %s: %v\nUsing flat structure in output directory.\n", dingoFile, err)
			return filepath.Join(outDir, filepath.Base(base))
		}

		return filepath.Join(outDir, strings.TrimSuffix(rel, ".dingo")+".go")
	}

	// Use unified path calculation (respects config's OutDir, defaults to build/)
	goPath, err := transpiler.CalculateGoPath(dingoFile, cfg)
	if err != nil {
		// Fallback to in-place if calculation fails
		fmt.Fprintf(os.Stderr, "Warning: Failed to calculate output path: %v\nFalling back to in-place output.\n", err)
		return strings.TrimSuffix(dingoFile, ".dingo") + ".go"
	}
	return goPath
}

// invokeGoTool executes go build or go run with the constructed arguments
func invokeGoTool(opts *CompileOptions, generatedGoFiles []string, goCmd string) error {
	return invokeGoToolWithOutput(opts, generatedGoFiles, goCmd, true)
}

// invokeGoToolSilent executes go build/run without verbose output (for use during spinner)
func invokeGoToolSilent(opts *CompileOptions, generatedGoFiles []string, goCmd string) error {
	return invokeGoToolWithOutput(opts, generatedGoFiles, goCmd, false)
}

// invokeGoToolWithOutput executes go build or go run with configurable output
func invokeGoToolWithOutput(opts *CompileOptions, generatedGoFiles []string, goCmd string, showVerbose bool) error {
	// Build final argument list
	args := []string{goCmd}

	// Add debug flags: disable optimizations and inlining for better debugging
	// -N: disable optimizations
	// -l: disable inlining
	if opts.Debug {
		args = append(args, "-gcflags=all=-N -l")
	}

	// Add -o flag if specified (only for build, not run)
	if opts.OutputPath != "" && goCmd == "build" {
		args = append(args, "-o", opts.OutputPath)
	}

	// Add all go flags and package paths (already in GoArgs)
	args = append(args, opts.GoArgs...)

	// Add generated .go files for file mode (when no package paths specified)
	// For package mode, go build/run discovers files in the package directory
	if len(opts.PackagePaths) == 0 && len(generatedGoFiles) > 0 {
		args = append(args, generatedGoFiles...)
	}

	// Verbose: print the command (only if showVerbose and opts.Verbose)
	if showVerbose && opts.Verbose {
		fmt.Printf("+ go %s\n", strings.Join(args, " "))
	}

	cmd := exec.Command("go", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	return cmd.Run()
}
