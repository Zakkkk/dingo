package dmap

import (
	"encoding/binary"
	"fmt"
	"os"
	"sort"
	"sync"
)

// Reader provides thread-safe access to .dmap binary source map files.
// It loads the entire file into memory and provides efficient O(log N)
// bidirectional position lookups using dual indexes.
type Reader struct {
	mu sync.RWMutex // Protects all fields for thread-safe access

	data []byte // Raw file contents (kept for potential future mmap)
	hdr  Header // Parsed file header

	// Dual indexes for O(log N) bidirectional lookup
	goEntries    []Entry // Sorted by GoStart (for Go->Dingo lookup)
	dingoEntries []Entry // Sorted by DingoStart (for Dingo->Go lookup)

	// Line offset arrays for byte<->line conversion
	dingoLines []uint32 // Byte offset of each line start in .dingo
	goLines    []uint32 // Byte offset of each line start in .go

	// Decoded kind strings
	kinds []string // Kind strings indexed by KindIdx

	// Line-level mappings (v2 format)
	lineMappings []LineMappingEntry // Line mappings for v2 format
}

// Open reads and parses a .dmap file from disk.
// Returns an error if the file cannot be read or is invalid.
func Open(path string) (*Reader, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read .dmap file: %w", err)
	}
	return OpenBytes(data)
}

// OpenBytes parses a .dmap file from in-memory bytes.
// Useful for testing and when the file is already loaded.
func OpenBytes(data []byte) (*Reader, error) {
	r := &Reader{data: data}

	if err := r.parseHeader(); err != nil {
		return nil, err
	}

	if err := r.parseIndexes(); err != nil {
		return nil, err
	}

	if err := r.parseLineIndex(); err != nil {
		return nil, err
	}

	if err := r.parseKindStrings(); err != nil {
		return nil, err
	}

	// Parse line mappings if v2 format
	if err := r.parseLineMappings(); err != nil {
		return nil, err
	}

	return r, nil
}

// Close releases resources. Currently a no-op, but future-proofs for mmap.
func (r *Reader) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.data = nil
	r.goEntries = nil
	r.dingoEntries = nil
	r.dingoLines = nil
	r.goLines = nil
	r.kinds = nil
	r.lineMappings = nil
	return nil
}

// parseHeader reads and validates the file header (36 bytes for v1, 44 bytes for v2).
func (r *Reader) parseHeader() error {
	if len(r.data) < HeaderSize {
		return ErrCorruptedFile
	}

	data := r.data[:HeaderSize]

	// Read magic bytes
	copy(r.hdr.Magic[:], data[0:4])
	if r.hdr.Magic != Magic {
		return ErrInvalidMagic
	}

	// Read version
	r.hdr.Version = binary.LittleEndian.Uint16(data[4:6])
	if r.hdr.Version > Version {
		return ErrUnsupportedVer
	}

	// Read remaining header fields
	r.hdr.Flags = binary.LittleEndian.Uint16(data[6:8])
	r.hdr.EntryCount = binary.LittleEndian.Uint32(data[8:12])
	r.hdr.DingoLen = binary.LittleEndian.Uint32(data[12:16])
	r.hdr.GoLen = binary.LittleEndian.Uint32(data[16:20])
	r.hdr.GoIdxOff = binary.LittleEndian.Uint32(data[20:24])
	r.hdr.DingoIdxOff = binary.LittleEndian.Uint32(data[24:28])
	r.hdr.LineIdxOff = binary.LittleEndian.Uint32(data[28:32])
	r.hdr.KindStrOff = binary.LittleEndian.Uint32(data[32:36])

	// Read v2-specific fields if version >= 2
	if r.hdr.Version >= 2 {
		r.hdr.LineMappingOff = binary.LittleEndian.Uint32(data[36:40])
		r.hdr.LineMappingCnt = binary.LittleEndian.Uint32(data[40:44])
	}

	return nil
}

