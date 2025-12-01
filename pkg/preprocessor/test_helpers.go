package preprocessor

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// assertValidGoSyntax validates that the generated code is syntactically valid Go.
// This is a critical validation that ensures transformations produce compilable output.
// If the code is not a full program, it wraps it in a function.
func assertValidGoSyntax(t *testing.T, code string) {
	t.Helper()

	fset := token.NewFileSet()

	// Try as-is first (for full programs)
	fullCode := "package main\n\n" + code
	_, err := parser.ParseFile(fset, "test.go", fullCode, 0)
	if err == nil {
		return // Valid as-is
	}

	// If that fails, try wrapping in a function (for statements/expressions)
	wrappedCode := "package main\n\nfunc test() {\n" + code + "\n}"
	_, err = parser.ParseFile(fset, "test.go", wrappedCode, 0)
	if err != nil {
		t.Fatalf("generated code is not valid Go (tried both as-is and wrapped in function): %v\nCode:\n%s", err, code)
	}
}

// assertCompiles is an alias for assertValidGoSyntax for semantic clarity
// when checking that generated code compiles.
func assertCompiles(t *testing.T, code string) {
	t.Helper()
	assertValidGoSyntax(t, code)
}

// normalizeWhitespace removes extra whitespace for exact comparison testing.
// Useful when comparing expected vs actual output where whitespace may vary.
func normalizeWhitespace(s string) string {
	// Replace multiple spaces with single space
	s = strings.Join(strings.Fields(s), " ")
	// Normalize newlines
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.TrimSpace(s)
}

// assertExactMatch compares normalized code strings for exact equality.
// Use this for critical transformations where substring matching is insufficient.
func assertExactMatch(t *testing.T, expected, actual string) {
	t.Helper()

	normExpected := normalizeWhitespace(expected)
	normActual := normalizeWhitespace(actual)

	if normExpected != normActual {
		t.Errorf("exact output mismatch:\nexpected:\n%s\ngot:\n%s", expected, actual)
	}
}
