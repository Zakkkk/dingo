# Sum Types (Enums)

Sum types, also called tagged unions or algebraic data types, allow you to define a type that can be one of several variants. They're the foundation of type-safe state machines, error handling, and domain modeling in Dingo.

## Design Decision: Interface Pattern

Dingo enums compile to Go's **interface + struct pattern**:

```go
// Dingo
enum Status { Pending, Active, Done }

// Generated Go
type Status interface { isStatus() }
type StatusPending struct{}
func (StatusPending) isStatus() {}
type StatusActive struct{}
func (StatusActive) isStatus() {}
type StatusDone struct{}
func (StatusDone) isStatus() {}
```

**Why interfaces instead of tagged structs?**
1. **Type safety** - Go's type switch enforces exhaustiveness
2. **Zero overhead** - No tag field storage
3. **Idiomatic** - Go developers recognize the pattern
4. **gopls support** - Full IDE completion and type info

Variants with data become structs with fields:
```go
enum Shape { Circle { radius: float64 } }
→ type ShapeCircle struct { radius float64 }
```

## Why Sum Types?

Go's approach to representing "one of several types" typically uses interfaces or separate types:

```go
// Go approach: interface + type assertions
type Shape interface {
    Area() float64
}

type Circle struct { Radius float64 }
type Rectangle struct { Width, Height float64 }

func process(s Shape) {
    switch v := s.(type) {
    case Circle:
        // use v.Radius
    case Rectangle:
        // use v.Width, v.Height
    }
}
```

**Problems:**
- No exhaustiveness checking (easy to miss cases)
- Can add types later that break assumptions
- Verbose boilerplate

**Sum types solution:**

```go
// Dingo: explicit, closed set of variants
enum Shape {
    Circle { radius: float64 },
    Rectangle { width: float64, height: float64 },
    Point,
}

// Compiler ensures all cases handled!
match shape {
    Circle(r) => π * r * r,
    Rectangle(w, h) => w * h,
    Point => 0.0
}
```

## Basic Syntax

### Simple Enums (No Data)

```go
enum Status {
    Pending,
    Active,
    Complete,
}
```

### Enums with Associated Data

```go
enum Result {
    Ok(int),
    Err(string),
}

enum Option {
    Some(string),
    None,
}
```

### Named Fields

```go
enum Shape {
    Point,
    Circle { radius: float64 },
    Rectangle {
        width: float64,
        height: float64,
    },
}
```

## Real-World Examples

### HTTP Response Types

```go
package main

import "fmt"

enum ApiResponse {
    Success {
        status_code: int,
        body: []byte,
    },
    ClientError {
        status_code: int,
        message: string,
    },
    ServerError {
        status_code: int,
        error: string,
    },
    NetworkError(string),
}

func handleResponse(resp: ApiResponse) {
    match resp {
        Success(code, body) => {
            fmt.Printf("Success %d: %s\n", code, string(body))
        },
        ClientError(code, msg) => {
            fmt.Printf("Client error %d: %s\n", code, msg)
        },
        ServerError(code, err) => {
            fmt.Printf("Server error %d: %s\n", code, err)
        },
        NetworkError(err) => {
            fmt.Printf("Network error: %s\n", err)
        }
    }
}
```

### State Machine

```go
enum ConnectionState {
    Disconnected,
    Connecting { attempts: int },
    Connected {
        session_id: string,
        start_time: int64,
    },
    Error(string),
}

func transitionState(current: ConnectionState, event: Event) ConnectionState {
    match current {
        Disconnected => {
            return ConnectionState_Connecting(1)
        },
        Connecting(attempts) => {
            if attempts >= 3 {
                return ConnectionState_Error("Max retries exceeded")
            }
            return ConnectionState_Connecting(attempts + 1)
        },
        Connected(sessionID, startTime) => {
            return ConnectionState_Disconnected()
        },
        Error(msg) => {
            return ConnectionState_Disconnected()
        }
    }
}
```

### Domain Modeling

