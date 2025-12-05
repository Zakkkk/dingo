package ast

import (
	"bytes"
	"fmt"
)

// FindNullCoalesceExpressions finds all ?? operators in source code
// Returns byte positions of each ?? operator (not inside strings/comments)
func FindNullCoalesceExpressions(src []byte) []int {
	var positions []int
	inString := false
	inLineComment := false
	inBlockComment := false
	stringChar := byte(0)

	for i := 0; i < len(src); i++ {
		ch := src[i]

		// Handle newlines (end line comments)
		if ch == '\n' {
			inLineComment = false
			continue
		}

		// Check for line comment start
		if !inString && !inBlockComment && i+1 < len(src) && ch == '/' && src[i+1] == '/' {
			inLineComment = true
			i++
			continue
		}

		// Check for block comment start
		if !inString && !inLineComment && i+1 < len(src) && ch == '/' && src[i+1] == '*' {
			inBlockComment = true
			i++
			continue
		}

		// Check for block comment end
		if inBlockComment && i+1 < len(src) && ch == '*' && src[i+1] == '/' {
			inBlockComment = false
			i++
			continue
		}

		// Skip if in comment
		if inLineComment || inBlockComment {
			continue
		}

		// Handle strings
		if ch == '"' || ch == '\'' || ch == '`' {
			if !inString {
				inString = true
				stringChar = ch
			} else if ch == stringChar {
				// Check for escape
				if i > 0 && src[i-1] != '\\' {
					inString = false
					stringChar = 0
				}
			}
			continue
		}

		// Skip if in string
		if inString {
			continue
		}

		// Look for ?? operator (not ?.?)
		if i+1 < len(src) && ch == '?' && src[i+1] == '?' {
			// Make sure it's not ?.? (safe nav + ?)
			if i > 0 && src[i-1] == '.' {
				continue
			}
			positions = append(positions, i)
			i++ // Skip second ?
		}
	}

	return positions
}

// NullCoalesceCodeGen generates Go code from null coalescing expressions
type NullCoalesceCodeGen struct {
	buf      bytes.Buffer
	mappings []SourceMapping
	varCount int
}

// NewNullCoalesceCodeGen creates a new null coalesce code generator
func NewNullCoalesceCodeGen() *NullCoalesceCodeGen {
	return &NullCoalesceCodeGen{
		varCount: 0,
	}
}

// Generate produces Go code for a null coalescing expression
// Returns generated code and source mappings
func (g *NullCoalesceCodeGen) Generate(expr *NullCoalesceExpr, dingoStart int) ([]byte, []SourceMapping) {
	g.buf.Reset()
	g.mappings = nil

	// Collect all operands in the chain (right-associative)
	operands := g.collectChain(expr)

	// Generate: var coalesce T
	varName := "coalesce"
	if g.varCount > 0 {
		varName = fmt.Sprintf("coalesce%d", g.varCount-1)
	}
	g.varCount++

	startPos := g.buf.Len()
	g.buf.WriteString("var ")
	g.buf.WriteString(varName)
	g.buf.WriteString(" ")
	// Type will be inferred from first assignment
	g.buf.WriteString("interface{}")
	g.buf.WriteString("\n")

	// Track mapping for var declaration
	g.mappings = append(g.mappings, SourceMapping{
		DingoStart: dingoStart,
		DingoEnd:   dingoStart + len(operands[0]),
		GoStart:    startPos,
		GoEnd:      g.buf.Len(),
		Kind:       "null_coalesce_var",
	})

	// Generate if-else chain
	for i, operand := range operands {
		isLast := i == len(operands)-1

		if i == 0 {
			// First operand: if a != nil { coalesce = *a }
			startPos = g.buf.Len()
			g.buf.WriteString("if ")
			g.buf.WriteString(operand)
			g.buf.WriteString(" != nil {\n")
			g.buf.WriteString("\t")
			g.buf.WriteString(varName)
			g.buf.WriteString(" = *")
			g.buf.WriteString(operand)
			g.buf.WriteString("\n")

			g.mappings = append(g.mappings, SourceMapping{
				DingoStart: dingoStart,
				DingoEnd:   dingoStart + len(operand),
				GoStart:    startPos,
				GoEnd:      g.buf.Len(),
				Kind:       "null_coalesce_check",
			})

			g.buf.WriteString("}")
		} else if isLast {
			// Last operand (no nil check, final fallback)
			g.buf.WriteString(" else {\n")
			startPos = g.buf.Len()
			g.buf.WriteString("\t")
			g.buf.WriteString(varName)
			g.buf.WriteString(" = ")
			g.buf.WriteString(operand)
			g.buf.WriteString("\n")

			// Calculate approximate dingo position for this operand
			operandOffset := dingoStart
			for j := 0; j < i; j++ {
				operandOffset += len(operands[j]) + 3 // +3 for " ?? "
			}

			g.mappings = append(g.mappings, SourceMapping{
				DingoStart: operandOffset,
				DingoEnd:   operandOffset + len(operand),
				GoStart:    startPos,
				GoEnd:      g.buf.Len(),
				Kind:       "null_coalesce_default",
			})

			g.buf.WriteString("}")
		} else {
			// Middle operands: else if b != nil { coalesce = *b }
			g.buf.WriteString(" else if ")
			startPos = g.buf.Len()
			g.buf.WriteString(operand)
			g.buf.WriteString(" != nil {\n")
			g.buf.WriteString("\t")
			g.buf.WriteString(varName)
			g.buf.WriteString(" = *")
			g.buf.WriteString(operand)
			g.buf.WriteString("\n")

			// Calculate approximate dingo position for this operand
			operandOffset := dingoStart
			for j := 0; j < i; j++ {
				operandOffset += len(operands[j]) + 3 // +3 for " ?? "
			}

			g.mappings = append(g.mappings, SourceMapping{
				DingoStart: operandOffset,
				DingoEnd:   operandOffset + len(operand),
				GoStart:    startPos,
				GoEnd:      g.buf.Len(),
				Kind:       "null_coalesce_check",
			})

			g.buf.WriteString("}")
		}
	}

	g.buf.WriteString("\n")

	return g.buf.Bytes(), g.mappings
}

