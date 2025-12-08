package ast

import (
	"github.com/MadAppGang/dingo/pkg/tokenizer"
)

// StmtKind represents different statement types
type StmtKind int

const (
	StmtErrorPropAssign StmtKind = iota // x := foo()?
	StmtErrorPropLet                    // let x = foo()?
	StmtErrorPropReturn                 // return foo()?
)

// ErrorPropKind represents the type of error transformation
type ErrorPropKind int

const (
	ErrorPropBasic   ErrorPropKind = iota // expr?
	ErrorPropContext                      // expr ? "message"
	ErrorPropLambda                       // expr ? |err| transform OR expr ? (err) => transform
)

// StmtLocation represents a Dingo statement that needs transformation
type StmtLocation struct {
	Kind      StmtKind
	Start     int    // Start byte position of statement
	End       int    // End byte position of statement
	VarName   string // Target variable name (for assign/let)
	ExprStart int    // Start of expression (before ?)
	ExprEnd   int    // End of expression (after ?)

	// Advanced error propagation
	ErrorKind       ErrorPropKind // Type of error transformation
	ErrorContext    string        // Context message (for ErrorPropContext)
	LambdaParam     string        // Lambda parameter name (for ErrorPropLambda)
	LambdaBodyStart int           // Start byte position of lambda body
	LambdaBodyEnd   int           // End byte position of lambda body
}

