// Package parser provides Pratt parser implementation for expression parsing
package parser

import (
	"fmt"
	"go/token"

	"github.com/MadAppGang/dingo/pkg/ast"
	"github.com/MadAppGang/dingo/pkg/tokenizer"
)

// Precedence levels for operators (higher = tighter binding)
const (
	PrecLowest     = iota
	PrecTernary    // ? : (ternary)
	PrecNullCoal   // ?? (null coalescing)
	PrecLogicalOr  // ||
	PrecLogicalAnd // &&
	PrecEquality   // == !=
	PrecComparison // < > <= >=
	PrecAdditive   // + -
	PrecMultiply   // * / %
	PrecUnary      // ! - +
	PrecPostfix    // ? ?. (error prop, safe nav)
	PrecCall       // () [] .
)

// operatorPrecedence maps token types to their precedence levels
var operatorPrecedence = map[tokenizer.TokenKind]int{
	// Dingo operators
	tokenizer.QUESTION:          PrecTernary,  // ? : (ternary) - also handles x? (error prop) via disambiguation
	tokenizer.QUESTION_QUESTION: PrecNullCoal, // a ?? b (null coalescing)
	tokenizer.QUESTION_DOT:      PrecPostfix,  // x?.field (safe navigation)

	// Standard Go operators
	tokenizer.DOT:    PrecCall, // x.y (selector/method call)
	tokenizer.LPAREN: PrecCall, // x() (function call)

	// Binary operators
	tokenizer.OR:    PrecLogicalOr,  // ||
	tokenizer.AND:   PrecLogicalAnd, // &&
	tokenizer.EQ:    PrecEquality,   // ==
	tokenizer.NE:    PrecEquality,   // !=
	tokenizer.LT:    PrecComparison, // <
	tokenizer.GT:    PrecComparison, // >
	tokenizer.LE:    PrecComparison, // <=
	tokenizer.GE:    PrecComparison, // >=
	tokenizer.PLUS:  PrecAdditive,   // +
	tokenizer.MINUS: PrecAdditive,   // -
	tokenizer.STAR:  PrecMultiply,   // *
	tokenizer.SLASH: PrecMultiply,   // /
}

// PrattParser implements a Pratt parser for expressions
type PrattParser struct {
	tokenizer *tokenizer.Tokenizer
	errors    []ParseError

	// Current and peek tokens
	curToken  tokenizer.Token
	peekToken tokenizer.Token

	// Prefix and infix parse functions
	prefixParseFns map[tokenizer.TokenKind]prefixParseFn
	infixParseFns  map[tokenizer.TokenKind]infixParseFn
}

// ParseError represents a parser error
type ParseError struct {
	Pos     token.Pos
	Line    int
	Column  int
	Message string
}

func (e ParseError) Error() string {
	return fmt.Sprintf("parse error at %d:%d: %s", e.Line, e.Column, e.Message)
}

// Parse function types
type (
	prefixParseFn func() ast.Expr
	infixParseFn  func(ast.Expr) ast.Expr
)

// NewPrattParser creates a new Pratt parser
func NewPrattParser(t *tokenizer.Tokenizer) *PrattParser {
	p := &PrattParser{
		tokenizer:      t,
		errors:         []ParseError{},
		prefixParseFns: make(map[tokenizer.TokenKind]prefixParseFn),
		infixParseFns:  make(map[tokenizer.TokenKind]infixParseFn),
	}

	// Register prefix parse functions
	p.registerPrefix(tokenizer.IDENT, p.parseIdentifier)
	p.registerPrefix(tokenizer.INT, p.parseIntegerLiteral)
	p.registerPrefix(tokenizer.FLOAT, p.parseFloatLiteral)
	p.registerPrefix(tokenizer.STRING, p.parseStringLiteral)
	p.registerPrefix(tokenizer.TRUE, p.parseBoolLiteral)
	p.registerPrefix(tokenizer.FALSE, p.parseBoolLiteral)
	p.registerPrefix(tokenizer.LPAREN, p.parseGroupedOrLambda)
	p.registerPrefix(tokenizer.PIPE, p.parseLambda)     // Rust-style lambda: |x| expr
	p.registerPrefix(tokenizer.MATCH, p.parseMatchExpr) // Match expressions

	// Register infix parse functions for Dingo operators
	p.registerInfix(tokenizer.QUESTION, p.parseQuestionOperator)
	p.registerInfix(tokenizer.QUESTION_QUESTION, p.parseNullCoalescing)
	p.registerInfix(tokenizer.QUESTION_DOT, p.parseSafeNavigation)

	// Register infix parse functions for standard Go operators
	p.registerInfix(tokenizer.DOT, p.parseSelectorExpr)
	p.registerInfix(tokenizer.LPAREN, p.parseCallExpr)

	// Register binary operators
	binaryOps := []tokenizer.TokenKind{
		tokenizer.OR, tokenizer.AND,
		tokenizer.EQ, tokenizer.NE, tokenizer.LT, tokenizer.GT, tokenizer.LE, tokenizer.GE,
		tokenizer.PLUS, tokenizer.MINUS, tokenizer.STAR, tokenizer.SLASH,
	}
	for _, op := range binaryOps {
		p.registerInfix(op, p.parseBinaryExpr)
	}

	// Initialize current and peek tokens
	p.nextToken()
	p.nextToken()

	return p
}

