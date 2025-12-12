package sourcemap

import "sort"

// TransformTracker records transformations during transpilation and computes
// final line-level mappings after all transforms complete.
//
// Deprecated: TransformTracker uses byte offsets which become stale after go/printer
// reformats the code. Use PositionTracker instead, which stores token.Pos from the
// Dingo AST and resolves positions after go/printer completes.
//
// Migration:
//   // Before:
//   tracker := NewTransformTracker(dingoSource)
//   tracker.RecordTransform(startByte, endByte, "error_prop", generatedLen)
//
//   // After:
//   tracker := NewPositionTracker(dingoFset)
//   tracker.RecordTransform(node.Pos(), node.End(), "error_prop")
//
// Design: Instead of computing Go positions during transformation (fragile with
// cumulative byte deltas), we record raw transform metadata and compute final
// line mappings AFTER all transforms using actual line counting.
type TransformTracker struct {
	originalSource []byte           // Original .dingo source (immutable)
	transforms     []TransformRecord // Recorded transforms in source order
	finalized      bool              // True after Finalize() called
	lineMappings   []LineMapping     // Computed after finalization
	finalLineDelta int               // Total line difference: len(goLines) - len(dingoLines)
}

// TransformRecord captures metadata about a single transformation.
// Positions are relative to the ORIGINAL Dingo source (immutable during tracking).
type TransformRecord struct {
	DingoStart   int    // Start byte in original .dingo source
	DingoEnd     int    // End byte in original .dingo source
	Kind         string // Transform type: "error_prop", "safe_nav", "null_coalesce", etc.
	GeneratedLen int    // Length of generated Go code (bytes)
}

// LineMapping represents the final line-level mapping stored in .dmap v2.
// Key insight: ALL generated Go lines map back to the SAME Dingo line (1:N mapping).
//
// Example: "x := foo()?" (1 line) expands to:
//   tmp, err := foo()
//   if err != nil {
//       return err
//   }
//   x := tmp
// All 4 Go lines map to the single Dingo line.
type LineMapping struct {
	DingoLine   int    // 1-indexed line in .dingo source
	GoLineStart int    // 1-indexed start line in .go output
	GoLineEnd   int    // 1-indexed end line in .go output (inclusive)
	Kind        string // Transform type
}

// NewTransformTracker creates a tracker with the original Dingo source.
// This source must remain unchanged during transformation for accurate position mapping.
func NewTransformTracker(dingoSource []byte) *TransformTracker {
	return &TransformTracker{
		originalSource: dingoSource,
		transforms:     make([]TransformRecord, 0, 16), // Pre-allocate for typical cases
	}
}

// RecordTransform records a transformation at the given Dingo position.
//
// MUST be called BEFORE the transform is applied to the working buffer.
// dingoStart/dingoEnd are positions in the ORIGINAL source.
//
// Example:
//   dingoSource = "x := foo()?\n"
//   RecordTransform(0, 11, "error_prop", 80)
//   // Then apply transform to working buffer
func (t *TransformTracker) RecordTransform(dingoStart, dingoEnd int, kind string, generatedLen int) {
	t.transforms = append(t.transforms, TransformRecord{
		DingoStart:   dingoStart,
		DingoEnd:     dingoEnd,
		Kind:         kind,
		GeneratedLen: generatedLen,
	})
}

