package ast

import (
	"go/token"
	"strings"
)

// LambdaStyle represents the lambda syntax style
type LambdaStyle int

const (
	// TypeScriptStyle uses TypeScript/JavaScript arrow syntax: x => expr
	TypeScriptStyle LambdaStyle = iota
	// RustStyle uses Rust pipe syntax: |x| expr
	RustStyle
)

// LambdaExpr represents a lambda expression in Dingo
// Examples:
//   - TypeScript: x => x * 2
//   - TypeScript multi: (x, y) => x + y
//   - TypeScript typed: (x: int) => x * 2
//   - Rust: |x| x * 2
//   - Rust multi: |x, y| x + y
//   - Rust typed: |x: int| -> int { x * 2 }
type LambdaExpr struct {
	LambdaPos  token.Pos      // Position of lambda start (first param or opening paren/pipe)
	Style      LambdaStyle    // TypeScript (=>) or Rust (||)
	Params     []LambdaParam  // Parameters
	ReturnType string         // Return type annotation (optional, e.g., "int", "bool")
	Body       string         // Body expression or block (unparsed)
	IsBlock    bool           // true if body is a block { ... }, false if expression
}

// LambdaParam represents a lambda parameter
type LambdaParam struct {
	Name string // Parameter name
	Type string // Type annotation (optional, empty if type inference needed)
}

// Node implements DingoNode marker interface
func (l *LambdaExpr) Node() {}

// ToGo converts LambdaExpr to Go function literal
// Outputs:
//   - x => x * 2 → func(x any) { return x * 2 }
//   - (x: int) => x * 2 → func(x int) { return x * 2 }
//   - |x, y| x + y → func(x any, y any) { return x + y }
//   - |x: int| -> int { ... } → func(x int) int { ... }
func (l *LambdaExpr) ToGo() string {
	var result strings.Builder

	// Function literal opening
	result.WriteString("func(")

	// Parameters
	for i, param := range l.Params {
		if i > 0 {
			result.WriteString(", ")
		}
		result.WriteString(param.Name)
		if param.Type != "" {
			result.WriteString(" ")
			result.WriteString(param.Type)
		} else {
			// Add placeholder for type inference - will be replaced by typechecker
			result.WriteString(" any")
		}
	}

	result.WriteString(")")

	// Return type (if specified)
	if l.ReturnType != "" {
		result.WriteString(" ")
		result.WriteString(l.ReturnType)
	}

	// Body
	if l.IsBlock {
		// Block body - pass through with space prefix
		result.WriteString(" ")
		result.WriteString(strings.TrimSpace(l.Body))
	} else {
		// Expression body - wrap in { return ... }
		result.WriteString(" { return ")
		result.WriteString(strings.TrimSpace(l.Body))
		result.WriteString(" }")
	}

	return result.String()
}

// Pos returns the position of the lambda expression
func (l *LambdaExpr) Pos() token.Pos {
	return l.LambdaPos
}

// End returns the end position (approximation)
func (l *LambdaExpr) End() token.Pos {
	// Approximate end based on generated Go code
	return l.LambdaPos + token.Pos(len(l.ToGo()))
}

// exprNode implements the Expr interface marker method
func (l *LambdaExpr) exprNode() {}

// String returns a string representation of the lambda expression
// This is used for debugging and code generation
func (l *LambdaExpr) String() string {
	var result strings.Builder

	// Opening delimiter based on style
	if l.Style == TypeScriptStyle {
		if len(l.Params) == 1 && l.Params[0].Type == "" {
			// Single param without type: x => ...
			result.WriteString(l.Params[0].Name)
		} else {
			// Multiple params or typed: (x, y) => ... or (x: int) => ...
			result.WriteString("(")
			for i, param := range l.Params {
				if i > 0 {
					result.WriteString(", ")
				}
				result.WriteString(param.Name)
				if param.Type != "" {
					result.WriteString(": ")
					result.WriteString(param.Type)
				}
			}
			result.WriteString(")")
		}
		result.WriteString(" => ")
	} else {
		// Rust style: |x, y| ...
		result.WriteString("|")
		for i, param := range l.Params {
			if i > 0 {
				result.WriteString(", ")
			}
			result.WriteString(param.Name)
			if param.Type != "" {
				result.WriteString(": ")
				result.WriteString(param.Type)
			}
		}
		result.WriteString("|")
		if l.ReturnType != "" {
			result.WriteString(" -> ")
			result.WriteString(l.ReturnType)
		}
		result.WriteString(" ")
	}

	// Body
	if l.IsBlock {
		result.WriteString(strings.TrimSpace(l.Body))
	} else {
		result.WriteString(strings.TrimSpace(l.Body))
	}

	return result.String()
}
