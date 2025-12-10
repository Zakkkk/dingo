package analyzer

import (
	"go/ast"
	"go/token"

	dingoast "github.com/MadAppGang/dingo/pkg/ast"
)

// StyleAnalyzer checks for style-related issues
type StyleAnalyzer struct{}

func (a *StyleAnalyzer) Name() string     { return "style" }
func (a *StyleAnalyzer) Doc() string      { return "Checks for Dingo style preferences" }
func (a *StyleAnalyzer) Category() string { return "style" }

func (a *StyleAnalyzer) Run(fset *token.FileSet, file *dingoast.File, src []byte) []Diagnostic {
	var diagnostics []Diagnostic

	// D101: prefer-let - Use let instead of var for single assignment
	diagnostics = append(diagnostics, checkPreferLet(fset, file)...)

	// D102: prefer-match - Use match expression instead of type switch
	diagnostics = append(diagnostics, checkPreferMatch(fset, file)...)

	return diagnostics
}

// checkPreferLet checks for var declarations that should use let (D101)
// Pattern: var x = value (single assignment) → let x = value
func checkPreferLet(fset *token.FileSet, file *dingoast.File) []Diagnostic {
	var diagnostics []Diagnostic

	ast.Inspect(file.File, func(n ast.Node) bool {
		// Look for var declarations (GenDecl with VAR token)
		genDecl, ok := n.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.VAR {
			return true
		}

		// Check each spec in the declaration
		for _, spec := range genDecl.Specs {
			valueSpec, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}

			// Only suggest if there's exactly one value (single assignment)
			// Skip if:
			// - Multiple names: var a, b = 1, 2
			// - No values: var x int
			// - Multiple values without clear mapping
			if len(valueSpec.Values) != 1 || len(valueSpec.Names) != 1 {
				continue
			}

			// Check if this is a simple assignment (not a complex type declaration)
			// Skip if explicit type without initialization or complex scenarios
			if valueSpec.Type != nil && len(valueSpec.Values) == 0 {
				continue
			}

			name := valueSpec.Names[0]
			diagnostics = append(diagnostics, Diagnostic{
				Pos:      fset.Position(genDecl.Pos()),
				End:      fset.Position(genDecl.End()),
				Message:  "prefer 'let' over 'var' for single assignment",
				Severity: SeverityWarning,
				Code:     "D101",
				Category: "style",
				Related: []RelatedInfo{{
					Pos:     fset.Position(name.Pos()),
					Message: "variable declared here",
				}},
			})
		}

		return true
	})

	return diagnostics
}

// checkPreferMatch checks for type switches that should use match expressions (D102)
// Pattern: switch v := x.(type) { ... } → match x { ... }
func checkPreferMatch(fset *token.FileSet, file *dingoast.File) []Diagnostic {
	var diagnostics []Diagnostic

	ast.Inspect(file.File, func(n ast.Node) bool {
		// Look for type switch statements
		typeSwitch, ok := n.(*ast.TypeSwitchStmt)
		if !ok {
			return true
		}

		// Suggest match for all type switches
		// This is a style suggestion - match is more idiomatic in Dingo
		diagnostics = append(diagnostics, Diagnostic{
			Pos:      fset.Position(typeSwitch.Pos()),
			End:      fset.Position(typeSwitch.End()),
			Message:  "prefer 'match' expression over type switch for better readability",
			Severity: SeverityWarning,
			Code:     "D102",
			Category: "style",
			Related: []RelatedInfo{{
				Pos:     fset.Position(typeSwitch.Switch),
				Message: "type switch statement",
			}},
		})

		return true
	})

	return diagnostics
}
