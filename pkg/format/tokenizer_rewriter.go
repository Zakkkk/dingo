package format

import (
	"bytes"
	"strings"

	"github.com/MadAppGang/dingo/pkg/tokenizer"
)

// Writer handles token-based reformatting with proper spacing and indentation
type Writer struct {
	out    *bytes.Buffer
	config *Config
	src    []byte // Original source for extracting text

	// State
	indent              int  // Current indentation level
	atLineStart         bool // True if we're at the beginning of a line
	lastTokenKind       tokenizer.TokenKind
	needSpace           bool // True if we need to emit a space before next token
	consecutiveNewlines int  // Count of consecutive newlines (for suppressing excessive blank lines)
}

// newWriter creates a new token writer
func newWriter(out *bytes.Buffer, config *Config, src []byte) *Writer {
	return &Writer{
		out:         out,
		config:      config,
		src:         src,
		indent:      0,
		atLineStart: true,
		needSpace:   false,
	}
}

// writeTokens processes all tokens and writes formatted output
func (w *Writer) writeTokens(tokens []tokenizer.Token) error {
	for i := 0; i < len(tokens); i++ {
		tok := tokens[i]

		// Skip EOF
		if tok.Kind == tokenizer.EOF {
			break
		}

		// Handle special structures
		switch tok.Kind {
		case tokenizer.MATCH:
			i = w.writeMatch(tokens, i)
		case tokenizer.ENUM:
			i = w.writeEnum(tokens, i)
		case tokenizer.GUARD:
			i = w.writeGuard(tokens, i)
		case tokenizer.PIPE:
			// Check if this is a lambda (| ... | or |x| expr)
			if w.isLambdaStart(tokens, i) {
				i = w.writeLambda(tokens, i)
			} else {
				w.writeToken(tok)
			}
		case tokenizer.LBRACE:
			// Opening brace - increase indent for next line
			w.writeToken(tok)
			w.increaseIndent()
		case tokenizer.RBRACE:
			// Closing brace - decrease indent before writing
			w.decreaseIndent()
			w.writeToken(tok)
		default:
			w.writeToken(tok)
		}
	}

	// Ensure file ends with newline
	if !w.atLineStart {
		w.out.WriteByte('\n')
	}

	return nil
}

// writeToken writes a single token with appropriate spacing
func (w *Writer) writeToken(tok tokenizer.Token) {
	// Handle comments specially
	if tok.Kind == tokenizer.COMMENT {
		w.consecutiveNewlines = 0 // Comments break newline sequences
		w.writeComment(tok)
		return
	}

	// Handle NEWLINE tokens - preserve them immediately
	if tok.Kind == tokenizer.NEWLINE {
		// Limit to max 2 blank lines (3 consecutive newlines)
		if w.consecutiveNewlines < 3 {
			w.writeNewline()
			w.consecutiveNewlines++
		}
		return
	}

	// Reset consecutive newlines counter for non-newline tokens
	w.consecutiveNewlines = 0

	// Emit indentation if at line start
	if w.atLineStart {
		w.writeIndent()
		w.atLineStart = false
	}

	// Emit space if needed
	if w.needSpace && w.needsSpaceBefore(tok) {
		w.out.WriteByte(' ')
	}

	// Write the token text
	w.out.WriteString(w.tokenText(tok))

	// Update state for next token
	w.needSpace = w.needsSpaceAfter(tok)
	w.lastTokenKind = tok.Kind
}

// writeComment writes a comment token
func (w *Writer) writeComment(tok tokenizer.Token) {
	if w.atLineStart {
		w.writeIndent()
		w.atLineStart = false
	} else if w.needSpace {
		w.out.WriteByte(' ')
	}

	w.out.WriteString(tok.Lit)

	// Line comments: the source will have a NEWLINE token after this
	// so we just mark that we don't need space - the NEWLINE will handle it
	if strings.HasPrefix(tok.Lit, "//") {
		w.needSpace = false
		// Don't call writeNewline() here - let the source NEWLINE token do it
	} else {
		w.needSpace = true
	}

	w.lastTokenKind = tokenizer.COMMENT
}

