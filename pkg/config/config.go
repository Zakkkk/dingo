// Package config provides configuration management for the Dingo compiler
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// SyntaxStyle represents the error propagation syntax style
type SyntaxStyle string

const (
	// SyntaxQuestion uses the ? operator (expr?)
	SyntaxQuestion SyntaxStyle = "question"

	// SyntaxBang uses the ! operator (expr!)
	SyntaxBang SyntaxStyle = "bang"

	// SyntaxTry uses the try keyword (try expr)
	SyntaxTry SyntaxStyle = "try"
)

// IsValid reports whether the syntax style is valid
func (s SyntaxStyle) IsValid() bool {
	switch s {
	case SyntaxQuestion, SyntaxBang, SyntaxTry:
		return true
	default:
		return false
	}
}

// SourceMapFormat represents the source map output format
type SourceMapFormat string

const (
	// FormatInline embeds source maps as comments in generated Go files
	FormatInline SourceMapFormat = "inline"

	// FormatSeparate writes source maps to .go.map files
	FormatSeparate SourceMapFormat = "separate"

	// FormatBoth writes both inline and separate source maps
	FormatBoth SourceMapFormat = "both"

	// FormatNone disables source map generation
	FormatNone SourceMapFormat = "none"
)

// MatchConfig controls pattern matching feature behavior
type MatchConfig struct {
	// Syntax selects the pattern matching syntax style
	// Valid values: "rust" (only)
	// - "rust": Rust-style match syntax (match expr { ... })
	// Note: Swift syntax ("swift") was removed in Phase 4.2 (incomplete implementation)
	Syntax string `toml:"syntax"`
}

// DebugConfig controls debug-related output settings
type DebugConfig struct {
	// KeepMarkers controls whether internal markers remain in generated .go files
	// Markers like // dingo:let:x, // dingo:E:1 are used internally for source maps
	// When true: Keep markers (useful for debugging transpiler)
	// When false: Remove markers for clean production output (default)
	KeepMarkers bool `toml:"keep_markers"`
}

// BuildConfig contains build-related configuration
type BuildConfig struct {
	// OutDir specifies the default output directory for transpiled files
	// When set, all .go and .dmap files are written to this directory
	// and source directory structure is mirrored. Pure .go files are
	// automatically copied to maintain a buildable output tree.
	// Empty string (default) means output files are placed alongside source files.
	OutDir string `toml:"outdir"`

	// TranspileMode selects the transpilation strategy
	// Valid values: "legacy", "ast", "hybrid"
	// - "legacy": Preprocessor-based (current production implementation)
	// - "ast": New AST parser + transformer (under development)
	// - "hybrid": Try AST first, fall back to legacy on error
	// Default: "legacy" for backward compatibility
	TranspileMode string `toml:"transpile_mode"`
}

// Config represents the complete Dingo project configuration
type Config struct {
	Features      FeatureConfig   `toml:"features"`
	FeatureMatrix FeatureMatrix   `toml:"feature_matrix"`
	Match         MatchConfig     `toml:"match"`
	SourceMap     SourceMapConfig `toml:"sourcemaps"`
	Debug         DebugConfig     `toml:"debug"`
	Build         BuildConfig     `toml:"build"`
}

// FeatureMatrix controls which language features are enabled/disabled.
// All features are enabled by default. Setting a feature to false disables it.
// When a disabled feature's syntax is used, the transpiler will report an error.
type FeatureMatrix struct {
	// Character-level features (transform raw source)
	Enum             *bool `toml:"enum"`              // enum declarations
	Match            *bool `toml:"match"`             // match expressions
	EnumConstructors *bool `toml:"enum_constructors"` // Variant() -> NewVariant()
	ErrorProp        *bool `toml:"error_prop"`        // ? operator for error propagation
	GuardLet         *bool `toml:"guard_let"`         // guard let expressions
	SafeNavStatements *bool `toml:"safe_nav_statements"` // ?. in statements
	SafeNav          *bool `toml:"safe_nav"`          // ?. operator
	NullCoalesce     *bool `toml:"null_coalesce"`     // ?? operator
	Lambdas          *bool `toml:"lambdas"`           // |x| expr and x => expr

	// Token-level features (transform after tokenization)
	Generics        *bool `toml:"generics"`         // <T> syntax
	LetBinding      *bool `toml:"let_binding"`      // let x = expr
}

