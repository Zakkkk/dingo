// Package updater provides version checking and auto-update functionality
// for the Dingo CLI. It handles GitHub API integration, binary downloads,
// and safe binary replacement with rollback support.
package updater

import "time"

// ReleaseInfo represents a GitHub release.
type ReleaseInfo struct {
	TagName     string    `json:"tag_name"`     // e.g., "v0.11.6"
	Name        string    `json:"name"`         // e.g., "Dingo v0.11.6"
	PublishedAt time.Time `json:"published_at"` // When the release was published
	Prerelease  bool      `json:"prerelease"`   // Whether this is a pre-release
	HTMLURL     string    `json:"html_url"`     // URL to the release page
	Assets      []Asset   `json:"assets"`       // Release assets (binaries)
}

// Asset represents a release asset (binary) on GitHub.
type Asset struct {
	Name        string `json:"name"`                 // e.g., "dingo-darwin-arm64"
	DownloadURL string `json:"browser_download_url"` // Direct download URL
	Size        int64  `json:"size"`                 // Size in bytes
	ContentType string `json:"content_type"`         // MIME type
}

// UpdateStatus represents the result of an update check.
type UpdateStatus int

const (
	// UpdateAvailable indicates a newer version is available.
	UpdateAvailable UpdateStatus = iota
	// UpToDate indicates the current version is the latest.
	UpToDate
	// UnknownVersion indicates the current version couldn't be parsed.
	UnknownVersion
	// CheckError indicates an error occurred during the check.
	CheckError
)

// String returns a human-readable string for UpdateStatus.
func (s UpdateStatus) String() string {
	switch s {
	case UpdateAvailable:
		return "update available"
	case UpToDate:
		return "up to date"
	case UnknownVersion:
		return "unknown version"
	case CheckError:
		return "check error"
	default:
		return "unknown"
	}
}

// UpdateCheckResult contains the results of an update check.
type UpdateCheckResult struct {
	Status         UpdateStatus // Check result status
	CurrentVersion string       // Currently installed version
	LatestVersion  string       // Latest available version
	ReleaseURL     string       // URL to the release page
	LatestRelease  *ReleaseInfo // Full release info (nil if check failed)
	Error          error        // Error if check failed
}

// DownloadOptions configures binary download behavior.
type DownloadOptions struct {
	Version      string                        // Target version (or "latest")
	Binary       string                        // "dingo" or "dingo-lsp"
	ProgressFunc func(downloaded, total int64) // Progress callback
}

// DownloadResult contains the result of a binary download.
type DownloadResult struct {
	TempPath string // Path to downloaded binary
	Size     int64  // Size in bytes
	Version  string // Version that was downloaded
}

// InstallOptions configures installation behavior.
type InstallOptions struct {
	TempBinaryPath string // Path to downloaded binary
	TargetPath     string // Path to install (empty = auto-detect)
	BackupDir      string // Directory for backup (empty = temp)
	DryRun         bool   // Simulate installation without changes
}

// InstallResult contains the result of an installation.
type InstallResult struct {
	InstalledPath string // Where binary was installed
	BackupPath    string // Where backup was saved
	PreviousVer   string // Previous version (if detected)
	NewVersion    string // New version installed
}
