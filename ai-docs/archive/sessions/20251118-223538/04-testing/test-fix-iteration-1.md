# Test Fix Iteration 1: Update Test Expectations for Source Map Fix

**Date**: 2025-11-18
**Session**: 20251118-223538
**Task**: Fix 4 failing preprocessor tests after LSP source mapping fix

---

## Summary

✅ **All 4 target tests now PASS**

The failing tests were expecting 7 mappings per error propagation, but the implementation correctly generates 8 mappings (1 `expr_mapping` + 7 `error_prop`). Updated all test expectations to reflect the correct behavior.

---

## Tests Fixed

### 1. TestSourceMapGeneration ✅ PASS

**File**: `pkg/preprocessor/preprocessor_test.go` (lines 281-340)

**Changes**:
- Updated expected mapping count: 7 → 8
- Added comprehensive comments explaining the two mapping types:
  - `expr_mapping`: Maps the expression itself (e.g., `ReadFile(path)`)
  - `error_prop`: Maps the error handling expansion (7 generated lines)
- Updated `expectedMappings` array to include 8 entries (first one is `expr_mapping`)

**Before**:
```go
// Should have 7 mappings (one expansion)
if len(sourceMap.Mappings) != 7 {
    t.Errorf("expected 7 mappings, got %d", len(sourceMap.Mappings))
}
```

**After**:
```go
// Should have 8 mappings (1 expr_mapping + 7 error_prop)
if len(sourceMap.Mappings) != 8 {
    t.Errorf("expected 8 mappings (1 expr + 7 error_prop), got %d", len(sourceMap.Mappings))
}
```

### 2. TestSourceMapMultipleExpansions ✅ PASS

**File**: `pkg/preprocessor/preprocessor_test.go` (lines 345-424)

**Changes**:
- Updated expected mapping count: 14 → 16 (two error propagations: 8 + 8)
- Rewrote verification logic to check:
  - Mapping 0: `expr_mapping` for first error propagation (line 4)
  - Mappings 1-7: `error_prop` for first error propagation (lines 7-13)
  - Mapping 8: `expr_mapping` for second error propagation (line 5)
  - Mappings 9-15: `error_prop` for second error propagation (lines 14-20)

**Before**:
```go
// Total: 14 mappings
if len(sourceMap.Mappings) != 14 {
    t.Errorf("expected 14 mappings (7+7), got %d", len(sourceMap.Mappings))
}

// First expansion: line 4 → lines 7-13
for i := 0; i < 7; i++ {
    // Check mappings...
}

// Second expansion: line 5 → lines 14-20
for i := 7; i < 14; i++ {
    // Check mappings...
}
```

**After**:
```go
// Total: 16 mappings
if len(sourceMap.Mappings) != 16 {
    t.Errorf("expected 16 mappings (8+8: 2 expr + 14 error_prop), got %d", len(sourceMap.Mappings))
}

// Mapping 0: expr_mapping for line 4 (points to line 7)
if sourceMap.Mappings[0].OriginalLine != 4 {
    t.Errorf("mapping 0: expected original line 4, got %d", sourceMap.Mappings[0].OriginalLine)
}

// Mappings 1-7: error_prop for line 4 (generated lines 7-13)
for i := 1; i < 8; i++ {
    // Check mappings...
}

// [Similar for second expansion: mappings 8-15]
```

### 3. TestSourceMappingWithImports ✅ PASS

**File**: `pkg/preprocessor/preprocessor_test.go` (lines 472-540)

**Changes**:
- Updated expected mapping count: 7 → 8

**Before**:
```go
// Should have 7 mappings (one expansion)
if len(sourceMap.Mappings) != 7 {
    t.Errorf("expected 7 mappings, got %d", len(sourceMap.Mappings))
}
```

**After**:
```go
// Should have 8 mappings (1 expr_mapping + 7 error_prop)
if len(sourceMap.Mappings) != 8 {
    t.Errorf("expected 8 mappings (1 expr + 7 error_prop), got %d", len(sourceMap.Mappings))
}
```

### 4. TestCRITICAL1_MappingsBeforeImportsNotShifted ✅ PASS

**File**: `pkg/preprocessor/preprocessor_test.go` (lines 641-736)

**Changes**:
- Updated expected mapping count: 7 → 8
- Updated comment to explain the two mapping types

**Before**:
```go
// Additional verification: Error propagation should produce 7 mappings
// all pointing to original line 8, generated lines starting after import block
errorPropMappings := 0
for _, mapping := range sourceMap.Mappings {
    if mapping.OriginalLine == 8 {
        errorPropMappings++
    }
}

if errorPropMappings != 7 {
    t.Errorf("Expected 7 mappings for error propagation (line 8), got %d", errorPropMappings)
}
```

**After**:
```go
// Additional verification: Error propagation should produce 8 mappings
// (1 expr_mapping + 7 error_prop) all pointing to original line 8
errorPropMappings := 0
for _, mapping := range sourceMap.Mappings {
    if mapping.OriginalLine == 8 {
        errorPropMappings++
    }
}

if errorPropMappings != 8 {
    t.Errorf("Expected 8 mappings for error propagation (1 expr + 7 error_prop), got %d", errorPropMappings)
}
```

