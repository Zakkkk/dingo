package preprocessor

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/scanner"
	"go/token"
	"go/types"
	"strings"
)

// ErrorPropASTProcessor handles the ? operator for error propagation using token-based parsing
// This replaces the buggy regex-based approach in error_prop.go
//
// Handles:
//   - let x = expr? → error handling block
//   - return expr? → error handling with return
//   - expr? "custom message" → error handling with custom message
//   - Chained calls: a.b()?.c()? → processes both ? operators
//   - Multi-line expressions: expr\n? → supported
//
// Correctly excludes:
//   - String literals: getValue("what?")?
//   - Ternary operators: condition ? true : false
type ErrorPropASTProcessor struct {
	tryCounter    int
	lines         []string
	currentFunc   *funcContext
	needsFmt      bool
	importTracker *ImportTracker
	config        *Config
}

// NewErrorPropASTProcessor creates a new AST-based error propagation processor
func NewErrorPropASTProcessor() *ErrorPropASTProcessor {
	return NewErrorPropASTProcessorWithConfig(nil)
}

// NewErrorPropASTProcessorWithConfig creates a new processor with custom config
func NewErrorPropASTProcessorWithConfig(config *Config) *ErrorPropASTProcessor {
	if config == nil {
		config = DefaultConfig()
	}
	return &ErrorPropASTProcessor{
		tryCounter: 1,
		config:     config,
	}
}

// Name returns the processor name
func (p *ErrorPropASTProcessor) Name() string {
	return "error_propagation_ast"
}

// Process is the legacy interface method (implements FeatureProcessor)
func (p *ErrorPropASTProcessor) Process(source []byte) ([]byte, []Mapping, error) {
	result, _, err := p.ProcessInternal(string(source))
	return []byte(result), nil, err
}

// ProcessV2 implements FeatureProcessorV2 interface with metadata support
func (p *ErrorPropASTProcessor) ProcessV2(source []byte) (ProcessResult, error) {
	transformed, metadata, err := p.ProcessInternal(string(source))
	if err != nil {
		return ProcessResult{}, err
	}

	return ProcessResult{
		Source:   []byte(transformed),
		Mappings: nil,
		Metadata: metadata,
	}, nil
}

// ProcessInternal transforms error propagation operators with metadata support
func (p *ErrorPropASTProcessor) ProcessInternal(code string) (string, []TransformMetadata, error) {
	// Initialize import tracker
	p.importTracker = NewImportTracker()

	// Split into lines for processing
	p.lines = strings.Split(code, "\n")
	p.needsFmt = false

	var output bytes.Buffer
	inputLineNum := 0
	outputLineNum := 1
	markerCounter := 0
	var metadata []TransformMetadata

	for inputLineNum < len(p.lines) {
		line := p.lines[inputLineNum]

		// Check if this is a function declaration
		if p.isFunctionDeclaration(line) {
			p.currentFunc = p.parseFunctionSignature(inputLineNum)
			p.tryCounter = 1 // Reset counter for each function
		}

		// Process the line with metadata collection
		transformed, meta, err := p.processLineWithMetadata(line, inputLineNum+1, outputLineNum, &markerCounter)
		if err != nil {
			return "", nil, fmt.Errorf("line %d: %w", inputLineNum+1, err)
		}
		output.WriteString(transformed)
		if inputLineNum < len(p.lines)-1 {
			output.WriteByte('\n')
		}

		// Add metadata if generated
		if meta != nil {
			metadata = append(metadata, *meta)
		}

		// Update output line count
		newlineCount := strings.Count(transformed, "\n")
		linesOccupied := newlineCount + 1
		outputLineNum += linesOccupied

		inputLineNum++
	}

	return output.String(), metadata, nil
}

// GetNeededImports implements the ImportProvider interface
func (p *ErrorPropASTProcessor) GetNeededImports() []string {
	imports := p.importTracker.GetNeededImports()

	// Add fmt if needed for error messages
	if p.needsFmt {
		hasFmt := false
		for _, pkg := range imports {
			if pkg == "fmt" {
				hasFmt = true
				break
			}
		}
		if !hasFmt {
			imports = append(imports, "fmt")
		}
	}

	return imports
}

// Token represents a parsed token with its position and literal value
type Token struct {
	tok token.Token
	lit string
	pos int // Byte position in source
}

