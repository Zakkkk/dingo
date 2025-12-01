package preprocessor

import (
	"strings"
	"testing"
)

// Integration tests covering multi-processor combinations
// These tests ensure that AST-based processors work correctly when combined

// LAMBDA + FUNCTIONAL CHAINS

func TestIntegration_LambdaWithFunctionalChain(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains []string
	}{
		{
			name:  "map with lambda",
			input: `result := nums.map((x) => x * 2)`,
			contains: []string{
				// Lambda NOT fused (single map operation)
				"nums.map(func(x __TYPE_INFERENCE_NEEDED) { return x * 2 })",
			},
		},
		{
			name:  "filter and map with lambdas",
			input: `result := nums.filter((x) => x > 5).map((y) => y * 2)`,
			contains: []string{
				// Fusion creates IIFE with combined logic
				"func() []__TYPE_INFERENCE_NEEDED",
				"for _, x := range nums",
				"if x > 5 {",
				"append(tmp, x * 2)",
			},
		},
		{
			name:  "three operation chain with lambdas",
			input: `result := items.filter(x => x != "").map(s => s + "!").reduce((acc, v) => acc + v, "")`,
			contains: []string{
				"func(x __TYPE_INFERENCE_NEEDED) { return x != \"\" }",
				"func(s __TYPE_INFERENCE_NEEDED) { return s + \"!\" }",
				"func(acc __TYPE_INFERENCE_NEEDED, v __TYPE_INFERENCE_NEEDED) { return acc + v }",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// TODO(ast-migration): Reduce with multi-param lambdas not yet supported in functional fusion
			if tt.name == "three operation chain with lambdas" {
				t.Skip("Reduce with multi-param lambdas not yet supported")
			}

			// Process through lambda processor first
			lambdaProc := NewLambdaASTProcessor()
			afterLambda, _, err := lambdaProc.ProcessInternal(tt.input)
			if err != nil {
				t.Fatalf("lambda processing failed: %v", err)
			}

			// Then through functional processor
			funcProc := NewFunctionalASTProcessor()
			result, _, err := funcProc.ProcessInternal(afterLambda)
			if err != nil {
				t.Fatalf("functional processing failed: %v", err)
			}

			for _, expected := range tt.contains {
				if !strings.Contains(result, expected) {
					t.Errorf("expected output to contain:\n%s\ngot:\n%s", expected, result)
				}
			}
		})
	}
}

// SAFE NAV + NULL COALESCE

func TestIntegration_SafeNavWithNullCoalesce(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains []string
	}{
		{
			name:  "safe nav then coalesce",
			input: `let user: UserOption = getUser()
let name = user?.name ?? "Guest"`,
			contains: []string{
				// Safe nav creates IIFE
				"func() StringOption",
				// Null coalesce creates default-first pattern
				"name := \"Guest\"",
			},
		},
		{
			name:  "chained safe nav with coalesce",
			input: `let user: UserOption = getUser()
let city = user?.address?.city ?? "Unknown"`,
			contains: []string{
				"func() StringOption",
				"if user.IsNone()",
				"user.Unwrap().address.IsNone()",
				"city := \"Unknown\"",
			},
		},
		{
			name:  "coalesce then safe nav",
			input: `let user: UserOption = (getUser() ?? defaultUser)
let result = user?.name`,
			contains: []string{
				// Coalesce creates assignment
				"if val := getUser();",
				// Safe nav creates IIFE
				"func() StringOption",
			},
		},
		{
			name:  "multiple operators in expression",
			input: `let user: UserOption = getUser()
let display = user?.profile?.displayName ?? user?.username ?? "Anonymous"`,
			contains: []string{
				"func() StringOption",
				"display := \"Anonymous\"",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// TODO(ast-migration): Safe nav + null coalesce interaction produces mangled output
			// Need to fix processor ordering and output generation
			t.Skip("Safe nav + null coalesce interaction not yet stable")

			// Process through safe nav first
			safeNavProc := NewSafeNavASTProcessor()
			afterSafeNav, _, err := safeNavProc.ProcessInternal(tt.input)
			if err != nil {
				t.Fatalf("safe nav processing failed: %v", err)
			}

			// Then through null coalesce
			coalesceProc := NewNullCoalesceASTProcessor()
			result, _, err := coalesceProc.ProcessInternal(afterSafeNav)
			if err != nil {
				t.Fatalf("null coalesce processing failed: %v", err)
			}

			for _, expected := range tt.contains {
				if !strings.Contains(result, expected) {
					t.Errorf("expected output to contain:\n%s\ngot:\n%s", expected, result)
				}
			}
		})
	}
}

