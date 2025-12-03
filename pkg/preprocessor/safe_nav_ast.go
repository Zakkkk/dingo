package preprocessor

// AST-based safe navigation preprocessor
// Migrated from regex-based safe_nav.go
// See: ai-docs/sessions/20251130-234929-ast-migration/02-implementation/task-safe_nav-*.md

import (
	"bytes"
	"fmt"
	goast "go/ast"
	goparser "go/parser"
	"go/token"
	"strings"

	"github.com/MadAppGang/dingo/pkg/ast"
)

// SafeNavASTProcessor handles the ?. operator using AST-based parsing
// Transforms: user?.address?.city → null-safe chain with Option/pointer checks
// Supports both Option<T> types and raw Go pointers (*T)
type SafeNavASTProcessor struct {
	typeDetector   *TypeDetector   // Fallback regex-based type detector
	typeAnalyzer   *TypeAnalyzer   // NEW: go/types based analyzer (preferred)
	optionDetector *OptionDetector // NEW: dual-strategy Option detection
	tmpCounter     int
	optCounter     int // Counter for opt variables (opt, opt1, opt2, ...)
}

// SafeNavContext represents the context where a safe nav expression appears
type SafeNavContext int

const (
	ContextAssignment       SafeNavContext = iota // let x = user?.name
	ContextReturn                                 // return user?.name
	ContextFunctionArg                            // process(user?.name)
	ContextBooleanCondition                       // if user?.isActive
	ContextInline                                 // default inline replacement
)

// NewSafeNavASTProcessor creates a new AST-based safe navigation processor
func NewSafeNavASTProcessor() *SafeNavASTProcessor {
	return &SafeNavASTProcessor{
		typeDetector:   NewTypeDetector(),
		typeAnalyzer:   nil, // Initialized during Process if needed
		optionDetector: NewOptionDetector(),
		tmpCounter:     1,
		optCounter:     1,
	}
}

// NewSafeNavASTProcessorWithAnalyzer creates processor with TypeAnalyzer
func NewSafeNavASTProcessorWithAnalyzer(analyzer *TypeAnalyzer) *SafeNavASTProcessor {
	return &SafeNavASTProcessor{
		typeDetector:   NewTypeDetector(),
		typeAnalyzer:   analyzer,
		optionDetector: NewOptionDetector(),
		tmpCounter:     1,
		optCounter:     1,
	}
}

// Name returns the processor name
func (s *SafeNavASTProcessor) Name() string {
	return "safe_navigation_ast"
}

// ProcessBody implements BodyProcessor interface for lambda body processing
func (s *SafeNavASTProcessor) ProcessBody(body []byte) ([]byte, error) {
	result, _, err := s.Process(body)
	return result, err
}

// Process implements FeatureProcessor interface for backward compatibility
func (s *SafeNavASTProcessor) Process(source []byte) ([]byte, []Mapping, error) {
	result, err := s.ProcessV2(source)
	if err != nil {
		return nil, nil, err
	}
	return result.Source, result.Mappings, nil
}

// ProcessV2 implements FeatureProcessorV2 interface with metadata support
func (s *SafeNavASTProcessor) ProcessV2(source []byte) (ProcessResult, error) {
	transformed, metadata, err := s.ProcessInternal(string(source))
	if err != nil {
		return ProcessResult{}, err
	}
	return ProcessResult{
		Source:   []byte(transformed),
		Mappings: nil,
		Metadata: metadata,
	}, nil
}

