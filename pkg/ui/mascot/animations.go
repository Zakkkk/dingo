package mascot

import "strings"

// AnimationConfig defines a sequence of frames and playback settings
type AnimationConfig struct {
	Frames       [][]string // Array of frame line arrays to cycle through
	FrameDelayMs int        // Delay between frames in milliseconds
	Loop         bool       // Whether to loop continuously
}

// buildFrame constructs a complete mascot frame from component parts
func buildFrame(ears, head, eyes, snout, chin string, body []string) []string {
	frame := make([]string, 0, 10)

	// Add ears
	frame = append(frame, ears)

	// Add head
	frame = append(frame, head)

	// Add eyes
	frame = append(frame, eyes)

	// Add snout
	frame = append(frame, snout)

	// Add chin
	frame = append(frame, chin)

	// Add body (multiple lines)
	frame = append(frame, body...)

	return frame
}

// Pre-defined animations

// AnimIdle - Occasional blink while standing
var AnimIdle = AnimationConfig{
	Frames: [][]string{
		buildFrame(
			Ears.NORMAL[0],
			Head.NORMAL[0],
			Eyes.NORMAL,
			Snout.NORMAL[0],
			Chin.NORMAL,
			Body.NORMAL,
		),
		buildFrame(
			Ears.NORMAL[0],
			Head.NORMAL[0],
			Eyes.NORMAL,
			Snout.NORMAL[0],
			Chin.NORMAL,
			Body.NORMAL,
		),
		buildFrame(
			Ears.NORMAL[0],
			Head.NORMAL[0],
			Eyes.NORMAL,
			Snout.NORMAL[0],
			Chin.NORMAL,
			Body.NORMAL,
		),
		buildFrame(
			Ears.NORMAL[0],
			Head.NORMAL[0],
			Eyes.CLOSED,
			Snout.NORMAL[0],
			Chin.NORMAL,
			Body.NORMAL,
		),
	},
	FrameDelayMs: 1000,
	Loop:         true,
}

// AnimThinking - Looking around
var AnimThinking = AnimationConfig{
	Frames: [][]string{
		buildFrame(
			Ears.NORMAL[0],
			Head.NORMAL[0],
			Eyes.LOOK_L,
			Snout.NORMAL[0],
			Chin.NORMAL,
			Body.NORMAL,
		),
		buildFrame(
			Ears.NORMAL[0],
			Head.NORMAL[0],
			Eyes.LOOK_UP_L,
			Snout.NORMAL[0],
			Chin.NORMAL,
			Body.NORMAL,
		),
		buildFrame(
			Ears.NORMAL[0],
			Head.NORMAL[0],
			Eyes.LOOK_UP,
			Snout.NORMAL[0],
			Chin.NORMAL,
			Body.NORMAL,
		),
		buildFrame(
			Ears.NORMAL[0],
			Head.NORMAL[0],
			Eyes.LOOK_UP_R,
			Snout.NORMAL[0],
			Chin.NORMAL,
			Body.NORMAL,
		),
		buildFrame(
			Ears.NORMAL[0],
			Head.NORMAL[0],
			Eyes.LOOK_R,
			Snout.NORMAL[0],
			Chin.NORMAL,
			Body.NORMAL,
		),
	},
	FrameDelayMs: 500,
	Loop:         true,
}

// AnimLoading - Spinner eyes
var AnimLoading = AnimationConfig{
	Frames: [][]string{
		buildFrame(
			Ears.ALERT_BOTH[0],
			Head.NORMAL[0],
			Eyes.LOADING_1,
			Snout.NORMAL[0],
			Chin.NORMAL,
			Body.NORMAL,
		),
		buildFrame(
			Ears.ALERT_BOTH[0],
			Head.NORMAL[0],
			Eyes.LOADING_2,
			Snout.NORMAL[0],
			Chin.NORMAL,
			Body.NORMAL,
		),
		buildFrame(
			Ears.ALERT_BOTH[0],
			Head.NORMAL[0],
			Eyes.LOADING_3,
			Snout.NORMAL[0],
			Chin.NORMAL,
			Body.NORMAL,
		),
		buildFrame(
			Ears.ALERT_BOTH[0],
			Head.NORMAL[0],
			Eyes.LOADING_4,
			Snout.NORMAL[0],
			Chin.NORMAL,
			Body.NORMAL,
		),
	},
	FrameDelayMs: 150,
	Loop:         true,
}