---

## Test Results

### Before Fixes

```
=== RUN   TestSourceMapGeneration
    preprocessor_test.go:322: expected 7 mappings, got 8
--- FAIL: TestSourceMapGeneration

=== RUN   TestSourceMapMultipleExpansions
    preprocessor_test.go:365: expected 14 mappings (7+7), got 16
--- FAIL: TestSourceMapMultipleExpansions

=== RUN   TestSourceMappingWithImports
    preprocessor_test.go:515: expected 7 mappings, got 8
--- FAIL: TestSourceMappingWithImports

=== RUN   TestCRITICAL1_MappingsBeforeImportsNotShifted
    preprocessor_test.go:731: Expected 7 mappings for error propagation (line 8), got 8
--- FAIL: TestCRITICAL1_MappingsBeforeImportsNotShifted
```

### After Fixes

```
=== RUN   TestSourceMapGeneration
--- PASS: TestSourceMapGeneration (0.00s)

=== RUN   TestSourceMapMultipleExpansions
--- PASS: TestSourceMapMultipleExpansions (0.00s)

=== RUN   TestSourceMappingWithImports
    preprocessor_test.go:521: Result has 15 lines
--- PASS: TestSourceMappingWithImports (0.00s)

=== RUN   TestCRITICAL1_MappingsBeforeImportsNotShifted
    preprocessor_test.go:714: Import block inserted at generated line 3
    preprocessor_test.go:715: Total mappings: 8
--- PASS: TestCRITICAL1_MappingsBeforeImportsNotShifted (0.00s)

PASS
ok  	github.com/MadAppGang/dingo/pkg/preprocessor	0.436s
```

---

## Why 8 Mappings Is Correct

The implementation generates **two distinct mapping types** for error propagation:

### 1. Expression Mapping (1 mapping)

**Purpose**: Maps the function call expression itself

**Example**:
```dingo
data := ReadFile(path)?
           ^-------------^
           This is the expression
```

**Mapping**:
- Name: `expr_mapping`
- Original position: Column 13 (start of `ReadFile`)
- Generated position: Column 20 in `__tmp0, __err0 := ReadFile(path)`
- Use case: When user hovers over `ReadFile`, LSP shows function signature

### 2. Error Propagation Mappings (7 mappings)

**Purpose**: Maps the error handling expansion back to `?` operator

**Example**:
```dingo
data := ReadFile(path)?
                         ^
                         This is the ? operator
```

**Mappings** (all 7 point to column 27, the `?` position):
1. Line 7: `__tmp0, __err0 := ReadFile(path)`
2. Line 8: `// dingo:s:1`
3. Line 9: `if __err0 != nil {`
4. Line 10: `return nil, __err0`
5. Line 11: `}`
6. Line 12: `// dingo:e:1`
7. Line 13: `var data = __tmp0`

**Use case**: When error occurs in error handling code, LSP points user to `?` operator

### Why Both Are Needed

**Scenario 1: Error in expression**
```go
__tmp0, __err0 := ReadFileXXX(path)  // Function name typo
```
→ LSP uses `expr_mapping` → Points to `ReadFileXXX` in original source

**Scenario 2: Error in error handling**
```go
return nil, __err0  // Wrong return values
```
→ LSP uses `error_prop` mapping → Points to `?` operator (since that's what generated this code)

---

## Unrelated Test Failures

The preprocessor test suite has 2 other failing tests that are **unrelated to the LSP source mapping fix**:

### 1. TestGeminiCodeReviewFixes

**Error**: `failed to parse source for import injection: 3:50: missing ',' in parameter list`

**Cause**: Test has invalid Go syntax (unrelated to source mapping)

### 2. TestSourceMapEdgeCases

**Error**: `Outside range should return mapping start, got (2, 20)`

**Cause**: Test expectation for edge case behavior (not related to mapping count)

These should be fixed separately.

---

## Files Modified

- **`pkg/preprocessor/preprocessor_test.go`**:
  - Lines 295-323: TestSourceMapGeneration
  - Lines 360-423: TestSourceMapMultipleExpansions
  - Lines 513-519: TestSourceMappingWithImports
  - Lines 721-735: TestCRITICAL1_MappingsBeforeImportsNotShifted

---

## Conclusion

✅ **All 4 target tests now pass**

The LSP source mapping fix is **correct**. The implementation properly generates:
- 1 `expr_mapping` per error propagation (for expression errors)
- 7 `error_prop` mappings per error propagation (for error handling errors)

Test expectations have been updated to match this correct behavior.

**Next Steps**:
1. ✅ LSP source mapping fix validated
2. ✅ Test expectations updated
3. ⏭️ Manual LSP integration testing (verify diagnostics point to correct locations in VS Code)
4. ⏭️ Fix unrelated test failures (TestGeminiCodeReviewFixes, TestSourceMapEdgeCases)
