# Phase 3: Result/Option Integration - Final Implementation Plan

## Executive Summary

**Phase Name:** Phase 3 - Result/Option Type Integration
**Duration:** 4-5 weeks
**Status:** Ready to Start (Plan Finalized)
**Dependencies:** Phase 2.14 Complete (Preprocessor, Error Propagation, Config System)

**User Decisions Incorporated:**
- ✅ Go Interop: Three-mode configuration system (opt-in/auto/disabled)
- ✅ None Inference: Compilation error with helpful messages
- ✅ Helper Methods: Generate complete API for all types
- ✅ Lambdas: Deferred to Phase 4 (focused approach)

---

## Context Analysis

### What We Have (Phase 2.14 Complete)
1. ✅ **Preprocessor Infrastructure** - Text-based transformation system
   - Error propagation (`?` operator) fully working
   - Import injection with source mapping
   - Multi-value return support
   - Configuration system with validation

2. ✅ **Plugin System** - Modular transformation architecture
   - Plugin registry and dependency resolution
   - AST transformation pipeline
   - Golden test framework (47 tests)

3. ✅ **Sum Types Foundation** - Enum/variant system
   - Pattern matching with destructuring
   - IIFE wrapping for expression contexts
   - Type inference engine
   - Nil safety modes

4. ✅ **Parser** - Full Dingo syntax support
   - Generic types (Result[T, E], Option[T])
   - Type declarations
   - Tuple returns
   - Method calls

### What We Need (Phase 3 Goals)

**P0 Critical Features (Not Yet Integrated):**
- ❌ Result[T, E] type constructor integration (Ok/Err)
- ❌ Option[T] type constructor integration (Some/None)
- ❌ Pattern matching on Result/Option
- ❌ Helper methods (unwrap, unwrapOr, map, etc.)
- ❌ Go interoperability with configurable wrapping modes

### Strategic Approach

**Bottom-Up Implementation:** Implement Result/Option as builtin plugins (like sum_types)

**Rationale:**
- Reuses proven sum_types architecture
- More modular and testable
- Cleaner separation of concerns
- Aligns with existing plugin system

---

## Phase 3 Task Breakdown

### Stage 0: Configuration System Extension (Week 1, Day 1)
**NEW STAGE** - Added based on user decision for go_interop modes
**Duration:** 1 day
**Files:** `pkg/config/config.go`, `dingo.toml`, tests

#### Task 0.1: Go Interop Configuration (1 day)
**Effort:** Low
**Dependencies:** None

**Implementation:**
Extend `dingo.toml` configuration to support three-mode go_interop:

```toml
[features.result_type]
enabled = true
go_interop = "opt-in"  # Options: "opt-in", "auto", "disabled"

[features.option_type]
enabled = true
go_interop = "opt-in"  # Options: "opt-in", "auto", "disabled"
```

**Modes:**
1. **`"opt-in"`** (default) - Manual wrapping required
   - Users call `Result.FromGo(fetchUser(id))` explicitly
   - Safe, predictable, no surprises
   - Best for existing Go codebases

2. **`"auto"`** - Automatic wrapping
   - `(T, error)` → `Result[T, E]` automatically
   - `*T` → `Option[T]` automatically
   - Convenient for greenfield Dingo projects

3. **`"disabled"`** - No Go interop
   - Pure Dingo types only
   - No wrapping functionality generated
   - Minimal binary size

**Configuration Validation:**
- Enum validation: Only "opt-in", "auto", "disabled" allowed
- Default to "opt-in" if not specified
- Error on invalid values with helpful message

**Success Criteria:**
- Configuration loads correctly
- Invalid values rejected with clear errors
- Unit tests: 6+ test cases (valid/invalid configs)

**Files to Modify:**
- MODIFY: `pkg/config/config.go` (~50 lines)
- MODIFY: `pkg/config/config_test.go` (~40 lines)
- MODIFY: `dingo.toml` (example config)

---

### Stage 1: Result Type Plugin (Week 1-2)
**Duration:** 8-10 days
**Files:** `pkg/plugin/builtin/result_type.go`, tests, golden files

#### Task 1.1: Result Type Declaration Generator (3 days)
**Effort:** Medium
**Dependencies:** Task 0.1

