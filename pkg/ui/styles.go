// Package ui provides beautiful, styled CLI output using lipgloss
package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/MadAppGang/dingo/pkg/ui/mascot"
)

// Color palette - carefully chosen for readability and aesthetics
var (
	// Primary colors
	colorPrimary   = lipgloss.Color("#7D56F4") // Purple (Dingo brand)
	colorSecondary = lipgloss.Color("#56C3F4") // Cyan
	colorSuccess   = lipgloss.Color("#5AF78E") // Green
	colorWarning   = lipgloss.Color("#F7DC6F") // Yellow
	colorError     = lipgloss.Color("#FF6B9D") // Pink/Red
	colorMuted     = lipgloss.Color("#6C7086") // Gray

	// Semantic colors
	colorText      = lipgloss.Color("#CDD6F4") // Light text
	colorSubtle    = lipgloss.Color("#7F849C") // Subtle text
	colorBorder    = lipgloss.Color("#45475A") // Border
	colorHighlight = lipgloss.Color("#F5E0DC") // Highlight
	colorNormal    = lipgloss.Color("#FFFFFF") // Normal white text
)

// Styles
var (
	// Header style - main title
	styleHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary).
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(colorPrimary).
			Padding(0, 2).
			MarginBottom(1)

	// Version badge
	styleVersion = lipgloss.NewStyle().
			Foreground(colorSubtle).
			Italic(true)

	// Section title
	styleSection = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorSecondary).
			MarginTop(1)

	// File path styles
	styleFilePath = lipgloss.NewStyle().
			Foreground(colorHighlight).
			Bold(true)

	styleFileInput = lipgloss.NewStyle().
			Foreground(colorText)

	styleFileOutput = lipgloss.NewStyle().
			Foreground(colorSuccess)

	// Status styles
	styleSuccess = lipgloss.NewStyle().
			Foreground(colorSuccess).
			Bold(true)

	styleWarning = lipgloss.NewStyle().
			Foreground(colorWarning).
			Bold(true)

	styleError = lipgloss.NewStyle().
			Foreground(colorError).
			Bold(true)

	styleMuted = lipgloss.NewStyle().
			Foreground(colorMuted).
			Italic(true)

	// Step styles
	styleStepLabel = lipgloss.NewStyle().
			Foreground(colorText).
			Width(12).
			Align(lipgloss.Left)

	styleStepStatus = lipgloss.NewStyle().
			Bold(true)

	styleStepTime = lipgloss.NewStyle().
			Foreground(colorSubtle).
			Italic(true)

	// Summary box
	styleSummary = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderTop(true).
			BorderForeground(colorBorder).
			MarginTop(1).
			PaddingTop(1)

	// Indent for step output
	styleIndent = lipgloss.NewStyle().
			PaddingLeft(2)

	styleNormalText = lipgloss.NewStyle().
			Foreground(colorNormal)
)

// BuildOutput manages the build output display
type BuildOutput struct {
	startTime time.Time
	fileCount int
	currentFile string
}

// NewBuildOutput creates a new build output manager
func NewBuildOutput() *BuildOutput {
	return &BuildOutput{
		startTime: time.Now(),
	}
}

// PrintHeader prints the main Dingo header (text only, mascot shown at summary)
func (b *BuildOutput) PrintHeader(version string) {
	// Styles for title
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(colorPrimary)
	versionStyle := lipgloss.NewStyle().Foreground(colorSubtle)

	// Simple text header (no mascot - mascot will be shown at the end)
	fmt.Println(titleStyle.Render("🐕 Dingo") + " " + versionStyle.Render("v"+version))
	fmt.Println()
}

// PrintBuildStart prints the build start message
func (b *BuildOutput) PrintBuildStart(fileCount int) {
	b.fileCount = fileCount

	var msg string
	if fileCount == 1 {
		msg = "📦 Building 1 file"
	} else {
		msg = fmt.Sprintf("📦 Building %d files", fileCount)
	}

	fmt.Println(styleSection.Render(msg))
	fmt.Println()
}

// PrintFileStart prints the file being processed
func (b *BuildOutput) PrintFileStart(inputPath, outputPath string) {
	b.currentFile = inputPath

	input := styleFileInput.Render(inputPath)
	arrow := styleMuted.Render("→")
	output := styleFileOutput.Render(outputPath)

	fmt.Printf("  %s %s %s\n", input, arrow, output)
	fmt.Println()
}

