package preprocessor

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
)

// EnumASTProcessor handles enum declarations using token-based parsing
// This replaces the regex-based approach in enum.go, fixing bugs with:
//   - Nested braces in struct variants: Variant { map: map[string]struct{} }
//   - Generic types in variants: Some(Option<T>)
//   - Comments inside enum bodies
//   - String literals containing "enum"
type EnumASTProcessor struct {
	source  []byte
	pos     int
	line    int
	col     int
	counter int
}

// NewEnumASTProcessor creates a new AST-based enum processor
func NewEnumASTProcessor() *EnumASTProcessor {
	return &EnumASTProcessor{}
}

// Name returns the processor name
func (p *EnumASTProcessor) Name() string {
	return "enum_ast"
}

// Process implements FeatureProcessor interface
func (p *EnumASTProcessor) Process(source []byte) ([]byte, []Mapping, error) {
	result, _, err := p.ProcessInternal(string(source))
	return []byte(result), nil, err
}

// ProcessInternal transforms enum declarations with metadata support
func (p *EnumASTProcessor) ProcessInternal(code string) (string, []TransformMetadata, error) {
	p.source = []byte(code)
	p.pos = 0
	p.line = 1
	p.col = 1
	p.counter = 0

	var metadata []TransformMetadata
	var result bytes.Buffer

	// Track enum info for constructor transformation
	type enumInfo struct {
		name     string
		variants []string
	}
	var enumInfos []enumInfo

	// Find all enum declarations
	enumDecls := p.findEnumDeclarations()

	// Build result by replacing enums
	lastPos := 0
	for _, decl := range enumDecls {
		// Copy source before enum
		result.Write(p.source[lastPos:decl.start])

		// Parse variants
		variants, err := p.parseVariantsFromBody(decl.body)
		if err != nil {
			// Skip this enum if parsing fails
			result.Write(p.source[decl.start:decl.end])
			lastPos = decl.end
			continue
		}

		// Collect variant names for constructor transformation
		variantNames := make([]string, len(variants))
		for i, v := range variants {
			variantNames[i] = v.Name
		}
		enumInfos = append(enumInfos, enumInfo{name: decl.name, variants: variantNames})

		// Generate Go sum type
		marker := fmt.Sprintf("// dingo:n:%d", p.counter)
		generated := p.generateSumType(decl.name, variants, marker)
		result.WriteString(generated)

		// Add metadata
		metadata = append(metadata, TransformMetadata{
			Type:            "enum",
			OriginalLine:    decl.startLine,
			OriginalColumn:  decl.start,
			OriginalLength:  decl.end - decl.start,
			OriginalText:    fmt.Sprintf("enum %s {...}", decl.name),
			GeneratedMarker: marker,
			ASTNodeType:     "TypeSpec",
		})
		p.counter++

		lastPos = decl.end
	}

	// Copy remaining source
	result.Write(p.source[lastPos:])

	// Transform underscore constructor calls to PascalCase
	// e.g., Status_Pending() → StatusPending()
	resultStr := result.String()
	for _, info := range enumInfos {
		for _, variant := range info.variants {
			// Replace EnumName_Variant with EnumNameVariant
			oldCall := info.name + "_" + variant
			newCall := info.name + variant
			resultStr = strings.ReplaceAll(resultStr, oldCall, newCall)
		}
	}

	return resultStr, metadata, nil
}

// EnumDecl represents a parsed enum declaration
type EnumDecl struct {
	start     int
	end       int
	startLine int
	name      string
	body      string
}

// findEnumDeclarations finds all enum declarations using proper tokenization
func (p *EnumASTProcessor) findEnumDeclarations() []EnumDecl {
	var decls []EnumDecl

	for p.pos < len(p.source) {
		// Skip whitespace
		p.skipWhitespace()

		// Skip comments
		if p.peek() == '/' && (p.peekN(1) == '/' || p.peekN(1) == '*') {
			p.skipComment()
			continue
		}

		// Skip string literals
		if p.peek() == '"' || p.peek() == '`' {
			p.skipString()
			continue
		}

		// Check for "enum" keyword
		if p.matchKeyword("enum") {
			enumStart := p.pos - 4
			enumStartLine := p.line

			// Skip whitespace after "enum"
			p.skipWhitespace()

			// Read enum name
			enumName := p.readIdentifier()
			if enumName == "" {
				p.advance()
				continue
			}

			// Skip whitespace before {
			p.skipWhitespace()

			// Expect opening brace
			if p.peek() != '{' {
				p.advance()
				continue
			}

			braceStart := p.pos
			p.advance() // consume {

			// Find matching closing brace
			braceEnd := p.findMatchingBrace()
			if braceEnd == -1 {
				p.advance()
				continue
			}

			// Extract body
			body := string(p.source[braceStart+1 : braceEnd])

			// Skip trailing whitespace (preserve one newline)
			enumEnd := braceEnd + 1
			for enumEnd < len(p.source) && (p.source[enumEnd] == ' ' || p.source[enumEnd] == '\t' || p.source[enumEnd] == '\n') {
				if p.source[enumEnd] == '\n' {
					enumEnd++
					break
				}
				enumEnd++
			}

			decls = append(decls, EnumDecl{
				start:     enumStart,
				end:       enumEnd,
				startLine: enumStartLine,
				name:      enumName,
				body:      body,
			})

			p.pos = enumEnd
		} else {
			p.advance()
		}
	}

	return decls
}

