package preprocessor

import (
	"fmt"
	"strings"
	"testing"
)

func TestTypeAnnotASTProcessor_BasicTransformation(t *testing.T) {
	processor := NewTypeAnnotASTProcessor()
	input := `package main

func add(x: int, y: int) int {
	return x + y
}`

	result, metadata, err := processor.ProcessInternal(input)
	if err != nil {
		t.Fatalf("ProcessInternal failed: %v", err)
	}

	// Should have one metadata entry
	if len(metadata) != 1 {
		t.Fatalf("Expected 1 metadata entry, got %d", len(metadata))
	}

	// Result should have transformed : to space
	if !strings.Contains(result, "func add(x int, y int)") {
		t.Errorf("Expected transformed parameters, got: %s", result)
	}
}

func TestTypeAnnotASTProcessor_NestedGenerics(t *testing.T) {
	processor := NewTypeAnnotASTProcessor()
	input := `func process(data: Map<string, List<int>>) error {
	return nil
}`

	result, _, err := processor.ProcessInternal(input)
	if err != nil {
		t.Fatalf("ProcessInternal failed: %v", err)
	}

	// Should transform Map<...> to Map[...] and handle nested List<int>
	expected := "func process(data Map[string, List[int]])"
	if !strings.Contains(result, expected) {
		t.Errorf("Expected '%s', got: %s", expected, result)
	}
}

func TestTypeAnnotASTProcessor_FunctionType(t *testing.T) {
	processor := NewTypeAnnotASTProcessor()
	input := `func register(handler: func(int) error) {
}`

	result, _, err := processor.ProcessInternal(input)
	if err != nil {
		t.Fatalf("ProcessInternal failed: %v", err)
	}

	expected := "func register(handler func(int) error)"
	if !strings.Contains(result, expected) {
		t.Errorf("Expected '%s', got: %s", expected, result)
	}
}

func TestTypeAnnotASTProcessor_ComplexFunctionType(t *testing.T) {
	processor := NewTypeAnnotASTProcessor()
	input := `func apply(f: func(a, b int) (string, error)) {
}`

	result, _, err := processor.ProcessInternal(input)
	if err != nil {
		t.Fatalf("ProcessInternal failed: %v", err)
	}

	expected := "func apply(f func(a, b int) (string, error))"
	if !strings.Contains(result, expected) {
		t.Errorf("Expected '%s', got: %s", expected, result)
	}
}

func TestTypeAnnotASTProcessor_VariadicParams(t *testing.T) {
	processor := NewTypeAnnotASTProcessor()
	input := `func printf(format: string, args: ...interface{}) {
}`

	result, _, err := processor.ProcessInternal(input)
	if err != nil {
		t.Fatalf("ProcessInternal failed: %v", err)
	}

	expected := "func printf(format string, args ...interface{})"
	if !strings.Contains(result, expected) {
		t.Errorf("Expected '%s', got: %s", expected, result)
	}
}

func TestTypeAnnotASTProcessor_MapTypes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple map",
			input:    `func process(m: map[string]int) {}`,
			expected: "func process(m map[string]int)",
		},
		{
			name:     "nested map",
			input:    `func process(m: map[string]map[int]string) {}`,
			expected: "func process(m map[string]map[int]string)",
		},
		{
			name:     "map with slice value",
			input:    `func process(m: map[string][]interface{}) {}`,
			expected: "func process(m map[string][]interface{})",
		},
	}

	processor := NewTypeAnnotASTProcessor()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := processor.ProcessInternal(tt.input)
			if err != nil {
				t.Fatalf("ProcessInternal failed: %v", err)
			}

			if !strings.Contains(result, tt.expected) {
				t.Errorf("Expected '%s', got: %s", tt.expected, result)
			}
		})
	}
}

