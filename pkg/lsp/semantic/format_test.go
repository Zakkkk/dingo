package semantic

import (
	"go/types"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
)

func TestFormatHover_NilEntity(t *testing.T) {
	result := FormatHover(nil, nil)
	if result != nil {
		t.Errorf("FormatHover(nil) should return nil, got %v", result)
	}
}

func TestFormatHover_SimpleVariable(t *testing.T) {
	pkg := types.NewPackage("main", "main")
	intType := types.Typ[types.Int]
	varObj := types.NewVar(0, pkg, "x", intType)

	entity := &SemanticEntity{
		Line:   1,
		Col:    5,
		EndCol: 6,
		Kind:   KindIdent,
		Object: varObj,
		Type:   intType,
	}

	hover := FormatHover(entity, pkg)
	if hover == nil {
		t.Fatal("FormatHover returned nil")
	}

	content := hover.Contents.Value
	if !strings.Contains(content, "var x int") {
		t.Errorf("Expected 'var x int', got: %s", content)
	}
	if !strings.Contains(content, "```go") {
		t.Errorf("Expected code fence, got: %s", content)
	}
}

func TestFormatHover_ErrorPropagationVariable(t *testing.T) {
	pkg := types.NewPackage("main", "main")

	// Create User type
	userType := types.NewNamed(
		types.NewTypeName(0, pkg, "User", nil),
		types.NewStruct(nil, nil),
		nil,
	)

	// Create Result[User, error] type (named type)
	resultTypeName := types.NewTypeName(0, pkg, "Result", nil)
	resultType := types.NewNamed(resultTypeName, types.NewStruct(nil, nil), nil)

	varObj := types.NewVar(0, pkg, "user", userType)

	entity := &SemanticEntity{
		Line:   1,
		Col:    5,
		EndCol: 9,
		Kind:   KindIdent,
		Object: varObj,
		Type:   userType,
		Context: &DingoContext{
			Kind:          ContextErrorProp,
			OriginalType:  resultType,
			UnwrappedType: userType,
		},
	}

	hover := FormatHover(entity, pkg)
	if hover == nil {
		t.Fatal("FormatHover returned nil")
	}

	content := hover.Contents.Value

	// Check for unwrapped type
	if !strings.Contains(content, "var user User") {
		t.Errorf("Expected 'var user User', got: %s", content)
	}

	// Check for origin information
	if !strings.Contains(content, "from") {
		t.Errorf("Expected 'from' in content, got: %s", content)
	}
	if !strings.Contains(content, "Result") {
		t.Errorf("Expected 'Result' type reference, got: %s", content)
	}

	// Check for explanation
	if !strings.Contains(content, "Error propagation") {
		t.Errorf("Expected error propagation explanation, got: %s", content)
	}
}

func TestFormatHover_ErrorPropOperator(t *testing.T) {
	pkg := types.NewPackage("main", "main")

	userType := types.NewNamed(
		types.NewTypeName(0, pkg, "User", nil),
		types.NewStruct(nil, nil),
		nil,
	)

	resultTypeName := types.NewTypeName(0, pkg, "Result", nil)
	resultType := types.NewNamed(resultTypeName, types.NewStruct(nil, nil), nil)

	entity := &SemanticEntity{
		Line:   1,
		Col:    20,
		EndCol: 21,
		Kind:   KindOperator,
		Context: &DingoContext{
			Kind:          ContextErrorProp,
			OriginalType:  resultType,
			UnwrappedType: userType,
		},
	}

	hover := FormatHover(entity, pkg)
	if hover == nil {
		t.Fatal("FormatHover returned nil")
	}

	content := hover.Contents.Value

	// Check for operator indicator (with markdown formatting)
	if !strings.Contains(content, "`?` error propagation") {
		t.Errorf("Expected '`?` error propagation', got: %s", content)
	}

	// Check for unwrapping information
	if !strings.Contains(content, "Unwraps") {
		t.Errorf("Expected 'Unwraps', got: %s", content)
	}
	if !strings.Contains(content, "User") {
		t.Errorf("Expected 'User' type, got: %s", content)
	}

	// Check for behavior description
	if !strings.Contains(content, "Returns early") {
		t.Errorf("Expected 'Returns early' description, got: %s", content)
	}
}

func TestFormatHover_NullCoalesceOperator(t *testing.T) {
	pkg := types.NewPackage("main", "main")
	intType := types.Typ[types.Int]

	entity := &SemanticEntity{
		Line:   1,
		Col:    10,
		EndCol: 12,
		Kind:   KindOperator,
		Context: &DingoContext{
			Kind:          ContextNullCoal,
			UnwrappedType: intType,
		},
	}

	hover := FormatHover(entity, pkg)
	if hover == nil {
		t.Fatal("FormatHover returned nil")
	}

	content := hover.Contents.Value

	if !strings.Contains(content, "`??` null coalescing") {
		t.Errorf("Expected '`??` null coalescing', got: %s", content)
	}
	if !strings.Contains(content, "Type:") {
		t.Errorf("Expected 'Type:', got: %s", content)
	}
	if !strings.Contains(content, "int") {
		t.Errorf("Expected 'int' type, got: %s", content)
	}
	if !strings.Contains(content, "non-nil") {
		t.Errorf("Expected 'non-nil' description, got: %s", content)
	}
}

