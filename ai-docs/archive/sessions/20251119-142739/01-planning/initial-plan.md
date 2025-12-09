# Architecture Plan: Migrate Enum Naming to CamelCase

## Executive Summary

Migrate all enum-related naming from underscore-based convention (Option A) to pure CamelCase (Option B) to align with Go idioms and unanimous expert recommendation from 6 AI models.

**Scope**: 3 core codegen files + 46 golden test regenerations
**Impact**: Generated Go code only (no Dingo syntax changes)
**Complexity**: Medium (consistent pattern replacement)
**Estimated Effort**: 3-4 hours

---

## Current vs. Target Naming

### Pattern Comparison

| Component | Current (Underscore) | Target (CamelCase) |
|-----------|---------------------|-------------------|
| **Constructors** | `Value_Int(42)` | `ValueInt(42)` |
| | `Result_Ok(value)` | `ResultOk(value)` ✅ (already done!) |
| | `Option_Some(data)` | `OptionSome(data)` ✅ (already done!) |
| **Tags** | `ValueTag_Int` | `ValueTagInt` |
| | `StatusTag_Pending` | `StatusTagPending` |
| | `ResultTag_Ok` | `ResultTagOk` ✅ (already done!) |
| **Fields** | `int_0`, `string_0` | `int0`, `string0` |
| | `pending_0`, `active_0` | `pending0`, `active0` |
| | `ok_0`, `err_0` | `ok0`, `err0` ✅ (already done!) |

### Current State Analysis

**✅ ALREADY MIGRATED (Result/Option types)**:
- `pkg/plugin/builtin/result_type.go` - Uses `ResultTagOk`, `ResultOk`, `ok0`
- `pkg/plugin/builtin/option_type.go` - Uses `OptionTagSome`, `OptionSome`, `some0`
- Golden tests: `result_*.go.golden`, `option_*.go.golden` - Already CamelCase

**⚠️ NEEDS MIGRATION (Custom enums - pattern matching)**:
- `pkg/preprocessor/rust_match.go` - Generates `StatusTag_Pending`, `Status_Pending`, etc.
- Golden tests: `pattern_match_*.go.golden` - Currently use underscore naming

**KEY INSIGHT**: The migration is **partially complete**. Result/Option types already use CamelCase, but custom enum generation in pattern matching still uses underscores. This creates **naming inconsistency** that must be resolved.

---

## Detailed File Changes

### 1. `pkg/preprocessor/rust_match.go` (Primary Target)

This file generates enum types for pattern matching. Currently emits underscore names.

#### Line-by-Line Changes

**Function: `getTagName()` (lines 942-966)**

Current implementation:
```go
func (r *RustMatchProcessor) getTagName(pattern string) string {
	switch pattern {
	case "Ok":
		return "ResultTagOk"  // ✅ Already CamelCase
	case "Err":
		return "ResultTagErr" // ✅ Already CamelCase
	case "Some":
		return "OptionTagSome" // ✅ Already CamelCase
	case "None":
		return "OptionTagNone" // ✅ Already CamelCase
	default:
		// ❌ PROBLEM: Custom enums use underscores
		// Example: Status_Pending → StatusTag_Pending
		if idx := strings.Index(pattern, "_"); idx > 0 {
			enumName := pattern[:idx]
			variantName := pattern[idx:] // includes the underscore
			return enumName + "Tag" + variantName // StatusTag_Pending
		}
		return pattern + "Tag"
	}
}
```

**Change Required**:
```go
default:
	// ✅ NEW: Remove underscore between Tag and variant name
	// Example: Status_Pending → StatusTagPending
	if idx := strings.Index(pattern, "_"); idx > 0 {
		enumName := pattern[:idx]
		variantName := pattern[idx+1:] // ✅ Skip underscore (idx+1 instead of idx)
		return enumName + "Tag" + variantName // StatusTagPending
	}
	return pattern + "Tag"
```

**Impact**: Affects all custom enum tag constants in generated code.

---

**Function: `generateBinding()` (lines 968-998)**