// AnimTailWag - Happy tail movement
var AnimTailWag = AnimationConfig{
	Frames: [][]string{
		buildFrame(
			Ears.NORMAL[0],
			Head.NORMAL[0],
			Eyes.HAPPY,
			Snout.NORMAL[0],
			Chin.WITH_TAIL_UP,
			Body.NORMAL,
		),
		buildFrame(
			Ears.NORMAL[0],
			Head.NORMAL[0],
			Eyes.HAPPY,
			Snout.NORMAL[0],
			Chin.WITH_TAIL_MID,
			Body.NORMAL,
		),
		buildFrame(
			Ears.NORMAL[0],
			Head.NORMAL[0],
			Eyes.HAPPY,
			Snout.NORMAL[0],
			Chin.WITH_TAIL_DOWN,
			Body.NORMAL,
		),
		buildFrame(
			Ears.NORMAL[0],
			Head.NORMAL[0],
			Eyes.HAPPY,
			Snout.NORMAL[0],
			Chin.WITH_TAIL_MID,
			Body.NORMAL,
		),
	},
	FrameDelayMs: 150,
	Loop:         true,
}

// AnimRunning - Run cycle
var AnimRunning = AnimationConfig{
	Frames: [][]string{
		buildFrame(
			Ears.ALERT_R[0],
			Head.NORMAL[0],
			Eyes.WIDE,
			Snout.NORMAL[0],
			Chin.NORMAL,
			Body.RUNNING_1,
		),
		buildFrame(
			Ears.ALERT_BOTH[0],
			Head.NORMAL[0],
			Eyes.WIDE,
			Snout.NORMAL[0],
			Chin.NORMAL,
			Body.RUNNING_2,
		),
		buildFrame(
			Ears.ALERT_L[0],
			Head.NORMAL[0],
			Eyes.WIDE,
			Snout.NORMAL[0],
			Chin.NORMAL,
			Body.RUNNING_3,
		),
	},
	FrameDelayMs: 100,
	Loop:         true,
}

// AnimJumping - Jump sequence
var AnimJumping = AnimationConfig{
	Frames: [][]string{
		buildFrame(
			Ears.ALERT_BOTH[0],
			Head.NORMAL[0],
			Eyes.WIDE,
			Snout.NORMAL[0],
			Chin.NORMAL,
			Body.CROUCH,
		),
		buildFrame(
			Ears.ALERT_BOTH[0],
			Head.NORMAL[0],
			Eyes.WIDE,
			Snout.NORMAL[0],
			Chin.NORMAL,
			Body.JUMPING,
		),
		buildFrame(
			Ears.ALERT_BOTH[0],
			Head.NORMAL[0],
			Eyes.CLOSED,
			Snout.NORMAL[0],
			Chin.NORMAL,
			Body.JUMPING_TUCKED,
		),
		buildFrame(
			Ears.ALERT_BOTH[0],
			Head.NORMAL[0],
			Eyes.WIDE,
			Snout.NORMAL[0],
			Chin.NORMAL,
			Body.JUMPING,
		),
		buildFrame(
			Ears.ALERT_BOTH[0],
			Head.NORMAL[0],
			Eyes.WIDE,
			Snout.NORMAL[0],
			Chin.NORMAL,
			Body.LANDING,
		),
	},
	FrameDelayMs: 100,
	Loop:         false,
}

// AnimSleeping - Zzz animation
var AnimSleeping = AnimationConfig{
	Frames: [][]string{
		append(buildFrame(
			Ears.DROOPY[0],
			Head.NORMAL[0],
			Eyes.CLOSED,
			Snout.NORMAL[0],
			Chin.NORMAL,
			Body.SITTING,
		), "          z            "),
		append(buildFrame(
			Ears.DROOPY[0],
			Head.NORMAL[0],
			Eyes.CLOSED,
			Snout.NORMAL[0],
			Chin.NORMAL,
			Body.SITTING,
		), "         Z z           "),
		append(buildFrame(
			Ears.DROOPY[0],
			Head.NORMAL[0],
			Eyes.CLOSED,
			Snout.NORMAL[0],
			Chin.NORMAL,
			Body.SITTING,
		), "        Z Z z          "),
	},
	FrameDelayMs: 800,
	Loop:         true,
}

