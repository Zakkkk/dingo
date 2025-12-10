package analyzer

import (
	"fmt"
	"go/token"
	"strings"

	dingoast "github.com/MadAppGang/dingo/pkg/ast"
)

// PatternAnalyzer checks pattern matching validity
// D005: Detects when a pattern uses an undefined enum variant
type PatternAnalyzer struct{}

func (a *PatternAnalyzer) Name() string {
	return "pattern-validity"
}

func (a *PatternAnalyzer) Doc() string {
	return "Detects when a pattern uses an undefined enum variant"
}

func (a *PatternAnalyzer) Category() string {
	return "correctness"
}

func (a *PatternAnalyzer) Run(fset *token.FileSet, file *dingoast.File, src []byte) []Diagnostic {
	var diagnostics []Diagnostic

	// First pass: collect all enum declarations
	enumVariants := make(map[string][]string) // enumName -> []variantName
	for _, node := range file.DingoNodes {
		if enumDecl, ok := node.(*dingoast.EnumDecl); ok {
			enumName := enumDecl.Name.Name
			variants := make([]string, len(enumDecl.Variants))
			for i, v := range enumDecl.Variants {
				variants[i] = v.Name.Name
			}
			enumVariants[enumName] = variants
		}
	}

	// Second pass: check all match expressions
	for _, node := range file.DingoNodes {
		if exprWrapper, ok := node.(*dingoast.ExprWrapper); ok {
			if matchExpr, ok := exprWrapper.DingoExpr.(*dingoast.MatchExpr); ok {
				diags := a.checkMatchExpr(fset, matchExpr, enumVariants)
				diagnostics = append(diagnostics, diags...)
			}
		}
	}

	return diagnostics
}

// checkMatchExpr validates all patterns in a match expression
func (a *PatternAnalyzer) checkMatchExpr(
	fset *token.FileSet,
	matchExpr *dingoast.MatchExpr,
	enumVariants map[string][]string,
) []Diagnostic {
	var diagnostics []Diagnostic

	for _, arm := range matchExpr.Arms {
		diags := a.checkPattern(fset, arm.Pattern, enumVariants)
		diagnostics = append(diagnostics, diags...)
	}

	return diagnostics
}

// checkPattern recursively validates a pattern
func (a *PatternAnalyzer) checkPattern(
	fset *token.FileSet,
	pattern dingoast.Pattern,
	enumVariants map[string][]string,
) []Diagnostic {
	var diagnostics []Diagnostic

	switch p := pattern.(type) {
	case *dingoast.ConstructorPattern:
		// Check if this is a known enum variant
		diags := a.checkConstructorPattern(fset, p, enumVariants)
		diagnostics = append(diagnostics, diags...)

		// Recursively check nested patterns in constructor params
		for _, param := range p.Params {
			diags := a.checkPattern(fset, param, enumVariants)
			diagnostics = append(diagnostics, diags...)
		}

	case *dingoast.TuplePattern:
		// Recursively check each element in tuple
		for _, elem := range p.Elements {
			diags := a.checkPattern(fset, elem, enumVariants)
			diagnostics = append(diagnostics, diags...)
		}

	case *dingoast.VariablePattern, *dingoast.WildcardPattern, *dingoast.LiteralPattern:
		// These don't reference enum variants, no validation needed

	// Handle other pattern types from pattern.go
	case *dingoast.OrPattern:
		// Recursively check left and right sides of or pattern
		diags := a.checkPattern(fset, p.Left, enumVariants)
		diagnostics = append(diagnostics, diags...)
		diags = a.checkPattern(fset, p.Right, enumVariants)
		diagnostics = append(diagnostics, diags...)

	case *dingoast.SlicePattern:
		// Recursively check each element in slice pattern
		for _, elem := range p.Elements {
			diags := a.checkPattern(fset, elem, enumVariants)
			diagnostics = append(diagnostics, diags...)
		}
	}

	return diagnostics
}

// checkConstructorPattern validates a constructor pattern against known enum variants
func (a *PatternAnalyzer) checkConstructorPattern(
	fset *token.FileSet,
	pattern *dingoast.ConstructorPattern,
	enumVariants map[string][]string,
) []Diagnostic {
	var diagnostics []Diagnostic

	// Parse constructor name - could be:
	// - Simple: "Some", "None", "Ok", "Err"
	// - Qualified: "Status_Active", "EnumName_Variant"
	name := pattern.Name

	// Check for qualified names (EnumName_Variant)
	if strings.Contains(name, "_") {
		parts := strings.Split(name, "_")
		if len(parts) == 2 {
			enumName := parts[0]
			variantName := parts[1]

			// Check if enum exists
			if variants, exists := enumVariants[enumName]; exists {
				// Check if variant exists in this enum
				if !containsString(variants, variantName) {
					diagnostics = append(diagnostics, Diagnostic{
						Pos:      fset.Position(pattern.NamePos),
						End:      fset.Position(pattern.End()),
						Message:  fmt.Sprintf("undefined variant '%s' for enum '%s' (available: %s)", variantName, enumName, strings.Join(variants, ", ")),
						Severity: SeverityWarning,
						Code:     "D005",
						Category: "correctness",
					})
				}
			}
			// Note: If enum doesn't exist, it might be from dgo package (Option, Result)
			// We don't report errors for those - they're handled by the Go type checker
		}
	} else {
		// Simple name - could be builtin (Some, None, Ok, Err) or custom enum variant
		// For custom enums without package qualification, we need to check all known enums
		// to see if there's a variant with this name in any of them
		// However, we only report an error if the variant is clearly meant for a specific
		// enum but doesn't exist. Simple names like "Some" and "None" are assumed to be
		// from the dgo.Option type and are validated by the Go compiler.

		// We can enhance this later with type information to detect mismatches,
		// but for the MVP, we focus on qualified names only.
	}

	return diagnostics
}

// containsString checks if a string exists in a slice
func containsString(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
