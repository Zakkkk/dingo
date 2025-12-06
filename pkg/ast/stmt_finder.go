package ast

import (
	"github.com/MadAppGang/dingo/pkg/tokenizer"
)

// StmtKind represents different statement types
type StmtKind int

const (
	StmtErrorPropAssign StmtKind = iota // x := foo()?
	StmtErrorPropLet                     // let x = foo()?
	StmtErrorPropReturn                  // return foo()?
)

// StmtLocation represents a Dingo statement that needs transformation
type StmtLocation struct {
	Kind      StmtKind
	Start     int    // Start byte position of statement
	End       int    // End byte position of statement
	VarName   string // Target variable name (for assign/let)
	ExprStart int    // Start of expression (before ?)
	ExprEnd   int    // End of expression (after ?)
}

// FindErrorPropStatements finds statements containing error propagation
func FindErrorPropStatements(src []byte) ([]StmtLocation, error) {
	tok := tokenizer.New(src)
	tokens, err := tok.Tokenize()
	if err != nil {
		return nil, err
	}

	var locations []StmtLocation

	for i := 0; i < len(tokens); i++ {
		t := tokens[i]

		// Look for patterns ending in ?
		// Pattern 1: IDENT := ... ?
		// Pattern 2: let IDENT = ... ?
		// Pattern 3: return ... ?

		if t.Kind == tokenizer.IDENT || t.Kind == tokenizer.UNDERSCORE {
			// Check for "ident :=" or "_ :=" pattern
			if i+1 < len(tokens) && tokens[i+1].Kind == tokenizer.DEFINE {
				loc := scanForQuestionMark(tokens, i, src)
				if loc != nil {
					loc.Kind = StmtErrorPropAssign
					loc.VarName = t.Lit
					locations = append(locations, *loc)
					// Skip past this statement
					i = findTokenAtByte(tokens, loc.End)
				}
			}
		} else if t.Kind == tokenizer.LET {
			// Check for "let ident =" pattern
			if i+2 < len(tokens) &&
				tokens[i+1].Kind == tokenizer.IDENT &&
				tokens[i+2].Kind == tokenizer.ASSIGN {
				loc := scanForQuestionMark(tokens, i, src)
				if loc != nil {
					loc.Kind = StmtErrorPropLet
					loc.VarName = tokens[i+1].Lit
					locations = append(locations, *loc)
					// Skip past this statement
					i = findTokenAtByte(tokens, loc.End)
				}
			}
		} else if t.Kind == tokenizer.RETURN {
			loc := scanForQuestionMark(tokens, i, src)
			if loc != nil {
				loc.Kind = StmtErrorPropReturn
				locations = append(locations, *loc)
				// Skip past this statement
				i = findTokenAtByte(tokens, loc.End)
			}
		}
	}

	return locations, nil
}

// scanForQuestionMark scans forward from startIdx looking for a statement ending with ?
func scanForQuestionMark(tokens []tokenizer.Token, startIdx int, src []byte) *StmtLocation {
	depth := 0
	stmtStart := tokens[startIdx].BytePos()

	for i := startIdx; i < len(tokens); i++ {
		t := tokens[i]

		switch t.Kind {
		case tokenizer.LPAREN, tokenizer.LBRACKET, tokenizer.LBRACE:
			depth++
		case tokenizer.RPAREN, tokenizer.RBRACKET, tokenizer.RBRACE:
			depth--
		case tokenizer.QUESTION:
			// Check it's standalone ? (not ?? or ?.)
			if i+1 < len(tokens) {
				next := tokens[i+1]
				if next.Kind == tokenizer.QUESTION || next.Kind == tokenizer.DOT {
					continue // It's ?? or ?.
				}
			}
			if depth == 0 {
				// Found statement with ? at end
				// Find expression start (after := or = or return)
				exprStart := findExprStart(tokens, startIdx, i)
				return &StmtLocation{
					Start:     stmtStart,
					End:       t.ByteEnd(),
					ExprStart: exprStart,
					ExprEnd:   t.ByteEnd(),
				}
			}
		case tokenizer.SEMICOLON, tokenizer.NEWLINE:
			// End of statement without finding ?
			if depth == 0 {
				return nil
			}
		}
	}
	return nil
}

// findExprStart finds where the expression starts (after := or = or return)
func findExprStart(tokens []tokenizer.Token, stmtStart, questionIdx int) int {
	for i := stmtStart; i < questionIdx; i++ {
		t := tokens[i]
		if t.Kind == tokenizer.DEFINE || t.Kind == tokenizer.ASSIGN || t.Kind == tokenizer.RETURN {
			// Expression starts after this token (skip whitespace)
			if i+1 < len(tokens) {
				return tokens[i+1].BytePos()
			}
		}
	}
	return tokens[stmtStart].BytePos()
}

// findTokenAtByte finds the index of the token at or after the given byte position
func findTokenAtByte(tokens []tokenizer.Token, bytePos int) int {
	for i, t := range tokens {
		if t.BytePos() >= bytePos {
			return i
		}
	}
	return len(tokens) - 1
}
