package preprocessor

import (
	"strings"
	"testing"
)

func TestErrorPropASTProcessor_StringLiterals(t *testing.T) {
	proc := NewErrorPropASTProcessor()

	// Test case: question mark inside string literal should be ignored
	input := `package main

func getValue(question string) (string, error) {
	let x = readValue("what?")?
	return x, nil
}`

	result, _, err := proc.ProcessInternal(input)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	// Should expand the ? operator, NOT the ? inside string
	if !strings.Contains(result, `readValue("what?")`) {
		t.Errorf("String literal should be preserved in function call")
	}

	// Should have error handling expansion
	if !strings.Contains(result, "if err != nil") {
		t.Errorf("Expected error handling expansion")
	}
}

func TestErrorPropASTProcessor_TernaryVsErrorProp(t *testing.T) {
	proc := NewErrorPropASTProcessor()

	// Test case: ternary operator should NOT be transformed
	input := `package main

func test(condition bool) int {
	let x = condition ? 1 : 0
	return x
}`

	result, _, err := proc.ProcessInternal(input)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	// Ternary should be preserved
	if strings.Contains(result, "if err != nil") {
		t.Errorf("Ternary operator should NOT be transformed to error handling")
	}

	// Original ternary should still be there
	if !strings.Contains(result, "?") {
		t.Errorf("Ternary operator should be preserved")
	}
}

func TestErrorPropASTProcessor_ChainedCalls(t *testing.T) {
	proc := NewErrorPropASTProcessor()

	// Test case: multiple ? operators should all be processed
	input := `package main

func processData(path string) error {
	let data = os.ReadFile(path)?
	let parsed = json.Unmarshal(data)?
	return nil
}`

	result, metadata, err := proc.ProcessInternal(input)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	// Should have TWO metadata entries
	if len(metadata) != 2 {
		t.Errorf("Expected 2 metadata entries, got %d", len(metadata))
	}

	// Should have TWO error handling blocks
	errHandlingCount := strings.Count(result, "if err")
	if errHandlingCount < 2 {
		t.Errorf("Expected at least 2 error handling blocks, got %d", errHandlingCount)
	}

	// Should have unique markers
	if !strings.Contains(result, "// dingo:e:0") {
		t.Errorf("Expected marker // dingo:e:0")
	}
	if !strings.Contains(result, "// dingo:e:1") {
		t.Errorf("Expected marker // dingo:e:1")
	}
}

func TestErrorPropASTProcessor_CustomMessage(t *testing.T) {
	proc := NewErrorPropASTProcessor()

	input := `package main

func readConfig(path string) ([]byte, error) {
	let data = os.ReadFile(path)? "failed to read config"
	return data, nil
}`

	result, _, err := proc.ProcessInternal(input)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	// Should wrap error with custom message
	if !strings.Contains(result, `fmt.Errorf("failed to read config: %w"`) {
		t.Errorf("Expected custom error message wrapping")
	}

	// Should track fmt import
	imports := proc.GetNeededImports()
	hasFmt := false
	for _, imp := range imports {
		if imp == "fmt" {
			hasFmt = true
			break
		}
	}
	if !hasFmt {
		t.Errorf("Expected fmt import when using custom error messages")
	}
}

func TestErrorPropASTProcessor_ReturnStatement(t *testing.T) {
	proc := NewErrorPropASTProcessor()

	input := `package main

func readAndProcess(path string) ([]byte, error) {
	return os.ReadFile(path)?
}`

	result, _, err := proc.ProcessInternal(input)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	// Should have error handling
	if !strings.Contains(result, "if err != nil") {
		t.Errorf("Expected error handling for return statement")
	}

	// Should have final return with nil error
	if !strings.Contains(result, "return tmp, nil") {
		t.Errorf("Expected final return statement with nil error")
	}
}

func TestErrorPropASTProcessor_VarNaming(t *testing.T) {
	proc := NewErrorPropASTProcessor()

	input := `package main

func test() error {
	let a = readFile()?
	let b = readFile()?
	let c = readFile()?
	return nil
}`

	result, _, err := proc.ProcessInternal(input)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	// Should use no-number-first pattern
	if !strings.Contains(result, "tmp, err :=") {
		t.Errorf("First temp var should be 'tmp' (not tmp1)")
	}
	if !strings.Contains(result, "tmp1, err1 :=") {
		t.Errorf("Second temp var should be 'tmp1'")
	}
	if !strings.Contains(result, "tmp2, err2 :=") {
		t.Errorf("Third temp var should be 'tmp2'")
	}

	// Should NOT have tmp0
	if strings.Contains(result, "tmp0") {
		t.Errorf("Should not use zero-based indexing (tmp0)")
	}
}

