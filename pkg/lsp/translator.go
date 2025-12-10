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

// TranslatePosition translates a single position between Dingo and Go files
func (t *Translator) TranslatePosition(
	uri protocol.DocumentURI,
	pos protocol.Position,
	dir Direction,
) (protocol.DocumentURI, protocol.Position, error) {
	dirName := "DingoToGo"
	if dir == GoToDingo {
		dirName = "GoToDingo"
	}
	log.Printf("[LSP Translator] TranslatePosition START: direction=%s, uri=%s, line=%d, col=%d",
		dirName, uri.Filename(), pos.Line, pos.Character)

	// Convert LSP position (0-based) to 1-based line:column
	line := int(pos.Line) + 1
	col := int(pos.Character) + 1

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
		// V2 line-based mapping: Calculate line shift and apply
		lineShift := reader.CalculateLineShift(line)

		// Add line shift to get Go line number (column stays the same)
		newLine = line + lineShift
		newCol = col

		log.Printf("[LSP Translator] DingoToGo: dingoLine=%d, lineShift=%d, goLine=%d", line, lineShift, newLine)
		log.Printf("[LSP Translator] AFTER DingoToGo: newLine=%d, newCol=%d", newLine, newCol)
		newURI = lspuri.File(goPath)
	} else {
		// Go → Dingo translation using V2 line mappings
		var kind string
		newLine, kind = reader.GoLineToDingoLine(line)
		newCol = col // Column stays the same (identity mapping within lines)

		if kind != "" {
			log.Printf("[LSP Translator] GoToDingo: goLine=%d -> dingoLine=%d (kind=%s)", line, newLine, kind)
		} else {
			log.Printf("[LSP Translator] GoToDingo: goLine=%d -> dingoLine=%d (identity)", line, newLine)
		}

		dingoPath := goToDingoPath(goPath)
		newURI = lspuri.File(dingoPath)
	}

	// Convert back to LSP position (0-based)
	newPos := protocol.Position{
		Line:      uint32(newLine - 1),
		Character: uint32(newCol - 1),
	}

	log.Printf("[LSP Translator] TranslatePosition END: returning uri=%s, line=%d, col=%d",
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