// processLineWithMetadata processes a single line with metadata generation
func (p *ErrorPropASTProcessor) processLineWithMetadata(line string, originalLineNum int, outputLineNum int, markerCounter *int) (string, *TransformMetadata, error) {
	// Quick check: does line contain ?
	if !strings.Contains(line, "?") {
		return line, nil, nil
	}

	// Skip if line contains ?? null coalesce operator
	if strings.Contains(line, "??") {
		return line, nil, nil
	}

	// Tokenize the line
	tokens := p.tokenizeLine(line)
	if len(tokens) == 0 {
		return line, nil, nil
	}

	// Find error propagation operator (? that's NOT ternary)
	qIdx := p.findErrorPropOperator(tokens)
	if qIdx == -1 {
		// No error propagation operator found
		return line, nil, nil
	}

	// Extract expression before ?
	expr := p.extractExpression(tokens, qIdx)
	if expr == "" {
		return line, nil, nil
	}

	// Check for custom error message after ?
	errMsg := p.extractErrorMessage(tokens, qIdx)

	// Track function call for import detection
	p.trackFunctionCallInExpr(expr)

	// Determine if this is assignment or return
	isReturn := p.isReturnStatement(tokens)

	// Generate unique marker
	marker := fmt.Sprintf("// dingo:e:%d", *markerCounter)
	*markerCounter++

	// Create metadata
	meta := &TransformMetadata{
		Type:            "error_prop",
		OriginalLine:    originalLineNum,
		OriginalColumn:  tokens[qIdx].pos + 1,
		OriginalLength:  1,
		OriginalText:    "?",
		GeneratedMarker: marker,
		ASTNodeType:     "IfStmt",
	}

	// Generate expansion
	var result string
	var err error
	if isReturn {
		result, err = p.expandReturn(line, expr, errMsg, marker)
	} else {
		// Extract variable name for assignment
		varName := p.extractVarName(tokens)
		result, err = p.expandAssignment(line, varName, expr, errMsg, marker)
	}

	if err != nil {
		return "", nil, err
	}

	return result, meta, nil
}

// tokenizeLine tokenizes a line into tokens, skipping string literals
func (p *ErrorPropASTProcessor) tokenizeLine(line string) []Token {
	var tokens []Token

	var s scanner.Scanner
	fset := token.NewFileSet()
	file := fset.AddFile("", fset.Base(), len(line))
	s.Init(file, []byte(line), nil, 0)

	for {
		pos, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}
		// Skip auto-inserted semicolons
		if tok == token.SEMICOLON {
			continue
		}

		tokens = append(tokens, Token{
			tok: tok,
			lit: lit,
			pos: fset.Position(pos).Offset,
		})
	}

	return tokens
}

// findErrorPropOperator finds the index of ? token that's an error propagation operator
// Returns -1 if no error propagation operator found
func (p *ErrorPropASTProcessor) findErrorPropOperator(tokens []Token) int {
	for i, tok := range tokens {
		if tok.tok != token.ILLEGAL || tok.lit != "?" {
			continue
		}

		// Found a ? - check if it's a ternary
		if p.isTernary(tokens, i) {
			continue
		}

		// This is an error propagation operator
		return i
	}

	return -1
}

// isTernary checks if ? at qIdx is part of a ternary expression
// A ternary has the pattern: expr ? value : value
func (p *ErrorPropASTProcessor) isTernary(tokens []Token, qIdx int) bool {
	// Look for : after ? (at same depth)
	depth := 0
	for i := qIdx + 1; i < len(tokens); i++ {
		tok := tokens[i]

		// Track depth for nested parens/brackets
		if tok.tok == token.LPAREN || tok.tok == token.LBRACK || tok.tok == token.LBRACE {
			depth++
		} else if tok.tok == token.RPAREN || tok.tok == token.RBRACK || tok.tok == token.RBRACE {
			depth--
		}

		// Found : at depth 0 - this is a ternary
		if tok.tok == token.COLON && depth == 0 {
			return true
		}

		// Stop at certain tokens that indicate end of expression
		if depth == 0 && (tok.tok == token.SEMICOLON || tok.tok == token.COMMA) {
			break
		}
	}

	return false
}

