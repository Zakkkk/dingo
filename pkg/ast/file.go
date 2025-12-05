package ast

import "go/ast"

// File wraps go/ast.File for Dingo
type File struct {
	*ast.File
	DingoNodes []DingoNode // All Dingo-specific AST nodes
}

// DingoNode is a marker interface for Dingo AST nodes
type DingoNode interface {
	Node()
}

// ExprWrapper wraps a Dingo Expr to implement DingoNode
type ExprWrapper struct {
	DingoExpr Expr // Dingo ast.Expr
}

// Node implements DingoNode
func (e *ExprWrapper) Node() {}
