// Package parser provides statement parsing for the Dingo parser
package parser

import (
	"go/ast"
	"go/token"

	"github.com/MadAppGang/dingo/pkg/tokenizer"
)

// StmtParser extends PrattParser with statement parsing capabilities
type StmtParser struct {
	*PrattParser
	recovery *RecoveryHelper
	fset     *token.FileSet
}

// NewStmtParser creates a new StmtParser with statement parsing support
func NewStmtParser(t *tokenizer.Tokenizer, fset *token.FileSet) *StmtParser {
	pratt := NewPrattParser(t)
	recovery := NewRecoveryHelper(nil, fset, NewRecoveryContext())

	return &StmtParser{
		PrattParser: pratt,
		recovery:    recovery,
		fset:        fset,
	}
}

// ParseStatement parses a single statement
func (p *StmtParser) ParseStatement() (ast.Stmt, error) {
	// Use recovery wrapper for error handling
	result, recovered := p.recovery.TryParse(func() (interface{}, error) {
		return p.parseStmt(), nil
	}, RecoverToStatement)

	if recovered {
		// Return BadStmt on recovery
		return &ast.BadStmt{From: p.curToken.Pos, To: p.curToken.Pos}, nil
	}

	if result == nil {
		return nil, nil
	}

	return result.(ast.Stmt), nil
}

// parseStmt is the core statement parsing logic
func (p *StmtParser) parseStmt() ast.Stmt {
	// Map lexer tokens to go/token for standard statements
	switch p.curToken.Kind {
	case tokenizer.VAR:
		return p.parseVarStmt()
	case tokenizer.LET:
		return p.parseLetStmt()
	default:
		// For now, try parsing as expression statement
		return p.parseExprStmt()
	}
}

// parseVarStmt parses variable declarations (var x = expr)
func (p *StmtParser) parseVarStmt() ast.Stmt {
	startPos := p.curToken.Pos
	p.nextToken() // consume 'var'

	// Expect identifier
	if !p.curTokenIs(tokenizer.IDENT) {
		p.addError("expected identifier after 'var'")
		return &ast.BadStmt{From: startPos, To: p.curToken.Pos}
	}

	ident := &ast.Ident{
		NamePos: p.curToken.Pos,
		Name:    p.curToken.Lit,
	}
	p.nextToken()

	// Check for assignment
	if p.curTokenIs(tokenizer.ASSIGN) {
		p.nextToken()                     // consume '='
		_ = p.ParseExpression(PrecLowest) // Parse but don't use yet
	}

	// Create declaration statement
	// TODO: Values need conversion from Dingo AST to go/ast
	decl := &ast.GenDecl{
		Tok: token.VAR,
		Specs: []ast.Spec{
			&ast.ValueSpec{
				Names:  []*ast.Ident{ident},
				Values: nil, // Will be filled during transformation
			},
		},
	}

	return &ast.DeclStmt{Decl: decl}
}

// parseLetStmt parses let declarations (let x = expr)
// This is Dingo-specific syntax that will be transformed to Go
func (p *StmtParser) parseLetStmt() ast.Stmt {
	startPos := p.curToken.Pos
	p.nextToken() // consume 'let'

	// Expect identifier
	if !p.curTokenIs(tokenizer.IDENT) {
		p.addError("expected identifier after 'let'")
		return &ast.BadStmt{From: startPos, To: p.curToken.Pos}
	}

	ident := &ast.Ident{
		NamePos: p.curToken.Pos,
		Name:    p.curToken.Lit,
	}
	p.nextToken()

	// Expect assignment
	if !p.curTokenIs(tokenizer.ASSIGN) && !p.curTokenIs(tokenizer.DEFINE) {
		p.addError("expected '=' or ':=' after identifier")
		return &ast.BadStmt{From: startPos, To: p.curToken.Pos}
	}

	isDefine := p.curTokenIs(tokenizer.DEFINE)
	p.nextToken() // consume '=' or ':='

	// Parse value expression
	_ = p.ParseExpression(PrecLowest) // Parse but don't use yet

	// Create assignment statement (let is syntactic sugar for :=)
	// TODO: Convert Dingo AST to go/ast during transformation
	if isDefine {
		return &ast.AssignStmt{
			Lhs: []ast.Expr{ident},
			Tok: token.DEFINE,
			Rhs: nil, // Will be filled during transformation
		}
	}

	// Regular assignment
	return &ast.AssignStmt{
		Lhs: []ast.Expr{ident},
		Tok: token.ASSIGN,
		Rhs: nil, // Will be filled during transformation
	}
}

