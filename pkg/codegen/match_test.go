package codegen

import (
	"strings"
	"testing"

	"github.com/MadAppGang/dingo/pkg/ast"
)

// TestMatchCodeGen_ConstructorPattern tests code generation for constructor patterns.
func TestMatchCodeGen_ConstructorPattern(t *testing.T) {
	tests := []struct {
		name         string
		match        *ast.MatchExpr
		expectedPart string // Part of the output we expect to see
	}{
		{
			name: "Simple Ok/Err match",
			match: &ast.MatchExpr{
				Match: 1,
				Scrutinee: &ast.RawExpr{
					StartPos: 7,
					EndPos:   13,
					Text:     "result",
				},
				Arms: []*ast.MatchArm{
					{
						Pattern: &ast.ConstructorPattern{
							Name: "Ok",
							Params: []ast.Pattern{
								&ast.VariablePattern{Name: "value"},
							},
						},
						Body: &ast.RawExpr{Text: "value"},
					},
					{
						Pattern: &ast.ConstructorPattern{
							Name: "Err",
							Params: []ast.Pattern{
								&ast.VariablePattern{Name: "e"},
							},
						},
						Body: &ast.RawExpr{Text: "0"},
					},
				},
				IsExpr: false,
			},
			expectedPart: "res.IsOk()", // NEW: method-based Result matching
		},
		{
			name: "Some/None match",
			match: &ast.MatchExpr{
				Match: 1,
				Scrutinee: &ast.RawExpr{
					Text: "opt",
				},
				Arms: []*ast.MatchArm{
					{
						Pattern: &ast.ConstructorPattern{
							Name: "Some",
							Params: []ast.Pattern{
								&ast.VariablePattern{Name: "x"},
							},
						},
						Body: &ast.RawExpr{Text: "x"},
					},
					{
						Pattern: &ast.ConstructorPattern{
							Name:   "None",
							Params: []ast.Pattern{},
						},
						Body: &ast.RawExpr{Text: "0"},
					},
				},
				IsExpr: false,
			},
			expectedPart: "opt.IsSome()", // NEW: method-based Option matching
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gen := NewMatchCodeGen(tt.match).(*MatchCodeGen)
			result := gen.Generate()

			code := string(result.Output)
			if !strings.Contains(code, tt.expectedPart) {
				t.Errorf("Generate() missing expected part %q\nGot: %s", tt.expectedPart, code)
			}

			// Verify value is cached (prevents double evaluation)
			// New Option/Result codegen uses opt/res as temp var
			hasCaching := strings.Contains(code, "val :=") ||
				strings.Contains(code, "opt :=") ||
				strings.Contains(code, "res :=")
			if !hasCaching {
				t.Errorf("Expected value caching, got: %s", code)
			}

			// Verify switch uses cached variable (not original expression)
			// Only applicable for enum type switches, not Option/Result method-based matches
			if strings.Contains(code, "switch v") && strings.Contains(code, ".(type)") {
				if !strings.Contains(code, "switch v1 := val.(type)") {
					t.Errorf("Expected switch to use cached val, got: %s", code)
				}
			}
		})
	}
}

