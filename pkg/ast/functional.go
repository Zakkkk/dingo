package ast

import "go/token"

// FunctionalCall represents a functional method call in Dingo
// Examples:
//   - nums.map(func(x int) int { return x * 2 })
//   - items.filter(func(x Item) bool { return x.active })
//   - data.reduce(0, func(acc int, x int) int { return acc + x })
type FunctionalCall struct {
	CallPos    token.Pos    // Position of method call start
	Receiver   string       // The slice/array expression (e.g., "nums", "items")
	Method     string       // Method name (map, filter, reduce, etc.)
	Args       []string     // Arguments (function literals, initial values)
	Lambda     *FuncLiteral // Parsed lambda function (if applicable)
	StartPos   int          // Start position in line (for chain detection)
	EndPos     int          // End position in line (for chain detection)
}

// FuncLiteral represents a Go function literal parsed from functional call
// This is the result of lambda expansion, so it's already valid Go code
type FuncLiteral struct {
	Params     []Param // Function parameters
	ReturnType string  // Return type (may be empty for inference)
	Body       string  // Function body (inside braces)
	IsExpr     bool    // true if body is "return expr", false if multi-statement
}

// Param represents a function parameter with optional type
type Param struct {
	Name string // Parameter name
	Type string // Parameter type (may be empty)
}

// ChainExpr represents a chained functional call
// Examples:
//   - nums.filter(f).map(g)
//   - items.map(f).filter(g).reduce(init, r)
type ChainExpr struct {
	ChainPos   token.Pos         // Position of chain start
	Receiver   string            // Initial receiver (e.g., "nums")
	Operations []FunctionalCall  // Ordered chain of operations
	CanFuse    bool              // Whether this chain can be fused into single loop
	StartPos   int               // Start position in line (for replacement)
	EndPos     int               // End position in line (for replacement)
}

// Node implements DingoNode marker interface
func (f *FunctionalCall) Node() {}
func (c *ChainExpr) Node() {}

// Pos returns the position of the functional call
func (f *FunctionalCall) Pos() token.Pos {
	return f.CallPos
}

// Pos returns the position of the chain expression
func (c *ChainExpr) Pos() token.Pos {
	return c.ChainPos
}

// End returns approximate end position
func (f *FunctionalCall) End() token.Pos {
	return f.CallPos + token.Pos(100) // Approximation
}

// End returns approximate end position
func (c *ChainExpr) End() token.Pos {
	return c.ChainPos + token.Pos(200) // Approximation
}

// ToGo is implemented by the processor, not the AST node
// The processor generates the IIFE-wrapped loop code
