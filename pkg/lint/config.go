package lint

import (
	"os"
	"path/filepath"

	"github.com/MadAppGang/dingo/pkg/lint/analyzer"
	"github.com/pelletier/go-toml/v2"
)

// Config holds configuration for the Dingo linter
type Config struct {
	// Disabled rules (by name)
	Disabled []string `toml:"disable"`

	// Severity overrides (by code, e.g., "D101" -> SeverityHint)
	SeverityOverrides map[string]analyzer.Severity `toml:"-"`

	// Refactoring suggestions configuration
	Refactor RefactorConfig `toml:"refactor"`
}

// RefactorConfig controls refactoring suggestions
type RefactorConfig struct {
	Enabled  bool     `toml:"enabled"`
	Disabled []string `toml:"disable"` // Disabled refactoring rules
}

// SeverityConfig is used for TOML parsing of severity overrides
type SeverityConfig map[string]string

// ConfigFile represents the full dingo.toml structure
type ConfigFile struct {
	Lint struct {
		Disable  []string       `toml:"disable"`
		Severity SeverityConfig `toml:"severity"`
		Refactor RefactorConfig `toml:"refactor"`
	} `toml:"lint"`
}

// DefaultConfig returns the default linter configuration
func DefaultConfig() *Config {
	return &Config{
		Disabled:          []string{},
		SeverityOverrides: make(map[string]analyzer.Severity),
		Refactor: RefactorConfig{
			Enabled:  true,
			Disabled: []string{},
		},
	}
}

// LoadConfig loads configuration from dingo.toml in the current directory or parent directories
func LoadConfig() *Config {
	cfg := DefaultConfig()

	// Search for dingo.toml starting from current directory
	configPath := findConfigFile("dingo.toml")
	if configPath == "" {
		return cfg // Use defaults if no config file found
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return cfg // Use defaults on error
	}

	var cfgFile ConfigFile
	if err := toml.Unmarshal(data, &cfgFile); err != nil {
		return cfg // Use defaults on error
	}

	// Apply configuration
	cfg.Disabled = cfgFile.Lint.Disable
	cfg.Refactor = cfgFile.Lint.Refactor

	// Parse severity overrides
	cfg.SeverityOverrides = parseSeverityOverrides(cfgFile.Lint.Severity)

	return cfg
}

// LoadConfigFromFile loads configuration from a specific file path
func LoadConfigFromFile(path string) (*Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfgFile ConfigFile
	if err := toml.Unmarshal(data, &cfgFile); err != nil {
		return nil, err
	}

	cfg.Disabled = cfgFile.Lint.Disable
	cfg.Refactor = cfgFile.Lint.Refactor
	cfg.SeverityOverrides = parseSeverityOverrides(cfgFile.Lint.Severity)

	return cfg, nil
}

// IsEnabled checks if an analyzer is enabled
func (c *Config) IsEnabled(name string) bool {
	// Handle nil config or nil Disabled slice
	if c == nil || c.Disabled == nil {
		return true // Default: all analyzers enabled
	}

	for _, disabled := range c.Disabled {
		if disabled == name {
			return false
		}
	}
	return true
}

// IsRefactorEnabled checks if a refactoring rule is enabled
func (c *Config) IsRefactorEnabled(code string) bool {
	if !c.Refactor.Enabled {
		return false
	}

	for _, disabled := range c.Refactor.Disabled {
		if disabled == code {
			return false
		}
	}
	return true
}

// findConfigFile searches for a config file starting from the current directory
// and walking up to parent directories
func findConfigFile(name string) string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}

	for {
		configPath := filepath.Join(dir, name)
		if _, err := os.Stat(configPath); err == nil {
			return configPath
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root directory
			break
		}
		dir = parent
	}

	return ""
}

// parseSeverityOverrides converts string severity values to Severity enum
func parseSeverityOverrides(severityMap SeverityConfig) map[string]analyzer.Severity {
	result := make(map[string]analyzer.Severity)

	for code, severityStr := range severityMap {
		switch severityStr {
		case "hint":
			result[code] = analyzer.SeverityHint
		case "info":
			result[code] = analyzer.SeverityInfo
		case "warning":
			result[code] = analyzer.SeverityWarning
		default:
			// Unknown severity, use default (warning)
			result[code] = analyzer.SeverityWarning
		}
	}

	return result
}
