package refactor

import (
	"bytes"
	"fmt"
	"go/ast"
	goparser "go/parser"
	"go/token"

	dingoast "github.com/MadAppGang/dingo/pkg/ast"
	"github.com/MadAppGang/dingo/pkg/lint/analyzer"
)

// TypeSwitchDetector implements PatternDetector for R003: prefer-match-type
//
// Detects Go type switch statements and suggests transforming them to Dingo
// match expressions with type patterns.
//
// Pattern detected:
//
//	switch v := x.(type) {
//	case string:
//	    handleString(v)
//	case int:
//	    handleInt(v)
//	default:
//	    handleOther(v)
//	}
//
// Suggested refactoring:
//
//	match x {
//	    string => handleString(x)
//	    int    => handleInt(x)
//	    _      => handleOther(x)
//	}
type TypeSwitchDetector struct{}

// Code implements PatternDetector
func (d *TypeSwitchDetector) Code() string {
	return "R003"
}

// Name implements PatternDetector
func (d *TypeSwitchDetector) Name() string {
	return "prefer-match-type"
}

// Doc implements PatternDetector
func (d *TypeSwitchDetector) Doc() string {
	return "Suggests using match expression with type patterns instead of type switch"
}

// Detect implements PatternDetector
//
// Walks the AST looking for type switch statements (ast.TypeSwitchStmt).
// For each type switch found, generates a diagnostic with a Fix that transforms
// it to a Dingo match expression.
func (d *TypeSwitchDetector) Detect(fset *token.FileSet, file *dingoast.File, src []byte) []analyzer.Diagnostic {
	// Use Go's standard parser to get full AST with function bodies
	goFile, err := goparser.ParseFile(fset, "", src, goparser.ParseComments)
	if err != nil {
		return nil
	}

	var diagnostics []analyzer.Diagnostic

	// Walk AST looking for type switches
	ast.Inspect(goFile, func(n ast.Node) bool {
		typeSwitch, ok := n.(*ast.TypeSwitchStmt)
		if !ok {
			return true
		}

		// Extract type switch components
		info := d.extractTypeSwitchInfo(typeSwitch)
		if info == nil {
			// Not a valid type switch pattern (e.g., missing assignment)
			return true
		}

		// Generate the match expression transformation
		matchExpr := d.generateMatchExpression(info, src)

		// Create diagnostic with fix
		diagnostic := analyzer.Diagnostic{
			Pos:      fset.Position(typeSwitch.Pos()),
			End:      fset.Position(typeSwitch.End()),
			Message:  "Consider using match expression with type patterns instead of type switch",
			Severity: analyzer.SeverityHint,
			Code:     "R003",
			Category: "refactor",
			Fixes: []analyzer.Fix{
				{
					Title:       "Convert to match expression",
					IsPreferred: true,
					Edits: []analyzer.TextEdit{
						{
							Pos:     fset.Position(typeSwitch.Pos()),
							End:     fset.Position(typeSwitch.End()),
							NewText: matchExpr,
						},
					},
				},
			},
		}

		diagnostics = append(diagnostics, diagnostic)
		return true
	})

	return diagnostics
}

// typeSwitchInfo holds extracted information from a type switch statement
type typeSwitchInfo struct {
	scrutineeVar string           // Variable being switched on (e.g., "x")
	assignedVar  string           // Variable assigned in switch (e.g., "v"), can be ""
	cases        []typeSwitchCase // Case clauses
	hasDefault   bool             // Whether there's a default case
	defaultBody  string           // Body of default case
	entireSwitch string           // Entire switch statement source
}

// typeSwitchCase represents a single case in the type switch
type typeSwitchCase struct {
	typeNames []string // Type names in the case (can be multiple)
	body      string   // Body of the case
}

// extractTypeSwitchInfo extracts information from a type switch statement
func (d *TypeSwitchDetector) extractTypeSwitchInfo(typeSwitch *ast.TypeSwitchStmt) *typeSwitchInfo {
	info := &typeSwitchInfo{}

	// Extract the type assertion from the Assign field
	// Pattern: switch v := x.(type) or switch x.(type)
	assign, ok := typeSwitch.Assign.(*ast.AssignStmt)
	if !ok {
		return nil
	}

	if len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
		return nil
	}

	// Extract assigned variable (can be blank "_")
	if ident, ok := assign.Lhs[0].(*ast.Ident); ok {
		info.assignedVar = ident.Name
	}

	// Extract scrutinee from type assertion: x.(type)
	typeAssert, ok := assign.Rhs[0].(*ast.TypeAssertExpr)
	if !ok {
		return nil
	}

	if ident, ok := typeAssert.X.(*ast.Ident); ok {
		info.scrutineeVar = ident.Name
	} else {
		// Could be more complex expression, we'll skip for now
		return nil
	}

	// Extract cases from the switch body
	if typeSwitch.Body == nil {
		return nil
	}

	for _, stmt := range typeSwitch.Body.List {
		caseClause, ok := stmt.(*ast.CaseClause)
		if !ok {
			continue
		}

		if caseClause.List == nil {
			// Default case
			info.hasDefault = true
			// We'll extract the body later when generating the match
		} else {
			// Regular type case
			var typeNames []string
			for _, expr := range caseClause.List {
				typeName := d.extractTypeName(expr)
				if typeName != "" {
					typeNames = append(typeNames, typeName)
				}
			}

			if len(typeNames) > 0 {
				info.cases = append(info.cases, typeSwitchCase{
					typeNames: typeNames,
				})
			}
		}
	}

	return info
}

// extractTypeName extracts the type name from a case expression
func (d *TypeSwitchDetector) extractTypeName(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		// Handle qualified types like pkg.Type
		if ident, ok := e.X.(*ast.Ident); ok {
			return ident.Name + "." + e.Sel.Name
		}
	case *ast.StarExpr:
		// Handle pointer types like *Type
		return "*" + d.extractTypeName(e.X)
	case *ast.ArrayType:
		// Handle slice/array types like []Type
		if e.Len == nil {
			return "[]" + d.extractTypeName(e.Elt)
		}
	}
	return ""
}

// generateMatchExpression generates the match expression from type switch info
func (d *TypeSwitchDetector) generateMatchExpression(info *typeSwitchInfo, src []byte) string {
	var buf bytes.Buffer

	// Start match expression
	fmt.Fprintf(&buf, "match %s {\n", info.scrutineeVar)

	// Generate arms for each case
	// We need to extract the actual body from source since we're doing AST walking
	// For now, we'll generate a placeholder body
	for _, c := range info.cases {
		for _, typeName := range c.typeNames {
			// Align type names for readability
			fmt.Fprintf(&buf, "\t%s => /* TODO: extract body */\n", typeName)
		}
	}

	// Generate default case as wildcard
	if info.hasDefault {
		fmt.Fprintf(&buf, "\t_ => /* TODO: extract default body */\n")
	}

	buf.WriteString("}")
	return buf.String()
}

// Note: The current implementation generates TODO placeholders for case bodies.
// A complete implementation would need to:
// 1. Extract the actual case body source code
// 2. Replace references to the assigned variable (e.g., "v") with the scrutinee variable
// 3. Handle return statements appropriately
// 4. Preserve formatting and comments
//
// This is a skeleton implementation demonstrating the pattern detection logic.
// The full implementation would require more sophisticated source code extraction
// and transformation, potentially using go/printer or direct source byte manipulation.