// parseIndexes reads both the Go index and Dingo index sections.
func (r *Reader) parseIndexes() error {
	// v2 format has no token-level indexes (EntryCount = 0)
	if r.hdr.Version >= 2 && r.hdr.EntryCount == 0 {
		r.goEntries = []Entry{}
		r.dingoEntries = []Entry{}
		return nil
	}

	entryCount := int(r.hdr.EntryCount)

	// Parse Go index (sorted by GoStart)
	goIdxStart := int(r.hdr.GoIdxOff)
	goIdxEnd := goIdxStart + entryCount*EntrySize
	if goIdxEnd > len(r.data) {
		return ErrCorruptedFile
	}
	r.goEntries = make([]Entry, entryCount)
	if err := r.parseEntries(r.data[goIdxStart:goIdxEnd], r.goEntries); err != nil {
		return err
	}

	// Parse Dingo index (sorted by DingoStart)
	dingoIdxStart := int(r.hdr.DingoIdxOff)
	dingoIdxEnd := dingoIdxStart + entryCount*EntrySize
	if dingoIdxEnd > len(r.data) {
		return ErrCorruptedFile
	}
	r.dingoEntries = make([]Entry, entryCount)
	if err := r.parseEntries(r.data[dingoIdxStart:dingoIdxEnd], r.dingoEntries); err != nil {
		return err
	}

	// CRITICAL FIX I1: Validate entries are properly sorted
	// Binary search depends on this invariant, so fail fast if violated
	for i := 1; i < len(r.goEntries); i++ {
		if r.goEntries[i].GoStart < r.goEntries[i-1].GoStart {
			return fmt.Errorf("dmap: Go index not sorted at entry %d", i)
		}
	}
	for i := 1; i < len(r.dingoEntries); i++ {
		if r.dingoEntries[i].DingoStart < r.dingoEntries[i-1].DingoStart {
			return fmt.Errorf("dmap: Dingo index not sorted at entry %d", i)
		}
	}

	return nil
}

// parseEntries decodes a slice of Entry structs from raw bytes.
func (r *Reader) parseEntries(data []byte, entries []Entry) error {
	if len(data) != len(entries)*EntrySize {
		return ErrCorruptedFile
	}

	for i := range entries {
		offset := i * EntrySize
		if offset+EntrySize > len(data) {
			return ErrCorruptedFile
		}

		entries[i].DingoStart = binary.LittleEndian.Uint32(data[offset:offset+4])
		entries[i].DingoEnd = binary.LittleEndian.Uint32(data[offset+4:offset+8])
		entries[i].GoStart = binary.LittleEndian.Uint32(data[offset+8:offset+12])
		entries[i].GoEnd = binary.LittleEndian.Uint32(data[offset+12:offset+16])
		entries[i].KindIdx = binary.LittleEndian.Uint16(data[offset+16:offset+18])
		entries[i].Reserved = binary.LittleEndian.Uint16(data[offset+18:offset+20])
	}
	return nil
}

// parseLineIndex reads the line offset arrays for both .dingo and .go files.
func (r *Reader) parseLineIndex() error {
	lineIdxStart := int(r.hdr.LineIdxOff)
	if lineIdxStart+8 > len(r.data) {
		return ErrCorruptedFile
	}

	data := r.data[lineIdxStart:]

	// Read line counts
	dingoLineCount := binary.LittleEndian.Uint32(data[0:4])
	goLineCount := binary.LittleEndian.Uint32(data[4:8])

	// Read Dingo line offsets
	r.dingoLines = make([]uint32, dingoLineCount)
	dingoOffset := 8
	for i := range r.dingoLines {
		if dingoOffset+4 > len(data) {
			return ErrCorruptedFile
		}
		r.dingoLines[i] = binary.LittleEndian.Uint32(data[dingoOffset : dingoOffset+4])
		dingoOffset += 4
	}

	// Read Go line offsets
	r.goLines = make([]uint32, goLineCount)
	for i := range r.goLines {
		if dingoOffset+4 > len(data) {
			return ErrCorruptedFile
		}
		r.goLines[i] = binary.LittleEndian.Uint32(data[dingoOffset : dingoOffset+4])
		dingoOffset += 4
	}

	return nil
}

