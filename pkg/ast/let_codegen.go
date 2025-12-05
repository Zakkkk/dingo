package ast

import (
	"bytes"
	"fmt"
	"strings"
)

// LetCodeGen generates Go code from let declarations.
// This transforms Dingo's let syntax to Go var/:= declarations.
type LetCodeGen struct {
	buf bytes.Buffer
}

// NewLetCodeGen creates a new let code generator.
func NewLetCodeGen() *LetCodeGen {
	return &LetCodeGen{}
}

// Generate produces Go code for a let declaration.
// Returns the generated Go code as bytes.
func (g *LetCodeGen) Generate(decl *LetDecl) []byte {
	g.buf.Reset()

	// Check if this is a tuple destructuring pattern
	// (multiple names from single value, not comma-separated assignments)
	isTupleDestructure := len(decl.Names) > 1 && decl.Value != "" && !isFunctionReturningMultiple(decl.Value)

	// Determine if we need var or := syntax
	hasType := decl.TypeAnnot != ""

	if hasType {
		// With type annotation: var x Type = value
		g.buf.WriteString("var ")
		g.writeNames(decl.Names)
		// Remove leading colon and leading whitespace from type annotation
		typeAnnot := decl.TypeAnnot
		if len(typeAnnot) > 0 && typeAnnot[0] == ':' {
			typeAnnot = typeAnnot[1:]
		}
		// Trim leading whitespace
		typeAnnot = strings.TrimLeft(typeAnnot, " \t")
		g.buf.WriteString(" ")
		g.buf.WriteString(typeAnnot)
		if decl.HasInit {
			g.buf.WriteString(" = ")
			g.buf.WriteString(decl.Value)
		}
	} else if isTupleDestructure {
		// Tuple destructuring: let (a, b) = tuple → tmp := tuple; a, b := tmp.__0, tmp.__1
		g.generateTupleDestructure(decl)
	} else {
		// Without type: name := value
		g.writeNames(decl.Names)
		g.buf.WriteString(" := ")
		if decl.HasInit {
			g.buf.WriteString(decl.Value)
		}
	}

	return g.buf.Bytes()
}

// writeNames writes comma-separated names
func (g *LetCodeGen) writeNames(names []string) {
	for i, name := range names {
		if i > 0 {
			g.buf.WriteString(", ")
		}
		g.buf.WriteString(name)
	}
}

// generateTupleDestructure generates Go code for tuple destructuring
// let (a, b) = tuple → tmp := tuple; a, b := tmp.__0, tmp.__1
func (g *LetCodeGen) generateTupleDestructure(decl *LetDecl) {
	// Generate temp variable for tuple value
	tempName := "tmp"
	g.buf.WriteString(tempName)
	g.buf.WriteString(" := ")
	g.buf.WriteString(decl.Value)
	g.buf.WriteString("; ")

	// Generate destructuring assignment
	g.writeNames(decl.Names)
	g.buf.WriteString(" := ")

	// Generate field access for each name
	for i := range decl.Names {
		if i > 0 {
			g.buf.WriteString(", ")
		}
		g.buf.WriteString(tempName)
		g.buf.WriteString(fmt.Sprintf(".__%d", i))
	}
}

// isFunctionReturningMultiple checks if the value expression looks like a function call
// that returns multiple values (which Go supports natively)
// This is a heuristic - we assume function calls with () might return multiple values
func isFunctionReturningMultiple(value string) bool {
	// Simple heuristic: if value contains '(' it's likely a function call
	// that might return multiple values (Go's native multi-return)
	for _, ch := range value {
		if ch == '(' {
			return true
		}
	}
	return false
}

// FindLetDeclarations finds all positions of 'let' keywords in source.
// Returns byte offsets where 'let' declarations start.
func FindLetDeclarations(src []byte) []int {
	var positions []int
	inString := false
	inComment := false
	inMultiLineComment := false
	escapeNext := false

	for i := 0; i < len(src); i++ {
		ch := src[i]

		// Handle string escape sequences
		if escapeNext {
			escapeNext = false
			continue
		}
		if ch == '\\' && inString {
			escapeNext = true
			continue
		}

		// Track string literals
		if ch == '"' && !inComment && !inMultiLineComment {
			inString = !inString
			continue
		}

		// Skip if inside string
		if inString {
			continue
		}

		// Track multi-line comments
		if !inComment && i+1 < len(src) && ch == '/' && src[i+1] == '*' {
			inMultiLineComment = true
			i++ // Skip '*'
			continue
		}
		if inMultiLineComment && i+1 < len(src) && ch == '*' && src[i+1] == '/' {
			inMultiLineComment = false
			i++ // Skip '/'
			continue
		}

		// Track single-line comments
		if !inMultiLineComment && i+1 < len(src) && ch == '/' && src[i+1] == '/' {
			inComment = true
			i++ // Skip second '/'
			continue
		}
		if inComment && ch == '\n' {
			inComment = false
			continue
		}

		// Skip if inside comment
		if inComment || inMultiLineComment {
			continue
		}

		// Check for 'let' keyword
		if i+3 < len(src) &&
			ch == 'l' &&
			src[i+1] == 'e' &&
			src[i+2] == 't' &&
			(i+3 == len(src) || !isAlphaNumeric(src[i+3])) {
			// Verify preceded by whitespace or start of line
			if i == 0 || isWhitespace(src[i-1]) || src[i-1] == '\n' || src[i-1] == ';' || src[i-1] == '{' {
				positions = append(positions, i)
			}
		}
	}

	return positions
}

// isAlphaNumeric checks if a byte is alphanumeric or underscore
func isAlphaNumeric(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') ||
		(ch >= 'A' && ch <= 'Z') ||
		(ch >= '0' && ch <= '9') ||
		ch == '_'
}

// TransformLetSource transforms Dingo source containing let declarations to Go source.
// This is the main entry point that replaces string-based let transforms.
func TransformLetSource(src []byte) ([]byte, []SourceMapping) {
	letPositions := FindLetDeclarations(src)
	if len(letPositions) == 0 {
		return src, nil
	}

	var mappings []SourceMapping
	result := make([]byte, 0, len(src))
	lastPos := 0

	for _, letStart := range letPositions {
		// Copy source before this let
		result = append(result, src[lastPos:letStart]...)

		// Parse the let declaration
		parser := NewLetParser(src[letStart:], letStart)
		decl, endOffset, err := parser.ParseLetDecl()
		if err != nil {
			// Parsing failed, keep original source
			result = append(result, src[letStart:letStart+3]...) // Copy "let"
			lastPos = letStart + 3
			continue
		}

		// Generate Go code
		goStart := len(result)
		codegen := NewLetCodeGen()
		goCode := codegen.Generate(decl)
		result = append(result, goCode...)
		goEnd := len(result)

		// Create source mapping
		mappings = append(mappings, SourceMapping{
			DingoStart: letStart,
			DingoEnd:   letStart + endOffset,
			GoStart:    goStart,
			GoEnd:      goEnd,
			Kind:       "let",
		})

		lastPos = letStart + endOffset
	}

	// Copy remaining source
	result = append(result, src[lastPos:]...)

	return result, mappings
}
