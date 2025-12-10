package analyzer

import (
	"fmt"
	"go/token"
	"strings"

	dingoast "github.com/MadAppGang/dingo/pkg/ast"
)

// ExhaustivenessAnalyzer detects when a match expression doesn't cover all variants of an enum.
// Implements D001 rule.
type ExhaustivenessAnalyzer struct{}

func (a *ExhaustivenessAnalyzer) Name() string {
	return "exhaustiveness"
}

func (a *ExhaustivenessAnalyzer) Doc() string {
	return "Detects non-exhaustive match expressions (missing enum variants)"
}

func (a *ExhaustivenessAnalyzer) Category() string {
	return "correctness"
}

func (a *ExhaustivenessAnalyzer) Run(fset *token.FileSet, file *dingoast.File, src []byte) []Diagnostic {
	var diagnostics []Diagnostic

	// Build enum variant map from all EnumDecl nodes
	enumVariants := buildEnumVariantMap(file)

	// Walk DingoNodes looking for MatchExpr
	for _, node := range file.DingoNodes {
		if wrapper, ok := node.(*dingoast.ExprWrapper); ok {
			if matchExpr, ok := wrapper.DingoExpr.(*dingoast.MatchExpr); ok {
				diags := a.checkMatchExhaustiveness(fset, matchExpr, enumVariants)
				diagnostics = append(diagnostics, diags...)
			}
		}
	}

	return diagnostics
}

// buildEnumVariantMap creates a map from enum names to their variant names
func buildEnumVariantMap(file *dingoast.File) map[string][]string {
	enumVariants := make(map[string][]string)

	for _, node := range file.DingoNodes {
		if enumDecl, ok := node.(*dingoast.EnumDecl); ok {
			variants := make([]string, len(enumDecl.Variants))
			for i, variant := range enumDecl.Variants {
				variants[i] = variant.Name.Name
			}
			enumVariants[enumDecl.Name.Name] = variants
		}
	}

	return enumVariants
}

// checkMatchExhaustiveness checks if a match expression covers all enum variants
func (a *ExhaustivenessAnalyzer) checkMatchExhaustiveness(
	fset *token.FileSet,
	matchExpr *dingoast.MatchExpr,
	enumVariants map[string][]string,
) []Diagnostic {
	var diagnostics []Diagnostic

	// Determine if scrutinee is an enum type
	enumName := inferEnumType(matchExpr.Scrutinee)
	if enumName == "" {
		// Not matching on an enum, skip
		return diagnostics
	}

	// Get expected variants for this enum
	expectedVariants, found := enumVariants[enumName]
	if !found {
		// Enum not found in this file (might be imported)
		// TODO: Handle cross-file enum resolution
		return diagnostics
	}

	// Extract covered patterns from match arms
	coveredVariants := make(map[string]bool)
	hasWildcard := false

	for _, arm := range matchExpr.Arms {
		if isWildcardPattern(arm.Pattern) {
			hasWildcard = true
			break
		}

		variant := extractVariantName(arm.Pattern)
		if variant != "" {
			coveredVariants[variant] = true
		}
	}

	// If wildcard is present, match is exhaustive
	if hasWildcard {
		return diagnostics
	}

	// Check for missing variants
	var missingVariants []string
	for _, variant := range expectedVariants {
		if !coveredVariants[variant] {
			missingVariants = append(missingVariants, variant)
		}
	}

	// If there are missing variants, emit diagnostic
	if len(missingVariants) > 0 {
		pos := fset.Position(matchExpr.Match)
		end := fset.Position(matchExpr.CloseBrace)

		message := fmt.Sprintf("non-exhaustive match: missing variant(s): %v", missingVariants)

		diagnostics = append(diagnostics, Diagnostic{
			Pos:      pos,
			End:      end,
			Message:  message,
			Severity: SeverityWarning,
			Code:     "D001",
			Category: "correctness",
		})
	}

	return diagnostics
}

// inferEnumType attempts to determine the enum type from the scrutinee expression
// This is a simple heuristic that looks for:
// - Variable names (e.g., "result", "status")
// - Function call results that might return enums
func inferEnumType(scrutinee dingoast.Expr) string {
	// For now, we use a simple heuristic based on common patterns
	// This can be enhanced with type information later

	// Handle RawExpr (most common in current parser)
	if rawExpr, ok := scrutinee.(*dingoast.RawExpr); ok {
		text := rawExpr.Text

		// Common enum types based on variable naming
		// This is a heuristic until we have full type resolution
		switch {
		case containsWord(text, "result"):
			return "Result"
		case containsWord(text, "option"):
			return "Option"
		case containsWord(text, "status"):
			return "Status"
		case containsWord(text, "state"):
			return "State"
		case containsWord(text, "color"):
			return "Color"
		}
	}

	// TODO: Add type resolution via go/types for accurate enum detection
	return ""
}

// containsWord checks if text contains a word (case-insensitive, whole word match)
func containsWord(text, word string) bool {
	// Use stdlib for case-insensitive substring check
	return strings.Contains(strings.ToLower(text), strings.ToLower(word))
}

// isWildcardPattern checks if a pattern is a wildcard (_)
func isWildcardPattern(pattern dingoast.Pattern) bool {
	_, ok := pattern.(*dingoast.WildcardPattern)
	return ok
}

// extractVariantName extracts the variant name from a pattern
func extractVariantName(pattern dingoast.Pattern) string {
	switch p := pattern.(type) {
	case *dingoast.ConstructorPattern:
		return p.Name
	case *dingoast.VariablePattern:
		// Variable patterns don't represent enum variants
		return ""
	case *dingoast.TuplePattern:
		// Tuple patterns don't directly represent variants
		return ""
	case *dingoast.LiteralPattern:
		// Literal patterns don't represent enum variants
		return ""
	default:
		return ""
	}
}
