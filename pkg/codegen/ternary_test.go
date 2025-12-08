package codegen

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/MadAppGang/dingo/pkg/ast"
)

func TestTernaryCodeGen_BasicExpression(t *testing.T) {
	// Input: isValid ? "yes" : "no"
	expr := &ast.TernaryExpr{
		CondStr:    "isValid",
		TrueStr:    `"yes"`,
		FalseStr:   `"no"`,
		ResultType: "string",
	}

	gen := NewTernaryCodeGen(expr)
	result := gen.Generate()

	got := string(result.Output)
	expected := `func() string {
    if isValid {
        return "yes"
    }
    return "no"
}()`

	if got != expected {
		t.Errorf("Generate() output mismatch\nExpected:\n%s\n\nGot:\n%s", expected, got)
	}

	// Verify source mapping exists
	if len(result.Mappings) != 1 {
		t.Errorf("Expected 1 source mapping, got %d", len(result.Mappings))
	}
	if result.Mappings[0].Kind != "ternary" {
		t.Errorf("Expected mapping kind 'ternary', got '%s'", result.Mappings[0].Kind)
	}
}

func TestTernaryCodeGen_NumericExpression(t *testing.T) {
	// Input: count > 0 ? count : 0
	expr := &ast.TernaryExpr{
		CondStr:    "count > 0",
		TrueStr:    "count",
		FalseStr:   "0",
		ResultType: "int",
	}

	gen := NewTernaryCodeGen(expr)
	result := gen.Generate()

	got := string(result.Output)

	// Verify structure
	if !strings.Contains(got, "func() int {") {
		t.Error("Missing IIFE with int return type")
	}
	if !strings.Contains(got, "if count > 0 {") {
		t.Error("Missing condition")
	}
	if !strings.Contains(got, "return count") {
		t.Error("Missing true branch return")
	}
	if !strings.Contains(got, "return 0") {
		t.Error("Missing false branch return")
	}
}

func TestTernaryCodeGen_ASTBasedExpression(t *testing.T) {
	// Parse actual Go expressions
	fset := token.NewFileSet()
	condExpr, _ := parser.ParseExpr("x != nil")
	trueExpr, _ := parser.ParseExpr("x.Value")
	falseExpr, _ := parser.ParseExpr("defaultValue")

	expr := &ast.TernaryExpr{
		Cond:       condExpr,
		True:       trueExpr,
		False:      falseExpr,
		ResultType: "string",
		Question:   token.Pos(fset.Base()),
		Colon:      token.Pos(fset.Base() + 10),
	}

	gen := NewTernaryCodeGen(expr)
	result := gen.Generate()

	got := string(result.Output)

	// Verify AST expressions are rendered correctly
	if !strings.Contains(got, "if x != nil {") {
		t.Errorf("AST condition not rendered correctly:\n%s", got)
	}
	if !strings.Contains(got, "return x.Value") {
		t.Errorf("AST true expression not rendered correctly:\n%s", got)
	}
	if !strings.Contains(got, "return defaultValue") {
		t.Errorf("AST false expression not rendered correctly:\n%s", got)
	}
}

func TestTernaryCodeGen_NoResultType(t *testing.T) {
	// Input: cond ? a : b (no explicit type)
	expr := &ast.TernaryExpr{
		CondStr:  "cond",
		TrueStr:  "a",
		FalseStr: "b",
		// No ResultType specified
	}

	gen := NewTernaryCodeGen(expr)
	result := gen.Generate()

	got := string(result.Output)

	// Should work without explicit return type (Go will infer)
	if !strings.Contains(got, "func() {") {
		t.Error("Missing IIFE declaration")
	}
	if strings.Contains(got, "func() string") || strings.Contains(got, "func() int") {
		t.Error("Should not have explicit return type when ResultType is empty")
	}
}

func TestTernaryCodeGen_ComplexCondition(t *testing.T) {
	// Input: (x > 0 && y < 10) ? "valid" : "invalid"
	expr := &ast.TernaryExpr{
		CondStr:    "(x > 0 && y < 10)",
		TrueStr:    `"valid"`,
		FalseStr:   `"invalid"`,
		ResultType: "string",
	}

	gen := NewTernaryCodeGen(expr)
	result := gen.Generate()

	got := string(result.Output)

	// Verify complex condition is preserved
	if !strings.Contains(got, "if (x > 0 && y < 10) {") {
		t.Errorf("Complex condition not preserved:\n%s", got)
	}
}

