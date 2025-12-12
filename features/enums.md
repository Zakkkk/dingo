# Type-Safe Enums

**Priority:** P1 (High - Essential for production use)
**Status:** ✅ Implemented (Phase 10 - Token-Based Parser)
**Implementation:** `pkg/goparser/parser/parser.go` - `transformEnum()`
**Community Demand:** ⭐⭐⭐⭐⭐ (900+ combined 👍 across proposals)
**Inspiration:** Rust, Swift, Java, Kotlin

---

## Overview

Type-safe enums provide closed sets of named constants with compile-time validation and exhaustiveness checking. Unlike Go's iota-based approach, true enums prevent invalid values and guarantee all cases are handled.

## Motivation

### The Problem in Go

```go
// Go's current approach (iota constants)
type Status int
const (
    Pending Status = iota
    Approved
    Rejected
)

// Problems:
// 1. No type safety - any int can be cast
var s Status = Status(999)  // Valid but nonsensical

// 2. No exhaustiveness checking
switch s {
case Pending:
    fmt.Println("pending")
case Approved:
    fmt.Println("approved")
// Forgot Rejected - NO WARNING
}

// 3. No string conversion
fmt.Println(s)  // Prints: 2 (not "Rejected")

// 4. No iteration over values
// Must maintain separate slice manually
```

**Research Data:**
- Multiple proposals: #28438, #28987, #36387
- 900+ combined upvotes
- Active in LanguageChangeReview

---

## Proposed Syntax

### Basic Enum

```dingo
enum Status {
    Pending,
    Approved,
    Rejected
}

// Usage - constructor functions (Go-idiomatic)
status := NewStatusPending()

// Or direct struct literal
status := StatusPending{}

match status {
    StatusPending => "waiting",
    StatusApproved => "done",
    StatusRejected => "cancelled"
    // Compiler enforces all cases
}
```

### Enums with Values

```dingo
// Explicit values (like iota)
enum Priority {
    Low = 1,
    Medium = 5,
    High = 10
}

// String-based enums
enum Color {
    Red = "red",
    Green = "green",
    Blue = "blue"
}
```

### Methods on Enums

```dingo
enum Status {
    Pending,
    Approved,
    Rejected
}

impl Status {
    func isComplete() -> bool {
        match self {
            Pending => false,
            Approved | Rejected => true
        }
    }

    func message() -> string {
        match self {
            Pending => "Waiting for approval",
            Approved => "Request approved",
            Rejected => "Request rejected"
        }
    }
}
```

---

## Transpilation Strategy

### Design Decision: Interface-Based Sum Types

Dingo uses **interface-based sum types** instead of tagged structs because:

1. **True sum types** - Only one variant's data exists at a time
2. **Type-safe** - Must type-assert to access fields (compiler enforces)
3. **Memory efficient** - Only active variant allocated
4. **Go-idiomatic** - How go/ast, protobuf, and stdlib implement unions

**Why NOT tagged struct?**
```go
// ❌ Tagged struct (product type) - NOT USED
type Status struct {
    tag         StatusTag
    pendingData *PendingData  // nil when not Pending - but still accessible!
    activeData  *ActiveData   // nil when not Active
}
// Problem: Nothing stops Go code from accessing wrong variant's fields
```

### Actual Transpilation

```dingo
// Dingo source
enum Status {
    Pending,
    Approved,
    Rejected
}
```

```go
// Transpiled Go - Interface-based pattern (true sum type)
type Status interface {
    isStatus() // unexported marker = sealed interface
}

type StatusPending struct{}
func (StatusPending) isStatus() {}
func NewStatusPending() Status { return StatusPending{} }

type StatusApproved struct{}
func (StatusApproved) isStatus() {}
func NewStatusApproved() Status { return StatusApproved{} }

type StatusRejected struct{}
func (StatusRejected) isStatus() {}
func NewStatusRejected() Status { return StatusRejected{} }
```

### Pattern Matching → Type Switch

```go
// Dingo match compiles to Go's idiomatic type switch
switch v := status.(type) {
case StatusPending:
    return "waiting"
case StatusApproved:
    return "done"
case StatusRejected:
    return "cancelled"
}
```

---

## Inspiration from Other Languages

### Rust's Enums (Simple Variants)

```rust
enum Status {
    Pending,
    Approved,
    Rejected,
}

// Pattern matching requires exhaustiveness
match status {
    Status::Pending => "waiting",
    Status::Approved => "done",
    Status::Rejected => "cancelled",
}
```

### Swift's Enums

```swift
enum Status {
    case pending
    case approved
    case rejected
}

// CaseIterable for iteration
enum Status: CaseIterable {
    case pending, approved, rejected
}

Status.allCases.forEach { print($0) }
```

### Java's Enums

```java
enum Status {
    PENDING,
    APPROVED,
    REJECTED;

    public String message() {
        return switch(this) {
            case PENDING -> "Waiting";
            case APPROVED -> "Done";
            case REJECTED -> "Cancelled";
        };
    }
}
```

---

## Benefits

### Type Safety

```dingo
// ❌ Cannot create invalid values
let s: Status = 999  // Compile error

// ✅ Only valid constructors
s := NewStatusPending()  // OK
s := StatusPending{}     // Also OK
```

### Exhaustiveness

```dingo
// ❌ Compile error - missing case
match status {
    StatusPending => "waiting",
    StatusApproved => "done"
    // ERROR: StatusRejected not handled
}
```

### String Conversion

```dingo
status := NewStatusPending()
println(status)  // Prints: "StatusPending" (or implement String() method)
```

### Iteration

```dingo
// Iterate over all values
for status in Status.values() {
    println("${status}: ${status.message()}")
}
```

---

## Implementation Complexity

**Effort:** Low-Medium
**Timeline:** 1 week

---

## References

- Go Proposals: #28438, #28987, #36387
- Rust Enums: https://doc.rust-lang.org/book/ch06-01-defining-an-enum.html
- Swift Enums: https://docs.swift.org/swift-book/documentation/the-swift-programming-language/enumerations/
