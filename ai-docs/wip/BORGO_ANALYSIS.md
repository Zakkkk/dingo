# Borgo Language: Deep Dive Analysis for Dingo Implementation

**Date:** 2025-11-16
**Purpose:** Understand Borgo's architecture to inform Dingo's implementation
**Status:** Research Complete

---

## Executive Summary

Borgo is a **statically-typed language that transpiles to Go**, written in Rust, with Rust-like syntax. It successfully demonstrates that adding type-safe features (Result, Option, pattern matching, enums) to Go is **feasible and valuable**.

**Key Takeaways for Dingo:**
1. ✅ **Proven Architecture:** Parser → Type Inference → Code Generation works
2. ✅ **Go Interop:** Automatic wrapping of Go's `(T, error)` → `Result[T, E]` is critical
3. ✅ **Type Inference:** Makes the language ergonomic (less type annotations)
4. ✅ **Built-in Types:** Result/Option as language primitives (not libraries)
5. ⚠️ **Licensing Issue:** Borgo has no license (project may be dead/archived)
6. 🔄 **Different Approach:** Borgo uses Rust parser (we'll use Go-based tooling)

---

## Borgo Overview

### Project Statistics

| Metric | Value |
|--------|-------|
| **GitHub Stars** | 4.5k+ |
| **Implementation** | Rust (89.6%), Go runtime (4.5%) |
| **Status** | ⚠️ No activity in 5+ months, no license |
| **Syntax** | Rust-like (reuses Rust parser) |
| **Target** | Go (generates `.go` files) |
| **Use Case** | "More expressive than Go, less complex than Rust" |

### Design Philosophy

**Goals:**
- Add type safety features Go lacks (Result, Option, enums)
- Maintain 100% Go ecosystem compatibility
- No runtime overhead (transpiles to pure Go)
- Type inference to reduce verbosity

**Non-Goals:**
- Replace Go entirely
- Add runtime library
- Complex type system (keep it simple)

---

## Architecture Deep Dive

### Compiler Structure (Rust Modules)

```
borgo/compiler/src/
├── lib.rs              # Module organization
├── ast.rs              # Abstract Syntax Tree definitions
├── lexer.rs            # Tokenization
├── parser.rs           # Syntax parsing (Rust parser wrapper)
├── type_.rs            # Type system definitions
├── infer.rs            # Type inference engine
├── substitute.rs       # Type substitution
├── exhaustive.rs       # Exhaustiveness checking (pattern matching)
├── codegen.rs          # Go code generation
├── error.rs            # Error handling/reporting
├── fs.rs               # File system operations
├── global_state.rs     # Global compiler state
└── prelude.rs          # Standard library/built-ins
```

### Compilation Pipeline

```
Input: program.brg
    ↓
┌─────────────────┐
│  1. LEXER       │ → Tokens
└─────────────────┘
    ↓
┌─────────────────┐
│  2. PARSER      │ → AST (reuses Rust's parser)
└─────────────────┘
    ↓
┌─────────────────┐
│  3. TYPE INFER  │ → Typed AST
│  - infer.rs     │   • Hindley-Milner type inference
│  - type_.rs     │   • Type unification
│  - substitute.rs│   • Generic instantiation
└─────────────────┘
    ↓
┌─────────────────┐
│  4. VALIDATE    │ → Checked AST
│  - exhaustive.rs│   • Pattern match exhaustiveness
│  - error.rs     │   • Semantic validation
└─────────────────┘
    ↓
┌─────────────────┐
│  5. CODEGEN     │ → program.go
│  - codegen.rs   │   • Emit idiomatic Go
└─────────────────┘
```

---

## Feature Implementations

### 1. Result[T, E] Type

**Borgo Definition:**
```borgo
enum Result[T, E] {
    Ok(T),
    Err(E)
}
```

**How It Transpiles:**

```rust
// Borgo transpiles to Go struct with tag
type ResultTag int
const (
    Result_Ok ResultTag = iota
    Result_Err
)

type Result[T, E] struct {
    tag ResultTag
    Ok0 T      // Value if Ok
    Err0 E     // Error if Err
}

// Constructor functions
func make_Result_Ok[T, E](value T) Result[T, E] {
    return Result[T, E]{tag: Result_Ok, Ok0: value}
}

func make_Result_Err[T, E](err E) Result[T, E] {
    return Result[T, E]{tag: Result_Err, Err0: err}
}
```

**Key Insights:**
- ✅ Uses Go generics (requires Go 1.18+)
- ✅ Tagged union with `tag` discriminant
- ✅ Separate fields for each variant (`Ok0`, `Err0`)
- ✅ Constructor functions enforce correct construction

### 2. Option[T] Type

**Borgo Definition:**
```borgo
enum Option[T] {
    Some(T),
    None
}
```

**Transpilation Pattern:** Same as Result (tagged struct)

### 3. Error Propagation (`?` Operator)

**Borgo Code:**
```borgo
fn fetchUser(id: string) -> Result[User, error] {
    let resp = http.Get(url)?
    let user = parseUser(resp)?
    Ok(user)
}
```

**Transpiled Go:**
```go
func fetchUser(id string) (User, error) {
    // http.Get returns (Response, error) - auto-wrapped
    __result0 := http.Get(url)
    if __result0.Err != nil {
        return User{}, __result0.Err
    }
    resp := __result0.Ok

    __result1 := parseUser(resp)
    if __result1.Err != nil {
        return User{}, __result1.Err
    }
    user := __result1.Ok

    return user, nil
}
```

**Code Generation Strategy (from codegen.rs):**

```rust
// Context-dependent behavior
match wrap_mode {
    CallWrapMode::Wrapped => {
        // Functions returning (ok, err) or (ok, bool)
        emit!("check, err := function()")
        emit!("if err != nil { return nil, err }")
    }
    CallWrapMode::Unwrapped => {
        // Result[T, E] constructors
        // Automatically unwrap without intermediate wrapping
    }
}
```

**Key Insights:**
- ✅ `?` generates early return on error
- ✅ Temporary variables (`__result0`, `__result1`)
- ✅ Context-aware: knows when to wrap/unwrap
- ✅ Works with both Result and Go tuples

### 4. Pattern Matching

**Borgo Code:**
```borgo
match result {
    Ok(user) => println("Found: {user.name}"),
    Err(err) => println("Error: {err}")
}
```

**Transpiled Go:**
```go
// Sentinel-based matching system
is_matching := 0  // 0=start, 1=fail, 2=success

// Try Ok pattern
if result.tag == Result_Ok {
    user := result.Ok0
    fmt.Println("Found: " + user.name)
    is_matching = 2
}

// Try Err pattern (if Ok failed)
if is_matching == 0 && result.tag == Result_Err {
    err := result.Err0
    fmt.Println("Error: " + err.Error())
    is_matching = 2
}

// Exhaustiveness check (compile-time verified)
if is_matching == 0 {
    panic("non-exhaustive match")
}
```

**Exhaustiveness Checking (exhaustive.rs):**

```rust
// Compiler tracks:
1. All possible enum variants
2. Which variants are covered by patterns
3. If wildcard (_) is present

// Error if:
- Any variant is uncovered AND no wildcard
- Pattern after wildcard (unreachable code)
```

**Key Insights:**
- ✅ Sentinel variable tracks match state
- ✅ Sequential pattern testing
- ✅ Compile-time exhaustiveness checking
- ✅ Panic on unreachable (proves exhaustiveness)

### 5. Go Interoperability

**Automatic Type Conversion:**

```borgo
// Go function signature:
func LookupEnv(key string) (string, bool)

// Borgo sees it as:
fn LookupEnv(key: string) -> Option[string]

// Go function signature:
func Stat(name string) (FileInfo, error)

// Borgo sees it as:
fn Stat(name: string) -> Result[FileInfo, error]
```

**Implementation (infer.rs):**

```rust
// Special handling during type inference
fn add_optional_error_to_result(&mut self, ty: &Type, args: &[TypeAst])
  -> Vec<TypeAst>
{
    // If Result[T] (missing error type), add 'error'
    if args.len() == 1 {
        args.push(Type::Error)
    }
}

// Automatic wrapping at call sites
fn infer_call() {
    if returns_tuple_with_error(callee) {
        wrap_in_result(callee)
    }
    if returns_tuple_with_bool(callee) {
        wrap_in_option(callee)
    }
}
```

**Key Insights:**
- ✅ **Critical for ecosystem compatibility**
- ✅ User doesn't write wrappers manually
- ✅ Works at type inference time
- ✅ Seamless Go package usage

### 6. Type Inference

**Borgo Example:**
```borgo
// No type annotations needed
let numbers = [1, 2, 3, 4, 5]              // Inferred: []int
let doubled = numbers.map(|x| x * 2)       // Inferred: []int
let greeting = "Hello"                     // Inferred: string
```

**Implementation (infer.rs):**

```rust
// Hindley-Milner type inference
1. Assign type variables to unknowns
2. Collect constraints from expressions
3. Unify constraints (find substitutions)
4. Apply substitutions to get concrete types

// Special handling for built-in types
fn builtin_type(&self, name: &str) -> Type {
    self.cache.get(name)  // Result, Option, Slice, Map, etc.
}

fn type_result(&self, x: Type, y: Type) -> Type {
    self.builtin_type("Result").swap_arg(0, x).swap_arg(1, y)
}
```

**Key Insights:**
- ✅ Makes language ergonomic (less boilerplate)
- ✅ Standard Hindley-Milner algorithm
- ✅ Cached built-in types for efficiency
- ✅ Supports generics transparently

---

## Code Generation Insights

### Expression Context System (codegen.rs)

```rust
enum Ctx {
    Discard,       // Result is ignored: foo()
    Var(String),   // Assign to variable: x = foo()
    Arg,           // Temporary variable: __temp = foo()
}

enum EmitMode {
    Return,        // Function return context
    Expr(Ctx),     // Expression context
}
```

**Why This Matters:**
- Go has statements vs. expressions
- Borgo has expression-based syntax (like Rust)
- Context determines how to emit Go code

**Example:**

```borgo
// Borgo (expression-based)
let x = if condition { 42 } else { 0 }
```

```go
// Emitted Go (statement-based)
var x int
if condition {
    x = 42
} else {
    x = 0
}
```

### Variable Binding Strategy

```rust
// Borgo manages scope with variable renaming
let x = expr
// becomes:
var var0 <type>
var0 = expr

// Inner scope rebinds:
{
    let x = other_expr
    // becomes:
    var var1 <type>
    var1 = other_expr
}
```

**Why:**
- Prevents Go shadowing issues
- Maintains correct scoping semantics
- Enables variable reuse tracking

### Enum Code Generation

```rust
// For each enum variant, generate:
1. Tag constant
2. Struct fields for variant data
3. Constructor function
4. Pattern match code (tag checking)
```

**Example:**

```borgo
enum HttpResponse {
    Ok(body: string),
    NotFound,
    Error(code: int, message: string)
}
```

```go
// Generated Go:
type HttpResponseTag int
const (
    HttpResponse_Ok HttpResponseTag = iota
    HttpResponse_NotFound
    HttpResponse_Error
)

type HttpResponse struct {
    tag HttpResponseTag
    Ok0 string              // Ok variant
    Error0 int              // Error variant (first field)
    Error1 string           // Error variant (second field)
}

func make_HttpResponse_Ok(body string) HttpResponse {
    return HttpResponse{tag: HttpResponse_Ok, Ok0: body}
}

func make_HttpResponse_NotFound() HttpResponse {
    return HttpResponse{tag: HttpResponse_NotFound}
}

func make_HttpResponse_Error(code int, message string) HttpResponse {
    return HttpResponse{tag: HttpResponse_Error, Error0: code, Error1: message}
}
```

---

## What Dingo Should Learn from Borgo

### ✅ Adopt These Patterns

**1. Three-Stage Architecture**
```
Parser → Type Checker → Code Generator
```
- Clear separation of concerns
- Each stage independently testable
- Type information flows through pipeline

**2. Tagged Union Pattern for Enums**
```go
type Tag int
const (Variant1 Tag = iota; Variant2)
type Enum struct {
    tag Tag
    variant1_field T1
    variant2_field T2
}
```
- Works with Go's type system
- No unsafe code needed
- Efficient (single allocation)

**3. Automatic Go Interop**
```
(T, error) → Result[T, error]
(T, bool)  → Option[T]
```
- **Critical for ecosystem adoption**
- Must be transparent to users
- Implement in type checker, not runtime

**4. Type Inference**
- Reduces boilerplate dramatically
- Makes language feel modern
- Standard algorithms (Hindley-Milner)

**5. Exhaustiveness Checking**
- Compile-time verification
- Prevents bugs before runtime
- Track covered variants in pattern matches

**6. Context-Aware Code Generation**
```rust
enum Ctx { Discard, Var, Arg }
```
- Go needs different code for statements vs. expressions
- Context determines emission strategy

---

### ⚠️ Do Differently for Dingo

**1. Implementation Language**

| Borgo | Dingo |
|-------|-------|
| Written in Rust | ✅ Written in Go |
| Uses Rust parser | ✅ Custom parser (participle → tree-sitter) |
| Rust toolchain required | ✅ Pure Go (no external dependencies) |

**Why Go for Dingo:**
- ✅ Target users already have Go installed
- ✅ No Rust toolchain dependency
- ✅ Easier contribution (Go developers write Go)
- ✅ Faster iteration (no cargo compile times)

**2. Plugin Architecture**

| Borgo | Dingo |
|-------|-------|
| Monolithic compiler | ✅ **Plugin-based architecture** |
| All features always on | ✅ **Features toggleable via config** |
| Tightly coupled | ✅ **Modular, independent plugins** |

**Why Plugins for Dingo:**
- ✅ Build features incrementally
- ✅ Users enable only what they want
- ✅ Community can contribute plugins
- ✅ Easier testing (one plugin at a time)

**3. Gradual Type System**

| Borgo | Dingo |
|-------|-------|
| Fully typed from day 1 | ✅ **Start simple, add types gradually** |
| Requires type inference | ✅ **Optional type annotations** |
| Complex type system | ✅ **Pragmatic, not academic** |

**Rationale:**
- Phase 1 doesn't need full type inference
- Add complexity only when needed
- Keep it simple (Go philosophy)

**4. Source Maps**

| Borgo | Dingo |
|-------|-------|
| No source maps | ✅ **Source maps from day 1** |
| Debugging shows Go code | ✅ **Debugging shows Dingo code** |
| No LSP integration | ✅ **LSP integration planned** |

**Why Critical:**
- Error messages must reference Dingo code
- IDE features need position mapping
- Debugging experience matters

---

## Concrete Learnings for Dingo Implementation

### Phase 1: Parser

**Borgo Approach:**
- Reuses Rust's `syn` parser
- Rust-like syntax for free
- Fast, battle-tested

**Dingo Approach:**
```
Phase 1: participle (rapid prototyping)
Phase 2: tree-sitter (IDE support)
```

**Why Different:**
- Can't reuse Rust parser (we're in Go)
- participle lets us move fast
- tree-sitter later for incremental parsing

