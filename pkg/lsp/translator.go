package lsp

import (
	"fmt"
	"log"
	"strings"

	"go.lsp.dev/protocol"
	lspuri "go.lsp.dev/uri"
)

// Direction specifies translation direction
type Direction int

const (
	DingoToGo Direction = iota // .dingo → .go
	GoToDingo                   // .go → .dingo
)

// Translator handles bidirectional position translation using source maps
type Translator struct {
	cache SourceMapGetter
}

// NewTranslator creates a new position translator
func NewTranslator(cache SourceMapGetter) *Translator {
	return &Translator{cache: cache}
}

// TranslatePosition translates a single position between Dingo and Go files.
// LSP uses character offsets (0-indexed), where each character (including tabs) counts as 1.
// This is NOT visual columns - tabs are 1 character, not 4 visual columns.
func (t *Translator) TranslatePosition(
	uri protocol.DocumentURI,
	pos protocol.Position,
	dir Direction,
) (protocol.DocumentURI, protocol.Position, error) {
	dirName := "DingoToGo"
	if dir == GoToDingo {
		dirName = "GoToDingo"
	}
	log.Printf("[LSP Translator] TranslatePosition START: direction=%s, uri=%s, line=%d, char=%d",
		dirName, uri.Filename(), pos.Line, pos.Character)

	// Convert LSP position (0-based) to 1-based for dmap
	line := int(pos.Line) + 1
	col := int(pos.Character) + 1 // LSP character offset is 0-indexed, dmap uses 1-indexed

	// Determine file paths
	var goPath string
	if dir == DingoToGo {
		goPath = dingoToGoPath(uri.Filename())
	} else {
		goPath = uri.Filename()
	}

	// Load source map reader
	reader, err := t.cache.Get(goPath)
	if err != nil {
		// CRITICAL FIX C6: Still translate URI even with 1:1 positions
		// Bug was: returning .dingo URI to gopls when source map missing
		if dir == DingoToGo {
			// Must return .go URI, not .dingo URI
			goURI := lspuri.File(goPath)
			return goURI, pos, fmt.Errorf("source map not found: %s (file not transpiled)", goPath)
		}
		// CRITICAL FIX: For Go->Dingo without map, return location unchanged
		// This handles standard library and external package definitions
		// No source map = not a transpiled file, so return the .go location as-is
		log.Printf("[LSP Translator] No source map for %s, returning location unchanged (likely stdlib/external package)", goPath)
		return uri, pos, nil
	}

	// Translate position using binary .dmap reader
	var newLine, newCol int
	var newURI protocol.DocumentURI

	if dir == DingoToGo {
		// Use DingoLineToGoLine which handles both:
		// - Transformed lines (returns GoLineStart where actual code is)
		// - Untransformed lines (uses line shift calculation)
		newLine = reader.DingoLineToGoLine(line)

		// Apply column translation for transformed lines (error propagation, etc.)
		// This adjusts for LHS changes: "userID := func()?" -> "tmp, err := func()"
		var colFound bool
		newCol, colFound = reader.TranslateDingoColumn(line, col)
		if colFound {
			log.Printf("[LSP Translator] DingoToGo: column translated %d -> %d", col, newCol)
		} else {
			// For transformed lines, positions outside the mapped range:
			// - Don't fall back to first code char (that gives wrong hover info for operators like := or ?)
			// - Use identity mapping and let gopls handle it (may return empty for non-symbols)
			if reader.IsTransformedLine(line) {
				log.Printf("[LSP Translator] DingoToGo: transformed line, unmapped col %d -> identity (no mapping)", col)
			} else {
				log.Printf("[LSP Translator] DingoToGo: column identity %d", col)
			}
			// newCol stays as col (identity mapping)
		}

		log.Printf("[LSP Translator] DingoToGo: dingoLine=%d -> goLine=%d", line, newLine)
		log.Printf("[LSP Translator] AFTER DingoToGo: newLine=%d, newCol=%d", newLine, newCol)
		newURI = lspuri.File(goPath)
	} else {
		// Go → Dingo translation using V2 line mappings
		var kind string
		newLine, kind = reader.GoLineToDingoLine(line)

		// Apply column translation for transformed lines (reverse direction)
		var colFound bool
		newCol, colFound = reader.TranslateGoColumn(line, col)
		if colFound {
			log.Printf("[LSP Translator] GoToDingo: column translated %d -> %d", col, newCol)
		} else {
			newCol = col // Fallback to identity mapping
		}

		// Always clamp column to Dingo line bounds when column exceeds line length.
		// This prevents column overflow when Go line is longer than Dingo line
		// (e.g., tuple literal expansion: "(x,y)" -> "tuples.Tuple2[...]{...}")
		// We do this unconditionally because even with stale/empty source maps,
		// the column should never exceed the line length in the Dingo file.
		lineLen := reader.DingoLineLength(newLine)
		if lineLen >= 0 && newCol > lineLen {
			// Clamp to end of line (reasonable fallback for expanded lines)
			newCol = lineLen
			log.Printf("[LSP Translator] GoToDingo: clamped column %d -> %d (line length %d, kind=%q)", col, newCol, lineLen, kind)
		}

		if kind != "" {
			log.Printf("[LSP Translator] GoToDingo: goLine=%d -> dingoLine=%d (kind=%s)", line, newLine, kind)
		} else {
			log.Printf("[LSP Translator] GoToDingo: goLine=%d -> dingoLine=%d (identity)", line, newLine)
		}

		dingoPath := goToDingoPath(goPath)
		newURI = lspuri.File(dingoPath)
	}

	// Convert 1-indexed column back to 0-indexed LSP character offset
	// LSP uses character offsets directly (no visual column conversion needed)
	finalCharacter := newCol - 1
	log.Printf("[LSP Translator] %s: returning character offset %d", dirName, finalCharacter)

	newPos := protocol.Position{
		Line:      uint32(newLine - 1),
		Character: uint32(finalCharacter),
	}

	log.Printf("[LSP Translator] TranslatePosition END: returning uri=%s, line=%d, character=%d",
		newURI.Filename(), newPos.Line, newPos.Character)

	return newURI, newPos, nil
}

