# Proposed Fix: Absolute ? Position in Mappings

## Exact Code Changes

**File**: `pkg/preprocessor/error_prop.go`

**In `expandAssignment`** (around line 328) and **symmetrically in `expandReturn`** (line 490):

```diff
-       qPos := strings.Index(expr, "?")
+       qPos := strings.Index(matches[0], "?")
        if qPos == -1 {
            qPos = 0 // fallback
        }
```

- `matches[0]` = full original line (e.g., `"data := ReadFile(path)?"`).
- `strings.Index(matches[0], "?")` = absolute byte offset from line start.
- `qPos + 1` = correct 1-based column of ?.

## No Other Changes
- `expr_mapping` already correct (uses `matches[0]`).
- All `error_prop` mappings updated automatically (use same `qPos + 1`).
- Sourcemap lookup unchanged (exact range match works).
- Markers (`dingo:s:1` etc.) unchanged.

## Validation
1. Regenerate golden `error_prop_01_simple.dingo` → `.go.golden` + sourcemap.
2. Check sourcemap JSON: error_prop `OriginalColumn` now absolute (e.g., 24 not 15).
3. LSP test: `dingo-lsp`, open file with bad `ReadFileXXX?` → underline under full expr + correct ? col.
4. Multi-line diags span correctly.

## Impact
- Fixes column shift for all expansion mappings.
- Expr errors → precise expr positions.
- Propagation errors → exact ? position.
- Zero runtime/perf change.
- Backward compatible (old sourcemaps invalid anyway on regen).