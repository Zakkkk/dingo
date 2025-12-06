package ast

import (
	"go/token"
)

// NullCoalesceExpr represents a null coalescing expression: a ?? b
// The ?? operator provides a default value when the left operand is None/nil
// Example: name ?? "Guest" → returns name if Some, "Guest" if None
// Right-associative: a ?? b ?? c is parsed as a ?? (b ?? c)
// Nesting is implicit in the AST structure (Right can be another NullCoalesceExpr)
type NullCoalesceExpr struct {
	Left  Expr      // Left operand (expression that might be nil/None) - pkg/ast.Expr
	OpPos token.Pos // Position of ?? operator
	Right Expr      // Right operand (default value if Left is nil) - pkg/ast.Expr
}

// Pos returns the position of the left operand start
func (n *NullCoalesceExpr) Pos() token.Pos {
	if n.Left != nil {
		return n.Left.Pos()
	}
	return n.OpPos
}

// End returns the end position of the right operand
func (n *NullCoalesceExpr) End() token.Pos {
	if n.Right != nil {
		return n.Right.End()
	}
	return n.OpPos + 2 // Length of "??"
}

// exprNode implements the Expr marker method
func (n *NullCoalesceExpr) exprNode() {}

// String returns a string representation
func (n *NullCoalesceExpr) String() string {
	return "??" // Simplified representation
}

// SafeNavExpr represents a safe navigation expression for field access: expr?.field
// The ?. operator safely accesses a field, returning None if the receiver is None/nil
// Example: user?.name → returns Some(user.name) if user is Some, None if user is None
// Chaining is implicit in the AST structure (X can be another SafeNavExpr)
type SafeNavExpr struct {
	X     Expr        // Expression that might be nil (receiver) - pkg/ast.Expr
	OpPos token.Pos   // Position of ?. operator
	Sel   *DingoIdent // Field being accessed - pkg/ast.DingoIdent
}

// Pos returns the position of the receiver start
func (s *SafeNavExpr) Pos() token.Pos {
	return s.X.Pos()
}

// End returns the end position
func (s *SafeNavExpr) End() token.Pos {
	return s.Sel.End()
}

// exprNode implements the Expr marker method
func (s *SafeNavExpr) exprNode() {}

// String returns a string representation
func (s *SafeNavExpr) String() string {
	return "?." // Simplified representation
}

// SafeNavCallExpr represents a safe navigation expression for method calls: expr?.method(args)
// The ?. operator safely calls a method, returning None if the receiver is None/nil
// Example: user?.getName() → returns Some(user.getName()) if user is Some, None if user is None
// Chaining is implicit in the AST structure (X can be another SafeNavExpr or SafeNavCallExpr)
type SafeNavCallExpr struct {
	X     Expr        // Expression that might be nil (receiver) - pkg/ast.Expr
	OpPos token.Pos   // Position of ?. operator
	Fun   *DingoIdent // Method name - pkg/ast.DingoIdent
	Args  []Expr      // Call arguments - pkg/ast.Expr
}

// Pos returns the position of the receiver start
func (s *SafeNavCallExpr) Pos() token.Pos {
	return s.X.Pos()
}

// End returns the end position
func (s *SafeNavCallExpr) End() token.Pos {
	if len(s.Args) > 0 {
		return s.Args[len(s.Args)-1].End() + 1 // +1 for closing paren
	}
	return s.Fun.End() + 2 // +2 for ()
}

// exprNode implements the Expr marker method
func (s *SafeNavCallExpr) exprNode() {}

// String returns a string representation
func (s *SafeNavCallExpr) String() string {
	return "?.call" // Simplified representation
}
