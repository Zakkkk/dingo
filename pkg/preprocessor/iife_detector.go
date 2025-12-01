package preprocessor

import (
	"unicode"
)

// IIFEDetector detects and extracts Immediately Invoked Function Expressions (IIFEs)
// An IIFE is a function literal that is immediately invoked: func(...) Type { ... }()
type IIFEDetector struct{}

// NewIIFEDetector creates a new IIFE detector
func NewIIFEDetector() *IIFEDetector {
	return &IIFEDetector{}
}

// FindIIFEBoundary returns (start, end) of IIFE containing position, or (-1,-1) if none
// An IIFE is: func(...) Type { ... }()
func (d *IIFEDetector) FindIIFEBoundary(line string, pos int) (int, int) {
	if pos < 0 || pos >= len(line) {
		return -1, -1
	}

	// Search backwards for potential IIFE start
	funcStart := d.findFuncKeyword(line, pos)
	if funcStart == -1 {
		return -1, -1
	}

	// Search forwards from funcStart to find the complete IIFE
	iifeEnd := d.findIIFEEnd(line, funcStart)
	if iifeEnd == -1 {
		return -1, -1
	}

	return funcStart, iifeEnd
}

// IsInsideIIFE returns true if position is within an IIFE
func (d *IIFEDetector) IsInsideIIFE(line string, pos int) bool {
	start, end := d.FindIIFEBoundary(line, pos)
	return start != -1 && end != -1
}

// ExtractIIFEAwareOperand extracts the left operand of an operator at opPos
// If the operand ends with an IIFE invocation (), includes the entire IIFE
// Example: "func() int { return 1 }() ?? 0" at ?? position -> returns "func() int { return 1 }()"
func (d *IIFEDetector) ExtractIIFEAwareOperand(line string, opPos int) string {
	if opPos <= 0 || opPos >= len(line) {
		return ""
	}

	// Skip whitespace backward from operator position
	pos := d.skipWhitespaceBackward(line, opPos-1)
	if pos < 0 {
		return ""
	}

	char := line[pos]

	// Dispatch to appropriate handler based on character at position
	switch {
	case char == ')':
		return d.extractCallOrIIFEOperand(line, pos, opPos)
	case char == ']':
		return d.extractArrayAccessOperand(line, pos)
	default:
		return d.extractIdentifierOperand(line, pos)
	}
}

// skipWhitespaceBackward skips whitespace characters backward from pos
func (d *IIFEDetector) skipWhitespaceBackward(line string, pos int) int {
	for pos >= 0 && unicode.IsSpace(rune(line[pos])) {
		pos--
	}
	return pos
}

// extractCallOrIIFEOperand handles expressions ending with ')' - could be IIFE, method call, or function call
func (d *IIFEDetector) extractCallOrIIFEOperand(line string, endPos, opPos int) string {
	// Find matching opening paren
	parenStart := d.findMatchingOpenParen(line, endPos)
	if parenStart == -1 {
		return ""
	}

	// Check if this is a method call chain: obj.method() or iife().method()
	if methodChain := d.tryExtractMethodChain(line, parenStart, endPos); methodChain != "" {
		return methodChain
	}

	// Check if this is an IIFE invocation pattern: }()
	if iifeOperand := d.tryExtractIIFE(line, parenStart, endPos, opPos); iifeOperand != "" {
		return iifeOperand
	}

	// Not an IIFE - extract regular function call or parenthesized expression
	return d.extractRegularCallOperand(line, parenStart, endPos)
}

// tryExtractMethodChain attempts to extract a method call chain (e.g., obj.method() or iife().field)
func (d *IIFEDetector) tryExtractMethodChain(line string, parenStart, endPos int) string {
	beforeParen := d.skipWhitespaceBackward(line, parenStart-1)
	if beforeParen < 0 {
		return ""
	}

	// If there's an identifier before the paren, check for dot before it
	if isIdentifierChar(line[beforeParen]) || line[beforeParen] == '_' {
		// Walk back through the identifier
		for beforeParen >= 0 && (isIdentifierChar(line[beforeParen]) || line[beforeParen] == '_') {
			beforeParen--
		}
		beforeParen = d.skipWhitespaceBackward(line, beforeParen)

		// Check for dot (method call or field access)
		if beforeParen >= 0 && line[beforeParen] == '.' {
			// Recursively extract the operand before the dot
			baseOperand := d.ExtractIIFEAwareOperand(line, beforeParen)
			if baseOperand != "" {
				// Find where baseOperand starts in the line
				baseStart := d.findSubstringPosition(line, baseOperand, beforeParen)
				if baseStart >= 0 {
					return line[baseStart : endPos+1]
				}
			}
		}
	}
	return ""
}