### Phase 1: Error Propagation (`?`)

**What to Copy from Borgo:**
```rust
// 1. Generate temporary variable
__result0 := expr

// 2. Check for error
if __result0.err != nil {
    return __result0.err
}

// 3. Unwrap value
value := __result0.value
```

**Dingo Plugin Implementation:**
```go
// pkg/plugin/error_propagation/transform.go

func (p *Plugin) Transform(expr *ast.ErrorPropExpr) ast.Node {
    resultVar := genTempVar()

    return &ast.Block{
        // __result := expr
        assign(resultVar, expr.Expr),

        // if __result.err != nil { return __result.err }
        errorCheck(resultVar),

        // value := __result.value
        unwrap(expr.Ident, resultVar),
    }
}
```

### Phase 2: Result Type

**What to Copy from Borgo:**
```go
// Tagged union structure
type Result[T, E] struct {
    tag ResultTag
    Ok0 T
    Err0 E
}

// Constructor functions
func make_Result_Ok[T, E](v T) Result[T, E]
func make_Result_Err[T, E](e E) Result[T, E]
```

**Dingo Approach:**
```go
// Generate this code in codegen.rs equivalent
// User writes in .dingo:
enum Result[T, E] {
    Ok(T),
    Err(E)
}

// Transpiler generates above Go code
```

