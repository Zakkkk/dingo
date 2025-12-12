package dmap

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/MadAppGang/dingo/pkg/sourcemap"
)

// TestWriterStandalone tests the writer without depending on reader
func TestWriterStandalone(t *testing.T) {
	dingoSrc := []byte("let x = 10\nlet y = 20\n")
	goSrc := []byte("x := 10\ny := 20\n")

	mappings := []sourcemap.LineMapping{
		{DingoLine: 1, GoLineStart: 1, GoLineEnd: 1, Kind: "identifier"},
		{DingoLine: 2, GoLineStart: 2, GoLineEnd: 2, Kind: "identifier"},
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

	// v2 format has EntryCount = 0 (no token-level entries)
	entryCount := binary.LittleEndian.Uint32(data[8:12])
	if entryCount != 0 {
		t.Errorf("Invalid entry count: got %d, want 0 (v2 format)", entryCount)
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

	// Read v2 offsets
	lineIdxOff := binary.LittleEndian.Uint32(data[28:32])
	kindStrOff := binary.LittleEndian.Uint32(data[32:36])
	lineMappingOff := binary.LittleEndian.Uint32(data[36:40])
	lineMappingCnt := binary.LittleEndian.Uint32(data[40:44])

	t.Logf("Offsets: LineIdx=%d, KindStr=%d, LineMapping=%d, LineMappingCnt=%d",
		lineIdxOff, kindStrOff, lineMappingOff, lineMappingCnt)

	// Verify line mapping count
	if lineMappingCnt != 2 {
		t.Errorf("LineMappingCnt incorrect: got %d, want 2", lineMappingCnt)
	}
}

func TestWriterFileWrite(t *testing.T) {
	dingoSrc := []byte("package main\n")
	goSrc := []byte("package main\n")

	mappings := []sourcemap.LineMapping{}

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

	mappings := []sourcemap.LineMapping{
		{DingoLine: 1, GoLineStart: 1, GoLineEnd: 1, Kind: "identifier"},
		{DingoLine: 1, GoLineStart: 1, GoLineEnd: 1, Kind: "match_expr"},
		{DingoLine: 1, GoLineStart: 1, GoLineEnd: 1, Kind: "pattern"},
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
	data, err := writer.Write([]sourcemap.LineMapping{})
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

func TestStandaloneWriterLineMappingEntries(t *testing.T) {
	dingoSrc := []byte("line1\nline2\nline3\n")
	goSrc := []byte("out1\nout2\nout3\nout4\n")

	// Line mappings: dingo line -> go line range
	mappings := []sourcemap.LineMapping{
		{DingoLine: 1, GoLineStart: 1, GoLineEnd: 2, Kind: "multi_line"},
		{DingoLine: 2, GoLineStart: 3, GoLineEnd: 3, Kind: "single_line"},
		{DingoLine: 3, GoLineStart: 4, GoLineEnd: 4, Kind: "single_line"},
	}

	writer := NewWriter(dingoSrc, goSrc)
	data, err := writer.Write(mappings)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Read line mapping section
	lineMappingOff := binary.LittleEndian.Uint32(data[36:40])
	lineMappingCnt := binary.LittleEndian.Uint32(data[40:44])

	if lineMappingCnt != 3 {
		t.Errorf("LineMappingCnt incorrect: got %d, want 3", lineMappingCnt)
	}

	// Verify first entry
	entry1Off := lineMappingOff
	dingoLine1 := binary.LittleEndian.Uint32(data[entry1Off : entry1Off+4])
	goLineStart1 := binary.LittleEndian.Uint32(data[entry1Off+4 : entry1Off+8])
	goLineEnd1 := binary.LittleEndian.Uint32(data[entry1Off+8 : entry1Off+12])

	if dingoLine1 != 1 {
		t.Errorf("Entry 1 DingoLine incorrect: got %d, want 1", dingoLine1)
	}
	if goLineStart1 != 1 {
		t.Errorf("Entry 1 GoLineStart incorrect: got %d, want 1", goLineStart1)
	}
	if goLineEnd1 != 2 {
		t.Errorf("Entry 1 GoLineEnd incorrect: got %d, want 2", goLineEnd1)
	}
}
