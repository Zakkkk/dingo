package codegen

import (
	"go/token"
	"strings"
	"testing"

	"github.com/MadAppGang/dingo/pkg/ast"
)

func TestLambdaCodeGen_RustStyleSimple(t *testing.T) {
	// |x| x + 1
	lambda := &ast.LambdaExpr{
		LambdaPos: token.Pos(1),
		Style:     ast.RustStyle,
		Params: []ast.LambdaParam{
			{Name: "x", Type: ""},
		},
		ReturnType: "",
		Body:       "x + 1",
		IsBlock:    false,
	}

	gen := NewLambdaCodeGen(lambda)
	result := gen.Generate()

	expected := "func(x __TYPE_INFERENCE_NEEDED) { return x + 1 }"
	actual := string(result.Output)

	if actual != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, actual)
	}
}

func TestLambdaCodeGen_RustStyleTyped(t *testing.T) {
	// |x: int| x * 2
	lambda := &ast.LambdaExpr{
		LambdaPos: token.Pos(1),
		Style:     ast.RustStyle,
		Params: []ast.LambdaParam{
			{Name: "x", Type: "int"},
		},
		ReturnType: "",
		Body:       "x * 2",
		IsBlock:    false,
	}

	gen := NewLambdaCodeGen(lambda)
	result := gen.Generate()

	expected := "func(x int) { return x * 2 }"
	actual := string(result.Output)

	if actual != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, actual)
	}
}

func TestLambdaCodeGen_RustStyleWithReturnType(t *testing.T) {
	// |x: int| -> int { x * 2 }
	lambda := &ast.LambdaExpr{
		LambdaPos: token.Pos(1),
		Style:     ast.RustStyle,
		Params: []ast.LambdaParam{
			{Name: "x", Type: "int"},
		},
		ReturnType: "int",
		Body:       "x * 2",
		IsBlock:    false,
	}

	gen := NewLambdaCodeGen(lambda)
	result := gen.Generate()

	expected := "func(x int) int { return x * 2 }"
	actual := string(result.Output)

	if actual != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, actual)
	}
}

func TestLambdaCodeGen_RustStyleBlockBody(t *testing.T) {
	// |x: int| -> int { doSomething(); return x * 2 }
	lambda := &ast.LambdaExpr{
		LambdaPos: token.Pos(1),
		Style:     ast.RustStyle,
		Params: []ast.LambdaParam{
			{Name: "x", Type: "int"},
		},
		ReturnType: "int",
		Body:       "{ doSomething(); return x * 2 }",
		IsBlock:    true,
	}

	gen := NewLambdaCodeGen(lambda)
	result := gen.Generate()

	expected := "func(x int) int { doSomething(); return x * 2 }"
	actual := string(result.Output)

	if actual != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, actual)
	}
}

func TestLambdaCodeGen_TypeScriptStyleSimple(t *testing.T) {
	// x => x + 1
	lambda := &ast.LambdaExpr{
		LambdaPos: token.Pos(1),
		Style:     ast.TypeScriptStyle,
		Params: []ast.LambdaParam{
			{Name: "x", Type: ""},
		},
		ReturnType: "",
		Body:       "x + 1",
		IsBlock:    false,
	}

	gen := NewLambdaCodeGen(lambda)
	result := gen.Generate()

	expected := "func(x __TYPE_INFERENCE_NEEDED) { return x + 1 }"
	actual := string(result.Output)

	if actual != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, actual)
	}
}

func TestLambdaCodeGen_TypeScriptStyleTyped(t *testing.T) {
	// (x: int) => x * 2
	lambda := &ast.LambdaExpr{
		LambdaPos: token.Pos(1),
		Style:     ast.TypeScriptStyle,
		Params: []ast.LambdaParam{
			{Name: "x", Type: "int"},
		},
		ReturnType: "",
		Body:       "x * 2",
		IsBlock:    false,
	}

	gen := NewLambdaCodeGen(lambda)
	result := gen.Generate()

	expected := "func(x int) { return x * 2 }"
	actual := string(result.Output)

	if actual != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, actual)
	}
}

func TestLambdaCodeGen_MultipleParams(t *testing.T) {
	// |x, y| x + y
	lambda := &ast.LambdaExpr{
		LambdaPos: token.Pos(1),
		Style:     ast.RustStyle,
		Params: []ast.LambdaParam{
			{Name: "x", Type: ""},
			{Name: "y", Type: ""},
		},
		ReturnType: "",
		Body:       "x + y",
		IsBlock:    false,
	}

	gen := NewLambdaCodeGen(lambda)
	result := gen.Generate()

	expected := "func(x __TYPE_INFERENCE_NEEDED, y __TYPE_INFERENCE_NEEDED) { return x + y }"
	actual := string(result.Output)

	if actual != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, actual)
	}
}

