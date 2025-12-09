# Option[T] Type

The `Option[T]` type provides null safety by explicitly representing values that may or may not exist. It's Dingo's solution to Go's nil pointer problems.

## Why Option Types?

Go's approach to nullable values uses pointers and nil:

```go
// Go
func findUser(id int) *User {
    if id > 0 {
        return &User{ID: id}
    }
    return nil  // Easy to forget checking!
}

user := findUser(42)
// Panic if user is nil!
println(user.Name)
```

**Problems:**
- Runtime panics from nil dereferences
- No compile-time enforcement
- Unclear if `nil` is valid

**Option type solution:**

```go
// Dingo
enum Option {
    Some(User),
    None,
}

func findUser(id: int) Option {
    if id > 0 {
        return Option_Some(User{ID: id})
    }
    return Option_None()
}

// Compiler forces you to handle None case!
```

## Basic Usage

### Defining Option Types

```go
package main

enum Option {
    Some(string),
    None,
}
```

### Creating Option Values

```go
// Value present
let found = Option_Some("User123")

// Value absent
let notFound = Option_None()
```

### Checking Option State

```go
if option.IsSome() {
    let value = *option.some
    println("Found:", value)
} else {
    println("Not found")
}
```

## Real-World Examples

### Database Lookup

```go
package main

import "database/sql"

enum UserOption {
    Some(User),
    None,
}

func findUserByEmail(email: string) UserOption {
    let row = db.QueryRow("SELECT id, name FROM users WHERE email = ?", email)

    var user User
    err := row.Scan(&user.ID, &user.Name)

    if err == sql.ErrNoRows {
        return UserOption_None()
    }

    if err != nil {
        // Could also return None for errors, or use Result type
        return UserOption_None()
    }

    return UserOption_Some(user)
}

func main() {
    let result = findUserByEmail("alice@example.com")

    if result.IsSome() {
        let user = *result.some
        println("Found user:", user.Name)
    } else {
        println("User not found")
    }
}
```

### Configuration Values

```go
enum ConfigOption {
    Some(string),
    None,
}

func getConfigValue(key: string) ConfigOption {
    value, exists := configMap[key]
    if !exists {
        return ConfigOption_None()
    }
    return ConfigOption_Some(value)
}

func main() {
    let apiKey = getConfigValue("API_KEY")

    if apiKey.IsSome() {
        println("Using API key:", *apiKey.some)
    } else {
        println("No API key configured")
    }
}
```

### Search Results

```go
enum SearchResult {
    Some(Product),
    None,
}

func searchProduct(query: string) SearchResult {
    products := database.Search(query)

    if len(products) == 0 {
        return SearchResult_None()
    }

    return SearchResult_Some(products[0])
}

func displayResult(query: string) {
    let result = searchProduct(query)

    if result.IsSome() {
        let product = *result.some
        println("Found:", product.Name, "-", product.Price)
    } else {
        println("No products found for:", query)
    }
}
```

## Pattern Matching

Option types work perfectly with pattern matching:

```go
enum Option {
    Some(int),
    None,
}

func describe(opt: Option) string {
    match opt {
        Some(value) => "Found: " + string(value),
        None => "Nothing here"
    }
}
```

See [pattern-matching.md](./pattern-matching.md) for advanced patterns.

## Generated Go Code

When you define an Option enum:

```go
enum Option {
    Some(string),
    None,
}
```

Dingo generates:

```go
type OptionTag uint8

const (
    OptionTagSome OptionTag = iota
    OptionTagNone
)

type Option struct {
    tag    OptionTag
    some   *string
}

func OptionSome(some string) Option {
    return Option{tag: OptionTagSome, some: &some}
}

func OptionNone() Option {
    return Option{tag: OptionTagNone}
}

func (o Option) IsSome() bool {
    return o.tag == OptionTagSome
}

func (o Option) IsNone() bool {
    return o.tag == OptionTagNone
}
```

**Features:**
- Type-safe tagged union
- Clean, idiomatic Go
- Zero allocation for None
- Compile-time exhaustiveness

## Go Interoperability

### From Go Pointers to Option

