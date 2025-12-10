package dmap

import (
	"testing"

	"github.com/MadAppGang/dingo/pkg/ast"
)

// Helper function to create a .dmap file in memory for testing
func createTestDmap(t *testing.T, dingoSrc, goSrc []byte, mappings []ast.SourceMapping) []byte {
	t.Helper()
	writer := NewWriter(dingoSrc, goSrc)
	data, err := writer.Write(mappings)
	if err != nil {
		t.Fatalf("Failed to create test dmap: %v", err)
	}
	return data
}

func TestReaderOpenBytes(t *testing.T) {
	dingoSrc := []byte("let x = 10\nlet y = 20\n")
	goSrc := []byte("x := 10\ny := 20\n")

	mappings := []ast.SourceMapping{
		{DingoStart: 0, DingoEnd: 10, GoStart: 0, GoEnd: 7, Kind: "let_binding"},
		{DingoStart: 11, DingoEnd: 21, GoStart: 8, GoEnd: 15, Kind: "let_binding"},
	}

	data := createTestDmap(t, dingoSrc, goSrc, mappings)

	reader, err := OpenBytes(data)
	if err != nil {
		t.Fatalf("OpenBytes failed: %v", err)
	}
	defer reader.Close()

	if reader.EntryCount() != 2 {
		t.Errorf("EntryCount: got %d, want 2", reader.EntryCount())
	}

	if reader.KindCount() != 1 {
		t.Errorf("KindCount: got %d, want 1", reader.KindCount())
	}

	hdr := reader.Header()
	if hdr.DingoLen != uint32(len(dingoSrc)) {
		t.Errorf("Header.DingoLen: got %d, want %d", hdr.DingoLen, len(dingoSrc))
	}
	if hdr.GoLen != uint32(len(goSrc)) {
		t.Errorf("Header.GoLen: got %d, want %d", hdr.GoLen, len(goSrc))
	}
}

func TestReaderInvalidMagic(t *testing.T) {
	// Create data with invalid magic
	data := make([]byte, HeaderSize)
	copy(data[0:4], []byte("XXXX"))

	_, err := OpenBytes(data)
	if err != ErrInvalidMagic {
		t.Errorf("Expected ErrInvalidMagic, got %v", err)
	}
}

func TestReaderUnsupportedVersion(t *testing.T) {
	// Create valid header but with wrong version
	dingoSrc := []byte("test")
	goSrc := []byte("test")

	data := createTestDmap(t, dingoSrc, goSrc, nil)

	// Corrupt version field (bytes 4-5)
	data[4] = 0xFF
	data[5] = 0xFF

	_, err := OpenBytes(data)
	if err != ErrUnsupportedVer {
		t.Errorf("Expected ErrUnsupportedVer, got %v", err)
	}
}

func TestReaderTruncatedFile(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"empty", []byte{}},
		{"too short for header", make([]byte, HeaderSize-1)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := OpenBytes(tt.data)
			if err != ErrCorruptedFile {
				t.Errorf("Expected ErrCorruptedFile, got %v", err)
			}
		})
	}
}