func TestFormatHover_SafeNavOperator(t *testing.T) {
	pkg := types.NewPackage("main", "main")
	stringType := types.Typ[types.String]
	ptrType := types.NewPointer(stringType)

	entity := &SemanticEntity{
		Line:   1,
		Col:    5,
		EndCol: 7,
		Kind:   KindOperator,
		Context: &DingoContext{
			Kind:          ContextSafeNav,
			UnwrappedType: ptrType,
		},
	}

	hover := FormatHover(entity, pkg)
	if hover == nil {
		t.Fatal("FormatHover returned nil")
	}

	content := hover.Contents.Value

	if !strings.Contains(content, "`?.` safe navigation") {
		t.Errorf("Expected '`?.` safe navigation', got: %s", content)
	}
	if !strings.Contains(content, "Type:") {
		t.Errorf("Expected 'Type:', got: %s", content)
	}
	if !strings.Contains(content, "string") {
		t.Errorf("Expected 'string' type, got: %s", content)
	}
	if !strings.Contains(content, "`nil` if receiver is `nil`") {
		t.Errorf("Expected '`nil` if receiver is `nil`' description, got: %s", content)
	}
}

func TestFormatHover_Function(t *testing.T) {
	pkg := types.NewPackage("main", "main")

	// Create function: func add(a int, b int) int
	params := types.NewTuple(
		types.NewVar(0, pkg, "a", types.Typ[types.Int]),
		types.NewVar(0, pkg, "b", types.Typ[types.Int]),
	)
	results := types.NewTuple(
		types.NewVar(0, pkg, "", types.Typ[types.Int]),
	)
	sig := types.NewSignature(nil, params, results, false)
	funcObj := types.NewFunc(0, pkg, "add", sig)

	entity := &SemanticEntity{
		Line:   1,
		Col:    5,
		EndCol: 8,
		Kind:   KindIdent,
		Object: funcObj,
		Type:   sig,
	}

	hover := FormatHover(entity, pkg)
	if hover == nil {
		t.Fatal("FormatHover returned nil")
	}

	content := hover.Contents.Value

	if !strings.Contains(content, "func add") {
		t.Errorf("Expected 'func add', got: %s", content)
	}
	if !strings.Contains(content, "int") {
		t.Errorf("Expected 'int' parameter/return type, got: %s", content)
	}
}

func TestFormatHover_Const(t *testing.T) {
	pkg := types.NewPackage("main", "main")
	constObj := types.NewConst(0, pkg, "MaxSize", types.Typ[types.Int], nil)

	entity := &SemanticEntity{
		Line:   1,
		Col:    7,
		EndCol: 14,
		Kind:   KindIdent,
		Object: constObj,
		Type:   types.Typ[types.Int],
	}

	hover := FormatHover(entity, pkg)
	if hover == nil {
		t.Fatal("FormatHover returned nil")
	}

	content := hover.Contents.Value

	if !strings.Contains(content, "const MaxSize int") {
		t.Errorf("Expected 'const MaxSize int', got: %s", content)
	}
}

func TestFormatHover_Field(t *testing.T) {
	pkg := types.NewPackage("main", "main")

	// Create field using NewField (IsField is determined by construction method)
	field := types.NewField(0, pkg, "Name", types.Typ[types.String], false)

	entity := &SemanticEntity{
		Line:   1,
		Col:    5,
		EndCol: 9,
		Kind:   KindField,
		Object: field,
		Type:   types.Typ[types.String],
	}

	hover := FormatHover(entity, pkg)
	if hover == nil {
		t.Fatal("FormatHover returned nil")
	}

	content := hover.Contents.Value

	if !strings.Contains(content, "field Name string") {
		t.Errorf("Expected 'field Name string', got: %s", content)
	}
}

func TestFormatHover_TypeName(t *testing.T) {
	pkg := types.NewPackage("main", "main")

	// Create type User struct { Name string }
	fields := []*types.Var{
		types.NewField(0, pkg, "Name", types.Typ[types.String], false),
	}
	structType := types.NewStruct(fields, nil)
	typeName := types.NewTypeName(0, pkg, "User", structType)

	entity := &SemanticEntity{
		Line:   1,
		Col:    6,
		EndCol: 10,
		Kind:   KindType,
		Object: typeName,
		Type:   types.NewNamed(typeName, structType, nil),
	}

	hover := FormatHover(entity, pkg)
	if hover == nil {
		t.Fatal("FormatHover returned nil")
	}

	content := hover.Contents.Value

	if !strings.Contains(content, "type User") {
		t.Errorf("Expected 'type User', got: %s", content)
	}
	if !strings.Contains(content, "struct") {
		t.Errorf("Expected 'struct', got: %s", content)
	}
}

