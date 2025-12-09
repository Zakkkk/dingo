// Package parser provides enum declaration parsing for the Dingo parser
package parser

import (
	"fmt"
	"go/ast"
	"go/token"

	dingoast "github.com/MadAppGang/dingo/pkg/ast"
	"github.com/MadAppGang/dingo/pkg/tokenizer"
)

// parseEnumDecl parses an enum declaration
// enum Result[T, E] { Ok(T), Err(E) }
// enum Color { Red, Green, Blue, RGB { r: int, g: int, b: int } }
//
// NOTE: Returns ast.BadDecl as a placeholder since Dingo EnumDecl doesn't implement go/ast.Decl.
// The actual EnumDecl will be stored in the parser for later transformation.
// TODO: Update when Dingo AST to Go AST transformation is implemented.
func (p *StmtParser) parseEnumDecl() ast.Decl {
	// Save enum position
	enumPos := p.curToken.Pos
	p.nextToken() // consume 'enum'

	// Parse enum name
	if !p.curTokenIs(tokenizer.IDENT) {
		p.addError(fmt.Sprintf("expected enum name, got %s", p.curToken.Kind))
		return &ast.BadDecl{From: enumPos, To: p.curToken.Pos}
	}

	name := &dingoast.Ident{
		NamePos: p.curToken.Pos,
		Name:    p.curToken.Lit,
	}
	p.nextToken()

	// Parse optional type parameters: <T, E>
	var typeParams *dingoast.TypeParamList
	if p.curTokenIs(tokenizer.LT) {
		typeParams = p.parseTypeParams()
	}

	// Expect opening brace
	if !p.curTokenIs(tokenizer.LBRACE) {
		p.addError(fmt.Sprintf("expected '{' after enum name, got %s", p.curToken.Kind))
		// Attempt recovery - skip to next declaration
		p.synchronizeToDecl()
		return &ast.BadDecl{From: enumPos, To: p.curToken.Pos}
	}

	lbracePos := p.curToken.Pos
	p.nextToken() // consume '{'

	// Parse variants
	variants := p.parseEnumVariants()

	// Expect closing brace
	if !p.curTokenIs(tokenizer.RBRACE) {
		p.addError(fmt.Sprintf("expected '}' after enum variants, got %s", p.curToken.Kind))
		// Return BadDecl on parse error
		return &ast.BadDecl{From: enumPos, To: p.curToken.Pos}
	}

	rbracePos := p.curToken.Pos
	p.nextToken() // consume '}'

	// Store the Dingo EnumDecl for later use by transformation pipeline
	_ = &dingoast.EnumDecl{
		Enum:       enumPos,
		Name:       name,
		TypeParams: typeParams,
		LBrace:     lbracePos,
		Variants:   variants,
		RBrace:     rbracePos,
	}

	// Return BadDecl as placeholder
	// TODO: Replace with proper Dingo AST node storage mechanism
	return &ast.BadDecl{From: enumPos, To: rbracePos + 1}

	// Alternative: Could use ast.GenDecl with TYPE token and specs
	// representing the enum, but that would lose Dingo-specific structure
}

// parseTypeParams parses generic type parameters: <T, E>
func (p *StmtParser) parseTypeParams() *dingoast.TypeParamList {
	openingPos := p.curToken.Pos
	p.nextToken() // consume '<'

	var params []*dingoast.Ident

	// Parse type parameters separated by commas
	for !p.curTokenIs(tokenizer.GT) && !p.curTokenIs(tokenizer.EOF) {
		if !p.curTokenIs(tokenizer.IDENT) {
			p.addError(fmt.Sprintf("expected type parameter name, got %s", p.curToken.Kind))
			break
		}

		param := &dingoast.Ident{
			NamePos: p.curToken.Pos,
			Name:    p.curToken.Lit,
		}
		params = append(params, param)
		p.nextToken()

		// Check for comma or closing '>'
		if p.curTokenIs(tokenizer.COMMA) {
			p.nextToken() // consume ','
		} else if !p.curTokenIs(tokenizer.GT) {
			p.addError(fmt.Sprintf("expected ',' or '>' in type parameters, got %s", p.curToken.Kind))
			break
		}
	}

	if !p.curTokenIs(tokenizer.GT) {
		p.addError("expected '>' to close type parameters")
		return &dingoast.TypeParamList{
			Opening: openingPos,
			Params:  params,
			Closing: 0, // Invalid position
		}
	}

	closingPos := p.curToken.Pos
	p.nextToken() // consume '>'

	return &dingoast.TypeParamList{
		Opening: openingPos,
		Params:  params,
		Closing: closingPos,
	}
}

