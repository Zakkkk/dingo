package ast

import (
	"bytes"
	"fmt"
	"unicode"
)

// TernaryCodeGen generates Go code from ternary expressions.
// Transforms: condition ? trueVal : falseVal
// To inline if statement (no IIFE):
//   var ternary T
//   if condition {
//       ternary = trueVal
//   } else {
//       ternary = falseVal
//   }
type TernaryCodeGen struct {
	buf     bytes.Buffer
	builder *MappingBuilder
	counter int // For unique variable naming
}

// NewTernaryCodeGen creates a new ternary code generator.
func NewTernaryCodeGen() *TernaryCodeGen {
	return &TernaryCodeGen{
		builder: NewMappingBuilder(),
		counter: 1, // Start at 1 (ternary, ternary1, ternary2...)
	}
}

// Generate produces Go code for a ternary expression.
// Returns the generated Go code and source mappings.
func (g *TernaryCodeGen) Generate(expr *TernaryExpr, dingoStart, dingoEnd int) CodeGenResult {
	g.buf.Reset()
	g.builder.Reset()

	varName := g.genVarName()
	resultType := expr.ResultType
	if resultType == "" {
		resultType = "any" // Fallback for untyped ternary
	}

	goStart := 0 // Will be adjusted by caller if needed

	// Generate: var ternary T
	varDecl := fmt.Sprintf("var %s %s\n", varName, resultType)
	g.buf.WriteString(varDecl)
	g.builder.Skip(len(varDecl))

	// Generate: if condition {
	ifLine := fmt.Sprintf("if %s {\n", expr.CondStr)
	g.buf.WriteString(ifLine)
	g.builder.Add(dingoStart, dingoStart+len(expr.CondStr), len(ifLine), "ternary_cond")

	// Generate: ternary = trueVal
	trueLine := fmt.Sprintf("\t%s = %s\n", varName, expr.TrueStr)
	g.buf.WriteString(trueLine)
	trueStart := dingoStart + len(expr.CondStr) + 1 // After '?'
	g.builder.Add(trueStart, trueStart+len(expr.TrueStr), len(trueLine), "ternary_true")

	// Generate: } else {
	elseLine := "} else {\n"
	g.buf.WriteString(elseLine)
	g.builder.Skip(len(elseLine))

	// Generate: ternary = falseVal
	falseLine := fmt.Sprintf("\t%s = %s\n", varName, expr.FalseStr)
	g.buf.WriteString(falseLine)
	falseStart := trueStart + len(expr.TrueStr) + 1 // After ':'
	g.builder.Add(falseStart, dingoEnd, len(falseLine), "ternary_false")

	// Generate: }
	closeBrace := "}\n"
	g.buf.WriteString(closeBrace)
	g.builder.Skip(len(closeBrace))

	// Add overall mapping for the entire ternary expression
	overallMapping := NewSourceMapping(dingoStart, dingoEnd, goStart, g.buf.Len(), "ternary")
	mappings := append([]SourceMapping{overallMapping}, g.builder.Build()...)

	return CodeGenResult{
		Output:   g.buf.Bytes(),
		Mappings: mappings,
	}
}

// genVarName generates a unique variable name for the ternary result.
func (g *TernaryCodeGen) genVarName() string {
	if g.counter == 1 {
		g.counter++
		return "ternary"
	}
	name := fmt.Sprintf("ternary%d", g.counter-1)
	g.counter++
	return name
}

// FindTernaryExpressions finds all ternary operator positions in source.
// Returns byte offsets of '?' characters that are part of ternary expressions.
// Distinguishes from ?? (null coalesce) and ?. (safe nav).
func FindTernaryExpressions(src []byte) []int {
	var positions []int
	inString := false
	inComment := false
	stringChar := byte(0)

	for i := 0; i < len(src); i++ {
		// Track string literals
		if (src[i] == '"' || src[i] == '`' || src[i] == '\'') && !inComment {
			if !inString {
				inString = true
				stringChar = src[i]
			} else if src[i] == stringChar && (i == 0 || src[i-1] != '\\') {
				inString = false
				stringChar = 0
			}
			continue
		}

		if inString {
			continue
		}

		// Track comments
		if i+1 < len(src) && src[i] == '/' && src[i+1] == '/' {
			inComment = true
			continue
		}
		if inComment {
			if src[i] == '\n' {
				inComment = false
			}
			continue
		}

		// Check for block comments
		if i+1 < len(src) && src[i] == '/' && src[i+1] == '*' {
			// Skip to end of block comment
			for j := i + 2; j+1 < len(src); j++ {
				if src[j] == '*' && src[j+1] == '/' {
					i = j + 1
					break
				}
			}
			continue
		}

		// Check for '?' that's not '??' or '?.'
		if src[i] == '?' {
			// Skip if it's ?? (null coalesce)
			if i+1 < len(src) && src[i+1] == '?' {
				i++ // Skip the second ?
				continue
			}
			// Skip if it's ?. (safe navigation)
			if i+1 < len(src) && src[i+1] == '.' {
				i++ // Skip the .
				continue
			}

			// Verify there's a ':' later (basic ternary validation)
			if hasMatchingColon(src, i) {
				positions = append(positions, i)
			}
		}
	}

	return positions
}

