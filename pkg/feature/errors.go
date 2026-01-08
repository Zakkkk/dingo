package feature

import (
	"fmt"
	"strings"
)

// DisabledFeatureError is returned when syntax for a disabled feature is detected
type DisabledFeatureError struct {
	Feature   string           // Name of the disabled feature
	Locations []SyntaxLocation // Where the syntax was found
	Message   string           // User-friendly error message
}

func (e *DisabledFeatureError) Error() string {
	if len(e.Locations) == 0 {
		return e.Message
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("feature '%s' is disabled but syntax was found:\n", e.Feature))

	for i, loc := range e.Locations {
		if i >= 5 {
			sb.WriteString(fmt.Sprintf("  ... and %d more locations\n", len(e.Locations)-5))
			break
		}
		sb.WriteString(fmt.Sprintf("  line %d, column %d: %s\n", loc.Line, loc.Column, loc.Snippet))
	}

	sb.WriteString(fmt.Sprintf("\nTo enable this feature, add to dingo.toml:\n  [features]\n  %s = true", e.Feature))

	return sb.String()
}

// DependencyError is returned when a plugin's dependencies are not satisfied
type DependencyError struct {
	Plugin   string   // Plugin that has unmet dependencies
	Missing  []string // Names of missing dependencies
	Disabled []string // Names of disabled dependencies
}

func (e *DependencyError) Error() string {
	var parts []string

	if len(e.Missing) > 0 {
		parts = append(parts, fmt.Sprintf("missing plugins: %s", strings.Join(e.Missing, ", ")))
	}
	if len(e.Disabled) > 0 {
		parts = append(parts, fmt.Sprintf("disabled plugins: %s", strings.Join(e.Disabled, ", ")))
	}

	return fmt.Sprintf("plugin '%s' has unmet dependencies: %s", e.Plugin, strings.Join(parts, "; "))
}

// ConflictError is returned when conflicting plugins are enabled
type ConflictError struct {
	Plugin    string   // Plugin that detected the conflict
	Conflicts []string // Names of conflicting plugins that are enabled
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("plugin '%s' conflicts with enabled plugins: %s",
		e.Plugin, strings.Join(e.Conflicts, ", "))
}

// TransformError wraps an error that occurred during transformation
type TransformError struct {
	Plugin  string // Plugin that failed
	Phase   string // Phase of transformation (e.g., "detect", "transform")
	Line    int    // Line where error occurred (if known)
	Column  int    // Column where error occurred (if known)
	Message string // Error message
	Cause   error  // Underlying error
}

func (e *TransformError) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("%s plugin error at line %d, column %d: %s", e.Plugin, e.Line, e.Column, e.Message)
	}
	return fmt.Sprintf("%s plugin error: %s", e.Plugin, e.Message)
}

func (e *TransformError) Unwrap() error {
	return e.Cause
}

// NewTransformError creates a new TransformError
func NewTransformError(plugin, phase, message string, cause error) *TransformError {
	return &TransformError{
		Plugin:  plugin,
		Phase:   phase,
		Message: message,
		Cause:   cause,
	}
}

// NewTransformErrorAt creates a TransformError with location information
func NewTransformErrorAt(plugin string, line, col int, message string, cause error) *TransformError {
	return &TransformError{
		Plugin:  plugin,
		Phase:   "transform",
		Line:    line,
		Column:  col,
		Message: message,
		Cause:   cause,
	}
}
