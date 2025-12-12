# Enum Variant Naming Convention Analysis for Dingo

## Executive Summary

After analyzing language precedents, Go community preferences, and Dingo's current implementation, I recommend maintaining **PascalCase** for enum variants with specific refinements for Go compatibility. This approach balances familiar syntax from modern languages with Go's idiomatic patterns while ensuring clean transpilation and pattern matching readability.

## 1. Language Precedent Analysis

### Rust
- **Convention**: PascalCase for variants (`Ok`, `Err`, `Some`, `None`)
- **Rationale**: Variants are types/constructors, not values
- **Usage**: `Result::Ok(value)`, `Option::Some(42)`
- **Pattern Matching**: `match result { Ok(x) => ..., Err(e) => ... }`

### Swift
- **Convention**: lowerCamelCase for enum cases
- **Rationale**: Cases are values, not types
- **Usage**: `.success(value)`, `.failure(error)`
- **Pattern Matching**: `switch result { case .success(let x): ... }`
- **Note**: Swift uses dot syntax for enum access

### Kotlin
- **Convention**: UPPER_CASE for simple enums, PascalCase for sealed classes
- **Usage**: `Success(data)`, `Error(message)`
- **Pattern Matching**: `when (result) { is Success -> ... }`

### Haskell
- **Convention**: PascalCase for constructors
- **Usage**: `Just 42`, `Nothing`, `Left "error"`, `Right value`
- **Rationale**: Constructors are functions that create types

### TypeScript
- **Convention**: Mixed (no strong standard)
- **Common**: PascalCase for discriminated unions
- **Usage**: `{ kind: 'Success', value }`, `{ kind: 'Error', error }`

## 2. Go Compatibility Analysis

### Current Go Patterns for Sum Type Emulation

1. **Interface-based (most common)**
```go
type Result interface {
    isResult()
}
type Ok struct { Value interface{} }
type Err struct { Error error }
func (Ok) isResult() {}
func (Err) isResult() {}
```

