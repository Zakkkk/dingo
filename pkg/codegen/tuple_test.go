package codegen

import (
	"go/token"
	"strings"
	"testing"

	"github.com/MadAppGang/dingo/pkg/ast"
)

// Test literal generation

func TestTupleCodeGen_SimpleLiteral(t *testing.T) {
	// (10, 20)
	lit := &ast.TupleLiteral{
		Lparen: token.Pos(1),
		Elements: []ast.Element{
			{Expr: "10"},
			{Expr: "20"},
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
			{Expr: "a"},
			{Expr: "b"},
			{Expr: "c"},
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
			{Expr: "foo.Bar()"},
			{Expr: "baz[0]"},
			{Expr: `someMap["key"]`},
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
			{Expr: "1"},
			{Expr: "2"},
		},
		Rparen: token.Pos(7),
	}

	outer := &ast.TupleLiteral{
		Lparen: token.Pos(1),
		Elements: []ast.Element{
			{Nested: inner},
			{Expr: "3"},
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

func TestTupleCodeGen_LiteralSourceMapping(t *testing.T) {
	// (10, 20)
	lit := &ast.TupleLiteral{
		Lparen: token.Pos(100),
		Elements: []ast.Element{
			{Expr: "10"},
			{Expr: "20"},
		},
		Rparen: token.Pos(107),
	}

	gen := NewTupleCodeGen()
	result := gen.GenerateLiteral(lit)

	// Should have one mapping
	if len(result.Mappings) != 1 {
		t.Fatalf("Expected 1 mapping, got %d", len(result.Mappings))
	}

	m := result.Mappings[0]
	if m.DingoStart != 100 {
		t.Errorf("Expected DingoStart=100, got %d", m.DingoStart)
	}

	if m.GoStart != 0 {
		t.Errorf("Expected GoStart=0, got %d", m.GoStart)
	}

	if m.GoEnd != len(result.Output) {
		t.Errorf("Expected GoEnd=%d, got %d", len(result.Output), m.GoEnd)
	}

	if m.Kind != "tuple_literal" {
		t.Errorf("Expected Kind='tuple_literal', got '%s'", m.Kind)
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
		Value:  "point",
	}

	gen := NewTupleCodeGen()
	result := gen.GenerateDestructure(dest)

	expected := `__tupleDest2__("x", "y", point)`
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
		Value:  "pair",
	}

	gen := NewTupleCodeGen()
	result := gen.GenerateDestructure(dest)

	expected := `__tupleDest2__("x", "_", pair)`
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
		Value:  "triple",
	}

	gen := NewTupleCodeGen()
	result := gen.GenerateDestructure(dest)

	expected := `__tupleDest3__("x", "_", "z", triple)`
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
		Value:  "getPoint()",
	}

	gen := NewTupleCodeGen()
	result := gen.GenerateDestructure(dest)

	expected := `__tupleDest2__("a", "b", getPoint())`
	actual := string(result.Output)

	if actual != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, actual)
	}
}

func TestTupleCodeGen_DestructureSourceMapping(t *testing.T) {
	// let (x, y) = point
	dest := &ast.TupleDestructure{
		LetPos: token.Pos(200),
		Pattern: []ast.DestructureElement{
			{Name: "x"},
			{Name: "y"},
		},
		Assign: token.Pos(211),
		Value:  "point",
	}

	gen := NewTupleCodeGen()
	result := gen.GenerateDestructure(dest)

	// Should have one mapping
	if len(result.Mappings) != 1 {
		t.Fatalf("Expected 1 mapping, got %d", len(result.Mappings))
	}

	m := result.Mappings[0]
	if m.DingoStart != 200 {
		t.Errorf("Expected DingoStart=200, got %d", m.DingoStart)
	}

	if m.Kind != "tuple_destructure" {
		t.Errorf("Expected Kind='tuple_destructure', got '%s'", m.Kind)
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

// Test GenerateFromLocation convenience method

func TestTupleCodeGen_FromLocation_Literal(t *testing.T) {
	src := []byte("let x = (10, 20)")
	loc := ast.TupleLocation{
		Kind:     ast.TupleKindLiteral,
		Start:    8,  // Position of (
		End:      16, // Position after )
		Elements: 2,
	}

	gen := NewTupleCodeGen()
	result, err := gen.GenerateFromLocation(loc, src)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := "__tuple2__(10, 20)"
	actual := string(result)

	if actual != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, actual)
	}
}

