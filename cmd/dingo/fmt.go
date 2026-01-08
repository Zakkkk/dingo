package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/MadAppGang/dingo/pkg/format"
	"github.com/charmbracelet/lipgloss"
	"github.com/pmezard/go-difflib/difflib"
	"github.com/spf13/cobra"
)

// fmtCmd creates the "dingo fmt" subcommand
func fmtCmd() *cobra.Command {
	var (
		writeBack bool
		check     bool
		showDiff  bool
		list      bool
	)

	cmd := &cobra.Command{
		Use:   "fmt [files/directories...]",
		Short: "Format Dingo source files",
		Long: `Format formats Dingo source files using the Dingo formatter.

By default, formatted output is written to stdout.
Use -w to write back to files in-place.
Use --check to verify files are formatted without modifying them.
Use --diff to show unified diff of changes.

Reads from stdin when no files are specified or when "-" is given.

Configuration is loaded from dingo.toml if present.

Examples:
  dingo fmt main.dingo                   # Format to stdout
  dingo fmt -w main.dingo                # Format in-place
  dingo fmt -w ./...                     # Format all .dingo files in workspace
  dingo fmt --check main.dingo           # Check if file needs formatting
  dingo fmt --diff main.dingo            # Show unified diff
  dingo fmt -l main.dingo                # List files needing formatting
  echo "let x=1" | dingo fmt             # Format from stdin
  cat file.dingo | dingo fmt -           # Format stdin explicitly`,
		Args: cobra.ArbitraryArgs, // Allow zero args for stdin mode
		RunE: func(cmd *cobra.Command, args []string) error {
			return runFormat(args, writeBack, check, showDiff, list)
		},
	}

	cmd.Flags().BoolVarP(&writeBack, "write", "w", false, "Write result to source file instead of stdout")
	cmd.Flags().BoolVar(&check, "check", false, "Exit with non-zero status if files are not formatted")
	cmd.Flags().BoolVarP(&showDiff, "diff", "d", false, "Display diffs instead of rewriting files")
	cmd.Flags().BoolVarP(&list, "list", "l", false, "List files whose formatting differs")

	return cmd
}

// runFormat executes the formatter on the specified files/directories
func runFormat(paths []string, writeBack, check, showDiff, list bool) error {
	// Validate flags - mutually exclusive options
	exclusiveFlags := 0
	if writeBack {
		exclusiveFlags++
	}
	if check {
		exclusiveFlags++
	}
	if showDiff {
		exclusiveFlags++
	}
	if list {
		exclusiveFlags++
	}
	if exclusiveFlags > 1 {
		return fmt.Errorf("--write, --check, --diff, and --list are mutually exclusive")
	}

	// Load configuration from dingo.toml
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	cfg, err := format.LoadConfig(cwd)
	if err != nil {
		// Log warning but continue with defaults
		fmt.Fprintf(os.Stderr, "Warning: failed to load format config: %v\n", err)
		cfg = format.DefaultConfig()
	}

	// Create formatter
	formatter := format.New(cfg)

	// Check for stdin mode: no paths or single "-" argument
	if len(paths) == 0 || (len(paths) == 1 && paths[0] == "-") {
		return runFormatStdin(formatter, showDiff)
	}

	// Expand paths to find all .dingo files
	dingoFiles, err := expandPathsForFormat(paths)
	if err != nil {
		return fmt.Errorf("failed to find files: %w", err)
	}

	if len(dingoFiles) == 0 {
		return fmt.Errorf("no .dingo files found in specified paths")
	}

	// Track formatting results
	needsFormatting := []string{}
	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#5AF78E"))
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#F38BA8"))
	fileStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#CDD6F4"))

	// Format each file
	for _, file := range dingoFiles {
		src, err := os.ReadFile(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s Failed to read %s: %v\n",
				errorStyle.Render("✗"),
				file,
				err)
			continue
		}

		formatted, err := formatter.Format(src)
		if err != nil {
			// C4: Report syntax errors in check mode instead of silent suppression
			fmt.Fprintf(os.Stderr, "Warning: syntax error in %s: %v\n", file, err)
			formatted = src
		}

		// I7: Use bytes.Equal to avoid unnecessary string allocations
		changed := !bytes.Equal(src, formatted)

		if check {
			// Check mode: compare formatted output with original
			if changed {
				needsFormatting = append(needsFormatting, file)
				fmt.Printf("%s %s\n",
					errorStyle.Render("✗"),
					fileStyle.Render(file))
			} else {
				fmt.Printf("%s %s\n",
					successStyle.Render("✓"),
					fileStyle.Render(file))
			}
		} else if list {
			// List mode: only print files that need formatting
			if changed {
				needsFormatting = append(needsFormatting, file)
				fmt.Println(file)
			}
		} else if showDiff {
			// Diff mode: show unified diff
			if changed {
				diff := difflib.UnifiedDiff{
					A:        difflib.SplitLines(string(src)),
					B:        difflib.SplitLines(string(formatted)),
					FromFile: file,
					ToFile:   file,
					Context:  3,
				}
				// I5: Check diff generation errors instead of silently ignoring
				text, diffErr := difflib.GetUnifiedDiffString(diff)
				if diffErr != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to generate diff for %s: %v\n", file, diffErr)
					continue
				}
				fmt.Print(text)
			}
		} else if writeBack {
			// Write mode: save formatted output to file
			if changed {
				// I4: Preserve original file permissions instead of hardcoded 0644
				info, statErr := os.Stat(file)
				if statErr != nil {
					fmt.Fprintf(os.Stderr, "%s Failed to stat %s: %v\n",
						errorStyle.Render("✗"),
						file,
						statErr)
					continue
				}
				if err := os.WriteFile(file, formatted, info.Mode().Perm()); err != nil {
					fmt.Fprintf(os.Stderr, "%s Failed to write %s: %v\n",
						errorStyle.Render("✗"),
						file,
						err)
					continue
				}
				fmt.Printf("%s %s\n",
					successStyle.Render("✓"),
					fileStyle.Render(file))
			} else {
				// File already formatted
				fmt.Printf("%s %s (already formatted)\n",
					successStyle.Render("✓"),
					fileStyle.Render(file))
			}
		} else {
			// stdout mode: output formatted code
			if len(dingoFiles) > 1 {
				// Multiple files: show file header
				fmt.Printf("=== %s ===\n", file)
			}
			os.Stdout.Write(formatted)
			if len(dingoFiles) > 1 {
				fmt.Println() // Blank line between files
			}
		}
	}

	// C2: Return error instead of os.Exit(1) to enable testing and proper cleanup
	if (check || list) && len(needsFormatting) > 0 {
		if check {
			fmt.Fprintf(os.Stderr, "\n%d file(s) need formatting\n", len(needsFormatting))
		}
		return fmt.Errorf("%d file(s) need formatting", len(needsFormatting))
	}

	return nil
}