// writeIndent writes the current indentation
func (w *Writer) writeIndent() {
	indentStr := w.config.IndentString()
	for i := 0; i < w.indent; i++ {
		w.out.WriteString(indentStr)
	}
}

// writeNewline writes a newline and updates state
func (w *Writer) writeNewline() {
	w.out.WriteByte('\n')
	w.atLineStart = true
	w.needSpace = false
}

// increaseIndent increases indentation level (internal)
func (w *Writer) increaseIndent() {
	w.indent++
}

// IncreaseIndent increases indentation level
// This is part of the TokenWriter interface used by formatting rules
func (w *Writer) IncreaseIndent() {
	w.increaseIndent()
}

// decreaseIndent decreases indentation level (internal)
func (w *Writer) decreaseIndent() {
	if w.indent > 0 {
		w.indent--
	}
}

// DecreaseIndent decreases indentation level
// This is part of the TokenWriter interface used by formatting rules
func (w *Writer) DecreaseIndent() {
	w.decreaseIndent()
}

// WriteSpaces writes N space characters
// This is part of the TokenWriter interface used by formatting rules
func (w *Writer) WriteSpaces(count int) {
	for i := 0; i < count; i++ {
		w.out.WriteByte(' ')
	}
}

// WriteToken writes a single token with appropriate spacing
// This is part of the TokenWriter interface used by formatting rules
func (w *Writer) WriteToken(tok tokenizer.Token) {
	w.writeToken(tok)
}

// WriteNewline writes a newline and updates state
// This is part of the TokenWriter interface used by formatting rules
func (w *Writer) WriteNewline() {
	w.writeNewline()
}

// tokenText returns the text to write for a token
func (w *Writer) tokenText(tok tokenizer.Token) string {
	if tok.Lit != "" {
		return tok.Lit
	}
	// Use the token kind's string representation
	return tok.Kind.String()
}

// needsSpaceBefore returns true if we should emit a space before this token
func (w *Writer) needsSpaceBefore(tok tokenizer.Token) bool {
	// Never space before certain tokens
	switch tok.Kind {
	case tokenizer.COMMA, tokenizer.SEMICOLON, tokenizer.COLON, tokenizer.RPAREN,
		tokenizer.RBRACE, tokenizer.RBRACKET, tokenizer.DOT, tokenizer.QUESTION_DOT:
		return false
	case tokenizer.QUESTION:
		// No space before ? in error propagation (x?)
		if w.lastTokenKind == tokenizer.IDENT || w.lastTokenKind == tokenizer.RPAREN {
			return false
		}
	case tokenizer.LPAREN:
		// No space before ( after identifier (function calls)
		if w.lastTokenKind == tokenizer.IDENT {
			return false
		}
	}

	// Never space after certain tokens
	switch w.lastTokenKind {
	case tokenizer.LPAREN, tokenizer.LBRACE, tokenizer.LBRACKET,
		tokenizer.DOT, tokenizer.NOT, tokenizer.QUESTION, tokenizer.QUESTION_DOT:
		return false
	}

	return true
}

// needsSpaceAfter returns true if we should emit space after this token
func (w *Writer) needsSpaceAfter(tok tokenizer.Token) bool {
	switch tok.Kind {
	case tokenizer.COMMA:
		return true // Always space after comma
	case tokenizer.LPAREN, tokenizer.LBRACE, tokenizer.LBRACKET:
		return false // No space after opening delimiters
	case tokenizer.DOT, tokenizer.QUESTION_DOT, tokenizer.QUESTION:
		return false // No space after dots and operators that bind tightly
	case tokenizer.COLON:
		return true // Space after colon
	case tokenizer.SEMICOLON:
		return true // Space after semicolon
	default:
		return true // Default: emit space
	}
}

