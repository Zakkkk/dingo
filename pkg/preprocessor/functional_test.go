package preprocessor

import (
	"strings"
	"testing"
)

// Test Map Transformation

func TestMapBasic(t *testing.T) {
	processor := NewFunctionalProcessor()
	// Note: Lambda must be pre-expanded by LambdaProcessor with types
	input := `result := nums.map(func(x int) int { return x * 2 })`

	output, _, err := processor.ProcessInternal(input)
	if err != nil {
		t.Fatalf("ProcessInternal failed: %v", err)
	}

	// Verify key transformation elements - now uses proper types from lambda
	if !strings.Contains(output, "func() []int") {
		t.Errorf("Map should generate IIFE with []int return type, got: %s", output)
	}
	if !strings.Contains(output, "tmp := make([]int, 0, len(nums))") {
		t.Errorf("Map should initialize tmp slice with []int, got: %s", output)
	}
	if !strings.Contains(output, "for _, x := range nums") {
		t.Error("Map should iterate over receiver with correct loop var")
	}
	if !strings.Contains(output, "tmp = append(tmp, x * 2)") {
		t.Error("Map should append transformed value")
	}
	if !strings.Contains(output, "return tmp") {
		t.Error("Map should return tmp slice")
	}
	if !strings.Contains(output, "}()") {
		t.Error("Map should close IIFE with }()")
	}
}

func TestMapWithComplexExpression(t *testing.T) {
	processor := NewFunctionalProcessor()
	input := `result := items.map(func(item Item) int { return item.value * 2 + 1 })`

	output, _, err := processor.ProcessInternal(input)
	if err != nil {
		t.Fatalf("ProcessInternal failed: %v", err)
	}

	// Verify transformation contains the complex expression
	if !strings.Contains(output, "item.value * 2 + 1") {
		t.Errorf("Map transformation should preserve complex expression, got: %s", output)
	}

	// Verify IIFE structure with proper return type
	if !strings.Contains(output, "func() []int") {
		t.Errorf("Map transformation should generate IIFE with []int return type, got: %s", output)
	}
}

func TestMapErrorHandling(t *testing.T) {
	processor := NewFunctionalProcessor()
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "No arguments",
			input: `result := nums.map()`,
			want:  "map() requires exactly 1 argument",
		},
		{
			name:  "Too many arguments",
			input: `result := nums.map(func(x) { return x }, func(y) { return y })`,
			want:  "map() requires exactly 1 argument",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := processor.ProcessInternal(tt.input)
			if err == nil {
				t.Errorf("Expected error containing %q, got nil", tt.want)
			} else if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("Expected error containing %q, got: %v", tt.want, err)
			}
		})
	}
}

// Test Filter Transformation

func TestFilterBasic(t *testing.T) {
	processor := NewFunctionalProcessor()
	input := `result := nums.filter(func(x int) bool { return x > 0 })`

	output, _, err := processor.ProcessInternal(input)
	if err != nil {
		t.Fatalf("ProcessInternal failed: %v", err)
	}

	// Verify key transformation elements - uses param type for slice
	if !strings.Contains(output, "func() []int") {
		t.Errorf("Filter should generate IIFE with []int return type, got: %s", output)
	}
	if !strings.Contains(output, "tmp := make([]int, 0, len(nums))") {
		t.Errorf("Filter should initialize tmp slice with []int, got: %s", output)
	}
	if !strings.Contains(output, "for _, x := range nums") {
		t.Error("Filter should iterate over receiver")
	}
	if !strings.Contains(output, "if x > 0") {
		t.Error("Filter should check predicate")
	}
	if !strings.Contains(output, "tmp = append(tmp, x)") {
		t.Error("Filter should append matching element")
	}
	if !strings.Contains(output, "return tmp") {
		t.Error("Filter should return tmp slice")
	}
}

func TestFilterWithComplexPredicate(t *testing.T) {
	processor := NewFunctionalProcessor()
	input := `result := users.filter(func(u) { return u.age >= 18 && u.active })`

	output, _, err := processor.ProcessInternal(input)
	if err != nil {
		t.Fatalf("ProcessInternal failed: %v", err)
	}

	// Verify predicate is preserved
	if !strings.Contains(output, "u.age >= 18 && u.active") {
		t.Errorf("Filter transformation should preserve predicate, got: %s", output)
	}

	// Verify structure: if predicate { tmp = append(tmp, param) }
	if !strings.Contains(output, "tmp = append(tmp, u)") {
		t.Errorf("Filter should append the filtered element, got: %s", output)
	}
}

