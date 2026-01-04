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
// Checks if current LPAREN starts a lambda: (params) => ... or (params): RetType => ...
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
	// 2. (): RetType => ...  (no params, return type)
	// 3. (x) => ...  (single param)
	// 4. (x): RetType => ...  (single param, return type)
	// 5. (x: Type) => ...  (single param with Dingo-style type)
	// 6. (x: Type): RetType => ...  (single param with Dingo-style type and return type)
	// 7. (x, y) => ...  (multiple params)
	// 8. (x: Type, y: Type): RetType => ...  (multiple params with types and return type)
	// 9. (x Type) => ...  (single param with Go-style type, after TransformSource)
	// 10. (x Type): RetType => ...  (Go-style type with return type)

	isLambda := false

	// Pattern 1/2: () => ... or (): RetType => ...
	if p.curTokenIs(tokenizer.RPAREN) {
		p.nextToken() // move past RPAREN
		// Check for optional return type
		if p.curTokenIs(tokenizer.COLON) {
			p.nextToken() // move past COLON
			if p.curTokenIs(tokenizer.IDENT) {
				p.nextToken() // move past return type
			}
		}
		if p.curTokenIs(tokenizer.ARROW) {
			isLambda = true
		}
	} else if p.curTokenIs(tokenizer.IDENT) {
		// Could be param name, scan ahead
		p.nextToken() // move past IDENT

		// Check for: RPAREN, COLON, COMMA, or IDENT (Go-style type)
		if p.curTokenIs(tokenizer.RPAREN) {
			// (x) - check for return type or =>
			p.nextToken() // move past RPAREN
			if p.curTokenIs(tokenizer.COLON) {
				p.nextToken() // move past COLON
				if p.curTokenIs(tokenizer.IDENT) {
					p.nextToken() // move past return type
				}
			}
			if p.curTokenIs(tokenizer.ARROW) {
				// Pattern 3/4: (x) => or (x): RetType =>
				isLambda = true
			}
		} else if p.curTokenIs(tokenizer.COLON) {
			// Pattern 5/6/7/8: (x: Type) => or (x: Type, y: Type) =>
			// Skip type annotation
			p.nextToken() // move past COLON
			if p.curTokenIs(tokenizer.IDENT) {
				p.nextToken() // move past Type
				if p.curTokenIs(tokenizer.RPAREN) {
					// (x: Type) - check for return type or =>
					p.nextToken() // move past RPAREN
					if p.curTokenIs(tokenizer.COLON) {
						p.nextToken() // move past COLON
						if p.curTokenIs(tokenizer.IDENT) {
							p.nextToken() // move past return type
						}
					}
					if p.curTokenIs(tokenizer.ARROW) {
						isLambda = true
					}
				} else if p.curTokenIs(tokenizer.COMMA) {
					// Pattern 5/6/8: (x: Type, ...) - multiple typed params
					isLambda = true
				}
			}
		} else if p.curTokenIs(tokenizer.IDENT) {
			// Pattern 9/10: Go-style type annotation (x Type) or (x Type, ...)
			// After TransformSource, "x: Type" becomes "x Type"
			p.nextToken() // move past Type
			if p.curTokenIs(tokenizer.RPAREN) {
				// (x Type) - check for return type or =>
				p.nextToken() // move past RPAREN
				if p.curTokenIs(tokenizer.COLON) {
					// Dingo-style return type: ): RetType =>
					p.nextToken() // move past COLON
					if p.curTokenIs(tokenizer.IDENT) {
						p.nextToken() // move past return type
					}
				} else if p.curTokenIs(tokenizer.IDENT) {
					// Go-style return type (after TransformSource): ) RetType =>
					// The colon was already transformed away
					p.nextToken() // move past return type
				}
				if p.curTokenIs(tokenizer.ARROW) {
					isLambda = true
				}
			} else if p.curTokenIs(tokenizer.COMMA) {
				// Pattern 7: (x Type, ...) - multiple Go-style typed params
				isLambda = true
			}
		} else if p.curTokenIs(tokenizer.COMMA) {
			// Pattern 4: (x, ...) - multiple params without types
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

	lambdaExpr := &ast.LambdaExpr{
		LambdaPos:  lambdaPos,
		Style:      ast.RustStyle,
		Params:     params,
		ReturnType: returnType,
		Body:       body,
		IsBlock:    isBlock,
	}

	// Collect LambdaExpr for lint analyzers
	p.collectDingoNode(lambdaExpr)

	return lambdaExpr
}

// parseTSLambda parses TypeScript-style lambda: (params) => body or (params): RetType => body
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

	// Check for optional return type: : Type or just Type (after TransformSource)
	var returnType string
	if p.curTokenIs(tokenizer.COLON) {
		// Dingo-style: ): Type =>
		p.nextToken() // consume :
		if p.curTokenIs(tokenizer.IDENT) {
			returnType = p.curToken.Lit
			p.nextToken() // consume return type
		} else {
			p.errors = append(p.errors, ParseError{
				Pos:     p.curToken.Pos,
				Line:    p.curToken.Line,
				Column:  p.curToken.Column,
				Message: "expected type after ':'",
			})
			return nil
		}
	} else if p.curTokenIs(tokenizer.IDENT) && p.peekTokenIs(tokenizer.ARROW) {
		// Go-style (after TransformSource): ) Type =>
		// The colon was already transformed away by TransformSource
		returnType = p.curToken.Lit
		p.nextToken() // consume return type
	}

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

	lambdaExpr := &ast.LambdaExpr{
		LambdaPos:  lambdaPos,
		Style:      ast.TypeScriptStyle,
		Params:     params,
		ReturnType: returnType,
		Body:       body,
		IsBlock:    isBlock,
	}

	// Collect LambdaExpr for lint analyzers
	p.collectDingoNode(lambdaExpr)

	return lambdaExpr
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

	lambdaExpr := &ast.LambdaExpr{
		LambdaPos:  lambdaPos,
		Style:      ast.TypeScriptStyle,
		Params:     params,
		ReturnType: "",
		Body:       body,
		IsBlock:    isBlock,
	}

	// Collect LambdaExpr for lint analyzers
	p.collectDingoNode(lambdaExpr)

	return lambdaExpr
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
// Supports:
//   - x              (no type)
//   - x: Type        (Dingo-style with colon)
//   - x Type         (Go-style without colon, after TransformSource)
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

	// Check for type annotation: : Type (Dingo-style)
	if p.peekTokenIs(tokenizer.COLON) {
		p.nextToken() // consume IDENT (param name)
		p.nextToken() // consume COLON

		if p.curTokenIs(tokenizer.IDENT) {
			paramType = p.curToken.Lit
			// Token will be consumed by caller's nextToken()
		} else {
			p.errors = append(p.errors, ParseError{
				Pos:     p.curToken.Pos,
				Line:    p.curToken.Line,
				Column:  p.curToken.Column,
				Message: "expected type after ':'",
			})
			return nil
		}
	} else if p.peekTokenIs(tokenizer.IDENT) {
		// Go-style type annotation: x Type (after TransformSource removes colon)
		// Type is followed by COMMA, RPAREN, or PIPE (for rust-style)
		p.nextToken() // consume param name
		paramType = p.curToken.Lit
		// Token will be consumed by caller's nextToken()
	}
	// If no type annotation, param name token will be consumed by caller's nextToken()

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
// Returns the raw block text (preserving whitespace) and true (isBlock)
func (p *PrattParser) parseLambdaBlock() (string, bool) {
	// Record the starting byte position (at the opening brace)
	startPos := p.curToken.BytePos()
	braceDepth := 0
	var endPos int

	// Scan tokens until matching closing brace
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
				endPos = p.curToken.ByteEnd()
				p.nextToken() // consume closing brace
				break
			}
		}

		p.nextToken()
	}

	// Extract original source bytes (preserves whitespace/newlines)
	src := p.source()
	if endPos > startPos && endPos <= len(src) {
		return string(src[startPos:endPos]), true
	}
	return "{}", true
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
