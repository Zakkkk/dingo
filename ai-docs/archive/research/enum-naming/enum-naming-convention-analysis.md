# Enum Variant Naming Convention Recommendations for Dingo

## Executive Summary

After analyzing language precedents, Go compatibility requirements, and pattern matching implications, **I recommend maintaining the current PascalCase convention** for enum variants in Dingo, with specific rules for translation to idiomatic Go code.

## Current Convention Analysis

Dingo currently uses:
- **Enum names**: PascalCase (e.g., `Status`, `Result`, `Option`)
- **Variant names**: PascalCase (e.g., `Pending`, `Active`, `Ok`, `Err`)
- **Go translation**: `EnumName_VariantName` pattern (e.g., `Status_Pending`)

## Language Precedent Analysis

### Rust (Primary Inspiration)
```rust
enum Result[T, E] {
    Ok(T),
    Err(E),
}
```
- Uses PascalCase for both enums and variants
- Constructors accessed as `Result::Ok(value)`
- Pattern matching: `Ok(x) => ...`

### Swift
```swift
enum CompassPoint {
    case north
    case south
}
```
- Enum: PascalCase
- Variants: camelCase
- Usage: `.north` (dot syntax)

### Kotlin
```kotlin
sealed class Result[T, E] {
    data class Success<T>(val value: T) : Result[T, Nothing]()
    data class Failure<E>(val error: E) : Result[Nothing, E]()
}
```
- Uses PascalCase for sealed class variants

### Haskell
```haskell
data Maybe a = Nothing | Just a
```
- PascalCase for constructors (traditional)

## Go Community Analysis

### Go Proposal #19412 (Sum Types)
- Most upvoted proposal in Go history (996+ 👍)
- Proposed syntax uses PascalCase variants:
```go
type Result[T any] interface {
    case Ok(T)
    case Err(error)
}
```

### Popular Go Libraries
- **pkg/errors**: Uses camelCase functions (`errors.Wrap`)
- **github.com/oklog/ulid**: PascalCase types (`ULID`)
- General Go pattern: PascalCase for exported types

## Pattern Matching Implications

### Current Implementation (Optimal)
```dingo
match result {
    Ok(value) => println!("Success: {}", value),
    Err(err) => println!("Error: {}", err),
}
```
Transpiles to:
```go
switch result.Tag() {
case Result_Ok:
    value := result.(Result_Ok[T, E]).Value
    fmt.Printf("Success: %v\n", value)
case Result_Err:
    err := result.(Result_Err[T, E]).Value
    fmt.Printf("Error: %v\n", err)
}
```

### Alternative Conventions (Problematic)

**UPPER_CASE**:
```dingo
match result {
    OK(value) => ...,  // Looks like constants
    ERR(err) => ...,
}
```
- ❌ Not idiomatic in any inspiration language
- ❌ Conflicts with Go constant conventions

**snake_case**:
```dingo
match result {
    ok(value) => ...,  // Looks like function calls
    err(err) => ...,
}
```
- ❌ Not Go-like
- ❌ Requires complex name mapping

## Recommended Naming Standard

### 1. Primary Rules

```
ENUM_NAME    ::= PascalCase
VARIANT_NAME ::= PascalCase
CONSTRUCTOR  ::= VariantName (bare, no namespace)
```

### 2. Examples

**Good** ✅:
```dingo
enum Status {
    Pending
    Active(since time.Time)
    Inactive(reason string)
}

s := Active(time.Now())

match s {
    Pending => println!("Waiting"),
    Active(t) => println!("Active since {}", t),
    Inactive(r) => println!("Inactive: {}", r),
}
```

**Bad** ❌:
```dingo
// Don't use snake_case
enum status {
    pending
    active
}

// Don't use UPPER_CASE
enum Status {
    PENDING
    ACTIVE
}

// Don't use verb prefixes
enum Status {
    IsPending
    IsActive
}
```

### 3. Special Cases

**Single-Letter Variants**: Keep PascalCase
```dingo
enum SimpleResult {
    S  // Success
    E  // Error
}
```

**Acronyms**: Follow Go conventions
```dingo
enum HTTPStatus {
    OK      // Not Ok
    NotFound
    InternalServerError
}
```

**Unit Variants**: No special treatment
```dingo
enum State {
    Start    // No parentheses needed
    Running
    Done
}
```

### 4. Constructor Rules

**Bare constructors** (current approach):
```dingo
r := Ok(42)          // Simple, clean
e := Err("failed")
s := Some(value)
n := None
```

**Why not namespaced?**
- `Result.Ok(42)` - More verbose, no clear benefit
- `Result::Ok(42)` - Rust syntax, foreign to Go
- Keep it simple and readable

### 5. Go Code Generation

Transform `EnumName` + `VariantName` → `EnumName_VariantName`:

```go
// Enum: Status, Variant: Active
type Status_Active struct {
    Since time.Time
}

func (Status_Active) StatusTag() StatusTag {
    return StatusTag_Active
}
```

## Implementation Guidelines

### 1. Parser/Preprocessor
- Enforce PascalCase in enum definitions
- Generate `EnumName_VariantName` for Go types
- Create constructor functions matching variant names

### 2. Error Messages
```
error: enum variant names must be PascalCase
  --> example.dingo:3:5
   |
3  |     pending  // should be 'Pending'
   |     ^^^^^^^
```

### 3. IDE Support
- Autocomplete should suggest PascalCase variants
- Quick fixes to correct casing

## Migration Path

Since Dingo is pre-1.0:
1. No backward compatibility needed
2. Update all tests to follow convention
3. Document in style guide
4. Enforce in linter (future)

## Rationale Summary

**Why PascalCase?**

1. **Consistency**: Matches Rust, Haskell, and proposed Go syntax
2. **Clarity**: Distinguishes types/constructors from functions
3. **Go Compatibility**: PascalCase is idiomatic for Go types
4. **Pattern Matching**: Reads naturally in match expressions
5. **Tooling**: Easy to parse and transform
6. **Community**: Aligns with Go proposal #19412

**Why not alternatives?**

- **snake_case**: Not Go-like, requires complex mapping
- **UPPER_CASE**: Looks like constants, not constructors
- **camelCase**: Would be inconsistent with enum name style
- **Verb prefixes**: Unnecessarily verbose

## Conclusion

The current PascalCase convention is optimal for Dingo. It balances:
- Language precedent (especially Rust)
- Go idioms and compatibility
- Pattern matching readability
- Implementation simplicity
- Future Go evolution alignment

No changes to the current naming convention are recommended.