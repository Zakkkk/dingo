// Package generator - marker injection utilities
package generator

import (
	"fmt"
	"regexp"
	"strings"
)

// Plugin IDs for marker generation:
// 1 = error_propagation (? operator)
// 2 = result_type (Result<T, E>)
// 3 = option_type (Option<T>)
// 4 = pattern_matching (match expressions)
// 5 = sum_types (enum)

// MarkerInjector handles injection of DINGO:GENERATED markers into Go source code
type MarkerInjector struct {
	enabled bool
}

// NewMarkerInjector creates a new marker injector
func NewMarkerInjector(enabled bool) *MarkerInjector {
	return &MarkerInjector{
		enabled: enabled,
	}
}

// InjectMarkers injects DINGO:GENERATED markers into generated Go code
// This is a post-processing step that runs after AST generation
func (m *MarkerInjector) InjectMarkers(source []byte) ([]byte, error) {
	if !m.enabled {
		return source, nil
	}

	sourceStr := string(source)

	// Check if markers are already present (added by preprocessor)
	// If so, skip injection to avoid duplicates
	if strings.Contains(sourceStr, "// dingo:s:") || strings.Contains(sourceStr, "// dingo:e:") {
		return source, nil
	}

	// Pattern to detect error propagation generated code
	// Looks for: if __err0 != nil { return ... }
	errorCheckPattern := regexp.MustCompile(`(?m)(^[ \t]*if __err\d+ != nil \{[^}]*return[^}]*\}[ \t]*\n)`)

	// Inject markers around error propagation blocks
	// Using plugin ID 1 for error_propagation
	result := errorCheckPattern.ReplaceAllStringFunc(sourceStr, func(match string) string {
		// Extract indentation from the if statement
		indent := ""
		if idx := strings.Index(match, "if"); idx > 0 {
			indent = match[:idx]
		}

		startMarker := fmt.Sprintf("%s// dingo:s:1\n", indent)
		endMarker := fmt.Sprintf("%s// dingo:e:1\n", indent)

		return startMarker + match + endMarker
	})

	return []byte(result), nil
}

// RemoveDebugMarkers removes internal dingo:* markers from generated code
// Called when config.Debug.KeepMarkers is false (default)
// Preserves non-marker content in comments
//
// CRITICAL: This function preserves line count to maintain source map accuracy.
// Standalone marker lines are replaced with empty lines, not removed.
//
// Marker patterns:
//   - // dingo:e:N  (error propagation)
//   - // dingo:let:x (immutability)
//   - // dingo:n:N  (line numbers)
//   - // dingo:t:N  (type markers)
//   - // dingo:s:N  (start markers)
//
// Edge cases:
//   - Standalone marker line: replaced with empty line (preserves line numbers)
//   - Marker at end of line: removed, keep rest of line
//   - Marker with other content: remove marker only
func RemoveDebugMarkers(content []byte) []byte {
	lines := strings.Split(string(content), "\n")
	result := make([]string, 0, len(lines))

	// Pattern matches: dingo:X:Y where X is letters and Y is alphanumeric/comma-separated
	// Examples: dingo:e:0, dingo:let:x, dingo:let:a,b, dingo:n:10
	// Note: We don't include // in the pattern to preserve comment prefix
	markerPattern := regexp.MustCompile(`dingo:[a-z]+:[a-zA-Z0-9_,]+`)

	for _, line := range lines {
		// Check if line contains a dingo marker
		if strings.Contains(line, "// dingo:") {
			// Check if the entire comment is just the marker
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "// dingo:") {
				// Standalone marker line - check if there's anything after the marker
				cleaned := markerPattern.ReplaceAllString(trimmed, "")
				cleaned = strings.TrimSpace(cleaned)
				if cleaned == "" || cleaned == "//" {
					// CRITICAL: Replace with empty line instead of removing
					// This preserves line numbers for source map accuracy
					result = append(result, "")
					continue
				}
			}

			// Line has marker + other content - remove just the marker
			cleaned := markerPattern.ReplaceAllString(line, "")

			// Normalize "//  " to "// " (double space after comment start)
			cleaned = strings.ReplaceAll(cleaned, "//  ", "// ")

			// Clean up trailing "// " left after marker removal
			cleaned = strings.TrimRight(cleaned, " \t")

			// If the line now ends with just "//", remove it
			if strings.HasSuffix(strings.TrimSpace(cleaned), "//") {
				// Remove the trailing "//" and any spaces before it
				cleaned = strings.TrimRight(cleaned, "/ \t")
			}

			result = append(result, cleaned)
		} else {
			// No marker, keep line as-is
			result = append(result, line)
		}
	}

	return []byte(strings.Join(result, "\n"))
}

// injectErrorPropagationMarkers wraps error propagation blocks with markers
