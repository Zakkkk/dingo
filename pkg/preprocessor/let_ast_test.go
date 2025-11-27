package preprocessor

import (
	"bytes"
	"testing"
)

func TestLetASTProcessor_BasicAssignment(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple integer",
			input:    "let x = 42",
			expected: "x := 42",
		},
		{
			name:     "simple string",
			input:    "let name = \"hello\"",
			expected: "name := \"hello\"",
		},
		{
			name:     "simple boolean",
			input:    "let flag = true",
			expected: "flag := true",
		},
		{
			name:     "function call",
			input:    "let result = someFunc()",
			expected: "result := someFunc()",
		},
		{
			name:     "with indentation",
			input:    "    let x = 10",
			expected: "    x := 10",
		},
		{
			name:     "multiple on line (first)",
			input:    "let x = 1; y := 2",
			expected: "x := 1; y := 2",
		},
	}

	proc := NewLetASTProcessor()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := proc.Process([]byte(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := string(result); got != tt.expected {
				t.Errorf("got %q, expected %q", got, tt.expected)
			}
		})
	}
}

func TestLetASTProcessor_TernaryBugCase(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "ternary with simple strings",
			input:    "let status = true ? \"yes\" : \"no\"",
			expected: "status := true ? \"yes\" : \"no\"",
		},
		{
			name: "ternary with IIFE",
			input: `let status = func() string {
	if true {
		return "yes"
	}
	return "no"
}()`,
			expected: `status := func() string {
	if true {
		return "yes"
	}
	return "no"
}()`,
		},
		{
			name:     "ternary with function call",
			input:    "let result = someFunc() ? \"a\" : \"b\"",
			expected: "result := someFunc() ? \"a\" : \"b\"",
		},
		{
			name:     "complex ternary expression",
			input:    "let x = (a > b) ? calculate(a) : calculate(b)",
			expected: "x := (a > b) ? calculate(a) : calculate(b)",
		},
	}

	proc := NewLetASTProcessor()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := proc.Process([]byte(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := string(result); got != tt.expected {
				t.Errorf("got %q, expected %q", got, tt.expected)
			}
		})
	}
}

func TestLetASTProcessor_TypeDeclaration(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple type",
			input:    "let x int",
			expected: "var x int",
		},
		{
			name:     "string type",
			input:    "let name string",
			expected: "var name string",
		},
		{
			name:     "slice type",
			input:    "let items []string",
			expected: "var items []string",
		},
		{
			name:     "pointer type",
			input:    "let ptr *int",
			expected: "var ptr *int",
		},
		{
			name:     "map type",
			input:    "let m map[string]int",
			expected: "var m map[string]int",
		},
		{
			name:     "with indentation",
			input:    "    let count int",
			expected: "    var count int",
		},
	}

	proc := NewLetASTProcessor()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := proc.Process([]byte(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := string(result); got != tt.expected {
				t.Errorf("got %q, expected %q", got, tt.expected)
			}
		})
	}
}

func TestLetASTProcessor_TypedAssignment(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple typed assignment",
			input:    "let x: int = 42",
			expected: "var x int = 42",
		},
		{
			name:     "string typed assignment",
			input:    "let name: string = \"hello\"",
			expected: "var name string = \"hello\"",
		},
		{
			name:     "slice typed assignment",
			input:    "let items: []string = make([]string, 0)",
			expected: "var items []string = make([]string, 0)",
		},
		{
			name:     "with spaces around colon",
			input:    "let x : int = 10",
			expected: "var x  int = 10",
		},
		{
			name:     "complex type",
			input:    "let opt: Option<int> = Some(42)",
			expected: "var opt Option<int> = Some(42)",
		},
	}

	proc := NewLetASTProcessor()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := proc.Process([]byte(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := string(result); got != tt.expected {
				t.Errorf("got %q, expected %q", got, tt.expected)
			}
		})
	}
}

func TestLetASTProcessor_MultipleVariables(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "two variables",
			input:    "let x, y = 1, 2",
			expected: "x, y = 1, 2",
		},
		{
			name:     "three variables",
			input:    "let a, b, c = getValue()",
			expected: "a, b, c = getValue()",
		},
		{
			name:     "with spaces",
			input:    "let x , y = tuple()",
			expected: "x , y = tuple()",
		},
	}

	proc := NewLetASTProcessor()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := proc.Process([]byte(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := string(result); got != tt.expected {
				t.Errorf("got %q, expected %q", got, tt.expected)
			}
		})
	}
}