```go
enum Payment {
    Cash,
    Card {
        number: string,
        expiry: string,
    },
    Crypto {
        wallet: string,
        currency: string,
    },
}

enum Order {
    Draft {
        items: []Item,
        customer_id: string,
    },
    Pending {
        order_id: string,
        total: float64,
    },
    Paid {
        order_id: string,
        payment: Payment,
        receipt_url: string,
    },
    Cancelled(string),
}
```

## Constructor Functions

Each variant gets an auto-generated constructor:

```go
enum Status {
    Pending,
    Active,
    Complete,
}

// Auto-generated:
func StatusPending() Status
func StatusActive() Status
func StatusComplete() Status

// Usage:
let s = StatusPending()
```

### With Data

```go
enum Result {
    Ok(int),
    Err(string),
}

// Auto-generated:
func ResultOk(ok0 int) Result
func ResultErr(err0 string) Result

// Usage:
let success = ResultOk(42)
let failure = ResultErr("something went wrong")
```

### With Named Fields

```go
enum Shape {
    Circle { radius: float64 },
    Rectangle { width: float64, height: float64 },
}

// Auto-generated:
func ShapeCircle(radius float64) Shape
func ShapeRectangle(width, height float64) Shape

// Usage:
let circle = ShapeCircle(5.0)
let rect = ShapeRectangle(10.0, 20.0)
```

## Type Checking Methods

Each variant gets an `Is...()` method:

```go
enum Status {
    Pending,
    Active,
    Complete,
}

let status = StatusActive()

if status.IsActive() {
    println("Status is active!")
}

if status.IsPending() {
    println("This won't print")
}
```

## Accessing Data

Fields are accessible as pointers after type checking:

```go
enum Result {
    Ok(int),
    Err(string),
}

let result = ResultOk(42)

if result.IsOk() {
    let value = *result.ok0
    println("Value:", value)
}

if result.IsErr() {
    let err = *result.err0
    println("Error:", err)
}
```

### Named Fields

```go
enum Shape {
    Circle { radius: float64 },
}

let shape = ShapeCircle(5.0)

if shape.IsCircle() {
    let r = *shape.circleRadius
    println("Radius:", r)
}
```

## Generated Go Code

Dingo uses the **interface + struct pattern** for enums:

Input:
```go
enum Status {
    Pending,
    Active,
    Complete,
}
```

Generated:
```go
// Interface defines the sum type
type Status interface { isStatus() }

// Each variant is a struct implementing the interface
type StatusPending struct{}
func (StatusPending) isStatus() {}

type StatusActive struct{}
func (StatusActive) isStatus() {}

type StatusComplete struct{}
func (StatusComplete) isStatus() {}

// Constructor functions
func NewStatusPending() Status { return StatusPending{} }
func NewStatusActive() Status { return StatusActive{} }
func NewStatusComplete() Status { return StatusComplete{} }
```

**With data:**

```go
enum Shape {
    Point,
    Circle { radius: float64 },
    Rectangle { width: float64, height: float64 },
}
```

Becomes:

```go
// Interface
type Shape interface { isShape() }

// Variants with data are structs with fields
type ShapePoint struct{}
func (ShapePoint) isShape() {}

type ShapeCircle struct {
    radius float64
}
func (ShapeCircle) isShape() {}

type ShapeRectangle struct {
    width  float64
    height float64
}
func (ShapeRectangle) isShape() {}

// Constructors
func NewShapePoint() Shape { return ShapePoint{} }
func NewShapeCircle(radius float64) Shape { return ShapeCircle{radius: radius} }
func NewShapeRectangle(width, height float64) Shape { return ShapeRectangle{width: width, height: height} }
```

**Pattern matching** uses Go's type switch:
```go
switch v := shape.(type) {
case ShapePoint:
    fmt.Println("Point")
case ShapeCircle:
    fmt.Printf("Circle with radius %.2f\n", v.radius)
case ShapeRectangle:
    fmt.Printf("Rectangle %.2f x %.2f\n", v.width, v.height)
}
```

## Best Practices

### 1. Use Sum Types for Closed Sets

