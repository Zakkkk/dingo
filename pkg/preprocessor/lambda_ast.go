package preprocessor

import (
	"fmt"
	"os"
	"strings"

	"github.com/MadAppGang/dingo/pkg/ast"
	"github.com/MadAppGang/dingo/pkg/config"
)

// LambdaASTProcessor converts lambda syntax to Go function literals using token-based parsing
// Supports two styles (config-driven):
// - TypeScript arrows: x => expr, (x) => expr, (x, y) => expr, (x: int) => expr
// - Rust pipes: |x| expr, |x, y| expr, |x: int| expr, |x: int| -> bool { ... }
//
// This replaces the regex-based approach in lambda.go, fixing bugs with:
// - Nested function calls in body: (x: int) => transform(foo(x, 1), 2)
// - Complex expressions with commas: (x) => fmt.Sprintf("%d,%d", x, x*2)
// - Accurate position tracking for error messages
type LambdaASTProcessor struct {
	style              ast.LambdaStyle
	source             []byte
	pos                int
	line               int
	col                int
	counter            int
	errors             []error
	strictTypeChecking bool
	bodyProcessors     []BodyProcessor
}

// NewLambdaASTProcessor creates a new AST-based lambda processor with default style (TypeScript)
func NewLambdaASTProcessor() *LambdaASTProcessor {
	return &LambdaASTProcessor{
		style:              ast.TypeScriptStyle,
		strictTypeChecking: false,
		bodyProcessors:     nil, // No body processors by default
	}
}

// NewLambdaASTProcessorWithBodyProcessors creates a lambda processor with injected body processors
// Body processors are applied to lambda bodies BEFORE wrapping in func() literal
// This enables decoupled architecture - lambda doesn't import concrete expression processors
func NewLambdaASTProcessorWithBodyProcessors(procs []BodyProcessor) *LambdaASTProcessor {
	return &LambdaASTProcessor{
		style:              ast.TypeScriptStyle,
		strictTypeChecking: false,
		bodyProcessors:     procs,
	}
}

// NewLambdaASTProcessorWithConfig creates a new lambda processor with config-driven style
func NewLambdaASTProcessorWithConfig(cfg *config.Config) *LambdaASTProcessor {
	style := ast.TypeScriptStyle // Default
	strictChecking := false       // Default

	if cfg != nil {
		if cfg.Features.LambdaStyle == "rust" {
			style = ast.RustStyle
		}
		strictChecking = false // Future: cfg.Features.StrictLambdaTypes
	}

	return &LambdaASTProcessor{
		style:              style,
		strictTypeChecking: strictChecking,
	}
}

// WithStrictTypeChecking enables strict type checking for standalone lambdas
func (p *LambdaASTProcessor) WithStrictTypeChecking(strict bool) *LambdaASTProcessor {
	p.strictTypeChecking = strict
	return p
}

// Name returns the processor name
func (p *LambdaASTProcessor) Name() string {
	return "lambda_ast"
}

// Process implements FeatureProcessor interface
func (p *LambdaASTProcessor) Process(source []byte) ([]byte, []Mapping, error) {
	result, _, err := p.ProcessInternal(string(source))
	return []byte(result), nil, err
}

// ProcessInternal transforms lambda syntax with metadata support
// Uses multi-pass approach to handle nested lambdas and currying: (x) => (y) => x + y
func (p *LambdaASTProcessor) ProcessInternal(code string) (string, []TransformMetadata, error) {
	p.source = []byte(code)
	p.pos = 0
	p.line = 1
	p.col = 1
	p.counter = 0
	p.errors = nil

	var allMetadata []TransformMetadata
	result := code
	maxPasses := 10 // Prevent infinite loops

	// Multi-pass transformation for nested lambdas
	for pass := 0; pass < maxPasses; pass++ {
		p.source = []byte(result)
		p.pos = 0
		p.line = 1
		p.col = 1

		// Find all lambda expressions in current pass
		lambdas := p.findLambdaExpressions()

		// If no lambdas found, we're done
		if len(lambdas) == 0 {
			break
		}

		// Build result by replacing lambdas from right to left (preserve indices)
		for i := len(lambdas) - 1; i >= 0; i-- {
			lambda := lambdas[i]

			// Generate Go function literal
			goCode := lambda.expr.ToGo()

			// Replace in source
			result = result[:lambda.start] + goCode + result[lambda.end:]

			// Add metadata
			marker := fmt.Sprintf("// dingo:l:%d", p.counter)
			allMetadata = append(allMetadata, TransformMetadata{
				Type:            "lambda",
				OriginalLine:    lambda.startLine,
				OriginalColumn:  lambda.start,
				OriginalLength:  lambda.end - lambda.start,
				OriginalText:    string(p.source[lambda.start:lambda.end]),
				GeneratedMarker: marker,
				ASTNodeType:     "FuncLit",
			})
			p.counter++
		}
	}

	// Return errors if any
	if len(p.errors) > 0 {
		return "", nil, p.errors[0]
	}

	return result, allMetadata, nil
}

