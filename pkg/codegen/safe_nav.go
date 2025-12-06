package codegen

import (
	"fmt"

	"github.com/MadAppGang/dingo/pkg/ast"
)

// chainSegment represents one step in a safe navigation chain.
// For config?.Database?.Host, we have segments: [config, Database, Host]
type chainSegment struct {
	name     string // Field or method name (or base receiver name)
	isMethod bool   // Is this a method call?
	args     []ast.Expr
}

// SafeNavCodeGen generates Go code for safe navigation expressions.
//
// With context (human-like output):
//
//	return config?.Database?.Host
//	→
//	if config != nil && config.Database != nil {
//	    return config.Database.Host
//	}
//	return nil
//
// Without context (IIFE fallback):
//
//	config?.Database?.Host → func() interface{} {
//	   tmp := config
//	   if tmp == nil { return nil }
//	   tmp1 := tmp.Database
//	   if tmp1 == nil { return nil }
//	   return tmp1.Host
//	}()
//
// This avoids duplicate receiver evaluation and ensures correct type inference.
type SafeNavCodeGen struct {
	*BaseGenerator
	expr     *ast.SafeNavExpr
	callExpr *ast.SafeNavCallExpr
	Context  *GenContext // Optional context for human-like code generation
}

// NewSafeNavGenerator creates a SafeNavCodeGen for field access (x?.field)
func NewSafeNavGenerator(expr *ast.SafeNavExpr) *SafeNavCodeGen {
	return &SafeNavCodeGen{
		BaseGenerator: NewBaseGenerator(),
		expr:          expr,
	}
}

// NewSafeNavCallGenerator creates a SafeNavCodeGen for method calls (x?.method(args))
func NewSafeNavCallGenerator(expr *ast.SafeNavCallExpr) *SafeNavCodeGen {
	return &SafeNavCodeGen{
		BaseGenerator: NewBaseGenerator(),
		callExpr:      expr,
	}
}

// Generate produces Go code for the safe navigation expression.
//
// For chained expressions like config?.Database?.Host, it generates a flat IIFE:
//
//	func() interface{} {
//	   tmp := config
//	   if tmp == nil { return nil }
//	   tmp1 := tmp.Database
//	   if tmp1 == nil { return nil }
//	   return tmp1.Host
//	}()
func (g *SafeNavCodeGen) Generate() ast.CodeGenResult {
	if g.expr == nil && g.callExpr == nil {
		return ast.CodeGenResult{}
	}

	// Track start position
	var dingoStart, dingoEnd int
	if g.expr != nil {
		dingoStart = int(g.expr.Pos())
		dingoEnd = int(g.expr.End())
	} else {
		dingoStart = int(g.callExpr.Pos())
		dingoEnd = int(g.callExpr.End())
	}
	outputStart := g.Buf.Len()

	// Collect the chain segments (flattening nested SafeNavExpr)
	chain, baseReceiver := g.collectChain()

	// Generate the flat IIFE
	g.Write("func() interface{} { ")

	// Generate base receiver assignment
	g.Write("tmp := ")
	g.Write(baseReceiver)
	g.Write("; ")

	// Generate nil checks for each segment (except the last which is the final access)
	tmpVar := "tmp"
	for i, seg := range chain[:len(chain)-1] {
		// Check current tmp
		g.Write("if ")
		g.Write(tmpVar)
		g.Write(" == nil { return nil }; ")

		// Access and assign to next tmp
		nextTmp := fmt.Sprintf("tmp%d", i+1)
		g.Write(nextTmp)
		g.Write(" := ")
		g.Write(tmpVar)
		g.Write(".")
		g.Write(seg.name)
		if seg.isMethod {
			g.WriteByte('(')
			g.generateArgsFrom(seg.args)
			g.WriteByte(')')
		}
		g.Write("; ")
		tmpVar = nextTmp
	}

	// Final nil check before the last access
	g.Write("if ")
	g.Write(tmpVar)
	g.Write(" == nil { return nil }; ")

	// Final access (the last segment)
	lastSeg := chain[len(chain)-1]
	g.Write("return ")
	g.Write(tmpVar)
	g.Write(".")
	g.Write(lastSeg.name)
	if lastSeg.isMethod {
		g.WriteByte('(')
		g.generateArgsFrom(lastSeg.args)
		g.WriteByte(')')
	}

	g.Write(" }()")

	outputEnd := g.Buf.Len()

	result := g.Result()
	result.Mappings = append(result.Mappings, ast.NewSourceMapping(
		dingoStart,
		dingoEnd,
		outputStart,
		outputEnd,
		"safe_nav",
	))

	return result
}