// ToEnabledFeatures converts the FeatureMatrix to a map[string]bool
// for use with the feature engine. Features not explicitly set are enabled by default.
func (fm *FeatureMatrix) ToEnabledFeatures() map[string]bool {
	result := make(map[string]bool)

	// Helper to add feature if explicitly set
	addIfSet := func(name string, val *bool) {
		if val != nil {
			result[name] = *val
		}
	}

	// Character-level features
	addIfSet("enum", fm.Enum)
	addIfSet("match", fm.Match)
	addIfSet("enum_constructors", fm.EnumConstructors)
	addIfSet("error_prop", fm.ErrorProp)
	addIfSet("guard_let", fm.GuardLet)
	addIfSet("safe_nav_statements", fm.SafeNavStatements)
	addIfSet("safe_nav", fm.SafeNav)
	addIfSet("null_coalesce", fm.NullCoalesce)
	addIfSet("lambdas", fm.Lambdas)

	// Token-level features
	addIfSet("generics", fm.Generics)
	addIfSet("let_binding", fm.LetBinding)

	return result
}

// IsFeatureEnabled checks if a specific feature is enabled.
// Returns true if not explicitly disabled (features enabled by default).
func (fm *FeatureMatrix) IsFeatureEnabled(name string) bool {
	switch name {
	case "enum":
		return fm.Enum == nil || *fm.Enum
	case "match":
		return fm.Match == nil || *fm.Match
	case "enum_constructors":
		return fm.EnumConstructors == nil || *fm.EnumConstructors
	case "error_prop":
		return fm.ErrorProp == nil || *fm.ErrorProp
	case "guard_let":
		return fm.GuardLet == nil || *fm.GuardLet
	case "safe_nav_statements":
		return fm.SafeNavStatements == nil || *fm.SafeNavStatements
	case "safe_nav":
		return fm.SafeNav == nil || *fm.SafeNav
	case "null_coalesce":
		return fm.NullCoalesce == nil || *fm.NullCoalesce
	case "lambdas":
		return fm.Lambdas == nil || *fm.Lambdas
	case "generics":
		return fm.Generics == nil || *fm.Generics
	case "let_binding":
		return fm.LetBinding == nil || *fm.LetBinding
	default:
		return true // Unknown features enabled by default
	}
}

// ResultTypeConfig controls Result[T, E] type behavior
type ResultTypeConfig struct {
	// Enabled controls whether Result type is available
	Enabled bool `toml:"enabled"`

	// GoInterop controls how Go (T, error) returns are handled
	// Valid values: "opt-in", "auto", "disabled"
	// - "opt-in": Requires explicit Result.FromGo() wrapper (safe default)
	// - "auto": Automatically wraps (T, error) → Result[T, E]
	// - "disabled": No Go interop, pure Dingo types only
	GoInterop string `toml:"go_interop"`
}

// OptionTypeConfig controls Option[T] type behavior
type OptionTypeConfig struct {
	// Enabled controls whether Option type is available
	Enabled bool `toml:"enabled"`

	// GoInterop controls how Go pointer types (*T) are handled
	// Valid values: "opt-in", "auto", "disabled"
	// - "opt-in": Requires explicit Option.FromPtr() wrapper (safe default)
	// - "auto": Automatically wraps *T → Option[T]
	// - "disabled": No Go interop, pure Dingo types only
	GoInterop string `toml:"go_interop"`
}

// FeatureConfig controls which language features are enabled
type FeatureConfig struct {
	// ErrorPropagationSyntax selects the error propagation operator
	// Valid values: "question", "bang", "try"
	ErrorPropagationSyntax SyntaxStyle `toml:"error_propagation_syntax"`

	// ReuseErrVariable controls whether to reuse a single "err" variable
	// instead of generating __err0, __err1, etc. in the same scope
	// When true: always uses "err" (cleaner, more idiomatic)
	// When false: generates unique names (safer, avoids shadowing)
	ReuseErrVariable bool `toml:"reuse_err_variable"`

	// NilSafetyChecks controls nil pointer validation in pattern destructuring
	// Valid values: "off", "on", "debug"
	// - "off": No nil checks (trust constructors, maximum performance)
	// - "on": Always check with runtime panic (safe, default)
	// - "debug": Check only when DINGO_DEBUG env var is set
	NilSafetyChecks string `toml:"nil_safety_checks"`

	// LambdaStyle controls which lambda function syntax style is used
	// Valid values: "typescript", "rust"
	// - "typescript": Only TypeScript/JavaScript arrow syntax: x => expr (default)
	// - "rust": Only Rust-style pipe syntax: |x| expr
	// Note: Only ONE style is active per project to avoid ambiguity
	LambdaStyle string `toml:"lambda_style"`

	// SafeNavigationUnwrap controls how the ?. operator handles return types
	// Valid values: "always_option", "smart"
	// - "always_option": Always returns Option[T]
	// - "smart": Unwraps to T based on context (default)
	SafeNavigationUnwrap string `toml:"safe_navigation_unwrap"`

	// NullCoalescingPointers enables ?? operator for Go pointers (*T)
	// When true: Works with both Option[T] and *T
	// When false: Works only with Option[T] (stricter type safety)
	NullCoalescingPointers bool `toml:"null_coalescing_pointers"`

	// OperatorPrecedence controls ternary/null-coalescing precedence checking
	// Valid values: "standard", "explicit"
	// - "standard": Follow C/TypeScript precedence rules
	// - "explicit": Require parentheses for ambiguous mixing
	OperatorPrecedence string `toml:"operator_precedence"`

	// ResultType controls Result[T, E] type generation and Go interop
	ResultType ResultTypeConfig `toml:"result_type"`

	// OptionType controls Option[T] type generation and Go interop
	OptionType OptionTypeConfig `toml:"option_type"`
}

