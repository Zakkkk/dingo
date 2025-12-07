package ast

// TransformEnumConstructors transforms variant constructor calls to use the NewVariant() pattern.
//
// Transformations:
// - Status.Pending{} -> NewStatusPending()
// - Result.Ok(x) -> NewResultOk(x)
// - Ok(x) -> NewResultOk(x)  (if registry knows Ok is a variant)
//
// IMPORTANT: This function skips transformations inside match expressions.
// Match patterns like `UserCreated(userID, email) =>` are NOT constructor calls,
// they are patterns for type assertions and should remain as-is.
//
// This function requires the enum registry from TransformEnumSource to know which
// identifiers are variants.
func TransformEnumConstructors(src []byte, enumRegistry map[string]string) []byte {
	if len(enumRegistry) == 0 {
		return src
	}

	result := make([]byte, 0, len(src))
	i := 0

	for i < len(src) {
		// Check if we're entering a match expression - skip its body
		if matchStart := matchMatchExpression(src, i); matchStart != -1 {
			// Find the end of the match expression (closing brace)
			matchEnd := findMatchExpressionEnd(src, matchStart)
			if matchEnd != -1 {
				// Copy the entire match expression as-is (no transformations)
				result = append(result, src[i:matchEnd+1]...)
				i = matchEnd + 1
				continue
			}
		}

		// Look for qualified constructor: EnumName.Variant{...} or EnumName.Variant(...)
		if match := matchQualifiedConstructor(src, i, enumRegistry); match != nil {
			// Copy everything before match
			result = append(result, src[i:match.start]...)

			// Generate NewEnumVariant call
			constructorName := "New" + match.enumName + match.variantName
			result = append(result, constructorName...)

			// Copy arguments (parentheses or braces)
			if match.hasArgs {
				result = append(result, '(')
				// Copy the argument content (skip the opening brace/paren)
				argContent := extractConstructorArgs(src, match.argStart, match.argEnd)
				result = append(result, argContent...)
				result = append(result, ')')
			} else {
				result = append(result, '(', ')')
			}

			i = match.end
			continue
		}

		// Look for unqualified constructor: Variant(...) where Variant is in registry
		if match := matchUnqualifiedConstructor(src, i, enumRegistry); match != nil {
			// Copy everything before match
			result = append(result, src[i:match.start]...)

			// Generate NewEnumVariant call
			constructorName := "New" + match.enumName + match.variantName
			result = append(result, constructorName...)

			// Copy arguments
			if match.hasArgs {
				result = append(result, '(')
				argContent := extractConstructorArgs(src, match.argStart, match.argEnd)
				result = append(result, argContent...)
				result = append(result, ')')
			} else {
				result = append(result, '(', ')')
			}

			i = match.end
			continue
		}

		// No match, copy character
		result = append(result, src[i])
		i++
	}

	return result
}

// matchMatchExpression checks if position i starts a 'match' keyword.
// Returns the position of the opening brace if found, -1 otherwise.
func matchMatchExpression(src []byte, i int) int {
	// Check for word boundary before 'match'
	if i > 0 && isIdentChar(src[i-1]) {
		return -1
	}

	// Check for 'match' keyword
	if i+5 > len(src) {
		return -1
	}
	if src[i] != 'm' || src[i+1] != 'a' || src[i+2] != 't' || src[i+3] != 'c' || src[i+4] != 'h' {
		return -1
	}

	// Check for word boundary after 'match'
	if i+5 < len(src) && isIdentChar(src[i+5]) {
		return -1
	}

	// Find the opening brace of the match expression
	j := i + 5
	braceCount := 0
	parenCount := 0
	for j < len(src) {
		ch := src[j]
		switch ch {
		case '(':
			parenCount++
		case ')':
			parenCount--
		case '{':
			if parenCount == 0 {
				return j // Found the opening brace of match body
			}
			braceCount++
		case '}':
			braceCount--
		}
		j++
	}

	return -1
}