```go
// Go function returns *User
func getUserPtr(id int) *User {
    // ... returns nil or *User
}

// Wrap in Option
func getUserSafe(id: int) UserOption {
    let ptr = getUserPtr(id)
    if ptr == nil {
        return UserOption_None()
    }
    return UserOption_Some(*ptr)
}
```

### From Option to Go Pointers

```go
func convertToPtr(opt: UserOption) *User {
    if opt.IsSome() {
        user := *opt.some
        return &user
    }
    return nil
}
```

### Using Go's sql.Null Types

```go
import "database/sql"

func convertNullString(ns: sql.NullString) StringOption {
    if ns.Valid {
        return StringOption_Some(ns.String)
    }
    return StringOption_None()
}
```

## Best Practices

### 1. Use Option for Truly Optional Values

```go
// Good: Config value may not exist
func getConfig(key: string) StringOption

// Bad: ID should never be optional
func getUserID(user: User) IntOption  // Just return int!
```

### 2. Provide Default Values

```go
func getPort() int {
    let portOpt = getConfigValue("PORT")

    if portOpt.IsSome() {
        return parsePort(*portOpt.some)
    }

    return 8080  // sensible default
}
```

### 3. Document None Cases

```go
// findUserByEmail searches for a user by email address.
// Returns UserOption_Some if user exists.
// Returns UserOption_None if:
//   - Email not found in database
//   - Database connection error (consider using Result instead)
func findUserByEmail(email: string) UserOption {
    // ...
}
```

### 4. Consider Result for Errors

If the "nothing" case represents an error, use Result instead:

```go
// Bad: Option doesn't distinguish errors from "not found"
func fetchUser(id: int) UserOption

// Good: Result shows WHY it failed
enum UserResult {
    Ok(User),
    NotFound,
    DatabaseError(error),
}

func fetchUser(id: int) UserResult
```

## Common Patterns

### Map Lookup with Validation

```go
enum ValidatedValue {
    Some(string),
    None,
}

func getValidatedEnv(key: string) ValidatedValue {
    value, exists := os.LookupEnv(key)

    if !exists || value == "" {
        return ValidatedValue_None()
    }

    return ValidatedValue_Some(value)
}
```

### First Element

```go
func first(items: []string) StringOption {
    if len(items) == 0 {
        return StringOption_None()
    }
    return StringOption_Some(items[0])
}
```

### Find in Slice

```go
enum FoundUser {
    Some(User),
    None,
}

func findByName(users: []User, name: string) FoundUser {
    for _, user := range users {
        if user.Name == name {
            return FoundUser_Some(user)
        }
    }
    return FoundUser_None()
}
```

### Chained Lookups

```go
func getNestedValue(data: map[string]interface{}, keys: []string) Option {
    current := data

    for _, key := range keys {
        value, exists := current[key]
        if !exists {
            return Option_None()
        }

        nextMap, ok := value.(map[string]interface{})
        if !ok {
            return Option_None()
        }

        current = nextMap
    }

    return Option_Some(current)
}
```

## Safe Navigation with Option Types

The safe navigation operator (`?.`) makes working with Option types dramatically cleaner:

```go
// Instead of verbose unwrapping:
func getCityName(user: UserOption) string {
    if !user.IsSome() {
        return "Unknown"
    }
    u := *user.some
    if u.Address == nil {
        return "Unknown"
    }
    if u.Address.City == nil {
        return "Unknown"
    }
    return *u.Address.City
}

// Use safe navigation:
let city = user?.address?.city?.name ?? "Unknown"
```

### Property Access

```go
enum UserOption {
    Some(User),
    None,
}

type User struct {
    name    string
    email   string
    profile *Profile
}

// Access properties safely
let name = user?.name     // Returns Option[string]
let email = user?.email   // Returns Option[string]

// Chain through nested structures
let bio = user?.profile?.bio
```

### Method Calls

```go
type User struct {
    id int
}

func (u User) getName() string {
    return fmt.Sprintf("User-%d", u.id)
}

func (u User) getEmail(domain: string) string {
    return fmt.Sprintf("user%d@%s", u.id, domain)
}

// Call methods safely
let name = user?.getName()                    // Returns Option[string]
let email = user?.getEmail("example.com")    // Returns Option[string]

// Chain method calls
let formatted = user?.getName()?.toUpper()
```

### Combining with Null Coalescing

Safe navigation works perfectly with the `??` operator:

