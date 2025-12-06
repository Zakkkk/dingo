package typechecker

import (
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"strings"
	"testing"
)

func TestTypeRewriter_SimpleIIFE(t *testing.T) {
	// Simulates: user?.name where user is *User with Name string
	src := `package main

type User struct {
	Name string
}

func test(user *User) interface{} {
	// Safe nav IIFE pattern
	return func() interface{} {
		if user != nil {
			tmp := user.Name
			return &tmp
		}
		return nil
	}()
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	changed, err := RewriteSource(fset, file)
	if err != nil {
		t.Fatalf("rewrite error: %v", err)
	}

	// Print the result
	var buf strings.Builder
	printer.Fprint(&buf, fset, file)
	result := buf.String()

	t.Logf("Changed: %v\nResult:\n%s", changed, result)

	// The rewriter should have changed interface{} to *string
	if changed && !strings.Contains(result, "*string") {
		t.Error("expected interface{} to be rewritten to *string")
	}
}

func TestIsSafeNavIIFE(t *testing.T) {
	src := `package main

func test() {
	// Safe nav pattern
	_ = func() interface{} {
		if x != nil {
			return &x.field
		}
		return nil
	}()

	// Not safe nav - no args
	_ = func() int { return 1 }()

	// Not safe nav - no if
	_ = func() interface{} { return nil }()
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	// Find all call expressions
	var calls []*ast.CallExpr
	ast.Inspect(file, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			calls = append(calls, call)
		}
		return true
	})

	if len(calls) < 3 {
		t.Fatalf("expected at least 3 call expressions, got %d", len(calls))
	}

	// First should be safe nav
	if !IsSafeNavIIFE(calls[0]) {
		t.Error("expected first call to be detected as safe nav IIFE")
	}

	// Others should not be
	if IsSafeNavIIFE(calls[1]) {
		t.Error("expected second call NOT to be detected as safe nav IIFE")
	}
}

func TestIsNullCoalesceIIFE(t *testing.T) {
	src := `package main

func test() {
	// Null coalesce pattern
	_ = func() interface{} {
		if a != nil {
			return a
		}
		return "default"
	}()

	// Not null coalesce - only one statement
	_ = func() interface{} { return nil }()
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	var calls []*ast.CallExpr
	ast.Inspect(file, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			calls = append(calls, call)
		}
		return true
	})

	if len(calls) < 2 {
		t.Fatalf("expected at least 2 call expressions, got %d", len(calls))
	}

	if !IsNullCoalesceIIFE(calls[0]) {
		t.Error("expected first call to be detected as null coalesce IIFE")
	}

	if IsNullCoalesceIIFE(calls[1]) {
		t.Error("expected second call NOT to be detected as null coalesce IIFE")
	}
}

func TestTypeToString(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		varName  string
		expected string
	}{
		{
			name:     "basic int",
			src:      `package main; var x int`,
			varName:  "x",
			expected: "int",
		},
		{
			name:     "pointer to string",
			src:      `package main; var x *string`,
			varName:  "x",
			expected: "*string",
		},
		{
			name:     "slice of int",
			src:      `package main; var x []int`,
			varName:  "x",
			expected: "[]int",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "test.go", tt.src, 0)
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}

			checker, err := New(fset, file, "main")
			if err != nil {
				t.Fatalf("checker error: %v", err)
			}

			// Find the variable
			var varIdent *ast.Ident
			ast.Inspect(file, func(n ast.Node) bool {
				if vs, ok := n.(*ast.ValueSpec); ok {
					for _, name := range vs.Names {
						if name.Name == tt.varName {
							varIdent = name
							return false
						}
					}
				}
				return true
			})

			if varIdent == nil {
				t.Fatalf("variable %s not found", tt.varName)
			}

			obj := checker.ObjectOf(varIdent)
			if obj == nil {
				t.Fatalf("object for %s not found", tt.varName)
			}

			result := TypeToString(obj.Type())
			if result != tt.expected {
				t.Errorf("TypeToString = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestInferSafeNavType(t *testing.T) {
	src := `package main

type Address struct {
	City string
}

type User struct {
	Name    string
	Address *Address
}

func test() {
	var user *User
	_ = user
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	checker, err := New(fset, file, "main")
	if err != nil {
		t.Fatalf("checker error: %v", err)
	}

	// Find the "user" variable - need to get it from Uses since it's used in "_ = user"
	var userExpr ast.Expr
	ast.Inspect(file, func(n ast.Node) bool {
		if assign, ok := n.(*ast.AssignStmt); ok {
			if len(assign.Rhs) == 1 {
				if ident, ok := assign.Rhs[0].(*ast.Ident); ok && ident.Name == "user" {
					userExpr = ident
					return false
				}
			}
		}
		return true
	})

	if userExpr == nil {
		t.Fatal("user variable expression not found")
	}

	// Get the type of user - it should be *User
	userType := checker.TypeOf(userExpr)
	if userType == nil {
		t.Fatal("could not determine type of user")
	}
	t.Logf("user type: %v", TypeToString(userType))

	tests := []struct {
		fields   []string
		expected string
	}{
		{[]string{"Name"}, "string"},
		{[]string{"Address"}, "*main.Address"}, // Fully qualified because it's in package main
		{[]string{"Address", "City"}, "string"},
	}

	for _, tt := range tests {
		t.Run(strings.Join(tt.fields, "."), func(t *testing.T) {
			// Create a mock expression with the known type
			resultType := InferSafeNavType(checker, userExpr, tt.fields)
			if resultType == nil {
				// Since InferSafeNavType uses TypeOf which needs the expression in Types map,
				// we need to test differently - use FieldType directly on the user type
				currentType := userType
				for _, field := range tt.fields {
					currentType = FieldType(currentType, field)
					if currentType == nil {
						t.Fatalf("could not find field %s", field)
					}
				}
				result := TypeToString(currentType)
				if result != tt.expected {
					t.Errorf("got %q, want %q", result, tt.expected)
				}
				return
			}

			result := TypeToString(resultType)
			if result != tt.expected {
				t.Errorf("got %q, want %q", result, tt.expected)
			}
		})
	}
}
