// Package lsp provides Dingo-native symbol extraction for document outlines.
//
// This file implements a symbol extractor that scans Dingo source files using
// go/scanner to identify Dingo-specific language constructs (enums, match
// expressions, lambdas) and represents them as LSP DocumentSymbol entries.
//
// CLAUDE.md Compliance:
// - Uses go/scanner for tokenization (NOT bytes.Index, regex, or string scanning)
// - Positions come from token.FileSet, NOT manual byte offset calculation
// - Follows the principle: "Position info flows through the token system"
package lsp

import (
	"go/scanner"
	"go/token"
	"os"
	"strings"

	"go.lsp.dev/protocol"
)

// DingoSymbolExtractor scans Dingo source for language-specific constructs
// using go/scanner (CLAUDE.md compliant - token-based, NOT byte-based).
type DingoSymbolExtractor struct {
	logger Logger
}

// NewDingoSymbolExtractor creates a new extractor instance.
func NewDingoSymbolExtractor(logger Logger) *DingoSymbolExtractor {
	return &DingoSymbolExtractor{logger: logger}
}

// ExtractDingoSymbols returns Dingo-native symbols not visible in Go output.
// These include:
//   - Enum declarations with variants as children
//   - Match expressions with arms as children (when inside functions)
//   - Lambda expressions as anonymous functions
//
// The content parameter should be the raw Dingo source bytes.
// The uri parameter is used for logging and identification.
func (e *DingoSymbolExtractor) ExtractDingoSymbols(content []byte, uri protocol.DocumentURI) ([]protocol.DocumentSymbol, error) {
	e.logger.Debugf("[DingoSymbols] Extracting symbols from %s (%d bytes)", uri.Filename(), len(content))

	// Create FileSet for position tracking (CLAUDE.md compliant)
	fset := token.NewFileSet()
	file := fset.AddFile(string(uri), fset.Base(), len(content))

	// Initialize scanner with comment scanning enabled
	var s scanner.Scanner
	var errors []string
	errorHandler := func(pos token.Position, msg string) {
		errors = append(errors, msg)
	}
	s.Init(file, content, errorHandler, scanner.ScanComments)

	var symbols []protocol.DocumentSymbol

	// Scan for Dingo-specific constructs
	for {
		pos, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}

		// Check for Dingo keywords
		if tok == token.IDENT {
			switch lit {
			case "enum":
				// Parse enum declaration
				enumSymbol := e.parseEnumDeclaration(fset, &s, pos, content)
				if enumSymbol != nil {
					symbols = append(symbols, *enumSymbol)
				}

			case "match":
				// Parse match expression
				// Note: Match expressions are typically inside functions,
				// so they'll appear as nested symbols in the gopls output.
				// We extract them here as standalone symbols for the outline.
				matchSymbol := e.parseMatchExpression(fset, &s, pos, content)
				if matchSymbol != nil {
					symbols = append(symbols, *matchSymbol)
				}
			}
		}

		// Check for lambda syntax
		// Rust-style: |params| body
		if tok == token.OR {
			lambdaSymbol := e.parseLambdaExpression(fset, &s, pos, content)
			if lambdaSymbol != nil {
				symbols = append(symbols, *lambdaSymbol)
			}
		}

		// TypeScript-style lambdas: (params) => body or ident => body
		// These are harder to detect without full parsing context,
		// as we need to distinguish from regular function calls.
		// We'll detect the => token and backtrack to find the params.
		if tok == token.ARROW {
			// Arrow found - this is likely a lambda
			// The params were already scanned, so we need to track context
			// For now, we mark the arrow position as the lambda start
			// (Full implementation would require lookahead/lookbehind)
			lambdaSymbol := e.parseArrowLambda(fset, pos, content)
			if lambdaSymbol != nil {
				symbols = append(symbols, *lambdaSymbol)
			}
		}
	}

	if len(errors) > 0 {
		e.logger.Warnf("[DingoSymbols] Scanner errors: %v", errors)
	}

	e.logger.Debugf("[DingoSymbols] Extracted %d Dingo symbols", len(symbols))
	return symbols, nil
}

