// Package parser provides lookahead utilities for disambiguation
package parser

import (
	"github.com/MadAppGang/dingo/pkg/tokenizer"
)

// questionKind classifies the type of ? operator usage
type questionKind int

const (
	qkUnknown             questionKind = iota
	qkErrorPropPostfix                 // expr? (basic error propagation)
	qkErrorWithContext                 // expr ? "message"
	qkErrorWithRustLambda              // expr ? |e| transform
	qkErrorWithTSLambda                // expr ? (e) => transform or expr ? e => transform
	qkTernary                          // cond ? trueVal : falseVal
)

// String returns a human-readable representation of the question kind
func (k questionKind) String() string {
	switch k {
	case qkErrorPropPostfix:
		return "error_prop_postfix"
	case qkErrorWithContext:
		return "error_prop_context"
	case qkErrorWithRustLambda:
		return "error_prop_rust_lambda"
	case qkErrorWithTSLambda:
		return "error_prop_ts_lambda"
	case qkTernary:
		return "ternary"
	default:
		return "unknown"
	}
}

// lookaheadResult captures the result of structured lookahead
type lookaheadResult struct {
	kind     questionKind
	consumed int // tokens consumed during lookahead (for debugging)
}

// classifyQuestionOperator determines what kind of ? usage this is
// Called AFTER consuming the ? token, with parser positioned on the token after ?
// Returns classification without permanently modifying parser state
//
// Decision table based on token after ?:
//
//	Token after ?   | Check                      | Classification
//	----------------|----------------------------|---------------
//	terminator      | -                          | qkErrorPropPostfix
//	?               | -                          | qkErrorPropPostfix (chained)
//	STRING          | has colon after?           | context/ternary
//	PIPE            | -                          | qkErrorWithRustLambda
//	LPAREN          | isTypeScriptLambda()?      | lambda/ternary
//	IDENT + ARROW   | -                          | qkErrorWithTSLambda
//	other           | try parse, has colon?      | ternary/postfix
func (p *PrattParser) classifyQuestionOperator() lookaheadResult {
	// Pattern 1: Terminator = postfix error propagation
	if p.isExpressionTerminator() {
		return lookaheadResult{kind: qkErrorPropPostfix, consumed: 0}
	}

	// Pattern 2: Another ? = chained error prop (first is postfix)
	if p.curTokenIs(tokenizer.QUESTION) {
		return lookaheadResult{kind: qkErrorPropPostfix, consumed: 0}
	}

	// Pattern 3: PIPE = Rust-style lambda
	if p.curTokenIs(tokenizer.PIPE) {
		return lookaheadResult{kind: qkErrorWithRustLambda, consumed: 0}
	}

	// Pattern 4: IDENT followed by ARROW = TS single-param lambda
	if p.curTokenIs(tokenizer.IDENT) && p.peekTokenIs(tokenizer.ARROW) {
		return lookaheadResult{kind: qkErrorWithTSLambda, consumed: 0}
	}

	// Pattern 5: LPAREN - could be TS lambda or grouped expression in ternary
	if p.curTokenIs(tokenizer.LPAREN) {
		if p.isTypeScriptLambda() {
			return lookaheadResult{kind: qkErrorWithTSLambda, consumed: 0}
		}
		// Not a lambda - continue to ternary check
	}

	// Pattern 6: STRING - needs lookahead to distinguish context from ternary
	if p.curTokenIs(tokenizer.STRING) {
		if p.hasColonAfterToken() {
			return lookaheadResult{kind: qkTernary, consumed: 1}
		}
		return lookaheadResult{kind: qkErrorWithContext, consumed: 0}
	}

	// Default: Try to detect ternary by looking for colon
	if p.hasTernaryColon() {
		return lookaheadResult{kind: qkTernary, consumed: 0}
	}

	// Fallback: Treat as postfix error propagation
	return lookaheadResult{kind: qkErrorPropPostfix, consumed: 0}
}

// hasColonAfterToken checks if there's a colon after the current token
// Used to distinguish "string" in context vs ternary
func (p *PrattParser) hasColonAfterToken() bool {
	state := p.saveState()
	defer p.restoreState(state)

	p.nextToken() // move past current token
	p.consumeNewlinesAndComments()
	return p.curTokenIs(tokenizer.COLON)
}

// maxTernaryLookahead limits how many tokens we scan looking for ternary colon.
// 20 tokens covers most reasonable ternary expressions (e.g., `cond ? fn(a,b,c) : d`)
// while preventing O(n) scanning on pathological inputs.
const maxTernaryLookahead = 20

