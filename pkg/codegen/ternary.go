package codegen

import (
	"strings"

	"github.com/MadAppGang/dingo/pkg/ast"
)

// TernaryCodeGen generates Go code from Dingo ternary expressions.
//
// Transforms:
//   cond ? trueVal : falseVal
//
// With context (return):
//   if cond { return trueVal }
//   return falseVal
//
// With context (assignment):
//   if cond { x = trueVal } else { x = falseVal }
//
// Without context (IIFE):
//   func() T { if cond { return trueVal }; return falseVal }()
//
// Handles:
//   - AST-based expressions (Cond, True, False)
//   - Legacy string-based expressions (CondStr, TrueStr, FalseStr)
//   - Inferred result type for IIFE return type
//   - Source mappings for LSP support
type TernaryCodeGen struct {
	*BaseGenerator
	expr    *ast.TernaryExpr
	Context *GenContext // Optional context for human-like code generation
}

// Generate produces Go code for the ternary expression.
//
// Output varies based on context:
//   - Return context: if cond { return trueVal }; return falseVal
//   - Assignment context: if cond { x = trueVal } else { x = falseVal }
//   - IIFE (default): func() T { if cond { return trueVal }; return falseVal }()
//
// Source mappings track:
//   - Ternary start position → entire generated code
func (g *TernaryCodeGen) Generate() ast.CodeGenResult {
	if g.expr == nil {
		return ast.CodeGenResult{}
	}

	// Check if we have context for human-like generation
	if g.Context != nil && g.Context.Context == ast.ContextReturn {
		return g.generateReturnContext()
	}

	if g.Context != nil && g.Context.Context == ast.ContextAssignment {
		return g.generateAssignmentContext()
	}

	// Default: generate IIFE
	return g.generateIIFE()
}

// generateReturnContext generates code for return statement context.
//
// Input:  return cond ? trueVal : falseVal
// Output: if cond { return trueVal }
//         return falseVal
//
// For nested ternaries in the false branch, recursively applies return context.
func (g *TernaryCodeGen) generateReturnContext() ast.CodeGenResult {
	dingoStart := int(g.expr.Pos())
	dingoEnd := int(g.expr.End())

	// Mark this as statement-level output (not expression replacement)
	var stmt []byte

	// if cond {
	stmt = append(stmt, []byte("if ")...)
	stmt = append(stmt, g.exprToBytes(g.expr.Cond, g.expr.CondStr)...)
	stmt = append(stmt, []byte(" {\n    return ")...)

	// return trueVal
	stmt = append(stmt, g.exprToBytes(g.expr.True, g.expr.TrueStr)...)
	stmt = append(stmt, []byte("\n}\n")...)

	// Check if false branch is nested ternary - if so, use return context recursively
	if nestedTernary, ok := g.expr.False.(*ast.TernaryExpr); ok {
		// Recursively generate with return context
		nestedGen := &TernaryCodeGen{
			BaseGenerator: g.BaseGenerator,
			expr:          nestedTernary,
			Context:       g.Context, // Propagate return context
		}
		nestedResult := nestedGen.Generate()
		stmt = append(stmt, nestedResult.StatementOutput...)
	} else {
		// Simple value: return falseVal
		stmt = append(stmt, []byte("return ")...)
		stmt = append(stmt, g.exprToBytes(g.expr.False, g.expr.FalseStr)...)
	}

	result := ast.CodeGenResult{
		StatementOutput: stmt,
		Mappings: []ast.SourceMapping{
			ast.NewSourceMapping(dingoStart, dingoEnd, 0, len(stmt), "ternary"),
		},
	}

	return result
}

// generateAssignmentContext generates code for assignment context.
//
// Input:  x := cond ? trueVal : falseVal
// Output: var x TYPE
//         if cond { x = trueVal } else { x = falseVal }
func (g *TernaryCodeGen) generateAssignmentContext() ast.CodeGenResult {
	dingoStart := int(g.expr.Pos())
	dingoEnd := int(g.expr.End())

	var stmt []byte

	// if cond {
	stmt = append(stmt, []byte("if ")...)
	stmt = append(stmt, g.exprToBytes(g.expr.Cond, g.expr.CondStr)...)
	stmt = append(stmt, []byte(" {\n    ")...)
	stmt = append(stmt, []byte(g.Context.VarName)...)
	stmt = append(stmt, []byte(" = ")...)

	// x = trueVal
	stmt = append(stmt, g.exprToBytes(g.expr.True, g.expr.TrueStr)...)
	stmt = append(stmt, []byte("\n} else {\n    ")...)
	stmt = append(stmt, []byte(g.Context.VarName)...)
	stmt = append(stmt, []byte(" = ")...)

	// x = falseVal
	stmt = append(stmt, g.exprToBytes(g.expr.False, g.expr.FalseStr)...)
	stmt = append(stmt, []byte("\n}")...)

	result := ast.CodeGenResult{
		StatementOutput: stmt,
		Mappings: []ast.SourceMapping{
			ast.NewSourceMapping(dingoStart, dingoEnd, 0, len(stmt), "ternary"),
		},
	}

	return result
}

// generateIIFE generates an IIFE (Immediately Invoked Function Expression).
//
// Output format:
//   func() T {
//       if cond {
//           return trueVal
//       }
//       return falseVal
//   }()
func (g *TernaryCodeGen) generateIIFE() ast.CodeGenResult {
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
	g.generateIIFEBody()
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

// generateIIFEBody generates the if/return body of the IIFE.
//
// Format:
//     if cond {
//         return trueVal
//     }
//     return falseVal
func (g *TernaryCodeGen) generateIIFEBody() {
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
// Handles nested ternaries by recursively generating their IIFE code.
func (g *TernaryCodeGen) writeExpr(expr ast.Expr, fallback string) {
	if expr != nil {
		// Special case: nested ternary expression
		if nestedTernary, ok := expr.(*ast.TernaryExpr); ok {
			// Inherit result type from parent if not set
			if nestedTernary.ResultType == "" {
				nestedTernary.ResultType = g.expr.ResultType
			}
			// Recursively generate IIFE for nested ternary
			nestedGen := &TernaryCodeGen{
				BaseGenerator: g.BaseGenerator,
				expr:          nestedTernary,
			}
			result := nestedGen.generateIIFE()
			g.Buf.Write(result.Output)
			return
		}
		// Use Dingo AST String() method, trimming whitespace to avoid
		// Go parser issues with newlines (e.g., "return \n value" becomes "return value")
		g.Write(strings.TrimSpace(expr.String()))
	} else {
		// Legacy path: use string representation, also trimmed
		g.Write(strings.TrimSpace(fallback))
	}
}

// exprToBytes converts an expression to bytes, preferring AST if available.
// Handles nested ternaries by recursively generating their IIFE code.
func (g *TernaryCodeGen) exprToBytes(expr ast.Expr, fallback string) []byte {
	if expr != nil {
		// Special case: nested ternary expression
		if nestedTernary, ok := expr.(*ast.TernaryExpr); ok {
			// Inherit result type from parent if not set
			if nestedTernary.ResultType == "" {
				nestedTernary.ResultType = g.expr.ResultType
			}
			// Recursively generate IIFE for nested ternary
			nestedGen := &TernaryCodeGen{
				BaseGenerator: g.BaseGenerator,
				expr:          nestedTernary,
			}
			result := nestedGen.generateIIFE()
			return result.Output
		}
		return []byte(strings.TrimSpace(expr.String()))
	}
	return []byte(strings.TrimSpace(fallback))
}
