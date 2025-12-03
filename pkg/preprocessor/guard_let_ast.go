package preprocessor

import (
	"bytes"
	"fmt"
	"regexp"
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
	VarNames       []string // Single var or tuple destructuring
	Expr           string   // Expression to unwrap
	ElseBody       string   // Body of else block
	ExprType       ExprType // Result, Option, or Unknown
	IsInline       bool     // Single-line syntax
	Indent         string   // Leading whitespace
	Line           int      // For source mapping
	ConsumedLines  int      // Number of lines consumed (for multiline)
	ErrorBinding   string   // The name from |name|, empty if not specified
	HasPipeBinding bool     // True if |name| syntax was used
	ParseError     error    // Error from parsing (e.g., malformed pipe binding)
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
			// Check for parse errors first
			if match.ParseError != nil {
				return "", nil, fmt.Errorf("line %d: %w", match.Line, match.ParseError)
			}

			// Determine expression type from registry
			match.ExprType = p.inferExpressionType(match.Expr)

			// Validate guard let against pipe binding rules
			if err := p.validateGuardLet(match); err != nil {
				return "", nil, fmt.Errorf("line %d: %w", match.Line, err)
			}

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

	// Parse pipe binding if present
	binding, restAfterPipe, err := p.parsePipeBinding(afterElse)
	if err != nil {
		// Invalid pipe binding syntax - store error for reporting
		return &GuardLetMatch{
			Line:       lineIdx + 1,
			ParseError: fmt.Errorf("pipe binding syntax error: %w\n    hint: Use format |name| (e.g., |err| or |e|)", err),
		}
	}

	hasPipeBinding := binding != ""

	// Parse else block (inline or multiline)
	// Use restAfterPipe if we found a binding, otherwise use original afterElse
	elseBlockInput := afterElse
	if hasPipeBinding {
		elseBlockInput = restAfterPipe
	}
	elseBody, consumedLines, isInline := p.parseElseBlock(elseBlockInput, lines, lineIdx)

	return &GuardLetMatch{
		VarNames:       varNames,
		Expr:           expr,
		ElseBody:       elseBody,
		IsInline:       isInline,
		Indent:         indent,
		Line:           lineIdx + 1,
		ConsumedLines:  consumedLines,
		ErrorBinding:   binding,
		HasPipeBinding: hasPipeBinding,
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
// Now returns pipe binding information as well
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
		var baseIndent string
		braceDepth := 1 // We've seen the opening brace

		// Skip opening brace line
		for i := lineIdx + 1; i < len(lines); i++ {
			line := lines[i]
			trimmed := strings.TrimSpace(line)

			// Track brace depth to handle nested blocks
			openCount := strings.Count(trimmed, "{")
			closeCount := strings.Count(trimmed, "}")
			braceDepth += openCount - closeCount

			// When depth reaches 0, we've found the matching closing brace
			if braceDepth == 0 && trimmed == "}" {
				consumed = i - lineIdx + 1
				break
			}

			// Preserve relative indentation by detecting base indent from first non-empty line
			if baseIndent == "" && trimmed != "" {
				baseIndent = getIndent(line)
			}

			// Remove base indent but preserve any additional indentation
			lineContent := line
			if baseIndent != "" && strings.HasPrefix(line, baseIndent) {
				lineContent = line[len(baseIndent):]
			} else {
				// Line has less indentation than base - just trim leading whitespace
				lineContent = strings.TrimLeft(line, " \t")
			}

			bodyLines = append(bodyLines, lineContent)
		}

		body = strings.Join(bodyLines, "\n")
		return body, consumed, false
	}

	return "", 1, false
}

// parsePipeBinding extracts |name| binding from after "else"
// Returns binding name and remaining string after |name|
func (p *GuardLetASTProcessor) parsePipeBinding(s string) (binding string, rest string, err error) {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "|") {
		return "", s, nil // No pipe binding
	}

	// Find closing |
	closeIdx := strings.Index(s[1:], "|")
	if closeIdx == -1 {
		return "", s, fmt.Errorf("unclosed pipe binding: missing closing |")
	}

	binding = strings.TrimSpace(s[1 : closeIdx+1])
	if binding == "" {
		return "", s, fmt.Errorf("empty pipe binding")
	}

	// Validate binding is valid identifier
	if !isValidIdentifier(binding) {
		return "", s, fmt.Errorf("invalid identifier in pipe binding: %s", binding)
	}

	rest = strings.TrimSpace(s[closeIdx+2:])
	return binding, rest, nil
}

// goKeywords contains all Go reserved keywords
var goKeywords = map[string]bool{
	"break": true, "case": true, "chan": true, "const": true,
	"continue": true, "default": true, "defer": true, "else": true,
	"fallthrough": true, "for": true, "func": true, "go": true,
	"goto": true, "if": true, "import": true, "interface": true,
	"map": true, "package": true, "range": true, "return": true,
	"select": true, "struct": true, "switch": true, "type": true,
	"var": true,
}