// parseKindStrings reads and decodes the kind string table.
func (r *Reader) parseKindStrings() error {
	kindStrStart := int(r.hdr.KindStrOff)
	if kindStrStart+4 > len(r.data) {
		return ErrCorruptedFile
	}

	data := r.data[kindStrStart:]

	// Read kind count
	kindCount := binary.LittleEndian.Uint32(data[0:4])

	// Read kind offsets
	kindOffsets := make([]uint32, kindCount)
	offset := 4
	for i := range kindOffsets {
		if offset+4 > len(data) {
			return ErrCorruptedFile
		}
		kindOffsets[i] = binary.LittleEndian.Uint32(data[offset : offset+4])
		offset += 4
	}

	// Read string data (null-terminated strings)
	stringDataStart := kindStrStart + 4 + int(kindCount)*4
	stringData := r.data[stringDataStart:]

	// Decode strings
	r.kinds = make([]string, kindCount)
	for i, offset := range kindOffsets {
		start := int(offset)
		if start >= len(stringData) {
			return ErrCorruptedFile
		}
		// Find null terminator with max length check (prevent infinite loop)
		end := start
		maxEnd := start + 1024 // Reasonable max string length
		for end < len(stringData) && end < maxEnd && stringData[end] != 0 {
			end++
		}
		if end >= len(stringData) || end >= maxEnd {
			return fmt.Errorf("%w: kind string %d missing null terminator or exceeds max length", ErrCorruptedFile, i)
		}
		r.kinds[i] = string(stringData[start:end])
	}

	return nil
}

// parseLineMappings reads the line mapping section (v2 format only).
// Returns nil if version < 2 or no line mappings exist.
func (r *Reader) parseLineMappings() error {
	// Skip if not v2 or no line mappings
	if r.hdr.Version < 2 || r.hdr.LineMappingCnt == 0 {
		return nil
	}

	lineMappingStart := int(r.hdr.LineMappingOff)
	entryCount := int(r.hdr.LineMappingCnt)
	lineMappingEnd := lineMappingStart + entryCount*LineMappingEntrySize

	if lineMappingEnd > len(r.data) {
		return ErrCorruptedFile
	}

	// Allocate and parse line mapping entries
	r.lineMappings = make([]LineMappingEntry, entryCount)
	data := r.data[lineMappingStart:lineMappingEnd]

	for i := range r.lineMappings {
		offset := i * LineMappingEntrySize
		if offset+LineMappingEntrySize > len(data) {
			return ErrCorruptedFile
		}

		r.lineMappings[i].DingoLine = binary.LittleEndian.Uint32(data[offset : offset+4])
		r.lineMappings[i].GoLineStart = binary.LittleEndian.Uint32(data[offset+4 : offset+8])
		r.lineMappings[i].GoLineEnd = binary.LittleEndian.Uint32(data[offset+8 : offset+12])
		r.lineMappings[i].KindIdx = binary.LittleEndian.Uint16(data[offset+12 : offset+14])
		r.lineMappings[i].Reserved = binary.LittleEndian.Uint16(data[offset+14 : offset+16])
	}

	// Sort by GoLineStart for efficient Go→Dingo lookups
	sort.Slice(r.lineMappings, func(i, j int) bool {
		return r.lineMappings[i].GoLineStart < r.lineMappings[j].GoLineStart
	})

	return nil
}

