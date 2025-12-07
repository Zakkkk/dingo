package ast

import (
	"bytes"
	"fmt"
)

// EnumCodeGen generates Go code from EnumDecl AST nodes.
// This replaces the string-based transformEnum function with proper AST-based generation.
type EnumCodeGen struct {
	buf bytes.Buffer
}

// NewEnumCodeGen creates a new enum code generator.
func NewEnumCodeGen() *EnumCodeGen {
	return &EnumCodeGen{}
}

// Generate produces Go code for an EnumDecl.
// Returns the generated Go code as bytes.
func (g *EnumCodeGen) Generate(decl *EnumDecl) []byte {
	g.buf.Reset()

	enumName := decl.Name.Name
	interfaceMethod := "is" + enumName
	typeParams := g.getTypeParams(decl)

	// 1. Generate interface type with unexported marker method
	g.buf.WriteString("type ")
	g.buf.WriteString(enumName)
	g.buf.WriteString(typeParams)
	g.buf.WriteString(" interface { ")
	g.buf.WriteString(interfaceMethod)
	g.buf.WriteString("() }\n\n")

	// 2. Generate variant structs, marker methods, and constructors
	for _, variant := range decl.Variants {
		g.generateVariant(enumName, interfaceMethod, typeParams, variant)
	}

	return g.buf.Bytes()
}

// getTypeParams returns the type parameters string (e.g., "[T, E any]") or empty string
func (g *EnumCodeGen) getTypeParams(decl *EnumDecl) string {
	if decl.TypeParams == nil || len(decl.TypeParams.Params) == 0 {
		return ""
	}

	var buf bytes.Buffer
	buf.WriteString("[")
	for i, param := range decl.TypeParams.Params {
		if i > 0 {
			buf.WriteString(", ")
		}
		buf.WriteString(param.Name)
	}
	buf.WriteString(" any]")
	return buf.String()
}

// generateVariant generates struct, interface method, and constructor for one variant
func (g *EnumCodeGen) generateVariant(enumName, interfaceMethod, typeParams string, variant *EnumVariant) {
	structName := enumName + variant.Name.Name

	// Struct definition
	g.buf.WriteString("type ")
	g.buf.WriteString(structName)
	g.buf.WriteString(typeParams)
	g.buf.WriteString(" struct {")

	if len(variant.Fields) > 0 {
		for i, field := range variant.Fields {
			if i > 0 {
				g.buf.WriteString("; ")
			}

			fieldName := g.getFieldName(variant, field, i)
			g.buf.WriteString(fieldName)
			g.buf.WriteString(" ")
			g.buf.WriteString(field.Type.Text)
		}
	}
	g.buf.WriteString("}\n")

	// Interface method
	g.buf.WriteString("func (")
	g.buf.WriteString(structName)
	g.buf.WriteString(typeParams)
	g.buf.WriteString(") ")
	g.buf.WriteString(interfaceMethod)
	g.buf.WriteString("() {}\n")

	// Constructor function
	g.generateConstructor(enumName, typeParams, variant)
	g.buf.WriteString("\n")
}

// generateConstructor generates a constructor function for a variant
func (g *EnumCodeGen) generateConstructor(enumName, typeParams string, variant *EnumVariant) {
	structName := enumName + variant.Name.Name
	constructorName := "New" + enumName + variant.Name.Name

	g.buf.WriteString("func ")
	g.buf.WriteString(constructorName)
	g.buf.WriteString(typeParams)
	g.buf.WriteString("(")

	// Parameters
	for i, field := range variant.Fields {
		if i > 0 {
			g.buf.WriteString(", ")
		}
		paramName := g.getParameterName(variant, field, i)
		g.buf.WriteString(paramName)
		g.buf.WriteString(" ")
		g.buf.WriteString(field.Type.Text)
	}

	g.buf.WriteString(") ")
	g.buf.WriteString(enumName)
	g.buf.WriteString(typeParams)
	g.buf.WriteString(" { return ")
	g.buf.WriteString(structName)
	g.buf.WriteString(typeParams)
	g.buf.WriteString("{")

	// Field initializers
	for i, field := range variant.Fields {
		if i > 0 {
			g.buf.WriteString(", ")
		}
		fieldName := g.getFieldName(variant, field, i)
		paramName := g.getParameterName(variant, field, i)
		g.buf.WriteString(fieldName)
		g.buf.WriteString(": ")
		g.buf.WriteString(paramName)
	}

	g.buf.WriteString("} }\n")
}