// parseEnumVariants parses a list of enum variants
func (p *StmtParser) parseEnumVariants() []*dingoast.EnumVariant {
	var variants []*dingoast.EnumVariant

	// Parse variants until we hit closing brace or EOF
	for !p.curTokenIs(tokenizer.RBRACE) && !p.curTokenIs(tokenizer.EOF) {
		// Skip newlines between variants
		for p.curTokenIs(tokenizer.NEWLINE) {
			p.nextToken()
		}

		// Check if we've reached the end
		if p.curTokenIs(tokenizer.RBRACE) || p.curTokenIs(tokenizer.EOF) {
			break
		}

		variant := p.parseEnumVariant()
		if variant != nil {
			variants = append(variants, variant)
		}

		// Handle trailing comma or newline
		if p.curTokenIs(tokenizer.COMMA) {
			p.nextToken() // consume ','
		}

		// Skip newlines after variant
		for p.curTokenIs(tokenizer.NEWLINE) {
			p.nextToken()
		}
	}

	return variants
}

// parseEnumVariant parses a single enum variant
// Supports three forms:
//   - Unit: Red
//   - Tuple: Ok(T)
//   - Struct: RGB { r: int, g: int, b: int }
func (p *StmtParser) parseEnumVariant() *dingoast.EnumVariant {
	if !p.curTokenIs(tokenizer.IDENT) {
		p.addError(fmt.Sprintf("expected variant name, got %s", p.curToken.Kind))
		// Skip to next comma or closing brace
		p.synchronizeToVariant()
		return nil
	}

	name := &dingoast.Ident{
		NamePos: p.curToken.Pos,
		Name:    p.curToken.Lit,
	}
	p.nextToken()

	// Check variant kind
	switch {
	case p.curTokenIs(tokenizer.LPAREN):
		// Tuple variant: Ok(T)
		return p.parseTupleVariant(name)

	case p.curTokenIs(tokenizer.LBRACE):
		// Struct variant: RGB { r: int, g: int, b: int }
		return p.parseStructVariant(name)

	default:
		// Unit variant: Red
		return &dingoast.EnumVariant{
			Name:   name,
			Kind:   dingoast.UnitVariant,
			LDelim: 0,
			Fields: nil,
			RDelim: 0,
		}
	}
}

// parseTupleVariant parses a tuple variant: Ok(T)
func (p *StmtParser) parseTupleVariant(name *dingoast.Ident) *dingoast.EnumVariant {
	lparenPos := p.curToken.Pos
	p.nextToken() // consume '('

	var fields []*dingoast.EnumField

	// Parse tuple fields (types only, no names)
	for !p.curTokenIs(tokenizer.RPAREN) && !p.curTokenIs(tokenizer.EOF) {
		// Parse type
		typeExpr := p.parseTypeExpr()
		if typeExpr == nil {
			p.addError("expected type in tuple variant")
			break
		}

		fields = append(fields, &dingoast.EnumField{
			Name:  nil, // No name for tuple fields
			Colon: 0,
			Type:  typeExpr,
		})

		// Check for comma or closing paren
		if p.curTokenIs(tokenizer.COMMA) {
			p.nextToken() // consume ','
		} else if !p.curTokenIs(tokenizer.RPAREN) {
			p.addError(fmt.Sprintf("expected ',' or ')' in tuple variant, got %s", p.curToken.Kind))
			break
		}
	}

	if !p.curTokenIs(tokenizer.RPAREN) {
		p.addError("expected ')' to close tuple variant")
		return &dingoast.EnumVariant{
			Name:   name,
			Kind:   dingoast.TupleVariant,
			LDelim: lparenPos,
			Fields: fields,
			RDelim: 0, // Invalid position
		}
	}

	rparenPos := p.curToken.Pos
	p.nextToken() // consume ')'

	return &dingoast.EnumVariant{
		Name:   name,
		Kind:   dingoast.TupleVariant,
		LDelim: lparenPos,
		Fields: fields,
		RDelim: rparenPos,
	}
}

