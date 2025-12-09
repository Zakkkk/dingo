# Action Items: Sum Types Implementation Fixes (Iteration 01)

Generated from consolidated review of Grok and Codex feedback.

---

## CRITICAL (Block Merge - Fix Immediately)

### 1. Fix Tag Constant Naming and Enum Registry Usage
**Related Issues:** C1, C2
**Files:** sum_types.go:483-487, :92-102
**Estimated Time:** 2-3 hours

**Problem:**
- Match arms generate `Tag_VARIANT` but should use `EnumTag_Variant`
- enumRegistry collected but never used
- No type inference to determine enum from match subject

**Fix:**
1. Update transformMatchArm to accept enumName parameter
2. Use enumRegistry to generate correct tag constant names: `enumName + "Tag_" + variantName`
3. For Phase 1, add error if enum type cannot be inferred from match subject
4. Document that full type inference is Phase 3 work

---

### 2. Fix Duplicated Declaration Output
**Related Issues:** C3
**Files:** sum_types.go:84-87, :143-160, :208-216
**Estimated Time:** 1 hour

**Problem:**
Tag const block appended twice (once in generateTagEnum, once in transformEnumDecl)

**Fix:**
- Ensure generateTagEnum ONLY returns declaration, doesn't append
- transformEnumDecl should be sole place that appends to generatedDecls
- Verify each generated declaration added exactly once

---

### 3. Fix Unsafe Tuple Variant Field Handling
**Related Issues:** C4
**Files:** sum_types.go:265-281, participle.go:660-668
**Estimated Time:** 2 hours

**Problem:**
- generateVariantFields assumes named Idents but tuples use synthesized `_0`, `_1`
- No nil check for variant.Fields
- Field naming mismatch between parser and plugin

**Fix:**
1. Add nil check: `if variant.Kind == dingoast.VariantUnit || variant.Fields == nil { return nil }`
2. Standardize tuple field naming across parser and plugin
3. Add validation for f.Names before dereferencing
4. Pre-allocate fields slice with proper capacity

---

### 4. Verify and Fix Plugin Registration
**Related Issues:** C5
**Files:** Transpiler/generator initialization files
**Estimated Time:** 30 minutes

**Problem:**
Plugin may not be registered with transpiler pipeline

**Fix:**
1. Search for plugin registration in transpiler/generator setup
2. Add registration if missing: `generator.RegisterPlugin(&SumTypesPlugin{})`
3. Verify plugin hooks are called during transpilation

---

### 5. Add Validation for Duplicate Variant Names
**Related Issues:** C6
**Files:** sum_types.go:92-102
**Estimated Time:** 1 hour

**Problem:**
Parser accepts duplicate variant identifiers

**Fix:**
1. Modify collectEnums to return error
2. Check for duplicate enum names in registry
3. Check for duplicate variant names within each enum
4. Return clear error messages
5. Update Transform method to handle collectEnums errors

---

### 6. Add Minimum Viable Test Coverage
**Related Issues:** C7
**Files:** New test files
**Estimated Time:** 1 day

**Problem:**
Zero test coverage - cannot verify correctness

**Fix - Priority 1 (Blocking):**
1. Parser tests for unit/tuple/struct variants
2. Parser tests for match expressions with all pattern types
3. Enum transformation golden test (simple enum → Go output)
4. Match lowering unit test (verify switch generation)

**Fix - Priority 2 (Recommended):**
5. Generic enum test (Option[T])
6. Integration test: .dingo → .go → go build
7. Negative tests (duplicate variants, invalid patterns)

---

### 7. Fix Match Expression/Statement Type Mismatch
**Related Issues:** Related to C1/C2
**Files:** sum_types.go:444-469
**Estimated Time:** 3 hours (or 30 min for quick fix)

**Problem:**
Replaces ast.Expr with ast.Stmt, breaks expression contexts

**Fix Option A (Quick):**
- Detect expression context
- Return error: "match expressions not yet supported (use match statements only)"
- Document as Phase 3 work

**Fix Option B (Complete):**
- Wrap in IIFE for expression contexts
- Requires more complex code generation

---

