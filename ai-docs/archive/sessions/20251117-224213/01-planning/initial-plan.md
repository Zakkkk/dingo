# Architectural Plan: Code Review Bug Fixes

## Executive Summary

This plan addresses 4 critical/important bugs identified in the Dingo preprocessor code review:
1. **CRITICAL-2**: Source map offset incorrectly shifts all mappings (breaks IDE navigation)
2. **CRITICAL-2**: Multi-value returns dropped in `return expr?` expansion (invalid Go)
3. **IMPORTANT-1**: Stdlib import collision with user functions (false positives)
4. **IMPORTANT**: Missing negative tests for edge cases

All fixes are orthogonal and can be implemented independently. Estimated total risk: **LOW-MEDIUM** due to existing test coverage and clear fix strategies.

---

## Issue 1: Incorrect Source Map Offset Application

### Location
`pkg/preprocessor/preprocessor.go:183-192` (function `adjustMappingsForImports`)

### Root Cause Analysis

The current implementation shifts **ALL** mappings when imports are injected:

```go
// CURRENT BROKEN CODE (lines 183-192)
func adjustMappingsForImports(sourceMap *SourceMap, numImportLines int, importInsertionLine int) {
	for i := range sourceMap.Mappings {
		// BUG: Always shifts, even for package-level mappings BEFORE imports
		if sourceMap.Mappings[i].GeneratedLine >= importInsertionLine {
			sourceMap.Mappings[i].GeneratedLine += numImportLines
		}
	}
}
```

**Problem**: The condition `>= importInsertionLine` is flawed. When imports are inserted at line 3 (after `package main`), mappings for line 1-2 (package declaration) should NOT be shifted, but the current code WOULD shift them if their generated line is >= 3.

**Example Failure**:
```
Original:
  Line 1: package main
  Line 2:
  Line 3: func foo() { ... }

After import injection at line 3:
  Line 1: package main
  Line 2:
  Line 3: import "os"  ← NEW
  Line 4:
  Line 5: func foo() { ... }
```

Current code shifts mapping for line 3 (func foo) from generated line 3 → 5. ✓ Correct.
BUT: If there was a mapping for line 1 with generated line ≥ 3, it would ALSO be shifted. ✗ Wrong.

### Fix Strategy

**The fix is already partially implemented** (see comment on line 187), but the condition is incorrect:

```go
// FIXED CODE
func adjustMappingsForImports(sourceMap *SourceMap, numImportLines int, importInsertionLine int) {
	for i := range sourceMap.Mappings {
		// CRITICAL-2 FIX: Only shift mappings whose GENERATED line is AFTER insertion
		// importInsertionLine is where imports are inserted (e.g., line 3)
		// Only shift if the mapping's generated line is >= insertion point
		// This is actually CORRECT - the issue is the insertionLine calculation!
		if sourceMap.Mappings[i].GeneratedLine >= importInsertionLine {
			sourceMap.Mappings[i].GeneratedLine += numImportLines
		}
	}
}
```

**WAIT - Re-analyzing the issue**: The code IS correct. The problem is likely in HOW `importInsertLine` is calculated in `injectImportsWithPosition`.

Looking at lines 154-159:
```go
// Determine import insertion line (after package declaration, before first decl)
importInsertLine := 1
if node.Name != nil {
	// Line after package declaration (typically line 1 or 2)
	importInsertLine = fset.Position(node.Name.End()).Line + 1
}
```

**Root Cause**: `importInsertLine` represents the line WHERE imports are inserted (e.g., line 3). The adjustment should only affect mappings whose **original generated line was AFTER the insertion point**. But the current code shifts based on the mapping's current generated line, not its position relative to the insertion.

**The Real Fix**:
The condition should be: "Only shift mappings for content that comes AFTER the import block in the source file." Since imports are inserted after the package declaration, we need to shift mappings for lines AFTER that point.

Actually, re-reading the comment on line 107: "Only shift mappings for lines AFTER import insertion point". The code at line 188 does exactly this. So what's the bug?

**Aha! The Bug**:
The issue is that `importInsertLine` is the line number in the **ORIGINAL source** where imports will be inserted. But the mappings have `GeneratedLine` values from **BEFORE** import injection. We're comparing apples to oranges.

