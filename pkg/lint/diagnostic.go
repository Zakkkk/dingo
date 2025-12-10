package lint

import (
	"fmt"
	"sort"

	"github.com/MadAppGang/dingo/pkg/lint/analyzer"
)

// SortDiagnostics sorts diagnostics by file position (line, then column)
func SortDiagnostics(diagnostics []analyzer.Diagnostic) {
	sort.Slice(diagnostics, func(i, j int) bool {
		if diagnostics[i].Pos.Filename != diagnostics[j].Pos.Filename {
			return diagnostics[i].Pos.Filename < diagnostics[j].Pos.Filename
		}
		if diagnostics[i].Pos.Line != diagnostics[j].Pos.Line {
			return diagnostics[i].Pos.Line < diagnostics[j].Pos.Line
		}
		return diagnostics[i].Pos.Column < diagnostics[j].Pos.Column
	})
}

// FormatDiagnostic formats a diagnostic for display
// Format: filename:line:col: severity[code]: message
func FormatDiagnostic(d analyzer.Diagnostic) string {
	return fmt.Sprintf("%s:%d:%d: %s[%s]: %s",
		d.Pos.Filename,
		d.Pos.Line,
		d.Pos.Column,
		d.Severity.String(),
		d.Code,
		d.Message,
	)
}

// FilterByCategory filters diagnostics by category
func FilterByCategory(diagnostics []analyzer.Diagnostic, category string) []analyzer.Diagnostic {
	var filtered []analyzer.Diagnostic
	for _, d := range diagnostics {
		if d.Category == category {
			filtered = append(filtered, d)
		}
	}
	return filtered
}

// FilterBySeverity filters diagnostics by minimum severity level
func FilterBySeverity(diagnostics []analyzer.Diagnostic, minSeverity analyzer.Severity) []analyzer.Diagnostic {
	var filtered []analyzer.Diagnostic
	for _, d := range diagnostics {
		if d.Severity >= minSeverity {
			filtered = append(filtered, d)
		}
	}
	return filtered
}

// GroupByFile groups diagnostics by filename
func GroupByFile(diagnostics []analyzer.Diagnostic) map[string][]analyzer.Diagnostic {
	groups := make(map[string][]analyzer.Diagnostic)
	for _, d := range diagnostics {
		filename := d.Pos.Filename
		groups[filename] = append(groups[filename], d)
	}
	return groups
}
