// Package shadow provides shadow build directory management for the Dingo compiler.
// It creates a complete Go module in a build directory, allowing clean source
// directories while maintaining full Go toolchain compatibility.
package shadow

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/MadAppGang/dingo/pkg/config"
	"github.com/MadAppGang/dingo/pkg/sourcemap/dmap"
	"github.com/MadAppGang/dingo/pkg/transpiler"
	"github.com/MadAppGang/dingo/pkg/typeloader"
)

// ProgressCallback is called during build to report progress
type ProgressCallback func(current, total int, file string)

// Builder creates and manages shadow build directories
type Builder struct {
	// WorkspaceRoot is the path containing go.mod
	WorkspaceRoot string

	// ShadowDir is the path to the build directory (e.g., "build")
	ShadowDir string

	// Config is the dingo configuration
	Config *config.Config

	// Verbose enables verbose output
	Verbose bool

	// Debug enables debug mode: emits //line directives for Delve debugging
	Debug bool

	// OnProgress is called during transpilation to report progress
	OnProgress ProgressCallback

	// generatedFiles tracks files we've transpiled (to avoid copying them as pure Go)
	generatedFiles map[string]bool

	// TypeCache provides pre-loaded type information for multi-file builds.
	// When DINGO_FULL_TYPE_RESOLUTION=1, this cache is populated before
	// transpilation with all imports from all files, enabling fast type
	// lookup during transpilation (~0.02ms instead of ~1.4s per file).
	TypeCache *typeloader.BuildCache
}

// NewBuilder creates a new shadow builder
func NewBuilder(workspaceRoot, shadowDir string, cfg *config.Config) *Builder {
	if shadowDir == "" {
		shadowDir = "build"
	}
	return &Builder{
		WorkspaceRoot:  workspaceRoot,
		ShadowDir:      filepath.Join(workspaceRoot, shadowDir),
		Config:         cfg,
		generatedFiles: make(map[string]bool),
	}
}

// BuildResult contains the result of building the shadow directory
type BuildResult struct {
	// ShadowDir is the absolute path to the shadow directory
	ShadowDir string

	// GeneratedFiles is the list of .go files generated from .dingo files
	GeneratedFiles []string

	// CopiedFiles is the list of files copied to the shadow directory
	CopiedFiles []string

	// TranspiledCount is the number of files actually transpiled (not skipped)
	TranspiledCount int

	// SkippedCount is the number of files skipped (already up to date)
	SkippedCount int
}