// tryExtractIIFE attempts to extract an IIFE (Immediately Invoked Function Expression)
func (d *IIFEDetector) tryExtractIIFE(line string, parenStart, endPos, opPos int) string {
	if parenStart <= 0 || line[parenStart-1] != '}' {
		return ""
	}

	// Check if the parens are empty (invocation, not call with args)
	if !d.areParensEmpty(line, parenStart, endPos) {
		return ""
	}

	// Find the start of the IIFE
	funcStart := d.findIIFEStart(line, parenStart-1)
	if funcStart == -1 {
		return ""
	}

	// Extend end position to include any method calls after the IIFE
	extendedEnd := d.extendToMethodCalls(line, endPos, opPos)

	// Check for binary operators before the IIFE
	start := d.maybeExtendToBinaryExpression(line, funcStart)

	return line[start : extendedEnd+1]
}

// areParensEmpty checks if parentheses contain only whitespace
func (d *IIFEDetector) areParensEmpty(line string, start, end int) bool {
	for i := start + 1; i < end; i++ {
		if !unicode.IsSpace(rune(line[i])) {
			return false
		}
	}
	return true
}

// extendToMethodCalls extends the end position to include method calls after the current position
func (d *IIFEDetector) extendToMethodCalls(line string, endPos, opPos int) int {
	extendedEnd := endPos
	pos := endPos + 1

	for pos < opPos {
		if unicode.IsSpace(rune(line[pos])) {
			pos++
			continue
		}

		if line[pos] != '.' {
			break
		}

		// Field access or method call
		pos++
		pos = d.skipWhitespaceForward(line, pos, opPos)

		// Read identifier
		if pos < opPos && (isIdentifierChar(line[pos]) || line[pos] == '_') {
			for pos < opPos && (isIdentifierChar(line[pos]) || line[pos] == '_') {
				pos++
			}

			// Check for method call
			if pos < opPos && line[pos] == '(' {
				closeParen := d.findMatchingCloseParen(line, pos)
				if closeParen != -1 && closeParen < opPos {
					pos = closeParen + 1
					extendedEnd = closeParen
				} else {
					break
				}
			} else {
				extendedEnd = pos - 1
			}
		} else {
			break
		}
	}

	return extendedEnd
}

// skipWhitespaceForward skips whitespace characters forward from pos up to limit
func (d *IIFEDetector) skipWhitespaceForward(line string, pos, limit int) int {
	for pos < limit && unicode.IsSpace(rune(line[pos])) {
		pos++
	}
	return pos
}

// maybeExtendToBinaryExpression checks if there's a binary operator before start and extends if so
func (d *IIFEDetector) maybeExtendToBinaryExpression(line string, start int) int {
	if start <= 0 {
		return start
	}

	beforeStart := d.skipWhitespaceBackward(line, start-1)
	if beforeStart < 0 {
		return start
	}

	// Check for binary operators
	if d.isBinaryOperator(line[beforeStart]) {
		return d.findExpressionStart(line, beforeStart)
	}

	return start
}

// isBinaryOperator checks if a character is a binary operator
func (d *IIFEDetector) isBinaryOperator(char byte) bool {
	return char == '+' || char == '-' || char == '*' || char == '/' || char == '%'
}

// extractRegularCallOperand extracts a regular function call or parenthesized expression
func (d *IIFEDetector) extractRegularCallOperand(line string, parenStart, endPos int) string {
	start := parenStart

	// Look for identifier before the opening paren
	beforePos := d.skipWhitespaceBackward(line, parenStart-1)

	// Extract method call chain or identifier
	if beforePos >= 0 && (isIdentifierChar(line[beforePos]) || line[beforePos] == '.') {
		for beforePos >= 0 && (isIdentifierChar(line[beforePos]) || line[beforePos] == '.') {
			beforePos--
		}
		start = beforePos + 1
	}

	return line[start : endPos+1]
}

