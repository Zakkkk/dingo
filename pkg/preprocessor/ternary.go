package preprocessor

import (
	"bytes"
	"fmt"
	"strings"
)

// MaxTernaryNestingDepth is the maximum depth of nested ternary operators allowed.
// This prevents overly complex nested ternaries that harm readability.
const MaxTernaryNestingDepth = 3

// TernaryProcessor handles the ternary operator (condition ? trueValue : falseValue)
// Transforms: condition ? trueValue : falseValue → IIFE pattern
//
// Example transformation:
//   Dingo:  let x = age >= 18 ? "adult" : "minor"
//   Go:     var x = func() string {
//               if age >= 18 {
//                   return "adult"
//               }
//               return "minor"
//           }()
//
// Key Features:
// - Uses IIFE (Immediately Invoked Function Expression) pattern
// - Zero runtime overhead (compiler inlines IIFEs)
// - Works in any expression context
// - Supports nesting up to 3 levels (enforced)
// - Concrete type inference via TernaryTypeInferrer (string, int, bool)
//
// Processing Order:
// CRITICAL: Must run BEFORE ErrorPropProcessor to avoid ? conflicts
// Disambiguates: condition ? true : false (ternary) vs expr? (error propagation)
//
// Edge Cases Handled:
//
// 1. Comparison Operators:
//    ✅ a == b ? c : d     (== not treated as assignment)
//    ✅ a != b ? c : d     (!= properly excluded)
//    ✅ a >= b ? c : d     (>=, <= properly excluded)
//
// 2. String Literals:
//    ✅ cond ? "http://url" : "default"  (colon in string ignored)
//    ✅ cond ? `host:port` : "default"   (raw strings supported)
//    ✅ cond ? "Is this?" : "No"         (? in string ignored)
//
// 3. Nested Contexts:
//    ✅ ((a) ? (b) : (c))               (nested parentheses)
//    ✅ a ? (b ? c : d) : e             (nested ternaries up to 3 levels)
//    ✅ foo(a ? b : c)                  (function arguments)
//    ✅ struct{ f: a ? b : c }          (struct fields)
//
// Known Limitations:
//
// 1. Multi-ternary Per Line:
//    ❌ let x = a ? 1 : 2, y = b ? 3 : 4
//    Error emitted with clear message
//
// 2. String Interpolation:
//    ❌ message := f"Result: {cond ? 'yes' : 'no'}"
//    Not yet supported (future feature)
//
// 3. Tuple Context:
//    ❌ let x, y = cond ? (1, 2) : (3, 4)
//    Not yet supported (may be added in future)
type TernaryProcessor struct {
	// tmpCounter reserves naming for potential future nested ternary temp variables.
	// Currently unused but reserved for complex nested expression optimization.
	tmpCounter   int
	mappings     []Mapping             // Source mappings
	typeInferrer *TernaryTypeInferrer  // Concrete type inference
}

// ternaryPosition represents a located ternary operator in a line
type ternaryPosition struct {
	conditionStart int    // Start position of condition
	qPos           int    // Position of ? operator
	colonPos       int    // Position of : operator
	condition      string // Expression before ?
	trueVal        string // Expression between ? and :
	falseVal       string // Expression after :
}

// delimiterTracker tracks nested delimiters and string contexts while scanning.
// This helper consolidates duplicate delimiter tracking logic across multiple functions.
type delimiterTracker struct {
	parenDepth    int
	bracketDepth  int
	braceDepth    int
	inDoubleQuote bool
	inBacktick    bool
	escaped       bool
}

// process updates the tracker state based on the current character.
func (d *delimiterTracker) process(ch byte) {
	if d.escaped {
		d.escaped = false
		return
	}

	if ch == '\\' && !d.inBacktick {
		d.escaped = true
		return
	}

	if ch == '"' && !d.inBacktick {
		d.inDoubleQuote = !d.inDoubleQuote
		return
	}

	if ch == '`' && !d.inDoubleQuote {
		d.inBacktick = !d.inBacktick
		return
	}

	if d.inDoubleQuote || d.inBacktick {
		return
	}

	switch ch {
	case '(':
		d.parenDepth++
	case ')':
		d.parenDepth--
	case '[':
		d.bracketDepth++
	case ']':
		d.bracketDepth--
	case '{':
		d.braceDepth++
	case '}':
		d.braceDepth--
	}
}

