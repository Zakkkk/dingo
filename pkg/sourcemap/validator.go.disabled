// Package sourcemap provides source map validation for Dingo transpiler
package sourcemap

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/MadAppGang/dingo/pkg/preprocessor"
)

// ValidationResult represents the result of source map validation
type ValidationResult struct {
	Valid          bool
	Errors         []ValidationError
	Warnings       []ValidationWarning
	TotalMappings  int
	RoundTripTests int
	PassedTests    int
	Accuracy       float64 // Percentage (0-100)
}

// ValidationError represents a validation error
type ValidationError struct {
	Type    string
	Message string
	Line    int // Optional: relevant line number
	Column  int // Optional: relevant column number
}

// ValidationWarning represents a validation warning
type ValidationWarning struct {
	Type    string
	Message string
}

// Validator validates source map correctness and accuracy
type Validator struct {
	sourceMap *preprocessor.SourceMap
	strict    bool // Strict mode: warnings become errors
}

// NewValidator creates a new source map validator
func NewValidator(sm *preprocessor.SourceMap) *Validator {
	return &Validator{
		sourceMap: sm,
		strict:    false,
	}
}

// NewValidatorFromFile loads and validates a source map file
func NewValidatorFromFile(path string) (*Validator, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open source map file: %w", err)
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read source map file: %w", err)
	}

	sm, err := preprocessor.FromJSON(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse source map: %w", err)
	}

	return &Validator{
		sourceMap: sm,
		strict:    false,
	}, nil
}

// SetStrict enables strict validation mode (warnings become errors)
func (v *Validator) SetStrict(strict bool) {
	v.strict = strict
}

// Validate performs comprehensive source map validation
func (v *Validator) Validate() ValidationResult {
	result := ValidationResult{
		Valid:          true,
		Errors:         make([]ValidationError, 0),
		Warnings:       make([]ValidationWarning, 0),
		TotalMappings:  len(v.sourceMap.Mappings),
		RoundTripTests: 0,
		PassedTests:    0,
	}

	// Validation checks
	v.validateSchema(&result)
	v.validateMappings(&result)
	v.validateRoundTrip(&result)
	v.validateConsistency(&result)

	// Calculate accuracy
	if result.RoundTripTests > 0 {
		result.Accuracy = (float64(result.PassedTests) / float64(result.RoundTripTests)) * 100.0
	}

	// In strict mode, convert warnings to errors
	if v.strict && len(result.Warnings) > 0 {
		for _, w := range result.Warnings {
			result.Errors = append(result.Errors, ValidationError{
				Type:    w.Type,
				Message: w.Message,
			})
		}
		result.Warnings = nil
		result.Valid = false
	}

	// Result is invalid if there are any errors
	if len(result.Errors) > 0 {
		result.Valid = false
	}

	return result
}

// validateSchema validates the basic source map schema
func (v *Validator) validateSchema(result *ValidationResult) {
	sourcePath := v.sourceMap.DingoFile
	if sourcePath == "" {
		sourcePath = "<unknown source map>"
	}

	// Check version
	if v.sourceMap.Version != 1 {
		result.Errors = append(result.Errors, ValidationError{
			Type: "schema",
			Message: fmt.Sprintf(
				"source map %s: unsupported version %d (expected 1)",
				sourcePath, v.sourceMap.Version,
			),
		})
	}

	// Check file paths (optional fields)
	if v.sourceMap.DingoFile == "" {
		result.Warnings = append(result.Warnings, ValidationWarning{
			Type: "schema",
			Message: fmt.Sprintf(
				"source map: missing dingo_file field (optional but recommended for debugging)",
			),
		})
	}

	if v.sourceMap.GoFile == "" {
		result.Warnings = append(result.Warnings, ValidationWarning{
			Type: "schema",
			Message: fmt.Sprintf(
				"source map %s: missing go_file field (optional but recommended for debugging)",
				sourcePath,
			),
		})
	}

	// Check mappings array
	if v.sourceMap.Mappings == nil {
		result.Errors = append(result.Errors, ValidationError{
			Type: "schema",
			Message: fmt.Sprintf(
				"source map %s: mappings array is nil (should be initialized, even if empty)",
				sourcePath,
			),
		})
	}
}

