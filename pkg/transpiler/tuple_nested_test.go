package transpiler

import (
	"strings"
	"testing"
)

func TestNestedTupleDestructuring(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "flat destructure",
			input:    "(x, y) := point",
			expected: `__tupleDest2__("x:0", "y:1", point)`,
		},
		{
			name:     "nested destructure",
			input:    "((a, b), c) := bbox",
			expected: `__tupleDest3__("a:0.0", "b:0.1", "c:1", bbox)`,
		},
		{
			name:     "wildcard skip",
			input:    "(x, _) := pair",
			expected: `__tupleDest1__("x:0", pair)`,
		},
		{
			name:     "nested with wildcard",
			input:    "((_, b), c) := nested",
			expected: `__tupleDest2__("b:0.1", "c:1", nested)`,
		},
		{
			name:     "deeply nested",
			input:    "(((a, b), c), d) := deep",
			expected: `__tupleDest4__("a:0.0.0", "b:0.0.1", "c:0.1", "d:1", deep)`,
		},
		{
			name:     "all wildcards",
			input:    "(_, _) := pair",
			expected: "_ = pair", // When all wildcards, just evaluate RHS
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			input := []byte(tc.input)
			result, err := transformTupleDestructuring(input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			resultStr := string(result)
			if !strings.Contains(resultStr, tc.expected) {
				t.Errorf("expected to contain %q, got %q", tc.expected, resultStr)
			}
		})
	}
}
