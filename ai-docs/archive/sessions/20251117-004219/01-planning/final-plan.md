# Final Architectural Plan: Configurable Null Safety, Null Coalescing, Ternary, and Lambda Features

## Executive Summary

This document presents the final architectural design incorporating user preferences for maximum configurability. All four features support behavioral switches via a new configuration system:

1. **Null Safety Operator (`?.`)** - Smart unwrapping based on context
2. **Null Coalescing Operator (`??`)** - Works with both Option[T] and Go pointers (*T)
3. **Ternary Operator (`? :`)** - Configurable precedence modes
4. **Lambda Functions** - Switchable syntax styles (Rust/Arrow/Both)

---

## 0. Configuration System Architecture

### 0.1 Configuration File Structure

**File:** `.dingorc` or `dingo.config.json` in project root

```json
{
  "language": {
    "lambda_syntax": "both",        // "rust" | "arrow" | "both"
    "operator_precedence": "standard", // "standard" | "explicit"
    "safe_navigation_unwrap": "smart" // "always_option" | "smart"
  },
  "transpiler": {
    "null_coalescing_pointers": true, // Enable ?? for *T
    "generate_source_maps": true
  }
}
```

### 0.2 Configuration Loader

**New Package:** `pkg/config/`

```go
// pkg/config/config.go
package config

import (
    "encoding/json"
    "os"
    "path/filepath"
)

type LambdaSyntax string

const (
    LambdaSyntaxRust  LambdaSyntax = "rust"
    LambdaSyntaxArrow LambdaSyntax = "arrow"
    LambdaSyntaxBoth  LambdaSyntax = "both"
)

type OperatorPrecedence string

const (
    OperatorPrecedenceStandard OperatorPrecedence = "standard"
    OperatorPrecedenceExplicit OperatorPrecedence = "explicit"
)

type SafeNavUnwrap string

const (
    SafeNavUnwrapAlways SafeNavUnwrap = "always_option"
    SafeNavUnwrapSmart  SafeNavUnwrap = "smart"
)

type LanguageConfig struct {
    LambdaSyntax         LambdaSyntax       `json:"lambda_syntax"`
    OperatorPrecedence   OperatorPrecedence `json:"operator_precedence"`
    SafeNavigationUnwrap SafeNavUnwrap      `json:"safe_navigation_unwrap"`
}

type TranspilerConfig struct {
    NullCoalescingPointers bool `json:"null_coalescing_pointers"`
    GenerateSourceMaps     bool `json:"generate_source_maps"`
}

type Config struct {
    Language   LanguageConfig   `json:"language"`
    Transpiler TranspilerConfig `json:"transpiler"`
}

// Default returns sensible defaults
func Default() *Config {
    return &Config{
        Language: LanguageConfig{
            LambdaSyntax:         LambdaSyntaxBoth,
            OperatorPrecedence:   OperatorPrecedenceStandard,
            SafeNavigationUnwrap: SafeNavUnwrapSmart,
        },
        Transpiler: TranspilerConfig{
            NullCoalescingPointers: true,
            GenerateSourceMaps:     true,
        },
    }
}

// Load reads config from .dingorc or dingo.config.json
func Load(dir string) (*Config, error) {
    cfg := Default()

    // Try .dingorc first
    path := filepath.Join(dir, ".dingorc")
    if _, err := os.Stat(path); err == nil {
        return loadFromFile(cfg, path)
    }

    // Try dingo.config.json
    path = filepath.Join(dir, "dingo.config.json")
    if _, err := os.Stat(path); err == nil {
        return loadFromFile(cfg, path)
    }

    // Return defaults if no config file
    return cfg, nil
}

func loadFromFile(cfg *Config, path string) (*Config, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, err
    }

    if err := json.Unmarshal(data, cfg); err != nil {
        return nil, err
    }

    return cfg, nil
}
```

### 0.3 Integration with Parser and Plugins

**Parser receives config:**

