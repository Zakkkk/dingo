package rules

import (
	"github.com/MadAppGang/dingo/pkg/tokenizer"
)

// EnumFormatter defines formatting rules for enum declarations
type EnumFormatter struct {
	OneVariantPerLine bool // Force one variant per line (default: true)
	AlignTypes        bool // Align variant type annotations (default: false)
}

// NewEnumFormatter creates a new enum formatter with default settings
func NewEnumFormatter() *EnumFormatter {
	return &EnumFormatter{
		OneVariantPerLine: true,
		AlignTypes:        false,
	}
}

// Format formats an enum declaration starting at the given token index
// Returns the index of the last token consumed
//
// Example formatting:
//
//	Before: enum Status{Active,Inactive(string),Pending}
//	After:  enum Status {
//	            Active
//	            Inactive(string)
//	            Pending
//	        }
func (e *EnumFormatter) Format(tokens []tokenizer.Token, startIdx int, writer TokenWriter) int {
	idx := startIdx

	// Write "enum"
	writer.WriteToken(tokens[idx])
	idx++

	// Write name
	if idx < len(tokens) && tokens[idx].Kind == tokenizer.IDENT {
		writer.WriteToken(tokens[idx])
		idx++
	}

	// Write generic parameters if present (e.g., enum Result[T, E])
	if idx < len(tokens) && tokens[idx].Kind == tokenizer.LBRACKET {
		writer.WriteToken(tokens[idx])
		idx++

		// Write generic params until ]
		for idx < len(tokens) && tokens[idx].Kind != tokenizer.RBRACKET {
			writer.WriteToken(tokens[idx])
			idx++
		}

		if idx < len(tokens) && tokens[idx].Kind == tokenizer.RBRACKET {
			writer.WriteToken(tokens[idx])
			idx++
		}
	}

	// Write until {
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

	// Write variants
	for idx < len(tokens) && tokens[idx].Kind != tokenizer.RBRACE {
		tok := tokens[idx]

		// Skip newlines - we control formatting
		if tok.Kind == tokenizer.NEWLINE {
			idx++
			continue
		}

		// Skip commas - we'll add newlines instead
		if tok.Kind == tokenizer.COMMA {
			idx++
			continue
		}

		// Preserve comments
		if tok.Kind == tokenizer.COMMENT {
			writer.WriteToken(tok)
			idx++
			if idx < len(tokens) && tokens[idx].Kind == tokenizer.NEWLINE {
				writer.WriteNewline()
				idx++
			}
			continue
		}

		// Write variant (name + optional type)
		for idx < len(tokens) && !e.isVariantEnd(tokens[idx]) {
			writer.WriteToken(tokens[idx])
			idx++
		}

		// Emit newline after variant (unless it's the last one before })
		if idx < len(tokens) {
			next := tokens[idx]
			if next.Kind == tokenizer.COMMA {
				idx++ // Skip comma
			}

			// Look ahead to see if closing brace is next (ignoring whitespace)
			nextIdx := idx
			for nextIdx < len(tokens) && (tokens[nextIdx].Kind == tokenizer.NEWLINE || tokens[nextIdx].Kind == tokenizer.COMMENT) {
				if tokens[nextIdx].Kind == tokenizer.COMMENT {
					break // Don't skip comments when looking ahead
				}
				nextIdx++
			}

			if nextIdx < len(tokens) && tokens[nextIdx].Kind != tokenizer.RBRACE {
				writer.WriteNewline()
			}
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

// isVariantEnd checks if a token marks the end of a variant declaration
func (e *EnumFormatter) isVariantEnd(tok tokenizer.Token) bool {
	return tok.Kind == tokenizer.COMMA ||
		tok.Kind == tokenizer.RBRACE ||
		tok.Kind == tokenizer.NEWLINE ||
		tok.Kind == tokenizer.COMMENT
}

// FormatCompact formats an enum on a single line (for simple enums)
// This is useful for enums with short variant names and no type parameters
//
// Example: enum Status { Active, Inactive, Pending }
func (e *EnumFormatter) FormatCompact(tokens []tokenizer.Token, startIdx int, writer TokenWriter) int {
	idx := startIdx

	// Write "enum"
	writer.WriteToken(tokens[idx])
	idx++

	// Write name
	if idx < len(tokens) && tokens[idx].Kind == tokenizer.IDENT {
		writer.WriteToken(tokens[idx])
		idx++
	}

	// Write until {
	for idx < len(tokens) && tokens[idx].Kind != tokenizer.LBRACE {
		writer.WriteToken(tokens[idx])
		idx++
	}

	if idx >= len(tokens) {
		return idx - 1
	}

	// Write opening {
	writer.WriteToken(tokens[idx])
	idx++

	// Write variants on same line
	firstVariant := true
	for idx < len(tokens) && tokens[idx].Kind != tokenizer.RBRACE {
		tok := tokens[idx]

		if tok.Kind == tokenizer.NEWLINE {
			idx++
			continue
		}

		if tok.Kind == tokenizer.COMMA {
			writer.WriteToken(tok)
			idx++
			continue
		}

		// Don't write space before first variant
		if !firstVariant {
			// Space is already handled by token writer
		}
		firstVariant = false

		// Write variant
		for idx < len(tokens) && !e.isVariantEnd(tokens[idx]) {
			writer.WriteToken(tokens[idx])
			idx++
		}
	}

	// Write closing }
	if idx < len(tokens) && tokens[idx].Kind == tokenizer.RBRACE {
		writer.WriteToken(tokens[idx])
		idx++
	}

	return idx - 1
}
