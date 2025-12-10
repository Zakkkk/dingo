package ast

import (
	"github.com/MadAppGang/dingo/pkg/tokenizer"
)

// GuardLetLocation represents a guard let statement in source
type GuardLetLocation struct {
	// Statement boundaries
	Start int // byte offset of 'guard' keyword
	End   int // byte offset after closing brace

	// Binding information
	IsTuple  bool     // true if (a, b) pattern
	VarNames []string // ["user"] or ["name", "age"]

	// Expression (RHS of =)
	ExprStart int    // start of expression
	ExprEnd   int    // end of expression
	ExprText  string // raw expression text for type inference

	// Else block
	HasBinding  bool   // true if |param| present
	BindingName string // "err", "e", etc.
	ElseStart   int    // start of else block content (after opening brace)
	ElseEnd     int    // end of else block content (before closing brace)

	// Source location for LSP
	Line   int // 1-indexed line number
	Column int // 1-indexed column number
}

// FindGuardLetStatements finds guard let statements in source
func FindGuardLetStatements(src []byte) ([]GuardLetLocation, error) {
	tok := tokenizer.New(src)
	tokens, err := tok.Tokenize()
	if err != nil {
		return nil, err
	}

	var locations []GuardLetLocation

	for i := 0; i < len(tokens); i++ {
		t := tokens[i]

		// Look for: guard let ...
		if t.Kind == tokenizer.GUARD {
			if i+1 < len(tokens) && tokens[i+1].Kind == tokenizer.LET {
				loc := parseGuardLetStatement(tokens, i, src)
				if loc != nil {
					locations = append(locations, *loc)
					// Skip past this statement
					i = findTokenAtByte(tokens, loc.End)
				}
			}
		}
	}

	return locations, nil
}

// parseGuardLetStatement parses a complete guard let statement
// Pattern: guard let PATTERN = EXPR else [|PARAM|] { BLOCK }
func parseGuardLetStatement(tokens []tokenizer.Token, startIdx int, src []byte) *GuardLetLocation {
	guardToken := tokens[startIdx]
	stmtStart := guardToken.BytePos()

	loc := &GuardLetLocation{
		Start:  stmtStart,
		Line:   guardToken.Line,
		Column: guardToken.Column,
	}

	// Skip 'guard let'
	i := startIdx + 2
	if i >= len(tokens) {
		return nil
	}

	// Parse binding pattern (single or tuple)
	varNames, isTuple, nextIdx := parseBindingPattern(tokens, i)
	if varNames == nil {
		return nil
	}

	loc.VarNames = varNames
	loc.IsTuple = isTuple
	i = nextIdx

	// Expect '='
	if i >= len(tokens) || tokens[i].Kind != tokenizer.ASSIGN {
		return nil
	}
	i++ // skip '='

	// Parse expression until 'else'
	exprStart := tokens[i].BytePos()
	exprEnd, elseIdx := findElseKeyword(tokens, i)
	if elseIdx == -1 {
		return nil
	}

	loc.ExprStart = exprStart
	loc.ExprEnd = exprEnd
	loc.ExprText = string(src[exprStart:exprEnd])

	// Skip 'else'
	i = elseIdx + 1
	if i >= len(tokens) {
		return nil
	}

	// Check for pipe binding: |param|
	if tokens[i].Kind == tokenizer.PIPE {
		if parsed := parsePipeBinding(tokens, i); parsed != nil {
			loc.HasBinding = true
			loc.BindingName = parsed.param
			i = parsed.nextIdx
		}
	}

	// Expect opening brace
	if i >= len(tokens) || tokens[i].Kind != tokenizer.LBRACE {
		return nil
	}

	// Parse else block
	blockStart, blockEnd, endIdx := parseElseBlock(tokens, i)
	if blockStart == -1 {
		return nil
	}

	loc.ElseStart = blockStart
	loc.ElseEnd = blockEnd
	loc.End = tokens[endIdx].ByteEnd()

	return loc
}

