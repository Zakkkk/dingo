package transpiler

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/MadAppGang/dingo/pkg/config"
)

// DefaultOutputDir is the default output directory for generated files
// when OutDir is not configured in dingo.toml
const DefaultOutputDir = "build"

// OutputPaths contains all calculated output paths for a transpiled file
type OutputPaths struct {
	GoPath    string // Path to generated .go file (in build/ or OutDir)
	DmapPath  string // Path to generated .dmap file (always in .dmap/)
	OutputDir string // The output directory for .go files
	DmapDir   string // The .dmap directory
	RelPath   string // Relative path from workspace root (without extension)
}

// CalculateOutputPaths computes all output paths for a given Dingo source file.
// It respects the OutDir config setting, defaulting to "build/" if not set.
// Dmap files always go in .dmap/ folder (separate from Go output).
//
//	Example: /project/src/main.dingo -> OutputPaths{
//	  GoPath:   /project/build/src/main.go
//	  DmapPath: /project/.dmap/src/main.dmap
//	}
func CalculateOutputPaths(dingoPath string, cfg *config.Config) (*OutputPaths, error) {
	absPath, err := filepath.Abs(dingoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	inputDir := filepath.Dir(absPath)
	workspaceRoot, err := detectWorkspaceRoot(inputDir)
	if err != nil {
		// Fall back to the input directory if no workspace markers found
		workspaceRoot = inputDir
	}

	// Calculate relative path from workspace root
	relPath, err := filepath.Rel(workspaceRoot, absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate relative path: %w", err)
	}

	// Remove .dingo extension
	basePath := strings.TrimSuffix(relPath, ".dingo")

	// Determine output directory for .go files
	// Empty string means "in-place" (alongside source files)
	// Non-empty value is a directory name (e.g., "build" or ".dingo")
	outDir := DefaultOutputDir
	if cfg != nil {
		// If config explicitly sets outdir (even to empty string), use it
		// Empty string = in-place output
		outDir = cfg.Build.OutDir
	}

	// Build .go output path
	var goOutputBase string
	if outDir == "" {
		// In-place: .go file goes next to .dingo file
		goOutputBase = filepath.Join(workspaceRoot, basePath)
	} else {
		// Out-of-place: .go file goes in outDir (e.g., build/)
		goOutputBase = filepath.Join(workspaceRoot, outDir, basePath)
	}
	goOutputDir := filepath.Dir(goOutputBase)

	// Build .dmap output path (always in .dmap/ folder)
	dmapOutputBase := filepath.Join(workspaceRoot, ".dmap", basePath)
	dmapOutputDir := filepath.Dir(dmapOutputBase)

	// Ensure output directories exist
	if err := os.MkdirAll(goOutputDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}
	if err := os.MkdirAll(dmapOutputDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create .dmap directory: %w", err)
	}

	return &OutputPaths{
		GoPath:    goOutputBase + ".go",
		DmapPath:  dmapOutputBase + ".dmap",
		OutputDir: goOutputDir,
		DmapDir:   dmapOutputDir,
		RelPath:   basePath,
	}, nil
}

// CalculateGoPath is a convenience function that returns just the Go output path.
// This is useful for quick lookups without needing the full OutputPaths struct.
func CalculateGoPath(dingoPath string, cfg *config.Config) (string, error) {
	paths, err := CalculateOutputPaths(dingoPath, cfg)
	if err != nil {
		return "", err
	}
	return paths.GoPath, nil
}

// CalculateDmapPath is a convenience function that returns just the dmap output path.
func CalculateDmapPath(dingoPath string, cfg *config.Config) (string, error) {
	paths, err := CalculateOutputPaths(dingoPath, cfg)
	if err != nil {
		return "", err
	}
	return paths.DmapPath, nil
}

// GoPathToDingoPath converts a Go path back to its source Dingo path.
// This is the inverse of CalculateGoPath.
//
// Example: /project/build/src/main.go -> /project/src/main.dingo
func GoPathToDingoPath(goPath string, cfg *config.Config) (string, error) {
	absPath, err := filepath.Abs(goPath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Determine output directory name
	outDir := DefaultOutputDir
	if cfg != nil && cfg.Build.OutDir != "" {
		outDir = cfg.Build.OutDir
	}

	// For shadow builds, find the project root (with dingo.toml) first
	// This is more reliable than detectWorkspaceRoot which might find
	// the shadow build's go.mod instead of the actual project root
	projectRoot, err := findDingoProjectRoot(filepath.Dir(absPath))
	if err == nil {
		// Found dingo.toml - this is definitely a Dingo project
		buildDir := filepath.Join(projectRoot, outDir)

		// Check if the Go path is under the build directory
		relFromBuild, err := filepath.Rel(buildDir, absPath)
		if err == nil && !strings.HasPrefix(relFromBuild, "..") {
			// Go file is in build directory - convert to dingo path
			basePath := strings.TrimSuffix(relFromBuild, ".go")
			dingoPath := filepath.Join(projectRoot, basePath+".dingo")
			return dingoPath, nil
		}
	}

	// Fallback: use standard workspace root detection
	dir := filepath.Dir(absPath)
	workspaceRoot, err := detectWorkspaceRoot(dir)
	if err != nil {
		// Try parent directories in case we're deep in build/
		for {
			parent := filepath.Dir(dir)
			if parent == dir {
				return "", fmt.Errorf("could not find workspace root")
			}
			dir = parent
			workspaceRoot, err = detectWorkspaceRoot(dir)
			if err == nil {
				break
			}
		}
	}

	buildDir := filepath.Join(workspaceRoot, outDir)

	// Check if the Go path is under the build directory
	relFromBuild, err := filepath.Rel(buildDir, absPath)
	if err != nil || strings.HasPrefix(relFromBuild, "..") {
		// Go file is not in build directory - might be legacy layout
		// Fall back to simple suffix replacement
		if strings.HasSuffix(goPath, ".go") {
			dingoPath := strings.TrimSuffix(goPath, ".go") + ".dingo"
			if _, err := os.Stat(dingoPath); err == nil {
				return dingoPath, nil
			}
		}
		return "", fmt.Errorf("go file is not in expected build directory: %s", goPath)
	}

	// Remove .go extension and add .dingo
	basePath := strings.TrimSuffix(relFromBuild, ".go")
	dingoPath := filepath.Join(workspaceRoot, basePath+".dingo")

	return dingoPath, nil
}

// findDingoProjectRoot walks up the directory tree looking specifically for dingo.toml.
// This is used for shadow builds where the build/ directory has go.mod but we need
// to find the actual project root with dingo.toml.
func findDingoProjectRoot(startPath string) (string, error) {
	current := startPath
	// Safety limit: max 100 directory levels to prevent infinite loops
	// in case of symlink cycles or other filesystem anomalies
	const maxDepth = 100
	for i := 0; i < maxDepth; i++ {
		if _, err := os.Stat(filepath.Join(current, "dingo.toml")); err == nil {
			return current, nil
		}

		parent := filepath.Dir(current)
		if parent == current {
			return "", fmt.Errorf("no dingo.toml found")
		}
		current = parent
	}
	return "", fmt.Errorf("no dingo.toml found (exceeded max depth of %d directories)", maxDepth)
}

// GoPathToDmapPath converts a Go path to its corresponding dmap path.
// Both live in the same output directory.
func GoPathToDmapPath(goPath string) string {
	return strings.TrimSuffix(goPath, ".go") + ".dmap"
}

// detectWorkspaceRoot finds the workspace root by looking for `dingo.toml`, `go.work`, or `go.mod`.
func detectWorkspaceRoot(startPath string) (string, error) {
	current := startPath
	for {
		if _, err := os.Stat(filepath.Join(current, "dingo.toml")); err == nil {
			return current, nil
		}
		if _, err := os.Stat(filepath.Join(current, "go.work")); err == nil {
			return current, nil
		}
		if _, err := os.Stat(filepath.Join(current, "go.mod")); err == nil {
			return current, nil
		}

		parent := filepath.Dir(current)
		if parent == current {
			return startPath, fmt.Errorf("no workspace root found")
		}
		current = parent
	}
}
