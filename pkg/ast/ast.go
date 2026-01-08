// Package ast provides Dingo-specific AST extensions
package ast

import (
	"go/token"
)

// Node is the base interface for all Dingo AST nodes
type Node interface {
	Pos() token.Pos // Position of first character belonging to the node
	End() token.Pos // Position of first character immediately after the node
}

// Expr represents a Dingo expression node
// All Dingo expression types must implement this interface
type Expr interface {
	Node
	exprNode()      // Marker method (unexported, only for Dingo AST nodes)
	String() string // String representation (for codegen)
}

// Stmt represents a Dingo statement node
// All Dingo statement types must implement this interface
type Stmt interface {
	Node
	stmtNode() // Marker method (unexported, only for Dingo AST nodes)
}

// Decl represents a Dingo declaration node
// All Dingo declaration types must implement this interface
type Decl interface {
	Node
	declNode() // Marker method (unexported, only for Dingo AST nodes)
}

// ReturnExpr represents a return statement used as a match arm body.
// This allows match arms to directly return from the enclosing function:
//
//	match result {
//	    Ok(v) => return Ok[T, E](v),  // ReturnExpr wrapping Ok[T, E](v)
//	    Err(e) => return Err[T, E](e),
//	}
//
// The Value field contains the expression being returned.
// If Value is nil, it represents a bare `return` statement.
type ReturnExpr struct {
	Return token.Pos // Position of 'return' keyword
	Value  Expr      // Expression to return (may be nil for bare return)
}

func (e *ReturnExpr) Node()          {}
func (e *ReturnExpr) exprNode()      {}
func (e *ReturnExpr) Pos() token.Pos { return e.Return }
func (e *ReturnExpr) End() token.Pos {
	if e.Value != nil {
		return e.Value.End()
	}
	return e.Return + 6 // len("return")
}
func (e *ReturnExpr) String() string {
	if e.Value != nil {
		return "return " + e.Value.String()
	}
	return "return"
}