**Implementation:**
1. Create `ResultTypePlugin` in `pkg/plugin/builtin/`
2. Register with dependency on `SumTypesPlugin`
3. Implement type declaration emission:
   ```go
   type Result_T_E struct {
       tag ResultTag
       ok_0 T
       err_0 E
   }
   type ResultTag uint8
   const (
       ResultTag_Ok ResultTag = iota
       ResultTag_Err
   )
   ```
4. Type name sanitization (same logic as sum_types)
5. Generic type parameter handling (IndexExpr/IndexListExpr)

**Success Criteria:**
- Generates Result declarations for all used types
- Compiles without errors
- Unit tests: 10+ test cases

**Files to Create/Modify:**
- NEW: `pkg/plugin/builtin/result_type.go` (~450 lines - increased for config)
- NEW: `pkg/plugin/builtin/result_type_test.go` (~220 lines)
- MODIFY: `pkg/plugin/builtin/builtin.go` (register plugin)

#### Task 1.2: Ok/Err Constructor Transformation (2 days)
**Effort:** Low-Medium
**Dependencies:** Task 1.1

**Implementation:**
1. Detect `Ok(value)` and `Err(error)` function calls
2. Use type inference to determine T and E
3. Transform to struct literal:
   ```go
   Result_T_E{tag: ResultTag_Ok, ok_0: value}
   Result_T_E{tag: ResultTag_Err, err_0: error}
   ```
4. Handle edge cases (nested, chained, expression contexts)

**Success Criteria:**
- `result_01_basic.dingo` passes
- All Ok/Err calls correctly transformed
- Type inference works for common cases

**Golden Tests to Pass:**
- ✅ `result_01_basic.dingo` - Basic Ok/Err usage

#### Task 1.3: Helper Methods Generation (2 days)
**Effort:** Medium
**Dependencies:** Task 1.2

**Implementation:**
**USER DECISION:** Generate ALL helper methods (complete API)

Generate complete method set for each Result type:
```go
// Basic predicates
func (r Result_T_E) IsOk() bool { return r.tag == ResultTag_Ok }
func (r Result_T_E) IsErr() bool { return r.tag == ResultTag_Err }

// Unwrapping
func (r Result_T_E) Unwrap() T {
    if r.tag != ResultTag_Ok { panic("called Unwrap on Err") }
    return r.ok_0
}
func (r Result_T_E) UnwrapOr(defaultValue T) T {
    if r.tag == ResultTag_Ok { return r.ok_0 }
    return defaultValue
}
func (r Result_T_E) UnwrapErr() E {
    if r.tag != ResultTag_Err { panic("called UnwrapErr on Ok") }
    return r.err_0
}

// Transformations
func (r Result_T_E) Map(fn func(T) U) Result_U_E {
    if r.tag == ResultTag_Ok {
        return Result_U_E{tag: ResultTag_Ok, ok_0: fn(r.ok_0)}
    }
    return Result_U_E{tag: ResultTag_Err, err_0: r.err_0}
}
func (r Result_T_E) MapErr(fn func(E) F) Result_T_F {
    if r.tag == ResultTag_Err {
        return Result_T_F{tag: ResultTag_Err, err_0: fn(r.err_0)}
    }
    return Result_T_F{tag: ResultTag_Ok, ok_0: r.ok_0}
}
func (r Result_T_E) And(other Result_U_E) Result_U_E {
    if r.tag == ResultTag_Ok { return other }
    return Result_U_E{tag: ResultTag_Err, err_0: r.err_0}
}
func (r Result_T_E) Or(other Result_T_E) Result_T_E {
    if r.tag == ResultTag_Ok { return r }
    return other
}
```

**Success Criteria:**
- All helper methods generated
- Methods work correctly in golden tests
- Panic messages are clear and actionable

**Golden Tests to Pass:**
- ✅ `result_02_propagation.dingo` - Uses IsOk/IsErr
- ✅ `result_04_chaining.dingo` - Uses Map, And, Or

#### Task 1.4: Pattern Matching Integration (2 days)
**Effort:** Medium
**Dependencies:** Task 1.3

**Implementation:**
1. Extend pattern matching to recognize Result variants
2. Destructure to access ok_0 and err_0 fields
3. Generate type guards (tag checks)
4. Integration test with sum_types plugin

**Pattern Syntax:**
```dingo
match result {
    Ok(value) => println(value),
    Err(error) => println(error)
}
```

