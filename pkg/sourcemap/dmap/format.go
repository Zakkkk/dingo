package dmap

// Magic bytes for .dmap file format
var Magic = [4]byte{'D', 'M', 'A', 'P'}

// Version is the current .dmap format version
const Version = uint16(1)

// HeaderSize is the fixed size of the file header in bytes
const HeaderSize = 36

// EntrySize is the fixed size of each mapping entry in bytes
const EntrySize = 20

// Header represents the fixed 36-byte .dmap file header
type Header struct {
	Magic       [4]byte // File magic: "DMAP" (0x444D4150)
	Version     uint16  // Format version (currently 1)
	Flags       uint16  // Reserved for future use (compression, etc.)
	EntryCount  uint32  // Number of mapping entries (same for both indexes)
	DingoLen    uint32  // Original .dingo file size in bytes
	GoLen       uint32  // Generated .go file size in bytes
	GoIdxOff    uint32  // Byte offset to Go index section (always 36)
	DingoIdxOff uint32  // Byte offset to Dingo index section
	LineIdxOff  uint32  // Byte offset to line index section
	KindStrOff  uint32  // Byte offset to kind strings section
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