// hasMatchingColon checks if there's a ':' that could match this '?' for ternary.
// This is a heuristic check - full parsing happens later.
func hasMatchingColon(src []byte, questionPos int) bool {
	// Look ahead for ':' within reasonable distance (max 200 chars for simple cases)
	maxLookAhead := 200
	endPos := questionPos + maxLookAhead
	if endPos > len(src) {
		endPos = len(src)
	}

	inString := false
	stringChar := byte(0)
	parenDepth := 0
	braceDepth := 0

	for i := questionPos + 1; i < endPos; i++ {
		// Track string literals
		if (src[i] == '"' || src[i] == '`' || src[i] == '\'') {
			if !inString {
				inString = true
				stringChar = src[i]
			} else if src[i] == stringChar && (i == 0 || src[i-1] != '\\') {
				inString = false
				stringChar = 0
			}
			continue
		}

		if inString {
			continue
		}

		// Track nesting
		switch src[i] {
		case '(':
			parenDepth++
		case ')':
			parenDepth--
			if parenDepth < 0 {
				return false // Unmatched paren, not a valid ternary
			}
		case '{':
			braceDepth++
		case '}':
			braceDepth--
			if braceDepth < 0 {
				return false // Unmatched brace
			}
		case ':':
			// Found potential matching colon at same nesting level
			if parenDepth == 0 && braceDepth == 0 {
				return true
			}
		case ';', '\n':
			// End of statement without finding colon
			if parenDepth == 0 && braceDepth == 0 {
				return false
			}
		}
	}

	return false
}

// parseTernaryExpr extracts a ternary expression starting at the given position.
// Returns the TernaryExpr, end offset, and any error.
func parseTernaryExpr(src []byte, questionPos int) (*TernaryExpr, int, error) {
	// Extract condition (backward scan from ?)
	condStart := findConditionStart(src, questionPos)
	if condStart == -1 {
		return nil, 0, fmt.Errorf("could not find condition start before '?'")
	}
	condStr := string(bytes.TrimSpace(src[condStart:questionPos]))

	// Extract true branch (from ? to :)
	colonPos := findMatchingColon(src, questionPos)
	if colonPos == -1 {
		return nil, 0, fmt.Errorf("missing ':' in ternary expression")
	}
	trueStr := string(bytes.TrimSpace(src[questionPos+1 : colonPos]))

	// Extract false branch (from : to end of expression)
	falseEnd := findExpressionEnd(src, colonPos+1)
	falseStr := string(bytes.TrimSpace(src[colonPos+1 : falseEnd]))

	// Infer result type (simplified - would need full type inference in real impl)
	resultType := inferTernaryType(trueStr, falseStr)

	return &TernaryExpr{
		CondStr:    condStr,
		TrueStr:    trueStr,
		FalseStr:   falseStr,
		ResultType: resultType,
	}, falseEnd, nil
}

// findConditionStart scans backward from '?' to find where the condition starts.
func findConditionStart(src []byte, questionPos int) int {
	// Scan backward looking for statement boundary or operator
	depth := 0
	for i := questionPos - 1; i >= 0; i-- {
		ch := src[i]

		// Track nesting
		if ch == ')' {
			depth++
		} else if ch == '(' {
			depth--
			if depth < 0 {
				// Found start of condition
				return i + 1
			}
		}

		if depth > 0 {
			continue
		}

		// Stop at statement boundaries
		if ch == ';' || ch == '\n' || ch == '{' || ch == '}' {
			return skipWhitespace(src, i+1)
		}

		// Stop at assignment or declaration operators
		if ch == '=' || ch == ':' {
			// Check if it's := or ==
			if i > 0 && (src[i-1] == ':' || src[i-1] == '=' || src[i-1] == '!') {
				continue
			}
			return skipWhitespace(src, i+1)
		}
	}

	return 0 // Start of file
}

