package lexer

import (
	"go/token"
)

// TokenType represents the type of a lexical token
type TokenType int

const (
	// Special tokens
	ILLEGAL TokenType = iota
	EOF
	NEWLINE

	// Identifiers and literals
	IDENT  // x, foo, bar
	INT    // 123, 456
	FLOAT  // 1.23, 4.56
	STRING // "hello", `world`

	// Keywords
	LET
	VAR

	// Operators and delimiters
	ASSIGN   // =
	DEFINE   // :=
	COLON    // :
	COMMA    // ,
	LPAREN   // (
	RPAREN   // )
	LBRACKET // [
	RBRACKET // ]
	LANGLE   // <
	RANGLE   // >
	SEMICOLON // ;
)

var tokenTypeNames = map[TokenType]string{
	ILLEGAL:   "ILLEGAL",
	EOF:       "EOF",
	NEWLINE:   "NEWLINE",
	IDENT:     "IDENT",
	INT:       "INT",
	FLOAT:     "FLOAT",
	STRING:    "STRING",
	LET:       "LET",
	VAR:       "VAR",
	ASSIGN:    "ASSIGN",
	DEFINE:    "DEFINE",
	COLON:     "COLON",
	COMMA:     "COMMA",
	LPAREN:    "LPAREN",
	RPAREN:    "RPAREN",
	LBRACKET:  "LBRACKET",
	RBRACKET:  "RBRACKET",
	LANGLE:    "LANGLE",
	RANGLE:    "RANGLE",
	SEMICOLON: "SEMICOLON",
}

// String returns the string representation of the token type
func (tt TokenType) String() string {
	if name, ok := tokenTypeNames[tt]; ok {
		return name
	}
	return "UNKNOWN"
}

// Token represents a lexical token with position information
type Token struct {
	Type      TokenType
	Literal   string
	Pos       token.Pos // Position in the file (compatible with go/token)
	Line      int       // Line number (1-indexed)
	Column    int       // Column number (1-indexed)
	QuoteType byte      // Quote character for STRING tokens ('"' or '`')
}

// String returns a string representation of the token
func (t Token) String() string {
	if t.Literal != "" {
		return t.Type.String() + "(" + t.Literal + ")"
	}
	return t.Type.String()
}