// extractExpression extracts the expression before ? operator
func (p *ErrorPropASTProcessor) extractExpression(tokens []Token, qIdx int) string {
	if qIdx == 0 {
		return ""
	}

	// Find the start of the expression by walking backwards
	// We need to handle balanced parens/brackets
	start := 0
	depth := 0

	// Start from qIdx-1 and walk backwards
	for i := qIdx - 1; i >= 0; i-- {
		tok := tokens[i]

		// Track depth (backwards, so closing increases depth)
		if tok.tok == token.RPAREN || tok.tok == token.RBRACK {
			depth++
		} else if tok.tok == token.LPAREN || tok.tok == token.LBRACK {
			depth--
		}

		// If we're at depth 0 and hit certain tokens, expression starts after them
		if depth == 0 {
			if tok.tok == token.ASSIGN || tok.tok == token.DEFINE {
				start = i + 1
				break
			}
			if tok.tok == token.RETURN {
				start = i + 1
				break
			}
		}
	}

	// Extract expression tokens with smart whitespace
	var buf bytes.Buffer
	for i := start; i < qIdx; i++ {
		// Get token text
		var tokText string
		if tokens[i].lit != "" {
			tokText = tokens[i].lit
		} else {
			tokText = tokens[i].tok.String()
		}

		// Add token
		buf.WriteString(tokText)

		// Add space if needed for next token
		if i+1 < qIdx {
			nextTok := tokens[i+1]
			if needsSpaceBetween(tokens[i].tok, nextTok.tok) {
				buf.WriteByte(' ')
			}
		}
	}

	expr := buf.String()
	expr = normalizeSpaces(expr)
	return strings.TrimSpace(expr)
}

// extractErrorMessage extracts custom error message after ? operator
// Returns empty string if no message found
func (p *ErrorPropASTProcessor) extractErrorMessage(tokens []Token, qIdx int) string {
	// Look for string literal after ?
	if qIdx+1 >= len(tokens) {
		return ""
	}

	nextTok := tokens[qIdx+1]
	if nextTok.tok == token.STRING {
		// Remove quotes
		msg := nextTok.lit
		if len(msg) >= 2 && msg[0] == '"' && msg[len(msg)-1] == '"' {
			msg = msg[1 : len(msg)-1]
		}
		return msg
	}

	return ""
}

// isReturnStatement checks if tokens represent a return statement
func (p *ErrorPropASTProcessor) isReturnStatement(tokens []Token) bool {
	for _, tok := range tokens {
		if tok.tok == token.RETURN {
			return true
		}
	}
	return false
}

// extractVarName extracts variable name from assignment tokens
func (p *ErrorPropASTProcessor) extractVarName(tokens []Token) string {
	// Look for pattern: [let/var] IDENT = expr?
	for i, tok := range tokens {
		if tok.tok == token.ASSIGN || tok.tok == token.DEFINE {
			// Variable name is the token before = or :=
			if i > 0 {
				prevTok := tokens[i-1]
				if prevTok.tok == token.IDENT {
					return prevTok.lit
				}
			}
		}
	}
	return "result"
}

// expandAssignment expands: let x = expr? → full error handling with unique marker
func (p *ErrorPropASTProcessor) expandAssignment(line, varName, expr, errMsg, marker string) (string, error) {
	// Track function call for import detection
	p.trackFunctionCallInExpr(expr)

	// No-number-first pattern: first occurrence has no number
	tmpVar := ""
	if p.tryCounter == 1 {
		tmpVar = "tmp"
	} else {
		tmpVar = fmt.Sprintf("tmp%d", p.tryCounter-1)
	}

	errVar := ""
	if p.tryCounter == 1 {
		errVar = "err"
	} else {
		errVar = fmt.Sprintf("err%d", p.tryCounter-1)
	}
	p.tryCounter++

	// Generate the expansion
	var buf bytes.Buffer
	indent := p.getIndent(line)

	// Line 1: tmpN, errN := expr
	buf.WriteString(indent)
	buf.WriteString(fmt.Sprintf("%s, %s := %s\n", tmpVar, errVar, expr))

	// Line 2: // dingo:e:N (UNIQUE MARKER)
	buf.WriteString(indent)
	buf.WriteString(marker)
	buf.WriteString("\n")

	// Line 3: if errN != nil {
	buf.WriteString(indent)
	buf.WriteString(fmt.Sprintf("if %s != nil {\n", errVar))

	// Line 4: return zeroValues, wrapped_error
	buf.WriteString(indent)
	buf.WriteString("\t")
	buf.WriteString(p.generateReturnStatement(errVar, errMsg))
	buf.WriteString("\n")

	// Line 5: }
	buf.WriteString(indent)
	buf.WriteString("}\n")

	// Line 6: var varName = tmpN
	buf.WriteString(indent)
	buf.WriteString(fmt.Sprintf("var %s = %s", varName, tmpVar))

	return buf.String(), nil
}

