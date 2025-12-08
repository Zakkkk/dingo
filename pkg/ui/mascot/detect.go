package mascot

import (
	"os"

	"golang.org/x/term"
)

// MinTerminalWidth is the minimum width required for side-by-side mode.
// Below this, the mascot is disabled and only output is shown.
const MinTerminalWidth = 80

// Capabilities represents terminal capabilities and environment settings.
type Capabilities struct {
	IsInteractive bool // True if stdout is a TTY
	Width         int  // Terminal width in columns
	Height        int  // Terminal height in rows
	SupportsColor bool // True if ANSI colors are supported
	IsCI          bool // True if running in a CI environment
	NoMascot      bool // True if mascot is disabled via env var
}

// Detect detects terminal capabilities and environment settings.
// It checks:
// - TTY status using golang.org/x/term
// - Terminal size (width and height)
// - CI environment detection (CI, GITHUB_ACTIONS, GITLAB_CI)
// - Color support (NO_COLOR env var)
// - Mascot disable flag (DINGO_NO_MASCOT env var)
func Detect() Capabilities {
	caps := Capabilities{}

	// Check if stdout is a terminal (TTY)
	fd := int(os.Stdout.Fd())
	caps.IsInteractive = term.IsTerminal(fd)

	// If interactive, get terminal dimensions
	if caps.IsInteractive {
		w, h, err := term.GetSize(fd)
		if err == nil {
			caps.Width = w
			caps.Height = h
		} else {
			// Fallback to conservative defaults
			caps.Width = 80
			caps.Height = 24
		}
	}

	// Check for CI environments
	// Common CI environment variables
	caps.IsCI = os.Getenv("CI") != "" ||
		os.Getenv("GITHUB_ACTIONS") != "" ||
		os.Getenv("GITLAB_CI") != ""

	// Check color support
	// Colors are supported if:
	// - Not in CI
	// - NO_COLOR is not set
	// - Terminal is interactive
	caps.SupportsColor = !caps.IsCI &&
		os.Getenv("NO_COLOR") == "" &&
		caps.IsInteractive

	// Check if mascot is explicitly disabled via env var
	caps.NoMascot = os.Getenv("DINGO_NO_MASCOT") != ""

	return caps
}

// ShouldShowMascot returns true if the mascot should be displayed.
// It considers:
// - flagNoMascot: the --no-mascot CLI flag
// - caps.NoMascot: the DINGO_NO_MASCOT env var
// - caps.IsCI: whether running in CI
// - caps.IsInteractive: whether stdout is a TTY
// - caps.Width: whether terminal is wide enough (>= MinTerminalWidth)
func ShouldShowMascot(caps Capabilities, flagNoMascot bool) bool {
	// Explicit disable via flag or env var
	if flagNoMascot || caps.NoMascot {
		return false
	}

	// Disable in CI environments
	if caps.IsCI {
		return false
	}

	// Disable if not interactive (piped/redirected output)
	if !caps.IsInteractive {
		return false
	}

	// Disable if terminal is too narrow
	if caps.Width < MinTerminalWidth {
		return false
	}

	return true
}
