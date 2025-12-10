package refactor

import (
	"go/ast"
	"go/token"
	"strings"

	dingoast "github.com/MadAppGang/dingo/pkg/ast"
	"github.com/MadAppGang/dingo/pkg/lint/analyzer"
)

// ErrorPropDetector detects the pattern:
//
//	result, err := someCall()
//	if err != nil {
//	    return ..., err
//	}
//
// and suggests using the ? operator instead:
//
//	result := someCall()?
//
// This is refactoring rule R001: prefer-error-prop
type ErrorPropDetector struct{}

// Code implements PatternDetector
func (d *ErrorPropDetector) Code() string {
	return "R001"
}

// Name implements PatternDetector
func (d *ErrorPropDetector) Name() string {
	return "prefer-error-prop"
}

// Doc implements PatternDetector
func (d *ErrorPropDetector) Doc() string {
	return "Suggests using ? operator for error propagation instead of manual if err != nil checks"
}

// Detect implements PatternDetector
//
// Walks the Go AST looking for the pattern:
// 1. Assignment statement declaring "err" variable
// 2. Immediately followed by "if err != nil" check
// 3. If body contains return statement that returns err
//
// Generates a diagnostic with a Fix that replaces the pattern with ? operator.
func (d *ErrorPropDetector) Detect(fset *token.FileSet, file *dingoast.File, src []byte) []analyzer.Diagnostic {
	if file == nil || file.File == nil {
		return nil
	}

	var diagnostics []analyzer.Diagnostic

	// Walk the Go AST to find function bodies
	ast.Inspect(file.File, func(n ast.Node) bool {
		funcDecl, ok := n.(*ast.FuncDecl)
		if !ok || funcDecl.Body == nil {
			return true
		}

		// Scan statements in function body looking for pattern
		stmts := funcDecl.Body.List
		for i := 0; i < len(stmts)-1; i++ {
			// Check if current statement is an assignment with err
			assignStmt, ok := stmts[i].(*ast.AssignStmt)
			if !ok {
				continue
			}

			// Check if assignment declares err variable
			errIdx := findErrInAssignment(assignStmt)
			if errIdx == -1 {
				continue
			}

			// Check if next statement is "if err != nil"
			ifStmt, ok := stmts[i+1].(*ast.IfStmt)
			if !ok {
				continue
			}

			if !isErrNilCheck(ifStmt) {
				continue
			}

			// Check if if body returns err
			if !returnsErr(ifStmt.Body) {
				continue
			}

			// CRITICAL: Check if return values are zero values
			// Only suggest R001 if error branch returns zero/nil values
			// Otherwise, refactoring would change semantics
			if !returnsZeroValuesOnly(ifStmt.Body) {
				continue // Skip - non-zero returns would change semantics
			}

			// Found the pattern! Generate diagnostic with fix
			diag := d.generateDiagnostic(fset, assignStmt, ifStmt, errIdx, src)
			diagnostics = append(diagnostics, diag)
		}

		return true
	})

	return diagnostics
}

// findErrInAssignment finds the index of "err" in assignment left-hand side
// Returns -1 if err not found or not a define assignment (:=)
func findErrInAssignment(stmt *ast.AssignStmt) int {
	if stmt.Tok != token.DEFINE {
		return -1 // Only detect := assignments (not = reassignments)
	}

	for i, lhs := range stmt.Lhs {
		if ident, ok := lhs.(*ast.Ident); ok {
			if ident.Name == "err" {
				return i
			}
		}
	}

	return -1
}

// isErrNilCheck returns true if the if condition is "err != nil"
func isErrNilCheck(ifStmt *ast.IfStmt) bool {
	binExpr, ok := ifStmt.Cond.(*ast.BinaryExpr)
	if !ok {
		return false
	}

	if binExpr.Op != token.NEQ {
		return false
	}

	// Check if one side is "err" and other is "nil"
	leftIsErr := isIdentNamed(binExpr.X, "err")
	rightIsNil := isNil(binExpr.Y)

	if leftIsErr && rightIsNil {
		return true
	}

	// Also check reverse: nil != err
	leftIsNil := isNil(binExpr.X)
	rightIsErr := isIdentNamed(binExpr.Y, "err")

	return leftIsNil && rightIsErr
}

// returnsErr returns true if the block contains a return statement that returns err
func returnsErr(block *ast.BlockStmt) bool {
	if block == nil {
		return false
	}

	// Check all statements in the block
	for _, stmt := range block.List {
		if retStmt, ok := stmt.(*ast.ReturnStmt); ok {
			// Check if any return value is "err"
			for _, result := range retStmt.Results {
				if isIdentNamed(result, "err") {
					return true
				}
			}
		}
	}

	return false
}

// isIdentNamed returns true if expr is an identifier with the given name
func isIdentNamed(expr ast.Expr, name string) bool {
	ident, ok := expr.(*ast.Ident)
	return ok && ident.Name == name
}

