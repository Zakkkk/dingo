package rules

import (
	"github.com/MadAppGang/dingo/pkg/tokenizer"
)

// MatchFormatter defines formatting rules for match expressions
type MatchFormatter struct {
	AlignArms bool // Align match arms at => (default: true)
}

// NewMatchFormatter creates a new match formatter with default settings
func NewMatchFormatter() *MatchFormatter {
	return &MatchFormatter{
		AlignArms: true,
	}
}

// Format formats a match expression starting at the given token index
// Returns the index of the last token consumed
//
// Example formatting:
//
//	Before: match x{Some(v)=>v*2,None=>0}
//	After:  match x {
//	            Some(v) => v * 2
//	            None    => 0
//	        }
func (m *MatchFormatter) Format(tokens []tokenizer.Token, startIdx int, writer TokenWriter) int {
	idx := startIdx

	// Write "match"
	writer.WriteToken(tokens[idx])
	idx++

	// Write expression until {
	for idx < len(tokens) && tokens[idx].Kind != tokenizer.LBRACE {
		writer.WriteToken(tokens[idx])
		idx++
	}

	if idx >= len(tokens) {
		return idx - 1
	}

	// Write opening {
	writer.WriteToken(tokens[idx])
	writer.WriteNewline()
	writer.IncreaseIndent()
	idx++

	// Calculate alignment width if needed
	var alignWidth int
	if m.AlignArms {
		alignWidth = m.calculateAlignWidth(tokens, idx)
	}

	// Write match arms
	for idx < len(tokens) && tokens[idx].Kind != tokenizer.RBRACE {
		tok := tokens[idx]

		// Skip newlines - we control newline placement
		if tok.Kind == tokenizer.NEWLINE {
			idx++
			continue
		}

		// Track pattern start for alignment
		patternStart := idx

		// Write pattern until =>
		for idx < len(tokens) && tokens[idx].Kind != tokenizer.ARROW && tokens[idx].Kind != tokenizer.RBRACE {
			if tokens[idx].Kind != tokenizer.NEWLINE {
				writer.WriteToken(tokens[idx])
			}
			idx++
		}

		if idx >= len(tokens) || tokens[idx].Kind == tokenizer.RBRACE {
			break
		}

		// Apply alignment if enabled
		if m.AlignArms && alignWidth > 0 {
			patternWidth := m.calculatePatternWidth(tokens, patternStart, idx)
			padding := alignWidth - patternWidth
			if padding > 0 {
				writer.WriteSpaces(padding)
			}
		}

		// Write =>
		writer.WriteToken(tokens[idx])
		idx++

		// Write expression until comma or } (for next arm)
		depth := 0
		for idx < len(tokens) {
			tok := tokens[idx]
			if tok.Kind == tokenizer.NEWLINE {
				idx++
				continue
			}
			if tok.Kind == tokenizer.LBRACE {
				depth++
			} else if tok.Kind == tokenizer.RBRACE {
				if depth == 0 {
					break // End of match
				}
				depth--
			} else if tok.Kind == tokenizer.COMMA && depth == 0 {
				// End of this arm, skip comma
				idx++
				break
			}
			writer.WriteToken(tok)
			idx++
		}

		// Emit newline after arm (unless we're at closing brace)
		if idx < len(tokens) && tokens[idx].Kind != tokenizer.RBRACE {
			writer.WriteNewline()
		}
	}

	// Write closing }
	writer.DecreaseIndent()
	if idx < len(tokens) && tokens[idx].Kind == tokenizer.RBRACE {
		writer.WriteToken(tokens[idx])
		idx++
	}

	return idx - 1
}

// calculateAlignWidth finds the widest pattern in all arms
func (m *MatchFormatter) calculateAlignWidth(tokens []tokenizer.Token, startIdx int) int {
	maxWidth := 0
	idx := startIdx

	for idx < len(tokens) && tokens[idx].Kind != tokenizer.RBRACE {
		if tokens[idx].Kind == tokenizer.NEWLINE {
			idx++
			continue
		}

		// Measure pattern until =>
		patternStart := idx
		for idx < len(tokens) && tokens[idx].Kind != tokenizer.ARROW && tokens[idx].Kind != tokenizer.RBRACE {
			if tokens[idx].Kind != tokenizer.NEWLINE {
				idx++
			} else {
				idx++
			}
		}

		width := m.calculatePatternWidth(tokens, patternStart, idx)
		if width > maxWidth {
			maxWidth = width
		}

		if idx >= len(tokens) || tokens[idx].Kind == tokenizer.RBRACE {
			break
		}

		// Skip past => and expression
		idx++
		depth := 0
		for idx < len(tokens) {
			tok := tokens[idx]
			if tok.Kind == tokenizer.LBRACE {
				depth++
			} else if tok.Kind == tokenizer.RBRACE {
				if depth == 0 {
					break
				}
				depth--
			} else if tok.Kind == tokenizer.COMMA && depth == 0 {
				idx++
				break
			}
			idx++
		}
	}

	return maxWidth
}

// calculatePatternWidth calculates the display width of a pattern
func (m *MatchFormatter) calculatePatternWidth(tokens []tokenizer.Token, start, end int) int {
	width := 0
	needSpace := false

	for i := start; i < end && i < len(tokens); i++ {
		tok := tokens[i]
		if tok.Kind == tokenizer.NEWLINE {
			continue
		}

		// Add space if needed
		if needSpace && m.needsSpaceBefore(tok) {
			width++
		}

		// Add token width
		if tok.Lit != "" {
			width += len(tok.Lit)
		} else {
			width += len(tok.Kind.String())
		}

		needSpace = m.needsSpaceAfter(tok)
	}

	return width
}

// needsSpaceBefore checks if token needs space before it
func (m *MatchFormatter) needsSpaceBefore(tok tokenizer.Token) bool {
	switch tok.Kind {
	case tokenizer.COMMA, tokenizer.RPAREN, tokenizer.RBRACE, tokenizer.RBRACKET:
		return false
	default:
		return true
	}
}

// needsSpaceAfter checks if token needs space after it
func (m *MatchFormatter) needsSpaceAfter(tok tokenizer.Token) bool {
	switch tok.Kind {
	case tokenizer.LPAREN, tokenizer.LBRACE, tokenizer.LBRACKET:
		return false
	case tokenizer.COMMA:
		return true
	default:
		return true
	}
}

// TokenWriter is the interface that formatters use to write output
// This allows the rules to be independent of the Writer implementation
type TokenWriter interface {
	WriteToken(tok tokenizer.Token)
	WriteNewline()
	WriteSpaces(count int)
	IncreaseIndent()
	DecreaseIndent()
}
