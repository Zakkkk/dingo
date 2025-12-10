package lint

import (
	"github.com/MadAppGang/dingo/pkg/lint/analyzer"
)

// MergeDiagnostics merges diagnostics from multiple sources, deduplicates,
// and sorts by position (file, line, column).
//
// Diagnostics are considered duplicates if they have:
// - Same filename
// - Same line and column
// - Same message
//
// When duplicates are found, the first occurrence is preserved with all
// its Fix information intact (important for LSP Code Actions).
//
// The merger handles diagnostics from:
// - Dingo analyzer (correctness + style rules)
// - Go linter (mapped from generated .go files via .dmap)
// - Refactoring analyzer (R0xx rules with Fix suggestions)
//
// Diagnostics that couldn't be mapped from Go code (generated code with no
// Dingo source mapping) should have empty Pos.Filename and will be filtered out.
func MergeDiagnostics(sources ...[]analyzer.Diagnostic) []analyzer.Diagnostic {
	// Collect all diagnostics from all sources
	var all []analyzer.Diagnostic
	for _, source := range sources {
		all = append(all, source...)
	}

	// Filter out diagnostics with no valid position (unmapped generated code)
	all = filterUnmapped(all)

	// Deduplicate while preserving Fix information
	all = deduplicate(all)

	// Sort by file, line, column
	SortDiagnostics(all)

	return all
}

// filterUnmapped removes diagnostics that couldn't be mapped to Dingo source.
// These typically come from Go linter output for generated code sections
// that have no corresponding Dingo source (e.g., auto-generated match helpers).
func filterUnmapped(diagnostics []analyzer.Diagnostic) []analyzer.Diagnostic {
	var filtered []analyzer.Diagnostic
	for _, d := range diagnostics {
		// Keep diagnostics with valid filename
		if d.Pos.Filename != "" {
			filtered = append(filtered, d)
		}
	}
	return filtered
}

// deduplicate removes duplicate diagnostics at the same position with the same message.
// The first occurrence is kept, preserving all Fix information for LSP Code Actions.
//
// This handles cases where:
// - Multiple analyzers report the same issue
// - Both Dingo analyzer and mapped Go linter find the same problem
// - Refactoring suggestions overlap with style warnings
func deduplicate(diagnostics []analyzer.Diagnostic) []analyzer.Diagnostic {
	seen := make(map[diagnosticKey]bool)
	var unique []analyzer.Diagnostic

	for _, d := range diagnostics {
		key := makeDiagnosticKey(d)
		if !seen[key] {
			seen[key] = true
			unique = append(unique, d)
		}
	}

	return unique
}

// diagnosticKey uniquely identifies a diagnostic for deduplication purposes
type diagnosticKey struct {
	filename string
	line     int
	column   int
	message  string
}

// makeDiagnosticKey creates a deduplication key for a diagnostic
func makeDiagnosticKey(d analyzer.Diagnostic) diagnosticKey {
	return diagnosticKey{
		filename: d.Pos.Filename,
		line:     d.Pos.Line,
		column:   d.Pos.Column,
		message:  d.Message,
	}
}

// MergeAndPreserveFixes merges diagnostics with special handling for Fixes.
// When duplicates are found, this variant combines Fix arrays from all occurrences.
//
// This is useful when multiple analyzers can provide different fix strategies
// for the same issue (e.g., both a style fix and a refactoring suggestion).
func MergeAndPreserveFixes(sources ...[]analyzer.Diagnostic) []analyzer.Diagnostic {
	// Collect all diagnostics
	var all []analyzer.Diagnostic
	for _, source := range sources {
		all = append(all, source...)
	}

	// Filter unmapped
	all = filterUnmapped(all)

	// Deduplicate while combining fixes
	all = deduplicateWithFixMerge(all)

	// Sort by position
	SortDiagnostics(all)

	return all
}

// deduplicateWithFixMerge removes duplicates but combines Fix arrays
func deduplicateWithFixMerge(diagnostics []analyzer.Diagnostic) []analyzer.Diagnostic {
	merged := make(map[diagnosticKey]analyzer.Diagnostic)

	for _, d := range diagnostics {
		key := makeDiagnosticKey(d)
		if existing, found := merged[key]; found {
			// Merge fixes from duplicate into existing
			existing.Fixes = append(existing.Fixes, d.Fixes...)
			merged[key] = existing
		} else {
			// First occurrence
			merged[key] = d
		}
	}

	// Convert map back to slice
	var unique []analyzer.Diagnostic
	for _, d := range merged {
		unique = append(unique, d)
	}

	return unique
}
