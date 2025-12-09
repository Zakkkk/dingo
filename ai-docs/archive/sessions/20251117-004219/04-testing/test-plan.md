# Test Plan: Four New Features Testing

**Date:** 2025-11-17
**Session:** 20251117-004219

## Executive Summary

This document outlines the comprehensive testing strategy for four new Dingo language features:
1. Safe Navigation operator (?.)
2. Null Coalescing operator (??)
3. Ternary Operator (? :)
4. Lambda Functions

**Key Constraint:** Parser integration is NOT complete, so tests focus on plugin transformation correctness with manually constructed AST nodes.

---

## 1. Requirements Understanding

### 1.1 Safe Navigation (?.)
**Purpose:** Null-safe field access that avoids verbose nil checks

**Key Behaviors:**
- Smart mode (default): Returns T with zero value fallback
- Always-option mode: Returns Option[T] wrapping the result
- Should handle nil base expressions gracefully
- Configuration-driven behavior via `safe_navigation_unwrap`

**Critical Edge Cases:**
- Nil base expression (user?.name where user is nil)
- Chaining (user?.address?.city) - KNOWN LIMITATION
- Different result types (pointer, struct, primitive)

### 1.2 Null Coalescing (??)
**Purpose:** Provide fallback values for nullable types

**Key Behaviors:**
- Works with Option[T] types (IsSome/Unwrap)
- Works with Go pointers *T when enabled (nil check and dereference)
- Configuration-driven via `null_coalescing_pointers`
- Chaining support (a ?? b ?? c)

**Critical Edge Cases:**
- Option[T] with Some value
- Option[T] with None value
- Go pointer with non-nil value
- Go pointer with nil value
- Fallback value type matching

### 1.3 Ternary Operator (? :)
**Purpose:** Inline conditional expressions

**Key Behaviors:**
- Generates IIFE for expression contexts
- Clean if-else for statement contexts (method exists but not used)
- Supports precedence configuration (standard/explicit)
- Nested ternaries should work

**Critical Edge Cases:**
- True condition
- False condition
- Nested ternaries
- Complex condition expressions

### 1.4 Lambda Functions
**Purpose:** Anonymous function expressions

**Key Behaviors:**
- Rust-style syntax: |x| expr
- Arrow-style syntax: (x) => expr (prepared but not parsed)
- Transforms to Go func literals
- Expression bodies wrapped in return statement
- Configuration-driven via `lambda_syntax`

**Critical Edge Cases:**
- No parameters
- Single parameter
- Multiple parameters
- Expression body
- Type inference (basic, not fully implemented)

---

## 2. Test Scenarios

### 2.1 Safe Navigation Plugin Tests

#### Scenario 1: Basic Safe Navigation (Smart Mode)
- **Purpose:** Validate smart unwrapping with zero value fallback
- **Input:** SafeNavigationExpr with user?.name
- **Expected Output:** IIFE returning field or nil
- **Rationale:** Core use case for smart mode

#### Scenario 2: Safe Navigation (Always-Option Mode)
- **Purpose:** Validate Option[T] wrapping in strict mode
- **Input:** SafeNavigationExpr with config = "always_option"
- **Expected Output:** IIFE returning Option_Some or Option_None
- **Rationale:** Tests configuration-driven behavior

#### Scenario 3: Non-SafeNavigation Node
- **Purpose:** Verify plugin doesn't transform unrelated nodes
- **Input:** Regular ast.SelectorExpr
- **Expected Output:** Node returned unchanged
- **Rationale:** Ensures plugin is well-behaved

#### Scenario 4: Invalid Configuration
- **Purpose:** Validate error handling for bad config
- **Input:** SafeNavigationExpr with config = "invalid"
- **Expected Output:** Error with descriptive message
- **Rationale:** Configuration validation

#### Scenario 5: Nil Configuration (Default Behavior)
- **Purpose:** Verify default mode is smart unwrapping
- **Input:** SafeNavigationExpr with nil config
- **Expected Output:** Smart mode IIFE
- **Rationale:** Sensible defaults

### 2.2 Null Coalescing Plugin Tests

#### Scenario 1: Option Type with Some
- **Purpose:** Validate Option unwrapping when value present
- **Input:** NullCoalescingExpr with Option type (mocked)
- **Expected Output:** IIFE with IsSome/Unwrap calls
- **Rationale:** Primary Option[T] use case

#### Scenario 2: Pointer Type (Enabled)
- **Purpose:** Validate Go pointer support when enabled
- **Input:** NullCoalescingExpr with *T type and config enabled
- **Expected Output:** IIFE with nil check and dereference
- **Rationale:** Tests pointer feature flag

#### Scenario 3: Pointer Type (Disabled)
- **Purpose:** Verify pointers rejected when config disabled
- **Input:** NullCoalescingExpr with *T type and config disabled
- **Expected Output:** Falls back to Option transformation
- **Rationale:** Configuration boundary testing