func TestFilterErrorHandling(t *testing.T) {
	processor := NewFunctionalProcessor()
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "No arguments",
			input: `result := nums.filter()`,
			want:  "filter() requires exactly 1 argument",
		},
		{
			name:  "Too many arguments",
			input: `result := nums.filter(func(x) { return x > 0 }, func(y) { return y < 10 })`,
			want:  "filter() requires exactly 1 argument",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := processor.ProcessInternal(tt.input)
			if err == nil {
				t.Errorf("Expected error containing %q, got nil", tt.want)
			} else if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("Expected error containing %q, got: %v", tt.want, err)
			}
		})
	}
}

// Test Reduce Transformation

func TestReduceBasic(t *testing.T) {
	processor := NewFunctionalProcessor()
	input := `result := nums.reduce(0, func(acc int, x int) int { return acc + x })`

	output, _, err := processor.ProcessInternal(input)
	if err != nil {
		t.Fatalf("ProcessInternal failed: %v", err)
	}

	// Verify key transformation elements - uses return type from lambda
	if !strings.Contains(output, "func() int") {
		t.Errorf("Reduce should generate IIFE with int return type, got: %s", output)
	}
	if !strings.Contains(output, "acc := 0") {
		t.Error("Reduce should initialize accumulator")
	}
	if !strings.Contains(output, "for _, x := range nums") {
		t.Error("Reduce should iterate over receiver")
	}
	if !strings.Contains(output, "acc = acc + x") {
		t.Error("Reduce should update accumulator")
	}
	if !strings.Contains(output, "return acc") {
		t.Error("Reduce should return accumulator")
	}
}

func TestReduceWithDifferentInitialValue(t *testing.T) {
	processor := NewFunctionalProcessor()
	input := `result := items.reduce(100, func(total, item) { return total + item.value })`

	output, _, err := processor.ProcessInternal(input)
	if err != nil {
		t.Fatalf("ProcessInternal failed: %v", err)
	}

	// Verify initial value
	if !strings.Contains(output, "total := 100") {
		t.Errorf("Reduce should use initial value 100, got: %s", output)
	}

	// Verify accumulator update
	if !strings.Contains(output, "total = total + item.value") {
		t.Errorf("Reduce should update accumulator correctly, got: %s", output)
	}
}

func TestReduceErrorHandling(t *testing.T) {
	processor := NewFunctionalProcessor()
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "No arguments",
			input: `result := nums.reduce()`,
			want:  "reduce() requires exactly 2 arguments",
		},
		{
			name:  "Only one argument",
			input: `result := nums.reduce(0)`,
			want:  "reduce() requires exactly 2 arguments",
		},
		{
			name:  "Too many arguments",
			input: `result := nums.reduce(0, func(a, x) { return a + x }, extra)`,
			want:  "reduce() requires exactly 2 arguments",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := processor.ProcessInternal(tt.input)
			if err == nil {
				t.Errorf("Expected error containing %q, got nil", tt.want)
			} else if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("Expected error containing %q, got: %v", tt.want, err)
			}
		})
	}
}

// Test Sum Transformation

func TestSumBasic(t *testing.T) {
	processor := NewFunctionalProcessor()
	input := `result := nums.sum()`

	output, _, err := processor.ProcessInternal(input)
	if err != nil {
		t.Fatalf("ProcessInternal failed: %v", err)
	}

	// Verify key transformation elements - sum uses int as default type
	if !strings.Contains(output, "func() int") {
		t.Errorf("Sum should generate IIFE with int return type, got: %s", output)
	}
	if !strings.Contains(output, "sum := 0") {
		t.Error("Sum should initialize sum variable")
	}
	if !strings.Contains(output, "for _, x := range nums") {
		t.Error("Sum should iterate over receiver")
	}
	if !strings.Contains(output, "sum = sum + x") {
		t.Error("Sum should accumulate values")
	}
	if !strings.Contains(output, "return sum") {
		t.Error("Sum should return sum variable")
	}
}

