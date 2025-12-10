package analyzer

import (
	"go/token"

	dingoast "github.com/MadAppGang/dingo/pkg/ast"
)

// Severity levels - all are advisory (warnings)
type Severity int

const (
	SeverityWarning Severity = iota // Default for all rules (zero value)
	SeverityInfo
	SeverityHint
)

// String returns the string representation of the severity level
func (s Severity) String() string {
	switch s {
	case SeverityWarning:
		return "warning"
	case SeverityInfo:
		return "info"
	case SeverityHint:
		return "hint"
	default:
		return "warning" // Default unknown to warning
	}
}

// Diagnostic represents a linting issue
type Diagnostic struct {
	Pos      token.Position
	End      token.Position
	Message  string
	Severity Severity
	Code     string        // e.g., "D001", "R001"
	Category string        // "correctness", "style", or "refactor"
	Related  []RelatedInfo // Related information for context
	Fixes    []Fix         // Suggested fixes (for refactoring)
}

// RelatedInfo provides additional context for a diagnostic
type RelatedInfo struct {
	Pos     token.Position
	Message string
}

// Fix represents an automated fix for a diagnostic
type Fix struct {
	Title       string     // Human-readable description (e.g., "Use ? operator")
	Edits       []TextEdit // Edits to apply
	IsPreferred bool       // Show as primary action in IDE
}

// TextEdit represents a text replacement
type TextEdit struct {
	Pos     token.Position
	End     token.Position
	NewText string
}

// Analyzer is the interface for all Dingo analyzers.
//
// Implementations include correctness rules (D0xx), style rules (D1xx),
// and refactoring suggestions (R0xx via RefactoringAnalyzer).
//
// CRITICAL - CLAUDE.md COMPLIANCE:
// The src parameter is ONLY for position offset calculations and error reporting.
// DO NOT use bytes.Index, strings.Contains, or regexp on src.
// ALL analysis MUST be performed on the parsed AST (file parameter).
//
// Forbidden patterns (see CLAUDE.md):
//   - bytes.Index(), strings.Index(), strings.Contains()
//   - regexp.MustCompile(), regexp.Match(), regexp.Find*()
//   - Character scanning: for i := 0; i < len(src); i++
//
// Correct approach:
//   Source → pkg/tokenizer/ → []Token → pkg/parser/ → AST → Analyzer
//
// See CLAUDE.md for complete list of forbidden patterns and architecture rules.
type Analyzer interface {
	Name() string     // Unique identifier (e.g., "exhaustiveness")
	Doc() string      // Human-readable description
	Category() string // "correctness", "style", or "refactor"

	// Run analyzes the given Dingo AST and returns diagnostics.
	//
	// Parameters:
	//   - fset: Token file set for position resolution
	//   - file: Parsed Dingo AST (ALL analysis must use this)
	//   - src: Raw source bytes (ONLY for offset calculations, NOT parsing)
	Run(fset *token.FileSet, file *dingoast.File, src []byte) []Diagnostic
}