// TestMatchCodeGen_LiteralPattern tests code generation for literal patterns.
func TestMatchCodeGen_LiteralPattern(t *testing.T) {
	match := &ast.MatchExpr{
		Match: 1,
		Scrutinee: &ast.RawExpr{
			Text: "status",
		},
		Arms: []*ast.MatchArm{
			{
				Pattern: &ast.LiteralPattern{
					Value: "1",
					Kind:  ast.IntLiteral,
				},
				Body: &ast.RawExpr{Text: "\"active\""},
			},
			{
				Pattern: &ast.LiteralPattern{
					Value: "2",
					Kind:  ast.IntLiteral,
				},
				Body: &ast.RawExpr{Text: "\"pending\""},
			},
			{
				Pattern: &ast.WildcardPattern{},
				Body:    &ast.RawExpr{Text: "\"unknown\""},
			},
		},
		IsExpr: false,
	}

	gen := NewMatchCodeGen(match).(*MatchCodeGen)
	result := gen.Generate()

	code := string(result.Output)

	// Check for value caching
	if !strings.Contains(code, "val := status") {
		t.Errorf("Expected value caching 'val := status', got: %s", code)
	}

	// Check for switch statement using cached variable
	if !strings.Contains(code, "switch val") {
		t.Errorf("Expected switch statement using cached val, got: %s", code)
	}

	// Check for case clauses
	if !strings.Contains(code, "case 1:") {
		t.Errorf("Expected 'case 1:', got: %s", code)
	}
	if !strings.Contains(code, "case 2:") {
		t.Errorf("Expected 'case 2:', got: %s", code)
	}
	if !strings.Contains(code, "default:") {
		t.Errorf("Expected 'default:', got: %s", code)
	}
}

// TestMatchCodeGen_WildcardPattern tests wildcard pattern generation.
func TestMatchCodeGen_WildcardPattern(t *testing.T) {
	match := &ast.MatchExpr{
		Match: 1,
		Scrutinee: &ast.RawExpr{
			Text: "value",
		},
		Arms: []*ast.MatchArm{
			{
				Pattern: &ast.WildcardPattern{
					Pos_: 10,
				},
				Body: &ast.RawExpr{Text: "42"},
			},
		},
		IsExpr: false,
	}

	gen := NewMatchCodeGen(match).(*MatchCodeGen)
	result := gen.Generate()

	code := string(result.Output)

	// Wildcard should generate default case
	if !strings.Contains(code, "default:") {
		t.Errorf("Expected 'default:' for wildcard pattern, got: %s", code)
	}

	// Should contain body
	if !strings.Contains(code, "42") {
		t.Errorf("Expected body '42', got: %s", code)
	}
}

// TestMatchCodeGen_VariablePattern tests variable pattern with binding.
func TestMatchCodeGen_VariablePattern(t *testing.T) {
	match := &ast.MatchExpr{
		Match: 1,
		Scrutinee: &ast.RawExpr{
			Text: "value",
		},
		Arms: []*ast.MatchArm{
			{
				Pattern: &ast.VariablePattern{
					Name: "x",
				},
				Body: &ast.RawExpr{Text: "x * 2"},
			},
		},
		IsExpr: false,
	}

	gen := NewMatchCodeGen(match).(*MatchCodeGen)
	result := gen.Generate()

	code := string(result.Output)

	// Variable pattern should generate default case
	if !strings.Contains(code, "default:") {
		t.Errorf("Expected 'default:' for variable pattern, got: %s", code)
	}

	// Should bind variable
	if !strings.Contains(code, "x :=") {
		t.Errorf("Expected variable binding 'x :=', got: %s", code)
	}
}

// TestMatchCodeGen_GuardCondition tests guard condition generation.
func TestMatchCodeGen_GuardCondition(t *testing.T) {
	match := &ast.MatchExpr{
		Match: 1,
		Scrutinee: &ast.RawExpr{
			Text: "result",
		},
		Arms: []*ast.MatchArm{
			{
				Pattern: &ast.ConstructorPattern{
					Name: "Ok",
					Params: []ast.Pattern{
						&ast.VariablePattern{Name: "x"},
					},
				},
				Guard:    &ast.RawExpr{Text: "x > 0"},
				GuardPos: 20,
				Body:     &ast.RawExpr{Text: "x * 2"},
			},
			{
				Pattern: &ast.WildcardPattern{},
				Body:    &ast.RawExpr{Text: "0"},
			},
		},
		IsExpr: false,
	}

	gen := NewMatchCodeGen(match).(*MatchCodeGen)
	result := gen.Generate()

	code := string(result.Output)

	// Should contain guard condition
	if !strings.Contains(code, "if x > 0") {
		t.Errorf("Expected guard condition 'if x > 0', got: %s", code)
	}

	// Should have nested body in guard
	if !strings.Contains(code, "x * 2") {
		t.Errorf("Expected body 'x * 2', got: %s", code)
	}
}

