package parser

import (
	"go/token"
	"strings"
	"testing"

	"github.com/MadAppGang/dingo/pkg/tokenizer"
)

// TestSpanErrorFormat tests SpanError string formatting
func TestSpanErrorFormat(t *testing.T) {
	tests := []struct {
		name        string
		err         SpanError
		wantMessage bool
		wantHint    bool
	}{
		{
			name: "message_only",
			err: SpanError{
				Message: "unexpected token",
			},
			wantMessage: true,
			wantHint:    false,
		},
		{
			name: "message_with_hint",
			err: SpanError{
				Message: "unexpected token",
				Hint:    "try using a different operator",
			},
			wantMessage: true,
			wantHint:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.err.Error()

			if tc.wantMessage && !strings.Contains(got, tc.err.Message) {
				t.Errorf("Error() missing message: got %q", got)
			}
			if tc.wantHint && !strings.Contains(got, tc.err.Hint) {
				t.Errorf("Error() missing hint: got %q", got)
			}
			if !tc.wantHint && strings.Contains(got, "Hint:") {
				t.Errorf("Error() should not contain hint: got %q", got)
			}
		})
	}
}

// TestErrorWithSpan tests the ErrorWithSpan constructor
func TestErrorWithSpan(t *testing.T) {
	err := ErrorWithSpan(
		token.Pos(10), token.Pos(15),
		1, 5, 10,
		ErrUnexpectedToken,
		"unexpected token",
		"use a different token",
		"in function body",
	)

	if err.Pos != token.Pos(10) {
		t.Errorf("Pos = %v, want 10", err.Pos)
	}
	if err.EndPos != token.Pos(15) {
		t.Errorf("EndPos = %v, want 15", err.EndPos)
	}
	if err.Line != 1 {
		t.Errorf("Line = %v, want 1", err.Line)
	}
	if err.Column != 5 {
		t.Errorf("Column = %v, want 5", err.Column)
	}
	if err.EndCol != 10 {
		t.Errorf("EndCol = %v, want 10", err.EndCol)
	}
	if err.Code != ErrUnexpectedToken {
		t.Errorf("Code = %v, want %v", err.Code, ErrUnexpectedToken)
	}
}

// TestErrorBuilder tests the fluent error builder API
func TestErrorBuilder(t *testing.T) {
	err := NewErrorBuilder(token.Pos(1), 1, 1).
		EndPos(token.Pos(5), 5).
		Code(ErrMissingArrow).
		Context("in lambda expression").
		Build("missing '=>'", "add arrow after parameters")

	if err.Pos != token.Pos(1) {
		t.Errorf("Pos = %v, want 1", err.Pos)
	}
	if err.EndPos != token.Pos(5) {
		t.Errorf("EndPos = %v, want 5", err.EndPos)
	}
	if err.EndCol != 5 {
		t.Errorf("EndCol = %v, want 5", err.EndCol)
	}
	if err.Code != ErrMissingArrow {
		t.Errorf("Code = %v, want %v", err.Code, ErrMissingArrow)
	}
	if err.Context != "in lambda expression" {
		t.Errorf("Context = %v, want 'in lambda expression'", err.Context)
	}
	if err.Message != "missing '=>'" {
		t.Errorf("Message = %v, want 'missing =>''", err.Message)
	}
	if err.Hint != "add arrow after parameters" {
		t.Errorf("Hint = %v, want 'add arrow after parameters'", err.Hint)
	}
}