// needsNewlineBefore returns true if we should emit newline before this token
func (w *Writer) needsNewlineBefore(tok tokenizer.Token) bool {
	// Newline before top-level declarations (but not after package)
	switch tok.Kind {
	case tokenizer.FUNC, tokenizer.TYPE, tokenizer.CONST, tokenizer.VAR, tokenizer.IMPORT:
		return true
	case tokenizer.LET:
		// Let after package should be on new line
		if w.lastTokenKind == tokenizer.IDENT && w.atLineStart == false {
			return true
		}
	}
	return false
}

// writeMatch formats a match expression
// Returns the index of the last token consumed
func (w *Writer) writeMatch(tokens []tokenizer.Token, startIdx int) int {
	idx := startIdx

	// Write "match"
	w.writeToken(tokens[idx])
	idx++

	// Write expression until {
	for idx < len(tokens) && tokens[idx].Kind != tokenizer.LBRACE {
		w.writeToken(tokens[idx])
		idx++
	}

	if idx >= len(tokens) {
		return idx - 1
	}

	// Write opening { and let source newlines handle formatting
	w.writeToken(tokens[idx])
	w.increaseIndent()
	idx++

	// Write all tokens until closing }, preserving source formatting
	depth := 0
	for idx < len(tokens) && (tokens[idx].Kind != tokenizer.RBRACE || depth > 0) {
		tok := tokens[idx]

		// Track brace depth for nested blocks
		if tok.Kind == tokenizer.LBRACE {
			depth++
		} else if tok.Kind == tokenizer.RBRACE {
			if depth == 0 {
				break // End of match
			}
			depth--
		}

		// Write all tokens including newlines
		w.writeToken(tok)
		idx++
	}

	// Write closing }
	w.decreaseIndent()
	if idx < len(tokens) && tokens[idx].Kind == tokenizer.RBRACE {
		w.writeToken(tokens[idx])
		idx++
	}

	return idx - 1
}

// writeEnum formats an enum declaration
// Returns the index of the last token consumed
func (w *Writer) writeEnum(tokens []tokenizer.Token, startIdx int) int {
	idx := startIdx

	// Write "enum"
	w.writeToken(tokens[idx])
	idx++

	// Write name
	if idx < len(tokens) && tokens[idx].Kind == tokenizer.IDENT {
		w.writeToken(tokens[idx])
		idx++
	}

	// Write until {
	for idx < len(tokens) && tokens[idx].Kind != tokenizer.LBRACE {
		w.writeToken(tokens[idx])
		idx++
	}

	if idx >= len(tokens) {
		return idx - 1
	}

	// Write opening { and increase indent
	w.writeToken(tokens[idx])
	w.increaseIndent()
	idx++

	// Write all tokens until matching closing }, tracking brace depth
	// for nested braces in variant fields like: Active { id: int }
	braceDepth := 1
	for idx < len(tokens) && braceDepth > 0 {
		tok := tokens[idx]
		if tok.Kind == tokenizer.LBRACE {
			braceDepth++
		} else if tok.Kind == tokenizer.RBRACE {
			braceDepth--
			if braceDepth == 0 {
				break // Don't write the closing } yet
			}
		}
		w.writeToken(tok)
		idx++
	}

	// Write closing }
	w.decreaseIndent()
	if idx < len(tokens) && tokens[idx].Kind == tokenizer.RBRACE {
		w.writeToken(tokens[idx])
		idx++
	}

	return idx - 1
}

