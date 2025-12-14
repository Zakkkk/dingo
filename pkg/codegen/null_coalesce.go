package codegen

import (
	"bytes"
	"fmt"

	"github.com/MadAppGang/dingo/pkg/ast"
)

// NullCoalesceGenerator generates Go code for null coalescing expressions.
//
// With context (human-like output):
//
//	return config?.Host ?? "default"
//	→
//	if config != nil && config.Host != nil {
//	    return *config.Host
//	}
//	return "default"
//
// Without context (IIFE fallback):
//
//	a ?? b → func() TYPE { if a != nil { return a } return b }()
type NullCoalesceGenerator struct {
	*BaseGenerator
	expr    *ast.NullCoalesceExpr
	Context *GenContext // Optional context for human-like code generation
}

// NewNullCoalesceGenerator creates a generator for null coalescing expressions.
func NewNullCoalesceGenerator(expr *ast.NullCoalesceExpr) *NullCoalesceGenerator {
	return &NullCoalesceGenerator{
		BaseGenerator: NewBaseGenerator(),
		expr:          expr,
	}
}

// Generate produces Go code for the null coalescing expression.
//
// For safe nav on left (config?.Database?.Host ?? "default"):
//
//	func() TYPE {
//	    tmp := config
//	    if tmp == nil { return "default" }
//	    tmp1 := tmp.Database
//	    if tmp1 == nil { return "default" }
//	    return tmp1.Host
//	}()
//
// For simple null coalesce (a ?? b):
//
//	func() TYPE {
//	    if a != nil { return a }
//	    return b
//	}()
func (g *NullCoalesceGenerator) Generate() ast.CodeGenResult {
	if g.expr == nil {
		return ast.CodeGenResult{}
	}

	// Check if left is a safe nav chain - if so, generate combined IIFE
	if chain, baseReceiver := g.getSafeNavChain(g.expr.Left); chain != nil {
		return g.generateCombinedSafeNavCoalesce(chain, baseReceiver)
	}

	// Standard null coalesce generation
	return g.generateStandard()
}

// getSafeNavChain extracts a safe nav chain from an expression if present.
// Returns nil if the expression is not a safe nav.
func (g *NullCoalesceGenerator) getSafeNavChain(expr ast.Expr) ([]safeNavSegment, string) {
	switch e := expr.(type) {
	case *ast.SafeNavExpr:
		return g.collectSafeNavChain(e, nil)
	case *ast.SafeNavCallExpr:
		return g.collectSafeNavCallChain(e, nil)
	default:
		return nil, ""
	}
}

// safeNavSegment represents one step in a safe navigation chain.
type safeNavSegment struct {
	name     string
	isMethod bool
	args     []ast.Expr
}

// collectSafeNavChain walks a SafeNavExpr chain and collects segments.
func (g *NullCoalesceGenerator) collectSafeNavChain(expr *ast.SafeNavExpr, segments []safeNavSegment) ([]safeNavSegment, string) {
	// Add this segment
	segments = append(segments, safeNavSegment{
		name:     expr.Sel.Name,
		isMethod: false,
	})

	// Walk up to find base receiver
	switch x := expr.X.(type) {
	case *ast.SafeNavExpr:
		return g.collectSafeNavChain(x, segments)
	case *ast.SafeNavCallExpr:
		return g.collectSafeNavCallChain(x, segments)
	case *ast.DingoIdent:
		// Reverse segments (we walked backwards)
		reversed := make([]safeNavSegment, len(segments))
		for i := range segments {
			reversed[len(segments)-1-i] = segments[i]
		}
		return reversed, x.Name
	case *ast.RawExpr:
		reversed := make([]safeNavSegment, len(segments))
		for i := range segments {
			reversed[len(segments)-1-i] = segments[i]
		}
		return reversed, x.Text
	default:
		// Complex base receiver
		reversed := make([]safeNavSegment, len(segments))
		for i := range segments {
			reversed[len(segments)-1-i] = segments[i]
		}
		return reversed, g.dingoExprToString(x)
	}
}

// collectSafeNavCallChain walks a SafeNavCallExpr chain.
func (g *NullCoalesceGenerator) collectSafeNavCallChain(expr *ast.SafeNavCallExpr, segments []safeNavSegment) ([]safeNavSegment, string) {
	segments = append(segments, safeNavSegment{
		name:     expr.Fun.Name,
		isMethod: true,
		args:     expr.Args,
	})

	switch x := expr.X.(type) {
	case *ast.SafeNavExpr:
		return g.collectSafeNavChain(x, segments)
	case *ast.SafeNavCallExpr:
		return g.collectSafeNavCallChain(x, segments)
	case *ast.DingoIdent:
		reversed := make([]safeNavSegment, len(segments))
		for i := range segments {
			reversed[len(segments)-1-i] = segments[i]
		}
		return reversed, x.Name
	case *ast.RawExpr:
		reversed := make([]safeNavSegment, len(segments))
		for i := range segments {
			reversed[len(segments)-1-i] = segments[i]
		}
		return reversed, x.Text
	default:
		reversed := make([]safeNavSegment, len(segments))
		for i := range segments {
			reversed[len(segments)-1-i] = segments[i]
		}
		return reversed, g.dingoExprToString(x)
	}
}