// extractArrayAccessOperand handles array/map access expressions (e.g., arr[0], map[key])
func (d *IIFEDetector) extractArrayAccessOperand(line string, endPos int) string {
	// Find matching opening bracket
	bracketStart := d.findMatchingOpenBracket(line, endPos)
	if bracketStart == -1 {
		return ""
	}

	// Look for identifier before the opening bracket
	beforeBracket := d.skipWhitespaceBackward(line, bracketStart-1)

	start := bracketStart
	// Extract the identifier or selector expression
	if beforeBracket >= 0 && (isIdentifierChar(line[beforeBracket]) || line[beforeBracket] == '.') {
		for beforeBracket >= 0 && (isIdentifierChar(line[beforeBracket]) || line[beforeBracket] == '.') {
			beforeBracket--
		}
		start = beforeBracket + 1
	}

	return line[start : endPos+1]
}

// extractIdentifierOperand handles simple identifiers and field access expressions
func (d *IIFEDetector) extractIdentifierOperand(line string, endPos int) string {
	// Only handle identifiers and dots
	if !isIdentifierChar(line[endPos]) && line[endPos] != '.' {
		return ""
	}

	// Walk backwards through identifiers and dots
	start := endPos
	for start >= 0 && (isIdentifierChar(line[start]) || line[start] == '.') {
		start--
	}
	start++

	// Check for binary operators before the identifier
	start = d.maybeExtendToBinaryExpression(line, start)

	return line[start : endPos+1]
}

// findSubstringPosition finds the position of substring in line, searching backward from maxPos
func (d *IIFEDetector) findSubstringPosition(line, substring string, maxPos int) int {
	for i := 0; i <= maxPos; i++ {
		if i+len(substring) <= len(line) && line[i:i+len(substring)] == substring {
			return i
		}
	}
	return -1
}

// findFuncKeyword searches backwards from pos to find a 'func' keyword
func (d *IIFEDetector) findFuncKeyword(line string, pos int) int {
	// Search backwards for 'func' keyword
	for i := pos; i >= 4; i-- {
		if i+4 <= len(line) && line[i:i+4] == "func" {
			// Verify it's a word boundary
			if i == 0 || !isIdentifierChar(line[i-1]) {
				if i+4 >= len(line) || !isIdentifierChar(line[i+4]) {
					return i
				}
			}
		}
	}
	return -1
}

// findIIFEEnd finds the end of an IIFE starting at funcStart
// Returns the position after the invocation () or -1 if not found
func (d *IIFEDetector) findIIFEEnd(line string, funcStart int) int {
	if funcStart+4 > len(line) {
		return -1
	}

	pos := funcStart + 4 // Skip 'func'

	// Skip whitespace
	for pos < len(line) && unicode.IsSpace(rune(line[pos])) {
		pos++
	}

	// Expect parameter list '('
	if pos >= len(line) || line[pos] != '(' {
		return -1
	}

	// Find matching ')' for parameters
	pos = d.findMatchingCloseParen(line, pos)
	if pos == -1 {
		return -1
	}
	pos++ // Move past ')'

	// Skip whitespace and optional return type
	for pos < len(line) && (unicode.IsSpace(rune(line[pos])) || isIdentifierChar(line[pos]) || line[pos] == '*' || line[pos] == '[' || line[pos] == ']') {
		if line[pos] == '[' {
			// Skip generic type parameters like [T any]
			depth := 1
			pos++
			for pos < len(line) && depth > 0 {
				if line[pos] == '[' {
					depth++
				} else if line[pos] == ']' {
					depth--
				}
				pos++
			}
		} else {
			pos++
		}
	}

	// Expect function body '{'
	if pos >= len(line) || line[pos] != '{' {
		return -1
	}

	// Find matching '}' for body
	pos = d.findMatchingCloseBrace(line, pos)
	if pos == -1 {
		return -1
	}
	pos++ // Move past '}'

	// Skip whitespace
	for pos < len(line) && unicode.IsSpace(rune(line[pos])) {
		pos++
	}

	// Expect invocation '()'
	if pos+1 >= len(line) || line[pos] != '(' {
		return -1
	}

	// Find matching ')' for invocation
	pos = d.findMatchingCloseParen(line, pos)
	if pos == -1 {
		return -1
	}

	return pos
}

