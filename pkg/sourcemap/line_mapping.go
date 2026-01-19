package sourcemap

// LineMapping represents line-level mapping stored in .dmap v2/v3.
// Maps a range of Go output lines back to Dingo source lines.
//
// For single-line transforms (error propagation), DingoLineCount is 1:
// Example: "x := foo()?" (1 Dingo line) expands to 4 Go lines.
//
// For multi-line transforms (match expressions), DingoLineCount > 1:
// Example: "match x { ... }" spanning 18 Dingo lines expands to 23 Go lines.
//
// The DingoLineCount field is essential for correct line calculations
// when navigating between Dingo and Go positions.
type LineMapping struct {
	DingoLine      int    // 1-indexed line in .dingo source (start line)
	DingoLineCount int    // Number of Dingo lines in transform (0 = single line for backwards compat)
	GoLineStart    int    // 1-indexed start line in .go output
	GoLineEnd      int    // 1-indexed end line in .go output (inclusive)
	Kind           string // Transform type
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
