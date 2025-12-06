# Parser Integration Test Suite

**Created**: 2025-12-05
**Status**: Complete - Test infrastructure implemented
**Coverage**: Mode comparison, golden test integration, error recovery, incremental parsing

## Overview

The `parser_integration_test.go` file provides comprehensive integration tests for verifying the new AST-based parser produces equivalent output to the legacy preprocessor pipeline. These tests will track migration progress and catch regressions.

## Test Categories

### 1. Mode Comparison Tests (`TestModeComparison`)

**Purpose**: Verify AST mode produces equivalent output to Legacy mode

**Test Cases**:
- ✅ Simple error propagation (`?` operator)
- ⏸️ Null coalesce (`??`) - AST mode pending
- ⏸️ Safe navigation (`?.`) - AST mode pending
- ⏸️ Lambda expressions - AST mode pending
- ⏸️ Enum declarations - AST mode pending

**Current Status**:
- Infrastructure complete
- AST mode incomplete (expected - Phase Z in progress)
- Tests correctly identify differences between modes

**Example Output**:
```
=== FAIL: TestModeComparison/simple_error_propagation
    Mode outputs differ:
      Legacy: Full transpiled Go code
      AST:    Partial/incomplete (expected)
```

### 2. Golden Test Integration (`TestGoldenTestsWithASTMode`)

**Purpose**: Run existing golden tests with AST mode to track compatibility

**Test Files Validated**:
- `error_prop_01_simple.dingo`
- `lambda_01_typescript_basic.dingo`
- `ternary_01_basic.dingo`

**Current Results**:
```
=== AST Mode Golden Test Results ===
Pass: 0/3 (0.0%)
Fail: 3
Skip: 0
```

**Expected**: Pass rate will increase as AST mode implementation progresses

### 3. Error Recovery Tests (`TestErrorRecovery`)

**Purpose**: Verify parser produces partial AST for LSP support even when errors occur

**Test Scenarios**:
- ❌ Incomplete function signature (mid-typing)
- ❌ Incomplete error propagation expression
- ⚠️ Syntax error in lambda (partial success)
- ❌ Missing closing brace

**Current Status**:
- Tests identify that current parser does NOT produce partial AST
- This is a critical requirement for LSP support
- Tests serve as acceptance criteria for error recovery implementation

**Expected Behavior**:
```go
// Incomplete source (mid-typing)
source := `package main

