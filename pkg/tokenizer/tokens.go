package tokenizer

import (
	"fmt"
	"go/token"
)

// TokenKind represents the type of token
type TokenKind int

const (
	// Special tokens
	ILLEGAL TokenKind = iota
	EOF
	COMMENT // // or /* */

	// Identifiers and literals
	IDENT  // match, Ok, Err, x, y
	INT    // 123
	FLOAT  // 1.23
	STRING // "hello" or `raw`
	CHAR   // 'a'

	// Operators and delimiters
	ARROW      // =>
	COMMA      // ,
	LPAREN     // (
	RPAREN     // )
	LBRACE     // {
	RBRACE     // }
	LBRACKET   // [
	RBRACKET   // ]
	COLON      // :
	SEMICOLON  // ;
	UNDERSCORE // _
	PIPE       // |  (for future or-patterns)

	// Keywords
	MATCH // match
	IF    // if (for guards)
	WHERE // where (alternate guard syntax)
	VAR   // var
	LET   // let
	CONST // const

	// Compound operators (needed for expression parsing)
	EQ     // ==
	NE     // !=
	LT     // <
	LE     // <=
	GT     // >
	GE     // >=
	AND    // &&
	OR     // ||
	NOT    // !
	ASSIGN // =
	DEFINE // :=

	// For expression parsing
	PLUS     // +
	MINUS    // -
	STAR     // *
	SLASH    // /
	DOT      // .
	QUESTION // ?

	// Dingo-specific operators
	QUESTION_QUESTION // ?? (null coalescing)
	QUESTION_DOT      // ?. (safe navigation)
	FAT_ARROW         // => (already exists as ARROW, keeping for clarity)
	THIN_ARROW        // -> (Rust-style lambda)

	// Dingo keywords
	ENUM  // enum (sum types)
	GUARD // guard (pattern matching guards)

	// Boundaries (help with expression extraction)
	NEWLINE // explicit newline tracking for position accuracy
)

var tokenKindStrings = map[TokenKind]string{
	ILLEGAL:    "ILLEGAL",
	EOF:        "EOF",
	COMMENT:    "COMMENT",
	IDENT:      "IDENT",
	INT:        "INT",
	FLOAT:      "FLOAT",
	STRING:     "STRING",
	CHAR:       "CHAR",
	ARROW:      "=>",
	COMMA:      ",",
	LPAREN:     "(",
	RPAREN:     ")",
	LBRACE:     "{",
	RBRACE:     "}",
	LBRACKET:   "[",
	RBRACKET:   "]",
	COLON:      ":",
	SEMICOLON:  ";",
	UNDERSCORE: "_",
	PIPE:       "|",
	MATCH:      "match",
	IF:         "if",
	WHERE:      "where",
	VAR:        "var",
	LET:        "let",
	CONST:      "const",
	EQ:         "==",
	NE:         "!=",
	LT:         "<",
	LE:         "<=",
	GT:         ">",
	GE:         ">=",
	AND:        "&&",
	OR:         "||",
	NOT:        "!",
	ASSIGN:     "=",
	DEFINE:     ":=",
	PLUS:       "+",
	MINUS:      "-",
	STAR:       "*",
	SLASH:      "/",
	DOT:               ".",
	QUESTION:          "?",
	QUESTION_QUESTION: "??",
	QUESTION_DOT:      "?.",
	FAT_ARROW:         "=>",
	THIN_ARROW:        "->",
	ENUM:              "enum",
	GUARD:             "guard",
	NEWLINE:           "NEWLINE",
}

// String returns the string representation of a token kind
func (k TokenKind) String() string {
	if s, ok := tokenKindStrings[k]; ok {
		return s
	}
	return fmt.Sprintf("TokenKind(%d)", k)
}

// Token represents a single token
type Token struct {
	Kind   TokenKind
	Pos    token.Pos // Absolute position in file
	End    token.Pos // End position
	Lit    string    // Literal value (for IDENT, INT, STRING, COMMENT)
	Line   int       // Line number (1-based)
	Column int       // Column number (1-based)
}

// String returns a human-readable representation
func (t Token) String() string {
	if t.Lit != "" {
		return fmt.Sprintf("%s(%q)", t.Kind, t.Lit)
	}
	return t.Kind.String()
}

// IsKeyword returns true if token is a Dingo keyword
func (t Token) IsKeyword() bool {
	return t.Kind == MATCH || t.Kind == IF || t.Kind == WHERE || t.Kind == ENUM || t.Kind == GUARD
}
