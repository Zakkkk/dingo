package lsp

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/MadAppGang/dingo/pkg/sourcemap"
	"github.com/MadAppGang/dingo/pkg/sourcemap/dmap"
)

// recordingLogger captures log messages for test assertions
type recordingLogger struct {
	mu       sync.Mutex
	messages []string
}

func (l *recordingLogger) Debugf(format string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.messages = append(l.messages, "DEBUG: "+format)
}

func (l *recordingLogger) Infof(format string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.messages = append(l.messages, "INFO: "+format)
}

func (l *recordingLogger) Warnf(format string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.messages = append(l.messages, "WARN: "+format)
}

func (l *recordingLogger) Errorf(format string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.messages = append(l.messages, "ERROR: "+format)
}

func (l *recordingLogger) Fatalf(format string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.messages = append(l.messages, "FATAL: "+format)
}

// Helper to create a test workspace with .dmap files
func setupTestWorkspace(t *testing.T) (string, func()) {
	t.Helper()

	// Create temp directory
	tempDir, err := os.MkdirTemp("", "dmap-cache-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Create go.mod to mark workspace root
	goMod := filepath.Join(tempDir, "go.mod")
	if err := os.WriteFile(goMod, []byte("module test\n"), 0644); err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to create go.mod: %v", err)
	}

	// Create .dmap directory
	dmapDir := filepath.Join(tempDir, ".dmap")
	if err := os.MkdirAll(dmapDir, 0755); err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to create .dmap dir: %v", err)
	}

	cleanup := func() {
		os.RemoveAll(tempDir)
	}

	return tempDir, cleanup
}

// Helper to create a .dmap file in the test workspace
func createTestDmapFile(t *testing.T, workspaceRoot, relPath string) {
	t.Helper()

	dingoSrc := []byte("x := 10\n")
	goSrc := []byte("x := 10\n")

	mappings := []sourcemap.LineMapping{
		{DingoLine: 1, GoLineStart: 1, GoLineEnd: 1, Kind: "identity"},
	}

	writer := dmap.NewWriter(dingoSrc, goSrc)
	data, err := writer.Write(mappings)
	if err != nil {
		t.Fatalf("Failed to create dmap data: %v", err)
	}

	dmapPath := filepath.Join(workspaceRoot, ".dmap", relPath)
	dmapDir := filepath.Dir(dmapPath)

	if err := os.MkdirAll(dmapDir, 0755); err != nil {
		t.Fatalf("Failed to create dmap directory: %v", err)
	}

	if err := os.WriteFile(dmapPath, data, 0644); err != nil {
		t.Fatalf("Failed to write dmap file: %v", err)
	}
}

func TestSourceMapCacheNew(t *testing.T) {
	logger := &testLogger{}
	cache, err := NewSourceMapCache(logger)
	if err != nil {
		t.Fatalf("NewSourceMapCache failed: %v", err)
	}

	if cache == nil {
		t.Fatal("Expected non-nil cache")
	}

	if cache.Size() != 0 {
		t.Errorf("New cache should be empty, got size %d", cache.Size())
	}
}

func TestSourceMapCacheGetMissing(t *testing.T) {
	logger := &testLogger{}
	cache, err := NewSourceMapCache(logger)
	if err != nil {
		t.Fatalf("NewSourceMapCache failed: %v", err)
	}

	// Try to get a non-existent source map
	_, err = cache.Get("/nonexistent/path/file.go")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}

func TestSourceMapCacheGetSuccess(t *testing.T) {
	workspaceRoot, cleanup := setupTestWorkspace(t)
	defer cleanup()

	// Create a .dmap file
	createTestDmapFile(t, workspaceRoot, "test.dmap")

	logger := &testLogger{}
	cache, err := NewSourceMapCache(logger)
	if err != nil {
		t.Fatalf("NewSourceMapCache failed: %v", err)
	}

	// Get the source map using the .go file path
	goPath := filepath.Join(workspaceRoot, "test.go")
	reader, err := cache.Get(goPath)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if reader == nil {
		t.Fatal("Expected non-nil reader")
	}

	// v2 format stores line mappings - verify via line lookup API
	// The test dmap has: DingoLine: 1, GoLineStart: 1, GoLineEnd: 1, Kind: "identity"
	dingoLine, kind := reader.GoLineToDingoLine(1)
	if dingoLine != 1 {
		t.Errorf("Expected dingo line 1, got %d", dingoLine)
	}
	if kind != "identity" {
		t.Errorf("Expected kind 'identity', got %q", kind)
	}

	// Cache should now have 1 entry
	if cache.Size() != 1 {
		t.Errorf("Cache size should be 1, got %d", cache.Size())
	}
}

