# Action Items - Critical and Important Fixes

**Priority Order:** Work through this list sequentially. Do not skip critical issues.

---

## CRITICAL (Must Fix Immediately - Blocks All Functionality)

### 1. Generate Type Declarations for Result Structs
**Files:** `/pkg/plugin/builtin/result_type.go`
**Effort:** 6-8 hours

**Action:**
Before generating any `Result_T_E` composite literal, emit the struct declaration:
- Add method `emitResultDeclaration(file *ast.File, T, E string)`
- Track emitted types to avoid duplicates
- Generate: struct type, tag enum, constants
- Coordinate with sum_types plugin or generate independently

**Success Criteria:**
- Generated code compiles without "undefined type" errors
- `go/types` can type-check Result usages
- All Result transformations produce valid Go code

---

### 2. Generate Type Declarations for Option Structs
**Files:** `/pkg/plugin/builtin/option_type.go`
**Effort:** 6-8 hours

**Action:**
Same as #1 but for Option types:
- Add method `emitOptionDeclaration(file *ast.File, T string)`
- Generate Option_T struct, OptionTag enum, constants
- Track emitted types

**Success Criteria:**
- Generated code compiles without "undefined type" errors
- All Some() transformations produce valid Go code

---

### 3. Fix Err() Type Inference
**Files:** `/pkg/plugin/builtin/result_type.go` (lines 134-177)
**Effort:** 8-10 hours