// TestMatchCodeGen_MatchExpression tests IIFE generation for match expressions.
func TestMatchCodeGen_MatchExpression(t *testing.T) {
	match := &ast.MatchExpr{
		Match: 1,
		Scrutinee: &ast.RawExpr{
			Text: "result",
		},
		Arms: []*ast.MatchArm{
			{
				Pattern: &ast.ConstructorPattern{
					Name: "Ok",
					Params: []ast.Pattern{
						&ast.VariablePattern{Name: "value"},
					},
				},
				Body: &ast.RawExpr{Text: "value"},
			},
			{
				Pattern: &ast.ConstructorPattern{
					Name: "Err",
					Params: []ast.Pattern{
						&ast.VariablePattern{Name: "e"},
					},
				},
				Body: &ast.RawExpr{Text: "0"},
			},
		},
		IsExpr: true, // This is an expression
	}

	gen := NewMatchCodeGen(match).(*MatchCodeGen)
	result := gen.Generate()

	code := string(result.Output)

	// Should wrap in IIFE
	if !strings.Contains(code, "func()") {
		t.Errorf("Expected IIFE wrapper 'func()', got: %s", code)
	}

	// Should have immediate invocation
	if !strings.Contains(code, "}()") {
		t.Errorf("Expected IIFE invocation '}()', got: %s", code)
	}

	// Should have return statements
	if !strings.Contains(code, "return") {
		t.Errorf("Expected return statements in IIFE, got: %s", code)
	}
}

// TestMatchCodeGen_EmptyMatch tests handling of empty match expression.
func TestMatchCodeGen_EmptyMatch(t *testing.T) {
	gen := NewMatchCodeGen(nil).(*MatchCodeGen)
	result := gen.Generate()

	if len(result.Output) > 0 {
		t.Errorf("Expected empty result for nil match, got: %s", string(result.Output))
	}
}

// TestMatchCodeGen_NoConstructorPatterns tests value switch for non-constructor patterns.
func TestMatchCodeGen_NoConstructorPatterns(t *testing.T) {
	match := &ast.MatchExpr{
		Match: 1,
		Scrutinee: &ast.RawExpr{
			Text: "status",
		},
		Arms: []*ast.MatchArm{
			{
				Pattern: &ast.LiteralPattern{
					Value: "\"active\"",
					Kind:  ast.StringLiteral,
				},
				Body: &ast.RawExpr{Text: "1"},
			},
			{
				Pattern: &ast.WildcardPattern{},
				Body:    &ast.RawExpr{Text: "0"},
			},
		},
		IsExpr: false,
	}

	gen := NewMatchCodeGen(match).(*MatchCodeGen)
	result := gen.Generate()

	code := string(result.Output)

	// Should use value switch, not type switch
	if strings.Contains(code, ".(type)") {
		t.Errorf("Expected value switch (no type assertion), got: %s", code)
	}

	// Should have value caching
	if !strings.Contains(code, "val := status") {
		t.Errorf("Expected value caching, got: %s", code)
	}

	// Should have switch statement using cached variable
	if !strings.Contains(code, "switch val") {
		t.Errorf("Expected switch using cached val, got: %s", code)
	}
}

// =============================================================================
// Value Enum Match Tests (Phase 3)
// =============================================================================

