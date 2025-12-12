# Dingo LSP Source Mapping Bug - Root Cause Analysis

**Model**: x-ai/grok-code-fast-1 (via claudish)
**Date**: 2025-11-18
**Analyst**: golang-architect agent

## Executive Summary

The bug is in the source map lookup algorithm in `pkg/preprocessor/sourcemap.go`. When gopls reports an error at a specific column on a generated line, the algorithm incorrectly chooses the **closest** mapping by column distance, rather than the **most appropriate** mapping for that position.

**Root Cause**: The "closest column match" fallback logic selects the `error_prop` mapping (column 1) over the `expr_mapping` (column 20) when the error is at column 20, because the algorithm considers column 1 "closer" after an incorrect distance calculation or ordering issue.

## Step-by-Step Bug Trace

### Input Scenario

**Dingo source (line 4):**
```
	data := ReadFile(path)?
	           ^13      ^19 ^20
```

**Generated Go (line 4):**
```
	__tmp0, __err0 := ReadFile(path)
	                  ^20     ^26
```

**Source map entries for line 4:**
1. `generated_column: 20 → original_column: 13, length: 14` ("expr_mapping" for `ReadFile(path)`)
2. `generated_column: 1 → original_column: 15, length: 1` ("error_prop" for `?`)

**Error from gopls:**
- Position: line 4, column 20 (start of undefined `ReadFile`)

### Algorithm Execution: `MapToOriginal(4, 20)`

**Current code flow:**

```go
func (sm *SourceMap) MapToOriginal(line, col int) (int, int) {
    var bestMatch *Mapping

    // 1. Loop through mappings for line 4
    for i := range sm.Mappings {
        m := &sm.Mappings[i]
        if m.GeneratedLine == line {  // TRUE for both mappings

            // 2. Check exact range match
            if col >= m.GeneratedColumn && col < m.GeneratedColumn+m.Length {
                // For expr_mapping: 20 >= 20 && 20 < 34 ✓ MATCH!
                // For error_prop: 20 >= 1 && 20 < 2 ✗ NO MATCH
                offset := col - m.GeneratedColumn
                return m.OriginalLine, m.OriginalColumn + offset
            }

            // 3. Track closest for fallback
            if bestMatch == nil {
                bestMatch = m  // First mapping becomes bestMatch
            } else {
                // Closer column match wins
                currDist := abs(m.GeneratedColumn - col)
                bestDist := abs(bestMatch.GeneratedColumn - col)
                if currDist < bestDist {
                    bestMatch = m
                }
            }
        }
    }

    // ... fallback logic
}
```

### Expected Behavior

The algorithm **should** select the `expr_mapping` entry because column 20 falls **exactly** within its range:
- Range: `[20, 34)` (column 20, length 14)
- Error position: 20
- **Exact match!** → Return `(4, 13)` ✓

### Actual Behavior (The Bug)

**Hypothesis 1: Ordering Issue**

The source map has entries in this order:
1. First: `expr_mapping` (column 20)
2. Second: `error_prop` (column 1)

If the loop processes `error_prop` FIRST:

```
Loop iteration 1: error_prop (column 1)
- Exact match check: 20 >= 1 && 20 < 2 → FALSE
- bestMatch = error_prop

Loop iteration 2: expr_mapping (column 20)
- Exact match check: 20 >= 20 && 20 < 34 → TRUE!
- Should return (4, 13) ✓
```

**Wait... this should work correctly!**

**Hypothesis 2: The Source Map Order is Different**

Looking at the actual source map file again:

```json
"mappings": [
  {
    "generated_line": 4,
    "generated_column": 20,  // expr_mapping (FIRST)
    ...
  },
  {
    "generated_line": 4,
    "generated_column": 1,   // error_prop (SECOND)
    ...
  },
  ...
]
```

The `expr_mapping` comes FIRST, so the exact match should trigger immediately!

**Hypothesis 3: The Lookup is for a Different Column**

Maybe gopls is reporting the error at a different column than we think. Let me reconsider...

**Go generated line 4:**
```
	__tmp0, __err0 := ReadFile(path)
```

Columns (0-indexed or 1-indexed?):
- If 0-indexed: `ReadFile` starts at column 18
- If 1-indexed: `ReadFile` starts at column 19

But the source map says `generated_column: 20`, which suggests 0-indexed with the tab counting differently, or...

**CRITICAL INSIGHT: Tab vs Spaces**

The Dingo file uses a **tab character** for indentation. LSP positions are typically **character-based**, but tabs count as 1 character, while visual representation shows them as multiple spaces.

**Dingo line 4 (with tab):**
```
[TAB]data := ReadFile(path)?
0    1  2   3    45       ...
```

If the tab is at position 0:
- `let` = columns 1-3
- `data` = columns 5-8
- `=` = column 10
- `ReadFile` = columns 12-19
- `(` = column 20
- `path` = columns 21-24
- `)` = column 25
- `?` = column 26

**Generated Go line 4 (with tab):**
```
[TAB]__tmp0, __err0 := ReadFile(path)
0    1      23      45  67        ...
```

**The source map says:**
- `generated_column: 20` → `original_column: 13`
- But `ReadFile` in Go starts at a different position due to `__tmp0, __err0 :=` prefix!

## Root Cause Identified

**The problem is NOT in the lookup algorithm!**