// FindByGoPos finds the mapping containing the given Go byte offset.
// Returns (dingoStart, dingoEnd, kind) if found, or identity mapping if not found.
// Uses binary search on the Go index for O(log N) lookup.
func (r *Reader) FindByGoPos(goByteOffset int) (dingoStart, dingoEnd int, kind string) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Binary search for entry containing goByteOffset
	idx := sort.Search(len(r.goEntries), func(i int) bool {
		return r.goEntries[i].GoEnd > uint32(goByteOffset)
	})

	// Check if found and offset is within range
	if idx < len(r.goEntries) {
		entry := &r.goEntries[idx]
		if uint32(goByteOffset) >= entry.GoStart && uint32(goByteOffset) < entry.GoEnd {
			// Found mapping - return range boundaries
			dingoStart = int(entry.DingoStart)
			dingoEnd = int(entry.DingoEnd)
			if entry.KindIdx < uint16(len(r.kinds)) {
				kind = r.kinds[entry.KindIdx]
			}
			return
		}
	}

	// Not found - return identity mapping
	return goByteOffset, goByteOffset, ""
}

// FindByDingoPos finds the mapping containing the given Dingo byte offset.
// Returns (goStart, goEnd, kind) if found, or identity mapping if not found.
// Uses binary search on the Dingo index for O(log N) lookup.
func (r *Reader) FindByDingoPos(dingoByteOffset int) (goStart, goEnd int, kind string) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Binary search for entry containing dingoByteOffset
	idx := sort.Search(len(r.dingoEntries), func(i int) bool {
		return r.dingoEntries[i].DingoEnd > uint32(dingoByteOffset)
	})

	// Check if found and offset is within range
	if idx < len(r.dingoEntries) {
		entry := &r.dingoEntries[idx]
		if uint32(dingoByteOffset) >= entry.DingoStart && uint32(dingoByteOffset) < entry.DingoEnd {
			// Found mapping - return range boundaries
			goStart = int(entry.GoStart)
			goEnd = int(entry.GoEnd)
			if entry.KindIdx < uint16(len(r.kinds)) {
				kind = r.kinds[entry.KindIdx]
			}
			return
		}
	}

	// Not found - return identity mapping
	return dingoByteOffset, dingoByteOffset, ""
}

// GoByteToLine converts a Go byte offset to a 1-indexed line number.
// Returns 0 if offset is out of range.
func (r *Reader) GoByteToLine(byteOffset int) int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if byteOffset < 0 || byteOffset > int(r.hdr.GoLen) {
		return 0
	}

	// Binary search for line containing offset
	idx := sort.Search(len(r.goLines), func(i int) bool {
		return r.goLines[i] > uint32(byteOffset)
	})

	// idx is now the first line that starts AFTER byteOffset
	// So the line containing byteOffset is idx-1 (but line numbers are 1-indexed)
	if idx == 0 {
		return 1 // First line
	}
	return idx
}

// DingoByteToLine converts a Dingo byte offset to a 1-indexed line number.
// Returns 0 if offset is out of range.
func (r *Reader) DingoByteToLine(byteOffset int) int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if byteOffset < 0 || byteOffset > int(r.hdr.DingoLen) {
		return 0
	}

	// Binary search for line containing offset
	idx := sort.Search(len(r.dingoLines), func(i int) bool {
		return r.dingoLines[i] > uint32(byteOffset)
	})

	// idx is now the first line that starts AFTER byteOffset
	if idx == 0 {
		return 1 // First line
	}
	return idx
}

// GoLineToByteOffset converts a 1-indexed Go line number to byte offset.
// Returns -1 if line number is out of range.
func (r *Reader) GoLineToByteOffset(line int) int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if line < 1 || line > len(r.goLines) {
		return -1
	}
	return int(r.goLines[line-1])
}

// DingoLineToByteOffset converts a 1-indexed Dingo line number to byte offset.
// Returns -1 if line number is out of range.
func (r *Reader) DingoLineToByteOffset(line int) int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if line < 1 || line > len(r.dingoLines) {
		return -1
	}
	return int(r.dingoLines[line-1])
}

