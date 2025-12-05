// Package transpiler provides mode selection for transpilation strategies
package transpiler

import (
	"fmt"
)

// TranspileMode represents the transpilation strategy to use
type TranspileMode int

const (
	// ModeAST is the ONLY transpilation mode
	// Pure AST-based pipeline with no preprocessor fallback
	ModeAST TranspileMode = iota
)

// String returns the string representation of the mode
func (m TranspileMode) String() string {
	switch m {
	case ModeAST:
		return "ast"
	default:
		return "unknown"
	}
}

// ParseMode parses a string into a TranspileMode
// Only "ast" is valid. Legacy and hybrid modes have been removed.
func ParseMode(s string) (TranspileMode, error) {
	switch s {
	case "ast":
		return ModeAST, nil
	case "legacy", "hybrid":
		return 0, fmt.Errorf("mode %q is no longer supported - only 'ast' mode is available", s)
	default:
		return 0, fmt.Errorf("invalid transpile mode: %q (must be 'ast')", s)
	}
}