### Phase 2: Pattern Matching

**What to Copy from Borgo:**
```go
// Sentinel-based matching
is_matching := 0

// Test each pattern
if value.tag == Variant1 {
    // ... pattern 1 code ...
    is_matching = 2
}

if is_matching == 0 && value.tag == Variant2 {
    // ... pattern 2 code ...
    is_matching = 2
}

// Exhaustiveness check
if is_matching == 0 {
    panic("non-exhaustive match")
}
```

**Dingo Improvement:**
```go
// Could use switch instead for cleaner Go:
switch value.tag {
case Variant1:
    // ... pattern 1 ...
case Variant2:
    // ... pattern 2 ...
default:
    panic("non-exhaustive match")
}
```

### Critical: Go Interop

**Borgo's Auto-Wrapping (MUST IMPLEMENT):**

```rust
// Type inference phase
if function_returns_tuple_with_error(func) {
    wrap_return_type_in_result(func)
}

// Example:
http.Get(url) // Returns (Response, error) in Go
// Borgo sees: Result[Response, error]
```

**Dingo Implementation:**
```go
// pkg/transform/go_interop.go

func WrapGoFunctions(file *ast.File) {
    // Find all Go imports
    for _, imp := range file.Imports {
        pkg := loadGoPackage(imp.Path)

        // Wrap each function
        for _, fn := range pkg.Functions {
            if returns_value_and_error(fn) {
                wrap_in_result(fn)
            }
            if returns_value_and_bool(fn) {
                wrap_in_option(fn)
            }
        }
    }
}
```

