package preprocessor

import (
	"strings"
	"testing"
)

func TestNullCoalesceASTProcessor_SimpleIdentifier(t *testing.T) {
	processor := NewNullCoalesceASTProcessor()

	source := `let x = value ?? "default"`
	result, _, err := processor.ProcessInternal(source)
	if err != nil {
		t.Fatalf("ProcessInternal failed: %v", err)
	}

	// Should use default-first pattern
	if !strings.Contains(result, `x := "default"`) {
		t.Errorf("Expected default-first assignment, got: %s", result)
	}

	// Should check and reassign if not nil
	if !strings.Contains(result, "if val := value;") {
		t.Errorf("Expected val assignment, got: %s", result)
	}

	// Should have dingo marker
	if !strings.Contains(result, "// dingo:c:") {
		t.Errorf("Expected dingo marker, got: %s", result)
	}
}

func TestNullCoalesceASTProcessor_ChainedCoalesce(t *testing.T) {
	processor := NewNullCoalesceASTProcessor()

	source := `let x = value ?? fallback ?? "default"`
	result, _, err := processor.ProcessInternal(source)
	if err != nil {
		t.Fatalf("ProcessInternal failed: %v", err)
	}

	// Should use default-first pattern for rightmost default
	if !strings.Contains(result, `"default"`) {
		t.Errorf("Expected default value, got: %s", result)
	}

	// Should check value
	if !strings.Contains(result, "value") {
		t.Errorf("Expected value check, got: %s", result)
	}

	// Should check fallback
	if !strings.Contains(result, "fallback") {
		t.Errorf("Expected fallback check, got: %s", result)
	}

	// Should have nested if checks
	if !strings.Contains(result, "if val :=") {
		t.Errorf("Expected if val checks, got: %s", result)
	}
}

func TestNullCoalesceASTProcessor_ComplexLeft(t *testing.T) {
	processor := NewNullCoalesceASTProcessor()

	source := `let x = getValue() ?? "default"`
	result, _, err := processor.ProcessInternal(source)
	if err != nil {
		t.Fatalf("ProcessInternal failed: %v", err)
	}

	// Should use default-first pattern
	if !strings.Contains(result, `x := "default"`) {
		t.Errorf("Expected default-first assignment, got: %s", result)
	}

	// Should call getValue() in condition
	if !strings.Contains(result, "getValue()") {
		t.Errorf("Expected getValue() call, got: %s", result)
	}

	// Should have if check
	if !strings.Contains(result, "if val :=") {
		t.Errorf("Expected if val check, got: %s", result)
	}
}

func TestNullCoalesceASTProcessor_OptionType(t *testing.T) {
	processor := NewNullCoalesceASTProcessor()

	// Test with Option type variable
	source := `let opt: StringOption = getOpt()
let x = opt ?? "fallback"`
	result, _, err := processor.ProcessInternal(source)
	if err != nil {
		t.Fatalf("ProcessInternal failed: %v", err)
	}

	// Should check IsSome() for Option type
	if !strings.Contains(result, "IsSome()") {
		t.Errorf("Expected IsSome() check, got: %s", result)
	}

	// Should unwrap value
	if !strings.Contains(result, "Unwrap()") {
		t.Errorf("Expected Unwrap() call, got: %s", result)
	}
}

func TestNullCoalesceASTProcessor_NoOperator(t *testing.T) {
	processor := NewNullCoalesceASTProcessor()

	source := `let x = "no coalesce here"`
	result, _, err := processor.ProcessInternal(source)
	if err != nil {
		t.Fatalf("ProcessInternal failed: %v", err)
	}

	// Should return unchanged
	if result != source {
		t.Errorf("Expected unchanged output, got: %s", result)
	}
}

func TestNullCoalesceASTProcessor_CommentIgnored(t *testing.T) {
	processor := NewNullCoalesceASTProcessor()

	source := `let x = value // comment with ?? in it`
	result, _, err := processor.ProcessInternal(source)
	if err != nil {
		t.Fatalf("ProcessInternal failed: %v", err)
	}

	// Should not transform ?? in comment
	if !strings.Contains(result, "// comment with ?? in it") {
		t.Errorf("Expected comment preserved, got: %s", result)
	}
}

