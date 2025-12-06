package ast

import (
	"go/scanner"
	gotoken "go/token"
)

// TransformSource transforms Dingo source to valid Go source.
// It uses a token-based transformer to handle simple Dingo syntax.
//
// This is a legacy implementation that handles basic token-level transformations.
// Most features are now handled by the AST-based pipeline in pkg/ast/ast_transformer.go
//
// Currently handles:
// - Enums: enum Name { Variant } → Go interface pattern
// - Type annotations: param: Type → param Type
// - Generic syntax: Result<T,E> → Result[T,E]
// - Let declarations: let x = → x :=
//
// Complex features handled by AST pipeline (ast_transformer.go):
// - Error propagation: x? → expanded error handling
// - Pattern matching: match expr { ... } → type switch
// - Lambdas: |x| → func(x any) any
// - Null coalescing: a ?? b → (future)
// - Safe navigation: x?.field → (future)
func TransformSource(src []byte) ([]byte, []SourceMapping, error) {
	// First pass: Transform enums (uses separate parser + codegen)
	src, _ = TransformEnumSource(src)

	var mappings []SourceMapping

	// Create a file set for tokenization
	fset := gotoken.NewFileSet()
	file := fset.AddFile("", -1, len(src))

	// Use Go's scanner to tokenize
	var s scanner.Scanner
	s.Init(file, src, nil, scanner.ScanComments)

	// Collect all tokens with their positions
	type tokenInfo struct {
		pos gotoken.Pos
		tok gotoken.Token
		lit string
	}
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
	genericDepth := 0

	for i := 0; i < len(tokens)-1; i++ {
		t := tokens[i]
		offset := file.Offset(t.pos)

		// Track parentheses for parameter context
		if t.tok == gotoken.LPAREN {
			if i > 0 {
				prev := tokens[i-1]
				if prev.tok == gotoken.IDENT || prev.tok == gotoken.RBRACK || prev.tok == gotoken.FUNC {
					inParamList = true
				}
			}
			parenDepth++
		}
		if t.tok == gotoken.RPAREN {
			parenDepth--
			if parenDepth == 0 {
				inParamList = false
			}
		}

		// Handle generic type syntax: Result<T, E> -> Result[T, E]
		if t.tok == gotoken.LSS {
			if i > 0 && tokens[i-1].tok == gotoken.IDENT {
				prevLit := tokens[i-1].lit
				isFieldAccess := i >= 2 && tokens[i-2].tok == gotoken.PERIOD

				isLikelyGeneric := false
				if !isFieldAccess && len(prevLit) > 0 {
					knownGenerics := map[string]bool{
						"Result": true, "Option": true,
						"Some": true, "None": true, "Ok": true, "Err": true,
					}
					if knownGenerics[prevLit] {
						isLikelyGeneric = true
					}
				}

				if isLikelyGeneric && i+1 < len(tokens) {
					next := tokens[i+1]
					if next.tok == gotoken.IDENT || next.tok == gotoken.MUL || next.tok == gotoken.LBRACK {
						result = append(result, src[lastCopied:offset]...)
						result = append(result, '[')
						lastCopied = offset + 1
						genericDepth++

						mappings = append(mappings, SourceMapping{
							DingoStart: offset,
							DingoEnd:   offset + 1,
							GoStart:    len(result) - 1,
							GoEnd:      len(result),
							Kind:       "generic_open",
						})
						continue
					}
				}
			}
		}

		// Handle generic closing: > -> ]
		if t.tok == gotoken.GTR && genericDepth > 0 {
			result = append(result, src[lastCopied:offset]...)
			result = append(result, ']')
			lastCopied = offset + 1
			genericDepth--

			mappings = append(mappings, SourceMapping{
				DingoStart: offset,
				DingoEnd:   offset + 1,
				GoStart:    len(result) - 1,
				GoEnd:      len(result),
				Kind:       "generic_close",
			})
			continue
		}

		// Handle type annotations: param: Type -> param Type
		if t.tok == gotoken.COLON && inParamList {
			if i > 0 && tokens[i-1].tok == gotoken.IDENT {
				result = append(result, src[lastCopied:offset]...)
				result = append(result, ' ')
				lastCopied = offset + 1

				mappings = append(mappings, SourceMapping{
					DingoStart: offset,
					DingoEnd:   offset + 1,
					GoStart:    len(result) - 1,
					GoEnd:      len(result),
					Kind:       "type_annotation",
				})
				continue
			}
		}

		// Handle 'let' keyword
		if t.tok == gotoken.IDENT && t.lit == "let" {
			if i+2 < len(tokens) {
				next := tokens[i+1]
				afterNext := tokens[i+2]
				if next.tok == gotoken.IDENT && afterNext.tok == gotoken.ASSIGN {
					result = append(result, src[lastCopied:offset]...)
					lastCopied = offset + len("let")

					for lastCopied < len(src) && (src[lastCopied] == ' ' || src[lastCopied] == '\t') {
						lastCopied++
					}

					mappings = append(mappings, SourceMapping{
						DingoStart: offset,
						DingoEnd:   offset + len("let"),
						GoStart:    len(result),
						GoEnd:      len(result),
						Kind:       "let_keyword",
					})
				}
			}
		}

		// Handle = after 'let varname' -> change to :=
		if t.tok == gotoken.ASSIGN {
			if i >= 2 && tokens[i-2].tok == gotoken.IDENT && tokens[i-2].lit == "let" {
				result = append(result, src[lastCopied:offset]...)
				result = append(result, ':', '=')
				lastCopied = offset + 1

				mappings = append(mappings, SourceMapping{
					DingoStart: offset,
					DingoEnd:   offset + 1,
					GoStart:    len(result) - 2,
					GoEnd:      len(result),
					Kind:       "let_assign",
				})
				continue
			}
		}
	}

	// Copy remaining bytes
	if lastCopied < len(src) {
		result = append(result, src[lastCopied:]...)
	}

	return result, mappings, nil
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
