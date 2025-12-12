package mapper

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MadAppGang/dingo/pkg/sourcemap"
	"github.com/MadAppGang/dingo/pkg/sourcemap/dmap"
)

// TestMapToDingo_WithValidDmap tests mapping with a real .dmap file
func TestMapToDingo_WithValidDmap(t *testing.T) {
	// Create temporary directory
	tmpDir := t.TempDir()

	// Create a simple .dmap file for testing
	dmapPath := filepath.Join(tmpDir, "test.dmap")
	goPath := filepath.Join(tmpDir, "test.go")

	// Create mock source files
	dingoSrc := []byte("0123456789\n0123456789\n0123456789\n0123456789\n")
	goSrc := []byte("0123456789ABCDE\n0123456789ABCDE\n0123456789ABCDE\n0123456789ABCDE\n0123456789ABCDE\n0123456789ABCDE\n")

	// Create a test .dmap file with known mappings
	writer := dmap.NewWriter(dingoSrc, goSrc)

	// Simulate some simple line mappings:
	// Dingo line 1 -> Go line 1 (kind: "match")
	// Dingo line 3 -> Go line 4 (kind: "let")
	// Line structure:
	//   Dingo: each line is 11 bytes (10 chars + \n)
	//     Line 1: 0-10, Line 2: 11-21, Line 3: 22-32, Line 4: 33-43
	//   Go: each line is 16 bytes (15 chars + \n)
	//     Line 1: 0-15, Line 2: 16-31, Line 3: 32-47, Line 4: 48-63
	mappings := []sourcemap.LineMapping{
		{DingoLine: 1, GoLineStart: 1, GoLineEnd: 1, Kind: "match"},
		{DingoLine: 3, GoLineStart: 4, GoLineEnd: 4, Kind: "let"},
	}

	data, err := writer.Write(mappings)
	if err != nil {
		t.Fatalf("Failed to create test .dmap: %v", err)
	}

	if err := os.WriteFile(dmapPath, data, 0644); err != nil {
		t.Fatalf("Failed to write .dmap file: %v", err)
	}

	// Create mapper and test
	mapper := New()
	defer mapper.Close()

	// Test case 1: Map a position in the first mapping (match)
	// Go line 1, col 1 -> byte offset 0
	dingoPath, line, col, err := mapper.MapToDingo(goPath, 1, 1)
	if err != nil {
		t.Errorf("MapToDingo failed: %v", err)
	}

	expectedPath := strings.TrimSuffix(goPath, ".go") + ".dingo"
	if dingoPath != expectedPath {
		t.Errorf("Expected path %s, got %s", expectedPath, dingoPath)
	}
	if line != 1 || col != 1 {
		t.Errorf("Expected line 1, col 1; got line %d, col %d", line, col)
	}

	// Test case 2: Map a position in the second mapping (let)
	// Go line 4, col 1 -> byte offset 50 (line 4 starts at byte 48, plus col 1 = byte 49)
	// This should map to Dingo byte 20 (line 3, col 1)
	// But we need to verify the actual line offsets first
	dingoPath, line, col, err = mapper.MapToDingo(goPath, 4, 1)
	if err != nil {
		t.Logf("Go source: %q", goSrc)
		t.Logf("Dingo source: %q", dingoSrc)
		t.Logf("Trying to map Go line 4, col 1")
		t.Errorf("MapToDingo failed: %v", err)
	}
	// Line 3 in Dingo (bytes 20-30), col should be 1 (byte 20)
	if line != 3 || col != 1 {
		t.Errorf("Expected line 3, col 1; got line %d, col %d", line, col)
	}
}

// TestMapToDingo_NoDmapFile tests behavior when .dmap file doesn't exist (pure Go)
func TestMapToDingo_NoDmapFile(t *testing.T) {
	tmpDir := t.TempDir()
	goPath := filepath.Join(tmpDir, "pure.go")

	mapper := New()
	defer mapper.Close()

	// Should return original position when .dmap doesn't exist
	dingoPath, line, col, err := mapper.MapToDingo(goPath, 10, 5)
	if err != nil {
		t.Errorf("Expected no error for pure Go file, got: %v", err)
	}

	if dingoPath != goPath {
		t.Errorf("Expected path %s (unchanged), got %s", goPath, dingoPath)
	}
	if line != 10 || col != 5 {
		t.Errorf("Expected line 10, col 5 (unchanged); got line %d, col %d", line, col)
	}
}