**Success Criteria:**
- Pattern matching works on Result types
- Exhaustiveness checking works
- Destructuring accesses correct fields

**Golden Tests to Pass:**
- ✅ `result_03_pattern_match.dingo`

#### Task 1.5: Go Interoperability (3 days)
**Effort:** Medium-High
**Dependencies:** Task 1.4, Task 0.1 (configuration)

**Implementation:**
**USER DECISION:** Three-mode configuration system

**Mode 1: opt-in (default)**
Require explicit wrapping:
```dingo
user := Result.FromGo(fetchUser(id))  // fetchUser returns (User, error)
```

Transforms to:
```go
user := func() Result_User_error {
    __tmp0, __err0 := fetchUser(id)
    if __err0 != nil { return Result_User_error{tag: ResultTag_Err, err_0: __err0} }
    return Result_User_error{tag: ResultTag_Ok, ok_0: __tmp0}
}()
```

**Mode 2: auto**
Automatically wrap Go function returns:
```dingo
user := fetchUser(id)  // Automatically wrapped to Result
```

Same generated code as above, but triggered automatically.

**Mode 3: disabled**
No wrapping functionality. Pure Dingo types only.

**Type Detection:**
1. Use go/types to detect `(T, error)` returns
2. Only wrap when second return is assignable to error
3. Preserve type information for source maps

**Configuration Check:**
```go
config := ctx.Config.Features.ResultType.GoInterop
switch config {
case "opt-in":
    // Only wrap Result.FromGo() calls
case "auto":
    // Wrap all (T, error) returns
case "disabled":
    // No wrapping
}
```

**Success Criteria:**
- All three modes work correctly
- opt-in mode requires explicit FromGo calls
- auto mode wraps automatically
- disabled mode generates no wrapper code
- Type detection is accurate (no false positives)

**Golden Tests to Pass:**
- ✅ `result_05_go_interop.dingo` (tests all three modes)

---

### Stage 2: Option Type Plugin (Week 2-3)
**Duration:** 6-8 days
**Files:** `pkg/plugin/builtin/option_type.go`, tests, golden files

#### Task 2.1: Option Type Declaration Generator (2 days)
**Effort:** Low-Medium
**Dependencies:** Stage 1 complete

**Implementation:**
Very similar to Result, but simpler (one type parameter):
```go
type Option_T struct {
    tag OptionTag
    some_0 T
}
type OptionTag uint8
const (
    OptionTag_Some OptionTag = iota
    OptionTag_None
)
```

**Success Criteria:**
- Generates Option declarations
- Unit tests: 8+ test cases

**Files to Create/Modify:**
- NEW: `pkg/plugin/builtin/option_type.go` (~400 lines - increased for config)
- NEW: `pkg/plugin/builtin/option_type_test.go` (~200 lines)

#### Task 2.2: Some/None Constructor Transformation (2 days)
**Effort:** Low-Medium
**Dependencies:** Task 2.1

**Implementation:**
1. Transform `Some(value)` → `Option_T{tag: OptionTag_Some, some_0: value}`
2. Transform `None` → **SPECIAL CASE** (requires type context)

**USER DECISION:** None without type context → Compilation error

**None Type Inference Strategy:**
1. Check for explicit type annotation: `let x: Option[string] = None` ✅
2. Check for function return type: `func foo() -> Option[int] { return None }` ✅
3. Check for assignment to typed variable: `var x Option[bool]; x = None` ✅
4. **OTHERWISE:** Emit compilation error

**Error Message:**
```
Error: Cannot infer type for None
  --> file.dingo:12:15
   |
12 |     x := None
   |             ^^^^ type cannot be determined
   |
Help: Add explicit type annotation:
      let x: Option[YourType] = None
```

**Success Criteria:**
- Some/None constructors work with type context
- None fails gracefully without context
- Error messages are helpful and actionable
- Type inference succeeds in 90%+ of real-world cases

**Golden Tests to Pass:**
- ✅ `option_01_basic.dingo` (includes error cases)

#### Task 2.3: Helper Methods Generation (2 days)
**Effort:** Low-Medium
**Dependencies:** Task 2.2

**Implementation:**
**USER DECISION:** Generate ALL helper methods (complete API)