// registerPrefix registers a prefix parse function for a token type
func (p *PrattParser) registerPrefix(tokenType tokenizer.TokenKind, fn prefixParseFn) {
	p.prefixParseFns[tokenType] = fn
}

// registerInfix registers an infix parse function for a token type
func (p *PrattParser) registerInfix(tokenType tokenizer.TokenKind, fn infixParseFn) {
	p.infixParseFns[tokenType] = fn
}

// nextToken advances to the next token
func (p *PrattParser) nextToken() {
	p.curToken = p.peekToken
	p.peekToken = p.tokenizer.NextToken()
}

// parserState captures the full state for lookahead/backtracking
type parserState struct {
	curToken  tokenizer.Token
	peekToken tokenizer.Token
	tokPos    int
}

// saveState saves the current parser state for backtracking
func (p *PrattParser) saveState() parserState {
	return parserState{
		curToken:  p.curToken,
		peekToken: p.peekToken,
		tokPos:    p.tokenizer.SavePos(),
	}
}

// restoreState restores a previously saved parser state
func (p *PrattParser) restoreState(state parserState) {
	p.curToken = state.curToken
	p.peekToken = state.peekToken
	p.tokenizer.RestorePos(state.tokPos)
}

// source returns the original source bytes for extracting text.
func (p *PrattParser) source() []byte {
	return p.tokenizer.Source()
}

// curTokenIs checks if current token is of given type
func (p *PrattParser) curTokenIs(t tokenizer.TokenKind) bool {
	return p.curToken.Kind == t
}

// peekTokenIs checks if peek token is of given type
func (p *PrattParser) peekTokenIs(t tokenizer.TokenKind) bool {
	return p.peekToken.Kind == t
}

// expectPeek advances if peek token is expected type
func (p *PrattParser) expectPeek(t tokenizer.TokenKind) bool {
	if p.peekTokenIs(t) {
		p.nextToken()
		return true
	}
	p.peekError(t)
	return false
}

// peekPrecedence returns the precedence of the peek token
func (p *PrattParser) peekPrecedence() int {
	if prec, ok := operatorPrecedence[p.peekToken.Kind]; ok {
		return prec
	}
	return PrecLowest
}

// curPrecedence returns the precedence of the current token
func (p *PrattParser) curPrecedence() int {
	if prec, ok := operatorPrecedence[p.curToken.Kind]; ok {
		return prec
	}
	return PrecLowest
}

// ParseExpression is the core Pratt parsing method
// minPrec is the minimum precedence level for operators to bind to the left expression
func (p *PrattParser) ParseExpression(minPrec int) ast.Expr {
	// Parse prefix expression (literals, identifiers, unary ops, grouped expressions)
	prefix := p.prefixParseFns[p.curToken.Kind]
	if prefix == nil {
		p.noPrefixParseFnError(p.curToken.Kind)
		return nil
	}

	leftExpr := prefix()

	// Parse infix/postfix expressions while precedence allows
	for !p.peekTokenIs(tokenizer.EOF) && !p.peekTokenIs(tokenizer.SEMICOLON) && minPrec < p.peekPrecedence() {
		infix := p.infixParseFns[p.peekToken.Kind]
		if infix == nil {
			return leftExpr
		}

		p.nextToken()
		leftExpr = infix(leftExpr)
	}

	return leftExpr
}