// parseEnumDeclaration parses an enum declaration starting from 'enum' keyword.
// Returns a DocumentSymbol with variants as children.
//
// Syntax: enum Name[T, E] { Variant1, Variant2(T), Variant3 { field: Type } }
func (e *DingoSymbolExtractor) parseEnumDeclaration(fset *token.FileSet, s *scanner.Scanner, enumPos token.Pos, content []byte) *protocol.DocumentSymbol {
	startPosition := fset.Position(enumPos)

	// Expect identifier (enum name)
	namePos, tok, name := s.Scan()
	if tok != token.IDENT {
		e.logger.Debugf("[DingoSymbols] Expected enum name, got %s", tok)
		return nil
	}

	e.logger.Debugf("[DingoSymbols] Parsing enum: %s at line %d", name, startPosition.Line)

	// Skip optional type parameters <T, E>
	var nextTok token.Token
	_, nextTok, _ = s.Scan()
	if nextTok == token.LSS { // '<'
		// Skip until '>'
		depth := 1
		for depth > 0 {
			_, tok, _ := s.Scan()
			if tok == token.EOF {
				return nil
			}
			if tok == token.LSS {
				depth++
			} else if tok == token.GTR {
				depth--
			}
		}
		_, nextTok, _ = s.Scan()
	}

	// Expect '{'
	if nextTok != token.LBRACE {
		e.logger.Debugf("[DingoSymbols] Expected '{' after enum name, got %s", nextTok)
		return nil
	}

	// Parse variants until '}'
	var variants []protocol.DocumentSymbol
	var endPos token.Pos

	for {
		pos, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}
		if tok == token.RBRACE {
			endPos = pos
			break
		}

		// Skip newlines, commas, and whitespace
		if tok == token.SEMICOLON || tok == token.COMMA {
			continue
		}

		// Variant name
		if tok == token.IDENT {
			variantPosition := fset.Position(pos)
			variantSymbol := protocol.DocumentSymbol{
				Name: lit,
				Kind: protocol.SymbolKindEnumMember,
				Range: protocol.Range{
					Start: protocol.Position{
						Line:      uint32(variantPosition.Line - 1),
						Character: uint32(variantPosition.Column - 1),
					},
					End: protocol.Position{
						Line:      uint32(variantPosition.Line - 1),
						Character: uint32(variantPosition.Column - 1 + len(lit)),
					},
				},
				SelectionRange: protocol.Range{
					Start: protocol.Position{
						Line:      uint32(variantPosition.Line - 1),
						Character: uint32(variantPosition.Column - 1),
					},
					End: protocol.Position{
						Line:      uint32(variantPosition.Line - 1),
						Character: uint32(variantPosition.Column - 1 + len(lit)),
					},
				},
			}

			// Check for variant fields: Ok(T) or RGB { r: int }
			peekPos, peekTok, _ := s.Scan()
			if peekTok == token.LPAREN {
				// Tuple variant: skip until ')'
				variantSymbol.Detail = "tuple variant"
				depth := 1
				for depth > 0 {
					_, tok, _ := s.Scan()
					if tok == token.EOF {
						break
					}
					if tok == token.LPAREN {
						depth++
					} else if tok == token.RPAREN {
						depth--
					}
				}
			} else if peekTok == token.LBRACE {
				// Struct variant: skip until '}'
				variantSymbol.Detail = "struct variant"
				depth := 1
				for depth > 0 {
					_, tok, _ := s.Scan()
					if tok == token.EOF {
						break
					}
					if tok == token.LBRACE {
						depth++
					} else if tok == token.RBRACE {
						depth--
					}
				}
			} else {
				// Unit variant - push back the token we peeked
				// Note: go/scanner doesn't support unread, so we just continue
				// The token will be re-evaluated in the next loop iteration
				variantSymbol.Detail = "unit variant"
				// We need to check if this is the closing brace
				if peekTok == token.RBRACE {
					variants = append(variants, variantSymbol)
					endPos = peekPos
					break
				}
			}

			variants = append(variants, variantSymbol)
		}
	}

	// Build enum symbol with variants as children
	endPosition := fset.Position(endPos)
	if !endPos.IsValid() {
		// If we didn't find closing brace, estimate end position
		endPosition = fset.Position(namePos)
		endPosition.Column += len(name)
	}

	enumSymbol := protocol.DocumentSymbol{
		Name:   name,
		Kind:   protocol.SymbolKindEnum,
		Detail: "enum",
		Range: protocol.Range{
			Start: protocol.Position{
				Line:      uint32(startPosition.Line - 1),
				Character: uint32(startPosition.Column - 1),
			},
			End: protocol.Position{
				Line:      uint32(endPosition.Line - 1),
				Character: uint32(endPosition.Column), // +1 for '}'
			},
		},
		SelectionRange: protocol.Range{
			Start: protocol.Position{
				Line:      uint32(fset.Position(namePos).Line - 1),
				Character: uint32(fset.Position(namePos).Column - 1),
			},
			End: protocol.Position{
				Line:      uint32(fset.Position(namePos).Line - 1),
				Character: uint32(fset.Position(namePos).Column - 1 + len(name)),
			},
		},
		Children: variants,
	}

	return &enumSymbol
}

