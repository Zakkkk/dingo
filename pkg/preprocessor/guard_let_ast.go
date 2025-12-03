package preprocessor

import (
	"bytes"
	"fmt"
	"strings"
	"unicode"
)

// GuardLetASTProcessor handles guard let statements using AST-based parsing.
// Uses TypeRegistry to determine if expression is Result or Option.
//
// Syntax:
//   guard let <variable> = <result_or_option_expr> else { <action> }
//   guard let (<vars>) = <expr> else { <action> }
//
// Examples:
//   guard let user = GetUser(id) else { return err }
//   guard let (user, profile) = GetUserWithProfile(id) else { return err }
//   guard let config = maybeConfig else { return "no config" }
type GuardLetASTProcessor struct {
	registry            TypeRegistry
	counter             int
	functionReturnTypes map[string]string // funcName -> returnType
}

// NewGuardLetASTProcessor creates a new guard let processor
func NewGuardLetASTProcessor() *GuardLetASTProcessor {
	return &GuardLetASTProcessor{
		counter:           1,
		functionReturnTypes: make(map[string]string),
	}
}

// Name implements BodyProcessor interface
func (p *GuardLetASTProcessor) Name() string {
	return "GuardLetAST"
}

// SetRegistry sets the TypeRegistry for type detection
func (p *GuardLetASTProcessor) SetRegistry(r interface{}) {
	if tr, ok := r.(TypeRegistry); ok {
		p.registry = tr
	}
}

// GuardLetMatch represents a parsed guard let statement
type GuardLetMatch struct {
	VarNames      []string // Single var or tuple destructuring
	Expr          string   // Expression to unwrap
	ElseBody      string   // Body of else block
	ExprType      ExprType // Result, Option, or Unknown
	IsInline      bool     // Single-line syntax
	Indent        string   // Leading whitespace
	Line          int      // For source mapping
	ConsumedLines int      // Number of lines consumed (for multiline)
}

// ExprType indicates the monadic type
type ExprType int

const (
	ExprTypeUnknown ExprType = iota
	ExprTypeResult
	ExprTypeOption
)

// Process implements FeatureProcessor interface
func (p *GuardLetASTProcessor) Process(source []byte) ([]byte, []Mapping, error) {
	result, _, err := p.ProcessInternal(string(source))
	return []byte(result), nil, err
}

// ProcessInternal processes the code and returns transformations with metadata
func (p *GuardLetASTProcessor) ProcessInternal(code string) (string, []TransformMetadata, error) {
	// First pass: scan for function signatures to build return type map
	p.scanFunctionSignatures(code)

	lines := strings.Split(code, "\n")
	var output bytes.Buffer
	var metadata []TransformMetadata
	markerCounter := 0

	i := 0
	for i < len(lines) {
		line := lines[i]

		match := p.parseGuardLet(line, lines, i)
		if match != nil {
			// Determine expression type from registry
			match.ExprType = p.inferExpressionType(match.Expr)

			// Generate transformation
			transformed, meta := p.generateTransform(match, &markerCounter)
			output.WriteString(transformed)

			if meta != nil {
				metadata = append(metadata, *meta)
			}

			// Skip consumed lines (multiline else block)
			i += match.ConsumedLines
		} else {
			output.WriteString(line)
			if i < len(lines)-1 {
				output.WriteByte('\n')
			}
			i++
		}
	}

	return output.String(), metadata, nil
}

// parseGuardLet attempts to parse a guard let statement
func (p *GuardLetASTProcessor) parseGuardLet(line string, lines []string, lineIdx int) *GuardLetMatch {
	trimmed := strings.TrimSpace(line)

	// Check for guard let keyword
	if !strings.HasPrefix(trimmed, "guard let ") {
		return nil
	}

	indent := getIndent(line)

	// Extract variable name(s) and expression
	// Format: guard let <var> = <expr> else { <body> }
	//     or: guard let (<vars>) = <expr> else { <body> }

	afterGuard := strings.TrimPrefix(trimmed, "guard let ")

	// Find the = sign
	eqIdx := strings.Index(afterGuard, "=")
	if eqIdx == -1 {
		return nil
	}

	varPart := strings.TrimSpace(afterGuard[:eqIdx])
	afterEq := strings.TrimSpace(afterGuard[eqIdx+1:])

	// Parse variable names (single or tuple)
	varNames := p.parseVarNames(varPart)
	if len(varNames) == 0 {
		return nil
	}

	// Find 'else' keyword
	elseIdx := strings.Index(afterEq, " else ")
	if elseIdx == -1 {
		return nil
	}

	expr := strings.TrimSpace(afterEq[:elseIdx])
	afterElse := strings.TrimSpace(afterEq[elseIdx+6:]) // skip " else "

	// Parse else block (inline or multiline)
	elseBody, consumedLines, isInline := p.parseElseBlock(afterElse, lines, lineIdx)

	return &GuardLetMatch{
		VarNames:      varNames,
		Expr:          expr,
		ElseBody:      elseBody,
		IsInline:      isInline,
		Indent:        indent,
		Line:          lineIdx + 1,
		ConsumedLines: consumedLines,
	}
}

