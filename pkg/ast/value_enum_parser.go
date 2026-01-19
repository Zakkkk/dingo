package ast

import (
	"fmt"
	"go/token"
)

// ValueEnumParser parses Dingo value enum declarations into AST nodes.
// Value enums are typed constants: enum Status: int { Pending, Active, Closed }
// Also parses attributes like @prefix(false) before the enum keyword.
type ValueEnumParser struct {
	src    []byte
	pos    int
	fset   *token.FileSet
	file   *token.File
	offset int // Base offset in source file
}

// AttributeParseError represents an error during attribute parsing
type AttributeParseError struct {
	Message string
	Pos     int
}

func (e *AttributeParseError) Error() string {
	return e.Message
}

// NewValueEnumParser creates a new value enum parser for the given source.
func NewValueEnumParser(src []byte, offset int) *ValueEnumParser {
	fset := token.NewFileSet()
	file := fset.AddFile("", -1, len(src))
	return &ValueEnumParser{
		src:    src,
		pos:    0,
		fset:   fset,
		file:   file,
		offset: offset,
	}
}

// ParseValueEnumDecl parses a value enum declaration starting at current position.
// Syntax: enum Name: Type { Variant1, Variant2 = "value", ... }
// Returns the ValueEnumDecl and the end position in the source.
func (p *ValueEnumParser) ParseValueEnumDecl() (*ValueEnumDecl, int, error) {
	startPos := p.pos

	// Expect "enum" keyword
	if !p.matchKeyword("enum") {
		return nil, startPos, fmt.Errorf("expected 'enum' keyword")
	}
	enumPos := token.Pos(p.offset + startPos + 1)

	p.skipWhitespace()

	// Parse enum name
	name, err := p.parseIdent()
	if err != nil {
		return nil, p.pos, fmt.Errorf("expected enum name: %w", err)
	}

	p.skipWhitespace()

	// Expect ':' after name (distinguisher for value enum)
	if p.peek() != ':' {
		return nil, p.pos, fmt.Errorf("expected ':' after enum name for value enum, got %q", string(p.peek()))
	}
	colonPos := token.Pos(p.offset + p.pos + 1)
	p.advance()

	p.skipWhitespace()

	// Parse base type
	baseType, err := p.parseTypeExpr()
	if err != nil {
		return nil, p.pos, fmt.Errorf("expected base type: %w", err)
	}

	// Validate base type
	if !isValidValueEnumBaseType(baseType.Text) {
		return nil, p.pos, fmt.Errorf("invalid value enum base type: %s (use string, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, byte, or rune)", baseType.Text)
	}

	p.skipWhitespace()

	// Expect '{'
	if p.peek() != '{' {
		return nil, p.pos, fmt.Errorf("expected '{' after base type, got %q", string(p.peek()))
	}
	lbracePos := token.Pos(p.offset + p.pos + 1)
	p.advance()

	// Parse variants
	variants, err := p.parseValueEnumVariants(baseType.Text)
	if err != nil {
		return nil, p.pos, fmt.Errorf("invalid variants: %w", err)
	}

	p.skipWhitespaceAndCommas()

	// Expect '}'
	if p.peek() != '}' {
		return nil, p.pos, fmt.Errorf("expected '}' to close enum, got %q at pos %d", string(p.peek()), p.pos)
	}
	rbracePos := token.Pos(p.offset + p.pos + 1)
	p.advance()

	return &ValueEnumDecl{
		Enum:       enumPos,
		Name:       name,
		Colon:      colonPos,
		BaseType:   baseType,
		LBrace:     lbracePos,
		Variants:   variants,
		RBrace:     rbracePos,
		Attributes: nil, // Set by caller if attributes present
	}, p.pos, nil
}

// parseValueEnumVariants parses all value enum variants
func (p *ValueEnumParser) parseValueEnumVariants(baseType string) ([]*ValueEnumVariant, error) {
	var variants []*ValueEnumVariant
	requiresExplicitValue := baseType == "string"
	seenNames := make(map[string]token.Pos)

	for {
		p.skipWhitespaceAndCommas()

		// Check for end of variants
		if p.peek() == '}' || p.pos >= len(p.src) {
			break
		}

		variant, err := p.parseValueEnumVariant(baseType, requiresExplicitValue)
		if err != nil {
			return nil, err
		}

		// Check for duplicates
		if prevPos, exists := seenNames[variant.Name.Name]; exists {
			return nil, fmt.Errorf("duplicate variant %q (first declared at position %d)", variant.Name.Name, prevPos)
		}
		seenNames[variant.Name.Name] = variant.Name.NamePos

		variants = append(variants, variant)
	}

	return variants, nil
}

