package lsp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/MadAppGang/dingo/pkg/ast"
	"github.com/MadAppGang/dingo/pkg/sourcemap/dmap"
	"go.lsp.dev/protocol"
	lspuri "go.lsp.dev/uri"
)

// mockSourceMapGetter implements SourceMapGetter for testing
type mockSourceMapGetter struct {
	readers map[string]*dmap.Reader
	err     error
}

func (m *mockSourceMapGetter) Get(goFilePath string) (*dmap.Reader, error) {
	if m.err != nil {
		return nil, m.err
	}
	if reader, ok := m.readers[goFilePath]; ok {
		return reader, nil
	}
	return nil, os.ErrNotExist
}

func (m *mockSourceMapGetter) Invalidate(goFilePath string) {}
func (m *mockSourceMapGetter) InvalidateAll()               {}
func (m *mockSourceMapGetter) Size() int                    { return len(m.readers) }

// createMockReader creates a dmap.Reader with test data
func createMockReader(t *testing.T, dingoSrc, goSrc string, mappings []ast.SourceMapping) *dmap.Reader {
	t.Helper()
	writer := dmap.NewWriter([]byte(dingoSrc), []byte(goSrc))
	data, err := writer.Write(mappings)
	if err != nil {
		t.Fatalf("Failed to create dmap: %v", err)
	}
	reader, err := dmap.OpenBytes(data)
	if err != nil {
		t.Fatalf("Failed to open dmap bytes: %v", err)
	}
	return reader
}

func TestNewTranslator(t *testing.T) {
	cache := &mockSourceMapGetter{}
	translator := NewTranslator(cache)
	if translator == nil {
		t.Fatal("Expected non-nil translator")
	}
}

func TestTranslatePositionDingoToGo(t *testing.T) {
	// Create source files
	// Dingo:
	//   Line 1: "x := 10\n"  (bytes 0-7)
	//   Line 2: "y := 20\n"  (bytes 8-15)
	// Go:
	//   Line 1: "x := 10\n"  (bytes 0-7)
	//   Line 2: "y := 20\n"  (bytes 8-15)
	dingoSrc := "x := 10\ny := 20\n"
	goSrc := "x := 10\ny := 20\n"

	mappings := []ast.SourceMapping{
		{DingoStart: 0, DingoEnd: 7, GoStart: 0, GoEnd: 7, Kind: "identity"},
		{DingoStart: 8, DingoEnd: 15, GoStart: 8, GoEnd: 15, Kind: "identity"},
	}

	reader := createMockReader(t, dingoSrc, goSrc, mappings)
	defer reader.Close()

	goPath := "/test/file.go"
	dingoPath := "/test/file.dingo"

	cache := &mockSourceMapGetter{
		readers: map[string]*dmap.Reader{
			goPath: reader,
		},
	}

	translator := NewTranslator(cache)

	// Test translating first line from Dingo to Go
	// Position in Dingo: line 1 (0-indexed=0), column 5 (0-indexed)
	dingoURI := lspuri.File(dingoPath)
	dingoPos := protocol.Position{Line: 0, Character: 5}

	newURI, newPos, err := translator.TranslatePosition(dingoURI, dingoPos, DingoToGo)
	if err != nil {
		t.Fatalf("TranslatePosition failed: %v", err)
	}

	// Should return Go URI
	expectedURI := lspuri.File(goPath)
	if newURI != expectedURI {
		t.Errorf("URI: got %s, want %s", newURI, expectedURI)
	}

	// Position should be within first line (mapped range)
	if newPos.Line != 0 {
		t.Errorf("Line: got %d, want 0", newPos.Line)
	}
}

