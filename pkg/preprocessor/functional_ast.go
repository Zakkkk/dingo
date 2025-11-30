package preprocessor

// AST-based functional utilities processor
// Migrated from regex-based functional.go and functional_chain.go
// See: ai-docs/AST_MIGRATION.md

import (
	"bytes"
	"fmt"
	"go/scanner"
	"go/token"
	"strings"

	dingoast "github.com/MadAppGang/dingo/pkg/ast"
	"github.com/MadAppGang/dingo/pkg/plugin/builtin"
)

// FunctionalASTProcessor handles functional utility method transformations using AST-based parsing
// Replaces regex-based FunctionalProcessor with proper tokenization and parsing
//
// Supported Methods:
// - map(f): Transform each element
// - filter(f): Select elements matching predicate
// - reduce(init, f): Aggregate into single value
// - sum(): Sum all elements (numeric slices only)
// - count(): Count elements
// - all(f): Check if all elements match predicate
// - any(f): Check if any element matches predicate
// - find(f): Find first matching element → Option<T>
// - findIndex(f): Find index of first match → Option<int>
// - mapResult(f): Map with error propagation → Result<[]R, E>
// - filterMap(f): Filter and map combined (Option values)
// - partition(f): Split into two slices by predicate
//
// Chain Fusion:
// Combines chained operations into single loop for performance:
//   items.filter(f).map(g) → single loop with if + append
//   nums.map(f).map(g) → single loop with composed transform
type FunctionalASTProcessor struct {
	baseCounters  map[string]int      // Per-base-name counters for temp variables (no-number-first pattern)
	markerCounter int                 // Counter for unique transformation markers
	errors        []error             // Collected errors during processing
	metadata      []TransformMetadata // Transformation metadata for source maps
	fset          *token.FileSet      // File set for position tracking
}

// NewFunctionalASTProcessor creates a new AST-based functional utilities preprocessor
func NewFunctionalASTProcessor() *FunctionalASTProcessor {
	return &FunctionalASTProcessor{
		baseCounters:  make(map[string]int),
		markerCounter: 1,
		errors:        []error{},
		metadata:      []TransformMetadata{},
		fset:          token.NewFileSet(),
	}
}

// Name returns the processor name
func (f *FunctionalASTProcessor) Name() string {
	return "functional_ast"
}

// Process implements FeatureProcessor interface for backward compatibility
func (f *FunctionalASTProcessor) Process(source []byte) ([]byte, []Mapping, error) {
	result, err := f.ProcessV2(source)
	if err != nil {
		return nil, nil, err
	}
	return result.Source, result.Mappings, nil
}

// ProcessV2 implements FeatureProcessorV2 interface with metadata support
func (f *FunctionalASTProcessor) ProcessV2(source []byte) (ProcessResult, error) {
	transformed, metadata, err := f.ProcessInternal(string(source))
	if err != nil {
		return ProcessResult{}, err
	}

	return ProcessResult{
		Source:   []byte(transformed),
		Mappings: nil, // We use metadata instead of legacy mappings
		Metadata: metadata,
	}, nil
}

// ProcessInternal transforms functional method calls using AST-based parsing
// Emits TransformMetadata with unique markers for each transformation
func (f *FunctionalASTProcessor) ProcessInternal(code string) (string, []TransformMetadata, error) {
	// Reset state for this processing run
	f.baseCounters = make(map[string]int)
	f.markerCounter = 1
	f.errors = []error{}
	f.metadata = []TransformMetadata{}
	f.fset = token.NewFileSet()

	// Split into lines for processing
	lines := strings.Split(code, "\n")
	var output bytes.Buffer

	for lineNum, line := range lines {
		// Process the line
		transformed, err := f.processLine(line, lineNum+1)
		if err != nil {
			return "", nil, fmt.Errorf("line %d: %w", lineNum+1, err)
		}
		output.WriteString(transformed)
		if lineNum < len(lines)-1 {
			output.WriteByte('\n')
		}
	}

	// If we collected errors, return the first one
	if len(f.errors) > 0 {
		return "", nil, f.errors[0]
	}

	return output.String(), f.metadata, nil
}