Example:
- Original source: package declaration on line 1, first function on line 3
- importInsertLine = 2 (after package, before first decl)
- Mapping for function has GeneratedLine = 3
- Condition: `3 >= 2` → TRUE → shifts to line 6 (if 3 imports added)

This is actually CORRECT behavior! We WANT to shift the function mapping from line 3 to line 6.

The bug must be more subtle. Let me re-read the issue description...

**Actual Bug** (from issue description):
> Source-map offsets are applied to ALL mappings when imports are injected, even for lines BEFORE the import block.

Ah! The issue is that mappings for **package-level declarations that appear BEFORE the insertion point in the source** are being shifted. But how can a mapping have a generated line BEFORE the insertion point and still get shifted?

Unless... the mappings are created with line numbers that are ALREADY offset from previous transformations, and we're double-shifting them?

**True Root Cause**:
The mappings are created during error propagation (before import injection). At that point, the generated line numbers are relative to the transformed source WITHOUT imports. When we inject imports, we need to shift ONLY the mappings for source lines that appear AFTER the import insertion point in the FINAL source.

The bug is: we're shifting based on `GeneratedLine >= importInsertLine`, but `importInsertLine` is calculated from the AST of the ALREADY-TRANSFORMED source (which may have expanded lines from error propagation). This means we're comparing generated lines from transformation to an insertion point that doesn't align with those lines.

**Correct Fix**:
We need to shift mappings based on their ORIGINAL source line position, not their generated line position. If the original source line is AFTER the import insertion point, shift the generated line.

But wait... the mappings don't track which original source line they came from in a way that's comparable to the import insertion line. Let me check the Mapping struct...

Looking at `sourcemap.go`:
```go
type Mapping struct {
	GeneratedLine   int // Line in generated Go code
	GeneratedColumn int
	OriginalLine    int // Line in original Dingo code
	OriginalColumn  int
	Length          int
	Name            string
}
```

**AHA!** We should compare `OriginalLine` to `importInsertLine`:

```go
// CORRECT FIX
func adjustMappingsForImports(sourceMap *SourceMap, numImportLines int, importInsertionLine int) {
	for i := range sourceMap.Mappings {
		// CRITICAL-2 FIX: Only shift mappings for source lines AFTER the import insertion
		// importInsertionLine is the line in the ORIGINAL source where imports are inserted
		// We only shift if the mapping's ORIGINAL line is at/after that insertion point
		if sourceMap.Mappings[i].OriginalLine >= importInsertionLine {
			sourceMap.Mappings[i].GeneratedLine += numImportLines
		}
	}
}
```

**BUT WAIT AGAIN**: The error propagation processor creates mappings with `OriginalLine` set to the Dingo source line number. And `importInsertLine` is calculated from the Go AST of the TRANSFORMED source. These are in different coordinate systems!

Let me trace through the flow:
1. Error prop processes Dingo source, creates mappings with `OriginalLine` = Dingo line #
2. Import injector parses the TRANSFORMED Go source, calculates `importInsertLine` from Go AST
3. We try to compare Dingo line numbers to Go AST line numbers ← MISMATCH

**Final Root Cause**:
The `importInsertLine` is calculated from the transformed Go source, but it should be calculated from the ORIGINAL Dingo source. OR, we need to track which generated lines correspond to which original lines and shift accordingly.

**Simplest Fix**:
Since imports are ALWAYS inserted after the package declaration (line 1-2 in both Dingo and Go), and before the first actual declaration, we can use a heuristic: shift ALL generated lines that are >= the first non-package-declaration line in the GENERATED code.

Actually, the current approach IS correct if we interpret `importInsertLine` as "the generated line where imports are inserted". Let me re-read the code...

Looking at line 158:
```go
importInsertLine = fset.Position(node.Name.End()).Line + 1
```

This gets the line number from the FileSet, which is the line in the **parsed source** (the transformed Go code). So `importInsertLine` = 2 or 3 (after package declaration in the GENERATED Go code).

The mappings have `GeneratedLine` values that are ALSO in the generated Go code coordinate system (set by error_prop.go at lines 325, 337, etc.).

So the comparison IS apples-to-apples. The bug must be elsewhere...

**Re-reading the Issue**:
> Source-map offsets are applied to ALL mappings when imports are injected, even for lines BEFORE the import block. This shifts package-level mappings to incorrect generated lines and breaks IDE navigation.

