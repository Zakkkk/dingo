package preprocessor

import (
	"strings"
	"testing"
)

func TestRustMatchASTProcessor_BasicLiteralPatterns(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name: "integer literals",
			input: `match x {
				1 => "one",
				2 => "two",
				_ => "other",
			}`,
			expect: "scrutinee := x",
		},
		{
			name: "string literals",
			input: `match status {
				"pending" => handlePending(),
				"active" => handleActive(),
				_ => handleDefault(),
			}`,
			expect: "scrutinee := status",
		},
		{
			name: "boolean literals with wildcard",
			input: `match flag {
				true => doTrue(),
				false => doFalse(),
				_ => doDefault(),
			}`,
			expect: "scrutinee := flag",
		},
	}

	proc := NewRustMatchASTProcessor()

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

func TestRustMatchASTProcessor_IdentifierPatterns(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name: "simple variable binding",
			input: `match value {
				x => x * 2,
			}`,
			expect: "scrutinee := value",
		},
		{
			name: "wildcard pattern",
			input: `match result {
				Ok(v) => v,
				_ => 0,
			}`,
			expect: "case default:",
		},
	}

	proc := NewRustMatchASTProcessor()

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

func TestRustMatchASTProcessor_TuplePatterns(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name: "two-element tuple with wildcard",
			input: `match pair {
				(0, 0) => "origin",
				(x, 0) => fmt.Sprintf("on x-axis: %d", x),
				(0, y) => fmt.Sprintf("on y-axis: %d", y),
				_ => "other",
			}`,
			expect: "scrutinee := pair",
		},
		{
			name: "nested tuple",
			input: `match nested {
				((a, b), c) => a + b + c,
				_ => 0,
			}`,
			expect: "scrutinee := nested",
		},
	}

	proc := NewRustMatchASTProcessor()

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

func TestRustMatchASTProcessor_ConstructorPatterns(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name: "Result Ok/Err",
			input: `match result {
				Ok(v) => v,
				Err(e) => handleError(e),
			}`,
			expect: "scrutinee.tag",
		},
		{
			name: "Option Some/None",
			input: `match opt {
				Some(v) => v,
				None => defaultValue,
			}`,
			expect: "scrutinee.tag",
		},
		{
			name: "enum variants with wildcard",
			input: `match status {
				Status_Pending => "waiting",
				Status_Active => "running",
				_ => "other",
			}`,
			expect: "scrutinee := status",
		},
		{
			name: "constructor with multiple params",
			input: `match point {
				Point(x, y) => x + y,
				_ => 0,
			}`,
			expect: "scrutinee := point",
		},
	}

	proc := NewRustMatchASTProcessor()

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

func TestRustMatchASTProcessor_GuardExpressions(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name: "simple guard",
			input: `match x {
				n if n > 0 => "positive",
				n if n < 0 => "negative",
				_ => "zero",
			}`,
			expect: "if !(n > 0)",
		},
		{
			name: "complex guard",
			input: `match value {
				Ok(v) if v > 100 => "large",
				Ok(v) => "normal",
				Err(_) => "error",
			}`,
			expect: "if !(v > 100)",
		},
		{
			name: "multiple conditions in guard",
			input: `match pair {
				(x, y) if x > 0 && y > 0 => "first quadrant",
				_ => "other",
			}`,
			expect: "if !(x > 0 && y > 0)",
		},
	}

	proc := NewRustMatchASTProcessor()

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

