# Dingo Sourcemap v3 Architecture — Code Review

Date: 2025-12-13
Scope: Sourcemap v3 migration (token.Pos tracking + .dmap v3 format)
Reviewer: Claude agent (Anthropic Claude Agent SDK) — model claude-sonnet-4-5-20250929

## Strengths

1. **Good direction: token.Pos over byte deltas**
   - The intent in `/Users/jack/mag/dingo/pkg/sourcemap/position_tracker.go` to track `token.Pos` and resolve after `go/printer` is the right architectural move. It aligns with Go’s position model (`token.FileSet`) and avoids v2’s “byte arithmetic becomes stale” failure mode.

2. **Binary format is simple and mostly self-describing**
   - `/Users/jack/mag/dingo/pkg/sourcemap/dmap/format.go` v3 header is fixed-size and uses explicit offsets/counts. The design is easy to parse via `encoding/binary` and should remain forward-compatible.

3. **Writer has useful internal consistency checks**
   - `/Users/jack/mag/dingo/pkg/sourcemap/dmap/writer.go` validates section offsets at boundaries (line index → line mappings → column mappings → kind strings). That is a pragmatic correctness guard against offset drift during refactors.

4. **Reader API is broadly testable and concurrency-safe for lookups**
   - `/Users/jack/mag/dingo/pkg/sourcemap/dmap/reader.go` uses `sync.RWMutex` and pure functions for lookup. The tests in `/Users/jack/mag/dingo/pkg/sourcemap/dmap/reader_test.go` include a concurrent access smoke test.

## Concerns

### CRITICAL

1. **PositionTracker is effectively a stub (identity mapping + 1-line blocks)**
   - File: `/Users/jack/mag/dingo/pkg/sourcemap/position_tracker.go`
   - Lines: 90–170, 192–217
   - Issue:
     - `findGoPosition()` currently returns `goLine=dingoLine` and `goCol=dingoCol` unconditionally.
     - `findGoEndLine()` currently returns `goStartLine` unconditionally.
   - Impact:
     - v3 column mappings are not meaningful, and even line mapping ranges will be wrong for any transform that expands a single Dingo line into multiple Go lines (error propagation, match lowering, etc.).
     - This also makes the `ColumnMapping.Length` field nonsense (see next item).
   - Evidence snippet:
     ```go
     // /Users/jack/mag/dingo/pkg/sourcemap/position_tracker.go
     func (t *PositionTracker) findGoPosition(...) (goLine, goCol int) {
         goLine = dingoLine
         goCol = dingoCol
         return goLine, goCol
     }

     func (t *PositionTracker) findGoEndLine(...) int {
         return goStartLine // Single-line default
     }
     ```

2. **ColumnMapping.Length semantic mismatch (bytes vs lines)**
   - Files:
     - `/Users/jack/mag/dingo/pkg/sourcemap/position_tracker.go:157–166`
     - `/Users/jack/mag/dingo/pkg/sourcemap/dmap/format.go:61–70`
   - Issue:
     - `dmap.ColumnMappingEntry.Length` is documented as “Length of the mapped region (bytes)”.
     - `PositionTracker` sets `Length = tr.GoLineEnd - tr.GoLine + 1` (a line count).
   - Impact:
     - Any consumer interpreting Length as bytes (as the format indicates) will compute incorrect highlight/range extents.

3. **Tests for writer header/offset layout are inconsistent with v3 (appear to assert v2 offsets)**
   - File: `/Users/jack/mag/dingo/pkg/sourcemap/dmap/writer_test.go`
   - Issue:
     - Several tests read fields at offsets that correspond to the old v2 header layout (e.g., treating bytes 8–12 as an “entryCount”). In v3, bytes 8–12 are `DingoLen`.
   - Impact:
     - Tests may be failing or (worse) asserting the wrong invariants; either outcome reduces confidence in the binary format migration.
   - Evidence snippet:
     ```go
     // /Users/jack/mag/dingo/pkg/sourcemap/dmap/writer_test.go
     entryCount := binary.LittleEndian.Uint32(data[8:12])
     // ... comment says "v2 format" but this repo claims v3
     ```

4. **Reader’s `parseKindStrings` can panic on corrupted input**
   - File: `/Users/jack/mag/dingo/pkg/sourcemap/dmap/reader.go`
   - Lines: ~164–210
   - Issue:
     - `stringDataStart := kindStrStart + 4 + int(kindCount)*4` is not bounds-checked before slicing `r.data[stringDataStart:]`.
   - Impact:
     - A malformed `.dmap` can crash the LSP server / tooling process (panic), which is a reliability/security issue.

### IMPORTANT

