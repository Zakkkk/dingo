package dmap

import (
	"bytes"
	"testing"
)

func TestMagicConstant(t *testing.T) {
	expected := [4]byte{'D', 'M', 'A', 'P'}
	if !bytes.Equal(Magic[:], expected[:]) {
		t.Errorf("Magic constant incorrect: got %v, want %v", Magic, expected)
	}
}

func TestVersion(t *testing.T) {
	if Version != 3 {
		t.Errorf("Version incorrect: got %d, want 3", Version)
	}
}

func TestHeaderSize(t *testing.T) {
	if HeaderSize != 56 {
		t.Errorf("HeaderSize incorrect: got %d, want 56", HeaderSize)
	}
}

func TestLineMappingEntrySize(t *testing.T) {
	if LineMappingEntrySize != 16 {
		t.Errorf("LineMappingEntrySize incorrect: got %d, want 16", LineMappingEntrySize)
	}
}

func TestColumnMappingEntrySize(t *testing.T) {
	if ColumnMappingEntrySize != 16 {
		t.Errorf("ColumnMappingEntrySize incorrect: got %d, want 16", ColumnMappingEntrySize)
	}
}
