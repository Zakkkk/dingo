package preprocessor

import (
	"bytes"
	"fmt"
	"go/scanner"
	"go/token"
	"strings"

	"github.com/MadAppGang/dingo/pkg/registry"
)

// LetASTProcessor converts Dingo let keyword to Go using token-based parsing
// This replaces the buggy regex-based approach in keywords.go
type LetASTProcessor struct {
	// registry is the optional type registry for tracking variable declarations
	// If nil, registration is skipped (maintains backward compatibility)
	registry registry.TypeRegistry
}

// NewLetASTProcessor creates a new let processor
func NewLetASTProcessor() *LetASTProcessor {
	return &LetASTProcessor{}
}

// NewLetASTProcessorWithRegistry creates a new let processor with a type registry
func NewLetASTProcessorWithRegistry(reg registry.TypeRegistry) *LetASTProcessor {
	return &LetASTProcessor{
		registry: reg,
	}
}

// Name returns the processor name
func (p *LetASTProcessor) Name() string {
	return "let-ast"
}

// Process transforms Dingo let declarations to Go using token-based parsing
// Converts:
//   - let x = value          → x := value
//   - let x Type             → var x Type
//   - let x: Type = value    → var x Type = value
//   - let (a, b) = tuple     → a, b := tuple (destructuring - passthrough for now)
func (p *LetASTProcessor) Process(source []byte) ([]byte, []Mapping, error) {
	lines := bytes.Split(source, []byte("\n"))
	result := make([][]byte, len(lines))

	for i, line := range lines {
		transformed, err := p.processLine(line)
		if err != nil {
			return nil, nil, fmt.Errorf("line %d: %w", i+1, err)
		}
		result[i] = transformed
	}

	return bytes.Join(result, []byte("\n")), nil, nil
}

// processLine processes a single line, transforming let declarations
func (p *LetASTProcessor) processLine(line []byte) ([]byte, error) {
	// Quick check: does line contain "let" keyword?
	if !bytes.Contains(line, []byte("let")) {
		return line, nil
	}

	// Tokenize the line
	var s scanner.Scanner
	fset := token.NewFileSet()
	file := fset.AddFile("", fset.Base(), len(line))
	s.Init(file, line, nil, scanner.ScanComments)

	// Look for "let" keyword
	tokens := []tokenInfo{}
	for {
		pos, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}
		tokens = append(tokens, tokenInfo{
			pos: pos,
			tok: tok,
			lit: lit,
		})
	}

	// Find "let" keyword
	letIdx := -1
	for i, t := range tokens {
		if t.tok == token.IDENT && t.lit == "let" {
			letIdx = i
			break
		}
	}

	// No "let" found
	if letIdx == -1 {
		return line, nil
	}

	// Check if "let" is in a comment
	if p.isInComment(line, tokens, letIdx) {
		return line, nil
	}

	// Try to parse as let declaration
	transformed, ok := p.transformLetDecl(line, tokens, letIdx)
	if ok {
		return transformed, nil
	}

	// Not a valid let declaration - return unchanged
	return line, nil
}

// tokenInfo holds token information
type tokenInfo struct {
	pos token.Pos
	tok token.Token
	lit string
}

// isInComment checks if the let keyword is inside a comment
func (p *LetASTProcessor) isInComment(line []byte, tokens []tokenInfo, letIdx int) bool {
	if letIdx == 0 {
		return false
	}

	// Check for comment token before let
	for i := 0; i < letIdx; i++ {
		if tokens[i].tok == token.COMMENT {
			return true
		}
	}

	// Check for // comment before let position
	letPos := int(tokens[letIdx].pos) - 1 // token.Pos is 1-based
	if letPos > 1 {
		prefix := string(line[:letPos])
		if idx := strings.Index(prefix, "//"); idx != -1 {
			return true
		}
	}

	return false
}

