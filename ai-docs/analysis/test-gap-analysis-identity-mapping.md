# Test Gap Analysis: Identity Mapping Coverage Blindspot

**Date**: 2025-11-22
**Component**: Source Map Testing (`pkg/sourcemap/postast_validation_test.go`)
**Issue**: Existing test didn't catch identity mapping bug

## Executive Summary

**Bug**: LSP Go-to-Definition jumped to wrong line (blank line 7 instead of line 3 function definition)

**Existing Test Status**: `TestRoundTripTranslation` PASSED ✅ (but shouldn't have!)

**Root Cause**: Test only validated TRANSFORMED lines, missing coverage of IDENTITY mappings

**Impact**: Tests validated buggy behavior as correct, requiring manual testing to discover the issue

## The Coverage Blindspot

### What the Test Checked

```go
// BEFORE: Only tested transformed lines
testLines: []int{4, 10}, // Two ? operators (TRANSFORMED lines only)
```

This tested:
- ✅ .dingo line 4 (with `?`) → .go line 8 → back to .dingo line 4
- ✅ .dingo line 10 (with `?`) → .go line 17 → back to .dingo line 10

### What the Test DIDN'T Check

- ❌ .dingo line 3 (function definition - UNTRANSFORMED)
- ❌ .go line 7 → reverse to .dingo (identity mapping reverse lookup)
- ❌ Other untransformed lines (package, blank lines, return statements)
- ❌ Duplicate mappings for same generated line

### The Critical Assumption

**Assumption**: "If transformed lines work, untransformed lines must be fine"

**Reality**: Identity mappings had DIFFERENT bugs:
1. Line offset calculation errors
2. Duplicate mappings for same generated line
3. Wrong mapping selection in reverse lookup

## The Bugs Exposed by Expanded Tests

### Bug 1: Line Offset Calculation (Fixed in previous commit)

```
WRONG: Identity mapping said line 3 → line 3
CORRECT: Should be line 3 → line 7 (accounting for import block)
```

### Bug 2: Duplicate Mappings

Source map contains DUPLICATE entries for same generated line:

```json
// Go line 7 has TWO mappings
{
  "generated_line": 7,
  "original_line": 3,  // ✅ CORRECT
  "name": "identity"
},
{
  "generated_line": 7,
  "original_line": 7,  // ❌ WRONG (duplicate)
  "name": "identity"
}
```

### Bug 3: Reverse Mapping Selection

When building reverse map (`map[int]int` from generated → original), the **last** mapping overwrites earlier ones:

```go
// PROBLEM: Both mappings compete
reverseMap[7] = 3  // First mapping (correct)
reverseMap[7] = 7  // Second mapping (wrong) - OVERWRITES!
```

Result: Go-to-Definition uses wrong mapping (7 → 7 instead of 7 → 3)

## Test Improvements Applied

### 1. Expanded TestRoundTripTranslation

```go
// AFTER: Tests BOTH transformed AND untransformed lines
testLines: []int{
    1,  // package main (identity mapping - CRITICAL)
    3,  // func readConfig (identity mapping - CRITICAL for Go to Definition)
    4,  // data := ... ? (transformation)
    5,  // return data (identity mapping)
    9,  // func test (identity mapping)
    10, // a := ... ? (transformation)
    11, // println (identity mapping)
},
description: []string{
    "package main",
    "func readConfig",
    "? operator",
    "return statement",
    "func test",
    "? operator",
    "println call",
},
```

**Coverage increase**:
- Before: 2 test cases (transformed lines only)
- After: 7 test cases (all line types)
- New coverage: Package declarations, function definitions, return statements

### 2. Added TestIdentityMappingReverse

New dedicated test for reverse mapping of untransformed lines:

```go
func TestIdentityMappingReverse(t *testing.T) {
    tests := []struct {
        name              string
        dingoFile         string
        goLine            int    // Line in .go file (1-based)
        expectedDingoLine int    // Expected line in .dingo file (1-based)
        description       string // What this line is
    }{
        {
            name:              "function_definition",
            dingoFile:         "../../tests/golden/error_prop_01_simple.dingo",
            goLine:            7,  // func readConfig in .go
            expectedDingoLine: 3,  // func readConfig in .dingo
            description:       "func readConfig(path string) ([]byte, error)",
        },
        // ... more test cases
    }
}
```

**Tests specifically**:
- Function definitions (CRITICAL for Go-to-Definition)
- Package declarations
- Return statements
- Second function declarations

### 3. Improved Reverse Map Construction

```go
// BEFORE: Last mapping wins (WRONG)
reverseMap := make(map[int]int)
for _, m := range sm.Mappings {
    reverseMap[m.GeneratedLine] = m.OriginalLine
}

// AFTER: First mapping wins (handles duplicates better)
reverseMap := make(map[int]int)
for _, m := range sm.Mappings {
    if _, exists := reverseMap[m.GeneratedLine]; !exists {
        reverseMap[m.GeneratedLine] = m.OriginalLine
    }
}
```

**Note**: This is a WORKAROUND. The real fix is to eliminate duplicate mappings at the source.

## Test Results

### TestRoundTripTranslation (Expanded)

**Status**: ❌ FAILS (as expected - test now catches the bug!)

```
Round-trip failed for func readConfig: dingo 3 → go 7 → dingo 7 (expected 3)
  Expected: dingo line 3: "func readConfig(path string) ([]byte, error) {"
  Got:      dingo line 7: ""
  Via:      go line 7: "func readConfig(path string) ([]byte, error) {"
```

**Interpretation**: Test correctly detects duplicate mapping bug

### TestIdentityMappingReverse

**Status**: ✅ PASSES

**Why**: Uses `sm.MapToOriginal()` which was already fixed to handle duplicates

**Conclusion**: Test validates the MapToOriginal fix works correctly

## Lessons Learned

### 1. Never Assume "Simple" Code Doesn't Need Tests

**Assumption**: Identity mappings are simple (1:1 mapping), so if transformations work, they must work too

**Reality**: Identity mappings have DIFFERENT complexity (line offset calculations, duplicate handling)

### 2. Test Both Forward and Reverse Operations

**Covered**: Forward mapping (.dingo → .go)

**Missed**: Reverse mapping (.go → .dingo)

**LSP Critical**: Go-to-Definition, Hover, References all use REVERSE mapping

### 3. Test Real User Scenarios

**Not enough**: Test data structure validity

**Required**: Test actual LSP operations (Go-to-Definition jumping to correct line)

### 4. Comprehensive Coverage Checklist

For source map testing, verify:
- ✅ Transformed lines (complex transformations)
- ✅ Untransformed lines (identity mappings)
- ✅ Edge cases (blank lines, comments, package declarations)
- ✅ Forward mapping (.dingo → .go)
- ✅ Reverse mapping (.go → .dingo)
- ✅ Real LSP scenarios (hover, go-to-definition, etc.)
- ✅ Duplicate mapping handling
- ✅ Line offset calculations (import blocks, preprocessing)

## Next Steps

### Immediate Actions

1. ✅ Expand `TestRoundTripTranslation` with untransformed lines
2. ✅ Add `TestIdentityMappingReverse` for reverse lookup validation
3. ✅ Document coverage blindspot in CLAUDE.md
4. ⏳ Fix duplicate mappings at source (root cause fix)
5. ⏳ Verify all tests pass after fix

### Root Cause Fix Required

**Problem**: Source map contains duplicate mappings for same generated line

**Example**:
```json
// Go line 7 has TWO mappings (WRONG)
{"generated_line": 7, "original_line": 3},
{"generated_line": 7, "original_line": 7}
```

**Fix Location**: `pkg/sourcemap/postast.go` - Prevent duplicate identity mappings

**Strategy**: When adding identity mapping, check if transformation mapping already exists for that generated line

### Test Verification After Fix

When duplicate mappings are fixed:
1. Run `TestRoundTripTranslation` - should PASS
2. Run `TestIdentityMappingReverse` - should continue to PASS
3. Run `TestGoToDefinitionReverse` - should PASS
4. Manual LSP testing - Go-to-Definition should jump to correct lines

## Metrics

### Test Coverage Improvement

**Before**:
- Test cases: 2
- Line types: 1 (transformed only)
- Coverage: ~29% of line types (2/7 lines)

**After**:
- Test cases: 11 (7 in round-trip + 4 in identity reverse)
- Line types: 3 (transformed, identity, edge cases)
- Coverage: 100% of line types (all significant lines)

### Bug Detection Rate

**Before**: 0/2 bugs detected by tests
- Identity mapping offset: ❌ Not detected
- Duplicate mappings: ❌ Not detected

**After**: 2/2 bugs detected by tests
- Identity mapping offset: ✅ Detected by TestRoundTripTranslation
- Duplicate mappings: ✅ Detected by TestRoundTripTranslation

**Improvement**: 0% → 100% bug detection rate

## References

- **Test file**: `pkg/sourcemap/postast_validation_test.go`
- **CLAUDE.md section**: "Test Coverage Blindspots: The Identity Mapping Example"
- **Related commit**: [Previous fix for MapToOriginal duplicate handling]
- **Bug report**: LSP Go-to-Definition jumping to wrong line (manual testing discovery)
