package rules

import (
	"github.com/MadAppGang/dingo/pkg/tokenizer"
)

// SpacingRules defines general whitespace normalization rules
type SpacingRules struct {
	// Operators
	SpaceAroundBinaryOps  bool // Space around +, -, *, /, ==, etc. (default: true)
	SpaceAroundAssignment bool // Space around =, := (default: true)
	SpaceAfterComma       bool // Space after comma (default: true)
	SpaceAfterColon       bool // Space after colon in key:value (default: true)

	// Delimiters
	NoSpaceInsideParens   bool // No space after ( or before ) (default: true)
	NoSpaceInsideBrackets bool // No space after [ or before ] (default: true)
	NoSpaceInsideBraces   bool // No space after { or before } (default: false, for {x} vs { x })

	// Function calls
	NoSpaceBeforeCallParen bool // No space between func and ( in calls (default: true)

	// Dingo-specific
	NoSpaceBeforeQuestion bool // No space before ? in error propagation (default: true)
	SpaceAroundNullCoal   bool // Space around ?? operator (default: true)
	NoSpaceAroundSafeNav  bool // No space around ?. operator (default: true)
}

// NewSpacingRules creates spacing rules with default settings
func NewSpacingRules() *SpacingRules {
	return &SpacingRules{
		SpaceAroundBinaryOps:   true,
		SpaceAroundAssignment:  true,
		SpaceAfterComma:        true,
		SpaceAfterColon:        true,
		NoSpaceInsideParens:    true,
		NoSpaceInsideBrackets:  true,
		NoSpaceInsideBraces:    false,
		NoSpaceBeforeCallParen: true,
		NoSpaceBeforeQuestion:  true,
		SpaceAroundNullCoal:    true,
		NoSpaceAroundSafeNav:   true,
	}
}

// NeedsSpaceBefore returns true if a space should be emitted before this token
func (s *SpacingRules) NeedsSpaceBefore(tok tokenizer.Token, lastKind tokenizer.TokenKind) bool {
	// Never space before certain tokens
	switch tok.Kind {
	case tokenizer.COMMA:
		return false
	case tokenizer.SEMICOLON:
		return false
	case tokenizer.RPAREN:
		return s.NoSpaceInsideParens == false
	case tokenizer.RBRACE:
		return s.NoSpaceInsideBraces == false
	case tokenizer.RBRACKET:
		return s.NoSpaceInsideBrackets == false
	case tokenizer.DOT:
		return false
	case tokenizer.QUESTION_DOT:
		return s.NoSpaceAroundSafeNav == false
	case tokenizer.QUESTION:
		// No space before ? in error propagation (x?)
		if s.NoSpaceBeforeQuestion {
			if lastKind == tokenizer.IDENT || lastKind == tokenizer.RPAREN {
				return false
			}
		}
	case tokenizer.LPAREN:
		// No space before ( after identifier (function calls)
		if s.NoSpaceBeforeCallParen && lastKind == tokenizer.IDENT {
			return false
		}
	case tokenizer.COLON:
		// No space before colon in key:value or labels
		return false
	}

	// Never space after certain tokens
	switch lastKind {
	case tokenizer.LPAREN:
		return s.NoSpaceInsideParens == false
	case tokenizer.LBRACE:
		return s.NoSpaceInsideBraces == false
	case tokenizer.LBRACKET:
		return s.NoSpaceInsideBrackets == false
	case tokenizer.DOT:
		return false
	case tokenizer.NOT:
		return false
	case tokenizer.QUESTION:
		return false
	case tokenizer.QUESTION_DOT:
		return s.NoSpaceAroundSafeNav == false
	case tokenizer.AMPERSAND:
		// No space after & in &Type or &value
		return false
	case tokenizer.STAR:
		// Context-dependent: *Type (no space) vs a * b (space)
		// For now, assume no space after * for pointer types
		// This is a simplification - proper handling needs AST context
		return false
	}

	return true
}

