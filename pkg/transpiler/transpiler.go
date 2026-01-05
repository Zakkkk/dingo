// Package transpiler provides the core Dingo-to-Go transpilation functionality as a library.
// This allows LSP and other tools to transpile files without shelling out to the CLI.
package transpiler

import (
	"fmt"
	"os"

	"github.com/MadAppGang/dingo/pkg/config"
	"github.com/MadAppGang/dingo/pkg/sourcemap/dmap"
)

// ErrorKind identifies the type of transpilation error for structured formatting.
type ErrorKind int

const (
	ErrorKindGeneric ErrorKind = iota
	ErrorKindUnresolvedLambda  // Lambda type inference failed
	ErrorKindParsing           // Syntax/parsing error
	ErrorKindTypeCheck         // Type checking error
	ErrorKindNullCoalesce      // Null coalescing operator error
	ErrorKindSafeNavigation    // Safe navigation error
	ErrorKindMatchExpression   // Match expression error
	ErrorKindEnumDefinition    // Enum definition error
	ErrorKindErrorPropagation  // Error propagation (?) error
)

// UnresolvedLambdaErrorData contains structured data for lambda inference errors.
// The LSP server uses this to format editor-specific messages.
type UnresolvedLambdaErrorData struct {
	ParamNames   []string // Parameter names that couldn't be inferred
	HasAnyReturn bool     // Whether return type couldn't be inferred
}

// ParsingErrorData contains structured data for syntax/parsing errors.
type ParsingErrorData struct {
	Expected string // What was expected (e.g., ")", "}")
	Found    string // What was found instead
	Context  string // Surrounding code context
}

// NullCoalesceErrorData contains structured data for ?? operator errors.
type NullCoalesceErrorData struct {
	Expression string // The expression that had the error
}

// ErrorPropagationErrorData contains structured data for ? operator errors.
type ErrorPropagationErrorData struct {
	Expression   string // The expression
	ExpectedType string // Expected Result/Option type
	ActualType   string // Actual type found
}

// TranspileError represents a structured transpilation error with position information.
// This allows LSP and other tools to display errors at the correct location without
// parsing error message strings.
type TranspileError struct {
	File    string    // Source file path
	Line    int       // 1-indexed line number (0 means unknown)
	Col     int       // 1-indexed column number (0 means unknown)
	Message string    // Fallback message for CLI and simple contexts
	Kind    ErrorKind // Error type for structured formatting
	Data    any       // Type-specific error data (e.g., UnresolvedLambdaErrorData)
}

func (e *TranspileError) Error() string {
	if e.Line > 0 && e.Col > 0 {
		return fmt.Sprintf("%s:%d:%d: %s", e.File, e.Line, e.Col, e.Message)
	}
	if e.Line > 0 {
		return fmt.Sprintf("%s:%d: %s", e.File, e.Line, e.Message)
	}
	return fmt.Sprintf("%s: %s", e.File, e.Message)
}

// Transpiler handles transpilation of .dingo files to .go files
type Transpiler struct {
	config *config.Config
}

// New creates a new Transpiler instance with default configuration
func New() (*Transpiler, error) {
	cfg, err := config.Load(nil)
	if err != nil {
		// Fall back to defaults on error
		cfg = config.DefaultConfig()
	}
	return &Transpiler{
		config: cfg,
	}, nil
}

// NewWithConfig creates a new Transpiler with custom configuration
func NewWithConfig(cfg *config.Config) *Transpiler {
	return &Transpiler{
		config: cfg,
	}
}

// TranspileFile transpiles a single .dingo file to .go
// This is the library equivalent of `dingo build file.dingo`
func (t *Transpiler) TranspileFile(inputPath string) error {
	return t.TranspileFileWithOutput(inputPath, "")
}

// TranspileFileWithOutput transpiles with custom output path using AST-based pipeline
func (t *Transpiler) TranspileFileWithOutput(inputPath, outputPath string) error {
	if outputPath == "" {
		// Default: replace .dingo with .go
		if len(inputPath) > 6 && inputPath[len(inputPath)-6:] == ".dingo" {
			outputPath = inputPath[:len(inputPath)-6] + ".go"
		} else {
			outputPath = inputPath + ".go"
		}
	}

	// Read source
	src, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Use full AST-based pipeline with mappings so we can keep LSP `.dmap` files in sync.
	// This is critical for reliable Go↔Dingo diagnostic mapping in dingo-lsp.
	result, err := PureASTTranspileWithMappings(src, inputPath, true)
	if err != nil {
		return fmt.Errorf("transpilation error: %w", err)
	}

	// Write output
	if err := os.WriteFile(outputPath, result.GoCode, 0o644); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}

	// Write `.dmap` source map (used by dingo-lsp and other tooling).
	// Keep this non-fatal: transpilation output is still valuable without maps.
	if dmapPath, err := calculateDmapPath(inputPath); err == nil {
		writer := dmap.NewWriter(result.DingoSource, result.GoCode)
		// Write v3 format with column mappings
		// Intentionally non-fatal: tooling can still use the `.go` file.
		_ = writer.WriteFile(dmapPath, result.LineMappings, result.ColumnMappings)
	}

	return nil
}
