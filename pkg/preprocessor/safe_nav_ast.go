package preprocessor

// AST-based safe navigation preprocessor
// Migrated from regex-based safe_nav.go
// See: ai-docs/sessions/20251130-234929-ast-migration/02-implementation/task-safe_nav-*.md

import (
	"bytes"
	"fmt"
	"go/token"
	"strings"

	"github.com/MadAppGang/dingo/pkg/ast"
)

// SafeNavASTProcessor handles the ?. operator using AST-based parsing
// Transforms: user?.address?.city → null-safe chain with Option/pointer checks
// Supports both Option<T> types and raw Go pointers (*T)
type SafeNavASTProcessor struct {
	typeDetector *TypeDetector
	tmpCounter   int
}

// NewSafeNavASTProcessor creates a new AST-based safe navigation processor
func NewSafeNavASTProcessor() *SafeNavASTProcessor {
	return &SafeNavASTProcessor{
		typeDetector: NewTypeDetector(),
		tmpCounter:   1,
	}
}

// Name returns the processor name
func (s *SafeNavASTProcessor) Name() string {
	return "safe_navigation_ast"
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

	// Reset state
	s.tmpCounter = 1

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

	// Transform line with all safe nav expressions
	result := line
	var meta *TransformMetadata
	firstTransform := true

	// Process in reverse order to maintain string positions
	for i := len(exprs) - 1; i >= 0; i-- {
		exprPos := exprs[i]

		// Detect base type
		baseType := s.typeDetector.DetectType(exprPos.expr.Receiver)

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
				return nil, fmt.Errorf("line %d: safe navigation ?. without receiver at position %d", lineNum, i)
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
					"line %d: trailing safe navigation operator without property: %s?.\n"+
						"  Help: Safe navigation (?.) requires a property or method after it\n"+
						"  Example: %s?.name or %s?.getName()\n"+
						"  Note: Did you mean error propagation (?) instead of safe navigation (?.)?",
					lineNum, base, base, base)
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

	// For IIFE pattern (Option/Pointer mode), insert marker after opening brace
	// Pattern: "func() __INFER__ {\n" → "func() __INFER__ { " + marker + "\n"
	braceIdx := strings.Index(code, "{\n")
	if braceIdx != -1 {
		// Insert marker after the brace, before the newline
		before := code[:braceIdx+1] // Include the {
		after := code[braceIdx+1:]  // Include the \n and rest
		code = before + " " + marker + after
	} else {
		// For placeholder pattern, prepend marker as comment line
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
		return "", fmt.Errorf(
			"line %d: cannot infer type for safe navigation on '%s'\n"+
				"  Help: Add explicit type annotation to enable type inference\n"+
				"  Example: var %s: UserOption = getUser()\n"+
				"  Example: var %s: *User = getUser()\n"+
				"  Note: Safe navigation requires Option<T> or pointer type (*T)",
			originalLine, base, base, base)
	case TypeRegular:
		// Error: cannot use ?. on non-nullable type
		return "", fmt.Errorf(
			"line %d: safe navigation requires nullable type\n"+
				"  Variable '%s' is not Option<T> or pointer type (*T)\n"+
				"  Help: Use Option<T> for nullable values, or use pointer type (*T)\n"+
				"  Note: If this is a pointer/Option, ensure type annotation is explicit",
			originalLine, base)
	}

	return "", nil
}

