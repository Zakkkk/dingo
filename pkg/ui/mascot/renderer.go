package mascot

import (
	"fmt"
	"io"
	"strings"
)

// ANSI escape codes for terminal control
const (
	// ANSI cursor movement
	ansiMoveUp        = "\x1b[%dA"   // Move cursor up N lines
	ansiMoveDown      = "\x1b[%dB"   // Move cursor down N lines
	ansiMoveRight     = "\x1b[%dC"   // Move cursor right N columns
	ansiMoveLeft      = "\x1b[%dD"   // Move cursor left N columns
	ansiMoveBOL       = "\x1b[1G"    // Move cursor to beginning of line
	ansiMoveToLine    = "\x1b[%d;1H" // Move cursor to specific line

	// ANSI clearing
	ansiClearFromCursor = "\x1b[0J" // Clear from cursor to end of screen
	ansiClearLine       = "\x1b[2K" // Clear entire current line
	ansiClearLineToCursor = "\x1b[1K" // Clear from line start to cursor

	// ANSI cursor visibility
	ansiHideCursor = "\x1b[?25l" // Hide cursor
	ansiShowCursor = "\x1b[?25h" // Show cursor

	// ANSI reset
	ansiReset = "\x1b[0m" // Reset all attributes
)

// Renderer handles terminal rendering of mascot frames with ANSI cursor control.
// It manages frame output, clearing, and cursor positioning for smooth animations.
type Renderer struct {
	w               io.Writer // Output writer (usually os.Stdout)
	lastFrameHeight int       // Height of the last rendered frame
	cursorHidden    bool      // Whether cursor is currently hidden
}

// NewRenderer creates a new terminal renderer.
func NewRenderer(w io.Writer) *Renderer {
	return &Renderer{
		w:               w,
		lastFrameHeight: 0,
		cursorHidden:    false,
	}
}

// RenderFrame renders a frame to the terminal with the given color scheme.
// It applies colors to the frame and outputs it to the writer.
// The frame is expected to be a slice of strings, one per line.
func (r *Renderer) RenderFrame(frame []string, scheme ColorScheme) error {
	// Apply colors to the frame
	coloredFrame := ApplyColorToFrame(frame, scheme)

	// Join lines with newlines
	output := strings.Join(coloredFrame, "\n")

	// Write to output
	_, err := r.w.Write([]byte(output + "\n"))
	if err != nil {
		return fmt.Errorf("failed to render frame: %w", err)
	}

	// Update last frame height
	r.lastFrameHeight = len(frame)

	return nil
}

// Clear clears the last rendered frame from the terminal.
// It moves the cursor up to the start of the frame and clears all lines.
func (r *Renderer) Clear() error {
	if r.lastFrameHeight == 0 {
		return nil // Nothing to clear
	}

	// Generate the ANSI escape sequence to clear the frame
	clearSeq := r.ClearFrameSequence(r.lastFrameHeight)

	// Write the clear sequence
	_, err := r.w.Write([]byte(clearSeq))
	if err != nil {
		return fmt.Errorf("failed to clear frame: %w", err)
	}

	// Reset frame height
	r.lastFrameHeight = 0

	return nil
}

// ClearFrameSequence generates the ANSI escape sequence to clear N lines.
// This moves the cursor up N lines, then clears from cursor to end of screen.
func (r *Renderer) ClearFrameSequence(height int) string {
	if height == 0 {
		return ""
	}

	var sb strings.Builder

	// Move cursor to beginning of line
	sb.WriteString(ansiMoveBOL)

	// Move cursor up (height) lines to the start of the frame
	// We need to move up (height) lines because we're at the line after the frame
	if height > 0 {
		sb.WriteString(fmt.Sprintf(ansiMoveUp, height))
	}

	// Clear from cursor to end of screen
	sb.WriteString(ansiClearFromCursor)

	return sb.String()
}

// HideCursor hides the terminal cursor.
func (r *Renderer) HideCursor() error {
	if r.cursorHidden {
		return nil // Already hidden
	}

	_, err := r.w.Write([]byte(ansiHideCursor))
	if err != nil {
		return fmt.Errorf("failed to hide cursor: %w", err)
	}

	r.cursorHidden = true
	return nil
}

// ShowCursor shows the terminal cursor.
func (r *Renderer) ShowCursor() error {
	if !r.cursorHidden {
		return nil // Already visible
	}

	_, err := r.w.Write([]byte(ansiShowCursor))
	if err != nil {
		return fmt.Errorf("failed to show cursor: %w", err)
	}

	r.cursorHidden = false
	return nil
}