func TestTupleCodeGen_FromLocation_Destructure(t *testing.T) {
	src := []byte("let (x, y) = point")
	loc := ast.TupleLocation{
		Kind:     ast.TupleKindDestructure,
		Start:    4,  // Position of (
		End:      10, // Position after )
		Elements: 2,
		// ElementsInfo must be populated (no string manipulation per CLAUDE.md)
		ElementsInfo: []ast.ElementInfo{
			{Name: "x", Start: 5, End: 6},
			{Name: "y", Start: 8, End: 9},
		},
	}

	gen := NewTupleCodeGen()
	result, err := gen.GenerateFromLocation(loc, src)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := `__tupleDest2__("x", "y", point)`
	actual := string(result)

	if actual != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, actual)
	}
}

func TestTupleCodeGen_FromLocation_TypeAlias(t *testing.T) {
	src := []byte("type Point = (int, int)")
	loc := ast.TupleLocation{
		Kind:     ast.TupleKindTypeAlias,
		Start:    13, // Position of (
		End:      23, // Position after )
		Elements: 2,
		// ElementsInfo must be populated (no string manipulation per CLAUDE.md)
		ElementsInfo: []ast.ElementInfo{
			{Name: "int", Start: 14, End: 17},
			{Name: "int", Start: 19, End: 22},
		},
	}

	gen := NewTupleCodeGen()
	result, err := gen.GenerateFromLocation(loc, src)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := "__tupleType2__(int, int)"
	actual := string(result)

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

func TestTupleCodeGen_FromLocation_InvalidBounds(t *testing.T) {
	src := []byte("let x = (10, 20)")
	loc := ast.TupleLocation{
		Kind:     ast.TupleKindLiteral,
		Start:    100, // Out of bounds
		End:      200,
		Elements: 2,
	}

	gen := NewTupleCodeGen()
	_, err := gen.GenerateFromLocation(loc, src)

	if err == nil {
		t.Error("Expected error for invalid bounds, got nil")
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

func TestFormatTmpVar(t *testing.T) {
	tests := []struct {
		counter  int
		expected string
	}{
		{1, "tmp"},
		{2, "tmp1"},
		{3, "tmp2"},
		{10, "tmp9"},
	}

	for _, tt := range tests {
		actual := formatTmpVar(tt.counter)
		if actual != tt.expected {
			t.Errorf("formatTmpVar(%d): expected %s, got %s", tt.counter, tt.expected, actual)
		}
	}
}

func TestFormatFieldIndex(t *testing.T) {
	tests := []struct {
		index    int
		expected string
	}{
		{0, "0"},
		{1, "1"},
		{5, "5"},
		{10, "10"},
	}

	for _, tt := range tests {
		actual := formatFieldIndex(tt.index)
		if actual != tt.expected {
			t.Errorf("formatFieldIndex(%d): expected %s, got %s", tt.index, tt.expected, actual)
		}
	}
}

// Benchmark tests

func BenchmarkTupleCodeGen_SimpleLiteral(b *testing.B) {
	lit := &ast.TupleLiteral{
		Lparen: token.Pos(1),
		Elements: []ast.Element{
			{Expr: "10"},
			{Expr: "20"},
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
		Value:  "getTuple()",
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
			{Expr: "x"},
			{Expr: "y"},
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
			{Expr: "minX"},
			{Expr: "minY"},
		},
	}

	innerMax := &ast.TupleLiteral{
		Elements: []ast.Element{
			{Expr: "maxX"},
			{Expr: "maxY"},
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
		Value: "point",
	}

	gen := NewTupleCodeGen()
	result := gen.GenerateDestructure(dest)

	expected := `__tupleDest2__("x", "y", point)`
	if string(result.Output) != expected {
		t.Errorf("Expected: %s, Got: %s", expected, string(result.Output))
	}
}
