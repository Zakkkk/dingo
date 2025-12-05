package preprocessor

import (
	"bytes"
	"fmt"
	"go/token"
	"strings"

	"github.com/MadAppGang/dingo/pkg/ast"
)

// NullCoalesceASTProcessor handles the ?? operator using AST-based parsing
// Migrated from regex-based null_coalesce.go to proper AST parsing
// See: ai-docs/sessions/20251130-234929-ast-migration/
type NullCoalesceASTProcessor struct {
	typeDetector *TypeDetector
	tmpCounter   int
}

// NewNullCoalesceASTProcessor creates a new AST-based null coalescing preprocessor
func NewNullCoalesceASTProcessor() *NullCoalesceASTProcessor {
	return &NullCoalesceASTProcessor{
		typeDetector: NewTypeDetector(),
		tmpCounter:   1,
	}
}

// Name implements BodyProcessor interface
func (n *NullCoalesceASTProcessor) Name() string {
	return "NullCoalesce"
}

// ProcessBody implements BodyProcessor interface for lambda body processing
func (n *NullCoalesceASTProcessor) ProcessBody(body []byte) ([]byte, error) {
	result, _, err := n.Process(body)
	return result, err
}

// Process implements FeatureProcessor interface for backward compatibility
func (n *NullCoalesceASTProcessor) Process(source []byte) ([]byte, []Mapping, error) {
	result, err := n.ProcessV2(source)
	if err != nil {
		return nil, nil, err
	}
	return result.Source, result.Mappings, nil
}

// ProcessV2 implements FeatureProcessorV2 interface with metadata support
func (n *NullCoalesceASTProcessor) ProcessV2(source []byte) (ProcessResult, error) {
	transformed, metadata, err := n.ProcessInternal(string(source))
	if err != nil {
		return ProcessResult{}, err
	}
	return ProcessResult{
		Source:   []byte(transformed),
		Mappings: nil,
		Metadata: metadata,
	}, nil
}

