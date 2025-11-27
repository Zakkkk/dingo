# Dingo Feature Validation & Fix Plan

**Created**: 2025-11-27
**Based on**: Comprehensive Feature Audit
**Goal**: Bring all features to production-ready status

---

## Current State Summary

| Status | Count | Percentage |
|--------|-------|------------|
| ✅ Working | 4 | 27% |
| 🟡 Partial | 4 | 27% |
| 🔴 Broken | 7 | 47% |

---

## Phase 1: Critical Bug Fixes (1-2 days)

These are quick wins that unblock multiple features.

### 1.1 Fix Ternary + KeywordProcessor Interaction
**Priority**: P0
**Estimated Time**: 2-4 hours
**Impact**: Unblocks ternary operator (100% of usage broken)

**Problem**: KeywordProcessor regex incorrectly parses ternary IIFE output.
- `let status = func() string {...}` becomes `var statu s = ...`
- Regex captures wrong boundaries

**Files to Modify**:
- `pkg/preprocessor/keywords.go` - Fix regex pattern

**Validation Steps**:
```bash
# Create test file
echo 'package main
func main() {
    let x = true ? "yes" : "no"
    println(x)
}' > /tmp/test_ternary.dingo

# Transpile and verify
dingo build /tmp/test_ternary.dingo
cat /tmp/test_ternary.go  # Should show: var x = func() string {...}()

# Compile generated Go
go build /tmp/test_ternary.go
```

**Success Criteria**:
- [ ] `let x = cond ? a : b` produces valid Go
- [ ] All 3 ternary golden tests pass
- [ ] 42/42 unit tests still pass

---

### 1.2 Fix Sum Types Pattern Matching Code Generation
**Priority**: P0
**Estimated Time**: 4-6 hours
**Impact**: Unblocks sum types, enums, Result/Option pattern matching

**Problem**: Pattern matching generates invalid Go code.
- Uses undefined tag names (`PendingTag` instead of `StatusTagPending`)
- Missing return statements in match arms

**Files to Modify**:
- `pkg/preprocessor/rust_match.go` - Fix tag name generation
- `pkg/preprocessor/enum.go` - Verify tag constant names

**Validation Steps**:
```bash
# Test basic enum pattern matching
echo 'package main

enum Status { Pending, Active, Complete }

func main() {
    let s = Status_Pending()
    match s {
        Pending => println("waiting"),
        Active => println("running"),
        Complete => println("done"),
    }
}' > /tmp/test_enum_match.dingo

dingo build /tmp/test_enum_match.dingo
go build /tmp/test_enum_match.go
./test_enum_match  # Should print: waiting
```

**Success Criteria**:
- [ ] Generated code uses correct tag names (`StatusTagPending`)
- [ ] Match arms have proper return statements
- [ ] 6/6 sum_types golden tests pass
- [ ] 14/14 pattern_match golden tests pass

---

### 1.3 Fix Workspace Build CLI Integration
**Priority**: P1
**Estimated Time**: 2-3 hours
**Impact**: Enables `dingo build ./...`

**Problem**: Build command treats `./...` as file path instead of pattern.

**Files to Modify**:
- `cmd/dingo/build.go` - Add pattern detection before file handling

**Validation Steps**:
```bash
cd /Users/jack/mag/dingo
dingo build ./...  # Should build all packages
dingo build ./pkg/...  # Should build pkg subtree
```

**Success Criteria**:
- [ ] `dingo build ./...` scans and builds all packages
- [ ] `dingo build ./pkg/...` works for subtrees
- [ ] Single file builds still work

---

## Phase 2: Plugin Integration Fixes (2-3 days)

These features have working preprocessors but broken AST plugin integration.

### 2.1 Fix Safe Navigation (`?.`) Plugin
**Priority**: P0
**Estimated Time**: 6-8 hours
**Impact**: Enables null safety feature (currently 0% functional)

**Problem**:
- Preprocessor generates `__SAFE_NAV_INFER__(expr, "field")` placeholders
- SafeNavTypePlugin exists but doesn't transform placeholders
- End result: `?.` is stripped, not transformed

**Files to Investigate**:
- `pkg/preprocessor/safe_nav.go` - Preprocessor (working)
- `pkg/plugin/builtin/safe_nav_types.go` - Plugin (not executing)
- `pkg/plugin/builtin/placeholder_resolver.go` - Resolution logic

