package parser

import (
	"fmt"

	"github.com/MadAppGang/dingo/pkg/ast"
	"github.com/MadAppGang/dingo/pkg/tokenizer"
)

// parseMatchExpr parses a complete match expression
// Grammar: 'match' scrutinee '{' arm* '}'
// Called when MATCH token is detected in prefix position
func (p *PrattParser) parseMatchExpr() ast.Expr {
	matchPos := p.curToken.Pos

	// Move to scrutinee
	p.nextToken()

	// Parse scrutinee expression (everything until '{')
	scrutinee := p.parseScrutinee()
	if scrutinee == nil {
		return nil
	}

	// Expect opening brace
	if !p.expectPeek(tokenizer.LBRACE) {
		return nil
	}
	openBrace := p.curToken.Pos

	// Parse match arms
	arms, comments := p.parseMatchArms()

	// Expect closing brace
	if !p.expectPeek(tokenizer.RBRACE) {
		return nil
	}
	closeBrace := p.curToken.Pos

	return &ast.MatchExpr{
		Match:      matchPos,
		Scrutinee:  scrutinee,
		OpenBrace:  openBrace,
		Arms:       arms,
		CloseBrace: closeBrace,
		IsExpr:     false, // Will be determined during transformation
		MatchID:    0,     // Will be assigned during transformation
		Comments:   comments,
	}
}

// parseScrutinee parses the expression being matched
// Parses until we hit opening brace '{'
func (p *PrattParser) parseScrutinee() ast.Expr {
	// Parse as a normal expression with lowest precedence
	// The expression parsing will stop before the '{'
	expr := p.ParseExpression(PrecLowest)

	// Skip any newlines between scrutinee and '{'
	for p.peekTokenIs(tokenizer.NEWLINE) {
		p.nextToken()
	}

	return expr
}

// parseMatchArms parses all match arms until closing brace
func (p *PrattParser) parseMatchArms() ([]*ast.MatchArm, []*ast.Comment) {
	var arms []*ast.MatchArm
	var comments []*ast.Comment

	for {
		// Skip newlines between arms
		for p.peekTokenIs(tokenizer.NEWLINE) {
			p.nextToken()
		}

		// Check for closing brace
		if p.peekTokenIs(tokenizer.RBRACE) {
			break
		}

		// Move to next token
		p.nextToken()

		// Check for standalone comment
		if p.curTokenIs(tokenizer.COMMENT) {
			comments = append(comments, &ast.Comment{
				Pos:  p.curToken.Pos,
				Text: p.curToken.Lit,
				Kind: ast.LineComment,
			})
			continue
		}

		// Parse match arm
		arm := p.parseMatchArm()
		if arm != nil {
			arms = append(arms, arm)
		}
	}

	return arms, comments
}

// parseMatchArm parses a single match arm
// Grammar: pattern ('|' pattern)* ('if' | 'where' guard)? '=>' body ','?
func (p *PrattParser) parseMatchArm() *ast.MatchArm {
	patternPos := p.curToken.Pos

	// Parse pattern (with potential alternatives: pattern | pattern | ...)
	pattern := p.parsePattern()
	if pattern == nil {
		return nil
	}

	arm := &ast.MatchArm{
		Pattern:    pattern,
		PatternPos: patternPos,
	}

	// Optional guard: 'if' or 'where' expr
	if p.peekTokenIs(tokenizer.IF) || p.peekTokenIs(tokenizer.WHERE) {
		p.nextToken()
		arm.GuardPos = p.curToken.Pos
		p.nextToken() // Move to guard expression start

		guard := p.parseGuard()
		if guard != nil {
			arm.Guard = guard
		}
		// parseGuard leaves us AT the ARROW token, so we're ready for the check below
	}

	// Expect '=>' arrow
	if arm.Guard != nil {
		// parseGuard leaves us AT the ARROW
		if !p.curTokenIs(tokenizer.ARROW) {
			p.errors = append(p.errors, ParseError{
				Pos:     p.curToken.Pos,
				Line:    p.curToken.Line,
				Column:  p.curToken.Column,
				Message: fmt.Sprintf("match arm missing '=>' after guard"),
			})
			return nil
		}
		arm.Arrow = p.curToken.Pos
	} else {
		// No guard, check peek token for ARROW
		if !p.expectPeek(tokenizer.ARROW) {
			p.errors = append(p.errors, ParseError{
				Pos:     p.curToken.Pos,
				Line:    p.curToken.Line,
				Column:  p.curToken.Column,
				Message: fmt.Sprintf("match arm missing '=>' after pattern"),
			})
			return nil
		}
		arm.Arrow = p.curToken.Pos
	}

	// Skip any newlines between arrow and body
	for p.peekTokenIs(tokenizer.NEWLINE) {
		p.nextToken()
	}

	// Move to body
	p.nextToken()

	// Parse body (expression or block)
	body, isBlock := p.parseMatchBody()
	if body != nil {
		arm.Body = body
		arm.IsBlock = isBlock
	}

	// Optional comma after body
	if p.peekTokenIs(tokenizer.COMMA) {
		p.nextToken()
		arm.Comma = p.curToken.Pos
	}

	// Check for trailing comment (after comma)
	if p.peekTokenIs(tokenizer.COMMENT) {
		p.nextToken()
		arm.Comment = &ast.Comment{
			Pos:  p.curToken.Pos,
			Text: p.curToken.Lit,
			Kind: ast.LineComment,
		}
	}

	return arm
}