// lambdaMatch represents a matched lambda expression
type lambdaMatch struct {
	start     int
	end       int
	startLine int
	expr      *ast.LambdaExpr
}

// findLambdaExpressions finds all lambda expressions using proper tokenization
func (p *LambdaASTProcessor) findLambdaExpressions() []lambdaMatch {
	var matches []lambdaMatch

	// Reset position for scanning
	p.pos = 0
	p.line = 1
	p.col = 1

	for p.pos < len(p.source) {
		// Skip whitespace
		p.skipWhitespace()

		// Skip comments
		if p.peek() == '/' && (p.peekN(1) == '/' || p.peekN(1) == '*') {
			p.skipComment()
			continue
		}

		// Skip string literals
		if p.peek() == '"' || p.peek() == '`' {
			p.skipString()
			continue
		}

		// Skip existing func literals (don't transform)
		if p.matchKeyword("func") && p.peek() == '(' {
			p.skipFuncLiteral()
			continue
		}

		// Try to match lambda - support BOTH styles in same file
		// Try TypeScript style first (=> operator)
		match := p.tryMatchTypeScriptLambda()
		if match == nil {
			// Try Rust style (|...|)
			match = p.tryMatchRustLambda()
		}

		if match != nil {
			// Pre-process lambda body through injected body processors
			if len(p.bodyProcessors) > 0 {
				// Use injected processors (new architecture)
				processedBody, err := processLambdaBody([]byte(match.expr.Body), p.bodyProcessors)
				if err != nil {
					// Log error but continue processing with original body
					// The error will be caught later during Go compilation
					fmt.Fprintf(os.Stderr, "warning: failed to process lambda body at line %d: %v\n", match.startLine, err)
				} else {
					// Update lambda body with processed version
					match.expr.Body = string(processedBody)
				}
			} else {
				// Fallback to inline processing (legacy - for backward compatibility)
				processedBody, err := p.processLambdaBodyLegacy(match.expr.Body, match.expr.ReturnType)
				if err != nil {
					// Log error but continue processing with original body
					fmt.Fprintf(os.Stderr, "warning: failed to process lambda body at line %d: %v\n", match.startLine, err)
				} else {
					// Update lambda body with processed version
					match.expr.Body = processedBody
				}
			}

			matches = append(matches, *match)
			continue
		}

		// Advance position
		p.advance()
	}

	return matches
}

// processLambdaBody processes lambda body through injected expression processors
// This is a standalone function that can be tested independently
// Returns the processed body or error if any processor fails
func processLambdaBody(body []byte, processors []BodyProcessor) ([]byte, error) {
	if len(processors) == 0 {
		return body, nil
	}

	processed := body
	for _, proc := range processors {
		var err error
		processed, err = proc.ProcessBody(processed)
		if err != nil {
			return nil, fmt.Errorf("lambda body processing (%s): %w", proc.Name(), err)
		}
	}
	return processed, nil
}