// transformLetDecl attempts to transform a let declaration
// Returns (transformed line, true) if successful, (original, false) otherwise
func (p *LetASTProcessor) transformLetDecl(line []byte, tokens []tokenInfo, letIdx int) ([]byte, bool) {
	// Need at least: let IDENT [...]
	if letIdx+1 >= len(tokens) {
		return line, false
	}

	// Next token should be identifier or (
	nextTok := tokens[letIdx+1]

	// Handle destructuring: let (a, b) = value
	if nextTok.tok == token.LPAREN {
		// For now, just replace "let" with empty to get: (a, b) = value
		// Go will handle this as: a, b := value (if in short decl context)
		// This is a passthrough - actual destructuring handled elsewhere
		return p.replaceToken(line, tokens[letIdx], ""), true
	}

	// Must be identifier
	if nextTok.tok != token.IDENT {
		return line, false
	}

	// Look for what comes after identifier
	// Possibilities:
	//   - let x = value          → x := value
	//   - let x Type             → var x Type
	//   - let x: Type = value    → var x Type = value
	//   - let x, y = value       → x, y := value

	// Find the next significant token (skip whitespace conceptually)
	afterIdentIdx := letIdx + 2
	if afterIdentIdx >= len(tokens) {
		// Just "let x" - treat as incomplete declaration
		return line, false
	}

	afterIdent := tokens[afterIdentIdx]

	switch afterIdent.tok {
	case token.ASSIGN:
		// let x = value → x := value
		// Replace "let " with empty, replace "=" with ":="
		return p.transformShortDecl(line, tokens, letIdx, afterIdentIdx)

	case token.COLON:
		// let x: Type = value → var x Type = value
		// OR let x: Type (declaration only)
		return p.transformTypedDecl(line, tokens, letIdx, afterIdentIdx)

	case token.COMMA:
		// let x, y = value → x, y := value
		// Replace "let " with empty
		return p.replaceToken(line, tokens[letIdx], ""), true

	case token.IDENT:
		// let x Type (no colon, no equals)
		// let x Type → var x Type
		return p.transformVarDecl(line, tokens, letIdx)

	case token.LBRACK, token.MUL, token.MAP, token.CHAN, token.STRUCT, token.INTERFACE, token.FUNC:
		// let x []Type or let x *Type or let x map[K]V or let x chan T or let x struct{...}
		// These are type declarations: let x []int → var x []int
		return p.transformVarDecl(line, tokens, letIdx)

	default:
		// Unrecognized pattern
		return line, false
	}
}

// transformShortDecl transforms: let x = value → x := value
func (p *LetASTProcessor) transformShortDecl(line []byte, tokens []tokenInfo, letIdx, assignIdx int) ([]byte, bool) {
	// Strategy: Remove "let " and change "=" to ":="

	// Calculate how many bytes we're removing
	letStart := int(tokens[letIdx].pos) - 1
	letEnd := letStart + len(tokens[letIdx].lit)

	// Check if there's a space after "let"
	if letEnd < len(line) && line[letEnd] == ' ' {
		letEnd++ // Include trailing space
	}

	bytesRemoved := letEnd - letStart

	// Now find "=" position and adjust for removal
	assignPos := int(tokens[assignIdx].pos) - 1
	adjustedAssignPos := assignPos - bytesRemoved

	// Remove "let " first
	result := make([]byte, 0, len(line))
	result = append(result, line[:letStart]...)
	result = append(result, line[letEnd:]...)

	// Replace "=" with ":=" at adjusted position
	if adjustedAssignPos >= 0 && adjustedAssignPos < len(result) && result[adjustedAssignPos] == '=' {
		// Replace single '=' with ':='
		result = append(result[:adjustedAssignPos], append([]byte(":="), result[adjustedAssignPos+1:]...)...)
		return result, true
	}

	// Fallback: use simple replacement (should not reach here normally)
	return bytes.Replace(result, []byte("="), []byte(":="), 1), true
}

