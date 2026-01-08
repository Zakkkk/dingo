package lsp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/MadAppGang/dingo/pkg/sourcemap"
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
func createMockReader(t *testing.T, dingoSrc, goSrc string, mappings []sourcemap.LineMapping) *dmap.Reader {
	return createMockReaderWithColumns(t, dingoSrc, goSrc, mappings, nil)
}

// createMockReaderWithColumns creates a dmap.Reader with test data including column mappings
func createMockReaderWithColumns(t *testing.T, dingoSrc, goSrc string, mappings []sourcemap.LineMapping, columnMappings []sourcemap.ColumnMapping) *dmap.Reader {
	t.Helper()
	writer := dmap.NewWriter([]byte(dingoSrc), []byte(goSrc))
	data, err := writer.Write(mappings, columnMappings)
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

	mappings := []sourcemap.LineMapping{
		{DingoLine: 1, GoLineStart: 1, GoLineEnd: 1, Kind: "identity"},
		{DingoLine: 2, GoLineStart: 2, GoLineEnd: 2, Kind: "identity"},
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

	mappings := []sourcemap.LineMapping{
		{DingoLine: 1, GoLineStart: 1, GoLineEnd: 1, Kind: "identity"},
		{DingoLine: 2, GoLineStart: 2, GoLineEnd: 2, Kind: "identity"},
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

	mappings := []sourcemap.LineMapping{
		{DingoLine: 1, GoLineStart: 1, GoLineEnd: 1, Kind: "identity"},
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

	mappings := []sourcemap.LineMapping{
		{DingoLine: 1, GoLineStart: 1, GoLineEnd: 1, Kind: "identity"},
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
	mappings := []sourcemap.LineMapping{
		{DingoLine: 3, GoLineStart: 3, GoLineEnd: 3, Kind: "identity"},
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

// TestTranslatePositionErrorPropagationColumn tests column mapping for error propagation transforms.
// This is a REGRESSION TEST for the issue where hovering on `extractUserID` in Dingo
// shows hover info for `tmp` instead, because column positions differ:
//   - Dingo: `userID := extractUserID(r)?`  → extractUserID at col 12
//   - Go:    `tmp, err := extractUserID(r)` → extractUserID at col 14
//
// The test verifies that column positions are correctly mapped for transformed lines.
func TestTranslatePositionErrorPropagationColumn(t *testing.T) {
	// Realistic error propagation example
	// Dingo line: "	userID := extractUserID(r)?\n"
	// Go line:    "	tmp, err := extractUserID(r)\n"
	//
	// Character positions (1-indexed):
	//   Dingo: extractUserID starts at col 12 (after tab + "userID := ")
	//   Go:    extractUserID starts at col 14 (after tab + "tmp, err := ")
	dingoSrc := "\tuserID := extractUserID(r)?\n"
	goSrc := "\ttmp, err := extractUserID(r)\n"

	// Error propagation expands 1 dingo line to 5 go lines typically,
	// but for this test we simplify to just the first line mapping
	mappings := []sourcemap.LineMapping{
		{DingoLine: 1, GoLineStart: 1, GoLineEnd: 1, Kind: "error_prop"},
	}

	// Column mapping for the function call expression
	// Dingo: "\tuserID := extractUserID(r)?" - extractUserID at col 12 (1-indexed)
	// Go:    "\ttmp, err := extractUserID(r)" - extractUserID at col 14 (1-indexed)
	// Offset = 14 - 12 = +2 (Go has 2 more leading chars due to "tmp, err" vs "userID")
	columnMappings := []sourcemap.ColumnMapping{
		{
			DingoLine: 1,
			DingoCol:  12, // 1-indexed position of 'e' in extractUserID
			GoLine:    1,
			GoCol:     14, // 1-indexed position of 'e' in extractUserID
			Length:    13, // length of "extractUserID"
			Kind:      "error_prop",
		},
	}

	reader := createMockReaderWithColumns(t, dingoSrc, goSrc, mappings, columnMappings)
	defer reader.Close()

	goPath := "/test/file.go"
	dingoPath := "/test/file.dingo"

	cache := &mockSourceMapGetter{
		readers: map[string]*dmap.Reader{
			goPath: reader,
		},
	}

	translator := NewTranslator(cache)
	dingoURI := lspuri.File(dingoPath)

	// Test: Position on 'extractUserID' in Dingo (col 12, 0-indexed = 11)
	// In Dingo: "\tuserID := extractUserID(r)?"
	//            ^          ^
	//            0          11 (0-indexed)
	dingoPos := protocol.Position{Line: 0, Character: 11}

	newURI, newPos, err := translator.TranslatePosition(dingoURI, dingoPos, DingoToGo)
	if err != nil {
		t.Fatalf("TranslatePosition failed: %v", err)
	}

	// Should return Go URI
	expectedURI := lspuri.File(goPath)
	if newURI != expectedURI {
		t.Errorf("URI: got %s, want %s", newURI, expectedURI)
	}

	// Line should be correct (line 0)
	if newPos.Line != 0 {
		t.Errorf("Line: got %d, want 0", newPos.Line)
	}

	// CRITICAL: Column should map to 'extractUserID' in Go, not to 'tmp' or '='
	// In Go: "\ttmp, err := extractUserID(r)"
	//         ^            ^
	//         0            13 (0-indexed)
	//
	// Currently FAILS because column is passed through as-is (11 → 11)
	// which points to '=' in Go, not to 'extractUserID'
	//
	// Expected: col 13 (0-indexed) = start of 'extractUserID' in Go
	expectedCol := uint32(13) // 0-indexed position of 'e' in extractUserID

	if newPos.Character != expectedCol {
		t.Errorf("Column: got %d, want %d (extractUserID position)", newPos.Character, expectedCol)
		t.Logf("This test verifies column mapping for error propagation transforms.")
		t.Logf("Dingo: '\\tuserID := extractUserID(r)?' - 'extractUserID' at col 11 (0-indexed)")
		t.Logf("Go:    '\\ttmp, err := extractUserID(r)' - 'extractUserID' at col 13 (0-indexed)")
		t.Logf("Without proper column mapping, hovering on extractUserID shows info for 'tmp' or '='")
	}
}

// TestTranslatePositionErrorPropagationColumnMultipleIdentifiers tests column mapping
// when there are multiple identifiers on a transformed line.
func TestTranslatePositionErrorPropagationColumnMultipleIdentifiers(t *testing.T) {
	// Dingo: "	_ := checkPermissions(r, user) ? |err| NewAppError(403, "denied", err)\n"
	// Go:    "	tmp2, err2 := checkPermissions(r, user)\n"
	//
	// When hovering on 'checkPermissions' in Dingo, we should get 'checkPermissions' in Go,
	// not 'tmp2' or 'err2'
	dingoSrc := "\t_ := checkPermissions(r, user) ? |err| NewAppError(403, \"denied\", err)\n"
	goSrc := "\ttmp2, err2 := checkPermissions(r, user)\n"

	mappings := []sourcemap.LineMapping{
		{DingoLine: 1, GoLineStart: 1, GoLineEnd: 1, Kind: "error_prop"},
	}

	// Column mapping for the function call expression
	// Dingo: "\t_ := checkPermissions..." - checkPermissions at col 7 (1-indexed)
	// Go:    "\ttmp2, err2 := checkPermissions..." - checkPermissions at col 16 (1-indexed)
	// Offset = 16 - 7 = +9 (Go has 9 more leading chars)
	columnMappings := []sourcemap.ColumnMapping{
		{
			DingoLine: 1,
			DingoCol:  7, // 1-indexed position of 'c' in checkPermissions
			GoLine:    1,
			GoCol:     16, // 1-indexed position of 'c' in checkPermissions
			Length:    16, // length of "checkPermissions"
			Kind:      "error_prop",
		},
	}

	reader := createMockReaderWithColumns(t, dingoSrc, goSrc, mappings, columnMappings)
	defer reader.Close()

	goPath := "/test/file.go"
	dingoPath := "/test/file.dingo"

	cache := &mockSourceMapGetter{
		readers: map[string]*dmap.Reader{
			goPath: reader,
		},
	}

	translator := NewTranslator(cache)
	dingoURI := lspuri.File(dingoPath)

	// In Dingo: "\t_ := checkPermissions(r, user) ? |err| NewAppError(403, \"denied\", err)"
	//            ^    ^
	//            0    6 (0-indexed, start of 'checkPermissions')
	dingoPos := protocol.Position{Line: 0, Character: 6}

	newURI, newPos, err := translator.TranslatePosition(dingoURI, dingoPos, DingoToGo)
	if err != nil {
		t.Fatalf("TranslatePosition failed: %v", err)
	}

	expectedURI := lspuri.File(goPath)
	if newURI != expectedURI {
		t.Errorf("URI: got %s, want %s", newURI, expectedURI)
	}

	// In Go: "\ttmp2, err2 := checkPermissions(r, user)"
	//         ^              ^
	//         0              15 (0-indexed, start of 'checkPermissions')
	expectedCol := uint32(15)

	if newPos.Character != expectedCol {
		t.Errorf("Column: got %d, want %d (checkPermissions position)", newPos.Character, expectedCol)
		t.Logf("Dingo: 'checkPermissions' at col 6 (0-indexed)")
		t.Logf("Go:    'checkPermissions' at col 15 (0-indexed)")
	}
}

// TestTranslatePositionSecondErrorPropagation tests that hovering on a function call
// in the SECOND error propagation transform works correctly.
// REGRESSION TEST for issue: hovering on loadUserFromDB showed fmt.Errorf info
// because DingoLineToGoLine returned the wrong Go line (line with fmt.Errorf, not loadUserFromDB)
func TestTranslatePositionSecondErrorPropagation(t *testing.T) {
	// Simulate two consecutive error propagation transforms
	// Line 1: first transform (extractUserID)
	// Line 2: comment
	// Line 3: second transform (loadUserFromDB) - THIS IS THE REGRESSION
	dingoSrc := "\tuserID := extractUserID(r)?\n// comment\n\tuser := loadUserFromDB(userID) ? \"db error\"\n"

	// Go output after transformation:
	// Line 1: tmp, err := extractUserID(r)
	// Line 2: if err != nil {
	// Line 3: return err
	// Line 4: }
	// Line 5: userID := tmp
	// Line 6: // comment
	// Line 7: //line directive
	// Line 8: tmp1, err1 := loadUserFromDB(userID)  <- loadUserFromDB is HERE
	// Line 9: if err1 != nil {
	// Line 10: return fmt.Errorf("db error: %w", err1)  <- NOT here!
	// Line 11: }
	// Line 12: user := tmp1
	goSrc := "\ttmp, err := extractUserID(r)\n\tif err != nil {\n\t\treturn err\n\t}\n\tuserID := tmp\n// comment\n//line test.dingo:3:2\n\ttmp1, err1 := loadUserFromDB(userID)\n\tif err1 != nil {\n\t\treturn fmt.Errorf(\"db error: %w\", err1)\n\t}\n\tuser := tmp1\n"

	// Line mappings: first transform spans Go lines 1-5, second spans Go lines 8-12
	// CRITICAL: GoLineStart for the second transform should be 8 (where loadUserFromDB is),
	// NOT 10 (where fmt.Errorf is)
	mappings := []sourcemap.LineMapping{
		{DingoLine: 1, GoLineStart: 1, GoLineEnd: 5, Kind: "error_prop"},
		{DingoLine: 3, GoLineStart: 8, GoLineEnd: 12, Kind: "error_prop"}, // GoLineStart=8, not 10!
	}

	// Column mapping for the second transform
	// Dingo line 3: "\tuser := loadUserFromDB(userID) ? \"db error\""
	//               ^        ^
	//               1        10 (1-indexed, 'l' in loadUserFromDB)
	// Go line 8: "\ttmp1, err1 := loadUserFromDB(userID)"
	//            ^              ^
	//            1              16 (1-indexed, 'l' in loadUserFromDB)
	columnMappings := []sourcemap.ColumnMapping{
		{
			DingoLine: 3,
			DingoCol:  10, // 1-indexed
			GoLine:    8,  // MUST be the actual Go line with the function call
			GoCol:     16, // 1-indexed
			Length:    14, // "loadUserFromDB"
			Kind:      "error_prop",
		},
	}

	reader := createMockReaderWithColumns(t, dingoSrc, goSrc, mappings, columnMappings)
	defer reader.Close()

	goPath := "/test/file.go"
	dingoPath := "/test/file.dingo"

	cache := &mockSourceMapGetter{
		readers: map[string]*dmap.Reader{
			goPath: reader,
		},
	}

	translator := NewTranslator(cache)
	dingoURI := lspuri.File(dingoPath)

	// Test: hovering on 'loadUserFromDB' in Dingo line 3 (0-indexed = 2), col 9 (0-indexed)
	dingoPos := protocol.Position{Line: 2, Character: 9}

	newURI, newPos, err := translator.TranslatePosition(dingoURI, dingoPos, DingoToGo)
	if err != nil {
		t.Fatalf("TranslatePosition failed: %v", err)
	}

	expectedURI := lspuri.File(goPath)
	if newURI != expectedURI {
		t.Errorf("URI: got %s, want %s", newURI, expectedURI)
	}

	// CRITICAL: Line should be 7 (0-indexed = Go line 8 where loadUserFromDB is)
	// NOT line 9 (0-indexed = Go line 10 where fmt.Errorf is)
	expectedLine := uint32(7) // 0-indexed Go line 8
	if newPos.Line != expectedLine {
		t.Errorf("Line: got %d, want %d", newPos.Line, expectedLine)
		t.Logf("REGRESSION: If line is 9, it points to fmt.Errorf instead of loadUserFromDB")
	}

	// Column should be 15 (0-indexed = col 16 where 'l' in loadUserFromDB is)
	expectedCol := uint32(15)
	if newPos.Character != expectedCol {
		t.Errorf("Column: got %d, want %d (loadUserFromDB position)", newPos.Character, expectedCol)
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
	mappings := []sourcemap.LineMapping{
		{DingoLine: 1, GoLineStart: 1, GoLineEnd: 1, Kind: "identity"},
	}

	writer := dmap.NewWriter([]byte(dingoSrc), []byte(goSrc))
	data, err := writer.Write(mappings, nil)
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
