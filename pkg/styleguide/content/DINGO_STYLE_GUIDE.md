# Dingo Style Guide

**Version**: 0.12.1
**Last Updated**: 2026-01-09

A comprehensive guide to writing idiomatic Dingo code, covering best practices, common patterns, and pitfalls to avoid.

---

## Table of Contents

- [Part 1: Quick Reference](#part-1-quick-reference)
  - [Feature Summary](#feature-summary)
  - [Syntax Cheat Sheet](#syntax-cheat-sheet)
  - [Common Mistakes](#common-mistakes)
- [Part 2: Feature Reference](#part-2-feature-reference)
  - [1. Error Propagation (?)](#1-error-propagation-)
  - [2. Result Type](#2-result-type)
  - [3. Option Type](#3-option-type)
  - [4. Guard Statements](#4-guard-statements)
  - [5. Pattern Matching](#5-pattern-matching)
  - [6. Lambdas](#6-lambdas)
  - [7. Safe Navigation](#7-safe-navigation)
  - [8. Null Coalescing](#8-null-coalescing)
- [Part 3: Go Patterns Improved](#part-3-go-patterns-improved)
  - [Error Handling Patterns](#error-handling-patterns)
  - [Optional Value Patterns](#optional-value-patterns)
  - [Data Processing Patterns](#data-processing-patterns)
- [Part 4: Layer Patterns](#part-4-layer-patterns)
  - [Repository Layer](#repository-layer)
  - [Service Layer](#service-layer)
  - [Handler Layer](#handler-layer)

---

# Part 1: Quick Reference

## Feature Summary

| Feature | Syntax | Use Case | Go Equivalent |
|---------|--------|----------|---------------|
| Error Propagation | `expr?` | Propagate errors | `if err != nil { return err }` |
| Error Context | `expr? "message"` | Wrap errors | `fmt.Errorf("message: %w", err)` |
| Result Type | `Result[T, E]` | Type-safe errors | `(T, error)` |
| Option Type | `Option[T]` | Optional values | `*T` (nil pointer) |
| Guard | `guard x := expr else \|err\| { }` | Unwrap or exit | `if err != nil { ... }` |
| Match | `match expr { Pat => val }` | Pattern match | `switch` |
| Lambda | `\|x\| expr` or `x => expr` | Inline function | `func(x T) T { return expr }` |
| Safe Nav | `x?.y` | Null-safe access | `if x != nil { x.y }` |
| Null Coalesce | `a ?? b` | Default value | `if a != nil { a } else { b }` |

## Syntax Cheat Sheet

### Error Propagation
```dingo
// Basic propagation
value := fallibleCall()?

// With error context
value := fallibleCall() ? "operation failed"

// With error transform (Rust-style)
value := fallibleCall() ? |err| CustomError{cause: err}

// With error transform (TS-style)
value := fallibleCall() ? err => CustomError{cause: err}
```

### Result/Option Constructors
```dingo
// Result
Ok[T, E](value)       // Success
Err[T, E](error)      // Failure

// Option
Some(value)           // Has value
None[T]()             // No value
```

### Result/Option Methods
```dingo
// Checking state
result.IsOk()         // true if Ok
result.IsErr()        // true if Err
option.IsSome()       // true if Some
option.IsNone()       // true if None

// Extracting values (may panic)
result.MustOk()       // Get value or panic
result.MustErr()      // Get error or panic
option.MustSome()     // Get value or panic

// Safe extraction
result.OkOr(default)       // Value or default
result.OkOrElse(fn)        // Value or compute
option.SomeOr(default)     // Value or default
option.SomeOrElse(fn)      // Value or compute
```

### Guard Statements
```dingo
// With error binding
guard value := expr else |err| { return err }

// Without error binding
guard value := expr else { return defaultValue }

// Assignment (existing variable)
guard value = expr else |err| { return err }
```

### Pattern Matching
```dingo
match result {
    Ok(value) => processValue(value),
    Err(err) => handleError(err),
}

match option {
    Some(value) => useValue(value),
    None => getDefault(),
}
```

## Common Mistakes

### ❌ Using `?` on functions returning only `error`
```dingo
// WRONG: tx.Commit() returns error, not (T, error)
tx.Commit()?

// RIGHT: Use explicit error handling
if err := tx.Commit(); err != nil {
    return Err[T, error](err)
}
```

### ❌ Field assignment with `?`
```dingo
// WRONG: Transpiler issues with field assignment
user.Token = generateToken()?

// RIGHT: Use intermediate variable
token := generateToken()?
user.Token = token
```

### ❌ Implicit error in guard block
```dingo
// WRONG: Compile error - implicit err
guard user := FindUser(id) else { return err }

// RIGHT: Explicit binding required
guard user := FindUser(id) else |err| { return err }
```

### ❌ Lambda without type annotation
```dingo
// WRONG: Cannot infer type for 'p'
names := dgo.Map(products, |p| p.Name)

// RIGHT: Add type annotation
names := dgo.Map(products, |p Product| p.Name)
```

### ❌ Mixing Result and (T, error) incorrectly
```dingo
// WRONG: Can't use ? on Go's (T, error)
data := os.ReadFile(path)?

// RIGHT: Wrap Go function result
data, err := os.ReadFile(path)
if err != nil {
    return Err[[]byte, error](err)
}
```

---

# Part 2: Feature Reference

## 1. Error Propagation (?)

The `?` operator propagates errors from `Result[T, E]` types, eliminating verbose `if err != nil` patterns.

### Syntax

```dingo
// Basic: propagate error unchanged
value := fallibleCall()?

// With string context: wraps with fmt.Errorf
value := fallibleCall() ? "descriptive message"

// With lambda transform: custom error handling
value := fallibleCall() ? |err| transformError(err)
value := fallibleCall() ? err => transformError(err)
```

### When to Use

✅ **Use `?` when:**
- Chaining multiple fallible operations
- Error should bubble up unchanged
- Function returns `Result[T, E]`
- You want concise error propagation

```dingo
// GOOD: Clean chain of operations
func processOrder(id string) Result[Order, error] {
    order := fetchOrder(id)?
    validated := validateOrder(order)?
    paid := processPayment(validated)?
    return Ok[Order, error](paid)
}
```

### When NOT to Use

❌ **Don't use `?` when:**
- Function returns only `error` (not `(T, error)`)
- Need custom error handling per call
- At package boundary (prefer explicit handling)
- Single fallible call (overhead not worth it)

```dingo
// BAD: Single call, no benefit
func simple(id string) Result[User, error] {
    return fetchUser(id)  // Just return directly, don't use ?
}

// BAD: Function returns only error
func commit(tx *sql.Tx) Result[Unit, error] {
    tx.Commit()?  // Won't work - Commit returns error only
}
```

### Go Equivalent

```dingo
// Dingo
user := fetchUser(id)?
```

```go
// Transpiled Go
tmp, err := fetchUser(id)
if err != nil {
    return Result{Err: &err}
}
user := tmp
```

### Error Context Examples

```dingo
// String context - wraps with fmt.Errorf
config := loadConfig(path) ? "failed to load config"

// Lambda context - custom error type
user := findUser(id) ? |e| UserNotFoundError{ID: id, Cause: e}

// Lambda with logging
data := fetchData(url) ? |err| {
    log.Error("fetch failed", "url", url, "err", err)
    return ServiceError{Op: "fetch", Err: err}
}
```

---

## 2. Result Type

`Result[T, E]` represents either a success value (`Ok`) or an error (`Err`).

### Import

```dingo
import "github.com/MadAppGang/dingo/pkg/dgo"
```

### Constructors

```dingo
// Success
result := dgo.Ok[User, error](user)

// Failure
result := dgo.Err[User, error](fmt.Errorf("not found"))
```

### Methods Reference

| Method | Description | Panics? |
|--------|-------------|---------|
| `IsOk()` | Returns true if Ok | No |
| `IsErr()` | Returns true if Err | No |
| `MustOk()` | Returns value | Yes, if Err |
| `MustErr()` | Returns error | Yes, if Ok |
| `OkOr(default)` | Returns value or default | No |
| `OkOrElse(fn)` | Returns value or computed | No |
| `Map(fn)` | Transform Ok value | No |
| `MapErr(fn)` | Transform Err value | No |
| `AndThen(fn)` | Chain Result operations | No |
| `OrElse(fn)` | Recover from Err | No |

### When to Use

✅ **Use Result when:**
- Function can fail and caller must handle it
- Type-safe error handling is important
- Composing multiple fallible operations
- Internal APIs within your codebase

```dingo
// GOOD: Internal function with Result
func findUserByEmail(email string) Result[*User, error] {
    if !isValidEmail(email) {
        return Err[*User, error](ErrInvalidEmail)
    }
    user := db.FindByEmail(email)
    if user == nil {
        return Err[*User, error](ErrUserNotFound)
    }
    return Ok[*User, error](user)
}
```

### When NOT to Use

❌ **Don't use Result when:**
- At public API boundaries (use Go's `(T, error)`)
- Simple functions that rarely fail
- Performance-critical hot paths
- Interfacing with Go standard library

```dingo
// BAD: Public API should use Go convention
func (s *UserService) GetUser(id string) Result[*User, error] { }

// GOOD: Public API with Go convention
func (s *UserService) GetUser(id string) (*User, error) { }
```

### Go Interop

```dingo
// Calling Go function that returns (T, error)
data, err := os.ReadFile(path)
if err != nil {
    return Err[[]byte, error](err)
}
result := Ok[[]byte, error](data)

// Converting Result to Go tuple
func ToGoTuple[T, E any](r Result[T, E]) (T, E) {
    if r.IsOk() {
        return r.MustOk(), *new(E)
    }
    return *new(T), r.MustErr()
}
```

---

## 3. Option Type

`Option[T]` represents a value that may or may not be present.

### Constructors

```dingo
// Has value
opt := dgo.Some(user)

// No value
opt := dgo.None[User]()
```

### Methods Reference

| Method | Description | Panics? |
|--------|-------------|---------|
| `IsSome()` | Returns true if Some | No |
| `IsNone()` | Returns true if None | No |
| `MustSome()` | Returns value | Yes, if None |
| `SomeOr(default)` | Returns value or default | No |
| `SomeOrElse(fn)` | Returns value or computed | No |
| `Map(fn)` | Transform Some value | No |
| `Filter(pred)` | Keep if predicate true | No |
| `AndThen(fn)` | Chain Option operations | No |
| `OkOr(err)` | Convert to Result | No |

### When to Use

✅ **Use Option when:**
- Value is truly optional (not an error case)
- Configuration with defaults
- Optional function parameters
- Cache lookups (may or may not exist)

```dingo
// GOOD: Optional configuration
type Config struct {
    Theme    Option[string]  // Optional with default
    FontSize Option[int]     // Optional with default
}

func (c *Config) GetTheme() string {
    return c.Theme.SomeOr("light")
}
```

### When NOT to Use

❌ **Don't use Option when:**
- Absence indicates an error (use Result)
- Go's nil pointer is sufficient
- Performance-critical (Option has overhead)

```dingo
// BAD: Absence is an error, use Result
func findUser(id string) Option[*User] { }

// GOOD: Use Result for error cases
func findUser(id string) Result[*User, error] { }
```

---

## 4. Guard Statements

Guard statements unwrap `Result` or `Option` values with early exit on failure.

### Syntax

```dingo
// Declaration with error binding
guard value := resultExpr else |err| {
    // err is bound from Result's error
    return Err[T, error](err)
}

// Declaration without error binding
guard value := optionExpr else {
    return defaultValue
}

// Assignment to existing variable
var value T
guard value = resultExpr else |err| {
    return Err[T, error](err)
}
```

### When to Use

✅ **Use guard when:**
- Unwrapping Result/Option with early return
- Multiple sequential unwraps
- Want flattened code (not nested)

```dingo
// GOOD: Multiple guards keep code flat
func processTransaction(fromID, toID string, amount float64) Result[*Transaction, error] {
    guard from := findAccount(fromID) else |err| { return err }
    guard to := findAccount(toID) else |err| { return err }
    guard _ := validateBalance(from, amount) else |err| { return err }

    // All unwrapped, continue with business logic
    return transfer(from, to, amount)
}
```

### When NOT to Use

❌ **Don't use guard when:**
- Boolean condition (use regular `if`)
- Need error in scope after success
- Non-Result/Option types

```dingo
// BAD: Boolean condition
guard isValid := checkValid(x) else { return err }

// GOOD: Use regular if
if !checkValid(x) {
    return Err[T, error](ErrInvalid)
}
```

### Important Rules

1. **Explicit error binding required** when using error:
   ```dingo
   // WRONG
   guard user := findUser(id) else { return err }

   // RIGHT
   guard user := findUser(id) else |err| { return err }
   ```

2. **No binding for Option types** (they have no error):
   ```dingo
   // WRONG
   guard user := maybeUser else |val| { return nil }

   // RIGHT
   guard user := maybeUser else { return nil }
   ```

---

## 5. Pattern Matching

Match expressions provide exhaustive pattern matching on Result, Option, and enums.

### Syntax

```dingo
match expression {
    Pattern1 => result1,
    Pattern2 => result2,
    _ => defaultResult,  // Wildcard
}
```

### Result Matching

```dingo
match fetchUser(id) {
    Ok(user) => {
        log.Info("found user", user.Name)
        processUser(user)
    },
    Err(err) => {
        log.Error("user not found", err)
        handleError(err)
    },
}
```

### Option Matching

```dingo
match config.Theme {
    Some(theme) => applyTheme(theme),
    None => applyTheme("default"),
}
```

### When to Use

✅ **Use match when:**
- Need to handle all cases of Result/Option
- Complex branching logic
- Exhaustive handling required
- Binding values from variants

### When NOT to Use

❌ **Don't use match when:**
- Only checking one case (use `if`)
- Simple value comparison (use `switch`)
- Just need the value (use guard or `?`)

```dingo
// BAD: Only checking one case
match result {
    Ok(v) => process(v),
    _ => {},
}

// GOOD: Use if or guard
if result.IsOk() {
    process(result.MustOk())
}
```

---

## 6. Lambdas

Lambdas are inline anonymous functions with concise syntax.

### Syntax

```dingo
// Rust-style (pipe syntax)
|x| x * 2
|x, y| x + y
|x Type| x.Method()  // With type annotation

// TypeScript-style (arrow syntax)
x => x * 2
(x, y) => x + y
(x Type) => x.Method()  // With type annotation
```

### Type Inference

Lambda parameter types are inferred when possible:

```dingo
// Type inferred from context
numbers := []int{1, 2, 3}
doubled := dgo.Map(numbers, |n| n * 2)  // n inferred as int
```

### When Type Annotation is Required

```dingo
// REQUIRED: Generic function with interface result
products := []Product{...}
names := dgo.Map(products, |p Product| p.Name)  // Must specify Product

// REQUIRED: Ambiguous receiver type
items := dgo.Filter(mixed, |item Item| item.IsActive())
```

### When to Use

✅ **Use lambdas when:**
- Passing to higher-order functions (Map, Filter, etc.)
- Short inline logic
- Error transforms with `?`

### When NOT to Use

❌ **Don't use lambdas when:**
- Logic is complex (extract to named function)
- Need to reuse the function
- Debugging is needed (named functions easier)

---

## 7. Safe Navigation

Safe navigation (`?.`) accesses members only if the receiver is non-nil.

### Syntax

```dingo
// Safe field access
user?.Address?.City

// Safe method call
user?.GetProfile()?.Name
```

### When to Use

✅ **Use `?.` when:**
- Chain of optional accesses
- Don't need error info (just nil propagation)
- Quick nil-safe access

### When NOT to Use

❌ **Don't use `?.` when:**
- Need to know which part was nil
- Should handle nil as error
- Deep chains (hard to debug)

---

## 8. Null Coalescing

Null coalescing (`??`) provides a default value when the left side is nil/None.

### Syntax

```dingo
// With nil pointer
value := maybeNil ?? defaultValue

// With Option
value := option.SomeOr(defaultValue)  // Preferred for Option

// Chained
value := first ?? second ?? third ?? fallback
```

### When to Use

✅ **Use `??` when:**
- Simple default value needed
- nil pointer scenario
- Configuration defaults

### When NOT to Use

❌ **Don't use `??` when:**
- Default requires computation (use `??` with function or `SomeOrElse`)
- Need to know if value was nil
- Using Option type (prefer `.SomeOr()`)

---

# Part 3: Go Patterns Improved

## Error Handling Patterns

### Pattern: Multiple Fallible Operations

**Go (Traditional)**
```go
func processOrder(id string) (*Order, error) {
    order, err := fetchOrder(id)
    if err != nil {
        return nil, fmt.Errorf("fetch failed: %w", err)
    }

    validated, err := validateOrder(order)
    if err != nil {
        return nil, fmt.Errorf("validation failed: %w", err)
    }

    paid, err := processPayment(validated)
    if err != nil {
        return nil, fmt.Errorf("payment failed: %w", err)
    }

    return paid, nil
}
```

**Dingo (Improved)**
```dingo
func processOrder(id string) Result[*Order, error] {
    order := fetchOrder(id) ? "fetch failed"
    validated := validateOrder(order) ? "validation failed"
    paid := processPayment(validated) ? "payment failed"
    return Ok[*Order, error](paid)
}
```

**Reduction**: 16 lines → 5 lines (69% less code)

### Pattern: Transaction with Cleanup

**Go (Traditional)**
```go
func transfer(from, to string, amount float64) error {
    tx, err := db.Begin()
    if err != nil {
        return fmt.Errorf("begin tx: %w", err)
    }
    defer tx.Rollback()

    fromAcct, err := getAccount(tx, from)
    if err != nil {
        return fmt.Errorf("get from account: %w", err)
    }

    toAcct, err := getAccount(tx, to)
    if err != nil {
        return fmt.Errorf("get to account: %w", err)
    }

    if err := debit(tx, fromAcct, amount); err != nil {
        return fmt.Errorf("debit: %w", err)
    }

    if err := credit(tx, toAcct, amount); err != nil {
        return fmt.Errorf("credit: %w", err)
    }

    if err := tx.Commit(); err != nil {
        return fmt.Errorf("commit: %w", err)
    }

    return nil
}
```

**Dingo (Improved)**
```dingo
func transfer(from, to string, amount float64) Result[Unit, error] {
    tx := db.Begin() ? "begin tx"
    defer tx.Rollback()

    fromAcct := getAccount(tx, from) ? "get from account"
    toAcct := getAccount(tx, to) ? "get to account"

    debit(tx, fromAcct, amount) ? "debit"
    credit(tx, toAcct, amount) ? "credit"

    // Note: Commit returns only error, handle explicitly
    if err := tx.Commit(); err != nil {
        return Err[Unit, error](fmt.Errorf("commit: %w", err))
    }

    return Ok[Unit, error](Unit{})
}
```

## Optional Value Patterns

### Pattern: Configuration with Defaults

**Go (Traditional)**
```go
type Config struct {
    Theme    *string
    FontSize *int
    Debug    *bool
}

func (c *Config) GetTheme() string {
    if c.Theme != nil {
        return *c.Theme
    }
    return "light"
}

func (c *Config) GetFontSize() int {
    if c.FontSize != nil {
        return *c.FontSize
    }
    return 14
}
```

**Dingo (Improved)**
```dingo
type Config struct {
    Theme    Option[string]
    FontSize Option[int]
    Debug    Option[bool]
}

func (c *Config) GetTheme() string {
    return c.Theme.SomeOr("light")
}

func (c *Config) GetFontSize() int {
    return c.FontSize.SomeOr(14)
}
```

### Pattern: Null-Safe Navigation

**Go (Traditional)**
```go
func getUserCity(user *User) string {
    if user == nil {
        return ""
    }
    if user.Address == nil {
        return ""
    }
    if user.Address.City == nil {
        return ""
    }
    return *user.Address.City
}
```

**Dingo (Improved)**
```dingo
func getUserCity(user *User) string {
    return user?.Address?.City ?? ""
}
```

---

# Part 4: Layer Patterns

## Repository Layer

Repository functions interact with the database and return Results.

```dingo
type UserRepository struct {
    db *sql.DB
}

// FindByID retrieves a user by ID
func (r *UserRepository) FindByID(ctx context.Context, id uuid.UUID) Result[*User, error] {
    query := `SELECT id, email, name, created_at FROM users WHERE id = $1`

    var user User
    err := r.db.QueryRowContext(ctx, query, id).Scan(
        &user.ID, &user.Email, &user.Name, &user.CreatedAt,
    )

    if err == sql.ErrNoRows {
        return Err[*User, error](ErrUserNotFound)
    }
    if err != nil {
        return Err[*User, error](fmt.Errorf("query user: %w", err))
    }

    return Ok[*User, error](&user)
}

// Create inserts a new user
func (r *UserRepository) Create(ctx context.Context, user *User) Result[*User, error] {
    query := `INSERT INTO users (id, email, name) VALUES ($1, $2, $3) RETURNING created_at`

    err := r.db.QueryRowContext(ctx, query, user.ID, user.Email, user.Name).Scan(&user.CreatedAt)
    if err != nil {
        return Err[*User, error](fmt.Errorf("insert user: %w", err))
    }

    return Ok[*User, error](user)
}
```

## Service Layer

Service functions compose repository calls with business logic.

```dingo
type UserService struct {
    repo *UserRepository
}

// GetUserWithOrders retrieves a user with their orders
func (s *UserService) GetUserWithOrders(ctx context.Context, userID uuid.UUID) Result[*UserWithOrders, error] {
    // Use guard for sequential unwrapping
    guard user := s.repo.FindByID(ctx, userID) else |err| {
        return Err[*UserWithOrders, error](fmt.Errorf("find user: %w", err))
    }

    guard orders := s.orderRepo.FindByUserID(ctx, userID) else |err| {
        return Err[*UserWithOrders, error](fmt.Errorf("find orders: %w", err))
    }

    return Ok[*UserWithOrders, error](&UserWithOrders{
        User:   user,
        Orders: orders,
    })
}

// CreateUser creates a new user with validation
func (s *UserService) CreateUser(ctx context.Context, req CreateUserRequest) Result[*User, error] {
    // Validate using ?
    s.validateEmail(req.Email) ? "invalid email"
    s.validateName(req.Name) ? "invalid name"

    user := &User{
        ID:    uuid.New(),
        Email: req.Email,
        Name:  req.Name,
    }

    return s.repo.Create(ctx, user)
}
```

## Handler Layer

HTTP handlers convert between HTTP and domain types.

```dingo
type UserHandler struct {
    service *UserService
}

// GetUser handles GET /users/:id
func (h *UserHandler) GetUser(c *gin.Context) {
    id, err := uuid.Parse(c.Param("id"))
    if err != nil {
        c.JSON(400, gin.H{"error": "invalid user ID"})
        return
    }

    // Use match for explicit handling
    match h.service.GetUser(c.Request.Context(), id) {
        Ok(user) => c.JSON(200, user),
        Err(err) => {
            if errors.Is(err, ErrUserNotFound) {
                c.JSON(404, gin.H{"error": "user not found"})
            } else {
                log.Error("get user failed", "err", err)
                c.JSON(500, gin.H{"error": "internal error"})
            }
        },
    }
}

// CreateUser handles POST /users
func (h *UserHandler) CreateUser(c *gin.Context) {
    var req CreateUserRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": "invalid request"})
        return
    }

    match h.service.CreateUser(c.Request.Context(), req) {
        Ok(user) => c.JSON(201, user),
        Err(err) => {
            log.Error("create user failed", "err", err)
            c.JSON(500, gin.H{"error": err.Error()})
        },
    }
}
```

---

## Summary: When to Use What

| Scenario | Recommended Approach |
|----------|---------------------|
| Chain of 2+ fallible calls | `?` operator |
| Single fallible call | Direct return or explicit handling |
| Unwrap with early exit | `guard` statement |
| Handle all cases | `match` expression |
| Optional configuration | `Option[T]` with `.SomeOr()` |
| Null-safe access chain | `?.` navigation |
| Default for nil | `??` coalescing |
| Public API boundary | Go's `(T, error)` pattern |
| Internal functions | `Result[T, E]` |
| Transform collections | Lambda with `.Map()`, `.Filter()` |

---

## References

- Feature Documentation: `features/` directory
- API Reference: `pkg/dgo/`
- Examples: `examples/` directory
- Golden Tests: `tests/golden/`
