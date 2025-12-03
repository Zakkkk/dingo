# Ternary Operator (`? :`)

## Scenario
Permission checks and status formatting where simple conditions determine return values. Ternary is perfect for these "if A then X else Y" patterns.

## The Problem
Go lacks a ternary operator, forcing verbose if-else for simple conditionals:
```go
func GetStatusBadge(user User) string {
    if user.Active {
        return "active"
    }
    return "inactive"
}
```

This is 5 lines for what should be 1.

## Dingo Solution
Classic ternary syntax:

```dingo
func GetStatusBadge(user: User) string {
    return user.Active ? "active" : "inactive"
}
```

## Comparison

| Pattern | Go | Dingo |
|---------|-----|-------|
| Simple conditional | 5 lines | 1 line |
| Nested conditional | 9+ lines | 1 line |
| Inline assignment | Not possible | `x := cond ? a : b` |

## Key Points

### Syntax
```dingo
condition ? valueIfTrue : valueIfFalse
```

### Type Requirements
Both branches must have the same type:
```dingo
// OK: both strings
status := active ? "on" : "off"

// ERROR: type mismatch
result := flag ? "yes" : 42  // string vs int
```

### Nesting (Limited)
Ternary can be nested but limited to 3 levels for readability:
```dingo
level := isAdmin ? 3 : isOwner ? 2 : isPublic ? 1 : 0
```

For more complex logic, use `match` instead.

### Use in Expressions
Ternary works anywhere an expression is valid:
```dingo
fmt.Printf("Status: %s\n", user.Active ? "active" : "inactive")
price := basePrice * (isPremium ? 0.8 : 1.0)
```

### When to Use
- Simple A/B choices
- Status badges and labels
- Discount/multiplier selection
- Default value selection
- Inline conditional formatting

### When NOT to Use
- Complex conditions (use `match` or `if`)
- More than 2-3 options (use `match`)
- Side effects (use `if` statement)
- When readability suffers

## Generated Code
The transpiler generates:
- IIFE (Immediately Invoked Function Expression) for expression form
- Standard if-else for statement context
- Zero runtime overhead (compiler inlines IIFEs)
- Concrete types (no interface{})

Example transformation:
```dingo
status := active ? "on" : "off"
```
becomes:
```go
status := func() string {
    if active {
        return "on"
    }
    return "off"
}()
```
