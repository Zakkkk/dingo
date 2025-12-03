# Pattern Matching (`match` expression)

## Scenario
An event processing system that handles different event types. This pattern is everywhere: message queues, state machines, command handlers, protocol parsing.

## The Problem
Go's type switches are verbose and error-prone:
1. No exhaustiveness checking (easy to forget a case)
2. Manual field extraction
3. Guards require nested if statements
4. No expression form (always statements)

```go
switch e := event.(type) {
case *UserCreated:
    userID := e.UserID  // Manual extraction
    // ...
case *OrderPlaced:
    if e.Amount > 1000 {  // Nested guard
        // ...
    }
// Easy to forget a case - no compiler warning
}
```

## Dingo Solution
`match` expression with exhaustiveness and guards:

```dingo
match event {
    UserCreated { userID, email } =>
        fmt.Sprintf("Welcome %s", email),

    OrderPlaced { orderID, amount, .. } if amount > 1000 =>
        fmt.Sprintf("HIGH VALUE: %s", orderID),

    OrderPlaced { orderID, .. } =>
        fmt.Sprintf("Order %s confirmed", orderID),
}
```

## Comparison

| Feature | Go switch | Dingo match |
|---------|-----------|-------------|
| Exhaustiveness | No | Yes (compile error) |
| Field destructuring | Manual | Automatic |
| Guards | Nested if | `if` clause |
| Expression form | No | Yes |
| Wildcard | No | `_` and `..` |

## Key Points

### Exhaustiveness Checking
Compiler ensures all variants are handled:
```dingo
match event {
    UserCreated { .. } => "...",
    // ERROR: missing cases for UserDeleted, OrderPlaced, etc.
}
```

### Destructuring
Extract fields directly in the pattern:
```dingo
OrderPlaced { orderID, amount, userID } => ...
```

### Wildcards
- `_` matches any single value (ignored)
- `..` matches remaining fields (struct rest pattern)

```dingo
OrderPlaced { orderID, .. } => ...  // Ignore amount, userID
```

### Guards
Add conditions with `if`:
```dingo
OrderPlaced { amount, .. } if amount > 1000 => "high value",
OrderPlaced { .. } => "normal",  // Fallback
```

### Expression vs Statement
`match` returns a value:
```dingo
let message = match event {
    PaymentFailed { .. } => "URGENT",
    _ => "normal",
}
```

## Generated Code
The transpiler generates:
- Type switches for sum type matching
- Field extraction as local variables
- Guard conditions as nested if statements
- Panic for non-exhaustive matches (caught at compile time)
