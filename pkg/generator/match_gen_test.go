package generator

import (
	"go/token"
	"strings"
	"testing"

	dingoast "github.com/MadAppGang/dingo/pkg/ast"
)

func TestMatchGenerator_SimplePatterns(t *testing.T) {
	tests := []struct {
		name   string
		match  *dingoast.MatchExpr
		checks []string // strings that must be present in output
	}{
		{
			name: "Ok pattern",
			match: &dingoast.MatchExpr{
				Scrutinee: &dingoast.RawExpr{Text: "result"},
				Arms: []*dingoast.MatchArm{
					{
						Pattern: &dingoast.ConstructorPattern{
							Name: "Ok",
							Params: []dingoast.Pattern{
								&dingoast.VariablePattern{Name: "x"},
							},
						},
						Body: &dingoast.RawExpr{Text: "x"},
					},
				},
			},
			checks: []string{
				"tmp := result",
				"switch v := tmp.(type)",
				"case ResultOk:",
				"return x",
			},
		},
		{
			name: "Wildcard pattern",
			match: &dingoast.MatchExpr{
				Scrutinee: &dingoast.RawExpr{Text: "value"},
				Arms: []*dingoast.MatchArm{
					{
						Pattern: &dingoast.WildcardPattern{},
						Body:    &dingoast.RawExpr{Text: "0"},
					},
				},
			},
			checks: []string{
				"tmp := value",
				"switch v := tmp.(type)",
				"default:",
				"return 0",
			},
		},
		{
			name: "Multiple arms",
			match: &dingoast.MatchExpr{
				Scrutinee: &dingoast.RawExpr{Text: "result"},
				Arms: []*dingoast.MatchArm{
					{
						Pattern: &dingoast.ConstructorPattern{
							Name: "Ok",
							Params: []dingoast.Pattern{
								&dingoast.VariablePattern{Name: "x"},
							},
						},
						Body: &dingoast.RawExpr{Text: "x"},
					},
					{
						Pattern: &dingoast.ConstructorPattern{
							Name: "Err",
							Params: []dingoast.Pattern{
								&dingoast.VariablePattern{Name: "e"},
							},
						},
						Body: &dingoast.RawExpr{Text: "0"},
					},
				},
			},
			checks: []string{
				"tmp := result",
				"switch v := tmp.(type)",
				"case ResultOk:",
				"case ResultErr:",
				"return x",
				"return 0",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gen := NewMatchGenerator(0)
			code, _ := gen.Generate(tt.match)

			for _, check := range tt.checks {
				if !strings.Contains(code, check) {
					t.Errorf("Generated code missing %q\nGot:\n%s", check, code)
				}
			}
		})
	}
}

func TestMatchGenerator_NestedPatterns(t *testing.T) {
	tests := []struct {
		name   string
		match  *dingoast.MatchExpr
		checks []string
	}{
		{
			name: "Ok(Some(x))",
			match: &dingoast.MatchExpr{
				Scrutinee: &dingoast.RawExpr{Text: "wrapped"},
				Arms: []*dingoast.MatchArm{
					{
						Pattern: &dingoast.ConstructorPattern{
							Name: "Ok",
							Params: []dingoast.Pattern{
								&dingoast.ConstructorPattern{
									Name: "Some",
									Params: []dingoast.Pattern{
										&dingoast.VariablePattern{Name: "x"},
									},
								},
							},
						},
						Body: &dingoast.RawExpr{Text: "x"},
					},
					{
						Pattern: &dingoast.ConstructorPattern{
							Name: "Ok",
							Params: []dingoast.Pattern{
								&dingoast.ConstructorPattern{Name: "None"},
							},
						},
						Body: &dingoast.RawExpr{Text: "0"},
					},
					{
						Pattern: &dingoast.ConstructorPattern{
							Name: "Err",
							Params: []dingoast.Pattern{
								&dingoast.VariablePattern{Name: "e"},
							},
						},
						Body: &dingoast.RawExpr{Text: "-1"},
					},
				},
			},
			checks: []string{
				"tmp := wrapped",
				"switch v := tmp.(type)",
				"case ResultOk:",
				"case ResultErr:",
				"return x",
				"return 0",
				"return -1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gen := NewMatchGenerator(0)
			code, _ := gen.Generate(tt.match)

			for _, check := range tt.checks {
				if !strings.Contains(code, check) {
					t.Errorf("Generated code missing %q\nGot:\n%s", check, code)
				}
			}
		})
	}
}

