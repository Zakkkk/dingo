package parser

import (
	"go/token"
	"testing"

	"github.com/MadAppGang/dingo/pkg/ast"
	"github.com/MadAppGang/dingo/pkg/tokenizer"
)

func TestParseMatchExpr_Simple(t *testing.T) {
	src := []byte(`match x {
		Ok(v) => v,
		Err(e) => 0,
	}`)

	tok := tokenizer.New(src)
	_, err := tok.Tokenize()
	if err != nil {
		t.Fatalf("tokenization failed: %v", err)
	}
	tok.Reset()

	parser := NewPrattParser(tok)
	expr := parser.ParseExpression(PrecLowest)

	if expr == nil {
		if len(parser.Errors()) > 0 {
			for _, err := range parser.Errors() {
				t.Logf("parser error: %v", err)
			}
		}
		t.Logf("current token: %v", parser.curToken)
		t.Fatalf("expected match expression, got nil")
	}

	matchExpr, ok := expr.(*ast.MatchExpr)
	if !ok {
		t.Fatalf("expected *ast.MatchExpr, got %T", expr)
	}

	// Check scrutinee exists
	if matchExpr.Scrutinee == nil {
		t.Error("scrutinee is nil")
	}

	// Check we have 2 arms
	if len(matchExpr.Arms) != 2 {
		t.Errorf("expected 2 arms, got %d", len(matchExpr.Arms))
	}

	// Check first arm is Ok(v) pattern
	if len(matchExpr.Arms) > 0 {
		arm := matchExpr.Arms[0]
		pattern, ok := arm.Pattern.(*ast.ConstructorPattern)
		if !ok {
			t.Errorf("expected first pattern to be ConstructorPattern, got %T", arm.Pattern)
		} else if pattern.Name != "Ok" {
			t.Errorf("expected pattern name 'Ok', got %q", pattern.Name)
		}
	}
}

func TestParseMatchExpr_WithGuard(t *testing.T) {
	src := []byte(`match x {
		Ok(v) if v > 0 => v,
		_ => 0,
	}`)

	tok := tokenizer.New(src)
	_, err := tok.Tokenize()
	if err != nil {
		t.Fatalf("tokenization failed: %v", err)
	}
	tok.Reset()

	parser := NewPrattParser(tok)
	expr := parser.ParseExpression(PrecLowest)

	matchExpr, ok := expr.(*ast.MatchExpr)
	if !ok {
		t.Fatalf("expected *ast.MatchExpr, got %T", expr)
	}

	// Check first arm has guard
	if len(matchExpr.Arms) > 0 {
		arm := matchExpr.Arms[0]
		if arm.Guard == nil {
			t.Error("expected first arm to have guard, got nil")
		}
	}
}

func TestParseMatchExpr_NestedPatterns(t *testing.T) {
	src := []byte(`match result {
		Ok(Some(x)) => x,
		Ok(None) => 0,
		Err(e) => -1,
	}`)

	tok := tokenizer.New(src)
	_, err := tok.Tokenize()
	if err != nil {
		t.Fatalf("tokenization failed: %v", err)
	}
	tok.Reset()

	parser := NewPrattParser(tok)
	expr := parser.ParseExpression(PrecLowest)

	matchExpr, ok := expr.(*ast.MatchExpr)
	if !ok {
		t.Fatalf("expected *ast.MatchExpr, got %T", expr)
	}

	// Check first arm has nested pattern
	if len(matchExpr.Arms) > 0 {
		arm := matchExpr.Arms[0]
		pattern, ok := arm.Pattern.(*ast.ConstructorPattern)
		if !ok {
			t.Fatalf("expected ConstructorPattern, got %T", arm.Pattern)
		}

		if len(pattern.Params) != 1 {
			t.Errorf("expected 1 param, got %d", len(pattern.Params))
		}

		// Check nested pattern is Some(x)
		if len(pattern.Params) > 0 {
			nested, ok := pattern.Params[0].(*ast.ConstructorPattern)
			if !ok {
				t.Errorf("expected nested ConstructorPattern, got %T", pattern.Params[0])
			} else if nested.Name != "Some" {
				t.Errorf("expected nested pattern name 'Some', got %q", nested.Name)
			}
		}
	}
}

func TestParseMatchExpr_TuplePattern(t *testing.T) {
	src := []byte(`match pair {
		(Ok(x), Ok(y)) => x + y,
		_ => 0,
	}`)

	tok := tokenizer.New(src)
	_, err := tok.Tokenize()
	if err != nil {
		t.Fatalf("tokenization failed: %v", err)
	}
	tok.Reset()

	parser := NewPrattParser(tok)
	expr := parser.ParseExpression(PrecLowest)

	matchExpr, ok := expr.(*ast.MatchExpr)
	if !ok {
		t.Fatalf("expected *ast.MatchExpr, got %T", expr)
	}

	// Check first arm has tuple pattern
	if len(matchExpr.Arms) > 0 {
		arm := matchExpr.Arms[0]
		pattern, ok := arm.Pattern.(*ast.TuplePattern)
		if !ok {
			t.Fatalf("expected TuplePattern, got %T", arm.Pattern)
		}

		if len(pattern.Elements) != 2 {
			t.Errorf("expected 2 tuple elements, got %d", len(pattern.Elements))
		}
	}
}