// Reset resets all terminal attributes and shows the cursor.
func (r *Renderer) Reset() error {
	// Show cursor if hidden
	if r.cursorHidden {
		if err := r.ShowCursor(); err != nil {
			return err
		}
	}

	// Reset terminal attributes
	_, err := r.w.Write([]byte(ansiReset))
	if err != nil {
		return fmt.Errorf("failed to reset terminal: %w", err)
	}

	return nil
}

// GetFrameHeight returns the height (number of lines) of a frame.
func GetFrameHeight(frame []string) int {
	return len(frame)
}

// PadFrameHeight pads a frame to a target height by adding empty lines.
// If the frame is already taller than targetHeight, it is returned unchanged.
func PadFrameHeight(frame []string, targetHeight int) []string {
	currentHeight := len(frame)

	if currentHeight >= targetHeight {
		return frame // Already at or above target height
	}

	// Create a new slice with the padded height
	paddedFrame := make([]string, targetHeight)

	// Copy original frame
	copy(paddedFrame, frame)

	// Fill remaining lines with empty strings
	for i := currentHeight; i < targetHeight; i++ {
		paddedFrame[i] = ""
	}

	return paddedFrame
}

// ApplyColorToFrame applies a color scheme to all lines in a frame.
// This is a simple implementation that applies body color to all non-empty lines.
// For more sophisticated coloring, use the ColorScheme methods directly.
func ApplyColorToFrame(frame []string, scheme ColorScheme) []string {
	colored := make([]string, len(frame))

	for i, line := range frame {
		if line == "" {
			colored[i] = line // Keep empty lines empty
			continue
		}

		// Apply body color to the entire line
		// For more sophisticated coloring, you would parse the line
		// and apply different colors to different parts
		colored[i] = scheme.ApplyBodyColor(line)
	}

	return colored
}

// ClearScreen clears the entire terminal screen.
func (r *Renderer) ClearScreen() error {
	// Clear entire screen and move cursor to top-left
	_, err := r.w.Write([]byte("\x1b[2J\x1b[H"))
	if err != nil {
		return fmt.Errorf("failed to clear screen: %w", err)
	}
	return nil
}

// MoveCursorUp moves the cursor up by n lines.
func (r *Renderer) MoveCursorUp(n int) error {
	if n <= 0 {
		return nil
	}

	_, err := r.w.Write([]byte(fmt.Sprintf(ansiMoveUp, n)))
	if err != nil {
		return fmt.Errorf("failed to move cursor up: %w", err)
	}
	return nil
}

// MoveCursorDown moves the cursor down by n lines.
func (r *Renderer) MoveCursorDown(n int) error {
	if n <= 0 {
		return nil
	}

	_, err := r.w.Write([]byte(fmt.Sprintf(ansiMoveDown, n)))
	if err != nil {
		return fmt.Errorf("failed to move cursor down: %w", err)
	}
	return nil
}

// ClearLine clears the current line.
func (r *Renderer) ClearLine() error {
	_, err := r.w.Write([]byte(ansiClearLine))
	if err != nil {
		return fmt.Errorf("failed to clear line: %w", err)
	}
	return nil
}

// RenderAt renders a frame at a specific position on the screen.
// This is useful for side-by-side layouts where the mascot needs to be at a fixed position.
func (r *Renderer) RenderAt(frame []string, scheme ColorScheme, x, y int) error {
	// Move to starting position
	_, err := r.w.Write([]byte(fmt.Sprintf("\x1b[%d;%dH", y, x)))
	if err != nil {
		return fmt.Errorf("failed to move cursor to position: %w", err)
	}

	// Render each line at the specified column
	coloredFrame := ApplyColorToFrame(frame, scheme)
	for i, line := range coloredFrame {
		// Move to column x on line y+i
		_, err := r.w.Write([]byte(fmt.Sprintf("\x1b[%d;%dH", y+i, x)))
		if err != nil {
			return fmt.Errorf("failed to move cursor: %w", err)
		}

		// Write the line
		_, err = r.w.Write([]byte(line))
		if err != nil {
			return fmt.Errorf("failed to write line: %w", err)
		}
	}

	// Update last frame height
	r.lastFrameHeight = len(frame)

	return nil
}

// Flush flushes the output writer if it implements io.Flusher.
func (r *Renderer) Flush() error {
	if flusher, ok := r.w.(interface{ Flush() error }); ok {
		return flusher.Flush()
	}
	return nil
}
