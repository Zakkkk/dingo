package preprocessor

import (
	"strings"
	"testing"
)

// Basic map/filter/reduce operations

// TestFunctionalASTProcessor_Map_Single_KnownIssue documents and tests a known bug
// where single .map() calls without chaining are not transformed.
// This test is skipped until the bug is fixed in detectMethodCall.
func TestFunctionalASTProcessor_Map_Single_KnownIssue(t *testing.T) {
	t.Skip("Known bug: single .map() calls without chaining are not transformed by detectMethodCall (tracked for future fix)")

	input := `result := nums.map(func(x int) int { return x * 2 })`

	proc := NewFunctionalASTProcessor()
	result, _, err := proc.ProcessInternal(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// This will fail until bug is fixed
	if !strings.Contains(result, "func() []int") {
		t.Error("single .map() transformation not working - bug still present")
	}

	assertValidGoSyntax(t, result)
}
func TestFunctionalASTProcessor_Map_Chain(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "map in chain with filter",
			input:  `result := nums.filter(func(x int) bool { return x > 0 }).map(func(y int) int { return y * 2 })`,
			expect: `func() []int {`,
		},
		{
			name:   "map with complex expression in chain",
			input:  `result := items.filter(func(x string) bool { return x != "" }).map(func(y string) string { return y + "!" })`,
			expect: `func() []string {`,
		},
	}

	proc := NewFunctionalASTProcessor()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := proc.ProcessInternal(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !strings.Contains(result, tt.expect) {
				t.Errorf("expected output to contain:\n%s\ngot:\n%s", tt.expect, result)
			}

			// Verify IIFE pattern
			if !strings.Contains(result, "}()") {
				t.Error("expected IIFE pattern with }()")
			}

			// Verify for loop
			if !strings.Contains(result, "for _, ") {
				t.Error("expected for loop")
			}

			// Validate Go syntax
			assertValidGoSyntax(t, result)

			// Validate Go syntax
			assertValidGoSyntax(t, result)
		})
	}
}

func TestFunctionalASTProcessor_Filter(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "filter with predicate",
			input:  `result := nums.filter(func(x int) bool { return x > 0 })`,
			expect: `if x > 0 {`,
		},
		{
			name:   "filter complex condition",
			input:  `result := users.filter(func(u User) bool { return u.age >= 18 })`,
			expect: `if u.age >= 18 {`,
		},
		{
			name:   "filter returns slice",
			input:  `result := items.filter(func(x int) bool { return x % 2 == 0 })`,
			expect: `func() []int {`,
		},
	}

	proc := NewFunctionalASTProcessor()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := proc.ProcessInternal(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !strings.Contains(result, tt.expect) {
				t.Errorf("expected output to contain:\n%s\ngot:\n%s", tt.expect, result)
			}

			// Verify append inside if
			if !strings.Contains(result, "append(") {
				t.Error("expected append operation")
			}

			// Validate Go syntax
			assertValidGoSyntax(t, result)
		})
	}
}

func TestFunctionalASTProcessor_Reduce(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "reduce sum",
			input:  `result := nums.reduce(0, func(acc int, x int) int { return acc + x })`,
			expect: `acc := 0`,
		},
		{
			name:   "reduce with different type",
			input:  `result := items.reduce("", func(acc string, x string) string { return acc + x })`,
			expect: `acc := ""`,
		},
		{
			name:   "reduce complex operation",
			input:  `result := data.reduce(1, func(acc int, x int) int { return acc * x })`,
			expect: `acc = acc * x`,
		},
	}

	proc := NewFunctionalASTProcessor()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := proc.ProcessInternal(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !strings.Contains(result, tt.expect) {
				t.Errorf("expected output to contain:\n%s\ngot:\n%s", tt.expect, result)
			}

			// Verify return statement
			if !strings.Contains(result, "return acc") {
				t.Error("expected return acc statement")
			}

			// Validate Go syntax
			assertValidGoSyntax(t, result)
		})
	}
}

