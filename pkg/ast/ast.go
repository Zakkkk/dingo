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
	exprNode() // Marker method (unexported, only for Dingo AST nodes)
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
