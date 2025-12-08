package mascot

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mattn/go-runewidth"
)

// LayoutMode determines how mascot and output are arranged.
type LayoutMode int

const (
	LayoutSideBySide LayoutMode = iota // Mascot on left, output on right
	LayoutDisabled                      // No mascot, output only
)

// DefaultMascotWidth is the fixed width for the mascot column.
const DefaultMascotWidth = 24

// DefaultSeparatorWidth is the width of the separator between mascot and output.
const DefaultSeparatorWidth = 2

// Layout manages the side-by-side rendering of mascot and output.
// It implements io.Writer to capture command output and combines it
// with the animated mascot in a side-by-side layout.
type Layout struct {
	mascot       *Mascot
	outputBuffer *strings.Builder
	mode         LayoutMode
	termWidth    int
	mascotWidth  int // Fixed width for mascot column
	outputWidth  int // Remaining width for output
	writer       io.Writer
	renderer     *Renderer
	lastHeight   int // Height of last rendered frame for clearing

	// Animation control
	paused  bool
	stopCh  chan struct{}
	doneCh  chan struct{}
	running atomic.Bool

	// Thread safety
	mu sync.Mutex

	// Static mode (for non-animated display like version/help)
	static bool
}

// LayoutOption is a functional option for configuring the Layout.
type LayoutOption func(*Layout)

// WithMascotState sets the initial mascot state.
func WithMascotState(state MascotState) LayoutOption {
	return func(l *Layout) {
		if l.mascot != nil {
			l.mascot.SetState(state)
		}
	}
}

// WithLayoutWriter sets the output writer for the layout.
// Default is os.Stdout.
func WithLayoutWriter(w io.Writer) LayoutOption {
	return func(l *Layout) {
		l.writer = w
	}
}

// WithStatic sets static mode for non-animated display.
// In static mode, the mascot is rendered once without animation.
func WithStatic(static bool) LayoutOption {
	return func(l *Layout) {
		l.static = static
	}
}

// NewLayout creates a layout with automatic mode detection.
// It detects terminal capabilities and chooses the appropriate layout mode.
func NewLayout(opts ...LayoutOption) *Layout {
	// Detect terminal capabilities
	caps := Detect()

	// Determine layout mode based on capabilities
	mode := LayoutDisabled
	termWidth := caps.Width

	// Check if we should show mascot
	// Note: flagNoMascot is not available here, will be checked by caller
	// For now, we assume no flag and rely on env var and terminal detection
	if ShouldShowMascot(caps, false) {
		mode = LayoutSideBySide
	}

	// Create layout
	l := &Layout{
		mode:         mode,
		termWidth:    termWidth,
		mascotWidth:  DefaultMascotWidth,
		outputWidth:  termWidth - DefaultMascotWidth - DefaultSeparatorWidth,
		outputBuffer: &strings.Builder{},
		writer:       os.Stdout,
		stopCh:       make(chan struct{}),
		doneCh:       make(chan struct{}),
		paused:       false,
		static:       false,
	}

	// Create mascot if in side-by-side mode
	if l.mode == LayoutSideBySide {
		l.mascot = New(
			WithWriter(io.Discard), // Mascot renders via layout, not directly
		)
		l.renderer = NewRenderer(l.writer)
	}

	// Apply options
	for _, opt := range opts {
		opt(l)
	}

	return l
}

// Mode returns the current layout mode.
func (l *Layout) Mode() LayoutMode {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.mode
}

// Write implements io.Writer for capturing output.
// It appends output to the internal buffer and re-renders the layout.
func (l *Layout) Write(p []byte) (n int, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Always append to buffer
	n, err = l.outputBuffer.Write(p)
	if err != nil {
		return n, err
	}

	// If disabled mode, write directly to output
	if l.mode == LayoutDisabled {
		_, writeErr := l.writer.Write(p)
		if writeErr != nil {
			return n, writeErr
		}
	}

	// In side-by-side mode, layout rendering happens in the animation loop
	// or via Flush() for static mode

	return n, nil
}

// SetMascotState updates the mascot animation state.
func (l *Layout) SetMascotState(state MascotState) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.mascot != nil {
		l.mascot.SetState(state)
	}
}

// Start begins the animation loop.
// This starts the mascot animation and renders the side-by-side layout.
func (l *Layout) Start() {
	l.mu.Lock()
	mode := l.mode
	static := l.static
	l.mu.Unlock()

	// Only start in side-by-side mode
	if mode != LayoutSideBySide {
		return
	}

	// Static mode: render once and return
	if static {
		l.renderOnce()
		return
	}

	// Start mascot animation
	if l.mascot != nil {
		l.mascot.Start()
	}

	// Start layout rendering loop
	if l.running.CompareAndSwap(false, true) {
		go l.renderLoop()
	}
}

// Stop ends the animation and clears the display.
func (l *Layout) Stop() {
	l.mu.Lock()
	mode := l.mode
	static := l.static
	l.mu.Unlock()

	// Only relevant in side-by-side mode
	if mode != LayoutSideBySide {
		return
	}

	// Static mode: just clear
	if static {
		l.clearLastFrame()
		return
	}

	// Check if already running
	if !l.running.Load() {
		return // Not running, nothing to stop
	}

	// Stop mascot animation
	if l.mascot != nil {
		l.mascot.Stop()
	}

	// Signal render loop to stop
	close(l.stopCh)

	// Wait for render loop to finish
	<-l.doneCh

	l.running.Store(false)

	// Clear the last rendered frame
	l.clearLastFrame()

	// Show cursor
	if l.renderer != nil {
		l.renderer.ShowCursor()
	}
}