```go
// pkg/parser/participle.go
type Parser struct {
    parser *participle.Parser[File]
    config *config.Config  // NEW: Configuration
}

func NewParser(cfg *config.Config) (*Parser, error) {
    // Adjust lexer/grammar based on config
    // ...
}
```

**Plugins receive config:**

```go
// pkg/plugin/context.go
type Context struct {
    File     *ast.File
    TypeInfo *TypeInfo
    Errors   []error
    Config   *config.Config  // NEW: Configuration available to all plugins
}
```

---

## 1. Null Safety Operator (`?.`) - Smart Unwrapping

### 1.1 Configuration Impact

**Setting:** `safe_navigation_unwrap`

**Modes:**
1. `"always_option"` - Always returns Option[T]
2. `"smart"` (default) - Unwraps when context expects T

### 1.2 Smart Unwrapping Logic

```go
// pkg/plugin/builtin/safe_navigation.go
func (p *SafeNavigationPlugin) Transform(ctx *plugin.Context, node ast.Node) (ast.Node, error) {
    safeNav, ok := node.(*dingoast.SafeNavigationExpr)
    if !ok {
        return node, nil
    }

    // Check configuration
    unwrapMode := ctx.Config.Language.SafeNavigationUnwrap

    switch unwrapMode {
    case config.SafeNavUnwrapAlways:
        return p.transformAlwaysOption(ctx, safeNav)
    case config.SafeNavUnwrapSmart:
        return p.transformSmart(ctx, safeNav)
    }

    return node, nil
}

func (p *SafeNavigationPlugin) transformSmart(ctx *plugin.Context, safeNav *dingoast.SafeNavigationExpr) (ast.Node, error) {
    // Determine expected type from context
    expectedType := ctx.TypeInfo.TypeOf(safeNav)

    // If parent expects Option[T], return Option
    if isOptionType(expectedType) {
        return p.transformAlwaysOption(ctx, safeNav)
    }

    // If parent expects T (unwrapped), generate with nil check
    // user?.name in context expecting string
    return p.transformUnwrapped(ctx, safeNav)
}

func (p *SafeNavigationPlugin) transformUnwrapped(ctx *plugin.Context, safeNav *dingoast.SafeNavigationExpr) (ast.Node, error) {
    // Generate: user != nil ? user.Name : ""
    // (zero value of T if nil)

    // For chained: user?.address?.city
    // Generate: user != nil && user.Address != nil ? user.Address.City : nil
}
```

### 1.3 Examples

**Mode: `always_option`**
```dingo
city := user?.address?.city  // Type: Option[*City]
```

```go
var city Option_City
if user != nil && user.Address != nil {
    city = Option_Some(user.Address.City)
} else {
    city = Option_None[*City]()
}
```

**Mode: `smart`**
```dingo
// Context expects string (unwrapped)
let name: string = user?.name  // Auto-unwrap with zero value
```

```go
var name string
if user != nil {
    name = user.Name
} else {
    name = ""  // Zero value
}
```

```dingo
// Context expects Option[string] (keep wrapped)
let nameOpt: Option[string] = user?.name
```

```go
var nameOpt Option_string
if user != nil {
    nameOpt = Option_Some(user.Name)
} else {
    nameOpt = Option_None[string]()
}
```

### 1.4 AST (unchanged from initial plan)

```go
type SafeNavigationExpr struct {
    X      ast.Expr
    OpPos  token.Pos
    Sel    *ast.Ident
}
```

---

## 2. Null Coalescing Operator (`??`) - Dual Type Support

### 2.1 Configuration Impact

**Setting:** `null_coalescing_pointers`

**Modes:**
- `true` (default) - Works with both Option[T] and *T
- `false` - Works only with Option[T]

### 2.2 Type Checking Logic

