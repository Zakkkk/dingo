package mascot

import (
	"strings"
	"testing"
)

func TestComposeFrame(t *testing.T) {
	tests := []struct {
		name         string
		config       FrameConfig
		wantLines    int
		wantContains string
	}{
		{
			name:         "Empty config",
			config:       FrameConfig{},
			wantLines:    1, // ComposeFrame adds chin even if empty
			wantContains: "",
		},
		{
			name: "Basic config with ears and eyes",
			config: FrameConfig{
				Ears: []string{"  /\\  /\\  "},
				Eyes: "  O  O  ",
			},
			wantLines:    2,
			wantContains: "O  O",
		},
		{
			name: "Config with badge",
			config: FrameConfig{
				Eyes:      "  O  O  ",
				EyesBadge: "✓",
			},
			wantLines:    1,
			wantContains: "✓",
		},
		{
			name: "Config with vertical offset",
			config: FrameConfig{
				Eyes:    "  O  O  ",
				OffsetY: 2,
			},
			wantLines:    3, // 2 empty + 1 eyes
			wantContains: "O  O",
		},
		{
			name: "Config with horizontal offset",
			config: FrameConfig{
				Eyes:    "O  O",
				OffsetX: 4,
			},
			wantLines:    1,
			wantContains: "    O  O", // 4 spaces + eyes
		},
		{
			name: "Full config",
			config: FrameConfig{
				Above:  "  ✧ ★ ✧  ",
				Ears:   []string{"  /\\  /\\  "},
				Head:   []string{" |     | "},
				Eyes:   "  O  O  ",
				Snout:  []string{"   ^   "},
				Chin:   "  \\_/  ",
				Body:   []string{" |   | ", " |___| "},
				Ground: "========",
			},
			wantLines:    9, // Above(1) + Ears(1) + Head(1) + Eyes(1) + Snout(1) + Chin(1) + Body(2) + Ground(1) = 9
			wantContains: "O  O",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frame := ComposeFrame(tt.config)

			if len(frame) != tt.wantLines {
				t.Errorf("ComposeFrame() returned %d lines, want %d", len(frame), tt.wantLines)
			}

			if tt.wantContains != "" {
				found := false
				for _, line := range frame {
					if strings.Contains(line, tt.wantContains) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("ComposeFrame() frame does not contain %q\nGot:\n%s",
						tt.wantContains, strings.Join(frame, "\n"))
				}
			}
		})
	}
}

func TestPrecomposedFrames(t *testing.T) {
	tests := []struct {
		name     string
		frame    []string
		minLines int
		maxLines int
	}{
		{
			name:     "FrameNeutral",
			frame:    FrameNeutral,
			minLines: 7,
			maxLines: 12,
		},
		{
			name:     "FrameHappy",
			frame:    FrameHappy,
			minLines: 7,
			maxLines: 12,
		},
		{
			name:     "FrameSad",
			frame:    FrameSad,
			minLines: 7,
			maxLines: 12,
		},
		{
			name:     "FrameAlert",
			frame:    FrameAlert,
			minLines: 7,
			maxLines: 12,
		},
		{
			name:     "FrameCompiling",
			frame:    FrameCompiling,
			minLines: 7,
			maxLines: 12,
		},
		{
			name:     "FrameBuildSuccess",
			frame:    FrameBuildSuccess,
			minLines: 7,
			maxLines: 12,
		},
		{
			name:     "FrameBuildFailed",
			frame:    FrameBuildFailed,
			minLines: 7,
			maxLines: 12,
		},
		{
			name:     "FrameLoading1",
			frame:    FrameLoading1,
			minLines: 7,
			maxLines: 12,
		},
		{
			name:     "FrameLoading2",
			frame:    FrameLoading2,
			minLines: 7,
			maxLines: 12,
		},
		{
			name:     "FrameLoading3",
			frame:    FrameLoading3,
			minLines: 7,
			maxLines: 12,
		},
		{
			name:     "FrameLoading4",
			frame:    FrameLoading4,
			minLines: 7,
			maxLines: 12,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if len(tt.frame) < tt.minLines {
				t.Errorf("%s has %d lines, expected at least %d", tt.name, len(tt.frame), tt.minLines)
			}
			if len(tt.frame) > tt.maxLines {
				t.Errorf("%s has %d lines, expected at most %d", tt.name, len(tt.frame), tt.maxLines)
			}

			// Check that frame is not empty
			if len(tt.frame) == 0 {
				t.Errorf("%s is empty", tt.name)
			}

			// Check that at least one line has content
			hasContent := false
			for _, line := range tt.frame {
				if strings.TrimSpace(line) != "" {
					hasContent = true
					break
				}
			}
			if !hasContent {
				t.Errorf("%s has no content", tt.name)
			}
		})
	}
}

