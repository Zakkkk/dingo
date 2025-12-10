package parser

import (
	"go/ast"
	"go/token"
	"testing"

	"github.com/MadAppGang/dingo/pkg/tokenizer"
)

func TestParseType_CompositeTypes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantType string // Expected type structure
	}{
		{
			name:     "empty interface",
			input:    "package main\nvar x: interface{}",
			wantType: "*ast.InterfaceType",
		},
		{
			name:     "map with interface value",
			input:    "package main\nvar x: map[string]interface{}",
			wantType: "*ast.MapType",
		},
		{
			name:     "slice type",
			input:    "package main\nvar x: []string",
			wantType: "*ast.ArrayType",
		},
		{
			name:     "map with slice value",
			input:    "package main\nvar x: map[string][]int",
			wantType: "*ast.MapType",
		},
		{
			name:     "pointer type",
			input:    "package main\nvar x: *string",
			wantType: "*ast.StarExpr",
		},
		{
			name:     "chan type",
			input:    "package main\nvar x: chan int",
			wantType: "*ast.ChanType",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Tokenize
			tok := tokenizer.New([]byte(tt.input))
			fset := token.NewFileSet()

			// Parse
			parser := NewStmtParser(tok, fset)
			_, err := parser.ParseFile()
			if err != nil {
				t.Fatalf("parsing failed: %v", err)
			}

			// Success if no errors (we don't validate the exact AST structure)
		})
	}
}

func TestParseMapType(t *testing.T) {
	input := "package main\nvar config: map[string]interface{}"

	tok := tokenizer.New([]byte(input))
	fset := token.NewFileSet()

	parser := NewStmtParser(tok, fset)
	file, err := parser.ParseFile()
	if err != nil {
		t.Fatalf("parsing failed: %v", err)
	}

	// Verify we have a var declaration
	if len(file.Decls) != 1 {
		t.Fatalf("expected 1 declaration, got %d", len(file.Decls))
	}

	genDecl, ok := file.Decls[0].(*ast.GenDecl)
	if !ok {
		t.Fatalf("expected *ast.GenDecl, got %T", file.Decls[0])
	}

	if len(genDecl.Specs) != 1 {
		t.Fatalf("expected 1 spec, got %d", len(genDecl.Specs))
	}

	valueSpec, ok := genDecl.Specs[0].(*ast.ValueSpec)
	if !ok {
		t.Fatalf("expected *ast.ValueSpec, got %T", genDecl.Specs[0])
	}

	// Verify the type is a MapType
	mapType, ok := valueSpec.Type.(*ast.MapType)
	if !ok {
		t.Fatalf("expected *ast.MapType, got %T", valueSpec.Type)
	}

	// Verify key type is string
	keyIdent, ok := mapType.Key.(*ast.Ident)
	if !ok {
		t.Fatalf("expected key type to be *ast.Ident, got %T", mapType.Key)
	}
	if keyIdent.Name != "string" {
		t.Errorf("expected key type 'string', got %q", keyIdent.Name)
	}

	// Verify value type is interface{}
	_, ok = mapType.Value.(*ast.InterfaceType)
	if !ok {
		t.Fatalf("expected value type to be *ast.InterfaceType, got %T", mapType.Value)
	}
}

func TestParseInterfaceType(t *testing.T) {
	input := "package main\nvar x: interface{}"

	tok := tokenizer.New([]byte(input))
	fset := token.NewFileSet()

	parser := NewStmtParser(tok, fset)
	file, err := parser.ParseFile()
	if err != nil {
		t.Fatalf("parsing failed: %v", err)
	}

	genDecl := file.Decls[0].(*ast.GenDecl)
	valueSpec := genDecl.Specs[0].(*ast.ValueSpec)

	// Verify it's an InterfaceType
	interfaceType, ok := valueSpec.Type.(*ast.InterfaceType)
	if !ok {
		t.Fatalf("expected *ast.InterfaceType, got %T", valueSpec.Type)
	}

	// Verify it's empty (no methods)
	if len(interfaceType.Methods.List) != 0 {
		t.Errorf("expected empty interface, got %d methods", len(interfaceType.Methods.List))
	}
}

func TestParseSliceType(t *testing.T) {
	input := "package main\nvar items: []string"

	tok := tokenizer.New([]byte(input))
	fset := token.NewFileSet()

	parser := NewStmtParser(tok, fset)
	file, err := parser.ParseFile()
	if err != nil {
		t.Fatalf("parsing failed: %v", err)
	}

	genDecl := file.Decls[0].(*ast.GenDecl)
	valueSpec := genDecl.Specs[0].(*ast.ValueSpec)

	// Verify it's an ArrayType (slices are represented as ArrayType with nil Len)
	arrayType, ok := valueSpec.Type.(*ast.ArrayType)
	if !ok {
		t.Fatalf("expected *ast.ArrayType, got %T", valueSpec.Type)
	}

	// Verify it's a slice (nil length)
	if arrayType.Len != nil {
		t.Errorf("expected slice (nil Len), got array with length")
	}

	// Verify element type
	eltIdent, ok := arrayType.Elt.(*ast.Ident)
	if !ok {
		t.Fatalf("expected element type to be *ast.Ident, got %T", arrayType.Elt)
	}
	if eltIdent.Name != "string" {
		t.Errorf("expected element type 'string', got %q", eltIdent.Name)
	}
}