```go
// Basic predicates
func (o Option_T) IsSome() bool { return o.tag == OptionTag_Some }
func (o Option_T) IsNone() bool { return o.tag == OptionTag_None }

// Unwrapping
func (o Option_T) Unwrap() T {
    if o.tag != OptionTag_Some { panic("called Unwrap on None") }
    return o.some_0
}
func (o Option_T) UnwrapOr(defaultValue T) T {
    if o.tag == OptionTag_Some { return o.some_0 }
    return defaultValue
}

// Transformations
func (o Option_T) Map(fn func(T) U) Option_U {
    if o.tag == OptionTag_Some {
        return Option_U{tag: OptionTag_Some, some_0: fn(o.some_0)}
    }
    return Option_U{tag: OptionTag_None}
}
func (o Option_T) Filter(fn func(T) bool) Option_T {
    if o.tag == OptionTag_Some && fn(o.some_0) {
        return o
    }
    return Option_T{tag: OptionTag_None}
}
func (o Option_T) And(other Option_U) Option_U {
    if o.tag == OptionTag_Some { return other }
    return Option_U{tag: OptionTag_None}
}
func (o Option_T) Or(other Option_T) Option_T {
    if o.tag == OptionTag_Some { return o }
    return other
}
```

**Success Criteria:**
- All helper methods generated
- Methods work in golden tests
- Panic messages clear

**Golden Tests to Pass:**
- ✅ `option_03_chaining.dingo` - Uses Map/Filter

#### Task 2.4: Pattern Matching Integration (1 day)
**Effort:** Low
**Dependencies:** Task 2.3

**Implementation:**
Same as Result, but simpler (no error type).

**Pattern Syntax:**
```dingo
match option {
    Some(value) => println(value),
    None => println("nothing")
}
```

**Golden Tests to Pass:**
- ✅ `option_02_pattern_match.dingo`

#### Task 2.5: Go Interoperability (2 days)
**Effort:** Medium
**Dependencies:** Task 2.4

**Implementation:**
**USER DECISION:** Three-mode configuration system (same as Result)

**Mode 1: opt-in (default)**
```dingo
user := Option.FromPtr(findUser(id))  // findUser returns *User
```

**Mode 2: auto**
```dingo
user := findUser(id)  // Automatically wrapped to Option[User]
```

**Mode 3: disabled**
No wrapping functionality.

**Transformation:**
```go
// Generated wrapper for pointer → Option
user := func() Option_User {
    __tmp0 := findUser(id)
    if __tmp0 == nil {
        return Option_User{tag: OptionTag_None}
    }
    return Option_User{tag: OptionTag_Some, some_0: *__tmp0}
}()
```

**Configuration Check:**
```go
config := ctx.Config.Features.OptionType.GoInterop
switch config {
case "opt-in":
    // Only wrap Option.FromPtr() calls
case "auto":
    // Wrap all *T returns
case "disabled":
    // No wrapping
}
```

**Success Criteria:**
- All three modes work correctly
- Pointer wrapping works when enabled
- Type detection accurate
- No wrapping for non-pointer types

**Golden Tests to Pass:**
- ✅ `option_04_go_interop.dingo` (tests all three modes)

---

### Stage 3: Integration & Polish (Week 4)
**Duration:** 5-7 days

#### Task 3.1: Error Propagation Integration (2 days)
**Effort:** Medium
**Dependencies:** Stage 1 complete

**Implementation:**
Make `?` operator work with Result types:
```dingo
func processUser(id: string) -> Result[User, Error] {
    user := fetchUser(id)?  // Propagate Err variant
    return Ok(user)
}
```

Transform to:
```go
func processUser(id string) Result_User_Error {
    __tmp0 := fetchUser(id)
    if __tmp0.tag == ResultTag_Err {
        return __tmp0  // Early return with Err
    }
    user := __tmp0.ok_0
    return Result_User_Error{tag: ResultTag_Ok, ok_0: user}
}
```

**Success Criteria:**
- `?` operator works on Result types
- Works in statement and expression contexts
- Preserves error type
- Integrates with existing error_prop preprocessor

**Golden Tests to Pass:**
- ✅ `result_02_propagation.dingo` (enhanced)

#### Task 3.2: Null Safety Integration (2 days)
**Effort:** Medium
**Dependencies:** Stage 2 complete

**Implementation:**
Make `?.` operator work with Option types:
```dingo
name := user?.name  // Returns Option[string]
```

