package ast

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// TupleCodeGen generates Go code from TupleLiteral AST nodes.
// This transforms Dingo tuple syntax to anonymous struct literals.
type TupleCodeGen struct {
	buf     bytes.Buffer
	builder *MappingBuilder
}

// NewTupleCodeGen creates a new tuple code generator.
func NewTupleCodeGen() *TupleCodeGen {
	return &TupleCodeGen{
		builder: NewMappingBuilder(),
	}
}

// Generate produces Go code for a TupleLiteral.
// Transforms: (10, 20) -> struct{ _0 int; _1 int }{10, 20}
func (g *TupleCodeGen) Generate(tuple *TupleLiteral, dingoStart int) CodeGenResult {
	g.buf.Reset()
	g.builder.Reset()

	// Generate struct type definition and literal
	goStart := g.builder.CurrentGoPosition()

	// struct{ _0 T0; _1 T1; ... }
	g.buf.WriteString("struct{ ")
	g.builder.SkipBytes([]byte("struct{ "))

	// Field declarations
	for i := range tuple.Elements {
		if i > 0 {
			g.buf.WriteString("; ")
			g.builder.SkipBytes([]byte("; "))
		}

		// Field name: _0, _1, etc.
		fieldName := fmt.Sprintf("_%d", i)
		g.buf.WriteString(fieldName)
		g.builder.SkipBytes([]byte(fieldName))

		// Type inference placeholder (actual types determined by element expressions)
		g.buf.WriteString(" any")
		g.builder.SkipBytes([]byte(" any"))
	}

	g.buf.WriteString(" }")
	g.builder.SkipBytes([]byte(" }"))

	// Literal values: {expr1, expr2, ...}
	g.buf.WriteString("{")
	g.builder.SkipBytes([]byte("{"))

	for i, elem := range tuple.Elements {
		if i > 0 {
			g.buf.WriteString(", ")
			g.builder.SkipBytes([]byte(", "))
		}

		if elem.Nested != nil {
			// Nested tuple: recursively generate
			nestedGen := NewTupleCodeGen()
			nestedResult := nestedGen.Generate(elem.Nested, dingoStart+int(elem.Nested.Lparen))
			g.buf.Write(nestedResult.Output)
			g.builder.SkipBytes(nestedResult.Output)
		} else {
			// Regular expression element
			g.buf.WriteString(elem.Expr)
			g.builder.SkipBytes([]byte(elem.Expr))
		}
	}

	g.buf.WriteString("}")
	g.builder.SkipBytes([]byte("}"))

	goEnd := g.builder.CurrentGoPosition()

	// Create mapping for entire tuple literal
	dingoEnd := dingoStart + int(tuple.End()-tuple.Lparen)
	g.builder.Add(dingoStart, dingoEnd, 0, "tuple_literal")
	g.builder.mappings[len(g.builder.mappings)-1].GoStart = goStart
	g.builder.mappings[len(g.builder.mappings)-1].GoEnd = goEnd

	return CodeGenResult{
		Output:   g.buf.Bytes(),
		Mappings: g.builder.Build(),
	}
}

// GenerateDestructure produces Go code for a TupleDestructure.
// Transforms: let (a, b) = tuple -> tmp := tuple; a, b := tmp._0, tmp._1
func (g *TupleCodeGen) GenerateDestructure(destructure *TupleDestructure, dingoStart int) CodeGenResult {
	g.buf.Reset()
	g.builder.Reset()

	goStart := g.builder.CurrentGoPosition()

	// Generate: tmp := value
	g.buf.WriteString("tmp := ")
	g.builder.SkipBytes([]byte("tmp := "))

	g.buf.WriteString(destructure.Value)
	g.builder.SkipBytes([]byte(destructure.Value))

	g.buf.WriteString("\n")
	g.builder.SkipBytes([]byte("\n"))

	// Generate destructuring assignments
	g.generateDestructuring(destructure.Pattern, "tmp", 1)

	goEnd := g.builder.CurrentGoPosition()

	// Create mapping for entire destructuring statement
	dingoEnd := dingoStart + int(destructure.End()-destructure.LetPos)
	g.builder.Add(dingoStart, dingoEnd, 0, "tuple_destructure")
	g.builder.mappings[len(g.builder.mappings)-1].GoStart = goStart
	g.builder.mappings[len(g.builder.mappings)-1].GoEnd = goEnd

	return CodeGenResult{
		Output:   g.buf.Bytes(),
		Mappings: g.builder.Build(),
	}
}

