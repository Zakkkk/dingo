package codegen

import (
	"strings"
	"testing"

	dingoast "github.com/MadAppGang/dingo/pkg/ast"
)

func TestNullCoalesceGenerator_Simple(t *testing.T) {
	// Test: config ?? defaultConfig
	expr := &dingoast.NullCoalesceExpr{
		Left: &dingoast.DingoIdent{
			NamePos: 1,
			Name:    "config",
		},
		OpPos: 8,
		Right: &dingoast.DingoIdent{
			NamePos: 11,
			Name:    "defaultConfig",
		},
	}

	gen := NewNullCoalesceGenerator(expr)
	result := gen.Generate()

	output := string(result.Output)

	// Verify structure
	if !strings.Contains(output, "func() interface{} {") {
		t.Error("Expected IIFE wrapper")
	}
	if !strings.Contains(output, "if config != nil {") {
		t.Error("Expected nil check for left operand")
	}
	if !strings.Contains(output, "return config") {
		t.Error("Expected return of left operand")
	}
	if !strings.Contains(output, "return defaultConfig") {
		t.Error("Expected return of right operand")
	}
	if !strings.Contains(output, "}()") {
		t.Error("Expected IIFE invocation")
	}
}

func TestNullCoalesceGenerator_Simple_MultipleVariables(t *testing.T) {
	// Test: x ?? y
	// Note: Chaining would require parser support to create nested NullCoalesceExpr
	// This test focuses on single null coalesce with different variable names
	expr := &dingoast.NullCoalesceExpr{
		Left: &dingoast.DingoIdent{
			NamePos: 1,
			Name:    "x",
		},
		OpPos: 3,
		Right: &dingoast.DingoIdent{
			NamePos: 6,
			Name:    "y",
		},
	}

	gen := NewNullCoalesceGenerator(expr)
	result := gen.Generate()

	output := string(result.Output)

	// Should have IIFE structure
	if !strings.Contains(output, "if x != nil") {
		t.Error("Expected nil check for 'x'")
	}
	if !strings.Contains(output, "return x") {
		t.Error("Expected return of 'x'")
	}
	if !strings.Contains(output, "return y") {
		t.Error("Expected fallback to 'y'")
	}
}

func TestNullCoalesceGenerator_ComplexExpressions(t *testing.T) {
	// Test: obj.field ?? getDefault()
	expr := &dingoast.NullCoalesceExpr{
		Left: &dingoast.RawExpr{
			Text: "obj.field",
		},
		OpPos: 11,
		Right: &dingoast.RawExpr{
			Text: "getDefault()",
		},
	}

	gen := NewNullCoalesceGenerator(expr)
	result := gen.Generate()

	output := string(result.Output)

	// Verify complex left expression
	if !strings.Contains(output, "obj.field != nil") {
		t.Error("Expected nil check for selector expression")
	}
	if !strings.Contains(output, "return obj.field") {
		t.Error("Expected return of selector expression")
	}

	// Verify complex right expression (function call)
	if !strings.Contains(output, "return getDefault()") {
		t.Error("Expected return of function call")
	}
}

func TestNullCoalesceGenerator_NilInput(t *testing.T) {
	gen := NewNullCoalesceGenerator(nil)
	result := gen.Generate()

	if len(result.Output) != 0 {
		t.Error("Expected empty output for nil expression")
	}
}

func TestNullCoalesceGenerator_ShortCircuit(t *testing.T) {
	// Verify that generated code structure supports short-circuit evaluation
	expr := &dingoast.NullCoalesceExpr{
		Left: &dingoast.DingoIdent{
			NamePos: 1,
			Name:    "cached",
		},
		OpPos: 8,
		Right: &dingoast.RawExpr{
			Text: "expensive()",
		},
	}

	gen := NewNullCoalesceGenerator(expr)
	result := gen.Generate()

	output := string(result.Output)

	// Verify structure ensures right side is only evaluated when left is nil
	// This is indicated by the if/return structure
	lines := strings.Split(output, "\n")

	// Find the if statement line
	var ifLineIdx int
	for i, line := range lines {
		if strings.Contains(line, "if cached != nil") {
			ifLineIdx = i
			break
		}
	}

	if ifLineIdx == 0 {
		t.Fatal("Could not find if statement")
	}

	// The return statement for right operand should come after the if block
	var rightReturnIdx int
	for i := ifLineIdx; i < len(lines); i++ {
		if strings.Contains(lines[i], "return expensive()") {
			rightReturnIdx = i
			break
		}
	}

	if rightReturnIdx <= ifLineIdx {
		t.Error("Right operand return should be outside/after the if block for short-circuit evaluation")
	}
}
