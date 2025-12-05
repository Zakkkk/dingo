package parser

import (
	"testing"

	"github.com/MadAppGang/dingo/pkg/tokenizer"
)

func TestTokenizeMatch(t *testing.T) {
	src := []byte(`match x { }`)

	tok := tokenizer.New(src)
	tokens, err := tok.Tokenize()
	if err != nil {
		t.Fatalf("tokenization failed: %v", err)
	}

	t.Logf("Token count: %d", len(tokens))
	for i, token := range tokens {
		t.Logf("Token %d: %v", i, token)
	}
}

func TestPrattParserInit(t *testing.T) {
	src := []byte(`match x { }`)

	tok := tokenizer.New(src)
	_, err := tok.Tokenize()
	if err != nil {
		t.Fatalf("tokenization failed: %v", err)
	}
	tok.Reset()

	parser := NewPrattParser(tok)
	t.Logf("After init - curToken: %v, peekToken: %v", parser.curToken, parser.peekToken)

	// Check if MATCH prefix parser is registered
	if fn, ok := parser.prefixParseFns[tokenizer.MATCH]; ok {
		t.Logf("MATCH prefix parser is registered: %v", fn != nil)
	} else {
		t.Error("MATCH prefix parser NOT registered!")
	}

	// Try to parse
	expr := parser.ParseExpression(PrecLowest)
	t.Logf("After ParseExpression - expr: %v (type: %T)", expr, expr)
	if expr == nil {
		t.Logf("Errors: %v", parser.Errors())
		t.Logf("curToken after parse: %v", parser.curToken)
	}
}
