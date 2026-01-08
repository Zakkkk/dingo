package matchparser

import (
	"strings"
	"testing"

	"github.com/MadAppGang/dingo/pkg/ast"
	"github.com/MadAppGang/dingo/pkg/tokenizer"
)

func TestMatchParser_SimplePatterns(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantArms    int
		wantPattern string
		wantBody    string
	}{
		{
			name:        "wildcard pattern",
			input:       "match x { _ => 0 }",
			wantArms:    1,
			wantPattern: "_",
			wantBody:    "0",
		},
		{
			name:        "variable pattern",
			input:       "match x { value => value }",
			wantArms:    1,
			wantPattern: "value",
			wantBody:    "value",
		},
		{
			name:        "int literal pattern",
			input:       "match x { 42 => \"answer\" }",
			wantArms:    1,
			wantPattern: "42",
			wantBody:    `"answer"`,
		},
		{
			name:        "string literal pattern",
			input:       `match x { "hello" => 1 }`,
			wantArms:    1,
			wantPattern: `"hello"`,
			wantBody:    "1",
		},
		{
			name:        "simple constructor Ok",
			input:       "match result { Ok(x) => x }",
			wantArms:    1,
			wantPattern: "Ok(x)",
			wantBody:    "x",
		},
		{
			name:        "simple constructor Err",
			input:       "match result { Err(e) => e }",
			wantArms:    1,
			wantPattern: "Err(e)",
			wantBody:    "e",
		},
		{
			name:        "nullary constructor None",
			input:       "match opt { None => 0 }",
			wantArms:    1,
			wantPattern: "None",
			wantBody:    "0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tok := tokenizer.New([]byte(tt.input))
			_, err := tok.Tokenize()
			if err != nil {
				t.Fatalf("tokenize error: %v", err)
			}

			parser := NewMatchParser(tok)
			expr, err := parser.ParseMatchExpr()
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}

			if len(expr.Arms) != tt.wantArms {
				t.Errorf("got %d arms, want %d", len(expr.Arms), tt.wantArms)
			}

			if len(expr.Arms) > 0 {
				gotPattern := expr.Arms[0].Pattern.String()
				if gotPattern != tt.wantPattern {
					t.Errorf("got pattern %q, want %q", gotPattern, tt.wantPattern)
				}

				gotBody := expr.Arms[0].Body.String()
				if gotBody != tt.wantBody {
					t.Errorf("got body %q, want %q", gotBody, tt.wantBody)
				}
			}
		})
	}
}

func TestMatchParser_NestedPatterns(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantPattern string
		wantNested  bool
	}{
		{
			name:        "Ok(Some(x))",
			input:       "match wrapped { Ok(Some(x)) => x }",
			wantPattern: "Ok(Some(x))",
			wantNested:  true,
		},
		{
			name:        "Err(Error(code, msg))",
			input:       "match result { Err(Error(code, msg)) => code }",
			wantPattern: "Err(Error(code, msg))",
			wantNested:  true,
		},
		{
			name:        "Ok(Some(Ok(value)))",
			input:       "match deep { Ok(Some(Ok(value))) => value }",
			wantPattern: "Ok(Some(Ok(value)))",
			wantNested:  true,
		},
		{
			name:        "tuple with nested",
			input:       "match pair { (Ok(x), Err(e)) => x }",
			wantPattern: "(Ok(x), Err(e))",
			wantNested:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tok := tokenizer.New([]byte(tt.input))
			_, err := tok.Tokenize()
			if err != nil {
				t.Fatalf("tokenize error: %v", err)
			}

			parser := NewMatchParser(tok)
			expr, err := parser.ParseMatchExpr()
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}

			if len(expr.Arms) == 0 {
				t.Fatal("no arms parsed")
			}

			gotPattern := expr.Arms[0].Pattern.String()
			if gotPattern != tt.wantPattern {
				t.Errorf("got pattern %q, want %q", gotPattern, tt.wantPattern)
			}

			// Verify nesting by checking pattern structure
			if tt.wantNested {
				verifyNested(t, expr.Arms[0].Pattern)
			}
		})
	}
}

func TestMatchParser_TuplePatterns(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantPattern string
		wantElems   int
	}{
		{
			name:        "simple tuple (a, b)",
			input:       "match pair { (a, b) => a }",
			wantPattern: "(a, b)",
			wantElems:   2,
		},
		{
			name:        "tuple with constructors",
			input:       "match pair { (Ok(x), Err(e)) => x }",
			wantPattern: "(Ok(x), Err(e))",
			wantElems:   2,
		},
		{
			name:        "triple tuple",
			input:       "match triple { (a, b, c) => a }",
			wantPattern: "(a, b, c)",
			wantElems:   3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tok := tokenizer.New([]byte(tt.input))
			_, err := tok.Tokenize()
			if err != nil {
				t.Fatalf("tokenize error: %v", err)
			}

			parser := NewMatchParser(tok)
			expr, err := parser.ParseMatchExpr()
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}

			if len(expr.Arms) == 0 {
				t.Fatal("no arms parsed")
			}

			gotPattern := expr.Arms[0].Pattern.String()
			if gotPattern != tt.wantPattern {
				t.Errorf("got pattern %q, want %q", gotPattern, tt.wantPattern)
			}

			// Verify tuple structure
			tuple, ok := expr.Arms[0].Pattern.(*ast.TuplePattern)
			if !ok {
				t.Fatalf("pattern is not a tuple: %T", expr.Arms[0].Pattern)
			}

			if len(tuple.Elements) != tt.wantElems {
				t.Errorf("got %d elements, want %d", len(tuple.Elements), tt.wantElems)
			}
		})
	}
}

