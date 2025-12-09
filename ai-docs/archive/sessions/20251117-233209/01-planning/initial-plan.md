# Phase 3: Result/Option Integration - Architectural Plan

## Executive Summary

**Phase Name:** Phase 3 - Result/Option Type Integration
**Duration:** 4-5 weeks
**Status:** Ready to Start
**Dependencies:** Phase 2.14 Complete (Preprocessor, Error Propagation, Config System)

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
According to CLAUDE.md, features/INDEX.md, and the feature specs:

**P0 Critical Features (Not Yet Integrated):**
- ❌ Result[T, E] type constructor integration (Ok/Err)
- ❌ Option[T] type constructor integration (Some/None)
- ❌ Pattern matching on Result/Option
- ❌ Helper methods (unwrap, unwrapOr, map, etc.)
- ❌ Go interoperability (auto-wrap `(T, error)` → `Result[T, E]`)

### Strategic Decision Point

We have **golden tests** for Result/Option (result_01-05, option_01-04) but they're **not passing** because:
1. Result/Option type declarations not generated
2. Ok/Err/Some/None constructors not transformed
3. Helper methods not emitted
4. Pattern matching not integrated with Result/Option

**Two possible approaches:**
1. **Bottom-Up:** Implement Result/Option as builtin plugins (like sum_types)
2. **Top-Down:** Extend preprocessor to handle Result/Option as special cases

**Recommendation:** **Bottom-Up (Builtin Plugins)** because:
- Reuses proven sum_types architecture
- More modular and testable
- Cleaner separation of concerns
- Aligns with existing plugin system

---

## Phase 3 Task Breakdown

### Stage 1: Result Type Plugin (Week 1-2)
**Duration:** 8-10 days
**Files:** `pkg/plugin/builtin/result_type.go`, tests, golden files

#### Task 1.1: Result Type Declaration Generator (3 days)
**Effort:** Medium
**Dependencies:** None

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
- NEW: `pkg/plugin/builtin/result_type.go` (~400 lines)
- NEW: `pkg/plugin/builtin/result_type_test.go` (~200 lines)
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
Generate helper methods for each Result type:
```go
func (r Result_T_E) IsOk() bool { return r.tag == ResultTag_Ok }
func (r Result_T_E) IsErr() bool { return r.tag == ResultTag_Err }
func (r Result_T_E) Unwrap() T {
    if r.tag != ResultTag_Ok { panic("called Unwrap on Err") }
    return r.ok_0
}
func (r Result_T_E) UnwrapOr(defaultValue T) T {
    if r.tag == ResultTag_Ok { return r.ok_0 }
    return defaultValue
}
func (r Result_T_E) Map(fn func(T) U) Result_U_E { ... }
```

**Success Criteria:**
- All helper methods generated
- Methods work correctly in golden tests
- Panic messages are clear

**Golden Tests to Pass:**
- ✅ `result_02_propagation.dingo` - Uses IsOk/IsErr
- ✅ `result_04_chaining.dingo` - Uses Map

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

#### Task 1.5: Go Interoperability (2 days)
**Effort:** Medium-High
**Dependencies:** Task 1.4

**Implementation:**
1. Detect Go functions returning `(T, error)`
2. Auto-wrap in Result constructor:
   ```dingo
   let user = fetchUser(id)  // fetchUser returns (User, error)
   // Transforms to:
   let user = (func() Result_User_error {
       __tmp0, __err0 := fetchUser(id)
       if __err0 != nil { return Err(__err0) }
       return Ok(__tmp0)
   })()
   ```
3. Configuration flag: `auto_wrap_go_errors` (default: false)
4. Type detection using go/types

**Success Criteria:**
- Go functions auto-wrap when flag enabled
- No wrapping when flag disabled
- Type detection is accurate

**Golden Tests to Pass:**
- ✅ `result_05_go_interop.dingo`

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
- NEW: `pkg/plugin/builtin/option_type.go` (~350 lines)
- NEW: `pkg/plugin/builtin/option_type_test.go` (~180 lines)

#### Task 2.2: Some/None Constructor Transformation (2 days)
**Effort:** Low-Medium
**Dependencies:** Task 2.1

**Implementation:**
1. Transform `Some(value)` → `Option_T{tag: OptionTag_Some, some_0: value}`
2. Transform `None` → **SPECIAL CASE** (no value, need type context)
3. Type inference for None (look at surrounding context)
4. Fallback: `Option_interface{}{tag: OptionTag_None}` if type unknown

