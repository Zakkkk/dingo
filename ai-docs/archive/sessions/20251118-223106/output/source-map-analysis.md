# Source Map Analysis – ReadFile Diagnostic Span

## Summary of Failure
When gopls reports `undefined: ReadFile` for a line such as `data := ReadFile(path)?`, the Dingo LSP translates the diagnostic to the Dingo file but highlights `e(path)?` (the tail that includes the `?`) instead of the `ReadFile` identifier. This happens even though the generated Go diagnostic range targets the identifier correctly.

## Root Cause
The preprocessor source map entry that should cover the original `ReadFile(path)` span is produced in `pkg/preprocessor/error_prop.go` inside `expandAssignment`. The mapping calculates

```go
origCol := len(indent) + exprPosInOriginal + 1
```

where `exprPosInOriginal` is already measured from the start of the line (including the indent). Adding `len(indent)` again double-counts the indent width, so the recorded `OriginalColumn` is shifted to the right by the indentation length. When `MapToOriginal` later remaps the gopls diagnostic column (which is correct for the Go source), it lands several characters to the right of the actual expression, which, for short function names, places the highlight on `path)?` or directly on the `?`. The `expandReturn` path already removed this bug (see the inline comment that warns against double-counting indent), confirming that only the assignment branch regressed.

The translator, source map cache, and `MapToOriginal` fallback logic behave as expected; the incorrect column metadata originates solely from the `expandAssignment` mapping.

## Proposed Fix
1. Update `pkg/preprocessor/error_prop.go`, function `expandAssignment`, to compute `origCol := exprPosInOriginal + 1` (mirroring the `expandReturn` fix) so the recorded column matches the original Dingo source. Include an explanatory comment to prevent future regressions.
2. Add a targeted regression test (e.g., extend `pkg/preprocessor/preprocessor_test.go` or a new unit test) that runs the error propagation processor on a snippet with indentation and asserts that the emitted mapping for the expression starts at the correct column.

## Verification Plan
1. `go test ./pkg/preprocessor -run TestErrorProp...` (once the new regression test is in place) to ensure mappings are correct.
2. `dingo build tests/golden/error_prop_01_simple.dingo` (or any file that triggers `ReadFile` without an import) to regenerate `.go` and `.map` outputs with the corrected columns.
3. Launch `dingo-lsp`, open the same `.dingo` file in an editor, and confirm that the undefined-symbol diagnostic now highlights the `ReadFile` identifier rather than the trailing `?` span.
