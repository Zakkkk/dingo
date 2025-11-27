package preprocessor

import (
	"strings"
	"testing"
)

func TestLetTransformation(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple assignment",
			input:    `let x = 5`,
			expected: `x := // dingo:let:x 5`,
		},
		{
			name:     "multiple variables",
			input:    `let x, y, z = func()`,
			expected: `x, y, z := // dingo:let:x,y,z func()`,
		},
		{
			name:     "with type annotation",
			input:    `let name: string = "hello"`,
			expected: `name: string := // dingo:let:name "hello"`,
		},
		{
			name:     "simple declaration",
			input:    `let x int`,
			expected: `var x int // dingo:let:x`,
		},
		{
			name:     "declaration with colon",
			input:    `let action: Action`,
			expected: `var action Action // dingo:let:action`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kp := NewKeywordProcessor()
			result, _, err := kp.Process([]byte(tt.input))
			if err != nil {
				t.Fatalf("Process failed: %v", err)
			}

			actual := strings.TrimSpace(string(result))
			expected := strings.TrimSpace(tt.expected)

			if actual != expected {
				t.Errorf("output mismatch:\n=== EXPECTED ===\n%s\n\n=== ACTUAL ===\n%s\n", expected, actual)
			}
		})
	}
}

// TestLetBacktrackingBug tests the regex backtracking bug where
// `let status = func() string { return "ok" }` incorrectly becomes
// `var statu s = func() string { return "ok" }`
func TestLetBacktrackingBug(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		buggy    string // What the buggy regex produces
	}{
		{
			name:     "function returning type",
			input:    `let status = func() string { return "ok" }`,
			expected: `status := // dingo:let:status func() string { return "ok" }`,
			buggy:    `var statu s = func() string { return "ok" }`,
		},
		{
			name:     "function with params",
			input:    `let callback = func(x int) int { return x * 2 }`,
			expected: `callback := // dingo:let:callback func(x int) int { return x * 2 }`,
			buggy:    `var callbac k = func(x int) int { return x * 2 }`,
		},
		{
			name:     "qualified type",
			input:    `let handler = http.HandlerFunc(myHandler)`,
			expected: `handler := // dingo:let:handler http.HandlerFunc(myHandler)`,
			buggy:    `var handle r = http.HandlerFunc(myHandler)`,
		},
		{
			name:     "simple call (should work even with regex)",
			input:    `let result = someCall()`,
			expected: `result := // dingo:let:result someCall()`,
			buggy:    `result := // dingo:let:result someCall()`, // This one works
		},
		{
			name:     "assignment with return type in function",
			input:    `let getData = func() ([]byte, error) { return nil, nil }`,
			expected: `getData := // dingo:let:getData func() ([]byte, error) { return nil, nil }`,
			buggy:    `var getDat a = func() ([]byte, error) { return nil, nil }`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kp := NewKeywordProcessor()
			result, _, err := kp.Process([]byte(tt.input))
			if err != nil {
				t.Fatalf("Process failed: %v", err)
			}

			actual := strings.TrimSpace(string(result))
			expected := strings.TrimSpace(tt.expected)

			if actual != expected {
				// Check if it's the known buggy output
				if strings.TrimSpace(tt.buggy) == actual {
					t.Errorf("BACKTRACKING BUG DETECTED:\n"+
						"=== INPUT ===\n%s\n\n"+
						"=== EXPECTED ===\n%s\n\n"+
						"=== ACTUAL (BUGGY) ===\n%s\n\n"+
						"The regex backtracked and split the identifier incorrectly!",
						tt.input, expected, actual)
				} else {
					t.Errorf("output mismatch:\n"+
						"=== EXPECTED ===\n%s\n\n"+
						"=== ACTUAL ===\n%s\n",
						expected, actual)
				}
			}
		})
	}
}

// TestLetInContext tests let transformations in realistic code contexts
func TestLetInContext(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "let in function",
			input: `package main

func process() error {
	let status = func() string { return "ok" }
	let x = 42
	return nil
}`,
			expected: `package main

func process() error {
	status := // dingo:let:status func() string { return "ok" }
	x := // dingo:let:x 42
	return nil
}`,
		},
		{
			name: "mixed let usage",
			input: `package main

func main() {
	let handler = http.HandlerFunc(myFunc)
	let name: string = "test"
	let count int
}`,
			expected: `package main

func main() {
	handler := // dingo:let:handler http.HandlerFunc(myFunc)
	name: string := // dingo:let:name "test"
	var count int // dingo:let:count
}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kp := NewKeywordProcessor()
			result, _, err := kp.Process([]byte(tt.input))
			if err != nil {
				t.Fatalf("Process failed: %v", err)
			}

			actual := strings.TrimSpace(string(result))
			expected := strings.TrimSpace(tt.expected)

			if actual != expected {
				t.Errorf("output mismatch:\n=== EXPECTED ===\n%s\n\n=== ACTUAL ===\n%s\n", expected, actual)
			}
		})
	}
}
