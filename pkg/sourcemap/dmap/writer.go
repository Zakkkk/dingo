package dmap

import (
	"encoding/binary"
	"fmt"
	"os"
	"sort"

	"github.com/MadAppGang/dingo/pkg/ast"
)

// Writer generates binary .dmap files from source mappings
type Writer struct {
	dingoSrc []byte
	goSrc    []byte
}

// NewWriter creates a Writer with source files for line index generation
func NewWriter(dingoSrc, goSrc []byte) *Writer {
	return &Writer{
		dingoSrc: dingoSrc,
		goSrc:    goSrc,
	}
}

// Write generates .dmap bytes from source mappings
func (w *Writer) Write(mappings []ast.SourceMapping) ([]byte, error) {
	// Build kind string table (deduplicated)
	kindMap := make(map[string]uint16)
	kinds := []string{}
	for _, m := range mappings {
		if _, exists := kindMap[m.Kind]; !exists {
			kindMap[m.Kind] = uint16(len(kinds))
			kinds = append(kinds, m.Kind)
		}
	}

	// Convert ast.SourceMapping to Entry with KindIdx
	entries := make([]Entry, len(mappings))
	for i, m := range mappings {
		entries[i] = Entry{
			DingoStart: uint32(m.DingoStart),
			DingoEnd:   uint32(m.DingoEnd),
			GoStart:    uint32(m.GoStart),
			GoEnd:      uint32(m.GoEnd),
			KindIdx:    kindMap[m.Kind],
			Reserved:   0,
		}
	}

	// Create Go index (sorted by GoStart)
	goEntries := make([]Entry, len(entries))
	copy(goEntries, entries)
	sort.Slice(goEntries, func(i, j int) bool {
		return goEntries[i].GoStart < goEntries[j].GoStart
	})

	// Create Dingo index (sorted by DingoStart)
	dingoEntries := make([]Entry, len(entries))
	copy(dingoEntries, entries)
	sort.Slice(dingoEntries, func(i, j int) bool {
		return dingoEntries[i].DingoStart < dingoEntries[j].DingoStart
	})

	// Build line offsets for Dingo and Go sources
	dingoLines := buildLineOffsets(w.dingoSrc)
	goLines := buildLineOffsets(w.goSrc)

	// Calculate section offsets
	goIdxOff := uint32(HeaderSize)
	dingoIdxOff := goIdxOff + uint32(len(goEntries))*EntrySize
	lineIdxOff := dingoIdxOff + uint32(len(dingoEntries))*EntrySize
	lineIdxSize := 8 + uint32(len(dingoLines))*4 + uint32(len(goLines))*4
	kindStrOff := lineIdxOff + lineIdxSize

	// Build header
	header := Header{
		Magic:       Magic,
		Version:     Version,
		Flags:       0,
		EntryCount:  uint32(len(entries)),
		DingoLen:    uint32(len(w.dingoSrc)),
		GoLen:       uint32(len(w.goSrc)),
		GoIdxOff:    goIdxOff,
		DingoIdxOff: dingoIdxOff,
		LineIdxOff:  lineIdxOff,
		KindStrOff:  kindStrOff,
	}

	// Estimate total size
	kindStrSize := estimateKindStrSize(kinds)
	totalSize := int(kindStrOff) + kindStrSize

	// Allocate output buffer
	buf := make([]byte, totalSize)

	// Write header
	writeHeader(buf, header)

	// Write Go index
	offset := int(goIdxOff)
	for _, entry := range goEntries {
		writeEntry(buf[offset:], entry)
		offset += EntrySize
	}

	// Verify we're at the expected Dingo index offset
	if offset != int(dingoIdxOff) {
		return nil, fmt.Errorf("internal error: writer offset mismatch at Go index (expected %d, got %d)", dingoIdxOff, offset)
	}

	// Write Dingo index
	for _, entry := range dingoEntries {
		writeEntry(buf[offset:], entry)
		offset += EntrySize
	}

	// Verify we're at the expected line index offset
	if offset != int(lineIdxOff) {
		return nil, fmt.Errorf("internal error: writer offset mismatch at Dingo index (expected %d, got %d)", lineIdxOff, offset)
	}

	// Write line index
	binary.LittleEndian.PutUint32(buf[offset:], uint32(len(dingoLines)))
	offset += 4
	binary.LittleEndian.PutUint32(buf[offset:], uint32(len(goLines)))
	offset += 4
	for _, lineOff := range dingoLines {
		binary.LittleEndian.PutUint32(buf[offset:], lineOff)
		offset += 4
	}
	for _, lineOff := range goLines {
		binary.LittleEndian.PutUint32(buf[offset:], lineOff)
		offset += 4
	}

	// Verify we're at the expected kind strings offset
	if offset != int(kindStrOff) {
		return nil, fmt.Errorf("internal error: writer offset mismatch at line index (expected %d, got %d)", kindStrOff, offset)
	}

	// Write kind strings
	finalOffset := writeKindStrings(buf[offset:], kinds)

	// Return exact buffer
	return buf[:offset+finalOffset], nil
}