Current implementation:
```go
func (r *RustMatchProcessor) generateBinding(scrutinee string, pattern string, binding string) string {
	switch pattern {
	case "Ok":
		return fmt.Sprintf("%s := *%s.ok_0", binding, scrutinee) // ❌ Should be ok0
	case "Err":
		return fmt.Sprintf("%s := %s.err_0", binding, scrutinee) // ❌ Should be err0
	case "Some":
		return fmt.Sprintf("%s := *%s.some_0", binding, scrutinee) // ❌ Should be some0
	default:
		// ❌ PROBLEM: Field names use underscores
		variantName := pattern
		if idx := strings.LastIndex(pattern, "_"); idx != -1 {
			variantName = pattern[idx+1:] // Extract "Int" from "Value_Int"
		}
		fieldName := strings.ToLower(variantName) + "_0" // int_0 ❌

		if binding != "_" {
			return fmt.Sprintf("%s := *%s.%s", binding, scrutinee, fieldName)
		}
		return fmt.Sprintf("%s := %s.%s", binding, scrutinee, fieldName)
	}
}
```

**Changes Required**:
```go
func (r *RustMatchProcessor) generateBinding(scrutinee string, pattern string, binding string) string {
	switch pattern {
	case "Ok":
		return fmt.Sprintf("%s := *%s.ok0", binding, scrutinee) // ✅ Remove underscore
	case "Err":
		return fmt.Sprintf("%s := %s.err0", binding, scrutinee) // ✅ Remove underscore
	case "Some":
		return fmt.Sprintf("%s := *%s.some0", binding, scrutinee) // ✅ Remove underscore
	default:
		variantName := pattern
		if idx := strings.LastIndex(pattern, "_"); idx != -1 {
			variantName = pattern[idx+1:]
		}
		fieldName := strings.ToLower(variantName) + "0" // ✅ Remove underscore: int0

		if binding != "_" {
			return fmt.Sprintf("%s := *%s.%s", binding, scrutinee, fieldName)
		}
		return fmt.Sprintf("%s := %s.%s", binding, scrutinee, fieldName)
	}
}
```

**Impact**: Affects field access in pattern match bindings.

---

**Function: `generateTupleBinding()` (lines 1000-1025)**

Current implementation:
```go
func (r *RustMatchProcessor) generateTupleBinding(elemVar string, variant string, binding string) string {
	switch variant {
	case "Ok":
		return fmt.Sprintf("%s := *%s.ok_0", binding, elemVar) // ❌ Should be ok0
	case "Err":
		return fmt.Sprintf("%s := *%s.err_0", binding, elemVar) // ❌ Should be err0
	case "Some":
		return fmt.Sprintf("%s := *%s.some_0", binding, elemVar) // ❌ Should be some0
	case "None":
		return ""
	default:
		// ❌ PROBLEM: Custom enum field names
		fieldName := variant + "_0" // Status_Pending_0 ❌
		return fmt.Sprintf("%s := *%s.%s", binding, elemVar, fieldName)
	}
}
```

**Changes Required**:
```go
func (r *RustMatchProcessor) generateTupleBinding(elemVar string, variant string, binding string) string {
	switch variant {
	case "Ok":
		return fmt.Sprintf("%s := *%s.ok0", binding, elemVar) // ✅ Remove underscore
	case "Err":
		return fmt.Sprintf("%s := *%s.err0", binding, elemVar) // ✅ Remove underscore
	case "Some":
		return fmt.Sprintf("%s := *%s.some0", binding, elemVar) // ✅ Remove underscore
	case "None":
		return ""
	default:
		// ✅ NEW: Remove underscores from custom enum field names
		// Example: Status_Pending → statuspending0 or pending0
		// Strategy: Use lowercase variant name without underscores
		variantName := strings.ToLower(strings.ReplaceAll(variant, "_", ""))
		fieldName := variantName + "0"
		return fmt.Sprintf("%s := *%s.%s", binding, elemVar, fieldName)
	}
}
```

**⚠️ EDGE CASE**: Custom enum field naming is ambiguous. See "Gaps & Edge Cases" section.

---

### 2. `pkg/plugin/builtin/result_type.go` ✅

**Status**: Already uses CamelCase naming!

**Evidence**:
- Tag constants: `ResultTagOk`, `ResultTagErr` (lines in generated code)
- Constructors: `ResultOk()`, `ResultErr()` (generated by plugin)
- Fields: `ok0`, `err0` (no underscores)

**Action Required**: **None** (verify no regression in tests)

---

### 3. `pkg/plugin/builtin/option_type.go` ✅

**Status**: Already uses CamelCase naming!

**Evidence**:
- Tag constants: `OptionTagSome`, `OptionTagNone`
- Constructors: `OptionSome()`, `OptionNone()`
- Fields: `some0` (no underscores)

**Action Required**: **None** (verify no regression in tests)

---

### 4. Golden Test Files (46 files)

