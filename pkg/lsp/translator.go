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
		// Convert Dingo line:col → byte offset
		dingoByteOffset := reader.DingoLineToByteOffset(line)
		if dingoByteOffset < 0 {
			// Line out of range - return identity mapping
			log.Printf("[LSP Translator] Dingo line %d out of range, using identity mapping", line)
			newLine, newCol = line, col
		} else {
			// Calculate line shift from transforms that occur BEFORE this position
			// Each transform may add or remove lines, affecting subsequent line numbers
			lineShift := reader.CalculateLineShift(dingoByteOffset)

			// Add column offset (col is 1-based, so subtract 1)
			fullDingoOffset := dingoByteOffset + (col - 1)

			// Look up mapping: Dingo byte offset → Go byte range
			_, _, kind := reader.FindByDingoPos(fullDingoOffset)

			// Apply line shift to get the Go line number
			newLine = line + lineShift
			newCol = col

			if kind != "" {
				log.Printf("[LSP Translator] Position in mapped region (kind=%s), lineShift=%d", kind, lineShift)
			} else {
				log.Printf("[LSP Translator] No mapping for Dingo position, lineShift=%d", lineShift)
			}
		}

		log.Printf("[LSP Translator] AFTER DingoToGo: newLine=%d, newCol=%d", newLine, newCol)
		newURI = lspuri.File(goPath)
	} else {
		// Go → Dingo translation
		// Convert Go line:col → byte offset
		goByteOffset := reader.GoLineToByteOffset(line)
		if goByteOffset < 0 {
			// Line out of range - return identity mapping
			log.Printf("[LSP Translator] Go line %d out of range, using identity mapping", line)
			newLine, newCol = line, col
		} else {
			// Add column offset (col is 1-based, so subtract 1)
			goByteOffset += (col - 1)

			// Look up mapping: Go byte offset → Dingo byte range
			dingoStart, dingoEnd, kind := reader.FindByGoPos(goByteOffset)

			// If no mapping found (identity), dingoStart == goByteOffset
			if kind == "" {
				// Identity mapping - translate coordinates directly
				log.Printf("[LSP Translator] No mapping for Go position, using identity")
				newLine, newCol = line, col
			} else {
				// Proportional mapping within the range
				log.Printf("[LSP Translator] Mapped Go byte %d to Dingo byte range [%d, %d), kind=%s",
					goByteOffset, dingoStart, dingoEnd, kind)

				// Convert Dingo byte offset → line:column
				newLine = reader.DingoByteToLine(dingoStart)
				lineStartOffset := reader.DingoLineToByteOffset(newLine)
				if lineStartOffset >= 0 {
					newCol = dingoStart - lineStartOffset + 1 // +1 for 1-based
				} else {
					newCol = 1
				}
			}
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
