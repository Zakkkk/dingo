# AI Instructions for Writing Dingo Code

This document teaches AI assistants how to write idiomatic Dingo code instead of plain Go. Dingo is a meta-language for Go that transpiles `.dingo` files to `.go` files, providing modern language features while maintaining 100% Go ecosystem compatibility.

## Core Philosophy

Dingo is **syntax sugar for Go**, not a new type system. All type checking happens via gopls. The goal is to make common patterns more concise and safer without departing from Go idioms.

## Feature Reference

### 1. Error Propagation (`?` operator)

**Instead of verbose Go error handling, use the `?` operator.**

#### Basic Propagation
```dingo
// ❌ Go way - verbose
func LoadUser(id string) (*User, error) {
    user, err := db.FindUser(id)
    if err != nil {
        return nil, err
    }
    return user, nil
}

// ✅ Dingo way - concise
func LoadUser(id string) (*User, error) {
    user := db.FindUser(id)?
    return user, nil
}
```

#### With Context Message
```dingo
// ❌ Go way
user, err := db.FindUser(id)
if err != nil {
    return nil, fmt.Errorf("database lookup failed: %w", err)
}

// ✅ Dingo way
user := db.FindUser(id) ? "database lookup failed"
```

#### With Custom Error Transform (Rust-style lambda)
```dingo
// ❌ Go way
result, err := parseJSON(data)
if err != nil {
    return nil, NewAppError(400, "parse failed", err)
}

// ✅ Dingo way
result := parseJSON(data) ? |e| NewAppError(400, "parse failed", e)
```

#### With Custom Error Transform (TypeScript-style lambda)
```dingo
// Same as above, different syntax preference
result := parseJSON(data) ? (e) => NewAppError(400, "parse failed", e)
// Or without parens for single param:
result := parseJSON(data) ? e => NewAppError(400, "parse failed", e)
```

### 2. Option[T] Type

**Use Option[T] instead of nil pointers for values that may or may not exist.**

```dingo
import "github.com/MadAppGang/dingo/pkg/dgo"

// ❌ Go way - nil pointer bugs
type Settings struct {
    Theme *string  // nil means "not set"
}
func GetTheme(s Settings) string {
    if s.Theme != nil {
        return *s.Theme
    }
    return "default"
}

// ✅ Dingo way - explicit optionality
type Settings struct {
    Theme Option[string]  // Explicit: Some or None
}
func GetTheme(s Settings) string {
    return s.Theme.SomeOr("default")
}
```

#### Option API
```dingo
// Creating Options
theme := Some("dark")           // Option[string] with value
empty := None[string]()         // Option[string] without value

// Checking state
if opt.IsSome() { ... }
if opt.IsNone() { ... }

// Extracting values
value := opt.MustSome()         // Panics if None
value := opt.SomeOr("default")  // Returns default if None
value := opt.SomeOrElse(func() string { return computeDefault() })  // Lazy default
```

### 3. Result[T, E] Type

**Use Result[T, E] instead of (T, error) when you want explicit success/failure modeling.**

```dingo
import "github.com/MadAppGang/dingo/pkg/dgo"

// ❌ Go way - implicit error handling
func FindUser(id int) (*User, error) {
    // ...
}

// ✅ Dingo way - explicit Result type
func FindUser(id int) Result[User, DBError] {
    if notFound {
        return DBError{Code: "NOT_FOUND", Message: "user not found"}
    }
    return user  // Implicitly wrapped as Ok
}
```

#### Result API
```dingo
// Creating Results
ok := Ok[User, DBError](user)
err := Err[User, DBError](DBError{...})
// Or return directly (auto-wrapped):
return user      // becomes Ok
return dbError   // becomes Err

// Checking state
if result.IsOk() { ... }
if result.IsErr() { ... }

// Extracting values
user := result.MustOk()         // Panics if Err
err := result.MustErr()         // Panics if Ok
user := result.OkOr(defaultUser)
```

### 4. Sum Types / Enums

**Use `enum` for discriminated unions instead of interfaces with many implementations.**

```dingo
// ❌ Go way - verbose interface pattern
type Event interface { isEvent() }
type UserCreated struct { UserID int; Email string }
func (UserCreated) isEvent() {}
type OrderPlaced struct { OrderID string; Amount float64 }
func (OrderPlaced) isEvent() {}

// ✅ Dingo way - concise enum
enum Event {
    UserCreated { userID: int, email: string }
    OrderPlaced { orderID: string, amount: float64 }
}
```

#### Creating Enum Values
```dingo
// Constructor syntax
event := Event.UserCreated(1, "alice@example.com")
order := Event.OrderPlaced("ORD-001", 99.99)
```

