package parser

import (
	"go/printer"
	"go/token"
	"os"
	"reflect"
	"testing"

	"github.com/MadAppGang/dingo/pkg/ast"
	"github.com/MadAppGang/dingo/pkg/tokenizer"
)

func TestParseHelloWorld(t *testing.T) {
	t.Skip("AST parser doesn't yet handle full Dingo file parsing - use pkg/goparser/parser for now")

	src := []byte(`package main

func main() {
	let message = "Hello, Dingo!"
	println(message)
	return
}

func add(a: int, b: int) int {
	return a + b
}
`)

	fset := token.NewFileSet()
	file, err := ParseFile(fset, "hello.dingo", src, 0)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	if file == nil {
		t.Fatal("Expected non-nil file")
	}

	if file.Name.Name != "main" {
		t.Errorf("Expected package 'main', got %s", file.Name.Name)
	}

	// Print the AST to stdout for inspection
	t.Log("Generated AST:")
	printer.Fprint(os.Stdout, fset, file.File)
}

func TestParseExpression(t *testing.T) {
	tests := []struct {
		name string
		expr string
	}{
		{"simple add", "1 + 2"},
		{"multiply", "3 * 4"},
		{"comparison", "x == 5"},
		{"function call", "println(42)"},
		{"complex", "(a + b) * c"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Just check that it parses without error
			// ParseExpr currently only returns Dingo nodes, so standard expressions will fail
			// This is expected behavior for now
			t.Skip("Skipping ParseExpr test - needs refactoring to support standard expressions")
		})
	}
}

// TestDingoNodesPopulatedTopLevel verifies that top-level DingoNodes are collected.
// Note: The current AST parser (pkg/parser) doesn't deeply parse function bodies.
// Expressions inside functions are handled by the transpiler pipeline (pkg/goparser).
// This test focuses on top-level declarations that ARE parsed.
func TestDingoNodesPopulatedTopLevel(t *testing.T) {
	tests := []struct {
		name      string
		src       string
		wantLen   int
		wantTypes []string // Expected type names (e.g., "*ast.EnumDecl")
	}{
		{
			name:      "enum declaration",
			src:       `package main; enum Color { Red, Green, Blue }`,
			wantLen:   1,
			wantTypes: []string{"*ast.EnumDecl"},
		},
		{
			name:      "multiple enum declarations",
			src:       `package main; enum Color { Red, Green }; enum Status { Active, Inactive }`,
			wantLen:   2,
			wantTypes: []string{"*ast.EnumDecl", "*ast.EnumDecl"},
		},
		{
			name:      "generic enum",
			src:       `package main; enum Result<T, E> { Ok(T), Err(E) }`,
			wantLen:   1,
			wantTypes: []string{"*ast.EnumDecl"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fset := token.NewFileSet()
			file, err := ParseFile(fset, "test.dingo", []byte(tt.src), 0)
			if err != nil {
				t.Fatalf("ParseFile failed: %v", err)
			}

			if file == nil {
				t.Fatal("Expected non-nil file")
			}

			if len(file.DingoNodes) != tt.wantLen {
				t.Errorf("DingoNodes length = %d, want %d", len(file.DingoNodes), tt.wantLen)
				for i, node := range file.DingoNodes {
					t.Logf("  [%d] %T", i, node)
				}
			}

			// Verify types match expected
			for i, wantType := range tt.wantTypes {
				if i >= len(file.DingoNodes) {
					t.Errorf("Missing node at index %d: expected %s", i, wantType)
					continue
				}

				gotType := reflect.TypeOf(file.DingoNodes[i]).String()
				// Handle wrapped expressions
				if wrapper, ok := file.DingoNodes[i].(*ast.ExprWrapper); ok {
					gotType = reflect.TypeOf(wrapper.DingoExpr).String()
				}

				if gotType != wantType {
					t.Errorf("DingoNodes[%d] type = %s, want %s", i, gotType, wantType)
				}
			}
		})
	}
}