// TestMatchCodeGen_ValueEnumDetection tests detection of value enum patterns.
func TestMatchCodeGen_ValueEnumDetection(t *testing.T) {
	// Create a value enum registry
	registry := ast.NewEnumRegistry()
	registry.RegisterValueEnum("Status", []string{"Pending", "Active", "Closed"}, true)

	match := &ast.MatchExpr{
		Match: 1,
		Scrutinee: &ast.RawExpr{
			Text: "status",
		},
		Arms: []*ast.MatchArm{
			{
				Pattern: &ast.ConstructorPattern{Name: "Pending"},
				Body:    &ast.RawExpr{Text: `"waiting"`},
			},
			{
				Pattern: &ast.ConstructorPattern{Name: "Active"},
				Body:    &ast.RawExpr{Text: `"running"`},
			},
			{
				Pattern: &ast.ConstructorPattern{Name: "Closed"},
				Body:    &ast.RawExpr{Text: `"done"`},
			},
		},
		IsExpr: false,
	}

	gen := &MatchCodeGen{
		BaseGenerator: NewBaseGenerator(),
		Match:         match,
		ValueEnumReg:  registry,
	}

	// Test detection
	info := gen.detectValueEnum()
	if info == nil {
		t.Fatal("Expected value enum to be detected")
	}

	if info.EnumName != "Status" {
		t.Errorf("EnumName = %q, want %q", info.EnumName, "Status")
	}

	if !info.UsePrefix {
		t.Error("UsePrefix = false, want true (default)")
	}

	if len(info.Variants) != 3 {
		t.Errorf("Variants count = %d, want 3", len(info.Variants))
	}
}

// TestMatchCodeGen_ValueEnumCodeGen tests code generation for value enum match.
func TestMatchCodeGen_ValueEnumCodeGen(t *testing.T) {
	// Create a value enum registry
	registry := ast.NewEnumRegistry()
	registry.RegisterValueEnum("Status", []string{"Pending", "Active", "Closed"}, true)

	match := &ast.MatchExpr{
		Match: 1,
		Scrutinee: &ast.RawExpr{
			Text: "status",
		},
		Arms: []*ast.MatchArm{
			{
				Pattern: &ast.ConstructorPattern{Name: "Pending"},
				Body:    &ast.RawExpr{Text: `"waiting"`},
			},
			{
				Pattern: &ast.ConstructorPattern{Name: "Active"},
				Body:    &ast.RawExpr{Text: `"running"`},
			},
			{
				Pattern: &ast.ConstructorPattern{Name: "Closed"},
				Body:    &ast.RawExpr{Text: `"done"`},
			},
		},
		IsExpr: false,
	}

	gen := &MatchCodeGen{
		BaseGenerator: NewBaseGenerator(),
		Match:         match,
		ValueEnumReg:  registry,
	}

	result := gen.Generate()
	code := string(result.Output)

	// Should use value switch (NOT type switch)
	if strings.Contains(code, ".(type)") {
		t.Errorf("Value enum should use value switch, not type switch. Got: %s", code)
	}

	// Should have prefixed const names
	if !strings.Contains(code, "case StatusPending:") {
		t.Errorf("Expected 'case StatusPending:', got: %s", code)
	}
	if !strings.Contains(code, "case StatusActive:") {
		t.Errorf("Expected 'case StatusActive:', got: %s", code)
	}
	if !strings.Contains(code, "case StatusClosed:") {
		t.Errorf("Expected 'case StatusClosed:', got: %s", code)
	}

	// Should have body expressions
	if !strings.Contains(code, `"waiting"`) {
		t.Errorf("Expected body \"waiting\", got: %s", code)
	}
}

