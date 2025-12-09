# Bug Report: Block Lambda Parse Error

## Summary
Block lambdas with TypeScript-style syntax `(params) => { ... }` fail to transpile with "missing ',' in argument list" error.

## Severity
**High** - Blocks users from using multi-statement lambdas, a core feature for complex functional programming patterns.

## Reproduction

### Minimal Example
```dingo
package main

func main() {
    numbers := []int{1, 2, 3}

    // This FAILS:
    csv := Reduce(numbers, "", (acc, x) => {
        if acc == "" {
            return fmt.Sprintf("%d", x)
        }
        return fmt.Sprintf("%s,%d", acc, x)
    })
}
```

### Error Message
```
transpilation error: parse error: examples/06_lambdas/data_pipeline.dingo:31:53: missing ',' in argument list (and 10 more errors)
```

### Working Alternative (single-expression)
```dingo
// This WORKS:
sum := Reduce(numbers, 0, |acc, x| acc + x)
```

## Affected Files
- `examples/06_lambdas/data_pipeline.dingo` - Cannot transpile due to block lambda on line 31
- Any code using `(params) => { block }` syntax

## Analysis

The block lambda parser in `pkg/parser/lambda.go` appears to correctly parse block bodies via `parseLambdaBlock()`, but the issue occurs during Go AST transformation.

### Suspected Root Cause
The generated Go code from block lambdas may have malformed syntax. Looking at the already-generated `data_pipeline.go`:

```go
// Line 31 - this was previously generated and works:
summary := Reduce(eligible, "", func(acc string, u User,) string {
    if acc == "" {
        return u.Name
    }
    return acc + ", " + u.Name
})
```

Note the trailing comma after `User,` - this is valid Go but unusual. The current transpiler may be generating something different that Go's parser rejects.

## Steps to Debug
1. Run transpiler with verbose output to see intermediate AST
2. Check `pkg/ast/lambda_codegen.go` for block body handling
3. Compare generated code between working single-expression and failing block lambdas

## Workaround
Use Rust-style lambdas with single expressions, or define named functions for complex logic.

## Related Code
- `pkg/parser/lambda.go:414-464` - `parseLambdaBody()` and `parseLambdaBlock()`
- `pkg/ast/lambda_codegen.go` - Lambda code generation

## Date Reported
2025-12-09
