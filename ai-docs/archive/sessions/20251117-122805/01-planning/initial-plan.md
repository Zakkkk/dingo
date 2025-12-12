# Dingo Priority Tasks: Architectural Plan

**Session:** 20251117-122805
**Date:** 2025-11-17
**Architect:** Claude Sonnet 4.5

---

## Executive Summary

**Current State:** Phase 2.7 with 8,306 lines of production code, 7 fully working features, robust plugin architecture, but 89/97 tests passing (91.8%). The critical blocker is the **Type Inference System**, which gates completion of Result/Option types, lambda inference, and safe navigation fixes.

**Recommended Approach:** Sequential 4-phase execution prioritizing test fixes, then type inference system integration, followed by Result/Option completion and parser enhancements.

**Timeline Estimate:** 18-24 hours total development effort across 4 phases.

---

## Problem Analysis

### Root Cause Assessment

1. **Type Inference Fragmentation**: Current TypeInference exists but is not integrated into the plugin pipeline. Plugins operate without shared type context, leading to:
   - Safe navigation cannot determine result types for chaining
   - Lambda parameters cannot infer types from context
   - Result/Option plugins cannot detect type wrapping scenarios
   - Error propagation zero values may be incorrect in edge cases

2. **Parser Limitations**: Advanced syntax patterns (destructuring, complex ternaries) are not yet parsed, blocking golden file tests even when transformation logic is correct.

3. **Test Infrastructure Gaps**:
   - Field name mismatches (trivial fix)
   - Missing utility functions (contains())
   - Golden tests blocked by parser, not logic

### Dependencies Identified

```
Type Inference System (Central Hub)
    ├─> Result Type Plugin (blocked)
    ├─> Option Type Plugin (blocked)
    ├─> Safe Navigation Plugin (chaining bug)
    ├─> Lambda Plugin (parameter inference)
    ├─> Error Propagation Plugin (optimization)
    └─> Null Coalescing Plugin (Option detection)

Parser Fixes
    ├─> Golden File Tests (4/20 passing)
    └─> Advanced Pattern Matching
```

---

## Recommended Architecture

### Phase 1: Test Stabilization (2-3 hours)

**Goal:** Achieve 100% test pass rate, establish stable baseline.

#### 1.1 Fix Error Propagation Tests
**Files:** `pkg/plugin/builtin/error_propagation_test.go`

**Issue:** Field name mismatches between generated code and test expectations.

**Solution:**
- Review test cases for expected field names (lowercase vs camelCase)
- Ensure consistency with sum_types plugin conventions (lowercase: ok_0, err_0, some_0)
- Update test assertions to match actual generated output
- Validate against current sum_types implementation

**Risk:** Low - Isolated to test code, no logic changes needed.

#### 1.2 Fix Lambda Tests
**Files:** `pkg/plugin/builtin/lambda_test.go`, possibly add utility

**Issue:** Tests reference missing `contains()` utility function.

**Solution Option A (Preferred):**
- Add `contains()` to functional_utils plugin
- Implements early-exit loop checking slice membership
- Pattern: `items.contains(func(x T) bool { return x == target })`
- Zero overhead IIFE generation like other utilities

**Solution Option B:**
- Update tests to use standard Go range loops
- Less elegant but no feature addition

**Risk:** Low - New utility follows established pattern, or simple test rewrite.

#### 1.3 Fix Generator Marker Tests
**Files:** `pkg/generator/markers_test.go`

**Issue:** 2/4 tests failing - marker injection not working as expected.

**Investigation Required:**
- Check if marker generation is properly integrated
- Verify marker format matches test expectations
- Ensure config flag correctly enables/disables markers

**Solution:** Debug and fix marker injection logic based on findings.

**Risk:** Low - Test-only issue, marker feature is non-critical for transpilation.

#### 1.4 Fix Parser Feature Tests
**Files:** `pkg/parser/new_features_test.go`

**Issue:** Ternary tests failing on parse errors (unexpected ":").

