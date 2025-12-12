package refactor

import (
	"bytes"
	"go/ast"
	goparser "go/parser"
	"go/token"

	dingoast "github.com/MadAppGang/dingo/pkg/ast"
	"github.com/MadAppGang/dingo/pkg/lint/analyzer"
)

// OkPatternDetector detects the val, ok := ...; if !ok pattern and suggests
// using match expressions with Option semantics.
//
// Example:
//
//	val, ok := someMap[key]
//	if !ok {
//	    val = defaultVal
//	}
//
// Suggests:
//
//	val := match someMap[key] {
//	    Some(v) => v
//	    None    => defaultVal
//	}
//
// Code: R007
type OkPatternDetector struct{}

func (d *OkPatternDetector) Code() string { return "R007" }
func (d *OkPatternDetector) Name() string { return "prefer-match-ok" }
func (d *OkPatternDetector) Doc() string {
	return "Suggests using match expression instead of val, ok := ...; if !ok pattern"
}

func (d *OkPatternDetector) Detect(fset *token.FileSet, file *dingoast.File, src []byte) []analyzer.Diagnostic {
	// Use Go's standard parser to get full AST with function bodies
	goFile, err := goparser.ParseFile(fset, "", src, goparser.ParseComments)
	if err != nil {
		return nil
	}

	var diagnostics []analyzer.Diagnostic

	// Walk the AST looking for assignment followed by if !ok
	ast.Inspect(goFile, func(n ast.Node) bool {
		// Look for assignment statements
		assignStmt, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}

		// Must be := (short variable declaration)
		if assignStmt.Tok != token.DEFINE {
			return true
		}

		// Must assign 2 values (val, ok)
		if len(assignStmt.Lhs) != 2 || len(assignStmt.Rhs) != 1 {
			return true
		}

		// Second identifier must be named 'ok'
		okIdent, ok := assignStmt.Lhs[1].(*ast.Ident)
		if !ok || okIdent.Name != "ok" {
			return true
		}

		// Get the value identifier
		valIdent, ok := assignStmt.Lhs[0].(*ast.Ident)
		if !ok {
			return true
		}

		// Right-hand side: what we're checking
		rhsExpr := assignStmt.Rhs[0]

		// Check if the next statement is if !ok
		nextStmt := findNextStatement(goFile, assignStmt)
		ifStmt, isIfStmt := nextStmt.(*ast.IfStmt)
		if !isIfStmt {
			return true
		}

		// Check if condition is !ok
		if !isNotOkCondition(ifStmt.Cond, okIdent.Name) {
			return true
		}

		// Try to extract the default value from the if body
		defaultVal := extractDefaultValue(ifStmt.Body, valIdent.Name)
		if defaultVal == "" {
			// Can't auto-fix if we can't determine default value
			// Still report but without fix
			diagnostics = append(diagnostics, analyzer.Diagnostic{
				Pos:      fset.Position(assignStmt.Pos()),
				End:      fset.Position(ifStmt.End()),
				Message:  "Consider using match expression for ok-pattern",
				Severity: analyzer.SeverityHint,
				Code:     "R007",
				Category: "refactor",
			})
			return true
		}

		// Generate the match expression
		rhsStr := exprToString(src, fset, rhsExpr)
		matchExpr := generateMatchOkFix(valIdent.Name, rhsStr, defaultVal)

		// Create diagnostic with auto-fix
		diagnostics = append(diagnostics, analyzer.Diagnostic{
			Pos:      fset.Position(assignStmt.Pos()),
			End:      fset.Position(ifStmt.End()),
			Message:  "Consider using match expression for ok-pattern",
			Severity: analyzer.SeverityHint,
			Code:     "R007",
			Category: "refactor",
			Fixes: []analyzer.Fix{{
				Title:       "Replace with match expression",
				IsPreferred: true,
				Edits: []analyzer.TextEdit{{
					Pos:     fset.Position(assignStmt.Pos()),
					End:     fset.Position(ifStmt.End()),
					NewText: matchExpr,
				}},
			}},
		})

		return true
	})

	return diagnostics
}

