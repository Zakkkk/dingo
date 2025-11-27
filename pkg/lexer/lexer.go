package lexer

import (
	"go/token"
	"unicode"
	"unicode/utf8"
)

// Lexer tokenizes Dingo source code
type Lexer struct {
	input        string
	position     int  // current position in input (points to current char)
	readPosition int  // current reading position in input (after current char)
	ch           rune // current char under examination
	line         int  // current line number (1-indexed)
	column       int  // current column number (1-indexed)
	fileSet      *token.FileSet
	file         *token.File
	peeked       *Token // buffered next token for lookahead
}

var keywords = map[string]TokenType{
	"let": LET,
	"var": VAR,
}

// New creates a new Lexer for the given input
func New(input string) *Lexer {
	fileSet := token.NewFileSet()
	file := fileSet.AddFile("", -1, len(input))

	l := &Lexer{
		input:   input,
		line:    1,
		column:  0,
		fileSet: fileSet,
		file:    file,
	}
	l.readChar() // initialize first character
	return l
}

// readChar advances the lexer position and reads the next character
func (l *Lexer) readChar() {
	if l.readPosition >= len(l.input) {
		l.ch = 0 // EOF
		l.position = l.readPosition
	} else {
		r, size := utf8.DecodeRuneInString(l.input[l.readPosition:])
		l.ch = r
		l.position = l.readPosition
		l.readPosition += size
	}
	l.column++
}

// peekChar looks ahead one character without advancing position
func (l *Lexer) peekChar() rune {
	if l.readPosition >= len(l.input) {
		return 0
	}
	r, _ := utf8.DecodeRuneInString(l.input[l.readPosition:])
	return r
}

// skipWhitespace skips spaces and tabs but NOT newlines
func (l *Lexer) skipWhitespace() {
	for l.ch == ' ' || l.ch == '\t' || l.ch == '\r' {
		l.readChar()
	}
}

// skipComment skips line comments (//) and block comments (/* */)
func (l *Lexer) skipComment() {
	if l.ch == '/' && l.peekChar() == '/' {
		// Line comment - skip until newline
		for l.ch != '\n' && l.ch != 0 {
			l.readChar()
		}
	} else if l.ch == '/' && l.peekChar() == '*' {
		// Block comment - skip until */
		l.readChar() // skip /
		l.readChar() // skip *
		for {
			if l.ch == 0 {
				break
			}
			if l.ch == '*' && l.peekChar() == '/' {
				l.readChar() // skip *
				l.readChar() // skip /
				break
			}
			if l.ch == '\n' {
				l.line++
				l.column = 0
			}
			l.readChar()
		}
	}
}

// readIdentifier reads an identifier or keyword
func (l *Lexer) readIdentifier() string {
	startPos := l.position
	for isLetter(l.ch) || isDigit(l.ch) {
		l.readChar()
	}
	return l.input[startPos:l.position]
}

// readNumber reads an integer or float literal
func (l *Lexer) readNumber() (string, TokenType) {
	startPos := l.position
	tokenType := INT

	for isDigit(l.ch) {
		l.readChar()
	}

	// Check for decimal point
	if l.ch == '.' && isDigit(l.peekChar()) {
		tokenType = FLOAT
		l.readChar() // consume '.'
		for isDigit(l.ch) {
			l.readChar()
		}
	}

	return l.input[startPos:l.position], tokenType
}

// readString reads a string literal (quoted or raw)
func (l *Lexer) readString(delimiter rune) string {
	startPos := l.position + 1 // skip opening quote

	for {
		l.readChar()
		if l.ch == delimiter || l.ch == 0 {
			break
		}
		if l.ch == '\\' && delimiter == '"' {
			// Skip escaped character in regular strings
			l.readChar()
		}
		if l.ch == '\n' {
			l.line++
			l.column = 0
		}
	}

	str := l.input[startPos:l.position]
	l.readChar() // skip closing quote
	return str
}

// makeToken creates a token with current position information
func (l *Lexer) makeToken(tokenType TokenType, literal string, startCol int) Token {
	pos := l.file.Pos(l.position)
	return Token{
		Type:    tokenType,
		Literal: literal,
		Pos:     pos,
		Line:    l.line,
		Column:  startCol,
	}
}

// NextToken returns the next token from the input
func (l *Lexer) NextToken() Token {
	// Return buffered token if present
	if l.peeked != nil {
		tok := *l.peeked
		l.peeked = nil
		return tok
	}

	return l.nextTokenImpl()
}

// nextTokenImpl implements the actual token scanning logic
func (l *Lexer) nextTokenImpl() Token {
	l.skipWhitespace()

	// Check for comments
	if l.ch == '/' && (l.peekChar() == '/' || l.peekChar() == '*') {
		l.skipComment()
		return l.nextTokenImpl() // Recursively get next non-comment token
	}

	// Save starting column for this token
	startCol := l.column

	var tok Token

	switch l.ch {
	case '=':
		tok = l.makeToken(ASSIGN, string(l.ch), startCol)
	case ':':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok = l.makeToken(DEFINE, string(ch)+string(l.ch), startCol)
		} else {
			tok = l.makeToken(COLON, string(l.ch), startCol)
		}
	case ',':
		tok = l.makeToken(COMMA, string(l.ch), startCol)
	case '(':
		tok = l.makeToken(LPAREN, string(l.ch), startCol)
	case ')':
		tok = l.makeToken(RPAREN, string(l.ch), startCol)
	case '[':
		tok = l.makeToken(LBRACKET, string(l.ch), startCol)
	case ']':
		tok = l.makeToken(RBRACKET, string(l.ch), startCol)
	case '<':
		tok = l.makeToken(LANGLE, string(l.ch), startCol)
	case '>':
		tok = l.makeToken(RANGLE, string(l.ch), startCol)
	case ';':
		tok = l.makeToken(SEMICOLON, string(l.ch), startCol)
	case '\n':
		tok = l.makeToken(NEWLINE, "\\n", startCol)
		l.line++
		l.column = 0
	case '"':
		literal := l.readString('"')
		tok = l.makeToken(STRING, literal, startCol)
		tok.QuoteType = '"'
		return tok
	case '`':
		literal := l.readString('`')
		tok = l.makeToken(STRING, literal, startCol)
		tok.QuoteType = '`'
		return tok
	case 0:
		tok = l.makeToken(EOF, "", startCol)
	default:
		if isLetter(l.ch) {
			literal := l.readIdentifier()
			tokenType := IDENT
			if kw, ok := keywords[literal]; ok {
				tokenType = kw
			}
			return l.makeToken(tokenType, literal, startCol)
		} else if isDigit(l.ch) {
			literal, tokenType := l.readNumber()
			return l.makeToken(tokenType, literal, startCol)
		} else {
			tok = l.makeToken(ILLEGAL, string(l.ch), startCol)
		}
	}

	l.readChar()
	return tok
}

// PeekToken looks ahead at the next token without consuming it
func (l *Lexer) PeekToken() Token {
	if l.peeked == nil {
		tok := l.nextTokenImpl()
		l.peeked = &tok
	}
	return *l.peeked
}

// Helper functions

func isLetter(ch rune) bool {
	return unicode.IsLetter(ch) || ch == '_'
}

func isDigit(ch rune) bool {
	return unicode.IsDigit(ch)
}
