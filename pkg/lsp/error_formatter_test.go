package lsp

import (
	"strings"
	"testing"

	"github.com/MadAppGang/dingo/pkg/transpiler"
)

func TestSimpleFormatter_UnresolvedLambda(t *testing.T) {
	err := &transpiler.TranspileError{
		File:    "test.dingo",
		Line:    7,
		Col:     11,
		Message: "fallback message",
		Kind:    transpiler.ErrorKindUnresolvedLambda,
		Data: transpiler.UnresolvedLambdaErrorData{
			ParamNames:   []string{"x"},
			HasAnyReturn: true,
		},
	}

	formatter := &SimpleFormatter{}
	msg := formatter.Format(err)

	if !strings.Contains(msg, "cannot infer type") {
		t.Errorf("expected 'cannot infer type' in message, got: %s", msg)
	}
	if !strings.Contains(msg, "'x, return'") {
		t.Errorf("expected 'x, return' in message, got: %s", msg)
	}
	if strings.Contains(msg, "\n") {
		t.Errorf("simple formatter should produce single line, got: %s", msg)
	}
}

func TestMultilineFormatter_UnresolvedLambda(t *testing.T) {
	err := &transpiler.TranspileError{
		File:    "test.dingo",
		Line:    7,
		Col:     11,
		Message: "fallback message",
		Kind:    transpiler.ErrorKindUnresolvedLambda,
		Data: transpiler.UnresolvedLambdaErrorData{
			ParamNames:   []string{"user"},
			HasAnyReturn: false,
		},
	}

	formatter := &MultilineFormatter{}
	msg := formatter.Format(err)

	if !strings.Contains(msg, "Cannot infer type for 'user'") {
		t.Errorf("expected 'user' in message, got: %s", msg)
	}
	if !strings.Contains(msg, "\n") {
		t.Errorf("multiline formatter should contain newlines, got: %s", msg)
	}
	if !strings.Contains(msg, "Rust style") {
		t.Errorf("expected 'Rust style' in message, got: %s", msg)
	}
	if !strings.Contains(msg, "TypeScript style") {
		t.Errorf("expected 'TypeScript style' in message, got: %s", msg)
	}
	if !strings.Contains(msg, "❌") {
		t.Errorf("expected error emoji in message, got: %s", msg)
	}
}

func TestJetBrainsFormatter_UnresolvedLambda(t *testing.T) {
	err := &transpiler.TranspileError{
		File:    "test.dingo",
		Line:    7,
		Col:     11,
		Message: "fallback message",
		Kind:    transpiler.ErrorKindUnresolvedLambda,
		Data: transpiler.UnresolvedLambdaErrorData{
			ParamNames:   []string{"item"},
			HasAnyReturn: true,
		},
	}

	formatter := &JetBrainsFormatter{}
	msg := formatter.Format(err)

	// JetBrains formatter should produce HTML for hover tooltip
	if !strings.Contains(msg, "<html>") {
		t.Errorf("JetBrains formatter should produce HTML, got: %s", msg)
	}
	if !strings.Contains(msg, "Cannot infer type") {
		t.Errorf("expected 'Cannot infer type' in message, got: %s", msg)
	}
	if !strings.Contains(msg, "item") {
		t.Errorf("expected 'item' in message, got: %s", msg)
	}
	// Should show annotation examples with HTML formatting
	if !strings.Contains(msg, "<b>Rust:</b>") {
		t.Errorf("expected '<b>Rust:</b>' in message, got: %s", msg)
	}
	if !strings.Contains(msg, "<br/>") {
		t.Errorf("expected '<br/>' in message, got: %s", msg)
	}
}

func TestGetFormatterForEditor(t *testing.T) {
	tests := []struct {
		editor   EditorType
		expected string
	}{
		{EditorVSCode, "*lsp.MultilineFormatter"},
		{EditorNeovim, "*lsp.MultilineFormatter"},
		{EditorEmacs, "*lsp.MultilineFormatter"},
		{EditorJetBrains, "*lsp.JetBrainsFormatter"},
		{EditorSublime, "*lsp.SimpleFormatter"},
		{EditorUnknown, "*lsp.SimpleFormatter"},
	}

	for _, tt := range tests {
		formatter := GetFormatterForEditor(tt.editor)

		// Simple type check via type assertion
		switch formatter.(type) {
		case *MultilineFormatter:
			if tt.expected != "*lsp.MultilineFormatter" {
				t.Errorf("editor %d: expected %s, got MultilineFormatter", tt.editor, tt.expected)
			}
		case *JetBrainsFormatter:
			if tt.expected != "*lsp.JetBrainsFormatter" {
				t.Errorf("editor %d: expected %s, got JetBrainsFormatter", tt.editor, tt.expected)
			}
		case *SimpleFormatter:
			if tt.expected != "*lsp.SimpleFormatter" {
				t.Errorf("editor %d: expected %s, got SimpleFormatter", tt.editor, tt.expected)
			}
		default:
			t.Errorf("editor %d: unknown formatter type", tt.editor)
		}
	}
}

