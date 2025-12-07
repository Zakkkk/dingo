package transpiler

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"strings"
	"testing"
)

// astToSource converts an AST back to source code for verification
func astToSource(t *testing.T, fset *token.FileSet, file *ast.File) string {
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fset, file); err != nil {
		t.Fatalf("failed to print AST: %v", err)
	}
	return buf.String()
}

// TestNoneInStructField tests Config{Language: None} infers to None[string]()
func TestNoneInStructField(t *testing.T) {
	src := `package main

type Config struct {
	Language Option[string]
}

func main() {
	cfg := Config{Language: None}
}`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	transformer := NewNoneInferenceTransformer(fset, file)
	if err := transformer.Transform(); err != nil {
		t.Fatalf("transform error: %v", err)
	}

	output := astToSource(t, fset, file)
	if !strings.Contains(output, "None[string]()") {
		t.Errorf("expected None[string](), got:\n%s", output)
	}
	if strings.Contains(output, "Language: None\n") || strings.Contains(output, "Language: None}") {
		t.Errorf("bare None should not remain, got:\n%s", output)
	}
}

// TestNoneCallInStructField tests Config{Language: None()} also works
func TestNoneCallInStructField(t *testing.T) {
	src := `package main

type Config struct {
	Language Option[string]
}

func main() {
	cfg := Config{Language: None()}
}`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	transformer := NewNoneInferenceTransformer(fset, file)
	if err := transformer.Transform(); err != nil {
		t.Fatalf("transform error: %v", err)
	}

	output := astToSource(t, fset, file)
	if !strings.Contains(output, "None[string]()") {
		t.Errorf("expected None[string](), got:\n%s", output)
	}
	// Ensure we don't have None() without type param
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Language:") && strings.Contains(line, "None()") && !strings.Contains(line, "None[") {
			t.Errorf("None() should be transformed to None[string](), got line: %s", line)
		}
	}
}

// TestNoneInReturnStatement tests func f() Option[int] { return None }
func TestNoneInReturnStatement(t *testing.T) {
	src := `package main

func getOptional() Option[int] {
	return None
}`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	transformer := NewNoneInferenceTransformer(fset, file)
	if err := transformer.Transform(); err != nil {
		t.Fatalf("transform error: %v", err)
	}

	output := astToSource(t, fset, file)
	if !strings.Contains(output, "None[int]()") {
		t.Errorf("expected None[int](), got:\n%s", output)
	}
	if strings.Contains(output, "return None\n") || strings.Contains(output, "return None}") {
		t.Errorf("bare None should not remain, got:\n%s", output)
	}
}

// TestNoneInGenericReturn tests func GetValue[T any]() Option[T] { return None }
func TestNoneInGenericReturn(t *testing.T) {
	src := `package main

func GetValue[T any]() Option[T] {
	return None
}`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	transformer := NewNoneInferenceTransformer(fset, file)
	if err := transformer.Transform(); err != nil {
		t.Fatalf("transform error: %v", err)
	}

	output := astToSource(t, fset, file)
	if !strings.Contains(output, "None[T]()") {
		t.Errorf("expected None[T](), got:\n%s", output)
	}
	if strings.Contains(output, "return None\n") || strings.Contains(output, "return None}") {
		t.Errorf("bare None should not remain, got:\n%s", output)
	}
}

// TestNoneInVariableDecl tests var x Option[string] = None
func TestNoneInVariableDecl(t *testing.T) {
	src := `package main

var x Option[string] = None`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	transformer := NewNoneInferenceTransformer(fset, file)
	if err := transformer.Transform(); err != nil {
		t.Fatalf("transform error: %v", err)
	}

	output := astToSource(t, fset, file)
	if !strings.Contains(output, "None[string]()") {
		t.Errorf("expected None[string](), got:\n%s", output)
	}
	// Check bare None is gone
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "var x") && strings.Contains(line, "= None") && !strings.Contains(line, "None[") {
			t.Errorf("bare None should be transformed, got line: %s", line)
		}
	}
}

// TestNoneInFunctionArg tests process(None) where param is Option[string]
func TestNoneInFunctionArg(t *testing.T) {
	src := `package main

func process(val Option[string]) {
}

func main() {
	process(None)
}`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	transformer := NewNoneInferenceTransformer(fset, file)
	if err := transformer.Transform(); err != nil {
		t.Fatalf("transform error: %v", err)
	}

	output := astToSource(t, fset, file)
	// Check for None[string]() - ignore whitespace/formatting differences
	if !strings.Contains(output, "None[string]()") {
		t.Errorf("expected None[string]() in function call, got:\n%s", output)
	}
	// Verify process function is called
	if !strings.Contains(output, "process(") {
		t.Errorf("expected process() call, got:\n%s", output)
	}
}

