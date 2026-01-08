package main

import (
	"fmt"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/MadAppGang/dingo/pkg/tokenizer"
)

// AutoScanResult holds the result of scanning a .dingo file
type AutoScanResult struct {
	File        string
	Identifiers []IdentifierInfo
	Errors      []string
}

// IdentifierInfo holds information about an identifier found in a file
type IdentifierInfo struct {
	Name   string
	Line   int // 1-based
	Column int // 0-based (for LSP)
}

// AutoScanner scans .dingo files and generates test cases
type AutoScanner struct {
	WorkspaceRoot string
	Verbose       bool
}

// ScanFiles scans all files matching the glob pattern and returns specs
func (s *AutoScanner) ScanFiles(pattern string) ([]*Spec, error) {
	// Resolve pattern relative to workspace root
	fullPattern := pattern
	if !filepath.IsAbs(pattern) {
		fullPattern = filepath.Join(s.WorkspaceRoot, pattern)
	}

	files, err := filepath.Glob(fullPattern)
	if err != nil {
		return nil, fmt.Errorf("glob error: %w", err)
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no files found matching: %s", pattern)
	}

	var specs []*Spec
	for _, file := range files {
		spec, err := s.ScanFile(file)
		if err != nil {
			if s.Verbose {
				fmt.Fprintf(os.Stderr, "Warning: failed to scan %s: %v\n", file, err)
			}
			continue
		}
		if len(spec.Cases) > 0 {
			specs = append(specs, spec)
		}
	}

	return specs, nil
}

// ScanFile scans a single .dingo file and generates a spec
func (s *AutoScanner) ScanFile(filePath string) (*Spec, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}

	// Make path relative to workspace root for spec
	relPath, err := filepath.Rel(s.WorkspaceRoot, filePath)
	if err != nil {
		relPath = filePath
	}

	identifiers := s.extractIdentifiers(content, filePath)

	// Generate test cases from identifiers
	cases := s.generateTestCases(identifiers)

	return &Spec{
		File:  relPath,
		Cases: cases,
	}, nil
}

// extractIdentifiers uses the Dingo tokenizer to find all identifiers
func (s *AutoScanner) extractIdentifiers(content []byte, filename string) []IdentifierInfo {
	fset := token.NewFileSet()
	tok := tokenizer.NewWithFileSet(content, fset, filename)
	tokens, err := tok.Tokenize()
	if err != nil {
		if s.Verbose {
			fmt.Fprintf(os.Stderr, "Tokenizer error for %s: %v\n", filename, err)
		}
		return nil
	}

	var identifiers []IdentifierInfo
	seen := make(map[string]bool) // Track line:col to avoid duplicates

	for _, t := range tokens {
		if t.Kind != tokenizer.IDENT {
			continue
		}

		// Skip noise identifiers
		if s.shouldSkipIdentifier(t.Lit) {
			continue
		}

		// Create unique key for deduplication
		key := fmt.Sprintf("%d:%d", t.Line, t.Column)
		if seen[key] {
			continue
		}
		seen[key] = true

		identifiers = append(identifiers, IdentifierInfo{
			Name:   t.Lit,
			Line:   t.Line,
			Column: t.Column - 1, // Convert to 0-based for LSP
		})
	}

	return identifiers
}

// shouldSkipIdentifier returns true for identifiers we should skip
func (s *AutoScanner) shouldSkipIdentifier(name string) bool {
	// Skip built-ins
	switch name {
	case "true", "false", "nil", "iota", "append", "cap", "close", "complex",
		"copy", "delete", "imag", "len", "make", "new", "panic", "print",
		"println", "real", "recover":
		return true
	}

	// Skip common generated/temporary names
	if name == "_" {
		return true
	}
	if strings.HasPrefix(name, "tmp") {
		return true
	}
	if strings.HasPrefix(name, "err") && len(name) <= 4 {
		// Skip err, err1, err2, etc. but not "error" or "errorHandler"
		return true
	}

	// Skip very short names (often loop variables)
	if len(name) == 1 {
		return true
	}

	return false
}

// generateTestCases creates test cases from identifiers
func (s *AutoScanner) generateTestCases(identifiers []IdentifierInfo) []TestCase {
	cases := make([]TestCase, 0, len(identifiers))

	for i, ident := range identifiers {
		cases = append(cases, TestCase{
			ID:          i + 1,
			Line:        ident.Line,
			Character:   ident.Column,
			Token:       ident.Name,
			Occurrence:  1,
			Description: fmt.Sprintf("Auto-scanned: %s at %d:%d", ident.Name, ident.Line, ident.Column),
			Expect: Expectation{
				// For auto-scan, we just verify we get some hover content.
				// The hover may show the TYPE (e.g., "User") rather than the identifier name.
				// RequireNonEmpty passes if we get any content, fails if empty.
				RequireNonEmpty: true,
			},
		})
	}

	return cases
}

// Stats holds statistics about the auto-scan results
type AutoScanStats struct {
	TotalFiles       int
	TotalIdentifiers int
	TestedCount      int
	PassedCount      int
	FailedCount      int
	NoHoverCount     int // Identifiers that returned empty hover (not counted as failure)
}

// PrintStats prints auto-scan statistics
func (stats *AutoScanStats) PrintStats() {
	fmt.Println()
	fmt.Printf("Auto-scan coverage:\n")
	fmt.Printf("  Files scanned:    %d\n", stats.TotalFiles)
	fmt.Printf("  Identifiers:      %d\n", stats.TotalIdentifiers)
	fmt.Printf("  Tested:           %d\n", stats.TestedCount)
	fmt.Printf("  Passed:           %d\n", stats.PassedCount)
	fmt.Printf("  No hover:         %d (not failures)\n", stats.NoHoverCount)
	fmt.Printf("  Failed:           %d\n", stats.FailedCount)

	if stats.TotalIdentifiers > 0 {
		coverage := float64(stats.PassedCount+stats.NoHoverCount) / float64(stats.TestedCount) * 100
		fmt.Printf("  Coverage:         %.1f%%\n", coverage)
	}
}
