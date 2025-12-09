# Pattern Matching Syntax Options - Detailed Examples

## Question 1: Syntax Style

### Option A: Rust-like Syntax (RECOMMENDED)

**Dingo code:**
```go
fn processResult(result: Result[int, string]) -> int {
    match result {
        Ok(value) => {
            println("Success:", value)
            return value * 2
        }
        Err(error) => {
            println("Error:", error)
            return 0
        }
    }
}

// Enum matching
enum Status {
    Pending
    Active(userId: int)
    Completed(userId: int, timestamp: int64)
}

fn handleStatus(status: Status) -> string {
    match status {
        Pending => "Waiting..."
        Active(id) => fmt.Sprintf("User %d is active", id)
        Completed(id, time) => fmt.Sprintf("User %d completed at %d", id, time)
    }
}
```

**Why Rust-like?**
- Consistent with `Result[T,E]` and `Option[T]` (already Rust-inspired)
- Pattern syntax directly mirrors variant construction: `Ok(x)` construct → `Ok(x) =>` pattern
- Very readable: `match value { Pattern => expression }`
- Users coming from TypeScript/Rust find it familiar

---

### Option B: Kotlin-like Syntax

**Dingo code:**
```go
fn processResult(result: Result[int, string]) -> int {
    when (result) {
        is Ok -> {
            value := result.unwrap()
            println("Success:", value)
            return value * 2
        }
        is Err -> {
            error := result.unwrapErr()
            println("Error:", error)
            return 0
        }
    }
}

// Enum matching
when (status) {
    is Pending -> "Waiting..."
    is Active -> {
        id := status.userId  // Smart cast to Active variant
        return fmt.Sprintf("User %d is active", id)
    }
    is Completed -> {
        id := status.userId
        time := status.timestamp
        return fmt.Sprintf("User %d completed at %d", id, time)
    }
}
```

**Why Kotlin-like?**
- Uses smart casts (type inference after `is` check)
- No destructuring syntax to learn (access fields via `.field`)
- Familiar to Java/Kotlin developers

**Drawbacks:**
- Requires separate unwrap calls to get values
- Less concise than Rust syntax
- Smart casts add complexity to type inference

---

### Option C: Swift-like Syntax

**Dingo code:**
```go
fn processResult(result: Result[int, string]) -> int {
    switch result {
        case .ok(let value):
            println("Success:", value)
            return value * 2
        case .err(let error):
            println("Error:", error)
            return 0
    }
}

// Enum matching
switch status {
    case .pending:
        return "Waiting..."
    case .active(let id):
        return fmt.Sprintf("User %d is active", id)
    case .completed(let id, let time):
        return fmt.Sprintf("User %d completed at %d", id, time)
}
```

**Why Swift-like?**
- Extends Go's existing `switch` statement (familiar)
- `.variant` syntax clearly shows enum membership
- `let` keyword explicit about binding

**Drawbacks:**
- `.lowercase` variant names conflict with Go's exported name conventions
- More verbose than Rust syntax
- Less natural for Dingo's existing Result/Option types

---

## Question 2: Exhaustiveness Checking

### Option A: Compile Error (Strict - RECOMMENDED)

**Dingo code:**
```go
fn handleResult(result: Result[int, string]) -> int {
    match result {
        Ok(x) => x * 2
        // COMPILER ERROR: non-exhaustive match, missing Err case
    }
}

// Fix with explicit wildcard:
fn handleResult(result: Result[int, string]) -> int {
    match result {
        Ok(x) => x * 2
        _ => 0  // Catch-all required
    }
}
```

**Transpiles to:**
```go
func handleResult(result Result[int, string]) int {
    switch result.tag {
    case ResultTagOk:
        x := result.okValue
        return x * 2
    default:
        return 0
    }
}
```

**Why strict?**
- Prevents runtime panics from unhandled cases
- Forces you to think about all possibilities
- Rust/TypeScript do this (proven safe)

---

### Option B: Compile Warning (Lenient)

**Dingo code:**
```go
fn handleResult(result: Result[int, string]) -> int {
    match result {
        Ok(x) => x * 2
        // COMPILER WARNING: non-exhaustive match, missing Err case
        // (but code still compiles)
    }
}
```

**Transpiles to:**
```go
func handleResult(result Result[int, string]) int {
    switch result.tag {
    case ResultTagOk:
        x := result.okValue
        return x * 2
    }
    // Missing default case - runtime panic if result is Err!
    panic("non-exhaustive match")
}
```

