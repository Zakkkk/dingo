package ast

import (
	"fmt"

	"github.com/MadAppGang/dingo/pkg/tokenizer"
)

// TupleKind represents the type of tuple syntax
type TupleKind int

const (
	TupleKindLiteral     TupleKind = iota // (a, b)
	TupleKindDestructure                  // let (x, y) = expr
	TupleKindTypeAlias                    // type Point = (int, int)
	TupleKindFuncReturn                   // func foo() (int, int)
)

// String returns string representation of TupleKind
func (k TupleKind) String() string {
	switch k {
	case TupleKindLiteral:
		return "literal"
	case TupleKindDestructure:
		return "destructure"
	case TupleKindTypeAlias:
		return "type_alias"
	case TupleKindFuncReturn:
		return "func_return"
	default:
		return fmt.Sprintf("TupleKind(%d)", k)
	}
}

// TupleContext represents where a tuple appears
type TupleContext int

const (
	TupleContextStatement  TupleContext = iota // standalone statement
	TupleContextAssignment                     // result := expr
	TupleContextReturn                         // return expr
	TupleContextArgument                       // foo(expr)
	TupleContextTypeDecl                       // type T = (...)
)

// String returns string representation of TupleContext
func (c TupleContext) String() string {
	switch c {
	case TupleContextStatement:
		return "statement"
	case TupleContextAssignment:
		return "assignment"
	case TupleContextReturn:
		return "return"
	case TupleContextArgument:
		return "argument"
	case TupleContextTypeDecl:
		return "type_decl"
	default:
		return fmt.Sprintf("TupleContext(%d)", c)
	}
}

// TupleElementInfo contains information about a single tuple element.
type TupleElementInfo struct {
	Name string // Type name for type tuples, empty for value tuples
}

// ElementInfo contains position and name information for a tuple element.
// Used for destructuring patterns and type alias elements.
type ElementInfo struct {
	Name  string // Element name or type name
	Start int    // Byte offset start (inclusive)
	End   int    // Byte offset end (exclusive)
}

// TupleLocation represents the location and context of a tuple
type TupleLocation struct {
	Kind         TupleKind
	Start        int              // byte offset of tuple start (inclusive) - the '('
	End          int              // byte offset of tuple end (exclusive) - after ')'
	Context      TupleContext     // where tuple appears
	Elements     int              // number of elements
	ElementsInfo []TupleElementInfo // element info (for func return types)
	HasWildcard  bool             // for destructuring: contains _ elements

	// Extended location info (populated by finder to avoid string scanning in transformer)
	// For destructuring: let (x, y) = expr
	//   KeywordStart = start of "let"
	//   AssignPos = position of "=" or ":="
	//   ExprEnd = end of full statement (newline or semicolon)
	//   PatternNames = ["x", "y"]
	// For type alias: type Name = (int, int)
	//   KeywordStart = start of "type"
	//   NameEnd = position after type name, before "="
	//   TypeContent = element type strings
	KeywordStart int      // Start of "let" or "type" keyword
	AssignPos    int      // Position of "=" or ":="
	ExprEnd      int      // End of full statement (for destructuring)
	NameEnd      int      // End of type name (for type alias)
	PatternNames []string // Variable names in destructuring pattern
	TypeContent  []string // Type strings for type alias
}