func TestSumErrorHandling(t *testing.T) {
	processor := NewFunctionalProcessor()
	input := `result := nums.sum(123)`

	_, _, err := processor.ProcessInternal(input)
	if err == nil {
		t.Error("Expected error for sum() with arguments, got nil")
	} else if !strings.Contains(err.Error(), "sum() takes no arguments") {
		t.Errorf("Expected error about no arguments, got: %v", err)
	}
}

// Test Count Transformation

func TestCountBasic(t *testing.T) {
	processor := NewFunctionalProcessor()
	input := `result := nums.count(func(x) { return x > 0 })`

	output, _, err := processor.ProcessInternal(input)
	if err != nil {
		t.Fatalf("ProcessInternal failed: %v", err)
	}

	// Verify key transformation elements
	if !strings.Contains(output, "func() int") {
		t.Error("Count should generate IIFE with int return type")
	}
	if !strings.Contains(output, "count := 0") {
		t.Error("Count should initialize count variable")
	}
	if !strings.Contains(output, "for _, x := range nums") {
		t.Error("Count should iterate over receiver")
	}
	if !strings.Contains(output, "if x > 0") {
		t.Error("Count should check predicate")
	}
	if !strings.Contains(output, "count++") {
		t.Error("Count should increment counter")
	}
	if !strings.Contains(output, "return count") {
		t.Error("Count should return count variable")
	}
}

func TestCountWithComplexPredicate(t *testing.T) {
	processor := NewFunctionalProcessor()
	input := `result := users.count(func(u) { return u.age >= 18 && u.verified })`

	output, _, err := processor.ProcessInternal(input)
	if err != nil {
		t.Fatalf("ProcessInternal failed: %v", err)
	}

	// Verify return type is int
	if !strings.Contains(output, "func() int") {
		t.Errorf("Count should return int, got: %s", output)
	}

	// Verify increment operation
	if !strings.Contains(output, "count++") {
		t.Errorf("Count should increment counter, got: %s", output)
	}
}

func TestCountErrorHandling(t *testing.T) {
	processor := NewFunctionalProcessor()
	input := `result := nums.count()`

	_, _, err := processor.ProcessInternal(input)
	if err == nil {
		t.Error("Expected error for count() without arguments, got nil")
	} else if !strings.Contains(err.Error(), "count() requires exactly 1 argument") {
		t.Errorf("Expected error about required argument, got: %v", err)
	}
}

// Test All Transformation (Early Exit)

func TestAllBasic(t *testing.T) {
	processor := NewFunctionalProcessor()
	input := `result := nums.all(func(x) { return x > 0 })`

	output, _, err := processor.ProcessInternal(input)
	if err != nil {
		t.Fatalf("ProcessInternal failed: %v", err)
	}

	// Verify key transformation elements
	if !strings.Contains(output, "func() bool") {
		t.Error("All should generate IIFE with bool return type")
	}
	if !strings.Contains(output, "for _, x := range nums") {
		t.Error("All should iterate over receiver")
	}
	if !strings.Contains(output, "if !(x > 0)") {
		t.Error("All should negate predicate for early exit")
	}
	if !strings.Contains(output, "return false") {
		t.Error("All should return false on first failure")
	}
	if !strings.Contains(output, "return true") {
		t.Error("All should return true if all pass")
	}
}

func TestAllEarlyExitStructure(t *testing.T) {
	processor := NewFunctionalProcessor()
	input := `result := items.all(func(item) { return item.valid })`

	output, _, err := processor.ProcessInternal(input)
	if err != nil {
		t.Fatalf("ProcessInternal failed: %v", err)
	}

	// Verify early exit pattern: if !(predicate) { return false }
	if !strings.Contains(output, "if !(item.valid)") {
		t.Errorf("All should negate predicate for early exit, got: %s", output)
	}

	if !strings.Contains(output, "return false") {
		t.Errorf("All should return false on first failure, got: %s", output)
	}

	// Verify final return true (all passed)
	if !strings.Contains(output, "return true") {
		t.Errorf("All should return true if all pass, got: %s", output)
	}
}

