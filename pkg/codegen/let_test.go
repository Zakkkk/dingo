package codegen

import (
	"go/token"
	"testing"

	"github.com/MadAppGang/dingo/pkg/ast"
)

func TestLetCodeGen_SimpleDeclaration(t *testing.T) {
	tests := []struct {
		name     string
		decl     *ast.LetDecl
		expected string
	}{
		{
			name: "short declaration without type",
			decl: &ast.LetDecl{
				LetPos:    token.Pos(1),
				Names:     []string{"x"},
				TypeAnnot: "",
				Value:     "42",
				HasInit:   true,
			},
			expected: "x := 42",
		},
		{
			name: "var declaration with type annotation",
			decl: &ast.LetDecl{
				LetPos:    token.Pos(1),
				Names:     []string{"y"},
				TypeAnnot: ": int",
				Value:     "100",
				HasInit:   true,
			},
			expected: "var y int = 100",
		},
		{
			name: "var declaration with type, no init",
			decl: &ast.LetDecl{
				LetPos:    token.Pos(1),
				Names:     []string{"z"},
				TypeAnnot: ": string",
				Value:     "",
				HasInit:   false,
			},
			expected: "var z string",
		},
		{
			name: "multiple names without type",
			decl: &ast.LetDecl{
				LetPos:    token.Pos(1),
				Names:     []string{"a", "b"},
				TypeAnnot: "",
				Value:     "getValues()",
				HasInit:   true,
			},
			expected: "a, b := getValues()",
		},
		{
			name: "multiple names with type",
			decl: &ast.LetDecl{
				LetPos:    token.Pos(1),
				Names:     []string{"x", "y"},
				TypeAnnot: ": int",
				Value:     "getPair()",
				HasInit:   true,
			},
			expected: "var x, y int = getPair()",
		},
		{
			name: "declaration without init or type (edge case)",
			decl: &ast.LetDecl{
				LetPos:    token.Pos(1),
				Names:     []string{"w"},
				TypeAnnot: "",
				Value:     "",
				HasInit:   false,
			},
			expected: "var w",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gen := &LetCodeGen{
				BaseGenerator: NewBaseGenerator(),
				decl:          tt.decl,
			}

			result := gen.Generate()

			got := string(result.Output)
			if got != tt.expected {
				t.Errorf("Generate() output mismatch:\ngot:  %q\nwant: %q", got, tt.expected)
			}

			// Verify mapping was created
			if len(result.Mappings) != 1 {
				t.Errorf("Expected 1 mapping, got %d", len(result.Mappings))
			}

			if len(result.Mappings) > 0 {
				mapping := result.Mappings[0]
				if mapping.Kind != "let_decl" {
					t.Errorf("Expected mapping kind 'let_decl', got %q", mapping.Kind)
				}
			}
		})
	}
}

func TestLetCodeGen_ComplexExpressions(t *testing.T) {
	tests := []struct {
		name     string
		decl     *ast.LetDecl
		expected string
	}{
		{
			name: "function call expression",
			decl: &ast.LetDecl{
				LetPos:    token.Pos(1),
				Names:     []string{"result"},
				TypeAnnot: "",
				Value:     "calculate(x, y, z)",
				HasInit:   true,
			},
			expected: "result := calculate(x, y, z)",
		},
		{
			name: "struct literal",
			decl: &ast.LetDecl{
				LetPos:    token.Pos(1),
				Names:     []string{"user"},
				TypeAnnot: ": User",
				Value:     "User{Name: \"Alice\"}",
				HasInit:   true,
			},
			expected: "var user User = User{Name: \"Alice\"}",
		},
		{
			name: "generic type annotation",
			decl: &ast.LetDecl{
				LetPos:    token.Pos(1),
				Names:     []string{"data"},
				TypeAnnot: ": Option<string>",
				Value:     "Some(\"hello\")",
				HasInit:   true,
			},
			expected: "var data Option<string> = Some(\"hello\")",
		},
		{
			name: "error handling expression",
			decl: &ast.LetDecl{
				LetPos:    token.Pos(1),
				Names:     []string{"file"},
				TypeAnnot: "",
				Value:     "os.Open(\"test.txt\")",
				HasInit:   true,
			},
			expected: "file := os.Open(\"test.txt\")",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gen := &LetCodeGen{
				BaseGenerator: NewBaseGenerator(),
				decl:          tt.decl,
			}

			result := gen.Generate()

			got := string(result.Output)
			if got != tt.expected {
				t.Errorf("Generate() output mismatch:\ngot:  %q\nwant: %q", got, tt.expected)
			}
		})
	}
}

func TestLetCodeGen_NilDeclaration(t *testing.T) {
	gen := &LetCodeGen{
		BaseGenerator: NewBaseGenerator(),
		decl:          nil,
	}

	result := gen.Generate()

	if len(result.Output) != 0 {
		t.Errorf("Expected empty output for nil declaration, got %q", string(result.Output))
	}

	if len(result.Mappings) != 0 {
		t.Errorf("Expected no mappings for nil declaration, got %d", len(result.Mappings))
	}
}

func TestLetCodeGen_SourceMapping(t *testing.T) {
	decl := &ast.LetDecl{
		LetPos:    token.Pos(10),
		Names:     []string{"x"},
		TypeAnnot: ": int",
		Value:     "42",
		HasInit:   true,
	}

	gen := &LetCodeGen{
		BaseGenerator: NewBaseGenerator(),
		decl:          decl,
	}

	result := gen.Generate()

	if len(result.Mappings) != 1 {
		t.Fatalf("Expected 1 mapping, got %d", len(result.Mappings))
	}

	mapping := result.Mappings[0]

	// Verify mapping positions
	if mapping.DingoStart != int(decl.Pos()) {
		t.Errorf("Expected DingoStart=%d, got %d", int(decl.Pos()), mapping.DingoStart)
	}

	if mapping.DingoEnd != int(decl.End()) {
		t.Errorf("Expected DingoEnd=%d, got %d", int(decl.End()), mapping.DingoEnd)
	}

	if mapping.GoStart != 0 {
		t.Errorf("Expected GoStart=0, got %d", mapping.GoStart)
	}

	expectedOutput := "var x int = 42"
	if mapping.GoEnd != len(expectedOutput) {
		t.Errorf("Expected GoEnd=%d, got %d", len(expectedOutput), mapping.GoEnd)
	}

	if mapping.Kind != "let_decl" {
		t.Errorf("Expected mapping kind 'let_decl', got %q", mapping.Kind)
	}
}

func TestLetCodeGen_TypeAnnotationWithoutColon(t *testing.T) {
	// Test that TypeAnnot without leading colon is handled correctly
	decl := &ast.LetDecl{
		LetPos:    token.Pos(1),
		Names:     []string{"x"},
		TypeAnnot: "int", // No leading colon
		Value:     "42",
		HasInit:   true,
	}

	gen := &LetCodeGen{
		BaseGenerator: NewBaseGenerator(),
		decl:          decl,
	}

	result := gen.Generate()

	expected := "var xint = 42" // Should append type directly (ToGo removes colon if present)
	got := string(result.Output)
	if got != expected {
		t.Errorf("Generate() output mismatch:\ngot:  %q\nwant: %q", got, expected)
	}
}