func TestTernaryCodeGen_FunctionCallExpressions(t *testing.T) {
	// Input: hasPermission() ? execute() : deny()
	expr := &ast.TernaryExpr{
		CondStr:    "hasPermission()",
		TrueStr:    "execute()",
		FalseStr:   "deny()",
		ResultType: "error",
	}

	gen := NewTernaryCodeGen(expr)
	result := gen.Generate()

	got := string(result.Output)

	// Verify function calls are preserved
	if !strings.Contains(got, "if hasPermission() {") {
		t.Error("Condition function call missing")
	}
	if !strings.Contains(got, "return execute()") {
		t.Error("True branch function call missing")
	}
	if !strings.Contains(got, "return deny()") {
		t.Error("False branch function call missing")
	}
}

func TestTernaryCodeGen_NilExpression(t *testing.T) {
	// Edge case: nil expression
	gen := NewTernaryCodeGen(nil)
	result := gen.Generate()

	if len(result.Output) != 0 {
		t.Error("Expected empty output for nil expression")
	}
	if len(result.Mappings) != 0 {
		t.Error("Expected no mappings for nil expression")
	}
}

func TestTernaryCodeGen_SourceMappings(t *testing.T) {
	// Verify source mappings are correct
	startPos := token.Pos(10)
	colonPos := token.Pos(30)

	expr := &ast.TernaryExpr{
		CondStr:    "true",
		TrueStr:    "1",
		FalseStr:   "0",
		ResultType: "int",
		Question:   startPos,
		Colon:      colonPos,
	}

	gen := NewTernaryCodeGen(expr)
	result := gen.Generate()

	if len(result.Mappings) != 1 {
		t.Fatalf("Expected 1 mapping, got %d", len(result.Mappings))
	}

	mapping := result.Mappings[0]
	if mapping.DingoStart != int(startPos) {
		t.Errorf("Expected DingoStart=%d, got %d", startPos, mapping.DingoStart)
	}
	// TernaryExpr.End() returns Colon + 1, so expected end is colonPos + 1
	expectedEnd := int(colonPos) + 1
	if mapping.DingoEnd != expectedEnd {
		t.Errorf("Expected DingoEnd=%d, got %d", expectedEnd, mapping.DingoEnd)
	}
	if mapping.GoStart != 0 {
		t.Errorf("Expected GoStart=0, got %d", mapping.GoStart)
	}
	if mapping.GoEnd != len(result.Output) {
		t.Errorf("Expected GoEnd=%d, got %d", len(result.Output), mapping.GoEnd)
	}
}

func TestTernaryCodeGen_OutputCompiles(t *testing.T) {
	// Verify generated code is valid Go
	tests := []struct {
		name string
		expr *ast.TernaryExpr
	}{
		{
			name: "string literal",
			expr: &ast.TernaryExpr{
				CondStr:    "true",
				TrueStr:    `"yes"`,
				FalseStr:   `"no"`,
				ResultType: "string",
			},
		},
		{
			name: "integer literal",
			expr: &ast.TernaryExpr{
				CondStr:    "x > 0",
				TrueStr:    "x",
				FalseStr:   "0",
				ResultType: "int",
			},
		},
		{
			name: "boolean literal",
			expr: &ast.TernaryExpr{
				CondStr:    "flag",
				TrueStr:    "true",
				FalseStr:   "false",
				ResultType: "bool",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gen := NewTernaryCodeGen(tt.expr)
			result := gen.Generate()

			// Try to parse as Go expression
			_, err := parser.ParseExpr(string(result.Output))
			if err != nil {
				t.Errorf("Generated code does not parse as valid Go expression: %v\nGenerated:\n%s", err, string(result.Output))
			}
		})
	}
}

