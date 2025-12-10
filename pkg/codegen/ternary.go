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
// Handles hoisting: if the condition contains BuiltinCallExpr like len(c?.Region),
// the hoisted code is prepended before the if statement.
//
// Null-state inference: if condition is `len(x?.y) > 0` and true branch is `x?.y`,
// the true branch is optimized to `*x.y` (direct dereference, no IIFE).
func (g *TernaryCodeGen) generateReturnContext() ast.CodeGenResult {
	// Use relative positions (0 to exprLen) - transformer adds loc.Start offset
	dingoStart := 0
	dingoEnd := int(g.expr.End() - g.expr.Pos())

	// Mark this as statement-level output (not expression replacement)
	var stmt []byte

	// Create context for transforming condition (enables hoisting for BuiltinCallExpr)
	counter := 0
	ctx := &GenContext{
		Context:     ast.ContextArgument,
		TempCounter: &counter,
	}

	// Detect len(x?.y) > 0 pattern for null-state inference
	nullStatePattern := DetectLenSafeNavPattern(g.expr.Cond)

	// Transform condition - this may produce hoisted code
	condResult := TransformExprForTernary(g.expr.Cond, ctx)

	// Prepend hoisted code (e.g., var tmp int; if ... { tmp = len(...) })
	if len(condResult.HoistedCode) > 0 {
		stmt = append(stmt, condResult.HoistedCode...)
	}

	// if cond {
	stmt = append(stmt, []byte("if ")...)
	if len(condResult.Output) > 0 {
		stmt = append(stmt, condResult.Output...)
	} else {
		stmt = append(stmt, g.exprToBytes(g.expr.Cond, g.expr.CondStr)...)
	}
	stmt = append(stmt, []byte(" {\n    return ")...)

	// return trueVal - with null-state optimization if pattern matches
	if nullStatePattern != nil && MatchesSafeNavPath(g.expr.True, nullStatePattern) {
		// Optimized: direct dereference instead of IIFE
		stmt = append(stmt, []byte(nullStatePattern.ToDerefExpr())...)
	} else {
		stmt = append(stmt, g.exprToBytes(g.expr.True, g.expr.TrueStr)...)
	}
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
//
// Handles hoisting: if the condition contains BuiltinCallExpr like len(c?.Region),
// the hoisted code is prepended before the if statement.
//
// Null-state inference: if condition is `len(x?.y) > 0` and true branch is `x?.y`,
// the true branch is optimized to `*x.y` (direct dereference, no IIFE).
func (g *TernaryCodeGen) generateAssignmentContext() ast.CodeGenResult {
	// Use relative positions (0 to exprLen) - transformer adds loc.Start offset
	dingoStart := 0
	dingoEnd := int(g.expr.End() - g.expr.Pos())

	var stmt []byte

	// Create context for transforming condition (enables hoisting for BuiltinCallExpr)
	counter := 0
	ctx := &GenContext{
		Context:     ast.ContextArgument,
		TempCounter: &counter,
	}

	// Detect len(x?.y) > 0 pattern for null-state inference
	nullStatePattern := DetectLenSafeNavPattern(g.expr.Cond)

	// Transform condition - this may produce hoisted code
	condResult := TransformExprForTernary(g.expr.Cond, ctx)

	// Prepend variable declaration before the if statement
	// This ensures the variable is declared in the outer scope
	if g.Context != nil && g.Context.VarName != "" {
		stmt = append(stmt, []byte("var ")...)
		stmt = append(stmt, []byte(g.Context.VarName)...)
		stmt = append(stmt, []byte(" ")...)
		if g.Context.VarType != "" {
			stmt = append(stmt, []byte(g.Context.VarType)...)
		} else {
			// Fallback to any if type unknown (Go 1.18+)
			stmt = append(stmt, []byte("any")...)
		}
		stmt = append(stmt, []byte("\n")...)
	}

	// Prepend hoisted code (e.g., var tmp int; if ... { tmp = len(...) })
	if len(condResult.HoistedCode) > 0 {
		stmt = append(stmt, condResult.HoistedCode...)
	}

	// if cond {
	stmt = append(stmt, []byte("if ")...)
	if len(condResult.Output) > 0 {
		stmt = append(stmt, condResult.Output...)
	} else {
		stmt = append(stmt, g.exprToBytes(g.expr.Cond, g.expr.CondStr)...)
	}
	stmt = append(stmt, []byte(" {\n    ")...)
	stmt = append(stmt, []byte(g.Context.VarName)...)
	stmt = append(stmt, []byte(" = ")...)

	// x = trueVal - with null-state optimization if pattern matches
	if nullStatePattern != nil && MatchesSafeNavPath(g.expr.True, nullStatePattern) {
		// Optimized: direct dereference instead of IIFE
		stmt = append(stmt, []byte(nullStatePattern.ToDerefExpr())...)
	} else {
		stmt = append(stmt, g.exprToBytes(g.expr.True, g.expr.TrueStr)...)
	}
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
	// Use relative positions (0 to exprLen) - transformer adds loc.Start offset
	dingoStart := 0
	dingoEnd := int(g.expr.End() - g.expr.Pos())
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
//
//	if cond {
//	    return trueVal
//	}
//	return falseVal
//
// Handles hoisting: if the condition contains BuiltinCallExpr like len(c?.Region),
// the hoisted code is placed at the start of the IIFE body.
//
// Null-state inference: if condition is `len(x?.y) > 0` and true branch is `x?.y`,
// the true branch is optimized to `*x.y` (direct dereference, no nested IIFE).
func (g *TernaryCodeGen) generateIIFEBody() {
	// Create context for transforming condition (enables hoisting for BuiltinCallExpr)
	counter := 0
	ctx := &GenContext{
		Context:     ast.ContextArgument,
		TempCounter: &counter,
	}

	// Detect len(x?.y) > 0 pattern for null-state inference
	nullStatePattern := DetectLenSafeNavPattern(g.expr.Cond)

	// Transform condition - this may produce hoisted code
	condResult := TransformExprForTernary(g.expr.Cond, ctx)

	// Write hoisted code at start of IIFE body
	if len(condResult.HoistedCode) > 0 {
		g.Write("    ")
		g.Buf.Write(condResult.HoistedCode)
	}

	// if cond {
	g.Write("    if ")
	if len(condResult.Output) > 0 {
		g.Buf.Write(condResult.Output)
	} else {
		g.writeExpr(g.expr.Cond, g.expr.CondStr)
	}
	g.Write(" {\n")

	// return trueVal - with null-state optimization if pattern matches
	g.Write("        return ")
	if nullStatePattern != nil && MatchesSafeNavPath(g.expr.True, nullStatePattern) {
		// Optimized: direct dereference instead of nested IIFE
		g.Write(nullStatePattern.ToDerefExpr())
	} else {
		g.writeExpr(g.expr.True, g.expr.TrueStr)
	}
	g.Write("\n    }\n")

	// return falseVal
	g.Write("    return ")
	g.writeExpr(g.expr.False, g.expr.FalseStr)
	g.Write("\n")
}

// writeExpr writes an expression, preferring AST if available, falling back to string.
// Handles nested Dingo expressions by recursively generating their code.
func (g *TernaryCodeGen) writeExpr(expr ast.Expr, fallback string) {
	if expr != nil {
		// Check for Dingo expression types that need code generation
		switch e := expr.(type) {
		case *ast.TernaryExpr:
			// Inherit result type from parent if not set
			if e.ResultType == "" {
				e.ResultType = g.expr.ResultType
			}
			// Recursively generate IIFE for nested ternary
			nestedGen := &TernaryCodeGen{
				BaseGenerator: g.BaseGenerator,
				expr:          e,
			}
			result := nestedGen.generateIIFE()
			g.Buf.Write(result.Output)
			return
		case *ast.NullCoalesceExpr, *ast.SafeNavExpr, *ast.SafeNavCallExpr, *ast.MatchExpr, *ast.LambdaExpr, *ast.BuiltinCallExpr:
			// Use GenerateExpr for other Dingo expression types
			result := GenerateExpr(expr)
			g.Buf.Write(result.Output)
			return
		}
		// For other expressions (RawExpr, DingoIdent, etc.), use String()
		// Trim whitespace to avoid Go parser issues with newlines
		g.Write(strings.TrimSpace(expr.String()))
	} else {
		// Legacy path: use string representation, also trimmed
		g.Write(strings.TrimSpace(fallback))
	}
}

// exprToBytes converts an expression to bytes, preferring AST if available.
// Handles nested Dingo expressions by recursively generating their code.
func (g *TernaryCodeGen) exprToBytes(expr ast.Expr, fallback string) []byte {
	if expr != nil {
		// Check for Dingo expression types that need code generation
		switch e := expr.(type) {
		case *ast.TernaryExpr:
			// Inherit result type from parent if not set
			if e.ResultType == "" {
				e.ResultType = g.expr.ResultType
			}
			// Recursively generate IIFE for nested ternary
			nestedGen := &TernaryCodeGen{
				BaseGenerator: g.BaseGenerator,
				expr:          e,
			}
			result := nestedGen.generateIIFE()
			return result.Output
		case *ast.NullCoalesceExpr, *ast.SafeNavExpr, *ast.SafeNavCallExpr, *ast.MatchExpr, *ast.LambdaExpr, *ast.BuiltinCallExpr:
			// Use GenerateExpr for other Dingo expression types
			result := GenerateExpr(expr)
			return result.Output
		}
		// For other expressions (RawExpr, DingoIdent, etc.), use String()
		return []byte(strings.TrimSpace(expr.String()))
	}
	return []byte(strings.TrimSpace(fallback))
}
