package lsp

import (
	"fmt"
	"os"
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
// Path translation: goFilePath (.go) → dingoPath (.dingo) → dmapPath (.dmap)
func (c *SourceMapCache) Get(goFilePath string) (*dmap.Reader, error) {
	// Translate: foo.go -> foo.dingo -> foo.dmap
	dingoPath := strings.TrimSuffix(goFilePath, ".go") + ".dingo"
	dmapPath := strings.TrimSuffix(dingoPath, ".dingo") + ".dmap"

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
	reader, err := dmap.Open(dmapPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("source map not found: %s (transpile .dingo file first: dingo build)", dmapPath)
		}
		return nil, fmt.Errorf("failed to read source map %s: %w", dmapPath, err)
	}

	// Store with consistent key (dingoPath)
	c.maps[dingoPath] = reader
	c.logger.Infof("Source map loaded: %s (%d entries)", dmapPath, reader.EntryCount())

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