// AnimSearching - Sniffing around
var AnimSearching = AnimationConfig{
	Frames: [][]string{
		buildFrame(
			Ears.ALERT_L[0],
			Head.NORMAL[0],
			Eyes.LOOK_L,
			Snout.SNIFF_L[0],
			Chin.NORMAL,
			Body.NORMAL,
		),
		buildFrame(
			Ears.NORMAL[0],
			Head.NORMAL[0],
			Eyes.NORMAL,
			Snout.NORMAL[0],
			Chin.NORMAL,
			Body.NORMAL,
		),
		buildFrame(
			Ears.ALERT_R[0],
			Head.NORMAL[0],
			Eyes.LOOK_R,
			Snout.SNIFF_R[0],
			Chin.NORMAL,
			Body.NORMAL,
		),
		buildFrame(
			Ears.NORMAL[0],
			Head.NORMAL[0],
			Eyes.NORMAL,
			Snout.NORMAL[0],
			Chin.NORMAL,
			Body.NORMAL,
		),
	},
	FrameDelayMs: 300,
	Loop:         true,
}

// AnimCelebrate - Victory dance
var AnimCelebrate = AnimationConfig{
	Frames: [][]string{
		buildFrame(
			Ears.ALERT_BOTH[0],
			Head.NORMAL[0],
			Eyes.HAPPY,
			Snout.NORMAL[0],
			Chin.HAPPY,
			Body.ARMS_UP,
		),
		buildFrame(
			Ears.ALERT_BOTH[0],
			Head.NORMAL[0],
			Eyes.STARS,
			Snout.NORMAL[0],
			Chin.HAPPY,
			Body.JUMPING,
		),
		buildFrame(
			Ears.ALERT_BOTH[0],
			Head.NORMAL[0],
			Eyes.HAPPY,
			Snout.NORMAL[0],
			Chin.WITH_TAIL_UP,
			Body.ARMS_UP,
		),
		buildFrame(
			Ears.ALERT_BOTH[0],
			Head.NORMAL[0],
			Eyes.STARS,
			Snout.NORMAL[0],
			Chin.WITH_TAIL_MID,
			Body.STRETCH,
		),
	},
	FrameDelayMs: 200,
	Loop:         false,
}

// AnimTyping - Dot dot dot
var AnimTyping = AnimationConfig{
	Frames: [][]string{
		append(buildFrame(
			Ears.NORMAL[0],
			Head.NORMAL[0],
			Eyes.LOOK_L,
			Snout.NORMAL[0],
			Chin.NORMAL,
			Body.NORMAL,
		), "      .                "),
		append(buildFrame(
			Ears.NORMAL[0],
			Head.NORMAL[0],
			Eyes.LOOK_L,
			Snout.NORMAL[0],
			Chin.NORMAL,
			Body.NORMAL,
		), "      . .              "),
		append(buildFrame(
			Ears.NORMAL[0],
			Head.NORMAL[0],
			Eyes.LOOK_L,
			Snout.NORMAL[0],
			Chin.NORMAL,
			Body.NORMAL,
		), "      . . .            "),
	},
	FrameDelayMs: 400,
	Loop:         true,
}

// GetAnimation retrieves an animation by name for convenience
func GetAnimation(name string) *AnimationConfig {
	animations := map[string]*AnimationConfig{
		"idle":      &AnimIdle,
		"thinking":  &AnimThinking,
		"loading":   &AnimLoading,
		"tailwag":   &AnimTailWag,
		"running":   &AnimRunning,
		"jumping":   &AnimJumping,
		"sleeping":  &AnimSleeping,
		"searching": &AnimSearching,
		"celebrate": &AnimCelebrate,
		"typing":    &AnimTyping,
	}

	if anim, ok := animations[strings.ToLower(name)]; ok {
		return anim
	}

	// Default to idle
	return &AnimIdle
}
