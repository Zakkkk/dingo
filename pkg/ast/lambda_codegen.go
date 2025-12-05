package ast

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
)

// LambdaCodeGen generates Go code from LambdaExpr AST nodes.
// This replaces the string-based transformLambdas function with proper AST-based generation.
type LambdaCodeGen struct {
	buf bytes.Buffer
}

// NewLambdaCodeGen creates a new lambda code generator.
func NewLambdaCodeGen() *LambdaCodeGen {
	return &LambdaCodeGen{}
}

// Generate produces Go code for a LambdaExpr.
// Returns the generated Go code as bytes.
// Uses 'any' type for untyped parameters following plan requirements.
func (g *LambdaCodeGen) Generate(expr *LambdaExpr) []byte {
	g.buf.Reset()

	// Function literal opening
	g.buf.WriteString("func(")

	// Parameters - use 'any' for untyped parameters
	for i, param := range expr.Params {
		if i > 0 {
			g.buf.WriteString(", ")
		}
		g.buf.WriteString(param.Name)
		if param.Type != "" {
			g.buf.WriteString(" ")
			g.buf.WriteString(param.Type)
		} else {
			// Use 'any' for untyped parameters (Go 1.18+)
			g.buf.WriteString(" any")
		}
	}

	g.buf.WriteString(")")

	// Return type - use 'any' if not specified
	if expr.ReturnType != "" {
		g.buf.WriteString(" ")
		g.buf.WriteString(expr.ReturnType)
	} else {
		g.buf.WriteString(" any")
	}

	// Body
	if expr.IsBlock {
		// Block body - pass through
		g.buf.WriteString(" ")
		g.buf.WriteString(strings.TrimSpace(expr.Body))
	} else {
		// Expression body - wrap in { return ... }
		g.buf.WriteString(" { return ")
		g.buf.WriteString(strings.TrimSpace(expr.Body))
		g.buf.WriteString(" }")
	}

	return g.buf.Bytes()
}

// FindLambdaExpressions scans source and returns byte positions where lambda expressions start.
// Detects both TypeScript style (=>) and Rust style (|...|).
func FindLambdaExpressions(src []byte) []int {
	var positions []int
	srcStr := string(src)

	// Pattern 1: TypeScript arrow syntax
	// Matches: x => expr, (x) => expr, (x, y) => expr, (x: int) => expr
	tsPattern := regexp.MustCompile(`\b\w+\s*=>|[\(\s]\([\w\s,:]+\)\s*=>`)
	tsMatches := tsPattern.FindAllStringIndex(srcStr, -1)
	for _, match := range tsMatches {
		positions = append(positions, match[0])
	}

	// Pattern 2: Rust pipe syntax
	// Matches: |x| expr, |x, y| expr, |x: int| expr, |x: int| -> int { ... }
	rustPattern := regexp.MustCompile(`\|[\w\s,:]+\|`)
	rustMatches := rustPattern.FindAllStringIndex(srcStr, -1)
	for _, match := range rustMatches {
		positions = append(positions, match[0])
	}

	// Sort positions (simple bubble sort for small lists)
	for i := 0; i < len(positions); i++ {
		for j := i + 1; j < len(positions); j++ {
			if positions[i] > positions[j] {
				positions[i], positions[j] = positions[j], positions[i]
			}
		}
	}

	return positions
}

// ParseLambdaExpr parses a lambda expression starting at the given position.
// Returns the parsed LambdaExpr, end offset, and any error.
// This is a simplified parser - full parser would be in lambda_parser.go.
func ParseLambdaExpr(src []byte, startPos int) (*LambdaExpr, int, error) {
	srcStr := string(src[startPos:])
	expr := &LambdaExpr{}

	// Detect style
	if strings.Contains(srcStr[:min(100, len(srcStr))], "=>") {
		expr.Style = TypeScriptStyle
		return parseTypeScriptLambda(srcStr, expr)
	} else if strings.HasPrefix(strings.TrimSpace(srcStr), "|") {
		expr.Style = RustStyle
		return parseRustLambda(srcStr, expr)
	}

	return nil, 0, fmt.Errorf("invalid lambda syntax at position %d", startPos)
}

// parseTypeScriptLambda parses TypeScript arrow syntax
func parseTypeScriptLambda(srcStr string, expr *LambdaExpr) (*LambdaExpr, int, error) {
	srcStr = strings.TrimSpace(srcStr)
	arrowIdx := strings.Index(srcStr, "=>")
	if arrowIdx == -1 {
		return nil, 0, fmt.Errorf("missing '=>' in TypeScript lambda")
	}

	// Parse parameters
	paramStr := strings.TrimSpace(srcStr[:arrowIdx])
	paramStr = strings.TrimPrefix(paramStr, "(")
	paramStr = strings.TrimSuffix(paramStr, ")")
	paramStr = strings.TrimSpace(paramStr)

	if paramStr != "" {
		params := strings.Split(paramStr, ",")
		for _, p := range params {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}

			// Check for type annotation (x: int)
			if strings.Contains(p, ":") {
				parts := strings.SplitN(p, ":", 2)
				expr.Params = append(expr.Params, LambdaParam{
					Name: strings.TrimSpace(parts[0]),
					Type: strings.TrimSpace(parts[1]),
				})
			} else {
				expr.Params = append(expr.Params, LambdaParam{
					Name: p,
					Type: "", // Will use 'any'
				})
			}
		}
	}

	// Parse body
	bodyStr := strings.TrimSpace(srcStr[arrowIdx+2:])
	if strings.HasPrefix(bodyStr, "{") {
		expr.IsBlock = true
		// Find matching }
		depth := 0
		for i, ch := range bodyStr {
			if ch == '{' {
				depth++
			} else if ch == '}' {
				depth--
				if depth == 0 {
					expr.Body = bodyStr[:i+1]
					return expr, arrowIdx + 2 + i + 1, nil
				}
			}
		}
		return nil, 0, fmt.Errorf("unclosed block in lambda body")
	} else {
		// Expression body - find end (simplified: until newline or semicolon)
		expr.IsBlock = false
		endIdx := strings.IndexAny(bodyStr, "\n;")
		if endIdx == -1 {
			endIdx = len(bodyStr)
		}
		expr.Body = bodyStr[:endIdx]
		return expr, arrowIdx + 2 + endIdx, nil
	}
}

