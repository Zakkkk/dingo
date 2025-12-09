# Bug Report: Lambda Return Type Inference for Void Functions

## Summary
Lambdas passed to functions expecting `func(T)` (void return) incorrectly get `any` return type inferred, causing Go compilation errors.

## Severity
**High** - Breaks `ForEach`, `ForEachWithIndex`, and any void-returning lambda usage.

## Reproduction

### Minimal Example
```dingo
package main

import "github.com/MadAppGang/dingo/pkg/dgo"

func main() {
    // This FAILS after transpilation:
    dgo.ForEach([]string{"a", "b"}, |s| fmt.Print(s))
}
```

### Generated Go Code (WRONG)
```go
dgo.ForEach([]string{"a", "b"}, func(s string) any { return fmt.Print(s) })
//                                             ^^^
// Should be NO return type, not `any`
```

### Expected Go Code
```go
dgo.ForEach([]string{"a", "b"}, func(s string) { fmt.Print(s) })
```

### Error Message
```
in call to dgo.ForEach, type func(s string) any of func(s string) any {…}
does not match inferred type func(string) for func(T)
```

## Analysis
The lambda codegen is always adding a return type (`any` as fallback) even when the target function signature expects a void function `func(T)`.

The type inference should detect when a lambda is passed to a function that expects `func(T)` (not `func(T) R`) and omit the return type.

## Related Files
- `pkg/ast/lambda_codegen.go` - Lambda code generation
- `pkg/typechecker/lambda_inference.go` - Lambda type inference

## Date Reported
2025-12-09