2. **Tagged struct pattern (Dingo's approach)**
```go
type ResultTag uint8
const (
    ResultTag_Ok ResultTag = iota
    ResultTag_Err
)
type Result struct {
    tag ResultTag
    ok_0 *interface{}
    err_0 *error
}
```

3. **Go Proposal #19412 (Sum Types)**
- Community's most requested feature (996+ 👍)
- Proposed syntax: `type Result[T any] Ok(T) | Err(error)`
- Variants would likely be PascalCase to match Go conventions

### Go Naming Conventions
- **Exported types**: PascalCase (`UserAccount`, `HTTPClient`)
- **Unexported types**: camelCase (`userAccount`, `httpClient`)
- **Constants**: PascalCase or UPPER_CASE (`MaxRetries`, `DEFAULT_TIMEOUT`)
- **Interfaces**: PascalCase with -er suffix (`Reader`, `Stringer`)

## 3. Pattern Matching Readability Comparison

### PascalCase (Current - Recommended)
```dingo
match result {
    Ok(value) => println("Success: ", value),
    Err(err) => println("Error: ", err),
}

match tree {
    Leaf(val) => val,
    Node(left, right) => process(left) + process(right),
}
```
**Pros**: Clear, follows Rust/Haskell precedent, visually distinct from variables
**Cons**: None significant

### lowerCase
```dingo
match result {
    ok(value) => println("Success: ", value),
    err(err) => println("Error: ", err),
}
```
**Pros**: More "Go-like" for unexported elements
**Cons**: Confusing with variables, breaks precedent, harder to spot patterns

### UPPER_CASE
```dingo
match result {
    OK(value) => println("Success: ", value),
    ERR(err) => println("Error: ", err),
}
```
**Pros**: Very clear these are constants/constructors
**Cons**: Shouting, not idiomatic in any major language for this use case

### Prefixed (IsOk, NewOk)
```dingo
match result {
    IsOk(value) => println("Success: ", value),
    IsErr(err) => println("Error: ", err),
}
```
**Pros**: Very Go-like (follows IsXxx pattern)
**Cons**: Verbose, implies boolean check not pattern match

## 4. Constructor Usage Analysis

### Current Approach (Global Functions)
```dingo
r := Ok(42)
e := Err("failed")
s := Some("hello")
n := None
```
Transpiles to:
```go
r := Result_int_error_Ok(42)
e := Result_int_error_Err(errors.New("failed"))
s := Option_string_Some("hello")
n := Option_string_None()
```

### Alternative: Namespaced
```dingo
r := Result.Ok(42)
e := Result.Err("failed")
```
**Pros**: Clearer namespace, avoids name conflicts
**Cons**: More verbose, requires parser changes

### Alternative: Factory Functions
```dingo
r := NewOk(42)
e := NewErr("failed")
```
**Pros**: Very Go-idiomatic
**Cons**: Loses elegance, longer names

## 5. Go Translation Mapping

| Dingo | Go Generated | Notes |
|-------|--------------|-------|
| `enum Status { Active, Pending }` | `type StatusTag uint8`<br>`const StatusTag_Active ...` | Tag enum pattern |
| `Ok(42)` | `Result_T_E_Ok(42)` | Type-parameterized constructor |
| `Some(x)` | `Option_T_Some(x)` | Generic constructor |
| `None` | `Option_T_None()` | Unit variant |
| `match x { Ok(v) => ... }` | `switch { case x.IsOk(): ... }` | Pattern to method |

## 6. Implementation Trade-offs

### PascalCase (Recommended) ✅
**Pros:**
- Aligns with Rust/Haskell (proven precedent)
- Clear distinction: variants are types/constructors
- Natural for pattern matching
- Exports properly in Go (public API)
- Already implemented and working

**Cons:**
- May conflict with user-defined Go types (manageable with prefixes)

### lowerCamelCase
**Pros:**
- Follows Swift convention
- Differentiates from Go types

**Cons:**
- Unexported in Go (requires workarounds)
- Less familiar to Rust developers
- Harder to distinguish in patterns

### Special Cases to Consider

1. **Single-letter variants**: Keep as-is (`T`, `E` in generics)
2. **Acronyms**: Follow Go style (`HTTP`, not `Http`)
3. **Unit variants**: Same rules (`None`, not `NONE`)

## 7. Final Recommendations

### Primary Recommendation: PascalCase with Smart Prefixing

1. **Default Convention**: PascalCase for all variants
   ```dingo
   enum Status { Active, Pending, Complete }
   enum Result[T, E] { Ok(T), Err(E) }
   ```

2. **Transpilation Strategy**:
   - Generate prefixed names to avoid conflicts
   - Use enum name as prefix for tags: `StatusTag_Active`
   - Use full type signature for generics: `Result_int_error_Ok`

3. **Naming Rules**:
   - Variants: Always PascalCase (`Ok`, `Some`, `Active`)
   - Generated Go types: Add enum prefix (`Status_Active`)
   - Tag constants: EnumTag_Variant (`StatusTag_Active`)
   - Constructors: Full signature (`Result_T_E_Ok`)

4. **Pattern Matching**: Keep readable syntax
   ```dingo
   match status {
       Active => "working",
       Pending => "waiting",
       Complete => "done",
   }
   ```

### Style Guide Examples

**Good:**
```dingo
enum Color { Red, Green, Blue }
enum Result[T, E] { Ok(T), Err(E) }
enum Tree { Leaf(int), Node(left: Tree, right: Tree) }
```

**Bad:**
```dingo
enum Color { RED, GREEN, BLUE }        // Don't use UPPER_CASE
enum Result[T, E] { ok(T), err(E) }    // Don't use lowerCase
enum Tree { IsLeaf(int), IsNode(...) } // Don't add prefixes
```

### Migration Path

Since Dingo is pre-1.0:
1. Current code already follows PascalCase ✅
2. No migration needed
3. Document convention clearly in style guide
4. Enforce via linter (future enhancement)

### Transpiler Implementation Notes

1. **Parser**: No changes needed (already accepts PascalCase)
2. **Code Generation**: Current prefixing strategy is good
3. **Error Messages**: Ensure they use original Dingo names
4. **Source Maps**: Map variant names correctly
5. **IDE Support**: Autocomplete should show PascalCase variants

## 8. Comparison with Go Proposal #19412

The Go proposal for sum types (if accepted) would likely use:
```go
type Result[T any] Ok(T) | Err(error)
```

Our Dingo syntax aligns well:
```dingo
enum Result[T, E] { Ok(T), Err(E) }
```

This means Dingo code could potentially migrate smoothly if Go ever adds native sum types.

## 9. Real-World Library Examples

### Popular Go Libraries Using Sum Type Patterns

1. **github.com/pkg/errors**
   - Uses interface-based approach
   - Constructors: `New()`, `Wrap()`, `Cause()`
   - Would benefit from: `enum Error { Base(string), Wrapped(error, string) }`

2. **database/sql**
   - `NullString`, `NullInt64`, etc.
   - Natural fit for Option: `Option[string]`, `Option[int64]`

3. **encoding/json**
   - `RawMessage` for deferred parsing
   - Could use: `enum JsonValue { Parsed(T), Raw([]byte) }`

### If These Libraries Used Dingo

```dingo
// pkg/errors equivalent
enum Error {
    Simple(message: string),
    Wrapped(cause: Error, message: string),
    WithStack(err: Error, stack: []Frame),
}

// database/sql equivalent
enum NullableValue<T> {
    Present(T),
    Null,
}

// Better than sql.NullString
let name: Option[string] = row.getName()
match name {
    Some(n) => println("Name: ", n),
    None => println("Name is NULL"),
}
```

## Conclusion

PascalCase for enum variants is the right choice for Dingo because:

1. **Precedent**: Matches Rust, Haskell, and likely future Go
2. **Clarity**: Variants are constructors/types, not values
3. **Compatibility**: Exports properly in generated Go
4. **Readability**: Clear in pattern matching
5. **Simplicity**: Already implemented and tested
6. **Go Integration**: Maps naturally to how Go libraries emulate sum types

The current implementation is correct. We should document this convention clearly and ensure all examples follow it consistently.