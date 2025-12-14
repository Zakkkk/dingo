package ast

import (
	"go/scanner"
	gotoken "go/token"
)

// tokenInfo holds token information during transformation
type tokenInfo struct {
	pos gotoken.Pos
	tok gotoken.Token
	lit string
}

// TransformSource transforms Dingo source to valid Go source.
// It uses a token-based transformer to handle simple Dingo syntax.
//
// This is a legacy implementation that handles basic token-level transformations.
// Most features are now handled by the AST-based pipeline in pkg/ast/ast_transformer.go
//
// Currently handles:
// - Enums: enum Name { Variant } → Go interface pattern
// - Type annotations: param: Type → param Type
//
// NOTE: Generic syntax uses Go's native [T] syntax directly. No transformation needed.
//
// Complex features handled by AST pipeline (ast_transformer.go):
// - Error propagation: x? → expanded error handling
// - Pattern matching: match expr { ... } → type switch
// - Lambdas: |x| → func(x any) any
// - Null coalescing: a ?? b → (future)
// - Safe navigation: x?.field → (future)
//
// Note: Previously returned []SourceMapping for byte-offset tracking, but this has
// been removed. Position tracking now uses //line directives + LineMappings.
func TransformSource(src []byte) ([]byte, error) {
	// First pass: Transform enums (uses separate parser + codegen)
	src, enumRegistry := TransformEnumSource(src)

	// Second pass: Transform enum constructor calls to NewVariant() pattern
	src = TransformEnumConstructors(src, enumRegistry)

	// Create a file set for tokenization
	fset := gotoken.NewFileSet()
	file := fset.AddFile("", -1, len(src))

	// Use Go's scanner to tokenize
	var s scanner.Scanner
	s.Init(file, src, nil, scanner.ScanComments)

	// Collect all tokens with their positions
	var tokens []tokenInfo

	for {
		pos, tok, lit := s.Scan()
		tokens = append(tokens, tokenInfo{pos, tok, lit})
		if tok == gotoken.EOF {
			break
		}
	}

	// Process tokens and build output
	result := make([]byte, 0, len(src))
	lastCopied := 0

	// State tracking
	parenDepth := 0
	inParamList := false
	inLambdaParams := false // Track TypeScript-style lambda (x: Type) => ...

	for i := 0; i < len(tokens)-1; i++ {
		t := tokens[i]
		offset := file.Offset(t.pos)

		// Track parentheses for parameter context
		// IMPORTANT: We only set inParamList for function DECLARATIONS, not function CALLS.
		// Function calls like Ok[User](User{ID: 1}) should NOT have colons transformed.
		if t.tok == gotoken.LPAREN {
			if i > 0 {
				prev := tokens[i-1]
				// FUNC( = function type literal: func(x: int)
				// IDENT( = function declaration after name: func foo(x: int)
				// NOTE: Do NOT include RBRACK - that's for generic function CALLS: Ok[T](...)
				// which contain struct literals where colons should be preserved
				if prev.tok == gotoken.FUNC || prev.tok == gotoken.IDENT {
					inParamList = true
				}
			}
			// Check for TypeScript-style lambda: ( ... ) =>
			// Lookahead to find matching ) followed by => or ): Type =>
			if isLambdaParenStart(tokens, i) {
				inLambdaParams = true
			}
			parenDepth++
		}
		if t.tok == gotoken.RPAREN {
			parenDepth--
			if parenDepth == 0 {
				inParamList = false
				inLambdaParams = false
			}
		}

		// NOTE: Generic syntax transformation (<T> -> [T]) has been REMOVED.
		// Dingo now uses Go's native generic syntax [T] directly.
		// Users should write Result[int, error], not Result[int, error].

		// Handle type annotations: param: Type -> param Type
		// Also handles lambda parameter annotations and return type annotations
		if t.tok == gotoken.COLON {
			// Case 1: Inside parameter list (func params, method receiver)
			// Case 2: Inside lambda parameter list (x: Type) => ...
			// Case 3: Lambda return type ): Type =>
			if (inParamList || inLambdaParams) && i > 0 && tokens[i-1].tok == gotoken.IDENT {
				result = append(result, src[lastCopied:offset]...)
				result = append(result, ' ')
				lastCopied = offset + 1
				continue
			}
			// Case 3: Lambda return type - ): Type => pattern
			// We just saw ), now we see :, and we expect IDENT then =>
			if i > 0 && tokens[i-1].tok == gotoken.RPAREN {
				if isLambdaReturnType(tokens, i) {
					// Remove the colon, lambda codegen will handle it properly
					result = append(result, src[lastCopied:offset]...)
					// Don't output colon - the lambda parser expects (x Type) string => not (x Type): string =>
					// Actually, we need to skip the colon AND the type, let lambda parser handle it
					// For now, just skip the colon - the issue is go/parser doesn't understand this syntax
					// Solution: We need to transform the ENTIRE lambda BEFORE this stage
					lastCopied = offset + 1 // Skip the colon
					continue
				}
			}
		}

	}

	// Copy remaining bytes
	if lastCopied < len(src) {
		result = append(result, src[lastCopied:]...)
	}

	return result, nil
}

