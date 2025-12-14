package codegen

import (
	"go/token"
	"strings"
	"testing"

	"github.com/MadAppGang/dingo/pkg/ast"
)

// Helper to create RawExpr from string for tests
func rawExpr(text string) ast.Expr {
	return &ast.RawExpr{Text: text}
}

// Test literal generation

func TestTupleCodeGen_SimpleLiteral(t *testing.T) {
	// (10, 20)
	lit := &ast.TupleLiteral{
		Lparen: token.Pos(1),
		Elements: []ast.Element{
			{Expr: rawExpr("10")},
			{Expr: rawExpr("20")},
		},
		Rparen: token.Pos(8),
	}

	gen := NewTupleCodeGen()
	result := gen.GenerateLiteral(lit)

	expected := "__tuple2__(10, 20)"
	actual := string(result.Output)

	if actual != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, actual)
	}
}

func TestTupleCodeGen_ThreeElementLiteral(t *testing.T) {
	// (a, b, c)
	lit := &ast.TupleLiteral{
		Lparen: token.Pos(1),
		Elements: []ast.Element{
			{Expr: rawExpr("a")},
			{Expr: rawExpr("b")},
			{Expr: rawExpr("c")},
		},
		Rparen: token.Pos(9),
	}

	gen := NewTupleCodeGen()
	result := gen.GenerateLiteral(lit)

	expected := "__tuple3__(a, b, c)"
	actual := string(result.Output)

	if actual != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, actual)
	}
}

func TestTupleCodeGen_ComplexExpressions(t *testing.T) {
	// (foo.Bar(), baz[0], someMap["key"])
	lit := &ast.TupleLiteral{
		Lparen: token.Pos(1),
		Elements: []ast.Element{
			{Expr: rawExpr("foo.Bar()")},
			{Expr: rawExpr("baz[0]")},
			{Expr: rawExpr(`someMap["key"]`)},
		},
		Rparen: token.Pos(35),
	}

	gen := NewTupleCodeGen()
	result := gen.GenerateLiteral(lit)

	expected := `__tuple3__(foo.Bar(), baz[0], someMap["key"])`
	actual := string(result.Output)

	if actual != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, actual)
	}
}

func TestTupleCodeGen_NestedLiteral(t *testing.T) {
	// ((1, 2), 3)
	inner := &ast.TupleLiteral{
		Lparen: token.Pos(2),
		Elements: []ast.Element{
			{Expr: rawExpr("1")},
			{Expr: rawExpr("2")},
		},
		Rparen: token.Pos(7),
	}

	outer := &ast.TupleLiteral{
		Lparen: token.Pos(1),
		Elements: []ast.Element{
			{Nested: inner},
			{Expr: rawExpr("3")},
		},
		Rparen: token.Pos(11),
	}

	gen := NewTupleCodeGen()
	result := gen.GenerateLiteral(outer)

	expected := "__tuple2__(__tuple2__(1, 2), 3)"
	actual := string(result.Output)

	if actual != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, actual)
	}
}

func TestTupleCodeGen_LiteralOutput(t *testing.T) {
	// (10, 20)
	lit := &ast.TupleLiteral{
		Lparen: token.Pos(100),
		Elements: []ast.Element{
			{Expr: rawExpr("10")},
			{Expr: rawExpr("20")},
		},
		Rparen: token.Pos(107),
	}

	gen := NewTupleCodeGen()
	result := gen.GenerateLiteral(lit)

	// Verify output is not empty
	if len(result.Output) == 0 {
		t.Error("Expected non-empty output for tuple literal")
	}
}

// Test destructuring generation

func TestTupleCodeGen_SimpleDestructure(t *testing.T) {
	// let (x, y) = point
	dest := &ast.TupleDestructure{
		LetPos: token.Pos(1),
		Pattern: []ast.DestructureElement{
			{Name: "x"},
			{Name: "y"},
		},
		Assign: token.Pos(11),
		Value:  rawExpr("point"),
	}

	gen := NewTupleCodeGen()
	result := gen.GenerateDestructure(dest)

	expected := `__tupleDest2__("x:0", "y:1", point)`
	actual := string(result.Output)

	if actual != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, actual)
	}
}

