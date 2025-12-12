package dmap

import (
	"testing"

	"github.com/MadAppGang/dingo/pkg/sourcemap"
)

// Helper function to create a .dmap file in memory for testing
func createTestDmap(t *testing.T, dingoSrc, goSrc []byte, mappings []sourcemap.LineMapping) []byte {
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

	mappings := []sourcemap.LineMapping{
		{DingoLine: 1, GoLineStart: 1, GoLineEnd: 1, Kind: "identifier"},
		{DingoLine: 2, GoLineStart: 2, GoLineEnd: 2, Kind: "identifier"},
	}

	data := createTestDmap(t, dingoSrc, goSrc, mappings)

	reader, err := OpenBytes(data)
	if err != nil {
		t.Fatalf("OpenBytes failed: %v", err)
	}
	defer reader.Close()

	// v2 format has no token-level entries
	if reader.EntryCount() != 0 {
		t.Errorf("EntryCount: got %d, want 0 (v2 format)", reader.EntryCount())
	}

	// But should have line mappings
	if reader.Header().LineMappingCnt != 2 {
		t.Errorf("LineMappingCnt: got %d, want 2", reader.Header().LineMappingCnt)
	}

	// Kind count should be 1 (deduplicated)
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

	if reader.Header().LineMappingCnt != 0 {
		t.Errorf("LineMappingCnt: got %d, want 0", reader.Header().LineMappingCnt)
	}
}

func TestReaderGoLineToDingoLine(t *testing.T) {
	dingoSrc := []byte("let x = 10\nlet y = 20\n")
	goSrc := []byte("x := 10\ny := 20\n")

	mappings := []sourcemap.LineMapping{
		{DingoLine: 1, GoLineStart: 1, GoLineEnd: 1, Kind: "identifier"},
		{DingoLine: 2, GoLineStart: 2, GoLineEnd: 2, Kind: "identifier"},
	}

	data := createTestDmap(t, dingoSrc, goSrc, mappings)
	reader, err := OpenBytes(data)
	if err != nil {
		t.Fatalf("OpenBytes failed: %v", err)
	}
	defer reader.Close()

	tests := []struct {
		name      string
		goLine    int
		wantDingo int
	}{
		{"line 1", 1, 1},
		{"line 2", 2, 2},
		// Line 0 is invalid (1-indexed), should clamp to first line
		{"unmapped line 0", 0, 1},
		// Line 100 is beyond file (3 lines), should clamp using proportional mapping
		{"unmapped line 100", 100, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dingoLine, _ := reader.GoLineToDingoLine(tt.goLine)

			if dingoLine != tt.wantDingo {
				t.Errorf("GoLineToDingoLine(%d) = %d, want %d",
					tt.goLine, dingoLine, tt.wantDingo)
			}
		})
	}
}

func TestReaderMultipleKinds(t *testing.T) {
	dingoSrc := []byte("let x = 10\nmatch y { A => 1 }\n")
	goSrc := []byte("x := 10\nswitch y { case A: return 1 }\n")

	mappings := []sourcemap.LineMapping{
		{DingoLine: 1, GoLineStart: 1, GoLineEnd: 1, Kind: "identifier"},
		{DingoLine: 2, GoLineStart: 2, GoLineEnd: 2, Kind: "match_expr"},
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
}

func TestReaderConcurrentAccess(t *testing.T) {
	dingoSrc := []byte("foo x = 10\nbar y = 20\n")
	goSrc := []byte("x := 10\ny := 20\n")

	mappings := []sourcemap.LineMapping{
		{DingoLine: 1, GoLineStart: 1, GoLineEnd: 1, Kind: "identifier"},
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
				reader.GoLineToDingoLine(1)
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
