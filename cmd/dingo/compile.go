package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/MadAppGang/dingo/pkg/config"
	"github.com/MadAppGang/dingo/pkg/sourcemap/dmap"
	"github.com/MadAppGang/dingo/pkg/transpiler"
	"github.com/MadAppGang/dingo/pkg/ui"
	"github.com/MadAppGang/dingo/pkg/ui/mascot"
	"github.com/MadAppGang/dingo/pkg/version"
)

// CompileOptions holds parsed compile command options
type CompileOptions struct {
	// Verbose prints the go build command before execution
	Verbose bool

	// NoMascot disables mascot animation
	NoMascot bool

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

Examples:
  dingo build main.dingo                    # Compile single file
  dingo build -o myapp main.dingo           # With output name
  dingo build ./cmd/myapp                   # Package mode
  dingo build --verbose -race ./...         # Verbose with race detector
  dingo build -ldflags="-s -w" main.dingo   # With linker flags`,
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

Examples:
  dingo run main.dingo                    # Run single file
  dingo run ./cmd/myapp                   # Run package
  dingo run --verbose main.dingo          # Show go run command
  dingo run main.dingo -- --port 8080     # Pass args to program
  dingo run -race main.dingo              # Run with race detector`,
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
		return err
	}

	// For 'run' command, always disable mascot animation
	// The running program needs full access to stdin/stdout
	if goCmd == "run" {
		opts.NoMascot = true
	}

	// Load config for outdir setting
	cfg, err := config.Load(nil)
	if err != nil {
		// Silently use defaults - this is normal if no dingo.toml exists
		cfg = config.DefaultConfig()
	}

	// Force in-place generation - go build/run need .go files next to .dingo files
	cfg.Build.OutDir = ""
	opts.OutDir = cfg.Build.OutDir

	// Resolve package paths to .dingo files
	if err := resolveDingoFiles(opts); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return err
	}

	// Validate we have something to compile
	if len(opts.DingoFiles) == 0 && len(opts.GoFiles) == 0 && len(opts.PackagePaths) == 0 {
		err := fmt.Errorf("no source files or packages specified")
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return err
	}

	// Create build UI for mascot animation (unless --no-mascot)
	var buildUI *ui.SimpleBuildUI
	if !opts.NoMascot {
		buildUI = ui.NewSimpleBuildUI()
		buildUI.Start()
		defer buildUI.Stop()

		// Print header
		titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7D56F4"))
		versionStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6C7086"))
		fmt.Printf("%s %s\n\n", titleStyle.Render("🐕 Dingo"), versionStyle.Render("v"+version.Version))

		// Print build info
		if len(opts.DingoFiles) == 1 {
			fmt.Println("Building 1 file")
		} else {
			fmt.Printf("Building %d files\n", len(opts.DingoFiles))
		}
	}

	// Step 1: Transpile .dingo files with UI
	generatedFiles, err := transpileDingoFilesWithUI(opts, buildUI)
	if err != nil {
		if buildUI != nil {
			buildUI.SetStatus(mascot.StateFailed, "Build failed!", "see error above")
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return err
	}

	// Step 2: Invoke go build or go run (with timing and UI)
	var goDuration time.Duration
	var goErr error

	if buildUI != nil {
		buildUI.SetStatus(mascot.StateCompiling, "Running go "+goCmd+"...", "")
		goErr = runWithSpinnerCompile("Go "+goCmd, func() error {
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
			buildUI.SetStatus(mascot.StateFailed, "Go "+goCmd+" failed!", "see error above")
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

// parseCompileArgs separates dingo options from go build args
func parseCompileArgs(args []string) (*CompileOptions, error) {
	opts := &CompileOptions{}

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

	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#5AF78E"))
	timeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6C7086")).Italic(true)
	inputStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#CDD6F4"))
	arrowStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6C7086"))
	outputStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#5AF78E"))

	for _, dingoFile := range opts.DingoFiles {
		// Determine output path
		goFile := computeOutputPath(dingoFile, opts.OutDir)

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
			readErr = runWithSpinnerCompile("Read", func() error {
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

		if buildUI != nil {
			buildUI.SetStatus(mascot.StateCompiling, "Transpiling...", filepath.Base(dingoFile))
			transpileErr = runWithSpinnerCompile("Transpile", func() error {
				start := time.Now()
				var err error
				result, err = transpiler.PureASTTranspileWithMappings(src, dingoFile, true)
				transpileDuration = time.Since(start)
				return err
			})
		} else {
			start := time.Now()
			result, transpileErr = transpiler.PureASTTranspileWithMappings(src, dingoFile, true)
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
			writeErr = runWithSpinnerCompile("Write", func() error {
				start := time.Now()

				// Write .go file
				err := os.WriteFile(goFile, goSource, 0644)
				if err != nil {
					return err
				}

				// Write .dmap file to project root .dmap/ folder
				// Path: examples/03_option/user.dingo -> .dmap/examples/03_option/user.dmap
				dmapPath, dmapPathErr := calculateDmapPath(dingoFile)
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
				// Write .dmap file to project root .dmap/ folder
				// Path: examples/03_option/user.dingo -> .dmap/examples/03_option/user.dmap
				dmapPath, dmapPathErr := calculateDmapPath(dingoFile)
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

// runWithSpinnerCompile runs a function while showing animated mascot spinner
func runWithSpinnerCompile(stepName string, fn func() error) error {
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

	// Draw mascot frame
	drawMascotFrame := func(frame []string) {
		// Separator
		fmt.Println(separatorStyle.Render("────────────────────────────────────────────────────────────"))

		maxLines := len(frame)
		for i := 0; i < maxLines; i++ {
			mascotLine := mascotColor.Render(frame[i])
			statusLine := ""
			if i < len(statusLines) {
				statusLine = "  " + statusLines[i]
			}
			fmt.Printf("%s%s\n", mascotLine, statusLine)
		}
	}

	// Hide cursor
	fmt.Print("\033[?25l")

	// Draw first frame
	drawMascotFrame(mascotFrames[frameIdx])
	mascotHeight := len(mascotFrames[0]) + 1 // +1 for separator

	for {
		select {
		case err := <-done:
			// Move cursor up and clear mascot area
			fmt.Printf("\033[%dA", mascotHeight)
			for i := 0; i < mascotHeight; i++ {
				fmt.Print("\033[2K\n")
			}
			fmt.Printf("\033[%dA", mascotHeight)
			// Show cursor
			fmt.Print("\033[?25h")
			return err
		case <-ticker.C:
			frameIdx = (frameIdx + 1) % len(mascotFrames)
			// Move cursor up to redraw mascot
			fmt.Printf("\033[%dA", mascotHeight)
			drawMascotFrame(mascotFrames[frameIdx])
		}
	}
}

// computeOutputPath determines where to write the .go file
func computeOutputPath(dingoFile, outDir string) string {
	base := strings.TrimSuffix(dingoFile, ".dingo") + ".go"

	if outDir == "" {
		// In-place: .go file next to .dingo file
		return base
	}

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