// processLambdaBodyLegacy is the old inline body processing method
// DEPRECATED: This will be removed once all processors implement BodyProcessor interface
// Handles error propagation (?), null coalescing (??), and safe navigation (?.)
// Returns the processed body with Dingo operators transformed to Go code
func (p *LambdaASTProcessor) processLambdaBodyLegacy(body string, returnType string) (string, error) {
	// Quick check: if no Dingo operators, return as-is
	if !containsDingoOperators(body) {
		return body, nil
	}

	// Determine if this is a block body or expression body
	trimmedBody := strings.TrimSpace(body)
	isBlock := strings.HasPrefix(trimmedBody, "{")

	var codeToProcess string
	if isBlock {
		// Block body: wrap in synthetic function for proper context
		if returnType == "" {
			returnType = "__TYPE_INFERENCE_NEEDED"
		}
		// Remove outer braces and wrap
		innerBlock := strings.TrimSpace(trimmedBody)
		if strings.HasPrefix(innerBlock, "{") && strings.HasSuffix(innerBlock, "}") {
			innerBlock = innerBlock[1 : len(innerBlock)-1]
		}
		codeToProcess = fmt.Sprintf("func __lambda__() %s {\n%s\n}", returnType, innerBlock)
	} else {
		// Expression body: wrap in synthetic function for proper context
		// This allows error prop processor to infer return types correctly
		if returnType == "" {
			returnType = "__TYPE_INFERENCE_NEEDED"
		}
		codeToProcess = fmt.Sprintf("func __lambda__() %s {\n\treturn %s\n}", returnType, body)
	}

	// Process operators in correct order (safe nav -> null coalesce -> error prop)
	// This matches the order in preprocessor.go's processor chain

	// 1. Safe navigation (?.)
	safeNav := NewSafeNavASTProcessor()
	processed1, _, err := safeNav.ProcessInternal(codeToProcess)
	if err != nil {
		return "", fmt.Errorf("safe navigation in lambda body: %w", err)
	}

	// 2. Null coalescing (??)
	nullCoalesce := NewNullCoalesceASTProcessor()
	processed2, _, err := nullCoalesce.ProcessInternal(processed1)
	if err != nil {
		return "", fmt.Errorf("null coalescing in lambda body: %w", err)
	}

	// 3. Error propagation (?)
	errorProp := NewErrorPropASTProcessor()
	processed3, _, err := errorProp.ProcessInternal(processed2)
	if err != nil {
		return "", fmt.Errorf("error propagation in lambda body: %w", err)
	}

	// Extract the processed body from synthetic function
	// Format: "func __lambda__() RetType {\n<BODY>\n}"

	// Find the opening brace
	openBrace := strings.Index(processed3, "{")
	if openBrace == -1 {
		return processed3, nil
	}

	// Find the matching closing brace (last one)
	closeBrace := strings.LastIndex(processed3, "}")
	if closeBrace == -1 || closeBrace <= openBrace {
		return processed3, nil
	}

	// Extract body between braces
	extractedBody := processed3[openBrace+1 : closeBrace]
	extractedBody = strings.TrimSpace(extractedBody)

	// Check if body became multi-statement (contains newlines with statements)
	// This happens when error propagation transforms a single expression into multiple statements
	lines := strings.Split(extractedBody, "\n")
	hasMultipleStatements := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Check for statement indicators (not comments, not empty)
		if trimmed != "" && !strings.HasPrefix(trimmed, "//") {
			// If we find multiple non-comment, non-empty lines, it's multi-statement
			if hasMultipleStatements {
				// Already found one, this is the second - definitely multi-statement
				isBlock = true
				break
			}
			hasMultipleStatements = true
		}
	}

	if isBlock {
		// Block body: wrap back in braces
		// Don't modify the body - it's already correct with its statements
		return "{ " + extractedBody + " }", nil
	}

	// Expression body: remove leading "return " if present
	extractedBody = strings.TrimSpace(extractedBody)
	if strings.HasPrefix(extractedBody, "return ") {
		extractedBody = strings.TrimPrefix(extractedBody, "return ")
		extractedBody = strings.TrimSpace(extractedBody)
	}

	return extractedBody, nil
}

// containsDingoOperators checks if the body contains any Dingo operators that need processing
func containsDingoOperators(body string) bool {
	// Check for error propagation (?), but exclude ternary (? :)
	if idx := strings.Index(body, "?"); idx != -1 {
		// Check if it's not a ternary operator
		// Simple heuristic: ternary always has : after the ?
		remaining := body[idx+1:]
		if colonIdx := strings.Index(remaining, ":"); colonIdx == -1 || strings.Index(remaining, "?") < colonIdx {
			// Either no colon, or another ? before colon = likely error prop
			return true
		}
	}

	// Check for null coalescing (??)
	if strings.Contains(body, "??") {
		return true
	}

	// Check for safe navigation (?.)
	if strings.Contains(body, "?.") {
		return true
	}

	return false
}