// parseValueEnumVariant parses a single value enum variant
func (p *ValueEnumParser) parseValueEnumVariant(baseType string, requiresExplicitValue bool) (*ValueEnumVariant, error) {
	name, err := p.parseIdent()
	if err != nil {
		return nil, fmt.Errorf("expected variant name: %w", err)
	}

	p.skipWhitespace()

	variant := &ValueEnumVariant{
		Name: name,
	}

	// Check for explicit value: Name = value
	if p.peek() == '=' {
		variant.Assign = token.Pos(p.offset + p.pos + 1)
		p.advance()
		p.skipWhitespace()

		value, err := p.parseValueExpr()
		if err != nil {
			return nil, fmt.Errorf("expected value after '=': %w", err)
		}
		variant.Value = value

		// Validate value type matches base type
		if err := p.validateValueType(value, baseType); err != nil {
			return nil, err
		}
	} else if requiresExplicitValue {
		return nil, fmt.Errorf("string enum variant %q requires explicit value", name.Name)
	}

	return variant, nil
}

// parseValueExpr parses a literal value (string, int, bool)
func (p *ValueEnumParser) parseValueExpr() (Expr, error) {
	startPos := p.pos

	// String literal
	if p.peek() == '"' {
		return p.parseStringLiteral()
	}

	// Raw string literal
	if p.peek() == '`' {
		return p.parseRawStringLiteral()
	}

	// Numeric literal (int or float)
	if p.isDigit(p.peek()) || (p.peek() == '-' && p.pos+1 < len(p.src) && p.isDigit(p.src[p.pos+1])) {
		return p.parseNumericLiteral()
	}

	// Boolean or identifier
	if p.isAlpha(p.peek()) {
		ident, err := p.parseIdent()
		if err != nil {
			return nil, err
		}
		// Check for boolean literals
		if ident.Name == "true" || ident.Name == "false" {
			return &RawExpr{
				StartPos: ident.NamePos,
				EndPos:   ident.End(),
				Text:     ident.Name,
			}, nil
		}
		// Allow other identifiers (constants, iota)
		return &RawExpr{
			StartPos: ident.NamePos,
			EndPos:   ident.End(),
			Text:     ident.Name,
		}, nil
	}

	return nil, fmt.Errorf("expected literal value at position %d, got %q", startPos, string(p.peek()))
}

// parseStringLiteral parses a double-quoted string literal
func (p *ValueEnumParser) parseStringLiteral() (Expr, error) {
	startPos := p.pos
	if p.peek() != '"' {
		return nil, fmt.Errorf("expected '\"'")
	}
	p.advance() // consume opening "

	for p.pos < len(p.src) && p.peek() != '"' {
		if p.peek() == '\\' {
			p.advance() // skip escape character
		}
		p.advance()
	}

	if p.peek() != '"' {
		return nil, fmt.Errorf("unterminated string literal")
	}
	p.advance() // consume closing "

	return &RawExpr{
		StartPos: token.Pos(p.offset + startPos + 1),
		EndPos:   token.Pos(p.offset + p.pos + 1),
		Text:     string(p.src[startPos:p.pos]),
	}, nil
}

// parseRawStringLiteral parses a backtick-quoted raw string literal
func (p *ValueEnumParser) parseRawStringLiteral() (Expr, error) {
	startPos := p.pos
	if p.peek() != '`' {
		return nil, fmt.Errorf("expected '`'")
	}
	p.advance() // consume opening `

	for p.pos < len(p.src) && p.peek() != '`' {
		p.advance()
	}

	if p.peek() != '`' {
		return nil, fmt.Errorf("unterminated raw string literal")
	}
	p.advance() // consume closing `

	return &RawExpr{
		StartPos: token.Pos(p.offset + startPos + 1),
		EndPos:   token.Pos(p.offset + p.pos + 1),
		Text:     string(p.src[startPos:p.pos]),
	}, nil
}

