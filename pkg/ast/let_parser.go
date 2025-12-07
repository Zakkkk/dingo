package ast

import (
	"fmt"
	"go/token"
)

// LetParser parses Dingo let declarations into AST nodes.
type LetParser struct {
	src    []byte
	pos    int
	fset   *token.FileSet
	file   *token.File
	offset int // Base offset in source file
}

// NewLetParser creates a new let parser for the given source.
func NewLetParser(src []byte, offset int) *LetParser {
	fset := token.NewFileSet()
	file := fset.AddFile("", -1, len(src))
	return &LetParser{
		src:    src,
		pos:    0,
		fset:   fset,
		file:   file,
		offset: offset,
	}
}

// ParseLetDecl parses a let declaration starting at current position.
// Returns the LetDecl and the end position in the source.
func (p *LetParser) ParseLetDecl() (*LetDecl, int, error) {
	startPos := p.pos

	// Expect "let" keyword
	if !p.matchKeyword("let") {
		return nil, startPos, fmt.Errorf("expected 'let' keyword")
	}
	letPos := token.Pos(p.offset + startPos + 1)

	p.skipWhitespace()

	// Parse names (can be multiple: let a, b = ...)
	// or tuple destructure: let (a, b) = ...
	isTupleDestructure := false
	if p.peek() == '(' {
		isTupleDestructure = true
		p.advance() // Skip '('
		p.skipWhitespace()
	}

	names, err := p.parseNames()
	if err != nil {
		return nil, p.pos, fmt.Errorf("expected variable name(s): %w", err)
	}

	if isTupleDestructure {
		p.skipWhitespace()
		if p.peek() != ')' {
			return nil, p.pos, fmt.Errorf("expected ')' after tuple destructure names")
		}
		p.advance() // Skip ')'
	}

	p.skipWhitespace()

	// Parse optional type annotation
	var typeAnnot string
	// If next char is NOT '=' and NOT newline/semi, assume it is a type
	if p.peek() != '=' && p.peek() != '\n' && p.peek() != ';' {
		// Handle optional legacy colon
		if p.peek() == ':' {
			p.advance() // Skip ':'
			p.skipWhitespace()
		}

		typeAnnot, err = p.parseType()
		if err != nil {
			return nil, p.pos, fmt.Errorf("invalid type annotation: %w", err)
		}
		p.skipWhitespace()
	}

	// Parse optional initialization (= expr)
	var value string
	hasInit := false
	if p.peek() == '=' {
		p.advance() // Skip '='
		p.skipWhitespace()
		value, err = p.parseValue()
		if err != nil {
			return nil, p.pos, fmt.Errorf("invalid initialization value: %w", err)
		}
		hasInit = true
	}

	return &LetDecl{
		LetPos:    letPos,
		Names:     names,
		TypeAnnot: typeAnnot,
		Value:     value,
		HasInit:   hasInit,
	}, p.pos, nil
}

// parseNames parses comma-separated variable names
func (p *LetParser) parseNames() ([]string, error) {
	var names []string

	for {
		name, err := p.parseIdentString()
		if err != nil {
			return nil, err
		}
		names = append(names, name)

		p.skipWhitespace()
		if p.peek() == ',' {
			p.advance() // Skip ','
			p.skipWhitespace()
			continue
		}
		break
	}

	return names, nil
}

// parseIdentString parses an identifier and returns it as a string
func (p *LetParser) parseIdentString() (string, error) {
	start := p.pos
	if !p.isAlpha(p.peek()) && p.peek() != '_' {
		return "", fmt.Errorf("expected identifier, got %q", string(p.peek()))
	}

	for p.pos < len(p.src) && (p.isAlphaNumeric(p.peek()) || p.peek() == '_') {
		p.advance()
	}

	return string(p.src[start:p.pos]), nil
}

// parseType parses a type annotation
// Returns the type string (e.g. "int", "[]string") without leading colon
func (p *LetParser) parseType() (string, error) {
	// Parse type expression (identifier, generic types, etc.)
	// For now, simple implementation: read until '=' or newline
	typeStart := p.pos
	depth := 0 // Track < > nesting for generics
	for p.pos < len(p.src) {
		ch := p.peek()
		if ch == '<' {
			depth++
		} else if ch == '>' {
			depth--
		} else if depth == 0 && (ch == '=' || ch == '\n' || ch == ';' || p.isWhitespace(ch)) {
			break
		}
		p.advance()
	}

	return string(p.src[typeStart:p.pos]), nil
}

// parseValue parses the initialization value expression
// Returns the expression as a string (unparsed)
func (p *LetParser) parseValue() (string, error) {
	start := p.pos
	depth := 0 // Track parentheses/braces nesting

	for p.pos < len(p.src) {
		ch := p.peek()

		// Track nesting
		if ch == '(' || ch == '{' || ch == '[' {
			depth++
		} else if ch == ')' || ch == '}' || ch == ']' {
			if depth == 0 {
				// End of expression (closing paren/brace from outer context)
				break
			}
			depth--
		} else if depth == 0 && (ch == '\n' || ch == ';') {
			// End of expression
			break
		}

		p.advance()
	}

	if p.pos == start {
		return "", fmt.Errorf("expected value expression")
	}

	return string(p.src[start:p.pos]), nil
}

// matchKeyword checks if the current position matches a keyword
func (p *LetParser) matchKeyword(keyword string) bool {
	if p.pos+len(keyword) > len(p.src) {
		return false
	}
	// Check if keyword matches
	for i := 0; i < len(keyword); i++ {
		if p.src[p.pos+i] != keyword[i] {
			return false
		}
	}
	// Check that keyword is not part of a larger identifier
	if p.pos+len(keyword) < len(p.src) {
		nextCh := p.src[p.pos+len(keyword)]
		if p.isAlphaNumeric(nextCh) || nextCh == '_' {
			return false
		}
	}
	p.pos += len(keyword)
	return true
}

// skipWhitespace skips whitespace characters
func (p *LetParser) skipWhitespace() {
	for p.pos < len(p.src) && p.isWhitespace(p.peek()) {
		p.advance()
	}
}

// peek returns the current character without advancing
func (p *LetParser) peek() byte {
	if p.pos >= len(p.src) {
		return 0
	}
	return p.src[p.pos]
}

// advance moves to the next character
func (p *LetParser) advance() {
	p.pos++
}

// isWhitespace checks if a character is whitespace
func (p *LetParser) isWhitespace(ch byte) bool {
	return ch == ' ' || ch == '\t' || ch == '\r' || ch == '\n'
}

// isAlpha checks if a character is alphabetic
func (p *LetParser) isAlpha(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
}

// isAlphaNumeric checks if a character is alphanumeric
func (p *LetParser) isAlphaNumeric(ch byte) bool {
	return p.isAlpha(ch) || (ch >= '0' && ch <= '9')
}
