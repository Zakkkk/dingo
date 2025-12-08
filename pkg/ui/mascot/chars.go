package mascot

// Chars contains all Unicode character constants used for building the ASCII art mascot.
// These constants are organized by category for easy reference and composition.

// Block characters for constructing the dingo's body
const (
	BlockFull  = "█" // Full block
	BlockUpper = "▀" // Upper half block
	BlockLower = "▄" // Lower half block
	BlockLeft  = "▌" // Left half block
	BlockRight = "▐" // Right half block
	BlockLight = "░" // Light shade
	BlockMed   = "▒" // Medium shade
	BlockDark  = "▓" // Dark shade
)

// Line drawing characters
const (
	LineH     = "─" // Horizontal line
	LineV     = "│" // Vertical line
	LineDiagU = "╱" // Diagonal up-right
	LineDiagD = "╲" // Diagonal down-right
	LineSlash = "/"
	LineBack  = "\\"
)

// Eye characters for different expressions
const (
	EyeNormal      = "●" // Normal solid eye
	EyeSmall       = "•" // Small dot eye
	EyeLarge       = "◉" // Large outlined eye
	EyeClosed      = "-" // Closed eye (sleeping/happy)
	EyeHappy       = "^" // Happy closed eye
	EyeSad         = "╥" // Sad eye with tear
	EyeX           = "X" // Dead/failed eye
	EyeHalfL       = "◖" // Left half circle (looking left)
	EyeHalfR       = "◗" // Right half circle (looking right)
	EyeHalfU       = "◠" // Upper half circle (looking up)
	EyeHalfD       = "◡" // Lower half circle (looking down)
	EyeSquint      = "⌐" // Squinting eye
	EyeWink        = "◕" // Winking eye
	EyeSparkle     = "✧" // Sparkle eye (excited)
	EyeHeart       = "♥" // Heart eye
	EyeLoader1     = "◜" // Loading animation frame 1
	EyeLoader2     = "◝" // Loading animation frame 2
	EyeLoader3     = "◞" // Loading animation frame 3
	EyeLoader4     = "◟" // Loading animation frame 4
	EyeSpinner1    = "⠋" // Braille spinner frame 1
	EyeSpinner2    = "⠙" // Braille spinner frame 2
	EyeSpinner3    = "⠹" // Braille spinner frame 3
	EyeSpinner4    = "⠸" // Braille spinner frame 4
	EyeSpinner5    = "⠼" // Braille spinner frame 5
	EyeSpinner6    = "⠴" // Braille spinner frame 6
	EyeSpinner7    = "⠦" // Braille spinner frame 7
	EyeSpinner8    = "⠧" // Braille spinner frame 8
)

// Nose/snout characters
const (
	NoseNormal   = "▲" // Triangle nose pointing up
	NoseSmall    = "△" // Small triangle nose
	NoseSniff    = "◭" // Sniffing nose
	NoseTwitch   = "▴" // Twitching nose
	NoseFlat     = "▬" // Flat nose (happy/content)
)

// Tail characters
const (
	TailNormal  = "~" // Wavy normal tail
	TailUp      = "╯" // Tail up (alert)
	TailDown    = "╰" // Tail down (sad)
	TailWag1    = "∼" // Wag animation frame 1
	TailWag2    = "≈" // Wag animation frame 2
	TailCurved  = "⌒" // Curved tail (happy)
	TailStraight = "│" // Straight tail
)

// Status icon characters (displayed next to mascot)
const (
	IconCheck    = "✓" // Success checkmark
	IconCross    = "✗" // Error cross
	IconWarning  = "⚠" // Warning triangle
	IconGear     = "⚙" // Working/building gear
	IconRocket   = "🚀" // Fast/launching
	IconStar     = "★" // Achievement/highlight
	IconStarO    = "☆" // Empty star
	IconSparkle  = "✨" // Sparkles (celebration)
	IconHeart    = "♥" // Love/favorite
	IconDiamond  = "◆" // Diamond (premium/special)
	IconDot      = "●" // Simple dot
	IconDotO     = "○" // Empty dot
	IconArrowR   = "→" // Arrow right
	IconArrowL   = "←" // Arrow left
	IconArrowU   = "↑" // Arrow up
	IconArrowD   = "↓" // Arrow down
	IconBullet   = "•" // Bullet point
	IconPlus     = "+" // Plus
	IconMinus    = "-" // Minus
	IconEquals   = "=" // Equals
)

