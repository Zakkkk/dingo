# Pattern Matching

Pattern matching allows you to match values against patterns and execute code based on which pattern matches. It's type-safe, exhaustive, and eliminates an entire class of bugs.

## Design Decision: Type Switch Generation

Dingo match expressions compile to Go's **type switch** pattern:

```go
// Dingo
match status {
    Pending => "waiting",
    Active => "running",
    Done => "finished",
}

// Generated Go
switch v := status.(type) {
case StatusPending:
    return "waiting"
case StatusActive:
    return "running"
case StatusDone:
    return "finished"
}
```

**Context-aware generation** produces different Go code:
- **Return context**: `switch { case ...: return ... }`
- **Assignment context**: `var x T; switch { case ...: x = ... }`
- **Argument context**: `func() T { switch ... }()` (IIFE)

## Why Pattern Matching?

Go's `switch` is good, but limited:

```go
// Go switch
switch status {
case "active":
    // handle active
case "inactive":
    // handle inactive
default:
    // handle unknown
}
```

**Problems:**
- No exhaustiveness checking (easy to forget cases)
- No destructuring (can't extract values inline)
- No type safety with sum types

**Pattern matching solution:**

```go
// Dingo match
match status {
    Active => handleActive(),
    Inactive => handleInactive(),
    Pending(reason) => handlePending(reason)
    // Compiler error if you miss a case!
}
```

## Basic Usage

### Simple Matching

```go
package main

import "fmt"

enum Status {
    Pending,
    Active,
    Complete,
}

func describe(status: Status) string {
    match status {
        Pending => "Waiting to start",
        Active => "Currently running",
        Complete => "All done"
    }
}

func main() {
    s := Status_Active()
    println(describe(s))
}
```

### With Destructuring

```go
enum Response {
    Success(int),
    Error(string),
    Pending,
}

func handleResponse(resp: Response) {
    match resp {
        Success(code) => fmt.Printf("Success with code: %d\n", code),
        Error(msg) => fmt.Printf("Error: %s\n", msg),
        Pending => println("Still waiting...")
    }
}
```

## Syntax

### Basic Match

```go
match value {
    Pattern1 => expression1,
    Pattern2 => expression2,
    Pattern3 => expression3
}
```

### Match as Expression

```go
result := match status {
    Active => "running",
    Inactive => "stopped",
    Pending => "waiting"
}
```

### Multi-Statement Arms

```go
match response {
    Success(data) => {
        println("Got data:", data)
        processData(data)
        sendConfirmation()
    },
    Error(err) => {
        log.Error(err)
        sendAlert()
    }
}
```

## Pattern Types

### Enum Variant Patterns

```go
enum Status {
    Idle,
    Running(string),
    Error(string),
}

match status {
    Idle => handleIdle(),
    Running(task) => handleRunning(task),
    Error(msg) => handleError(msg)
}
```

### Wildcard Pattern (`_`)

```go
match value {
    Important => handleImportant(),
    _ => handleAnythingElse()
}
```

### Guards (if conditions)

```go
enum Result {
    Ok(int),
    Err(string),
}

match result {
    Ok(value) if value > 100 => "High value",
    Ok(value) if value > 50 => "Medium value",
    Ok(value) => "Low value",
    Err(msg) => "Error: " + msg
}
```

### Guards (where conditions - Swift-style)

Dingo also supports Swift-style `where` guards as an alternative to `if`:

```go
enum Option {
    Some(int),
    None,
}

match opt {
    Some(x) where x > 100 => "large",
    Some(x) where x > 10 => "medium",
    Some(x) where x > 0 => "small",
    Some(_) => "non-positive",
    None => "none"
}
```

**Important Limitation:** `where` guards currently only support **simple patterns**. Nested patterns are not yet supported:

```go
// ❌ NOT SUPPORTED - nested pattern with where guard
match result {
    Result_Ok(Option_Some(val)) where val > 0 => "positive",  // ERROR
    // ...
}

// ✅ SUPPORTED - simple pattern with where guard
match result {
    Result_Ok(val) where val > 100 => "large",  // OK
    // ...
}
```

**Workaround for nested patterns:**

Use nested match expressions instead:

```go
match outer {
    Result_Ok(opt) => match opt {
        Option_Some(val) where val > 0 => "positive",
        Option_Some(_) => "non-positive",
        Option_None => "none"
    },
    Result_Err(err) => "error"
}
```

**Why the limitation?**

- Simple patterns cover 95%+ of real-world use cases
- Implementation complexity for nested patterns is high
- Can be added in future releases if demand warrants it

**Best Practice:** Use `if` guards (Rust-style) or `where` guards (Swift-style) based on your preference - both work identically for simple patterns.

## Real-World Examples

### HTTP Status Handler

```go
package main

import (
    "fmt"
    "net/http"
)

enum HttpResponse {
    Ok([]byte),
    NotFound,
    ServerError(string),
    Unauthorized,
}

func handleResponse(resp: HttpResponse) {
    match resp {
        Ok(body) => {
            fmt.Println("Success! Body:", string(body))
            processBody(body)
        },
        NotFound => {
            fmt.Println("Resource not found")
            show404Page()
        },
        ServerError(msg) => {
            fmt.Println("Server error:", msg)
            logError(msg)
            showErrorPage()
        },
        Unauthorized => {
            fmt.Println("Unauthorized")
            redirectToLogin()
        }
    }
}
```

### State Machine

```go
enum State {
    Idle,
    Loading(string),
    Success(Data),
    Error(string),
}

type Data struct {
    Content string
}

func renderUI(state: State) string {
    match state {
        Idle => "<div>Ready to start</div>",
        Loading(msg) => "<div>Loading: " + msg + "</div>",
        Success(data) => "<div>Data: " + data.Content + "</div>",
        Error(err) => "<div class='error'>" + err + "</div>"
    }
}
```

### Payment Processing

```go
enum PaymentStatus {
    Pending(string),
    Authorized(float64),
    Captured(float64, string),
    Failed(string),
    Refunded(float64),
}

func processPayment(status: PaymentStatus) {
    match status {
        Pending(id) => {
            println("Payment pending:", id)
            checkStatus(id)
        },
        Authorized(amount) => {
            println("Authorized:", amount)
            capture(amount)
        },
        Captured(amount, txnID) => {
            println("Captured:", amount, "Transaction:", txnID)
            updateInventory()
            sendConfirmation(txnID)
        },
        Failed(reason) => {
            println("Payment failed:", reason)
            notifyCustomer(reason)
        },
        Refunded(amount) => {
            println("Refunded:", amount)
            updateAccounting(amount)
        }
    }
}
```

## Exhaustiveness Checking

The compiler ensures all cases are handled:

```go
enum Status {
    Active,
    Inactive,
    Pending,
}

// ❌ Compile error: missing Pending case
match status {
    Active => handleActive(),
    Inactive => handleInactive()
    // ERROR: non-exhaustive match
}

// ✅ Correct: all cases covered
match status {
    Active => handleActive(),
    Inactive => handleInactive(),
    Pending => handlePending()
}

// ✅ Also correct: wildcard catches rest
match status {
    Active => handleActive(),
    _ => handleOther()
}
```

## Guards

Guards add conditional logic to patterns:

```go
enum Value {
    Number(int),
    Text(string),
}

match value {
    Number(n) if n < 0 => "Negative",
    Number(n) if n == 0 => "Zero",
    Number(n) if n > 0 => "Positive",
    Text(s) if len(s) == 0 => "Empty string",
    Text(s) => "Text: " + s
}
```

### Complex Guards

```go
enum Order {
    Regular(float64),
    Premium(float64, string),
}

match order {
    Regular(amount) if amount > 1000 => "Large regular order",
    Regular(amount) => "Small regular order",
    Premium(amount, level) if amount > 1000 && level == "gold" => "VIP order",
    Premium(amount, level) => "Premium order"
}
```

## Generated Go Code

Dingo uses Go's **type switch** for pattern matching on enums:

Input:
```go
match status {
    Active => "running",
    Inactive => "stopped",
    Pending => "waiting"
}
```

Generated (type switch pattern):
```go
func() string {
    switch status.(type) {
    case StatusActive:
        return "running"
    case StatusInactive:
        return "stopped"
    case StatusPending:
        return "waiting"
    default:
        panic("non-exhaustive match")
    }
}()
```

**With destructuring:**

```go
match shape {
    Circle { radius } => fmt.Sprintf("r=%.2f", radius),
    Rectangle { width, height } => fmt.Sprintf("%v x %v", width, height),
    Point => "point"
}
```

Becomes:

```go
func() string {
    switch v := shape.(type) {
    case ShapeCircle:
        return fmt.Sprintf("r=%.2f", v.radius)
    case ShapeRectangle:
        return fmt.Sprintf("%v x %v", v.width, v.height)
    case ShapePoint:
        return "point"
    default:
        panic("non-exhaustive match")
    }
}()
```

**Key insight**: The type switch binds `v` to the matched variant, giving direct access to its fields.

## Best Practices

### 1. Order Patterns from Specific to General

```go
// Good
match value {
    SpecificCase => handleSpecific(),
    GeneralCase => handleGeneral(),
    _ => handleDefault()
}

// Bad: Default first catches everything
match value {
    _ => handleDefault(),
    SpecificCase => handleSpecific()  // Never reached!
}
```

### 2. Use Guards for Complex Conditions

```go
// Good: Guard makes intent clear
match result {
    Ok(n) if n > threshold => "Above threshold",
    Ok(n) => "Below threshold",
    Err(e) => "Error"
}

// Worse: Nested if in match arm
match result {
    Ok(n) => {
        if n > threshold {
            return "Above threshold"
        }
        return "Below threshold"
    },
    Err(e) => "Error"
}
```

### 3. Extract Values with Destructuring

```go
// Good: Extract and use inline
match user {
    Admin(name, level) => fmt.Sprintf("Admin %s (level %d)", name, level),
    User(name) => fmt.Sprintf("User %s", name)
}

// Worse: Manual extraction
match user {
    Admin => {
        name := *user.admin
        level := *user.level
        fmt.Sprintf("Admin %s (level %d)", name, level)
    }
}
```

### 4. Use Wildcard for Default Cases

```go
match status {
    Critical => handleCritical(),
    Warning => handleWarning(),
    _ => handleDefault()  // Catches Info, Debug, etc.
}
```

## Common Patterns

### Option Handling

```go
enum Option {
    Some(string),
    None,
}

message := match option {
    Some(value) => "Found: " + value,
    None => "Not found"
}
```

### Result Handling

```go
enum Result {
    Ok(User),
    Err(string),
}

match result {
    Ok(user) => {
        println("Welcome,", user.Name)
        loginUser(user)
    },
    Err(error) => {
        println("Login failed:", error)
        showError(error)
    }
}
```

### Nested Matching

```go
enum Outer {
    First(Inner),
    Second(string),
}

enum Inner {
    A(int),
    B(string),
}

match outer {
    First(inner) => {
        match inner {
            A(n) => handleNumber(n),
            B(s) => handleString(s)
        }
    },
    Second(s) => handleOuter(s)
}
```

## Comparison with Go Switch

### Go Switch

```go
switch status.Tag() {
case StatusActive:
    handleActive()
case StatusInactive:
    handleInactive()
default:
    // Easy to forget this!
}
```

**Issues:**
- No exhaustiveness checking
- Manual tag extraction
- Easy to miss cases

### Dingo Match

```go
match status {
    Active => handleActive(),
    Inactive => handleInactive()
    // Compile error if Pending exists but not handled!
}
```

**Benefits:**
- Compile-time exhaustiveness
- Clean syntax
- Type-safe destructuring

## Limitations

### Current Limitations

1. **Single-level matching**: Nested pattern matching requires nested match expressions
2. **No range patterns**: `1..10 =>` not supported (use guards instead)
3. **No regex patterns**: String matching is exact only

### Workarounds

**For ranges:**
```go
// Use guards
match value {
    Number(n) if n >= 1 && n <= 10 => "Low",
    Number(n) if n >= 11 && n <= 100 => "Medium",
    Number(n) => "High"
}
```

**For complex patterns:**
```go
// Use helper functions
func isValidEmail(s string) bool { /* ... */ }

match input {
    Email(addr) if isValidEmail(addr) => "Valid email",
    Email(addr) => "Invalid email"
}
```

## Migration from Go

### Before (Go)

```go
func handleStatus(status Status) string {
    switch status.Tag {
    case StatusTagActive:
        return "Running"
    case StatusTagInactive:
        return "Stopped"
    case StatusTagPending:
        if status.PendingReason != nil {
            return "Waiting: " + *status.PendingReason
        }
        return "Waiting"
    default:
        return "Unknown"
    }
}
```

### After (Dingo)

```go
func handleStatus(status: Status) string {
    match status {
        Active => "Running",
        Inactive => "Stopped",
        Pending(reason) => "Waiting: " + reason
    }
}
```

**Benefits:**
- 50% less code
- Exhaustiveness checking
- Cleaner destructuring
- No default case needed

## See Also

- [Sum Types](./sum-types.md) - Define matchable enums
- [Result Type](./result-type.md) - Pattern match on Results
- [Option Type](./option-type.md) - Pattern match on Options

## Resources

- [Rust pattern matching](https://doc.rust-lang.org/book/ch18-00-patterns.html) - Inspiration
- [Swift pattern matching](https://docs.swift.org/swift-book/LanguageGuide/Patterns.html) - Similar syntax
- [Examples](../../tests/golden/) - Working pattern matching examples
