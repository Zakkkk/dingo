// Package main implements the Dingo compiler CLI
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/MadAppGang/dingo/pkg/config"
	"github.com/MadAppGang/dingo/pkg/transpiler"
	"github.com/MadAppGang/dingo/pkg/ui"
)

var (
	version = "0.1.0-alpha"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "dingo",
		Short: "Dingo - A meta-language for Go",
		Long: `Dingo is a meta-language that transpiles to idiomatic Go code.
It provides Result/Option types, pattern matching, error propagation,
and other quality-of-life features while maintaining 100% Go ecosystem compatibility.`,
		Version: version,
		SilenceUsage: true, // Don't show usage on errors
		Run: func(cmd *cobra.Command, args []string) {
			// Show colorful help when no command is provided
			ui.PrintDingoHelp(version)
		},
	}

	// Override help flag to use our custom help
	rootCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		ui.PrintDingoHelp(version)
	})

	// Set custom help command
	rootCmd.SetHelpCommand(&cobra.Command{
		Use:   "help [command]",
		Short: "Help about any command",
		Run: func(cmd *cobra.Command, args []string) {
			ui.PrintDingoHelp(version)
		},
	})

	rootCmd.AddCommand(buildCmd())
	rootCmd.AddCommand(runCmd())
	rootCmd.AddCommand(versionCmd())

	if err := rootCmd.Execute(); err != nil {
		// Error is already printed by cobra
		os.Exit(1)
	}
}

func buildCmd() *cobra.Command {
	var (
		output string
		outdir string
		watch  bool
	)

	cmd := &cobra.Command{
		Use:   "build [file.dingo | ./...]",
		Short: "Transpile Dingo source files to Go",
		Long: `Build command transpiles Dingo source files (.dingo) to Go source files (.go).

The transpiler:
1. Parses Dingo source code into AST
2. Transforms Dingo-specific features to Go equivalents
3. Generates idiomatic Go code

Examples:
  dingo build hello.dingo              # Generates hello.go in same directory
  dingo build -o output.go main.dingo  # Custom output file
  dingo build ./...                    # Build all .dingo files in workspace`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBuild(args, output, outdir, watch)
		},
	}

	cmd.Flags().StringVarP(&output, "output", "o", "", "Output file path (single file only)")
	cmd.Flags().StringVarP(&outdir, "outdir", "O", "", "Output directory (mirrors source structure)")
	cmd.Flags().BoolVarP(&watch, "watch", "w", false, "Watch for file changes and rebuild")

	return cmd
}

func runCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run [file.dingo] [-- args...]",
		Short: "Compile and run a Dingo program",
		Long: `Run compiles a Dingo source file and executes it immediately.

This is equivalent to:
  dingo build file.dingo
  go run file.go

The generated .go file is created and then executed. You can pass arguments
to your program after -- (double dash).

Examples:
  dingo run hello.dingo
  dingo run main.dingo -- arg1 arg2 arg3
  dingo run server.dingo -- --port 8080`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			inputFile := args[0]
			programArgs := []string{}

			// If there are args after --, pass them to the program
			if len(args) > 1 {
				programArgs = args[1:]
			}

			return runDingoFile(inputFile, programArgs)
		},
	}

	return cmd
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version number of Dingo",
		Run: func(cmd *cobra.Command, args []string) {
			ui.PrintVersionInfo(version)
		},
	}
}

func runBuild(files []string, output, outdir string, watch bool) error {
	// Validate mutually exclusive flags
	if output != "" && outdir != "" {
		return fmt.Errorf("--output and --outdir are mutually exclusive")
	}

	// Load config (for future use)
	cfg, err := config.Load(nil)
	if err != nil {
		cfg = config.DefaultConfig()
	}
	_ = cfg // Reserved for future config options

	// Expand workspace patterns (./..., ./pkg/...) to actual files
	expandedFiles, err := expandWorkspacePatterns(files)
	if err != nil {
		return fmt.Errorf("failed to expand workspace patterns: %w", err)
	}

	// Create beautiful output handler
	buildUI := ui.NewBuildOutput()

	// Print header
	buildUI.PrintHeader(version)

	// Print build start
	buildUI.PrintBuildStart(len(expandedFiles))

	// Output directory not yet supported
	if outdir != "" {
		return fmt.Errorf("--outdir flag is not currently supported")
	}

	// Build each file
	success := true
	var lastError error
	transpiled := 0

	for _, file := range expandedFiles {
		var outputPath string

		if output != "" {
			// Using single output file mode
			outputPath = output
		}

		if err := buildFile(file, outputPath, buildUI); err != nil {
			success = false
			lastError = err
			buildUI.PrintError(err.Error())
			break
		}
		transpiled++
	}

	// Print summary
	if success {
		summary := fmt.Sprintf("Transpiled: %d files", transpiled)
		buildUI.PrintSummary(true, summary)
		if watch {
			fmt.Println()
			buildUI.PrintInfo("Watch mode not yet implemented")
		}
	} else {
		buildUI.PrintSummary(false, lastError.Error())
		return lastError
	}

	return nil
}

