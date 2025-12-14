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
// position lookups using line and column mappings.
type Reader struct {
	mu sync.RWMutex // Protects all fields for thread-safe access

	data []byte // Raw file contents (kept for potential future mmap)
	hdr  Header // Parsed file header

	// Line offset arrays for byte<->line conversion
	dingoLines []uint32 // Byte offset of each line start in .dingo
	goLines    []uint32 // Byte offset of each line start in .go

	// Decoded kind strings
	kinds []string // Kind strings indexed by KindIdx

	// Line-level mappings (v3 format)
	lineMappings []LineMappingEntry // Line mappings for v3 format

	// Column-level mappings (v3 format)
	columnMappings []ColumnMappingEntry // Column mappings for v3 format
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

// OpenBytes parses a .dmap v3 file from in-memory bytes.
// Returns ErrMigrationRequired for v1/v2 files (migration required).
func OpenBytes(data []byte) (*Reader, error) {
	r := &Reader{data: data}

	if err := r.parseHeader(); err != nil {
		return nil, err
	}

	if err := r.parseLineIndex(); err != nil {
		return nil, err
	}

	if err := r.parseKindStrings(); err != nil {
		return nil, err
	}

	// Parse line mappings (v3 format)
	if err := r.parseLineMappings(); err != nil {
		return nil, err
	}

	// Parse column mappings (v3 format)
	if err := r.parseColumnMappings(); err != nil {
		return nil, err
	}

	return r, nil
}

// Close releases resources. Currently a no-op, but future-proofs for mmap.
func (r *Reader) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.data = nil
	r.dingoLines = nil
	r.goLines = nil
	r.kinds = nil
	r.lineMappings = nil
	r.columnMappings = nil
	return nil
}

// parseHeader reads and validates the v3 file header (56 bytes).
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
	if r.hdr.Version < 3 {
		return ErrMigrationRequired
	}
	if r.hdr.Version > Version {
		return ErrUnsupportedVer
	}

	// Read v3 header fields
	r.hdr.Flags = binary.LittleEndian.Uint16(data[6:8])
	r.hdr.DingoLen = binary.LittleEndian.Uint32(data[8:12])
	r.hdr.GoLen = binary.LittleEndian.Uint32(data[12:16])
	r.hdr.LineIdxOff = binary.LittleEndian.Uint32(data[16:20])
	r.hdr.DingoLineCnt = binary.LittleEndian.Uint32(data[20:24])
	r.hdr.GoLineCnt = binary.LittleEndian.Uint32(data[24:28])
	r.hdr.LineMappingOff = binary.LittleEndian.Uint32(data[28:32])
	r.hdr.LineMappingCnt = binary.LittleEndian.Uint32(data[32:36])
	r.hdr.ColumnMappingOff = binary.LittleEndian.Uint32(data[36:40])
	r.hdr.ColumnMappingCnt = binary.LittleEndian.Uint32(data[40:44])
	r.hdr.KindStrOff = binary.LittleEndian.Uint32(data[44:48])
	r.hdr.LineDirectiveCnt = binary.LittleEndian.Uint32(data[48:52])
	// bytes 52-56 reserved

	return nil
}