// TestMatchCodeGen_ValueEnumNoPrefixCodeGen tests code generation with @prefix(false).
func TestMatchCodeGen_ValueEnumNoPrefixCodeGen(t *testing.T) {
	// Create a value enum registry with no prefix
	registry := ast.NewEnumRegistry()
	registry.RegisterValueEnum("Flags", []string{"Read", "Write"}, false) // UsePrefix = false

	match := &ast.MatchExpr{
		Match: 1,
		Scrutinee: &ast.RawExpr{
			Text: "flag",
		},
		Arms: []*ast.MatchArm{
			{
				Pattern: &ast.ConstructorPattern{Name: "Read"},
				Body:    &ast.RawExpr{Text: "1"},
			},
			{
				Pattern: &ast.ConstructorPattern{Name: "Write"},
				Body:    &ast.RawExpr{Text: "2"},
			},
		},
		IsExpr: false,
	}

	gen := &MatchCodeGen{
		BaseGenerator: NewBaseGenerator(),
		Match:         match,
		ValueEnumReg:  registry,
	}

	result := gen.Generate()
	code := string(result.Output)

	// Should have unprefixed const names (since @prefix(false))
	if !strings.Contains(code, "case Read:") {
		t.Errorf("Expected unprefixed 'case Read:', got: %s", code)
	}
	if !strings.Contains(code, "case Write:") {
		t.Errorf("Expected unprefixed 'case Write:', got: %s", code)
	}

	// Should NOT have prefixed names
	if strings.Contains(code, "FlagsRead") {
		t.Errorf("Should not have prefixed 'FlagsRead', got: %s", code)
	}
}

// TestMatchCodeGen_ValueEnumExhaustiveness tests exhaustiveness checking.
func TestMatchCodeGen_ValueEnumExhaustiveness(t *testing.T) {
	tests := []struct {
		name        string
		arms        []*ast.MatchArm
		expectError bool
	}{
		{
			name: "exhaustive - all variants",
			arms: []*ast.MatchArm{
				{Pattern: &ast.ConstructorPattern{Name: "Pending"}, Body: &ast.RawExpr{Text: "1"}},
				{Pattern: &ast.ConstructorPattern{Name: "Active"}, Body: &ast.RawExpr{Text: "2"}},
				{Pattern: &ast.ConstructorPattern{Name: "Closed"}, Body: &ast.RawExpr{Text: "3"}},
			},
			expectError: false,
		},
		{
			name: "exhaustive - with wildcard",
			arms: []*ast.MatchArm{
				{Pattern: &ast.ConstructorPattern{Name: "Pending"}, Body: &ast.RawExpr{Text: "1"}},
				{Pattern: &ast.WildcardPattern{}, Body: &ast.RawExpr{Text: "0"}},
			},
			expectError: false,
		},
		{
			name: "non-exhaustive - missing variant",
			arms: []*ast.MatchArm{
				{Pattern: &ast.ConstructorPattern{Name: "Pending"}, Body: &ast.RawExpr{Text: "1"}},
				{Pattern: &ast.ConstructorPattern{Name: "Active"}, Body: &ast.RawExpr{Text: "2"}},
				// Missing: Closed
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create registry
			registry := ast.NewEnumRegistry()
			registry.RegisterValueEnum("Status", []string{"Pending", "Active", "Closed"}, true)

			match := &ast.MatchExpr{
				Match:     1,
				Scrutinee: &ast.RawExpr{Text: "status"},
				Arms:      tt.arms,
				IsExpr:    true, // Expression match requires exhaustiveness
			}

			gen := &MatchCodeGen{
				BaseGenerator: NewBaseGenerator(),
				Match:         match,
				ValueEnumReg:  registry,
			}

			result := gen.Generate()

			if tt.expectError {
				if result.Error == nil {
					t.Error("Expected exhaustiveness error, got none")
				} else if !strings.Contains(result.Error.Message, "non-exhaustive") {
					t.Errorf("Expected 'non-exhaustive' in error, got: %s", result.Error.Message)
				}
			} else {
				if result.Error != nil {
					t.Errorf("Expected no error, got: %s", result.Error.Message)
				}
			}
		})
	}
}