// writeGuard formats a guard statement
// Returns the index of the last token consumed
func (w *Writer) writeGuard(tokens []tokenizer.Token, startIdx int) int {
	idx := startIdx

	// Write "guard"
	w.writeToken(tokens[idx])
	idx++

	// Write until "else"
	for idx < len(tokens) && tokens[idx].Kind != tokenizer.ELSE {
		w.writeToken(tokens[idx])
		idx++
	}

	if idx >= len(tokens) {
		return idx - 1
	}

	// Write "else"
	w.writeToken(tokens[idx])
	idx++

	// Write block
	if idx < len(tokens) && tokens[idx].Kind == tokenizer.LBRACE {
		// Write opening { and let source newlines handle formatting
		w.writeToken(tokens[idx])
		w.increaseIndent()
		idx++

		// Write all tokens until closing }
		depth := 1
		for idx < len(tokens) && depth > 0 {
			tok := tokens[idx]
			if tok.Kind == tokenizer.LBRACE {
				depth++
			} else if tok.Kind == tokenizer.RBRACE {
				depth--
				if depth == 0 {
					break
				}
			}
			w.writeToken(tok)
			idx++
		}

		// Write closing }
		w.decreaseIndent()
		if idx < len(tokens) && tokens[idx].Kind == tokenizer.RBRACE {
			w.writeToken(tokens[idx])
			idx++
		}
	}

	return idx - 1
}

// isLambdaStart checks if a PIPE token starts a lambda expression
func (w *Writer) isLambdaStart(tokens []tokenizer.Token, idx int) bool {
	if tokens[idx].Kind != tokenizer.PIPE {
		return false
	}

	// Look ahead for closing pipe
	for i := idx + 1; i < len(tokens); i++ {
		switch tokens[i].Kind {
		case tokenizer.PIPE:
			// Found closing pipe - check for arrow or expression
			if i+1 < len(tokens) {
				next := tokens[i+1].Kind
				return next == tokenizer.ARROW || next == tokenizer.THIN_ARROW
			}
			return true
		case tokenizer.LBRACE, tokenizer.SEMICOLON, tokenizer.NEWLINE:
			// Not a lambda
			return false
		}
	}

	return false
}

// writeLambda formats a lambda expression
// Returns the index of the last token consumed
func (w *Writer) writeLambda(tokens []tokenizer.Token, startIdx int) int {
	idx := startIdx

	// Write opening |
	w.writeToken(tokens[idx])
	idx++

	// Write parameters until closing |
	paramStart := idx
	for idx < len(tokens) && tokens[idx].Kind != tokenizer.PIPE {
		if idx > paramStart && tokens[idx].Kind == tokenizer.COMMA {
			w.writeToken(tokens[idx]) // comma
			idx++
			if w.config.LambdaSpacing {
				w.needSpace = true
			}
		} else {
			w.writeToken(tokens[idx])
			idx++
		}
	}

	// Write closing |
	if idx < len(tokens) && tokens[idx].Kind == tokenizer.PIPE {
		w.writeToken(tokens[idx])
		idx++
	}

	// Write arrow (=> or ->)
	if idx < len(tokens) && (tokens[idx].Kind == tokenizer.ARROW || tokens[idx].Kind == tokenizer.THIN_ARROW) {
		if w.config.LambdaSpacing {
			w.needSpace = true
		}
		w.writeToken(tokens[idx])
		idx++
		if w.config.LambdaSpacing {
			w.needSpace = true
		}
	}

	// Write expression (until comma, semicolon, or structural boundary)
	depth := 0
	for idx < len(tokens) {
		tok := tokens[idx]
		switch tok.Kind {
		case tokenizer.LPAREN, tokenizer.LBRACE, tokenizer.LBRACKET:
			depth++
		case tokenizer.RPAREN, tokenizer.RBRACE, tokenizer.RBRACKET:
			if depth == 0 {
				// End of lambda expression
				return idx - 1
			}
			depth--
		case tokenizer.COMMA, tokenizer.SEMICOLON:
			if depth == 0 {
				// End of lambda expression
				return idx - 1
			}
		case tokenizer.NEWLINE:
			if depth == 0 {
				return idx - 1
			}
		}
		w.writeToken(tok)
		idx++
	}

	return idx - 1
}
