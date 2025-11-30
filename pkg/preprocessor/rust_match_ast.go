package preprocessor

import (
	"bytes"
	"fmt"
	"go/token"
	"regexp"
	"strings"

	dingoast "github.com/MadAppGang/dingo/pkg/ast"
)

// RustMatchASTProcessor handles Rust-like pattern matching using AST parsing
// Replaces regex-based RustMatchProcessor with proper parsing
type RustMatchASTProcessor struct {
	matchCounter int
	mappings     []Mapping
}

// NewRustMatchASTProcessor creates a new AST-based match processor
func NewRustMatchASTProcessor() *RustMatchASTProcessor {
	return &RustMatchASTProcessor{
		matchCounter: 0,
		mappings:     []Mapping{},
	}
}

// matchExprPattern matches the entire match expression: match expr { ... }
var matchExprASTPattern = regexp.MustCompile(`(?s)match\s+([^{]+)\s*\{(.+)\}`)

// Name returns the processor name
func (p *RustMatchASTProcessor) Name() string {
	return "rust_match_ast"
}

// Process transforms Rust-style match expressions to Go switch statements
func (p *RustMatchASTProcessor) Process(source []byte) ([]byte, []Mapping, error) {
	// Default filename for error reporting
	filename := "match.dingo"
	return p.ProcessWithFilename(source, filename)
}

// ProcessWithFilename transforms match expressions with explicit filename for error reporting
func (p *RustMatchASTProcessor) ProcessWithFilename(source []byte, filename string) ([]byte, []Mapping, error) {
	// Reset state
	p.mappings = []Mapping{}

	// Parse all match expressions
	fset := token.NewFileSet()
	matches, err := p.parseMatchExpressions(source, fset)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse match expressions: %w", err)
	}

	if len(matches) == 0 {
		// No match expressions found
		return source, p.mappings, nil
	}

	// Transform in reverse order to preserve positions
	result := source
	for i := len(matches) - 1; i >= 0; i-- {
		match := matches[i]
		result, err = p.transformMatch(result, match, filename)
		if err != nil {
			return nil, nil, err
		}
	}

	return result, p.mappings, nil
}

// parseMatchExpressions finds and parses all match expressions in the source
func (p *RustMatchASTProcessor) parseMatchExpressions(source []byte, fset *token.FileSet) ([]*dingoast.MatchExpr, error) {
	var matches []*dingoast.MatchExpr

	// Find all match keywords
	matchKeyword := []byte("match ")
	pos := 0
	for {
		idx := bytes.Index(source[pos:], matchKeyword)
		if idx == -1 {
			break
		}
		matchStart := pos + idx

		// Check if this is in a comment (simple check: is it after //)
		if p.isInComment(source, matchStart) {
			pos = matchStart + 1
			continue
		}

		// Find the scrutinee and opening brace
		scrutineeStart := matchStart + len(matchKeyword)
		braceIdx := bytes.IndexByte(source[scrutineeStart:], '{')
		if braceIdx == -1 {
			// No opening brace found
			pos = matchStart + 1
			continue
		}
		braceStart := scrutineeStart + braceIdx
		scrutinee := string(source[scrutineeStart:braceStart])
		scrutinee = strings.TrimSpace(scrutinee)

		// Validate scrutinee looks like an identifier (not multi-line or containing enum)
		if strings.Contains(scrutinee, "\n") || strings.Contains(scrutinee, "enum") {
			pos = matchStart + 1
			continue
		}

		// Find matching closing brace
		bodyStart := braceStart + 1
		bodyEnd, found := p.findMatchingBrace(source, braceStart)
		if !found {
			// No matching brace
			pos = matchStart + 1
			continue
		}

		bodyText := string(source[bodyStart:bodyEnd])

		// DEBUG: Log what we captured
		// fmt.Printf("DEBUG: Match found\n")
		// fmt.Printf("  Scrutinee: %q\n", scrutinee)
		// fmt.Printf("  Body: %q\n", bodyText)

		// Parse match arms
		arms, err := p.parseMatchArms(bodyText)
		if err != nil {
			return nil, fmt.Errorf("failed to parse match arms (scrutinee=%q): %w", scrutinee, err)
		}

		// Create match expression
		matchExpr := &dingoast.MatchExpr{
			MatchPos:  token.Pos(matchStart),
			Scrutinee: scrutinee,
			Arms:      arms,
			IsExpr:    p.isExpressionContext(source, matchStart),
			MatchID:   p.matchCounter,
		}

		p.matchCounter++
		matches = append(matches, matchExpr)

		// Continue searching after this match
		pos = bodyEnd + 1
	}

	return matches, nil
}