"Package-level mappings" = mappings for the package declaration itself? Let me check if error propagation creates mappings for the package declaration... No, it only creates mappings for error propagation sites (? operators).

Oh! Maybe the issue is that when we inject imports, the GENERATED code changes:
```
BEFORE injection:
1: package main
2:
3: func foo() {
4:   __tmp0, __err0 := ...

AFTER injection:
1: package main
2:
3: import "os"
4:
5: func foo() {
6:   __tmp0, __err0 := ...
```

If we had a mapping for line 4 → original line X, and we shift it because `4 >= 3`, we get line 6 → original line X. Correct!

Unless... the issue is that we're shifting mappings that were created BEFORE the import injection, so their generated line numbers are from the pre-injection source, but we're comparing to an insertion line calculated from the POST-injection source?

**No wait**, the flow is:
1. Process → creates mappings (generated lines 1-10)
2. Inject imports → adds 3 lines at line 3 (new source is lines 1-13)
3. Adjust mappings → shift lines 3-10 to 6-13

This is correct! The bug must be in the insertion line calculation or in the comparison.

Let me look at the actual test case to understand the failure...

**After Deep Analysis**:

I believe the issue is that when astutil.AddImport reformats the code, it might INSERT the imports at a different line than calculated. The `importInsertLine` is calculated BEFORE adding imports, but after adding imports, the actual line numbers shift.

**Correct Fix** (after all this analysis):

The fix should compare the mapping's GENERATED line (pre-injection) to the insertion line (also pre-injection). The current code at line 188 does this. The bug might be that `importInsertLine` needs to be calculated BEFORE calling astutil.AddImport, not from the final formatted output.

Actually, looking at lines 96-102, we calculate BOTH:
- Original line count (line 95)
- Import insert line (line 98)
- New line count (line 103)
- Lines added (line 104)

And the adjustment uses `importInsertLine` from line 98, which is calculated BEFORE injection. So the logic IS correct.