```go
// Provide defaults for missing values
let displayName = user?.name ?? "Guest"
let city = user?.address?.city ?? "Unknown"

// Multiple fallbacks
let theme = user?.settings?.theme ?? config?.defaultTheme ?? "light"

// Method results with defaults
let email = user?.getEmail("example.com") ?? "noreply@example.com"
```

**See [safe-navigation.md](./safe-navigation.md) for complete documentation.**

## Null Coalescing

The null coalescing operator (`??`) provides elegant default values for Option types:

```go
enum IntOption {
    Some(int),
    None,
}

let value: IntOption = IntOption_None()
let result = value ?? 42  // result is 42 (unwrapped)
```

### Basic Usage

```go
// Simple default
let port = config?.port ?? 8080

// Chained fallbacks
let timeout = user?.timeout ?? project?.timeout ?? 30

// With transformations
let displayName = user?.name ?? user?.email ?? "Anonymous"
```

### Type Handling

```go
// Option[T] → T (automatically unwrapped)
let opt: StringOption = StringOption_Some("hello")
let str: string = opt ?? "default"  // str is "hello"

// None case
let opt: StringOption = StringOption_None()
let str: string = opt ?? "default"  // str is "default"
```

### Real-World Example

```go
package main

enum UserOption {
    Some(User),
    None,
}

type User struct {
    name  string
    email string
}

func greetUser(user: UserOption) {
    // Elegant default handling
    let name = user?.name ?? user?.email ?? "Guest"
    println("Hello,", name)
}

func main() {
    // With user
    let user = UserOption_Some(User{name: "Alice", email: "alice@example.com"})
    greetUser(user)  // Hello, Alice

    // Without user
    let noUser = UserOption_None()
    greetUser(noUser)  // Hello, Guest
}
```

**See [null-coalescing.md](./null-coalescing.md) for complete documentation.**

## Migration from Go

### Before (Go)

```go
func findUser(id int) *User {
    if id <= 0 {
        return nil
    }
    return &User{ID: id, Name: "Alice"}
}

user := findUser(42)
if user != nil {
    fmt.Println("User:", user.Name)
} else {
    fmt.Println("Not found")
}
```

### After (Dingo)

```go
enum UserOption {
    Some(User),
    None,
}

func findUser(id: int) UserOption {
    if id <= 0 {
        return UserOption_None()
    }
    return UserOption_Some(User{ID: id, Name: "Alice"})
}

let user = findUser(42)
if user.IsSome() {
    println("User:", (*user.some).Name)
} else {
    println("Not found")
}
```

**Benefits:**
- No nil pointer panics
- Explicit handling required
- Self-documenting code
- Type-safe

## Gotchas

### 1. Accessing None Values

```go
let opt = Option_None()

// BAD: Will panic!
let value = *opt.some

// GOOD: Always check first
if opt.IsSome() {
    let value = *opt.some
}
```

### 2. Nested Options

```go
// Avoid this if possible
enum OptionOption {
    Some(Option),
    None,
}

// Better: Use Result or custom type
enum Nested {
    Found(User),
    Empty,
    Error(string),
}
```

## Limitations

### Current Limitations

1. **No safe navigation operator (`?.`)**: Coming soon
2. **No null coalescing (`??`)**: Coming soon
3. **Manual unwrapping**: Must explicitly check IsSome()
4. **No map/flatMap helpers**: Planned

### Workarounds

**For default values:**

```go
func getOrDefault(opt: StringOption, defaultVal: string) string {
    if opt.IsSome() {
        return *opt.some
    }
    return defaultVal
}

let value = getOrDefault(configOpt, "default")
```

## See Also

- [Result Type](./result-type.md) - For error handling
- [Pattern Matching](./pattern-matching.md) - Match on Option types
- [Sum Types](./sum-types.md) - General enum documentation
- [Error Propagation](./error-propagation.md) - The `?` operator

## Resources

- [Rust Option documentation](https://doc.rust-lang.org/std/option/) - Inspiration
- [Billion-dollar mistake](https://www.infoq.com/presentations/Null-References-The-Billion-Dollar-Mistake-Tony-Hoare/) - Tony Hoare's apology for inventing null
- [Examples](../../tests/golden/) - Working Option examples in test suite