// WriteFile writes .dmap directly to a file
func (w *Writer) WriteFile(path string, mappings []ast.SourceMapping) error {
	data, err := w.Write(mappings)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// buildLineOffsets scans source and returns byte offset of each line start
func buildLineOffsets(src []byte) []uint32 {
	if len(src) == 0 {
		return []uint32{0}
	}

	offsets := []uint32{0} // First line starts at byte 0
	for i := 0; i < len(src); i++ {
		if src[i] == '\n' {
			// Check for CRLF sequence
			if i > 0 && src[i-1] == '\r' {
				// CRLF - line starts after the \n
				offsets = append(offsets, uint32(i+1))
			} else {
				// Just LF - line starts after the \n
				offsets = append(offsets, uint32(i+1))
			}
		} else if src[i] == '\r' && (i+1 >= len(src) || src[i+1] != '\n') {
			// Bare CR (not followed by LF) - treat as line ending
			offsets = append(offsets, uint32(i+1))
		}
	}
	return offsets
}

// writeHeader writes the Header to the buffer
func writeHeader(buf []byte, h Header) {
	copy(buf[0:4], h.Magic[:])
	binary.LittleEndian.PutUint16(buf[4:6], h.Version)
	binary.LittleEndian.PutUint16(buf[6:8], h.Flags)
	binary.LittleEndian.PutUint32(buf[8:12], h.EntryCount)
	binary.LittleEndian.PutUint32(buf[12:16], h.DingoLen)
	binary.LittleEndian.PutUint32(buf[16:20], h.GoLen)
	binary.LittleEndian.PutUint32(buf[20:24], h.GoIdxOff)
	binary.LittleEndian.PutUint32(buf[24:28], h.DingoIdxOff)
	binary.LittleEndian.PutUint32(buf[28:32], h.LineIdxOff)
	binary.LittleEndian.PutUint32(buf[32:36], h.KindStrOff)
}

// writeEntry writes an Entry to the buffer
func writeEntry(buf []byte, e Entry) {
	binary.LittleEndian.PutUint32(buf[0:4], e.DingoStart)
	binary.LittleEndian.PutUint32(buf[4:8], e.DingoEnd)
	binary.LittleEndian.PutUint32(buf[8:12], e.GoStart)
	binary.LittleEndian.PutUint32(buf[12:16], e.GoEnd)
	binary.LittleEndian.PutUint16(buf[16:18], e.KindIdx)
	binary.LittleEndian.PutUint16(buf[18:20], e.Reserved)
}

// estimateKindStrSize estimates the size needed for kind strings section
func estimateKindStrSize(kinds []string) int {
	size := 4 // KindCount uint32
	size += len(kinds) * 4 // KindOffsets array
	for _, k := range kinds {
		size += len(k) + 1 // string + null terminator
	}
	return size
}

// writeKindStrings writes the kind strings section and returns final offset
func writeKindStrings(buf []byte, kinds []string) int {
	offset := 0

	// Write KindCount
	binary.LittleEndian.PutUint32(buf[offset:], uint32(len(kinds)))
	offset += 4

	// Write KindOffsets array (offsets relative to start of StringData)
	offsetsStart := offset
	offset += len(kinds) * 4

	// Write StringData
	stringDataStart := offset
	for i, k := range kinds {
		// Write offset for this string
		relOffset := uint32(offset - stringDataStart)
		binary.LittleEndian.PutUint32(buf[offsetsStart+i*4:], relOffset)

		// Write string + null terminator
		copy(buf[offset:], k)
		offset += len(k)
		buf[offset] = 0
		offset++
	}

	return offset
}