// parseExprStmt parses expression statements
func (p *StmtParser) parseExprStmt() ast.Stmt {
	_ = p.ParseExpression(PrecLowest) // Parse but don't use yet

	// Check if this is an assignment by looking ahead
	// This is a simplified version - full Go parser would handle more cases
	if p.peekTokenIs(tokenizer.ASSIGN) || p.peekTokenIs(tokenizer.DEFINE) {
		return p.parseAssignment()
	}

	// TODO: Convert Dingo AST to go/ast during transformation
	return &ast.ExprStmt{X: nil} // Placeholder
}

// parseAssignment parses assignment statements (x = y, x := y, etc.)
func (p *StmtParser) parseAssignment() ast.Stmt {
	// Parse left-hand side expressions
	// TODO: Collect LHS expressions properly

	// Check for multiple LHS (x, y = ...)
	for p.peekTokenIs(tokenizer.COMMA) {
		p.nextToken()                     // consume ','
		p.nextToken()                     // move to next expression
		_ = p.ParseExpression(PrecLowest) // Parse but don't use yet
	}

	// Get assignment operator
	if !p.peekTokenIs(tokenizer.ASSIGN) && !p.peekTokenIs(tokenizer.DEFINE) {
		// Not an assignment after all
		return &ast.ExprStmt{X: nil} // Placeholder
	}

	p.nextToken() // move to operator
	tok := token.ASSIGN
	if p.curTokenIs(tokenizer.DEFINE) {
		tok = token.DEFINE
	}
	p.nextToken() // consume operator

	// Parse right-hand side
	_ = p.ParseExpression(PrecLowest) // Parse but don't use yet

	// Check for multiple RHS
	for p.peekTokenIs(tokenizer.COMMA) {
		p.nextToken()                     // consume ','
		p.nextToken()                     // move to next expression
		_ = p.ParseExpression(PrecLowest) // Parse but don't use yet
	}

	// TODO: Convert Dingo AST to go/ast during transformation
	return &ast.AssignStmt{
		Lhs: nil, // Placeholder
		Tok: tok,
		Rhs: nil, // Placeholder
	}
}

// parseReturnStmt parses return statements
func (p *StmtParser) parseReturnStmt() *ast.ReturnStmt {
	pos := p.curToken.Pos
	p.nextToken() // consume 'return'

	// Check if there are return values
	if !p.curTokenIs(tokenizer.SEMICOLON) && !p.curTokenIs(tokenizer.EOF) {
		// Parse first return value
		_ = p.ParseExpression(PrecLowest) // Parse but don't use yet

		// Check for multiple return values
		for p.peekTokenIs(tokenizer.COMMA) {
			p.nextToken()                     // consume ','
			p.nextToken()                     // move to next expression
			_ = p.ParseExpression(PrecLowest) // Parse but don't use yet
		}
	}

	// TODO: Convert Dingo AST to go/ast during transformation
	return &ast.ReturnStmt{
		Return:  pos,
		Results: nil, // Placeholder
	}
}