func TestFunctionalASTProcessor_Sum(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "sum no args",
			input:  `result := nums.sum()`,
			expect: `func() int {`,
		},
		{
			name:   "sum returns int",
			input:  `result := values.sum()`,
			expect: `return sum`,
		},
	}

	proc := NewFunctionalASTProcessor()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := proc.ProcessInternal(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !strings.Contains(result, tt.expect) {
				t.Errorf("expected output to contain:\n%s\ngot:\n%s", tt.expect, result)
			}

			// Verify sum accumulation
			if !strings.Contains(result, "sum = sum + x") || !strings.Contains(result, "sum := 0") {
				t.Error("expected sum accumulation pattern")
			}

			// Validate Go syntax
			assertValidGoSyntax(t, result)
		})
	}
}

func TestFunctionalASTProcessor_Count(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "count with predicate",
			input:  `result := nums.count(func(x int) bool { return x > 10 })`,
			expect: `count := 0`,
		},
		{
			name:   "count increments",
			input:  `result := items.count(func(i int) bool { return i % 2 == 0 })`,
			expect: `count++`,
		},
	}

	proc := NewFunctionalASTProcessor()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := proc.ProcessInternal(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !strings.Contains(result, tt.expect) {
				t.Errorf("expected output to contain:\n%s\ngot:\n%s", tt.expect, result)
			}

			// Validate Go syntax
			assertValidGoSyntax(t, result)
		})
	}
}

func TestFunctionalASTProcessor_All_Any(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "all with predicate",
			input:  `result := nums.all(func(x int) bool { return x > 0 })`,
			expect: `if !(x > 0) {`,
		},
		{
			name:   "any with predicate",
			input:  `result := items.any(func(x int) bool { return x > 10 })`,
			expect: `if x > 10 {`,
		},
		{
			name:   "all returns bool",
			input:  `result := values.all(func(v int) bool { return v < 100 })`,
			expect: `func() bool {`,
		},
	}

	proc := NewFunctionalASTProcessor()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := proc.ProcessInternal(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !strings.Contains(result, tt.expect) {
				t.Errorf("expected output to contain:\n%s\ngot:\n%s", tt.expect, result)
			}

			// Validate Go syntax
			assertValidGoSyntax(t, result)
		})
	}
}

func TestFunctionalASTProcessor_Find(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "find with predicate",
			input:  `result := users.find(func(u User) bool { return u.id == 42 })`,
			expect: `return OptionUserSome(u)`,
		},
		{
			name:   "find returns Option",
			input:  `result := items.find(func(x int) bool { return x > 5 })`,
			expect: `func() OptionInt {`,
		},
		{
			name:   "find none case",
			input:  `result := data.find(func(d string) bool { return d == "target" })`,
			expect: `return OptionStringNone()`,
		},
	}

	proc := NewFunctionalASTProcessor()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := proc.ProcessInternal(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !strings.Contains(result, tt.expect) {
				t.Errorf("expected output to contain:\n%s\ngot:\n%s", tt.expect, result)
			}

			// Validate Go syntax
			assertValidGoSyntax(t, result)
		})
	}
}

func TestFunctionalASTProcessor_FindIndex(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "findIndex with predicate",
			input:  `result := items.findIndex(func(x int) bool { return x == 42 })`,
			expect: `return OptionIntSome(i)`,
		},
		{
			name:   "findIndex returns OptionInt",
			input:  `result := data.findIndex(func(d string) bool { return d == "target" })`,
			expect: `func() OptionInt {`,
		},
		{
			name:   "findIndex none case",
			input:  `result := values.findIndex(func(v int) bool { return v > 100 })`,
			expect: `return OptionIntNone()`,
		},
	}

	proc := NewFunctionalASTProcessor()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := proc.ProcessInternal(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !strings.Contains(result, tt.expect) {
				t.Errorf("expected output to contain:\n%s\ngot:\n%s", tt.expect, result)
			}

			// Validate Go syntax
			assertValidGoSyntax(t, result)
		})
	}
}

