package codegen

import (
	"github.com/MadAppGang/dingo/pkg/ast"
	"github.com/MadAppGang/dingo/pkg/typechecker"
)

// GenContext provides context for code generation to produce human-like output.
type GenContext struct {
	Context        ast.ExprContext      // Statement context (return, assignment, argument)
	VarName        string               // For assignments: variable name being assigned
	VarType        string               // For assignments: inferred type (e.g., "*string")
	StatementStart int                  // Byte offset of containing statement start
	StatementEnd   int                  // Byte offset of containing statement end
	EnumRegistry   map[string]string    // Maps variant name to enum type name (e.g., "UserCreated" -> "Event")
	TempCounter    *int                 // Shared counter for unique temp var names across expressions
	TypeChecker    *typechecker.Checker // For go/types queries (e.g., detecting Option/Result types in match)
}

// GenerateExprWithContext generates Go code with context-aware output.
// When context is provided, it can generate statement-level code instead of IIFEs.
//
// For example, with return context, `config?.Host ?? "default"` generates:
//
//	if config != nil && config.Host != nil {
//	    return *config.Host
//	}
//	return "default"
//
// Instead of an IIFE.
func GenerateExprWithContext(expr ast.Expr, ctx *GenContext) ast.CodeGenResult {
	if expr == nil {
		return ast.CodeGenResult{}
	}

	switch e := expr.(type) {
	case *ast.MatchExpr:
		gen := &MatchCodeGen{
			BaseGenerator: NewBaseGenerator(),
			Match:         e,
		}
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
	case *ast.TernaryExpr:
		gen := NewTernaryCodeGen(e)
		if ctx != nil {
			gen.Context = ctx
		}
		return gen.Generate()
	case *ast.BuiltinCallExpr:
		gen := NewBuiltinCallCodeGen(e)
		if ctx != nil {
			gen.Context = ctx
		}
		return gen.Generate()
	default:
		// For other expression types, use standard generation
		return GenerateExpr(expr)
	}
}

// GenerateExpr generates Go code for any Dingo expression.
// It dispatches to the appropriate codegen based on expression type.
//
// This is the main entry point for expression code generation.
// It handles all Dingo expression types:
//   - MatchExpr: Pattern matching
//   - LambdaExpr: Lambda expressions
//   - ErrorPropExpr: Error propagation (?)
//   - LetDecl: Let declarations
//   - TernaryExpr: Ternary operator
//   - NullCoalesceExpr: Null coalescing (??)
//   - SafeNavExpr: Safe navigation (?.)
//   - RawExpr: Pass-through Go code
//
// Returns CodeGenResult with generated Go code and source mappings.
func GenerateExpr(expr ast.Expr) ast.CodeGenResult {
	if expr == nil {
		return ast.CodeGenResult{}
	}

	switch e := expr.(type) {
	case *ast.MatchExpr:
		return NewMatchCodeGen(e).Generate()
	case *ast.LambdaExpr:
		return NewLambdaCodeGen(e).Generate()
	case *ast.ErrorPropExpr:
		return NewErrorPropCodeGen(e).Generate()
	case *ast.TernaryExpr:
		return NewTernaryCodeGen(e).Generate()
	case *ast.NullCoalesceExpr:
		return NewNullCoalesceCodeGen(e).Generate()
	case *ast.SafeNavExpr:
		return NewSafeNavCodeGen(e).Generate()
	case *ast.SafeNavCallExpr:
		return NewSafeNavCallCodeGen(e).Generate()
	case *ast.BuiltinCallExpr:
		return NewBuiltinCallCodeGen(e).Generate()
	case *ast.RawExpr:
		// RawExpr is pass-through - just return the text
		return ast.NewCodeGenResult([]byte(e.Text))
	default:
		// Unknown expression - use String() method if available
		if stringer, ok := expr.(interface{ String() string }); ok {
			return ast.NewCodeGenResult([]byte(stringer.String()))
		}
		return ast.CodeGenResult{}
	}
}

// Stub implementations (to be implemented in their respective files):

// NewMatchCodeGen creates a match expression codegen (match.go)
// Implementation in match.go
func NewMatchCodeGen(e *ast.MatchExpr) Generator {
	return &MatchCodeGen{
		BaseGenerator: NewBaseGenerator(),
		Match:         e,
	}
}

// NewLambdaCodeGen creates a lambda expression codegen (lambda.go)
// Implementation in lambda.go
func NewLambdaCodeGen(e *ast.LambdaExpr) Generator {
	return &LambdaCodeGen{
		BaseGenerator: NewBaseGenerator(),
		expr:          e,
	}
}

// NewErrorPropCodeGen creates an error propagation codegen (error_prop.go)
// Implementation in error_prop.go
// Note: Uses default return types (nil). For type inference, use NewErrorPropGenerator directly.
func NewErrorPropCodeGen(e *ast.ErrorPropExpr) Generator {
	return NewErrorPropGenerator(e, []string{"nil"})
}

// NewTernaryCodeGen creates a ternary operator codegen (ternary.go)
// Implementation in ternary.go
func NewTernaryCodeGen(e *ast.TernaryExpr) *TernaryCodeGen {
	return &TernaryCodeGen{
		BaseGenerator: NewBaseGenerator(),
		expr:          e,
	}
}

// NewNullCoalesceCodeGen creates a null coalescing codegen (null_coalesce.go)
func NewNullCoalesceCodeGen(e *ast.NullCoalesceExpr) Generator {
	return NewNullCoalesceGenerator(e)
}

// NewSafeNavCodeGen creates a safe navigation codegen (safe_nav.go)
func NewSafeNavCodeGen(e *ast.SafeNavExpr) Generator {
	return NewSafeNavGenerator(e)
}

// NewSafeNavCallCodeGen creates a safe navigation call codegen (safe_nav.go)
func NewSafeNavCallCodeGen(e *ast.SafeNavCallExpr) Generator {
	return NewSafeNavCallGenerator(e)
}

// stubGenerator is a placeholder for unimplemented codegens
type stubGenerator struct{}

func (s *stubGenerator) Generate() ast.CodeGenResult {
	return ast.CodeGenResult{}
}
