package ast

import (
	"fmt"
	"go/token"
	"strings"
)

// BuiltinCallExpr represents a Go built-in function call containing Dingo expressions.
// This allows expressions like len(config?.Items) to be properly parsed and transformed.
//
// Example: len(config?.Items)
// - Func: "len"
// - Args: [SafeNavExpr{X: config, Sel: Items}]
//
// Only len and cap are supported; other built-ins pass through as RawExpr.
type BuiltinCallExpr struct {
	Func    string    // Built-in function name: "len" or "cap"
	FuncPos token.Pos // Position of the function name
	Args    []Expr    // Arguments (may contain Dingo expressions like SafeNavExpr)
	RParen  token.Pos // Position of closing parenthesis
}

// Pos returns the position of the function name
func (b *BuiltinCallExpr) Pos() token.Pos {
	return b.FuncPos
}

// End returns the position after the closing parenthesis
func (b *BuiltinCallExpr) End() token.Pos {
	return b.RParen + 1
}

// exprNode implements the Expr marker method
func (b *BuiltinCallExpr) exprNode() {}

// String returns a string representation for debugging
func (b *BuiltinCallExpr) String() string {
	var args []string
	for _, arg := range b.Args {
		args = append(args, arg.String())
	}
	return fmt.Sprintf("%s(%s)", b.Func, strings.Join(args, ", "))
}

// ContainsDingoExpr returns true if any argument contains Dingo-specific expressions
// that need code generation (SafeNavExpr, NullCoalesceExpr, etc.)
func (b *BuiltinCallExpr) ContainsDingoExpr() bool {
	for _, arg := range b.Args {
		if containsDingoExpr(arg) {
			return true
		}
	}
	return false
}

// containsDingoExpr recursively checks if an expression contains Dingo syntax
func containsDingoExpr(expr Expr) bool {
	if expr == nil {
		return false
	}
	switch e := expr.(type) {
	case *SafeNavExpr:
		return true
	case *SafeNavCallExpr:
		return true
	case *NullCoalesceExpr:
		return true
	case *TernaryExpr:
		return true
	case *MatchExpr:
		return true
	case *LambdaExpr:
		return true
	case *ErrorPropExpr:
		return true
	case *BuiltinCallExpr:
		return e.ContainsDingoExpr()
	default:
		return false
	}
}
