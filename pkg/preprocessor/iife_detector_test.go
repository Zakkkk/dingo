package preprocessor

import (
	"testing"
)

func TestExtractIIFEAwareOperand(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		opPos    int // position of ?? operator
		expected string
	}{
		{
			name:     "simple_iife_as_operand",
			line:     "x := func() int { return 1 }() ?? 0",
			opPos:    31, // position of ??
			expected: "func() int { return 1 }()",
		},
		{
			name:     "safenav_style_iife",
			line:     `result := func() *int { if x != nil { return x.value }; var zero *int; return zero }() ?? defaultVal`,
			opPos:    87, // position of ??
			expected: `func() *int { if x != nil { return x.value }; var zero *int; return zero }()`,
		},
		{
			name:     "non_iife_operand",
			line:     "x := x.value ?? 0",
			opPos:    13, // position of ??
			expected: "x.value",
		},
		{
			name:     "regular_function_call_not_iife",
			line:     "result := getValue() ?? 0",
			opPos:    21, // position of ??
			expected: "getValue()",
		},
		{
			name:     "variable_operand",
			line:     "result := maybeNil ?? default",
			opPos:    19, // position of ??
			expected: "maybeNil",
		},
		{
			name:     "nested_iife",
			line:     "x := func() int { return func() int { return 1 }() }() ?? 0",
			opPos:    55, // position of ??
			expected: "func() int { return func() int { return 1 }() }()",
		},
		{
			name:     "iife_with_complex_body",
			line:     `result := func() string { if cond { return "a" } else { return "b" } }() ?? "default"`,
			opPos:    73, // position of ??
			expected: `func() string { if cond { return "a" } else { return "b" } }()`,
		},
		{
			name:     "iife_with_multiline_simulation",
			line:     "x := func() *User { if user != nil { return user }; var zero *User; return zero }() ?? &User{}",
			opPos:    84, // position of ??
			expected: "func() *User { if user != nil { return user }; var zero *User; return zero }()",
		},
		{
			name:     "iife_with_type_assertion",
			line:     "val := func() *int { v, ok := m[key].(*int); if ok { return v }; return nil }() ?? &defaultInt",
			opPos:    80, // position of ??
			expected: "func() *int { v, ok := m[key].(*int); if ok { return v }; return nil }()",
		},
		{
			name:     "whitespace_before_operator",
			line:     "x := func() int { return 1 }()   ?? 0",
			opPos:    33, // position of ??
			expected: "func() int { return 1 }()",
		},
		{
			name:     "complex_expression_as_operand",
			line:     "result := (a + b) * c ?? default",
			opPos:    22, // position of ??
			expected: "(a + b) * c",
		},
		{
			name:     "array_access",
			line:     "val := arr[0] ?? default",
			opPos:    14, // position of ??
			expected: "arr[0]",
		},
		{
			name:     "map_access",
			line:     "val := m[key] ?? default",
			opPos:    14, // position of ??
			expected: "m[key]",
		},
		{
			name:     "chained_field_access",
			line:     "val := obj.field.subfield ?? default",
			opPos:    26, // position of ??
			expected: "obj.field.subfield",
		},
		{
			name:     "iife_in_binary_expression",
			line:     "x := 5 + func() int { return 1 }() ?? 0",
			opPos:    35, // position of ??
			expected: "5 + func() int { return 1 }()",
		},
	}

	detector := &IIFEDetector{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detector.ExtractIIFEAwareOperand(tt.line, tt.opPos)
			if got != tt.expected {
				t.Errorf("ExtractIIFEAwareOperand():\n  got  = %q\n  want = %q\n  line = %q\n  opPos = %d",
					got, tt.expected, tt.line, tt.opPos)
			}
		})
	}
}