func TestAllErrorHandling(t *testing.T) {
	processor := NewFunctionalProcessor()
	input := `result := nums.all()`

	_, _, err := processor.ProcessInternal(input)
	if err == nil {
		t.Error("Expected error for all() without arguments, got nil")
	} else if !strings.Contains(err.Error(), "all() requires exactly 1 argument") {
		t.Errorf("Expected error about required argument, got: %v", err)
	}
}

// Test Any Transformation (Early Exit)

func TestAnyBasic(t *testing.T) {
	processor := NewFunctionalProcessor()
	input := `result := nums.any(func(x) { return x > 0 })`

	output, _, err := processor.ProcessInternal(input)
	if err != nil {
		t.Fatalf("ProcessInternal failed: %v", err)
	}

	// Verify key transformation elements
	if !strings.Contains(output, "func() bool") {
		t.Error("Any should generate IIFE with bool return type")
	}
	if !strings.Contains(output, "for _, x := range nums") {
		t.Error("Any should iterate over receiver")
	}
	if !strings.Contains(output, "if x > 0") {
		t.Error("Any should check predicate for early exit")
	}
	if !strings.Contains(output, "return true") {
		t.Error("Any should return true on first match")
	}
	if !strings.Contains(output, "return false") {
		t.Error("Any should return false if none match")
	}
}

func TestAnyEarlyExitStructure(t *testing.T) {
	processor := NewFunctionalProcessor()
	input := `result := items.any(func(item) { return item.available })`

	output, _, err := processor.ProcessInternal(input)
	if err != nil {
		t.Fatalf("ProcessInternal failed: %v", err)
	}

	// Verify early exit pattern: if predicate { return true }
	if !strings.Contains(output, "if item.available") {
		t.Errorf("Any should check predicate for early exit, got: %s", output)
	}

	if !strings.Contains(output, "return true") {
		t.Errorf("Any should return true on first match, got: %s", output)
	}

	// Verify final return false (none matched)
	if !strings.Contains(output, "return false") {
		t.Errorf("Any should return false if none match, got: %s", output)
	}
}

func TestAnyErrorHandling(t *testing.T) {
	processor := NewFunctionalProcessor()
	input := `result := nums.any()`

	_, _, err := processor.ProcessInternal(input)
	if err == nil {
		t.Error("Expected error for any() without arguments, got nil")
	} else if !strings.Contains(err.Error(), "any() requires exactly 1 argument") {
		t.Errorf("Expected error about required argument, got: %v", err)
	}
}

// Test Naming Convention (No-Number-First Pattern)

func TestNamingConventionTmpVars(t *testing.T) {
	processor := NewFunctionalProcessor()
	// Note: Only one operation per line is supported in single-operation mode
	input := `x := a.map(func(i) { return i * 2 })
y := b.map(func(j) { return j * 3 })`

	output, _, err := processor.ProcessInternal(input)
	if err != nil {
		t.Fatalf("ProcessInternal failed: %v", err)
	}

	// First occurrence: tmp (no number)
	if !strings.Contains(output, "tmp := make") {
		t.Errorf("First temp var should be 'tmp', output: %s", output)
	}

	// Second occurrence: tmp1
	if !strings.Contains(output, "tmp1 := make") {
		t.Errorf("Second temp var should be 'tmp1', output: %s", output)
	}

	// Should NOT contain tmp0, __tmp, or __tmp0
	if strings.Contains(output, "tmp0") || strings.Contains(output, "__tmp") {
		t.Errorf("Should follow no-number-first pattern (tmp, tmp1, not tmp0)")
	}
}

func TestNamingConventionSumVars(t *testing.T) {
	processor := NewFunctionalProcessor()
	input := `x := a.sum()
y := b.sum()`

	output, _, err := processor.ProcessInternal(input)
	if err != nil {
		t.Fatalf("ProcessInternal failed: %v", err)
	}

	// First: sum
	if !strings.Contains(output, "sum := 0") {
		t.Errorf("First sum var should be 'sum', got output: %s", output)
	}

	// Second: sum1
	if !strings.Contains(output, "sum1 := 0") {
		t.Errorf("Second sum var should be 'sum1', got output: %s", output)
	}
}