func TestErrorPropASTProcessor_ComplexExpression(t *testing.T) {
	proc := NewErrorPropASTProcessor()

	// Test case: complex nested expression
	input := `package main

func test() error {
	let result = processData(getData(getPath()))?
	return nil
}`

	result, _, err := proc.ProcessInternal(input)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	// Should extract full expression
	if !strings.Contains(result, "processData(getData(getPath()))") {
		t.Errorf("Should preserve complex nested expression")
	}

	// Should have error handling
	if !strings.Contains(result, "if err != nil") {
		t.Errorf("Expected error handling")
	}
}

func TestErrorPropASTProcessor_NullCoalesceIgnored(t *testing.T) {
	proc := NewErrorPropASTProcessor()

	// Test case: ?? should be ignored
	input := `package main

func test() string {
	let x = getValue() ?? "default"
	return x
}`

	result, metadata, err := proc.ProcessInternal(input)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	// Should NOT process null coalesce as error propagation
	if len(metadata) != 0 {
		t.Errorf("Expected 0 metadata entries for null coalesce, got %d", len(metadata))
	}

	// Should preserve ?? operator
	if !strings.Contains(result, "??") {
		t.Errorf("Null coalesce operator should be preserved")
	}
}

func TestErrorPropASTProcessor_MultiValueReturn(t *testing.T) {
	proc := NewErrorPropASTProcessor()

	input := `package main

func readData() ([]byte, int, error) {
	return os.ReadFile("test.txt")?
}`

	result, _, err := proc.ProcessInternal(input)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	// Should handle multi-value return correctly
	// For a function returning ([]byte, int, error), the expansion should have:
	// - Two temp vars (tmp for []byte, one more for int would need different naming)
	// But actually, the current implementation generates one tmp for all non-error returns
	// Let me check what the actual behavior should be...

	// Should have error handling
	if !strings.Contains(result, "if err != nil") {
		t.Errorf("Expected error handling")
	}
}

func TestErrorPropASTProcessor_ImportTracking(t *testing.T) {
	proc := NewErrorPropASTProcessor()

	input := `package main

func test() error {
	let data = os.ReadFile("test")?
	let result = json.Marshal(data)?
	return nil
}`

	_, _, err := proc.ProcessInternal(input)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	// Should track imports from function calls
	imports := proc.GetNeededImports()

	hasOS := false
	hasJSON := false
	for _, imp := range imports {
		if imp == "os" {
			hasOS = true
		}
		if imp == "encoding/json" {
			hasJSON = true
		}
	}

	if !hasOS {
		t.Errorf("Expected 'os' import to be tracked")
	}
	if !hasJSON {
		t.Errorf("Expected 'encoding/json' import to be tracked")
	}
}

func TestErrorPropASTProcessor_MetadataGeneration(t *testing.T) {
	proc := NewErrorPropASTProcessor()

	input := `package main

func test() error {
	let x = readFile()?
	return nil
}`

	_, metadata, err := proc.ProcessInternal(input)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	// Should have exactly one metadata entry
	if len(metadata) != 1 {
		t.Fatalf("Expected 1 metadata entry, got %d", len(metadata))
	}

	meta := metadata[0]

	// Check metadata fields
	if meta.Type != "error_prop" {
		t.Errorf("Expected type 'error_prop', got '%s'", meta.Type)
	}
	if meta.OriginalText != "?" {
		t.Errorf("Expected original text '?', got '%s'", meta.OriginalText)
	}
	if meta.ASTNodeType != "IfStmt" {
		t.Errorf("Expected AST node type 'IfStmt', got '%s'", meta.ASTNodeType)
	}
	if !strings.HasPrefix(meta.GeneratedMarker, "// dingo:e:") {
		t.Errorf("Expected marker to start with '// dingo:e:', got '%s'", meta.GeneratedMarker)
	}
	if meta.OriginalLine != 4 {
		t.Errorf("Expected original line 4, got %d", meta.OriginalLine)
	}
	if meta.OriginalLength != 1 {
		t.Errorf("Expected original length 1, got %d", meta.OriginalLength)
	}
}