// parseStructVariant parses a struct variant: RGB { r: int, g: int, b: int }
func (p *StmtParser) parseStructVariant(name *dingoast.Ident) *dingoast.EnumVariant {
	lbracePos := p.curToken.Pos
	p.nextToken() // consume '{'

	var fields []*dingoast.EnumField

	// Parse struct fields (name: type pairs)
	for !p.curTokenIs(tokenizer.RBRACE) && !p.curTokenIs(tokenizer.EOF) {
		// Skip newlines
		for p.curTokenIs(tokenizer.NEWLINE) {
			p.nextToken()
		}

		// Check if we've reached the end
		if p.curTokenIs(tokenizer.RBRACE) || p.curTokenIs(tokenizer.EOF) {
			break
		}

		// Parse field name
		if !p.curTokenIs(tokenizer.IDENT) {
			p.addError(fmt.Sprintf("expected field name, got %s", p.curToken.Kind))
			break
		}

		fieldName := &dingoast.Ident{
			NamePos: p.curToken.Pos,
			Name:    p.curToken.Lit,
		}
		p.nextToken()

		// Expect colon
		if !p.curTokenIs(tokenizer.COLON) {
			p.addError(fmt.Sprintf("expected ':' after field name, got %s", p.curToken.Kind))
			break
		}

		colonPos := p.curToken.Pos
		p.nextToken() // consume ':'

		// Parse field type
		typeExpr := p.parseTypeExpr()
		if typeExpr == nil {
			p.addError("expected field type")
			break
		}

		fields = append(fields, &dingoast.EnumField{
			Name:  fieldName,
			Colon: colonPos,
			Type:  typeExpr,
		})

		// Check for comma or closing brace
		if p.curTokenIs(tokenizer.COMMA) {
			p.nextToken() // consume ','
		} else if !p.curTokenIs(tokenizer.RBRACE) {
			p.addError(fmt.Sprintf("expected ',' or '}' in struct variant, got %s", p.curToken.Kind))
			break
		}
	}

	if !p.curTokenIs(tokenizer.RBRACE) {
		p.addError("expected '}' to close struct variant")
		return &dingoast.EnumVariant{
			Name:   name,
			Kind:   dingoast.StructVariant,
			LDelim: lbracePos,
			Fields: fields,
			RDelim: 0, // Invalid position
		}
	}

	rbracePos := p.curToken.Pos
	p.nextToken() // consume '}'

	return &dingoast.EnumVariant{
		Name:   name,
		Kind:   dingoast.StructVariant,
		LDelim: lbracePos,
		Fields: fields,
		RDelim: rbracePos,
	}
}

// parseTypeExpr parses a type expression
// This handles:
//   - Simple types: int, string, T
//   - Generic types: Option[T], Result[T, E]
//   - Slice types: []int
//   - Pointer types: *int (if supported)
func (p *StmtParser) parseTypeExpr() *dingoast.TypeExpr {
	startPos := p.curToken.Pos

	// Handle slice types: []T
	if p.curTokenIs(tokenizer.LBRACKET) {
		p.nextToken() // consume '['

		if !p.curTokenIs(tokenizer.RBRACKET) {
			p.addError("expected ']' for slice type")
			return nil
		}
		p.nextToken() // consume ']'

		// Parse element type
		elemType := p.parseTypeExpr()
		if elemType == nil {
			return nil
		}

		return &dingoast.TypeExpr{
			StartPos: startPos,
			EndPos:   elemType.EndPos,
			Text:     "[]" + elemType.Text,
		}
	}

	// Parse base type name
	if !p.curTokenIs(tokenizer.IDENT) {
		p.addError(fmt.Sprintf("expected type name, got %s", p.curToken.Kind))
		return nil
	}

	typeName := p.curToken.Lit
	endPos := p.curToken.Pos + token.Pos(len(typeName))
	p.nextToken()

	// Check for generic type arguments: Option[T]
	if p.curTokenIs(tokenizer.LT) {
		p.nextToken() // consume '<'

		var typeArgs []string

		// Parse type arguments
		for !p.curTokenIs(tokenizer.GT) && !p.curTokenIs(tokenizer.EOF) {
			typeArg := p.parseTypeExpr()
			if typeArg == nil {
				break
			}

			typeArgs = append(typeArgs, typeArg.Text)

			if p.curTokenIs(tokenizer.COMMA) {
				p.nextToken() // consume ','
			} else if !p.curTokenIs(tokenizer.GT) {
				p.addError(fmt.Sprintf("expected ',' or '>' in type arguments, got %s", p.curToken.Kind))
				break
			}
		}

		if !p.curTokenIs(tokenizer.GT) {
			p.addError("expected '>' to close type arguments")
			return &dingoast.TypeExpr{
				StartPos: startPos,
				EndPos:   endPos,
				Text:     typeName, // Return base type on error
			}
		}

		endPos = p.curToken.Pos + 1
		p.nextToken() // consume '>'

		// Build generic type text: Result[T, E]
		typeText := typeName + "<"
		for i, arg := range typeArgs {
			if i > 0 {
				typeText += ", "
			}
			typeText += arg
		}
		typeText += ">"

		return &dingoast.TypeExpr{
			StartPos: startPos,
			EndPos:   endPos,
			Text:     typeText,
		}
	}

	// Simple type
	return &dingoast.TypeExpr{
		StartPos: startPos,
		EndPos:   endPos,
		Text:     typeName,
	}
}

// synchronizeToVariant skips tokens until a variant separator or closing brace is found
func (p *StmtParser) synchronizeToVariant() {
	// Skip at least one token to make progress
	p.nextToken()

	for !p.curTokenIs(tokenizer.EOF) {
		// Stop at comma (next variant) or closing brace (end of enum)
		if p.curTokenIs(tokenizer.COMMA) || p.curTokenIs(tokenizer.RBRACE) {
			return
		}
		p.nextToken()
	}
}