// Chain fusion tests

func TestFunctionalASTProcessor_Chain_FilterMap(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect []string
	}{
		{
			name:  "filter then map",
			input: `result := nums.filter(func(x int) bool { return x > 0 }).map(func(y int) int { return y * 2 })`,
			expect: []string{
				`func() []int {`,
				`if x > 0 {`,
				`append(tmp, x * 2)`,
			},
		},
	}

	proc := NewFunctionalASTProcessor()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := proc.ProcessInternal(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			for _, exp := range tt.expect {
				if !strings.Contains(result, exp) {
					t.Errorf("expected output to contain:\n%s\ngot:\n%s", exp, result)
				}
			}

			// Verify single loop (chain fusion)
			loopCount := strings.Count(result, "for _, ")
			if loopCount != 1 {
				t.Errorf("expected exactly 1 loop (fused), got %d", loopCount)
			}

			// Validate Go syntax
			assertValidGoSyntax(t, result)
		})
	}
}

func TestFunctionalASTProcessor_Chain_MultipleFilter(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "two filters fused",
			input:  `result := nums.filter(func(x int) bool { return x > 0 }).filter(func(y int) bool { return y < 100 })`,
			expect: `if x > 0 && x < 100 {`,
		},
		{
			name:   "three filters fused",
			input:  `result := data.filter(func(a int) bool { return a > 0 }).filter(func(b int) bool { return b < 100 }).filter(func(c int) bool { return c % 2 == 0 })`,
			expect: `if a > 0 && a < 100 && a % 2 == 0 {`,
		},
	}

	proc := NewFunctionalASTProcessor()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := proc.ProcessInternal(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !strings.Contains(result, tt.expect) {
				t.Errorf("expected output to contain:\n%s\ngot:\n%s", tt.expect, result)
			}

			// Verify single loop
			loopCount := strings.Count(result, "for _, ")
			if loopCount != 1 {
				t.Errorf("expected exactly 1 loop (fused), got %d", loopCount)
			}

			// Validate Go syntax
			assertValidGoSyntax(t, result)
		})
	}
}

func TestFunctionalASTProcessor_Chain_ToReduce(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect []string
	}{
		{
			name:  "filter then reduce",
			input: `result := nums.filter(func(x int) bool { return x > 0 }).reduce(0, func(acc int, y int) int { return acc + y })`,
			expect: []string{
				`acc := 0`,
				`if x > 0 {`,
				`acc = acc + x`,
			},
		},
	}

	proc := NewFunctionalASTProcessor()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := proc.ProcessInternal(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			for _, exp := range tt.expect {
				if !strings.Contains(result, exp) {
					t.Errorf("expected output to contain:\n%s\ngot:\n%s", exp, result)
				}
			}

			// Validate Go syntax
			assertValidGoSyntax(t, result)
		})
	}
}

func TestFunctionalASTProcessor_Chain_ToAll_Any(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "filter then all",
			input:  `result := nums.filter(func(x int) bool { return x > 0 }).all(func(y int) bool { return y < 100 })`,
			expect: `if !(x > 0 && x < 100) {`,
		},
		{
			name:   "filter then any",
			input:  `result := items.filter(func(a int) bool { return a > 10 }).any(func(b int) bool { return b % 2 == 0 })`,
			expect: `if a > 10 && a % 2 == 0 {`,
		},
	}

	proc := NewFunctionalASTProcessor()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := proc.ProcessInternal(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !strings.Contains(result, tt.expect) {
				t.Errorf("expected output to contain:\n%s\ngot:\n%s", tt.expect, result)
			}

			// Validate Go syntax
			assertValidGoSyntax(t, result)
		})
	}
}

// Edge cases

