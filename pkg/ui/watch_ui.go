package ui

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/MadAppGang/dingo/pkg/ui/mascot"
	"github.com/MadAppGang/dingo/pkg/version"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"golang.org/x/term"
	"os"
)

// WatchUI provides persistent UI for watch mode with mascot states and build history
type WatchUI struct {
	mascot      *mascot.Mascot
	state       mascot.MascotState
	colorScheme mascot.ColorScheme

	// Build history
	history    []BuildEvent
	maxHistory int // Default: 10

	// Current status
	currentStatus string
	currentDetail string

	// Terminal state
	isTTY        bool
	screenHeight int
	screenWidth  int

	// Control
	enabled bool // Set to false when --no-mascot is used

	mu sync.Mutex
}

// BuildEvent represents a build event in history
type BuildEvent struct {
	Timestamp time.Time
	EventType BuildEventType
	Message   string
	FilePath  string        // Optional: file that triggered rebuild
	Duration  time.Duration // Optional: for build events
}

// BuildEventType represents different types of build events
type BuildEventType int

const (
	EventStartup BuildEventType = iota
	EventFileChanged
	EventBuildStart
	EventBuildSuccess
	EventBuildFailed
	EventRestart
	EventShutdown
)

// NewWatchUI creates a new watch UI
func NewWatchUI() *WatchUI {
	lipgloss.SetColorProfile(termenv.TrueColor)

	isTTY := term.IsTerminal(int(os.Stdout.Fd()))
	width, height, _ := term.GetSize(int(os.Stdout.Fd()))

	return &WatchUI{
		mascot:       mascot.New(mascot.WithInitialState(mascot.StateIdle)),
		state:        mascot.StateIdle,
		colorScheme:  mascot.DefaultColorScheme,
		history:      make([]BuildEvent, 0, 10),
		maxHistory:   10,
		isTTY:        isTTY,
		screenWidth:  width,
		screenHeight: height,
		enabled:      isTTY, // Only enable UI in TTY terminals
	}
}

// WithMaxHistory sets the maximum number of history entries
func (ui *WatchUI) WithMaxHistory(n int) *WatchUI {
	ui.mu.Lock()
	defer ui.mu.Unlock()
	ui.maxHistory = n
	return ui
}

// Disable disables the UI (for --no-mascot flag)
func (ui *WatchUI) Disable() {
	ui.mu.Lock()
	defer ui.mu.Unlock()
	ui.enabled = false
}

// Start initializes and displays the UI
func (ui *WatchUI) Start() {
	ui.mu.Lock()
	defer ui.mu.Unlock()

	if !ui.enabled {
		return
	}

	if ui.isTTY {
		// Hide cursor during watch mode
		fmt.Print("\033[?25l")

		// Initial render
		ui.render()
	}
}

// Stop cleans up the UI
func (ui *WatchUI) Stop() {
	ui.mu.Lock()
	defer ui.mu.Unlock()

	if !ui.enabled {
		return
	}

	if ui.isTTY {
		// Show cursor
		fmt.Print("\033[?25h")
		fmt.Println() // Final newline
	}
}

// SetState transitions mascot to new state
func (ui *WatchUI) SetState(state mascot.MascotState, status, detail string) {
	ui.mu.Lock()
	defer ui.mu.Unlock()

	ui.state = state
	ui.currentStatus = status
	ui.currentDetail = detail
	ui.mascot.SetState(state)

	// Update color scheme based on state
	switch state {
	case mascot.StateSuccess:
		ui.colorScheme = mascot.SuccessColorScheme
	case mascot.StateFailed:
		ui.colorScheme = mascot.FailureColorScheme
	case mascot.StateCompiling:
		ui.colorScheme = mascot.CompileColorScheme
	default:
		ui.colorScheme = mascot.DefaultColorScheme
	}

	if ui.enabled {
		ui.render()
	}
}

// AddEvent adds an event to build history and re-renders
func (ui *WatchUI) AddEvent(event BuildEvent) {
	ui.mu.Lock()
	defer ui.mu.Unlock()

	// Add event to history
	ui.history = append(ui.history, event)

	// Trim history to max size
	if len(ui.history) > ui.maxHistory {
		ui.history = ui.history[len(ui.history)-ui.maxHistory:]
	}

	if ui.enabled {
		ui.render()
	}
}

// Render redraws the entire UI screen (must be called with lock held)
func (ui *WatchUI) render() {
	if !ui.isTTY {
		return
	}

	// Clear screen and move cursor to top
	ui.clearScreen()

	// Build and print layout
	layout := ui.renderLayout()
	fmt.Print(layout)
}

// renderLayout builds the complete UI layout (must be called with lock held)
func (ui *WatchUI) renderLayout() string {
	var output strings.Builder

	// Header
	output.WriteString(ui.renderHeader())
	output.WriteString("\n")

	// Separator
	separatorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#45475A"))
	output.WriteString(separatorStyle.Render(strings.Repeat("─", 60)))
	output.WriteString("\n\n")

	// Mascot + Status section
	output.WriteString(ui.renderMascotSection())
	output.WriteString("\n")

	// Separator
	output.WriteString(separatorStyle.Render(strings.Repeat("─", 60)))
	output.WriteString("\n")

	// Build history
	output.WriteString(ui.renderHistory())
	output.WriteString("\n")

	return output.String()
}