// findMatchExpressionEnd finds the closing brace of a match expression.
// openBrace is the position of the opening '{' of the match body.
func findMatchExpressionEnd(src []byte, openBrace int) int {
	depth := 1
	i := openBrace + 1
	inString := false
	inRawString := false
	stringChar := byte(0)

	for i < len(src) {
		ch := src[i]

		// Track strings to avoid counting braces inside strings
		if !inString && !inRawString {
			if ch == '"' {
				inString = true
				stringChar = '"'
			} else if ch == '\'' {
				inString = true
				stringChar = '\''
			} else if ch == '`' {
				inRawString = true
			} else if ch == '{' {
				depth++
			} else if ch == '}' {
				depth--
				if depth == 0 {
					return i
				}
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

		i++
	}

	return -1 // No matching brace found
}

// constructorMatch represents a matched constructor call
type constructorMatch struct {
	start       int    // Start position in source
	end         int    // End position in source
	enumName    string // Enum type name
	variantName string // Variant name
	hasArgs     bool   // Whether constructor has arguments
	argStart    int    // Start of argument content
	argEnd      int    // End of argument content
}

// matchQualifiedConstructor matches EnumName.Variant{...} or EnumName.Variant(...)
func matchQualifiedConstructor(src []byte, pos int, registry map[string]string) *constructorMatch {
	// Pattern: CapitalizedIdent.CapitalizedIdent followed by { or (
	// Must start with uppercase letter
	if pos >= len(src) || src[pos] < 'A' || src[pos] > 'Z' {
		return nil
	}

	// Scan first identifier (enum name)
	i := pos
	for i < len(src) && isIdentChar(src[i]) {
		i++
	}
	if i == pos {
		return nil
	}
	enumName := string(src[pos:i])

	// Expect '.'
	if i >= len(src) || src[i] != '.' {
		return nil
	}
	i++

	// Second identifier must start with uppercase letter
	if i >= len(src) || src[i] < 'A' || src[i] > 'Z' {
		return nil
	}

	// Scan second identifier (variant name)
	variantStart := i
	for i < len(src) && isIdentChar(src[i]) {
		i++
	}
	if i == variantStart {
		return nil
	}
	variantName := string(src[variantStart:i])

	// Expect '{' or '('
	if i >= len(src) || (src[i] != '{' && src[i] != '(') {
		return nil
	}
	bracketType := src[i]
	bracketStart := i

	// Check if this is a known enum variant
	// Registry maps variant name (e.g., "UserCreated") to enum name (e.g., "Event")
	// For qualified syntax "Event.UserCreated", verify variantName is registered under enumName
	if registeredEnum, ok := registry[variantName]; ok && registeredEnum == enumName {
		// Find matching closing bracket
		bracketEnd := findMatchingBracket(src, bracketStart, bracketType)
		if bracketEnd == -1 {
			return nil
		}

		// Check if there are arguments
		hasArgs := bracketEnd > bracketStart+1
		var argStart, argEnd int
		if hasArgs {
			argStart = bracketStart + 1
			argEnd = bracketEnd
		}

		return &constructorMatch{
			start:       pos,
			end:         bracketEnd + 1,
			enumName:    enumName,
			variantName: variantName,
			hasArgs:     hasArgs,
			argStart:    argStart,
			argEnd:      argEnd,
		}
	}

	return nil
}

// matchUnqualifiedConstructor matches Variant(...) where Variant is in registry
func matchUnqualifiedConstructor(src []byte, pos int, registry map[string]string) *constructorMatch {
	// Check that previous character is NOT part of an identifier (word boundary)
	if pos > 0 && isIdentChar(src[pos-1]) {
		return nil // Part of larger identifier, not a standalone constructor
	}

	// Pattern: CapitalizedIdent followed by (
	// Must start with uppercase letter
	if pos >= len(src) || src[pos] < 'A' || src[pos] > 'Z' {
		return nil
	}

	// Scan identifier (variant name)
	i := pos
	for i < len(src) && isIdentChar(src[i]) {
		i++
	}
	if i == pos {
		return nil
	}
	variantName := string(src[pos:i])

	// Expect '('
	if i >= len(src) || src[i] != '(' {
		return nil
	}
	parenStart := i

	// Check if this variant is in registry (unqualified lookup)
	if enumName, ok := registry[variantName]; ok {
		// Find matching closing paren
		parenEnd := findMatchingBracket(src, parenStart, '(')
		if parenEnd == -1 {
			return nil
		}

		// Check if there are arguments
		hasArgs := parenEnd > parenStart+1
		var argStart, argEnd int
		if hasArgs {
			argStart = parenStart + 1
			argEnd = parenEnd
		}

		return &constructorMatch{
			start:       pos,
			end:         parenEnd + 1,
			enumName:    enumName,
			variantName: variantName,
			hasArgs:     hasArgs,
			argStart:    argStart,
			argEnd:      argEnd,
		}
	}

	return nil
}

// findMatchingBracket finds the matching closing bracket for opening bracket at pos
func findMatchingBracket(src []byte, pos int, openChar byte) int {
	if pos >= len(src) {
		return -1
	}

	var closeChar byte
	switch openChar {
	case '{':
		closeChar = '}'
	case '(':
		closeChar = ')'
	case '[':
		closeChar = ']'
	default:
		return -1
	}

	depth := 1
	i := pos + 1
	inString := false
	inRawString := false
	stringChar := byte(0)

	for i < len(src) {
		ch := src[i]

		// Track strings to avoid counting brackets inside strings
		if !inString && !inRawString {
			if ch == '"' {
				inString = true
				stringChar = '"'
			} else if ch == '\'' {
				inString = true
				stringChar = '\''
			} else if ch == '`' {
				inRawString = true
			} else if ch == openChar {
				depth++
			} else if ch == closeChar {
				depth--
				if depth == 0 {
					return i
				}
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

		i++
	}

	return -1 // No matching bracket found
}

// extractConstructorArgs extracts and cleans constructor arguments
func extractConstructorArgs(src []byte, start, end int) []byte {
	if start >= end {
		return nil
	}

	content := src[start:end]

	// Check if content contains a colon (struct literal syntax)
	// This is a semantic check on already-extracted content, not parsing
	hasColon := false
	for i := 0; i < len(content); i++ {
		if content[i] == ':' {
			hasColon = true
			break
		}
	}

	if hasColon {
		// This is a struct literal - extract just the values
		return extractStructLiteralValues(content)
	}

	// For tuple syntax (value1, value2), just return trimmed
	return trimSpaceBytes(content)
}

// extractStructLiteralValues converts {field: value, ...} to value, ...
// Uses bracket-aware parsing to handle nested structures
func extractStructLiteralValues(content []byte) []byte {
	result := make([]byte, 0, len(content))
	firstValue := true

	i := 0
	for i < len(content) {
		// Skip leading whitespace
		for i < len(content) && isWhitespace(content[i]) {
			i++
		}
		if i >= len(content) {
			break
		}

		// Find the colon that separates field from value
		colonPos := -1
		for j := i; j < len(content); j++ {
			if content[j] == ':' {
				colonPos = j
				break
			}
		}
		if colonPos == -1 {
			break // No more field:value pairs
		}

		// Skip whitespace after colon
		valueStart := colonPos + 1
		for valueStart < len(content) && isWhitespace(content[valueStart]) {
			valueStart++
		}

		// Find the end of the value (next top-level comma or end of content)
		valueEnd := findValueEnd(content, valueStart)

		// Add separator if not first
		if !firstValue {
			result = append(result, ',', ' ')
		}
		firstValue = false

		// Append the trimmed value
		value := trimSpaceBytes(content[valueStart:valueEnd])
		result = append(result, value...)

		// Move past the comma (if any)
		i = valueEnd
		if i < len(content) && content[i] == ',' {
			i++
		}
	}

	return result
}

// findValueEnd finds the end of a value, handling nested brackets
func findValueEnd(content []byte, start int) int {
	depth := 0
	inString := false
	inRawString := false
	stringChar := byte(0)

	for i := start; i < len(content); i++ {
		ch := content[i]

		if !inString && !inRawString {
			switch ch {
			case '"':
				inString = true
				stringChar = '"'
			case '\'':
				inString = true
				stringChar = '\''
			case '`':
				inRawString = true
			case '(', '{', '[':
				depth++
			case ')', '}', ']':
				depth--
			case ',':
				if depth == 0 {
					return i
				}
			}
		} else if inString {
			if ch == stringChar && (i == start || content[i-1] != '\\') {
				inString = false
			}
		} else if inRawString {
			if ch == '`' {
				inRawString = false
			}
		}
	}

	return len(content)
}

// trimSpaceBytes trims leading and trailing whitespace from bytes
func trimSpaceBytes(b []byte) []byte {
	start := 0
	for start < len(b) && isWhitespace(b[start]) {
		start++
	}
	end := len(b)
	for end > start && isWhitespace(b[end-1]) {
		end--
	}
	return b[start:end]
}

// isWhitespace checks if a byte is whitespace
func isWhitespace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

// isIdentChar checks if a byte is part of a Go identifier
func isIdentChar(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_'
}
