# Enum Variant Naming Convention Investigation for Dingo

## Background
Dingo is a Go transpiler that adds sum types through enum keywords. We need to establish clear naming conventions for enum variants that balance:

1. Language precedents from Rust, Swift, Kotlin, Haskell
2. Idiomatic Go compatibility
3. Pattern matching readability
4. Constructor clarity
5. Consistency with Dingo's type system

## Current Examples

```dingo
enum Result[T, E] {
    Ok(T)
    Err(E)
}

enum Option[T] {
    Some(T)
    None
}

enum Tree {
    Leaf(value int)
    Node(left *Tree, right *Tree)
}
```

## Investigation Tasks

### 1. Language Precedent Analysis
Analyze naming conventions from:
- **Rust**: How does Rust name enum variants? (e.g., Result::Ok, Option::Some)
- **Swift**: Swift's enum naming patterns
- **Kotlin**: Sealed class and enum naming
- **Haskell**: ADT constructor naming
- **TypeScript**: Union type discriminator naming

### 2. Go Compatibility Analysis
- How do variant names translate to Go struct/interface names?
- What are Go community preferences for sum type emulation?
- Review existing Go proposals for sum types (#19412)
- Analyze popular Go libraries that emulate sum types

### 3. Pattern Matching Implications
Show how different naming conventions affect readability:
```dingo
match result {
    Ok(value) => // ...
    Err(err) => // ...
}
```

vs alternatives like:
- `OK` / `ERR` (uppercase)
- `ok` / `err` (lowercase)
- `IsOk` / `IsErr` (verb prefix)

### 4. Constructor Usage Examples
Compare constructor clarity:
```dingo
// Current
let r = Ok(42)
let e = Err("failed")

// Alternatives?
let r = Result.Ok(42)
let r = NewOk(42)
let r = MakeOk(42)
```

### 5. Trade-off Analysis

Create a comparison table:

| Convention | Example | Pros | Cons | Go Translation |
|------------|---------|------|------|----------------|
| PascalCase | `Ok(v)` | Standard in Rust/Swift | May conflict with Go types | `type Ok struct` |
| UPPER_CASE | `OK(v)` | Clear constant-like | Not idiomatic | `type OK struct` |
| snake_case | `ok(v)` | Simple | Not Go-like | Would need mapping |
| Prefixed | `IsOk(v)` | Clear intent | Verbose | `type IsOk struct` |

### 6. Implementation Considerations

- Parser complexity for each approach
- Source map implications
- Error message clarity
- IDE autocomplete behavior

### 7. Final Recommendations

Provide:
1. **Primary recommendation** with rationale
2. **Naming rules** (when to use what case)
3. **Code style guide** with good/bad examples
4. **Migration path** if changing existing code
5. **Transpiler implementation notes**

## Specific Questions to Answer

1. Should Dingo follow Rust's PascalCase convention strictly?
2. How do we handle single-letter variants (E, T) vs words (Error, Success)?
3. Should constructors be namespaced (Result.Ok) or global (Ok)?
4. How do we ensure generated Go code remains idiomatic?
5. What about variants with no data (unit variants)?

## Expected Output

Comprehensive recommendation document with:
- Clear naming standard
- Code examples (good and bad)
- Implementation guide for transpiler
- Style guide for users
- Rationale for all decisions