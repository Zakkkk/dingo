// Package builtin provides the TupleReturnFixer plugin
package builtin

import (
	"fmt"
	"go/ast"
	"strconv"

	"github.com/MadAppGang/dingo/pkg/plugin"
	"golang.org/x/tools/go/ast/astutil"
)

// TupleReturnFixer detects Tuple2(...), Tuple3(...), etc. function calls
// inside return statements and reverts them back to multi-value returns.
//
// The tuple preprocessor converts `(a, b)` to `Tuple2(a, b)` everywhere,
// but for return statements in functions with multiple return values,
// Go natively supports `return a, b` without wrapping.
//
// Example transformation:
//   Before: return Tuple2(x, y)
//   After:  return x, y
//
// This plugin runs in the Transform phase after tuple type inference.
type TupleReturnFixer struct {
	ctx *plugin.Context
}

// NewTupleReturnFixer creates a new TupleReturnFixer plugin
func NewTupleReturnFixer() *TupleReturnFixer {
	return &TupleReturnFixer{}
}

// Name returns the plugin name
func (p *TupleReturnFixer) Name() string {
	return "tuple-return-fixer"
}

// SetContext sets the plugin context (ContextAware interface)
func (p *TupleReturnFixer) SetContext(ctx *plugin.Context) {
	p.ctx = ctx
}

// Process is a no-op for this plugin (no discovery phase needed)
func (p *TupleReturnFixer) Process(node ast.Node) error {
	// No discovery phase needed - we work directly in Transform
	return nil
}

// Transform walks the AST and fixes tuple calls in return statements
func (p *TupleReturnFixer) Transform(node ast.Node) (ast.Node, error) {
	if p.ctx == nil {
		return node, fmt.Errorf("plugin context not initialized")
	}

	// Walk the AST looking for return statements
	result := astutil.Apply(node, func(c *astutil.Cursor) bool {
		n := c.Node()

		// Only process ReturnStmt nodes
		returnStmt, ok := n.(*ast.ReturnStmt)
		if !ok {
			return true
		}

		// Track modification locally to avoid outer scope capture
		modified := false

		// Check each return value
		newResults := make([]ast.Expr, 0, len(returnStmt.Results))
		for _, result := range returnStmt.Results {
			// Check if this is a CallExpr to a TupleN function
			if expanded := p.expandTupleCall(result); expanded != nil {
				// Replace single TupleN(a, b, ...) with a, b, ...
				newResults = append(newResults, expanded...)
				modified = true
			} else {
				// Keep as-is
				newResults = append(newResults, result)
			}
		}

		// Update return statement only if we expanded any tuple calls
		if modified {
			returnStmt.Results = newResults
			c.Replace(returnStmt)
		}

		return true
	}, nil)

	if result != nil {
		return result, nil
	}

	return node, nil
}

// expandTupleCall checks if expr is a call to __TUPLE_N__LITERAL__{hash}(...) from preprocessor.
// If so, returns the arguments (expanding the tuple).
// If not, returns nil (keep expr as-is).
//
// CRITICAL: Only expands calls created by the tuple preprocessor (format: __TUPLE_N__LITERAL__{hash}).
// User-defined functions named Tuple2, Tuple3, etc. are NOT expanded.
func (p *TupleReturnFixer) expandTupleCall(expr ast.Expr) []ast.Expr {
	// Must be a CallExpr
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return nil
	}

	// Function must be an identifier (not a selector or complex expression)
	ident, ok := call.Fun.(*ast.Ident)
	if !ok {
		return nil
	}

	// Check if name matches preprocessor marker pattern: __TUPLE_N__LITERAL__{hash}
	// This ensures we ONLY expand tuples created by the preprocessor, not user-defined TupleN functions
	if !isPreprocessorTupleMarker(ident.Name) {
		return nil
	}

	// Extract the N from __TUPLE_N__LITERAL__{hash}
	n := extractMarkerArity(ident.Name)
	if n <= 0 {
		return nil
	}

	// Verify we have exactly N arguments
	if len(call.Args) != n {
		// Mismatch - this shouldn't happen with preprocessor-generated markers
		// Only log at debug level to reduce noise
		if p.ctx != nil && p.ctx.Logger != nil {
			p.ctx.Logger.Debugf("TupleReturnFixer: %s call has %d args, expected %d",
				ident.Name, len(call.Args), n)
		}
		return nil
	}

	// Return the arguments (expanding the tuple)
	return call.Args
}

// isPreprocessorTupleMarker checks if name matches the preprocessor marker pattern:
// __TUPLE_N__LITERAL__{hash} where N is a number
func isPreprocessorTupleMarker(name string) bool {
	// Must start with __TUPLE_
	if len(name) < len("__TUPLE_2__LITERAL__") {
		return false
	}
	if name[:8] != "__TUPLE_" {
		return false
	}

	// Find the position of __LITERAL__
	literalMarker := "__LITERAL__"
	literalIdx := -1
	for i := 8; i < len(name)-len(literalMarker); i++ {
		if name[i:i+len(literalMarker)] == literalMarker {
			literalIdx = i
			break
		}
	}

	if literalIdx == -1 {
		return false
	}

	// Extract the N between __TUPLE_ and __LITERAL__
	arityStr := name[8:literalIdx]
	if len(arityStr) == 0 {
		return false
	}

	// Verify N is all digits
	for _, c := range arityStr {
		if c < '0' || c > '9' {
			return false
		}
	}

	return true
}

// extractMarkerArity extracts the N from "__TUPLE_N__LITERAL__{hash}"
// Returns 0 if invalid format
func extractMarkerArity(name string) int {
	if !isPreprocessorTupleMarker(name) {
		return 0
	}

	// Find __LITERAL__ position
	literalMarker := "__LITERAL__"
	literalIdx := -1
	for i := 8; i < len(name)-len(literalMarker); i++ {
		if name[i:i+len(literalMarker)] == literalMarker {
			literalIdx = i
			break
		}
	}

	if literalIdx == -1 {
		return 0
	}

	// Extract and parse arity
	arityStr := name[8:literalIdx]
	n, err := strconv.Atoi(arityStr)
	if err != nil {
		return 0
	}

	return n
}