// isInComment checks if a position is inside a comment
func (p *RustMatchASTProcessor) isInComment(source []byte, pos int) bool {
	// Find the start of the line
	lineStart := pos
	for lineStart > 0 && source[lineStart-1] != '\n' {
		lineStart--
	}

	// Check if there's a // before our position on this line
	line := source[lineStart:pos]
	return bytes.Contains(line, []byte("//"))
}

// findMatchingBrace finds the matching closing brace for an opening brace at pos
func (p *RustMatchASTProcessor) findMatchingBrace(source []byte, openPos int) (int, bool) {
	depth := 1
	i := openPos + 1
	inString := false
	var stringChar byte

	for i < len(source) && depth > 0 {
		ch := source[i]

		// Handle strings
		if ch == '"' || ch == '\'' || ch == '`' {
			if !inString {
				inString = true
				stringChar = ch
			} else if ch == stringChar && (i == 0 || source[i-1] != '\\') {
				inString = false
			}
		}

		if !inString {
			if ch == '{' {
				depth++
			} else if ch == '}' {
				depth--
				if depth == 0 {
					return i, true
				}
			}
		}

		i++
	}

	return 0, false
}

// parseMatchArms parses the arms of a match expression
func (p *RustMatchASTProcessor) parseMatchArms(body string) ([]dingoast.MatchArm, error) {
	var arms []dingoast.MatchArm

	// Split body into arms (comma-separated, but handle nested braces)
	armTexts := p.splitMatchArms(body)

	for _, armText := range armTexts {
		armText = strings.TrimSpace(armText)
		if armText == "" {
			continue
		}

		arm, err := p.parseMatchArm(armText)
		if err != nil {
			return nil, err
		}

		arms = append(arms, arm)
	}

	return arms, nil
}

// parseMatchArm parses a single match arm: Pattern [if Guard] => Body
func (p *RustMatchASTProcessor) parseMatchArm(armText string) (dingoast.MatchArm, error) {
	// Find the => separator
	arrowIdx := strings.Index(armText, "=>")
	if arrowIdx == -1 {
		return dingoast.MatchArm{}, fmt.Errorf("match arm missing '=>': %q (len=%d)", strings.TrimSpace(armText), len(armText))
	}

	// Split into pattern+guard and body
	patternPart := strings.TrimSpace(armText[:arrowIdx])
	bodyPart := strings.TrimSpace(armText[arrowIdx+2:])

	// Parse pattern and guard
	pattern, guard, err := p.parsePatternWithGuard(patternPart)
	if err != nil {
		return dingoast.MatchArm{}, err
	}

	// Check if body is a block
	isBlock := strings.HasPrefix(bodyPart, "{")

	return dingoast.MatchArm{
		Pattern: pattern,
		Guard:   guard,
		Body:    bodyPart,
		IsBlock: isBlock,
	}, nil
}

// parsePatternWithGuard parses a pattern with optional guard
func (p *RustMatchASTProcessor) parsePatternWithGuard(text string) (dingoast.Pattern, string, error) {
	// Check for guard: " if condition"
	ifIdx := strings.Index(text, " if ")
	var patternText, guard string

	if ifIdx != -1 {
		patternText = strings.TrimSpace(text[:ifIdx])
		guard = strings.TrimSpace(text[ifIdx+4:]) // Skip " if "
	} else {
		patternText = text
		guard = ""
	}

	// Parse the pattern
	pattern, err := p.parsePattern(patternText)
	if err != nil {
		return nil, "", err
	}

	return pattern, guard, nil
}