---

## Borgo Limitations (Opportunities for Dingo)

### Issues with Borgo

**1. No License** ⚠️
- Project is in legal limbo
- Can't fork or build upon it directly
- May be abandoned

**2. Written in Rust**
- High barrier for Go developers
- Rust toolchain required
- Slower contribution rate

**3. Monolithic**
- All features always enabled
- No plugin system
- Hard to add new features

**4. No Source Maps**
- Debugging shows Go code
- Error messages reference Go line numbers
- No IDE integration

**5. No LSP**
- No editor support
- No autocomplete
- No go-to-definition

### Dingo's Advantages

| Issue | Dingo Solution |
|-------|----------------|
| No license | ✅ MIT/Apache dual license from day 1 |
| Rust dependency | ✅ Pure Go implementation |
| Monolithic | ✅ Plugin architecture |
| No source maps | ✅ Source maps built-in |
| No LSP | ✅ LSP planned (Phase 3) |
| No community | ✅ Open development, contributor-friendly |

---

## Implementation Roadmap Based on Borgo

### Phase 1: Foundation (4-5 weeks)

**Goal:** Working transpiler with 4 features (no type inference yet)

```
Week 1: Project setup + Parser
  - participle parser
  - Basic AST
  - CLI skeleton

Week 2: Plugin system
  - Plugin interface
  - Registry
  - Config loader

Week 3: Phase 1 Plugins
  - Error Propagation (?)
  - Null Coalescing (??)

Week 4: Phase 1 Plugins + Codegen
  - Ternary Operator (? :)
  - Functional Utilities
  - Code generator
  - Source maps

Week 5: Testing + Docs
  - >80% coverage
  - Golden file tests
  - Documentation
```