Transform to:
```go
var name Option_string
if user.tag == OptionTag_Some {
    name = Option_string{tag: OptionTag_Some, some_0: user.some_0.name}
} else {
    name = Option_string{tag: OptionTag_None}
}
```

**Success Criteria:**
- Safe navigation returns Option
- Chaining works: `user?.address?.city`
- Type inference correct
- Integrates with existing safe_nav transform

**Golden Tests to Pass:**
- ✅ `safe_nav_01_basic.dingo` (enhanced with Option integration)

#### Task 3.3: Comprehensive Testing (2 days)
**Effort:** Medium
**Dependencies:** All tasks complete

**Activities:**
1. Run all 47 golden tests
2. Verify Result tests pass: result_01 through result_05 (5 tests)
3. Verify Option tests pass: option_01 through option_04 (4 tests)
4. Regression test error_prop suite (9 tests)
5. Integration test sum_types suite (5 tests)
6. Test all three go_interop modes (opt-in/auto/disabled)
7. Test None type inference error messages
8. Fix any failures

**Success Criteria:**
- 90%+ golden test pass rate
- All Result/Option tests passing
- No regressions in Phase 2 features
- All three config modes work correctly

#### Task 3.4: Documentation & Code Review (1 day)
**Effort:** Low
**Dependencies:** Task 3.3

**Activities:**
1. Update CHANGELOG.md with Phase 3 changes
2. Document Result/Option plugin architecture
3. Document go_interop configuration modes
4. Add inline comments for complex transformations
5. Run external code review (code-reviewer agent)
6. Fix critical issues

**Success Criteria:**
- Documentation complete
- Configuration modes documented
- Code review shows no CRITICAL issues
- All IMPORTANT issues addressed or documented

---

## Estimated Timeline

### Week 1: Configuration & Result Foundation
- Day 1: Task 0.1 (Go interop configuration)
- Days 2-4: Task 1.1 (Result type declarations)
- Days 5: Task 1.2 (Ok/Err constructors)

### Week 2: Result Type Completion
- Days 1-2: Task 1.3 (Helper methods)
- Days 3-4: Task 1.4 (Pattern matching)
- Days 5-7: Task 1.5 (Go interop with three modes)

### Week 3: Option Type
- Days 1-2: Task 2.1 (Type declarations)
- Days 3-4: Task 2.2 (Some/None with error handling)
- Day 5: Task 2.3 (Helper methods)

### Week 4: Option Completion & Integration
- Day 1: Task 2.4 (Pattern matching)
- Days 2-3: Task 2.5 (Go interop)
- Days 4-5: Stage 3 (Integration & testing)

**Total Duration:** 4-5 weeks

---

## Success Criteria

### Must Have (P0)
- [ ] All Result golden tests passing (5/5)
- [ ] All Option golden tests passing (4/4)
- [ ] Zero regressions in existing tests
- [ ] Generated Go code compiles without errors
- [ ] Three go_interop modes working (opt-in/auto/disabled)
- [ ] None type inference errors are helpful
- [ ] All helper methods generated

### Should Have (P1)
- [ ] Go interop works in all three modes
- [ ] Integration with `?` operator
- [ ] Integration with `?.` operator
- [ ] 90%+ golden test pass rate overall
- [ ] Configuration validation with clear errors

### Nice to Have (P2)
- [ ] Helper method chaining optimizations
- [ ] Zero allocation for unwrap operations
- [ ] Comprehensive error messages for type inference failures
- [ ] Documentation with examples for each mode

---

## Risk Assessment

### High Risks

1. **Type Inference Complexity (None)**
   - **Risk:** None type requires surrounding context
   - **User Decision:** Compilation error when context unavailable
   - **Mitigation:** Clear, helpful error messages
   - **Fallback:** Users add explicit type annotations

2. **Go Interop Edge Cases**
   - **Risk:** Auto mode might break some Go patterns
   - **User Decision:** Three-mode configuration (opt-in default)
   - **Mitigation:** Default to safe opt-in mode
   - **Fallback:** Users can disable entirely

3. **Performance Overhead**
   - **Risk:** Extra struct allocations vs raw (T, error)
   - **Mitigation:** Profile and optimize hot paths
   - **Fallback:** Document tradeoffs, users choose

### Medium Risks

1. **Pattern Matching Integration**
   - **Risk:** Complex interaction with sum_types plugin
   - **Mitigation:** Extensive integration tests

