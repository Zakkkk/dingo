# Lambda Expressions

## Scenario
Data processing pipeline with filtering, mapping, and reducing. Lambdas make functional programming patterns concise and readable.

## The Problem
Go's anonymous function syntax is verbose:
```go
// 47 characters for a simple predicate
users = Filter(users, func(u User) bool { return u.Active })
```

This verbosity discourages functional patterns even when they're the best solution.

## Dingo Solution
Two concise lambda syntaxes:

```dingo
// TypeScript-style (20 characters)
Filter(users, (u) => u.Active)

// Rust-style (17 characters)
Filter(users, |u| u.Active)
```

## Comparison

| Syntax | Go | Dingo TS-style | Dingo Rust-style |
|--------|-----|----------------|------------------|
| Simple | `func(x int) int { return x * 2 }` | `(x) => x * 2` | `\|x\| x * 2` |
| Multi-param | `func(a, b int) int { return a + b }` | `(a, b) => a + b` | `\|a, b\| a + b` |
| Multi-line | `func(x int) int { ... }` | `(x) => { ... }` | `\|x\| { ... }` |

## Key Points

### TypeScript-Style Syntax
```dingo
(params) => expression
(params) => { statements }
```

### Rust-Style Syntax
```dingo
|params| expression
|params| { statements }
```

### Type Inference
Types are inferred from context:
```dingo
// Filter expects func(User) bool
// So |u| u.Active infers u: User, returns bool
Filter(users, |u| u.Active)
```

### When to Use Which
- **TypeScript-style**: Familiar to JS/TS developers
- **Rust-style**: More concise for short lambdas
- **Multi-line block**: Complex logic needing multiple statements

### Closures
Lambdas capture variables from enclosing scope:
```dingo
minAge := 18
Filter(users, |u| u.Age >= minAge)  // minAge is captured
```

## Common Patterns

### Filter + Map Chain
```dingo
names := Map(
    Filter(users, |u| u.Active),
    |u| u.Name,
)
```

### Custom Sorting
```dingo
SortUsers(users, |a, b| a.Age < b.Age)
```

### Event Handlers
```dingo
button.OnClick((event) => {
    handleClick(event)
})
```

## Generated Code
The transpiler generates:
- Standard Go anonymous functions
- Preserved closure semantics
- No runtime overhead
- Fully type-safe
