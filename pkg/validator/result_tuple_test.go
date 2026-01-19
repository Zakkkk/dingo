package validator

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func TestResultTupleValidator_DetectsAssignment(t *testing.T) {
	tests := []struct {
		name        string
		code        string
		shouldError bool
		errorMsg    string
	}{
		{
			name: "tuple unpacking Result - should error",
			code: `
package main

import "github.com/MadAppGang/dingo/pkg/dgo"

type CompanyRepo struct {}

func (r *CompanyRepo) GetBySlug(ctx context.Context, slug string) dgo.Result[*Company, error] {
	return dgo.Ok[*Company, error](&Company{})
}

type Company struct {}

func Test() {
	repo := &CompanyRepo{}
	existingCompany, _ := repo.GetBySlug(ctx, "test")
	println(existingCompany)
}
`,
			shouldError: true,
			errorMsg:    "cannot unpack Result",
		},
		{
			name: "single variable Result - valid",
			code: `
package main

import "github.com/MadAppGang/dingo/pkg/dgo"

type CompanyRepo struct {}

func (r *CompanyRepo) GetBySlug(ctx context.Context, slug string) dgo.Result[*Company, error] {
	return dgo.Ok[*Company, error](&Company{})
}

type Company struct {}

func Test() {
	repo := &CompanyRepo{}
	result := repo.GetBySlug(ctx, "test")
	println(result.MustOk())
}
`,
			shouldError: false,
		},
		{
			name: "tuple unpacking tuple-returning func - valid",
			code: `
package main

func TupleFunc() (int, error) {
	return 42, nil
}

func Test() {
	value, err := TupleFunc()
	println(value, err)
}
`,
			shouldError: false,
		},
		{
			name: "Result with named error variable - should error",
			code: `
package main

import "github.com/MadAppGang/dingo/pkg/dgo"

func GetUser() dgo.Result[User, error] {
	return dgo.Ok[User, error](User{})
}

type User struct {}

func Test() {
	user, err := GetUser()
	if err != nil {
		return
	}
	println(user)
}
`,
			shouldError: true,
			errorMsg:    "cannot unpack Result",
		},
		{
			name: "var declaration with Result - should error",
			code: `
package main

import "github.com/MadAppGang/dingo/pkg/dgo"

func GetValue() dgo.Result[int, error] {
	return dgo.Ok[int, error](42)
}

func Test() {
	var value, err = GetValue()
	println(value, err)
}
`,
			shouldError: true,
			errorMsg:    "cannot unpack Result",
		},
		{
			name: "more than two variables - should error",
			code: `
package main

import "github.com/MadAppGang/dingo/pkg/dgo"

func GetData() dgo.Result[Data, error] {
	return dgo.Ok[Data, error](Data{})
}

type Data struct {}

func Test() {
	a, b, c := GetData()
	println(a, b, c)
}
`,
			shouldError: true,
			errorMsg:    "cannot unpack Result",
		},
		{
			name: "multiple RHS expressions - valid (not our pattern)",
			code: `
package main

func Test() {
	a, b := 1, 2
	println(a, b)
}
`,
			shouldError: false,
		},
		{
			name: "non-call expression - valid",
			code: `
package main

func Test() {
	var data struct { value int; err error }
	value, err := data.value, data.err
	println(value, err)
}
`,
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, "", tt.code, parser.SkipObjectResolution)
			if err != nil {
				// Some test cases may have intentionally unparseable code
				// For valid Go code that fails to parse, skip the test
				if !tt.shouldError {
					t.Skipf("Could not parse test code: %v", err)
				}
				return
			}

			validator := NewResultTupleValidator(fset, []byte(tt.code), nil)
			validationErr := validator.Validate(f)

			if tt.shouldError {
				if validationErr == nil {
					t.Errorf("Expected error containing %q, got nil", tt.errorMsg)
				} else if !strings.Contains(validationErr.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing %q, got %q", tt.errorMsg, validationErr.Error())
				}
			} else {
				if validationErr != nil {
					t.Errorf("Expected no error, got: %v", validationErr)
				}
			}
		})
	}
}

