# Bug Report: Lambda Return Type Inferred as `any` Instead of Concrete Type

## Summary
Lambdas returning concrete types (like `[]string`, `float64`) get their return type inferred as `any` or `[]any`, causing type mismatches with generic functions.

## Severity
**High** - Breaks FlatMap, Map, and other generics that depend on return type inference.

## Reproduction

### Minimal Example
```dingo
package main

import "github.com/MadAppGang/dingo/pkg/dgo"

type Order struct {
    Items []string
}

func main() {
    orders := []Order{{Items: []string{"a", "b"}}}

    // This FAILS after transpilation:
    allItems := dgo.FlatMap(orders, |o| o.Items)
}
```

### Generated Go Code (WRONG)
```go
allItems := dgo.FlatMap(orders, func(o Order) []any { return o.Items })
//                                             ^^^^
// Should be []string, not []any
```

### Expected Go Code
```go
allItems := dgo.FlatMap(orders, func(o Order) []string { return o.Items })
```

### Error Message
```
cannot use o.Items (variable of type []string) as []any value in return statement
```

## Additional Cases

### Float64 inferred as any
```dingo
total := dgo.Reduce(prices, 0.0, |acc, p| acc + p)
```
Generates:
```go
total := dgo.Reduce(prices, 0.0, func(acc float64, p any) any { return acc + p })
// ERROR: invalid operation: acc + p (mismatched types float64 and any)
```

## Analysis
The lambda type inference should:
1. Look at the generic function signature (`FlatMap[T, U any](slice []T, fn func(T) []U)`)
2. Infer `T` from the slice element type
3. Infer `U` from the lambda body's return expression type
4. Use `U` (not `any`) as the lambda return type

Currently, it falls back to `any` when it can't infer the type.

## Related Files
- `pkg/typechecker/lambda_inference.go` - Lambda type inference
- `pkg/ast/lambda_codegen.go` - Lambda code generation

## Date Reported
2025-12-09
