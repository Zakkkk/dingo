package preprocessor

import (
	"bytes"
	"fmt"
	"go/scanner"
	"go/token"
	"strings"
)

// LetASTProcessor converts Dingo let keyword to Go using token-based parsing
// This replaces the buggy regex-based approach in keywords.go
type LetASTProcessor struct{}

// NewLetASTProcessor creates a new let processor
func NewLetASTProcessor() *LetASTProcessor {
	return &LetASTProcessor{}
}

// Name returns the processor name
func (p *LetASTProcessor) Name() string {
	return "let-ast"
}

// Process transforms Dingo let declarations to Go using token-based parsing
// Converts:
//   - let x = value          → x := value
//   - let x Type             → var x Type
//   - let x: Type = value    → var x Type = value
//   - let (a, b) = tuple     → a, b := tuple (destructuring - passthrough for now)
func (p *LetASTProcessor) Process(source []byte) ([]byte, []Mapping, error) {
	lines := bytes.Split(source, []byte("\n"))
	result := make([][]byte, len(lines))

	for i, line := range lines {
		transformed, err := p.processLine(line)
		if err != nil {
			return nil, nil, fmt.Errorf("line %d: %w", i+1, err)
		}
		result[i] = transformed
	}

	return bytes.Join(result, []byte("\n")), nil, nil
}

// processLine processes a single line, transforming let declarations
func (p *LetASTProcessor) processLine(line []byte) ([]byte, error) {
	// Quick check: does line contain "let" keyword?
	if !bytes.Contains(line, []byte("let")) {
		return line, nil
	}

	// Tokenize the line
	var s scanner.Scanner
	fset := token.NewFileSet()
	file := fset.AddFile("", fset.Base(), len(line))
	s.Init(file, line, nil, scanner.ScanComments)

	// Look for "let" keyword
	tokens := []tokenInfo{}
	for {
		pos, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}
		tokens = append(tokens, tokenInfo{
			pos: pos,
			tok: tok,
			lit: lit,
		})
	}

	// Find "let" keyword
	letIdx := -1
	for i, t := range tokens {
		if t.tok == token.IDENT && t.lit == "let" {
			letIdx = i
			break
		}
	}

	// No "let" found
	if letIdx == -1 {
		return line, nil
	}

	// Check if "let" is in a comment
	if p.isInComment(line, tokens, letIdx) {
		return line, nil
	}

	// Try to parse as let declaration
	transformed, ok := p.transformLetDecl(line, tokens, letIdx)
	if ok {
		return transformed, nil
	}

	// Not a valid let declaration - return unchanged
	return line, nil
}

// tokenInfo holds token information
type tokenInfo struct {
	pos token.Pos
	tok token.Token
	lit string
}

// isInComment checks if the let keyword is inside a comment
func (p *LetASTProcessor) isInComment(line []byte, tokens []tokenInfo, letIdx int) bool {
	if letIdx == 0 {
		return false
	}

	// Check for comment token before let
	for i := 0; i < letIdx; i++ {
		if tokens[i].tok == token.COMMENT {
			return true
		}
	}

	// Check for // comment before let position
	letPos := int(tokens[letIdx].pos) - 1 // token.Pos is 1-based
	if letPos > 1 {
		prefix := string(line[:letPos])
		if idx := strings.Index(prefix, "//"); idx != -1 {
			return true
		}
	}

	return false
}