**Quick Fix:**
- Review ternary operator grammar in `pkg/parser/participle.go`
- Ensure ternary syntax is correctly defined in parser
- May be precedence issue with "?" vs "?:" operator
- Add missing grammar rules if needed

**Risk:** Low-Medium - May require parser grammar adjustments but isolated to ternary.

**Deliverables:**
- 97/97 tests passing (or known deferred count documented)
- Clean CI/CD pipeline
- Baseline for future changes

---

### Phase 2: Type Inference System Integration (6-8 hours)

**Goal:** Create centralized type inference service accessible to all plugins, enabling type-aware transformations.

#### 2.1 Architecture: Shared Type Context

**Current State:**
- `TypeInference` exists in `pkg/plugin/builtin/type_inference.go` (261 lines)
- Used only by ErrorPropagationPlugin
- Created per-transformation, not shared
- No inter-plugin communication

**Target State:**
```
Plugin Pipeline Context
    └─> TypeInferenceService (Shared)
        ├─> go/types type checker
        ├─> Type information cache
        ├─> Zero value generator
        └─> Type-to-AST converter

Each Plugin Access:
    - ctx.TypeInference.InferType(expr)
    - ctx.TypeInference.IsOptionType(typ)
    - ctx.TypeInference.IsResultType(typ)
    - ctx.TypeInference.GenerateZeroValue(typ)
```

**Design Pattern:** Service object in plugin.Context, lifecycle managed by pipeline.

#### 2.2 Implementation Plan

**Step 1: Enhance plugin.Context** (1 hour)
- Add `TypeInference *TypeInferenceService` field to `plugin.Context`
- Create `NewTypeInferenceService(fset, file)` constructor
- Add lifecycle methods: `Initialize()`, `Close()`
- Update `Pipeline.Execute()` to create and inject service

**File:** `pkg/plugin/context.go` (new) or extend `pkg/plugin/plugin.go`

**Step 2: Refactor TypeInference as Service** (2 hours)
- Rename to `TypeInferenceService` for clarity
- Add Result/Option type detection methods:
  ```go
  func (s *TypeInferenceService) IsResultType(typ types.Type) (T, E types.Type, bool)
  func (s *TypeInferenceService) IsOptionType(typ types.Type) (T types.Type, bool)
  func (s *TypeInferenceService) IsGoErrorTuple(sig *types.Signature) bool
  ```
- Add named type detection (checks for `Result_*`, `Option_*` patterns)
- Cache type lookups to avoid repeated `go/types` queries
- Add synthetic type registration (for compiler-generated enums)

**File:** `pkg/plugin/builtin/type_inference.go` (refactor existing)

**Step 3: Update ErrorPropagationPlugin** (1 hour)
- Remove internal TypeInference creation
- Use `ctx.TypeInference` instead
- Simplify initialization logic
- Update tests to inject mock TypeInferenceService

**File:** `pkg/plugin/builtin/error_propagation.go`

**Step 4: Update SumTypesPlugin** (1 hour)
- Register synthetic enums with TypeInferenceService
- When generating `Result[T, E]` or `Option[T]`, call:
  ```go
  ctx.TypeInference.RegisterSyntheticType("Result_T_E", enumDecl)
  ```
- Enables other plugins to detect these types

**File:** `pkg/plugin/builtin/sum_types.go`

