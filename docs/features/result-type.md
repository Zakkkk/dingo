# Result[T,E] Type

The `Result[T,E]` type is Dingo's primary error handling mechanism, inspired by Rust. It represents either a successful value (`Ok`) or an error (`Err`).

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
enum Result {
    Ok(float64),
    Err(error),
}

func divide(a: float64, b: float64) Result {
    if b == 0.0 {
        return Result_Err(errors.New("division by zero"))
    }
    return Result_Ok(a / b)
}
```

## Basic Usage

### Defining Result Types

Use the `enum` keyword to create Result types:

```go
package main

import "errors"

// Result with float64 value or error
enum Result {
    Ok(float64),
    Err(error),
}
```

### Creating Result Values

```go
// Success case
let success = Result_Ok(42.0)

// Error case
let failure = Result_Err(errors.New("something went wrong"))
```

### Checking Result Type

```go
if result.IsOk() {
    // Access the value
    let value = *result.ok
    println("Success:", value)
} else {
    // Access the error
    let err = result.err
    println("Error:", err.Error())
}
```

## Real-World Example

### File Processing

```go
package main

import (
    "errors"
    "os"
)

enum FileResult {
    Ok([]byte),
    Err(error),
}

func readConfig(path: string) FileResult {
    data, err := os.ReadFile(path)
    if err != nil {
        return FileResult_Err(err)
    }

    if len(data) == 0 {
        return FileResult_Err(errors.New("empty config file"))
    }

    return FileResult_Ok(data)
}

func main() {
    let result = readConfig("config.json")

    if result.IsOk() {
        println("Read", len(*result.ok), "bytes")
    } else {
        println("Error:", result.err.Error())
    }
}
```

### API Handler

```go
enum ApiResult {
    Ok(User),
    Err(string),
}

func fetchUser(id: int) ApiResult {
    if id <= 0 {
        return ApiResult_Err("invalid user ID")
    }

    // Simulate API call
    let user = User{ID: id, Name: "Alice"}
    return ApiResult_Ok(user)
}

func handleRequest(userID: int) {
    let result = fetchUser(userID)

    match result {
        case result.IsOk():
            let user = *result.ok
            println("Found user:", user.Name)
        default:
            let errMsg = result.err
            println("Error:", errMsg)
    }
}
```

## Working with the `?` Operator

The `?` operator works seamlessly with Result types for error propagation:

```go
func getUser(id: int) (User, error) {
    // Regular Go function returning (T, error)
    return User{ID: id}, nil
}

func processUser(id: int) (string, error) {
    let user = getUser(id)?  // Auto-unwrap or return error
    return user.Name, nil
}
```

See [error-propagation.md](./error-propagation.md) for details.

## Pattern Matching

Result types work perfectly with pattern matching:

```go
enum Result {
    Ok(int),
    Err(string),
}

func handleResult(r: Result) string {
    match r {
        Ok(value) => "Success: " + string(value),
        Err(msg) => "Error: " + msg
    }
}
```

See [pattern-matching.md](./pattern-matching.md) for advanced patterns.

## Generated Go Code

When you define a Result enum in Dingo:

```go
enum Result {
    Ok(float64),
    Err(error),
}
```

Dingo generates this idiomatic Go code:

```go
type ResultTag uint8

const (
    ResultTagOk ResultTag = iota
    ResultTagErr
)

type Result struct {
    tag   ResultTag
    ok    *float64
    err   *error
}

func ResultOk(ok float64) Result {
    return Result{tag: ResultTagOk, ok: &ok}
}

func ResultErr(err error) Result {
    return Result{tag: ResultTagErr, err: &err}
}

func (r Result) IsOk() bool {
    return r.tag == ResultTagOk
}

func (r Result) IsErr() bool {
    return r.tag == ResultTagErr
}
```

**Key points:**
- Clean, readable Go code
- Type-safe tagged union
- Zero runtime overhead
- Works with all Go tools

## Go Interoperability

### Calling Go Functions from Dingo

Go functions returning `(T, error)` can be used directly:

```go
import "os"

func readFile(path: string) ([]byte, error) {
    // Call Go standard library
    return os.ReadFile(path)
}
```

Use the `?` operator to propagate errors:

```go
func loadConfig() (Config, error) {
    let data = os.ReadFile("config.json")?
    let config = parseConfig(data)?
    return config, nil
}
```

### Calling Dingo from Go

Since Result types transpile to regular Go structs, Go code can use them:

```go
// In Go code
result := divide(10.0, 2.0)
if result.IsOk() {
    value := *result.ok
    fmt.Println("Result:", value)
}
```

## Best Practices

### 1. Use Descriptive Error Messages

```go
// Good
return Result_Err(errors.New("user not found: invalid ID"))

// Less helpful
return Result_Err(errors.New("error"))
```

### 2. Prefer Specific Result Types

```go
// Specific to domain
enum UserResult {
    Ok(User),
    Err(error),
}

// Can add more context
enum ApiResult {
    Success(User),
    NotFound,
    InvalidRequest(string),
    ServerError(error),
}
```

### 3. Chain Operations

```go
func pipeline(input: string) (Output, error) {
    let validated = validate(input)?
    let parsed = parse(validated)?
    let transformed = transform(parsed)?
    return transformed, nil
}
```

### 4. Document Error Cases

```go
// fetchUser retrieves a user by ID.
// Returns UserResult_Ok on success.
// Returns UserResult_Err if:
//   - ID is invalid (<=0)
//   - Database connection fails
//   - User not found
func fetchUser(id: int) UserResult {
    // ...
}
```

## Common Patterns

### Multiple Error Types

```go
enum DatabaseResult {
    Ok(User),
    NotFound,
    ConnectionError(error),
    ValidationError(string),
}

func getUser(id: int) DatabaseResult {
    if id <= 0 {
        return DatabaseResult_ValidationError("invalid ID")
    }

    user, err := db.Query(id)
    if err != nil {
        if isNotFound(err) {
            return DatabaseResult_NotFound()
        }
        return DatabaseResult_ConnectionError(err)
    }

    return DatabaseResult_Ok(user)
}
```

### Result with Data

```go
enum ParseResult {
    Ok(Config),
    Err(string),
}

func parseConfig(data: []byte) ParseResult {
    var config Config
    err := json.Unmarshal(data, &config)
    if err != nil {
        return ParseResult_Err("invalid JSON: " + err.Error())
    }
    return ParseResult_Ok(config)
}
```

### Validation Pipeline

```go
func validateAndSave(input: UserInput) (User, error) {
    let validated = validateEmail(input.Email)?
    let user = createUser(validated)?
    let saved = db.Save(user)?
    return saved, nil
}
```

## Limitations

### Current Limitations

1. **Type inference for Result types**: Currently requires explicit type annotations
2. **No generic Result[T,E]**: Must define specific Result types for each use case
3. **Pattern matching**: Available but requires explicit enum variant names

### Workarounds

**For generic Result:**

```go
// Define specific types as needed
enum IntResult {
    Ok(int),
    Err(error),
}

enum StringResult {
    Ok(string),
    Err(error),
}
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
enum OrderResult {
    Ok(Order),
    Err(error),
}

func processOrder(orderID: string) (Order, error) {
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
- [Go error handling proposal](https://github.com/golang/go/issues/32437) - Community discussion
- [Examples](../../tests/golden/) - Working Result examples in test suite
