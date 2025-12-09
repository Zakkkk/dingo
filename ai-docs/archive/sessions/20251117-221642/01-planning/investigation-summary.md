# Current State Investigation Summary

## Code Review Findings (GPT-5.1 Codex)

### CRITICAL Issues Identified (2)

#### C1: Source-Map Offset Bug
**Location:** `pkg/preprocessor/preprocessor.go:93-110, 166-170`

**Problem:**
When imports are injected, the code applies offset adjustments to ALL source mappings, even for lines that appear BEFORE the import insertion point. This causes mappings at the top of the file (package declaration, etc.) to point to incorrect generated lines.

**Impact:**
- IDE navigation breaks for package-level code
- Error messages point to wrong lines
- Debugging experience degraded

**Current Code (Line 109):**
```go
adjustMappingsForImports(sourceMap, importLinesAdded, importInsertLine)
```

**Root Cause:**
The function doesn't check if a mapping's generated line is >= import insertion line before applying the offset.

**Fix Required:**
Only shift mappings whose `GeneratedLine >= importInsertLine`

---

#### C2: Multi-Value Return Handling
**Location:** `pkg/preprocessor/error_prop.go:477-487`

**Problem:**
When handling `return expr?` in statement context, the success path always generates:
```go
return tmp, nil
```

If `expr` returns multiple non-error values (e.g., `(A, B, error)`), the extra values (A, B) are silently dropped, producing invalid Go code.

**Example:**
```dingo
// This Dingo code:
func foo() (int, string, error) {
    return bar()?  // bar returns (int, string, error)
}

// Generates THIS broken Go:
func foo() (int, string, error) {
    __tmp0, __err0 := bar()
    if __err0 != nil {
        return 0, "", __err0  // ← Wrong! Should preserve __tmp0 values
    }
    return __tmp0, nil  // ← Wrong! Drops string value, wrong nil placement
}
```

**Root Cause:**
Code doesn't parse the return type tuple to determine how many non-error values exist.

**Fix Required:**
1. Parse function return type to get tuple length
2. Generate correct number of zero values for error path
3. Return all non-error temporaries + nil for success path

---

### IMPORTANT Issues Identified (2)

#### I1: Import Detection False Positives
**Location:** `pkg/preprocessor/error_prop.go:29-113`

**Problem:**
Import detection keys only on bare function names. If a user defines:
```go
func ReadFile(path string) error { ... }  // User-defined helper
```

And uses it in error propagation:
```dingo
let data = ReadFile(path)?
```

The import tracker will inject `import "os"` even though `os.ReadFile` was never called.

**Impact:**
- Unused import compile errors
- Confusing build failures
- False positives break clean codebases

**Current Detection:**
```go
stdLibFunctions = map[string]string{
    "ReadFile": "os",  // ← Matches ANY ReadFile call
    ...
}
```

**Fix Options:**
1. **Require package qualification:** Only detect `os.ReadFile`, `json.Marshal`, etc.
2. **AST resolution:** Check if function actually resolves to stdlib package
3. **Conservative detection:** Only inject imports for qualified calls

**Recommendation:** Start with option #1 (require qualification) as it's simpler and prevents false positives.

---

#### I2: Missing Negative Tests
**Location:** `pkg/preprocessor/preprocessor_test.go`

**Problem:**
No tests cover:
1. User-defined functions shadowing stdlib names
2. Source mappings for lines before import block when offsets applied

**Impact:**
- Bugs C1 and I1 weren't caught before code review
- Regression risk when making changes

**Required Tests:**
```go
TestUserDefinedFunctionsDontTriggerImports()
TestMappingsBeforeImportsNotShifted()
TestMultiValueReturnWithErrorProp()
```

---

## Current Project Status

### ✅ Working Features (Phase 2.7 Complete)
- Sum types with `enum` keyword
- Pattern matching with `match` expressions
- Error propagation with `?` operator (has bugs above)
- Functional utilities (`map`, `filter`, `reduce`, etc.)
- Clean CLI tooling (`dingo build`, `dingo run`)
- Plugin architecture with dependency resolution

