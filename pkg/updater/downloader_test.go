package updater

import (
	"runtime"
	"testing"
)

func TestGetBinaryAssetName(t *testing.T) {
	// Test current platform
	expected := "dingo-" + runtime.GOOS + "-" + runtime.GOARCH
	if runtime.GOOS == "windows" {
		expected += ".exe"
	}

	got := GetBinaryAssetName("dingo")
	if got != expected {
		t.Errorf("GetBinaryAssetName(dingo) = %v, want %v", got, expected)
	}

	// Test LSP binary
	expectedLSP := "dingo-lsp-" + runtime.GOOS + "-" + runtime.GOARCH
	if runtime.GOOS == "windows" {
		expectedLSP += ".exe"
	}

	gotLSP := GetBinaryAssetName("dingo-lsp")
	if gotLSP != expectedLSP {
		t.Errorf("GetBinaryAssetName(dingo-lsp) = %v, want %v", gotLSP, expectedLSP)
	}
}

func TestFindAsset(t *testing.T) {
	release := &ReleaseInfo{
		Assets: []Asset{
			{Name: "dingo-darwin-arm64", DownloadURL: "https://example.com/darwin-arm64", Size: 1000},
			{Name: "dingo-linux-amd64", DownloadURL: "https://example.com/linux-amd64", Size: 2000},
			{Name: "dingo-windows-amd64.exe", DownloadURL: "https://example.com/windows-amd64", Size: 3000},
		},
	}

	tests := []struct {
		name      string
		assetName string
		wantNil   bool
		wantSize  int64
	}{
		{"find darwin-arm64", "dingo-darwin-arm64", false, 1000},
		{"find linux-amd64", "dingo-linux-amd64", false, 2000},
		{"find windows", "dingo-windows-amd64.exe", false, 3000},
		{"not found", "dingo-freebsd-amd64", true, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FindAsset(release, tt.assetName)
			if tt.wantNil && got != nil {
				t.Errorf("FindAsset() = %v, want nil", got)
			}
			if !tt.wantNil && got == nil {
				t.Errorf("FindAsset() = nil, want non-nil")
			}
			if !tt.wantNil && got != nil && got.Size != tt.wantSize {
				t.Errorf("FindAsset().Size = %v, want %v", got.Size, tt.wantSize)
			}
		})
	}
}

func TestSupportedPlatforms(t *testing.T) {
	platforms := SupportedPlatforms()

	// Check that we have the expected platforms
	expected := []string{
		"darwin-amd64",
		"darwin-arm64",
		"linux-amd64",
		"linux-arm64",
		"windows-amd64",
	}

	if len(platforms) != len(expected) {
		t.Errorf("SupportedPlatforms() returned %d platforms, want %d", len(platforms), len(expected))
	}

	for _, p := range expected {
		found := false
		for _, got := range platforms {
			if got == p {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("SupportedPlatforms() missing %q", p)
		}
	}
}

func TestIsPlatformSupported(t *testing.T) {
	// Current platform should be supported (assuming we're on a supported platform)
	current := runtime.GOOS + "-" + runtime.GOARCH
	supported := IsPlatformSupported()

	// Check if current is in supported list
	platforms := SupportedPlatforms()
	isInList := false
	for _, p := range platforms {
		if p == current {
			isInList = true
			break
		}
	}

	if isInList != supported {
		t.Errorf("IsPlatformSupported() = %v, but platform %s is in list: %v", supported, current, isInList)
	}
}

func TestGetPlatformInfo(t *testing.T) {
	os, arch := GetPlatformInfo()

	if os != runtime.GOOS {
		t.Errorf("GetPlatformInfo() os = %v, want %v", os, runtime.GOOS)
	}
	if arch != runtime.GOARCH {
		t.Errorf("GetPlatformInfo() arch = %v, want %v", arch, runtime.GOARCH)
	}
}