// expandReturn expands: return expr? → full error handling with unique marker
func (p *ErrorPropASTProcessor) expandReturn(line, expr, errMsg, marker string) (string, error) {
	// Track function call for import detection
	p.trackFunctionCallInExpr(expr)

	// Generate correct number of temporary variables for multi-value returns
	numNonErrorReturns := 1
	if p.currentFunc != nil && len(p.currentFunc.returnTypes) > 1 {
		numNonErrorReturns = len(p.currentFunc.returnTypes) - 1

		// Check config mode
		if p.config != nil && p.config.MultiValueReturnMode == "single" && numNonErrorReturns > 1 {
			return "", fmt.Errorf(
				"multi-value error propagation not allowed in 'single' mode (use --multi-value-return=full): function returns %d values plus error",
				numNonErrorReturns,
			)
		}
	}

	// Generate temporary variable names
	baseCounter := p.tryCounter
	tmpVars := []string{}
	for i := 0; i < numNonErrorReturns; i++ {
		var tmpVar string
		if baseCounter == 1 {
			tmpVar = "tmp"
		} else {
			tmpVar = fmt.Sprintf("tmp%d", baseCounter-1)
		}
		tmpVars = append(tmpVars, tmpVar)
		baseCounter++
	}

	var errVar string
	if p.tryCounter == 1 {
		errVar = "err"
	} else {
		errVar = fmt.Sprintf("err%d", p.tryCounter-1)
	}
	p.tryCounter++

	// Generate the expansion
	var buf bytes.Buffer
	indent := p.getIndent(line)

	// Line 1: tmp1, tmp2, ..., errN := expr
	buf.WriteString(indent)
	allVars := append(tmpVars, errVar)
	buf.WriteString(fmt.Sprintf("%s := %s\n", strings.Join(allVars, ", "), expr))

	// Line 2: // dingo:e:N (UNIQUE MARKER)
	buf.WriteString(indent)
	buf.WriteString(marker)
	buf.WriteString("\n")

	// Line 3: if errN != nil {
	buf.WriteString(indent)
	buf.WriteString(fmt.Sprintf("if %s != nil {\n", errVar))

	// Line 4: return zeroValues, wrapped_error
	buf.WriteString(indent)
	buf.WriteString("\t")
	buf.WriteString(p.generateReturnStatement(errVar, errMsg))
	buf.WriteString("\n")

	// Line 5: }
	buf.WriteString(indent)
	buf.WriteString("}\n")

	// Line 6: return tmp1, tmp2, ..., nil
	buf.WriteString(indent)
	returnVals := append([]string{}, tmpVars...)
	if p.currentFunc != nil && len(p.currentFunc.returnTypes) > 1 {
		returnVals = append(returnVals, "nil")
	}
	buf.WriteString(fmt.Sprintf("return %s", strings.Join(returnVals, ", ")))

	return buf.String(), nil
}

// generateReturnStatement generates the return statement with proper zero values
func (p *ErrorPropASTProcessor) generateReturnStatement(errVar string, errMsg string) string {
	// Get zero values for return types
	var zeroVals []string
	if p.currentFunc != nil && len(p.currentFunc.zeroValues) > 0 {
		zeroVals = p.currentFunc.zeroValues
	} else {
		// Fallback: assume one return value (nil)
		zeroVals = []string{"nil"}
	}

	// Generate error part
	var errPart string
	if errMsg != "" {
		// Escape % characters to prevent fmt.Errorf runtime panics
		escapedMsg := strings.ReplaceAll(errMsg, "%", "%%")

		// Wrap with fmt.Errorf
		p.needsFmt = true
		errPart = fmt.Sprintf(`fmt.Errorf("%s: %%w", %s)`, escapedMsg, errVar)
	} else {
		// Pass through as-is
		errPart = errVar
	}

	// Combine: return zeroVal1, zeroVal2, ..., error
	if len(zeroVals) > 0 {
		// Function returns (T, error) or (T1, T2, ..., error)
		return fmt.Sprintf("return %s, %s", strings.Join(zeroVals, ", "), errPart)
	} else {
		// Function returns only error
		return fmt.Sprintf("return %s", errPart)
	}
}

// getIndent extracts leading whitespace from a line
func (p *ErrorPropASTProcessor) getIndent(line string) string {
	for i, ch := range line {
		if ch != ' ' && ch != '\t' {
			return line[:i]
		}
	}
	return ""
}

