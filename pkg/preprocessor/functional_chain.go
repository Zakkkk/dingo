package preprocessor

// TODO(ast-migration): Chain detection uses regex which is fragile.
// MIGRATE TO: pkg/ast/functional.go with proper AST-based chain analysis
// See: ai-docs/AST_MIGRATION.md for migration plan

import (
	"fmt"
	"strings"
)

// ChainedOperation represents a single operation in a method chain
type ChainedOperation struct {
	Method   string   // "map", "filter", "reduce", etc.
	Args     []string // Parsed arguments (lambdas, initial values)
	Position int      // Position in source line (for error reporting)
	EndPos   int      // End position in source line
}

// FusedChain represents a complete chain of operations to be fused
type FusedChain struct {
	Receiver   string              // Original slice expression
	Operations []ChainedOperation  // Ordered chain of operations
	ResultType string              // Inferred or placeholder type
	StartPos   int                 // Start position in line
	EndPos     int                 // End position in line
}

// detectChain detects and parses a chain of method calls
// Returns nil if no chain found, or a FusedChain if detected
//
// Examples:
//   nums.filter(f).map(g) → FusedChain{filter, map}
//   items.map(f).filter(g).reduce(init, r) → FusedChain{map, filter, reduce}
func (f *FunctionalProcessor) detectChain(line string) *FusedChain {
	// Try to find first method call
	firstCall := f.detectMethodCall(line)
	if firstCall == nil {
		return nil
	}

	// Build chain starting from first call
	chain := &FusedChain{
		Receiver:   firstCall.Receiver,
		Operations: []ChainedOperation{},
		StartPos:   firstCall.StartPos,
		EndPos:     firstCall.EndPos,
	}

	// Add first operation
	chain.Operations = append(chain.Operations, ChainedOperation{
		Method:   firstCall.Method,
		Args:     firstCall.Args,
		Position: firstCall.StartPos,
	})

	// Check if there's a continuation (. immediately after closing paren)
	pos := firstCall.EndPos
	for pos < len(line) {
		// Skip whitespace
		for pos < len(line) && (line[pos] == ' ' || line[pos] == '\t') {
			pos++
		}

		// Check for . (chain continuation)
		if pos >= len(line) || line[pos] != '.' {
			// No more chain
			break
		}
		pos++ // Skip the '.'

		// Try to detect next method call from this position
		// We need to construct a substring that looks like "receiver.method("
		// to match our pattern
		restOfLine := line[pos:]
		nextCall := f.detectNextChainedCall(restOfLine, pos)
		if nextCall == nil {
			// Not a functional method - chain ends
			break
		}

		// Add to chain
		chain.Operations = append(chain.Operations, ChainedOperation{
			Method:   nextCall.Method,
			Args:     nextCall.Args,
			Position: pos,
		})

		// Update end position
		chain.EndPos = pos + nextCall.EndPos
		pos = chain.EndPos
	}

	// Only return chain if we have 2+ operations (single ops handled by processLine)
	if len(chain.Operations) < 2 {
		return nil
	}

	return chain
}

// detectNextChainedCall detects a method call at the start of a string
// Used for finding .map(), .filter(), etc. in a chain
// substring should be the part after the '.' in a chain
func (f *FunctionalProcessor) detectNextChainedCall(substring string, offset int) *ChainedOperation {
	// Parse method name (alphanumeric + underscore)
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

	// Check if this is a functional method
	validMethods := map[string]bool{
		"map": true, "filter": true, "reduce": true,
		"sum": true, "count": true, "all": true, "any": true,
		"find": true, "findIndex": true, "mapResult": true,
		"filterMap": true, "partition": true,
	}

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

	return &ChainedOperation{
		Method:   methodName,
		Args:     args,
		Position: offset + methodStart,
		EndPos:   endPos,
	}
}

// fuseChain generates fused IIFE code for a chain of operations
// Combines multiple operations into a single loop for optimal performance
func (f *FunctionalProcessor) fuseChain(chain *FusedChain) (string, error) {
	// Determine fusion strategy based on operation sequence
	// Fusion rules:
	// 1. filter -> map: Single loop with if + append transformed
	// 2. filter -> filter: Combined predicates with &&
	// 3. map -> map: Composed transformation
	// 4. map -> filter: Transform then check
	// 5. any chain -> reduce: Conditional/transformed accumulation
	// 6. any chain -> all/any: Early exit terminates

	// Check for terminal operations (reduce, all, any)
	lastOp := chain.Operations[len(chain.Operations)-1]
	isTerminal := lastOp.Method == "reduce" || lastOp.Method == "all" || lastOp.Method == "any"

	if isTerminal {
		return f.fuseToTerminal(chain)
	}

	// Check for slice-producing chains (filter, map combinations)
	return f.fuseToSlice(chain)
}

