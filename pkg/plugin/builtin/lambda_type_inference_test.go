package builtin

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/MadAppGang/dingo/pkg/plugin"
)

func TestVarDeclTypeInference(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		varName  string
		wantType string
	}{
		{
			name: "string literal assignment",
			source: `package main
func main() {
	var matchResult __TYPE_INFERENCE_NEEDED
	matchResult = "hello"
}`,
			varName:  "matchResult",
			wantType: "string",
		},
		{
			name: "int literal assignment",
			source: `package main
func main() {
	var counter __TYPE_INFERENCE_NEEDED
	counter = 42
}`,
			varName:  "counter",
			wantType: "int",
		},
		{
			name: "float literal assignment",
			source: `package main
func main() {
	var value __TYPE_INFERENCE_NEEDED
	value = 3.14
}`,
			varName:  "value",
			wantType: "float64",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse source
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "test.go", tt.source, parser.ParseComments)
			if err != nil {
				t.Fatalf("Failed to parse source: %v", err)
			}

			// Create plugin with context
			p := NewLambdaTypeInferencePlugin()
			ctx := &plugin.Context{
				FileSet: fset,
				Logger:  plugin.NewNoOpLogger(),
			}
			p.SetContext(ctx)

			// Process the AST
			err = p.Process(file)
			if err != nil {
				t.Fatalf("Process failed: %v", err)
			}

			// Find the variable declaration and check its type
			found := false
			ast.Inspect(file, func(n ast.Node) bool {
				if genDecl, ok := n.(*ast.GenDecl); ok && genDecl.Tok == token.VAR {
					for _, spec := range genDecl.Specs {
						if valueSpec, ok := spec.(*ast.ValueSpec); ok {
							for _, name := range valueSpec.Names {
								if name.Name == tt.varName {
									found = true
									if valueSpec.Type == nil {
										t.Errorf("Type not inferred for %s", tt.varName)
										return false
									}
									if ident, ok := valueSpec.Type.(*ast.Ident); ok {
										if ident.Name != tt.wantType {
											t.Errorf("Wrong type for %s: got %s, want %s",
												tt.varName, ident.Name, tt.wantType)
										}
									} else {
										t.Errorf("Type is not an identifier: %T", valueSpec.Type)
									}
									return false
								}
							}
						}
					}
				}
				return true
			})

			if !found {
				t.Errorf("Variable %s not found in AST", tt.varName)
			}
		})
	}
}

func TestVarDeclTypeInference_MatchPattern(t *testing.T) {
	// Simulate the pattern that match preprocessor would generate
	source := `package main
func main() {
	var matchResult __TYPE_INFERENCE_NEEDED
	switch x := getValue(); x {
	case 1:
		matchResult = "one"
	case 2:
		matchResult = "two"
	default:
		matchResult = "other"
	}
	println(matchResult)
}`

	// Parse source
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", source, parser.ParseComments)
	if err != nil {
		t.Fatalf("Failed to parse source: %v", err)
	}

	// Create plugin with context
	p := NewLambdaTypeInferencePlugin()
	ctx := &plugin.Context{
		FileSet: fset,
		Logger:  plugin.NewNoOpLogger(),
	}
	p.SetContext(ctx)

	// Process the AST
	err = p.Process(file)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	// Find the variable declaration and check its type
	found := false
	ast.Inspect(file, func(n ast.Node) bool {
		if genDecl, ok := n.(*ast.GenDecl); ok && genDecl.Tok == token.VAR {
			for _, spec := range genDecl.Specs {
				if valueSpec, ok := spec.(*ast.ValueSpec); ok {
					for _, name := range valueSpec.Names {
						if name.Name == "matchResult" {
							found = true
							if valueSpec.Type == nil {
								t.Errorf("Type not inferred for matchResult")
								return false
							}
							if ident, ok := valueSpec.Type.(*ast.Ident); ok {
								if ident.Name != "string" {
									t.Errorf("Wrong type for matchResult: got %s, want string", ident.Name)
								}
							} else {
								t.Errorf("Type is not an identifier: %T", valueSpec.Type)
							}
							return false
						}
					}
				}
			}
		}
		return true
	})

	if !found {
		t.Errorf("Variable matchResult not found in AST")
	}
}