func TestFrameLoadingSequence(t *testing.T) {
	// Test that loading frames exist and are different
	frames := [][]string{FrameLoading1, FrameLoading2, FrameLoading3, FrameLoading4}

	for i, frame := range frames {
		if len(frame) == 0 {
			t.Errorf("FrameLoading%d is empty", i+1)
		}
	}

	// Verify frames are different (at least one line differs between frames)
	for i := 0; i < len(frames)-1; i++ {
		if areFramesIdentical(frames[i], frames[i+1]) {
			t.Errorf("FrameLoading%d and FrameLoading%d are identical, should be different", i+1, i+2)
		}
	}
}

func TestFrameWithBadge(t *testing.T) {
	// Test that frames with badges include the badge icon
	badgeFrames := map[string]struct {
		frame []string
		badge string
	}{
		"FrameCompiling":    {FrameCompiling, IconGear},
		"FrameBuildSuccess": {FrameBuildSuccess, IconCheck},
		"FrameBuildFailed":  {FrameBuildFailed, IconCross},
	}

	for name, test := range badgeFrames {
		t.Run(name, func(t *testing.T) {
			frameText := strings.Join(test.frame, "\n")
			if !strings.Contains(frameText, test.badge) {
				t.Errorf("%s should contain badge %q but doesn't\nFrame:\n%s",
					name, test.badge, frameText)
			}
		})
	}
}

func TestAnimationConfigs(t *testing.T) {
	tests := []struct {
		name        string
		anim        *AnimationConfig
		wantFrames  int
		wantDelayMs int
		wantLoop    bool
	}{
		{
			name:        "AnimIdle",
			anim:        &AnimIdle,
			wantFrames:  4,
			wantDelayMs: 1000,
			wantLoop:    true,
		},
		{
			name:        "AnimLoading",
			anim:        &AnimLoading,
			wantFrames:  4,
			wantDelayMs: 150,
			wantLoop:    true,
		},
		{
			name:        "AnimRunning",
			anim:        &AnimRunning,
			wantFrames:  3,
			wantDelayMs: 100,
			wantLoop:    true,
		},
		{
			name:        "AnimCelebrate",
			anim:        &AnimCelebrate,
			wantFrames:  4,
			wantDelayMs: 200,
			wantLoop:    false,
		},
		{
			name:        "AnimTailWag",
			anim:        &AnimTailWag,
			wantFrames:  4,
			wantDelayMs: 150,
			wantLoop:    true,
		},
		{
			name:        "AnimThinking",
			anim:        &AnimThinking,
			wantFrames:  5,
			wantDelayMs: 500,
			wantLoop:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if len(tt.anim.Frames) != tt.wantFrames {
				t.Errorf("%s has %d frames, want %d", tt.name, len(tt.anim.Frames), tt.wantFrames)
			}

			if tt.anim.FrameDelayMs != tt.wantDelayMs {
				t.Errorf("%s has FrameDelayMs=%d, want %d", tt.name, tt.anim.FrameDelayMs, tt.wantDelayMs)
			}

			if tt.anim.Loop != tt.wantLoop {
				t.Errorf("%s has Loop=%v, want %v", tt.name, tt.anim.Loop, tt.wantLoop)
			}

			// Verify all frames have content
			for i, frame := range tt.anim.Frames {
				if len(frame) == 0 {
					t.Errorf("%s frame %d is empty", tt.name, i)
				}
			}
		})
	}
}

func TestGetAnimation(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantAnim *AnimationConfig
	}{
		{
			name:     "idle lowercase",
			input:    "idle",
			wantAnim: &AnimIdle,
		},
		{
			name:     "idle uppercase",
			input:    "IDLE",
			wantAnim: &AnimIdle,
		},
		{
			name:     "idle mixed case",
			input:    "Idle",
			wantAnim: &AnimIdle,
		},
		{
			name:     "loading",
			input:    "loading",
			wantAnim: &AnimLoading,
		},
		{
			name:     "running",
			input:    "running",
			wantAnim: &AnimRunning,
		},
		{
			name:     "celebrate",
			input:    "celebrate",
			wantAnim: &AnimCelebrate,
		},
		{
			name:     "invalid name defaults to idle",
			input:    "nonexistent",
			wantAnim: &AnimIdle,
		},
		{
			name:     "empty string defaults to idle",
			input:    "",
			wantAnim: &AnimIdle,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetAnimation(tt.input)

			// Compare by checking frame count and delay
			if len(got.Frames) != len(tt.wantAnim.Frames) {
				t.Errorf("GetAnimation(%q) returned animation with %d frames, want %d",
					tt.input, len(got.Frames), len(tt.wantAnim.Frames))
			}

			if got.FrameDelayMs != tt.wantAnim.FrameDelayMs {
				t.Errorf("GetAnimation(%q) returned animation with FrameDelayMs=%d, want %d",
					tt.input, got.FrameDelayMs, tt.wantAnim.FrameDelayMs)
			}
		})
	}
}

// Helper function to compare two frames
func areFramesIdentical(frame1, frame2 []string) bool {
	if len(frame1) != len(frame2) {
		return false
	}
	for i := range frame1 {
		if frame1[i] != frame2[i] {
			return false
		}
	}
	return true
}