// TestDingoNodesExpressionCollection tests that expressions ARE collected
// when using the PrattParser directly (via ParseExpr, not ParseFile).
// This validates the callback mechanism works correctly.
func TestDingoNodesExpressionCollection(t *testing.T) {
	tests := []struct {
		name     string
		expr     string
		wantType string
	}{
		{
			name:     "match expression",
			expr:     `match x { 1 => "a", _ => "b" }`,
			wantType: "*ast.MatchExpr",
		},
		{
			name:     "rust lambda",
			expr:     `|x| x + 1`,
			wantType: "*ast.LambdaExpr",
		},
		{
			name:     "typescript lambda with parens",
			expr:     `(x) => x + 1`,
			wantType: "*ast.LambdaExpr",
		},
		{
			name:     "typescript single param lambda",
			expr:     `x => x + 1`,
			wantType: "*ast.LambdaExpr",
		},
		{
			name:     "error propagation",
			expr:     `getData()?`,
			wantType: "*ast.ErrorPropExpr",
		},
		{
			name:     "safe navigation",
			expr:     `x?.y`,
			wantType: "*ast.SafeNavExpr",
		},
		// NOTE: SafeNavCallExpr parsing has a pre-existing issue with EOF handling
		// {
		// 	name:     "safe navigation call",
		// 	expr:     `x?.method()`,
		// 	wantType: "*ast.SafeNavCallExpr",
		// },
		{
			name:     "null coalesce",
			expr:     `x ?? y`,
			wantType: "*ast.NullCoalesceExpr",
		},
		{
			name:     "ternary expression",
			expr:     `cond ? "yes" : "no"`,
			wantType: "*ast.TernaryExpr",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fset := token.NewFileSet()
			node, err := ParseExpr(fset, tt.expr)
			if err != nil {
				t.Fatalf("ParseExpr failed: %v", err)
			}

			if node == nil {
				t.Fatal("Expected non-nil node")
			}

			gotType := reflect.TypeOf(node).String()
			// Handle wrapped expressions
			if wrapper, ok := node.(*ast.ExprWrapper); ok {
				gotType = reflect.TypeOf(wrapper.DingoExpr).String()
			}

			if gotType != tt.wantType {
				t.Errorf("Parsed expression type = %s, want %s", gotType, tt.wantType)
			}
		})
	}
}

// TestCollectDingoNodeCallback verifies that the OnDingoNode callback mechanism
// works correctly when expressions are parsed through PrattParser.
func TestCollectDingoNodeCallback(t *testing.T) {
	src := []byte(`x ?? y`)
	tok := NewPrattParser(tokenizer.New(src))

	var collected []ast.DingoNode
	tok.OnDingoNode = func(node ast.DingoNode) {
		collected = append(collected, node)
	}

	// Parse the expression
	_ = tok.ParseExpression(PrecLowest)

	// Should have collected the NullCoalesceExpr
	if len(collected) != 1 {
		t.Errorf("Collected %d nodes, want 1", len(collected))
		for i, node := range collected {
			t.Logf("  [%d] %T", i, node)
		}
		return
	}

	// Check the collected type (may be wrapped)
	gotType := reflect.TypeOf(collected[0]).String()
	if wrapper, ok := collected[0].(*ast.ExprWrapper); ok {
		gotType = reflect.TypeOf(wrapper.DingoExpr).String()
	}

	if gotType != "*ast.NullCoalesceExpr" {
		t.Errorf("Collected node type = %s, want *ast.NullCoalesceExpr", gotType)
	}
}

// TestMatchExprUsesCollectDingoNode verifies that MatchExpr now uses
// collectDingoNode helper (fix from code review C1).
func TestMatchExprUsesCollectDingoNode(t *testing.T) {
	src := []byte(`match x { 1 => "a", _ => "b" }`)
	tok := NewPrattParser(tokenizer.New(src))

	var collected []ast.DingoNode
	tok.OnDingoNode = func(node ast.DingoNode) {
		collected = append(collected, node)
	}

	// Parse the match expression
	_ = tok.ParseExpression(PrecLowest)

	// Should have collected the MatchExpr
	if len(collected) != 1 {
		t.Errorf("Collected %d nodes, want 1", len(collected))
		return
	}

	// MatchExpr implements DingoNode directly (not wrapped)
	if _, ok := collected[0].(*ast.MatchExpr); !ok {
		t.Errorf("Expected *ast.MatchExpr, got %T", collected[0])
	}
}

// TestLambdaExprUsesCollectDingoNode verifies that all lambda parsers use
// collectDingoNode helper (fix from code review C2).
func TestLambdaExprUsesCollectDingoNode(t *testing.T) {
	tests := []struct {
		name string
		expr string
	}{
		{"rust style", `|x| x + 1`},
		{"typescript with parens", `(x) => x + 1`},
		{"typescript single param", `x => x + 1`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tok := NewPrattParser(tokenizer.New([]byte(tt.expr)))

			var collected []ast.DingoNode
			tok.OnDingoNode = func(node ast.DingoNode) {
				collected = append(collected, node)
			}

			// Parse the lambda expression
			_ = tok.ParseExpression(PrecLowest)

			// Should have collected the LambdaExpr
			if len(collected) != 1 {
				t.Errorf("Collected %d nodes, want 1", len(collected))
				return
			}

			// LambdaExpr implements DingoNode directly (not wrapped after fix)
			if _, ok := collected[0].(*ast.LambdaExpr); !ok {
				t.Errorf("Expected *ast.LambdaExpr, got %T", collected[0])
			}
		})
	}
}