**Step 5: Integration Testing** (1-2 hours)
- Create comprehensive tests for TypeInferenceService
- Test cross-plugin type detection scenarios
- Verify Result/Option detection works
- Test zero value generation for all Go types
- Performance testing (type inference shouldn't slow builds)

**Files:**
- `pkg/plugin/builtin/type_inference_test.go` (expand existing)
- `tests/integration_test.go` (add type-aware scenarios)

#### 2.3 API Design

```go
// TypeInferenceService provides centralized type information for all plugins
type TypeInferenceService struct {
    fset   *token.FileSet
    info   *types.Info
    pkg    *types.Package
    config *types.Config

    // Cache for performance
    typeCache map[ast.Expr]types.Type

    // Synthetic types registered by plugins (Result, Option, etc)
    syntheticTypes map[string]*SyntheticTypeInfo
}

type SyntheticTypeInfo struct {
    TypeName   string
    Underlying *types.Named
    GenDecl    *ast.GenDecl // The generated AST
}

// Core inference methods
func (s *TypeInferenceService) InferType(expr ast.Expr) (types.Type, error)
func (s *TypeInferenceService) InferFunctionReturnType(fn *ast.FuncDecl) (types.Type, error)
func (s *TypeInferenceService) GenerateZeroValue(typ types.Type) ast.Expr

// Type detection helpers
func (s *TypeInferenceService) IsResultType(typ types.Type) (T, E types.Type, ok bool)
func (s *TypeInferenceService) IsOptionType(typ types.Type) (T types.Type, ok bool)
func (s *TypeInferenceService) IsPointerType(typ types.Type) bool
func (s *TypeInferenceService) IsErrorType(typ types.Type) bool

// Synthetic type registry
func (s *TypeInferenceService) RegisterSyntheticType(name string, info *SyntheticTypeInfo)
func (s *TypeInferenceService) GetSyntheticType(name string) (*SyntheticTypeInfo, bool)

// Go interop detection
func (s *TypeInferenceService) IsGoErrorTuple(sig *types.Signature) (valueType types.Type, ok bool)
func (s *TypeInferenceService) ShouldWrapAsResult(callExpr *ast.CallExpr) bool
```

#### 2.4 Integration Points

**Plugin Pipeline Modification:**

```go
// pkg/plugin/pipeline.go

func (p *Pipeline) Execute(file *ast.File) error {
    // Create shared type inference service
    typeService, err := NewTypeInferenceService(p.fset, file)
    if err != nil {
        return err
    }
    defer typeService.Close()

    // Inject into context
    p.Ctx.TypeInference = typeService

    // Execute plugin transformations
    for _, plugin := range p.sortedPlugins {
        // Plugins now have access to ctx.TypeInference
        if err := plugin.Transform(file, p.Ctx); err != nil {
            return err
        }

        // Re-run type checking after transformations
        if err := typeService.Refresh(file); err != nil {
            p.Ctx.Logger.Warn("Type refresh failed: %v", err)
        }
    }

    return nil
}
```

**Deliverables:**
- Shared TypeInferenceService integrated into plugin pipeline
- All plugins have access to centralized type information
- Result/Option type detection working
- Performance benchmarks showing <5% build time increase
- 100% backward compatibility (no existing tests broken)

---

### Phase 3: Result/Option Type Completion (6-8 hours)

**Goal:** Transform Result/Option from foundation-only to fully functional types with automatic Go interop.

#### 3.1 Current State Analysis

**What Exists:**
- `pkg/plugin/builtin/result_type.go` - Foundation plugin (placeholder Transform())
- `pkg/plugin/builtin/option_type.go` - Foundation plugin (placeholder Transform())
- Enum structure defined (Ok/Err variants for Result, Some/None for Option)
- Helper methods designed (IsOk, Unwrap, UnwrapOr, etc.)

**What's Missing:**
- No actual type detection in user code
- No automatic wrapping of Go `(T, error)` returns
- No Result/Option literal syntax transformation
- No integration with error propagation operator
- Type inference integration (Phase 2 dependency)

#### 3.2 Result Type Implementation

**Feature 1: Result Literal Transformation** (2 hours)

Syntax:
```dingo
result := Ok(42)              // Result[int, error]
failure := Err(errors.New())  // Result[T, error]
```

Transform to:
```go
result := Result_int_error{tag: ResultTag_Ok, ok_0: 42}
failure := Result_T_error{tag: ResultTag_Err, err_0: errors.New()}
```

**Implementation:**
- Detect `Ok()` and `Err()` call expressions in AST
- Use TypeInference to determine T and E types
- Generate appropriate Result_T_E struct literal
- Register Result_T_E type with SumTypesPlugin if not exists

**Feature 2: Go Interop - Auto-wrapping** (2-3 hours)

Detect Go functions returning `(T, error)` and auto-wrap in Result:

```dingo
data := readFile("config.json")  // readFile returns ([]byte, error)
// Automatically wraps to Result[[]byte, error]
```

Transform to:
```go
data := func() Result_bytes_error {
    __tmp0, __tmp1 := readFile("config.json")
    if __tmp1 != nil {
        return Result_bytes_error{tag: ResultTag_Err, err_0: __tmp1}
    }
    return Result_bytes_error{tag: ResultTag_Ok, ok_0: __tmp0}
}()
```

**Implementation:**
- In assignment statements, check RHS call expression
- Use TypeInference.IsGoErrorTuple() to detect `(T, error)` signature
- Generate IIFE wrapper (same pattern as functional utilities)
- Only wrap when LHS expects Result type (avoid breaking existing code)

**Feature 3: Error Propagation Integration** (1 hour)

Enable `?` operator to work with Result types:

```dingo
func processData() Result[Data, error] {
    config := readConfig()?  // readConfig returns Result[Config, error]
    validated := validate(config)?
    return Ok(validated)
}
```

**Implementation:**
- ErrorPropagationPlugin checks if expression is Result type
- If Result, generate pattern match instead of error check:
  ```go
  config := func() Config {
      __result := readConfig()
      if __result.tag == ResultTag_Err {
          return Result_Data_error{tag: ResultTag_Err, err_0: __result.err_0}
      }
      return __result.ok_0
  }()
  ```
- Coordinate with SumTypesPlugin for tag constants

**File Changes:**
- `pkg/plugin/builtin/result_type.go` - Implement Transform() logic
- `pkg/plugin/builtin/error_propagation.go` - Add Result-aware path
- Tests: `pkg/plugin/builtin/result_type_test.go` (comprehensive)

#### 3.3 Option Type Implementation

**Feature 1: Option Literal Transformation** (1-2 hours)

Syntax:
```dingo
value := Some(42)        // Option[int]
empty := None            // Option[T]
```

Transform to:
```go
value := Option_int{tag: OptionTag_Some, some_0: 42}
empty := Option_T{tag: OptionTag_None}
```

**Feature 2: Null Coalescing Integration** (1 hour)

Update NullCoalescingPlugin to detect Option types:

```dingo
value := maybeValue ?? 0  // maybeValue is Option[int]
```

Transform to:
```go
value := func() int {
    if maybeValue.tag == OptionTag_Some {
        return maybeValue.some_0
    }
    return 0
}()
```

**Implementation:**
- Use TypeInference.IsOptionType() in NullCoalescingPlugin
- Generate pattern match for Option types
- Existing pointer logic remains for backward compatibility

**Feature 3: Safe Navigation Integration** (1 hour)

Fix chaining bug by detecting Option returns:

```dingo
name := user?.address?.city  // Each step returns Option
```

**Implementation:**
- SafeNavigationPlugin uses TypeInference to check intermediate types
- If previous navigation returned Option, unwrap before next access
- Chain Option wrapping through entire expression

**File Changes:**
- `pkg/plugin/builtin/option_type.go` - Implement Transform()
- `pkg/plugin/builtin/null_coalescing.go` - Add Option detection (use TypeInference)
- `pkg/plugin/builtin/safe_navigation.go` - Fix chaining with type awareness
- Tests: `pkg/plugin/builtin/option_type_test.go`

#### 3.4 Testing Strategy

**Unit Tests:**
- Result literal transformation (Ok, Err)
- Option literal transformation (Some, None)
- Type inference for generic instantiation
- Auto-wrapping Go error tuples

**Integration Tests:**
- Result with error propagation operator
- Option with null coalescing
- Chained safe navigation returning Options
- Mixed Result/Option in same codebase

**Golden File Tests:**
- Create `tests/golden/result_basic.dingo`
- Create `tests/golden/option_basic.dingo`
- Create `tests/golden/result_error_prop.dingo`
- Validate generated Go code quality

**Deliverables:**
- Result[T, E] fully functional with auto-wrapping
- Option[T] fully functional with null coalescing integration
- Error propagation works with both Result and Go error tuples
- Safe navigation chaining bug fixed
- 95%+ test coverage for new features
- Golden tests passing for Result/Option scenarios

---

### Phase 4: Parser Enhancements (2-3 hours)

**Goal:** Fix advanced syntax parsing to unlock remaining golden file tests.

#### 4.1 Issue Analysis

**Current:** 4/20 golden tests passing
**Root Cause:** Parser doesn't support advanced patterns, not transformation logic

**Known Gaps:**
1. Pattern destructuring syntax (struct fields, tuple unpacking)
2. Complex ternary operator parsing (precedence with "?")
3. Map type syntax
4. Type declaration statements
5. String escape sequences

#### 4.2 Priority Fixes

**Fix 1: Ternary Operator Precedence** (30 min)

**Issue:** Parser fails on `condition ? value : other` with "unexpected ':'"

**Solution:**
- Review grammar in `pkg/parser/participle.go`
- Ensure ternary has lower precedence than other operators
- May need to move ternary to statement level or adjust expression grammar
- Test with nested ternaries: `a ? b ? c : d : e`

**Fix 2: Pattern Destructuring** (1 hour)

**Issue:** Match patterns like `Circle{radius}` not parsed

**Solution:**
- Extend `Pattern` grammar to support struct destructuring
- Add tuple destructuring: `Point(x, y)`
- Parse field bindings vs field comparisons
- Update AST nodes for destructured patterns

**Fix 3: Map Type Syntax** (30 min)

**Issue:** `map[string]int` not recognized

**Solution:**
- Add MapType to Type grammar
- Parse `map[KeyType]ValueType` syntax
- Generate corresponding Go AST MapType node

**Fix 4: Type Declarations** (30 min)

**Issue:** `type User struct { ... }` statements not parsed

**Solution:**
- Add TypeDecl to top-level declarations grammar
- Parse `type Name Type` syntax
- Support both type aliases and type definitions
- Generate GenDecl with TypeSpec

#### 4.3 Implementation Approach

**Strategy:** Incremental grammar additions with immediate testing

1. **Update Grammar** - Add syntax rules to `pkg/parser/participle.go`
2. **Update AST** - Add nodes to `pkg/ast/ast.go` if needed
3. **Test Parse** - Verify parsing with unit tests
4. **Run Golden Tests** - Check which tests now pass
5. **Iterate** - Repeat for next syntax gap

**Deliverables:**
- Ternary operator fully parsed
- Pattern destructuring working
- Map types supported
- Type declarations parsed
- 15+/20 golden tests passing (75%+ pass rate)
- Remaining failures documented with clear TODOs

---

## Implementation Sequence

### Week 1 (Critical Path)

**Day 1-2: Phase 1 - Test Stabilization**
- Priority: Fix broken tests first
- Output: 100% test pass rate, clean baseline
- Risk: Low, quick wins

**Day 3-4: Phase 2 - Type Inference Integration**
- Priority: Critical blocker, unlocks everything else
- Output: Shared type service, cross-plugin type awareness
- Risk: Medium, requires careful architecture

**Day 5: Phase 3 Start - Result Type**
- Priority: Core feature completion
- Output: Result type fully working
- Risk: Low-Medium, depends on Phase 2

### Week 2 (Feature Completion)

**Day 1-2: Phase 3 Complete - Option Type**
- Priority: Core feature completion
- Output: Option type + integration fixes
- Risk: Low-Medium

**Day 3: Phase 4 - Parser Enhancements**
- Priority: Unlock golden tests
- Output: Advanced syntax support
- Risk: Low, isolated to parser

**Day 4-5: Polish & Documentation**
- Integration testing
- Performance benchmarking
- Update CHANGELOG.md
- Update feature docs in `features/`

---

## Risk Assessment

### High Risk Items

1. **Type Inference Performance**
   - Risk: `go/types` type checking on every build could slow transpilation
   - Mitigation: Cache type information, only recheck after transformations
   - Fallback: Make type inference optional, degrade gracefully

2. **Backward Compatibility**
   - Risk: Type inference changes could break existing working code
   - Mitigation: Comprehensive regression testing, feature flags
   - Fallback: TypeInference as opt-in for new plugins only

### Medium Risk Items

1. **Result/Option Auto-wrapping**
   - Risk: Automatic wrapping might trigger in unexpected places
   - Mitigation: Conservative detection (only wrap when LHS explicitly expects Result/Option)
   - Fallback: Require explicit opt-in syntax

2. **Parser Grammar Conflicts**
   - Risk: New syntax might conflict with existing grammar rules
   - Mitigation: Careful precedence management, incremental testing
   - Fallback: Deferred syntax features don't block core functionality

### Low Risk Items

1. Test fixes (Phase 1) - Isolated, well-understood
2. Golden file expansion - Doesn't affect existing functionality
3. Documentation updates - No code impact

---

## Success Metrics

### Phase 1 Complete
- [ ] 97/97 plugin tests passing (or documented deferrals)
- [ ] 0 failing tests in CI/CD
- [ ] Clean git status, ready for next phase

### Phase 2 Complete
- [ ] TypeInferenceService integrated into plugin pipeline
- [ ] All plugins can access shared type information
- [ ] Result/Option type detection working
- [ ] Build time increase <5% on benchmark suite
- [ ] 100% backward compatibility maintained

### Phase 3 Complete
- [ ] Result[T, E] literals working (Ok/Err)
- [ ] Option[T] literals working (Some/None)
- [ ] Go `(T, error)` auto-wrapping functional
- [ ] Error propagation works with Result types
- [ ] Null coalescing works with Option types
- [ ] Safe navigation chaining bug fixed
- [ ] 30+ new tests passing (Result/Option coverage)

### Phase 4 Complete
- [ ] Ternary operator fully parsed
- [ ] Pattern destructuring supported
- [ ] Map types parsed
- [ ] Type declarations parsed
- [ ] 15+/20 golden tests passing
- [ ] Remaining parser TODOs documented

### Overall Success
- [ ] 100% of originally planned features working
- [ ] Performance meets targets (<5% overhead vs baseline)
- [ ] Code quality: 95%+ test coverage, all critical paths tested
- [ ] Documentation: CHANGELOG.md updated, feature docs current
- [ ] Ready for Phase 3 (next major features)

---

## Alternative Approaches Considered

### Alternative 1: Type Inference as Separate Pass

**Approach:** Run full type inference before any plugin transformations, cache all results.

**Pros:**
- Single type-checking pass, potentially faster
- Simpler plugin integration (just lookup cached types)

**Cons:**
- Types become stale after transformations
- Can't infer types of generated code
- Doesn't work for multi-pass plugins (sum_types, error_prop)

**Verdict:** Rejected. Plugins need fresh type information after transformations.

### Alternative 2: Plugin-Local Type Inference

**Approach:** Keep current model, each plugin creates its own TypeInference.

**Pros:**
- No shared state, simpler plugin interface
- No pipeline changes needed

**Cons:**
- Massive performance overhead (type-check file N times)
- No cross-plugin communication
- Can't detect Result/Option types generated by other plugins

**Verdict:** Rejected. Doesn't solve the core problem.

### Alternative 3: Hybrid - Lazy Type Inference

**Approach:** Only run type inference when plugins explicitly request it, cache results.

**Pros:**
- Zero overhead for plugins that don't need types
- Gradual adoption path

**Cons:**
- More complex caching logic
- Unclear when to invalidate cache
- Subtle bugs if cache is stale

**Verdict:** Consider as optimization after Phase 2 proves too slow (unlikely).

---

## Open Questions & Decisions Needed

### Question 1: Result/Option Syntax Registration

**Issue:** How do users declare Result/Option types in their code?

**Option A - Automatic:**
- Parser recognizes `Result[T, E]` and `Option[T]` as keywords
- No import needed, types are built-in
- SumTypesPlugin auto-generates enum definitions

**Option B - Import-based:**
- Users import from `dingo/runtime` package
- Types defined in actual Go library
- No auto-generation needed

**Recommendation:** Option A (automatic). Fits meta-language model better, no runtime dependency.

### Question 2: Go Interop Auto-wrapping Opt-in

**Issue:** Should `(T, error)` auto-wrap in Result be automatic or explicit?

**Option A - Automatic:**
- Any assignment from `(T, error)` to `Result[T, E]` auto-wraps
- Seamless interop, minimal boilerplate

**Option B - Explicit:**
- Require `Result.from(goFunc())` wrapper syntax
- More control, no surprises

**Option C - Configuration:**
- `auto_wrap_go_errors = true/false` in dingo.toml
- Per-project choice

**Recommendation:** Option C (configurable, default true). Balance convenience with control.

### Question 3: Performance Budget

**Issue:** What's acceptable build time increase for type inference?

**Proposed Budget:**
- <5% increase on small projects (< 10 files)
- <10% increase on medium projects (< 100 files)
- <15% increase on large projects (> 100 files)

**Mitigation Plan:**
- Profile type inference separately
- Add caching if budget exceeded
- Consider incremental type checking

---

## Next Steps

1. **User Approval** - Review this plan, answer open questions
2. **Phase 1 Kick-off** - Start with test fixes (quick wins)
3. **Type Inference Design Review** - Deep dive on API before implementation
4. **Incremental Delivery** - Ship each phase independently, gather feedback

---

## Appendix: File Modification Estimates

### Files to Modify (Phase 1)
- `pkg/plugin/builtin/error_propagation_test.go` - Field name fixes
- `pkg/plugin/builtin/lambda_test.go` - Add contains() or rewrite tests
- `pkg/generator/markers_test.go` - Debug marker injection
- `pkg/parser/new_features_test.go` - Fix ternary parsing
- **Estimate:** 4 files, ~50 lines changed

### Files to Modify (Phase 2)
- `pkg/plugin/plugin.go` - Add TypeInference to Context
- `pkg/plugin/pipeline.go` - Lifecycle management
- `pkg/plugin/builtin/type_inference.go` - Refactor to service (major)
- `pkg/plugin/builtin/error_propagation.go` - Use shared service
- `pkg/plugin/builtin/sum_types.go` - Register synthetic types
- `pkg/plugin/builtin/null_coalescing.go` - Use IsOptionType()
- `pkg/plugin/builtin/safe_navigation.go` - Use type detection
- **Estimate:** 7 files, ~400 lines changed/added

### Files to Modify (Phase 3)
- `pkg/plugin/builtin/result_type.go` - Full implementation (major)
- `pkg/plugin/builtin/option_type.go` - Full implementation (major)
- `pkg/plugin/builtin/error_propagation.go` - Result integration
- `pkg/plugin/builtin/null_coalescing.go` - Option integration
- `pkg/plugin/builtin/safe_navigation.go` - Option chaining fix
- **Estimate:** 5 files, ~600 lines added

### Files to Modify (Phase 4)
- `pkg/parser/participle.go` - Grammar extensions (major)
- `pkg/ast/ast.go` - New AST nodes (if needed)
- **Estimate:** 2 files, ~200 lines changed

### Test Files to Add
- `pkg/plugin/builtin/type_inference_test.go` - Expand
- `pkg/plugin/builtin/result_type_test.go` - New
- `pkg/plugin/builtin/option_type_test.go` - New
- `tests/golden/result_basic.dingo` - New
- `tests/golden/option_basic.dingo` - New
- `tests/golden/result_error_prop.dingo` - New
- **Estimate:** 6 files, ~800 lines of tests

**Total Estimated Changes:** 24 files, ~2,050 lines of code (production + tests)

---

## References

- User Request: `/Users/jack/mag/dingo/ai-docs/sessions/20251117-122805/01-planning/user-request.md`
- Current Codebase: 8,306 lines production code, 21 plugin files
- CHANGELOG: `/Users/jack/mag/dingo/CHANGELOG.md`
- Feature Specs: `/Users/jack/mag/dingo/features/*.md`
- Plugin Architecture: `/Users/jack/mag/dingo/PLUGIN_SYSTEM_DESIGN.md` (if exists)