// parseLineIndex reads the line offset arrays for both .dingo and .go files.
func (r *Reader) parseLineIndex() error {
	const MaxReasonableLineCount = 10_000_000 // ~10M lines

	lineIdxStart := int(r.hdr.LineIdxOff)
	if lineIdxStart+8 > len(r.data) {
		return ErrCorruptedFile
	}

	data := r.data[lineIdxStart:]

	// Read line counts
	dingoLineCount := binary.LittleEndian.Uint32(data[0:4])
	goLineCount := binary.LittleEndian.Uint32(data[4:8])

	// Sanity check to prevent gigabyte allocations from corrupted files
	if dingoLineCount > MaxReasonableLineCount || goLineCount > MaxReasonableLineCount {
		return fmt.Errorf("%w: line count exceeds sanity limit (%d or %d > %d)",
			ErrCorruptedFile, dingoLineCount, goLineCount, MaxReasonableLineCount)
	}

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
	const MaxReasonableKindCount = 1_000 // 1000 transform types

	kindStrStart := int(r.hdr.KindStrOff)
	if kindStrStart+4 > len(r.data) {
		return ErrCorruptedFile
	}

	data := r.data[kindStrStart:]

	// Read kind count
	kindCount := binary.LittleEndian.Uint32(data[0:4])

	// Sanity check to prevent excessive allocations
	if kindCount > MaxReasonableKindCount {
		return fmt.Errorf("%w: kind count exceeds sanity limit (%d > %d)",
			ErrCorruptedFile, kindCount, MaxReasonableKindCount)
	}

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
	if stringDataStart > len(r.data) {
		return fmt.Errorf("%w: kind string section beyond file end", ErrCorruptedFile)
	}
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

// parseLineMappings reads the line mapping section (v3 format).
// Returns nil if no line mappings exist.
func (r *Reader) parseLineMappings() error {
	if r.hdr.LineMappingCnt == 0 {
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

// parseColumnMappings reads the column mapping section (v3 format).
// Returns nil if no column mappings exist.
func (r *Reader) parseColumnMappings() error {
	if r.hdr.ColumnMappingCnt == 0 {
		return nil
	}

	colMappingStart := int(r.hdr.ColumnMappingOff)
	entryCount := int(r.hdr.ColumnMappingCnt)
	colMappingEnd := colMappingStart + entryCount*ColumnMappingEntrySize

	if colMappingEnd > len(r.data) {
		return ErrCorruptedFile
	}

	// Allocate and parse column mapping entries
	r.columnMappings = make([]ColumnMappingEntry, entryCount)
	data := r.data[colMappingStart:colMappingEnd]

	for i := range r.columnMappings {
		offset := i * ColumnMappingEntrySize
		if offset+ColumnMappingEntrySize > len(data) {
			return ErrCorruptedFile
		}

		r.columnMappings[i].DingoLine = binary.LittleEndian.Uint16(data[offset : offset+2])
		r.columnMappings[i].DingoCol = binary.LittleEndian.Uint16(data[offset+2 : offset+4])
		r.columnMappings[i].GoLine = binary.LittleEndian.Uint16(data[offset+4 : offset+6])
		r.columnMappings[i].GoCol = binary.LittleEndian.Uint16(data[offset+6 : offset+8])
		r.columnMappings[i].Length = binary.LittleEndian.Uint16(data[offset+8 : offset+10])
		r.columnMappings[i].KindIdx = binary.LittleEndian.Uint16(data[offset+10 : offset+12])
		r.columnMappings[i].Reserved = binary.LittleEndian.Uint32(data[offset+12 : offset+16])
	}

	return nil
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

// DingoLineLength returns the length of a 1-indexed Dingo line in bytes.
// This is useful for clamping columns to valid positions.
// Returns -1 if line number is out of range.
func (r *Reader) DingoLineLength(line int) int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if line < 1 || line > len(r.dingoLines) {
		return -1
	}

	lineStart := int(r.dingoLines[line-1])
	var lineEnd int
	if line < len(r.dingoLines) {
		lineEnd = int(r.dingoLines[line])
	} else {
		// Last line - use DingoLen from header
		lineEnd = int(r.hdr.DingoLen)
	}

	// Subtract 1 for newline character (if present)
	length := lineEnd - lineStart
	if length > 0 {
		length-- // Exclude newline
	}
	return length
}

// GoLineToDingoLine converts a Go line to Dingo line using v2 line mappings.
// Returns the Dingo line and kind string if a mapping is found.
// For unmapped lines, uses cumulative delta to compute approximate Dingo line.
// Uses binary search for O(log N) performance instead of O(N) linear search.
func (r *Reader) GoLineToDingoLine(goLine int) (dingoLine int, kind string) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// If no v2 mappings available, use simple proportional fallback
	if len(r.lineMappings) == 0 {
		return r.proportionalFallback(goLine), ""
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
			if entry.KindIdx >= uint16(len(r.kinds)) {
				// Invalid kind index - return empty kind
				return dingoLine, ""
			}
			kind = r.kinds[entry.KindIdx]
			return dingoLine, kind
		}
	}

	// No direct mapping found - compute using cumulative delta from previous mappings
	return r.computeDeltaFallback(goLine, idx), ""
}

// proportionalFallback returns a proportionally mapped Dingo line when no v2 mappings exist.
// Uses the ratio of Dingo/Go line counts for approximate mapping.
func (r *Reader) proportionalFallback(goLine int) int {
	goLineCount := len(r.goLines)
	dingoLineCount := len(r.dingoLines)

	if goLineCount == 0 || dingoLineCount == 0 {
		return goLine
	}

	// Clamp goLine to valid range
	if goLine < 1 {
		return 1
	}
	if goLine > goLineCount {
		goLine = goLineCount
	}

	// Proportional mapping: dingoLine ≈ goLine * (dingoCount / goCount)
	// Use integer math to avoid float precision issues
	dingoLine := (goLine * dingoLineCount) / goLineCount
	if dingoLine < 1 {
		dingoLine = 1
	}
	if dingoLine > dingoLineCount {
		dingoLine = dingoLineCount
	}
	return dingoLine
}

// computeDeltaFallback computes Dingo line using cumulative delta from mappings.
// For a Go line between mappings, we find the cumulative line delta from all
// previous transforms and subtract it from the Go line.
//
// The algorithm:
// 1. For lines BEFORE the first mapping: use the first mapping to derive header offset
// 2. For lines BETWEEN or AFTER mappings: track cumulative delta from passed transforms
func (r *Reader) computeDeltaFallback(goLine int, searchIdx int) int {
	goLineCount := len(r.goLines)
	dingoLineCount := len(r.dingoLines)

	if dingoLineCount == 0 {
		return goLine
	}

	// Sanity check: if goLine > goLineCount, use proportional
	if goLine > goLineCount {
		return r.proportionalFallback(goLine)
	}

	// If no mappings, use identity
	if len(r.lineMappings) == 0 {
		return goLine
	}

	firstMapping := &r.lineMappings[0]

	// For lines BEFORE the first mapping, derive offset from the first mapping
	// First mapping tells us: GoLineStart -> DingoLine
	// So for lines before that, offset = GoLineStart - DingoLine
	if goLine < int(firstMapping.GoLineStart) {
		// Header offset: how many more Go lines than Dingo lines before first transform
		headerOffset := int(firstMapping.GoLineStart) - int(firstMapping.DingoLine)
		dingoLine := goLine - headerOffset

		// Clamp to valid range
		if dingoLine < 1 {
			dingoLine = 1
		}
		if dingoLine > dingoLineCount {
			dingoLine = dingoLineCount
		}
		return dingoLine
	}

	// For lines AT or AFTER the first mapping, track cumulative delta
	cumulativeDelta := 0

	for i := 0; i < len(r.lineMappings); i++ {
		entry := &r.lineMappings[i]
		if int(entry.GoLineEnd) < goLine {
			// This mapping is entirely before goLine
			goLinesInMapping := int(entry.GoLineEnd) - int(entry.GoLineStart) + 1
			cumulativeDelta += goLinesInMapping - 1
		} else if goLine < int(entry.GoLineStart) {
			// goLine is in a gap between this mapping and the previous
			// Use the last mapping's end to compute offset
			break
		} else {
			// goLine is inside this mapping - but that should have been caught
			// by the caller, so this shouldn't happen. Return DingoLine.
			return int(entry.DingoLine)
		}
	}

	// Compute based on cumulative delta
	// headerOffset derived from first mapping
	headerOffset := int(firstMapping.GoLineStart) - int(firstMapping.DingoLine)
	dingoLine := goLine - headerOffset - cumulativeDelta

	// Clamp to valid range
	if dingoLine < 1 {
		dingoLine = 1
	}
	if dingoLine > dingoLineCount {
		dingoLine = dingoLineCount
	}

	return dingoLine
}

// DingoLineToGoLine converts a 1-indexed Dingo line to the corresponding Go line.
// For transformed lines (error propagation, match, etc.), returns GoLineStart where
// the actual code is located (not the //line directive which is at GoLineStart-1).
// For untransformed lines, uses CalculateLineShift for proper offset calculation.
func (r *Reader) DingoLineToGoLine(dingoLine int) int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Check if this Dingo line has a direct mapping (is a transformed line)
	for _, m := range r.lineMappings {
		if int(m.DingoLine) == dingoLine {
			// Found direct mapping - return GoLineStart where the code is
			// The //line directive is at GoLineStart-1, but we want the actual code
			return int(m.GoLineStart)
		}
	}

	// No direct mapping - use line shift calculation
	baseOffset := r.calculateBaseOffset()
	transformShift := 0
	for _, m := range r.lineMappings {
		if int(m.DingoLine) < dingoLine {
			// Each transform expansion adds extra Go lines:
			// - 1 line for the //line directive (at GoLineStart - 1)
			// - (GoLineEnd - GoLineStart) extra lines beyond the 1 Dingo line
			goLines := int(m.GoLineEnd - m.GoLineStart + 1)
			dingoLines := 1
			directiveLine := 1 // The //line directive adds one more Go line
			transformShift += (goLines - dingoLines) + directiveLine
		}
	}
	return dingoLine + baseOffset + transformShift
}

