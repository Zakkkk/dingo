package sourcemap

// LineMapping represents line-level mapping stored in .dmap v2/v3.
// Maps a range of Go output lines back to a single Dingo source line.
//
// Example: "x := foo()?" (1 Dingo line) expands to:
//
//	tmp, err := foo()
//	if err != nil {
//	    return err
//	}
//	x := tmp
//
// All 4 Go lines map to the single Dingo line.
type LineMapping struct {
	DingoLine   int    // 1-indexed line in .dingo source
	GoLineStart int    // 1-indexed start line in .go output
	GoLineEnd   int    // 1-indexed end line in .go output (inclusive)
	Kind        string // Transform type
}

// buildLineOffsets creates a slice of byte offsets where each line starts.
// offsets[0] = 0 (first line starts at byte 0)
// offsets[1] = byte offset of second line (first char after first \n)
// etc.
func buildLineOffsets(src []byte) []int {
	offsets := []int{0} // Line 1 starts at byte 0
	for i, b := range src {
		if b == '\n' {
			offsets = append(offsets, i+1) // Next line starts after \n
		}
	}
	return offsets
}
