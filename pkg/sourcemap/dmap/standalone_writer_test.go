package dmap

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/MadAppGang/dingo/pkg/ast"
)

// TestWriterStandalone tests the writer without depending on reader
func TestWriterStandalone(t *testing.T) {
	dingoSrc := []byte("let x = 10\nlet y = 20\n")
	goSrc := []byte("x := 10\ny := 20\n")

	mappings := []ast.SourceMapping{
		{DingoStart: 0, DingoEnd: 10, GoStart: 0, GoEnd: 7, Kind: "let_binding"},
		{DingoStart: 11, DingoEnd: 21, GoStart: 8, GoEnd: 15, Kind: "let_binding"},
	}

	writer := NewWriter(dingoSrc, goSrc)
	data, err := writer.Write(mappings)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	t.Logf("Generated %d bytes of .dmap data", len(data))

	// Verify we can parse the data manually
	if len(data) < HeaderSize {
		t.Fatalf("Data too short: got %d bytes, want at least %d", len(data), HeaderSize)
	}

	// Verify magic
	if !bytes.Equal(data[0:4], Magic[:]) {
		t.Errorf("Invalid magic: got %x, want %x", data[0:4], Magic)
	}

	// Verify version
	version := binary.LittleEndian.Uint16(data[4:6])
	if version != Version {
		t.Errorf("Invalid version: got %d, want %d", version, Version)
	}

	// Verify entry count
	entryCount := binary.LittleEndian.Uint32(data[8:12])
	if entryCount != 2 {
		t.Errorf("Invalid entry count: got %d, want 2", entryCount)
	}

	// Verify source lengths
	dingoLen := binary.LittleEndian.Uint32(data[12:16])
	if dingoLen != uint32(len(dingoSrc)) {
		t.Errorf("Invalid DingoLen: got %d, want %d", dingoLen, len(dingoSrc))
	}

	goLen := binary.LittleEndian.Uint32(data[16:20])
	if goLen != uint32(len(goSrc)) {
		t.Errorf("Invalid GoLen: got %d, want %d", goLen, len(goSrc))
	}

	// Read all offsets
	goIdxOff := binary.LittleEndian.Uint32(data[20:24])
	dingoIdxOff := binary.LittleEndian.Uint32(data[24:28])
	lineIdxOff := binary.LittleEndian.Uint32(data[28:32])
	kindStrOff := binary.LittleEndian.Uint32(data[32:36])

	t.Logf("Offsets: GoIdx=%d, DingoIdx=%d, LineIdx=%d, KindStr=%d",
		goIdxOff, dingoIdxOff, lineIdxOff, kindStrOff)

	// Verify Go index is at expected offset
	if goIdxOff != HeaderSize {
		t.Errorf("GoIdxOff incorrect: got %d, want %d", goIdxOff, HeaderSize)
	}

	// Read first entry from Go index
	entry1Start := binary.LittleEndian.Uint32(data[goIdxOff+8 : goIdxOff+12])
	if entry1Start != 0 {
		t.Errorf("First Go entry start incorrect: got %d, want 0", entry1Start)
	}

	// Read first entry from Dingo index
	entry1DingoStart := binary.LittleEndian.Uint32(data[dingoIdxOff : dingoIdxOff+4])
	if entry1DingoStart != 0 {
		t.Errorf("First Dingo entry start incorrect: got %d, want 0", entry1DingoStart)
	}
}

func TestWriterFileWrite(t *testing.T) {
	dingoSrc := []byte("package main\n")
	goSrc := []byte("package main\n")

	mappings := []ast.SourceMapping{}

	writer := NewWriter(dingoSrc, goSrc)

	// Write to temp file
	tmpDir := t.TempDir()
	dmapPath := filepath.Join(tmpDir, "test.dmap")

	err := writer.WriteFile(dmapPath, mappings)
	if err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Verify file exists and has content
	data, err := os.ReadFile(dmapPath)
	if err != nil {
		t.Fatalf("Failed to read written file: %v", err)
	}

	if len(data) < HeaderSize {
		t.Errorf("Written file too small: got %d bytes", len(data))
	}

	// Verify magic
	if !bytes.Equal(data[0:4], Magic[:]) {
		t.Errorf("Written file has invalid magic")
	}
}