```go
// pkg/plugin/builtin/null_coalescing.go
func (p *NullCoalescingPlugin) Transform(ctx *plugin.Context, node ast.Node) (ast.Node, error) {
    nc, ok := node.(*dingoast.NullCoalescingExpr)
    if !ok {
        return node, nil
    }

    leftType := ctx.TypeInfo.TypeOf(nc.X)

    // Check if left is Option[T]
    if isOptionType(leftType) {
        return p.transformOption(ctx, nc)
    }

    // Check if pointers are enabled and left is *T
    if ctx.Config.Transpiler.NullCoalescingPointers && isPointer(leftType) {
        return p.transformPointer(ctx, nc)
    }

    return nil, fmt.Errorf("?? requires Option[T] or *T (if enabled), got %v", leftType)
}

func (p *NullCoalescingPlugin) transformPointer(ctx *plugin.Context, nc *dingoast.NullCoalescingExpr) (ast.Node, error) {
    // Generate: ptr != nil ? *ptr : defaultValue

    return &ast.ConditionalExpr{
        Cond: &ast.BinaryExpr{
            X:  nc.X,
            Op: token.NEQ,
            Y:  ast.NewIdent("nil"),
        },
        Then: &ast.StarExpr{X: nc.X},  // Dereference pointer
        Else: nc.Y,
    }, nil
}
```

### 2.3 Examples

**With Option[T]:**
```dingo
let name: string = optionalName ?? "default"
```

```go
var name string
if optionalName.IsSome() {
    name = optionalName.Unwrap()
} else {
    name = "default"
}
```

**With Go Pointers (if enabled):**
```dingo
let name: string = user.Name ?? "anonymous"  // Name is *string
```

```go
var name string
if user.Name != nil {
    name = *user.Name
} else {
    name = "anonymous"
}
```

**Chaining mixed types:**
```dingo
value := optionalValue ?? pointerValue ?? "default"
```

```go
var value string
var __tmp0 string
if pointerValue != nil {
    __tmp0 = *pointerValue
} else {
    __tmp0 = "default"
}

if optionalValue.IsSome() {
    value = optionalValue.Unwrap()
} else {
    value = __tmp0
}
```

### 2.4 AST (unchanged)

```go
type NullCoalescingExpr struct {
    X      ast.Expr
    OpPos  token.Pos
    Y      ast.Expr
}
```

---

## 3. Ternary Operator (`? :`) - Configurable Precedence

### 3.1 Configuration Impact

**Setting:** `operator_precedence`

**Modes:**
1. `"standard"` (default) - Follow C/TypeScript precedence
2. `"explicit"` - Require parentheses for ambiguous cases

### 3.2 Precedence Enforcement

**Standard Mode:**
```dingo
// These parse without error
x := a ?? b ? c : d        // (a ?? b) ? c : d
y := cond ? a ?? b : c     // cond ? (a ?? b) : c
```

**Explicit Mode:**
```dingo
// These require parentheses
x := a ?? b ? c : d        // ERROR: ambiguous, add ()
x := (a ?? b) ? c : d      // OK

y := cond ? a ?? b : c     // ERROR: ambiguous
y := cond ? (a ?? b) : c   // OK
```

### 3.3 Parser Implementation

```go
// pkg/parser/participle.go
func (p *Parser) parseTernary() (*TernaryExpr, error) {
    left := p.parseNullCoalesce()

    if !p.match(token.QUESTION) {
        return left, nil
    }

    // Check precedence mode
    if p.config.Language.OperatorPrecedence == config.OperatorPrecedenceExplicit {
        // Verify no ambiguous mixing
        if containsNullCoalesce(left) {
            return nil, fmt.Errorf("ambiguous precedence: use parentheses")
        }
    }

    then := p.parseTernary()
    p.expect(token.COLON)
    els := p.parseTernary()

    return &TernaryExpr{Cond: left, Then: then, Else: els}, nil
}
```

### 3.4 Precedence Table

**Standard Mode:**
| Precedence | Operators |
|------------|-----------|
| 14 | `()` `[]` `.` `?.` |
| 13 | `!` `-` (unary) |
| 12 | `*` `/` `%` |
| 11 | `+` `-` |
| 10 | `<` `>` `<=` `>=` |
| 9 | `==` `!=` |
| 8 | `&&` |
| 7 | `||` |
| **6** | `? :` (ternary) |
| **5** | `??` (null coalescing) |
| 4 | `?` (error prop) |