func TestTranslatePositionGoToDingo(t *testing.T) {
	dingoSrc := "x := 10\ny := 20\n"
	goSrc := "x := 10\ny := 20\n"

	mappings := []ast.SourceMapping{
		{DingoStart: 0, DingoEnd: 7, GoStart: 0, GoEnd: 7, Kind: "identity"},
		{DingoStart: 8, DingoEnd: 15, GoStart: 8, GoEnd: 15, Kind: "identity"},
	}

	reader := createMockReader(t, dingoSrc, goSrc, mappings)
	defer reader.Close()

	goPath := "/test/file.go"
	dingoPath := "/test/file.dingo"

	cache := &mockSourceMapGetter{
		readers: map[string]*dmap.Reader{
			goPath: reader,
		},
	}

	translator := NewTranslator(cache)

	// Test translating from Go to Dingo
	// Position in Go: line 1 (0-indexed=0), column 3 (0-indexed)
	goURI := lspuri.File(goPath)
	goPos := protocol.Position{Line: 0, Character: 3}

	newURI, newPos, err := translator.TranslatePosition(goURI, goPos, GoToDingo)
	if err != nil {
		t.Fatalf("TranslatePosition failed: %v", err)
	}

	// Should return Dingo URI
	expectedURI := lspuri.File(dingoPath)
	if newURI != expectedURI {
		t.Errorf("URI: got %s, want %s", newURI, expectedURI)
	}

	// Position should be in first line
	if newPos.Line != 0 {
		t.Errorf("Line: got %d, want 0", newPos.Line)
	}
}

func TestTranslatePositionNoSourceMap(t *testing.T) {
	cache := &mockSourceMapGetter{
		readers: map[string]*dmap.Reader{},
	}

	translator := NewTranslator(cache)

	// For GoToDingo without source map, should return unchanged position
	goPath := "/test/file.go"
	goURI := lspuri.File(goPath)
	goPos := protocol.Position{Line: 5, Character: 10}

	newURI, newPos, err := translator.TranslatePosition(goURI, goPos, GoToDingo)

	// Should NOT error - returns identity mapping for non-transpiled files
	if err != nil {
		t.Fatalf("TranslatePosition should not error for GoToDingo without map: %v", err)
	}

	// Should return same URI and position
	if newURI != goURI {
		t.Errorf("URI should be unchanged: got %s, want %s", newURI, goURI)
	}
	if newPos.Line != goPos.Line || newPos.Character != goPos.Character {
		t.Errorf("Position should be unchanged: got (%d,%d), want (%d,%d)",
			newPos.Line, newPos.Character, goPos.Line, goPos.Character)
	}
}

func TestTranslatePositionDingoToGoNoSourceMap(t *testing.T) {
	cache := &mockSourceMapGetter{
		readers: map[string]*dmap.Reader{},
	}

	translator := NewTranslator(cache)

	// For DingoToGo without source map, should return error but with Go URI
	dingoPath := "/test/file.dingo"
	dingoURI := lspuri.File(dingoPath)
	dingoPos := protocol.Position{Line: 5, Character: 10}

	newURI, newPos, err := translator.TranslatePosition(dingoURI, dingoPos, DingoToGo)

	// Should error (source map not found)
	if err == nil {
		t.Fatal("TranslatePosition should error for DingoToGo without map")
	}

	// But should return Go URI (CRITICAL FIX C6)
	goPath := "/test/file.go"
	expectedURI := lspuri.File(goPath)
	if newURI != expectedURI {
		t.Errorf("URI should be .go path: got %s, want %s", newURI, expectedURI)
	}

	// Position should be identity
	if newPos.Line != dingoPos.Line || newPos.Character != dingoPos.Character {
		t.Errorf("Position should be identity: got (%d,%d), want (%d,%d)",
			newPos.Line, newPos.Character, dingoPos.Line, dingoPos.Character)
	}
}