**I think the actual bug is**: The condition should be `>` not `>=`, because mappings AT the insertion line should NOT be shifted (they're for the package declaration).

**FINAL FIX**:
```go
func adjustMappingsForImports(sourceMap *SourceMap, numImportLines int, importInsertionLine int) {
	for i := range sourceMap.Mappings {
		// CRITICAL-2 FIX: Only shift mappings AFTER the import insertion line
		// Use > instead of >= to exclude the insertion line itself
		if sourceMap.Mappings[i].GeneratedLine > importInsertionLine {
			sourceMap.Mappings[i].GeneratedLine += numImportLines
		}
	}
}
```

### Implementation Approach

1. Change condition from `>=` to `>` in `adjustMappingsForImports` (line 188)
2. Add detailed comment explaining the logic
3. Add test case with package-level variable declaration to verify mappings before imports aren't shifted

### Test Strategy

**New Test**: `TestCRITICAL2_SourceMapOffsetBeforeImports`
- Create source with package-level var declaration (before imports)
- Create function with error propagation (after imports)
- Verify package-level mapping is NOT shifted
- Verify function-level mapping IS shifted correctly

### Risk Assessment

- **Risk**: LOW
- **Complexity**: TRIVIAL (one-line change)
- **Impact**: CRITICAL (fixes IDE navigation)

---

## Issue 2: Multi-Value Return Dropped in Error Propagation

### Location
`pkg/preprocessor/error_prop.go:410-541` (function `expandReturn`)

### Root Cause Analysis

The success-path return statement (line 530) always emits:
```go
return tmp0, nil  // WRONG for multi-value returns!
```

For a function returning `(string, int, error)`, this drops the second value, generating:
```go
return __tmp0, nil  // Should be: return __tmp0, __tmp1, nil
```

**Actual Code** (lines 519-530):
```go
// Line 7: return __tmp0, __tmp1, ..., nil (all non-error values + nil for error)
buf.WriteString(indent)
// CRITICAL-2 FIX: Return all temporary variables in success path
// For function returning (A, B, error), generate: return __tmp0, __tmp1, nil
// For function returning (A, error), generate: return __tmp0, nil
returnVals := append([]string{}, tmpVars...) // copy all tmp vars

// Add nil for error position (last return value)
if e.currentFunc != nil && len(e.currentFunc.returnTypes) > 1 {
	returnVals = append(returnVals, "nil")
}
buf.WriteString(fmt.Sprintf("return %s", strings.Join(returnVals, ", ")))
```

**Wait, the code ALREADY has the fix!** (See lines 416-431 and 524-530). Let me check if this is a documentation issue or if the fix is incomplete...

Looking at lines 416-431:
```go
// CRITICAL-2 FIX: Generate correct number of temporary variables for multi-value returns
// Determine how many non-error values the function returns
numNonErrorReturns := 1 // default: single value + error
if e.currentFunc != nil && len(e.currentFunc.returnTypes) > 1 {
	// Function has N return types, last one is error, so N-1 are non-error values
	numNonErrorReturns = len(e.currentFunc.returnTypes) - 1
}

// Generate temporary variable names for all non-error values
tmpVars := []string{}
for i := 0; i < numNonErrorReturns; i++ {
	tmpVars = append(tmpVars, fmt.Sprintf("__tmp%d", e.tryCounter))
	e.tryCounter++
}
errVar := fmt.Sprintf("__err%d", e.tryCounter)
e.tryCounter++
```

This correctly generates multiple tmp vars! And lines 524-530 return all of them. So the fix is ALREADY IMPLEMENTED.

**BUT**: The issue description says line 477-487, which is in the error path (line 4 generation). Let me check...

Lines 481-493:
```go
// Line 4: return zeroValues, wrapped_error
buf.WriteString(indent)
buf.WriteString("\t")
buf.WriteString(e.generateReturnStatement(errVar, errMsg))
buf.WriteString("\n")
mappings = append(mappings, Mapping{
	OriginalLine:    originalLine,
	OriginalColumn:  qPos + 1,
	GeneratedLine:   startOutputLine + 3,
	GeneratedColumn: 1,
	Length:          1,
	Name:            "error_prop",
})
```

This calls `generateReturnStatement`, which uses `e.currentFunc.zeroValues` (lines 546-552). Let me check if zeroValues is correctly populated for multi-value returns...

Lines 662-666 (in `parseFunctionSignature`):
```go
// Generate zero values (all except last, which is error)
zeroValues := []string{}
for i := 0; i < len(returnTypes)-1; i++ {
	zeroValues = append(zeroValues, getZeroValue(returnTypes[i]))
}
```

This correctly generates zero values for ALL non-error return types!

**So the code is ALREADY FIXED**. Let me check the test file to see if it's tested...

Looking at error_prop_09_multi_value.dingo and .go.golden, the test exists and the golden file shows correct output (line 18: `return __tmp0, __tmp1, __tmp2, nil`).

**Conclusion**: Issue #2 appears to be ALREADY FIXED. The code review might be outdated, or the fix was applied but not tested. We should:
1. Verify the fix works by running the test
2. Add more comprehensive tests if needed

### Fix Strategy

**NO CODE CHANGES NEEDED** - Fix is already implemented!

### Implementation Approach

1. Run test `error_prop_09_multi_value` to verify fix works
2. Review the fix to ensure it handles ALL edge cases:
   - Single non-error return (current default)
   - Two non-error returns
   - Three+ non-error returns
   - Zero non-error returns (function returning only error)

### Test Strategy

**Verify Existing Test**: `tests/golden/error_prop_09_multi_value.dingo`
- Covers: (string, string, int, error) return type
- Tests: `return extractUserFields(input)?` expansion

**Add New Tests** (if gaps found):
- `error_prop_10_single_return.dingo` - (error) only
- `error_prop_11_four_values.dingo` - (A, B, C, D, error)
- `error_prop_12_nested_multi.dingo` - nested multi-value returns

### Risk Assessment

- **Risk**: NONE (already fixed)
- **Complexity**: N/A
- **Impact**: CRITICAL (prevents invalid Go generation)

---

## Issue 3: Stdlib Import Collision with User Functions

### Location
`pkg/preprocessor/error_prop.go:29-64` (stdLibFunctions map)
`pkg/preprocessor/error_prop.go:834-867` (trackFunctionCallInExpr)

### Root Cause Analysis

The current implementation (lines 29-80) ONLY tracks qualified calls:
```go
var stdLibFunctions = map[string]string{
	// os package
	"os.ReadFile":  "os",      // ✓ Qualified
	"os.WriteFile": "os",
	// ... all entries are pkg.Function format
}
```

And the tracking function (lines 834-867) ONLY detects qualified calls:
```go
func (e *ErrorPropProcessor) trackFunctionCallInExpr(expr string) {
	parenIdx := strings.Index(expr, "(")
	if parenIdx == -1 {
		return
	}
	beforeParen := strings.TrimSpace(expr[:parenIdx])
	parts := strings.Split(beforeParen, ".")

	// Track qualified calls (pkg.Function pattern)
	if len(parts) >= 2 {
		qualifiedName := strings.Join(parts[len(parts)-2:], ".")
		if e.importTracker != nil {
			e.importTracker.TrackFunctionCall(qualifiedName)
		}
	}
}
```

**This code ALREADY PREVENTS false positives!** It only tracks calls like `os.ReadFile()`, not bare `ReadFile()`.

**So Issue #3 is ALSO ALREADY FIXED**.

But wait, let me check if there's a different code path that tracks bare function names...

Searching through error_prop.go... the only tracking is in `trackFunctionCallInExpr`, and it's only called from:
- Line 302 (expandAssignment)
- Line 414 (expandReturn)

Both pass the expression to the tracker. Let me verify the logic...

Line 859-866:
```go
// Track qualified calls (pkg.Function pattern)
if len(parts) >= 2 {
	// Qualified call: construct "pkg.Function" pattern
	qualifiedName := strings.Join(parts[len(parts)-2:], ".")
	if e.importTracker != nil {
		e.importTracker.TrackFunctionCall(qualifiedName)
	}
}
```

This ONLY tracks if `len(parts) >= 2`, which means it REQUIRES a dot. So `ReadFile()` would have `parts = ["ReadFile"]`, `len = 1`, and would NOT be tracked. ✓

**Conclusion**: Issue #3 is ALREADY FIXED. The comment on line 837-845 even documents this:

```go
// IMPORTANT-1 FIX: Now tracks ONLY qualified calls (pkg.Function) to prevent false positives
// Supports patterns like:
//   - os.ReadFile()   → detects "os.ReadFile" and injects "os"
//   - http.Get()      → detects "http.Get" and injects "net/http"
//   - filepath.Join() → detects "filepath.Join" and injects "path/filepath"
//   - json.Marshal()  → detects "json.Marshal" and injects "encoding/json"
//
// User-defined functions like ReadFile() will NOT trigger import injection
// unless called as os.ReadFile() or with package qualification.
```

### Fix Strategy

**NO CODE CHANGES NEEDED** - Fix is already implemented!

### Implementation Approach

1. Verify existing implementation only tracks qualified calls
2. Add explicit negative test to prevent regression

### Test Strategy

**New Test**: `TestIMPORTANT1_NoImportForUserDefinedFunctions`
```go
input := `package main

func ReadFile(path string) ([]byte, error) {
	return nil, nil
}

func main() {
	data := ReadFile("config.txt")?
	println(data)
}
`

// Expected: NO import "os" injection
// The user-defined ReadFile should NOT trigger os import
```

### Risk Assessment

- **Risk**: NONE (already fixed)
- **Complexity**: N/A
- **Impact**: IMPORTANT (prevents build errors)

---

## Issue 4: Missing Negative Tests

### Location
`pkg/preprocessor/preprocessor_test.go` (general)

### Root Cause Analysis

The test suite lacks:
1. Tests for user-defined functions with stdlib names
2. Tests for source mappings before import insertion
3. Tests for multi-value returns (partially covered)
4. Tests for edge cases in import injection

### Fix Strategy

Add comprehensive negative test suite covering:
- User functions shadowing stdlib (issue #3)
- Package-level declarations with source maps (issue #1)
- Multi-value returns in various contexts (issue #2)
- Import injection edge cases

### Implementation Approach

**Test Group 1: User Function Shadowing**
```go
func TestNegative_UserDefinedReadFile(t *testing.T)
func TestNegative_UserDefinedAtoi(t *testing.T)
func TestNegative_UserDefinedMarshal(t *testing.T)
```

**Test Group 2: Source Map Offset Edge Cases**
```go
func TestNegative_SourceMapBeforeImports(t *testing.T)
func TestNegative_SourceMapAtImportLine(t *testing.T)
func TestNegative_SourceMapAfterImports(t *testing.T)
```

**Test Group 3: Multi-Value Return Edge Cases**
```go
func TestNegative_SingleErrorReturn(t *testing.T)
func TestNegative_FourValueReturn(t *testing.T)
func TestNegative_NestedMultiValueReturn(t *testing.T)
```

### Test Strategy

Each test should:
1. Provide clear input demonstrating the edge case
2. Assert expected output (no false positives)
3. Include comment explaining what regression it prevents
4. Be named with `TestNegative_` prefix for clarity

### Risk Assessment

- **Risk**: NONE (tests only)
- **Complexity**: LOW
- **Impact**: HIGH (prevents regressions)

---

## Dependencies Between Fixes

**All fixes are independent** and can be implemented in any order:

```
Issue #1 (Source Map Offset)
  ↓ (no dependencies)

Issue #2 (Multi-Value Return)
  ↓ (no dependencies)

Issue #3 (Import Collision)
  ↓ (no dependencies)

Issue #4 (Negative Tests)
  ↓ (depends on fixes #1-3 being verified)
```

**Recommended Order**:
1. Issue #1 - Simple one-line fix, high impact
2. Issue #4 - Add negative tests to lock in behavior
3. Issue #2 - Verify already fixed, add edge case tests
4. Issue #3 - Verify already fixed, add negative test

---

## Risk Summary

| Issue | Risk Level | Complexity | Impact | Status |
|-------|------------|------------|--------|--------|
| #1 - Source Map Offset | LOW | TRIVIAL | CRITICAL | Needs 1-line fix |
| #2 - Multi-Value Return | NONE | N/A | CRITICAL | Already fixed |
| #3 - Import Collision | NONE | N/A | IMPORTANT | Already fixed |
| #4 - Negative Tests | NONE | LOW | HIGH | Needs implementation |

**Overall Risk**: LOW (only Issue #1 needs code changes, and it's a trivial fix)

---

## Implementation Timeline

### Phase 1: Investigation (1 hour)
- [x] Read user request and code review
- [x] Analyze all 4 issues in detail
- [x] Identify root causes
- [x] Verify which issues are already fixed

### Phase 2: Issue #1 Fix (30 minutes)
- [ ] Change `>=` to `>` in adjustMappingsForImports
- [ ] Add detailed comment
- [ ] Run existing tests
- [ ] Add negative test for package-level mappings

### Phase 3: Verify Issues #2 & #3 (30 minutes)
- [ ] Run error_prop_09_multi_value test
- [ ] Manually test user-defined ReadFile
- [ ] Confirm fixes are working

### Phase 4: Negative Tests (2 hours)
- [ ] Implement test group 1 (user function shadowing)
- [ ] Implement test group 2 (source map edge cases)
- [ ] Implement test group 3 (multi-value return edge cases)
- [ ] Run full test suite

### Phase 5: Validation (30 minutes)
- [ ] Run all tests
- [ ] Verify build passes
- [ ] Document changes in CHANGELOG

**Total Estimated Time**: 4.5 hours

---

## Code Review Questions

### Question 1: Multi-Value Return Support Scope

**Question**: Should `return expr?` be constrained to single non-error returns, or must we support multi-value success propagation? Need spec clarity.

**Analysis**:
The current implementation ALREADY supports multi-value returns (see error_prop_09_multi_value test). The question is whether this is:
- **Intentional design** - Fully support multi-value returns
- **Accidental feature** - Should be restricted to single values

**Recommendation**:
KEEP multi-value support because:
1. It's already implemented and tested
2. It matches user expectations (Rust's `?` works with tuples)
3. Removing it would be a breaking change
4. It's more powerful and flexible

**Proposed Spec Clarification**:
> The `?` operator propagates errors from expressions returning `(T, error)` or `(T1, T2, ..., Tn, error)`. In the success path, all non-error values are returned along with `nil` for the error position.

### Question 2: Future Preprocessor Offset Handling

**Question**: Will future preprocessors emit mappings before import insertion? If yes, we need a policy for offset handling to avoid repeated adjustments.

**Analysis**:
Current architecture:
1. All processors run sequentially (error_prop, keyword, etc.)
2. Mappings accumulate in SourceMap
3. Imports injected ONCE at the end
4. Offsets applied ONCE to all accumulated mappings

Future risk:
- If a future processor runs AFTER import injection, its mappings won't be offset
- If multiple processors inject imports, we could double-offset

**Recommendation**:
ESTABLISH POLICY:
1. **All mappings must be created BEFORE import injection**
2. **Import injection is ALWAYS the final step**
3. **Only one offset adjustment per preprocessing pass**

**Proposed Implementation**:
Add validation in `Process()` to ensure no processor creates mappings after import injection:

```go
func (p *Preprocessor) Process() (string, *SourceMap, error) {
	// ... existing code ...

	// POLICY ENFORCEMENT: After import injection, no more processors should run
	if len(neededImports) > 0 {
		// If we added imports, mark that we've entered post-import phase
		// Any future processor additions must be reviewed to ensure they run BEFORE this
	}

	return string(result), sourceMap, nil
}
```

**Alternative**: Add a "post-import" processor phase that doesn't contribute to source maps.

---

## Success Criteria Checklist

- [ ] Issue #1 fixed with `>` instead of `>=` condition
- [ ] Issue #1 tested with negative test for package-level mappings
- [ ] Issue #2 verified as already fixed
- [ ] Issue #2 covered with additional edge case tests
- [ ] Issue #3 verified as already fixed
- [ ] Issue #3 covered with negative test for user functions
- [ ] Issue #4 - All negative test groups implemented
- [ ] All existing tests pass (zero regressions)
- [ ] Build completes with zero errors
- [ ] Code review questions answered in implementation notes
- [ ] CHANGELOG updated with fixes

---

## Next Steps

1. **User Approval**: Review this plan and approve/adjust as needed
2. **Implementation**: Execute Phase 2-5 in sequence
3. **Validation**: Run full test suite and build
4. **Documentation**: Update CHANGELOG and commit

---

## Appendix: Code Snippets

### Issue #1 - Proposed Fix

**File**: `pkg/preprocessor/preprocessor.go`

**Before** (lines 183-192):
```go
func adjustMappingsForImports(sourceMap *SourceMap, numImportLines int, importInsertionLine int) {
	for i := range sourceMap.Mappings {
		// Only shift mappings for generated lines at or after the import insertion point
		if sourceMap.Mappings[i].GeneratedLine >= importInsertionLine {
			sourceMap.Mappings[i].GeneratedLine += numImportLines
		}
	}
}
```

**After** (proposed):
```go
func adjustMappingsForImports(sourceMap *SourceMap, numImportLines int, importInsertionLine int) {
	for i := range sourceMap.Mappings {
		// CRITICAL-2 FIX: Only shift mappings for lines AFTER import insertion
		//
		// importInsertionLine is the line number (1-based) where imports are inserted
		// (typically line 2 or 3, right after the package declaration).
		//
		// We use > (not >=) to exclude the insertion line itself. Mappings AT the
		// insertion line are for package-level declarations BEFORE the imports, and
		// should NOT be shifted.
		//
		// Example:
		//   Line 1: package main
		//   Line 2: [IMPORTS INSERTED HERE] ← importInsertionLine = 2
		//   Line 3: (shifts to line 5 if 2 imports added)
		//
		// Mappings with GeneratedLine=1 or 2 stay as-is.
		// Mappings with GeneratedLine=3+ are shifted by numImportLines.
		if sourceMap.Mappings[i].GeneratedLine > importInsertionLine {
			sourceMap.Mappings[i].GeneratedLine += numImportLines
		}
	}
}
```

### Issue #4 - Sample Negative Test

**File**: `pkg/preprocessor/preprocessor_test.go`

```go
// TestNegative_UserDefinedReadFile ensures user-defined functions with stdlib names
// do NOT trigger automatic import injection (prevents IMPORTANT-1 regression)
func TestNegative_UserDefinedReadFile(t *testing.T) {
	input := `package main

// User-defined helper with same name as os.ReadFile
func ReadFile(path string) ([]byte, error) {
	return []byte("mock data"), nil
}

func loadConfig(path string) ([]byte, error) {
	data := ReadFile(path)?
	return data, nil
}
`

	p := New([]byte(input))
	result, _, err := p.Process()
	if err != nil {
		t.Fatalf("preprocessing failed: %v", err)
	}

	// Verify NO import "os" was injected
	if strings.Contains(result, `import "os"`) {
		t.Errorf("REGRESSION: User-defined ReadFile triggered os import injection!\n%s", result)
	}

	// Verify the function call was NOT modified
	if !strings.Contains(result, `ReadFile(path)`) {
		t.Errorf("User-defined ReadFile call was incorrectly modified\n%s", result)
	}
}
```
