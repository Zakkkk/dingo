//go:build ignore

package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

// DiffVisualizer generates markdown diffs for failed golden tests
type DiffVisualizer struct {
	failures []TestFailure
}

// TestFailure represents a single golden test failure
type TestFailure struct {
	Name     string
	Expected string
	Actual   string
	DiffInfo DiffInfo
}

// DiffInfo contains statistics about the difference
type DiffInfo struct {
	LinesAdded   int
	LinesRemoved int
	LinesChanged int
	TotalDiff    int
}

var (
	// Regex patterns for parsing test output
	failurePattern  = regexp.MustCompile(`FAIL: TestGoldenFiles/(.+)`)
	expectedPattern = regexp.MustCompile(`Expected: (.+\.go\.golden)`)
	actualPattern   = regexp.MustCompile(`Actual: (.+\.go)`)
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: diff-visualizer <test-output-file>")
		os.Exit(1)
	}

	inputFile := os.Args[1]
	file, err := os.Open(inputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening file: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	visualizer := NewDiffVisualizer()
	if err := visualizer.Parse(file); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing test output: %v\n", err)
		os.Exit(1)
	}

	if err := visualizer.GenerateMarkdown(os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating markdown: %v\n", err)
		os.Exit(1)
	}
}

// NewDiffVisualizer creates a new DiffVisualizer instance
func NewDiffVisualizer() *DiffVisualizer {
	return &DiffVisualizer{
		failures: make([]TestFailure, 0),
	}
}

// Parse extracts test failures from test output
func (dv *DiffVisualizer) Parse(reader io.Reader) error {
	scanner := bufio.NewScanner(reader)
	var currentFailure *TestFailure

	for scanner.Scan() {
		line := scanner.Text()

		// Check for test failure
		if matches := failurePattern.FindStringSubmatch(line); matches != nil {
			if currentFailure != nil {
				dv.failures = append(dv.failures, *currentFailure)
			}
			currentFailure = &TestFailure{
				Name: matches[1],
			}
		}

		// Extract expected file path
		if currentFailure != nil {
			if matches := expectedPattern.FindStringSubmatch(line); matches != nil {
				currentFailure.Expected = matches[1]
			}
			if matches := actualPattern.FindStringSubmatch(line); matches != nil {
				currentFailure.Actual = matches[1]
			}
		}
	}

	// Add last failure
	if currentFailure != nil {
		dv.failures = append(dv.failures, *currentFailure)
	}

	return scanner.Err()
}

// GenerateMarkdown writes markdown-formatted diff report
func (dv *DiffVisualizer) GenerateMarkdown(writer io.Writer) error {
	fmt.Fprintln(writer, "# Golden Test Failures - Diff Report")
	fmt.Fprintln(writer, "")
	fmt.Fprintf(writer, "**Total Failures**: %d\n\n", len(dv.failures))

	if len(dv.failures) == 0 {
		fmt.Fprintln(writer, "✅ All tests passed!")
		return nil
	}

	// Summary table
	fmt.Fprintln(writer, "## Summary")
	fmt.Fprintln(writer, "")
	fmt.Fprintln(writer, "| Test Name | Status | Details |")
	fmt.Fprintln(writer, "|-----------|--------|---------|")

	for _, failure := range dv.failures {
		fmt.Fprintf(writer, "| `%s` | ❌ Failed | [View Diff](#%s) |\n",
			failure.Name,
			strings.ReplaceAll(failure.Name, "_", "-"))
	}
	fmt.Fprintln(writer, "")

	// Detailed diffs
	fmt.Fprintln(writer, "## Detailed Diffs")
	fmt.Fprintln(writer, "")

	for _, failure := range dv.failures {
		if err := dv.generateFailureDiff(writer, failure); err != nil {
			return err
		}
	}

	return nil
}