func TestTranslateRange(t *testing.T) {
	dingoSrc := "x := 10\ny := 20\n"
	goSrc := "x := 10\ny := 20\n"

	mappings := []ast.SourceMapping{
		{DingoStart: 0, DingoEnd: 7, GoStart: 0, GoEnd: 7, Kind: "identity"},
	}

	reader := createMockReader(t, dingoSrc, goSrc, mappings)
	defer reader.Close()

	goPath := "/test/file.go"

	cache := &mockSourceMapGetter{
		readers: map[string]*dmap.Reader{
			goPath: reader,
		},
	}

	translator := NewTranslator(cache)

	// Test translating a range
	goURI := lspuri.File(goPath)
	goRange := protocol.Range{
		Start: protocol.Position{Line: 0, Character: 0},
		End:   protocol.Position{Line: 0, Character: 5},
	}

	newURI, newRange, err := translator.TranslateRange(goURI, goRange, GoToDingo)
	if err != nil {
		t.Fatalf("TranslateRange failed: %v", err)
	}

	dingoPath := "/test/file.dingo"
	expectedURI := lspuri.File(dingoPath)
	if newURI != expectedURI {
		t.Errorf("URI: got %s, want %s", newURI, expectedURI)
	}

	// Range should be valid
	if newRange.Start.Line > newRange.End.Line {
		t.Error("Invalid range: start line > end line")
	}
}

func TestTranslateLocation(t *testing.T) {
	dingoSrc := "x := 10\n"
	goSrc := "x := 10\n"

	mappings := []ast.SourceMapping{
		{DingoStart: 0, DingoEnd: 7, GoStart: 0, GoEnd: 7, Kind: "identity"},
	}

	reader := createMockReader(t, dingoSrc, goSrc, mappings)
	defer reader.Close()

	goPath := "/test/file.go"

	cache := &mockSourceMapGetter{
		readers: map[string]*dmap.Reader{
			goPath: reader,
		},
	}

	translator := NewTranslator(cache)

	goURI := lspuri.File(goPath)
	loc := protocol.Location{
		URI: goURI,
		Range: protocol.Range{
			Start: protocol.Position{Line: 0, Character: 0},
			End:   protocol.Position{Line: 0, Character: 3},
		},
	}

	newLoc, err := translator.TranslateLocation(loc, GoToDingo)
	if err != nil {
		t.Fatalf("TranslateLocation failed: %v", err)
	}

	dingoPath := "/test/file.dingo"
	expectedURI := lspuri.File(dingoPath)
	if newLoc.URI != expectedURI {
		t.Errorf("URI: got %s, want %s", newLoc.URI, expectedURI)
	}
}

func TestHelperFunctions(t *testing.T) {
	// Test isDingoFile
	tests := []struct {
		uri      protocol.DocumentURI
		expected bool
	}{
		{protocol.DocumentURI("file:///test/file.dingo"), true},
		{protocol.DocumentURI("file:///test/file.go"), false},
		{protocol.DocumentURI("file:///test/file.dingo.go"), false},
	}

	for _, tt := range tests {
		got := isDingoFile(tt.uri)
		if got != tt.expected {
			t.Errorf("isDingoFile(%s) = %v, want %v", tt.uri, got, tt.expected)
		}
	}
}

func TestPathConversion(t *testing.T) {
	// Test dingoToGoPath
	dingoTests := []struct {
		input    string
		expected string
	}{
		{"/test/file.dingo", "/test/file.go"},
		{"/test/file.go", "/test/file.go"},
		{"/test/file.dingo.bak", "/test/file.dingo.bak"},
	}

	for _, tt := range dingoTests {
		got := dingoToGoPath(tt.input)
		if got != tt.expected {
			t.Errorf("dingoToGoPath(%s) = %s, want %s", tt.input, got, tt.expected)
		}
	}

	// Test goToDingoPath
	goTests := []struct {
		input    string
		expected string
	}{
		{"/test/file.go", "/test/file.dingo"},
		{"/test/file.dingo", "/test/file.dingo"},
		{"/test/file.go.bak", "/test/file.go.bak"},
	}

	for _, tt := range goTests {
		got := goToDingoPath(tt.input)
		if got != tt.expected {
			t.Errorf("goToDingoPath(%s) = %s, want %s", tt.input, got, tt.expected)
		}
	}
}