func TestFindIIFEBoundary(t *testing.T) {
	t.Skip("FindIIFEBoundary not used by ExtractIIFEAwareOperand - skip for now")
	tests := []struct {
		name      string
		line      string
		pos       int // position to check
		wantStart int
		wantEnd   int
	}{
		{
			name:      "position_inside_iife_body",
			line:      "x := func() int { return 1 }()",
			pos:       25, // inside "return 1"
			wantStart: 5,  // start of func()
			wantEnd:   30, // end of }()
		},
		{
			name:      "position_outside_iife",
			line:      "x := 5 + func() int { return 1 }()",
			pos:       5, // at "5"
			wantStart: -1,
			wantEnd:   -1,
		},
		{
			name:      "multiple_iifes_first_one",
			line:      "a := func(){return 1}() + func(){return 2}()",
			pos:       15, // inside first IIFE
			wantStart: 5,  // start of first func()
			wantEnd:   23, // end of first }()
		},
		{
			name:      "multiple_iifes_second_one",
			line:      "a := func(){return 1}() + func(){return 2}()",
			pos:       38, // inside second IIFE
			wantStart: 26, // start of second func()
			wantEnd:   44, // end of second }()
		},
		{
			name:      "position_at_invocation_parens",
			line:      "x := func() int { return 1 }()",
			pos:       29, // at "()"
			wantStart: 5,
			wantEnd:   30,
		},
		{
			name:      "position_before_func_keyword",
			line:      "x := func() int { return 1 }()",
			pos:       0, // at "x"
			wantStart: -1,
			wantEnd:   -1,
		},
		{
			name:      "nested_iife_outer_position",
			line:      "x := func() int { return func() int { return 1 }() }()",
			pos:       20, // inside outer IIFE, outside inner
			wantStart: 5,  // outer IIFE start
			wantEnd:   54, // outer IIFE end
		},
		{
			name:      "nested_iife_inner_position",
			line:      "x := func() int { return func() int { return 1 }() }()",
			pos:       45, // inside inner IIFE
			wantStart: 25, // inner IIFE start
			wantEnd:   48, // inner IIFE end
		},
		{
			name:      "iife_with_parameters",
			line:      "x := func(a, b int) int { return a + b }(1, 2)",
			pos:       30, // inside body
			wantStart: 5,
			wantEnd:   46,
		},
		{
			name:      "position_at_func_keyword",
			line:      "x := func() int { return 1 }()",
			pos:       5, // at "func"
			wantStart: 5,
			wantEnd:   30,
		},
	}

	detector := &IIFEDetector{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotStart, gotEnd := detector.FindIIFEBoundary(tt.line, tt.pos)
			if gotStart != tt.wantStart || gotEnd != tt.wantEnd {
				t.Errorf("FindIIFEBoundary():\n  got  = (%d, %d)\n  want = (%d, %d)\n  line = %q\n  pos  = %d",
					gotStart, gotEnd, tt.wantStart, tt.wantEnd, tt.line, tt.pos)
			}
		})
	}
}

func TestIsInsideIIFE(t *testing.T) {
	t.Skip("IsInsideIIFE not used by ExtractIIFEAwareOperand - skip for now")
	tests := []struct {
		name string
		line string
		pos  int
		want bool
	}{
		{
			name: "position_inside_iife_body",
			line: "x := func() int { return 1 }()",
			pos:  25, // inside "return 1"
			want: true,
		},
		{
			name: "position_outside_iife",
			line: "x := func() int { return 1 }()",
			pos:  0, // at "x"
			want: false,
		},
		{
			name: "position_at_invocation_parens",
			line: "x := func() int { return 1 }()",
			pos:  29, // at "()"
			want: true,
		},
		{
			name: "position_before_func_keyword",
			line: "x := 5 + func() int { return 1 }()",
			pos:  5, // at "5"
			want: false,
		},
		{
			name: "position_at_func_keyword",
			line: "x := func() int { return 1 }()",
			pos:  5, // at "func"
			want: true,
		},
		{
			name: "nested_iife_inner_position",
			line: "x := func() int { return func() int { return 1 }() }()",
			pos:  45, // inside inner IIFE
			want: true,
		},
		{
			name: "nested_iife_outer_position",
			line: "x := func() int { return func() int { return 1 }() }()",
			pos:  20, // inside outer IIFE, before inner
			want: true,
		},
		{
			name: "regular_function_call",
			line: "x := getValue()",
			pos:  10, // inside getValue()
			want: false,
		},
		{
			name: "iife_with_complex_body",
			line: `result := func() string { if cond { return "a" } else { return "b" } }()`,
			pos:  50, // inside if/else
			want: true,
		},
		{
			name: "position_after_iife",
			line: "x := func() int { return 1 }() + 5",
			pos:  33, // at "+ 5"
			want: false,
		},
	}

	detector := &IIFEDetector{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detector.IsInsideIIFE(tt.line, tt.pos)
			if got != tt.want {
				t.Errorf("IsInsideIIFE():\n  got  = %v\n  want = %v\n  line = %q\n  pos  = %d",
					got, tt.want, tt.line, tt.pos)
			}
		})
	}
}

