package transpiler_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/MadAppGang/dingo/pkg/transpiler"
)

func TestTranspileFile(t *testing.T) {
	t.Skip("AST migration: Source maps not yet implemented in pure AST pipeline")

	// TODO(ast-migration): Re-enable when AST pipeline supports:
	// 1. Source map generation (.go.map files)
	// 2. Error propagation (? operator) transformation
	// 3. Let keyword transformation

	// Create temp directory
	tmpDir := t.TempDir()

	// Create test .dingo file
	dingoPath := filepath.Join(tmpDir, "test.dingo")
	dingoSrc := `package main

func readConfig(path string) ([]byte, error) {
	let data = os.ReadFile(path)?
	return data, nil
}
`
	if err := os.WriteFile(dingoPath, []byte(dingoSrc), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Create transpiler
	tr, err := transpiler.New()
	if err != nil {
		t.Fatalf("Failed to create transpiler: %v", err)
	}

	// Transpile
	err = tr.TranspileFile(dingoPath)
	if err != nil {
		t.Fatalf("Transpile failed: %v", err)
	}

	// Verify .go file exists
	goPath := filepath.Join(tmpDir, "test.go")
	if _, err := os.Stat(goPath); os.IsNotExist(err) {
		t.Errorf(".go file not created: %s", goPath)
	}

	// Verify .go.map file exists
	mapPath := goPath + ".map"
	if _, err := os.Stat(mapPath); os.IsNotExist(err) {
		t.Errorf(".go.map file not created: %s", mapPath)
	}

	// Read and verify .go file contains expected transformations
	goContent, err := os.ReadFile(goPath)
	if err != nil {
		t.Fatalf("Failed to read .go file: %v", err)
	}

	goStr := string(goContent)
	// Should contain error propagation expansion
	if !contains(goStr, "if err != nil") {
		t.Errorf(".go file should contain error propagation, got:\n%s", goStr)
	}
}

func TestTranspileFileWithCustomOutput(t *testing.T) {
	t.Skip("AST migration: Source maps not yet implemented in pure AST pipeline")

	// TODO(ast-migration): Re-enable when AST pipeline supports:
	// 1. Source map generation (.go.map files)
	// 2. Let keyword transformation

	tmpDir := t.TempDir()

	dingoPath := filepath.Join(tmpDir, "input.dingo")
	dingoSrc := `package main

func test() {
	let x = 42
}
`
	if err := os.WriteFile(dingoPath, []byte(dingoSrc), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	tr, err := transpiler.New()
	if err != nil {
		t.Fatalf("Failed to create transpiler: %v", err)
	}

	// Transpile with custom output
	customOutput := filepath.Join(tmpDir, "custom_output.go")
	err = tr.TranspileFileWithOutput(dingoPath, customOutput)
	if err != nil {
		t.Fatalf("Transpile failed: %v", err)
	}

	// Verify custom .go file exists
	if _, err := os.Stat(customOutput); os.IsNotExist(err) {
		t.Errorf("Custom .go file not created: %s", customOutput)
	}

	// Verify source map uses custom path
	mapPath := customOutput + ".map"
	if _, err := os.Stat(mapPath); os.IsNotExist(err) {
		t.Errorf("Custom .go.map file not created: %s", mapPath)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || contains(s[1:], substr)))
}