func TestReaderFindByGoPos(t *testing.T) {
	dingoSrc := []byte("let x = 10\nlet y = 20\n")
	goSrc := []byte("x := 10\ny := 20\n")

	mappings := []ast.SourceMapping{
		{DingoStart: 0, DingoEnd: 10, GoStart: 0, GoEnd: 7, Kind: "let_binding"},
		{DingoStart: 11, DingoEnd: 21, GoStart: 8, GoEnd: 15, Kind: "let_binding"},
	}

	data := createTestDmap(t, dingoSrc, goSrc, mappings)
	reader, err := OpenBytes(data)
	if err != nil {
		t.Fatalf("OpenBytes failed: %v", err)
	}
	defer reader.Close()

	tests := []struct {
		name       string
		goOffset   int
		wantStart  int
		wantEnd    int
		wantKind   string
		wantFound  bool
	}{
		{"first mapping start", 0, 0, 10, "let_binding", true},
		{"first mapping middle", 3, 0, 10, "let_binding", true},
		{"first mapping end-1", 6, 0, 10, "let_binding", true},
		{"second mapping start", 8, 11, 21, "let_binding", true},
		{"second mapping middle", 10, 11, 21, "let_binding", true},
		{"gap between mappings", 7, 7, 7, "", false},
		{"after all mappings", 100, 100, 100, "", false},
		{"before all mappings (negative)", -1, -1, -1, "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dingoStart, dingoEnd, kind := reader.FindByGoPos(tt.goOffset)

			if tt.wantFound {
				if dingoStart != tt.wantStart || dingoEnd != tt.wantEnd || kind != tt.wantKind {
					t.Errorf("FindByGoPos(%d) = (%d, %d, %q), want (%d, %d, %q)",
						tt.goOffset, dingoStart, dingoEnd, kind,
						tt.wantStart, tt.wantEnd, tt.wantKind)
				}
			} else {
				// Identity mapping expected
				if dingoStart != tt.goOffset || dingoEnd != tt.goOffset || kind != "" {
					t.Errorf("FindByGoPos(%d) identity mapping: got (%d, %d, %q), want (%d, %d, \"\")",
						tt.goOffset, dingoStart, dingoEnd, kind, tt.goOffset, tt.goOffset)
				}
			}
		})
	}
}

func TestReaderFindByDingoPos(t *testing.T) {
	dingoSrc := []byte("let x = 10\nlet y = 20\n")
	goSrc := []byte("x := 10\ny := 20\n")

	mappings := []ast.SourceMapping{
		{DingoStart: 0, DingoEnd: 10, GoStart: 0, GoEnd: 7, Kind: "let_binding"},
		{DingoStart: 11, DingoEnd: 21, GoStart: 8, GoEnd: 15, Kind: "let_binding"},
	}

	data := createTestDmap(t, dingoSrc, goSrc, mappings)
	reader, err := OpenBytes(data)
	if err != nil {
		t.Fatalf("OpenBytes failed: %v", err)
	}
	defer reader.Close()

	tests := []struct {
		name        string
		dingoOffset int
		wantStart   int
		wantEnd     int
		wantKind    string
		wantFound   bool
	}{
		{"first mapping start", 0, 0, 7, "let_binding", true},
		{"first mapping middle", 5, 0, 7, "let_binding", true},
		{"second mapping start", 11, 8, 15, "let_binding", true},
		{"gap between mappings", 10, 10, 10, "", false},
		{"after all mappings", 100, 100, 100, "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			goStart, goEnd, kind := reader.FindByDingoPos(tt.dingoOffset)

			if tt.wantFound {
				if goStart != tt.wantStart || goEnd != tt.wantEnd || kind != tt.wantKind {
					t.Errorf("FindByDingoPos(%d) = (%d, %d, %q), want (%d, %d, %q)",
						tt.dingoOffset, goStart, goEnd, kind,
						tt.wantStart, tt.wantEnd, tt.wantKind)
				}
			} else {
				// Identity mapping expected
				if goStart != tt.dingoOffset || goEnd != tt.dingoOffset || kind != "" {
					t.Errorf("FindByDingoPos(%d) identity mapping: got (%d, %d, %q), want (%d, %d, \"\")",
						tt.dingoOffset, goStart, goEnd, kind, tt.dingoOffset, tt.dingoOffset)
				}
			}
		})
	}
}

