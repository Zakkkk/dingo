# Enum Variant Naming Conventions Recommendations

## Overview
Dingo's enum sum type implementation currently uses verbose naming patterns (`ShapeTag_Circle`) that violate Go naming conventions and reduce developer ergonomics. This document provides comprehensive recommendations for improving enum variant naming to align with Go idioms and improve pattern matching usability.

## Current Implementation Analysis

### Problems with Current Pattern
The preprocessor's current approach generates:
- **Tag constants**: `ShapeTag_Circle`, `ShapeTag_Square`
- **Struct fields**: `circle_radius`, `square_side`
- **Go code**: Verbose, unnatural names that feel foreign to Go developers

**Issues**:
1. **Underscore naming**: Against Go conventions (only used in special cases like `context.WithValue`)
2. **Verbose**: 15+ characters instead of 5-8
3. **Non-idiomatic**: Doesn't match any Go standard library patterns
4. **Poor DX**: Makes pattern matching cumbersome

### Code Example - Current vs Proposed

**Current (Bad)**:
```go
if shape.tag == ShapeTag_Circle {
    radius := *shape.circle_radius
} else if shape.tag == ShapeTag_Square {
    side := *shape.square_side
}
```

**Proposed (Good)**:
```go
if shape.tag == Circle {
    radius := *shape.Radius
} else if shape.tag == Square {
    side := *shape.Side
}
```

## Language Precedents Analysis

### Rust Conventions
**Pattern**: `enum Color { Red, Green, Blue }`
- PascalCase for enum name and variants
- No prefixes or suffixes
- Clean, modern sum type naming
- Influences many newer language designs

### Swift Conventions  
**Pattern**: `enum Color { case red, green, blue }`
- camelCase for variants (optionals, context-dependent)
- Makes variants look more like function names
- Good for optional values and result types

### Haskell Conventions
**Pattern**: `data Color = Red | Green | Blue | RGB Int Int Int`
- PascalCase for both enum and variants
- Clean algebraic data types
- Influences functional language designs

### Kotlin Conventions
**Pattern**: `enum class Color { RED, GREEN, BLUE }`
- Screaming caps identical to Java
- Traditional OOP-style enum naming
- Maintains Java interoperability

### Go Community Context
- **Sum types not native**: No existing conventions to follow
- **Existing constants**: `time.Monday`, `net.FlagUp`, `syscall.O_RDONLY`
- **Pattern observations**: 
  - Simple names for common constants (`Monday`, not `TimeWeekdayMonday`)
  - Namespaced only when risking collisions (`http.MethodGet`)
  - Screaming caps reserved for system-level (`O_RDONLY`, not business logic)

## Go Ecosystem Patterns

Analyzing Go standard library reveals clear naming patterns for enumerations:

### Primary Pattern: Minimal Exported Constants
```go
// time package
const (
    Sunday Weekday = iota
    Monday
    Tuesday
    // ... full pattern
)

// net package  
const (
    FlagUp Flag = 1 << iota
    FlagBroadcast
    FlagLoopback
)
```

**Characteristics**:
- Single word, exported PascalCase
- No redundant prefixes matching type name
- Package scoping handles collisions
- Minimal typing burden

### Collision Avoidance Patterns
```go
// http package - namespaced to avoid conflicts with SQL methods
const (
    MethodGet     = "GET"
    MethodPost    = "POST"
    MethodPut     = "PUT"
    MethodDelete  = "DELETE"
    // ...
)

// syscall package - screaming caps for low-level constants
const (
    O_RDONLY = iota
    O_WRONLY
    O_RDWR
)
```

### Go Sum Types Proposal Insights
Go proposal #19412 (996 👍) shows community interest in sum types but provides no naming guidance:

```go
// Example syntax from proposal discussions
type OptionalInt nil | int      // builtin nil handling
type Reader io.Reader | io.ReadCloser  // interface composition
```

The proposal focuses on syntax but implies variants should be type names, suggesting simple naming would be preferred.

## idiomatic Go Compatibility Recommendations

For Dingo enums, adopt the primary Go pattern of simple exported constants:

### Recommended Pattern
```go
// Generated Go code
type ShapeTag uint8
const (
    Circle ShapeTag = iota  // ✅ Clean, Go-like
    Square
    Triangle  
)

// NOT: ShapeTag_Circle  ❌ Verbose, unidiomatic
```

### Field Naming
```go
// Recommended: PascalCase for pointer-typed variant fields
type Shape struct {
    tag       ShapeTag
    Radius    *float64  // ✅ PascalCase (exported pointers)
    Side      *float64
    Base      *float64
    Height    *float64
}

// NOT: circle_radius  ❌ camelCase with underscores
```