func TestFormatter_GenericError(t *testing.T) {
	err := &transpiler.TranspileError{
		File:    "test.dingo",
		Line:    5,
		Col:     10,
		Message: "some generic error",
		Kind:    transpiler.ErrorKindGeneric,
	}

	// All formatters should return something sensible for generic errors
	formatters := []ErrorFormatter{
		&SimpleFormatter{},
		&MultilineFormatter{},
		&JetBrainsFormatter{},
	}

	for _, f := range formatters {
		msg := f.Format(err)
		if msg == "" {
			t.Errorf("formatter returned empty message for generic error")
		}
	}
}

func TestMultilineFormatter_NullCoalesce(t *testing.T) {
	err := &transpiler.TranspileError{
		File:    "test.dingo",
		Line:    10,
		Col:     5,
		Message: "fallback",
		Kind:    transpiler.ErrorKindNullCoalesce,
		Data: transpiler.NullCoalesceErrorData{
			Expression: "x",
		},
	}

	formatter := &MultilineFormatter{}
	msg := formatter.Format(err)

	if !strings.Contains(msg, "??") {
		t.Errorf("expected '??' in message, got: %s", msg)
	}
	if !strings.Contains(msg, "defaultValue") {
		t.Errorf("expected 'defaultValue' in message, got: %s", msg)
	}
	if !strings.Contains(msg, "❌") {
		t.Errorf("expected error emoji in message, got: %s", msg)
	}
}

func TestJetBrainsFormatter_NullCoalesce(t *testing.T) {
	err := &transpiler.TranspileError{
		File:    "test.dingo",
		Line:    10,
		Col:     5,
		Message: "fallback",
		Kind:    transpiler.ErrorKindNullCoalesce,
		Data: transpiler.NullCoalesceErrorData{
			Expression: "x",
		},
	}

	formatter := &JetBrainsFormatter{}
	msg := formatter.Format(err)

	if !strings.Contains(msg, "<html>") {
		t.Errorf("expected HTML, got: %s", msg)
	}
	if !strings.Contains(msg, "??") {
		t.Errorf("expected '??' in message, got: %s", msg)
	}
}

func TestParseErrorMessage(t *testing.T) {
	tests := []struct {
		msg      string
		wantKind transpiler.ErrorKind
	}{
		{"expected ')', found '{'", transpiler.ErrorKindParsing},
		{"expected '}', found EOF", transpiler.ErrorKindParsing},
		{"null coalescing ?? requires value", transpiler.ErrorKindNullCoalesce},
		{"some other error", transpiler.ErrorKindGeneric},
	}

	for _, tt := range tests {
		kind, _ := ParseErrorMessage(tt.msg)
		if kind != tt.wantKind {
			t.Errorf("ParseErrorMessage(%q) = %v, want %v", tt.msg, kind, tt.wantKind)
		}
	}
}

func TestEnhanceGenericMessageMultiline(t *testing.T) {
	tests := []struct {
		msg     string
		wantSub string
	}{
		{"expected ')'", "missing closing parenthesis"},
		{"expected '}'", "missing closing brace"},
		{"undefined: foo", "Check spelling"},
		{"random error", "random error"},
	}

	for _, tt := range tests {
		result := enhanceGenericMessageMultiline(tt.msg)
		if !strings.Contains(result, tt.wantSub) {
			t.Errorf("enhanceGenericMessageMultiline(%q) should contain %q, got: %s", tt.msg, tt.wantSub, result)
		}
		// All should have error emoji
		if !strings.Contains(result, "❌") {
			t.Errorf("enhanceGenericMessageMultiline(%q) should contain ❌, got: %s", tt.msg, result)
		}
	}
}
