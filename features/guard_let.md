# Guard Let Statement

Swift-inspired guard let statement for Result/Option unwrapping with automatic error binding.

## Status
- **Phase**: 10
- **Status**: Complete
- **Date**: 2025-12-03

## Syntax

```dingo
guard let <variable> = <result_or_option_expr> else {
    // For Result: 'err' is automatically bound to the error value
    // For Option: no binding (value is None)
    <early_return_or_action>
}
// <variable> is available here with unwrapped value
```

## Examples

### Result Type

```dingo
func GetUserOrder(userID int) Result {
    guard let user = FindUser(userID) else { return err }
    guard let order = GetOrder(user.ID) else { return err }

    return ResultOk(fmt.Sprintf("%s: %s", user.Name, order.ID))
}
```

### Option Type

```dingo
func ProcessUser(maybeUser Option) string {
    guard let user = maybeUser else {
        return "no user"  // No 'err' binding for Option
    }
    return user.Name
}
```

### Inline Syntax

```dingo
guard let from = FindUser(fromID) else { return err }
guard let to = FindUser(toID) else { return err }
```

### Custom Error Handling

```dingo
guard let user = FindUser(id) else {
    log.Error("Failed to find user", err)
    return ServiceError{Code: "NOT_FOUND", Message: err.Error()}
}
```

## Generated Code

### Result Type Input
```dingo
guard let user = userResult else {
    return err
}
fmt.Println(user.Name)
```

### Result Type Output
```go
tmp := userResult
if tmp.IsErr() {
    err := *tmp.err
    return ResultErr(err)
}
user := *tmp.ok
fmt.Println(user.Name)
```

### Option Type Input
```dingo
guard let user = maybeUser else {
    return "no user"
}
```

### Option Type Output
```go
tmp := maybeUser
if tmp.IsNone() {
    return "no user"
}
user := *tmp.some
```

## Type Detection

Guard let determines the expression type using:

1. **Function signature scanning** - Parses `func Name() ReturnType` declarations
2. **TypeRegistry lookup** - Queries registered variables and functions
3. **Naming convention fallback** - Checks for "Result" or "Option" in expression

## Design Decisions

### Error Variable Name: `err`
Uses standard Go convention. User handles shadowing themselves.

### Temp Variable Strategy: Smart Detection
- Simple identifiers (e.g., `userResult`) → use directly
- Function calls (e.g., `FindUser(id)`) → use temp variable to avoid multiple evaluations

### Return Transformation
`return err` in Result else blocks automatically transforms to `return ResultErr(err)`.

## Comparison: Before and After

### Before (16 lines)
```dingo
func GetUserOrderTotal(db *sql.DB, userID int) Result {
    userResult := FindUser(db, userID)
    if userResult.IsErr() {
        return userResult.UnwrapErr()
    }
    user := userResult.Unwrap()

    ordersResult := FindOrdersByUser(db, user.ID)
    if ordersResult.IsErr() {
        return ordersResult.UnwrapErr()
    }
    orders := ordersResult.Unwrap()

    var total float64
    for _, order := range orders {
        total += order.Total
    }
    return total
}
```

### After (8 lines, 50% reduction)
```dingo
func GetUserOrderTotal(db *sql.DB, userID int) Result {
    guard let user = FindUser(db, userID) else { return err }
    guard let orders = FindOrdersByUser(db, user.ID) else { return err }

    var total float64
    for _, order := range orders {
        total += order.Total
    }
    return total
}
```

## Implementation

- **Processor**: `pkg/preprocessor/guard_let_ast.go` (473 lines)
- **Pipeline Position**: Pass 1 (Structural), position 1 (after DingoPreParser)
- **Type Info**: Uses TypeRegistry from `pkg/registry/`

## Golden Tests

- `tests/golden/guard_let_result.dingo` - Result type scenarios
- `tests/golden/guard_let_option.dingo` - Option type scenarios
- `tests/golden/guard_let_inline.dingo` - Inline syntax variations
- `tests/golden/guard_let_tuple.dingo` - Multiple guard lets