// Prefix parse functions

func (p *PrattParser) parseIdentifier() ast.Expr {
	// Check if this is a single-param TypeScript lambda: x => expr
	if p.peekTokenIs(tokenizer.ARROW) {
		return p.parseLambda()
	}

	// Return a DingoIdent node
	return &ast.DingoIdent{
		NamePos: p.curToken.Pos,
		Name:    p.curToken.Lit,
	}
}

func (p *PrattParser) parseIntegerLiteral() ast.Expr {
	return &ast.RawExpr{
		StartPos: p.curToken.Pos,
		EndPos:   p.curToken.End,
		Text:     p.curToken.Lit,
	}
}

func (p *PrattParser) parseFloatLiteral() ast.Expr {
	// TODO: Return proper ast.BasicLit node
	return nil
}

func (p *PrattParser) parseStringLiteral() ast.Expr {
	return &ast.RawExpr{
		StartPos: p.curToken.Pos,
		EndPos:   p.curToken.End,
		Text:     p.curToken.Lit,
	}
}

func (p *PrattParser) parseBoolLiteral() ast.Expr {
	lit := p.curToken.Lit
	if lit == "" {
		// Fallback if tokenizer doesn't provide literal text
		if p.curToken.Kind == tokenizer.TRUE {
			lit = "true"
		} else {
			lit = "false"
		}
	}
	return &ast.RawExpr{
		StartPos: p.curToken.Pos,
		EndPos:   p.curToken.End,
		Text:     lit,
	}
}

// parseGroupedOrLambda handles both grouped expressions and TypeScript lambdas
// Performs lookahead to distinguish (expr) from (params) => body
func (p *PrattParser) parseGroupedOrLambda() ast.Expr {
	// Check if this is a TypeScript lambda
	lambda := p.parseLambda()
	if lambda != nil {
		return lambda
	}

	// Not a lambda, parse as grouped expression
	return p.parseGroupedExpression()
}

func (p *PrattParser) parseGroupedExpression() ast.Expr {
	p.nextToken() // consume '('

	expr := p.ParseExpression(PrecLowest)

	if !p.expectPeek(tokenizer.RPAREN) {
		return nil
	}

	return expr
}

// Infix parse functions for Dingo operators

// parseErrorPropagation handles the postfix ? operator (x?) with optional error transformation.
//
// Syntax patterns:
//   - expr?                      (basic - propagate error as-is)
//   - expr ? "message"           (context wrapping with fmt.Errorf)
//   - expr ? |err| transform     (Rust-style lambda transform)
//   - expr ? (err) => transform  (TypeScript-style lambda transform)
//   - expr ? err => transform    (TypeScript single-param lambda)
//
// Examples:
//
//	value := getData()?
//	order := fetchOrder(id) ? "fetch failed"
//	user := loadUser(id) ? |err| wrap("user", err)
//	config := loadConfig(path) ? (e) => fmt.Errorf("config: %w", e)
func (p *PrattParser) parseErrorPropagation(left ast.Expr) ast.Expr {
	questionPos := p.curToken.Pos
	p.nextToken() // Consume the ? token

	expr := &ast.ErrorPropExpr{
		Question: questionPos,
		Operand:  left,
		// ResultType and ErrorType will be filled by type checker during semantic analysis
	}

	// Pattern 1: ? "message" (string context)
	if p.curTokenIs(tokenizer.STRING) {
		msg := p.curToken.Lit
		// Strip quotes if tokenizer includes them
		if len(msg) >= 2 && msg[0] == '"' && msg[len(msg)-1] == '"' {
			msg = msg[1 : len(msg)-1]
		} else if len(msg) >= 2 && msg[0] == '`' && msg[len(msg)-1] == '`' {
			// Raw string literals
			msg = msg[1 : len(msg)-1]
		}
		expr.ErrorContext = &ast.ErrorContext{
			Message:    msg,
			MessagePos: p.curToken.Pos,
		}
		p.nextToken() // consume string
		return expr
	}

	// Pattern 2: ? |err| transform (Rust-style lambda)
	if p.curTokenIs(tokenizer.PIPE) {
		lambda := p.parseRustLambda()
		if lambda != nil {
			expr.ErrorTransform = lambda.(*ast.LambdaExpr)
		}
		return expr
	}

	// Pattern 3: ? (err) => transform (TypeScript-style lambda with parens)
	if p.curTokenIs(tokenizer.LPAREN) && p.isTypeScriptLambda() {
		lambda := p.parseTSLambda()
		if lambda != nil {
			expr.ErrorTransform = lambda.(*ast.LambdaExpr)
		}
		return expr
	}

	// Pattern 4: ? err => transform (TypeScript single-param without parens)
	if p.curTokenIs(tokenizer.IDENT) && p.peekTokenIs(tokenizer.ARROW) {
		lambda := p.parseTSSingleParamLambda()
		if lambda != nil {
			expr.ErrorTransform = lambda.(*ast.LambdaExpr)
		}
		return expr
	}

	// Pattern 5: basic ? (no transformation)
	return expr
}

