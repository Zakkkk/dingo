package lsp

import (
	"fmt"
	"go/scanner"
	"go/token"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/MadAppGang/dingo/pkg/config"
	"github.com/MadAppGang/dingo/pkg/transpiler"
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
	cache       SourceMapGetter
	dingoConfig *config.Config
}

// NewTranslator creates a new position translator
func NewTranslator(cache SourceMapGetter) *Translator {
	return NewTranslatorWithConfig(cache, nil)
}

// NewTranslatorWithConfig creates a translator with explicit config
func NewTranslatorWithConfig(cache SourceMapGetter, cfg *config.Config) *Translator {
	if cfg == nil {
		// Load config or use defaults
		cfg, _ = config.Load(nil)
		if cfg == nil {
			cfg = config.DefaultConfig()
		}
	}
	return &Translator{cache: cache, dingoConfig: cfg}
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

	// Determine file paths using config-aware path calculation
	var goPath string
	if dir == DingoToGo {
		goPath = dingoToGoPathWithConfig(uri.Filename(), t.dingoConfig)
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

		// If no explicit mapping found (kind == ""), try //line directives
		// This handles:
		// - Enum expansion which adds lines but doesn't create dmap entries
		// - Code between transforms (e.g., function signature before match body)
		// - Any line not covered by an explicit transform mapping
		// The //line directives in the generated Go file provide the correct
		// Dingo position via Go's auto-increment from the last directive.
		if kind == "" {
			if directiveLine, found := translateUsingLineDirectives(goPath, line); found {
				newLine = directiveLine
				kind = "line_directive"
			}
		}

		// Apply column translation for transformed lines (reverse direction)
		var colFound bool
		newCol, colFound = reader.TranslateGoColumn(line, col)
		if colFound {
			log.Printf("[LSP Translator] GoToDingo: column translated %d -> %d", col, newCol)
		} else {
			// No explicit column mapping - apply tab expansion
			// Go files use tabs, Dingo files use 4 spaces
			newCol = expandTabsInColumn(goPath, line, col)
		}

		// Always clamp column to Dingo line bounds when column exceeds line length.
		// This prevents column overflow when Go line is longer than Dingo line
		// (e.g., tuple literal expansion: "(x,y)" -> "tuples.Tuple2[...]{...}")
		// We do this unconditionally because even with stale/empty source maps,
		// the column should never exceed the line length in the Dingo file.
		// Note: Allow lineLen+1 for exclusive end positions in LSP ranges.
		lineLen := reader.DingoLineLength(newLine)
		if lineLen >= 0 && newCol > lineLen+1 {
			// Clamp to end of line + 1 (for exclusive range ends)
			newCol = lineLen + 1
			log.Printf("[LSP Translator] GoToDingo: clamped column %d -> %d (line length %d, kind=%q)", col, newCol, lineLen, kind)
		}

		if kind != "" {
			log.Printf("[LSP Translator] GoToDingo: goLine=%d -> dingoLine=%d (kind=%s)", line, newLine, kind)
		} else {
			log.Printf("[LSP Translator] GoToDingo: goLine=%d -> dingoLine=%d (identity)", line, newLine)
		}

		dingoPath := goToDingoPathWithConfig(goPath, t.dingoConfig)
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

	// WORKAROUND: Ensure start <= end after translation
	//
	// Column translations can produce inverted ranges when:
	// 1. Go code structure differs significantly from Dingo (error_prop, safe_nav, etc.)
	// 2. Column mappings in .dmap are computed from byte offsets before go/printer reformats
	//
	// ROOT CAUSE: error_prop uses byte-level StmtLocation (pkg/ast/stmt_finder.go) instead of
	// token.Pos from the Dingo parser. This violates CLAUDE.md's "no byte arithmetic" principle.
	//
	// PROPER FIX: Integrate error_prop codegen with PositionTracker.RecordTransform() which:
	// 1. Stores token.Pos from Dingo AST during codegen
	// 2. Resolves to line/col AFTER go/printer reformats (via Finalize())
	// 3. Produces accurate column mappings for the .dmap file
	//
	// Until then, swapping ensures the diagnostic range is at least valid for display.
	if newStart.Line == newEnd.Line && newStart.Character > newEnd.Character {
		newStart.Character, newEnd.Character = newEnd.Character, newStart.Character
		log.Printf("[LSP Translator] TranslateRange: swapped inverted range %d-%d (see TODO in translator.go)",
			newStart.Character, newEnd.Character)
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

// dingoToGoPath converts a .dingo path to its corresponding .go path.
// This uses the unified path calculation to respect the configured output directory.
func dingoToGoPath(dingoPath string) string {
	return dingoToGoPathWithConfig(dingoPath, nil)
}

// dingoToGoPathWithConfig converts a .dingo path to its .go path using config.
func dingoToGoPathWithConfig(dingoPath string, cfg *config.Config) string {
	if !strings.HasSuffix(dingoPath, ".dingo") {
		return dingoPath
	}

	// Use unified path calculation
	goPath, err := transpiler.CalculateGoPath(dingoPath, cfg)
	if err != nil {
		// Fallback to simple suffix replacement if calculation fails
		log.Printf("[LSP Translator] dingoToGoPath: calculation failed for %s: %v, using fallback", dingoPath, err)
		return strings.TrimSuffix(dingoPath, ".dingo") + ".go"
	}
	return goPath
}

// goToDingoPath converts a .go path back to its source .dingo path.
func goToDingoPath(goPath string) string {
	return goToDingoPathWithConfig(goPath, nil)
}

// goToDingoPathWithConfig converts a .go path to its .dingo source path using config.
func goToDingoPathWithConfig(goPath string, cfg *config.Config) string {
	if !strings.HasSuffix(goPath, ".go") {
		return goPath
	}

	// Use unified path calculation
	dingoPath, err := transpiler.GoPathToDingoPath(goPath, cfg)
	if err != nil {
		// Fallback to simple suffix replacement if calculation fails
		log.Printf("[LSP Translator] goToDingoPath: calculation failed for %s: %v, using fallback", goPath, err)
		return strings.TrimSuffix(goPath, ".go") + ".dingo"
	}
	return dingoPath
}

// expandTabsInColumn adjusts a column position from Go (tabs) to Dingo (spaces).
// Go files use tabs for indentation, Dingo files use 4 spaces.
// LSP counts tabs as 1 character, so we need to expand: goCol + (tabs * 3)
//
// CLAUDE.md compliance note:
// - Uses token.FileSet for line extraction (compliant)
// - The tab counting loop operates on FINAL generated .go (not source .dingo)
// - This is visual column adjustment, not position tracking in source
// - Tabs here are actual indentation, not transformed code
func expandTabsInColumn(goPath string, line, col int) int {
	src, err := os.ReadFile(goPath)
	if err != nil {
		return col // Can't read file, return unchanged
	}

	// Use token.FileSet to get line boundaries (CLAUDE.md compliant)
	fset := token.NewFileSet()
	file := fset.AddFile(goPath, fset.Base(), len(src))
	file.SetLinesForContent(src)

	if line < 1 || line > file.LineCount() {
		return col // Line out of range
	}

	// Get line start and end offsets using FileSet
	lineStart := file.Offset(file.LineStart(line))
	lineEnd := len(src)
	if line < file.LineCount() {
		lineEnd = file.Offset(file.LineStart(line + 1))
	}

	// Extract line content using FileSet-derived offsets
	lineContent := src[lineStart:lineEnd]

	// Count tabs before the column position
	// This is checking indentation characters, not parsing positions
	tabCount := 0
	limit := col - 1
	if limit > len(lineContent) {
		limit = len(lineContent)
	}
	for i := 0; i < limit; i++ {
		if lineContent[i] == '\t' {
			tabCount++
		}
	}

	// Each tab expands from 1 char to 4 chars, so add 3 per tab
	expandedCol := col + (tabCount * 3)
	if tabCount > 0 {
		log.Printf("[LSP Translator] Tab expansion: col %d -> %d (%d tabs)", col, expandedCol, tabCount)
	}

	return expandedCol
}

// lineDirectiveEntry represents a parsed //line directive
type lineDirectiveEntry struct {
	goLine    int    // Line number in Go file where directive appears (1-indexed)
	dingoLine int    // Target Dingo line from the directive
	filename  string // Target filename (may be empty for relative)
}

// parseLineDirectives reads a Go file and extracts all //line directives using go/scanner.
// This follows CLAUDE.md architectural guidelines: use go/scanner for Go source analysis.
func parseLineDirectives(goPath string) ([]lineDirectiveEntry, error) {
	src, err := os.ReadFile(goPath)
	if err != nil {
		return nil, err
	}

	// Create two file sets:
	// 1. scannerFset - used by the scanner (will be modified by //line directives)
	// 2. lookupFset - for looking up physical line numbers (stays pristine)
	scannerFset := token.NewFileSet()
	scannerFile := scannerFset.AddFile(goPath, scannerFset.Base(), len(src))
	scannerFile.SetLinesForContent(src)

	lookupFset := token.NewFileSet()
	lookupFile := lookupFset.AddFile(goPath, lookupFset.Base(), len(src))
	lookupFile.SetLinesForContent(src)

	var s scanner.Scanner
	s.Init(scannerFile, src, nil, scanner.ScanComments)

	var entries []lineDirectiveEntry

	for {
		pos, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}

		// Look for //line directives in COMMENT tokens
		if tok == token.COMMENT && strings.HasPrefix(lit, "//line ") {
			// Get offset from scanner's position, then lookup physical line in pristine file
			offset := scannerFile.Offset(pos)
			goLine := lookupFset.Position(lookupFile.Pos(offset)).Line

			// Parse the directive: //line filename:line:col or //line filename:line
			// Format: "//line filename:line:col"
			content := strings.TrimPrefix(lit, "//line ")

			// Find the last two colons (for line:col)
			// Work backwards to handle filenames with colons
			lastColon := strings.LastIndex(content, ":")
			if lastColon == -1 {
				continue
			}

			// Check if there's a second-to-last colon (line:col format)
			secondLastColon := strings.LastIndex(content[:lastColon], ":")
			if secondLastColon == -1 {
				continue
			}

			// Parse line number (between second-to-last and last colon)
			lineStr := content[secondLastColon+1 : lastColon]
			dingoLine, err := strconv.Atoi(lineStr)
			if err != nil || dingoLine <= 0 {
				continue
			}

			filename := content[:secondLastColon]

			entries = append(entries, lineDirectiveEntry{
				goLine:    goLine,
				dingoLine: dingoLine,
				filename:  filename,
			})
		}
	}

	return entries, nil
}

// translateUsingLineDirectives translates a Go line to Dingo line using //line directives
// Returns (dingoLine, found) where found indicates if a directive-based translation was used
func translateUsingLineDirectives(goPath string, goLine int) (int, bool) {
	entries, err := parseLineDirectives(goPath)
	if err != nil || len(entries) == 0 {
		return goLine, false
	}

	// Find the last directive that affects this line
	// //line directives affect the NEXT line, so we look for directive at goLine-1 or earlier
	var activeEntry *lineDirectiveEntry
	for i := range entries {
		if entries[i].goLine < goLine {
			activeEntry = &entries[i]
		} else {
			break
		}
	}

	if activeEntry == nil {
		return goLine, false
	}

	// Calculate Dingo line:
	// If directive is at Go line D and sets Dingo line to L,
	// then Go line D+1 → Dingo line L
	// So Go line G → Dingo line L + (G - D - 1)
	dingoLine := activeEntry.dingoLine + (goLine - activeEntry.goLine - 1)
	if dingoLine < 1 {
		dingoLine = 1
	}

	log.Printf("[LSP Translator] LineDirective: Go line %d → Dingo line %d (directive at Go line %d sets Dingo line %d)",
		goLine, dingoLine, activeEntry.goLine, activeEntry.dingoLine)

	return dingoLine, true
}