// generateCombinedSafeNavCoalesce generates code for safe nav + null coalesce.
// With context, generates human-like if/else statements.
// Without context, generates IIFE for expression compatibility.
func (g *NullCoalesceGenerator) generateCombinedSafeNavCoalesce(chain []safeNavSegment, baseReceiver string) ast.CodeGenResult {
	// If we have return context, generate human-like code
	if g.Context != nil && g.Context.Context == ast.ContextReturn {
		return g.generateHumanLikeReturn(chain, baseReceiver)
	}

	// If we have assignment context, generate human-like code
	if g.Context != nil && g.Context.Context == ast.ContextAssignment {
		return g.generateHumanLikeAssignment(chain, baseReceiver)
	}

	// Fall back to IIFE for other contexts or when context not available
	return g.generateIIFE(chain, baseReceiver)
}

// generateHumanLikeReturn generates human-readable code for return statements.
//
// Input: return config?.Database?.Host ?? "localhost"
// Output:
//
//	if config != nil && config.Database != nil {
//	    return config.Database.Host
//	}
//	return "localhost"
func (g *NullCoalesceGenerator) generateHumanLikeReturn(chain []safeNavSegment, baseReceiver string) ast.CodeGenResult {
	defaultValue := g.dingoExprToString(g.expr.Right)

	// Build the nil check condition: config != nil && config.Database != nil
	var nilChecks string
	currentPath := baseReceiver
	for i, seg := range chain[:len(chain)-1] {
		if i > 0 {
			nilChecks += " && "
		}
		nilChecks += currentPath + " != nil"
		currentPath += "." + seg.name
		if seg.isMethod {
			currentPath += "("
			for j, arg := range seg.args {
				if j > 0 {
					currentPath += ", "
				}
				currentPath += g.dingoExprToString(arg)
			}
			currentPath += ")"
		}
	}

	// Add final receiver check
	if len(chain) > 1 {
		nilChecks += " && " + currentPath + " != nil"
	} else {
		nilChecks = baseReceiver + " != nil"
	}

	// Build the value access path
	valuePath := currentPath + "." + chain[len(chain)-1].name
	if chain[len(chain)-1].isMethod {
		valuePath += "("
		for j, arg := range chain[len(chain)-1].args {
			if j > 0 {
				valuePath += ", "
			}
			valuePath += g.dingoExprToString(arg)
		}
		valuePath += ")"
	}

	// For single-segment chains with pointer fields, add extra nil check and dereference
	var returnStmt string
	if len(chain) == 1 {
		// Single segment like config?.Host where Host is likely *string
		nilChecks += " && " + valuePath + " != nil"
		returnStmt = "*" + valuePath
	} else {
		returnStmt = valuePath
	}

	// Generate the statement-level code
	var output bytes.Buffer
	output.WriteString("if ")
	output.WriteString(nilChecks)
	output.WriteString(" {\n\treturn ")
	output.WriteString(returnStmt)
	output.WriteString("\n}\nreturn ")
	output.WriteString(defaultValue)

	result := g.Result()
	result.StatementOutput = output.Bytes()

	// Also generate IIFE for Output field (backward compatibility)
	g.generateIIFEContent(chain, baseReceiver, defaultValue)
	result.Output = g.Buf.Bytes()

	return result
}

