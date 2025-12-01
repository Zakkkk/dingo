package preprocessor

// TODO(ast-migration): This file uses regex-based transformations which are fragile.
// MIGRATE TO: pkg/ast/functional.go with FunctionalExpr AST nodes
// See: ai-docs/AST_MIGRATION.md for migration plan
// DO NOT fix regex bugs - implement AST-based solution instead

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"github.com/MadAppGang/dingo/pkg/plugin/builtin"
)

// Package-level compiled regexes - LEGACY, TO BE REPLACED WITH AST
var (
	// Method call pattern: receiver.method(args)
	// Captures: receiver, method name
	// Examples: nums.map(...), users.filter(...), items.reduce(...)
	// \b word boundaries prevent false positives like "interfacemap"
	methodCallPattern = regexp.MustCompile(`\b([a-zA-Z_][a-zA-Z0-9_]*)\b\s*\.\s*\b(map|filter|reduce|sum|count|all|any|find|findIndex|mapResult|filterMap|partition)\b\s*\(`)
)

// FunctionalProcessor handles functional utility method transformations
// Transforms: receiver.method(args) → IIFE pattern with loop fusion
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
//
// Processing Order:
// CRITICAL: Must run AFTER LambdaProcessor (lambdas must be expanded first)
//
// Example Transformation:
//   Dingo:  let doubled = nums.map(|x| x * 2)
//   Go:     var doubled = func() []int {
//               tmp := make([]int, 0, len(nums))
//               for _, x := range nums {
//                   tmp = append(tmp, x*2)
//               }
//               return tmp
//           }()
type FunctionalProcessor struct {
	baseCounters  map[string]int // Per-base-name counters for temp variables (no-number-first pattern)
	markerCounter int            // Counter for unique transformation markers
	errors        []error        // Collected errors during processing
	metadata      []TransformMetadata // Transformation metadata for source maps
}

// NewFunctionalProcessor creates a new functional utilities preprocessor
func NewFunctionalProcessor() *FunctionalProcessor {
	return &FunctionalProcessor{
		baseCounters:  make(map[string]int),
		markerCounter: 1,
		errors:        []error{},
		metadata:      []TransformMetadata{},
	}
}

// Name returns the processor name
func (f *FunctionalProcessor) Name() string {
	return "functional"
}

// ProcessBody implements BodyProcessor interface for lambda body processing
func (f *FunctionalProcessor) ProcessBody(body []byte) ([]byte, error) {
	result, _, err := f.Process(body)
	return result, err
}

// Process is the legacy interface method (implements FeatureProcessor)
func (f *FunctionalProcessor) Process(source []byte) ([]byte, []Mapping, error) {
	result, _, err := f.ProcessInternal(string(source))
	return []byte(result), nil, err
}

