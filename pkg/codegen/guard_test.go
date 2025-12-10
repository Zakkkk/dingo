package codegen

import (
	"strings"
	"testing"

	dingoast "github.com/MadAppGang/dingo/pkg/ast"
)

func TestInferExprTypeWithBinding(t *testing.T) {
	tests := []struct {
		name       string
		exprText   string
		hasBinding bool
		want       ExprType
	}{
		{
			name:       "With binding - Result type",
			exprText:   "FindUser(id)",
			hasBinding: true,
			want:       TypeResult,
		},
		{
			name:       "Without binding - Option type",
			exprText:   "FindUser(id)",
			hasBinding: false,
			want:       TypeOption,
		},
		{
			name:       "Any expression with binding - Result",
			exprText:   "SomeFunction()",
			hasBinding: true,
			want:       TypeResult,
		},
		{
			name:       "Any expression without binding - Option",
			exprText:   "SomeFunction()",
			hasBinding: false,
			want:       TypeOption,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := InferExprTypeWithBinding(tt.exprText, tt.hasBinding)
			if err != nil {
				t.Fatalf("InferExprTypeWithBinding() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("InferExprTypeWithBinding() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestInferExprType_Deprecated(t *testing.T) {
	// InferExprType is deprecated and always returns error
	_, err := InferExprType("SomeFunction()", nil)
	if err == nil {
		t.Fatal("InferExprType() should return error (deprecated function)")
	}
}

func TestGuardLetGenerator_SingleBinding_Result(t *testing.T) {
	loc := dingoast.GuardLetLocation{
		Start:       0,
		End:         50,
		IsTuple:     false,
		VarNames:    []string{"user"},
		ExprStart:   16,
		ExprEnd:     30,
		ExprText:    "FindUser(id)",
		HasBinding:  true,
		BindingName: "err",
		ElseStart:   40,
		ElseEnd:     48,
		Line:        1,
		Column:      1,
	}

	gen := NewGuardLetGenerator(loc, TypeResult)
	result := gen.Generate()

	output := string(result.Output)

	// Check generated code structure
	if !strings.Contains(output, "tmp := FindUser(id)") {
		t.Errorf("Missing temp assignment, got:\n%s", output)
	}

	if !strings.Contains(output, "if tmp.IsErr()") {
		t.Errorf("Missing IsErr check, got:\n%s", output)
	}

	if !strings.Contains(output, "err := *tmp.Err") {
		t.Errorf("Missing error binding, got:\n%s", output)
	}

	if !strings.Contains(output, "user := *tmp.Ok") {
		t.Errorf("Missing variable binding, got:\n%s", output)
	}
}

func TestGuardLetGenerator_SingleBinding_Option(t *testing.T) {
	loc := dingoast.GuardLetLocation{
		Start:       0,
		End:         45,
		IsTuple:     false,
		VarNames:    []string{"config"},
		ExprStart:   16,
		ExprEnd:     28,
		ExprText:    "LoadConfig()",
		HasBinding:  false,
		BindingName: "",
		ElseStart:   35,
		ElseEnd:     43,
		Line:        1,
		Column:      1,
	}

	gen := NewGuardLetGenerator(loc, TypeOption)
	result := gen.Generate()

	output := string(result.Output)

	// Check generated code structure
	if !strings.Contains(output, "tmp := LoadConfig()") {
		t.Errorf("Missing temp assignment, got:\n%s", output)
	}

	if !strings.Contains(output, "if tmp.IsNone()") {
		t.Errorf("Missing IsNone check, got:\n%s", output)
	}

	if strings.Contains(output, "err :=") {
		t.Errorf("Should not have error binding for Option, got:\n%s", output)
	}

	if !strings.Contains(output, "config := *tmp.Some") {
		t.Errorf("Missing variable binding, got:\n%s", output)
	}
}

func TestGuardLetGenerator_TupleBinding_Result(t *testing.T) {
	loc := dingoast.GuardLetLocation{
		Start:       0,
		End:         60,
		IsTuple:     true,
		VarNames:    []string{"name", "age"},
		ExprStart:   23,
		ExprEnd:     40,
		ExprText:    "ParseInfo(data)",
		HasBinding:  true,
		BindingName: "e",
		ElseStart:   50,
		ElseEnd:     58,
		Line:        1,
		Column:      1,
	}

	gen := NewGuardLetGenerator(loc, TypeResult)
	result := gen.Generate()

	output := string(result.Output)

	// Check generated code structure
	if !strings.Contains(output, "tmp := ParseInfo(data)") {
		t.Errorf("Missing temp assignment, got:\n%s", output)
	}

	if !strings.Contains(output, "if tmp.IsErr()") {
		t.Errorf("Missing IsErr check, got:\n%s", output)
	}

	if !strings.Contains(output, "e := *tmp.Err") {
		t.Errorf("Missing error binding, got:\n%s", output)
	}

	if !strings.Contains(output, "name := (*tmp.Ok).Item1") {
		t.Errorf("Missing first tuple item binding, got:\n%s", output)
	}

	if !strings.Contains(output, "age := (*tmp.Ok).Item2") {
		t.Errorf("Missing second tuple item binding, got:\n%s", output)
	}
}

func TestGuardLetGenerator_TupleBinding_Option(t *testing.T) {
	loc := dingoast.GuardLetLocation{
		Start:       0,
		End:         50,
		IsTuple:     true,
		VarNames:    []string{"x", "y"},
		ExprStart:   20,
		ExprEnd:     32,
		ExprText:    "GetCoords()",
		HasBinding:  false,
		BindingName: "",
		ElseStart:   40,
		ElseEnd:     48,
		Line:        1,
		Column:      1,
	}

	gen := NewGuardLetGenerator(loc, TypeOption)
	result := gen.Generate()

	output := string(result.Output)

	// Check generated code structure
	if !strings.Contains(output, "tmp := GetCoords()") {
		t.Errorf("Missing temp assignment, got:\n%s", output)
	}

	if !strings.Contains(output, "if tmp.IsNone()") {
		t.Errorf("Missing IsNone check, got:\n%s", output)
	}

	if !strings.Contains(output, "x := (*tmp.Some).Item1") {
		t.Errorf("Missing first tuple item binding, got:\n%s", output)
	}

	if !strings.Contains(output, "y := (*tmp.Some).Item2") {
		t.Errorf("Missing second tuple item binding, got:\n%s", output)
	}
}

func TestGuardLetGenerator_VariableNaming(t *testing.T) {
	// Test that multiple guard lets use tmp, tmp1, tmp2 pattern
	loc1 := dingoast.GuardLetLocation{
		VarNames: []string{"a"},
		ExprText: "GetA()",
		Line:     1,
	}

	gen1 := NewGuardLetGenerator(loc1, TypeOption)
	result1 := gen1.Generate()
	output1 := string(result1.Output)

	if !strings.Contains(output1, "tmp := GetA()") {
		t.Errorf("First generator should use 'tmp', got:\n%s", output1)
	}

	// Second generator should use tmp1
	loc2 := dingoast.GuardLetLocation{
		VarNames: []string{"b"},
		ExprText: "GetB()",
		Line:     2,
	}

	gen2 := NewGuardLetGenerator(loc2, TypeOption)
	gen2.Counter = 2 // Simulate second usage
	result2 := gen2.Generate()
	output2 := string(result2.Output)

	if !strings.Contains(output2, "tmp1 := GetB()") {
		t.Errorf("Second generator should use 'tmp1', got:\n%s", output2)
	}
}

func TestGuardLetGenerator_ThreeElementTuple(t *testing.T) {
	loc := dingoast.GuardLetLocation{
		Start:       0,
		End:         70,
		IsTuple:     true,
		VarNames:    []string{"name", "age", "email"},
		ExprStart:   30,
		ExprEnd:     50,
		ExprText:    "ParseUserData(raw)",
		HasBinding:  true,
		BindingName: "err",
		ElseStart:   60,
		ElseEnd:     68,
		Line:        1,
		Column:      1,
	}

	gen := NewGuardLetGenerator(loc, TypeResult)
	result := gen.Generate()

	output := string(result.Output)

	// Check all three tuple items are bound
	if !strings.Contains(output, "name := (*tmp.Ok).Item1") {
		t.Errorf("Missing first tuple item, got:\n%s", output)
	}

	if !strings.Contains(output, "age := (*tmp.Ok).Item2") {
		t.Errorf("Missing second tuple item, got:\n%s", output)
	}

	if !strings.Contains(output, "email := (*tmp.Ok).Item3") {
		t.Errorf("Missing third tuple item, got:\n%s", output)
	}
}

func TestGenerateGuardLet_EntryPoint(t *testing.T) {
	loc := dingoast.GuardLetLocation{
		VarNames: []string{"user"},
		ExprText: "FindUser(id)",
		Line:     1,
	}

	result := GenerateGuardLet(loc, TypeResult)

	output := string(result.Output)

	// Verify it produces valid output
	if len(output) == 0 {
		t.Error("GenerateGuardLet should produce output")
	}

	if !strings.Contains(output, "FindUser(id)") {
		t.Errorf("Output should contain expression, got:\n%s", output)
	}
}

func TestGenerateGuardLetWithSource_ElseBlock(t *testing.T) {
	// guard let user = FindUser(id) else |err| { return Err(err) }
	//                                            ^^^^^^^^^^^^^^^^
	//                                            positions 43-59
	src := []byte(`guard let user = FindUser(id) else |err| { return Err(err) }`)

	loc := dingoast.GuardLetLocation{
		Start:       0,
		End:         60,
		VarNames:    []string{"user"},
		ExprText:    "FindUser(id)",
		HasBinding:  true,
		BindingName: "err",
		ElseStart:   43, // Start after "{ "
		ElseEnd:     59, // End before " }" - "return Err(err)"
		Line:        1,
		Column:      1,
	}

	result := GenerateGuardLetWithSource(loc, TypeResult, src)

	output := string(result.Output)

	// Verify else block content is extracted
	if !strings.Contains(output, "return Err(err)") {
		t.Errorf("Output should contain else block content, got:\n%s", output)
	}

	// Verify no placeholder
	if strings.Contains(output, "GUARD_LET_ELSE_BLOCK") {
		t.Errorf("Output should not contain placeholder when source bytes provided, got:\n%s", output)
	}
}