// parsePattern parses a pattern (recursive for nested patterns)
// Grammar:
//   pattern := '_' | literal | identifier | constructor | tuple
//   constructor := IDENT '(' pattern (',' pattern)* ')'
//   tuple := '(' pattern (',' pattern)* ')'
func (p *PrattParser) parsePattern() ast.Pattern {
	tok := p.curToken

	switch tok.Kind {
	case tokenizer.UNDERSCORE:
		return &ast.WildcardPattern{Pos_: tok.Pos}

	case tokenizer.INT, tokenizer.FLOAT, tokenizer.STRING:
		return &ast.LiteralPattern{
			ValuePos: tok.Pos,
			Value:    tok.Lit,
			Kind:     literalKindFromToken(tok.Kind),
		}

	case tokenizer.IDENT:
		// Could be: variable, constructor (with parens/braces), or enum variant
		ident := tok

		// Check for constructor pattern: Ident(...) or Ident{...}
		if p.peekTokenIs(tokenizer.LPAREN) {
			return p.parseConstructorPattern(ident)
		}
		if p.peekTokenIs(tokenizer.LBRACE) {
			return p.parseStructPattern(ident)
		}

		// Check for known constructors without params
		if isNullaryConstructor(ident.Lit) {
			return &ast.ConstructorPattern{
				NamePos: ident.Pos,
				Name:    ident.Lit,
				Params:  nil,
			}
		}

		// Variable binding pattern
		return &ast.VariablePattern{
			NamePos: ident.Pos,
			Name:    ident.Lit,
		}

	case tokenizer.LPAREN:
		return p.parseTuplePattern()

	default:
		p.errors = append(p.errors, ParseError{
			Pos:     tok.Pos,
			Line:    tok.Line,
			Column:  tok.Column,
			Message: fmt.Sprintf("unexpected token %s in pattern", tok.Kind),
		})
		return nil
	}
}

// parseConstructorPattern parses Ok(x), Err(e), Some(v), etc.
// Also handles nested: Ok(Some(x))
func (p *PrattParser) parseConstructorPattern(nameTok tokenizer.Token) ast.Pattern {
	// Expect and consume '('
	if !p.expectPeek(tokenizer.LPAREN) {
		return nil
	}
	lparen := p.curToken.Pos

	var params []ast.Pattern

	// Parse parameters
	for !p.peekTokenIs(tokenizer.RPAREN) && !p.peekTokenIs(tokenizer.EOF) {
		// Skip newlines inside constructor
		for p.peekTokenIs(tokenizer.NEWLINE) {
			p.nextToken()
		}

		if p.peekTokenIs(tokenizer.RPAREN) {
			break
		}

		// Move to next pattern
		p.nextToken()

		// Parse nested pattern (RECURSIVE)
		param := p.parsePattern()
		if param != nil {
			params = append(params, param)
		}

		// Skip newlines after pattern
		for p.peekTokenIs(tokenizer.NEWLINE) {
			p.nextToken()
		}

		// Check for comma
		if !p.peekTokenIs(tokenizer.COMMA) {
			break
		}
		p.nextToken() // consume comma
	}

	// Expect closing ')'
	if !p.expectPeek(tokenizer.RPAREN) {
		return nil
	}
	rparen := p.curToken.Pos

	return &ast.ConstructorPattern{
		NamePos: nameTok.Pos,
		Name:    nameTok.Lit,
		LParen:  lparen,
		Params:  params,
		RParen:  rparen,
	}
}

// parseStructPattern parses struct-like patterns: Color_RGB{r, g, b}
func (p *PrattParser) parseStructPattern(nameTok tokenizer.Token) ast.Pattern {
	// Expect and consume '{'
	if !p.expectPeek(tokenizer.LBRACE) {
		return nil
	}
	lbrace := p.curToken.Pos

	var params []ast.Pattern

	// Parse parameters
	for !p.peekTokenIs(tokenizer.RBRACE) && !p.peekTokenIs(tokenizer.EOF) {
		// Skip newlines inside struct
		for p.peekTokenIs(tokenizer.NEWLINE) {
			p.nextToken()
		}

		if p.peekTokenIs(tokenizer.RBRACE) {
			break
		}

		// Move to next pattern
		p.nextToken()

		// Parse nested pattern (RECURSIVE)
		param := p.parsePattern()
		if param != nil {
			params = append(params, param)
		}

		// Skip newlines after pattern
		for p.peekTokenIs(tokenizer.NEWLINE) {
			p.nextToken()
		}

		// Check for comma
		if !p.peekTokenIs(tokenizer.COMMA) {
			break
		}
		p.nextToken() // consume comma
	}

	// Expect closing '}'
	if !p.expectPeek(tokenizer.RBRACE) {
		return nil
	}
	rbrace := p.curToken.Pos

	// Reuse ConstructorPattern for struct patterns
	return &ast.ConstructorPattern{
		NamePos: nameTok.Pos,
		Name:    nameTok.Lit,
		LParen:  lbrace, // Using LParen/RParen fields for brace positions
		Params:  params,
		RParen:  rbrace,
	}
}