// processLine processes a single line for functional method calls
// CRITICAL: Tries chain detection first, falls back to single operation
func (f *FunctionalASTProcessor) processLine(line string, lineNum int) (string, error) {
	// Quick check: does line contain . (method call)?
	if !strings.Contains(line, ".") {
		return line, nil
	}

	// TRY CHAIN DETECTION FIRST
	chain := f.detectChain(line, lineNum)
	if chain != nil {
		// Found a chain - fuse it into single loop
		iife, err := f.fuseChain(chain)
		if err != nil {
			return "", fmt.Errorf("chain fusion: %w", err)
		}

		// Replace chain with fused IIFE
		before := line[:chain.StartPos]
		after := ""
		if chain.EndPos < len(line) {
			after = line[chain.EndPos:]
		}

		transformed := before + iife + after

		// Emit metadata for chain
		marker := fmt.Sprintf("// dingo:f:%d", f.markerCounter)
		f.markerCounter++
		f.metadata = append(f.metadata, TransformMetadata{
			Type:            "functional_chain",
			OriginalLine:    lineNum,
			OriginalColumn:  chain.StartPos + 1,
			OriginalLength:  chain.EndPos - chain.StartPos,
			OriginalText:    line[chain.StartPos:chain.EndPos],
			GeneratedMarker: marker,
			ASTNodeType:     "FuncLit",
		})

		return transformed, nil
	}

	// NO CHAIN - fall back to single operation detection
	methodCall := f.detectMethodCall(line, lineNum)
	if methodCall == nil {
		// No functional method call found
		return line, nil
	}

	// Transform based on method type
	var iife string
	var err error

	switch methodCall.Method {
	case "map":
		iife, err = f.transformMap(methodCall)
	case "filter":
		iife, err = f.transformFilter(methodCall)
	case "reduce":
		iife, err = f.transformReduce(methodCall)
	case "sum":
		iife, err = f.transformSum(methodCall)
	case "count":
		iife, err = f.transformCount(methodCall)
	case "all":
		iife, err = f.transformAll(methodCall)
	case "any":
		iife, err = f.transformAny(methodCall)
	case "find":
		iife, err = f.transformFind(methodCall)
	case "findIndex":
		iife, err = f.transformFindIndex(methodCall)
	case "mapResult":
		iife, err = f.transformMapResult(methodCall)
	case "filterMap":
		iife, err = f.transformFilterMap(methodCall)
	case "partition":
		iife, err = f.transformPartition(methodCall)
	default:
		// Unknown method - leave as-is
		return line, nil
	}

	if err != nil {
		return "", err
	}

	// Replace method call with IIFE
	before := line[:methodCall.StartPos]
	after := ""
	if methodCall.EndPos < len(line) {
		after = line[methodCall.EndPos:]
	}

	transformed := before + iife + after

	// Emit metadata
	marker := fmt.Sprintf("// dingo:f:%d", f.markerCounter)
	f.markerCounter++
	f.metadata = append(f.metadata, TransformMetadata{
		Type:            "functional",
		OriginalLine:    lineNum,
		OriginalColumn:  methodCall.StartPos + 1,
		OriginalLength:  methodCall.EndPos - methodCall.StartPos,
		OriginalText:    line[methodCall.StartPos:methodCall.EndPos],
		GeneratedMarker: marker,
		ASTNodeType:     "FuncLit",
	})

	return transformed, nil
}

// detectMethodCall detects functional method calls using token-based parsing
// Returns nil if no method call found
func (f *FunctionalASTProcessor) detectMethodCall(line string, lineNum int) *dingoast.FunctionalCall {
	// Create scanner for the line
	file := f.fset.AddFile("", f.fset.Base(), len(line))
	var s scanner.Scanner
	s.Init(file, []byte(line), nil, 0)

	// Valid functional methods
	validMethods := map[string]bool{
		"map": true, "filter": true, "reduce": true,
		"sum": true, "count": true, "all": true, "any": true,
		"find": true, "findIndex": true, "mapResult": true,
		"filterMap": true, "partition": true,
	}

	// Scan for pattern: IDENT . IDENT (
	var receiver string
	var method string
	var receiverPos int
	state := 0 // 0=looking for receiver, 1=expect dot, 2=expect method, 3=expect lparen

	for {
		pos, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}

		switch state {
		case 0: // Looking for identifier (potential receiver)
			if tok == token.IDENT {
				receiver = lit
				receiverPos = f.fset.Position(pos).Offset
				state = 1
			}
		case 1: // Expect dot
			if tok == token.PERIOD {
				state = 2
			} else if tok == token.IDENT {
				// Could be chained, restart
				receiver = lit
				receiverPos = f.fset.Position(pos).Offset
			} else {
				state = 0
			}
		case 2: // Expect method name
			if tok == token.IDENT && validMethods[lit] {
				method = lit
				state = 3
			} else {
				state = 0
			}
		case 3: // Expect lparen
			if tok == token.LPAREN {
				// Found a match! Now extract arguments
				argsStart := f.fset.Position(pos).Offset + 1
				args, argsEnd := f.extractArguments(line, argsStart)

				return &dingoast.FunctionalCall{
					CallPos:  token.Pos(receiverPos),
					Receiver: receiver,
					Method:   method,
					Args:     args,
					StartPos: receiverPos,
					EndPos:   argsEnd,
				}
			}
			state = 0
		}
	}

	return nil
}

// extractArguments extracts function arguments using balanced parenthesis parsing
// Returns: (arguments, endPosition)
func (f *FunctionalASTProcessor) extractArguments(line string, startPos int) ([]string, int) {
	depth := 1 // We're already inside one level of parens
	var argBuf bytes.Buffer
	args := []string{}

	for i := startPos; i < len(line); i++ {
		ch := line[i]

		switch ch {
		case '(':
			depth++
			argBuf.WriteByte(ch)
		case ')':
			depth--
			if depth == 0 {
				// Found matching closing paren
				if argBuf.Len() > 0 {
					args = append(args, strings.TrimSpace(argBuf.String()))
				}
				return args, i + 1
			}
			argBuf.WriteByte(ch)
		case ',':
			if depth == 1 {
				// Top-level comma - argument separator
				if argBuf.Len() > 0 {
					args = append(args, strings.TrimSpace(argBuf.String()))
				}
				argBuf.Reset()
			} else {
				argBuf.WriteByte(ch)
			}
		default:
			argBuf.WriteByte(ch)
		}
	}

	// Reached end without finding closing paren
	if argBuf.Len() > 0 {
		args = append(args, strings.TrimSpace(argBuf.String()))
	}
	return args, len(line)
}

