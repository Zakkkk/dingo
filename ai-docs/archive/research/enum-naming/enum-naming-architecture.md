# Enum Variant Naming Conventions Architecture

## Current Implementation

The current Dingo enum implementation generates Go code with this naming pattern:

- **Tag Type**: `{EnumName}Tag` (e.g., `ShapeTag`)
- **Enum Constants**: `{EnumName}Tag_{VariantName}` (e.g., `ShapeTag_Circle`)
- **Struct Fields**: `{lowercase_variant_name}_{field_name}` (e.g., `circle_radius`)

Example generated code:
```go
type ShapeTag uint8

const (
    ShapeTag_Circle ShapeTag = iota
    ShapeTag_Square
    ShapeTag_Rectangle
)

type Shape struct {
    tag ShapeTag
    circle_radius *float64
    square_side *float64
    rectangle_width *float64
    rectangle_height *float64
}
```

## Problems with Current Approach

1. **Violates Go Naming Conventions**: Underscore-separated names like `ShapeTag_Circle` are rare in Go, reserved for special cases
2. **Not Aligned with Go Ecosystem**: Standard library uses simple names like `time.Monday`, not `TimeWeekday_Monday`
3. **Verbose and Unnatural**: Feels clunky compared to Go's clean naming style
4. **Poor Ergonomics**: Longer names make pattern matching cumbersome
5. **Field Naming Issues**: `circle_radius` uses underscores inappropriately

## Language Precedents

**Rust**: Uses PascalCase enum variants (`Ok`, `Err`, `Some`, `None`)
**Kotlin**: Uses PascalCase with optional package scoping
**Swift**: Uses PascalCase variants (`success`, `failure`)
**Haskell**: Uses camelCase (`Just`, `Nothing`)

All modern languages prefer clean, unprefixed variant names.

## Go Community Preferences

Go proposal #19412 (Sum Types) shows community preference for simple variant naming:
- Examples use `Ok`, `Err` for Result[T,E]
- Examples use `Some`, `None` for Option[T]

Go standard library analysis shows predominance of unprefixed constants:
- Time: `Sunday`, `Monday`, `Asia`, `January`
- HTTP: `MethodGet`, `MethodPost` (some namespacing)
- Net: `FlagUp`, `FlagBroadcast`
- Syscall: `O_RDONLY`, `O_WRONLY`

Pattern: Exported constants without redundant type prefixes.

## Alternative Approaches

### Alternative 1: Go-Idiomatic Package Constants (RECOMMENDED)

**Pattern**: Plain PascalCase variant names
- Constants: `Circle`, `Square`, `Rectangle`
- Fields: `Radius`, `Side`, `Width`, `Height` (PascalCase for exported)

Generated code:
```go
type ShapeTag uint8

const (
    Circle ShapeTag = iota
    Square
    Rectangle
)

type Shape struct {
    tag ShapeTag
    Radius *float64
    Side *float64
    Width *float64
    Height *float64
}
```

**Advantages**:
- Perfect alignment with Go ecosystem
- Dramatically cleaner pattern matching
- Less typing, more natural
- Unicode-free naming style
- Follows Go's "quiet typing"

**Disadvantages**:
- Constants at package level (minor API difference)

### Alternative 2: Namespaced PascalCase

**Pattern**: `{EnumName}{VariantName}`
- Constants: `ShapeCircle`, `ShapeSquare`
- Similar to `http.MethodGet`, `reflect.SliceHeader`

**Advantages**:
- Clear namespacing
- Easy collision avoidance

**Disadvantages**:
- More verbose than necessary
- Still doesn't match standard library patterns

### Alternative 3: Screaming Caps

**Pattern**: `SHAPE_CIRCLE`
- Traditional C-style constants

**Advantages**:
- Familiar to systems developers

**Disadvantages**:
- Looks non-idiomatic in Go
- Violates "quiet typing" philosophy

## Recommendation

**Use Alternative 1: Go-Idiomatic Package Constants**

This makes enum usage feel natural:

**Current (Bad)**:
```go
match result {
    Err(e) => fmt.Println("error:", e)    // ShapeTag_Err feels wrong
}
if shape.tag == ShapeTag_Circle {
    radius := *shape.circle_radius
}
```

**Recommended (Good)**:
```go
match result {
    Err(e) => fmt.Println("error:", e)    // Clean, natural
}
if shape.tag == Circle {
    radius := *shape.Radius
}
```

**Pattern Matching Readability**: Much cleaner with short variant names
**Constructor Clarity**: `Shape.Circle(radius)` or `Circle()` constructor functions
**Type System Consistency**: Aligns with standard library Result/Option types

## Trade-offs

**Implementing Go-Idiomatic Constants**:
- ✅ **Pros**: Matches ecosystem, cleaner usage, better DX, follows conventions
- ⚠️ **Cons**: Package-level constants, potential name collisions (mitigated by packages)

The benefits far outweigh the minor trade-off of package-scoped constants.

## Migration Implementation

1. **Preprocessor Changes**:
   - Update enum generation to produce `Circle` instead of `ShapeTag_Circle`
   - Change field patterns to `Radius` instead of `circle_radius`

2. **Transpiler Impact**:
   - No AST changes needed
   - Constructor generation: Consider `Shape.Circle(radius)` syntax

3. **Test Updates**:
   - Regenerate all golden tests with new naming
   - Update pattern matching tests to use `Circle` vs `ShapeTag_Circle`

4. **Constructor Enhancement**:
   ```go
   // Current
   func Circle(radius float64) Shape {
       return Shape{tag: Circle, Radius: &radius}
   }

   // Possible enhancement
   func (Shape) Circle(radius float64) Shape {
       return Shape{tag: Circle, Radius: &radius}
   }
   ```

## Code Examples

**Good (Recommended)**:
```go
enum Color { Red(u8), Green(u8), Blue(u8) }

// Generates:
const (
    Red ColorTag = iota
    Green
    Blue
)

type Color struct {
    tag ColorTag
    Red *uint8
    Green *uint8
    Blue *uint8
}

// Usage:
if color.tag == Red {
    value := *color.Red
}
```

**Bad (Current)**:
```go
// Avoid ShapeTag_Red - violates Go naming
```

## Final Recommendation

The Go community and ecosystem consistently use unprefixed constant names for enum variants (time.Monday, http.MethodGet, net.FlagUp). Dingo should follow this pattern with:

- Variant names: `Ok`, `Err`, `Some`, `None`
- Package scoping for collision avoidance
- PascalCase fields matching Go conventions

This creates the most natural, idiomatic experience for Go developers using sum types.