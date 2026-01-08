package tests

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestIntegrationPhase2EndToEnd tests the complete pipeline:
// .dingo → transpile → .go → compile → run
func TestIntegrationPhase2EndToEnd(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "dingo-integration-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Test case 1: Error propagation with error handling
	t.Run("error_propagation_result_type", func(t *testing.T) {
		dingoFile := filepath.Join(tmpDir, "test_result.dingo")
		dingoCode := `package main

import "fmt"

func processNumber(s string) (int, error) {
	num := parseInt(s)?
	return num * 2, nil
}

func parseInt(s string) (int, error) {
	return 42, nil
}

func main() {
	result, err := processNumber("21")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Println("Result:", result)
}
`
		if err := os.WriteFile(dingoFile, []byte(dingoCode), 0644); err != nil {
			t.Fatal(err)
		}

		// Build with dingo CLI
		// dingo build uses in-place output (.go next to .dingo)
		goFile := filepath.Join(tmpDir, "test_result.go")
		cmd := exec.Command("go", "run", filepath.Join("..", "cmd", "dingo"), "build", dingoFile)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Logf("Build output: %s", output)
			t.Fatalf("Failed to transpile: %v", err)
		}

		// Verify .go file was created in build/ folder
		if _, err := os.Stat(goFile); os.IsNotExist(err) {
			t.Fatal("Generated .go file not found")
		}

		// Compile the generated Go file
		cmd = exec.Command("go", "build", "-o", filepath.Join(tmpDir, "test_result"), goFile)
		output, err = cmd.CombinedOutput()
		if err != nil {
			t.Logf("Generated Go file:\n%s", mustReadFile(t, goFile))
			t.Logf("Compile output: %s", output)
			t.Fatalf("Failed to compile generated Go: %v", err)
		}

		t.Log("✓ Integration test passed: .dingo → .go → compile successful")
	})

	// Test case 2: Enum type generation
	t.Run("enum_type_generation", func(t *testing.T) {
		dingoFile := filepath.Join(tmpDir, "test_enum.dingo")
		dingoCode := `package main

enum Status {
	Pending,
	Active,
	Complete
}

func main() {
	s := NewStatusActive()
	// Use type switch to check variant (interface-based enum pattern)
	switch s.(type) {
	case StatusActive:
		println("Status is active")
	}
}
`
		if err := os.WriteFile(dingoFile, []byte(dingoCode), 0644); err != nil {
			t.Fatal(err)
		}

		// Build with dingo CLI
		// dingo build uses in-place output (.go next to .dingo)
		goFile := filepath.Join(tmpDir, "test_enum.go")
		cmd := exec.Command("go", "run", filepath.Join("..", "cmd", "dingo"), "build", dingoFile)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Logf("Build output: %s", output)
			t.Fatalf("Failed to transpile: %v", err)
		}

		// Verify .go file was created in build/ folder
		if _, err := os.Stat(goFile); os.IsNotExist(err) {
			t.Fatal("Generated .go file not found")
		}

		// Compile the generated Go file
		cmd = exec.Command("go", "build", "-o", filepath.Join(tmpDir, "test_enum"), goFile)
		output, err = cmd.CombinedOutput()
		if err != nil {
			t.Logf("Generated Go file:\n%s", mustReadFile(t, goFile))
			t.Logf("Compile output: %s", output)
			t.Fatalf("Failed to compile generated Go: %v", err)
		}

		// Verify enum code was generated (interface pattern)
		goCode := mustReadFile(t, goFile)
		if !strings.Contains(goCode, "type Status interface") {
			t.Error("Generated code missing Status interface")
		}
		if !strings.Contains(goCode, "StatusPending") {
			t.Error("Generated code missing StatusPending struct")
		}
		if !strings.Contains(goCode, "NewStatusActive()") {
			t.Error("Generated code missing NewStatusActive() constructor")
		}

		t.Log("✓ Integration test passed: Enum generation working correctly")
	})
}

func mustReadFile(t *testing.T, path string) string {
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read file %s: %v", path, err)
	}
	return string(content)
}
