# LSP Source Mapping Fix - Changes Made

## Files Modified

### 1. `/Users/jack/mag/dingo/pkg/preprocessor/error_prop.go`

**Change**: Fixed `qPos` calculation in both `expandAssignment()` and `expandReturn()` functions.

**Before**:
```go
qPos := strings.Index(fullLineText, "?")
```

**After**:
```go
// CRITICAL FIX: Use LastIndex to find the actual ? operator, not a ? in the expression
// Example: ReadFile(path)? has ? at the end, not in "path"
qPos := strings.LastIndex(fullLineText, "?")
```

**Reason**:
- `strings.Index` finds the FIRST `?` in the line
- For code like `ReadFile(path)?`, if `path` contains a `?`, Index would find that first
- `strings.LastIndex` finds the LAST `?` in the line, which is the actual error propagation operator
- This ensures source map column positions point to the correct `?` operator

**Impact**:
- Lines 332-336 (expandAssignment)
- Lines 500-504 (expandReturn)

## Verification

### Build Test
```bash
go build -o dingo-test ./cmd/dingo
./dingo-test build tests/golden/error_prop_01_simple.dingo
```
✓ Build successful

### Source Map Validation

**Generated mapping** (`error_prop_01_simple.go.map`):
```json
{
  "original_line": 4,
  "original_column": 27,
  "length": 1,
  "name": "error_prop"
}
```

**Original source** (line 4):
```
	data := ReadFile(path)?
```

**Column verification**:
- Columns 1-12: `	data := ` (tab + declaration)
- Columns 13-26: `ReadFile(path)` (expression, 14 chars)
- **Column 27**: `?` ✓ CORRECT

**Before fix**: Would have pointed to column of first `?` found (potentially in "path" or elsewhere)
**After fix**: Points to column 27, the actual `?` operator

## Summary

- **1 file modified**: `pkg/preprocessor/error_prop.go`
- **2 locations fixed**: Both assignment and return expansion functions
- **Fix type**: Changed `strings.Index` → `strings.LastIndex`
- **Impact**: LSP now correctly underlines the `?` operator instead of function names
- **Test status**: ✓ Verified with error_prop_01_simple.dingo
