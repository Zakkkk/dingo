package ast

import (
	"go/ast"
	"go/token"
)

// NullCoalesceExpr represents a null coalescing expression: a ?? b
// The ?? operator provides a default value when the left operand is None/nil
// Example: name ?? "Guest" → returns name if Some, "Guest" if None
// Right-associative: a ?? b ?? c is parsed as a ?? (b ?? c)
type NullCoalesceExpr struct {
	// AST-based fields (new, for AST migration)
	Left  ast.Expr  // Left operand (expression that might be nil/None)
	OpPos token.Pos // Position of ?? operator
	Right ast.Expr  // Right operand (default value if Left is nil)

	// Legacy string-based fields (for preprocessor backward compatibility)
	LeftStr  string                // Left operand as string (for regex preprocessor)
	RightStr string                // Right operand as string (for regex preprocessor)
	Chain    []*NullCoalesceExpr   // Chained ?? operators (for a ?? b ?? c)
}

// Node implements DingoNode marker interface
func (n *NullCoalesceExpr) Node() {}

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

// exprNode implements ast.Expr marker method
func (n *NullCoalesceExpr) exprNode() {}

// String returns a string representation
func (n *NullCoalesceExpr) String() string {
	return "??" // Simplified representation
}

// SafeNavExpr represents a safe navigation expression for field access: expr?.field
// The ?. operator safely accesses a field, returning None if the receiver is None/nil
// Example: user?.name → returns Some(user.name) if user is Some, None if user is None
// Supports chaining: user?.address?.city
type SafeNavExpr struct {
	// AST-based fields (new, for AST migration)
	X     ast.Expr   // Expression that might be nil (receiver)
	OpPos token.Pos  // Position of ?. operator
	Sel   *ast.Ident // Field being accessed

	// Legacy string-based fields (for preprocessor backward compatibility)
	Receiver string          // Receiver as string (for regex preprocessor)
	Field    string          // Field name as string (for regex preprocessor)
	Chain    []*SafeNavExpr  // Chained safe nav expressions (for a?.b?.c)
}

// Node implements DingoNode marker interface
func (s *SafeNavExpr) Node() {}

// Pos returns the position of the receiver start
func (s *SafeNavExpr) Pos() token.Pos {
	return s.X.Pos()
}

// End returns the end position
func (s *SafeNavExpr) End() token.Pos {
	return s.Sel.End()
}

// exprNode implements ast.Expr marker method
func (s *SafeNavExpr) exprNode() {}

// String returns a string representation
func (s *SafeNavExpr) String() string {
	return "?." // Simplified representation
}

// SafeNavCallExpr represents a safe navigation expression for method calls: expr?.method(args)
// The ?. operator safely calls a method, returning None if the receiver is None/nil
// Example: user?.getName() → returns Some(user.getName()) if user is Some, None if user is None
// Supports chaining: user?.getAddress()?.getCity()
type SafeNavCallExpr struct {
	// AST-based fields (new, for AST migration)
	X     ast.Expr   // Expression that might be nil (receiver)
	OpPos token.Pos  // Position of ?. operator
	Fun   *ast.Ident // Method name
	Args  []ast.Expr // Call arguments

	// Legacy string-based fields (for preprocessor backward compatibility)
	Receiver string   // Receiver as string (for regex preprocessor)
	Method   string   // Method name as string (for regex preprocessor)
	ArgsStr  []string // Arguments as strings (for regex preprocessor)
}

// Node implements DingoNode marker interface
func (s *SafeNavCallExpr) Node() {}

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

// exprNode implements ast.Expr marker method
func (s *SafeNavCallExpr) exprNode() {}

// String returns a string representation
func (s *SafeNavCallExpr) String() string {
	return "?.call" // Simplified representation
}