// parseVariantsFromBody parses variants from enum body
func (p *EnumASTProcessor) parseVariantsFromBody(body string) ([]Variant, error) {
	parser := &variantParser{
		source: []byte(body),
		pos:    0,
	}
	return parser.parseVariants()
}

// variantParser handles parsing of variant list
type variantParser struct {
	source []byte
	pos    int
}

// parseVariants parses all variants in the enum body
func (vp *variantParser) parseVariants() ([]Variant, error) {
	var variants []Variant

	for vp.pos < len(vp.source) {
		startPos := vp.pos
		vp.skipWhitespace()
		vp.skipComments()
		vp.skipWhitespace()

		if vp.pos >= len(vp.source) {
			break
		}

		// Read variant
		variant, err := vp.parseVariant()
		if err != nil {
			return nil, err
		}
		if variant.Name != "" {
			variants = append(variants, variant)
		}

		// Skip trailing comma
		vp.skipWhitespace()
		if vp.peek() == ',' {
			vp.advance()
		}

		// Safety check: ensure we made progress
		if vp.pos == startPos {
			// Stuck in infinite loop - advance to avoid it
			vp.advance()
		}
	}

	if len(variants) == 0 {
		return nil, fmt.Errorf("no variants found")
	}

	return variants, nil
}

// parseVariant parses a single variant
// Grammar:
//   Variant = IDENT                          // unit variant
//           | IDENT "(" TypeList ")"         // tuple variant
//           | IDENT "{" FieldList "}"        // struct variant
func (vp *variantParser) parseVariant() (Variant, error) {
	vp.skipWhitespace()

	// Read variant name
	name := vp.readIdentifier()
	if name == "" {
		return Variant{}, nil
	}

	vp.skipWhitespace()

	// Check what follows
	ch := vp.peek()

	switch ch {
	case '(':
		// Tuple variant: Name(type1, type2, ...)
		vp.advance() // consume (
		fields, err := vp.parseTupleFields()
		if err != nil {
			return Variant{}, err
		}
		// Expect closing paren
		if vp.peek() != ')' {
			return Variant{}, fmt.Errorf("expected ')' after tuple fields")
		}
		vp.advance()
		return Variant{Name: name, Fields: fields}, nil

	case '{':
		// Struct variant: Name { field: type, ... }
		vp.advance() // consume {
		fields, err := vp.parseStructFields()
		if err != nil {
			return Variant{}, err
		}
		// Expect closing brace (already consumed by parseStructFields)
		return Variant{Name: name, Fields: fields}, nil

	default:
		// Unit variant: Name
		return Variant{Name: name, Fields: nil}, nil
	}
}

// parseTupleFields parses tuple variant fields: type1, type2, ...
func (vp *variantParser) parseTupleFields() ([]Field, error) {
	var fields []Field
	fieldIdx := 0

	for {
		vp.skipWhitespace()

		// Check for closing paren
		if vp.peek() == ')' {
			break
		}

		// Parse type
		fieldType, err := vp.parseType()
		if err != nil {
			return nil, err
		}
		if fieldType == "" {
			break
		}

		// Auto-generate field name as index
		fields = append(fields, Field{
			Name: fmt.Sprintf("%d", fieldIdx),
			Type: fieldType,
		})
		fieldIdx++

		vp.skipWhitespace()

		// Check for comma
		if vp.peek() == ',' {
			vp.advance()
		} else {
			break
		}
	}

	return fields, nil
}