// TestMatchCodeGen_ValueEnumWithGuard tests value enum match with guard conditions.
func TestMatchCodeGen_ValueEnumWithGuard(t *testing.T) {
	registry := ast.NewEnumRegistry()
	registry.RegisterValueEnum("Status", []string{"Pending", "Active", "Closed"}, true)

	match := &ast.MatchExpr{
		Match: 1,
		Scrutinee: &ast.RawExpr{
			Text: "status",
		},
		Arms: []*ast.MatchArm{
			{
				Pattern: &ast.ConstructorPattern{Name: "Pending"},
				Guard:   &ast.RawExpr{Text: "urgent"},
				Body:    &ast.RawExpr{Text: `"urgent pending"`},
			},
			{
				Pattern: &ast.WildcardPattern{},
				Body:    &ast.RawExpr{Text: `"other"`},
			},
		},
		IsExpr: false,
	}

	gen := &MatchCodeGen{
		BaseGenerator: NewBaseGenerator(),
		Match:         match,
		ValueEnumReg:  registry,
	}

	result := gen.Generate()
	code := string(result.Output)

	// Should have guard condition
	if !strings.Contains(code, "if urgent") {
		t.Errorf("Expected guard condition 'if urgent', got: %s", code)
	}

	// Should have case for Pending
	if !strings.Contains(code, "case StatusPending:") {
		t.Errorf("Expected 'case StatusPending:', got: %s", code)
	}
}

// TestMatchCodeGen_NotValueEnum tests that sum type enums are NOT detected as value enums.
func TestMatchCodeGen_NotValueEnum(t *testing.T) {
	registry := ast.NewEnumRegistry()
	registry.RegisterValueEnum("Status", []string{"Pending", "Active"}, true)

	// Match with constructor pattern that has params = sum type, not value enum
	match := &ast.MatchExpr{
		Match:     1,
		Scrutinee: &ast.RawExpr{Text: "result"},
		Arms: []*ast.MatchArm{
			{
				Pattern: &ast.ConstructorPattern{
					Name:   "Ok",
					Params: []ast.Pattern{&ast.VariablePattern{Name: "x"}}, // Has params!
				},
				Body: &ast.RawExpr{Text: "x"},
			},
			{
				Pattern: &ast.ConstructorPattern{
					Name:   "Err",
					Params: []ast.Pattern{&ast.VariablePattern{Name: "e"}},
				},
				Body: &ast.RawExpr{Text: "0"},
			},
		},
		IsExpr: false,
	}

	gen := &MatchCodeGen{
		BaseGenerator: NewBaseGenerator(),
		Match:         match,
		ValueEnumReg:  registry,
	}

	// Should NOT detect as value enum (has params)
	info := gen.detectValueEnum()
	if info != nil {
		t.Error("Should NOT detect sum type (with params) as value enum")
	}
}

// TestMatchCodeGen_MixedPatterns tests that mixed patterns don't match value enum.
func TestMatchCodeGen_MixedPatterns(t *testing.T) {
	registry := ast.NewEnumRegistry()
	registry.RegisterValueEnum("Status", []string{"Pending", "Active"}, true)
	registry.RegisterValueEnum("Flags", []string{"Read", "Write"}, false)

	// Match mixing variants from different enums
	match := &ast.MatchExpr{
		Match:     1,
		Scrutinee: &ast.RawExpr{Text: "val"},
		Arms: []*ast.MatchArm{
			{
				Pattern: &ast.ConstructorPattern{Name: "Pending"}, // From Status
				Body:    &ast.RawExpr{Text: "1"},
			},
			{
				Pattern: &ast.ConstructorPattern{Name: "Read"}, // From Flags!
				Body:    &ast.RawExpr{Text: "2"},
			},
		},
		IsExpr: false,
	}

	gen := &MatchCodeGen{
		BaseGenerator: NewBaseGenerator(),
		Match:         match,
		ValueEnumReg:  registry,
	}

	// Should NOT detect as value enum (mixed enums)
	info := gen.detectValueEnum()
	if info != nil {
		t.Error("Should NOT detect mixed enum patterns as value enum")
	}
}
