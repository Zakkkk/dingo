# Phase 1.6 Error Propagation - Comprehensive Test Plan

**Date:** 2025-11-16
**Session:** 20251116-185104
**Phase:** Testing

## Executive Summary

This test plan validates all implemented features for Phase 1.6: Error Propagation Operator (?) Pipeline Integration. The testing strategy encompasses unit tests, golden file tests, integration tests, type inference validation, and error handling verification.

## Testing Scope

### Implemented Features to Test

1. **Statement Context** - `x := expr?`
   - Simple assignment with error propagation
   - Multiple error propagations in sequence
   - Error wrapping in statement context

2. **Expression Context** - `return expr?`
   - Expression in return statements
   - Expression in function calls
   - Nested expressions
   - Statement lifting verification

3. **Error Wrapping** - `expr? "message"`
   - Simple error messages
   - Messages with special characters (quotes, newlines, tabs)
   - fmt.Errorf generation with %w
   - fmt import injection

4. **Type Inference** - go/types integration
   - Basic types (int, string, bool, float64)
   - Pointer types (*T)
   - Slice types ([]T)
   - Map types (map[K]V)
   - Struct types
   - Channel types
   - Interface types
   - Zero value generation accuracy

5. **Source Maps** - Position tracking
   - Mapping infrastructure exists
   - VLQ encoding (ready for future enhancement)

## Test Categories

### Category 1: Golden File Tests

**Purpose:** Verify Dingo → Go transpilation produces correct, idiomatic output

**Test Files Location:** `/Users/jack/mag/dingo/tests/golden/`

**Test Cases:**

1. **simple_statement.dingo** - Basic statement context
   - Single error propagation
   - Expected: Clean error check pattern

2. **multiple_statements.dingo** - Chained operations
   - Multiple `?` in sequence
   - Expected: Multiple error checks in order

3. **expression_return.dingo** - Expression in return
   - Statement lifting
   - Expected: Statements before return

4. **error_wrapping.dingo** - Error message wrapping
   - Message with ?
   - Expected: fmt.Errorf with %w

5. **complex_types.dingo** - Various return types
   - Pointer, slice, map returns
   - Expected: Correct zero values

6. **nested_expression.dingo** - Deeply nested ?
   - Multiple levels of nesting
   - Expected: All lifted correctly

7. **mixed_context.dingo** - Statement + expression in same function
   - Both contexts used
   - Expected: Correct transformation for each

8. **special_chars.dingo** - Error messages with special characters
   - Quotes, newlines, tabs in messages
   - Expected: Proper escaping

### Category 2: Integration Tests

**Purpose:** Verify generated code compiles and runs correctly

**Test Files Location:** `/Users/jack/mag/dingo/tests/integration_test.go`

**Test Scenarios:**

1. **End-to-End Transpilation**
   - Parse Dingo file
   - Transform with plugin
   - Generate Go code
   - Verify compilable

2. **Runtime Behavior**
   - Generated code executes correctly
   - Error propagation works as expected
   - Zero values are correct

3. **Import Injection**
   - fmt import added when needed
   - No duplicate imports
   - Import placement correct

### Category 3: Type Inference Tests

**Purpose:** Validate go/types integration produces accurate zero values

**Test Location:** Enhanced `/Users/jack/mag/dingo/tests/error_propagation_test.go`

**Type Categories:**

1. **Basic Types**
   - int, int8, int16, int32, int64
   - uint, uint8, uint16, uint32, uint64
   - float32, float64
   - complex64, complex128
   - bool
   - string
   - byte, rune

2. **Composite Types**
   - Pointers: *User, *int
   - Slices: []int, []string, []*User
   - Arrays: [5]int, [10]string
   - Maps: map[string]int, map[int]*User
   - Channels: chan int, chan<- int, <-chan int

3. **Named Types**
   - struct types: User, Config
   - interface types: error, io.Reader
   - Type aliases
   - Imported types

4. **Edge Cases**
   - nil-able types → nil
   - Struct types → TypeName{}
   - Function types → nil
   - Unnamed struct types

