package dmap

import "errors"

// Magic bytes for .dmap file format
var Magic = [4]byte{'D', 'M', 'A', 'P'}

// Version is the current .dmap format version
const Version = uint16(3)

// HeaderSize is the fixed size of the v3 file header in bytes
const HeaderSize = 56

// LineMappingEntrySize is the fixed size of each line mapping entry in bytes
const LineMappingEntrySize = 16

// ColumnMappingEntrySize is the fixed size of each column mapping entry
const ColumnMappingEntrySize = 16

// Header represents the fixed 56-byte .dmap v3 file header
type Header struct {
	Magic [4]byte // File magic: "DMAP" (0x444D4150)
	Version uint16 // Format version (3)
	Flags uint16 // Bit 0: has column mappings
	DingoLen uint32 // Original .dingo file size in bytes
	GoLen uint32 // Generated .go file size in bytes

	// Line offset sections
	LineIdxOff uint32 // Byte offset to line index section
	DingoLineCnt uint32 // Number of Dingo line offsets
	GoLineCnt uint32 // Number of Go line offsets

	// Line mapping section (Dingo line -> Go line range)
	LineMappingOff uint32 // Byte offset to line mapping section
	LineMappingCnt uint32 // Number of line mapping entries

	// Column mapping section (for hover/go-to-definition)
	ColumnMappingOff uint32 // Byte offset to column mapping section
	ColumnMappingCnt uint32 // Number of column mapping entries

	// Kind strings section
	KindStrOff uint32 // Byte offset to kind strings section

	// //line directive index (for validation)
	LineDirectiveCnt uint32 // Number of //line directives emitted
}

// Flags constants
const (
	FlagHasColumnMappings = 1 << 0
)

// LineMappingEntry represents a line-level mapping (16 bytes)
type LineMappingEntry struct {
	DingoLine uint32 // Line number in .dingo source (1-indexed)
	GoLineStart uint32 // Start line number in .go output (1-indexed)
	GoLineEnd uint32 // End line number in .go output (1-indexed, inclusive)
	KindIdx uint16 // Index into kind string table
	Reserved uint16 // Alignment padding for future use
}

// ColumnMappingEntry provides precise position mapping for hover/go-to-definition
type ColumnMappingEntry struct {
	DingoLine uint16 // Line in .dingo (1-indexed)
	DingoCol uint16 // Column in .dingo (1-indexed)
	GoLine uint16 // Line in .go (1-indexed)
	GoCol uint16 // Column in .go (1-indexed)
	Length uint16 // Length of the mapped region (bytes)
	KindIdx uint16 // Index into kind string table
	Reserved uint32 // Padding for alignment
}

// ErrMigrationRequired indicates a v1/v2 .dmap file that needs regeneration
var ErrMigrationRequired = errors.New("dmap: v1/v2 format not supported, regenerate with dingo build")