**Explicit Mode:**
- Same precedence, but parser errors on mixing without `()`

### 3.5 AST (unchanged)

```go
type TernaryExpr struct {
    Cond     ast.Expr
    Question token.Pos
    Then     ast.Expr
    Colon    token.Pos
    Else     ast.Expr
}
```

---

## 4. Lambda Functions - Configurable Syntax

### 4.1 Configuration Impact

**Setting:** `lambda_syntax`

**Modes:**
1. `"rust"` - Only `|x| expr`
2. `"arrow"` - Only `(x) => expr`
3. `"both"` (default) - Accept both styles

### 4.2 Parser Grammar Switching

```go
// pkg/parser/participle.go
func (p *Parser) buildGrammar(cfg *config.Config) *participle.Parser[File] {
    switch cfg.Language.LambdaSyntax {
    case config.LambdaSyntaxRust:
        return p.buildRustLambdaGrammar()
    case config.LambdaSyntaxArrow:
        return p.buildArrowLambdaGrammar()
    case config.LambdaSyntaxBoth:
        return p.buildBothLambdaGrammar()
    }
}

func (p *Parser) buildBothLambdaGrammar() *participle.Parser[File] {
    // Accept both:
    // |x| expr
    // (x) => expr
    //
    // Strategy: Try Rust-style first (unambiguous), fall back to arrow
}
```

### 4.3 Examples

**Rust Style:**
```dingo
add := |a, b| a + b
double := |x| x * 2
```

**Arrow Style:**
```dingo
add := (a, b) => a + b
double := x => x * 2  // Single param, no parens
```

**Both (allowed simultaneously):**
```dingo
rustStyle := |x| x + 1
arrowStyle := (x) => x + 1
// Both valid in same file
```

### 4.4 AST Representation

```go
type LambdaExpr struct {
    Style  LambdaStyle    // Which syntax was used
    Pipe   token.Pos
    Params *ast.FieldList
    Arrow  token.Pos      // Position of '=>' or '->'
    Body   ast.Expr
    Rpipe  token.Pos
}

type LambdaStyle int

const (
    LambdaStyleRust LambdaStyle = iota
    LambdaStyleArrow
)

func (l *LambdaExpr) String() string {
    if l.Style == LambdaStyleRust {
        return "|...| ..."
    }
    return "(...) => ..."
}
```

### 4.5 Transpilation (style-agnostic)

Both styles transpile identically to Go:

```dingo
// Rust
fn1 := |x| x * 2

// Arrow
fn2 := (x) => x * 2
```

```go
// Both produce:
var fn1 = func(x int) int {
    return x * 2
}

var fn2 = func(x int) int {
    return x * 2
}
```

---

## 5. Updated Plugin Architecture

### 5.1 Plugin Interface with Config

```go
// pkg/plugin/plugin.go
type Plugin interface {
    Name() string
    Dependencies() []string
    Transform(ctx *Context, node ast.Node) (ast.Node, error)

    // NEW: Optional configuration validation
    ValidateConfig(cfg *config.Config) error
}

type Context struct {
    File     *ast.File
    TypeInfo *TypeInfo
    Errors   []error
    Config   *config.Config  // Available to all plugins
}
```

### 5.2 Plugin Configuration Usage

**Example: Safe Navigation Plugin**

```go
type SafeNavigationPlugin struct {
    plugin.BasePlugin
}

func (p *SafeNavigationPlugin) ValidateConfig(cfg *config.Config) error {
    // Validate smart unwrap is compatible with other settings
    if cfg.Language.SafeNavigationUnwrap == config.SafeNavUnwrapSmart {
        if !cfg.Transpiler.GenerateSourceMaps {
            return fmt.Errorf("smart unwrap requires source maps for type inference")
        }
    }
    return nil
}

func (p *SafeNavigationPlugin) Transform(ctx *Context, node ast.Node) (ast.Node, error) {
    // Access config
    mode := ctx.Config.Language.SafeNavigationUnwrap

    // Transform based on mode
    // ...
}
```