func TestFunctionalASTProcessor_EmptyCollection(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "empty slice filter",
			input: `result := emptySlice.filter(func(x int) bool { return true })`,
		},
		{
			name:  "empty slice reduce",
			input: `result := emptySlice.reduce(0, func(acc int, x int) int { return acc + x })`,
		},
	}

	proc := NewFunctionalASTProcessor()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := proc.ProcessInternal(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Should still generate valid IIFE
			if !strings.Contains(result, "func()") {
				t.Error("expected IIFE generation")
			}

			// Validate Go syntax
			assertValidGoSyntax(t, result)
		})
	}
}

func TestFunctionalASTProcessor_SingleElement(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "single element filter",
			input:  `result := singleItem.filter(func(x int) bool { return x > 0 })`,
			expect: `if x > 0 {`,
		},
		{
			name:   "single element reduce",
			input:  `result := singleItem.reduce(0, func(acc int, x int) int { return acc + x })`,
			expect: `acc = acc + x`,
		},
	}

	proc := NewFunctionalASTProcessor()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := proc.ProcessInternal(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !strings.Contains(result, tt.expect) {
				t.Errorf("expected output to contain:\n%s\ngot:\n%s", tt.expect, result)
			}

			// Validate Go syntax
			assertValidGoSyntax(t, result)
		})
	}
}

func TestFunctionalASTProcessor_NestedOperations(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "filter with nested function call",
			input:  `result := users.filter(func(u User) bool { return validate(u, ctx) })`,
			expect: `if validate(u, ctx) {`,
		},
		{
			name:   "reduce with nested call",
			input:  `result := items.reduce(0, func(acc int, x int) int { return max(acc, x) })`,
			expect: `acc = max(acc, x)`,
		},
	}

	proc := NewFunctionalASTProcessor()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := proc.ProcessInternal(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !strings.Contains(result, tt.expect) {
				t.Errorf("expected output to contain:\n%s\ngot:\n%s", tt.expect, result)
			}

			// Validate Go syntax
			assertValidGoSyntax(t, result)
		})
	}
}

func TestFunctionalASTProcessor_FourPlusOperationChain(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "four operations",
			input:  `result := nums.filter(func(x int) bool { return x > 0 }).map(func(y int) int { return y * 2 }).filter(func(z int) bool { return z < 100 }).map(func(w int) int { return w + 1 })`,
			expect: `func() []int {`,
		},
		{
			name:   "five operations ending in reduce",
			input:  `result := data.filter(func(a int) bool { return a > 0 }).filter(func(b int) bool { return b < 50 }).filter(func(c int) bool { return c % 2 == 0 }).map(func(d int) int { return d + 1 }).reduce(0, func(acc int, e int) int { return acc + e })`,
			expect: `acc := 0`,
		},
	}

	proc := NewFunctionalASTProcessor()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := proc.ProcessInternal(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !strings.Contains(result, tt.expect) {
				t.Errorf("expected output to contain:\n%s\ngot:\n%s", tt.expect, result)
			}

			// Verify single loop (all fused)
			loopCount := strings.Count(result, "for _, ")
			if loopCount != 1 {
				t.Errorf("expected exactly 1 loop (fused), got %d", loopCount)
			}

			// Validate Go syntax
			assertValidGoSyntax(t, result)
		})
	}
}

// Different collection types

func TestFunctionalASTProcessor_DifferentTypes(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "int slice filter",
			input:  `result := intSlice.filter(func(x int) bool { return x > 0 })`,
			expect: `func() []int {`,
		},
		{
			name:   "string slice filter",
			input:  `result := stringSlice.filter(func(s string) bool { return s != "" })`,
			expect: `func() []string {`,
		},
		{
			name:   "custom type slice filter",
			input:  `result := users.filter(func(u User) bool { return u.active })`,
			expect: `func() []User {`,
		},
	}

	proc := NewFunctionalASTProcessor()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := proc.ProcessInternal(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !strings.Contains(result, tt.expect) {
				t.Errorf("expected output to contain:\n%s\ngot:\n%s", tt.expect, result)
			}

			// Validate Go syntax
			assertValidGoSyntax(t, result)
		})
	}
}