#### Scenario 4: Non-NullCoalescing Node
- **Purpose:** Verify plugin doesn't transform unrelated nodes
- **Input:** Regular ast.BinaryExpr
- **Expected Output:** Node returned unchanged
- **Rationale:** Plugin isolation

#### Scenario 5: No Type Information
- **Purpose:** Validate fallback behavior without type info
- **Input:** NullCoalescingExpr with nil TypeInfo
- **Expected Output:** Default Option transformation
- **Rationale:** Graceful degradation

### 2.3 Ternary Plugin Tests

#### Scenario 1: Basic Ternary to IIFE
- **Purpose:** Validate ternary transformation
- **Input:** TernaryExpr with simple condition
- **Expected Output:** IIFE with if-return-else-return
- **Rationale:** Core transformation

#### Scenario 2: Nested Ternary
- **Purpose:** Verify nested ternaries transform correctly
- **Input:** TernaryExpr with ternary in then/else branches
- **Expected Output:** Nested IIFEs
- **Rationale:** Complex expression handling

#### Scenario 3: Standard Precedence Mode
- **Purpose:** Validate precedence configuration read
- **Input:** TernaryExpr with config = "standard"
- **Expected Output:** Transformation succeeds (no validation yet)
- **Rationale:** Configuration integration

#### Scenario 4: Explicit Precedence Mode
- **Purpose:** Validate explicit mode (no-op currently)
- **Input:** TernaryExpr with config = "explicit"
- **Expected Output:** Transformation succeeds
- **Rationale:** Future-proofing for parser validation

#### Scenario 5: Non-Ternary Node
- **Purpose:** Verify plugin doesn't transform unrelated nodes
- **Input:** Regular ast.IfStmt
- **Expected Output:** Node returned unchanged
- **Rationale:** Plugin isolation

### 2.4 Lambda Plugin Tests

#### Scenario 1: Basic Lambda Transformation
- **Purpose:** Validate lambda to func literal
- **Input:** LambdaExpr with single parameter
- **Expected Output:** FuncLit with return statement
- **Rationale:** Core transformation

#### Scenario 2: Multi-Parameter Lambda
- **Purpose:** Verify multiple parameters preserved
- **Input:** LambdaExpr with multiple params
- **Expected Output:** FuncLit with matching params
- **Rationale:** Parameter handling

#### Scenario 3: No-Parameter Lambda
- **Purpose:** Validate empty parameter list
- **Input:** LambdaExpr with nil Params
- **Expected Output:** FuncLit with empty FieldList
- **Rationale:** Edge case handling

#### Scenario 4: Rust Syntax Mode
- **Purpose:** Validate Rust-style acceptance
- **Input:** LambdaExpr with config = "rust"
- **Expected Output:** Transformation succeeds
- **Rationale:** Configuration validation

#### Scenario 5: Invalid Syntax Mode
- **Purpose:** Validate error on invalid config
- **Input:** LambdaExpr with config = "invalid"
- **Expected Output:** Error with descriptive message
- **Rationale:** Configuration validation

#### Scenario 6: Non-Lambda Node
- **Purpose:** Verify plugin doesn't transform unrelated nodes
- **Input:** Regular ast.FuncLit
- **Expected Output:** Node returned unchanged
- **Rationale:** Plugin isolation

### 2.5 Configuration Integration Tests

#### Scenario 1: Default Configuration
- **Purpose:** Verify all plugins work with defaults
- **Input:** All plugins with DefaultConfig()
- **Expected Output:** Expected transformations
- **Rationale:** Default behavior validation

#### Scenario 2: Configuration Validation
- **Purpose:** Verify Config.Validate() catches bad values
- **Input:** Config with invalid lambda_syntax, etc.
- **Expected Output:** Validation errors
- **Rationale:** Config validation testing

#### Scenario 3: Configuration Override
- **Purpose:** Verify config changes affect transformations
- **Input:** Plugins with custom configs
- **Expected Output:** Behavior matches config
- **Rationale:** Configuration integration

---

## 3. Test Implementation Strategy

### 3.1 Unit Tests (pkg/plugin/builtin/*_test.go)

**Files to Create:**
- `safe_navigation_test.go`
- `null_coalescing_test.go`
- `ternary_test.go`
- `lambda_test.go`

**Test Structure Pattern:**
```go
func TestNewXPlugin(t *testing.T)
func TestTransformNonXNode(t *testing.T)
func TestTransformBasicX(t *testing.T)
func TestTransformXWithConfig(t *testing.T)
func TestTransformXEdgeCases(t *testing.T)
```

**Test Helpers:**
- `createTestContext(cfg *config.Config) *plugin.Context`
- `assertNodeType(t *testing.T, node ast.Node, expectedType string)`
- `assertIIFEStructure(t *testing.T, node ast.Node)`

### 3.2 Compilation Tests (tests/golden/*.dingo)

**Note:** Parser integration incomplete, so golden tests will:
- Use existing test framework
- Focus on verifying generated Go compiles
- Check for basic structural correctness
- NOT test end-to-end transpilation (parser limitation)

