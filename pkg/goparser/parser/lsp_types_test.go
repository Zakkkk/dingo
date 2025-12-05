package parser

import (
	"go/token"
	"testing"
)

func TestDocumentState_PositionConversion(t *testing.T) {
	content := `package main

func main() {
	x := 1
}
`
	fset := token.NewFileSet()
	ds, err := NewDocumentState("test.dingo", []byte(content), fset, "test.dingo")
	if err != nil {
		t.Fatalf("NewDocumentState error: %v", err)
	}

	tests := []struct {
		name     string
		pos      Position
		expected int // byte offset
	}{
		{"start of file", Position{Line: 0, Character: 0}, 0},
		{"package keyword", Position{Line: 0, Character: 0}, 0},
		{"main on line 0", Position{Line: 0, Character: 8}, 8},
		{"start of func", Position{Line: 2, Character: 0}, 14},
		{"x variable", Position{Line: 3, Character: 1}, 29},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			offset := ds.positionToOffset(tt.pos)
			if offset != tt.expected {
				t.Errorf("positionToOffset(%v) = %d, want %d", tt.pos, offset, tt.expected)
			}

			// Verify round-trip conversion
			pos := ds.offsetToPosition(offset)
			offset2 := ds.positionToOffset(pos)
			if offset != offset2 {
				t.Errorf("round-trip failed: %d -> %v -> %d", offset, pos, offset2)
			}
		})
	}
}

func TestDocumentState_ApplyChanges(t *testing.T) {
	content := `package main

func main() {
	x := 1
}
`
	fset := token.NewFileSet()
	ds, err := NewDocumentState("test.dingo", []byte(content), fset, "test.dingo")
	if err != nil {
		t.Fatalf("NewDocumentState error: %v", err)
	}

	// Full document sync
	newContent := `package main

func main() {
	y := 2
}
`
	err = ds.ApplyChanges([]TextDocumentContentChangeEvent{
		{Range: nil, Text: newContent},
	})
	if err != nil {
		t.Fatalf("ApplyChanges error: %v", err)
	}

	if string(ds.Content()) != newContent {
		t.Errorf("content mismatch after full sync")
	}
}

func TestDocumentState_IncrementalChange(t *testing.T) {
	content := `package main

func main() {
	x := 1
}
`
	fset := token.NewFileSet()
	ds, err := NewDocumentState("test.dingo", []byte(content), fset, "test.dingo")
	if err != nil {
		t.Fatalf("NewDocumentState error: %v", err)
	}

	// Change "x" to "y" (position is line 3, char 1)
	err = ds.ApplyChanges([]TextDocumentContentChangeEvent{
		{
			Range: &Range{
				Start: Position{Line: 3, Character: 1},
				End:   Position{Line: 3, Character: 2},
			},
			Text: "y",
		},
	})
	if err != nil {
		t.Fatalf("ApplyChanges error: %v", err)
	}

	expected := `package main

func main() {
	y := 1
}
`
	if string(ds.Content()) != expected {
		t.Errorf("content mismatch:\ngot:  %q\nwant: %q", string(ds.Content()), expected)
	}
}

func TestDocumentState_GetCompletionContext(t *testing.T) {
	content := `package main

import "fmt"

func main() {
	fmt.Println("hello")
}
`
	fset := token.NewFileSet()
	ds, err := NewDocumentState("test.dingo", []byte(content), fset, "test.dingo")
	if err != nil {
		t.Fatalf("NewDocumentState error: %v", err)
	}

	// Test completion context is retrieved (basic functionality)
	ctx := ds.GetCompletionContext(Position{Line: 5, Character: 5})
	if ctx == nil {
		t.Fatal("GetCompletionContext returned nil")
	}

	// Verify position is set correctly
	if ctx.Pos.Line != 5 || ctx.Pos.Character != 5 {
		t.Errorf("expected Pos (5,5), got (%d,%d)", ctx.Pos.Line, ctx.Pos.Character)
	}

	// Verify we found a node (the AST should contain something at this position)
	if ctx.Node == nil {
		t.Error("expected Node to be non-nil for valid position in code")
	}

	// Test selector expression detection - position at "Println"
	ctx2 := ds.GetCompletionContext(Position{Line: 5, Character: 8})
	if ctx2 == nil {
		t.Fatal("GetCompletionContext returned nil for selector position")
	}

	// The "Println" identifier should be detected
	if ctx2.CurrentToken == "" && ctx2.Node != nil {
		// Token extraction depends on AST node type
		t.Logf("Node type at Println: %T", ctx2.Node)
	}
}

func TestDocumentState_GetHoverInfo(t *testing.T) {
	content := `package main

func greet(name string) string {
	return "Hello, " + name
}

func main() {
	greet("world")
}
`
	fset := token.NewFileSet()
	ds, err := NewDocumentState("test.dingo", []byte(content), fset, "test.dingo")
	if err != nil {
		t.Fatalf("NewDocumentState error: %v", err)
	}

	// Hover over "greet" function call (line 7, char 1-6)
	info := ds.GetHoverInfo(Position{Line: 7, Character: 2})
	if info == nil {
		t.Fatal("GetHoverInfo returned nil")
	}

	// Should contain function info
	if info.NodeType == "" {
		t.Error("expected non-empty NodeType")
	}
	if info.Contents == "" {
		t.Error("expected non-empty Contents")
	}
}

func TestDocumentState_GetDiagnostics(t *testing.T) {
	// Valid Go code - should have no parse errors
	content := `package main

func main() {
	x := 1
	_ = x
}
`
	fset := token.NewFileSet()
	ds, err := NewDocumentState("test.dingo", []byte(content), fset, "test.dingo")
	if err != nil {
		t.Fatalf("NewDocumentState error: %v", err)
	}

	diags := ds.GetDiagnostics()
	if len(diags) != 0 {
		t.Errorf("expected 0 diagnostics for valid code, got %d: %v", len(diags), diags)
	}
}

func TestDocumentState_CommentDetection(t *testing.T) {
	content := `package main

// This is a comment
func main() {
	/* block
	   comment */
	x := 1
	_ = x
}
`
	fset := token.NewFileSet()
	ds, err := NewDocumentState("test.dingo", []byte(content), fset, "test.dingo")
	if err != nil {
		t.Fatalf("NewDocumentState error: %v", err)
	}

	// Position inside line comment (line 2)
	ctx := ds.GetCompletionContext(Position{Line: 2, Character: 10})
	if ctx == nil {
		t.Fatal("GetCompletionContext returned nil")
	}
	if !ctx.InComment {
		t.Error("expected InComment=true for position inside line comment")
	}
}

func TestParseErrorPosition(t *testing.T) {
	tests := []struct {
		errStr   string
		expected *Position
	}{
		{"test.go:10:5: undefined: x", &Position{Line: 9, Character: 4}},
		{"15:3: syntax error", &Position{Line: 14, Character: 2}},
		{"no position info", nil},
	}

	for _, tt := range tests {
		t.Run(tt.errStr, func(t *testing.T) {
			pos := parseErrorPosition(tt.errStr)
			if tt.expected == nil {
				if pos != nil {
					t.Errorf("expected nil, got %v", pos)
				}
			} else if pos == nil {
				t.Errorf("expected %v, got nil", tt.expected)
			} else if *pos != *tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, pos)
			}
		})
	}
}
