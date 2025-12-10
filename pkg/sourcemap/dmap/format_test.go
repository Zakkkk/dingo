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
	if Version != 2 {
		t.Errorf("Version incorrect: got %d, want 2", Version)
	}
}

func TestHeaderSize(t *testing.T) {
	if HeaderSize != 44 {
		t.Errorf("HeaderSize incorrect: got %d, want 44", HeaderSize)
	}
}

func TestLineMappingEntrySize(t *testing.T) {
	if LineMappingEntrySize != 16 {
		t.Errorf("LineMappingEntrySize incorrect: got %d, want 16", LineMappingEntrySize)
	}
}

func TestEntrySize(t *testing.T) {
	if EntrySize != 20 {
		t.Errorf("EntrySize incorrect: got %d, want 20", EntrySize)
	}
}
