package tokenizer

import (
	"fmt"
	"go/token"
	"unicode"
)

// Tokenizer converts source into tokens
type Tokenizer struct {
	scanner *Scanner
	tokens  []Token
	pos     int // position in tokens slice (for parser)

	// State
	lastToken Token
}

// New creates a tokenizer for the given source
func New(src []byte) *Tokenizer {
	return &Tokenizer{
		scanner: NewScanner(src),
		tokens:  nil,
		pos:     0,
	}
}

// NewWithFileSet creates tokenizer with position tracking
func NewWithFileSet(src []byte, fset *token.FileSet, filename string) *Tokenizer {
	return &Tokenizer{
		scanner: NewScannerWithFileSet(src, fset, filename),
		tokens:  nil,
		pos:     0,
	}
}

// Tokenize converts entire source to tokens
func (t *Tokenizer) Tokenize() ([]Token, error) {
	t.tokens = nil

	for {
		tok, err := t.nextToken()
		if err != nil {
			return nil, err
		}
		t.tokens = append(t.tokens, tok)
		if tok.Kind == EOF {
			break
		}
	}

	return t.tokens, nil
}

// nextToken scans and returns the next token
func (t *Tokenizer) nextToken() (Token, error) {
	t.scanner.SkipWhitespace()

	if t.scanner.AtEOF() {
		pos := t.scanner.Pos()
		return Token{Kind: EOF, Pos: pos, End: pos}, nil
	}

	startPos := t.scanner.Pos()
	line, col := t.scanner.Position()
	r := t.scanner.Peek()

	// Comments
	if r == '/' {
		peek2 := t.scanner.PeekN(2)
		if peek2 == "//" {
			return t.scanLineComment(startPos, line, col)
		}
		if peek2 == "/*" {
			return t.scanBlockComment(startPos, line, col)
		}
	}

	// String literals
	if r == '"' {
		return t.scanString(startPos, line, col)
	}
	if r == '`' {
		return t.scanRawString(startPos, line, col)
	}
	if r == '\'' {
		return t.scanChar(startPos, line, col)
	}

	// Numbers
	if unicode.IsDigit(r) {
		return t.scanNumber(startPos, line, col)
	}

	// Check for decimal numbers starting with '.'
	if r == '.' {
		// Peek at next rune after '.'
		savedPos := t.scanner.pos
		t.scanner.Next() // Skip '.'
		nextRune := t.scanner.Peek()
		t.scanner.pos = savedPos // Restore position

		if unicode.IsDigit(nextRune) {
			return t.scanNumber(startPos, line, col)
		}
	}

	// Identifiers and keywords
	if unicode.IsLetter(r) {
		return t.scanIdentifier(startPos, line, col)
	}

	// Underscore - check if standalone or start of identifier
	if r == '_' {
		// Peek ahead to see if it's followed by letter/digit/underscore
		next := t.scanner.PeekN(2)
		if len(next) > 1 {
			nextChar := rune(next[1])
			// Check for letter, digit, OR another underscore
			if unicode.IsLetter(nextChar) || unicode.IsDigit(nextChar) || nextChar == '_' {
				return t.scanIdentifier(startPos, line, col)
			}
		}
		// Standalone underscore - handle as operator below
	}

	// Newline (explicit tracking)
	if r == '\n' {
		t.scanner.Next()
		return Token{Kind: NEWLINE, Pos: startPos, End: t.scanner.Pos(), Line: line, Column: col}, nil
	}

	// Operators and delimiters
	return t.scanOperator(startPos, line, col)
}

// scanLineComment scans a // comment
func (t *Tokenizer) scanLineComment(startPos token.Pos, line, col int) (Token, error) {
	t.scanner.SkipBytes(2) // Skip //
	start := t.scanner.pos

	for !t.scanner.AtEOF() && t.scanner.Peek() != '\n' {
		t.scanner.Next()
	}

	lit := string(t.scanner.src[start:t.scanner.pos])
	return Token{
		Kind:   COMMENT,
		Pos:    startPos,
		End:    t.scanner.Pos(),
		Lit:    "//" + lit,
		Line:   line,
		Column: col,
	}, nil
}

// scanBlockComment scans a /* */ comment
func (t *Tokenizer) scanBlockComment(startPos token.Pos, line, col int) (Token, error) {
	t.scanner.SkipBytes(2) // Skip /*
	start := t.scanner.pos

	for !t.scanner.AtEOF() {
		if t.scanner.PeekN(2) == "*/" {
			end := t.scanner.pos
			t.scanner.SkipBytes(2)
			lit := "/*" + string(t.scanner.src[start:end]) + "*/"
			return Token{
				Kind:   COMMENT,
				Pos:    startPos,
				End:    t.scanner.Pos(),
				Lit:    lit,
				Line:   line,
				Column: col,
			}, nil
		}
		t.scanner.Next()
	}

	return Token{}, fmt.Errorf("unterminated block comment at line %d", line)
}

