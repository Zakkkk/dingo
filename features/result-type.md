# Result Type

**Priority:** P0 (Critical - Core MVP Feature)
**Status:** 🔴 Not Started
**Community Demand:** ⭐⭐⭐⭐⭐ (Highest - #1 in Go surveys)
**Inspiration:** Swift, Rust, Kotlin

---

## Overview

The `Result[T, E]` type is a discriminated union that represents either a successful value of type `T` or an error of type `E`. This eliminates Go's verbose `(value, error)` tuple pattern and provides type-safe error handling without exceptions.

## Motivation

### The Problem in Go

```go
// Verbose error handling in Go
func processUser(id string) (*User, error) {
    user, err := fetchUser(id)
    if err != nil {
        return nil, fmt.Errorf("fetch failed: %w", err)
    }

    validated, err := validateUser(user)
    if err != nil {
        return nil, fmt.Errorf("validation failed: %w", err)
    }

    saved, err := saveUser(validated)
    if err != nil {
        return nil, fmt.Errorf("save failed: %w", err)
    }

    return saved, nil
}
```

**Issues:**
- 60% of the function is error handling boilerplate
- Easy to forget error checks (silent bugs)
- No compiler enforcement of error handling
- Inconsistent error wrapping patterns

### Research Data

- **1.5-1.6%** of all Go code is `if err != nil` checks
- **200+ proposals** for error handling improvements
- **#1 complaint** in Go developer surveys (2020-2024)
- Go team moratorium on error syntax changes (Jan 2025) = opportunity for Dingo

---

## Proposed Syntax

### Type Definition

```dingo
// Dingo built-in type
enum Result[T, E] {
    Ok(T)
    Err(E)
}
```

### Usage Examples

```dingo
// Function returning Result
func fetchUser(id: string) -> Result[User, Error] {
    if !isValidID(id) {
        return Err(errors.New("invalid ID"))
    }

    user := database.query(id)
    return Ok(user)
}

// Consuming Result
func processUser(id: string) -> Result[User, Error] {
    result := fetchUser(id)

    match result {
        Ok(user) => {
            println("Got user: ${user.name}")
            return Ok(user)
        }
        Err(error) => {
            println("Error: ${error}")
            return Err(error)
        }
    }
}
```

### With Error Propagation (`?` operator)

```dingo
func processUser(id: string) -> Result[User, Error] {
    user := fetchUser(id)?           // Auto-return on Err
    validated := validateUser(user)? // Chain safely
    saved := saveUser(validated)?    // No boilerplate

    return Ok(saved)
}
```

---

## Transpilation Strategy

### Go Output

```go
// Transpiled to idiomatic Go
type ResultUserError struct {
    value *User
    err   error
}

func fetchUser(id string) ResultUserError {
    if !isValidID(id) {
        return ResultUserError{err: errors.New("invalid ID")}
    }

    user := database.query(id)
    return ResultUserError{value: &user}
}

func processUser(id string) ResultUserError {
    tmp, err := fetchUser(id)
    if err != nil {
        return ResultUserError{err: err}
    }
    user := tmp

    tmp1, err1 := validateUser(user)
    if err1 != nil {
        return ResultUserError{err: err1}
    }
    validated := tmp1

    tmp2, err2 := saveUser(validated)
    if err2 != nil {
        return ResultUserError{err: err2}
    }
    saved := tmp2

    return ResultUserError{value: &saved}
}
```

### Optimization Notes

- **Zero allocations** for error path (error is value type)
- **Single allocation** for success path (pointer to result)
- **Inlined** for small result types
- **Readable output** - looks like hand-written Go

---

## Inspiration from Other Languages

### Swift's Result Type

```swift
enum Result[Success, Failure: Error] {
    case success(Success)
    case failure(Failure)
}

// Usage
func fetchUser(id: String) -> Result[User, Error] {
    guard isValid(id) else {
        return .failure(ValidationError.invalidID)
    }
    return .success(user)
}

// With pattern matching
switch fetchUser("123") {
case .success(let user):
    print("Got: \(user)")
case .failure(let error):
    print("Error: \(error)")
}
```

**Key Takeaways:**
- Generic enum with associated values
- Type-safe error types (Failure must conform to Error)
- First-class pattern matching support
- Widely adopted in Swift ecosystem

### Kotlin's Sealed Result Class

```kotlin
sealed class Result[out T] {
    data class Success<out T>(val value: T) : Result[T]()
    data class Error(val message: String) : Result[Nothing]()
}

// Usage
fun fetchUser(id: String): Result[User] {
    return if (isValid(id)) {
        Result.Success(user)
    } else {
        Result.Error("Invalid ID")
    }
}

// With when expression
when (val result = fetchUser("123")) {
    is Result.Success -> println("Got: ${result.value}")
    is Result.Error -> println("Error: ${result.message}")
}
```

**Key Takeaways:**
- Sealed classes provide exhaustive checking
- Data classes provide automatic equality/toString
- When expressions force handling all cases
- Smart casts eliminate manual unwrapping

### Rust's Result Type

```rust
enum Result[T, E] {
    Ok(T),
    Err(E),
}

// Usage with ? operator
fn process_user(id: &str) -> Result[User, Error] {
    user := fetch_user(id)?;      // Early return on Err
    validated := validate(user)?;  // Chains elegantly
    saved := save(validated)?;     // No boilerplate
    Ok(saved)
}
```

**Key Takeaways:**
- `?` operator for error propagation (Dingo will adopt this)
- Zero-cost abstractions (no runtime overhead)
- Compiler-enforced error handling
- Most loved feature in Rust

---

## Method Naming Design

### Cross-Language Analysis

| Language | Get Value (panic) | Get or Default | Get or Compute |
|----------|-------------------|----------------|----------------|
| **Rust** | `unwrap()` | `unwrap_or(d)` | `unwrap_or_else(f)` |
| **Swift** | `!` operator | `??` operator | - |
| **Kotlin** | `!!` operator | `?:` elvis | - |
| **TypeScript** | - | `??` operator | - |

### Go Idioms

Go's `Must*` pattern in stdlib:
```go
template.Must(t, err)     // wraps (T, error) → T, panics on error
regexp.MustCompile(pat)   // wraps Compile, panics on error
```

Pattern: **Must{Operation}** for functions that panic.

### Dingo Naming Decision

We chose **variant-based naming** that references the actual type states:

| Method | Description | Style | Rationale |
|--------|-------------|-------|-----------|
| **MustOk()** | Returns value, panics if Err | Go | Matches variant name, Go's Must* convention |
| **MustErr()** | Returns error, panics if Ok | Go | Matches variant name, Go's Must* convention |
| **OkOr(default)** | Returns value or default | Go | Reads naturally: "ok or default" |
| **OkOrElse(fn)** | Computes value if Err | Go | Consistent with OkOr |
| Unwrap() | Alias for MustOk() | Rust | Deprecated, for Rust users |
| UnwrapOr() | Alias for OkOr() | Rust | Deprecated, for Rust users |
| UnwrapOrElse() | Alias for OkOrElse() | Rust | Deprecated, for Rust users |

### Why Not UnwrapOr?

1. **"Unwrap" is Rust terminology** - not idiomatic Go
2. **Doesn't reference type semantics** - Result has Ok/Err, not "wrap"
3. **OkOr reads naturally** - `result.OkOr(defaultValue)` = "ok or default"

### Parallel with Option

| Result[T, E] | Option[T] | Description |
|--------------|-----------|-------------|
| MustOk() | MustSome() | Panic if error/empty |
| MustErr() | - | Panic if success (Result only) |
| OkOr(d) | SomeOr(d) | Default if error/empty |
| OkOrElse(f) | SomeOrElse(f) | Compute if error/empty |

---

## Implementation Details

### Type System Integration

```dingo
// Result is a built-in generic sum type
enum Result[T, E] {
    Ok(T)    // Success variant with value
    Err(E)   // Error variant with error
}

// Compiler-generated methods
impl Result[T, E] {
    // Check if result is Ok
    func isOk() -> bool

    // Check if result is Err
    func isErr() -> bool

    // Unwrap value (panic if Err)
    func unwrap() -> T

    // Unwrap or return default
    func unwrapOr(default: T) -> T

    // Unwrap or compute default
    func unwrapOrElse(f: fn(E) -> T) -> T

    // Map the Ok value
    func map<U>(f: fn(T) -> U) -> Result[U, E]

    // Map the Err value
    func mapErr[F](f: fn(E) -> F) -> Result[T, F]
}
```

### Interop with Go

```dingo
// Automatic conversion from Go (value, error) tuples
goFunc := import("some/package").GoFunction

// Dingo wraps automatically
let result: Result[string, error] = goFunc.call("arg")

// Or explicit conversion
result := Result.fromGo(goFunc("arg"))

// Convert back to Go tuple
(value, err) = result.toGo()
```

### Error Type Constraints

```dingo
// Generic error type (any type)
func fetch<E>(id: string) -> Result[User, E]

// Constrained error type (must implement Error interface)
func fetch(id: string) -> Result[User, Error]

// Specific error type
func fetch(id: string) -> Result[User, UserError]
```

---

## Benefits

### Developer Experience

1. **Explicit error handling** - Result type forces consideration of errors
2. **Type-safe** - Compiler tracks error types through call chain
3. **Composable** - Chain operations with `?` operator
4. **Readable** - Intent is clear from function signature

### Code Quality

1. **Eliminate forgotten error checks** - Compiler forces handling
2. **Consistent error handling** - One pattern for all errors
3. **Better error context** - Error types carry full context
4. **Testable** - Easy to test both success and error paths

### Compatibility

1. **Zero runtime cost** - Transpiles to standard Go structs
2. **Go interop** - Seamless conversion to/from `(T, error)` tuples
3. **Standard library** - Works with all existing Go packages
4. **Incremental adoption** - Use only where it helps

---

## Tradeoffs

### Advantages over Go's approach
- ✅ Compiler-enforced error handling
- ✅ Eliminates `if err != nil` boilerplate
- ✅ Type-safe error propagation
- ✅ Better composability with `?` operator

### Potential Concerns
- ❓ Larger type signatures (mitigated by type inference)
- ❓ Memory overhead (1 pointer + 1 error vs Go's 1 value + 1 error)
- ❓ Learning curve (but familiar from Rust/Swift)

### Mitigation Strategies
- Type inference reduces verbosity: `result := fetch(id)` not `let result: Result[User, Error] = ...`
- Compiler optimizations can eliminate wrapper overhead
- Excellent documentation and examples
- Gradual adoption path (mix with Go error handling)

---

## Implementation Complexity

**Effort:** Medium
**Timeline:** 2-3 weeks for MVP

### Phase 1: Core Type (Week 1)
- [ ] Define Result enum in type system
- [ ] Implement basic transpilation to Go structs
- [ ] Add isOk/isErr/unwrap methods
- [ ] Write transpilation tests

### Phase 2: Pattern Matching (Week 2)
- [ ] Integrate with match expressions
- [ ] Add exhaustiveness checking
- [ ] Implement smart unwrapping
- [ ] Add error context tests

### Phase 3: Interop (Week 3)
- [ ] Auto-wrap Go (T, error) returns
- [ ] Add fromGo/toGo conversion helpers
- [ ] Test with real Go standard library
- [ ] Optimize generated code

### Phase 4: Polish
- [ ] Add helper methods (map, mapErr, etc.)
- [ ] Documentation and examples
- [ ] Performance benchmarks
- [ ] IDE autocomplete support

---

## Examples

### Example 1: HTTP Request

```dingo
func fetchJSON(url: string) -> Result[Response, HttpError] {
    resp := http.Get(url)?

    if resp.StatusCode != 200 {
        return Err(HttpError{
            code: resp.StatusCode,
            message: "HTTP error"
        })
    }

    return Ok(resp)
}

func main() {
    match fetchJSON("https://api.example.com/users") {
        Ok(resp) => println("Success: ${resp.Body}"),
        Err(error) => println("Failed: ${error.message}")
    }
}
```

### Example 2: File Operations

```dingo
func readConfig(path: string) -> Result[Config, IOError] {
    data := os.ReadFile(path)?
    config := json.Unmarshal(data)?
    return Ok(config)
}

func main() {
    config := readConfig("config.json")
        .unwrapOr(Config.default())

    println("Config loaded: ${config}")
}
```

### Example 3: Database Query

```dingo
func getUser(db: Database, id: int) -> Result[User, DbError] {
    row := db.QueryRow("SELECT * FROM users WHERE id = ?", id)?

    user := User{}
    row.Scan(&user.ID, &user.Name, &user.Email)?

    return Ok(user)
}

// Chain multiple operations
func getUserWithPosts(db: Database, id: int) -> Result[UserWithPosts, DbError] {
    user := getUser(db, id)?
    posts := getPosts(db, user.ID)?

    return Ok(UserWithPosts{user, posts})
}
```

---

## Success Criteria

- [ ] Result type works in 100% of Go error handling scenarios
- [ ] Transpiled code has zero runtime overhead vs hand-written Go
- [ ] Error propagation (`?`) reduces error handling code by 60%+
- [ ] Full interop with Go standard library and packages
- [ ] Positive feedback from 10+ early users
- [ ] Comprehensive test coverage (>90%)

---

## References

- Go Proposals: #32437 (try), #71203 (? operator)
- Swift Result: https://developer.apple.com/documentation/swift/result
- Rust Result: https://doc.rust-lang.org/std/result/
- Kotlin Result: https://kotlinlang.org/api/latest/jvm/stdlib/kotlin/-result/
- Research: [../ai-docs/research/golang_missing/](../ai-docs/research/golang_missing/)

---

## Next Steps

1. Review this proposal with community
2. Prototype basic Result type transpilation
3. Design `?` operator syntax (see [error-propagation.md](./error-propagation.md))
4. Implement pattern matching integration (see [pattern-matching.md](./pattern-matching.md))
5. Test with real-world Go codebases
