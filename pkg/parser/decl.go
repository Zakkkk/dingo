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
	// Skip leading comments (file-level documentation)
	p.skipComments()

	file := &ast.File{
		Name:  p.parsePackageClause(),
		Decls: []ast.Decl{},
	}

	// Parse imports
	for p.curTokenIs(tokenizer.IMPORT) {
		p.skipComments()
		if p.curTokenIs(tokenizer.IMPORT) {
			decl := p.parseImportDecl()
			if decl != nil {
				file.Decls = append(file.Decls, decl)
			}
		}
		p.skipComments()
	}

	// Parse top-level declarations
	for !p.curTokenIs(tokenizer.EOF) {
		// Skip comments between declarations
		p.skipComments()

		if p.curTokenIs(tokenizer.EOF) {
			break
		}

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
	// Skip leading whitespace/comments that may have been missed
	p.skipComments()

	// Check for package keyword
	if !p.curTokenIs(tokenizer.PACKAGE) {
		p.addError("expected 'package' keyword at start of file")
		return &ast.Ident{Name: "main"} // Default fallback
	}
	p.nextToken() // consume 'package'

	// Expect package name (identifier)
	if !p.curTokenIs(tokenizer.IDENT) {
		p.addError("expected package name after 'package' keyword")
		return &ast.Ident{Name: "main"} // Default fallback
	}

	packageName := &ast.Ident{
		Name:    p.curToken.Lit,
		NamePos: p.curToken.Pos,
	}
	p.nextToken() // consume package name

	// Skip optional semicolon or newline after package declaration
	for p.curTokenIs(tokenizer.SEMICOLON) || p.curTokenIs(tokenizer.NEWLINE) {
		p.nextToken()
	}

	return packageName
}