// parseStructFields parses struct variant fields: field1: type1, field2: type2, ...
func (vp *variantParser) parseStructFields() ([]Field, error) {
	var fields []Field

	for {
		vp.skipWhitespace()

		// Check for closing brace
		if vp.peek() == '}' {
			vp.advance()
			break
		}

		// Read field name
		fieldName := vp.readIdentifier()
		if fieldName == "" {
			break
		}

		vp.skipWhitespace()

		// Expect colon
		if vp.peek() != ':' {
			return nil, fmt.Errorf("expected ':' after field name '%s'", fieldName)
		}
		vp.advance()

		vp.skipWhitespace()

		// Parse type
		fieldType, err := vp.parseType()
		if err != nil {
			return nil, err
		}

		fields = append(fields, Field{
			Name: fieldName,
			Type: fieldType,
		})

		vp.skipWhitespace()

		// Check for comma
		if vp.peek() == ',' {
			vp.advance()
		}
	}

	return fields, nil
}

// parseType parses a Go type expression
// Handles: basic types, pointers, arrays, slices, maps, channels, functions, structs, generics
func (vp *variantParser) parseType() (string, error) {
	var buf bytes.Buffer
	vp.skipWhitespace()

	// Handle pointer: *Type, **Type
	for vp.peek() == '*' {
		buf.WriteByte('*')
		vp.advance()
	}

	// Handle array/slice: []Type, [10]Type
	if vp.peek() == '[' {
		buf.WriteByte('[')
		vp.advance()

		// Read size or close bracket
		for vp.peek() != ']' && vp.pos < len(vp.source) {
			buf.WriteByte(vp.peek())
			vp.advance()
		}

		if vp.peek() == ']' {
			buf.WriteByte(']')
			vp.advance()
		}

		// Parse element type recursively
		elemType, err := vp.parseType()
		if err != nil {
			return "", err
		}
		buf.WriteString(elemType)
		return buf.String(), nil
	}

	// Handle map: map[K]V
	if vp.matchKeyword("map") {
		buf.WriteString("map")

		if vp.peek() == '[' {
			buf.WriteByte('[')
			vp.advance()

			// Parse key type
			keyType, err := vp.parseType()
			if err != nil {
				return "", err
			}
			buf.WriteString(keyType)

			if vp.peek() == ']' {
				buf.WriteByte(']')
				vp.advance()
			}

			// Parse value type
			valType, err := vp.parseType()
			if err != nil {
				return "", err
			}
			buf.WriteString(valType)
		}

		return buf.String(), nil
	}

	// Handle chan: chan T, <-chan T, chan<- T
	if vp.peek() == '<' && vp.peekN(1) == '-' {
		buf.WriteString("<-")
		vp.advance()
		vp.advance()
	}

	if vp.matchKeyword("chan") {
		buf.WriteString("chan")

		if vp.peek() == '<' && vp.peekN(1) == '-' {
			buf.WriteString("<-")
			vp.advance()
			vp.advance()
		}

		buf.WriteByte(' ')

		// Parse element type
		elemType, err := vp.parseType()
		if err != nil {
			return "", err
		}
		buf.WriteString(elemType)
		return buf.String(), nil
	}

	// Handle func: func(args) returnType
	if vp.matchKeyword("func") {
		buf.WriteString("func")

		// Parse signature
		if vp.peek() == '(' {
			depth := 1
			buf.WriteByte('(')
			vp.advance()

			for depth > 0 && vp.pos < len(vp.source) {
				ch := vp.peek()
				if ch == '(' {
					depth++
				} else if ch == ')' {
					depth--
				}
				buf.WriteByte(ch)
				vp.advance()
			}

			// Parse return type if present
			vp.skipWhitespace()
			if vp.pos < len(vp.source) && vp.peek() != ',' && vp.peek() != '}' && vp.peek() != ')' {
				buf.WriteByte(' ')

				// Check for multiple return values: (Type1, Type2)
				if vp.peek() == '(' {
					depth := 1
					buf.WriteByte('(')
					vp.advance()

					for depth > 0 && vp.pos < len(vp.source) {
						ch := vp.peek()
						if ch == '(' {
							depth++
						} else if ch == ')' {
							depth--
						}
						buf.WriteByte(ch)
						vp.advance()
					}
				} else {
					// Single return type
					retType, err := vp.parseType()
					if err != nil {
						return "", err
					}
					buf.WriteString(retType)
				}
			}
		}

		return buf.String(), nil
	}

	// Handle struct/interface inline definitions
	vp.skipWhitespace()
	structStart := vp.pos
	if vp.matchKeyword("struct") {
		buf.WriteString("struct")

		vp.skipWhitespace()
		if vp.peek() == '{' {
			depth := 1
			buf.WriteByte('{')
			vp.advance()

			for depth > 0 && vp.pos < len(vp.source) {
				ch := vp.peek()
				if ch == '{' {
					depth++
				} else if ch == '}' {
					depth--
				}
				buf.WriteByte(ch)
				vp.advance()
			}
		}

		return buf.String(), nil
	}

	// Reset position if struct didn't match
	vp.pos = structStart

	if vp.matchKeyword("interface") {
		buf.WriteString("interface")

		vp.skipWhitespace()
		if vp.peek() == '{' {
			depth := 1
			buf.WriteByte('{')
			vp.advance()

			for depth > 0 && vp.pos < len(vp.source) {
				ch := vp.peek()
				if ch == '{' {
					depth++
				} else if ch == '}' {
					depth--
				}
				buf.WriteByte(ch)
				vp.advance()
			}
		}

		return buf.String(), nil
	}

	// Reset position if interface didn't match
	vp.pos = structStart

	// Handle basic type: TypeName or pkg.TypeName
	ident := vp.readIdentifier()
	if ident == "" {
		return "", nil
	}
	buf.WriteString(ident)

	// Check for qualified type: pkg.Type
	if vp.peek() == '.' {
		buf.WriteByte('.')
		vp.advance()

		subIdent := vp.readIdentifier()
		buf.WriteString(subIdent)
	}

	// Handle generics: Type<T1, T2> → Type[T1, T2]
	// Or keep angle brackets if they're part of the syntax
	if vp.peek() == '<' {
		buf.WriteByte('[')
		vp.advance()

		depth := 1
		for depth > 0 && vp.pos < len(vp.source) {
			ch := vp.peek()
			if ch == '<' {
				depth++
				buf.WriteByte('[')
				vp.advance()
			} else if ch == '>' {
				depth--
				if depth > 0 {
					buf.WriteByte(']')
				}
				vp.advance()
			} else if ch == ',' {
				buf.WriteByte(',')
				buf.WriteByte(' ')
				vp.advance()
				vp.skipWhitespace()
			} else if ch == ' ' || ch == '\t' || ch == '\n' {
				// Skip whitespace
				vp.advance()
			} else {
				// Parse nested type recursively
				startPos := vp.pos
				nestedType, err := vp.parseType()
				if err != nil {
					return "", err
				}
				if nestedType != "" {
					buf.WriteString(nestedType)
				}
				// Ensure we always advance if parseType didn't
				if vp.pos == startPos {
					// Unknown character - skip it to avoid infinite loop
					buf.WriteByte(ch)
					vp.advance()
				}
			}
		}
		buf.WriteByte(']')
	}

	// Handle square bracket generics: Type[T1, T2] (Go-style)
	if vp.peek() == '[' {
		buf.WriteByte('[')
		vp.advance()

		depth := 1
		for depth > 0 && vp.pos < len(vp.source) {
			ch := vp.peek()
			if ch == '[' {
				depth++
				buf.WriteByte('[')
				vp.advance()
			} else if ch == ']' {
				depth--
				if depth > 0 {
					buf.WriteByte(']')
				}
				vp.advance()
			} else if ch == ',' {
				buf.WriteByte(',')
				buf.WriteByte(' ')
				vp.advance()
				vp.skipWhitespace()
			} else if ch == ' ' || ch == '\t' || ch == '\n' {
				// Skip whitespace
				vp.advance()
			} else {
				// Parse nested type recursively
				startPos := vp.pos
				nestedType, err := vp.parseType()
				if err != nil {
					return "", err
				}
				if nestedType != "" {
					buf.WriteString(nestedType)
				}
				// Ensure we always advance if parseType didn't
				if vp.pos == startPos {
					// Unknown character - skip it to avoid infinite loop
					buf.WriteByte(ch)
					vp.advance()
				}
			}
		}
		buf.WriteByte(']')
	}

	return buf.String(), nil
}

