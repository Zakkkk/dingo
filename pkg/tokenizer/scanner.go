package tokenizer

import (
	"go/token"
	"unicode"
	"unicode/utf8"
)

// Scanner reads characters from source and provides character-level primitives
type Scanner struct {
	src       []byte // source code
	pos       int    // current position in src
	line      int    // current line (1-based)
	column    int    // current column (1-based)
	lineStart int    // position of current line start

	// Position tracking for source maps
	fset *token.FileSet
	file *token.File
}

// NewScanner creates a scanner for the given source
func NewScanner(src []byte) *Scanner {
	return &Scanner{
		src:       src,
		pos:       0,
		line:      1,
		column:    1,
		lineStart: 0,
	}
}

// NewScannerWithFileSet creates scanner with go/token integration
func NewScannerWithFileSet(src []byte, fset *token.FileSet, filename string) *Scanner {
	file := fset.AddFile(filename, -1, len(src))
	return &Scanner{
		src:       src,
		pos:       0,
		line:      1,
		column:    1,
		lineStart: 0,
		fset:      fset,
		file:      file,
	}
}

// Peek returns the next character without consuming it
func (s *Scanner) Peek() rune {
	if s.pos >= len(s.src) {
		return 0 // EOF
	}
	r, _ := utf8.DecodeRune(s.src[s.pos:])
	return r
}

// PeekN looks ahead N characters
func (s *Scanner) PeekN(n int) string {
	end := s.pos + n
	if end > len(s.src) {
		end = len(s.src)
	}
	return string(s.src[s.pos:end])
}

// Next consumes and returns the next character
func (s *Scanner) Next() rune {
	if s.pos >= len(s.src) {
		return 0
	}
	r, size := utf8.DecodeRune(s.src[s.pos:])
	s.pos += size

	if r == '\n' {
		s.line++
		s.column = 1
		s.lineStart = s.pos
	} else {
		s.column++
	}

	return r
}

// SkipBytes advances N bytes (for known byte counts like "//" or "/*")
// IMPORTANT: Also updates column counter for accurate position tracking
func (s *Scanner) SkipBytes(n int) {
	end := s.pos + n
	if end > len(s.src) {
		end = len(s.src)
	}
	// Update column for each byte skipped (handle newlines)
	for i := s.pos; i < end; i++ {
		if s.src[i] == '\n' {
			s.line++
			s.column = 1
			s.lineStart = i + 1
		} else {
			s.column++
		}
	}
	s.pos = end
}

// SkipRunes advances N Unicode characters
func (s *Scanner) SkipRunes(n int) {
	for i := 0; i < n && s.pos < len(s.src); i++ {
		s.Next()
	}
}

// Skip is an alias for SkipBytes (common case)
func (s *Scanner) Skip(n int) {
	s.SkipBytes(n)
}

// Pos returns current position as token.Pos
func (s *Scanner) Pos() token.Pos {
	if s.file != nil {
		return s.file.Pos(s.pos)
	}
	return token.Pos(s.pos + 1) // 1-based
}

// Position returns detailed position info
func (s *Scanner) Position() (line, column int) {
	return s.line, s.column
}

// AtEOF returns true if at end of source
func (s *Scanner) AtEOF() bool {
	return s.pos >= len(s.src)
}

// SkipWhitespace consumes whitespace except newlines
func (s *Scanner) SkipWhitespace() {
	for !s.AtEOF() {
		r := s.Peek()
		if r == ' ' || r == '\t' || r == '\r' {
			s.Next()
		} else {
			break
		}
	}
}

// SkipWhitespaceAndNewlines consumes all whitespace including newlines
func (s *Scanner) SkipWhitespaceAndNewlines() {
	for !s.AtEOF() {
		r := s.Peek()
		if unicode.IsSpace(r) {
			s.Next()
		} else {
			break
		}
	}
}