// getTempVar generates a temporary variable name following no-number-first pattern
// First: tmp, then tmp1, tmp2, etc.
func (f *FunctionalASTProcessor) getTempVar(base string) string {
	count, exists := f.baseCounters[base]
	if !exists {
		// First use of this base name - no number suffix
		f.baseCounters[base] = 1
		return base
	}
	// Subsequent uses - add number suffix starting from 1
	f.baseCounters[base] = count + 1
	return fmt.Sprintf("%s%d", base, count)
}

// parseFuncLiteral parses a Go function literal (already expanded by LambdaProcessor)
// Input: "func(x int) int { return x * 2 }" or "func(acc, x int) int { return acc + x }"
// Returns: dingoast.FuncLiteral with parsed structure
func (f *FunctionalASTProcessor) parseFuncLiteral(funcLit string) (*dingoast.FuncLiteral, error) {
	// Find parameter list
	openParen := strings.Index(funcLit, "(")
	if openParen == -1 {
		return nil, fmt.Errorf("invalid function literal: missing '('")
	}

	// Find matching close paren using depth counting
	depth := 1
	closeParen := openParen + 1
	for closeParen < len(funcLit) && depth > 0 {
		if funcLit[closeParen] == '(' {
			depth++
		} else if funcLit[closeParen] == ')' {
			depth--
		}
		closeParen++
	}

	if depth != 0 {
		return nil, fmt.Errorf("invalid function literal: unbalanced parentheses")
	}

	paramsStr := strings.TrimSpace(funcLit[openParen+1 : closeParen-1])

	// Find body using balanced brace counting
	openBrace := strings.Index(funcLit[closeParen:], "{")
	if openBrace == -1 {
		return nil, fmt.Errorf("invalid function literal: missing '{'")
	}

	// Extract return type (between ) and {)
	returnTypeStr := strings.TrimSpace(funcLit[closeParen : closeParen+openBrace])

	openBrace += closeParen

	depth = 1
	closeBrace := openBrace + 1
	for closeBrace < len(funcLit) && depth > 0 {
		if funcLit[closeBrace] == '{' {
			depth++
		} else if funcLit[closeBrace] == '}' {
			depth--
		}
		closeBrace++
	}

	if depth != 0 {
		return nil, fmt.Errorf("invalid function literal: unbalanced braces")
	}

	bodyContent := strings.TrimSpace(funcLit[openBrace+1 : closeBrace-1])

	// Check if body is a simple return expression
	isExpr := strings.HasPrefix(bodyContent, "return ") &&
		!strings.Contains(bodyContent, "\n") &&
		!strings.Contains(bodyContent, ";")

	// Extract expression from "return expr"
	body := bodyContent
	if isExpr {
		body = strings.TrimPrefix(bodyContent, "return ")
		body = strings.TrimSpace(body)
	}

	// Parse parameters
	params := f.parseParams(paramsStr)

	return &dingoast.FuncLiteral{
		Params:     params,
		ReturnType: returnTypeStr,
		Body:       body,
		IsExpr:     isExpr,
	}, nil
}

// parseParams parses parameter list from function literal
// Input: "x int" → [{Name: "x", Type: "int"}]
// Input: "acc int, x int" → [{Name: "acc", Type: "int"}, {Name: "x", Type: "int"}]
// Input: "acc, x" → [{Name: "acc", Type: ""}, {Name: "x", Type: ""}]
func (f *FunctionalASTProcessor) parseParams(paramsStr string) []dingoast.Param {
	if paramsStr == "" {
		return nil
	}

	var params []dingoast.Param

	// Split by comma for multiple params
	parts := strings.Split(paramsStr, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Split by whitespace to separate name from type
		tokens := strings.Fields(part)
		param := dingoast.Param{}
		if len(tokens) >= 1 {
			param.Name = tokens[0]
		}
		if len(tokens) >= 2 {
			param.Type = tokens[1]
		}
		params = append(params, param)
	}

	return params
}

// Transformation Methods (Single Operations)

// transformMap transforms: nums.map(func(x int) int { return x * 2 })
func (f *FunctionalASTProcessor) transformMap(mc *dingoast.FunctionalCall) (string, error) {
	if len(mc.Args) != 1 {
		return "", fmt.Errorf("map() requires exactly 1 argument (function), got %d", len(mc.Args))
	}

	lambda, err := f.parseFuncLiteral(mc.Args[0])
	if err != nil {
		return "", fmt.Errorf("map: %w", err)
	}

	// Get parameter name
	paramName := ""
	if len(lambda.Params) > 0 {
		paramName = lambda.Params[0].Name
	}

	// Get return type for slice (fall back to interface{} if not specified)
	elemType := lambda.ReturnType
	if elemType == "" {
		elemType = "interface{}"
	}

	tmpVar := f.getTempVar("tmp")

	iife := fmt.Sprintf(`func() []%s {
	%s := make([]%s, 0, len(%s))
	for _, %s := range %s {
		%s = append(%s, %s)
	}
	return %s
}()`, elemType, tmpVar, elemType, mc.Receiver, paramName, mc.Receiver, tmpVar, tmpVar, lambda.Body, tmpVar)

	return iife, nil
}

