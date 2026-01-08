package mascot

// Parts contains all body part variants for composing the dingo mascot.
// Each variant is either a single string (one line) or a string slice (multiple lines).
// The mascot is designed to be 24 characters wide for consistent layout.

// Ears variants - Different ear positions for emotional states
var Ears = struct {
	NORMAL     []string
	ALERT_L    []string
	ALERT_R    []string
	ALERT_BOTH []string
	DROOPY     []string
}{
	NORMAL: []string{
		"     ▄▀▀▄   ▄▀▀▄      ",
	},
	ALERT_L: []string{
		"     ▄▀▄    ▄▀▀▄      ",
	},
	ALERT_R: []string{
		"     ▄▀▀▄    ▄▀▄      ",
	},
	ALERT_BOTH: []string{
		"     ▄▀▄    ▄▀▄       ",
	},
	DROOPY: []string{
		"     ▄▀▄    ▄▀▄       ",
	},
}

// Head variants - Head shapes for different expressions
var Head = struct {
	NORMAL []string
	DROOPY []string
}{
	NORMAL: []string{
		"     █  ▀▀▀▀▀  █      ",
	},
	DROOPY: []string{
		"     █  ▀▀▀▀▀  █      ",
	},
}

// Eyes variants - Single-line eye expressions
var Eyes = struct {
	NORMAL    string
	HAPPY     string
	SAD       string
	CLOSED    string
	LOOK_L    string
	LOOK_R    string
	LOOK_UP   string
	LOOK_UP_L string
	LOOK_UP_R string
	WIDE      string
	X_X       string
	WINK_R    string
	WINK_L    string
	LOADING_1 string
	LOADING_2 string
	LOADING_3 string
	LOADING_4 string
	DOTS      string
	STARS     string
}{
	NORMAL:    "     █  ●   ●  █      ",
	HAPPY:     "     █  ^   ^  █      ",
	SAD:       "     █  ╥   ╥  █      ",
	CLOSED:    "     █  -   -  █      ",
	LOOK_L:    "     █ ●    ●  █      ",
	LOOK_R:    "     █  ●    ● █      ",
	LOOK_UP:   "     █  ◠   ◠  █      ",
	LOOK_UP_L: "     █ ◠    ◠  █      ",
	LOOK_UP_R: "     █  ◠    ◠ █      ",
	WIDE:      "     █  ◉   ◉  █      ",
	X_X:       "     █  X   X  █      ",
	WINK_R:    "     █  ●   -  █      ",
	WINK_L:    "     █  -   ●  █      ",
	LOADING_1: "     █  ◜   ◜  █      ",
	LOADING_2: "     █  ◝   ◝  █      ",
	LOADING_3: "     █  ◞   ◞  █      ",
	LOADING_4: "     █  ◟   ◟  █      ",
	DOTS:      "     █  •   •  █      ",
	STARS:     "     █  ✧   ✧  █      ",
}

// Snout variants - Nose and mouth area
var Snout = struct {
	NORMAL  []string
	SNIFF_L []string
	SNIFF_R []string
}{
	NORMAL: []string{
		"     ▀▄   ▲   ▄▀      ",
	},
	SNIFF_L: []string{
		"     ▀▄  ▲    ▄▀      ",
	},
	SNIFF_R: []string{
		"     ▀▄    ▲  ▄▀      ",
	},
}

// Chin variants - Single-line chin/jaw with optional tail
var Chin = struct {
	NORMAL         string
	HAPPY          string
	WITH_TAIL_UP   string
	WITH_TAIL_MID  string
	WITH_TAIL_DOWN string
}{
	NORMAL:         "       ▀▄▄▄▄▄▀        ",
	HAPPY:          "       ▀▄▄▄▄▄▀        ",
	WITH_TAIL_UP:   "       ▀▄▄▄▄▄▀╯       ",
	WITH_TAIL_MID:  "       ▀▄▄▄▄▄▀~       ",
	WITH_TAIL_DOWN: "       ▀▄▄▄▄▄▀╰       ",
}

// Body variants - Multi-line body shapes for different poses
var Body = struct {
	NORMAL         []string
	SITTING        []string
	CROUCH         []string
	JUMPING        []string
	JUMPING_TUCKED []string
	RUNNING_1      []string
	RUNNING_2      []string
	RUNNING_3      []string
	LANDING        []string
	STRETCH        []string
	ARMS_UP        []string
}{
	NORMAL: []string{
		"      ▄█▀   ▀█▄       ",
		"     ██  ███  ██      ",
		"     ▀█▄▄▀ ▀▄▄█▀      ",
	},
	SITTING: []string{
		"      ▄█▀   ▀█▄       ",
		"     ██  ███  ██      ",
		"     ██▄▄▄▄▄▄▄██      ",
		"     ▀▀       ▀▀      ",
	},
	CROUCH: []string{
		"      ▄█     █▄       ",
		"     ██▄▄███▄▄██      ",
		"     ▀█▀     ▀█▀      ",
	},
	JUMPING: []string{
		"      ╱█▀   ▀█╲       ",
		"     ╱█  ███  █╲      ",
		"     ▀█       █▀      ",
	},
	JUMPING_TUCKED: []string{
		"      ╱█▀▀▀▀▀█╲       ",
		"     ╱ ███████ ╲      ",
		"     ▀         ▀      ",
	},
	RUNNING_1: []string{
		"      ▄█▀   ▀█▄       ",
		"     ██  ███  ██╲     ",
		"     ▀█▄▄▀ ▀▄▄ ▀╲     ",
	},
	RUNNING_2: []string{
		"      ▄█▀   ▀█▄       ",
		"    ╱██  ███  ██      ",
		"   ╱ ▀█▄▄▀ ▀▄▄█▀      ",
	},
	RUNNING_3: []string{
		"      ▄█▀   ▀█▄       ",
		"     ██  ███  ██      ",
		"    ╱▀█▄▄▀ ▀▄▄█╲      ",
	},
	LANDING: []string{
		"      ▄█▀   ▀█▄       ",
		"     ██  ███  ██      ",
		"     ██▄▄▀ ▀▄▄██      ",
		"     ▀▀       ▀▀      ",
	},
	STRETCH: []string{
		"      ▄█▀▀▀▀▀█▄       ",
		"     ██  ███  ██      ",
		"     ▀█▄▄▀ ▀▄▄█▀      ",
	},
	ARMS_UP: []string{
		"      ╱█▀   ▀█╲       ",
		"     │ █ ███ █ │      ",
		"      ▀█▄▄▄▄▄█▀       ",
	},
}