// isAtTopLevel returns true if we're not inside any delimiters.
func (d *delimiterTracker) isAtTopLevel() bool {
	return d.parenDepth == 0 && d.bracketDepth == 0 && d.braceDepth == 0
}

// inString returns true if we're currently inside a string literal.
func (d *delimiterTracker) inString() bool {
	return d.inDoubleQuote || d.inBacktick
}

// NewTernaryProcessor creates a new ternary operator preprocessor
func NewTernaryProcessor() *TernaryProcessor {
	return &TernaryProcessor{
		tmpCounter:   0,
		mappings:     []Mapping{},
		typeInferrer: NewTernaryTypeInferrer(),
	}
}

// Name returns the processor name
func (t *TernaryProcessor) Name() string {
	return "ternary_operator"
}

// ProcessBody implements BodyProcessor interface for lambda body processing
func (t *TernaryProcessor) ProcessBody(body []byte) ([]byte, error) {
	result, _, err := t.Process(body)
	return result, err
}

// Process transforms ternary operators into IIFE patterns
func (t *TernaryProcessor) Process(source []byte) ([]byte, []Mapping, error) {
	// Initialize state
	t.mappings = []Mapping{}
	t.tmpCounter = 0

	// Split into lines for processing
	lines := strings.Split(string(source), "\n")

	var output bytes.Buffer
	inputLineNum := 0
	outputLineNum := 1 // Track current output line number (1-based)

	for inputLineNum < len(lines) {
		line := lines[inputLineNum]

		// Process the line, passing the current output line number
		transformed, newMappings, err := t.processLine(line, inputLineNum+1, outputLineNum)
		if err != nil {
			return nil, nil, fmt.Errorf("line %d: %w", inputLineNum+1, err)
		}
		output.WriteString(transformed)
		if inputLineNum < len(lines)-1 {
			output.WriteByte('\n')
		}

		// Add all mappings from this line
		if len(newMappings) > 0 {
			t.mappings = append(t.mappings, newMappings...)
		}

		// Update output line count
		newlineCount := strings.Count(transformed, "\n")
		linesOccupied := newlineCount + 1
		outputLineNum += linesOccupied

		inputLineNum++
	}

	return output.Bytes(), t.mappings, nil
}

// processLine processes a single line for ternary operators
// Returns: (transformed_text, mappings, error)
func (t *TernaryProcessor) processLine(line string, origLine, outLine int) (string, []Mapping, error) {
	// Quick check: does line contain ? operator?
	if !strings.Contains(line, "?") {
		// No ternary operator - return as-is
		return line, nil, nil
	}

	// Find all ternary positions in this line
	positions := t.findTernaryPositions(line)
	if len(positions) == 0 {
		// No ternary operators found (might be error prop or null coalesce)
		return line, nil, nil
	}

	// CRITICAL: Emit error for multiple ternaries per line
	if len(positions) > 1 {
		return "", nil, fmt.Errorf(
			"multiple ternary operators on one line not yet supported (found %d). "+
				"Please split into separate lines",
			len(positions))
	}

	pos := positions[0]

	// Extract the full ternary expression
	ternaryExpr := fmt.Sprintf("%s ? %s : %s", pos.condition, pos.trueVal, pos.falseVal)

	// Expand the ternary into IIFE pattern
	// Start at nesting level 1 (top-level ternary)
	iife, mappings, err := t.expandTernary(ternaryExpr, pos.condition, pos.trueVal, pos.falseVal, 1)
	if err != nil {
		return "", nil, err
	}

	// Replace ternary expression with IIFE in the original line
	// Calculate the actual start position (where the condition begins)
	conditionStart := t.findExpressionStart(line, pos.qPos)

	// Calculate the end position (after the false value)
	// Use findFalseValueEnd to properly handle balanced delimiters
	falseValStart := pos.colonPos + 1
	// Skip whitespace
	for falseValStart < len(line) && (line[falseValStart] == ' ' || line[falseValStart] == '\t') {
		falseValStart++
	}
	end := t.findFalseValueEnd(line, falseValStart)

	// Construct transformed line
	before := line[:conditionStart]
	after := ""
	if end < len(line) {
		after = line[end:]
	}

	// Ensure there's whitespace before the IIFE if needed
	// (e.g., "let x =" should become "let x = func()" not "let x =func()")
	if len(before) > 0 && before[len(before)-1] != ' ' && before[len(before)-1] != '\t' {
		before += " "
	}

	transformed := before + iife + after

	// Generate source mappings
	mappings = t.generateMappings(origLine, outLine, pos)

	return transformed, mappings, nil
}