// generateHumanLikeAssignment generates human-readable code for assignments.
//
// Input: x := config?.Database?.Host ?? "localhost"
// Output:
//
//	var x string
//	if config != nil && config.Database != nil {
//	    x = config.Database.Host
//	} else {
//	    x = "localhost"
//	}
func (g *NullCoalesceGenerator) generateHumanLikeAssignment(chain []safeNavSegment, baseReceiver string) ast.CodeGenResult {
	defaultValue := g.dingoExprToString(g.expr.Right)
	varName := g.Context.VarName
	returnType := g.inferType(nil, g.expr.Right)

	// Build the nil check condition
	var nilChecks string
	currentPath := baseReceiver
	for i, seg := range chain[:len(chain)-1] {
		if i > 0 {
			nilChecks += " && "
		}
		nilChecks += currentPath + " != nil"
		currentPath += "." + seg.name
		if seg.isMethod {
			currentPath += "("
			for j, arg := range seg.args {
				if j > 0 {
					currentPath += ", "
				}
				currentPath += g.dingoExprToString(arg)
			}
			currentPath += ")"
		}
	}

	// Add final receiver check
	if len(chain) > 1 {
		nilChecks += " && " + currentPath + " != nil"
	} else {
		nilChecks = baseReceiver + " != nil"
	}

	// Build the value access path
	valuePath := currentPath + "." + chain[len(chain)-1].name
	if chain[len(chain)-1].isMethod {
		valuePath += "("
		for j, arg := range chain[len(chain)-1].args {
			if j > 0 {
				valuePath += ", "
			}
			valuePath += g.dingoExprToString(arg)
		}
		valuePath += ")"
	}

	// For single-segment chains with pointer fields
	var valueExpr string
	if len(chain) == 1 {
		nilChecks += " && " + valuePath + " != nil"
		valueExpr = "*" + valuePath
	} else {
		valueExpr = valuePath
	}

	// Generate the statement-level code
	var output bytes.Buffer
	output.WriteString("var ")
	output.WriteString(varName)
	output.WriteString(" ")
	output.WriteString(returnType)
	output.WriteString("\nif ")
	output.WriteString(nilChecks)
	output.WriteString(" {\n\t")
	output.WriteString(varName)
	output.WriteString(" = ")
	output.WriteString(valueExpr)
	output.WriteString("\n} else {\n\t")
	output.WriteString(varName)
	output.WriteString(" = ")
	output.WriteString(defaultValue)
	output.WriteString("\n}")

	result := g.Result()
	result.StatementOutput = output.Bytes()

	// Also generate IIFE for Output field (backward compatibility)
	g.generateIIFEContent(chain, baseReceiver, defaultValue)
	result.Output = g.Buf.Bytes()

	return result
}

// generateIIFE generates an IIFE for contexts where statement-level replacement isn't possible.
func (g *NullCoalesceGenerator) generateIIFE(chain []safeNavSegment, baseReceiver string) ast.CodeGenResult {
	defaultValue := g.dingoExprToString(g.expr.Right)
	g.generateIIFEContent(chain, baseReceiver, defaultValue)

	return g.Result()
}

// generateIIFEContent writes the IIFE code to the buffer.
func (g *NullCoalesceGenerator) generateIIFEContent(chain []safeNavSegment, baseReceiver, defaultValue string) {
	returnType := g.inferType(nil, g.expr.Right)

	g.Write("func() ")
	g.Write(returnType)
	g.Write(" { ")

	// Base receiver assignment
	g.Write("tmp := ")
	g.Write(baseReceiver)
	g.Write("; ")

	// Generate nil checks with default returns for all but the last segment
	tmpVar := "tmp"
	for i, seg := range chain[:len(chain)-1] {
		g.Write("if ")
		g.Write(tmpVar)
		g.Write(" == nil { return ")
		g.Write(defaultValue)
		g.Write(" }; ")

		nextTmp := fmt.Sprintf("tmp%d", i+1)
		g.Write(nextTmp)
		g.Write(" := ")
		g.Write(tmpVar)
		g.Write(".")
		g.Write(seg.name)
		if seg.isMethod {
			g.WriteByte('(')
			for j, arg := range seg.args {
				if j > 0 {
					g.Write(", ")
				}
				g.Write(g.dingoExprToString(arg))
			}
			g.WriteByte(')')
		}
		g.Write("; ")
		tmpVar = nextTmp
	}

	// Final nil check on the receiver
	g.Write("if ")
	g.Write(tmpVar)
	g.Write(" == nil { return ")
	g.Write(defaultValue)
	g.Write(" }; ")

	// Access the final field/method
	lastSeg := chain[len(chain)-1]
	finalAccessor := tmpVar + "." + lastSeg.name
	if lastSeg.isMethod {
		finalAccessor += "("
		for j, arg := range lastSeg.args {
			if j > 0 {
				finalAccessor += ", "
			}
			finalAccessor += g.dingoExprToString(arg)
		}
		finalAccessor += ")"
	}

	if len(chain) == 1 {
		g.Write("v := ")
		g.Write(finalAccessor)
		g.Write("; if v == nil { return ")
		g.Write(defaultValue)
		g.Write(" }; return *v }()")
	} else {
		g.Write("return ")
		g.Write(finalAccessor)
		g.Write(" }()")
	}
}