// parseVarNames parses variable names (single or tuple)
// Examples: "user" -> ["user"], "(user, profile)" -> ["user", "profile"]
func (p *GuardLetASTProcessor) parseVarNames(varPart string) []string {
	varPart = strings.TrimSpace(varPart)

	// Check for tuple syntax
	if strings.HasPrefix(varPart, "(") && strings.HasSuffix(varPart, ")") {
		inner := varPart[1 : len(varPart)-1]
		parts := strings.Split(inner, ",")
		varNames := make([]string, 0, len(parts))
		for _, part := range parts {
			name := strings.TrimSpace(part)
			if name != "" {
				varNames = append(varNames, name)
			}
		}
		return varNames
	}

	// Single variable
	return []string{varPart}
}

// parseElseBlock parses the else block (inline or multiline)
func (p *GuardLetASTProcessor) parseElseBlock(afterElse string, lines []string, lineIdx int) (body string, consumed int, isInline bool) {
	// Check for inline syntax: else { <single-statement> }
	if strings.HasPrefix(afterElse, "{") {
		// Find closing brace
		if closeIdx := strings.Index(afterElse, "}"); closeIdx != -1 {
			// Inline: single line with { ... }
			inner := afterElse[1:closeIdx]
			return strings.TrimSpace(inner), 1, true
		}

		// Multiline: closing brace on next line(s)
		var bodyLines []string
		consumed = 1 // Current line

		// Skip opening brace line
		for i := lineIdx + 1; i < len(lines); i++ {
			line := lines[i]
			trimmed := strings.TrimSpace(line)

			if trimmed == "}" {
				consumed = i - lineIdx + 1
				break
			}

			bodyLines = append(bodyLines, strings.TrimSpace(line))
		}

		body = strings.Join(bodyLines, "\n")
		return body, consumed, false
	}

	return "", 1, false
}

// scanFunctionSignatures scans the source code for function signatures and builds return type map
// Example: "func FindUser(id int) Result {" -> functionReturnTypes["FindUser"] = "Result"
func (p *GuardLetASTProcessor) scanFunctionSignatures(code string) {
	lines := strings.Split(code, "\n")
	// Regex: func <name>(<params>) <returnType> {
	// We use simple string parsing instead of regex for better performance
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "func ") {
			continue
		}

		// Extract function name and return type
		// Format: func Name(params) ReturnType {
		afterFunc := strings.TrimPrefix(trimmed, "func ")

		// Find opening paren
		parenIdx := strings.Index(afterFunc, "(")
		if parenIdx == -1 {
			continue
		}

		funcName := strings.TrimSpace(afterFunc[:parenIdx])

		// Find closing paren
		closeParenIdx := strings.Index(afterFunc, ")")
		if closeParenIdx == -1 {
			continue
		}

		// Extract return type (between ) and {)
		afterParen := strings.TrimSpace(afterFunc[closeParenIdx+1:])
		if afterParen == "{" || afterParen == "" {
			// No return type
			continue
		}

		// Return type is the text before {
		braceIdx := strings.Index(afterParen, "{")
		var returnType string
		if braceIdx != -1 {
			returnType = strings.TrimSpace(afterParen[:braceIdx])
		} else {
			returnType = strings.TrimSpace(afterParen)
		}

		if returnType != "" {
			p.functionReturnTypes[funcName] = returnType
		}
	}
}

// inferExpressionType determines if expression is Result or Option
func (p *GuardLetASTProcessor) inferExpressionType(expr string) ExprType {
	// Check if expression is a function call
	funcName := extractFunctionName(expr)
	if funcName != "" {
		// Check our scanned function signatures first
		if returnType, ok := p.functionReturnTypes[funcName]; ok {
			return p.inferTypeFromReturnType(returnType)
		}

		// Check registry
		if p.registry != nil {
			if funcInfo, ok := p.registry.GetFunction(funcName); ok {
				if len(funcInfo.Results) > 0 {
					if funcInfo.Results[0].IsResult {
						return ExprTypeResult
					}
					if funcInfo.Results[0].IsOption {
						return ExprTypeOption
					}
				}
			}
		}
	}

	// Check if expression is a variable reference
	if p.registry != nil {
		varName := extractBaseVariable(expr)
		if varInfo, ok := p.registry.GetVariable(varName); ok {
			if varInfo.Type.IsResult {
				return ExprTypeResult
			}
			if varInfo.Type.IsOption {
				return ExprTypeOption
			}
		}
	}

	// Fallback: use naming convention
	return p.inferTypeByNaming(expr)
}