// parseRustLambda parses Rust pipe syntax
func parseRustLambda(srcStr string, expr *LambdaExpr) (*LambdaExpr, int, error) {
	srcStr = strings.TrimSpace(srcStr)
	if !strings.HasPrefix(srcStr, "|") {
		return nil, 0, fmt.Errorf("invalid Rust lambda: must start with |")
	}

	// Find closing |
	closeIdx := strings.Index(srcStr[1:], "|")
	if closeIdx == -1 {
		return nil, 0, fmt.Errorf("missing closing | in Rust lambda")
	}
	closeIdx++ // Adjust for offset

	// Parse parameters
	paramStr := strings.TrimSpace(srcStr[1:closeIdx])
	if paramStr != "" {
		params := strings.Split(paramStr, ",")
		for _, p := range params {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}

			// Check for type annotation (x: int)
			if strings.Contains(p, ":") {
				parts := strings.SplitN(p, ":", 2)
				expr.Params = append(expr.Params, LambdaParam{
					Name: strings.TrimSpace(parts[0]),
					Type: strings.TrimSpace(parts[1]),
				})
			} else {
				expr.Params = append(expr.Params, LambdaParam{
					Name: p,
					Type: "", // Will use 'any'
				})
			}
		}
	}

	// Check for return type (-> Type)
	remaining := strings.TrimSpace(srcStr[closeIdx+1:])
	if strings.HasPrefix(remaining, "->") {
		remaining = strings.TrimSpace(remaining[2:])
		// Extract return type (until { or space)
		endIdx := strings.IndexAny(remaining, "{ \t\n")
		if endIdx == -1 {
			return nil, 0, fmt.Errorf("invalid return type syntax")
		}
		expr.ReturnType = strings.TrimSpace(remaining[:endIdx])
		remaining = strings.TrimSpace(remaining[endIdx:])
	}

	// Parse body
	if strings.HasPrefix(remaining, "{") {
		expr.IsBlock = true
		// Find matching }
		depth := 0
		for i, ch := range remaining {
			if ch == '{' {
				depth++
			} else if ch == '}' {
				depth--
				if depth == 0 {
					expr.Body = remaining[:i+1]
					return expr, closeIdx + 1 + (len(srcStr[closeIdx+1:]) - len(remaining)) + i + 1, nil
				}
			}
		}
		return nil, 0, fmt.Errorf("unclosed block in lambda body")
	} else {
		// Expression body
		expr.IsBlock = false
		endIdx := strings.IndexAny(remaining, "\n;")
		if endIdx == -1 {
			endIdx = len(remaining)
		}
		expr.Body = remaining[:endIdx]
		totalOffset := closeIdx + 1 + (len(srcStr[closeIdx+1:]) - len(remaining)) + endIdx
		return expr, totalOffset, nil
	}
}

// TransformLambdaSource transforms Dingo source containing lambdas to Go source.
// This is the main entry point that replaces the old string-based transformLambdas.
// Returns transformed source and source mappings.
func TransformLambdaSource(src []byte) ([]byte, []SourceMapping) {
	lambdaPositions := FindLambdaExpressions(src)
	if len(lambdaPositions) == 0 {
		return src, nil
	}

	var mappings []SourceMapping
	result := make([]byte, 0, len(src)+500)
	lastPos := 0

	for _, lambdaStart := range lambdaPositions {
		// Copy source before this lambda
		result = append(result, src[lastPos:lambdaStart]...)

		// Parse the lambda
		expr, endOffset, err := ParseLambdaExpr(src, lambdaStart)
		if err != nil {
			// Parsing failed, keep original source
			// Try to find a reasonable fallback (skip to next space or newline)
			skipLen := 1
			for i := lambdaStart + 1; i < len(src) && i < lambdaStart+20; i++ {
				if src[i] == ' ' || src[i] == '\n' || src[i] == '\t' {
					break
				}
				skipLen++
			}
			result = append(result, src[lambdaStart:lambdaStart+skipLen]...)
			lastPos = lambdaStart + skipLen
			continue
		}

		// Generate Go code
		codegen := NewLambdaCodeGen()
		goCode := codegen.Generate(expr)

		// Record source mapping
		goStart := len(result)
		result = append(result, goCode...)
		goEnd := len(result)

		mappings = append(mappings, SourceMapping{
			DingoStart: lambdaStart,
			DingoEnd:   lambdaStart + endOffset,
			GoStart:    goStart,
			GoEnd:      goEnd,
			Kind:       "lambda",
		})

		lastPos = lambdaStart + endOffset
	}

	// Copy remaining source
	result = append(result, src[lastPos:]...)

	return result, mappings
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
