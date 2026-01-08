package builtin

import (
	"go/parser"
	"go/printer"
	"go/token"
	"strings"
	"testing"

	"github.com/MadAppGang/dingo/pkg/plugin"
)

func TestImmutabilityPlugin_MarkerCleanup(t *testing.T) {
	tests := []struct {
		name              string
		input             string
		wantMarker        bool // Should marker appear in output?
		wantComment       string
		shouldHaveComment bool
	}{
		{
			name: "marker only - should be removed",
			input: `package main
func main() {
	x := 42 // dingo:let:x
}`,
			wantMarker:        false,
			shouldHaveComment: false,
		},
		{
			name: "marker with additional comment - keep comment",
			input: `package main
func main() {
	x := 42 // dingo:let:x important config value
}`,
			wantMarker:        false,
			wantComment:       "important config value",
			shouldHaveComment: true,
		},
		{
			name: "multiple variables in marker",
			input: `package main
func main() {
	a, b := 1, 2 // dingo:let:a,b
}`,
			wantMarker:        false,
			shouldHaveComment: false,
		},
		{
			name: "non-marker comment - should be preserved",
			input: `package main
func main() {
	x := 42 // regular comment
}`,
			wantMarker:        false,
			wantComment:       "regular comment",
			shouldHaveComment: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse input
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "test.go", tt.input, parser.ParseComments)
			if err != nil {
				t.Fatalf("Failed to parse input: %v", err)
			}

			// Create plugin and process
			p := NewImmutabilityPlugin()
			ctx := &plugin.Context{}
			p.SetContext(ctx)

			err = p.Process(file)
			if err != nil {
				t.Fatalf("Process failed: %v", err)
			}

			// Generate output
			var buf strings.Builder
			err = printer.Fprint(&buf, fset, file)
			if err != nil {
				t.Fatalf("Failed to print AST: %v", err)
			}
			output := buf.String()

			// Check for marker presence
			hasMarker := strings.Contains(output, "dingo:let:")
			if hasMarker != tt.wantMarker {
				t.Errorf("Marker presence = %v, want %v\nOutput:\n%s", hasMarker, tt.wantMarker, output)
			}

			// Check for comment presence
			if tt.shouldHaveComment {
				if !strings.Contains(output, tt.wantComment) {
					t.Errorf("Expected comment %q not found in output:\n%s", tt.wantComment, output)
				}
			} else {
				// Verify no comments remain
				if strings.Contains(output, "//") {
					t.Errorf("Expected no comments, but found some in output:\n%s", output)
				}
			}
		})
	}
}

func TestImmutabilityPlugin_MarkerCleanup_PreservesNormalComments(t *testing.T) {
	input := `package main
// This is a package comment
func main() {
	// This is a function comment
	x := 42 // dingo:let:x
	y := 10 // This should stay
}`

	// Parse input
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", input, parser.ParseComments)
	if err != nil {
		t.Fatalf("Failed to parse input: %v", err)
	}

	// Create plugin and process
	p := NewImmutabilityPlugin()
	ctx := &plugin.Context{}
	p.SetContext(ctx)

	err = p.Process(file)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	// Generate output
	var buf strings.Builder
	err = printer.Fprint(&buf, fset, file)
	if err != nil {
		t.Fatalf("Failed to print AST: %v", err)
	}
	output := buf.String()

	// Marker should be removed
	if strings.Contains(output, "dingo:let:") {
		t.Errorf("Marker should be removed from output:\n%s", output)
	}

	// Normal comments should be preserved
	if !strings.Contains(output, "This is a package comment") {
		t.Errorf("Package comment should be preserved")
	}
	if !strings.Contains(output, "This is a function comment") {
		t.Errorf("Function comment should be preserved")
	}
	if !strings.Contains(output, "This should stay") {
		t.Errorf("Regular inline comment should be preserved")
	}
}
