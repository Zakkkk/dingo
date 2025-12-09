# Enum Variant Naming Convention Validation

## Context

Dingo is a meta-language for Go that provides sum types/enums. We need to validate our naming convention for enum variants.

## Current Implementation

### Enum Definition (Dingo syntax):
```dingo
enum Value {
    Int(int),
    String(string),
}
```

### Generated Go Code:
```go
type ValueTag uint8

const (
    ValueTag_Int ValueTag = iota
    ValueTag_String
)

type Value struct {
    tag      ValueTag
    int_0    *int
    string_0 *string
}

// Constructor functions
func Value_Int(arg0 int) Value {
    return Value{tag: ValueTag_Int, int_0: &arg0}
}

func Value_String(arg0 string) Value {
    return Value{tag: ValueTag_String, string_0: &arg0}
}
```

### Pattern Matching Usage:
```dingo
match v {
    Value_Int(n) if n > 0 => "Positive"
    Value_Int(n) if n < 0 => "Negative"
    Value_Int(_) => "Zero"
    Value_String(s) => "String value"
}
```

## Naming Convention Questions

1. **Variant Constructors**: `Value_Int`, `Value_String`
   - Uses underscore separator
   - Pattern: `{TypeName}_{VariantName}`

2. **Tag Constants**: `ValueTag_Int`, `ValueTag_String`
   - Pattern: `{TypeName}Tag_{VariantName}`

3. **Struct Fields**: `int_0`, `string_0`
   - Lowercase with underscore
   - Pattern: `{variant_lowercase}_{index}`

## Concerns

The user notes: "it doesn't look like Golang-native"

Go naming conventions typically use:
- **CamelCase** for exported identifiers: `ValueInt`, `ValueString`
- **mixedCase** for unexported: `valueInt`, `valueString`
- **No underscores** in standard library (with few exceptions)

## Alternative Naming Schemes

### Option A (Current - Underscore):
```go
Value_Int(42)           // Constructor
ValueTag_Int            // Tag constant
match v { Value_Int(n) => ... }
```

### Option B (Pure CamelCase):
```go
ValueInt(42)            // Constructor
ValueTagInt             // Tag constant  
match v { ValueInt(n) => ... }
```

### Option C (Namespaced):
```go
Value.Int(42)           // Constructor (requires different approach)
ValueTag.Int            // Tag constant
match v { Value.Int(n) => ... }
```

### Option D (All Lowercase):
```go
value_int(42)           // Constructor (unexported?)
VALUE_TAG_INT           // Tag constant (screaming snake?)
match v { value_int(n) => ... }
```

## Evaluation Criteria

1. **Go Idiomaticity**: Does it feel natural to Go developers?
2. **Clarity**: Is it clear what the identifier represents?
3. **Collision Avoidance**: Does it prevent naming conflicts?
4. **Pattern Matching Syntax**: Does it work well in match expressions?
5. **Type Safety**: Does it maintain clear type boundaries?
6. **Familiarity**: Does it match patterns from Rust/Swift/TypeScript users expect?

## Real-World Go Examples

**Similar patterns in Go standard library and popular packages:**

- `http.MethodGet`, `http.MethodPost` (CamelCase, no underscore)
- `ast.BadDecl`, `ast.GenDecl` (CamelCase type + variant)
- `token.ILLEGAL`, `token.IDENT` (ALL_CAPS for tokens)
- Error types: `io.EOF`, `sql.ErrNoRows` (CamelCase)

## Questions for External Models

1. **Which naming convention (A, B, C, or D) is most Go-idiomatic?**
2. **Are there better alternatives not listed above?**
3. **What are the trade-offs of each approach?**
4. **How do other Go libraries handle discriminated unions / sum types?**
5. **What would Go developers expect when using Dingo-generated code?**
6. **Does the underscore convention have any benefits that outweigh non-idiomaticity?**

## Context from Dingo Design Principles

From CLAUDE.md:
- **Full Compatibility**: Interoperate with all Go packages and tools  
- **Readable Output**: Generated Go should look hand-written
- **Simplicity**: Only add features that solve real pain points
- **IDE-First**: Maintain gopls feature parity

## Expected Output

Please provide:
1. **Recommendation**: Which naming convention to use (A, B, C, or D, or other)
2. **Rationale**: Why this choice is best for Go developers
3. **Trade-offs**: What we gain and lose with this choice
4. **Migration Path**: How to transition if we change from current (Option A)
5. **Edge Cases**: Any scenarios where the naming might cause issues

## Real Code Examples to Consider

```dingo
// Result type
enum Result[T, E] {
    Ok(T),
    Err(E),
}

// Option type  
enum Option[T] {
    Some(T),
    None,
}

// Custom domain enum
enum HttpStatus {
    Ok,
    NotFound,
    ServerError,
}
```

Should generate:
- `Result_Ok` vs `ResultOk`?
- `Option_Some` vs `OptionSome`?
- `HttpStatus_NotFound` vs `HttpStatusNotFound`?

Which feels more natural in Go code?
