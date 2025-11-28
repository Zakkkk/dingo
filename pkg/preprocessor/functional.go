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

// transformMap transforms: nums.map(func(x) { return x * 2 })
// → func() []_ { tmp := make([]_, 0, len(nums)); for _, x := range nums { tmp = append(tmp, x*2) }; return tmp }()
// NOTE: Uses []_ placeholder - Go's type inference determines concrete type
func (f *FunctionalProcessor) transformMap(mc *MethodCall) (string, error) {
	if len(mc.Args) != 1 {
		return "", fmt.Errorf("map() requires exactly 1 argument (function), got %d", len(mc.Args))
	}

	lambda := mc.Args[0]
	param, body, err := f.parseLambda(lambda)
	if err != nil {
		return "", fmt.Errorf("map: %w", err)
	}

	tmpVar := f.getTempVar("tmp")

	iife := fmt.Sprintf(`func() []_ {
	%s := make([]_, 0, len(%s))
	for _, %s := range %s {
		%s = append(%s, %s)
	}
	return %s
}()`, tmpVar, mc.Receiver, param, mc.Receiver, tmpVar, tmpVar, body, tmpVar)

	return iife, nil
}

// transformFilter transforms: nums.filter(func(x) { return x > 0 })
// → func() []_ { tmp := make([]_, 0, len(nums)); for _, x := range nums { if x > 0 { tmp = append(tmp, x) } }; return tmp }()
// NOTE: Uses []_ placeholder - Go's type inference determines concrete type
func (f *FunctionalProcessor) transformFilter(mc *MethodCall) (string, error) {
	if len(mc.Args) != 1 {
		return "", fmt.Errorf("filter() requires exactly 1 argument (predicate function), got %d", len(mc.Args))
	}

	lambda := mc.Args[0]
	param, body, err := f.parseLambda(lambda)
	if err != nil {
		return "", fmt.Errorf("filter: %w", err)
	}

	tmpVar := f.getTempVar("tmp")

	iife := fmt.Sprintf(`func() []_ {
	%s := make([]_, 0, len(%s))
	for _, %s := range %s {
		if %s {
			%s = append(%s, %s)
		}
	}
	return %s
}()`, tmpVar, mc.Receiver, param, mc.Receiver, body, tmpVar, tmpVar, param, tmpVar)

	return iife, nil
}

// transformReduce transforms: nums.reduce(0, func(acc, x) { return acc + x })
// → func() _ { acc := 0; for _, x := range nums { acc = acc + x }; return acc }()
// NOTE: Uses _ placeholder - Go's type inference determines concrete type from accumulator
func (f *FunctionalProcessor) transformReduce(mc *MethodCall) (string, error) {
	if len(mc.Args) != 2 {
		return "", fmt.Errorf("reduce() requires exactly 2 arguments (initialValue, function), got %d", len(mc.Args))
	}

	initValue := mc.Args[0]
	lambda := mc.Args[1]

	params, body, err := f.parseLambda(lambda)
	if err != nil {
		return "", fmt.Errorf("reduce: %w", err)
	}

	// Parse params: "acc, x" → (acc, x)
	accName, elemName, err := f.parseReduceParams(params)
	if err != nil {
		return "", fmt.Errorf("reduce: %w", err)
	}

	iife := fmt.Sprintf(`func() _ {
	%s := %s
	for _, %s := range %s {
		%s = %s
	}
	return %s
}()`, accName, initValue, elemName, mc.Receiver, accName, body, accName)

	return iife, nil
}

// transformSum transforms: nums.sum()
// → func() _ { sum := 0; for _, x := range nums { sum = sum + x }; return sum }()
// NOTE: Uses _ placeholder - Go's type inference determines numeric type from slice elements
func (f *FunctionalProcessor) transformSum(mc *MethodCall) (string, error) {
	if len(mc.Args) != 0 {
		return "", fmt.Errorf("sum() takes no arguments, got %d", len(mc.Args))
	}

	sumVar := f.getTempVar("sum")

	iife := fmt.Sprintf(`func() _ {
	%s := 0
	for _, x := range %s {
		%s = %s + x
	}
	return %s
}()`, sumVar, mc.Receiver, sumVar, sumVar, sumVar)

	return iife, nil
}