// Step represents a build step status
type Step struct {
	Name     string
	Status   StepStatus
	Duration time.Duration
	Message  string // Optional message (for warnings, etc.)
}

// StepStatus represents the status of a build step
type StepStatus int

const (
	StepSuccess StepStatus = iota
	StepSkipped
	StepWarning
	StepError
)

// PrintStep prints a build step with status
func (b *BuildOutput) PrintStep(step Step) {
	var icon, status, statusStyle string

	switch step.Status {
	case StepSuccess:
		icon = "✓"
		status = "Done"
		statusStyle = styleSuccess.Render(status)
	case StepSkipped:
		icon = "○"
		status = "Skipped"
		statusStyle = styleMuted.Render(status)
	case StepWarning:
		icon = "⚠"
		status = "Warning"
		statusStyle = styleWarning.Render(status)
	case StepError:
		icon = "✗"
		status = "Failed"
		statusStyle = styleError.Render(status)
	}

	// Format: "  ✓ Parse       Done (12ms)"
	label := styleStepLabel.Render(step.Name)

	line := fmt.Sprintf("  %s %s", icon, label)

	// Add status
	line += styleStepStatus.Render(statusStyle)

	// Add duration if provided
	if step.Duration > 0 {
		durationStr := formatDuration(step.Duration)
		line += " " + styleStepTime.Render("("+durationStr+")")
	}

	fmt.Println(line)

	// Print message if provided (for skipped/warning/error details)
	if step.Message != "" {
		msg := styleMuted.Render("    " + step.Message)
		fmt.Println(msg)
	}
}

// PrintSummary prints the final build summary with mascot
func (b *BuildOutput) PrintSummary(success bool, errorMsg string) {
	elapsed := time.Since(b.startTime)

	fmt.Println() // Extra line before summary

	// Get mascot for success or failure
	var m *mascot.Mascot
	if success {
		m = mascot.New(mascot.WithInitialState(mascot.StateSuccess))
	} else {
		m = mascot.New(mascot.WithInitialState(mascot.StateFailed))
	}
	frame := m.Render()

	// Apply color based on status
	var mascotColor lipgloss.Style
	if success {
		mascotColor = lipgloss.NewStyle().Foreground(colorSuccess)
	} else {
		mascotColor = lipgloss.NewStyle().Foreground(colorError)
	}

	// Build summary lines
	var summaryLines []string
	if success {
		duration := formatDuration(elapsed)
		summaryLines = []string{
			"",
			"  " + styleSuccess.Render("Build Successful!"),
			"  " + styleStepTime.Render("Completed in " + duration),
		}
	} else {
		summaryLines = []string{
			"",
			"  " + styleError.Render("Build Failed"),
		}
		if errorMsg != "" {
			summaryLines = append(summaryLines, "  "+styleError.Render("Error: ")+errorMsg)
		}
	}

	// Print mascot with summary on the right
	maxLines := len(frame)
	if len(summaryLines) > maxLines {
		maxLines = len(summaryLines)
	}

	for i := 0; i < maxLines; i++ {
		mascotLine := ""
		if i < len(frame) {
			mascotLine = mascotColor.Render(frame[i])
		}

		summaryLine := ""
		if i < len(summaryLines) {
			summaryLine = summaryLines[i]
		}

		fmt.Printf("%s%s\n", mascotLine, summaryLine)
	}
}

// PrintError prints an error message
func (b *BuildOutput) PrintError(msg string) {
	errLine := styleError.Render("✗ Error: ") + msg
	fmt.Println(styleIndent.Render(errLine))
}

// PrintWarning prints a warning message
func (b *BuildOutput) PrintWarning(msg string) {
	warnLine := styleWarning.Render("⚠ Warning: ") + msg
	fmt.Println(styleIndent.Render(warnLine))
}

// PrintInfo prints an info message
func (b *BuildOutput) PrintInfo(msg string) {
	infoLine := styleMuted.Render("ℹ " + msg)
	fmt.Println(styleIndent.Render(infoLine))
}

// Helper functions

// formatDuration formats a duration in a human-readable way
func formatDuration(d time.Duration) string {
	switch {
	case d < time.Microsecond:
		return fmt.Sprintf("%dns", d.Nanoseconds())
	case d < time.Millisecond:
		return fmt.Sprintf("%dµs", d.Microseconds())
	case d < time.Second:
		return fmt.Sprintf("%dms", d.Milliseconds())
	default:
		return fmt.Sprintf("%.2fs", d.Seconds())
	}
}