func TestTypeAnnotASTProcessor_ChannelTypes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "bidirectional channel",
			input:    `func process(ch: chan int) {}`,
			expected: "func process(ch chan int)",
		},
		{
			name:     "receive-only channel",
			input:    `func process(ch: <-chan string) {}`,
			expected: "func process(ch <-chan string)",
		},
		{
			name:     "send-only channel",
			input:    `func process(ch: chan<- int) {}`,
			expected: "func process(ch chan<- int)",
		},
	}

	processor := NewTypeAnnotASTProcessor()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := processor.ProcessInternal(tt.input)
			if err != nil {
				t.Fatalf("ProcessInternal failed: %v", err)
			}

			if !strings.Contains(result, tt.expected) {
				t.Errorf("Expected '%s', got: %s", tt.expected, result)
			}
		})
	}
}

func TestTypeAnnotASTProcessor_PointerTypes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "single pointer",
			input:    `func process(p: *int) {}`,
			expected: "func process(p *int)",
		},
		{
			name:     "double pointer",
			input:    `func process(p: **string) {}`,
			expected: "func process(p **string)",
		},
		{
			name:     "pointer to struct",
			input:    `func process(p: *MyStruct) {}`,
			expected: "func process(p *MyStruct)",
		},
	}

	processor := NewTypeAnnotASTProcessor()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := processor.ProcessInternal(tt.input)
			if err != nil {
				t.Fatalf("ProcessInternal failed: %v", err)
			}

			if !strings.Contains(result, tt.expected) {
				t.Errorf("Expected '%s', got: %s", tt.expected, result)
			}
		})
	}
}

func TestTypeAnnotASTProcessor_SliceAndArrayTypes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "slice",
			input:    `func process(s: []int) {}`,
			expected: "func process(s []int)",
		},
		{
			name:     "array",
			input:    `func process(a: [10]int) {}`,
			expected: "func process(a [10]int)",
		},
		{
			name:     "slice of pointers",
			input:    `func process(s: []*string) {}`,
			expected: "func process(s []*string)",
		},
	}

	processor := NewTypeAnnotASTProcessor()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := processor.ProcessInternal(tt.input)
			if err != nil {
				t.Fatalf("ProcessInternal failed: %v", err)
			}

			if !strings.Contains(result, tt.expected) {
				t.Errorf("Expected '%s', got: %s", tt.expected, result)
			}
		})
	}
}

func TestTypeAnnotASTProcessor_QualifiedTypes(t *testing.T) {
	processor := NewTypeAnnotASTProcessor()
	input := `func process(ctx: context.Context, r: http.Request) error {
	return nil
}`

	result, _, err := processor.ProcessInternal(input)
	if err != nil {
		t.Fatalf("ProcessInternal failed: %v", err)
	}

	expected := "func process(ctx context.Context, r http.Request)"
	if !strings.Contains(result, expected) {
		t.Errorf("Expected '%s', got: %s", expected, result)
	}
}

func TestTypeAnnotASTProcessor_ReturnArrow(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple return",
			input:    `func getValue() -> string { return "" }`,
			expected: "func getValue() string {",
		},
		{
			name:     "error return",
			input:    `func process() -> error { return nil }`,
			expected: "func process() error {",
		},
		{
			name:     "pointer return",
			input:    `func create() -> *User { return nil }`,
			expected: "func create() *User {",
		},
	}

	processor := NewTypeAnnotASTProcessor()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := processor.ProcessInternal(tt.input)
			if err != nil {
				t.Fatalf("ProcessInternal failed: %v", err)
			}

			if !strings.Contains(result, tt.expected) {
				t.Errorf("Expected '%s', got: %s", tt.expected, result)
			}
		})
	}
}

func TestTypeAnnotASTProcessor_CombinedTransformations(t *testing.T) {
	processor := NewTypeAnnotASTProcessor()
	input := `func transform(x: int, y: string) -> Result<string, error> {
	return Ok(fmt.Sprintf("%d: %s", x, y))
}`

	result, metadata, err := processor.ProcessInternal(input)
	if err != nil {
		t.Fatalf("ProcessInternal failed: %v", err)
	}

	// Should transform both parameters and return type
	expected := "func transform(x int, y string) Result[string, error]"
	if !strings.Contains(result, expected) {
		t.Errorf("Expected '%s', got: %s", expected, result)
	}

	// Should have metadata
	if len(metadata) != 1 {
		t.Fatalf("Expected 1 metadata entry, got %d", len(metadata))
	}
}