// collectChain collects all operands in a right-associative chain
// For a ?? b ?? c (parsed as a ?? (b ?? c)), returns [a, b, c]
func (g *NullCoalesceCodeGen) collectChain(expr *NullCoalesceExpr) []string {
	operands := []string{expr.LeftStr}

	// If right side is also a null coalesce, recurse
	if expr.Chain != nil && len(expr.Chain) > 0 {
		// Chain holds subsequent operands
		for _, chainExpr := range expr.Chain {
			operands = append(operands, chainExpr.LeftStr)
		}
		// Last operand is the rightmost
		operands = append(operands, expr.Chain[len(expr.Chain)-1].RightStr)
	} else {
		// Simple a ?? b
		operands = append(operands, expr.RightStr)
	}

	return operands
}

// TransformNullCoalesceSource transforms Dingo source containing ?? to Go source
// This is the main entry point
func TransformNullCoalesceSource(src []byte) ([]byte, []SourceMapping) {
	positions := FindNullCoalesceExpressions(src)
	if len(positions) == 0 {
		return src, nil
	}

	result := make([]byte, 0, len(src)*2)
	allMappings := []SourceMapping{}
	lastPos := 0
	codegen := NewNullCoalesceCodeGen()

	for _, opPos := range positions {
		// Copy source before this operator
		result = append(result, src[lastPos:opPos]...)

		// Parse the expression (simple for now - find left and right operands)
		left, right, endPos := parseNullCoalesceOperands(src, opPos)
		if left == "" || right == "" {
			// Parsing failed, keep original
			result = append(result, src[opPos:opPos+2]...)
			lastPos = opPos + 2
			continue
		}

		// Create expression node
		expr := &NullCoalesceExpr{
			LeftStr:  left,
			RightStr: right,
		}

		// Check for chaining (right side contains ??)
		expr.Chain = parseChain(right)

		// Generate Go code
		goCode, mappings := codegen.Generate(expr, opPos-len(left))

		// Adjust mapping positions based on result buffer position
		baseOffset := len(result)
		for i := range mappings {
			mappings[i].GoStart += baseOffset
			mappings[i].GoEnd += baseOffset
		}

		result = append(result, goCode...)
		allMappings = append(allMappings, mappings...)

		lastPos = endPos
	}

	// Copy remaining source
	result = append(result, src[lastPos:]...)

	return result, allMappings
}

// parseNullCoalesceOperands extracts left and right operands around ??
// Returns left operand, right operand, and end position
func parseNullCoalesceOperands(src []byte, opPos int) (string, string, int) {
	// Find left operand (go backwards from opPos)
	leftStart := opPos - 1
	for leftStart >= 0 && isIdentChar(src[leftStart]) {
		leftStart--
	}
	leftStart++ // Move to first char of identifier

	if leftStart >= opPos {
		return "", "", opPos + 2
	}

	left := string(src[leftStart:opPos])

	// Find right operand (go forwards from opPos+2)
	rightStart := opPos + 2
	// Skip whitespace
	for rightStart < len(src) && (src[rightStart] == ' ' || src[rightStart] == '\t') {
		rightStart++
	}

	rightEnd := rightStart
	// Read identifier or literal
	if rightEnd < len(src) && src[rightEnd] == '"' {
		// String literal
		rightEnd++
		for rightEnd < len(src) && src[rightEnd] != '"' {
			if src[rightEnd] == '\\' {
				rightEnd++
			}
			rightEnd++
		}
		rightEnd++ // Include closing quote
	} else if rightEnd < len(src) && src[rightEnd] >= '0' && src[rightEnd] <= '9' {
		// Number literal
		for rightEnd < len(src) && (isDigit(src[rightEnd]) || src[rightEnd] == '.') {
			rightEnd++
		}
	} else {
		// Identifier or expression
		parenDepth := 0
		for rightEnd < len(src) {
			ch := src[rightEnd]
			if ch == '(' {
				parenDepth++
			} else if ch == ')' {
				if parenDepth == 0 {
					break
				}
				parenDepth--
			} else if parenDepth == 0 && !isIdentChar(ch) && ch != '.' {
				break
			}
			rightEnd++
		}
	}

	right := string(src[rightStart:rightEnd])

	return left, right, rightEnd
}

// parseChain checks if the right operand contains chained ?? operators
func parseChain(operand string) []*NullCoalesceExpr {
	// For now, return nil - chaining will be handled by multiple passes
	// TODO: Implement proper chain parsing if needed
	return nil
}

// isDigit returns true if the character is a digit
func isDigit(ch byte) bool {
	return ch >= '0' && ch <= '9'
}