// tryMatchTypeScriptLambda tries to match TypeScript-style lambda: x => expr or (x, y) => expr
func (p *LambdaASTProcessor) tryMatchTypeScriptLambda() *lambdaMatch {
	saved := p.pos

	// Try single param without parens: x => expr
	if p.isIdentifierStart(p.peek()) {
		// Check if preceded by word char (if so, not a lambda)
		if saved > 0 && p.isWordChar(p.source[saved-1]) {
			return nil
		}

		startPos := p.pos
		startLine := p.line
		paramName := p.scanIdentifier()

		p.skipWhitespace()
		if p.peek() == '=' && p.peekN(1) == '>' {
			// Found single param lambda
			p.advance() // =
			p.advance() // >
			p.skipWhitespace()

			// Parse body
			body := p.scanLambdaBody()

			expr := &ast.LambdaExpr{
				Style: ast.TypeScriptStyle,
				Params: []ast.LambdaParam{
					{Name: paramName, Type: ""}, // No type, will get __TYPE_INFERENCE_NEEDED
				},
				Body:    body,
				IsBlock: strings.HasPrefix(strings.TrimSpace(body), "{"),
			}

			return &lambdaMatch{
				start:     startPos,
				end:       p.pos,
				startLine: startLine,
				expr:      expr,
			}
		}
	}

	// Reset and try multi-param with parens: (x, y) => expr
	p.pos = saved

	if p.peek() == '(' {
		// Check if preceded by word char (if so, might be function call, not lambda)
		if saved > 0 && p.isWordChar(p.source[saved-1]) {
			// Could be a function call with lambda argument
			// We'll still try to parse it
		}

		startPos := p.pos
		startLine := p.line
		p.advance() // (

		// Parse parameters
		params, err := p.parseParamList()
		if err != nil {
			p.pos = saved
			return nil
		}

		p.skipWhitespace()
		if p.peek() != ')' {
			p.pos = saved
			return nil
		}
		p.advance() // )

		// Check for return type annotation: ): type =>
		p.skipWhitespace()
		returnType := ""
		if p.peek() == ':' {
			p.advance() // :
			p.skipWhitespace()
			// Scan return type (identifier or complex type)
			returnType = p.scanTypeAnnotation()
			p.skipWhitespace()
		}

		// Must have =>
		if p.peek() != '=' || p.peekN(1) != '>' {
			p.pos = saved
			return nil
		}
		p.advance() // =
		p.advance() // >

		p.skipWhitespace()

		// Parse body
		body := p.scanLambdaBody()

		expr := &ast.LambdaExpr{
			Style:      ast.TypeScriptStyle,
			Params:     params,
			ReturnType: returnType,
			Body:       body,
			IsBlock:    strings.HasPrefix(strings.TrimSpace(body), "{"),
		}

		return &lambdaMatch{
			start:     startPos,
			end:       p.pos,
			startLine: startLine,
			expr:      expr,
		}
	}

	return nil
}

// tryMatchRustLambda tries to match Rust-style lambda: |x| expr or |x, y| -> type { expr }
func (p *LambdaASTProcessor) tryMatchRustLambda() *lambdaMatch {
	saved := p.pos

	if p.peek() != '|' {
		return nil
	}

	// Check if preceded by word char (if so, not a lambda)
	if saved > 0 && p.isWordChar(p.source[saved-1]) {
		return nil
	}

	// Special handling for || (either empty-param Rust lambda OR logical OR operator)
	if p.peekN(1) == '|' {
		// This is ||. Could be:
		// 1) Empty-param Rust lambda: || 42
		// 2) Logical OR in expression: x > 0 && x < 100 || y == 1

		// Heuristic: If we're inside a func() body (from previous lambda transformation),
		// treat || as logical OR, not a new lambda
		lookback := saved - 1
		foundFunc := false
		for lookback >= 0 && lookback >= saved-100 { // Check up to 100 chars back
			if lookback+5 <= len(p.source) && string(p.source[lookback:lookback+5]) == "func(" {
				foundFunc = true
				break
			}
			if p.source[lookback] == '\n' {
				// Crossed line boundary without finding func(
				break
			}
			lookback--
		}

		if foundFunc {
			// We're inside a func() body - treat || as logical OR, not lambda
			return nil
		}
		// Otherwise, treat || as empty-param Rust lambda (will be validated by param parsing)
	}

	startPos := p.pos
	startLine := p.line
	p.advance() // |

	// Parse parameters
	params, err := p.parseParamList()
	if err != nil {
		p.pos = saved
		return nil
	}

	if p.peek() != '|' {
		p.pos = saved
		return nil
	}
	p.advance() // |

	// Check for return type annotation: -> type
	p.skipWhitespace()
	returnType := ""
	if p.peek() == '-' && p.peekN(1) == '>' {
		p.advance() // -
		p.advance() // >
		p.skipWhitespace()
		returnType = p.scanTypeAnnotation()
		p.skipWhitespace()
	}

	// Parse body
	body := p.scanLambdaBody()

	expr := &ast.LambdaExpr{
		Style:      ast.RustStyle,
		Params:     params,
		ReturnType: returnType,
		Body:       body,
		IsBlock:    strings.HasPrefix(strings.TrimSpace(body), "{"),
	}

	return &lambdaMatch{
		start:     startPos,
		end:       p.pos,
		startLine: startLine,
		expr:      expr,
	}
}