// hasTernaryColon performs limited lookahead to detect ternary pattern
// Scans ahead to find : that would indicate ternary, respecting nesting
func (p *PrattParser) hasTernaryColon() bool {
	state := p.saveState()
	defer p.restoreState(state)

	depth := 0

	for i := 0; i < maxTernaryLookahead && !p.curTokenIs(tokenizer.EOF); i++ {
		switch p.curToken.Kind {
		case tokenizer.LPAREN, tokenizer.LBRACE, tokenizer.LBRACKET:
			depth++
		case tokenizer.RPAREN, tokenizer.RBRACE, tokenizer.RBRACKET:
			depth--
			if depth < 0 {
				return false // Unbalanced - not a valid ternary
			}
		case tokenizer.COLON:
			if depth == 0 {
				return true // Found colon at top level
			}
		case tokenizer.SEMICOLON, tokenizer.NEWLINE:
			if depth == 0 {
				return false // Hit statement boundary without colon
			}
		}
		p.nextToken()
	}

	return false
}

// lambdaClassification for TypeScript lambda detection
type lambdaClassification int

const (
	lcNotLambda        lambdaClassification = iota
	lcEmptyParams                           // () =>
	lcSingleParam                           // (x) =>
	lcSingleTypedParam                      // (x: Type) => or (x Type) =>
	lcMultiParam                            // (x, y) =>
	lcMultiTypedParam                       // (x: Type, y: Type) =>
	lcWithReturnType                        // (...): RetType =>
)

// String returns a human-readable representation of the lambda classification
func (lc lambdaClassification) String() string {
	switch lc {
	case lcNotLambda:
		return "not_lambda"
	case lcEmptyParams:
		return "empty_params"
	case lcSingleParam:
		return "single_param"
	case lcSingleTypedParam:
		return "single_typed_param"
	case lcMultiParam:
		return "multi_param"
	case lcMultiTypedParam:
		return "multi_typed_param"
	case lcWithReturnType:
		return "with_return_type"
	default:
		return "unknown"
	}
}

// classifyTSLambda provides detailed classification of TypeScript lambda patterns
// Returns classification and whether it's definitely a lambda
// This is more detailed than isTypeScriptLambda which just returns bool
func (p *PrattParser) classifyTSLambda() (lambdaClassification, bool) {
	if !p.curTokenIs(tokenizer.LPAREN) {
		return lcNotLambda, false
	}

	state := p.saveState()
	defer p.restoreState(state)

	p.nextToken() // consume LPAREN

	// Pattern: () => (empty params)
	if p.curTokenIs(tokenizer.RPAREN) {
		p.nextToken()
		if p.checkReturnTypeAndArrow() {
			return lcEmptyParams, true
		}
		return lcNotLambda, false
	}

	// Must start with IDENT for parameter
	if !p.curTokenIs(tokenizer.IDENT) {
		return lcNotLambda, false
	}

	p.nextToken() // past first IDENT

	// Pattern: (x) => (single param, no type)
	if p.curTokenIs(tokenizer.RPAREN) {
		p.nextToken()
		if p.checkReturnTypeAndArrow() {
			return lcSingleParam, true
		}
		return lcNotLambda, false
	}

	// Pattern: (x: Type) => or (x Type) => (single typed param)
	if p.curTokenIs(tokenizer.COLON) || p.curTokenIs(tokenizer.IDENT) {
		hasColon := p.curTokenIs(tokenizer.COLON)
		if hasColon {
			p.nextToken() // past COLON
		}
		if p.curTokenIs(tokenizer.IDENT) {
			p.nextToken() // past type
			if p.curTokenIs(tokenizer.RPAREN) {
				p.nextToken()
				if p.checkReturnTypeAndArrow() {
					return lcSingleTypedParam, true
				}
			} else if p.curTokenIs(tokenizer.COMMA) {
				// Multi-param with types
				return lcMultiTypedParam, true
			}
		}
		return lcNotLambda, false
	}

	// Pattern: (x, y) => (multi param)
	if p.curTokenIs(tokenizer.COMMA) {
		return lcMultiParam, true
	}

	return lcNotLambda, false
}

// checkReturnTypeAndArrow checks for optional return type and =>
func (p *PrattParser) checkReturnTypeAndArrow() bool {
	// Check for return type: : Type or just Type (Go-style after transform)
	if p.curTokenIs(tokenizer.COLON) {
		p.nextToken() // past COLON
		if p.curTokenIs(tokenizer.IDENT) {
			p.nextToken() // past type
		}
	} else if p.curTokenIs(tokenizer.IDENT) && p.peekTokenIs(tokenizer.ARROW) {
		p.nextToken() // past type
	}

	return p.curTokenIs(tokenizer.ARROW)
}