// validateMappings validates individual mapping entries
func (v *Validator) validateMappings(result *ValidationResult) {
	for i, m := range v.sourceMap.Mappings {
		// Validate position values are positive
		if m.GeneratedLine < 1 {
			result.Errors = append(result.Errors, ValidationError{
				Type:    "mapping",
				Message: fmt.Sprintf("mapping %d: invalid generated_line %d (must be >= 1)", i, m.GeneratedLine),
			})
		}

		if m.GeneratedColumn < 0 {
			result.Errors = append(result.Errors, ValidationError{
				Type:    "mapping",
				Message: fmt.Sprintf("mapping %d: invalid generated_column %d (must be >= 0)", i, m.GeneratedColumn),
			})
		}

		if m.OriginalLine < 1 {
			result.Errors = append(result.Errors, ValidationError{
				Type:    "mapping",
				Message: fmt.Sprintf("mapping %d: invalid original_line %d (must be >= 1)", i, m.OriginalLine),
			})
		}

		if m.OriginalColumn < 0 {
			result.Errors = append(result.Errors, ValidationError{
				Type:    "mapping",
				Message: fmt.Sprintf("mapping %d: invalid original_column %d (must be >= 0)", i, m.OriginalColumn),
			})
		}

		if m.Length < 0 {
			result.Errors = append(result.Errors, ValidationError{
				Type:    "mapping",
				Message: fmt.Sprintf("mapping %d: invalid length %d (must be >= 0)", i, m.Length),
			})
		}

		// Warn about zero-length mappings
		if m.Length == 0 {
			result.Warnings = append(result.Warnings, ValidationWarning{
				Type:    "mapping",
				Message: fmt.Sprintf("mapping %d: zero length (may indicate incomplete mapping)", i),
			})
		}

		// Warn about very large lengths (potential error)
		if m.Length > 1000 {
			result.Warnings = append(result.Warnings, ValidationWarning{
				Type:    "mapping",
				Message: fmt.Sprintf("mapping %d: unusually large length %d (verify correctness)", i, m.Length),
			})
		}
	}
}

// validateRoundTrip tests round-trip position mapping accuracy
func (v *Validator) validateRoundTrip(result *ValidationResult) {
	// Test each mapping for round-trip accuracy
	for i, m := range v.sourceMap.Mappings {
		result.RoundTripTests++

		// Forward mapping: Dingo → Go
		goLine, goCol := v.sourceMap.MapToGenerated(m.OriginalLine, m.OriginalColumn)

		// Backward mapping: Go → Dingo
		dingoLine, dingoCol := v.sourceMap.MapToOriginal(goLine, goCol)

		// Check if we get back to the original position
		if dingoLine != m.OriginalLine || dingoCol != m.OriginalColumn {
			sourcePath := v.sourceMap.DingoFile
			if sourcePath == "" {
				sourcePath = "<unknown>"
			}
			result.Errors = append(result.Errors, ValidationError{
				Type: "round-trip",
				Message: fmt.Sprintf(
					"source map %s: mapping %d round-trip validation failed\n"+
						"  Dingo position: line %d, column %d\n"+
						"  → Go position: line %d, column %d\n"+
						"  → Round-trip: line %d, column %d (expected %d, %d)",
					sourcePath, i,
					m.OriginalLine, m.OriginalColumn,
					goLine, goCol,
					dingoLine, dingoCol,
					m.OriginalLine, m.OriginalColumn,
				),
				Line:   m.OriginalLine,
				Column: m.OriginalColumn,
			})
		} else {
			result.PassedTests++
		}

		// Also test reverse round-trip: Go → Dingo → Go
		result.RoundTripTests++
		backToGoLine, backToGoCol := v.sourceMap.MapToGenerated(dingoLine, dingoCol)

		if backToGoLine != goLine || backToGoCol != goCol {
			goPath := v.sourceMap.GoFile
			if goPath == "" {
				goPath = "<unknown>"
			}
			result.Errors = append(result.Errors, ValidationError{
				Type: "round-trip",
				Message: fmt.Sprintf(
					"source map %s: mapping %d reverse round-trip failed\n"+
						"  Go position: line %d, column %d\n"+
						"  → Dingo position: line %d, column %d\n"+
						"  → Round-trip Go: line %d, column %d (expected %d, %d)",
					goPath, i,
					goLine, goCol,
					dingoLine, dingoCol,
					backToGoLine, backToGoCol,
					goLine, goCol,
				),
				Line:   m.GeneratedLine,
				Column: m.GeneratedColumn,
			})
		} else {
			result.PassedTests++
		}
	}
}