// inferTypeFromReturnType determines if a return type string is Result or Option
func (p *GuardLetASTProcessor) inferTypeFromReturnType(returnType string) ExprType {
	lower := strings.ToLower(returnType)

	// Result patterns
	if strings.Contains(lower, "result") {
		return ExprTypeResult
	}

	// Option patterns
	if strings.Contains(lower, "option") {
		return ExprTypeOption
	}

	return ExprTypeUnknown
}

// inferTypeByNaming uses naming conventions as fallback
func (p *GuardLetASTProcessor) inferTypeByNaming(expr string) ExprType {
	lower := strings.ToLower(expr)

	// Result patterns
	if strings.Contains(lower, "result") || strings.Contains(lower, "error") ||
	   strings.Contains(lower, "err") {
		return ExprTypeResult
	}

	// Option patterns
	if strings.Contains(lower, "option") || strings.Contains(lower, "maybe") ||
	   strings.HasPrefix(lower, "opt") {
		return ExprTypeOption
	}

	return ExprTypeUnknown
}

// generateTransform generates the transformation code
func (p *GuardLetASTProcessor) generateTransform(match *GuardLetMatch, markerCounter *int) (string, *TransformMetadata) {
	marker := fmt.Sprintf("// dingo:g:%d", *markerCounter)
	*markerCounter++

	switch match.ExprType {
	case ExprTypeResult:
		return p.generateResultGuard(match, marker)
	case ExprTypeOption:
		return p.generateOptionGuard(match, marker)
	default:
		// Unknown type - generate error comment
		return p.generateUnknownGuard(match), nil
	}
}

// generateResultGuard generates code for Result types
// Pattern:
//   tmp := expr (only if complex expression)
//   // dingo:g:N
//   if expr.IsErr() {
//       err := expr.UnwrapErr()
//       <else-body>
//   }
//   var := expr.Unwrap()
func (p *GuardLetASTProcessor) generateResultGuard(match *GuardLetMatch, marker string) (string, *TransformMetadata) {
	var buf bytes.Buffer
	indent := match.Indent

	// Generate temp variable for complex expressions, use direct for simple
	tmpVar := p.generateTempVar(match.Expr)
	needsTmp := tmpVar != ""

	exprVar := match.Expr
	if needsTmp {
		// tmp := expr
		buf.WriteString(fmt.Sprintf("%s%s := %s\n", indent, tmpVar, match.Expr))
		exprVar = tmpVar
	}

	// // dingo:g:N
	buf.WriteString(fmt.Sprintf("%s%s\n", indent, marker))

	// if expr.IsErr() {
	buf.WriteString(fmt.Sprintf("%sif %s.IsErr() {\n", indent, exprVar))

	// err := *expr.err
	buf.WriteString(fmt.Sprintf("%s\terr := *%s.err\n", indent, exprVar))

	// else body (with err available)
	// Transform "return err" to "return ResultErr(err)" for Result types
	elseBody := p.transformElseBody(match.ElseBody, match.ExprType)
	elseBody = strings.TrimSpace(elseBody)
	if match.IsInline {
		buf.WriteString(fmt.Sprintf("%s\t%s\n", indent, elseBody))
	} else {
		// Multiline: indent each line
		bodyLines := strings.Split(elseBody, "\n")
		for _, line := range bodyLines {
			buf.WriteString(fmt.Sprintf("%s\t%s\n", indent, line))
		}
	}

	// }
	buf.WriteString(fmt.Sprintf("%s}\n", indent))

	// Handle tuple destructuring
	if len(match.VarNames) > 1 {
		// Tuple case: (user, profile) := *tmp.ok
		buf.WriteString(fmt.Sprintf("%s%s := *%s.ok\n",
			indent,
			strings.Join(match.VarNames, ", "),
			exprVar))
	} else {
		// Simple case: user := *tmp.ok
		buf.WriteString(fmt.Sprintf("%s%s := *%s.ok\n", indent, match.VarNames[0], exprVar))
	}

	meta := &TransformMetadata{
		Type:            "guard_let",
		OriginalLine:    match.Line,
		GeneratedMarker: marker,
		ASTNodeType:     "IfStmt",
	}

	return buf.String(), meta
}