// TranslateRange translates a range between Dingo and Go files
func (t *Translator) TranslateRange(
	uri protocol.DocumentURI,
	rng protocol.Range,
	dir Direction,
) (protocol.DocumentURI, protocol.Range, error) {
	// Translate start position
	newURI, newStart, err := t.TranslatePosition(uri, rng.Start, dir)
	if err != nil {
		return uri, rng, err
	}

	// Translate end position
	_, newEnd, err := t.TranslatePosition(uri, rng.End, dir)
	if err != nil {
		return uri, rng, err
	}

	// Preserve error span width when both positions get clamped to same location
	// This happens when Go line is longer than Dingo line (e.g., tuple expansion)
	// Without this fix, "nilf" (4 chars) would show as 1 char underline
	if newStart.Line == newEnd.Line && newStart.Character == newEnd.Character {
		// Both positions clamped to same spot - preserve original width
		// LSP ranges are [start, end) so width = end - start
		originalWidth := rng.End.Character - rng.Start.Character
		if originalWidth > 0 {
			// Move end forward by 1 to make room for full width
			// (end was clamped to last char position, but needs to be exclusive)
			newEnd.Character++
			// Then move start back to get full width
			if newEnd.Character >= originalWidth {
				newStart.Character = newEnd.Character - originalWidth
			} else {
				newStart.Character = 0
			}
			log.Printf("[LSP Translator] TranslateRange: preserved width %d, range now %d-%d",
				originalWidth, newStart.Character, newEnd.Character)
		}
	}

	newRange := protocol.Range{
		Start: newStart,
		End:   newEnd,
	}

	return newURI, newRange, nil
}

// TranslateLocation translates a location (URI + range)
func (t *Translator) TranslateLocation(
	loc protocol.Location,
	dir Direction,
) (protocol.Location, error) {
	newURI, newRange, err := t.TranslateRange(loc.URI, loc.Range, dir)
	if err != nil {
		return loc, err
	}

	return protocol.Location{
		URI:   newURI,
		Range: newRange,
	}, nil
}

// Helper functions for file path conversion

func isDingoFile(uri protocol.DocumentURI) bool {
	return strings.HasSuffix(string(uri), ".dingo")
}

func isDingoFilePath(path string) bool {
	return strings.HasSuffix(path, ".dingo")
}

func dingoToGoPath(dingoPath string) string {
	if !strings.HasSuffix(dingoPath, ".dingo") {
		return dingoPath
	}
	return strings.TrimSuffix(dingoPath, ".dingo") + ".go"
}

func goToDingoPath(goPath string) string {
	if !strings.HasSuffix(goPath, ".go") {
		return goPath
	}
	return strings.TrimSuffix(goPath, ".go") + ".dingo"
}