// isIdentifier checks if a string is a valid Go identifier.
func isIdentifier(s string) bool {
	if len(s) == 0 {
		return false
	}
	for i, ch := range s {
		if i == 0 {
			if !isLetter(ch) && ch != '_' {
				return false
			}
		} else {
			if !isLetter(ch) && !isDigit(ch) && ch != '_' {
				return false
			}
		}
	}
	return true
}

func isLetter(ch rune) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
}

func isDigit(ch rune) bool {
	return ch >= '0' && ch <= '9'
}

// isArrowToken checks if tokens[i] and tokens[i+1] form => (ASSIGN followed by GTR)
func isArrowToken(tokens []tokenInfo, i int) bool {
	if i+1 >= len(tokens) {
		return false
	}
	return tokens[i].tok == gotoken.ASSIGN && tokens[i+1].tok == gotoken.GTR
}

// isLambdaParenStart checks if this ( starts a TypeScript-style lambda
// by looking ahead for pattern like (params) => or (params): Type =>
// Note: Go scanner tokenizes => as two tokens: = (ASSIGN) and > (GTR)
func isLambdaParenStart(tokens []tokenInfo, start int) bool {
	// Look for matching ) then check for => or : Type =>
	depth := 0
	for i := start; i < len(tokens); i++ {
		switch tokens[i].tok {
		case gotoken.LPAREN:
			depth++
		case gotoken.RPAREN:
			depth--
			if depth == 0 {
				// Found matching ), check what follows
				if i+2 < len(tokens) {
					// Check for => (tokenized as = >)
					if isArrowToken(tokens, i+1) {
						return true
					}
					// Check for : Type => pattern
					if tokens[i+1].tok == gotoken.COLON {
						// : should be followed by IDENT (type) then = >
						if i+3 < len(tokens) && tokens[i+2].tok == gotoken.IDENT {
							if isArrowToken(tokens, i+3) {
								return true
							}
						}
					}
				}
				return false
			}
		case gotoken.SEMICOLON, gotoken.LBRACE, gotoken.RBRACE:
			// Hit statement boundary, not a lambda
			return false
		}
	}
	return false
}

// isLambdaReturnType checks if this : at position i is a lambda return type annotation
// Pattern: ): Type => (where => is tokenized as = >)
func isLambdaReturnType(tokens []tokenInfo, i int) bool {
	// Current token is :, previous is )
	// Check for IDENT (type name) followed by = >
	if i+3 < len(tokens) && tokens[i+1].tok == gotoken.IDENT {
		if isArrowToken(tokens, i+2) {
			return true
		}
	}
	return false
}