// validateConsistency checks for logical consistency in mappings
func (v *Validator) validateConsistency(result *ValidationResult) {
	if len(v.sourceMap.Mappings) == 0 {
		// Empty source maps are valid (but warn)
		result.Warnings = append(result.Warnings, ValidationWarning{
			Type:    "consistency",
			Message: "source map has no mappings (empty file?)",
		})
		return
	}

	// Check for duplicate mappings (same position mapped multiple times)
	seen := make(map[string]bool)
	for i, m := range v.sourceMap.Mappings {
		key := fmt.Sprintf("gen:%d:%d", m.GeneratedLine, m.GeneratedColumn)
		if seen[key] {
			result.Warnings = append(result.Warnings, ValidationWarning{
				Type:    "consistency",
				Message: fmt.Sprintf("mapping %d: duplicate generated position %d:%d", i, m.GeneratedLine, m.GeneratedColumn),
			})
		}
		seen[key] = true
	}

	// Check for overlapping mappings on the same line
	lineGroups := make(map[int][]preprocessor.Mapping)
	for _, m := range v.sourceMap.Mappings {
		lineGroups[m.GeneratedLine] = append(lineGroups[m.GeneratedLine], m)
	}

	for line, mappings := range lineGroups {
		if len(mappings) < 2 {
			continue
		}

		// Check for overlaps
		for i := 0; i < len(mappings); i++ {
			for j := i + 1; j < len(mappings); j++ {
				m1, m2 := mappings[i], mappings[j]

				// Check if ranges overlap
				m1End := m1.GeneratedColumn + m1.Length
				m2End := m2.GeneratedColumn + m2.Length

				overlap := false
				if m1.GeneratedColumn <= m2.GeneratedColumn && m2.GeneratedColumn < m1End {
					overlap = true
				} else if m2.GeneratedColumn <= m1.GeneratedColumn && m1.GeneratedColumn < m2End {
					overlap = true
				}

				if overlap {
					result.Warnings = append(result.Warnings, ValidationWarning{
						Type: "consistency",
						Message: fmt.Sprintf(
							"overlapping mappings on line %d: [%d-%d] and [%d-%d]",
							line, m1.GeneratedColumn, m1End, m2.GeneratedColumn, m2End,
						),
					})
				}
			}
		}
	}
}

// ValidateJSON validates a source map JSON file
func ValidateJSON(data []byte) (*ValidationResult, error) {
	// Try to parse as JSON first
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return &ValidationResult{
			Valid: false,
			Errors: []ValidationError{
				{Type: "json", Message: fmt.Sprintf("invalid JSON: %v", err)},
			},
		}, nil
	}

	// Parse as source map
	sm, err := preprocessor.FromJSON(data)
	if err != nil {
		return &ValidationResult{
			Valid: false,
			Errors: []ValidationError{
				{Type: "parse", Message: fmt.Sprintf("failed to parse source map: %v", err)},
			},
		}, nil
	}

	// Validate
	validator := NewValidator(sm)
	result := validator.Validate()
	return &result, nil
}

// String formats the validation result as a human-readable string
func (r ValidationResult) String() string {
	var s string
	if r.Valid {
		s += "✓ Source map is VALID\n"
	} else {
		s += "✗ Source map is INVALID\n"
	}

	s += fmt.Sprintf("\nStatistics:\n")
	s += fmt.Sprintf("  Total mappings: %d\n", r.TotalMappings)
	s += fmt.Sprintf("  Round-trip tests: %d\n", r.RoundTripTests)
	s += fmt.Sprintf("  Passed tests: %d\n", r.PassedTests)
	s += fmt.Sprintf("  Accuracy: %.2f%%\n", r.Accuracy)

	if len(r.Errors) > 0 {
		s += fmt.Sprintf("\nErrors (%d):\n", len(r.Errors))
		for _, e := range r.Errors {
			s += fmt.Sprintf("  [%s] %s\n", e.Type, e.Message)
		}
	}

	if len(r.Warnings) > 0 {
		s += fmt.Sprintf("\nWarnings (%d):\n", len(r.Warnings))
		for _, w := range r.Warnings {
			s += fmt.Sprintf("  [%s] %s\n", w.Type, w.Message)
		}
	}

	return s
}