// fuseToSlice fuses a chain that produces a slice (filter/map combinations)
// Examples:
//   filter -> map: if pred { append(transform) }
//   filter -> filter: if pred1 && pred2 { append }
//   map -> map: append(compose(f, g))
func (f *FunctionalProcessor) fuseToSlice(chain *FusedChain) (string, error) {
	tmpVar := f.getTempVar("tmp")
	loopVar := "x" // Default loop variable

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

		info, err := f.parseLambdaFull(op.Args[0])
		if err != nil {
			return "", fmt.Errorf("%s: %w", op.Method, err)
		}

		param := ""
		paramType := ""
		if len(info.ParamNames) > 0 {
			param = info.ParamNames[0]
		}
		if len(info.ParamTypes) > 0 {
			paramType = info.ParamTypes[0]
		}

		ops = append(ops, ParsedOp{
			Method:     op.Method,
			Param:      param,
			ParamType:  paramType,
			ReturnType: info.ReturnType,
			Body:       info.Body,
		})

		// Use the first operation's parameter as loop variable
		if len(ops) == 1 {
			loopVar = param
		}
	}

	// Build fused loop body
	var conditions []string // Filter predicates
	var transforms []string // Map transformations
	currentVar := loopVar   // Track variable name through transformations

	// Determine element type for the result slice
	// - For map chains: use the return type of the last map operation
	// - For filter chains: use the param type of the first filter
	// - Default to interface{} if no type info
	elemType := "interface{}"

	for i, op := range ops {
		switch op.Method {
		case "filter":
			// Accumulate filter conditions
			// Substitute the parameter name with current variable
			predicate := op.Body
			if op.Param != currentVar {
				predicate = strings.ReplaceAll(predicate, op.Param, currentVar)
			}
			conditions = append(conditions, predicate)
			// Filter preserves element type
			if i == 0 && op.ParamType != "" {
				elemType = op.ParamType
			}

		case "map":
			// Chain map transformations
			transform := op.Body
			if op.Param != currentVar {
				transform = strings.ReplaceAll(transform, op.Param, currentVar)
			}
			transforms = append(transforms, transform)
			// Map changes element type to return type
			if op.ReturnType != "" {
				elemType = op.ReturnType
			}
		}

		// Check for unsupported patterns
		if op.Method != "filter" && op.Method != "map" {
			return "", fmt.Errorf("chain fusion not yet implemented for operation: %s", op.Method)
		}

		// Detect invalid fusion patterns
		if i > 0 {
			_ = ops[i-1].Method
			// map -> filter is supported
			// filter -> map is supported
			// filter -> filter is supported
			// map -> map is supported
			// All current combinations work with our strategy
		}
	}

	// Generate final transformation expression
	finalValue := loopVar
	if len(transforms) > 0 {
		// Compose all map transformations by progressive substitution
		// All transforms are already normalized to use loopVar (done in initial parsing loop above)
		// For map chain: nums.map(|x| x*2).map(|y| y+1)
		//   transforms = ["x*2", "x+1"] (y substituted to x)
		//   Result: compose by substituting x in "x+1" with "x*2" → "(x*2)+1"
		for i, transform := range transforms {
			if i == 0 {
				// First transform uses loop variable directly
				finalValue = transform
			} else {
				// Subsequent transforms: substitute loopVar with accumulated value
				prevValue := finalValue
				// Wrap in parens if previous value contains operators
				if strings.ContainsAny(prevValue, "+-*/&|^%<>=!") {
					prevValue = "(" + prevValue + ")"
				}
				finalValue = strings.ReplaceAll(transform, loopVar, prevValue)
			}
		}
	}

	// Build IIFE with proper element type
	var iife strings.Builder
	iife.WriteString(fmt.Sprintf("func() []%s {\n", elemType))
	iife.WriteString(fmt.Sprintf("\t%s := make([]%s, 0, len(%s))\n", tmpVar, elemType, chain.Receiver))
	iife.WriteString(fmt.Sprintf("\tfor _, %s := range %s {\n", loopVar, chain.Receiver))

	// Add filter conditions
	if len(conditions) > 0 {
		combinedCondition := strings.Join(conditions, " && ")
		iife.WriteString(fmt.Sprintf("\t\tif %s {\n", combinedCondition))
		iife.WriteString(fmt.Sprintf("\t\t\t%s = append(%s, %s)\n", tmpVar, tmpVar, finalValue))
		iife.WriteString("\t\t}\n")
	} else {
		// No filters, just transformations
		iife.WriteString(fmt.Sprintf("\t\t%s = append(%s, %s)\n", tmpVar, tmpVar, finalValue))
	}

	iife.WriteString("\t}\n")
	iife.WriteString(fmt.Sprintf("\treturn %s\n", tmpVar))
	iife.WriteString("}()")

	return iife.String(), nil
}

