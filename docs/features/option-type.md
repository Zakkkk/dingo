# Option[T] Type

The `Option[T]` type provides null safety by explicitly representing values that may or may not exist. It's Dingo's solution to Go's nil pointer problems.

## Design Decision: Generic Types via dgo Package

Dingo uses Go 1.18+ generics for Option types via the `dgo` runtime package:

```
Option[T]  → dgo.Option[T]   (single generic struct)
Some(val)  → dgo.Some(val)  (Go infers T from argument)
None[T]()  → dgo.None[T]()
```

**Why generics instead of code generation?**

1. **No code bloat** - One generic type serves all uses, vs generating
   `OptionString`, `OptionInt`, `OptionUser` for each type.

2. **Smaller binaries** - Generic types are instantiated by Go compiler
   at build time, with better dead code elimination.

3. **Better IDE support** - gopls understands `dgo.Option[T]` directly,
   providing accurate autocomplete and type checking.

4. **Cleaner output** - Generated `.go` files are minimal and readable.

**Trade-offs:**
- `None` requires explicit type: `None[string]()` vs Rust's `None`
- The dgo package becomes a runtime dependency (but it's tiny)

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
func findUser(id int) Option[User] {
    if id > 0 {
        return Some(User{ID: id})
    }
    return None[User]()
}

// Compiler forces you to handle None case!
```

## Basic Usage

### Writing Functions with Option Types

```go
package main

type UserSettings struct {
    Theme       Option[string]
    FontSize    Option[int]
    Language    Option[string]
    NotifyEmail Option[bool]
}

func FindUserByLanguage(users []User, lang string) Option[User] {
    for _, user := range users {
        if user.Settings.Language.IsSome() && user.Settings.Language.MustSome() == lang {
            return Some[User](user)
        }
    }
    return None[User]()
}
```

### Using Constructors

```go
// Value present - type inferred from argument
found := Some("User123")
number := Some(42)

// Value absent - explicit type required
notFound := None[string]()
noNumber := None[int]()

// With explicit types when needed
user := Some[User](User{ID: 1, Name: "Alice"})
```

### Checking Option State

```go
if option.IsSome() {
    value := option.MustSome()  // Go-style: panics if None
    println("Found:", value)
} else {
    println("Not found")
}

// Or use default values
value := option.SomeOr("default")
computed := option.SomeOrElse(func() string { return computeDefault() })
```

### Available Methods

The `dgo.Option[T]` type provides these methods:

```go
// Check state
option.IsSome() bool    // true if contains value
option.IsNone() bool    // true if empty

// Access values (Go-style naming, recommended)
option.MustSome() T     // Returns value, panics if None
option.SomeOr(def T) T  // Returns value or default
option.SomeOrElse(fn func() T) T  // Computes default lazily

// Access values (Rust-style aliases, deprecated)
option.Unwrap() T       // Alias for MustSome()
option.UnwrapOr(def T) T // Alias for SomeOr()
option.UnwrapOrElse(fn func() T) T // Alias for SomeOrElse()

// Transformations
option.Map(fn func(T) T) Option[T]       // Transform value if present
option.Filter(fn func(T) bool) Option[T] // Keep value only if predicate passes
option.AndThen(fn func(T) Option[T]) Option[T]  // Chain operations (flatMap)
option.OrElse(fn func() Option[T]) Option[T]    // Alternative if None

// Combining
option.And(other Option[T]) Option[T]  // Returns other if this is Some
option.Or(other Option[T]) Option[T]   // Returns other if this is None

// Panic with custom message
option.Expect("must have value") T  // Returns value or panics with message

// Take/Replace
option.Take() (T, Option[T])  // Returns value and new None
option.Replace(value T) (Option[T], T)  // Replaces value, returns old

// Convert to Result
option.OkOr(err error) Result[T, error]  // Convert to Result
option.OkOrElse(fn func() error) Result[T, error]  // Convert with computed error
```

### Combining Options

```go
// Zip two options together
combined := dgo.Zip(optionA, optionB)  // Option[struct{First A; Second B}]
if combined.IsSome() {
    pair := combined.MustSome()
    // use pair.First and pair.Second
}
```

## Real-World Example

### User Settings

```go
package main

import "fmt"

type UserSettings struct {
    Theme       Option[string]
    FontSize    Option[int]
    Language    Option[string]
    NotifyEmail Option[bool]
}

type User struct {
    ID       int
    Name     string
    Settings UserSettings
}

// GetTheme returns the user's theme or system default
func GetTheme(user User) string {
    return user.Settings.Theme.SomeOr("system")
}

// GetFontSize applies validation and returns CSS value
func GetFontSize(user User) string {
    if user.Settings.FontSize.IsSome() {
        size := user.Settings.FontSize.MustSome()
        if size < 10 {
            return "10px"
        }
        if size > 32 {
            return "32px"
        }
        return fmt.Sprintf("%dpx", size)
    }
    return "16px"
}

func main() {
    alice := User{
        ID:   1,
        Name: "Alice",
        Settings: UserSettings{
            Theme:    Some("dark"),
            FontSize: Some(18),
            Language: None[string](),  // Will use system language
        },
    }

    bob := User{
        ID:       2,
        Name:     "Bob",
        Settings: UserSettings{
            Theme:    None[string](),
            FontSize: None[int](),
            Language: Some("es"),
        },
    }

    fmt.Printf("Alice's theme: %s\n", GetTheme(alice))  // "dark"
    fmt.Printf("Bob's theme: %s\n", GetTheme(bob))      // "system"
}
```

## Generated Go Code

When you write Dingo code using `Option[T]`:

```go
// Dingo source
func FindUser(id int) Option[User] {
    if id <= 0 {
        return None[User]()
    }
    return Some(User{ID: id})
}
```

Dingo generates:

```go
// Generated Go code
import "github.com/MadAppGang/dingo/pkg/dgo"

func FindUser(id int) dgo.Option[User] {
    if id <= 0 {
        return dgo.None[User]()
    }
    return dgo.Some(User{ID: id})
}
```

**Key points:**
- Uses Go 1.18+ generics
- Single `dgo.Option[T]` type for all types
- Automatic import of dgo package
- Clean, readable output

## Pattern Matching

Option types work with pattern matching:

```go
func describe(opt Option[int]) string {
    match opt {
        Some(value) => "Found: " + string(value),
        None => "Nothing here"
    }
}
```

See [pattern-matching.md](./pattern-matching.md) for advanced patterns.

## Safe Navigation with Option Types

The safe navigation operator (`?.`) works with Option types:

```go
// Instead of verbose unwrapping
city := user?.address?.city?.name ?? "Unknown"
```

See [safe-navigation.md](./safe-navigation.md) for details.

## Null Coalescing

The null coalescing operator (`??`) provides elegant default values:

```go
port := config?.port ?? 8080
name := user?.name ?? user?.email ?? "Anonymous"
```

See [null-coalescing.md](./null-coalescing.md) for details.

## Go Interoperability

### From Go Pointers to Option

```go
// Go function returns *User
func getUserPtr(id int) *User {
    // ... returns nil or *User
}

// Wrap in Option
func getUserSafe(id int) Option[User] {
    ptr := getUserPtr(id)
    if ptr == nil {
        return None[User]()
    }
    return Some(*ptr)
}
```

### From Option to Go Pointers

```go
func convertToPtr(opt Option[User]) *User {
    if opt.IsSome() {
        user := opt.MustSome()
        return &user
    }
    return nil
}
```

### Using Go's sql.Null Types

```go
import "database/sql"

func convertNullString(ns sql.NullString) Option[string] {
    if ns.Valid {
        return Some(ns.String)
    }
    return None[string]()
}
```

### Calling Dingo from Go

Since Option uses `dgo.Option[T]`, Go code can use it directly:

```go
// In Go code
import "github.com/MadAppGang/dingo/pkg/dgo"

opt := FindUser(42)
if opt.IsSome() {
    user := opt.MustSome()
    fmt.Println("Found:", user.Name)
}
```

## Best Practices

### 1. Use Option for Truly Optional Values

```go
// Good: Config value may not exist
func getConfig(key string) Option[string]

// Bad: ID should never be optional
func getUserID(user User) Option[int]  // Just return int!
```

### 2. Provide Default Values

```go
func getPort() int {
    return getConfigValue("PORT").SomeOr(8080)
}
```

### 3. Document None Cases

```go
// FindUserByEmail searches for a user by email address.
// Returns Some(User) if user exists.
// Returns None if:
//   - Email not found in database
//   - Database connection error (consider using Result instead)
func FindUserByEmail(email string) Option[User]
```

### 4. Consider Result for Errors

If the "nothing" case represents an error, use Result instead:

```go
// Bad: Option doesn't distinguish errors from "not found"
func fetchUser(id int) Option[User]

// Good: Result shows WHY it failed
func fetchUser(id int) Result[User, DBError]
```

## Common Patterns

### First Element

```go
func first(items []string) Option[string] {
    if len(items) == 0 {
        return None[string]()
    }
    return Some(items[0])
}
```

### Find in Slice

```go
func findByName(users []User, name string) Option[User] {
    for _, user := range users {
        if user.Name == name {
            return Some(user)
        }
    }
    return None[User]()
}
```

### Map Lookup with Validation

```go
func getValidatedEnv(key string) Option[string] {
    value, exists := os.LookupEnv(key)
    if !exists || value == "" {
        return None[string]()
    }
    return Some(value)
}
```

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
func findUser(id int) Option[User] {
    if id <= 0 {
        return None[User]()
    }
    return Some(User{ID: id, Name: "Alice"})
}

user := findUser(42)
if user.IsSome() {
    println("User:", user.MustSome().Name)
} else {
    println("Not found")
}

// Or more concisely
println("User:", user.SomeOr(User{Name: "Guest"}).Name)
```

**Benefits:**
- No nil pointer panics
- Explicit handling required
- Self-documenting code
- Type-safe

## Gotchas

### 1. Accessing None Values

```go
opt := None[string]()

// BAD: Will panic!
value := opt.MustSome()

// GOOD: Always check first
if opt.IsSome() {
    value := opt.MustSome()
}

// BETTER: Use default values
value := opt.SomeOr("default")
```

### 2. None Requires Type Parameter

```go
// Go can't infer T from nothing, so type is required
return None[string]()  // Correct
return None()          // Won't compile
```

## See Also

- [Result Type](./result-type.md) - For error handling
- [Safe Navigation](./safe-navigation.md) - The `?.` operator
- [Null Coalescing](./null-coalescing.md) - The `??` operator
- [Pattern Matching](./pattern-matching.md) - Match on Option types
- [Sum Types](./sum-types.md) - General enum documentation

## Resources

- [Rust Option documentation](https://doc.rust-lang.org/std/option/) - Inspiration
- [dgo package source](../../pkg/dgo/option.go) - Runtime implementation
- [Examples](../../examples/03_option/) - Working Option examples
- [Billion-dollar mistake](https://www.infoq.com/presentations/Null-References-The-Billion-Dollar-Mistake-Tony-Hoare/) - Tony Hoare's apology for inventing null
