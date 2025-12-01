package preprocessor

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"testing"
)

// TestOptionDetectorByName tests naming convention detection
func TestOptionDetectorByName(t *testing.T) {
	detector := NewOptionDetector()

	tests := []struct {
		name     string
		typeName string
		want     bool
	}{
		// Positive cases
		{"Plain Option", "Option", true},
		{"Pointer Option", "*Option", true},
		{"UserOption", "UserOption", true},
		{"StringOption", "StringOption", true},
		{"Generic Option[T]", "Option[string]", true},
		{"Generic Option[int]", "Option[int]", true},
		{"Custom ConfigOption", "ConfigOption", true},

		// Negative cases
		{"Empty string", "", false},
		{"Regular type User", "User", false},
		{"Regular type string", "string", false},
		{"Regular type int", "int", false},
		{"Pointer User", "*User", false},
		{"Similar but not Option", "Optional", false}, // Intentionally strict
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detector.IsOptionByName(tt.typeName)
			if got != tt.want {
				t.Errorf("IsOptionByName(%q) = %v, want %v", tt.typeName, got, tt.want)
			}
		})
	}
}

// TestOptionDetectorByMethods tests method signature detection
func TestOptionDetectorByMethods(t *testing.T) {
	detector := NewOptionDetector()

	// Create test type with Option methods
	src := `
package test

type UserOption struct {
	value *User
	valid bool
}

func (u UserOption) IsNone() bool { return !u.valid }
func (u UserOption) IsSome() bool { return u.valid }
func (u UserOption) Unwrap() User { return *u.value }
func (u UserOption) UnwrapOr(def User) User {
	if u.valid {
		return *u.value
	}
	return def
}

// Type with only 1 method (should fail)
type PartialOption struct {
	value *string
}
func (p PartialOption) IsNone() bool { return p.value == nil }

// Type with 2 methods (should pass)
type MinimalOption struct {
	value *int
}
func (m MinimalOption) IsNone() bool { return m.value == nil }
func (m MinimalOption) Unwrap() int { return *m.value }

// Type with no Option methods
type RegularStruct struct {
	name string
}
func (r RegularStruct) GetName() string { return r.name }

type User struct {
	name string
}
`

	// Parse and type-check
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	conf := types.Config{Importer: importer.Default()}
	pkg, err := conf.Check("test", fset, []*ast.File{f}, nil)
	if err != nil {
		t.Fatalf("Failed to type-check: %v", err)
	}

	tests := []struct {
		name     string
		typeName string
		want     bool
	}{
		{"UserOption with all methods", "UserOption", true},
		{"MinimalOption with 2 methods", "MinimalOption", true},
		{"PartialOption with 1 method", "PartialOption", false},
		{"RegularStruct no Option methods", "RegularStruct", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := pkg.Scope().Lookup(tt.typeName)
			if obj == nil {
				t.Fatalf("Type %s not found", tt.typeName)
			}

			typ := obj.Type()
			got := detector.IsOptionByMethods(typ)
			if got != tt.want {
				t.Errorf("IsOptionByMethods(%s) = %v, want %v", tt.typeName, got, tt.want)
			}
		})
	}
}

// TestOptionDetectorDualStrategy tests combined naming + methods
func TestOptionDetectorDualStrategy(t *testing.T) {
	detector := NewOptionDetector()

	src := `
package test

// Type matching naming convention (no methods needed)
type StringOption struct {
	value *string
}

// Type with methods but no "Option" in name
type MaybeUser struct {
	value *User
}
func (m MaybeUser) IsNone() bool { return m.value == nil }
func (m MaybeUser) Unwrap() User { return *m.value }

// Type with neither naming nor methods
type RegularStruct struct {
	name string
}

type User struct {
	name string
}
`

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	conf := types.Config{Importer: importer.Default()}
	pkg, err := conf.Check("test", fset, []*ast.File{f}, nil)
	if err != nil {
		t.Fatalf("Failed to type-check: %v", err)
	}

	tests := []struct {
		name     string
		typeName string
		want     bool
		reason   string
	}{
		{
			"StringOption matches by name",
			"StringOption",
			true,
			"Has 'Option' in name",
		},
		{
			"MaybeUser matches by methods",
			"MaybeUser",
			true,
			"Has IsNone + Unwrap methods",
		},
		{
			"RegularStruct matches neither",
			"RegularStruct",
			false,
			"No 'Option' in name, no methods",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := pkg.Scope().Lookup(tt.typeName)
			if obj == nil {
				t.Fatalf("Type %s not found", tt.typeName)
			}

			typ := obj.Type()
			got := detector.IsOption(typ)
			if got != tt.want {
				t.Errorf("IsOption(%s) = %v, want %v (%s)", tt.typeName, got, tt.want, tt.reason)
			}
		})
	}
}

