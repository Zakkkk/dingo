package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Workspace represents a multi-package Dingo workspace
type Workspace struct {
	Root     string    // Workspace root directory
	Packages []Package // All packages in workspace
}

// Package represents a single package within a workspace
type Package struct {
	Path       string   // Relative path from workspace root
	Name       string   // Package name
	DingoFiles []string // List of .dingo files
	GoFiles    []string // List of .go files (existing)
}

// DetectWorkspaceRoot finds the workspace root by looking for dingo.toml or go.work
func DetectWorkspaceRoot(startPath string) (string, error) {
	current := startPath
	for {
		// Check for dingo.toml
		dingoToml := filepath.Join(current, "dingo.toml")
		if _, err := os.Stat(dingoToml); err == nil {
			return current, nil
		}

		// Check for go.work
		goWork := filepath.Join(current, "go.work")
		if _, err := os.Stat(goWork); err == nil {
			return current, nil
		}

		// Check for go.mod as fallback
		goMod := filepath.Join(current, "go.mod")
		if _, err := os.Stat(goMod); err == nil {
			return current, nil
		}

		// Move to parent directory
		parent := filepath.Dir(current)
		if parent == current {
			// Reached filesystem root without finding workspace marker
			return startPath, fmt.Errorf("no workspace root found (no dingo.toml, go.work, or go.mod)")
		}
		current = parent
	}
}

// ScanWorkspace finds all .dingo packages in the workspace
func ScanWorkspace(root string) (*Workspace, error) {
	ws := &Workspace{
		Root:     root,
		Packages: make([]Package, 0),
	}

	// Read .dingoignore patterns if exists
	ignorePatterns, err := readDingoIgnore(root)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to read .dingoignore: %w", err)
	}

	// Track packages by directory
	packageMap := make(map[string]*Package)

	// Walk directory tree
	err = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories we should ignore
		if info.IsDir() {
			relPath, _ := filepath.Rel(root, path)
			if shouldIgnore(relPath, ignorePatterns) {
				return filepath.SkipDir
			}
			return nil
		}

		// Get relative path and directory
		relPath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}

		// Check if should be ignored
		if shouldIgnore(relPath, ignorePatterns) {
			return nil
		}

		// Get package directory
		pkgDir := filepath.Dir(relPath)
		if pkgDir == "." {
			pkgDir = ""
		}

		// Process .dingo files
		if strings.HasSuffix(path, ".dingo") {
			pkg, exists := packageMap[pkgDir]
			if !exists {
				pkg = &Package{
					Path:       pkgDir,
					Name:       filepath.Base(filepath.Dir(path)),
					DingoFiles: make([]string, 0),
					GoFiles:    make([]string, 0),
				}
				packageMap[pkgDir] = pkg
			}
			pkg.DingoFiles = append(pkg.DingoFiles, relPath)
		}

		// Track .go files (for mixed codebases)
		if strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			pkg, exists := packageMap[pkgDir]
			if !exists {
				pkg = &Package{
					Path:       pkgDir,
					Name:       filepath.Base(filepath.Dir(path)),
					DingoFiles: make([]string, 0),
					GoFiles:    make([]string, 0),
				}
				packageMap[pkgDir] = pkg
			}
			pkg.GoFiles = append(pkg.GoFiles, relPath)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to scan workspace: %w", err)
	}

	// Convert map to slice, filtering out packages without .dingo files
	for _, pkg := range packageMap {
		if len(pkg.DingoFiles) > 0 {
			ws.Packages = append(ws.Packages, *pkg)
		}
	}

	return ws, nil
}

// readDingoIgnore reads ignore patterns from .dingoignore file
func readDingoIgnore(root string) ([]string, error) {
	ignorePath := filepath.Join(root, ".dingoignore")
	data, err := os.ReadFile(ignorePath)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(data), "\n")
	patterns := make([]string, 0)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, line)
	}

	return patterns, nil
}

// shouldIgnore checks if a path matches any ignore pattern
func shouldIgnore(path string, patterns []string) bool {
	// Always ignore these directories
	defaultIgnores := []string{
		".git",
		".dingo-cache",
		"node_modules",
		"vendor",
		".idea",
		".vscode",
	}

	// Check default ignores
	parts := strings.Split(path, string(filepath.Separator))
	for _, part := range parts {
		for _, ignore := range defaultIgnores {
			if part == ignore {
				return true
			}
		}
	}

	// Check custom patterns
	for _, pattern := range patterns {
		// Simple glob matching (basic implementation)
		if matchPattern(path, pattern) {
			return true
		}
	}

	return false
}

// matchPattern performs simple glob pattern matching
func matchPattern(path, pattern string) bool {
	// Handle exact matches
	if path == pattern {
		return true
	}

	// Handle wildcard patterns
	if strings.Contains(pattern, "*") {
		// Simple implementation: just check if pattern parts are in path
		parts := strings.Split(pattern, "*")
		currentPath := path
		for _, part := range parts {
			if part == "" {
				continue
			}
			idx := strings.Index(currentPath, part)
			if idx == -1 {
				return false
			}
			currentPath = currentPath[idx+len(part):]
		}
		return true
	}

	// Handle directory patterns (ending with /)
	if strings.HasSuffix(pattern, "/") {
		return strings.HasPrefix(path, pattern) || strings.Contains(path, "/"+pattern)
	}

	return false
}

// MatchesPattern checks if a path matches a package pattern (e.g., "./...", "./pkg/...")
func MatchesPattern(path, pattern string) bool {
	// Normalize paths
	path = filepath.Clean(path)
	pattern = filepath.Clean(pattern)

	// Handle ./... pattern (all packages)
	if pattern == "./..." || pattern == "..." {
		return true
	}

	// Handle ./pkg/... pattern (package and subpackages)
	if strings.HasSuffix(pattern, "/...") {
		prefix := strings.TrimSuffix(pattern, "/...")
		prefix = strings.TrimPrefix(prefix, "./")
		if prefix == "" {
			return true
		}
		return strings.HasPrefix(path, prefix+"/") || path == prefix
	}

	// Handle exact package match
	pattern = strings.TrimPrefix(pattern, "./")
	return path == pattern
}
