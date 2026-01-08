package mascot

// frames.go provides pre-composed static frames for each mascot state.
// Each frame is a complete composition of body parts into a multi-line ASCII art representation.

// FrameConfig defines the configuration for composing a mascot frame
type FrameConfig struct {
	Above     string   // Decoration above head (e.g., sparkles, Zzz)
	Ears      []string // Ear lines
	Head      []string // Head shape lines
	Eyes      string   // Eye expression line
	EyesBadge string   // Status icon after eyes (e.g., ✓, ✗, ⚙)
	Snout     []string // Snout/nose lines
	Chin      string   // Chin/jaw line
	Body      []string // Body shape lines
	Ground    string   // Ground line (for standing poses)
	OffsetX   int      // Horizontal offset in spaces
	OffsetY   int      // Vertical offset in lines
}

// ComposeFrame takes a FrameConfig and returns a complete frame as a slice of strings
func ComposeFrame(config FrameConfig) []string {
	var lines []string

	// Add vertical offset (empty lines at top)
	for i := 0; i < config.OffsetY; i++ {
		lines = append(lines, "")
	}

	// Apply horizontal offset prefix
	prefix := ""
	for i := 0; i < config.OffsetX; i++ {
		prefix += " "
	}

	// Add decoration above head
	if config.Above != "" {
		lines = append(lines, prefix+config.Above)
	}

	// Add ears
	for _, line := range config.Ears {
		lines = append(lines, prefix+line)
	}

	// Add head
	for _, line := range config.Head {
		lines = append(lines, prefix+line)
	}

	// Add eyes with optional badge
	eyesLine := config.Eyes
	if config.EyesBadge != "" {
		eyesLine += " " + config.EyesBadge
	}
	lines = append(lines, prefix+eyesLine)

	// Add snout
	for _, line := range config.Snout {
		lines = append(lines, prefix+line)
	}

	// Add chin
	if config.Chin != "" {
		lines = append(lines, prefix+config.Chin)
	}

	// Add body
	for _, line := range config.Body {
		lines = append(lines, prefix+line)
	}

	// Add ground
	if config.Ground != "" {
		lines = append(lines, prefix+config.Ground)
	}

	return lines
}

// Pre-composed frames for basic emotional states

// FrameNeutral is the default neutral/idle state
var FrameNeutral = ComposeFrame(FrameConfig{
	Ears:  Ears.NORMAL,
	Head:  Head.NORMAL,
	Eyes:  Eyes.NORMAL,
	Snout: Snout.NORMAL,
	Chin:  Chin.NORMAL,
	Body:  Body.NORMAL,
})

// FrameHappy shows a happy, excited dingo
var FrameHappy = ComposeFrame(FrameConfig{
	Ears:  Ears.NORMAL,
	Head:  Head.NORMAL,
	Eyes:  Eyes.HAPPY,
	Snout: Snout.NORMAL,
	Chin:  Chin.HAPPY,
	Body:  Body.NORMAL,
})

// FrameSad shows a sad, disappointed dingo
var FrameSad = ComposeFrame(FrameConfig{
	Ears:  Ears.DROOPY,
	Head:  Head.DROOPY,
	Eyes:  Eyes.SAD,
	Snout: Snout.NORMAL,
	Chin:  Chin.NORMAL,
	Body:  Body.CROUCH,
})

// FrameAlert shows an alert, attentive dingo
var FrameAlert = ComposeFrame(FrameConfig{
	Ears:  Ears.ALERT_BOTH,
	Head:  Head.NORMAL,
	Eyes:  Eyes.WIDE,
	Snout: Snout.NORMAL,
	Chin:  Chin.NORMAL,
	Body:  Body.NORMAL,
})

// FrameWink shows a playful winking dingo
var FrameWink = ComposeFrame(FrameConfig{
	Ears:  Ears.NORMAL,
	Head:  Head.NORMAL,
	Eyes:  Eyes.WINK_R,
	Snout: Snout.NORMAL,
	Chin:  Chin.HAPPY,
	Body:  Body.NORMAL,
})