// parseTuplePattern parses (a, b) or (Ok(x), Err(e))
func (p *PrattParser) parseTuplePattern() ast.Pattern {
	lparen := p.curToken.Pos

	var elements []ast.Pattern

	// Parse elements
	for !p.peekTokenIs(tokenizer.RPAREN) && !p.peekTokenIs(tokenizer.EOF) {
		// Skip newlines inside tuple
		for p.peekTokenIs(tokenizer.NEWLINE) {
			p.nextToken()
		}

		if p.peekTokenIs(tokenizer.RPAREN) {
			break
		}

		// Move to next pattern
		p.nextToken()

		// Parse nested pattern (RECURSIVE)
		elem := p.parsePattern()
		if elem != nil {
			elements = append(elements, elem)
		}

		// Skip newlines after pattern
		for p.peekTokenIs(tokenizer.NEWLINE) {
			p.nextToken()
		}

		// Check for comma
		if !p.peekTokenIs(tokenizer.COMMA) {
			break
		}
		p.nextToken() // consume comma
	}

	// Expect closing ')'
	if !p.expectPeek(tokenizer.RPAREN) {
		return nil
	}
	rparen := p.curToken.Pos

	return &ast.TuplePattern{
		LParen:   lparen,
		Elements: elements,
		RParen:   rparen,
	}
}

// parseGuard parses guard expression after 'if'/'where'
// Collects tokens until '=>' arrow
func (p *PrattParser) parseGuard() ast.Expr {
	// For now, collect raw tokens until we hit '=>'
	// A full implementation would parse this as a proper expression
	startPos := p.curToken.Pos
	var text string

	depth := 0
	for !p.curTokenIs(tokenizer.EOF) {
		// Check for arrow at depth 0
		if p.curTokenIs(tokenizer.ARROW) && depth == 0 {
			break
		}

		// Track depth for nested expressions
		if p.curTokenIs(tokenizer.LPAREN) || p.curTokenIs(tokenizer.LBRACE) {
			depth++
		} else if p.curTokenIs(tokenizer.RPAREN) || p.curTokenIs(tokenizer.RBRACE) {
			depth--
		}

		// Skip newlines
		if !p.curTokenIs(tokenizer.NEWLINE) {
			if text != "" {
				text += " "
			}
			text += p.curToken.Lit
		}

		p.nextToken()
	}

	// Move back one token since we've consumed one too many
	// (we're now at the ARROW token which should be consumed by parseMatchArm)
	// Note: This is a limitation of the current approach
	// A better solution would be to look ahead without consuming

	return &ast.RawExpr{
		StartPos: startPos,
		EndPos:   p.curToken.Pos,
		Text:     text,
	}
}

// parseMatchBody parses arm body (expression or block)
func (p *PrattParser) parseMatchBody() (ast.Expr, bool) {
	// Check for block body
	if p.curTokenIs(tokenizer.LBRACE) {
		return p.parseBlockBody()
	}

	// Expression body - parse as normal expression
	// Will stop at comma, closing brace, or newline (depending on context)
	expr := p.ParseExpression(PrecLowest)
	return expr, false
}

// parseBlockBody parses { ... } body
func (p *PrattParser) parseBlockBody() (ast.Expr, bool) {
	// For now, we'll create a RawExpr that captures the entire block
	// A full implementation would parse the block's statements
	startPos := p.curToken.Pos

	depth := 1
	p.nextToken() // consume '{'

	// Collect tokens until we find matching '}'
	var text string
	for depth > 0 && !p.curTokenIs(tokenizer.EOF) {
		if p.curTokenIs(tokenizer.LBRACE) {
			depth++
		} else if p.curTokenIs(tokenizer.RBRACE) {
			depth--
			if depth == 0 {
				break
			}
		}

		if p.curTokenIs(tokenizer.NEWLINE) {
			text += "\n"
		} else {
			if text != "" {
				text += " "
			}
			text += p.curToken.Lit
		}
		p.nextToken()
	}

	endPos := p.curToken.Pos

	return &ast.RawExpr{
		StartPos: startPos,
		EndPos:   endPos,
		Text:     "{" + text + "}",
	}, true
}

// Helper functions

func isNullaryConstructor(name string) bool {
	// Check for None or qualified variants like Option_None
	return name == "None" ||
	       name == "Option_None" ||
	       name == "Result_None"
}

func literalKindFromToken(kind tokenizer.TokenKind) ast.LiteralKind {
	switch kind {
	case tokenizer.INT:
		return ast.IntLiteral
	case tokenizer.FLOAT:
		return ast.FloatLiteral
	case tokenizer.STRING:
		return ast.StringLiteral
	default:
		return ast.IntLiteral
	}
}