**Action:**
Implement context-based type inference for Err():
- Walk AST to find enclosing function
- Extract Result[T, E] from function return type
- Parse T parameter from Result type annotation
- Fall back to error message if inference fails (don't use "T")

**Immediate Workaround (1 hour):**
Generate compile error instead of placeholder:
```go
if valueTypeName == "" {
    // Generate invalid code that clearly shows the problem
    ctx.Logger.Error("Err() requires function return type annotation")
    valueTypeName = "ERROR_CANNOT_INFER_TYPE"  // Will fail compilation with clear message
}
```

**Success Criteria:**
- Err() infers correct success type in functions with Result return types
- Clear error messages when inference fails
- No more "Result_T_error" placeholders

---

### 4. Fix Empty Enum GenDecl
**Files:** `/pkg/parser/participle.go` (lines 466-480)
**Effort:** 4-6 hours

**Action:**
Generate valid AST for enum declarations:
- Option A: Generate proper struct + enum + constants immediately
- Option B: Skip empty GenDecl, store metadata for later processing
- Add validation to ensure all GenDecls have non-empty Specs

**Success Criteria:**
- go/types no longer crashes on generated AST
- All golden tests can run type checking
- Phase 4 tests pass

---

### 5. Fix Silent Type Inference Errors
**Files:** `/pkg/plugin/builtin/type_inference.go` (lines 71-87)
**Effort:** 3-4 hours

**Action:**
Replace silent error handler with proper logging and collection:
- Collect errors in slice: `s.errors = append(s.errors, err)`
- Log all errors (minimum: Warn level)
- Add strict mode configuration (fail-fast vs permissive)
- Add methods: HasErrors(), GetErrors()

**Success Criteria:**
- All type errors are logged
- Developers can see why type inference failed
- Configurable strict mode works

---

### 6. Restore or Verify Error Propagation Plugin
**Files:** `/pkg/plugin/builtin/error_propagation.go`, `error_propagation_test.go`
**Effort:** 4-6 hours

**Action:**
- Check if error_propagation.go exists
- If missing: restore from git history
- Update tests to match new plugin architecture
- Add integration tests for `?` operator
- Document current state

**Success Criteria:**
- `?` operator works correctly
- Tests pass
- Integration with Result types verified

---

## IMPORTANT (Fix in Next Sprint)

### 7. Implement None Transformation
**Files:** `/pkg/plugin/builtin/option_type.go`
**Effort:** 6-8 hours

**Action:**
Same approach as Err() fix:
- Infer T from function return type or assignment target
- Support explicit type syntax: `None::<T>()`
- Generate proper Option_T_None composite literal

**Success Criteria:**
- None returns work in functions with Option return types
- Clear error messages when inference fails

---

### 8. Replace Reflection in Pipeline
**Files:** `/pkg/plugin/pipeline.go` (lines 183-225)
**Effort:** 2-3 hours

**Action:**
- Create `pkg/typeinfo/service.go` with Service interface
- Define Refresh, Close, InferType methods
- Update pipeline to use interface instead of reflection
- Update TypeInferenceService to implement interface

**Success Criteria:**
- Type-safe method calls
- Better performance
- Compile-time error checking

---

### 9. Fix Type Name Collision Risk
**Files:** `/pkg/plugin/builtin/result_type.go`, `option_type.go`
**Effort:** 2 hours

**Action:**
Update sanitizeTypeName with unique delimiters:
- Use `_L_` and `_R_` for brackets
- Use `_PTR_` for pointers
- Use `_DOT_` for package separators
- Hash very long type names (>100 chars)

**Success Criteria:**
- No ambiguous type names
- Complex nested types work correctly

---

### 10. Fix Type Inference Cache Invalidation
**Files:** `/pkg/plugin/builtin/type_inference.go`
**Effort:** 1-2 hours

**Action:**
In Refresh(), clear all cached data:
- Set caches to nil before recreating
- Clear types.Info maps completely
- Ensure no stale references

**Success Criteria:**
- No cache hits on wrong nodes after transformations
- No panics from missing cache entries

---

### 11. Fix TypeInference Lifecycle Management
**Files:** `/pkg/plugin/pipeline.go`, `/pkg/generator/generator.go`
**Effort:** 3-4 hours

**Action:**
Move lifecycle to generator level:
- Initialize TypeInferenceService once in Generator
- Defer Close() in generator, not pipeline
- Add nil checks in all service methods
- Document lifecycle requirements

**Success Criteria:**
- No premature Close() calls
- Service remains valid throughout transformation
- No race conditions in multi-file builds

---

### 12. Add Synthetic Type Registration
**Files:** `/pkg/plugin/builtin/result_type.go`, `option_type.go`
**Effort:** 2 hours

**Action:**
After generating Result/Option literals, register with TypeInferenceService:
```go
if service, ok := ctx.TypeInference.(*TypeInferenceService); ok {
    service.RegisterSyntheticType(typeName, &SyntheticTypeInfo{...})
}
```

**Success Criteria:**
- IsResultType() and IsOptionType() work correctly
- Cross-plugin type detection works

---

### 13. Fix Memory Leaks in Close()
**Files:** `/pkg/plugin/builtin/type_inference.go`
**Effort:** 1 hour

**Action:**
In Close(), set all fields to nil:
```go
s.typeCache = nil
s.syntheticTypes = nil
s.info = nil
s.pkg = nil
s.config = nil
```

**Success Criteria:**
- Memory released properly
- No growth in long-running builds

---

### 14. Add Error Propagation Tests
**Files:** `/pkg/plugin/builtin/error_propagation_test.go`
**Effort:** 3-4 hours

**Action:**
Create comprehensive test suite:
- TestErrorPropagationPlugin_Transform
- TestErrorPropagationPlugin_StatementLifting
- TestErrorPropagationPlugin_IntegrationWithResult

**Success Criteria:**
- 80%+ code coverage
- All transformation cases tested

---

### 15. Document or Remove Dead Config Flags
**Files:** `/pkg/config/config.go`
**Effort:** 1 hour (documentation) OR remove

**Action:**
Either document as planned features or remove:
- AutoWrapGoErrors - add "NOTE: Not yet implemented"
- AutoWrapGoNils - add "NOTE: Not yet implemented"
Or delete fields entirely until implementation

**Success Criteria:**
- No user confusion about feature availability

---

### 16. Establish Error Handling Conventions
**Files:** `CLAUDE.md`, multiple plugins
**Effort:** 2-3 hours

**Action:**
- Document error handling policy in CLAUDE.md
- Add examples of proper error handling
- Audit plugins for consistency
- Add TransformResult type with Errors/Warnings

**Success Criteria:**
- Consistent error handling across all plugins
- Clear documentation for contributors

---

### 17. Add Golden Tests for Result/Option
**Files:** `tests/golden/result_*.dingo`, `option_*.dingo`
**Effort:** 4-5 hours

**Action:**
Create golden test files:
- result_ok.dingo
- result_err.dingo
- option_some.dingo
- option_none.dingo
- result_helpers.dingo (IsOk, Unwrap, etc.)

**Success Criteria:**
- All tests compile and run
- Generated code validates correctly

---

## Summary

**Total Action Items:** 17 (6 CRITICAL + 11 IMPORTANT)
**Estimated Total Effort:** 48-66 hours

**Recommended Schedule:**
- **Week 1:** Critical issues #1-6 (25-34 hours)
- **Week 2:** Important issues #7-12 (17-22 hours)
- **Week 3:** Important issues #13-17 (10-16 hours)

**Dependencies:**
- #1 and #2 should be done together (similar implementation)
- #3 and #7 can share infrastructure (context-based type inference)
- #4 must be done before golden tests can run
- #5 should be done early (affects debugging of other issues)
- #6 is independent and can be done in parallel

**Success Metric:**
All action items complete when:
- Generated Go code compiles without errors
- All golden tests pass
- go/types can type-check all generated AST
- No silent failures or crashes
