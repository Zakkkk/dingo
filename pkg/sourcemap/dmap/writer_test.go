package dmap

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/MadAppGang/dingo/pkg/ast"
)

func TestWriterBasic(t *testing.T) {
	dingoSrc := []byte("let x = 10\nlet y = 20\n")
	goSrc := []byte("x := 10\ny := 20\n")

	mappings := []ast.SourceMapping{
		{DingoStart: 0, DingoEnd: 10, GoStart: 0, GoEnd: 7, Kind: "identifier"},
		{DingoStart: 11, DingoEnd: 21, GoStart: 8, GoEnd: 15, Kind: "identifier"},
	}

	writer := NewWriter(dingoSrc, goSrc)
	data, err := writer.Write(mappings)
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

	// Check entry count
	entryCount := binary.LittleEndian.Uint32(data[8:12])
	if entryCount != 2 {
		t.Errorf("Invalid entry count: got %d, want 2", entryCount)
	}

	// Check source lengths
	dingoLen := binary.LittleEndian.Uint32(data[12:16])
	if dingoLen != uint32(len(dingoSrc)) {
		t.Errorf("Invalid DingoLen: got %d, want %d", dingoLen, len(dingoSrc))
	}

	goLen := binary.LittleEndian.Uint32(data[16:20])
	if goLen != uint32(len(goSrc)) {
		t.Errorf("Invalid GoLen: got %d, want %d", goLen, len(goSrc))
	}

	// Check that GoIdxOff is HeaderSize
	goIdxOff := binary.LittleEndian.Uint32(data[20:24])
	if goIdxOff != HeaderSize {
		t.Errorf("Invalid GoIdxOff: got %d, want %d", goIdxOff, HeaderSize)
	}
}

func TestWriterEmptyMappings(t *testing.T) {
	dingoSrc := []byte("package main\n")
	goSrc := []byte("package main\n")

	writer := NewWriter(dingoSrc, goSrc)
	data, err := writer.Write([]ast.SourceMapping{})
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Should still have valid header
	if len(data) < HeaderSize {
		t.Fatalf("Output too short: got %d bytes", len(data))
	}

	entryCount := binary.LittleEndian.Uint32(data[8:12])
	if entryCount != 0 {
		t.Errorf("Invalid entry count: got %d, want 0", entryCount)
	}
}

func TestWriterKindDeduplication(t *testing.T) {
	dingoSrc := []byte("let x = 10\nlet y = 20\nlet z = 30\n")
	goSrc := []byte("x := 10\ny := 20\nz := 30\n")

	// All mappings have the same kind
	mappings := []ast.SourceMapping{
		{DingoStart: 0, DingoEnd: 10, GoStart: 0, GoEnd: 7, Kind: "identifier"},
		{DingoStart: 11, DingoEnd: 21, GoStart: 8, GoEnd: 15, Kind: "identifier"},
		{DingoStart: 22, DingoEnd: 32, GoStart: 16, GoEnd: 23, Kind: "identifier"},
	}

	writer := NewWriter(dingoSrc, goSrc)
	data, err := writer.Write(mappings)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Read kind strings section
	kindStrOff := binary.LittleEndian.Uint32(data[32:36])
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
	data, err := writer.Write([]ast.SourceMapping{})
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Read line index section
	lineIdxOff := binary.LittleEndian.Uint32(data[28:32])
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

func TestWriterSorting(t *testing.T) {
	dingoSrc := []byte("a\nb\nc\n")
	goSrc := []byte("x\ny\nz\n")

	// Add mappings in random order
	mappings := []ast.SourceMapping{
		{DingoStart: 4, DingoEnd: 6, GoStart: 4, GoEnd: 6, Kind: "c"},
		{DingoStart: 0, DingoEnd: 2, GoStart: 0, GoEnd: 2, Kind: "a"},
		{DingoStart: 2, DingoEnd: 4, GoStart: 2, GoEnd: 4, Kind: "b"},
	}

	writer := NewWriter(dingoSrc, goSrc)
	data, err := writer.Write(mappings)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Read Go index (should be sorted by GoStart)
	goIdxOff := binary.LittleEndian.Uint32(data[20:24])
	entry0GoStart := binary.LittleEndian.Uint32(data[goIdxOff+8 : goIdxOff+12])
	entry1GoStart := binary.LittleEndian.Uint32(data[goIdxOff+28 : goIdxOff+32])
	entry2GoStart := binary.LittleEndian.Uint32(data[goIdxOff+48 : goIdxOff+52])

	if entry0GoStart != 0 || entry1GoStart != 2 || entry2GoStart != 4 {
		t.Errorf("Go index not sorted: got [%d, %d, %d], want [0, 2, 4]",
			entry0GoStart, entry1GoStart, entry2GoStart)
	}

	// Read Dingo index (should be sorted by DingoStart)
	dingoIdxOff := binary.LittleEndian.Uint32(data[24:28])
	entry0DingoStart := binary.LittleEndian.Uint32(data[dingoIdxOff : dingoIdxOff+4])
	entry1DingoStart := binary.LittleEndian.Uint32(data[dingoIdxOff+20 : dingoIdxOff+24])
	entry2DingoStart := binary.LittleEndian.Uint32(data[dingoIdxOff+40 : dingoIdxOff+44])

	if entry0DingoStart != 0 || entry1DingoStart != 2 || entry2DingoStart != 4 {
		t.Errorf("Dingo index not sorted: got [%d, %d, %d], want [0, 2, 4]",
			entry0DingoStart, entry1DingoStart, entry2DingoStart)
	}
}
