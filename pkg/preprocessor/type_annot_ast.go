package preprocessor

import (
	"bytes"
	"fmt"
	"go/scanner"
	"go/token"
)

// TypeAnnotASTProcessor converts Dingo type annotations using token-based parsing
// This replaces the buggy regex-based approach in type_annot.go
//
// Handles:
//   - param: Type → param Type
//   - ) -> Type { → ) Type {
//   - Complex types: Map<string, List<int>>, func(int) error
//   - Variadic: args: ...int → args ...int
type TypeAnnotASTProcessor struct{}

// NewTypeAnnotASTProcessor creates a new AST-based type annotation processor
func NewTypeAnnotASTProcessor() *TypeAnnotASTProcessor {
	return &TypeAnnotASTProcessor{}
}

// Name returns the processor name
func (p *TypeAnnotASTProcessor) Name() string {
	return "type_annotations_ast"
}

// ProcessBody implements BodyProcessor interface for lambda body processing
func (p *TypeAnnotASTProcessor) ProcessBody(body []byte) ([]byte, error) {
	result, _, err := p.Process(body)
	return result, err
}

// Process is the legacy interface method (implements FeatureProcessor)
func (p *TypeAnnotASTProcessor) Process(source []byte) ([]byte, []Mapping, error) {
	result, _, err := p.ProcessInternal(string(source))
	return []byte(result), nil, err
}

// ProcessInternal transforms type annotations with metadata emission support
// Converts: func foo(x: int, y: string) -> error
// To:       func foo(x int, y string) error
func (p *TypeAnnotASTProcessor) ProcessInternal(code string) (string, []TransformMetadata, error) {
	var metadata []TransformMetadata
	counter := 0

	lines := bytes.Split([]byte(code), []byte("\n"))
	var result bytes.Buffer

	for lineIdx, line := range lines {
		lineNum := lineIdx + 1

		// Check if this line contains a function declaration
		if bytes.Contains(line, []byte("func ")) {
			transformed, hadTransformation := p.processLine(line)

			if hadTransformation {
				marker := fmt.Sprintf("// dingo:t:%d", counter)

				// Write transformed line (no marker in output)
				result.Write(transformed)

				// Metadata only - marker not written to output
				metadata = append(metadata, TransformMetadata{
					Type:            "type_annot",
					OriginalLine:    lineNum,
					OriginalColumn:  1,
					OriginalLength:  len(line),
					OriginalText:    string(line),
					GeneratedMarker: marker,
					ASTNodeType:     "FuncDecl",
				})
				counter++
			} else {
				result.Write(line)
			}
		} else {
			result.Write(line)
		}

		if lineIdx < len(lines)-1 {
			result.WriteByte('\n')
		}
	}

	return result.String(), metadata, nil
}

// processLine processes a single line containing a function declaration
// Returns (transformed line, true) if transformations were made, (original, false) otherwise
func (p *TypeAnnotASTProcessor) processLine(line []byte) ([]byte, bool) {
	// Quick check: does line contain : or -> ?
	hasColon := bytes.Contains(line, []byte(":"))
	hasArrow := bytes.Contains(line, []byte("->"))

	if !hasColon && !hasArrow {
		return line, false
	}

	hadTransformation := false
	result := line

	// First handle return arrow: ) -> Type { → ) Type {
	if hasArrow {
		transformed, changed := p.transformReturnArrow(result)
		if changed {
			result = transformed
			hadTransformation = true
		}
	}

	// Then handle parameter type annotations: param: Type → param Type
	if hasColon {
		transformed, changed := p.transformParameters(result)
		if changed {
			result = transformed
			hadTransformation = true
		}
	}

	return result, hadTransformation
}