// parseNumericLiteral parses an integer or float literal
func (p *ValueEnumParser) parseNumericLiteral() (Expr, error) {
	startPos := p.pos

	// Handle negative numbers
	if p.peek() == '-' {
		p.advance()
	}

	// Parse integer part
	for p.isDigit(p.peek()) {
		p.advance()
	}

	// Check for float
	if p.peek() == '.' && p.pos+1 < len(p.src) && p.isDigit(p.src[p.pos+1]) {
		p.advance() // consume '.'
		for p.isDigit(p.peek()) {
			p.advance()
		}
	}

	// Check for hex, octal, binary prefixes
	if p.pos-startPos >= 2 && p.src[startPos] == '0' {
		prefix := p.src[startPos+1]
		if prefix == 'x' || prefix == 'X' || prefix == 'o' || prefix == 'O' || prefix == 'b' || prefix == 'B' {
			for p.isHexDigit(p.peek()) {
				p.advance()
			}
		}
	}

	return &RawExpr{
		StartPos: token.Pos(p.offset + startPos + 1),
		EndPos:   token.Pos(p.offset + p.pos + 1),
		Text:     string(p.src[startPos:p.pos]),
	}, nil
}

// validateValueType checks if value matches the expected base type
func (p *ValueEnumParser) validateValueType(value Expr, baseType string) error {
	rawExpr, ok := value.(*RawExpr)
	if !ok {
		return nil // Can't validate non-RawExpr
	}

	text := rawExpr.Text
	if len(text) == 0 {
		return fmt.Errorf("empty value")
	}

	switch baseType {
	case "string":
		if !((text[0] == '"' && text[len(text)-1] == '"') || (text[0] == '`' && text[len(text)-1] == '`')) {
			return fmt.Errorf("string enum variant requires string literal value, got %q", text)
		}
	case "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64", "byte", "rune":
		// Allow numeric literals, iota, or identifiers (constants)
		if !p.isNumericOrIdent(text) {
			return fmt.Errorf("integer enum variant requires integer literal or iota, got %q", text)
		}
	}

	return nil
}

// isNumericOrIdent checks if text is a numeric literal, iota, or identifier
func (p *ValueEnumParser) isNumericOrIdent(text string) bool {
	if len(text) == 0 {
		return false
	}
	// Numeric (possibly negative)
	if p.isDigit(text[0]) || (text[0] == '-' && len(text) > 1 && p.isDigit(text[1])) {
		return true
	}
	// Hex/octal/binary prefixes
	if len(text) >= 2 && text[0] == '0' {
		prefix := text[1]
		if prefix == 'x' || prefix == 'X' || prefix == 'o' || prefix == 'O' || prefix == 'b' || prefix == 'B' {
			return true
		}
	}
	// Identifier (iota, constants)
	if p.isAlpha(text[0]) {
		return true
	}
	return false
}

// parseIdent parses an identifier
func (p *ValueEnumParser) parseIdent() (*Ident, error) {
	start := p.pos
	if !p.isAlpha(p.peek()) {
		return nil, fmt.Errorf("expected identifier, got %q", string(p.peek()))
	}

	for p.isAlphaNum(p.peek()) {
		p.advance()
	}

	return &Ident{
		NamePos: token.Pos(p.offset + start + 1),
		Name:    string(p.src[start:p.pos]),
	}, nil
}

// parseTypeExpr parses a simple type expression (just the type name)
func (p *ValueEnumParser) parseTypeExpr() (*TypeExpr, error) {
	start := p.pos

	// Parse type name
	if !p.isAlpha(p.peek()) {
		return nil, fmt.Errorf("expected type name")
	}
	for p.isAlphaNum(p.peek()) {
		p.advance()
	}

	return &TypeExpr{
		StartPos: token.Pos(p.offset + start + 1),
		EndPos:   token.Pos(p.offset + p.pos + 1),
		Text:     string(p.src[start:p.pos]),
	}, nil
}

// isValidValueEnumBaseType checks if type is valid for value enums
func isValidValueEnumBaseType(typeName string) bool {
	switch typeName {
	case "string",
		"int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64",
		"byte", "rune":
		return true
	default:
		return false
	}
}

// IsValueEnum checks if source at given position is a value enum (has colon after name)
func IsValueEnum(src []byte) bool {
	p := &ValueEnumParser{src: src, pos: 0}

	// Skip "enum" keyword
	if !p.matchKeyword("enum") {
		return false
	}

	p.skipWhitespace()

	// Skip enum name
	if !p.isAlpha(p.peek()) {
		return false
	}
	for p.isAlphaNum(p.peek()) {
		p.advance()
	}

	p.skipWhitespace()

	// Check for colon (value enum) vs brace (sum type)
	return p.peek() == ':'
}