// generateSumType generates Go sum type code from enum definition
func (p *EnumASTProcessor) generateSumType(enumName string, variants []Variant, marker string) string {
	var buf bytes.Buffer

	// Write marker
	buf.WriteString(marker)
	buf.WriteByte('\n')

	// 1. Generate tag type
	tagTypeName := fmt.Sprintf("%sTag", enumName)
	buf.WriteString(fmt.Sprintf("type %s uint8\n\n", tagTypeName))

	// 2. Generate tag constants
	buf.WriteString("const (\n")
	for i, variant := range variants {
		tagConstName := fmt.Sprintf("%s%s", tagTypeName, variant.Name)
		if i == 0 {
			buf.WriteString(fmt.Sprintf("\t%s %s = iota\n", tagConstName, tagTypeName))
		} else {
			buf.WriteString(fmt.Sprintf("\t%s\n", tagConstName))
		}
	}
	buf.WriteString(")\n\n")

	// 3. Generate struct with tag and fields
	buf.WriteString(fmt.Sprintf("type %s struct {\n", enumName))
	buf.WriteString("\ttag " + tagTypeName + "\n")

	// Collect all fields from all variants
	fieldMap := make(map[string]string) // fieldName -> fieldType

	for _, variant := range variants {
		if len(variant.Fields) > 0 {
			// Determine field naming strategy
			isSingleTupleVariant := len(variant.Fields) == 1 &&
				len(variant.Fields[0].Name) > 0 &&
				variant.Fields[0].Name[0] >= '0' &&
				variant.Fields[0].Name[0] <= '9'

			for fieldIdx, field := range variant.Fields {
				var fieldName string
				isTupleField := len(field.Name) > 0 && field.Name[0] >= '0' && field.Name[0] <= '9'

				if isSingleTupleVariant {
					// Single tuple field - use variant name
					fieldName = strings.ToLower(variant.Name)
				} else if isTupleField {
					// Multiple tuple fields
					baseName := strings.ToLower(variant.Name)
					if fieldIdx == 0 {
						fieldName = baseName
					} else {
						fieldName = fmt.Sprintf("%s%d", baseName, fieldIdx)
					}
				} else {
					// Struct variant with named fields
					fieldName = strings.ToLower(variant.Name) + "_" + field.Name
				}

				fieldMap[fieldName] = field.Type
			}
		}
	}

	// Generate fields in alphabetical order
	var fieldNames []string
	for name := range fieldMap {
		fieldNames = append(fieldNames, name)
	}
	sort.Strings(fieldNames)

	for _, fieldName := range fieldNames {
		fieldType := fieldMap[fieldName]
		buf.WriteString(fmt.Sprintf("\t%s *%s\n", fieldName, fieldType))
	}

	buf.WriteString("}\n\n")

	// 4. Generate constructor functions
	for _, variant := range variants {
		constructorName := fmt.Sprintf("%s%s", enumName, variant.Name)
		tagConstName := fmt.Sprintf("%s%s", tagTypeName, variant.Name)

		if len(variant.Fields) == 0 {
			// Unit variant constructor
			buf.WriteString(fmt.Sprintf("func %s() %s {\n", constructorName, enumName))
			buf.WriteString(fmt.Sprintf("\treturn %s{tag: %s}\n", enumName, tagConstName))
			buf.WriteString("}\n")
		} else {
			// Variant with fields
			params := []string{}
			assignments := []string{}

			isSingleTupleVariant := len(variant.Fields) == 1 &&
				len(variant.Fields[0].Name) > 0 &&
				variant.Fields[0].Name[0] >= '0' &&
				variant.Fields[0].Name[0] <= '9'

			for fieldIdx, field := range variant.Fields {
				// Determine parameter name
				paramName := field.Name
				isTupleField := len(field.Name) > 0 && field.Name[0] >= '0' && field.Name[0] <= '9'
				if isTupleField {
					paramName = "arg" + field.Name
				}

				// Determine field name (same logic as struct generation)
				var fieldName string
				if isSingleTupleVariant {
					fieldName = strings.ToLower(variant.Name)
				} else if isTupleField {
					baseName := strings.ToLower(variant.Name)
					if fieldIdx == 0 {
						fieldName = baseName
					} else {
						fieldName = fmt.Sprintf("%s%d", baseName, fieldIdx)
					}
				} else {
					fieldName = strings.ToLower(variant.Name) + "_" + field.Name
				}

				params = append(params, fmt.Sprintf("%s %s", paramName, field.Type))
				assignments = append(assignments, fmt.Sprintf("%s: &%s", fieldName, paramName))
			}

			buf.WriteString(fmt.Sprintf("func %s(%s) %s {\n",
				constructorName, strings.Join(params, ", "), enumName))
			buf.WriteString(fmt.Sprintf("\treturn %s{tag: %s, %s}\n",
				enumName, tagConstName, strings.Join(assignments, ", ")))
			buf.WriteString("}\n")
		}
	}

	// 5. Generate Is* methods
	for _, variant := range variants {
		tagConstName := fmt.Sprintf("%s%s", tagTypeName, variant.Name)
		buf.WriteString(fmt.Sprintf("func (e %s) Is%s() bool {\n", enumName, variant.Name))
		buf.WriteString(fmt.Sprintf("\treturn e.tag == %s\n", tagConstName))
		buf.WriteString("}\n")
	}

	// 6. Generate helper methods for Option/Result-like enums
	p.generateHelperMethods(&buf, enumName, tagTypeName, variants)

	return buf.String()
}

