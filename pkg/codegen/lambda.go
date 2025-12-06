package codegen

import (
	"github.com/MadAppGang/dingo/pkg/ast"
)

// LambdaCodeGen generates Go function literals from Dingo lambda expressions.
//
// Transforms:
//   - Rust-style: |x| x + 1 → func(x TYPE) TYPE { return x + 1 }
//   - TypeScript-style: (x) => x + 1 → func(x TYPE) TYPE { return x + 1 }
//   - Block lambda: |x| { ... } → func(x TYPE) TYPE { ... }
//
// Handles:
//   - Type annotations on parameters
//   - Return type annotations
//   - Expression bodies (wrap in { return ... })
//   - Block bodies (pass through)
//   - Type inference markers (__TYPE_INFERENCE_NEEDED)
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

	// Return type (if specified)
	if g.expr.ReturnType != "" {
		g.WriteByte(' ')
		g.Write(g.expr.ReturnType)
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
//   - If no type: param __TYPE_INFERENCE_NEEDED
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
			// Type inference marker
			g.Write("__TYPE_INFERENCE_NEEDED")
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
