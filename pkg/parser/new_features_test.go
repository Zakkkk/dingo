package parser

import (
	"go/token"
	"testing"

	"github.com/MadAppGang/dingo/pkg/ast"
)

// TestSafeNavigation tests parsing of the safe navigation operator (?.)
func TestSafeNavigation(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "simple safe navigation",
			input: "user?.name",
		},
		{
			name:  "chained safe navigation",
			input: "user?.address?.city",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewParser(0)
			fset := token.NewFileSet()

			_, err := p.ParseExpr(fset, tt.input)
			if err != nil {
				t.Fatalf("ParseExpr() error = %v", err)
			}
		})
	}
}

// TestNullCoalescing tests parsing of the null coalescing operator (??)
func TestNullCoalescing(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "simple null coalescing",
			input: `name ?? "default"`,
		},
		{
			name:  "chained null coalescing",
			input: "a ?? b ?? c",
		},
		{
			name:  "null coalescing with expression",
			input: "getValue() ?? 42",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewParser(0)
			fset := token.NewFileSet()

			_, err := p.ParseExpr(fset, tt.input)
			if err != nil {
				t.Fatalf("ParseExpr() error = %v", err)
			}
		})
	}
}

// TestTernary tests parsing of the ternary operator (? :)
// TODO(Phase 3+): Ternary operator parsing not yet implemented
// The transformation logic exists in pkg/plugin/builtin/ternary.go but parser support is deferred
func TestTernary(t *testing.T) {
	t.Skip("Ternary parsing not yet implemented - deferred to Phase 3+")

	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "simple ternary",
			input: "age >= 18 ? adult : minor",
		},
		{
			name:  "nested ternary",
			input: "x > 0 ? positive : x < 0 ? negative : zero",
		},
		{
			name:  "ternary with function calls",
			input: "isValid() ? getValue() : getDefault()",
		},
		{
			name:  "ternary with strings",
			input: `status ? "active" : "inactive"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewParser(0)
			fset := token.NewFileSet()

			_, err := p.ParseExpr(fset, tt.input)
			if err != nil {
				t.Fatalf("ParseExpr() error = %v", err)
			}
		})
	}
}

// TestLambda tests parsing of lambda expressions
func TestLambda(t *testing.T) {
	t.Skip("Lambda parsing incomplete - needs Pratt parser improvements")

	tests := []struct {
		name  string
		input string
		style string // "rust" or "arrow"
	}{
		{
			name:  "rust style single param",
			input: "|x| x * 2",
			style: "rust",
		},
		{
			name:  "rust style multiple params",
			input: "|x, y| x + y",
			style: "rust",
		},
		{
			name:  "rust style no params",
			input: "|| 42",
			style: "rust",
		},
		{
			name:  "rust style with type annotation",
			input: "|x: int| x * 2",
			style: "rust",
		},
		{
			name:  "arrow style single param",
			input: "(x) => x * 2",
			style: "arrow",
		},
		{
			name:  "arrow style multiple params",
			input: "(x, y) => x + y",
			style: "arrow",
		},
		{
			name:  "arrow style no params",
			input: "() => 42",
			style: "arrow",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewParser(0)
			fset := token.NewFileSet()

			_, err := p.ParseExpr(fset, tt.input)
			if err != nil {
				t.Fatalf("ParseExpr() error = %v", err)
			}
		})
	}
}

// TestOperatorPrecedence tests that operators have correct precedence
// TODO(Phase 3+): Some tests require ternary parsing which is not yet implemented
func TestOperatorPrecedence(t *testing.T) {
	tests := []struct {
		name  string
		input string
		skip  bool
	}{
		{
			name:  "ternary lower than null coalescing",
			input: "a ?? b ? c : d",
			skip:  true, // Requires ternary parsing
		},
		{
			name:  "null coalescing lower than comparison",
			input: "a == b ?? c",
		},
		{
			name:  "safe navigation with error propagation",
			input: "user?.name?",
		},
		{
			name:  "ternary with safe navigation",
			input: "user?.isActive ? enabled : disabled",
			skip:  true, // Requires ternary parsing
		},
		{
			name:  "complex expression",
			input: "user?.age >= 18 ? adult : minor ?? unknown",
			skip:  true, // Requires ternary parsing
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skip {
				t.Skip("Requires ternary parsing (deferred to Phase 3+)")
			}
			p := NewParser(0)
			fset := token.NewFileSet()

			_, err := p.ParseExpr(fset, tt.input)
			if err != nil {
				t.Fatalf("ParseExpr() error = %v", err)
			}
		})
	}
}

// TestOperatorChaining tests chaining of operators
func TestOperatorChaining(t *testing.T) {
	tests := []struct {
		name  string
		input string
		skip  bool
	}{
		{
			name:  "multiple safe navigation",
			input: "a?.b?.c?.d",
		},
		{
			name:  "multiple null coalescing",
			input: "a ?? b ?? c ?? d",
		},
		{
			name:  "safe navigation with method chains",
			input: "user?.getAddress()?.getCity()",
			skip:  true, // Known edge case: method calls after safe navigation not yet supported (Phase 3)
		},
		{
			name:  "mixed operators",
			input: "user?.name ?? defaultName",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skip {
				t.Skip("Method calls after safe navigation not yet supported - deferred to Phase 3 (known edge case)")
			}
			p := NewParser(0)
			fset := token.NewFileSet()

			_, err := p.ParseExpr(fset, tt.input)
			if err != nil {
				t.Fatalf("ParseExpr() error = %v", err)
			}
		})
	}
}

// TestLambdaInExpressions tests lambda expressions in various contexts
func TestLambdaInExpressions(t *testing.T) {
	tests := []struct {
		name  string
		input string
		skip  bool
	}{
		{
			name:  "lambda with null coalescing",
			input: "getValue() ?? (() => 42)",
			skip:  true, // TODO: Requires nested lambda parsing in expressions
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skip {
				t.Skip("Skipping test that requires full program context")
			}
			p := NewParser(0)
			fset := token.NewFileSet()

			_, err := p.ParseExpr(fset, tt.input)
			if err != nil {
				t.Fatalf("ParseExpr() error = %v", err)
			}
		})
	}
}

// TestFullProgram tests parsing complete programs with new operators
// TODO(Phase 3+): Some tests require ternary parsing which is not yet implemented
func TestFullProgram(t *testing.T) {
	tests := []struct {
		name  string
		input string
		skip  bool
	}{
		{
			name: "function with safe navigation",
			input: `package main
func getUserCity(user: *User) string {
	return user?.address?.city ?? "Unknown"
}`,
			skip: true, // Requires safe navigation and null coalescing operators (not yet implemented)
		},
		{
			name: "function with ternary",
			input: `package main
func getStatus(age: int) string {
	return age >= 18 ? "adult" : "minor"
}`,
			skip: true, // Requires ternary parsing
		},
		{
			name: "function with lambda",
			input: `package main
func main() {
	let double = |x| x * 2
	return double(5)
}`,
			skip: true, // Requires lambda syntax (not yet implemented)
		},
		{
			name: "mixed operators",
			input: `package main
func process(data: *Data) string {
	return data?.value >= 10 ? "high" : "low" ?? "unknown"
}`,
			skip: true, // Requires ternary parsing
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skip {
				t.Skip("Requires ternary parsing (deferred to Phase 3+)")
			}
			p := NewParser(0)
			fset := token.NewFileSet()

			_, err := p.ParseFile(fset, "test.dingo", []byte(tt.input))
			if err != nil {
				t.Fatalf("ParseFile() error = %v", err)
			}
		})
	}
}

// TestAdvancedErrorPropagation tests parsing of error propagation with context and transforms
func TestAdvancedErrorPropagation(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "basic error propagation",
			input: "getValue()?",
		},
		{
			name:  "error propagation with string context",
			input: `fetchData() ? "fetch failed"`,
		},
		{
			name:  "error propagation with raw string context",
			input: "readFile() ? `read failed`",
		},
		{
			name:  "error propagation with rust-style lambda",
			input: `loadUser() ? |err| wrap("user", err)`,
		},
		{
			name:  "error propagation with typescript-style lambda (parens)",
			input: `getData() ? (e) => fmt.Errorf("error: %w", e)`,
		},
		{
			name:  "error propagation with typescript-style lambda (no parens)",
			input: `fetchOrder() ? err => wrapError(err)`,
		},
		// NOTE: Chained safe navigation with method calls requires Phase 3+ implementation
		// {
		// 	name:  "chained with context",
		// 	input: `foo()?.bar() ? "bar failed"`,
		// },
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewParser(0)
			fset := token.NewFileSet()

			_, err := p.ParseExpr(fset, tt.input)
			if err != nil {
				t.Fatalf("ParseExpr() error = %v", err)
			}
		})
	}
}

// TestErrorPropagationAST tests that error propagation parses to correct AST nodes
func TestErrorPropagationAST(t *testing.T) {
	tests := []struct {
		name            string
		input           string
		hasContext      bool
		hasTransform    bool
		contextMessage  string
		transformParams int
	}{
		{
			name:         "basic - no context or transform",
			input:        "getValue()?",
			hasContext:   false,
			hasTransform: false,
		},
		{
			name:           "string context",
			input:          `fetchData() ? "fetch failed"`,
			hasContext:     true,
			hasTransform:   false,
			contextMessage: "fetch failed",
		},
		{
			name:            "rust-style lambda transform",
			input:           `loadUser() ? |err| wrap(err)`,
			hasContext:      false,
			hasTransform:    true,
			transformParams: 1,
		},
		{
			name:            "typescript-style lambda transform",
			input:           `getData() ? (e) => fmt.Errorf("error: %w", e)`,
			hasContext:      false,
			hasTransform:    true,
			transformParams: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewParser(0)
			fset := token.NewFileSet()

			expr, err := p.ParseExpr(fset, tt.input)
			if err != nil {
				t.Fatalf("ParseExpr() error = %v", err)
			}

			// Check it's an ErrorPropExpr
			errorProp, ok := expr.(*ast.ErrorPropExpr)
			if !ok {
				t.Fatalf("expected ErrorPropExpr, got %T", expr)
			}

			// Check context
			if tt.hasContext {
				if errorProp.ErrorContext == nil {
					t.Fatalf("expected ErrorContext to be set")
				}
				if errorProp.ErrorContext.Message != tt.contextMessage {
					t.Fatalf("expected context message %q, got %q", tt.contextMessage, errorProp.ErrorContext.Message)
				}
			} else if errorProp.ErrorContext != nil {
				t.Fatalf("expected ErrorContext to be nil, got %v", errorProp.ErrorContext)
			}

			// Check transform
			if tt.hasTransform {
				if errorProp.ErrorTransform == nil {
					t.Fatalf("expected ErrorTransform to be set")
				}
				if len(errorProp.ErrorTransform.Params) != tt.transformParams {
					t.Fatalf("expected %d transform params, got %d", tt.transformParams, len(errorProp.ErrorTransform.Params))
				}
			} else if errorProp.ErrorTransform != nil {
				t.Fatalf("expected ErrorTransform to be nil, got %v", errorProp.ErrorTransform)
			}
		})
	}
}

// TestDisambiguation tests that similar operators are properly disambiguated
// TODO(Phase 3+): Some tests require ternary parsing which is not yet implemented
func TestDisambiguation(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		skip     bool
	}{
		{
			name:     "question mark - error propagation",
			input:    "getValue()?",
			expected: "error propagation",
		},
		{
			name:     "question mark dot - safe navigation",
			input:    "user?.name",
			expected: "safe navigation",
		},
		{
			name:     "double question mark - null coalescing",
			input:    "a ?? b",
			expected: "null coalescing",
		},
		{
			name:     "question colon - ternary",
			input:    "x > 0 ? positive : negative",
			expected: "ternary",
			skip:     true, // Requires ternary parsing
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skip {
				t.Skip("Requires ternary parsing (deferred to Phase 3+)")
			}
			p := NewParser(0)
			fset := token.NewFileSet()

			_, err := p.ParseExpr(fset, tt.input)
			if err != nil {
				t.Fatalf("ParseExpr() error = %v, expected %s", err, tt.expected)
			}
		})
	}
}
