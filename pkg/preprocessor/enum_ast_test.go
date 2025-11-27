package preprocessor

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// TestEnumASTProcessor_NestedBraces tests the fix for nested braces bug
// Bug: Variant { map: map[string]struct{} } broke regex-based parser
func TestEnumASTProcessor_NestedBraces(t *testing.T) {
	source := `package main

enum Container {
	Empty,
	MapVariant { data: map[string]struct{} },
	NestedStruct { cfg: struct{ inner struct{} } },
}
`

	processor := NewEnumASTProcessor()
	result, _, err := processor.Process([]byte(source))
	if err != nil {
		t.Fatalf("Process() failed: %v", err)
	}

	output := string(result)

	// Verify MapVariant field is generated correctly
	if !strings.Contains(output, "mapvariant_data *map[string]struct{}") {
		t.Error("Missing or incorrect mapvariant_data field for nested map type")
	}

	// Verify NestedStruct field is generated correctly
	if !strings.Contains(output, "nestedstruct_cfg *struct{ inner struct{} }") {
		t.Error("Missing or incorrect nestedstruct_cfg field for nested struct type")
	}

	// Verify constructors
	if !strings.Contains(output, "func ContainerMapVariant(data map[string]struct{}) Container") {
		t.Error("Missing ContainerMapVariant constructor")
	}
	if !strings.Contains(output, "func ContainerNestedStruct(cfg struct{ inner struct{} }) Container") {
		t.Error("Missing ContainerNestedStruct constructor")
	}

	// Verify generated code compiles
	fset := token.NewFileSet()
	_, parseErr := parser.ParseFile(fset, "", result, parser.AllErrors)
	if parseErr != nil {
		t.Errorf("Generated code does not compile: %v\nGenerated code:\n%s", parseErr, output)
	}
}

// TestEnumASTProcessor_GenericTypes tests the fix for generic type bug
// Bug: Some(Option<T>) - nested <> not handled by regex
func TestEnumASTProcessor_GenericTypes(t *testing.T) {
	source := `package main

enum NestedGeneric {
	None,
	Single { value: Option[T] },
	Double { value: Result[Option[T], Error] },
	Complex { value: Map[string, List[Option[int]]] },
}
`

	processor := NewEnumASTProcessor()
	result, _, err := processor.Process([]byte(source))
	if err != nil {
		t.Fatalf("Process() failed: %v", err)
	}

	output := string(result)

	// Verify fields with generic types are generated correctly
	// Note: Input uses [] (Go syntax), output should preserve it
	if !strings.Contains(output, "single_value *Option[T]") {
		t.Error("Missing or incorrect single_value field for Option[T]")
	}
	if !strings.Contains(output, "double_value *Result[Option[T], Error]") {
		t.Error("Missing or incorrect double_value field for nested generics")
	}
	if !strings.Contains(output, "complex_value *Map[string, List[Option[int]]]") {
		t.Error("Missing or incorrect complex_value field for deeply nested generics")
	}

	// Verify constructors
	if !strings.Contains(output, "func NestedGenericSingle(value Option[T]) NestedGeneric") {
		t.Error("Missing NestedGenericSingle constructor")
	}
	if !strings.Contains(output, "func NestedGenericDouble(value Result[Option[T], Error]) NestedGeneric") {
		t.Error("Missing NestedGenericDouble constructor")
	}

	// Note: Generated code with unbound generics won't compile, but structure should be correct
}

