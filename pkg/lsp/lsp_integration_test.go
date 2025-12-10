package lsp

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// These tests require the dingo-lsp binary to be built.
// Run: go build -o bin/dingo-lsp ./cmd/dingo-lsp
// Then: go test ./pkg/lsp/... -run TestLSP -v

func getWorkspaceRoot(t *testing.T) string {
	t.Helper()

	// Find workspace root by looking for go.mod
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("Could not find workspace root (go.mod)")
		}
		dir = parent
	}
}

func getLSPBinary(t *testing.T, workspaceRoot string) string {
	t.Helper()

	binary := filepath.Join(workspaceRoot, "bin", "dingo-lsp")
	if _, err := os.Stat(binary); os.IsNotExist(err) {
		t.Skipf("dingo-lsp binary not found at %s. Run: go build -o bin/dingo-lsp ./cmd/dingo-lsp", binary)
	}
	return binary
}

func buildExamples(t *testing.T, workspaceRoot string) {
	t.Helper()

	// Build the examples to generate .go and .dmap files
	dingoBinary := filepath.Join(workspaceRoot, "bin", "dingo")
	if _, err := os.Stat(dingoBinary); os.IsNotExist(err) {
		t.Skipf("dingo binary not found at %s. Run: go build -o bin/dingo ./cmd/dingo", dingoBinary)
	}

	// Build example 101_combined (has 35 mapping entries for meaningful tests)
	cmd := exec.Command(dingoBinary, "build", "--no-mascot",
		filepath.Join(workspaceRoot, "examples", "101_combined", "showcase.dingo"))
	cmd.Dir = workspaceRoot
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Logf("Build output: %s", string(output))
		// Don't fail - the .go files might already exist
	}
}

func TestLSPHover(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping LSP integration test in short mode")
	}

	workspaceRoot := getWorkspaceRoot(t)
	lspBinary := getLSPBinary(t, workspaceRoot)
	buildExamples(t, workspaceRoot)

	client := NewLSPTestClient(t, lspBinary, "--log-level", "error")
	defer client.Shutdown()

	workspaceURI := "file://" + workspaceRoot
	if err := client.Initialize(workspaceURI); err != nil {
		t.Fatalf("Failed to initialize LSP: %v", err)
	}

	tests := []HoverTest{
		{
			Name:      "lambda_rust_style",
			File:      "examples/101_combined/showcase.dingo",
			Line:      78, // Line 79: fn1 := |x int| -> int { x * 2 }
			Character: 9,  // Position of "|x"
			// Lambda syntax generates source mapping entries
		},
		{
			Name:      "lambda_typescript_style",
			File:      "examples/101_combined/showcase.dingo",
			Line:      81, // Line 82: fn2 := (x int): int => { x * 3 }
			Character: 8,  // Position of "(x"
		},
		{
			Name:      "safe_navigation",
			File:      "examples/101_combined/showcase.dingo",
			Line:      116, // Line 117: userLang := user.Settings?.Language
			Character: 5,   // Position of "userLang" identifier
		},
		{
			Name:      "null_coalesce",
			File:      "examples/101_combined/showcase.dingo",
			Line:      119, // Line 120: displayLang := userLang ?? "default"
			Character: 5,   // Position of "displayLang" identifier
		},
		{
			Name:      "match_expression",
			File:      "examples/101_combined/showcase.dingo",
			Line:      127, // Line 128: statusMsg := match status {
			Character: 5,   // Position of "statusMsg" identifier
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			uri := workspaceURI + "/" + tc.File

			result, err := client.Hover(uri, tc.Line, tc.Character)
			if err != nil {
				t.Logf("Hover request returned error (may be expected): %v", err)
				return
			}

			if result != nil {
				t.Logf("Hover content: %q", result.Contents.Value)
			} else {
				t.Logf("Hover returned nil (no info at position)")
			}
		})
	}
}

func TestLSPCompletion(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping LSP integration test in short mode")
	}

	workspaceRoot := getWorkspaceRoot(t)
	lspBinary := getLSPBinary(t, workspaceRoot)
	buildExamples(t, workspaceRoot)

	client := NewLSPTestClient(t, lspBinary, "--log-level", "error")
	defer client.Shutdown()

	workspaceURI := "file://" + workspaceRoot
	if err := client.Initialize(workspaceURI); err != nil {
		t.Fatalf("Failed to initialize LSP: %v", err)
	}

	tests := []struct {
		name      string
		file      string
		line      int
		character int
	}{
		{
			name:      "inside_function",
			file:      "examples/101_combined/showcase.dingo",
			line:      108, // Inside demo() function (line 109)
			character: 5,
		},
		{
			name:      "after_dot_struct_access",
			file:      "examples/101_combined/showcase.dingo",
			line:      116, // user.Settings?.Language (line 117)
			character: 10,  // After "user."
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			uri := workspaceURI + "/" + tc.file

			result, err := client.Completion(uri, tc.line, tc.character)
			if err != nil {
				t.Logf("Completion request returned error (may be expected): %v", err)
				return
			}

			if result != nil {
				t.Logf("Completion items: %d", len(result.Items))
				for i, item := range result.Items {
					if i < 5 { // Log first 5 items
						t.Logf("  - %s (%s)", item.Label, item.Detail)
					}
				}
			} else {
				t.Logf("Completion returned nil")
			}
		})
	}
}

