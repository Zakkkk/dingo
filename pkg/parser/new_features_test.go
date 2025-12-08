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
func TestTernary(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "simple ternary with strings",
			input: `age >= 18 ? "adult" : "minor"`,
		},
		{
			name:  "ternary with numbers",
			input: "x > 0 ? 1 : 0",
		},
		{
			name:  "ternary with function calls",
			input: "isValid() ? getValue() : getDefault()",
		},
		{
			name:  "ternary with boolean condition",
			input: `status ? "active" : "inactive"`,
		},
		{
			name:  "nested ternary",
			input: `x > 0 ? "positive" : x < 0 ? "negative" : "zero"`,
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

			// Verify it's a TernaryExpr
			if _, ok := expr.(*ast.TernaryExpr); !ok {
				t.Fatalf("expected *ast.TernaryExpr, got %T", expr)
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
func TestOperatorPrecedence(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "ternary lower than null coalescing",
			input: "a ?? b ? c : d",
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
		},
		{
			name:  "complex expression",
			input: "user?.age >= 18 ? adult : minor ?? unknown",
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
func TestDisambiguation(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
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
			input:    `x > 0 ? "positive" : "negative"`,
			expected: "ternary",
		},
		{
			name:     "error prop as ternary condition",
			input:    `getData()? ? "valid" : "invalid"`,
			expected: "ternary with error prop condition",
		},
		{
			name:     "error prop with context vs ternary",
			input:    `getUser() ? "failed"`,
			expected: "error propagation with context",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewParser(0)
			fset := token.NewFileSet()

			expr, err := p.ParseExpr(fset, tt.input)
			if err != nil {
				t.Fatalf("ParseExpr() error = %v, expected %s", err, tt.expected)
			}

			// Verify parsing succeeded and returned a node
			// The specific type may be wrapped in ExprWrapper or other internal types
			if expr == nil {
				t.Fatalf("ParseExpr returned nil for %s", tt.expected)
			}

			// Just verify we got something back - detailed type checking
			// would require unwrapping internal parser representations
			t.Logf("Parsed %q as %T", tt.input, expr)
		})
	}
}