1. **Pure pipeline is still using TransformTracker; v3 integration is incomplete**
   - File: `/Users/jack/mag/dingo/pkg/transpiler/pure_pipeline.go`
   - Lines: 122–134, 335–358
   - Issue:
     - The pipeline creates a dummy `dingoFset`, but then uses `sourcemap.NewTransformTracker(source)` and returns empty `ColumnMappings`.
   - Impact:
     - The repository claims a token.Pos-based v3 architecture, but the current end-to-end behavior still depends on byte-offset tracking (deprecated) and does not emit/propagate column mappings.

2. **Reader lookup correctness relies on a non-overlap invariant that is not validated**
   - File: `/Users/jack/mag/dingo/pkg/sourcemap/dmap/reader.go`
   - Lines: 245–249, 400–421
   - Issue:
     - `GoLineToDingoLine` uses `sort.Search` with predicate `GoLineEnd >= goLine`.
     - This binary search is only valid if mapping intervals are non-overlapping and sorted such that `GoLineEnd` is monotonic.
     - `parseLineMappings` sorts only by `GoLineStart` and does not validate non-overlap.
   - Impact:
     - If mappings overlap (possible if multiple transforms map to overlapping Go ranges, or if generation is buggy), lookups can return wrong results.

3. **DingoLineLength subtracts newline unconditionally (last line without trailing newline)**
   - File: `/Users/jack/mag/dingo/pkg/sourcemap/dmap/reader.go`
   - Lines: 359–385
   - Issue:
     - For the last line, `lineEnd` is `hdr.DingoLen`. If the file does not end in `\n`, subtracting 1 is incorrect.
   - Impact:
     - Column clamping can underflow by one, causing off-by-one cursor/diagnostic mapping.

4. **V3 format types may truncate large files (uint16 for lines/cols/length)**
   - File: `/Users/jack/mag/dingo/pkg/sourcemap/dmap/format.go`
   - Lines: 61–70
   - Issue:
     - `ColumnMappingEntry` stores line/col/length as `uint16`. Large files or columns > 65535 will overflow/truncate.
   - Impact:
     - Silent corruption for large generated outputs. If this is an intended constraint, it should be explicitly documented and validated at write time.

5. **Inconsistent line offset handling between packages**
   - Files:
     - `/Users/jack/mag/dingo/pkg/sourcemap/tracker.go` (`buildLineOffsets` counts only `\n`)
     - `/Users/jack/mag/dingo/pkg/sourcemap/dmap/writer.go` (`buildLineOffsets` handles CRLF / bare CR)
   - Impact:
     - Depending on the path, different components can disagree on line counts/offsets for CRLF files.

### MINOR

1. **Outdated comments (“v2” wording) reduce clarity**
   - File: `/Users/jack/mag/dingo/pkg/sourcemap/dmap/reader.go`
   - Lines: 387–390
   - Example: comment says “using v2 line mappings” in a v3-only reader.

2. **PositionTracker API doesn’t protect internal slices**
   - File: `/Users/jack/mag/dingo/pkg/sourcemap/position_tracker.go`
   - Issue:
     - `LineMappings()` / `ColumnMappings()` return internal slices directly.
   - Impact:
     - External callers could mutate internal state (accidentally). Often acceptable in Go, but worth deciding intentionally.

3. **SourceMapCache holds a write lock while doing disk I/O**
   - File: `/Users/jack/mag/dingo/pkg/lsp/sourcemap_cache.go`
   - Lines: 53–81
   - Impact:
     - Simplicity/correctness trade-off is explicit and reasonable for now, but it can become a contention hotspot under frequent file opens.

## Questions

1. **Intended semantics of `ColumnMappingEntry.Length`**
   - Should this be *byte length*, *rune length*, *token count*, or something else?
   - The current format and tracker disagree.

2. **What is the source of truth for Go↔Dingo mapping?**
   - If `//line` directives are intended to be the truth for diagnostics, is `.dmap` primarily for hover/go-to-definition only?
   - If yes, should the `.dmap` be derived from `goFset.PositionFor(pos, /*adjusted*/ true)` + directive info instead of scanning text?

3. **Should `.dmap` parsing be hardened for hostile inputs?**
   - Given LSP use-cases, `.dmap` files can be edited/corrupted. Is the expectation “robust errors, never panic”?

## Summary

Overall assessment: CHANGES_NEEDED

Top priorities:
1. Make `PositionTracker` produce real Go ranges (or gate v3 column mapping until it does).
2. Align `ColumnMapping.Length` semantics across tracker ↔ format ↔ writer ↔ reader.
3. Fix v3 writer/reader tests so they assert the actual v3 header layout.
4. Harden `Reader` against panics on corrupted input (bounds checks before slicing).

Testability: Medium
- dmap writer/reader are unit-testable (pure byte buffers), but correctness confidence is currently limited by (a) inconsistent tests and (b) incomplete PositionTracker logic.
