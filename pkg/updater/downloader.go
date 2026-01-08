package updater

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// GetBinaryAssetName returns the expected asset name for the current platform.
// Format: {binary}-{os}-{arch}
// Examples: dingo-darwin-arm64, dingo-linux-amd64, dingo-windows-amd64.exe
func GetBinaryAssetName(binary string) string {
	os := runtime.GOOS
	arch := runtime.GOARCH

	name := fmt.Sprintf("%s-%s-%s", binary, os, arch)

	// Add .exe extension on Windows
	if os == "windows" {
		name += ".exe"
	}

	return name
}

// FindAsset searches for a matching asset in the release.
// Returns nil if no matching asset is found.
func FindAsset(release *ReleaseInfo, assetName string) *Asset {
	for i := range release.Assets {
		if release.Assets[i].Name == assetName {
			return &release.Assets[i]
		}
	}
	return nil
}

// DownloadBinary downloads a binary from GitHub releases.
// It returns the path to the downloaded file in a temporary directory.
func DownloadBinary(opts DownloadOptions) (*DownloadResult, error) {
	// Fetch release info
	var release *ReleaseInfo
	var err error

	if opts.Version == "" || opts.Version == "latest" {
		release, err = FetchLatestRelease()
	} else {
		release, err = FetchRelease(opts.Version)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to fetch release: %w", err)
	}

	// Determine binary name
	binary := opts.Binary
	if binary == "" {
		binary = "dingo"
	}

	// Find the matching asset
	assetName := GetBinaryAssetName(binary)
	asset := FindAsset(release, assetName)
	if asset == nil {
		// List available assets for better error message
		var available []string
		for _, a := range release.Assets {
			available = append(available, a.Name)
		}
		return nil, fmt.Errorf("no asset found for %s (available: %s)", assetName, strings.Join(available, ", "))
	}

	// Download to temp directory
	tempDir, err := os.MkdirTemp("", "dingo-update-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	tempPath := filepath.Join(tempDir, binary)
	if runtime.GOOS == "windows" {
		tempPath += ".exe"
	}

	// Download the asset
	err = downloadFile(asset.DownloadURL, tempPath, asset.Size, opts.ProgressFunc)
	if err != nil {
		os.RemoveAll(tempDir)
		return nil, fmt.Errorf("failed to download binary: %w", err)
	}

	// Make executable (no-op on Windows)
	if runtime.GOOS != "windows" {
		if err := os.Chmod(tempPath, 0755); err != nil {
			os.RemoveAll(tempDir)
			return nil, fmt.Errorf("failed to make binary executable: %w", err)
		}
	}

	return &DownloadResult{
		TempPath: tempPath,
		Size:     asset.Size,
		Version:  strings.TrimPrefix(release.TagName, "v"),
	}, nil
}

// downloadFile downloads a file from URL to destPath with optional progress reporting.
func downloadFile(url, destPath string, expectedSize int64, progressFunc func(downloaded, total int64)) error {
	// Create request
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "dingo-cli")

	// Send request
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("download request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	// Get total size from response if not provided
	total := expectedSize
	if total == 0 && resp.ContentLength > 0 {
		total = resp.ContentLength
	}

	// Create destination file
	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close()

	// Download with progress tracking
	if progressFunc != nil {
		reader := &progressReader{
			reader:   resp.Body,
			total:    total,
			callback: progressFunc,
		}
		_, err = io.Copy(out, reader)
	} else {
		_, err = io.Copy(out, resp.Body)
	}

	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// progressReader wraps an io.Reader to report download progress.
type progressReader struct {
	reader     io.Reader
	total      int64
	downloaded int64
	callback   func(downloaded, total int64)
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	if n > 0 {
		pr.downloaded += int64(n)
		if pr.callback != nil {
			pr.callback(pr.downloaded, pr.total)
		}
	}
	return n, err
}

// GetPlatformInfo returns the current OS and architecture.
func GetPlatformInfo() (os, arch string) {
	return runtime.GOOS, runtime.GOARCH
}

// SupportedPlatforms returns a list of supported platform combinations.
func SupportedPlatforms() []string {
	return []string{
		"darwin-amd64",
		"darwin-arm64",
		"linux-amd64",
		"linux-arm64",
		"windows-amd64",
	}
}

// IsPlatformSupported checks if the current platform is supported.
func IsPlatformSupported() bool {
	current := fmt.Sprintf("%s-%s", runtime.GOOS, runtime.GOARCH)
	for _, p := range SupportedPlatforms() {
		if p == current {
			return true
		}
	}
	return false
}