// TestNoneInAssignment tests x = None where x was declared as Option[string]
func TestNoneInAssignment(t *testing.T) {
	src := `package main

var x Option[string]

func main() {
	x = None
}`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	transformer := NewNoneInferenceTransformer(fset, file)
	if err := transformer.Transform(); err != nil {
		t.Fatalf("transform error: %v", err)
	}

	output := astToSource(t, fset, file)
	if !strings.Contains(output, "x = None[string]()") {
		t.Errorf("expected x = None[string](), got:\n%s", output)
	}
}

// TestNoneInShortDeclError tests x := None produces error
func TestNoneInShortDeclError(t *testing.T) {
	src := `package main

func main() {
	x := None
}`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	transformer := NewNoneInferenceTransformer(fset, file)
	err = transformer.Transform()
	if err == nil {
		t.Fatalf("expected error for x := None, got nil")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "cannot infer Option type") {
		t.Errorf("expected error about type inference, got: %v", err)
	}
	if !strings.Contains(errMsg, "short declaration") {
		t.Errorf("expected error to mention 'short declaration', got: %v", err)
	}
}

// TestNoneCallFormError tests x := None() also produces error
func TestNoneCallFormError(t *testing.T) {
	src := `package main

func main() {
	x := None()
}`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	transformer := NewNoneInferenceTransformer(fset, file)
	err = transformer.Transform()
	if err == nil {
		t.Fatalf("expected error for x := None(), got nil")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "cannot infer Option type") {
		t.Errorf("expected error about type inference, got: %v", err)
	}
}

// TestNestedOptionType tests Option[Option[string]] works correctly
func TestNestedOptionType(t *testing.T) {
	src := `package main

type Config struct {
	Nested Option[Option[string]]
}

func main() {
	cfg := Config{Nested: None}
}`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	transformer := NewNoneInferenceTransformer(fset, file)
	if err := transformer.Transform(); err != nil {
		t.Fatalf("transform error: %v", err)
	}

	output := astToSource(t, fset, file)
	// Should infer to outer Option type parameter
	if !strings.Contains(output, "None[Option[string]]()") {
		t.Errorf("expected None[Option[string]](), got:\n%s", output)
	}
}

// TestMultipleNoneFields tests multiple None fields in same struct
func TestMultipleNoneFields(t *testing.T) {
	src := `package main

type Config struct {
	Name     Option[string]
	Age      Option[int]
	Active   Option[bool]
}

func main() {
	cfg := Config{
		Name:   None,
		Age:    None,
		Active: None,
	}
}`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	transformer := NewNoneInferenceTransformer(fset, file)
	if err := transformer.Transform(); err != nil {
		t.Fatalf("transform error: %v", err)
	}

	output := astToSource(t, fset, file)
	if !strings.Contains(output, "None[string]()") {
		t.Errorf("expected None[string]() for Name field, got:\n%s", output)
	}
	if !strings.Contains(output, "None[int]()") {
		t.Errorf("expected None[int]() for Age field, got:\n%s", output)
	}
	if !strings.Contains(output, "None[bool]()") {
		t.Errorf("expected None[bool]() for Active field, got:\n%s", output)
	}
	// Ensure no bare None remains
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if (strings.Contains(line, "Name:") || strings.Contains(line, "Age:") || strings.Contains(line, "Active:")) &&
			strings.Contains(line, "None") && !strings.Contains(line, "None[") {
			t.Errorf("bare None should not remain in struct literal, got line: %s", line)
		}
	}
}

// TestMultipleReturns tests function with multiple return values
func TestMultipleReturns(t *testing.T) {
	src := `package main

func getTwo() (Option[string], Option[int]) {
	return None, None
}`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	transformer := NewNoneInferenceTransformer(fset, file)
	if err := transformer.Transform(); err != nil {
		t.Fatalf("transform error: %v", err)
	}

	output := astToSource(t, fset, file)
	if !strings.Contains(output, "None[string]()") {
		t.Errorf("expected None[string]() for first return, got:\n%s", output)
	}
	if !strings.Contains(output, "None[int]()") {
		t.Errorf("expected None[int]() for second return, got:\n%s", output)
	}
}

// TestDgoPackagePrefix tests dgo.Option[T] works
func TestDgoPackagePrefix(t *testing.T) {
	src := `package main

import "github.com/dingolang/dgo"

type Config struct {
	Language dgo.Option[string]
}

func main() {
	cfg := Config{Language: None}
}`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	transformer := NewNoneInferenceTransformer(fset, file)
	if err := transformer.Transform(); err != nil {
		t.Fatalf("transform error: %v", err)
	}

	output := astToSource(t, fset, file)
	if !strings.Contains(output, "None[string]()") {
		t.Errorf("expected None[string]() for dgo.Option, got:\n%s", output)
	}
}