func TestFormatType_PackageQualification(t *testing.T) {
	pkg1 := types.NewPackage("main", "main")
	pkg2 := types.NewPackage("github.com/user/other", "other")

	// Type from different package
	typeName := types.NewTypeName(0, pkg2, "User", nil)
	namedType := types.NewNamed(typeName, types.NewStruct(nil, nil), nil)

	// Format from pkg1's perspective
	formatted := formatType(namedType, pkg1)

	// Should include package qualifier
	if !strings.Contains(formatted, "other.User") {
		t.Errorf("Expected 'other.User' with package qualifier, got: %s", formatted)
	}

	// Format from pkg2's perspective (same package)
	formatted = formatType(namedType, pkg2)

	// Should NOT include package qualifier
	if strings.Contains(formatted, "other.") {
		t.Errorf("Expected 'User' without qualifier, got: %s", formatted)
	}
}

func TestFormatSignatureType_Variadic(t *testing.T) {
	pkg := types.NewPackage("main", "main")

	// func printf(format string, args ...interface{})
	params := types.NewTuple(
		types.NewVar(0, pkg, "format", types.Typ[types.String]),
		types.NewVar(0, pkg, "args", types.NewSlice(types.NewInterfaceType(nil, nil))),
	)
	sig := types.NewSignature(nil, params, nil, true)

	formatted := formatSignatureType(sig, pkg)

	if !strings.Contains(formatted, "...") {
		t.Errorf("Expected variadic '...' syntax, got: %s", formatted)
	}
}

func TestFormatSignatureType_MultipleResults(t *testing.T) {
	pkg := types.NewPackage("main", "main")

	// func divide(a, b int) (int, error)
	params := types.NewTuple(
		types.NewVar(0, pkg, "a", types.Typ[types.Int]),
		types.NewVar(0, pkg, "b", types.Typ[types.Int]),
	)
	results := types.NewTuple(
		types.NewVar(0, pkg, "", types.Typ[types.Int]),
		types.NewVar(0, pkg, "", types.Universe.Lookup("error").Type()),
	)
	sig := types.NewSignature(nil, params, results, false)

	formatted := formatSignatureType(sig, pkg)

	// Should have parentheses around multiple results
	if !strings.Contains(formatted, "(int, error)") {
		t.Errorf("Expected '(int, error)' for multiple results, got: %s", formatted)
	}
}

func TestFormatHover_MarkdownFormat(t *testing.T) {
	pkg := types.NewPackage("main", "main")
	varObj := types.NewVar(0, pkg, "x", types.Typ[types.Int])

	entity := &SemanticEntity{
		Line:   1,
		Col:    5,
		EndCol: 6,
		Kind:   KindIdent,
		Object: varObj,
		Type:   types.Typ[types.Int],
	}

	hover := FormatHover(entity, pkg)
	if hover == nil {
		t.Fatal("FormatHover returned nil")
	}

	// Verify protocol.MarkupContent structure
	if hover.Contents.Kind != protocol.Markdown {
		t.Errorf("Expected Markdown kind, got: %v", hover.Contents.Kind)
	}

	if hover.Contents.Value == "" {
		t.Error("Expected non-empty Value")
	}
}

func TestFormatHover_ExpressionWithoutObject(t *testing.T) {
	pkg := types.NewPackage("main", "main")
	intType := types.Typ[types.Int]

	entity := &SemanticEntity{
		Line:   1,
		Col:    10,
		EndCol: 15,
		Kind:   KindCall,
		Object: nil, // Expression, no object
		Type:   intType,
	}

	hover := FormatHover(entity, pkg)
	if hover == nil {
		t.Fatal("FormatHover returned nil")
	}

	content := hover.Contents.Value

	if !strings.Contains(content, "int") {
		t.Errorf("Expected 'int' type, got: %s", content)
	}
}

func TestFormatHover_WithContextDescription(t *testing.T) {
	pkg := types.NewPackage("main", "main")
	varObj := types.NewVar(0, pkg, "x", types.Typ[types.Int])

	entity := &SemanticEntity{
		Line:   1,
		Col:    5,
		EndCol: 6,
		Kind:   KindIdent,
		Object: varObj,
		Type:   types.Typ[types.Int],
		Context: &DingoContext{
			Kind:        ContextNone,
			Description: "This is a test variable",
		},
	}

	hover := FormatHover(entity, pkg)
	if hover == nil {
		t.Fatal("FormatHover returned nil")
	}

	content := hover.Contents.Value

	if !strings.Contains(content, "This is a test variable") {
		t.Errorf("Expected context description, got: %s", content)
	}
}
