# Configuration Integration Notes

## Existing Configuration System

Dingo already has a robust TOML-based configuration system:
- **File**: `pkg/config/config.go`
- **Format**: TOML (dingo.toml in project root, ~/.dingo/config.toml for user defaults)
- **Precedence**: CLI flags > Project dingo.toml > User config > Defaults

## Current Configuration Structure

```toml
[features]
error_propagation_syntax = "question"  # or "bang" or "try"
reuse_err_variable = true
nil_safety_checks = "on"  # or "off" or "debug"

[sourcemaps]
enabled = true
format = "inline"  # or "separate", "both", "none"
```

## Integration Plan for New Features

We need to extend the existing `FeatureConfig` struct with new fields:

```go
type FeatureConfig struct {
    // Existing fields...
    ErrorPropagationSyntax SyntaxStyle `toml:"error_propagation_syntax"`
    ReuseErrVariable       bool        `toml:"reuse_err_variable"`
    NilSafetyChecks        string      `toml:"nil_safety_checks"`

    // NEW: Lambda syntax configuration
    LambdaSyntax           string      `toml:"lambda_syntax"`  // "rust", "arrow", "both"

    // NEW: Safe navigation configuration
    SafeNavigationUnwrap   string      `toml:"safe_navigation_unwrap"`  // "always_option", "smart"

    // NEW: Null coalescing configuration
    NullCoalescingPointers bool        `toml:"null_coalescing_pointers"`  // true = work with *T, false = Option[T] only

    // NEW: Operator precedence configuration
    OperatorPrecedence     string      `toml:"operator_precedence"`  // "standard", "explicit"
}
```

## Default Values

```toml
[features]
lambda_syntax = "rust"  # Start with Rust-style as default
safe_navigation_unwrap = "smart"  # Smart unwrapping
null_coalescing_pointers = true  # Support Go pointers
operator_precedence = "standard"  # Follow TypeScript/C# conventions
```

This integrates seamlessly with the existing config system without breaking changes.