// generateHelperMethods generates Map and AndThen methods for Option/Result-like enums
func (p *EnumASTProcessor) generateHelperMethods(buf *bytes.Buffer, enumName, tagTypeName string, variants []Variant) {
	// Detect if this is an Option or Result type
	isOption := p.hasVariants(variants, []string{"Some", "None"})
	isResult := p.hasVariants(variants, []string{"Ok", "Err"})

	if !isOption && !isResult {
		return
	}

	if isOption {
		p.generateOptionHelpers(buf, enumName, tagTypeName, variants)
	}

	if isResult {
		p.generateResultHelpers(buf, enumName, tagTypeName, variants)
	}
}

// hasVariants checks if enum has specific variant names
func (p *EnumASTProcessor) hasVariants(variants []Variant, names []string) bool {
	found := make(map[string]bool)
	for _, v := range variants {
		for _, name := range names {
			if v.Name == name {
				found[name] = true
			}
		}
	}
	return len(found) == len(names)
}

// generateOptionHelpers generates Map and AndThen for Option types
func (p *EnumASTProcessor) generateOptionHelpers(buf *bytes.Buffer, enumName, tagTypeName string, variants []Variant) {
	// Find the Some variant
	var someVariant *Variant
	for i := range variants {
		if variants[i].Name == "Some" {
			someVariant = &variants[i]
			break
		}
	}

	if someVariant == nil || len(someVariant.Fields) == 0 {
		return
	}

	valueType := someVariant.Fields[0].Type
	fieldName := "some"

	// Map method
	buf.WriteString("\n")
	buf.WriteString(fmt.Sprintf("func (o %s) Map(fn func(%s) %s) %s {\n", enumName, valueType, valueType, enumName))
	buf.WriteString("\tswitch o.tag {\n")
	buf.WriteString(fmt.Sprintf("\tcase %sSome:\n", tagTypeName))
	buf.WriteString(fmt.Sprintf("\t\tif o.%s != nil {\n", fieldName))
	buf.WriteString(fmt.Sprintf("\t\t\treturn %sSome(fn(*o.%s))\n", enumName, fieldName))
	buf.WriteString("\t\t}\n")
	buf.WriteString(fmt.Sprintf("\tcase %sNone:\n", tagTypeName))
	buf.WriteString("\t\treturn o\n")
	buf.WriteString("\t}\n")
	buf.WriteString(fmt.Sprintf("\tpanic(\"invalid %s state\")\n", enumName))
	buf.WriteString("}\n")

	// AndThen method
	buf.WriteString("\n")
	buf.WriteString(fmt.Sprintf("func (o %s) AndThen(fn func(%s) %s) %s {\n", enumName, valueType, enumName, enumName))
	buf.WriteString("\tswitch o.tag {\n")
	buf.WriteString(fmt.Sprintf("\tcase %sSome:\n", tagTypeName))
	buf.WriteString(fmt.Sprintf("\t\tif o.%s != nil {\n", fieldName))
	buf.WriteString(fmt.Sprintf("\t\t\treturn fn(*o.%s)\n", fieldName))
	buf.WriteString("\t\t}\n")
	buf.WriteString(fmt.Sprintf("\tcase %sNone:\n", tagTypeName))
	buf.WriteString("\t\treturn o\n")
	buf.WriteString("\t}\n")
	buf.WriteString(fmt.Sprintf("\tpanic(\"invalid %s state\")\n", enumName))
	buf.WriteString("}\n")

	// Unwrap method
	buf.WriteString("\n")
	buf.WriteString(fmt.Sprintf("func (o %s) Unwrap() %s {\n", enumName, valueType))
	buf.WriteString(fmt.Sprintf("\tif o.tag != %sSome {\n", tagTypeName))
	buf.WriteString("\t\tpanic(\"called Unwrap on None\")\n")
	buf.WriteString("\t}\n")
	buf.WriteString(fmt.Sprintf("\treturn *o.%s\n", fieldName))
	buf.WriteString("}\n")
}

