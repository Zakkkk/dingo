package matchparser

import (
	"testing"

	"github.com/MadAppGang/dingo/pkg/tokenizer"
)

func TestMultilineExpressionBody(t *testing.T) {
	source := []byte(`match x {
		1 => fmt.Sprintf("one: %d", x),
		2 => fmt.Sprintf("two: %d", x),
		_ => "other",
	}`)

	tok := tokenizer.New(source)
	_, err := tok.Tokenize()
	if err != nil {
		t.Fatalf("tokenize error: %v", err)
	}

	parser := NewMatchParser(tok)
	matchExpr, err := parser.ParseMatchExpr()
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if len(matchExpr.Arms) != 3 {
		t.Errorf("expected 3 arms, got %d", len(matchExpr.Arms))
	}

	// Check first arm body contains the full fmt.Sprintf call
	// Note: Tokenizer separates tokens with spaces
	firstBody := matchExpr.Arms[0].Body.String()
	expected := `fmt . Sprintf ( "one: %d" , x )`
	if firstBody != expected {
		t.Errorf("unexpected first arm body:\ngot:  %q\nwant: %q", firstBody, expected)
	}
}