// transformReturnArrow transforms: ) -> Type { to ) Type {
// Also handles generic types: ) -> Result<T,E> { to ) Result[T,E] {
func (p *TypeAnnotASTProcessor) transformReturnArrow(line []byte) ([]byte, bool) {
	// Find ) -> pattern
	arrowIdx := bytes.Index(line, []byte("->"))
	if arrowIdx == -1 {
		return line, false
	}

	// Check for ) before ->
	beforeArrow := line[:arrowIdx]
	parenIdx := bytes.LastIndexByte(beforeArrow, ')')
	if parenIdx == -1 {
		return line, false
	}

	// Find { after ->
	afterArrow := line[arrowIdx+2:]
	braceIdx := bytes.IndexByte(afterArrow, '{')
	if braceIdx == -1 {
		return line, false
	}

	// Extract return type between -> and {
	returnTypeBytes := bytes.TrimSpace(afterArrow[:braceIdx])

	// Parse and transform return type (handles generics: Result<T,E> → Result[T,E])
	transformedReturnType := p.transformTypeString(returnTypeBytes)

	// Build result: everything before ) + ) + space + returnType + space + { + rest
	var buf bytes.Buffer
	buf.Write(line[:parenIdx+1]) // Include the )
	buf.WriteByte(' ')
	buf.Write(transformedReturnType)
	buf.WriteByte(' ')
	buf.WriteByte('{')
	buf.Write(afterArrow[braceIdx+1:])

	return buf.Bytes(), true
}

// transformTypeString transforms a type string, converting <> to [] and underscore names to camelCase
func (p *TypeAnnotASTProcessor) transformTypeString(typeBytes []byte) []byte {
	// Simple transformation: replace < with [ and > with ]
	// This handles the common case of generics like Result<T,E> → Result[T,E]
	result := bytes.ReplaceAll(typeBytes, []byte("<"), []byte("["))
	result = bytes.ReplaceAll(result, []byte(">"), []byte("]"))

	// Transform underscore type names to camelCase
	// Option_int → OptionInt, Result_string_error → ResultStringError
	result = transformUnderscoreType(result)

	return result
}

// transformParameters transforms parameter type annotations in a function signature
func (p *TypeAnnotASTProcessor) transformParameters(line []byte) ([]byte, bool) {
	// Find parameter list: ( ... )
	openParen := bytes.IndexByte(line, '(')
	if openParen == -1 {
		return line, false
	}

	// Find matching close paren
	closeParen := p.findMatchingParen(line, openParen)
	if closeParen == -1 {
		return line, false
	}

	// Extract parameters
	params := line[openParen+1 : closeParen]

	// Check if params contain : pattern (but not inside string literals)
	colonIdx := bytes.IndexByte(params, ':')
	if colonIdx == -1 {
		return line, false
	}

	// Quick check: is this a string literal colon? (basic check)
	// If there's a quote before the colon and another after, skip
	beforeColon := params[:colonIdx]
	if bytes.Contains(beforeColon, []byte(`"`)) {
		return line, false
	}

	// Transform parameters
	transformedParams := p.transformParamList(params)

	// Build result: before ( + ( + transformed params + ) + after )
	var buf bytes.Buffer
	buf.Write(line[:openParen+1])
	buf.Write(transformedParams)
	buf.Write(line[closeParen:])

	return buf.Bytes(), true
}