## Pattern Matching Readability

Simple constant names dramatically improve pattern matching ergonomy:

### Match Expression Examples

**Current Verbose Pattern**:
```go
match result {
    Ok(x) => println!("Success: {}", x),
    Err(e) => println!("Error: {}", e),
}
```

**Constructor Usage**:
```go
let success = Ok(42);        // ✅ Already good
let error = Err("failed");   // ✅ Already good
```

### Switch Statement Examples
The naming directly affects all pattern matching:

```go
// Before (cumbersome)
switch result.tag {
case ResultTag_Ok:
    x := *result.ok_value
case ResultTag_Err:
    e := result.err_value
}

// After (natural)  
switch result.tag {
case Ok:
    x := result.Value  // PascalCase field
case Err:
    e := result.Error  // PascalCase field
}
```

## Constructor Clarity Considerations

Good naming enables clean constructor functions following Go conventions:

### Recommended Constructor API
```go
// Package-level constructor functions
func Ok[T, E any](value T) Result[T, E] {
    return Result[T, E]{tag: Ok, value: &value}
}

// OR: Type method constructors  
func (Result[T, E]) Ok(value T) Result[T, E] {
    return Result[T, E]{tag: Ok, value: &value}
}
```

This allows usage like:
```go
result := Ok(42)           // ✅ Intuitive
shape := Circle(5.0)       // ✅ Clean
```

## Consistency with Dingo Type System

Dingo's standard library types should maintain consistent naming:

### Result[T, E] Variants
```go
enum Result[T, E] {
    Ok[T],     // ✅ Consistent with Rust/Swift expectations
    Err[E]     // ✅ Standard error terminology
}
```

### Option[T] Variants  
```go
enum Option[T] {
    Some[T],   // ✅ Widest language precedent
    None       // ✅ Haskell/Rust/Swift standard
}
```

Maintains familiarity for developers coming from other languages.

## Trade-offs Between Approaches

### Alternative 1: Go-Idiomatic (RECOMMENDED)
- **Pattern**: `Circle`, `Square`
- **Pros**: Natural Go, minimal typing, standard library aligned
- **Cons**: Package-level constants (not type-scoped)
- **Usage Risk**: Rare collision potential in large packages

### Alternative 2: Namespaced PascalCase
- **Pattern**: `ShapeCircle`, `ShapeSquare`  
- **Pros**: Clear namespacing, collision-free
- **Cons**: Verbose, doesn't match Go patterns
- **Usage Risk**: Feels unidiomatic

### Alternative 3: Screaming Caps
- **Pattern**: `CIRCLE`, `SQUARE`
- **Pros**: Seen in systems code, collision-free
- **Cons**: Violates quiet typing, feels aggressive
- **Usage Risk**: Non-modern Go style

### Transpiler Implementation Considerations
- **Simple regex replacement**: Current `{Enum}Tag_{Variant}` → `{Variant}`
- **Field name updating**: `variant_field` → `Field`
- **Backward compatibility**: Breaking change but appropriate pre-1.0
- **Golden tests**: All need regeneration (~20 enum test files)
- **Parser updates**: AST references need adjustment

## Final Recommendations

### 1. Adopt Go-Idiomatic Constants
**Primary Recommendation**: Replace the current verbose pattern with simple PascalCase constants like `time.Monday`.

This provides the best blend of:
- Go developer familiarity
- Minimal typing burden  
- Pattern matching ergonomics
- Ecosystem alignment

### 2. PascalCase Fields for Variants
Use PascalCase for struct fields containing variant data (since they're pointer-typed), following Go exported field conventions.

### 3. Keep Standard Library Consistency
Maintain familiar naming for `Result[T,E]` (`Ok`, `Err`) and `Option[T]` (`Some`, `None`) to align with language precedents.

### 4. Migration Strategy
1. Update preprocessor unit: Change enum generation logic
2. Regenerate golden tests: Apply new naming across all enum tests
3. Update documentation examples
4. Provide migration guide for any existing Dingo code

### Implementation Timeline
- **Phase 4.2.1**: Update preprocessor enum generation
- **Phase 4.2.2**: Regenerate affected golden tests  
- **Phase 4.2.3**: Update documentation and examples
- **Phase 5**: Integrate with broader language improvements

## Rationale Summary
The current `ShapeTag_Circle` pattern creates friction for Go developers and doesn't align with any established Go idioms. Adopting `Circle` style naming brings Dingo enums in line with Go's most common constant naming patterns, improving developer experience and reducing keystrokes in pattern matching scenarios.

This change prioritizes Go idiomaticity over strict namespacing, trusting Go's package system for collision management—as the standard library consistently does.