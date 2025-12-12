# Recommendations for Next Steps

## TL;DR

**Recommendation: Fix all 4 identified issues before starting Phase 3**

**Timeline:** 1-2 days (~11 hours total)
**Impact:** High - Fixes bugs that generate invalid Go code and break IDE navigation
**Risk:** Low - All fixes are localized, well-understood changes

---

## The 4 Issues to Fix

### 1. CRITICAL: Source-Map Offset Bug (C1)
**Time:** 2 hours
**File:** `pkg/preprocessor/preprocessor.go:109`

**Fix:**
```go
// BEFORE (wrong - shifts ALL mappings):
adjustMappingsForImports(sourceMap, importLinesAdded, importInsertLine)

// AFTER (correct - only shifts mappings after import block):
func adjustMappingsForImports(sm *SourceMap, offset, insertLine int) {
    for i := range sm.Mappings {
        if sm.Mappings[i].GeneratedLine >= insertLine {
            sm.Mappings[i].GeneratedLine += offset
        }
    }
}
```

**Why Critical:** Breaks IDE navigation to package-level code when imports are injected.

---

### 2. CRITICAL: Multi-Value Return Bug (C2)
**Time:** 4 hours
**File:** `pkg/preprocessor/error_prop.go:477-487`

**Problem:**
```dingo
// Input:
func foo() (int, string, error) {
    return bar()?  // bar: (int, string, error)
}

// Current (WRONG):
func foo() (int, string, error) {
    __tmp0, __err0 := bar()
    if __err0 != nil { return 0, "", __err0 }
    return __tmp0, nil  // ← Drops string value!
}

// Correct:
func foo() (int, string, error) {
    __tmp0, __tmp1, __err0 := bar()
    if __err0 != nil { return 0, "", __err0 }
    return __tmp0, __tmp1, nil
}
```

**Fix Steps:**
1. Parse function return type to count non-error values
2. Generate correct number of temporaries
3. Return all temporaries in success path

**Why Critical:** Generates invalid Go code that won't compile or has wrong behavior.

---

### 3. IMPORTANT: Import Detection False Positives (I1)
**Time:** 2 hours
**File:** `pkg/preprocessor/error_prop.go:29-113`

**Problem:**
```go
// User defines:
func ReadFile(path string) error { ... }

// Uses it:
err := ReadFile(path)?

// Import tracker WRONGLY injects:
import "os"  // ← Unused import! Compile error!
```

**Fix:**
Remove bare function names from `stdLibFunctions` map, keep only qualified calls:
```go
var stdLibFunctions = map[string]string{
    // Remove these:
    // "ReadFile": "os",
    // "Marshal": "encoding/json",

    // Keep only these:
    "os.ReadFile": "os",
    "json.Marshal": "encoding/json",
    // ... etc
}
```

**Why Important:** Causes confusing "unused import" compile errors.

---

### 4. IMPORTANT: Add Negative Tests (I2)
**Time:** 3 hours
**File:** `pkg/preprocessor/preprocessor_test.go`

**Add 3 Tests:**
```go
TestUserDefinedFunctionsDontTriggerImports()
// Verifies user's ReadFile() doesn't inject "os" import

TestMappingsBeforeImportsNotShifted()
// Verifies package-level mappings stay at correct line

TestMultiValueReturnWithErrorProp()
// Verifies (int, string, error) returns work correctly
```

**Why Important:** Prevents regressions of bugs C1, C2, I1.

---

## Three Options

### Option A: Fix All Issues First ⭐ **RECOMMENDED**
**Timeline:** 1-2 days
**Pros:**
- Solid foundation for Phase 3
- High confidence in core functionality
- Better developer experience (working source maps)
- Prevents building on buggy code

**Cons:**
- Delays Phase 3 by 2 days

**Verdict:** ✅ **Best choice** - Quality over speed at this stage

---

### Option B: Start Phase 3 Immediately
**Timeline:** Phase 3 starts now
**Pros:**
- Maintain feature velocity
- Get to Result/Option integration faster

**Cons:**
- Building on buggy foundation
- Source maps unreliable during development
- May need to refactor error propagation later
- Multi-value returns generate broken code

**Verdict:** ❌ **Not recommended** - Technical debt compounds

---

### Option C: Fix Only CRITICAL, Defer IMPORTANT
**Timeline:** 1 day for C1+C2, defer I1+I2
**Pros:**
- Fix show-stoppers (invalid code generation)
- Faster than Option A

**Cons:**
- Import detection still buggy
- No regression tests
- Will need to circle back

**Verdict:** ⚠️ **Acceptable compromise** if time-constrained

---

## After Fixes: Phase 3 Roadmap

### Phase 3.1: Result/Option Integration (2 weeks)
- Constructor integration with type inference
- Pattern matching support
- Go interop (auto-wrapping)
- Comprehensive golden tests

### Phase 3.2: Lambda Syntax (2 weeks)
- 4 syntax styles (Rust/TS/Kotlin/Swift)
- Integration with functional utilities
- Type inference for closures

### Phase 3.3: Safe Navigation (1 week)
- `?.` operator
- `??` operator
- Option type integration

**Total Phase 3 Timeline:** ~5-6 weeks

---

## Decision Points

### Question 1: Multi-Value Return Semantics
**Should `return expr?` support multi-value returns?**

**Option A:** Yes, full support (recommended)
- Matches Go's semantics
- More powerful
- Requires parsing return types

**Option B:** No, constraint to single value + error
- Simpler implementation
- Less useful
- Breaks common patterns

**Recommendation:** Option A - Go developers expect multi-value support.

---

### Question 2: Import Detection Policy
**How aggressive should import detection be?**

**Option A:** Require package qualification (recommended)
- No false positives
- Clearer code (`os.ReadFile` vs `ReadFile`)
- Simpler implementation

**Option B:** Attempt AST resolution
- Allows bare function names
- Complex implementation
- Still has edge cases

**Recommendation:** Option A - Simple and bulletproof.

---

## Action Plan (Option A)

### Day 1 (6 hours)
1. Fix C1: Source-map offset logic (2h)
2. Fix C2: Multi-value return handling (4h)
3. Run existing test suite

### Day 2 (5 hours)
4. Fix I1: Import detection (2h)
5. Add I2: Negative tests (3h)
6. Full test suite + validation
7. Update CHANGELOG.md

### Day 3 (Optional - External Validation)
8. Run external code reviews (Grok, Gemini)
9. Address any new findings
10. Tag as "Phase 2.12 - Polish Complete"

---

## Success Metrics

✅ All 4 issues resolved
✅ All existing tests pass
✅ 3 new negative tests added and passing
✅ No regressions introduced
✅ CHANGELOG.md updated
✅ Ready to start Phase 3 with confidence

---

## Risk Assessment

**Low Risk:**
- All fixes are localized changes
- No architectural changes
- Existing tests provide safety net
- Can validate with golden tests

**Mitigation:**
- Run full test suite after each fix
- Test with real Dingo code examples
- External code review after fixes (optional)

---

## Conclusion

**Fix the bugs first. 2 days of quality improvement will save weeks of debugging later.**

Phase 3 is ambitious (Result/Option integration). Starting it on a buggy foundation will compound problems. The fixes are quick, well-understood, and high-impact.

**Recommended Timeline:**
- **Nov 17-18:** Fix all 4 issues (this week)
- **Nov 19:** Start Phase 3 with confidence
- **Early Dec:** Phase 3.1 complete (Result/Option)
- **Late Dec:** Phase 3 complete (Lambdas + Safe Nav)
- **Jan 2026:** Phase 4 (Language Server)
- **Mid-2026:** v1.0 🎉