// parseIfStmt parses if statements
func (p *StmtParser) parseIfStmt() *ast.IfStmt {
	pos := p.curToken.Pos
	p.nextToken() // consume 'if'

	// Parse optional init statement (if x := foo(); x > 0)
	var init ast.Stmt
	var cond ast.Expr

	// Try to parse first expression
	_ = p.ParseExpression(PrecLowest) // Parse but don't use yet

	// Check if this is init statement followed by semicolon
	if p.peekTokenIs(tokenizer.SEMICOLON) {
		p.nextToken() // consume semicolon
		p.nextToken() // move to condition

		// First expression was init statement - wrap in ExprStmt
		// TODO: Convert Dingo AST to go/ast during transformation
		init = &ast.ExprStmt{X: nil} // Placeholder

		// Parse actual condition
		_ = p.ParseExpression(PrecLowest) // Parse but don't use yet
		cond = nil                        // Placeholder
	} else {
		// No init statement, first expression is the condition
		cond = nil // Placeholder
	}

	// Parse body (simplified - would need full block parsing)
	body := &ast.BlockStmt{
		Lbrace: p.curToken.Pos,
		List:   []ast.Stmt{},
		Rbrace: p.curToken.Pos,
	}

	// Parse optional else clause
	var elseStmt ast.Stmt
	// This would check for 'else' keyword when lexer supports it

	return &ast.IfStmt{
		If:   pos,
		Init: init,
		Cond: cond,
		Body: body,
		Else: elseStmt,
	}
}

// parseForStmt parses for statements (all three forms)
func (p *StmtParser) parseForStmt() *ast.ForStmt {
	pos := p.curToken.Pos
	p.nextToken() // consume 'for'

	// For now, parse as basic for loop
	// Full implementation would handle for-range and condition-only forms
	return &ast.ForStmt{
		For: pos,
		Body: &ast.BlockStmt{
			Lbrace: p.curToken.Pos,
			List:   []ast.Stmt{},
			Rbrace: p.curToken.Pos,
		},
	}
}

// parseBlockStmt parses block statements ({ ... })
func (p *StmtParser) parseBlockStmt() *ast.BlockStmt {
	lbrace := p.curToken.Pos
	p.nextToken() // consume '{'

	stmts := []ast.Stmt{}

	// Parse statements until we hit '}'
	// This is simplified - would need proper token support
	for !p.curTokenIs(tokenizer.EOF) {
		stmt := p.parseStmt()
		if stmt != nil {
			stmts = append(stmts, stmt)
		}

		p.nextToken()

		// Break on closing brace (would need lexer support)
		break
	}

	rbrace := p.curToken.Pos

	return &ast.BlockStmt{
		Lbrace: lbrace,
		List:   stmts,
		Rbrace: rbrace,
	}
}

// parseBranchStmt parses branch statements (break, continue, goto, fallthrough)
func (p *StmtParser) parseBranchStmt(tok token.Token) *ast.BranchStmt {
	pos := p.curToken.Pos
	p.nextToken() // consume keyword

	var label *ast.Ident
	if p.curTokenIs(tokenizer.IDENT) {
		label = &ast.Ident{
			NamePos: p.curToken.Pos,
			Name:    p.curToken.Lit,
		}
		p.nextToken()
	}

	return &ast.BranchStmt{
		TokPos: pos,
		Tok:    tok,
		Label:  label,
	}
}

// parseDeferStmt parses defer statements
func (p *StmtParser) parseDeferStmt() *ast.DeferStmt {
	pos := p.curToken.Pos
	p.nextToken() // consume 'defer'

	// Parse the call expression
	call := p.ParseExpression(PrecLowest)

	// Ensure it's a call expression
	// TODO: Type assertion will fail since call is ast.Expr (Dingo), not go/ast.Expr
	// This needs proper AST conversion
	callExpr := &ast.CallExpr{} // Placeholder
	if call == nil {
		p.addError("defer requires a function call")
		return &ast.DeferStmt{
			Defer: pos,
			Call:  &ast.CallExpr{},
		}
	}

	return &ast.DeferStmt{
		Defer: pos,
		Call:  callExpr,
	}
}