func TestNamingConventionCountVars(t *testing.T) {
	processor := NewFunctionalProcessor()
	input := `x := a.count(func(i) { return i > 0 })
y := b.count(func(j) { return j < 10 })`

	output, _, err := processor.ProcessInternal(input)
	if err != nil {
		t.Fatalf("ProcessInternal failed: %v", err)
	}

	// First: count
	if !strings.Contains(output, "count := 0") {
		t.Errorf("First count var should be 'count', got output: %s", output)
	}

	// Second: count1
	if !strings.Contains(output, "count1 := 0") {
		t.Errorf("Second count var should be 'count1', got output: %s", output)
	}
}

// Test Metadata Generation

func TestMetadataGeneration(t *testing.T) {
	processor := NewFunctionalProcessor()
	input := `result := nums.map(func(x) { return x * 2 })`

	_, metadata, err := processor.ProcessInternal(input)
	if err != nil {
		t.Fatalf("ProcessInternal failed: %v", err)
	}

	if len(metadata) != 1 {
		t.Fatalf("Expected 1 metadata entry, got %d", len(metadata))
	}

	meta := metadata[0]
	if meta.Type != "functional" {
		t.Errorf("Expected metadata type 'functional', got %q", meta.Type)
	}

	if meta.ASTNodeType != "FuncLit" {
		t.Errorf("Expected AST node type 'FuncLit', got %q", meta.ASTNodeType)
	}

	if !strings.HasPrefix(meta.GeneratedMarker, "// dingo:f:") {
		t.Errorf("Expected marker to start with '// dingo:f:', got %q", meta.GeneratedMarker)
	}
}

// Test Edge Cases

func TestNoFunctionalMethodCall(t *testing.T) {
	processor := NewFunctionalProcessor()
	input := `result := someFunc(x)`

	output, _, err := processor.ProcessInternal(input)
	if err != nil {
		t.Fatalf("ProcessInternal failed: %v", err)
	}

	// Should be unchanged
	if output != input {
		t.Errorf("Non-functional code should be unchanged.\nExpected: %s\nGot: %s", input, output)
	}
}

func TestMultilineInput(t *testing.T) {
	processor := NewFunctionalProcessor()
	input := `x := nums.map(func(n) { return n * 2 })
y := nums.filter(func(n) { return n > 0 })
z := nums.reduce(0, func(acc, n) { return acc + n })`

	output, metadata, err := processor.ProcessInternal(input)
	if err != nil {
		t.Fatalf("ProcessInternal failed: %v", err)
	}

	// Should have 3 transformations
	if len(metadata) != 3 {
		t.Errorf("Expected 3 metadata entries, got %d", len(metadata))
	}

	// Verify all transformations present
	if !strings.Contains(output, "tmp := make") {
		t.Error("Map transformation missing")
	}
	if !strings.Contains(output, "if n > 0") {
		t.Error("Filter transformation missing")
	}
	if !strings.Contains(output, "acc := 0") {
		t.Error("Reduce transformation missing")
	}
}

// Test Option/Result Integration Operations (Task D)

func TestFindBasic(t *testing.T) {
	processor := NewFunctionalProcessor()
	// Use typed lambda to get proper type inference
	input := `result := users.find(func(u User) bool { return u.id == targetId })`

	output, _, err := processor.ProcessInternal(input)
	if err != nil {
		t.Fatalf("ProcessInternal failed: %v", err)
	}

	// Verify key transformation elements - uses camelCase naming
	if !strings.Contains(output, "func() OptionUser") {
		t.Error("find should generate IIFE with OptionUser return type")
	}
	if !strings.Contains(output, "for _, u := range users") {
		t.Error("find should iterate over receiver with correct loop var")
	}
	if !strings.Contains(output, "if u.id == targetId") {
		t.Error("find should check predicate")
	}
	if !strings.Contains(output, "return OptionUserSome(u)") {
		t.Error("find should return OptionUserSome on match")
	}
	if !strings.Contains(output, "return OptionUserNone()") {
		t.Error("find should return OptionUserNone if no match")
	}
}

