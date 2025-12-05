package tokenizer

import (
	"go/token"
)

// IncrementalTokenizer supports efficient re-tokenization of changed text regions
// This is critical for LSP performance where we need to update tokens on every keystroke
type IncrementalTokenizer struct {
	source     []byte
	tokens     []Token
	positions  []int // Byte offset to token index mapping (for fast lookup)
	fset       *token.FileSet
	filename   string
	version    int // Incremented on each update for cache invalidation
	tokenizer  *Tokenizer
}

// NewIncremental creates an incremental tokenizer for the given source
func NewIncremental(src []byte, fset *token.FileSet, filename string) (*IncrementalTokenizer, error) {
	it := &IncrementalTokenizer{
		source:   src,
		fset:     fset,
		filename: filename,
		version:  1,
	}

	// Initial tokenization
	err := it.retokenizeFull()
	if err != nil {
		return nil, err
	}

	return it, nil
}

// retokenizeFull performs full tokenization (used for initial load)
func (it *IncrementalTokenizer) retokenizeFull() error {
	it.tokenizer = NewWithFileSet(it.source, it.fset, it.filename)
	tokens, err := it.tokenizer.Tokenize()
	if err != nil {
		return err
	}

	it.tokens = tokens
	it.buildPositionIndex()
	return nil
}

// buildPositionIndex creates fast byte-offset to token-index mapping
func (it *IncrementalTokenizer) buildPositionIndex() {
	if len(it.tokens) == 0 {
		it.positions = nil
		return
	}

	// Allocate positions array sized to source length
	it.positions = make([]int, len(it.source)+1)

	// Map each byte offset to its containing token index
	tokenIdx := 0
	for i := 0; i <= len(it.source); i++ {
		// Advance to next token if we've passed current token's end
		for tokenIdx < len(it.tokens)-1 && int(it.tokens[tokenIdx].End) <= i {
			tokenIdx++
		}
		it.positions[i] = tokenIdx
	}
}

// Retokenize updates tokens for a text change
// start, end are byte offsets in the old source
// newText is the replacement text
func (it *IncrementalTokenizer) Retokenize(start, end int, newText []byte) error {
	// Update source text
	newSource := make([]byte, 0, len(it.source)-int(end-start)+len(newText))
	newSource = append(newSource, it.source[:start]...)
	newSource = append(newSource, newText...)
	newSource = append(newSource, it.source[end:]...)
	it.source = newSource

	// Find affected token range with safety margin
	// We need to retokenize a bit before/after to handle cases where
	// the edit splits/merges tokens (e.g., "x? ?" -> "x??")
	const safetyMargin = 10 // bytes

	affectedStart := max(0, start-safetyMargin)
	affectedEnd := min(len(it.source), start+len(newText)+safetyMargin)

	// Find token indices for affected range
	startTokenIdx := it.tokenAt(affectedStart)
	endTokenIdx := it.tokenAt(affectedEnd)

	// Extend to token boundaries
	if startTokenIdx < len(it.tokens) {
		affectedStart = int(it.tokens[startTokenIdx].Pos)
	}
	if endTokenIdx < len(it.tokens) {
		affectedEnd = int(it.tokens[endTokenIdx].End)
	}

	// Re-tokenize affected region
	affectedSource := it.source[affectedStart:affectedEnd]
	tmpTokenizer := NewWithFileSet(affectedSource, it.fset, it.filename)
	newTokens, err := tmpTokenizer.Tokenize()
	if err != nil {
		// On error, fall back to full retokenization
		return it.retokenizeFull()
	}

	// Adjust positions for the new tokens (they're relative to affectedStart)
	positionOffset := token.Pos(affectedStart)
	for i := range newTokens {
		newTokens[i].Pos += positionOffset
		newTokens[i].End += positionOffset
	}

	// Splice new tokens into existing token stream
	it.tokens = spliceTokens(it.tokens, startTokenIdx, endTokenIdx+1, newTokens)

	// Rebuild position index
	it.buildPositionIndex()

	// Increment version
	it.version++

	return nil
}

// tokenAt returns the token index containing the given byte offset
func (it *IncrementalTokenizer) tokenAt(offset int) int {
	if offset < 0 || offset >= len(it.positions) {
		if len(it.tokens) > 0 {
			return len(it.tokens) - 1
		}
		return 0
	}
	return it.positions[offset]
}

// spliceTokens replaces tokens[start:end] with newTokens
func spliceTokens(tokens []Token, start, end int, newTokens []Token) []Token {
	result := make([]Token, 0, len(tokens)-int(end-start)+len(newTokens))
	result = append(result, tokens[:start]...)
	result = append(result, newTokens...)
	if end < len(tokens) {
		result = append(result, tokens[end:]...)
	}
	return result
}

// Tokens returns the current token stream
func (it *IncrementalTokenizer) Tokens() []Token {
	return it.tokens
}

// Version returns the current version (incremented on each update)
func (it *IncrementalTokenizer) Version() int {
	return it.version
}

// Source returns the current source text
func (it *IncrementalTokenizer) Source() []byte {
	return it.source
}

// TokenAtPosition returns the token at the given byte offset
func (it *IncrementalTokenizer) TokenAtPosition(offset int) *Token {
	idx := it.tokenAt(offset)
	if idx >= 0 && idx < len(it.tokens) {
		return &it.tokens[idx]
	}
	return nil
}

// TokensInRange returns all tokens that overlap with the given byte range
func (it *IncrementalTokenizer) TokensInRange(start, end int) []Token {
	if start < 0 || start >= len(it.source) {
		return nil
	}

	startIdx := it.tokenAt(start)
	endIdx := it.tokenAt(end)

	if endIdx >= len(it.tokens) {
		endIdx = len(it.tokens) - 1
	}

	if startIdx > endIdx || startIdx >= len(it.tokens) {
		return nil
	}

	return it.tokens[startIdx : endIdx+1]
}

// Helper functions for min/max
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
