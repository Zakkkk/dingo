package ast

import (
	"testing"
)

func TestTransformEnumConstructors(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		registry map[string]string
		expected string
	}{
		{
			name: "qualified constructor with empty struct",
			input: `
status := PaymentStatus.Pending{}
`,
			registry: map[string]string{
				"Pending":              "PaymentStatus",
				"PaymentStatusPending": "PaymentStatus",
			},
			expected: `
status := NewPaymentStatusPending()
`,
		},
		{
			name: "qualified constructor with struct fields",
			input: `
resp := APIResponse.Success{transactionID: "TXN-123", amount: 99.99}
`,
			registry: map[string]string{
				"Success":            "APIResponse",
				"APIResponseSuccess": "APIResponse",
			},
			expected: `
resp := NewAPIResponseSuccess("TXN-123", 99.99)
`,
		},
		{
			name: "unqualified constructor with args",
			input: `
result := Ok(42)
`,
			registry: map[string]string{
				"Ok":       "Result",
				"ResultOk": "Result",
			},
			expected: `
result := NewResultOk(42)
`,
		},
		{
			name: "multiple constructors",
			input: `
s1 := Status.Active{}
s2 := Status.Pending{}
`,
			registry: map[string]string{
				"Active":        "Status",
				"Pending":       "Status",
				"StatusActive":  "Status",
				"StatusPending": "Status",
			},
			expected: `
s1 := NewStatusActive()
s2 := NewStatusPending()
`,
		},
		{
			name: "constructor with multiple fields",
			input: `
payment := PaymentStatus.Processing{processorID: "STRIPE"}
`,
			registry: map[string]string{
				"Processing":              "PaymentStatus",
				"PaymentStatusProcessing": "PaymentStatus",
			},
			expected: `
payment := NewPaymentStatusProcessing("STRIPE")
`,
		},
		{
			name: "no transformation when not in registry",
			input: `
x := SomeType.SomeField{}
`,
			registry: map[string]string{
				"Ok": "Result",
			},
			expected: `
x := SomeType.SomeField{}
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TransformEnumConstructors([]byte(tt.input), tt.registry)
			if string(result) != tt.expected {
				t.Errorf("TransformEnumConstructors() =\n%q\nwant:\n%q", string(result), tt.expected)
			}
		})
	}
}

func TestFindMatchingBracket(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		pos      int
		openChar byte
		expected int
	}{
		{
			name:     "simple braces",
			src:      "test{value}",
			pos:      4, // position of '{'
			openChar: '{',
			expected: 10, // position of '}'
		},
		{
			name:     "nested braces",
			src:      "test{a{b}c}",
			pos:      4, // position of outer '{'
			openChar: '{',
			expected: 10, // position of outer '}'
		},
		{
			name:     "with string literal",
			src:      `test{"value"}`,
			pos:      4, // position of '{'
			openChar: '{',
			expected: 12, // position of '}'
		},
		{
			name:     "simple parens",
			src:      "func(x, y)",
			pos:      4, // position of '('
			openChar: '(',
			expected: 9, // position of ')'
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := findMatchingBracket([]byte(tt.src), tt.pos, tt.openChar)
			if result != tt.expected {
				t.Errorf("findMatchingBracket() = %d, want %d", result, tt.expected)
			}
		})
	}
}