// transformFilter transforms: nums.filter(func(x int) bool { return x > 0 })
func (f *FunctionalASTProcessor) transformFilter(mc *dingoast.FunctionalCall) (string, error) {
	if len(mc.Args) != 1 {
		return "", fmt.Errorf("filter() requires exactly 1 argument (predicate function), got %d", len(mc.Args))
	}

	lambda, err := f.parseFuncLiteral(mc.Args[0])
	if err != nil {
		return "", fmt.Errorf("filter: %w", err)
	}

	// Get parameter name and type
	paramName := ""
	elemType := "interface{}"
	if len(lambda.Params) > 0 {
		paramName = lambda.Params[0].Name
		if lambda.Params[0].Type != "" {
			elemType = lambda.Params[0].Type
		}
	}

	tmpVar := f.getTempVar("tmp")

	iife := fmt.Sprintf(`func() []%s {
	%s := make([]%s, 0, len(%s))
	for _, %s := range %s {
		if %s {
			%s = append(%s, %s)
		}
	}
	return %s
}()`, elemType, tmpVar, elemType, mc.Receiver, paramName, mc.Receiver, lambda.Body, tmpVar, tmpVar, paramName, tmpVar)

	return iife, nil
}

// transformReduce transforms: nums.reduce(0, func(acc int, x int) int { return acc + x })
func (f *FunctionalASTProcessor) transformReduce(mc *dingoast.FunctionalCall) (string, error) {
	if len(mc.Args) != 2 {
		return "", fmt.Errorf("reduce() requires exactly 2 arguments (initialValue, function), got %d", len(mc.Args))
	}

	initValue := mc.Args[0]
	lambda, err := f.parseFuncLiteral(mc.Args[1])
	if err != nil {
		return "", fmt.Errorf("reduce: %w", err)
	}

	// Get param names: (acc, x)
	accName := "acc"
	elemName := "x"
	if len(lambda.Params) >= 2 {
		accName = lambda.Params[0].Name
		elemName = lambda.Params[1].Name
	}

	// Get return type (fall back to interface{} if not specified)
	resultType := lambda.ReturnType
	if resultType == "" {
		resultType = "interface{}"
	}

	iife := fmt.Sprintf(`func() %s {
	%s := %s
	for _, %s := range %s {
		%s = %s
	}
	return %s
}()`, resultType, accName, initValue, elemName, mc.Receiver, accName, lambda.Body, accName)

	return iife, nil
}

// transformSum transforms: nums.sum()
func (f *FunctionalASTProcessor) transformSum(mc *dingoast.FunctionalCall) (string, error) {
	if len(mc.Args) != 0 {
		return "", fmt.Errorf("sum() takes no arguments, got %d", len(mc.Args))
	}

	sumVar := f.getTempVar("sum")

	iife := fmt.Sprintf(`func() int {
	%s := 0
	for _, x := range %s {
		%s = %s + x
	}
	return %s
}()`, sumVar, mc.Receiver, sumVar, sumVar, sumVar)

	return iife, nil
}

// transformCount transforms: nums.count(func(x int) bool { return x > 0 })
func (f *FunctionalASTProcessor) transformCount(mc *dingoast.FunctionalCall) (string, error) {
	if len(mc.Args) != 1 {
		return "", fmt.Errorf("count() requires exactly 1 argument (predicate function), got %d", len(mc.Args))
	}

	lambda, err := f.parseFuncLiteral(mc.Args[0])
	if err != nil {
		return "", fmt.Errorf("count: %w", err)
	}

	paramName := ""
	if len(lambda.Params) > 0 {
		paramName = lambda.Params[0].Name
	}

	countVar := f.getTempVar("count")

	iife := fmt.Sprintf(`func() int {
	%s := 0
	for _, %s := range %s {
		if %s {
			%s++
		}
	}
	return %s
}()`, countVar, paramName, mc.Receiver, lambda.Body, countVar, countVar)

	return iife, nil
}

// transformAll transforms: nums.all(func(x int) bool { return x > 0 })
func (f *FunctionalASTProcessor) transformAll(mc *dingoast.FunctionalCall) (string, error) {
	if len(mc.Args) != 1 {
		return "", fmt.Errorf("all() requires exactly 1 argument (predicate function), got %d", len(mc.Args))
	}

	lambda, err := f.parseFuncLiteral(mc.Args[0])
	if err != nil {
		return "", fmt.Errorf("all: %w", err)
	}

	paramName := ""
	if len(lambda.Params) > 0 {
		paramName = lambda.Params[0].Name
	}

	iife := fmt.Sprintf(`func() bool {
	for _, %s := range %s {
		if !(%s) {
			return false
		}
	}
	return true
}()`, paramName, mc.Receiver, lambda.Body)

	return iife, nil
}