// SourceMapConfig controls source map generation
type SourceMapConfig struct {
	// Enabled controls whether source maps are generated
	Enabled bool `toml:"enabled"`

	// Format controls the source map output format
	// Valid values: "inline", "separate", "both", "none"
	Format SourceMapFormat `toml:"format"`
}

// NilSafetyMode represents nil safety check modes
type NilSafetyMode int

const (
	NilSafetyOff NilSafetyMode = iota
	NilSafetyOn
	NilSafetyDebug
)

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		Features: FeatureConfig{
			ErrorPropagationSyntax: SyntaxQuestion, // Default to ? operator
			ReuseErrVariable:       true,           // Default to reusing "err" for cleaner code
			NilSafetyChecks:        "on",           // Default to safe mode
			LambdaStyle:            "typescript",   // Default to TypeScript arrow syntax
			SafeNavigationUnwrap:   "smart",        // Default to smart unwrapping
			NullCoalescingPointers: true,           // Default to supporting Go pointers
			OperatorPrecedence:     "standard",     // Default to standard precedence
			ResultType: ResultTypeConfig{
				Enabled:   true,
				GoInterop: "opt-in", // Default to safe explicit wrapping
			},
			OptionType: OptionTypeConfig{
				Enabled:   true,
				GoInterop: "opt-in", // Default to safe explicit wrapping
			},
		},
		Match: MatchConfig{
			Syntax: "rust", // Default to Rust-style match syntax
		},
		SourceMap: SourceMapConfig{
			Enabled: true,
			Format:  FormatInline, // Default to inline for development
		},
		Debug: DebugConfig{
			KeepMarkers: false, // Default to clean output for production
		},
		Build: BuildConfig{
			OutDir:        "",       // Default to placing output alongside source
			TranspileMode: "legacy", // Default to legacy mode for stability
		},
	}
}