func TestSourceMapCacheCacheHit(t *testing.T) {
	workspaceRoot, cleanup := setupTestWorkspace(t)
	defer cleanup()

	// Create a .dmap file
	createTestDmapFile(t, workspaceRoot, "cached.dmap")

	logger := &testLogger{}
	cache, err := NewSourceMapCache(logger)
	if err != nil {
		t.Fatalf("NewSourceMapCache failed: %v", err)
	}

	goPath := filepath.Join(workspaceRoot, "cached.go")

	// First get - cache miss, loads from disk
	reader1, err := cache.Get(goPath)
	if err != nil {
		t.Fatalf("First Get failed: %v", err)
	}

	// Second get - should hit cache
	reader2, err := cache.Get(goPath)
	if err != nil {
		t.Fatalf("Second Get failed: %v", err)
	}

	// Should return the same reader instance (pointer equality)
	if reader1 != reader2 {
		t.Error("Cache should return same reader instance")
	}
}

func TestSourceMapCacheInvalidate(t *testing.T) {
	workspaceRoot, cleanup := setupTestWorkspace(t)
	defer cleanup()

	// Create a .dmap file
	createTestDmapFile(t, workspaceRoot, "invalidate.dmap")

	logger := &testLogger{}
	cache, err := NewSourceMapCache(logger)
	if err != nil {
		t.Fatalf("NewSourceMapCache failed: %v", err)
	}

	goPath := filepath.Join(workspaceRoot, "invalidate.go")

	// Load into cache
	_, err = cache.Get(goPath)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if cache.Size() != 1 {
		t.Errorf("Cache size should be 1, got %d", cache.Size())
	}

	// Invalidate
	cache.Invalidate(goPath)

	if cache.Size() != 0 {
		t.Errorf("Cache size should be 0 after invalidate, got %d", cache.Size())
	}
}

func TestSourceMapCacheInvalidateAll(t *testing.T) {
	workspaceRoot, cleanup := setupTestWorkspace(t)
	defer cleanup()

	// Create multiple .dmap files
	createTestDmapFile(t, workspaceRoot, "file1.dmap")
	createTestDmapFile(t, workspaceRoot, "file2.dmap")

	logger := &testLogger{}
	cache, err := NewSourceMapCache(logger)
	if err != nil {
		t.Fatalf("NewSourceMapCache failed: %v", err)
	}

	// Load both into cache
	_, err = cache.Get(filepath.Join(workspaceRoot, "file1.go"))
	if err != nil {
		t.Fatalf("Get file1 failed: %v", err)
	}
	_, err = cache.Get(filepath.Join(workspaceRoot, "file2.go"))
	if err != nil {
		t.Fatalf("Get file2 failed: %v", err)
	}

	if cache.Size() != 2 {
		t.Errorf("Cache size should be 2, got %d", cache.Size())
	}

	// Invalidate all
	cache.InvalidateAll()

	if cache.Size() != 0 {
		t.Errorf("Cache size should be 0 after InvalidateAll, got %d", cache.Size())
	}
}

func TestSourceMapCacheConcurrentAccess(t *testing.T) {
	workspaceRoot, cleanup := setupTestWorkspace(t)
	defer cleanup()

	// Create a .dmap file
	createTestDmapFile(t, workspaceRoot, "concurrent.dmap")

	logger := &testLogger{}
	cache, err := NewSourceMapCache(logger)
	if err != nil {
		t.Fatalf("NewSourceMapCache failed: %v", err)
	}

	goPath := filepath.Join(workspaceRoot, "concurrent.go")

	// Run concurrent gets
	const numGoroutines = 10
	const numIterations = 50

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*numIterations)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numIterations; j++ {
				reader, err := cache.Get(goPath)
				if err != nil {
					errors <- err
					return
				}
				if reader == nil {
					errors <- err
					return
				}
				// Verify we can read from the reader
				_ = reader.EntryCount()
			}
		}()
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("Concurrent access error: %v", err)
	}
}