// parseNullCoalescing handles the infix ?? operator (a ?? b)
// Right-associative: a ?? b ?? c is parsed as a ?? (b ?? c)
func (p *PrattParser) parseNullCoalescing(left ast.Expr) ast.Expr {
	opPos := p.curToken.Pos
	precedence := p.curPrecedence()

	// Move to right operand
	p.nextToken()

	// Right-associative: use same precedence (not precedence + 1)
	// This makes a ?? b ?? c parse as a ?? (b ?? c)
	right := p.ParseExpression(precedence)

	return &ast.NullCoalesceExpr{
		Left:  left,
		OpPos: opPos,
		Right: right,
	}
}

// parseSafeNavigation handles the postfix ?. operator (x?.field or x?.method(args))
func (p *PrattParser) parseSafeNavigation(left ast.Expr) ast.Expr {
	opPos := p.curToken.Pos

	// After ?., we expect a field name or method call
	if !p.expectPeek(tokenizer.IDENT) {
		return nil
	}

	// Create identifier using Dingo AST
	sel := &ast.DingoIdent{
		NamePos: p.curToken.Pos,
		Name:    p.curToken.Lit,
	}

	// Check if this is a method call (next token is LPAREN)
	if p.peekTokenIs(tokenizer.LPAREN) {
		// Method call: x?.method(args)
		p.nextToken() // consume IDENT
		p.nextToken() // consume LPAREN

		// Parse arguments
		args := []ast.Expr{}
		if !p.curTokenIs(tokenizer.RPAREN) {
			// Parse first argument
			arg := p.ParseExpression(PrecLowest)
			if arg != nil {
				args = append(args, arg)
			}

			// Parse remaining arguments
			for p.peekTokenIs(tokenizer.COMMA) {
				p.nextToken() // consume comma
				p.nextToken() // move to next argument
				arg := p.ParseExpression(PrecLowest)
				if arg != nil {
					args = append(args, arg)
				}
			}
		}

		// Expect closing paren
		if !p.expectPeek(tokenizer.RPAREN) {
			return nil
		}

		return &ast.SafeNavCallExpr{
			X:     left,
			OpPos: opPos,
			Fun:   sel,
			Args:  args,
		}
	}

	// Field access: x?.field
	return &ast.SafeNavExpr{
		X:     left,
		OpPos: opPos,
		Sel:   sel,
	}
}

// Error handling

func (p *PrattParser) noPrefixParseFnError(t tokenizer.TokenKind) {
	msg := fmt.Sprintf("no prefix parse function for %s", t)
	p.errors = append(p.errors, ParseError{
		Pos:     p.curToken.Pos,
		Line:    p.curToken.Line,
		Column:  p.curToken.Column,
		Message: msg,
	})
}

func (p *PrattParser) peekError(t tokenizer.TokenKind) {
	msg := fmt.Sprintf("expected next token to be %s, got %s instead", t, p.peekToken.Kind)
	p.errors = append(p.errors, ParseError{
		Pos:     p.peekToken.Pos,
		Line:    p.peekToken.Line,
		Column:  p.peekToken.Column,
		Message: msg,
	})
}