// Helper methods

func (p *ValueEnumParser) peek() byte {
	if p.pos >= len(p.src) {
		return 0
	}
	return p.src[p.pos]
}

func (p *ValueEnumParser) advance() {
	if p.pos < len(p.src) {
		p.pos++
	}
}

func (p *ValueEnumParser) skipWhitespace() {
	for p.pos < len(p.src) && (p.src[p.pos] == ' ' || p.src[p.pos] == '\t' || p.src[p.pos] == '\n' || p.src[p.pos] == '\r') {
		p.pos++
	}
}

func (p *ValueEnumParser) skipWhitespaceAndCommas() {
	for p.pos < len(p.src) && (p.src[p.pos] == ' ' || p.src[p.pos] == '\t' || p.src[p.pos] == '\n' || p.src[p.pos] == '\r' || p.src[p.pos] == ',') {
		p.pos++
	}
}

func (p *ValueEnumParser) matchKeyword(keyword string) bool {
	if p.pos+len(keyword) > len(p.src) {
		return false
	}
	if string(p.src[p.pos:p.pos+len(keyword)]) != keyword {
		return false
	}
	// Ensure it's not part of a larger identifier
	if p.pos+len(keyword) < len(p.src) && p.isAlphaNum(p.src[p.pos+len(keyword)]) {
		return false
	}
	p.pos += len(keyword)
	return true
}

func (p *ValueEnumParser) isAlpha(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || b == '_'
}

func (p *ValueEnumParser) isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}

