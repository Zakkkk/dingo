package analyzer

import (
	"go/token"
	"unicode"

	dingoast "github.com/MadAppGang/dingo/pkg/ast"
)

// NamingAnalyzer checks for naming convention issues
type NamingAnalyzer struct{}

func (a *NamingAnalyzer) Name() string     { return "naming" }
func (a *NamingAnalyzer) Doc() string      { return "Checks for Dingo naming conventions" }
func (a *NamingAnalyzer) Category() string { return "style" }

func (a *NamingAnalyzer) Run(fset *token.FileSet, file *dingoast.File, src []byte) []Diagnostic {
	var diagnostics []Diagnostic

	// Walk Dingo-specific nodes
	for _, node := range file.DingoNodes {
		// D103: enum-naming - Enum variants should be PascalCase
		if enumDecl, ok := node.(*dingoast.EnumDecl); ok {
			diagnostics = append(diagnostics, checkEnumVariantNaming(fset, enumDecl)...)
		}

		// D104: lambda-param - Lambda parameters should be lowercase
		if wrapper, ok := node.(*dingoast.ExprWrapper); ok {
			if lambdaExpr, ok := wrapper.DingoExpr.(*dingoast.LambdaExpr); ok {
				diagnostics = append(diagnostics, checkLambdaParamNaming(fset, lambdaExpr)...)
			}
		}
	}

	return diagnostics
}

// checkEnumVariantNaming checks that enum variants follow PascalCase (D103)
func checkEnumVariantNaming(fset *token.FileSet, enumDecl *dingoast.EnumDecl) []Diagnostic {
	var diagnostics []Diagnostic

	for _, variant := range enumDecl.Variants {
		name := variant.Name.Name

		// Check if variant name is PascalCase
		if !isPascalCase(name) {
			diagnostics = append(diagnostics, Diagnostic{
				Pos:      fset.Position(variant.Name.Pos()),
				End:      fset.Position(variant.Name.End()),
				Message:  "enum variant '" + name + "' should use PascalCase",
				Severity: SeverityWarning,
				Code:     "D103",
				Category: "style",
				Related: []RelatedInfo{{
					Pos:     fset.Position(enumDecl.Name.Pos()),
					Message: "in enum '" + enumDecl.Name.Name + "'",
				}},
			})
		}
	}

	return diagnostics
}

// checkLambdaParamNaming checks that lambda parameters are lowercase (D104)
func checkLambdaParamNaming(fset *token.FileSet, lambdaExpr *dingoast.LambdaExpr) []Diagnostic {
	var diagnostics []Diagnostic

	for _, param := range lambdaExpr.Params {
		name := param.Name

		// Check if parameter name starts with uppercase
		// Lambda parameters should be lowercase or camelCase
		if len(name) > 0 && unicode.IsUpper(rune(name[0])) {
			diagnostics = append(diagnostics, Diagnostic{
				Pos:      fset.Position(lambdaExpr.Pos()),
				End:      fset.Position(lambdaExpr.End()),
				Message:  "lambda parameter '" + name + "' should start with lowercase letter",
				Severity: SeverityWarning,
				Code:     "D104",
				Category: "style",
				Related: []RelatedInfo{{
					Pos:     fset.Position(lambdaExpr.Pos()),
					Message: "in lambda expression",
				}},
			})
		}
	}

	return diagnostics
}

// isPascalCase checks if a string follows PascalCase convention
// PascalCase: First letter uppercase, no underscores, camelCase after
func isPascalCase(s string) bool {
	if len(s) == 0 {
		return false
	}

	// First character must be uppercase
	if !unicode.IsUpper(rune(s[0])) {
		return false
	}

	// Should not contain underscores (Go identifier style)
	for _, r := range s {
		if r == '_' {
			return false
		}
	}

	return true
}
