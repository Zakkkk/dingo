# Phase 3 Integration Testing Plan
**Date**: 2025-11-18
**Session**: 20251118-114514
**Phase**: 3 - Fix A4/A5 + Option[T] + Helper Methods

---

## Testing Strategy

### 1. Unit Test Suite (Primary Verification)
**Command**: `go test ./pkg/... -v`

**Target Coverage**:
- pkg/config: All tests passing (baseline)
- pkg/preprocessor: All 48 tests passing (baseline - Phase 2.16)
- pkg/plugin/builtin: Target 39/39 tests passing (up from 31/39)
  - Type inference tests (24 new tests from Task 1a)
  - Addressability tests (50+ new tests from Task 1c)
  - Result plugin tests (Fix A4/A5 integration)
  - Option plugin tests (Fix A4/A5 + None constant)
  - Helper method tests (8 methods × 2 types = 16 tests)
- pkg/generator: All tests passing
- pkg/sourcemap: All tests passing
- pkg/errors: All 7 tests passing (new package from Task 1b)

**Expected Failures** (out of scope):
- Parser tests: Known issues with full program parsing (deferred to future)
- Some edge cases require full go/types context integration

### 2. Golden Test Suite
**Command**: `go test ./tests/... -v`

**Expected Results**:
- ~25/46 golden tests passing (up from ~15/46)
- Result tests:
  - result_01_basic.dingo - ✅ Working (Phase 2.16)
  - result_02_propagation.dingo - ✅ Working (Phase 2.16)
  - result_06_helpers.dingo - 🆕 NEW (Task 3a)
- Option tests:
  - option_01_basic.dingo - ✅ Working (updated for None constant)
  - option_02_literals.dingo - 🆕 NEW (Task 2b - Fix A4)
  - option_05_helpers.dingo - 🆕 NEW (Task 3b)
- Error propagation tests: Baseline (whitespace differences expected)

**Known Issues**:
- Golden tests fail to compile (missing imports for ReadFile, etc.)
- This is expected - golden tests use stub functions
- Tests verify transpilation correctness, not compilation

### 3. End-to-End Verification
**Objective**: Verify dingo binary works end-to-end

**Test Cases**:
1. Build dingo binary: `go build ./cmd/dingo`
2. Verify version command works
3. Transpile Result helper test: `dingo build result_06_helpers.dingo`
4. Transpile Option helper test: `dingo build option_05_helpers.dingo`
5. Verify generated .go files compile (if possible)

### 4. Regression Testing
**Objective**: Ensure Phase 2.16 functionality still works

**Verification**:
- All preprocessor tests pass (48/48)
- All error propagation tests pass (8 tests)
- Enum preprocessing still works
- Multi-value returns still work
- No breaking changes to existing API

### 5. Metrics Collection

**Test Pass Rates**:
- Calculate: Passing tests / Total tests
- Compare before/after Phase 3
- Target: 39/39 builtin tests (100%)

**Type Inference Accuracy**:
- Measure: How often go/types successfully resolves types
- Baseline: ~40% (many interface{} fallbacks)
- Target: >90% success rate

**Code Quality**:
- No linter errors: `golangci-lint run` (if available)
- All code formatted: `go fmt ./...`
- No compilation warnings

---

## Success Criteria

### Passing Tests
- [ ] All pkg/config tests pass (baseline)
- [ ] All pkg/errors tests pass (7/7 new tests)
- [ ] All pkg/preprocessor tests pass (48/48 baseline)
- [ ] pkg/plugin/builtin: 217+ tests passing
  - [ ] Type inference tests (24 tests)
  - [ ] Addressability tests (50+ tests)
  - [ ] Result plugin tests (updated for Fix A4/A5)
  - [ ] Option plugin tests (updated for Fix A4/A5 + None)
  - [ ] Helper method tests (16 tests for both types)
- [ ] All pkg/generator tests pass
- [ ] All pkg/sourcemap tests pass

### Known Expected Failures
- ✅ TestInferNoneTypeFromContext - Requires full go/types context (Phase 4)
- ✅ TestConstructor_OkWithIdentifier - Needs full type checker integration
- ✅ TestConstructor_OkWithFunctionCall - Needs full type checker integration
- ✅ TestEdgeCase_InferTypeFromExprEdgeCases (3 subtests) - Behavior changed per Fix A5

### Golden Tests
- [ ] Error propagation tests still pass (with whitespace differences)
- [ ] result_06_helpers.dingo transpiles successfully
- [ ] option_05_helpers.dingo transpiles successfully
- [ ] No regressions in existing golden tests

### End-to-End
- [ ] dingo binary builds successfully
- [ ] Version command works
- [ ] Can transpile .dingo files without panics
- [ ] Generated .go files are syntactically correct

### Code Quality
- [ ] No compilation errors in any package
- [ ] go fmt ./... shows no changes needed
- [ ] No critical TODOs or FIXMEs left in code

---

## Test Execution Log

### Run 1: Full Package Tests
**Command**: `go test ./pkg/... -v`
**Date**: 2025-11-18
**Result**: TBD

### Run 2: Builtin Plugin Tests
**Command**: `go test ./pkg/plugin/builtin/... -v`
**Date**: 2025-11-18
**Result**: TBD

### Run 3: Golden Tests
**Command**: `go test ./tests/... -v`
**Date**: 2025-11-18
**Result**: TBD

### Run 4: End-to-End
**Command**: `go build ./cmd/dingo && ./dingo build <test>.dingo`
**Date**: 2025-11-18
**Result**: TBD

---

## Risk Mitigation

**Risk**: Golden tests may fail compilation
**Mitigation**: Focus on transpilation correctness, not compilation (stub functions OK)

**Risk**: Some tests expect old behavior (interface{} fallback)
**Mitigation**: Update test expectations to match Fix A5 requirements

**Risk**: Type inference may fail without full go/types context
**Mitigation**: Expected - tests verify graceful fallback behavior

**Risk**: Parser tests fail (known issue)
**Mitigation**: Out of scope for Phase 3, deferred to future

---

## Test Report Sections

### 1. Test Results (test-results.md)
- Detailed pass/fail counts
- Test output excerpts
- Failure analysis
- Metrics comparison

### 2. Test Summary (test-summary.txt)
- Brief one-line summary
- Overall pass/fail status
- Key metrics
- Link to full report

---

**Plan Status**: Ready for Execution
**Next Step**: Run all test suites and collect results
