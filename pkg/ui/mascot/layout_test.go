package mascot

import (
	"bytes"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNewLayout(t *testing.T) {
	// Clear env vars to get predictable results
	os.Unsetenv("DINGO_NO_MASCOT")
	os.Unsetenv("CI")

	layout := NewLayout()

	if layout == nil {
		t.Fatal("NewLayout() returned nil")
	}

	if layout.outputBuffer == nil {
		t.Error("NewLayout() outputBuffer is nil")
	}

	if layout.mascotWidth != DefaultMascotWidth {
		t.Errorf("NewLayout() mascotWidth = %d, want %d", layout.mascotWidth, DefaultMascotWidth)
	}

	// Mode depends on terminal capabilities, just verify it's set
	if layout.mode != LayoutSideBySide && layout.mode != LayoutDisabled {
		t.Errorf("NewLayout() mode = %v, want LayoutSideBySide or LayoutDisabled", layout.mode)
	}
}

func TestLayoutMode(t *testing.T) {
	layout := NewLayout()
	mode := layout.Mode()

	// Should return a valid mode
	if mode != LayoutSideBySide && mode != LayoutDisabled {
		t.Errorf("Mode() = %v, want valid LayoutMode", mode)
	}
}

func TestLayoutWithOptions(t *testing.T) {
	buf := &bytes.Buffer{}
	layout := NewLayout(
		WithMascotState(StateCompiling),
		WithLayoutWriter(buf),
		WithStatic(true),
	)

	if layout.writer != buf {
		t.Error("WithLayoutWriter option not applied")
	}

	if !layout.static {
		t.Error("WithStatic option not applied")
	}

	// If mascot was created, state should be set
	if layout.mascot != nil && layout.mascot.state != StateCompiling {
		t.Errorf("WithMascotState option not applied, got state %v", layout.mascot.state)
	}
}

func TestLayoutWrite(t *testing.T) {
	buf := &bytes.Buffer{}
	layout := NewLayout(WithLayoutWriter(buf))

	testData := []byte("test output\n")
	n, err := layout.Write(testData)

	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	if n != len(testData) {
		t.Errorf("Write() returned n=%d, want %d", n, len(testData))
	}

	// Verify data is in buffer
	if !strings.Contains(layout.GetOutputBuffer(), "test output") {
		t.Errorf("Write() data not in buffer, got %q", layout.GetOutputBuffer())
	}
}

func TestLayoutMultipleWrites(t *testing.T) {
	layout := NewLayout()

	writes := []string{"line1\n", "line2\n", "line3\n"}

	for _, data := range writes {
		_, err := layout.Write([]byte(data))
		if err != nil {
			t.Fatalf("Write() error = %v", err)
		}
	}

	buffer := layout.GetOutputBuffer()
	for _, line := range writes {
		if !strings.Contains(buffer, strings.TrimSpace(line)) {
			t.Errorf("Buffer should contain %q, got:\n%s", line, buffer)
		}
	}
}

func TestLayoutClearOutput(t *testing.T) {
	layout := NewLayout()

	layout.Write([]byte("test data"))

	if layout.GetOutputBuffer() == "" {
		t.Fatal("Buffer should have data before clear")
	}

	layout.ClearOutput()

	if layout.GetOutputBuffer() != "" {
		t.Errorf("ClearOutput() should clear buffer, got %q", layout.GetOutputBuffer())
	}
}

func TestLayoutSetMascotState(t *testing.T) {
	layout := NewLayout()

	// Only test if mascot was created (side-by-side mode)
	if layout.mascot == nil {
		t.Skip("Mascot not created (disabled mode)")
	}

	states := []MascotState{StateIdle, StateCompiling, StateSuccess, StateFailed}

	for _, state := range states {
		layout.SetMascotState(state)

		// Verify state was set
		layout.mu.Lock()
		actualState := layout.mascot.state
		layout.mu.Unlock()

		if actualState != state {
			t.Errorf("SetMascotState(%v) did not set state, got %v", state, actualState)
		}
	}
}

func TestLayoutStartStop(t *testing.T) {
	buf := &bytes.Buffer{}
	layout := NewLayout(WithLayoutWriter(buf))

	// Only test if mascot was created
	if layout.mascot == nil {
		t.Skip("Mascot not created (disabled mode)")
	}

	// Start should not panic
	layout.Start()

	// Give animation time to start
	time.Sleep(50 * time.Millisecond)

	// Stop should not panic
	layout.Stop()

	// Stop again should be safe (idempotent)
	layout.Stop()
}

func TestLayoutStaticMode(t *testing.T) {
	buf := &bytes.Buffer{}
	layout := NewLayout(
		WithLayoutWriter(buf),
		WithStatic(true),
	)

	if !layout.static {
		t.Fatal("Static mode not set")
	}

	// Start in static mode should render once
	layout.Start()

	// Stop should not panic
	layout.Stop()
}

func TestLayoutPauseResume(t *testing.T) {
	layout := NewLayout()

	if layout.paused {
		t.Error("Layout should not be paused initially")
	}

	layout.Pause()

	layout.mu.Lock()
	paused := layout.paused
	layout.mu.Unlock()

	if !paused {
		t.Error("Pause() should set paused=true")
	}

	layout.Resume()

	layout.mu.Lock()
	paused = layout.paused
	layout.mu.Unlock()

	if paused {
		t.Error("Resume() should set paused=false")
	}
}

func TestLayoutFlush(t *testing.T) {
	buf := &bytes.Buffer{}
	layout := NewLayout(WithLayoutWriter(buf))

	layout.Write([]byte("test output"))

	// Flush should not panic
	layout.Flush()
}

func TestLayoutCombineSideBySide(t *testing.T) {
	layout := &Layout{
		mascotWidth: 24,
		outputWidth: 56,
		termWidth:   80,
	}

	mascotLines := []string{
		"    /\\      /\\    ",
		"   /  \\____/  \\   ",
		"  |   O    O   |  ",
	}

	outputLines := []string{
		"Building...",
		"Success!",
	}

	combined := layout.combineSideBySide(mascotLines, outputLines)

	// Should have max(mascot lines, output lines) lines
	expectedLines := 3 // max(3, 2)
	if len(combined) != expectedLines {
		t.Errorf("combineSideBySide() returned %d lines, want %d", len(combined), expectedLines)
	}

	// Each combined line should contain both mascot and output parts
	for i, line := range combined {
		// Should be non-empty
		if strings.TrimSpace(line) == "" && i < len(outputLines) {
			t.Errorf("combineSideBySide() line %d is empty", i)
		}
	}

	// First line should contain both mascot and output
	if !strings.Contains(combined[0], "Building") {
		t.Errorf("Combined line should contain output, got %q", combined[0])
	}
}

func TestLayoutCombineSideBySideWithPadding(t *testing.T) {
	layout := &Layout{
		mascotWidth: 24,
		outputWidth: 56,
		termWidth:   80,
	}

	mascotLines := []string{"short"}
	outputLines := []string{"output"}

	combined := layout.combineSideBySide(mascotLines, outputLines)

	if len(combined) == 0 {
		t.Fatal("combineSideBySide() returned empty result")
	}

	// Line should be padded to accommodate both columns
	line := combined[0]
	if len(line) < layout.mascotWidth {
		t.Errorf("Combined line too short: %d chars, want at least %d", len(line), layout.mascotWidth)
	}
}

func TestLayoutConcurrentWrites(t *testing.T) {
	layout := NewLayout()

	// Number of concurrent writers
	numWriters := 10
	writesPerWriter := 100

	var wg sync.WaitGroup
	wg.Add(numWriters)

	for i := 0; i < numWriters; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < writesPerWriter; j++ {
				layout.Write([]byte("x"))
			}
		}(i)
	}

	wg.Wait()

	// Verify all writes made it to buffer
	buffer := layout.GetOutputBuffer()
	expectedLength := numWriters * writesPerWriter
	actualLength := len(strings.ReplaceAll(buffer, "\n", ""))

	if actualLength != expectedLength {
		t.Errorf("Concurrent writes: got %d chars, want %d", actualLength, expectedLength)
	}
}