// TestFunctionLiteralReturn tests None in function literal
func TestFunctionLiteralReturn(t *testing.T) {
	src := `package main

func main() {
	f := func() Option[string] {
		return None
	}
	_ = f
}`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	transformer := NewNoneInferenceTransformer(fset, file)
	if err := transformer.Transform(); err != nil {
		t.Fatalf("transform error: %v", err)
	}

	output := astToSource(t, fset, file)
	if !strings.Contains(output, "None[string]()") {
		t.Errorf("expected None[string]() in function literal, got:\n%s", output)
	}
}

// TestNoTransformNeeded tests that None[T]() is left unchanged
func TestNoTransformNeeded(t *testing.T) {
	src := `package main

func getOptional() Option[int] {
	return None[int]()
}`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	transformer := NewNoneInferenceTransformer(fset, file)
	if err := transformer.Transform(); err != nil {
		t.Fatalf("transform error: %v", err)
	}

	output := astToSource(t, fset, file)
	// Should remain None[int]() (count occurrences to ensure no duplication)
	count := strings.Count(output, "None[int]()")
	if count != 1 {
		t.Errorf("expected exactly 1 occurrence of None[int](), got %d in:\n%s", count, output)
	}
}

// TestComplexNestedStruct tests nested struct literals
func TestComplexNestedStruct(t *testing.T) {
	src := `package main

type Inner struct {
	Value Option[int]
}

type Outer struct {
	Data Option[Inner]
}

func main() {
	obj := Outer{
		Data: None,
	}
}`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	transformer := NewNoneInferenceTransformer(fset, file)
	if err := transformer.Transform(); err != nil {
		t.Fatalf("transform error: %v", err)
	}

	output := astToSource(t, fset, file)
	if !strings.Contains(output, "None[Inner]()") {
		t.Errorf("expected None[Inner](), got:\n%s", output)
	}
}

// TestNamedReturnValues tests functions with named return values
func TestNamedReturnValues(t *testing.T) {
	src := `package main

func getOptional() (result Option[string]) {
	return None
}`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	transformer := NewNoneInferenceTransformer(fset, file)
	if err := transformer.Transform(); err != nil {
		t.Fatalf("transform error: %v", err)
	}

	output := astToSource(t, fset, file)
	if !strings.Contains(output, "None[string]()") {
		t.Errorf("expected None[string]() for named return, got:\n%s", output)
	}
}

// TestMixedNoneAndSome tests struct with both None and Some values
func TestMixedNoneAndSome(t *testing.T) {
	src := `package main

type Config struct {
	Name Option[string]
	Age  Option[int]
}

func main() {
	cfg := Config{
		Name: Some("Alice"),
		Age:  None,
	}
}`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	transformer := NewNoneInferenceTransformer(fset, file)
	if err := transformer.Transform(); err != nil {
		t.Fatalf("transform error: %v", err)
	}

	output := astToSource(t, fset, file)
	if !strings.Contains(output, "None[int]()") {
		t.Errorf("expected None[int]() for Age field, got:\n%s", output)
	}
	// Some should remain unchanged
	if !strings.Contains(output, `Some("Alice")`) {
		t.Errorf("expected Some(\"Alice\") to remain unchanged, got:\n%s", output)
	}
}

// TestErrorMessages tests that error messages include file positions
func TestErrorMessages(t *testing.T) {
	src := `package main

func main() {
	x := None
	y := None()
}`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	transformer := NewNoneInferenceTransformer(fset, file)
	err = transformer.Transform()
	if err == nil {
		t.Fatalf("expected error for short declarations, got nil")
	}

	// Should have 2 errors
	if len(transformer.Errors()) != 2 {
		t.Errorf("expected 2 errors, got %d", len(transformer.Errors()))
	}

	// Check error messages include positions
	formattedErrors := transformer.FormatErrors(fset)
	if !strings.Contains(formattedErrors, "test.go:") {
		t.Errorf("expected error messages to include file name, got:\n%s", formattedErrors)
	}
	if !strings.Contains(formattedErrors, "cannot infer Option type") {
		t.Errorf("expected error messages to mention 'cannot infer Option type', got:\n%s", formattedErrors)
	}
}

// TestVariadicFunction tests None in variadic function arguments
// KNOWN LIMITATION: Current implementation doesn't handle variadic params (param.Type is *ast.Ellipsis)
// This test documents the expected behavior when variadic support is added
func TestVariadicFunction(t *testing.T) {
	t.Skip("Variadic parameter inference not yet implemented - param.Type needs Ellipsis unwrapping")

	src := `package main

func process(values ...Option[string]) {
}

func main() {
	process(None, None)
}`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	transformer := NewNoneInferenceTransformer(fset, file)
	if err := transformer.Transform(); err != nil {
		t.Fatalf("transform error: %v", err)
	}

	output := astToSource(t, fset, file)
	// Both None should be transformed (when variadic support is added)
	count := strings.Count(output, "None[string]()")
	if count < 2 {
		t.Errorf("expected at least 2 occurrences of None[string]() for variadic args, got %d in:\n%s", count, output)
	}
}