// ProcessV2 implements FeatureProcessorV2 interface with metadata support
func (f *FunctionalProcessor) ProcessV2(source []byte) (ProcessResult, error) {
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

// ProcessInternal transforms functional method calls with metadata support
// Emits TransformMetadata with unique markers for each transformation
func (f *FunctionalProcessor) ProcessInternal(code string) (string, []TransformMetadata, error) {
	// Reset state for this processing run
	f.baseCounters = make(map[string]int)
	f.markerCounter = 1
	f.errors = []error{}
	f.metadata = []TransformMetadata{}

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
func (f *FunctionalProcessor) processLine(line string, lineNum int) (string, error) {
	// Quick check: does line contain . (method call)?
	if !strings.Contains(line, ".") {
		return line, nil
	}

	// TRY CHAIN DETECTION FIRST (C1 fix: integrate chain fusion into execution path)
	chain := f.detectChain(line)
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
	methodCall := f.detectMethodCall(line)
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

// MethodCall represents a detected functional method call
type MethodCall struct {
	Receiver   string   // The slice/array expression
	Method     string   // Method name (map, filter, etc.)
	Args       []string // Parsed arguments
	StartPos   int      // Start position in line
	EndPos     int      // End position in line
	LineNum    int      // Line number
}

// detectMethodCall detects functional method calls in a line
// Returns nil if no method call found
func (f *FunctionalProcessor) detectMethodCall(line string) *MethodCall {
	// Find all method call matches
	matches := methodCallPattern.FindAllStringSubmatchIndex(line, -1)
	if len(matches) == 0 {
		return nil
	}

	// For now, handle only the first match (chain detection in Task C)
	match := matches[0]

	// Extract receiver and method name from the match
	receiver := line[match[2]:match[3]]
	method := line[match[4]:match[5]]

	// Find the start of arguments (after opening paren)
	argsStart := match[1] // Position after "method("

	// Extract arguments using balanced parenthesis parsing
	args, argsEnd := f.extractArguments(line, argsStart)

	return &MethodCall{
		Receiver: receiver,
		Method:   method,
		Args:     args,
		StartPos: match[0],
		EndPos:   argsEnd,
	}
}

// extractArguments extracts function arguments using balanced parenthesis parsing
// Reuses the pattern from lambda.go's extractBalancedBody
// Returns: (arguments, endPosition)
func (f *FunctionalProcessor) extractArguments(line string, startPos int) ([]string, int) {
	// The startPos should be right after the opening paren
	// We need to find the matching closing paren

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
	// Return what we have
	if argBuf.Len() > 0 {
		args = append(args, strings.TrimSpace(argBuf.String()))
	}
	return args, len(line)
}

// getIndent extracts leading whitespace from a line
func (f *FunctionalProcessor) getIndent(line string) string {
	for i, ch := range line {
		if ch != ' ' && ch != '\t' {
			return line[:i]
		}
	}
	return ""
}

// getTempVar generates a temporary variable name following no-number-first pattern
// First: tmp, then tmp1, tmp2, etc.
func (f *FunctionalProcessor) getTempVar(base string) string {
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

// Transformation Methods (Single Operations)

// transformMap transforms: nums.map(func(x int) int { return x * 2 })
// → func() []int { tmp := make([]int, 0, len(nums)); for _, x := range nums { tmp = append(tmp, x*2) }; return tmp }()
// Uses return type from lambda for slice element type
func (f *FunctionalProcessor) transformMap(mc *MethodCall) (string, error) {
	if len(mc.Args) != 1 {
		return "", fmt.Errorf("map() requires exactly 1 argument (function), got %d", len(mc.Args))
	}

	lambda := mc.Args[0]
	info, err := f.parseLambdaFull(lambda)
	if err != nil {
		return "", fmt.Errorf("map: %w", err)
	}

	// Get parameter name
	paramName := ""
	if len(info.ParamNames) > 0 {
		paramName = info.ParamNames[0]
	}

	// Get return type for slice (fall back to interface{} if not specified)
	elemType := info.ReturnType
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
}()`, elemType, tmpVar, elemType, mc.Receiver, paramName, mc.Receiver, tmpVar, tmpVar, info.Body, tmpVar)

	return iife, nil
}

// transformFilter transforms: nums.filter(func(x int) bool { return x > 0 })
// → func() []int { tmp := make([]int, 0, len(nums)); for _, x := range nums { if x > 0 { tmp = append(tmp, x) } }; return tmp }()
// Uses parameter type from lambda for slice element type
func (f *FunctionalProcessor) transformFilter(mc *MethodCall) (string, error) {
	if len(mc.Args) != 1 {
		return "", fmt.Errorf("filter() requires exactly 1 argument (predicate function), got %d", len(mc.Args))
	}

	lambda := mc.Args[0]
	info, err := f.parseLambdaFull(lambda)
	if err != nil {
		return "", fmt.Errorf("filter: %w", err)
	}

	// Get parameter name and type
	paramName := ""
	elemType := "interface{}"
	if len(info.ParamNames) > 0 {
		paramName = info.ParamNames[0]
	}
	if len(info.ParamTypes) > 0 {
		elemType = info.ParamTypes[0]
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
}()`, elemType, tmpVar, elemType, mc.Receiver, paramName, mc.Receiver, info.Body, tmpVar, tmpVar, paramName, tmpVar)

	return iife, nil
}

// transformReduce transforms: nums.reduce(0, func(acc int, x int) int { return acc + x })
// → func() int { acc := 0; for _, x := range nums { acc = acc + x }; return acc }()
// Uses return type from lambda for result type
func (f *FunctionalProcessor) transformReduce(mc *MethodCall) (string, error) {
	if len(mc.Args) != 2 {
		return "", fmt.Errorf("reduce() requires exactly 2 arguments (initialValue, function), got %d", len(mc.Args))
	}

	initValue := mc.Args[0]
	lambda := mc.Args[1]

	info, err := f.parseLambdaFull(lambda)
	if err != nil {
		return "", fmt.Errorf("reduce: %w", err)
	}

	// Get param names: (acc, x)
	accName := "acc"
	elemName := "x"
	if len(info.ParamNames) >= 2 {
		accName = info.ParamNames[0]
		elemName = info.ParamNames[1]
	}

	// Get return type (fall back to interface{} if not specified)
	resultType := info.ReturnType
	if resultType == "" {
		resultType = "interface{}"
	}

	iife := fmt.Sprintf(`func() %s {
	%s := %s
	for _, %s := range %s {
		%s = %s
	}
	return %s
}()`, resultType, accName, initValue, elemName, mc.Receiver, accName, info.Body, accName)

	return iife, nil
}

// transformSum transforms: nums.sum()
// → func() int { sum := 0; for _, x := range nums { sum = sum + x }; return sum }()
// NOTE: Uses int as default type since sum is typically for numeric slices
func (f *FunctionalProcessor) transformSum(mc *MethodCall) (string, error) {
	if len(mc.Args) != 0 {
		return "", fmt.Errorf("sum() takes no arguments, got %d", len(mc.Args))
	}

	sumVar := f.getTempVar("sum")

	// Use int as default type for sum - works for most numeric types
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
// → func() int { count := 0; for _, x := range nums { if x > 0 { count++ } }; return count }()
func (f *FunctionalProcessor) transformCount(mc *MethodCall) (string, error) {
	if len(mc.Args) != 1 {
		return "", fmt.Errorf("count() requires exactly 1 argument (predicate function), got %d", len(mc.Args))
	}

	lambda := mc.Args[0]
	info, err := f.parseLambdaFull(lambda)
	if err != nil {
		return "", fmt.Errorf("count: %w", err)
	}

	paramName := ""
	if len(info.ParamNames) > 0 {
		paramName = info.ParamNames[0]
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
}()`, countVar, paramName, mc.Receiver, info.Body, countVar, countVar)

	return iife, nil
}

// transformAll transforms: nums.all(func(x int) bool { return x > 0 })
// → func() bool { for _, x := range nums { if !(x > 0) { return false } }; return true }()
func (f *FunctionalProcessor) transformAll(mc *MethodCall) (string, error) {
	if len(mc.Args) != 1 {
		return "", fmt.Errorf("all() requires exactly 1 argument (predicate function), got %d", len(mc.Args))
	}

	lambda := mc.Args[0]
	info, err := f.parseLambdaFull(lambda)
	if err != nil {
		return "", fmt.Errorf("all: %w", err)
	}

	paramName := ""
	if len(info.ParamNames) > 0 {
		paramName = info.ParamNames[0]
	}

	iife := fmt.Sprintf(`func() bool {
	for _, %s := range %s {
		if !(%s) {
			return false
		}
	}
	return true
}()`, paramName, mc.Receiver, info.Body)

	return iife, nil
}

// transformAny transforms: nums.any(func(x int) bool { return x > 0 })
// → func() bool { for _, x := range nums { if x > 0 { return true } }; return false }()
func (f *FunctionalProcessor) transformAny(mc *MethodCall) (string, error) {
	if len(mc.Args) != 1 {
		return "", fmt.Errorf("any() requires exactly 1 argument (predicate function), got %d", len(mc.Args))
	}

	lambda := mc.Args[0]
	info, err := f.parseLambdaFull(lambda)
	if err != nil {
		return "", fmt.Errorf("any: %w", err)
	}

	paramName := ""
	if len(info.ParamNames) > 0 {
		paramName = info.ParamNames[0]
	}

	iife := fmt.Sprintf(`func() bool {
	for _, %s := range %s {
		if %s {
			return true
		}
	}
	return false
}()`, paramName, mc.Receiver, info.Body)

	return iife, nil
}

// Helper Methods

// LambdaInfo holds parsed lambda information
type LambdaInfo struct {
	ParamNames  []string // Just the parameter names (e.g., ["x"] or ["acc", "x"])
	ParamTypes  []string // Parameter types if specified (e.g., ["int"] or ["int", "int"])
	ReturnType  string   // Return type if specified (e.g., "int")
	Body        string   // Lambda body (expression after "return" if present)
}

// parseLambdaFull extracts complete lambda information
// Input: "func(x int) int { return x * 2 }" or "func(acc, x int) int { return acc + x }"
// Returns: LambdaInfo with parsed names, types, and body
func (f *FunctionalProcessor) parseLambdaFull(lambda string) (*LambdaInfo, error) {
	// Lambda format: func(params) [returnType] { return body }
	// Extract params using balanced parenthesis counting
	openParen := strings.Index(lambda, "(")
	if openParen == -1 {
		return nil, fmt.Errorf("invalid lambda: missing '('")
	}

	// Find matching close paren using depth counting
	depth := 1
	closeParen := openParen + 1
	for closeParen < len(lambda) && depth > 0 {
		if lambda[closeParen] == '(' {
			depth++
		} else if lambda[closeParen] == ')' {
			depth--
		}
		closeParen++
	}

	if depth != 0 {
		return nil, fmt.Errorf("invalid lambda: unbalanced parentheses")
	}

	paramsStr := strings.TrimSpace(lambda[openParen+1 : closeParen-1])

	// Extract body using balanced brace counting
	openBrace := strings.Index(lambda[closeParen:], "{")
	if openBrace == -1 {
		return nil, fmt.Errorf("invalid lambda: missing '{'")
	}

	// Extract return type (between ) and {)
	returnTypeStr := strings.TrimSpace(lambda[closeParen:closeParen+openBrace])

	openBrace += closeParen

	depth = 1
	closeBrace := openBrace + 1
	for closeBrace < len(lambda) && depth > 0 {
		if lambda[closeBrace] == '{' {
			depth++
		} else if lambda[closeBrace] == '}' {
			depth--
		}
		closeBrace++
	}

	if depth != 0 {
		return nil, fmt.Errorf("invalid lambda: unbalanced braces")
	}

	bodyContent := strings.TrimSpace(lambda[openBrace+1 : closeBrace-1])

	// Remove "return " prefix if present
	body := strings.TrimPrefix(bodyContent, "return ")
	body = strings.TrimSpace(body)

	// Parse parameters into names and types
	paramNames, paramTypes := f.parseParams(paramsStr)

	return &LambdaInfo{
		ParamNames: paramNames,
		ParamTypes: paramTypes,
		ReturnType: returnTypeStr,
		Body:       body,
	}, nil
}

// parseParams splits parameter string into names and types
// Input: "x int" → (["x"], ["int"])
// Input: "acc int, x int" → (["acc", "x"], ["int", "int"])
// Input: "acc, x" → (["acc", "x"], [])
// Input: "x" → (["x"], [])
func (f *FunctionalProcessor) parseParams(paramsStr string) ([]string, []string) {
	if paramsStr == "" {
		return nil, nil
	}

	var names []string
	var types []string

	// Split by comma for multiple params
	parts := strings.Split(paramsStr, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Split by whitespace to separate name from type
		tokens := strings.Fields(part)
		if len(tokens) >= 1 {
			names = append(names, tokens[0])
		}
		if len(tokens) >= 2 {
			types = append(types, tokens[1])
		}
	}

	return names, types
}

// parseLambda extracts parameter(s) and body from a lambda function (legacy interface)
// Input: "func(x) { return x * 2 }" or "func(acc, x) { return acc + x }"
// Returns: (paramName, body, error) - only first param name for single-param lambdas
// Uses balanced parenthesis/brace counting to handle nested structures
func (f *FunctionalProcessor) parseLambda(lambda string) (string, string, error) {
	info, err := f.parseLambdaFull(lambda)
	if err != nil {
		return "", "", err
	}

	// Return first param name for backward compatibility
	paramName := ""
	if len(info.ParamNames) > 0 {
		paramName = info.ParamNames[0]
	}

	return paramName, info.Body, nil
}

// parseReduceParams splits reduce parameters into (accName, elemName)
// Input: "acc, x"
// Returns: ("acc", "x", error)
func (f *FunctionalProcessor) parseReduceParams(params string) (string, string, error) {
	parts := strings.Split(params, ",")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("reduce lambda must have exactly 2 parameters (accumulator, element), got %d", len(parts))
	}

	accName := strings.TrimSpace(parts[0])
	elemName := strings.TrimSpace(parts[1])

	if accName == "" || elemName == "" {
		return "", "", fmt.Errorf("reduce parameters cannot be empty")
	}

	return accName, elemName, nil
}

// Option/Result Integration Operations (Task D)
// NOTE: Type name sanitization now uses builtin.SanitizeTypeName for consistent camelCase format

// transformFind transforms: users.find(func(u) { return u.id == targetId })
// → func() OptionUser { for _, u := range users { if u.id == targetId { return OptionUserSome(u) } }; return OptionUserNone() }()
// NOTE: Returns Option[T] with camelCase constructors following Go naming conventions
func (f *FunctionalProcessor) transformFind(mc *MethodCall) (string, error) {
	if len(mc.Args) != 1 {
		return "", fmt.Errorf("find() requires exactly 1 argument (predicate function), got %d", len(mc.Args))
	}

	lambda := mc.Args[0]
	info, err := f.parseLambdaFull(lambda)
	if err != nil {
		return "", fmt.Errorf("find: %w", err)
	}

	// Extract parameter name and type
	paramName := ""
	elemType := "interface{}" // Fallback for untyped lambdas
	if len(info.ParamNames) > 0 {
		paramName = info.ParamNames[0]
	}
	if len(info.ParamTypes) > 0 {
		elemType = info.ParamTypes[0]
	}

	// Extract predicate body (everything after parameter declaration)
	body := info.Body

	// Generate camelCase Option type name using SanitizeTypeName
	optionType := "Option" + builtin.SanitizeTypeName(elemType)

	// Return Option{Type} with Some/None constructors (camelCase format)
	iife := fmt.Sprintf(`func() %s {
	for _, %s := range %s {
		if %s {
			return %sSome(%s)
		}
	}
	return %sNone()
}()`, optionType, paramName, mc.Receiver, body, optionType, paramName, optionType)

	return iife, nil
}

// transformFindIndex transforms: items.findIndex(func(x) { return x.name == "target" })
// → func() OptionInt { for i, x := range items { if x.name == "target" { return OptionIntSome(i) } }; return OptionIntNone() }()
// NOTE: Returns Option[int] with camelCase constructors following Go naming conventions
func (f *FunctionalProcessor) transformFindIndex(mc *MethodCall) (string, error) {
	if len(mc.Args) != 1 {
		return "", fmt.Errorf("findIndex() requires exactly 1 argument (predicate function), got %d", len(mc.Args))
	}

	lambda := mc.Args[0]
	param, body, err := f.parseLambda(lambda)
	if err != nil {
		return "", fmt.Errorf("findIndex: %w", err)
	}

	// Generate camelCase Option type name for int
	optionType := "OptionInt"

	// Return OptionInt with Some/None constructors (camelCase format)
	iife := fmt.Sprintf(`func() %s {
	for i, %s := range %s {
		if %s {
			return %sSome(i)
		}
	}
	return %sNone()
}()`, optionType, param, mc.Receiver, body, optionType, optionType)

	return iife, nil
}

// transformMapResult transforms: strings.mapResult(func(s) { return parseInt(s) })
// → func() ResultSliceIntError { tmp := make([]int, 0, len(strings)); for _, s := range strings { res := parseInt(s); if res.IsErr() { return ResultSliceIntErrorErr(res.UnwrapErr()) }; tmp = append(tmp, res.Unwrap()) }; return ResultSliceIntErrorOk(tmp) }()
// NOTE: Returns Result[[]T, error] with Ok/Err constructors (camelCase format)
func (f *FunctionalProcessor) transformMapResult(mc *MethodCall) (string, error) {
	if len(mc.Args) != 1 {
		return "", fmt.Errorf("mapResult() requires exactly 1 argument (function returning Result), got %d", len(mc.Args))
	}

	lambda := mc.Args[0]
	info, err := f.parseLambdaFull(lambda)
	if err != nil {
		return "", fmt.Errorf("mapResult: %w", err)
	}

	param := ""
	if len(info.ParamNames) > 0 {
		param = info.ParamNames[0]
	}

	tmpVar := f.getTempVar("tmp")
	resVar := f.getTempVar("res")

	// Extract element type from lambda's return type (should be Result<T, E>)
	// For now, we use interface{} as the element type placeholder
	// Generate camelCase Result type name: ResultSliceInterfaceError
	sliceType := "[]interface{}"
	errorType := "error"
	resultType := "Result" + builtin.SanitizeTypeName(sliceType, errorType)

	// Return Result with camelCase format and Ok/Err constructors
	// Lambda must return a Result type, we propagate the Err variant
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
}()`, resultType, tmpVar, mc.Receiver, param, mc.Receiver, resVar, info.Body, resVar, resultType, resVar, tmpVar, tmpVar, resVar, resultType, tmpVar)

	return iife, nil
}

// transformFilterMap transforms: items.filterMap(func(x) { return x.maybeParse() })
// → func() []_ { tmp := make([]_, 0, len(items)); for _, x := range items { if opt := x.maybeParse(); opt.IsSome() { tmp = append(tmp, opt.Unwrap()) } }; return tmp }()
func (f *FunctionalProcessor) transformFilterMap(mc *MethodCall) (string, error) {
	if len(mc.Args) != 1 {
		return "", fmt.Errorf("filterMap() requires exactly 1 argument (function returning Option), got %d", len(mc.Args))
	}

	lambda := mc.Args[0]
	param, body, err := f.parseLambda(lambda)
	if err != nil {
		return "", fmt.Errorf("filterMap: %w", err)
	}

	tmpVar := f.getTempVar("tmp")
	optVar := f.getTempVar("opt")

	// Lambda must return an Option type, we keep only Some values
	// NOTE: Uses []interface{} - Go's type inference determines concrete type at usage
	iife := fmt.Sprintf(`func() []interface{} {
	%s := make([]interface{}, 0, len(%s))
	for _, %s := range %s {
		if %s := %s; %s.IsSome() {
			%s = append(%s, %s.Unwrap())
		}
	}
	return %s
}()`, tmpVar, mc.Receiver, param, mc.Receiver, optVar, body, optVar, tmpVar, tmpVar, optVar, tmpVar)

	return iife, nil
}

// transformPartition transforms: users.partition(func(u) { return u.active })
// → func() ([]_, []_) { trueSlice := make([]_, 0, len(users)); falseSlice := make([]_, 0, len(users)); for _, u := range users { if u.active { trueSlice = append(trueSlice, u) } else { falseSlice = append(falseSlice, u) } }; return trueSlice, falseSlice }()
func (f *FunctionalProcessor) transformPartition(mc *MethodCall) (string, error) {
	if len(mc.Args) != 1 {
		return "", fmt.Errorf("partition() requires exactly 1 argument (predicate function), got %d", len(mc.Args))
	}

	lambda := mc.Args[0]
	param, body, err := f.parseLambda(lambda)
	if err != nil {
		return "", fmt.Errorf("partition: %w", err)
	}

	// Generate two temp variable names for the result slices
	trueVar := f.getTempVar("trueSlice")
	falseVar := f.getTempVar("falseSlice")

	// Returns tuple of two slices: (matching, notMatching)
	// NOTE: Uses []interface{} - Go's type inference determines concrete type at usage
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
}()`, trueVar, mc.Receiver, falseVar, mc.Receiver, param, mc.Receiver, body, trueVar, trueVar, param, falseVar, falseVar, param, trueVar, falseVar)

	return iife, nil
}