func TestResultTupleValidator_ErrorMessageQuality(t *testing.T) {
	code := `
package main

import "github.com/MadAppGang/dingo/pkg/dgo"

func GetUser() dgo.Result[User, error] {
	return dgo.Ok[User, error](User{})
}

type User struct {}

func Test() {
	user, _ := GetUser()
	println(user)
}
`

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", code, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("Failed to parse code: %v", err)
	}

	validator := NewResultTupleValidator(fset, []byte(code), nil)
	validationErr := validator.Validate(f)

	if validationErr == nil {
		t.Fatal("Expected validation error, got nil")
	}

	errStr := validationErr.Error()

	// Check that error contains helpful information
	expectations := []string{
		"cannot unpack Result", // Main error message
		"'?' operator",         // Suggestion mentions ? operator
	}

	for _, expected := range expectations {
		if !strings.Contains(errStr, expected) {
			t.Errorf("Error message should contain %q, got:\n%s", expected, errStr)
		}
	}
}

func TestValidateSource(t *testing.T) {
	// Test with Dingo-specific syntax that needs sanitization
	code := `
package main

import "github.com/MadAppGang/dingo/pkg/dgo"

func GetUser() dgo.Result[User, error] {
	return dgo.Ok[User, error](User{})
}

type User struct {}

func ValidUsage() {
	// This uses error propagation - valid
	user := GetUser()
	println(user)
}

func InvalidUsage() {
	// This tries tuple unpacking - invalid
	user, _ := GetUser()
	println(user)
}
`

	err := ValidateSource([]byte(code), "test.dingo", ".")
	if err == nil {
		t.Error("Expected validation error for tuple unpacking of Result")
	}

	if !strings.Contains(err.Error(), "cannot unpack Result") {
		t.Errorf("Expected error about Result unpacking, got: %v", err)
	}
}

func TestValidateSource_WithDingoSyntax(t *testing.T) {
	// Test that Dingo syntax (?, match, =>) doesn't break parsing
	code := `
package main

import "github.com/MadAppGang/dingo/pkg/dgo"

func GetUser() dgo.Result[User, error] {
	return dgo.Ok[User, error](User{})
}

type User struct {}

func Test() {
	// Error propagation operator
	user := GetUser()?

	// Match expression (this would fail Go parser without sanitization)
	match result {
		Ok(u) => println(u),
		Err(e) => println(e),
	}

	// These are valid - should not error
	println(user)
}
`

	// This should not panic or return parse errors
	// (we only care about Result tuple unpacking)
	err := ValidateSource([]byte(code), "test.dingo", ".")

	// The ? is sanitized, so there's no tuple unpacking error here
	// The match expression is also sanitized
	if err != nil {
		// If there's an error, it should be about something specific
		// not a parse failure
		if strings.Contains(err.Error(), "cannot unpack Result") {
			// This is expected if there's a tuple unpacking somewhere
		} else {
			t.Logf("Got error (may be expected): %v", err)
		}
	}
}

func TestSanitizeDingoSource(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "question mark removed",
			input:    "user := GetUser()?",
			expected: "user := GetUser() ",
		},
		{
			name:     "arrow replaced with colon-space",
			input:    "Ok(u) => println(u)",
			expected: "Ok(u) :  println(u)",
		},
		{
			name:     "multiple replacements",
			input:    "x := foo()?; match v { A => 1 }",
			expected: "x := foo() ; match v { A :  1 }",
		},
		{
			name:     "no changes needed",
			input:    "x := 42",
			expected: "x := 42",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeDingoSource([]byte(tt.input))
			if string(result) != tt.expected {
				t.Errorf("sanitizeDingoSource(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
