package transpiler

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// calculateDmapPath calculates the `.dmap` file path in the workspace root `.dmap/` folder.
// Example: /project/examples/foo.dingo -> /project/.dmap/examples/foo.dmap
func calculateDmapPath(dingoPath string) (string, error) {
	absPath, err := filepath.Abs(dingoPath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	inputDir := filepath.Dir(absPath)
	workspaceRoot, err := detectWorkspaceRoot(inputDir)
	if err != nil {
		// Fall back to the input directory if no workspace markers are found.
		workspaceRoot = inputDir
	}

	relPath, err := filepath.Rel(workspaceRoot, absPath)
	if err != nil {
		return "", fmt.Errorf("failed to calculate relative path: %w", err)
	}

	relDmap := strings.TrimSuffix(relPath, ".dingo") + ".dmap"
	dmapPath := filepath.Join(workspaceRoot, ".dmap", relDmap)

	if err := os.MkdirAll(filepath.Dir(dmapPath), 0o755); err != nil {
		return "", fmt.Errorf("failed to create .dmap directory: %w", err)
	}

	return dmapPath, nil
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