// transformLetDecl attempts to transform a let declaration
// Returns (transformed line, true) if successful, (original, false) otherwise
func (p *LetASTProcessor) transformLetDecl(line []byte, tokens []tokenInfo, letIdx int) ([]byte, bool) {
	// Need at least: let IDENT [...]
	if letIdx+1 >= len(tokens) {
		return line, false
	}

	// Next token should be identifier or (
	nextTok := tokens[letIdx+1]

	// Handle destructuring: let (a, b) = value
	if nextTok.tok == token.LPAREN {
		// For now, just replace "let" with empty to get: (a, b) = value
		// Go will handle this as: a, b := value (if in short decl context)
		// This is a passthrough - actual destructuring handled elsewhere
		return p.replaceToken(line, tokens[letIdx], ""), true
	}

	// Must be identifier
	if nextTok.tok != token.IDENT {
		return line, false
	}

	// Look for what comes after identifier
	// Possibilities:
	//   - let x = value          → x := value
	//   - let x Type             → var x Type
	//   - let x: Type = value    → var x Type = value
	//   - let x, y = value       → x, y := value

	// Find the next significant token (skip whitespace conceptually)
	afterIdentIdx := letIdx + 2
	if afterIdentIdx >= len(tokens) {
		// Just "let x" - treat as incomplete declaration
		return line, false
	}

	afterIdent := tokens[afterIdentIdx]

	switch afterIdent.tok {
	case token.ASSIGN:
		// let x = value → x := value
		// Replace "let " with empty, replace "=" with ":="
		return p.transformShortDecl(line, tokens, letIdx, afterIdentIdx)

	case token.COLON:
		// let x: Type = value → var x Type = value
		// OR let x: Type (declaration only)
		return p.transformTypedDecl(line, tokens, letIdx, afterIdentIdx)

	case token.COMMA:
		// let x, y = value → x, y := value
		// Replace "let " with empty
		return p.replaceToken(line, tokens[letIdx], ""), true

	case token.IDENT:
		// let x Type (no colon, no equals)
		// let x Type → var x Type
		return p.transformVarDecl(line, tokens, letIdx)

	case token.LBRACK, token.MUL, token.MAP, token.CHAN, token.STRUCT, token.INTERFACE, token.FUNC:
		// let x []Type or let x *Type or let x map[K]V or let x chan T or let x struct{...}
		// These are type declarations: let x []int → var x []int
		return p.transformVarDecl(line, tokens, letIdx)

	default:
		// Unrecognized pattern
		return line, false
	}
}

// transformShortDecl transforms: let x = value → x := value
func (p *LetASTProcessor) transformShortDecl(line []byte, tokens []tokenInfo, letIdx, assignIdx int) ([]byte, bool) {
	// Replace "let " with empty (removes "let" keyword)
	// The "=" is already in place, we just need to ensure ":=" instead

	// Strategy: Remove "let", change "=" to ":="
	result := p.replaceToken(line, tokens[letIdx], "")

	// Now find "=" in result and replace with ":="
	// Adjust position since we removed "let "
	letLen := len("let")
	adjustedAssignPos := int(tokens[assignIdx].pos) - 1 - letLen

	if adjustedAssignPos >= 0 && adjustedAssignPos < len(result) && result[adjustedAssignPos] == '=' {
		// Replace single '=' with ':='
		result = append(result[:adjustedAssignPos], append([]byte(":="), result[adjustedAssignPos+1:]...)...)
		return result, true
	}

	// Fallback: use simple replacement
	return bytes.Replace(result, []byte("="), []byte(":="), 1), true
}

// transformTypedDecl transforms: let x: Type = value → var x Type = value
func (p *LetASTProcessor) transformTypedDecl(line []byte, tokens []tokenInfo, letIdx, colonIdx int) ([]byte, bool) {
	// Replace "let" with "var"
	// Remove the ":"

	result := p.replaceToken(line, tokens[letIdx], "var")

	// Find and remove the colon
	// Adjust position since we may have changed "let" length
	adjustment := len("var") - len("let")
	colonPos := int(tokens[colonIdx].pos) - 1 + adjustment

	if colonPos >= 0 && colonPos < len(result) && result[colonPos] == ':' {
		// Remove colon by slicing it out
		result = append(result[:colonPos], result[colonPos+1:]...)
		return result, true
	}

	// Fallback: simple replacement
	return bytes.Replace(result, []byte(":"), []byte(""), 1), true
}

// transformVarDecl transforms: let x Type → var x Type
func (p *LetASTProcessor) transformVarDecl(line []byte, tokens []tokenInfo, letIdx int) ([]byte, bool) {
	// Simply replace "let" with "var"
	return p.replaceToken(line, tokens[letIdx], "var"), true
}

// replaceToken replaces a token at given position with new text
func (p *LetASTProcessor) replaceToken(line []byte, tok tokenInfo, replacement string) []byte {
	// token.Pos is 1-based, convert to 0-based
	start := int(tok.pos) - 1
	end := start + len(tok.lit)

	if start < 0 || end > len(line) {
		// Invalid position, return unchanged
		return line
	}

	// When removing a token (empty replacement), also remove trailing space if present
	if replacement == "" && end < len(line) && line[end] == ' ' {
		end++ // Include the space after "let"
	}

	result := make([]byte, 0, len(line)+len(replacement)-len(tok.lit))
	result = append(result, line[:start]...)
	result = append(result, []byte(replacement)...)
	result = append(result, line[end:]...)

	return result
}
