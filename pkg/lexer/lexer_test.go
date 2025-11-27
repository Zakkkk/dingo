package lexer

import (
	"testing"
)

func TestLexerBasicTokens(t *testing.T) {
	input := `x foo 123 3.14 "hello" bar`

	tests := []struct {
		expectedType    TokenType
		expectedLiteral string
	}{
		{IDENT, "x"},
		{IDENT, "foo"},
		{INT, "123"},
		{FLOAT, "3.14"},
		{STRING, "hello"},
		{IDENT, "bar"},
		{EOF, ""},
	}

	l := New(input)

	for i, tt := range tests {
		tok := l.NextToken()

		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - tokentype wrong. expected=%q, got=%q",
				i, tt.expectedType, tok.Type)
		}

		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - literal wrong. expected=%q, got=%q",
				i, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestLexerKeywords(t *testing.T) {
	input := `let var letx varx`

	tests := []struct {
		expectedType    TokenType
		expectedLiteral string
	}{
		{LET, "let"},
		{VAR, "var"},
		{IDENT, "letx"},
		{IDENT, "varx"},
		{EOF, ""},
	}

	l := New(input)

	for i, tt := range tests {
		tok := l.NextToken()

		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - tokentype wrong. expected=%q, got=%q",
				i, tt.expectedType, tok.Type)
		}

		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - literal wrong. expected=%q, got=%q",
				i, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestLexerOperators(t *testing.T) {
	input := `= := : , ( ) [ ] < >`

	tests := []struct {
		expectedType    TokenType
		expectedLiteral string
	}{
		{ASSIGN, "="},
		{DEFINE, ":="},
		{COLON, ":"},
		{COMMA, ","},
		{LPAREN, "("},
		{RPAREN, ")"},
		{LBRACKET, "["},
		{RBRACKET, "]"},
		{LANGLE, "<"},
		{RANGLE, ">"},
		{EOF, ""},
	}

	l := New(input)

	for i, tt := range tests {
		tok := l.NextToken()

		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - tokentype wrong. expected=%q, got=%q",
				i, tt.expectedType, tok.Type)
		}

		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - literal wrong. expected=%q, got=%q",
				i, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestLexerLetDeclarations(t *testing.T) {
	input := `let x: int = 5
let a, b = getValues()`

	tests := []struct {
		expectedType    TokenType
		expectedLiteral string
	}{
		{LET, "let"},
		{IDENT, "x"},
		{COLON, ":"},
		{IDENT, "int"},
		{ASSIGN, "="},
		{INT, "5"},
		{NEWLINE, "\\n"},
		{LET, "let"},
		{IDENT, "a"},
		{COMMA, ","},
		{IDENT, "b"},
		{ASSIGN, "="},
		{IDENT, "getValues"},
		{LPAREN, "("},
		{RPAREN, ")"},
		{EOF, ""},
	}

	l := New(input)

	for i, tt := range tests {
		tok := l.NextToken()

		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - tokentype wrong. expected=%q, got=%q",
				i, tt.expectedType, tok.Type)
		}

		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - literal wrong. expected=%q, got=%q",
				i, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestLexerPositions(t *testing.T) {
	input := `let x = 5
let y = 10`

	tests := []struct {
		expectedType TokenType
		expectedLine int
		expectedCol  int
	}{
		{LET, 1, 1},
		{IDENT, 1, 5},
		{ASSIGN, 1, 7},
		{INT, 1, 9},
		{NEWLINE, 1, 10},
		{LET, 2, 1},
		{IDENT, 2, 5},
		{ASSIGN, 2, 7},
		{INT, 2, 9},
	}

	l := New(input)

	for i, tt := range tests {
		tok := l.NextToken()

		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - tokentype wrong. expected=%q, got=%q",
				i, tt.expectedType, tok.Type)
		}

		if tok.Line != tt.expectedLine {
			t.Fatalf("tests[%d] - line wrong. expected=%d, got=%d",
				i, tt.expectedLine, tok.Line)
		}

		if tok.Column != tt.expectedCol {
			t.Fatalf("tests[%d] - column wrong. expected=%d, got=%d",
				i, tt.expectedCol, tok.Column)
		}
	}
}

func TestLexerComments(t *testing.T) {
	input := `let x = 5 // this is a comment
let y = 10 /* block comment */ let z = 15`

	tests := []struct {
		expectedType    TokenType
		expectedLiteral string
	}{
		{LET, "let"},
		{IDENT, "x"},
		{ASSIGN, "="},
		{INT, "5"},
		{NEWLINE, "\\n"},
		{LET, "let"},
		{IDENT, "y"},
		{ASSIGN, "="},
		{INT, "10"},
		{LET, "let"},
		{IDENT, "z"},
		{ASSIGN, "="},
		{INT, "15"},
		{EOF, ""},
	}

	l := New(input)

	for i, tt := range tests {
		tok := l.NextToken()

		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - tokentype wrong. expected=%q, got=%q (literal=%q)",
				i, tt.expectedType, tok.Type, tok.Literal)
		}

		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - literal wrong. expected=%q, got=%q",
				i, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestLexerRawStrings(t *testing.T) {
	input := "`raw string with\nnewline` \"regular string\""

	tests := []struct {
		expectedType    TokenType
		expectedLiteral string
	}{
		{STRING, "raw string with\nnewline"},
		{STRING, "regular string"},
		{EOF, ""},
	}

	l := New(input)

	for i, tt := range tests {
		tok := l.NextToken()

		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - tokentype wrong. expected=%q, got=%q",
				i, tt.expectedType, tok.Type)
		}

		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - literal wrong. expected=%q, got=%q",
				i, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestPeekToken(t *testing.T) {
	input := `let x = 5`

	l := New(input)

	// Peek at first token
	peek := l.PeekToken()
	if peek.Type != LET {
		t.Fatalf("peek wrong. expected=%q, got=%q", LET, peek.Type)
	}

	// Actually consume first token
	tok := l.NextToken()
	if tok.Type != LET {
		t.Fatalf("token wrong. expected=%q, got=%q", LET, tok.Type)
	}

	// Peek at second token
	peek = l.PeekToken()
	if peek.Type != IDENT || peek.Literal != "x" {
		t.Fatalf("peek wrong. expected=IDENT(x), got=%q(%q)", peek.Type, peek.Literal)
	}

	// Consume second token
	tok = l.NextToken()
	if tok.Type != IDENT || tok.Literal != "x" {
		t.Fatalf("token wrong. expected=IDENT(x), got=%q(%q)", tok.Type, tok.Literal)
	}
}

func TestLexerFloats(t *testing.T) {
	input := `3.14 0.5 123.456`

	tests := []struct {
		expectedType    TokenType
		expectedLiteral string
	}{
		{FLOAT, "3.14"},
		{FLOAT, "0.5"},
		{FLOAT, "123.456"},
		{EOF, ""},
	}

	l := New(input)

	for i, tt := range tests {
		tok := l.NextToken()

		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - tokentype wrong. expected=%q, got=%q",
				i, tt.expectedType, tok.Type)
		}

		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - literal wrong. expected=%q, got=%q",
				i, tt.expectedLiteral, tok.Literal)
		}
	}
}