// collectChain flattens a nested SafeNavExpr chain into a list of segments
// and returns the base receiver expression.
// For config?.Database?.Host:
//   - Returns segments: [{Database, false, nil}, {Host, false, nil}]
//   - Returns base receiver: "config"
func (g *SafeNavCodeGen) collectChain() ([]chainSegment, string) {
	var segments []chainSegment
	var currentExpr ast.Expr

	if g.expr != nil {
		// Add the final segment (the one we're directly generating)
		segments = append(segments, chainSegment{
			name:     g.expr.Sel.Name,
			isMethod: false,
		})
		currentExpr = g.expr.X
	} else {
		// Method call variant
		segments = append(segments, chainSegment{
			name:     g.callExpr.Fun.Name,
			isMethod: true,
			args:     g.callExpr.Args,
		})
		currentExpr = g.callExpr.X
	}

	// Walk up the chain, collecting segments
	for {
		switch e := currentExpr.(type) {
		case *ast.SafeNavExpr:
			// Prepend this segment (we're walking backwards)
			segments = append([]chainSegment{{
				name:     e.Sel.Name,
				isMethod: false,
			}}, segments...)
			currentExpr = e.X
		case *ast.SafeNavCallExpr:
			// Prepend this segment
			segments = append([]chainSegment{{
				name:     e.Fun.Name,
				isMethod: true,
				args:     e.Args,
			}}, segments...)
			currentExpr = e.X
		case *ast.DingoIdent:
			// Base receiver found
			return segments, e.Name
		case *ast.RawExpr:
			// Base receiver as raw text
			return segments, e.Text
		default:
			// Some other expression as base (generate it)
			return segments, g.dingoExprToString(currentExpr)
		}
	}
}

// generateArgsFrom generates method call arguments from a slice.
func (g *SafeNavCodeGen) generateArgsFrom(args []ast.Expr) {
	for i, arg := range args {
		if i > 0 {
			g.Write(", ")
		}
		g.Write(g.dingoExprToString(arg))
	}
}

// dingoExprToString converts a Dingo Expr to its string representation.
func (g *SafeNavCodeGen) dingoExprToString(expr ast.Expr) string {
	if expr == nil {
		return ""
	}

	switch e := expr.(type) {
	case *ast.DingoIdent:
		return e.Name
	case *ast.RawExpr:
		return e.Text
	case *ast.SafeNavExpr:
		// Nested safe nav - generate recursively (for complex base receivers)
		gen := NewSafeNavGenerator(e)
		result := gen.Generate()
		return string(result.Output)
	case *ast.SafeNavCallExpr:
		// Nested safe nav call - generate recursively
		gen := NewSafeNavCallGenerator(e)
		result := gen.Generate()
		return string(result.Output)
	case *ast.NullCoalesceExpr:
		// Nested ?? - generate recursively
		gen := NewNullCoalesceGenerator(e)
		result := gen.Generate()
		return string(result.Output)
	default:
		// Fallback: try String() method if available
		if stringer, ok := expr.(interface{ String() string }); ok {
			return stringer.String()
		}
		return "/* unknown */"
	}
}
