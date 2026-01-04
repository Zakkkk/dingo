package format

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

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

// tomlFormatConfig represents the [format] section in dingo.toml
type tomlFormatConfig struct {
	IndentWidth       *int  `toml:"indent_width"`
	UseTabs           *bool `toml:"use_tabs"`
	MatchArmAlignment *bool `toml:"match_arm_alignment"`
	LambdaSpacing     *bool `toml:"lambda_spacing"`
	MaxLineWidth      *int  `toml:"max_line_width"`
}

// tomlConfig represents the structure of dingo.toml with a [format] section
type tomlConfig struct {
	Format tomlFormatConfig `toml:"format"`
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

// I2: applyTOMLConfig applies TOML config values to a Config struct
// Only applies values that were explicitly set (non-nil pointers)
func applyTOMLConfig(cfg *Config, fc tomlFormatConfig) {
	if fc.IndentWidth != nil {
		cfg.IndentWidth = *fc.IndentWidth
	}
	if fc.UseTabs != nil {
		cfg.UseTabs = *fc.UseTabs
	}
	if fc.MatchArmAlignment != nil {
		cfg.MatchArmAlignment = *fc.MatchArmAlignment
	}
	if fc.LambdaSpacing != nil {
		cfg.LambdaSpacing = *fc.LambdaSpacing
	}
	if fc.MaxLineWidth != nil {
		cfg.MaxLineWidth = *fc.MaxLineWidth
	}
}

// LoadConfig loads format configuration from dingo.toml.
// It searches for dingo.toml starting from startDir and walking up parent directories.
// Returns the default config if no dingo.toml is found or if it has no [format] section.
func LoadConfig(startDir string) (*Config, error) {
	cfg := DefaultConfig()

	// Find dingo.toml
	tomlPath, err := findDingoToml(startDir)
	if err != nil {
		return nil, fmt.Errorf("error searching for dingo.toml: %w", err)
	}
	if tomlPath == "" {
		// No dingo.toml found, use defaults
		return cfg, nil
	}

	// Parse dingo.toml
	var tc tomlConfig
	if _, err := toml.DecodeFile(tomlPath, &tc); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", tomlPath, err)
	}

	// I2: Use helper function to apply config values
	applyTOMLConfig(cfg, tc.Format)

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid format configuration in %s: %w", tomlPath, err)
	}

	return cfg, nil
}

// LoadConfigFromFile loads format configuration from a specific dingo.toml path.
// Returns the default config if the file doesn't exist or has no [format] section.
func LoadConfigFromFile(tomlPath string) (*Config, error) {
	cfg := DefaultConfig()

	// Check if file exists
	if _, err := os.Stat(tomlPath); os.IsNotExist(err) {
		return cfg, nil
	}

	// Parse dingo.toml
	var tc tomlConfig
	if _, err := toml.DecodeFile(tomlPath, &tc); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", tomlPath, err)
	}

	// I2: Use helper function to apply config values
	applyTOMLConfig(cfg, tc.Format)

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid format configuration in %s: %w", tomlPath, err)
	}

	return cfg, nil
}

// findDingoToml walks up the directory tree from startDir looking for dingo.toml.
// Returns the path to dingo.toml if found, or empty string if not found.
func findDingoToml(startDir string) (string, error) {
	// Resolve to absolute path
	absStart, err := filepath.Abs(startDir)
	if err != nil {
		return "", fmt.Errorf("failed to resolve start directory: %w", err)
	}

	currentDir := absStart
	for {
		configPath := filepath.Join(currentDir, "dingo.toml")
		if _, err := os.Stat(configPath); err == nil {
			return configPath, nil
		}

		// Move up one directory
		parentDir := filepath.Dir(currentDir)
		if parentDir == currentDir {
			// Reached filesystem root
			return "", nil
		}
		currentDir = parentDir
	}
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.IndentWidth < 0 {
		return fmt.Errorf("indent_width must be non-negative, got %d", c.IndentWidth)
	}
	if c.IndentWidth > 16 {
		return fmt.Errorf("indent_width must be <= 16, got %d", c.IndentWidth)
	}
	if c.MaxLineWidth < 0 {
		return fmt.Errorf("max_line_width must be non-negative (0 = unlimited), got %d", c.MaxLineWidth)
	}
	return nil
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
