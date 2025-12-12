
[claudish] Model: openrouter/sherlock-think-alpha

# LSP Source Mapping Bug Investigation - Complete Analysis

## 1. Root Cause
**0-based vs 1-based indexing mismatch** between LSP protocol (0-based lines/characters) and SourceMap logic (1-based lines/columns from Go `token.Position`).

- **gopls diagnostic**: For `ReadFile` undefined error → `Range{Start: {Line:3, Character:19}, End: {Line:3, Character:27}}` (0-based, UTF-16 units).
- **MapToOriginal(3,19)**: `m.GeneratedLine == 3`? No (mappings on line **4**). No exact match → fallback `bestMatch` scans same-line only (empty) → identity return `(3,19)`.
- **Observed symptom** ("?" underlined): Fallback "closest" picks `error_prop` mapping (gen col=1, orig col=15 for `?`), dist=18, offsets to wrong pos. Identity snaps to nearby.

**Exact failure**: Line mismatch skips all mappings → wrong fallback.

## 2. Detailed Analysis
**Flow trace** (error_prop_01_simple):
- Dingo line 4: `data := ReadFile(path)?` (orig col13="R", col15="?").
- Gen Go line 4: `__tmp0, __err0 := ReadFile(path)` (col20="R" after `__tmp0, __err0 := ` ~9 chars + indent1=20).
- Mappings:
  | Type | Gen L/C | Orig L/C | Len | Covers |
  |------|---------|----------|-----|--------|
  | expr | 4/20 | 4/13 | 14 | ReadFile(path) |
  | error_prop | 4/1 | 4/15 | 1 | ? expansion (lines 4-10) |

1. `handlePublishDiagnostics` → `TranslateDiagnostics` → `TranslateRange(goURI, Range{3,19-27}, GoToDingo)`.
2. `TranslatePosition(3,19)` → `MapToOriginal(3,19)`:
   - Loop: `if 4==3`? Skip all.
   - `bestMatch=nil` (no same-line mappings).
   - `dist>10`? No best → `return 3,19` (identity).
3. IDE: Underlines Dingo ~line4 col20 (`path)` or fuzzy-snaps to `?` via partial offset.

**Breakdown point**: No **line normalization** in translator → raw 0-based into 1-based logic.

## 3. Fix Design
**Normalize LSP positions to 1-based** before mapping; enhance fallback to prefer longer mappings.

### Code Changes
**A. pkg/lsp/translator.go** (or handlers.go Translate* funcs):
```go
// lspToGoPos normalizes LSP 0-based → Go 1-based
func lspToGoPos(p protocol.Position) (int, int) {
    return int(p.Line)+1, int(p.Character)  // uint32→int, +1 line/col
}

// In TranslatePosition/Range:
goLine, goCol := lspToGoPos(lspPos)
origLine, origCol := sm.MapToOriginal(goLine, goCol)
return protocol.Position{Line: uint32(origLine-1), Character: uint32(origCol-1)}
```

**B. pkg/preprocessor/sourcemap.go** (robustness):
```go
func (sm *SourceMap) MapToOriginal(line, col int) (int, int) {
    // Normalize: LSP may pass 0-based
    if line < 1 { line++ }
    if col < 0 { col = 0 }

    var exact *Mapping
    var longest *Mapping  // New: prefer len>1 (expr over op)

    for _, m := range sm.Mappings {
        if m.GeneratedLine == line {
            if col >= m.GeneratedColumn && col < m.GeneratedColumn+m.Length {
                return m.OriginalLine, m.OriginalColumn + (col - m.GeneratedColumn)
            }
            if col == m.GeneratedColumn {
                exact = m  // Start-of-mapping
            }
            // Prefer longer (expr_mapping len=14 > error_prop=1)
            if longest == nil || m.Length > longest.Length {
                longest = m
            }
        }
    }
    if exact != nil { return exact.OriginalLine, exact.OriginalColumn }
    if longest != nil && abs(longest.GeneratedColumn-col) <= 5 {
        return longest.OriginalLine, longest.OriginalColumn + (col - longest.GeneratedColumn)
    }
    return line, col  // Identity
}
```

**Edge cases**:
- UTF-16 vs bytes: Go identifiers ASCII → ok; add `utf16RuneCount` if needed.
- No mappings: Identity ok.
- Multi-line diags: Range start/end both normalized.
- Import shifts: Already handled in `adjustMappingsForImports`.

## 4. Validation Strategy
1. **Unit Tests** (pkg/preprocessor/sourcemap_test.go):
   ```go
   func TestMapToOriginal_LSPZeroBase(t *testing.T) {
       sm := load("error_prop_01_simple.go.map")
       l4c20, _ := sm.MapToOriginal(4,20)  // 1b → 4,13 "R"
       if l4c20 != (4,13) { t.Fatal() }
       l3c19, _ := sm.MapToOriginal(3,19)  // 0b sim → fallback to 4,20→4,13
   }
   ```

2. **Integration** (pkg/lsp/lsp_test.json or e2e):
   - `go test ./pkg/lsp -run TestDiagnosticsTranslation`
   - Mock gopls diag JSON → assert translated Range{3,12-25} (0b, covers "ReadFile").

3. **Manual**:
   - `dingo build tests/golden/error_prop_01_simple.dingo`
   - `dingo-lsp` → VSCode/Neovim open `.dingo` → Underline **exactly** "ReadFile", not "?".

4. **Golden**:
   - Add `lsp_error_prop_01.dingo` → `.go.golden` + `.map.golden` + `expected_diag.json`.

**Files**: Edit `pkg/lsp/handlers.go`, `pkg/preprocessor/sourcemap.go`. Test: `go test ./pkg/... -v`. Commit after verification.

**Status**: Analysis complete. Ready for implementation (delegate to golang-developer?).

[claudish] Shutting down proxy server...
[claudish] Done