// parseGoStmt parses go statements
func (p *StmtParser) parseGoStmt() *ast.GoStmt {
	pos := p.curToken.Pos
	p.nextToken() // consume 'go'

	// Parse the call expression
	call := p.ParseExpression(PrecLowest)

	// Ensure it's a call expression
	// TODO: Type assertion will fail since call is ast.Expr (Dingo), not go/ast.Expr
	// This needs proper AST conversion
	callExpr := &ast.CallExpr{} // Placeholder
	if call == nil {
		p.addError("go requires a function call")
		return &ast.GoStmt{
			Go:   pos,
			Call: &ast.CallExpr{},
		}
	}

	return &ast.GoStmt{
		Go:   pos,
		Call: callExpr,
	}
}

// parseSendStmt parses channel send statements (ch <- value)
func (p *StmtParser) parseSendStmt() *ast.SendStmt {
	arrow := p.curToken.Pos
	p.nextToken() // consume '<-'

	_ = p.ParseExpression(PrecLowest) // Parse but don't use yet

	// TODO: Convert Dingo AST to go/ast during transformation
	return &ast.SendStmt{
		Chan:  nil, // Placeholder
		Arrow: arrow,
		Value: nil, // Placeholder
	}
}

// parseIncDecStmt parses increment/decrement statements (x++, x--)
func (p *StmtParser) parseIncDecStmt(expr ast.Expr) *ast.IncDecStmt {
	tok := token.INC
	if p.curToken.Lit == "--" {
		tok = token.DEC
	}

	return &ast.IncDecStmt{
		X:      expr,
		TokPos: p.curToken.Pos,
		Tok:    tok,
	}
}

// parseLabeledStmt parses labeled statements (label: stmt)
func (p *StmtParser) parseLabeledStmt(label *ast.Ident) *ast.LabeledStmt {
	colon := p.curToken.Pos
	p.nextToken() // consume ':'

	stmt := p.parseStmt()

	return &ast.LabeledStmt{
		Label: label,
		Colon: colon,
		Stmt:  stmt,
	}
}

// parseEmptyStmt parses empty statements (just a semicolon)
func (p *StmtParser) parseEmptyStmt() *ast.EmptyStmt {
	pos := p.curToken.Pos
	p.nextToken() // consume ';'

	return &ast.EmptyStmt{
		Semicolon: pos,
		Implicit:  false,
	}
}

// parseSwitchStmt parses switch statements
func (p *StmtParser) parseSwitchStmt() *ast.SwitchStmt {
	pos := p.curToken.Pos
	p.nextToken() // consume 'switch'

	// Parse optional init and tag expression
	var init ast.Stmt
	var tag ast.Expr

	// Simplified - would need full implementation
	return &ast.SwitchStmt{
		Switch: pos,
		Init:   init,
		Tag:    tag,
		Body: &ast.BlockStmt{
			Lbrace: p.curToken.Pos,
			List:   []ast.Stmt{},
			Rbrace: p.curToken.Pos,
		},
	}
}

// parseSelectStmt parses select statements
func (p *StmtParser) parseSelectStmt() *ast.SelectStmt {
	pos := p.curToken.Pos
	p.nextToken() // consume 'select'

	return &ast.SelectStmt{
		Select: pos,
		Body: &ast.BlockStmt{
			Lbrace: p.curToken.Pos,
			List:   []ast.Stmt{},
			Rbrace: p.curToken.Pos,
		},
	}
}

// Helper methods

// mapLexerToken maps tokenizer.TokenKind to go/token.Token
// This is a helper for converting between the two token systems
func mapLexerToken(lt tokenizer.TokenKind) token.Token {
	switch lt {
	case tokenizer.ASSIGN:
		return token.ASSIGN
	case tokenizer.DEFINE:
		return token.DEFINE
	case tokenizer.COMMA:
		return token.COMMA
	case tokenizer.SEMICOLON:
		return token.SEMICOLON
	case tokenizer.LPAREN:
		return token.LPAREN
	case tokenizer.RPAREN:
		return token.RPAREN
	case tokenizer.LBRACKET:
		return token.LBRACK
	case tokenizer.RBRACKET:
		return token.RBRACK
	default:
		return token.ILLEGAL
	}
}
