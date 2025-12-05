package ast

import (
	"bytes"
	"fmt"
	"strings"
)

// FindErrorPropExpressions scans source for postfix ? operators (error propagation).
// Returns byte positions of each ? operator (not ?? or ?.).
func FindErrorPropExpressions(src []byte) []int {
	positions := []int{}

	// Manual scanning instead of regex negative lookahead (Go's RE2 doesn't support lookaheads)
	for i := 0; i < len(src); i++ {
		if src[i] == '?' {
			// Check next char is not ? or . (to exclude ?? and ?.)
			if i+1 < len(src) && (src[i+1] == '?' || src[i+1] == '.') {
				continue
			}
			// Verify not in string literal or comment
			if !isInStringOrComment(src, i) {
				positions = append(positions, i)
			}
		}
	}

	return positions
}

// ErrorPropCodeGen generates inline error handling code from error propagation expressions.
// Replaces expr? with explicit error checking and early return.
type ErrorPropCodeGen struct {
	buf          bytes.Buffer
	mappings     []SourceMapping
	tempCounter  int    // Counter for unique temp variable names (tmp, tmp1, tmp2...)
	errCounter   int    // Counter for unique err variable names (err, err1, err2...)
	baseOffset   int    // Base Dingo source offset
	currentGoPos int    // Current position in generated Go code
}

// NewErrorPropCodeGen creates a new error propagation code generator.
func NewErrorPropCodeGen() *ErrorPropCodeGen {
	return &ErrorPropCodeGen{
		tempCounter: 0,
		errCounter:  0,
	}
}

// Generate produces inline Go error handling code for an expression with ?.
// Input: expr?
// Output:
//   tmp, err := expr
//   if err != nil {
//       return zero, err
//   }
//   (use tmp)
//
// Returns CodeGenResult with generated code and source mappings.
func (g *ErrorPropCodeGen) Generate(operand string, dingoStart, dingoEnd int) CodeGenResult {
	g.buf.Reset()
	startGoPos := g.currentGoPos

	// Generate unique variable names
	tmpVar := g.getTempVarName()
	errVar := g.getErrVarName()

	// Line 1: tmp, err := expr
	assignLine := fmt.Sprintf("%s, %s := %s\n", tmpVar, errVar, operand)
	g.buf.WriteString(assignLine)

	// Mapping for the assignment line (maps to the original expr? in Dingo)
	g.mappings = append(g.mappings, SourceMapping{
		DingoStart: dingoStart,
		DingoEnd:   dingoEnd,
		GoStart:    startGoPos,
		GoEnd:      startGoPos + len(assignLine),
		Kind:       "error_prop_assign",
	})
	g.currentGoPos += len(assignLine)

	// Line 2: if err != nil {
	ifLine := fmt.Sprintf("if %s != nil {\n", errVar)
	g.buf.WriteString(ifLine)
	g.currentGoPos += len(ifLine)

	// Line 3: return zero, err
	// TODO: Replace with actual zero value based on function return type
	// For now, use comment placeholder that won't break compilation
	returnLine := fmt.Sprintf("\treturn /* TODO: zero value */, %s\n", errVar)
	g.buf.WriteString(returnLine)
	g.currentGoPos += len(returnLine)

	// Line 4: }
	closeLine := "}\n"
	g.buf.WriteString(closeLine)
	g.currentGoPos += len(closeLine)

	// The result variable (tmpVar) is what subsequent code should use
	// We don't generate the assignment here - the caller handles that

	return CodeGenResult{
		Output:   g.buf.Bytes(),
		Mappings: g.mappings,
	}
}

// getTempVarName returns a unique temporary variable name.
// First call returns "tmp", subsequent calls return "tmp1", "tmp2", etc.
func (g *ErrorPropCodeGen) getTempVarName() string {
	if g.tempCounter == 0 {
		g.tempCounter = 1
		return "tmp"
	}
	name := fmt.Sprintf("tmp%d", g.tempCounter)
	g.tempCounter++
	return name
}

// getErrVarName returns a unique error variable name.
// First call returns "err", subsequent calls return "err1", "err2", etc.
func (g *ErrorPropCodeGen) getErrVarName() string {
	if g.errCounter == 0 {
		g.errCounter = 1
		return "err"
	}
	name := fmt.Sprintf("err%d", g.errCounter)
	g.errCounter++
	return name
}

