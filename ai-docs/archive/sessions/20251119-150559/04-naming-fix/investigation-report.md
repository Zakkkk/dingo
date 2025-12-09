# Result/Option Type Naming Issue Investigation

## Problem Summary

After fixing the match reprocessing bug, 6 golden tests fail due to **naming inconsistencies** in generated Result/Option types.

**Generated (Current - WRONG)**:
```go
type Resultinterror struct {  // ❌ Concatenated without separators
    tag  ResultTag
    ok0  *int                  // ❌ No underscore before digit
    err0 *error
}
```

**Expected (Golden Files - CORRECT)**:
```go
type Result_int_error struct {  // ✅ Underscored separators
    tag  ResultTag
    ok_0  *int                   // ✅ Underscore before digit
    err_0 *error
}
```

## Root Cause Analysis

### Issue 1: Type Name Generation (Missing Underscores)

**Location**: `pkg/plugin/builtin/result_type.go` and `pkg/plugin/builtin/option_type.go`

**Problem**: Inconsistent formatting patterns exist in the same file:

**Result Type (Lines 121, 124, 157, 175)**:
```go
resultType = fmt.Sprintf("Result%s%s",           // ❌ WRONG (no underscores)
    p.sanitizeTypeName(okType),
    p.sanitizeTypeName(errType))
```

**Result Type (Lines 240, 322)** - Constructor context:
```go
resultTypeName := fmt.Sprintf("Result_%s_%s",   // ✅ CORRECT (with underscores)
    p.sanitizeTypeName(okType),
    p.sanitizeTypeName(errType))
```

**Option Type (Lines 131, 186, 236)**:
```go
optionType := fmt.Sprintf("Option%s", p.sanitizeTypeName(typeName))  // ❌ WRONG
```

**Impact**: Type names in declaration differ from constructor context, causing:
- `Resultinterror` instead of `Result_int_error`
- `OptionInt` instead of `Option_int`

### Issue 2: Field Name Generation (Missing Underscores)

**Location**: `pkg/plugin/builtin/result_type.go` (lines 278, 360, 575, 584, 673, 676, etc.)

**Problem**: Field names use digits without separating underscores:

```go
Key: ast.NewIdent("ok0"),    // ❌ Line 278 - WRONG
Key: ast.NewIdent("err0"),   // ❌ Line 360 - WRONG
Name: "ok0",                 // ❌ Line 575 - WRONG
```

**Expected** (per comments on lines 25-26):
```go
//     ok_0   *T        // ✅ Correct format in comments
//     err_0  *E        // ✅ Correct format in comments
```

**Similar issue in option_type.go** (line 280, 336, etc.):
```go
Key: ast.NewIdent("some0"),  // ❌ WRONG
```

**Expected** (per comment on line 22):
```go
//     some_0  *T       // ✅ Correct format in comments
```

## Historical Context

**Git Analysis**:
- Commit `07ffa04` (2025-11-18): "Achieve 92.2% test passing - Fix critical bugs and regenerate golden files"
  - This commit regenerated golden files with **underscored format**
  - Golden files expect `Result_int_error`, `ok_0`, `err_0`

- Commit `2a76f92`: "Implement Variable Hoisting and eliminate comment pollution"
  - Pattern matching implementation

**No explicit "CamelCase migration" or deliberate naming change found in recent commits.**

**Conclusion**: The current code generation is **inconsistent** - it was likely a partial refactoring that was never completed. The golden files represent the **intended standard**.

## Golden File Standard

Checked all golden files in `tests/golden/*.go.golden`:

**Result Types** (9 files):
- `Result_int_error` (pattern_match_01_simple.go.golden)
- `Result_unknown_error` (result_01_basic.go.golden)
- Fields: `ok_0`, `err_0` (consistent across all files)

**Option Types** (6 files):
- `Option_int`, `Option_string`, `Option_float64` (option_*.go.golden)
- Fields: `some_0` (consistent across all files)

**100% consistency**: All golden files use **underscored naming**.

## Affected Tests

1. `pattern_match_01_simple` - Result[int, error], Option[string]
2. `pattern_match_04_exhaustive` - Custom enums with struct variants
3. `pattern_match_05_guards_basic` - Result types with guards
4. `pattern_match_06_guards_nested` - Nested patterns
5. `pattern_match_07_guards_complex` - Complex guard expressions
6. `pattern_match_08_guards_edge_cases` - Edge cases

Plus potentially ALL Result/Option tests if they use explicit type names.

## Recommendation: Option A (Fix Code)

**Approach**: Update code to match golden file standard (underscored names).

**Rationale**:
1. **Consistency**: 100% of golden files expect underscored format
2. **Readability**: `Result_int_error` is more readable than `Resultinterror`
3. **Go Conventions**: Underscores separate semantic parts in generated names
4. **Existing Standard**: Golden files already document this as the standard

**Changes Required**:
1. **result_type.go**: Fix 4 `fmt.Sprintf` calls to use `"Result_%s_%s"` format
2. **result_type.go**: Change all `"ok0"` → `"ok_0"` and `"err0"` → `"err_0"` (20+ locations)
3. **option_type.go**: Fix 3 `fmt.Sprintf` calls to use `"Option_%s"` format
4. **option_type.go**: Change all `"some0"` → `"some_0"` (10+ locations)

**Risk**: Low - this is purely cosmetic, doesn't affect logic or semantics.

## Files to Modify

1. `/Users/jack/mag/dingo/pkg/plugin/builtin/result_type.go` (40+ changes)
2. `/Users/jack/mag/dingo/pkg/plugin/builtin/option_type.go` (15+ changes)

## Expected Outcome

After fix:
- All 6 failing pattern match tests pass
- All existing Result/Option tests continue to pass
- Generated code matches golden file standard
- Type names become more readable and conventional
