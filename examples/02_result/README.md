# Result Type (`Result<T, E>`)

## Scenario
A database repository that needs to explicitly handle success and failure cases. This is critical for data operations where "not found" is different from "database error".

## The Problem
Go's `(T, error)` pattern has issues:
1. Caller can ignore the error
2. `nil` result is ambiguous - not found? or error?
3. No compile-time enforcement of error handling

```go
user, err := FindUser(id)
// Easy to forget: if err != nil check
// user might be nil even when err is nil (not found)
fmt.Println(user.Name) // potential nil pointer panic
```

## Dingo Solution
`Result<T, E>` makes success/failure explicit:

```dingo
func FindUserByID(db: *sql.DB, id: int) Result<User, DBError> {
    // Return Err for not found
    return Err[User](DBError{Code: "NOT_FOUND", Message: "user not found"})

    // Return Ok for success
    return Ok[DBError](user)
}
```

## Comparison

| Aspect | Go `(T, error)` | Dingo `Result<T, E>` |
|--------|-----------------|----------------------|
| Ignore error | Possible | Compilation fails |
| Not found vs error | Ambiguous | Distinct error types |
| Nil pointer risk | High | Zero (must unwrap) |
| Type safety | Partial | Full |

## Key Points

### Result Methods
- `IsOk()` / `IsErr()` - Check variant
- `Unwrap()` - Get value (panics if Err)
- `UnwrapErr()` - Get error (panics if Ok)
- `UnwrapOr(default)` - Get value or default
- `Map(fn)` - Transform Ok value
- `AndThen(fn)` - Chain Result-returning functions

### When to Use
- Database operations (not found vs error)
- API calls (different error types)
- Validation (specific failure reasons)
- Any operation with multiple failure modes

### When Go's `(T, error)` is Fine
- Simple I/O operations
- Functions with single failure mode
- Interop with existing Go code

## Type Parameter Order
Dingo uses `Result<T, E>` where:
- `T` = success type
- `E` = error type (must implement `error`)

Constructor type inference:
```dingo
Ok[DBError](user)    // T inferred from value, E specified
Err[User](dbError)   // E inferred from value, T specified
```

## Generated Code
The transpiler generates:
- Generic `Result` struct with `isOk` discriminant
- Type-safe `Ok` and `Err` constructors
- Methods for safe value access
- Zero runtime overhead (all inlined)
