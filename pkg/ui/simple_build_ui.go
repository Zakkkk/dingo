// Package ui provides a simple animated build UI
package ui

import (
	"fmt"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"golang.org/x/term"

	"github.com/MadAppGang/dingo/pkg/ui/mascot"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// SimpleBuildUI provides animated mascot during build without complex TUI
type SimpleBuildUI struct {
	mascot       *mascot.Mascot
	state        mascot.MascotState
	status       string
	statusDetail string
	colorScheme  mascot.ColorScheme

	// Animation
	stopCh  chan struct{}
	doneCh  chan struct{}
	running bool
	isTTY   bool

	mu sync.Mutex
}

// NewSimpleBuildUI creates a simple animated build UI
func NewSimpleBuildUI() *SimpleBuildUI {
	lipgloss.SetColorProfile(termenv.TrueColor)

	isTTY := term.IsTerminal(int(os.Stdout.Fd()))

	return &SimpleBuildUI{
		mascot:      mascot.New(mascot.WithInitialState(mascot.StateCompiling)),
		state:       mascot.StateCompiling,
		colorScheme: mascot.CompileColorScheme,
		stopCh:      make(chan struct{}),
		doneCh:      make(chan struct{}),
		isTTY:       isTTY,
	}
}

// Start begins the animated mascot (if TTY)
func (ui *SimpleBuildUI) Start() {
	ui.mu.Lock()
	if ui.running {
		ui.mu.Unlock()
		return
	}
	ui.running = true
	ui.mu.Unlock()

	if ui.isTTY {
		// Hide cursor during build
		fmt.Print("\033[?25l")
	}
}

// Stop ends animation and shows final mascot
func (ui *SimpleBuildUI) Stop() {
	ui.mu.Lock()
	if !ui.running {
		ui.mu.Unlock()
		return
	}
	ui.running = false
	ui.mu.Unlock()

	if ui.isTTY {
		// Show cursor
		fmt.Print("\033[?25h")
	}

	// Print final mascot with status
	ui.printMascot()
}

// SetStatus updates mascot state and status
func (ui *SimpleBuildUI) SetStatus(state mascot.MascotState, status, detail string) {
	ui.mu.Lock()
	defer ui.mu.Unlock()

	ui.state = state
	ui.status = status
	ui.statusDetail = detail
	ui.mascot.SetState(state)

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
}

// Printf prints formatted output
func (ui *SimpleBuildUI) Printf(format string, args ...interface{}) {
	fmt.Printf(format, args...)
}

// printMascot prints the mascot with status
func (ui *SimpleBuildUI) printMascot() {
	ui.mu.Lock()
	defer ui.mu.Unlock()

	fmt.Println()

	// Separator
	separatorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#45475A"))
	fmt.Println(separatorStyle.Render(strings.Repeat("─", 60)))

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

		fmt.Printf("%s%s\n", mascotLine, statusLine)
	}
}

func (ui *SimpleBuildUI) getStatusLines() []string {
	var lines []string

	lines = append(lines, "") // Spacing

	statusStyle := lipgloss.NewStyle().Bold(true)
	switch ui.state {
	case mascot.StateSuccess:
		statusStyle = statusStyle.Foreground(lipgloss.Color("#5AF78E"))
	case mascot.StateFailed:
		statusStyle = statusStyle.Foreground(lipgloss.Color("#FF6B9D"))
	default:
		statusStyle = statusStyle.Foreground(lipgloss.Color("#7D56F4"))
	}

	if ui.status != "" {
		lines = append(lines, statusStyle.Render(ui.status))
	}

	if ui.statusDetail != "" {
		detailStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6C7086")).Italic(true)
		lines = append(lines, detailStyle.Render(ui.statusDetail))
	}

	// Add funny tagline for success
	if ui.state == mascot.StateSuccess {
		lines = append(lines, "") // Spacing
		taglineStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#F7DC6F")).Italic(true)
		lines = append(lines, taglineStyle.Render(getRandomTagline()))
		lines = append(lines, "") // Spacing

		// Show ONE random promo link (not spammy!)
		lines = append(lines, getRandomPromo())
	}

	// Add sad tagline for failure
	if ui.state == mascot.StateFailed {
		lines = append(lines, "") // Spacing
		taglineStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF6B9D")).Italic(true)
		lines = append(lines, taglineStyle.Render(getRandomFailTagline()))
	}

	return lines
}