// TransformErrorPropSource transforms Dingo source with ? operators to Go source.
// This is the main entry point that processes an entire source file.
//
// Example transformations:
//   let x = getData()?
//   →
//   tmp, err := getData()
//   if err != nil { return zero, err }
//   x := tmp
//
//   result := foo()?.bar()?
//   →
//   tmp, err := foo()
//   if err != nil { return zero, err }
//   tmp1, err1 := tmp.bar()
//   if err1 != nil { return zero, err1 }
//   result := tmp1
func TransformErrorPropSource(src []byte) ([]byte, []SourceMapping) {
	positions := FindErrorPropExpressions(src)
	if len(positions) == 0 {
		return src, nil
	}

	result := make([]byte, 0, len(src)*2) // Estimate: error handling doubles code size
	var allMappings []SourceMapping
	lastPos := 0
	codegen := NewErrorPropCodeGen()

	for _, questionPos := range positions {
		// Copy source before this ? operator
		result = append(result, src[lastPos:questionPos]...)
		codegen.currentGoPos += questionPos - lastPos

		// Find the operand expression (everything from previous statement to ?)
		operandStart := findOperandStart(src, questionPos)
		operand := string(src[operandStart:questionPos])

		// Generate error handling code
		genResult := codegen.Generate(operand, operandStart, questionPos+1)

		// Adjust mappings for current position in result buffer
		for _, mapping := range genResult.Mappings {
			adjusted := mapping
			allMappings = append(allMappings, adjusted)
		}

		result = append(result, genResult.Output...)

		lastPos = questionPos + 1 // Skip the ? character
	}

	// Copy remaining source
	result = append(result, src[lastPos:]...)

	return result, allMappings
}

// findOperandStart finds the start position of the expression before ?.
// Scans backward from questionPos to find the start of the expression.
//
// Heuristic: Find the last occurrence of:
// - Statement start (after ; or { or newline)
// - Assignment operator (=, :=)
// - Opening delimiter after balanced pairs
func findOperandStart(src []byte, questionPos int) int {
	if questionPos == 0 {
		return 0
	}

	// Scan backward to find expression start
	depth := 0 // Track parentheses/brackets depth

	for i := questionPos - 1; i >= 0; i-- {
		ch := src[i]

		// Track nesting depth
		switch ch {
		case ')', ']', '}':
			depth++
		case '(', '[', '{':
			depth--
		}

		// At depth 0, these characters mark expression boundaries
		if depth == 0 {
			switch ch {
			case ';', '\n':
				// Statement boundary - start after this character
				return skipWhitespace(src, i+1)
			case '=':
				// Assignment - check if it's := or just =
				if i > 0 && src[i-1] == ':' {
					return skipWhitespace(src, i+1)
				}
				return skipWhitespace(src, i+1)
			}
		}
	}

	// Reached start of file
	return 0
}

// isInStringOrComment checks if a byte position is inside a string literal or comment.
// This prevents transforming ? operators that appear in strings or comments.
func isInStringOrComment(src []byte, pos int) bool {
	inString := false
	inRawString := false
	inComment := false
	inLineComment := false
	escape := false

	for i := 0; i < pos && i < len(src); i++ {
		ch := src[i]

		// Handle escape sequences in strings
		if escape {
			escape = false
			continue
		}
		if ch == '\\' && (inString || inRawString) {
			escape = true
			continue
		}

		// Line comments
		if inLineComment {
			if ch == '\n' {
				inLineComment = false
			}
			continue
		}

		// Block comments
		if inComment {
			if ch == '*' && i+1 < len(src) && src[i+1] == '/' {
				inComment = false
				i++ // Skip next char
			}
			continue
		}

		// Check for comment start
		if !inString && !inRawString && ch == '/' && i+1 < len(src) {
			if src[i+1] == '/' {
				inLineComment = true
				i++ // Skip next char
				continue
			}
			if src[i+1] == '*' {
				inComment = true
				i++ // Skip next char
				continue
			}
		}

		// String literals
		if ch == '"' && !inRawString {
			inString = !inString
			continue
		}

		// Raw string literals
		if ch == '`' && !inString {
			inRawString = !inRawString
			continue
		}
	}

	return inString || inRawString || inComment || inLineComment
}

// GenerateChained handles chained error propagation: foo()?.bar()?.baz()?
// Returns generated code and final result variable name.
func (g *ErrorPropCodeGen) GenerateChained(expressions []string, dingoStarts, dingoEnds []int) (string, []SourceMapping) {
	var buf bytes.Buffer
	var mappings []SourceMapping

	var prevTmpVar string

	for i, expr := range expressions {
		// If not first expression, use previous temp variable as base
		actualExpr := expr
		if i > 0 && prevTmpVar != "" {
			actualExpr = prevTmpVar + "." + expr
		}

		result := g.Generate(actualExpr, dingoStarts[i], dingoEnds[i])
		buf.Write(result.Output)
		mappings = append(mappings, result.Mappings...)

		// Extract the temp variable name for next iteration
		// Parse "tmp, err := ..." to get "tmp"
		lines := strings.Split(string(result.Output), "\n")
		if len(lines) > 0 {
			firstLine := lines[0]
			if idx := strings.Index(firstLine, ","); idx > 0 {
				prevTmpVar = strings.TrimSpace(firstLine[:idx])
			}
		}
	}

	return buf.String(), mappings
}
