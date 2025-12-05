// Package scanner extends go/scanner with Dingo-specific token scanning.
// It wraps go/scanner and intercepts token scanning to handle Dingo operators.
package scanner

import (
	"go/scanner"
	gotoken "go/token"

	"github.com/MadAppGang/dingo/pkg/goparser/token"
)

// ErrorHandler is called for each error encountered during scanning
type ErrorHandler func(pos gotoken.Position, msg string)

// Scanner extends go/scanner.Scanner with Dingo token support
type Scanner struct {
	base    scanner.Scanner
	file    *gotoken.File
	src     []byte
	offset  int // current reading offset
	rdOffset int // reading offset (position after current character)

	// Current token state
	pos gotoken.Pos
	tok token.Token
	lit string

	// Lookahead for multi-char Dingo operators
	peeked    bool
	peekPos   gotoken.Pos
	peekTok   token.Token
	peekLit   string
}

// Init initializes the scanner with source code
func (s *Scanner) Init(file *gotoken.File, src []byte, err ErrorHandler, mode scanner.Mode) {
	s.file = file
	s.src = src
	s.offset = 0
	s.rdOffset = 0
	s.peeked = false

	// Initialize the underlying Go scanner
	var errHandler scanner.ErrorHandler
	if err != nil {
		errHandler = func(pos gotoken.Position, msg string) {
			err(pos, msg)
		}
	}
	s.base.Init(file, src, errHandler, mode)
}

// Scan scans the next token, handling Dingo-specific tokens
func (s *Scanner) Scan() (pos gotoken.Pos, tok token.Token, lit string) {
	// If we have a peeked token, return it
	if s.peeked {
		s.peeked = false
		return s.peekPos, s.peekTok, s.peekLit
	}

	// Scan from base scanner
	basePos, baseTok, baseLit := s.base.Scan()

	// Check for Dingo operators that start with ?
	if baseTok == gotoken.ILLEGAL && baseLit == "?" {
		return s.scanQuestion(basePos)
	}

	// Check for => (fat arrow) - need to look ahead after =
	if baseTok == gotoken.ASSIGN {
		nextPos, nextTok, nextLit := s.base.Scan()
		if nextTok == gotoken.GTR {
			// It's => (fat arrow)
			return basePos, token.FAT_ARROW, "=>"
		}
		// Not a fat arrow, save the peeked token
		s.peeked = true
		s.peekPos = nextPos
		s.peekTok = token.Token(nextTok)
		s.peekLit = nextLit
		return basePos, token.ASSIGN, baseLit
	}

	// Check for -> (thin arrow) - need to look ahead after -
	if baseTok == gotoken.SUB {
		nextPos, nextTok, nextLit := s.base.Scan()
		if nextTok == gotoken.GTR {
			// It's -> (thin arrow)
			return basePos, token.THIN_ARROW, "->"
		}
		// Not a thin arrow, save the peeked token
		s.peeked = true
		s.peekPos = nextPos
		s.peekTok = token.Token(nextTok)
		s.peekLit = nextLit
		return basePos, token.SUB, baseLit
	}

	// Check for Dingo keywords
	if baseTok == gotoken.IDENT {
		if dingoTok := token.LookupDingo(baseLit); dingoTok != token.IDENT {
			return basePos, dingoTok, baseLit
		}
	}

	return basePos, token.Token(baseTok), baseLit
}

// scanQuestion handles ? and its multi-character variants
func (s *Scanner) scanQuestion(pos gotoken.Pos) (gotoken.Pos, token.Token, string) {
	// Look ahead to check for ?? or ?.
	nextPos, nextTok, nextLit := s.base.Scan()

	if nextTok == gotoken.ILLEGAL && nextLit == "?" {
		// It's ?? (null coalescing)
		return pos, token.QUESTION_QUESTION, "??"
	}

	if nextTok == gotoken.PERIOD {
		// It's ?. (safe navigation)
		return pos, token.QUESTION_DOT, "?."
	}

	// Just a single ?, save the peeked token
	s.peeked = true
	s.peekPos = nextPos
	s.peekTok = token.Token(nextTok)
	s.peekLit = nextLit

	return pos, token.QUESTION, "?"
}

// ErrorList is a list of scanner errors
type ErrorList = scanner.ErrorList

// Error represents a scanner error
type Error = scanner.Error

// Mode controls scanner behavior
type Mode = scanner.Mode

// Scanner modes
const (
	ScanComments Mode = scanner.ScanComments
)