// scanString scans a "..." string
func (t *Tokenizer) scanString(startPos token.Pos, line, col int) (Token, error) {
	t.scanner.Next() // Skip opening "
	start := t.scanner.pos
	escaped := false

	for !t.scanner.AtEOF() {
		r := t.scanner.Next()
		if escaped {
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if r == '"' {
			lit := `"` + string(t.scanner.src[start:t.scanner.pos-1]) + `"`
			return Token{
				Kind:   STRING,
				Pos:    startPos,
				End:    t.scanner.Pos(),
				Lit:    lit,
				Line:   line,
				Column: col,
			}, nil
		}
		if r == '\n' {
			return Token{}, fmt.Errorf("unterminated string at line %d", line)
		}
	}

	return Token{}, fmt.Errorf("unterminated string at line %d", line)
}

// scanRawString scans a `...` raw string
func (t *Tokenizer) scanRawString(startPos token.Pos, line, col int) (Token, error) {
	t.scanner.Next() // Skip opening `
	start := t.scanner.pos

	for !t.scanner.AtEOF() {
		r := t.scanner.Next()
		if r == '`' {
			lit := "`" + string(t.scanner.src[start:t.scanner.pos-1]) + "`"
			return Token{
				Kind:   STRING,
				Pos:    startPos,
				End:    t.scanner.Pos(),
				Lit:    lit,
				Line:   line,
				Column: col,
			}, nil
		}
	}

	return Token{}, fmt.Errorf("unterminated raw string at line %d", line)
}

// scanChar scans a 'x' character literal
func (t *Tokenizer) scanChar(startPos token.Pos, line, col int) (Token, error) {
	t.scanner.Next() // Skip opening '
	start := t.scanner.pos
	escaped := false

	for !t.scanner.AtEOF() {
		r := t.scanner.Next()
		if escaped {
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if r == '\'' {
			lit := "'" + string(t.scanner.src[start:t.scanner.pos-1]) + "'"
			return Token{
				Kind:   CHAR,
				Pos:    startPos,
				End:    t.scanner.Pos(),
				Lit:    lit,
				Line:   line,
				Column: col,
			}, nil
		}
		if r == '\n' {
			return Token{}, fmt.Errorf("unterminated char literal at line %d", line)
		}
	}

	return Token{}, fmt.Errorf("unterminated char literal at line %d", line)
}

// scanNumber scans an integer or float
func (t *Tokenizer) scanNumber(startPos token.Pos, line, col int) (Token, error) {
	start := t.scanner.pos
	hasDecimal := false

	// Check if starts with decimal point
	if t.scanner.Peek() == '.' {
		hasDecimal = true
		t.scanner.Next()
	}

	for !t.scanner.AtEOF() {
		r := t.scanner.Peek()
		if r == '.' && !hasDecimal {
			hasDecimal = true
			t.scanner.Next()
			continue
		}
		if !unicode.IsDigit(r) {
			break
		}
		t.scanner.Next()
	}

	lit := string(t.scanner.src[start:t.scanner.pos])
	kind := INT
	if hasDecimal {
		kind = FLOAT
	}

	return Token{
		Kind:   kind,
		Pos:    startPos,
		End:    t.scanner.Pos(),
		Lit:    lit,
		Line:   line,
		Column: col,
	}, nil
}

// scanIdentifier scans identifier or keyword
func (t *Tokenizer) scanIdentifier(startPos token.Pos, line, col int) (Token, error) {
	start := t.scanner.pos

	for !t.scanner.AtEOF() {
		r := t.scanner.Peek()
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
			break
		}
		t.scanner.Next()
	}

	lit := string(t.scanner.src[start:t.scanner.pos])
	kind := t.lookupKeyword(lit)

	return Token{
		Kind:   kind,
		Pos:    startPos,
		End:    t.scanner.Pos(),
		Lit:    lit,
		Line:   line,
		Column: col,
	}, nil
}

// lookupKeyword returns keyword kind or IDENT
func (t *Tokenizer) lookupKeyword(ident string) TokenKind {
	keywords := map[string]TokenKind{
		// Dingo keywords
		"match": MATCH,
		"where": WHERE,
		"enum":  ENUM,
		"guard": GUARD,
		"let":   LET,
		// Go keywords
		"if":          IF,
		"var":         VAR,
		"const":       CONST,
		"package":     PACKAGE,
		"import":      IMPORT,
		"func":        FUNC,
		"return":      RETURN,
		"type":        TYPE,
		"struct":      STRUCT,
		"interface":   INTERFACE,
		"map":         MAP,
		"chan":        CHAN,
		"for":         FOR,
		"range":       RANGE,
		"switch":      SWITCH,
		"case":        CASE,
		"default":     DEFAULT,
		"select":      SELECT,
		"break":       BREAK,
		"continue":    CONTINUE,
		"goto":        GOTO,
		"fallthrough": FALLTHROUGH,
		"defer":       DEFER,
		"go":          GO,
		"else":        ELSE,
		"nil":         NIL,
		"true":        TRUE,
		"false":       FALSE,
		"iota":        IOTA,
	}
	if kind, ok := keywords[ident]; ok {
		return kind
	}
	return IDENT
}

// scanOperator scans operators and delimiters
func (t *Tokenizer) scanOperator(startPos token.Pos, line, col int) (Token, error) {
	peek2 := t.scanner.PeekN(2)

	// Check for three-character operators first
	peek3 := t.scanner.PeekN(3)
	if peek3 == "..." {
		t.scanner.SkipBytes(3)
		return Token{Kind: ELLIPSIS, Pos: startPos, End: t.scanner.Pos(), Lit: "...", Line: line, Column: col}, nil
	}

	// Two-character operators (check Dingo-specific first)
	twoChar := map[string]TokenKind{
		":=": DEFINE,            // Go: short variable declaration
		"??": QUESTION_QUESTION, // Dingo: null coalescing
		"?.": QUESTION_DOT,      // Dingo: safe navigation
		"=>": ARROW,             // Dingo/existing: fat arrow (match arms)
		"->": THIN_ARROW,        // Dingo: Rust-style lambda arrow
		"<-": CHAN_ARROW,        // Go: channel receive/send
		"==": EQ,
		"!=": NE,
		"<=": LE,
		">=": GE,
		"&&": AND,
		"||": OR,
	}

	if kind, ok := twoChar[peek2]; ok {
		lit := peek2
		t.scanner.SkipBytes(2)
		return Token{Kind: kind, Pos: startPos, End: t.scanner.Pos(), Lit: lit, Line: line, Column: col}, nil
	}

	// Single-character operators
	r := t.scanner.Next()
	oneChar := map[rune]TokenKind{
		',': COMMA,
		'(': LPAREN,
		')': RPAREN,
		'{': LBRACE,
		'}': RBRACE,
		'[': LBRACKET,
		']': RBRACKET,
		':': COLON,
		';': SEMICOLON,
		'_': UNDERSCORE,
		'|': PIPE,
		'=': ASSIGN,
		'<': LT,
		'>': GT,
		'!': NOT,
		'+': PLUS,
		'-': MINUS,
		'*': STAR,
		'/': SLASH,
		'.': DOT,
		'?': QUESTION,
		'&': AMPERSAND,
	}

	if kind, ok := oneChar[r]; ok {
		return Token{Kind: kind, Pos: startPos, End: t.scanner.Pos(), Lit: string(r), Line: line, Column: col}, nil
	}

	return Token{Kind: ILLEGAL, Pos: startPos, End: t.scanner.Pos(), Lit: string(r), Line: line, Column: col}, nil
}

// --- Parser helper methods ---

// Reset resets token position for re-parsing
func (t *Tokenizer) Reset() {
	t.pos = 0
}

// SavePos returns the current position for later restoration
func (t *Tokenizer) SavePos() int {
	return t.pos
}

// RestorePos restores a previously saved position
func (t *Tokenizer) RestorePos(pos int) {
	t.pos = pos
}

// Current returns current token
func (t *Tokenizer) Current() Token {
	if t.pos >= len(t.tokens) {
		return Token{Kind: EOF}
	}
	return t.tokens[t.pos]
}

// Advance moves to next token
func (t *Tokenizer) Advance() Token {
	tok := t.Current()
	if t.pos < len(t.tokens) {
		t.pos++
	}
	return tok
}

// PeekToken returns next token without consuming
func (t *Tokenizer) PeekToken() Token {
	if t.pos+1 >= len(t.tokens) {
		return Token{Kind: EOF}
	}
	return t.tokens[t.pos+1]
}

// Expect consumes token of expected kind or returns error
func (t *Tokenizer) Expect(kind TokenKind) (Token, error) {
	tok := t.Current()
	if tok.Kind != kind {
		return Token{}, fmt.Errorf("expected %s, got %s at line %d", kind, tok.Kind, tok.Line)
	}
	t.Advance()
	return tok, nil
}

// Match returns true and advances if current token matches
func (t *Tokenizer) Match(kinds ...TokenKind) bool {
	for _, kind := range kinds {
		if t.Current().Kind == kind {
			t.Advance()
			return true
		}
	}
	return false
}

// NextToken returns the next token (for parser interface compatibility)
func (t *Tokenizer) NextToken() Token {
	if t.tokens == nil {
		// Lazily tokenize if not done yet
		t.Tokenize()
		t.pos = 0
	}

	tok := t.Current()
	t.Advance()
	return tok
}