// FindTuples scans source for all tuple syntax locations
func FindTuples(src []byte) ([]TupleLocation, error) {
	tok := tokenizer.New(src)
	allTokens, err := tok.Tokenize()
	if err != nil {
		return nil, fmt.Errorf("tokenize: %w", err)
	}

	var locations []TupleLocation
	processedPositions := make(map[int]bool) // Track already-processed tuple starts
	tok.Reset()

	for {
		current := tok.Current()
		if current.Kind == tokenizer.EOF {
			break
		}

		// Skip comments and strings (never look for tuples inside)
		if current.Kind == tokenizer.COMMENT || current.Kind == tokenizer.STRING || current.Kind == tokenizer.CHAR {
			tok.Advance()
			continue
		}

		// Type alias: type Name = (...)
		if current.Kind == tokenizer.TYPE {
			loc, found := detectTypeAlias(tok, allTokens, src)
			if found {
				processedPositions[loc.Start] = true
				locations = append(locations, loc)
				continue
			}
		}

		// Function return type tuple: func foo() (Type1, Type2)
		if current.Kind == tokenizer.FUNC {
			result := detectFuncReturnTuples(tok, allTokens, src)
			// Mark nested tuple locations as processed and add to results
			for _, loc := range result.Locations {
				processedPositions[loc.Start] = true
				locations = append(locations, loc)
			}
			// CRITICAL: Mark outer return type paren positions as processed
			// These are Go's standard multiple return value syntax, NOT Dingo tuples
			// Without this, detectTupleLiteral will incorrectly detect them as tuple literals
			for _, skipPos := range result.SkipPositions {
				processedPositions[skipPos] = true
			}
			if len(result.Locations) > 0 || len(result.SkipPositions) > 0 {
				continue
			}
		}

		// Destructuring: let (...) = expr or let ((...)) = expr
		if current.Kind == tokenizer.LET {
			loc, found := detectDestructuring(tok, allTokens, src)
			if found {
				processedPositions[loc.Start] = true
				locations = append(locations, loc)
				continue
			}
		}

		// Tuple literal: (...) - but NOT function call or grouping
		// Skip if this LPAREN was already processed as part of type alias or destructuring
		if current.Kind == tokenizer.LPAREN {
			if !processedPositions[current.BytePos()] {
				loc, found := detectTupleLiteral(tok, allTokens, src)
				if found {
					processedPositions[loc.Start] = true
					locations = append(locations, loc)
					continue
				}
			}
		}

		tok.Advance()
	}

	return locations, nil
}

// detectTypeAlias checks if TYPE keyword starts a tuple type alias
// Pattern: type Name = (Type1, Type2, ...)
func detectTypeAlias(tok *tokenizer.Tokenizer, allTokens []tokenizer.Token, src []byte) (TupleLocation, bool) {
	savedPos := tok.SavePos()
	defer func() {
		tok.RestorePos(savedPos)
		tok.Advance() // Move past TYPE for outer loop
	}()

	// Record type keyword position
	typeStart := tok.Current().BytePos()
	tok.Advance() // skip TYPE

	// Expect IDENT (type name)
	if tok.Current().Kind != tokenizer.IDENT {
		return TupleLocation{}, false
	}
	nameEnd := tok.Current().ByteEnd()
	tok.Advance()

	// Expect = or ASSIGN
	if tok.Current().Kind != tokenizer.ASSIGN {
		return TupleLocation{}, false
	}
	tok.Advance()

	// Expect LPAREN
	if tok.Current().Kind != tokenizer.LPAREN {
		return TupleLocation{}, false
	}

	lparenPos := tok.Current().BytePos()
	tok.Advance()

	// Find matching RPAREN, count elements, collect type strings
	depth := 1
	elemCount := 0
	hasComma := false
	var typeContent []string
	var currentType []byte
	typeStart2 := tok.Current().BytePos() // Start of first type element

	for depth > 0 {
		current := tok.Current()
		if current.Kind == tokenizer.EOF {
			return TupleLocation{}, false
		}

		switch current.Kind {
		case tokenizer.LPAREN:
			depth++
			currentType = append(currentType, src[current.BytePos():current.ByteEnd()]...)
		case tokenizer.RPAREN:
			depth--
			if depth == 0 {
				// Count final element if we had any commas
				if hasComma {
					elemCount++
					// Collect final type (from last comma to before rparen)
					typeStr := string(src[typeStart2:current.BytePos()])
					typeContent = append(typeContent, trimWhitespace(typeStr))
				} else {
					// Single element in parens = not a tuple type
					return TupleLocation{}, false
				}

				return TupleLocation{
					Kind:         TupleKindTypeAlias,
					Start:        lparenPos,
					End:          current.ByteEnd(),
					Context:      TupleContextTypeDecl,
					Elements:     elemCount,
					KeywordStart: typeStart,
					NameEnd:      nameEnd,
					TypeContent:  typeContent,
				}, true
			}
			currentType = append(currentType, src[current.BytePos():current.ByteEnd()]...)
		case tokenizer.COMMA:
			if depth == 1 {
				// Collect type before this comma
				typeStr := string(src[typeStart2:current.BytePos()])
				typeContent = append(typeContent, trimWhitespace(typeStr))
				elemCount++
				hasComma = true
				// Next type starts after this comma
				typeStart2 = current.ByteEnd()
			} else {
				currentType = append(currentType, src[current.BytePos():current.ByteEnd()]...)
			}
		default:
			currentType = append(currentType, src[current.BytePos():current.ByteEnd()]...)
		}

		tok.Advance()
	}

	return TupleLocation{}, false
}

