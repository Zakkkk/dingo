// Package parser provides declaration parsing for the Dingo parser
package parser

import (
	"fmt"
	"go/ast"
	"go/token"

	"github.com/MadAppGang/dingo/pkg/tokenizer"
)

// NOTE: StmtParser struct is defined in stmt.go
// This file provides declaration parsing methods for StmtParser

// ParseFile parses a complete Dingo source file
func (p *StmtParser) ParseFile() (*ast.File, error) {
	file := &ast.File{
		Name:  p.parsePackageClause(),
		Decls: []ast.Decl{},
	}

	// Parse imports if any
	// TODO: Implement parseImportDecl when needed

	// Parse top-level declarations
	for !p.curTokenIs(tokenizer.EOF) {
		decl := p.parseDecl()
		if decl != nil {
			file.Decls = append(file.Decls, decl)
		}

		// Skip semicolons and newlines between declarations
		for p.curTokenIs(tokenizer.SEMICOLON) || p.curTokenIs(tokenizer.NEWLINE) {
			p.nextToken()
		}
	}

	// Return errors if any were collected
	if len(p.errors) > 0 {
		return file, ErrorList(p.errors)
	}

	return file, nil
}

// parsePackageClause parses the package declaration
func (p *StmtParser) parsePackageClause() *ast.Ident {
	// TODO: Implement proper package parsing
	// For now, return a default package name
	return &ast.Ident{Name: "main"}
}

// parseDecl parses a top-level declaration
func (p *StmtParser) parseDecl() ast.Decl {
	switch p.curToken.Kind {
	case tokenizer.VAR:
		return p.parseVarDecl()
	case tokenizer.ENUM:
		return p.parseEnumDecl()
	// TODO: Add other declaration types:
	// case tokenizer.CONST:
	//     return p.parseConstDecl()
	// case tokenizer.TYPE:
	//     return p.parseTypeDecl()
	// case tokenizer.FUNC:
	//     return p.parseFuncDecl()
	default:
		// Unknown declaration - create BadDecl and synchronize
		p.addError(fmt.Sprintf("expected declaration, got %s", p.curToken.Kind))
		return p.createBadDecl()
	}
}

// parseVarDecl parses a var declaration (var x: int = 10)
func (p *StmtParser) parseVarDecl() *ast.GenDecl {
	varPos := p.curToken.Pos
	p.nextToken() // consume 'var'

	var specs []ast.Spec

	// Check if this is a grouped declaration (var ( ... ))
	// For now, just parse single variable
	spec := p.parseVarSpec()
	if spec != nil {
		specs = append(specs, spec)
	}

	return &ast.GenDecl{
		Tok:    token.VAR,
		TokPos: varPos,
		Specs:  specs,
	}
}

// parseVarSpec parses a variable specification (x: int = 10)
func (p *StmtParser) parseVarSpec() *ast.ValueSpec {
	if !p.curTokenIs(tokenizer.IDENT) {
		p.addError(fmt.Sprintf("expected identifier, got %s", p.curToken.Kind))
		return nil
	}

	// Parse variable name
	name := &ast.Ident{
		Name:    p.curToken.Lit,
		NamePos: p.curToken.Pos,
	}
	p.nextToken()

	var typ ast.Expr
	var values []ast.Expr

	// Check for type annotation (: Type)
	if p.curTokenIs(tokenizer.COLON) {
		p.nextToken() // consume ':'
		typ = p.parseType()
	}

	// Check for initialization (= expr)
	if p.curTokenIs(tokenizer.ASSIGN) {
		p.nextToken() // consume '='
		_ = p.ParseExpression(PrecLowest) // Parse but don't use yet
		// TODO: Convert Dingo AST to go/ast during transformation
	}

	return &ast.ValueSpec{
		Names:  []*ast.Ident{name},
		Type:   typ,
		Values: values, // Empty for now - will be filled during transformation
	}
}

// parseType parses a type expression
// This handles basic types, generic types like Result<T,E>, and composite types
func (p *StmtParser) parseType() ast.Expr {
	if !p.curTokenIs(tokenizer.IDENT) {
		p.addError(fmt.Sprintf("expected type, got %s", p.curToken.Kind))
		return nil
	}

	// Parse base type name
	typeName := &ast.Ident{
		Name:    p.curToken.Lit,
		NamePos: p.curToken.Pos,
	}
	p.nextToken()

	// Check for generic type arguments (Result<T, E>)
	if p.curTokenIs(tokenizer.LT) {
		return p.parseGenericType(typeName)
	}

	return typeName
}

// parseGenericType parses a generic type with type arguments (Result<T, E>)
func (p *StmtParser) parseGenericType(baseName *ast.Ident) ast.Expr {
	p.nextToken() // consume '<'

	var typeArgs []ast.Expr

	// Parse type arguments separated by commas
	for !p.curTokenIs(tokenizer.GT) && !p.curTokenIs(tokenizer.EOF) {
		typeArg := p.parseType()
		if typeArg != nil {
			typeArgs = append(typeArgs, typeArg)
		}

		if p.curTokenIs(tokenizer.COMMA) {
			p.nextToken() // consume ','
		} else if !p.curTokenIs(tokenizer.GT) {
			p.addError(fmt.Sprintf("expected ',' or '>', got %s", p.curToken.Kind))
			break
		}
	}

	if !p.curTokenIs(tokenizer.GT) {
		p.addError("expected '>' to close generic type arguments")
		return baseName
	}
	p.nextToken() // consume '>'

	// Create IndexListExpr for generic instantiation (Go 1.18+)
	return &ast.IndexListExpr{
		X:       baseName,
		Lbrack:  baseName.NamePos, // Approximate position
		Indices: typeArgs,
		Rbrack:  p.curToken.Pos,
	}
}

// parseFuncDecl parses a function declaration with Dingo type annotations
// func foo(x: int, y: string) int { ... }
func (p *StmtParser) parseFuncDecl() *ast.FuncDecl {
	// TODO: Implement when FUNC token is added to lexer
	// This would parse:
	// 1. func keyword
	// 2. function name
	// 3. type parameters (if any)
	// 4. parameters with Dingo-style type annotations (param: Type)
	// 5. return type
	// 6. function body
	return nil
}

// parseEnumDecl is implemented in enum.go

// createBadDecl creates a BadDecl node and synchronizes to next declaration
func (p *StmtParser) createBadDecl() *ast.BadDecl {
	from := p.curToken.Pos

	// Synchronize to next declaration using recovery
	p.synchronizeToDecl()

	return &ast.BadDecl{
		From: from,
		To:   p.curToken.Pos,
	}
}

// synchronizeToDecl skips tokens until a declaration keyword is found
func (p *StmtParser) synchronizeToDecl() {
	// Skip at least one token to make progress
	p.nextToken()

	for !p.curTokenIs(tokenizer.EOF) {
		// Check for declaration keywords
		switch p.curToken.Kind {
		case tokenizer.VAR, tokenizer.ENUM:
			return
		// TODO: Add other declaration keywords when added to lexer
		// case tokenizer.CONST, tokenizer.TYPE, tokenizer.FUNC:
		//     return
		}
		p.nextToken()
	}
}

// addError adds a parse error to the error list (delegates to PrattParser)
func (p *StmtParser) addError(msg string) {
	p.PrattParser.errors = append(p.PrattParser.errors, ParseError{
		Pos:     p.curToken.Pos,
		Line:    p.curToken.Line,
		Column:  p.curToken.Column,
		Message: msg,
	})
}