// transformAny transforms: nums.any(func(x int) bool { return x > 0 })
func (f *FunctionalASTProcessor) transformAny(mc *dingoast.FunctionalCall) (string, error) {
	if len(mc.Args) != 1 {
		return "", fmt.Errorf("any() requires exactly 1 argument (predicate function), got %d", len(mc.Args))
	}

	lambda, err := f.parseFuncLiteral(mc.Args[0])
	if err != nil {
		return "", fmt.Errorf("any: %w", err)
	}

	paramName := ""
	if len(lambda.Params) > 0 {
		paramName = lambda.Params[0].Name
	}

	iife := fmt.Sprintf(`func() bool {
	for _, %s := range %s {
		if %s {
			return true
		}
	}
	return false
}()`, paramName, mc.Receiver, lambda.Body)

	return iife, nil
}

// transformFind transforms: users.find(func(u User) bool { return u.id == targetId })
func (f *FunctionalASTProcessor) transformFind(mc *dingoast.FunctionalCall) (string, error) {
	if len(mc.Args) != 1 {
		return "", fmt.Errorf("find() requires exactly 1 argument (predicate function), got %d", len(mc.Args))
	}

	lambda, err := f.parseFuncLiteral(mc.Args[0])
	if err != nil {
		return "", fmt.Errorf("find: %w", err)
	}

	// Extract parameter name and type
	paramName := ""
	elemType := "interface{}"
	if len(lambda.Params) > 0 {
		paramName = lambda.Params[0].Name
		if lambda.Params[0].Type != "" {
			elemType = lambda.Params[0].Type
		}
	}

	// Generate camelCase Option type name using SanitizeTypeName
	optionType := "Option" + builtin.SanitizeTypeName(elemType)

	iife := fmt.Sprintf(`func() %s {
	for _, %s := range %s {
		if %s {
			return %sSome(%s)
		}
	}
	return %sNone()
}()`, optionType, paramName, mc.Receiver, lambda.Body, optionType, paramName, optionType)

	return iife, nil
}

// transformFindIndex transforms: items.findIndex(func(x Item) bool { return x.name == "target" })
func (f *FunctionalASTProcessor) transformFindIndex(mc *dingoast.FunctionalCall) (string, error) {
	if len(mc.Args) != 1 {
		return "", fmt.Errorf("findIndex() requires exactly 1 argument (predicate function), got %d", len(mc.Args))
	}

	lambda, err := f.parseFuncLiteral(mc.Args[0])
	if err != nil {
		return "", fmt.Errorf("findIndex: %w", err)
	}

	paramName := ""
	if len(lambda.Params) > 0 {
		paramName = lambda.Params[0].Name
	}

	optionType := "OptionInt"

	iife := fmt.Sprintf(`func() %s {
	for i, %s := range %s {
		if %s {
			return %sSome(i)
		}
	}
	return %sNone()
}()`, optionType, paramName, mc.Receiver, lambda.Body, optionType, optionType)

	return iife, nil
}

// transformMapResult transforms: strings.mapResult(func(s string) ResultIntError { return parseInt(s) })
func (f *FunctionalASTProcessor) transformMapResult(mc *dingoast.FunctionalCall) (string, error) {
	if len(mc.Args) != 1 {
		return "", fmt.Errorf("mapResult() requires exactly 1 argument (function returning Result), got %d", len(mc.Args))
	}

	lambda, err := f.parseFuncLiteral(mc.Args[0])
	if err != nil {
		return "", fmt.Errorf("mapResult: %w", err)
	}

	paramName := ""
	if len(lambda.Params) > 0 {
		paramName = lambda.Params[0].Name
	}

	tmpVar := f.getTempVar("tmp")
	resVar := f.getTempVar("res")

	// Use interface{} as placeholder - proper type inference would require analysis
	sliceType := "[]interface{}"
	errorType := "error"
	resultType := "Result" + builtin.SanitizeTypeName(sliceType, errorType)

	iife := fmt.Sprintf(`func() %s {
	%s := make([]interface{}, 0, len(%s))
	for _, %s := range %s {
		%s := %s
		if %s.IsErr() {
			return %sErr(%s.UnwrapErr())
		}
		%s = append(%s, %s.Unwrap())
	}
	return %sOk(%s)
}()`, resultType, tmpVar, mc.Receiver, paramName, mc.Receiver, resVar, lambda.Body, resVar, resultType, resVar, tmpVar, tmpVar, resVar, resultType, tmpVar)

	return iife, nil
}

// transformFilterMap transforms: items.filterMap(func(x Item) OptionInt { return x.maybeParse() })
func (f *FunctionalASTProcessor) transformFilterMap(mc *dingoast.FunctionalCall) (string, error) {
	if len(mc.Args) != 1 {
		return "", fmt.Errorf("filterMap() requires exactly 1 argument (function returning Option), got %d", len(mc.Args))
	}

	lambda, err := f.parseFuncLiteral(mc.Args[0])
	if err != nil {
		return "", fmt.Errorf("filterMap: %w", err)
	}

	paramName := ""
	if len(lambda.Params) > 0 {
		paramName = lambda.Params[0].Name
	}

	tmpVar := f.getTempVar("tmp")
	optVar := f.getTempVar("opt")

	iife := fmt.Sprintf(`func() []interface{} {
	%s := make([]interface{}, 0, len(%s))
	for _, %s := range %s {
		if %s := %s; %s.IsSome() {
			%s = append(%s, %s.Unwrap())
		}
	}
	return %s
}()`, tmpVar, mc.Receiver, paramName, mc.Receiver, optVar, lambda.Body, optVar, tmpVar, tmpVar, optVar, tmpVar)

	return iife, nil
}