// TestEnumASTProcessor_CommentsInEnum tests the fix for comment bug
// Bug: Regex captures comments as variant names
func TestEnumASTProcessor_CommentsInEnum(t *testing.T) {
	source := `package main

enum Status {
	// This is the initial state
	Pending,
	// Active processing state
	// with multiple comment lines
	Active,
	/* Block comment variant */
	Complete,
	/* Multi-line
	   block comment */
	Done,
}
`

	processor := NewEnumASTProcessor()
	result, _, err := processor.Process([]byte(source))
	if err != nil {
		t.Fatalf("Process() failed: %v", err)
	}

	output := string(result)

	// Verify only actual variants are generated (not comments)
	expectedVariants := []string{"Pending", "Active", "Complete", "Done"}
	for _, variant := range expectedVariants {
		if !strings.Contains(output, "StatusTag"+variant) {
			t.Errorf("Missing variant constant StatusTag%s", variant)
		}
		if !strings.Contains(output, "func Status"+variant+"() Status") {
			t.Errorf("Missing constructor Status%s", variant)
		}
	}

	// Verify no comment text appears in generated code as variant names
	unexpectedVariants := []string{"This", "with", "Block", "Multi"}
	for _, unexpected := range unexpectedVariants {
		if strings.Contains(output, "StatusTag"+unexpected) {
			t.Errorf("Incorrectly generated variant StatusTag%s from comment", unexpected)
		}
	}

	// Verify generated code compiles
	fset := token.NewFileSet()
	_, parseErr := parser.ParseFile(fset, "", result, parser.AllErrors)
	if parseErr != nil {
		t.Errorf("Generated code does not compile: %v\nGenerated code:\n%s", parseErr, output)
	}
}

// TestEnumASTProcessor_EnumInString tests the fix for string literal bug
// Bug: "enum Status" incorrectly matched by regex
func TestEnumASTProcessor_EnumInString(t *testing.T) {
	source := `package main

const description = "enum Status { Pending, Active }"

func main() {
	println("enum in string")
}

enum RealEnum {
	Variant1,
	Variant2,
}
`

	processor := NewEnumASTProcessor()
	result, _, err := processor.Process([]byte(source))
	if err != nil {
		t.Fatalf("Process() failed: %v", err)
	}

	output := string(result)

	// Verify string literal is preserved
	if !strings.Contains(output, `const description = "enum Status { Pending, Active }"`) {
		t.Error("String literal with 'enum' keyword was incorrectly modified")
	}
	if !strings.Contains(output, `println("enum in string")`) {
		t.Error("String literal with 'enum' keyword was incorrectly modified")
	}

	// Verify only RealEnum was processed
	if !strings.Contains(output, "type RealEnumTag uint8") {
		t.Error("Missing RealEnumTag type")
	}
	if !strings.Contains(output, "RealEnumTagVariant1") {
		t.Error("Missing RealEnumTagVariant1 constant")
	}

	// Verify no "Status" enum was generated from string
	if strings.Contains(output, "type StatusTag uint8") {
		t.Error("Incorrectly generated StatusTag from string literal")
	}

	// Verify generated code compiles
	fset := token.NewFileSet()
	_, parseErr := parser.ParseFile(fset, "", result, parser.AllErrors)
	if parseErr != nil {
		t.Errorf("Generated code does not compile: %v\nGenerated code:\n%s", parseErr, output)
	}
}

// TestEnumASTProcessor_TupleVariantWithGenerics tests tuple variants with generic types
func TestEnumASTProcessor_TupleVariantWithGenerics(t *testing.T) {
	source := `package main

enum Container {
	Empty,
	Single(Option[string]),
	Pair(Result[int, error], Option[string]),
	Triple(int, Map[string, List[int]], error),
}
`

	processor := NewEnumASTProcessor()
	result, _, err := processor.Process([]byte(source))
	if err != nil {
		t.Fatalf("Process() failed: %v", err)
	}

	output := string(result)

	// Verify tuple variant fields are generated
	if !strings.Contains(output, "single *Option[string]") {
		t.Error("Missing single field for Single variant")
	}
	if !strings.Contains(output, "pair *Result[int, error]") {
		t.Error("Missing pair field for first tuple element")
	}
	if !strings.Contains(output, "pair1 *Option[string]") {
		t.Error("Missing pair1 field for second tuple element")
	}
	if !strings.Contains(output, "triple *int") {
		t.Error("Missing triple field")
	}
	if !strings.Contains(output, "triple1 *Map[string, List[int]]") {
		t.Error("Missing triple1 field with nested generics")
	}
	if !strings.Contains(output, "triple2 *error") {
		t.Error("Missing triple2 field")
	}

	// Verify constructors with proper parameter types
	if !strings.Contains(output, "func ContainerSingle(arg0 Option[string]) Container") {
		t.Error("Missing ContainerSingle constructor with generic parameter")
	}
	if !strings.Contains(output, "func ContainerPair(arg0 Result[int, error], arg1 Option[string]) Container") {
		t.Error("Missing ContainerPair constructor with generic parameters")
	}
	if !strings.Contains(output, "func ContainerTriple(arg0 int, arg1 Map[string, List[int]], arg2 error) Container") {
		t.Error("Missing ContainerTriple constructor with mixed parameters")
	}
}

