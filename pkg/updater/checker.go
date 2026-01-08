package updater

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
)

// GitHubAPIURL is the base URL for GitHub API.
const GitHubAPIURL = "https://api.github.com"

// Repository constants for Dingo.
const (
	RepoOwner = "MadAppGang"
	RepoName  = "dingo"
)

// httpClient is used for all HTTP requests (allows mocking in tests).
var httpClient = &http.Client{
	Timeout: 10 * time.Second,
}

// CheckForUpdates queries GitHub for the latest release and compares it
// with the current version. Returns an UpdateCheckResult with the status.
func CheckForUpdates(currentVersion string) (*UpdateCheckResult, error) {
	result := &UpdateCheckResult{
		CurrentVersion: currentVersion,
	}

	// Fetch latest release
	release, err := FetchLatestRelease()
	if err != nil {
		result.Status = CheckError
		result.Error = err
		return result, err
	}

	result.LatestRelease = release
	result.LatestVersion = strings.TrimPrefix(release.TagName, "v")
	result.ReleaseURL = release.HTMLURL

	// Compare versions
	cmp, err := CompareVersions(currentVersion, result.LatestVersion)
	if err != nil {
		result.Status = UnknownVersion
		result.Error = fmt.Errorf("failed to compare versions: %w", err)
		return result, nil // Not a fatal error
	}

	if cmp < 0 {
		result.Status = UpdateAvailable
	} else {
		result.Status = UpToDate
	}

	return result, nil
}

// FetchLatestRelease fetches the latest release from GitHub.
func FetchLatestRelease() (*ReleaseInfo, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases/latest", GitHubAPIURL, RepoOwner, RepoName)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "dingo-cli")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch release: %w", err)
	}
	defer resp.Body.Close()

	// Handle rate limiting
	if resp.StatusCode == http.StatusForbidden {
		remaining := resp.Header.Get("X-RateLimit-Remaining")
		if remaining == "0" {
			resetTime := resp.Header.Get("X-RateLimit-Reset")
			return nil, fmt.Errorf("GitHub API rate limit exceeded, resets at %s", resetTime)
		}
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API returned %d: %s", resp.StatusCode, string(body))
	}

	var release ReleaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to parse release: %w", err)
	}

	return &release, nil
}

// FetchRelease fetches a specific release by tag.
func FetchRelease(tag string) (*ReleaseInfo, error) {
	// Ensure tag has 'v' prefix
	if !strings.HasPrefix(tag, "v") {
		tag = "v" + tag
	}

	url := fmt.Sprintf("%s/repos/%s/%s/releases/tags/%s", GitHubAPIURL, RepoOwner, RepoName, tag)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "dingo-cli")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("release %s not found", tag)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API returned %d: %s", resp.StatusCode, string(body))
	}

	var release ReleaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to parse release: %w", err)
	}

	return &release, nil
}

// CompareVersions compares two semantic version strings.
// Returns:
//   - -1 if v1 < v2 (v1 is older)
//   - 0 if v1 == v2
//   - 1 if v1 > v2 (v1 is newer)
func CompareVersions(v1, v2 string) (int, error) {
	// Strip 'v' prefix if present
	v1 = strings.TrimPrefix(v1, "v")
	v2 = strings.TrimPrefix(v2, "v")

	ver1, err := semver.NewVersion(v1)
	if err != nil {
		return 0, fmt.Errorf("invalid version %q: %w", v1, err)
	}

	ver2, err := semver.NewVersion(v2)
	if err != nil {
		return 0, fmt.Errorf("invalid version %q: %w", v2, err)
	}

	return ver1.Compare(ver2), nil
}

// IsValidVersion checks if a string is a valid semantic version.
func IsValidVersion(v string) bool {
	v = strings.TrimPrefix(v, "v")
	_, err := semver.NewVersion(v)
	return err == nil
}

// NormalizeVersion normalizes a version string by stripping 'v' prefix
// and returning a clean semantic version string.
func NormalizeVersion(v string) (string, error) {
	v = strings.TrimPrefix(v, "v")
	ver, err := semver.NewVersion(v)
	if err != nil {
		return "", fmt.Errorf("invalid version %q: %w", v, err)
	}
	return ver.String(), nil
}
