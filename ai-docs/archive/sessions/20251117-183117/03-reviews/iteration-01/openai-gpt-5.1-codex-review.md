# Code Review: Phase 2.2 Error Propagation Implementation

**Reviewer:** GPT-5.1 Codex (openai/gpt-5.1-codex)
**Date:** 2025-11-17
**Session:** 20251117-183117
**Scope:** 10 files (7 new, 3 modified)

---

## CRITICAL Issues

### 1. Invalid Return Statement Generation
**File:** `pkg/preprocessor/error_prop.go:205-268`
**Category:** CRITICAL

**Problem:**
Expansion for `return expr?` blindly emits `return __tmpN`, but `expr?` forms (e.g. `Read()` returning `(T, error)`) require `return __tmpN` *and* its paired error removed from the tuple. As written it returns only the first value, dropping the error and producing invalid Go signatures.

**Impact:**
Generated code will not match function signature, resulting in compilation errors. This breaks the core functionality of error propagation for return statements.

**Recommended Fix:**
Assign both values from the call and return `__tmpN` plus existing tail of original return expression, or reuse zero-value inference to emit the correct tuple. For example:
```go
// Instead of:
return __tmpN

// Should generate:
__tmpN, __errN := expr
if __errN != nil {
    return zeroValues..., __errN
}
return __tmpN, nil
```

---

### 2. Incorrect Zero Values for Composite Types
**File:** `pkg/preprocessor/error_prop.go:334-393`
**Category:** CRITICAL

**Problem:**
`getZeroValue` returns `"nil"` for all non-primitive types, including structs and arrays. When a function returns a struct (common for data builders), `nil` is not assignable and the generated code will not compile.

**Impact:**
Any function returning a struct type will generate uncompilable Go code. This is a fundamental correctness issue that breaks many real-world use cases.

**Recommended Fix:**
Implement proper zero-value generation using `typ{}` for structs/arrays or leverage `go/types` to detect composite kinds:
```go
// Add cases for:
- Struct types → T{}
- Array types → [N]T{}
- Named types → consult go/types for underlying kind
```

---

### 3. Import Management Fails for Multi-line Imports
**File:** `pkg/preprocessor/error_prop.go:416-446`
**Category:** CRITICAL

**Problem:**
`ensureFmtImport` assumes there is at most one-line import block. When a file already has a multi-line `import (...)`, the function does not insert `"fmt"` and no error is raised, producing generated code with `fmt.Errorf` but no fmt import (compile failure).

**Impact:**
Files with existing multi-line imports will fail to compile when error wrapping is used. This is a common case in production code.

**Recommended Fix:**
Parse/modify import blocks via `go/parser` or detect `import (` and inject `"fmt"` inside the parentheses:
```go
// Detect import block:
if strings.Contains(line, "import (") {
    // Find closing ) and inject "fmt" before it
}
```

---

## IMPORTANT Issues

### 4. Multi-line Expression Support Missing
**File:** `pkg/preprocessor/error_prop.go:121-133` and `135-233`
**Category:** IMPORTANT

**Problem:**
Regular-expression driven detection/expansion only supports single-line `let`/`return` statements. Multi-line expressions or nested constructs (e.g. `x := foo(\n ...\n)?`) bypass transformation, so `?` silently survives and the compiler later errors.

**Impact:**
Code using multi-line function calls with error propagation will silently fail to be transformed, leading to confusing compiler errors about unknown `?` operator.

**Recommended Fix:**
Replace ad-hoc regex parsing with a token/AST-based walker to recognize multi-line statements. Consider buffering lines until finding complete statement boundaries.

---

### 5. Type Annotation Fails for Multi-line Signatures
**File:** `pkg/preprocessor/type_annot.go:35-66`
**Category:** IMPORTANT

**Problem:**
Type annotation replacement scans only the single line containing `func` and fails when signatures span multiple lines (typical in Go). As a result parameters below the first line retain `:` syntax and break parsing.

**Impact:**
Functions with long parameter lists (common in production code) will fail to parse, limiting the usability of Dingo's type annotation syntax.

**Recommended Fix:**
Implement a proper parser or multiline scanner that walks until the closing parenthesis:
```go
// Buffer lines from func keyword until finding matching )
// Then process entire signature as one unit
```

---

### 6. Keyword Replacement Affects Strings/Comments
**File:** `pkg/preprocessor/keywords.go:20-27`
**Category:** IMPORTANT

**Problem:**
Regex replaces every `let` token globally, so `let` inside strings/comments/identifiers (e.g. `outlet`) are mutated incorrectly.

**Impact:**
String literals containing "let", comments, or identifiers like "outlet" will be corrupted, leading to compilation errors or semantic bugs.