// parseParamList parses comma-separated parameter list
// Format: name or name: type or name: type, name2: type2
func (p *LambdaASTProcessor) parseParamList() ([]ast.LambdaParam, error) {
	var params []ast.LambdaParam

	p.skipWhitespace()

	// Empty param list
	if p.peek() == ')' || p.peek() == '|' {
		return params, nil
	}

	for {
		p.skipWhitespace()

		// Parse parameter name
		if !p.isIdentifierStart(p.peek()) {
			return nil, fmt.Errorf("expected parameter name")
		}
		paramName := p.scanIdentifier()

		// Check for type annotation
		p.skipWhitespace()
		paramType := ""
		if p.peek() == ':' {
			p.advance() // :
			p.skipWhitespace()
			paramType = p.scanTypeAnnotation()
		}

		params = append(params, ast.LambdaParam{
			Name: paramName,
			Type: paramType,
		})

		p.skipWhitespace()

		// Check for more parameters
		if p.peek() == ',' {
			p.advance() // ,
			continue
		}

		// End of parameter list
		break
	}

	return params, nil
}

// scanTypeAnnotation scans a type annotation (identifier or complex type)
// Handles: int, string, Option<T>, map[string]int, func(int) bool, etc.
func (p *LambdaASTProcessor) scanTypeAnnotation() string {
	start := p.pos

	// Track if we've seen a complete type identifier
	seenIdent := false

	// Skip balanced delimiters for complex types
	for p.pos < len(p.source) {
		ch := p.peek()

		// Stop at delimiter that ends type annotation
		if ch == '=' || ch == '-' || ch == '|' || ch == ')' || ch == ',' || ch == '{' || ch == '\n' {
			break
		}

		// For Rust, '>' ends type but we need to check context (not inside angle brackets)
		if ch == '>' && p.peekN(1) != '>' {
			// Check if we're at top level (not inside brackets)
			// Simple heuristic: if we haven't seen matching '<', stop
			break
		}

		// Handle balanced brackets/parens
		if ch == '[' || ch == '(' || ch == '<' {
			p.skipBalanced(ch)
			seenIdent = true
			continue
		}

		// If we hit whitespace after seeing an identifier/type, check carefully
		if ch == ' ' || ch == '\t' {
			// If we've seen a complete type and hit whitespace, stop
			// unless the next char is clearly a type continuation
			next := p.peekN(1)
			if next == '[' || next == '<' || next == '(' {
				// Type continuation like "map [string]int"
				p.advance()
				continue
			}
			// Whitespace after complete type - stop
			if seenIdent {
				break
			}
			// Skip whitespace and continue
			p.advance()
			continue
		}

		// Mark that we've seen identifier characters
		if p.isIdentifierChar(ch) {
			seenIdent = true
		}

		p.advance()
	}

	return strings.TrimSpace(string(p.source[start:p.pos]))
}

// scanLambdaBody scans lambda body (expression or block)
// Uses balanced delimiter tracking to handle complex expressions
func (p *LambdaASTProcessor) scanLambdaBody() string {
	start := p.pos

	// Check if block body (starts with {)
	if p.peek() == '{' {
		// Scan block with balanced braces
		p.skipBalanced('{')
		return string(p.source[start:p.pos])
	}

	// Expression body - scan until delimiter at depth 0
	depth := 0
	for p.pos < len(p.source) {
		ch := p.peek()

		// Track depth
		switch ch {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
			// If depth goes negative, we've hit enclosing delimiter
			if depth < 0 {
				return strings.TrimSpace(string(p.source[start:p.pos]))
			}
		case ',':
			// Comma at depth 0 ends expression
			if depth == 0 {
				return strings.TrimSpace(string(p.source[start:p.pos]))
			}
		case '\n':
			// Newline at depth 0 ends expression
			if depth == 0 {
				return strings.TrimSpace(string(p.source[start:p.pos]))
			}
		}

		p.advance()
	}

	// Reached end of source
	return strings.TrimSpace(string(p.source[start:p.pos]))
}

