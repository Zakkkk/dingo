package codegen

import (
	"strings"

	"github.com/MadAppGang/dingo/pkg/ast"
)

// LambdaCodeGen generates Go function literals from Dingo lambda expressions.
//
// Transforms:
//   - Rust-style: |x| x + 1 → func(x any) any { return x + 1 }
//   - TypeScript-style: (x) => x + 1 → func(x any) any { return x + 1 }
//   - Block lambda: |x| { ... } → func(x any) any { ... }
//
// Handles:
//   - Type annotations on parameters
//   - Return type annotations
//   - Expression bodies (wrap in { return ... })
//   - Block bodies (pass through)
//   - Type inference placeholders (any - replaced in type inference pass)
type LambdaCodeGen struct {
	*BaseGenerator
	expr *ast.LambdaExpr
}

// Generate produces Go code for the lambda expression.
//
// Output format:
//
//	func(param1 type1, param2 type2, ...) returnType { body }
//
// For expression bodies, wraps in { return ... }.
// For block bodies, uses as-is.
//
// Source mappings track:
//   - Lambda start position → entire generated function
func (g *LambdaCodeGen) Generate() ast.CodeGenResult {
	if g.expr == nil {
		return ast.CodeGenResult{}
	}

	// func(
	g.Write("func(")

	// Parameters
	g.generateParams()

	// )
	g.WriteByte(')')

	// Return type
	// - If explicitly specified: use it
	// - If expression body (has return stmt): default to 'any' as placeholder
	// - If block body: NO DEFAULT (may be void)
	// The type inferrer will replace 'any' with actual types when in call context
	if g.expr.ReturnType != "" {
		g.WriteByte(' ')
		g.Write(g.expr.ReturnType)
	} else if !g.expr.IsBlock {
		// Expression body - add 'any' return type so { return ... } is valid
		g.Write(" any")
	}
	// Block bodies have no default return type (may be void)

	// Body
	g.WriteByte(' ')
	g.generateBody()

	return g.Result()
}

// generateParams generates the parameter list.
//
// For each parameter:
//   - If type is specified: param Type
//   - If no type: param any (placeholder for type inference)
//
// Multiple parameters are comma-separated.
func (g *LambdaCodeGen) generateParams() {
	for i, param := range g.expr.Params {
		if i > 0 {
			g.Write(", ")
		}

		// Parameter name
		g.Write(param.Name)

		// Type
		g.WriteByte(' ')
		if param.Type != "" {
			g.Write(param.Type)
		} else {
			// Use "any" as placeholder - type inferrer will replace with actual type
			g.Write("any")
		}
	}
}

// generateBody generates the function body.
//
// For expression bodies:
//   - Wraps in { return ... }
//
// For block bodies:
//   - If has return type and single expression: adds implicit return
//   - Otherwise: uses body as-is (already has { ... })
func (g *LambdaCodeGen) generateBody() {
	if g.expr.IsBlock {
		// Block body - check if we need implicit return
		if g.expr.ReturnType != "" && g.needsImplicitReturn() {
			g.writeBlockWithImplicitReturn()
		} else {
			// Pass through as-is
			g.Write(g.expr.Body)
		}
	} else {
		// Expression body - wrap in { return ... }
		g.Write("{ return ")
		g.Write(g.expr.Body)
		g.Write(" }")
	}
}

// needsImplicitReturn checks if a block body needs an implicit return.
// Returns true if the block contains a single expression without return/if/for/switch.
func (g *LambdaCodeGen) needsImplicitReturn() bool {
	body := g.expr.Body
	if len(body) < 2 {
		return false
	}

	// Remove outer braces and trim whitespace
	inner := strings.TrimSpace(body[1 : len(body)-1])
	if inner == "" {
		return false
	}

	// If already has return, don't add another
	if strings.HasPrefix(inner, "return ") || strings.HasPrefix(inner, "return\t") || inner == "return" {
		return false
	}

	// If contains control flow statements, don't add implicit return
	// These indicate multi-statement blocks
	if strings.Contains(inner, "if ") || strings.Contains(inner, "for ") ||
		strings.Contains(inner, "switch ") || strings.Contains(inner, "select ") ||
		strings.Contains(inner, "go ") || strings.Contains(inner, "defer ") {
		return false
	}

	// If contains semicolons or newlines with statements, it's multi-statement
	if strings.Contains(inner, ";") || strings.Contains(inner, "\n") {
		return false
	}

	return true
}

// writeBlockWithImplicitReturn writes a block body with return added.
// Transforms { expr } to { return expr }
func (g *LambdaCodeGen) writeBlockWithImplicitReturn() {
	body := g.expr.Body
	inner := strings.TrimSpace(body[1 : len(body)-1])
	g.Write("{ return ")
	g.Write(inner)
	g.Write(" }")
}