### Category 4: Error Case Tests

**Purpose:** Verify graceful degradation and error handling

**Test Scenarios:**

1. **Type Inference Failures**
   - Function without return type
   - Ambiguous types
   - Fallback to nil

2. **Invalid Syntax**
   - Malformed error messages
   - ? without function call
   - Type checking errors

3. **Context Detection**
   - Unusual AST structures
   - Edge case expressions

### Category 5: AST Transformation Tests

**Purpose:** Verify correct AST manipulation

**Test Scenarios:**

1. **Parent Chain Traversal**
   - Deeply nested expressions
   - Find enclosing block
   - Find enclosing statement

2. **Statement Injection**
   - Multiple injections in same block
   - Index preservation
   - Order preservation

3. **Two-Pass Transformation**
   - Pending injections queued
   - Applied after traversal
   - No AST corruption

## Test Execution Strategy

### Phase 1: Unit Tests
Run existing tests in `tests/error_propagation_test.go`
- Smoke tests (10 tests)
- Type inference tests (5 tests)
- Statement lifter test (1 test)
- Error wrapper tests (4 tests)

### Phase 2: Golden File Tests
Create and run golden file comparisons
- 8+ golden file test pairs
- Exact string matching
- Diff reporting on failure

### Phase 3: Integration Tests
Compile and run generated code
- Create temp Go files
- Run `go build`
- Execute and verify output

### Phase 4: Comprehensive Validation
Full test suite execution
- All categories
- Race condition check
- Memory leak check

## Success Criteria

### Must Pass
- ✅ All 20 existing tests pass
- ✅ All 8 golden file tests produce correct output
- ✅ All generated code compiles with `go build`
- ✅ Type inference produces correct zero values for all types
- ✅ Error wrapping generates valid fmt.Errorf calls
- ✅ No race conditions (`go test -race`)

### Quality Metrics
- Code coverage > 80% for plugin code
- All edge cases documented and tested
- Generated code is readable and idiomatic
- Performance acceptable (< 100ms per file)

## Test Implementation Plan

### Step 1: Create Golden File Infrastructure
1. Create `tests/golden/` directory
2. Write 8 .dingo test files
3. Write corresponding .go.golden files
4. Implement golden file test harness

### Step 2: Create Integration Test Framework
1. Add integration_test.go
2. Implement parse → transform → compile pipeline
3. Add runtime behavior verification

### Step 3: Enhance Type Inference Tests
1. Add tests for all type categories
2. Test zero value generation
3. Verify edge cases

### Step 4: Run and Analyze
1. Execute full test suite
2. Collect results
3. Fix any failures
4. Document findings

## Test Environment

- **Go Version:** 1.21+
- **OS:** macOS (Darwin 25.1.0)
- **Working Directory:** `/Users/jack/mag/dingo`
- **Test Command:** `go test -v ./tests/...`
- **Race Detection:** `go test -race ./tests/...`
- **Coverage:** `go test -cover ./tests/...`

## Risk Assessment

### Low Risk
- Unit tests already passing
- Plugin architecture proven
- Type inference implementation complete

### Medium Risk
- Golden file format matching (whitespace, formatting)
- Complex type zero values (unnamed types)
- Edge case coverage completeness

### High Risk (Mitigated)
- AST manipulation correctness ✅ FIXED (two-pass approach)
- Parent chain traversal ✅ FIXED (parent map)
- Race conditions ✅ FIXED (removed global variables)

## Expected Outcomes

### Test Results
- **Total Tests:** 40-50 tests
- **Expected Pass Rate:** 95%+
- **Known Issues:** None currently identified

### Documentation Outputs
1. `test-plan.md` - This document
2. `test-results.md` - Detailed results for each test
3. `test-summary.txt` - STATUS: PASS/FAIL with metrics

## Timeline

- Test Plan Creation: 30 minutes ✅
- Golden File Implementation: 1.5 hours
- Integration Tests: 1 hour
- Test Execution & Analysis: 1 hour
- Documentation: 30 minutes

**Total Estimated Time:** 4.5 hours
