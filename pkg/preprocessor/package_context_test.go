package preprocessor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverDingoFiles(t *testing.T) {
	// Create temporary directory
	tmpDir := t.TempDir()

	// Create test files
	testFiles := []string{
		"main.dingo",
		"utils.dingo",
		"helper.go",      // Should be ignored
		"README.md",      // Should be ignored
		".hidden.dingo",  // Should be included
	}

	for _, file := range testFiles {
		path := filepath.Join(tmpDir, file)
		if err := os.WriteFile(path, []byte("package main"), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", file, err)
		}
	}

	// Create subdirectory (should not recurse)
	subDir := filepath.Join(tmpDir, "subpkg")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}
	subFile := filepath.Join(subDir, "sub.dingo")
	if err := os.WriteFile(subFile, []byte("package subpkg"), 0644); err != nil {
		t.Fatalf("Failed to create subdir file: %v", err)
	}

	// Test discovery
	files, err := discoverDingoFiles(tmpDir)
	if err != nil {
		t.Fatalf("discoverDingoFiles failed: %v", err)
	}

	// Verify results
	expected := []string{"main.dingo", "utils.dingo", ".hidden.dingo"}
	if len(files) != len(expected) {
		t.Errorf("Expected %d files, got %d", len(expected), len(files))
	}

	// Check that .dingo files are found
	foundMap := make(map[string]bool)
	for _, file := range files {
		basename := filepath.Base(file)
		foundMap[basename] = true
	}

	for _, exp := range expected {
		if !foundMap[exp] {
			t.Errorf("Expected to find %s, but didn't", exp)
		}
	}

	// Verify subdirectory file NOT included (no recursion)
	for _, file := range files {
		if filepath.Base(file) == "sub.dingo" {
			t.Errorf("Should not include files from subdirectories")
		}
	}

	// Verify .go files NOT included
	for _, file := range files {
		if filepath.Ext(file) != ".dingo" {
			t.Errorf("Should only include .dingo files, found: %s", file)
		}
	}
}