// Funny success taglines
var successTaglines = []string{
	"Your secret is safe with me! Nobody will know it's Dingo",
	"*sniff sniff* That Go code smells delicious!",
	"Even gophers can't tell the difference!",
	"Another masterpiece disguised as pure Go!",
	"The Go compiler will never suspect a thing...",
	"Woof! Your code is looking sharp!",
	"Transpiled with love and tail wags!",
	"*chef's kiss* Magnifique Go code!",
	"Incognito mode: ON. It's all Go now!",
	"Go-ing places with your code!",
	"That's some tasty spaghetti... I mean, Go code!",
	"100% organic, free-range Go code!",
	"No dingos were harmed in making this Go code",
	"Shh... let them think you wrote Go the hard way",
}

// Sad failure taglines
var failTaglines = []string{
	"*whimper* Something went wrong...",
	"Even dingos make mistakes sometimes",
	"Let's try that again, shall we?",
	"Oops! My paw slipped on the keyboard",
	"Not every hunt is successful...",
}

func getRandomTagline() string {
	return successTaglines[rand.Intn(len(successTaglines))]
}

func getRandomFailTagline() string {
	return failTaglines[rand.Intn(len(failTaglines))]
}

// Promo links - show one at random
var promoLinks = []struct {
	icon string
	text string
	link string
}{
	{"⭐", "Star us:", "github.com/MadAppGang/dingo"},
	{"🐕", "Follow:", "x.com/jackrudenko"},
	{"💼", "Hire us:", "madappgang.com"},
	{"🤖", "Try:", "claudish.com - Claude Code, Any Model"},
}

func getRandomPromo() string {
	linkStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6C7086"))
	highlightStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#56C3F4"))

	promo := promoLinks[rand.Intn(len(promoLinks))]
	return promo.icon + " " + linkStyle.Render(promo.text) + " " + highlightStyle.Render(promo.link)
}

// AnimatedBuildUI runs the build with an animated spinner mascot
type AnimatedBuildUI struct {
	*SimpleBuildUI
	spinnerFrames []string
	frameIndex    int
}

// NewAnimatedBuildUI creates a build UI with animated spinner during processing
func NewAnimatedBuildUI() *AnimatedBuildUI {
	return &AnimatedBuildUI{
		SimpleBuildUI: NewSimpleBuildUI(),
		spinnerFrames: []string{"◜", "◝", "◞", "◟"},
	}
}

// RunWithSpinner runs a function while showing animated spinner
func (ui *AnimatedBuildUI) RunWithSpinner(status, detail string, fn func() error) error {
	ui.SetStatus(mascot.StateCompiling, status, detail)

	if !ui.isTTY {
		// No animation in non-TTY
		return fn()
	}

	// Start spinner animation
	done := make(chan error, 1)
	go func() {
		done <- fn()
	}()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	// Print initial spinner line
	spinnerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#7D56F4"))
	statusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#CDD6F4"))

	for {
		select {
		case err := <-done:
			// Clear spinner line
			fmt.Print("\r\033[2K")
			return err
		case <-ticker.C:
			spinner := ui.spinnerFrames[ui.frameIndex]
			ui.frameIndex = (ui.frameIndex + 1) % len(ui.spinnerFrames)
			fmt.Printf("\r  %s %s", spinnerStyle.Render(spinner), statusStyle.Render(status))
		}
	}
}
