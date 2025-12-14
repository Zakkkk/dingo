package codegen

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/MadAppGang/dingo/pkg/ast"
)

// TupleCodeGen generates marker-based Go code for tuple expressions (Pass 1).
//
// This is the FIRST pass of a two-pass pipeline:
// - Pass 1 (this): Transform tuple syntax to markers (no type info needed)
// - Pass 2 (tuple_types.go): Use go/types to resolve markers to final structs
//
// Marker formats:
// - Literals: __tuple{N}__(elem1, elem2, ...) where N is element count
// - Destructuring: __tupleDest{N}__("name1", "name2", ..., expr)
// - Type aliases: __tupleType{N}__(type1, type2, ...)
type TupleCodeGen struct {
	*BaseGenerator
}

// NewTupleCodeGen creates a tuple codegen for Pass 1 marker generation.
func NewTupleCodeGen() *TupleCodeGen {
	return &TupleCodeGen{
		BaseGenerator: NewBaseGenerator(),
	}
}

// GenerateLiteral generates marker function call for tuple literal.
//
// Input: TupleLiteral AST node
// Output: __tuple{N}__(elem1, elem2, ...)
//
// Example:
//
//	(10, 20) → __tuple2__(10, 20)
//	(a, b, c) → __tuple3__(a, b, c)
//	((1, 2), 3) → __tuple2__(__tuple2__(1, 2), 3)
func (g *TupleCodeGen) GenerateLiteral(lit *ast.TupleLiteral) ast.CodeGenResult {
	if lit == nil {
		return ast.CodeGenResult{}
	}

	// Marker function: __tuple{N}__
	elemCount := len(lit.Elements)
	markerName := fmt.Sprintf("__tuple%d__", elemCount)
	g.Write(markerName)
	g.WriteByte('(')

	// Generate elements
	for i, elem := range lit.Elements {
		if i > 0 {
			g.Write(", ")
		}

		if elem.Nested != nil {
			// Nested tuple - recursive generation
			nestedGen := NewTupleCodeGen()
			nestedResult := nestedGen.GenerateLiteral(elem.Nested)
			g.Write(string(nestedResult.Output))
		} else if elem.Expr != nil {
			// Simple expression - convert AST to Go code
			g.Write(g.exprToGoCode(elem.Expr))
		}
	}

	g.WriteByte(')')

	return g.Result()
}

// collectBindings recursively collects variable bindings with path encoding.
// Path is dot-separated indices: "0.0" means First.First
// Wildcards (_) are skipped entirely - no binding generated.
func collectBindings(pattern []ast.DestructureElement, pathPrefix string) []string {
	var bindings []string
	for i, elem := range pattern {
		path := pathPrefix
		if path != "" {
			path += "."
		}
		path += strconv.Itoa(i)

		if elem.IsNested() {
			// Recursively collect nested bindings
			bindings = append(bindings, collectBindings(elem.Nested, path)...)
		} else if elem.Name != "_" {
			// Skip wildcards - only add named bindings
			bindings = append(bindings, fmt.Sprintf(`"%s:%s"`, elem.Name, path))
		}
	}
	return bindings
}

// GenerateDestructure generates marker for tuple destructuring.
//
// Input: TupleDestructure AST node
// Output: __tupleDest{N}__("name1:path1", "name2:path2", ..., expr)
//
// Wildcards are skipped entirely (no binding generated).
// Nested patterns are flattened with dot-separated paths.
//
// Example:
//
//	let (x, y) = point → __tupleDest2__("x:0", "y:1", point)
//	let (x, _) = pair → __tupleDest1__("x:0", pair)
//	let ((a, b), c) = nested → __tupleDest3__("a:0.0", "b:0.1", "c:1", nested)
//	let ((_, b), _) = nested → __tupleDest1__("b:0.1", nested)
func (g *TupleCodeGen) GenerateDestructure(dest *ast.TupleDestructure) ast.CodeGenResult {
	if dest == nil {
		return ast.CodeGenResult{}
	}

	// Collect all bindings with path encoding (handles nesting and skips wildcards)
	bindings := collectBindings(dest.Pattern, "")

	if len(bindings) == 0 {
		// All wildcards - generate minimal statement
		g.Write("_ = ")
		if dest.Value != nil {
			g.Write(g.exprToGoCode(dest.Value))
		}
		return g.Result()
	}

	// Marker function: __tupleDest{N}__ where N is number of actual bindings
	markerName := fmt.Sprintf("__tupleDest%d__", len(bindings))
	g.Write(markerName)
	g.WriteByte('(')

	// Write all bindings
	for i, binding := range bindings {
		if i > 0 {
			g.Write(", ")
		}
		g.Write(binding)
	}

	// Add the value expression
	g.Write(", ")
	if dest.Value != nil {
		g.Write(g.exprToGoCode(dest.Value))
	}

	g.WriteByte(')')

	return g.Result()
}

