package format

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.IndentWidth != 4 {
		t.Errorf("IndentWidth = %d, want 4", cfg.IndentWidth)
	}
	if cfg.UseTabs != false {
		t.Errorf("UseTabs = %v, want false", cfg.UseTabs)
	}
	if cfg.MatchArmAlignment != true {
		t.Errorf("MatchArmAlignment = %v, want true", cfg.MatchArmAlignment)
	}
	if cfg.LambdaSpacing != true {
		t.Errorf("LambdaSpacing = %v, want true", cfg.LambdaSpacing)
	}
	if cfg.MaxLineWidth != 100 {
		t.Errorf("MaxLineWidth = %d, want 100", cfg.MaxLineWidth)
	}
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
	}{
		{
			name:    "valid_default",
			cfg:     DefaultConfig(),
			wantErr: false,
		},
		{
			name: "valid_custom",
			cfg: &Config{
				IndentWidth:       2,
				UseTabs:           true,
				MatchArmAlignment: false,
				LambdaSpacing:     false,
				MaxLineWidth:      120,
			},
			wantErr: false,
		},
		{
			name: "valid_unlimited_line_width",
			cfg: &Config{
				IndentWidth:  4,
				MaxLineWidth: 0,
			},
			wantErr: false,
		},
		{
			name: "invalid_negative_indent",
			cfg: &Config{
				IndentWidth: -1,
			},
			wantErr: true,
		},
		{
			name: "invalid_large_indent",
			cfg: &Config{
				IndentWidth: 17,
			},
			wantErr: true,
		},
		{
			name: "invalid_negative_line_width",
			cfg: &Config{
				IndentWidth:  4,
				MaxLineWidth: -10,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLoadConfigFromFile(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir := t.TempDir()

	tests := []struct {
		name         string
		tomlContent  string
		wantIndent   int
		wantTabs     bool
		wantMaxWidth int
		wantErr      bool
	}{
		{
			name:         "no_file",
			tomlContent:  "", // No file will be created
			wantIndent:   4,  // Default
			wantTabs:     false,
			wantMaxWidth: 100,
			wantErr:      false,
		},
		{
			name: "full_format_section",
			tomlContent: `
[format]
indent_width = 2
use_tabs = true
match_arm_alignment = false
lambda_spacing = false
max_line_width = 80
`,
			wantIndent:   2,
			wantTabs:     true,
			wantMaxWidth: 80,
			wantErr:      false,
		},
		{
			name: "partial_format_section",
			tomlContent: `
[format]
indent_width = 8
`,
			wantIndent:   8,
			wantTabs:     false, // Default
			wantMaxWidth: 100,   // Default
			wantErr:      false,
		},
		{
			name: "empty_format_section",
			tomlContent: `
[format]
`,
			wantIndent:   4, // Default
			wantTabs:     false,
			wantMaxWidth: 100,
			wantErr:      false,
		},
		{
			name: "with_other_sections",
			tomlContent: `
[build]
outdir = "out"

[format]
indent_width = 3

[features]
lambda_style = "typescript"
`,
			wantIndent:   3,
			wantTabs:     false,
			wantMaxWidth: 100,
			wantErr:      false,
		},
		{
			name: "unlimited_line_width",
			tomlContent: `
[format]
max_line_width = 0
`,
			wantIndent:   4,
			wantTabs:     false,
			wantMaxWidth: 0, // Unlimited
			wantErr:      false,
		},
		{
			name: "invalid_toml_syntax",
			tomlContent: `
[format
indent_width = 4
`,
			wantErr: true,
		},
		{
			name: "invalid_indent_width",
			tomlContent: `
[format]
indent_width = 20
`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test file path
			tomlPath := filepath.Join(tmpDir, tt.name+"_dingo.toml")

			// Create file if content is provided
			if tt.tomlContent != "" {
				err := os.WriteFile(tomlPath, []byte(tt.tomlContent), 0644)
				if err != nil {
					t.Fatalf("Failed to create test file: %v", err)
				}
			}

			// Load config
			cfg, err := LoadConfigFromFile(tomlPath)

			if tt.wantErr {
				if err == nil {
					t.Errorf("LoadConfigFromFile() error = nil, wantErr = true")
				}
				return
			}

			if err != nil {
				t.Fatalf("LoadConfigFromFile() error = %v, wantErr = false", err)
			}

			// Check values
			if cfg.IndentWidth != tt.wantIndent {
				t.Errorf("IndentWidth = %d, want %d", cfg.IndentWidth, tt.wantIndent)
			}
			if cfg.UseTabs != tt.wantTabs {
				t.Errorf("UseTabs = %v, want %v", cfg.UseTabs, tt.wantTabs)
			}
			if cfg.MaxLineWidth != tt.wantMaxWidth {
				t.Errorf("MaxLineWidth = %d, want %d", cfg.MaxLineWidth, tt.wantMaxWidth)
			}
		})
	}
}

func TestLoadConfigDirectoryWalk(t *testing.T) {
	// Create a temporary directory structure:
	// tmpDir/
	//   dingo.toml (root config)
	//   subdir/
	//     deeper/
	//       (no config, should find root)
	tmpDir := t.TempDir()

	rootToml := `
[format]
indent_width = 2
use_tabs = true
`
	err := os.WriteFile(filepath.Join(tmpDir, "dingo.toml"), []byte(rootToml), 0644)
	if err != nil {
		t.Fatalf("Failed to create root dingo.toml: %v", err)
	}

	// Create subdirectory structure
	deepDir := filepath.Join(tmpDir, "subdir", "deeper")
	if err := os.MkdirAll(deepDir, 0755); err != nil {
		t.Fatalf("Failed to create subdirectories: %v", err)
	}

	// Load config from deep directory - should find root config
	cfg, err := LoadConfig(deepDir)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if cfg.IndentWidth != 2 {
		t.Errorf("IndentWidth = %d, want 2 (from root config)", cfg.IndentWidth)
	}
	if cfg.UseTabs != true {
		t.Errorf("UseTabs = %v, want true (from root config)", cfg.UseTabs)
	}
}

func TestLoadConfigNearerConfigWins(t *testing.T) {
	// Create a temporary directory structure:
	// tmpDir/
	//   dingo.toml (indent_width = 2)
	//   subdir/
	//     dingo.toml (indent_width = 8)
	//     file.dingo
	tmpDir := t.TempDir()

	rootToml := `
[format]
indent_width = 2
`
	err := os.WriteFile(filepath.Join(tmpDir, "dingo.toml"), []byte(rootToml), 0644)
	if err != nil {
		t.Fatalf("Failed to create root dingo.toml: %v", err)
	}

	// Create subdirectory with its own config
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	subToml := `
[format]
indent_width = 8
`
	err = os.WriteFile(filepath.Join(subDir, "dingo.toml"), []byte(subToml), 0644)
	if err != nil {
		t.Fatalf("Failed to create subdir dingo.toml: %v", err)
	}

	// Load config from subdirectory - should use subdirectory config (nearer)
	cfg, err := LoadConfig(subDir)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if cfg.IndentWidth != 8 {
		t.Errorf("IndentWidth = %d, want 8 (from nearer config)", cfg.IndentWidth)
	}
}

func TestLoadConfigNoConfigFile(t *testing.T) {
	// Create an empty temp directory with no dingo.toml
	tmpDir := t.TempDir()

	cfg, err := LoadConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	// Should return defaults
	defaultCfg := DefaultConfig()
	if cfg.IndentWidth != defaultCfg.IndentWidth {
		t.Errorf("IndentWidth = %d, want default %d", cfg.IndentWidth, defaultCfg.IndentWidth)
	}
	if cfg.UseTabs != defaultCfg.UseTabs {
		t.Errorf("UseTabs = %v, want default %v", cfg.UseTabs, defaultCfg.UseTabs)
	}
	if cfg.MaxLineWidth != defaultCfg.MaxLineWidth {
		t.Errorf("MaxLineWidth = %d, want default %d", cfg.MaxLineWidth, defaultCfg.MaxLineWidth)
	}
}
