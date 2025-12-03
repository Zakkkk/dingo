package typeloader

import (
	"fmt"
	"go/parser"
	"go/token"
	"regexp"
	"strings"
)

// ExtractImports extracts import paths from Dingo source code
// Returns a list of import paths (e.g., "fmt", "os", "database/sql")
// Uses go/parser for speed, with regex fallback for Dingo-specific syntax
func ExtractImports(source []byte) ([]string, error) {
	// Validate input size to prevent ReDoS attacks
	if len(source) > 1_000_000 { // 1MB limit
		return nil, fmt.Errorf("source file too large for processing (>1MB): %d bytes", len(source))
	}

	// Try fast path: use go/parser with ImportsOnly mode
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", source, parser.ImportsOnly)

	if err == nil && file != nil {
		// Success - extract import paths from AST
		seen := make(map[string]bool)
		imports := make([]string, 0, len(file.Imports))
		for _, imp := range file.Imports {
			// Remove quotes from import path
			path := strings.Trim(imp.Path.Value, `"`)
			if !seen[path] {
				imports = append(imports, path)
				seen[path] = true
			}
		}
		return imports, nil
	}

	// Fallback: use regex extraction if parser fails
	// This handles cases where Dingo syntax confuses the parser
	return extractImportsRegex(source)
}

// extractImportsRegex is a fallback that uses regex to extract imports
// Handles both single imports and grouped import blocks
func extractImportsRegex(source []byte) ([]string, error) {
	var imports []string
	seen := make(map[string]bool) // Deduplicate

	sourceStr := string(source)

	// Pattern 1: Single import statements
	// import "package/path"
	singlePattern := regexp.MustCompile(`(?m)^\s*import\s+"([^"]+)"`)
	matches := singlePattern.FindAllStringSubmatch(sourceStr, -1)
	for _, match := range matches {
		if len(match) > 1 {
			path := match[1]
			if !seen[path] {
				imports = append(imports, path)
				seen[path] = true
			}
		}
	}

	// Pattern 2: Grouped imports
	// import (
	//     "fmt"
	//     "os"
	// )
	groupPattern := regexp.MustCompile(`(?s)import\s*\(\s*([^)]+)\)`)
	groupMatches := groupPattern.FindAllStringSubmatch(sourceStr, -1)

	for _, groupMatch := range groupMatches {
		if len(groupMatch) > 1 {
			block := groupMatch[1]
			// Extract individual imports from the block
			importPattern := regexp.MustCompile(`"([^"]+)"`)
			blockImports := importPattern.FindAllStringSubmatch(block, -1)
			for _, imp := range blockImports {
				if len(imp) > 1 {
					path := imp[1]
					if !seen[path] {
						imports = append(imports, path)
						seen[path] = true
					}
				}
			}
		}
	}

	// If we found no imports, this might be an error or a file with no imports
	// Return empty list (not an error - file might legitimately have no imports)
	return imports, nil
}

// ExtractImportsWithAliases extracts imports with their aliases (if any)
// Returns a map of alias -> import path
// For example: io "io/ioutil" -> map["io"] = "io/ioutil"
// Regular imports use the package name as key: import "fmt" -> map["fmt"] = "fmt"
func ExtractImportsWithAliases(source []byte) (map[string]string, error) {
	result := make(map[string]string)

	// Try fast path: use go/parser
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", source, parser.ImportsOnly)

	if err == nil && file != nil {
		for _, imp := range file.Imports {
			path := strings.Trim(imp.Path.Value, `"`)

			var alias string
			if imp.Name != nil {
				// Explicit alias (including ".")
				alias = imp.Name.Name
			} else {
				// No alias - use package name (last component)
				parts := strings.Split(path, "/")
				alias = parts[len(parts)-1]
			}

			result[alias] = path
		}
		return result, nil
	}

	// Fallback: regex extraction
	return extractImportsWithAliasesRegex(source)
}

// extractImportsWithAliasesRegex is fallback for alias extraction
func extractImportsWithAliasesRegex(source []byte) (map[string]string, error) {
	result := make(map[string]string)
	sourceStr := string(source)

	// Pattern 1: Aliased imports
	// alias "package/path"
	aliasPattern := regexp.MustCompile(`(?m)^\s*import\s+(\w+)\s+"([^"]+)"`)
	matches := aliasPattern.FindAllStringSubmatch(sourceStr, -1)
	for _, match := range matches {
		if len(match) > 2 {
			alias := match[1]
			path := match[2]
			result[alias] = path
		}
	}

	// Pattern 2: Regular imports (no alias)
	// import "package/path"
	regularPattern := regexp.MustCompile(`(?m)^\s*import\s+"([^"]+)"`)
	regularMatches := regularPattern.FindAllStringSubmatch(sourceStr, -1)
	for _, match := range regularMatches {
		if len(match) > 1 {
			path := match[1]
			// Use last component as alias
			parts := strings.Split(path, "/")
			alias := parts[len(parts)-1]
			result[alias] = path
		}
	}

	// Pattern 3: Grouped imports
	groupPattern := regexp.MustCompile(`(?s)import\s*\(\s*([^)]+)\)`)
	groupMatches := groupPattern.FindAllStringSubmatch(sourceStr, -1)

	for _, groupMatch := range groupMatches {
		if len(groupMatch) > 1 {
			block := groupMatch[1]

			// Extract aliased imports from block
			aliasedPattern := regexp.MustCompile(`(?m)^\s*(\w+)\s+"([^"]+)"`)
			aliased := aliasedPattern.FindAllStringSubmatch(block, -1)
			for _, imp := range aliased {
				if len(imp) > 2 {
					alias := imp[1]
					path := imp[2]
					result[alias] = path
				}
			}

			// Extract regular imports from block
			regularPattern := regexp.MustCompile(`(?m)^\s*"([^"]+)"`)
			regular := regularPattern.FindAllStringSubmatch(block, -1)
			for _, imp := range regular {
				if len(imp) > 1 {
					path := imp[1]
					parts := strings.Split(path, "/")
					alias := parts[len(parts)-1]
					result[alias] = path
				}
			}
		}
	}

	return result, nil
}

// ValidateImports checks if imports can be loaded from the current environment
// Returns a list of import paths that failed to validate
// This is a quick pre-check before attempting full type loading
func ValidateImports(imports []string, workingDir string) ([]string, error) {
	// Use go list to quickly check if packages exist
	// This is faster than full go/packages loading

	var failed []string

	// For now, just return empty (all valid)
	// Full validation will happen in the loader
	// This is a placeholder for future optimization

	return failed, nil
}

// DeduplicateImports removes duplicate import paths
func DeduplicateImports(imports []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(imports))

	for _, imp := range imports {
		if !seen[imp] {
			result = append(result, imp)
			seen[imp] = true
		}
	}

	return result
}

// FormatImportError creates a user-friendly error message for import issues
func FormatImportError(importPath string, err error) error {
	return fmt.Errorf(
		"failed to process import %q: %w\n\n"+
		"Troubleshooting:\n"+
		"  1. Ensure the package exists: go get %s\n"+
		"  2. Run 'go mod download' to fetch dependencies\n"+
		"  3. Check that go.mod is in the current directory",
		importPath, err, importPath)
}
