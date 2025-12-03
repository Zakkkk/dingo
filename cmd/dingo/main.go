// Package main implements the Dingo compiler CLI
package main

import (
	"encoding/json"
	"fmt"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/MadAppGang/dingo/pkg/build"
	"github.com/MadAppGang/dingo/pkg/config"
	"github.com/MadAppGang/dingo/pkg/generator"
	"github.com/MadAppGang/dingo/pkg/parser"
	"github.com/MadAppGang/dingo/pkg/plugin"
	"github.com/MadAppGang/dingo/pkg/plugin/builtin"
	"github.com/MadAppGang/dingo/pkg/preprocessor"
	"github.com/MadAppGang/dingo/pkg/sourcemap"
	"github.com/MadAppGang/dingo/pkg/typeloader"
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
		output               string
		outdir               string
		watch                bool
		multiValueReturnMode string
	)

	cmd := &cobra.Command{
		Use:   "build [file.dingo | ./...]",
		Short: "Transpile Dingo source files to Go",
		Long: `Build command transpiles Dingo source files (.dingo) to Go source files (.go).

The transpiler:
1. Parses Dingo source code into AST
2. Transforms Dingo-specific features to Go equivalents
3. Generates idiomatic Go code with source maps

When using --outdir (or configured in dingo.toml), all .go files are automatically
copied to create a complete, buildable output directory.

Examples:
  dingo build hello.dingo              # Generates hello.go in same directory
  dingo build -o output.go main.dingo  # Custom output file
  dingo build --outdir build/ ./...    # Output all to build/ (mirrors structure)
  dingo build -O build/ ./...          # Short form

Config file (dingo.toml):
  [build]
  outdir = "build/"                    # Sets default output directory`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBuild(args, output, outdir, watch, multiValueReturnMode)
		},
	}

	cmd.Flags().StringVarP(&output, "output", "o", "", "Output file path (single file only)")
	cmd.Flags().StringVarP(&outdir, "outdir", "O", "", "Output directory (mirrors source structure)")
	cmd.Flags().BoolVarP(&watch, "watch", "w", false, "Watch for file changes and rebuild")
	cmd.Flags().StringVar(&multiValueReturnMode, "multi-value-return", "full",
		"Multi-value return propagation mode: 'full' (default, supports (A,B,error)) or 'single' (restricts to (T,error))")

	return cmd
}

