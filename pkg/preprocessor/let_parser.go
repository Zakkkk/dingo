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
	lastTokenType := lexer.ILLEGAL

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

		// Smart spacing for type annotations
		if typeBuilder.Len() > 2 { // More than ": "
			needSpace := true

			// No space before <
			if tok.Type == lexer.LANGLE {
				needSpace = false
			}
			// No space after <
			if lastTokenType == lexer.LANGLE {
				needSpace = false
			}
			// No space before >
			if tok.Type == lexer.RANGLE {
				needSpace = false
			}
			// No space before comma
			if tok.Type == lexer.COMMA {
				needSpace = false
			}
			// No space before (
			if tok.Type == lexer.LPAREN {
				needSpace = false
			}
			// No space after (
			if lastTokenType == lexer.LPAREN {
				needSpace = false
			}
			// No space before )
			if tok.Type == lexer.RPAREN {
				needSpace = false
			}
			// No space before [
			if tok.Type == lexer.LBRACKET {
				needSpace = false
			}
			// No space after [
			if lastTokenType == lexer.LBRACKET {
				needSpace = false
			}
			// No space before ]
			if tok.Type == lexer.RBRACKET {
				needSpace = false
			}

			if needSpace {
				typeBuilder.WriteString(" ")
			}
		}
		typeBuilder.WriteString(tok.Literal)

		// Track bracket depths AFTER spacing logic
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

		lastTokenType = tok.Type
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
	lastLiteral := ""

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

		// Add token to value string with smart spacing
		// Note: We check spacing BEFORE updating bracket depths so we can detect generics
		if valueBuilder.Len() > 0 {
			// Don't add space before/after certain tokens
			needSpace := true

			// Don't add space before closing parens/brackets
			if tok.Type == lexer.RPAREN || tok.Type == lexer.RBRACKET {
				needSpace = false
			}
			// Don't add space after opening parens/brackets
			if lastTokenType == lexer.LPAREN || lastTokenType == lexer.LBRACKET {
				needSpace = false
			}
			// Don't add space before ( for function calls (after IDENT or RPAREN)
			if tok.Type == lexer.LPAREN && (lastTokenType == lexer.IDENT || lastTokenType == lexer.RPAREN || lastTokenType == lexer.RANGLE) {
				needSpace = false
			}
			// Don't add space before comma
			if tok.Type == lexer.COMMA {
				needSpace = false
			}
			// Don't add space before = if previous token forms compound operator (>=, <=, :=, ==)
			if tok.Type == lexer.ASSIGN {
				if lastTokenType == lexer.RANGLE || lastTokenType == lexer.LANGLE ||
					lastTokenType == lexer.COLON || lastTokenType == lexer.ASSIGN {
					needSpace = false
				}
			}
			// Don't add space before > if previous token is = (lambda arrow =>)
			if tok.Type == lexer.RANGLE && lastTokenType == lexer.ASSIGN {
				needSpace = false
			}
			// Don't add space before > if previous token is - (return type arrow ->)
			if tok.Type == lexer.RANGLE && lastLiteral == "-" {
				needSpace = false
			}
			// Handle angle brackets for generics: Option<T>, Result<T, E>
			// Don't add space after < or before > when used as generics (angleDepth > 0)
			if tok.Type == lexer.RANGLE && angleDepth > 0 {
				needSpace = false
			}
			if lastTokenType == lexer.LANGLE && angleDepth > 0 {
				needSpace = false
			}
			// Don't add space between consecutive ? characters (preserve ?? null coalescing operator)
			if tok.Literal == "?" && lastLiteral == "?" {
				needSpace = false
			}
			// Don't add space between ? and . (preserve ?. safe navigation operator)
			if tok.Literal == "." && lastLiteral == "?" {
				needSpace = false
			}

			if needSpace {
				valueBuilder.WriteString(" ")
			}
		}

		// Track bracket depths AFTER spacing logic
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
		lastLiteral = tok.Literal
	}

	return strings.TrimSpace(valueBuilder.String()), nil
}
