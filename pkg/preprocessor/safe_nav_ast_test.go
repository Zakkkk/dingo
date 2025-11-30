package preprocessor

import (
	"strings"
	"testing"
)

func TestSafeNavASTProcessor_PropertyAccess_Option(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains []string // Strings that must appear in output
	}{
		{
			name: "simple property access",
			input: `let user: UserOption = getUser()
let name = user?.name`,
			contains: []string{
				"func() __INFER__",
				"if user.IsNone()",
				"return __INFER__None()",
				"user.Unwrap()",
				"user :=",
				"return __INFER__Some(user.name)",
			},
		},
		{
			name: "chained property access",
			input: `let user: UserOption = getUser()
let city = user?.address?.city`,
			contains: []string{
				"func() __INFER__",
				"if user.IsNone()",
				"user := user.Unwrap()",
				"if user.address.IsNone()",
				"user1 := user.address.Unwrap()",
				"return __INFER__Some(user1.city)",
			},
		},
		{
			name: "three-level chain",
			input: `let user: UserOption = getUser()
let value = user?.profile?.settings?.theme`,
			contains: []string{
				"func() __INFER__",
				"if user.IsNone()",
				"user.Unwrap()",
				"user.profile.IsNone()",
				"user1.settings.IsNone()",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor := NewSafeNavASTProcessor()
			output, _, err := processor.ProcessInternal(tt.input)
			if err != nil {
				t.Fatalf("ProcessInternal() error = %v", err)
			}

			result := output

			// Check required strings
			for _, str := range tt.contains {
				if !strings.Contains(result, str) {
					t.Errorf("Output missing expected string: %q\nGot:\n%s", str, result)
				}
			}
		})
	}
}

func TestSafeNavASTProcessor_PropertyAccess_Pointer(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains []string
	}{
		{
			name: "pointer simple access",
			input: `let user: *User = getUser()
let name = user?.name`,
			contains: []string{
				"func() __INFER__",
				"if user == nil",
				"return nil",
				"return user.name",
			},
		},
		{
			name: "pointer chained access",
			input: `let user: *User = getUser()
let city = user?.address?.city`,
			contains: []string{
				"func() __INFER__",
				"if user == nil",
				"userTmp := user.address",
				"if userTmp == nil",
				"return userTmp.city",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor := NewSafeNavASTProcessor()
			output, _, err := processor.ProcessInternal(tt.input)
			if err != nil {
				t.Fatalf("ProcessInternal() error = %v", err)
			}

			result := output

			// Check required strings
			for _, str := range tt.contains {
				if !strings.Contains(result, str) {
					t.Errorf("Output missing expected string: %q\nGot:\n%s", str, result)
				}
			}
		})
	}
}

func TestSafeNavASTProcessor_MethodCalls(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains []string
	}{
		{
			name: "simple method call",
			input: `let user: UserOption = getUser()
let name = user?.getName()`,
			contains: []string{
				"func() __INFER__",
				"if user.IsNone()",
				"user.Unwrap()",
				".getName()",
			},
		},
		{
			name: "method with arguments",
			input: `let user: UserOption = getUser()
let result = user?.process(42, "test")`,
			contains: []string{
				"func() __INFER__",
				".process(42, \"test\")",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor := NewSafeNavASTProcessor()
			output, _, err := processor.ProcessInternal(tt.input)
			if err != nil {
				t.Fatalf("ProcessInternal() error = %v", err)
			}

			result := output

			// Check required strings
			for _, str := range tt.contains {
				if !strings.Contains(result, str) {
					t.Errorf("Output missing expected string: %q\nGot:\n%s", str, result)
				}
			}
		})
	}
}

func TestSafeNavASTProcessor_Errors(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantErrText string
	}{
		{
			name:        "trailing ?. operator",
			input:       `let user: UserOption = getUser()\nlet x = user?.`,
			wantErrText: "trailing safe navigation operator",
		},
		{
			name:        "unknown type",
			input:       `let x = unknown?.field`,
			wantErrText: "cannot infer type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor := NewSafeNavASTProcessor()
			_, _, err := processor.ProcessInternal(tt.input)
			if err == nil {
				t.Errorf("ProcessInternal() expected error containing %q, got nil", tt.wantErrText)
				return
			}
			if !strings.Contains(err.Error(), tt.wantErrText) {
				t.Errorf("ProcessInternal() error = %v, want error containing %q", err, tt.wantErrText)
			}
		})
	}
}

func TestSafeNavASTProcessor_Comments(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "?. in line comment",
			input: `let x = 42 // user?.name`,
			want:  `let x = 42 // user?.name`,
		},
		{
			name: "?. before comment",
			input: `let user: UserOption = getUser()
let name = user?.name // get name`,
			want: `func() __INFER__`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor := NewSafeNavASTProcessor()
			output, _, err := processor.ProcessInternal(tt.input)
			if err != nil {
				t.Fatalf("ProcessInternal() error = %v", err)
			}

			if !strings.Contains(output, tt.want) {
				t.Errorf("ProcessInternal() = %v, want to contain %v", output, tt.want)
			}
		})
	}
}

func TestSafeNavASTProcessor_Metadata(t *testing.T) {
	input := `let user: UserOption = getUser()
let name = user?.name`

	processor := NewSafeNavASTProcessor()
	output, metadata, err := processor.ProcessInternal(input)
	if err != nil {
		t.Fatalf("ProcessInternal() error = %v", err)
	}

	// Should have metadata
	if len(metadata) == 0 {
		t.Error("ProcessInternal() should generate metadata for ?. operator")
	}

	// Should have marker in output
	if !strings.Contains(output, "// dingo:s:") {
		t.Error("ProcessInternal() should include source map marker in output")
	}

	// Check metadata fields
	if len(metadata) > 0 {
		m := metadata[0]
		if m.Type != "safe_nav" {
			t.Errorf("metadata[0].Type = %v, want safe_nav", m.Type)
		}
		if m.OriginalText != "?." {
			t.Errorf("metadata[0].OriginalText = %v, want ?.", m.OriginalText)
		}
		if m.ASTNodeType != "CallExpr" {
			t.Errorf("metadata[0].ASTNodeType = %v, want CallExpr", m.ASTNodeType)
		}
	}
}
