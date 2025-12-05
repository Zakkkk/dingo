package ast

import (
	"testing"
)

func TestLetCodeGen_Generate(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple let with inference",
			input:    "let x = 5",
			expected: "x := 5",
		},
		{
			name:     "let with type annotation",
			input:    "let x: int = 5",
			expected: "var x int = 5",
		},
		{
			name:     "multiple names from function",
			input:    "let a, b = getValues()",
			expected: "a, b := getValues()",
		},
		{
			name:     "tuple destructuring",
			input:    "let (a, b) = tuple",
			expected: "tmp := tuple; a, b := tmp.__0, tmp.__1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse the let declaration
			parser := NewLetParser([]byte(tt.input), 0)
			decl, _, err := parser.ParseLetDecl()
			if err != nil {
				t.Fatalf("ParseLetDecl() error = %v", err)
			}

			// Generate Go code
			codegen := NewLetCodeGen()
			output := codegen.Generate(decl)

			if string(output) != tt.expected {
				t.Errorf("Generate() = %q, want %q", string(output), tt.expected)
			}
		})
	}
}

func TestFindLetDeclarations(t *testing.T) {
	src := []byte(`package main

func main() {
	let x = 5
	let y: int = 10
	println(x + y)
	// let z = 3 (comment)
	"let x = 1" // string
	let a, b = getValues()
}`)

	positions := FindLetDeclarations(src)

	// Should find 3 let declarations (not the ones in comment or string)
	expected := 3
	if len(positions) != expected {
		t.Errorf("FindLetDeclarations() found %d positions, want %d", len(positions), expected)
		t.Logf("Positions: %v", positions)
	}
}

func TestTransformLetSource(t *testing.T) {
	src := []byte(`package main

func main() {
	let x = 5
	let y: int = 10
	let a, b = getValues()
}`)

	output, mappings := TransformLetSource(src)

	// Verify output contains transformed code
	outputStr := string(output)
	if outputStr == string(src) {
		t.Error("TransformLetSource() did not transform anything")
	}

	// Verify mappings were created
	if len(mappings) == 0 {
		t.Error("TransformLetSource() did not create any source mappings")
	}

	// Should have 3 mappings (one for each let)
	expected := 3
	if len(mappings) != expected {
		t.Errorf("TransformLetSource() created %d mappings, want %d", len(mappings), expected)
	}

	// Verify all mappings have "let" kind
	for i, mapping := range mappings {
		if mapping.Kind != "let" {
			t.Errorf("Mapping %d has kind %q, want %q", i, mapping.Kind, "let")
		}
	}
}