// Pause pauses the animation (for program execution).
func (l *Layout) Pause() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.paused = true
}

// Resume resumes the animation.
func (l *Layout) Resume() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.paused = false
}

// Flush writes the final output.
// In side-by-side mode, it renders the final frame.
// In disabled mode, it's a no-op (output already written).
func (l *Layout) Flush() {
	l.mu.Lock()
	mode := l.mode
	l.mu.Unlock()

	if mode == LayoutSideBySide {
		l.renderOnce()
	}
}

// renderLoop runs the rendering loop in a goroutine.
func (l *Layout) renderLoop() {
	defer close(l.doneCh)

	// Hide cursor for smoother animation
	if l.renderer != nil {
		l.renderer.HideCursor()
	}

	ticker := time.NewTicker(100 * time.Millisecond) // 10 FPS
	defer ticker.Stop()

	for {
		select {
		case <-l.stopCh:
			return
		case <-ticker.C:
			func() {
				l.mu.Lock()
				defer l.mu.Unlock()

				if !l.paused {
					l.renderFrame()
				}
			}()
		}
	}
}

// renderOnce renders the layout once (for static mode).
func (l *Layout) renderOnce() {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.renderFrame()
}

// renderFrame renders the current frame of the side-by-side layout.
// This must be called with l.mu locked.
func (l *Layout) renderFrame() {
	if l.mascot == nil || l.renderer == nil {
		return
	}

	// Clear previous frame
	if l.lastHeight > 0 {
		clearSeq := l.renderer.ClearFrameSequence(l.lastHeight)
		if _, err := l.writer.Write([]byte(clearSeq)); err != nil {
			// Stop rendering on write failure
			l.paused = true
			return
		}
	}

	// Get mascot lines
	mascotLines := l.mascot.Render()

	// Apply color scheme to mascot lines
	colorScheme := l.getColorSchemeForState()
	coloredMascotLines := ApplyColorToFrame(mascotLines, colorScheme)

	// Get output lines
	outputText := l.outputBuffer.String()
	outputLines := strings.Split(outputText, "\n")

	// Combine side-by-side
	combined := l.combineSideBySide(coloredMascotLines, outputLines)

	// Write combined output
	combinedText := strings.Join(combined, "\n")
	if _, err := l.writer.Write([]byte(combinedText + "\n")); err != nil {
		// Stop rendering on write failure
		l.paused = true
		return
	}

	// Update last height
	l.lastHeight = len(combined)
}

// combineSideBySide combines mascot and output lines side-by-side.
// This must be called with l.mu locked.
func (l *Layout) combineSideBySide(mascotLines, outputLines []string) []string {
	// Determine maximum height
	maxHeight := len(mascotLines)
	if len(outputLines) > maxHeight {
		maxHeight = len(outputLines)
	}

	// Create combined lines
	combined := make([]string, maxHeight)

	for i := 0; i < maxHeight; i++ {
		var line strings.Builder

		// Add mascot column (left)
		if i < len(mascotLines) {
			mascotLine := mascotLines[i]
			visualWidth := runewidth.StringWidth(mascotLine) // Get visual width

			if visualWidth < l.mascotWidth {
				line.WriteString(mascotLine)
				line.WriteString(strings.Repeat(" ", l.mascotWidth-visualWidth))
			} else if visualWidth > l.mascotWidth {
				// Truncate to exact visual width without cutting characters
				line.WriteString(runewidth.Truncate(mascotLine, l.mascotWidth, ""))
			} else {
				line.WriteString(mascotLine)
			}
		} else {
			// Empty mascot line
			line.WriteString(strings.Repeat(" ", l.mascotWidth))
		}

		// Add separator (2 spaces)
		line.WriteString("  ")

		// Add output column (right)
		if i < len(outputLines) {
			outputLine := outputLines[i]
			visualWidth := runewidth.StringWidth(outputLine)

			if l.termWidth > 0 && visualWidth > l.outputWidth {
				line.WriteString(runewidth.Truncate(outputLine, l.outputWidth, ""))
			} else {
				line.WriteString(outputLine)
			}
		}

		combined[i] = line.String()
	}

	return combined
}

// clearLastFrame clears the last rendered frame.
func (l *Layout) clearLastFrame() {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.renderer != nil && l.lastHeight > 0 {
		clearSeq := l.renderer.ClearFrameSequence(l.lastHeight)
		l.writer.Write([]byte(clearSeq))
		l.lastHeight = 0
	}
}

// GetOutputBuffer returns the accumulated output as a string.
// This is useful for testing or retrieving the final output.
func (l *Layout) GetOutputBuffer() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.outputBuffer.String()
}

// ClearOutput clears the output buffer.
func (l *Layout) ClearOutput() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.outputBuffer.Reset()
}

// getColorSchemeForState returns the appropriate color scheme based on mascot state.
func (l *Layout) getColorSchemeForState() ColorScheme {
	if l.mascot == nil {
		return DefaultColorScheme
	}

	state := l.mascot.state
	return getColorSchemeForState(state)
}

// getColorSchemeForState maps mascot states to color schemes.
func getColorSchemeForState(state MascotState) ColorScheme {
	switch state {
	case StateSuccess:
		return SuccessColorScheme
	case StateFailed:
		return FailureColorScheme
	case StateCompiling, StateRunning:
		return CompileColorScheme
	default:
		return DefaultColorScheme
	}
}

// String returns the current layout as a string (for debugging).
func (l *Layout) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()

	return fmt.Sprintf("Layout{mode=%v, termWidth=%d, mascotWidth=%d, outputWidth=%d, paused=%v, static=%v}",
		l.mode, l.termWidth, l.mascotWidth, l.outputWidth, l.paused, l.static)
}
