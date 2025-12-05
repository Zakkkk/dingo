// Package parser provides Dingo source file parsing.
// It uses a token-based preprocessing approach:
// 1. Tokenize with Dingo scanner (handles ?, ??, ?., etc.)
// 2. Transform tokens to valid Go tokens
// 3. Reconstruct source code
// 4. Parse with go/parser
//
// This approach is pure AST-based - no regex string manipulation.
package parser

import (
	"fmt"
	"go/ast"
	goparser "go/parser"
	"go/scanner"
	gotoken "go/token"
	"strings"

	"github.com/MadAppGang/dingo/pkg/goparser/token"
)

// Mode controls parser behavior
type Mode uint

const (
	ParseComments Mode = 1 << iota
	Trace
	AllErrors
)

// ParseFile parses a Dingo source file and returns a Go AST.
// It handles Dingo-specific syntax by tokenizing first, then
// transforming to valid Go before parsing.
func ParseFile(fset *gotoken.FileSet, filename string, src []byte, mode Mode) (*ast.File, error) {
	// Step 1: Transform Dingo tokens to Go-compatible source
	goSrc, mappings, err := TransformToGo(src)
	if err != nil {
		return nil, fmt.Errorf("dingo transform: %w", err)
	}

	// Step 2: Parse with standard Go parser
	goMode := goparser.Mode(0)
	if mode&ParseComments != 0 {
		goMode |= goparser.ParseComments
	}
	if mode&AllErrors != 0 {
		goMode |= goparser.AllErrors
	}

	file, err := goparser.ParseFile(fset, filename, goSrc, goMode)
	if err != nil {
		// TODO: Map error positions back to Dingo source
		return nil, err
	}

	// Step 3: Store mappings for source map generation
	_ = mappings // TODO: Use for source maps

	return file, nil
}

// TokenMapping tracks the relationship between Dingo and Go source positions
type TokenMapping struct {
	DingoStart, DingoEnd int // Position in original Dingo source
	GoStart, GoEnd       int // Position in transformed Go source
	Kind                 string // Type of transformation
}

