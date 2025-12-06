package parser

import (
	"strings"

	"github.com/MadAppGang/dingo/pkg/ast"
	"github.com/MadAppGang/dingo/pkg/tokenizer"
)

// parseLambda is the main entry point for lambda parsing
// Called from parsePrefix when we detect a lambda starting token
func (p *PrattParser) parseLambda() ast.Expr {
	// Detect style based on current token
	if p.curTokenIs(tokenizer.PIPE) {
		return p.parseRustLambda()
	}

	if p.curTokenIs(tokenizer.LPAREN) {
		// Need lookahead to distinguish (expr) from (params) =>
		// Save current position
		if p.isTypeScriptLambda() {
			return p.parseTSLambda()
		}
		// Not a lambda, must be grouped expression
		// Return nil to let parseGroupedExpression handle it
		return nil
	}

	// Single-parameter TypeScript lambda without parens: x => expr
	if p.curTokenIs(tokenizer.IDENT) && p.peekTokenIs(tokenizer.ARROW) {
		return p.parseTSSingleParamLambda()
	}

	return nil
}

// isTypeScriptLambda performs lookahead to detect TypeScript lambda
// Checks if current LPAREN starts a lambda: (params) => ...
func (p *PrattParser) isTypeScriptLambda() bool {
	if !p.curTokenIs(tokenizer.LPAREN) {
		return false
	}

	// Save full parser state including tokenizer position for backtracking
	savedState := p.saveState()

	// Advance past LPAREN
	p.nextToken()

	// Check for patterns:
	// 1. () => ...  (no params)
	// 2. (x) => ...  (single param)
	// 3. (x: Type) => ...  (single param with type)
	// 4. (x, y) => ...  (multiple params)
	// 5. (x: Type, y: Type) => ...  (multiple params with types)

	isLambda := false

	// Pattern 1: () =>
	if p.curTokenIs(tokenizer.RPAREN) && p.peekTokenIs(tokenizer.ARROW) {
		isLambda = true
	} else if p.curTokenIs(tokenizer.IDENT) {
		// Could be param name, scan ahead
		p.nextToken() // move past IDENT

		// Check for: RPAREN, COLON, or COMMA
		if p.curTokenIs(tokenizer.RPAREN) && p.peekTokenIs(tokenizer.ARROW) {
			// Pattern 2: (x) =>
			isLambda = true
		} else if p.curTokenIs(tokenizer.COLON) {
			// Pattern 3: (x: Type) =>
			// Skip type annotation
			p.nextToken() // move past COLON
			if p.curTokenIs(tokenizer.IDENT) {
				p.nextToken() // move past Type
				if p.curTokenIs(tokenizer.RPAREN) && p.peekTokenIs(tokenizer.ARROW) {
					isLambda = true
				}
			}
		} else if p.curTokenIs(tokenizer.COMMA) {
			// Pattern 4/5: (x, ...) or (x: Type, ...)
			isLambda = true // Assume lambda if we see comma after param
		}
	}

	// Restore full parser state
	p.restoreState(savedState)

	return isLambda
}

// parseRustLambda parses Rust-style lambda: |params| body or |params| -> RetType { body }
func (p *PrattParser) parseRustLambda() ast.Expr {
	lambdaPos := p.curToken.Pos

	// We should already be at the opening |
	if !p.curTokenIs(tokenizer.PIPE) {
		p.errors = append(p.errors, ParseError{
			Pos:     p.curToken.Pos,
			Line:    p.curToken.Line,
			Column:  p.curToken.Column,
			Message: "expected '|' to start lambda",
		})
		return nil
	}
	p.nextToken() // move past opening |

	// Parse parameters
	params := p.parseLambdaParams(tokenizer.PIPE)

	// Expect closing |
	if !p.curTokenIs(tokenizer.PIPE) {
		p.errors = append(p.errors, ParseError{
			Pos:     p.curToken.Pos,
			Line:    p.curToken.Line,
			Column:  p.curToken.Column,
			Message: "expected '|' after lambda parameters",
		})
		return nil
	}

	p.nextToken() // consume closing |

	// Check for optional return type: -> Type
	var returnType string
	if p.curTokenIs(tokenizer.THIN_ARROW) {
		p.nextToken() // consume ->
		if p.curTokenIs(tokenizer.IDENT) {
			returnType = p.curToken.Lit
			p.nextToken()
		} else {
			p.errors = append(p.errors, ParseError{
				Pos:     p.curToken.Pos,
				Line:    p.curToken.Line,
				Column:  p.curToken.Column,
				Message: "expected type after '->'",
			})
			return nil
		}
	}

	// Parse body
	body, isBlock := p.parseLambdaBody()

	return &ast.LambdaExpr{
		LambdaPos:  lambdaPos,
		Style:      ast.RustStyle,
		Params:     params,
		ReturnType: returnType,
		Body:       body,
		IsBlock:    isBlock,
	}
}