func TestNullCoalesceASTProcessor_StringLiteralIgnored(t *testing.T) {
	processor := NewNullCoalesceASTProcessor()

	source := `let x = value ?? "string with ?? in it"`
	result, _, err := processor.ProcessInternal(source)
	if err != nil {
		t.Fatalf("ProcessInternal failed: %v", err)
	}

	// Should preserve ?? in string literal
	if !strings.Contains(result, `"string with ?? in it"`) {
		t.Errorf("Expected string literal preserved, got: %s", result)
	}

	// Should still transform the actual ?? operator
	if !strings.Contains(result, "if val :=") {
		t.Errorf("Expected transformation of ?? operator, got: %s", result)
	}
}

func TestNullCoalesceASTProcessor_Metadata(t *testing.T) {
	processor := NewNullCoalesceASTProcessor()

	source := `let x = value ?? "default"`
	_, metadata, err := processor.ProcessInternal(source)
	if err != nil {
		t.Fatalf("ProcessInternal failed: %v", err)
	}

	// Should generate metadata
	if len(metadata) != 1 {
		t.Fatalf("Expected 1 metadata entry, got: %d", len(metadata))
	}

	meta := metadata[0]
	if meta.Type != "null_coalesce" {
		t.Errorf("Expected type 'null_coalesce', got: %s", meta.Type)
	}

	if meta.OriginalText != "??" {
		t.Errorf("Expected original text '??', got: %s", meta.OriginalText)
	}

	if meta.ASTNodeType != "NullCoalesceExpr" {
		t.Errorf("Expected ASTNodeType 'NullCoalesceExpr', got: %s", meta.ASTNodeType)
	}

	if !strings.HasPrefix(meta.GeneratedMarker, "// dingo:c:") {
		t.Errorf("Expected marker starting with '// dingo:c:', got: %s", meta.GeneratedMarker)
	}
}

// Edge Case Tests

func TestNullCoalesceASTProcessor_LongChain(t *testing.T) {
	processor := NewNullCoalesceASTProcessor()

	// Test long chain: a ?? b ?? c ?? d
	source := `let result = primary ?? secondary ?? tertiary ?? "final"`
	result, _, err := processor.ProcessInternal(source)
	if err != nil {
		t.Fatalf("ProcessInternal failed: %v", err)
	}

	// Should contain all four values
	if !strings.Contains(result, "primary") {
		t.Errorf("Expected 'primary' in output, got: %s", result)
	}
	if !strings.Contains(result, "secondary") {
		t.Errorf("Expected 'secondary' in output, got: %s", result)
	}
	if !strings.Contains(result, "tertiary") {
		t.Errorf("Expected 'tertiary' in output, got: %s", result)
	}
	if !strings.Contains(result, `"final"`) {
		t.Errorf("Expected 'final' in output, got: %s", result)
	}

	// Should have nested if statements
	ifCount := strings.Count(result, "if val")
	if ifCount < 2 {
		t.Errorf("Expected at least 2 'if val' checks for chain, got: %d", ifCount)
	}
}

func TestNullCoalesceASTProcessor_MethodCallChain(t *testing.T) {
	processor := NewNullCoalesceASTProcessor()

	// Test method call chain
	source := `let x = obj.GetValue() ?? "default"`
	result, _, err := processor.ProcessInternal(source)
	if err != nil {
		t.Fatalf("ProcessInternal failed: %v", err)
	}

	// Should preserve method call
	if !strings.Contains(result, "obj.GetValue()") {
		t.Errorf("Expected 'obj.GetValue()' preserved, got: %s", result)
	}

	// Should use default-first pattern
	if !strings.Contains(result, `x := "default"`) {
		t.Errorf("Expected default-first assignment, got: %s", result)
	}

	// Should have if check
	if !strings.Contains(result, "if val :=") {
		t.Errorf("Expected if val check, got: %s", result)
	}
}

