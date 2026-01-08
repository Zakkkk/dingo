package rules

import (
	"github.com/MadAppGang/dingo/pkg/tokenizer"
)

// LambdaFormatter defines formatting rules for lambda expressions
type LambdaFormatter struct {
	SpacingAroundArrow bool // Add spaces around => or -> (default: true)
	SpaceAfterPipe     bool // Add space after opening | (default: false)
	SpaceBeforePipe    bool // Add space before closing | (default: false)
}

// NewLambdaFormatter creates a new lambda formatter with default settings
func NewLambdaFormatter() *LambdaFormatter {
	return &LambdaFormatter{
		SpacingAroundArrow: true,
		SpaceAfterPipe:     false,
		SpaceBeforePipe:    false,
	}
}

// Format formats a lambda expression starting at the given token index
// Returns the index of the last token consumed
//
// Example formatting:
//
//	Before: |x,y|x+y
//	After:  |x, y| => x + y
//
//	Before: |x|x*2
//	After:  |x| => x * 2
//
// Supports both syntaxes:
//
//	|x| => expr  (Rust-style with =>)
//	|x| -> expr  (Alternative with ->)
//	x => expr    (Single param shorthand)
func (l *LambdaFormatter) Format(tokens []tokenizer.Token, startIdx int, writer TokenWriter) int {
	idx := startIdx

	// Check for single-param shorthand: ident => expr (no pipes)
	if tokens[idx].Kind == tokenizer.IDENT {
		if idx+1 < len(tokens) && (tokens[idx+1].Kind == tokenizer.ARROW || tokens[idx+1].Kind == tokenizer.THIN_ARROW) {
			return l.formatShorthand(tokens, idx, writer)
		}
	}

	// Standard lambda: |params| => expr
	if tokens[idx].Kind != tokenizer.PIPE {
		return idx - 1
	}

	// Write opening |
	writer.WriteToken(tokens[idx])
	idx++

	if l.SpaceAfterPipe {
		writer.WriteSpaces(1)
	}

	// Write parameters until closing |
	paramCount := 0
	for idx < len(tokens) && tokens[idx].Kind != tokenizer.PIPE {
		tok := tokens[idx]

		if tok.Kind == tokenizer.COMMA {
			writer.WriteToken(tok)
			idx++
			paramCount++
			// Always space after comma in param list
			continue
		}

		writer.WriteToken(tok)
		idx++
	}

	if l.SpaceBeforePipe && paramCount > 0 {
		writer.WriteSpaces(1)
	}

	// Write closing |
	if idx < len(tokens) && tokens[idx].Kind == tokenizer.PIPE {
		writer.WriteToken(tokens[idx])
		idx++
	}

	// Write arrow (=> or ->)
	if idx < len(tokens) && (tokens[idx].Kind == tokenizer.ARROW || tokens[idx].Kind == tokenizer.THIN_ARROW) {
		if l.SpacingAroundArrow {
			writer.WriteSpaces(1)
		}
		writer.WriteToken(tokens[idx])
		idx++
		if l.SpacingAroundArrow {
			writer.WriteSpaces(1)
		}
	}

	// Write lambda body (expression until structural boundary)
	idx = l.formatLambdaBody(tokens, idx, writer)

	return idx
}

// formatShorthand formats single-parameter shorthand lambda: x => expr
func (l *LambdaFormatter) formatShorthand(tokens []tokenizer.Token, startIdx int, writer TokenWriter) int {
	idx := startIdx

	// Write parameter
	writer.WriteToken(tokens[idx])
	idx++

	// Write arrow
	if idx < len(tokens) && (tokens[idx].Kind == tokenizer.ARROW || tokens[idx].Kind == tokenizer.THIN_ARROW) {
		if l.SpacingAroundArrow {
			writer.WriteSpaces(1)
		}
		writer.WriteToken(tokens[idx])
		idx++
		if l.SpacingAroundArrow {
			writer.WriteSpaces(1)
		}
	}

	// Write expression
	idx = l.formatLambdaBody(tokens, idx, writer)

	return idx
}

// formatLambdaBody formats the body expression of a lambda
func (l *LambdaFormatter) formatLambdaBody(tokens []tokenizer.Token, startIdx int, writer TokenWriter) int {
	idx := startIdx
	depth := 0

	for idx < len(tokens) {
		tok := tokens[idx]

		switch tok.Kind {
		case tokenizer.LPAREN, tokenizer.LBRACE, tokenizer.LBRACKET:
			depth++
			writer.WriteToken(tok)
			idx++

		case tokenizer.RPAREN, tokenizer.RBRACE, tokenizer.RBRACKET:
			if depth == 0 {
				// End of lambda expression
				return idx - 1
			}
			depth--
			writer.WriteToken(tok)
			idx++

		case tokenizer.COMMA, tokenizer.SEMICOLON:
			if depth == 0 {
				// End of lambda expression
				return idx - 1
			}
			writer.WriteToken(tok)
			idx++

		case tokenizer.NEWLINE:
			if depth == 0 {
				// End of lambda expression
				return idx - 1
			}
			// Inside nested structure, preserve newline
			idx++

		default:
			writer.WriteToken(tok)
			idx++
		}
	}

	return idx - 1
}

// IsLambdaStart checks if a token sequence starts a lambda expression
// This is useful for the main formatter to detect when to delegate to LambdaFormatter
func IsLambdaStart(tokens []tokenizer.Token, idx int) bool {
	if idx >= len(tokens) {
		return false
	}

	// Check for pipe-based lambda: |...
	if tokens[idx].Kind == tokenizer.PIPE {
		// Look ahead for closing pipe
		for i := idx + 1; i < len(tokens); i++ {
			switch tokens[i].Kind {
			case tokenizer.PIPE:
				// Found closing pipe - check for arrow
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

	// Check for shorthand: ident => expr
	if tokens[idx].Kind == tokenizer.IDENT {
		if idx+1 < len(tokens) {
			next := tokens[idx+1].Kind
			return next == tokenizer.ARROW || next == tokenizer.THIN_ARROW
		}
	}

	return false
}