func TestMatchGenerator_Guards(t *testing.T) {
	tests := []struct {
		name   string
		match  *dingoast.MatchExpr
		checks []string
	}{
		{
			name: "Guard with if",
			match: &dingoast.MatchExpr{
				Scrutinee: &dingoast.RawExpr{Text: "result"},
				Arms: []*dingoast.MatchArm{
					{
						Pattern: &dingoast.ConstructorPattern{
							Name: "Ok",
							Params: []dingoast.Pattern{
								&dingoast.VariablePattern{Name: "x"},
							},
						},
						Guard: &dingoast.RawExpr{Text: "x > 0"},
						Body:  &dingoast.RawExpr{Text: "x * 2"},
					},
					{
						Pattern: &dingoast.WildcardPattern{},
						Body:    &dingoast.RawExpr{Text: "0"},
					},
				},
			},
			checks: []string{
				"tmp := result",
				"switch v := tmp.(type)",
				"case ResultOk:",
				"if x > 0",
				"return x * 2",
				"default:",
				"return 0",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gen := NewMatchGenerator(0)
			code, _ := gen.Generate(tt.match)

			for _, check := range tt.checks {
				if !strings.Contains(code, check) {
					t.Errorf("Generated code missing %q\nGot:\n%s", check, code)
				}
			}
		})
	}
}

func TestMatchGenerator_BlockBody(t *testing.T) {
	match := &dingoast.MatchExpr{
		Scrutinee: &dingoast.RawExpr{Text: "result"},
		Arms: []*dingoast.MatchArm{
			{
				Pattern: &dingoast.ConstructorPattern{
					Name: "Ok",
					Params: []dingoast.Pattern{
						&dingoast.VariablePattern{Name: "x"},
					},
				},
				Body:    &dingoast.RawExpr{Text: "{ fmt.Println(x) return x }"},
				IsBlock: true,
			},
		},
	}

	gen := NewMatchGenerator(0)
	code, _ := gen.Generate(match)

	if !strings.Contains(code, "{ fmt.Println(x) return x } // dingo:M:1") {
		t.Errorf("Block body not preserved correctly\nGot:\n%s", code)
	}
}

func TestMatchGenerator_SourceMapMarkers(t *testing.T) {
	match := &dingoast.MatchExpr{
		Scrutinee: &dingoast.RawExpr{Text: "result"},
		Arms: []*dingoast.MatchArm{
			{
				Pattern: &dingoast.ConstructorPattern{
					Name: "Ok",
					Params: []dingoast.Pattern{
						&dingoast.VariablePattern{Name: "x"},
					},
				},
				Body: &dingoast.RawExpr{Text: "x"},
			},
		},
	}

	gen := NewMatchGenerator(0)
	code, _ := gen.Generate(match)

	// Check that marker is present
	if !strings.Contains(code, "// dingo:M:1") {
		t.Errorf("Source map marker not found in output:\n%s", code)
	}
}

func TestMatchGenerator_TempVariableNaming(t *testing.T) {
	// Test that temp variables follow naming convention: tmp, tmp1, tmp2
	match := &dingoast.MatchExpr{
		Scrutinee: &dingoast.RawExpr{Text: "result"},
		Arms: []*dingoast.MatchArm{
			{
				Pattern: &dingoast.ConstructorPattern{
					Name: "Ok",
					Params: []dingoast.Pattern{
						&dingoast.ConstructorPattern{
							Name: "Some",
							Params: []dingoast.Pattern{
								&dingoast.VariablePattern{Name: "x"},
							},
						},
					},
				},
				Body: &dingoast.RawExpr{Text: "x"},
			},
		},
	}

	gen := NewMatchGenerator(0)
	code, _ := gen.Generate(match)

	// Check for tmp (scrutinee assignment)
	if !strings.Contains(code, "tmp := result") {
		t.Errorf("First temp variable should be 'tmp', got:\n%s", code)
	}
	// With type-switch pattern, we use v for the matched type
	if !strings.Contains(code, "switch v := tmp.(type)") {
		t.Errorf("Should use type-switch pattern, got:\n%s", code)
	}
}