// getFieldName returns the appropriate field name for a variant field (struct field, uppercase)
func (g *EnumCodeGen) getFieldName(variant *EnumVariant, field *EnumField, index int) string {
	// Struct variant with named fields
	if field.Name != nil {
		return field.Name.Name
	}

	// Tuple variant - use "Value" for single field, "Value0", "Value1" for multiple
	if len(variant.Fields) == 1 {
		return "Value"
	}
	return fmt.Sprintf("Value%d", index)
}

// getParameterName returns the appropriate parameter name for a constructor parameter (lowercase)
func (g *EnumCodeGen) getParameterName(variant *EnumVariant, field *EnumField, index int) string {
	// Struct variant with named fields - use field name as-is
	if field.Name != nil {
		return field.Name.Name
	}

	// Tuple variant - use "value" for single field, "value0", "value1" for multiple
	if len(variant.Fields) == 1 {
		return "value"
	}
	return fmt.Sprintf("value%d", index)
}

// ExtractEnumRegistry extracts the enum registry from Dingo source without transforming it.
// This is useful when you need the registry for match expressions but don't want to
// re-transform the enum declarations.
func ExtractEnumRegistry(src []byte) map[string]string {
	enumPositions := FindEnumDeclarations(src)
	if len(enumPositions) == 0 {
		return nil
	}

	registry := make(map[string]string)

	for _, enumStart := range enumPositions {
		parser := NewEnumParser(src[enumStart:], enumStart)
		decl, _, err := parser.ParseEnumDecl()
		if err != nil {
			continue
		}

		// Register variants for match expression lookup
		for _, v := range decl.Variants {
			registry[v.Name.Name] = decl.Name.Name
		}
	}

	return registry
}

// TransformEnumSource transforms Dingo source containing enums to Go source.
// This is the main entry point that replaces the old regex-based transformEnum.
func TransformEnumSource(src []byte) ([]byte, map[string]string) {
	enumPositions := FindEnumDeclarations(src)
	if len(enumPositions) == 0 {
		return src, nil
	}

	// Registry maps variant names to enum names for match expression support
	registry := make(map[string]string)

	result := make([]byte, 0, len(src)+500)
	lastPos := 0

	for _, enumStart := range enumPositions {
		// Copy source before this enum
		result = append(result, src[lastPos:enumStart]...)

		// Parse the enum
		parser := NewEnumParser(src[enumStart:], enumStart)
		decl, endOffset, err := parser.ParseEnumDecl()
		if err != nil {
			// Parsing failed, keep original source
			result = append(result, src[enumStart:enumStart+4]...)
			lastPos = enumStart + 4
			continue
		}

		// Register variants for match expression lookup
		// ONLY register the bare variant name, NOT the struct name
		for _, v := range decl.Variants {
			registry[v.Name.Name] = decl.Name.Name
			// DO NOT register struct name (EnumName+VariantName) as it causes
			// double-prefix bug when transformer matches generated struct literals
			// registry[decl.Name.Name+v.Name.Name] = decl.Name.Name  // REMOVED
		}

		// Generate Go code
		codegen := NewEnumCodeGen()
		goCode := codegen.Generate(decl)
		result = append(result, goCode...)

		lastPos = enumStart + endOffset
	}

	// Copy remaining source
	result = append(result, src[lastPos:]...)

	return result, registry
}