// fuseToTerminal fuses a chain ending in a terminal operation (reduce, all, any)
// Examples:
//   filter -> reduce: Conditional accumulation
//   map -> reduce: Transformed accumulation
//   filter -> map -> reduce: Conditional + transformed accumulation
//   filter -> all: Combined predicate with early exit
func (f *FunctionalProcessor) fuseToTerminal(chain *FusedChain) (string, error) {
	lastOp := chain.Operations[len(chain.Operations)-1]
	loopVar := "x"

	// Parse all operations except the last (terminal)
	var filters []string  // Filter predicates
	var transforms []string // Map transformations

	for i := 0; i < len(chain.Operations)-1; i++ {
		op := chain.Operations[i]

		if len(op.Args) == 0 {
			return "", fmt.Errorf("operation %s requires arguments", op.Method)
		}

		param, body, err := f.parseLambda(op.Args[0])
		if err != nil {
			return "", fmt.Errorf("%s: %w", op.Method, err)
		}

		if i == 0 {
			loopVar = param
		}

		switch op.Method {
		case "filter":
			predicate := body
			if param != loopVar {
				predicate = strings.ReplaceAll(predicate, param, loopVar)
			}
			filters = append(filters, predicate)

		case "map":
			transform := body
			if param != loopVar {
				transform = strings.ReplaceAll(transform, param, loopVar)
			}
			transforms = append(transforms, transform)
		}
	}

	// Compute final value expression
	finalValue := loopVar
	if len(transforms) > 0 {
		for _, transform := range transforms {
			finalValue = transform
		}
	}

	// Generate IIFE based on terminal operation
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
func (f *FunctionalProcessor) fuseReduceChain(receiver, loopVar string, filters []string, finalValue string, reduceOp ChainedOperation) (string, error) {
	if len(reduceOp.Args) != 2 {
		return "", fmt.Errorf("reduce() requires exactly 2 arguments (initialValue, function), got %d", len(reduceOp.Args))
	}

	initValue := reduceOp.Args[0]
	reduceLambda := reduceOp.Args[1]

	info, err := f.parseLambdaFull(reduceLambda)
	if err != nil {
		return "", fmt.Errorf("reduce: %w", err)
	}

	// Get param names: (acc, elem)
	accName := "acc"
	elemName := "x"
	if len(info.ParamNames) >= 2 {
		accName = info.ParamNames[0]
		elemName = info.ParamNames[1]
	} else if len(info.ParamNames) == 1 {
		accName = info.ParamNames[0]
	}

	// Get return type (fall back to interface{} if not specified)
	resultType := info.ReturnType
	if resultType == "" {
		resultType = "interface{}"
	}

	// Substitute element name with finalValue in reduce body
	reduceBody := strings.ReplaceAll(info.Body, elemName, finalValue)

	var iife strings.Builder
	iife.WriteString(fmt.Sprintf("func() %s {\n", resultType))
	iife.WriteString(fmt.Sprintf("\t%s := %s\n", accName, initValue))
	iife.WriteString(fmt.Sprintf("\tfor _, %s := range %s {\n", loopVar, receiver))

	// Add filter conditions
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

// fuseAllChain fuses filter chain into all() with combined predicate
func (f *FunctionalProcessor) fuseAllChain(receiver, loopVar string, filters []string, allOp ChainedOperation) (string, error) {
	if len(allOp.Args) != 1 {
		return "", fmt.Errorf("all() requires exactly 1 argument (predicate function), got %d", len(allOp.Args))
	}

	param, body, err := f.parseLambda(allOp.Args[0])
	if err != nil {
		return "", fmt.Errorf("all: %w", err)
	}

	predicate := body
	if param != loopVar {
		predicate = strings.ReplaceAll(predicate, param, loopVar)
	}

	// Combine all filters with the all() predicate
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

// fuseAnyChain fuses filter chain into any() with combined predicate
func (f *FunctionalProcessor) fuseAnyChain(receiver, loopVar string, filters []string, anyOp ChainedOperation) (string, error) {
	if len(anyOp.Args) != 1 {
		return "", fmt.Errorf("any() requires exactly 1 argument (predicate function), got %d", len(anyOp.Args))
	}

	param, body, err := f.parseLambda(anyOp.Args[0])
	if err != nil {
		return "", fmt.Errorf("any: %w", err)
	}

	predicate := body
	if param != loopVar {
		predicate = strings.ReplaceAll(predicate, param, loopVar)
	}

	// Combine filters with the any() predicate
	// For any(), we need: (filter1 && filter2 && ... && anyPredicate)
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