// IsTransformedLine returns true if the given Dingo line has a direct transformation mapping.
// Transformed lines have different code structure and require special column handling.
func (r *Reader) IsTransformedLine(dingoLine int) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, m := range r.lineMappings {
		if int(m.DingoLine) == dingoLine {
			return true
		}
	}
	return false
}

// Header returns a copy of the file header.
func (r *Reader) Header() Header {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.hdr
}

// LineMappingCount returns the number of line mapping entries.
func (r *Reader) LineMappingCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.lineMappings)
}

// ColumnMappingCount returns the number of column mapping entries.
func (r *Reader) ColumnMappingCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.columnMappings)
}

// KindCount returns the number of unique kind strings.
func (r *Reader) KindCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.kinds)
}

// CalculateLineShift calculates the cumulative line shift at a given Dingo line.
// Returns the number of lines to ADD to the Dingo line number to get the Go line number.
func (r *Reader) CalculateLineShift(dingoLine int) int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// V3 format: Use line mappings for efficient calculation
	// First, calculate base offset from header differences
	// (e.g., if Go removed an empty comment, lines shift by -1)
	baseOffset := r.calculateBaseOffset()

	// Then add shifts from transformed code expansions
	transformShift := 0
	for _, m := range r.lineMappings {
		// Only count mappings BEFORE the target line
		if int(m.DingoLine) < dingoLine {
			// Each transform expansion adds extra Go lines:
			// - 1 line for the //line directive (at GoLineStart - 1)
			// - (GoLineEnd - GoLineStart) extra lines beyond the 1 Dingo line
			goLines := int(m.GoLineEnd - m.GoLineStart + 1)
			dingoLines := 1
			directiveLine := 1 // The //line directive adds one more Go line
			transformShift += (goLines - dingoLines) + directiveLine
		}
	}
	return baseOffset + transformShift
}

