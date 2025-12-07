# Option Type

**Priority:** P0 (Critical - Core MVP Feature)
**Status:** 🔴 Not Started
**Community Demand:** ⭐⭐⭐⭐⭐ (Null safety highly requested)
**Inspiration:** Swift (Optional), Rust (Option), Scala

---

## Overview

The `Option<T>` type represents a value that may or may not be present. It eliminates Go's pervasive nil pointer bugs by making nullability explicit in the type system.

## Motivation

### The Problem in Go

```go
// Go's nil pointers are a billion-dollar mistake
func findUser(id string) *User {
    // Returns nil if not found - caller might forget to check
    return nil
}

func main() {
    user := findUser("123")
    println(user.Name)  // PANIC: nil pointer dereference
}

// Defensive nil checks everywhere
func processUser(user *User) error {
    if user == nil {
        return errors.New("user is nil")
    }
    if user.Address == nil {
        return errors.New("address is nil")
    }
    if user.Address.City == nil {
        return errors.New("city is nil")
    }
    // Finally do something...
}
```

**Problems:**
- Nil pointer panics are the #1 runtime error in Go
- No compile-time enforcement of nil checks
- Unclear API contracts (does this return nil?)
- Defensive coding leads to noise

### Research Data

- **Proposal #49202** - Nillability annotations (C#-style flow analysis)
- **37% of Go developers** impacted by nil-related bugs
- Tony Hoare called nil references his "billion-dollar mistake"

---

## Proposed Syntax

### Type Definition

```dingo
// Built-in generic sum type
enum Option<T> {
    Some(T)  // Value is present
    None     // Value is absent
}
```

### Basic Usage

```dingo
// Function returning optional value
func findUser(id: string) -> Option<User> {
    let user = database.query(id)
    if user.exists() {
        return Some(user)
    }
    return None
}

// Consuming Option with pattern matching
func processUser(id: string) {
    match findUser(id) {
        Some(user) => println("Found: ${user.name}"),
        None => println("User not found")
    }
}
```

### With Unwrapping Operators

```dingo
// Safe unwrap with default value
let user = findUser("123").unwrapOr(User.default())

// Safe unwrap with nil coalescing
let name = findUser("123")?.name ?? "Anonymous"

// Unsafe unwrap (panics if None)
let user = findUser("123").unwrap()  // Use sparingly

// Check if present
if findUser("123").isSome() {
    println("User exists")
}
```

---

## Transpilation Strategy

### Go Output

```go
// Option<T> transpiles to pointer + boolean
type OptionUser struct {
    value *User
    isSet bool
}

// Some constructor
func SomeUser(val User) OptionUser {
    return OptionUser{value: &val, isSet: true}
}

// None constructor
func NoneUser() OptionUser {
    return OptionUser{isSet: false}
}

// Usage
func findUser(id string) OptionUser {
    user := database.query(id)
    if user.exists() {
        return SomeUser(user)
    }
    return NoneUser()
}

func processUser(id string) {
    opt := findUser(id)
    if opt.isSet {
        user := *opt.value
        fmt.Printf("Found: %s\n", user.name)
    } else {
        fmt.Println("User not found")
    }
}
```

### Optimization Notes

- **Zero allocations** for None case (just bool = false)
- **Single allocation** for Some case (pointer to value)
- **Inlined** for primitive types (Option<int> → *int)
- **Compatible** with Go's nil (can convert)

---

## Inspiration from Other Languages

### Swift's Optional Type

```swift
// Built-in generic enum
enum Optional<Wrapped> {
    case some(Wrapped)
    case none
}

// Shorthand syntax with ?
var user: User? = findUser("123")

// Nil coalescing operator
let name = user?.name ?? "Anonymous"

// Optional binding
if let user = findUser("123") {
    print("Found: \(user.name)")
} else {
    print("Not found")
}

// Optional chaining
let city = user?.address?.city?.name
```

**Key Insights:**
- `Type?` sugar for `Optional<Type>`
- `?.` for safe member access (returns Optional)
- `??` for providing defaults
- `if let` for conditional unwrapping
- First-class language support

### Rust's Option Type

```rust
enum Option<T> {
    Some(T),
    None,
}

// Pattern matching
match find_user("123") {
    Some(user) => println!("Found: {}", user.name),
    None => println!("Not found"),
}

// Chaining with map/and_then
let user_name = find_user("123")
    .map(|u| u.name)
    .unwrap_or("Anonymous".to_string());

// Combining with Result
fn get_user(id: &str) -> Result<User, Error> {
    find_user(id).ok_or(Error::NotFound)
}
```

**Key Insights:**
- No null pointers in Rust at all
- Option is the only way to represent absence
- Rich combinator API (map, and_then, filter, etc.)
- Converts to/from Result

### Kotlin's Nullable Types

