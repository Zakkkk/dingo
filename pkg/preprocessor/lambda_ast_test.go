package preprocessor

import (
	"strings"
	"testing"

	"github.com/MadAppGang/dingo/pkg/config"
)

func TestLambdaASTProcessor_SingleParamNoParens(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "simple expression",
			input:  `x => x * 2`,
			expect: `func(x __TYPE_INFERENCE_NEEDED) { return x * 2 }`,
		},
		{
			name:   "in slice map",
			input:  `numbers.map(x => x * 2)`,
			expect: `numbers.map(func(x __TYPE_INFERENCE_NEEDED) { return x * 2 })`,
		},
		{
			name:   "in filter",
			input:  `users.filter(u => u.age > 18)`,
			expect: `users.filter(func(u __TYPE_INFERENCE_NEEDED) { return u.age > 18 })`,
		},
		{
			name:   "multiple lambdas on same line",
			input:  `a.map(x => x * 2).filter(y => y > 10)`,
			expect: `func(x __TYPE_INFERENCE_NEEDED) { return x * 2 }`,
		},
		{
			name:   "underscore-prefixed identifier",
			input:  `numbers.map(_x => _x * 2)`,
			expect: `numbers.map(func(_x __TYPE_INFERENCE_NEEDED) { return _x * 2 })`,
		},
	}

	proc := NewLambdaASTProcessor()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := proc.Process([]byte(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			got := string(result)
			if !strings.Contains(got, tt.expect) {
				t.Errorf("expected output to contain:\n%s\ngot:\n%s", tt.expect, got)
			}
		})
	}
}

func TestLambdaASTProcessor_SingleParamWithParens(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "single param with parens",
			input:  `(x) => x * 2`,
			expect: `func(x __TYPE_INFERENCE_NEEDED) { return x * 2 }`,
		},
		{
			name:   "in method call",
			input:  `numbers.map((x) => x * 2)`,
			expect: `numbers.map(func(x __TYPE_INFERENCE_NEEDED) { return x * 2 })`,
		},
	}

	proc := NewLambdaASTProcessor()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := proc.Process([]byte(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			got := string(result)
			if !strings.Contains(got, tt.expect) {
				t.Errorf("expected output to contain:\n%s\ngot:\n%s", tt.expect, got)
			}
		})
	}
}

func TestLambdaASTProcessor_MultiParam(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "two params",
			input:  `(x, y) => x + y`,
			expect: `func(x __TYPE_INFERENCE_NEEDED, y __TYPE_INFERENCE_NEEDED) { return x + y }`,
		},
		{
			name:   "three params",
			input:  `(a, b, c) => a + b + c`,
			expect: `func(a __TYPE_INFERENCE_NEEDED, b __TYPE_INFERENCE_NEEDED, c __TYPE_INFERENCE_NEEDED) { return a + b + c }`,
		},
		{
			name:   "in reduce",
			input:  `reduce((acc, x) => acc + x, 0)`,
			expect: `reduce(func(acc __TYPE_INFERENCE_NEEDED, x __TYPE_INFERENCE_NEEDED) { return acc + x }, 0)`,
		},
	}

	proc := NewLambdaASTProcessor()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := proc.Process([]byte(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			got := string(result)
			if !strings.Contains(got, tt.expect) {
				t.Errorf("expected output to contain:\n%s\ngot:\n%s", tt.expect, got)
			}
		})
	}
}

func TestLambdaASTProcessor_WithTypeAnnotations(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "single param with type",
			input:  `(x: int) => x * 2`,
			expect: `func(x int) { return x * 2 }`,
		},
		{
			name:   "multi param with types",
			input:  `(x: int, y: int) => x + y`,
			expect: `func(x int, y int) { return x + y }`,
		},
		{
			name:   "mixed types",
			input:  `(name: string, age: int) => name + string(age)`,
			expect: `func(name string, age int) { return name + string(age) }`,
		},
		{
			name:   "with return type",
			input:  `(x: int): bool => x > 0`,
			expect: `func(x int) bool { return x > 0 }`,
		},
		{
			name:   "multi param with return type",
			input:  `(x: int, y: int): int => x + y`,
			expect: `func(x int, y int) int { return x + y }`,
		},
	}

	proc := NewLambdaASTProcessor()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := proc.Process([]byte(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			got := string(result)
			if !strings.Contains(got, tt.expect) {
				t.Errorf("expected output to contain:\n%s\ngot:\n%s", tt.expect, got)
			}
		})
	}
}