**Validation Steps**:
```bash
echo 'package main

type User struct {
    name *string
}

func main() {
    var user *User = nil
    let name = user?.name ?? "Guest"
    println(name)
}' > /tmp/test_safe_nav.dingo

dingo build /tmp/test_safe_nav.dingo
cat /tmp/test_safe_nav.go  # Should show IIFE nil check pattern
go build /tmp/test_safe_nav.go
./test_safe_nav  # Should print: Guest
```

**Success Criteria**:
- [ ] `user?.name` generates nil-check IIFE
- [ ] Works with pointers (*T)
- [ ] Works with Option<T>
- [ ] 11/11 safe_nav golden tests pass
- [ ] Add unit tests for SafeNavTypePlugin

---

### 2.2 Fix Tuples Plugin Integration
**Priority**: P1
**Estimated Time**: 4-6 hours
**Impact**: Enables tuple literals and destructuring

**Problem**:
- Preprocessor generates `__TUPLE_2__LITERAL__(a, b)` markers
- TuplesPlugin exists but doesn't resolve markers
- Parse error: `expected ')', found ','`

**Files to Investigate**:
- `pkg/preprocessor/tuples.go` - Preprocessor (working, 21/21 tests)
- `pkg/plugin/builtin/tuples.go` - Plugin (not resolving markers)

**Validation Steps**:
```bash
echo 'package main

func main() {
    let pair = (1, "hello")
    let (x, y) = pair
    println(x, y)
}' > /tmp/test_tuples.dingo

dingo build /tmp/test_tuples.dingo
go build /tmp/test_tuples.go
./test_tuples  # Should print: 1 hello
```

**Success Criteria**:
- [ ] Tuple literals transpile to struct literals
- [ ] Destructuring works
- [ ] 8/8 tuples golden tests pass

---

### 2.3 Fix Null Coalescing (`??`) Pipeline
**Priority**: P1
**Estimated Time**: 4-6 hours
**Impact**: Enables default value operator

**Problem**:
- Full implementation exists (994 LOC, 15/15 tests)
- Was working (golden files dated 2025-11-22)
- Recent regression: import injection breaks parsing

**Files to Investigate**:
- `pkg/preprocessor/null_coalesce.go` - Implementation
- `pkg/generator/generator.go` - Import injection logic

**Validation Steps**:
```bash
echo 'package main

func getName() *string { return nil }

func main() {
    let name = getName() ?? "default"
    println(name)
}' > /tmp/test_coalesce.dingo

dingo build /tmp/test_coalesce.dingo
go build /tmp/test_coalesce.go
./test_coalesce  # Should print: default
```

**Success Criteria**:
- [ ] `a ?? b` generates proper unwrap logic
- [ ] Import injection doesn't break parsing
- [ ] 8/8 null_coalesce golden tests pass

---

## Phase 3: Missing Implementations (3-5 days)

### 3.1 Implement Functional Utilities
**Priority**: P2
**Estimated Time**: 2-3 days
**Impact**: Enables `.map()`, `.filter()`, `.reduce()` method syntax

**Problem**: Feature not implemented despite spec claiming 100% completion.

**Implementation Plan**:
1. Create `pkg/preprocessor/functional.go`
2. Transform `slice.map(fn)` → loop with IIFE
3. Support: map, filter, reduce, sum, count, all, any
4. Add to preprocessor pipeline

**Files to Create**:
- `pkg/preprocessor/functional.go`
- `pkg/preprocessor/functional_test.go`

**Validation Steps**:
```bash
echo 'package main

func main() {
    let nums = []int{1, 2, 3, 4, 5}
    let doubled = nums.map(x => x * 2)
    let evens = nums.filter(x => x % 2 == 0)
    let sum = nums.reduce(0, (acc, x) => acc + x)
    println(doubled, evens, sum)
}' > /tmp/test_functional.dingo

dingo build /tmp/test_functional.dingo
go run /tmp/test_functional.go
# Should print: [2 4 6 8 10] [2 4] 15
```

**Success Criteria**:
- [ ] `.map()` transforms slices
- [ ] `.filter()` filters by predicate
- [ ] `.reduce()` aggregates values
- [ ] Method chaining works
- [ ] 4/4 func_util golden tests pass

---

### 3.2 Complete Option Type
**Priority**: P1
**Estimated Time**: 1 day
**Impact**: Fixes type declaration injection, adds missing methods

**Tasks**:
1. Fix type declaration injection bug in `option_05_helpers`
2. Add `okOr()` method for Result conversion
3. Add unit tests for OptionTypePlugin (currently 0)

**Files to Modify**:
- `pkg/plugin/builtin/option_type.go`

**Success Criteria**:
- [ ] All 7 option golden tests pass
- [ ] `okOr()` method implemented
- [ ] Unit tests added for plugin