// PrintVersionInfo prints version information
func PrintVersionInfo(version string) {
	fmt.Println(styleHeader.Render("🐕 Dingo"))
	fmt.Println()
	fmt.Printf("  %s %s\n", styleMuted.Render("Version:"), styleSuccess.Render(version))
	fmt.Printf("  %s %s\n", styleMuted.Render("Runtime:"), styleNormalText.Render("Go"))
	fmt.Printf("  %s %s\n", styleMuted.Render("Website:"), styleFilePath.Render("https://dingo-lang.org"))
	fmt.Println()
}

// Box creates a bordered box around content
func Box(title, content string) string {
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorPrimary).
		Padding(1, 2).
		Width(60)

	if title != "" {
		titleStyle := lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary)

		content = titleStyle.Render(title) + "\n\n" + content
	}

	return boxStyle.Render(content)
}

// Table creates a simple two-column table
func Table(rows [][]string) string {
	var lines []string

	// Find max width of first column
	maxWidth := 0
	for _, row := range rows {
		if len(row) > 0 && len(row[0]) > maxWidth {
			maxWidth = len(row[0])
		}
	}

	for _, row := range rows {
		if len(row) >= 2 {
			label := styleMuted.Render(fmt.Sprintf("%-*s", maxWidth, row[0]))
			value := styleNormalText.Render(row[1])
			lines = append(lines, fmt.Sprintf("  %s  %s", label, value))
		}
	}

	return strings.Join(lines, "\n")
}

// ProgressBar creates a simple progress bar
func ProgressBar(current, total, width int) string {
	if width <= 0 {
		width = 40
	}

	percentage := float64(current) / float64(total)
	filled := int(percentage * float64(width))

	barStyle := lipgloss.NewStyle().Foreground(colorSuccess)
	emptyStyle := lipgloss.NewStyle().Foreground(colorMuted)

	filledBar := barStyle.Render(strings.Repeat("█", filled))
	emptyBar := emptyStyle.Render(strings.Repeat("░", width-filled))

	percentText := styleNormalText.Render(fmt.Sprintf(" %3d%%", int(percentage*100)))

	return filledBar + emptyBar + percentText
}

// Divider creates a horizontal divider
func Divider() string {
	return styleMuted.Render(strings.Repeat("─", 60))
}

// PrintDingoHelp prints colorful help output
func PrintDingoHelp(version string) {
	// Styles
	header := lipgloss.NewStyle().Bold(true).Foreground(colorPrimary)
	muted := lipgloss.NewStyle().Foreground(colorMuted)
	desc := lipgloss.NewStyle().Foreground(colorText)
	section := lipgloss.NewStyle().Bold(true).Foreground(colorSecondary)
	command := lipgloss.NewStyle().Foreground(colorSuccess)
	flag := lipgloss.NewStyle().Foreground(colorHighlight)

	// Header
	fmt.Println()
	fmt.Println(header.Render("🐕 Dingo") + " " + muted.Render("- A meta-language for Go"))
	fmt.Println(muted.Render("  v" + version))
	fmt.Println()

	// Description
	fmt.Println(desc.Render("Dingo transpiles to idiomatic Go code with Result/Option types,"))
	fmt.Println(desc.Render("pattern matching, error propagation, and 100% Go compatibility."))
	fmt.Println()

	// Usage
	fmt.Println(section.Render("Usage:"))
	fmt.Println("  dingo [command] [flags]")
	fmt.Println()

	// Commands
	fmt.Println(section.Render("Available Commands:"))
	commands := []struct{ name, desc string }{
		{"build", "Compile Dingo sources to binary (like go build)"},
		{"run", "Compile and run Dingo program (like go run)"},
		{"go", "Transpile Dingo to Go only (no compilation)"},
		{"version", "Print the version number of Dingo"},
		{"help", "Help about any command"},
	}

	for _, cmd := range commands {
		fmt.Printf("  %s  %s\n", command.Render(fmt.Sprintf("%-12s", cmd.name)), cmd.desc)
	}
	fmt.Println()

	// Flags
	fmt.Println(section.Render("Flags:"))
	fmt.Printf("  %s      help for dingo\n", flag.Render("-h, --help"))
	fmt.Printf("  %s   version for dingo\n", flag.Render("-v, --version"))
	fmt.Println()

	// Footer
	fmt.Println(muted.Render("Use \"dingo [command] --help\" for more information about a command."))
	fmt.Println()
}