func TestNullCoalesceASTProcessor_MultipleMethodCalls(t *testing.T) {
	processor := NewNullCoalesceASTProcessor()

	// Test chained methods with coalesce
	source := `let x = obj.Method1().Method2() ?? "default"`
	result, _, err := processor.ProcessInternal(source)
	if err != nil {
		t.Fatalf("ProcessInternal failed: %v", err)
	}

	// Should preserve full method chain
	if !strings.Contains(result, "obj.Method1().Method2()") {
		t.Errorf("Expected 'obj.Method1().Method2()' preserved, got: %s", result)
	}

	// Should transform to default-first
	if !strings.Contains(result, `x := "default"`) {
		t.Errorf("Expected default-first assignment, got: %s", result)
	}
}

func TestNullCoalesceASTProcessor_ComplexLHSExpression(t *testing.T) {
	processor := NewNullCoalesceASTProcessor()

	// Test complex expression on left: (a + b) ?? c
	source := `let x = (a + b) ?? fallback`
	result, _, err := processor.ProcessInternal(source)
	if err != nil {
		t.Fatalf("ProcessInternal failed: %v", err)
	}

	// Should preserve expression
	if !strings.Contains(result, "(a + b)") {
		t.Errorf("Expected '(a + b)' preserved, got: %s", result)
	}

	// Should have fallback
	if !strings.Contains(result, "fallback") {
		t.Errorf("Expected 'fallback' in output, got: %s", result)
	}

	// Should have if check
	if !strings.Contains(result, "if val :=") {
		t.Errorf("Expected if val check, got: %s", result)
	}
}

func TestNullCoalesceASTProcessor_CombinedWithSafeNav(t *testing.T) {
	processor := NewNullCoalesceASTProcessor()

	// Test safe navigation + null coalesce: a?.b ?? c
	source := `let x = user?.name ?? "unknown"`
	result, _, err := processor.ProcessInternal(source)
	if err != nil {
		t.Fatalf("ProcessInternal failed: %v", err)
	}

	// Should preserve safe navigation
	if !strings.Contains(result, "user?.name") {
		t.Errorf("Expected 'user?.name' preserved, got: %s", result)
	}

	// Should have default value
	if !strings.Contains(result, `"unknown"`) {
		t.Errorf("Expected 'unknown' in output, got: %s", result)
	}

	// Should transform to default-first
	if !strings.Contains(result, "x :=") {
		t.Errorf("Expected assignment, got: %s", result)
	}
}

func TestNullCoalesceASTProcessor_NestedFunctionCalls(t *testing.T) {
	processor := NewNullCoalesceASTProcessor()

	// Test nested functions: f(g(x)) ?? default
	source := `let result = formatValue(getValue(key)) ?? "error"`
	result, _, err := processor.ProcessInternal(source)
	if err != nil {
		t.Fatalf("ProcessInternal failed: %v", err)
	}

	// Should preserve nested calls
	if !strings.Contains(result, "formatValue(getValue(key))") {
		t.Errorf("Expected 'formatValue(getValue(key))' preserved, got: %s", result)
	}

	// Should use default-first pattern
	if !strings.Contains(result, `result := "error"`) {
		t.Errorf("Expected default-first assignment, got: %s", result)
	}
}

func TestNullCoalesceASTProcessor_IIFEPatternBasic(t *testing.T) {
	processor := NewNullCoalesceASTProcessor()

	// Test in IIFE pattern context
	source := `let x = func() string { return value ?? "default" }()`
	result, _, err := processor.ProcessInternal(source)
	if err != nil {
		t.Fatalf("ProcessInternal failed: %v", err)
	}

	// Should transform ?? inside IIFE
	if !strings.Contains(result, "if val :=") {
		t.Errorf("Expected transformation inside IIFE, got: %s", result)
	}

	// Should preserve IIFE structure
	if !strings.Contains(result, "func()") {
		t.Errorf("Expected IIFE structure preserved, got: %s", result)
	}
}