// findTernaryPositions locates all ternary operators in a line
// Returns positions sorted by appearance
func (t *TernaryProcessor) findTernaryPositions(line string) []ternaryPosition {
	positions := []ternaryPosition{}

	// Scan line for ? operators
	for i := 0; i < len(line); i++ {
		if line[i] != '?' {
			continue
		}

		// Check if this is a ternary operator (not error prop or null coalesce)
		if !t.isTernaryOperator(line, i) {
			continue
		}

		// Find the matching : operator
		colonPos := t.findMatchingColon(line, i)
		if colonPos == -1 {
			// No matching : found - invalid ternary (will be caught as error later)
			continue
		}

		// Find the start of the ternary expression (after =, (, [, {, etc.)
		conditionStart := t.findExpressionStart(line, i)

		// Extract components
		condition := strings.TrimSpace(line[conditionStart:i])
		trueVal := strings.TrimSpace(line[i+1 : colonPos])

		// Extract falseVal with proper boundary detection (handle balanced parens)
		falseValEnd := t.findFalseValueEnd(line, colonPos+1)
		falseVal := strings.TrimSpace(line[colonPos+1 : falseValEnd])

		positions = append(positions, ternaryPosition{
			conditionStart: conditionStart,
			qPos:           i,
			colonPos:       colonPos,
			condition:      condition,
			trueVal:        trueVal,
			falseVal:       falseVal,
		})

		// Continue scanning for additional ternaries on the same line
		// Start scanning after the false value end
		i = falseValEnd - 1 // -1 because loop will increment
	}

	return positions
}

// findFalseValueEnd finds where the false value ends
// by tracking balanced parentheses, brackets, and braces
func (t *TernaryProcessor) findFalseValueEnd(line string, startPos int) int {
	tracker := &delimiterTracker{}

	for i := startPos; i < len(line); i++ {
		ch := line[i]

		// Track closing delimiters before processing (for unmatched check)
		isClosing := (ch == ')' || ch == ']' || ch == '}')
		wasAtTopLevel := tracker.isAtTopLevel()

		tracker.process(ch)

		// Check for unmatched closing delimiters
		if isClosing && wasAtTopLevel && !tracker.inString() {
			// Unmatched closing delimiter - end of false value
			return i
		}

		// At top level, check for expression terminators
		if tracker.isAtTopLevel() && !tracker.inString() {
			if ch == ',' || ch == ';' {
				return i
			}
		}
	}

	// Reached end of line - false value extends to end
	return len(line)
}

// findExpressionStart finds where a ternary expression starts
// by scanning backwards from ? to find the last assignment/delimiter
func (t *TernaryProcessor) findExpressionStart(line string, qPos int) int {
	// Track parenthesis depth to skip balanced pairs
	parenDepth := 0
	bracketDepth := 0
	braceDepth := 0

	// Scan backwards to find assignment operator or other delimiter
	for i := qPos - 1; i >= 0; i-- {
		ch := line[i]

		// Track closing delimiters (scanning backwards, so these increase depth)
		if ch == ')' {
			parenDepth++
			continue
		}
		if ch == ']' {
			bracketDepth++
			continue
		}
		if ch == '}' {
			braceDepth++
			continue
		}

		// Track opening delimiters (scanning backwards, so these decrease depth)
		if ch == '(' {
			if parenDepth > 0 {
				parenDepth--
				continue
			}
			// Unmatched opening paren - this is a delimiter
			return i + 1
		}
		if ch == '[' {
			if bracketDepth > 0 {
				bracketDepth--
				continue
			}
			// Unmatched opening bracket - this is a delimiter
			return i + 1
		}
		if ch == '{' {
			if braceDepth > 0 {
				braceDepth--
				continue
			}
			// Unmatched opening brace - this is a delimiter
			return i + 1
		}

		// Only check for delimiters when we're at depth 0 (not inside parens/brackets/braces)
		if parenDepth == 0 && bracketDepth == 0 && braceDepth == 0 {
			// Check for assignment operators (but NOT comparison operators)
			if ch == '=' {
				// CRITICAL FIX: Check if this is the second = in == (scanning backwards)
				if i > 0 && line[i-1] == '=' {
					continue // Skip second = in ==
				}

				// Check if next char makes it ==
				isDoubleChar := false
				if i+1 < len(line) && line[i+1] == '=' {
					isDoubleChar = true
				}

				// Check for !=, :=, <=, >= (look behind)
				if i > 0 {
					prevCh := line[i-1]
					if prevCh == '!' || prevCh == ':' || prevCh == '<' || prevCh == '>' {
						// This is part of comparison/walrus - keep scanning
						continue
					}
				}

				if isDoubleChar {
					continue // Skip first = in ==
				}

				// This is a standalone = (assignment)
				return i + 1
			}

			// Check for comma (function argument separator)
			if ch == ',' {
				return i + 1
			}

			// Check for "return " keyword
			if ch == 'n' && i >= 5 {
				// Check if this is part of "return "
				if line[i-5:i+1] == "return" {
					// Check if followed by whitespace
					if i+1 < len(line) && (line[i+1] == ' ' || line[i+1] == '\t') {
						return i + 2 // Return position after "return "
					}
				}
			}
		}
	}

	// No delimiter found - start from beginning
	return 0
}

