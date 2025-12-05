package parser

import (
	"testing"
)

func TestTransformGenerics(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple generic type",
			input:    "Result<int, error>",
			expected: "Result[int, error]",
		},
		{
			name:     "function with generic return",
			input:    "func foo() Result<int, error> { }",
			expected: "func foo() Result[int, error] { }",
		},
		{
			name:     "function with params and generic",
			input:    "func FindUser(id: int) Result<User, DBError> { }",
			expected: "func FindUser(id int) Result[User, DBError] { }",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := TransformToGo([]byte(tt.input))
			if err != nil {
				t.Fatalf("TransformToGo failed: %v", err)
			}

			got := normalizeWhitespace(string(result))
			want := normalizeWhitespace(tt.expected)

			t.Logf("Input:    %q", tt.input)
			t.Logf("Output:   %q", string(result))
			t.Logf("Expected: %q", tt.expected)

			if got != want {
				t.Errorf("TransformToGo mismatch:\n  got:  %q\n  want: %q", got, want)
			}
		})
	}
}