func TestLambdaCodeGen_MultipleParamsTyped(t *testing.T) {
	// |x: int, y: int| -> int { x + y }
	lambda := &ast.LambdaExpr{
		LambdaPos: token.Pos(1),
		Style:     ast.RustStyle,
		Params: []ast.LambdaParam{
			{Name: "x", Type: "int"},
			{Name: "y", Type: "int"},
		},
		ReturnType: "int",
		Body:       "x + y",
		IsBlock:    false,
	}

	gen := NewLambdaCodeGen(lambda)
	result := gen.Generate()

	expected := "func(x int, y int) int { return x + y }"
	actual := string(result.Output)

	if actual != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, actual)
	}
}

func TestLambdaCodeGen_ComplexExpression(t *testing.T) {
	// |arr| len(arr) > 0 && arr[0] == "test"
	lambda := &ast.LambdaExpr{
		LambdaPos: token.Pos(1),
		Style:     ast.RustStyle,
		Params: []ast.LambdaParam{
			{Name: "arr", Type: "[]string"},
		},
		ReturnType: "bool",
		Body:       `len(arr) > 0 && arr[0] == "test"`,
		IsBlock:    false,
	}

	gen := NewLambdaCodeGen(lambda)
	result := gen.Generate()

	expected := `func(arr []string) bool { return len(arr) > 0 && arr[0] == "test" }`
	actual := string(result.Output)

	if actual != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, actual)
	}
}

func TestLambdaCodeGen_NoParams(t *testing.T) {
	// || getValue()
	lambda := &ast.LambdaExpr{
		LambdaPos:  token.Pos(1),
		Style:      ast.RustStyle,
		Params:     []ast.LambdaParam{},
		ReturnType: "",
		Body:       "getValue()",
		IsBlock:    false,
	}

	gen := NewLambdaCodeGen(lambda)
	result := gen.Generate()

	expected := "func() { return getValue() }"
	actual := string(result.Output)

	if actual != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, actual)
	}
}

func TestLambdaCodeGen_SourceMappings(t *testing.T) {
	// |x| x + 1
	lambda := &ast.LambdaExpr{
		LambdaPos: token.Pos(100),
		Style:     ast.RustStyle,
		Params: []ast.LambdaParam{
			{Name: "x", Type: "int"},
		},
		ReturnType: "",
		Body:       "x + 1",
		IsBlock:    false,
	}

	gen := NewLambdaCodeGen(lambda)
	result := gen.Generate()

	// Should have at least one mapping
	if len(result.Mappings) == 0 {
		t.Error("Expected source mappings, got none")
	}

	// First mapping should map from lambda start position
	if result.Mappings[0].DingoStart != 100 {
		t.Errorf("Expected DingoStart=100, got %d", result.Mappings[0].DingoStart)
	}

	// Should map to output start
	if result.Mappings[0].GoStart != 0 {
		t.Errorf("Expected GoStart=0, got %d", result.Mappings[0].GoStart)
	}

	// Should map to output end
	expectedLen := len(result.Output)
	if result.Mappings[0].GoEnd != expectedLen {
		t.Errorf("Expected GoEnd=%d, got %d", expectedLen, result.Mappings[0].GoEnd)
	}
}

func TestLambdaCodeGen_NilExpr(t *testing.T) {
	gen := NewLambdaCodeGen(nil)
	result := gen.Generate()

	if len(result.Output) != 0 {
		t.Errorf("Expected empty code for nil expr, got: %s", string(result.Output))
	}

	if len(result.Mappings) != 0 {
		t.Errorf("Expected no mappings for nil expr, got %d", len(result.Mappings))
	}
}

func TestLambdaCodeGen_IntegrationWithGenerateExpr(t *testing.T) {
	// Test that lambda codegen integrates with GenerateExpr dispatcher
	lambda := &ast.LambdaExpr{
		LambdaPos: token.Pos(1),
		Style:     ast.TypeScriptStyle,
		Params: []ast.LambdaParam{
			{Name: "x", Type: "int"},
		},
		ReturnType: "int",
		Body:       "x * 2",
		IsBlock:    false,
	}

	result := GenerateExpr(lambda)

	expected := "func(x int) int { return x * 2 }"
	actual := strings.TrimSpace(string(result.Output))

	if actual != expected {
		t.Errorf("GenerateExpr integration failed.\nExpected:\n%s\nGot:\n%s", expected, actual)
	}
}
