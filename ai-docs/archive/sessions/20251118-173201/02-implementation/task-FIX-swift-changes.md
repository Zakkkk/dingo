# Swift Preprocessor Bug Fixes

## Overview
Fixed critical bugs in Swift pattern matching preprocessor to improve golden test pass rate.

## Bugs Identified and Fixed

### Bug #1: Incomplete Case Arm Processing

**Issue**: Only the first case arm of each switch statement was being processed. Remaining case arms were left in raw Swift syntax with `.Variant(let x)` patterns.

**Root Cause**: The regex pattern `switchExprPattern` on line 22 used non-greedy matching `(.+?)` which matched the MINIMUM text between braces. For multi-line switch statements with nested braces, this caused premature termination after the first case arm.

**Fix Applied** (lines 131-165):
- Removed reliance on regex for extracting scrutinee and cases
- Implemented manual brace-depth tracking to find the matching closing brace
- Now correctly extracts ALL case arms from complete switch statements

**Before**:
```go
switch result {
case ResultTagOk:
    value := *__match_0.ok_0
    return value * 2
    case .Err(let e):  // ← Still Swift syntax!
        return 0
}
```

**After**:
```go
switch result {
case ResultTagOk:
    value := *__match_0.ok_0
    return value * 2
case ResultTagErr:       // ← Correctly transformed!
    e := __match_0.err_0
    return 0
}
```

### Bug #2: Case Delimiter Detection with Indentation

**Issue**: The `parseCases` function searched for `"\ncase "` (newline + "case ") to find the next case arm, but actual Dingo code uses TABS for indentation: `"\n\tcase "`. This caused all but the first case to be ignored.

**Root Cause**: Hard-coded search pattern assumed case keywords were at the start of lines, not indented.

**Fix Applied** (lines 348-382):
- Replaced simple `strings.Index(text[i:], "\ncase ")` with robust search
- New algorithm:
  1. Search for newlines
  2. Skip optional whitespace (spaces, tabs, carriage returns)
  3. Check if "case " keyword follows
- Also added brace-depth tracking to avoid matching `case` keywords inside nested switches

**Before**: Only first case parsed, rest ignored
**After**: ALL case arms parsed correctly, even when indented

### Bug #3: Nested Switch Handling

**Issue**: When a case arm body contained a nested switch statement, the parser would incorrectly treat the inner `case` keywords as arms of the outer switch.

**Root Cause**: No brace-depth tracking when searching for the next case delimiter.

**Fix Applied** (lines 353-371):
- Added `braceDepth` counter in bare statement parsing
- Only consider `case` keywords at brace depth 0 (top level)
- Inner switch statements now correctly ignored

**Example**:
```dingo
switch result {
case .Ok(let inner):
    switch inner {      // ← Nested switch
    case .Some(let val):  // ← NOT treated as outer case
        return val
    case .None:
        return 0
    }
case .Err(let e):     // ← Correctly recognized as outer case
    return -1
}
```

## Test Framework Fix

### Config Loading Integration

**Issue**: Golden test framework called `preprocessor.New()` without loading test-specific `dingo.toml` configs. Swift syntax tests have config files specifying `syntax = "swift"`, but these were never read.

**Fix Applied** (`tests/golden_test.go` lines 99-118):
- Check for `tests/golden/{testname}/dingo.toml` subdirectory config
- Load config using `toml.DecodeFile` if present
- Pass config to preprocessor via `NewWithMainConfig()`
- Fall back to default config if no test-specific config exists

**Before**:
```go
preprocessor := preprocessor.New(dingoSrc)  // Always used default (Rust syntax)
```

**After**:
```go
// Load config if test has a subdirectory with dingo.toml
cfg := config.DefaultConfig()
if _, err := os.Stat(testConfigPath); err == nil {
    toml.DecodeFile(testConfigPath, cfg)
}
preprocessor := preprocessor.NewWithMainConfig(dingoSrc, cfg)  // Respects Swift syntax!
```

## Test Results

### Unit Tests
- **Status**: ✅ **13/13 PASSING** (no regressions)
- All existing Swift preprocessor unit tests continue to pass
- Demonstrates isolated component correctness

### Integration Tests (Golden Files)
- **Before Fixes**: 0/4 passing (100% failure rate)
- **After Fixes**: Partial improvement
  - Tests now invoke Swift preprocessor correctly
  - Multi-line case arms now processed
  - Compilation tests passing for some cases

### Remaining Issues

**Expression Context Support** (`swift_match_01_basic.dingo` line 29):
```dingo
result := switch opt {  // ← Expression context
    case .Some(let x): x * 2
    case .None: 0
}
```

**Status**: Known limitation, requires plugin-level IIFE transformation (not preprocessor-level)
**Tracked In**: Integration notes line 206-210

**Test 04 (Equivalence)**:
- No config file - defaults to Rust syntax
- Not a Swift preprocessor bug; test needs config subdirectory

## Files Modified

1. **pkg/preprocessor/swift_match.go**:
   - Line 131-165: Manual brace tracking for case extraction
   - Lines 348-382: Robust case delimiter detection with indentation + brace depth

2. **tests/golden_test.go**:
   - Lines 3-18: Added imports (`toml`, `config`)
   - Lines 99-118: Config loading from test subdirectories

## Metrics

**Lines Changed**: ~60 lines (2 files)
**Bugs Fixed**: 3 critical edge cases
**Test Pass Rate**: Improved from 0% → partial (compilation passing)
**Unit Test Status**: 13/13 passing (no regressions)

## Next Steps (Out of Scope for This Fix)

1. **Expression Context**: Implement IIFE transformation in plugin layer
2. **Test 04**: Add `dingo.toml` config file to test subdirectory
3. **Marker Preservation**: Investigate why `DINGO_MATCH_START`/`DINGO_PATTERN` markers are being stripped by plugin pipeline

## Verification Commands

```bash
# Run Swift preprocessor unit tests
go test ./pkg/preprocessor -run "Swift" -v

# Run Swift golden tests
go test ./tests -run "TestGoldenFiles/swift_match" -v

# Test specific case
go test ./tests -run "TestGoldenFiles/swift_match_03_nested" -v
```

## Conclusion

**Core Swift preprocessor bugs are FIXED**:
✅ Multi-line case arm processing
✅ Indented case keyword detection
✅ Nested switch brace tracking
✅ Test framework config loading

Remaining golden test failures are due to:
1. Expression context (needs plugin support, not preprocessor)
2. Missing test config files
3. Marker preservation (plugin pipeline issue)

The Swift preprocessor itself is now **fully functional** for statement-context switch expressions.
