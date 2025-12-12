# Error Propagation (`?` operator)

## Scenario
An HTTP handler that loads user data from a database. This is one of the most common patterns in Go web applications where multiple operations can fail.

## The Problem
In standard Go, every fallible operation requires 3 lines of boilerplate:
```go
result, err := operation()
if err != nil {
    return err
}
```

A typical handler with 4 operations needs 12+ lines just for error handling, obscuring the actual business logic.

## Dingo Solution
The `?` operator propagates errors automatically:
```dingo
userID := extractUserID(r)?
user := loadUserFromDB(userID)?
_ := checkPermissions(r, user)?
response := json.Marshal(user)?
```

## Comparison

| Metric | Dingo | Go |
|--------|-------|-----|
| Lines in handler | 15 | 35 |
| Error checks | 4 (implicit) | 4 (explicit, 12 lines) |
| Business logic visibility | High | Low (buried in boilerplate) |

## Key Points

### What `?` Does
1. Calls the function
2. If error is non-nil, returns immediately with that error
3. If error is nil, unwraps the value

### When to Use
- Functions returning `(T, error)`
- When you want to propagate errors unchanged
- Sequential operations where failure means early return

### When NOT to Use
- When you need to handle errors differently (use explicit `if err != nil`)
- When you need to wrap errors with context (use `? "context message"`)
- Non-error returning functions

## Real-World Impact
Based on Go proposal #71203 data:
- Average Go file has 23 `if err != nil` per 1000 lines
- `?` operator reduces this by ~67%
- Clearer code flow, fewer bugs from forgotten error checks

## Generated Code Notes
The transpiler generates:
- Unique temp variables (`tmp`, `tmp1`, `tmp2`)
- Standard Go error checking idiom
- No runtime overhead (pure compile-time transformation)
