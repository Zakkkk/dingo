package codegen

import (
	"strings"
	"testing"

	"github.com/MadAppGang/dingo/pkg/ast"
	"go/token"
)

func TestErrorPropGeneratorWithTypeInference(t *testing.T) {
	tests := []struct {
		name         string
		operand      ast.Expr
		returnTypes  []string
		wantContains []string
	}{
		{
			name:         "single nil return",
			operand:      &ast.RawExpr{Text: "foo()"},
			returnTypes:  []string{"nil"},
			wantContains: []string{"tmp, err := foo()", "return nil, err", "tmp"},
		},
		{
			name:         "int return",
			operand:      &ast.RawExpr{Text: "bar()"},
			returnTypes:  []string{"0"},
			wantContains: []string{"tmp, err := bar()", "return 0, err", "tmp"},
		},
		{
			name:         "multiple returns",
			operand:      &ast.RawExpr{Text: "baz()"},
			returnTypes:  []string{"0", `""`},
			wantContains: []string{"tmp, err := baz()", `return 0, "", err`, "tmp"},
		},
		{
			name:        "no explicit return types (fallback to nil)",
			operand:     &ast.RawExpr{Text: "qux()"},
			returnTypes: []string{},
			wantContains: []string{"tmp, err := qux()", "return err", "tmp"},
		},
		{
			name:         "pointer return",
			operand:      &ast.RawExpr{Text: "getUser()"},
			returnTypes:  []string{"nil"},
			wantContains: []string{"tmp, err := getUser()", "return nil, err", "tmp"},
		},
		{
			name:         "mixed types",
			operand:      &ast.RawExpr{Text: "process()"},
			returnTypes:  []string{"0", "nil", `""`},
			wantContains: []string{"tmp, err := process()", `return 0, nil, "", err`, "tmp"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr := &ast.ErrorPropExpr{Operand: tt.operand}
			gen := NewErrorPropGenerator(expr, tt.returnTypes)
			result := gen.Generate()

			code := string(result.Output)
			for _, want := range tt.wantContains {
				if !strings.Contains(code, want) {
					t.Errorf("generated code missing %q:\n%s", want, code)
				}
			}
		})
	}
}

func TestErrorPropGeneratorVariableNaming(t *testing.T) {
	tests := []struct {
		name        string
		wantPairs   [][]string // Each pair is [tmpVar, errVar]
	}{
		{
			name:      "first generation",
			wantPairs: [][]string{{"tmp", "err"}},
		},
		{
			name:      "second generation",
			wantPairs: [][]string{{"tmp", "err"}, {"tmp1", "err1"}},
		},
		{
			name:      "third generation",
			wantPairs: [][]string{{"tmp", "err"}, {"tmp1", "err1"}, {"tmp2", "err2"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gen := NewErrorPropGenerator(&ast.ErrorPropExpr{
				Operand: &ast.RawExpr{Text: "foo()"},
			}, []string{"nil"})

			// Generate multiple times and accumulate output
			for i := 0; i < len(tt.wantPairs); i++ {
				result := gen.Generate()
				code := string(result.Output)

				// Check that the final accumulated output contains all expected variable pairs
				for j := 0; j <= i; j++ {
					tmpVar, errVar := tt.wantPairs[j][0], tt.wantPairs[j][1]
					if !strings.Contains(code, tmpVar+", "+errVar+" :=") {
						t.Errorf("after generation %d: accumulated output missing variables %s, %s\ncode:\n%s",
							i+1, tmpVar, errVar, code)
					}
				}

				// Reset operand for next generation (simulate multiple ? operators)
				gen.Expr.Operand = &ast.RawExpr{Text: "bar()"}
			}
		})
	}
}

func TestErrorPropGeneratorWithDingoIdent(t *testing.T) {
	expr := &ast.ErrorPropExpr{
		Operand: &ast.DingoIdent{
			NamePos: token.NoPos,
			Name:    "myFunc",
		},
	}

	gen := NewErrorPropGenerator(expr, []string{"nil"})
	result := gen.Generate()
	code := string(result.Output)

	// Should use the identifier name directly
	if !strings.Contains(code, "tmp, err := myFunc") {
		t.Errorf("generated code should use identifier name:\n%s", code)
	}
}

func TestErrorPropGeneratorCamelCaseNaming(t *testing.T) {
	// Test that variable names use camelCase, not underscores
	gen := NewErrorPropGenerator(&ast.ErrorPropExpr{
		Operand: &ast.RawExpr{Text: "foo()"},
	}, []string{"nil"})

	result := gen.Generate()
	code := string(result.Output)

	// Should NOT contain underscores in variable names (except maybe in operand)
	// tmp, err := foo() is OK
	// __tmp0, __err0 is NOT OK
	if strings.Contains(code, "__") {
		t.Errorf("generated code should not use double underscores in variable names:\n%s", code)
	}

	// Generate again to check second pair
	result = gen.Generate()
	code = string(result.Output)

	// Should be tmp1, err1 (not __tmp1)
	if !strings.Contains(code, "tmp1, err1") {
		t.Errorf("second generation should use tmp1, err1:\n%s", code)
	}
}