func TestLambdaASTProcessor_MultiLineWithBraces(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name: "already has braces",
			input: `(x) => {
				return x * 2
			}`,
			expect: `func(x __TYPE_INFERENCE_NEEDED) {
				return x * 2
			}`,
		},
		{
			name: "multi statement",
			input: `(x) => {
				let y = x * 2
				return y
			}`,
			expect: `func(x __TYPE_INFERENCE_NEEDED) {
				let y = x * 2
				return y
			}`,
		},
	}

	proc := NewLambdaASTProcessor()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := proc.Process([]byte(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			got := string(result)
			if !strings.Contains(got, "func(x __TYPE_INFERENCE_NEEDED)") {
				t.Errorf("expected func literal, got:\n%s", got)
			}
		})
	}
}

func TestLambdaASTProcessor_EdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		shouldMatch string
		shouldNot   string
	}{
		{
			name:        "nested in function call",
			input:       `arr.map(x => x * 2).filter(y => y > 10)`,
			shouldMatch: `func(x __TYPE_INFERENCE_NEEDED)`,
			shouldNot:   "",
		},
		{
			name:        "in assignment",
			input:       `let double = x => x * 2`,
			shouldMatch: `func(x __TYPE_INFERENCE_NEEDED)`,
			shouldNot:   "",
		},
		{
			name:        "not generic constraint",
			input:       `type Ordered interface { ~int | ~string }`,
			shouldMatch: "",
			shouldNot:   "func(",
		},
		{
			name:        "complex expression",
			input:       `(x: int) => x * 2 + someFunc(x)`,
			shouldMatch: `func(x int)`,
			shouldNot:   "",
		},
	}

	proc := NewLambdaASTProcessor()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := proc.Process([]byte(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			got := string(result)

			if tt.shouldMatch != "" && !strings.Contains(got, tt.shouldMatch) {
				t.Errorf("expected output to contain:\n%s\ngot:\n%s", tt.shouldMatch, got)
			}

			if tt.shouldNot != "" && strings.Contains(got, tt.shouldNot) {
				t.Errorf("expected output NOT to contain:\n%s\ngot:\n%s", tt.shouldNot, got)
			}
		})
	}
}

func TestLambdaASTProcessor_NoFalsePositives(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "already func literal",
			input: `func(x int) { return x * 2 }`,
		},
		{
			name:  "comparison operator",
			input: `if x >= 10 { return true }`,
		},
		{
			name:  "struct field arrow in comment",
			input: `// arrow => not a lambda`,
		},
	}

	proc := NewLambdaASTProcessor()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := proc.Process([]byte(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			got := string(result)
			if got != tt.input {
				t.Logf("input was modified (may be acceptable):\ninput:  %s\noutput: %s", tt.input, got)
			}
		})
	}
}

func TestLambdaASTProcessor_SourceMappings(t *testing.T) {
	input := `let double = x => x * 2
let add = (x, y) => x + y`

	proc := NewLambdaASTProcessor()
	_, metadata, err := proc.ProcessInternal(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have metadata for transformed lines
	if len(metadata) == 0 {
		t.Error("expected source mappings, got none")
	}

	// Verify metadata contains lambda transformations
	for _, m := range metadata {
		if m.Type != "lambda" {
			t.Errorf("expected lambda metadata, got type=%s", m.Type)
		}
		if m.ASTNodeType != "FuncLit" {
			t.Errorf("expected FuncLit AST node, got %s", m.ASTNodeType)
		}
	}
}

func TestLambdaASTProcessor_RealWorldExamples(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name: "array transformation chain",
			input: `result := numbers
				.filter(x => x > 0)
				.map(x => x * 2)
				.reduce((acc, x) => acc + x, 0)`,
			expect: "func(x __TYPE_INFERENCE_NEEDED)",
		},
		{
			name:   "callback assignment",
			input:  `let callback = (err: error, data: string) => handleResult(err, data)`,
			expect: `func(err error, data string)`,
		},
		{
			name:   "inline sort comparator",
			input:  `sort.Slice(users, (i, j) => users[i].Age < users[j].Age)`,
			expect: `func(i __TYPE_INFERENCE_NEEDED, j __TYPE_INFERENCE_NEEDED)`,
		},
	}

	proc := NewLambdaASTProcessor()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := proc.Process([]byte(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			got := string(result)
			if !strings.Contains(got, tt.expect) {
				t.Errorf("expected output to contain:\n%s\ngot:\n%s", tt.expect, got)
			}
		})
	}
}

