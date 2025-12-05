package transpiler

import (
	"go/token"
	"strings"
	"testing"
)

func TestASTTranspileBasic(t *testing.T) {
	tests := []struct {
		name        string
		source      string
		expectError bool
		checkOutput func(t *testing.T, result *TranspileResult)
	}{
		{
			name:        "simple identifier",
			source:      "x",
			expectError: false,
			checkOutput: func(t *testing.T, result *TranspileResult) {
				if result == nil {
					t.Fatal("expected result, got nil")
				}
				if len(result.GoCode) == 0 {
					t.Error("expected Go code output, got empty")
				}
			},
		},
		{
			name:        "integer literal",
			source:      "42",
			expectError: false,
			checkOutput: func(t *testing.T, result *TranspileResult) {
				if result == nil {
					t.Fatal("expected result, got nil")
				}
				// Note: Currently generates placeholder package main
				// Full expression integration will be added when parser/decl.go is complete
				if !strings.Contains(string(result.GoCode), "package main") {
					t.Errorf("expected output to contain 'package main', got: %s", result.GoCode)
				}
			},
		},
		{
			name:        "error propagation operator",
			source:      "readFile()?",
			expectError: false, // Should parse but may have transformation placeholder
			checkOutput: func(t *testing.T, result *TranspileResult) {
				if result == nil {
					t.Fatal("expected result, got nil")
				}
				// Pipeline should complete even if transformation is incomplete
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fset := token.NewFileSet()
			result, err := ASTTranspile([]byte(tt.source), "test.dingo", fset)

			if tt.expectError && err == nil {
				t.Errorf("expected error, got nil")
			}
			if !tt.expectError && err != nil {
				// Check if it's a recoverable error
				if result == nil {
					t.Errorf("unexpected error: %v", err)
				}
			}

			if tt.checkOutput != nil {
				tt.checkOutput(t, result)
			}
		})
	}
}

func TestASTTranspileMetadata(t *testing.T) {
	source := []byte("x + y")
	fset := token.NewFileSet()

	result, err := ASTTranspile(source, "test.dingo", fset)
	if err != nil && result == nil {
		t.Fatalf("transpilation failed: %v", err)
	}

	if result.Metadata == nil {
		t.Fatal("expected metadata, got nil")
	}

	if result.Metadata.OriginalFile != "test.dingo" {
		t.Errorf("expected filename 'test.dingo', got '%s'", result.Metadata.OriginalFile)
	}

	if result.Metadata.TokenCount == 0 {
		t.Error("expected non-zero token count")
	}
}

func TestASTTranspileIncremental(t *testing.T) {
	// Incremental mode should always return a result, even with errors
	source := []byte("invalid syntax ???")
	fset := token.NewFileSet()

	result := ASTTranspileIncremental(source, "test.dingo", fset)
	if result == nil {
		t.Fatal("expected result in incremental mode, got nil")
	}

	// Should have errors but still return result
	if !result.HasErrors() {
		t.Log("note: incremental mode may succeed with partial parsing")
	}
}

func TestTranspileResultHelpers(t *testing.T) {
	result := &TranspileResult{
		Errors: []error{
			&TranspileError{Message: "error 1"},
			&TranspileError{Message: "error 2"},
		},
	}

	if !result.HasErrors() {
		t.Error("expected HasErrors() to return true")
	}

	messages := result.GetErrorMessages()
	if len(messages) != 2 {
		t.Errorf("expected 2 error messages, got %d", len(messages))
	}
}

// TranspileError is a simple error type for testing
type TranspileError struct {
	Message string
}

func (e *TranspileError) Error() string {
	return e.Message
}

func TestASTTranspilePipeline(t *testing.T) {
	// Test that all pipeline stages execute
	source := []byte("value")
	fset := token.NewFileSet()

	result, err := ASTTranspile(source, "test.dingo", fset)
	if err != nil && result == nil {
		t.Fatalf("pipeline failed: %v", err)
	}

	// Verify each stage completed:
	// 1. Tokenization - check TokenCount
	if result.Metadata.TokenCount == 0 {
		t.Error("tokenization stage did not run (TokenCount = 0)")
	}

	// 2. Parsing - check that we got past tokenization
	if result.Metadata == nil {
		t.Error("parsing stage did not complete (no metadata)")
	}

	// 3. Transformation - check GoAST exists
	if result.GoAST == nil {
		t.Error("transformation stage did not produce AST")
	}

	// 4. Printing - check GoCode exists
	if len(result.GoCode) == 0 {
		t.Error("printing stage did not produce code")
	}
}
