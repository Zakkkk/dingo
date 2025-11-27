package preprocessor

import (
	"fmt"
	"strings"

	"github.com/MadAppGang/dingo/pkg/ast"
	"github.com/MadAppGang/dingo/pkg/lexer"
)

// parseLetDeclaration parses a let declaration starting from LET token
// Grammar: 'let' ident_list [':' type] ['=' expr]
func parseLetDeclaration(l *lexer.Lexer) (*ast.LetDecl, error) {
	// Current token should be LET
	tok := l.NextToken()
	if tok.Type != lexer.LET {
		return nil, fmt.Errorf("expected LET token, got %s", tok.Type)
	}

	decl := &ast.LetDecl{
		LetPos: tok.Pos,
	}

	// Parse identifier list: a, b, c
	names, err := parseIdentList(l)
	if err != nil {
		return nil, fmt.Errorf("parseLetDeclaration: %w", err)
	}
	decl.Names = names

	// Check for type annotation: ": Type"
	typeAnnot, err := parseTypeAnnotation(l)
	if err != nil {
		return nil, fmt.Errorf("parseLetDeclaration: %w", err)
	}
	decl.TypeAnnot = typeAnnot

	// Check for initialization: "= expr"
	tok = l.PeekToken()
	if tok.Type == lexer.ASSIGN {
		l.NextToken() // consume '='
		decl.HasInit = true

		// Parse value expression
		value, err := parseValueExpr(l)
		if err != nil {
			return nil, fmt.Errorf("parseLetDeclaration: %w", err)
		}
		decl.Value = value
	}

	return decl, nil
}

// parseIdentList parses comma-separated identifiers: a, b, c
func parseIdentList(l *lexer.Lexer) ([]string, error) {
	var names []string

	// First identifier is required
	tok := l.NextToken()
	if tok.Type != lexer.IDENT {
		return nil, fmt.Errorf("expected identifier, got %s", tok.Type)
	}
	names = append(names, tok.Literal)

	// Check for additional identifiers
	for {
		tok = l.PeekToken()
		if tok.Type != lexer.COMMA {
			break
		}
		l.NextToken() // consume comma

		// Next must be identifier
		tok = l.NextToken()
		if tok.Type != lexer.IDENT {
			return nil, fmt.Errorf("expected identifier after comma, got %s", tok.Type)
		}
		names = append(names, tok.Literal)
	}

	return names, nil
}

// parseTypeAnnotation parses ": Type" including the colon
// Handles complex types like Option<Result<int, Error>>, []int, func(int) string
// Returns empty string if no colon found
func parseTypeAnnotation(l *lexer.Lexer) (string, error) {
	tok := l.PeekToken()
	if tok.Type != lexer.COLON {
		return "", nil
	}
	l.NextToken() // consume colon

	var typeBuilder strings.Builder
	typeBuilder.WriteString(": ")

	// Read tokens until we hit '=' or newline/semicolon
	// Track bracket nesting to handle complex types
	angleDepth := 0
	parenDepth := 0
	bracketDepth := 0

	for {
		tok = l.PeekToken()

		// Stop conditions
		if tok.Type == lexer.EOF {
			break
		}
		if tok.Type == lexer.NEWLINE || tok.Type == lexer.SEMICOLON {
			break
		}
		if tok.Type == lexer.ASSIGN && angleDepth == 0 && parenDepth == 0 && bracketDepth == 0 {
			break
		}

		// Consume token
		tok = l.NextToken()

		// Track bracket depths
		switch tok.Type {
		case lexer.LANGLE:
			angleDepth++
		case lexer.RANGLE:
			angleDepth--
		case lexer.LPAREN:
			parenDepth++
		case lexer.RPAREN:
			parenDepth--
		case lexer.LBRACKET:
			bracketDepth++
		case lexer.RBRACKET:
			bracketDepth--
		}

		// Add token to type string
		if typeBuilder.Len() > 2 { // More than ": "
			typeBuilder.WriteString(" ")
		}
		typeBuilder.WriteString(tok.Literal)
	}

	return strings.TrimSpace(typeBuilder.String()), nil
}

// parseValueExpr reads everything after = until newline/semicolon
// Respects bracket nesting: (), [], {}, <>
func parseValueExpr(l *lexer.Lexer) (string, error) {
	var valueBuilder strings.Builder

	// Track bracket nesting
	angleDepth := 0
	parenDepth := 0
	bracketDepth := 0
	braceDepth := 0

	lastTokenType := lexer.ILLEGAL

	for {
		tok := l.PeekToken()

		// Stop conditions
		if tok.Type == lexer.EOF {
			break
		}
		if tok.Type == lexer.NEWLINE || tok.Type == lexer.SEMICOLON {
			if angleDepth == 0 && parenDepth == 0 && bracketDepth == 0 && braceDepth == 0 {
				break
			}
		}

		// Consume token
		tok = l.NextToken()

		// Track bracket depths
		switch tok.Type {
		case lexer.LANGLE:
			angleDepth++
		case lexer.RANGLE:
			angleDepth--
		case lexer.LPAREN:
			parenDepth++
		case lexer.RPAREN:
			parenDepth--
		case lexer.LBRACKET:
			bracketDepth++
		case lexer.RBRACKET:
			bracketDepth--
		}

		// Handle braces (not in lexer yet, but handle string literals)
		if tok.Literal == "{" {
			braceDepth++
		} else if tok.Literal == "}" {
			braceDepth--
		}

		// Add token to value string with smart spacing
		if valueBuilder.Len() > 0 {
			// Don't add space before closing brackets or after opening brackets
			needSpace := true
			if tok.Type == lexer.RPAREN || tok.Type == lexer.RBRACKET || tok.Type == lexer.RANGLE {
				needSpace = false
			}
			if lastTokenType == lexer.LPAREN || lastTokenType == lexer.LBRACKET || lastTokenType == lexer.LANGLE {
				needSpace = false
			}
			// Don't add space before comma
			if tok.Type == lexer.COMMA {
				needSpace = false
			}

			if needSpace {
				valueBuilder.WriteString(" ")
			}
		}

		// Handle string literals - need to add quotes back with original quote type
		if tok.Type == lexer.STRING {
			if tok.QuoteType == '`' {
				valueBuilder.WriteString("`")
				valueBuilder.WriteString(tok.Literal)
				valueBuilder.WriteString("`")
			} else {
				valueBuilder.WriteString(`"`)
				valueBuilder.WriteString(tok.Literal)
				valueBuilder.WriteString(`"`)
			}
		} else {
			valueBuilder.WriteString(tok.Literal)
		}

		lastTokenType = tok.Type
	}

	return strings.TrimSpace(valueBuilder.String()), nil
}
