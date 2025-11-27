// Package transpiler provides the core Dingo-to-Go transpilation functionality as a library.
// This allows LSP and other tools to transpile files without shelling out to the CLI.
package transpiler

import (
	"encoding/json"
	"fmt"
	"go/token"
	"os"
	"path/filepath"

	"github.com/MadAppGang/dingo/pkg/config"
	"github.com/MadAppGang/dingo/pkg/generator"
	"github.com/MadAppGang/dingo/pkg/parser"
	"github.com/MadAppGang/dingo/pkg/plugin"
	"github.com/MadAppGang/dingo/pkg/plugin/builtin"
	"github.com/MadAppGang/dingo/pkg/preprocessor"
	"github.com/MadAppGang/dingo/pkg/sourcemap"
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
	return &Transpiler{config: cfg}, nil
}

// NewWithConfig creates a new Transpiler with custom configuration
func NewWithConfig(cfg *config.Config) *Transpiler {
	return &Transpiler{config: cfg}
}

// TranspileFile transpiles a single .dingo file to .go with source maps
// This is the library equivalent of `dingo build file.dingo`
func (t *Transpiler) TranspileFile(inputPath string) error {
	return t.TranspileFileWithOutput(inputPath, "")
}

// TranspileFileWithOutput transpiles with custom output path
func (t *Transpiler) TranspileFileWithOutput(inputPath, outputPath string) error {
	if outputPath == "" {
		// Default: replace .dingo with .go
		if len(inputPath) > 6 && inputPath[len(inputPath)-6:] == ".dingo" {
			outputPath = inputPath[:len(inputPath)-6] + ".go"
		} else {
			outputPath = inputPath + ".go"
		}
	}

	// Step 1: Read source
	src, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Step 2: Preprocess
	var goSource string
	var metadata []preprocessor.TransformMetadata

	pkgDir := filepath.Dir(inputPath)
	cache := preprocessor.NewFunctionExclusionCache(pkgDir)
	err = cache.ScanPackage([]string{inputPath})

	if err != nil {
		// Fall back to no cache if scanning fails
		prep := preprocessor.NewWithMainConfig(src, t.config)
		var legacyMap *preprocessor.SourceMap
		goSource, legacyMap, metadata, err = prep.ProcessWithMetadata()
		_ = legacyMap // Discard legacy map - Phase 3 uses PostASTGenerator
		if err != nil {
			return fmt.Errorf("preprocessing error: %w", err)
		}
	} else {
		// Cache scan successful
		prep := preprocessor.NewWithCache(src, cache)
		var legacyMap *preprocessor.SourceMap
		goSource, legacyMap, metadata, err = prep.ProcessWithMetadata()
		_ = legacyMap
		if err != nil {
			return fmt.Errorf("preprocessing error: %w", err)
		}
	}

	// Step 3: Parse preprocessed Go
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, inputPath, []byte(goSource), parser.ParseComments)
	if err != nil {
		return fmt.Errorf("parse error: %w", err)
	}

	// Step 4: Setup plugins
	registry, err := builtin.NewDefaultRegistry()
	if err != nil {
		return fmt.Errorf("failed to setup plugins: %w", err)
	}

	// Step 5: Generate with plugins
	// IMPORTANT: Always keep markers during generation - they're needed for source maps
	// Marker cleanup happens AFTER source map generation (Step 8)
	logger := plugin.NewNoOpLogger() // Silent logger for library use
	gen, err := generator.NewWithPlugins(fset, registry, logger)
	if err != nil {
		return fmt.Errorf("failed to create generator: %w", err)
	}

	// Keep markers during generation - cleanup happens after source map generation
	gen.SetKeepMarkers(true)

	outputCode, err := gen.Generate(file)
	if err != nil {
		return fmt.Errorf("generation error: %w", err)
	}

	// Step 6: Write .go file WITH markers (needed for source map generation)
	if err := os.WriteFile(outputPath, outputCode, 0644); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}

	// Step 7: Generate source map (reads file with markers)
	sourceMapPath := outputPath + ".map"
	sourceMap, err := sourcemap.GenerateFromFiles(inputPath, outputPath, metadata)
	if err != nil {
		// Non-fatal: just skip source map
		return nil
	}

	// Write source map
	sourceMapJSON, err := json.MarshalIndent(sourceMap, "", "  ")
	if err != nil {
		return nil // Non-fatal
	}

	if err := os.WriteFile(sourceMapPath, sourceMapJSON, 0644); err != nil {
		return nil // Non-fatal
	}

	// Step 8: Clean markers from .go file if configured (default: clean)
	// This happens AFTER source map generation so markers are available for mapping
	if !t.config.Debug.KeepMarkers {
		cleanedCode := generator.RemoveDebugMarkers(outputCode)
		if err := os.WriteFile(outputPath, cleanedCode, 0644); err != nil {
			// Non-fatal: file was already written with markers
			return nil
		}
	}

	return nil
}
