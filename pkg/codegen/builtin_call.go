package codegen

import (
	"bytes"
	"fmt"

	"github.com/MadAppGang/dingo/pkg/ast"
)

// BuiltinCallCodeGen generates Go code for built-in function calls containing Dingo expressions.
//
// For expressions like len(env?.Region), generates:
//
// With hoisting (argument context):
//
//	var tmp int
//	if env != nil && env.Region != nil {
//	    tmp = len(*env.Region)
//	}
//	// ... then use "tmp" in the expression
//
// Without context (IIFE fallback):
//
//	func() int { if env != nil && env.Region != nil { return len(*env.Region) }; return 0 }()
type BuiltinCallCodeGen struct {
	*BaseGenerator
	expr    *ast.BuiltinCallExpr
	Context *GenContext // Optional context for hoisting
}

// NewBuiltinCallCodeGen creates a generator for built-in function calls.
func NewBuiltinCallCodeGen(expr *ast.BuiltinCallExpr) *BuiltinCallCodeGen {
	return &BuiltinCallCodeGen{
		BaseGenerator: NewBaseGenerator(),
		expr:          expr,
	}
}

// Generate produces Go code for the built-in function call.
//
// If the expression contains Dingo expressions and we have context,
// generates hoisted code that can be placed before the statement.
// Otherwise, generates an IIFE.
func (g *BuiltinCallCodeGen) Generate() ast.CodeGenResult {
	if g.expr == nil {
		return ast.CodeGenResult{}
	}

	// If no Dingo expressions, just pass through
	if !g.expr.ContainsDingoExpr() {
		return ast.NewCodeGenResult([]byte(g.expr.String()))
	}

	// For argument context (inside a larger expression), use hoisting
	if g.Context != nil && g.Context.Context == ast.ContextArgument {
		return g.generateHoisted()
	}

	// Fallback to IIFE
	return g.generateIIFE()
}

// generateHoisted generates hoisted code for argument context.
//
// Input: len(env?.Region) in a ternary condition
// Output:
//   - HoistedCode: var tmp int\nif env != nil && env.Region != nil {\n\ttmp = len(*env.Region)\n}\n
//   - Output: tmp
func (g *BuiltinCallCodeGen) generateHoisted() ast.CodeGenResult {
	dingoStart := int(g.expr.Pos())
	dingoEnd := int(g.expr.End())

	// Generate unique temp variable name
	var tmpName string
	if g.Context != nil && g.Context.TempCounter != nil {
		count := *g.Context.TempCounter
		if count == 0 {
			tmpName = "tmp"
		} else {
			tmpName = fmt.Sprintf("tmp%d", count)
		}
		*g.Context.TempCounter++
	} else {
		tmpName = "tmp"
	}

	// Determine return type based on built-in function
	returnType := "int" // len and cap always return int

	// Build the hoisted code
	var hoisted bytes.Buffer

	// var tmp int
	hoisted.WriteString("var ")
	hoisted.WriteString(tmpName)
	hoisted.WriteString(" ")
	hoisted.WriteString(returnType)
	hoisted.WriteString("\n")

	// Generate nil checks and assignment for the first (and only) argument
	// For now, we only support single-argument len/cap
	if len(g.expr.Args) == 1 {
		g.generateArgHoisting(&hoisted, tmpName, g.expr.Args[0])
	}

	result := g.Result()
	result.HoistedCode = hoisted.Bytes()
	result.Output = []byte(tmpName)
	result.Mappings = append(result.Mappings, ast.NewSourceMapping(
		dingoStart,
		dingoEnd,
		0,
		len(tmpName),
		"builtin_call_hoisted",
	))

	return result
}

// generateArgHoisting generates the nil-checking code for an argument.
func (g *BuiltinCallCodeGen) generateArgHoisting(buf *bytes.Buffer, tmpName string, arg ast.Expr) {
	// Collect safe nav chain from the argument
	chain, baseReceiver := g.collectSafeNavChain(arg)

	if chain == nil {
		// Not a safe nav expression, generate simple assignment
		buf.WriteString(tmpName)
		buf.WriteString(" = ")
		buf.WriteString(g.expr.Func)
		buf.WriteString("(")
		buf.WriteString(g.dingoExprToString(arg))
		buf.WriteString(")\n")
		return
	}

	// Build nil check condition for each segment in the chain
	// For c?.Region: "c != nil && c.Region != nil"
	// For a?.b?.c: "a != nil && a.b != nil && a.b.c != nil"
	var nilChecks string
	currentPath := baseReceiver

	// First check the base receiver
	nilChecks = currentPath + " != nil"

	// Then check each segment (they're all pointer fields in a safe nav chain)
	for _, seg := range chain {
		currentPath += "." + seg.name
		nilChecks += " && " + currentPath + " != nil"
	}

	// Build the final access path (with dereference for pointer field)
	finalPath := currentPath

	// if c != nil && c.Region != nil {
	buf.WriteString("if ")
	buf.WriteString(nilChecks)
	buf.WriteString(" {\n\t")

	// tmp = len(*c.Region)
	buf.WriteString(tmpName)
	buf.WriteString(" = ")
	buf.WriteString(g.expr.Func)
	buf.WriteString("(*")
	buf.WriteString(finalPath)
	buf.WriteString(")\n}\n")
}