**Success Criteria:**
- Some/None constructors work
- Type inference for None succeeds in 90% of cases

**Golden Tests to Pass:**
- ✅ `option_01_basic.dingo`

#### Task 2.3: Helper Methods Generation (2 days)
**Effort:** Low-Medium
**Dependencies:** Task 2.2

**Implementation:**
```go
func (o Option_T) IsSome() bool { return o.tag == OptionTag_Some }
func (o Option_T) IsNone() bool { return o.tag == OptionTag_None }
func (o Option_T) Unwrap() T { ... }
func (o Option_T) UnwrapOr(defaultValue T) T { ... }
func (o Option_T) Map(fn func(T) U) Option_U { ... }
func (o Option_T) Filter(fn func(T) bool) Option_T { ... }
```

**Success Criteria:**
- All helper methods generated
- Methods work in golden tests

**Golden Tests to Pass:**
- ✅ `option_03_chaining.dingo` - Uses Map/Filter

#### Task 2.4: Pattern Matching Integration (1 day)
**Effort:** Low
**Dependencies:** Task 2.3

**Implementation:**
Same as Result, but simpler (no error type).

**Golden Tests to Pass:**
- ✅ `option_02_pattern_match.dingo`

#### Task 2.5: Go Interoperability (2 days)
**Effort:** Medium
**Dependencies:** Task 2.4

**Implementation:**
Auto-wrap Go pointers to Option:
```dingo
let user = findUser(id)  // returns *User
// Wraps to:
let user = (func() Option_User {
    __tmp0 := findUser(id)
    if __tmp0 == nil { return None }
    return Some(*__tmp0)
})()
```

Configuration flag: `auto_wrap_go_nils` (default: false)

**Success Criteria:**
- Pointer wrapping works when enabled
- Type detection accurate

**Golden Tests to Pass:**
- ✅ `option_04_go_interop.dingo`

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
    let user = fetchUser(id)?  // Propagate Err variant
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

**Golden Tests to Pass:**
- ✅ `result_02_propagation.dingo` (enhanced)

#### Task 3.2: Null Safety Integration (2 days)
**Effort:** Medium
**Dependencies:** Stage 2 complete