### 5. Pattern Matching (`match`)

**Use `match` instead of type switches and nested if-else chains.**

```dingo
// ❌ Go way - verbose type switch
func ProcessEvent(event Event) string {
    switch v := event.(type) {
    case EventUserCreated:
        return fmt.Sprintf("Welcome %s", v.Email)
    case EventOrderPlaced:
        if v.Amount > 1000 {
            return fmt.Sprintf("HIGH VALUE: %s", v.OrderID)
        }
        return fmt.Sprintf("Order %s confirmed", v.OrderID)
    default:
        return "Unknown event"
    }
}

// ✅ Dingo way - expressive match
func ProcessEvent(event Event) string {
    return match event {
        UserCreated(userID, email) =>
            fmt.Sprintf("Welcome %s (user #%d)", email, userID),

        OrderPlaced(orderID, amount, _) if amount > 1000 =>
            fmt.Sprintf("HIGH VALUE: Order %s", orderID),

        OrderPlaced(orderID, _, _) =>
            fmt.Sprintf("Order %s confirmed", orderID),

        _ => "Unknown event",
    }
}
```

#### Match Features
- **Destructuring**: Extract fields directly in the pattern
- **Guards**: Add `if condition` after pattern for additional filtering
- **Wildcards**: Use `_` to ignore values
- **Default case**: Use `_` alone as catch-all

### 6. Lambda Expressions

**Use concise lambda syntax instead of verbose func literals.**

```dingo
// ❌ Go way - verbose
users := Filter(users, func(u User) bool { return u.Active })
names := Map(users, func(u User) string { return u.Name })

// ✅ Dingo way - Rust-style
users := Filter(users, |u| u.Active)
names := Map(users, |u| u.Name)

// ✅ Dingo way - TypeScript-style
users := Filter(users, (u) => u.Active)
names := Map(users, u => u.Name)  // parens optional for single param
```

#### Multi-line Lambdas
```dingo
// With block body (use braces)
result := Reduce(items, "", |acc, item| {
    if acc == "" {
        return item.Name
    }
    return acc + ", " + item.Name
})
```

#### Multiple Parameters
```dingo
sorted := SortUsers(users, |a, b| a.Age < b.Age)
sorted := SortUsers(users, (a, b) => a.Name < b.Name)
```

### 7. Safe Navigation (`?.`)

**Use `?.` to safely navigate nullable pointer chains.**

```dingo
// ❌ Go way - nil check pyramid
func GetDBHost(config *ServerConfig) string {
    if config != nil {
        if config.Database != nil {
            return config.Database.Host
        }
    }
    return "localhost"
}

// ✅ Dingo way - safe navigation
func GetDBHost(config *ServerConfig) string {
    return config?.Database?.Host ?? "localhost"
}
```

#### How It Works
- `?.` short-circuits to zero value if any part of the chain is nil
- Combine with `??` for custom defaults
- Works with nested pointer types

### 8. Null Coalescing (`??`)

**Use `??` to provide default values for nil pointers.**

```dingo
// ❌ Go way
func GetHost(config *AppConfig) string {
    if config != nil && config.Host != nil {
        return *config.Host
    }
    return "localhost"
}

// ✅ Dingo way
func GetHost(config *AppConfig) string {
    return config?.Host ?? "localhost"
}
```

#### Chained Defaults
```dingo
// Check multiple sources in order
endpoint := primary ?? secondary ?? tertiary ?? "https://default.api.com"
```

### 9. Ternary Operator (`? :`)

**Use C-style ternary instead of if-else for simple conditionals.**

```dingo
// ❌ Go way
func GetStatus(isAdmin bool) string {
    if isAdmin {
        return "Administrator"
    }
    return "User"
}

// ✅ Dingo way
func GetStatus(isAdmin bool) string {
    return isAdmin ? "Administrator" : "User"
}
```

#### Nested Ternary
```dingo
// Multiple conditions
category := age >= 65 ? "Senior" : age >= 18 ? "Adult" : "Minor"
```

### 10. Tuples

**Use tuples for grouping related values without creating a struct.**

```dingo
// ❌ Go way - create struct or use multiple returns
type Point struct { X, Y float64 }
func GetCoords() (float64, float64) { return x, y }

// ✅ Dingo way - tuple types
type Point2D = (float64, float64)

func GetCoords() Point2D {
    return (3.5, 4.2)
}

// Destructuring
(x, y) := GetCoords()
```

#### Tuple Features
```dingo
// Create tuples
point := (10.0, 20.0)
triple := (1, "hello", true)

// Destructure
(a, b) := point
(x, y, z) := triple

// Type aliases
type Point2D = (float64, float64)
type BoundingBox = (Point2D, Point2D)
```

