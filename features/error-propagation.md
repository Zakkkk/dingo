# Error Propagation Operator (`?`)

**Priority:** P0 (Critical - Core MVP Feature)
**Status:** ✅ Implemented (Phase 10 - Token-Based Parser)
**Implementation:** `pkg/goparser/parser/parser.go` - `transformErrorProp()`
**Community Demand:** ⭐⭐⭐⭐⭐ (Go proposal #71203 active, Rust's most loved feature)
**Inspiration:** Rust, Swift

---

## Overview

The `?` operator provides concise error propagation by automatically returning early if a `Result` contains an error. This eliminates the repetitive `if err != nil { return err }` pattern while maintaining explicit control flow.

## Motivation

### The Problem in Go

```go
func processOrder(orderID string) (*Order, error) {
    order, err := fetchOrder(orderID)
    if err != nil {
        return nil, fmt.Errorf("fetch failed: %w", err)
    }

    validated, err := validateOrder(order)
    if err != nil {
        return nil, fmt.Errorf("validation failed: %w", err)
    }

    payment, err := processPayment(validated)
    if err != nil {
        return nil, fmt.Errorf("payment failed: %w", err)
    }

    saved, err := saveOrder(payment)
    if err != nil {
        return nil, fmt.Errorf("save failed: %w", err)
    }

    return saved, nil
}
```

**Problem:** 75% of this function is error handling boilerplate. The actual business logic (fetch → validate → process → save) is obscured.

### Research Data

- **880+ comments** on Go's `try()` proposal (rejected)
- **#71203** - Active `?` operator proposal (Jan 2025)
- **Rust developers** cite `?` as a top feature (95% satisfaction)
- Go team moratorium = opportunity for meta-language solution

---

## Proposed Syntax

### Basic Usage

```dingo
func processOrder(orderID: string) -> Result[Order, Error] {
    order := fetchOrder(orderID)?      // Return Err if failed
    validated := validateOrder(order)? // Continue if Ok
    payment := processPayment(validated)?
    saved := saveOrder(payment)?
    return Ok(saved)
}
```

### How It Works

```dingo
// This Dingo code...
user := fetchUser(id)?

// ...is syntactic sugar for:
user := match fetchUser(id) {
    Ok(value) => value,
    Err(error) => return Err(error)
}
```

### Error Context (Advanced)

Dingo provides three ways to add context to propagated errors:

#### 1. String Context (`? "message"`)

The simplest form - wraps the error with `fmt.Errorf`:

```dingo
func processOrder(orderID: string) -> Result[Order, Error] {
    order := fetchOrder(orderID) ? "fetch failed"
    validated := validateOrder(order) ? "validation failed"
    return Ok(validated)
}
```

Transpiles to:
```go
func processOrder(orderID string) ResultOrderError {
    tmp, err := fetchOrder(orderID)
    if err != nil {
        return ResultOrderError{err: fmt.Errorf("fetch failed: %w", err)}
    }
    order := tmp
    // ...
}
```

#### 2. Lambda Transform - Rust Style (`? |err| expr`)

For custom error transformation using Rust-style lambda syntax:

```dingo
func loadUserData(userID: int) -> Result[UserData, AppError] {
    user := fetchUser(userID) ? |err| AppError.wrap("user fetch", err)
    posts := fetchPosts(user.ID) ? |e| AppError.wrap("posts fetch", e)
    return Ok(UserData{user, posts})
}
```

Transpiles to:
```go
func loadUserData(userID int) ResultUserDataAppError {
    tmp, err := fetchUser(userID)
    if err != nil {
        return ResultUserDataAppError{err: func(err error) error { return AppError.wrap("user fetch", err) }(err)}
    }
    user := tmp
    // ...
}
```

#### 3. Lambda Transform - TypeScript Style (`? (err) => expr` or `? err => expr`)

Same functionality with TypeScript/JavaScript arrow syntax:

```dingo
func loadConfig(path: string) -> Result[Config, error] {
    content := readFile(path) ? (e) => fmt.Errorf("read failed: %w", e)
    config := parseJSON(content) ? err => fmt.Errorf("parse failed: %w", err)
    return Ok(config)
}
```

#### Lambda Parameter Naming

Lambda parameters can use any valid identifier:
- `|err|` - full name
- `|e|` - short form
- `|error|` - verbose
- All are equivalent

#### Choosing Between Forms

| Form | Use Case | Example |
|------|----------|---------|
| `? "message"` | Simple context, standard wrapping | `fetchUser(id) ? "user not found"` |
| `? \|err\| expr` | Custom error types, complex transforms | `fetchUser(id) ? \|e\| AppError.fromDB(e)` |
| `? (err) => expr` | JS/TS developers, same as above | `fetchUser(id) ? e => wrap(e)` |

**Note:** Nested error transforms are NOT supported. Use separate statements:
```dingo
// ❌ Invalid - nested transforms
x := foo()? |e1| bar()? |e2| combine(e1, e2)

// ✅ Valid - use separate statements
a := foo() ? |e| wrap("foo", e)
b := bar() ? |e| wrap("bar", e)
```

---

## Transpilation Strategy

### Simple Case

```dingo
// Dingo source
user := fetchUser(id)?
processUser(user)
```

```go
// Transpiled Go
tmp, err := fetchUser(id)
if err != nil {
    return ResultUserError{err: err}
}
user := tmp
processUser(user)
```

### With Error Wrapping

```dingo
// Dingo source
user := fetchUser(id) ? "failed to fetch user"
```

```go
// Transpiled Go
tmp, err := fetchUser(id)
if err != nil {
    return ResultUserError{
        err: fmt.Errorf("failed to fetch user: %w", err),
    }
}
user := tmp
```

### Chained Operations

```dingo
// Dingo source
func processOrder(id: string) -> Result[Order, Error] {
    order := fetchOrder(id)?
    validated := validateOrder(order)?
    paid := processPayment(validated)?
    return Ok(paid)
}
```

```go
// Transpiled Go (readable, idiomatic)
func processOrder(id string) ResultOrderError {
    tmp, err := fetchOrder(id)
    if err != nil {
        return ResultOrderError{err: err}
    }
    order := tmp

    tmp1, err1 := validateOrder(order)
    if err1 != nil {
        return ResultOrderError{err: err1}
    }
    validated := tmp1

    tmp2, err2 := processPayment(validated)
    if err2 != nil {
        return ResultOrderError{err: err2}
    }
    paid := tmp2

    return ResultOrderError{value: &paid}
}
```

---

## Inspiration from Other Languages

### Rust's `?` Operator

```rust
fn process_order(id: &str) -> Result[Order, Error] {
    let order = fetch_order(id)?;      // Propagate error
    let validated = validate(order)?;   // Early return if Err
    let paid = process_payment(validated)?;
    Ok(paid)
}

// Equivalent verbose version
fn process_order_verbose(id: &str) -> Result[Order, Error] {
    let order = match fetch_order(id) {
        Ok(o) => o,
        Err(e) => return Err(e),
    };
    // ... same for other steps
}
```

**Key Insights:**
- Most loved Rust feature (developer surveys)
- Zero runtime cost (compile-time transformation)
- Maintains explicit control flow (visible where errors can occur)
- Works with `Option[T]` too (returns `None` instead of error)

**Rust's Evolution:**
- Originally `try!()` macro (2014)
- Changed to `?` operator (2017, RFC 243)
- Community unanimously preferred `?` over `try!()`

### Swift's `try` Keyword

```swift
func processOrder(id: String) throws -> Order {
    let order = try fetchOrder(id)      // Propagate error
    let validated = try validate(order)
    let paid = try processPayment(validated)
    return paid
}

// With error handling
func main() {
    do {
        let order = try processOrder("123")
        print("Success: \(order)")
    } catch {
        print("Error: \(error)")
    }
}
```

**Key Insights:**
- `try` keyword makes error points explicit
- `throws` in signature makes error handling visible
- Exception-based (different from Dingo's Result approach)
- Still cleaner than Go's `if err != nil` pattern

**Why Dingo Prefers `?` over `try`:**
- `?` is more concise (1 char vs 4 chars + space)
- `try` in Go proposals was rejected (confused with try/catch)
- Rust's `?` has proven track record
- Visual consistency with `?` for nullable types (Option)

---

## Design Decisions

### Why `?` and not other operators?

| Operator | Pros | Cons | Decision |
|----------|------|------|----------|
| `?` | Concise, proven (Rust), visual | Could confuse with ternary | ✅ **Chosen** |
| `!` | Even shorter | Conflicts with null assertions, "unwrap" meaning | ❌ Rejected |
| `try()` | Explicit function | Rejected by Go community, verbose | ❌ Rejected |
| `!!` | Clear propagation | Too similar to `!`, non-standard | ❌ Rejected |
| postfix `?:` | Unique to Dingo | Unfamiliar, harder to type | ❌ Rejected |

**Rationale:** `?` is proven by Rust, concise, and doesn't conflict with Go's lack of ternary operator.

### Where can `?` be used?

```dingo
// ✅ Valid: After function call returning Result
user := fetchUser(id)?

// ✅ Valid: After method call
data := file.read()?

// ✅ Valid: In expression
return processUser(fetchUser(id)?)

// ✅ Valid: Multiple in one line (discouraged for readability)
result := fetch(id)?.validate()?.save()?

// ❌ Invalid: On non-Result types
x := 42?  // Compile error

// ❌ Invalid: In function not returning Result
func main() {
    user := fetchUser(id)?  // Error: main doesn't return Result
}
```

### Error Type Compatibility

```dingo
// ✅ Same error type
func process() -> Result[User, HttpError] {
    data := fetchData()?  // Returns Result[Data, HttpError]
    return Ok(transformData(data))
}

// ✅ Error type conversion (automatic if conversion exists)
func process() -> Result[User, AppError] {
    data := fetchData()?  // Returns Result[Data, HttpError]
    // HttpError auto-converts to AppError if impl exists
    return Ok(transformData(data))
}

// ❌ Incompatible error types (compile error)
func process() -> Result[User, AppError] {
    data := fetchData()?  // Returns Result[Data, DatabaseError]
    // Error: Cannot convert DatabaseError to AppError
}
```

---

## Implementation Details

### Parsing

```ebnf
PrimaryExpr = Operand
            | PrimaryExpr "?"           // Error propagation
            | PrimaryExpr "[" Expr "]"
            | PrimaryExpr "." identifier
            | ...
```

### Type Checking

```
1. Check that `?` is applied to Result[T, E]
2. Check that enclosing function returns Result[_, E'] where E converts to E'
3. Unwrap inner type: Result[T, E]? → T
4. Generate early return code if Result is Err
```

### AST Representation

```go
type ErrorPropagationExpr struct {
    Expr Expr              // The expression returning Result
    ErrorContext string    // Optional error wrapping message
    Pos token.Pos
}
```

### Transpilation Algorithm

```
For each `expr?` in source:
  1. Generate unique temp variable names (camelCase, 1-based):
     - First: tmp, err
     - Second: tmp1, err1
     - Third: tmp2, err2
  2. Assign expression to temp: tmp, err := expr
  3. Check for error: if err != nil
  4. Early return with error: return Result{err: err}
  5. Unwrap value: value := tmp
  6. Continue with unwrapped value
```

---

## Benefits

### Code Reduction

```dingo
// Dingo: 5 lines
func process(id: string) -> Result[Order, Error] {
    order := fetchOrder(id)?
    validated := validateOrder(order)?
    return Ok(validated)
}
```

```go
// Go: 11 lines (120% more code)
func process(id string) (*Order, error) {
    order, err := fetchOrder(id)
    if err != nil {
        return nil, err
    }

    validated, err := validateOrder(order)
    if err != nil {
        return nil, err
    }

    return validated, nil
}
```

**Metrics:**
- **60-70% reduction** in error handling code
- **90% reduction** in visual noise
- **Same number** of error handling points (explicit)

### Improved Readability

```dingo
// Happy path is clear
func processOrder(id: string) -> Result[Order, Error] {
    order := fetchOrder(id)?
    validated := validateOrder(order)?
    paid := processPayment(validated)?
    shipped := shipOrder(paid)?
    return Ok(shipped)
}

// Business logic is immediately obvious:
// fetch → validate → pay → ship
```

### Type Safety

```dingo
// Compiler tracks error types
func fetch() -> Result[User, DbError] { ... }
func validate(u: User) -> Result[User, ValidationError] { ... }

// ❌ This won't compile (error type mismatch)
func process() -> Result[User, DbError] {
    user := fetch()?
    validated := validate(user)?  // ERROR: ValidationError != DbError
    return Ok(validated)
}

// ✅ Must handle conversion explicitly
func process() -> Result[User, AppError] {
    user := fetch().mapErr(AppError.from)?
    validated := validate(user).mapErr(AppError.from)?
    return Ok(validated)
}
```

---

## Tradeoffs

### Advantages
- ✅ **Dramatic code reduction** (60-70% less error handling code)
- ✅ **Explicit error points** (? is visible, shows where errors can occur)
- ✅ **Type-safe** (compiler tracks error types)
- ✅ **Zero runtime cost** (pure compile-time transformation)
- ✅ **Proven design** (Rust has used this for 8+ years)

### Potential Concerns
- ❓ **New syntax** (developers must learn `?`)
  - *Mitigation:* Familiar from Rust, simple mental model
- ❓ **Hidden control flow** (early return not immediately obvious)
  - *Mitigation:* `?` is visual indicator, better than Go's if/return
- ❓ **Requires Result type** (can't use with raw errors)
  - *Mitigation:* Interop with Go via automatic wrapping

---

## Implementation Complexity

**Effort:** Medium-Low
**Timeline:** 1-2 weeks

### Phase 1: Parser (3 days)
- [ ] Add `?` to grammar
- [ ] Parse postfix `?` operator
- [ ] Handle precedence and associativity
- [ ] Parser tests

### Phase 2: Type Checker (4 days)
- [ ] Validate `?` applied to Result types
- [ ] Check enclosing function returns Result
- [ ] Verify error type compatibility
- [ ] Type checker tests

### Phase 3: Transpiler (3 days)
- [ ] Generate temp variable for Result
- [ ] Generate error check and early return
- [ ] Unwrap and assign value
- [ ] Integration tests

### Phase 4: Error Context (2 days)
- [ ] Support `expr ? "message"` syntax
- [ ] Generate fmt.Errorf wrapping
- [ ] Tests with error context

---

## Examples

### Example 1: File Processing

```dingo
func loadConfig(path: string) -> Result[Config, IOError] {
    data := os.ReadFile(path)?
    config := json.Unmarshal(data)?
    return Ok(config)
}
```

Transpiles to:

```go
func loadConfig(path string) ResultConfigIOError {
    tmp, err := osReadFile(path)
    if err != nil {
        return ResultConfigIOError{err: err}
    }
    data := tmp

    tmp1, err1 := jsonUnmarshal(data)
    if err1 != nil {
        return ResultConfigIOError{err: err1}
    }
    config := tmp1

    return ResultConfigIOError{value: &config}
}
```

### Example 2: HTTP API

```dingo
func fetchUserData(userID: string) -> Result[UserData, ApiError] {
    resp := http.Get("/api/users/" + userID)?
    user := parseUser(resp.Body)?
    posts := fetchPosts(user.ID)?
    comments := fetchComments(user.ID)?

    return Ok(UserData{user, posts, comments})
}
```

### Example 3: Database Transaction

```dingo
func transferMoney(from: int, to: int, amount: decimal) -> Result[Transaction, DbError] {
    tx := db.Begin()?
    defer tx.Rollback()

    fromAccount := tx.GetAccount(from)?
    toAccount := tx.GetAccount(to)?

    fromAccount.Balance -= amount
    toAccount.Balance += amount

    tx.Update(fromAccount)?
    tx.Update(toAccount)?
    tx.Commit()?

    return Ok(Transaction{from, to, amount})
}
```

---

## Success Criteria

- [ ] `?` operator reduces error handling code by 60%+
- [ ] Works with all Result[T, E] types
- [ ] Type checker catches incompatible error types
- [ ] Transpiled code is readable and idiomatic Go
- [ ] Zero performance overhead vs manual error handling
- [ ] Comprehensive test coverage (edge cases, error paths)
- [ ] Positive feedback from Rust developers testing Dingo

---

## References

- Go Proposal #71203: `?` operator (Jan 2025)
- Go Proposal #32437: `try()` builtin (rejected)
- Rust RFC 243: `?` operator
- Rust Survey 2024: 95% love the `?` operator
- Swift Error Handling: https://docs.swift.org/swift-book/documentation/the-swift-programming-language/errorhandling/

---

## Next Steps

1. Prototype parser support for `?` operator
2. Implement type checking rules
3. Generate transpiled Go code for test cases
4. Compare output quality with hand-written Go
5. Measure code reduction metrics on real projects
6. Gather community feedback on syntax
