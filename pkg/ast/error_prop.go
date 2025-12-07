package ast

import (
	"fmt"
	"go/token"
	"go/types"
)

// ErrorContext represents string-based error wrapping: ? "message"
// This adds contextual information when propagating errors.
//
// Example:
//
//	let order = fetchOrder(orderID) ? "fetch failed"
//
// Generates:
//
//	tmp, err := fetchOrder(orderID)
//	if err != nil {
//	    return ResultOrderError{err: fmt.Errorf("fetch failed: %w", err)}
//	}
//	order := tmp
type ErrorContext struct {
	Message    string    // The context message (without quotes)
	MessagePos token.Pos // Position of the string literal
}

// ErrorPropExpr represents the error propagation operator: expr?
// The ? operator is a postfix operator that unwraps a Result<T,E> or (T, error) value,
// automatically returning the error if present.
//
// Syntax:
//
//	expr?                      (basic - propagate error as-is)
//	expr ? "message"           (context wrapping with fmt.Errorf)
//	expr ? |err| transform     (Rust-style lambda transform)
//	expr ? (err) => transform  (TypeScript-style lambda transform)
//
// Examples:
//
//	value := getData()?                           // Unwrap Result<Data, error>
//	order := fetchOrder(id) ? "fetch failed"     // Add context message
//	user := loadUser(id) ? |err| wrap("user", err) // Custom transform
//
// Type Requirements:
//   - Operand must be one of:
//   - Result<T, E> type
//   - (T, error) tuple
//   - The ? operator extracts T and propagates E/error upward
type ErrorPropExpr struct {
	Question token.Pos // Position of the ? token
	Operand  Expr      // Expression before ? (must return Result or (T, error))

	// Type information (filled by type checker during semantic analysis)
	ResultType types.Type // The success type T (what we extract from Result<T,E>)
	ErrorType  types.Type // The error type E (what we propagate if error)

	// Error transformation options (mutually exclusive, both nil for basic ?)
	ErrorContext   *ErrorContext // For: expr ? "message"
	ErrorTransform *LambdaExpr   // For: expr ? |err| transform OR expr ? (err) => transform
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
	var base string
	if e.Operand != nil {
		// For go/ast.Expr types, we can't call String() directly
		// We'll use a simple representation
		base = fmt.Sprintf("%v?", e.Operand)
	} else {
		base = "?"
	}

	// Append error context or transform if present
	if e.ErrorContext != nil {
		return fmt.Sprintf("%s %q", base, e.ErrorContext.Message)
	}
	if e.ErrorTransform != nil {
		return fmt.Sprintf("%s %s", base, e.ErrorTransform.String())
	}

	return base
}