// generateResultHelpers generates Map and AndThen for Result types
func (p *EnumASTProcessor) generateResultHelpers(buf *bytes.Buffer, enumName, tagTypeName string, variants []Variant) {
	// Find Ok and Err variants
	var okVariant, errVariant *Variant
	for i := range variants {
		if variants[i].Name == "Ok" {
			okVariant = &variants[i]
		} else if variants[i].Name == "Err" {
			errVariant = &variants[i]
		}
	}

	if okVariant == nil || errVariant == nil || len(okVariant.Fields) == 0 || len(errVariant.Fields) == 0 {
		return
	}

	okType := okVariant.Fields[0].Type
	okFieldName := "ok"

	// Map method
	buf.WriteString("\n")
	buf.WriteString(fmt.Sprintf("func (r %s) Map(fn func(%s) %s) %s {\n", enumName, okType, okType, enumName))
	buf.WriteString("\tswitch r.tag {\n")
	buf.WriteString(fmt.Sprintf("\tcase %sOk:\n", tagTypeName))
	buf.WriteString(fmt.Sprintf("\t\tif r.%s != nil {\n", okFieldName))
	buf.WriteString(fmt.Sprintf("\t\t\treturn %sOk(fn(*r.%s))\n", enumName, okFieldName))
	buf.WriteString("\t\t}\n")
	buf.WriteString(fmt.Sprintf("\tcase %sErr:\n", tagTypeName))
	buf.WriteString("\t\treturn r\n")
	buf.WriteString("\t}\n")
	buf.WriteString(fmt.Sprintf("\tpanic(\"invalid %s state\")\n", enumName))
	buf.WriteString("}\n")

	// AndThen method
	buf.WriteString("\n")
	buf.WriteString(fmt.Sprintf("func (r %s) AndThen(fn func(%s) %s) %s {\n", enumName, okType, enumName, enumName))
	buf.WriteString("\tswitch r.tag {\n")
	buf.WriteString(fmt.Sprintf("\tcase %sOk:\n", tagTypeName))
	buf.WriteString(fmt.Sprintf("\t\tif r.%s != nil {\n", okFieldName))
	buf.WriteString(fmt.Sprintf("\t\t\treturn fn(*r.%s)\n", okFieldName))
	buf.WriteString("\t\t}\n")
	buf.WriteString(fmt.Sprintf("\tcase %sErr:\n", tagTypeName))
	buf.WriteString("\t\treturn r\n")
	buf.WriteString("\t}\n")
	buf.WriteString(fmt.Sprintf("\tpanic(\"invalid %s state\")\n", enumName))
	buf.WriteString("}\n")
}

