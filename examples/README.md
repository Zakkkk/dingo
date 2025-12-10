# Dingo Examples

Real-world examples demonstrating Dingo's features. Each example shows a practical use case with the Dingo source code and the generated Go output.

## Structure

Each example folder contains:
- `*.dingo` - Dingo source code
- `*.go` - Generated Go code (for comparison)
- `README.md` - Feature explanation, use cases, and generated code notes

## Examples by Feature

| # | Feature | Scenario | Key Benefit |
|---|---------|----------|-------------|
| 01 | [Error Propagation](01_error_propagation/) | HTTP handler | 67% less error handling boilerplate |
| 02 | [Result Type](02_result/) | Database repository | Explicit success/failure modeling |
| 03 | [Option Type](03_option/) | User settings | Zero nil pointer panics |
| 04 | [Pattern Matching](04_pattern_matching/) | Event handler | Exhaustive case handling |
| 05 | [Sum Types](05_sum_types/) | API responses | Type-safe variants |
| 06 | [Lambdas](06_lambdas/) | Data pipeline | Concise functional code |
| 07 | [Tuples](07_tuples/) | Geometry | Group values without structs |
| 08 | [Safe Navigation](08_safe_navigation/) | Config access | Safe deep object traversal |
| 09 | [Ternary](09_ternary/) | Permissions | Inline conditionals |
| 10 | [Null Coalesce](10_null_coalesce/) | Defaults | Chained fallback values |

## Quick Comparison

### Before (Go)
```go
func GetUser(id int) (*User, error) {
    data, err := db.Query(id)
    if err != nil {
        return nil, err
    }
    user, err := parseUser(data)
    if err != nil {
        return nil, err
    }
    return user, nil
}
```

### After (Dingo)
```dingo
func GetUser(id: int) (*User, error) {
    data := db.Query(id)?
    user := parseUser(data)?
    return user, nil
}
```

## Running Examples

```bash
# Transpile a single file
dingo build examples/01_error_propagation/http_handler.dingo

# Run the generated Go
go run examples/01_error_propagation/http_handler.go
```

## Philosophy

These examples prioritize:
1. **Real scenarios** - Code you'd actually write
2. **Honest comparison** - Show both advantages and trade-offs
3. **Clarity** - One feature per example
4. **Completeness** - Both Dingo and generated Go code

The generated Go code shows exactly what Dingo produces - no magic, just cleaner syntax that transpiles to standard Go.