// transformCount transforms: nums.count(func(x) { return x > 0 })
// → func() int { count := 0; for _, x := range nums { if x > 0 { count++ } }; return count }()
func (f *FunctionalProcessor) transformCount(mc *MethodCall) (string, error) {
	if len(mc.Args) != 1 {
		return "", fmt.Errorf("count() requires exactly 1 argument (predicate function), got %d", len(mc.Args))
	}

	lambda := mc.Args[0]
	param, body, err := f.parseLambda(lambda)
	if err != nil {
		return "", fmt.Errorf("count: %w", err)
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
}()`, countVar, param, mc.Receiver, body, countVar, countVar)

	return iife, nil
}

// transformAll transforms: nums.all(func(x) { return x > 0 })
// → func() bool { for _, x := range nums { if !(x > 0) { return false } }; return true }()
func (f *FunctionalProcessor) transformAll(mc *MethodCall) (string, error) {
	if len(mc.Args) != 1 {
		return "", fmt.Errorf("all() requires exactly 1 argument (predicate function), got %d", len(mc.Args))
	}

	lambda := mc.Args[0]
	param, body, err := f.parseLambda(lambda)
	if err != nil {
		return "", fmt.Errorf("all: %w", err)
	}

	iife := fmt.Sprintf(`func() bool {
	for _, %s := range %s {
		if !(%s) {
			return false
		}
	}
	return true
}()`, param, mc.Receiver, body)

	return iife, nil
}

// transformAny transforms: nums.any(func(x) { return x > 0 })
// → func() bool { for _, x := range nums { if x > 0 { return true } }; return false }()
func (f *FunctionalProcessor) transformAny(mc *MethodCall) (string, error) {
	if len(mc.Args) != 1 {
		return "", fmt.Errorf("any() requires exactly 1 argument (predicate function), got %d", len(mc.Args))
	}

	lambda := mc.Args[0]
	param, body, err := f.parseLambda(lambda)
	if err != nil {
		return "", fmt.Errorf("any: %w", err)
	}

	iife := fmt.Sprintf(`func() bool {
	for _, %s := range %s {
		if %s {
			return true
		}
	}
	return false
}()`, param, mc.Receiver, body)

	return iife, nil
}

// Helper Methods

// parseLambda extracts parameter(s) and body from a lambda function
// Input: "func(x) { return x * 2 }" or "func(acc, x) { return acc + x }"
// Returns: (params, body, error)
// Uses balanced parenthesis/brace counting to handle nested structures
func (f *FunctionalProcessor) parseLambda(lambda string) (string, string, error) {
	// Lambda format: func(params) { return body }
	// Extract params using balanced parenthesis counting
	openParen := strings.Index(lambda, "(")
	if openParen == -1 {
		return "", "", fmt.Errorf("invalid lambda: missing '('")
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
		return "", "", fmt.Errorf("invalid lambda: unbalanced parentheses")
	}

	params := strings.TrimSpace(lambda[openParen+1 : closeParen-1])

	// Extract body using balanced brace counting
	openBrace := strings.Index(lambda[closeParen:], "{")
	if openBrace == -1 {
		return "", "", fmt.Errorf("invalid lambda: missing '{'")
	}
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
		return "", "", fmt.Errorf("invalid lambda: unbalanced braces")
	}

	bodyContent := strings.TrimSpace(lambda[openBrace+1 : closeBrace-1])

	// Remove "return " prefix if present
	body := strings.TrimPrefix(bodyContent, "return ")
	body = strings.TrimSpace(body)

	return params, body, nil
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

// transformFind transforms: users.find(func(u) { return u.id == targetId })
// → func() _ { for _, u := range users { if u.id == targetId { return u } }; return nil }()
// NOTE: Returns element or nil - Go's type inference determines concrete type from slice
// TODO(types): Generate proper Option[T] with Some/None when go/types integration is complete
func (f *FunctionalProcessor) transformFind(mc *MethodCall) (string, error) {
	if len(mc.Args) != 1 {
		return "", fmt.Errorf("find() requires exactly 1 argument (predicate function), got %d", len(mc.Args))
	}

	lambda := mc.Args[0]
	param, body, err := f.parseLambda(lambda)
	if err != nil {
		return "", fmt.Errorf("find: %w", err)
	}

	// Return Option_ with Some/None constructors (mangled format)
	iife := fmt.Sprintf(`func() Option_ {
	for _, %s := range %s {
		if %s {
			return Option__Some(%s)
		}
	}
	return Option__None()
}()`, param, mc.Receiver, body, param)

	return iife, nil
}

// transformFindIndex transforms: items.findIndex(func(x) { return x.name == "target" })
// → func() int { for i, x := range items { if x.name == "target" { return i } }; return -1 }()
// NOTE: Returns int with -1 for not found temporarily - proper Option[int] requires type inference
// TODO(types): Generate proper Option[int] with Some/None when go/types integration is complete
func (f *FunctionalProcessor) transformFindIndex(mc *MethodCall) (string, error) {
	if len(mc.Args) != 1 {
		return "", fmt.Errorf("findIndex() requires exactly 1 argument (predicate function), got %d", len(mc.Args))
	}

	lambda := mc.Args[0]
	param, body, err := f.parseLambda(lambda)
	if err != nil {
		return "", fmt.Errorf("findIndex: %w", err)
	}

	// Return Option_int with Some/None constructors (mangled format)
	iife := fmt.Sprintf(`func() Option_int {
	for i, %s := range %s {
		if %s {
			return Option_int_Some(i)
		}
	}
	return Option_int_None()
}()`, param, mc.Receiver, body)

	return iife, nil
}

// transformMapResult transforms: strings.mapResult(func(s) { return parseInt(s) })
// → func() Result[[]_, error] { tmp := make([]_, 0, len(strings)); for _, s := range strings { res := parseInt(s); if res.IsErr() { return Err[[]_](res.UnwrapErr()) }; tmp = append(tmp, res.Unwrap()) }; return Ok(tmp) }()
// NOTE: Returns Result[[]_, error] with Ok/Err constructors
func (f *FunctionalProcessor) transformMapResult(mc *MethodCall) (string, error) {
	if len(mc.Args) != 1 {
		return "", fmt.Errorf("mapResult() requires exactly 1 argument (function returning Result), got %d", len(mc.Args))
	}

	lambda := mc.Args[0]
	param, body, err := f.parseLambda(lambda)
	if err != nil {
		return "", fmt.Errorf("mapResult: %w", err)
	}

	tmpVar := f.getTempVar("tmp")
	resVar := f.getTempVar("res")

	// Return Result_[]__error with mangled type format and Ok/Err constructors
	// Lambda must return a Result type, we propagate the Err variant
	iife := fmt.Sprintf(`func() Result_[]__error {
	%s := make([]_, 0, len(%s))
	for _, %s := range %s {
		%s := %s
		if %s.IsErr() {
			return Result_[]__error_Err(%s.UnwrapErr())
		}
		%s = append(%s, %s.Unwrap())
	}
	return Result_[]__error_Ok(%s)
}()`, tmpVar, mc.Receiver, param, mc.Receiver, resVar, body, resVar, resVar, tmpVar, tmpVar, resVar, tmpVar)

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
	// NOTE: Uses []_ placeholder - Go's type inference determines concrete type
	iife := fmt.Sprintf(`func() []_ {
	%s := make([]_, 0, len(%s))
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
	// NOTE: Uses []_ placeholder - Go's type inference determines concrete type
	iife := fmt.Sprintf(`func() ([]_, []_) {
	%s := make([]_, 0, len(%s))
	%s := make([]_, 0, len(%s))
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