// trimWhitespace removes leading/trailing whitespace from a string.
func trimWhitespace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}

// detectDestructuring checks if LET keyword starts tuple destructuring
// Pattern: let (x, y) = expr or let ((a, b), c) = expr
func detectDestructuring(tok *tokenizer.Tokenizer, allTokens []tokenizer.Token, src []byte) (TupleLocation, bool) {
	savedPos := tok.SavePos()
	defer func() {
		tok.RestorePos(savedPos)
		tok.Advance() // Move past LET for outer loop
	}()

	// Record let keyword position
	letStart := tok.Current().BytePos()
	tok.Advance() // skip LET

	// Expect LPAREN
	if tok.Current().Kind != tokenizer.LPAREN {
		return TupleLocation{}, false
	}

	lparenPos := tok.Current().BytePos()
	tok.Advance()

	// Find matching RPAREN, count elements, check for wildcards, collect pattern names
	depth := 1
	elemCount := 0
	hasWildcard := false
	hasComma := false
	var patternNames []string

	for depth > 0 {
		current := tok.Current()
		if current.Kind == tokenizer.EOF {
			return TupleLocation{}, false
		}

		switch current.Kind {
		case tokenizer.LPAREN:
			depth++
		case tokenizer.RPAREN:
			depth--
			if depth == 0 {
				rparenEnd := current.ByteEnd()
				tok.Advance()

				// Must have found ASSIGN or DEFINE after this RPAREN
				assignTok := tok.Current()
				if assignTok.Kind != tokenizer.ASSIGN && assignTok.Kind != tokenizer.DEFINE {
					return TupleLocation{}, false
				}
				assignPos := assignTok.BytePos()
				tok.Advance()

				// Count final element if we had commas
				if hasComma {
					elemCount++
				} else {
					// Single element in parens after let = grouping, not destructure
					return TupleLocation{}, false
				}

				// Find end of expression (scan to newline, semicolon, or EOF)
				exprEnd := assignPos + 1
				for {
					exprTok := tok.Current()
					if exprTok.Kind == tokenizer.EOF || exprTok.Kind == tokenizer.NEWLINE || exprTok.Kind == tokenizer.SEMICOLON {
						exprEnd = exprTok.BytePos()
						break
					}
					exprEnd = exprTok.ByteEnd()
					tok.Advance()
				}

				return TupleLocation{
					Kind:         TupleKindDestructure,
					Start:        lparenPos,
					End:          rparenEnd,
					Context:      TupleContextAssignment,
					Elements:     elemCount,
					HasWildcard:  hasWildcard,
					KeywordStart: letStart,
					AssignPos:    assignPos,
					ExprEnd:      exprEnd,
					PatternNames: patternNames,
				}, true
			}
		case tokenizer.COMMA:
			if depth == 1 {
				elemCount++
				hasComma = true
			}
		case tokenizer.UNDERSCORE:
			if depth == 1 {
				hasWildcard = true
				patternNames = append(patternNames, "_")
			}
		case tokenizer.IDENT:
			if depth == 1 {
				// Extract identifier name from source
				name := string(src[current.BytePos():current.ByteEnd()])
				patternNames = append(patternNames, name)
			}
		}

		tok.Advance()
	}

	return TupleLocation{}, false
}

