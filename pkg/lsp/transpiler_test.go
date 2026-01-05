package lsp

import (
	"fmt"
	"go/scanner"
	"go/token"
	"testing"

	"github.com/MadAppGang/dingo/pkg/transpiler"
	"go.lsp.dev/protocol"
)

func TestParseTranspileError_NilError(t *testing.T) {
	diag := ParseTranspileError("test.dingo", nil)
	if diag != nil {
		t.Errorf("expected nil diagnostic for nil error, got: %+v", diag)
	}
}

func TestParseTranspileError_TranspileError(t *testing.T) {
	err := &transpiler.TranspileError{
		File:    "test.dingo",
		Line:    5,
		Col:     10,
		Message: "unexpected token",
	}

	diag := ParseTranspileError("test.dingo", err)
	if diag == nil {
		t.Fatal("expected diagnostic, got nil")
	}

	// Line should be 0-based (5-1 = 4)
	if diag.Range.Start.Line != 4 {
		t.Errorf("expected line 4 (0-based), got %d", diag.Range.Start.Line)
	}
	if diag.Severity != protocol.DiagnosticSeverityError {
		t.Errorf("expected Error severity, got %v", diag.Severity)
	}
	if diag.Source != "dingo" {
		t.Errorf("expected source 'dingo', got %q", diag.Source)
	}
	if diag.Message != "unexpected token" {
		t.Errorf("expected message 'unexpected token', got %q", diag.Message)
	}
}

func TestParseTranspileError_ScannerErrorList(t *testing.T) {
	// Simulate what pure_pipeline.go does with Go parser errors
	errList := scanner.ErrorList{
		&scanner.Error{
			Pos: token.Position{
				Filename: "test.dingo",
				Line:     2,
				Column:   11,
			},
			Msg: "expected ')', found '{'",
		},
	}
	wrappedErr := fmt.Errorf("parse error: %w", errList)

	diag := ParseTranspileError("test.dingo", wrappedErr)
	if diag == nil {
		t.Fatal("expected diagnostic, got nil")
	}

	// Line should be 0-based (2-1 = 1)
	if diag.Range.Start.Line != 1 {
		t.Errorf("expected line 1 (0-based), got %d", diag.Range.Start.Line)
	}
	// Column should be 0-based (11-1 = 10)
	if diag.Range.Start.Character != 10 {
		t.Errorf("expected character 10 (0-based), got %d", diag.Range.Start.Character)
	}
	if diag.Severity != protocol.DiagnosticSeverityError {
		t.Errorf("expected Error severity, got %v", diag.Severity)
	}
	if diag.Source != "dingo" {
		t.Errorf("expected source 'dingo', got %q", diag.Source)
	}
	if diag.Message != "expected ')', found '{'" {
		t.Errorf("expected message \"expected ')', found '{'\", got %q", diag.Message)
	}
}

func TestParseTranspileError_GenericError(t *testing.T) {
	err := fmt.Errorf("some generic error")

	diag := ParseTranspileError("test.dingo", err)
	if diag == nil {
		t.Fatal("expected diagnostic, got nil")
	}

	// Generic errors should be at line 0
	if diag.Range.Start.Line != 0 {
		t.Errorf("expected line 0 for generic error, got %d", diag.Range.Start.Line)
	}
	if diag.Message != "some generic error" {
		t.Errorf("expected message 'some generic error', got %q", diag.Message)
	}
}

func TestParseTranspileError_WrappedTranspileError(t *testing.T) {
	innerErr := &transpiler.TranspileError{
		File:    "test.dingo",
		Line:    10,
		Col:     5,
		Message: "cannot infer type",
	}
	wrappedErr := fmt.Errorf("AST transform error: %w", innerErr)

	diag := ParseTranspileError("test.dingo", wrappedErr)
	if diag == nil {
		t.Fatal("expected diagnostic, got nil")
	}

	// Should unwrap to the TranspileError and use its position
	if diag.Range.Start.Line != 9 { // 0-based
		t.Errorf("expected line 9 (0-based), got %d", diag.Range.Start.Line)
	}
	if diag.Message != "cannot infer type" {
		t.Errorf("expected message 'cannot infer type', got %q", diag.Message)
	}
}