func TestFindErrorHandling(t *testing.T) {
	processor := NewFunctionalProcessor()
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "No arguments",
			input: `result := nums.find()`,
			want:  "find() requires exactly 1 argument",
		},
		{
			name:  "Too many arguments",
			input: `result := nums.find(func(x) { return x > 0 }, 42)`,
			want:  "find() requires exactly 1 argument",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := processor.ProcessInternal(tt.input)
			if err == nil {
				t.Errorf("Expected error containing %q, got nil", tt.want)
			} else if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("Expected error containing %q, got: %v", tt.want, err)
			}
		})
	}
}

func TestFindIndexBasic(t *testing.T) {
	processor := NewFunctionalProcessor()
	input := `result := items.findIndex(func(x) { return x.name == "target" })`

	output, _, err := processor.ProcessInternal(input)
	if err != nil {
		t.Fatalf("ProcessInternal failed: %v", err)
	}

	// Verify key transformation elements
	if !strings.Contains(output, "func() OptionInt") {
		t.Error("findIndex should generate IIFE with OptionInt return type")
	}
	if !strings.Contains(output, "for i, x := range items") {
		t.Error("findIndex should iterate with index variable")
	}
	if !strings.Contains(output, "if x.name == \"target\"") {
		t.Error("findIndex should check predicate")
	}
	if !strings.Contains(output, "return OptionIntSome(i)") {
		t.Error("findIndex should return Some(index) on match")
	}
	if !strings.Contains(output, "return OptionIntNone()") {
		t.Error("findIndex should return None if no match")
	}
}

func TestMapResultBasic(t *testing.T) {
	processor := NewFunctionalProcessor()
	input := `result := strings.mapResult(func(s) { return parseInt(s) })`

	output, _, err := processor.ProcessInternal(input)
	if err != nil {
		t.Fatalf("ProcessInternal failed: %v", err)
	}

	// Verify key transformation elements
	if !strings.Contains(output, "func() ResultSliceInterfaceError") {
		t.Error("mapResult should generate IIFE with Result return type")
	}
	if !strings.Contains(output, "tmp := make([]interface{}, 0, len(strings))") {
		t.Error("mapResult should initialize tmp slice")
	}
	if !strings.Contains(output, "for _, s := range strings") {
		t.Error("mapResult should iterate over receiver")
	}
	if !strings.Contains(output, "res := parseInt(s)") {
		t.Error("mapResult should call lambda and store result")
	}
	if !strings.Contains(output, "if res.IsErr()") {
		t.Error("mapResult should check for errors")
	}
	if !strings.Contains(output, "return ResultSliceInterfaceErrorErr(res.UnwrapErr())") {
		t.Error("mapResult should propagate errors")
	}
	if !strings.Contains(output, "tmp = append(tmp, res.Unwrap())") {
		t.Error("mapResult should append unwrapped values")
	}
	if !strings.Contains(output, "return ResultSliceInterfaceErrorOk(tmp)") {
		t.Error("mapResult should return Ok with collected values")
	}
}

func TestMapResultErrorHandling(t *testing.T) {
	processor := NewFunctionalProcessor()
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "No arguments",
			input: `result := nums.mapResult()`,
			want:  "mapResult() requires exactly 1 argument",
		},
		{
			name:  "Too many arguments",
			input: `result := nums.mapResult(func(x) { return Ok(x) }, 42)`,
			want:  "mapResult() requires exactly 1 argument",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := processor.ProcessInternal(tt.input)
			if err == nil {
				t.Errorf("Expected error containing %q, got nil", tt.want)
			} else if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("Expected error containing %q, got: %v", tt.want, err)
			}
		})
	}
}

func TestFilterMapBasic(t *testing.T) {
	processor := NewFunctionalProcessor()
	input := `result := items.filterMap(func(x) { return x.maybeParse() })`

	output, _, err := processor.ProcessInternal(input)
	if err != nil {
		t.Fatalf("ProcessInternal failed: %v", err)
	}

	// Verify key transformation elements
	if !strings.Contains(output, "func() []interface{}") {
		t.Error("filterMap should generate IIFE with slice return type")
	}
	if !strings.Contains(output, "tmp := make([]interface{}, 0, len(items))") {
		t.Error("filterMap should initialize tmp slice")
	}
	if !strings.Contains(output, "for _, x := range items") {
		t.Error("filterMap should iterate over receiver")
	}
	if !strings.Contains(output, "if opt := x.maybeParse(); opt.IsSome()") {
		t.Error("filterMap should check if Option is Some in short variable declaration")
	}
	if !strings.Contains(output, "tmp = append(tmp, opt.Unwrap())") {
		t.Error("filterMap should append unwrapped values")
	}
	if !strings.Contains(output, "return tmp") {
		t.Error("filterMap should return collected values")
	}
}