// FrameSleeping shows a sleeping dingo
var FrameSleeping = ComposeFrame(FrameConfig{
	Above: "       Zzz              ",
	Ears:  Ears.DROOPY,
	Head:  Head.NORMAL,
	Eyes:  Eyes.CLOSED,
	Snout: Snout.NORMAL,
	Chin:  Chin.NORMAL,
	Body:  Body.SITTING,
})

// Build-related frames

// FrameCompiling shows the dingo actively working/compiling
var FrameCompiling = ComposeFrame(FrameConfig{
	Ears:      Ears.ALERT_L,
	Head:      Head.NORMAL,
	Eyes:      Eyes.LOOK_L,
	EyesBadge: IconGear,
	Snout:     Snout.NORMAL,
	Chin:      Chin.NORMAL,
	Body:      Body.NORMAL,
})

// FrameBuildSuccess shows celebration after successful build
var FrameBuildSuccess = ComposeFrame(FrameConfig{
	Above:     "       ✧ ★ ✧           ",
	Ears:      Ears.NORMAL,
	Head:      Head.NORMAL,
	Eyes:      Eyes.STARS,
	EyesBadge: IconCheck,
	Snout:     Snout.NORMAL,
	Chin:      Chin.WITH_TAIL_UP,
	Body:      Body.ARMS_UP,
})

// FrameBuildFailed shows disappointment after failed build
var FrameBuildFailed = ComposeFrame(FrameConfig{
	Ears:      Ears.DROOPY,
	Head:      Head.DROOPY,
	Eyes:      Eyes.X_X,
	EyesBadge: IconCross,
	Snout:     Snout.NORMAL,
	Chin:      Chin.NORMAL,
	Body:      Body.CROUCH,
})

// Loading animation frames (4-frame spinner cycle)

// FrameLoading1 is the first frame of the loading animation
var FrameLoading1 = ComposeFrame(FrameConfig{
	Ears:      Ears.ALERT_L,
	Head:      Head.NORMAL,
	Eyes:      Eyes.LOADING_1,
	EyesBadge: Spinner1,
	Snout:     Snout.NORMAL,
	Chin:      Chin.WITH_TAIL_MID,
	Body:      Body.NORMAL,
})

// FrameLoading2 is the second frame of the loading animation
var FrameLoading2 = ComposeFrame(FrameConfig{
	Ears:      Ears.NORMAL,
	Head:      Head.NORMAL,
	Eyes:      Eyes.LOADING_2,
	EyesBadge: Spinner3,
	Snout:     Snout.NORMAL,
	Chin:      Chin.WITH_TAIL_MID,
	Body:      Body.NORMAL,
})

// FrameLoading3 is the third frame of the loading animation
var FrameLoading3 = ComposeFrame(FrameConfig{
	Ears:      Ears.ALERT_R,
	Head:      Head.NORMAL,
	Eyes:      Eyes.LOADING_3,
	EyesBadge: Spinner5,
	Snout:     Snout.NORMAL,
	Chin:      Chin.WITH_TAIL_MID,
	Body:      Body.NORMAL,
})

// FrameLoading4 is the fourth frame of the loading animation
var FrameLoading4 = ComposeFrame(FrameConfig{
	Ears:      Ears.NORMAL,
	Head:      Head.NORMAL,
	Eyes:      Eyes.LOADING_4,
	EyesBadge: Spinner7,
	Snout:     Snout.NORMAL,
	Chin:      Chin.WITH_TAIL_MID,
	Body:      Body.NORMAL,
})

// Status frames

// FrameSuccess shows general success state
var FrameSuccess = ComposeFrame(FrameConfig{
	Above:     "         ✓              ",
	Ears:      Ears.NORMAL,
	Head:      Head.NORMAL,
	Eyes:      Eyes.HAPPY,
	EyesBadge: IconCheck,
	Snout:     Snout.NORMAL,
	Chin:      Chin.WITH_TAIL_UP,
	Body:      Body.NORMAL,
})