func TestReaderByteToLine(t *testing.T) {
	dingoSrc := []byte("line1\nline2\nline3\n")
	goSrc := []byte("a\nb\nc\nd\n")

	data := createTestDmap(t, dingoSrc, goSrc, nil)
	reader, err := OpenBytes(data)
	if err != nil {
		t.Fatalf("OpenBytes failed: %v", err)
	}
	defer reader.Close()

	// Test DingoByteToLine
	dingoTests := []struct {
		offset int
		want   int
	}{
		{0, 1},   // Start of line 1
		{5, 1},   // End of line 1 content
		{6, 2},   // Start of line 2
		{11, 2},  // End of line 2 content
		{12, 3},  // Start of line 3
		{17, 3},  // End of line 3 content
		{-1, 0},  // Out of range (negative)
		{100, 0}, // Out of range (too large)
	}

	for _, tt := range dingoTests {
		got := reader.DingoByteToLine(tt.offset)
		if got != tt.want {
			t.Errorf("DingoByteToLine(%d) = %d, want %d", tt.offset, got, tt.want)
		}
	}

	// Test GoByteToLine
	goTests := []struct {
		offset int
		want   int
	}{
		{0, 1}, // Start of line 1
		{1, 1}, // End of line 1 content
		{2, 2}, // Start of line 2
		{4, 3}, // Start of line 3
		{6, 4}, // Start of line 4
	}

	for _, tt := range goTests {
		got := reader.GoByteToLine(tt.offset)
		if got != tt.want {
			t.Errorf("GoByteToLine(%d) = %d, want %d", tt.offset, got, tt.want)
		}
	}
}

func TestReaderLineToByteOffset(t *testing.T) {
	dingoSrc := []byte("line1\nline2\nline3\n")
	goSrc := []byte("a\nb\nc\nd\n")

	data := createTestDmap(t, dingoSrc, goSrc, nil)
	reader, err := OpenBytes(data)
	if err != nil {
		t.Fatalf("OpenBytes failed: %v", err)
	}
	defer reader.Close()

	// Test DingoLineToByteOffset
	dingoTests := []struct {
		line int
		want int
	}{
		{1, 0},   // Line 1 starts at byte 0
		{2, 6},   // Line 2 starts at byte 6
		{3, 12},  // Line 3 starts at byte 12
		{0, -1},  // Out of range (zero)
		{10, -1}, // Out of range (too large)
	}

	for _, tt := range dingoTests {
		got := reader.DingoLineToByteOffset(tt.line)
		if got != tt.want {
			t.Errorf("DingoLineToByteOffset(%d) = %d, want %d", tt.line, got, tt.want)
		}
	}

	// Test GoLineToByteOffset
	goTests := []struct {
		line int
		want int
	}{
		{1, 0}, // Line 1 starts at byte 0
		{2, 2}, // Line 2 starts at byte 2
		{3, 4}, // Line 3 starts at byte 4
		{4, 6}, // Line 4 starts at byte 6
	}

	for _, tt := range goTests {
		got := reader.GoLineToByteOffset(tt.line)
		if got != tt.want {
			t.Errorf("GoLineToByteOffset(%d) = %d, want %d", tt.line, got, tt.want)
		}
	}
}

func TestReaderEmptyMappings(t *testing.T) {
	dingoSrc := []byte("package main\n")
	goSrc := []byte("package main\n")

	data := createTestDmap(t, dingoSrc, goSrc, nil)
	reader, err := OpenBytes(data)
	if err != nil {
		t.Fatalf("OpenBytes failed: %v", err)
	}
	defer reader.Close()

	if reader.EntryCount() != 0 {
		t.Errorf("EntryCount: got %d, want 0", reader.EntryCount())
	}

	// Lookups should return identity mappings
	dingoStart, dingoEnd, kind := reader.FindByGoPos(5)
	if dingoStart != 5 || dingoEnd != 5 || kind != "" {
		t.Errorf("FindByGoPos with no entries: got (%d, %d, %q), want (5, 5, \"\")",
			dingoStart, dingoEnd, kind)
	}
}

