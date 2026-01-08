package golint

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config represents golangci-lint configuration
type Config struct {
	Linters struct {
		Enable  []string `yaml:"enable"`
		Disable []string `yaml:"disable"`
	} `yaml:"linters"`

	LintersSettings struct {
		Govet struct {
			CheckShadowing bool `yaml:"check-shadowing"`
		} `yaml:"govet"`
		Errcheck struct {
			CheckTypeAssertions bool `yaml:"check-type-assertions"`
		} `yaml:"errcheck"`
	} `yaml:"linters-settings"`

	Issues struct {
		ExcludeRules []ExcludeRule `yaml:"exclude-rules"`
	} `yaml:"issues"`

	Run struct {
		Timeout             string   `yaml:"timeout"`
		SkipDirs            []string `yaml:"skip-dirs"`
		SkipFiles           []string `yaml:"skip-files"`
		ModulesDownloadMode string   `yaml:"modules-download-mode"`
	} `yaml:"run"`
}

// ExcludeRule represents an exclusion rule for issues
type ExcludeRule struct {
	Path    string   `yaml:"path"`
	Linters []string `yaml:"linters"`
	Text    string   `yaml:"text"`
}

// DefaultConfig returns sensible defaults for Dingo-generated Go code
func DefaultConfig() *Config {
	cfg := &Config{}

	// Enable commonly useful linters for generated code
	cfg.Linters.Enable = []string{
		"errcheck",    // Check for unchecked errors
		"govet",       // Go vet
		"staticcheck", // Staticcheck
		"unused",      // Check for unused code
		"gosimple",    // Suggest code simplifications
		"ineffassign", // Detect ineffective assignments
	}

	// Disable linters that are noisy on generated code
	cfg.Linters.Disable = []string{
		"gofmt",      // We format with dingo fmt, not gofmt
		"goimports",  // Import management handled by transpiler
		"golint",     // Deprecated, replaced by revive
		"stylecheck", // Style issues handled by Dingo linter
	}

	// Configure govet
	cfg.LintersSettings.Govet.CheckShadowing = true

	// Configure errcheck
	cfg.LintersSettings.Errcheck.CheckTypeAssertions = true

	// Exclude rules for generated code patterns
	cfg.Issues.ExcludeRules = []ExcludeRule{
		{
			// Skip temp variable naming issues (we use tmp, tmp1, etc.)
			Text:    "var.*tmp.*is unused",
			Linters: []string{"unused"},
		},
		{
			// Skip generated type switch exhaustiveness
			Text:    "missing cases in switch",
			Linters: []string{"exhaustive"},
		},
	}

	// Run configuration
	cfg.Run.Timeout = "5m"
	cfg.Run.ModulesDownloadMode = "readonly"
	cfg.Run.SkipDirs = []string{
		"vendor",
		"testdata",
	}

	return cfg
}

// WriteToFile writes the config to a YAML file
func (c *Config) WriteToFile(path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write config file: %w", err)
	}

	return nil
}

// LoadFromFile loads config from a YAML file
func LoadFromFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	return &cfg, nil
}
