package lint

import (
	"go/token"
	"testing"

	"github.com/MadAppGang/dingo/pkg/lint/analyzer"
)

func TestMergeDiagnostics(t *testing.T) {
	tests := []struct {
		name     string
		sources  [][]analyzer.Diagnostic
		expected []analyzer.Diagnostic
	}{
		{
			name: "merge multiple sources",
			sources: [][]analyzer.Diagnostic{
				{
					{
						Pos:      token.Position{Filename: "test.dingo", Line: 10, Column: 5},
						End:      token.Position{Filename: "test.dingo", Line: 10, Column: 10},
						Message:  "Use ? operator",
						Severity: analyzer.SeverityWarning,
						Code:     "D002",
						Category: "correctness",
					},
				},
				{
					{
						Pos:      token.Position{Filename: "test.dingo", Line: 15, Column: 3},
						End:      token.Position{Filename: "test.dingo", Line: 15, Column: 6},
						Message:  "Prefer let",
						Severity: analyzer.SeverityHint,
						Code:     "D101",
						Category: "style",
					},
				},
			},
			expected: []analyzer.Diagnostic{
				{
					Pos:      token.Position{Filename: "test.dingo", Line: 10, Column: 5},
					End:      token.Position{Filename: "test.dingo", Line: 10, Column: 10},
					Message:  "Use ? operator",
					Severity: analyzer.SeverityWarning,
					Code:     "D002",
					Category: "correctness",
				},
				{
					Pos:      token.Position{Filename: "test.dingo", Line: 15, Column: 3},
					End:      token.Position{Filename: "test.dingo", Line: 15, Column: 6},
					Message:  "Prefer let",
					Severity: analyzer.SeverityHint,
					Code:     "D101",
					Category: "style",
				},
			},
		},
		{
			name: "deduplicate same position and message",
			sources: [][]analyzer.Diagnostic{
				{
					{
						Pos:      token.Position{Filename: "test.dingo", Line: 10, Column: 5},
						End:      token.Position{Filename: "test.dingo", Line: 10, Column: 10},
						Message:  "Use ? operator",
						Severity: analyzer.SeverityWarning,
						Code:     "D002",
						Category: "correctness",
						Fixes: []analyzer.Fix{
							{Title: "Apply ?", IsPreferred: true},
						},
					},
				},
				{
					{
						Pos:      token.Position{Filename: "test.dingo", Line: 10, Column: 5},
						End:      token.Position{Filename: "test.dingo", Line: 10, Column: 10},
						Message:  "Use ? operator",
						Severity: analyzer.SeverityWarning,
						Code:     "R001", // Different code but same position/message
						Category: "refactor",
					},
				},
			},
			expected: []analyzer.Diagnostic{
				{
					Pos:      token.Position{Filename: "test.dingo", Line: 10, Column: 5},
					End:      token.Position{Filename: "test.dingo", Line: 10, Column: 10},
					Message:  "Use ? operator",
					Severity: analyzer.SeverityWarning,
					Code:     "D002",
					Category: "correctness",
					Fixes: []analyzer.Fix{
						{Title: "Apply ?", IsPreferred: true},
					},
				},
			},
		},
		{
			name: "filter out unmapped diagnostics",
			sources: [][]analyzer.Diagnostic{
				{
					{
						Pos:      token.Position{Filename: "test.dingo", Line: 10, Column: 5},
						Message:  "Valid diagnostic",
						Severity: analyzer.SeverityWarning,
						Code:     "D001",
					},
					{
						Pos:      token.Position{Filename: "", Line: 0, Column: 0},
						Message:  "Unmapped generated code",
						Severity: analyzer.SeverityWarning,
						Code:     "GO001",
					},
				},
			},
			expected: []analyzer.Diagnostic{
				{
					Pos:      token.Position{Filename: "test.dingo", Line: 10, Column: 5},
					Message:  "Valid diagnostic",
					Severity: analyzer.SeverityWarning,
					Code:     "D001",
				},
			},
		},
		{
			name: "sort by file then line then column",
			sources: [][]analyzer.Diagnostic{
				{
					{
						Pos:     token.Position{Filename: "b.dingo", Line: 5, Column: 1},
						Message: "B file",
						Code:    "D001",
					},
					{
						Pos:     token.Position{Filename: "a.dingo", Line: 20, Column: 3},
						Message: "A file line 20",
						Code:    "D002",
					},
					{
						Pos:     token.Position{Filename: "a.dingo", Line: 10, Column: 5},
						Message: "A file line 10 col 5",
						Code:    "D003",
					},
					{
						Pos:     token.Position{Filename: "a.dingo", Line: 10, Column: 2},
						Message: "A file line 10 col 2",
						Code:    "D004",
					},
				},
			},
			expected: []analyzer.Diagnostic{
				{
					Pos:     token.Position{Filename: "a.dingo", Line: 10, Column: 2},
					Message: "A file line 10 col 2",
					Code:    "D004",
				},
				{
					Pos:     token.Position{Filename: "a.dingo", Line: 10, Column: 5},
					Message: "A file line 10 col 5",
					Code:    "D003",
				},
				{
					Pos:     token.Position{Filename: "a.dingo", Line: 20, Column: 3},
					Message: "A file line 20",
					Code:    "D002",
				},
				{
					Pos:     token.Position{Filename: "b.dingo", Line: 5, Column: 1},
					Message: "B file",
					Code:    "D001",
				},
			},
		},
		{
			name: "preserve all fixes from first occurrence",
			sources: [][]analyzer.Diagnostic{
				{
					{
						Pos:     token.Position{Filename: "test.dingo", Line: 10, Column: 5},
						Message: "Use match",
						Code:    "R002",
						Fixes: []analyzer.Fix{
							{Title: "Convert to match", IsPreferred: true},
							{Title: "Add nil check", IsPreferred: false},
						},
					},
				},
			},
			expected: []analyzer.Diagnostic{
				{
					Pos:     token.Position{Filename: "test.dingo", Line: 10, Column: 5},
					Message: "Use match",
					Code:    "R002",
					Fixes: []analyzer.Fix{
						{Title: "Convert to match", IsPreferred: true},
						{Title: "Add nil check", IsPreferred: false},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MergeDiagnostics(tt.sources...)

			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d diagnostics, got %d", len(tt.expected), len(result))
				return
			}

			for i := range result {
				if !diagnosticEqual(result[i], tt.expected[i]) {
					t.Errorf("Diagnostic %d mismatch:\nGot:  %+v\nWant: %+v",
						i, result[i], tt.expected[i])
				}
			}
		})
	}
}

func TestMergeAndPreserveFixes(t *testing.T) {
	tests := []struct {
		name     string
		sources  [][]analyzer.Diagnostic
		expected int // Expected number of fixes in the merged diagnostic
	}{
		{
			name: "combine fixes from duplicates",
			sources: [][]analyzer.Diagnostic{
				{
					{
						Pos:     token.Position{Filename: "test.dingo", Line: 10, Column: 5},
						Message: "Use ? operator",
						Code:    "D002",
						Fixes: []analyzer.Fix{
							{Title: "Fix from analyzer 1", IsPreferred: true},
						},
					},
				},
				{
					{
						Pos:     token.Position{Filename: "test.dingo", Line: 10, Column: 5},
						Message: "Use ? operator",
						Code:    "R001",
						Fixes: []analyzer.Fix{
							{Title: "Fix from analyzer 2", IsPreferred: false},
						},
					},
				},
			},
			expected: 2, // Both fixes should be present
		},
		{
			name: "no fixes to merge",
			sources: [][]analyzer.Diagnostic{
				{
					{
						Pos:     token.Position{Filename: "test.dingo", Line: 10, Column: 5},
						Message: "Warning without fix",
						Code:    "D001",
					},
				},
			},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MergeAndPreserveFixes(tt.sources...)

			if len(result) != 1 {
				t.Errorf("Expected 1 merged diagnostic, got %d", len(result))
				return
			}

			if len(result[0].Fixes) != tt.expected {
				t.Errorf("Expected %d fixes, got %d", tt.expected, len(result[0].Fixes))
			}
		})
	}
}

