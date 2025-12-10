package analyzer

import (
	"go/ast"
	"go/token"
	"go/types"

	dingoast "github.com/MadAppGang/dingo/pkg/ast"
)

// ErrorPropAnalyzer validates that the error propagation operator (?)
// is only used on expressions that return Result[T,E] or (T, error).
//
// Rule: D002 - invalid-error-prop
// Category: correctness
type ErrorPropAnalyzer struct{}

func (a *ErrorPropAnalyzer) Name() string {
	return "invalid-error-prop"
}

func (a *ErrorPropAnalyzer) Doc() string {
	return "Validates that ? operator is only used on Result[T,E] or (T, error) types"
}

func (a *ErrorPropAnalyzer) Category() string {
	return "correctness"
}

func (a *ErrorPropAnalyzer) Run(fset *token.FileSet, file *dingoast.File, src []byte) []Diagnostic {
	var diagnostics []Diagnostic

	// Walk the Dingo AST looking for ErrorPropExpr nodes
	for _, node := range file.DingoNodes {
		// Check if this is an ExprWrapper containing an ErrorPropExpr
		if wrapper, ok := node.(*dingoast.ExprWrapper); ok {
			if errorProp, ok := wrapper.DingoExpr.(*dingoast.ErrorPropExpr); ok {
				// Validate the operand type
				if diag := a.validateErrorProp(fset, errorProp, file); diag != nil {
					diagnostics = append(diagnostics, *diag)
				}
			}
		}
	}

	// Also need to recursively search within match expressions, lambdas, etc.
	diagnostics = append(diagnostics, a.findErrorPropInAST(fset, file)...)

	return diagnostics
}

// validateErrorProp checks if the operand of ? is a valid type
func (a *ErrorPropAnalyzer) validateErrorProp(fset *token.FileSet, errorProp *dingoast.ErrorPropExpr, file *dingoast.File) *Diagnostic {
	// Strategy:
	// 1. If ErrorProp already has ResultType/ErrorType filled by type checker, use that
	// 2. Otherwise, try to infer from AST structure
	// 3. For MVP, we'll use heuristic-based detection since full type checking may not be available

	// If type information is available (from type checker), use it
	if errorProp.ResultType != nil && errorProp.ErrorType != nil {
		// Type checker has validated this - trust it
		return nil
	}

	// Heuristic approach: Try to determine if operand looks like it could be Result/error
	// This is a conservative check - we'll check if the operand is:
	// 1. A function call (could return Result[T,E] or (T, error))
	// 2. A variable (harder to verify without type info)
	// 3. Another Dingo expression (could return Result[T,E])

	// For now, we'll emit a warning if we can't verify the type
	// In a full implementation, this would integrate with go/types

	// Try to get the operand as a go/ast expression
	operand := errorProp.Operand
	if operand == nil {
		return &Diagnostic{
			Pos:      fset.Position(errorProp.Question),
			End:      fset.Position(errorProp.Question + 1),
			Message:  "? operator requires an operand",
			Severity: SeverityWarning,
			Code:     "D002",
			Category: "correctness",
		}
	}

	// Check if operand is a valid expression type
	// For MVP: We'll trust that if the operand exists, it's likely valid
	// A full implementation would use go/types to verify:
	// - operand type is Result[T, E]
	// - OR operand type is (T, error) tuple
	// - Otherwise: emit diagnostic

	// Try to detect obviously wrong cases:
	// - Literal values (can't be Result or error tuple)
	if isLiteralExpr(operand) {
		return &Diagnostic{
			Pos:      fset.Position(operand.Pos()),
			End:      fset.Position(errorProp.Question + 1),
			Message:  "? operator cannot be used on literal values (expected Result[T,E] or (T, error))",
			Severity: SeverityWarning,
			Code:     "D002",
			Category: "correctness",
		}
	}

	// For now, assume other cases are valid (conservative approach)
	// TODO: Integrate with go/types for full validation
	return nil
}

// isLiteralExpr checks if an expression is a literal value
func isLiteralExpr(expr dingoast.Expr) bool {
	if expr == nil {
		return false
	}

	// Check for RawExpr containing literals
	if raw, ok := expr.(*dingoast.RawExpr); ok {
		text := raw.Text
		// Basic heuristics for literals
		if len(text) > 0 {
			first := text[0]
			// String literals
			if first == '"' || first == '`' {
				return true
			}
			// Numeric literals
			if first >= '0' && first <= '9' {
				return true
			}
			// Boolean literals
			if text == "true" || text == "false" {
				return true
			}
			// Nil literal
			if text == "nil" {
				return true
			}
		}
	}

	return false
}

// findErrorPropInAST recursively searches the Go AST for embedded ErrorPropExpr
func (a *ErrorPropAnalyzer) findErrorPropInAST(fset *token.FileSet, file *dingoast.File) []Diagnostic {
	var diagnostics []Diagnostic

	// Use ast.Inspect to walk the Go AST
	ast.Inspect(file.File, func(n ast.Node) bool {
		// Look for comment markers that indicate Dingo expressions
		// This is a fallback for expressions embedded in Go AST
		// For MVP, we rely on DingoNodes being populated correctly
		return true
	})

	return diagnostics
}

// isResultType checks if a type is Result[T, E]
// Requires go/types integration (placeholder for now)
func isResultType(typ types.Type) bool {
	if typ == nil {
		return false
	}

	// Check if type is a named type with name "Result"
	if named, ok := typ.(*types.Named); ok {
		obj := named.Obj()
		if obj != nil && obj.Name() == "Result" {
			// Check if it has 2 type parameters
			if named.TypeArgs() != nil && named.TypeArgs().Len() == 2 {
				return true
			}
		}
	}

	return false
}

// isErrorTuple checks if a type is (T, error)
// Requires go/types integration (placeholder for now)
func isErrorTuple(typ types.Type) bool {
	if typ == nil {
		return false
	}

	// Check if type is a tuple with 2 elements where second is error
	if tuple, ok := typ.(*types.Tuple); ok {
		if tuple.Len() == 2 {
			second := tuple.At(1)
			if second != nil {
				// Check if second element is error interface
				if named, ok := second.Type().(*types.Named); ok {
					return named.Obj().Name() == "error"
				}
			}
		}
	}

	return false
}