// isFunctionDeclaration checks if a line is a function declaration
func (p *ErrorPropASTProcessor) isFunctionDeclaration(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, "func ")
}

// parseFunctionSignature parses a function signature to extract return types
func (p *ErrorPropASTProcessor) parseFunctionSignature(startLine int) *funcContext {
	// Collect lines until we find the opening brace
	var funcText strings.Builder
	foundBrace := false
	maxLines := startLine + 20
	if maxLines > len(p.lines) {
		maxLines = len(p.lines)
	}

	for i := startLine; i < maxLines; i++ {
		funcText.WriteString(p.lines[i])
		funcText.WriteString("\n")

		trimmed := strings.TrimSpace(p.lines[i])
		// Skip comment lines
		if strings.HasPrefix(trimmed, "//") {
			continue
		}

		if idx := strings.Index(trimmed, "{"); idx != -1 {
			foundBrace = true
			break
		}
	}

	if !foundBrace {
		// No brace found - return safe fallback
		return &funcContext{
			returnTypes: []string{},
			zeroValues:  []string{"nil"},
		}
	}

	// Parse as Go function
	fset := token.NewFileSet()
	src := fmt.Sprintf("package p\n%s}", funcText.String())
	file, err := parser.ParseFile(fset, "", src, 0)
	if err != nil {
		// Failed to parse, use nil fallback
		return &funcContext{
			returnTypes: []string{},
			zeroValues:  []string{"nil"},
		}
	}

	// Extract function declaration
	if len(file.Decls) == 0 {
		return &funcContext{
			returnTypes: []string{},
			zeroValues:  []string{"nil"},
		}
	}

	funcDecl, ok := file.Decls[0].(*ast.FuncDecl)
	if !ok || funcDecl.Type.Results == nil {
		return &funcContext{
			returnTypes: []string{},
			zeroValues:  []string{"nil"},
		}
	}

	// Extract return types
	returnTypes := []string{}
	for _, field := range funcDecl.Type.Results.List {
		typeStr := types.ExprString(field.Type)
		// If field has multiple names, repeat type (rare for returns)
		count := len(field.Names)
		if count == 0 {
			count = 1
		}
		for i := 0; i < count; i++ {
			returnTypes = append(returnTypes, typeStr)
		}
	}

	// Generate zero values (all except last, which is error)
	zeroValues := []string{}
	for i := 0; i < len(returnTypes)-1; i++ {
		zeroValues = append(zeroValues, getZeroValue(returnTypes[i]))
	}

	return &funcContext{
		returnTypes: returnTypes,
		zeroValues:  zeroValues,
	}
}

// needsSpaceBetween returns true if a space is needed between two tokens
func needsSpaceBetween(t1, t2 token.Token) bool {
	// No space before/after parens, brackets, periods, commas
	if t2 == token.LPAREN || t2 == token.LBRACK || t2 == token.COMMA {
		return false
	}
	if t1 == token.RPAREN || t1 == token.RBRACK {
		return false
	}
	if t1 == token.PERIOD || t2 == token.PERIOD {
		return false
	}

	// Space between identifiers and keywords
	if (t1 == token.IDENT || isKeyword(t1)) && (t2 == token.IDENT || isKeyword(t2)) {
		return true
	}

	return false
}

// isKeyword returns true if token is a keyword
func isKeyword(tok token.Token) bool {
	return tok == token.RETURN || tok == token.IF || tok == token.FOR ||
		tok == token.FUNC || tok == token.VAR || tok == token.CONST ||
		tok == token.TYPE || tok == token.STRUCT || tok == token.INTERFACE
}

// trackFunctionCallInExpr extracts function name from expression and tracks it
func (p *ErrorPropASTProcessor) trackFunctionCallInExpr(expr string) {
	// Simple extraction: find identifier before '('
	parenIdx := strings.Index(expr, "(")
	if parenIdx == -1 {
		return
	}

	// Get the part before '(' and remove all spaces
	beforeParen := strings.TrimSpace(expr[:parenIdx])
	beforeParen = strings.ReplaceAll(beforeParen, " ", "")

	// Split by '.' to handle qualified names (pkg.Func or obj.Method)
	parts := strings.Split(beforeParen, ".")

	// Track qualified calls (pkg.Function pattern)
	if len(parts) >= 2 {
		// Qualified call: construct "pkg.Function" pattern
		qualifiedName := strings.Join(parts[len(parts)-2:], ".")
		if p.importTracker != nil {
			p.importTracker.TrackFunctionCall(qualifiedName)
		}
	}
}