func TestSourceMapCachePathTranslation(t *testing.T) {
	// Test calculateDmapPath helper
	workspaceRoot, cleanup := setupTestWorkspace(t)
	defer cleanup()

	// Create nested structure
	nestedDir := filepath.Join(workspaceRoot, "examples", "subdir")
	if err := os.MkdirAll(nestedDir, 0755); err != nil {
		t.Fatalf("Failed to create nested dir: %v", err)
	}

	// Test path calculation
	dingoPath := filepath.Join(workspaceRoot, "examples", "subdir", "test.dingo")
	dmapPath, err := calculateDmapPath(dingoPath)
	if err != nil {
		t.Fatalf("calculateDmapPath failed: %v", err)
	}

	// Should be: workspaceRoot/.dmap/examples/subdir/test.dmap
	expected := filepath.Join(workspaceRoot, ".dmap", "examples", "subdir", "test.dmap")
	if dmapPath != expected {
		t.Errorf("calculateDmapPath: got %s, want %s", dmapPath, expected)
	}
}

func TestDetectWorkspaceRoot(t *testing.T) {
	// Test with go.mod
	tempDir, err := os.MkdirTemp("", "workspace-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create nested structure
	nestedDir := filepath.Join(tempDir, "a", "b", "c")
	if err := os.MkdirAll(nestedDir, 0755); err != nil {
		t.Fatalf("Failed to create nested dir: %v", err)
	}

	// Create go.mod at root
	goMod := filepath.Join(tempDir, "go.mod")
	if err := os.WriteFile(goMod, []byte("module test\n"), 0644); err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}

	// Detect from nested directory
	root, err := detectWorkspaceRoot(nestedDir)
	if err != nil {
		t.Fatalf("detectWorkspaceRoot failed: %v", err)
	}

	if root != tempDir {
		t.Errorf("detectWorkspaceRoot: got %s, want %s", root, tempDir)
	}
}

func TestDetectWorkspaceRootWithDingoToml(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "workspace-test-dingo")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	nestedDir := filepath.Join(tempDir, "x", "y")
	if err := os.MkdirAll(nestedDir, 0755); err != nil {
		t.Fatalf("Failed to create nested dir: %v", err)
	}

	// Create dingo.toml at root
	dingoToml := filepath.Join(tempDir, "dingo.toml")
	if err := os.WriteFile(dingoToml, []byte("[build]\n"), 0644); err != nil {
		t.Fatalf("Failed to create dingo.toml: %v", err)
	}

	root, err := detectWorkspaceRoot(nestedDir)
	if err != nil {
		t.Fatalf("detectWorkspaceRoot failed: %v", err)
	}

	if root != tempDir {
		t.Errorf("detectWorkspaceRoot: got %s, want %s", root, tempDir)
	}
}

func TestSourceMapCacheReaderLookups(t *testing.T) {
	workspaceRoot, cleanup := setupTestWorkspace(t)
	defer cleanup()

	// Create a .dmap file with known mappings
	createTestDmapFile(t, workspaceRoot, "lookup.dmap")

	logger := &testLogger{}
	cache, err := NewSourceMapCache(logger)
	if err != nil {
		t.Fatalf("NewSourceMapCache failed: %v", err)
	}

	goPath := filepath.Join(workspaceRoot, "lookup.go")
	reader, err := cache.Get(goPath)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	// Test v2 line-level mapping API: GoLineToDingoLine
	// The test dmap has: DingoLine: 1, GoLineStart: 1, GoLineEnd: 1, Kind: "identity"
	dingoLine, kind := reader.GoLineToDingoLine(1)
	if kind != "identity" {
		t.Errorf("GoLineToDingoLine: got kind %q, want %q", kind, "identity")
	}
	if dingoLine != 1 {
		t.Errorf("GoLineToDingoLine: got line %d, want 1", dingoLine)
	}
}

// Test that the logger gets called appropriately
func TestSourceMapCacheLogging(t *testing.T) {
	workspaceRoot, cleanup := setupTestWorkspace(t)
	defer cleanup()

	createTestDmapFile(t, workspaceRoot, "logging.dmap")

	logger := &recordingLogger{}
	cache, err := NewSourceMapCache(logger)
	if err != nil {
		t.Fatalf("NewSourceMapCache failed: %v", err)
	}

	goPath := filepath.Join(workspaceRoot, "logging.go")

	// First get - should log load
	_, err = cache.Get(goPath)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	// Check that info message was logged
	logger.mu.Lock()
	hasLoadMessage := false
	for _, msg := range logger.messages {
		if len(msg) > 5 && msg[:5] == "INFO:" {
			hasLoadMessage = true
			break
		}
	}
	logger.mu.Unlock()

	if !hasLoadMessage {
		t.Error("Expected INFO log message for source map load")
	}
}