func TestTupleCodeGen_DestructureWithWildcard(t *testing.T) {
	// let (x, _) = pair
	dest := &ast.TupleDestructure{
		LetPos: token.Pos(1),
		Pattern: []ast.DestructureElement{
			{Name: "x"},
			{Name: "_"},
		},
		Assign: token.Pos(11),
		Value:  rawExpr("pair"),
	}

	gen := NewTupleCodeGen()
	result := gen.GenerateDestructure(dest)

	// Wildcards are skipped - only x binding at index 0
	expected := `__tupleDest1__("x:0", pair)`
	actual := string(result.Output)

	if actual != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, actual)
	}
}

func TestTupleCodeGen_DestructureMultipleWildcards(t *testing.T) {
	// let (x, _, z) = triple
	dest := &ast.TupleDestructure{
		LetPos: token.Pos(1),
		Pattern: []ast.DestructureElement{
			{Name: "x"},
			{Name: "_"},
			{Name: "z"},
		},
		Assign: token.Pos(14),
		Value:  rawExpr("triple"),
	}

	gen := NewTupleCodeGen()
	result := gen.GenerateDestructure(dest)

	// Wildcards are skipped - only x at index 0 and z at index 2
	expected := `__tupleDest2__("x:0", "z:2", triple)`
	actual := string(result.Output)

	if actual != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, actual)
	}
}

func TestTupleCodeGen_DestructureComplexExpression(t *testing.T) {
	// let (a, b) = getPoint()
	dest := &ast.TupleDestructure{
		LetPos: token.Pos(1),
		Pattern: []ast.DestructureElement{
			{Name: "a"},
			{Name: "b"},
		},
		Assign: token.Pos(11),
		Value:  rawExpr("getPoint()"),
	}

	gen := NewTupleCodeGen()
	result := gen.GenerateDestructure(dest)

	expected := `__tupleDest2__("a:0", "b:1", getPoint())`
	actual := string(result.Output)

	if actual != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, actual)
	}
}

func TestTupleCodeGen_DestructureOutput(t *testing.T) {
	// let (x, y) = point
	dest := &ast.TupleDestructure{
		LetPos: token.Pos(200),
		Pattern: []ast.DestructureElement{
			{Name: "x"},
			{Name: "y"},
		},
		Assign: token.Pos(211),
		Value:  rawExpr("point"),
	}

	gen := NewTupleCodeGen()
	result := gen.GenerateDestructure(dest)

	// Verify output is not empty
	if len(result.Output) == 0 {
		t.Error("Expected non-empty output for tuple destructure")
	}
}

// Test type alias generation

func TestTupleCodeGen_SimpleTypeAlias(t *testing.T) {
	// type Point = (int, int) → __tupleType2__(int, int)
	elementTypes := []string{"int", "int"}

	gen := NewTupleCodeGen()
	result := gen.GenerateTypeAlias(elementTypes)

	expected := "__tupleType2__(int, int)"
	actual := string(result.Output)

	if actual != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, actual)
	}
}

func TestTupleCodeGen_MixedTypeAlias(t *testing.T) {
	// type Pair = (string, int) → __tupleType2__(string, int)
	elementTypes := []string{"string", "int"}

	gen := NewTupleCodeGen()
	result := gen.GenerateTypeAlias(elementTypes)

	expected := "__tupleType2__(string, int)"
	actual := string(result.Output)

	if actual != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, actual)
	}
}

func TestTupleCodeGen_ComplexTypeAlias(t *testing.T) {
	// type Triple = (*User, []string, map[string]int) → __tupleType3__(...)
	elementTypes := []string{"*User", "[]string", "map[string]int"}

	gen := NewTupleCodeGen()
	result := gen.GenerateTypeAlias(elementTypes)

	expected := "__tupleType3__(*User, []string, map[string]int)"
	actual := string(result.Output)

	if actual != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, actual)
	}
}

// Test error cases

func TestTupleCodeGen_NilLiteral(t *testing.T) {
	gen := NewTupleCodeGen()
	result := gen.GenerateLiteral(nil)

	if len(result.Output) != 0 {
		t.Errorf("Expected empty output for nil literal, got: %s", string(result.Output))
	}
}

func TestTupleCodeGen_NilDestructure(t *testing.T) {
	gen := NewTupleCodeGen()
	result := gen.GenerateDestructure(nil)

	if len(result.Output) != 0 {
		t.Errorf("Expected empty output for nil destructure, got: %s", string(result.Output))
	}
}