// TestCommonErrorConstructors tests the common error constructors
func TestCommonErrorConstructors(t *testing.T) {
	t.Run("UnexpectedTokenError", func(t *testing.T) {
		err := UnexpectedTokenError("}", "expression", token.Pos(10), 5, 8)
		if err.Code != ErrUnexpectedToken {
			t.Errorf("Code = %v, want %v", err.Code, ErrUnexpectedToken)
		}
		if !strings.Contains(err.Message, "unexpected '}'") {
			t.Errorf("Message should contain unexpected token: %q", err.Message)
		}
	})

	t.Run("AmbiguousQuestionError", func(t *testing.T) {
		err := AmbiguousQuestionError(token.Pos(5), 2, 3)
		if err.Code != ErrAmbiguousQuestion {
			t.Errorf("Code = %v, want %v", err.Code, ErrAmbiguousQuestion)
		}
		if err.Hint == "" {
			t.Error("Hint should not be empty")
		}
	})

	t.Run("MissingArrowError", func(t *testing.T) {
		err := MissingArrowError("in lambda", token.Pos(10), 3, 5)
		if err.Code != ErrMissingArrow {
			t.Errorf("Code = %v, want %v", err.Code, ErrMissingArrow)
		}
		if err.Context != "in lambda" {
			t.Errorf("Context = %v, want 'in lambda'", err.Context)
		}
	})

	t.Run("MissingColonError", func(t *testing.T) {
		err := MissingColonError(token.Pos(15), 4, 10)
		if err.Code != ErrMissingColon {
			t.Errorf("Code = %v, want %v", err.Code, ErrMissingColon)
		}
		if !strings.Contains(err.Hint, "ternary") {
			t.Errorf("Hint should mention ternary: %q", err.Hint)
		}
	})

	t.Run("MissingOperandError", func(t *testing.T) {
		err := MissingOperandError("+", token.Pos(5), 1, 10)
		if err.Code != ErrMissingOperand {
			t.Errorf("Code = %v, want %v", err.Code, ErrMissingOperand)
		}
		if !strings.Contains(err.Message, "'+'") {
			t.Errorf("Message should mention operator: %q", err.Message)
		}
	})

	t.Run("UnclosedDelimiterError", func(t *testing.T) {
		err := UnclosedDelimiterError("(", token.Pos(1), 1, 1)
		if err.Code != ErrUnclosedDelimiter {
			t.Errorf("Code = %v, want %v", err.Code, ErrUnclosedDelimiter)
		}
		if !strings.Contains(err.Message, "'('") {
			t.Errorf("Message should mention delimiter: %q", err.Message)
		}
	})
}

// TestSyncSetContains tests the SyncSet.Contains method
func TestSyncSetContains(t *testing.T) {
	t.Run("StatementSync", func(t *testing.T) {
		if !StatementSync.Contains(tokenizer.SEMICOLON) {
			t.Error("StatementSync should contain SEMICOLON")
		}
		if !StatementSync.Contains(tokenizer.RBRACE) {
			t.Error("StatementSync should contain RBRACE")
		}
		if StatementSync.Contains(tokenizer.LPAREN) {
			t.Error("StatementSync should not contain LPAREN")
		}
	})

	t.Run("ExpressionSync", func(t *testing.T) {
		if !ExpressionSync.Contains(tokenizer.COMMA) {
			t.Error("ExpressionSync should contain COMMA")
		}
		if !ExpressionSync.Contains(tokenizer.RPAREN) {
			t.Error("ExpressionSync should contain RPAREN")
		}
	})

	t.Run("MatchArmSync", func(t *testing.T) {
		if !MatchArmSync.Contains(tokenizer.ARROW) {
			t.Error("MatchArmSync should contain ARROW")
		}
	})

	t.Run("LambdaSync", func(t *testing.T) {
		if !LambdaSync.Contains(tokenizer.PIPE) {
			t.Error("LambdaSync should contain PIPE")
		}
	})
}

// TestGetHintForToken tests hint retrieval for tokens
func TestGetHintForToken(t *testing.T) {
	tests := []struct {
		kind     tokenizer.TokenKind
		wantHint bool
	}{
		{tokenizer.QUESTION, true},
		{tokenizer.ARROW, true},
		{tokenizer.COLON, true},
		{tokenizer.PIPE, true},
		{tokenizer.IDENT, false},
		{tokenizer.INT, false},
	}

	for _, tc := range tests {
		t.Run(tc.kind.String(), func(t *testing.T) {
			hint := GetHintForToken(tc.kind)
			gotHint := hint != ""
			if gotHint != tc.wantHint {
				t.Errorf("GetHintForToken(%v) = %q, wantHint = %v", tc.kind, hint, tc.wantHint)
			}
		})
	}
}

// TestErrorCodes tests that error codes are unique
func TestErrorCodes(t *testing.T) {
	codes := []string{
		ErrUnexpectedToken,
		ErrMissingExpression,
		ErrUnclosedDelimiter,
		ErrAmbiguousQuestion,
		ErrInvalidPattern,
		ErrMissingArrow,
		ErrMissingColon,
		ErrEmptyMatchArms,
		ErrInvalidLambda,
		ErrMissingOperand,
	}

	seen := make(map[string]bool)
	for _, code := range codes {
		if seen[code] {
			t.Errorf("Duplicate error code: %s", code)
		}
		seen[code] = true

		// Verify code format (E followed by 3 digits)
		if len(code) != 4 || code[0] != 'E' {
			t.Errorf("Invalid error code format: %s (expected E###)", code)
		}
	}
}
