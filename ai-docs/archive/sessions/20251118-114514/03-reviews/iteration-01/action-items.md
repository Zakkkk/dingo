# Phase 3 Action Items
**Date**: 2025-11-18
**Session**: 20251118-114514
**Source**: Consolidated review of 4 reviews

---

## CRITICAL (Must Fix Before Phase 4)

### 1. Fix Type Parsing Vulnerability for Complex Types
**Severity**: CRITICAL
**Files**: `pkg/plugin/builtin/type_inference.go:220-285`

**Problem**:
`parseTypeFromTokensBackward` and `parseTypeFromTokensForward` break for complex types like `Result[map[string]int, error]` because they assume simple `_` token splitting.

**Action**:
- [ ] Store original type parameters in `ResultTypeInfo` struct
  - Add `OkTypeString string` field
  - Add `ErrTypeString string` field
- [ ] Modify `GetResultTypeParams()` to use cached values only (don't reverse-parse)
- [ ] Add validation in `RegisterResultType()` to verify round-trip consistency
- [ ] Add golden tests for complex types:
  - [ ] `Result[map[string]int, error]`
  - [ ] `Result[chan int, error]`
  - [ ] `Result[struct{}, error]`
  - [ ] `Result[*User, error]` (nested)

**Code Changes**:
```go
// pkg/plugin/builtin/type_inference.go
type ResultTypeInfo struct {
    TypeName      string
    OkType        types.Type
    ErrType       types.Type
    OkTypeString  string  // NEW: Store original, don't reverse-parse
    ErrTypeString string  // NEW: Store original, don't reverse-parse
}

func (s *TypeInferenceService) GetResultTypeParams(typeName string) (T, E types.Type, ok bool) {
    if cached, found := s.resultTypeCache[typeName]; found {
        return cached.OkType, cached.ErrType, true
    }
    // Don't reverse-parse - fail if not cached
    s.logger.Warn("Result type %s not in cache - cannot infer types", typeName)
    return nil, nil, false
}
```

---

### 2. Add Error Accumulation Limits
**Severity**: CRITICAL
**Files**: `pkg/plugin/plugin.go:121`, `Context.errors`

**Problem**:
Unbounded `errors` slice can cause OOM on large files with many type inference failures (10,000+ errors).

**Action**:
- [ ] Add `MaxErrors` constant (recommend 100-500)
- [ ] Modify `ReportError()` to check limit
- [ ] Add "too many errors" sentinel when limit reached
- [ ] Add unit test for error accumulation limit

**Code Changes**:
```go
// pkg/plugin/plugin.go
const MaxErrors = 100

func (ctx *Context) ReportError(msg string, pos token.Pos) {
    if len(ctx.errors) >= MaxErrors {
        if len(ctx.errors) == MaxErrors {
            ctx.errors = append(ctx.errors,
                fmt.Errorf("too many errors (>%d), stopping error collection", MaxErrors))
        }
        return
    }
    ctx.errors = append(ctx.errors, ...)
}
```

**Test**:
```go
func TestErrorAccumulationLimit(t *testing.T) {
    ctx := &plugin.Context{}
    for i := 0; i < 200; i++ {
        ctx.ReportError("error", token.NoPos)
    }
    require.LessOrEqual(t, len(ctx.errors), plugin.MaxErrors+1) // +1 for sentinel
}
```

---

### 3. Fix Type Inference Fallback Returns Empty String
**Severity**: CRITICAL
**Files**: `pkg/plugin/builtin/result_type.go:190-199`, `option_type.go:130-144`

**Problem**:
`inferTypeFromExpr` returns `""` on failure, but callers inconsistently check this, leading to invalid type names like `Result_int_`.

**Action**:
- [ ] Change `inferTypeFromExpr` return signature to `(string, error)`
- [ ] Update all call sites to check error
- [ ] Add validation before all `fmt.Sprintf` type name construction
- [ ] Fix failing unit tests:
  - [ ] `TestEdgeCase_InferTypeFromExprEdgeCases/identifier`
  - [ ] `TestEdgeCase_InferTypeFromExprEdgeCases/function_call`
  - [ ] `TestConstructor_OkWithIdentifier`
  - [ ] `TestConstructor_OkWithFunctionCall`

**Code Changes**:
```go
// pkg/plugin/builtin/result_type.go
func (p *ResultTypePlugin) inferTypeFromExpr(expr ast.Expr) (string, error) {
    // ... inference logic ...
    if typeName == "" {
        return "", fmt.Errorf("type inference failed for %s", FormatExprForDebug(expr))
    }
    return typeName, nil
}

// Usage
okType, err := p.inferTypeFromExpr(valueArg)
if err != nil {
    p.ctx.ReportError(err.Error(), valueArg.Pos())
    return call
}

errType, err := p.inferTypeFromExpr(errArg)
if err != nil {
    p.ctx.ReportError(err.Error(), errArg.Pos())
    return call
}

// Validate before constructing type name
if okType == "" || errType == "" {
    p.ctx.ReportError("Type inference incomplete", call.Pos())
    return call
}
resultTypeName := fmt.Sprintf("Result_%s_%s", okType, errType)
```

---

## IMPORTANT (Should Fix in Phase 4)

### 4. Error Reporting Doesn't Fail Compilation
**Severity**: IMPORTANT
**Files**: `pkg/plugin/plugin.go`, `pkg/plugin/builtin/result_type.go:192-199`

**Problem**:
When type inference fails, code returns unchanged call expression. Generated Go code fails to compile with confusing errors.

**Action**:
- [ ] Generate `BadExpr` or sentinel on type inference failure
- [ ] OR: Collect errors and fail transpilation if any errors accumulated
- [ ] Add integration test for error reporting

**Code Changes** (Option 1 - Fail transpilation):
```go
// pkg/generator/generator.go
func (g *Generator) Generate(file *ast.File) error {
    // ... run plugins ...

    if len(ctx.Errors) > 0 {
        for _, err := range ctx.Errors {
            fmt.Fprintf(os.Stderr, "%s\n", err.FormatWithPosition(fset))
        }
        return fmt.Errorf("transpilation failed with %d error(s)", len(ctx.Errors))
    }
}
```

**Code Changes** (Option 2 - Generate BadExpr):
```go
// pkg/plugin/builtin/result_type.go
if okType == "" {
    return &ast.BadExpr{
        From: call.Pos(),
        To:   call.End(),
    }
}
```

---

### 5. Fix Map Index Addressability Edge Case
**Severity**: IMPORTANT
**Files**: `pkg/plugin/builtin/addressability.go:49`

**Problem**:
Map index expressions (`m[key]`) are NOT addressable in Go, but `isAddressable()` returns `true`.

**Action**:
- [ ] Check if base type is map using `go/types`
- [ ] Return `false` for map indexing
- [ ] Add test for `Ok(myMap[key])`
- [ ] Document workaround in user docs

**Code Changes**:
```go
// pkg/plugin/builtin/addressability.go
case *ast.IndexExpr:
    // Check if base is map type
    if s.typeInference != nil {
        if baseType, ok := s.typeInference.InferType(e.X); ok {
            if _, isMap := baseType.Underlying().(*types.Map); isMap {
                return false // Map values are not addressable
            }
        }
    }
    // Array/slice indexing is addressable
    return true
```

---

### 6. Implement or Document None Constant Limitations
**Severity**: IMPORTANT
**Files**: `pkg/plugin/builtin/option_type.go:124-173`, `type_inference.go:601-610`

**Problem**:
`InferTypeFromContext()` is a stub that always returns `false`. None constant doesn't work without explicit types.

**Action** (Choose one):

**Option A - Document limitation (Phase 3)**:
- [ ] Add experimental warning to `handleNoneExpression()` godoc
- [ ] Update CHANGELOG.md: "None constant (limited - requires explicit types)"
- [ ] Update test expectations (mark as expected failure)

**Option B - Implement context inference (Phase 4)**:
- [ ] Implement `InferTypeFromContext()` with AST parent tracking
- [ ] Support assignment LHS type inference
- [ ] Support return type inference
- [ ] Support parameter type inference

**Code Changes** (Option B - Phase 4):
```go
// pkg/plugin/builtin/type_inference.go
func (s *TypeInferenceService) InferTypeFromContext(node ast.Node) (types.Type, bool) {
    // Use astutil to find parent node
    // Check if parent is:
    //   - *ast.AssignStmt → get LHS type
    //   - *ast.ReturnStmt → get function return type
    //   - *ast.CallExpr → get parameter type
    // Use go/types to resolve these types
}
```

---

### 7. Make TypeRegistry Thread-Safe
**Severity**: IMPORTANT
**Files**: `pkg/plugin/builtin/type_inference.go:56-69`

**Problem**:
`TypeRegistry` maps are not synchronized, risking data races if plugins run in parallel.

**Action**:
- [ ] Add `sync.RWMutex` to `TypeRegistry`
- [ ] Wrap all map accesses with locks
- [ ] Add `go test -race` to CI
- [ ] Document concurrency expectations

**Code Changes**:
```go
// pkg/plugin/builtin/type_inference.go
type TypeRegistry struct {
    mu          sync.RWMutex
    resultTypes map[string]*ResultTypeInfo
    optionTypes map[string]*OptionTypeInfo
}

func (r *TypeRegistry) RegisterResult(name string, info *ResultTypeInfo) {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.resultTypes[name] = info
}

func (r *TypeRegistry) GetResult(name string) (*ResultTypeInfo, bool) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    info, ok := r.resultTypes[name]
    return info, ok
}
```

---

## MINOR (Nice to Have)

### 8. Extract Duplicated Code to Shared Helpers
**Severity**: MINOR
**Files**: `result_type.go`, `option_type.go`

**Action**:
- [ ] Create `pkg/plugin/builtin/type_helpers.go`
- [ ] Extract `SanitizeTypeName()`, `GetTypeName()`, `TypeToAST()`
- [ ] Update Result and Option plugins to use shared functions

---

### 9. Add Nil Checks for Defensive Programming
**Severity**: MINOR
**Files**: `addressability.go:27,122`

**Action**:
- [ ] Add nil check to `isAddressable(expr ast.Expr)`
- [ ] Add nil check to `wrapInIIFE(expr ast.Expr, ...)`
- [ ] Add nil check to `MaybeWrapForAddressability()`

---

### 10. Improve Package-Level Documentation
**Severity**: MINOR
**Files**: `pkg/errors/type_inference.go:1-2`

**Action**:
- [ ] Expand package comment to explain error categories
- [ ] Document error reporting strategy
- [ ] Explain relationship to Go's error handling

---

### 11. Benchmark IIFE Performance
**Severity**: MINOR
**Files**: `pkg/plugin/builtin/addressability_test.go`

**Action**:
- [ ] Add benchmark for IIFE overhead vs direct address-of
- [ ] Verify Go compiler inlines IIFEs with `-gcflags=-m`
- [ ] Document performance characteristics in godoc

---

### 12. Document Helper Method Limitations
**Severity**: MINOR
**Files**: `pkg/plugin/builtin/result_type.go`, `option_type.go`

**Action**:
- [ ] Document that `Map()`, `AndThen()` require type assertions
- [ ] Add examples to godoc showing type assertion usage
- [ ] Plan generics support for Phase 5

---

### 13. Improve Type Sanitization Reversibility
**Severity**: MINOR
**Files**: `result_type.go`, `option_type.go:1699-1724`

**Action**:
- [ ] Use bidirectional map for type name mappings
- [ ] Add type hash for disambiguation
- [ ] Verify round-trip consistency in tests

---

## Summary

**Total Action Items**: 13
- CRITICAL: 3 (must fix before Phase 4)
- IMPORTANT: 4 (should fix in Phase 4)
- MINOR: 6 (nice to have)

**Estimated Effort**:
- CRITICAL items: 1-2 days
- IMPORTANT items: 2-3 days
- MINOR items: 1-2 days

**Priority Order**:
1. Item 3 (Type inference fallback) - Highest user impact
2. Item 1 (Type parsing) - Prevents silent failures
3. Item 2 (Error limits) - Prevents OOM
4. Item 4 (Error reporting) - Better UX
5. Items 5-7 (Phase 4 improvements)
6. Items 8-13 (Code quality)

---

**End of Action Items**