**Recommended Fix:**
Restrict replacements to actual declarations, e.g., by tokenizing source or piggybacking on AST:
```go
// Use go/scanner to identify actual keywords vs literals
// Or restrict regex to statement positions only
```

---

### 7. Source Map Becomes Inaccurate After Generator
**File:** `cmd/dingo/main.go:217-277`
**Category:** IMPORTANT

**Problem:**
Build pipeline writes preprocessed code to parser, then runs plugin generator but never updates source map with plugin transformations or final output offsets. As soon as generator rewrites AST (future work), `.map` becomes inaccurate.

**Impact:**
LSP features relying on source maps will provide incorrect positions, breaking go-to-definition, diagnostics, and other IDE features.

**Recommended Fix:**
Either disable map emission until full pipeline integration, or extend generator to emit mapping deltas before writing files. Document limitation clearly if deferring.

---

### 8. Plugin Pipeline Stub Misleads Architecture
**File:** `pkg/plugin/plugin.go:27-45`
**Category:** IMPORTANT

**Problem:**
Pipeline `Transform` is stubbed to no-op and Stats always zero, yet CLI advertises "plugins integrated." This breaks architectural alignment with final-plan.md (plugins expected to run transformations, type inference, logging).

**Impact:**
Architectural confusion and misleading CLI output. Future developers may assume plugin system is functional when it's actually stubbed.

**Recommended Fix:**
Either wire actual plugin execution or clearly disable plugin path in CLI until functional. Add TODO comments indicating stub status.

---

## MINOR Issues

### 9. Brace Handling in Function Signature Parsing
**File:** `pkg/preprocessor/error_prop.go:292`
**Category:** MINOR

**Problem:**
`src := fmt.Sprintf("package p\n%s}", funcText.String())` appends `}` unconditionally, producing mismatched braces when signature already includes body opening. Parsing happens to succeed for trivial cases but is brittle and will panic for signatures containing literal braces (e.g., generics in comments).

**Impact:**
Edge cases with complex signatures may cause parser panics or incorrect AST generation.

**Recommended Fix:**
Use `fmt.Sprintf("package p\n%s", funcText.String())` and ensure captured snippet is balanced before parsing.

---

### 10. Silent Nil Return in ParseExpr
**File:** `pkg/parser/simple.go:37-39`
**Category:** MINOR

**Problem:**
`ParseExpr` silently returns `nil, nil`, masking unsupported paths. Callers expecting an AST node will dereference nil and panic.

**Impact:**
Potential nil pointer panics in code paths using ParseExpr.

**Recommended Fix:**
Return `nil, fmt.Errorf("ParseExpr not implemented")` to fail fast.

---

### 11. Limited Type Pattern Support in Regex
**File:** `pkg/preprocessor/type_annot.go:71-89`
**Category:** MINOR

**Problem:**
Regex for parameter types doesn't handle qualified names (`pkg.Type`), generics, or interfaces with spaces, so many honest cases pass through.

**Impact:**
Type annotations with qualified names or complex types will not be converted, leading to parse errors.

**Recommended Fix:**
Consider leveraging Go parser to extract parameter lists rather than regex:
```go
// Use go/parser to parse function signature
// Extract parameter types from AST
// Replace : with space based on AST structure
```

---

## Summary

### Issue Counts
- **CRITICAL:** 3
- **IMPORTANT:** 5
- **MINOR:** 3
- **TOTAL:** 11

### Key Concerns

1. **Error propagation expansion emits invalid return statements** and uses incorrect zero values for most composite types, resulting in uncompilable Go.

2. **Import injection and keyword/type-annotation preprocessors rely on brittle regexes**, failing for common multi-line/qualified scenarios.

3. **Plugin pipeline and source-map integration diverge from architectural plans**, so CLI claims about plugin-enabled builds are misleading.

### Recommendations

The implementation demonstrates solid progress on the core error propagation feature, but has critical correctness issues that must be addressed before merging:

1. **Fix zero value generation** to handle all Go types correctly (structs, arrays, custom types)
2. **Fix return statement expansion** to emit complete, valid return tuples
3. **Improve import management** to handle multi-line import blocks
4. **Consider AST-based preprocessing** to replace brittle regex patterns for multi-line support
5. **Clarify plugin system status** and align CLI output with actual functionality

### Testing Gap

While the implementation reportedly passes all 8 error_prop golden tests, the critical issues identified suggest either:
- The golden tests don't cover composite return types, multi-line imports, or multi-line signatures
- The tests are passing with incorrect/incomplete golden files

Recommend expanding test coverage to include:
- Functions returning struct types
- Multi-line function signatures with type annotations
- Multi-line import blocks with error wrapping
- Edge cases for keyword replacement in strings/comments

---

**STATUS:** CHANGES_NEEDED
**CRITICAL_COUNT:** 3
**IMPORTANT_COUNT:** 5
**MINOR_COUNT:** 3
