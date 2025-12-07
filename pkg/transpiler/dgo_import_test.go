package transpiler

import (
	"go/parser"
	"go/token"
	"testing"
)

func TestDetectDgoTypes_ResultWithTwoParams(t *testing.T) {
	src := `package main

func foo() Result[User, error] {
	return nil
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if !DetectDgoTypes(file) {
		t.Error("Expected DetectDgoTypes to return true for Result[T, E]")
	}
}

func TestDetectDgoTypes_OptionWithOneParam(t *testing.T) {
	src := `package main

func foo() Option[User] {
	return nil
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if !DetectDgoTypes(file) {
		t.Error("Expected DetectDgoTypes to return true for Option[T]")
	}
}

func TestDetectDgoTypes_NoTypes_ReturnsFalse(t *testing.T) {
	src := `package main

func foo() (User, error) {
	return nil, nil
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if DetectDgoTypes(file) {
		t.Error("Expected DetectDgoTypes to return false when no Result/Option types present")
	}
}

func TestDetectDgoTypes_ResultInVariableDeclaration(t *testing.T) {
	src := `package main

var result Result[int, error]
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if !DetectDgoTypes(file) {
		t.Error("Expected DetectDgoTypes to return true for Result in variable declaration")
	}
}

func TestInjectDgoImport_AddsImport(t *testing.T) {
	src := `package main

func foo() Result[User, error] {
	return nil
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	// Before injection, should have no imports
	if len(file.Imports) != 0 {
		t.Fatalf("Expected no imports before injection, got %d", len(file.Imports))
	}

	// Inject import
	injected := InjectDgoImport(fset, file)
	if !injected {
		t.Error("Expected InjectDgoImport to return true when import was added")
	}

	// After injection, should have 1 import
	if len(file.Imports) != 1 {
		t.Fatalf("Expected 1 import after injection, got %d", len(file.Imports))
	}

	// Verify import path
	if file.Imports[0].Path.Value != `"`+DgoImportPath+`"` {
		t.Errorf("Expected import path %q, got %q", DgoImportPath, file.Imports[0].Path.Value)
	}
}

func TestInjectDgoImport_AlreadyImported_NoChange(t *testing.T) {
	src := `package main

import "github.com/MadAppGang/dingo/pkg/dgo"

func foo() Result[User, error] {
	return nil
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	// Should have 1 import before
	if len(file.Imports) != 1 {
		t.Fatalf("Expected 1 import before injection, got %d", len(file.Imports))
	}

	// Try to inject
	injected := InjectDgoImport(fset, file)
	if injected {
		t.Error("Expected InjectDgoImport to return false when import already exists")
	}

	// Should still have 1 import after
	if len(file.Imports) != 1 {
		t.Fatalf("Expected 1 import after injection attempt, got %d", len(file.Imports))
	}
}

func TestInjectDgoImport_NoTypes_NoImport(t *testing.T) {
	src := `package main

func foo() (User, error) {
	return nil, nil
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	// Try to inject
	injected := InjectDgoImport(fset, file)
	if injected {
		t.Error("Expected InjectDgoImport to return false when no Result/Option types present")
	}

	// Should have no imports
	if len(file.Imports) != 0 {
		t.Fatalf("Expected 0 imports, got %d", len(file.Imports))
	}
}

func TestDetectDgoTypes_OptionInStructField(t *testing.T) {
	src := `package main

type User struct {
	Email Option[string]
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if !DetectDgoTypes(file) {
		t.Error("Expected DetectDgoTypes to return true for Option in struct field")
	}
}

func TestDetectDgoTypes_ResultInMapType(t *testing.T) {
	src := `package main

var cache map[int]Result[User, error]
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if !DetectDgoTypes(file) {
		t.Error("Expected DetectDgoTypes to return true for Result in map value type")
	}
}