// findMatchingColonForErrorProp looks ahead from questionIdx to find a matching : at the same depth.
// Returns the index of the colon token, or -1 if no matching colon is found.
// This is used to distinguish ternary operators (? ... :) from error propagation (?).
func findMatchingColonForErrorProp(tokens []tokenizer.Token, questionIdx int) int {
	depth := 0
	ternaryDepth := 0 // Track nested ternaries
	for i := questionIdx + 1; i < len(tokens); i++ {
		tok := tokens[i]
		switch tok.Kind {
		case tokenizer.LPAREN, tokenizer.LBRACKET, tokenizer.LBRACE:
			depth++
		case tokenizer.RPAREN, tokenizer.RBRACKET, tokenizer.RBRACE:
			depth--
			if depth < 0 {
				// Hit closing delimiter of containing expression
				return -1
			}
		case tokenizer.QUESTION:
			// Check if this is a ternary ? (not ?? or ?.)
			if i+1 < len(tokens) {
				next := tokens[i+1]
				if next.Kind != tokenizer.QUESTION && next.Kind != tokenizer.DOT {
					// Nested ternary
					ternaryDepth++
				}
			}
		case tokenizer.COLON:
			if depth == 0 {
				if ternaryDepth > 0 {
					// This colon closes a nested ternary
					ternaryDepth--
				} else {
					// Found matching colon for our ? at depth 0
					return i
				}
			}
		case tokenizer.NEWLINE, tokenizer.COMMENT:
			// Skip newlines and comments - ternaries can be multi-line
			continue
		case tokenizer.SEMICOLON, tokenizer.EOF:
			// Statement boundary - no matching colon
			return -1
		}
	}
	return -1
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
				loc := scanForQuestionMark(tokens, i)
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
				loc := scanForQuestionMark(tokens, i)
				if loc != nil {
					loc.Kind = StmtErrorPropLet
					loc.VarName = tokens[i+1].Lit
					locations = append(locations, *loc)
					// Skip past this statement
					i = findTokenAtByte(tokens, loc.End)
				}
			}
		} else if t.Kind == tokenizer.RETURN {
			loc := scanForQuestionMark(tokens, i)
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
// Also detects advanced patterns: ? "message", ? |err| expr, ? (err) => expr, ? err => expr
func scanForQuestionMark(tokens []tokenizer.Token, startIdx int) *StmtLocation {
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
				if next.Kind == tokenizer.QUESTION {
					continue // It's ??
				}
				if next.Kind == tokenizer.DOT {
					continue // It's ?.
				}
			}
			if depth == 0 {
				// CRITICAL: Check if this is a ternary operator (has matching :)
				// Ternaries should NOT be treated as error propagation
				colonIdx := findMatchingColonForErrorProp(tokens, i)
				if colonIdx > i {
					// This is a ternary expression, not error propagation
					continue
				}

				// Found ? at statement level - check for advanced patterns
				exprStart := findExprStart(tokens, startIdx, i)
				loc := &StmtLocation{
					Start:     stmtStart,
					ExprStart: exprStart,
					ExprEnd:   t.ByteEnd(), // End of expression (excluding ? and transform)
					ErrorKind: ErrorPropBasic,
				}

				// Look at what follows the ?
				if i+1 < len(tokens) {
					next := tokens[i+1]

					// Pattern: ? "message" (string context)
					if next.Kind == tokenizer.STRING {
						loc.ErrorKind = ErrorPropContext
						// Strip quotes from string literal
						msg := next.Lit
						if len(msg) >= 2 {
							if (msg[0] == '"' && msg[len(msg)-1] == '"') ||
								(msg[0] == '`' && msg[len(msg)-1] == '`') {
								msg = msg[1 : len(msg)-1]
							}
						}
						loc.ErrorContext = msg
						loc.End = next.ByteEnd()
						return loc
					}

					// Pattern: ? |param| body (Rust-style lambda)
					if next.Kind == tokenizer.PIPE {
						if parsed := parseRustLambdaTransform(tokens, i+1); parsed != nil {
							loc.ErrorKind = ErrorPropLambda
							loc.LambdaParam = parsed.param
							loc.LambdaBodyStart = parsed.bodyStart
							loc.LambdaBodyEnd = parsed.bodyEnd
							loc.End = parsed.bodyEnd
							return loc
						}
					}

					// Pattern: ? (param) => body (TypeScript-style with parens)
					if next.Kind == tokenizer.LPAREN {
						if parsed := parseTSLambdaTransform(tokens, i+1); parsed != nil {
							loc.ErrorKind = ErrorPropLambda
							loc.LambdaParam = parsed.param
							loc.LambdaBodyStart = parsed.bodyStart
							loc.LambdaBodyEnd = parsed.bodyEnd
							loc.End = parsed.bodyEnd
							return loc
						}
					}

					// Pattern: ? param => body (TypeScript-style single param, no parens)
					if next.Kind == tokenizer.IDENT {
						if i+2 < len(tokens) && tokens[i+2].Kind == tokenizer.ARROW {
							if parsed := parseTSSingleParamLambdaTransform(tokens, i+1); parsed != nil {
								loc.ErrorKind = ErrorPropLambda
								loc.LambdaParam = parsed.param
								loc.LambdaBodyStart = parsed.bodyStart
								loc.LambdaBodyEnd = parsed.bodyEnd
								loc.End = parsed.bodyEnd
								return loc
							}
						}
					}
				}

				// Basic ? (no transform)
				loc.End = t.ByteEnd()
				return loc
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

// lambdaParsed holds the result of parsing a lambda transform
type lambdaParsed struct {
	param     string
	bodyStart int // Start byte position of lambda body
	bodyEnd   int // End byte position of lambda body (from last token's ByteEnd)
}

// parseRustLambdaTransform parses: |param| body
// startIdx points to the first PIPE token
func parseRustLambdaTransform(tokens []tokenizer.Token, startIdx int) *lambdaParsed {
	if startIdx >= len(tokens) || tokens[startIdx].Kind != tokenizer.PIPE {
		return nil
	}

	// Expect: PIPE IDENT PIPE
	if startIdx+2 >= len(tokens) {
		return nil
	}
	if tokens[startIdx+1].Kind != tokenizer.IDENT {
		return nil
	}
	if tokens[startIdx+2].Kind != tokenizer.PIPE {
		return nil
	}

	param := tokens[startIdx+1].Lit
	bodyStartIdx := startIdx + 3 // After closing |

	// Parse body until end of statement
	return parseLambdaBody(tokens, bodyStartIdx, param)
}

// parseTSLambdaTransform parses: (param) => body
// startIdx points to LPAREN
func parseTSLambdaTransform(tokens []tokenizer.Token, startIdx int) *lambdaParsed {
	if startIdx >= len(tokens) || tokens[startIdx].Kind != tokenizer.LPAREN {
		return nil
	}

	// Expect: LPAREN IDENT RPAREN ARROW
	if startIdx+3 >= len(tokens) {
		return nil
	}
	if tokens[startIdx+1].Kind != tokenizer.IDENT {
		return nil
	}
	if tokens[startIdx+2].Kind != tokenizer.RPAREN {
		return nil
	}
	if tokens[startIdx+3].Kind != tokenizer.ARROW {
		return nil
	}

	param := tokens[startIdx+1].Lit
	bodyStartIdx := startIdx + 4 // After =>

	return parseLambdaBody(tokens, bodyStartIdx, param)
}

// parseTSSingleParamLambdaTransform parses: param => body
// startIdx points to IDENT
func parseTSSingleParamLambdaTransform(tokens []tokenizer.Token, startIdx int) *lambdaParsed {
	if startIdx >= len(tokens) || tokens[startIdx].Kind != tokenizer.IDENT {
		return nil
	}

	// Expect: IDENT ARROW
	if startIdx+1 >= len(tokens) || tokens[startIdx+1].Kind != tokenizer.ARROW {
		return nil
	}

	param := tokens[startIdx].Lit
	bodyStartIdx := startIdx + 2 // After =>

	return parseLambdaBody(tokens, bodyStartIdx, param)
}

// parseLambdaBody parses the lambda body expression until end of statement
// Returns token positions only - NO string extraction or manipulation
func parseLambdaBody(tokens []tokenizer.Token, bodyStartIdx int, param string) *lambdaParsed {
	if bodyStartIdx >= len(tokens) {
		return nil
	}

	depth := 0
	lastContentIdx := bodyStartIdx // Track last non-whitespace token

	for i := bodyStartIdx; i < len(tokens); i++ {
		t := tokens[i]

		switch t.Kind {
		case tokenizer.LPAREN, tokenizer.LBRACKET, tokenizer.LBRACE:
			depth++
			lastContentIdx = i
		case tokenizer.RPAREN:
			if depth == 0 {
				// End of expression - return positions from last content token
				if lastContentIdx < bodyStartIdx {
					return nil
				}
				return &lambdaParsed{
					param:     param,
					bodyStart: tokens[bodyStartIdx].BytePos(),
					bodyEnd:   tokens[lastContentIdx].ByteEnd(),
				}
			}
			depth--
			lastContentIdx = i
		case tokenizer.RBRACKET, tokenizer.RBRACE:
			if depth == 0 {
				if lastContentIdx < bodyStartIdx {
					return nil
				}
				return &lambdaParsed{
					param:     param,
					bodyStart: tokens[bodyStartIdx].BytePos(),
					bodyEnd:   tokens[lastContentIdx].ByteEnd(),
				}
			}
			depth--
			lastContentIdx = i
		case tokenizer.SEMICOLON:
			if depth == 0 {
				if lastContentIdx < bodyStartIdx {
					return nil
				}
				return &lambdaParsed{
					param:     param,
					bodyStart: tokens[bodyStartIdx].BytePos(),
					bodyEnd:   tokens[lastContentIdx].ByteEnd(),
				}
			}
		case tokenizer.NEWLINE:
			// Newline at depth 0 ALWAYS ends the expression in Dingo
			// Lambda bodies are single-expression, not multi-line
			if depth == 0 {
				if lastContentIdx < bodyStartIdx {
					return nil
				}
				return &lambdaParsed{
					param:     param,
					bodyStart: tokens[bodyStartIdx].BytePos(),
					bodyEnd:   tokens[lastContentIdx].ByteEnd(),
				}
			}
		default:
			// Track last content token (not newline/semicolon/whitespace)
			lastContentIdx = i
		}
	}

	// Reached end of tokens
	if lastContentIdx < bodyStartIdx {
		return nil
	}
	return &lambdaParsed{
		param:     param,
		bodyStart: tokens[bodyStartIdx].BytePos(),
		bodyEnd:   tokens[lastContentIdx].ByteEnd(),
	}
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
