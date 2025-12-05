package preprocessor

import (
	"strings"
	"testing"
)

func TestMatchProcessor_CommentsInArms(t *testing.T) {
	// P0 Bug Fix: Comments after match arms should not break parsing
	input := `match event {
	Click(x, y) => handleClick(x, y),  // Click events
	Scroll(delta) => handleScroll(delta),  // Scroll events
	_ => {}
}`

	proc := NewMatchProcessor()
	output, _, err := proc.Process([]byte(input))
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	result := string(output)

	// Should generate valid Go code with switch
	if !strings.Contains(result, "switch") {
		t.Errorf("Expected switch statement in output")
	}

	// Should preserve comment intent (may be reformatted)
	if !strings.Contains(result, "Click") && !strings.Contains(result, "Scroll") {
		t.Errorf("Expected comments or their context to be preserved")
	}

	// Should not have parse errors
	if strings.Contains(result, "unexpected") || strings.Contains(result, "error") {
		t.Errorf("Output contains error indicators: %s", result)
	}
}

func TestMatchProcessor_NestedPatterns(t *testing.T) {
	// P0 Feature: Nested patterns like Ok(Some(x)) should work
	input := `match wrapped {
	Ok(Some(value)) => process(value),
	Ok(None) => useDefault(),
	Err(e) => handleError(e)
}`

	proc := NewMatchProcessor()
	output, _, err := proc.Process([]byte(input))
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	result := string(output)

	// Should generate nested switches for Ok(Some(x))
	if !strings.Contains(result, "switch") {
		t.Errorf("Expected switch statement in output")
	}

	// Should handle Result tags
	if !strings.Contains(result, "ResultTagOk") || !strings.Contains(result, "ResultTagErr") {
		t.Errorf("Expected Result tag constants in output")
	}

	// Should handle Option tags for nested Some/None
	if !strings.Contains(result, "OptionTagSome") || !strings.Contains(result, "OptionTagNone") {
		t.Errorf("Expected Option tag constants for nested pattern")
	}
}

func TestMatchProcessor_SimplePattern(t *testing.T) {
	input := `match result {
	Ok(x) => x,
	Err(e) => 0
}`

	proc := NewMatchProcessor()
	output, _, err := proc.Process([]byte(input))
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	result := string(output)

	// Should generate switch with Result tags
	if !strings.Contains(result, "switch") {
		t.Errorf("Expected switch statement")
	}
	if !strings.Contains(result, "ResultTagOk") || !strings.Contains(result, "ResultTagErr") {
		t.Errorf("Expected Result tag constants")
	}

	// Should extract bindings (x, e)
	if !strings.Contains(result, "x :=") || !strings.Contains(result, "e :=") {
		t.Errorf("Expected variable bindings for x and e")
	}
}

func TestMatchProcessor_Guards(t *testing.T) {
	input := `match value {
	Ok(x) if x > 10 => "high",
	Ok(x) if x > 0 => "positive",
	_ => "other"
}`

	proc := NewMatchProcessor()
	output, _, err := proc.Process([]byte(input))
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	result := string(output)

	// Debug: print output
	t.Logf("Generated code:\n%s", result)

	// Should generate guard checks (inside conditional blocks)
	// Guards are generated as if !(condition) { break }
	if !strings.Contains(result, "x > 10") && !strings.Contains(result, "x > 0") {
		t.Errorf("Expected guard conditions in output")
	}
}

func TestMatchProcessor_BlockBody(t *testing.T) {
	input := `match status {
	Active => {
		log("Active")
		return true
	},
	Inactive => {
		log("Inactive")
		return false
	}
}`

	proc := NewMatchProcessor()
	output, _, err := proc.Process([]byte(input))
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	result := string(output)

	// Should preserve block structure
	if !strings.Contains(result, "log") {
		t.Errorf("Expected block body code preserved")
	}
}

func TestMatchProcessor_NoMatch(t *testing.T) {
	// No match expressions - should return unchanged
	input := `func foo() {
	x := 42
	return x
}`

	proc := NewMatchProcessor()
	output, _, err := proc.Process([]byte(input))
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	if string(output) != input {
		t.Errorf("Expected unchanged output for non-match code")
	}
}

func TestMatchProcessor_MultipleMatches(t *testing.T) {
	input := `
func process() {
	x := match opt {
		Some(v) => v,
		None => 0
	}

	y := match result {
		Ok(val) => val,
		Err(e) => -1
	}
}
`

	proc := NewMatchProcessor()
	output, _, err := proc.Process([]byte(input))
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	result := string(output)

	// Should process both match expressions
	if !strings.Contains(result, "OptionTagSome") {
		t.Errorf("Expected first match (Option) to be processed")
	}
	if !strings.Contains(result, "ResultTagOk") {
		t.Errorf("Expected second match (Result) to be processed")
	}
}

func TestMatchProcessor_DeepNesting(t *testing.T) {
	// Deep nesting: Err(Error(code, msg))
	input := `match result {
	Ok(Success(value)) => value,
	Err(Error(code, msg)) => handleError(code, msg)
}`

	proc := NewMatchProcessor()
	output, _, err := proc.Process([]byte(input))
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	result := string(output)

	// Should generate nested switches
	if !strings.Contains(result, "switch") {
		t.Errorf("Expected switch statements for nested patterns")
	}
}

func TestMatchProcessor_TuplePattern(t *testing.T) {
	input := `match pair {
	(Ok(x), Ok(y)) => x + y,
	_ => 0
}`

	proc := NewMatchProcessor()
	output, _, err := proc.Process([]byte(input))
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	result := string(output)

	// Should handle tuple patterns (even if simplified in this phase)
	if !strings.Contains(result, "switch") {
		t.Errorf("Expected switch statement")
	}
}

func TestMatchProcessor_Wildcard(t *testing.T) {
	input := `match x {
	1 => "one",
	2 => "two",
	_ => "other"
}`

	proc := NewMatchProcessor()
	output, _, err := proc.Process([]byte(input))
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	result := string(output)

	// Should generate default case for wildcard
	if !strings.Contains(result, "default") {
		t.Errorf("Expected default case for wildcard pattern")
	}
}