// parseBindingPattern parses single or tuple binding
// Returns: varNames, isTuple, nextTokenIdx
func parseBindingPattern(tokens []tokenizer.Token, startIdx int) ([]string, bool, int) {
	if startIdx >= len(tokens) {
		return nil, false, -1
	}

	// Check for tuple: (a, b, c)
	if tokens[startIdx].Kind == tokenizer.LPAREN {
		return parseTupleBinding(tokens, startIdx)
	}

	// Single identifier
	if tokens[startIdx].Kind == tokenizer.IDENT {
		return []string{tokens[startIdx].Lit}, false, startIdx + 1
	}

	return nil, false, -1
}

// parseTupleBinding parses tuple pattern: (a, b, c)
func parseTupleBinding(tokens []tokenizer.Token, startIdx int) ([]string, bool, int) {
	if tokens[startIdx].Kind != tokenizer.LPAREN {
		return nil, false, -1
	}

	var names []string
	i := startIdx + 1 // skip LPAREN

	for i < len(tokens) {
		t := tokens[i]

		if t.Kind == tokenizer.RPAREN {
			// End of tuple
			if len(names) == 0 {
				return nil, false, -1 // Empty tuple
			}
			return names, true, i + 1 // skip RPAREN
		}

		if t.Kind == tokenizer.IDENT {
			names = append(names, t.Lit)
			i++
			continue
		}

		if t.Kind == tokenizer.COMMA {
			i++
			continue
		}

		// Unexpected token
		return nil, false, -1
	}

	// No closing paren found
	return nil, false, -1
}

// findElseKeyword finds the 'else' keyword in a guard let statement
// Returns: exprEnd (byte position), elseIdx (token index)
func findElseKeyword(tokens []tokenizer.Token, startIdx int) (int, int) {
	depth := 0

	for i := startIdx; i < len(tokens); i++ {
		t := tokens[i]

		switch t.Kind {
		case tokenizer.LPAREN, tokenizer.LBRACKET, tokenizer.LBRACE:
			depth++
		case tokenizer.RPAREN, tokenizer.RBRACKET, tokenizer.RBRACE:
			depth--
		case tokenizer.ELSE:
			if depth == 0 {
				// Found else at statement level
				// Expression ends at previous token
				if i > 0 {
					return tokens[i-1].ByteEnd(), i
				}
				return t.BytePos(), i
			}
		}
	}

	return -1, -1
}

// pipeBindingParsed holds result of parsing |param|
type pipeBindingParsed struct {
	param   string
	nextIdx int // next token index after closing |
}

// parsePipeBinding parses: |param|
func parsePipeBinding(tokens []tokenizer.Token, startIdx int) *pipeBindingParsed {
	if startIdx+2 >= len(tokens) {
		return nil
	}

	// Expect: PIPE IDENT PIPE
	if tokens[startIdx].Kind != tokenizer.PIPE {
		return nil
	}
	if tokens[startIdx+1].Kind != tokenizer.IDENT {
		return nil
	}
	if tokens[startIdx+2].Kind != tokenizer.PIPE {
		return nil
	}

	return &pipeBindingParsed{
		param:   tokens[startIdx+1].Lit,
		nextIdx: startIdx + 3,
	}
}

// parseElseBlock parses the else block with brace depth tracking
// Returns: blockStart (byte), blockEnd (byte), closingBraceIdx (token index)
func parseElseBlock(tokens []tokenizer.Token, lbraceIdx int) (int, int, int) {
	if lbraceIdx >= len(tokens) || tokens[lbraceIdx].Kind != tokenizer.LBRACE {
		return -1, -1, -1
	}

	depth := 1
	blockStart := tokens[lbraceIdx].ByteEnd() // Start after opening brace
	i := lbraceIdx + 1

	for i < len(tokens) {
		t := tokens[i]

		if t.Kind == tokenizer.LBRACE {
			depth++
		} else if t.Kind == tokenizer.RBRACE {
			depth--
			if depth == 0 {
				// Found matching closing brace
				blockEnd := t.BytePos() // End before closing brace
				return blockStart, blockEnd, i
			}
		}

		i++
	}

	// No matching closing brace
	return -1, -1, -1
}