2. **Helper Method Code Size**
   - **Risk:** Generating all methods increases binary size
   - **User Decision:** Accept tradeoff for complete API
   - **Future:** Dead code elimination in later phase

### Low Risks

1. **Plugin Registration** - Well-understood pattern
2. **Constructor Transformation** - Similar to sum_types
3. **Testing** - Golden test framework proven
4. **Configuration System** - Existing infrastructure

---

## User Decisions Summary

All open questions resolved:

### 1. Go Interoperability
**Decision:** Three-mode configuration system
- `"opt-in"` (default) - Explicit wrapping required
- `"auto"` - Automatic wrapping
- `"disabled"` - No Go interop

**Rationale:** Flexibility for different use cases, safe default

### 2. None Type Inference
**Decision:** Compilation error when type cannot be inferred
**Rationale:** Type safety over convenience, forces clarity

### 3. Helper Methods Generation
**Decision:** Generate all methods (complete API)
**Rationale:** Simplicity, complete API surface, optimization later

### 4. Lambda Integration
**Decision:** Keep in Phase 4 (separate)
**Rationale:** Focused timeline, lambdas orthogonal to Result/Option

---

## Dependencies

### External Dependencies
- None (uses existing go/types, go/ast)

### Internal Dependencies
- ✅ pkg/plugin (plugin system)
- ✅ pkg/plugin/builtin/sum_types (pattern matching)
- ✅ pkg/plugin/builtin/type_inference (type detection)
- ✅ pkg/preprocessor/error_prop (`?` operator)
- ✅ pkg/transform/safe_nav (`?.` operator)
- ✅ pkg/config (configuration system) - EXTENDED in Task 0.1

### New Dependencies to Create
- pkg/plugin/builtin/result_type
- pkg/plugin/builtin/option_type

---

## Code Quality Standards

### Must Follow
1. Unit test coverage: 80%+ for new code
2. Golden test coverage: All Result/Option scenarios + all config modes
3. Code review: External LLM review before completion
4. Documentation: Inline comments for complex logic
5. Error handling: Graceful degradation, helpful error messages
6. Configuration validation: Clear errors for invalid modes

### Review Checkpoints
1. After Task 0.1: Review configuration system design
2. After Task 1.2: Review Result constructor logic
3. After Task 1.5: Review Go interop strategy (all three modes)
4. After Task 2.2: Review None type inference error handling
5. After Stage 2: Review Option implementation
6. After Stage 3: Full code review

---

## Alternatives Considered

### Alternative 1: Preprocessor-Based Approach
**Rejected because:**
- Preprocessor is text-based, harder to do type inference
- Less modular than plugin approach
- Error propagation in preprocessor was necessary (text transformation), but Result/Option are AST-level features

### Alternative 2: Runtime Library Approach
**Rejected because:**
- Violates "zero runtime overhead" principle
- Adds dependency users must import
- Goes against Dingo philosophy (transpile to clean Go)

### Alternative 3: Manual Type Annotations Only
**Rejected because:**
- Type inference is essential for ergonomics
- Users expect Rust/Swift-like inference
- Defeats purpose of reducing boilerplate

### Alternative 4: Single go_interop Mode (auto only or opt-in only)
**Rejected because:**
- Different projects have different needs
- User requested both approaches
- Configuration system allows flexibility

### Alternative 5: Generate Only Used Methods
**Rejected for Phase 3 because:**
- User decision: Simplicity over optimization
- Complete API is more ergonomic
- Optimization can be added in later phase

---

## Post-Phase 3 Roadmap

### Phase 4: Advanced Features (Next)
**USER DECISION:** Lambdas moved to Phase 4
- Lambda functions (lambda_01-04 tests)
- Null coalescing operator (null_coalesce_01-03)
- Ternary operator (ternary_01-03)
- Tuples (tuples_01-03)

### Phase 5: Language Server
- gopls proxy architecture
- Source map integration
- IDE support (autocomplete, navigation)

---

## Appendix A: File Structure