func TestParsePattern_Wildcard(t *testing.T) {
	src := []byte(`match x {
		_ => 0,
	}`)

	tok := tokenizer.New(src)
	_, err := tok.Tokenize()
	if err != nil {
		t.Fatalf("tokenization failed: %v", err)
	}
	tok.Reset()

	parser := NewPrattParser(tok)
	expr := parser.ParseExpression(PrecLowest)

	matchExpr, ok := expr.(*ast.MatchExpr)
	if !ok {
		t.Fatalf("expected *ast.MatchExpr, got %T", expr)
	}

	if len(matchExpr.Arms) != 1 {
		t.Fatalf("expected 1 arm, got %d", len(matchExpr.Arms))
	}

	pattern, ok := matchExpr.Arms[0].Pattern.(*ast.WildcardPattern)
	if !ok {
		t.Errorf("expected WildcardPattern, got %T", matchExpr.Arms[0].Pattern)
	}
	if !ok && pattern.String() != "_" {
		t.Errorf("expected wildcard string '_', got %q", pattern.String())
	}
}

func TestParsePattern_Literal(t *testing.T) {
	src := []byte(`match status {
		200 => "OK",
		404 => "Not Found",
		_ => "Error",
	}`)

	tok := tokenizer.New(src)
	_, err := tok.Tokenize()
	if err != nil {
		t.Fatalf("tokenization failed: %v", err)
	}
	tok.Reset()

	parser := NewPrattParser(tok)
	expr := parser.ParseExpression(PrecLowest)

	matchExpr, ok := expr.(*ast.MatchExpr)
	if !ok {
		t.Fatalf("expected *ast.MatchExpr, got %T", expr)
	}

	if len(matchExpr.Arms) < 2 {
		t.Fatalf("expected at least 2 arms, got %d", len(matchExpr.Arms))
	}

	// Check first arm has literal pattern
	pattern, ok := matchExpr.Arms[0].Pattern.(*ast.LiteralPattern)
	if !ok {
		t.Errorf("expected LiteralPattern, got %T", matchExpr.Arms[0].Pattern)
	} else if pattern.Value != "200" {
		t.Errorf("expected literal value '200', got %q", pattern.Value)
	}
}

func TestParsePattern_Variable(t *testing.T) {
	src := []byte(`match value {
		x => x * 2,
	}`)

	tok := tokenizer.New(src)
	_, err := tok.Tokenize()
	if err != nil {
		t.Fatalf("tokenization failed: %v", err)
	}
	tok.Reset()

	parser := NewPrattParser(tok)
	expr := parser.ParseExpression(PrecLowest)

	matchExpr, ok := expr.(*ast.MatchExpr)
	if !ok {
		t.Fatalf("expected *ast.MatchExpr, got %T", expr)
	}

	if len(matchExpr.Arms) != 1 {
		t.Fatalf("expected 1 arm, got %d", len(matchExpr.Arms))
	}

	pattern, ok := matchExpr.Arms[0].Pattern.(*ast.VariablePattern)
	if !ok {
		t.Errorf("expected VariablePattern, got %T", matchExpr.Arms[0].Pattern)
	} else if pattern.Name != "x" {
		t.Errorf("expected variable name 'x', got %q", pattern.Name)
	}
}

func TestParseMatchBody_Block(t *testing.T) {
	src := []byte(`match x {
		Ok(v) => {
			println("success")
			v
		},
		_ => 0,
	}`)

	tok := tokenizer.New(src)
	_, err := tok.Tokenize()
	if err != nil {
		t.Fatalf("tokenization failed: %v", err)
	}
	tok.Reset()

	parser := NewPrattParser(tok)
	expr := parser.ParseExpression(PrecLowest)

	matchExpr, ok := expr.(*ast.MatchExpr)
	if !ok {
		t.Fatalf("expected *ast.MatchExpr, got %T", expr)
	}

	if len(matchExpr.Arms) < 1 {
		t.Fatalf("expected at least 1 arm, got %d", len(matchExpr.Arms))
	}

	arm := matchExpr.Arms[0]
	if !arm.IsBlock {
		t.Error("expected first arm to have block body")
	}

	// Check body is RawExpr
	if _, ok := arm.Body.(*ast.RawExpr); !ok {
		t.Errorf("expected block body to be RawExpr, got %T", arm.Body)
	}
}

// Helper to create a simple token for testing
func makeToken(kind tokenizer.TokenKind, lit string) tokenizer.Token {
	return tokenizer.Token{
		Kind: kind,
		Pos:  token.Pos(1),
		End:  token.Pos(1),
		Lit:  lit,
		Line: 1,
		Column: 1,
	}
}
