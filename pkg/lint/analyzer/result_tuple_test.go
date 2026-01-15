package analyzer

import (
	"go/token"
	"testing"

	"github.com/MadAppGang/dingo/pkg/parser"
)

func TestResultTupleAnalyzer_Name(t *testing.T) {
	a := &ResultTupleAnalyzer{}
	if got := a.Name(); got != "result-tuple-destructure" {
		t.Errorf("Name() = %v, want %v", got, "result-tuple-destructure")
	}
}

func TestResultTupleAnalyzer_Category(t *testing.T) {
	a := &ResultTupleAnalyzer{}
	if got := a.Category(); got != "correctness" {
		t.Errorf("Category() = %v, want %v", got, "correctness")
	}
}

func TestResultTupleAnalyzer_Run(t *testing.T) {
	tests := []struct {
		name        string
		source      string
		wantCount   int
		wantCode    string
		wantMessage string
	}{
		{
			name: "tuple destructure of Result-returning function in same file",
			source: `package main

import "github.com/MadAppGang/dgo"

func GetUser() dgo.Result[*User, error] {
	return dgo.Ok[*User, error](&User{})
}

func main() {
	user, err := GetUser()
	_ = user
	_ = err
}

type User struct{}
`,
			wantCount: 1,
			wantCode:  "D005",
		},
		{
			name: "single assignment of Result - no diagnostic",
			source: `package main

import "github.com/MadAppGang/dgo"

func GetUser() dgo.Result[*User, error] {
	return dgo.Ok[*User, error](&User{})
}

func main() {
	result := GetUser()
	_ = result
}

type User struct{}
`,
			wantCount: 0,
		},
		{
			name: "tuple destructure of tuple-returning function - no diagnostic",
			source: `package main

func GetUser() (*User, error) {
	return &User{}, nil
}

func main() {
	user, err := GetUser()
	_ = user
	_ = err
}

type User struct{}
`,
			wantCount: 0,
		},
		{
			name: "tuple destructure of Result-returning method",
			source: `package main

import "github.com/MadAppGang/dgo"

type Repo struct{}

func (r *Repo) GetByID(id int) dgo.Result[*User, error] {
	return dgo.Ok[*User, error](&User{})
}

func main() {
	repo := &Repo{}
	user, err := repo.GetByID(1)
	_ = user
	_ = err
}

type User struct{}
`,
			wantCount: 1,
			wantCode:  "D005",
		},
		{
			name: "multiple tuple destructures of Result",
			source: `package main

import "github.com/MadAppGang/dgo"

func GetUser() dgo.Result[*User, error] {
	return dgo.Ok[*User, error](&User{})
}

func GetCompany() dgo.Result[*Company, error] {
	return dgo.Ok[*Company, error](&Company{})
}

func main() {
	user, err := GetUser()
	company, err2 := GetCompany()
	_ = user
	_ = err
	_ = company
	_ = err2
}

type User struct{}
type Company struct{}
`,
			wantCount: 2,
			wantCode:  "D005",
		},
		{
			name: "Result type without dgo prefix",
			source: `package main

func GetUser() Result[*User, error] {
	return Ok[*User, error](&User{})
}

func main() {
	user, err := GetUser()
	_ = user
	_ = err
}

type User struct{}
`,
			wantCount: 1,
			wantCode:  "D005",
		},
		{
			name: "triple assignment - not a Result case but checks LHS >= 2",
			source: `package main

func GetMultiple() (int, string, error) {
	return 1, "test", nil
}

func main() {
	a, b, c := GetMultiple()
	_ = a
	_ = b
	_ = c
}
`,
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &ResultTupleAnalyzer{}
			fset := token.NewFileSet()
			src := []byte(tt.source)

			file, err := parser.ParseFile(fset, "test.dingo", src, parser.ParseComments)
			if err != nil {
				t.Fatalf("Failed to parse source: %v", err)
			}

			diagnostics := a.Run(fset, file, src)

			if len(diagnostics) != tt.wantCount {
				t.Errorf("Run() returned %d diagnostics, want %d", len(diagnostics), tt.wantCount)
				for i, d := range diagnostics {
					t.Logf("  Diagnostic %d: [%s] %s at %s", i, d.Code, d.Message, d.Pos)
				}
			}

			if tt.wantCount > 0 && len(diagnostics) > 0 {
				if diagnostics[0].Code != tt.wantCode {
					t.Errorf("First diagnostic code = %s, want %s", diagnostics[0].Code, tt.wantCode)
				}
				if diagnostics[0].Category != "correctness" {
					t.Errorf("First diagnostic category = %s, want correctness", diagnostics[0].Category)
				}
			}
		})
	}
}

func TestResultTupleAnalyzer_DiagnosticContent(t *testing.T) {
	source := `package main

import "github.com/MadAppGang/dgo"

func GetUser() dgo.Result[*User, error] {
	return dgo.Ok[*User, error](&User{})
}

func main() {
	user, err := GetUser()
	_ = user
	_ = err
}

type User struct{}
`

	a := &ResultTupleAnalyzer{}
	fset := token.NewFileSet()
	src := []byte(source)

	file, err := parser.ParseFile(fset, "test.dingo", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("Failed to parse source: %v", err)
	}

	diagnostics := a.Run(fset, file, src)

	if len(diagnostics) != 1 {
		t.Fatalf("Expected 1 diagnostic, got %d", len(diagnostics))
	}

	d := diagnostics[0]

	// Check diagnostic fields
	if d.Code != "D005" {
		t.Errorf("Code = %s, want D005", d.Code)
	}

	if d.Category != "correctness" {
		t.Errorf("Category = %s, want correctness", d.Category)
	}

	if d.Severity != SeverityError {
		t.Errorf("Severity = %v, want SeverityError", d.Severity)
	}

	expectedMsg := "Cannot destructure Result[T, E] as tuple. Use `result := ...` then `result.IsOk()`/`result.MustOk()`, or use `?` for error propagation."
	if d.Message != expectedMsg {
		t.Errorf("Message = %q, want %q", d.Message, expectedMsg)
	}

	// Check that Related info is present
	if len(d.Related) != 1 {
		t.Errorf("Expected 1 related info, got %d", len(d.Related))
	}
}