// ProcessInternal transforms null coalescing operators with metadata emission
func (n *NullCoalesceASTProcessor) ProcessInternal(code string) (string, []TransformMetadata, error) {
	// Parse source for type detection
	n.typeDetector.ParseSource([]byte(code))

	// Reset state
	n.tmpCounter = 1

	var metadata []TransformMetadata
	markerCounter := 0

	lines := strings.Split(code, "\n")
	var output bytes.Buffer

	inputLineNum := 0
	outputLineNum := 1

	for inputLineNum < len(lines) {
		line := lines[inputLineNum]

		// Process the line with metadata
		transformed, meta, err := n.processLine(line, inputLineNum+1, &markerCounter)
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

// processLine processes a single line for null coalescing operators
// Supports multiple ?? operators on the same line (separated by semicolons)
func (n *NullCoalesceASTProcessor) processLine(line string, lineNum int, markerCounter *int) (string, *TransformMetadata, error) {
	// Quick check: if no ?? in line, skip parsing
	if !strings.Contains(line, "??") {
		return line, nil, nil
	}

	// Split line by semicolons (outside strings/comments) for multiple statements
	statements := n.splitStatements(line)

	// If only one statement, process normally
	if len(statements) == 1 {
		return n.processStatement(line, lineNum, markerCounter)
	}

	// Multiple statements - process each independently
	var results []string
	var firstMeta *TransformMetadata

	for i, stmt := range statements {
		transformed, meta, err := n.processStatement(stmt, lineNum, markerCounter)
		if err != nil {
			return "", nil, err
		}
		results = append(results, transformed)

		// Keep only first metadata (for source map tracking)
		if i == 0 && meta != nil {
			firstMeta = meta
		}
	}

	// Join statements with semicolons
	return strings.Join(results, "; "), firstMeta, nil
}

// processStatement processes a single statement (no semicolons)
func (n *NullCoalesceASTProcessor) processStatement(stmt string, lineNum int, markerCounter *int) (string, *TransformMetadata, error) {
	// Quick check: if no ?? in statement, skip parsing
	if !strings.Contains(stmt, "??") {
		return stmt, nil, nil
	}

	// Parse the statement for ?? operators
	expr, err := n.parseNullCoalesce(stmt)
	if err != nil {
		// If parsing fails, statement doesn't contain valid ?? operator
		return stmt, nil, nil
	}

	if expr == nil {
		// No ?? operator found
		return stmt, nil, nil
	}

	// Check if this is a let assignment: let varName = expr ?? default
	letMatch, varName, indent := n.extractLetPattern(stmt)

	marker := fmt.Sprintf("// dingo:c:%d", *markerCounter)
	*markerCounter++

	var transformed string
	var meta *TransformMetadata

	if letMatch {
		// Generate if-else block with actual variable name
		transformed, meta = n.generateLetAssignment(expr, varName, indent, marker, lineNum, markerCounter)
	} else {
		// Generate inline if-else block (for expressions, function args, etc.)
		transformed, meta = n.generateInlineExpression(expr, marker, lineNum, stmt)
	}

	return transformed, meta, nil
}

// splitStatements splits a line by semicolons, respecting strings and comments
// Returns slice of statements (semicolons removed)
func (n *NullCoalesceASTProcessor) splitStatements(line string) []string {
	// Quick check: if no semicolon, return line as-is
	if !strings.Contains(line, ";") {
		return []string{line}
	}

	var statements []string
	var current strings.Builder
	inString := false
	stringChar := byte(0)
	inComment := false

	for i := 0; i < len(line); i++ {
		ch := line[i]

		// Check for comment start
		if !inString && !inComment && i < len(line)-1 && ch == '/' && line[i+1] == '/' {
			inComment = true
			current.WriteByte(ch)
			continue
		}

		// Track string state
		if !inComment && !inString {
			if ch == '"' || ch == '\'' || ch == '`' {
				inString = true
				stringChar = ch
			}
		} else if !inComment && inString {
			// Check for closing quote
			if ch == stringChar {
				// Count consecutive backslashes before quote
				escapeCount := 0
				for j := i - 1; j >= 0 && line[j] == '\\'; j-- {
					escapeCount++
				}
				// Quote is escaped only if ODD number of backslashes
				if escapeCount%2 == 0 {
					inString = false
				}
			}
		}

		// Split on semicolon (only outside strings and comments)
		if !inString && !inComment && ch == ';' {
			// Save current statement
			stmt := strings.TrimSpace(current.String())
			if stmt != "" {
				statements = append(statements, stmt)
			}
			current.Reset()
			continue
		}

		current.WriteByte(ch)
	}

	// Add final statement
	stmt := strings.TrimSpace(current.String())
	if stmt != "" {
		statements = append(statements, stmt)
	}

	// If no statements collected, return original line
	if len(statements) == 0 {
		return []string{line}
	}

	return statements
}

// parseNullCoalesce parses a line for ?? operators using token-based scanning
// Returns the parsed expression or nil if no ?? operator found
func (n *NullCoalesceASTProcessor) parseNullCoalesce(line string) (*ast.NullCoalesceExpr, error) {
	// Find comment start to exclude it from parsing
	commentStart := findCommentStart(line)
	searchLine := line
	if commentStart != -1 {
		searchLine = line[:commentStart]
	}

	// Find all ?? positions using token scanner
	positions := n.findCoalesceOperators(searchLine)
	if len(positions) == 0 {
		return nil, nil
	}

	// Build expression tree (right-associative: a ?? b ?? c = a ?? (b ?? c))
	// For simplicity, we'll handle chains by collecting all operands
	var operands []string

	// Check if this is a let/var declaration or short declaration and extract RHS
	rhsStart := 0
	trimmed := strings.TrimSpace(searchLine)
	if strings.HasPrefix(trimmed, "let ") || strings.HasPrefix(trimmed, "var ") {
		// Find the = sign
		eqIndex := strings.Index(searchLine, "=")
		if eqIndex != -1 {
			// Start parsing from after the =
			rhsStart = eqIndex + 1
		}
	} else if strings.Contains(trimmed, ":=") {
		// Short declaration: varName := expr ?? default
		colonEqIndex := strings.Index(searchLine, ":=")
		if colonEqIndex != -1 {
			// Start parsing from after the :=
			rhsStart = colonEqIndex + 2
		}
	}

	// Extract operands between ?? operators
	// Note: For raw Dingo syntax (user?.name ?? default), use simple extraction
	// IIFEDetector is available (n.iifeDetector) for when SafeNav has already
	// transformed ?.  into IIFE patterns, which can then be passed to NullCoalesce
	// via ProcessBody() when processing lambda bodies.
	lastEnd := rhsStart
	for i, pos := range positions {
		if i == 0 {
			// Extract left operand (from start to first ??)
			left := strings.TrimSpace(searchLine[lastEnd:pos.start])
			if left == "" {
				return nil, fmt.Errorf("empty left operand before ??")
			}
			operands = append(operands, left)
		}

		// Extract right operand (from ?? to next ?? or end)
		if i < len(positions)-1 {
			// There's another ?? ahead
			right := strings.TrimSpace(searchLine[pos.end:positions[i+1].start])
			if right == "" {
				return nil, fmt.Errorf("empty operand between ?? operators")
			}
			operands = append(operands, right)
		} else {
			// Last ?? - extract to end of line
			right := strings.TrimSpace(searchLine[pos.end:])
			if right == "" {
				return nil, fmt.Errorf("empty right operand after ??")
			}
			operands = append(operands, right)
		}

		lastEnd = pos.end
	}

	// Create expression with all operands
	if len(operands) < 2 {
		return nil, fmt.Errorf("null coalesce requires at least 2 operands, got %d", len(operands))
	}
	expr := &ast.NullCoalesceExpr{
		LeftStr:  operands[0],
		OpPos:    token.Pos(positions[0].start),
		RightStr: operands[len(operands)-1],
	}

	// Store chain if more than 2 operands
	if len(operands) > 2 {
		// Create chain representation
		var chain []*ast.NullCoalesceExpr
		for i := 1; i < len(operands)-1; i++ {
			chain = append(chain, &ast.NullCoalesceExpr{
				LeftStr: operands[i],
				OpPos:   token.Pos(positions[i].start),
			})
		}
		expr.Chain = chain
	}

	return expr, nil
}

type operatorPos struct {
	start int
	end   int
}

// findCoalesceOperators finds all ?? operator positions in a line
func (n *NullCoalesceASTProcessor) findCoalesceOperators(line string) []operatorPos {
	var positions []operatorPos

	// Simple character-by-character scan for ?? operator
	// Skip strings and comments
	inString := false
	stringChar := byte(0)

	for i := 0; i < len(line)-1; i++ {
		ch := line[i]

		// Track string state
		if !inString {
			if ch == '"' || ch == '\'' || ch == '`' {
				inString = true
				stringChar = ch
			}
		} else {
			// Inside string - check for closing quote
			if ch == stringChar {
				// Count consecutive backslashes before quote
				escapeCount := 0
				for j := i - 1; j >= 0 && line[j] == '\\'; j-- {
					escapeCount++
				}
				// Quote is escaped only if ODD number of backslashes
				if escapeCount%2 == 1 {
					continue // Quote is escaped
				}
				inString = false
			}
			continue
		}

		// Look for ?? operator outside strings
		if !inString && ch == '?' && line[i+1] == '?' {
			positions = append(positions, operatorPos{
				start: i,
				end:   i + 2,
			})
			i++ // Skip next ?
		}
	}

	return positions
}

// extractLetPattern checks if line matches assignment pattern with ??
// Supports:
//   - "let varName = ..." (original Dingo)
//   - "var varName Type = ..." (after DingoPreParser with type)
//   - "varName := ..." (short declaration after DingoPreParser without type)
// Returns: (matched, varName, indent)
func (n *NullCoalesceASTProcessor) extractLetPattern(line string) (bool, string, string) {
	trimmed := strings.TrimSpace(line)

	// Extract indent
	indent := ""
	for i, ch := range line {
		if ch == ' ' || ch == '\t' {
			indent += string(ch)
		} else {
			if i > 0 {
				indent = line[:i]
			}
			break
		}
	}

	// Pattern 1: let varName = ... or var varName = ...
	if strings.HasPrefix(trimmed, "let ") || strings.HasPrefix(trimmed, "var ") {
		parts := strings.Fields(trimmed)
		if len(parts) < 4 {
			// Minimum: "let x = value"
			return false, "", ""
		}

		// Extract variable name (parts[1])
		varName := parts[1]

		// Check for type annotation: let x: Type = ...
		if strings.Contains(varName, ":") {
			// Remove type annotation
			varName = strings.Split(varName, ":")[0]
		}

		return true, varName, indent
	}

	// Pattern 2: varName := ... (short declaration)
	if strings.Contains(trimmed, ":=") {
		// Find := position
		colonIdx := strings.Index(trimmed, ":=")
		if colonIdx > 0 {
			// Extract variable name(s) before :=
			varPart := strings.TrimSpace(trimmed[:colonIdx])
			// Handle multiple variables: a, b := ...
			if strings.Contains(varPart, ",") {
				// Multiple vars - take first one
				varPart = strings.TrimSpace(strings.Split(varPart, ",")[0])
			}
			// Validate it looks like an identifier
			if len(varPart) > 0 && isValidIdent(varPart) {
				return true, varPart, indent
			}
		}
	}

	return false, "", ""
}

// isValidIdent checks if a string looks like a valid Go identifier
func isValidIdent(s string) bool {
	if len(s) == 0 {
		return false
	}
	// First char must be letter or underscore
	first := s[0]
	if !((first >= 'a' && first <= 'z') || (first >= 'A' && first <= 'Z') || first == '_') {
		return false
	}
	// Rest must be letter, digit, or underscore
	for i := 1; i < len(s); i++ {
		ch := s[i]
		if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_') {
			return false
		}
	}
	return true
}

// generateLetAssignment generates if-else block for let assignment
// Uses default-first pattern: x := defaultValue; if condition { x = actualValue }
func (n *NullCoalesceASTProcessor) generateLetAssignment(expr *ast.NullCoalesceExpr, varName string, indent string, marker string, lineNum int, markerCounter *int) (string, *TransformMetadata) {
	// Build operand chain
	operands := []string{expr.LeftStr}
	if expr.Chain != nil {
		for _, ch := range expr.Chain {
			operands = append(operands, ch.LeftStr)
		}
	}
	operands = append(operands, expr.RightStr)

	// Detect type of first operand
	leftType := n.typeDetector.DetectType(strings.TrimSpace(operands[0]))

	// Generate code
	var buf bytes.Buffer

	// Default-first pattern: Initialize with final default (last operand)
	lastOperand := strings.TrimSpace(operands[len(operands)-1])
	buf.WriteString(fmt.Sprintf("%s%s := %s\n", indent, varName, lastOperand))

	// Check if all operands are Options (determines unwrapping)
	allOptions := leftType == TypeOption
	if allOptions {
		lastType := n.typeDetector.DetectType(lastOperand)
		if lastType != TypeOption {
			allOptions = false
		}
	}

	// Generate if-else chain for checking operands
	for i := 0; i < len(operands)-1; i++ {
		operand := strings.TrimSpace(operands[i])

		// First element uses the provided marker
		currentMarker := marker
		if i > 0 {
			currentMarker = fmt.Sprintf("// dingo:c:%d", *markerCounter)
			*markerCounter++
		}

		// Generate condition based on type
		condition := n.generateCondition(operand, leftType)
		assignment := n.generateAssignment(varName, operand, leftType, allOptions)

		if i == 0 {
			// First check: if
			buf.WriteString(fmt.Sprintf("%sif val := %s; %s { %s\n", indent, operand, condition, currentMarker))
			buf.WriteString(fmt.Sprintf("%s\t%s\n", indent, assignment))
		} else {
			// Subsequent checks: else if
			buf.WriteString(fmt.Sprintf("%s} else if val := %s; %s { %s\n", indent, operand, condition, currentMarker))
			buf.WriteString(fmt.Sprintf("%s\t%s\n", indent, assignment))
		}
	}

	// Close if-else chain
	buf.WriteString(fmt.Sprintf("%s}", indent))

	// Create metadata
	meta := &TransformMetadata{
		Type:            "null_coalesce",
		OriginalLine:    lineNum,
		OriginalColumn:  int(expr.OpPos) + 1,
		OriginalLength:  2, // length of ??
		OriginalText:    "??",
		GeneratedMarker: marker,
		ASTNodeType:     "NullCoalesceExpr",
	}

	return buf.String(), meta
}

// generateInlineExpression generates IIFE pattern for inline usage
// Returns: transformed line and metadata
func (n *NullCoalesceASTProcessor) generateInlineExpression(expr *ast.NullCoalesceExpr, marker string, lineNum int, originalLine string) (string, *TransformMetadata) {
	// Operands are already strings in the AST
	leftOperand := strings.TrimSpace(expr.LeftStr)
	rightOperand := strings.TrimSpace(expr.RightStr)

	// Detect types for both operands
	leftType := n.typeDetector.DetectType(leftOperand)
	rightType := n.typeDetector.DetectType(rightOperand)

	// Determine if both operands are Option types
	allOptions := leftType == TypeOption && rightType == TypeOption

	// Generate IIFE pattern: func() T { ... }()
	// This allows inline usage while maintaining proper type handling
	var buf strings.Builder

	// Use __INFER__ placeholder for return type (will be inferred by plugin pipeline)
	buf.WriteString("func() __INFER__ {\n")

	// Check left operand
	switch leftType {
	case TypeOption:
		buf.WriteString(fmt.Sprintf("\t\tif val := %s; val.IsSome() {\n", leftOperand))
		if allOptions {
			// Option ?? Option → return as-is
			buf.WriteString("\t\t\treturn val\n")
		} else {
			// Option ?? Primitive → unwrap
			buf.WriteString("\t\t\treturn val.Unwrap()\n")
		}
		buf.WriteString("\t\t}\n")
	case TypePointer:
		buf.WriteString(fmt.Sprintf("\t\tif val := %s; val != nil {\n", leftOperand))
		buf.WriteString("\t\t\treturn val\n")
		buf.WriteString("\t\t}\n")
	default:
		// Assume Option type
		buf.WriteString(fmt.Sprintf("\t\tif val := %s; val.IsSome() {\n", leftOperand))
		buf.WriteString("\t\t\treturn val.Unwrap()\n")
		buf.WriteString("\t\t}\n")
	}

	// Return right operand as fallback
	buf.WriteString(fmt.Sprintf("\t\treturn %s %s\n", rightOperand, marker))
	buf.WriteString("\t}()")

	// Create metadata
	meta := &TransformMetadata{
		Type:            "null_coalesce_inline",
		OriginalLine:    lineNum,
		OriginalColumn:  int(expr.OpPos) + 1,
		OriginalLength:  2, // length of ??
		OriginalText:    "??",
		GeneratedMarker: marker,
		ASTNodeType:     "NullCoalesceExpr",
	}

	return buf.String(), meta
}

// generateCondition generates the condition expression for a given type
func (n *NullCoalesceASTProcessor) generateCondition(operand string, typeKind TypeKind) string {
	switch typeKind {
	case TypeOption:
		return "val.IsSome()"
	case TypePointer:
		return "val != nil"
	case TypeUnknown, TypeRegular:
		// Assume Option type
		return "val.IsSome()"
	default:
		return "val != nil"
	}
}

// generateAssignment generates the assignment statement for a given type
func (n *NullCoalesceASTProcessor) generateAssignment(varName string, operand string, typeKind TypeKind, allOptions bool) string {
	switch typeKind {
	case TypeOption:
		if allOptions {
			// Option ?? Option → no unwrap
			return fmt.Sprintf("%s = val", varName)
		}
		// Option ?? Primitive → unwrap
		return fmt.Sprintf("%s = val.Unwrap()", varName)

	case TypePointer:
		return fmt.Sprintf("%s = *val", varName)

	case TypeUnknown, TypeRegular:
		if allOptions {
			return fmt.Sprintf("%s = val", varName)
		}
		return fmt.Sprintf("%s = val.Unwrap()", varName)

	default:
		return fmt.Sprintf("%s = val", varName)
	}
}
