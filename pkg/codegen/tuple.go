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
//   (10, 20) → __tuple2__(10, 20)
//   (a, b, c) → __tuple3__(a, b, c)
//   ((1, 2), 3) → __tuple2__(__tuple2__(1, 2), 3)
func (g *TupleCodeGen) GenerateLiteral(lit *ast.TupleLiteral) ast.CodeGenResult {
	if lit == nil {
		return ast.CodeGenResult{}
	}

	dingoStart := int(lit.Pos())
	dingoEnd := int(lit.End())
	outputStart := g.Buf.Len()

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

	// Create source mapping
	outputEnd := g.Buf.Len()
	result := g.Result()
	result.Mappings = append(result.Mappings, ast.NewSourceMapping(
		dingoStart,
		dingoEnd,
		outputStart,
		outputEnd,
		"tuple_literal",
	))

	return result
}

// GenerateDestructure generates marker for tuple destructuring.
//
// Input: TupleDestructure AST node
// Output: __tupleDest{N}__("name1", "name2", ..., expr)
//
// Wildcards are represented as "_" string literals.
//
// Example:
//   let (x, y) = point → __tupleDest2__("x", "y", point)
//   let (x, _) = pair → __tupleDest2__("x", "_", pair)
//   let (x, _, z) = triple → __tupleDest3__("x", "_", "z", triple)
func (g *TupleCodeGen) GenerateDestructure(dest *ast.TupleDestructure) ast.CodeGenResult {
	if dest == nil {
		return ast.CodeGenResult{}
	}

	// Validate: nested destructure patterns are not yet supported
	// e.g., let ((a, b), c) = tuple would have elem.IsNested() == true
	for _, elem := range dest.Pattern {
		if elem.IsNested() {
			// Return empty result - nested destructure not supported
			// TODO: Implement nested destructure support in Pass 2
			g.Write("/* ERROR: nested tuple destructure not yet supported */")
			return g.Result()
		}
	}

	dingoStart := int(dest.Pos())
	dingoEnd := int(dest.End())
	outputStart := g.Buf.Len()

	// Marker function: __tupleDest{N}__
	elemCount := len(dest.Pattern)
	markerName := fmt.Sprintf("__tupleDest%d__", elemCount)
	g.Write(markerName)
	g.WriteByte('(')

	// Generate pattern as string literals
	for i, elem := range dest.Pattern {
		if i > 0 {
			g.Write(", ")
		}

		// Quote the identifier name (including "_" for wildcards)
		// At this point, we know elem.IsNested() is false, so Name is valid
		g.WriteByte('"')
		g.Write(elem.Name)
		g.WriteByte('"')
	}

	// Add the value expression
	g.Write(", ")
	if dest.Value != nil {
		g.Write(g.exprToGoCode(dest.Value))
	}

	g.WriteByte(')')

	// Create source mapping
	outputEnd := g.Buf.Len()
	result := g.Result()
	result.Mappings = append(result.Mappings, ast.NewSourceMapping(
		dingoStart,
		dingoEnd,
		outputStart,
		outputEnd,
		"tuple_destructure",
	))

	return result
}

// GenerateTypeAlias generates marker for tuple type alias.
//
// Input: Element types as strings
// Output: __tupleType{N}__(type1, type2, ...)
//
// Example:
//   ["int", "int"] → __tupleType2__(int, int)
//   ["string", "int"] → __tupleType2__(string, int)
func (g *TupleCodeGen) GenerateTypeAlias(elementTypes []string) ast.CodeGenResult {
	if len(elementTypes) == 0 {
		return ast.CodeGenResult{}
	}

	outputStart := g.Buf.Len()

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

	outputEnd := g.Buf.Len()
	result := g.Result()

	// Add mapping (positions are relative to output)
	result.Mappings = append(result.Mappings, ast.NewSourceMapping(
		0, // Placeholder - caller should set actual dingo positions
		0,
		outputStart,
		outputEnd,
		"tuple_type_alias",
	))

	return result
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

// formatTmpVar formats temporary variable name following CLAUDE.md naming convention.
// First tmp is unnumbered, subsequent are tmp1, tmp2, etc.
func formatTmpVar(counter int) string {
	if counter == 1 {
		return "tmp"
	}
	return "tmp" + strconv.Itoa(counter-1)
}