```
pkg/
├── config/
│   ├── config.go                   (MODIFY: add go_interop modes)
│   └── config_test.go              (MODIFY: test validation)
│
├── plugin/builtin/
│   ├── builtin.go                  (MODIFY: register new plugins)
│   ├── result_type.go              (NEW: ~450 lines)
│   ├── result_type_test.go         (NEW: ~220 lines)
│   ├── option_type.go              (NEW: ~400 lines)
│   ├── option_type_test.go         (NEW: ~200 lines)
│   ├── sum_types.go                (REFERENCE: pattern to follow)
│   └── type_inference.go           (USE: for type detection)
│
└── preprocessor/
    └── error_prop/                 (MODIFY: integrate with Result)

tests/golden/
├── result_01_basic.dingo               (EXISTS: needs to pass)
├── result_01_basic.go.golden           (UPDATE: with config modes)
├── result_02_propagation.dingo         (EXISTS: needs to pass)
├── result_03_pattern_match.dingo       (EXISTS: needs to pass)
├── result_04_chaining.dingo            (EXISTS: needs to pass)
├── result_05_go_interop.dingo          (EXISTS: UPDATE with 3 modes)
├── option_01_basic.dingo               (EXISTS: needs to pass)
├── option_01_basic.go.golden           (UPDATE: with error cases)
├── option_02_pattern_match.dingo       (EXISTS: needs to pass)
├── option_03_chaining.dingo            (EXISTS: needs to pass)
└── option_04_go_interop.dingo          (EXISTS: UPDATE with 3 modes)

dingo.toml                              (UPDATE: add go_interop config)
```

**Total New Code Estimate:**
- Production: ~1,750 lines (2 plugins + config + integration)
- Tests: ~700 lines (unit + integration + config tests)
- Total: ~2,450 lines

**Increase from initial plan:** +350 lines (config system, error handling, three modes)

---

## Appendix B: Configuration Examples

### Example 1: Safe Default (opt-in)
```toml
[features.result_type]
enabled = true
go_interop = "opt-in"  # Explicit wrapping required

[features.option_type]
enabled = true
go_interop = "opt-in"
```

**Usage:**
```dingo
// Requires explicit wrapping
user := Result.FromGo(fetchUser(id))
data := Option.FromPtr(findData(key))
```

### Example 2: Automatic Wrapping
```toml
[features.result_type]
enabled = true
go_interop = "auto"  # Automatic wrapping

[features.option_type]
enabled = true
go_interop = "auto"
```

**Usage:**
```dingo
// Automatically wrapped
user := fetchUser(id)  // → Result[User, error]
data := findData(key)  // → Option[Data]
```

### Example 3: Pure Dingo (disabled)
```toml
[features.result_type]
enabled = true
go_interop = "disabled"  # No Go interop

[features.option_type]
enabled = true
go_interop = "disabled"
```

**Usage:**
```dingo
// No wrapping available, pure Dingo types only
// Must use Dingo functions that return Result/Option
user := dingoFetchUser(id)  // Already returns Result
```

---

## Appendix C: None Type Inference Error Examples

### Error Case 1: No Context
```dingo
x := None  // ERROR
```

**Error Message:**
```
Error: Cannot infer type for None
  --> example.dingo:3:9
   |
 3 | x := None
   |         ^^^^ type cannot be determined
   |
Help: Add explicit type annotation:
      let x: Option[YourType] = None
```

### Success Case 1: Explicit Type
```dingo
let x: Option[string] = None  // ✅ OK
```

### Success Case 2: Function Return
```dingo
func findUser(id: string) -> Option[User] {
    return None  // ✅ OK - type inferred from return type
}
```

### Success Case 3: Typed Assignment
```dingo
var x: Option[int]
x = None  // ✅ OK - type inferred from variable declaration
```

---

## Appendix D: Similar Projects Reference

### Rust
- Result/Option are core language types
- Pattern matching is idiomatic
- `?` operator is widely used
- **Lesson:** Complete API surface matters

### Swift
- Optional type (similar to Option)
- Optional chaining (similar to `?.`)
- Guard statements for unwrapping
- **Lesson:** Type inference critical for ergonomics

### Kotlin
- Nullable types (T?)
- Elvis operator (`?:`) similar to `??`
- Safe call operator (`?.`)
- **Lesson:** Multiple interop modes useful

**Overall Lesson:** All modern languages converge on these patterns. Dingo bringing them to Go with configurable interop is valuable and practical.

---

**End of Final Plan**

**Plan Status:** ✅ Complete - All user decisions incorporated
**Ready for Implementation:** Yes
**Next Step:** Begin Task 0.1 (Configuration System Extension)