// findMatchingParen finds the matching closing paren for an opening paren
func (p *TypeAnnotASTProcessor) findMatchingParen(line []byte, openIdx int) int {
	depth := 0
	for i := openIdx; i < len(line); i++ {
		switch line[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// transformParamList transforms a parameter list using token-based parsing
// Handles: x: int, y: string → x int, y string
// Handles: args: ...int → args ...int
// Handles: x: Map<string, List<int>> → x Map[string, List[int]]
// Handles: f: func(int) error → f func(int) error
func (p *TypeAnnotASTProcessor) transformParamList(params []byte) []byte {
	// If params are empty or whitespace only, return as-is
	trimmed := bytes.TrimSpace(params)
	if len(trimmed) == 0 {
		return params
	}

	// Tokenize parameters
	var s scanner.Scanner
	fset := token.NewFileSet()
	file := fset.AddFile("", fset.Base(), len(params))
	s.Init(file, params, nil, 0)

	var result bytes.Buffer
	var tokens []tokenWithPos

	// Collect all tokens (skip auto-inserted semicolons)
	for {
		pos, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}
		// Skip auto-inserted semicolons (scanner adds these with \n literal)
		if tok == token.SEMICOLON {
			continue
		}
		tokens = append(tokens, tokenWithPos{
			pos: pos,
			tok: tok,
			lit: lit,
		})
	}

	// State machine to transform param: Type to param Type
	i := 0
	for i < len(tokens) {
		// Look for pattern: IDENT COLON Type
		if i+2 < len(tokens) &&
		   tokens[i].tok == token.IDENT &&
		   tokens[i+1].tok == token.COLON {

			// Found param: Type pattern
			paramName := tokens[i].lit

			// Write parameter name
			result.WriteString(paramName)
			result.WriteByte(' ')

			// Skip colon (tokens[i+1])

			// Parse and write type (starts at tokens[i+2])
			typeEnd := p.parseType(tokens, i+2, &result)

			i = typeEnd

			// Keep comma if present
			if i < len(tokens) && tokens[i].tok == token.COMMA {
				result.WriteString(", ")
				i++
			}
		} else {
			// Not a param: Type pattern, just copy token
			if tokens[i].lit != "" {
				result.WriteString(tokens[i].lit)
			} else {
				result.WriteString(tokens[i].tok.String())
			}

			// Add space if needed
			if i+1 < len(tokens) && needsSpace(tokens[i], tokens[i+1]) {
				result.WriteByte(' ')
			}

			i++
		}
	}

	return result.Bytes()
}

// tokenWithPos holds token information with position
type tokenWithPos struct {
	pos token.Pos
	tok token.Token
	lit string
}

// parseType parses a type starting at startIdx and writes it to buf
// Returns the index of the next token after the type
//
// Handles:
//   - Basic types: int, string, MyType
//   - Qualified: pkg.Type
//   - Pointers: *Type, **Type
//   - Arrays/Slices: []Type, [10]int
//   - Maps: map[K]V
//   - Channels: chan T, <-chan T, chan<- T
//   - Functions: func(int) error, func(a, b int) (string, error)
//   - Generics: Map<K, V>, Option<T> (angle brackets → square brackets)
//   - Variadic: ...Type
func (p *TypeAnnotASTProcessor) parseType(tokens []tokenWithPos, startIdx int, buf *bytes.Buffer) int {
	i := startIdx

	if i >= len(tokens) {
		return i
	}

	// Handle variadic: ...Type
	if tokens[i].tok == token.ELLIPSIS {
		buf.WriteString("...")
		i++
		if i >= len(tokens) {
			return i
		}
	}

	// Handle pointer: *Type, **Type
	for i < len(tokens) && tokens[i].tok == token.MUL {
		buf.WriteByte('*')
		i++
	}

	// Handle array/slice: []Type, [10]int
	if i < len(tokens) && tokens[i].tok == token.LBRACK {
		buf.WriteByte('[')
		i++

		// Look for size or ]
		for i < len(tokens) && tokens[i].tok != token.RBRACK {
			if tokens[i].lit != "" {
				buf.WriteString(tokens[i].lit)
			} else {
				buf.WriteString(tokens[i].tok.String())
			}
			i++
		}

		if i < len(tokens) && tokens[i].tok == token.RBRACK {
			buf.WriteByte(']')
			i++
		}

		// Parse element type
		return p.parseType(tokens, i, buf)
	}

	// Handle map: map[K]V
	if i < len(tokens) && tokens[i].tok == token.MAP {
		buf.WriteString("map")
		i++

		if i < len(tokens) && tokens[i].tok == token.LBRACK {
			buf.WriteByte('[')
			i++

			// Parse key type
			i = p.parseType(tokens, i, buf)

			if i < len(tokens) && tokens[i].tok == token.RBRACK {
				buf.WriteByte(']')
				i++
			}

			// Parse value type
			return p.parseType(tokens, i, buf)
		}

		return i
	}

	// Handle channel: chan T, <-chan T, chan<- T
	if i < len(tokens) && tokens[i].tok == token.ARROW {
		buf.WriteString("<-")
		i++
	}

	if i < len(tokens) && tokens[i].tok == token.CHAN {
		buf.WriteString("chan")
		i++

		if i < len(tokens) && tokens[i].tok == token.ARROW {
			buf.WriteString("<-")
			i++
		}

		buf.WriteByte(' ')

		// Parse element type
		return p.parseType(tokens, i, buf)
	}

	// Handle func: func(args) returnType
	if i < len(tokens) && tokens[i].tok == token.FUNC {
		buf.WriteString("func")
		i++

		// Parse signature
		if i < len(tokens) && tokens[i].tok == token.LPAREN {
			depth := 1
			buf.WriteByte('(')
			i++

			prevTok := token.LPAREN
			for i < len(tokens) && depth > 0 {
				if tokens[i].tok == token.LPAREN {
					depth++
				} else if tokens[i].tok == token.RPAREN {
					depth--
					if depth == 0 {
						buf.WriteByte(')')
						i++
						break
					}
				}

				// Add spacing for readability
				if needsSpaceInFunc(prevTok, tokens[i].tok) {
					buf.WriteByte(' ')
				}

				if tokens[i].lit != "" {
					buf.WriteString(tokens[i].lit)
				} else {
					buf.WriteString(tokens[i].tok.String())
				}
				prevTok = tokens[i].tok
				i++
			}

			// Parse return type if present
			if i < len(tokens) {
				if tokens[i].tok == token.LPAREN {
					// Multiple return values: (string, error)
					buf.WriteByte(' ')
					depth := 1
					buf.WriteByte('(')
					i++

					prevTok := token.LPAREN
					for i < len(tokens) && depth > 0 {
						if tokens[i].tok == token.LPAREN {
							depth++
						} else if tokens[i].tok == token.RPAREN {
							depth--
							if depth == 0 {
								buf.WriteByte(')')
								i++
								break
							}
						}

						// Add spacing
						if needsSpaceInFunc(prevTok, tokens[i].tok) {
							buf.WriteByte(' ')
						}

						if tokens[i].lit != "" {
							buf.WriteString(tokens[i].lit)
						} else {
							buf.WriteString(tokens[i].tok.String())
						}
						prevTok = tokens[i].tok
						i++
					}
				} else if tokens[i].tok == token.IDENT || tokens[i].tok == token.MUL || tokens[i].tok == token.LBRACK || tokens[i].tok == token.FUNC {
					// Single return value (including nested function types)
					buf.WriteByte(' ')
					return p.parseType(tokens, i, buf)
				}
			}
		}

		return i
	}

	// Handle struct/interface
	if i < len(tokens) && (tokens[i].tok == token.STRUCT || tokens[i].tok == token.INTERFACE) {
		buf.WriteString(tokens[i].lit)
		i++

		if i < len(tokens) && tokens[i].tok == token.LBRACE {
			depth := 1
			buf.WriteByte('{')
			i++

			for i < len(tokens) && depth > 0 {
				if tokens[i].tok == token.LBRACE {
					depth++
				} else if tokens[i].tok == token.RBRACE {
					depth--
				}

				if tokens[i].lit != "" {
					buf.WriteString(tokens[i].lit)
				} else {
					buf.WriteString(tokens[i].tok.String())
				}
				i++
			}
		}

		return i
	}

	// Handle basic type: int, string, MyType, pkg.Type
	if i < len(tokens) && tokens[i].tok == token.IDENT {
		// Transform underscore type names to camelCase
		// Option_int → OptionInt, Result_string_error → ResultStringError
		typeName := transformUnderscoreType([]byte(tokens[i].lit))
		buf.Write(typeName)
		i++

		// Check for qualified type: pkg.Type
		if i < len(tokens) && tokens[i].tok == token.PERIOD {
			buf.WriteByte('.')
			i++

			if i < len(tokens) && tokens[i].tok == token.IDENT {
				// Also transform qualified types
				qualifiedType := transformUnderscoreType([]byte(tokens[i].lit))
				buf.Write(qualifiedType)
				i++
			}
		}

		// Handle generics: Type<T1, T2> → Type[T1, T2]
		if i < len(tokens) && tokens[i].tok == token.LSS {
			buf.WriteByte('[')
			i++

			depth := 1
			for i < len(tokens) && depth > 0 {
				if tokens[i].tok == token.LSS {
					depth++
					buf.WriteByte('[')
					i++
				} else if tokens[i].tok == token.GTR {
					depth--
					if depth > 0 {
						buf.WriteByte(']')
					}
					i++
				} else {
					// Recursively parse type arguments
					if tokens[i].tok == token.COMMA {
						buf.WriteByte(',')
						buf.WriteByte(' ')
						i++
					} else {
						oldLen := buf.Len()
						i = p.parseType(tokens, i, buf)
						// If nothing was written, we've hit an unexpected token
						if buf.Len() == oldLen {
							i++
						}
					}
				}
			}

			buf.WriteByte(']')
		}
	}

	return i
}

// needsSpace returns true if a space is needed between two tokens
func needsSpace(t1, t2 tokenWithPos) bool {
	// Space after comma
	if t1.tok == token.COMMA {
		return true
	}

	// No space before comma, semicolon, closing brackets
	if t2.tok == token.COMMA || t2.tok == token.SEMICOLON ||
	   t2.tok == token.RPAREN || t2.tok == token.RBRACK || t2.tok == token.RBRACE {
		return false
	}

	// No space after opening brackets
	if t1.tok == token.LPAREN || t1.tok == token.LBRACK || t1.tok == token.LBRACE {
		return false
	}

	// No space around period (pkg.Type)
	if t1.tok == token.PERIOD || t2.tok == token.PERIOD {
		return false
	}

	// Space between identifiers
	if t1.tok == token.IDENT && t2.tok == token.IDENT {
		return true
	}

	return false
}

// needsSpaceInFunc returns true if a space is needed in function signature
func needsSpaceInFunc(t1, t2 token.Token) bool {
	// Space after comma: func(a,b int) → func(a, b int)
	if t1 == token.COMMA {
		return true
	}

	// Space between identifiers: func(a b int) → func(a b int)
	if t1 == token.IDENT && t2 == token.IDENT {
		return true
	}

	// Space after ) when followed by func keyword: func() func() error
	if t1 == token.RPAREN && t2 == token.FUNC {
		return true
	}

	// Space after ) when followed by identifier (return type): func() error
	if t1 == token.RPAREN && t2 == token.IDENT {
		return true
	}

	// No space before comma
	if t2 == token.COMMA {
		return false
	}

	// No space after opening paren
	if t1 == token.LPAREN {
		return false
	}

	// No space before closing paren
	if t2 == token.RPAREN {
		return false
	}

	return false
}

// transformUnderscoreType transforms underscore-based type names to camelCase
// Examples:
//   Option_int → OptionInt
//   Result_string_error → ResultStringError
//   Option_bool → OptionBool
//   normal_type → normalType (doesn't start with capital, so stays as-is)
func transformUnderscoreType(typeName []byte) []byte {
	// If no underscore, return as-is
	if !bytes.Contains(typeName, []byte("_")) {
		return typeName
	}

	// Split by underscore
	parts := bytes.Split(typeName, []byte("_"))
	if len(parts) <= 1 {
		return typeName
	}

	// Join parts with each part capitalized
	var result bytes.Buffer
	for _, part := range parts {
		if len(part) == 0 {
			continue
		}
		// Capitalize first letter of each part
		result.Write(capitalize(part))
	}

	return result.Bytes()
}

// capitalize capitalizes the first letter of a byte slice
func capitalize(s []byte) []byte {
	if len(s) == 0 {
		return s
	}

	result := make([]byte, len(s))
	copy(result, s)

	// Capitalize first letter
	if result[0] >= 'a' && result[0] <= 'z' {
		result[0] = result[0] - 'a' + 'A'
	}

	return result
}