func runCmd() *cobra.Command {
	var multiValueReturnMode string

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
  dingo run server.dingo -- --port 8080
  dingo run --multi-value-return=single file.dingo`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			inputFile := args[0]
			programArgs := []string{}

			// If there are args after --, pass them to the program
			if len(args) > 1 {
				programArgs = args[1:]
			}

			return runDingoFile(inputFile, programArgs, multiValueReturnMode)
		},
	}

	cmd.Flags().StringVar(&multiValueReturnMode, "multi-value-return", "full",
		"Multi-value return propagation mode: 'full' (default, supports (A,B,error)) or 'single' (restricts to (T,error))")

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

func runBuild(files []string, output, outdir string, watch bool, _ string) error {
	// Validate mutually exclusive flags
	if output != "" && outdir != "" {
		return fmt.Errorf("--output and --outdir are mutually exclusive")
	}

	// Load main Dingo configuration (C1: Config Integration)
	//
	// Priority order:
	// 1. dingo.toml in current directory
	// 2. ~/.dingo/config.toml
	// 3. Built-in defaults
	cfg, err := config.Load(nil)
	if err != nil {
		// Non-fatal: fall back to defaults and warn
		cfg = config.DefaultConfig()
		fmt.Fprintf(os.Stderr, "Warning: config load failed, using defaults: %v\n", err)
	}

	// CLI flag overrides config for outdir
	effectiveOutdir := outdir
	if effectiveOutdir == "" && cfg.Build.OutDir != "" {
		effectiveOutdir = cfg.Build.OutDir
	}

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

	// Setup output resolver if outdir is specified
	var resolver *build.OutputResolver
	var sourceRoot string

	if effectiveOutdir != "" {
		// Determine source root from workspace
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get working directory: %w", err)
		}
		sourceRoot, err = DetectWorkspaceRoot(cwd)
		if err != nil {
			// Fall back to current directory if no workspace root found
			sourceRoot = cwd
		}

		resolver, err = build.NewOutputResolver(sourceRoot, effectiveOutdir)
		if err != nil {
			return fmt.Errorf("failed to create output resolver: %w", err)
		}
	}

	// Create BuildCache for this build invocation
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}
	buildCache := typeloader.NewBuildCache(typeloader.LoaderConfig{
		WorkingDir: cwd,
		FailFast:   true,
	})
	defer buildCache.Clear()

	// Build each file
	success := true
	var lastError error
	transpiled := 0
	skipped := 0

	for _, file := range expandedFiles {
		var outputPath string

		if resolver != nil {
			// Using output directory mode
			outputPath, err = resolver.ResolvePath(file)
			if err != nil {
				buildUI.PrintError(err.Error())
				success = false
				lastError = err
				break
			}

			// Incremental: check if needs rebuild
			needsRebuild, err := resolver.NeedsRebuild(file, outputPath)
			if err != nil {
				buildUI.PrintError(err.Error())
				success = false
				lastError = err
				break
			}

			if !needsRebuild {
				skipped++
				continue
			}

			// Ensure output directory exists
			if err := resolver.EnsureDir(outputPath); err != nil {
				buildUI.PrintError(err.Error())
				success = false
				lastError = err
				break
			}
		} else if output != "" {
			// Using single output file mode
			outputPath = output
		}

		if err := buildFile(file, outputPath, buildUI, cfg, buildCache); err != nil {
			success = false
			lastError = err
			buildUI.PrintError(err.Error())
			break
		}
		transpiled++
	}

	// Auto-copy pure .go files when using outdir
	copied := 0
	if success && resolver != nil {
		ws, err := ScanWorkspace(sourceRoot)
		if err == nil {
			for _, pkg := range ws.Packages {
				for _, goFile := range pkg.GoFiles {
					absGoFile := filepath.Join(sourceRoot, goFile)
					// Skip generated files (have corresponding .dingo)
					dingoFile := strings.TrimSuffix(goFile, ".go") + ".dingo"
					hasDingoSource := false
					for _, df := range pkg.DingoFiles {
						if df == dingoFile {
							hasDingoSource = true
							break
						}
					}
					if !hasDingoSource {
						// Resolve output path
						outputGoPath, err := resolver.ResolveGoFile(absGoFile)
						if err != nil {
							buildUI.PrintInfo(fmt.Sprintf("Warning: failed to resolve output path for %s: %v", goFile, err))
							continue
						}
						// Copy file (incremental)
						wasCopied, err := resolver.CopyGoFile(absGoFile, outputGoPath)
						if err != nil {
							buildUI.PrintInfo(fmt.Sprintf("Warning: failed to copy %s: %v", goFile, err))
						} else if wasCopied {
							copied++
						}
					}
				}
			}
		}
	}

	// Print summary
	if success {
		summary := ""
		if resolver != nil {
			summary = fmt.Sprintf("Transpiled: %d, Copied: %d, Skipped (up-to-date): %d", transpiled, copied, skipped)
		}
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

func buildFile(inputPath, outputPath string, buildUI *ui.BuildOutput, cfg *config.Config, buildCache *typeloader.BuildCache) error {
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
	src, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Step 2: Preprocess (with dynamic type loading via BuildCache)
	prepStart := time.Now()
	var goSource string
	var metadata []preprocessor.TransformMetadata // Phase 3: Collect metadata for PostASTGenerator
	var prepDuration time.Duration

	// Use NewWithTypeLoading if BuildCache is available, otherwise fall back to legacy behavior
	if buildCache != nil {
		// Note: WorkingDir is handled internally by typeloader using current directory
		prep, err := preprocessor.NewWithTypeLoading(src, cfg, buildCache)
		if err != nil {
			buildUI.PrintStep(ui.Step{
				Name:     "Preprocess",
				Status:   ui.StepError,
				Duration: time.Since(prepStart),
			})
			return fmt.Errorf("type loading failed: %w", err)
		}

		var legacyMap *preprocessor.SourceMap
		goSource, legacyMap, metadata, err = prep.ProcessWithMetadata()
		_ = legacyMap // Discard legacy map - Phase 3 uses PostASTGenerator
		prepDuration = time.Since(prepStart)
		if err != nil {
			buildUI.PrintStep(ui.Step{
				Name:     "Preprocess",
				Status:   ui.StepError,
				Duration: prepDuration,
			})
			return fmt.Errorf("preprocessing error: %w", err)
		}
	} else {
		// Fall back to legacy behavior (for backwards compatibility)
		// For single-file builds, create a simple cache for just this file
		pkgDir := filepath.Dir(inputPath)
		cache := preprocessor.NewFunctionExclusionCache(pkgDir)
		err = cache.ScanPackage([]string{inputPath}) // Only scan the file being built
		if err != nil {
			// Fall back to no cache if scanning fails (e.g., syntax errors in .dingo file)
			prep := preprocessor.NewWithMainConfig(src, cfg)
			var legacyMap *preprocessor.SourceMap
			goSource, legacyMap, metadata, err = prep.ProcessWithMetadata()
			_ = legacyMap // Discard legacy map - Phase 3 uses PostASTGenerator
			prepDuration = time.Since(prepStart)
			if err != nil {
				buildUI.PrintStep(ui.Step{
					Name:     "Preprocess",
					Status:   ui.StepError,
					Duration: prepDuration,
				})
				return fmt.Errorf("preprocessing error: %w", err)
			}
		} else {
			// Cache scan successful, use preprocessor with unqualified import inference
			prep := preprocessor.NewWithCache(src, cache)
			var legacyMap *preprocessor.SourceMap
			goSource, legacyMap, metadata, err = prep.ProcessWithMetadata()
			_ = legacyMap // Discard legacy map - Phase 3 uses PostASTGenerator
			prepDuration = time.Since(prepStart)
			if err != nil {
				buildUI.PrintStep(ui.Step{
					Name:     "Preprocess",
					Status:   ui.StepError,
					Duration: prepDuration,
				})
				return fmt.Errorf("preprocessing error: %w", err)
			}
		}
	}

	buildUI.PrintStep(ui.Step{
		Name:     "Preprocess",
		Status:   ui.StepSuccess,
		Duration: prepDuration,
	})

	// Step 3: Parse preprocessed Go
	parseStart := time.Now()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, inputPath, []byte(goSource), parser.ParseComments)
	parseDuration := time.Since(parseStart)

	if err != nil {
		buildUI.PrintStep(ui.Step{
			Name:     "Parse",
			Status:   ui.StepError,
			Duration: parseDuration,
		})
		return fmt.Errorf("parse error: %w", err)
	}

	buildUI.PrintStep(ui.Step{
		Name:     "Parse",
		Status:   ui.StepSuccess,
		Duration: parseDuration,
	})

	// DEBUG: Check if markers are in AST
	// Step 2: Setup plugins
	registry, err := builtin.NewDefaultRegistry()
	if err != nil {
		buildUI.PrintStep(ui.Step{
			Name:   "Setup",
			Status: ui.StepError,
		})
		return fmt.Errorf("failed to setup plugins: %w", err)
	}

	// Step 3: Generate with plugins
	genStart := time.Now()
	logger := plugin.NewNoOpLogger() // Silent logger for CLI
	gen, err := generator.NewWithPlugins(fset, registry, logger)
	if err != nil {
		buildUI.PrintStep(ui.Step{
			Name:     "Generate",
			Status:   ui.StepError,
			Duration: time.Since(genStart),
		})
		return fmt.Errorf("failed to create generator: %w", err)
	}

	outputCode, err := gen.Generate(file)
	genDuration := time.Since(genStart)

	if err != nil {
		buildUI.PrintStep(ui.Step{
			Name:     "Generate",
			Status:   ui.StepError,
			Duration: genDuration,
		})
		return fmt.Errorf("generation error: %w", err)
	}

	buildUI.PrintStep(ui.Step{
		Name:     "Generate",
		Status:   ui.StepSuccess,
		Duration: genDuration,
	})

	// Step 4: Write .go file
	writeStart := time.Now()
	if err := os.WriteFile(outputPath, outputCode, 0o644); err != nil {
		writeDuration := time.Since(writeStart)
		buildUI.PrintStep(ui.Step{
			Name:     "Write",
			Status:   ui.StepError,
			Duration: writeDuration,
		})
		return fmt.Errorf("failed to write output: %w", err)
	}

	// Phase 3: Generate source map AFTER go/printer using PostASTGenerator
	// CRITICAL: Parse the WRITTEN .go file to get accurate FileSet positions
	// (The in-memory version may have different line numbers after go/printer formatting)
	sourceMapPath := outputPath + ".map"
	sourceMap, err := sourcemap.GenerateFromFiles(inputPath, outputPath, metadata)
	if err != nil {
		// Non-fatal: just log warning
		buildUI.PrintInfo(fmt.Sprintf("Warning: source map generation failed: %v", err))
	} else {
		// Write source map
		sourceMapJSON, _ := json.MarshalIndent(sourceMap, "", "  ")
		if err := os.WriteFile(sourceMapPath, sourceMapJSON, 0o644); err != nil {
			// Non-fatal: just log warning
			buildUI.PrintInfo(fmt.Sprintf("Warning: failed to write source map: %v", err))
		}
	}

	writeDuration := time.Since(writeStart)

	buildUI.PrintStep(ui.Step{
		Name:     "Write",
		Status:   ui.StepSuccess,
		Duration: writeDuration,
		Message:  fmt.Sprintf("%d bytes written", len(outputCode)),
	})

	return nil
}

func runDingoFile(inputPath string, programArgs []string, _ string) error {
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

	// Load main Dingo configuration (C1: Config Integration)
	cfg, err := config.Load(nil)
	if err != nil {
		// Non-fatal: fall back to defaults
		cfg = config.DefaultConfig()
	}

	// Read source
	src, err := os.ReadFile(inputPath)
	if err != nil {
		buildUI.PrintError(fmt.Sprintf("Failed to read %s: %v", inputPath, err))
		return err
	}

	// Preprocess (with main config + package context for unqualified imports)
	var goSource string
	pkgDir := filepath.Dir(inputPath)
	pkgCtx, err := preprocessor.NewPackageContext(pkgDir, preprocessor.DefaultBuildOptions())
	if err != nil {
		// Fall back to no cache if package context fails
		prep := preprocessor.NewWithMainConfig(src, cfg)
		goSource, _, err = prep.Process()
		if err != nil {
			buildUI.PrintError(fmt.Sprintf("Preprocessing error: %v", err))
			return err
		}
	} else {
		prep := preprocessor.NewWithCache(src, pkgCtx.GetCache())
		goSource, _, err = prep.Process()
		if err != nil {
			buildUI.PrintError(fmt.Sprintf("Preprocessing error: %v", err))
			return err
		}
	}

	// Parse
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, inputPath, []byte(goSource), parser.ParseComments)
	if err != nil {
		buildUI.PrintError(fmt.Sprintf("Parse error: %v", err))
		return err
	}

	// Generate with plugins
	registry, err := builtin.NewDefaultRegistry()
	if err != nil {
		buildUI.PrintError(fmt.Sprintf("Failed to setup plugins: %v", err))
		return err
	}

	logger := plugin.NewNoOpLogger()
	gen, err := generator.NewWithPlugins(fset, registry, logger)
	if err != nil {
		buildUI.PrintError(fmt.Sprintf("Failed to create generator: %v", err))
		return err
	}

	goCode, err := gen.Generate(file)
	if err != nil {
		buildUI.PrintError(fmt.Sprintf("Generation error: %v", err))
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
	root, err := DetectWorkspaceRoot(cwd)
	if err != nil {
		// Fall back to current directory if no workspace root found
		root = cwd
	}

	// Scan workspace for packages
	ws, err := ScanWorkspace(root)
	if err != nil {
		return nil, fmt.Errorf("failed to scan workspace: %w", err)
	}

	// Collect files matching the pattern
	var files []string
	for _, pkg := range ws.Packages {
		if MatchesPattern(pkg.Path, pattern) {
			for _, dingoFile := range pkg.DingoFiles {
				// Convert to absolute path
				absPath := filepath.Join(root, dingoFile)
				files = append(files, absPath)
			}
		}
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no .dingo files found matching pattern: %s", pattern)
	}

	return files, nil
}