**Implementation:**
Make `?.` operator work with Option types:
```dingo
let name = user?.name  // Returns Option[string]
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

**Golden Tests to Pass:**
- ✅ `safe_nav_01_basic.dingo` (enhanced)

#### Task 3.3: Comprehensive Testing (2 days)
**Effort:** Medium
**Dependencies:** All tasks complete

**Activities:**
1. Run all 47 golden tests
2. Verify Result tests pass: result_01 through result_05 (5 tests)
3. Verify Option tests pass: option_01 through option_04 (4 tests)
4. Regression test error_prop suite (9 tests)
5. Integration test sum_types suite (5 tests)
6. Fix any failures

**Success Criteria:**
- 90%+ golden test pass rate
- All Result/Option tests passing
- No regressions in Phase 2 features

#### Task 3.4: Documentation & Code Review (1 day)
**Effort:** Low
**Dependencies:** Task 3.3

**Activities:**
1. Update CHANGELOG.md with Phase 3 changes
2. Document Result/Option plugin architecture
3. Add inline comments for complex transformations
4. Run external code review (GPT-5 Codex)
5. Fix critical issues

**Success Criteria:**
- Documentation complete
- Code review shows no CRITICAL issues
- All IMPORTANT issues addressed or documented

---

## Estimated Timeline

### Week 1: Result Type Foundation
- Days 1-3: Task 1.1 (Type declarations)
- Days 4-5: Task 1.2 (Constructors)

### Week 2: Result Type Completion
- Days 1-2: Task 1.3 (Helper methods)
- Days 3-4: Task 1.4 (Pattern matching)
- Days 5: Task 1.5 (Go interop)

### Week 3: Option Type
- Days 1-2: Task 2.1 (Type declarations)
- Days 3-4: Task 2.2 (Constructors)
- Days 5: Task 2.3 (Helper methods)

### Week 4: Option Completion & Integration
- Day 1: Task 2.4 (Pattern matching)
- Days 2-3: Task 2.5 (Go interop)
- Days 4-5: Stage 3 (Integration & testing)

**Total Duration:** 4-5 weeks (depending on complexity discoveries)

---

## Success Criteria

### Must Have (P0)
- [ ] All Result golden tests passing (5/5)
- [ ] All Option golden tests passing (4/4)
- [ ] Zero regressions in existing tests
- [ ] Generated Go code compiles without errors
- [ ] Type inference works for common cases

### Should Have (P1)
- [ ] Go interop works correctly (auto-wrap enabled)
- [ ] Integration with `?` operator
- [ ] Integration with `?.` operator
- [ ] 90%+ golden test pass rate overall

### Nice to Have (P2)
- [ ] Helper method chaining optimizations
- [ ] Zero allocation for unwrap operations
- [ ] Comprehensive error messages for type inference failures

---

## Risk Assessment

### High Risks
1. **Type Inference Complexity**
   - Risk: None type requires surrounding context
   - Mitigation: Start with explicit types, add inference incrementally
   - Fallback: Use interface{} when type unknown

2. **Go Interop Edge Cases**
   - Risk: Auto-wrapping might break some Go patterns
   - Mitigation: Make it opt-in via config flag
   - Fallback: Manual wrapping always available

3. **Performance Overhead**
   - Risk: Extra struct allocations vs raw (T, error)
   - Mitigation: Profile and optimize hot paths
   - Fallback: Document tradeoffs, users choose

### Medium Risks
1. **Pattern Matching Integration**
   - Risk: Complex interaction with sum_types plugin
   - Mitigation: Extensive integration tests

2. **Helper Method Code Size**
   - Risk: Generating many methods increases binary size
   - Mitigation: Only generate used methods (future optimization)

### Low Risks
1. **Plugin Registration** - Well-understood pattern
2. **Constructor Transformation** - Similar to sum_types
3. **Testing** - Golden test framework proven

---

## Open Questions for User

See `gaps.json` for structured questions.

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

### New Dependencies to Create
- pkg/plugin/builtin/result_type
- pkg/plugin/builtin/option_type

---

## Code Quality Standards

### Must Follow
1. Unit test coverage: 80%+ for new code
2. Golden test coverage: All Result/Option scenarios
3. Code review: External LLM review before completion
4. Documentation: Inline comments for complex logic
5. Error handling: Graceful degradation when type inference fails

### Review Checkpoints
1. After Task 1.2: Review Result constructor logic
2. After Task 1.5: Review Go interop strategy
3. After Stage 2: Review Option implementation
4. After Stage 3: Full code review

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

### Alternative 3: Manual Type Annotations
**Rejected because:**
- Type inference is essential for ergonomics
- Users expect Rust/Swift-like inference
- Defeats purpose of reducing boilerplate

---

## Post-Phase 3 Roadmap

### Phase 4: Advanced Features (Next)
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
pkg/plugin/builtin/
├── builtin.go              (MODIFY: register new plugins)
├── result_type.go          (NEW: ~400 lines)
├── result_type_test.go     (NEW: ~200 lines)
├── option_type.go          (NEW: ~350 lines)
├── option_type_test.go     (NEW: ~180 lines)
├── sum_types.go            (REFERENCE: pattern to follow)
└── type_inference.go       (USE: for type detection)

tests/golden/
├── result_01_basic.dingo           (EXISTS: needs to pass)
├── result_02_propagation.dingo     (EXISTS: needs to pass)
├── result_03_pattern_match.dingo   (EXISTS: needs to pass)
├── result_04_chaining.dingo        (EXISTS: needs to pass)
├── result_05_go_interop.dingo      (EXISTS: needs to pass)
├── option_01_basic.dingo           (EXISTS: needs to pass)
├── option_02_pattern_match.dingo   (EXISTS: needs to pass)
├── option_03_chaining.dingo        (EXISTS: needs to pass)
└── option_04_go_interop.dingo      (EXISTS: needs to pass)
```

**Total New Code Estimate:**
- Production: ~1,500 lines (2 plugins + integration)
- Tests: ~600 lines (unit + integration)
- Total: ~2,100 lines

---

## Appendix B: Similar Projects Reference

### Rust
- Result/Option are core language types
- Pattern matching is idiomatic
- `?` operator is widely used

### Swift
- Optional type (similar to Option)
- Optional chaining (similar to `?.`)
- Guard statements for unwrapping

### Kotlin
- Nullable types (T?)
- Elvis operator (`?:`) similar to `??`
- Safe call operator (`?.`)

**Lesson:** All modern languages converge on these patterns. Dingo bringing them to Go is valuable.

---

**End of Plan**
