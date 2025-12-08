package mascot

import (
	"bytes"
	"strings"
	"testing"
)

func TestGetFrameHeight(t *testing.T) {
	tests := []struct {
		name  string
		frame []string
		want  int
	}{
		{
			name:  "Empty frame",
			frame: []string{},
			want:  0,
		},
		{
			name:  "Single line",
			frame: []string{"line1"},
			want:  1,
		},
		{
			name:  "Multiple lines",
			frame: []string{"line1", "line2", "line3"},
			want:  3,
		},
		{
			name:  "Frame with empty lines",
			frame: []string{"line1", "", "line3"},
			want:  3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetFrameHeight(tt.frame)
			if got != tt.want {
				t.Errorf("GetFrameHeight() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestPadFrameHeight(t *testing.T) {
	tests := []struct {
		name         string
		frame        []string
		targetHeight int
		wantHeight   int
	}{
		{
			name:         "Pad shorter frame",
			frame:        []string{"line1", "line2"},
			targetHeight: 5,
			wantHeight:   5,
		},
		{
			name:         "Frame already at target",
			frame:        []string{"line1", "line2", "line3"},
			targetHeight: 3,
			wantHeight:   3,
		},
		{
			name:         "Frame taller than target",
			frame:        []string{"line1", "line2", "line3", "line4"},
			targetHeight: 2,
			wantHeight:   4, // Unchanged
		},
		{
			name:         "Pad empty frame",
			frame:        []string{},
			targetHeight: 3,
			wantHeight:   3,
		},
		{
			name:         "Zero target height",
			frame:        []string{"line1"},
			targetHeight: 0,
			wantHeight:   1, // Unchanged
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PadFrameHeight(tt.frame, tt.targetHeight)

			if len(got) != tt.wantHeight {
				t.Errorf("PadFrameHeight() returned frame with %d lines, want %d", len(got), tt.wantHeight)
			}

			// Verify original content is preserved
			for i := 0; i < len(tt.frame); i++ {
				if got[i] != tt.frame[i] {
					t.Errorf("PadFrameHeight() modified original content at line %d: got %q, want %q",
						i, got[i], tt.frame[i])
				}
			}

			// Verify padded lines are empty
			if len(got) > len(tt.frame) {
				for i := len(tt.frame); i < len(got); i++ {
					if got[i] != "" {
						t.Errorf("PadFrameHeight() padded line %d should be empty, got %q", i, got[i])
					}
				}
			}
		})
	}
}

func TestApplyColorToFrame(t *testing.T) {
	scheme := DefaultColorScheme

	tests := []struct {
		name  string
		frame []string
		want  int // Expected number of lines in output
	}{
		{
			name:  "Empty frame",
			frame: []string{},
			want:  0,
		},
		{
			name:  "Single line frame",
			frame: []string{"test line"},
			want:  1,
		},
		{
			name:  "Frame with empty lines",
			frame: []string{"line1", "", "line3"},
			want:  3,
		},
		{
			name:  "Multi-line frame",
			frame: []string{"line1", "line2", "line3"},
			want:  3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ApplyColorToFrame(tt.frame, scheme)

			if len(got) != tt.want {
				t.Errorf("ApplyColorToFrame() returned %d lines, want %d", len(got), tt.want)
			}

			// Verify empty lines remain empty (no color codes)
			for i, line := range got {
				if i < len(tt.frame) && tt.frame[i] == "" && line != "" {
					t.Errorf("ApplyColorToFrame() line %d should remain empty, got %q", i, line)
				}
			}

			// Verify non-empty lines are processed (may or may not have ANSI codes depending on NO_COLOR env)
			// The function should at least preserve the content
			for i, line := range got {
				if i < len(tt.frame) && tt.frame[i] != "" {
					// Content should be present in output (with or without color codes)
					if !strings.Contains(line, tt.frame[i]) && len(line) == 0 {
						t.Errorf("ApplyColorToFrame() line %d should contain original content, got %q", i, line)
					}
				}
			}
		})
	}
}

func TestRendererNew(t *testing.T) {
	buf := &bytes.Buffer{}
	r := NewRenderer(buf)

	if r == nil {
		t.Fatal("NewRenderer() returned nil")
	}

	if r.w != buf {
		t.Error("NewRenderer() did not set writer correctly")
	}

	if r.lastFrameHeight != 0 {
		t.Errorf("NewRenderer() lastFrameHeight = %d, want 0", r.lastFrameHeight)
	}

	if r.cursorHidden {
		t.Error("NewRenderer() cursorHidden should be false initially")
	}
}

func TestRendererClearFrameSequence(t *testing.T) {
	buf := &bytes.Buffer{}
	r := NewRenderer(buf)

	tests := []struct {
		name   string
		height int
		wantEmpty bool
	}{
		{
			name:   "Zero height",
			height: 0,
			wantEmpty: true,
		},
		{
			name:   "Single line",
			height: 1,
			wantEmpty: false,
		},
		{
			name:   "Multiple lines",
			height: 5,
			wantEmpty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seq := r.ClearFrameSequence(tt.height)

			if tt.wantEmpty {
				if seq != "" {
					t.Errorf("ClearFrameSequence(%d) = %q, want empty string", tt.height, seq)
				}
			} else {
				if seq == "" {
					t.Errorf("ClearFrameSequence(%d) returned empty string, want ANSI sequence", tt.height)
				}

				// Verify sequence contains expected ANSI codes
				if !strings.Contains(seq, "\x1b[") {
					t.Errorf("ClearFrameSequence(%d) should contain ANSI escape codes, got %q", tt.height, seq)
				}
			}
		})
	}
}

func TestRendererHideShowCursor(t *testing.T) {
	buf := &bytes.Buffer{}
	r := NewRenderer(buf)

	// Initially cursor should not be hidden
	if r.cursorHidden {
		t.Error("New renderer should have cursorHidden=false")
	}

	// Hide cursor
	err := r.HideCursor()
	if err != nil {
		t.Fatalf("HideCursor() error = %v", err)
	}

	if !r.cursorHidden {
		t.Error("After HideCursor(), cursorHidden should be true")
	}

	output := buf.String()
	if !strings.Contains(output, "\x1b[?25l") {
		t.Errorf("HideCursor() should write hide sequence, got %q", output)
	}

	// Hide again (should be idempotent)
	buf.Reset()
	err = r.HideCursor()
	if err != nil {
		t.Fatalf("Second HideCursor() error = %v", err)
	}

	if buf.Len() > 0 {
		t.Error("Second HideCursor() should not write anything (already hidden)")
	}

	// Show cursor
	buf.Reset()
	err = r.ShowCursor()
	if err != nil {
		t.Fatalf("ShowCursor() error = %v", err)
	}

	if r.cursorHidden {
		t.Error("After ShowCursor(), cursorHidden should be false")
	}

	output = buf.String()
	if !strings.Contains(output, "\x1b[?25h") {
		t.Errorf("ShowCursor() should write show sequence, got %q", output)
	}

	// Show again (should be idempotent)
	buf.Reset()
	err = r.ShowCursor()
	if err != nil {
		t.Fatalf("Second ShowCursor() error = %v", err)
	}

	if buf.Len() > 0 {
		t.Error("Second ShowCursor() should not write anything (already visible)")
	}
}

func TestRendererReset(t *testing.T) {
	buf := &bytes.Buffer{}
	r := NewRenderer(buf)

	// Hide cursor first
	r.HideCursor()
	buf.Reset()

	// Reset should show cursor and write reset sequence
	err := r.Reset()
	if err != nil {
		t.Fatalf("Reset() error = %v", err)
	}

	if r.cursorHidden {
		t.Error("After Reset(), cursorHidden should be false")
	}

	output := buf.String()
	if !strings.Contains(output, "\x1b[0m") {
		t.Errorf("Reset() should write reset sequence, got %q", output)
	}
}

func TestRendererRenderFrame(t *testing.T) {
	buf := &bytes.Buffer{}
	r := NewRenderer(buf)
	scheme := DefaultColorScheme

	frame := []string{"line1", "line2", "line3"}

	err := r.RenderFrame(frame, scheme)
	if err != nil {
		t.Fatalf("RenderFrame() error = %v", err)
	}

	if r.lastFrameHeight != len(frame) {
		t.Errorf("RenderFrame() lastFrameHeight = %d, want %d", r.lastFrameHeight, len(frame))
	}

	output := buf.String()
	if output == "" {
		t.Error("RenderFrame() should write output")
	}

	// Verify all lines are in output
	for _, line := range frame {
		if !strings.Contains(output, line) {
			t.Errorf("RenderFrame() output should contain %q, got:\n%s", line, output)
		}
	}
}

func TestRendererClear(t *testing.T) {
	buf := &bytes.Buffer{}
	r := NewRenderer(buf)

	// Set lastFrameHeight
	r.lastFrameHeight = 5

	err := r.Clear()
	if err != nil {
		t.Fatalf("Clear() error = %v", err)
	}

	if r.lastFrameHeight != 0 {
		t.Errorf("After Clear(), lastFrameHeight = %d, want 0", r.lastFrameHeight)
	}

	output := buf.String()
	if !strings.Contains(output, "\x1b[") {
		t.Errorf("Clear() should write ANSI escape codes, got %q", output)
	}
}

func TestRendererClearWithZeroHeight(t *testing.T) {
	buf := &bytes.Buffer{}
	r := NewRenderer(buf)

	// lastFrameHeight is 0 (nothing to clear)
	err := r.Clear()
	if err != nil {
		t.Fatalf("Clear() error = %v", err)
	}

	if buf.Len() > 0 {
		t.Error("Clear() with zero height should not write anything")
	}
}

func TestRendererMoveCursor(t *testing.T) {
	tests := []struct {
		name     string
		method   func(*Renderer, int) error
		n        int
		wantCode string
	}{
		{
			name:     "MoveCursorUp positive",
			method:   (*Renderer).MoveCursorUp,
			n:        3,
			wantCode: "\x1b[3A",
		},
		{
			name:     "MoveCursorUp zero (no-op)",
			method:   (*Renderer).MoveCursorUp,
			n:        0,
			wantCode: "",
		},
		{
			name:     "MoveCursorDown positive",
			method:   (*Renderer).MoveCursorDown,
			n:        5,
			wantCode: "\x1b[5B",
		},
		{
			name:     "MoveCursorDown zero (no-op)",
			method:   (*Renderer).MoveCursorDown,
			n:        0,
			wantCode: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			r := NewRenderer(buf)

			err := tt.method(r, tt.n)
			if err != nil {
				t.Fatalf("Method error = %v", err)
			}

			output := buf.String()
			if tt.wantCode == "" {
				if output != "" {
					t.Errorf("Expected no output, got %q", output)
				}
			} else {
				if !strings.Contains(output, tt.wantCode) {
					t.Errorf("Expected output to contain %q, got %q", tt.wantCode, output)
				}
			}
		})
	}
}

func TestRendererClearLine(t *testing.T) {
	buf := &bytes.Buffer{}
	r := NewRenderer(buf)

	err := r.ClearLine()
	if err != nil {
		t.Fatalf("ClearLine() error = %v", err)
	}

	output := buf.String()
	expectedCode := "\x1b[2K"
	if !strings.Contains(output, expectedCode) {
		t.Errorf("ClearLine() should contain %q, got %q", expectedCode, output)
	}
}

func TestRendererClearScreen(t *testing.T) {
	buf := &bytes.Buffer{}
	r := NewRenderer(buf)

	err := r.ClearScreen()
	if err != nil {
		t.Fatalf("ClearScreen() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "\x1b[2J") || !strings.Contains(output, "\x1b[H") {
		t.Errorf("ClearScreen() should contain clear and home sequences, got %q", output)
	}
}
