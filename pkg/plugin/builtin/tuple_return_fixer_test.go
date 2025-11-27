package builtin

import (
	"go/parser"
	"go/printer"
	"go/token"
	"strings"
	"testing"

	"github.com/MadAppGang/dingo/pkg/plugin"
)

func TestTupleReturnFixer_SimpleSingleLine(t *testing.T) {
	input := `package main

func test() (int, string) {
	return __TUPLE_2__LITERAL__abc12345(42, "hello")
}
`

	expected := `package main

func test() (int, string) {
	return 42, "hello"
}
`

	testTupleReturnFixer(t, input, expected)
}

func TestTupleReturnFixer_MultilineReturn(t *testing.T) {
	input := `package main

func readConfig() (*Config, error) {
	cfg := &Config{}
	return __TUPLE_2__LITERAL__def67890(
		cfg, nil,
	)
}
`

	expected := `package main

func readConfig() (*Config, error) {
	cfg := &Config{}
	return cfg, nil

}
`

	testTupleReturnFixer(t, input, expected)
}

func TestTupleReturnFixer_Tuple3(t *testing.T) {
	input := `package main

func multi() (int, string, bool) {
	return __TUPLE_3__LITERAL__abc12345(1, "two", true)
}
`

	expected := `package main

func multi() (int, string, bool) {
	return 1, "two", true
}
`

	testTupleReturnFixer(t, input, expected)
}

func TestTupleReturnFixer_NestedTuplesInReturn(t *testing.T) {
	// Inner Tuple2 should NOT be expanded (it's an argument, not the return itself)
	input := `package main

func nested() (Tuple2IntString, int) {
	return __TUPLE_2__LITERAL__abc12345(__TUPLE_2__LITERAL__abc12345(1, "inner"), 42)
}
`

	// Both tuples should be expanded in return context
	expected := `package main

func nested() (Tuple2IntString, int) {
	return __TUPLE_2__LITERAL__abc12345(1, "inner"), 42
}
`

	testTupleReturnFixer(t, input, expected)
}

func TestTupleReturnFixer_NonReturnContextUnchanged(t *testing.T) {
	input := `package main

func test() {
	x := __TUPLE_2__LITERAL__abc12345(1, 2)
	y := __TUPLE_3__LITERAL__abc12345(1, 2, 3)
	println(x, y)
}
`

	// No changes - Tuple calls outside return statements should be preserved
	expected := input

	testTupleReturnFixer(t, input, expected)
}

func TestTupleReturnFixer_MixedReturns(t *testing.T) {
	input := `package main

func mixed() (int, string, bool) {
	if true {
		return __TUPLE_3__LITERAL__abc12345(1, "one", true)
	}
	return 2, "two", false
}
`

	expected := `package main

func mixed() (int, string, bool) {
	if true {
		return 1, "one", true
	}
	return 2, "two", false
}
`

	testTupleReturnFixer(t, input, expected)
}

func TestTupleReturnFixer_MultipleReturnStatements(t *testing.T) {
	input := `package main

func multiple() (int, error) {
	if err := check(); err != nil {
		return __TUPLE_2__LITERAL__abc12345(0, err)
	}
	return __TUPLE_2__LITERAL__abc12345(42, nil)
}
`

	expected := `package main

func multiple() (int, error) {
	if err := check(); err != nil {
		return 0, err
	}
	return 42, nil
}
`

	testTupleReturnFixer(t, input, expected)
}

func TestTupleReturnFixer_ComplexExpressions(t *testing.T) {
	input := `package main

func complex() (int, string) {
	return __TUPLE_2__LITERAL__abc12345(1+2*3, "hello" + " world")
}
`

	expected := `package main

func complex() (int, string) {
	return 1 + 2*3, "hello" + " world"
}
`

	testTupleReturnFixer(t, input, expected)
}

func TestTupleReturnFixer_NonTupleFunctionUnchanged(t *testing.T) {
	input := `package main

func test() SomeType {
	return SomeFunction(1, 2, 3)
}
`

	// No changes - not a Tuple function
	expected := input

	testTupleReturnFixer(t, input, expected)
}

func TestTupleReturnFixer_ArityMismatch(t *testing.T) {
	// Tuple2 with 3 arguments - should be left unchanged (invalid)
	input := `package main

func test() (int, string, bool) {
	return __TUPLE_2__LITERAL__abc12345(1, "two", true)
}
`

	// No changes - arity mismatch means this isn't a valid tuple call
	expected := input

	testTupleReturnFixer(t, input, expected)
}

func TestIsPreprocessorTupleMarker(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"Valid Tuple2 marker", "__TUPLE_2__LITERAL__abc12345", true},
		{"Valid Tuple3 marker", "__TUPLE_3__LITERAL__def67890", true},
		{"Valid Tuple10 marker", "__TUPLE_10__LITERAL__12345678", true},
		{"Invalid - old Tuple2 format", "Tuple2", false},
		{"Invalid - missing __LITERAL__", "__TUPLE_2__abc", false},
		{"Invalid - no number", "__TUPLE___LITERAL__abc", false},
		{"Invalid - lowercase", "__tuple_2__LITERAL__abc", false},
		{"Invalid - non-digit", "__TUPLE_2X__LITERAL__abc", false},
		{"Invalid - too short", "__TUPLE_", false},
		{"Invalid - empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isPreprocessorTupleMarker(tt.input)
			if result != tt.expected {
				t.Errorf("isPreprocessorTupleMarker(%q) = %v, want %v",
					tt.input, result, tt.expected)
			}
		})
	}
}

func TestExtractMarkerArity(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{"Marker Tuple2", "__TUPLE_2__LITERAL__abc12345", 2},
		{"Marker Tuple3", "__TUPLE_3__LITERAL__def67890", 3},
		{"Marker Tuple10", "__TUPLE_10__LITERAL__12345678", 10},
		{"Invalid - old format", "Tuple2", 0},
		{"Invalid - missing __LITERAL__", "__TUPLE_2__abc", 0},
		{"Invalid format", "__TUPLE___LITERAL__abc", 0},
		{"Not a marker", "SomethingElse", 0},
		{"Empty", "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractMarkerArity(tt.input)
			if result != tt.expected {
				t.Errorf("extractMarkerArity(%q) = %d, want %d",
					tt.input, result, tt.expected)
			}
		})
	}
}

// testTupleReturnFixer is a helper that runs the plugin and compares output
func testTupleReturnFixer(t *testing.T, input, expected string) {
	t.Helper()

	// Parse input
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", input, parser.ParseComments)
	if err != nil {
		t.Fatalf("Failed to parse input: %v", err)
	}

	// Create plugin with context
	ctx := &plugin.Context{
		FileSet: fset,
		Logger:  plugin.NewNoOpLogger(),
	}

	plug := NewTupleReturnFixer()
	plug.SetContext(ctx)

	// Run Process phase (no-op for this plugin)
	if err := plug.Process(file); err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	// Run Transform phase
	transformed, err := plug.Transform(file)
	if err != nil {
		t.Fatalf("Transform failed: %v", err)
	}

	// Convert back to source
	var buf strings.Builder
	if err := printer.Fprint(&buf, fset, transformed); err != nil {
		t.Fatalf("Failed to print transformed AST: %v", err)
	}

	// Compare (normalize whitespace)
	actual := strings.TrimSpace(buf.String())
	expectedNorm := strings.TrimSpace(expected)

	if actual != expectedNorm {
		t.Errorf("Output mismatch:\n=== Expected ===\n%s\n=== Actual ===\n%s\n",
			expectedNorm, actual)
	}
}
