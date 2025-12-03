# Tuples

## Scenario
Geometry calculations where points and coordinates are naturally pairs of values. Tuples let you group related values without defining a full struct.

## The Problem
Go lacks tuples, forcing you to either:
1. Define a struct for every small grouping
2. Return multiple values (can't store in variables easily)
3. Use arrays/slices (lose type safety on elements)

```go
// Overkill for simple coordinate pair
type Point struct {
    X float64
    Y float64
}

// Or awkward multiple returns
func GetCoordinates() (float64, float64) { ... }
x, y := GetCoordinates()
// Can't easily pass (x, y) to another function
```

## Dingo Solution
First-class tuple support:

```dingo
type Point2D = (float64, float64)

// Tuple literals
origin := (0.0, 0.0)

// Destructuring
let (x, y) = point

// Nested tuples
type BoundingBox = (Point2D, Point2D)
```

## Comparison

| Need | Go | Dingo |
|------|-----|-------|
| Pair of values | Define struct | `(T1, T2)` |
| Return pair | Multiple returns | Return tuple |
| Store pair | Struct variable | Tuple variable |
| Unpack pair | Manual field access | Destructuring |

## Key Points

### Tuple Literals
```dingo
point := (3.0, 4.0)
triple := (1, "hello", true)
nested := ((0, 0), (10, 10))
```

### Type Aliases
```dingo
type Point2D = (float64, float64)
type Pair[T] = (T, T)
```

### Destructuring
```dingo
let (x, y) = point              // Both values
let (first, _) = pair           // Ignore second
let ((minX, minY), (maxX, maxY)) = bbox  // Nested
```

### Returning Tuples
```dingo
func Divide(a: int, b: int) (int, int) {
    return (a / b, a % b)  // Quotient and remainder
}
```

### When to Use
- Coordinate pairs
- Key-value pairs
- Multiple return values that belong together
- Temporary groupings not worth a struct

### When to Use Structs
- Named fields improve readability
- Data has methods
- Used across many functions
- Part of public API

## Generated Code
The transpiler generates:
- Anonymous structs with `_0`, `_1`, etc. fields
- Type aliases for named tuple types
- Destructuring as field access
- Zero overhead (same as struct)