// skipBalanced skips balanced delimiters (parens, brackets, braces, angle brackets)
func (p *LambdaASTProcessor) skipBalanced(open byte) {
	if p.pos >= len(p.source) {
		return
	}

	var close byte
	switch open {
	case '(':
		close = ')'
	case '[':
		close = ']'
	case '{':
		close = '}'
	case '<':
		close = '>'
	default:
		return
	}

	// Skip opening delimiter
	if p.peek() != open {
		return
	}
	p.advance()

	depth := 1
	for p.pos < len(p.source) && depth > 0 {
		ch := p.peek()

		// Skip string literals and comments within balanced section
		if ch == '"' || ch == '`' {
			p.skipString()
			continue
		}
		if ch == '/' && (p.peekN(1) == '/' || p.peekN(1) == '*') {
			p.skipComment()
			continue
		}

		if ch == open {
			depth++
		} else if ch == close {
			depth--
		}

		p.advance()
	}
}

// skipFuncLiteral skips an existing func literal (don't transform)
func (p *LambdaASTProcessor) skipFuncLiteral() {
	// Skip "func"
	for i := 0; i < 4; i++ {
		p.advance()
	}

	// Skip signature
	p.skipWhitespace()
	if p.peek() == '(' {
		p.skipBalanced('(')
	}

	// Skip return type(s)
	p.skipWhitespace()
	for p.pos < len(p.source) && p.peek() != '{' {
		if p.peek() == '(' {
			p.skipBalanced('(')
		}
		p.advance()
	}

	// Skip body
	if p.peek() == '{' {
		p.skipBalanced('{')
	}
}

// Tokenization helpers

func (p *LambdaASTProcessor) peek() byte {
	if p.pos >= len(p.source) {
		return 0
	}
	return p.source[p.pos]
}

func (p *LambdaASTProcessor) peekN(n int) byte {
	if p.pos+n >= len(p.source) {
		return 0
	}
	return p.source[p.pos+n]
}

func (p *LambdaASTProcessor) advance() {
	if p.pos < len(p.source) {
		if p.source[p.pos] == '\n' {
			p.line++
			p.col = 1
		} else {
			p.col++
		}
		p.pos++
	}
}

func (p *LambdaASTProcessor) skipWhitespace() {
	for p.pos < len(p.source) {
		ch := p.peek()
		if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
			p.advance()
		} else {
			break
		}
	}
}

func (p *LambdaASTProcessor) skipComment() {
	if p.peek() != '/' {
		return
	}

	if p.peekN(1) == '/' {
		// Line comment
		for p.pos < len(p.source) && p.peek() != '\n' {
			p.advance()
		}
		if p.peek() == '\n' {
			p.advance()
		}
	} else if p.peekN(1) == '*' {
		// Block comment
		p.advance() // /
		p.advance() // *
		for p.pos < len(p.source) {
			if p.peek() == '*' && p.peekN(1) == '/' {
				p.advance() // *
				p.advance() // /
				break
			}
			p.advance()
		}
	}
}

func (p *LambdaASTProcessor) skipString() {
	quote := p.peek()
	if quote != '"' && quote != '`' {
		return
	}

	p.advance() // opening quote

	for p.pos < len(p.source) {
		ch := p.peek()

		if ch == quote {
			p.advance() // closing quote
			break
		}

		// Handle escape sequences
		if ch == '\\' && quote == '"' {
			p.advance() // backslash
			if p.pos < len(p.source) {
				p.advance() // escaped char
			}
			continue
		}

		p.advance()
	}
}

func (p *LambdaASTProcessor) matchKeyword(keyword string) bool {
	if p.pos+len(keyword) > len(p.source) {
		return false
	}

	// Check keyword match
	match := string(p.source[p.pos:p.pos+len(keyword)]) == keyword

	// Check word boundary after keyword
	if match && p.pos+len(keyword) < len(p.source) {
		nextChar := p.source[p.pos+len(keyword)]
		if p.isWordChar(nextChar) {
			return false
		}
	}

	return match
}

func (p *LambdaASTProcessor) scanIdentifier() string {
	start := p.pos
	for p.pos < len(p.source) && p.isIdentifierChar(p.peek()) {
		p.advance()
	}
	return string(p.source[start:p.pos])
}

func (p *LambdaASTProcessor) isIdentifierStart(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_'
}

func (p *LambdaASTProcessor) isIdentifierChar(ch byte) bool {
	return p.isIdentifierStart(ch) || (ch >= '0' && ch <= '9')
}

func (p *LambdaASTProcessor) isWordChar(ch byte) bool {
	return p.isIdentifierChar(ch) || ch == '.'
}