// TestMapToDingo_UnmappedLine tests behavior for unmapped lines (uses fallback)
// v2 format uses cumulative delta fallback for unmapped lines rather than returning errors.
// This provides better UX for LSP - we always return an approximate position.
func TestMapToDingo_UnmappedLine(t *testing.T) {
	tmpDir := t.TempDir()
	dmapPath := filepath.Join(tmpDir, "test.dmap")
	goPath := filepath.Join(tmpDir, "test.go")

	// Create mock source files:
	// Dingo: 3 lines
	// Go: 5 lines (more lines = expansion happened)
	dingoSrc := []byte("0123456789\n0123456789\n0123456789\n")
	goSrc := []byte("0123456789\n0123456789\n0123456789\n0123456789\n0123456789\n")

	// Create a .dmap with only line 1 mapped explicitly
	writer := dmap.NewWriter(dingoSrc, goSrc)

	// Only map line 1, leave lines 2+ unmapped
	mappings := []sourcemap.LineMapping{
		{DingoLine: 1, GoLineStart: 1, GoLineEnd: 1, Kind: "match"},
	}

	data, err := writer.Write(mappings)
	if err != nil {
		t.Fatalf("Failed to create test .dmap: %v", err)
	}

	if err := os.WriteFile(dmapPath, data, 0644); err != nil {
		t.Fatalf("Failed to write .dmap file: %v", err)
	}

	mapper := New()
	defer mapper.Close()

	// Try to map a position in unmapped Go line 3
	// v2 format uses fallback (cumulative delta) - should NOT error
	dingoPath, line, col, err := mapper.MapToDingo(goPath, 3, 1)
	if err != nil {
		t.Errorf("Expected no error for unmapped line (v2 uses fallback), got: %v", err)
	}

	expectedPath := strings.TrimSuffix(goPath, ".go") + ".dingo"
	if dingoPath != expectedPath {
		t.Errorf("Expected path %s, got %s", expectedPath, dingoPath)
	}

	// Line should be a valid fallback (proportional or delta-based)
	// We just verify it's reasonable (within source bounds)
	if line < 1 || line > 3 {
		t.Errorf("Expected line in range 1-3 (fallback), got %d", line)
	}
	if col != 1 {
		t.Errorf("Expected col 1 (preserved), got %d", col)
	}
}

// TestMapToDingo_Caching tests that the mapper caches .dmap readers
func TestMapToDingo_Caching(t *testing.T) {
	tmpDir := t.TempDir()
	dmapPath := filepath.Join(tmpDir, "test.dmap")
	goPath := filepath.Join(tmpDir, "test.go")

	// Create mock source files
	dingoSrc := []byte("0123456789\n0123456789\n")
	goSrc := []byte("0123456789\n0123456789\n")

	// Create a minimal .dmap
	writer := dmap.NewWriter(dingoSrc, goSrc)
	mappings := []sourcemap.LineMapping{
		{DingoLine: 1, GoLineStart: 1, GoLineEnd: 1, Kind: "test"},
	}

	data, err := writer.Write(mappings)
	if err != nil {
		t.Fatalf("Failed to create test .dmap: %v", err)
	}

	if err := os.WriteFile(dmapPath, data, 0644); err != nil {
		t.Fatalf("Failed to write .dmap file: %v", err)
	}

	mapper := New()
	defer mapper.Close()

	// First call - should load .dmap
	_, _, _, err = mapper.MapToDingo(goPath, 1, 1)
	if err != nil {
		t.Fatalf("First MapToDingo failed: %v", err)
	}

	// Check cache stats
	stats := mapper.CacheStats()
	if stats.CachedFiles != 1 {
		t.Errorf("Expected 1 cached file, got %d", stats.CachedFiles)
	}
	if len(stats.Files) != 1 || stats.Files[0] != "test.go" {
		t.Errorf("Expected cached file 'test.go', got %v", stats.Files)
	}

	// Second call - should use cache (we can't directly verify this,
	// but at least confirm it still works)
	_, _, _, err = mapper.MapToDingo(goPath, 1, 1)
	if err != nil {
		t.Fatalf("Second MapToDingo failed: %v", err)
	}

	// Stats should be unchanged
	stats = mapper.CacheStats()
	if stats.CachedFiles != 1 {
		t.Errorf("Expected 1 cached file after second call, got %d", stats.CachedFiles)
	}
}