// generateOptionGuard generates code for Option types
// Pattern:
//   tmp := expr (only if complex expression)
//   // dingo:g:N
//   if expr.IsNone() {
//       <else-body>
//   }
//   var := expr.Unwrap()
func (p *GuardLetASTProcessor) generateOptionGuard(match *GuardLetMatch, marker string) (string, *TransformMetadata) {
	var buf bytes.Buffer
	indent := match.Indent

	tmpVar := p.generateTempVar(match.Expr)
	needsTmp := tmpVar != ""

	exprVar := match.Expr
	if needsTmp {
		buf.WriteString(fmt.Sprintf("%s%s := %s\n", indent, tmpVar, match.Expr))
		exprVar = tmpVar
	}

	buf.WriteString(fmt.Sprintf("%s%s\n", indent, marker))
	buf.WriteString(fmt.Sprintf("%sif %s.IsNone() {\n", indent, exprVar))

	// Note: No err binding for Option types
	elseBody := strings.TrimSpace(match.ElseBody)
	if match.IsInline {
		buf.WriteString(fmt.Sprintf("%s\t%s\n", indent, elseBody))
	} else {
		bodyLines := strings.Split(elseBody, "\n")
		for _, line := range bodyLines {
			buf.WriteString(fmt.Sprintf("%s\t%s\n", indent, line))
		}
	}

	buf.WriteString(fmt.Sprintf("%s}\n", indent))

	if len(match.VarNames) > 1 {
		buf.WriteString(fmt.Sprintf("%s%s := *%s.some\n",
			indent,
			strings.Join(match.VarNames, ", "),
			exprVar))
	} else {
		buf.WriteString(fmt.Sprintf("%s%s := *%s.some\n", indent, match.VarNames[0], exprVar))
	}

	meta := &TransformMetadata{
		Type:            "guard_let",
		OriginalLine:    match.Line,
		GeneratedMarker: marker,
		ASTNodeType:     "IfStmt",
	}

	return buf.String(), meta
}

// transformElseBody transforms the else block body for guard let
// For Result types: "return err" -> "return ResultErr(err)"
// For Option types: no transformation needed
func (p *GuardLetASTProcessor) transformElseBody(elseBody string, exprType ExprType) string {
	if exprType != ExprTypeResult {
		return elseBody
	}

	// Transform "return err" -> "return ResultErr(err)"
	// This is a simple heuristic - matches common pattern
	elseBody = strings.ReplaceAll(elseBody, "return err", "return ResultErr(err)")

	return elseBody
}

// generateUnknownGuard generates an error comment for unknown types
func (p *GuardLetASTProcessor) generateUnknownGuard(match *GuardLetMatch) string {
	return fmt.Sprintf("%s// ERROR: guard let could not determine type for: %s\n%s// Original: guard let %s = %s else { %s }",
		match.Indent,
		match.Expr,
		match.Indent,
		strings.Join(match.VarNames, ", "),
		match.Expr,
		match.ElseBody)
}

// generateTempVar returns a temp var name for complex expressions
// Returns empty string if expression is simple (identifier only)
func (p *GuardLetASTProcessor) generateTempVar(expr string) string {
	// Simple identifier check
	if isSimpleIdentifier(expr) {
		return ""
	}

	// Complex expression - generate temp var
	varName := ""
	if p.counter == 1 {
		varName = "tmp"
	} else {
		varName = fmt.Sprintf("tmp%d", p.counter-1)
	}
	p.counter++
	return varName
}

// isSimpleIdentifier checks if expression is a simple identifier
func isSimpleIdentifier(expr string) bool {
	expr = strings.TrimSpace(expr)
	if len(expr) == 0 {
		return false
	}

	// Check for function calls, field access, etc.
	for _, ch := range expr {
		if !unicode.IsLetter(ch) && !unicode.IsDigit(ch) && ch != '_' {
			return false
		}
	}
	return true
}

// extractBaseVariable extracts the base variable name from an expression
// Examples: "user.profile" -> "user", "GetUser(id)" -> "GetUser", "result" -> "result"
func extractBaseVariable(expr string) string {
	expr = strings.TrimSpace(expr)

	// Check for function call
	if parenIdx := strings.Index(expr, "("); parenIdx != -1 {
		return expr[:parenIdx]
	}

	// Check for field access
	if dotIdx := strings.Index(expr, "."); dotIdx != -1 {
		return expr[:dotIdx]
	}

	return expr
}

// extractFunctionName extracts function name if expression is a function call
// Examples: "GetUser(id)" -> "GetUser", "user.GetProfile()" -> "", "result" -> ""
func extractFunctionName(expr string) string {
	expr = strings.TrimSpace(expr)

	// Must have parentheses
	if !strings.Contains(expr, "(") {
		return ""
	}

	// Must not have dot (method calls not supported yet)
	if strings.Contains(expr, ".") {
		return ""
	}

	parenIdx := strings.Index(expr, "(")
	return expr[:parenIdx]
}

// getIndent extracts leading whitespace from a line
func getIndent(line string) string {
	for i, ch := range line {
		if ch != ' ' && ch != '\t' {
			return line[:i]
		}
	}
	return line
}
