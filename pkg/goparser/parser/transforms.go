// Package parser provides token-based transformation of Dingo syntax to Go.
// This is a simplified transformer that handles Dingo operators (?,,?., etc.)
// as markers that will be processed by later stages.
package parser

import (
	"go/scanner"
	gotoken "go/token"
)

// TokenMapping tracks the relationship between Dingo and Go source positions
type TokenMapping struct {
	DingoStart, DingoEnd int    // Position in original Dingo source
	GoStart, GoEnd       int    // Position in transformed Go source
	Kind                 string // Type of transformation
}

// TransformToGo transforms Dingo source to valid Go source.
// It operates at the token level using Go's scanner.
//
// This is a minimal transformer that:
// 1. Converts ? to markers (?, ??, ?. → comments for now)
// 2. Handles generic syntax (Result[T,E] → Result[T,E])
//
// More complex transformations (enum, match, lambda) are handled by pkg/parser/
// and pkg/codegen/ in the full AST pipeline.
func TransformToGo(src []byte) ([]byte, []TokenMapping, error) {
	// First pass: handle characters that Go's scanner sees as ILLEGAL
	// For now, just replace ? with markers
	src = replaceQuestionMarks(src)

	var mappings []TokenMapping

	// Create a file set for tokenization
	fset := gotoken.NewFileSet()
	file := fset.AddFile("", -1, len(src))

	// Use Go's scanner to tokenize
	var s scanner.Scanner
	s.Init(file, src, nil, scanner.ScanComments)

	// Collect all tokens with their positions
	type tokenInfo struct {
		pos gotoken.Pos
		tok gotoken.Token
		lit string
	}
	var tokens []tokenInfo

	for {
		pos, tok, lit := s.Scan()
		tokens = append(tokens, tokenInfo{pos, tok, lit})
		if tok == gotoken.EOF {
			break
		}
	}

	// Now process tokens and build output by copying source with modifications
	result := make([]byte, 0, len(src))
	lastCopied := 0 // Last byte position we've copied from src

	// State tracking
	genericDepth := 0 // Track depth of generic type brackets

	for i := 0; i < len(tokens)-1; i++ { // -1 because last is EOF
		t := tokens[i]
		offset := file.Offset(t.pos) // Convert Pos to byte offset

		// Handle generic type syntax: Result[T, E] -> Result[T, E]
		// Replace '<' with '[' when after an identifier (type name)
		if t.tok == gotoken.LSS { // '<'
			if i > 0 && tokens[i-1].tok == gotoken.IDENT {
				prevLit := tokens[i-1].lit

				// Check if this is a field access (preceded by .)
				// If so, it's NOT a generic type
				isFieldAccess := false
				if i >= 2 && tokens[i-2].tok == gotoken.PERIOD {
					isFieldAccess = true
				}

				// Check if this looks like generic syntax (not comparison)
				// Known Dingo generic types
				isLikelyGeneric := false
				if !isFieldAccess && len(prevLit) > 0 {
					knownGenerics := map[string]bool{
						"Result": true, "Option": true,
						"Some": true, "None": true, "Ok": true, "Err": true,
					}
					if knownGenerics[prevLit] {
						isLikelyGeneric = true
					} else {
						firstChar := prevLit[0]
						// For other uppercase identifiers, be conservative
						if firstChar >= 'A' && firstChar <= 'Z' {
							isLikelyGeneric = false
						}
					}
				}

				if isLikelyGeneric {
					// Also check that next token looks like a type parameter
					if i+1 < len(tokens) {
						next := tokens[i+1]
						if next.tok == gotoken.IDENT || next.tok == gotoken.MUL || next.tok == gotoken.LBRACK {
							// Copy up to <
							result = append(result, src[lastCopied:offset]...)
							// Replace < with [
							result = append(result, '[')
							lastCopied = offset + 1
							genericDepth++ // Track that we're inside a generic

							mappings = append(mappings, TokenMapping{
								DingoStart: offset,
								DingoEnd:   offset + 1,
								GoStart:    len(result) - 1,
								GoEnd:      len(result),
								Kind:       "generic_open",
							})
							continue
						}
					}
				}
			}
		}

		// Handle generic closing: > -> ] when matching a generic open
		if t.tok == gotoken.GTR && genericDepth > 0 { // '>' only if inside generic
			// Copy up to >
			result = append(result, src[lastCopied:offset]...)
			// Replace > with ]
			result = append(result, ']')
			lastCopied = offset + 1
			genericDepth-- // Decrement generic depth

			mappings = append(mappings, TokenMapping{
				DingoStart: offset,
				DingoEnd:   offset + 1,
				GoStart:    len(result) - 1,
				GoEnd:      len(result),
				Kind:       "generic_close",
			})
			continue
		}
	}

	// Copy remaining bytes
	if lastCopied < len(src) {
		result = append(result, src[lastCopied:]...)
	}

	return result, mappings, nil
}

// replaceQuestionMarks replaces Dingo's ? operators with Go-compatible markers.
// This is a simple character-level pass that handles:
// - ?? → /*DINGO_NULL_COAL*/
// - ?. → /*DINGO_SAFE_NAV*/.
// - ?  → /*DINGO_ERR_PROP*/
//
// These markers are then processed by pkg/parser/ and pkg/codegen/.
func replaceQuestionMarks(src []byte) []byte {
	result := make([]byte, 0, len(src)+100)

	i := 0
	inString := false
	inRawString := false
	stringChar := byte(0)

	for i < len(src) {
		ch := src[i]

		// Track string literals - don't transform inside strings
		if !inString && !inRawString {
			if ch == '"' {
				inString = true
				stringChar = '"'
			} else if ch == '\'' {
				inString = true
				stringChar = '\''
			} else if ch == '`' {
				inRawString = true
			}
		} else if inString {
			if ch == stringChar && (i == 0 || src[i-1] != '\\') {
				inString = false
			}
		} else if inRawString {
			if ch == '`' {
				inRawString = false
			}
		}

		// Only transform ? outside of strings
		if !inString && !inRawString && ch == '?' {
			if i+1 < len(src) && src[i+1] == '?' {
				// ?? -> null coalescing
				result = append(result, " /*DINGO_NULL_COAL*/ "...)
				i += 2
				continue
			}
			if i+1 < len(src) && src[i+1] == '.' {
				// ?. -> safe navigation
				result = append(result, " /*DINGO_SAFE_NAV*/."...)
				i += 2
				continue
			}
			// Single ? -> error propagation
			result = append(result, " /*DINGO_ERR_PROP*/"...)
			i++
			continue
		}
		result = append(result, ch)
		i++
	}

	return result
}