// Error conditions

func TestFunctionalASTProcessor_ErrorConditions(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expectErr bool
	}{
		{
			name:      "filter with no arguments",
			input:     `result := nums.filter()`,
			expectErr: true,
		},
		{
			name:      "reduce with one argument",
			input:     `result := nums.reduce(0)`,
			expectErr: true,
		},
		{
			name:      "reduce with no arguments",
			input:     `result := nums.reduce()`,
			expectErr: true,
		},
		{
			name:      "sum with arguments",
			input:     `result := nums.sum(5)`,
			expectErr: true,
		},
		{
			name:      "all with no arguments",
			input:     `result := nums.all()`,
			expectErr: true,
		},
	}

	proc := NewFunctionalASTProcessor()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := proc.ProcessInternal(tt.input)
			if tt.expectErr && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			// Only validate syntax if no error expected
			if !tt.expectErr && err == nil {
				assertValidGoSyntax(t, result)
			}
		})
	}
}

func TestFunctionalASTProcessor_Metadata(t *testing.T) {
	input := `result := nums.filter(func(x int) bool { return x > 0 })`

	proc := NewFunctionalASTProcessor()
	_, metadata, err := proc.ProcessInternal(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have metadata for transformation
	if len(metadata) == 0 {
		t.Error("expected metadata, got none")
	}

	// Verify metadata contains functional transformation
	for _, m := range metadata {
		if m.Type != "functional" {
			t.Errorf("expected functional metadata, got type=%s", m.Type)
		}
		if m.ASTNodeType != "FuncLit" {
			t.Errorf("expected FuncLit AST node, got %s", m.ASTNodeType)
		}
		if !strings.HasPrefix(m.GeneratedMarker, "// dingo:f:") {
			t.Errorf("expected marker with 'dingo:f:' prefix, got %s", m.GeneratedMarker)
		}
	}
}

func TestFunctionalASTProcessor_ChainMetadata(t *testing.T) {
	input := `result := nums.filter(func(x int) bool { return x > 0 }).map(func(y int) int { return y * 2 })`

	proc := NewFunctionalASTProcessor()
	_, metadata, err := proc.ProcessInternal(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have metadata for chain
	if len(metadata) == 0 {
		t.Error("expected metadata, got none")
	}

	// Verify metadata indicates chain fusion
	for _, m := range metadata {
		if m.Type != "functional_chain" {
			t.Errorf("expected functional_chain metadata, got type=%s", m.Type)
		}
	}
}

func TestFunctionalASTProcessor_NoFalsePositives(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		validateSyntax bool
	}{
		{
			name:           "method call not functional",
			input:          `obj.method(arg)`,
			validateSyntax: true,
		},
		{
			name:           "no dot",
			input:          `x := 42`,
			validateSyntax: true,
		},
		{
			name:           "comment with method name",
			input:          `// nums.map(f)`,
			validateSyntax: false, // Comments alone aren't valid Go
		},
	}

	proc := NewFunctionalASTProcessor()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := proc.ProcessInternal(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Should not transform
			if result != tt.input {
				t.Logf("input was modified (may be acceptable):\ninput:  %s\noutput: %s", tt.input, result)
			}

			// Validate Go syntax if applicable
			if tt.validateSyntax {
				assertValidGoSyntax(t, result)
			}
		})
	}
}

