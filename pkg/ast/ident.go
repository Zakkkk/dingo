package ast

import (
	"go/token"
)

// DingoIdent represents an identifier in Dingo syntax
// This is a simple wrapper around go/ast.Ident for Dingo AST compatibility
type DingoIdent struct {
	NamePos token.Pos // Position of the identifier
	Name    string    // Identifier name
}

// Node implements the Node interface
func (i *DingoIdent) Node() {}

// Pos returns the position of the identifier
func (i *DingoIdent) Pos() token.Pos {
	return i.NamePos
}

// End returns the end position of the identifier
func (i *DingoIdent) End() token.Pos {
	return i.NamePos + token.Pos(len(i.Name))
}

// exprNode implements the Expr marker method
func (i *DingoIdent) exprNode() {}

// String returns the identifier name
func (i *DingoIdent) String() string {
	return i.Name
}