func TestNewPackageContext(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test .dingo file
	testFile := filepath.Join(tmpDir, "test.dingo")
	dingoCode := `package main

func LocalFunc() string {
	return "hello"
}

func AnotherFunc() int {
	return 42
}
`
	if err := os.WriteFile(testFile, []byte(dingoCode), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Test: Create package context
	opts := DefaultBuildOptions()
	ctx, err := NewPackageContext(tmpDir, opts)
	if err != nil {
		t.Fatalf("NewPackageContext failed: %v", err)
	}

	// Verify fields
	if ctx.packagePath == "" {
		t.Error("Package path should not be empty")
	}

	if len(ctx.dingoFiles) != 1 {
		t.Errorf("Expected 1 .dingo file, got %d", len(ctx.dingoFiles))
	}

	if ctx.cache == nil {
		t.Error("Cache should be initialized")
	}

	// Verify cache scanned the file
	metrics := ctx.cache.Metrics()
	if metrics.TotalSymbols == 0 {
		t.Error("Cache should have scanned symbols")
	}

	// Verify local functions detected
	if !ctx.cache.IsLocalSymbol("LocalFunc") {
		t.Error("Expected LocalFunc to be in cache")
	}
	if !ctx.cache.IsLocalSymbol("AnotherFunc") {
		t.Error("Expected AnotherFunc to be in cache")
	}
}

func TestPackageContext_CacheLoading(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test .dingo file
	testFile := filepath.Join(tmpDir, "test.dingo")
	dingoCode := `package main

func CachedFunc() string {
	return "cached"
}
`
	if err := os.WriteFile(testFile, []byte(dingoCode), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// First build: Create cache
	opts1 := BuildOptions{Incremental: true, Force: false, Verbose: false}
	ctx1, err := NewPackageContext(tmpDir, opts1)
	if err != nil {
		t.Fatalf("First NewPackageContext failed: %v", err)
	}

	// Verify cache file created
	cacheFile := filepath.Join(tmpDir, ".dingo-cache.json")
	if _, err := os.Stat(cacheFile); os.IsNotExist(err) {
		t.Error("Cache file should have been created")
	}

	// Second build: Load from cache
	opts2 := BuildOptions{Incremental: true, Force: false, Verbose: false}
	ctx2, err := NewPackageContext(tmpDir, opts2)
	if err != nil {
		t.Fatalf("Second NewPackageContext failed: %v", err)
	}

	// Verify cache loaded
	if !ctx2.cache.IsLocalSymbol("CachedFunc") {
		t.Error("Cache should have loaded CachedFunc from disk")
	}

	// Verify metrics show cold start only once
	metrics1 := ctx1.cache.Metrics()
	metrics2 := ctx2.cache.Metrics()

	if metrics2.ColdStarts > 0 {
		t.Error("Second build should not have done a cold start (loaded from cache)")
	}

	// Both should have same symbols
	if metrics1.TotalSymbols != metrics2.TotalSymbols {
		t.Errorf("Symbol counts should match: %d vs %d", metrics1.TotalSymbols, metrics2.TotalSymbols)
	}
}

func TestPackageContext_IncrementalBuild(t *testing.T) {
	tmpDir := t.TempDir()

	// Create initial test file
	testFile := filepath.Join(tmpDir, "test.dingo")
	initialCode := `package main

func OriginalFunc() string {
	return "original"
}
`
	if err := os.WriteFile(testFile, []byte(initialCode), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// First build
	opts := BuildOptions{Incremental: true, Force: false, Verbose: false}
	ctx1, err := NewPackageContext(tmpDir, opts)
	if err != nil {
		t.Fatalf("First build failed: %v", err)
	}

	if !ctx1.cache.IsLocalSymbol("OriginalFunc") {
		t.Error("OriginalFunc should be in cache")
	}

	// Modify file (add new function)
	modifiedCode := `package main

func OriginalFunc() string {
	return "original"
}

func NewFunc() int {
	return 42
}
`
	if err := os.WriteFile(testFile, []byte(modifiedCode), 0644); err != nil {
		t.Fatalf("Failed to modify test file: %v", err)
	}

	// Second build (should detect change and rescan)
	ctx2, err := NewPackageContext(tmpDir, opts)
	if err != nil {
		t.Fatalf("Second build failed: %v", err)
	}

	// Verify both functions detected
	if !ctx2.cache.IsLocalSymbol("OriginalFunc") {
		t.Error("OriginalFunc should still be in cache")
	}
	if !ctx2.cache.IsLocalSymbol("NewFunc") {
		t.Error("NewFunc should be in cache after rescan")
	}

	// Verify metrics show rescan happened
	metrics := ctx2.cache.Metrics()
	if metrics.TotalSymbols != 2 {
		t.Errorf("Expected 2 symbols after rescan, got %d", metrics.TotalSymbols)
	}
}

func TestPackageContext_ForceRebuild(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test file
	testFile := filepath.Join(tmpDir, "test.dingo")
	dingoCode := `package main

func TestFunc() string {
	return "test"
}
`
	if err := os.WriteFile(testFile, []byte(dingoCode), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// First build (creates cache)
	opts1 := BuildOptions{Incremental: true, Force: false, Verbose: false}
	ctx1, err := NewPackageContext(tmpDir, opts1)
	if err != nil {
		t.Fatalf("First build failed: %v", err)
	}

	metrics1 := ctx1.cache.Metrics()
	if metrics1.ColdStarts == 0 {
		t.Error("First build should have done a cold start")
	}

	// Second build with Force=true (should ignore cache)
	opts2 := BuildOptions{Incremental: true, Force: true, Verbose: false}
	ctx2, err := NewPackageContext(tmpDir, opts2)
	if err != nil {
		t.Fatalf("Second build failed: %v", err)
	}

	metrics2 := ctx2.cache.Metrics()
	if metrics2.ColdStarts == 0 {
		t.Error("Force rebuild should have done a cold start")
	}

	// Verify symbols still correct
	if !ctx2.cache.IsLocalSymbol("TestFunc") {
		t.Error("TestFunc should be in cache after force rebuild")
	}
}

func TestPackageContext_TranspileFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test .dingo file
	testFile := filepath.Join(tmpDir, "test.dingo")
	dingoCode := `package main

func main() {
	let x: int = 42
}
`
	if err := os.WriteFile(testFile, []byte(dingoCode), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create package context
	opts := DefaultBuildOptions()
	ctx, err := NewPackageContext(tmpDir, opts)
	if err != nil {
		t.Fatalf("NewPackageContext failed: %v", err)
	}

	// Transpile
	if err := ctx.TranspileFile(testFile); err != nil {
		t.Fatalf("TranspileFile failed: %v", err)
	}

	// Verify .go file created
	goFile := filepath.Join(tmpDir, "test.go")
	if _, err := os.Stat(goFile); os.IsNotExist(err) {
		t.Error(".go file should have been created")
	}

	// Verify source map created
	mapFile := filepath.Join(tmpDir, "test.go.map")
	if _, err := os.Stat(mapFile); os.IsNotExist(err) {
		t.Error("source map file should have been created")
	}

	// Read and verify .go output
	goContent, err := os.ReadFile(goFile)
	if err != nil {
		t.Fatalf("Failed to read .go file: %v", err)
	}

	// Verify transformation: DingoPreParser transforms let x: int = 42 → var x: int = 42
	// TypeAnnotProcessor then removes the colon: var x: int = 42 → var x int = 42
	goSource := string(goContent)
	if !containsString(goSource, "var x int = 42") {
		t.Errorf("Expected 'var x int = 42' in output, got:\n%s", goSource)
	}
}

func TestPackageContext_TranspileAll(t *testing.T) {
	tmpDir := t.TempDir()

	// Create multiple .dingo files
	file1 := filepath.Join(tmpDir, "main.dingo")
	file2 := filepath.Join(tmpDir, "utils.dingo")

	dingoCode1 := `package main

func main() {
	let x: int = 1
}
`
	dingoCode2 := `package main

func Helper() {
	let y: string = "hello"
}
`

	if err := os.WriteFile(file1, []byte(dingoCode1), 0644); err != nil {
		t.Fatalf("Failed to create file1: %v", err)
	}
	if err := os.WriteFile(file2, []byte(dingoCode2), 0644); err != nil {
		t.Fatalf("Failed to create file2: %v", err)
	}

	// Create package context
	opts := DefaultBuildOptions()
	ctx, err := NewPackageContext(tmpDir, opts)
	if err != nil {
		t.Fatalf("NewPackageContext failed: %v", err)
	}

	// Transpile all
	if err := ctx.TranspileAll(); err != nil {
		t.Fatalf("TranspileAll failed: %v", err)
	}

	// Verify both .go files created
	goFile1 := filepath.Join(tmpDir, "main.go")
	goFile2 := filepath.Join(tmpDir, "utils.go")

	if _, err := os.Stat(goFile1); os.IsNotExist(err) {
		t.Error("main.go should have been created")
	}
	if _, err := os.Stat(goFile2); os.IsNotExist(err) {
		t.Error("utils.go should have been created")
	}
}

func TestPackageContext_NoFilesError(t *testing.T) {
	tmpDir := t.TempDir()

	// Create directory with no .dingo files
	opts := DefaultBuildOptions()
	_, err := NewPackageContext(tmpDir, opts)

	if err == nil {
		t.Error("Expected error when no .dingo files found")
	}

	if err != nil && !containsString(err.Error(), "no .dingo files found") {
		t.Errorf("Expected 'no .dingo files found' error, got: %v", err)
	}
}

// Helper function to check if string contains substring
func containsString(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
