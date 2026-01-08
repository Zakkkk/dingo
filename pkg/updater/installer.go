package updater

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// InstallBinary replaces the current binary with a new version.
// It creates a backup before replacement and supports rollback on failure.
func InstallBinary(opts InstallOptions) (*InstallResult, error) {
	result := &InstallResult{}

	// Determine target path
	targetPath := opts.TargetPath
	if targetPath == "" {
		var err error
		targetPath, err = os.Executable()
		if err != nil {
			return nil, fmt.Errorf("failed to get current executable path: %w", err)
		}
		// Resolve symlinks
		targetPath, err = filepath.EvalSymlinks(targetPath)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve symlinks: %w", err)
		}
	}
	result.InstalledPath = targetPath

	// Check if temp binary exists
	if _, err := os.Stat(opts.TempBinaryPath); err != nil {
		return nil, fmt.Errorf("temp binary not found: %w", err)
	}

	// Determine backup directory
	backupDir := opts.BackupDir
	if backupDir == "" {
		backupDir = os.TempDir()
	}

	// Create backup with timestamp
	timestamp := time.Now().Format("20060102-150405")
	backupName := fmt.Sprintf("%s.backup.%s", filepath.Base(targetPath), timestamp)
	backupPath := filepath.Join(backupDir, backupName)
	result.BackupPath = backupPath

	// Dry run: just report what would happen
	if opts.DryRun {
		return result, nil
	}

	// Create backup
	if err := copyFile(targetPath, backupPath); err != nil {
		return nil, fmt.Errorf("failed to create backup: %w", err)
	}

	// Try to replace the binary
	err := replaceBinary(targetPath, opts.TempBinaryPath)
	if err != nil {
		// Attempt rollback
		rollbackErr := RollbackInstallation(backupPath, targetPath)
		if rollbackErr != nil {
			return nil, fmt.Errorf("installation failed and rollback failed: %w (rollback: %v)", err, rollbackErr)
		}
		return nil, fmt.Errorf("installation failed (rolled back): %w", err)
	}

	// Verify the new binary works
	if err := verifyBinary(targetPath); err != nil {
		// Rollback on verification failure
		rollbackErr := RollbackInstallation(backupPath, targetPath)
		if rollbackErr != nil {
			return nil, fmt.Errorf("verification failed and rollback failed: %w (rollback: %v)", err, rollbackErr)
		}
		return nil, fmt.Errorf("verification failed (rolled back): %w", err)
	}

	// Extract new version
	newVersion, _ := extractVersion(targetPath)
	result.NewVersion = newVersion

	// Clean up temp file
	os.Remove(opts.TempBinaryPath)
	os.Remove(filepath.Dir(opts.TempBinaryPath)) // Remove temp directory

	return result, nil
}

// RollbackInstallation restores a backup after a failed installation.
func RollbackInstallation(backupPath, targetPath string) error {
	// Verify backup exists
	if _, err := os.Stat(backupPath); err != nil {
		return fmt.Errorf("backup not found: %w", err)
	}

	return copyFile(backupPath, targetPath)
}

// replaceBinary replaces the target binary with the source.
// On Unix, we try rename first, then fall back to copy+delete.
// On Windows, we need to rename the old binary first since it may be in use.
func replaceBinary(targetPath, sourcePath string) error {
	if runtime.GOOS == "windows" {
		return replaceWindowsBinary(targetPath, sourcePath)
	}
	return replaceUnixBinary(targetPath, sourcePath)
}