// isTernaryOperator determines if a ? at position qPos is a ternary operator
// Disambiguates from error propagation (?) and null coalesce (??)
func (t *TernaryProcessor) isTernaryOperator(line string, qPos int) bool {
	// Rule 1: Check for ?? (null coalesce) - skip if found
	if qPos+1 < len(line) && line[qPos+1] == '?' {
		return false
	}

	// Rule 2: Check for preceding ?? (null coalesce) - skip if found
	if qPos > 0 && line[qPos-1] == '?' {
		return false
	}

	// Rule 3: Look for : after ? (not in string literals)
	// If found → ternary, else → error propagation (will be handled by ErrorPropProcessor)
	remainder := line[qPos+1:]
	return t.containsColonOutsideString(remainder)
}

// containsColonOutsideString checks if a string contains : outside of string literals
func (t *TernaryProcessor) containsColonOutsideString(s string) bool {
	inDoubleQuote := false
	inBacktick := false
	escaped := false

	for _, ch := range s {
		if escaped {
			escaped = false
			continue
		}

		// No escapes in raw strings (backticks)
		if ch == '\\' && !inBacktick {
			escaped = true
			continue
		}

		if ch == '"' && !inBacktick {
			inDoubleQuote = !inDoubleQuote
			continue
		}

		if ch == '`' && !inDoubleQuote {
			inBacktick = !inBacktick
			continue
		}

		// Found : outside of string - this is a ternary
		if ch == ':' && !inDoubleQuote && !inBacktick {
			return true
		}
	}

	return false
}

// findMatchingColon finds the : that matches a ? operator
// Returns position of : or -1 if not found
// Handles nested ternaries by tracking ? and : pairing
func (t *TernaryProcessor) findMatchingColon(line string, qPos int) int {
	// Scan from ? position to end of line
	// Track nesting depth: each ? increments, each : decrements
	// When depth reaches 0, we found our matching :
	tracker := &delimiterTracker{}
	depth := 1 // Start at 1 for the initial ?

	for i := qPos + 1; i < len(line); i++ {
		ch := line[i]
		tracker.process(ch)

		// Only process ? and : at top level (not inside delimiters)
		if tracker.isAtTopLevel() && !tracker.inString() {
			// Check for nested ternary operator
			if ch == '?' {
				// Check if this is actually a ternary (not ?? or error prop)
				if i+1 < len(line) && line[i+1] == '?' {
					// This is ??, skip it
					continue
				}
				if !t.containsColonAfter(line, i) {
					// No : after this ?, so it's error propagation, not ternary
					continue
				}
				// This is a nested ternary - increase depth
				depth++
				continue
			}

			// Found : outside of string and delimiters
			if ch == ':' {
				depth--
				if depth == 0 {
					// This is our matching :
					return i
				}
				continue
			}
		}
	}

	return -1 // No matching : found
}

// containsColonAfter checks if there's a : after position pos (not in strings/delimiters)
func (t *TernaryProcessor) containsColonAfter(line string, pos int) bool {
	tracker := &delimiterTracker{}

	for i := pos + 1; i < len(line); i++ {
		ch := line[i]
		tracker.process(ch)

		if ch == ':' && tracker.isAtTopLevel() && !tracker.inString() {
			return true
		}
	}
	return false
}

// truncateExpr truncates long expressions for error messages
func truncateExpr(expr string, maxLen int) string {
	if len(expr) <= maxLen {
		return expr
	}
	return expr[:maxLen] + "..."
}

