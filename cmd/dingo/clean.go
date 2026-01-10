package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/MadAppGang/dingo/pkg/config"
)

// cleanCmd creates the "dingo clean" command
func cleanCmd() *cobra.Command {
	var (
		all     bool
		stray   bool
		verbose bool
		dryRun  bool
	)

	cmd := &cobra.Command{
		Use:   "clean",
		Short: "Remove build artifacts and generated files",
		Long: `Clean removes the shadow build directory and optionally source maps.

By default, only the build directory is removed (default: build/).
Use --all to also remove source maps (.dmap/).
Use --stray to find and remove .go files sitting alongside .dingo sources.

Examples:
  dingo clean              # Remove build directory
  dingo clean --all        # Remove build and source maps
  dingo clean --stray      # Also remove stray generated .go files
  dingo clean --dry-run    # Show what would be removed
  dingo clean --verbose    # Show detailed cleanup progress`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runClean(all, stray, verbose, dryRun)
		},
	}

	cmd.Flags().BoolVarP(&all, "all", "a", false, "Also remove .dmap/ directory")
	cmd.Flags().BoolVarP(&stray, "stray", "s", false, "Remove .go files alongside .dingo sources")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show verbose output")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be deleted without deleting")

	return cmd
}

// runClean executes the clean operation
func runClean(all, stray, verbose, dryRun bool) error {
	// Define styles
	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#5AF78E"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6C7086"))
	warningStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#F7DC6F"))

	// Detect workspace root
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	workspaceRoot, err := DetectWorkspaceRoot(cwd)
	if err != nil {
		// If no workspace root found, use current directory
		workspaceRoot = cwd
	}

	// Load configuration
	cfg, err := config.LoadFromDir(workspaceRoot)
	if err != nil {
		// Use defaults if no config or config invalid
		cfg = config.DefaultConfig()
	}

	// Determine directories to clean
	outDir := cfg.Build.OutDir
	if outDir == "" {
		outDir = "build"
	}
	shadowPath := filepath.Join(workspaceRoot, outDir)
	dmapPath := filepath.Join(workspaceRoot, ".dmap")

	// Track cleanup stats
	var cleanedDirs int
	var cleanedFiles int
	var totalSize int64

	// Clean shadow/build directory
	if err := cleanDirectory(shadowPath, verbose, dryRun, &cleanedDirs, &totalSize, successStyle, dimStyle, warningStyle); err != nil {
		return err
	}

	// Clean .dmap directory if --all flag is set
	if all {
		if err := cleanDirectory(dmapPath, verbose, dryRun, &cleanedDirs, &totalSize, successStyle, dimStyle, warningStyle); err != nil {
			return err
		}
	}

	// Clean stray .go files alongside .dingo sources
	if stray {
		strayFiles, err := findStrayGoFiles(workspaceRoot, outDir)
		if err != nil {
			return fmt.Errorf("failed to find stray files: %w", err)
		}

		for _, goFile := range strayFiles {
			info, err := os.Stat(goFile)
			if err != nil {
				continue
			}
			fileSize := info.Size()

			relPath, _ := filepath.Rel(workspaceRoot, goFile)
			if relPath == "" {
				relPath = goFile
			}

			if dryRun {
				fmt.Printf("Would remove: %s %s\n",
					relPath,
					dimStyle.Render("("+formatSize(fileSize)+")"))
			} else {
				if verbose {
					fmt.Printf("Removing stray: %s\n", relPath)
				}
				if err := os.Remove(goFile); err != nil {
					fmt.Printf("  %s Failed to remove %s: %v\n",
						warningStyle.Render("!"),
						relPath,
						err)
					continue
				}
				if verbose {
					fmt.Printf("  %s Removed %s %s\n",
						successStyle.Render("OK"),
						relPath,
						dimStyle.Render("("+formatSize(fileSize)+")"))
				}
			}
			cleanedFiles++
			totalSize += fileSize
		}
	}

	// Print summary
	printCleanSummaryEx(cleanedDirs, cleanedFiles, totalSize, dryRun, successStyle, dimStyle)

	return nil
}