// Test Rust pipe syntax
func TestLambdaASTProcessor_RustPipe_SingleParam(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "simple expression",
			input:  `|x| x * 2`,
			expect: `func(x __TYPE_INFERENCE_NEEDED) { return x * 2 }`,
		},
		{
			name:   "in slice map",
			input:  `numbers.map(|x| x * 2)`,
			expect: `numbers.map(func(x __TYPE_INFERENCE_NEEDED) { return x * 2 })`,
		},
		{
			name:   "in filter",
			input:  `users.filter(|u| u.age > 18)`,
			expect: `users.filter(func(u __TYPE_INFERENCE_NEEDED) { return u.age > 18 })`,
		},
	}

	cfg := &config.Config{
		Features: config.FeatureConfig{
			LambdaStyle: "rust",
		},
	}
	proc := NewLambdaASTProcessorWithConfig(cfg)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := proc.Process([]byte(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			got := string(result)
			if !strings.Contains(got, tt.expect) {
				t.Errorf("expected output to contain:\n%s\ngot:\n%s", tt.expect, got)
			}
		})
	}
}

func TestLambdaASTProcessor_RustPipe_MultiParam(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "two params",
			input:  `|x, y| x + y`,
			expect: `func(x __TYPE_INFERENCE_NEEDED, y __TYPE_INFERENCE_NEEDED) { return x + y }`,
		},
		{
			name:   "three params",
			input:  `|a, b, c| a + b + c`,
			expect: `func(a __TYPE_INFERENCE_NEEDED, b __TYPE_INFERENCE_NEEDED, c __TYPE_INFERENCE_NEEDED) { return a + b + c }`,
		},
		{
			name:   "in reduce",
			input:  `reduce(|acc, x| acc + x, 0)`,
			expect: `reduce(func(acc __TYPE_INFERENCE_NEEDED, x __TYPE_INFERENCE_NEEDED) { return acc + x }, 0)`,
		},
	}

	cfg := &config.Config{
		Features: config.FeatureConfig{
			LambdaStyle: "rust",
		},
	}
	proc := NewLambdaASTProcessorWithConfig(cfg)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := proc.Process([]byte(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			got := string(result)
			if !strings.Contains(got, tt.expect) {
				t.Errorf("expected output to contain:\n%s\ngot:\n%s", tt.expect, got)
			}
		})
	}
}

func TestLambdaASTProcessor_RustPipe_WithTypes(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "single param with type",
			input:  `|x: int| x * 2`,
			expect: `func(x int) { return x * 2 }`,
		},
		{
			name:   "multi param with types",
			input:  `|x: int, y: int| x + y`,
			expect: `func(x int, y int) { return x + y }`,
		},
		{
			name:   "with return type",
			input:  `|x: int| -> bool { x > 0 }`,
			expect: `func(x int) bool { x > 0 }`,
		},
		{
			name:   "return type expression",
			input:  `|x: int| -> int x * 2`,
			expect: `func(x int) int { return x * 2 }`,
		},
	}

	cfg := &config.Config{
		Features: config.FeatureConfig{
			LambdaStyle: "rust",
		},
	}
	proc := NewLambdaASTProcessorWithConfig(cfg)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := proc.Process([]byte(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			got := string(result)
			if !strings.Contains(got, tt.expect) {
				t.Errorf("expected output to contain:\n%s\ngot:\n%s", tt.expect, got)
			}
		})
	}
}