// generateIIFE generates an IIFE for contexts where hoisting isn't possible.
func (g *BuiltinCallCodeGen) generateIIFE() ast.CodeGenResult {
	dingoStart := int(g.expr.Pos())
	dingoEnd := int(g.expr.End())
	outputStart := g.Buf.Len()

	// Determine return type
	returnType := "int" // len and cap always return int

	// Generate IIFE
	g.Write("func() ")
	g.Write(returnType)
	g.Write(" { ")

	// Generate the body based on the argument
	if len(g.expr.Args) == 1 {
		g.generateIIFEBody(g.expr.Args[0])
	} else {
		// Fallback for unexpected cases
		g.Write("return 0 }()")
	}

	outputEnd := g.Buf.Len()

	result := g.Result()
	result.Mappings = append(result.Mappings, ast.NewSourceMapping(
		dingoStart,
		dingoEnd,
		outputStart,
		outputEnd,
		"builtin_call_iife",
	))

	return result
}

// generateIIFEBody generates the body of the IIFE.
func (g *BuiltinCallCodeGen) generateIIFEBody(arg ast.Expr) {
	chain, baseReceiver := g.collectSafeNavChain(arg)

	if chain == nil {
		// Not a safe nav, just call directly
		g.Write("return ")
		g.Write(g.expr.Func)
		g.Write("(")
		g.Write(g.dingoExprToString(arg))
		g.Write(") }()")
		return
	}

	// Generate sequential nil checks
	g.Write("tmp := ")
	g.Write(baseReceiver)
	g.Write("; ")

	tmpVar := "tmp"
	for i, seg := range chain[:len(chain)-1] {
		g.Write("if ")
		g.Write(tmpVar)
		g.Write(" == nil { return 0 }; ")

		nextTmp := fmt.Sprintf("tmp%d", i+1)
		g.Write(nextTmp)
		g.Write(" := ")
		g.Write(tmpVar)
		g.Write(".")
		g.Write(seg.name)
		g.Write("; ")
		tmpVar = nextTmp
	}

	// Final nil check
	g.Write("if ")
	g.Write(tmpVar)
	g.Write(" == nil { return 0 }; ")

	// Final access with len/cap
	lastSeg := chain[len(chain)-1]
	g.Write("return ")
	g.Write(g.expr.Func)
	g.Write("(*")
	g.Write(tmpVar)
	g.Write(".")
	g.Write(lastSeg.name)
	g.Write(") }()")
}

// safeNavSeg represents a segment in a safe navigation chain.
type safeNavSeg struct {
	name string
}

// collectSafeNavChain extracts a safe navigation chain from an expression.
// Returns nil if the expression is not a safe nav.
func (g *BuiltinCallCodeGen) collectSafeNavChain(expr ast.Expr) ([]safeNavSeg, string) {
	switch e := expr.(type) {
	case *ast.SafeNavExpr:
		return g.collectFromSafeNav(e)
	default:
		return nil, ""
	}
}

// collectFromSafeNav walks a SafeNavExpr chain and collects segments.
func (g *BuiltinCallCodeGen) collectFromSafeNav(expr *ast.SafeNavExpr) ([]safeNavSeg, string) {
	var segments []safeNavSeg

	// Walk the chain backwards
	current := expr
	for {
		segments = append([]safeNavSeg{{name: current.Sel.Name}}, segments...)

		switch x := current.X.(type) {
		case *ast.SafeNavExpr:
			current = x
		case *ast.DingoIdent:
			return segments, x.Name
		case *ast.RawExpr:
			return segments, x.Text
		default:
			return segments, g.dingoExprToString(x)
		}
	}
}

// dingoExprToString converts a Dingo expression to its string representation.
func (g *BuiltinCallCodeGen) dingoExprToString(expr ast.Expr) string {
	if expr == nil {
		return ""
	}

	switch e := expr.(type) {
	case *ast.DingoIdent:
		return e.Name
	case *ast.RawExpr:
		return e.Text
	case *ast.SafeNavExpr:
		// Generate the safe nav as IIFE
		gen := NewSafeNavGenerator(e)
		result := gen.Generate()
		return string(result.Output)
	case *ast.NullCoalesceExpr:
		gen := NewNullCoalesceGenerator(e)
		result := gen.Generate()
		return string(result.Output)
	default:
		if stringer, ok := expr.(interface{ String() string }); ok {
			return stringer.String()
		}
		return "/* unknown */"
	}
}