// transformPartition transforms: users.partition(func(u User) bool { return u.active })
func (f *FunctionalASTProcessor) transformPartition(mc *dingoast.FunctionalCall) (string, error) {
	if len(mc.Args) != 1 {
		return "", fmt.Errorf("partition() requires exactly 1 argument (predicate function), got %d", len(mc.Args))
	}

	lambda, err := f.parseFuncLiteral(mc.Args[0])
	if err != nil {
		return "", fmt.Errorf("partition: %w", err)
	}

	paramName := ""
	if len(lambda.Params) > 0 {
		paramName = lambda.Params[0].Name
	}

	trueVar := f.getTempVar("trueSlice")
	falseVar := f.getTempVar("falseSlice")

	iife := fmt.Sprintf(`func() ([]interface{}, []interface{}) {
	%s := make([]interface{}, 0, len(%s))
	%s := make([]interface{}, 0, len(%s))
	for _, %s := range %s {
		if %s {
			%s = append(%s, %s)
		} else {
			%s = append(%s, %s)
		}
	}
	return %s, %s
}()`, trueVar, mc.Receiver, falseVar, mc.Receiver, paramName, mc.Receiver, lambda.Body, trueVar, trueVar, paramName, falseVar, falseVar, paramName, trueVar, falseVar)

	return iife, nil
}

// Chain Detection and Fusion (from functional_chain.go)

// detectChain detects and parses a chain of method calls using token-based parsing
// Returns nil if no chain found, or a ChainExpr if detected
func (f *FunctionalASTProcessor) detectChain(line string, lineNum int) *dingoast.ChainExpr {
	// Try to find first method call
	firstCall := f.detectMethodCall(line, lineNum)
	if firstCall == nil {
		return nil
	}

	// Build chain starting from first call
	chain := &dingoast.ChainExpr{
		ChainPos:   firstCall.CallPos,
		Receiver:   firstCall.Receiver,
		Operations: []dingoast.FunctionalCall{*firstCall},
		CanFuse:    true,
		StartPos:   firstCall.StartPos,
		EndPos:     firstCall.EndPos,
	}

	// Check if there's a continuation (. immediately after closing paren)
	pos := firstCall.EndPos
	for pos < len(line) {
		// Skip whitespace
		for pos < len(line) && (line[pos] == ' ' || line[pos] == '\t') {
			pos++
		}

		// Check for . (chain continuation)
		if pos >= len(line) || line[pos] != '.' {
			break
		}
		pos++ // Skip the '.'

		// Try to detect next method call from this position
		restOfLine := line[pos:]
		nextCall := f.detectNextChainedCall(restOfLine, pos, lineNum)
		if nextCall == nil {
			break
		}

		chain.Operations = append(chain.Operations, *nextCall)
		chain.EndPos = nextCall.EndPos
		pos = nextCall.EndPos
	}

	// Only return chain if we have 2+ operations
	if len(chain.Operations) < 2 {
		return nil
	}

	return chain
}

// detectNextChainedCall detects a method call at the start of a string
func (f *FunctionalASTProcessor) detectNextChainedCall(substring string, offset int, lineNum int) *dingoast.FunctionalCall {
	validMethods := map[string]bool{
		"map": true, "filter": true, "reduce": true,
		"sum": true, "count": true, "all": true, "any": true,
		"find": true, "findIndex": true, "mapResult": true,
		"filterMap": true, "partition": true,
	}

	// Parse method name
	methodStart := 0
	for methodStart < len(substring) && (substring[methodStart] == ' ' || substring[methodStart] == '\t') {
		methodStart++
	}

	methodEnd := methodStart
	for methodEnd < len(substring) {
		ch := substring[methodEnd]
		if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_') {
			break
		}
		methodEnd++
	}

	if methodEnd == methodStart {
		return nil
	}

	methodName := substring[methodStart:methodEnd]

	if !validMethods[methodName] {
		return nil
	}

	// Find opening paren
	parenPos := methodEnd
	for parenPos < len(substring) && (substring[parenPos] == ' ' || substring[parenPos] == '\t') {
		parenPos++
	}

	if parenPos >= len(substring) || substring[parenPos] != '(' {
		return nil
	}
	parenPos++ // Skip '('

	// Extract arguments
	args, endPos := f.extractArguments(substring, parenPos)

	return &dingoast.FunctionalCall{
		CallPos:  token.Pos(offset + methodStart),
		Method:   methodName,
		Args:     args,
		StartPos: offset,
		EndPos:   offset + endPos,
	}
}

// fuseChain generates fused IIFE code for a chain of operations
func (f *FunctionalASTProcessor) fuseChain(chain *dingoast.ChainExpr) (string, error) {
	// Validate chain
	if chain == nil || len(chain.Operations) == 0 {
		return "", fmt.Errorf("cannot fuse empty chain")
	}

	// Determine fusion strategy based on operation sequence
	lastOp := chain.Operations[len(chain.Operations)-1]
	isTerminal := lastOp.Method == "reduce" || lastOp.Method == "all" || lastOp.Method == "any"

	if isTerminal {
		return f.fuseToTerminal(chain)
	}

	return f.fuseToSlice(chain)
}

