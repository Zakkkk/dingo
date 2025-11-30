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
