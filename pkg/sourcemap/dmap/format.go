package dmap

// Magic bytes for .dmap file format
var Magic = [4]byte{'D', 'M', 'A', 'P'}

// Version is the current .dmap format version
const Version = uint16(2)

// HeaderSize is the fixed size of the file header in bytes
const HeaderSize = 44

// EntrySize is the fixed size of each mapping entry in bytes
const EntrySize = 20

// LineMappingEntrySize is the fixed size of each line mapping entry in bytes
const LineMappingEntrySize = 16

// Header represents the fixed 44-byte .dmap file header
type Header struct {
	Magic          [4]byte // File magic: "DMAP" (0x444D4150)
	Version        uint16  // Format version (currently 2)
	Flags          uint16  // Reserved for future use (compression, etc.)
	EntryCount     uint32  // Number of mapping entries (same for both indexes)
	DingoLen       uint32  // Original .dingo file size in bytes
	GoLen          uint32  // Generated .go file size in bytes
	GoIdxOff       uint32  // Byte offset to Go index section (always 44)
	DingoIdxOff    uint32  // Byte offset to Dingo index section
	LineIdxOff     uint32  // Byte offset to line index section
	KindStrOff     uint32  // Byte offset to kind strings section
	LineMappingOff uint32  // Byte offset to line mapping section
	LineMappingCnt uint32  // Number of line mapping entries
}

// Entry represents a single source mapping (20 bytes)
type Entry struct {
	DingoStart uint32 // Byte offset in .dingo source (inclusive)
	DingoEnd   uint32 // Byte offset in .dingo source (exclusive)
	GoStart    uint32 // Byte offset in .go output (inclusive)
	GoEnd      uint32 // Byte offset in .go output (exclusive)
	KindIdx    uint16 // Index into kind string table
	Reserved   uint16 // Alignment padding for future use
}

// LineMappingEntry represents a line-level mapping (16 bytes)
type LineMappingEntry struct {
	DingoLine   uint32 // Line number in .dingo source (1-indexed)
	GoLineStart uint32 // Start line number in .go output (1-indexed)
	GoLineEnd   uint32 // End line number in .go output (1-indexed, inclusive)
	KindIdx     uint16 // Index into kind string table
	Reserved    uint16 // Alignment padding for future use
}