**Golden Files Created:**
- `safe_nav_01_basic.{dingo,go.golden}` - Already exists
- `null_coalesce_01_basic.{dingo,go.golden}` - Already exists
- `ternary_01_basic.{dingo,go.golden}` - Already exists
- `lambda_01_rust_style.{dingo,go.golden}` - Already exists

### 3.3 AST Structure Validation

**Verification Checklist:**
- IIFE structure: func() T { ... }()
- Correct token positions (for source maps)
- Proper field access generation
- Return statements in correct positions
- Type placeholders (Option_T, interface{})

---

## 4. Test Coverage Summary

### 4.1 Coverage Goals

| Feature | Plugin Tests | Golden Tests | Config Tests | Total Scenarios |
|---------|--------------|--------------|--------------|-----------------|
| Safe Navigation | 5 | 1 | Integrated | 6 |
| Null Coalescing | 5 | 1 | Integrated | 6 |
| Ternary | 5 | 1 | Integrated | 6 |
| Lambda | 6 | 1 | Integrated | 7 |
| **Total** | **21** | **4** | **3** | **28** |

### 4.2 What's Well-Covered

✅ Plugin transformation logic
✅ Configuration integration
✅ Edge case handling (nil configs, wrong node types)
✅ AST structure generation
✅ Error handling and validation

### 4.3 Known Gaps (Accepted Limitations)

❌ End-to-end parsing (parser not integrated)
❌ Type inference (go/types not integrated)
❌ Safe navigation chaining (known bug C3)
❌ Smart mode zero values (needs type inference)
❌ Arrow-style lambda parsing (not implemented)
❌ Precedence validation (deferred to parser)

### 4.4 Confidence Level

**Plugin Correctness: HIGH** - Unit tests verify transformation logic
**AST Generation: HIGH** - Structure validation ensures correct output
**Configuration: HIGH** - Integration and validation well-tested
**End-to-End: LOW** - Parser limitations prevent full testing

**Overall Confidence: MEDIUM-HIGH** for plugin implementation, pending parser integration

---

## 5. Verification Checklist

Before reporting test results, verify:

- [x] Test logic matches requirements from final-plan.md
- [x] Test setup creates valid AST nodes
- [x] Expected values match plugin implementations
- [x] Failures are reproducible
- [x] Can articulate why failures are implementation bugs (not test bugs)

---

## 6. Test Execution Plan

### Phase 1: Unit Tests
1. Run all plugin unit tests: `go test ./pkg/plugin/builtin/`
2. Capture output for each feature
3. Verify pass/fail status
4. Document any failures with evidence

### Phase 2: Configuration Tests
1. Run config tests: `go test ./pkg/config/`
2. Verify validation logic
3. Test default values

### Phase 3: Golden Tests (Compilation Check)
1. Run golden tests: `go test ./tests/`
2. Check if generated Go compiles
3. Note: Not testing transpilation (parser limitation)

### Phase 4: Build Verification
1. Verify entire project builds: `go build ./...`
2. Check for compilation errors
3. Verify no import issues

---

## 7. Test Results Expectations

### Expected Outcomes

**PASS Scenarios:**
- Plugin unit tests for basic transformations
- Configuration validation
- Non-matching node pass-through
- Default configuration handling

**POTENTIAL FAIL Scenarios:**
- Type inference dependent tests (expected, known limitation)
- Complex chaining scenarios (known bug C3)
- Golden tests (parser not integrated)

### Success Criteria

Tests are considered successful if:
1. Plugin transformation logic is verified correct
2. Configuration integration works
3. AST node structure is valid
4. Generated code compiles (when tested manually)
5. Failures are documented with root cause

---

## 8. Documentation Requirements

### Test Results Report Must Include:

For each test:
- **Test Name:** Descriptive identifier
- **Status:** PASS or FAIL
- **If FAIL:**
  - Actual behavior observed
  - Expected behavior per spec
  - Evidence this is implementation bug (not test bug)
  - Root cause analysis
  - Suggested fix (if apparent)

### Summary Statistics:
- Total tests run
- Tests passed
- Tests failed
- Known limitations (not counted as failures)
- Confidence level in implementation

---

## 9. Context Awareness: Dingo Project

**Special Considerations:**
- Generated Go must be idiomatic
- Source maps accuracy (not tested here)
- Transpiled code must compile
- Integration with existing Option types
- Configuration system integration

**Testing Philosophy:**
- Necessary and sufficient coverage
- Focus on critical paths and edge cases
- Document known limitations clearly
- Provide actionable failure reports
- Balance rigor with pragmatism

---

## 10. Next Steps After Testing

### If Tests Pass:
1. Document test coverage
2. Identify remaining work (parser integration)
3. Update changelog
4. Consider phase 2 work (type inference)

### If Tests Fail:
1. Categorize failures: test bug vs implementation bug
2. Fix critical issues (interface implementations, etc.)
3. Document known issues for future work
4. Re-run tests
5. Provide concrete fix recommendations

---

**End of Test Plan**
