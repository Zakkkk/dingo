package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/MadAppGang/dingo/pkg/format"
)

// fmtCmd creates the "dingo fmt" subcommand
func fmtCmd() *cobra.Command {
	var (
		writeBack bool
		check     bool
	)

	cmd := &cobra.Command{
		Use:   "fmt [files/directories...]",
		Short: "Format Dingo source files",
		Long: `Format formats Dingo source files using the Dingo formatter.

By default, formatted output is written to stdout.
Use -w to write back to files in-place.
Use --check to verify files are formatted without modifying them.

Configuration is loaded from dingo.toml if present.

Examples:
  dingo fmt main.dingo                   # Format to stdout
  dingo fmt -w main.dingo                # Format in-place
  dingo fmt -w ./...                     # Format all .dingo files in workspace
  dingo fmt --check main.dingo           # Check if file needs formatting`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runFormat(args, writeBack, check)
		},
	}

	cmd.Flags().BoolVarP(&writeBack, "write", "w", false, "Write result to source file instead of stdout")
	cmd.Flags().BoolVar(&check, "check", false, "Exit with non-zero status if files are not formatted")

	return cmd
}

// runFormat executes the formatter on the specified files/directories
func runFormat(paths []string, writeBack, check bool) error {
	// Validate flags
	if writeBack && check {
		return fmt.Errorf("--write and --check are mutually exclusive")
	}

	// Load configuration
	cfg := format.DefaultConfig()
	// TODO: Load from dingo.toml when format config is fully implemented

	// Create formatter
	formatter := format.New(cfg)

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
			fmt.Fprintf(os.Stderr, "%s Failed to format %s: %v\n",
				errorStyle.Render("✗"),
				file,
				err)
			continue
		}

		if check {
			// Check mode: compare formatted output with original
			if string(formatted) != string(src) {
				needsFormatting = append(needsFormatting, file)
				fmt.Printf("%s %s\n",
					errorStyle.Render("✗"),
					fileStyle.Render(file))
			} else {
				fmt.Printf("%s %s\n",
					successStyle.Render("✓"),
					fileStyle.Render(file))
			}
		} else if writeBack {
			// Write mode: save formatted output to file
			if string(formatted) != string(src) {
				// Only write if content changed
				if err := os.WriteFile(file, formatted, 0644); err != nil {
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

	// In check mode, exit non-zero if any files need formatting
	if check && len(needsFormatting) > 0 {
		fmt.Fprintf(os.Stderr, "\n%d file(s) need formatting\n", len(needsFormatting))
		os.Exit(1)
	}

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