func test() {
    x :=
`

// Parser should return:
// - error (parse failed)
// - *ast.File (partial AST for LSP)
file, err := parser.ParseFile(fset, "test.dingo", source, parser.ParseComments)
assert.Error(t, err)          // Has errors
assert.NotNil(t, file)        // But still returns partial AST
```

### 4. Incremental Parsing Tests (`TestIncrementalParsing`)

**Purpose**: Verify incremental parsing is faster than full re-parse

**Current Status**: Skipped in short mode (performance test)

**Test Approach**:
1. Generate large source file (1000 functions)
2. Measure full parse time
3. Make small edit (add 1 function)
4. Measure incremental parse time
5. Verify incremental < 50% of full parse time

**Note**: Currently measures baseline; incremental optimization not yet implemented

### 5. Mixed Features Test (`TestMixedFeatures`)

**Purpose**: Test files using multiple Dingo features simultaneously

**Test Source**:
```dingo
package main

import "os"

enum Status {
    Success,
    Failure
}

func processFile(path: string) Result<string, error> {
    data := os.ReadFile(path)?
    content := string(data)

    status := path != "" ? Status.Success : Status.Failure

    match status {
        Success => return Ok(content),
        Failure => return Err("invalid path")
    }
}
```

**Current Status**: Skipped until AST mode fully implemented

### 6. Hybrid Mode Test (`TestHybridMode`)

**Purpose**: Verify hybrid mode correctly falls back from AST to Legacy

**Status**: ✅ PASS

**Behavior**:
- Attempts AST transpilation first
- Falls back to Legacy on error
- Produces valid Go output via fallback

### 7. Partial AST for LSP Test (`TestPartialASTForLSP`)

**Purpose**: Verify incomplete code produces usable AST for LSP operations

**Current Status**: ❌ FAIL (parser returns nil on errors)

**Expected Fix**: Parser should return partial AST even when errors occur

### 8. Config Propagation Test (`TestConfigPropagation`)

**Purpose**: Verify config settings are respected in both modes

**Status**: ✅ PASS

## Helper Functions

### `transpileWithMode(source, filename, mode) ([]byte, error)`

Routes transpilation to Legacy or AST pipeline based on mode.

**Modes**:
- `ModeLegacy`: Preprocessor-based (current production)
- `ModeAST`: New AST parser (under development)
- `ModeHybrid`: AST first, fallback to Legacy

### `compareOutputs(legacy, ast) (bool, []Diff)`

Compares normalized Go code from both modes.

**Normalization**:
- `gofmt` formatting
- Whitespace normalization
- Line-by-line diff generation

### `normalizeGoCode(code) string`

Applies `go/format` and whitespace normalization for comparison.

### `generateLargeSource(numFunctions) string`

Generates test files for performance benchmarking.

## Benchmarks

### `BenchmarkLegacyVsAST`

Compares transpilation performance between modes.

**Current Status**: Skipped (AST mode incomplete)

**Expected Use**: Track performance as AST mode matures

## Test Results Summary

| Test Category | Status | Pass Rate | Notes |
|--------------|--------|-----------|-------|
| Mode Comparison | ⚠️ Partial | 0/5 skip | AST mode incomplete (expected) |
| Golden Integration | 📊 Tracking | 0/3 (0%) | Baseline established |
| Error Recovery | ❌ Fail | 0/4 | Parser needs partial AST support |
| Hybrid Mode | ✅ Pass | 1/1 | Fallback working correctly |
| Partial AST | ❌ Fail | 0/1 | Critical for LSP |
| Config Propagation | ✅ Pass | 1/1 | Config system working |

**Overall**: Infrastructure complete, tests ready to track AST parser development

## Key Findings

### Critical Requirements for AST Parser

1. **Error Recovery**: Parser MUST return partial AST even when errors occur
   - Required for LSP support (autocomplete, navigation, etc.)
   - Current parser returns `nil` on errors
   - Tests: `TestErrorRecovery`, `TestPartialASTForLSP`

2. **Incremental Parsing**: Performance optimization for LSP
   - Tests ready to measure improvement
   - Not yet implemented

3. **Feature Parity**: AST mode must match Legacy output
   - Tests track compatibility via golden file comparison
   - Current: 0% parity (AST mode incomplete)
   - Target: 100% parity before migration

## Usage

### Run All Integration Tests

```bash
# Full test suite
go test ./tests -run "TestModeComparison|TestGoldenTestsWithASTMode|TestErrorRecovery|TestHybridMode|TestPartialASTForLSP|TestConfigPropagation" -v

# Short mode (skip performance tests)
go test ./tests -run "TestModeComparison|TestGoldenTestsWithASTMode|TestErrorRecovery|TestHybridMode|TestPartialASTForLSP|TestConfigPropagation" -v -short
```

### Run Specific Test Category

```bash
# Mode comparison
go test ./tests -run TestModeComparison -v

# Golden test integration
go test ./tests -run TestGoldenTestsWithASTMode -v

# Error recovery
go test ./tests -run TestErrorRecovery -v
```

### Run Benchmarks

```bash
go test ./tests -bench BenchmarkLegacyVsAST -benchmem
```

## Migration Progress Tracking

As AST parser development continues, update this section:

### Phase Z Progress

- [ ] Error recovery (partial AST on errors)
- [ ] Error propagation (`?`)
- [ ] Null coalesce (`??`)
- [ ] Safe navigation (`?.`)
- [ ] Lambda expressions
- [ ] Enum declarations
- [ ] Match expressions
- [ ] Incremental parsing optimization

**Golden Test Pass Rate**: 0/3 (0.0%)
**Mode Comparison Pass Rate**: 0/5 (all skipped - AST incomplete)

### Acceptance Criteria

Before switching to AST mode as default:

- ✅ All mode comparison tests pass (5/5)
- ✅ Golden test pass rate ≥ 95% (equivalent to Legacy mode)
- ✅ Error recovery tests pass (4/4) - partial AST on errors
- ✅ Incremental parsing shows ≥2x speedup on edits
- ✅ Mixed features test passes

## References

- **Implementation**: `tests/parser_integration_test.go`
- **AST Pipeline**: `pkg/transpiler/ast_pipeline.go`
- **Mode Selection**: `pkg/transpiler/mode.go`
- **Legacy Pipeline**: `pkg/transpiler/transpiler.go`
- **Golden Tests**: `tests/golden_test.go`
- **Parser**: `pkg/parser/pratt.go`

## Next Steps

1. **Implement Error Recovery**: Parser should return partial AST + errors
2. **Complete AST Mode**: Implement missing features (lambdas, enums, etc.)
3. **Track Progress**: Monitor golden test pass rate as features are added
4. **Performance Optimization**: Implement incremental parsing for LSP
5. **Migration**: Switch default mode when parity reaches 100%

---

**Last Updated**: 2025-12-05
**Test Suite Version**: 1.0
**Parser Phase**: Z (AST Migration - In Progress)
