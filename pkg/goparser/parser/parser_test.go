package parser

import (
	"go/token"
	"strings"
	"testing"
)

func TestTransformTypeAnnotations(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple function with type annotations",
			input:    "func foo(x: int, y: string) int { return x }",
			expected: "func foo(x int, y string) int { return x }",
		},
		{
			name:     "function without annotations (should pass through)",
			input:    "func foo(x int) int { return x }",
			expected: "func foo(x int) int { return x }",
		},
		{
			name:     "method with type annotations",
			input:    "func (s: *Server) Handle(w: http.ResponseWriter, r: *http.Request) {}",
			expected: "func (s *Server) Handle(w http.ResponseWriter, r *http.Request) {}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := TransformToGo([]byte(tt.input))
			if err != nil {
				t.Fatalf("TransformToGo failed: %v", err)
			}

			// Normalize whitespace for comparison
			got := normalizeWhitespace(string(result))
			want := normalizeWhitespace(tt.expected)

			if got != want {
				t.Errorf("TransformToGo mismatch:\n  got:  %q\n  want: %q", got, want)
			}
		})
	}
}

func TestTransformLetDeclarations(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple let declaration",
			input:    "let x = 42",
			expected: "x := 42",
		},
		{
			name:     "let with string",
			input:    `let name = "hello"`,
			expected: `name := "hello"`,
		},
		{
			name:     "let in function",
			input:    "func foo() { let x = 1 }",
			expected: "func foo() { x := 1 }",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := TransformToGo([]byte(tt.input))
			if err != nil {
				t.Fatalf("TransformToGo failed: %v", err)
			}

			got := normalizeWhitespace(string(result))
			want := normalizeWhitespace(tt.expected)

			if got != want {
				t.Errorf("TransformToGo mismatch:\n  got:  %q\n  want: %q", got, want)
			}
		})
	}
}

func TestParseFile(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name: "simple package with function",
			input: `package main

func main() {
	println("hello")
}
`,
			wantErr: false,
		},
		{
			name: "function with type annotations",
			input: `package main

func add(x: int, y: int) int {
	return x + y
}
`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fset := token.NewFileSet()
			file, err := ParseFile(fset, "test.go", []byte(tt.input), 0)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("ParseFile failed: %v", err)
			}

			if file == nil {
				t.Fatal("ParseFile returned nil file")
			}

			if file.Name == nil || file.Name.Name != "main" {
				t.Errorf("expected package name 'main', got %v", file.Name)
			}
		})
	}
}

func normalizeWhitespace(s string) string {
	// Replace multiple spaces/tabs/newlines with single space
	fields := strings.Fields(s)
	return strings.Join(fields, " ")
}

func TestGuardLetTransformation(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple guard let",
			input:    `guard let user = FindUser(id) else |err| { return err }`,
			expected: `user, err := FindUser(id) if err != nil { return err }`,
		},
		{
			name:     "guard let without error binding",
			input:    `guard let user = FindUser(id) else { return nil }`,
			expected: `user, err := FindUser(id) if err != nil { return nil }`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := transformGuardLet([]byte(tt.input))

			got := normalizeWhitespace(string(result))
			want := normalizeWhitespace(tt.expected)

			if got != want {
				t.Errorf("transformGuardLet mismatch:\n  input: %q\n  got:   %q\n  want:  %q", tt.input, got, want)
			}
		})
	}
}

func TestLambdaTransformation(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "rust-style simple lambda",
			input:    `|x| x + 1`,
			expected: `func(x) { return x + 1 }`,
		},
		{
			name:     "rust-style two params",
			input:    `|a, b| a + b`,
			expected: `func(a, b) { return a + b }`,
		},
		{
			name:     "typescript-style simple lambda",
			input:    `(x) => x + 1`,
			expected: `func(x) { return x + 1 }`,
		},
		{
			name:     "typescript-style two params",
			input:    `(a, b) => a + b`,
			expected: `func(a, b) { return a + b }`,
		},
		{
			name:     "lambda with block body",
			input:    `|x| { return x + 1 }`,
			expected: `func(x) { return x + 1 }`,
		},
		{
			name:     "lambda in function call",
			input:    `Filter(users, |u| u.Active)`,
			expected: `Filter(users, func(u) { return u.Active })`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := transformLambdas([]byte(tt.input))

			got := normalizeWhitespace(string(result))
			want := normalizeWhitespace(tt.expected)

			if got != want {
				t.Errorf("transformLambdas mismatch:\n  input: %q\n  got:   %q\n  want:  %q", tt.input, got, want)
			}
		})
	}
}

func TestEnumTransformation(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "simple enum",
			input: `enum Status {
				Pending
				Active
				Completed
			}`,
			expected: `type Status interface { isStatus() }
				type StatusPending struct {}
				func (StatusPending) isStatus() {}
				func NewStatusPending() Status { return StatusPending{} }
				type StatusActive struct {}
				func (StatusActive) isStatus() {}
				func NewStatusActive() Status { return StatusActive{} }
				type StatusCompleted struct {}
				func (StatusCompleted) isStatus() {}
				func NewStatusCompleted() Status { return StatusCompleted{} }`,
		},
		{
			name: "enum with fields",
			input: `enum Event {
				UserCreated { userID: int, email: string }
				UserDeleted { userID: int }
			}`,
			expected: `type Event interface { isEvent() }
				type EventUserCreated struct {userID int; email string}
				func (EventUserCreated) isEvent() {}
				func NewEventUserCreated(userID int, email string) Event { return EventUserCreated{userID: userID, email: email} }
				type EventUserDeleted struct {userID int}
				func (EventUserDeleted) isEvent() {}
				func NewEventUserDeleted(userID int) Event { return EventUserDeleted{userID: userID} }`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := transformEnum([]byte(tt.input))

			got := normalizeWhitespace(string(result))
			want := normalizeWhitespace(tt.expected)

			if got != want {
				t.Errorf("transformEnum mismatch:\n  got:  %q\n  want: %q", got, want)
			}
		})
	}
}