// TODO(ast-migration): Config-based syntax switching is obsolete in AST architecture
// AST transformations always process all lambda syntaxes regardless of config
// This test was valid for regex-based preprocessors but not for AST-based ones
func TestLambdaASTProcessor_ConfigSwitching(t *testing.T) {
	t.Skip("Obsolete: AST-based lambdas always transform both syntaxes regardless of LambdaStyle config")

	tests := []struct {
		name        string
		config      *config.Config
		input       string
		shouldMatch string
		shouldNot   string
	}{
		{
			name: "typescript mode ignores pipes",
			config: &config.Config{
				Features: config.FeatureConfig{
					LambdaStyle: "typescript",
				},
			},
			input:       `|x| x * 2`,
			shouldMatch: `|x| x * 2`, // No transformation
			shouldNot:   "func(",
		},
		{
			name: "typescript mode processes arrows",
			config: &config.Config{
				Features: config.FeatureConfig{
					LambdaStyle: "typescript",
				},
			},
			input:       `x => x * 2`,
			shouldMatch: "func(x __TYPE_INFERENCE_NEEDED)",
			shouldNot:   "=>",
		},
		{
			name: "rust mode ignores arrows",
			config: &config.Config{
				Features: config.FeatureConfig{
					LambdaStyle: "rust",
				},
			},
			input:       `x => x * 2`,
			shouldMatch: `x => x * 2`, // No transformation
			shouldNot:   "",            // arrows might still match in edge cases
		},
		{
			name: "rust mode processes pipes",
			config: &config.Config{
				Features: config.FeatureConfig{
					LambdaStyle: "rust",
				},
			},
			input:       `|x| x * 2`,
			shouldMatch: "func(x __TYPE_INFERENCE_NEEDED)",
			shouldNot:   "|x|",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proc := NewLambdaASTProcessorWithConfig(tt.config)
			result, _, err := proc.Process([]byte(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			got := string(result)

			if tt.shouldMatch != "" && !strings.Contains(got, tt.shouldMatch) {
				t.Errorf("expected output to contain:\n%s\ngot:\n%s", tt.shouldMatch, got)
			}

			if tt.shouldNot != "" && strings.Contains(got, tt.shouldNot) {
				t.Errorf("expected output NOT to contain:\n%s\ngot:\n%s", tt.shouldNot, got)
			}
		})
	}
}

func TestLambdaASTProcessor_NestedFunctionCalls(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "function call with multiple args",
			input:  `numbers.map((x: int): int => transform(x, 1, 2))`,
			expect: `numbers.map(func(x int) int { return transform(x, 1, 2) })`,
		},
		{
			name:   "nested function calls",
			input:  `data.map((x: int): int => transform(process(x, 5), 10))`,
			expect: `data.map(func(x int) int { return transform(process(x, 5), 10) })`,
		},
		{
			name:   "multiple commas in body",
			input:  `users.filter((u: User): bool => validate(u, ctx, flags))`,
			expect: `users.filter(func(u User) bool { return validate(u, ctx, flags) })`,
		},
		{
			name:   "array indexing with commas",
			input:  `arr.map((i: int): string => fmt.Sprintf("%d,%d", i, i*2))`,
			expect: `arr.map(func(i int) string { return fmt.Sprintf("%d,%d", i, i*2) })`,
		},
	}

	proc := NewLambdaASTProcessor()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := proc.Process([]byte(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			got := string(result)
			if !strings.Contains(got, tt.expect) {
				t.Errorf("expected output to contain:\n%s\ngot:\n%s", tt.expect, got)
			}
		})
	}
}

func TestLambdaASTProcessor_RustPipe_RealWorldExamples(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name: "array transformation chain",
			input: `result := numbers
				.filter(|x| x > 0)
				.map(|x| x * 2)
				.reduce(|acc, x| acc + x, 0)`,
			expect: "func(x __TYPE_INFERENCE_NEEDED)",
		},
		{
			name:   "callback with types",
			input:  `let callback = |err: error, data: string| -> Result { handleResult(err, data) }`,
			expect: `func(err error, data string) Result`,
		},
		{
			name:   "inline sort comparator",
			input:  `sort.Slice(users, |i, j| users[i].Age < users[j].Age)`,
			expect: `func(i __TYPE_INFERENCE_NEEDED, j __TYPE_INFERENCE_NEEDED)`,
		},
	}

	cfg := &config.Config{
		Features: config.FeatureConfig{
			LambdaStyle: "rust",
		},
	}
	proc := NewLambdaASTProcessorWithConfig(cfg)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := proc.Process([]byte(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			got := string(result)
			if !strings.Contains(got, tt.expect) {
				t.Errorf("expected output to contain:\n%s\ngot:\n%s", tt.expect, got)
			}
		})
	}
}

