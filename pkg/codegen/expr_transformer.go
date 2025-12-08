package codegen

import (
	"github.com/MadAppGang/dingo/pkg/ast"
)

// TransformExpr recursively transforms an expression, generating Go code
// for any Dingo expressions found within it.
//
// Unlike GenerateExpr which only handles top-level Dingo expressions,
// TransformExpr descends into BinaryExpr, UnaryExpr, CallExpr, etc.
// to find and transform nested Dingo expressions.
//
// The context parameter is used to pass hoisting context (temp counter, etc.)
// to nested expressions that support hoisting.
//
// Example:
//
//	len(c?.Region) > 0
//
// Becomes:
//
//	HoistedCode: var tmp int\nif c != nil && c.Region != nil { tmp = len(*c.Region) }\n
//	Output: tmp > 0
func TransformExpr(expr ast.Expr, ctx *GenContext) ast.CodeGenResult {
	if expr == nil {
		return ast.CodeGenResult{}
	}

	switch e := expr.(type) {
	// Dingo expression types - use their generators
	case *ast.TernaryExpr:
		gen := NewTernaryCodeGen(e)
		if ctx != nil {
			gen.Context = ctx
		}
		return gen.Generate()

	case *ast.MatchExpr:
		gen := &MatchCodeGen{
			BaseGenerator: NewBaseGenerator(),
			Match:         e,
		}
		if ctx != nil {
			gen.Context = ctx
		}
		return gen.Generate()

	case *ast.SafeNavExpr:
		gen := NewSafeNavGenerator(e)
		if ctx != nil {
			gen.Context = ctx
		}
		return gen.Generate()

	case *ast.SafeNavCallExpr:
		gen := NewSafeNavCallGenerator(e)
		if ctx != nil {
			gen.Context = ctx
		}
		return gen.Generate()

	case *ast.NullCoalesceExpr:
		gen := NewNullCoalesceGenerator(e)
		if ctx != nil {
			gen.Context = ctx
		}
		return gen.Generate()

	case *ast.BuiltinCallExpr:
		gen := NewBuiltinCallCodeGen(e)
		// For BuiltinCallExpr in ternary conditions, use ContextArgument to trigger hoisting
		if ctx != nil {
			gen.Context = ctx
		} else {
			// Create a context with ContextArgument to enable hoisting
			counter := 0
			gen.Context = &GenContext{
				Context:     ast.ContextArgument,
				TempCounter: &counter,
			}
		}
		return gen.Generate()

	case *ast.LambdaExpr:
		return NewLambdaCodeGen(e).Generate()

	case *ast.ErrorPropExpr:
		return NewErrorPropCodeGen(e).Generate()

	// Composite expression types - recurse into children
	case *ast.BinaryExpr:
		return transformBinaryExpr(e, ctx)

	// Leaf expression types - pass through
	case *ast.RawExpr:
		return ast.NewCodeGenResult([]byte(e.Text))

	case *ast.DingoIdent:
		return ast.NewCodeGenResult([]byte(e.Name))

	default:
		// Unknown type - use String() fallback
		// This handles go/ast types wrapped in expressions
		if stringer, ok := expr.(interface{ String() string }); ok {
			return ast.NewCodeGenResult([]byte(stringer.String()))
		}
		return ast.CodeGenResult{}
	}
}

// transformBinaryExpr transforms a binary expression, recursively transforming
// any nested Dingo expressions in the operands.
//
// Example: len(c?.Region) > 0
// Transforms to: tmp > 0 with hoisted code for the len call
func transformBinaryExpr(e *ast.BinaryExpr, ctx *GenContext) ast.CodeGenResult {
	// Transform left and right operands
	left := TransformExpr(e.X, ctx)
	right := TransformExpr(e.Y, ctx)

	// Combine hoisted code from both sides (left first, then right)
	var result ast.CodeGenResult
	result.HoistedCode = append(result.HoistedCode, left.HoistedCode...)
	result.HoistedCode = append(result.HoistedCode, right.HoistedCode...)

	// Build output: left op right
	result.Output = append(result.Output, left.Output...)
	result.Output = append(result.Output, ' ')
	result.Output = append(result.Output, []byte(e.Op)...)
	result.Output = append(result.Output, ' ')
	result.Output = append(result.Output, right.Output...)

	return result
}

// TransformExprForTernary transforms an expression for use in a ternary context.
// It handles the special case where nested Dingo expressions need to be transformed
// and their hoisted code collected for prepending before the if statement.
//
// Returns the transformed expression result with any hoisted code.
func TransformExprForTernary(expr ast.Expr, ctx *GenContext) ast.CodeGenResult {
	if expr == nil {
		return ast.CodeGenResult{}
	}

	// Check if this is a RawExpr containing a binary expression with Dingo syntax
	// The Pratt parser wraps binary expressions as RawExpr
	if raw, ok := expr.(*ast.RawExpr); ok {
		// Check if the raw text might contain Dingo expressions
		// This is a heuristic - we look for ?. or ?? patterns
		if containsDingoSyntax(raw.Text) {
			// The raw text contains Dingo syntax that wasn't properly parsed
			// This shouldn't happen if the parser is working correctly
			// Fall through to String() for now
			return ast.NewCodeGenResult([]byte(raw.Text))
		}
		return ast.NewCodeGenResult([]byte(raw.Text))
	}

	// For Dingo expressions, use TransformExpr
	return TransformExpr(expr, ctx)
}

// containsDingoSyntax checks if a string contains Dingo-specific syntax markers
func containsDingoSyntax(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == '?' {
			// Check for ?. or ??
			if i+1 < len(s) && (s[i+1] == '.' || s[i+1] == '?') {
				return true
			}
		}
	}
	return false
}