func TestReaderMultipleKinds(t *testing.T) {
	dingoSrc := []byte("let x = 10\nmatch y { A => 1 }\n")
	goSrc := []byte("x := 10\nswitch y { case A: return 1 }\n")

	mappings := []ast.SourceMapping{
		{DingoStart: 0, DingoEnd: 10, GoStart: 0, GoEnd: 7, Kind: "let_binding"},
		{DingoStart: 11, DingoEnd: 30, GoStart: 8, GoEnd: 35, Kind: "match_expr"},
	}

	data := createTestDmap(t, dingoSrc, goSrc, mappings)
	reader, err := OpenBytes(data)
	if err != nil {
		t.Fatalf("OpenBytes failed: %v", err)
	}
	defer reader.Close()

	if reader.KindCount() != 2 {
		t.Errorf("KindCount: got %d, want 2", reader.KindCount())
	}

	// Verify first kind
	_, _, kind1 := reader.FindByGoPos(3)
	if kind1 != "let_binding" {
		t.Errorf("First mapping kind: got %q, want %q", kind1, "let_binding")
	}

	// Verify second kind
	_, _, kind2 := reader.FindByGoPos(20)
	if kind2 != "match_expr" {
		t.Errorf("Second mapping kind: got %q, want %q", kind2, "match_expr")
	}
}

func TestReaderClose(t *testing.T) {
	dingoSrc := []byte("test\n")
	goSrc := []byte("test\n")

	data := createTestDmap(t, dingoSrc, goSrc, nil)
	reader, err := OpenBytes(data)
	if err != nil {
		t.Fatalf("OpenBytes failed: %v", err)
	}

	// Close should succeed
	if err := reader.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// After close, internal state should be cleared
	// (This tests the current implementation behavior)
}

func TestReaderZeroLengthGoRange(t *testing.T) {
	// Test the case where GoStart == GoEnd (removed syntax like 'let')
	dingoSrc := []byte("let x = 10\n")
	goSrc := []byte("x := 10\n")

	// Simulate 'let' keyword being removed (zero-length Go range)
	mappings := []ast.SourceMapping{
		{DingoStart: 0, DingoEnd: 3, GoStart: 0, GoEnd: 0, Kind: "let_keyword"},
		{DingoStart: 4, DingoEnd: 10, GoStart: 0, GoEnd: 7, Kind: "let_assign"},
	}

	data := createTestDmap(t, dingoSrc, goSrc, mappings)
	reader, err := OpenBytes(data)
	if err != nil {
		t.Fatalf("OpenBytes failed: %v", err)
	}
	defer reader.Close()

	// Looking up Go position 0 should NOT find the zero-length 'let_keyword' mapping
	// because the range check is GoStart <= offset < GoEnd, and 0 < 0 is false
	dingoStart, dingoEnd, kind := reader.FindByGoPos(0)

	// Should find the let_assign mapping (0-7)
	if kind != "let_assign" {
		t.Errorf("FindByGoPos(0) with zero-length range: got kind %q, want %q", kind, "let_assign")
	}
	if dingoStart != 4 || dingoEnd != 10 {
		t.Errorf("FindByGoPos(0): got (%d, %d), want (4, 10)", dingoStart, dingoEnd)
	}

	// Looking up Dingo position 0 (in the zero-length range) should find it
	goStart, goEnd, kind2 := reader.FindByDingoPos(0)
	if kind2 != "let_keyword" {
		t.Errorf("FindByDingoPos(0) for let_keyword: got kind %q, want %q", kind2, "let_keyword")
	}
	if goStart != 0 || goEnd != 0 {
		t.Errorf("FindByDingoPos(0): got (%d, %d), want (0, 0)", goStart, goEnd)
	}
}

func TestReaderConcurrentAccess(t *testing.T) {
	dingoSrc := []byte("let x = 10\nlet y = 20\n")
	goSrc := []byte("x := 10\ny := 20\n")

	mappings := []ast.SourceMapping{
		{DingoStart: 0, DingoEnd: 10, GoStart: 0, GoEnd: 7, Kind: "let_binding"},
	}

	data := createTestDmap(t, dingoSrc, goSrc, mappings)
	reader, err := OpenBytes(data)
	if err != nil {
		t.Fatalf("OpenBytes failed: %v", err)
	}
	defer reader.Close()

	// Run concurrent reads to verify thread safety
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				reader.FindByGoPos(3)
				reader.FindByDingoPos(5)
				reader.GoByteToLine(3)
				reader.DingoByteToLine(5)
				reader.EntryCount()
				reader.Header()
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}