// isLambdaParameters checks if LPAREN at currentIdx starts lambda parameters
// by looking for => after the matching RPAREN.
// Pattern: (params) => body
func isLambdaParameters(allTokens []tokenizer.Token, currentIdx int) bool {
	if allTokens[currentIdx].Kind != tokenizer.LPAREN {
		return false
	}

	// Scan forward to find matching RPAREN
	depth := 0
	for i := currentIdx; i < len(allTokens); i++ {
		switch allTokens[i].Kind {
		case tokenizer.LPAREN:
			depth++
		case tokenizer.RPAREN:
			depth--
			if depth == 0 {
				// Found matching RPAREN - check next token
				if i+1 < len(allTokens) && allTokens[i+1].Kind == tokenizer.ARROW {
					return true // (params) =>
				}
				// Also check for optional return type: ) Type => or ): Type =>
				if i+1 < len(allTokens) {
					next := allTokens[i+1]
					if next.Kind == tokenizer.IDENT || next.Kind == tokenizer.COLON {
						// Could be return type annotation
						// Scan forward past optional type annotation to look for ARROW
						j := i + 1
						if next.Kind == tokenizer.COLON {
							j++ // skip colon
						}
						// Skip type tokens (IDENT, brackets, etc.)
						for j < len(allTokens) {
							if allTokens[j].Kind == tokenizer.ARROW {
								return true // ) Type => or ): Type =>
							}
							// Stop at tokens that indicate end of type annotation
							if allTokens[j].Kind != tokenizer.IDENT &&
								allTokens[j].Kind != tokenizer.LBRACKET &&
								allTokens[j].Kind != tokenizer.RBRACKET &&
								allTokens[j].Kind != tokenizer.STAR {
								break
							}
							j++
						}
					}
				}
				return false
			}
		}
	}

	return false
}

// detectTupleLiteral checks if LPAREN starts a tuple literal
// Pattern: (expr1, expr2, ...) - but NOT foo(a, b) or (a + b)
func detectTupleLiteral(tok *tokenizer.Tokenizer, allTokens []tokenizer.Token, src []byte) (TupleLocation, bool) {
	// Find current token index in allTokens
	currentIdx := -1
	currentPos := tok.Current().Pos
	for i, t := range allTokens {
		if t.Pos == currentPos {
			currentIdx = i
			break
		}
	}

	if currentIdx < 0 {
		tok.Advance()
		return TupleLocation{}, false
	}

	// Check if previous token indicates this is a function call:
	// - IDENT( = normal function call: foo(...)
	// - RBRACKET( = generic function call: foo[T](...) or Ok[User, string](...)
	if currentIdx > 0 {
		prev := allTokens[currentIdx-1]
		if prev.Kind == tokenizer.IDENT || prev.Kind == tokenizer.RBRACKET {
			tok.Advance()
			return TupleLocation{}, false
		}
	}

	// Check if this is a lambda parameter list by looking for => after closing paren
	// Pattern: (params) => body or (..., (params) => body, ...)
	// We need to scan forward to find the matching ) and check what follows
	if isLambdaParameters(allTokens, currentIdx) {
		tok.Advance()
		return TupleLocation{}, false
	}

	savedPos := tok.SavePos()
	lparenPos := tok.Current().BytePos()
	tok.Advance()

	// Scan to closing paren, counting commas at depth 0
	depth := 1
	elemCount := 0
	hasComma := false

	for depth > 0 {
		current := tok.Current()
		if current.Kind == tokenizer.EOF {
			tok.RestorePos(savedPos)
			tok.Advance()
			return TupleLocation{}, false
		}

		switch current.Kind {
		case tokenizer.LPAREN:
			depth++
		case tokenizer.RPAREN:
			depth--
			if depth == 0 {
				// Check if we had commas - single expr in parens is grouping
				if !hasComma {
					tok.RestorePos(savedPos)
					tok.Advance()
					return TupleLocation{}, false
				}

				// Count final element
				elemCount++

				// Determine context from surrounding tokens
				context := detectTupleContext(allTokens, currentIdx)

				loc := TupleLocation{
					Kind:     TupleKindLiteral,
					Start:    lparenPos,
					End:      current.ByteEnd(),
					Context:  context,
					Elements: elemCount,
				}

				tok.RestorePos(savedPos)
				tok.Advance()
				return loc, true
			}
		case tokenizer.COMMA:
			if depth == 1 {
				elemCount++
				hasComma = true
			}
		}

		tok.Advance()
	}

	tok.RestorePos(savedPos)
	tok.Advance()
	return TupleLocation{}, false
}