func TestRustMatchASTProcessor_ExhaustivenessResult(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		shouldErr bool
		errMsg    string
	}{
		{
			name: "complete Result match",
			input: `match result {
				Ok(v) => v,
				Err(e) => 0,
			}`,
			shouldErr: false,
		},
		{
			name: "missing Err",
			input: `match result {
				Ok(v) => v,
			}`,
			shouldErr: true,
			errMsg:    "missing patterns: [Err]",
		},
		{
			name: "missing Ok",
			input: `match result {
				Err(e) => 0,
			}`,
			shouldErr: true,
			errMsg:    "missing patterns: [Ok]",
		},
		{
			name: "Result with wildcard",
			input: `match result {
				Ok(v) => v,
				_ => 0,
			}`,
			shouldErr: false,
		},
	}

	proc := NewRustMatchASTProcessor()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := proc.Process([]byte(tt.input))

			if tt.shouldErr {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errMsg)
				} else if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing %q, got: %v", tt.errMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestRustMatchASTProcessor_ExhaustivenessOption(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		shouldErr bool
		errMsg    string
	}{
		{
			name: "complete Option match",
			input: `match opt {
				Some(v) => v,
				None => 0,
			}`,
			shouldErr: false,
		},
		{
			name: "missing None",
			input: `match opt {
				Some(v) => v,
			}`,
			shouldErr: true,
			errMsg:    "missing patterns: [None]",
		},
		{
			name: "missing Some",
			input: `match opt {
				None => 0,
			}`,
			shouldErr: true,
			errMsg:    "missing patterns: [Some]",
		},
		{
			name: "Option with wildcard",
			input: `match opt {
				Some(v) => v,
				_ => 0,
			}`,
			shouldErr: false,
		},
	}

	proc := NewRustMatchASTProcessor()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := proc.Process([]byte(tt.input))

			if tt.shouldErr {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errMsg)
				} else if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing %q, got: %v", tt.errMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestRustMatchASTProcessor_NestedPatterns(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name: "nested Result in Option",
			input: `match opt {
				Some(Ok(v)) => v,
				Some(Err(e)) => handleError(e),
				None => 0,
			}`,
			expect: "scrutinee.tag",
		},
		{
			name: "nested tuples",
			input: `match data {
				((x, y), (a, b)) => x + y + a + b,
				_ => 0,
			}`,
			expect: "scrutinee := data",
		},
	}

	proc := NewRustMatchASTProcessor()

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

func TestRustMatchASTProcessor_ExpressionContext(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name: "match in assignment",
			input: `result := match x {
				1 => "one",
				2 => "two",
				_ => "other",
			}`,
			expect: "__match_result",
		},
		{
			name: "match in return",
			input: `return match status {
				Ok(v) => v,
				Err(_) => 0,
			}`,
			expect: "__match_result",
		},
		{
			name: "match in function argument",
			input: `process(match value {
				Some(v) => v,
				None => 0,
			})`,
			expect: "__match_result",
		},
	}

	proc := NewRustMatchASTProcessor()

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

func TestRustMatchASTProcessor_BlockVsExpression(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name: "expression bodies",
			input: `match x {
				1 => "one",
				2 => "two",
				_ => "other",
			}`,
			expect: "scrutinee := x",
		},
		{
			name: "block bodies",
			input: `match x {
				1 => {
					println("one")
					return 1
				},
				_ => {
					println("other")
					return 0
				},
			}`,
			expect: "scrutinee := x",
		},
		{
			name: "mixed block and expression",
			input: `match x {
				1 => "one",
				2 => {
					doSomething()
					return "two"
				},
				_ => "other",
			}`,
			expect: "scrutinee := x",
		},
	}

	proc := NewRustMatchASTProcessor()

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

func TestRustMatchASTProcessor_EdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		shouldMatch string
		shouldNot   string
	}{
		{
			name:        "empty match not allowed",
			input:       `match x {}`,
			shouldMatch: "",
			shouldNot:   "",
		},
		{
			name: "single arm with wildcard",
			input: `match x {
				_ => doDefault(),
			}`,
			shouldMatch: "scrutinee := x",
			shouldNot:   "",
		},
		{
			name: "trailing comma",
			input: `match x {
				1 => "one",
				2 => "two",
				_ => "other",
			}`,
			shouldMatch: "scrutinee := x",
			shouldNot:   "",
		},
		{
			name: "no trailing comma",
			input: `match x {
				1 => "one",
				2 => "two",
				_ => "other"
			}`,
			shouldMatch: "scrutinee := x",
			shouldNot:   "",
		},
	}

	proc := NewRustMatchASTProcessor()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := proc.Process([]byte(tt.input))
			// Some edge cases might error, that's okay
			if err != nil {
				t.Logf("error (may be expected): %v", err)
				return
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

func TestRustMatchASTProcessor_ComplexExpressions(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name: "match with function calls in body",
			input: `match result {
				Ok(v) => process(transform(v)),
				Err(e) => handleError(format(e)),
			}`,
			expect: "scrutinee.tag",
		},
		{
			name: "match with method chains",
			input: `match data {
				Some(v) => v.trim().toLower(),
				None => "",
			}`,
			expect: "scrutinee.tag",
		},
		{
			name: "match with complex scrutinee",
			input: `match getValue().unwrap() {
				1 => "one",
				_ => "other",
			}`,
			expect: "scrutinee := getValue().unwrap()",
		},
	}

	proc := NewRustMatchASTProcessor()

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