func TestWriterMultipleKinds(t *testing.T) {
	dingoSrc := []byte("let x = match y { Some(v) => v }\n")
	goSrc := []byte("var x = func() { switch ... }()\n")

	mappings := []ast.SourceMapping{
		{DingoStart: 0, DingoEnd: 5, GoStart: 0, GoEnd: 5, Kind: "let_binding"},
		{DingoStart: 8, DingoEnd: 30, GoStart: 8, GoEnd: 25, Kind: "match_expr"},
		{DingoStart: 14, DingoEnd: 22, GoStart: 14, GoEnd: 20, Kind: "pattern"},
	}

	writer := NewWriter(dingoSrc, goSrc)
	data, err := writer.Write(mappings)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Read kind strings section
	kindStrOff := binary.LittleEndian.Uint32(data[32:36])
	kindCount := binary.LittleEndian.Uint32(data[kindStrOff : kindStrOff+4])

	// Should have 3 unique kinds
	if kindCount != 3 {
		t.Errorf("Kind count incorrect: got %d, want 3", kindCount)
	}
}

func TestWriterLineOffsetsCorrectness(t *testing.T) {
	// Source with known line boundaries
	dingoSrc := []byte("a\nb\nc")  // Lines at: 0, 2, 4
	goSrc := []byte("x\ny\nz\nw") // Lines at: 0, 2, 4, 6

	writer := NewWriter(dingoSrc, goSrc)
	data, err := writer.Write([]ast.SourceMapping{})
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Read line index section
	lineIdxOff := binary.LittleEndian.Uint32(data[28:32])

	dingoLineCount := binary.LittleEndian.Uint32(data[lineIdxOff : lineIdxOff+4])
	goLineCount := binary.LittleEndian.Uint32(data[lineIdxOff+4 : lineIdxOff+8])

	if dingoLineCount != 3 {
		t.Errorf("DingoLineCount incorrect: got %d, want 3", dingoLineCount)
	}

	if goLineCount != 4 {
		t.Errorf("GoLineCount incorrect: got %d, want 4", goLineCount)
	}

	// Verify Dingo line offsets
	offset := lineIdxOff + 8
	expectedDingoOffsets := []uint32{0, 2, 4}
	for i, expected := range expectedDingoOffsets {
		actual := binary.LittleEndian.Uint32(data[offset : offset+4])
		if actual != expected {
			t.Errorf("DingoLine[%d] offset incorrect: got %d, want %d", i, actual, expected)
		}
		offset += 4
	}

	// Verify Go line offsets
	expectedGoOffsets := []uint32{0, 2, 4, 6}
	for i, expected := range expectedGoOffsets {
		actual := binary.LittleEndian.Uint32(data[offset : offset+4])
		if actual != expected {
			t.Errorf("GoLine[%d] offset incorrect: got %d, want %d", i, actual, expected)
		}
		offset += 4
	}
}

func TestWriterEntrySorting(t *testing.T) {
	dingoSrc := []byte("aaabbbccc")
	goSrc := []byte("xxxyyyzzzz")

	// Create mappings in random order
	mappings := []ast.SourceMapping{
		{DingoStart: 6, DingoEnd: 9, GoStart: 6, GoEnd: 10, Kind: "c"},
		{DingoStart: 0, DingoEnd: 3, GoStart: 0, GoEnd: 3, Kind: "a"},
		{DingoStart: 3, DingoEnd: 6, GoStart: 3, GoEnd: 6, Kind: "b"},
	}

	writer := NewWriter(dingoSrc, goSrc)
	data, err := writer.Write(mappings)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Read Go index - should be sorted by GoStart (0, 3, 6)
	goIdxOff := binary.LittleEndian.Uint32(data[20:24])

	for i := 0; i < 3; i++ {
		entryOff := goIdxOff + uint32(i*EntrySize)
		goStart := binary.LittleEndian.Uint32(data[entryOff+8 : entryOff+12])
		expectedGoStart := uint32(i * 3)
		if i == 2 {
			expectedGoStart = 6 // Last entry spans 6-10
		}
		if goStart != expectedGoStart {
			t.Errorf("Go index entry[%d] not sorted: got GoStart=%d, want %d",
				i, goStart, expectedGoStart)
		}
	}

	// Read Dingo index - should be sorted by DingoStart (0, 3, 6)
	dingoIdxOff := binary.LittleEndian.Uint32(data[24:28])

	for i := 0; i < 3; i++ {
		entryOff := dingoIdxOff + uint32(i*EntrySize)
		dingoStart := binary.LittleEndian.Uint32(data[entryOff : entryOff+4])
		expectedDingoStart := uint32(i * 3)
		if dingoStart != expectedDingoStart {
			t.Errorf("Dingo index entry[%d] not sorted: got DingoStart=%d, want %d",
				i, dingoStart, expectedDingoStart)
		}
	}
}