// isValidIdentifier checks if a string is a valid Go identifier
func isValidIdentifier(name string) bool {
	if len(name) == 0 {
		return false
	}

	// Check for Go keywords
	if goKeywords[name] {
		return false
	}

	// First character must be letter or underscore
	first := rune(name[0])
	if !unicode.IsLetter(first) && first != '_' {
		return false
	}
	// Remaining characters must be letter, digit, or underscore
	for _, ch := range name[1:] {
		if !unicode.IsLetter(ch) && !unicode.IsDigit(ch) && ch != '_' {
			return false
		}
	}
	return true
}

// validateGuardLet validates guard let match against pipe binding rules
func (p *GuardLetASTProcessor) validateGuardLet(match *GuardLetMatch) error {
	switch match.ExprType {
	case ExprTypeResult:
		// Check if body uses "err" without explicit binding
		if !match.HasPipeBinding && p.usesErrorInBody(match.ElseBody, "err") {
			return fmt.Errorf("implicit 'err' not allowed: use explicit binding |err| or |e|\n    hint: Change: else { return err } -> else |err| { return err }")
		}

	case ExprTypeOption:
		// Option types cannot have pipe binding
		if match.HasPipeBinding {
			return fmt.Errorf("pipe binding not allowed on Option types (no error to bind)\n    hint: Option types only have Some/None, not an error value")
		}
	}
	return nil
}

// usesErrorInBody checks if the else body references a specific error variable name
func (p *GuardLetASTProcessor) usesErrorInBody(body string, varName string) bool {
	// Remove comments (both // and /* */)
	body = removeComments(body)

	// Remove string literals
	body = removeStringLiterals(body)

	// Pattern: varName as standalone identifier (word boundary)
	pattern := regexp.MustCompile(`\b` + regexp.QuoteMeta(varName) + `\b`)
	return pattern.MatchString(body)
}

// removeComments removes both single-line and multi-line comments from source code
func removeComments(s string) string {
	// Remove single-line comments
	s = regexp.MustCompile(`//.*`).ReplaceAllString(s, "")
	// Remove multi-line comments
	s = regexp.MustCompile(`/\*.*?\*/`).ReplaceAllString(s, "")
	return s
}

// removeStringLiterals removes string literals from source code
func removeStringLiterals(s string) string {
	// Remove double-quoted strings
	s = regexp.MustCompile(`"[^"]*"`).ReplaceAllString(s, `""`)
	// Remove backtick strings
	s = regexp.MustCompile("`[^`]*`").ReplaceAllString(s, "``")
	return s
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
	// Validation first
	if err := p.validateGuardLet(match); err != nil {
		// Return error as comment in generated code
		return fmt.Sprintf("%s// ERROR: %v\n", match.Indent, err), nil
	}

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

	// Generate error binding ONLY if explicit pipe binding provided
	if match.HasPipeBinding {
		errVarName := match.ErrorBinding
		buf.WriteString(fmt.Sprintf("%s\t%s := *%s.err\n", indent, errVarName, exprVar))

		// Transform else body: replace "return <binding>" with "return ResultErr(<binding>)"
		transformedBody := p.transformReturnStatements(match.ElseBody, errVarName)
		transformedBody = strings.TrimSpace(transformedBody)
		if match.IsInline {
			buf.WriteString(fmt.Sprintf("%s\t%s\n", indent, transformedBody))
		} else {
			// Multiline: indent each line
			bodyLines := strings.Split(transformedBody, "\n")
			for _, line := range bodyLines {
				buf.WriteString(fmt.Sprintf("%s\t%s\n", indent, line))
			}
		}
	} else {
		// No binding - body cannot reference error
		elseBody := strings.TrimSpace(match.ElseBody)
		if match.IsInline {
			buf.WriteString(fmt.Sprintf("%s\t%s\n", indent, elseBody))
		} else {
			// Multiline: indent each line
			bodyLines := strings.Split(elseBody, "\n")
			for _, line := range bodyLines {
				buf.WriteString(fmt.Sprintf("%s\t%s\n", indent, line))
			}
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
	// Validation first
	if err := p.validateGuardLet(match); err != nil {
		// Return error as comment in generated code
		return fmt.Sprintf("%s// ERROR: %v\n", match.Indent, err), nil
	}

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

// transformReturnStatements wraps bare error returns with ResultErr()
// Handles: return e -> return ResultErr(e)
// Handles: return fmt.Errorf(...) -> unchanged (already error type)
func (p *GuardLetASTProcessor) transformReturnStatements(body string, boundVar string) string {
	lines := strings.Split(body, "\n")
	result := make([]string, len(lines))

	// Pattern: "return <boundVar>" at end of line (with optional whitespace/comment)
	// Uses word boundaries to avoid matching "return err.Error()" or "return myerr"
	pattern := regexp.MustCompile(`\breturn\s+` + regexp.QuoteMeta(boundVar) + `\s*($|//|/\*)`)
	replacement := "return ResultErr(" + boundVar + ")"

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip if already wrapped
		if strings.Contains(trimmed, "ResultErr("+boundVar+")") {
			result[i] = line
			continue
		}

		// Only transform bare "return <boundVar>" at end of line
		if pattern.MatchString(trimmed) {
			result[i] = pattern.ReplaceAllString(line, replacement+"$1")
		} else {
			result[i] = line
		}
	}

	return strings.Join(result, "\n")
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