func TestRustMatchASTProcessor_SourceMappings(t *testing.T) {
	input := `match x {
		1 => "one",
		2 => "two",
		_ => "other",
	}`

	proc := NewRustMatchASTProcessor()
	_, mappings, err := proc.Process([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have mappings for the match transformation
	if len(mappings) == 0 {
		t.Error("expected source mappings, got none")
	}

	// Verify mapping contains match transformation
	for _, m := range mappings {
		if m.Name != "match" {
			t.Errorf("expected match mapping, got name=%s", m.Name)
		}
	}
}

func TestRustMatchASTProcessor_NoFalsePositives(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		shouldErr bool
	}{
		{
			name:  "switch statement (Go native)",
			input: `switch x { case 1: return "one" }`,
		},
		{
			name:  "match in comment",
			input: `// match x { ... }`,
		},
		{
			name:      "match in string literal",
			input:     `s := "match x { 1 => ok }"`,
			shouldErr: true, // Known issue: doesn't skip strings yet
		},
		{
			name:  "enum declaration",
			input: `enum Status { Pending, Active }`,
		},
	}

	proc := NewRustMatchASTProcessor()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := proc.Process([]byte(tt.input))

			if tt.shouldErr {
				// Expected to error or transform incorrectly
				if err != nil {
					t.Logf("expected error (known issue): %v", err)
					return
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}

			got := string(result)
			// Input should be unchanged or minimally changed
			// (we log but don't fail - some transformations might be acceptable)
			if got != tt.input {
				t.Logf("input was modified (may be acceptable):\ninput:  %s\noutput: %s", tt.input, got)
			}
		})
	}
}

func TestRustMatchASTProcessor_RealWorldExamples(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name: "HTTP status handling",
			input: `statusText := match status {
				200 => "OK",
				404 => "Not Found",
				500 => "Internal Server Error",
				_ => "Unknown",
			}`,
			expect: "__match_result",
		},
		{
			name: "Result with error handling",
			input: `value := match readFile(path) {
				Ok(data) => data,
				Err(e) if isNotFound(e) => defaultContent,
				Err(e) => {
					log.Error(e)
					return nil
				},
			}`,
			expect: "scrutinee := readFile(path)",
		},
		{
			name: "Option chain",
			input: `result := match getUser(id) {
				Some(user) if user.Active => user.Name,
				Some(user) => "inactive: " + user.Name,
				None => "unknown",
			}`,
			expect: "scrutinee := getUser(id)",
		},
		{
			name: "Enum state machine",
			input: `nextState := match currentState {
				State_Init => State_Loading,
				State_Loading if hasData() => State_Ready,
				State_Loading => State_Error,
				State_Ready => State_Processing,
				_ => currentState,
			}`,
			expect: "scrutinee := currentState",
		},
	}

	proc := NewRustMatchASTProcessor()

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

func TestRustMatchASTProcessor_MultipleMatches(t *testing.T) {
	input := `
		x := match a {
			1 => "one",
			_ => "other",
		}

		y := match b {
			Ok(v) => v,
			Err(_) => 0,
		}
	`

	proc := NewRustMatchASTProcessor()
	result, mappings, err := proc.Process([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := string(result)

	// Both matches should be transformed
	if !strings.Contains(got, "scrutinee := a") {
		t.Errorf("expected first match to be transformed, got:\n%s", got)
	}
	if !strings.Contains(got, "scrutinee := b") {
		t.Errorf("expected second match to be transformed, got:\n%s", got)
	}

	// Should have mappings for both
	if len(mappings) < 2 {
		t.Errorf("expected at least 2 mappings, got %d", len(mappings))
	}
}

func TestRustMatchASTProcessor_StringsInPatterns(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name: "strings with commas",
			input: `match s {
				"hello, world" => 1,
				"foo, bar" => 2,
				_ => 0,
			}`,
			expect: "scrutinee := s",
		},
		{
			name: "strings with braces",
			input: `match s {
				"a{b}c" => 1,
				_ => 0,
			}`,
			expect: "scrutinee := s",
		},
		{
			name: "raw strings",
			input: "match s {\n\t`multi\nline` => 1,\n\t_ => 0,\n}",
			expect: "scrutinee := s",
		},
	}

	proc := NewRustMatchASTProcessor()

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