// TranslateDingoColumn translates a Dingo column to Go column for transformed lines.
// Takes 1-indexed line and column. Returns the translated Go column and whether a mapping was found.
// If no column mapping applies, returns the original column unchanged.
func (r *Reader) TranslateDingoColumn(dingoLine, dingoCol int) (goCol int, found bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Look for a column mapping that contains this position
	for _, m := range r.columnMappings {
		if int(m.DingoLine) == dingoLine {
			// Check if column falls within the mapped expression range
			// The mapping covers [DingoCol, DingoCol + Length)
			if dingoCol >= int(m.DingoCol) && dingoCol < int(m.DingoCol)+int(m.Length) {
				// Translate using the column offset
				// offset = GoCol - DingoCol (positive means Go has more leading chars)
				offset := int(m.GoCol) - int(m.DingoCol)
				return dingoCol + offset, true
			}
		}
	}
	return dingoCol, false
}

// TranslateGoColumn translates a Go column to Dingo column for transformed lines.
// Takes 1-indexed line and column. Returns the translated Dingo column and whether a mapping was found.
// If no column mapping applies, returns the original column unchanged.
func (r *Reader) TranslateGoColumn(goLine, goCol int) (dingoCol int, found bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Look for a column mapping that contains this position
	for _, m := range r.columnMappings {
		if int(m.GoLine) == goLine {
			// Check if column falls within the mapped expression range
			// The mapping covers [GoCol, GoCol + Length)
			if goCol >= int(m.GoCol) && goCol < int(m.GoCol)+int(m.Length) {
				// Translate using the column offset (reverse direction)
				// offset = GoCol - DingoCol
				offset := int(m.GoCol) - int(m.DingoCol)
				return goCol - offset, true
			}
		}
	}
	return goCol, false
}

// calculateBaseOffset computes the initial line offset from header differences.
// This accounts for lines removed/added during Go formatting (e.g., empty comments).
// Returns negative if Go has fewer lines, positive if Go has more lines.
func (r *Reader) calculateBaseOffset() int {
	// If we have line mappings, use the first mapping to compute header offset.
	// The //line directive forces GoLineStart = DingoLine, but lines BEFORE
	// the directive may have an offset due to removed/added lines during formatting.
	if len(r.lineMappings) > 0 {
		first := r.lineMappings[0]

		// Check if this is an identity/simple mapping (no //line directive)
		// Identity mappings have no expansion: GoLineEnd == GoLineStart
		// Transformed code has expansion: GoLineEnd > GoLineStart
		if first.GoLineEnd == first.GoLineStart {
			// No expansion means no //line directive was emitted, so no header offset
			return 0
		}

		// For transformed code with //line directive:
		// The //line directive appears at Go line (GoLineStart - 1) and says:
		//   "the next line (GoLineStart) is Dingo line DingoLine"
		// The directive line itself doesn't correspond to any Dingo line.
		//
		// For lines BEFORE the directive, the content at Go line N should be
		// Dingo line (N + offset) if there's an offset.
		//
		// If there's no header offset:
		//   - Go line (GoLineStart - 2) = Dingo line (DingoLine - 1) content
		//   - So Dingo line N maps to Go line N
		// If header offset is -1 (Go has 1 fewer line):
		//   - Go line (GoLineStart - 3) = Dingo line (DingoLine - 1) content
		//   - So Dingo line N maps to Go line (N - 1)
		//
		// Formula: headerOffset = (GoLineStart - 2) - (DingoLine - 1) = GoLineStart - DingoLine - 1
		// This gives the offset to ADD to Dingo line to get Go line.
		headerOffset := int(first.GoLineStart) - int(first.DingoLine) - 1

		return headerOffset
	}

	// No mappings - simple case, check line counts
	dingoTotal := int(r.hdr.DingoLineCnt)
	goTotal := int(r.hdr.GoLineCnt)
	return goTotal - dingoTotal
}