### 🔨 Infrastructure Ready
- `Result[T, E]` type infrastructure (not integrated)
- `Option[T]` type infrastructure (not integrated)
- Type inference service
- Import injection pipeline (has bugs above)

### 📊 Test Coverage
- Core tests (pkg/*): 164/164 passing (100%)
- Integration tests: 8 passing, 4 failing, 33+ skipped
- Golden tests: 1/18 passing (most need parser features)

### 🏗️ Architecture Status
- **Preprocessor:** Text-based transforms (error propagation, type annotations)
- **Transformer:** AST-based transforms (lambdas, pattern matching, safe navigation)
- **Separation:** Clear, documented in README files

---

## Priority Assessment

### Must Fix Before Phase 3 (Blockers)
1. **C1: Source-map offset bug** - Breaks IDE navigation
2. **C2: Multi-value return bug** - Generates invalid Go code
3. **I2: Add negative tests** - Prevent regressions

### Should Fix (Quality Improvement)
4. **I1: Import detection false positives** - Reduces user frustration

### Can Defer (Nice-to-Have)
- None identified in this review

---

## Recommended Next Steps

### Option A: Fix Critical Bugs First (Recommended)
**Timeline:** 1-2 days

1. Fix C1: Source-map offset logic (~2 hours)
2. Fix C2: Multi-value return handling (~4 hours)
3. Add comprehensive negative tests (~3 hours)
4. Fix I1: Improve import detection (~2 hours)
5. Run full test suite and validate fixes

**Rationale:**
- These bugs break core functionality
- Quick fixes with high impact
- Builds confidence for Phase 3
- No new features, just correctness

### Option B: Start Phase 3 (Result/Option Integration)
**Timeline:** 1-2 weeks

Proceed with Result/Option integration while accepting current bugs:
- Result[T, E] constructor integration
- Option[T] constructor integration
- Pattern matching integration
- Go interop (auto-wrapping)

**Risk:**
- Building on buggy foundation
- May need to refactor error propagation later
- Source maps unreliable during development

### Option C: Hybrid Approach
**Timeline:** 3-4 days

Fix only CRITICAL issues (C1, C2), defer IMPORTANT:
1. Fix source-map offset bug (C1)
2. Fix multi-value return bug (C2)
3. Add minimal tests for fixed issues
4. Start Phase 3 with stable foundation

**Rationale:**
- Address show-stoppers
- Defer nice-to-haves (I1) to later
- Maintain momentum on new features

---

## Recommendation: **Option A** (Fix All Issues First)

**Why:**
1. **Quality Over Speed:** We're pre-v1.0, now is the time to fix bugs
2. **Low Time Cost:** Total fix time ~11 hours (1.5 days)
3. **High Impact:** Stable foundation for Phase 3
4. **Test Confidence:** Comprehensive tests prevent future regressions
5. **User Experience:** Working source maps and correct code generation

**Phase 3 can start with confidence knowing the foundation is solid.**

---

## Phase 3 Preview (After Fixes)

Once critical bugs are resolved, Phase 3 goals:

### Phase 3.1: Result/Option Integration (2 weeks)
- Integrate Result[T, E] constructors with type inference
- Integrate Option[T] constructors with type inference
- Pattern matching support for Result/Option
- Golden tests for all scenarios

### Phase 3.2: Go Interop (1 week)
- Auto-wrap Go functions returning (T, error) → Result[T, E]
- Auto-wrap Go functions returning *T → Option[T]
- Configurable wrapping modes

### Phase 3.3: Lambda Syntax (2 weeks)
- Rust-style: `|x| x * 2`
- TypeScript-style: `(x) => x * 2`
- Kotlin-style: `{ it * 2 }`
- Swift-style: `{ $0 * 2 }`

**Total Phase 3 Timeline:** ~5-6 weeks

---

## Open Questions

1. **Multi-value returns:** Should `return expr?` be constrained to single non-error returns, or must we support full multi-value propagation?

2. **Import detection policy:** Should we require package qualification (conservative) or implement full AST resolution (complex but accurate)?

3. **Test coverage targets:** What percentage should we aim for before starting Phase 3?

4. **External reviews:** Should we run more external code reviews (Grok, Gemini) after fixes?