// parseTSLambda parses TypeScript-style lambda: (params) => body
func (p *PrattParser) parseTSLambda() ast.Expr {
	lambdaPos := p.curToken.Pos

	// Expect LPAREN
	if !p.curTokenIs(tokenizer.LPAREN) {
		return nil
	}
	p.nextToken() // consume LPAREN

	// Parse parameters
	params := p.parseLambdaParams(tokenizer.RPAREN)

	// Expect closing RPAREN
	if !p.curTokenIs(tokenizer.RPAREN) {
		p.errors = append(p.errors, ParseError{
			Pos:     p.curToken.Pos,
			Line:    p.curToken.Line,
			Column:  p.curToken.Column,
			Message: "expected ')' after lambda parameters",
		})
		return nil
	}

	p.nextToken() // consume RPAREN

	// Expect =>
	if !p.curTokenIs(tokenizer.ARROW) {
		p.errors = append(p.errors, ParseError{
			Pos:     p.curToken.Pos,
			Line:    p.curToken.Line,
			Column:  p.curToken.Column,
			Message: "expected '=>' after lambda parameters",
		})
		return nil
	}

	p.nextToken() // consume =>

	// Parse body
	body, isBlock := p.parseLambdaBody()

	return &ast.LambdaExpr{
		LambdaPos:  lambdaPos,
		Style:      ast.TypeScriptStyle,
		Params:     params,
		ReturnType: "", // TypeScript style doesn't have return type annotation before body
		Body:       body,
		IsBlock:    isBlock,
	}
}

// parseTSSingleParamLambda parses TypeScript single-param lambda without parens: x => expr
func (p *PrattParser) parseTSSingleParamLambda() ast.Expr {
	lambdaPos := p.curToken.Pos

	// Current token is IDENT (param name)
	paramName := p.curToken.Lit
	params := []ast.LambdaParam{
		{Name: paramName, Type: ""}, // No type annotation for single-param form
	}

	p.nextToken() // consume IDENT

	// Expect =>
	if !p.curTokenIs(tokenizer.ARROW) {
		p.errors = append(p.errors, ParseError{
			Pos:     p.curToken.Pos,
			Line:    p.curToken.Line,
			Column:  p.curToken.Column,
			Message: "expected '=>' after parameter",
		})
		return nil
	}

	p.nextToken() // consume =>

	// Parse body
	body, isBlock := p.parseLambdaBody()

	return &ast.LambdaExpr{
		LambdaPos:  lambdaPos,
		Style:      ast.TypeScriptStyle,
		Params:     params,
		ReturnType: "",
		Body:       body,
		IsBlock:    isBlock,
	}
}

// parseLambdaParams parses lambda parameters until endToken
// endToken is PIPE for Rust style, RPAREN for TypeScript style
func (p *PrattParser) parseLambdaParams(endToken tokenizer.TokenKind) []ast.LambdaParam {
	var params []ast.LambdaParam

	// Handle empty param list
	if p.curTokenIs(endToken) {
		return params
	}

	// Parse first parameter
	param := p.parseLambdaParam()
	if param != nil {
		params = append(params, *param)
	}

	// Parse remaining parameters
	for p.peekTokenIs(tokenizer.COMMA) {
		p.nextToken() // consume current token
		p.nextToken() // consume COMMA

		param := p.parseLambdaParam()
		if param != nil {
			params = append(params, *param)
		}
	}

	p.nextToken() // move to endToken

	return params
}