// Errors returns all parse errors encountered
func (p *PrattParser) Errors() []ParseError {
	return p.errors
}

// parseSelectorExpr handles the infix DOT operator (x.field or pkg.Func)
// For qualified identifiers like fmt.Sprintf, this builds the full expression
func (p *PrattParser) parseSelectorExpr(left ast.Expr) ast.Expr {
	// Current token is DOT (not used in RawExpr approach)
	_ = p.curToken.Pos

	// Expect identifier after DOT
	if !p.expectPeek(tokenizer.IDENT) {
		p.addError("expected identifier after '.'")
		return nil
	}

	// Get the selector identifier
	selName := p.curToken.Lit
	selEnd := p.curToken.End

	// Build the full selector expression as RawExpr
	// We need to reconstruct the full text: left.selector
	leftText := ""
	if ident, ok := left.(*ast.DingoIdent); ok {
		leftText = ident.Name
	} else if raw, ok := left.(*ast.RawExpr); ok {
		leftText = raw.Text
	} else {
		// Fallback: use left's String() method
		leftText = left.String()
	}

	fullText := leftText + "." + selName

	return &ast.RawExpr{
		StartPos: left.Pos(),
		EndPos:   selEnd,
		Text:     fullText,
	}
}

// parseCallExpr handles the infix LPAREN operator (function calls)
// This allows parsing expressions like fmt.Sprintf("hello %s", name)
func (p *PrattParser) parseCallExpr(left ast.Expr) ast.Expr {
	// Current token is LPAREN (not used in RawExpr approach)
	_ = p.curToken.Pos

	// Parse arguments
	args := []string{}
	if !p.peekTokenIs(tokenizer.RPAREN) {
		// Parse first argument
		p.nextToken()
		arg := p.ParseExpression(PrecLowest)
		if arg != nil {
			args = append(args, arg.String())
		}

		// Parse remaining arguments
		for p.peekTokenIs(tokenizer.COMMA) {
			p.nextToken() // consume comma
			p.nextToken() // move to next argument
			arg := p.ParseExpression(PrecLowest)
			if arg != nil {
				args = append(args, arg.String())
			}
		}
	}

	// Expect closing paren
	if !p.expectPeek(tokenizer.RPAREN) {
		return nil
	}
	rparenPos := p.curToken.End

	// Build the full call expression as RawExpr
	funcText := left.String()
	argsText := ""
	for i, arg := range args {
		if i > 0 {
			argsText += ", "
		}
		argsText += arg
	}

	fullText := funcText + "(" + argsText + ")"

	return &ast.RawExpr{
		StartPos: left.Pos(),
		EndPos:   rparenPos,
		Text:     fullText,
	}
}

// addError is a helper to add errors with current token position
func (p *PrattParser) addError(msg string) {
	p.errors = append(p.errors, ParseError{
		Pos:     p.curToken.Pos,
		Line:    p.curToken.Line,
		Column:  p.curToken.Column,
		Message: msg,
	})
}

