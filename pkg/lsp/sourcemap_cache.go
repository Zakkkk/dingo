package lsp

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/MadAppGang/dingo/pkg/sourcemap/dmap"
)

// SourceMapGetter is an interface for retrieving source maps
type SourceMapGetter interface {
	Get(goFilePath string) (*dmap.Reader, error)
	Invalidate(goFilePath string)
	InvalidateAll()
	Size() int
}

// SourceMapCache provides in-memory caching of binary .dmap source maps
type SourceMapCache struct {
	mu      sync.RWMutex
	maps    map[string]*dmap.Reader // dingoPath -> dmap.Reader
	logger  Logger
	maxSize int
}

// NewSourceMapCache creates a new source map cache
func NewSourceMapCache(logger Logger) (*SourceMapCache, error) {
	return &SourceMapCache{
		maps:    make(map[string]*dmap.Reader),
		logger:  logger,
		maxSize: 100, // LRU limit (future: implement eviction)
	}, nil
}

// Get retrieves a source map from cache or loads it from disk.
// Path translation: goFilePath (.go) → dingoPath (.dingo) → .dmap/<relPath>.dmap
func (c *SourceMapCache) Get(goFilePath string) (*dmap.Reader, error) {
	// Translate: foo.go -> foo.dingo
	dingoPath := strings.TrimSuffix(goFilePath, ".go") + ".dingo"

	c.logger.Debugf("[SourceMapCache] Get called: goFilePath=%s -> dingoPath=%s", goFilePath, dingoPath)

	// Calculate .dmap path in project root .dmap/ folder
	dmapPath, err := calculateDmapPath(dingoPath)
	if err != nil {
		c.logger.Debugf("[SourceMapCache] Failed to calculate dmap path: %v", err)
		return nil, fmt.Errorf("failed to calculate dmap path: %w", err)
	}
	c.logger.Debugf("[SourceMapCache] Calculated dmapPath=%s", dmapPath)

	// CRITICAL FIX C3: Simplified locking (correctness over optimization)
	// Hold write lock during entire operation to avoid memory model issues
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check cache under write lock
	if reader, ok := c.maps[dingoPath]; ok {
		c.logger.Debugf("Source map cache hit: %s", dmapPath)
		return reader, nil
	}

	// Load binary .dmap file (still holding lock)
	c.logger.Debugf("[SourceMapCache] Opening dmap file: %s", dmapPath)
	reader, err := dmap.Open(dmapPath)
	if err != nil {
		if os.IsNotExist(err) {
			c.logger.Debugf("[SourceMapCache] File does not exist: %s", dmapPath)
			return nil, fmt.Errorf("source map not found: %s (transpile .dingo file first: dingo build)", dmapPath)
		}
		c.logger.Debugf("[SourceMapCache] Failed to open dmap: %v", err)
		return nil, fmt.Errorf("failed to read source map %s: %w", dmapPath, err)
	}
	c.logger.Debugf("[SourceMapCache] Successfully opened dmap with %d line mappings", reader.LineMappingCount())

	// Store with consistent key (dingoPath)
	c.maps[dingoPath] = reader
	c.logger.Infof("Source map loaded: %s (%d line mappings)", dmapPath, reader.LineMappingCount())

	return reader, nil
}

// Invalidate removes a source map from cache (called after file changes)
func (c *SourceMapCache) Invalidate(goFilePath string) {
	dingoPath := strings.TrimSuffix(goFilePath, ".go") + ".dingo"
	dmapPath := strings.TrimSuffix(dingoPath, ".dingo") + ".dmap"

	c.mu.Lock()
	defer c.mu.Unlock()

	// CRITICAL FIX C3: Use dingoPath as key (consistent with Get())
	if reader, ok := c.maps[dingoPath]; ok {
		reader.Close() // Release resources (currently no-op, but future-proofs for mmap)
		delete(c.maps, dingoPath)
		c.logger.Debugf("Source map invalidated: %s", dmapPath)
	}
}

// InvalidateAll clears the entire cache
func (c *SourceMapCache) InvalidateAll() {
	c.mu.Lock()
	defer c.mu.Unlock()

	count := len(c.maps)

	// Close all readers
	for _, reader := range c.maps {
		reader.Close()
	}

	c.maps = make(map[string]*dmap.Reader)
	c.logger.Infof("All source maps invalidated (%d entries cleared)", count)
}

// Size returns the number of cached source maps
func (c *SourceMapCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.maps)
}

// calculateDmapPath calculates the .dmap file path in the project root .dmap/ folder.
// Example: /project/examples/03_option/user.dingo -> /project/.dmap/examples/03_option/user.dmap
func calculateDmapPath(dingoPath string) (string, error) {
	// Convert to absolute path
	absPath, err := filepath.Abs(dingoPath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Find workspace root (go.mod, go.work, or dingo.toml)
	inputDir := filepath.Dir(absPath)
	workspaceRoot, err := detectWorkspaceRoot(inputDir)
	if err != nil {
		// Fall back to parent directory traversal
		workspaceRoot = inputDir
	}

	// Calculate relative path from workspace root
	relPath, err := filepath.Rel(workspaceRoot, absPath)
	if err != nil {
		return "", fmt.Errorf("failed to calculate relative path: %w", err)
	}

	// Replace .dingo extension with .dmap
	relDmap := strings.TrimSuffix(relPath, ".dingo") + ".dmap"

	// Build final path: workspaceRoot/.dmap/relPath
	return filepath.Join(workspaceRoot, ".dmap", relDmap), nil
}

// detectWorkspaceRoot finds the workspace root by looking for dingo.toml, go.work, or go.mod
func detectWorkspaceRoot(startPath string) (string, error) {
	current := startPath
	for {
		// Check for dingo.toml
		if _, err := os.Stat(filepath.Join(current, "dingo.toml")); err == nil {
			return current, nil
		}

		// Check for go.work
		if _, err := os.Stat(filepath.Join(current, "go.work")); err == nil {
			return current, nil
		}

		// Check for go.mod
		if _, err := os.Stat(filepath.Join(current, "go.mod")); err == nil {
			return current, nil
		}

		// Move to parent directory
		parent := filepath.Dir(current)
		if parent == current {
			// Reached filesystem root
			return startPath, fmt.Errorf("no workspace root found")
		}
		current = parent
	}
}