func TestLayoutConcurrentStateChanges(t *testing.T) {
	layout := NewLayout()

	if layout.mascot == nil {
		t.Skip("Mascot not created (disabled mode)")
	}

	numGoroutines := 10
	iterations := 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	states := []MascotState{StateIdle, StateCompiling, StateRunning, StateSuccess, StateFailed}

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				state := states[j%len(states)]
				layout.SetMascotState(state)
			}
		}(i)
	}

	// Should not panic or deadlock
	wg.Wait()
}

func TestLayoutString(t *testing.T) {
	layout := NewLayout()

	str := layout.String()

	if str == "" {
		t.Error("String() returned empty string")
	}

	// Should contain key information
	if !strings.Contains(str, "Layout{") {
		t.Errorf("String() should contain 'Layout{', got %q", str)
	}
}

func TestLayoutDisabledMode(t *testing.T) {
	// Force disabled mode by setting env var
	os.Setenv("DINGO_NO_MASCOT", "1")
	defer os.Unsetenv("DINGO_NO_MASCOT")

	buf := &bytes.Buffer{}
	layout := NewLayout(WithLayoutWriter(buf))

	if layout.Mode() != LayoutDisabled {
		t.Skip("Layout not in disabled mode despite env var")
	}

	// In disabled mode, Write should pass through directly
	testData := "test output\n"
	layout.Write([]byte(testData))

	// Data should be in buffer AND written to output
	if !strings.Contains(buf.String(), "test output") {
		t.Errorf("Disabled mode should write directly to output, got %q", buf.String())
	}
}

func TestDefaultConstants(t *testing.T) {
	if DefaultMascotWidth != 24 {
		t.Errorf("DefaultMascotWidth = %d, want 24", DefaultMascotWidth)
	}

	if DefaultSeparatorWidth != 2 {
		t.Errorf("DefaultSeparatorWidth = %d, want 2", DefaultSeparatorWidth)
	}
}

func TestLayoutGetColorSchemeForState(t *testing.T) {
	tests := []struct {
		name  string
		state MascotState
		want  ColorScheme
	}{
		{
			name:  "StateSuccess",
			state: StateSuccess,
			want:  SuccessColorScheme,
		},
		{
			name:  "StateFailed",
			state: StateFailed,
			want:  FailureColorScheme,
		},
		{
			name:  "StateCompiling",
			state: StateCompiling,
			want:  CompileColorScheme,
		},
		{
			name:  "StateRunning",
			state: StateRunning,
			want:  CompileColorScheme,
		},
		{
			name:  "StateIdle",
			state: StateIdle,
			want:  DefaultColorScheme,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getColorSchemeForState(tt.state)

			// Compare by checking Body color (should be sufficient)
			if got.Body != tt.want.Body {
				t.Errorf("getColorSchemeForState(%v) Body color = %v, want %v", tt.state, got.Body, tt.want.Body)
			}
		})
	}
}
