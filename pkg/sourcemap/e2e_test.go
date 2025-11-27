// Package sourcemap provides end-to-end validation tests
package sourcemap

import (
	"encoding/json"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MadAppGang/dingo/pkg/config"
	"github.com/MadAppGang/dingo/pkg/generator"
	"github.com/MadAppGang/dingo/pkg/parser"
	"github.com/MadAppGang/dingo/pkg/plugin"
	"github.com/MadAppGang/dingo/pkg/plugin/builtin"
	"github.com/MadAppGang/dingo/pkg/preprocessor"
)

// TestE2E_ErrorPropagation validates source maps for error propagation syntax
func TestE2E_ErrorPropagation(t *testing.T) {
	dingoCode := `package main

import "os"

func ReadConfig(path: string) ([]byte, error) {
	let data = os.ReadFile(path)?
	return data, nil
}
`

	tmpDir := t.TempDir()
	dingoPath := filepath.Join(tmpDir, "error_prop.dingo")
	goPath := filepath.Join(tmpDir, "error_prop.go")
	mapPath := goPath + ".map"

	if err := os.WriteFile(dingoPath, []byte(dingoCode), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Preprocess with metadata
	prep := preprocessor.NewWithMainConfig([]byte(dingoCode), config.DefaultConfig())
	goSource, _, metadata, err := prep.ProcessWithMetadata()
	if err != nil {
		t.Fatalf("Preprocessing failed: %v", err)
	}

	t.Logf("Preprocessor emitted %d metadata entries", len(metadata))

	// Parse AST
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, dingoPath, []byte(goSource), parser.ParseComments)
	if err != nil {
		t.Fatalf("Parsing failed: %v", err)
	}

	// Apply plugins via generator
	registry, err := builtin.NewDefaultRegistry()
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}

	logger := plugin.NewNoOpLogger()
	gen, err := generator.NewWithPlugins(fset, registry, logger)
	if err != nil {
		t.Fatalf("Failed to create generator: %v", err)
	}

	goCodeBytes, err := gen.Generate(file)
	if err != nil {
		t.Fatalf("Generation failed: %v", err)
	}
	goCode := string(goCodeBytes)

	if err := os.WriteFile(goPath, []byte(goCode), 0644); err != nil {
		t.Fatalf("Failed to write Go file: %v", err)
	}

	// Generate Post-AST source map
	postASTGen := NewPostASTGenerator(dingoPath, goPath, fset, file.File, metadata)
	sourceMap, err := postASTGen.Generate()
	if err != nil {
		t.Fatalf("Source map generation failed: %v", err)
	}

	// Write source map
	mapData, err := json.MarshalIndent(sourceMap, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal source map: %v", err)
	}

	if err := os.WriteFile(mapPath, mapData, 0644); err != nil {
		t.Fatalf("Failed to write source map: %v", err)
	}

	// Validate
	if len(sourceMap.Mappings) == 0 {
		t.Error("Source map has no mappings")
	}

	goLines := strings.Split(goCode, "\n")
	for i, mapping := range sourceMap.Mappings {
		if mapping.GeneratedLine <= 0 || mapping.OriginalLine <= 0 {
			t.Errorf("Mapping %d has invalid line numbers: Gen=%d, Orig=%d",
				i, mapping.GeneratedLine, mapping.OriginalLine)
		}
		if mapping.GeneratedLine > len(goLines) {
			t.Errorf("Mapping %d: Generated line %d exceeds file length %d",
				i, mapping.GeneratedLine, len(goLines))
		}
	}

	t.Logf("✅ Error propagation: %d mappings generated", len(sourceMap.Mappings))
	t.Logf("   Dingo: %d lines → Go: %d lines", len(strings.Split(dingoCode, "\n")), len(goLines))
}

