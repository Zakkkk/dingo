// Package transpiler provides the core Dingo-to-Go transpilation functionality as a library.
// This allows LSP and other tools to transpile files without shelling out to the CLI.
package transpiler

import (
	"fmt"
	"go/token"
	"os"

	"github.com/MadAppGang/dingo/pkg/config"
)

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

	// Use new AST-based pipeline
	fset := token.NewFileSet()
	result, err := ASTTranspile(src, inputPath, fset)
	if err != nil {
		return fmt.Errorf("transpilation error: %w", err)
	}

	// Write output
	if err := os.WriteFile(outputPath, result.GoCode, 0644); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}

	return nil
}