func buildFile(inputPath, outputPath string, buildUI *ui.BuildOutput) error {
	if outputPath == "" {
		// Default: replace .dingo with .go
		if len(inputPath) > 6 && inputPath[len(inputPath)-6:] == ".dingo" {
			outputPath = inputPath[:len(inputPath)-6] + ".go"
		} else {
			outputPath = inputPath + ".go"
		}
	}

	// Print file header
	buildUI.PrintFileStart(inputPath, outputPath)

	// Step 1: Read source
	readStart := time.Now()
	src, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	buildUI.PrintStep(ui.Step{
		Name:     "Read",
		Status:   ui.StepSuccess,
		Duration: time.Since(readStart),
	})

	// Step 2: Transpile using pure AST pipeline
	transpileStart := time.Now()
	goSource, err := transpiler.PureASTTranspile(src, inputPath)
	if err != nil {
		buildUI.PrintStep(ui.Step{
			Name:     "Transpile",
			Status:   ui.StepError,
			Duration: time.Since(transpileStart),
		})
		return fmt.Errorf("transpilation error: %w", err)
	}

	buildUI.PrintStep(ui.Step{
		Name:     "Transpile",
		Status:   ui.StepSuccess,
		Duration: time.Since(transpileStart),
	})

	// Step 3: Write .go file
	writeStart := time.Now()
	if err := os.WriteFile(outputPath, goSource, 0o644); err != nil {
		buildUI.PrintStep(ui.Step{
			Name:     "Write",
			Status:   ui.StepError,
			Duration: time.Since(writeStart),
		})
		return fmt.Errorf("failed to write output: %w", err)
	}

	buildUI.PrintStep(ui.Step{
		Name:     "Write",
		Status:   ui.StepSuccess,
		Duration: time.Since(writeStart),
		Message:  fmt.Sprintf("%d bytes written", len(goSource)),
	})

	return nil
}

func runDingoFile(inputPath string, programArgs []string) error {
	// Create beautiful output
	buildUI := ui.NewBuildOutput()

	// Print minimal header for run mode
	buildUI.PrintHeader(version)
	fmt.Println()

	// Determine output path
	outputPath := ""
	if len(inputPath) > 6 && inputPath[len(inputPath)-6:] == ".dingo" {
		outputPath = inputPath[:len(inputPath)-6] + ".go"
	} else {
		outputPath = inputPath + ".go"
	}

	// Step 1: Build (transpile)
	buildStart := time.Now()

	// Read source
	src, err := os.ReadFile(inputPath)
	if err != nil {
		buildUI.PrintError(fmt.Sprintf("Failed to read %s: %v", inputPath, err))
		return err
	}

	// Transpile using pure AST pipeline
	goCode, err := transpiler.PureASTTranspile(src, inputPath)
	if err != nil {
		buildUI.PrintError(fmt.Sprintf("Transpilation error: %v", err))
		return err
	}

	// Write
	if err := os.WriteFile(outputPath, goCode, 0o644); err != nil {
		buildUI.PrintError(fmt.Sprintf("Failed to write %s: %v", outputPath, err))
		return err
	}

	buildDuration := time.Since(buildStart)

	// Show build status
	fmt.Printf("  📝 Compiled %s → %s (%s)\n",
		filepath.Base(inputPath),
		filepath.Base(outputPath),
		formatDuration(buildDuration))
	fmt.Println()

	// Step 2: Run with go run
	fmt.Println("  🚀 Running...")
	fmt.Println()

	// Prepare go run command
	cmdArgs := []string{"run", outputPath}
	cmdArgs = append(cmdArgs, programArgs...)

	cmd := exec.Command("go", cmdArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	// Run and get exit code
	err = cmd.Run()

	fmt.Println()

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			// Program ran but exited with error
			os.Exit(exitErr.ExitCode())
		}
		// Failed to run
		buildUI.PrintError(fmt.Sprintf("Failed to run: %v", err))
		return err
	}

	return nil
}

func formatDuration(d time.Duration) string {
	switch {
	case d < time.Microsecond:
		return fmt.Sprintf("%dns", d.Nanoseconds())
	case d < time.Millisecond:
		return fmt.Sprintf("%dµs", d.Microseconds())
	case d < time.Second:
		return fmt.Sprintf("%dms", d.Milliseconds())
	default:
		return fmt.Sprintf("%.2fs", d.Seconds())
	}
}

// expandWorkspacePatterns expands workspace patterns like ./... to actual .dingo files
func expandWorkspacePatterns(patterns []string) ([]string, error) {
	var result []string

	for _, pattern := range patterns {
		// Check if this is a workspace pattern
		if isWorkspacePattern(pattern) {
			files, err := expandPattern(pattern)
			if err != nil {
				return nil, err
			}
			result = append(result, files...)
		} else {
			// Regular file path, keep as-is
			result = append(result, pattern)
		}
	}

	return result, nil
}

// isWorkspacePattern checks if a string is a workspace pattern (contains ...)
func isWorkspacePattern(s string) bool {
	return s == "..." || s == "./..." || strings.HasSuffix(s, "/...")
}

// expandPattern expands a workspace pattern to actual .dingo files
func expandPattern(pattern string) ([]string, error) {
	// Get current directory
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get working directory: %w", err)
	}

	// Find workspace root
	// TODO: Re-enable workspace detection
	// root, err := DetectWorkspaceRoot(cwd)
	// if err != nil {
	// 	// Fall back to current directory if no workspace root found
	// 	root = cwd
	// }
	_ = cwd // Suppress unused variable warning

	// Scan workspace for packages
	// TODO: Re-enable workspace scanning
	// ws, err := ScanWorkspace(root)
	// if err != nil {
	// 	return nil, fmt.Errorf("failed to scan workspace: %w", err)
	// }

	// Collect files matching the pattern
	var files []string
	// TODO: Re-enable pattern matching
	// for _, pkg := range ws.Packages {
	// 	if MatchesPattern(pkg.Path, pattern) {
	// 		for _, dingoFile := range pkg.DingoFiles {
	// 			// Convert to absolute path
	// 			absPath := filepath.Join(root, dingoFile)
	// 			files = append(files, absPath)
	// 		}
	// 	}
	// }

	if len(files) == 0 {
		return nil, fmt.Errorf("no .dingo files found matching pattern: %s", pattern)
	}

	return files, nil
}