// replaceUnixBinary replaces a binary on Unix systems.
func replaceUnixBinary(targetPath, sourcePath string) error {
	// Get the original permissions
	info, err := os.Stat(targetPath)
	if err != nil {
		return fmt.Errorf("failed to stat target: %w", err)
	}
	mode := info.Mode()

	// Try atomic rename first (works if same filesystem)
	err = os.Rename(sourcePath, targetPath)
	if err == nil {
		// Restore permissions
		os.Chmod(targetPath, mode)
		return nil
	}

	// Rename failed (cross-filesystem), fall back to copy
	if err := copyFile(sourcePath, targetPath); err != nil {
		return err
	}

	// Restore permissions
	if err := os.Chmod(targetPath, mode); err != nil {
		return fmt.Errorf("failed to set permissions: %w", err)
	}

	// Remove source
	os.Remove(sourcePath)
	return nil
}

// replaceWindowsBinary handles binary replacement on Windows.
// Windows doesn't allow overwriting a running executable, so we
// rename the old one first.
func replaceWindowsBinary(targetPath, sourcePath string) error {
	oldPath := targetPath + ".old"

	// Remove any previous .old file
	os.Remove(oldPath)

	// Rename current to .old
	if err := os.Rename(targetPath, oldPath); err != nil {
		return fmt.Errorf("failed to rename old binary: %w", err)
	}

	// Move new binary into place
	if err := os.Rename(sourcePath, targetPath); err != nil {
		// Try to restore old binary
		os.Rename(oldPath, targetPath)
		return fmt.Errorf("failed to install new binary: %w", err)
	}

	// Schedule old binary for deletion (will be deleted on next reboot)
	// For now, just try to delete it
	os.Remove(oldPath)

	return nil
}

// copyFile copies a file from src to dst.
func copyFile(src, dst string) error {
	// Open source file
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source: %w", err)
	}
	defer srcFile.Close()

	// Get source info for permissions
	srcInfo, err := srcFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat source: %w", err)
	}

	// Create destination file
	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return fmt.Errorf("failed to create destination: %w", err)
	}
	defer dstFile.Close()

	// Copy contents
	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("failed to copy: %w", err)
	}

	return nil
}

// verifyBinary runs the binary with --version to verify it works.
func verifyBinary(path string) error {
	cmd := exec.Command(path, "version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("binary verification failed: %w (output: %s)", err, string(output))
	}
	return nil
}

// extractVersion runs the binary with version command and extracts the version string.
func extractVersion(path string) (string, error) {
	cmd := exec.Command(path, "version")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	// Parse output to extract version
	// Expected format contains "Version: X.Y.Z" or just "X.Y.Z"
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Look for version pattern
		if strings.Contains(line, "Version") {
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				return strings.TrimSpace(parts[1]), nil
			}
		}
		// Check if line looks like a version (X.Y.Z)
		if isVersionLike(line) {
			return line, nil
		}
	}

	return "", fmt.Errorf("could not extract version from output")
}

// isVersionLike checks if a string looks like a semantic version.
func isVersionLike(s string) bool {
	s = strings.TrimPrefix(s, "v")
	parts := strings.Split(s, ".")
	return len(parts) >= 2 && len(parts) <= 4
}

// FindLSPBinary finds the dingo-lsp binary in PATH.
func FindLSPBinary() (string, error) {
	// First check PATH
	path, err := exec.LookPath("dingo-lsp")
	if err == nil {
		return path, nil
	}

	// Check common locations
	commonPaths := []string{
		"/usr/local/bin/dingo-lsp",
		"/usr/bin/dingo-lsp",
	}

	// Add user's Go bin
	if gopath := os.Getenv("GOPATH"); gopath != "" {
		commonPaths = append(commonPaths, filepath.Join(gopath, "bin", "dingo-lsp"))
	}
	if home, err := os.UserHomeDir(); err == nil {
		commonPaths = append(commonPaths, filepath.Join(home, "go", "bin", "dingo-lsp"))
	}

	for _, p := range commonPaths {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}

	return "", fmt.Errorf("dingo-lsp not found in PATH or common locations")
}

// RequiresSudo checks if the target path requires elevated privileges.
func RequiresSudo(targetPath string) bool {
	// Try to open the file for writing
	f, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		return os.IsPermission(err)
	}
	f.Close()
	return false
}