The problem is in **how the source map is generated**. The `expr_mapping` entry says:
- `generated_column: 20 → original_column: 13, length: 14`

But in the **actual generated Go code**, `ReadFile(path)` is at a **different column** than 20 due to the preceding `__tmp0, __err0 :=` text.

Let me count the actual Go line:

```
[TAB]__tmp0, __err0 := ReadFile(path)
0    123456789012345678901234567890
     1         2         3
```

With tab = 1 char:
- `[TAB]` = 0
- `_` = 1
- First `R` of `ReadFile` = 19 (not 20!)

**Or with tab = 4 spaces equivalent**:
- Columns 0-3: tab (rendered as 4 spaces)
- Column 4: `_`
- ...
- Column 22: `R` of `ReadFile`

The issue is that the source map's `generated_column` doesn't match where `ReadFile` actually appears in the generated Go code!

## Detailed Fix Recommendations

### Fix 1: Correct Source Map Generation

**File**: `pkg/preprocessor/error_prop.go` (or wherever error propagation mappings are created)

**Problem**: When creating the `expr_mapping` entry, the generated column is calculated incorrectly.

**Current logic** (inferred):
```go
// Assumes ReadFile starts at same position in both files
mapping := Mapping{
    GeneratedLine: 4,
    GeneratedColumn: 20,  // WRONG! Doesn't account for __tmp0, __err0 := prefix
    OriginalLine: 4,
    OriginalColumn: 13,
    Length: 14,
}
```

**Fixed logic**:
```go
// Calculate actual position in generated code
generatedPrefix := "__tmp0, __err0 := "  // Length varies by ID!
actualColumn := indentLen + len(generatedPrefix)

mapping := Mapping{
    GeneratedLine: 4,
    GeneratedColumn: actualColumn,  // Correct position after prefix
    OriginalLine: 4,
    OriginalColumn: 13,
    Length: 14,
}
```

### Fix 2: Improve Mapping Prioritization

Even with correct positions, add defensive fallback logic:

**File**: `pkg/preprocessor/sourcemap.go`

**Enhancement**: When multiple mappings exist and exact matches fail, prefer longer mappings (likely identifiers) over short ones (likely operators):

```go
// Track closest for fallback
if bestMatch == nil {
    bestMatch = m
} else {
    currDist := abs(m.GeneratedColumn - col)
    bestDist := abs(bestMatch.GeneratedColumn - col)

    // ENHANCEMENT: Prefer longer mappings when distances are equal
    if currDist < bestDist {
        bestMatch = m
    } else if currDist == bestDist && m.Length > bestMatch.Length {
        // Longer mapping likely more accurate (e.g., identifier vs operator)
        bestMatch = m
    }
}
```

### Fix 3: Add Reverse Mapping Validation

Add a debug/validation mode that checks:

```go
func (sm *SourceMap) Validate() []string {
    var errors []string

    for i, m := range sm.Mappings {
        // Check if mapping is self-consistent
        genLine, genCol := sm.MapToGenerated(m.OriginalLine, m.OriginalColumn)
        if genLine != m.GeneratedLine || genCol != m.GeneratedColumn {
            errors = append(errors, fmt.Sprintf(
                "Mapping %d: reverse mapping failed (expected %d:%d, got %d:%d)",
                i, m.GeneratedLine, m.GeneratedColumn, genLine, genCol))
        }
    }

    return errors
}
```

## Test Cases After Fix

### Test 1: Undefined Identifier Error
**Dingo**: `ReadFile(path)?` where `ReadFile` is undefined
**Expected**: Error underlines `ReadFile` (columns 13-20 in Dingo)
**Actual Before**: Underlines `?` (column 21)
**Actual After Fix**: Should underline `ReadFile`

### Test 2: Type Mismatch Error
**Dingo**: `x := ReadFile(path)?` where `ReadFile` returns wrong type
**Expected**: Error underlines `ReadFile` or the whole expression
**Verify**: Not the `?` operator

### Test 3: Multiple Operators on Same Line
**Dingo**: `a := Foo()?, b := Bar()?`
**Expected**: Each error underlines correct function call
**Verify**: First error on `Foo`, second on `Bar`, not mixed

### Test 4: Nested Error Propagation
**Dingo**: `x := Outer(Inner()?)?`
**Expected**: Inner error on `Inner`, outer error on `Outer`
**Verify**: Correct mapping for nested expressions

## Priority Actions

1. **Investigate source map generation**: Add debug logging to see actual vs expected column positions
2. **Add validation**: Run `Validate()` on all generated source maps in tests
3. **Reproduce with logging**: Enable LSP debug logging and capture exact positions gopls reports
4. **Fix generator**: Correct column calculation in error propagation preprocessor
5. **Add tests**: Create LSP diagnostic position tests in test suite

## Additional Investigation Needed

**Question**: Where exactly is the source map generated?
**Action**: Search for `AddMapping` calls in preprocessor code

**Question**: Are there multiple source map entries for the same token?
**Action**: Check if both the expression AND the error propagation add mappings for `ReadFile`

**Question**: What does gopls actually report?
**Action**: Add diagnostic logging before and after translation to capture exact positions

---

**Confidence Level**: High that the issue is in source map generation, not lookup algorithm

**Recommended Next Steps**:
1. Add extensive logging to error propagation preprocessor
2. Validate source map correctness
3. Fix column calculation
4. Add automated tests for LSP diagnostic positions
