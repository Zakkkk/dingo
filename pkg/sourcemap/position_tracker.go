package sourcemap

import (
	"fmt"
	"go/token"
	"sort"
)

// Transform represents a single Dingo->Go transformation with AST positions.
// Uses token.Pos from Dingo AST for accurate position tracking.
//
// DESIGN: Unlike the deprecated TransformTracker which uses byte offsets,
// PositionTracker stores token.Pos and resolves to lines AFTER go/printer.
type Transform struct {
	// Source positions from Dingo AST (via node.Pos(), node.End())
	DingoPos token.Pos // Start position in Dingo source
	DingoEnd token.Pos // End position in Dingo source

	// Computed after Finalize()
	DingoLine  int // Line in .dingo (1-indexed)
	DingoCol   int // Column in .dingo (1-indexed)
	GoLine     int // Start line in .go (1-indexed)
	GoLineEnd  int // End line in .go (1-indexed, inclusive)
	GoCol      int // Column in .go (1-indexed)
	ByteLength int // Length of the transformed region in bytes (for ColumnMapping.Length)

	Kind string // Transform type: "error_prop", "lambda", "match", etc.
}

// ColumnMapping for v3 format - precise mapping for hover/go-to-definition
type ColumnMapping struct {
	DingoLine int
	DingoCol  int
	GoLine    int
	GoCol     int
	Length    int
	Kind      string
}

// PositionTracker records transformations using AST positions.
// This replaces TransformTracker with token.Pos-based tracking.
//
// Design: Store token.Pos from Dingo AST nodes during codegen,
// then resolve to lines AFTER go/printer reformats.
//
// Usage:
//
//	tracker := NewPositionTracker(dingoFset)
//	// During codegen:
//	tracker.RecordTransform(node.Pos(), node.End(), "error_prop")
//	// After go/printer produces final Go code:
//	tracker.Finalize(goSource, goFset)
type PositionTracker struct {
	dingoFset  *token.FileSet // Dingo's FileSet (from pkg/parser)
	transforms []Transform

	// Set after Finalize()
	finalized    bool
	lineMappings []LineMapping
	colMappings  []ColumnMapping
	droppedCount int // Count of transforms dropped due to invalid positions
}

// NewPositionTracker creates a tracker with the Dingo FileSet.
// dingoFset must be the FileSet used by the Dingo parser to create AST nodes.
func NewPositionTracker(dingoFset *token.FileSet) *PositionTracker {
	return &PositionTracker{
		dingoFset:  dingoFset,
		transforms: make([]Transform, 0, 16),
	}
}

// RecordTransform records a transformation from AST node positions.
// MUST be called during codegen while AST positions are valid.
//
// Example:
//
//	// In error_prop codegen:
//	tracker.RecordTransform(errorPropExpr.Pos(), errorPropExpr.End(), "error_prop")
//
// The positions will be resolved to line/column numbers during Finalize().
func (t *PositionTracker) RecordTransform(dingoPos, dingoEnd token.Pos, kind string) {
	if !dingoPos.IsValid() || !dingoEnd.IsValid() {
		// Skip invalid positions (shouldn't happen in normal codegen)
		t.droppedCount++
		return
	}

	t.transforms = append(t.transforms, Transform{
		DingoPos: dingoPos,
		DingoEnd: dingoEnd,
		Kind:     kind,
	})
}

// DroppedCount returns the number of transforms dropped due to invalid positions.
func (t *PositionTracker) DroppedCount() int {
	return t.droppedCount
}

// Finalize computes line/column mappings using fset.Position().
// MUST be called AFTER go/printer has produced final output.
// goFset is the FileSet from parsing the generated Go code.
//
// Algorithm:
// 1. Resolve all Dingo positions to line/col using dingoFset.Position()
// 2. Sort transforms by Dingo line number
// 3. For each transform, find corresponding Go line using //line directive matching
// 4. Generate both line-level and column-level mappings
//
// Note: This assumes //line directives have been emitted in the Go code.
// Without //line directives, the mapping is approximate.
func (t *PositionTracker) Finalize(goSource []byte, goFset *token.FileSet) error {
	if t.finalized {
		return nil // Already finalized
	}

	if t.dingoFset == nil {
		return fmt.Errorf("position_tracker: dingoFset is nil (was NewPositionTracker called with valid FileSet?)")
	}

	if goSource == nil || len(goSource) == 0 {
		return fmt.Errorf("position_tracker: goSource is nil or empty")
	}

	// Build Go line offset table for byte-to-line conversions
	goLineOffsets := buildLineOffsets(goSource)

	// Resolve all Dingo positions using the FileSet
	for i := range t.transforms {
		tr := &t.transforms[i]

		// Resolve Dingo positions
		dingoStartPos := t.dingoFset.Position(tr.DingoPos)
		dingoEndPos := t.dingoFset.Position(tr.DingoEnd)

		tr.DingoLine = dingoStartPos.Line
		tr.DingoCol = dingoStartPos.Column

		// For Go positions, we need to parse //line directives or use heuristics
		// For v3, we expect //line directives to be present in goSource
		// Find the corresponding Go line by scanning for //line directives
		goLine, goCol := t.findGoPosition(goSource, goLineOffsets, dingoStartPos.Line, dingoStartPos.Column)
		tr.GoLine = goLine
		tr.GoCol = goCol

		// Calculate end line by counting lines in the generated block
		// Heuristic: count lines from GoLine until next //line directive or end
		goEndLine := t.findGoEndLine(goSource, goLineOffsets, tr.GoLine)
		tr.GoLineEnd = goEndLine

		// Calculate byte length from Dingo positions (for column mappings)
		tr.ByteLength = dingoEndPos.Offset - dingoStartPos.Offset
	}

	// Sort transforms by Dingo line (ascending) for easier processing
	sortedTransforms := make([]Transform, len(t.transforms))
	copy(sortedTransforms, t.transforms)
	sort.Slice(sortedTransforms, func(i, j int) bool {
		if sortedTransforms[i].DingoLine == sortedTransforms[j].DingoLine {
			return sortedTransforms[i].DingoCol < sortedTransforms[j].DingoCol
		}
		return sortedTransforms[i].DingoLine < sortedTransforms[j].DingoLine
	})

	// Generate line mappings (1:N mapping - multiple Go lines per Dingo line)
	for _, tr := range sortedTransforms {
		t.lineMappings = append(t.lineMappings, LineMapping{
			DingoLine:   tr.DingoLine,
			GoLineStart: tr.GoLine,
			GoLineEnd:   tr.GoLineEnd,
			Kind:        tr.Kind,
		})

		// Generate column mapping for precise hover/go-to-definition
		t.colMappings = append(t.colMappings, ColumnMapping{
			DingoLine: tr.DingoLine,
			DingoCol:  tr.DingoCol,
			GoLine:    tr.GoLine,
			GoCol:     tr.GoCol,
			Length:    tr.ByteLength, // Byte length, not line count
			Kind:      tr.Kind,
		})
	}

	t.finalized = true
	return nil
}