// generateStandard generates a standard null coalesce IIFE.
// Handles pointer-to-value coalescing: *string ?? "default" → string
func (g *NullCoalesceGenerator) generateStandard() ast.CodeGenResult {
	// Infer return type from operands (use right side type for pointer ?? value case)
	returnType := g.inferType(g.expr.Left, g.expr.Right)

	// Check if right is a literal value (needs dereference when left is pointer)
	needsDeref := g.needsDereference()

	// Generate left expression
	leftSrc := g.dingoExprToString(g.expr.Left)

	// Generate IIFE wrapper: func() TYPE {
	g.Write("func() ")
	g.Write(returnType)
	g.Write(" {\n")
	g.Write("\tif ")

	// Generate left expression for nil check
	g.Write(leftSrc)

	// nil check
	g.Write(" != nil {\n")
	g.Write("\t\treturn ")

	// Return left expression (with dereference if pointer ?? value)
	if needsDeref {
		g.Write("*")
	}
	g.Write(leftSrc)
	g.Write("\n\t}\n")

	// Return right expression (default value)
	g.Write("\treturn ")
	rightSrc := g.dingoExprToString(g.expr.Right)
	g.Write(rightSrc)
	g.Write("\n}()")

	return g.Result()
}

// dingoExprToString converts Dingo AST expression to Go source string
func (g *NullCoalesceGenerator) dingoExprToString(expr ast.Expr) string {
	if expr == nil {
		return ""
	}

	switch e := expr.(type) {
	case *ast.DingoIdent:
		return e.Name
	case *ast.RawExpr:
		return e.Text
	case *ast.NullCoalesceExpr:
		// Nested ?? - generate recursively
		gen := NewNullCoalesceGenerator(e)
		result := gen.Generate()
		return string(result.Output)
	case *ast.SafeNavExpr:
		// Nested ?. field access - generate recursively
		gen := NewSafeNavGenerator(e)
		result := gen.Generate()
		return string(result.Output)
	case *ast.SafeNavCallExpr:
		// Nested ?. method call - generate recursively
		gen := NewSafeNavCallGenerator(e)
		result := gen.Generate()
		return string(result.Output)
	default:
		// Fallback: use String() method if available
		if stringer, ok := expr.(interface{ String() string }); ok {
			return stringer.String()
		}
		return "/* unknown */"
	}
}

// needsDereference checks if the left operand needs to be dereferenced.
// This is true when right is a non-pointer literal (string, int, bool).
// Example: setting ?? "default" where setting is *string → need to dereference
func (g *NullCoalesceGenerator) needsDereference() bool {
	if g.expr.Right == nil {
		return false
	}
	if raw, ok := g.expr.Right.(*ast.RawExpr); ok {
		typ := g.inferTypeFromText(raw.Text)
		// If right is a literal (string, int, bool, float64), assume left is pointer
		// This heuristic works because ?? is used for nil coalescing
		// and literals can't be nil, so left must be nilable (pointer)
		return typ == "string" || typ == "int" || typ == "bool" || typ == "float64"
	}
	return false
}

// inferType attempts to infer concrete type from operands
// Tries to infer from literal values in RawExpr text
func (g *NullCoalesceGenerator) inferType(left, right ast.Expr) string {
	// Try to infer from right operand (default value) - usually a literal
	if right != nil {
		if raw, ok := right.(*ast.RawExpr); ok {
			if typ := g.inferTypeFromText(raw.Text); typ != "" {
				return typ
			}
		}
	}

	// Try to infer from left operand
	if left != nil {
		if raw, ok := left.(*ast.RawExpr); ok {
			if typ := g.inferTypeFromText(raw.Text); typ != "" {
				return typ
			}
		}
	}

	// Default to interface{} if type cannot be inferred
	return "interface{}"
}

// inferTypeFromText attempts to infer type from literal text
func (g *NullCoalesceGenerator) inferTypeFromText(text string) string {
	if len(text) == 0 {
		return ""
	}

	// String literal: "..." or `...`
	if (text[0] == '"' && text[len(text)-1] == '"') ||
		(text[0] == '`' && text[len(text)-1] == '`') {
		return "string"
	}

	// Bool literals
	if text == "true" || text == "false" {
		return "bool"
	}

	// Check for numeric literals (simplified)
	// Int: digits only
	// Float: contains '.'
	hasDigit := false
	hasDot := false
	for _, ch := range text {
		if ch >= '0' && ch <= '9' {
			hasDigit = true
		} else if ch == '.' {
			hasDot = true
		} else if ch != '-' && ch != '+' {
			// Not a numeric literal
			return ""
		}
	}

	if hasDigit {
		if hasDot {
			return "float64"
		}
		return "int"
	}

	return ""
}