// expandTernary expands a ternary expression into IIFE pattern
// Handles nested ternaries recursively with nesting depth check
func (t *TernaryProcessor) expandTernary(expr, condition, trueVal, falseVal string, nestingLevel int) (string, []Mapping, error) {
	// CRITICAL: Enforce maximum nesting depth
	if nestingLevel > MaxTernaryNestingDepth {
		return "", nil, fmt.Errorf(
			"ternary operator nesting too deep (level %d, max %d). "+
				"Consider extracting nested logic into variables for readability",
			nestingLevel, MaxTernaryNestingDepth,
		)
	}

	// Check if branches contain nested ternaries
	hasNestedTrue := t.containsTernary(trueVal)
	hasNestedFalse := t.containsTernary(falseVal)

	// Recursively process nested ternaries
	if hasNestedTrue {
		// Find nested ternary in trueVal
		nestedPos := t.findTernaryPositions(trueVal)
		if len(nestedPos) > 0 {
			np := nestedPos[0]
			expandedTrue, _, err := t.expandTernary(
				fmt.Sprintf("%s ? %s : %s", np.condition, np.trueVal, np.falseVal),
				np.condition,
				np.trueVal,
				np.falseVal,
				nestingLevel+1,
			)
			if err != nil {
				return "", nil, err
			}
			trueVal = expandedTrue
		}
	}

	if hasNestedFalse {
		// Find nested ternary in falseVal
		nestedPos := t.findTernaryPositions(falseVal)
		if len(nestedPos) > 0 {
			np := nestedPos[0]
			expandedFalse, _, err := t.expandTernary(
				fmt.Sprintf("%s ? %s : %s", np.condition, np.trueVal, np.falseVal),
				np.condition,
				np.trueVal,
				np.falseVal,
				nestingLevel+1,
			)
			if err != nil {
				return "", nil, err
			}
			falseVal = expandedFalse
		}
	}

	// Generate IIFE with concrete type inference
	returnType := t.detectTernaryType(trueVal, falseVal)
	iife := t.generateIIFE(condition, trueVal, falseVal, returnType)

	// Note: Source mappings are generated in processLine, not here
	// Nested ternaries inherit mappings from their parent
	mappings := []Mapping{}

	return iife, mappings, nil
}

// generateMappings creates source map entries for a ternary operator transformation
func (t *TernaryProcessor) generateMappings(origLine, outLine int, pos ternaryPosition) []Mapping {
	return []Mapping{
		// Map condition to "if condition" line
		{
			OriginalLine:    origLine,
			OriginalColumn:  pos.conditionStart,
			GeneratedLine:   outLine + 1, // "if condition {" line
			GeneratedColumn: 4,           // After "if "
			Length:          len(pos.condition),
		},
		// Map true value to "return trueVal" line
		{
			OriginalLine:    origLine,
			OriginalColumn:  pos.qPos + 1, // After ?
			GeneratedLine:   outLine + 2,  // "return trueVal" line
			GeneratedColumn: 10,           // After "return "
			Length:          len(pos.trueVal),
		},
		// Map false value to "return falseVal" line
		{
			OriginalLine:    origLine,
			OriginalColumn:  pos.colonPos + 1, // After :
			GeneratedLine:   outLine + 4,      // "return falseVal" line
			GeneratedColumn: 9,                // After "return "
			Length:          len(pos.falseVal),
		},
	}
}

// containsTernary checks if a string contains a ternary operator
func (t *TernaryProcessor) containsTernary(s string) bool {
	positions := t.findTernaryPositions(s)
	return len(positions) > 0
}

// detectTernaryType infers the concrete return type for a ternary operator
// based on the types of both branches.
//
// Examples:
//   - ("adult", "minor") → "string"
//   - (100, 200) → "int"
//   - ("text", 42) → "any"
func (t *TernaryProcessor) detectTernaryType(trueVal, falseVal string) string {
	return t.typeInferrer.InferBranchTypes(trueVal, falseVal)
}

// generateIIFE generates the IIFE pattern for a ternary operator
//
// Template:
//   func() returnType {
//       if condition {
//           return trueVal
//       }
//       return falseVal
//   }()
func (t *TernaryProcessor) generateIIFE(condition, trueVal, falseVal, returnType string) string {
	return fmt.Sprintf(`func() %s {
	if %s {
		return %s
	}
	return %s
}()`, returnType, condition, trueVal, falseVal)
}