// LineMappings returns the computed line mappings (after Finalize).
// Returns empty slice if not yet finalized.
func (t *PositionTracker) LineMappings() []LineMapping {
	return t.lineMappings
}

// ColumnMappings returns column-level mappings for precise hover/go-to-def.
// Returns empty slice if not yet finalized.
func (t *PositionTracker) ColumnMappings() []ColumnMapping {
	return t.colMappings
}

// findGoPosition locates the Go line/column corresponding to a Dingo position.
// Scans goSource for //line directives that match the Dingo position.
//
// Algorithm:
// 1. Scan goSource line-by-line
// 2. Track current Dingo line based on //line directives
// 3. When we find a //line directive matching dingoLine:dingoCol, record Go line
// 4. If no exact match, use heuristic (Dingo line maps to Go line with cumulative delta)
func (t *PositionTracker) findGoPosition(goSource []byte, goLineOffsets []int, dingoLine, dingoCol int) (goLine, goCol int) {
	// Default: assume identity mapping (no transform)
	goLine = dingoLine
	goCol = dingoCol

	// Parse //line directives from goSource
	currentGoLine := 1

	for i := 0; i < len(goSource); {
		// Find next newline
		lineEnd := i
		for lineEnd < len(goSource) && goSource[lineEnd] != '\n' {
			lineEnd++
		}

		// Extract line content
		line := goSource[i:lineEnd]

		// Check if this is a //line directive
		if len(line) >= 7 && string(line[0:7]) == "//line " {
			// Parse: //line filename.dingo:42:5
			directive := string(line[7:])

			// Find the last colon (column)
			lastColon := -1
			for j := len(directive) - 1; j >= 0; j-- {
				if directive[j] == ':' {
					lastColon = j
					break
				}
			}

			if lastColon > 0 {
				// Find the second-to-last colon (line number)
				secondLastColon := -1
				for j := lastColon - 1; j >= 0; j-- {
					if directive[j] == ':' {
						secondLastColon = j
						break
					}
				}

				if secondLastColon > 0 {
					// Extract line and column numbers
					var parsedLine, parsedCol int
					_, err := fmt.Sscanf(directive[secondLastColon+1:lastColon], "%d", &parsedLine)
					if err == nil {
						fmt.Sscanf(directive[lastColon+1:], "%d", &parsedCol)

						// Check if this directive matches our target Dingo position
						if parsedLine == dingoLine {
							// The next Go line after this directive corresponds to dingoLine
							return currentGoLine + 1, parsedCol
						}
					}
				}
			}
		}

		currentGoLine++
		i = lineEnd + 1
	}

	// Fallback to identity mapping if no directive found
	return dingoLine, dingoCol
}

// findGoEndLine calculates the end line of a transformed block in Go code.
// Scans forward from goStartLine until we hit the next //line directive or EOF.
func (t *PositionTracker) findGoEndLine(goSource []byte, goLineOffsets []int, goStartLine int) int {
	// Bounds check
	if goStartLine < 1 || goStartLine >= len(goLineOffsets) {
		return goStartLine
	}

	// Start scanning from the goStartLine
	currentLine := goStartLine
	i := 0

	// Skip to the start line
	for lineNum := 1; lineNum < goStartLine && i < len(goSource); lineNum++ {
		for i < len(goSource) && goSource[i] != '\n' {
			i++
		}
		i++ // Skip the newline
	}

	// Now scan forward to find the next //line directive or EOF
	for i < len(goSource) {
		// Find next newline
		lineEnd := i
		for lineEnd < len(goSource) && goSource[lineEnd] != '\n' {
			lineEnd++
		}

		// Extract line content
		line := goSource[i:lineEnd]

		// Check if this is a //line directive (but not the current line)
		if currentLine > goStartLine && len(line) >= 7 && string(line[0:7]) == "//line " {
			// Found next directive - the previous line is the end
			return currentLine - 1
		}

		currentLine++
		i = lineEnd + 1
	}

	// Reached EOF - return the last line
	return currentLine - 1
}
