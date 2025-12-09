# Default Parameters

**Priority:** P3 (Lower - API convenience feature)
**Status:** 🔴 Not Started
**Complexity:** 🟡 Medium (2 weeks implementation)
**Community Demand:** ⭐⭐ (Rejected by Go team, but common in other languages)
**Inspiration:** Swift, Kotlin, Python, C++, JavaScript

---

## Overview

Default parameter values allow functions to have optional arguments without overloading, reducing boilerplate for common parameter patterns.

## Motivation

### The Problem in Go

```go
// Must create multiple function variants manually
func Connect(host string) error {
    return ConnectWithPort(host, 8080)
}

func ConnectWithPort(host string, port int) error {
    return ConnectWithTimeout(host, port, 30*time.Second)
}

func ConnectWithTimeout(host string, port int, timeout time.Duration) error {
    // Actual implementation
}

// Or use options struct (verbose)
type ConnectOptions struct {
    Host    string
    Port    int
    Timeout time.Duration
}

func Connect(opts ConnectOptions) error {
    // Must handle zero values
    if opts.Port == 0 {
        opts.Port = 8080
    }
    if opts.Timeout == 0 {
        opts.Timeout = 30 * time.Second
    }
    // ...
}
```

**Go Team's Reasoning:**
- "Deliberate simplification"
- Concern about APIs growing unwieldy with many optional params
- Prefer explicit function names or options structs

---

## Why Dingo Should Implement It

### Counter-Arguments

**Go's Concern:** "Leads to API bloat"
**Dingo's Response:**
- Can set limits (e.g., max 3 defaulted params)
- Linter warnings for excessive defaults
- Still better than manual variants
- Transpiles cleanly to either strategy

**Go's Concern:** "Options struct is clearer"
**Dingo's Response:**
- For 1-2 optional params, defaults are simpler
- For many params, yes, use options struct
- Dingo can support BOTH patterns
- User chooses appropriate approach

---

## Proposed Syntax

### Basic Usage

```dingo
func connect(host: string, port: int = 8080, timeout: duration = 30s) -> Result[Connection, Error] {
    // Implementation
}

// Calls
connect("localhost")                    // Uses 8080, 30s
connect("localhost", 9000)              // Uses 30s
connect("localhost", 9000, 60s)         // All specified
connect("localhost", timeout: 5s)       // Named params: port=8080, timeout=5s
```

### Type Safety

```dingo
// ✅ Default must match parameter type
func greet(name: string = "World") { ... }  // OK

// ❌ Type mismatch
func greet(name: string = 42) { ... }  // ERROR: int != string

// ✅ Can use expressions
func connect(timeout: duration = 30 * time.Second) { ... }

// ✅ Can reference earlier params
func pad(s: string, width: int, char: string = " ") { ... }
```

### Named Arguments (Phase 2)

```dingo
// Call with named args to skip defaults
connect("localhost", timeout: 5s)       // port uses default
connect(host: "localhost", port: 9000)  // timeout uses default
```

---

## Transpilation Strategies

### Strategy 1: Generate Function Variants

```dingo
// Dingo source
func connect(host: string, port: int = 8080) -> Error {
    return dial(host, port)
}
```

```go
// Transpiled Go (generate all combinations)
func connect__0(host string) error {
    return connect__1(host, 8080)
}

func connect__1(host string, port int) error {
    return dial(host, port)
}

// User facing (via wrapper)
func connect(host string, args ...interface{}) error {
    switch len(args) {
    case 0:
        return connect__0(host)
    case 1:
        return connect__1(host, args[0].(int))
    default:
        panic("invalid arguments")
    }
}
```

### Strategy 2: Options Struct (Preferred)

```dingo
// Dingo source
func connect(host: string, port: int = 8080, timeout: duration = 30s) -> Error
```

```go
// Transpiled Go
type connect_Options struct {
    port    int
    timeout time.Duration
}

func connect(host string, opts ...connect_Options) error {
    var opt connect_Options
    if len(opts) > 0 {
        opt = opts[0]
    }

    // Apply defaults
    if opt.port == 0 {
        opt.port = 8080
    }
    if opt.timeout == 0 {
        opt.timeout = 30 * time.Second
    }

    return dial(host, opt.port, opt.timeout)
}

// Usage
connect("localhost")
connect("localhost", connect_Options{port: 9000})
```

---

## Complexity Analysis

**Implementation Complexity:** 🟡 Medium

### Parsing (2-3 days)
- Parse default value expressions
- Validate placement (defaults must be at end)
- Handle named arguments

### Type Checking (3-4 days)
- Verify default matches parameter type
- Evaluate default expressions
- Check for forward references
- Handle overload resolution with defaults

### Transpilation (5-6 days)
- Choose strategy (variants vs options struct)
- Generate helper types/functions
- Handle named argument reordering
- Optimize for common cases

**Total: ~2 weeks** for complete implementation

---

## Benefits

- ✅ Eliminates manual function variants
- ✅ Common in Swift, Kotlin, Python (developer expectation)
- ✅ Clean transpilation (either strategy works)
- ✅ Type-safe (defaults checked at compile time)

## Trade-offs

- ❓ Generated Go code is more complex
  - *Mitigation:* Options struct strategy is idiomatic Go
- ❓ Interaction with function overloading
  - *Mitigation:* Clear precedence rules

---

## Implementation Complexity

**Effort:** Medium
**Timeline:** 2 weeks

---

## References

- Go Proposal #24724: Default parameters (rejected)
- Swift Default Parameters: https://docs.swift.org/swift-book/documentation/the-swift-programming-language/functions/
- Kotlin Default Arguments: https://kotlinlang.org/docs/functions.html#default-arguments