// parseLambdaParam parses a single lambda parameter with optional type annotation
// Supports: x  or  x: Type
func (p *PrattParser) parseLambdaParam() *ast.LambdaParam {
	if !p.curTokenIs(tokenizer.IDENT) {
		p.errors = append(p.errors, ParseError{
			Pos:     p.curToken.Pos,
			Line:    p.curToken.Line,
			Column:  p.curToken.Column,
			Message: "expected parameter name",
		})
		return nil
	}

	paramName := p.curToken.Lit
	paramType := ""

	// Check for type annotation: : Type
	if p.peekTokenIs(tokenizer.COLON) {
		p.nextToken() // consume IDENT
		p.nextToken() // consume COLON

		if p.curTokenIs(tokenizer.IDENT) {
			paramType = p.curToken.Lit
		} else {
			p.errors = append(p.errors, ParseError{
				Pos:     p.curToken.Pos,
				Line:    p.curToken.Line,
				Column:  p.curToken.Column,
				Message: "expected type after ':'",
			})
			return nil
		}
	}

	return &ast.LambdaParam{
		Name: paramName,
		Type: paramType,
	}
}

// parseLambdaBody parses lambda body (expression or block)
// Returns: (body string, isBlock bool)
func (p *PrattParser) parseLambdaBody() (string, bool) {
	// Check if body is a block { ... }
	if p.curTokenIs(tokenizer.LBRACE) {
		return p.parseLambdaBlock()
	}

	// Expression body - collect tokens until natural boundary
	return p.parseLambdaExpression()
}

// parseLambdaBlock parses a block body { statements }
// Returns the raw block text and true (isBlock)
func (p *PrattParser) parseLambdaBlock() (string, bool) {
	var bodyTokens []string
	braceDepth := 0

	// Collect tokens until matching closing brace
	for {
		if p.curTokenIs(tokenizer.EOF) {
			p.errors = append(p.errors, ParseError{
				Pos:     p.curToken.Pos,
				Line:    p.curToken.Line,
				Column:  p.curToken.Column,
				Message: "unexpected EOF in lambda block",
			})
			break
		}

		if p.curTokenIs(tokenizer.LBRACE) {
			braceDepth++
		} else if p.curTokenIs(tokenizer.RBRACE) {
			braceDepth--
			if braceDepth == 0 {
				bodyTokens = append(bodyTokens, p.curToken.Lit)
				p.nextToken() // consume closing brace
				break
			}
		}

		bodyTokens = append(bodyTokens, p.curToken.Lit)
		p.nextToken()
	}

	return strings.Join(bodyTokens, " "), true
}

// parseLambdaExpression parses an expression body
// Returns the raw expression text and false (not a block)
func (p *PrattParser) parseLambdaExpression() (string, bool) {
	var exprTokens []string

	// Collect tokens until natural expression boundary
	// Stop at: comma (unless in nested parens/brackets), RPAREN (if not in call), RBRACE, semicolon, newline
	parenDepth := 0
	bracketDepth := 0
	braceDepth := 0

	for {
		if p.curTokenIs(tokenizer.EOF) || p.curTokenIs(tokenizer.SEMICOLON) {
			break
		}

		// Track nesting depth
		if p.curTokenIs(tokenizer.LPAREN) {
			parenDepth++
		} else if p.curTokenIs(tokenizer.RPAREN) {
			if parenDepth == 0 {
				// Unmatched closing paren - end of lambda expression
				break
			}
			parenDepth--
		} else if p.curTokenIs(tokenizer.LBRACKET) {
			bracketDepth++
		} else if p.curTokenIs(tokenizer.RBRACKET) {
			if bracketDepth == 0 {
				break
			}
			bracketDepth--
		} else if p.curTokenIs(tokenizer.LBRACE) {
			braceDepth++
		} else if p.curTokenIs(tokenizer.RBRACE) {
			if braceDepth == 0 {
				// Unmatched closing brace - end of lambda
				break
			}
			braceDepth--
		} else if p.curTokenIs(tokenizer.COMMA) {
			// Comma only terminates if we're not in nested parens/brackets
			if parenDepth == 0 && bracketDepth == 0 && braceDepth == 0 {
				break
			}
		}

		exprTokens = append(exprTokens, p.curToken.Lit)
		p.nextToken()
	}

	return strings.Join(exprTokens, " "), false
}
