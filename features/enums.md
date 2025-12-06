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

// Usage
let status = Status.Pending

match status {
    Pending => "waiting",
    Approved => "done",
    Rejected => "cancelled"
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

```dingo
// Dingo source
enum Status {
    Pending,
    Approved,
    Rejected
}
```

```go
// Transpiled Go (CamelCase naming - Go idiomatic)
type Status int

const (
    StatusPending Status = iota  // CamelCase: StatusPending (not Status_Pending)
    StatusApproved
    StatusRejected
)

var __StatusAll = []Status{
    StatusPending,
    StatusApproved,
    StatusRejected,
}

func (s Status) String() string {
    switch s {
    case StatusPending:
        return "Pending"
    case StatusApproved:
        return "Approved"
    case StatusRejected:
        return "Rejected"
    default:
        return fmt.Sprintf("Status(%d)", s)
    }
}

// Validation function (used in tests/asserts)
func (s Status) isValid() bool {
    return s >= StatusPending && s <= StatusRejected
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
let s = Status.Pending  // OK
```

### Exhaustiveness

```dingo
// ❌ Compile error - missing case
match status {
    Pending => "waiting",
    Approved => "done"
    // ERROR: Rejected not handled
}
```

### String Conversion

```dingo
let status = Status.Pending
println(status)  // Prints: "Pending"
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
