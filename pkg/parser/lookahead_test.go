package parser

import (
	"testing"

	"github.com/MadAppGang/dingo/pkg/tokenizer"
)

// TestClassifyQuestionOperator tests the question operator classification
func TestClassifyQuestionOperator(t *testing.T) {
	tests := []struct {
		name     string
		src      string // Token stream after the ? (parser starts on first token)
		wantKind questionKind
	}{
		// Error propagation postfix patterns
		{"terminator_rparen", ")", qkErrorPropPostfix},
		{"terminator_semicolon", ";", qkErrorPropPostfix},
		{"terminator_comma", ",", qkErrorPropPostfix},
		{"terminator_rbrace", "}", qkErrorPropPostfix},
		{"terminator_eof", "", qkErrorPropPostfix},
		{"chained_question", "?", qkErrorPropPostfix},

		// Error with context (string without colon)
		{"string_context", `"error message"`, qkErrorWithContext},
		{"string_context_raw", "`raw string`", qkErrorWithContext},

		// Error with Rust-style lambda
		{"rust_lambda", "|e| wrap(e)", qkErrorWithRustLambda},

		// Error with TS-style lambda
		{"ts_lambda_parens", "(e) => wrap(e)", qkErrorWithTSLambda},
		{"ts_lambda_single", "e => wrap(e)", qkErrorWithTSLambda},
		{"ts_lambda_multi", "(a, b) => a + b", qkErrorWithTSLambda},

		// Ternary patterns (have colon after expression)
		{"ternary_simple", "a : b", qkTernary},
		{"ternary_string", `"yes" : "no"`, qkTernary},
		{"ternary_number", "1 : 0", qkTernary},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tok := tokenizer.New([]byte(tc.src))
			p := NewPrattParser(tok)

			result := p.classifyQuestionOperator()

			if result.kind != tc.wantKind {
				t.Errorf("classifyQuestionOperator() = %v, want %v", result.kind, tc.wantKind)
			}
		})
	}
}

// TestHasTernaryColon tests the ternary colon detection
func TestHasTernaryColon(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want bool
	}{
		{"simple_colon", "x : y", true},
		{"nested_paren", "(a, b, c) : d", true},
		{"nested_brace", "{a: 1} : d", true},
		{"nested_bracket", "[1, 2] : d", true},
		{"no_colon", "x + y", false},
		{"colon_inside_nested", "(a : b) + c", false}, // colon is nested
		{"semicolon_before_colon", "x; y : z", false}, // semicolon terminates before colon
		{"deep_nesting", "((a, b), c) : d", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tok := tokenizer.New([]byte(tc.src))
			p := NewPrattParser(tok)

			got := p.hasTernaryColon()

			if got != tc.want {
				t.Errorf("hasTernaryColon() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestHasColonAfterToken tests single-token colon detection
func TestHasColonAfterToken(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want bool
	}{
		{"has_colon", `"text" :`, true},
		{"no_colon", `"text" x`, false},
		{"newline_then_colon", "\"text\"\n:", true},
		{"eof", `"text"`, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tok := tokenizer.New([]byte(tc.src))
			p := NewPrattParser(tok)

			got := p.hasColonAfterToken()

			if got != tc.want {
				t.Errorf("hasColonAfterToken() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestClassifyTSLambda tests the detailed TypeScript lambda classification
func TestClassifyTSLambda(t *testing.T) {
	tests := []struct {
		name       string
		src        string
		wantClass  lambdaClassification
		wantLambda bool
	}{
		{"empty_params", "() =>", lcEmptyParams, true},
		{"single_param", "(x) =>", lcSingleParam, true},
		{"single_typed_param_colon", "(x: int) =>", lcSingleTypedParam, true},
		{"single_typed_param_space", "(x int) =>", lcSingleTypedParam, true},
		{"multi_param", "(x, y) =>", lcMultiParam, true},
		{"multi_typed_param", "(x: int, y: int) =>", lcMultiTypedParam, true},
		{"not_lambda_paren_expr", "(a + b)", lcNotLambda, false},
		{"not_lambda_no_arrow", "(x)", lcNotLambda, false},
		{"not_lambda_ident", "x =>", lcNotLambda, false}, // starts with IDENT, not LPAREN
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tok := tokenizer.New([]byte(tc.src))
			p := NewPrattParser(tok)

			gotClass, gotLambda := p.classifyTSLambda()

			if gotClass != tc.wantClass {
				t.Errorf("classifyTSLambda() class = %v, want %v", gotClass, tc.wantClass)
			}
			if gotLambda != tc.wantLambda {
				t.Errorf("classifyTSLambda() isLambda = %v, want %v", gotLambda, tc.wantLambda)
			}
		})
	}
}

// TestQuestionKindString tests the string representation
func TestQuestionKindString(t *testing.T) {
	tests := []struct {
		kind questionKind
		want string
	}{
		{qkUnknown, "unknown"},
		{qkErrorPropPostfix, "error_prop_postfix"},
		{qkErrorWithContext, "error_prop_context"},
		{qkErrorWithRustLambda, "error_prop_rust_lambda"},
		{qkErrorWithTSLambda, "error_prop_ts_lambda"},
		{qkTernary, "ternary"},
	}

	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			if got := tc.kind.String(); got != tc.want {
				t.Errorf("questionKind.String() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestLambdaClassificationString tests the string representation
func TestLambdaClassificationString(t *testing.T) {
	tests := []struct {
		class lambdaClassification
		want  string
	}{
		{lcNotLambda, "not_lambda"},
		{lcEmptyParams, "empty_params"},
		{lcSingleParam, "single_param"},
		{lcSingleTypedParam, "single_typed_param"},
		{lcMultiParam, "multi_param"},
		{lcMultiTypedParam, "multi_typed_param"},
		{lcWithReturnType, "with_return_type"},
	}

	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			if got := tc.class.String(); got != tc.want {
				t.Errorf("lambdaClassification.String() = %v, want %v", got, tc.want)
			}
		})
	}
}