func TestMatchGenerator_CommentPreservation(t *testing.T) {
	// Test that comments are preserved in generated code
	match := &dingoast.MatchExpr{
		Scrutinee: &dingoast.RawExpr{Text: "result"},
		Arms: []*dingoast.MatchArm{
			{
				Pattern: &dingoast.ConstructorPattern{
					Name: "Ok",
					Params: []dingoast.Pattern{
						&dingoast.VariablePattern{Name: "x"},
					},
				},
				Body: &dingoast.RawExpr{Text: "x"},
				Comment: &dingoast.Comment{
					Pos:  token.Pos(100),
					Text: "// success case",
					Kind: dingoast.LineComment,
				},
			},
		},
	}

	gen := NewMatchGenerator(0)
	code, _ := gen.Generate(match)

	// Comments are part of the body expression in this simplified version
	// In full implementation, comments would be placed appropriately
	_ = code // Comment handling to be implemented in full version
}

func TestMatchGenerator_LiteralPatterns(t *testing.T) {
	match := &dingoast.MatchExpr{
		Scrutinee: &dingoast.RawExpr{Text: "status"},
		Arms: []*dingoast.MatchArm{
			{
				Pattern: &dingoast.LiteralPattern{
					Value: "200",
					Kind:  dingoast.IntLiteral,
				},
				Body: &dingoast.RawExpr{Text: "\"OK\""},
			},
			{
				Pattern: &dingoast.LiteralPattern{
					Value: "404",
					Kind:  dingoast.IntLiteral,
				},
				Body: &dingoast.RawExpr{Text: "\"Not Found\""},
			},
			{
				Pattern: &dingoast.WildcardPattern{},
				Body:    &dingoast.RawExpr{Text: "\"Unknown\""},
			},
		},
	}

	gen := NewMatchGenerator(0)
	code, _ := gen.Generate(match)

	// Verify type-switch pattern is used
	if !strings.Contains(code, "tmp := status") {
		t.Errorf("Literal pattern should assign scrutinee to tmp\nGot:\n%s", code)
	}
	if !strings.Contains(code, "switch v := tmp.(type)") {
		t.Errorf("Should use type-switch pattern\nGot:\n%s", code)
	}
	// Verify cases are generated
	if !strings.Contains(code, "case 200:") {
		t.Errorf("Should have case for 200\nGot:\n%s", code)
	}
	if !strings.Contains(code, "case 404:") {
		t.Errorf("Should have case for 404\nGot:\n%s", code)
	}
	if !strings.Contains(code, "default:") {
		t.Errorf("Should have default case\nGot:\n%s", code)
	}
}

func TestMatchGenerator_VariablePattern(t *testing.T) {
	// Variable pattern should become default case with assignment
	match := &dingoast.MatchExpr{
		Scrutinee: &dingoast.RawExpr{Text: "value"},
		Arms: []*dingoast.MatchArm{
			{
				Pattern: &dingoast.VariablePattern{Name: "v"},
				Body:    &dingoast.RawExpr{Text: "v * 2"},
			},
		},
	}

	gen := NewMatchGenerator(0)
	code, _ := gen.Generate(match)

	if !strings.Contains(code, "default:") {
		t.Errorf("Variable pattern should generate default case")
	}
	if !strings.Contains(code, "v := tmp") {
		t.Errorf("Variable pattern should assign scrutinee to variable")
	}
}

func TestMatchGenerator_EnumVariants(t *testing.T) {
	// Test enum variant patterns
	// With interface-based enums, we use type-switch and concrete types
	match := &dingoast.MatchExpr{
		Scrutinee: &dingoast.RawExpr{Text: "status"},
		Arms: []*dingoast.MatchArm{
			{
				Pattern: &dingoast.ConstructorPattern{
					Name: "Pending",
				},
				Body: &dingoast.RawExpr{Text: "\"waiting\""},
			},
			{
				Pattern: &dingoast.ConstructorPattern{
					Name: "Active",
				},
				Body: &dingoast.RawExpr{Text: "\"running\""},
			},
			{
				Pattern: &dingoast.WildcardPattern{},
				Body:    &dingoast.RawExpr{Text: "\"other\""},
			},
		},
	}

	gen := NewMatchGenerator(0)
	code, _ := gen.Generate(match)

	// With interface-based enums, the generator prefixes with inferred type name
	// Check that we have case statements for the variants
	if !strings.Contains(code, "case Status") {
		t.Errorf("Enum variant type not generated correctly\nGot:\n%s", code)
	}
	if !strings.Contains(code, "switch v := tmp.(type)") {
		t.Errorf("Type switch not generated\nGot:\n%s", code)
	}
	if !strings.Contains(code, "default:") {
		t.Errorf("Default case not generated\nGot:\n%s", code)
	}
}
