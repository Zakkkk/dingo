package refactor

import (
	"go/parser"
	"go/token"
	"testing"

	dingoast "github.com/MadAppGang/dingo/pkg/ast"
)

func TestNilCheckDetector(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		wantDiag bool
	}{
		{
			name: "if-else nil check - should detect",
			code: `package main

func example(user *User) {
	if user != nil {
		fmt.Println(user.Name)
	} else {
		fmt.Println("No user")
	}
}`,
			wantDiag: true,
		},
		{
			name: "if nil check - should detect",
			code: `package main

func example(user *User) {
	if user == nil {
		fmt.Println("No user")
	} else {
		fmt.Println(user.Name)
	}
}`,
			wantDiag: true,
		},
		{
			name: "if without else - should not detect",
			code: `package main

func example(user *User) {
	if user != nil {
		fmt.Println(user.Name)
	}
}`,
			wantDiag: false,
		},
		{
			name: "non-nil check - should not detect",
			code: `package main

func example(x int) {
	if x > 0 {
		fmt.Println(x)
	} else {
		fmt.Println("zero or negative")
	}
}`,
			wantDiag: false,
		},
	}

	detector := &NilCheckDetector{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, "test.go", tt.code, parser.AllErrors)
			if err != nil {
				t.Fatalf("failed to parse code: %v", err)
			}

			dingoFile := &dingoast.File{File: f}
			diags := detector.Detect(fset, dingoFile, []byte(tt.code))

			if tt.wantDiag && len(diags) == 0 {
				t.Errorf("expected diagnostic, got none")
			}
			if !tt.wantDiag && len(diags) > 0 {
				t.Errorf("expected no diagnostic, got %d", len(diags))
			}

			// Verify diagnostic properties if one was expected
			if tt.wantDiag && len(diags) > 0 {
				diag := diags[0]
				if diag.Code != "R002" {
					t.Errorf("expected code R002, got %s", diag.Code)
				}
				if diag.Category != "refactor" {
					t.Errorf("expected category refactor, got %s", diag.Category)
				}
				if len(diag.Fixes) == 0 {
					t.Errorf("expected at least one fix, got none")
				}
			}
		})
	}
}

func TestGuardLetDetector(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		wantDiag bool
	}{
		{
			name: "early return nil check - should detect",
			code: `package main

func example(config *Config) error {
	if config == nil {
		return errors.New("config required")
	}
	processConfig(config)
	return nil
}`,
			wantDiag: true,
		},
		{
			name: "if-else - should not detect (use NilCheckDetector)",
			code: `package main

func example(config *Config) {
	if config == nil {
		fmt.Println("No config")
	} else {
		processConfig(config)
	}
}`,
			wantDiag: false,
		},
		{
			name: "if not-nil early return - should not detect",
			code: `package main

func example(config *Config) error {
	if config != nil {
		return errors.New("unexpected config")
	}
	return nil
}`,
			wantDiag: false,
		},
		{
			name: "if without return - should not detect",
			code: `package main

func example(config *Config) {
	if config == nil {
		fmt.Println("No config")
	}
}`,
			wantDiag: false,
		},
	}

	detector := &GuardLetDetector{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, "test.go", tt.code, parser.AllErrors)
			if err != nil {
				t.Fatalf("failed to parse code: %v", err)
			}

			dingoFile := &dingoast.File{File: f}
			diags := detector.Detect(fset, dingoFile, []byte(tt.code))

			if tt.wantDiag && len(diags) == 0 {
				t.Errorf("expected diagnostic, got none")
			}
			if !tt.wantDiag && len(diags) > 0 {
				t.Errorf("expected no diagnostic, got %d", len(diags))
			}

			// Verify diagnostic properties if one was expected
			if tt.wantDiag && len(diags) > 0 {
				diag := diags[0]
				if diag.Code != "R004" {
					t.Errorf("expected code R004, got %s", diag.Code)
				}
				if diag.Category != "refactor" {
					t.Errorf("expected category refactor, got %s", diag.Category)
				}
				if len(diag.Fixes) == 0 {
					t.Errorf("expected at least one fix, got none")
				}
			}
		})
	}
}