func TestTypeAnnotASTProcessor_NoTransformation(t *testing.T) {
	processor := NewTypeAnnotASTProcessor()
	input := `package main

func add(x int, y int) int {
	return x + y
}`

	result, metadata, err := processor.ProcessInternal(input)
	if err != nil {
		t.Fatalf("ProcessInternal failed: %v", err)
	}

	// Should have NO metadata since no transformation happened
	if len(metadata) != 0 {
		t.Errorf("Expected 0 metadata entries (no transformation), got %d", len(metadata))
	}

	// Should not contain any markers
	if strings.Contains(result, "// dingo:t:") {
		t.Errorf("Result should not contain markers when no transformation occurred")
	}

	// Result should be unchanged
	if result != input {
		t.Errorf("Result should be unchanged when no transformations needed")
	}
}

func TestTypeAnnotASTProcessor_MultipleParameters(t *testing.T) {
	processor := NewTypeAnnotASTProcessor()
	// Note: Processor works line-by-line, so multi-line params won't be transformed
	// This tests single-line with multiple parameters
	input := `func complex(a: int, b: string, c: *User, d: []byte, e: map[string]int, f: func() error) -> error {
	return nil
}`

	result, _, err := processor.ProcessInternal(input)
	if err != nil {
		t.Fatalf("ProcessInternal failed: %v", err)
	}

	// Check each parameter transformation
	expectations := []string{
		"a int",
		"b string",
		"c *User",
		"d []byte",
		"e map[string]int",
		"f func() error",
		") error {",
	}

	for _, exp := range expectations {
		if !strings.Contains(result, exp) {
			t.Errorf("Expected '%s' in result, got: %s", exp, result)
		}
	}
}

func TestTypeAnnotASTProcessor_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty params",
			input:    `func foo() {}`,
			expected: `func foo() {}`,
		},
		{
			name:     "single param no type annotation",
			input:    `func foo(x int) {}`,
			expected: `func foo(x int) {}`,
		},
		{
			name:     "mixed annotated and non-annotated",
			input:    `func foo(x int, y: string) {}`,
			expected: `func foo(x int, y string) {}`,
		},
	}

	processor := NewTypeAnnotASTProcessor()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := processor.ProcessInternal(tt.input)
			if err != nil {
				t.Fatalf("ProcessInternal failed: %v", err)
			}

			if !strings.Contains(result, tt.expected) {
				t.Errorf("Expected '%s', got: %s", tt.expected, result)
			}
		})
	}
}

func TestTypeAnnotASTProcessor_Metadata(t *testing.T) {
	processor := NewTypeAnnotASTProcessor()
	input := `package main

func add(x: int, y: int) int {
	return x + y
}

func multiply(a: int, b: int) int {
	return a * b
}`

	_, metadata, err := processor.ProcessInternal(input)
	if err != nil {
		t.Fatalf("ProcessInternal failed: %v", err)
	}

	// Should have two metadata entries
	if len(metadata) != 2 {
		t.Fatalf("Expected 2 metadata entries, got %d", len(metadata))
	}

	// Check metadata fields
	for i, meta := range metadata {
		if meta.Type != "type_annot" {
			t.Errorf("Metadata[%d]: Expected type 'type_annot', got '%s'", i, meta.Type)
		}
		if meta.ASTNodeType != "FuncDecl" {
			t.Errorf("Metadata[%d]: Expected AST node type 'FuncDecl', got '%s'", i, meta.ASTNodeType)
		}
		expectedMarker := fmt.Sprintf("// dingo:t:%d", i)
		if meta.GeneratedMarker != expectedMarker {
			t.Errorf("Metadata[%d]: Expected marker '%s', got '%s'", i, expectedMarker, meta.GeneratedMarker)
		}
	}
}