// NeedsSpaceAfter returns true if a space should be emitted after this token
func (s *SpacingRules) NeedsSpaceAfter(tok tokenizer.Token) bool {
	switch tok.Kind {
	// Commas
	case tokenizer.COMMA:
		return s.SpaceAfterComma

	// Semicolons
	case tokenizer.SEMICOLON:
		return true

	// Opening delimiters
	case tokenizer.LPAREN:
		return s.NoSpaceInsideParens == false
	case tokenizer.LBRACE:
		return s.NoSpaceInsideBraces == false
	case tokenizer.LBRACKET:
		return s.NoSpaceInsideBrackets == false

	// Closing delimiters
	case tokenizer.RPAREN, tokenizer.RBRACE, tokenizer.RBRACKET:
		return true

	// Dots and navigation
	case tokenizer.DOT:
		return false
	case tokenizer.QUESTION_DOT:
		return s.NoSpaceAroundSafeNav == false
	case tokenizer.QUESTION:
		return false

	// Colons
	case tokenizer.COLON:
		return s.SpaceAfterColon

	// Assignment operators
	case tokenizer.ASSIGN, tokenizer.DEFINE:
		return s.SpaceAroundAssignment

	// Binary operators
	case tokenizer.PLUS, tokenizer.MINUS, tokenizer.STAR, tokenizer.SLASH:
		return s.SpaceAroundBinaryOps
	case tokenizer.EQ, tokenizer.NE, tokenizer.LT, tokenizer.LE, tokenizer.GT, tokenizer.GE:
		return s.SpaceAroundBinaryOps
	case tokenizer.AND, tokenizer.OR:
		return s.SpaceAroundBinaryOps

	// Dingo operators
	case tokenizer.QUESTION_QUESTION:
		return s.SpaceAroundNullCoal

	// Unary operators
	case tokenizer.NOT:
		return false
	case tokenizer.AMPERSAND:
		return false

	// Keywords that should have space after
	case tokenizer.FUNC, tokenizer.TYPE, tokenizer.VAR, tokenizer.LET, tokenizer.CONST:
		return true
	case tokenizer.RETURN, tokenizer.IF, tokenizer.FOR, tokenizer.SWITCH, tokenizer.CASE:
		return true
	case tokenizer.MATCH, tokenizer.ENUM, tokenizer.GUARD:
		return true
	case tokenizer.PACKAGE, tokenizer.IMPORT:
		return true

	default:
		// Default: emit space
		return true
	}
}

// IsBinaryOperator returns true if the token is a binary operator
func (s *SpacingRules) IsBinaryOperator(kind tokenizer.TokenKind) bool {
	switch kind {
	case tokenizer.PLUS, tokenizer.MINUS, tokenizer.STAR, tokenizer.SLASH:
		return true
	case tokenizer.EQ, tokenizer.NE, tokenizer.LT, tokenizer.LE, tokenizer.GT, tokenizer.GE:
		return true
	case tokenizer.AND, tokenizer.OR:
		return true
	case tokenizer.QUESTION_QUESTION:
		return true
	default:
		return false
	}
}

// IsUnaryOperator returns true if the token is a unary operator
func (s *SpacingRules) IsUnaryOperator(kind tokenizer.TokenKind) bool {
	switch kind {
	case tokenizer.NOT, tokenizer.AMPERSAND, tokenizer.STAR, tokenizer.MINUS, tokenizer.PLUS:
		return true
	default:
		return false
	}
}

// NormalizeWhitespace applies general whitespace normalization to a token stream
// This is used by the main formatter to apply consistent spacing rules
func (s *SpacingRules) NormalizeWhitespace(tokens []tokenizer.Token) []tokenizer.Token {
	// This is a helper method that could be used to pre-process tokens
	// For now, spacing is handled by the Writer through NeedsSpaceBefore/After
	// This method is a placeholder for future enhancements
	return tokens
}

// ShouldPreserveBlankLine returns true if a blank line should be preserved
// Useful for maintaining visual separation between declarations
func (s *SpacingRules) ShouldPreserveBlankLine(prevKind, nextKind tokenizer.TokenKind) bool {
	// Preserve blank lines between top-level declarations
	topLevelDecls := map[tokenizer.TokenKind]bool{
		tokenizer.FUNC:    true,
		tokenizer.TYPE:    true,
		tokenizer.CONST:   true,
		tokenizer.VAR:     true,
		tokenizer.ENUM:    true,
		tokenizer.PACKAGE: true,
		tokenizer.IMPORT:  true,
	}

	return topLevelDecls[prevKind] && topLevelDecls[nextKind]
}

// GetIndentLevel calculates the appropriate indentation level for a token
// This is a helper for the Writer to determine indentation
func (s *SpacingRules) GetIndentLevel(tokenStack []tokenizer.TokenKind) int {
	level := 0
	for _, kind := range tokenStack {
		switch kind {
		case tokenizer.LBRACE, tokenizer.LPAREN, tokenizer.LBRACKET:
			level++
		}
	}
	return level
}