// Helper: findNextStatement finds the statement immediately following the given statement
func findNextStatement(file *ast.File, target ast.Stmt) ast.Stmt {
	var nextStmt ast.Stmt
	var foundTarget bool

	ast.Inspect(file, func(n ast.Node) bool {
		// Look for block statements (function bodies, if blocks, etc.)
		block, ok := n.(*ast.BlockStmt)
		if !ok {
			return true
		}

		// Search for target in this block's statement list
		for i, stmt := range block.List {
			if stmt == target {
				foundTarget = true
				// Return next statement if it exists
				if i+1 < len(block.List) {
					nextStmt = block.List[i+1]
				}
				return false
			}
		}

		return true
	})

	if foundTarget {
		return nextStmt
	}
	return nil
}

// Helper: isNotOkCondition checks if expression is !ok
func isNotOkCondition(expr ast.Expr, okName string) bool {
	unary, ok := expr.(*ast.UnaryExpr)
	if !ok || unary.Op != token.NOT {
		return false
	}

	ident, ok := unary.X.(*ast.Ident)
	return ok && ident.Name == okName
}

// Helper: extractDefaultValue tries to extract the default value from if !ok body
// Looks for patterns like:
//
//	val = defaultVal
//	return defaultVal
func extractDefaultValue(body *ast.BlockStmt, valName string) string {
	if body == nil || len(body.List) == 0 {
		return ""
	}

	// Check first statement
	firstStmt := body.List[0]

	// Pattern 1: val = defaultVal
	if assignStmt, ok := firstStmt.(*ast.AssignStmt); ok {
		if len(assignStmt.Lhs) == 1 && len(assignStmt.Rhs) == 1 {
			if ident, ok := assignStmt.Lhs[0].(*ast.Ident); ok && ident.Name == valName {
				return exprToStringSimple(assignStmt.Rhs[0])
			}
		}
	}

	// Pattern 2: return defaultVal (for single return)
	if retStmt, ok := firstStmt.(*ast.ReturnStmt); ok {
		if len(retStmt.Results) == 1 {
			return exprToStringSimple(retStmt.Results[0])
		}
	}

	return ""
}

// Helper: exprToString converts an expression to string using source
func exprToString(src []byte, fset *token.FileSet, expr ast.Expr) string {
	start := fset.Position(expr.Pos())
	end := fset.Position(expr.End())

	// Calculate byte offsets
	startOffset := 0
	endOffset := 0
	line := 1
	col := 1
	for i, b := range src {
		if line == start.Line && col == start.Column {
			startOffset = i
		}
		if line == end.Line && col == end.Column {
			endOffset = i
			break
		}
		if b == '\n' {
			line++
			col = 1
		} else {
			col++
		}
	}

	if startOffset < endOffset && endOffset <= len(src) {
		return string(src[startOffset:endOffset])
	}

	return exprToStringSimple(expr)
}

// Helper: exprToStringSimple converts simple expressions to string
func exprToStringSimple(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.BasicLit:
		return e.Value
	case *ast.CallExpr:
		// Simple function call
		if ident, ok := e.Fun.(*ast.Ident); ok {
			return ident.Name + "(...)"
		}
		return "expr"
	case *ast.UnaryExpr:
		return token.Token(e.Op).String() + exprToStringSimple(e.X)
	case *ast.BinaryExpr:
		return exprToStringSimple(e.X) + " " + e.Op.String() + " " + exprToStringSimple(e.Y)
	case *ast.SelectorExpr:
		return exprToStringSimple(e.X) + "." + e.Sel.Name
	default:
		return "expr"
	}
}

// Helper: generateMatchOkFix generates the match expression replacement
func generateMatchOkFix(valName, rhsExpr, defaultVal string) string {
	var buf bytes.Buffer

	buf.WriteString(valName)
	buf.WriteString(" := match ")
	buf.WriteString(rhsExpr)
	buf.WriteString(" {\n")
	buf.WriteString("\tSome(v) => v\n")
	buf.WriteString("\tNone    => ")
	buf.WriteString(defaultVal)
	buf.WriteString("\n}")

	return buf.String()
}