func TestMatchParser_Guards(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantGuard string
	}{
		{
			name:      "if guard",
			input:     "match x { value if value > 0 => value }",
			wantGuard: "value > 0",
		},
		{
			name:      "where guard",
			input:     "match x { value where value > 0 => value }",
			wantGuard: "value > 0",
		},
		{
			name:      "complex guard",
			input:     "match x { value if value > 0 && value < 100 => value }",
			wantGuard: "value > 0 && value < 100",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tok := tokenizer.New([]byte(tt.input))
			_, err := tok.Tokenize()
			if err != nil {
				t.Fatalf("tokenize error: %v", err)
			}

			parser := NewMatchParser(tok)
			expr, err := parser.ParseMatchExpr()
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}

			if len(expr.Arms) == 0 {
				t.Fatal("no arms parsed")
			}

			if expr.Arms[0].Guard == nil {
				t.Fatal("guard is nil")
			}

			gotGuard := expr.Arms[0].Guard.String()
			if gotGuard != tt.wantGuard {
				t.Errorf("got guard %q, want %q", gotGuard, tt.wantGuard)
			}
		})
	}
}

func TestMatchParser_BlockBodies(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantBlock bool
	}{
		{
			name:      "simple block",
			input:     "match x { Ok(v) => { println(v) } }",
			wantBlock: true,
		},
		{
			name:      "multiline block",
			input:     "match x { Ok(v) => {\nlet y = v\nprintln(y)\n} }",
			wantBlock: true,
		},
		{
			name:      "expression body",
			input:     "match x { Ok(v) => v * 2 }",
			wantBlock: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tok := tokenizer.New([]byte(tt.input))
			_, err := tok.Tokenize()
			if err != nil {
				t.Fatalf("tokenize error: %v", err)
			}

			parser := NewMatchParser(tok)
			expr, err := parser.ParseMatchExpr()
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}

			if len(expr.Arms) == 0 {
				t.Fatal("no arms parsed")
			}

			if expr.Arms[0].IsBlock != tt.wantBlock {
				t.Errorf("got IsBlock=%v, want %v", expr.Arms[0].IsBlock, tt.wantBlock)
			}
		})
	}
}

func TestMatchParser_CommentsAfterArms(t *testing.T) {
	// P0 bug: Comments after arms should NOT be treated as separate arms
	input := `match event {
		Click(x, y) => handleClick(x, y), // Click events
		Scroll(delta) => handleScroll(delta), // Scroll events
		_ => {}
	}`

	tok := tokenizer.New([]byte(input))
	_, err := tok.Tokenize()
	if err != nil {
		t.Fatalf("tokenize error: %v", err)
	}

	parser := NewMatchParser(tok)
	expr, err := parser.ParseMatchExpr()
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	// Should have exactly 3 arms, NOT more (comments shouldn't create arms)
	if len(expr.Arms) != 3 {
		t.Errorf("got %d arms, want 3 (comments should not create arms)", len(expr.Arms))
	}

	// Verify comments are preserved on arms
	if expr.Arms[0].Comment == nil {
		t.Error("first arm should have comment")
	} else if !strings.Contains(expr.Arms[0].Comment.Text, "Click events") {
		t.Errorf("got comment %q, want to contain 'Click events'", expr.Arms[0].Comment.Text)
	}

	if expr.Arms[1].Comment == nil {
		t.Error("second arm should have comment")
	} else if !strings.Contains(expr.Arms[1].Comment.Text, "Scroll events") {
		t.Errorf("got comment %q, want to contain 'Scroll events'", expr.Arms[1].Comment.Text)
	}
}