// FuncReturnTupleResult contains the result of func return tuple detection.
// It includes both the nested tuple locations to transform AND the outer
// return type paren positions that should be skipped by the literal detector.
type FuncReturnTupleResult struct {
	Locations     []TupleLocation // Nested tuple locations to transform
	SkipPositions []int           // Outer return type paren positions to skip
}

// detectFuncReturnTuples detects tuple types in function return signatures
// Pattern: func Name(...) (Type1, Type2, ...) or func Name(...) ((Type1, Type2), error)
// Returns nested tuple locations AND outer return type paren positions to skip.
//
// The outer return type parens like in "((float64, float64), error)" are Go's
// standard multiple return value syntax, NOT Dingo tuples. We must mark these
// positions as "skip" so the literal detector doesn't incorrectly detect them.
func detectFuncReturnTuples(tok *tokenizer.Tokenizer, allTokens []tokenizer.Token, src []byte) FuncReturnTupleResult {
	savedPos := tok.SavePos()
	defer func() {
		tok.RestorePos(savedPos)
		tok.Advance() // Move past FUNC for outer loop
	}()

	tok.Advance() // skip FUNC

	// Skip optional receiver: (r ReceiverType)
	if tok.Current().Kind == tokenizer.LPAREN {
		depth := 1
		tok.Advance()
		for depth > 0 {
			current := tok.Current()
			if current.Kind == tokenizer.EOF {
				return FuncReturnTupleResult{}
			}
			if current.Kind == tokenizer.LPAREN {
				depth++
			}
			if current.Kind == tokenizer.RPAREN {
				depth--
			}
			tok.Advance()
		}
	}

	// Expect function name (IDENT)
	if tok.Current().Kind != tokenizer.IDENT {
		return FuncReturnTupleResult{}
	}
	tok.Advance()

	// Skip parameter list: (params)
	if tok.Current().Kind != tokenizer.LPAREN {
		return FuncReturnTupleResult{}
	}
	depth := 1
	tok.Advance()
	for depth > 0 {
		current := tok.Current()
		if current.Kind == tokenizer.EOF {
			return FuncReturnTupleResult{}
		}
		if current.Kind == tokenizer.LPAREN {
			depth++
		}
		if current.Kind == tokenizer.RPAREN {
			depth--
		}
		tok.Advance()
	}

	// Now we're after the parameter list
	// Check for return type - could be:
	// 1. No return type (next is {)
	// 2. Single type (IDENT)
	// 3. Tuple return type (LPAREN...)
	if tok.Current().Kind != tokenizer.LPAREN {
		return FuncReturnTupleResult{} // No tuple return type
	}

	// This could be a tuple return type OR just multiple named returns
	// Tuple return: (Type1, Type2) - types only, no names
	// Named returns: (name1 Type1, name2 Type2) - has names
	// We want tuple types like ((float64, float64), error)

	var locations []TupleLocation
	var skipPositions []int

	// CRITICAL: Mark the outer paren position as a "skip" position.
	// This outer paren is Go's standard multiple return value syntax, NOT a Dingo tuple.
	// Without this, detectTupleLiteral will incorrectly detect it as a tuple literal.
	outerLparenPos := tok.Current().BytePos()
	skipPositions = append(skipPositions, outerLparenPos)
	tok.Advance()

	// Scan the return type list looking for tuple elements
	depth = 1
	elemCount := 0
	hasComma := false
	elemStart := tok.Current().BytePos()
	var elementsInfo []TupleElementInfo

	for depth > 0 {
		current := tok.Current()
		if current.Kind == tokenizer.EOF {
			return FuncReturnTupleResult{}
		}

		switch current.Kind {
		case tokenizer.LPAREN:
			// This could be a nested tuple type
			nestedLparenPos := current.BytePos()
			depth++
			nestedDepth := 1
			tok.Advance()
			nestedElemCount := 0
			nestedHasComma := false
			nestedElemStart := tok.Current().BytePos()
			var nestedTypeContent []string

			for nestedDepth > 0 {
				nested := tok.Current()
				if nested.Kind == tokenizer.EOF {
					return FuncReturnTupleResult{}
				}
				switch nested.Kind {
				case tokenizer.LPAREN:
					nestedDepth++
				case tokenizer.RPAREN:
					nestedDepth--
					if nestedDepth == 0 {
						depth-- // This closes one level of the outer depth too
						if nestedHasComma {
							nestedElemCount++
							// Capture final element type
							nestedTypeStr := trimWhitespace(string(src[nestedElemStart:nested.BytePos()]))
							nestedTypeContent = append(nestedTypeContent, nestedTypeStr)

							// Found a nested tuple type
							loc := TupleLocation{
								Kind:        TupleKindFuncReturn,
								Start:       nestedLparenPos,
								End:         nested.ByteEnd(),
								Context:     TupleContextTypeDecl,
								Elements:    nestedElemCount,
								TypeContent: nestedTypeContent,
							}
							// Populate ElementsInfo for func return tuple
							for _, typeStr := range nestedTypeContent {
								loc.ElementsInfo = append(loc.ElementsInfo, TupleElementInfo{Name: typeStr})
							}
							locations = append(locations, loc)
						}
					}
				case tokenizer.COMMA:
					if nestedDepth == 1 {
						nestedElemCount++
						nestedHasComma = true
						// Capture element type
						nestedTypeStr := trimWhitespace(string(src[nestedElemStart:nested.BytePos()]))
						nestedTypeContent = append(nestedTypeContent, nestedTypeStr)
						// Next element starts after comma
						nestedElemStart = nested.ByteEnd()
					}
				}
				tok.Advance()
			}
			// Continue to next token (already advanced in nested loop)
			continue

		case tokenizer.RPAREN:
			depth--
			if depth == 0 {
				if hasComma {
					elemCount++
					// Capture final element type
					elemTypeStr := trimWhitespace(string(src[elemStart:current.BytePos()]))
					elementsInfo = append(elementsInfo, TupleElementInfo{Name: elemTypeStr})

					// Check if this outer tuple is ALSO a tuple return type
					// (e.g., func foo() (Point, error) is NOT a tuple type - it's named returns)
					// BUT func foo() ((float64, float64), error) - outer level IS a tuple type
					// Actually, Go doesn't support returning bare tuples, only named/anonymous struct types
					// So we need to transform ((float64, float64), error) where inner is a tuple
					// The outer parens are just Go's multiple return values
				}
			}

		case tokenizer.COMMA:
			if depth == 1 {
				elemCount++
				hasComma = true
				// Capture element type
				elemTypeStr := trimWhitespace(string(src[elemStart:current.BytePos()]))
				elementsInfo = append(elementsInfo, TupleElementInfo{Name: elemTypeStr})
				elemStart = current.ByteEnd()
			}
		}

		tok.Advance()
	}

	return FuncReturnTupleResult{
		Locations:     locations,
		SkipPositions: skipPositions,
	}
}

// detectTupleContext determines where a tuple literal appears
func detectTupleContext(tokens []tokenizer.Token, lparenIdx int) TupleContext {
	// Scan backward to find context
	for i := lparenIdx - 1; i >= 0; i-- {
		tok := tokens[i]

		switch tok.Kind {
		case tokenizer.RETURN:
			return TupleContextReturn
		case tokenizer.DEFINE, tokenizer.ASSIGN:
			return TupleContextAssignment
		case tokenizer.LPAREN, tokenizer.COMMA:
			return TupleContextArgument
		case tokenizer.NEWLINE, tokenizer.SEMICOLON, tokenizer.LBRACE:
			// Hit statement boundary
			return TupleContextStatement
		}
	}

	return TupleContextStatement
}