// parsePattern parses a single pattern
func (p *RustMatchASTProcessor) parsePattern(text string) (dingoast.Pattern, error) {
	text = strings.TrimSpace(text)

	// Wildcard pattern: _
	if text == "_" {
		return &dingoast.WildcardPattern{}, nil
	}

	// Tuple pattern: (a, b) or (Ok, Err)
	if strings.HasPrefix(text, "(") && strings.HasSuffix(text, ")") {
		return p.parseTuplePattern(text)
	}

	// Constructor pattern: Ok(x), Err(e), Status_Pending, Some(v), None
	if strings.Contains(text, "(") {
		return p.parseConstructorPattern(text)
	}

	// Literal patterns: numbers, strings, booleans
	if p.isLiteral(text) {
		return &dingoast.LiteralPattern{Value: text}, nil
	}

	// Check if it's a known constructor without params (None, or enum variant)
	if p.isKnownConstructor(text) {
		return &dingoast.ConstructorPattern{
			Name:   text,
			Params: nil,
		}, nil
	}

	// Variable pattern: x, result, etc.
	return &dingoast.VariablePattern{Name: text}, nil
}

// parseTuplePattern parses a tuple pattern: (a, b)
func (p *RustMatchASTProcessor) parseTuplePattern(text string) (dingoast.Pattern, error) {
	// Remove outer parentheses
	text = strings.TrimSpace(text)
	if !strings.HasPrefix(text, "(") || !strings.HasSuffix(text, ")") {
		return nil, fmt.Errorf("invalid tuple pattern: %s", text)
	}
	text = text[1 : len(text)-1]

	// Split by comma (handling nested parentheses)
	parts := p.splitByComma(text)

	var elements []dingoast.Pattern
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		elem, err := p.parsePattern(part)
		if err != nil {
			return nil, err
		}
		elements = append(elements, elem)
	}

	return &dingoast.TuplePattern{Elements: elements}, nil
}

// parseConstructorPattern parses a constructor pattern: Ok(x), Err(e)
func (p *RustMatchASTProcessor) parseConstructorPattern(text string) (dingoast.Pattern, error) {
	// Find the opening parenthesis
	parenIdx := strings.Index(text, "(")
	if parenIdx == -1 {
		return nil, fmt.Errorf("invalid constructor pattern: %s", text)
	}

	name := strings.TrimSpace(text[:parenIdx])
	paramsPart := text[parenIdx+1:]

	// Find matching closing parenthesis
	if !strings.HasSuffix(paramsPart, ")") {
		return nil, fmt.Errorf("invalid constructor pattern (missing closing paren): %s", text)
	}
	paramsPart = paramsPart[:len(paramsPart)-1]

	// Parse parameters
	var params []string
	if strings.TrimSpace(paramsPart) != "" {
		parts := p.splitByComma(paramsPart)
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part != "" {
				params = append(params, part)
			}
		}
	}

	return &dingoast.ConstructorPattern{
		Name:   name,
		Params: params,
	}, nil
}

// splitMatchArms splits match body into individual arms (comma-separated)
func (p *RustMatchASTProcessor) splitMatchArms(body string) []string {
	var arms []string
	var current strings.Builder
	depth := 0
	inString := false
	var stringChar rune

	for i, ch := range body {
		switch ch {
		case '"', '\'', '`':
			if !inString {
				inString = true
				stringChar = ch
			} else if ch == stringChar && (i == 0 || body[i-1] != '\\') {
				inString = false
			}
			current.WriteRune(ch)

		case '{', '(':
			if !inString {
				depth++
			}
			current.WriteRune(ch)

		case '}', ')':
			if !inString {
				depth--
			}
			current.WriteRune(ch)

		case ',':
			if !inString && depth == 0 {
				// End of arm
				arms = append(arms, current.String())
				current.Reset()
			} else {
				current.WriteRune(ch)
			}

		default:
			current.WriteRune(ch)
		}
	}

	// Add the last arm
	if current.Len() > 0 {
		arms = append(arms, current.String())
	}

	return arms
}