// parseQuestionOperator handles both error propagation (?) and ternary (? :)
// Disambiguates by looking ahead after parsing the first expression following ?
//
// Patterns:
//   - expr?                        -> error propagation (postfix)
//   - expr ? "message"             -> error propagation with context
//   - expr ? |err| transform       -> error propagation with transform
//   - cond ? trueVal : falseVal    -> ternary operator
//
// Disambiguation strategy:
// 1. If ? is followed by terminator -> error propagation
// 2. If ? is followed by string literal -> error propagation with context
// 3. If ? is followed by pipe (|) -> error propagation with lambda
// 4. Otherwise, parse expression and check for colon
//    - If colon found -> ternary
//    - If no colon -> error propagation
func (p *PrattParser) parseQuestionOperator(left ast.Expr) ast.Expr {
	questionPos := p.curToken.Pos

	// Save parser state for potential backtracking
	state := p.saveState()

	p.nextToken() // consume ?

	// Pattern 1: ? followed by terminator = error propagation
	if p.isExpressionTerminator() {
		return &ast.ErrorPropExpr{
			Question: questionPos,
			Operand:  left,
		}
	}

	// Pattern 1b: ? followed by another ? = first is error propagation, second will be ternary
	// Example: getData()? ? "valid" : "invalid"
	if p.curTokenIs(tokenizer.QUESTION) {
		return &ast.ErrorPropExpr{
			Question: questionPos,
			Operand:  left,
		}
	}

	// Pattern 2: ? "string" - could be error propagation OR ternary
	// Disambiguate by checking if there's a colon after the string
	if p.curTokenIs(tokenizer.STRING) {
		// Lookahead: is there a colon after this string?
		savedState2 := p.saveState()
		p.nextToken() // move past string
		hasColon := p.curTokenIs(tokenizer.COLON)
		p.restoreState(savedState2) // restore to string token

		if !hasColon {
			// No colon = error propagation with context
			p.restoreState(state)
			p.nextToken() // re-consume ?
			return p.parseErrorPropagation(left)
		}
		// Has colon = ternary, fall through to ternary parsing below
	}

	// Pattern 3: ? | = error propagation with lambda transform
	if p.curTokenIs(tokenizer.PIPE) {
		p.restoreState(state)
		p.nextToken() // re-consume ?
		return p.parseErrorPropagation(left)
	}

	// Pattern 4: ? ( where ( starts a lambda = error propagation with lambda
	if p.curTokenIs(tokenizer.LPAREN) && p.isTypeScriptLambda() {
		p.restoreState(state)
		p.nextToken() // re-consume ?
		return p.parseErrorPropagation(left)
	}

	// Pattern 5: ? ident => = error propagation with single-param lambda
	if p.curTokenIs(tokenizer.IDENT) && p.peekTokenIs(tokenizer.ARROW) {
		p.restoreState(state)
		p.nextToken() // re-consume ?
		return p.parseErrorPropagation(left)
	}

	// Try parsing as ternary: parse true branch expression
	trueExpr := p.ParseExpression(PrecTernary)

	// Check for colon to confirm ternary
	if p.peekTokenIs(tokenizer.COLON) {
		p.nextToken() // move to :
		colonPos := p.curToken.Pos
		p.nextToken() // consume :

		// Right-associative: parse false branch at same precedence
		// This makes a ? b : c ? d : e parse as a ? b : (c ? d : e)
		falseExpr := p.ParseExpression(PrecTernary)

		return &ast.TernaryExpr{
			Cond:     left,
			Question: questionPos,
			True:     trueExpr,
			Colon:    colonPos,
			False:    falseExpr,
		}
	}

	// No colon found - this is error propagation without context
	// Backtrack and parse as error propagation
	p.restoreState(state)
	p.nextToken() // re-consume ?
	return p.parseErrorPropagation(left)
}

// isExpressionTerminator returns true if current token ends an expression
func (p *PrattParser) isExpressionTerminator() bool {
	switch p.curToken.Kind {
	case tokenizer.EOF, tokenizer.SEMICOLON, tokenizer.RPAREN,
		tokenizer.RBRACE, tokenizer.COMMA, tokenizer.COLON,
		tokenizer.RBRACKET:
		return true
	}
	return false
}

// parseBinaryExpr parses binary expressions (left-associative)
// Handles operators like: +, -, *, /, ==, !=, <, >, <=, >=, &&, ||
func (p *PrattParser) parseBinaryExpr(left ast.Expr) ast.Expr {
	opToken := p.curToken
	precedence := p.curPrecedence()

	p.nextToken() // consume operator

	// Left-associative: parse right operand at precedence + 1
	// This prevents same-precedence operators from binding on the right
	// and allows lower-precedence operators (like ternary) to bind at top level
	right := p.ParseExpression(precedence + 1)

	// Build RawExpr with formatted binary expression
	leftStr := p.exprToString(left)
	rightStr := p.exprToString(right)

	return &ast.RawExpr{
		StartPos: opToken.Pos,
		EndPos:   opToken.End,
		Text:     leftStr + " " + opToken.Lit + " " + rightStr,
	}
}

// exprToString converts an AST expression to its string representation
func (p *PrattParser) exprToString(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.DingoIdent:
		return e.Name
	case *ast.RawExpr:
		return e.Text
	default:
		// Fallback to String() method if available
		if stringer, ok := expr.(interface{ String() string }); ok {
			return stringer.String()
		}
		return ""
	}
}
