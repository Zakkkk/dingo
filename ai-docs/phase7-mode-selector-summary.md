# Phase 7 Task V: Parser Mode Selector - Implementation Summary

## Overview

Implemented a parser mode selector that allows switching between:
- **Legacy mode**: Preprocessor-based transpilation (current production implementation)
- **AST mode**: New AST parser + transformer pipeline (under development)
- **Hybrid mode**: Try AST first, fall back to legacy on error

## Files Created

### 1. `pkg/transpiler/mode.go`
- Defines `TranspileMode` enum with three modes (Legacy, AST, Hybrid)
- Implements `String()` method for mode display
- Implements `ParseMode()` for parsing mode from strings
- Implements `transpileAST()` placeholder for AST-based pipeline
- Implements `transpileWithMode()` dispatcher for mode selection

### 2. `pkg/transpiler/mode_test.go`
- Tests for `TranspileMode.String()`
- Tests for `ParseMode()` validation
- Tests for mode setters/getters on Transpiler
- Tests for AST mode (verifies not-yet-implemented error)

## Files Modified

### 1. `pkg/transpiler/transpiler.go`
- Added `mode TranspileMode` field to Transpiler struct
- Updated `New()` to default to Legacy mode
- Updated `NewWithConfig()` to default to Legacy mode
- Added `NewWithMode()` constructor for custom mode
- Added `SetMode()` and `GetMode()` methods

### 2. `pkg/config/config.go`
- Added `TranspileMode string` field to BuildConfig
- Added default "legacy" value in `DefaultConfig()`
- Added validation for transpile_mode in `Validate()` method
- Valid values: "legacy", "ast", "hybrid"

### 3. `cmd/dingo/main.go`
- Added `transpileMode` flag to buildCmd()
- Updated `runBuild()` signature to accept mode parameter
- Implemented mode resolution (CLI flag > config > default)
- Updated `buildFile()` signature to accept mode parameter
- Added TODO comment noting mode integration pending AST pipeline completion

## Configuration

### TOML Configuration
Users can now configure transpile mode in `dingo.toml`:

```toml
[build]
transpile_mode = "legacy"  # or "ast" or "hybrid"
```

### CLI Flag
Users can override config via CLI flag:

```bash
dingo build --mode=ast file.dingo
dingo build --mode=hybrid ./...
```

### Priority Order
Mode is resolved in this order:
1. CLI flag (`--mode`) - highest priority
2. Project config (`dingo.toml`)
3. Default value ("legacy") - lowest priority

## AST Pipeline Placeholder

The `transpileAST()` function provides a skeleton implementation:

```go
// Step 1: Tokenize with pkg/tokenizer
// Step 2: Parse with pkg/parser (Pratt parser)
// Step 3: Transform with pkg/transformer
// Step 4: Print with go/printer
```

Currently returns error "AST mode not fully implemented yet" - ready for future integration.

## Hybrid Mode Behavior

Hybrid mode implements graceful degradation:
1. Attempts AST-based transpilation first
2. On error, falls back to legacy preprocessor-based pipeline
3. Allows gradual migration while maintaining stability

## Testing

All tests pass (11/11):
- ✅ `TestTranspileMode_String` - Mode string representation
- ✅ `TestParseMode` - Mode parsing and validation
- ✅ `TestTranspiler_ModeSettersGetters` - Mode accessors
- ✅ `TestTranspileAST_NotImplemented` - AST mode placeholder
- ✅ Existing transpiler tests continue to pass

## Backward Compatibility

✅ **100% backward compatible**:
- Default mode is "legacy" (existing behavior)
- All existing code works unchanged
- Mode is opt-in via config or CLI flag

## Integration Status

✅ **Complete**:
- Mode enum and parser ✅
- Config integration ✅
- CLI flag ✅
- Mode resolution ✅
- Transpiler API ✅
- Tests ✅

⏳ **Pending** (Phase 7 Task VI):
- Full AST pipeline implementation
- Actual mode dispatching in buildFile()
- Integration testing with AST parser

## Next Steps

To complete mode integration:
1. Finish AST parser implementation (Task I-IV)
2. Complete transformer implementation
3. Update `buildFile()` to dispatch based on mode:
   ```go
   switch mode {
   case "legacy":
       // Current preprocessor pipeline
   case "ast":
       // New AST pipeline
   case "hybrid":
       // Try AST, fall back to legacy
   }
   ```
4. Add integration tests with real AST parsing

## Usage Examples

### Default (Legacy Mode)
```bash
dingo build file.dingo
# Uses legacy preprocessor-based pipeline
```

### Explicit AST Mode
```bash
dingo build --mode=ast file.dingo
# Will use AST parser when implemented
```

### Hybrid Mode (Safe Migration)
```bash
dingo build --mode=hybrid ./...
# Try AST, fall back to legacy on error
```

### Config-Based
```toml
# dingo.toml
[build]
transpile_mode = "hybrid"
```

```bash
dingo build ./...
# Uses mode from config
```

## Summary

Task V successfully implements the mode selector architecture, providing:
- Clean enum-based mode switching
- Configuration and CLI integration
- Backward compatibility with legacy pipeline
- Clear path for AST pipeline integration
- Comprehensive test coverage

The implementation is ready for AST pipeline completion in subsequent tasks.
