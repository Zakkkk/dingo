# Test Plan: Build Fixes and Critical Issues

**Session:** 20251117-204314
**Date:** 2025-11-17
**Test Designer:** golang-developer agent

---

## 1. Requirements Understanding

### What Was Fixed:
1. **Duplicate Method Declaration** - Removed `transformErrorProp` from `pkg/transform/transformer.go`
2. **Import Injection** - Added automatic import detection and injection in preprocessor
3. **Source Mapping Accuracy** - Fixed column/offset calculations for `?` operator
4. **Source Map Import Offsets** - Only shift mappings after import insertion line
5. **Type Assertion Safety** - Safe type assertion in transformer
6. **Multi-value Returns** - Documented limitation (requires Phase 3)

### Key Behaviors to Validate:
- All packages build successfully without errors
- Unit tests pass (especially preprocessor with import injection)
- Source mappings are accurate after import injection
- Import detection tracks standard library function calls correctly
- Golden test files compile (with auto-injected imports)
- Transform package handles AST operations safely

### Critical Edge Cases:
- Import injection doesn't shift pre-import mappings
- Column positions for `?` operator are exact (not line-start)
- Type assertions don't panic on unexpected types
- Functions without `?` operator don't trigger import detection

---

## 2. Test Scenarios

### Scenario 1: Full Project Build
**Purpose:** Verify all build errors are resolved
**Input:** Entire codebase
**Expected Output:** Zero compilation errors
**Rationale:** Primary success criterion for this session

**Test Command:**
```bash
go build ./...
```

**Success Criteria:**
- Exit code 0
- No error messages
- All packages compile

---

### Scenario 2: Package-Level Unit Tests
**Purpose:** Ensure code changes didn't break existing functionality
**Input:** All test files in pkg/
**Expected Output:** All tests pass
**Rationale:** Regression detection

**Test Commands:**
```bash
go test ./pkg/config -v
go test ./pkg/generator -v
go test ./pkg/preprocessor -v
go test ./pkg/sourcemap -v
go test ./pkg/transform -v
```

**Success Criteria:**
- All test functions pass
- No panics or runtime errors
- Coverage maintained or improved

---

### Scenario 3: Preprocessor Import Detection
**Purpose:** Validate automatic import tracking works correctly
**Input:** Test cases in `pkg/preprocessor/preprocessor_test.go`
**Expected Output:** Correct imports injected for function calls
**Rationale:** Core feature of CRITICAL fixes

**Test Focus:**
- `TestAutomaticImportDetection` (4 subtests)
  - os.ReadFile detection
  - encoding/json.Marshal detection
  - strconv.Atoi detection
  - Multiple imports in single file

**Success Criteria:**
- All 4 subtests pass
- Import blocks contain correct packages
- No duplicate imports
- Imports sorted alphabetically

---

### Scenario 4: Source Mapping Accuracy
**Purpose:** Verify mappings remain accurate after import injection
**Input:** `TestSourceMappingWithImports` test case
**Expected Output:** Mappings point to correct lines/columns
**Rationale:** CRITICAL-1 and CRITICAL-2 fixes validation

**Test Focus:**
- Column position of `?` operator is exact (not column 1)
- Line numbers adjusted correctly for import injection
- Mappings before imports NOT shifted
- Mappings after imports shifted by import count

**Success Criteria:**
- `OriginalColumn` equals actual `?` position (1-based)
- `Length` equals 1 (length of `?`)
- `GeneratedLine` offset matches import insertion

---

### Scenario 5: Transform Package Safety
**Purpose:** Ensure transformer handles AST operations without panics
**Input:** AST transformation operations
**Expected Output:** Safe error handling, no panics
**Rationale:** CRITICAL-4 fix validation

**Test Focus:**
- Type assertions use comma-ok idiom
- Error messages are descriptive
- No unsafe operations

**Manual Verification:**
```go
// Check transformer.go line 48 uses:
if f, ok := result.(*ast.File); ok {
    return f, nil
}
return nil, fmt.Errorf(...)
```

**Success Criteria:**
- No `result.(*ast.File)` direct assertions
- All type assertions checked with comma-ok
- Errors returned instead of panics

---

### Scenario 6: Golden Test Compilation (Sample)
**Purpose:** Verify generated .go files compile with auto-injected imports
**Input:** Selected golden test files
**Expected Output:** All compile successfully
**Rationale:** End-to-end validation of import injection

**Test Sample:**
- `error_prop_01_simple_statement.go.golden` (os.ReadFile)
- `error_prop_05_json_parsing.go.golden` (encoding/json.Marshal)
- `error_prop_06_type_conversion.go.golden` (strconv.Atoi)

**Test Method:**
```bash
cd tests/golden
go build -o /tmp/test_$$ error_prop_01_simple_statement.go.golden
go build -o /tmp/test_$$ error_prop_05_json_parsing.go.golden
go build -o /tmp/test_$$ error_prop_06_type_conversion.go.golden
```