// cleanDirectory removes a directory and reports progress
func cleanDirectory(path string, verbose, dryRun bool, count *int, size *int64, successStyle, dimStyle, warningStyle lipgloss.Style) error {
	// Check if directory exists
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			if verbose {
				fmt.Printf("  %s %s %s\n",
					dimStyle.Render("-"),
					path,
					dimStyle.Render("(does not exist)"))
			}
			return nil
		}
		return fmt.Errorf("failed to stat %s: %w", path, err)
	}

	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", path)
	}

	// Calculate size
	dirSize, err := calculateDirSize(path)
	if err != nil {
		// Non-fatal: warn but continue
		if verbose {
			fmt.Printf("  %s Could not calculate size for %s: %v\n",
				warningStyle.Render("!"),
				path,
				err)
		}
		dirSize = 0
	}

	// Remove or report
	if dryRun {
		fmt.Printf("Would remove: %s %s\n",
			path,
			dimStyle.Render("("+formatSize(dirSize)+")"))
	} else {
		if verbose {
			fmt.Printf("Removing: %s\n", path)
		}
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("failed to remove %s: %w", path, err)
		}
		if verbose {
			fmt.Printf("  %s Removed %s %s\n",
				successStyle.Render("OK"),
				path,
				dimStyle.Render("("+formatSize(dirSize)+")"))
		}
	}

	*count++
	*size += dirSize
	return nil
}

// calculateDirSize calculates total size of directory contents
func calculateDirSize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}

// formatSize formats bytes in human-readable form
func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// printCleanSummaryEx prints cleanup summary with styled output (extended version)
func printCleanSummaryEx(dirs, files int, size int64, dryRun bool, successStyle, dimStyle lipgloss.Style) {
	if dryRun {
		fmt.Printf("\n%s\n", dimStyle.Render("Dry run - no files deleted"))
		return
	}

	if dirs == 0 && files == 0 {
		fmt.Println(dimStyle.Render("No build artifacts found"))
		return
	}

	var parts []string
	if dirs > 0 {
		suffix := "y"
		if dirs != 1 {
			suffix = "ies"
		}
		parts = append(parts, fmt.Sprintf("%d director%s", dirs, suffix))
	}
	if files > 0 {
		suffix := ""
		if files != 1 {
			suffix = "s"
		}
		parts = append(parts, fmt.Sprintf("%d stray file%s", files, suffix))
	}

	fmt.Printf("\n%s Cleaned %s (%s freed)\n",
		successStyle.Render("OK"),
		strings.Join(parts, ", "),
		formatSize(size))
}

// findStrayGoFiles finds .go files that sit alongside .dingo sources
// These are likely generated files that weren't placed in the build directory
func findStrayGoFiles(root, outDir string) ([]string, error) {
	var strayFiles []string

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors, continue walking
		}

		// Skip directories we don't want to scan
		if info.IsDir() {
			name := info.Name()
			// Skip hidden directories, build directory, vendor, etc.
			if strings.HasPrefix(name, ".") || name == outDir || name == "vendor" || name == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}

		// Only look at .go files
		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		// Check if there's a corresponding .dingo file
		dingoPath := strings.TrimSuffix(path, ".go") + ".dingo"
		if _, err := os.Stat(dingoPath); err == nil {
			// .dingo file exists - this .go file is likely generated
			// Verify it looks like a generated file by checking the header
			if isGeneratedGoFile(path) {
				strayFiles = append(strayFiles, path)
			}
		}

		return nil
	})

	return strayFiles, err
}

// isGeneratedGoFile checks if a .go file appears to be generated by Dingo
func isGeneratedGoFile(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	// Read first few lines looking for generation markers
	scanner := bufio.NewScanner(f)
	linesChecked := 0
	for scanner.Scan() && linesChecked < 10 {
		line := scanner.Text()
		// Look for common generated file markers
		if strings.Contains(line, "Code generated") ||
			strings.Contains(line, "DO NOT EDIT") ||
			strings.Contains(line, "//line") && strings.Contains(line, ".dingo:") {
			return true
		}
		linesChecked++
	}

	return false
}