**Files Requiring Regeneration**:

**Pattern Match Tests (Primary Impact)**:
- `pattern_match_01_basic.go.golden` - StatusTag_Pending → StatusTagPending
- `pattern_match_01_simple.go.golden` - Similar enum variants
- `pattern_match_02_guards.go.golden` - Guard expressions with enums
- `pattern_match_04_exhaustive.go.golden` - ColorTag_Red → ColorTagRed, rgb_r → rgbr
- `pattern_match_05_guards_basic.go.golden` - Status enums
- `pattern_match_12_tuple_exhaustiveness.go.golden` - Tuple patterns

**Result/Option Tests (Verify No Regression)**:
- `result_*.go.golden` (9 files) - Should already be correct
- `option_*.go.golden` (6 files) - Should already be correct

**Other Tests (Check for Enum Usage)**:
- `showcase_00_hero.go.golden` - May include enums
- `unqualified_import_*.go.golden` (4 files) - Check for enum imports

**Regeneration Strategy**:
```bash
# Run golden test suite - will auto-update .golden files
go test ./tests -run TestGoldenFiles -update

# Verify changes are correct
git diff tests/golden/*.go.golden
```

---

## Edge Cases & Ambiguities

### 1. Custom Enum Field Naming (CRITICAL)

**Problem**: How should multi-word variant fields be named?

**Example**:
```dingo
enum Color {
    RGB(r: int, g: int, b: int)
}
```

**Current Naming**:
```go
type Color struct {
    tag ColorTag
    rgb_r *int  // ❌ Underscore between variant and field
    rgb_g *int
    rgb_b *int
}
```

**Option 1: Concatenate All** (Simplest)
```go
type Color struct {
    tag ColorTag
    rgbr *int  // ✅ No underscores
    rgbg *int
    rgbb *int
}
```

**Option 2: CamelCase Variant + Lowercase Field**
```go
type Color struct {
    tag ColorTag
    rgbR *int  // ✅ CamelCase at variant boundary
    rgbG *int
    rgbB *int
}
```

**Option 3: Keep Field Names Separate**
```go
type Color struct {
    tag ColorTag
    r *int  // ✅ Just field name (no variant prefix)
    g *int
    b *int
}
```

**RECOMMENDATION**: Option 1 (all lowercase concatenation) for consistency with Result/Option pattern.

**USER DECISION REQUIRED**: Which field naming strategy?

---

### 2. Acronym Handling

**Problem**: How to handle acronyms in variant names?

**Example**:
```dingo
enum Network {
    HTTP_Status(code: int)
    TCP_Connection(port: int)
}
```

**Current**: `HTTPStatusTag_HTTP_Status` (confusing!)

**Options**:
1. Treat as regular words: `HttpstatusTagHttpstatus` (ugly but consistent)
2. Preserve acronyms: `HTTPStatusTagHTTPStatus` (idiomatic Go, but complex logic)
3. Initial caps only: `HttpStatusTagHttpStatus` (simple, readable)

**RECOMMENDATION**: Option 3 (initial caps) for simplicity.

**USER DECISION REQUIRED**: Acronym handling strategy?

---

### 3. Variant Name Collisions

**Problem**: CamelCase could create collisions.

**Example**:
```dingo
enum Value {
    Int_Value(v: int)     // IntValue
    IntValue(x: int)      // IntValue (collision!)
}
```

**Current**: `Value_Int_Value` vs `Value_IntValue` (no collision)

**Mitigation**:
1. Error on collision (compile-time check)
2. Add numeric suffix: `IntValue1`, `IntValue2`
3. Preserve single underscore for disambiguation: `IntValueValue` vs `IntValue`

**RECOMMENDATION**: Option 1 (error on collision) - forces explicit naming.

**USER DECISION REQUIRED**: Collision handling strategy?

---

### 4. Index Suffix Consistency

**Current Pattern**: All indexed fields use `_0` suffix
**Example**: `ok_0`, `err_0`, `int_0`, `some_0`

**Target Pattern**: Remove underscore → `ok0`, `err0`, `int0`, `some0`

**Question**: Why use numeric suffix at all?

**Analysis**:
- **Purpose**: Future-proof for tuple variants with multiple fields of same type
- **Example**: `Result(Ok(T1, T2))` → `ok0`, `ok1` fields
- **Current Reality**: No Dingo feature uses multi-field variants yet

**Options**:
1. Keep numeric suffix: `ok0`, `err0` (future-proof, consistent)
2. Remove for single-field: `ok`, `err` (cleaner, but breaking change later)