// MATCH + LAMBDA
// Tests that Match and Lambda work together when processed in correct order
// IMPORTANT: Match must run BEFORE Lambda (Match is structural, Lambda follows)

func TestIntegration_MatchWithLambda(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains []string
	}{
		{
			name: "match arm returns lambda",
			input: `match value {
	Some(x) => (y) => x + y,
	None => (y) => y,
}`,
			contains: []string{
				"scrutinee := value",
				"func(y __TYPE_INFERENCE_NEEDED) { return x + y }",
				"func(y __TYPE_INFERENCE_NEEDED) { return y }",
			},
		},
		{
			name: "match with lambda in arm expression",
			input: `match result {
	Ok(x) => items.map((i) => i + x),
	Err(e) => []int{},
}`,
			contains: []string{
				"scrutinee := result",
				"func(i __TYPE_INFERENCE_NEEDED) { return i + x }",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// CORRECT ORDER: Match first (structural), then Lambda
			// This matches the real pipeline order in PassConfig

			// Match processor (structural - runs first)
			matchProc := NewRustMatchASTProcessor()
			afterMatch, _, err := matchProc.Process([]byte(tt.input))
			if err != nil {
				t.Fatalf("match processing failed: %v", err)
			}

			// Lambda processor with body processors
			config := NewDefaultPassConfig()
			bodyProcessors := config.GetBodyProcessors()
			lambdaProc := NewLambdaASTProcessorWithBodyProcessors(bodyProcessors)
			result, _, err := lambdaProc.ProcessInternal(string(afterMatch))
			if err != nil {
				t.Fatalf("lambda processing failed: %v", err)
			}

			for _, expected := range tt.contains {
				if !strings.Contains(result, expected) {
					t.Errorf("expected output to contain:\n%s\ngot:\n%s", expected, result)
				}
			}
		})
	}
}

// ERROR PROP + FUNCTIONAL
// Tests error propagation inside lambda bodies using two-pass architecture with body processor injection

func TestIntegration_ErrorPropWithFunctional(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains []string
	}{
		{
			name:  "error prop in map",
			input: `result := files.map(|f| readFile(f)?)`,
			contains: []string{
				// Lambda transformation
				"func(f __TYPE_INFERENCE_NEEDED)",
				// Error prop creates if err != nil (inside lambda body via body processor)
				"if err != nil",
			},
		},
		{
			name:  "filter with lambda predicate",
			input: `result := items.filter(|x| x.isValid)`,
			contains: []string{
				// Functional processor fuses the filter into an IIFE
				"for _, x := range items",
				"if x.isValid",
			},
		},
		{
			name:  "chained operations with error prop",
			input: `result := data.map(|d| parse(d)?).filter(|x| x > 0)`,
			contains: []string{
				"func(d __TYPE_INFERENCE_NEEDED)",
				"func(x __TYPE_INFERENCE_NEEDED)",
				"if err != nil",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use PassConfig to get body processors for lambda
			config := NewDefaultPassConfig()
			bodyProcessors := config.GetBodyProcessors()

			// Lambda with body processors processes error prop INSIDE lambda bodies
			lambdaProc := NewLambdaASTProcessorWithBodyProcessors(bodyProcessors)
			afterLambda, _, err := lambdaProc.ProcessInternal(tt.input)
			if err != nil {
				t.Fatalf("lambda processing failed: %v", err)
			}

			// Functional processor (optional - for chain fusion)
			funcProc := NewFunctionalASTProcessor()
			result, _, err := funcProc.ProcessInternal(afterLambda)
			if err != nil {
				t.Fatalf("functional processing failed: %v", err)
			}

			for _, expected := range tt.contains {
				if !strings.Contains(result, expected) {
					t.Errorf("expected output to contain:\n%s\ngot:\n%s", expected, result)
				}
			}
		})
	}
}

// LET + ALL OPERATORS