// parseMatchExpression parses a match expression starting from 'match' keyword.
// Returns a DocumentSymbol with match arms as children.
//
// Syntax: match expr { Pattern1 => body1, Pattern2 => body2, ... }
func (e *DingoSymbolExtractor) parseMatchExpression(fset *token.FileSet, s *scanner.Scanner, matchPos token.Pos, content []byte) *protocol.DocumentSymbol {
	startPosition := fset.Position(matchPos)

	e.logger.Debugf("[DingoSymbols] Parsing match at line %d", startPosition.Line)

	// Skip scrutinee expression until '{'
	braceDepth := 0
	var openBracePos token.Pos
	foundBrace := false

	for !foundBrace {
		pos, tok, _ := s.Scan()
		if tok == token.EOF {
			return nil
		}
		if tok == token.LBRACE && braceDepth == 0 {
			openBracePos = pos
			foundBrace = true
			break
		}
		// Handle nested braces/parens in scrutinee (rare but possible)
		if tok == token.LPAREN {
			braceDepth++
		} else if tok == token.RPAREN {
			braceDepth--
		}
	}

	// Parse match arms until '}'
	var arms []protocol.DocumentSymbol
	var closeBracePos token.Pos
	armIndex := 0

	for {
		pos, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}
		if tok == token.RBRACE {
			closeBracePos = pos
			break
		}

		// Look for pattern start (identifier or other pattern tokens)
		if tok == token.IDENT || tok == token.LPAREN || tok == token.INT || tok == token.STRING {
			armStartPos := pos
			armStartPosition := fset.Position(armStartPos)
			patternText := lit

			// Collect pattern until '=>'
			for {
				_, nextTok, nextLit := s.Scan()
				if nextTok == token.EOF || nextTok == token.RBRACE {
					break
				}
				if nextTok == token.ARROW {
					// Found '=>', pattern complete
					break
				}
				if nextTok == token.IDENT || nextTok == token.LPAREN || nextTok == token.RPAREN ||
					nextTok == token.COMMA || nextTok == token.INT || nextTok == token.STRING {
					if nextTok == token.IDENT {
						patternText += " " + nextLit
					}
				}
			}

			// Skip arm body until comma or '}'
			bodyDepth := 0
			for {
				_, nextTok, _ := s.Scan()
				if nextTok == token.EOF {
					break
				}
				if nextTok == token.LBRACE {
					bodyDepth++
				} else if nextTok == token.RBRACE {
					if bodyDepth == 0 {
						// End of match block
						break
					}
					bodyDepth--
				} else if nextTok == token.COMMA && bodyDepth == 0 {
					// End of this arm
					break
				}
			}

			// Truncate pattern text for display
			if len(patternText) > 30 {
				patternText = patternText[:27] + "..."
			}

			armSymbol := protocol.DocumentSymbol{
				Name:   patternText,
				Kind:   protocol.SymbolKindEvent, // Event kind for match arms
				Detail: "match arm",
				Range: protocol.Range{
					Start: protocol.Position{
						Line:      uint32(armStartPosition.Line - 1),
						Character: uint32(armStartPosition.Column - 1),
					},
					End: protocol.Position{
						Line:      uint32(armStartPosition.Line - 1),
						Character: uint32(armStartPosition.Column - 1 + len(patternText)),
					},
				},
				SelectionRange: protocol.Range{
					Start: protocol.Position{
						Line:      uint32(armStartPosition.Line - 1),
						Character: uint32(armStartPosition.Column - 1),
					},
					End: protocol.Position{
						Line:      uint32(armStartPosition.Line - 1),
						Character: uint32(armStartPosition.Column - 1 + len(patternText)),
					},
				},
			}

			arms = append(arms, armSymbol)
			armIndex++
		}
	}

	// Build match symbol with arms as children
	endPosition := fset.Position(closeBracePos)
	if !closeBracePos.IsValid() {
		endPosition = fset.Position(openBracePos)
	}

	matchSymbol := protocol.DocumentSymbol{
		Name:   "match",
		Kind:   protocol.SymbolKindOperator, // Operator kind for match expressions
		Detail: "match expression",
		Range: protocol.Range{
			Start: protocol.Position{
				Line:      uint32(startPosition.Line - 1),
				Character: uint32(startPosition.Column - 1),
			},
			End: protocol.Position{
				Line:      uint32(endPosition.Line - 1),
				Character: uint32(endPosition.Column),
			},
		},
		SelectionRange: protocol.Range{
			Start: protocol.Position{
				Line:      uint32(startPosition.Line - 1),
				Character: uint32(startPosition.Column - 1),
			},
			End: protocol.Position{
				Line:      uint32(startPosition.Line - 1),
				Character: uint32(startPosition.Column - 1 + 5), // "match"
			},
		},
		Children: arms,
	}

	return &matchSymbol
}

