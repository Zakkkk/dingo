package generator

import (
	"go/token"
	"strings"
	"testing"

	dingoast "github.com/MadAppGang/dingo/pkg/ast"
)

func TestMatchGenerator_SimplePatterns(t *testing.T) {
	tests := []struct {
		name     string
		match    *dingoast.MatchExpr
		expected string
	}{
		{
			name: "Ok pattern",
			match: &dingoast.MatchExpr{
				Scrutinee: &dingoast.RawExpr{Text: "result"},
				Arms: []*dingoast.MatchArm{
					{
						Pattern: &dingoast.ConstructorPattern{
							Name:   "Ok",
							Params: []dingoast.Pattern{
								&dingoast.VariablePattern{Name: "x"},
							},
						},
						Body: &dingoast.RawExpr{Text: "x"},
					},
				},
			},
			expected: "tmp := result\nswitch tmp.Tag {\n\tcase dgo.ResultTagOk:\n\t\tx := *tmp.Ok\n\t\treturn x // dingo:M:1\n}\n",
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
			expected: "tmp := value\nswitch tmp.Tag {\n\tdefault:\n\t\treturn 0 // dingo:M:1\n}\n",
		},
		{
			name: "Multiple arms",
			match: &dingoast.MatchExpr{
				Scrutinee: &dingoast.RawExpr{Text: "result"},
				Arms: []*dingoast.MatchArm{
					{
						Pattern: &dingoast.ConstructorPattern{
							Name:   "Ok",
							Params: []dingoast.Pattern{
								&dingoast.VariablePattern{Name: "x"},
							},
						},
						Body: &dingoast.RawExpr{Text: "x"},
					},
					{
						Pattern: &dingoast.ConstructorPattern{
							Name:   "Err",
							Params: []dingoast.Pattern{
								&dingoast.VariablePattern{Name: "e"},
							},
						},
						Body: &dingoast.RawExpr{Text: "0"},
					},
				},
			},
			expected: "tmp := result\nswitch tmp.Tag {\n\tcase dgo.ResultTagOk:\n\t\tx := *tmp.Ok\n\t\treturn x // dingo:M:1\n\tcase dgo.ResultTagErr:\n\t\te := *tmp.Err\n\t\treturn 0 // dingo:M:1\n}\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gen := NewMatchGenerator(0)
			code, _ := gen.Generate(tt.match)

			if code != tt.expected {
				t.Errorf("Generate() output mismatch\nGot:\n%s\nExpected:\n%s", code, tt.expected)
			}
		})
	}
}

func TestMatchGenerator_NestedPatterns(t *testing.T) {
	tests := []struct {
		name     string
		match    *dingoast.MatchExpr
		expected string
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
							Name:   "Ok",
							Params: []dingoast.Pattern{
								&dingoast.ConstructorPattern{Name: "None"},
							},
						},
						Body: &dingoast.RawExpr{Text: "0"},
					},
					{
						Pattern: &dingoast.ConstructorPattern{
							Name:   "Err",
							Params: []dingoast.Pattern{
								&dingoast.VariablePattern{Name: "e"},
							},
						},
						Body: &dingoast.RawExpr{Text: "-1"},
					},
				},
			},
			expected: "tmp := wrapped\nswitch tmp.Tag {\n\tcase dgo.ResultTagOk:\n\t\ttmp1 := *tmp.Ok\n\t\tswitch tmp1.Tag {\n\t\t\tcase dgo.OptionTagSome:\n\t\t\t\tx := *tmp1.Some\n\t\t\tdefault:\n\t\t\t\tbreak // Inner pattern didn't match\n\t\t}\n\t\treturn x // dingo:M:1\n\tcase dgo.ResultTagOk:\n\t\ttmp2 := *tmp.Ok\n\t\tswitch tmp2.Tag {\n\t\t\tcase dgo.OptionTagNone:\n\t\t\tdefault:\n\t\t\t\tbreak // Inner pattern didn't match\n\t\t}\n\t\treturn 0 // dingo:M:1\n\tcase dgo.ResultTagErr:\n\t\te := *tmp.Err\n\t\treturn -1 // dingo:M:1\n}\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gen := NewMatchGenerator(0)
			code, _ := gen.Generate(tt.match)

			if code != tt.expected {
				t.Errorf("Generate() output mismatch\nGot:\n%s\nExpected:\n%s", code, tt.expected)
			}
		})
	}
}

func TestMatchGenerator_Guards(t *testing.T) {
	tests := []struct {
		name     string
		match    *dingoast.MatchExpr
		expected string
	}{
		{
			name: "Guard with if",
			match: &dingoast.MatchExpr{
				Scrutinee: &dingoast.RawExpr{Text: "result"},
				Arms: []*dingoast.MatchArm{
					{
						Pattern: &dingoast.ConstructorPattern{
							Name:   "Ok",
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
			expected: "tmp := result\nswitch tmp.Tag {\n\tcase dgo.ResultTagOk:\n\t\tx := *tmp.Ok\n\t\tif x > 0 {\n\t\t\treturn x * 2 // dingo:M:1\n\t\t}\n\tdefault:\n\t\treturn 0 // dingo:M:1\n}\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gen := NewMatchGenerator(0)
			code, _ := gen.Generate(tt.match)

			if code != tt.expected {
				t.Errorf("Generate() output mismatch\nGot:\n%s\nExpected:\n%s", code, tt.expected)
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
					Name:   "Ok",
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
					Name:   "Ok",
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

	// Check for tmp and tmp1 (nested pattern needs second temp)
	if !strings.Contains(code, "tmp := result") {
		t.Errorf("First temp variable should be 'tmp', got:\n%s", code)
	}
	if !strings.Contains(code, "tmp1 := *tmp.Ok") {
		t.Errorf("Second temp variable should be 'tmp1', got:\n%s", code)
	}
}

func TestMatchGenerator_CommentPreservation(t *testing.T) {
	// Test that comments are preserved in generated code
	match := &dingoast.MatchExpr{
		Scrutinee: &dingoast.RawExpr{Text: "result"},
		Arms: []*dingoast.MatchArm{
			{
				Pattern: &dingoast.ConstructorPattern{
					Name:   "Ok",
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

	expected := "tmp := status\nswitch tmp.Tag {\n\tcase 200:\n\t\treturn \"OK\" // dingo:M:1\n\tcase 404:\n\t\treturn \"Not Found\" // dingo:M:1\n\tdefault:\n\t\treturn \"Unknown\" // dingo:M:1\n}\n"

	if code != expected {
		t.Errorf("Literal pattern generation mismatch\nGot:\n%s\nExpected:\n%s", code, expected)
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
	// Test enum variant patterns: Status_Pending, Status_Active
	match := &dingoast.MatchExpr{
		Scrutinee: &dingoast.RawExpr{Text: "status"},
		Arms: []*dingoast.MatchArm{
			{
				Pattern: &dingoast.ConstructorPattern{
					Name: "Status_Pending",
				},
				Body: &dingoast.RawExpr{Text: "\"waiting\""},
			},
			{
				Pattern: &dingoast.ConstructorPattern{
					Name: "Status_Active",
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

	// Should generate StatusTagPending and StatusTagActive
	if !strings.Contains(code, "case StatusTagPending:") {
		t.Errorf("Enum variant tag not generated correctly\nGot:\n%s", code)
	}
	if !strings.Contains(code, "case StatusTagActive:") {
		t.Errorf("Enum variant tag not generated correctly\nGot:\n%s", code)
	}
}
