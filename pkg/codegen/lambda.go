package codegen

import (
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
//   func(param1 type1, param2 type2, ...) returnType { body }
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

	// Track start position
	dingoStart := int(g.expr.Pos())
	dingoEnd := int(g.expr.End())
	outputStart := g.Buf.Len()

	// func(
	g.Write("func(")

	// Parameters
	g.generateParams()

	// )
	g.WriteByte(')')

	// Return type
	// - If explicitly specified: use it
	// - If expression body (not block): needs return type, use "any" as placeholder
	// - If block body: may or may not return, user is responsible
	if g.expr.ReturnType != "" {
		g.WriteByte(' ')
		g.Write(g.expr.ReturnType)
	} else if !g.expr.IsBlock {
		// Expression lambdas always return - use "any" as placeholder
		// The LambdaTypeInferrer will refine this if the lambda is used in a typed context
		g.Write(" any")
	}

	// Body
	g.WriteByte(' ')
	g.generateBody()

	// Create mapping from lambda to generated function
	outputEnd := g.Buf.Len()

	result := g.Result()
	result.Mappings = append(result.Mappings, ast.NewSourceMapping(
		dingoStart,
		dingoEnd,
		outputStart,
		outputEnd,
		"lambda",
	))

	return result
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
			// Type inference placeholder (valid Go 1.18+ syntax)
			// Will be replaced by actual type in type inference pass
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
//   - Uses body as-is (already has { ... })
func (g *LambdaCodeGen) generateBody() {
	if g.expr.IsBlock {
		// Block body - pass through
		g.Write(g.expr.Body)
	} else {
		// Expression body - wrap in { return ... }
		g.Write("{ return ")
		g.Write(g.expr.Body)
		g.Write(" }")
	}
}
