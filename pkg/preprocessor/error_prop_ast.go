package preprocessor

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/scanner"
	"go/token"
	"go/types"
	"regexp"
	"strings"

	"github.com/MadAppGang/dingo/pkg/plugin/builtin"
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

// ProcessBody implements BodyProcessor interface for lambda body processing
func (p *ErrorPropASTProcessor) ProcessBody(body []byte) ([]byte, error) {
	result, _, err := p.Process(body)
	return result, err
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
	consumedLines := make(map[int]bool) // Track lines consumed by multiline processing

	for inputLineNum < len(p.lines) {
		// Skip if this line was already consumed by a previous multiline expression
		if consumedLines[inputLineNum] {
			inputLineNum++
			continue
		}

		line := p.lines[inputLineNum]

		// Check if this is a function declaration
		if p.isFunctionDeclaration(line) {
			p.currentFunc = p.parseFunctionSignature(inputLineNum)
			p.tryCounter = 1 // Reset counter for each function
		}

		// Process the line with metadata collection
		transformed, meta, consumedLineNums, err := p.processLineWithMetadata(line, inputLineNum+1, outputLineNum, &markerCounter)
		if err != nil {
			return "", nil, fmt.Errorf("line %d: %w", inputLineNum+1, err)
		}

		// Mark consumed lines
		for _, lineNum := range consumedLineNums {
			consumedLines[lineNum] = true
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
// Returns: (transformed_text, metadata, consumed_line_numbers, error)
func (p *ErrorPropASTProcessor) processLineWithMetadata(line string, originalLineNum int, outputLineNum int, markerCounter *int) (string, *TransformMetadata, []int, error) {
	// Quick check: does line contain ?
	if !strings.Contains(line, "?") {
		return line, nil, nil, nil
	}

	// Skip if line contains ?? null coalesce operator
	if strings.Contains(line, "??") {
		return line, nil, nil, nil
	}

	// Tokenize the line
	tokens := p.tokenizeLine(line)
	if len(tokens) == 0 {
		return line, nil, nil, nil
	}

	// Find error propagation operator (? that's NOT ternary)
	qIdx := p.findErrorPropOperator(tokens)
	if qIdx == -1 {
		// No error propagation operator found
		return line, nil, nil, nil
	}

	// Extract expression before ? and error expression after ?
	expr := p.extractExpression(tokens, qIdx)
	if expr == "" {
		return line, nil, nil, nil
	}

	// Extract after-? tokens to string
	var afterQBuf bytes.Buffer
	for i := qIdx + 1; i < len(tokens); i++ {
		tok := tokens[i]
		if tok.lit != "" {
			afterQBuf.WriteString(tok.lit)
		} else {
			afterQBuf.WriteString(tok.tok.String())
		}
		if i+1 < len(tokens) && needsSpaceBetween(tok.tok, tokens[i+1].tok) {
			afterQBuf.WriteByte(' ')
		}
	}
	afterQ := strings.TrimSpace(afterQBuf.String()) // Trim whitespace

	// Extract error expression (handles multiline)
	// Pass afterQ directly (already correctly parsed from tokens, handles string literals)
	// originalLineNum is 1-based, need to convert to 0-based for array access
	errExpr := p.extractErrorExprFromAfterQ(afterQ, originalLineNum-1)

	// Track consumed lines (if error expr was on next line)
	var consumedLines []int
	// originalLineNum is 1-based, so next line is at index originalLineNum (0-based)
	nextLineIdx := originalLineNum // This is the 0-based index of next line
	if afterQ == "" && nextLineIdx < len(p.lines) {
		// Next line might contain the error expression
		nextLine := strings.TrimSpace(p.lines[nextLineIdx])

		// Check if error expression was extracted from next line
		// (non-empty struct literal, method call, or function call)
		if errExpr.ExprType == ErrorExprStructLit && errExpr.StructType != "" {
			// Next line was consumed as struct literal
			consumedLines = append(consumedLines, nextLineIdx)
		} else if errExpr.ExprType == ErrorExprMethodCall && errExpr.ReceiverType != "" {
			// Next line was consumed as method call
			consumedLines = append(consumedLines, nextLineIdx)
		} else if errExpr.ExprType == ErrorExprFuncCall && errExpr.FuncName != "" {
			// Next line was consumed as function call
			consumedLines = append(consumedLines, nextLineIdx)
		} else if errExpr.ExprType == ErrorExprString && errExpr.Message != "" && nextLine != "" {
			// Next line was consumed as string message
			consumedLines = append(consumedLines, nextLineIdx)
		}
	}

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
		result, err = p.expandReturnV2(line, expr, errExpr, marker)
	} else {
		// Extract variable name for assignment
		varName := p.extractVarName(tokens)
		result, err = p.expandAssignmentV2(line, varName, expr, "", errExpr, marker)
	}

	if err != nil {
		return "", nil, consumedLines, err
	}

	return result, meta, consumedLines, nil
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

// expandAssignmentV2 expands assignment with ErrorExpr support
func (p *ErrorPropASTProcessor) expandAssignmentV2(line, varName, expr, errMsg string, errExpr ErrorExpr, marker string) (string, error) {
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

	// Line 4: return statement (V2 with ErrorExpr)
	buf.WriteString(indent)
	buf.WriteString("\t")
	buf.WriteString(p.generateReturnStatementV2(errVar, errExpr))
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

// expandReturnV2 expands return with ErrorExpr support
func (p *ErrorPropASTProcessor) expandReturnV2(line, expr string, errExpr ErrorExpr, marker string) (string, error) {
	// Track function call for import detection
	p.trackFunctionCallInExpr(expr)

	// For Result types, we don't need multiple temp variables
	if p.currentFunc != nil && p.currentFunc.isResultType {
		// Simple case: single temp variable
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

		// Line 1: tmp, err := expr (expr returns (T, error))
		buf.WriteString(indent)
		buf.WriteString(fmt.Sprintf("%s, %s := %s\n", tmpVar, errVar, expr))

		// Line 2: // dingo:e:N (UNIQUE MARKER)
		buf.WriteString(indent)
		buf.WriteString(marker)
		buf.WriteString("\n")

		// Line 3: if err != nil {
		buf.WriteString(indent)
		buf.WriteString(fmt.Sprintf("if %s != nil {\n", errVar))

		// Line 4: return ResultTEErr(customError)
		buf.WriteString(indent)
		buf.WriteString("\t")
		buf.WriteString(p.generateReturnStatementV2(errVar, errExpr))
		buf.WriteString("\n")

		// Line 5: }
		buf.WriteString(indent)
		buf.WriteString("}\n")

		// Line 6: return ResultTEOk(tmp)
		buf.WriteString(indent)
		buf.WriteString(fmt.Sprintf("return %sOk(%s)", p.currentFunc.resultTypeName, tmpVar))

		return buf.String(), nil
	}

	// Legacy behavior for (T, error) returns
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

	// Line 4: return statement (V2 with ErrorExpr)
	buf.WriteString(indent)
	buf.WriteString("\t")
	buf.WriteString(p.generateReturnStatementV2(errVar, errExpr))
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
	if p.currentFunc != nil {
		// Use parsed zero values (may be empty for functions without explicit return types)
		zeroVals = p.currentFunc.zeroValues

		// CRITICAL FIX: If function has no explicit return type but uses error propagation,
		// infer that it should return (T, error) where T defaults to nil
		if len(p.currentFunc.returnTypes) == 0 && len(zeroVals) == 0 {
			// No return types declared - infer (nil, error) for error propagation
			zeroVals = []string{"nil"}
		}
	} else {
		// Fallback: assume one return value (nil) when function context unknown
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
		// No brace found - return empty context (will use fallback in generateReturnStatement)
		return &funcContext{
			returnTypes: []string{},
			zeroValues:  []string{},
		}
	}

	// Parse as Go function
	fset := token.NewFileSet()
	src := fmt.Sprintf("package p\n%s}", funcText.String())
	file, err := parser.ParseFile(fset, "", src, 0)
	if err != nil {
		// Failed to parse - return empty context (will use fallback in generateReturnStatement)
		return &funcContext{
			returnTypes: []string{},
			zeroValues:  []string{},
		}
	}

	// Extract function declaration
	if len(file.Decls) == 0 {
		return &funcContext{
			returnTypes: []string{},
			zeroValues:  []string{},
		}
	}

	funcDecl, ok := file.Decls[0].(*ast.FuncDecl)
	if !ok || funcDecl.Type.Results == nil {
		// No return types or not a function - return empty context
		return &funcContext{
			returnTypes: []string{},
			zeroValues:  []string{},
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

	// Check if function returns Result<T, E> (Phase 11)
	ctx := &funcContext{
		returnTypes: returnTypes,
		zeroValues:  []string{},
	}

	if len(returnTypes) == 1 {
		resultInfo := p.parseResultType(returnTypes[0])
		if resultInfo != nil {
			// This is a Result type!
			ctx.isResultType = true
			ctx.resultOkType = resultInfo.okType
			ctx.resultErrType = resultInfo.errType
			ctx.resultTypeName = resultInfo.typeName
			// No zero values needed for Result types
			return ctx
		}
	}

	// Generate zero values (all except last, which is error)
	for i := 0; i < len(returnTypes)-1; i++ {
		ctx.zeroValues = append(ctx.zeroValues, getZeroValue(returnTypes[i]))
	}

	return ctx
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

// parseResultType attempts to parse a Result<T, E> type string
// Returns nil if not a Result type
func (p *ErrorPropASTProcessor) parseResultType(typeStr string) *struct {
	okType   string
	errType  string
	typeName string
} {
	// Match Result<T, E> or Result[T, E]
	// Pattern: Result followed by < or [, then type params
	resultPattern := regexp.MustCompile(`^Result[<\[]([^,>]+),\s*([^>\]]+)[>\]]$`)
	matches := resultPattern.FindStringSubmatch(typeStr)
	if matches == nil {
		return nil
	}

	okType := strings.TrimSpace(matches[1])
	errType := strings.TrimSpace(matches[2])

	// Use authoritative sanitization from Result type plugin
	typeName := "Result" + builtin.SanitizeTypeName(okType, errType)

	return &struct {
		okType   string
		errType  string
		typeName string
	}{
		okType:   okType,
		errType:  errType,
		typeName: typeName,
	}
}

// extractErrorExprFromAfterQ extracts the error expression from afterQ content
// afterQ is the content after the ? operator (already correctly parsed from tokens)
// lineNum is the 0-based index of the current line (with ?)
//
// Supports:
// 1. Struct literal: Type{field: value}
// 2. Method call: Type.Method(args)
// 3. Function call: Func(args)
// 4. String message: "message" (legacy)
// 5. Multiline: if afterQ is empty, checks next line
func (p *ErrorPropASTProcessor) extractErrorExprFromAfterQ(afterQ string, lineNum int) ErrorExpr {
	afterQ = strings.TrimSpace(afterQ)

	// If nothing after ?, check next line for error expression
	nextLineIdx := lineNum + 1
	if afterQ == "" && nextLineIdx < len(p.lines) {
		nextLine := strings.TrimSpace(p.lines[nextLineIdx])
		afterQ = nextLine
	}

	// If still nothing, return bare expression (no custom error)
	if afterQ == "" {
		return ErrorExpr{ExprType: ErrorExprString, Message: ""}
	}

	// Pattern 1: String message - "message"
	if strings.HasPrefix(afterQ, "\"") {
		msg := ""
		msgPattern := regexp.MustCompile(`^"((?:[^"\\]|\\.)*)"`)
		if matches := msgPattern.FindStringSubmatch(afterQ); matches != nil {
			msg = matches[1]
		}
		return ErrorExpr{
			ExprType: ErrorExprString,
			RawExpr:  afterQ,
			Message:  msg,
		}
	}

	// Pattern 2: Struct literal - Type{...}
	structLitPattern := regexp.MustCompile(`^([A-Z]\w*)\s*\{(.*)`)
	if matches := structLitPattern.FindStringSubmatch(afterQ); matches != nil {
		typeName := matches[1]
		fieldsStart := matches[2]

		// Extract full fields (handle nested braces)
		fields := p.extractBalancedBraces(fieldsStart)

		return ErrorExpr{
			ExprType:     ErrorExprStructLit,
			RawExpr:      afterQ,
			StructType:   typeName,
			StructFields: fields,
			InferredType: typeName,
		}
	}

	// Pattern 3: Method call - Type.Method(...)
	methodCallPattern := regexp.MustCompile(`^([A-Z]\w*)\.(\w+)\s*\((.*)`)
	if matches := methodCallPattern.FindStringSubmatch(afterQ); matches != nil {
		receiverType := matches[1]
		methodName := matches[2]
		argsStart := matches[3]

		// Extract full arguments (handle nested parens)
		args := p.extractBalancedParens(argsStart)

		return ErrorExpr{
			ExprType:     ErrorExprMethodCall,
			RawExpr:      afterQ,
			ReceiverType: receiverType,
			MethodName:   methodName,
			MethodArgs:   args,
			InferredType: receiverType,
		}
	}

	// Pattern 4: Function call - Func(...)
	funcCallPattern := regexp.MustCompile(`^([A-Z]\w*)\s*\((.*)`)
	if matches := funcCallPattern.FindStringSubmatch(afterQ); matches != nil {
		funcName := matches[1]
		argsStart := matches[2]

		// Extract full arguments (handle nested parens)
		args := p.extractBalancedParens(argsStart)

		return ErrorExpr{
			ExprType:     ErrorExprFuncCall,
			RawExpr:      afterQ,
			FuncName:     funcName,
			FuncArgs:     args,
			InferredType: "",
		}
	}

	// Fallback: treat as bare expression (no custom error)
	return ErrorExpr{ExprType: ErrorExprString, Message: ""}
}

// extractBalancedBraces extracts content until balanced closing brace
func (p *ErrorPropASTProcessor) extractBalancedBraces(text string) string {
	depth := 1 // We already consumed the opening brace
	var result strings.Builder

	for _, ch := range text {
		if ch == '{' {
			depth++
		} else if ch == '}' {
			depth--
			if depth == 0 {
				return result.String()
			}
		}
		result.WriteRune(ch)
	}

	// No matching closing brace found - return what we have
	return text
}

// extractBalancedParens extracts content until balanced closing paren
func (p *ErrorPropASTProcessor) extractBalancedParens(text string) string {
	depth := 1 // We already consumed the opening paren
	var result strings.Builder

	for _, ch := range text {
		if ch == '(' {
			depth++
		} else if ch == ')' {
			depth--
			if depth == 0 {
				return result.String()
			}
		}
		result.WriteRune(ch)
	}

	// No matching closing paren found - return what we have
	return text
}

// generateReturnStatementV2 generates the return statement for Result types with ErrorExpr
func (p *ErrorPropASTProcessor) generateReturnStatementV2(errVar string, errExpr ErrorExpr) string {
	// Check if function returns Result<T, E>
	if p.currentFunc != nil && p.currentFunc.isResultType {
		return p.generateResultReturnStatement(errVar, errExpr)
	}

	// Fallback to legacy behavior for (T, error) returns
	return p.generateReturnStatement(errVar, errExpr.Message)
}

// generateResultReturnStatement generates Result error constructor call
func (p *ErrorPropASTProcessor) generateResultReturnStatement(errVar string, errExpr ErrorExpr) string {
	typeName := p.currentFunc.resultTypeName

	switch errExpr.ExprType {
	case ErrorExprStructLit:
		// return ResultTEErr(Type{fields})
		return fmt.Sprintf("return %sErr(%s{%s})",
			typeName,
			errExpr.StructType,
			errExpr.StructFields)

	case ErrorExprMethodCall:
		// return ResultTEErr(Type.Method(args))
		return fmt.Sprintf("return %sErr(%s.%s(%s))",
			typeName,
			errExpr.ReceiverType,
			errExpr.MethodName,
			errExpr.MethodArgs)

	case ErrorExprFuncCall:
		// return ResultTEErr(Func(args))
		return fmt.Sprintf("return %sErr(%s(%s))",
			typeName,
			errExpr.FuncName,
			errExpr.FuncArgs)

	case ErrorExprString:
		// return ResultTEErr(fmt.Errorf("message: %w", err))
		if errExpr.Message != "" {
			// Check if error type is custom (not "error")
			if p.currentFunc != nil && p.currentFunc.resultErrType != "error" {
				// String messages not supported with custom error types
				return fmt.Sprintf("/* ERROR: String error messages not supported with custom error types (Result<%s, %s>) */\nreturn %sErr(nil)",
					p.currentFunc.resultOkType,
					p.currentFunc.resultErrType,
					typeName)
			}
			// Escape % characters
			escapedMsg := strings.ReplaceAll(errExpr.Message, "%", "%%")
			p.needsFmt = true
			return fmt.Sprintf("return %sErr(fmt.Errorf(\"%s: %%w\", %s))",
				typeName,
				escapedMsg,
				errVar)
		} else {
			// Bare error, just wrap it
			return fmt.Sprintf("return %sErr(%s)", typeName, errVar)
		}

	default:
		// Fallback: bare error
		return fmt.Sprintf("return %sErr(%s)", typeName, errVar)
	}
}