// FrameError shows general error state
var FrameError = ComposeFrame(FrameConfig{
	Ears:      Ears.DROOPY,
	Head:      Head.DROOPY,
	Eyes:      Eyes.SAD,
	EyesBadge: IconCross,
	Snout:     Snout.NORMAL,
	Chin:      Chin.NORMAL,
	Body:      Body.CROUCH,
})

// FrameWarning shows warning/caution state
var FrameWarning = ComposeFrame(FrameConfig{
	Ears:      Ears.ALERT_BOTH,
	Head:      Head.NORMAL,
	Eyes:      Eyes.WIDE,
	EyesBadge: IconWarning,
	Snout:     Snout.NORMAL,
	Chin:      Chin.NORMAL,
	Body:      Body.NORMAL,
})

// Running/executing frames

// FrameRunning shows the dingo in a running pose
var FrameRunning = ComposeFrame(FrameConfig{
	Ears:      Ears.ALERT_R,
	Head:      Head.NORMAL,
	Eyes:      Eyes.LOOK_R,
	EyesBadge: IconArrowR,
	Snout:     Snout.SNIFF_R,
	Chin:      Chin.WITH_TAIL_UP,
	Body:      Body.RUNNING_1,
})

// Help/informational frames

// FrameHelp shows a friendly, welcoming pose for help/version commands
var FrameHelp = ComposeFrame(FrameConfig{
	Ears:  Ears.NORMAL,
	Head:  Head.NORMAL,
	Eyes:  Eyes.HAPPY,
	Snout: Snout.NORMAL,
	Chin:  Chin.HAPPY,
	Body:  Body.SITTING,
})

// FrameThinking shows the dingo pondering
var FrameThinking = ComposeFrame(FrameConfig{
	Above: "       ☁ ?              ",
	Ears:  Ears.ALERT_L,
	Head:  Head.NORMAL,
	Eyes:  Eyes.LOOK_UP,
	Snout: Snout.NORMAL,
	Chin:  Chin.NORMAL,
	Body:  Body.SITTING,
})

// Special animation frames

// FrameJumping shows the dingo jumping up
var FrameJumping = ComposeFrame(FrameConfig{
	Ears:    Ears.ALERT_BOTH,
	Head:    Head.NORMAL,
	Eyes:    Eyes.WIDE,
	Snout:   Snout.NORMAL,
	Chin:    Chin.NORMAL,
	Body:    Body.JUMPING,
	OffsetY: 1, // Offset up by one line
})

// FrameLanding shows the dingo landing after jump
var FrameLanding = ComposeFrame(FrameConfig{
	Ears:  Ears.DROOPY,
	Head:  Head.NORMAL,
	Eyes:  Eyes.NORMAL,
	Snout: Snout.NORMAL,
	Chin:  Chin.NORMAL,
	Body:  Body.LANDING,
})

// FrameStretch shows the dingo stretching
var FrameStretch = ComposeFrame(FrameConfig{
	Ears:  Ears.NORMAL,
	Head:  Head.NORMAL,
	Eyes:  Eyes.CLOSED,
	Snout: Snout.NORMAL,
	Chin:  Chin.NORMAL,
	Body:  Body.STRETCH,
})

// Running animation frames (3-frame cycle)

// FrameRunning1 is the first frame of the running animation
var FrameRunning1 = ComposeFrame(FrameConfig{
	Ears:  Ears.ALERT_R,
	Head:  Head.NORMAL,
	Eyes:  Eyes.LOOK_R,
	Snout: Snout.SNIFF_R,
	Chin:  Chin.WITH_TAIL_UP,
	Body:  Body.RUNNING_1,
})