// parseDecl parses a top-level declaration
func (p *StmtParser) parseDecl() ast.Decl {
	switch p.curToken.Kind {
	case tokenizer.IMPORT:
		return p.parseImportDecl()
	case tokenizer.VAR:
		return p.parseVarDecl()
	case tokenizer.CONST:
		return p.parseConstDecl()
	case tokenizer.TYPE:
		return p.parseTypeDecl()
	case tokenizer.FUNC:
		return p.parseFuncDecl()
	case tokenizer.ENUM:
		return p.parseEnumDecl()
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
		p.nextToken()                     // consume '='
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
// This handles basic types, generic types like Result[T,E], and composite types
func (p *StmtParser) parseType() ast.Expr {
	// Handle composite types: map[], []slice, chan, func, interface
	switch p.curToken.Kind {
	case tokenizer.MAP:
		return p.parseMapType()
	case tokenizer.LBRACKET:
		return p.parseSliceOrArrayType()
	case tokenizer.CHAN:
		return p.parseChanType()
	case tokenizer.FUNC:
		return p.parseFuncType()
	case tokenizer.INTERFACE:
		return p.parseInterfaceType()
	case tokenizer.STAR:
		// Pointer type: *T
		p.nextToken() // consume '*'
		elemType := p.parseType()
		return &ast.StarExpr{
			X: elemType,
		}
	}

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

	// Check for generic type arguments (Result[T, E])
	if p.curTokenIs(tokenizer.LT) {
		return p.parseGenericType(typeName)
	}

	return typeName
}

// parseGenericType parses a generic type with type arguments (Result[T, E])
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

// parseMapType parses map[K]V type
func (p *StmtParser) parseMapType() ast.Expr {
	mapPos := p.curToken.Pos
	p.nextToken() // consume 'map'

	if !p.curTokenIs(tokenizer.LBRACKET) {
		p.addError("expected '[' after 'map'")
		return nil
	}
	p.nextToken() // consume '['

	// Parse key type
	keyType := p.parseType()

	if !p.curTokenIs(tokenizer.RBRACKET) {
		p.addError("expected ']' after map key type")
		return nil
	}
	p.nextToken() // consume ']'

	// Parse value type
	valueType := p.parseType()

	return &ast.MapType{
		Map:   mapPos,
		Key:   keyType,
		Value: valueType,
	}
}

// parseSliceOrArrayType parses []T or [N]T type
func (p *StmtParser) parseSliceOrArrayType() ast.Expr {
	lbrack := p.curToken.Pos
	p.nextToken() // consume '['

	// Check if it's array type [N]T or slice type []T
	var length ast.Expr
	if !p.curTokenIs(tokenizer.RBRACKET) {
		// Array type with length - for now, just skip to ]
		// Proper implementation would convert Dingo AST to go/ast
		for !p.curTokenIs(tokenizer.RBRACKET) && !p.curTokenIs(tokenizer.EOF) {
			p.nextToken()
		}
	}

	if !p.curTokenIs(tokenizer.RBRACKET) {
		p.addError("expected ']' in slice/array type")
		return nil
	}
	p.nextToken() // consume ']'

	// Parse element type
	elemType := p.parseType()

	return &ast.ArrayType{
		Lbrack: lbrack,
		Len:    length, // nil for slice, expr for array
		Elt:    elemType,
	}
}

// parseChanType parses chan T, chan<- T, or <-chan T
func (p *StmtParser) parseChanType() ast.Expr {
	chanPos := p.curToken.Pos
	p.nextToken() // consume 'chan'

	dir := ast.SEND | ast.RECV // bidirectional by default

	// Check for chan<- (send-only)
	if p.curTokenIs(tokenizer.CHAN_ARROW) {
		dir = ast.SEND
		p.nextToken() // consume '<-'
	}

	// Parse value type
	valueType := p.parseType()

	return &ast.ChanType{
		Begin: chanPos,
		Dir:   dir,
		Value: valueType,
	}
}

// parseFuncType parses func(...) ... type
func (p *StmtParser) parseFuncType() ast.Expr {
	funcPos := p.curToken.Pos
	p.nextToken() // consume 'func'

	// For now, skip function signature completely
	// Just consume until we've balanced parentheses
	depth := 0
	for !p.curTokenIs(tokenizer.EOF) {
		if p.curTokenIs(tokenizer.LPAREN) {
			depth++
		} else if p.curTokenIs(tokenizer.RPAREN) {
			depth--
			if depth == 0 {
				p.nextToken() // consume final ')'
				break
			}
		}
		p.nextToken()
	}

	// Create minimal FuncType (proper parsing would need parameter lists)
	return &ast.FuncType{
		Func:   funcPos,
		Params: &ast.FieldList{},
	}
}

// parseInterfaceType parses interface{...} type
func (p *StmtParser) parseInterfaceType() ast.Expr {
	interfacePos := p.curToken.Pos
	p.nextToken() // consume 'interface'

	if !p.curTokenIs(tokenizer.LBRACE) {
		p.addError("expected '{' after 'interface'")
		return nil
	}
	lbrace := p.curToken.Pos
	p.nextToken() // consume '{'

	// Parse methods/embedded interfaces until '}'
	var methods []*ast.Field
	for !p.curTokenIs(tokenizer.RBRACE) && !p.curTokenIs(tokenizer.EOF) {
		// For now, just skip interface body
		// Proper implementation would parse method signatures and embedded types
		if p.curTokenIs(tokenizer.RBRACE) {
			break
		}
		p.nextToken()
	}

	if !p.curTokenIs(tokenizer.RBRACE) {
		p.addError("expected '}' to close interface")
		return nil
	}
	rbrace := p.curToken.Pos
	p.nextToken() // consume '}'

	return &ast.InterfaceType{
		Interface: interfacePos,
		Methods: &ast.FieldList{
			Opening: lbrace,
			List:    methods, // Empty for interface{}
			Closing: rbrace,
		},
	}
}

// parseImportDecl parses an import declaration
func (p *StmtParser) parseImportDecl() *ast.GenDecl {
	importPos := p.curToken.Pos
	p.nextToken() // consume 'import'

	// For now, just skip the import statement completely
	// The transpiler will preserve imports from the original Go code
	// We just need to not error on them during parsing

	// Check for grouped imports: import ( ... )
	if p.curTokenIs(tokenizer.LPAREN) {
		p.nextToken() // consume '('

		// Skip until we find the closing paren
		depth := 1
		for !p.curTokenIs(tokenizer.EOF) && depth > 0 {
			if p.curTokenIs(tokenizer.LPAREN) {
				depth++
			} else if p.curTokenIs(tokenizer.RPAREN) {
				depth--
				if depth == 0 {
					p.nextToken() // consume ')'
					break
				}
			}
			p.nextToken()
		}
	} else {
		// Single import: import "foo"
		// Skip until newline or semicolon
		for !p.curTokenIs(tokenizer.NEWLINE) && !p.curTokenIs(tokenizer.SEMICOLON) && !p.curTokenIs(tokenizer.EOF) {
			p.nextToken()
		}
	}

	// Skip the newline/semicolon
	if p.curTokenIs(tokenizer.NEWLINE) || p.curTokenIs(tokenizer.SEMICOLON) {
		p.nextToken()
	}

	// Return empty import declaration (will be ignored)
	return &ast.GenDecl{
		Tok:    token.IMPORT,
		TokPos: importPos,
		Specs:  []ast.Spec{},
	}
}

// parseConstDecl parses a const declaration
func (p *StmtParser) parseConstDecl() *ast.GenDecl {
	constPos := p.curToken.Pos
	p.nextToken() // consume 'const'

	// For now, skip const declarations (similar to import)
	for !p.curTokenIs(tokenizer.NEWLINE) && !p.curTokenIs(tokenizer.SEMICOLON) && !p.curTokenIs(tokenizer.EOF) {
		p.nextToken()
	}

	if p.curTokenIs(tokenizer.NEWLINE) || p.curTokenIs(tokenizer.SEMICOLON) {
		p.nextToken()
	}

	return &ast.GenDecl{
		Tok:    token.CONST,
		TokPos: constPos,
		Specs:  []ast.Spec{},
	}
}

// parseTypeDecl parses a type declaration (type UserSettings struct { ... })
func (p *StmtParser) parseTypeDecl() *ast.GenDecl {
	typePos := p.curToken.Pos
	p.nextToken() // consume 'type'

	// For now, skip type declarations completely
	// We need to skip until we find a balanced set of braces or reach end
	depth := 0
	for !p.curTokenIs(tokenizer.EOF) {
		if p.curTokenIs(tokenizer.LBRACE) {
			depth++
		} else if p.curTokenIs(tokenizer.RBRACE) {
			depth--
			if depth <= 0 {
				p.nextToken() // consume closing brace
				break
			}
		} else if depth == 0 && (p.curTokenIs(tokenizer.NEWLINE) || p.curTokenIs(tokenizer.SEMICOLON)) {
			p.nextToken()
			break
		}
		p.nextToken()
	}

	return &ast.GenDecl{
		Tok:    token.TYPE,
		TokPos: typePos,
		Specs:  []ast.Spec{},
	}
}

// parseFuncDecl parses a function declaration
func (p *StmtParser) parseFuncDecl() *ast.FuncDecl {
	funcPos := p.curToken.Pos
	p.nextToken() // consume 'func'

	// Parse optional receiver
	if p.curTokenIs(tokenizer.LPAREN) {
		p.skipBalanced(tokenizer.LPAREN, tokenizer.RPAREN)
	}

	// Parse function name
	var funcName *ast.Ident
	if p.curTokenIs(tokenizer.IDENT) {
		funcName = &ast.Ident{Name: p.curToken.Lit, NamePos: p.curToken.Pos}
		p.nextToken()
	} else {
		funcName = &ast.Ident{Name: "placeholder", NamePos: funcPos}
	}

	// Parse type parameters [T, E]
	if p.curTokenIs(tokenizer.LBRACKET) {
		p.skipBalanced(tokenizer.LBRACKET, tokenizer.RBRACKET)
	}

	// Parse parameters (required)
	if p.curTokenIs(tokenizer.LPAREN) {
		p.skipBalanced(tokenizer.LPAREN, tokenizer.RPAREN)
	}

	// Parse return type(s) - this is where interface{} can appear
	// Return types continue until we hit the function body opening brace
	// We need to carefully handle interface{} which contains braces
	p.skipFuncReturnType()

	// Now we should be at the function body opening brace
	if p.curTokenIs(tokenizer.LBRACE) {
		p.skipBalanced(tokenizer.LBRACE, tokenizer.RBRACE)
	}

	return &ast.FuncDecl{
		Name: funcName,
		Type: &ast.FuncType{Func: funcPos, Params: &ast.FieldList{}},
		Body: &ast.BlockStmt{},
	}
}

// skipBalanced skips tokens until matching close delimiter is found
func (p *StmtParser) skipBalanced(open, close tokenizer.TokenKind) {
	if !p.curTokenIs(open) {
		return
	}
	depth := 1
	p.nextToken() // consume opening token
	for depth > 0 && !p.curTokenIs(tokenizer.EOF) {
		if p.curTokenIs(open) {
			depth++
		} else if p.curTokenIs(close) {
			depth--
		}
		p.nextToken()
	}
}

// skipFuncReturnType skips the return type portion of a function signature
// Handles: no return, single type, (multi return), and interface{}/struct{}
func (p *StmtParser) skipFuncReturnType() {
	// If immediately at LBRACE, no return type
	if p.curTokenIs(tokenizer.LBRACE) {
		return
	}

	// Handle parenthesized return types: (int, error)
	if p.curTokenIs(tokenizer.LPAREN) {
		p.skipBalanced(tokenizer.LPAREN, tokenizer.RPAREN)
		return
	}

	// Handle single return type which may include:
	// - simple: int, string, error
	// - pointer: *Foo
	// - slice: []byte
	// - map: map[string]interface{}
	// - interface: interface{}
	// - struct: struct{}
	// - chan: chan int
	// - func: func(x int) int
	// - generic: Result[T, E]
	for !p.curTokenIs(tokenizer.EOF) && !p.curTokenIs(tokenizer.LBRACE) {
		switch p.curToken.Kind {
		case tokenizer.INTERFACE, tokenizer.STRUCT:
			// interface{} or struct{} - consume the keyword and the {}
			p.nextToken()
			if p.curTokenIs(tokenizer.LBRACE) {
				p.skipBalanced(tokenizer.LBRACE, tokenizer.RBRACE)
			}
		case tokenizer.LBRACKET:
			// Generic type args or slice/array type
			p.skipBalanced(tokenizer.LBRACKET, tokenizer.RBRACKET)
		case tokenizer.LPAREN:
			// Function type parameters
			p.skipBalanced(tokenizer.LPAREN, tokenizer.RPAREN)
		case tokenizer.FUNC:
			// func type - skip the entire func signature
			p.nextToken()
			if p.curTokenIs(tokenizer.LPAREN) {
				p.skipBalanced(tokenizer.LPAREN, tokenizer.RPAREN)
			}
			// Recursively handle return type of func type
			p.skipFuncReturnType()
		default:
			p.nextToken()
		}
	}
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
		case tokenizer.VAR, tokenizer.ENUM, tokenizer.IMPORT, tokenizer.CONST, tokenizer.TYPE, tokenizer.FUNC:
			return
		}
		p.nextToken()
	}
}

// skipComments skips all consecutive comment and newline tokens
func (p *StmtParser) skipComments() {
	for p.curTokenIs(tokenizer.COMMENT) || p.curTokenIs(tokenizer.NEWLINE) {
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
