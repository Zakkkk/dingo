# Root Cause Analysis: LSP Source Mapping Only Maps ? Position

## Problem
LSP diagnostics for errors in error propagation expanded code (e.g., undefined ReadFile) map back only to the ? position in Dingo source, not the full expression position.

## Flow Trace
1. **Preprocessor** (`pkg/preprocessor/error_prop.go`): `expandAssignment`/`expandReturn` generates 7 lines from `x := expr?`.
   - Line 1: `__tmpN, __errN := exprClean`
   - Lines 2-6: markers + `if __errN != nil { return ... }`
   - Line 7: `var x = __tmpN`

2. **Mappings Generated**:
   - `expr_mapping`: Correct! Maps generated expr position (after vars prefix) back to original expr position using `strings.Index(matches[0], exprClean)`.
   - `error_prop` mappings (all 7 lines): **BUG** OriginalColumn = `qPos + 1` where `qPos = strings.Index(expr, "?")`.
     - `expr = msgPattern.FindStringSubmatch(rightSide)[1]` = `"ReadFile(path)?"` (rightSide after `=`).
     - qPos ~13 (position of ? relative to *after =*).
     - But real ? column ~24 (full line `"x := ReadFile(path)?"`).

3. **LSP Translation** (`pkg/lsp/handlers.go` `TranslateDiagnostics`):
   - Calls `TranslateRange(goURI, diag.Range, GoToDingo)`.
   - Uses sourcemap `MapToOriginal(genLine, genCol)`.

4. **Mapping Lookup** (`pkg/preprocessor/sourcemap.go` `MapToOriginal`):
   - Finds mapping where `genLine == GeneratedLine && genCol in [GeneratedColumn, GeneratedColumn+Length)`.
   - For expr errors (gen line1, col in expr): Hits `expr_mapping` → correct orig expr pos.
   - For expansion errors (gen lines2-7, col~1): Hits `error_prop` → wrong origCol (shifted left by prefix length ~10 cols).
   - Fallback: closest mapping if no exact, or identity if dist>10 → often wrong line/col.

## Why Only ? Position?
- gopls diagnostics for \"undefined ReadFile\" may span/underlie the *expansion block* (if/return), not just line1.
- These map via wrong `error_prop` origCol → appears at/near ? (but offset).
- Single-col ? mappings dominate line1 col1, shadowing if query col~1.
- Column mismatch due to spaces/prefix → fallback prefers ? mappings.

## Confirmation
Simulate:
- Full line: `data := os.ReadFile(path)?` (assume no indent/spaces, len(\"data := \")=10)
- exprClean=\"os.ReadFile(path)\", exprPosInOriginal=10, origCol_expr=11
- expr=\"os.ReadFile(path)?\", qPos=14 (len(os.ReadFile(path))=14?), origCol_?=15 (wrong! real ?=10+14=24, Index(full,\"?\")=23, col24)
- Mismatch: 15 vs 24 → 9 col shift.

Root cause confirmed: Relative vs absolute column calculation for error_prop mappings.