// ProcessInternal implements AST-based safe navigation transformation with metadata emission
func (s *SafeNavASTProcessor) ProcessInternal(code string) (string, []TransformMetadata, error) {
	// Parse source for type detection
	s.typeDetector.ParseSource([]byte(code))

	// Initialize type analyzer for this source if not already set
	if s.typeAnalyzer == nil {
		s.typeAnalyzer = NewTypeAnalyzer()
		if err := s.typeAnalyzer.AnalyzeFile(code); err != nil {
			// Log warning but continue with regex fallback
			// Type inference will gracefully fall back to regex-based detection
			// Note: This is expected for incomplete code during development
		}
	}

	// Reset state
	s.tmpCounter = 1
	s.optCounter = 1

	var metadata []TransformMetadata
	counter := 0

	lines := strings.Split(code, "\n")
	var output bytes.Buffer

	inputLineNum := 0
	outputLineNum := 1

	for inputLineNum < len(lines) {
		line := lines[inputLineNum]

		// Process the line with metadata
		transformed, meta, err := s.processLineWithMetadata(line, inputLineNum+1, outputLineNum, &counter)
		if err != nil {
			return "", nil, fmt.Errorf("line %d: %w", inputLineNum+1, err)
		}

		output.WriteString(transformed)
		if inputLineNum < len(lines)-1 {
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

// processLineWithMetadata processes a single line with metadata generation for Post-AST source maps
func (s *SafeNavASTProcessor) processLineWithMetadata(line string, originalLineNum int, outputLineNum int, markerCounter *int) (string, *TransformMetadata, error) {
	// Quick check: does line contain ?. operator?
	if !strings.Contains(line, "?.") {
		return line, nil, nil
	}

	// Check if all ?. occurrences are inside comments
	commentStart := findCommentStart(line)
	if commentStart != -1 {
		firstSafeNav := strings.Index(line, "?.")
		if firstSafeNav >= commentStart {
			return line, nil, nil
		}
	}

	// Parse the line for safe navigation expressions
	exprs, err := s.parseSafeNavLine(line, originalLineNum)
	if err != nil {
		return "", nil, err
	}

	// If no expressions found, return unchanged
	if len(exprs) == 0 {
		return line, nil, nil
	}

	// Detect the context using AST-based analysis
	ctx := s.detectContext(line, exprs)

	// Handle boolean condition context with short-circuit pattern
	if ctx == ContextBooleanCondition {
		return s.processWithBooleanShortCircuit(line, exprs, originalLineNum, outputLineNum, markerCounter)
	}

	// Handle contexts requiring hoisting (assignment, return, function arg)
	if ctx == ContextAssignment || ctx == ContextReturn || ctx == ContextFunctionArg {
		return s.processWithHoisting(line, exprs, originalLineNum, outputLineNum, markerCounter)
	}

	// Original behavior: inline replacement
	result := line
	var meta *TransformMetadata
	firstTransform := true

	// Process in reverse order to maintain string positions
	for i := len(exprs) - 1; i >= 0; i-- {
		exprPos := exprs[i]

		// Detect base type using new determineBaseType flow
		baseType := s.determineBaseType(exprPos.expr.Receiver)

		// Create marker for metadata
		marker := fmt.Sprintf("// dingo:s:%d", *markerCounter)

		// Generate code with marker inserted (pass full chain)
		replacement, err := s.generateSafeNavCodeWithMarker(exprPos.expr.Receiver, exprPos.chain, baseType, marker, originalLineNum, outputLineNum)
		if err != nil {
			return "", nil, err
		}

		// Create metadata for first transformation only
		if firstTransform {
			meta = &TransformMetadata{
				Type:            "safe_nav",
				OriginalLine:    originalLineNum,
				OriginalColumn:  exprPos.startCol + 1,
				OriginalLength:  2, // length of ?.
				OriginalText:    "?.",
				GeneratedMarker: marker,
				ASTNodeType:     "CallExpr",
			}
			*markerCounter++
			firstTransform = false
		}

		// Replace in result
		result = result[:exprPos.start] + replacement + result[exprPos.end:]
	}

	return result, meta, nil
}

// detectContext uses go/parser to determine the context of a safe nav expression
func (s *SafeNavASTProcessor) detectContext(line string, exprs []safeNavExprPosition) SafeNavContext {
	// Quick string-based checks for common cases
	trimmed := strings.TrimSpace(line)

	// Check for if statement - MOST IMPORTANT
	if strings.HasPrefix(trimmed, "if ") {
		// If statement always uses boolean context
		return ContextBooleanCondition
	}

	// Check for assignment
	if strings.Contains(trimmed, ":=") || (strings.Contains(trimmed, "=") && !strings.Contains(trimmed, "==") && !strings.Contains(trimmed, "!=")) {
		return ContextAssignment
	}

	// Check for return statement
	if strings.HasPrefix(trimmed, "return ") {
		return ContextReturn
	}

	// For more complex cases, replace ?. with regular . to make it parseable as Go
	// This is simpler and avoids position tracking issues
	parseable := strings.ReplaceAll(line, "?.", ".")

	// Parse the line as a Go statement to detect context
	// Wrap in minimal valid Go code for parsing
	wrapped := "package p\nfunc _() {\n" + parseable + "\n}"

	fset := token.NewFileSet()
	file, err := goparser.ParseFile(fset, "", wrapped, goparser.AllErrors)
	if err != nil {
		// Parse failed - fall back to inline
		return ContextInline
	}

	// Walk AST to find the context
	var foundContext SafeNavContext = ContextInline

	goast.Inspect(file, func(n goast.Node) bool {
		switch stmt := n.(type) {
		case *goast.IfStmt:
			// Check if safe nav is in the condition
			if stmt.Cond != nil {
				foundContext = ContextBooleanCondition
				return false
			}
		case *goast.CallExpr:
			// Check if safe nav is in function arguments
			if len(stmt.Args) > 0 {
				foundContext = ContextFunctionArg
				return false
			}
		case *goast.BinaryExpr:
			// Check if safe nav is operand of boolean operator
			if stmt.Op == token.LAND || stmt.Op == token.LOR {
				foundContext = ContextBooleanCondition
				return false
			}
		}
		return true
	})

	return foundContext
}

// processWithBooleanShortCircuit generates short-circuit pattern for boolean contexts
// Input:  if user?.isActive { }
// Output: if user.IsSome() && user.Unwrap().isActive { }
func (s *SafeNavASTProcessor) processWithBooleanShortCircuit(line string, exprs []safeNavExprPosition, originalLineNum int, outputLineNum int, markerCounter *int) (string, *TransformMetadata, error) {
	result := line
	var meta *TransformMetadata
	firstTransform := true

	// Process in reverse order to maintain string positions
	for i := len(exprs) - 1; i >= 0; i-- {
		exprPos := exprs[i]

		// Detect base type
		baseType := s.determineBaseType(exprPos.expr.Receiver)

		// Create marker for metadata
		marker := fmt.Sprintf("// dingo:s:%d", *markerCounter)

		// Generate short-circuit code based on type
		var replacement string
		if baseType == TypeOption {
			// Option type: user?.profile?.verified
			// Generates: user.IsSome() && user.Unwrap().profile.IsSome() && user.Unwrap().profile.Unwrap().verified
			replacement = s.generateOptionShortCircuit(exprPos.expr.Receiver, exprPos.chain)
		} else if baseType == TypePointer {
			// Pointer type: user?.profile?.verified
			// Generates: user != nil && user.profile != nil && user.profile.verified
			replacement = s.generatePointerShortCircuit(exprPos.expr.Receiver, exprPos.chain)
		} else {
			// Unknown type - return error
			return "", nil, fmt.Errorf(
				"line %d: cannot infer type for '%s' in boolean safe navigation\n\n"+
					"  Help: Add explicit type annotation (Option<T> or *T)",
				originalLineNum, exprPos.expr.Receiver)
		}

		// Create metadata for first transformation only
		if firstTransform {
			meta = &TransformMetadata{
				Type:            "safe_nav",
				OriginalLine:    originalLineNum,
				OriginalColumn:  exprPos.startCol + 1,
				OriginalLength:  2, // length of ?.
				OriginalText:    "?.",
				GeneratedMarker: marker,
				ASTNodeType:     "BinaryExpr",
			}
			*markerCounter++
			firstTransform = false
		}

		// Replace in result (no marker insertion for inline replacement)
		result = result[:exprPos.start] + replacement + result[exprPos.end:]
	}

	return result, meta, nil
}

// generateOptionShortCircuit generates short-circuit pattern for Option types
// user?.profile?.verified → user.IsSome() && user.Unwrap().profile.IsSome() && user.Unwrap().profile.Unwrap().verified
func (s *SafeNavASTProcessor) generateOptionShortCircuit(base string, chain []ChainElement) string {
	var parts []string
	currentExpr := base

	for i, elem := range chain {
		// Add IsSome() check
		parts = append(parts, fmt.Sprintf("%s.IsSome()", currentExpr))

		// Build the unwrapped access expression
		if i < len(chain)-1 {
			// Not the last element - need to access next level
			if elem.IsMethod {
				currentExpr = fmt.Sprintf("%s.Unwrap().%s(%s)", currentExpr, elem.Name, elem.RawArgs)
			} else {
				currentExpr = fmt.Sprintf("%s.Unwrap().%s", currentExpr, elem.Name)
			}
		} else {
			// Last element - this is the final value access
			var finalAccess string
			if elem.IsMethod {
				finalAccess = fmt.Sprintf("%s.Unwrap().%s(%s)", currentExpr, elem.Name, elem.RawArgs)
			} else {
				finalAccess = fmt.Sprintf("%s.Unwrap().%s", currentExpr, elem.Name)
			}
			parts = append(parts, finalAccess)
		}
	}

	return strings.Join(parts, " && ")
}

// generatePointerShortCircuit generates short-circuit pattern for pointer types
// user?.profile?.verified → user != nil && user.profile != nil && user.profile.verified
func (s *SafeNavASTProcessor) generatePointerShortCircuit(base string, chain []ChainElement) string {
	var parts []string
	currentExpr := base

	for i, elem := range chain {
		// Add nil check
		parts = append(parts, fmt.Sprintf("%s != nil", currentExpr))

		// Build the access expression
		if i < len(chain)-1 {
			// Not the last element - need to access next level
			if elem.IsMethod {
				currentExpr = fmt.Sprintf("%s.%s(%s)", currentExpr, elem.Name, elem.RawArgs)
			} else {
				currentExpr = fmt.Sprintf("%s.%s", currentExpr, elem.Name)
			}
		} else {
			// Last element - this is the final value access
			var finalAccess string
			if elem.IsMethod {
				finalAccess = fmt.Sprintf("%s.%s(%s)", currentExpr, elem.Name, elem.RawArgs)
			} else {
				finalAccess = fmt.Sprintf("%s.%s", currentExpr, elem.Name)
			}
			parts = append(parts, finalAccess)
		}
	}

	return strings.Join(parts, " && ")
}

// processWithHoisting generates hoisted if-statements before the line
func (s *SafeNavASTProcessor) processWithHoisting(line string, exprs []safeNavExprPosition, originalLineNum int, outputLineNum int, markerCounter *int) (string, *TransformMetadata, error) {
	var hoistedCode bytes.Buffer
	var meta *TransformMetadata
	firstTransform := true

	result := line
	var optVars []string

	// Process each expression and generate hoisted code
	// Process in reverse order to maintain string positions for replacement
	for i := len(exprs) - 1; i >= 0; i-- {
		exprPos := exprs[i]

		// Detect base type
		baseType := s.determineBaseType(exprPos.expr.Receiver)

		// Create marker for metadata
		marker := fmt.Sprintf("// dingo:s:%d", *markerCounter)

		// Generate hoisted if-statement block
		hoistedBlock, optVar, err := s.generateHoistedBlock(exprPos.expr.Receiver, exprPos.chain, baseType, marker, originalLineNum, outputLineNum)
		if err != nil {
			return "", nil, err
		}

		// Prepend to hoisted code (since we're processing in reverse, prepending gives correct left-to-right order)
		existingCode := hoistedCode.String()
		hoistedCode.Reset()
		hoistedCode.WriteString(hoistedBlock)
		hoistedCode.WriteString(existingCode)
		optVars = append([]string{optVar}, optVars...) // prepend to maintain order

		// Create metadata for first transformation
		if firstTransform {
			meta = &TransformMetadata{
				Type:            "safe_nav",
				OriginalLine:    originalLineNum,
				OriginalColumn:  exprPos.startCol + 1,
				OriginalLength:  2, // length of ?.
				OriginalText:    "?.",
				GeneratedMarker: marker,
				ASTNodeType:     "CallExpr",
			}
			*markerCounter++
			firstTransform = false
		}

		// Replace safe nav expression with just the opt variable
		result = result[:exprPos.start] + optVar + result[exprPos.end:]
	}

	// Combine hoisted code with modified line
	finalResult := hoistedCode.String() + result

	return finalResult, meta, nil
}

// generateHoistedBlock generates an if-statement block and returns it with the opt variable name
func (s *SafeNavASTProcessor) generateHoistedBlock(base string, chain []ChainElement, baseType TypeKind, marker string, originalLine int, outputLine int) (string, string, error) {
	// Generate the if-statement code
	code, err := s.generateSafeNavCode(base, chain, baseType, originalLine, outputLine)
	if err != nil {
		return "", "", err
	}

	// Extract the opt variable name from the generated code
	// Format: "var opt __INFER__\n..."
	varLine := strings.Split(code, "\n")[0]
	varParts := strings.Fields(varLine)
	var optVar string
	if len(varParts) >= 2 {
		optVar = varParts[1]
	} else {
		return "", "", fmt.Errorf("failed to extract opt variable from generated code")
	}

	// Insert marker after var declaration
	varIdx := strings.Index(code, "\n")
	if varIdx != -1 && strings.HasPrefix(code, "var opt") {
		before := code[:varIdx]
		after := code[varIdx:]
		code = before + " " + marker + after
	}

	// Add newline after the block for proper formatting
	code += "\n"

	return code, optVar, nil
}

// safeNavExprPosition tracks a safe nav expression's position in source
type safeNavExprPosition struct {
	expr     *ast.SafeNavExpr
	chain    []ChainElement // Full chain of elements
	start    int            // start byte position in line
	end      int            // end byte position in line
	startCol int            // start column (0-based)
}

// parseSafeNavLine parses a line for safe navigation expressions using character-level scanning
func (s *SafeNavASTProcessor) parseSafeNavLine(line string, lineNum int) ([]safeNavExprPosition, error) {
	var exprs []safeNavExprPosition

	// Find comment start position
	commentStart := findCommentStart(line)

	// Scan for ?. operators using character-level parsing
	i := 0
	for i < len(line) {
		// Skip if inside comment
		if commentStart != -1 && i >= commentStart {
			break
		}

		// Look for ?. pattern
		if i+1 < len(line) && line[i] == '?' && line[i+1] == '.' {
			// Found ?. - now extract the base identifier before it
			baseStart, baseEnd := extractBaseBefore(line, i)
			if baseStart == -1 {
				return nil, fmt.Errorf(
					"line %d, col %d: safe navigation operator without receiver\n\n"+
						"  Found: ?. at position %d (no variable before it)\n\n"+
						"  Help: Safe navigation requires a base expression\n\n"+
						"    Correct usage:\n"+
						"      user?.profile\n"+
						"      getUser()?.name\n"+
						"      repo.Find(id)?.Update()\n\n"+
						"  Note: Ensure there is a valid expression before ?.",
					lineNum, i+1, i+1)
			}

			base := line[baseStart:baseEnd]

			// Extract the full chain starting from this ?.
			chainStart := i

			// Parse the chain after ?.
			chain, chainEnd, err := s.parseChainAfter(line, chainStart+2, lineNum)
			if err != nil {
				return nil, err
			}

			if len(chain) == 0 {
				// Check if trailing ?. is inside comment
				if commentStart != -1 && i >= commentStart {
					// Trailing ?. in comment, skip
					i++
					continue
				}
				return nil, fmt.Errorf(
					"line %d, col %d: trailing safe navigation operator without property\n\n"+
						"  Found: %s?. (incomplete expression)\n\n"+
						"  Help: Safe navigation (?.) requires a property or method after it\n\n"+
						"    Add property access:\n"+
						"      %s?.name\n"+
						"      %s?.profile?.city\n\n"+
						"    Add method call:\n"+
						"      %s?.getName()\n"+
						"      %s?.getProfile()?.getCity()\n\n"+
						"  Note: Did you mean error propagation (?) instead of safe navigation (?.)?",
					lineNum, i+1, base, base, base, base, base)
			}

			// Build SafeNavExpr with all chain elements
			expr := &ast.SafeNavExpr{
				Receiver: base,
				OpPos:    token.Pos(chainStart),
				Field:    chain[0].Name,
				Chain:    nil, // Chain info stored in ChainElement slice
			}

			// Store position info along with full chain
			exprs = append(exprs, safeNavExprPosition{
				expr:     expr,
				chain:    chain,
				start:    baseStart,
				end:      chainEnd,
				startCol: baseStart,
			})

			// Skip past this chain
			i = chainEnd - 1
		}

		i++
	}

	return exprs, nil
}

// determineBaseType determines the type of a base identifier using TypeAnalyzer with fallback
// Flow:
//  1. Try TypeAnalyzer (go/types) first - most accurate
//  2. If fails, fallback to TypeDetector (regex-based)
//  3. If both fail, return TypeUnknown for clear error
func (s *SafeNavASTProcessor) determineBaseType(varName string) TypeKind {
	// Strategy 1: Try TypeAnalyzer (go/types based)
	if s.typeAnalyzer != nil && s.typeAnalyzer.HasTypeInfo() {
		if typ, ok := s.typeAnalyzer.TypeOf(varName); ok {
			// Successfully got type - now classify it

			// Use OptionDetector for Option type identification
			if s.optionDetector.IsOption(typ) {
				return TypeOption
			}

			// Use TypeAnalyzer for pointer detection
			if s.typeAnalyzer.IsPointer(typ) {
				return TypePointer
			}

			// Regular non-nullable type
			return TypeRegular
		}
	}

	// Strategy 2: Fallback to regex-based TypeDetector
	detectedType := s.typeDetector.DetectType(varName)

	// If TypeDetector found something, use it
	if detectedType != TypeUnknown {
		return detectedType
	}

	// Strategy 3: Both failed - return TypeUnknown for clear error
	return TypeUnknown
}

// parseChainAfter parses the chain after a ?. operator
// Returns: chain elements and end position
func (s *SafeNavASTProcessor) parseChainAfter(line string, start int, lineNum int) ([]ChainElement, int, error) {
	var chain []ChainElement
	i := start

	for i < len(line) {
		// Skip whitespace
		for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
			i++
		}

		if i >= len(line) {
			break
		}

		// Extract property/method name
		nameStart := i
		for i < len(line) {
			ch := line[i]
			if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' {
				i++
			} else {
				break
			}
		}

		if i == nameStart {
			// No name found
			break
		}

		name := line[nameStart:i]

		// Check if it's a method call (has parentheses)
		if i < len(line) && line[i] == '(' {
			// Method call: extract balanced parentheses
			argsStart := i + 1
			depth := 1
			i++ // Skip opening (

			for i < len(line) && depth > 0 {
				ch := line[i]

				if ch == '(' {
					depth++
				} else if ch == ')' {
					depth--
				} else if ch == '"' || ch == '\'' || ch == '`' {
					// Skip string literals
					quote := ch
					i++
					for i < len(line) {
						if line[i] == quote {
							if i > 0 && line[i-1] != '\\' {
								break
							}
						}
						i++
					}
				}

				i++
			}

			if depth != 0 {
				return nil, 0, fmt.Errorf("line %d: unbalanced parentheses in safe navigation method call", lineNum)
			}

			// Extract arguments
			argsStr := line[argsStart : i-1]
			args := parseMethodArgs(argsStr)

			chain = append(chain, ChainElement{
				Name:     name,
				IsMethod: true,
				Args:     args,
				RawArgs:  argsStr,
			})
		} else {
			// Property access
			chain = append(chain, ChainElement{
				Name:     name,
				IsMethod: false,
				Args:     nil,
				RawArgs:  "",
			})
		}

		// Check if another ?. follows
		if i+1 < len(line) && line[i] == '?' && line[i+1] == '.' {
			i += 2 // Skip ?.
			continue
		} else {
			// End of chain
			break
		}
	}

	return chain, i, nil
}

// generateSafeNavCodeWithMarker generates safe navigation code with marker inserted for metadata tracking
func (s *SafeNavASTProcessor) generateSafeNavCodeWithMarker(base string, chain []ChainElement, baseType TypeKind, marker string, originalLine int, outputLine int) (string, error) {
	// Generate the code based on type
	code, err := s.generateSafeNavCode(base, chain, baseType, originalLine, outputLine)
	if err != nil {
		return "", err
	}

	// For if-statement pattern, insert marker after var declaration line
	// Pattern: "var opt Type\n" → "var opt Type " + marker + "\n"
	varIdx := strings.Index(code, "\n")
	if varIdx != -1 && strings.HasPrefix(code, "var opt") {
		// Insert marker at end of var declaration line
		before := code[:varIdx]     // var declaration without newline
		after := code[varIdx:]      // \n and rest
		code = before + " " + marker + after
	} else {
		// Fallback: prepend marker as comment line
		code = marker + "\n" + code
	}

	return code, nil
}

// generateSafeNavCode generates the expanded safe navigation code
func (s *SafeNavASTProcessor) generateSafeNavCode(base string, elements []ChainElement, baseType TypeKind, originalLine int, outputLine int) (string, error) {
	// Determine mode based on base type
	switch baseType {
	case TypeOption:
		return s.generateOptionMode(base, elements, originalLine, outputLine)
	case TypePointer:
		return s.generatePointerMode(base, elements, originalLine, outputLine)
	case TypeUnknown:
		// Generate compile error for unknown type (cannot infer)
		// Clear error with contextual help and multiple suggestions
		return "", fmt.Errorf(
			"line %d: cannot infer type for '%s' in safe navigation expression\n\n"+
				"  The type of '%s' could not be determined automatically.\n"+
				"  This may be due to:\n"+
				"    • Type inference limitations (both go/types and regex fallback failed)\n"+
				"    • Missing imports\n"+
				"    • Non-nullable base type\n\n"+
				"  Help: Add explicit type annotation\n\n"+
				"    For Option types:\n"+
				"      let %s: UserOption = getUser()\n"+
				"      let %s: Option[User] = getUser()\n\n"+
				"    For pointer types:\n"+
				"      let %s: *User = getUser()\n"+
				"      var %s *User = getUser()\n\n"+
				"  Note: Safe navigation (?.) requires Option<T> or pointer type (*T)\n"+
				"  Note: Ensure imports are correct and types are visible in current scope",
			originalLine, base, base, base, base, base, base)
	case TypeRegular:
		// Error: cannot use ?. on non-nullable type
		return "", fmt.Errorf(
			"line %d: safe navigation requires nullable type\n\n"+
				"  Variable '%s' has non-nullable type\n"+
				"  Safe navigation (?.) cannot be used with non-nullable values\n\n"+
				"  Help: Choose one of these options\n\n"+
				"    1. Wrap in Option type:\n"+
				"       let %s: UserOption = Some(value)\n\n"+
				"    2. Use pointer type:\n"+
				"       let %s: *User = &value\n\n"+
				"    3. Use regular access (if value is guaranteed non-nil):\n"+
				"       %s.profile.name\n\n"+
				"  Note: If '%s' should be nullable, add explicit type annotation",
			originalLine, base, base, base, base, base)
	}

	return "", nil
}

// generateOptionMode generates safe navigation for Option<T> types using if-statements
func (s *SafeNavASTProcessor) generateOptionMode(base string, elements []ChainElement, originalLine int, outputLine int) (string, error) {
	var buf bytes.Buffer

	// Generate temp variable name (opt, opt1, opt2, ...)
	optVar := s.getOptVarName()

	// Declare var with __INFER__ type placeholder
	buf.WriteString(fmt.Sprintf("var %s __INFER__\n", optVar))

	// Generate if-statement with proper nesting for chains
	s.generateOptionChain(&buf, base, elements, optVar, 0)

	return buf.String(), nil
}

// generateOptionChain recursively generates nested if-statements for chained safe navigation
func (s *SafeNavASTProcessor) generateOptionChain(buf *bytes.Buffer, currentVar string, elements []ChainElement, optVar string, depth int) {
	indent := strings.Repeat("\t", depth)

	if len(elements) == 0 {
		return
	}

	// Track the current type through the chain for proper wrapping
	currentType := ""
	if varType, ok := s.typeDetector.varTypes[currentVar]; ok {
		currentType = varType
	}

	elem := elements[0]

	// Check if current value is Some
	buf.WriteString(fmt.Sprintf("%sif %s.IsSome() {\n", indent, currentVar))

	// Unwrap to get the value
	tmpVar := s.getTmpVarName()
	buf.WriteString(fmt.Sprintf("%s\t%s := %s.Unwrap()\n", indent, tmpVar, currentVar))

	if len(elements) == 1 {
		// Last element - assign to optVar
		var valueExpr string
		if elem.IsMethod {
			valueExpr = fmt.Sprintf("%s.%s(%s)", tmpVar, elem.Name, elem.RawArgs)
		} else {
			valueExpr = fmt.Sprintf("%s.%s", tmpVar, elem.Name)
		}

		// Conditional wrapping based on actual field type
		fieldType := ""
		if currentType != "" {
			structType := strings.TrimSuffix(currentType, "Option")
			fieldKey := structType + "." + elem.Name
			if ft, ok := s.typeDetector.fieldTypes[fieldKey]; ok {
				fieldType = ft
			}
		}

		isAlreadyOption := strings.HasSuffix(fieldType, "Option")

		if isAlreadyOption {
			// Already an Option type, assign as-is
			buf.WriteString(fmt.Sprintf("%s\t%s = %s\n", indent, optVar, valueExpr))
		} else {
			// Plain type, wrap in Some()
			buf.WriteString(fmt.Sprintf("%s\t%s = __INFER__Some(%s)\n", indent, optVar, valueExpr))
		}
	} else {
		// Not last - recurse for next element
		var nextVar string
		if elem.IsMethod {
			nextVar = fmt.Sprintf("%s.%s(%s)", tmpVar, elem.Name, elem.RawArgs)
		} else {
			nextVar = fmt.Sprintf("%s.%s", tmpVar, elem.Name)
		}

		// Update currentType for recursion
		if currentType != "" {
			structType := strings.TrimSuffix(currentType, "Option")
			fieldKey := structType + "." + elem.Name
			if ft, ok := s.typeDetector.fieldTypes[fieldKey]; ok {
				currentType = ft
			}
		}

		s.generateOptionChain(buf, nextVar, elements[1:], optVar, depth+1)
	}

	// Else clause - assign None
	buf.WriteString(fmt.Sprintf("%s} else {\n", indent))
	buf.WriteString(fmt.Sprintf("%s\t%s = __INFER__None()\n", indent, optVar))
	buf.WriteString(fmt.Sprintf("%s}\n", indent))
}

// getOptVarName returns the next opt variable name (opt, opt1, opt2, ...)
func (s *SafeNavASTProcessor) getOptVarName() string {
	if s.optCounter == 1 {
		s.optCounter++
		return "opt"
	}
	name := fmt.Sprintf("opt%d", s.optCounter-1)
	s.optCounter++
	return name
}

// getTmpVarName returns a temp variable name for unwrapping (tmp, tmp1, tmp2, ...)
func (s *SafeNavASTProcessor) getTmpVarName() string {
	if s.tmpCounter == 1 {
		s.tmpCounter++
		return "tmp"
	}
	name := fmt.Sprintf("tmp%d", s.tmpCounter-1)
	s.tmpCounter++
	return name
}

// generatePointerMode generates safe navigation for pointer types using if-statements
func (s *SafeNavASTProcessor) generatePointerMode(base string, elements []ChainElement, originalLine int, outputLine int) (string, error) {
	var buf bytes.Buffer

	// Generate temp variable name (opt, opt1, opt2, ...)
	optVar := s.getOptVarName()

	// Declare var with __INFER__ type placeholder
	buf.WriteString(fmt.Sprintf("var %s __INFER__\n", optVar))

	// Generate if-statement with proper nesting for chains
	s.generatePointerChain(&buf, base, elements, optVar, 0)

	return buf.String(), nil
}

// generatePointerChain recursively generates nested if-statements for chained pointer safe navigation
func (s *SafeNavASTProcessor) generatePointerChain(buf *bytes.Buffer, currentVar string, elements []ChainElement, optVar string, depth int) {
	indent := strings.Repeat("\t", depth)

	if len(elements) == 0 {
		return
	}

	elem := elements[0]

	// Check if current value is not nil
	buf.WriteString(fmt.Sprintf("%sif %s != nil {\n", indent, currentVar))

	if len(elements) == 1 {
		// Last element - assign to optVar
		var valueExpr string
		if elem.IsMethod {
			valueExpr = fmt.Sprintf("%s.%s(%s)", currentVar, elem.Name, elem.RawArgs)
		} else {
			valueExpr = fmt.Sprintf("%s.%s", currentVar, elem.Name)
		}
		buf.WriteString(fmt.Sprintf("%s\t%s = %s\n", indent, optVar, valueExpr))
	} else {
		// Not last - create intermediate variable and recurse
		tmpVar := s.getTmpVarName()
		var valueExpr string
		if elem.IsMethod {
			valueExpr = fmt.Sprintf("%s.%s(%s)", currentVar, elem.Name, elem.RawArgs)
		} else {
			valueExpr = fmt.Sprintf("%s.%s", currentVar, elem.Name)
		}
		buf.WriteString(fmt.Sprintf("%s\t%s := %s\n", indent, tmpVar, valueExpr))

		s.generatePointerChain(buf, tmpVar, elements[1:], optVar, depth+1)
	}

	// Else clause - assign nil
	buf.WriteString(fmt.Sprintf("%s} else {\n", indent))
	buf.WriteString(fmt.Sprintf("%s\t%s = nil\n", indent, optVar))
	buf.WriteString(fmt.Sprintf("%s}\n", indent))
}
