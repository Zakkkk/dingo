# Dingo vs Borgo: Critical Architectural Difference

## Why This Matters

Borgo and Dingo are both Go transpilers, but they have **fundamentally different goals** that require **different architectures**.

## The Core Difference

| Aspect | Borgo | Dingo |
|--------|-------|-------|
| **Goal** | New language that compiles to Go | Syntax sugar for Go |
| **Type System** | Rust-like (traits, Hindley-Milner) | Go's type system unchanged |
| **Interop** | Limited - Borgo types ≠ Go types | 100% - Dingo IS Go |
| **Output** | Go code (Borgo-flavored) | Idiomatic Go code |

## Why Borgo Built Its Own Type Checker

Borgo adds **fundamentally new type concepts** that don't exist in Go:

```borgo
// Borgo: Traits (don't exist in Go)
impl Display for User {
    fn display(self) -> string { ... }
}

// Borgo: Hindley-Milner type inference
let x = Some(1)  // Borgo infers Option<int> differently than Go would

// Borgo: Algebraic data types as first-class
enum Result<T, E> { Ok(T), Err(E) }
// This is a REAL sum type in Borgo, not an interface pattern
```

gopls **cannot** type-check Borgo code because Go doesn't have these concepts.

## Why Dingo Does NOT Need Its Own Type Checker

Dingo doesn't add new type concepts - it adds **syntax** for existing Go patterns:

```dingo
// Dingo: Just syntax sugar for Go generics
func fetch() -> Result<User, error> { ... }

// Transforms to REAL Go:
func fetch() Result[User, error] { ... }

// Dingo's Result IS a Go generic type:
type Result[T, E any] struct { ... }  // Standard Go!
```

```dingo
// Dingo enum:
enum Status { Pending, Active, Done }

// Transforms to Go interface pattern:
type Status interface { isStatus() }
type StatusPending struct{}
func (StatusPending) isStatus() {}
// gopls can type-check this perfectly!
```

## The Critical Insight

| | Borgo | Dingo |
|-|-------|-------|
| After transformation | Still needs Borgo semantics | Pure Go - gopls works |
| `Result<T,E>` | Borgo's own type | Go's `Result[T,E]` generic |
| Pattern matching | Borgo's exhaustiveness rules | Transforms to Go switch |
| Type inference | Borgo's rules (different from Go) | Go's rules (unchanged) |

## Architecture Comparison

```
BORGO (must build own type checker):
  .borgo → Borgo Parser → Borgo AST → Borgo Type Checker → Go Code
                                          ↑
                               REQUIRED (Go can't understand Borgo types)

DINGO (use gopls):
  .dingo → Dingo Parser → Transform → .go file → gopls
                                          ↑
                          Just valid Go! gopls works fine
```

## Cost/Benefit Analysis

| Factor | Build Own Type Checker | Use gopls Proxy |
|--------|------------------------|-----------------|
| Engineering effort | 50,000+ LOC, 18-24 months | 5,000-10,000 LOC |
| Maintenance | 1-2 FTE ongoing | Minimal |
| Go compatibility | Risk of drift | Automatic |
| IDE features | Must build everything | Full gopls parity |

## What Dingo Builds (Minimal Semantic Analysis)

Only things gopls **cannot** do:

| Check | Why Dingo Must Do It |
|-------|---------------------|
| Pattern exhaustiveness | Go switch doesn't have this |
| Enum variant validation | Dingo-specific construct |
| `?` in non-Result function | Syntax-level check |
| Error message translation | Make gopls errors Dingo-native |

## What Dingo Delegates to gopls

Everything Go-related:
- Type checking
- Symbol resolution
- Import resolution
- Interface satisfaction
- Generic inference
- Autocomplete, go-to-def, hover, rename, etc.

## Summary

**Borgo** = "I want Rust's type system but compile to Go" → Must build type checker

**Dingo** = "I want nicer syntax for writing Go" → Use gopls, focus on syntax/DX

Dingo's value proposition is **syntax and ergonomics**, not a new type system. Building a Go type checker would be reimplementing `go/types` (30K+ LOC) plus gopls (100K+ LOC). That's not where Dingo's value is.
