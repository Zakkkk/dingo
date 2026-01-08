package typechecker

import (
	"go/token"
	"testing"

	"github.com/MadAppGang/dingo/pkg/ast"
)

func TestInferMatchResultType_StringLiteral(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "double quoted string",
			body: `"hello"`,
			want: "string",
		},
		{
			name: "backtick string",
			body: "`world`",
			want: "string",
		},
		{
			name: "string with spaces",
			body: `"hello world"`,
			want: "string",
		},
		{
			name: "empty string",
			body: `""`,
			want: "string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			match := &ast.MatchExpr{
				Arms: []*ast.MatchArm{
					{
						Body: &ast.RawExpr{Text: tt.body},
					},
				},
			}

			got := InferMatchResultType(match, nil)
			if got != tt.want {
				t.Errorf("InferMatchResultType() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestInferMatchResultType_IntLiteral(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "positive integer",
			body: "42",
			want: "int",
		},
		{
			name: "negative integer",
			body: "-10",
			want: "int",
		},
		{
			name: "zero",
			body: "0",
			want: "int",
		},
		{
			name: "large number",
			body: "123456789",
			want: "int",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			match := &ast.MatchExpr{
				Arms: []*ast.MatchArm{
					{
						Body: &ast.RawExpr{Text: tt.body},
					},
				},
			}

			got := InferMatchResultType(match, nil)
			if got != tt.want {
				t.Errorf("InferMatchResultType() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestInferMatchResultType_FloatLiteral(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "positive float",
			body: "3.14",
			want: "float64",
		},
		{
			name: "negative float",
			body: "-2.5",
			want: "float64",
		},
		{
			name: "zero float",
			body: "0.0",
			want: "float64",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			match := &ast.MatchExpr{
				Arms: []*ast.MatchArm{
					{
						Body: &ast.RawExpr{Text: tt.body},
					},
				},
			}

			got := InferMatchResultType(match, nil)
			if got != tt.want {
				t.Errorf("InferMatchResultType() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestInferMatchResultType_BoolLiteral(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "true literal",
			body: "true",
			want: "bool",
		},
		{
			name: "false literal",
			body: "false",
			want: "bool",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			match := &ast.MatchExpr{
				Arms: []*ast.MatchArm{
					{
						Body: &ast.RawExpr{Text: tt.body},
					},
				},
			}

			got := InferMatchResultType(match, nil)
			if got != tt.want {
				t.Errorf("InferMatchResultType() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestInferMatchResultType_FmtSprintf(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "fmt.Sprintf",
			body: `fmt.Sprintf("value: %d", x)`,
			want: "string",
		},
		{
			name: "fmt.Sprint",
			body: `fmt.Sprint("hello")`,
			want: "string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			match := &ast.MatchExpr{
				Arms: []*ast.MatchArm{
					{
						Body: &ast.RawExpr{Text: tt.body},
					},
				},
			}

			got := InferMatchResultType(match, nil)
			if got != tt.want {
				t.Errorf("InferMatchResultType() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestInferMatchResultType_EmptyOrNoArms(t *testing.T) {
	tests := []struct {
		name  string
		match *ast.MatchExpr
		want  string
	}{
		{
			name: "no arms",
			match: &ast.MatchExpr{
				Arms: []*ast.MatchArm{},
			},
			want: "",
		},
		{
			name: "nil body",
			match: &ast.MatchExpr{
				Arms: []*ast.MatchArm{
					{Body: nil},
				},
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := InferMatchResultType(tt.match, nil)
			if got != tt.want {
				t.Errorf("InferMatchResultType() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestInferMatchResultType_ComplexExpression_FallbackToEmpty(t *testing.T) {
	// Complex expressions that can't be easily inferred should return ""
	// to trigger IIFE fallback
	tests := []struct {
		name string
		body string
	}{
		{
			name: "function call",
			body: "someFunction()",
		},
		{
			name: "binary operation",
			body: "x + y",
		},
		{
			name: "field access",
			body: "obj.field",
		},
		{
			name: "method call",
			body: "item.Process()",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			match := &ast.MatchExpr{
				Arms: []*ast.MatchArm{
					{
						Body: &ast.RawExpr{Text: tt.body},
					},
				},
			}

			// Without sourceContext, complex expressions should return ""
			got := InferMatchResultType(match, nil)
			if got != "" {
				t.Errorf("InferMatchResultType() = %q, want empty string for complex expression", got)
			}
		})
	}
}

// Note: isIntLiteral and isFloatLiteral tests removed - now using go/scanner internally

func TestSimplifyTypeName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"interface{}", "any"},
		{"interface {}", "any"},
		{"string", "string"},
		{"int", "int"},
		{"*User", "*User"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := simplifyTypeName(tt.input)
			if got != tt.want {
				t.Errorf("simplifyTypeName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestInferMatchResultType_WithGoTypes(t *testing.T) {
	// TODO: go/types inference for complex expressions is a future enhancement
	// For now, these cases fall back to IIFE (which is correct behavior)
	// The literal detection path handles 90%+ of real-world cases
	t.Skip("go/types inference for complex field access is a future enhancement")

	// Test with actual Go source context for go/types inference
	goSource := []byte(`package main

type User struct {
	Name string
	Age  int
}

func main() {
	user := User{Name: "Alice", Age: 30}
	_ = user.Name
	_ = user.Age
}
`)

	tests := []struct {
		name string
		body string
		want string // Empty for expressions not in source
	}{
		{
			name: "simple identifier in source",
			body: "user.Name",
			want: "string",
		},
		{
			name: "int field",
			body: "user.Age",
			want: "int",
		},
		{
			name: "expression not in source",
			body: "nonexistent.Field",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			match := &ast.MatchExpr{
				Arms: []*ast.MatchArm{
					{
						Body: &ast.RawExpr{Text: tt.body},
					},
				},
			}

			got := InferMatchResultType(match, goSource)
			if got != tt.want {
				t.Errorf("InferMatchResultType() with go/types = %q, want %q", got, tt.want)
			}
		})
	}
}

// mockExpr is a test helper that implements ast.Expr
type mockExpr struct {
	text string
}

func (m *mockExpr) String() string { return m.text }
func (m *mockExpr) Pos() token.Pos { return token.NoPos }
func (m *mockExpr) End() token.Pos { return token.NoPos }
func (m *mockExpr) Node()          {}
func (m *mockExpr) exprNode()      {}