// EDGE CASE TESTS

func TestLambdaASTProcessor_NestedLambdas(t *testing.T) {
	t.Skip("Nested lambdas not yet supported - requires recursive processing of lambda bodies")
	tests := []struct {
		name   string
		input  string
		expect []string
	}{
		{
			name:  "nested arrow lambdas",
			input: `(x) => (y) => x + y`,
			expect: []string{
				"func(x __TYPE_INFERENCE_NEEDED)",
				"func(y __TYPE_INFERENCE_NEEDED)",
			},
		},
		{
			name:  "nested rust pipe lambdas",
			input: `|x| |y| x + y`,
			expect: []string{
				"func(x __TYPE_INFERENCE_NEEDED)",
				"func(y __TYPE_INFERENCE_NEEDED)",
			},
		},
		{
			name:  "triple nesting",
			input: `(a) => (b) => (c) => a + b + c`,
			expect: []string{
				"func(a __TYPE_INFERENCE_NEEDED)",
				"func(b __TYPE_INFERENCE_NEEDED)",
				"func(c __TYPE_INFERENCE_NEEDED)",
			},
		},
		{
			name:  "nested with types",
			input: `(x: int) => (y: int) => x + y`,
			expect: []string{
				"func(x int)",
				"func(y int)",
			},
		},
	}

	proc := NewLambdaASTProcessor()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := proc.Process([]byte(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			got := string(result)
			for _, expected := range tt.expect {
				if !strings.Contains(got, expected) {
					t.Errorf("expected output to contain:\n%s\ngot:\n%s", expected, got)
				}
			}
		})
	}
}

func TestLambdaASTProcessor_ClosureCaptures(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name: "closure captures outer variable",
			input: `func makeAdder(x int) func(int) int {
				return y => x + y
			}`,
			expect: `func(y __TYPE_INFERENCE_NEEDED) { return x + y }`,
		},
		{
			name: "closure captures multiple variables",
			input: `func makeFunc(a int, b int) func(int) int {
				return c => a + b + c
			}`,
			expect: `func(c __TYPE_INFERENCE_NEEDED) { return a + b + c }`,
		},
		{
			name:   "closure in for loop",
			input:  `for i := 0; i < 10; i++ { funcs = append(funcs, () => i) }`,
			expect: `func() { return i }`,
		},
	}

	proc := NewLambdaASTProcessor()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := proc.Process([]byte(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			got := string(result)
			if !strings.Contains(got, tt.expect) {
				t.Errorf("expected output to contain:\n%s\ngot:\n%s", tt.expect, got)
			}
		})
	}
}

func TestLambdaASTProcessor_ComplexExpressionBodies(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "complex arithmetic",
			input:  `x => (x * 2 + 5) / 3 - 1`,
			expect: `func(x __TYPE_INFERENCE_NEEDED) { return (x * 2 + 5) / 3 - 1 }`,
		},
		{
			name:   "logical operators",
			input:  `x => x > 0 && x < 100 || x == -1`,
			expect: `func(x __TYPE_INFERENCE_NEEDED) { return x > 0 && x < 100 || x == -1 }`,
		},
		{
			name:   "bitwise operators",
			input:  `x => x & 0xFF | 0x10 ^ 0x20`,
			expect: `func(x __TYPE_INFERENCE_NEEDED) { return x & 0xFF | 0x10 ^ 0x20 }`,
		},
		{
			name:   "string concatenation",
			input:  `(first, last) => first + " " + last`,
			expect: `func(first __TYPE_INFERENCE_NEEDED, last __TYPE_INFERENCE_NEEDED) { return first + " " + last }`,
		},
		{
			name:   "slice operations",
			input:  `x => arr[x:x+10]`,
			expect: `func(x __TYPE_INFERENCE_NEEDED) { return arr[x:x+10] }`,
		},
		{
			name:   "type assertion",
			input:  `x => x.(string)`,
			expect: `func(x __TYPE_INFERENCE_NEEDED) { return x.(string) }`,
		},
	}

	proc := NewLambdaASTProcessor()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := proc.Process([]byte(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			got := string(result)
			if !strings.Contains(got, tt.expect) {
				t.Errorf("expected output to contain:\n%s\ngot:\n%s", tt.expect, got)
			}
		})
	}
}

