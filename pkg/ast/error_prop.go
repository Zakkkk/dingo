package ast

import (
	"fmt"
	"go/token"
	"go/types"
)

// ErrorPropExpr represents the error propagation operator: expr?
// The ? operator is a postfix operator that unwraps a Result<T,E> or (T, error) value,
// automatically returning the error if present.
//
// Syntax: expr?
//
// Examples:
//   value := getData()?           // Unwrap Result<Data, error>
//   result := foo()?.bar()?       // Chain multiple ? operators
//   return process(input?)?       // Use in return statements
//
// Type Requirements:
//   - Operand must be one of:
//     * Result<T, E> type
//     * (T, error) tuple
//   - The ? operator extracts T and propagates E/error upward
type ErrorPropExpr struct {
	Question token.Pos // Position of the ? token
	Operand  Expr      // Expression before ? (must return Result or (T, error))

	// Type information (filled by type checker during semantic analysis)
	ResultType types.Type // The success type T (what we extract from Result<T,E>)
	ErrorType  types.Type // The error type E (what we propagate if error)
}

// Node implements DingoNode marker interface
func (e *ErrorPropExpr) Node() {}

// Pos returns the starting position of the expression (start of operand)
func (e *ErrorPropExpr) Pos() token.Pos {
	if e.Operand != nil {
		return e.Operand.Pos()
	}
	return e.Question
}

// End returns the ending position of the expression (after the ? token)
func (e *ErrorPropExpr) End() token.Pos {
	return e.Question + 1
}

// exprNode implements ast.Expr marker method
func (e *ErrorPropExpr) exprNode() {}

// String returns a string representation of the error propagation expression
func (e *ErrorPropExpr) String() string {
	if e.Operand != nil {
		// For go/ast.Expr types, we can't call String() directly
		// We'll use a simple representation
		return fmt.Sprintf("%v?", e.Operand)
	}
	return "?"
}