// findIIFEStart finds the start of an IIFE that ends with a '}' at bracePos
func (d *IIFEDetector) findIIFEStart(line string, bracePos int) int {
	if bracePos < 0 || bracePos >= len(line) || line[bracePos] != '}' {
		return -1
	}

	// Find matching '{'
	braceStart := d.findMatchingOpenBrace(line, bracePos)
	if braceStart == -1 {
		return -1
	}

	// Work backwards from '{' to find 'func' keyword
	pos := braceStart - 1

	// Skip whitespace and return type
	for pos >= 0 && (unicode.IsSpace(rune(line[pos])) || isIdentifierChar(line[pos]) || line[pos] == '*' || line[pos] == ']') {
		if line[pos] == ']' {
			// Skip generic type parameters backwards
			depth := 1
			pos--
			for pos >= 0 && depth > 0 {
				if line[pos] == ']' {
					depth++
				} else if line[pos] == '[' {
					depth--
				}
				pos--
			}
		} else {
			pos--
		}
	}

	// Expect ')' (end of parameters)
	if pos < 0 || line[pos] != ')' {
		return -1
	}

	// Find matching '(' for parameters
	parenStart := d.findMatchingOpenParen(line, pos)
	if parenStart == -1 {
		return -1
	}

	// Skip whitespace before '('
	pos = parenStart - 1
	for pos >= 0 && unicode.IsSpace(rune(line[pos])) {
		pos--
	}

	// Check for 'func' keyword
	if pos >= 3 && line[pos-3:pos+1] == "func" {
		// Verify word boundary
		if pos-3 == 0 || !isIdentifierChar(line[pos-4]) {
			return pos - 3
		}
	}

	return -1
}

// isValidIIFE verifies that the code from funcStart to invokeEnd is a valid IIFE
func (d *IIFEDetector) isValidIIFE(line string, funcStart, invokeEnd int) bool {
	if funcStart < 0 || invokeEnd >= len(line) || funcStart >= invokeEnd {
		return false
	}

	// Check for 'func' keyword at start
	if funcStart+4 > len(line) || line[funcStart:funcStart+4] != "func" {
		return false
	}

	// Check for invocation '()' at end
	if invokeEnd < 1 || line[invokeEnd] != ')' {
		return false
	}

	// Find the opening '(' of invocation
	parenStart := d.findMatchingOpenParen(line, invokeEnd)
	if parenStart < funcStart {
		return false
	}

	// Check that there's a '}' before the invocation '('
	pos := parenStart - 1
	for pos > funcStart && unicode.IsSpace(rune(line[pos])) {
		pos--
	}

	return pos > funcStart && line[pos] == '}'
}

// extractRegularOperand extracts a non-IIFE operand before opPos
func (d *IIFEDetector) extractRegularOperand(line string, opPos int) string {
	if opPos <= 0 {
		return ""
	}

	// Work backwards to find operand start
	pos := opPos - 1

	// Skip whitespace
	for pos >= 0 && unicode.IsSpace(rune(line[pos])) {
		pos--
	}

	if pos < 0 {
		return ""
	}

	end := pos

	// Handle different operand types
	if line[pos] == ')' {
		// Function call or parenthesized expression
		parenStart := d.findMatchingOpenParen(line, pos)
		if parenStart == -1 {
			return ""
		}
		pos = parenStart - 1

		// Skip whitespace
		for pos >= 0 && unicode.IsSpace(rune(line[pos])) {
			pos--
		}

		// If there's an identifier before '(', include it
		if pos >= 0 && (isIdentifierChar(line[pos]) || line[pos] == '.') {
			for pos >= 0 && (isIdentifierChar(line[pos]) || line[pos] == '.') {
				pos--
			}
		}
		pos++
	} else if isIdentifierChar(line[pos]) || line[pos] == '.' {
		// Identifier or selector expression
		for pos >= 0 && (isIdentifierChar(line[pos]) || line[pos] == '.') {
			pos--
		}
		pos++
	} else {
		// Other cases (literals, etc.)
		return ""
	}

	return line[pos : end+1]
}

// findMatchingOpenParen finds the matching '(' for a ')' at pos
func (d *IIFEDetector) findMatchingOpenParen(line string, pos int) int {
	if pos < 0 || pos >= len(line) || line[pos] != ')' {
		return -1
	}

	depth := 1
	pos--

	for pos >= 0 && depth > 0 {
		if line[pos] == ')' {
			depth++
		} else if line[pos] == '(' {
			depth--
		}
		if depth > 0 {
			pos--
		}
	}

	if depth == 0 {
		return pos
	}
	return -1
}

// findMatchingCloseParen finds the matching ')' for a '(' at pos
func (d *IIFEDetector) findMatchingCloseParen(line string, pos int) int {
	if pos < 0 || pos >= len(line) || line[pos] != '(' {
		return -1
	}

	depth := 1
	pos++

	for pos < len(line) && depth > 0 {
		if line[pos] == '(' {
			depth++
		} else if line[pos] == ')' {
			depth--
		}
		if depth > 0 {
			pos++
		}
	}

	if depth == 0 {
		return pos
	}
	return -1
}

