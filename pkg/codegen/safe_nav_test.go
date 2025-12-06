package codegen

import (
	"go/token"
	"testing"

	dingoast "github.com/MadAppGang/dingo/pkg/ast"
)

func TestSafeNavCodeGen_FieldAccess(t *testing.T) {
	tests := []struct {
		name     string
		expr     *dingoast.SafeNavExpr
		expected string
	}{
		{
			name: "simple field access",
			expr: &dingoast.SafeNavExpr{
				X:     &dingoast.DingoIdent{Name: "user"},
				OpPos: token.Pos(5),
				Sel:   &dingoast.DingoIdent{Name: "name"},
			},
			// New flattened format with tmp variables
			expected: "func() interface{} { tmp := user; if tmp == nil { return nil }; return tmp.name }()",
		},
		{
			name: "nested field access",
			expr: &dingoast.SafeNavExpr{
				X: &dingoast.RawExpr{
					Text: "req.user",
				},
				OpPos: token.Pos(10),
				Sel:   &dingoast.DingoIdent{Name: "name"},
			},
			// New flattened format
			expected: "func() interface{} { tmp := req.user; if tmp == nil { return nil }; return tmp.name }()",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gen := &SafeNavCodeGen{
				BaseGenerator: NewBaseGenerator(),
				expr:          tt.expr,
			}

			result := gen.Generate()
			got := string(result.Output)

			if got != tt.expected {
				t.Errorf("SafeNavCodeGen.Generate() = %q, want %q", got, tt.expected)
			}

			// Verify source mappings
			if len(result.Mappings) != 1 {
				t.Errorf("expected 1 mapping, got %d", len(result.Mappings))
			}
		})
	}
}

func TestSafeNavCodeGen_MethodCall(t *testing.T) {
	tests := []struct {
		name     string
		expr     *dingoast.SafeNavCallExpr
		expected string
	}{
		{
			name: "method call no args",
			expr: &dingoast.SafeNavCallExpr{
				X:     &dingoast.DingoIdent{Name: "user"},
				OpPos: token.Pos(5),
				Fun:   &dingoast.DingoIdent{Name: "getName"},
				Args:  []dingoast.Expr{},
			},
			// New flattened format
			expected: "func() interface{} { tmp := user; if tmp == nil { return nil }; return tmp.getName() }()",
		},
		{
			name: "method call with args",
			expr: &dingoast.SafeNavCallExpr{
				X:     &dingoast.DingoIdent{Name: "user"},
				OpPos: token.Pos(5),
				Fun:   &dingoast.DingoIdent{Name: "getField"},
				Args: []dingoast.Expr{
					&dingoast.RawExpr{Text: `"name"`},
				},
			},
			// New flattened format
			expected: `func() interface{} { tmp := user; if tmp == nil { return nil }; return tmp.getField("name") }()`,
		},
		{
			name: "method call multiple args",
			expr: &dingoast.SafeNavCallExpr{
				X:     &dingoast.DingoIdent{Name: "calc"},
				OpPos: token.Pos(5),
				Fun:   &dingoast.DingoIdent{Name: "add"},
				Args: []dingoast.Expr{
					&dingoast.RawExpr{Text: "1"},
					&dingoast.RawExpr{Text: "2"},
				},
			},
			// New flattened format
			expected: "func() interface{} { tmp := calc; if tmp == nil { return nil }; return tmp.add(1, 2) }()",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gen := &SafeNavCodeGen{
				BaseGenerator: NewBaseGenerator(),
				callExpr:      tt.expr,
			}

			result := gen.Generate()
			got := string(result.Output)

			if got != tt.expected {
				t.Errorf("SafeNavCodeGen.Generate() = %q, want %q", got, tt.expected)
			}

			// Verify source mappings
			if len(result.Mappings) != 1 {
				t.Errorf("expected 1 mapping, got %d", len(result.Mappings))
			}
		})
	}
}

func TestSafeNavCodeGen_NilInput(t *testing.T) {
	gen := &SafeNavCodeGen{
		BaseGenerator: NewBaseGenerator(),
		expr:          nil,
		callExpr:      nil,
	}

	result := gen.Generate()

	if len(result.Output) != 0 {
		t.Errorf("expected empty output for nil input, got %q", result.Output)
	}
}