// isNil returns true if expr is the nil identifier
func isNil(expr ast.Expr) bool {
	ident, ok := expr.(*ast.Ident)
	return ok && ident.Name == "nil"
}

// returnsZeroValuesOnly checks if the return statement only returns zero values
// (nil, 0, false, "", empty struct literal, etc.) plus the error
// This ensures that the ? operator refactoring preserves semantics
func returnsZeroValuesOnly(block *ast.BlockStmt) bool {
	if block == nil {
		return false
	}

	// Find the return statement
	for _, stmt := range block.List {
		retStmt, ok := stmt.(*ast.ReturnStmt)
		if !ok {
			continue
		}

		// Check each return value except the last one (which should be err)
		for i, result := range retStmt.Results {
			// Last result should be err
			if i == len(retStmt.Results)-1 {
				continue
			}

			// Check if this result is a zero value
			if !isZeroValue(result) {
				return false // Non-zero value found
			}
		}

		return true
	}

	return false
}

// isZeroValue checks if an expression represents a zero value
func isZeroValue(expr ast.Expr) bool {
	switch e := expr.(type) {
	case *ast.Ident:
		// nil, false
		return e.Name == "nil" || e.Name == "false"

	case *ast.BasicLit:
		// 0, "", etc.
		return e.Value == "0" || e.Value == `""` || e.Value == "0.0"

	case *ast.CompositeLit:
		// Empty struct/array literal: Type{} or {}
		return len(e.Elts) == 0

	case *ast.UnaryExpr:
		// Address of zero value: &Type{}
		if e.Op == token.AND {
			if comp, ok := e.X.(*ast.CompositeLit); ok {
				return len(comp.Elts) == 0
			}
		}
		return false

	default:
		// Unknown expression type - conservatively assume non-zero
		return false
	}
}

// generateDiagnostic creates a diagnostic with a fix for the error propagation pattern
func (d *ErrorPropDetector) generateDiagnostic(
	fset *token.FileSet,
	assignStmt *ast.AssignStmt,
	ifStmt *ast.IfStmt,
	errIdx int,
	src []byte,
) analyzer.Diagnostic {
	assignPos := fset.Position(assignStmt.Pos())
	ifEnd := fset.Position(ifStmt.End())

	// Build the message
	message := "Consider using ? operator for error propagation"

	// Generate the fix
	fix := d.generateFix(fset, assignStmt, ifStmt, errIdx, src)

	return analyzer.Diagnostic{
		Pos:      assignPos,
		End:      ifEnd,
		Message:  message,
		Severity: analyzer.SeverityHint,
		Code:     d.Code(),
		Category: "refactor",
		Fixes:    []analyzer.Fix{fix},
	}
}

// generateFix creates a Fix that transforms the error propagation pattern
//
// Transforms:
//
//	result, err := someCall()
//	if err != nil {
//	    return defaultVal, err
//	}
//
// Into:
//
//	result := someCall()?
func (d *ErrorPropDetector) generateFix(
	fset *token.FileSet,
	assignStmt *ast.AssignStmt,
	ifStmt *ast.IfStmt,
	errIdx int,
	src []byte,
) analyzer.Fix {
	// Extract the call expression (right-hand side of assignment)
	var callExpr string
	if errIdx < len(assignStmt.Rhs) {
		rhsStart := fset.Position(assignStmt.Rhs[errIdx].Pos())
		rhsEnd := fset.Position(assignStmt.Rhs[errIdx].End())
		callExpr = string(src[rhsStart.Offset:rhsEnd.Offset])
	} else if len(assignStmt.Rhs) == 1 {
		// Common pattern: result, err := call() where call returns (T, error)
		rhsStart := fset.Position(assignStmt.Rhs[0].Pos())
		rhsEnd := fset.Position(assignStmt.Rhs[0].End())
		callExpr = string(src[rhsStart.Offset:rhsEnd.Offset])
	}

	// Extract the result variable name(s) (all LHS except err)
	var resultVars []string
	for i, lhs := range assignStmt.Lhs {
		if i == errIdx {
			continue // Skip err
		}
		if ident, ok := lhs.(*ast.Ident); ok {
			resultVars = append(resultVars, ident.Name)
		}
	}

	// Build the replacement text
	var newText string
	if len(resultVars) > 0 {
		// Single assignment: result := call()?
		newText = strings.Join(resultVars, ", ") + " := " + callExpr + "?"
	} else {
		// No result variables (only err): call()?
		newText = callExpr + "?"
	}

	assignStart := fset.Position(assignStmt.Pos())
	ifEnd := fset.Position(ifStmt.End())

	return analyzer.Fix{
		Title:       "Use ? operator",
		IsPreferred: true,
		Edits: []analyzer.TextEdit{
			{
				Pos:     assignStart,
				End:     ifEnd,
				NewText: newText,
			},
		},
	}
}
