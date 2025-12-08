package mascot

import (
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// MascotState represents the current state of the mascot.
// Each state corresponds to a different animation or static frame.
type MascotState int

const (
	StateIdle      MascotState = iota // Default idle state with occasional blink
	StateCompiling                     // Actively compiling/building (spinner eyes)
	StateRunning                       // Executing a program (running animation)
	StateSuccess                       // Build/operation succeeded (celebrate then static)
	StateFailed                        // Build/operation failed (sad/error pose)
	StateThinking                      // Pondering/analyzing (looking around)
	StateHelp                          // Friendly pose for help/version commands
)

// Mascot manages the animated mascot display with state transitions.
// It runs an animation loop in a goroutine and can be stopped gracefully.
type Mascot struct {
	state        MascotState
	animation    *AnimationConfig
	currentFrame int
	colorScheme  ColorScheme
	writer       io.Writer

	// Animation control
	stopCh  chan struct{}
	doneCh  chan struct{}
	running atomic.Bool

	// Thread safety
	mu sync.Mutex
}

// Option is a functional option for configuring the Mascot.
type Option func(*Mascot)

// WithWriter sets the output writer for the mascot.
// Default is os.Stdout.
func WithWriter(w io.Writer) Option {
	return func(m *Mascot) {
		m.writer = w
	}
}

// WithColorScheme sets the color scheme for the mascot.
// Default is DefaultColorScheme.
func WithColorScheme(cs ColorScheme) Option {
	return func(m *Mascot) {
		m.colorScheme = cs
	}
}

// WithInitialState sets the initial state for the mascot.
// Default is StateIdle.
func WithInitialState(state MascotState) Option {
	return func(m *Mascot) {
		m.state = state
		m.animation = getAnimationForState(state)
	}
}

// New creates a new Mascot instance with the given options.
// The mascot is not started automatically; call Start() to begin animation.
func New(opts ...Option) *Mascot {
	m := &Mascot{
		state:       StateIdle,
		animation:   &AnimIdle,
		colorScheme: DefaultColorScheme,
		writer:      os.Stdout,
		stopCh:      make(chan struct{}),
		doneCh:      make(chan struct{}),
	}

	// Apply options
	for _, opt := range opts {
		opt(m)
	}

	return m
}

// Start begins the animation loop in a goroutine.
// The animation continues until Stop() is called.
func (m *Mascot) Start() {
	if m.running.CompareAndSwap(false, true) {
		go m.animationLoop()
	}
}

// Stop halts the animation and clears the display.
// It blocks until the animation goroutine exits.
func (m *Mascot) Stop() {
	if !m.running.Load() {
		return // Not running, nothing to stop
	}

	// Signal stop
	close(m.stopCh)

	// Wait for animation to finish
	<-m.doneCh

	m.running.Store(false)
}

// SetState transitions to a new state with the appropriate animation.
// This can be called while the animation is running.
func (m *Mascot) SetState(state MascotState) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.state = state
	m.animation = getAnimationForState(state)
	m.currentFrame = 0
}

// Render returns the current frame as a slice of strings (lines).
// This is useful for layout compositors that need to combine mascot with other output.
func (m *Mascot) Render() []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.animation == nil || len(m.animation.Frames) == 0 {
		return FrameNeutral
	}

	// Get current frame from animation
	frame := m.animation.Frames[m.currentFrame]
	return frame
}

// Wait blocks until the animation completes.
// This is only useful for non-looping animations (e.g., celebrate, jump).
// For looping animations, this will block indefinitely.
func (m *Mascot) Wait() {
	m.mu.Lock()
	isLooping := m.animation != nil && m.animation.Loop
	m.mu.Unlock()

	// If looping, return immediately (would wait forever)
	if isLooping {
		return
	}

	// Wait for animation to complete one cycle
	// Calculate total duration
	m.mu.Lock()
	if m.animation == nil {
		m.mu.Unlock()
		return
	}
	frameCount := len(m.animation.Frames)
	delayMs := m.animation.FrameDelayMs
	m.mu.Unlock()

	totalDuration := time.Duration(frameCount*delayMs) * time.Millisecond
	time.Sleep(totalDuration)
}

// animationLoop runs the animation frames in a loop until stopped.
func (m *Mascot) animationLoop() {
	defer close(m.doneCh)

	// Get initial delay
	delay := m.getFrameDelay()
	ticker := time.NewTicker(delay)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return

		case <-ticker.C:
			m.advanceFrame()

			// Check if delay changed (e.g., state transition)
			newDelay := m.getFrameDelay()
			if newDelay != delay {
				ticker.Reset(newDelay)
				delay = newDelay
			}
		}
	}
}

// getFrameDelay returns the current frame delay based on animation config.
func (m *Mascot) getFrameDelay() time.Duration {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.animation == nil || m.animation.FrameDelayMs == 0 {
		return 100 * time.Millisecond // Default
	}
	return time.Duration(m.animation.FrameDelayMs) * time.Millisecond
}

// advanceFrame advances to the next frame in the animation.
func (m *Mascot) advanceFrame() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.animation == nil || len(m.animation.Frames) == 0 {
		return
	}

	// Advance frame
	m.currentFrame++

	// Handle looping
	if m.currentFrame >= len(m.animation.Frames) {
		if m.animation.Loop {
			m.currentFrame = 0
		} else {
			// Non-looping: stay on last frame
			m.currentFrame = len(m.animation.Frames) - 1
		}
	}
}

// getAnimationForState returns the appropriate animation for a given state.
func getAnimationForState(state MascotState) *AnimationConfig {
	switch state {
	case StateIdle:
		return &AnimIdle

	case StateCompiling:
		return &AnimLoading

	case StateRunning:
		return &AnimRunning

	case StateSuccess:
		// Celebrate animation, then transition to static success frame
		return &AnimCelebrate

	case StateFailed:
		// Static error frame (no animation)
		return &AnimationConfig{
			Frames:       [][]string{FrameError},
			FrameDelayMs: 1000,
			Loop:         false,
		}

	case StateThinking:
		return &AnimThinking

	case StateHelp:
		// Static friendly frame (no animation)
		return &AnimationConfig{
			Frames:       [][]string{FrameHappy},
			FrameDelayMs: 1000,
			Loop:         false,
		}

	default:
		return &AnimIdle
	}
}

// GetAnimationFrames returns the frames for the current animation.
// This is useful for external animation rendering (e.g., debug command).
func GetAnimationFrames(m *Mascot) [][]string {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.animation == nil || len(m.animation.Frames) == 0 {
		return nil
	}

	return m.animation.Frames
}
