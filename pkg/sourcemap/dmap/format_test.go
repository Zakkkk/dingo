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
	if Version != 1 {
		t.Errorf("Version incorrect: got %d, want 1", Version)
	}
}

func TestHeaderSize(t *testing.T) {
	if HeaderSize != 36 {
		t.Errorf("HeaderSize incorrect: got %d, want 36", HeaderSize)
	}
}

func TestEntrySize(t *testing.T) {
	if EntrySize != 20 {
		t.Errorf("EntrySize incorrect: got %d, want 20", EntrySize)
	}
}
