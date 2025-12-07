# Let CodeGen formatting fix

## Overview
Updated `pkg/ast/let.go` to correctly handle `TypeAnnot` formatting in the `ToGo()` method. The fix ensures that type annotations are always separated from the variable name by a space, and that the leading colon (if present) is properly removed.

## Files Changed
- `pkg/ast/let.go` (modified)
  - Updated `ToGo()` method to use `strings.TrimSpace` and `strings.TrimPrefix` for robust handling of type annotations.
  - Added "strings" import.
- `pkg/codegen/let_test.go` (modified)
  - Updated `TestLetCodeGen_TypeAnnotationWithoutColon` to expect correct Go output (`var x int` instead of `var xint`).

## Implementation Details
The previous implementation blindly concatenated `d.TypeAnnot` after the name, only checking for a leading colon. This meant that `TypeAnnot` values like "int" (without colon) resulted in "var xint".

New implementation:
```go
cleanType := strings.TrimSpace(d.TypeAnnot)
cleanType = strings.TrimPrefix(cleanType, ":")
cleanType = strings.TrimSpace(cleanType)
if len(cleanType) > 0 {
    result += " " + cleanType
}
```

This logic handles:
- `: int` -> ` int`
- `:int` -> ` int`
- `int` -> ` int`
- `   :  int` -> ` int`

## Test Results
Running `go test -v ./pkg/codegen -run TestLetCodeGen` verified the fix.
Running `go test -v ./pkg/ast` verified no regressions in AST package.

```
=== RUN   TestLetCodeGen_TypeAnnotationWithoutColon
--- PASS: TestLetCodeGen_TypeAnnotationWithoutColon (0.00s)
PASS
```