// renderHeader renders the title and timestamp (must be called with lock held)
func (ui *WatchUI) renderHeader() string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7D56F4"))
	versionStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6C7086"))
	timestampStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6C7086"))

	title := titleStyle.Render("🐕 Dingo Watch") + " " + versionStyle.Render("v"+version.Version)
	timestamp := timestampStyle.Render(fmt.Sprintf("[%s]", time.Now().Format("2006-01-02 15:04")))

	// Pad to align timestamp to right (assuming 60 char width)
	titleLen := 15 + len(version.Version) // Approximate visible length
	timestampLen := 18                    // [YYYY-MM-DD HH:MM]
	padding := 60 - titleLen - timestampLen
	if padding < 0 {
		padding = 0
	}

	return title + strings.Repeat(" ", padding) + timestamp
}

// renderMascotSection renders mascot with status on the right (must be called with lock held)
func (ui *WatchUI) renderMascotSection() string {
	var output strings.Builder

	// Get mascot frame
	frame := ui.mascot.Render()

	// Status lines
	statusLines := ui.getStatusLines()

	// Print side by side
	maxLines := len(frame)
	if len(statusLines) > maxLines {
		maxLines = len(statusLines)
	}

	for i := 0; i < maxLines; i++ {
		mascotLine := ""
		if i < len(frame) {
			mascotLine = ui.colorScheme.ApplyBodyColor(frame[i])
		} else {
			mascotLine = strings.Repeat(" ", 22) // Mascot width
		}

		statusLine := ""
		if i < len(statusLines) {
			statusLine = "  " + statusLines[i]
		}

		output.WriteString(mascotLine)
		output.WriteString(statusLine)
		output.WriteString("\n")
	}

	return output.String()
}

// getStatusLines builds the status text lines (must be called with lock held)
func (ui *WatchUI) getStatusLines() []string {
	var lines []string

	lines = append(lines, "") // Spacing

	statusStyle := lipgloss.NewStyle().Bold(true)
	switch ui.state {
	case mascot.StateSuccess:
		statusStyle = statusStyle.Foreground(lipgloss.Color("#5AF78E"))
	case mascot.StateFailed:
		statusStyle = statusStyle.Foreground(lipgloss.Color("#FF6B9D"))
	case mascot.StateCompiling:
		statusStyle = statusStyle.Foreground(lipgloss.Color("#7D56F4"))
	default:
		statusStyle = statusStyle.Foreground(lipgloss.Color("#CDD6F4"))
	}

	if ui.currentStatus != "" {
		lines = append(lines, statusStyle.Render("Status: ")+ui.currentStatus)
	}

	if ui.currentDetail != "" {
		detailStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6C7086")).Italic(true)
		lines = append(lines, detailStyle.Render(ui.currentDetail))
	}

	return lines
}

// renderHistory renders the build history (must be called with lock held)
func (ui *WatchUI) renderHistory() string {
	var output strings.Builder

	// Section header
	sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#56C3F4"))
	output.WriteString(sectionStyle.Render("Build History"))
	output.WriteString("\n\n")

	if len(ui.history) == 0 {
		mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6C7086")).Italic(true)
		output.WriteString(mutedStyle.Render("  No events yet"))
		output.WriteString("\n")
		return output.String()
	}

	// Show history in reverse order (most recent last)
	for _, event := range ui.history {
		output.WriteString(ui.formatEvent(event))
		output.WriteString("\n")
	}

	return output.String()
}

// formatEvent formats a single build event (must be called with lock held)
func (ui *WatchUI) formatEvent(event BuildEvent) string {
	timestampStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6C7086"))
	timestamp := timestampStyle.Render(fmt.Sprintf("[%s]", event.Timestamp.Format("15:04:05")))

	var icon, message string
	messageStyle := lipgloss.NewStyle()

	switch event.EventType {
	case EventStartup:
		icon = "🔄"
		message = event.Message
		messageStyle = messageStyle.Foreground(lipgloss.Color("#7D56F4"))

	case EventFileChanged:
		icon = "📝"
		message = fmt.Sprintf("Change detected: %s", event.Message)
		messageStyle = messageStyle.Foreground(lipgloss.Color("#56C3F4"))

	case EventBuildStart:
		icon = "🔨"
		message = event.Message
		messageStyle = messageStyle.Foreground(lipgloss.Color("#7D56F4"))

	case EventBuildSuccess:
		icon = "✓"
		if event.Duration > 0 {
			message = fmt.Sprintf("%s (%s)", event.Message, formatDuration(event.Duration))
		} else {
			message = event.Message
		}
		messageStyle = messageStyle.Foreground(lipgloss.Color("#5AF78E"))

	case EventBuildFailed:
		icon = "✗"
		message = event.Message
		messageStyle = messageStyle.Foreground(lipgloss.Color("#FF6B9D"))

	case EventRestart:
		icon = "▶"
		message = event.Message
		messageStyle = messageStyle.Foreground(lipgloss.Color("#5AF78E"))

	case EventShutdown:
		icon = "⏹"
		message = event.Message
		messageStyle = messageStyle.Foreground(lipgloss.Color("#6C7086"))
	}

	return fmt.Sprintf("  %s %s %s", timestamp, icon, messageStyle.Render(message))
}

// clearScreen clears the terminal and resets cursor to top
func (ui *WatchUI) clearScreen() {
	// Clear screen: ESC[2J
	// Move cursor to home: ESC[H
	fmt.Print("\033[2J\033[H")
}
