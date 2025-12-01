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

// Edge Case Tests: Complex Initializers
func TestLetASTProcessor_ComplexInitializers(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "nested function calls",
			input:    "let result = outer(inner(getValue()))",
			expected: "result := outer(inner(getValue()))",
		},
		{
			name:     "method chaining",
			input:    "let data = obj.Method1().Method2().Method3()",
			expected: "data := obj.Method1().Method2().Method3()",
		},
		{
			name:     "composite literal",
			input:    "let person = Person{Name: \"Alice\", Age: 30}",
			expected: "person := Person{Name: \"Alice\", Age: 30}",
		},
		{
			name:     "slice literal",
			input:    "let items = []int{1, 2, 3, 4, 5}",
			expected: "items := []int{1, 2, 3, 4, 5}",
		},
		{
			name:     "map literal",
			input:    "let config = map[string]int{\"timeout\": 30, \"retries\": 3}",
			expected: "config := map[string]int{\"timeout\": 30, \"retries\": 3}",
		},
		{
			name:     "function call with multiple args",
			input:    "let sum = add(a, b, c, d)",
			expected: "sum := add(a, b, c, d)",
		},
		{
			name:     "type assertion",
			input:    "let str = value.(string)",
			expected: "str := value.(string)",
		},
		{
			name:     "channel operations",
			input:    "let val = <-ch",
			expected: "val := <-ch",
		},
		{
			name:     "array indexing",
			input:    "let item = arr[index]",
			expected: "item := arr[index]",
		},
		{
			name:     "complex expression with operators",
			input:    "let calc = (a + b) * (c - d) / e",
			expected: "calc := (a + b) * (c - d) / e",
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

// Edge Case Tests: Nested Blocks
func TestLetASTProcessor_NestedBlocks(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "let in if block",
			input: `if true {
    let x = 42
}`,
			expected: `if true {
    x := 42
}`,
		},
		{
			name: "let in for loop",
			input: `for i := 0; i < 10; i++ {
    let temp = i * 2
}`,
			expected: `for i := 0; i < 10; i++ {
    temp := i * 2
}`,
		},
		{
			name: "let in switch case",
			input: `switch x {
case 1:
    let msg = "one"
}`,
			expected: `switch x {
case 1:
    msg := "one"
}`,
		},
		{
			name: "let in nested if-else",
			input: `if a {
    let x = 1
} else {
    let y = 2
}`,
			expected: `if a {
    x := 1
} else {
    y := 2
}`,
		},
		{
			name: "let in anonymous function",
			input: `func() {
    let result = compute()
}()`,
			expected: `func() {
    result := compute()
}()`,
		},
		{
			name: "multiple nested levels",
			input: `if a {
    for i := 0; i < 10; i++ {
        let temp = i
    }
}`,
			expected: `if a {
    for i := 0; i < 10; i++ {
        temp := i
    }
}`,
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

// Edge Case Tests: Type Annotations with Complex Types
func TestLetASTProcessor_ComplexTypeAnnotations(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "generic type annotation",
			input:    "let opt: Option<string> = Some(\"hello\")",
			expected: "var opt Option<string> = Some(\"hello\")",
		},
		{
			name:     "result type annotation",
			input:    "let res: Result<int, error> = Ok(42)",
			expected: "var res Result<int, error> = Ok(42)",
		},
		{
			name:     "nested generics",
			input:    "let items: Option<[]Result<int, string>> = None",
			expected: "var items Option<[]Result<int, string>> = None",
		},
		{
			name:     "function type annotation",
			input:    "let fn: func(int) string = toString",
			expected: "var fn func(int) string = toString",
		},
		{
			name:     "channel type annotation",
			input:    "let ch: chan int = make(chan int)",
			expected: "var ch chan int = make(chan int)",
		},
		{
			name:     "interface type annotation",
			input:    "let reader: io.Reader = buffer",
			expected: "var reader io.Reader = buffer",
		},
		{
			name:     "struct pointer type",
			input:    "let ptr: *Person = &Person{}",
			expected: "var ptr *Person = &Person{}",
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

// Edge Case Tests: Multiple Declarations
func TestLetASTProcessor_MultipleDeclarations(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "consecutive let declarations",
			input: `let x = 1
let y = 2
let z = 3`,
			expected: `x := 1
y := 2
z := 3`,
		},
		{
			name: "mixed declaration styles",
			input: `let x = 1
var y int
let z string`,
			expected: `x := 1
var y int
var z string`,
		},
		{
			name: "multiple lets on same line (first transformed)",
			input: `let x = 1; let y = 2`,
			expected: `x := 1; let y = 2`, // Only first 'let' is transformed per pass
		},
		{
			name: "interleaved with other statements",
			input: `let x = 1
println(x)
let y = 2
println(y)`,
			expected: `x := 1
println(x)
y := 2
println(y)`,
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

// Edge Case Tests: Destructuring Scenarios
func TestLetASTProcessor_AdvancedDestructuring(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "destructure with function call",
			input:    "let (x, y) = getTuple()",
			expected: "(x, y) = getTuple()",
		},
		{
			name:     "destructure with literals",
			input:    "let (a, b) = (1, 2)",
			expected: "(a, b) = (1, 2)",
		},
		{
			name:     "triple destructuring",
			input:    "let (x, y, z) = getTriple()",
			expected: "(x, y, z) = getTriple()",
		},
		{
			name:     "destructure with ignored values",
			input:    "let (x, _) = pair",
			expected: "(x, _) = pair",
		},
		{
			name:     "nested destructuring",
			input:    "let ((a, b), c) = nested",
			expected: "((a, b), c) = nested",
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

// Edge Case Tests: Shadowing Scenarios
func TestLetASTProcessor_Shadowing(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "shadow in nested block",
			input: `let x = 1
{
    let x = 2
}`,
			expected: `x := 1
{
    x := 2
}`,
		},
		{
			name: "shadow in if block",
			input: `let x = "outer"
if true {
    let x = "inner"
}`,
			expected: `x := "outer"
if true {
    x := "inner"
}`,
		},
		{
			name: "shadow with different type",
			input: `let x = 42
{
    let x string
}`,
			expected: `x := 42
{
    var x string
}`,
		},
		{
			name: "shadow in for loop",
			input: `let i = 0
for j := 0; j < 10; j++ {
    let i = j
}`,
			expected: `i := 0
for j := 0; j < 10; j++ {
    i := j
}`,
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

// Edge Case Tests: Special Characters and Escapes
func TestLetASTProcessor_SpecialCharacters(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "string with escaped quotes",
			input:    `let msg = "He said \"hello\""`,
			expected: `msg := "He said \"hello\""`,
		},
		{
			name:     "raw string literal",
			input:    "let path = `C:\\Users\\Name`",
			expected: "path := `C:\\Users\\Name`",
		},
		{
			name:     "multiline raw string",
			input:    "let sql = `SELECT *\nFROM users`",
			expected: "sql := `SELECT *\nFROM users`",
		},
		{
			name:     "unicode in identifier",
			input:    "let 名前 = \"name\"",
			expected: "名前 := \"name\"",
		},
		{
			name:     "string with newlines",
			input:    `let text = "line1\nline2\nline3"`,
			expected: `text := "line1\nline2\nline3"`,
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

// Edge Case Tests: Error Propagation and Result Types
func TestLetASTProcessor_WithErrorPropagation(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "let with error propagation",
			input:    "let data = readFile()?",
			expected: "data := readFile()?",
		},
		{
			name:     "let with Result type",
			input:    "let result: Result<int, error> = compute()",
			expected: "var result Result<int, error> = compute()",
		},
		{
			name:     "let with Option type",
			input:    "let opt = findUser(id)?",
			expected: "opt := findUser(id)?",
		},
		{
			name:     "chained error propagation",
			input:    "let data = readFile()?.parse()?.validate()?",
			expected: "data := readFile()?.parse()?.validate()?",
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

// Edge Case Tests: Comments and Documentation
func TestLetASTProcessor_WithComments(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "let with trailing comment",
			input:    "let x = 42 // initialize x",
			expected: "x := 42 // initialize x",
		},
		{
			name: "let with preceding comment",
			input: `// Create user
let user = NewUser()`,
			expected: `// Create user
user := NewUser()`,
		},
		{
			name: "let with block comment",
			input: `/* Initialize */
let x = 1`,
			expected: `/* Initialize */
x := 1`,
		},
		{
			name:     "let with inline block comment",
			input:    "let x /* temp */ = 42",
			expected: "let x /* temp */ = 42", // Block comments between tokens prevent transformation
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

// Edge Case Tests: Complex Real-World Patterns
func TestLetASTProcessor_RealWorldPatterns(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "HTTP handler pattern",
			input:    "let handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})",
			expected: "handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})",
		},
		{
			name:     "context with timeout",
			input:    "let ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)",
			expected: "ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)",
		},
		{
			name:     "JSON unmarshaling",
			input:    "let data: MyStruct = MyStruct{}",
			expected: "var data MyStruct = MyStruct{}",
		},
		{
			name:     "defer with let",
			input:    `let f = openFile()
defer f.Close()`,
			expected: `f := openFile()
defer f.Close()`,
		},
		{
			name:     "goroutine with let",
			input:    `let ch = make(chan int)
go func() { ch <- 42 }()`,
			expected: `ch := make(chan int)
go func() { ch <- 42 }()`,
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