func (p *ValueEnumParser) isHexDigit(b byte) bool {
	return p.isDigit(b) || (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F')
}

func (p *ValueEnumParser) isAlphaNum(b byte) bool {
	return p.isAlpha(b) || p.isDigit(b)
}

// =============================================================================
// Attribute Parsing
// =============================================================================

// ParseAttributes parses all attributes before a declaration.
// Syntax: @name(arg1, arg2, ...) or @name
// Returns the list of attributes and any error.
func (p *ValueEnumParser) ParseAttributes() ([]*Attribute, error) {
	var attributes []*Attribute

	for {
		p.skipWhitespace()

		// Check for @ symbol
		if p.peek() != '@' {
			break
		}

		attr, err := p.parseAttribute()
		if err != nil {
			return nil, err
		}
		attributes = append(attributes, attr)
	}

	return attributes, nil
}

// parseAttribute parses a single attribute: @name or @name(args)
func (p *ValueEnumParser) parseAttribute() (*Attribute, error) {
	if p.peek() != '@' {
		return nil, fmt.Errorf("expected '@', got %q", string(p.peek()))
	}

	atPos := token.Pos(p.offset + p.pos + 1)
	p.advance() // consume '@'

	// Parse attribute name
	name, err := p.parseIdent()
	if err != nil {
		return nil, fmt.Errorf("expected attribute name: %w", err)
	}

	attr := &Attribute{
		At:   atPos,
		Name: name,
	}

	p.skipWhitespace()

	// Check for arguments: @name(...)
	if p.peek() == '(' {
		attr.LParen = token.Pos(p.offset + p.pos + 1)
		p.advance() // consume '('

		// Parse arguments
		args, err := p.parseAttributeArgs()
		if err != nil {
			return nil, err
		}
		attr.Args = args

		p.skipWhitespace()

		if p.peek() != ')' {
			return nil, fmt.Errorf("expected ')' to close attribute arguments, got %q", string(p.peek()))
		}
		attr.RParen = token.Pos(p.offset + p.pos + 1)
		p.advance() // consume ')'
	}

	return attr, nil
}

// parseAttributeArgs parses comma-separated arguments inside parentheses
func (p *ValueEnumParser) parseAttributeArgs() ([]Expr, error) {
	var args []Expr

	for {
		p.skipWhitespace()

		// Check for end of arguments
		if p.peek() == ')' || p.pos >= len(p.src) {
			break
		}

		// Parse argument value
		arg, err := p.parseAttributeArg()
		if err != nil {
			return nil, err
		}
		args = append(args, arg)

		p.skipWhitespace()

		// Check for comma separator
		if p.peek() == ',' {
			p.advance()
		}
	}

	return args, nil
}

// parseAttributeArg parses a single attribute argument (boolean, string, int, ident)
func (p *ValueEnumParser) parseAttributeArg() (Expr, error) {
	startPos := p.pos

	// Boolean or identifier
	if p.isAlpha(p.peek()) {
		ident, err := p.parseIdent()
		if err != nil {
			return nil, err
		}
		return &RawExpr{
			StartPos: ident.NamePos,
			EndPos:   ident.End(),
			Text:     ident.Name,
		}, nil
	}

	// String literal
	if p.peek() == '"' || p.peek() == '`' {
		return p.parseValueExpr()
	}

	// Numeric literal
	if p.isDigit(p.peek()) || (p.peek() == '-' && p.pos+1 < len(p.src) && p.isDigit(p.src[p.pos+1])) {
		return p.parseNumericLiteral()
	}

	return nil, fmt.Errorf("expected attribute argument at position %d, got %q", startPos, string(p.peek()))
}

// ParseValueEnumWithAttributes parses attributes followed by a value enum declaration.
// This is the main entry point for parsing: @prefix(false) enum Status: int { ... }
func (p *ValueEnumParser) ParseValueEnumWithAttributes() (*ValueEnumDecl, int, error) {
	// First parse any attributes
	attributes, err := p.ParseAttributes()
	if err != nil {
		return nil, p.pos, fmt.Errorf("invalid attribute: %w", err)
	}

	// Then parse the value enum declaration
	decl, endPos, err := p.ParseValueEnumDecl()
	if err != nil {
		return nil, endPos, err
	}

	// Attach attributes to declaration
	decl.Attributes = attributes

	return decl, endPos, nil
}

// ValidatePrefixAttribute validates the @prefix attribute.
// Returns: (usePrefix bool, error if invalid)
func ValidatePrefixAttribute(attrs []*Attribute) (bool, error) {
	for _, attr := range attrs {
		if attr.Name.Name == "prefix" {
			if len(attr.Args) == 0 {
				return false, fmt.Errorf("@prefix requires a boolean argument: @prefix(true) or @prefix(false)")
			}
			if len(attr.Args) > 1 {
				return false, fmt.Errorf("@prefix accepts only one argument, got %d", len(attr.Args))
			}

			// Check if argument is boolean literal
			rawExpr, ok := attr.Args[0].(*RawExpr)
			if !ok {
				return false, fmt.Errorf("@prefix argument must be a boolean literal (true or false)")
			}

			switch rawExpr.Text {
			case "true":
				return true, nil
			case "false":
				return false, nil
			default:
				return false, fmt.Errorf("@prefix argument must be 'true' or 'false', got %q", rawExpr.Text)
			}
		}
	}
	return true, nil // Default: use prefix
}

// HasAttributeAt checks if source at given position starts with an attribute (@)
func HasAttributeAt(src []byte) bool {
	// Skip leading whitespace
	pos := 0
	for pos < len(src) && (src[pos] == ' ' || src[pos] == '\t' || src[pos] == '\n' || src[pos] == '\r') {
		pos++
	}
	return pos < len(src) && src[pos] == '@'
}

// FindAttributeEnumStart finds the start of value enum with attributes.
// Returns offset where @prefix(...) starts, or -1 if not found.
func FindAttributeEnumStart(src []byte) int {
	// Look for @prefix followed by enum
	for i := 0; i < len(src)-5; i++ {
		if src[i] == '@' {
			// Check if this @ is followed by an identifier, then possibly enum
			j := i + 1
			// Skip identifier name
			for j < len(src) && ((src[j] >= 'a' && src[j] <= 'z') || (src[j] >= 'A' && src[j] <= 'Z') || (src[j] >= '0' && src[j] <= '9') || src[j] == '_') {
				j++
			}
			// Skip whitespace and optional (...)
			for j < len(src) && (src[j] == ' ' || src[j] == '\t' || src[j] == '\n' || src[j] == '\r') {
				j++
			}
			if j < len(src) && src[j] == '(' {
				// Skip to closing paren
				depth := 1
				j++
				for j < len(src) && depth > 0 {
					if src[j] == '(' {
						depth++
					} else if src[j] == ')' {
						depth--
					}
					j++
				}
			}
			// Skip whitespace
			for j < len(src) && (src[j] == ' ' || src[j] == '\t' || src[j] == '\n' || src[j] == '\r') {
				j++
			}
			// Check for enum keyword
			if j+4 <= len(src) && string(src[j:j+4]) == "enum" {
				return i
			}
		}
	}
	return -1
}