func TestMatchTransformation(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple match with default",
			input:    `match x { _ => 0 }`,
			// P1 fix: infers int return type from literal 0, uses panic for exhaustiveness
			expected: `func() int { switch (x).(type) { default: return 0 } panic("exhaustive match failed") }()`,
		},
		{
			name:     "match with pattern (no enum)",
			input:    `match event { UserCreated(id, email) => id }`,
			// Without an enum definition, UserCreated is not expanded
			// Note: email is extracted but unused - Go compiler will catch this
			expected: `func() interface{} { switch __matchVal := (event).(type) { case UserCreated: id := __matchVal.id email := __matchVal.email return id } return nil }()`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use TransformToGo which resets the registry
			result, _, err := TransformToGo([]byte(tt.input))
			if err != nil {
				t.Fatalf("TransformToGo failed: %v", err)
			}

			got := normalizeWhitespace(string(result))
			want := normalizeWhitespace(tt.expected)

			if got != want {
				t.Errorf("TransformToGo mismatch:\n  got:  %q\n  want: %q", got, want)
			}
		})
	}
}

func TestQuestionMarkInStrings(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "question mark in SQL query",
			input:    `db.QueryRow("SELECT * FROM users WHERE id = ?", id)`,
			expected: `db.QueryRow("SELECT * FROM users WHERE id = ?", id)`,
		},
		{
			name:     "question mark in string should not transform",
			input:    `fmt.Println("Is this a question?")`,
			expected: `fmt.Println("Is this a question?")`,
		},
		{
			name:     "question mark outside string should transform",
			input:    `let result = getValue()?`,
			expected: `tmp, err := getValue() if err != nil { return err } var result = tmp`,
		},
		{
			name:     "mixed - string with ? and error prop",
			input:    `let x = query("SELECT * WHERE id = ?")?`,
			expected: `tmp, err := query("SELECT * WHERE id = ?") if err != nil { return err } var x = tmp`,
		},
		{
			name:     "raw string with question mark",
			input:    "s := `is this a question?`",
			expected: "s := `is this a question?`",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := TransformToGo([]byte(tt.input))
			if err != nil {
				t.Fatalf("TransformToGo failed: %v", err)
			}

			got := normalizeWhitespace(string(result))
			want := normalizeWhitespace(tt.expected)

			if got != want {
				t.Errorf("TransformToGo mismatch:\n  got:  %q\n  want: %q", got, want)
			}
		})
	}
}

func TestNullCoalescingTransformation(t *testing.T) {
	tests := []struct {
		name            string
		input           string
		shouldContain   []string
		shouldNotContain []string
	}{
		{
			name:  "null coalescing in raw string should NOT transform",
			input: "s := `user ?? default`",
			shouldContain: []string{
				"`user ?? default`",
			},
			shouldNotContain: []string{
				"func() interface{}",
			},
		},
		// NOTE: Current limitation - ?? operator conflicts with ? error propagation operator
		// When ? is processed first, it breaks ?? into ? + ?
		// This will be fixed when we move to AST-based transformation
		// For now, tests focus on string literal handling (ensuring ?? inside strings is preserved)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := TransformToGo([]byte(tt.input))
			if err != nil {
				t.Fatalf("TransformToGo failed: %v", err)
			}

			got := string(result)

			// Check that all expected patterns are present
			for _, pattern := range tt.shouldContain {
				if !strings.Contains(got, pattern) {
					t.Errorf("Expected output to contain %q, but it didn't.\n  input: %q\n  got:   %q", pattern, tt.input, got)
				}
			}

			// Check that unwanted patterns are not present
			for _, pattern := range tt.shouldNotContain {
				if strings.Contains(got, pattern) {
					t.Errorf("Expected output to NOT contain %q, but it did.\n  input: %q\n  got:   %q", pattern, tt.input, got)
				}
			}
		})
	}
}

func TestSafeNavigationTransformation(t *testing.T) {
	tests := []struct {
		name             string
		input            string
		shouldContain    []string
		shouldNotContain []string
	}{
		{
			name:  "safe navigation in raw string should NOT transform",
			input: "s := `obj?.field`",
			shouldContain: []string{
				"`obj?.field`",
			},
			shouldNotContain: []string{
				"var safeNav",
			},
		},
		{
			name:  "chained safe navigation with parentheses",
			input: `let city = (user)?.address?.city`,
			shouldContain: []string{
				"func() interface{}",
				"if address != nil",
				".city",
			},
		},
		{
			name:  "safe navigation combined with null coalescing",
			input: `let name = (user)?.name ?? "Anonymous"`,
			shouldContain: []string{
				"func() interface{}",
				".name",
				`"Anonymous"`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := TransformToGo([]byte(tt.input))
			if err != nil {
				t.Fatalf("TransformToGo failed: %v", err)
			}

			got := string(result)

			// Check that all expected patterns are present
			for _, pattern := range tt.shouldContain {
				if !strings.Contains(got, pattern) {
					t.Errorf("Expected output to contain %q, but it didn't.\n  input: %q\n  got:   %q", pattern, tt.input, got)
				}
			}

			// Check that unwanted patterns are not present
			for _, pattern := range tt.shouldNotContain {
				if strings.Contains(got, pattern) {
					t.Errorf("Expected output to NOT contain %q, but it did.\n  input: %q\n  got:   %q", pattern, tt.input, got)
				}
			}
		})
	}
}
