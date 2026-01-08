package updater

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsVersionLike(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"0.11.5", true},
		{"1.0.0", true},
		{"1.0", true},
		{"v0.11.5", true},
		{"0.11.5.1", true},    // 4 parts allowed
		{"0.11.5.1.2", false}, // 5 parts not allowed
		{"hello", false},
		{"1", false},    // Only 1 part
		{"a.b.c", true}, // Has 3 parts (even though not numeric)
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := isVersionLike(tt.input); got != tt.want {
				t.Errorf("isVersionLike(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestCopyFile(t *testing.T) {
	// Create temp directory
	tempDir, err := os.MkdirTemp("", "updater-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create source file
	srcPath := filepath.Join(tempDir, "source.txt")
	content := []byte("test content for copy")
	if err := os.WriteFile(srcPath, content, 0644); err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	// Copy file
	dstPath := filepath.Join(tempDir, "destination.txt")
	if err := copyFile(srcPath, dstPath); err != nil {
		t.Fatalf("copyFile() error = %v", err)
	}

	// Verify destination file
	gotContent, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("Failed to read destination: %v", err)
	}

	if string(gotContent) != string(content) {
		t.Errorf("copyFile() copied content = %q, want %q", string(gotContent), string(content))
	}

	// Verify permissions are preserved
	srcInfo, _ := os.Stat(srcPath)
	dstInfo, _ := os.Stat(dstPath)
	if srcInfo.Mode() != dstInfo.Mode() {
		t.Errorf("copyFile() mode = %v, want %v", dstInfo.Mode(), srcInfo.Mode())
	}
}

func TestCopyFile_NotFound(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "updater-test-*")
	defer os.RemoveAll(tempDir)

	err := copyFile(filepath.Join(tempDir, "nonexistent"), filepath.Join(tempDir, "dst"))
	if err == nil {
		t.Error("copyFile() should fail for nonexistent source")
	}
}

func TestRequiresSudo(t *testing.T) {
	// Create a temp file we definitely have access to
	tempDir, _ := os.MkdirTemp("", "updater-test-*")
	defer os.RemoveAll(tempDir)

	tempFile := filepath.Join(tempDir, "test-binary")
	if err := os.WriteFile(tempFile, []byte("test"), 0755); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	// Should not require sudo for file we own
	if RequiresSudo(tempFile) {
		t.Error("RequiresSudo() should return false for file we own")
	}

	// Test with system path (likely requires sudo unless running as root)
	// We just check it doesn't panic
	_ = RequiresSudo("/usr/local/bin/dingo")
}

func TestRollbackInstallation(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "updater-test-*")
	defer os.RemoveAll(tempDir)

	// Create "backup" file
	backupPath := filepath.Join(tempDir, "backup")
	backupContent := []byte("original content")
	if err := os.WriteFile(backupPath, backupContent, 0755); err != nil {
		t.Fatalf("Failed to create backup: %v", err)
	}

	// Create "target" file with different content
	targetPath := filepath.Join(tempDir, "target")
	if err := os.WriteFile(targetPath, []byte("new content"), 0755); err != nil {
		t.Fatalf("Failed to create target: %v", err)
	}

	// Rollback
	if err := RollbackInstallation(backupPath, targetPath); err != nil {
		t.Fatalf("RollbackInstallation() error = %v", err)
	}

	// Verify target has backup content
	gotContent, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("Failed to read target: %v", err)
	}

	if string(gotContent) != string(backupContent) {
		t.Errorf("RollbackInstallation() content = %q, want %q", string(gotContent), string(backupContent))
	}
}

func TestRollbackInstallation_MissingBackup(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "updater-test-*")
	defer os.RemoveAll(tempDir)

	err := RollbackInstallation(filepath.Join(tempDir, "nonexistent"), filepath.Join(tempDir, "target"))
	if err == nil {
		t.Error("RollbackInstallation() should fail for nonexistent backup")
	}
}

func TestInstallBinary_DryRun(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "updater-test-*")
	defer os.RemoveAll(tempDir)

	// Create "current" binary
	currentPath := filepath.Join(tempDir, "dingo")
	if err := os.WriteFile(currentPath, []byte("old binary"), 0755); err != nil {
		t.Fatalf("Failed to create current binary: %v", err)
	}

	// Create "new" binary
	newPath := filepath.Join(tempDir, "new-dingo")
	if err := os.WriteFile(newPath, []byte("new binary"), 0755); err != nil {
		t.Fatalf("Failed to create new binary: %v", err)
	}

	// Dry run installation
	opts := InstallOptions{
		TempBinaryPath: newPath,
		TargetPath:     currentPath,
		DryRun:         true,
	}

	result, err := InstallBinary(opts)
	if err != nil {
		t.Fatalf("InstallBinary() dry run error = %v", err)
	}

	// In dry run, target should be unchanged
	gotContent, _ := os.ReadFile(currentPath)
	if string(gotContent) != "old binary" {
		t.Errorf("InstallBinary() dry run changed target content")
	}

	if result.InstalledPath != currentPath {
		t.Errorf("InstallBinary() InstalledPath = %v, want %v", result.InstalledPath, currentPath)
	}
}