// Spinner animation frames (for loading states)
const (
	Spinner1  = "⠋" // Dots spinner frame 1
	Spinner2  = "⠙" // Dots spinner frame 2
	Spinner3  = "⠹" // Dots spinner frame 3
	Spinner4  = "⠸" // Dots spinner frame 4
	Spinner5  = "⠼" // Dots spinner frame 5
	Spinner6  = "⠴" // Dots spinner frame 6
	Spinner7  = "⠦" // Dots spinner frame 7
	Spinner8  = "⠧" // Dots spinner frame 8
	Spinner9  = "⠇" // Dots spinner frame 9
	Spinner10 = "⠏" // Dots spinner frame 10
)

// Alternative spinner styles
const (
	SpinBar1 = "|"  // Bar spinner frame 1
	SpinBar2 = "/"  // Bar spinner frame 2
	SpinBar3 = "-"  // Bar spinner frame 3
	SpinBar4 = "\\" // Bar spinner frame 4
)

const (
	SpinCircle1 = "◜" // Circle spinner frame 1
	SpinCircle2 = "◝" // Circle spinner frame 2
	SpinCircle3 = "◞" // Circle spinner frame 3
	SpinCircle4 = "◟" // Circle spinner frame 4
)

const (
	SpinBox1 = "▖" // Box spinner frame 1
	SpinBox2 = "▘" // Box spinner frame 2
	SpinBox3 = "▝" // Box spinner frame 3
	SpinBox4 = "▗" // Box spinner frame 4
)

// Music/sound effect characters
const (
	MusicNote1   = "♪" // Single eighth note
	MusicNote2   = "♫" // Beamed eighth notes
	MusicNoteBeam = "♬" // Beamed sixteenth notes
	SoundWave1   = "~" // Sound wave
	SoundWave2   = "≈" // Double sound wave
	SoundWave3   = "∿" // Sine wave
)

// Decoration characters (above head, around mascot)
const (
	DecoStar     = "★" // Solid star
	DecoStarO    = "☆" // Outline star
	DecoSparkle  = "✧" // Small sparkle
	DecoSparkleL = "✨" // Large sparkle
	DecoCircle   = "◯" // Circle
	DecoCloud    = "☁" // Cloud (thinking)
	DecoZzz      = "Zzz" // Sleep indicator
	DecoHeart    = "♥" // Heart
	DecoFlower   = "❀" // Flower
	DecoLeaf     = "🍃" // Leaf
)

// Corner and curve characters for smooth shapes
const (
	CornerTL  = "▄" // Top-left corner curve
	CornerTR  = "▄" // Top-right corner curve
	CornerBL  = "▀" // Bottom-left corner curve
	CornerBR  = "▀" // Bottom-right corner curve
	CurveUp   = "╭" // Curve up-right
	CurveDown = "╰" // Curve down-right
	CurveL    = "╮" // Curve left-down
	CurveR    = "╯" // Curve right-up
)

// Miscellaneous utility characters
const (
	Space      = " "  // Space (for alignment)
	Dot        = "·"  // Middle dot
	GroundLine = "▔"  // Ground line (for standing poses)
	GroundWave = "≋"  // Wavy ground (water)
	GroundDash = "┄"  // Dashed ground
)

// SpinnerFrames provides a convenient slice of all spinner frames
var SpinnerFrames = []string{
	Spinner1, Spinner2, Spinner3, Spinner4, Spinner5,
	Spinner6, Spinner7, Spinner8, Spinner9, Spinner10,
}

// BarSpinnerFrames provides bar-style spinner frames
var BarSpinnerFrames = []string{
	SpinBar1, SpinBar2, SpinBar3, SpinBar4,
}

// CircleSpinnerFrames provides circle-style spinner frames
var CircleSpinnerFrames = []string{
	SpinCircle1, SpinCircle2, SpinCircle3, SpinCircle4,
}

// BoxSpinnerFrames provides box-style spinner frames
var BoxSpinnerFrames = []string{
	SpinBox1, SpinBox2, SpinBox3, SpinBox4,
}

// EyeSpinnerFrames provides eye-specific spinner frames for animations
var EyeSpinnerFrames = []string{
	EyeSpinner1, EyeSpinner2, EyeSpinner3, EyeSpinner4,
	EyeSpinner5, EyeSpinner6, EyeSpinner7, EyeSpinner8,
}

// EyeLoaderFrames provides loader-style eye animations
var EyeLoaderFrames = []string{
	EyeLoader1, EyeLoader2, EyeLoader3, EyeLoader4,
}