// fuseToSlice fuses a chain that produces a slice (filter/map combinations)
func (f *FunctionalASTProcessor) fuseToSlice(chain *dingoast.ChainExpr) (string, error) {
	tmpVar := f.getTempVar("tmp")
	loopVar := "x"

	// Parse all operations to extract lambdas
	type ParsedOp struct {
		Method     string
		Param      string
		ParamType  string
		ReturnType string
		Body       string
	}

	var ops []ParsedOp
	for _, op := range chain.Operations {
		if len(op.Args) == 0 {
			return "", fmt.Errorf("operation %s requires arguments", op.Method)
		}

		lambda, err := f.parseFuncLiteral(op.Args[0])
		if err != nil {
			return "", fmt.Errorf("%s: %w", op.Method, err)
		}

		param := ""
		paramType := ""
		if len(lambda.Params) > 0 {
			param = lambda.Params[0].Name
			if lambda.Params[0].Type != "" {
				paramType = lambda.Params[0].Type
			}
		}

		ops = append(ops, ParsedOp{
			Method:     op.Method,
			Param:      param,
			ParamType:  paramType,
			ReturnType: lambda.ReturnType,
			Body:       lambda.Body,
		})

		if len(ops) == 1 {
			loopVar = param
		}
	}

	// Build fused loop body
	var conditions []string
	var transforms []string
	currentVar := loopVar

	elemType := "interface{}"

	for i, op := range ops {
		switch op.Method {
		case "filter":
			predicate := op.Body
			if op.Param != currentVar {
				predicate = strings.ReplaceAll(predicate, op.Param, currentVar)
			}
			conditions = append(conditions, predicate)
			if i == 0 && op.ParamType != "" {
				elemType = op.ParamType
			}

		case "map":
			transform := op.Body
			if op.Param != currentVar {
				transform = strings.ReplaceAll(transform, op.Param, currentVar)
			}
			transforms = append(transforms, transform)
			if op.ReturnType != "" {
				elemType = op.ReturnType
			}
		}

		if op.Method != "filter" && op.Method != "map" {
			return "", fmt.Errorf("chain fusion not yet implemented for operation: %s", op.Method)
		}
	}

	// Generate final transformation expression
	finalValue := loopVar
	if len(transforms) > 0 {
		for i, transform := range transforms {
			if i == 0 {
				finalValue = transform
			} else {
				prevValue := finalValue
				if strings.ContainsAny(prevValue, "+-*/&|^%<>=!") {
					prevValue = "(" + prevValue + ")"
				}
				finalValue = strings.ReplaceAll(transform, loopVar, prevValue)
			}
		}
	}

	// Build IIFE
	var iife strings.Builder
	iife.WriteString(fmt.Sprintf("func() []%s {\n", elemType))
	iife.WriteString(fmt.Sprintf("\t%s := make([]%s, 0, len(%s))\n", tmpVar, elemType, chain.Receiver))
	iife.WriteString(fmt.Sprintf("\tfor _, %s := range %s {\n", loopVar, chain.Receiver))

	if len(conditions) > 0 {
		combinedCondition := strings.Join(conditions, " && ")
		iife.WriteString(fmt.Sprintf("\t\tif %s {\n", combinedCondition))
		iife.WriteString(fmt.Sprintf("\t\t\t%s = append(%s, %s)\n", tmpVar, tmpVar, finalValue))
		iife.WriteString("\t\t}\n")
	} else {
		iife.WriteString(fmt.Sprintf("\t\t%s = append(%s, %s)\n", tmpVar, tmpVar, finalValue))
	}

	iife.WriteString("\t}\n")
	iife.WriteString(fmt.Sprintf("\treturn %s\n", tmpVar))
	iife.WriteString("}()")

	return iife.String(), nil
}

// fuseToTerminal fuses a chain ending in a terminal operation (reduce, all, any)
func (f *FunctionalASTProcessor) fuseToTerminal(chain *dingoast.ChainExpr) (string, error) {
	lastOp := chain.Operations[len(chain.Operations)-1]
	loopVar := "x"

	var filters []string
	var transforms []string

	for i := 0; i < len(chain.Operations)-1; i++ {
		op := chain.Operations[i]

		if len(op.Args) == 0 {
			return "", fmt.Errorf("operation %s requires arguments", op.Method)
		}

		lambda, err := f.parseFuncLiteral(op.Args[0])
		if err != nil {
			return "", fmt.Errorf("%s: %w", op.Method, err)
		}

		param := ""
		if len(lambda.Params) > 0 {
			param = lambda.Params[0].Name
		}

		if i == 0 {
			loopVar = param
		}

		switch op.Method {
		case "filter":
			predicate := lambda.Body
			if param != loopVar {
				predicate = strings.ReplaceAll(predicate, param, loopVar)
			}
			filters = append(filters, predicate)

		case "map":
			transform := lambda.Body
			if param != loopVar {
				transform = strings.ReplaceAll(transform, param, loopVar)
			}
			transforms = append(transforms, transform)
		}
	}

	finalValue := loopVar
	if len(transforms) > 0 {
		for _, transform := range transforms {
			finalValue = transform
		}
	}

	switch lastOp.Method {
	case "reduce":
		return f.fuseReduceChain(chain.Receiver, loopVar, filters, finalValue, lastOp)
	case "all":
		return f.fuseAllChain(chain.Receiver, loopVar, filters, lastOp)
	case "any":
		return f.fuseAnyChain(chain.Receiver, loopVar, filters, lastOp)
	default:
		return "", fmt.Errorf("unsupported terminal operation: %s", lastOp.Method)
	}
}

