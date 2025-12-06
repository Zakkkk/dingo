# Sum Types (Discriminated Unions)

**Priority:** P0 (Critical - Foundation for type system)
**Status:** ✅ Implemented (Phase 10 - Token-Based Parser)
**Implementation:** `pkg/goparser/parser/parser.go` - `transformEnum()` (generates tagged union interface pattern)
**Community Demand:** ⭐⭐⭐⭐⭐ (996+ 👍 on Go proposal #19412 - HIGHEST)
**Inspiration:** Rust, Swift, TypeScript, Kotlin

---

## Overview

Sum types (also called discriminated unions, tagged unions, or algebraic data types) allow a value to be one of several fixed types, with compile-time enforcement and exhaustive checking. This is the most requested Go feature and foundational for Dingo's type system.

## Motivation

### The Problem in Go

```go
// Go uses empty interface{} for "one of many types"
type Response interface{}

func handleResponse(resp Response) {
    // Unsafe type assertions, no exhaustiveness
    switch v := resp.(type) {
    case SuccessResponse:
        fmt.Println("Success:", v.Data)
    case ErrorResponse:
        fmt.Println("Error:", v.Message)
    // Forgot TimeoutResponse - no compiler warning!
    default:
        fmt.Println("Unknown response")
    }
}

// Or manual tagged struct (verbose, error-prone)
type Response struct {
    Tag string // "success" | "error" | "timeout"
    SuccessData *SuccessResponse
    ErrorData *ErrorResponse
    TimeoutData *TimeoutResponse
}

// Caller must remember to check tag AND correct field
if resp.Tag == "success" && resp.SuccessData != nil {
    // handle success
}
```

**Problems:**
- No type safety (interface{} accepts anything)
- No exhaustiveness checking (easy to forget cases)
- Runtime type assertions can panic
- Verbose workarounds with manual tagging
- Nil pointer bugs when accessing wrong variant

### Research Data

- **Go Proposal #19412** - 996+ 👍 (HIGHEST engagement ever)
- **#54685** - Sigma types (dependent type approach)
- **#57644** - Ian Lance Taylor's proposal (extending generics)
- Considered the logical "next step" after generics
- Go team acknowledges "overlap with interfaces in confusing ways"

---

## Proposed Syntax

### Enum-Style Declaration

```dingo
// Sum type with named variants
enum HttpResponse {
    Ok(body: string),
    NotFound,
    ServerError{code: int, message: string},
    Redirect(url: string)
}

// Generic sum types
enum Result<T, E> {
    Ok(T),
    Err(E)
}

enum Option<T> {
    Some(T),
    None
}
```

### Usage with Pattern Matching

```dingo
func handleResponse(resp: HttpResponse) -> string {
    match resp {
        Ok(body) => "Success: ${body}",
        NotFound => "404 Not Found",
        ServerError{code, message} => "Error ${code}: ${message}",
        Redirect(url) => "Redirecting to ${url}"
    }
}

// Compiler enforces exhaustiveness
match resp {
    Ok(body) => ...,
    NotFound => ...
    // ERROR: Missing ServerError and Redirect cases
}
```

### Constructing Variants

```dingo
// Creating variants
let success = Ok("Hello, World!")
let notFound = NotFound
let error = ServerError{code: 500, message: "Internal error"}
let redirect = Redirect("https://example.com")

// Type inference
let response: HttpResponse = Ok("data")  // Infers Ok variant
```

---

## Transpilation Strategy

### Go Output (Tagged Union Pattern)

```go
// Transpiled sum type (CamelCase naming - Go idiomatic)
type HttpResponse struct {
    tag HttpResponseTag
    ok *string                   // CamelCase: ok (not ok_0)
    serverErrorCode *int         // CamelCase: serverErrorCode (not serverError_code)
    serverErrorMessage *string
    redirect *string
}

type HttpResponseTag int
const (
    HttpResponseTagOk HttpResponseTag = iota  // CamelCase: HttpResponseTagOk
    HttpResponseTagNotFound
    HttpResponseTagServerError
    HttpResponseTagRedirect
)

// Constructor functions
func HttpResponseOk(body string) HttpResponse {  // CamelCase: HttpResponseOk
    return HttpResponse{
        tag: HttpResponseTagOk,
        ok: &body,
    }
}

func HttpResponseNotFound() HttpResponse {
    return HttpResponse{
        tag: HttpResponseTagNotFound,
    }
}

// Pattern match transpiles to switch on tag
func handleResponse(resp HttpResponse) string {
    switch resp.tag {
    case HttpResponseTagOk:
        body := *resp.ok
        return fmt.Sprintf("Success: %s", body)
    case HttpResponseTagNotFound:
        return "404 Not Found"
    case HttpResponseTagServerError:
        code := *resp.serverErrorCode
        message := *resp.serverErrorMessage
        return fmt.Sprintf("Error %d: %s", code, message)
    case HttpResponseTagRedirect:
        url := *resp.redirect
        return fmt.Sprintf("Redirecting to %s", url)
    default:
        panic("unreachable: unhandled HttpResponse variant")
    }
}
```

### Optimization Strategies

```dingo
// Dingo can optimize based on variant sizes
enum SmallEnum {
    A(byte),
    B(int16),
    C(int32)
}
```

```go
// Optimized: use smallest container
type SmallEnum struct {
    tag uint8
    value int32  // Large enough for any variant
}
```

---

## Inspiration from Other Languages

### Rust's Enum Types

```rust
enum HttpResponse {
    Ok { body: String },
    NotFound,
    ServerError { code: i32, message: String },
    Redirect(String),
}

// Pattern matching
match response {
    HttpResponse::Ok { body } => println!("Success: {}", body),
    HttpResponse::NotFound => println!("404"),
    HttpResponse::ServerError { code, message } =>
        println!("Error {}: {}", code, message),
    HttpResponse::Redirect(url) => println!("-> {}", url),
}

// Option and Result are just enums
enum Option<T> {
    Some(T),
    None,
}

enum Result<T, E> {
    Ok(T),
    Err(E),
}
```

**Key Insights:**
- Enums are sum types with associated values
- Zero-cost abstractions (no runtime overhead)
- Exhaustiveness checked by compiler
- Can derive traits (Debug, Clone, PartialEq, etc.)
- Most important feature in Rust

### Swift's Enums with Associated Values

```swift
enum HttpResponse {
    case ok(body: String)
    case notFound
    case serverError(code: Int, message: String)
    case redirect(url: String)
}

// Switch requires exhaustiveness
switch response {
case .ok(let body):
    print("Success: \(body)")
case .notFound:
    print("404")
case .serverError(let code, let message):
    print("Error \(code): \(message)")
case .redirect(let url):
    print("Redirect: \(url)")
}

// Can add methods
extension HttpResponse {
    func isSuccess() -> Bool {
        if case .ok = self { return true }
        return false
    }
}
```

**Key Insights:**
- Associated values make enums powerful
- First-class pattern matching support
- Can conform to protocols
- Widely used in Swift standard library

### TypeScript's Discriminated Unions

```typescript
type HttpResponse =
    | { kind: 'ok'; body: string }
    | { kind: 'notFound' }
    | { kind: 'serverError'; code: number; message: string }
    | { kind: 'redirect'; url: string };

// Type narrowing in switch
function handleResponse(resp: HttpResponse): string {
    switch (resp.kind) {
        case 'ok':
            return `Success: ${resp.body}`;
        case 'notFound':
            return '404';
        case 'serverError':
            return `Error ${resp.code}: ${resp.message}`;
        case 'redirect':
            return `Redirect: ${resp.url}`;
    }
    // Compiler error if any case is missing
}
```

**Key Insights:**
- Manual tag field (`kind`) discriminates
- Compiler narrows types in each branch
- Structural typing (not nominal)
- Exhaustiveness via control flow analysis

### Kotlin's Sealed Classes

```kotlin
sealed class HttpResponse {
    data class Ok(val body: String) : HttpResponse()
    object NotFound : HttpResponse()
    data class ServerError(val code: Int, val message: String) : HttpResponse()
    data class Redirect(val url: String) : HttpResponse()
}

// When expression with exhaustiveness
when (response) {
    is HttpResponse.Ok -> "Success: ${response.body}"
    is HttpResponse.NotFound -> "404"
    is HttpResponse.ServerError -> "Error ${response.code}: ${response.message}"
    is HttpResponse.Redirect -> "Redirect: ${response.url}"
    // No else needed - compiler verifies all cases
}
```

**Key Insights:**
- Sealed classes restrict inheritance
- Data classes provide value semantics
- When expression enforces exhaustiveness
- Smart casts eliminate manual casting

---

## Implementation Details

### Type System Integration

```dingo
// Sum types are closed (fixed set of variants)
enum Shape {
    Circle(radius: float),
    Rectangle{width: float, height: float},
    Point
}

// Cannot extend externally (unlike interfaces)
// This enables exhaustiveness checking

// Methods on sum types
impl Shape {
    func area() -> float {
        match self {
            Circle(r) => 3.14 * r * r,
            Rectangle{width, height} => width * height,
            Point => 0.0
        }
    }
}
```

### Generic Sum Types

```dingo
// Result and Option are generic sum types
enum Result<T, E> {
    Ok(T),
    Err(E)
}

// Instantiation
let success: Result<User, DbError> = Ok(user)
let failure: Result<User, DbError> = Err(DbError.notFound())

// Nested generics
let nested: Option<Result<User, Error>> = Some(Ok(user))
```

### Interop with Go Interfaces

```dingo
// Sum types can implement interfaces
enum Animal {
    Dog{name: string},
    Cat{name: string}
}

impl Animal: Speaker {
    func speak() -> string {
        match self {
            Dog{name} => "${name} barks",
            Cat{name} => "${name} meows"
        }
    }
}

// Can be used where interface is expected
let animal: Animal = Dog{name: "Rex"}
let speaker: Speaker = animal  // Implicit conversion
```

---

## Benefits

### Type Safety

```dingo
// ❌ Cannot construct invalid variants
let response = Ok("data", 404)  // ERROR: Ok takes only 1 argument

// ❌ Cannot access wrong variant's data
match response {
    Ok(body) => {
        // ERROR: code not available in Ok variant
        println(body.code)
    }
}

// ✅ Type-safe access
match response {
    ServerError{code, message} => {
        println("Code: ${code}")  // code is int, type-safe
    }
}
```

### Exhaustiveness

```dingo
// Compiler tracks which cases are handled
enum Status { Pending, Approved, Rejected }

// ❌ Compile error
match status {
    Pending => "waiting",
    Approved => "done"
    // ERROR: Rejected not handled
}

// ✅ Compiles
match status {
    Pending => "waiting",
    Approved => "done",
    Rejected => "rejected"
}

// ✅ Or use wildcard
match status {
    Pending => "waiting",
    _ => "other"
}
```

### Self-Documenting

```dingo
// API contract is clear from type
func fetchUser(id: string) -> Result<User, FetchError>

// Compared to Go
func fetchUser(id string) (*User, error)  // What errors? Can user be nil?
```

---

## Tradeoffs

### Advantages
- ✅ **Eliminates entire classes of bugs** (exhaustiveness prevents forgetting cases)
- ✅ **Type-safe variant access** (cannot access wrong variant's data)
- ✅ **Self-documenting** (type shows all possible cases)
- ✅ **Enables powerful patterns** (Result, Option, state machines)

### Potential Concerns
- ❓ **Memory overhead** (tag + largest variant)
  - *Mitigation:* Compiler can optimize layouts, usually negligible
- ❓ **Learning curve** (new concept for Go developers)
  - *Mitigation:* Excellent documentation, familiar from TS/Rust/Swift
- ❓ **Larger binary size** (more generated code)
  - *Mitigation:* Generated code is simple, compresses well

---

## Implementation Complexity

**Effort:** High (Foundational feature)
**Timeline:** 3-4 weeks

### Phase 1: Core Type System (Week 1-2)
- [ ] Parse enum declarations
- [ ] Type check variant definitions
- [ ] Implement variant constructors
- [ ] Basic pattern matching integration
- [ ] Core tests

### Phase 2: Generics Integration (Week 2)
- [ ] Generic sum types (Result, Option)
- [ ] Type parameter constraints
- [ ] Monomorphization strategy
- [ ] Generics tests

### Phase 3: Transpilation (Week 3)
- [ ] Generate tagged union structs
- [ ] Generate constructor functions
- [ ] Optimize memory layouts
- [ ] Transpilation tests

### Phase 4: Advanced Features (Week 4)
- [ ] Methods on sum types (impl blocks)
- [ ] Interface implementation
- [ ] Derive common traits (Debug, Eq, etc.)
- [ ] Real-world integration tests

---

## Examples

### Example 1: JSON Value

```dingo
enum JsonValue {
    Null,
    Bool(bool),
    Number(float64),
    String(string),
    Array([]JsonValue),
    Object(map[string]JsonValue)
}

func stringify(value: JsonValue) -> string {
    match value {
        Null => "null",
        Bool(b) => if b { "true" } else { "false" },
        Number(n) => n.toString(),
        String(s) => "\"${s}\"",
        Array(items) => "[" + items.map(stringify).join(", ") + "]",
        Object(pairs) => "{" + pairs.map(|(k,v)| "\"${k}\": ${stringify(v)}").join(", ") + "}"
    }
}
```

### Example 2: State Machine

```dingo
enum ConnectionState {
    Disconnected,
    Connecting{attempt: int},
    Connected{session: Session},
    Error{error: string, retryAfter: time.Duration}
}

func handleState(state: ConnectionState) {
    match state {
        Disconnected => startConnection(),
        Connecting{attempt} => showProgress(attempt),
        Connected{session} => useSession(session),
        Error{error, retryAfter} => scheduleRetry(retryAfter)
    }
}
```

### Example 3: AST for Expression Evaluator

```dingo
enum Expr {
    Literal(value: int),
    Variable(name: string),
    BinaryOp{op: string, left: Expr, right: Expr},
    FunctionCall{name: string, args: []Expr}
}

func eval(expr: Expr, env: map[string]int) -> Result<int, EvalError> {
    match expr {
        Literal(n) => Ok(n),
        Variable(name) => env.get(name).okOr(EvalError.undefinedVar(name)),
        BinaryOp{op, left, right} => {
            let l = eval(left, env)?
            let r = eval(right, env)?
            return evalOp(op, l, r)
        },
        FunctionCall{name, args} => {
            let values = args.map(|a| eval(a, env)).collect()?
            return callFunction(name, values)
        }
    }
}
```

---

## Success Criteria

- [ ] Sum types work for all use cases (Result, Option, custom types)
- [ ] Exhaustiveness checking catches missing cases at compile-time
- [ ] Pattern matching provides ergonomic variant access
- [ ] Transpiled code has minimal memory overhead
- [ ] Generic sum types work correctly
- [ ] Interface implementation supported
- [ ] Positive feedback from Rust/Swift/TS developers

---

## References

- Go Proposal #19412: Sum types (996+ 👍)
- Go Proposal #54685: Sigma types
- Go Proposal #57644: Extending generics with unions (Ian Lance Taylor)
- Rust Enums: https://doc.rust-lang.org/book/ch06-00-enums.html
- Swift Enums: https://docs.swift.org/swift-book/documentation/the-swift-programming-language/enumerations/
- TypeScript Discriminated Unions: https://www.typescriptlang.org/docs/handbook/unions-and-intersections.html

---

## Next Steps

1. Finalize enum syntax and semantics
2. Implement parser for enum declarations
3. Design memory layout optimization
4. Prototype Result<T, E> and Option<T>
5. Test exhaustiveness checking algorithm
6. Benchmark memory overhead vs Go interfaces
