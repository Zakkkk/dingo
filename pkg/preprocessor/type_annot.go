package preprocessor

// TODO(ast-migration): This file uses regex-based transformations which are fragile.
// MIGRATE TO: AST-based type annotation handling
// See: ai-docs/AST_MIGRATION.md for migration plan
// DO NOT fix regex bugs - implement AST-based solution instead

import (
	"bytes"
	"fmt"
	"regexp"
)

// Package-level compiled regex - LEGACY, TO BE REPLACED WITH AST
// IMPORTANT-2 FIX: Enhanced pattern to handle all Go type patterns robustly
// Examples:
//   - Basic: x: int, x: string
//   - Qualified: x: pkg.Type
//   - Pointers: x: *Type
//   - Arrays/Slices: x: []Type, x: [10]int
//   - Maps: x: map[string]int, x: map[string][]interface{}
//   - Channels: x: chan T, x: <-chan string, x: chan<- int
//   - Functions: x: func(int) error, x: func(a, b int) (string, error)
//   - Complex nested: x: map[string][]func() error
// Strategy: Match everything up to next comma or closing paren, handling nested brackets/parens
var (
	paramPattern       = regexp.MustCompile(`(\w+)\s*:\s*([^,)]+)`)
	returnArrowPattern = regexp.MustCompile(`\)\s*->\s*(.+?)\s*\{`)
)

// TypeAnnotProcessor converts Dingo type annotations (: type) to Go syntax (space type)
type TypeAnnotProcessor struct{}

// NewTypeAnnotProcessor creates a new type annotation processor
func NewTypeAnnotProcessor() *TypeAnnotProcessor {
	return &TypeAnnotProcessor{}
}

// Name returns the processor name
func (t *TypeAnnotProcessor) Name() string {
	return "type_annotations"
}

// Process is the legacy interface method (implements FeatureProcessor)
func (t *TypeAnnotProcessor) Process(source []byte) ([]byte, []Mapping, error) {
	result, _, err := t.ProcessInternal(string(source))
	return []byte(result), nil, err
}

// ProcessInternal transforms type annotations with metadata emission support
// Converts: func foo(x: int, y: string) -> error
// To:       func foo(x int, y string) error
func (t *TypeAnnotProcessor) ProcessInternal(code string) (string, []TransformMetadata, error) {
	var metadata []TransformMetadata
	counter := 0

	lines := bytes.Split([]byte(code), []byte("\n"))
	var result bytes.Buffer

	for lineIdx, line := range lines {
		lineNum := lineIdx + 1

		// Check if this line contains type annotations (: pattern)
		// This handles:
		//   - Top-level functions: func name(x: Type) {...}
		//   - Inline lambdas: .Map(func(x: Type) {...})
		//   - Multiple lambdas on same line
		if bytes.Contains(line, []byte("func(")) || bytes.Contains(line, []byte("func ")) {
			// Track if we made any transformations on this line
			hadTransformation := false

			// First handle return type arrow: ) -> Type {  →  ) Type {
			if returnArrowPattern.Match(line) {
				hadTransformation = true
				line = returnArrowPattern.ReplaceAllFunc(line, func(match []byte) []byte {
					submatch := returnArrowPattern.FindSubmatch(match)
					if len(submatch) != 2 {
						return match
					}
					returnType := submatch[1]

					var buf bytes.Buffer
					buf.WriteString(") ")
					buf.Write(returnType)
					buf.WriteString(" {")
					return buf.Bytes()
				})
			}

			// Use regex to replace ALL parameter type annotations on this line
			// This handles multiple func(...) patterns on the same line
			if bytes.Contains(line, []byte(":")) {
				hadTransformation = true
				line = paramPattern.ReplaceAll(line, []byte("$1 $2"))
			}

			// If we made a transformation, emit metadata
			if hadTransformation {
				marker := fmt.Sprintf("// dingo:t:%d", counter)

				// Write transformed line (no marker in output)
				result.Write(line)

				// Metadata only - marker not written to output
				metadata = append(metadata, TransformMetadata{
					Type:            "type_annot",
					OriginalLine:    lineNum,
					OriginalColumn:  1,
					OriginalLength:  len(line),
					OriginalText:    string(line),
					GeneratedMarker: marker,
					ASTNodeType:     "FuncDecl",
				})
				counter++
			} else {
				result.Write(line)
			}
		} else {
			result.Write(line)
		}

		if lineIdx < len(lines)-1 {
			result.WriteByte('\n')
		}
	}

	return result.String(), metadata, nil
}

// replaceColonInParams replaces : with space in function parameters
func (t *TypeAnnotProcessor) replaceColonInParams(params []byte) []byte {
	// Use package-level compiled regex
	return paramPattern.ReplaceAllFunc(params, func(match []byte) []byte {
		parts := bytes.Split(match, []byte(":"))
		if len(parts) != 2 {
			return match
		}

		identifier := bytes.TrimSpace(parts[0])
		typeName := bytes.TrimSpace(parts[1])

		// Reconstruct as: identifier type (space instead of :)
		var buf bytes.Buffer
		buf.Write(identifier)
		buf.WriteByte(' ')
		buf.Write(typeName)

		return buf.Bytes()
	})
}

// ProcessBody implements BodyProcessor interface (for lambda body processing)
func (t *TypeAnnotProcessor) ProcessBody(body []byte) ([]byte, error) {
	result, _, err := t.Process(body)
	return result, err
}