// Helper methods for parsing

func (p *EnumASTProcessor) peek() byte {
	if p.pos >= len(p.source) {
		return 0
	}
	return p.source[p.pos]
}

func (p *EnumASTProcessor) peekN(n int) byte {
	pos := p.pos + n
	if pos >= len(p.source) {
		return 0
	}
	return p.source[pos]
}

func (p *EnumASTProcessor) advance() {
	if p.pos < len(p.source) {
		if p.source[p.pos] == '\n' {
			p.line++
			p.col = 1
		} else {
			p.col++
		}
		p.pos++
	}
}

func (p *EnumASTProcessor) skipWhitespace() {
	for p.pos < len(p.source) {
		ch := p.peek()
		if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
			p.advance()
		} else {
			break
		}
	}
}

func (p *EnumASTProcessor) skipComment() {
	if p.peek() == '/' && p.peekN(1) == '/' {
		// Line comment
		for p.pos < len(p.source) && p.peek() != '\n' {
			p.advance()
		}
		if p.peek() == '\n' {
			p.advance()
		}
	} else if p.peek() == '/' && p.peekN(1) == '*' {
		// Block comment
		p.advance() // /
		p.advance() // *
		for p.pos < len(p.source) {
			if p.peek() == '*' && p.peekN(1) == '/' {
				p.advance() // *
				p.advance() // /
				break
			}
			p.advance()
		}
	}
}

func (p *EnumASTProcessor) skipString() {
	delimiter := p.peek()
	p.advance() // opening quote

	for p.pos < len(p.source) {
		ch := p.peek()
		if ch == delimiter {
			p.advance()
			break
		}
		if ch == '\\' && delimiter == '"' {
			p.advance() // skip backslash
			if p.pos < len(p.source) {
				p.advance() // skip escaped char
			}
		} else {
			p.advance()
		}
	}
}