func TestNullCoalesceASTProcessor_ArrayIndexAccess(t *testing.T) {
	processor := NewNullCoalesceASTProcessor()

	// Test array/slice index access: arr[i] ?? default
	source := `let x = arr[index] ?? "empty"`
	result, _, err := processor.ProcessInternal(source)
	if err != nil {
		t.Fatalf("ProcessInternal failed: %v", err)
	}

	// Should preserve array access
	if !strings.Contains(result, "arr[index]") {
		t.Errorf("Expected 'arr[index]' preserved, got: %s", result)
	}

	// Should use default-first pattern
	if !strings.Contains(result, `x := "empty"`) {
		t.Errorf("Expected default-first assignment, got: %s", result)
	}
}

func TestNullCoalesceASTProcessor_MapAccess(t *testing.T) {
	processor := NewNullCoalesceASTProcessor()

	// Test map access: m["key"] ?? default
	source := `let val = config["timeout"] ?? 30`
	result, _, err := processor.ProcessInternal(source)
	if err != nil {
		t.Fatalf("ProcessInternal failed: %v", err)
	}

	// Should preserve map access
	if !strings.Contains(result, `config["timeout"]`) {
		t.Errorf("Expected 'config[\"timeout\"]' preserved, got: %s", result)
	}

	// Should have default value
	if !strings.Contains(result, "30") {
		t.Errorf("Expected '30' in output, got: %s", result)
	}
}

func TestNullCoalesceASTProcessor_TypeAssertionLHS(t *testing.T) {
	processor := NewNullCoalesceASTProcessor()

	// Test type assertion: val.(string) ?? "default"
	source := `let x = anyVal.(string) ?? "fallback"`
	result, _, err := processor.ProcessInternal(source)
	if err != nil {
		t.Fatalf("ProcessInternal failed: %v", err)
	}

	// Should preserve type assertion
	if !strings.Contains(result, "anyVal.(string)") {
		t.Errorf("Expected 'anyVal.(string)' preserved, got: %s", result)
	}

	// Should use default-first pattern
	if !strings.Contains(result, `x := "fallback"`) {
		t.Errorf("Expected default-first assignment, got: %s", result)
	}
}

func TestNullCoalesceASTProcessor_BinaryExpressionLHS(t *testing.T) {
	processor := NewNullCoalesceASTProcessor()

	// Test binary expression: a == b ?? c
	source := `let match = (a == b) ?? false`
	result, _, err := processor.ProcessInternal(source)
	if err != nil {
		t.Fatalf("ProcessInternal failed: %v", err)
	}

	// Should preserve binary expression
	if !strings.Contains(result, "(a == b)") {
		t.Errorf("Expected '(a == b)' preserved, got: %s", result)
	}

	// Should have default value
	if !strings.Contains(result, "false") {
		t.Errorf("Expected 'false' in output, got: %s", result)
	}
}

func TestNullCoalesceASTProcessor_MultipleCoalesceInOneLine(t *testing.T) {
	processor := NewNullCoalesceASTProcessor()

	// Test multiple ?? on same line (separate expressions)
	source := `let x = a ?? "x"; let y = b ?? "y"`
	result, _, err := processor.ProcessInternal(source)
	if err != nil {
		t.Fatalf("ProcessInternal failed: %v", err)
	}

	// Should transform both
	xCount := strings.Count(result, `x := "x"`)
	yCount := strings.Count(result, `y := "y"`)

	if xCount < 1 {
		t.Errorf("Expected x transformation, got: %s", result)
	}

	if yCount < 1 {
		t.Errorf("Expected y transformation, got: %s", result)
	}

	// Should have dingo markers for both
	markerCount := strings.Count(result, "// dingo:c:")
	if markerCount < 2 {
		t.Errorf("Expected at least 2 markers, got: %d", markerCount)
	}
}

func TestNullCoalesceASTProcessor_NumericDefault(t *testing.T) {
	processor := NewNullCoalesceASTProcessor()

	// Test numeric default value
	source := `let x = count ?? 0`
	result, _, err := processor.ProcessInternal(source)
	if err != nil {
		t.Fatalf("ProcessInternal failed: %v", err)
	}

	// Should use default-first pattern with numeric value
	if !strings.Contains(result, "x := 0") {
		t.Errorf("Expected 'x := 0', got: %s", result)
	}

	// Should check count
	if !strings.Contains(result, "count") {
		t.Errorf("Expected 'count' in output, got: %s", result)
	}
}

