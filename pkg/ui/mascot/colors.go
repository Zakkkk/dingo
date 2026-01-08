// Package mascot provides an animated ASCII art dingo mascot for the CLI.
package mascot

import "github.com/charmbracelet/lipgloss"

// ColorScheme defines the color palette for the mascot.
type ColorScheme struct {
	Body   lipgloss.Color // Main body color
	Eyes   lipgloss.Color // Eye color
	Nose   lipgloss.Color // Nose color
	Badge  lipgloss.Color // Badge/accent color (for success indicators)
	Accent lipgloss.Color // Secondary accent color (for warnings/highlights)
}

// DefaultColorScheme uses Dingo brand colors for the default mascot appearance.
// Aligned with brand colors from pkg/ui/styles.go.
var DefaultColorScheme = ColorScheme{
	Body:   lipgloss.Color("#7D56F4"), // Dingo purple (brand primary)
	Eyes:   lipgloss.Color("#56C3F4"), // Cyan (brand secondary)
	Nose:   lipgloss.Color("#FF6B9D"), // Pink (brand error/accent)
	Badge:  lipgloss.Color("#5AF78E"), // Green (success)
	Accent: lipgloss.Color("#F7DC6F"), // Yellow (warning/attention)
}

// SuccessColorScheme applies a green glow for successful operations.
var SuccessColorScheme = ColorScheme{
	Body:   lipgloss.Color("#5AF78E"), // Green glow
	Eyes:   lipgloss.Color("#56C3F4"), // Cyan (kept from default)
	Nose:   lipgloss.Color("#FF6B9D"), // Pink (kept from default)
	Badge:  lipgloss.Color("#5AF78E"), // Green
	Accent: lipgloss.Color("#F7DC6F"), // Yellow
}

// FailureColorScheme applies red/pink tones for failed operations.
var FailureColorScheme = ColorScheme{
	Body:   lipgloss.Color("#FF6B9D"), // Red/pink
	Eyes:   lipgloss.Color("#FF3333"), // Bright red eyes
	Nose:   lipgloss.Color("#FF6B9D"), // Pink
	Badge:  lipgloss.Color("#FF6B9D"), // Red
	Accent: lipgloss.Color("#F7DC6F"), // Yellow (kept for contrast)
}

// CompileColorScheme applies vibrant colors for compilation state.
// Uses the default purple theme with slightly enhanced brightness.
var CompileColorScheme = ColorScheme{
	Body:   lipgloss.Color("#7D56F4"), // Dingo purple
	Eyes:   lipgloss.Color("#56C3F4"), // Cyan
	Nose:   lipgloss.Color("#FF6B9D"), // Pink
	Badge:  lipgloss.Color("#7D56F4"), // Purple
	Accent: lipgloss.Color("#F7DC6F"), // Yellow
}

// Helper functions to apply colors to strings

// ApplyColor applies a color to a string using lipgloss.
func ApplyColor(s string, color lipgloss.Color) string {
	return lipgloss.NewStyle().Foreground(color).Render(s)
}

// ApplyBodyColor applies the body color from a scheme to a string.
func (cs ColorScheme) ApplyBodyColor(s string) string {
	return ApplyColor(s, cs.Body)
}

// ApplyEyeColor applies the eye color from a scheme to a string.
func (cs ColorScheme) ApplyEyeColor(s string) string {
	return ApplyColor(s, cs.Eyes)
}

// ApplyNoseColor applies the nose color from a scheme to a string.
func (cs ColorScheme) ApplyNoseColor(s string) string {
	return ApplyColor(s, cs.Nose)
}

// ApplyBadgeColor applies the badge color from a scheme to a string.
func (cs ColorScheme) ApplyBadgeColor(s string) string {
	return ApplyColor(s, cs.Badge)
}

// ApplyAccentColor applies the accent color from a scheme to a string.
func (cs ColorScheme) ApplyAccentColor(s string) string {
	return ApplyColor(s, cs.Accent)
}

// Colorize applies colors from a scheme to different parts of the mascot.
// This is a convenience function for applying the color scheme to mascot parts.
//
// Usage example:
//
//	colored := scheme.Colorize(bodyPart, eyePart, nosePart)
func (cs ColorScheme) Colorize(body, eyes, nose string) (coloredBody, coloredEyes, coloredNose string) {
	return cs.ApplyBodyColor(body), cs.ApplyEyeColor(eyes), cs.ApplyNoseColor(nose)
}