```go
// Good: Limited set of states
enum TrafficLight {
    Red,
    Yellow,
    Green,
}

// Bad: Open-ended set (use struct instead)
enum Person {
    Alice,
    Bob,
    Charlie,
    // ... millions more?
}
```

### 2. Add Data to Variants When Needed

```go
// Good: Different data for different states
enum Order {
    Created(string),              // order ID
    Paid { id: string, amount: float64 },
    Shipped { id: string, tracking: string },
    Delivered,
}

// Less useful: Same data everywhere
enum OrderWithSameData {
    Created { id: string },
    Paid { id: string },
    Shipped { id: string },
}
```

### 3. Use Descriptive Variant Names

```go
// Good
enum HttpStatus {
    Ok,
    NotFound,
    ServerError,
}

// Less clear
enum HttpStatus {
    S200,
    S404,
    S500,
}
```

### 4. Document Invariants

```go
// Status represents the lifecycle of a task.
// Transitions: Pending → Active → Complete
//           or: Pending → Cancelled
enum Status {
    Pending,
    Active,
    Complete,
    Cancelled(string),  // reason for cancellation
}
```

## Common Patterns

### Optional Values

```go
enum Option {
    Some(string),
    None,
}

func findUser(id: int) Option {
    if id > 0 {
        return OptionSome("User123")
    }
    return OptionNone()
}
```

### Error Handling

```go
enum Result {
    Ok(Data),
    Err(string),
}

func processData(input: string) Result {
    if input == "" {
        return ResultErr("empty input")
    }
    return ResultOk(Data{Value: input})
}
```

### State Machines

```go
enum State {
    Idle,
    Loading(string),
    Success(Data),
    Error(string),
}

func nextState(current: State, event: Event) State {
    match current {
        Idle => StateLoading("Fetching data"),
        Loading(msg) => StateSuccess(event.Data),
        Success(data) => StateIdle(),
        Error(err) => StateIdle()
    }
}
```

## Limitations

### Current Limitations

1. **No recursive types**: Can't define `enum Tree { Leaf, Node(Tree, Tree) }`
2. **No type parameters**: Can't define generic `enum Result[T, E]`
3. **Limited pattern matching**: Requires explicit match syntax

### Workarounds

**For generic-like behavior:**
```go
// Define specific types as needed
enum IntResult {
    Ok(int),
    Err(string),
}

enum StringResult {
    Ok(string),
    Err(string),
}
```

**For recursive types:**
```go
// Use pointers
enum Tree {
    Leaf(int),
    Node { left: *Tree, right: *Tree },
}
```

## Migration from Go

### Before (Go - Interface Pattern)

```go
type Status interface {
    isStatus()
}

type Pending struct{}
func (Pending) isStatus() {}

type Active struct{ TaskName string }
func (Active) isStatus() {}

type Complete struct{}
func (Complete) isStatus() {}

func handle(s Status) {
    switch v := s.(type) {
    case Pending:
        println("Pending")
    case Active:
        println("Active:", v.TaskName)
    case Complete:
        println("Complete")
    // Easy to forget default!
    }
}
```

**33 lines, no exhaustiveness checking**

### After (Dingo)

```go
enum Status {
    Pending,
    Active(string),
    Complete,
}

func handle(s: Status) {
    match s {
        Pending => println("Pending"),
        Active(task) => println("Active:", task),
        Complete => println("Complete")
    }
}
```

**15 lines, compile-time exhaustiveness**

**79% code reduction!**

## See Also

- [Pattern Matching](./pattern-matching.md) - Match on sum types
- [Result Type](./result-type.md) - Specific sum type for errors
- [Option Type](./option-type.md) - Specific sum type for nullability

## Resources

- [Go Proposal #19412](https://github.com/golang/go/issues/19412) - Sum types (996+ 👍, highest-voted proposal)
- [Rust enums](https://doc.rust-lang.org/book/ch06-00-enums.html) - Inspiration
- [Swift enums](https://docs.swift.org/swift-book/LanguageGuide/Enumerations.html) - Similar syntax
- [Examples](../../tests/golden/) - Working sum type examples in test suite
