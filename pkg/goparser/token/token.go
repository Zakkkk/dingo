// Package token extends go/token with Dingo-specific tokens.
// We re-export go/token types and add Dingo tokens.
package token

import (
	gotoken "go/token"
)

// Re-export go/token types
type (
	Token    = gotoken.Token
	Pos      = gotoken.Pos
	Position = gotoken.Position
	File     = gotoken.File
	FileSet  = gotoken.FileSet
)

// Re-export go/token constants
const (
	// Special tokens
	ILLEGAL = gotoken.ILLEGAL
	EOF     = gotoken.EOF
	COMMENT = gotoken.COMMENT

	// Literals
	IDENT  = gotoken.IDENT
	INT    = gotoken.INT
	FLOAT  = gotoken.FLOAT
	IMAG   = gotoken.IMAG
	CHAR   = gotoken.CHAR
	STRING = gotoken.STRING

	// Operators
	ADD = gotoken.ADD
	SUB = gotoken.SUB
	MUL = gotoken.MUL
	QUO = gotoken.QUO
	REM = gotoken.REM

	AND     = gotoken.AND
	OR      = gotoken.OR
	XOR     = gotoken.XOR
	SHL     = gotoken.SHL
	SHR     = gotoken.SHR
	AND_NOT = gotoken.AND_NOT

	ADD_ASSIGN = gotoken.ADD_ASSIGN
	SUB_ASSIGN = gotoken.SUB_ASSIGN
	MUL_ASSIGN = gotoken.MUL_ASSIGN
	QUO_ASSIGN = gotoken.QUO_ASSIGN
	REM_ASSIGN = gotoken.REM_ASSIGN

	AND_ASSIGN     = gotoken.AND_ASSIGN
	OR_ASSIGN      = gotoken.OR_ASSIGN
	XOR_ASSIGN     = gotoken.XOR_ASSIGN
	SHL_ASSIGN     = gotoken.SHL_ASSIGN
	SHR_ASSIGN     = gotoken.SHR_ASSIGN
	AND_NOT_ASSIGN = gotoken.AND_NOT_ASSIGN

	LAND  = gotoken.LAND
	LOR   = gotoken.LOR
	ARROW = gotoken.ARROW
	INC   = gotoken.INC
	DEC   = gotoken.DEC

	EQL    = gotoken.EQL
	LSS    = gotoken.LSS
	GTR    = gotoken.GTR
	ASSIGN = gotoken.ASSIGN
	NOT    = gotoken.NOT

	NEQ      = gotoken.NEQ
	LEQ      = gotoken.LEQ
	GEQ      = gotoken.GEQ
	DEFINE   = gotoken.DEFINE
	ELLIPSIS = gotoken.ELLIPSIS

	LPAREN    = gotoken.LPAREN
	LBRACK    = gotoken.LBRACK
	LBRACE    = gotoken.LBRACE
	COMMA     = gotoken.COMMA
	PERIOD    = gotoken.PERIOD
	RPAREN    = gotoken.RPAREN
	RBRACK    = gotoken.RBRACK
	RBRACE    = gotoken.RBRACE
	SEMICOLON = gotoken.SEMICOLON
	COLON     = gotoken.COLON

	// Go keywords
	BREAK       = gotoken.BREAK
	CASE        = gotoken.CASE
	CHAN        = gotoken.CHAN
	CONST       = gotoken.CONST
	CONTINUE    = gotoken.CONTINUE
	DEFAULT     = gotoken.DEFAULT
	DEFER       = gotoken.DEFER
	ELSE        = gotoken.ELSE
	FALLTHROUGH = gotoken.FALLTHROUGH
	FOR         = gotoken.FOR
	FUNC        = gotoken.FUNC
	GO          = gotoken.GO
	GOTO        = gotoken.GOTO
	IF          = gotoken.IF
	IMPORT      = gotoken.IMPORT
	INTERFACE   = gotoken.INTERFACE
	MAP         = gotoken.MAP
	PACKAGE     = gotoken.PACKAGE
	RANGE       = gotoken.RANGE
	RETURN      = gotoken.RETURN
	SELECT      = gotoken.SELECT
	STRUCT      = gotoken.STRUCT
	SWITCH      = gotoken.SWITCH
	TYPE        = gotoken.TYPE
	VAR         = gotoken.VAR
	TILDE       = gotoken.TILDE
)

// Dingo-specific tokens start after Go's token range
// Go tokens are 0-89, we start at 100 to be safe
const (
	// Dingo operators
	QUESTION          Token = 100 + iota // ?  (error propagation postfix)
	QUESTION_QUESTION                    // ?? (null coalescing)
	QUESTION_DOT                         // ?. (safe navigation)
	FAT_ARROW                            // => (lambda/match arm)
	THIN_ARROW                           // -> (alternative lambda)

	// Dingo keywords
	LET   // let
	MATCH // match
	ENUM  // enum
	GUARD // guard
	WHERE // where (match guard)
)

// dingoTokens maps Dingo token values to their string representations
var dingoTokens = map[Token]string{
	QUESTION:          "?",
	QUESTION_QUESTION: "??",
	QUESTION_DOT:      "?.",
	FAT_ARROW:         "=>",
	THIN_ARROW:        "->",
	LET:               "let",
	MATCH:             "match",
	ENUM:              "enum",
	GUARD:             "guard",
	WHERE:             "where",
}

// String returns the string representation of a token
func String(tok Token) string {
	if s, ok := dingoTokens[tok]; ok {
		return s
	}
	return tok.String()
}

// IsDingoToken returns true if the token is a Dingo-specific token
func IsDingoToken(tok Token) bool {
	return tok >= QUESTION
}

// IsDingoKeyword returns true if the token is a Dingo keyword
func IsDingoKeyword(tok Token) bool {
	return tok >= LET && tok <= WHERE
}

// IsDingoOperator returns true if the token is a Dingo operator
func IsDingoOperator(tok Token) bool {
	return tok >= QUESTION && tok <= THIN_ARROW
}

// LookupDingo looks up a Dingo keyword by name
// Returns IDENT if not a Dingo keyword
func LookupDingo(ident string) Token {
	switch ident {
	case "let":
		return LET
	case "match":
		return MATCH
	case "enum":
		return ENUM
	case "guard":
		return GUARD
	case "where":
		return WHERE
	default:
		return IDENT
	}
}

// Lookup looks up both Go and Dingo keywords
func Lookup(ident string) Token {
	// First check Dingo keywords
	if tok := LookupDingo(ident); tok != IDENT {
		return tok
	}
	// Fall back to Go keywords
	return gotoken.Lookup(ident)
}

// NewFileSet creates a new file set
func NewFileSet() *FileSet {
	return gotoken.NewFileSet()
}
