package codegen

import (
	"bytes"
	"fmt"
	"strings"

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
// With context, generates human-like if/else statements.
// Without context, generates IIFE for expression compatibility.
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

	// Collect the chain segments (flattening nested SafeNavExpr)
	chain, baseReceiver := g.collectChain()

	// Validate chain is not empty (defensive programming)
	if len(chain) == 0 || baseReceiver == "" {
		return ast.CodeGenResult{}
	}

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

// generateHumanLikeReturn generates human-readable code for return statements.
//
// Input: return config?.Database?.Host
// Output:
//
//	tmp := config
//	if tmp == nil { return nil }
//	tmp1 := tmp.Database
//	if tmp1 == nil { return nil }
//	return tmp1.Host
//
// This uses temporaries to avoid duplicate receiver evaluation and method call side effects.
func (g *SafeNavCodeGen) generateHumanLikeReturn(chain []chainSegment, baseReceiver string) ast.CodeGenResult {
	var output bytes.Buffer

	// Generate sequential checks with temporaries (same pattern as IIFE)
	// tmp := config
	output.WriteString("tmp := ")
	output.WriteString(baseReceiver)
	output.WriteString("\n")

	// Generate nil checks for each segment except the last
	tmpVar := "tmp"
	for i, seg := range chain[:len(chain)-1] {
		// if tmp == nil { return nil }
		output.WriteString("if ")
		output.WriteString(tmpVar)
		output.WriteString(" == nil { return nil }\n")

		// tmp1 := tmp.Database (or tmp.GetHost(arg) for methods)
		nextTmp := fmt.Sprintf("tmp%d", i+1)
		output.WriteString(nextTmp)
		output.WriteString(" := ")
		output.WriteString(tmpVar)
		output.WriteString(".")
		output.WriteString(seg.name)
		if seg.isMethod {
			output.WriteString("(")
			for j, arg := range seg.args {
				if j > 0 {
					output.WriteString(", ")
				}
				output.WriteString(g.dingoExprToString(arg))
			}
			output.WriteString(")")
		}
		output.WriteString("\n")
		tmpVar = nextTmp
	}

	// Final nil check
	output.WriteString("if ")
	output.WriteString(tmpVar)
	output.WriteString(" == nil { return nil }\n")

	// Final access (last segment)
	lastSeg := chain[len(chain)-1]
	output.WriteString("return ")
	output.WriteString(tmpVar)
	output.WriteString(".")
	output.WriteString(lastSeg.name)
	if lastSeg.isMethod {
		output.WriteString("(")
		for j, arg := range lastSeg.args {
			if j > 0 {
				output.WriteString(", ")
			}
			output.WriteString(g.dingoExprToString(arg))
		}
		output.WriteString(")")
	}

	result := g.Result()
	result.StatementOutput = output.Bytes()

	return result
}

// generateHumanLikeAssignment generates human-readable code for assignments.
//
// With VarType available (from type inference), generates:
//
//	var path *string
//	if config != nil && config.Database != nil {
//	    path = config.Database.Host
//	}
//
// Without VarType, falls back to IIFE.
func (g *SafeNavCodeGen) generateHumanLikeAssignment(chain []chainSegment, baseReceiver string) ast.CodeGenResult {
	// If no type info, fall back to IIFE
	if g.Context == nil || g.Context.VarType == "" {
		return g.generateIIFE(chain, baseReceiver)
	}

	var output bytes.Buffer

	// var path *string
	output.WriteString("var ")
	output.WriteString(g.Context.VarName)
	output.WriteString(" ")
	output.WriteString(g.Context.VarType)
	output.WriteString("\n")

	// Build condition: config != nil && config.Database != nil
	output.WriteString("if ")

	// Check base receiver
	output.WriteString(baseReceiver)
	output.WriteString(" != nil")

	// Check each intermediate segment
	currentPath := baseReceiver
	for _, seg := range chain[:len(chain)-1] {
		currentPath = currentPath + "." + seg.name
		if seg.isMethod {
			// Skip method calls in condition - can't safely check method return for nil
			// in condition without calling it twice
			continue
		}
		output.WriteString(" && ")
		output.WriteString(currentPath)
		output.WriteString(" != nil")
	}

	output.WriteString(" {\n\t")

	// Assignment: path = config.Database.Host
	output.WriteString(g.Context.VarName)
	output.WriteString(" = ")
	output.WriteString(baseReceiver)
	for _, seg := range chain {
		output.WriteString(".")
		output.WriteString(seg.name)
		if seg.isMethod {
			output.WriteString("(")
			for j, arg := range seg.args {
				if j > 0 {
					output.WriteString(", ")
				}
				output.WriteString(g.dingoExprToString(arg))
			}
			output.WriteString(")")
		}
	}
	output.WriteString("\n}")

	result := g.Result()
	result.StatementOutput = output.Bytes()

	return result
}

// generateIIFE generates an IIFE for contexts where statement-level replacement isn't possible.
func (g *SafeNavCodeGen) generateIIFE(chain []chainSegment, baseReceiver string) ast.CodeGenResult {
	g.generateIIFEContent(chain, baseReceiver)
	return g.Result()
}

// generateIIFEContent writes the IIFE code to the buffer.
func (g *SafeNavCodeGen) generateIIFEContent(chain []chainSegment, baseReceiver string) {
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

	// Check if the method is known to return void (can't use "return void_call()")
	if lastSeg.isMethod && isVoidReturningMethod(lastSeg.name) {
		// For void methods: call then return nil
		g.Write(tmpVar)
		g.Write(".")
		g.Write(lastSeg.name)
		g.WriteByte('(')
		g.generateArgsFrom(lastSeg.args)
		g.WriteByte(')')
		g.Write("; return nil }()")
	} else {
		// For value-returning methods: return the result
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

// isVoidReturningMethod checks if a method name commonly returns void.
// This is used to avoid generating invalid "return voidCall()" in safe nav IIFEs.
func isVoidReturningMethod(methodName string) bool {
	// Common void-returning method names
	voidMethods := map[string]bool{
		// Lifecycle methods
		"Start": true, "Stop": true, "Run": true, "Close": true,
		"Shutdown": true, "Cancel": true, "Init": true, "Reset": true,
		// Sync methods
		"Lock": true, "Unlock": true, "RLock": true, "RUnlock": true,
		"Wait": true, "Signal": true, "Broadcast": true,
		// Logging/output
		"Log": true, "Print": true, "Printf": true, "Println": true,
		"Debug": true, "Info": true, "Warn": true, "Error": true, "Fatal": true,
		// Context
		"Done": true,
		// Timer/cleanup
		"Tick": true, "Flush": true, "Clear": true,
		// Notification
		"Notify": true, "Fire": true, "Emit": true, "Trigger": true,
	}

	if voidMethods[methodName] {
		return true
	}

	// Check common prefixes that suggest void methods
	// NOTE: Avoided ambiguous prefixes like "add" (could return sum) and "send" (could return result)
	lower := strings.ToLower(methodName)
	voidPrefixes := []string{
		"set", "remove", "delete", "put",
		"push", "pop", "enqueue", "dequeue",
		"register", "unregister", "subscribe", "unsubscribe",
		"enable", "disable", "activate", "deactivate",
		"handle", "process", "execute", "perform",
	}
	for _, prefix := range voidPrefixes {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}

	return false
}