**Success Criteria:**
- All files compile without errors
- No missing import errors
- Exit code 0 for each build

---

### Scenario 7: Error Propagation Regression
**Purpose:** Ensure error propagation still works after refactoring
**Input:** Error propagation test cases
**Expected Output:** `?` operator correctly expanded
**Rationale:** Core feature must not regress

**Test Focus:**
- Assignment context: `x := f()?`
- Return context: `return f()?`
- Multi-statement functions
- Nested error propagation

**Success Criteria:**
- All existing error propagation tests pass
- Generated code matches expected patterns
- Source maps accurate

---

### Scenario 8: Import Injection Edge Cases
**Purpose:** Verify import handling edge cases
**Input:** Various import scenarios
**Expected Output:** Correct behavior in all cases
**Rationale:** Prevent false positives and conflicts

**Edge Cases:**
- File with existing imports (should merge, not duplicate)
- File with no imports (should add import block)
- Multiple processors needing same import (no duplicates)
- Function call that doesn't need import (shouldn't add)

**Success Criteria:**
- No duplicate imports in output
- Imports sorted and formatted correctly
- astutil.AddImport handles conflicts

---

## 3. Test Execution Order

**Sequential Execution (Critical Path):**

1. **Build Verification** (Scenario 1)
   - STOP if fails - nothing else matters

2. **Core Package Tests** (Scenario 2)
   - config, generator, sourcemap, transform
   - STOP if failures in these packages

3. **Preprocessor Tests** (Scenarios 3, 4, 7)
   - Import detection
   - Source mapping
   - Error propagation
   - CONTINUE even if some fail (detailed analysis)

4. **Safety Verification** (Scenario 5)
   - Manual code inspection
   - Type assertion safety

5. **Golden Tests Sample** (Scenario 6)
   - Compile verification
   - Import injection validation

6. **Edge Case Testing** (Scenario 8)
   - Import handling edge cases

---

## 4. Test Coverage Summary

### What's Well-Covered:
- ✅ Full project build verification
- ✅ Preprocessor import detection (4 test cases)
- ✅ Source mapping accuracy (dedicated tests)
- ✅ Error propagation regression (existing suite)
- ✅ Transform package safety (code inspection)
- ✅ Golden test compilation (sample verification)

### Gaps Requiring Manual Testing:
- ⚠️ Large-scale golden test suite (46 files) - sampled instead
- ⚠️ Multi-value return limitation (documented, not fixable at preprocessor level)
- ⚠️ Complex nested error propagation patterns
- ⚠️ Performance impact of import injection

### Confidence Level:
**HIGH (85%)** - Critical issues addressed with comprehensive unit tests and build verification

---

## 5. Success Criteria

**PASS Criteria:**
- [ ] `go build ./...` exits with code 0
- [ ] All pkg/config tests pass
- [ ] All pkg/generator tests pass
- [ ] All pkg/preprocessor tests pass (19+ tests)
- [ ] All pkg/sourcemap tests pass
- [ ] pkg/transform compiles (no tests yet)
- [ ] Sample golden tests compile (3+ files)
- [ ] No unsafe type assertions in code
- [ ] Source mappings accurate (column/line offsets correct)

**FAIL Criteria:**
- Build fails with compilation errors
- Preprocessor tests fail (import detection or mapping)
- Regression in error propagation functionality
- Type assertion panics possible
- Golden tests don't compile

---

## 6. Verification Checklist

**Build Health:**
- [ ] No duplicate method declarations
- [ ] No unused variable warnings
- [ ] All imports resolved
- [ ] No type errors

**Functionality:**
- [ ] Import detection tracks function calls
- [ ] Import injection adds correct packages
- [ ] Source mappings accurate for `?` operator
- [ ] Error propagation still works
- [ ] Transform package safe (no panics)

**Quality:**
- [ ] All unit tests pass
- [ ] No regressions introduced
- [ ] Code follows Go best practices
- [ ] Documentation updated (CHANGELOG, READMEs)

---

## 7. Expected Test Results

### Optimistic Scenario (90% confidence):
- All builds succeed
- 19/19 preprocessor tests pass
- All other package tests pass
- Sample golden tests compile
- Zero critical issues found

### Realistic Scenario (expected):
- All builds succeed
- 18-19/19 preprocessor tests pass (1 may need adjustment)
- pkg/parser tests may still fail (pre-existing issues)
- Sample golden tests compile
- Minor issues in edge cases (non-critical)

### Pessimistic Scenario (10% probability):
- Build succeeds but tests reveal mapping bugs
- Import detection misses some edge cases
- Golden tests have import conflicts
- Need iteration to fix test issues

---

## 8. Test Artifacts

**Outputs:**
- `test-results.md` - Detailed test output and analysis
- `test-summary.txt` - STATUS and counts
- Console logs from all test runs
- Build verification results

**Metrics:**
- Total tests run
- Pass/fail counts
- Build time
- Test execution time

---

**Test Plan Created:** 2025-11-17
**Ready for Execution:** YES
**Estimated Time:** 15-20 minutes