// findMatchingColon finds the ':' that matches the '?' at questionPos.
func findMatchingColon(src []byte, questionPos int) int {
	inString := false
	stringChar := byte(0)
	depth := 0

	for i := questionPos + 1; i < len(src); i++ {
		// Track string literals
		if (src[i] == '"' || src[i] == '`' || src[i] == '\'') {
			if !inString {
				inString = true
				stringChar = src[i]
			} else if src[i] == stringChar && (i == 0 || src[i-1] != '\\') {
				inString = false
				stringChar = 0
			}
			continue
		}

		if inString {
			continue
		}

		// Track nested ternaries
		if src[i] == '?' {
			// Check it's not ?? or ?.
			if i+1 < len(src) && (src[i+1] == '?' || src[i+1] == '.') {
				continue
			}
			depth++
			continue
		}

		if src[i] == ':' {
			if depth == 0 {
				return i
			}
			depth--
		}
	}

	return -1
}

// findExpressionEnd finds where the false branch expression ends.
func findExpressionEnd(src []byte, start int) int {
	inString := false
	stringChar := byte(0)
	parenDepth := 0
	braceDepth := 0

	for i := start; i < len(src); i++ {
		// Track string literals
		if (src[i] == '"' || src[i] == '`' || src[i] == '\'') {
			if !inString {
				inString = true
				stringChar = src[i]
			} else if src[i] == stringChar && (i == 0 || src[i-1] != '\\') {
				inString = false
				stringChar = 0
			}
			continue
		}

		if inString {
			continue
		}

		// Track nesting
		switch src[i] {
		case '(':
			parenDepth++
		case ')':
			parenDepth--
			if parenDepth < 0 {
				return i // End at closing paren
			}
		case '{':
			braceDepth++
		case '}':
			braceDepth--
			if braceDepth < 0 {
				return i // End at closing brace
			}
		case ',', ';', '\n':
			if parenDepth == 0 && braceDepth == 0 {
				return i // End at delimiter
			}
		}
	}

	return len(src)
}

// inferTernaryType attempts to infer the result type from true/false branches.
// Simplified version - real implementation would need full type analysis.
func inferTernaryType(trueStr, falseStr string) string {
	// Check for string literals
	if (len(trueStr) > 0 && trueStr[0] == '"') || (len(falseStr) > 0 && falseStr[0] == '"') {
		return "string"
	}

	// Check for numeric literals
	if isNumeric(trueStr) && isNumeric(falseStr) {
		return "int"
	}

	// Check for boolean literals
	if (trueStr == "true" || trueStr == "false") && (falseStr == "true" || falseStr == "false") {
		return "bool"
	}

	// Default to any for complex expressions
	return "any"
}

// isNumeric checks if a string looks like a numeric literal.
func isNumeric(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, ch := range s {
		if !unicode.IsDigit(ch) && ch != '.' && ch != '-' {
			return false
		}
	}
	return true
}

// TransformTernarySource transforms Dingo source containing ternary expressions to Go source.
// This is the main entry point that will replace string-based ternary transformation.
func TransformTernarySource(src []byte) ([]byte, []SourceMapping) {
	positions := FindTernaryExpressions(src)
	if len(positions) == 0 {
		return src, nil
	}

	result := make([]byte, 0, len(src)+500)
	var allMappings []SourceMapping
	lastPos := 0
	goOffset := 0

	codegen := NewTernaryCodeGen()

	for _, questionPos := range positions {
		// Parse the ternary expression
		expr, endPos, err := parseTernaryExpr(src, questionPos)
		if err != nil {
			// Parsing failed, keep original source
			result = append(result, src[lastPos:questionPos+1]...)
			goOffset += (questionPos + 1) - lastPos
			lastPos = questionPos + 1
			continue
		}

		// Find actual start of the expression
		condStart := findConditionStart(src, questionPos)

		// Copy source before this ternary
		result = append(result, src[lastPos:condStart]...)
		goOffset += condStart - lastPos

		// Generate Go code
		genResult := codegen.Generate(expr, condStart, endPos)

		// Adjust mappings for current position
		genResult.AdjustGoPositions(goOffset)

		// Append generated code
		result = append(result, genResult.Output...)
		allMappings = append(allMappings, genResult.Mappings...)

		goOffset += len(genResult.Output)
		lastPos = endPos
	}

	// Copy remaining source
	result = append(result, src[lastPos:]...)

	return result, allMappings
}