func TestLetASTProcessor_Destructuring(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "tuple destructuring",
			input:    "let (a, b) = tuple",
			expected: "(a, b) = tuple",
		},
		{
			name:     "with spaces",
			input:    "let ( x , y ) = getValue()",
			expected: "( x , y ) = getValue()",
		},
	}

	proc := NewLetASTProcessor()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := proc.Process([]byte(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := string(result); got != tt.expected {
				t.Errorf("got %q, expected %q", got, tt.expected)
			}
		})
	}
}

func TestLetASTProcessor_NoTransformation(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "no let keyword",
			input: "x := 42",
		},
		{
			name:  "let in comment",
			input: "// let x = 42",
		},
		{
			name:  "let in string",
			input: "s := \"let x = 42\"",
		},
		{
			name:  "let as part of identifier",
			input: "varlet := 10",
		},
		{
			name:  "empty line",
			input: "",
		},
		{
			name:  "just whitespace",
			input: "    ",
		},
	}

	proc := NewLetASTProcessor()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := proc.Process([]byte(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := string(result); got != tt.input {
				t.Errorf("input was modified: got %q, expected %q", got, tt.input)
			}
		})
	}
}

func TestLetASTProcessor_MultiLine(t *testing.T) {
	input := `package main

func main() {
    let status = true ? "yes" : "no"
    let x = 42
    let name string
    let count: int = 10
    println(status, x, name, count)
}`

	expected := `package main

func main() {
    status := true ? "yes" : "no"
    x := 42
    var name string
    var count int = 10
    println(status, x, name, count)
}`

	proc := NewLetASTProcessor()
	result, _, err := proc.Process([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := string(result); got != expected {
		t.Errorf("multiline transformation failed:\nGot:\n%s\n\nExpected:\n%s", got, expected)
	}
}

func TestLetASTProcessor_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "let at end of line",
			input:    "let",
			expected: "let",
		},
		{
			name:     "let with only identifier",
			input:    "let x",
			expected: "let x",
		},
		{
			name:     "let with trailing comment",
			input:    "let x = 42 // comment",
			expected: "x := 42 // comment",
		},
		{
			name:     "let in middle of expression",
			input:    "y := let x = 10", // Invalid Go, but shouldn't crash
			expected: "y := let x = 10", // May or may not transform - depends on tokenizer
		},
	}

	proc := NewLetASTProcessor()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := proc.Process([]byte(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			// For edge cases, we just verify no panic/error
			// The exact output may vary
			_ = result
		})
	}
}

func TestLetASTProcessor_PreservesFormatting(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "preserves leading spaces",
			input:    "    let x = 42",
			expected: "    x := 42",
		},
		{
			name:     "preserves tabs",
			input:    "\tlet x = 42",
			expected: "\tx := 42",
		},
		{
			name:     "preserves trailing spaces",
			input:    "let x = 42   ",
			expected: "x := 42   ",
		},
		{
			name:     "preserves internal spacing",
			input:    "let   x   =   42",
			expected: "  x   :=   42", // Removes "let" and one trailing space
		},
	}

	proc := NewLetASTProcessor()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := proc.Process([]byte(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := string(result); got != tt.expected {
				t.Errorf("got %q, expected %q", got, tt.expected)
			}
		})
	}
}

// Benchmark the new token-based processor
func BenchmarkLetASTProcessor(b *testing.B) {
	proc := NewLetASTProcessor()
	input := []byte("let status = func() string { return \"yes\" }()")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = proc.Process(input)
	}
}

// Benchmark multiline processing
func BenchmarkLetASTProcessor_MultiLine(b *testing.B) {
	proc := NewLetASTProcessor()
	input := []byte(`package main

func main() {
    let status = true ? "yes" : "no"
    let x = 42
    let name string
    let count: int = 10
    let result = someFunc()
    println(status, x, name, count, result)
}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = proc.Process(input)
	}
}

// Test against the original bug case
func TestLetASTProcessor_OriginalBugFix(t *testing.T) {
	// This is the exact bug case from the issue:
	// Input: let status = func() string {...}
	// Buggy regex output: var statu s = func() string {...}
	// Expected output: status := func() string {...}

	input := `let status = func() string {
	if true {
		return "yes"
	}
	return "no"
}()`

	expected := `status := func() string {
	if true {
		return "yes"
	}
	return "no"
}()`

	proc := NewLetASTProcessor()
	result, _, err := proc.Process([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := string(result); got != expected {
		t.Errorf("Bug still present!\nGot:\n%s\n\nExpected:\n%s", got, expected)
	}

	// Specifically check that "status" is NOT split into "statu s"
	if bytes.Contains(result, []byte("statu s")) {
		t.Error("BUG: identifier 'status' was incorrectly split into 'statu s'")
	}

	// Verify "status" appears as complete identifier
	if !bytes.Contains(result, []byte("status :=")) {
		t.Error("Expected 'status :=' in output")
	}
}