**RECOMMENDATION**: Keep numeric suffix (`ok0`) for future compatibility.

**USER CONFIRMATION**: Acceptable?

---

## Testing Strategy

### Phase 1: Unit Tests (Preprocessor)

**Target**: `pkg/preprocessor/rust_match_test.go`

**New Tests Required**:
1. `TestGetTagName_CamelCase` - Verify tag naming
   ```go
   testCases := []struct{
       input string
       want  string
   }{
       {"Status_Pending", "StatusTagPending"},  // Custom enum
       {"Ok", "ResultTagOk"},                   // Result
       {"Color_RGB", "ColorTagRGB"},            // Multi-word
   }
   ```

2. `TestGenerateBinding_CamelCase` - Verify field access
   ```go
   testCases := []struct{
       pattern  string
       wantField string
   }{
       {"Ok", "ok0"},
       {"Status_Pending", "pending0"},
       {"Color_RGB", "rgb0"},
   }
   ```

3. `TestEnumFieldNaming_EdgeCases` - Acronyms, collisions
   ```go
   // Test HTTP_Status → httpstatus0
   // Test collision detection
   ```

---

### Phase 2: Golden Tests

**Command**:
```bash
# Regenerate all golden files with new naming
go test ./tests -run TestGoldenFiles -update

# Verify compilation
go test ./tests -run TestGoldenFiles -v

# Check for unexpected changes
git diff tests/golden/*.go.golden
```

**Expected Changes**:
- `pattern_match_*.go.golden`: Tag and field names updated
- `result_*.go.golden`, `option_*.go.golden`: No changes (already CamelCase)

**Validation Checklist**:
- [ ] All golden tests compile without errors
- [ ] No underscore naming in generated code (except in identifiers like `__match_0`)
- [ ] Pattern match bindings resolve correctly
- [ ] Enum tag switches use correct constant names

---

### Phase 3: End-to-End Compilation

**Test Cases**:
1. Simple enum pattern match compiles
2. Multi-variant enum with guards compiles
3. Tuple patterns with enums compile
4. Result/Option interop unchanged

**Commands**:
```bash
# Build transpiler
go build ./cmd/dingo

# Transpile test cases
./dingo build tests/golden/pattern_match_01_basic.dingo

# Compile generated Go
go build tests/golden/pattern_match_01_basic.go

# Verify output matches golden
diff tests/golden/pattern_match_01_basic.go tests/golden/pattern_match_01_basic.go.golden
```

---

### Phase 4: Regression Prevention

**New Test**: Add test for naming convention consistency
```go
// tests/naming_convention_test.go
func TestNamingConvention_NoUnderscores(t *testing.T) {
    // Parse all generated .go files
    // Assert: No pattern like "Tag_" or "some_0" exists
    // Allow: __match_0 (internal temp vars)
}
```

---

## Migration Steps (Ordered)

### Step 1: Backup & Branch
```bash
git checkout -b feature/camelcase-enum-naming
git status  # Verify clean state
```

### Step 2: Update Code Generation

**File**: `pkg/preprocessor/rust_match.go`

1. Update `getTagName()` - Remove underscore from tag names
2. Update `generateBinding()` - Remove underscore from field names
3. Update `generateTupleBinding()` - Same as above
4. Run linter: `go vet ./pkg/preprocessor/`

### Step 3: Regenerate Golden Tests
```bash
go test ./tests -run TestGoldenFiles -update
git diff tests/golden/*.go.golden  # Review changes
```

### Step 4: Verify Compilation
```bash
# Run full test suite
go test ./tests -v

# Expected: All tests pass
# If failures: Debug field name mismatches
```

### Step 5: Manual Inspection

Review generated code for:
- [ ] Tag constants: `StatusTagPending` (not `StatusTag_Pending`)
- [ ] Constructors: `StatusPending()` (if applicable)
- [ ] Fields: `pending0` (not `pending_0`)
- [ ] No underscore artifacts

### Step 6: Documentation Update
```bash
# Update CHANGELOG.md
echo "### Code Generation
- Migrated enum naming to pure CamelCase (matches Go stdlib)
- Tag constants: StatusTagPending (was StatusTag_Pending)
- Field names: ok0, err0 (was ok_0, err_0)
- Generated code now passes golint/go vet" >> CHANGELOG.md
```

