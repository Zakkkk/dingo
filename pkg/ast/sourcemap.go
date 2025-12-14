package ast

import "strconv"

// CodeGenResult represents the output of a code generation operation.
type CodeGenResult struct {
	Output          []byte        // Generated Go code
	StatementOutput []byte        // Statement-level output (for hoisting)
	HoistedCode     []byte        // Code to hoist before the expression
	Error           *CodeGenError // Error if code generation failed
	LineDirective   string        // //line directive for this generated code (e.g., "//line foo.dingo:42:5\n")
}

// CodeGenError represents an error during code generation.
type CodeGenError struct {
	Message  string
	Pos      int    // Position in source where error occurred
	Position int    // Line/column position (for LSP)
	Hint     string // Optional hint for fixing the error
}

// NewCodeGenResult creates a new CodeGenResult with the given output.
func NewCodeGenResult(output []byte) CodeGenResult {
	return CodeGenResult{
		Output: output,
	}
}

// FormatLineDirective generates a //line directive in Go 1.17+ format.
// Format: //line filename:line:col
// Returns the directive with trailing newline.
//
// Example:
//
//	FormatLineDirective("foo.dingo", 42, 5) → "//line foo.dingo:42:5\n"
func FormatLineDirective(filename string, line, col int) string {
	if filename == "" || line <= 0 || col <= 0 {
		return ""
	}
	// Go 1.17+ format: //line filename:line:col
	// Note: No space after //line
	return "//line " + filename + ":" + strconv.Itoa(line) + ":" + strconv.Itoa(col) + "\n"
}