func (p *EnumASTProcessor) matchKeyword(keyword string) bool {
	// Check if we have enough characters
	if p.pos+len(keyword) > len(p.source) {
		return false
	}

	// Check if keyword matches
	if string(p.source[p.pos:p.pos+len(keyword)]) != keyword {
		return false
	}

	// Check if followed by non-identifier character
	nextPos := p.pos + len(keyword)
	if nextPos < len(p.source) {
		nextCh := p.source[nextPos]
		if isIdentChar(nextCh) {
			return false
		}
	}

	// Check if preceded by non-identifier character
	if p.pos > 0 {
		prevCh := p.source[p.pos-1]
		if isIdentChar(prevCh) {
			return false
		}
	}

	// Consume keyword
	for i := 0; i < len(keyword); i++ {
		p.advance()
	}

	return true
}

func (p *EnumASTProcessor) readIdentifier() string {
	start := p.pos

	if p.pos >= len(p.source) || !isIdentStart(p.peek()) {
		return ""
	}

	p.advance()

	for p.pos < len(p.source) && isIdentChar(p.peek()) {
		p.advance()
	}

	return string(p.source[start:p.pos])
}

func (p *EnumASTProcessor) findMatchingBrace() int {
	depth := 1
	startPos := p.pos

	for p.pos < len(p.source) {
		// Skip strings
		if p.peek() == '"' || p.peek() == '`' {
			p.skipString()
			continue
		}

		// Skip comments
		if p.peek() == '/' && (p.peekN(1) == '/' || p.peekN(1) == '*') {
			p.skipComment()
			continue
		}

		ch := p.peek()
		if ch == '{' {
			depth++
		} else if ch == '}' {
			depth--
			if depth == 0 {
				endPos := p.pos
				p.pos = startPos // reset position for caller
				return endPos
			}
		}
		p.advance()
	}

	p.pos = startPos // reset on failure
	return -1
}

// Helper methods for variantParser

func (vp *variantParser) peek() byte {
	if vp.pos >= len(vp.source) {
		return 0
	}
	return vp.source[vp.pos]
}

func (vp *variantParser) peekN(n int) byte {
	pos := vp.pos + n
	if pos >= len(vp.source) {
		return 0
	}
	return vp.source[pos]
}

func (vp *variantParser) advance() {
	if vp.pos < len(vp.source) {
		vp.pos++
	}
}

func (vp *variantParser) skipWhitespace() {
	for vp.pos < len(vp.source) {
		ch := vp.peek()
		if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
			vp.advance()
		} else {
			break
		}
	}
}

func (vp *variantParser) skipComments() {
	for {
		if vp.peek() == '/' && vp.peekN(1) == '/' {
			// Line comment
			for vp.pos < len(vp.source) && vp.peek() != '\n' {
				vp.advance()
			}
			if vp.peek() == '\n' {
				vp.advance()
			}
		} else if vp.peek() == '/' && vp.peekN(1) == '*' {
			// Block comment
			vp.advance() // /
			vp.advance() // *
			for vp.pos < len(vp.source) {
				if vp.peek() == '*' && vp.peekN(1) == '/' {
					vp.advance() // *
					vp.advance() // /
					break
				}
				vp.advance()
			}
		} else {
			break
		}
	}
}

func (vp *variantParser) matchKeyword(keyword string) bool {
	if vp.pos+len(keyword) > len(vp.source) {
		return false
	}

	if string(vp.source[vp.pos:vp.pos+len(keyword)]) != keyword {
		return false
	}

	nextPos := vp.pos + len(keyword)
	if nextPos < len(vp.source) {
		nextCh := vp.source[nextPos]
		if isIdentChar(nextCh) {
			return false
		}
	}

	for i := 0; i < len(keyword); i++ {
		vp.advance()
	}

	return true
}

func (vp *variantParser) readIdentifier() string {
	start := vp.pos

	if vp.pos >= len(vp.source) || !isIdentStart(vp.peek()) {
		return ""
	}

	vp.advance()

	for vp.pos < len(vp.source) && isIdentChar(vp.peek()) {
		vp.advance()
	}

	return string(vp.source[start:vp.pos])
}

// Character classification helpers

func isIdentStart(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_'
}

func isIdentChar(ch byte) bool {
	return isIdentStart(ch) || (ch >= '0' && ch <= '9')
}
