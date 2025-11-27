package generator

import (
	"strings"
	"testing"
)

func TestMarkerInjector_InjectMarkers(t *testing.T) {
	tests := []struct {
		name     string
		enabled  bool
		source   string
		expected string
	}{
		{
			name:    "disabled - no markers",
			enabled: false,
			source: `package main

func process() error {
	x, __err0 := fetchUser()
	if __err0 != nil {
		return __err0
	}
	return nil
}
`,
			expected: `package main

func process() error {
	x, __err0 := fetchUser()
	if __err0 != nil {
		return __err0
	}
	return nil
}
`,
		},
		{
			name:    "enabled - adds markers",
			enabled: true,
			source: `package main

func process() error {
	x, __err0 := fetchUser()
	if __err0 != nil {
		return __err0
	}
	return nil
}
`,
			expected: `package main

func process() error {
	x, __err0 := fetchUser()
	// dingo:s:1
	if __err0 != nil {
		return __err0
	}
	// dingo:e:1
	return nil
}
`,
		},
		{
			name:    "enabled - multiple error checks",
			enabled: true,
			source: `package main

func process() error {
	x, __err0 := fetchUser()
	if __err0 != nil {
		return __err0
	}
	y, __err1 := fetchPost()
	if __err1 != nil {
		return __err1
	}
	return nil
}
`,
			expected: `package main

func process() error {
	x, __err0 := fetchUser()
	// dingo:s:1
	if __err0 != nil {
		return __err0
	}
	// dingo:e:1
	y, __err1 := fetchPost()
	// dingo:s:1
	if __err1 != nil {
		return __err1
	}
	// dingo:e:1
	return nil
}
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			injector := NewMarkerInjector(tt.enabled)
			result, err := injector.InjectMarkers([]byte(tt.source))
			if err != nil {
				t.Fatalf("InjectMarkers() error = %v", err)
			}

			resultStr := string(result)
			if !equalIgnoringWhitespace(resultStr, tt.expected) {
				t.Errorf("InjectMarkers() result mismatch\nGot:\n%s\n\nExpected:\n%s", resultStr, tt.expected)
			}
		})
	}
}

// equalIgnoringWhitespace compares two strings while normalizing whitespace
func equalIgnoringWhitespace(a, b string) bool {
	return strings.TrimSpace(a) == strings.TrimSpace(b)
}

func TestRemoveDebugMarkers(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "standalone error marker",
			input: `package main

func process() error {
	x, err := fetchUser()
	// dingo:e:0
	if err != nil {
		return err
	}
	return nil
}`,
			// CRITICAL: Empty line preserved to maintain source map line numbers
			expected: `package main

func process() error {
	x, err := fetchUser()

	if err != nil {
		return err
	}
	return nil
}`,
		},
		{
			name: "marker at end of line",
			input: `package main

func process() error {
	x := 42 // dingo:let:x
	return nil
}`,
			expected: `package main

func process() error {
	x := 42
	return nil
}`,
		},
		{
			name: "marker with other content",
			input: `package main

func process() error {
	x := 42 // dingo:let:x important comment
	return nil
}`,
			expected: `package main

func process() error {
	x := 42 // important comment
	return nil
}`,
		},
		{
			name: "multiple marker types",
			input: `package main

// dingo:n:0
func process() error {
	x := 42 // dingo:let:x
	// dingo:e:1
	if err != nil {
		return err
	}
	return nil
}`,
			// CRITICAL: Empty lines preserved to maintain source map line numbers
			// The blank line after "package main" is from the // dingo:n:0 marker
			expected: `package main


func process() error {
	x := 42

	if err != nil {
		return err
	}
	return nil
}`,
		},
		{
			name: "type marker",
			input: `package main

func process() error {
	tmp := getValue() // dingo:t:0
	return nil
}`,
			expected: `package main

func process() error {
	tmp := getValue()
	return nil
}`,
		},
		{
			name: "no markers",
			input: `package main

func process() error {
	x := 42 // normal comment
	return nil
}`,
			expected: `package main

func process() error {
	x := 42 // normal comment
	return nil
}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RemoveDebugMarkers([]byte(tt.input))
			resultStr := string(result)

			if resultStr != tt.expected {
				t.Errorf("RemoveDebugMarkers() mismatch\nGot:\n%s\n\nExpected:\n%s", resultStr, tt.expected)
			}
		})
	}
}