// findMatchingOpenBrace finds the matching '{' for a '}' at pos
func (d *IIFEDetector) findMatchingOpenBrace(line string, pos int) int {
	if pos < 0 || pos >= len(line) || line[pos] != '}' {
		return -1
	}

	depth := 1
	pos--

	for pos >= 0 && depth > 0 {
		if line[pos] == '}' {
			depth++
		} else if line[pos] == '{' {
			depth--
		}
		if depth > 0 {
			pos--
		}
	}

	if depth == 0 {
		return pos
	}
	return -1
}

// findMatchingCloseBrace finds the matching '}' for a '{' at pos
func (d *IIFEDetector) findMatchingCloseBrace(line string, pos int) int {
	if pos < 0 || pos >= len(line) || line[pos] != '{' {
		return -1
	}

	depth := 1
	pos++

	for pos < len(line) && depth > 0 {
		if line[pos] == '{' {
			depth++
		} else if line[pos] == '}' {
			depth--
		}
		if depth > 0 {
			pos++
		}
	}

	if depth == 0 {
		return pos
	}
	return -1
}

// findMatchingOpenBracket finds the matching '[' for a ']' at pos
func (d *IIFEDetector) findMatchingOpenBracket(line string, pos int) int {
	if pos < 0 || pos >= len(line) || line[pos] != ']' {
		return -1
	}

	depth := 1
	pos--

	for pos >= 0 && depth > 0 {
		if line[pos] == ']' {
			depth++
		} else if line[pos] == '[' {
			depth--
		}
		if depth > 0 {
			pos--
		}
	}

	if depth == 0 {
		return pos
	}
	return -1
}

// findExpressionStart finds the start of an expression that ends at operatorPos
// This handles binary expressions like "a + b" where we want to extract from "a"
func (d *IIFEDetector) findExpressionStart(line string, operatorPos int) int {
	// Skip the operator
	pos := operatorPos - 1
	for pos >= 0 && unicode.IsSpace(rune(line[pos])) {
		pos--
	}

	if pos < 0 {
		return 0
	}

	// Now find the start of the left operand
	start := pos

	// Handle different operand types
	if line[pos] == ')' {
		// Parenthesized expression or function call
		parenStart := d.findMatchingOpenParen(line, pos)
		if parenStart >= 0 {
			start = parenStart
			// Check for identifier before paren
			beforeParen := parenStart - 1
			for beforeParen >= 0 && unicode.IsSpace(rune(line[beforeParen])) {
				beforeParen--
			}
			if beforeParen >= 0 && (isIdentifierChar(line[beforeParen]) || line[beforeParen] == '.') {
				for beforeParen >= 0 && (isIdentifierChar(line[beforeParen]) || line[beforeParen] == '.') {
					beforeParen--
				}
				start = beforeParen + 1
			}
		}
	} else if line[pos] == ']' {
		// Array/map access
		bracketStart := d.findMatchingOpenBracket(line, pos)
		if bracketStart >= 0 {
			start = bracketStart
			// Check for identifier before bracket
			beforeBracket := bracketStart - 1
			for beforeBracket >= 0 && unicode.IsSpace(rune(line[beforeBracket])) {
				beforeBracket--
			}
			if beforeBracket >= 0 && (isIdentifierChar(line[beforeBracket]) || line[beforeBracket] == '.') {
				for beforeBracket >= 0 && (isIdentifierChar(line[beforeBracket]) || line[beforeBracket] == '.') {
					beforeBracket--
				}
				start = beforeBracket + 1
			}
		}
	} else if isIdentifierChar(line[pos]) || line[pos] == '.' {
		// Identifier or selector
		for start >= 0 && (isIdentifierChar(line[start]) || line[start] == '.') {
			start--
		}
		start++
	} else if unicode.IsDigit(rune(line[pos])) {
		// Numeric literal
		for start >= 0 && (unicode.IsDigit(rune(line[start])) || line[start] == '.') {
			start--
		}
		start++
	}

	// Check if there's another operator before (for chained binary expressions)
	if start > 0 {
		beforeStart := start - 1
		for beforeStart >= 0 && unicode.IsSpace(rune(line[beforeStart])) {
			beforeStart--
		}
		// If there's another operator, recurse
		if beforeStart >= 0 && (line[beforeStart] == '+' || line[beforeStart] == '-' ||
			line[beforeStart] == '*' || line[beforeStart] == '/' || line[beforeStart] == '%') {
			return d.findExpressionStart(line, beforeStart)
		}
	}

	return start
}