// TestEnumASTProcessor_FunctionTypeFields tests function type in struct variants
func TestEnumASTProcessor_FunctionTypeFields(t *testing.T) {
	source := `package main

enum Handler {
	Simple { handler: func(int) error },
	Complex { handler: func(a, b int) (string, error) },
	Variadic { handler: func(args ...string) error },
}
`

	processor := NewEnumASTProcessor()
	result, _, err := processor.Process([]byte(source))
	if err != nil {
		t.Fatalf("Process() failed: %v", err)
	}

	output := string(result)

	// Verify function type fields are preserved
	if !strings.Contains(output, "simple_handler *func(int) error") {
		t.Error("Missing simple_handler field with function type")
	}
	if !strings.Contains(output, "complex_handler *func(a, b int) (string, error)") {
		t.Error("Missing complex_handler field with complex function type")
	}
	if !strings.Contains(output, "variadic_handler *func(args ...string) error") {
		t.Error("Missing variadic_handler field with variadic function type")
	}

	// Verify constructors
	if !strings.Contains(output, "func HandlerSimple(handler func(int) error) Handler") {
		t.Error("Missing HandlerSimple constructor")
	}
	if !strings.Contains(output, "func HandlerComplex(handler func(a, b int) (string, error)) Handler") {
		t.Error("Missing HandlerComplex constructor")
	}

	// Verify generated code compiles
	fset := token.NewFileSet()
	_, parseErr := parser.ParseFile(fset, "", result, parser.AllErrors)
	if parseErr != nil {
		t.Errorf("Generated code does not compile: %v\nGenerated code:\n%s", parseErr, output)
	}
}

// TestEnumASTProcessor_ChannelTypes tests channel type handling
func TestEnumASTProcessor_ChannelTypes(t *testing.T) {
	source := `package main

enum Channel {
	BiDirectional { ch: chan int },
	SendOnly { ch: chan<- string },
	ReceiveOnly { ch: <-chan error },
}
`

	processor := NewEnumASTProcessor()
	result, _, err := processor.Process([]byte(source))
	if err != nil {
		t.Fatalf("Process() failed: %v", err)
	}

	output := string(result)

	// Verify channel type fields
	if !strings.Contains(output, "bidirectional_ch *chan int") {
		t.Error("Missing bidirectional_ch field")
	}
	if !strings.Contains(output, "sendonly_ch *chan<- string") {
		t.Error("Missing sendonly_ch field with send-only channel")
	}
	if !strings.Contains(output, "receiveonly_ch *<-chan error") {
		t.Error("Missing receiveonly_ch field with receive-only channel")
	}

	// Verify constructors
	if !strings.Contains(output, "func ChannelBiDirectional(ch chan int) Channel") {
		t.Error("Missing ChannelBiDirectional constructor")
	}
	if !strings.Contains(output, "func ChannelSendOnly(ch chan<- string) Channel") {
		t.Error("Missing ChannelSendOnly constructor")
	}
	if !strings.Contains(output, "func ChannelReceiveOnly(ch <-chan error) Channel") {
		t.Error("Missing ChannelReceiveOnly constructor")
	}

	// Verify generated code compiles
	fset := token.NewFileSet()
	_, parseErr := parser.ParseFile(fset, "", result, parser.AllErrors)
	if parseErr != nil {
		t.Errorf("Generated code does not compile: %v\nGenerated code:\n%s", parseErr, output)
	}
}