func TestFilterUnmapped(t *testing.T) {
	diagnostics := []analyzer.Diagnostic{
		{
			Pos:     token.Position{Filename: "valid.dingo", Line: 10, Column: 5},
			Message: "Valid",
		},
		{
			Pos:     token.Position{Filename: "", Line: 0, Column: 0},
			Message: "Unmapped - empty filename",
		},
		{
			Pos:     token.Position{Filename: "another.dingo", Line: 15, Column: 3},
			Message: "Also valid",
		},
	}

	filtered := filterUnmapped(diagnostics)

	if len(filtered) != 2 {
		t.Errorf("Expected 2 diagnostics after filtering, got %d", len(filtered))
	}

	for _, d := range filtered {
		if d.Pos.Filename == "" {
			t.Errorf("Unmapped diagnostic was not filtered out: %+v", d)
		}
	}
}

func TestDeduplicate(t *testing.T) {
	diagnostics := []analyzer.Diagnostic{
		{
			Pos:     token.Position{Filename: "test.dingo", Line: 10, Column: 5},
			Message: "Duplicate 1",
			Code:    "D001",
		},
		{
			Pos:     token.Position{Filename: "test.dingo", Line: 10, Column: 5},
			Message: "Duplicate 1",
			Code:    "D002", // Different code, but same position/message
		},
		{
			Pos:     token.Position{Filename: "test.dingo", Line: 10, Column: 5},
			Message: "Different message",
			Code:    "D003", // Same position, different message - should be kept
		},
		{
			Pos:     token.Position{Filename: "test.dingo", Line: 15, Column: 3},
			Message: "Duplicate 1",
			Code:    "D004", // Different position - should be kept
		},
	}

	unique := deduplicate(diagnostics)

	if len(unique) != 3 {
		t.Errorf("Expected 3 unique diagnostics, got %d", len(unique))
	}

	// Verify first occurrence preserved
	if unique[0].Code != "D001" {
		t.Errorf("Expected first occurrence to be preserved, got code %s", unique[0].Code)
	}
}

// Helper function to compare diagnostics
func diagnosticEqual(a, b analyzer.Diagnostic) bool {
	if a.Pos.Filename != b.Pos.Filename ||
		a.Pos.Line != b.Pos.Line ||
		a.Pos.Column != b.Pos.Column ||
		a.Message != b.Message ||
		a.Code != b.Code ||
		a.Severity != b.Severity ||
		a.Category != b.Category {
		return false
	}

	// Check fixes count matches
	if len(a.Fixes) != len(b.Fixes) {
		return false
	}

	// Check each fix
	for i := range a.Fixes {
		if a.Fixes[i].Title != b.Fixes[i].Title ||
			a.Fixes[i].IsPreferred != b.Fixes[i].IsPreferred {
			return false
		}
	}

	return true
}
