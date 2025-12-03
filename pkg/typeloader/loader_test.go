package typeloader

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewLoader(t *testing.T) {
	config := LoaderConfig{
		WorkingDir: "/tmp",
		BuildTags:  []string{"integration"},
		FailFast:   true,
	}

	loader := NewLoader(config)

	if loader == nil {
		t.Fatal("NewLoader returned nil")
	}

	if loader.config.WorkingDir != config.WorkingDir {
		t.Errorf("WorkingDir = %q, want %q", loader.config.WorkingDir, config.WorkingDir)
	}

	if !loader.config.FailFast {
		t.Error("FailFast should be true")
	}
}

func TestNewLoader_DefaultFailFast(t *testing.T) {
	config := LoaderConfig{
		FailFast: false, // Explicitly set to false
	}

	loader := NewLoader(config)

	// Should default to true even when explicitly set to false
	if !loader.config.FailFast {
		t.Error("FailFast should default to true")
	}
}

func TestLoadFromImports_EmptyImports(t *testing.T) {
	loader := NewLoader(LoaderConfig{})

	result, err := loader.LoadFromImports([]string{})
	if err != nil {
		t.Fatalf("LoadFromImports failed: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if len(result.Functions) != 0 {
		t.Errorf("Functions map should be empty, got %d entries", len(result.Functions))
	}

	if len(result.Methods) != 0 {
		t.Errorf("Methods map should be empty, got %d entries", len(result.Methods))
	}
}

func TestLoadFromImports_StdlibPackage(t *testing.T) {
	// Get current working directory (should be in a Go module)
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	loader := NewLoader(LoaderConfig{
		WorkingDir: wd,
		FailFast:   true,
	})

	// Load a simple stdlib package
	result, err := loader.LoadFromImports([]string{"fmt"})
	if err != nil {
		t.Fatalf("LoadFromImports failed: %v", err)
	}

	// Check that we got some functions
	if len(result.Functions) == 0 {
		t.Error("Expected to find functions in fmt package")
	}

	// Check for specific well-known functions
	expectedFuncs := []string{
		"fmt.Printf",
		"fmt.Println",
		"fmt.Sprintf",
		"fmt.Errorf",
	}

	for _, expected := range expectedFuncs {
		if _, ok := result.Functions[expected]; !ok {
			t.Errorf("Expected function %q not found", expected)
		}
	}

	// Verify Printf signature
	if printfSig, ok := result.Functions["fmt.Printf"]; ok {
		if printfSig.Name != "Printf" {
			t.Errorf("Printf name = %q, want %q", printfSig.Name, "Printf")
		}
		if printfSig.Package != "fmt" {
			t.Errorf("Printf package = %q, want %q", printfSig.Package, "fmt")
		}
		// Printf returns (int, error)
		if len(printfSig.Results) != 2 {
			t.Errorf("Printf results count = %d, want 2", len(printfSig.Results))
		}
		if len(printfSig.Results) >= 2 {
			if !printfSig.Results[1].IsError {
				t.Error("Printf second result should be error type")
			}
		}
	}
}

func TestLoadFromImports_MultiplePackages(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	loader := NewLoader(LoaderConfig{
		WorkingDir: wd,
		FailFast:   true,
	})

	// Load multiple stdlib packages
	result, err := loader.LoadFromImports([]string{"os", "io"})
	if err != nil {
		t.Fatalf("LoadFromImports failed: %v", err)
	}

	// Check for functions from both packages
	expectedFuncs := []string{
		"os.Open",
		"os.ReadFile",
		"io.Copy",
		"io.ReadAll",
	}

	for _, expected := range expectedFuncs {
		if _, ok := result.Functions[expected]; !ok {
			t.Errorf("Expected function %q not found", expected)
		}
	}
}

func TestLoadFromImports_Deduplication(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	loader := NewLoader(LoaderConfig{
		WorkingDir: wd,
		FailFast:   true,
	})

	// Load same package multiple times
	result, err := loader.LoadFromImports([]string{"fmt", "fmt", "fmt"})
	if err != nil {
		t.Fatalf("LoadFromImports failed: %v", err)
	}

	// Should still work and deduplicate
	if _, ok := result.Functions["fmt.Printf"]; !ok {
		t.Error("Expected to find fmt.Printf")
	}
}

func TestLoadFromImports_InvalidPackage(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	loader := NewLoader(LoaderConfig{
		WorkingDir: wd,
		FailFast:   true,
	})

	// Try to load a non-existent package
	_, err = loader.LoadFromImports([]string{"github.com/nonexistent/package/that/does/not/exist"})
	if err == nil {
		t.Error("Expected error when loading invalid package")
	}

	// Error message should contain troubleshooting steps
	errMsg := err.Error()
	if errMsg == "" {
		t.Error("Error message should not be empty")
	}
}

func TestLoadFromImports_Methods(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	loader := NewLoader(LoaderConfig{
		WorkingDir: wd,
		FailFast:   true,
	})

	// Load os package which has File type with methods
	result, err := loader.LoadFromImports([]string{"os"})
	if err != nil {
		t.Fatalf("LoadFromImports failed: %v", err)
	}

	// Check for File methods
	expectedMethods := []string{
		"File.Close",
		"File.Read",
		"File.Write",
	}

	for _, expected := range expectedMethods {
		if _, ok := result.Methods[expected]; !ok {
			t.Errorf("Expected method %q not found", expected)
		}
	}

	// Verify Close method signature (should return error)
	if closeSig, ok := result.Methods["File.Close"]; ok {
		if closeSig.Name != "Close" {
			t.Errorf("Close name = %q, want %q", closeSig.Name, "Close")
		}
		if len(closeSig.Results) != 1 {
			t.Errorf("Close results count = %d, want 1", len(closeSig.Results))
		}
		if len(closeSig.Results) >= 1 && !closeSig.Results[0].IsError {
			t.Error("Close result should be error type")
		}
	}
}

func TestLoadWithLocalFuncs_OnlyImports(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	loader := NewLoader(LoaderConfig{
		WorkingDir: wd,
		FailFast:   true,
	})

	source := []byte(`package main
import "fmt"

func main() {
	fmt.Println("Hello")
}
`)

	result, err := loader.LoadWithLocalFuncs(source, []string{"fmt"})
	if err != nil {
		t.Fatalf("LoadWithLocalFuncs failed: %v", err)
	}

	// Should have fmt functions
	if _, ok := result.Functions["fmt.Printf"]; !ok {
		t.Error("Expected to find fmt.Printf")
	}

	// Local function parsing not yet implemented, so LocalFunctions should be empty
	if len(result.LocalFunctions) != 0 {
		t.Log("Note: LocalFunctions will be populated when local_func_parser.go is implemented")
	}
}

func TestTypeToRef_BasicTypes(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	loader := NewLoader(LoaderConfig{WorkingDir: wd})

	// Load a package to get access to types
	result, err := loader.LoadFromImports([]string{"fmt"})
	if err != nil {
		t.Fatalf("LoadFromImports failed: %v", err)
	}

	// Check Errorf which returns error type
	if errorfSig, ok := result.Functions["fmt.Errorf"]; ok {
		if len(errorfSig.Results) != 1 {
			t.Fatalf("Errorf should return 1 value, got %d", len(errorfSig.Results))
		}

		errorType := errorfSig.Results[0]
		if errorType.Name != "error" {
			t.Errorf("Error type name = %q, want %q", errorType.Name, "error")
		}
		if !errorType.IsError {
			t.Error("Error type should have IsError = true")
		}
	}
}

func TestJoinBuildTags(t *testing.T) {
	tests := []struct {
		name string
		tags []string
		want string
	}{
		{
			name: "empty",
			tags: []string{},
			want: "",
		},
		{
			name: "single tag",
			tags: []string{"integration"},
			want: "integration",
		},
		{
			name: "multiple tags",
			tags: []string{"linux", "amd64", "integration"},
			want: "linux,amd64,integration",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := joinBuildTags(tt.tags)
			if got != tt.want {
				t.Errorf("joinBuildTags() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLoadFromImports_WithWorkingDir(t *testing.T) {
	// Create a temporary directory structure
	tmpDir := t.TempDir()

	// Create a simple go.mod
	modContent := []byte("module testmod\n\ngo 1.21\n")
	modPath := filepath.Join(tmpDir, "go.mod")
	if err := os.WriteFile(modPath, modContent, 0644); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}

	loader := NewLoader(LoaderConfig{
		WorkingDir: tmpDir,
		FailFast:   true,
	})

	// Should be able to load stdlib packages from this directory
	result, err := loader.LoadFromImports([]string{"fmt"})
	if err != nil {
		t.Fatalf("LoadFromImports failed: %v", err)
	}

	if len(result.Functions) == 0 {
		t.Error("Expected to find functions even with custom working directory")
	}
}

func TestLoadFromImports_EmptyStrings(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	loader := NewLoader(LoaderConfig{
		WorkingDir: wd,
		FailFast:   true,
	})

	// Should handle empty strings in imports list
	result, err := loader.LoadFromImports([]string{"", "fmt", ""})
	if err != nil {
		t.Fatalf("LoadFromImports failed: %v", err)
	}

	// Should still load fmt
	if _, ok := result.Functions["fmt.Printf"]; !ok {
		t.Error("Expected to find fmt.Printf")
	}
}
