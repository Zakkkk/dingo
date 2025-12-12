package dmap

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/MadAppGang/dingo/pkg/sourcemap"
)

func TestWriterBasic(t *testing.T) {
	dingoSrc := []byte("let x = 10\nlet y = 20\n")
	goSrc := []byte("x := 10\ny := 20\n")

	mappings := []sourcemap.LineMapping{
		{DingoLine: 1, GoLineStart: 1, GoLineEnd: 1, Kind: "identifier"},
		{DingoLine: 2, GoLineStart: 2, GoLineEnd: 2, Kind: "identifier"},
	}

	writer := NewWriter(dingoSrc, goSrc)
	data, err := writer.Write(mappings, nil) // No column mappings
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Verify header
	if len(data) < HeaderSize {
		t.Fatalf("Output too short: got %d bytes, want at least %d", len(data), HeaderSize)
	}

	// Check magic
	if !bytes.Equal(data[0:4], Magic[:]) {
		t.Errorf("Invalid magic: got %v, want %v", data[0:4], Magic)
	}

	// Check version
	version := binary.LittleEndian.Uint16(data[4:6])
	if version != Version {
		t.Errorf("Invalid version: got %d, want %d", version, Version)
	}

	// Check source lengths (v3 format: bytes 8-12 DingoLen, 12-16 GoLen)
	dingoLen := binary.LittleEndian.Uint32(data[8:12])
	if dingoLen != uint32(len(dingoSrc)) {
		t.Errorf("Invalid DingoLen: got %d, want %d", dingoLen, len(dingoSrc))
	}

	goLen := binary.LittleEndian.Uint32(data[12:16])
	if goLen != uint32(len(goSrc)) {
		t.Errorf("Invalid GoLen: got %d, want %d", goLen, len(goSrc))
	}

	// Check line mapping count (v3 format: bytes 32-36)
	lineMappingCnt := binary.LittleEndian.Uint32(data[32:36])
	if lineMappingCnt != 2 {
		t.Errorf("Invalid LineMappingCnt: got %d, want 2", lineMappingCnt)
	}
}

func TestWriterEmptyMappings(t *testing.T) {
	dingoSrc := []byte("package main\n")
	goSrc := []byte("package main\n")

	writer := NewWriter(dingoSrc, goSrc)
	data, err := writer.Write([]sourcemap.LineMapping{}, nil)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Should still have valid header
	if len(data) < HeaderSize {
		t.Fatalf("Output too short: got %d bytes", len(data))
	}

	// Line mapping count should be 0 (v3 format: bytes 32-36)
	lineMappingCnt := binary.LittleEndian.Uint32(data[32:36])
	if lineMappingCnt != 0 {
		t.Errorf("Invalid LineMappingCnt: got %d, want 0", lineMappingCnt)
	}
}

func TestWriterKindDeduplication(t *testing.T) {
	dingoSrc := []byte("let x = 10\nlet y = 20\nlet z = 30\n")
	goSrc := []byte("x := 10\ny := 20\nz := 30\n")

	// All mappings have the same kind
	mappings := []sourcemap.LineMapping{
		{DingoLine: 1, GoLineStart: 1, GoLineEnd: 1, Kind: "identifier"},
		{DingoLine: 2, GoLineStart: 2, GoLineEnd: 2, Kind: "identifier"},
		{DingoLine: 3, GoLineStart: 3, GoLineEnd: 3, Kind: "identifier"},
	}

	writer := NewWriter(dingoSrc, goSrc)
	data, err := writer.Write(mappings, nil) // No column mappings
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Read kind strings section (v3 format: bytes 44-48)
	kindStrOff := binary.LittleEndian.Uint32(data[44:48])
	kindCount := binary.LittleEndian.Uint32(data[kindStrOff : kindStrOff+4])

	// Should only have 1 unique kind
	if kindCount != 1 {
		t.Errorf("Kind not deduplicated: got %d kinds, want 1", kindCount)
	}
}

func TestWriterLineOffsets(t *testing.T) {
	dingoSrc := []byte("line1\nline2\nline3")
	goSrc := []byte("a\nb\nc\nd")

	writer := NewWriter(dingoSrc, goSrc)
	data, err := writer.Write([]sourcemap.LineMapping{}, nil)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Read line index section (v3 format: bytes 16-20)
	lineIdxOff := binary.LittleEndian.Uint32(data[16:20])
	dingoLineCount := binary.LittleEndian.Uint32(data[lineIdxOff : lineIdxOff+4])
	goLineCount := binary.LittleEndian.Uint32(data[lineIdxOff+4 : lineIdxOff+8])

	// dingoSrc has 3 lines (starts: 0, 6, 12)
	if dingoLineCount != 3 {
		t.Errorf("Invalid DingoLineCount: got %d, want 3", dingoLineCount)
	}

	// goSrc has 4 lines (starts: 0, 2, 4, 6)
	if goLineCount != 4 {
		t.Errorf("Invalid GoLineCount: got %d, want 4", goLineCount)
	}

	// Verify first few offsets
	offset := lineIdxOff + 8
	dingoLine0 := binary.LittleEndian.Uint32(data[offset : offset+4])
	if dingoLine0 != 0 {
		t.Errorf("Invalid dingoLine0 offset: got %d, want 0", dingoLine0)
	}

	dingoLine1 := binary.LittleEndian.Uint32(data[offset+4 : offset+8])
	if dingoLine1 != 6 {
		t.Errorf("Invalid dingoLine1 offset: got %d, want 6", dingoLine1)
	}
}

func TestWriterLineMappingEntries(t *testing.T) {
	dingoSrc := []byte("line1\nline2\nline3\n")
	goSrc := []byte("a\nb\nc\nd\n")

	// Add mappings in order
	mappings := []sourcemap.LineMapping{
		{DingoLine: 1, GoLineStart: 1, GoLineEnd: 2, Kind: "multi"},
		{DingoLine: 2, GoLineStart: 3, GoLineEnd: 3, Kind: "single"},
		{DingoLine: 3, GoLineStart: 4, GoLineEnd: 4, Kind: "single"},
	}

	writer := NewWriter(dingoSrc, goSrc)
	data, err := writer.Write(mappings, nil) // No column mappings
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Read line mapping section (v3 format: bytes 28-32 offset, 32-36 count)
	lineMappingOff := binary.LittleEndian.Uint32(data[28:32])
	lineMappingCnt := binary.LittleEndian.Uint32(data[32:36])

	if lineMappingCnt != 3 {
		t.Errorf("LineMappingCnt: got %d, want 3", lineMappingCnt)
	}

	// Verify first entry (DingoLine=1, GoLineStart=1, GoLineEnd=2)
	entry0Off := lineMappingOff
	entry0DingoLine := binary.LittleEndian.Uint32(data[entry0Off : entry0Off+4])
	entry0GoLineStart := binary.LittleEndian.Uint32(data[entry0Off+4 : entry0Off+8])
	entry0GoLineEnd := binary.LittleEndian.Uint32(data[entry0Off+8 : entry0Off+12])

	if entry0DingoLine != 1 {
		t.Errorf("Entry0 DingoLine: got %d, want 1", entry0DingoLine)
	}
	if entry0GoLineStart != 1 {
		t.Errorf("Entry0 GoLineStart: got %d, want 1", entry0GoLineStart)
	}
	if entry0GoLineEnd != 2 {
		t.Errorf("Entry0 GoLineEnd: got %d, want 2", entry0GoLineEnd)
	}
}