// TestTernaryCodeGen_ASTExpressions tests ternary with AST-based expressions
func TestTernaryCodeGen_ASTExpressions(t *testing.T) {
	tests := []struct {
		name     string
		condSrc  string
		trueSrc  string
		falseSrc string
		wantCond string
		wantTrue string
		wantFalse string
	}{
		{
			name:      "identifiers",
			condSrc:   "isValid",
			trueSrc:   "x",
			falseSrc:  "y",
			wantCond:  "isValid",
			wantTrue:  "x",
			wantFalse: "y",
		},
		{
			name:      "binary comparison",
			condSrc:   "x > 0",
			trueSrc:   "positive",
			falseSrc:  "negative",
			wantCond:  "x > 0",
			wantTrue:  "positive",
			wantFalse: "negative",
		},
		{
			name:      "function calls",
			condSrc:   "hasPermission()",
			trueSrc:   "allow()",
			falseSrc:  "deny()",
			wantCond:  "hasPermission()",
			wantTrue:  "allow()",
			wantFalse: "deny()",
		},
		{
			name:      "selector expressions",
			condSrc:   "user.Active",
			trueSrc:   "user.Name",
			falseSrc:  `"inactive"`,
			wantCond:  "user.Active",
			wantTrue:  "user.Name",
			wantFalse: `"inactive"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse AST expressions
			condExpr, _ := parser.ParseExpr(tt.condSrc)
			trueExpr, _ := parser.ParseExpr(tt.trueSrc)
			falseExpr, _ := parser.ParseExpr(tt.falseSrc)

			expr := &ast.TernaryExpr{
				Cond:  condExpr,
				True:  trueExpr,
				False: falseExpr,
			}

			gen := NewTernaryCodeGen(expr)
			result := gen.Generate()

			got := string(result.Output)

			// Verify AST expressions are rendered correctly
			if !strings.Contains(got, "if "+tt.wantCond+" {") {
				t.Errorf("condition not rendered correctly, want 'if %s {', got:\n%s", tt.wantCond, got)
			}
			if !strings.Contains(got, "return "+tt.wantTrue) {
				t.Errorf("true branch not rendered correctly, want 'return %s', got:\n%s", tt.wantTrue, got)
			}
			if !strings.Contains(got, "return "+tt.wantFalse) {
				t.Errorf("false branch not rendered correctly, want 'return %s', got:\n%s", tt.wantFalse, got)
			}
		})
	}
}

// TestTernaryCodeGen_NestedTernary tests nested ternary expressions
func TestTernaryCodeGen_NestedTernary(t *testing.T) {
	// Inner ternary: y < 0 ? "negative" : "zero"
	innerTernary := &ast.TernaryExpr{
		CondStr:  "y < 0",
		TrueStr:  `"negative"`,
		FalseStr: `"zero"`,
	}

	// Parse inner ternary as an expression to use as false branch of outer
	innerGen := NewTernaryCodeGen(innerTernary)
	innerResult := innerGen.Generate()
	innerCode := string(innerResult.Output)

	// Outer ternary: x > 0 ? "positive" : <inner>
	// For this test, we'll use string representation
	outerTernary := &ast.TernaryExpr{
		CondStr:  "x > 0",
		TrueStr:  `"positive"`,
		FalseStr: innerCode,
	}

	gen := NewTernaryCodeGen(outerTernary)
	result := gen.Generate()

	got := string(result.Output)

	// Verify nested structure
	if !strings.Contains(got, "if x > 0 {") {
		t.Error("Missing outer condition")
	}
	if !strings.Contains(got, `return "positive"`) {
		t.Error("Missing outer true branch")
	}
	// Inner IIFE should be in false branch
	if !strings.Contains(got, "func()") {
		t.Error("Missing inner IIFE in false branch")
	}
}

// TestTernaryCodeGen_EdgeCases tests edge cases
func TestTernaryCodeGen_EdgeCases(t *testing.T) {
	tests := []struct {
		name string
		expr *ast.TernaryExpr
		want string
	}{
		{
			name: "empty result type",
			expr: &ast.TernaryExpr{
				CondStr:  "true",
				TrueStr:  "1",
				FalseStr: "2",
				// No ResultType
			},
			want: "func() {", // No explicit return type
		},
		{
			name: "explicit result type",
			expr: &ast.TernaryExpr{
				CondStr:    "true",
				TrueStr:    "1",
				FalseStr:   "2",
				ResultType: "int",
			},
			want: "func() int {", // Explicit int return
		},
		{
			name: "complex result type",
			expr: &ast.TernaryExpr{
				CondStr:    "valid",
				TrueStr:    "result",
				FalseStr:   "nil",
				ResultType: "*MyStruct",
			},
			want: "func() *MyStruct {", // Pointer type
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gen := NewTernaryCodeGen(tt.expr)
			result := gen.Generate()

			got := string(result.Output)
			if !strings.Contains(got, tt.want) {
				t.Errorf("Expected output to contain %q, got:\n%s", tt.want, got)
			}
		})
	}
}