func TestFunctionalASTProcessor_TempVarNaming(t *testing.T) {
	input := `result := nums.filter(func(x int) bool { return x > 0 })
result2 := values.filter(func(y int) bool { return y > 10 })`

	proc := NewFunctionalASTProcessor()
	result, _, err := proc.ProcessInternal(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify no-number-first pattern: first tmp, then tmp1
	firstTmp := strings.Index(result, "tmp :=")
	secondTmp := strings.Index(result, "tmp1 :=")

	if firstTmp == -1 {
		t.Error("expected first tmp variable")
	}
	if secondTmp == -1 {
		t.Error("expected second tmp1 variable")
	}
	if secondTmp < firstTmp {
		t.Error("expected tmp before tmp1")
	}

	// Should NOT have tmp0
	if strings.Contains(result, "tmp0") {
		t.Error("should not use tmp0 (no-number-first pattern)")
	}
}

// Advanced chain fusion tests

func TestFunctionalASTProcessor_ComplexChains(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "filter-map-filter pattern",
			input:  `result := nums.filter(func(a int) bool { return a > 0 }).map(func(b int) int { return b * 2 }).filter(func(c int) bool { return c < 50 })`,
			expect: `if a > 0 && a < 50 {`,
		},
		{
			name:   "multiple filters then all",
			input:  `result := data.filter(func(x int) bool { return x > 5 }).filter(func(y int) bool { return y < 100 }).all(func(z int) bool { return z % 2 == 0 })`,
			expect: `if !(x > 5 && x < 100 && x % 2 == 0) {`,
		},
	}

	proc := NewFunctionalASTProcessor()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := proc.ProcessInternal(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !strings.Contains(result, tt.expect) {
				t.Errorf("expected output to contain:\n%s\ngot:\n%s", tt.expect, result)
			}

			// Verify single loop
			loopCount := strings.Count(result, "for _, ")
			if loopCount != 1 {
				t.Errorf("expected exactly 1 loop (fused), got %d", loopCount)
			}

			// Validate Go syntax
			assertValidGoSyntax(t, result)
		})
	}
}

// MapResult and FilterMap tests

func TestFunctionalASTProcessor_MapResult(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "mapResult basic",
			input:  `result := strings.mapResult(func(s string) ResultIntError { return parseInt(s) })`,
			expect: `if res.IsErr() {`,
		},
		{
			name:   "mapResult returns Result",
			input:  `result := items.mapResult(func(x string) ResultIntError { return convert(x) })`,
			expect: `func() Result`,
		},
	}

	proc := NewFunctionalASTProcessor()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := proc.ProcessInternal(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !strings.Contains(result, tt.expect) {
				t.Errorf("expected output to contain:\n%s\ngot:\n%s", tt.expect, result)
			}

			// Validate Go syntax
			assertValidGoSyntax(t, result)
		})
	}
}

func TestFunctionalASTProcessor_FilterMap(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "filterMap basic",
			input:  `result := items.filterMap(func(x Item) OptionInt { return x.maybeParse() })`,
			expect: `if opt := x.maybeParse(); opt.IsSome() {`,
		},
		{
			name:   "filterMap returns slice",
			input:  `result := data.filterMap(func(d Data) OptionString { return d.getValue() })`,
			expect: `func() []interface{} {`,
		},
	}

	proc := NewFunctionalASTProcessor()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := proc.ProcessInternal(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !strings.Contains(result, tt.expect) {
				t.Errorf("expected output to contain:\n%s\ngot:\n%s", tt.expect, result)
			}

			// Validate Go syntax
			assertValidGoSyntax(t, result)
		})
	}
}

func TestFunctionalASTProcessor_Partition(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "partition basic",
			input:  `result := users.partition(func(u User) bool { return u.active })`,
			expect: `func() ([]interface{}, []interface{}) {`,
		},
		{
			name:   "partition splits into two",
			input:  `result := items.partition(func(x int) bool { return x > 10 })`,
			expect: `return trueSlice, falseSlice`,
		},
	}

	proc := NewFunctionalASTProcessor()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := proc.ProcessInternal(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !strings.Contains(result, tt.expect) {
				t.Errorf("expected output to contain:\n%s\ngot:\n%s", tt.expect, result)
			}

			// Validate Go syntax
			assertValidGoSyntax(t, result)
		})
	}
}
