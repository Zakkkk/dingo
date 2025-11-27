package preprocessor

import (
	"strings"
	"testing"
)

func TestSafeNavProcessor_Metadata(t *testing.T) {
	p := NewSafeNavProcessor()

	code := `let user: *User = getUser()
let result = user?.name`

	result, metadata, err := p.ProcessInternal(code)
	if err != nil {
		t.Fatalf("ProcessV2 failed: %v", err)
	}

	// Should generate metadata
	if len(metadata) != 1 {
		t.Fatalf("Expected 1 metadata entry, got %d", len(metadata))
	}

	meta := metadata[0]
	if meta.Type != "safe_nav" {
		t.Errorf("Expected type 'safe_nav', got '%s'", meta.Type)
	}

	if meta.OriginalText != "?." {
		t.Errorf("Expected original text '?.', got '%s'", meta.OriginalText)
	}

	if meta.GeneratedMarker != "// dingo:s:0" {
		t.Errorf("Expected marker '// dingo:s:0', got '%s'", meta.GeneratedMarker)
	}

	if meta.ASTNodeType != "CallExpr" {
		t.Errorf("Expected AST node type 'CallExpr', got '%s'", meta.ASTNodeType)
	}

	// Should include marker in generated code
	if !strings.Contains(result, "// dingo:s:0") {
		t.Errorf("Expected marker '// dingo:s:0' in generated code, got:\n%s", result)
	}
}

func TestSafeNavProcessor_UniqueMarkers(t *testing.T) {
	p := NewSafeNavProcessor()

	code := `let user: *User = getUser()
let address: *Address = getAddress()
let name = user?.name
let city = address?.city
let zip = address?.zip`

	result, metadata, err := p.ProcessInternal(code)
	if err != nil {
		t.Fatalf("ProcessV2 failed: %v", err)
	}

	// Should generate 3 metadata entries (one per line with ?.)
	if len(metadata) != 3 {
		t.Fatalf("Expected 3 metadata entries, got %d", len(metadata))
	}

	// Check markers are unique
	markers := make(map[string]bool)
	for i, meta := range metadata {
		expectedMarker := "// dingo:s:" + string(rune('0'+i))
		if meta.GeneratedMarker != expectedMarker {
			t.Errorf("Metadata %d: Expected marker '%s', got '%s'", i, expectedMarker, meta.GeneratedMarker)
		}
		if markers[meta.GeneratedMarker] {
			t.Errorf("Duplicate marker: %s", meta.GeneratedMarker)
		}
		markers[meta.GeneratedMarker] = true

		// Verify marker is in output
		if !strings.Contains(result, meta.GeneratedMarker) {
			t.Errorf("Marker '%s' not found in output", meta.GeneratedMarker)
		}
	}
}


