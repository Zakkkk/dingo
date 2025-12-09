# Null Safety Operators

**Priority:** P1 (High - Prevents nil pointer bugs)
**Status:** ⚠️ Partial (Marker-based, needs full transformation)
**Implementation:** Token-level pass leaves `/*DINGO_SAFE_NAV*/` marker for `?.`
**Community Demand:** ⭐⭐⭐⭐
**Inspiration:** Swift, Kotlin, C#, TypeScript

---

## Overview

Safe navigation (`?.`) and null coalescing (`??`) operators eliminate nil pointer panics and reduce defensive nil-checking boilerplate.

## Motivation

### The Problem in Go

```go
// Nested nil checks are verbose
var city string
if user != nil && user.Address != nil && user.Address.City != nil {
    city = *user.Address.City
} else {
    city = "Unknown"
}

// Or panics if any is nil
city := user.Address.City.Name  // PANIC if any is nil
```

---

## Proposed Syntax

### Safe Navigation (`?.`)

```dingo
// Returns Option[T] - None if any is nil
let city = user?.address?.city?.name

// With unwrap
let city = user?.address?.city?.name ?? "Unknown"

// Method calls
let email = user?.getEmail()?.lowercase()
```

### Null Coalescing (`??`)

```dingo
// Provide default for None/nil
let name = user?.name ?? "Anonymous"

// Chain multiple fallbacks
let name = user?.name ?? settings.defaultName ?? "Guest"

// With expressions
let port = env.get("PORT")?.parseInt() ?? 8080
```

---

## Transpilation

```dingo
// Dingo
let city = user?.address?.city?.name ?? "Unknown"
```

```go
// Transpiled Go
var city string
if user != nil && user.Address != nil && user.Address.City != nil && user.Address.City.Name != nil {
    city = *user.Address.City.Name
} else {
    city = "Unknown"
}
```

---

## Inspiration

### Swift

```swift
let city = user?.address?.city?.name ?? "Unknown"
```

### Kotlin

```kotlin
val city = user?.address?.city?.name ?: "Unknown"
```

### TypeScript

```typescript
const city = user?.address?.city?.name ?? "Unknown";
```

---

## Implementation Complexity

**Effort:** Medium
**Timeline:** 1-2 weeks

---

## References

- Swift Optional Chaining: https://docs.swift.org/swift-book/documentation/the-swift-programming-language/optionalchaining/
- Kotlin Null Safety: https://kotlinlang.org/docs/null-safety.html