// Finalize computes line-level mappings using actual line counts from sources.
//
// Algorithm:
// 1. Build line offset tables for both Dingo and Go sources
// 2. Sort transforms by DingoStart (ascending)
// 3. For each transform:
//    - Skip untransformed region in BOTH sources (dual position tracking)
//    - Calculate which Dingo line it starts on
//    - Track cumulative line delta from all previous transforms
//    - Count actual lines in generated Go code at CORRECT position
//    - Create LineMapping where ALL Go lines map to the Dingo line
//
// Must be called after ALL transforms complete, with the final Go output.
func (t *TransformTracker) Finalize(goSource []byte) error {
	if t.finalized {
		return nil // Already finalized
	}

	// Build line offset tables for both sources
	dingoLineOffsets := buildLineOffsets(t.originalSource)

	// Sort transforms by DingoStart (ascending)
	// We process in order they appear in source
	sortedTransforms := make([]TransformRecord, len(t.transforms))
	copy(sortedTransforms, t.transforms)
	sort.Slice(sortedTransforms, func(i, j int) bool {
		return sortedTransforms[i].DingoStart < sortedTransforms[j].DingoStart
	})

	// Calculate line mappings
	// CRITICAL FIX: Track positions in BOTH sources to handle gaps
	dingoBytePos := 0     // Current position in Dingo source
	goBytePos := 0        // Current position in Go output
	cumulativeLineDelta := 0

	for _, tr := range sortedTransforms {
		// 1. Skip untransformed region in BOTH sources
		// Untransformed code has identity mapping (same bytes in both)
		untransformedLen := tr.DingoStart - dingoBytePos
		goBytePos += untransformedLen

		// 2. Find Dingo line for this transform
		dingoLine := byteToLine(dingoLineOffsets, tr.DingoStart)

		// Calculate lines in original Dingo range
		dingoLines := linesInRange(dingoLineOffsets, tr.DingoStart, tr.DingoEnd)

		// 3. Find corresponding Go start line (accounting for previous transforms)
		// cumulativeLineDelta tracks how many lines we've shifted so far
		goStartLine := dingoLine + cumulativeLineDelta

		// 4. Calculate lines in generated Go code at CORRECT position
		goLines := countNewlines(goSource[goBytePos:goBytePos+tr.GeneratedLen]) + 1

		// 5. Create mapping: ALL Go lines map to the Dingo line
		t.lineMappings = append(t.lineMappings, LineMapping{
			DingoLine:   dingoLine,
			GoLineStart: goStartLine,
			GoLineEnd:   goStartLine + goLines - 1,
			Kind:        tr.Kind,
		})

		// 6. Update positions for next transform
		lineDelta := goLines - dingoLines
		cumulativeLineDelta += lineDelta
		dingoBytePos = tr.DingoEnd      // Move past transform in Dingo
		goBytePos += tr.GeneratedLen    // Move past transform in Go
	}

	// Compute final line delta by counting lines in both sources
	dingoLineCount := countNewlines(t.originalSource) + 1
	goLineCount := countNewlines(goSource) + 1
	t.finalLineDelta = goLineCount - dingoLineCount

	t.finalized = true
	return nil
}

// LineMappings returns the computed line mappings (after Finalize).
// Returns empty slice if not yet finalized.
func (t *TransformTracker) LineMappings() []LineMapping {
	return t.lineMappings
}

// FinalLineDelta returns the total line difference between Go and Dingo sources.
// Positive means Go has more lines than Dingo.
// Returns 0 if not yet finalized.
func (t *TransformTracker) FinalLineDelta() int {
	return t.finalLineDelta
}

// Helper functions

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

// byteToLine converts a byte position to a 1-indexed line number.
// Uses binary search for O(log N) performance.
func byteToLine(offsets []int, bytePos int) int {
	// Binary search for line containing bytePos
	idx := sort.Search(len(offsets), func(i int) bool {
		return offsets[i] > bytePos
	})
	// idx is the first line that starts AFTER bytePos
	// So bytePos is on line idx-1 (but we want 1-indexed)
	if idx == 0 {
		return 1 // bytePos before first line offset (shouldn't happen)
	}
	return idx // Already 1-indexed since offsets[0] = byte 0 = line 1
}

// linesInRange calculates how many lines span from start to end byte positions.
// Inclusive of both start and end positions.
func linesInRange(offsets []int, start, end int) int {
	startLine := byteToLine(offsets, start)
	endLine := byteToLine(offsets, end)
	return endLine - startLine + 1
}

// countNewlines counts the number of newline characters in a byte slice.
// Simple linear scan - called only during finalization.
func countNewlines(b []byte) int {
	count := 0
	for _, c := range b {
		if c == '\n' {
			count++
		}
	}
	return count
}

