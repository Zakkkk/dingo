package ast

import (
	"bytes"
	"fmt"
	"strings"
	"unicode"
)

// SafeNavCodeGen generates Go code from safe navigation expressions (?.operator).
// Transforms user?.name to inline nil checks with proper pointer handling.
type SafeNavCodeGen struct {
	buf     bytes.Buffer
	tmpVars map[string]int // Track temp variable counters
}

// NewSafeNavCodeGen creates a new safe navigation code generator.
func NewSafeNavCodeGen() *SafeNavCodeGen {
	return &SafeNavCodeGen{
		tmpVars: make(map[string]int),
	}
}

// Generate produces Go code for a safe navigation expression.
// Returns the generated Go code as bytes.
//
// Transformations:
//   user?.name → var nav *string; if user != nil { nav = &user.Name }
//   user?.address?.city → var nav *string; if user != nil && user.address != nil { nav = &user.address.city }
//   user?.getName() → var nav *string; if user != nil { tmp := user.getName(); nav = &tmp }
func (g *SafeNavCodeGen) Generate(expr *SafeNavExpr) []byte {
	g.buf.Reset()

	// Parse the chain: user?.address?.city
	chain := g.parseChain(expr)
	if len(chain) == 0 {
		return nil
	}

	// Determine the result type (use string as default for now)
	resultType := "*string"

	// Generate: var nav *string
	g.buf.WriteString("var nav ")
	g.buf.WriteString(resultType)
	g.buf.WriteString("\n")

	// Generate conditions: if user != nil && user.address != nil
	g.buf.WriteString("if ")
	for i := range chain {
		if i > 0 {
			g.buf.WriteString(" && ")
		}
		// Build the path up to this segment
		path := g.buildPath(chain[:i+1])
		g.buf.WriteString(path)
		g.buf.WriteString(" != nil")
	}
	g.buf.WriteString(" {\n\t")

	// Generate assignment: nav = &user.address.city
	fullPath := g.buildPath(chain)
	g.buf.WriteString("nav = &")
	g.buf.WriteString(fullPath)
	g.buf.WriteString("\n}")

	return g.buf.Bytes()
}

// GenerateCall produces Go code for a safe navigation method call.
// Returns the generated Go code as bytes.
//
// Transformation:
//   user?.getName() → var nav *string; if user != nil { tmp := user.getName(); nav = &tmp }
func (g *SafeNavCodeGen) GenerateCall(expr *SafeNavCallExpr) []byte {
	g.buf.Reset()

	receiver := g.extractIdentifier(expr.Receiver)
	method := expr.Method
	args := strings.Join(expr.ArgsStr, ", ")

	// Determine result type
	resultType := "*string"

	// Generate: var nav *string
	g.buf.WriteString("var nav ")
	g.buf.WriteString(resultType)
	g.buf.WriteString("\n")

	// Generate: if user != nil {
	g.buf.WriteString("if ")
	g.buf.WriteString(receiver)
	g.buf.WriteString(" != nil {\n\t")

	// Get temp var name
	tmpVar := g.getTmpVar("tmp")

	// Generate: tmp := user.getName()
	g.buf.WriteString(tmpVar)
	g.buf.WriteString(" := ")
	g.buf.WriteString(receiver)
	g.buf.WriteString(".")
	g.buf.WriteString(method)
	g.buf.WriteString("(")
	g.buf.WriteString(args)
	g.buf.WriteString(")\n\t")

	// Generate: nav = &tmp
	g.buf.WriteString("nav = &")
	g.buf.WriteString(tmpVar)
	g.buf.WriteString("\n}")

	return g.buf.Bytes()
}

// parseChain extracts the navigation chain from a safe nav expression.
// user?.address?.city → ["user", "address", "city"]
func (g *SafeNavCodeGen) parseChain(expr *SafeNavExpr) []string {
	if expr.Receiver == "" || expr.Field == "" {
		return nil
	}

	// Parse receiver: might be "user" or "user.address"
	parts := strings.Split(expr.Receiver, ".")

	// Add the current field
	parts = append(parts, expr.Field)

	return parts
}

// buildPath constructs the full access path from chain segments.
// ["user", "address", "city"] → "user.address.city"
func (g *SafeNavCodeGen) buildPath(segments []string) string {
	if len(segments) == 0 {
		return ""
	}
	return strings.Join(segments, ".")
}

// getTmpVar generates a unique temporary variable name.
// First call returns "tmp", subsequent calls return "tmp1", "tmp2", etc.
func (g *SafeNavCodeGen) getTmpVar(base string) string {
	count, exists := g.tmpVars[base]
	if !exists {
		g.tmpVars[base] = 1
		return base
	}
	g.tmpVars[base] = count + 1
	return fmt.Sprintf("%s%d", base, count)
}

// extractIdentifier extracts the base identifier from a receiver expression.
// "user" → "user"
// "user.address" → "user.address"
func (g *SafeNavCodeGen) extractIdentifier(receiver string) string {
	return strings.TrimSpace(receiver)
}

