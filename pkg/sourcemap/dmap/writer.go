package dmap

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"

	"github.com/MadAppGang/dingo/pkg/sourcemap"
)

// Writer generates binary .dmap files from line-level source mappings
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

// WriteFile writes .dmap v3 format with line and column mappings directly to a file
func (w *Writer) WriteFile(path string, lineMappings []sourcemap.LineMapping, colMappings []sourcemap.ColumnMapping) error {
	data, err := w.Write(lineMappings, colMappings)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// countLineDirectives counts the number of //line directives in the source
func countLineDirectives(src []byte) uint32 {
	if len(src) == 0 {
		return 0
	}

	count := uint32(0)
	i := 0

	for i < len(src) {
		// Find next newline
		lineEnd := i
		for lineEnd < len(src) && src[lineEnd] != '\n' {
			lineEnd++
		}

		// Check if line starts with //line
		line := src[i:lineEnd]
		if len(line) >= 7 && bytes.Equal(line[0:7], []byte("//line ")) {
			count++
		}

		i = lineEnd + 1
	}

	return count
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
		} else if src[i] == '\r' && (i+1 < len(src) && src[i+1] != '\n') {
			// Bare CR (not followed by LF, and not at EOF) - treat as line ending
			offsets = append(offsets, uint32(i+1))
		}
		// Note: Bare CR at EOF is NOT a line ending
	}
	return offsets
}

