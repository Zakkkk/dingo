package generator

import (
	"go/token"
	"strings"
	"testing"

	dingoast "github.com/MadAppGang/dingo/pkg/ast"
)

// TestMatchGenerator_SimplePatterns tests basic pattern matching code generation
// using the interface-based type switch pattern
func TestMatchGenerator_SimplePatterns(t *testing.T) {
	tests := []struct {
		name     string
		match    *dingoast.MatchExpr
		contains []string // Check for key patterns instead of exact match
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
			contains: []string{
				"tmp := result",
				"switch v := tmp.(type)",
				"case ResultOk:",
				"x := v.Value",
				"return x // dingo:M:1",
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
			contains: []string{
				"tmp := value",
				"default:",
				"return 0 // dingo:M:1",
			},
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
			contains: []string{
				"tmp := result",
				"switch v := tmp.(type)",
				"case ResultOk:",
				"case ResultErr:",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gen := NewMatchGenerator(0)
			code, _ := gen.Generate(tt.match)

			for _, pattern := range tt.contains {
				if !strings.Contains(code, pattern) {
					t.Errorf("Expected pattern %q not found in output:\n%s", pattern, code)
				}
			}
		})
	}
}

// TestMatchGenerator_NestedPatterns tests nested constructor patterns like Ok(Some(x))
func TestMatchGenerator_NestedPatterns(t *testing.T) {
	match := &dingoast.MatchExpr{
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
					Name:   "Err",
					Params: []dingoast.Pattern{
						&dingoast.VariablePattern{Name: "e"},
					},
				},
				Body: &dingoast.RawExpr{Text: "-1"},
			},
		},
	}

	gen := NewMatchGenerator(0)
	code, _ := gen.Generate(match)

	// Check for nested type switch pattern
	expectedPatterns := []string{
		"tmp := wrapped",
		"switch v := tmp.(type)",
		"case ResultOk:",
		"case ResultErr:",
	}

	for _, pattern := range expectedPatterns {
		if !strings.Contains(code, pattern) {
			t.Errorf("Expected pattern %q not found in output:\n%s", pattern, code)
		}
	}
}

// TestMatchGenerator_Guards tests guard expressions in match arms
func TestMatchGenerator_Guards(t *testing.T) {
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
				Guard: &dingoast.RawExpr{Text: "x > 0"},
				Body:  &dingoast.RawExpr{Text: "x * 2"},
			},
			{
				Pattern: &dingoast.WildcardPattern{},
				Body:    &dingoast.RawExpr{Text: "0"},
			},
		},
	}

	gen := NewMatchGenerator(0)
	code, _ := gen.Generate(match)

	// Check for guard pattern (if statement inside case)
	expectedPatterns := []string{
		"case ResultOk:",
		"if x > 0 {",
		"return x * 2 // dingo:M:1",
		"default:",
		"return 0 // dingo:M:1",
	}

	for _, pattern := range expectedPatterns {
		if !strings.Contains(code, pattern) {
			t.Errorf("Expected pattern %q not found in output:\n%s", pattern, code)
		}
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

	// Check for tmp (first temp variable)
	if !strings.Contains(code, "tmp := result") {
		t.Errorf("First temp variable should be 'tmp', got:\n%s", code)
	}

	// Check for tmp1 in nested pattern (interface-based access)
	if !strings.Contains(code, "tmp1 :=") {
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

	// Check for literal pattern cases
	expectedPatterns := []string{
		"case 200:",
		"return \"OK\" // dingo:M:1",
		"case 404:",
		"return \"Not Found\" // dingo:M:1",
		"default:",
		"return \"Unknown\" // dingo:M:1",
	}

	for _, pattern := range expectedPatterns {
		if !strings.Contains(code, pattern) {
			t.Errorf("Literal pattern generation mismatch\nExpected: %q\nGot:\n%s", pattern, code)
		}
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
	// Test enum variant patterns using interface-based type switch
	// Pattern names should be the full variant type name (e.g., "StatusPending")
	// The match generator preserves these names directly as type switch cases
	match := &dingoast.MatchExpr{
		Scrutinee: &dingoast.RawExpr{Text: "status"},
		Arms: []*dingoast.MatchArm{
			{
				Pattern: &dingoast.ConstructorPattern{
					Name: "Pending", // Short name - generator will use Status prefix
				},
				Body: &dingoast.RawExpr{Text: "\"waiting\""},
			},
			{
				Pattern: &dingoast.ConstructorPattern{
					Name: "Active", // Short name - generator will use Status prefix
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

	// The generator adds "Status" prefix to variant names for type switch cases
	// This matches the enum transpilation: enum Status { Pending } -> StatusPending struct
	if !strings.Contains(code, "case StatusPending:") {
		t.Errorf("Enum variant type case not generated correctly\nGot:\n%s", code)
	}
	if !strings.Contains(code, "case StatusActive:") {
		t.Errorf("Enum variant type case not generated correctly\nGot:\n%s", code)
	}
}