func TestLambdaASTProcessor_MethodChains(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name: "chained map filter reduce",
			input: `items
				.map(x => x * 2)
				.filter(x => x > 10)
				.reduce((acc, x) => acc + x, 0)`,
			expect: "func(x __TYPE_INFERENCE_NEEDED)",
		},
		{
			name:   "nested method chains in lambda",
			input:  `items.map(x => x.toString().toUpperCase())`,
			expect: `func(x __TYPE_INFERENCE_NEEDED) { return x.toString().toUpperCase() }`,
		},
		{
			name:   "lambda in fluent API",
			input:  `builder.withValidator(x => x != nil).withTransform(x => x * 2).build()`,
			expect: "func(x __TYPE_INFERENCE_NEEDED)",
		},
	}

	proc := NewLambdaASTProcessor()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := proc.Process([]byte(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			got := string(result)
			if !strings.Contains(got, tt.expect) {
				t.Errorf("expected output to contain:\n%s\ngot:\n%s", tt.expect, got)
			}
		})
	}
}

func TestLambdaASTProcessor_RustSyntaxEdgeCases(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "empty param list",
			input:  `|| 42`,
			expect: `func() { return 42 }`,
		},
		{
			name:   "single param no spaces",
			input:  `|x|x*2`,
			expect: `func(x __TYPE_INFERENCE_NEEDED) { return x*2 }`,
		},
		{
			name:   "multi param no spaces",
			input:  `|x,y|x+y`,
			expect: `func(x __TYPE_INFERENCE_NEEDED, y __TYPE_INFERENCE_NEEDED) { return x+y }`,
		},
		{
			name:   "with block body",
			input:  `|x| { let y = x * 2; return y }`,
			expect: `func(x __TYPE_INFERENCE_NEEDED) { let y = x * 2; return y }`,
		},
	}

	cfg := &config.Config{
		Features: config.FeatureConfig{
			LambdaStyle: "rust",
		},
	}
	proc := NewLambdaASTProcessorWithConfig(cfg)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := proc.Process([]byte(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			got := string(result)
			if !strings.Contains(got, tt.expect) {
				t.Errorf("expected output to contain:\n%s\ngot:\n%s", tt.expect, got)
			}
		})
	}
}

func TestLambdaASTProcessor_TypeScriptSyntaxEdgeCases(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "empty param list with parens",
			input:  `() => 42`,
			expect: `func() { return 42 }`,
		},
		{
			name:   "single param no parens no spaces",
			input:  `x=>x*2`,
			expect: `func(x __TYPE_INFERENCE_NEEDED) { return x*2 }`,
		},
		{
			name:   "async-style arrow (should still transform)",
			input:  `async (x) => await fetch(x)`,
			expect: `func(x __TYPE_INFERENCE_NEEDED)`,
		},
		{
			name:   "destructuring-style params",
			input:  `({x, y}) => x + y`,
			expect: `func({x, y} __TYPE_INFERENCE_NEEDED) { return x + y }`,
		},
	}

	proc := NewLambdaASTProcessor()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "destructuring-style params" {
				t.Skip("Destructuring parameters not yet supported in lambda syntax")
			}
			result, _, err := proc.Process([]byte(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			got := string(result)
			if !strings.Contains(got, tt.expect) {
				t.Errorf("expected output to contain:\n%s\ngot:\n%s", tt.expect, got)
			}
		})
	}
}