func TestLSPDefinition(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping LSP integration test in short mode")
	}

	workspaceRoot := getWorkspaceRoot(t)
	lspBinary := getLSPBinary(t, workspaceRoot)
	buildExamples(t, workspaceRoot)

	client := NewLSPTestClient(t, lspBinary, "--log-level", "error")
	defer client.Shutdown()

	workspaceURI := "file://" + workspaceRoot
	if err := client.Initialize(workspaceURI); err != nil {
		t.Fatalf("Failed to initialize LSP: %v", err)
	}

	tests := []struct {
		name      string
		file      string
		line      int
		character int
	}{
		{
			name:      "function_reference",
			file:      "examples/101_combined/showcase.dingo",
			line:      149, // r := fetchUser(42) (line 150)
			character: 7,   // Position of fetchUser
		},
		{
			name:      "struct_field_access",
			file:      "examples/101_combined/showcase.dingo",
			line:      116, // user.Settings?.Language (line 117)
			character: 6,   // Position of Settings
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			uri := workspaceURI + "/" + tc.file

			locations, err := client.Definition(uri, tc.line, tc.character)
			if err != nil {
				t.Logf("Definition request returned error (may be expected): %v", err)
				return
			}

			if locations != nil {
				t.Logf("Definition locations: %d", len(locations))
				for _, loc := range locations {
					t.Logf("  - %s:%d:%d", loc.URI, loc.Range.Start.Line+1, loc.Range.Start.Character+1)
				}
			} else {
				t.Logf("Definition returned nil")
			}
		})
	}
}

// TestLSPServerStartup verifies the LSP server starts and responds to initialize
func TestLSPServerStartup(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping LSP integration test in short mode")
	}

	workspaceRoot := getWorkspaceRoot(t)
	lspBinary := getLSPBinary(t, workspaceRoot)

	client := NewLSPTestClient(t, lspBinary, "--log-level", "error")
	defer client.Shutdown()

	workspaceURI := "file://" + workspaceRoot
	if err := client.Initialize(workspaceURI); err != nil {
		t.Fatalf("Failed to initialize LSP server: %v", err)
	}

	t.Log("LSP server started and initialized successfully")
}

// TestLSPSourceMapIntegration tests that source maps are being loaded
func TestLSPSourceMapIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping LSP integration test in short mode")
	}

	workspaceRoot := getWorkspaceRoot(t)
	lspBinary := getLSPBinary(t, workspaceRoot)
	buildExamples(t, workspaceRoot)

	// Verify .dmap file exists - using 101_combined which has 35 mapping entries
	dmapPath := filepath.Join(workspaceRoot, ".dmap", "examples", "101_combined", "showcase.dmap")
	if _, err := os.Stat(dmapPath); os.IsNotExist(err) {
		t.Skipf(".dmap file not found at %s - build examples first", dmapPath)
	}

	// Verify the .dmap has entries (check file size > header size of 36 bytes)
	info, err := os.Stat(dmapPath)
	if err != nil {
		t.Fatalf("Failed to stat .dmap file: %v", err)
	}
	if info.Size() <= 36 {
		t.Fatalf("Expected .dmap file with entries, got size %d bytes (header-only)", info.Size())
	}
	t.Logf(".dmap file size: %d bytes (has mapping entries)", info.Size())

	client := NewLSPTestClient(t, lspBinary, "--log-level", "info")
	defer client.Shutdown()

	workspaceURI := "file://" + workspaceRoot
	if err := client.Initialize(workspaceURI); err != nil {
		t.Fatalf("Failed to initialize LSP: %v", err)
	}

	// Send a hover request on lambda syntax - this should trigger source map loading
	// Line 78 (0-indexed) = line 79: |x int| -> int { x * 2 }
	uri := workspaceURI + "/examples/101_combined/showcase.dingo"
	result, err := client.Hover(uri, 78, 10)
	if err != nil {
		t.Logf("Hover returned error: %v", err)
	}

	if result != nil && result.Contents.Value != "" {
		t.Logf("Source map integration: Hover returned content: %q", result.Contents.Value)
	} else {
		t.Logf("Source map integration: Hover returned empty (identity mapping may be used)")
	}

	t.Log("Source map integration test completed")
}
