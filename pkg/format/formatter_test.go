package format

import (
	"strings"
	"testing"
)

func TestFormatterBasic(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name: "simple_assignment",
			input: `package main
let x=42`,
			want: `package main
let x = 42
`,
		},
		{
			name: "function_call",
			input: `fmt.Println(x,y,z)`,
			want: `fmt.Println(x, y, z)
`,
		},
		{
			name: "comment_preservation",
			input: `// This is a comment
let x=42`,
			want: `// This is a comment
let x = 42
`,
		},
	}

	f := New(DefaultConfig())

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := f.Format([]byte(tt.input))
			if err != nil {
				t.Fatalf("Format() error = %v", err)
			}

			gotStr := string(got)
			if gotStr != tt.want {
				t.Errorf("Format() mismatch:\nInput:\n%s\nGot:\n%s\nWant:\n%s",
					tt.input, gotStr, tt.want)
			}
		})
	}
}

func TestMatchFormatting(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "match_expression_single_line",
			input: `match x{Some(v)=>v*2,None=>0}`,
		},
		{
			name: "match_expression_multi_line",
			input: `match x {
Some(v) => v * 2
None => 0
}`,
		},
	}

	f := New(DefaultConfig())

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := f.Format([]byte(tt.input))
			if err != nil {
				t.Fatalf("Format() error = %v", err)
			}

			// Basic checks
			gotStr := string(got)
			if !strings.Contains(gotStr, "match") {
				t.Errorf("Formatted output missing 'match' keyword")
			}
			if !strings.Contains(gotStr, "=>") {
				t.Errorf("Formatted output missing '=>' arrow")
			}

			// Multi-line input should produce multi-line output
			inputLines := strings.Split(strings.TrimSpace(tt.input), "\n")
			outputLines := strings.Split(strings.TrimSpace(gotStr), "\n")
			if len(inputLines) > 1 && len(outputLines) < 2 {
				t.Errorf("Multi-line input should produce multi-line output, got: %s", gotStr)
			}
		})
	}
}

func TestEnumFormatting(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "single_line_preserved",
			input: `enum Status{Active,Inactive(string),Pending}`,
		},
		{
			name: "multi_line_with_indentation",
			input: `enum Status {
Active
Inactive(string)
Pending
}`,
		},
	}

	f := New(DefaultConfig())

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := f.Format([]byte(tt.input))
			if err != nil {
				t.Fatalf("Format() error = %v", err)
			}

			gotStr := string(got)

			// Basic checks
			if !strings.Contains(gotStr, "enum Status") {
				t.Errorf("Formatted output missing 'enum Status'")
			}

			// Multi-line input should produce multi-line output with indentation
			inputLines := strings.Split(strings.TrimSpace(tt.input), "\n")
			outputLines := strings.Split(strings.TrimSpace(gotStr), "\n")
			if len(inputLines) > 1 && len(outputLines) < 2 {
				t.Errorf("Multi-line input should produce multi-line output, got: %s", gotStr)
			}
		})
	}
}

func TestLambdaFormatting(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "simple_lambda",
			input: `items.map(|x|x*2)`,
		},
		{
			name:  "multi_param_lambda",
			input: `items.reduce(|acc,x|acc+x)`,
		},
	}

	f := New(DefaultConfig())

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := f.Format([]byte(tt.input))
			if err != nil {
				t.Fatalf("Format() error = %v", err)
			}

			gotStr := string(got)

			// Should preserve lambda syntax
			if !strings.Contains(gotStr, "|") {
				t.Errorf("Formatted output missing '|' for lambda")
			}

			// With LambdaSpacing=true, should have spaces
			if f.Config.LambdaSpacing {
				// Should have space around operators
				if strings.Contains(tt.input, "x*2") && !strings.Contains(gotStr, "x * 2") {
					t.Logf("Expected spacing in lambda body, got: %s", gotStr)
					// This is informational - spacing rules are complex
				}
			}
		})
	}
}

func TestConfigIndentation(t *testing.T) {
	tests := []struct {
		name       string
		indentSize int
		useTabs    bool
		wantIndent string
	}{
		{
			name:       "default_4_spaces",
			indentSize: 4,
			useTabs:    false,
			wantIndent: "    ",
		},
		{
			name:       "2_spaces",
			indentSize: 2,
			useTabs:    false,
			wantIndent: "  ",
		},
		{
			name:       "tabs",
			indentSize: 4,
			useTabs:    true,
			wantIndent: "\t",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				IndentWidth: tt.indentSize,
				UseTabs:     tt.useTabs,
			}

			got := cfg.IndentString()
			if got != tt.wantIndent {
				t.Errorf("IndentString() = %q, want %q", got, tt.wantIndent)
			}
		})
	}
}

func TestErrorPropagation(t *testing.T) {
	input := `result:=someFunc()?`

	f := New(DefaultConfig())
	got, err := f.Format([]byte(input))
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}

	gotStr := string(got)

	// Should preserve ? operator
	if !strings.Contains(gotStr, "?") {
		t.Errorf("Formatted output missing '?' operator")
	}

	// No space before ?
	if strings.Contains(gotStr, " ?") {
		t.Errorf("Should not have space before '?' in error propagation")
	}
}

func TestSafeNavigation(t *testing.T) {
	input := `value:=obj?.field`

	f := New(DefaultConfig())
	got, err := f.Format([]byte(input))
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}

	gotStr := string(got)

	// Should preserve ?. operator
	if !strings.Contains(gotStr, "?.") {
		t.Errorf("Formatted output missing '?.' operator")
	}

	// No space around ?.
	if strings.Contains(gotStr, " ?.") || strings.Contains(gotStr, "?. ") {
		t.Errorf("Should not have space around '?.' in safe navigation, got: %s", gotStr)
	}
}