func TestNullCoalesceASTProcessor_BooleanDefault(t *testing.T) {
	processor := NewNullCoalesceASTProcessor()

	// Test boolean default value
	source := `let active = status ?? true`
	result, _, err := processor.ProcessInternal(source)
	if err != nil {
		t.Fatalf("ProcessInternal failed: %v", err)
	}

	// Should use default-first pattern with boolean
	if !strings.Contains(result, "active := true") {
		t.Errorf("Expected 'active := true', got: %s", result)
	}

	// Should check status
	if !strings.Contains(result, "status") {
		t.Errorf("Expected 'status' in output, got: %s", result)
	}
}

// Statement Splitting Tests

func TestNullCoalesceASTProcessor_SplitStatements_Basic(t *testing.T) {
	processor := NewNullCoalesceASTProcessor()

	// Test basic semicolon split with let (generates if-else blocks)
	source := `let a = x ?? 0; let b = y ?? 1`
	result, _, err := processor.ProcessInternal(source)
	if err != nil {
		t.Fatalf("ProcessInternal failed: %v", err)
	}

	// Should contain both transformations
	if !strings.Contains(result, "a := 0") {
		t.Errorf("Expected 'a := 0' in output, got: %s", result)
	}
	if !strings.Contains(result, "b := 1") {
		t.Errorf("Expected 'b := 1' in output, got: %s", result)
	}

	// Should preserve semicolon separator
	if !strings.Contains(result, ";") {
		t.Errorf("Expected semicolon separator, got: %s", result)
	}
}

func TestNullCoalesceASTProcessor_SplitStatements_ThreeStatements(t *testing.T) {
	processor := NewNullCoalesceASTProcessor()

	// Test three statements on one line with let
	source := `let a = x ?? 0; let b = y ?? 1; let c = z ?? 2`
	result, _, err := processor.ProcessInternal(source)
	if err != nil {
		t.Fatalf("ProcessInternal failed: %v", err)
	}

	// Should contain all three transformations
	if !strings.Contains(result, "a := 0") {
		t.Errorf("Expected 'a := 0' in output, got: %s", result)
	}
	if !strings.Contains(result, "b := 1") {
		t.Errorf("Expected 'b := 1' in output, got: %s", result)
	}
	if !strings.Contains(result, "c := 2") {
		t.Errorf("Expected 'c := 2' in output, got: %s", result)
	}

	// Should have at least 3 markers
	markerCount := strings.Count(result, "// dingo:c:")
	if markerCount < 3 {
		t.Errorf("Expected at least 3 markers, got: %d", markerCount)
	}
}

func TestNullCoalesceASTProcessor_SplitStatements_SemicolonInString(t *testing.T) {
	processor := NewNullCoalesceASTProcessor()

	// Test semicolon inside string literal (should NOT split) with let
	source := `let a = x ?? "a;b"; let b = y ?? "c"`
	result, _, err := processor.ProcessInternal(source)
	if err != nil {
		t.Fatalf("ProcessInternal failed: %v", err)
	}

	// Should preserve semicolon in string
	if !strings.Contains(result, `"a;b"`) {
		t.Errorf("Expected 'a;b' preserved in string, got: %s", result)
	}

	// Should split on semicolon outside string
	if !strings.Contains(result, `a := "a;b"`) {
		t.Errorf("Expected 'a :=' transformation, got: %s", result)
	}
	if !strings.Contains(result, `b := "c"`) {
		t.Errorf("Expected 'b :=' transformation, got: %s", result)
	}
}

