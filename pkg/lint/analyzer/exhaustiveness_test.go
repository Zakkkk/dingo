package analyzer

import (
	"go/token"
	"strings"
	"testing"

	dingoast "github.com/MadAppGang/dingo/pkg/ast"
)

func TestExhaustivenessAnalyzer_Name(t *testing.T) {
	analyzer := &ExhaustivenessAnalyzer{}
	if got := analyzer.Name(); got != "exhaustiveness" {
		t.Errorf("Name() = %q, want %q", got, "exhaustiveness")
	}
}

func TestExhaustivenessAnalyzer_Category(t *testing.T) {
	analyzer := &ExhaustivenessAnalyzer{}
	if got := analyzer.Category(); got != "correctness" {
		t.Errorf("Category() = %q, want %q", got, "correctness")
	}
}

func TestExhaustivenessAnalyzer_Run(t *testing.T) {
	tests := []struct {
		name              string
		enumVariants      []string
		matchedVariants   []string
		hasWildcard       bool
		expectDiagnostics bool
		expectedMissing   []string
	}{
		{
			name:              "exhaustive match - all variants covered",
			enumVariants:      []string{"Some", "None"},
			matchedVariants:   []string{"Some", "None"},
			hasWildcard:       false,
			expectDiagnostics: false,
		},
		{
			name:              "exhaustive match - wildcard present",
			enumVariants:      []string{"Some", "None"},
			matchedVariants:   []string{"Some"},
			hasWildcard:       true,
			expectDiagnostics: false,
		},
		{
			name:              "non-exhaustive match - missing None",
			enumVariants:      []string{"Some", "None"},
			matchedVariants:   []string{"Some"},
			hasWildcard:       false,
			expectDiagnostics: true,
			expectedMissing:   []string{"None"},
		},
		{
			name:              "non-exhaustive match - missing multiple variants",
			enumVariants:      []string{"Ok", "Err", "Pending"},
			matchedVariants:   []string{"Ok"},
			hasWildcard:       false,
			expectDiagnostics: true,
			expectedMissing:   []string{"Err", "Pending"},
		},
		{
			name:              "empty match arms",
			enumVariants:      []string{"Some", "None"},
			matchedVariants:   []string{},
			hasWildcard:       false,
			expectDiagnostics: true,
			expectedMissing:   []string{"Some", "None"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock enum declaration
			enumDecl := createMockEnumDecl("Option", tt.enumVariants)

			// Create mock match expression
			matchExpr := createMockMatchExpr("option", tt.matchedVariants, tt.hasWildcard)

			// Create mock file with enum and match
			file := &dingoast.File{
				DingoNodes: []dingoast.DingoNode{
					enumDecl,
					&dingoast.ExprWrapper{DingoExpr: matchExpr},
				},
			}

			fset := token.NewFileSet()
			analyzer := &ExhaustivenessAnalyzer{}
			diagnostics := analyzer.Run(fset, file, nil)

			if tt.expectDiagnostics {
				if len(diagnostics) == 0 {
					t.Errorf("Expected diagnostics but got none")
					return
				}

				// Check diagnostic code
				if diagnostics[0].Code != "D001" {
					t.Errorf("Expected diagnostic code D001, got %s", diagnostics[0].Code)
				}

				// Check severity
				if diagnostics[0].Severity != SeverityWarning {
					t.Errorf("Expected severity Warning, got %v", diagnostics[0].Severity)
				}

				// Check category
				if diagnostics[0].Category != "correctness" {
					t.Errorf("Expected category correctness, got %s", diagnostics[0].Category)
				}

				// Verify missing variants in message (simplified check)
				for _, missing := range tt.expectedMissing {
					if !strings.Contains(diagnostics[0].Message, missing) {
						t.Errorf("Expected missing variant %q in message: %s", missing, diagnostics[0].Message)
					}
				}
			} else {
				if len(diagnostics) > 0 {
					t.Errorf("Expected no diagnostics but got: %v", diagnostics)
				}
			}
		})
	}
}

func TestIsWildcardPattern(t *testing.T) {
	tests := []struct {
		name    string
		pattern dingoast.Pattern
		want    bool
	}{
		{
			name:    "wildcard pattern",
			pattern: &dingoast.WildcardPattern{Pos_: token.Pos(1)},
			want:    true,
		},
		{
			name: "constructor pattern",
			pattern: &dingoast.ConstructorPattern{
				Name: "Some",
			},
			want: false,
		},
		{
			name: "variable pattern",
			pattern: &dingoast.VariablePattern{
				Name: "x",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isWildcardPattern(tt.pattern); got != tt.want {
				t.Errorf("isWildcardPattern() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractVariantName(t *testing.T) {
	tests := []struct {
		name    string
		pattern dingoast.Pattern
		want    string
	}{
		{
			name: "constructor pattern",
			pattern: &dingoast.ConstructorPattern{
				Name: "Some",
			},
			want: "Some",
		},
		{
			name: "constructor pattern with params",
			pattern: &dingoast.ConstructorPattern{
				Name: "Ok",
				Params: []dingoast.Pattern{
					&dingoast.VariablePattern{Name: "x"},
				},
			},
			want: "Ok",
		},
		{
			name: "wildcard pattern",
			pattern: &dingoast.WildcardPattern{
				Pos_: token.Pos(1),
			},
			want: "",
		},
		{
			name: "variable pattern",
			pattern: &dingoast.VariablePattern{
				Name: "x",
			},
			want: "",
		},
		{
			name: "literal pattern",
			pattern: &dingoast.LiteralPattern{
				Value: "42",
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractVariantName(tt.pattern); got != tt.want {
				t.Errorf("extractVariantName() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Helper functions to create mock AST nodes

func createMockEnumDecl(name string, variants []string) *dingoast.EnumDecl {
	variantNodes := make([]*dingoast.EnumVariant, len(variants))
	for i, v := range variants {
		variantNodes[i] = &dingoast.EnumVariant{
			Name: &dingoast.Ident{
				Name: v,
			},
			Kind: dingoast.UnitVariant,
		}
	}

	return &dingoast.EnumDecl{
		Name: &dingoast.Ident{
			Name: name,
		},
		Variants: variantNodes,
	}
}

func createMockMatchExpr(scrutinee string, variants []string, hasWildcard bool) *dingoast.MatchExpr {
	arms := make([]*dingoast.MatchArm, 0, len(variants)+1)

	// Add arms for each variant
	for _, v := range variants {
		arms = append(arms, &dingoast.MatchArm{
			Pattern: &dingoast.ConstructorPattern{
				Name: v,
			},
			Body: &dingoast.RawExpr{
				Text: "/* body */",
			},
		})
	}

	// Add wildcard arm if needed
	if hasWildcard {
		arms = append(arms, &dingoast.MatchArm{
			Pattern: &dingoast.WildcardPattern{
				Pos_: token.Pos(1),
			},
			Body: &dingoast.RawExpr{
				Text: "/* default */",
			},
		})
	}

	return &dingoast.MatchExpr{
		Match: token.Pos(1),
		Scrutinee: &dingoast.RawExpr{
			Text: scrutinee,
		},
		Arms:       arms,
		CloseBrace: token.Pos(100),
	}
}
