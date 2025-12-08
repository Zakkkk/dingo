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

// TestMatchCodeGen_SourceMappings tests that source mappings are generated.
func TestMatchCodeGen_SourceMappings(t *testing.T) {
	match := &ast.MatchExpr{
		Match: 1,
		Scrutinee: &ast.RawExpr{
			StartPos: 7,
			EndPos:   13,
			Text:     "result",
		},
		Arms: []*ast.MatchArm{
			{
				PatternPos: 15,
				Pattern: &ast.ConstructorPattern{
					Name: "Ok",
					Params: []ast.Pattern{
						&ast.VariablePattern{Name: "value"},
					},
				},
				Body: &ast.RawExpr{
					StartPos: 25,
					EndPos:   30,
					Text:     "value",
				},
			},
		},
		IsExpr: false,
	}

	gen := NewMatchCodeGen(match).(*MatchCodeGen)
	result := gen.Generate()

	// Should have mappings
	if len(result.Mappings) == 0 {
		t.Error("Expected source mappings, got none")
	}

	// All mappings should be "match" kind
	for _, mapping := range result.Mappings {
		if mapping.Kind != "match" {
			t.Errorf("Expected mapping kind 'match', got %q", mapping.Kind)
		}
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