// Load loads configuration from multiple sources with precedence:
// 1. CLI flags (highest priority) - passed as overrides
// 2. Project dingo.toml (current directory)
// 3. User config (~/.dingo/config.toml)
// 4. Built-in defaults (lowest priority)
func Load(overrides *Config) (*Config, error) {
	// Start with defaults
	cfg := DefaultConfig()

	// Load user config if it exists
	userConfigPath := filepath.Join(os.Getenv("HOME"), ".dingo", "config.toml")
	if err := loadConfigFile(userConfigPath, cfg); err != nil {
		return nil, fmt.Errorf("failed to load user config: %w", err)
	}

	// Load project config if it exists
	projectConfigPath := "dingo.toml"
	if err := loadConfigFile(projectConfigPath, cfg); err != nil {
		return nil, fmt.Errorf("failed to load project config: %w", err)
	}

	// Apply overrides from CLI flags
	if overrides != nil {
		if overrides.Features.ErrorPropagationSyntax != "" {
			cfg.Features.ErrorPropagationSyntax = overrides.Features.ErrorPropagationSyntax
		}
		if overrides.SourceMap.Format != "" {
			cfg.SourceMap.Format = overrides.SourceMap.Format
		}
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

// FindConfig walks up the directory tree from startDir looking for dingo.toml
// Returns the loaded config, the directory containing dingo.toml, and any error
// If no dingo.toml is found, returns (nil, "", nil) - not an error
func FindConfig(startDir string) (*Config, string, error) {
	absStart, err := filepath.Abs(startDir)
	if err != nil {
		return nil, "", fmt.Errorf("failed to resolve start directory: %w", err)
	}

	currentDir := absStart
	for {
		configPath := filepath.Join(currentDir, "dingo.toml")
		if _, err := os.Stat(configPath); err == nil {
			// Found dingo.toml, load it
			cfg := DefaultConfig()
			if _, err := toml.DecodeFile(configPath, cfg); err != nil {
				return nil, "", fmt.Errorf("failed to parse %s: %w", configPath, err)
			}

			if err := cfg.Validate(); err != nil {
				return nil, "", fmt.Errorf("invalid configuration in %s: %w", configPath, err)
			}

			return cfg, currentDir, nil
		}

		// Move up one directory
		parentDir := filepath.Dir(currentDir)
		if parentDir == currentDir {
			// Reached filesystem root, no dingo.toml found
			return nil, "", nil
		}
		currentDir = parentDir
	}
}

// loadConfigFile loads a TOML configuration file into the provided config
// If the file doesn't exist, this is not an error (we use defaults)
func loadConfigFile(path string, cfg *Config) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil // File doesn't exist, use defaults
	}

	if _, err := toml.DecodeFile(path, cfg); err != nil {
		return fmt.Errorf("failed to parse %s: %w", path, err)
	}

	return nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	// Validate error propagation syntax
	if !c.Features.ErrorPropagationSyntax.IsValid() {
		return fmt.Errorf("invalid error_propagation_syntax: %q (must be 'question', 'bang', or 'try')",
			c.Features.ErrorPropagationSyntax)
	}

	// Validate match syntax
	if c.Match.Syntax != "" {
		switch c.Match.Syntax {
		case "rust":
			// Valid
		case "swift":
			// Deprecated: Swift syntax removed in Phase 4.2
			return fmt.Errorf("invalid match.syntax: %q (Swift syntax removed in Phase 4.2, use 'rust' only)",
				c.Match.Syntax)
		default:
			return fmt.Errorf("invalid match.syntax: %q (must be 'rust')",
				c.Match.Syntax)
		}
	}

	// Validate nil safety mode
	if c.Features.NilSafetyChecks != "" {
		switch c.Features.NilSafetyChecks {
		case "off", "on", "debug":
			// Valid
		default:
			return fmt.Errorf("invalid nil_safety_checks: %q (must be 'off', 'on', or 'debug')",
				c.Features.NilSafetyChecks)
		}
	}

	// Validate lambda style
	if c.Features.LambdaStyle != "" {
		switch c.Features.LambdaStyle {
		case "rust", "typescript":
			// Valid
		default:
			return fmt.Errorf("invalid lambda_style: %q (must be 'rust' or 'typescript')",
				c.Features.LambdaStyle)
		}
	}

	// Validate safe navigation unwrap mode
	if c.Features.SafeNavigationUnwrap != "" {
		switch c.Features.SafeNavigationUnwrap {
		case "always_option", "smart":
			// Valid
		default:
			return fmt.Errorf("invalid safe_navigation_unwrap: %q (must be 'always_option' or 'smart')",
				c.Features.SafeNavigationUnwrap)
		}
	}

	// Validate operator precedence
	if c.Features.OperatorPrecedence != "" {
		switch c.Features.OperatorPrecedence {
		case "standard", "explicit":
			// Valid
		default:
			return fmt.Errorf("invalid operator_precedence: %q (must be 'standard' or 'explicit')",
				c.Features.OperatorPrecedence)
		}
	}

	// Validate Result type go_interop mode
	if c.Features.ResultType.GoInterop != "" {
		switch c.Features.ResultType.GoInterop {
		case "opt-in", "auto", "disabled":
			// Valid
		default:
			return fmt.Errorf("invalid result_type.go_interop: %q (must be 'opt-in', 'auto', or 'disabled')",
				c.Features.ResultType.GoInterop)
		}
	}

	// Validate Option type go_interop mode
	if c.Features.OptionType.GoInterop != "" {
		switch c.Features.OptionType.GoInterop {
		case "opt-in", "auto", "disabled":
			// Valid
		default:
			return fmt.Errorf("invalid option_type.go_interop: %q (must be 'opt-in', 'auto', or 'disabled')",
				c.Features.OptionType.GoInterop)
		}
	}

	// Validate source map format
	switch c.SourceMap.Format {
	case FormatInline, FormatSeparate, FormatBoth, FormatNone:
		// Valid
	default:
		return fmt.Errorf("invalid sourcemap format: %q (must be 'inline', 'separate', 'both', or 'none')",
			c.SourceMap.Format)
	}

	// Validate transpile mode
	if c.Build.TranspileMode != "" {
		switch c.Build.TranspileMode {
		case "legacy", "ast", "hybrid":
			// Valid
		default:
			return fmt.Errorf("invalid build.transpile_mode: %q (must be 'legacy', 'ast', or 'hybrid')",
				c.Build.TranspileMode)
		}
	}

	return nil
}

// GetNilSafetyMode parses the nil safety string into enum
func (c *Config) GetNilSafetyMode() NilSafetyMode {
	switch c.Features.NilSafetyChecks {
	case "off":
		return NilSafetyOff
	case "on":
		return NilSafetyOn
	case "debug":
		return NilSafetyDebug
	default:
		return NilSafetyOn // Default to safe mode
	}
}
