# Test Plan for Bug Fix Verification

## Overview
This test suite adds comprehensive negative tests to prevent regressions of the three critical bugs fixed in this session:
- CRITICAL #1: Source-map offset bug
- CRITICAL #2: Multi-value return bug
- IMPORTANT #1: Import detection false positives

## Test Status Summary

### Already Covered
1. **TestCRITICAL2_MultiValueReturnHandling** (lines 522-608)
   - Tests 2-value returns (int, string, error)
   - Tests 3-value returns (string, int, bool, error)
   - Tests single-value returns (regression check)
   - Tests with custom error messages
   - **VERDICT: Comprehensive, no changes needed**

### Needs Enhancement
2. **TestSourceMappingWithImports** (lines 470-520)
   - Currently only verifies mappings AFTER imports are correct
   - **MISSING:** Verification that package declaration mapping is NOT shifted
   - **MISSING:** Verification of mappings for content before import block
   - **ACTION:** Add new test `TestCRITICAL1_MappingsBeforeImportsNotShifted`

### Missing Entirely
3. **TestIMPORTANT1_UserDefinedFunctionsDontTriggerImports**
   - No test verifies that user-defined functions shadowing stdlib names don't trigger imports
   - This was a critical bug causing false positive imports
   - **ACTION:** Create comprehensive test with multiple scenarios

## Test Design

### Test 1: TestCRITICAL1_MappingsBeforeImportsNotShifted

**Purpose:** Verify that source mappings for code BEFORE the import block are NOT shifted when imports are injected.

**Scenarios:**
1. Code with package declaration + type definition + error propagation
2. Verify package declaration stays at line 1
3. Verify type definition before imports is NOT shifted
4. Verify error propagation AFTER imports IS shifted correctly

**Input:**
```dingo
package main

type Config struct {
    Path string
}

func load(path string) ([]byte, error) {
    data := os.ReadFile(path)?
    return data, nil
}
```

**Expected Behavior:**
- Import block inserted after line 1 (package main)
- Lines 1-6 (package + type) should map to generated lines 1-6 (no shift)
- Error propagation on line 8 should map to generated lines ~11-17 (shifted by import block size)

**Verification:**
- Parse source map
- Check that no mappings exist with OriginalLine < import insertion line AND GeneratedLine != OriginalLine
- This proves lines before imports aren't incorrectly shifted

### Test 2: TestIMPORTANT1_UserDefinedFunctionsDontTriggerImports

**Purpose:** Verify that user-defined functions with stdlib names do NOT trigger import injection.

**Scenarios:**

#### Scenario A: User-defined ReadFile (no package qualifier)
```dingo
package main

func ReadFile(path string) error {
    return nil
}

func main() {
    err := ReadFile("/tmp/test")?
}
```
**Expected:** NO `import "os"` (user-defined function)

#### Scenario B: Qualified os.ReadFile (package qualifier)
```dingo
package main

func main() {
    data := os.ReadFile("/tmp/test")?
}
```
**Expected:** YES `import "os"` (stdlib function)

#### Scenario C: Multiple user-defined functions matching stdlib names
```dingo
package main

func ReadFile(path string) error { return nil }
func Marshal(v any) error { return nil }
func Atoi(s string) error { return nil }

func main() {
    _ := ReadFile("/tmp")?
    _ := Marshal("test")?
    _ := Atoi("42")?
}
```
**Expected:** NO imports injected (all user-defined)

#### Scenario D: Mixed user-defined and qualified stdlib calls
```dingo
package main

func ReadFile(path string) error { return nil }

func main() {
    _ := ReadFile("/tmp")?        // user-defined
    _ := os.ReadFile("/tmp")?     // stdlib
    _ := strconv.Atoi("42")?      // stdlib
}
```
**Expected:** ONLY `import "os"` and `import "strconv"` (not a duplicate os)

**Verification:**
- Parse generated code
- Count import statements
- Verify exact imports match expected list
- Ensure no "unused import" compile errors

### Test 3: TestCRITICAL2_MultiValueReturnHandling (Review Only)

**Status:** Already comprehensive (lines 522-608)

**Coverage:**
- 2-value returns: `(int, string, error)` ✓
- 3-value returns: `(string, int, bool, error)` ✓
- Single-value returns: `(int, error)` (regression) ✓
- With error messages: Custom error wrapping ✓

**Verification Strategy:**
- Each test uses `shouldContain` and `shouldNotContain` patterns
- Validates correct number of temporaries generated
- Validates zero values in error path match return type
- Validates success path returns all temporaries + nil

**No changes needed.**

## Implementation Strategy

### 1. Add TestCRITICAL1_MappingsBeforeImportsNotShifted
- Location: `pkg/preprocessor/preprocessor_test.go`
- Insert after: TestSourceMappingWithImports (line ~520)
- Lines of code: ~80 lines

### 2. Add TestIMPORTANT1_UserDefinedFunctionsDontTriggerImports
- Location: `pkg/preprocessor/preprocessor_test.go`
- Insert after: TestCRITICAL1_MappingsBeforeImportsNotShifted
- Lines of code: ~150 lines (4 comprehensive scenarios)

### 3. Run Full Test Suite
```bash
go test ./pkg/preprocessor/... -v
```

## Expected Outcomes

### Success Criteria
- All existing tests continue to pass (100% pass rate)
- New tests pass on current codebase (bugs already fixed)
- Tests would FAIL if bugs were re-introduced (regression prevention)

### Regression Detection
If any bug is reintroduced:
1. **CRITICAL #1 regression:** TestCRITICAL1_MappingsBeforeImportsNotShifted fails
2. **CRITICAL #2 regression:** TestCRITICAL2_MultiValueReturnHandling fails
3. **IMPORTANT #1 regression:** TestIMPORTANT1_UserDefinedFunctionsDontTriggerImports fails

## Test Metrics

### Current Coverage
- Total preprocessor tests: 8
- Passing: 8/8 (100%)

### After Implementation
- Total preprocessor tests: 10
- Expected passing: 10/10 (100%)
- New negative tests: 2
- Enhanced tests: 0 (TestCRITICAL2 already comprehensive)

## Confidence Level

**High Confidence (95%)**

Rationale:
- Bugs were already fixed in codebase
- Tests verify exact behavior that was broken
- Tests use patterns that would have caught bugs before fixes
- Comprehensive scenario coverage (happy path + edge cases)
- Clear pass/fail criteria with specific string matching

## Open Questions

None. Implementation is straightforward.