// splitByComma splits text by comma, handling nested parentheses
func (p *RustMatchASTProcessor) splitByComma(text string) []string {
	var parts []string
	var current strings.Builder
	depth := 0
	inString := false
	var stringChar rune

	for i, ch := range text {
		switch ch {
		case '"', '\'', '`':
			if !inString {
				inString = true
				stringChar = ch
			} else if ch == stringChar && (i == 0 || text[i-1] != '\\') {
				inString = false
			}
			current.WriteRune(ch)

		case '(', '{':
			if !inString {
				depth++
			}
			current.WriteRune(ch)

		case ')', '}':
			if !inString {
				depth--
			}
			current.WriteRune(ch)

		case ',':
			if !inString && depth == 0 {
				parts = append(parts, current.String())
				current.Reset()
			} else {
				current.WriteRune(ch)
			}

		default:
			current.WriteRune(ch)
		}
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}

// isLiteral checks if text is a literal value
func (p *RustMatchASTProcessor) isLiteral(text string) bool {
	// Numbers
	if matched, _ := regexp.MatchString(`^-?\d+(\.\d+)?$`, text); matched {
		return true
	}

	// Strings
	if strings.HasPrefix(text, `"`) && strings.HasSuffix(text, `"`) {
		return true
	}
	if strings.HasPrefix(text, "`") && strings.HasSuffix(text, "`") {
		return true
	}

	// Booleans
	if text == "true" || text == "false" {
		return true
	}

	return false
}

// isKnownConstructor checks if text is a known constructor without parameters
func (p *RustMatchASTProcessor) isKnownConstructor(text string) bool {
	// Result/Option constructors
	if text == "None" {
		return true
	}

	// Enum variants often have format: EnumName_VariantName
	if strings.Contains(text, "_") {
		return true
	}

	return false
}

// isExpressionContext checks if a match is used in expression context
func (p *RustMatchASTProcessor) isExpressionContext(source []byte, matchPos int) bool {
	// Look backwards for context clues
	lineStart := matchPos
	for lineStart > 0 && source[lineStart-1] != '\n' {
		lineStart--
	}

	prefix := string(source[lineStart:matchPos])
	prefix = strings.TrimSpace(prefix)

	// Expression contexts
	if strings.HasSuffix(prefix, ":=") ||
		strings.HasSuffix(prefix, "=") ||
		strings.HasSuffix(prefix, "return") ||
		strings.HasSuffix(prefix, "(") ||
		strings.HasSuffix(prefix, ",") {
		return true
	}

	return false
}

// transformMatch transforms a single match expression to Go code
func (p *RustMatchASTProcessor) transformMatch(source []byte, match *dingoast.MatchExpr, filename string) ([]byte, error) {
	// Check exhaustiveness before transformation
	if err := p.checkExhaustiveness(match, filename); err != nil {
		return nil, err
	}

	// Generate Go code
	goCode := match.ToGo()

	// Find the match expression in source and replace it
	// Use the match position to locate it
	matchStart := int(match.Pos())
	matchEnd := int(match.End())

	// Ensure positions are valid
	if matchStart < 0 || matchStart >= len(source) {
		matchStart = 0
	}
	if matchEnd > len(source) || matchEnd <= matchStart {
		matchEnd = len(source)
	}

	// Find the actual match expression by searching forward from matchStart
	matchText := p.findMatchExpression(source, matchStart)
	if matchText == "" {
		return nil, fmt.Errorf("could not locate match expression at position %d", matchStart)
	}

	// Replace match expression with Go code
	actualStart := bytes.Index(source[matchStart:], []byte("match "))
	if actualStart == -1 {
		return nil, fmt.Errorf("could not find 'match' keyword at position %d", matchStart)
	}
	actualStart += matchStart

	// Check if "return" precedes "match" - if so, include it in replacement
	// This handles "return match { ... }" where the return is part of the statement
	returnKeyword := []byte("return")
	checkStart := actualStart - 20 // Look back up to 20 bytes
	if checkStart < 0 {
		checkStart = 0
	}
	precedingText := source[checkStart:actualStart]
	if returnIdx := bytes.LastIndex(precedingText, returnKeyword); returnIdx != -1 {
		// Check if it's actually "return" followed by whitespace then "match"
		afterReturn := precedingText[returnIdx+len(returnKeyword):]
		if len(bytes.TrimSpace(afterReturn)) == 0 {
			// Yes, "return match ..." pattern found
			actualStart = checkStart + returnIdx
			// The generated code already includes "return" at the end
		}
	}

	actualEnd := actualStart + len(matchText)

	// Create result with replacement
	var result []byte
	result = append(result, source[:actualStart]...)
	result = append(result, []byte(goCode)...)
	result = append(result, source[actualEnd:]...)

	// Record mapping for source maps (simplified - just track line)
	line := bytes.Count(source[:actualStart], []byte("\n")) + 1
	p.recordMapping(line, 0, len(matchText))

	return result, nil
}

// findMatchExpression finds the full match expression starting at pos
func (p *RustMatchASTProcessor) findMatchExpression(source []byte, pos int) string {
	// Find "match " keyword
	matchStart := bytes.Index(source[pos:], []byte("match "))
	if matchStart == -1 {
		return ""
	}
	matchStart += pos

	// Find the opening brace
	braceStart := bytes.IndexByte(source[matchStart:], '{')
	if braceStart == -1 {
		return ""
	}
	braceStart += matchStart

	// Find the matching closing brace
	depth := 1
	i := braceStart + 1
	for i < len(source) && depth > 0 {
		if source[i] == '{' {
			depth++
		} else if source[i] == '}' {
			depth--
		}
		i++
	}

	if depth != 0 {
		return ""
	}

	return string(source[matchStart:i])
}

// checkExhaustiveness validates that all patterns are covered
func (p *RustMatchASTProcessor) checkExhaustiveness(match *dingoast.MatchExpr, filename string) error {
	// Check if there's a wildcard pattern
	hasWildcard := false
	for _, arm := range match.Arms {
		if _, ok := arm.Pattern.(*dingoast.WildcardPattern); ok {
			hasWildcard = true
			break
		}
		if _, ok := arm.Pattern.(*dingoast.VariablePattern); ok {
			hasWildcard = true
			break
		}
	}

	// If there's a wildcard, the match is exhaustive
	if hasWildcard {
		return nil
	}

	// Check based on scrutinee type
	// For now, detect type from patterns
	matchType := p.detectMatchType(match)

	switch matchType {
	case "Result":
		return p.checkResultExhaustiveness(match, filename)
	case "Option":
		return p.checkOptionExhaustiveness(match, filename)
	case "Enum":
		return p.checkEnumExhaustiveness(match, filename)
	case "Tuple":
		return p.checkTupleExhaustiveness(match, filename)
	default:
		// For other types, require wildcard
		if !hasWildcard {
			return p.exhaustivenessError(filename, match, "match expression is not exhaustive (missing wildcard pattern)")
		}
	}

	return nil
}

// detectMatchType detects the type being matched based on patterns
func (p *RustMatchASTProcessor) detectMatchType(match *dingoast.MatchExpr) string {
	if len(match.Arms) == 0 {
		return "Unknown"
	}

	// Check first pattern
	switch pattern := match.Arms[0].Pattern.(type) {
	case *dingoast.ConstructorPattern:
		if pattern.Name == "Ok" || pattern.Name == "Err" {
			return "Result"
		}
		if pattern.Name == "Some" || pattern.Name == "None" {
			return "Option"
		}
		// Enum variant (contains underscore or uppercase start)
		return "Enum"
	case *dingoast.TuplePattern:
		return "Tuple"
	default:
		return "Unknown"
	}
}

// checkResultExhaustiveness checks Result<T,E> exhaustiveness
func (p *RustMatchASTProcessor) checkResultExhaustiveness(match *dingoast.MatchExpr, filename string) error {
	hasOk := false
	hasErr := false

	for _, arm := range match.Arms {
		if ctor, ok := arm.Pattern.(*dingoast.ConstructorPattern); ok {
			if ctor.Name == "Ok" {
				hasOk = true
			}
			if ctor.Name == "Err" {
				hasErr = true
			}
		}
	}

	if !hasOk || !hasErr {
		missing := []string{}
		if !hasOk {
			missing = append(missing, "Ok")
		}
		if !hasErr {
			missing = append(missing, "Err")
		}
		return p.exhaustivenessError(filename, match, fmt.Sprintf("match on Result<T,E> is not exhaustive (missing patterns: %v)", missing))
	}

	return nil
}

// checkOptionExhaustiveness checks Option<T> exhaustiveness
func (p *RustMatchASTProcessor) checkOptionExhaustiveness(match *dingoast.MatchExpr, filename string) error {
	hasSome := false
	hasNone := false

	for _, arm := range match.Arms {
		if ctor, ok := arm.Pattern.(*dingoast.ConstructorPattern); ok {
			if ctor.Name == "Some" {
				hasSome = true
			}
			if ctor.Name == "None" {
				hasNone = true
			}
		}
	}

	if !hasSome || !hasNone {
		missing := []string{}
		if !hasSome {
			missing = append(missing, "Some")
		}
		if !hasNone {
			missing = append(missing, "None")
		}
		return p.exhaustivenessError(filename, match, fmt.Sprintf("match on Option<T> is not exhaustive (missing patterns: %v)", missing))
	}

	return nil
}

// checkEnumExhaustiveness checks enum exhaustiveness
func (p *RustMatchASTProcessor) checkEnumExhaustiveness(match *dingoast.MatchExpr, filename string) error {
	// For enums, we need to know all possible variants
	// This requires type information, which we don't have in preprocessor
	// The plugin phase will do exhaustiveness checking with full type info
	// For now, we just accept any enum match and let the plugin validate it

	return nil
}

// checkTupleExhaustiveness checks tuple exhaustiveness
func (p *RustMatchASTProcessor) checkTupleExhaustiveness(match *dingoast.MatchExpr, filename string) error {
	// Tuple exhaustiveness is complex - requires checking all combinations
	// For now, require wildcard
	hasWildcard := false
	for _, arm := range match.Arms {
		if _, ok := arm.Pattern.(*dingoast.WildcardPattern); ok {
			hasWildcard = true
			break
		}
		if _, ok := arm.Pattern.(*dingoast.VariablePattern); ok {
			hasWildcard = true
			break
		}
	}

	if !hasWildcard {
		return p.exhaustivenessError(filename, match, "tuple match should include wildcard pattern")
	}

	return nil
}

// exhaustivenessError creates a formatted exhaustiveness error
func (p *RustMatchASTProcessor) exhaustivenessError(filename string, match *dingoast.MatchExpr, message string) error {
	// TODO: Get actual line/column from token.FileSet
	// For now, use position
	return fmt.Errorf("%s:%d:1: %s", filename, match.Pos(), message)
}

// recordMapping records a source map mapping for the transformation
func (p *RustMatchASTProcessor) recordMapping(originalLine, originalCol, length int) {
	// Create mapping for the transformation
	mapping := Mapping{
		OriginalLine:   originalLine,
		OriginalColumn: originalCol,
		Length:         length,
		Name:           "match",
	}
	p.mappings = append(p.mappings, mapping)
}

// GetMappings returns the source map mappings
func (p *RustMatchASTProcessor) GetMappings() []Mapping {
	return p.mappings
}
