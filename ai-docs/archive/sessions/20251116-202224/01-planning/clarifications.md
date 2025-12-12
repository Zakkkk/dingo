# User Clarifications

**Session:** 20251116-202224
**Date:** 2025-11-16

## Design Decisions Made

### 1. Match Arm Syntax
**Decision:** `=>` (Rust-style)
**Rationale:** Avoids conflict with potential future arrow function syntax `() -> T`

**Example:**
```dingo
match shape {
    Circle { radius } => println("Circle with radius: {}", radius)
    Rectangle { width, height } => println("Rectangle: {}x{}", width, height)
}
```

### 2. Match Expression Type
**Decision:** Expression (can be used in assignments and return values)
**Rationale:** More powerful and functional, consistent with Rust/Swift

**Example:**
```dingo
area := match shape {
    Circle { radius } => 3.14 * radius * radius
    Rectangle { width, height } => width * height
}
```

### 3. Exhaustiveness Checking
**Decision:** Error on missing cases + allow `_` wildcard
**Rationale:** Strictness with escape hatch (Rust-style)

**Impact:**
- Compile error if not all variants are covered
- Wildcard `_` can be used as catch-all
- Ensures type safety while providing flexibility

### 4. Helper Method Generation
**Decision:** Auto-generate Is* methods for all variants
**Rationale:** Better Go interop and ergonomics

**Example Generated Methods:**
```go
func (s Shape) IsCircle() bool { return s.tag == ShapeTag_Circle }
func (s Shape) IsRectangle() bool { return s.tag == ShapeTag_Rectangle }
func (s Shape) IsTriangle() bool { return s.tag == ShapeTag_Triangle }
```

### 5. Standard Types Location
**Decision:** Standard prelude (auto-imported)
**Rationale:** Best user experience - defined as normal enums in `dingo/std` but automatically available

**Impact:**
- `Result[T, E]` and `Option[T]` work out of the box
- Defined in standard library, not compiler magic
- Can be inspected like any other enum

### 6. Trailing Commas
**Decision:** Allow trailing commas
**Rationale:** Better diffs and prevents errors when adding variants - consistent with Go

**Example:**
```dingo
enum Status {
    Pending,
    Active,
    Complete,  // trailing comma allowed
}
```

## Summary of Choices

**Philosophy:** Rust-inspired with Go pragmatism
- **Syntax:** Rust-style `=>` for match arms
- **Semantics:** Expression-based matching for functional style
- **Safety:** Strict exhaustiveness with escape hatch
- **Ergonomics:** Auto-generated helpers for better Go interop
- **Consistency:** Trailing commas like Go, prelude like Rust

## Next Steps

With these decisions, the golang-architect will finalize the implementation plan incorporating:
1. Match expression parsing with `=>` separator
2. Expression context handling for match results
3. Exhaustiveness checker with wildcard support
4. Helper method code generation
5. Standard prelude structure
6. Trailing comma support in parser