// TransformToGo transforms Dingo source to valid Go source.
// It operates at the token level, not with regex.
//
// Strategy: Walk through source bytes, using scanner to identify tokens,
// and make targeted replacements for Dingo syntax.
func TransformToGo(src []byte) ([]byte, []TokenMapping, error) {
	// First pass: handle characters that Go's scanner sees as ILLEGAL (?, ??, ?.)
	src = transformDingoChars(src)

	var mappings []TokenMapping

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

	// Now process tokens and build output by copying source with modifications
	result := make([]byte, 0, len(src))
	lastCopied := 0 // Last byte position we've copied from src

	// State tracking
	parenDepth := 0
	inParamList := false
	genericDepth := 0 // Track depth of generic type brackets

	for i := 0; i < len(tokens)-1; i++ { // -1 because last is EOF
		t := tokens[i]
		offset := file.Offset(t.pos) // Convert Pos to byte offset

		// Track parentheses for parameter context
		if t.tok == gotoken.LPAREN {
			// Check if previous token suggests parameter list
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
		// Replace '<' with '[' when after an identifier (type name)
		if t.tok == gotoken.LSS { // '<'
			if i > 0 && tokens[i-1].tok == gotoken.IDENT {
				prevLit := tokens[i-1].lit

				// Check if this is a field access (preceded by .)
				// If so, it's NOT a generic type
				isFieldAccess := false
				if i >= 2 && tokens[i-2].tok == gotoken.PERIOD {
					isFieldAccess = true
				}

				// Check if this looks like generic syntax (not comparison)
				// Generic type names typically:
				// 1. Start with uppercase (Result, Option, Map)
				// 2. Are known generic types
				// 3. Are NOT field accesses (x.Field<...)
				isLikelyGeneric := false
				if !isFieldAccess && len(prevLit) > 0 {
					firstChar := prevLit[0]
					// Uppercase letter indicates type name, but only for known patterns
					// Known Dingo generic types
					knownGenerics := map[string]bool{
						"Result": true, "Option": true,
						"Some": true, "None": true, "Ok": true, "Err": true,
					}
					if knownGenerics[prevLit] {
						isLikelyGeneric = true
					} else if firstChar >= 'A' && firstChar <= 'Z' {
						// For other uppercase identifiers, require them to be in a type position
						// Type positions: after func return, after :, after var, etc.
						// For now, only match known generics to be safe
						// This can be expanded later
						isLikelyGeneric = false
					}
				}

				if isLikelyGeneric {
					// Also check that next token looks like a type parameter
					if i+1 < len(tokens) {
						next := tokens[i+1]
						if next.tok == gotoken.IDENT || next.tok == gotoken.MUL || next.tok == gotoken.LBRACK {
							// Copy up to <
							result = append(result, src[lastCopied:offset]...)
							// Replace < with [
							result = append(result, '[')
							lastCopied = offset + 1
							genericDepth++ // Track that we're inside a generic

							mappings = append(mappings, TokenMapping{
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
		}

		// Handle generic closing: > -> ] when matching a generic open
		if t.tok == gotoken.GTR && genericDepth > 0 { // '>' only if inside generic
			// We're inside a generic type, so this > closes it
			// Copy up to >
			result = append(result, src[lastCopied:offset]...)
			// Replace > with ]
			result = append(result, ']')
			lastCopied = offset + 1
			genericDepth-- // Decrement generic depth

			mappings = append(mappings, TokenMapping{
				DingoStart: offset,
				DingoEnd:   offset + 1,
				GoStart:    len(result) - 1,
				GoEnd:      len(result),
				Kind:       "generic_close",
			})
			continue
		}

		// Handle type annotations: param: Type -> param Type
		// Replace ':' with ' ' when in parameter list after identifier
		if t.tok == gotoken.COLON && inParamList {
			if i > 0 && tokens[i-1].tok == gotoken.IDENT {
				// Copy everything up to the colon
				result = append(result, src[lastCopied:offset]...)
				// Replace colon with space
				result = append(result, ' ')
				lastCopied = offset + 1 // Skip the colon

				mappings = append(mappings, TokenMapping{
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
			// Check if next significant token is identifier, then =
			// let x = expr -> x := expr
			if i+2 < len(tokens) {
				next := tokens[i+1]
				afterNext := tokens[i+2]
				if next.tok == gotoken.IDENT && afterNext.tok == gotoken.ASSIGN {
					// Copy up to 'let'
					result = append(result, src[lastCopied:offset]...)
					// Skip 'let ' - we'll let the identifier be copied normally
					// but we need to change = to :=
					lastCopied = offset + len("let") // Skip "let" but keep the space after

					// Skip any whitespace after 'let'
					for lastCopied < len(src) && (src[lastCopied] == ' ' || src[lastCopied] == '\t') {
						lastCopied++
					}

					mappings = append(mappings, TokenMapping{
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
				// Copy up to =
				result = append(result, src[lastCopied:offset]...)
				// Write :=
				result = append(result, ':', '=')
				lastCopied = offset + 1 // Skip the original =

				mappings = append(mappings, TokenMapping{
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

// enumInfo tracks information about an enum for pattern matching
type enumInfo struct {
	name     string
	variants []string
}

// enumRegistry maps variant names to their enum name prefix
var enumRegistry = make(map[string]string)

// transformDingoChars handles characters that Go's scanner sees as ILLEGAL.
// This is a pre-tokenization pass that converts Dingo operators to Go-parseable forms.
//
// Current Transformations:
// - enum: enum Name { Variant } -> Go tagged union interface
// - match: match expr { Pattern => result } -> Go switch/if-else
// - Single ? (error propagation): let x = expr? -> x, err := expr; if err != nil { return err }
// - guard let: guard let x = expr else { ... } -> x, err := expr; if err != nil { ... }
// - Safe nav ?.: x?.field -> nil-checked pointer access
// - Null coalescing ??: a ?? b -> if a != nil { *a } else { b }
// - Fat arrow => (lambda): (x) => expr -> func(x) { return expr }
// - Rust pipe lambda: |x| expr -> func(x) { return expr }
func transformDingoChars(src []byte) []byte {
	// Reset registry for each file
	enumRegistry = make(map[string]string)

	// First pass: transform enums (must be before match since match uses enum types)
	// This also populates enumRegistry for match to use
	src = transformEnum(src)
	// Second pass: transform match expressions
	src = transformMatch(src)
	// Third pass: transform enum constructor calls (EventUserCreated(...) -> NewEventUserCreated(...))
	src = transformEnumConstructors(src)
	// Fourth pass: transform error propagation (before guard let since both use ?)
	src = transformErrorProp(src)
	// Fifth pass: transform guard let (before lambdas since it uses |err|)
	src = transformGuardLet(src)
	// Sixth pass: transform safe nav and null coalesce at statement level (human-like code)
	src = transformSafeNavStatements(src)
	// Seventh pass: transform safe navigation (IIFE fallback for complex expressions)
	src = transformSafeNav(src)
	// Eighth pass: transform null coalescing (IIFE fallback for complex expressions)
	src = transformNullCoalesce(src)
	// Ninth pass: transform lambdas (more complex patterns)
	src = transformLambdas(src)

	result := make([]byte, 0, len(src)+100) // Extra space for markers

	i := 0
	inString := false
	inRawString := false
	stringChar := byte(0)

	for i < len(src) {
		ch := src[i]

		// Track string literals - don't transform inside strings
		if !inString && !inRawString {
			if ch == '"' {
				inString = true
				stringChar = '"'
			} else if ch == '\'' {
				inString = true
				stringChar = '\''
			} else if ch == '`' {
				inRawString = true
			}
		} else if inString {
			if ch == stringChar && (i == 0 || src[i-1] != '\\') {
				inString = false
			}
		} else if inRawString {
			if ch == '`' {
				inRawString = false
			}
		}

		// Only transform ? outside of strings
		// Note: Single ?, ??, and ?. are now handled by dedicated transformation functions
		// (transformErrorProp, transformNullCoalesce, transformSafeNav)
		// This section is kept for backward compatibility but should be empty
		if !inString && !inRawString && ch == '?' {
			// All ? variants should already be transformed by earlier passes
			i++
			continue
		}
		result = append(result, ch)
		i++
	}

	return result
}

// transformSafeNavStatements transforms safe navigation (?.) and null coalescing (??)
// at statement level to generate human-like Go code.
//
// Transforms:
//   return user?.name ?? "Anonymous"
// Into:
//   if user != nil {
//       return user.Name
//   }
//   return "Anonymous"
//
// Transforms:
//   let x = user?.name ?? "Anonymous"
// Into:
//   var x string
//   if user != nil {
//       x = user.Name
//   } else {
//       x = "Anonymous"
//   }
func transformSafeNavStatements(src []byte) []byte {
	result := make([]byte, 0, len(src)+500)

	i := 0
	inString := false
	inRawString := false
	stringChar := byte(0)

	for i < len(src) {
		ch := src[i]

		// Track string literals - don't transform inside strings
		if !inString && !inRawString {
			if ch == '"' {
				inString = true
				stringChar = '"'
			} else if ch == '\'' {
				inString = true
				stringChar = '\''
			} else if ch == '`' {
				inRawString = true
			}
		} else if inString {
			if ch == stringChar && (i == 0 || src[i-1] != '\\') {
				inString = false
			}
		} else if inRawString {
			if ch == '`' {
				inRawString = false
			}
		}

		// Look for "return" keyword (only outside strings)
		if !inString && !inRawString && i+6 < len(src) && string(src[i:i+6]) == "return" {
			// Check if followed by whitespace
			if i+6 < len(src) && (src[i+6] == ' ' || src[i+6] == '\t') {
				j := i + 6
				// Skip whitespace
				for j < len(src) && (src[j] == ' ' || src[j] == '\t') {
					j++
				}

				// Parse the expression to check if it contains ?. or ??
				exprStart := j
				hasSafeNav := false
				hasNullCoalesce := false
				parenDepth := 0

				for j < len(src) && (parenDepth > 0 || (src[j] != '\n' && src[j] != ';')) {
					if src[j] == '(' {
						parenDepth++
					} else if src[j] == ')' {
						parenDepth--
					} else if parenDepth == 0 {
						if j+1 < len(src) && src[j] == '?' && src[j+1] == '.' {
							hasSafeNav = true
						}
						if j+1 < len(src) && src[j] == '?' && src[j+1] == '?' {
							hasNullCoalesce = true
						}
					}
					j++
				}

				if hasSafeNav || hasNullCoalesce {
					expr := string(src[exprStart:j])
					expr = trimRight(expr)

					// Preserve indentation
					indent := ""
					lineStart := i
					for lineStart > 0 && src[lineStart-1] != '\n' {
						lineStart--
					}
					for k := lineStart; k < i && (src[k] == ' ' || src[k] == '\t'); k++ {
						indent += string(src[k])
					}

					// Transform the return statement
					transformed := transformReturnSafeNavExpr(expr, indent)
					result = append(result, transformed...)

					// Skip past the newline/semicolon that ended the expression
					if j < len(src) && (src[j] == '\n' || src[j] == ';') {
						j++
					}
					i = j
					continue
				}
			}
		}

		// Look for "let" keyword (only outside strings)
		if !inString && !inRawString && i+3 < len(src) && string(src[i:i+3]) == "let" {
			// Check if followed by whitespace
			if i+3 < len(src) && (src[i+3] == ' ' || src[i+3] == '\t') {
				j := i + 3
				// Skip whitespace
				for j < len(src) && (src[j] == ' ' || src[j] == '\t') {
					j++
				}

				// Get variable name
				varStart := j
				for j < len(src) && (isAlphaNum(src[j]) || src[j] == '_') {
					j++
				}
				varName := string(src[varStart:j])

				// Skip whitespace
				for j < len(src) && (src[j] == ' ' || src[j] == '\t') {
					j++
				}

				// Expect '='
				if j < len(src) && src[j] == '=' {
					j++
					// Skip whitespace
					for j < len(src) && (src[j] == ' ' || src[j] == '\t') {
						j++
					}

					// Parse expression
					exprStart := j
					hasSafeNav := false
					hasNullCoalesce := false
					parenDepth := 0

					for j < len(src) && (parenDepth > 0 || (src[j] != '\n' && src[j] != ';')) {
						if src[j] == '(' {
							parenDepth++
						} else if src[j] == ')' {
							parenDepth--
						} else if parenDepth == 0 {
							if j+1 < len(src) && src[j] == '?' && src[j+1] == '.' {
								hasSafeNav = true
							}
							if j+1 < len(src) && src[j] == '?' && src[j+1] == '?' {
								hasNullCoalesce = true
							}
						}
						j++
					}

					if hasSafeNav || hasNullCoalesce {
						expr := string(src[exprStart:j])
						expr = trimRight(expr)

						// Preserve indentation
						indent := ""
						lineStart := i
						for lineStart > 0 && src[lineStart-1] != '\n' {
							lineStart--
						}
						for k := lineStart; k < i && (src[k] == ' ' || src[k] == '\t'); k++ {
							indent += string(src[k])
						}

						// Transform the let statement
						transformed := transformLetSafeNavExpr(varName, expr, indent)
						result = append(result, transformed...)

						// Skip past the newline/semicolon that ended the expression
						if j < len(src) && (src[j] == '\n' || src[j] == ';') {
							j++
						}
						i = j
						continue
					}
				}
			}
		}

		result = append(result, ch)
		i++
	}

	return result
}

// transformReturnSafeNavExpr transforms a return statement with ?. or ?? into human-like code
func transformReturnSafeNavExpr(expr, indent string) string {
	// Parse the expression to extract components
	base, chain, coalesceDefault := parseSafeNavChain(expr)

	if base == "" {
		// Couldn't parse, return original
		return indent + "return " + expr
	}

	var buf strings.Builder

	// Generate nested if checks for each element in the chain
	currentIndent := indent
	currentPath := base

	for i, elem := range chain {
		buf.WriteString(currentIndent)
		buf.WriteString("if ")

		// Build the nil check path
		if i == 0 {
			buf.WriteString(base)
		} else {
			buf.WriteString(currentPath)
		}
		buf.WriteString(" != nil {\n")

		currentIndent += "\t"

		// Update current path
		if elem.isCall {
			currentPath += "." + elem.name + elem.args
		} else {
			currentPath += "." + strings.ToUpper(elem.name[:1]) + elem.name[1:] // Capitalize field
		}
	}

	// Generate the return statement
	buf.WriteString(currentIndent)
	buf.WriteString("return ")
	buf.WriteString(currentPath)
	buf.WriteString("\n")

	// Close all if blocks
	for range chain {
		currentIndent = currentIndent[:len(currentIndent)-1]
		buf.WriteString(currentIndent)
		buf.WriteString("}\n")
	}

	// Generate default return if coalesce exists
	buf.WriteString(indent)
	buf.WriteString("return ")
	if coalesceDefault != "" {
		buf.WriteString(coalesceDefault)
	} else {
		buf.WriteString("nil")
	}

	return buf.String()
}

// transformLetSafeNavExpr transforms a let statement with ?. or ?? into human-like code
func transformLetSafeNavExpr(varName, expr, indent string) string {
	// Parse the expression to extract components
	base, chain, coalesceDefault := parseSafeNavChain(expr)

	if base == "" {
		// Couldn't parse, return original
		return indent + "let " + varName + " = " + expr
	}

	var buf strings.Builder

	// Infer type from default value
	varType := inferTypeFromLiteral(coalesceDefault)

	// Generate variable declaration
	buf.WriteString(indent)
	buf.WriteString("var ")
	buf.WriteString(varName)
	if varType != "" && coalesceDefault != "" {
		buf.WriteString(" ")
		buf.WriteString(varType)
	}
	buf.WriteString("\n")

	// Generate nested if checks
	currentIndent := indent
	currentPath := base

	for i, elem := range chain {
		buf.WriteString(currentIndent)
		buf.WriteString("if ")

		// Build the nil check path
		if i == 0 {
			buf.WriteString(base)
		} else {
			buf.WriteString(currentPath)
		}
		buf.WriteString(" != nil {\n")

		currentIndent += "\t"

		// Update current path
		if elem.isCall {
			currentPath += "." + elem.name + elem.args
		} else {
			currentPath += "." + strings.ToUpper(elem.name[:1]) + elem.name[1:] // Capitalize field
		}
	}

	// Generate the assignment
	buf.WriteString(currentIndent)
	buf.WriteString(varName)
	buf.WriteString(" = ")
	buf.WriteString(currentPath)
	buf.WriteString("\n")

	// Close if blocks
	for i := range chain {
		currentIndent = currentIndent[:len(currentIndent)-1]
		buf.WriteString(currentIndent)
		buf.WriteString("}")

		// Add else clause only for the outermost if (last iteration) and if we have a default
		if i == len(chain)-1 && coalesceDefault != "" {
			buf.WriteString(" else {\n")
			buf.WriteString(currentIndent)
			buf.WriteString("\t")
			buf.WriteString(varName)
			buf.WriteString(" = ")
			buf.WriteString(coalesceDefault)
			buf.WriteString("\n")
			buf.WriteString(currentIndent)
			buf.WriteString("}")
		}
		buf.WriteString("\n")
	}

	return buf.String()
}

// chainElement represents a field or method in a safe nav chain
type chainElement struct {
	name   string
	isCall bool
	args   string
}

// parseSafeNavChain parses expressions like "user?.name ?? 'default'" into components
func parseSafeNavChain(expr string) (base string, chain []chainElement, coalesceDefault string) {
	// First, check for ?? to separate safe nav from coalesce
	parts := strings.Split(expr, "??")
	mainExpr := strings.TrimSpace(parts[0])
	if len(parts) > 1 {
		coalesceDefault = strings.TrimSpace(parts[1])
	}

	// Parse the safe nav chain
	i := 0
	for i < len(mainExpr) && (isAlpha(byte(mainExpr[i])) || mainExpr[i] == '_') {
		i++
	}
	base = mainExpr[:i]

	// Parse the chain elements
	for i < len(mainExpr) {
		// Skip whitespace
		for i < len(mainExpr) && (mainExpr[i] == ' ' || mainExpr[i] == '\t') {
			i++
		}

		// Look for ?.
		if i+1 < len(mainExpr) && mainExpr[i] == '?' && mainExpr[i+1] == '.' {
			i += 2 // Skip ?.

			// Skip whitespace
			for i < len(mainExpr) && (mainExpr[i] == ' ' || mainExpr[i] == '\t') {
				i++
			}

			// Get field/method name
			nameStart := i
			for i < len(mainExpr) && (isAlpha(byte(mainExpr[i])) || isDigit(byte(mainExpr[i])) || mainExpr[i] == '_') {
				i++
			}

			if i > nameStart {
				name := mainExpr[nameStart:i]

				// Skip whitespace
				for i < len(mainExpr) && (mainExpr[i] == ' ' || mainExpr[i] == '\t') {
					i++
				}

				// Check if it's a method call
				elem := chainElement{name: name}
				if i < len(mainExpr) && mainExpr[i] == '(' {
					elem.isCall = true
					argStart := i
					parenDepth := 1
					i++ // Skip opening (
					for i < len(mainExpr) && parenDepth > 0 {
						if mainExpr[i] == '(' {
							parenDepth++
						} else if mainExpr[i] == ')' {
							parenDepth--
						}
						i++
					}
					elem.args = mainExpr[argStart:i]
				}

				chain = append(chain, elem)
			}
		} else {
			i++
		}
	}

	return base, chain, coalesceDefault
}

// transformErrorProp transforms error propagation with ? operator:
//   let varName = expr? -> varName, err := expr\nif err != nil { return err }
//   let _ = expr? -> _, err := expr\nif err != nil { return err }
// This must run BEFORE transformGuardLet since both use error handling patterns
func transformErrorProp(src []byte) []byte {
	result := make([]byte, 0, len(src)+500)
	errCounter := 0 // Counter for unique error variable names

	i := 0
	inString := false
	inRawString := false
	stringChar := byte(0)

	for i < len(src) {
		ch := src[i]

		// Track string literals - don't transform inside strings
		if !inString && !inRawString {
			if ch == '"' {
				inString = true
				stringChar = '"'
			} else if ch == '\'' {
				inString = true
				stringChar = '\''
			} else if ch == '`' {
				inRawString = true
			}
		} else if inString {
			if ch == stringChar && (i == 0 || src[i-1] != '\\') {
				inString = false
			}
		} else if inRawString {
			if ch == '`' {
				inRawString = false
			}
		}

		// Look for "let" keyword (only outside strings)
		if !inString && !inRawString && i+3 < len(src) && string(src[i:i+3]) == "let" {
			// Check if followed by whitespace
			if i+3 < len(src) && (src[i+3] == ' ' || src[i+3] == '\t') {
				// Found "let", parse the rest
				j := i + 3
				// Skip whitespace
				for j < len(src) && (src[j] == ' ' || src[j] == '\t') {
					j++
				}

				// Get variable name (could be _ or identifier)
				varStart := j
				for j < len(src) && (isAlphaNum(src[j]) || src[j] == '_') {
					j++
				}
				varName := string(src[varStart:j])

				// Skip whitespace
				for j < len(src) && (src[j] == ' ' || src[j] == '\t') {
					j++
				}

				// Expect '='
				if j < len(src) && src[j] == '=' {
					j++
					// Skip whitespace
					for j < len(src) && (src[j] == ' ' || src[j] == '\t') {
						j++
					}

					// Parse expression until we find ? or newline/semicolon
					exprStart := j
					foundQuestion := false
					parenDepth := 0
					for j < len(src) {
						if src[j] == '(' {
							parenDepth++
						} else if src[j] == ')' {
							parenDepth--
						} else if parenDepth == 0 && src[j] == '?' {
							// Check if it's ?. or ?? (safe navigation/null coalescing)
							// If so, skip this - it will be handled by transformSafeNavStatements
							if j+1 < len(src) && (src[j+1] == '.' || src[j+1] == '?') {
								// This is ?. or ??, not error propagation
								// Skip both characters
								j += 2
								continue
							}
							// Check it's not inside a string
							// (we're not tracking strings in this inner loop for simplicity)
							foundQuestion = true
							break
						} else if parenDepth == 0 && (src[j] == '\n' || src[j] == ';') {
							break
						}
						j++
					}

					if foundQuestion {
						// Extract expression (before ?)
						expr := string(src[exprStart:j])
						expr = trimRight(expr)

						// Preserve indentation from original line
						indent := ""
						lineStart := i
						for lineStart > 0 && src[lineStart-1] != '\n' {
							lineStart--
						}
						for k := lineStart; k < i && (src[k] == ' ' || src[k] == '\t'); k++ {
							indent += string(src[k])
						}

						// Generate error variable name
						var errVar string
						if errCounter == 0 {
							errVar = "err"
						} else {
							errVar = fmt.Sprintf("err%d", errCounter)
						}
						errCounter++

						// Generate temporary variable name
						var tmpVar string
						if errCounter == 1 {
							tmpVar = "tmp"
						} else {
							tmpVar = fmt.Sprintf("tmp%d", errCounter-1)
						}

						// Generate Go code:
						// tmpVar, errVar := expr
						//
						// if errVar != nil {
						//     return errVar
						// }
						// var varName = tmpVar
						result = append(result, tmpVar...)
						result = append(result, ", "...)
						result = append(result, errVar...)
						result = append(result, " := "...)
						result = append(result, expr...)
						result = append(result, '\n')
						result = append(result, '\n')
						result = append(result, indent...)
						result = append(result, "if "...)
						result = append(result, errVar...)
						result = append(result, " != nil {\n"...)
						result = append(result, indent...)
						result = append(result, "\treturn "...)
						result = append(result, errVar...)
						result = append(result, '\n')
						result = append(result, indent...)
						result = append(result, "}\n"...)
						result = append(result, indent...)
						result = append(result, "var "...)
						result = append(result, varName...)
						result = append(result, " = "...)
						result = append(result, tmpVar...)

						// Move past the ?
						i = j + 1
						continue
					}
				}
			}
		}

		result = append(result, ch)
		i++
	}

	return result
}

// transformGuardLet transforms guard let statements:
//   guard let x = expr else |err| { return err }
// To:
//   x, err := expr
//   if err != nil { return err }
func transformGuardLet(src []byte) []byte {
	result := make([]byte, 0, len(src)+200)

	i := 0
	for i < len(src) {
		// Look for "guard" keyword
		if i+5 < len(src) && string(src[i:i+5]) == "guard" {
			// Check if followed by whitespace and "let"
			j := i + 5
			for j < len(src) && (src[j] == ' ' || src[j] == '\t') {
				j++
			}
			if j+3 < len(src) && string(src[j:j+3]) == "let" {
				// Found "guard let", parse the rest
				j += 3
				// Skip whitespace
				for j < len(src) && (src[j] == ' ' || src[j] == '\t') {
					j++
				}

				// Get variable name
				varStart := j
				for j < len(src) && (isAlphaNum(src[j]) || src[j] == '_') {
					j++
				}
				varName := string(src[varStart:j])

				// Skip whitespace
				for j < len(src) && (src[j] == ' ' || src[j] == '\t') {
					j++
				}

				// Expect '='
				if j < len(src) && src[j] == '=' {
					j++
					// Skip whitespace
					for j < len(src) && (src[j] == ' ' || src[j] == '\t') {
						j++
					}

					// Find "else" keyword
					exprStart := j
					elsePos := -1
					for k := j; k+4 < len(src); k++ {
						if string(src[k:k+4]) == "else" && (k == 0 || !isAlphaNum(src[k-1])) && (k+4 >= len(src) || !isAlphaNum(src[k+4])) {
							elsePos = k
							break
						}
					}

					if elsePos > 0 {
						// Extract expression (before "else")
						expr := string(src[exprStart:elsePos])
						expr = trimRight(expr)

						// Move past "else"
						j = elsePos + 4
						// Skip whitespace
						for j < len(src) && (src[j] == ' ' || src[j] == '\t') {
							j++
						}

						// Check for |err| pattern
						errVar := "err"
						if j < len(src) && src[j] == '|' {
							j++
							errStart := j
							for j < len(src) && src[j] != '|' {
								j++
							}
							if j < len(src) {
								errVar = string(src[errStart:j])
								j++ // skip closing |
							}
						}

						// Skip whitespace
						for j < len(src) && (src[j] == ' ' || src[j] == '\t') {
							j++
						}

						// Find the else block
						if j < len(src) && src[j] == '{' {
							// Find matching }
							braceDepth := 1
							blockStart := j
							j++
							for j < len(src) && braceDepth > 0 {
								if src[j] == '{' {
									braceDepth++
								} else if src[j] == '}' {
									braceDepth--
								}
								j++
							}
							elseBlock := string(src[blockStart:j])

							// Generate Go code
							// varName, errVar := expr
							// if errVar != nil elseBlock
							result = append(result, varName...)
							result = append(result, ", "...)
							result = append(result, errVar...)
							result = append(result, " := "...)
							result = append(result, expr...)
							result = append(result, '\n')
							result = append(result, "if "...)
							result = append(result, errVar...)
							result = append(result, " != nil "...)
							result = append(result, elseBlock...)

							i = j
							continue
						}
					}
				}
			}
		}

		result = append(result, src[i])
		i++
	}

	return result
}

func isAlphaNum(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_'
}

func trimRight(s string) string {
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	return s
}

// transformNullCoalesce transforms null coalescing operator (??):
//   a ?? b -> if a != nil { use *a } else { use b }
//
// Handles:
// - Simple coalescing: value ?? "default"
// - Chained coalescing: a ?? b ?? c (left-to-right evaluation)
// - Works with safe navigation results: user?.name ?? "Anonymous"
//
// The left-hand side is assumed to be a pointer type (*T).
// The right-hand side provides the default value of type T.
//
// Phase 1 Limitation: Results use interface{} instead of inferred types.
// This is a temporary limitation until type inference is implemented in Phase 2.
//
// Phase 2 Plan: Implement type inference to generate typed results (string, int, etc.)
// based on right-hand side literals or explicit type annotations.
//
// Example transformation:
//   Input:  user?.name ?? "default"
//   Output: func() interface{} { if safeNav != nil { return *safeNav } else { return "default" } }()
func transformNullCoalesce(src []byte) []byte {
	result := make([]byte, 0, len(src)+500)

	i := 0
	inString := false
	inRawString := false
	stringChar := byte(0)

	for i < len(src) {
		ch := src[i]

		// Track string literals - don't transform inside strings
		if !inString && !inRawString {
			if ch == '"' {
				inString = true
				stringChar = '"'
			} else if ch == '\'' {
				inString = true
				stringChar = '\''
			} else if ch == '`' {
				inRawString = true
			}
		} else if inString {
			if ch == stringChar && (i == 0 || src[i-1] != '\\') {
				inString = false
			}
			result = append(result, ch)
			i++
			continue
		} else if inRawString {
			if ch == '`' {
				inRawString = false
			}
			result = append(result, ch)
			i++
			continue
		}

		// Look for ?? operator (only outside strings)
		if !inString && !inRawString && ch == '?' && i+1 < len(src) && src[i+1] == '?' {
			// Found ??, need to find the left and right expressions

			// Parse left-hand side by scanning backwards in the result buffer
			leftEnd := len(result) - 1
			// Skip whitespace backwards
			for leftEnd >= 0 && (result[leftEnd] == ' ' || result[leftEnd] == '\t') {
				leftEnd--
			}

			// Find start of left expression
			leftStart := leftEnd
			depth := 0
			for leftStart >= 0 {
				c := result[leftStart]

				// Handle closing delimiters
				if c == ')' || c == ']' || c == '}' {
					depth++
				} else if c == '(' || c == '[' || c == '{' {
					depth--
					if depth < 0 {
						leftStart++ // Don't include the opening delimiter
						break
					}
				}

				// At depth 0, check for expression boundaries
				if depth == 0 {
					// Stop at assignment operators
					if c == '=' {
						if leftStart > 0 && result[leftStart-1] == ':' {
							// := operator
							leftStart++
							break
						}
						leftStart++
						break
					}
					// Stop at other boundaries
					if c == ',' || c == ';' || c == '\n' {
						leftStart++
						break
					}
				}

				leftStart--
			}

			if leftStart < 0 {
				leftStart = 0
			}

			// Extract left expression from result buffer
			leftExpr := string(result[leftStart:])
			leftExpr = trim(leftExpr)

			// Skip ?? and whitespace
			j := i + 2
			for j < len(src) && (src[j] == ' ' || src[j] == '\t') {
				j++
			}

			// Parse right-hand side by scanning forward
			rightStart := j
			depth = 0
			inRightString := false
			rightStringChar := byte(0)

			for j < len(src) {
				c := src[j]

				// Track strings in right expression
				if !inRightString {
					if c == '"' || c == '\'' {
						inRightString = true
						rightStringChar = c
					} else if c == '`' {
						inRightString = true
						rightStringChar = '`'
					}
				} else {
					if c == rightStringChar && (j == 0 || src[j-1] != '\\') {
						inRightString = false
					}
					j++
					continue
				}

				// Handle depth tracking
				if c == '(' || c == '[' || c == '{' {
					depth++
				} else if c == ')' || c == ']' || c == '}' {
					if depth == 0 {
						break
					}
					depth--
				}

				// At depth 0, check for expression end
				if depth == 0 && !inRightString {
					if c == ',' || c == ';' || c == '\n' {
						break
					}
					// Check for another ?? (chained coalescing)
					if c == '?' && j+1 < len(src) && src[j+1] == '?' {
						break
					}
				}

				j++
			}

			rightExpr := string(src[rightStart:j])
			rightExpr = trim(rightExpr)

			// Remove left expression from result
			result = result[:leftStart]

			// Generate Go code using IIFE pattern for inline evaluation:
			// func() T { if leftExpr != nil { return *leftExpr } else { return rightExpr } }()
			// TODO(Phase 2): Infer return type from right-hand side expression
			// (string literal → string, int literal → int, etc.)
			result = append(result, "func() interface{} { if "...)
			result = append(result, leftExpr...)
			result = append(result, " != nil { return *"...)
			result = append(result, leftExpr...)
			result = append(result, " } else { return "...)
			result = append(result, rightExpr...)
			result = append(result, " } }()"...)

			i = j
			continue
		}

		result = append(result, ch)
		i++
	}

	return result
}


// transformSafeNav transforms safe navigation operator (?.) with character-level transformation.
// Transforms: user?.address?.city to nested nil checks with pointer results
// Supports method calls: user?.getName("arg")
//
// Phase 1 Limitation: Results use interface{} instead of typed pointers.
// This is a temporary limitation until type inference is implemented in Phase 2.
// Generated code will require type assertions at usage sites.
//
// Phase 2 Plan: Implement type inference to generate typed pointers (*string, *int, etc.)
// based on explicit type annotations (let x: string = user?.name) or contextual inference.
//
// Examples:
//   user?.name -> var safeNav interface{}; if user != nil { tmp := user.name; safeNav = &tmp }
//   user?.getName() -> var safeNav interface{}; if user != nil { tmp := user.getName(); safeNav = &tmp }
//   user?.address?.city -> nested nil checks with intermediate pointer variables
func transformSafeNav(src []byte) []byte {
	result := make([]byte, 0, len(src)+500)

	i := 0
	inString := false
	inRawString := false
	stringChar := byte(0)

	for i < len(src) {
		ch := src[i]

		// Track string literals - don't transform inside strings
		if !inString && !inRawString {
			if ch == '"' {
				inString = true
				stringChar = '"'
			} else if ch == '\'' {
				inString = true
				stringChar = '\''
			} else if ch == '`' {
				inRawString = true
			}
		} else if inString {
			if ch == stringChar && (i == 0 || src[i-1] != '\\') {
				inString = false
			}
			result = append(result, ch)
			i++
			continue
		} else if inRawString {
			if ch == '`' {
				inRawString = false
			}
			result = append(result, ch)
			i++
			continue
		}

		// Look for identifier followed by ?. (outside strings)
		if !inString && !inRawString && isAlpha(ch) {
			// Check if this starts a safe nav chain
			j := i
			for j < len(src) && isAlphaNum(src[j]) {
				j++
			}
			baseIdent := string(src[i:j])

			// Skip whitespace
			k := j
			for k < len(src) && (src[k] == ' ' || src[k] == '\t') {
				k++
			}

			// Check if followed by ?.
			if k+1 < len(src) && src[k] == '?' && src[k+1] == '.' {
				// Found safe nav! Parse the entire chain
				type chainElement struct {
					name   string
					isCall bool
					args   string
				}
				var chain []chainElement
				chain = append(chain, chainElement{name: baseIdent, isCall: false, args: ""})

				// Move past the identifier
				j = k + 2 // Skip ?.

				// Parse the chain
				for {
					// Skip whitespace
					for j < len(src) && (src[j] == ' ' || src[j] == '\t') {
						j++
					}

					// Get the next identifier or method name
					if j >= len(src) || !isAlpha(src[j]) {
						break
					}

					nameStart := j
					for j < len(src) && isAlphaNum(src[j]) {
						j++
					}
					name := string(src[nameStart:j])

					// Skip whitespace
					for j < len(src) && (src[j] == ' ' || src[j] == '\t') {
						j++
					}

					// Check if it's a method call
					elem := chainElement{name: name, isCall: false, args: ""}
					if j < len(src) && src[j] == '(' {
						// It's a method call - parse arguments
						elem.isCall = true
						argStart := j
						parenDepth := 1
						j++ // Skip opening (
						for j < len(src) && parenDepth > 0 {
							if src[j] == '(' {
								parenDepth++
							} else if src[j] == ')' {
								parenDepth--
							}
							j++
						}
						elem.args = string(src[argStart:j]) // Include parentheses
					}

					chain = append(chain, elem)

					// Skip whitespace
					for j < len(src) && (src[j] == ' ' || src[j] == '\t') {
						j++
					}

					// Check if there's another ?.
					if j+1 < len(src) && src[j] == '?' && src[j+1] == '.' {
						j += 2 // Skip ?.
						continue
					}

					// No more ?. in the chain
					break
				}

				// Generate code for the safe nav chain using IIFE pattern
				if len(chain) > 1 {
					// Use IIFE pattern (Immediately Invoked Function Expression)
					// This produces an expression, not statements, so it can appear
					// in any expression context (return statements, assignments, function calls)
					//
					// Example transformation:
					//   user?.Name → func() interface{} { if user != nil { tmp := user.Name; return &tmp }; return nil }()
					//
					// For simplicity, use interface{} type initially (Phase 1)
					// TODO(Phase 2): Replace interface{} with inferred pointer type (*string, *int, etc.)
					// based on type annotations or context analysis
					result = append(result, "func() interface{} { "...)

					// Build the if conditions and assignments
					tmpCounter := 1 // Start at 1 per CLAUDE.md convention
					currentVar := chain[0].name

					result = append(result, "if "...)
					result = append(result, currentVar...)
					result = append(result, " != nil { "...)

					for idx := 1; idx < len(chain); idx++ {
						elem := chain[idx]

						// Build the access expression
						var accessExpr string
						if elem.isCall {
							// Method call
							accessExpr = currentVar + "." + elem.name + elem.args
						} else {
							// Field access
							accessExpr = currentVar + "." + elem.name
						}

						if idx == len(chain)-1 {
							// Last element - return pointer to value
							result = append(result, "tmp := "...)
							result = append(result, accessExpr...)
							result = append(result, "; return &tmp "...)
						} else {
							// Intermediate element - No-Number-First Pattern: tmp, tmp1, tmp2
							var tmpVar string
							if tmpCounter == 1 {
								tmpVar = "tmp"
							} else {
								tmpVar = fmt.Sprintf("tmp%d", tmpCounter-1)
							}
							tmpCounter++

							result = append(result, tmpVar...)
							result = append(result, " := "...)
							result = append(result, accessExpr...)
							result = append(result, "; "...)

							// If next element is a call or field, we need nil check for pointer results
							result = append(result, "if "...)
							result = append(result, tmpVar...)
							result = append(result, " != nil { "...)

							currentVar = tmpVar
						}
					}

					// Close all the if blocks
					for idx := 1; idx < len(chain); idx++ {
						result = append(result, "} "...)
					}

					// Return nil if any part of chain was nil
					result = append(result, "; return nil }()"...)

					// Move past the chain
					i = j
					continue
				}
			}

			// Not a safe nav - just copy the identifier
			result = append(result, src[i:j]...)
			i = j
			continue
		}

		result = append(result, ch)
		i++
	}

	return result
}

// transformEnumConstructors transforms enum variant constructor calls:
//   EventUserCreated(1, "alice") -> NewEventUserCreated(1, "alice")
// Uses enumRegistry to identify enum variant names
func transformEnumConstructors(src []byte) []byte {
	result := make([]byte, 0, len(src)+100)

	i := 0
	inString := false
	inRawString := false
	stringChar := byte(0)

	for i < len(src) {
		ch := src[i]

		// Track string literals
		if !inString && !inRawString {
			if ch == '"' {
				inString = true
				stringChar = '"'
			} else if ch == '\'' {
				inString = true
				stringChar = '\''
			} else if ch == '`' {
				inRawString = true
			}
		} else if inString {
			if ch == stringChar && (i == 0 || src[i-1] != '\\') {
				inString = false
			}
			result = append(result, ch)
			i++
			continue
		} else if inRawString {
			if ch == '`' {
				inRawString = false
			}
			result = append(result, ch)
			i++
			continue
		}

		// Look for identifier followed by (
		if isAlpha(ch) {
			idStart := i
			for i < len(src) && isAlphaNum(src[i]) {
				i++
			}
			ident := string(src[idStart:i])

			// Skip whitespace
			j := i
			for j < len(src) && (src[j] == ' ' || src[j] == '\t') {
				j++
			}

			// Check if this is a function call (followed by '(')
			// and if the identifier is in our enum registry
			if j < len(src) && src[j] == '(' {
				if _, ok := enumRegistry[ident]; ok {
					// This is an enum constructor call
					// If ident is "EventUserCreated", output "NewEventUserCreated"
					result = append(result, "New"...)
					result = append(result, ident...)
					continue
				}
			}

			// Not an enum constructor, just copy the identifier
			result = append(result, src[idStart:i]...)
			continue
		}

		result = append(result, ch)
		i++
	}

	return result
}

// isAlpha checks if a character is a letter or underscore
func isAlpha(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_'
}

func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}

func trim(s string) string {
	// Trim leading whitespace including newlines and tabs
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t' || s[0] == '\n') {
		s = s[1:]
	}
	// Trim trailing whitespace
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t' || s[len(s)-1] == '\n') {
		s = s[:len(s)-1]
	}
	return s
}

// transformLambdaParams transforms lambda parameters to Go function parameters.
// Handles type annotations: |u: User| or (x: int, y: int)
// Transforms to: u User or x int, y int
func transformLambdaParams(params string) string {
	// Split on commas (respecting nesting)
	paramList := splitFields(params)
	var result []string

	for _, p := range paramList {
		p = trim(p)
		if p == "" {
			continue
		}

		// Check if parameter has type annotation (name: Type)
		colonIdx := -1
		depth := 0
		for i := 0; i < len(p); i++ {
			if p[i] == '<' || p[i] == '[' || p[i] == '(' {
				depth++
			} else if p[i] == '>' || p[i] == ']' || p[i] == ')' {
				depth--
			} else if p[i] == ':' && depth == 0 {
				colonIdx = i
				break
			}
		}

		if colonIdx > 0 {
			// Has type annotation: name: Type
			name := trim(p[:colonIdx])
			typ := trim(p[colonIdx+1:])
			result = append(result, name+" "+typ)
		} else {
			// No type annotation - keep as-is (will need type inference later)
			result = append(result, p)
		}
	}

	// Join with ", "
	output := ""
	for i, r := range result {
		if i > 0 {
			output += ", "
		}
		output += r
	}
	return output
}

// transformLambdas transforms lambda expressions:
// - TypeScript-style: (x) => expr OR (x) => { stmts }
// - Rust-style: |x| expr OR |x| { stmts }
//
// Transforms to: func(x) { return expr } OR func(x) { stmts }
func transformLambdas(src []byte) []byte {
	result := make([]byte, 0, len(src)+200)

	i := 0
	inString := false
	inRawString := false
	stringChar := byte(0)

	for i < len(src) {
		ch := src[i]

		// Track string literals
		if !inString && !inRawString {
			if ch == '"' {
				inString = true
				stringChar = '"'
			} else if ch == '\'' {
				inString = true
				stringChar = '\''
			} else if ch == '`' {
				inRawString = true
			}
		} else if inString {
			if ch == stringChar && (i == 0 || src[i-1] != '\\') {
				inString = false
			}
			result = append(result, ch)
			i++
			continue
		} else if inRawString {
			if ch == '`' {
				inRawString = false
			}
			result = append(result, ch)
			i++
			continue
		}

		// Check for Rust-style lambda: |params| expr
		if ch == '|' && !inString && !inRawString {
			// Look for closing |
			j := i + 1
			for j < len(src) && src[j] != '|' && src[j] != '\n' {
				j++
			}
			if j < len(src) && src[j] == '|' {
				// Found |params|, parse and transform to func(params)
				params := string(src[i+1 : j])
				transformedParams := transformLambdaParams(params)
				result = append(result, "func("...)
				result = append(result, transformedParams...)
				result = append(result, ')')

				// Skip past closing |
				i = j + 1

				// Skip whitespace
				for i < len(src) && (src[i] == ' ' || src[i] == '\t') {
					i++
				}

				// Check if body is a block or expression
				if i < len(src) && src[i] == '{' {
					// Block body - copy as-is
					result = append(result, ' ')
				} else {
					// Expression body - wrap in { return ... }
					result = append(result, " { return "...)
					// Find end of expression (comma, newline, or closing paren/bracket at depth 0)
					exprStart := i
					depth := 0
					for i < len(src) {
						c := src[i]
						if c == '(' || c == '[' || c == '{' {
							depth++
						} else if c == ')' || c == ']' || c == '}' {
							if depth == 0 {
								break
							}
							depth--
						} else if depth == 0 && (c == ',' || c == '\n') {
							break
						}
						i++
					}
					result = append(result, src[exprStart:i]...)
					result = append(result, " }"...)
				}
				continue
			}
		}

		// Check for TypeScript-style lambda: (params) => expr
		// We need to detect ) => pattern
		if ch == '=' && i+1 < len(src) && src[i+1] == '>' && !inString && !inRawString {
			// Look back in the SOURCE to find matching (
			// But first check if this is preceded by )
			srcJ := i - 1
			for srcJ >= 0 && (src[srcJ] == ' ' || src[srcJ] == '\t') {
				srcJ--
			}
			if srcJ >= 0 && src[srcJ] == ')' {
				// Find matching opening paren in source
				parenDepth := 1
				parenEnd := srcJ
				srcJ--
				for srcJ >= 0 && parenDepth > 0 {
					if src[srcJ] == ')' {
						parenDepth++
					} else if src[srcJ] == '(' {
						parenDepth--
					}
					srcJ--
				}
				parenStart := srcJ + 1

				if parenDepth == 0 {
					// Found (params) => pattern in source
					// Extract params from source (without parentheses)
					paramsInner := string(src[parenStart+1 : parenEnd]) // without ()
					transformedParams := transformLambdaParams(paramsInner)

					// Calculate how many chars we need to remove from result
					// We've already copied chars from parenStart to i into result
					charsToRemove := i - parenStart
					if charsToRemove > 0 && len(result) >= charsToRemove {
						result = result[:len(result)-charsToRemove]
					}

					// Write func(params)
					result = append(result, "func("...)
					result = append(result, transformedParams...)
					result = append(result, ')')

					// Skip =>
					i += 2

					// Skip whitespace
					for i < len(src) && (src[i] == ' ' || src[i] == '\t') {
						i++
					}

					// Check if body is a block or expression
					if i < len(src) && src[i] == '{' {
						// Block body - copy as-is
						result = append(result, ' ')
					} else {
						// Expression body - wrap in { return ... }
						result = append(result, " { return "...)
						// Find end of expression
						exprStart := i
						depth := 0
						for i < len(src) {
							c := src[i]
							if c == '(' || c == '[' || c == '{' {
								depth++
							} else if c == ')' || c == ']' || c == '}' {
								if depth == 0 {
									break
								}
								depth--
							} else if depth == 0 && (c == ',' || c == '\n') {
								break
							}
							i++
						}
						result = append(result, src[exprStart:i]...)
						result = append(result, " }"...)
					}
					continue
				}
			}
		}

		result = append(result, ch)
		i++
	}

	return result
}

// ParseExpr parses a Dingo expression and returns a Go AST expression.
func ParseExpr(src string) (ast.Expr, error) {
	// Transform Dingo syntax
	goSrc, _, err := TransformToGo([]byte(src))
	if err != nil {
		return nil, err
	}

	return goparser.ParseExpr(string(goSrc))
}

// transformEnum transforms enum declarations:
//
//	enum Name {
//	    Variant1
//	    Variant2 { field1: type1, field2: type2 }
//	}
//
// To Go tagged union interface pattern:
//
//	type Name interface { isName() }
//	type NameVariant1 struct {}
//	func (NameVariant1) isName() {}
//	type NameVariant2 struct { field1 type1; field2 type2 }
//	func (NameVariant2) isName() {}
//	func EventVariant1(...) Name { return NameVariant1{...} }
func transformEnum(src []byte) []byte {
	result := make([]byte, 0, len(src)+500)

	i := 0
	for i < len(src) {
		// Look for "enum" keyword
		if i+4 <= len(src) && string(src[i:i+4]) == "enum" {
			// Check if it's actually a keyword (not part of another identifier)
			if i > 0 && isAlphaNum(src[i-1]) {
				result = append(result, src[i])
				i++
				continue
			}
			if i+4 < len(src) && isAlphaNum(src[i+4]) {
				result = append(result, src[i])
				i++
				continue
			}

			// Found enum keyword, parse it
			j := i + 4
			// Skip whitespace
			for j < len(src) && (src[j] == ' ' || src[j] == '\t' || src[j] == '\n') {
				j++
			}

			// Get enum name
			nameStart := j
			for j < len(src) && isAlphaNum(src[j]) {
				j++
			}
			enumName := string(src[nameStart:j])
			if enumName == "" {
				result = append(result, src[i])
				i++
				continue
			}

			// Skip whitespace
			for j < len(src) && (src[j] == ' ' || src[j] == '\t' || src[j] == '\n') {
				j++
			}

			// Expect '{'
			if j >= len(src) || src[j] != '{' {
				result = append(result, src[i])
				i++
				continue
			}
			j++ // skip {

			// Parse variants
			type enumVariant struct {
				name   string
				fields []struct {
					name  string
					typ   string
				}
			}
			var variants []enumVariant

			for j < len(src) {
				// Skip whitespace and newlines
				for j < len(src) && (src[j] == ' ' || src[j] == '\t' || src[j] == '\n') {
					j++
				}

				// Check for closing brace
				if j < len(src) && src[j] == '}' {
					j++
					break
				}

				// Get variant name
				variantStart := j
				for j < len(src) && isAlphaNum(src[j]) {
					j++
				}
				variantName := string(src[variantStart:j])
				if variantName == "" {
					break
				}

				variant := enumVariant{name: variantName}

				// Skip whitespace
				for j < len(src) && (src[j] == ' ' || src[j] == '\t') {
					j++
				}

				// Check for variant fields
				if j < len(src) && src[j] == '{' {
					j++ // skip {
					// Parse fields until }
					for j < len(src) && src[j] != '}' {
						// Skip whitespace
						for j < len(src) && (src[j] == ' ' || src[j] == '\t' || src[j] == '\n' || src[j] == ',') {
							j++
						}
						if j < len(src) && src[j] == '}' {
							break
						}

						// Get field name
						fieldStart := j
						for j < len(src) && isAlphaNum(src[j]) {
							j++
						}
						fieldName := string(src[fieldStart:j])
						if fieldName == "" {
							break
						}

						// Skip whitespace
						for j < len(src) && (src[j] == ' ' || src[j] == '\t') {
							j++
						}

						// Expect ':'
						if j < len(src) && src[j] == ':' {
							j++
						}

						// Skip whitespace
						for j < len(src) && (src[j] == ' ' || src[j] == '\t') {
							j++
						}

						// Get field type (may contain generics, pointers, etc.)
						typeStart := j
						for j < len(src) && src[j] != ',' && src[j] != '}' && src[j] != '\n' {
							j++
						}
						fieldType := trimRight(string(src[typeStart:j]))

						variant.fields = append(variant.fields, struct {
							name string
							typ  string
						}{fieldName, fieldType})
					}
					if j < len(src) && src[j] == '}' {
						j++
					}
				}

				variants = append(variants, variant)
			}

			// Register variants for match expression lookup
			// Maps both short name (UserCreated) and full name (EventUserCreated)
			for _, v := range variants {
				enumRegistry[v.name] = enumName
				// Also register the full compound name for constructor call transformation
				enumRegistry[enumName+v.name] = enumName
			}

			// Generate Go code for enum
			// 1. Interface type
			result = append(result, "type "...)
			result = append(result, enumName...)
			result = append(result, " interface { is"...)
			result = append(result, enumName...)
			result = append(result, "() }\n\n"...)

			// 2. Variant structs and methods
			for _, v := range variants {
				// Struct
				result = append(result, "type "...)
				result = append(result, enumName...)
				result = append(result, v.name...)
				result = append(result, " struct {"...)
				for fi, f := range v.fields {
					if fi > 0 {
						result = append(result, "; "...)
					}
					result = append(result, f.name...)
					result = append(result, ' ')
					result = append(result, f.typ...)
				}
				result = append(result, "}\n"...)

				// Interface method
				result = append(result, "func ("...)
				result = append(result, enumName...)
				result = append(result, v.name...)
				result = append(result, ") is"...)
				result = append(result, enumName...)
				result = append(result, "() {}\n"...)

				// Constructor function with "New" prefix
				// Generates: func NewEventUserCreated(...) Event { return EventUserCreated{...} }
				result = append(result, "func New"...)
				result = append(result, enumName...)
				result = append(result, v.name...)
				result = append(result, "("...)
				for fi, f := range v.fields {
					if fi > 0 {
						result = append(result, ", "...)
					}
					result = append(result, f.name...)
					result = append(result, ' ')
					result = append(result, f.typ...)
				}
				result = append(result, ") "...)
				result = append(result, enumName...)
				result = append(result, " { return "...)
				result = append(result, enumName...)
				result = append(result, v.name...)
				result = append(result, "{"...)
				for fi, f := range v.fields {
					if fi > 0 {
						result = append(result, ", "...)
					}
					result = append(result, f.name...)
					result = append(result, ": "...)
					result = append(result, f.name...)
				}
				result = append(result, "} }\n\n"...)
			}

			i = j
			continue
		}

		result = append(result, src[i])
		i++
	}

	return result
}

// inferMatchReturnType attempts to infer the return type for a match expression
// by looking at the immediate context (return statement or assignment).
// Falls back to "interface{}" if inference fails.
func inferMatchReturnType(src []byte, matchPos int) string {
	// Strategy:
	// 1. Check if we're in a return statement (return match ...)
	//    -> Use enclosing function's return type
	// 2. Check if we're in an assignment (x := match ...)
	//    -> Try to infer from variable type (not implemented - use interface{})
	// 3. Fallback to interface{}

	// Look backwards for "return" keyword (most common case)
	i := matchPos - 1
	// Skip whitespace
	for i >= 0 && (src[i] == ' ' || src[i] == '\t' || src[i] == '\n') {
		i--
	}

	// Check if preceded by "return"
	if i >= 6 && string(src[i-5:i+1]) == "return" {
		// Verify it's a keyword
		if (i == 5 || !isAlphaNum(src[i-6])) {
			// We're in a return statement - find function return type
			return findFunctionReturnType(src, matchPos)
		}
	}

	// Check if preceded by := or = (assignment)
	// For now, we can't easily infer the variable type without go/types,
	// so fall back to interface{} for assignments
	// Future: Could use AST plugin with go/types for better inference

	return "interface{}"
}

// findFunctionReturnType finds the return type of the enclosing function
func findFunctionReturnType(src []byte, pos int) string {
	// Find the enclosing function by scanning backwards for "func"
	funcPos := -1
	braceDepth := 0
	for k := pos - 1; k >= 4; k-- {
		if src[k] == '}' {
			braceDepth++
		} else if src[k] == '{' {
			braceDepth--
			if braceDepth < 0 {
				// Found the opening brace of our function
				// Now scan backwards to find "func"
				for j := k - 1; j >= 4; j-- {
					if string(src[j-3:j+1]) == "func" {
						// Verify it's a keyword
						if (j == 3 || !isAlphaNum(src[j-4])) && (j+1 >= len(src) || !isAlphaNum(src[j+1])) {
							funcPos = j + 1
							break
						}
					}
				}
				break
			}
		}
	}

	if funcPos == -1 {
		return "interface{}"
	}

	// Parse from funcPos to find return type
	// Pattern: func [receiver] name(params) returnType {
	i := funcPos
	// Skip "func"
	for i < len(src) && isAlphaNum(src[i]) {
		i++
	}
	// Skip whitespace
	for i < len(src) && (src[i] == ' ' || src[i] == '\t' || src[i] == '\n') {
		i++
	}

	// Skip receiver if present: (r ReceiverType)
	if i < len(src) && src[i] == '(' {
		depth := 1
		i++
		for i < len(src) && depth > 0 {
			if src[i] == '(' {
				depth++
			} else if src[i] == ')' {
				depth--
			}
			i++
		}
		// Skip whitespace after receiver
		for i < len(src) && (src[i] == ' ' || src[i] == '\t' || src[i] == '\n') {
			i++
		}
	}

	// Skip function name
	for i < len(src) && isAlphaNum(src[i]) {
		i++
	}
	// Skip whitespace
	for i < len(src) && (src[i] == ' ' || src[i] == '\t' || src[i] == '\n') {
		i++
	}

	// Skip parameter list: (...)
	if i < len(src) && src[i] == '(' {
		depth := 1
		i++
		for i < len(src) && depth > 0 {
			if src[i] == '(' {
				depth++
			} else if src[i] == ')' {
				depth--
			}
			i++
		}
		// Skip whitespace after params
		for i < len(src) && (src[i] == ' ' || src[i] == '\t' || src[i] == '\n') {
			i++
		}
	}

	// Now we should be at the return type (or { if no return type)
	if i >= len(src) || src[i] == '{' {
		// No return type (void function)
		return "interface{}"
	}

	// Extract return type until we hit {
	typeStart := i
	depth := 0
	for i < len(src) && (src[i] != '{' || depth > 0) {
		if src[i] == '(' || src[i] == '[' {
			depth++
		} else if src[i] == ')' || src[i] == ']' {
			depth--
		}
		i++
	}

	returnType := string(src[typeStart:i])
	returnType = trimRight(returnType)

	if returnType == "" {
		return "interface{}"
	}

	return returnType
}

// inferTypeFromLiteral returns the Go type for a literal value
// Returns empty string if not a recognizable literal
func inferTypeFromLiteral(expr string) string {
	expr = trim(expr)

	// Boolean literals
	if expr == "true" || expr == "false" {
		return "bool"
	}

	// String literals (quoted)
	if len(expr) >= 2 && (expr[0] == '"' || expr[0] == '`') {
		return "string"
	}

	// Integer literals (all digits, possibly negative)
	if len(expr) > 0 {
		start := 0
		if expr[0] == '-' {
			start = 1
		}
		if start < len(expr) {
			allDigits := true
			for i := start; i < len(expr); i++ {
				if expr[i] < '0' || expr[i] > '9' {
					allDigits = false
					break
				}
			}
			if allDigits {
				return "int"
			}
		}
	}

	// Float literals (digits with a dot)
	if len(expr) > 0 {
		start := 0
		if expr[0] == '-' {
			start = 1
		}
		hasDot := false
		allDigitsOrDot := true
		for i := start; i < len(expr); i++ {
			if expr[i] == '.' {
				hasDot = true
			} else if expr[i] < '0' || expr[i] > '9' {
				allDigitsOrDot = false
				break
			}
		}
		if allDigitsOrDot && hasDot {
			return "float64"
		}
	}

	return ""
}

// inferTypeFromArmBodies attempts to infer return type from match arm body expressions
// If all arms return the same literal type, use that type
func inferTypeFromArmBodies(bodies []string) string {
	if len(bodies) == 0 {
		return ""
	}

	var inferredType string
	for _, body := range bodies {
		armType := inferTypeFromLiteral(body)
		if armType == "" {
			// Non-literal expression, can't infer
			return ""
		}
		if inferredType == "" {
			inferredType = armType
		} else if inferredType != armType {
			// Mixed types, can't unify
			return ""
		}
	}

	return inferredType
}

// transformMatch transforms match expressions:
//
//	match expr {
//	    Pattern(a, b) => result,
//	    Pattern { field } => result,
//	    _ => default,
//	}
//
// To Go switch with type assertions:
//
//	func() T {
//	    switch v := expr.(type) {
//	    case TypePattern:
//	        a, b := v.Field1, v.Field2
//	        return result
//	    default:
//	        return default
//	    }
//	}()
func transformMatch(src []byte) []byte {
	result := make([]byte, 0, len(src)+500)

	i := 0
	inString := false
	inRawString := false
	stringChar := byte(0)

	for i < len(src) {
		ch := src[i]

		// Track string literals
		if !inString && !inRawString {
			if ch == '"' {
				inString = true
				stringChar = '"'
			} else if ch == '\'' {
				inString = true
				stringChar = '\''
			} else if ch == '`' {
				inRawString = true
			}
		} else if inString {
			if ch == stringChar && (i == 0 || src[i-1] != '\\') {
				inString = false
			}
			result = append(result, ch)
			i++
			continue
		} else if inRawString {
			if ch == '`' {
				inRawString = false
			}
			result = append(result, ch)
			i++
			continue
		}

		// Look for "match" keyword
		if i+5 <= len(src) && string(src[i:i+5]) == "match" && !inString && !inRawString {
			// Check if it's actually a keyword (not part of another identifier)
			if i > 0 && isAlphaNum(src[i-1]) {
				result = append(result, ch)
				i++
				continue
			}
			if i+5 < len(src) && isAlphaNum(src[i+5]) {
				result = append(result, ch)
				i++
				continue
			}

			// Found match keyword
			j := i + 5

			// Skip whitespace
			for j < len(src) && (src[j] == ' ' || src[j] == '\t') {
				j++
			}

			// Get the expression to match on
			exprStart := j
			braceDepth := 0
			for j < len(src) {
				if src[j] == '{' {
					if braceDepth == 0 {
						break
					}
					braceDepth++
				} else if src[j] == '}' {
					braceDepth--
				} else if src[j] == '(' {
					braceDepth++
				} else if src[j] == ')' {
					braceDepth--
				}
				j++
			}
			matchExpr := trimRight(string(src[exprStart:j]))

			if j >= len(src) || src[j] != '{' {
				result = append(result, ch)
				i++
				continue
			}
			j++ // skip {

			// Parse match arms
			type matchArm struct {
				pattern     string // e.g., "Success", "PaymentFailed"
				fields      string // e.g., "transactionID, amount" or "{ field, message }"
				guard       string // e.g., "amount > 1000"
				body        string // the result expression or statements
				isDefault   bool   // _ pattern
				isFieldBind bool   // uses { } instead of ( )
			}
			var arms []matchArm

			for j < len(src) {
				// Skip whitespace, newlines, and comments
				for j < len(src) {
					// Skip whitespace and newlines
					if src[j] == ' ' || src[j] == '\t' || src[j] == '\n' || src[j] == ',' {
						j++
						continue
					}
					// Skip line comments
					if src[j] == '/' && j+1 < len(src) && src[j+1] == '/' {
						// Skip to end of line
						for j < len(src) && src[j] != '\n' {
							j++
						}
						continue
					}
					break
				}

				// Check for closing brace
				if j < len(src) && src[j] == '}' {
					j++
					break
				}

				arm := matchArm{}

				// Check for default pattern '_'
				if j < len(src) && src[j] == '_' {
					arm.isDefault = true
					j++
				} else {
					// Get pattern name
					patternStart := j
					for j < len(src) && isAlphaNum(src[j]) {
						j++
					}
					arm.pattern = string(src[patternStart:j])
				}

				// Skip whitespace
				for j < len(src) && (src[j] == ' ' || src[j] == '\t') {
					j++
				}

				// Check for field bindings ( ) or { }
				if j < len(src) && (src[j] == '(' || src[j] == '{') {
					opener := src[j]
					closer := byte(')')
					if opener == '{' {
						closer = '}'
						arm.isFieldBind = true
					}
					j++ // skip opener
					bindStart := j
					depth := 1
					for j < len(src) && depth > 0 {
						if src[j] == opener {
							depth++
						} else if src[j] == closer {
							depth--
						}
						j++
					}
					arm.fields = string(src[bindStart : j-1])
				}

				// Skip whitespace
				for j < len(src) && (src[j] == ' ' || src[j] == '\t') {
					j++
				}

				// Check for guard 'if'
				if j+2 < len(src) && string(src[j:j+2]) == "if" && (j+2 >= len(src) || !isAlphaNum(src[j+2])) {
					j += 2
					// Skip whitespace
					for j < len(src) && (src[j] == ' ' || src[j] == '\t') {
						j++
					}
					// Read guard expression until =>
					guardStart := j
					for j < len(src) && !(src[j] == '=' && j+1 < len(src) && src[j+1] == '>') {
						j++
					}
					arm.guard = trimRight(string(src[guardStart:j]))
				}

				// Skip whitespace
				for j < len(src) && (src[j] == ' ' || src[j] == '\t') {
					j++
				}

				// Expect '=>'
				if j+1 < len(src) && src[j] == '=' && src[j+1] == '>' {
					j += 2
				}

				// Skip whitespace
				for j < len(src) && (src[j] == ' ' || src[j] == '\t') {
					j++
				}

				// Read body until comma or closing brace (at depth 0)
				// Also stop at // comment
				bodyStart := j
				depth := 0
				inBodyString := false
				bodyStringChar := byte(0)
				for j < len(src) {
					// Track strings in body
					if !inBodyString && (src[j] == '"' || src[j] == '\'') {
						inBodyString = true
						bodyStringChar = src[j]
					} else if inBodyString && src[j] == bodyStringChar && (j == 0 || src[j-1] != '\\') {
						inBodyString = false
					}

					if !inBodyString {
						if src[j] == '{' || src[j] == '(' || src[j] == '[' {
							depth++
						} else if src[j] == '}' || src[j] == ')' || src[j] == ']' {
							if depth == 0 {
								break
							}
							depth--
						} else if src[j] == ',' && depth == 0 {
							break
						} else if src[j] == '/' && j+1 < len(src) && src[j+1] == '/' && depth == 0 {
							// Comment - stop here
							break
						}
					}
					j++
				}
				arm.body = trim(string(src[bodyStart:j]))

				// Skip comment if present
				if j < len(src) && src[j] == '/' && j+1 < len(src) && src[j+1] == '/' {
					// Skip to end of line
					for j < len(src) && src[j] != '\n' {
						j++
					}
				}

				// Skip trailing comma
				if j < len(src) && src[j] == ',' {
					j++
				}

				arms = append(arms, arm)
			}

			// Generate Go switch statement
			// Infer return type from context
			returnType := inferMatchReturnType(src, i)

			// If context-based inference failed, try to infer from arm expressions
			if returnType == "interface{}" {
				var armBodies []string
				for _, arm := range arms {
					armBodies = append(armBodies, arm.body)
				}
				if armType := inferTypeFromArmBodies(armBodies); armType != "" {
					returnType = armType
				}
			}

		// Check if any arm extracts fields - if not, use _ to avoid "declared and not used"
		hasFieldExtraction := false
		for _, arm := range arms {
			if arm.fields != "" {
				fields := parseFieldBindings(arm.fields)
				for _, f := range fields {
					if f.name != "_" && f.name != ".." {
						hasFieldExtraction = true
						break
					}
				}
				if hasFieldExtraction {
					break
				}
			}
		}

		matchVarName := "__matchVal"
		if !hasFieldExtraction {
			matchVarName = "" // No assignment needed when no field extractions
		}

		// We'll use a type switch pattern
		result = append(result, "func() "...)
		result = append(result, returnType...)
		if matchVarName != "" {
			// switch __matchVal := (expr).(type) - need the variable for field extraction
			result = append(result, " { switch "...)
			result = append(result, matchVarName...)
			result = append(result, " := ("...)
			result = append(result, matchExpr...)
			result = append(result, ").(type) {\n"...)
		} else {
			// switch (expr).(type) - no assignment needed
			result = append(result, " { switch ("...)
			result = append(result, matchExpr...)
			result = append(result, ").(type) {\n"...)
		}
		// Group arms by pattern type to handle guards correctly
		// This prevents duplicate case clauses when same pattern has different guards
		type armGroup struct {
			pattern   string
			typeName  string
			arms      []matchArm
			isDefault bool
		}
		groups := make(map[string]*armGroup)
		var groupOrder []string // Preserve order
		for _, arm := range arms {
			var key string
			var typeName string
			isDefault := arm.isDefault
			if isDefault {
				key = "__default__"
				typeName = ""
			} else {
				key = arm.pattern
				// Check if pattern is a short variant name that needs enum prefix
				typeName = arm.pattern
				if prefix, ok := enumRegistry[arm.pattern]; ok {
					typeName = prefix + arm.pattern
				}
			}
			group, exists := groups[key]
			if !exists {
				group = &armGroup{
					pattern:   arm.pattern,
					typeName:  typeName,
					isDefault: isDefault,
				}
				groups[key] = group
				groupOrder = append(groupOrder, key)
			}
			group.arms = append(group.arms, arm)
		}
		// Generate case blocks
		for _, key := range groupOrder {
			group := groups[key]
			if group.isDefault {
				result = append(result, "default:\n"...)
			} else {
				result = append(result, "case "...)
				result = append(result, group.typeName...)
				result = append(result, ":\n"...)
			}
			// If only one arm in group, use simple logic
			if len(group.arms) == 1 {
				arm := group.arms[0]
				// Extract field bindings
				if arm.fields != "" {
					fields := parseFieldBindings(arm.fields)
					for _, f := range fields {
						if f.name == "_" || f.name == ".." {
							continue
						}
						result = append(result, "\t"...)
						result = append(result, f.name...)
						result = append(result, " := __matchVal."...)
						result = append(result, f.name...)
						result = append(result, "\n"...)
					}
				}
				// Guard condition
				if arm.guard != "" {
					result = append(result, "\tif !("...)
					result = append(result, arm.guard...)
					result = append(result, ") { break }\n"...)
				}
				// Body
				body := arm.body
				if len(body) >= 6 && body[:6] == "return" {
					result = append(result, "\t"...)
					result = append(result, body...)
					result = append(result, "\n"...)
				} else {
					result = append(result, "\treturn "...)
					result = append(result, body...)
					result = append(result, "\n"...)
				}
			} else {
				// Multiple arms with same pattern type - generate nested if-else
				// First, extract all field bindings (union of all arms)
				allFields := make(map[string]bool)
				for _, arm := range group.arms {
					if arm.fields != "" {
						fields := parseFieldBindings(arm.fields)
						for _, f := range fields {
							if f.name != "_" && f.name != ".." {
								allFields[f.name] = true
							}
						}
					}
				}
				// Extract all fields once
				for fieldName := range allFields {
					result = append(result, "\t"...)
					result = append(result, fieldName...)
					result = append(result, " := __matchVal."...)
					result = append(result, fieldName...)
					result = append(result, "\n"...)
				}
				// Generate nested if-else chain
				for idx, arm := range group.arms {
					if idx == 0 {
						// First arm
						if arm.guard != "" {
							result = append(result, "\tif "...)
							result = append(result, arm.guard...)
							result = append(result, " {\n"...)
						} else {
							// No guard on first arm - just emit body directly
							body := arm.body
							if len(body) >= 6 && body[:6] == "return" {
								result = append(result, "\t"...)
								result = append(result, body...)
								result = append(result, "\n"...)
							} else {
								result = append(result, "\treturn "...)
								result = append(result, body...)
								result = append(result, "\n"...)
							}
							continue // Skip to next arm
						}
					} else if idx == len(group.arms)-1 {
						// Last arm
						if arm.guard != "" {
							result = append(result, "\t} else if "...)
							result = append(result, arm.guard...)
							result = append(result, " {\n"...)
						} else {
							result = append(result, "\t} else {\n"...)
						}
					} else {
						// Middle arm - must have guard
						result = append(result, "\t} else if "...)
						result = append(result, arm.guard...)
						result = append(result, " {\n"...)
					}
					// Body (inside if/else block)
					body := arm.body
					if len(body) >= 6 && body[:6] == "return" {
						result = append(result, "\t\t"...)
						result = append(result, body...)
						result = append(result, "\n"...)
					} else {
						result = append(result, "\t\treturn "...)
						result = append(result, body...)
						result = append(result, "\n"...)
					}
				}
				// Close if-else chain (if we started one)
				if group.arms[0].guard != "" {
					result = append(result, "\t}\n"...)
				}
			}
		}

		// Add fallback return - for exhaustive matches this should never be reached
		// Use panic instead of nil for non-pointer types
		if returnType == "interface{}" || returnType == "any" {
			result = append(result, "}\nreturn nil }()"...)
		} else {
			result = append(result, "}\npanic(\"exhaustive match failed\") }()"...)
		}

			i = j
			continue
		}

		result = append(result, ch)
		i++
	}

	return result
}

// fieldBinding represents a field binding in a match pattern
type fieldBinding struct {
	name  string
	alias string // for { field: alias } syntax
}

// parseFieldBindings parses field bindings like "a, b" or "field, message"
func parseFieldBindings(s string) []fieldBinding {
	var fields []fieldBinding
	parts := splitFields(s)
	for _, p := range parts {
		p = trimRight(p)
		// Trim leading whitespace
		for len(p) > 0 && (p[0] == ' ' || p[0] == '\t') {
			p = p[1:]
		}
		if p == "" || p == ".." {
			continue
		}
		fields = append(fields, fieldBinding{name: p})
	}
	return fields
}

// splitFields splits on commas, handling nested structures
func splitFields(s string) []string {
	var result []string
	var current []byte
	depth := 0
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch == '{' || ch == '(' || ch == '[' {
			depth++
		} else if ch == '}' || ch == ')' || ch == ']' {
			depth--
		} else if ch == ',' && depth == 0 {
			result = append(result, string(current))
			current = nil
			continue
		}
		current = append(current, ch)
	}
	if len(current) > 0 {
		result = append(result, string(current))
	}
	return result
}

// capitalizeFirst capitalizes the first letter of a string
func capitalizeFirst(s string) string {
	if len(s) == 0 {
		return s
	}
	if s[0] >= 'a' && s[0] <= 'z' {
		return string(s[0]-32) + s[1:]
	}
	return s
}

// Unused variable to ensure token package is used
var _ = token.QUESTION