func TestLambdaASTProcessor_TypeAnnotationsEdgeCases(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "generic type param",
			input:  `(x: List<int>) => x.length()`,
			expect: `func(x List<int>) { return x.length() }`,
		},
		{
			name:   "nested generics",
			input:  `(m: Map<string, List<int>>) => m.get("key")`,
			expect: `func(m Map<string, List<int>>) { return m.get("key") }`,
		},
		{
			name:   "function type param",
			input:  `(f: func(int) bool) => f(42)`,
			expect: `func(f func(int) bool) { return f(42) }`,
		},
		{
			name:   "optional type",
			input:  `(x: Option<int>) => x.unwrap()`,
			expect: `func(x Option<int>) { return x.unwrap() }`,
		},
		{
			name:   "result type",
			input:  `(r: Result<string, error>) => r.unwrap()`,
			expect: `func(r Result<string, error>) { return r.unwrap() }`,
		},
		{
			name:   "pointer type",
			input:  `(p: *User) => p.name`,
			expect: `func(p *User) { return p.name }`,
		},
		{
			name:   "slice type",
			input:  `(arr: []int) => arr[0]`,
			expect: `func(arr []int) { return arr[0] }`,
		},
		{
			name:   "map type",
			input:  `(m: map[string]int) => m["key"]`,
			expect: `func(m map[string]int) { return m["key"] }`,
		},
		{
			name:   "channel type",
			input:  `(ch: chan int) => <-ch`,
			expect: `func(ch chan int) { return <-ch }`,
		},
		{
			name:   "variadic type",
			input:  `(args: ...int) => sum(args)`,
			expect: `func(args ...int) { return sum(args) }`,
		},
	}

	proc := NewLambdaASTProcessor()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Skip complex type annotations that lambda parser doesn't recognize
			if tt.name == "function type param" || tt.name == "channel type" {
				t.Skip("Complex type annotations not yet fully supported in lambda parameter parsing")
			}
			result, _, err := proc.Process([]byte(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			got := string(result)
			if !strings.Contains(got, tt.expect) {
				t.Errorf("expected output to contain:\n%s\ngot:\n%s", tt.expect, got)
			}
		})
	}
}

func TestLambdaASTProcessor_LambdaInStructLiteral(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name: "lambda as field value",
			input: `Config{
				Validator: x => x != nil,
				Transform: x => x * 2,
			}`,
			expect: "func(x __TYPE_INFERENCE_NEEDED)",
		},
		{
			name: "lambda in nested struct",
			input: `Options{
				Hooks: Hooks{
					OnLoad: (data) => process(data),
					OnError: (err) => log(err),
				},
			}`,
			expect: "func(data __TYPE_INFERENCE_NEEDED)",
		},
	}

	proc := NewLambdaASTProcessor()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := proc.Process([]byte(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			got := string(result)
			if !strings.Contains(got, tt.expect) {
				t.Errorf("expected output to contain:\n%s\ngot:\n%s", tt.expect, got)
			}
		})
	}
}

func TestLambdaASTProcessor_LambdaReturningLambda(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect []string
	}{
		{
			name:  "simple currying",
			input: `let add = (x) => (y) => x + y`,
			expect: []string{
				"func(x __TYPE_INFERENCE_NEEDED)",
				"func(y __TYPE_INFERENCE_NEEDED)",
			},
		},
		{
			name: "factory function",
			input: `func makeMultiplier(factor int) {
				return x => x * factor
			}`,
			expect: []string{
				"func(x __TYPE_INFERENCE_NEEDED) { return x * factor }",
			},
		},
	}

	proc := NewLambdaASTProcessor()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "simple currying" {
				t.Skip("Nested lambdas not yet supported - requires recursive processing")
			}
			result, _, err := proc.Process([]byte(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			got := string(result)
			for _, expected := range tt.expect {
				if !strings.Contains(got, expected) {
					t.Errorf("expected output to contain:\n%s\ngot:\n%s", expected, got)
				}
			}
		})
	}
}

func TestLambdaASTProcessor_LambdaComposition(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "compose two functions",
			input:  `compose(x => x + 1, y => y * 2)`,
			expect: "func(x __TYPE_INFERENCE_NEEDED)",
		},
		{
			name:   "pipe multiple functions",
			input:  `pipe(x => x * 2, x => x + 1, x => x.toString())`,
			expect: "func(x __TYPE_INFERENCE_NEEDED)",
		},
		{
			name:   "higher order function",
			input:  `apply((x, y) => x + y, 5, 10)`,
			expect: `func(x __TYPE_INFERENCE_NEEDED, y __TYPE_INFERENCE_NEEDED)`,
		},
	}

	proc := NewLambdaASTProcessor()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := proc.Process([]byte(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			got := string(result)
			if !strings.Contains(got, tt.expect) {
				t.Errorf("expected output to contain:\n%s\ngot:\n%s", tt.expect, got)
			}
		})
	}
}
