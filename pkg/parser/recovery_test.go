package parser

import (
	"go/token"
	"strings"
	"testing"

	"github.com/MadAppGang/dingo/pkg/tokenizer"
)

func TestParseError_Error(t *testing.T) {
	tests := []struct {
		name    string
		err     ParseError
		wantMsg string
	}{
		{
			name: "error with position",
			err: ParseError{
				Pos:     100,
				Line:    10,
				Column:  5,
				Message: "unexpected token",
			},
			wantMsg: "parse error at 10:5: unexpected token",
		},
		{
			name: "error at line 1",
			err: ParseError{
				Pos:     1,
				Line:    1,
				Column:  1,
				Message: "missing semicolon",
			},
			wantMsg: "parse error at 1:1: missing semicolon",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.wantMsg {
				t.Errorf("Error() = %q, want %q", got, tt.wantMsg)
			}
		})
	}
}

func TestErrorList_Add(t *testing.T) {
	var list ErrorList

	list.Add(1, 1, 1, "first error")
	list.Add(2, 2, 1, "second error")

	if len(list) != 2 {
		t.Errorf("len(list) = %d, want 2", len(list))
	}

	if list[0].Message != "first error" {
		t.Errorf("list[0].Message = %q, want %q", list[0].Message, "first error")
	}

	if list[1].Message != "second error" {
		t.Errorf("list[1].Message = %q, want %q", list[1].Message, "second error")
	}
}

func TestErrorList_Error(t *testing.T) {
	tests := []struct {
		name      string
		errors    ErrorList
		wantEmpty bool
		wantCount int
	}{
		{
			name:      "empty list",
			errors:    ErrorList{},
			wantEmpty: true,
			wantCount: 0,
		},
		{
			name: "single error",
			errors: ErrorList{
				{Pos: 1, Line: 1, Column: 1, Message: "error 1"},
			},
			wantEmpty: false,
			wantCount: 1,
		},
		{
			name: "multiple errors",
			errors: ErrorList{
				{Pos: 1, Line: 1, Column: 1, Message: "error 1"},
				{Pos: 2, Line: 2, Column: 1, Message: "error 2"},
				{Pos: 3, Line: 3, Column: 1, Message: "error 3"},
			},
			wantEmpty: false,
			wantCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := tt.errors.Error()

			if tt.wantEmpty {
				if msg != "" {
					t.Errorf("Error() = %q, want empty string", msg)
				}
			} else {
				if !strings.Contains(msg, "error(s)") {
					t.Errorf("Error() missing 'error(s)': %q", msg)
				}
				// Check that error count is mentioned
				if !strings.Contains(msg, string(rune('0'+tt.wantCount))) {
					t.Errorf("Error() doesn't mention count %d: %q", tt.wantCount, msg)
				}
			}
		})
	}
}

func TestErrorList_Err(t *testing.T) {
	emptyList := ErrorList{}
	if err := emptyList.Err(); err != nil {
		t.Errorf("empty list Err() = %v, want nil", err)
	}

	nonEmptyList := ErrorList{
		{Pos: 1, Line: 1, Column: 1, Message: "error"},
	}
	if err := nonEmptyList.Err(); err == nil {
		t.Errorf("non-empty list Err() = nil, want error")
	}
}