### 5.3 Configuration Validation Pipeline

```go
// pkg/transpiler/transpiler.go
func (t *Transpiler) Validate(cfg *config.Config) error {
    // Load all plugins
    plugins := t.loadPlugins()

    // Validate each plugin's config requirements
    for _, plugin := range plugins {
        if validator, ok := plugin.(interface {
            ValidateConfig(*config.Config) error
        }); ok {
            if err := validator.ValidateConfig(cfg); err != nil {
                return fmt.Errorf("plugin %s: %w", plugin.Name(), err)
            }
        }
    }

    return nil
}
```

---

## 6. Cross-Feature Integration with Configuration

### 6.1 Combined Examples

**Standard Precedence Mode:**
```dingo
// Config: operator_precedence = "standard"

// Allowed: ?? binds tighter than ternary
result := value ?? default ? "yes" : "no"
// Parses as: (value ?? default) ? "yes" : "no"

// Allowed: Smart unwrap in ternary
name := user?.name == "admin" ? "Admin" : "User"
// user?.name unwraps to string in comparison context
```

**Explicit Precedence Mode:**
```dingo
// Config: operator_precedence = "explicit"

// ERROR: Requires parentheses
result := value ?? default ? "yes" : "no"

// OK
result := (value ?? default) ? "yes" : "no"
```

**Pointer Support:**
```dingo
// Config: null_coalescing_pointers = true

type User struct {
    Name *string  // Go pointer
}

userName := user.Name ?? "anonymous"  // Works!
```

**Lambda Style Mixing:**
```dingo
// Config: lambda_syntax = "both"

rustStyle := users.filter(|u| u.age > 18)
arrowStyle := users.map(u => u.name)
// Both valid!
```

### 6.2 Configuration Interactions

**Matrix of Settings:**

| Feature | Setting | Impacts |
|---------|---------|---------|
| Safe Nav | `always_option` | Forces Option return, requires ?? to unwrap |
| Safe Nav | `smart` | Auto-unwraps, integrates with ternary/comparison |
| Null Coalesce | `pointers=true` | Works with both Option and *T |
| Null Coalesce | `pointers=false` | Option[T] only, stricter |
| Precedence | `standard` | Allows complex expressions |
| Precedence | `explicit` | Requires (), clearer |
| Lambda | `rust` | Only pipes |
| Lambda | `arrow` | Only arrows |
| Lambda | `both` | Maximum flexibility |

**Recommended Presets:**

**Strict Mode (explicit, type-safe):**
```json
{
  "language": {
    "lambda_syntax": "rust",
    "operator_precedence": "explicit",
    "safe_navigation_unwrap": "always_option"
  },
  "transpiler": {
    "null_coalescing_pointers": false
  }
}
```

**Flexible Mode (permissive, Go-interop):**
```json
{
  "language": {
    "lambda_syntax": "both",
    "operator_precedence": "standard",
    "safe_navigation_unwrap": "smart"
  },
  "transpiler": {
    "null_coalescing_pointers": true
  }
}
```

---

## 7. Implementation Roadmap (Updated)

### 7.1 Phase 0: Configuration System (Week 1, Days 1-2)

**Tasks:**
- [ ] Create `pkg/config/` package
- [ ] Implement config loader (JSON parsing)
- [ ] Define all configuration structs
- [ ] Add default values and validation
- [ ] Integrate config into Parser and Context
- [ ] Write config loading tests
- **Estimate:** 1-2 days

### 7.2 Phase 1: Foundation (Week 1, Days 3-7)

**Null Coalescing (with pointer support):**
- [ ] Add lexer token for `??`
- [ ] Update parser grammar
- [ ] Implement `NullCoalescingPlugin` with config modes
- [ ] Support both Option[T] and *T based on config
- [ ] Write tests (both types, mixed chaining)
- **Estimate:** 2-3 days

