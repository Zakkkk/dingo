package ast

import "go/token"

// NullCoalesceExpr represents a null coalescing expression: a ?? b
// The ?? operator provides a default value when the left operand is None/nil
// Example: name ?? "Guest" → returns name if Some, "Guest" if None
type NullCoalesceExpr struct {
	Left  string    // Left operand expression as string
	OpPos token.Pos // Position of ?? operator
	Right string    // Right operand expression as string
	// For chained expressions: a ?? b ?? c
	// This is parsed as: a ?? (b ?? c) [right-associative]
	Chain []*NullCoalesceExpr // Chained coalesce expressions (nil if not chained)
}

// Node implements DingoNode marker interface
func (n *NullCoalesceExpr) Node() {}

// Pos returns the position of the left operand start
func (n *NullCoalesceExpr) Pos() token.Pos {
	return n.OpPos
}

// End returns the end position (approximation)
func (n *NullCoalesceExpr) End() token.Pos {
	return n.OpPos + token.Pos(len(n.Left)+2+len(n.Right))
}

// SafeNavExpr represents a safe navigation expression: a?.b
// The ?. operator safely accesses a field/method, returning None if receiver is None/nil
// Example: user?.name → returns Some(user.name) if user is Some, None if user is None
type SafeNavExpr struct {
	Receiver string    // Receiver expression as string
	OpPos    token.Pos // Position of ?. operator
	Field    string    // Field/method name
	// For chained expressions: a?.b?.c
	Chain []*SafeNavExpr // Chained safe nav expressions (nil if not chained)
}

// Node implements DingoNode marker interface
func (s *SafeNavExpr) Node() {}

// Pos returns the position of the receiver start
func (s *SafeNavExpr) Pos() token.Pos {
	return s.OpPos
}

// End returns the end position (approximation)
func (s *SafeNavExpr) End() token.Pos {
	return s.OpPos + token.Pos(len(s.Receiver)+2+len(s.Field))
}