// Build creates the shadow directory with all necessary files.
// If dingoFiles is empty, it discovers all .dingo files in the workspace.
// Uses incremental builds - only transpiles files that changed since last build.
func (b *Builder) Build(dingoFiles []string) (*BuildResult, error) {
	result := &BuildResult{
		ShadowDir: b.ShadowDir,
	}

	// 1. Ensure shadow directory exists
	if err := os.MkdirAll(b.ShadowDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create shadow directory: %w", err)
	}

	// 2. Copy go.mod and go.sum
	if err := b.copyModFiles(); err != nil {
		return nil, fmt.Errorf("failed to copy module files: %w", err)
	}
	result.CopiedFiles = append(result.CopiedFiles, "go.mod", "go.sum")

	// 3. Discover ALL .dingo files in the workspace
	// We must transpile all .dingo files for the module to build correctly,
	// since pure Go files are skipped if they have a .dingo counterpart.
	// The dingoFiles parameter only affects which package is built at the end,
	// not which files are transpiled.
	targetDingoFiles, err := b.findAllDingoFiles()
	if err != nil {
		return nil, fmt.Errorf("failed to find .dingo files: %w", err)
	}
	_ = dingoFiles // dingoFiles is used by caller for go build path, not for transpilation

	// 4. Filter to only files that need transpilation (incremental build)
	var filesToTranspile []string
	for _, dingoFile := range targetDingoFiles {
		if b.needsTranspile(dingoFile) {
			filesToTranspile = append(filesToTranspile, dingoFile)
		} else {
			// Track as generated even if skipped (for pure Go copy logic)
			relPath, _ := filepath.Rel(b.WorkspaceRoot, dingoFile)
			goRelPath := strings.TrimSuffix(relPath, ".dingo") + ".go"
			b.generatedFiles[goRelPath] = true
			// Add to result (existing file)
			goFile := filepath.Join(b.ShadowDir, goRelPath)
			result.GeneratedFiles = append(result.GeneratedFiles, goFile)
			result.SkippedCount++
		}
	}

	// 4.5. Pre-load types for all files (performance optimization)
	// Load types for ALL imports ONCE before transpilation.
	// This is fast (~150ms total) and enables accurate cross-file type resolution.
	if len(filesToTranspile) > 0 {
		if b.Verbose {
			fmt.Println("Pre-loading types for all files...")
		}

		startTime := time.Now()
		allImports := b.collectAllImports(filesToTranspile)

		b.TypeCache = typeloader.NewBuildCache(typeloader.LoaderConfig{
			WorkingDir: b.WorkspaceRoot,
			FailFast:   false, // Don't fail on first error - continue with partial type info
		})

		_, err := b.TypeCache.LoadImports(allImports)
		if err != nil {
			// Non-fatal: continue without type cache
			if b.Verbose {
				fmt.Fprintf(os.Stderr, "Warning: Failed to pre-load types: %v\n", err)
			}
			b.TypeCache = nil
		} else if b.Verbose {
			fmt.Printf("Pre-loaded %d packages in %v\n", len(allImports), time.Since(startTime))
		}
	}

	// 5. Transpile only changed .dingo files
	total := len(filesToTranspile)
	for i, dingoFile := range filesToTranspile {
		// Report progress
		if b.OnProgress != nil {
			relPath, _ := filepath.Rel(b.WorkspaceRoot, dingoFile)
			b.OnProgress(i+1, total, relPath)
		}

		goFile, err := b.transpileFile(dingoFile)
		if err != nil {
			return nil, fmt.Errorf("failed to transpile %s: %w", dingoFile, err)
		}
		result.GeneratedFiles = append(result.GeneratedFiles, goFile)
		result.TranspiledCount++

		// Track generated file to avoid copying it as pure Go
		relPath, _ := filepath.Rel(b.WorkspaceRoot, dingoFile)
		goRelPath := strings.TrimSuffix(relPath, ".dingo") + ".go"
		b.generatedFiles[goRelPath] = true
	}

	// 6. Copy pure .go files to shadow
	copiedGo, err := b.copyPureGoFiles()
	if err != nil {
		return nil, fmt.Errorf("failed to copy Go files: %w", err)
	}
	result.CopiedFiles = append(result.CopiedFiles, copiedGo...)

	// 7. Handle vendor directory if present
	if err := b.handleVendor(); err != nil {
		return nil, fmt.Errorf("failed to handle vendor directory: %w", err)
	}

	return result, nil
}

// needsTranspile checks if a .dingo file needs to be transpiled.
// Returns true if the .go file doesn't exist or is older than the .dingo file.
func (b *Builder) needsTranspile(dingoFile string) bool {
	// Calculate the output .go file path
	relPath, err := filepath.Rel(b.WorkspaceRoot, dingoFile)
	if err != nil {
		return true // Can't determine, transpile to be safe
	}
	goRelPath := strings.TrimSuffix(relPath, ".dingo") + ".go"
	goFile := filepath.Join(b.ShadowDir, goRelPath)

	// Check if .go file exists
	goInfo, err := os.Stat(goFile)
	if err != nil {
		return true // .go doesn't exist, needs transpile
	}

	// Check if .dingo is newer than .go
	dingoInfo, err := os.Stat(dingoFile)
	if err != nil {
		return true // Can't stat .dingo, transpile to be safe
	}

	// Transpile if .dingo is newer than .go
	return dingoInfo.ModTime().After(goInfo.ModTime())
}

// findAllDingoFiles discovers all .dingo files in the workspace
func (b *Builder) findAllDingoFiles() ([]string, error) {
	var files []string

	err := filepath.Walk(b.WorkspaceRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories we don't want to process
		if info.IsDir() {
			name := info.Name()
			// Skip hidden dirs, shadow dir, vendor, node_modules, editors, tests
			if strings.HasPrefix(name, ".") ||
				path == b.ShadowDir ||
				name == "vendor" ||
				name == "node_modules" ||
				name == "testdata" ||
				name == "editors" ||
				name == "tests" {
				return filepath.SkipDir
			}
			return nil
		}

		// Only process .dingo files
		if strings.HasSuffix(path, ".dingo") {
			files = append(files, path)
		}

		return nil
	})

	return files, err
}