**Why lenient?**
- Easier for beginners
- Gradual migration from Go code
- Can ignore warnings if you "know better"

**Drawback:** Runtime panics instead of compile-time safety

---

### Option C: Configurable

Compiler flag: `dingo build --strict-match` (error) vs `dingo build --lenient-match` (warning)

**Complexity:** Need to maintain two behaviors, document both, confusing for newcomers

---

## Question 3: Expression vs Statement

### Option A: Expression-Based (RECOMMENDED)

**Match always returns a value:**

```go
// Example 1: Assign match result
fn getValue(opt: Option[int]) -> int {
    let default = match opt {
        Some(x) => x          // Returns int
        None => 0             // Returns int (same type)
    }
    return default
}

// Example 2: Return match directly
fn processStatus(status: Status) -> string {
    return match status {
        Pending => "waiting"
        Active(id) => fmt.Sprintf("user_%d", id)
        Completed(_, _) => "done"
    }
}

// Example 3: Nested expression
fn compute(x: Option[int], y: Option[int]) -> int {
    return (match x { Some(v) => v, None => 0 }) +
           (match y { Some(v) => v, None => 0 })
}
```

**Type checking enforced:**
```go
// COMPILER ERROR: type mismatch in match arms
let x = match result {
    Ok(v) => v          // Returns int
    Err(e) => e         // Returns string - ERROR!
}

// Fix: make types compatible
let x = match result {
    Ok(v) => fmt.Sprintf("%d", v)   // Returns string
    Err(e) => e                     // Returns string - OK
}
```

**Why expression-based?**
- More functional, composable
- Type-safe: all arms must return same type
- Concise: no need for temporary variables
- Rust/Scala/Kotlin all do this

---

### Option B: Statement-Based

**Match doesn't return a value:**

```go
fn handleResult(result: Result[int, string]) {
    match result {
        Ok(x) => println("Success:", x)
        Err(e) => println("Error:", e)
    }
    // No return value from match
}

// Can't assign match result:
let x = match result { ... }  // ERROR: match is a statement, not expression

// Need temporary variables:
let x: int
match result {
    Ok(v) => x = v * 2
    Err(_) => x = 0
}
```

**Why statement-based?**
- More like Go's switch (familiar)
- No type checking complexity
- Simpler implementation

**Drawback:** Less functional, more verbose, can't compose

---

### Option C: Infer from Usage

**Smart detection:**

```go
// Used in assignment → expression mode (type-checked)
let x = match result {
    Ok(v) => v
    Err(_) => 0
}

// Used standalone → statement mode (no type checking)
match result {
    Ok(v) => println(v)
    Err(e) => println(e)
}
```

**Why infer?**
- Best of both worlds?
- Flexible for different use cases

**Drawback:**
- More complex to implement
- Confusing: same syntax, different semantics
- Harder to teach

---

## Comparison Table

| Feature | Rust-like | Kotlin-like | Swift-like |
|---------|-----------|-------------|------------|
| **Syntax** | `Ok(x) => expr` | `is Ok -> expr` | `case .ok(let x):` |
| **Destructuring** | Built-in | Manual unwrap | Built-in |
| **Readability** | ⭐⭐⭐⭐⭐ | ⭐⭐⭐ | ⭐⭐⭐⭐ |
| **Conciseness** | ⭐⭐⭐⭐⭐ | ⭐⭐⭐ | ⭐⭐⭐ |
| **Go familiarity** | ⭐⭐ | ⭐⭐⭐ | ⭐⭐⭐⭐⭐ |
| **Dingo consistency** | ⭐⭐⭐⭐⭐ | ⭐⭐⭐ | ⭐⭐ |

---

## Recommendations

Based on Dingo's existing design (Rust-inspired types, functional style):

1. **Syntax: Rust-like** - Most consistent with Result[T,E]/Option[T]
2. **Exhaustiveness: Compile error (strict)** - Prevents bugs, proven safe
3. **Match type: Expression-based** - More functional, composable, type-safe

**Rationale:**
- Dingo already committed to Rust-like syntax (Result, Option, enums)
- Users choosing Dingo want modern features (not just Go++)
- Safety and expressiveness over familiarity

**Migration path:**
- Rust/TypeScript users: feels natural immediately
- Go users: learning curve, but safety benefits are clear
- Documentation and examples ease transition