// runFormatStdin reads from stdin and writes formatted output to stdout
func runFormatStdin(formatter *format.Formatter, showDiff bool) error {
	src, err := io.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("failed to read stdin: %w", err)
	}

	formatted, err := formatter.Format(src)
	if err != nil {
		// Syntax error: return original content unchanged (per user decision)
		os.Stdout.Write(src)
		return nil
	}

	if showDiff {
		// Diff mode for stdin
		// I7: Use bytes.Equal instead of string comparison
		if !bytes.Equal(formatted, src) {
			diff := difflib.UnifiedDiff{
				A:        difflib.SplitLines(string(src)),
				B:        difflib.SplitLines(string(formatted)),
				FromFile: "<stdin>",
				ToFile:   "<stdin>",
				Context:  3,
			}
			// I5: Check diff generation errors instead of silently ignoring
			text, diffErr := difflib.GetUnifiedDiffString(diff)
			if diffErr != nil {
				return fmt.Errorf("failed to generate diff: %w", diffErr)
			}
			fmt.Print(text)
		}
		return nil
	}

	os.Stdout.Write(formatted)
	return nil
}

// expandPathsForFormat expands file paths and directory paths to .dingo files
func expandPathsForFormat(paths []string) ([]string, error) {
	var dingoFiles []string
	seen := make(map[string]bool) // Deduplicate files

	for _, path := range paths {
		// Handle workspace patterns (./..., ./pkg/...)
		if isWorkspacePattern(path) {
			files, err := expandWorkspacePatternForFormat(path)
			if err != nil {
				return nil, err
			}
			for _, f := range files {
				if !seen[f] {
					dingoFiles = append(dingoFiles, f)
					seen[f] = true
				}
			}
			continue
		}

		// Get absolute path
		absPath, err := filepath.Abs(path)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve %s: %w", path, err)
		}

		// Check if path exists
		info, err := os.Stat(absPath)
		if err != nil {
			return nil, fmt.Errorf("failed to access %s: %w", path, err)
		}

		if info.IsDir() {
			// Scan directory for .dingo files (recursively)
			files, err := scanDirForDingo(absPath)
			if err != nil {
				return nil, err
			}
			for _, f := range files {
				if !seen[f] {
					dingoFiles = append(dingoFiles, f)
					seen[f] = true
				}
			}
		} else {
			// Single file
			if strings.HasSuffix(absPath, ".dingo") {
				if !seen[absPath] {
					dingoFiles = append(dingoFiles, absPath)
					seen[absPath] = true
				}
			}
		}
	}

	return dingoFiles, nil
}

// expandWorkspacePatternForFormat expands workspace patterns like ./... to .dingo files
func expandWorkspacePatternForFormat(pattern string) ([]string, error) {
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

	// Determine base directory for pattern
	baseDir := root
	if pattern != "./..." && pattern != "..." {
		// Pattern like ./pkg/...
		prefix := strings.TrimSuffix(pattern, "/...")
		prefix = strings.TrimPrefix(prefix, "./")
		if prefix != "" {
			baseDir = filepath.Join(root, prefix)
		}
	}

	// Scan for .dingo files
	return scanDirForDingo(baseDir)
}