// generateFailureDiff creates a detailed diff for a single failure
func (dv *DiffVisualizer) generateFailureDiff(writer io.Writer, failure TestFailure) error {
	anchor := strings.ReplaceAll(failure.Name, "_", "-")
	fmt.Fprintf(writer, "### %s {#%s}\n\n", failure.Name, anchor)

	// Check if files exist
	if failure.Expected == "" || failure.Actual == "" {
		fmt.Fprintln(writer, "⚠️ **Could not extract file paths from test output**")
		fmt.Fprintln(writer, "")
		return nil
	}

	expectedExists := fileExists(failure.Expected)
	actualExists := fileExists(failure.Actual)

	if !expectedExists || !actualExists {
		fmt.Fprintln(writer, "⚠️ **File not found:**")
		if !expectedExists {
			fmt.Fprintf(writer, "- Expected: `%s`\n", failure.Expected)
		}
		if !actualExists {
			fmt.Fprintf(writer, "- Actual: `%s`\n", failure.Actual)
		}
		fmt.Fprintln(writer, "")
		return nil
	}

	// Read file contents
	expectedContent, err := os.ReadFile(failure.Expected)
	if err != nil {
		return fmt.Errorf("reading expected file: %w", err)
	}

	actualContent, err := os.ReadFile(failure.Actual)
	if err != nil {
		return fmt.Errorf("reading actual file: %w", err)
	}

	// Calculate diff statistics
	diffInfo := calculateDiffInfo(string(expectedContent), string(actualContent))
	failure.DiffInfo = diffInfo

	// Show statistics
	fmt.Fprintln(writer, "**Diff Statistics:**")
	fmt.Fprintf(writer, "- Lines Added: %d\n", diffInfo.LinesAdded)
	fmt.Fprintf(writer, "- Lines Removed: %d\n", diffInfo.LinesRemoved)
	fmt.Fprintf(writer, "- Lines Changed: %d\n", diffInfo.LinesChanged)
	fmt.Fprintln(writer, "")

	// Show side-by-side comparison
	fmt.Fprintln(writer, "**Expected (.go.golden):**")
	fmt.Fprintln(writer, "```go")
	fmt.Fprint(writer, string(expectedContent))
	fmt.Fprintln(writer, "```")
	fmt.Fprintln(writer, "")

	fmt.Fprintln(writer, "**Actual (transpiled output):**")
	fmt.Fprintln(writer, "```go")
	fmt.Fprint(writer, string(actualContent))
	fmt.Fprintln(writer, "```")
	fmt.Fprintln(writer, "")

	// Show unified diff
	fmt.Fprintln(writer, "**Unified Diff:**")
	fmt.Fprintln(writer, "```diff")
	unifiedDiff := generateUnifiedDiff(string(expectedContent), string(actualContent))
	fmt.Fprint(writer, unifiedDiff)
	fmt.Fprintln(writer, "```")
	fmt.Fprintln(writer, "")
	fmt.Fprintln(writer, "---")
	fmt.Fprintln(writer, "")

	return nil
}

// fileExists checks if a file exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// calculateDiffInfo computes diff statistics
func calculateDiffInfo(expected, actual string) DiffInfo {
	expectedLines := strings.Split(expected, "\n")
	actualLines := strings.Split(actual, "\n")

	info := DiffInfo{}

	// Simple line-based diff (not LCS, but sufficient for visualization)
	expectedSet := make(map[string]bool)
	for _, line := range expectedLines {
		expectedSet[line] = true
	}

	actualSet := make(map[string]bool)
	for _, line := range actualLines {
		actualSet[line] = true
	}

	// Lines in actual but not in expected
	for line := range actualSet {
		if !expectedSet[line] && strings.TrimSpace(line) != "" {
			info.LinesAdded++
		}
	}

	// Lines in expected but not in actual
	for line := range expectedSet {
		if !actualSet[line] && strings.TrimSpace(line) != "" {
			info.LinesRemoved++
		}
	}

	info.TotalDiff = info.LinesAdded + info.LinesRemoved
	info.LinesChanged = min(info.LinesAdded, info.LinesRemoved)

	return info
}

// generateUnifiedDiff creates a unified diff string
func generateUnifiedDiff(expected, actual string) string {
	expectedLines := strings.Split(expected, "\n")
	actualLines := strings.Split(actual, "\n")

	var diff strings.Builder

	// Header
	diff.WriteString("--- expected\n")
	diff.WriteString("+++ actual\n")

	// Simple line-by-line comparison
	maxLines := max(len(expectedLines), len(actualLines))
	for i := 0; i < maxLines; i++ {
		var expectedLine, actualLine string
		if i < len(expectedLines) {
			expectedLine = expectedLines[i]
		}
		if i < len(actualLines) {
			actualLine = actualLines[i]
		}

		if expectedLine != actualLine {
			if i < len(expectedLines) {
				diff.WriteString(fmt.Sprintf("-%s\n", expectedLine))
			}
			if i < len(actualLines) {
				diff.WriteString(fmt.Sprintf("+%s\n", actualLine))
			}
		}
	}

	return diff.String()
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// max returns the maximum of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
