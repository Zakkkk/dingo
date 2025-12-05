package ast

import (
	"bytes"
	"fmt"
	"strings"
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

	// 1. Generate interface type with Is* methods
	g.buf.WriteString("type ")
	g.buf.WriteString(enumName)
	g.buf.WriteString(" interface { ")
	g.buf.WriteString(interfaceMethod)
	g.buf.WriteString("()")
	// Add Is* methods to interface
	for _, variant := range decl.Variants {
		g.buf.WriteString("; Is")
		g.buf.WriteString(variant.Name.Name)
		g.buf.WriteString("() bool")
	}
	g.buf.WriteString(" }\n\n")

	// 2. Generate variant structs, methods, and constructors
	for _, variant := range decl.Variants {
		g.generateVariant(enumName, interfaceMethod, variant)
	}

	// 3. Generate Is* standalone functions for type checking
	for _, variant := range decl.Variants {
		g.generateIsMethod(enumName, variant)
	}

	// 4. Generate Is* methods on each variant struct for user-friendly syntax
	g.generateIsMethodsOnVariants(enumName, decl.Variants)

	return g.buf.Bytes()
}

// generateVariant generates struct, interface method, and constructor for one variant
func (g *EnumCodeGen) generateVariant(enumName, interfaceMethod string, variant *EnumVariant) {
	structName := enumName + variant.Name.Name

	// Struct definition
	g.buf.WriteString("type ")
	g.buf.WriteString(structName)
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
	g.buf.WriteString(") ")
	g.buf.WriteString(interfaceMethod)
	g.buf.WriteString("() {}\n")

	// Constructor function
	g.generateConstructor(enumName, variant)
	g.buf.WriteString("\n")
}

// generateConstructor generates a constructor function for a variant
func (g *EnumCodeGen) generateConstructor(enumName string, variant *EnumVariant) {
	structName := enumName + variant.Name.Name
	constructorName := "New" + structName

	g.buf.WriteString("func ")
	g.buf.WriteString(constructorName)
	g.buf.WriteString("(")

	// Parameters
	for i, field := range variant.Fields {
		if i > 0 {
			g.buf.WriteString(", ")
		}
		fieldName := g.getFieldName(variant, field, i)
		g.buf.WriteString(fieldName)
		g.buf.WriteString(" ")
		g.buf.WriteString(field.Type.Text)
	}

	g.buf.WriteString(") ")
	g.buf.WriteString(enumName)
	g.buf.WriteString(" { return ")
	g.buf.WriteString(structName)
	g.buf.WriteString("{")

	// Field initializers
	for i, field := range variant.Fields {
		if i > 0 {
			g.buf.WriteString(", ")
		}
		fieldName := g.getFieldName(variant, field, i)
		g.buf.WriteString(fieldName)
		g.buf.WriteString(": ")
		g.buf.WriteString(fieldName)
	}

	g.buf.WriteString("} }\n")
}

// generateIsMethod generates Is* helper functions for type checking.
// Two versions are generated:
// 1. A standalone function: IsStatusActive(v Status) bool
// 2. A method on the interface via type switch (can't have methods on interface,
//    so we add methods on the concrete types that implement the interface)
func (g *EnumCodeGen) generateIsMethod(enumName string, variant *EnumVariant) {
	structName := enumName + variant.Name.Name
	funcName := "Is" + enumName + variant.Name.Name

	// Standalone function version
	g.buf.WriteString("func ")
	g.buf.WriteString(funcName)
	g.buf.WriteString("(v ")
	g.buf.WriteString(enumName)
	g.buf.WriteString(") bool { _, ok := v.(")
	g.buf.WriteString(structName)
	g.buf.WriteString("); return ok }\n")
}

// generateIsMethodsOnVariants generates Is* methods on each concrete variant struct.
// This enables the user-friendly syntax: s.IsActive()
func (g *EnumCodeGen) generateIsMethodsOnVariants(enumName string, variants []*EnumVariant) {
	// For each variant struct, generate Is* methods for ALL variants
	for _, receiverVariant := range variants {
		receiverStructName := enumName + receiverVariant.Name.Name
		for _, targetVariant := range variants {
			methodName := "Is" + targetVariant.Name.Name
			isMatch := receiverVariant.Name.Name == targetVariant.Name.Name

			g.buf.WriteString("func (")
			g.buf.WriteString(receiverStructName)
			g.buf.WriteString(") ")
			g.buf.WriteString(methodName)
			g.buf.WriteString("() bool { return ")
			if isMatch {
				g.buf.WriteString("true")
			} else {
				g.buf.WriteString("false")
			}
			g.buf.WriteString(" }\n")
		}
	}
}

// getFieldName returns the appropriate field name for a variant field
func (g *EnumCodeGen) getFieldName(variant *EnumVariant, field *EnumField, index int) string {
	// Struct variant with named fields
	if field.Name != nil {
		return field.Name.Name
	}

	// Tuple variant - use lowercase variant name or indexed name
	baseName := strings.ToLower(variant.Name.Name)
	if len(variant.Fields) == 1 {
		return baseName
	}
	if index == 0 {
		return baseName
	}
	return fmt.Sprintf("%s%d", baseName, index)
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
		for _, v := range decl.Variants {
			registry[v.Name.Name] = decl.Name.Name
			registry[decl.Name.Name+v.Name.Name] = decl.Name.Name
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