### 11. Guard Statement

**Use `guard` for early returns from Result/Option types.**

```dingo
// ❌ Go way
func LoadUser(id int) Result[string, string] {
    result := findUser(id)
    if result.IsErr() {
        return Err[string](fmt.Sprintf("load failed: %s", result.MustErr()))
    }
    user := result.MustOk()
    return Ok[string, string](user.Name)
}

// ✅ Dingo way
func LoadUser(id int) Result[string, string] {
    guard user := findUser(id) else |err| {
        return Err[string](fmt.Sprintf("load failed: %s", err))
    }
    return Ok[string, string](user.Name)
}
```

#### Guard Syntax Variants
```dingo
// Result with error binding
guard x := expr else |err| { return Err[T](err) }

// Option without binding (None has no value)
guard x := expr else { return defaultValue }

// Reassign existing variable (= instead of :=)
guard x = expr else |err| { return Err[T](err) }
```

## Quick Reference Table

| Feature | Go Syntax | Dingo Syntax |
|---------|-----------|--------------|
| Error propagation | `x, err := f(); if err != nil { return err }` | `x := f()?` |
| Error with context | `if err != nil { return fmt.Errorf("msg: %w", err) }` | `x := f() ? "msg"` |
| Error transform | `if err != nil { return transform(err) }` | `x := f() ? \|e\| transform(e)` |
| Optional value | `*T` with nil checks | `Option[T]` |
| Result type | `(T, error)` | `Result[T, E]` |
| Sum type | interface + structs | `enum Name { Variant { field: type } }` |
| Pattern match | `switch v := x.(type)` | `match x { Pattern => expr }` |
| Lambda | `func(x T) R { return expr }` | `\|x\| expr` or `(x) => expr` |
| Safe navigation | Nested nil checks | `a?.b?.c` |
| Null coalescing | `if x != nil { return *x }; return default` | `x ?? default` |
| Ternary | `if c { x } else { y }` | `c ? x : y` |
| Tuple | Multiple returns or struct | `(a, b)` |
| Guard | `if result.IsErr() { ... }` | `guard x := expr else \|e\| { ... }` |

## Best Practices

### 1. Choose the Right Error Pattern

```dingo
// Use ? for Go's (T, error) functions
data := ioutil.ReadFile(path)?

// Use Result[T, E] for your own explicit error types
func Process() Result[Output, AppError] {
    return output  // or return appError
}
```

### 2. Prefer Option[T] Over *T for Optional Fields

```dingo
// ✅ Good - explicit optionality
type Config struct {
    Timeout Option[int]
}

// ❌ Avoid - nil pointer ambiguity
type Config struct {
    Timeout *int
}
```

### 3. Use Match for Complex Conditionals

```dingo
// ✅ Good - clear, exhaustive
priority := match event {
    PaymentFailed(_, _) => 1,
    OrderPlaced(_, amount, _) if amount > 500 => 2,
    _ => 3,
}

// ❌ Avoid - nested if-else chains
var priority int
if _, ok := event.(PaymentFailed); ok {
    priority = 1
} else if op, ok := event.(OrderPlaced); ok && op.Amount > 500 {
    priority = 2
} else {
    priority = 3
}
```

### 4. Lambda Style Consistency

Pick one style and stick to it within a project:
- **Rust-style** (`|x| expr`): More concise, familiar to Rust developers
- **TypeScript-style** (`x => expr`): Familiar to JavaScript/TypeScript developers

### 5. Combine Safe Navigation with Null Coalescing

```dingo
// ✅ Perfect combination
host := config?.Database?.Host ?? "localhost"
timeout := settings?.Connection?.Timeout ?? 30
```

## File Conventions

- Dingo files use `.dingo` extension
- Output Go files have `.go` extension in the same directory
- Import the dgo package for Option/Result: `"github.com/MadAppGang/dingo/pkg/dgo"`

## Compilation

```bash
# Transpile and build
dingo build myfile.dingo

# Transpile and run
dingo run myfile.dingo

# Transpile only (to .go)
dingo go myfile.dingo
```

## Summary

When writing Dingo code, always prefer:
1. `?` over manual error checking
2. `Option[T]` over `*T` for optional values
3. `Result[T, E]` over `(T, error)` for explicit error modeling
4. `enum` + `match` over interface + type switch
5. Lambdas over verbose `func` literals
6. `?.` + `??` over nested nil checks
7. Ternary over simple if-else expressions
8. `guard` over manual Result/Option unwrapping

These patterns make code more concise, safer, and easier to read while maintaining full Go compatibility.
