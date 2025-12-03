package typeloader

import (
	"strings"
	"sync"

	"golang.org/x/sync/singleflight"
)

// BuildCache provides per-build caching for type loading
// Thread-safe for concurrent file processing within a build
type BuildCache struct {
	mu       sync.RWMutex
	packages map[string]*LoadResult // Keyed by import path
	loader   *Loader
	group    singleflight.Group // Deduplicates concurrent loads of same imports
}

// NewBuildCache creates a cache for a single build invocation
func NewBuildCache(config LoaderConfig) *BuildCache {
	return &BuildCache{
		packages: make(map[string]*LoadResult),
		loader:   NewLoader(config),
	}
}

// LoadImports loads types for given imports, using cache when available
// Returns merged LoadResult from all requested imports
// Uses singleflight to deduplicate concurrent loads of the same imports
func (c *BuildCache) LoadImports(imports []string) (*LoadResult, error) {
	if len(imports) == 0 {
		return &LoadResult{
			Functions:      make(map[string]*FunctionSignature),
			Methods:        make(map[string]*FunctionSignature),
			LocalFunctions: make(map[string]*FunctionSignature),
		}, nil
	}

	// Check cache first (read lock)
	c.mu.RLock()
	uncached := c.findUncached(imports)
	if len(uncached) == 0 {
		// All cached - merge and return while holding read lock
		result := c.mergeFromCacheUnsafe(imports)
		c.mu.RUnlock()
		return result, nil
	}
	c.mu.RUnlock()

	// Use singleflight to deduplicate concurrent loads
	// Multiple goroutines requesting same imports will share one load operation
	key := strings.Join(uncached, ",")
	val, err, _ := c.group.Do(key, func() (interface{}, error) {
		return c.loader.LoadFromImports(uncached)
	})

	if err != nil {
		return nil, err // Fail fast
	}

	result := val.(*LoadResult)

	// Update cache (write lock)
	c.mu.Lock()
	c.updateCache(uncached, result)
	c.mu.Unlock()

	// Return merged result from all requested imports (read lock)
	c.mu.RLock()
	merged := c.mergeFromCacheUnsafe(imports)
	c.mu.RUnlock()

	return merged, nil
}

// Clear clears the cache (for testing or end of build)
func (c *BuildCache) Clear() {
	c.mu.Lock()
	c.packages = make(map[string]*LoadResult)
	c.mu.Unlock()
}

// findUncached returns the subset of imports not in cache
// Caller must hold read lock
func (c *BuildCache) findUncached(imports []string) []string {
	var uncached []string
	for _, imp := range imports {
		if _, ok := c.packages[imp]; !ok {
			uncached = append(uncached, imp)
		}
	}
	return uncached
}

// updateCache stores loaded results in cache per import path
// Caller must hold write lock
func (c *BuildCache) updateCache(imports []string, result *LoadResult) {
	// Store each import separately for future cache hits
	for _, imp := range imports {
		// Create a LoadResult for this specific import by filtering
		importResult := &LoadResult{
			Functions:      make(map[string]*FunctionSignature),
			Methods:        make(map[string]*FunctionSignature),
			LocalFunctions: make(map[string]*FunctionSignature),
		}

		// Filter functions/methods that belong to this import
		for key, sig := range result.Functions {
			if sig.Package == imp {
				importResult.Functions[key] = sig
			}
		}
		for key, sig := range result.Methods {
			if sig.Package == imp {
				importResult.Methods[key] = sig
			}
		}

		c.packages[imp] = importResult
	}
}

// mergeFromCacheUnsafe merges LoadResults for all requested imports
// IMPORTANT: Caller MUST hold read lock before calling this function
// This function does not acquire locks to prevent recursive locking deadlocks
func (c *BuildCache) mergeFromCacheUnsafe(imports []string) *LoadResult {
	merged := &LoadResult{
		Functions:      make(map[string]*FunctionSignature),
		Methods:        make(map[string]*FunctionSignature),
		LocalFunctions: make(map[string]*FunctionSignature),
	}

	for _, imp := range imports {
		if cached, ok := c.packages[imp]; ok {
			// Merge functions
			for key, sig := range cached.Functions {
				merged.Functions[key] = sig
			}
			// Merge methods
			for key, sig := range cached.Methods {
				merged.Methods[key] = sig
			}
			// LocalFunctions are per-file, not cached per import
		}
	}

	return merged
}