// generateDestructuring recursively generates destructuring assignments.
// tmpVar is the current temporary variable being destructured.
// tmpCounter is used to generate unique tmp variable names for nested patterns.
func (g *TupleCodeGen) generateDestructuring(pattern []DestructureElement, tmpVar string, tmpCounter int) int {
	var simpleNames []string
	var simpleFields []string

	for i, elem := range pattern {
		if elem.IsNested() {
			// Nested pattern: create intermediate tmp variable
			nestedTmp := formatTmpVarName(tmpCounter)
			tmpCounter++

			g.buf.WriteString(nestedTmp)
			g.buf.WriteString(" := ")
			g.buf.WriteString(tmpVar)
			g.buf.WriteString("._")
			g.buf.WriteString(strconv.Itoa(i))
			g.buf.WriteString("\n")

			g.builder.SkipBytes([]byte(nestedTmp + " := " + tmpVar + "._" + strconv.Itoa(i) + "\n"))

			// Recursively destructure the nested pattern
			tmpCounter = g.generateDestructuring(elem.Nested, nestedTmp, tmpCounter)
		} else {
			// Simple identifier: accumulate for batch assignment
			simpleNames = append(simpleNames, elem.Name)
			simpleFields = append(simpleFields, tmpVar+"._"+strconv.Itoa(i))
		}
	}

	// Generate batch assignment for simple identifiers
	if len(simpleNames) > 0 {
		assignment := strings.Join(simpleNames, ", ") + " := " + strings.Join(simpleFields, ", ") + "\n"
		g.buf.WriteString(assignment)
		g.builder.SkipBytes([]byte(assignment))
	}

	return tmpCounter
}

// formatTmpVarName formats temporary variable name following CLAUDE.md naming convention.
// First tmp is unnumbered, subsequent are tmp1, tmp2, etc.
func formatTmpVarName(counter int) string {
	if counter == 1 {
		return "tmp"
	}
	return "tmp" + strconv.Itoa(counter)
}

// FindTupleLiterals scans source code for tuple literal patterns.
// Returns byte positions of tuple literal starts.
// Distinguishes tuples from function calls and grouping parentheses.
func FindTupleLiterals(src []byte) []int {
	var positions []int
	i := 0
	n := len(src)

	for i < n {
		// Skip strings
		if src[i] == '"' || src[i] == '`' {
			i = skipString(src, i)
			continue
		}

		// Skip single-quoted strings
		if src[i] == '\'' {
			i = skipChar(src, i)
			continue
		}

		// Skip comments
		if i+1 < n && src[i] == '/' && (src[i+1] == '/' || src[i+1] == '*') {
			i = skipComment(src, i)
			continue
		}

		// Check for opening parenthesis
		if src[i] == '(' {
			if isTupleLiteral(src, i) {
				positions = append(positions, i)
			}
			i++
			continue
		}

		i++
	}

	return positions
}