## IMPORTANT (Fix Before v1.0)

### 8. Handle Match Expressions in Expression Contexts
**Related Issues:** I1
**Files:** sum_types.go:444-469
**Estimated Time:** 3-4 hours

**Problem:**
Need expression-friendly lowering, not just switch statements

**Fix:**
Implement IIFE wrapping or chained if-else for expression contexts

---

### 9. Implement or Document Match Transformation Limitations
**Related Issues:** I2
**Files:** sum_types.go transformation
**Estimated Time:** 4 hours

**Problem:**
Missing pattern destructuring, guards, wildcards

**Fix:**
Either:
1. Document as TODOs and emit errors for unsupported features
2. Implement basic destructuring for struct patterns
3. Transform guards into if conditions
4. Add wildcard pattern support

---

### 10. Clean Up Placeholder Nodes from DingoNodes Map
**Related Issues:** I3
**Files:** sum_types.go:162-165
**Estimated Time:** 1 hour

**Problem:**
Deleted placeholders remain in DingoNodes map

**Fix:**
1. Add RemoveDingoNode method to DingoFile (if not exists)
2. Call after cursor.Delete() in transformEnumDecl

---

### 11. Fix Constructor Parameter Aliasing
**Related Issues:** I4
**Files:** sum_types.go:295-299
**Estimated Time:** 1 hour

**Problem:**
Reuses variant.Fields.List as constructor params (shared reference)

**Fix:**
Deep copy field list before using as constructor parameters

---

### 12. Add Nil Guards for Variant Field Access
**Related Issues:** I5
**Files:** sum_types.go:489-494
**Estimated Time:** 2 hours

**Problem:**
Match lowering dereferences pointer fields without nil checks

**Fix:**
Add runtime nil checks or ensure all fields initialized to zero values

---

### 13. Add Errors for Unsupported Pattern Forms
**Related Issues:** I6
**Files:** sum_types.go:483-486
**Estimated Time:** 30 minutes

**Problem:**
Silently treats all patterns as variant patterns, panics on nil

**Fix:**
Validate pattern.Variant != nil, return clear error for unsupported patterns

---

### 14. Transform or Error on Match Guards
**Related Issues:** I7
**Files:** participle.go:166-169, sum_types.go
**Estimated Time:** 2 hours

**Problem:**
Guards parsed but silently discarded

**Fix:**
Either implement guard transformation or emit error until Phase 3

---

### 15. Add Field Name Collision Detection
**Related Issues:** I8
**Files:** Field naming strategy
**Estimated Time:** 2 hours

**Problem:**
`variantName_fieldName` can collide across variants

**Fix:**
1. Track used field names
2. Detect collisions
3. Add disambiguator (e.g., numeric suffix)

---

### 16. Document Memory Allocation Overhead
**Related Issues:** I9
**Files:** sum_types.go
**Estimated Time:** 15 minutes

**Problem:**
Current design uses more memory than optimal

**Fix:**
Add TODO comment noting future optimization opportunity (acceptable for Phase 1)

---

### 17. Add Comprehensive Match Transformation Tests
**Related Issues:** I10
**Files:** Test suite
**Estimated Time:** 1 day

**Problem:**
Need tests for expression vs statement contexts, wildcards, etc.

**Fix:**
Add comprehensive test suite covering all match transformation scenarios

---

## Summary

**Total Action Items:** 17 (7 CRITICAL, 10 IMPORTANT)

**Estimated Time:**
- CRITICAL fixes: 1.5-2 days
- Test coverage: 1 day
- IMPORTANT fixes: 2-3 days
- **Total: 4-6 days**

**Recommended Fix Order:**
1. Items 5, 2, 3 (validation and safety - 4 hours)
2. Item 4 (plugin registration - 30 min)
3. Items 1, 7 (tag naming and match basics - 3-4 hours)
4. Item 6 (tests - 1 day, can parallelize)
5. Items 8-17 (important fixes - 2-3 days)

**Critical Path Dependencies:**
- Item 1 requires Item 5 (error handling in collectEnums)
- Item 6 (tests) should verify all fixes
- Item 7 depends on Item 1 (tag constant resolution)
