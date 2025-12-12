# Dingo Configuration Guide

Dingo uses a TOML-based configuration system with clear precedence rules for flexibility.

## Configuration Files

### Project Configuration (`dingo.toml`)

Place `dingo.toml` in your project root:

```toml
[features]
# Error propagation syntax: "question" | "bang" | "try"
error_propagation_syntax = "question"

[sourcemaps]
# Enable source map generation
enabled = true

# Source map format: "inline" | "separate" | "both" | "none"
format = "inline"
```

### User Configuration (`~/.dingo/config.toml`)

Global defaults for all your projects:

```toml
[features]
error_propagation_syntax = "question"

[sourcemaps]
enabled = true
format = "inline"
```

## Configuration Precedence

Dingo applies configuration in this order (highest to lowest priority):

1. **CLI flags** (e.g., `--syntax=bang`)
2. **Project `dingo.toml`** (current directory)
3. **User config** (`~/.dingo/config.toml`)
4. **Built-in defaults**

### Example

```bash
# Uses project dingo.toml
dingo build main.dingo

# Overrides project config with CLI flag
dingo build --syntax=try main.dingo
```

## Feature Configuration

### Error Propagation Syntax

Controls which error propagation syntax to use.

**Option**: `features.error_propagation_syntax`

**Values**:
- `"question"` - Use `expr?` operator (default)
- `"bang"` - Use `expr!` operator
- `"try"` - Use `try expr` keyword

**Example**:

```toml
[features]
error_propagation_syntax = "question"
```

**CLI Override**:

```bash
dingo build --syntax=bang main.dingo
```

## Source Map Configuration

### Enable Source Maps

**Option**: `sourcemaps.enabled`

**Values**:
- `true` - Generate source maps (default)
- `false` - Disable source maps

**Example**:

```toml
[sourcemaps]
enabled = true
```

### Source Map Format

**Option**: `sourcemaps.format`

**Values**:
- `"inline"` - Embed source maps as base64 comments in `.go` files (default)
- `"separate"` - Write source maps to `.go.map` files
- `"both"` - Generate both inline and separate files
- `"none"` - Disable source maps (same as `enabled = false`)

**Example**:

```toml
[sourcemaps]
format = "inline"
```

**Recommendations**:
- **Development**: Use `"inline"` for convenience
- **Production**: Use `"separate"` for cleaner generated code
- **CI/CD**: Use `"both"` to support multiple workflows

## Complete Configuration Example

```toml
# dingo.toml
# Dingo Project Configuration

[features]
# Use question mark operator for error propagation
error_propagation_syntax = "question"

[sourcemaps]
# Generate source maps for better error messages
enabled = true

# Use inline source maps during development
format = "inline"
```

## CLI Reference

Dingo CLI mirrors Go's compiler structure with three main commands:

### Build Command (like go build)

```bash
dingo build [flags] <files/packages...>
```

Transpiles `.dingo` files and compiles to a binary.

**Flags**:
- `--verbose` - Show the go build command being executed
- `--no-mascot` - Disable animated mascot (for CI/scripts)
- `-o <path>` - Output binary path
- All Go build flags are passed through (`-race`, `-ldflags`, etc.)

**Examples**:

```bash
# Build single file
dingo build main.dingo

# Build with output name
dingo build -o myapp main.dingo

# Build package
dingo build ./cmd/myapp

# With race detector
dingo build -race main.dingo

# With linker flags
dingo build -ldflags="-s -w" main.dingo

# Verbose mode (shows go build command)
dingo build --verbose main.dingo
```

### Run Command (like go run)

```bash
dingo run [flags] <files/packages...> [-- program args]
```

Transpiles and runs immediately. Program output goes directly to stdout (no mascot animation).

**Examples**:

```bash
# Run single file
dingo run main.dingo

# Run package
dingo run ./cmd/myapp

# Pass arguments to program
dingo run main.dingo -- --port 8080 --debug

# With race detector
dingo run -race main.dingo
```

### Go Command (transpile only)

```bash
dingo go [flags] <files...>
```

Transpiles `.dingo` files to `.go` files without compilation. This is the original behavior.

**Flags**:
- `--syntax <style>` - Error propagation syntax (question|bang|try)
- `--no-sourcemaps` - Disable source map generation
- `--sourcemap-format <format>` - Source map format (inline|separate|both|none)
- `-o, --output <file>` - Output file path

**Examples**:

```bash
# Transpile single file
dingo go main.dingo

# Transpile multiple files
dingo go file1.dingo file2.dingo

# Override syntax
dingo go --syntax=try main.dingo

# Disable source maps
dingo go --no-sourcemaps main.dingo

# Custom output with separate source maps
dingo go -o build/main.go --sourcemap-format=separate main.dingo
```

## Validation

Dingo validates your configuration on startup. Invalid configurations will produce clear error messages:

```bash
$ dingo build main.dingo
Error: invalid configuration: invalid error_propagation_syntax: "unknown"
       (must be 'question', 'bang', or 'try')
```

## Environment Variables

Currently, Dingo does not support environment variable configuration. Use CLI flags or config files instead.

## Default Values

If no configuration is provided, Dingo uses these defaults:

```toml
[features]
error_propagation_syntax = "question"

[sourcemaps]
enabled = true
format = "inline"
```

## Migration Guide

### From No Configuration (v0.1.0-alpha)

If you're upgrading from an early version without configuration support:

1. **No action required** - Defaults match previous behavior
2. **Optional**: Create `dingo.toml` to customize settings
3. **Optional**: Create `~/.dingo/config.toml` for personal defaults

### Changing Syntax

To migrate from one syntax to another:

1. Update `dingo.toml`:
   ```toml
   [features]
   error_propagation_syntax = "try"  # Changed from "question"
   ```

2. Update your `.dingo` files:
   ```dingo
   # Before
   user := fetchUser()?

   # After
   user := try fetchUser()
   ```

3. Rebuild:
   ```bash
   dingo build *.dingo
   ```

## Troubleshooting

### Config Not Loading

**Symptoms**: CLI uses defaults instead of `dingo.toml`

**Solutions**:
- Ensure `dingo.toml` is in the current directory
- Check for TOML syntax errors: `dingo config validate`
- Verify file permissions

### Conflicting Configurations

**Symptoms**: Unexpected behavior with multiple config sources

**Solutions**:
- Check precedence order (CLI > project > user > defaults)
- Use `dingo config show` to see effective configuration
- Remove `~/.dingo/config.toml` if causing conflicts

## See Also

- [Error Propagation Feature](features/error-propagation.md)
- [Source Maps Specification](../ai-docs/sourcemap-spec.md)
- [CLI Reference](cli-reference.md)
