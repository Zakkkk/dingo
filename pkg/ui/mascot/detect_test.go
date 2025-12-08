package mascot

import (
	"os"
	"testing"
)

func TestDetect(t *testing.T) {
	// Note: This test runs against the actual environment
	// Some fields like IsInteractive may vary depending on test execution context
	caps := Detect()

	// In test environment (non-TTY), Width/Height may be 0 or fallback values
	// Just verify the function runs without panic
	// The actual values depend on whether tests are run in a TTY or not

	// If not interactive, width/height will be 0
	// If interactive, they should be > 0
	if caps.IsInteractive {
		if caps.Width <= 0 {
			t.Errorf("Interactive terminal should have Width > 0, got %d", caps.Width)
		}
		if caps.Height <= 0 {
			t.Errorf("Interactive terminal should have Height > 0, got %d", caps.Height)
		}
	}

	// IsCI should be deterministic based on environment
	// In test environment, it's typically false unless CI=1 is set
	// We'll test this separately with environment mocking
}

func TestDetectWithCIEnv(t *testing.T) {
	tests := []struct {
		name     string
		envVar   string
		envValue string
		wantCI   bool
	}{
		{
			name:     "CI env var set",
			envVar:   "CI",
			envValue: "1",
			wantCI:   true,
		},
		{
			name:     "GITHUB_ACTIONS set",
			envVar:   "GITHUB_ACTIONS",
			envValue: "true",
			wantCI:   true,
		},
		{
			name:     "GITLAB_CI set",
			envVar:   "GITLAB_CI",
			envValue: "yes",
			wantCI:   true,
		},
		{
			name:     "No CI env var",
			envVar:   "",
			envValue: "",
			wantCI:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear all CI env vars first
			clearCIEnvVars(t)

			// Set the test env var
			if tt.envVar != "" {
				os.Setenv(tt.envVar, tt.envValue)
				defer os.Unsetenv(tt.envVar)
			}

			caps := Detect()

			if caps.IsCI != tt.wantCI {
				t.Errorf("Expected IsCI=%v, got %v", tt.wantCI, caps.IsCI)
			}
		})
	}
}

func TestDetectNoColor(t *testing.T) {
	tests := []struct {
		name            string
		noColorValue    string
		wantColorSupport bool
	}{
		{
			name:            "NO_COLOR set",
			noColorValue:    "1",
			wantColorSupport: false,
		},
		{
			name:            "NO_COLOR empty",
			noColorValue:    "",
			wantColorSupport: true, // Depends on IsInteractive and IsCI
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear CI vars to avoid interference
			clearCIEnvVars(t)

			if tt.noColorValue != "" {
				os.Setenv("NO_COLOR", tt.noColorValue)
				defer os.Unsetenv("NO_COLOR")
			} else {
				os.Unsetenv("NO_COLOR")
			}

			caps := Detect()

			// Color support depends on: !IsCI && NO_COLOR=="" && IsInteractive
			// In test environment, IsInteractive may be false, so we check the NO_COLOR logic
			if tt.noColorValue != "" && caps.SupportsColor {
				t.Errorf("Expected SupportsColor=false when NO_COLOR is set, got true")
			}
		})
	}
}

func TestDetectNoMascot(t *testing.T) {
	tests := []struct {
		name      string
		envValue  string
		wantNoMascot bool
	}{
		{
			name:      "DINGO_NO_MASCOT set",
			envValue:  "1",
			wantNoMascot: true,
		},
		{
			name:      "DINGO_NO_MASCOT empty",
			envValue:  "",
			wantNoMascot: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv("DINGO_NO_MASCOT", tt.envValue)
				defer os.Unsetenv("DINGO_NO_MASCOT")
			} else {
				os.Unsetenv("DINGO_NO_MASCOT")
			}

			caps := Detect()

			if caps.NoMascot != tt.wantNoMascot {
				t.Errorf("Expected NoMascot=%v, got %v", tt.wantNoMascot, caps.NoMascot)
			}
		})
	}
}

func TestShouldShowMascot(t *testing.T) {
	tests := []struct {
		name         string
		caps         Capabilities
		flagNoMascot bool
		want         bool
	}{
		{
			name: "All conditions met",
			caps: Capabilities{
				IsInteractive: true,
				Width:         100,
				Height:        30,
				SupportsColor: true,
				IsCI:          false,
				NoMascot:      false,
			},
			flagNoMascot: false,
			want:         true,
		},
		{
			name: "Flag --no-mascot set",
			caps: Capabilities{
				IsInteractive: true,
				Width:         100,
				Height:        30,
				SupportsColor: true,
				IsCI:          false,
				NoMascot:      false,
			},
			flagNoMascot: true,
			want:         false,
		},
		{
			name: "Env DINGO_NO_MASCOT set",
			caps: Capabilities{
				IsInteractive: true,
				Width:         100,
				Height:        30,
				SupportsColor: true,
				IsCI:          false,
				NoMascot:      true,
			},
			flagNoMascot: false,
			want:         false,
		},
		{
			name: "CI environment",
			caps: Capabilities{
				IsInteractive: true,
				Width:         100,
				Height:        30,
				SupportsColor: false,
				IsCI:          true,
				NoMascot:      false,
			},
			flagNoMascot: false,
			want:         false,
		},
		{
			name: "Non-interactive (piped output)",
			caps: Capabilities{
				IsInteractive: false,
				Width:         100,
				Height:        30,
				SupportsColor: false,
				IsCI:          false,
				NoMascot:      false,
			},
			flagNoMascot: false,
			want:         false,
		},
		{
			name: "Terminal too narrow",
			caps: Capabilities{
				IsInteractive: true,
				Width:         70,
				Height:        30,
				SupportsColor: true,
				IsCI:          false,
				NoMascot:      false,
			},
			flagNoMascot: false,
			want:         false,
		},
		{
			name: "Width exactly at minimum",
			caps: Capabilities{
				IsInteractive: true,
				Width:         MinTerminalWidth,
				Height:        30,
				SupportsColor: true,
				IsCI:          false,
				NoMascot:      false,
			},
			flagNoMascot: false,
			want:         true,
		},
		{
			name: "Width just below minimum",
			caps: Capabilities{
				IsInteractive: true,
				Width:         MinTerminalWidth - 1,
				Height:        30,
				SupportsColor: true,
				IsCI:          false,
				NoMascot:      false,
			},
			flagNoMascot: false,
			want:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ShouldShowMascot(tt.caps, tt.flagNoMascot)
			if got != tt.want {
				t.Errorf("ShouldShowMascot() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMinTerminalWidth(t *testing.T) {
	// Verify the constant is set to the expected value
	expectedMin := 80
	if MinTerminalWidth != expectedMin {
		t.Errorf("MinTerminalWidth = %d, want %d", MinTerminalWidth, expectedMin)
	}
}

// Helper function to clear all CI environment variables
func clearCIEnvVars(t *testing.T) {
	t.Helper()
	ciVars := []string{"CI", "GITHUB_ACTIONS", "GITLAB_CI"}
	for _, v := range ciVars {
		os.Unsetenv(v)
	}
}
