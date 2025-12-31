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

// InsertLineDirectivesForEachLine inserts a //line directive before each line
// of the given code, all pointing to the same Dingo position.
//
// This is necessary for multi-line generated code (like error propagation blocks)
// where all lines should map back to the same Dingo source line.
// Without this, Go's //line auto-increment causes subsequent lines to report
// wrong positions.
//
// Example:
//
//	Input code:
//	  tmp, err := foo()
//	  if err != nil {
//	      return err
//	  }
//
//	Output with InsertLineDirectivesForEachLine("foo.dingo", 63, 2, indent, code):
//	  //line foo.dingo:63:2
//	  	tmp, err := foo()
//	  //line foo.dingo:63:2
//	  	if err != nil {
//	  //line foo.dingo:63:2
//	  		return err
//	  //line foo.dingo:63:2
//	  	}
func InsertLineDirectivesForEachLine(filename string, line, col int, indent, code []byte) []byte {
	if filename == "" || line <= 0 || col <= 0 {
		return append(indent, code...)
	}

	directive := FormatLineDirective(filename, line, col)
	if directive == "" {
		return append(indent, code...)
	}

	// Split code into lines
	lines := splitLines(code)
	if len(lines) == 0 {
		return append(indent, code...)
	}

	// Calculate result size for pre-allocation
	directiveBytes := []byte(directive)
	totalSize := 0
	for _, lineContent := range lines {
		// Each line: directive + indent + content + newline
		totalSize += len(directiveBytes) + len(indent) + len(lineContent) + 1
	}

	result := make([]byte, 0, totalSize)
	for i, lineContent := range lines {
		// Skip empty trailing line (from trailing newline in original)
		if i == len(lines)-1 && len(lineContent) == 0 {
			continue
		}
		// Add directive at column 1 (no indent before directive)
		result = append(result, directiveBytes...)
		// Add indent + line content
		result = append(result, indent...)
		result = append(result, lineContent...)
		result = append(result, '\n')
	}

	return result
}

// splitLines splits code into lines, preserving empty lines.
// Does not include the newline characters in the returned slices.
func splitLines(code []byte) [][]byte {
	if len(code) == 0 {
		return nil
	}

	var lines [][]byte
	start := 0
	for i := 0; i < len(code); i++ {
		if code[i] == '\n' {
			lines = append(lines, code[start:i])
			start = i + 1
		}
	}
	// Add remaining content after last newline
	if start < len(code) {
		lines = append(lines, code[start:])
	} else if start == len(code) {
		// Code ended with newline, add empty line marker
		lines = append(lines, []byte{})
	}
	return lines
}
