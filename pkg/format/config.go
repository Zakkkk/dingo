package format

// Config holds formatter configuration
type Config struct {
	// Indentation
	IndentWidth int  // Number of spaces for indentation (default: 4)
	UseTabs     bool // Use tabs instead of spaces (default: false)

	// Match expression formatting
	MatchArmAlignment bool // Align match arms at => (default: true)

	// Lambda formatting
	LambdaSpacing bool // Add spaces around lambda arrows (default: true)

	// General
	MaxLineWidth int // Maximum line width before wrapping (default: 100, 0 = no limit)
}

// DefaultConfig returns the default formatter configuration
func DefaultConfig() *Config {
	return &Config{
		IndentWidth:       4,
		UseTabs:           false,
		MatchArmAlignment: true,
		LambdaSpacing:     true,
		MaxLineWidth:      100,
	}
}

// IndentString returns the string to use for one level of indentation
func (c *Config) IndentString() string {
	if c.UseTabs {
		return "\t"
	}
	result := make([]byte, c.IndentWidth)
	for i := range result {
		result[i] = ' '
	}
	return string(result)
}
