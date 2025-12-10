package mapper

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/MadAppGang/dingo/pkg/sourcemap/dmap"
)

// Mapper provides efficient position mapping from Go files back to Dingo source.
// It caches .dmap readers for performance and handles missing .dmap files gracefully.
type Mapper struct {
	mu    sync.RWMutex
	cache map[string]*dmap.Reader // goPath -> reader
}

// New creates a new Mapper with an empty cache.
func New() *Mapper {
	return &Mapper{
		cache: make(map[string]*dmap.Reader),
	}
}

// MapToDingo maps a Go file position to its original Dingo source position.
//
// Parameters:
//   - goPath: Absolute path to the .go file
//   - goLine: 1-indexed line number in .go file
//   - goCol: 1-indexed column number in .go file
//
// Returns:
//   - dingoPath: Path to the .dingo source file
//   - line: 1-indexed line number in .dingo file
//   - col: 1-indexed column number in .dingo file
//   - err: Error if mapping failed
//
// Behavior:
//   - If .dmap file doesn't exist: returns original position (pure Go file)
//   - If position has no mapping: returns ErrNoMapping (generated code)
//   - If .dmap exists: maps to Dingo position using source map
func (m *Mapper) MapToDingo(goPath string, goLine, goCol int) (
	dingoPath string, line, col int, err error,
) {
	// Try to get reader from cache or load it
	reader, err := m.getReader(goPath)
	if err != nil {
		// No .dmap file - this is a pure Go file
		// Return original position unchanged
		return goPath, goLine, goCol, nil
	}

	// Convert line/col to byte offset
	// goLine is 1-indexed, goCol is 1-indexed
	goLineStart := reader.GoLineToByteOffset(goLine)
	if goLineStart == -1 {
		return "", 0, 0, fmt.Errorf("mapper: Go line %d out of range", goLine)
	}
	goByteOffset := goLineStart + (goCol - 1)

	// Find mapping for this byte offset
	dingoStart, _, kind := reader.FindByGoPos(goByteOffset)

	// If no mapping found (generated code), return error
	if kind == "" {
		return "", 0, 0, ErrNoMapping
	}

	// Convert Dingo byte offset to line/col
	dingoLine := reader.DingoByteToLine(dingoStart)
	if dingoLine == 0 {
		return "", 0, 0, fmt.Errorf("mapper: Dingo byte offset %d out of range", dingoStart)
	}

	dingoLineStart := reader.DingoLineToByteOffset(dingoLine)
	if dingoLineStart == -1 {
		return "", 0, 0, fmt.Errorf("mapper: Dingo line %d out of range", dingoLine)
	}
	dingoCol := dingoStart - dingoLineStart + 1

	// Construct .dingo path from .go path
	dingoPath = strings.TrimSuffix(goPath, ".go") + ".dingo"

	return dingoPath, dingoLine, dingoCol, nil
}

// getReader retrieves a cached .dmap reader or loads a new one.
// Returns error if .dmap file doesn't exist (not an error condition - means pure Go file).
func (m *Mapper) getReader(goPath string) (*dmap.Reader, error) {
	// Check cache first (read lock)
	m.mu.RLock()
	if reader, ok := m.cache[goPath]; ok {
		m.mu.RUnlock()
		return reader, nil
	}
	m.mu.RUnlock()

	// Not in cache - acquire write lock to load
	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check after acquiring write lock (another goroutine may have loaded it)
	if reader, ok := m.cache[goPath]; ok {
		return reader, nil
	}

	// Compute .dmap path: replace .go extension with .dmap
	dmapPath := strings.TrimSuffix(goPath, ".go") + ".dmap"

	// Check if .dmap file exists
	if _, err := os.Stat(dmapPath); os.IsNotExist(err) {
		// .dmap doesn't exist - this is a pure Go file
		return nil, err
	}

	// Load .dmap file
	reader, err := dmap.Open(dmapPath)
	if err != nil {
		return nil, fmt.Errorf("mapper: failed to open %s: %w", dmapPath, err)
	}

	// Cache the reader
	m.cache[goPath] = reader

	return reader, nil
}

// Close releases all cached readers.
// Should be called when the Mapper is no longer needed.
func (m *Mapper) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var errs []error
	for path, reader := range m.cache {
		if err := reader.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close %s: %w", path, err))
		}
	}

	m.cache = make(map[string]*dmap.Reader)

	if len(errs) > 0 {
		return fmt.Errorf("failed to close %d reader(s): %v", len(errs), errs)
	}
	return nil
}

// CacheStats returns statistics about the mapper cache.
// Useful for debugging and performance monitoring.
func (m *Mapper) CacheStats() CacheStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := CacheStats{
		CachedFiles: len(m.cache),
		Files:       make([]string, 0, len(m.cache)),
	}

	for path := range m.cache {
		// Store just the filename for readability
		stats.Files = append(stats.Files, filepath.Base(path))
	}

	return stats
}

// CacheStats holds statistics about the mapper's cache.
type CacheStats struct {
	CachedFiles int      // Number of .dmap readers in cache
	Files       []string // List of cached file names (basenames only)
}