```kotlin
// Nullable types with ?
var user: User? = findUser("123")

// Safe call operator
val name = user?.name

// Elvis operator (null coalescing)
val name = user?.name ?: "Anonymous"

// Non-null assertion (!!. - discouraged)
val name = user!!.name  // Throws if null

// Safe cast
val user = obj as? User
```

**Key Insights:**
- `Type?` for nullable types
- `?.` for safe navigation
- `?:` (Elvis) for defaults
- Compiler tracks nullability through flow analysis

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
| **MustSome()** | Returns value, panics if None | Go | Matches variant name, Go's Must* convention |
| **SomeOr(default)** | Returns value or default | Go | Reads naturally: "some or default" |
| **SomeOrElse(fn)** | Computes value if None | Go | Consistent with SomeOr |
| Unwrap() | Alias for MustSome() | Rust | Deprecated, for Rust users |
| UnwrapOr() | Alias for SomeOr() | Rust | Deprecated, for Rust users |
| UnwrapOrElse() | Alias for SomeOrElse() | Rust | Deprecated, for Rust users |

### Why Not UnwrapOr?

1. **"Unwrap" is Rust terminology** - not idiomatic Go
2. **Doesn't reference type semantics** - Option has Some/None, not "wrap"
3. **SomeOr reads naturally** - `user.SomeOr(defaultUser)` = "some or default"

### Parallel with Result

| Option[T] | Result[T, E] | Description |
|-----------|--------------|-------------|
| MustSome() | MustOk() | Panic if empty/error |
| - | MustErr() | Panic if success |
| SomeOr(d) | OkOr(d) | Default if empty/error |
| SomeOrElse(f) | OkOrElse(f) | Compute if empty/error |

---

## Implementation Details

### Type System

```dingo
enum Option<T> {
    Some(T)
    None
}

// Compiler-generated methods
impl Option<T> {
    // Check if Some
    func isSome() -> bool

    // Check if None
    func isNone() -> bool

    // Get value (panic if None) - Go style
    func mustSome() -> T

    // Get value or return default - Go style
    func someOr(default: T) -> T

    // Get value or compute default - Go style
    func someOrElse(f: fn() -> T) -> T

    // Deprecated Rust-style aliases
    func unwrap() -> T           // Use mustSome()
    func unwrapOr(default: T) -> T    // Use someOr()
    func unwrapOrElse(f: fn() -> T) -> T  // Use someOrElse()

    // Map the Some value
    func map<U>(f: fn(T) -> U) -> Option<U>

    // Flat map for chaining
    func andThen<U>(f: fn(T) -> Option<U>) -> Option<U>

    // Filter based on predicate
    func filter(f: fn(T) -> bool) -> Option<T>

    // Convert to Result
    func okOr<E>(err: E) -> Result<T, E>
}
```

### Shorthand Syntax (Phase 2)

```dingo
// Longhand
let user: Option<User> = findUser(id)

// Shorthand (inspired by Swift/Kotlin)
let user: User? = findUser(id)

// Both are equivalent
```

### Safe Navigation (Phase 2)

```dingo
// Chaining with ?.
let city = user?.address?.city?.name  // Returns Option<string>

// With nil coalescing
let city = user?.address?.city?.name ?? "Unknown"

// Method calls
let email = user?.getEmail()?.lowercase()
```

### Interop with Go

```dingo
// Go functions returning *T or nil
let goFunc = import("some/package").MaybeNil

// Dingo wraps automatically
let result: Option<User> = goFunc.call()

// Explicit conversion
let opt = Option.fromPtr(goPtr)

// Convert back to Go pointer
let ptr = opt.toPtr()  // Returns nil if None
```

---

## Benefits

### Compile-Time Safety

```dingo
// ❌ This won't compile
let user: User = findUser(id)  // Error: Expected User, got Option<User>

// ✅ Must handle None case
let user: User = match findUser(id) {
    Some(u) => u,
    None => User.default()
}

// ✅ Or unwrap explicitly
let user: User = findUser(id).unwrapOr(User.default())
```

### Explicit APIs

```dingo
// Clear from signature: might not find user
func findUser(id: string) -> Option<User>

// Clear from signature: always returns user (or panics)
func getUser(id: string) -> User

// Clear from signature: returns user or error
func loadUser(id: string) -> Result<User, Error>
```

### Chaining

```dingo
// Chain operations safely
let city = findUser(id)
    .map(|u| u.address)
    .map(|a| a.city)
    .unwrapOr("Unknown")

// Compare to Go
user := findUser(id)
var city string
if user != nil && user.Address != nil && user.Address.City != nil {
    city = user.Address.City
} else {
    city = "Unknown"
}
```

---

## Tradeoffs

### Advantages
- ✅ **Eliminates nil panics** at compile time
- ✅ **Self-documenting** (APIs clearly show optionality)
- ✅ **Composable** (map, andThen, filter)
- ✅ **Zero runtime cost** (transpiles to pointer + bool)

