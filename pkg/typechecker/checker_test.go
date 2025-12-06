package typechecker

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func TestCheckerBasic(t *testing.T) {
	src := `package main

type User struct {
	Name string
	Age  int
}

func getUser() *User {
	return &User{Name: "test", Age: 30}
}

func main() {
	u := getUser()
	_ = u.Name
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

	// Verify we have type info
	if checker.Info() == nil {
		t.Error("expected type info")
	}

	// Verify we have some types recorded
	if len(checker.Info().Types) == 0 {
		t.Error("expected some types to be recorded")
	}
}

func TestFieldType(t *testing.T) {
	src := `package main

type Address struct {
	City    string
	ZipCode int
}

type User struct {
	Name    string
	Address *Address
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

	// Find the User type
	var userType *ast.TypeSpec
	ast.Inspect(file, func(n ast.Node) bool {
		if ts, ok := n.(*ast.TypeSpec); ok && ts.Name.Name == "User" {
			userType = ts
			return false
		}
		return true
	})

	if userType == nil {
		t.Fatal("User type not found")
	}

	// Get the type object
	obj := checker.ObjectOf(userType.Name)
	if obj == nil {
		t.Fatal("User object not found")
	}

	userT := obj.Type()

	// Test FieldType
	nameType := FieldType(userT, "Name")
	if nameType == nil {
		t.Error("Name field type not found")
	} else if TypeString(nameType) != "string" {
		t.Errorf("expected Name type 'string', got %q", TypeString(nameType))
	}

	addrType := FieldType(userT, "Address")
	if addrType == nil {
		t.Error("Address field type not found")
	} else if !IsPointer(addrType) {
		t.Errorf("expected Address to be pointer, got %q", TypeString(addrType))
	}
}

func TestIsNilable(t *testing.T) {
	src := `package main

type User struct {
	Name string
}

var (
	ptr   *User
	slice []int
	m     map[string]int
	ch    chan int
	iface interface{}
	fn    func()
	str   string
	num   int
)
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

	tests := []struct {
		name     string
		expected bool
	}{
		{"ptr", true},
		{"slice", true},
		{"m", true},
		{"ch", true},
		{"iface", true},
		{"fn", true},
		{"str", false},
		{"num", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Find the variable
			var varIdent *ast.Ident
			ast.Inspect(file, func(n ast.Node) bool {
				if vs, ok := n.(*ast.ValueSpec); ok {
					for _, name := range vs.Names {
						if name.Name == tt.name {
							varIdent = name
							return false
						}
					}
				}
				return true
			})

			if varIdent == nil {
				t.Fatalf("variable %s not found", tt.name)
			}

			obj := checker.ObjectOf(varIdent)
			if obj == nil {
				t.Fatalf("object for %s not found", tt.name)
			}

			result := IsNilable(obj.Type())
			if result != tt.expected {
				t.Errorf("IsNilable(%s) = %v, want %v", tt.name, result, tt.expected)
			}
		})
	}
}

func TestMethodType(t *testing.T) {
	src := `package main

type User struct {
	name string
}

func (u *User) GetName() string {
	return u.name
}

func (u User) Age() int {
	return 30
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

	// Find the User type
	var userType *ast.TypeSpec
	ast.Inspect(file, func(n ast.Node) bool {
		if ts, ok := n.(*ast.TypeSpec); ok && ts.Name.Name == "User" {
			userType = ts
			return false
		}
		return true
	})

	if userType == nil {
		t.Fatal("User type not found")
	}

	obj := checker.ObjectOf(userType.Name)
	if obj == nil {
		t.Fatal("User object not found")
	}

	userT := obj.Type()

	// Test MethodType for GetName (pointer receiver)
	getNameSig := MethodType(userT, "GetName")
	if getNameSig == nil {
		t.Error("GetName method not found")
	} else {
		resultT := ResultType(getNameSig)
		if resultT == nil {
			t.Error("GetName has no return type")
		} else if TypeString(resultT) != "string" {
			t.Errorf("expected GetName return 'string', got %q", TypeString(resultT))
		}
	}

	// Test MethodType for Age (value receiver)
	ageSig := MethodType(userT, "Age")
	if ageSig == nil {
		t.Error("Age method not found")
	} else {
		resultT := ResultType(ageSig)
		if resultT == nil {
			t.Error("Age has no return type")
		} else if TypeString(resultT) != "int" {
			t.Errorf("expected Age return 'int', got %q", TypeString(resultT))
		}
	}
}