func TestIntegration_LetWithAllOperators(t *testing.T) {
	// Tests let declarations with various operators using correct processor ordering
	tests := []struct {
		name     string
		input    string
		contains []string
		skip     string // If not empty, skip with this reason
	}{
		{
			name:  "let with lambda and functional",
			input: `let doubled = items.map(x => x * 2)`,
			contains: []string{
				"func(x __TYPE_INFERENCE_NEEDED) { return x * 2 }",
			},
		},
		{
			name:  "let with safe nav and coalesce",
			input: `let name = user?.profile?.name ?? "Anonymous"`,
			skip:  "Safe nav requires type inference - pending Case 3",
		},
		{
			name: "let with match and lambda",
			input: `let handler = match mode {
	Fast => (x) => x,
	Slow => (x) => process(x),
}`,
			contains: []string{
				"scrutinee := mode",
				"func(x __TYPE_INFERENCE_NEEDED)",
			},
		},
		{
			name:  "let with error prop",
			input: `let data = readFile()?`,
			contains: []string{
				"if err != nil",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skip != "" {
				t.Skip(tt.skip)
			}

			result := tt.input

			// Let processor
			letProc := NewLetASTProcessor()
			letResult, _, err := letProc.Process([]byte(result))
			if err != nil {
				t.Fatalf("let processing failed: %v", err)
			}
			result = string(letResult)

			// Match processor (structural - before lambda)
			matchProc := NewRustMatchASTProcessor()
			matchResult, _, err := matchProc.Process([]byte(result))
			if err != nil {
				t.Fatalf("match processing failed: %v", err)
			}
			result = string(matchResult)

			// Lambda with body processors
			config := NewDefaultPassConfig()
			bodyProcessors := config.GetBodyProcessors()
			lambdaProc := NewLambdaASTProcessorWithBodyProcessors(bodyProcessors)
			result, _, err = lambdaProc.ProcessInternal(result)
			if err != nil {
				t.Fatalf("lambda processing failed: %v", err)
			}

			// Error prop for top-level expressions
			errorProc := NewErrorPropASTProcessor()
			result, _, err = errorProc.ProcessInternal(result)
			if err != nil {
				t.Fatalf("error prop processing failed: %v", err)
			}

			// Functional processor
			funcProc := NewFunctionalASTProcessor()
			result, _, err = funcProc.ProcessInternal(result)
			if err != nil {
				t.Fatalf("functional processing failed: %v", err)
			}

			for _, expected := range tt.contains {
				if !strings.Contains(result, expected) {
					t.Errorf("expected output to contain:\n%s\ngot:\n%s", expected, result)
				}
			}
		})
	}
}

// COMPLEX REAL-WORLD SCENARIOS

func TestIntegration_ComplexRealWorld_UserProcessing(t *testing.T) {
	t.Skip("Complex multi-processor integration requires type inference for safe nav - test individual processors separately")
	input := `let activeUsers = users
		.filter(u => u.active)
		.map(u => u?.profile?.name ?? "Unknown")
		.filter(name => name != "")`

	// Full pipeline
	result := input

	// Let processor
	letProc := NewLetASTProcessor()
	letResult, _, err := letProc.Process([]byte(result))
	if err != nil {
		t.Fatalf("let processing failed: %v", err)
	}
	result = string(letResult)

	// Other processors
	processors := []struct {
		name string
		proc interface {
			ProcessInternal(string) (string, []TransformMetadata, error)
		}
	}{
		{"lambda", NewLambdaASTProcessor()},
		{"safe-nav", NewSafeNavASTProcessor()},
		{"null-coalesce", NewNullCoalesceASTProcessor()},
		{"functional", NewFunctionalASTProcessor()},
	}

	for _, p := range processors {
		result, _, err = p.proc.ProcessInternal(result)
		if err != nil {
			t.Fatalf("%s processing failed: %v", p.name, err)
		}
	}

	// Verify combined features
	expectations := []string{
		"const activeUsers", // Let immutability
		"func(u __TYPE_INFERENCE_NEEDED) { return u.active }", // Lambda
		"func() __INFER__", // Safe nav IIFE
		"name := \"Unknown\"", // Null coalesce
		// Functional chain fusion (should have fused filters)
	}

	for _, expected := range expectations {
		if !strings.Contains(result, expected) {
			t.Errorf("expected output to contain:\n%s\ngot:\n%s", expected, result)
		}
	}
}

func TestIntegration_ComplexRealWorld_ErrorHandling(t *testing.T) {
	// Tests error propagation combined with match expression
	// Uses correct processor ordering: Let -> Match -> Lambda -> ErrorProp
	input := `let config = match readConfig()? {
	Some(c) => c,
	None => defaultConfig,
}`

	result := input

	// Let processor
	letProc := NewLetASTProcessor()
	letResult, _, err := letProc.Process([]byte(result))
	if err != nil {
		t.Fatalf("let processing failed: %v", err)
	}
	result = string(letResult)

	// Match processor (structural - before lambda)
	matchProc := NewRustMatchASTProcessor()
	matchResult, _, err := matchProc.Process([]byte(result))
	if err != nil {
		t.Fatalf("match processing failed: %v", err)
	}
	result = string(matchResult)

	// Lambda with body processors
	config := NewDefaultPassConfig()
	bodyProcessors := config.GetBodyProcessors()
	lambdaProc := NewLambdaASTProcessorWithBodyProcessors(bodyProcessors)
	result, _, err = lambdaProc.ProcessInternal(result)
	if err != nil {
		t.Fatalf("lambda processing failed: %v", err)
	}

	// Error prop for top-level expressions
	errorProc := NewErrorPropASTProcessor()
	result, _, err = errorProc.ProcessInternal(result)
	if err != nil {
		t.Fatalf("error-prop processing failed: %v", err)
	}

	expectations := []string{
		"if err != nil", // Error prop
		"scrutinee",     // Match
	}

	for _, expected := range expectations {
		if !strings.Contains(result, expected) {
			t.Errorf("expected output to contain:\n%s\ngot:\n%s", expected, result)
		}
	}
}