**Ternary (with precedence modes):**
- [ ] Disambiguate `?` in lexer
- [ ] Add ternary expression to parser
- [ ] Implement precedence checking (standard vs explicit)
- [ ] Implement `TernaryPlugin`
- [ ] Statement lifting for expression contexts
- [ ] Write tests (simple, chained, with precedence errors)
- **Estimate:** 2-3 days

### 7.3 Phase 2: Safe Navigation (Week 2)

**Safe Navigation (with smart unwrapping):**
- [ ] Add `SafeNavigationExpr` AST node
- [ ] Update lexer (distinguish `?.` from `?`)
- [ ] Parser support for chaining
- [ ] Implement `SafeNavigationPlugin` with unwrap modes
- [ ] Context-based type inference for smart mode
- [ ] Optimize nested conditions
- [ ] Integration with Option[T]
- [ ] Write tests (always_option vs smart modes)
- **Estimate:** 3-4 days

### 7.4 Phase 3: Lambda Functions (Week 3)

**Lambda (all three styles):**
- [ ] Implement Rust-style parser (`|params| body`)
- [ ] Implement Arrow-style parser (`params => body`)
- [ ] Config-based grammar switching
- [ ] Implement `LambdaPlugin`
- [ ] Type inference for parameters
- [ ] Return type inference
- [ ] Block vs expression bodies
- [ ] Write tests (all styles, mixed usage)
- **Estimate:** 4-5 days

### 7.5 Phase 4: Integration & Testing (Week 4)

**Tasks:**
- [ ] Cross-feature testing (all combinations)
- [ ] Configuration validation tests
- [ ] Preset configurations (strict, flexible, etc.)
- [ ] Precedence interaction tests
- [ ] Pointer + Option integration tests
- [ ] Performance benchmarks
- [ ] Documentation updates
- [ ] Migration guide with config examples
- **Estimate:** 4-5 days

**Total Estimate:** 4 weeks for all features with full configurability

---

## 8. Configuration-Specific Testing Strategy

### 8.1 Configuration Matrix Tests

**Test all combinations:**

```go
// tests/config_matrix_test.go
func TestConfigMatrix(t *testing.T) {
    configs := []*config.Config{
        strictConfig(),
        flexibleConfig(),
        defaultConfig(),
    }

    testCases := []string{
        "safe_nav_basic.dingo",
        "null_coalesce_pointer.dingo",
        "ternary_precedence.dingo",
        "lambda_mixed_styles.dingo",
    }

    for _, cfg := range configs {
        for _, tc := range testCases {
            t.Run(fmt.Sprintf("%s_%s", cfg.Name, tc), func(t *testing.T) {
                result := transpileWithConfig(tc, cfg)
                verify(t, result, cfg)
            })
        }
    }
}
```

### 8.2 Golden Files Per Config

**Organize by configuration:**

```
tests/golden/
├── strict/
│   ├── safe_nav_01.dingo
│   ├── safe_nav_01.go.golden
│   └── ...
├── flexible/
│   ├── safe_nav_01.dingo
│   ├── safe_nav_01.go.golden  # Different output!
│   └── ...
└── default/
    └── ...
```

### 8.3 Configuration Validation Tests

```go
func TestConfigValidation_Conflicts(t *testing.T) {
    // Test: Smart unwrap requires source maps
    cfg := &config.Config{
        Language: config.LanguageConfig{
            SafeNavigationUnwrap: config.SafeNavUnwrapSmart,
        },
        Transpiler: config.TranspilerConfig{
            GenerateSourceMaps: false,  // Conflict!
        },
    }

    err := validateConfig(cfg)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "smart unwrap requires source maps")
}
```

---

## 9. Documentation Impact

### 9.1 Configuration Documentation

**New file:** `docs/configuration.md`