func TestTranslatePositionIdentityMapping(t *testing.T) {
	// Test that positions outside of mapped ranges use identity mapping
	dingoSrc := "package main\n\nx := 10\n"
	goSrc := "package main\n\nx := 10\n"

	// Only map line 3
	mappings := []ast.SourceMapping{
		{DingoStart: 14, DingoEnd: 21, GoStart: 14, GoEnd: 21, Kind: "identity"},
	}

	reader := createMockReader(t, dingoSrc, goSrc, mappings)
	defer reader.Close()

	goPath := "/test/file.go"

	cache := &mockSourceMapGetter{
		readers: map[string]*dmap.Reader{
			goPath: reader,
		},
	}

	translator := NewTranslator(cache)

	// Position on line 1 (package declaration) - should use identity
	goURI := lspuri.File(goPath)
	goPos := protocol.Position{Line: 0, Character: 5}

	newURI, newPos, err := translator.TranslatePosition(goURI, goPos, GoToDingo)
	if err != nil {
		t.Fatalf("TranslatePosition failed: %v", err)
	}

	// Should return Dingo URI
	dingoPath := "/test/file.dingo"
	expectedURI := lspuri.File(dingoPath)
	if newURI != expectedURI {
		t.Errorf("URI: got %s, want %s", newURI, expectedURI)
	}

	// Position should be identity (same line/col)
	if newPos.Line != goPos.Line || newPos.Character != goPos.Character {
		t.Errorf("Position should be identity for unmapped region: got (%d,%d), want (%d,%d)",
			newPos.Line, newPos.Character, goPos.Line, goPos.Character)
	}
}

// Integration test with real workspace
func TestTranslatorWithRealWorkspace(t *testing.T) {
	// Create temp workspace
	tempDir, err := os.MkdirTemp("", "translator-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create go.mod
	if err := os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte("module test\n"), 0644); err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}

	// Create .dmap directory and file
	dmapDir := filepath.Join(tempDir, ".dmap")
	if err := os.MkdirAll(dmapDir, 0755); err != nil {
		t.Fatalf("Failed to create .dmap dir: %v", err)
	}

	dingoSrc := "x := 10\n"
	goSrc := "x := 10\n"
	mappings := []ast.SourceMapping{
		{DingoStart: 0, DingoEnd: 7, GoStart: 0, GoEnd: 7, Kind: "identity"},
	}

	writer := dmap.NewWriter([]byte(dingoSrc), []byte(goSrc))
	data, err := writer.Write(mappings)
	if err != nil {
		t.Fatalf("Failed to write dmap: %v", err)
	}

	dmapPath := filepath.Join(dmapDir, "test.dmap")
	if err := os.WriteFile(dmapPath, data, 0644); err != nil {
		t.Fatalf("Failed to write dmap file: %v", err)
	}

	// Create real cache and translator
	logger := &testLogger{}
	cache, err := NewSourceMapCache(logger)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	translator := NewTranslator(cache)

	// Test translation
	goPath := filepath.Join(tempDir, "test.go")
	goURI := lspuri.File(goPath)
	goPos := protocol.Position{Line: 0, Character: 3}

	newURI, newPos, err := translator.TranslatePosition(goURI, goPos, GoToDingo)
	if err != nil {
		t.Fatalf("TranslatePosition failed: %v", err)
	}

	// Should return Dingo URI
	dingoPath := filepath.Join(tempDir, "test.dingo")
	expectedURI := lspuri.File(dingoPath)
	if newURI != expectedURI {
		t.Errorf("URI: got %s, want %s", newURI, expectedURI)
	}

	// Position should be valid
	t.Logf("Translated Go(%d,%d) -> Dingo(%d,%d)", goPos.Line, goPos.Character, newPos.Line, newPos.Character)
}