func TestMatchParser_MultipleArms(t *testing.T) {
	input := `match result {
		Ok(x) => x,
		Err(e) => 0,
		_ => -1
	}`

	tok := tokenizer.New([]byte(input))
	_, err := tok.Tokenize()
	if err != nil {
		t.Fatalf("tokenize error: %v", err)
	}

	parser := NewMatchParser(tok)
	expr, err := parser.ParseMatchExpr()
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if len(expr.Arms) != 3 {
		t.Errorf("got %d arms, want 3", len(expr.Arms))
	}

	// Verify pattern types
	if _, ok := expr.Arms[0].Pattern.(*ast.ConstructorPattern); !ok {
		t.Errorf("arm 0 pattern should be ConstructorPattern, got %T", expr.Arms[0].Pattern)
	}

	if _, ok := expr.Arms[1].Pattern.(*ast.ConstructorPattern); !ok {
		t.Errorf("arm 1 pattern should be ConstructorPattern, got %T", expr.Arms[1].Pattern)
	}

	if _, ok := expr.Arms[2].Pattern.(*ast.WildcardPattern); !ok {
		t.Errorf("arm 2 pattern should be WildcardPattern, got %T", expr.Arms[2].Pattern)
	}
}

func TestMatchParser_ComplexScrutinee(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		wantScrutinee string
	}{
		{
			name:          "simple variable",
			input:         "match x { _ => 0 }",
			wantScrutinee: "x",
		},
		{
			name:          "function call",
			input:         "match getResult() { _ => 0 }",
			wantScrutinee: "getResult ( )",
		},
		{
			name:          "field access",
			input:         "match user.status { _ => 0 }",
			wantScrutinee: "user . status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tok := tokenizer.New([]byte(tt.input))
			_, err := tok.Tokenize()
			if err != nil {
				t.Fatalf("tokenize error: %v", err)
			}

			parser := NewMatchParser(tok)
			expr, err := parser.ParseMatchExpr()
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}

			gotScrutinee := expr.Scrutinee.String()
			if gotScrutinee != tt.wantScrutinee {
				t.Errorf("got scrutinee %q, want %q", gotScrutinee, tt.wantScrutinee)
			}
		})
	}
}

func TestMatchParser_ErrorCases(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantError string
	}{
		{
			name:      "missing open brace",
			input:     "match x Ok(v) => v }",
			wantError: "unexpected EOF in match scrutinee",
		},
		{
			name:      "missing close brace",
			input:     "match x { Ok(v) => v",
			wantError: "unexpected token EOF in pattern",
		},
		{
			name:      "missing arrow",
			input:     "match x { Ok(v) v }",
			wantError: "missing '=>'",
		},
		{
			name:      "unterminated block",
			input:     "match x { Ok(v) => { println(v) }",
			wantError: "unexpected token EOF in pattern",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tok := tokenizer.New([]byte(tt.input))
			_, err := tok.Tokenize()
			if err != nil {
				// Tokenization error is acceptable for some invalid inputs
				return
			}

			parser := NewMatchParser(tok)
			_, err = parser.ParseMatchExpr()
			if err == nil {
				t.Fatal("expected parse error, got nil")
			}

			if !strings.Contains(err.Error(), tt.wantError) {
				t.Errorf("got error %q, want to contain %q", err.Error(), tt.wantError)
			}
		})
	}
}

func TestMatchParser_BindingExtraction(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantBindings []string
	}{
		{
			name:         "simple binding",
			input:        "match x { Ok(value) => value }",
			wantBindings: []string{"value"},
		},
		{
			name:         "nested bindings",
			input:        "match x { Ok(Some(value)) => value }",
			wantBindings: []string{"value"},
		},
		{
			name:         "tuple bindings",
			input:        "match x { (a, b) => a }",
			wantBindings: []string{"a", "b"},
		},
		{
			name:         "nested tuple bindings",
			input:        "match x { (Ok(x), Err(e)) => x }",
			wantBindings: []string{"x", "e"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tok := tokenizer.New([]byte(tt.input))
			_, err := tok.Tokenize()
			if err != nil {
				t.Fatalf("tokenize error: %v", err)
			}

			parser := NewMatchParser(tok)
			expr, err := parser.ParseMatchExpr()
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}

			if len(expr.Arms) == 0 {
				t.Fatal("no arms parsed")
			}

			bindings := expr.Arms[0].Pattern.GetBindings()
			if len(bindings) != len(tt.wantBindings) {
				t.Errorf("got %d bindings, want %d", len(bindings), len(tt.wantBindings))
			}

			for i, want := range tt.wantBindings {
				if i >= len(bindings) {
					break
				}
				if bindings[i].Name != want {
					t.Errorf("binding %d: got %q, want %q", i, bindings[i].Name, want)
				}
			}
		})
	}
}

// Helper function to verify nested patterns
func verifyNested(t *testing.T, p ast.Pattern) {
	t.Helper()

	switch pattern := p.(type) {
	case *ast.ConstructorPattern:
		if len(pattern.Params) == 0 {
			t.Error("constructor pattern should have nested params")
			return
		}
		// Check if params contain constructors (nested)
		for _, param := range pattern.Params {
			if _, ok := param.(*ast.ConstructorPattern); ok {
				return // Found nested constructor
			}
		}
	case *ast.TuplePattern:
		for _, elem := range pattern.Elements {
			if _, ok := elem.(*ast.ConstructorPattern); ok {
				return // Found nested constructor
			}
		}
	}
}