// GoLineToDingoLine converts a Go line to Dingo line using v2 line mappings.
// Returns the Dingo line and kind string if a mapping is found.
// Returns (goLine, "") as identity mapping if no mapping found or v1 format.
// Uses binary search for O(log N) performance instead of O(N) linear search.
func (r *Reader) GoLineToDingoLine(goLine int) (dingoLine int, kind string) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Return identity if no v2 mappings available
	if len(r.lineMappings) == 0 {
		return goLine, ""
	}

	// Binary search for mapping containing goLine
	// Mappings are sorted by GoLineStart in parseLineMappings
	idx := sort.Search(len(r.lineMappings), func(i int) bool {
		return r.lineMappings[i].GoLineEnd >= uint32(goLine)
	})

	// Check if we found a valid mapping
	if idx < len(r.lineMappings) {
		entry := &r.lineMappings[idx]
		if uint32(goLine) >= entry.GoLineStart && uint32(goLine) <= entry.GoLineEnd {
			// Found mapping - return Dingo line and kind
			dingoLine = int(entry.DingoLine)
			if entry.KindIdx < uint16(len(r.kinds)) {
				kind = r.kinds[entry.KindIdx]
			}
			return dingoLine, kind
		}
	}

	// No mapping found - return identity
	return goLine, ""
}

// Header returns a copy of the file header.
func (r *Reader) Header() Header {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.hdr
}

// EntryCount returns the number of mapping entries.
func (r *Reader) EntryCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.goEntries)
}

// KindCount returns the number of unique kind strings.
func (r *Reader) KindCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.kinds)
}

// CalculateLineShift calculates the cumulative line shift at a given Dingo line.
// For v2 format: Uses line mappings to compute shift efficiently.
// For v1 format: Falls back to byte-level computation (legacy).
// Returns the number of lines to ADD to the Dingo line number to get the Go line number.
func (r *Reader) CalculateLineShift(dingoLine int) int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// V2 format: Use line mappings for efficient calculation
	if len(r.lineMappings) > 0 {
		totalShift := 0
		for _, m := range r.lineMappings {
			// Only count mappings BEFORE the target line
			if int(m.DingoLine) < dingoLine {
				// Each mapping spans (GoLineEnd - GoLineStart + 1) Go lines
				// but represents 1 Dingo line
				goLines := int(m.GoLineEnd - m.GoLineStart + 1)
				dingoLines := 1
				totalShift += (goLines - dingoLines)
			}
		}
		return totalShift
	}

	// V1 format fallback: Convert line to byte offset and use legacy algorithm
	dingoByteOffset := r.DingoLineToByteOffset(dingoLine)
	if dingoByteOffset < 0 {
		return 0
	}

	totalShift := 0
	// Iterate through all byte-level mappings
	for _, entry := range r.dingoEntries {
		// Only consider mappings that END before our position
		if int(entry.DingoEnd) <= dingoByteOffset {
			// Count lines in the Dingo range
			dingoLineCount := r.countLinesInRangeUint32(r.dingoLines, int(entry.DingoStart), int(entry.DingoEnd))
			// Count lines in the Go range
			goLineCount := r.countLinesInRangeUint32(r.goLines, int(entry.GoStart), int(entry.GoEnd))

			// Add the difference to the total shift
			totalShift += (goLineCount - dingoLineCount)
		}
	}

	return totalShift
}

// countLinesInRangeUint32 counts how many line boundaries are crossed in a byte range.
// Returns 1 for a range on a single line, 2 for a range spanning 2 lines, etc.
func (r *Reader) countLinesInRangeUint32(lineOffsets []uint32, start, end int) int {
	if len(lineOffsets) == 0 {
		return 1
	}

	startLine := 1
	endLine := 1

	for i, offset := range lineOffsets {
		if int(offset) <= start {
			startLine = i + 1
		}
		if int(offset) <= end {
			endLine = i + 1
		} else {
			break
		}
	}

	return endLine - startLine + 1
}