// copyModFiles copies go.mod and go.sum to the shadow directory,
// adjusting relative replace directives as needed
func (b *Builder) copyModFiles() error {
	// Copy go.mod with path adjustments
	srcMod := filepath.Join(b.WorkspaceRoot, "go.mod")
	dstMod := filepath.Join(b.ShadowDir, "go.mod")

	if err := b.copyGoMod(srcMod, dstMod); err != nil {
		return fmt.Errorf("go.mod: %w", err)
	}

	// Copy go.sum as-is (if exists)
	srcSum := filepath.Join(b.WorkspaceRoot, "go.sum")
	dstSum := filepath.Join(b.ShadowDir, "go.sum")

	if _, err := os.Stat(srcSum); err == nil {
		if err := copyFile(srcSum, dstSum); err != nil {
			return fmt.Errorf("go.sum: %w", err)
		}
	}

	return nil
}

// copyGoMod copies go.mod, adjusting relative replace directives
func (b *Builder) copyGoMod(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	// Calculate depth of shadow dir relative to workspace root
	relShadow, _ := filepath.Rel(b.WorkspaceRoot, b.ShadowDir)
	depth := len(strings.Split(relShadow, string(os.PathSeparator)))

	scanner := bufio.NewScanner(srcFile)
	writer := bufio.NewWriter(dstFile)
	defer writer.Flush()

	for scanner.Scan() {
		line := scanner.Text()

		// Adjust relative replace directives
		// Format: replace module => ../relative/path
		if strings.HasPrefix(strings.TrimSpace(line), "replace ") && strings.Contains(line, "=>") {
			parts := strings.SplitN(line, "=>", 2)
			if len(parts) == 2 {
				target := strings.TrimSpace(parts[1])
				// Check if it's a relative path (starts with . or ..)
				if strings.HasPrefix(target, ".") {
					// Add ../ for each level of shadow depth
					prefix := strings.Repeat("../", depth)
					newTarget := prefix + target
					line = parts[0] + "=> " + newTarget
				}
			}
		}

		fmt.Fprintln(writer, line)
	}

	return scanner.Err()
}

// collectAllImports scans all files and returns unique imports.
// This is fast (~1ms per file) because it only tokenizes imports, not full parsing.
// Used for pre-loading all types before transpilation.
//
// IMPORTANT: Excludes imports from the current module (local packages).
// Local packages may contain .dingo files that haven't been transpiled yet,
// so go/packages.Load() would fail on them.
func (b *Builder) collectAllImports(files []string) []string {
	collector := transpiler.ImportCollector{}
	importSet := make(map[string]bool)

	// Get the current module name to filter out local imports
	moduleName := b.getModuleName()

	for _, file := range files {
		src, err := os.ReadFile(file)
		if err != nil {
			if b.Verbose {
				fmt.Fprintf(os.Stderr, "Warning: Failed to read %s: %v\n", file, err)
			}
			continue
		}

		imports, err := collector.CollectImports(src)
		if err != nil {
			if b.Verbose {
				fmt.Fprintf(os.Stderr, "Warning: Failed to collect imports from %s: %v\n", file, err)
			}
			continue
		}

		for _, imp := range imports {
			// Skip local imports (from the current module)
			// These can't be loaded via go/packages because they may contain .dingo files
			if moduleName != "" && strings.HasPrefix(imp, moduleName) {
				continue
			}
			importSet[imp] = true
		}
	}

	// Convert set to slice
	allImports := make([]string, 0, len(importSet))
	for imp := range importSet {
		allImports = append(allImports, imp)
	}

	return allImports
}

// getModuleName reads the module name from go.mod in the workspace root.
// Returns empty string if go.mod doesn't exist or can't be read.
func (b *Builder) getModuleName() string {
	goModPath := filepath.Join(b.WorkspaceRoot, "go.mod")
	f, err := os.Open(goModPath)
	if err != nil {
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module "))
		}
	}
	return ""
}