// GenerateTypeAlias generates marker for tuple type alias.
//
// Input: Element types as strings
// Output: __tupleType{N}__(type1, type2, ...)
//
// Example:
//
//	["int", "int"] → __tupleType2__(int, int)
//	["string", "int"] → __tupleType2__(string, int)
func (g *TupleCodeGen) GenerateTypeAlias(elementTypes []string) ast.CodeGenResult {
	if len(elementTypes) == 0 {
		return ast.CodeGenResult{}
	}

	// Marker function: __tupleType{N}__
	elemCount := len(elementTypes)
	markerName := fmt.Sprintf("__tupleType%d__", elemCount)
	g.Write(markerName)
	g.WriteByte('(')

	// Generate type elements
	for i, typeStr := range elementTypes {
		if i > 0 {
			g.Write(", ")
		}
		g.Write(typeStr)
	}

	g.WriteByte(')')

	return g.Result()
}

// exprToGoCode converts an ast.Expr to Go source code string.
// This handles both Dingo-specific expressions and generic expressions.
//
// Supported Dingo expressions:
//   - ErrorPropExpr (?)
//   - SafeNavExpr (?.)
//   - SafeNavCallExpr (?.method())
//   - NullCoalesceExpr (??)
//   - TupleLiteral (nested tuples)
//   - DingoIdent (identifiers)
//   - RawExpr (raw Go code)
//
// For other expressions, falls back to String() method.
func (g *TupleCodeGen) exprToGoCode(expr ast.Expr) string {
	if expr == nil {
		return ""
	}

	switch e := expr.(type) {
	case *ast.DingoIdent:
		// Simple identifier
		return e.Name

	case *ast.RawExpr:
		// Raw Go expression text
		return e.Text

	case *ast.TupleLiteral:
		// Nested tuple - generate marker recursively
		nestedGen := NewTupleCodeGen()
		result := nestedGen.GenerateLiteral(e)
		return string(result.Output)

	case *ast.ErrorPropExpr:
		// Error propagation: expr?
		// Generate marker that will be processed by error_prop feature
		operandCode := g.exprToGoCode(e.Operand)
		return fmt.Sprintf("__errorProp__(%s)", operandCode)

	case *ast.SafeNavExpr:
		// Safe navigation: expr?.field
		// Generate marker that will be processed by safe_nav feature
		receiverCode := g.exprToGoCode(e.X)
		return fmt.Sprintf("__safeNav__(%s, %q)", receiverCode, e.Sel.Name)

	case *ast.SafeNavCallExpr:
		// Safe navigation call: expr?.method(args)
		// Generate marker that will be processed by safe_nav feature
		receiverCode := g.exprToGoCode(e.X)
		var argsCode []string
		for _, arg := range e.Args {
			argsCode = append(argsCode, g.exprToGoCode(arg))
		}
		return fmt.Sprintf("__safeNavCall__(%s, %q, %s)", receiverCode, e.Fun.Name, strings.Join(argsCode, ", "))

	case *ast.NullCoalesceExpr:
		// Null coalesce: a ?? b
		// Generate marker that will be processed by null_coalesce feature
		leftCode := g.exprToGoCode(e.Left)
		rightCode := g.exprToGoCode(e.Right)
		return fmt.Sprintf("__nullCoalesce__(%s, %s)", leftCode, rightCode)

	default:
		// Fallback: use String() method for other expression types
		// This handles BinaryExpr, TernaryExpr, MatchExpr, etc.
		return expr.String()
	}
}

// formatTplVar formats tuple variable name following CLAUDE.md naming convention.
// First tpl is unnumbered, subsequent are tpl1, tpl2, etc.
func formatTplVar(counter int) string {
	if counter == 1 {
		return "tpl"
	}
	return "tpl" + strconv.Itoa(counter-1)
}