// parseLambdaExpression parses a Rust-style lambda: |params| body
func (e *DingoSymbolExtractor) parseLambdaExpression(fset *token.FileSet, s *scanner.Scanner, pipePos token.Pos, content []byte) *protocol.DocumentSymbol {
	startPosition := fset.Position(pipePos)

	e.logger.Debugf("[DingoSymbols] Parsing lambda at line %d", startPosition.Line)

	// Check if this is actually a lambda (not bitwise OR)
	// A lambda has format: |params| body
	// Collect tokens until closing |

	var params []string
	foundClosingPipe := false

	for {
		_, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}
		if tok == token.OR {
			// Closing pipe
			foundClosingPipe = true
			break
		}
		if tok == token.SEMICOLON {
			// Newline before closing pipe - not a valid lambda
			break
		}
		if tok == token.IDENT {
			params = append(params, lit)
		}
	}

	if !foundClosingPipe {
		// Not a lambda, just a bitwise OR
		return nil
	}

	// Build parameter string
	paramStr := strings.Join(params, ", ")
	displayName := "|" + paramStr + "| <lambda>"

	lambdaSymbol := protocol.DocumentSymbol{
		Name:   displayName,
		Kind:   protocol.SymbolKindFunction,
		Detail: "lambda",
		Range: protocol.Range{
			Start: protocol.Position{
				Line:      uint32(startPosition.Line - 1),
				Character: uint32(startPosition.Column - 1),
			},
			End: protocol.Position{
				Line:      uint32(startPosition.Line - 1),
				Character: uint32(startPosition.Column - 1 + len(displayName)),
			},
		},
		SelectionRange: protocol.Range{
			Start: protocol.Position{
				Line:      uint32(startPosition.Line - 1),
				Character: uint32(startPosition.Column - 1),
			},
			End: protocol.Position{
				Line:      uint32(startPosition.Line - 1),
				Character: uint32(startPosition.Column - 1 + len(displayName)),
			},
		},
	}

	return &lambdaSymbol
}

