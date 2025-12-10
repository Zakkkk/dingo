package lint_test

import (
	"fmt"
	"go/token"

	"github.com/MadAppGang/dingo/pkg/lint"
	"github.com/MadAppGang/dingo/pkg/lint/analyzer"
)

// ExampleMergeDiagnostics demonstrates merging diagnostics from multiple sources
func ExampleMergeDiagnostics() {
	// Simulate diagnostics from Dingo analyzer (correctness rules)
	dingoResults := []analyzer.Diagnostic{
		{
			Pos:      token.Position{Filename: "example.dingo", Line: 10, Column: 5},
			Message:  "Match expression not exhaustive",
			Severity: analyzer.SeverityWarning,
			Code:     "D001",
			Category: "correctness",
		},
		{
			Pos:      token.Position{Filename: "example.dingo", Line: 25, Column: 12},
			Message:  "Invalid use of ? operator",
			Severity: analyzer.SeverityWarning,
			Code:     "D002",
			Category: "correctness",
		},
	}

	// Simulate diagnostics from refactoring analyzer
	refactorResults := []analyzer.Diagnostic{
		{
			Pos:      token.Position{Filename: "example.dingo", Line: 15, Column: 3},
			Message:  "Consider using ? operator for error propagation",
			Severity: analyzer.SeverityHint,
			Code:     "R001",
			Category: "refactor",
			Fixes: []analyzer.Fix{
				{
					Title:       "Use ? operator",
					IsPreferred: true,
				},
			},
		},
	}

	// Simulate diagnostics from Go linter (mapped back to .dingo positions)
	// Note: One diagnostic couldn't be mapped (generated code) - has empty filename
	goLintResults := []analyzer.Diagnostic{
		{
			Pos:      token.Position{Filename: "example.dingo", Line: 30, Column: 8},
			Message:  "Variable 'x' declared but not used",
			Severity: analyzer.SeverityWarning,
			Code:     "unused",
			Category: "style",
		},
		{
			Pos:      token.Position{Filename: "", Line: 0, Column: 0}, // Unmapped!
			Message:  "Generated helper function not used",
			Severity: analyzer.SeverityWarning,
			Code:     "unused",
			Category: "style",
		},
	}

	// Merge all sources
	merged := lint.MergeDiagnostics(dingoResults, refactorResults, goLintResults)

	// Print merged diagnostics
	for _, diag := range merged {
		fmt.Println(lint.FormatDiagnostic(diag))
	}

	// Output:
	// example.dingo:10:5: warning[D001]: Match expression not exhaustive
	// example.dingo:15:3: hint[R001]: Consider using ? operator for error propagation
	// example.dingo:25:12: warning[D002]: Invalid use of ? operator
	// example.dingo:30:8: warning[unused]: Variable 'x' declared but not used
}

// ExampleMergeDiagnostics_deduplication demonstrates deduplication behavior
func ExampleMergeDiagnostics_deduplication() {
	// Two analyzers detect the same issue
	source1 := []analyzer.Diagnostic{
		{
			Pos:      token.Position{Filename: "test.dingo", Line: 10, Column: 5},
			Message:  "Use ? operator",
			Code:     "D002",
			Severity: analyzer.SeverityWarning,
			Fixes: []analyzer.Fix{
				{Title: "Apply ? operator", IsPreferred: true},
			},
		},
	}

	source2 := []analyzer.Diagnostic{
		{
			Pos:      token.Position{Filename: "test.dingo", Line: 10, Column: 5},
			Message:  "Use ? operator",
			Code:     "R001", // Different code!
			Severity: analyzer.SeverityHint,
		},
	}

	// Merge will deduplicate (same position + message)
	merged := lint.MergeDiagnostics(source1, source2)

	// Only one diagnostic remains (first occurrence preserved)
	fmt.Printf("Count: %d\n", len(merged))
	fmt.Printf("Code: %s\n", merged[0].Code)
	fmt.Printf("Fixes: %d\n", len(merged[0].Fixes))

	// Output:
	// Count: 1
	// Code: D002
	// Fixes: 1
}