// generateOptionMode generates safe navigation for Option<T> types
func (s *SafeNavASTProcessor) generateOptionMode(base string, elements []ChainElement, originalLine int, outputLine int) (string, error) {
	var buf bytes.Buffer

	// Generate IIFE for safe navigation
	buf.WriteString("func() __INFER__ {\n")

	currentVar := base

	// Track the current type through the chain for proper wrapping
	currentType := ""
	if varType, ok := s.typeDetector.varTypes[base]; ok {
		currentType = varType
	}

	for i, elem := range elements {
		// Check if current value is None
		buf.WriteString(fmt.Sprintf("\tif %s.IsNone() {\n", currentVar))
		buf.WriteString("\t\treturn __INFER__None()\n")
		buf.WriteString("\t}\n")

		// Unwrap to get the value
		// No-number-first pattern
		tmpVar := ""
		if s.tmpCounter == 1 {
			tmpVar = base
		} else {
			tmpVar = fmt.Sprintf("%s%d", base, s.tmpCounter-1)
		}
		s.tmpCounter++
		buf.WriteString(fmt.Sprintf("\t%s := %s.Unwrap()\n", tmpVar, currentVar))

		// If last element, return it
		if i == len(elements)-1 {
			// Determine the return expression
			returnExpr := ""
			if elem.IsMethod {
				// Method call: call with arguments
				returnExpr = fmt.Sprintf("%s.%s(%s)", tmpVar, elem.Name, elem.RawArgs)
			} else {
				// Property access
				returnExpr = fmt.Sprintf("%s.%s", tmpVar, elem.Name)
			}

			// Conditional wrapping based on actual field type
			fieldType := ""
			if currentType != "" {
				// Strip "Option" suffix to get struct type
				structType := strings.TrimSuffix(currentType, "Option")
				// Build field key: "StructType.fieldName"
				fieldKey := structType + "." + elem.Name
				// Look up in TypeDetector
				if ft, ok := s.typeDetector.fieldTypes[fieldKey]; ok {
					fieldType = ft
				}
			}

			// Check if field type ends with "Option" (already an Option type)
			isAlreadyOption := strings.HasSuffix(fieldType, "Option")

			if isAlreadyOption {
				// Already an Option type, return as-is (no double-wrapping)
				buf.WriteString(fmt.Sprintf("\treturn %s\n", returnExpr))
			} else {
				// Plain type or unknown, wrap in Some()
				buf.WriteString(fmt.Sprintf("\treturn __INFER__Some(%s)\n", returnExpr))
			}
		} else {
			// Not last - prepare for next iteration
			if elem.IsMethod {
				// Method call: assign result to currentVar for next iteration
				currentVar = fmt.Sprintf("%s.%s(%s)", tmpVar, elem.Name, elem.RawArgs)
			} else {
				// Property access
				currentVar = fmt.Sprintf("%s.%s", tmpVar, elem.Name)
			}

			// Update currentType for next iteration
			if currentType != "" {
				structType := strings.TrimSuffix(currentType, "Option")
				fieldKey := structType + "." + elem.Name
				if ft, ok := s.typeDetector.fieldTypes[fieldKey]; ok {
					currentType = ft
				}
			}
		}
	}

	buf.WriteString("}()")

	return buf.String(), nil
}

// generatePointerMode generates safe navigation for pointer types
func (s *SafeNavASTProcessor) generatePointerMode(base string, elements []ChainElement, originalLine int, outputLine int) (string, error) {
	var buf bytes.Buffer

	// Generate IIFE for safe navigation
	buf.WriteString("func() __INFER__ {\n")

	currentVar := base

	for i, elem := range elements {
		// Check if current value is nil
		buf.WriteString(fmt.Sprintf("\tif %s == nil {\n", currentVar))
		buf.WriteString("\t\treturn nil\n")
		buf.WriteString("\t}\n")

		// Access the property or method
		if i < len(elements)-1 {
			// Not the last element - create intermediate variable to check next nil
			// CamelCase pattern without underscores
			var tmpVar string
			if i == 0 {
				tmpVar = base + "Tmp"
			} else {
				tmpVar = fmt.Sprintf("%sTmp%d", base, i)
			}
			if elem.IsMethod {
				buf.WriteString(fmt.Sprintf("\t%s := %s.%s(%s)\n", tmpVar, currentVar, elem.Name, elem.RawArgs))
			} else {
				buf.WriteString(fmt.Sprintf("\t%s := %s.%s\n", tmpVar, currentVar, elem.Name))
			}
			currentVar = tmpVar
		} else {
			// Last element - return it
			if elem.IsMethod {
				buf.WriteString(fmt.Sprintf("\treturn %s.%s(%s)\n", currentVar, elem.Name, elem.RawArgs))
			} else {
				buf.WriteString(fmt.Sprintf("\treturn %s.%s\n", currentVar, elem.Name))
			}
		}
	}

	buf.WriteString("}()")

	return buf.String(), nil
}