### Step 7: Commit & Review
```bash
git add pkg/preprocessor/rust_match.go
git add tests/golden/*.go.golden
git commit -m "feat: Migrate enum naming to CamelCase

- Remove underscores from tag constants (StatusTagPending)
- Remove underscores from field names (ok0, err0, pending0)
- Aligns with Go standard library conventions
- Passes golint and go vet without warnings
- All 46 golden tests regenerated and passing

Rationale: Unanimous recommendation from 6 AI models
(MiniMax M2, Grok Code Fast, GPT-5.1 Codex, Gemini 3 Pro, Sherlock, Sonnet)
Improves Go idiomaticity and developer experience.
"
```

---

## Success Criteria (Definition of Done)

### Code Quality
- [ ] No underscore separators in generated code (except `__match_N` temp vars)
- [ ] All tag constants use CamelCase: `ResultTagOk`, `StatusTagPending`
- [ ] All field names use camelCase+digit: `ok0`, `err0`, `pending0`
- [ ] Passes `go vet ./...` with zero warnings
- [ ] Passes `golint ./pkg/...` with zero warnings

### Test Coverage
- [ ] All 46 golden tests regenerated
- [ ] All golden tests pass compilation
- [ ] No new test failures introduced
- [ ] Pattern match tests verify correct tag/field names

### Consistency
- [ ] Result[T,E] naming unchanged (already CamelCase)
- [ ] Option[T] naming unchanged (already CamelCase)
- [ ] Custom enums now match Result/Option style
- [ ] No naming inconsistencies across enum types

### Documentation
- [ ] CHANGELOG.md updated with migration notes
- [ ] No user-facing Dingo syntax changes (only generated Go)
- [ ] Breaking change clearly documented (if applicable)

### Performance
- [ ] No compilation time regression
- [ ] Generated code size unchanged
- [ ] Runtime performance unchanged (naming has zero impact)

---

## Rollback Plan

**If issues discovered**:
1. Revert commit: `git revert HEAD`
2. Restore golden tests: `git checkout main tests/golden/*.go.golden`
3. Document issue in GitHub issue
4. Re-evaluate with user decisions on edge cases

**Rollback Triggers**:
- Compilation failures that can't be fixed in <1 hour
- More than 5 golden tests fail after regeneration
- User rejects edge case decisions after seeing implementation

---

## Risk Assessment

### Low Risk
- ✅ Result/Option types already use CamelCase (no change)
- ✅ Pattern is consistent across all enum types
- ✅ Automated golden test regeneration
- ✅ Go compiler catches field name errors immediately

### Medium Risk
- ⚠️ Custom enum field naming edge cases (RGB example)
- ⚠️ Acronym handling may need iteration
- ⚠️ Variant name collisions (rare but possible)

### Mitigation
- Get user decisions on edge cases BEFORE implementation
- Add unit tests for edge cases
- Manual review of all generated golden files
- Keep old tests as reference during migration

---

## Dependencies

**Internal**:
- `pkg/preprocessor/rust_match.go` (primary change)
- `pkg/plugin/builtin/result_type.go` (verify no regression)
- `pkg/plugin/builtin/option_type.go` (verify no regression)

**External**:
- None (pure code generation change)

**Backwards Compatibility**:
- **Breaking Change**: Yes, for generated Go code
- **Dingo Syntax**: No change (users don't see difference)
- **Mitigation**: Pre-v1.0, no compatibility promise

---

## Timeline

**Total Effort**: 3-4 hours

| Phase | Time | Activity |
|-------|------|----------|
| Planning | 30 min | Resolve edge case decisions with user |
| Implementation | 1 hour | Update `rust_match.go` functions |
| Testing | 1 hour | Regenerate goldens, run tests |
| Review | 30 min | Manual inspection of generated code |
| Documentation | 30 min | Update CHANGELOG, commit messages |
| Buffer | 30 min | Debug unexpected issues |

**Recommended Schedule**:
- Day 1: Get user decisions on edge cases
- Day 1: Implement changes + regenerate tests
- Day 1: Review + commit

---

## Conclusion

This migration brings Dingo's generated code into full alignment with Go standard library conventions. The change is **mechanical and low-risk**, with clear patterns and automated test validation.

**Key Success Factors**:
1. Resolve edge cases BEFORE implementation
2. Automated golden test regeneration
3. Compiler catches errors immediately
4. Pattern is simple: "remove underscores"

**Immediate Next Step**: Get user decisions on:
1. Custom enum field naming (RGB example)
2. Acronym handling strategy
3. Collision handling approach
