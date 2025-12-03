package typeloader

import (
	"sync"
	"testing"
)

func TestBuildCache_LoadImports_EmptyImports(t *testing.T) {
	cache := NewBuildCache(LoaderConfig{})

	result, err := cache.LoadImports([]string{})
	if err != nil {
		t.Fatalf("Expected no error for empty imports, got: %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result for empty imports")
	}

	if len(result.Functions) != 0 {
		t.Errorf("Expected empty Functions map, got %d entries", len(result.Functions))
	}
}

func TestBuildCache_LoadImports_CacheMiss(t *testing.T) {
	cache := NewBuildCache(LoaderConfig{
		WorkingDir: "../../", // Project root
		FailFast:   true,
	})
	defer cache.Clear()

	// First load - cache miss
	result, err := cache.LoadImports([]string{"fmt"})
	if err != nil {
		t.Fatalf("Failed to load fmt: %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	// Should have loaded some functions from fmt
	if len(result.Functions) == 0 {
		t.Error("Expected functions from fmt package")
	}

	// Check that fmt is now cached
	cache.mu.RLock()
	_, cached := cache.packages["fmt"]
	cache.mu.RUnlock()

	if !cached {
		t.Error("Expected fmt to be cached after first load")
	}
}

func TestBuildCache_LoadImports_CacheHit(t *testing.T) {
	cache := NewBuildCache(LoaderConfig{
		WorkingDir: "../../",
		FailFast:   true,
	})
	defer cache.Clear()

	// First load
	result1, err := cache.LoadImports([]string{"fmt"})
	if err != nil {
		t.Fatalf("Failed to load fmt: %v", err)
	}

	// Second load - should hit cache
	result2, err := cache.LoadImports([]string{"fmt"})
	if err != nil {
		t.Fatalf("Failed to load fmt from cache: %v", err)
	}

	// Results should be equivalent (same functions)
	if len(result1.Functions) != len(result2.Functions) {
		t.Errorf("Cache hit result differs: %d vs %d functions",
			len(result1.Functions), len(result2.Functions))
	}
}

func TestBuildCache_LoadImports_PartialCacheHit(t *testing.T) {
	cache := NewBuildCache(LoaderConfig{
		WorkingDir: "../../",
		FailFast:   true,
	})
	defer cache.Clear()

	// Load fmt first
	_, err := cache.LoadImports([]string{"fmt"})
	if err != nil {
		t.Fatalf("Failed to load fmt: %v", err)
	}

	// Load fmt + os (fmt cached, os not)
	result, err := cache.LoadImports([]string{"fmt", "os"})
	if err != nil {
		t.Fatalf("Failed to load fmt+os: %v", err)
	}

	// Should have functions from both packages
	if len(result.Functions) == 0 {
		t.Error("Expected functions from merged result")
	}

	// Both should now be cached
	cache.mu.RLock()
	_, fmtCached := cache.packages["fmt"]
	_, osCached := cache.packages["os"]
	cache.mu.RUnlock()

	if !fmtCached {
		t.Error("Expected fmt to remain cached")
	}
	if !osCached {
		t.Error("Expected os to be cached after load")
	}
}

func TestBuildCache_LoadImports_MultiplePackages(t *testing.T) {
	cache := NewBuildCache(LoaderConfig{
		WorkingDir: "../../",
		FailFast:   true,
	})
	defer cache.Clear()

	// Load multiple packages at once
	result, err := cache.LoadImports([]string{"fmt", "os", "io"})
	if err != nil {
		t.Fatalf("Failed to load multiple packages: %v", err)
	}

	// Should have functions from all three
	if len(result.Functions) == 0 {
		t.Error("Expected functions from multiple packages")
	}

	// All should be cached
	cache.mu.RLock()
	fmtCount := len(cache.packages)
	cache.mu.RUnlock()

	if fmtCount != 3 {
		t.Errorf("Expected 3 packages cached, got %d", fmtCount)
	}
}

func TestBuildCache_Clear(t *testing.T) {
	cache := NewBuildCache(LoaderConfig{
		WorkingDir: "../../",
		FailFast:   true,
	})

	// Load a package
	_, err := cache.LoadImports([]string{"fmt"})
	if err != nil {
		t.Fatalf("Failed to load fmt: %v", err)
	}

	// Verify cached
	cache.mu.RLock()
	countBefore := len(cache.packages)
	cache.mu.RUnlock()

	if countBefore == 0 {
		t.Error("Expected packages to be cached before Clear")
	}

	// Clear cache
	cache.Clear()

	// Verify empty
	cache.mu.RLock()
	countAfter := len(cache.packages)
	cache.mu.RUnlock()

	if countAfter != 0 {
		t.Errorf("Expected cache to be empty after Clear, got %d packages", countAfter)
	}
}

func TestBuildCache_ConcurrentAccess(t *testing.T) {
	cache := NewBuildCache(LoaderConfig{
		WorkingDir: "../../",
		FailFast:   true,
	})
	defer cache.Clear()

	var wg sync.WaitGroup
	errors := make(chan error, 10)

	// Simulate concurrent file processing
	packages := [][]string{
		{"fmt"},
		{"os"},
		{"fmt", "os"},
		{"io"},
		{"fmt", "io"},
		{"os", "io"},
		{"fmt"},
		{"os"},
		{"io"},
		{"fmt", "os", "io"},
	}

	for _, imports := range packages {
		wg.Add(1)
		go func(imps []string) {
			defer wg.Done()
			_, err := cache.LoadImports(imps)
			if err != nil {
				errors <- err
			}
		}(imports)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Errorf("Concurrent access error: %v", err)
	}

	// Verify all packages cached
	cache.mu.RLock()
	cachedCount := len(cache.packages)
	cache.mu.RUnlock()

	// Should have at least fmt, os, io cached
	if cachedCount < 3 {
		t.Errorf("Expected at least 3 packages cached, got %d", cachedCount)
	}
}

func TestBuildCache_findUncached(t *testing.T) {
	cache := NewBuildCache(LoaderConfig{})

	// Manually add some cached packages
	cache.mu.Lock()
	cache.packages["fmt"] = &LoadResult{}
	cache.packages["os"] = &LoadResult{}
	cache.mu.Unlock()

	tests := []struct {
		name     string
		imports  []string
		expected []string
	}{
		{
			name:     "all cached",
			imports:  []string{"fmt", "os"},
			expected: nil,
		},
		{
			name:     "all uncached",
			imports:  []string{"io", "net/http"},
			expected: []string{"io", "net/http"},
		},
		{
			name:     "mixed",
			imports:  []string{"fmt", "io", "os", "net/http"},
			expected: []string{"io", "net/http"},
		},
		{
			name:     "empty",
			imports:  []string{},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache.mu.RLock()
			uncached := cache.findUncached(tt.imports)
			cache.mu.RUnlock()

			if len(uncached) != len(tt.expected) {
				t.Errorf("Expected %d uncached, got %d", len(tt.expected), len(uncached))
				return
			}

			// Check all expected are in uncached
			uncachedMap := make(map[string]bool)
			for _, u := range uncached {
				uncachedMap[u] = true
			}

			for _, exp := range tt.expected {
				if !uncachedMap[exp] {
					t.Errorf("Expected %q to be uncached", exp)
				}
			}
		})
	}
}

func TestBuildCache_mergeFromCache(t *testing.T) {
	cache := NewBuildCache(LoaderConfig{})

	// Manually add cached results
	cache.mu.Lock()
	cache.packages["fmt"] = &LoadResult{
		Functions: map[string]*FunctionSignature{
			"fmt.Println": {Name: "Println", Package: "fmt"},
		},
		Methods:        make(map[string]*FunctionSignature),
		LocalFunctions: make(map[string]*FunctionSignature),
	}
	cache.packages["os"] = &LoadResult{
		Functions: map[string]*FunctionSignature{
			"os.Open": {Name: "Open", Package: "os"},
		},
		Methods:        make(map[string]*FunctionSignature),
		LocalFunctions: make(map[string]*FunctionSignature),
	}
	cache.mu.Unlock()

	// Merge fmt + os (acquire read lock as required)
	cache.mu.RLock()
	merged := cache.mergeFromCacheUnsafe([]string{"fmt", "os"})
	cache.mu.RUnlock()

	if len(merged.Functions) != 2 {
		t.Errorf("Expected 2 functions in merged result, got %d", len(merged.Functions))
	}

	if _, ok := merged.Functions["fmt.Println"]; !ok {
		t.Error("Expected fmt.Println in merged result")
	}

	if _, ok := merged.Functions["os.Open"]; !ok {
		t.Error("Expected os.Open in merged result")
	}
}

func TestBuildCache_LoadImports_FailFast(t *testing.T) {
	cache := NewBuildCache(LoaderConfig{
		WorkingDir: "../../",
		FailFast:   true,
	})
	defer cache.Clear()

	// Try to load non-existent package
	_, err := cache.LoadImports([]string{"nonexistent/fake/package"})
	if err == nil {
		t.Error("Expected error for non-existent package")
	}

	// Cache should not contain failed package
	cache.mu.RLock()
	_, cached := cache.packages["nonexistent/fake/package"]
	cache.mu.RUnlock()

	if cached {
		t.Error("Failed package should not be cached")
	}
}

func TestBuildCache_ThreadSafety(t *testing.T) {
	cache := NewBuildCache(LoaderConfig{
		WorkingDir: "../../",
		FailFast:   true,
	})
	defer cache.Clear()

	var wg sync.WaitGroup

	// Concurrent loads
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cache.LoadImports([]string{"fmt"})
		}()
	}

	// Concurrent clears
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cache.Clear()
		}()
	}

	// Concurrent reads
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cache.mu.RLock()
			_ = len(cache.packages)
			cache.mu.RUnlock()
		}()
	}

	wg.Wait()
	// Test passes if no race conditions detected
}