// FrameRunning2 is the second frame of the running animation
var FrameRunning2 = ComposeFrame(FrameConfig{
	Ears:  Ears.ALERT_R,
	Head:  Head.NORMAL,
	Eyes:  Eyes.LOOK_R,
	Snout: Snout.SNIFF_R,
	Chin:  Chin.WITH_TAIL_MID,
	Body:  Body.RUNNING_2,
})

// FrameRunning3 is the third frame of the running animation
var FrameRunning3 = ComposeFrame(FrameConfig{
	Ears:  Ears.ALERT_R,
	Head:  Head.NORMAL,
	Eyes:  Eyes.LOOK_R,
	Snout: Snout.SNIFF_R,
	Chin:  Chin.WITH_TAIL_DOWN,
	Body:  Body.RUNNING_3,
})

// Sniffing animation frames (left-right alternating)

// FrameSniffLeft shows the dingo sniffing to the left
var FrameSniffLeft = ComposeFrame(FrameConfig{
	Ears:  Ears.ALERT_L,
	Head:  Head.NORMAL,
	Eyes:  Eyes.LOOK_L,
	Snout: Snout.SNIFF_L,
	Chin:  Chin.NORMAL,
	Body:  Body.NORMAL,
})

// FrameSniffRight shows the dingo sniffing to the right
var FrameSniffRight = ComposeFrame(FrameConfig{
	Ears:  Ears.ALERT_R,
	Head:  Head.NORMAL,
	Eyes:  Eyes.LOOK_R,
	Snout: Snout.SNIFF_R,
	Chin:  Chin.NORMAL,
	Body:  Body.NORMAL,
})

// Tail wag animation frames

// FrameTailWag1 shows tail wagging up
var FrameTailWag1 = ComposeFrame(FrameConfig{
	Ears:  Ears.NORMAL,
	Head:  Head.NORMAL,
	Eyes:  Eyes.HAPPY,
	Snout: Snout.NORMAL,
	Chin:  Chin.WITH_TAIL_UP,
	Body:  Body.NORMAL,
})

// FrameTailWag2 shows tail wagging mid
var FrameTailWag2 = ComposeFrame(FrameConfig{
	Ears:  Ears.NORMAL,
	Head:  Head.NORMAL,
	Eyes:  Eyes.HAPPY,
	Snout: Snout.NORMAL,
	Chin:  Chin.WITH_TAIL_MID,
	Body:  Body.NORMAL,
})

// FrameTailWag3 shows tail wagging down
var FrameTailWag3 = ComposeFrame(FrameConfig{
	Ears:  Ears.NORMAL,
	Head:  Head.NORMAL,
	Eyes:  Eyes.HAPPY,
	Snout: Snout.NORMAL,
	Chin:  Chin.WITH_TAIL_DOWN,
	Body:  Body.NORMAL,
})

// Looking around animation frames

// FrameLookLeft shows the dingo looking left
var FrameLookLeft = ComposeFrame(FrameConfig{
	Ears:  Ears.ALERT_L,
	Head:  Head.NORMAL,
	Eyes:  Eyes.LOOK_L,
	Snout: Snout.NORMAL,
	Chin:  Chin.NORMAL,
	Body:  Body.NORMAL,
})

// FrameLookRight shows the dingo looking right
var FrameLookRight = ComposeFrame(FrameConfig{
	Ears:  Ears.ALERT_R,
	Head:  Head.NORMAL,
	Eyes:  Eyes.LOOK_R,
	Snout: Snout.NORMAL,
	Chin:  Chin.NORMAL,
	Body:  Body.NORMAL,
})

// FrameLookUp shows the dingo looking up
var FrameLookUp = ComposeFrame(FrameConfig{
	Ears:  Ears.ALERT_BOTH,
	Head:  Head.NORMAL,
	Eyes:  Eyes.LOOK_UP,
	Snout: Snout.NORMAL,
	Chin:  Chin.NORMAL,
	Body:  Body.NORMAL,
})
