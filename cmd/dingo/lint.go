package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/spf13/cobra"
	"github.com/MadAppGang/dingo/pkg/lint"
	"github.com/MadAppGang/dingo/pkg/lint/analyzer"
)

// lintCmd creates the "dingo lint" subcommand
func lintCmd() *cobra.Command {
	var (
		jsonOutput bool
	)

	cmd := &cobra.Command{
		Use:   "lint [files/directories...]",
		Short: "Run Dingo linter on source files",
		Long: `Lint runs all enabled analyzers on Dingo source files and reports diagnostics.

The linter runs in advisory mode - warnings are displayed but never block builds.
All diagnostics are treated as warnings, not errors.

Configuration is loaded from dingo.toml if present.

Examples:
  dingo lint main.dingo                  # Lint single file
  dingo lint ./...                       # Lint all .dingo files in workspace
  dingo lint ./pkg/                      # Lint all .dingo files in directory
  dingo lint --json main.dingo           # Output diagnostics as JSON`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLint(args, jsonOutput)
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output diagnostics in JSON format")

	return cmd
}

// runLint executes the linter on the specified files/directories
func runLint(paths []string, jsonOutput bool) error {
	// Load configuration
	cfg := lint.LoadConfig()

	// Expand paths to find all .dingo files
	dingoFiles, err := expandPathsForLint(paths)
	if err != nil {
		return fmt.Errorf("failed to find files: %w", err)
	}

	if len(dingoFiles) == 0 {
		return fmt.Errorf("no .dingo files found in specified paths")
	}

	// Create linter runner
	runner := lint.NewRunner(cfg)

	// Collect all diagnostics and track failed files
	var allDiagnostics []analyzer.Diagnostic
	var failedFiles []string

	for _, file := range dingoFiles {
		src, err := os.ReadFile(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to read %s: %v\n", file, err)
			failedFiles = append(failedFiles, file)
			continue
		}

		diags, err := runner.Run(file, src)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to lint %s: %v\n", file, err)
			failedFiles = append(failedFiles, file)
			continue
		}

		allDiagnostics = append(allDiagnostics, diags...)
	}

	// Sort diagnostics by position
	lint.SortDiagnostics(allDiagnostics)

	// Output diagnostics
	if jsonOutput {
		outputDiagnosticsJSON(allDiagnostics)
	} else {
		outputDiagnosticsTerminal(allDiagnostics)
	}

	// Report failed files if any
	if len(failedFiles) > 0 {
		fmt.Fprintf(os.Stderr, "\n⚠ %d file(s) could not be linted due to errors:\n", len(failedFiles))
		for _, f := range failedFiles {
			fmt.Fprintf(os.Stderr, "  - %s\n", f)
		}
	}

	// Advisory mode: always exit 0 (warnings don't fail)
	return nil
}

// outputDiagnosticsTerminal outputs diagnostics in colored terminal format
func outputDiagnosticsTerminal(diagnostics []analyzer.Diagnostic) {
	if len(diagnostics) == 0 {
		successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#5AF78E")).Bold(true)
		fmt.Println(successStyle.Render("✓ No issues found"))
		return
	}

	// Check if output is a TTY for colored output
	isTTY := termenv.DefaultOutput().TTY() != nil

	// Define color styles
	var (
		hintStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#6C7086"))
		infoStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#89B4FA"))
		warningStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FAB387"))
		fileStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#CDD6F4")).Bold(true)
		codeStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#6C7086"))
	)

	// Group diagnostics by file
	grouped := lint.GroupByFile(diagnostics)

	for filename, fileDiags := range grouped {
		// Print file header
		if isTTY {
			fmt.Println(fileStyle.Render(filename + ":"))
		} else {
			fmt.Printf("%s:\n", filename)
		}

		// Print each diagnostic
		for _, diag := range fileDiags {
			severityStr := diag.Severity.String()
			var formattedLine string

			if isTTY {
				// Colored output
				var severityStyled string
				switch diag.Severity {
				case analyzer.SeverityHint:
					severityStyled = hintStyle.Render(severityStr)
				case analyzer.SeverityInfo:
					severityStyled = infoStyle.Render(severityStr)
				case analyzer.SeverityWarning:
					severityStyled = warningStyle.Render(severityStr)
				}

				formattedLine = fmt.Sprintf("  %d:%d: %s%s: %s",
					diag.Pos.Line,
					diag.Pos.Column,
					severityStyled,
					codeStyle.Render("["+diag.Code+"]"),
					diag.Message,
				)
			} else {
				// Plain text output
				formattedLine = fmt.Sprintf("  %d:%d: %s[%s]: %s",
					diag.Pos.Line,
					diag.Pos.Column,
					severityStr,
					diag.Code,
					diag.Message,
				)
			}

			fmt.Println(formattedLine)
		}

		fmt.Println() // Blank line between files
	}

	// Print summary
	hintCount := 0
	infoCount := 0
	warningCount := 0

	for _, diag := range diagnostics {
		switch diag.Severity {
		case analyzer.SeverityHint:
			hintCount++
		case analyzer.SeverityInfo:
			infoCount++
		case analyzer.SeverityWarning:
			warningCount++
		}
	}

	summaryParts := []string{}
	if hintCount > 0 {
		summaryParts = append(summaryParts, fmt.Sprintf("%d hint(s)", hintCount))
	}
	if infoCount > 0 {
		summaryParts = append(summaryParts, fmt.Sprintf("%d info", infoCount))
	}
	if warningCount > 0 {
		summaryParts = append(summaryParts, fmt.Sprintf("%d warning(s)", warningCount))
	}

	summary := strings.Join(summaryParts, ", ")
	if isTTY {
		summaryStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6C7086")).Italic(true)
		fmt.Println(summaryStyle.Render(fmt.Sprintf("Found %s", summary)))
	} else {
		fmt.Printf("Found %s\n", summary)
	}
}

// outputDiagnosticsJSON outputs diagnostics in JSON format
func outputDiagnosticsJSON(diagnostics []analyzer.Diagnostic) {
	// Convert to JSON-friendly format
	type JSONDiagnostic struct {
		File     string `json:"file"`
		Line     int    `json:"line"`
		Column   int    `json:"column"`
		EndLine  int    `json:"end_line"`
		EndCol   int    `json:"end_column"`
		Severity string `json:"severity"`
		Code     string `json:"code"`
		Category string `json:"category"`
		Message  string `json:"message"`
	}

	jsonDiags := make([]JSONDiagnostic, len(diagnostics))
	for i, diag := range diagnostics {
		jsonDiags[i] = JSONDiagnostic{
			File:     diag.Pos.Filename,
			Line:     diag.Pos.Line,
			Column:   diag.Pos.Column,
			EndLine:  diag.End.Line,
			EndCol:   diag.End.Column,
			Severity: diag.Severity.String(),
			Code:     diag.Code,
			Category: diag.Category,
			Message:  diag.Message,
		}
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(jsonDiags); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding JSON: %v\n", err)
	}
}

// expandPathsForLint expands file paths and directory paths to .dingo files
func expandPathsForLint(paths []string) ([]string, error) {
	var dingoFiles []string
	seen := make(map[string]bool) // Deduplicate files

	for _, path := range paths {
		// Handle workspace patterns (./..., ./pkg/...)
		if isWorkspacePattern(path) {
			files, err := expandWorkspacePatternForLint(path)
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

// expandWorkspacePatternForLint expands workspace patterns like ./... to .dingo files
func expandWorkspacePatternForLint(pattern string) ([]string, error) {
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