func TestNullCoalesceASTProcessor_SplitStatements_SemicolonInComment(t *testing.T) {
	processor := NewNullCoalesceASTProcessor()

	// Test semicolon in comment (should NOT split) with let
	source := `let a = x ?? 0 // comment; with semicolon`
	result, _, err := processor.ProcessInternal(source)
	if err != nil {
		t.Fatalf("ProcessInternal failed: %v", err)
	}

	// Should transform the ?? operator (comment is stripped during parsing)
	if !strings.Contains(result, "a := 0") {
		t.Errorf("Expected 'a := 0' transformation, got: %s", result)
	}

	// Should not split on semicolon inside comment
	// (only one transformation should occur)
	markerCount := strings.Count(result, "// dingo:c:")
	if markerCount != 1 {
		t.Errorf("Expected exactly 1 marker (no split), got: %d", markerCount)
	}
}

func TestNullCoalesceASTProcessor_SplitStatements_RawString(t *testing.T) {
	processor := NewNullCoalesceASTProcessor()

	// Test semicolon in raw string (backticks) with let
	source := "let a = x ?? `raw;string`; let b = y ?? 2"
	result, _, err := processor.ProcessInternal(source)
	if err != nil {
		t.Fatalf("ProcessInternal failed: %v", err)
	}

	// Should preserve semicolon in raw string
	if !strings.Contains(result, "`raw;string`") {
		t.Errorf("Expected raw string preserved, got: %s", result)
	}

	// Should split on semicolon outside string
	if !strings.Contains(result, "a := `raw;string`") {
		t.Errorf("Expected 'a :=' transformation, got: %s", result)
	}
	if !strings.Contains(result, "b := 2") {
		t.Errorf("Expected 'b :=' transformation, got: %s", result)
	}
}

func TestNullCoalesceASTProcessor_SplitStatements_EscapedQuote(t *testing.T) {
	processor := NewNullCoalesceASTProcessor()

	// Test escaped quote in string with let
	source := `let a = x ?? "test\"quote;here"; let b = y ?? 3`
	result, _, err := processor.ProcessInternal(source)
	if err != nil {
		t.Fatalf("ProcessInternal failed: %v", err)
	}

	// Should preserve escaped quote and semicolon in string
	if !strings.Contains(result, `"test\"quote;here"`) {
		t.Errorf("Expected escaped string preserved, got: %s", result)
	}

	// Should split on semicolon outside string
	if !strings.Contains(result, `a := "test\"quote;here"`) {
		t.Errorf("Expected 'a :=' transformation, got: %s", result)
	}
	if !strings.Contains(result, "b := 3") {
		t.Errorf("Expected 'b :=' transformation, got: %s", result)
	}
}

func TestNullCoalesceASTProcessor_SplitStatements_MixedWithNonCoalesce(t *testing.T) {
	processor := NewNullCoalesceASTProcessor()

	// Test mix of ?? and non-?? statements with let
	source := `let a = x ?? 0; let b = 42; let c = y ?? 1`
	result, _, err := processor.ProcessInternal(source)
	if err != nil {
		t.Fatalf("ProcessInternal failed: %v", err)
	}

	// Should contain all statements
	if !strings.Contains(result, "a := 0") {
		t.Errorf("Expected 'a := 0' in output, got: %s", result)
	}
	if !strings.Contains(result, "let b = 42") {
		t.Errorf("Expected 'let b = 42' in output, got: %s", result)
	}
	if !strings.Contains(result, "c := 1") {
		t.Errorf("Expected 'c := 1' in output, got: %s", result)
	}

	// Should have markers for transformed statements only
	markerCount := strings.Count(result, "// dingo:c:")
	if markerCount < 2 {
		t.Errorf("Expected at least 2 markers, got: %d", markerCount)
	}
}

func TestNullCoalesceASTProcessor_SplitStatements_NoSemicolon(t *testing.T) {
	processor := NewNullCoalesceASTProcessor()

	// Test single statement (no semicolon) should work as before with let
	source := `let a = x ?? 0`
	result, _, err := processor.ProcessInternal(source)
	if err != nil {
		t.Fatalf("ProcessInternal failed: %v", err)
	}

	// Should transform normally
	if !strings.Contains(result, "a := 0") {
		t.Errorf("Expected 'a := 0' transformation, got: %s", result)
	}

	// Should have marker
	if !strings.Contains(result, "// dingo:c:") {
		t.Errorf("Expected marker, got: %s", result)
	}
}
