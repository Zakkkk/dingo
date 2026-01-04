// Package transpiler provides the core Dingo-to-Go transpilation functionality as a library.
// This allows LSP and other tools to transpile files without shelling out to the CLI.
package transpiler

import (
	"fmt"
	"os"

	"github.com/MadAppGang/dingo/pkg/config"
	"github.com/MadAppGang/dingo/pkg/sourcemap/dmap"
)

// TranspileError represents a structured transpilation error with position information.
// This allows LSP and other tools to display errors at the correct location without
// parsing error message strings.
type TranspileError struct {
	File    string // Source file path
	Line    int    // 1-indexed line number (0 means unknown)
	Col     int    // 1-indexed column number (0 means unknown)
	Message string // Error message
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