func TestRecoveryContext_ShouldContinue(t *testing.T) {
	tests := []struct {
		name       string
		maxErrors  int
		errorCount int
		want       bool
	}{
		{
			name:       "no errors, limit 10",
			maxErrors:  10,
			errorCount: 0,
			want:       true,
		},
		{
			name:       "5 errors, limit 10",
			maxErrors:  10,
			errorCount: 5,
			want:       true,
		},
		{
			name:       "10 errors, limit 10",
			maxErrors:  10,
			errorCount: 10,
			want:       false,
		},
		{
			name:       "unlimited errors",
			maxErrors:  0,
			errorCount: 1000,
			want:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &RecoveryContext{
				MaxErrors: tt.maxErrors,
				Errors:    make(ErrorList, tt.errorCount),
			}

			if got := ctx.ShouldContinue(); got != tt.want {
				t.Errorf("ShouldContinue() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRecoveryHelper_Synchronize(t *testing.T) {
	tests := []struct {
		name           string
		tokens         []tokenizer.Token
		startPos       int
		recoveryPoints []token.Token
		wantSkipped    int
		wantFinalPos   int
	}{
		{
			name: "find semicolon immediately",
			tokens: []tokenizer.Token{
				{Kind: tokenizer.IDENT},
				{Kind: tokenizer.SEMICOLON},
				{Kind: tokenizer.IDENT}, // RETURN not in tokenizer yet
			},
			startPos:       0,
			recoveryPoints: []token.Token{token.SEMICOLON},
			wantSkipped:    1,
			wantFinalPos:   1,
		},
		{
			name: "skip to const declaration",
			tokens: []tokenizer.Token{
				{Kind: tokenizer.IDENT},
				{Kind: tokenizer.ILLEGAL},
				{Kind: tokenizer.ILLEGAL},
				{Kind: tokenizer.CONST}, // CONST exists in tokenizer
				{Kind: tokenizer.IDENT},
			},
			startPos:       0,
			recoveryPoints: []token.Token{token.CONST},
			wantSkipped:    3,
			wantFinalPos:   3,
		},
		{
			name: "no recovery point found",
			tokens: []tokenizer.Token{
				{Kind: tokenizer.IDENT},
				{Kind: tokenizer.IDENT},
				{Kind: tokenizer.IDENT},
			},
			startPos:       0,
			recoveryPoints: []token.Token{token.SEMICOLON},
			wantSkipped:    3,
			wantFinalPos:   3,
		},
		{
			name: "multiple recovery points, find first",
			tokens: []tokenizer.Token{
				{Kind: tokenizer.IDENT},
				{Kind: tokenizer.ILLEGAL},
				{Kind: tokenizer.SEMICOLON},
				{Kind: tokenizer.CONST}, // CONST exists in tokenizer
			},
			startPos:       0,
			recoveryPoints: []token.Token{token.SEMICOLON, token.CONST},
			wantSkipped:    2,
			wantFinalPos:   2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			helper := &RecoveryHelper{
				tokens:  tt.tokens,
				pos:     tt.startPos,
				fset:    token.NewFileSet(),
				context: NewRecoveryContext(),
			}

			skipped := helper.Synchronize(tt.recoveryPoints)

			if skipped != tt.wantSkipped {
				t.Errorf("Synchronize() skipped = %d, want %d", skipped, tt.wantSkipped)
			}

			if helper.pos != tt.wantFinalPos {
				t.Errorf("final pos = %d, want %d", helper.pos, tt.wantFinalPos)
			}
		})
	}
}

func TestRecoveryHelper_SynchronizeToStatement(t *testing.T) {
	tokens := []tokenizer.Token{
		{Kind: tokenizer.IDENT},
		{Kind: tokenizer.ILLEGAL},
		{Kind: tokenizer.ILLEGAL},
		{Kind: tokenizer.IF},
		{Kind: tokenizer.LPAREN},
	}

	helper := &RecoveryHelper{
		tokens:  tokens,
		pos:     0,
		fset:    token.NewFileSet(),
		context: NewRecoveryContext(),
	}

	skipped := helper.SynchronizeToStatement()

	if skipped != 3 {
		t.Errorf("SynchronizeToStatement() skipped = %d, want 3", skipped)
	}

	if helper.pos != 3 {
		t.Errorf("pos = %d, want 3 (at IF token)", helper.pos)
	}
}

func TestRecoveryHelper_SynchronizeToDeclaration(t *testing.T) {
	tokens := []tokenizer.Token{
		{Kind: tokenizer.IDENT},
		{Kind: tokenizer.ILLEGAL},
		{Kind: tokenizer.ILLEGAL},
		{Kind: tokenizer.VAR}, // VAR exists in tokenizer
		{Kind: tokenizer.IDENT},
	}

	helper := &RecoveryHelper{
		tokens:  tokens,
		pos:     0,
		fset:    token.NewFileSet(),
		context: NewRecoveryContext(),
	}

	skipped := helper.SynchronizeToDeclaration()

	if skipped != 3 {
		t.Errorf("SynchronizeToDeclaration() skipped = %d, want 3", skipped)
	}

	if helper.pos != 3 {
		t.Errorf("pos = %d, want 3 (at VAR token)", helper.pos)
	}
}

func TestRecoveryHelper_AddError(t *testing.T) {
	fset := token.NewFileSet()
	file := fset.AddFile("test.dingo", -1, 100)

	helper := &RecoveryHelper{
		tokens:  []tokenizer.Token{{Kind: tokenizer.IDENT}},
		pos:     0,
		fset:    fset,
		context: NewRecoveryContext(),
	}

	pos := file.Pos(10)
	helper.AddError(pos, "test error")

	if len(helper.context.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(helper.context.Errors))
	}

	err := helper.context.Errors[0]
	if err.Message != "test error" {
		t.Errorf("Message = %q, want %q", err.Message, "test error")
	}
}

func TestRecoveryHelper_IsAtRecoveryPoint(t *testing.T) {
	tokens := []tokenizer.Token{
		{Kind: tokenizer.IDENT},
		{Kind: tokenizer.SEMICOLON},
		{Kind: tokenizer.VAR}, // VAR exists in tokenizer
	}

	helper := &RecoveryHelper{
		tokens:  tokens,
		pos:     1,
		fset:    token.NewFileSet(),
		context: NewRecoveryContext(),
	}

	recoveryPoints := []token.Token{token.SEMICOLON, token.VAR}

	if !helper.IsAtRecoveryPoint(recoveryPoints) {
		t.Errorf("IsAtRecoveryPoint() = false, want true (at SEMICOLON)")
	}

	helper.pos = 0
	if helper.IsAtRecoveryPoint(recoveryPoints) {
		t.Errorf("IsAtRecoveryPoint() = true, want false (at IDENT)")
	}

	helper.pos = 2
	if !helper.IsAtRecoveryPoint(recoveryPoints) {
		t.Errorf("IsAtRecoveryPoint() = false, want true (at VAR)")
	}
}

func TestRecoveryHelper_TryParse(t *testing.T) {
	tests := []struct {
		name          string
		parseFunc     ParseFunc
		recoverTo     RecoveryStrategy
		wantRecovered bool
		wantErrors    int
	}{
		{
			name: "successful parse",
			parseFunc: func() (interface{}, error) {
				return "success", nil
			},
			wantRecovered: false,
			wantErrors:    0,
		},
		{
			name: "failed parse with recovery",
			parseFunc: func() (interface{}, error) {
				return nil, ParseError{
					Pos:     1,
					Line:    1,
					Column:  1,
					Message: "syntax error",
				}
			},
			recoverTo:     RecoverToStatement,
			wantRecovered: true,
			wantErrors:    1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := []tokenizer.Token{
				{Kind: tokenizer.ILLEGAL},
				{Kind: tokenizer.SEMICOLON},
				{Kind: tokenizer.IDENT},
			}

			helper := &RecoveryHelper{
				tokens:  tokens,
				pos:     0,
				fset:    token.NewFileSet(),
				context: NewRecoveryContext(),
			}

			result, recovered := helper.TryParse(tt.parseFunc, tt.recoverTo)

			if recovered != tt.wantRecovered {
				t.Errorf("recovered = %v, want %v", recovered, tt.wantRecovered)
			}

			if len(helper.context.Errors) != tt.wantErrors {
				t.Errorf("error count = %d, want %d", len(helper.context.Errors), tt.wantErrors)
			}

			if !tt.wantRecovered && result == nil {
				t.Errorf("successful parse returned nil result")
			}
		})
	}
}

func TestRecoveryPoints_Defined(t *testing.T) {
	// Ensure recovery points are properly defined
	if len(StatementRecoveryPoints) == 0 {
		t.Error("StatementRecoveryPoints is empty")
	}

	if len(DeclarationRecoveryPoints) == 0 {
		t.Error("DeclarationRecoveryPoints is empty")
	}

	// Check specific tokens exist
	foundSemicolon := false
	for _, tok := range StatementRecoveryPoints {
		if tok == token.SEMICOLON {
			foundSemicolon = true
			break
		}
	}
	if !foundSemicolon {
		t.Error("StatementRecoveryPoints missing SEMICOLON")
	}

	foundFunc := false
	for _, tok := range DeclarationRecoveryPoints {
		if tok == token.FUNC {
			foundFunc = true
			break
		}
	}
	if !foundFunc {
		t.Error("DeclarationRecoveryPoints missing FUNC")
	}
}