// TestGetInnerType tests extraction of inner type T from Option[T]
func TestGetInnerType(t *testing.T) {
	detector := NewOptionDetector()

	src := `
package test

// Non-generic Option with Unwrap method
type UserOption struct {
	value *User
	valid bool
}
func (u UserOption) Unwrap() User { return *u.value }

// Type without Unwrap
type NoUnwrap struct {
	value *string
}

type User struct {
	name string
}
`

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	conf := types.Config{Importer: importer.Default()}
	pkg, err := conf.Check("test", fset, []*ast.File{f}, nil)
	if err != nil {
		t.Fatalf("Failed to type-check: %v", err)
	}

	tests := []struct {
		name      string
		typeName  string
		wantInner string
		wantOk    bool
	}{
		{
			"UserOption has Unwrap() User",
			"UserOption",
			"test.User",
			true,
		},
		{
			"NoUnwrap has no Unwrap method",
			"NoUnwrap",
			"",
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := pkg.Scope().Lookup(tt.typeName)
			if obj == nil {
				t.Fatalf("Type %s not found", tt.typeName)
			}

			typ := obj.Type()
			innerType, ok := detector.GetInnerType(typ)

			if ok != tt.wantOk {
				t.Errorf("GetInnerType(%s) ok = %v, want %v", tt.typeName, ok, tt.wantOk)
				return
			}

			if ok {
				innerName := innerType.String()
				if innerName != tt.wantInner {
					t.Errorf("GetInnerType(%s) inner = %s, want %s", tt.typeName, innerName, tt.wantInner)
				}
			}
		})
	}
}

// TestTypeNameFromType tests the helper function
func TestTypeNameFromType(t *testing.T) {
	src := `
package test

type UserOption struct {}
type User struct {}
`

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	conf := types.Config{Importer: importer.Default()}
	pkg, err := conf.Check("test", fset, []*ast.File{f}, nil)
	if err != nil {
		t.Fatalf("Failed to type-check: %v", err)
	}

	tests := []struct {
		name     string
		typeName string
		want     string
	}{
		{"UserOption", "UserOption", "UserOption"},
		{"User", "User", "User"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := pkg.Scope().Lookup(tt.typeName)
			if obj == nil {
				t.Fatalf("Type %s not found", tt.typeName)
			}

			typ := obj.Type()
			got := typeNameFromType(typ)
			if got != tt.want {
				t.Errorf("typeNameFromType(%s) = %s, want %s", tt.typeName, got, tt.want)
			}
		})
	}
}

// TestPointerTypes tests Option detection with pointer types
func TestPointerTypes(t *testing.T) {
	detector := NewOptionDetector()

	src := `
package test

type UserOption struct {
	value *User
}
func (u *UserOption) IsNone() bool { return u.value == nil }
func (u *UserOption) Unwrap() User { return *u.value }

type User struct {
	name string
}
`

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	conf := types.Config{Importer: importer.Default()}
	pkg, err := conf.Check("test", fset, []*ast.File{f}, nil)
	if err != nil {
		t.Fatalf("Failed to type-check: %v", err)
	}

	// Get the UserOption type
	obj := pkg.Scope().Lookup("UserOption")
	if obj == nil {
		t.Fatalf("Type UserOption not found")
	}

	typ := obj.Type()

	// Test non-pointer
	t.Run("Non-pointer UserOption", func(t *testing.T) {
		got := detector.IsOption(typ)
		if !got {
			t.Errorf("IsOption(UserOption) = false, want true (has methods on pointer receiver)")
		}
	})

	// Test pointer
	t.Run("Pointer *UserOption", func(t *testing.T) {
		ptrTyp := types.NewPointer(typ)
		got := detector.IsOption(ptrTyp)
		if !got {
			t.Errorf("IsOption(*UserOption) = false, want true")
		}
	})
}

// TestEdgeCases tests edge cases and error conditions
func TestEdgeCases(t *testing.T) {
	detector := NewOptionDetector()

	t.Run("Nil type", func(t *testing.T) {
		got := detector.IsOption(nil)
		if got {
			t.Errorf("IsOption(nil) = true, want false")
		}
	})

	t.Run("Empty type name", func(t *testing.T) {
		got := detector.IsOptionByName("")
		if got {
			t.Errorf("IsOptionByName(\"\") = true, want false")
		}
	})

	t.Run("GetInnerType with nil", func(t *testing.T) {
		_, ok := detector.GetInnerType(nil)
		if ok {
			t.Errorf("GetInnerType(nil) ok = true, want false")
		}
	})
}