// fuseReduceChain fuses filter/map chain into reduce
func (f *FunctionalASTProcessor) fuseReduceChain(receiver, loopVar string, filters []string, finalValue string, reduceOp dingoast.FunctionalCall) (string, error) {
	if len(reduceOp.Args) != 2 {
		return "", fmt.Errorf("reduce() requires exactly 2 arguments (initialValue, function), got %d", len(reduceOp.Args))
	}

	initValue := reduceOp.Args[0]
	lambda, err := f.parseFuncLiteral(reduceOp.Args[1])
	if err != nil {
		return "", fmt.Errorf("reduce: %w", err)
	}

	accName := "acc"
	elemName := "x"
	if len(lambda.Params) >= 2 {
		accName = lambda.Params[0].Name
		elemName = lambda.Params[1].Name
	}

	resultType := lambda.ReturnType
	if resultType == "" {
		resultType = "interface{}"
	}

	reduceBody := strings.ReplaceAll(lambda.Body, elemName, finalValue)

	var iife strings.Builder
	iife.WriteString(fmt.Sprintf("func() %s {\n", resultType))
	iife.WriteString(fmt.Sprintf("\t%s := %s\n", accName, initValue))
	iife.WriteString(fmt.Sprintf("\tfor _, %s := range %s {\n", loopVar, receiver))

	if len(filters) > 0 {
		combinedCondition := strings.Join(filters, " && ")
		iife.WriteString(fmt.Sprintf("\t\tif %s {\n", combinedCondition))
		iife.WriteString(fmt.Sprintf("\t\t\t%s = %s\n", accName, reduceBody))
		iife.WriteString("\t\t}\n")
	} else {
		iife.WriteString(fmt.Sprintf("\t\t%s = %s\n", accName, reduceBody))
	}

	iife.WriteString("\t}\n")
	iife.WriteString(fmt.Sprintf("\treturn %s\n", accName))
	iife.WriteString("}()")

	return iife.String(), nil
}

// fuseAllChain fuses filter chain into all()
func (f *FunctionalASTProcessor) fuseAllChain(receiver, loopVar string, filters []string, allOp dingoast.FunctionalCall) (string, error) {
	if len(allOp.Args) != 1 {
		return "", fmt.Errorf("all() requires exactly 1 argument (predicate function), got %d", len(allOp.Args))
	}

	lambda, err := f.parseFuncLiteral(allOp.Args[0])
	if err != nil {
		return "", fmt.Errorf("all: %w", err)
	}

	param := ""
	if len(lambda.Params) > 0 {
		param = lambda.Params[0].Name
	}

	predicate := lambda.Body
	if param != loopVar {
		predicate = strings.ReplaceAll(predicate, param, loopVar)
	}

	allConditions := append(filters, predicate)
	combinedCondition := strings.Join(allConditions, " && ")

	var iife strings.Builder
	iife.WriteString("func() bool {\n")
	iife.WriteString(fmt.Sprintf("\tfor _, %s := range %s {\n", loopVar, receiver))
	iife.WriteString(fmt.Sprintf("\t\tif !(%s) {\n", combinedCondition))
	iife.WriteString("\t\t\treturn false\n")
	iife.WriteString("\t\t}\n")
	iife.WriteString("\t}\n")
	iife.WriteString("\treturn true\n")
	iife.WriteString("}()")

	return iife.String(), nil
}

// fuseAnyChain fuses filter chain into any()
func (f *FunctionalASTProcessor) fuseAnyChain(receiver, loopVar string, filters []string, anyOp dingoast.FunctionalCall) (string, error) {
	if len(anyOp.Args) != 1 {
		return "", fmt.Errorf("any() requires exactly 1 argument (predicate function), got %d", len(anyOp.Args))
	}

	lambda, err := f.parseFuncLiteral(anyOp.Args[0])
	if err != nil {
		return "", fmt.Errorf("any: %w", err)
	}

	param := ""
	if len(lambda.Params) > 0 {
		param = lambda.Params[0].Name
	}

	predicate := lambda.Body
	if param != loopVar {
		predicate = strings.ReplaceAll(predicate, param, loopVar)
	}

	allConditions := append(filters, predicate)
	combinedCondition := strings.Join(allConditions, " && ")

	var iife strings.Builder
	iife.WriteString("func() bool {\n")
	iife.WriteString(fmt.Sprintf("\tfor _, %s := range %s {\n", loopVar, receiver))
	iife.WriteString(fmt.Sprintf("\t\tif %s {\n", combinedCondition))
	iife.WriteString("\t\t\treturn true\n")
	iife.WriteString("\t\t}\n")
	iife.WriteString("\t}\n")
	iife.WriteString("\treturn false\n")
	iife.WriteString("}()")

	return iife.String(), nil
}
