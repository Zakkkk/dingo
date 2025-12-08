package ast

import (
	"go/token"
)

// BinaryExpr represents a binary expression: X op Y
// This type is used to preserve AST structure when operands contain
// Dingo expressions that need code generation (like SafeNavExpr, BuiltinCallExpr).
//
// Example: len(c?.Region) > 0
//   X:  BuiltinCallExpr{Func: "len", Args: [SafeNavExpr{...}]}
//   Op: ">"
//   Y:  RawExpr{Text: "0"}
type BinaryExpr struct {
	X     Expr      // Left operand (may contain Dingo expressions)
	OpPos token.Pos // Position of operator
	Op    string    // Operator ("+", "-", "*", "/", ">", "<", "==", "!=", etc.)
	Y     Expr      // Right operand (may contain Dingo expressions)
}

// Pos returns the position of the left operand start
func (b *BinaryExpr) Pos() token.Pos {
	if b.X != nil {
		return b.X.Pos()
	}
	return b.OpPos
}

// End returns the end position of the right operand
func (b *BinaryExpr) End() token.Pos {
	if b.Y != nil {
		return b.Y.End()
	}
	return b.OpPos + token.Pos(len(b.Op))
}

// exprNode implements the Expr marker method
func (b *BinaryExpr) exprNode() {}

// String returns a string representation (for debugging)
// Note: This uses simplified representations for Dingo expressions.
// For full text, use the codegen pipeline.
func (b *BinaryExpr) String() string {
	left := ""
	right := ""
	if b.X != nil {
		left = b.X.String()
	}
	if b.Y != nil {
		right = b.Y.String()
	}
	return left + " " + b.Op + " " + right
}

// ContainsDingoExpr returns true if either operand contains Dingo-specific expressions
func (b *BinaryExpr) ContainsDingoExpr() bool {
	return containsDingoExpr(b.X) || containsDingoExpr(b.Y)
}