// transpileFile transpiles a single .dingo file to the shadow directory
func (b *Builder) transpileFile(dingoFile string) (string, error) {
	// Make path absolute for reliable relative path calculation
	absDingoFile, err := filepath.Abs(dingoFile)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Read source
	src, err := os.ReadFile(absDingoFile)
	if err != nil {
		return "", err
	}

	// Transpile with options (debug mode emits //line directives for Delve)
	opts := transpiler.TranspileOptions{
		InferTypes: true,
		Debug:      b.Debug,
		TypeCache:  b.TypeCache, // Pass pre-loaded types for performance
	}
	result, err := transpiler.PureASTTranspileWithMappingsOpts(src, absDingoFile, opts)
	if err != nil {
		return "", err
	}

	// Calculate output path in shadow directory
	relPath, err := filepath.Rel(b.WorkspaceRoot, absDingoFile)
	if err != nil {
		return "", fmt.Errorf("failed to calculate relative path: %w", err)
	}

	goRelPath := strings.TrimSuffix(relPath, ".dingo") + ".go"
	goFile := filepath.Join(b.ShadowDir, goRelPath)

	// Ensure output directory exists
	if err := os.MkdirAll(filepath.Dir(goFile), 0755); err != nil {
		return "", err
	}

	// Write .go file
	if err := os.WriteFile(goFile, result.GoCode, 0644); err != nil {
		return "", err
	}

	// Write .dmap file to .dmap/ folder (in workspace root, not shadow)
	dmapPath := filepath.Join(b.WorkspaceRoot, ".dmap", strings.TrimSuffix(relPath, ".dingo")+".dmap")
	if err := os.MkdirAll(filepath.Dir(dmapPath), 0755); err == nil {
		writer := dmap.NewWriter(result.DingoSource, result.GoCode)
		_ = writer.WriteFile(dmapPath, result.LineMappings, result.ColumnMappings)
	}

	return goFile, nil
}

// copyPureGoFiles copies non-generated .go files to the shadow directory
func (b *Builder) copyPureGoFiles() ([]string, error) {
	var copied []string

	err := filepath.Walk(b.WorkspaceRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories we don't want to process
		if info.IsDir() {
			name := info.Name()
			// Skip hidden dirs, shadow dir, vendor, node_modules, editors, tests
			if strings.HasPrefix(name, ".") ||
				path == b.ShadowDir ||
				name == "vendor" ||
				name == "node_modules" ||
				name == "testdata" ||
				name == "editors" ||
				name == "tests" {
				return filepath.SkipDir
			}
			return nil
		}

		// Only process .go files
		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		// Get relative path
		relPath, err := filepath.Rel(b.WorkspaceRoot, path)
		if err != nil {
			return nil // Skip on error
		}

		// Skip if this is a generated file (has corresponding .dingo)
		if b.generatedFiles[relPath] {
			return nil
		}

		// Check if there's a corresponding .dingo file
		dingoPath := strings.TrimSuffix(path, ".go") + ".dingo"
		if _, err := os.Stat(dingoPath); err == nil {
			// .dingo file exists, this .go is likely generated - skip
			return nil
		}

		// Check if file contains generated marker
		if isGeneratedFile(path) {
			return nil
		}

		// Copy the file to shadow
		dstPath := filepath.Join(b.ShadowDir, relPath)
		if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
			return err
		}

		if err := copyFile(path, dstPath); err != nil {
			return err
		}

		copied = append(copied, relPath)
		return nil
	})

	return copied, err
}

// handleVendor creates a symlink to vendor directory if it exists
func (b *Builder) handleVendor() error {
	vendorSrc := filepath.Join(b.WorkspaceRoot, "vendor")
	vendorDst := filepath.Join(b.ShadowDir, "vendor")

	// Check if vendor exists in source
	if _, err := os.Stat(vendorSrc); os.IsNotExist(err) {
		return nil // No vendor directory, nothing to do
	}

	// Remove existing vendor in shadow (if any)
	os.RemoveAll(vendorDst)

	// Create relative symlink
	relPath, err := filepath.Rel(b.ShadowDir, vendorSrc)
	if err != nil {
		return err
	}

	return os.Symlink(relPath, vendorDst)
}

// Clean removes the shadow directory
func (b *Builder) Clean() error {
	return os.RemoveAll(b.ShadowDir)
}

// isGeneratedFile checks if a .go file contains the dingo generation marker
func isGeneratedFile(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()

	// Read first 1KB to check for marker
	buf := make([]byte, 1024)
	n, err := file.Read(buf)
	if err != nil && err != io.EOF {
		return false
	}

	content := string(buf[:n])
	return strings.Contains(content, "// Code generated by dingo") ||
		strings.Contains(content, "//line ") // Contains line directives
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

// FindWorkspaceRoot finds the workspace root by looking for go.mod
func FindWorkspaceRoot(startPath string) (string, error) {
	absPath, err := filepath.Abs(startPath)
	if err != nil {
		return "", err
	}

	// If startPath is a file, start from its directory
	info, err := os.Stat(absPath)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		absPath = filepath.Dir(absPath)
	}

	current := absPath
	for {
		// Check for go.mod
		if _, err := os.Stat(filepath.Join(current, "go.mod")); err == nil {
			return current, nil
		}

		parent := filepath.Dir(current)
		if parent == current {
			return "", fmt.Errorf("no go.mod found")
		}
		current = parent
	}
}