// transformTypedDecl transforms: let x: Type = value → var x Type = value
func (p *LetASTProcessor) transformTypedDecl(line []byte, tokens []tokenInfo, letIdx, colonIdx int) ([]byte, bool) {
	// Replace "let" with "var"
	// Remove the ":"

	// Extract variable name and type annotation for registry
	varName := ""
	typeAnnot := ""
	if letIdx+1 < len(tokens) {
		varName = tokens[letIdx+1].lit // Variable name is right after "let"

		// Extract type annotation (between : and = or end of line)
		typeAnnot = p.extractTypeAnnotation(line, tokens, colonIdx)
	}

	result := p.replaceToken(line, tokens[letIdx], "var")

	// Find and remove the colon
	// Adjust position since we may have changed "let" length
	adjustment := len("var") - len("let")
	colonPos := int(tokens[colonIdx].pos) - 1 + adjustment

	if colonPos >= 0 && colonPos < len(result) && result[colonPos] == ':' {
		// Remove colon by slicing it out
		result = append(result[:colonPos], result[colonPos+1:]...)
	} else {
		// Fallback: simple replacement
		result = bytes.Replace(result, []byte(":"), []byte(""), 1)
	}

	// Register variable with type registry
	if varName != "" && typeAnnot != "" {
		p.registerVariable(varName, typeAnnot)
	}

	return result, true
}

// transformVarDecl transforms: let x Type → var x Type
func (p *LetASTProcessor) transformVarDecl(line []byte, tokens []tokenInfo, letIdx int) ([]byte, bool) {
	// Extract variable name and type annotation for registry
	varName := ""
	typeAnnot := ""
	if letIdx+1 < len(tokens) && letIdx+2 < len(tokens) {
		varName = tokens[letIdx+1].lit // Variable name is right after "let"

		// Type annotation is after the variable name
		// Collect all tokens from after varName until end of statement
		typeAnnot = p.extractTypeFromVarDecl(line, tokens, letIdx+2)
	}

	// Simply replace "let" with "var"
	result := p.replaceToken(line, tokens[letIdx], "var")

	// Register variable with type registry
	if varName != "" && typeAnnot != "" {
		p.registerVariable(varName, typeAnnot)
	}

	return result, true
}

// replaceToken replaces a token at given position with new text
func (p *LetASTProcessor) replaceToken(line []byte, tok tokenInfo, replacement string) []byte {
	// token.Pos is 1-based, convert to 0-based
	start := int(tok.pos) - 1
	end := start + len(tok.lit)

	if start < 0 || end > len(line) {
		// Invalid position, return unchanged
		return line
	}

	// When removing a token (empty replacement), also remove trailing space if present
	if replacement == "" && end < len(line) && line[end] == ' ' {
		end++ // Include the space after "let"
	}

	result := make([]byte, 0, len(line)+len(replacement)-len(tok.lit))
	result = append(result, line[:start]...)
	result = append(result, []byte(replacement)...)
	result = append(result, line[end:]...)

	return result
}