---

### 3.3 Complete Result Type Go Interop
**Priority**: P2
**Estimated Time**: 1 day
**Impact**: Enables `Result.fromGo()` and `result.toGo()`

**Tasks**:
1. Implement `fromGo((value, error))` conversion
2. Implement `toGo() (T, error)` conversion
3. Auto-wrap stdlib calls returning `(T, error)`

**Files to Modify**:
- `pkg/plugin/builtin/result_type.go`

**Success Criteria**:
- [ ] Can convert Go tuples to Result
- [ ] Can convert Result to Go tuples
- [ ] `result_05_go_interop` golden test passes

---

## Phase 4: Pattern Matching Enhancements (2-3 days)

### 4.1 Add Struct Destructuring
**Priority**: P2
**Estimated Time**: 1-2 days
**Impact**: Enables `User{name, age}` patterns

**Files to Modify**:
- `pkg/preprocessor/rust_match.go`

**Success Criteria**:
- [ ] `User{name: n, age: a}` extracts fields
- [ ] `User{name, ..}` with rest pattern works

---

### 4.2 Add Compile-Time Exhaustiveness
**Priority**: P2
**Estimated Time**: 1 day
**Impact**: Catches missing match arms at transpile time

**Current**: Runtime panic for missing cases
**Target**: Compile-time error

**Files to Modify**:
- `pkg/preprocessor/rust_match.go` - Add static analysis

---

## Phase 5: Documentation Updates (1 day)

### 5.1 Update README.md
- [ ] Remove false "Working" claims for broken features
- [ ] Add honest completion percentages
- [ ] Add "Known Issues" section

### 5.2 Update CLAUDE.md
- [ ] Update Current Stage section
- [ ] Fix test passing percentages
- [ ] Document actual feature status

### 5.3 Update features/INDEX.md
- [ ] Update status for all features
- [ ] Add accurate completion percentages
- [ ] Mark broken features appropriately

### 5.4 Update Individual Feature Specs
- [ ] `features/functional-utilities.md` - Remove false "Implemented" claim
- [ ] `features/null-safety.md` - Update status
- [ ] `features/ternary.md` - Document integration bug
- [ ] Remove duplicate `ternary-operator.md` vs `ternary.md`

---

## Validation Checklist

After all fixes, run full validation:

```bash
# 1. Run all unit tests
go test ./... -v

# 2. Run golden tests (should have 0 skipped)
go test ./tests/golden/... -v | grep -E "(PASS|FAIL|SKIP)"

# 3. Test each feature manually
for f in tests/golden/*.dingo; do
    echo "Testing $f..."
    dingo build "$f" && go build "${f%.dingo}.go" || echo "FAILED: $f"
done

# 4. Verify showcase compiles
dingo build tests/golden/showcase_01_api_server.dingo
go build tests/golden/showcase_01_api_server.go
```

---

## Progress Tracking

### Phase 1: Critical Bug Fixes
- [ ] 1.1 Ternary + KeywordProcessor
- [ ] 1.2 Sum Types Pattern Matching
- [ ] 1.3 Workspace Build CLI

### Phase 2: Plugin Integration
- [ ] 2.1 Safe Navigation Plugin
- [ ] 2.2 Tuples Plugin
- [ ] 2.3 Null Coalescing Pipeline

### Phase 3: Missing Implementations
- [ ] 3.1 Functional Utilities
- [ ] 3.2 Option Type Completion
- [ ] 3.3 Result Type Go Interop

### Phase 4: Pattern Matching
- [ ] 4.1 Struct Destructuring
- [ ] 4.2 Compile-Time Exhaustiveness

### Phase 5: Documentation
- [ ] 5.1 README.md
- [ ] 5.2 CLAUDE.md
- [ ] 5.3 INDEX.md
- [ ] 5.4 Feature Specs

---

## Estimated Total Effort

| Phase | Time Estimate | Priority |
|-------|---------------|----------|
| Phase 1 | 1-2 days | Critical |
| Phase 2 | 2-3 days | High |
| Phase 3 | 3-5 days | Medium |
| Phase 4 | 2-3 days | Medium |
| Phase 5 | 1 day | High |
| **Total** | **9-14 days** | - |

---

## Success Metrics

**Before** (Current State):
- 4/15 features working (27%)
- 7/15 features broken (47%)
- ~50% golden tests skipped

**After** (Target State):
- 15/15 features working (100%)
- 0/15 features broken (0%)
- 0% golden tests skipped
- All generated Go code compiles