func TestIntegration_ComplexRealWorld_DataTransform(t *testing.T) {
	// Tests error propagation inside lambda bodies using two-pass architecture
	input := `let results = items
		.map(item => parseItem(item)?)
		.filter(r => r.IsOk())
		.map(r => r.Unwrap())`
	// Note: reduce with multi-param lambdas tested separately

	result := input

	// Let processor
	letProc := NewLetASTProcessor()
	letResult, _, err := letProc.Process([]byte(result))
	if err != nil {
		t.Fatalf("let processing failed: %v", err)
	}
	result = string(letResult)

	// Use PassConfig for body processor injection
	config := NewDefaultPassConfig()
	bodyProcessors := config.GetBodyProcessors()

	// Lambda with body processors (error prop applied inside lambda bodies)
	lambdaProc := NewLambdaASTProcessorWithBodyProcessors(bodyProcessors)
	result, _, err = lambdaProc.ProcessInternal(result)
	if err != nil {
		t.Fatalf("lambda processing failed: %v", err)
	}

	// Functional processor
	funcProc := NewFunctionalASTProcessor()
	result, _, err = funcProc.ProcessInternal(result)
	if err != nil {
		t.Fatalf("functional processing failed: %v", err)
	}

	expectations := []string{
		"func(item __TYPE_INFERENCE_NEEDED)",
		"if err != nil", // Error prop inside lambda body
	}

	for _, expected := range expectations {
		if !strings.Contains(result, expected) {
			t.Errorf("expected output to contain:\n%s\ngot:\n%s", expected, result)
		}
	}
}

func TestIntegration_ComplexRealWorld_NestedStructures(t *testing.T) {
	t.Skip("Complex nested operators with safe nav require type inference - test individual processors separately")
	input := `let processor = match mode {
		Batch => items.map(|x| process(x)?).filter(|r| r.IsOk()),
		Single => items.filter(|x| x?.valid ?? false).map(|x| x.value),
	}`

	result := input

	// Let processor
	letProc := NewLetASTProcessor()
	letResult, _, err := letProc.Process([]byte(result))
	if err != nil {
		t.Fatalf("let processing failed: %v", err)
	}
	result = string(letResult)

	// Other processors
	processors := []struct {
		name string
		proc interface {
			ProcessInternal(string) (string, []TransformMetadata, error)
		}
	}{
		{"lambda", NewLambdaASTProcessor()},
		{"error-prop", NewErrorPropASTProcessor()},
		{"safe-nav", NewSafeNavASTProcessor()},
		{"null-coalesce", NewNullCoalesceASTProcessor()},
		{"functional", NewFunctionalASTProcessor()},
	}

	for _, p := range processors {
		result, _, err = p.proc.ProcessInternal(result)
		if err != nil {
			t.Fatalf("%s processing failed: %v", p.name, err)
		}
	}

	// Match processor
	matchProc := NewRustMatchASTProcessor()
	resultBytes, _, err := matchProc.Process([]byte(result))
	if err != nil {
		t.Fatalf("match processing failed: %v", err)
	}
	result = string(resultBytes)

	expectations := []string{
		"const processor",
		"scrutinee := mode",
		"func(x __TYPE_INFERENCE_NEEDED)",
	}

	for _, expected := range expectations {
		if !strings.Contains(result, expected) {
			t.Errorf("expected output to contain:\n%s\ngot:\n%s", expected, result)
		}
	}
}

// ORDER DEPENDENCY TESTS