// parseArrowLambda parses a TypeScript-style lambda detected by '=>' arrow.
// Note: This is called after the arrow is detected, so we use context hints.
func (e *DingoSymbolExtractor) parseArrowLambda(fset *token.FileSet, arrowPos token.Pos, content []byte) *protocol.DocumentSymbol {
	position := fset.Position(arrowPos)

	e.logger.Debugf("[DingoSymbols] Detected arrow lambda at line %d", position.Line)

	// Simple representation - we don't have the params since they were already scanned
	// In a real implementation, we'd need to track context or use the parser AST
	lambdaSymbol := protocol.DocumentSymbol{
		Name:   "() => <lambda>",
		Kind:   protocol.SymbolKindFunction,
		Detail: "arrow lambda",
		Range: protocol.Range{
			Start: protocol.Position{
				Line:      uint32(position.Line - 1),
				Character: uint32(position.Column - 1),
			},
			End: protocol.Position{
				Line:      uint32(position.Line - 1),
				Character: uint32(position.Column + 1), // '=>' is 2 chars
			},
		},
		SelectionRange: protocol.Range{
			Start: protocol.Position{
				Line:      uint32(position.Line - 1),
				Character: uint32(position.Column - 1),
			},
			End: protocol.Position{
				Line:      uint32(position.Line - 1),
				Character: uint32(position.Column + 1),
			},
		},
	}

	return &lambdaSymbol
}

// MergeDingoSymbols combines gopls symbols with Dingo-native symbols.
// It finds matching parent symbols (by position) and adds Dingo children,
// or adds standalone Dingo symbols at appropriate positions.
func MergeDingoSymbols(goplsSymbols, dingoSymbols []protocol.DocumentSymbol) []protocol.DocumentSymbol {
	if len(dingoSymbols) == 0 {
		return goplsSymbols
	}

	// Create a map of line -> Dingo symbols for quick lookup
	dingoByLine := make(map[uint32][]protocol.DocumentSymbol)
	for _, sym := range dingoSymbols {
		line := sym.Range.Start.Line
		dingoByLine[line] = append(dingoByLine[line], sym)
	}

	// Process gopls symbols - if they overlap with Dingo symbols, merge
	result := make([]protocol.DocumentSymbol, 0, len(goplsSymbols)+len(dingoSymbols))

	// Track which Dingo symbols have been merged
	mergedLines := make(map[uint32]bool)

	for _, goplsSym := range goplsSymbols {
		// Check if any Dingo symbols should be children of this gopls symbol
		// (e.g., enum variants should be children of the enum type)
		startLine := goplsSym.Range.Start.Line
		endLine := goplsSym.Range.End.Line

		var additionalChildren []protocol.DocumentSymbol
		for line := startLine; line <= endLine; line++ {
			if dingoSyms, ok := dingoByLine[line]; ok {
				for _, dingoSym := range dingoSyms {
					// Only merge if Dingo symbol is within gopls symbol range
					if dingoSym.Range.Start.Line >= startLine && dingoSym.Range.End.Line <= endLine {
						// For enums, replace gopls symbol entirely (gopls sees BadDecl)
						if dingoSym.Kind == protocol.SymbolKindEnum {
							// Skip gopls BadDecl, use Dingo enum instead
							result = append(result, dingoSym)
							mergedLines[line] = true
							goto nextGoplsSym
						}
						// For other symbols, add as children
						additionalChildren = append(additionalChildren, dingoSym)
						mergedLines[line] = true
					}
				}
			}
		}

		// Add any Dingo children found
		if len(additionalChildren) > 0 {
			goplsSym.Children = append(goplsSym.Children, additionalChildren...)
		}
		result = append(result, goplsSym)

	nextGoplsSym:
	}

	// Add remaining Dingo symbols that weren't merged
	for line, dingoSyms := range dingoByLine {
		if !mergedLines[line] {
			result = append(result, dingoSyms...)
		}
	}

	return result
}

// ExtractFromFile is a convenience function that reads a file and extracts symbols.
func (e *DingoSymbolExtractor) ExtractFromFile(path string) ([]protocol.DocumentSymbol, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	uri := protocol.DocumentURI("file://" + path)
	return e.ExtractDingoSymbols(content, uri)
}