```markdown
# Dingo Configuration

## Configuration File

Create `.dingorc` or `dingo.config.json` in your project root.

## Settings Reference

### language.lambda_syntax
Controls which lambda syntax styles are accepted.

- `"rust"`: Only `|x| expr`
- `"arrow"`: Only `(x) => expr`
- `"both"`: Accept both (default)

**Example:**
```dingo
// With lambda_syntax = "rust"
add := |a, b| a + b  ✓
add := (a, b) => a + b  ✗ ERROR
```

### language.operator_precedence
Controls precedence checking strictness.

- `"standard"`: Follow C/TypeScript (default)
- `"explicit"`: Require parentheses for mixing

**Example:**
```dingo
// With operator_precedence = "explicit"
x := a ?? b ? c : d  ✗ ERROR: Use ()
x := (a ?? b) ? c : d  ✓
```

### language.safe_navigation_unwrap
Controls how `?.` handles return types.

- `"always_option"`: Always returns Option[T]
- `"smart"`: Unwraps based on context (default)

[... continue for all settings ...]
```

### 9.2 Migration Guide Updates

**New section:** Choosing configurations

```markdown
## Migrating from Go to Dingo: Configuration Guide

### For Strict Type Safety (Rust-like)
Use this config for maximum type safety:
```json
{
  "language": {
    "safe_navigation_unwrap": "always_option",
    "operator_precedence": "explicit"
  }
}
```

### For Go Interoperability
Use this config when integrating with existing Go code:
```json
{
  "transpiler": {
    "null_coalescing_pointers": true
  }
}
```

### For Team Flexibility
Allow both lambda styles for developer preference:
```json
{
  "language": {
    "lambda_syntax": "both"
  }
}
```
```

---

## 10. Success Criteria (Updated)

### 10.1 Feature Completeness
- [ ] All four operators implemented with all config modes
- [ ] Configuration system fully functional
- [ ] All config combinations tested
- [ ] Golden tests pass for all modes

### 10.2 Configuration Quality
- [ ] Config validation catches conflicts
- [ ] Clear error messages for invalid configs
- [ ] Defaults are sensible and documented
- [ ] Presets available for common use cases

### 10.3 Code Quality
- [ ] Generated Go code varies correctly by config
- [ ] No runtime overhead (zero-cost abstraction)
- [ ] Proper error messages for violations
- [ ] Source maps track all transformations

### 10.4 Developer Experience
- [ ] Config file is intuitive and self-documenting
- [ ] Parser errors indicate config-related issues
- [ ] Documentation covers all configuration options
- [ ] Examples provided for each config mode

### 10.5 Performance
- [ ] Config loading is fast (< 1ms)
- [ ] No transpilation overhead from config checks
- [ ] Generated code quality independent of config complexity

---

## 11. Summary

This final plan incorporates all user preferences:

1. **Configuration System**
   - JSON-based config file (`.dingorc` or `dingo.config.json`)
   - Validation and defaults
   - Preset configurations for common scenarios

2. **Null Coalescing (`??`)**
   - Works with both `Option[T]` and Go pointers `*T`
   - Configurable via `null_coalescing_pointers` setting

3. **Safe Navigation (`?.`)**
   - Smart unwrapping based on context (default)
   - Optional always-Option mode
   - Configurable via `safe_navigation_unwrap` setting

4. **Ternary (`? :`)**
   - Standard precedence (C/TypeScript-like) or explicit mode
   - Configurable via `operator_precedence` setting

5. **Lambda Functions**
   - Supports Rust-style, Arrow-style, or both
   - Configurable via `lambda_syntax` setting

**Key Architectural Additions:**
- New `pkg/config/` package for configuration management
- Config passed to Parser and all Plugins via Context
- Validation pipeline ensures config consistency
- Matrix testing for all configuration combinations

**Estimated Timeline:** 4 weeks for all features with full configurability

**Next Steps:**
1. Implement configuration system (Phase 0)
2. Begin feature implementation with config support
3. Continuous integration testing across configs
4. Documentation and migration guides