// isTupleLiteral determines if a parenthesis starts a tuple literal.
// Distinguishes from function calls and grouping parentheses.
//
// Tuple literal heuristics:
// - Preceded by '=' or ':=' or ',' or '{' or '[' (assignment/initialization context)
// - Contains at least one comma at the same nesting level
// - NOT preceded by an identifier immediately (would be function call)
// - NOT single element without comma (would be grouping)
func isTupleLiteral(src []byte, parenPos int) bool {
	// Look backward to find context
	beforeParen := findPrecedingToken(src, parenPos)

	// Check for assignment/initialization context
	switch beforeParen {
	case "=", ":=", ",", "{", "[", "(", "return":
		// Potential tuple literal context
	default:
		// Check if preceded by identifier (function call)
		if isIdentifierChar(beforeParen) {
			return false
		}
	}

	// Scan forward to check for comma at same nesting level
	hasComma := false
	depth := 0
	for i := parenPos + 1; i < len(src); i++ {
		switch src[i] {
		case '(':
			depth++
		case ')':
			if depth == 0 {
				// Reached closing paren
				return hasComma
			}
			depth--
		case ',':
			if depth == 0 {
				hasComma = true
			}
		case '"', '`':
			i = skipString(src, i) - 1
		case '\'':
			i = skipChar(src, i) - 1
		case '/':
			if i+1 < len(src) && (src[i+1] == '/' || src[i+1] == '*') {
				i = skipComment(src, i) - 1
			}
		}
	}

	return false // Unclosed paren
}

// findPrecedingToken finds the token immediately before the given position.
// Skips whitespace and returns the token as a string.
func findPrecedingToken(src []byte, pos int) string {
	// Skip backward over whitespace
	i := pos - 1
	for i >= 0 && isWhitespace(src[i]) {
		i--
	}

	if i < 0 {
		return ""
	}

	// Check for multi-char operators
	if i >= 1 && src[i] == '=' && src[i-1] == ':' {
		return ":="
	}

	// Check for 'return' keyword
	if i >= 5 {
		word := string(src[i-5 : i+1])
		if word == "return" {
			return "return"
		}
	}

	// Single char token
	return string(src[i])
}

// isIdentifierChar checks if a string represents an identifier character.
func isIdentifierChar(s string) bool {
	if len(s) == 0 {
		return false
	}
	r := rune(s[0])
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

// skipString skips over a string literal starting at pos.
func skipString(src []byte, pos int) int {
	quote := src[pos]
	i := pos + 1

	if quote == '`' {
		// Raw string: skip until closing backtick
		for i < len(src) {
			if src[i] == '`' {
				return i + 1
			}
			i++
		}
		return i
	}

	// Regular string: handle escapes
	for i < len(src) {
		if src[i] == '\\' && i+1 < len(src) {
			i += 2
			continue
		}
		if src[i] == quote {
			return i + 1
		}
		i++
	}

	return i
}

// skipChar skips over a character literal starting at pos.
func skipChar(src []byte, pos int) int {
	i := pos + 1
	for i < len(src) {
		if src[i] == '\\' && i+1 < len(src) {
			i += 2
			continue
		}
		if src[i] == '\'' {
			return i + 1
		}
		i++
	}
	return i
}

// skipComment skips over a comment starting at pos.
func skipComment(src []byte, pos int) int {
	if pos+1 >= len(src) {
		return pos + 1
	}

	if src[pos+1] == '/' {
		// Line comment: skip until newline
		i := pos + 2
		for i < len(src) && src[i] != '\n' {
			i++
		}
		return i
	}

	if src[pos+1] == '*' {
		// Block comment: skip until */
		i := pos + 2
		for i+1 < len(src) {
			if src[i] == '*' && src[i+1] == '/' {
				return i + 2
			}
			i++
		}
		return i
	}

	return pos + 1
}

// TransformTupleSource transforms Dingo source containing tuples to Go source.
// This is the main entry point for tuple transformation.
func TransformTupleSource(src []byte) ([]byte, []SourceMapping) {
	tuplePositions := FindTupleLiterals(src)
	if len(tuplePositions) == 0 {
		return src, nil
	}

	// For now, we'll return source unchanged and empty mappings
	// Full implementation requires parsing tuple literals from positions
	// This would be done similar to enum_codegen.go's approach

	// TODO: Parse each tuple literal at found positions
	// TODO: Generate Go code for each
	// TODO: Assemble result with mappings

	return src, nil
}
