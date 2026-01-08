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

	"github.com/MadAppGang/dingo/pkg/config"
	"github.com/MadAppGang/dingo/pkg/sourcemap/dmap"
	"github.com/MadAppGang/dingo/pkg/transpiler"
)

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

	// generatedFiles tracks files we've transpiled (to avoid copying them as pure Go)
	generatedFiles map[string]bool
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
}

// Build creates the shadow directory with all necessary files
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

	// 3. Transpile .dingo files to shadow
	for _, dingoFile := range dingoFiles {
		goFile, err := b.transpileFile(dingoFile)
		if err != nil {
			return nil, fmt.Errorf("failed to transpile %s: %w", dingoFile, err)
		}
		result.GeneratedFiles = append(result.GeneratedFiles, goFile)

		// Track generated file to avoid copying it as pure Go
		relPath, _ := filepath.Rel(b.WorkspaceRoot, dingoFile)
		goRelPath := strings.TrimSuffix(relPath, ".dingo") + ".go"
		b.generatedFiles[goRelPath] = true
	}

	// 4. Copy pure .go files to shadow
	copiedGo, err := b.copyPureGoFiles()
	if err != nil {
		return nil, fmt.Errorf("failed to copy Go files: %w", err)
	}
	result.CopiedFiles = append(result.CopiedFiles, copiedGo...)

	// 5. Handle vendor directory if present
	if err := b.handleVendor(); err != nil {
		return nil, fmt.Errorf("failed to handle vendor directory: %w", err)
	}

	return result, nil
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

	// Transpile
	result, err := transpiler.PureASTTranspileWithMappings(src, absDingoFile, true)
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
			// Skip hidden dirs, shadow dir, vendor, node_modules
			if strings.HasPrefix(name, ".") ||
				path == b.ShadowDir ||
				name == "vendor" ||
				name == "node_modules" ||
				name == "testdata" {
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
