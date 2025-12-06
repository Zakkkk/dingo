package codegen

import (
	goast "go/ast"
	"go/printer"
	"go/token"

	"github.com/MadAppGang/dingo/pkg/ast"
)

// TernaryCodeGen generates Go IIFE from Dingo ternary expressions.
//
// Transforms:
//   cond ? trueVal : falseVal
// To:
//   func() T { if cond { return trueVal }; return falseVal }()
//
// Handles:
//   - AST-based expressions (Cond, True, False)
//   - Legacy string-based expressions (CondStr, TrueStr, FalseStr)
//   - Inferred result type for IIFE return type
//   - Source mappings for LSP support
type TernaryCodeGen struct {
	*BaseGenerator
	expr *ast.TernaryExpr
}

// Generate produces Go code for the ternary expression.
//
// Output format:
//   func() T {
//       if cond {
//           return trueVal
//       }
//       return falseVal
//   }()
//
// Source mappings track:
//   - Ternary start position → entire generated IIFE
func (g *TernaryCodeGen) Generate() ast.CodeGenResult {
	if g.expr == nil {
		return ast.CodeGenResult{}
	}

	// Track start position
	dingoStart := int(g.expr.Pos())
	dingoEnd := int(g.expr.End())
	outputStart := g.Buf.Len()

	// func()
	g.Write("func()")

	// Return type (if specified)
	if g.expr.ResultType != "" {
		g.WriteByte(' ')
		g.Write(g.expr.ResultType)
	}

	// IIFE body
	g.Write(" {\n")
	g.generateBody()
	g.Write("}()")

	// Create mapping from ternary to generated IIFE
	outputEnd := g.Buf.Len()

	result := g.Result()
	result.Mappings = append(result.Mappings, ast.NewSourceMapping(
		dingoStart,
		dingoEnd,
		outputStart,
		outputEnd,
		"ternary",
	))

	return result
}

// generateBody generates the if/return body of the IIFE.
//
// Format:
//     if cond {
//         return trueVal
//     }
//     return falseVal
func (g *TernaryCodeGen) generateBody() {
	// if cond {
	g.Write("    if ")
	g.writeExpr(g.expr.Cond, g.expr.CondStr)
	g.Write(" {\n")

	// return trueVal
	g.Write("        return ")
	g.writeExpr(g.expr.True, g.expr.TrueStr)
	g.Write("\n    }\n")

	// return falseVal
	g.Write("    return ")
	g.writeExpr(g.expr.False, g.expr.FalseStr)
	g.Write("\n")
}

// writeExpr writes an expression, preferring AST if available, falling back to string.
func (g *TernaryCodeGen) writeExpr(expr goast.Expr, fallback string) {
	if expr != nil {
		// Use go/printer to format the AST expression
		fset := token.NewFileSet()
		if err := printer.Fprint(&g.Buf, fset, expr); err != nil {
			// Fallback to string if printing fails
			g.Write(fallback)
		}
	} else {
		// Legacy path: use string representation
		g.Write(fallback)
	}
}