// FindSafeNavExpressions finds all occurrences of the ?. operator in source.
// Returns byte positions of each ?. occurrence.
func FindSafeNavExpressions(src []byte) []int {
	var positions []int
	inString := false
	inRawString := false
	inComment := false
	inLineComment := false
	var stringDelim byte

	for i := 0; i < len(src); i++ {
		ch := src[i]

		// Handle newlines (end line comments)
		if ch == '\n' {
			inLineComment = false
			continue
		}

		// Skip if in comment
		if inLineComment {
			continue
		}

		// Handle block comments
		if !inString && !inRawString && i+1 < len(src) {
			if ch == '/' && src[i+1] == '*' {
				inComment = true
				i++ // Skip *
				continue
			}
			if ch == '*' && src[i+1] == '/' {
				inComment = false
				i++ // Skip /
				continue
			}
			if ch == '/' && src[i+1] == '/' {
				inLineComment = true
				i++ // Skip second /
				continue
			}
		}

		if inComment {
			continue
		}

		// Handle raw strings
		if ch == '`' {
			inRawString = !inRawString
			continue
		}

		if inRawString {
			continue
		}

		// Handle regular strings
		if ch == '"' || ch == '\'' {
			if !inString {
				inString = true
				stringDelim = ch
			} else if ch == stringDelim {
				// Check if escaped
				if i > 0 && src[i-1] != '\\' {
					inString = false
				}
			}
			continue
		}

		if inString {
			continue
		}

		// Look for ?. operator
		if ch == '?' && i+1 < len(src) && src[i+1] == '.' {
			positions = append(positions, i)
			i++ // Skip the .
		}
	}

	return positions
}

// TransformSafeNavSource transforms Dingo source containing safe navigation to Go source.
// This is the main entry point for safe navigation transformation.
func TransformSafeNavSource(src []byte) ([]byte, []SourceMapping) {
	positions := FindSafeNavExpressions(src)
	if len(positions) == 0 {
		return src, nil
	}

	var mappings []SourceMapping
	result := make([]byte, 0, len(src)+500)
	lastPos := 0

	for _, pos := range positions {
		// Copy source before this ?.
		result = append(result, src[lastPos:pos]...)

		// Parse the safe navigation expression
		expr, endPos := parseSafeNavExpr(src, pos)
		if expr == nil {
			// Parsing failed, keep original
			result = append(result, src[pos:pos+2]...)
			lastPos = pos + 2
			continue
		}

		// Generate Go code
		codegen := NewSafeNavCodeGen()
		var goCode []byte

		if callExpr, ok := expr.(*SafeNavCallExpr); ok {
			goCode = codegen.GenerateCall(callExpr)
		} else if navExpr, ok := expr.(*SafeNavExpr); ok {
			goCode = codegen.Generate(navExpr)
		}

		if len(goCode) > 0 {
			goStart := len(result)
			result = append(result, goCode...)

			// Record source mapping
			mappings = append(mappings, SourceMapping{
				DingoStart: pos,
				DingoEnd:   endPos,
				GoStart:    goStart,
				GoEnd:      len(result),
				Kind:       "safe_nav",
			})
		}

		lastPos = endPos
	}

	// Copy remaining source
	result = append(result, src[lastPos:]...)

	return result, mappings
}

// parseSafeNavExpr parses a safe navigation expression starting at pos.
// Returns the parsed expression and the end position.
func parseSafeNavExpr(src []byte, pos int) (interface{}, int) {
	// pos points to the '?' in '?.'
	if pos+1 >= len(src) || src[pos+1] != '.' {
		return nil, pos + 1
	}

	// Parse backward to find receiver
	receiver := parseBackward(src, pos)
	if receiver == "" {
		return nil, pos + 2
	}

	// Parse forward to find field or method call
	fieldStart := pos + 2
	field, isCall, endPos := parseForward(src, fieldStart)
	if field == "" {
		return nil, fieldStart
	}

	if isCall {
		// Method call: user?.getName()
		method, args := splitMethodCall(field)
		return &SafeNavCallExpr{
			Receiver: receiver,
			Method:   method,
			ArgsStr:  []string{args},
		}, endPos
	}

	// Field access: user?.name
	return &SafeNavExpr{
		Receiver: receiver,
		Field:    field,
	}, endPos
}

// parseBackward extracts the receiver expression before ?.
// Handles simple identifiers and dot notation: user, user.address
func parseBackward(src []byte, pos int) string {
	if pos == 0 {
		return ""
	}

	end := pos
	start := pos - 1

	// Skip whitespace
	for start >= 0 && unicode.IsSpace(rune(src[start])) {
		start--
	}

	if start < 0 {
		return ""
	}

	// Find the start of the identifier
	for start >= 0 {
		ch := src[start]
		if !isIdentChar(ch) && ch != '.' {
			break
		}
		start--
	}
	start++

	if start >= end {
		return ""
	}

	return string(src[start:end])
}

// parseForward extracts the field or method after ?.
// Returns field name, whether it's a method call, and end position.
func parseForward(src []byte, start int) (string, bool, int) {
	// Skip whitespace
	pos := start
	for pos < len(src) && unicode.IsSpace(rune(src[pos])) {
		pos++
	}

	if pos >= len(src) {
		return "", false, start
	}

	// Read identifier
	idStart := pos
	for pos < len(src) && isIdentChar(src[pos]) {
		pos++
	}

	if pos == idStart {
		return "", false, start
	}

	field := string(src[idStart:pos])

	// Skip whitespace
	for pos < len(src) && unicode.IsSpace(rune(src[pos])) {
		pos++
	}

	// Check for method call
	if pos < len(src) && src[pos] == '(' {
		// Find closing paren
		parenDepth := 1
		pos++
		for pos < len(src) && parenDepth > 0 {
			if src[pos] == '(' {
				parenDepth++
			} else if src[pos] == ')' {
				parenDepth--
			}
			pos++
		}
		return field + string(src[idStart+len(field):pos]), true, pos
	}

	return field, false, pos
}

// splitMethodCall splits "getName()" into method name and args.
// Returns ("getName", "")
func splitMethodCall(call string) (string, string) {
	parenPos := strings.Index(call, "(")
	if parenPos == -1 {
		return call, ""
	}

	method := call[:parenPos]
	args := call[parenPos+1:]
	if strings.HasSuffix(args, ")") {
		args = args[:len(args)-1]
	}

	return method, args
}