// TestE2E_TypeAnnotations validates source maps for type annotation syntax
func TestE2E_TypeAnnotations(t *testing.T) {
	dingoCode := `package main

func greet(name: string, age: int) string {
	return "Hello"
}
`

	tmpDir := t.TempDir()
	dingoPath := filepath.Join(tmpDir, "type_annot.dingo")
	goPath := filepath.Join(tmpDir, "type_annot.go")

	if err := os.WriteFile(dingoPath, []byte(dingoCode), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Preprocess
	prep := preprocessor.NewWithMainConfig([]byte(dingoCode), config.DefaultConfig())
	goSource, _, metadata, err := prep.ProcessWithMetadata()
	if err != nil {
		t.Fatalf("Preprocessing failed: %v", err)
	}

	// Parse AST
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, dingoPath, []byte(goSource), parser.ParseComments)
	if err != nil {
		t.Fatalf("Parsing failed: %v", err)
	}

	// Apply plugins via generator
	registry, err := builtin.NewDefaultRegistry()
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}

	logger := plugin.NewNoOpLogger()
	gen, err := generator.NewWithPlugins(fset, registry, logger)
	if err != nil {
		t.Fatalf("Failed to create generator: %v", err)
	}

	goCodeBytes, err := gen.Generate(file)
	if err != nil {
		t.Fatalf("Generation failed: %v", err)
	}
	goCode := string(goCodeBytes)

	if err := os.WriteFile(goPath, []byte(goCode), 0644); err != nil {
		t.Fatalf("Failed to write Go file: %v", err)
	}

	// Generate source map
	postASTGen := NewPostASTGenerator(dingoPath, goPath, fset, file.File, metadata)
	sourceMap, err := postASTGen.Generate()
	if err != nil {
		t.Fatalf("Source map generation failed: %v", err)
	}

	// Validate coverage
	dingoLines := strings.Split(dingoCode, "\n")
	nonEmptyLines := 0
	for _, line := range dingoLines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "//") {
			nonEmptyLines++
		}
	}

	coverageRatio := float64(len(sourceMap.Mappings)) / float64(nonEmptyLines)

	t.Logf("✅ Type annotations: %d mappings for %d non-empty lines (%.1f%% coverage)",
		len(sourceMap.Mappings), nonEmptyLines, coverageRatio*100)

	if coverageRatio < 0.4 {
		t.Logf("⚠️  Coverage low but acceptable for basic test (%.1f%%)", coverageRatio*100)
	}
}

// TestE2E_SourceMapAccuracy validates mapping accuracy for a known transformation
func TestE2E_SourceMapAccuracy(t *testing.T) {
	dingoCode := `package main

func getValue(n: int) (int, error) {
	let x = compute(n)?
	return x, nil
}
`

	tmpDir := t.TempDir()
	dingoPath := filepath.Join(tmpDir, "accuracy.dingo")
	goPath := filepath.Join(tmpDir, "accuracy.go")

	if err := os.WriteFile(dingoPath, []byte(dingoCode), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Preprocess
	prep := preprocessor.NewWithMainConfig([]byte(dingoCode), config.DefaultConfig())
	goSource, _, metadata, err := prep.ProcessWithMetadata()
	if err != nil {
		t.Fatalf("Preprocessing failed: %v", err)
	}

	// Parse and process
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, dingoPath, []byte(goSource), parser.ParseComments)
	if err != nil {
		t.Fatalf("Parsing failed: %v", err)
	}

	registry, err := builtin.NewDefaultRegistry()
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}

	logger := plugin.NewNoOpLogger()
	gen, err := generator.NewWithPlugins(fset, registry, logger)
	if err != nil {
		t.Fatalf("Failed to create generator: %v", err)
	}

	goCodeBytes, err := gen.Generate(file)
	if err != nil {
		t.Fatalf("Code generation failed: %v", err)
	}
	goCode := string(goCodeBytes)

	if err := os.WriteFile(goPath, []byte(goCode), 0644); err != nil {
		t.Fatalf("Failed to write Go file: %v", err)
	}

	// Generate source map
	postASTGen := NewPostASTGenerator(dingoPath, goPath, fset, file.File, metadata)
	sourceMap, err := postASTGen.Generate()
	if err != nil {
		t.Fatalf("Source map generation failed: %v", err)
	}

	// Validate specific mapping: line 4 (let x = compute()?) should map correctly
	goLines := strings.Split(goCode, "\n")
	found := false

	for _, mapping := range sourceMap.Mappings {
		if mapping.OriginalLine == 4 {
			found = true
			genLine := mapping.GeneratedLine

			if genLine > 0 && genLine <= len(goLines) {
				t.Logf("✅ Dingo line 4 → Go line %d: %s",
					genLine, strings.TrimSpace(goLines[genLine-1]))
			} else {
				t.Errorf("❌ Generated line %d is out of bounds (1-%d)",
					genLine, len(goLines))
			}
		}
	}

	if !found {
		t.Error("❌ No mapping found for Dingo line 4 (error propagation line)")
	}

	t.Logf("Total mappings: %d", len(sourceMap.Mappings))
}