// parseTypeInfo extracts TypeInfo from a type annotation string
// Detects Result<T,E>, Option<T>, and other types
func parseTypeInfo(typeAnnot string) registry.TypeInfo {
	typeAnnot = strings.TrimSpace(typeAnnot)

	// Check for Result<T, E>
	if strings.HasPrefix(typeAnnot, "Result<") && strings.HasSuffix(typeAnnot, ">") {
		inner := typeAnnot[len("Result<") : len(typeAnnot)-1]
		parts := strings.Split(inner, ",")
		if len(parts) == 2 {
			return registry.TypeInfo{
				Kind:      registry.TypeKindResult,
				Name:      typeAnnot,
				ValueType: strings.TrimSpace(parts[0]),
				ErrorType: strings.TrimSpace(parts[1]),
			}
		} else if len(parts) == 1 {
			// Result<T> defaults to Result<T, error>
			return registry.TypeInfo{
				Kind:      registry.TypeKindResult,
				Name:      typeAnnot,
				ValueType: strings.TrimSpace(parts[0]),
				ErrorType: "error",
			}
		}
	}

	// Check for Option<T>
	if strings.HasPrefix(typeAnnot, "Option<") && strings.HasSuffix(typeAnnot, ">") {
		inner := typeAnnot[len("Option<") : len(typeAnnot)-1]
		return registry.TypeInfo{
			Kind:      registry.TypeKindOption,
			Name:      typeAnnot,
			ValueType: strings.TrimSpace(inner),
		}
	}

	// Check if it's a pointer type
	isPointer := strings.HasPrefix(typeAnnot, "*")
	if isPointer {
		typeAnnot = typeAnnot[1:]
	}

	// Check for basic types
	basicTypes := map[string]bool{
		"int": true, "int8": true, "int16": true, "int32": true, "int64": true,
		"uint": true, "uint8": true, "uint16": true, "uint32": true, "uint64": true,
		"float32": true, "float64": true,
		"string": true, "bool": true, "byte": true, "rune": true,
		"error": true,
	}

	kind := registry.TypeKindNamed
	if basicTypes[typeAnnot] {
		kind = registry.TypeKindBasic
	}

	return registry.TypeInfo{
		Kind:      kind,
		Name:      typeAnnot,
		IsPointer: isPointer,
	}
}

// registerVariable registers a variable with the type registry (if available)
func (p *LetASTProcessor) registerVariable(name, typeAnnot string) {
	if p.registry == nil {
		return // Skip registration if no registry
	}

	typeInfo := parseTypeInfo(typeAnnot)
	varInfo := registry.VariableInfo{
		Name:  name,
		Type:  typeInfo,
		Scope: p.registry.GetScopeLevel(),
	}

	p.registry.RegisterVariable(varInfo)
}

// extractTypeAnnotation extracts the type annotation from "let x: Type = value"
// Returns the type string between the colon and the equals sign (or end of line)
func (p *LetASTProcessor) extractTypeAnnotation(line []byte, tokens []tokenInfo, colonIdx int) string {
	// Find the range from after the colon to before the equals sign (or end)
	startIdx := colonIdx + 1
	endIdx := len(tokens)

	// Find the equals sign or end of tokens
	for i := startIdx; i < len(tokens); i++ {
		if tokens[i].tok == token.ASSIGN {
			endIdx = i
			break
		}
	}

	if startIdx >= endIdx {
		return ""
	}

	// Extract the byte range from the line
	startPos := int(tokens[startIdx].pos) - 1
	var endPos int
	if endIdx < len(tokens) {
		endPos = int(tokens[endIdx].pos) - 1
	} else {
		endPos = len(line)
	}

	if startPos < 0 || endPos > len(line) || startPos >= endPos {
		return ""
	}

	return strings.TrimSpace(string(line[startPos:endPos]))
}

// extractTypeFromVarDecl extracts the type from "let x Type" declarations
// Returns the type string from the given token index until end of statement
func (p *LetASTProcessor) extractTypeFromVarDecl(line []byte, tokens []tokenInfo, typeStartIdx int) string {
	if typeStartIdx >= len(tokens) {
		return ""
	}

	// Find end of type (usually end of tokens, but could be = or ;)
	endIdx := len(tokens)
	for i := typeStartIdx; i < len(tokens); i++ {
		if tokens[i].tok == token.ASSIGN || tokens[i].tok == token.SEMICOLON {
			endIdx = i
			break
		}
	}

	if typeStartIdx >= endIdx {
		return ""
	}

	// Extract the byte range from the line
	startPos := int(tokens[typeStartIdx].pos) - 1
	var endPos int
	if endIdx < len(tokens) {
		endPos = int(tokens[endIdx].pos) - 1
	} else {
		endPos = len(line)
	}

	if startPos < 0 || endPos > len(line) || startPos >= endPos {
		return ""
	}

	return strings.TrimSpace(string(line[startPos:endPos]))
}