func TestTupleCodeGen_EmptyTypeAlias(t *testing.T) {
	gen := NewTupleCodeGen()
	result := gen.GenerateTypeAlias([]string{})

	if len(result.Output) != 0 {
		t.Errorf("Expected empty output for empty type alias, got: %s", string(result.Output))
	}
}

// Test integration with GenerateExpr dispatcher (future work)

func TestTupleCodeGen_IntegrationPlaceholder(t *testing.T) {
	// NOTE: GenerateExpr integration will be added when tuple expressions
	// are added to the expression dispatcher in expr.go
	//
	// This test is a placeholder for future integration testing
	t.Skip("Integration with GenerateExpr pending tuple expression type addition")
}

// Test helper functions

func TestFormatTplVar(t *testing.T) {
	tests := []struct {
		counter  int
		expected string
	}{
		{1, "tpl"},
		{2, "tpl1"},
		{3, "tpl2"},
		{10, "tpl9"},
	}

	for _, tt := range tests {
		actual := formatTplVar(tt.counter)
		if actual != tt.expected {
			t.Errorf("formatTplVar(%d): expected %s, got %s", tt.counter, tt.expected, actual)
		}
	}
}

// Note: formatFieldIndex is now defined in pkg/ast/tuple.go as it's used
// for AST-based tuple destructuring. Tests for it are in pkg/ast/tuple_test.go.

// Benchmark tests

func BenchmarkTupleCodeGen_SimpleLiteral(b *testing.B) {
	lit := &ast.TupleLiteral{
		Lparen: token.Pos(1),
		Elements: []ast.Element{
			{Expr: rawExpr("10")},
			{Expr: rawExpr("20")},
		},
		Rparen: token.Pos(8),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gen := NewTupleCodeGen()
		_ = gen.GenerateLiteral(lit)
	}
}

func BenchmarkTupleCodeGen_ComplexDestructure(b *testing.B) {
	dest := &ast.TupleDestructure{
		LetPos: token.Pos(1),
		Pattern: []ast.DestructureElement{
			{Name: "a"},
			{Name: "_"},
			{Name: "b"},
			{Name: "_"},
			{Name: "c"},
		},
		Assign: token.Pos(20),
		Value:  rawExpr("getTuple()"),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gen := NewTupleCodeGen()
		_ = gen.GenerateDestructure(dest)
	}
}

// Real-world example tests

func TestTupleCodeGen_RealWorld_Point2D(t *testing.T) {
	// From geometry.dingo: return (x, y)
	lit := &ast.TupleLiteral{
		Lparen: token.Pos(100),
		Elements: []ast.Element{
			{Expr: rawExpr("x")},
			{Expr: rawExpr("y")},
		},
		Rparen: token.Pos(106),
	}

	gen := NewTupleCodeGen()
	result := gen.GenerateLiteral(lit)

	expected := "__tuple2__(x, y)"
	if string(result.Output) != expected {
		t.Errorf("Expected: %s, Got: %s", expected, string(result.Output))
	}
}

func TestTupleCodeGen_RealWorld_BoundingBox(t *testing.T) {
	// From geometry.dingo: return ((minX, minY), (maxX, maxY))
	innerMin := &ast.TupleLiteral{
		Elements: []ast.Element{
			{Expr: rawExpr("minX")},
			{Expr: rawExpr("minY")},
		},
	}

	innerMax := &ast.TupleLiteral{
		Elements: []ast.Element{
			{Expr: rawExpr("maxX")},
			{Expr: rawExpr("maxY")},
		},
	}

	outer := &ast.TupleLiteral{
		Elements: []ast.Element{
			{Nested: innerMin},
			{Nested: innerMax},
		},
	}

	gen := NewTupleCodeGen()
	result := gen.GenerateLiteral(outer)

	expected := "__tuple2__(__tuple2__(minX, minY), __tuple2__(maxX, maxY))"
	actual := strings.TrimSpace(string(result.Output))

	if actual != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, actual)
	}
}

func TestTupleCodeGen_RealWorld_Destructure(t *testing.T) {
	// From geometry.dingo: let (x, y) = point
	dest := &ast.TupleDestructure{
		Pattern: []ast.DestructureElement{
			{Name: "x"},
			{Name: "y"},
		},
		Value: rawExpr("point"),
	}

	gen := NewTupleCodeGen()
	result := gen.GenerateDestructure(dest)

	expected := `__tupleDest2__("x:0", "y:1", point)`
	if string(result.Output) != expected {
		t.Errorf("Expected: %s, Got: %s", expected, string(result.Output))
	}
}