func TestFilterMapErrorHandling(t *testing.T) {
	processor := NewFunctionalProcessor()
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "No arguments",
			input: `result := nums.filterMap()`,
			want:  "filterMap() requires exactly 1 argument",
		},
		{
			name:  "Too many arguments",
			input: `result := nums.filterMap(func(x) { return Some(x) }, 42)`,
			want:  "filterMap() requires exactly 1 argument",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := processor.ProcessInternal(tt.input)
			if err == nil {
				t.Errorf("Expected error containing %q, got nil", tt.want)
			} else if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("Expected error containing %q, got: %v", tt.want, err)
			}
		})
	}
}

func TestPartitionBasic(t *testing.T) {
	processor := NewFunctionalProcessor()
	input := `active, inactive := users.partition(func(u) { return u.active })`

	output, _, err := processor.ProcessInternal(input)
	if err != nil {
		t.Fatalf("ProcessInternal failed: %v", err)
	}

	// Verify key transformation elements
	if !strings.Contains(output, "func() ([]interface{}, []interface{})") {
		t.Error("partition should generate IIFE with tuple return type")
	}
	if !strings.Contains(output, "trueSlice := make([]interface{}, 0, len(users))") {
		t.Error("partition should initialize trueSlice")
	}
	if !strings.Contains(output, "falseSlice := make([]interface{}, 0, len(users))") {
		t.Error("partition should initialize falseSlice")
	}
	if !strings.Contains(output, "for _, u := range users") {
		t.Error("partition should iterate over receiver")
	}
	if !strings.Contains(output, "if u.active") {
		t.Error("partition should check predicate")
	}
	if !strings.Contains(output, "trueSlice = append(trueSlice, u)") {
		t.Error("partition should append to trueSlice on true")
	}
	if !strings.Contains(output, "falseSlice = append(falseSlice, u)") {
		t.Error("partition should append to falseSlice on false")
	}
	if !strings.Contains(output, "return trueSlice, falseSlice") {
		t.Error("partition should return both slices")
	}
}

func TestPartitionErrorHandling(t *testing.T) {
	processor := NewFunctionalProcessor()
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "No arguments",
			input: `a, b := nums.partition()`,
			want:  "partition() requires exactly 1 argument",
		},
		{
			name:  "Too many arguments",
			input: `a, b := nums.partition(func(x) { return x > 0 }, 42)`,
			want:  "partition() requires exactly 1 argument",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := processor.ProcessInternal(tt.input)
			if err == nil {
				t.Errorf("Expected error containing %q, got nil", tt.want)
			} else if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("Expected error containing %q, got: %v", tt.want, err)
			}
		})
	}
}

// Test naming convention for Option/Result operations
func TestNamingConventionOptionResult(t *testing.T) {
	processor := NewFunctionalProcessor()
	input := `
		r1 := items.find(func(x) { return x.id == 1 })
		r2 := items.filterMap(func(x) { return x.parse() })
		r3 := items.partition(func(x) { return x.valid })
	`

	output, _, err := processor.ProcessInternal(input)
	if err != nil {
		t.Fatalf("ProcessInternal failed: %v", err)
	}

	// First operation: no number suffix
	if !strings.Contains(output, "opt :=") {
		t.Error("First filterMap should use 'opt' without number suffix")
	}
	if !strings.Contains(output, "trueSlice :=") {
		t.Error("First partition should use 'trueSlice' without number suffix")
	}
	if !strings.Contains(output, "falseSlice :=") {
		t.Error("First partition should use 'falseSlice' without number suffix")
	}

	// Subsequent operations should have number suffix
	if strings.Contains(output, "opt0") {
		t.Error("Should NOT use zero-based naming (opt0)")
	}
	if strings.Contains(output, "trueSlice0") || strings.Contains(output, "falseSlice0") {
		t.Error("Should NOT use zero-based naming for partition slices")
	}
}