**Borgo Features Used:**
- ✅ Code generation patterns
- ✅ Expression context system
- ✅ Variable binding strategy
- ❌ Type inference (not yet)
- ❌ Pattern matching (not yet)

### Phase 2: Type System (8-10 weeks)

**Goal:** Result, Option, Sum Types, Pattern Matching

**Borgo Features to Implement:**
- ✅ Type inference engine (infer.rs)
- ✅ Tagged union pattern (enums)
- ✅ Pattern matching (exhaustiveness)
- ✅ Go interop (auto-wrapping)

**New Modules Needed:**
```
pkg/
├── types/           # Type system (from Borgo's type_.rs)
├── infer/           # Type inference (from Borgo's infer.rs)
├── exhaustive/      # Exhaustiveness (from Borgo's exhaustive.rs)
└── interop/         # Go interop (auto-wrapping)
```

### Phase 3: Tooling (4-6 weeks)

**Goal:** LSP + IDE support (Borgo doesn't have this)

**What Dingo Adds:**
- ✅ Language server (gopls proxy)
- ✅ Source map translation
- ✅ Real-time transpilation
- ✅ Editor extensions

**Architecture:**
```
Dingo LSP Proxy
    ├─> Transpile .dingo → .go (in-memory)
    ├─> Forward requests to gopls
    └─> Translate responses via source maps
```

---

## Borgo-Inspired Code Examples

### Example 1: Error Propagation

**Dingo Code (inspired by Borgo):**
```dingo
func fetchUserData(id: string) -> Result[UserData, Error] {
    let resp = http.Get("/api/users/" + id)?
    let user = parseUser(resp.Body)?
    let posts = fetchPosts(user.ID)?
    Ok(UserData{user, posts})
}
```

**Transpiled Go (Borgo pattern):**
```go
func fetchUserData(id string) (UserData, error) {
    __result0 := http.Get("/api/users/" + id)
    if __result0.Err != nil {
        return UserData{}, __result0.Err
    }
    resp := __result0.Ok

    __result1 := parseUser(resp.Body)
    if __result1.Err != nil {
        return UserData{}, __result1.Err
    }
    user := __result1.Ok

    __result2 := fetchPosts(user.ID)
    if __result2.Err != nil {
        return UserData{}, __result2.Err
    }
    posts := __result2.Ok

    return UserData{user: user, posts: posts}, nil
}
```

### Example 2: Pattern Matching

**Dingo Code (Borgo-style):**
```dingo
match response {
    Ok(data) => processData(data),
    Err(error) => logError(error)
}
```

**Transpiled Go (Borgo pattern):**
```go
switch response.tag {
case Result_Ok:
    data := response.Ok0
    processData(data)
case Result_Err:
    error := response.Err0
    logError(error)
default:
    panic("unreachable: non-exhaustive match")
}
```

---

## Key Takeaways

### What Borgo Proves

✅ **Transpiling to Go is viable**
- Borgo has 4.5k stars, real users
- Demonstrates demand for Go improvements
- Proves technical feasibility

✅ **Type-safe features can be added**
- Result/Option work in Go's type system
- Pattern matching transpiles cleanly
- No runtime overhead

✅ **Go ecosystem compatibility is achievable**
- Automatic wrapping of Go functions works
- Can use entire Go stdlib + packages
- Type inference makes it seamless

✅ **Rust-inspired features fit Go**
- Error propagation (`?`) reduces boilerplate
- Enums provide safety
- Match expressions prevent bugs

### What Dingo Will Do Better

✅ **Plugin Architecture**
- Borgo is monolithic, Dingo is modular
- Users control features
- Easier to contribute

✅ **Go Implementation**
- No Rust dependency
- Easier for Go devs to contribute
- Faster iteration

✅ **Source Maps + LSP**
- Borgo has neither
- Critical for IDE support
- Better debugging experience

✅ **Licensing + Community**
- Open from day 1
- Clear license
- Active maintenance

---

## Recommended Reading (Borgo Source)

**Essential Files:**
```
compiler/src/
├── lib.rs          # Module structure
├── infer.rs        # Type inference (complex but instructive)
├── codegen.rs      # Code generation patterns
└── exhaustive.rs   # Exhaustiveness checking

compiler/test/
├── codegen-emit.md # Transpilation examples
├── infer-expr.md   # Type inference tests
└── infer-file.md   # File-level inference
```

**Study Order:**
1. `codegen.rs` - See Go code generation patterns
2. `infer.rs` - Understand type inference (Phase 2)
3. `exhaustive.rs` - Learn exhaustiveness checking
4. `codegen-emit.md` - See input/output examples

---

## Conclusion

**Borgo is an excellent reference implementation** for Dingo:
- ✅ Proves technical feasibility
- ✅ Shows what Go developers want
- ✅ Provides concrete transpilation patterns
- ✅ Demonstrates Go interop is critical

**Dingo will improve upon Borgo** by:
- ✅ Plugin architecture (modularity)
- ✅ Go implementation (accessibility)
- ✅ Source maps + LSP (tooling)
- ✅ Active community (sustainability)

**Next Steps:**
1. ✅ Study Borgo's `codegen.rs` for Phase 1 plugins
2. ✅ Implement error propagation using Borgo's pattern
3. ✅ Plan type inference based on `infer.rs` (Phase 2)
4. ✅ Design plugin architecture (Borgo doesn't have this)

**Borgo taught us what to build. Now Dingo will build it better.** 🚀