### Potential Concerns
- ❓ **More typing** (Option<User> vs *User)
  - *Mitigation:* Shorthand syntax `User?` (Phase 2)
- ❓ **Learning curve** (new concept for Go developers)
  - *Mitigation:* Familiar from Swift/Kotlin/Rust
- ❓ **Wrapper overhead** (extra boolean field)
  - *Mitigation:* Compiler optimizes to just pointer for large types

---

## Implementation Complexity

**Effort:** Low-Medium
**Timeline:** 1-2 weeks

### Phase 1: Core Type (Week 1)
- [ ] Define Option enum in type system
- [ ] Implement transpilation to Go structs
- [ ] Add isSome/isNone/unwrap methods
- [ ] Pattern matching integration
- [ ] Tests for Some/None cases

### Phase 2: Combinators (Week 2)
- [ ] Add map/andThen/filter methods
- [ ] Implement unwrapOr/unwrapOrElse
- [ ] Add okOr for Result conversion
- [ ] Tests for chaining operations

### Phase 3: Interop
- [ ] Auto-wrap Go nullable pointers
- [ ] fromPtr/toPtr conversion
- [ ] Handle standard library integration
- [ ] Real-world compatibility tests

### Phase 4: Syntax Sugar (Future)
- [ ] Shorthand `T?` syntax
- [ ] Safe navigation `?.` operator
- [ ] Nil coalescing `??` operator
- [ ] IDE support for completions

---

## Examples

### Example 1: User Lookup

```dingo
func findUserByEmail(email: string) -> Option<User> {
    let users = database.query("SELECT * FROM users WHERE email = ?", email)
    if users.isEmpty() {
        return None
    }
    return Some(users[0])
}

func main() {
    match findUserByEmail("test@example.com") {
        Some(user) => {
            println("Found user: ${user.name}")
            sendEmail(user)
        },
        None => {
            println("User not found")
        }
    }
}
```

### Example 2: Configuration

```dingo
struct Config {
    port: Option<int>
    host: Option<string>
    timeout: Option<duration>
}

func loadConfig() -> Config {
    return Config{
        port: env.get("PORT").map(parseInt),
        host: env.get("HOST"),
        timeout: env.get("TIMEOUT").map(parseDuration)
    }
}

func main() {
    let config = loadConfig()

    // Use defaults for missing values
    let port = config.port.unwrapOr(8080)
    let host = config.host.unwrapOr("localhost")
    let timeout = config.timeout.unwrapOr(30 * time.Second)

    server.start(host, port, timeout)
}
```

### Example 3: Chaining Operations

```dingo
func getUserCity(userID: string) -> Option<string> {
    return findUser(userID)
        .andThen(|u| u.address)     // User might not have address
        .andThen(|a| a.city)        // Address might not have city
        .map(|c| c.name)            // Extract city name
}

// Using the value
let city = getUserCity("123").unwrapOr("Unknown location")
println("User is in: ${city}")
```

### Example 4: Database Queries

```dingo
func queryFirst<T>(sql: string, args: ...any) -> Option<T> {
    let rows = db.Query(sql, args...)
    if !rows.Next() {
        return None
    }

    let result: T
    rows.Scan(&result)
    return Some(result)
}

// Usage
let user = queryFirst<User>(
    "SELECT * FROM users WHERE id = ?",
    userID
).unwrapOr(User.guest())
```

---

## Integration with Result

```dingo
// Convert Option to Result
func getUser(id: string) -> Result<User, Error> {
    return findUser(id)
        .okOr(Error.notFound("user ${id} not found"))
}

// Convert Result to Option (discarding error)
func tryFindUser(id: string) -> Option<User> {
    return fetchUser(id).ok()  // Result<User, E> → Option<User>
}

// Combine both
func loadUserSafely(id: string) -> Option<User> {
    match fetchUser(id) {
        Ok(user) => Some(user),
        Err(_) => None
    }
}
```

---

## Success Criteria

- [ ] Option type works for all Go nullable scenarios
- [ ] Compile-time enforcement prevents nil pointer panics
- [ ] Transpiled code has zero overhead vs Go pointers
- [ ] Full interop with Go standard library
- [ ] Chaining operations (map/andThen) work correctly
- [ ] Pattern matching provides exhaustive checks
- [ ] Positive feedback from Swift/Rust developers

---

## References

- Swift Optional: https://developer.apple.com/documentation/swift/optional
- Rust Option: https://doc.rust-lang.org/std/option/
- Kotlin Null Safety: https://kotlinlang.org/docs/null-safety.html
- Go Proposal #49202: Nillability annotations
- Tony Hoare: "Null References: The Billion Dollar Mistake"

---

## Next Steps

1. Prototype Option enum implementation
2. Test transpilation to Go structs
3. Implement basic combinators (map, unwrapOr)
4. Design `?.` syntax (see [null-safety.md](./null-safety.md))
5. Plan `??` operator (see [null-coalescing.md](./null-coalescing.md))
