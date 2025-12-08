package ast

import (
	"go/token"
)

// TernaryExpr represents a ternary conditional expression: cond ? trueVal : falseVal
// Transforms to IIFE: func() T { if cond { return trueVal }; return falseVal }()
type TernaryExpr struct {
	// AST-based fields (new, for AST migration)
	Cond     Expr      // Condition expression (Dingo AST)
	Question token.Pos // Position of ? operator
	True     Expr      // True branch expression (Dingo AST)
	Colon    token.Pos // Position of : operator
	False    Expr      // False branch expression (Dingo AST)

	// Legacy string-based fields (for preprocessor backward compatibility)
	CondStr    string // Condition as string
	TrueStr    string // True branch as string
	FalseStr   string // False branch as string
	ResultType string // Inferred result type
}

// Node implements DingoNode marker interface
func (t *TernaryExpr) Node() {}

// Pos returns the position of the condition start
func (t *TernaryExpr) Pos() token.Pos {
	if t.Cond != nil {
		return t.Cond.Pos()
	}
	return t.Question
}

// End returns the end position of the false branch
func (t *TernaryExpr) End() token.Pos {
	if t.False != nil {
		return t.False.End()
	}
	return t.Colon + 1 // Length of ":"
}

// exprNode implements ast.Expr marker method
func (t *TernaryExpr) exprNode() {}

// String returns a string representation
func (t *TernaryExpr) String() string {
	return "?:" // Simplified representation
}