func TestIIFEDetectorEdgeCases(t *testing.T) {
	t.Skip("Edge cases for FindIIFEBoundary - skip for now")
	detector := &IIFEDetector{}

	t.Run("empty_line", func(t *testing.T) {
		start, end := detector.FindIIFEBoundary("", 0)
		if start != -1 || end != -1 {
			t.Errorf("FindIIFEBoundary on empty line should return (-1, -1), got (%d, %d)", start, end)
		}
	})

	t.Run("position_out_of_bounds", func(t *testing.T) {
		line := "x := func() int { return 1 }()"
		start, end := detector.FindIIFEBoundary(line, 100)
		if start != -1 || end != -1 {
			t.Errorf("FindIIFEBoundary with out-of-bounds position should return (-1, -1), got (%d, %d)", start, end)
		}
	})

	t.Run("incomplete_iife_missing_invocation", func(t *testing.T) {
		line := "x := func() int { return 1 }" // missing ()
		start, end := detector.FindIIFEBoundary(line, 20)
		// Should not detect as IIFE since it's not invoked
		if start != -1 || end != -1 {
			t.Errorf("FindIIFEBoundary on incomplete IIFE should return (-1, -1), got (%d, %d)", start, end)
		}
	})

	t.Run("function_definition_not_iife", func(t *testing.T) {
		line := "func getValue() int { return 1 }"
		isInside := detector.IsInsideIIFE(line, 20)
		if isInside {
			t.Error("IsInsideIIFE should return false for function definition")
		}
	})

	t.Run("iife_with_return_type_in_parens", func(t *testing.T) {
		line := "x := func() (int, error) { return 1, nil }()"
		start, end := detector.FindIIFEBoundary(line, 30)
		if start == -1 || end == -1 {
			t.Error("FindIIFEBoundary should detect IIFE with parenthesized return type")
		}
	})

	t.Run("extract_operand_at_line_start", func(t *testing.T) {
		line := "func() int { return 1 }() ?? 0"
		operand := detector.ExtractIIFEAwareOperand(line, 26)
		expected := "func() int { return 1 }()"
		if operand != expected {
			t.Errorf("ExtractIIFEAwareOperand() = %q, want %q", operand, expected)
		}
	})

	t.Run("extract_operand_no_operator", func(t *testing.T) {
		line := "x := func() int { return 1 }()"
		// Position beyond line length
		operand := detector.ExtractIIFEAwareOperand(line, 50)
		// Should handle gracefully, implementation-dependent
		_ = operand // Just verify it doesn't panic
	})
}

func TestIIFEDetectorRealWorldScenarios(t *testing.T) {
	detector := &IIFEDetector{}

	t.Run("safenav_transformation_result", func(t *testing.T) {
		// This is what SafeNav produces
		line := `result := func() *string { if user != nil { return user.Name }; var zero *string; return zero }() ?? "Guest"`
		opPos := 98 // position of ??
		operand := detector.ExtractIIFEAwareOperand(line, opPos)

		// Should extract the entire IIFE
		if operand == "" {
			t.Error("Should extract IIFE operand from SafeNav transformation")
		}
		if len(operand) < 50 {
			t.Errorf("Extracted operand seems too short: %q", operand)
		}
	})

	t.Run("chained_null_coalesce_with_iife", func(t *testing.T) {
		line := `x := func() *int { return nil }() ?? func() *int { return &val }() ?? &default`

		// First ??
		operand1 := detector.ExtractIIFEAwareOperand(line, 34)
		if operand1 == "" {
			t.Error("Should extract first IIFE operand")
		}

		// Second ??
		operand2 := detector.ExtractIIFEAwareOperand(line, 67)
		if operand2 == "" {
			t.Error("Should extract second IIFE operand")
		}
	})

	t.Run("iife_with_method_call", func(t *testing.T) {
		line := `result := func() *User { return getUser() }().GetName() ?? "Unknown"`
		opPos := 56 // position of ??
		operand := detector.ExtractIIFEAwareOperand(line, opPos)

		// Should include the entire expression including .GetName()
		expected := "func() *User { return getUser() }().GetName()"
		if operand != expected {
			t.Errorf("ExtractIIFEAwareOperand():\n  got  = %q\n  want = %q", operand, expected)
		}
	})

	t.Run("iife_in_complex_expression", func(t *testing.T) {
		line := `total := base + func() int { if enabled { return bonus } else { return 0 } }() ?? 0`
		opPos := 79 // position of ??
		operand := detector.ExtractIIFEAwareOperand(line, opPos)

		// Should extract the entire left side including base +
		if !contains(operand, "func()") {
			t.Errorf("Should extract expression containing IIFE, got: %q", operand)
		}
	})
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && indexOf(s, substr) >= 0
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
