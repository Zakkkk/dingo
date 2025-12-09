# Result[T, E] Type

The `Result[T, E]` type is Dingo's primary error handling mechanism, inspired by Rust. It represents either a successful value (`Ok`) or an error (`Err`).

## Design Decision: Generic Types via dgo Package

Dingo uses Go 1.18+ generics for Result types via the `dgo` runtime package:

```
Result[T, E] → dgo.Result[T, E]  (single generic struct)
Ok(value)    → dgo.Ok[T, E](value)
Err(err)     → dgo.Err[T, E](err)
```

**Why generics instead of code generation?**

1. **No code bloat** - One generic type serves all uses, vs generating
   `ResultUserDBError`, `ResultBoolError`, etc. for each combination.

2. **Smaller binaries** - Generic types are instantiated by Go compiler
   at build time, with better dead code elimination.

3. **Better IDE support** - gopls understands `dgo.Result[T, E]` directly,
   no need for generated type definitions.

4. **Cleaner output** - Generated `.go` files are minimal and readable.

**Trade-offs:**
- Go generics require explicit type parameters on constructors in some
  contexts: `dgo.Ok[User, DBError](user)` vs Rust's `Ok(user)`
- The dgo package becomes a runtime dependency (but it's tiny)

## Why Result Types?

Traditional Go error handling uses `(T, error)` tuples:

```go
// Go
func divide(a, b float64) (float64, error) {
    if b == 0 {
        return 0, errors.New("division by zero")
    }
    return a / b, nil
}

result, err := divide(10, 2)
if err != nil {
    // handle error
}
// use result
```

**Problems:**
- Easy to forget checking `err`
- Can't enforce error checking at compile time
- Verbose boilerplate

**Result type solution:**

```go
// Dingo
func divide(a, b float64) Result[float64, string] {
    if b == 0.0 {
        return "division by zero"  // Implicit wrapping to Err
    }
    return a / b  // Implicit wrapping to Ok
}
```

## Basic Usage

### Writing Functions with Result Types

```go
package main

import "errors"

// Result with User value or DBError
func FindUserByID(db *sql.DB, id int) Result[User, DBError] {
    row := db.QueryRow("SELECT id, name FROM users WHERE id = ?", id)

    var user User
    err := row.Scan(&user.ID, &user.Name)
    if err == sql.ErrNoRows {
        return DBError{Code: "NOT_FOUND", Message: "user not found"}
    }
    if err != nil {
        return DBError{Code: "SCAN_ERROR", Message: err.Error()}
    }

    return user  // Implicit Ok wrapping
}
```

### Using Explicit Constructors

When type inference isn't available, use explicit constructors:

```go
// Explicit Ok with both type parameters
return Ok[User, DBError](user)

// Explicit Err (only T needs to be specified, E is inferred from argument)
return Err[User](DBError{Code: "NOT_FOUND"})
```

### Checking Result Type

```go
result := FindUserByID(db, 123)

if result.IsOk() {
    user := result.MustOk()  // Go-style: panics if Err
    fmt.Printf("Found: %s\n", user.Name)
} else {
    err := result.MustErr()
    fmt.Printf("Error: %s\n", err.Message)
}
```

### Available Methods

The `dgo.Result[T, E]` type provides these methods:

```go
// Check state
result.IsOk() bool      // true if Ok
result.IsErr() bool     // true if Err

// Access values (Go-style naming, recommended)
result.MustOk() T       // Returns Ok value, panics if Err
result.MustErr() E      // Returns Err value, panics if Ok
result.OkOr(def T) T    // Returns Ok value or default
result.OkOrElse(fn func(E) T) T  // Computes default from error

// Access values (Rust-style aliases, deprecated)
result.Unwrap() T       // Alias for MustOk()
result.UnwrapOr(def T) T // Alias for OkOr()
result.UnwrapErr() E    // Alias for MustErr()

// Transformations
result.Map(fn func(T) T) Result[T, E]     // Transform Ok value
result.MapErr(fn func(E) E) Result[T, E]  // Transform Err value
result.AndThen(fn func(T) Result[T, E]) Result[T, E]  // Chain operations
result.OrElse(fn func(E) Result[T, E]) Result[T, E]   // Error recovery

// Panic with custom message
result.Expect("must have user") T      // Returns Ok or panics with message
result.ExpectErr("should fail") E      // Returns Err or panics with message

// Pointer access
result.OkPtr() *T   // Returns pointer to Ok value (nil if Err)
result.ErrPtr() *E  // Returns pointer to Err value (nil if Ok)
```

## Real-World Example

### Database Repository

```go
package main

import (
    "database/sql"
    "fmt"
)

type User struct {
    ID    int
    Name  string
    Email string
}

type DBError struct {
    Code    string
    Message string
}

func (e DBError) Error() string {
    return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// FindUserByID returns a Result that explicitly models success or failure
func FindUserByID(db *sql.DB, id int) Result[User, DBError] {
    row := db.QueryRow("SELECT id, name, email FROM users WHERE id = ?", id)

    var user User
    err := row.Scan(&user.ID, &user.Name, &user.Email)
    if err == sql.ErrNoRows {
        return DBError{Code: "NOT_FOUND", Message: "user not found"}
    }
    if err != nil {
        return DBError{Code: "SCAN_ERROR", Message: err.Error()}
    }

    return user
}

// TransferFunds shows Result chaining
func TransferFunds(db *sql.DB, fromID, toID int, amount float64) Result[bool, DBError] {
    let fromResult = FindUserByID(db, fromID)
    if fromResult.IsErr() {
        return fromResult.MustErr()
    }

    let toResult = FindUserByID(db, toID)
    if toResult.IsErr() {
        return toResult.MustErr()
    }

    fmt.Printf("Transferring $%.2f from %s to %s\n",
        amount, fromResult.MustOk().Name, toResult.MustOk().Name)

    return true
}
```

## Generated Go Code

When you write Dingo code using `Result[T, E]`:

```go
// Dingo source
func FindUser(id int) Result[User, DBError] {
    if id <= 0 {
        return DBError{Code: "INVALID"}
    }
    return User{ID: id}
}
```

Dingo generates:

```go
// Generated Go code
import "github.com/MadAppGang/dingo/pkg/dgo"

func FindUser(id int) dgo.Result[User, DBError] {
    if id <= 0 {
        return dgo.Err[User](DBError{Code: "INVALID"})
    }
    return dgo.Ok[User, DBError](User{ID: id})
}
```

**Key points:**
- Uses Go 1.18+ generics
- Single `dgo.Result[T, E]` type for all combinations
- Automatic import of dgo package
- Clean, readable output

## Working with the `?` Operator

The `?` operator works with Result types for error propagation:

```go
func getUser(id int) (User, error) {
    // Regular Go function returning (T, error)
    return User{ID: id}, nil
}

func processUser(id int) (string, error) {
    let user = getUser(id)?  // Auto-unwrap or return error
    return user.Name, nil
}
```

See [error-propagation.md](./error-propagation.md) for details.

## Pattern Matching

Result types work with pattern matching:

```go
func handleResult(r Result[int, string]) string {
    match r {
        Ok(value) => "Success: " + string(value),
        Err(msg) => "Error: " + msg
    }
}
```

See [pattern-matching.md](./pattern-matching.md) for advanced patterns.

## Go Interoperability

### Calling Go Functions from Dingo

Go functions returning `(T, error)` can be used directly:

```go
import "os"

func loadConfig() (Config, error) {
    let data = os.ReadFile("config.json")?
    let config = parseConfig(data)?
    return config, nil
}
```

### Calling Dingo from Go

Since Result types use `dgo.Result[T, E]`, Go code can use them directly:

```go
// In Go code
import "github.com/MadAppGang/dingo/pkg/dgo"

result := FindUserByID(db, 123)
if result.IsOk() {
    user := result.MustOk()
    fmt.Println("Found:", user.Name)
}
```

## Best Practices

### 1. Use Descriptive Error Types

```go
// Good: Custom error type with context
type DBError struct {
    Code    string
    Message string
    Query   string  // What query failed
}

func FindUser(id int) Result[User, DBError]

// Less helpful: Generic error
func FindUser(id int) Result[User, error]
```

### 2. Implicit Wrapping for Clean Code

```go
// Clean: Let Dingo wrap automatically
func divide(a, b float64) Result[float64, string] {
    if b == 0 {
        return "division by zero"  // Implicit Err
    }
    return a / b  // Implicit Ok
}

// Explicit when needed
return Ok[float64, string](result)
return Err[float64]("explicit error")
```

### 3. Chain Operations with AndThen

```go
func processUser(id int) Result[string, DBError] {
    return FindUserByID(db, id).AndThen(func(user User) dgo.Result[string, DBError] {
        return dgo.Ok[string, DBError](user.Email)
    })
}
```

### 4. Document Error Cases

```go
// FindUser retrieves a user by ID.
// Returns Ok(User) on success.
// Returns Err(DBError) with codes:
//   - "NOT_FOUND": User doesn't exist
//   - "INVALID_ID": ID <= 0
//   - "CONN_ERROR": Database connection failed
func FindUser(id int) Result[User, DBError]
```

## Migration from Go

### Before (Go)

```go
func processOrder(orderID string) (*Order, error) {
    order, err := fetchOrder(orderID)
    if err != nil {
        return nil, fmt.Errorf("fetch failed: %w", err)
    }

    validated, err := validateOrder(order)
    if err != nil {
        return nil, fmt.Errorf("validation failed: %w", err)
    }

    return validated, nil
}
```

### After (Dingo)

```go
func processOrder(orderID string) (Order, error) {
    let order = fetchOrder(orderID)?
    let validated = validateOrder(order)?
    return validated, nil
}
```

**Benefits:**
- 60% less code
- Clearer intent
- Same safety guarantees
- No runtime overhead

## See Also

- [Error Propagation](./error-propagation.md) - The `?` operator
- [Option Type](./option-type.md) - For nullable values
- [Pattern Matching](./pattern-matching.md) - Match on Result types
- [Sum Types](./sum-types.md) - General enum documentation

## Resources

- [Rust Result documentation](https://doc.rust-lang.org/std/result/) - Inspiration for Dingo's Result
- [dgo package source](../../pkg/dgo/result.go) - Runtime implementation
- [Examples](../../examples/02_result/) - Working Result examples