func TestIntegration_OrderDependency_LambdaThenFunctional(t *testing.T) {
	t.Skip("Functional chain fusion with lambdas needs refinement - test individual processors separately")
	input := `nums.map(x => x * 2)`

	// Lambda MUST be processed before functional
	lambdaProc := NewLambdaASTProcessor()
	afterLambda, _, err := lambdaProc.ProcessInternal(input)
	if err != nil {
		t.Fatalf("lambda failed: %v", err)
	}

	funcProc := NewFunctionalASTProcessor()
	result, _, err := funcProc.ProcessInternal(afterLambda)
	if err != nil {
		t.Fatalf("functional failed: %v", err)
	}

	// Should have both transformations
	if !strings.Contains(result, "func(x __TYPE_INFERENCE_NEEDED)") {
		t.Error("lambda transformation missing")
	}
	if !strings.Contains(result, "func() []") {
		t.Error("functional transformation missing")
	}
}

func TestIntegration_OrderDependency_SafeNavThenCoalesce(t *testing.T) {
	t.Skip("Safe nav requires type inference - test with explicit type annotations")
	input := `user?.name ?? "Guest"`

	// Safe nav MUST be processed before coalesce
	safeNavProc := NewSafeNavASTProcessor()
	afterSafeNav, _, err := safeNavProc.ProcessInternal(input)
	if err != nil {
		t.Fatalf("safe nav failed: %v", err)
	}

	coalesceProc := NewNullCoalesceASTProcessor()
	result, _, err := coalesceProc.ProcessInternal(afterSafeNav)
	if err != nil {
		t.Fatalf("coalesce failed: %v", err)
	}

	// Should have both transformations
	if !strings.Contains(result, "func() __INFER__") {
		t.Error("safe nav IIFE missing")
	}
	if !strings.Contains(result, "\"Guest\"") {
		t.Error("coalesce default missing")
	}
}

// METADATA PRESERVATION TESTS

func TestIntegration_MetadataPreservation(t *testing.T) {
	input := `let result = nums.filter(x => x > 0).map(y => y * 2)`

	// Process through pipeline collecting metadata
	letProc := NewLetASTProcessor()
	afterLetBytes, _, err := letProc.Process([]byte(input))
	if err != nil {
		t.Fatalf("let failed: %v", err)
	}
	afterLet := string(afterLetBytes)

	lambdaProc := NewLambdaASTProcessor()
	afterLambda, lambdaMeta, err := lambdaProc.ProcessInternal(afterLet)
	if err != nil {
		t.Fatalf("lambda failed: %v", err)
	}

	funcProc := NewFunctionalASTProcessor()
	_, funcMeta, err := funcProc.ProcessInternal(afterLambda)
	if err != nil {
		t.Fatalf("functional failed: %v", err)
	}

	// Verify metadata from processors that provide it
	if len(lambdaMeta) == 0 {
		t.Error("expected lambda metadata")
	}
	if len(funcMeta) == 0 {
		t.Error("expected functional metadata")
	}

	// Verify metadata types
	for _, m := range lambdaMeta {
		if m.Type != "lambda" {
			t.Errorf("expected lambda metadata, got %s", m.Type)
		}
	}
	for _, m := range funcMeta {
		if m.Type != "functional_chain" && m.Type != "functional" {
			t.Errorf("expected functional metadata, got %s", m.Type)
		}
	}
}

// EDGE CASE: All processors in single statement

func TestIntegration_AllProcessorsInOne(t *testing.T) {
	t.Skip("Complex combination of all processors with error prop and safe nav not yet supported - test individual features separately")
	input := `let final = match getMode()? {
		Fast => items.map(|x| x * 2).filter(|x| x > 0),
		Safe => items.filter(|x| validate(x)?).map(|x| x?.value ?? 0),
	}`

	result := input

	// Let processor
	letProc := NewLetASTProcessor()
	letResult, _, err := letProc.Process([]byte(result))
	if err != nil {
		t.Fatalf("let processing failed: %v", err)
	}
	result = string(letResult)

	// Other processors
	processors := []struct {
		name string
		proc interface {
			ProcessInternal(string) (string, []TransformMetadata, error)
		}
	}{
		{"lambda", NewLambdaASTProcessor()},
		{"error-prop", NewErrorPropASTProcessor()},
		{"safe-nav", NewSafeNavASTProcessor()},
		{"null-coalesce", NewNullCoalesceASTProcessor()},
		{"functional", NewFunctionalASTProcessor()},
	}

	for _, p := range processors {
		result, _, err = p.proc.ProcessInternal(result)
		if err != nil {
			t.Fatalf("%s processing failed: %v", p.name, err)
		}
	}

	// Match processor
	matchProc := NewRustMatchASTProcessor()
	resultBytes, _, err := matchProc.Process([]byte(result))
	if err != nil {
		t.Fatalf("match processing failed: %v", err)
	}
	result = string(resultBytes)

	// Should compile without errors and have all transformations
	// This is a stress test - we don't check exact output, just that it processes
	if result == "" {
		t.Error("expected non-empty result")
	}
}