// TestMapToDingo_MultipleDmapFiles tests caching multiple .dmap files
func TestMapToDingo_MultipleDmapFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create two .dmap files
	createTestDmap := func(name string) string {
		dmapPath := filepath.Join(tmpDir, name+".dmap")
		goPath := filepath.Join(tmpDir, name+".go")

		dingoSrc := []byte("0123456789\n0123456789\n")
		goSrc := []byte("0123456789\n0123456789\n")

		writer := dmap.NewWriter(dingoSrc, goSrc)
		mappings := []sourcemap.LineMapping{
			{DingoLine: 1, GoLineStart: 1, GoLineEnd: 1, Kind: "test"},
		}

		data, err := writer.Write(mappings)
		if err != nil {
			t.Fatalf("Failed to create .dmap: %v", err)
		}

		if err := os.WriteFile(dmapPath, data, 0644); err != nil {
			t.Fatalf("Failed to write .dmap: %v", err)
		}

		return goPath
	}

	goPath1 := createTestDmap("file1")
	goPath2 := createTestDmap("file2")

	mapper := New()
	defer mapper.Close()

	// Map from both files
	_, _, _, err := mapper.MapToDingo(goPath1, 1, 1)
	if err != nil {
		t.Fatalf("MapToDingo for file1 failed: %v", err)
	}

	_, _, _, err = mapper.MapToDingo(goPath2, 1, 1)
	if err != nil {
		t.Fatalf("MapToDingo for file2 failed: %v", err)
	}

	// Check that both are cached
	stats := mapper.CacheStats()
	if stats.CachedFiles != 2 {
		t.Errorf("Expected 2 cached files, got %d", stats.CachedFiles)
	}
}

// TestClose tests that Close properly releases resources
func TestClose(t *testing.T) {
	tmpDir := t.TempDir()
	dmapPath := filepath.Join(tmpDir, "test.dmap")
	goPath := filepath.Join(tmpDir, "test.go")

	// Create mock source files
	dingoSrc := []byte("0123456789\n0123456789\n")
	goSrc := []byte("0123456789\n0123456789\n")

	// Create a minimal .dmap
	writer := dmap.NewWriter(dingoSrc, goSrc)
	mappings := []sourcemap.LineMapping{
		{DingoLine: 1, GoLineStart: 1, GoLineEnd: 1, Kind: "test"},
	}

	data, err := writer.Write(mappings)
	if err != nil {
		t.Fatalf("Failed to create test .dmap: %v", err)
	}

	if err := os.WriteFile(dmapPath, data, 0644); err != nil {
		t.Fatalf("Failed to write .dmap file: %v", err)
	}

	mapper := New()

	// Load a .dmap
	_, _, _, err = mapper.MapToDingo(goPath, 1, 1)
	if err != nil {
		t.Fatalf("MapToDingo failed: %v", err)
	}

	// Verify it's cached
	if mapper.CacheStats().CachedFiles != 1 {
		t.Error("Expected 1 cached file before Close")
	}

	// Close
	if err := mapper.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// Verify cache is empty
	if mapper.CacheStats().CachedFiles != 0 {
		t.Error("Expected 0 cached files after Close")
	}
}

// BenchmarkMapToDingo benchmarks the position mapping performance
func BenchmarkMapToDingo(b *testing.B) {
	tmpDir := b.TempDir()
	dmapPath := filepath.Join(tmpDir, "test.dmap")
	goPath := filepath.Join(tmpDir, "test.go")

	// Create large mock source files
	dingoSrc := make([]byte, 100000)
	goSrc := make([]byte, 100000)
	for i := range dingoSrc {
		if i%100 == 99 {
			dingoSrc[i] = '\n'
			goSrc[i] = '\n'
		} else {
			dingoSrc[i] = 'x'
			goSrc[i] = 'x'
		}
	}

	// Create a .dmap with many mappings
	writer := dmap.NewWriter(dingoSrc, goSrc)
	mappings := make([]sourcemap.LineMapping, 1000)
	for i := 0; i < 1000; i++ {
		line := i + 1 // 1-indexed lines
		mappings[i] = sourcemap.LineMapping{
			DingoLine:   line,
			GoLineStart: line,
			GoLineEnd:   line,
			Kind:        "test",
		}
	}

	data, err := writer.Write(mappings)
	if err != nil {
		b.Fatalf("Failed to create test .dmap: %v", err)
	}

	if err := os.WriteFile(dmapPath, data, 0644); err != nil {
		b.Fatalf("Failed to write .dmap file: %v", err)
	}

	mapper := New()
	defer mapper.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Map various positions
		line := (i % 1000) + 1
		_, _, _, _ = mapper.MapToDingo(goPath, line, 1)
	}
}