// estimateKindStrSize estimates the size needed for kind strings section
func estimateKindStrSize(kinds []string) int {
	size := 4              // KindCount uint32
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

// Write generates .dmap v3 bytes with line and column mappings.
// This is the v3 format with 56-byte header and column mapping support.
func (w *Writer) Write(lineMappings []sourcemap.LineMapping, colMappings []sourcemap.ColumnMapping) ([]byte, error) {
	// Build kind string table from both line and column mappings (deduplicated)
	kindMap := make(map[string]uint16)
	kinds := []string{}

	for _, m := range lineMappings {
		if _, exists := kindMap[m.Kind]; !exists {
			kindMap[m.Kind] = uint16(len(kinds))
			kinds = append(kinds, m.Kind)
		}
	}

	for _, m := range colMappings {
		if _, exists := kindMap[m.Kind]; !exists {
			kindMap[m.Kind] = uint16(len(kinds))
			kinds = append(kinds, m.Kind)
		}
	}

	// Convert LineMapping to LineMappingEntry
	lineEntries := make([]LineMappingEntry, len(lineMappings))
	for i, m := range lineMappings {
		// DingoLineCount: default to 1 if not set (backwards compat with old code)
		dingoLineCount := m.DingoLineCount
		if dingoLineCount <= 0 {
			dingoLineCount = 1
		}
		lineEntries[i] = LineMappingEntry{
			DingoLine:      uint32(m.DingoLine),
			GoLineStart:    uint32(m.GoLineStart),
			GoLineEnd:      uint32(m.GoLineEnd),
			KindIdx:        kindMap[m.Kind],
			DingoLineCount: uint16(dingoLineCount),
		}
	}

	// Convert ColumnMapping to ColumnMappingEntry with overflow validation
	colEntries := make([]ColumnMappingEntry, len(colMappings))
	for i, m := range colMappings {
		// Validate uint16 bounds to prevent silent truncation
		if m.DingoLine > 65535 || m.GoLine > 65535 {
			return nil, fmt.Errorf("column mapping %d: line number exceeds uint16 max (file too large for v3 format)", i)
		}
		if m.DingoCol > 65535 || m.GoCol > 65535 {
			return nil, fmt.Errorf("column mapping %d: column number exceeds uint16 max", i)
		}
		if m.Length > 65535 {
			return nil, fmt.Errorf("column mapping %d: length exceeds uint16 max", i)
		}

		colEntries[i] = ColumnMappingEntry{
			DingoLine: uint16(m.DingoLine),
			DingoCol:  uint16(m.DingoCol),
			GoLine:    uint16(m.GoLine),
			GoCol:     uint16(m.GoCol),
			Length:    uint16(m.Length),
			KindIdx:   kindMap[m.Kind],
			Reserved:  0,
		}
	}

	// Build line offsets for Dingo and Go sources
	dingoLines := buildLineOffsets(w.dingoSrc)
	goLines := buildLineOffsets(w.goSrc)

	// Calculate section offsets for v3 format
	lineIdxOff := uint32(HeaderSize)
	lineIdxSize := 8 + uint32(len(dingoLines))*4 + uint32(len(goLines))*4
	lineMappingOff := lineIdxOff + lineIdxSize
	lineMappingSize := uint32(len(lineEntries)) * LineMappingEntrySize
	columnMappingOff := lineMappingOff + lineMappingSize
	columnMappingSize := uint32(len(colEntries)) * ColumnMappingEntrySize
	kindStrOff := columnMappingOff + columnMappingSize

	// Build v3 header
	flags := uint16(0)
	if len(colMappings) > 0 {
		flags |= FlagHasColumnMappings
	}

	// Count //line directives in Go source
	lineDirectiveCount := countLineDirectives(w.goSrc)

	header := Header{
		Magic:            Magic,
		Version:          Version,
		Flags:            flags,
		DingoLen:         uint32(len(w.dingoSrc)),
		GoLen:            uint32(len(w.goSrc)),
		LineIdxOff:       lineIdxOff,
		DingoLineCnt:     uint32(len(dingoLines)),
		GoLineCnt:        uint32(len(goLines)),
		LineMappingOff:   lineMappingOff,
		LineMappingCnt:   uint32(len(lineEntries)),
		ColumnMappingOff: columnMappingOff,
		ColumnMappingCnt: uint32(len(colEntries)),
		KindStrOff:       kindStrOff,
		LineDirectiveCnt: lineDirectiveCount,
	}

	// Estimate total size
	kindStrSize := estimateKindStrSize(kinds)
	totalSize := int(kindStrOff) + kindStrSize

	// Allocate output buffer
	buf := make([]byte, totalSize)

	// Write header
	writeHeaderV3(buf, header)

	// Write line index section
	offset := int(lineIdxOff)
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

	// Verify we're at the expected line mapping offset
	if offset != int(lineMappingOff) {
		return nil, fmt.Errorf("internal error: writer offset mismatch at line index (expected %d, got %d)", lineMappingOff, offset)
	}

	// Write line mapping entries
	for _, entry := range lineEntries {
		writeLineMappingEntry(buf[offset:], entry)
		offset += LineMappingEntrySize
	}

	// Verify we're at the expected column mapping offset
	if offset != int(columnMappingOff) {
		return nil, fmt.Errorf("internal error: writer offset mismatch at line mapping (expected %d, got %d)", columnMappingOff, offset)
	}

	// Write column mapping entries
	for _, entry := range colEntries {
		writeColumnMappingEntry(buf[offset:], entry)
		offset += ColumnMappingEntrySize
	}

	// Verify we're at the expected kind strings offset
	if offset != int(kindStrOff) {
		return nil, fmt.Errorf("internal error: writer offset mismatch at column mapping (expected %d, got %d)", kindStrOff, offset)
	}

	// Write kind strings
	finalOffset := writeKindStrings(buf[offset:], kinds)

	// Return exact buffer
	return buf[:offset+finalOffset], nil
}

// writeHeaderV3 writes the v3 Header (56 bytes) to the buffer
func writeHeaderV3(buf []byte, h Header) {
	copy(buf[0:4], h.Magic[:])
	binary.LittleEndian.PutUint16(buf[4:6], h.Version)
	binary.LittleEndian.PutUint16(buf[6:8], h.Flags)
	binary.LittleEndian.PutUint32(buf[8:12], h.DingoLen)
	binary.LittleEndian.PutUint32(buf[12:16], h.GoLen)
	binary.LittleEndian.PutUint32(buf[16:20], h.LineIdxOff)
	binary.LittleEndian.PutUint32(buf[20:24], h.DingoLineCnt)
	binary.LittleEndian.PutUint32(buf[24:28], h.GoLineCnt)
	binary.LittleEndian.PutUint32(buf[28:32], h.LineMappingOff)
	binary.LittleEndian.PutUint32(buf[32:36], h.LineMappingCnt)
	binary.LittleEndian.PutUint32(buf[36:40], h.ColumnMappingOff)
	binary.LittleEndian.PutUint32(buf[40:44], h.ColumnMappingCnt)
	binary.LittleEndian.PutUint32(buf[44:48], h.KindStrOff)
	binary.LittleEndian.PutUint32(buf[48:52], h.LineDirectiveCnt)
	// Zero-initialize reserved bytes for deterministic output
	buf[52] = 0
	buf[53] = 0
	buf[54] = 0
	buf[55] = 0
}

// writeLineMappingEntry writes a LineMappingEntry to the buffer
func writeLineMappingEntry(buf []byte, e LineMappingEntry) {
	binary.LittleEndian.PutUint32(buf[0:4], e.DingoLine)
	binary.LittleEndian.PutUint32(buf[4:8], e.GoLineStart)
	binary.LittleEndian.PutUint32(buf[8:12], e.GoLineEnd)
	binary.LittleEndian.PutUint16(buf[12:14], e.KindIdx)
	binary.LittleEndian.PutUint16(buf[14:16], e.DingoLineCount)
}

// writeColumnMappingEntry writes a ColumnMappingEntry to the buffer
func writeColumnMappingEntry(buf []byte, e ColumnMappingEntry) {
	binary.LittleEndian.PutUint16(buf[0:2], e.DingoLine)
	binary.LittleEndian.PutUint16(buf[2:4], e.DingoCol)
	binary.LittleEndian.PutUint16(buf[4:6], e.GoLine)
	binary.LittleEndian.PutUint16(buf[6:8], e.GoCol)
	binary.LittleEndian.PutUint16(buf[8:10], e.Length)
	binary.LittleEndian.PutUint16(buf[10:12], e.KindIdx)
	binary.LittleEndian.PutUint32(buf[12:16], e.Reserved)
}
