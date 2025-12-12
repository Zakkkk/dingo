# Architectural Plan: Null Safety, Null Coalescing, Ternary, and Lambda Features

## Executive Summary

This document outlines the complete architectural design for implementing four new Dingo features:
1. Null Safety Operator (`?.`)
2. Null Coalescing Operator (`??`)
3. Ternary Operator (`? :`)
4. Lambda Functions

All features follow Dingo's established plugin architecture, reuse existing AST infrastructure, and transpile to clean, idiomatic Go code with zero runtime overhead.

---

## 1. Null Safety Operator (`?.`)

### 1.1 Overview

The null safety operator provides safe navigation through potentially nil values, returning `Option[T]` instead of panicking.

### 1.2 Syntax Design

```dingo
// Basic chaining
city := user?.address?.city?.name

// With method calls
email := user?.getEmail()?.lowercase()

// Combined with null coalescing
city := user?.address?.city?.name ?? "Unknown"
```

### 1.3 AST Representation

**New Node Type:** `SafeNavigationExpr` (already partially defined as `NullCoalescingExpr` needs update)

```go
// pkg/ast/ast.go - Add new node
type SafeNavigationExpr struct {
    X      ast.Expr  // The expression being safely accessed
    OpPos  token.Pos // Position of '?.'
    Sel    *ast.Ident // The selector (field or method name)
}

func (s *SafeNavigationExpr) Pos() token.Pos { return s.X.Pos() }
func (s *SafeNavigationExpr) End() token.Pos { return s.Sel.End() }
func (*SafeNavigationExpr) exprNode() {}
```

### 1.4 Parser Modifications

**File:** `pkg/parser/participle.go`

**Changes Needed:**

1. Add `SafeNav` token to lexer rules:
```go
{Name: "SafeNav", Pattern: `\?\\.`}  // Must come before single '?'
```

2. Update `PostfixExpression`:
```go
type PostfixExpression struct {
    Primary        *PrimaryExpression  `parser:"@@"`
    SafeNavs       []*SafeNavigation   `parser:"@@*"`  // Multiple chainable ?.
    ErrorPropagate *bool               `parser:"@'?'?"`
    ErrorMessage   *string             `parser:"( @String )?"`
}

type SafeNavigation struct {
    OpPos token.Pos
    Selector string `parser:"'?' '.' @Ident"`
}
```

3. Parser precedence: `?.` has same precedence as `.`, higher than `?` (error propagation)

### 1.5 Transpilation Strategy

**Plugin:** `pkg/plugin/builtin/safe_navigation.go`

```dingo
// Dingo source
city := user?.address?.city?.name
```

```go
// Transpiled Go
var city Option_string
if user != nil {
    if user.Address != nil {
        if user.Address.City != nil {
            city = Option_Some(user.Address.City.Name)
        } else {
            city = Option_None[string]()
        }
    } else {
        city = Option_None[string]()
    }
} else {
    city = Option_None[string]()
}
```

**Optimization for chaining:**
```go
// More efficient nested check
var city Option_string
if user != nil && user.Address != nil && user.Address.City != nil {
    city = Option_Some(user.Address.City.Name)
} else {
    city = Option_None[string]()
}
```

### 1.6 Plugin Architecture

```go
type SafeNavigationPlugin struct {
    plugin.BasePlugin
    currentContext *plugin.Context
    currentFile    *dingoast.File
}

func (p *SafeNavigationPlugin) Name() string {
    return "safe_navigation"
}

func (p *SafeNavigationPlugin) Dependencies() []string {
    return []string{"option_type"}  // Depends on Option[T]
}

func (p *SafeNavigationPlugin) Transform(ctx *plugin.Context, node ast.Node) (ast.Node, error) {
    // 1. Find all SafeNavigationExpr nodes
    // 2. Build chain of conditions
    // 3. Generate nested if statements or combined condition
    // 4. Return Option_Some(value) or Option_None()
}
```

### 1.7 Integration with Existing Features

- **With Option Type:** Returns `Option[T]` naturally
- **With Null Coalescing:** `user?.name ?? "Unknown"` works seamlessly
- **With Pattern Matching:** Can match on returned Option
- **With Error Propagation:** `user?.fetchData()? "failed"` chains both operators

---

## 2. Null Coalescing Operator (`??`)

### 2.1 Overview

Provides default values for `Option[T]` or nullable values. Syntactic sugar for `UnwrapOr()`.

### 2.2 Syntax Design

```dingo
// Basic usage
name := user?.name ?? "Anonymous"

// Chaining multiple fallbacks
value := primary ?? secondary ?? tertiary ?? "default"

// With expressions
port := env.get("PORT")?.parseInt() ?? 8080
```

### 2.3 AST Representation

**Existing Node:** Already defined in `pkg/ast/ast.go` as `NullCoalescingExpr` (perfect!)

```go
type NullCoalescingExpr struct {
    X      ast.Expr  // Left operand (nullable/Option value)
    OpPos  token.Pos // Position of '??'
    Y      ast.Expr  // Right operand (default value)
}
```

### 2.4 Parser Modifications

**File:** `pkg/parser/participle.go`

**Changes Needed:**

1. Add to lexer:
```go
{Name: "NullCoalesce", Pattern: `\?\?`}  // Must come before single '?'
```

2. Add new expression level (precedence between OR and ternary):
```go
type Expression struct {
    NullCoalesce *NullCoalesceExpression `parser:"@@"`
}

type NullCoalesceExpression struct {
    Left  *TernaryExpression `parser:"@@"`
    Op    string             `parser:"( @'?' '?'"`
    Right *NullCoalesceExpression `parser:"  @@ )?"`  // Right-associative
}

type TernaryExpression struct {
    Comparison *ComparisonExpression `parser:"@@"`
    // ... ternary handling
}
```

3. Precedence: Lower than `?:`, higher than assignment

### 2.5 Transpilation Strategy

**Plugin:** `pkg/plugin/builtin/null_coalescing.go`

```dingo
// Dingo source
name := user?.name ?? "Anonymous"
```

```go
// Transpiled Go (simple case)
var name string
if __opt0.IsSome() {
    name = __opt0.Unwrap()
} else {
    name = "Anonymous"
}

// Or using helper method
name := __opt0.UnwrapOr("Anonymous")
```

**For chaining:**
```dingo
value := a ?? b ?? c ?? "default"
```

```go
// Transpiled (nested evaluation, right-to-left)
var value T
__tmp2 := c.UnwrapOr("default")
__tmp1 := b.UnwrapOr(__tmp2)
value = a.UnwrapOr(__tmp1)
```

### 2.6 Plugin Architecture

```go
type NullCoalescingPlugin struct {
    plugin.BasePlugin
    currentContext *plugin.Context
}

func (p *NullCoalescingPlugin) Dependencies() []string {
    return []string{"option_type"}
}

func (p *NullCoalescingPlugin) Transform(ctx *plugin.Context, node ast.Node) (ast.Node, error) {
    // 1. Identify NullCoalescingExpr
    // 2. Check if left side is Option[T]
    // 3. Generate UnwrapOr call or if/else check
    // 4. Handle chaining (right-associative)
}
```

### 2.7 Type Checking

**Key Rules:**
1. Left operand must be `Option[T]` or pointer type
2. Right operand must have compatible type with `T`
3. Result type is `T` (unwrapped)

**Examples:**
```dingo
// Valid
let name: string = optionalName ?? "default"  // Option[string] ?? string -> string
let user: User = optionalUser ?? User{}       // Option[User] ?? User -> User

// Invalid
x := someOption ?? 42  // ERROR if someOption is Option[string]
```

---

## 3. Ternary Operator (`? :`)

### 3.1 Overview

Concise conditional expressions. Not to be confused with error propagation `?` or safe navigation `?.`.

### 3.2 Syntax Design

```dingo
// Basic
max := a > b ? a : b
status := isActive ? "active" : "inactive"

// In expressions
println("You have ${count} friend${count == 1 ? "" : "s"}")

// Chaining (discouraged but allowed)
grade := score >= 90 ? "A" : score >= 80 ? "B" : "C"
```

### 3.3 AST Representation

**Existing Node:** Already defined in `pkg/ast/ast.go` as `TernaryExpr` (perfect!)

```go
type TernaryExpr struct {
    Cond     ast.Expr  // Condition
    Question token.Pos // Position of '?'
    Then     ast.Expr  // True branch
    Colon    token.Pos // Position of ':'
    Else     ast.Expr  // False branch
}
```

### 3.4 Parser Modifications

**File:** `pkg/parser/participle.go`

**Critical Challenge: Operator Disambiguation**

We now have THREE uses of `?`:
1. Error propagation: `expr?`
2. Safe navigation: `expr?.field`
3. Ternary: `cond ? then : else`

**Solution: Context-aware parsing**

```go
// Precedence hierarchy (highest to lowest):
// 1. Postfix operators: .field, [index], (args), ?.field
// 2. Unary operators: !, -, &, *
// 3. Binary operators: *, /, %, +, -, <, >, ==, etc.
// 4. Ternary: ? :
// 5. Null coalescing: ??
// 6. Assignment: =, :=

// Parsing strategy:
type TernaryExpression struct {
    Comparison *ComparisonExpression `parser:"@@"`
    Question   *bool                 `parser:"@'?'?"`
    Then       *TernaryExpression    `parser:"( @@ ':'"`
    Else       *TernaryExpression    `parser:"  @@ )?"`
}

// The key: '?' is only ternary if followed by non-'.' and there's a ':'
// Parser looks ahead to distinguish:
// - expr?      -> error propagation (no colon follows)
// - expr?.foo  -> safe navigation (followed by '.')
// - expr ? x : y -> ternary (colon found)
```

**Lexer token ordering (critical!):**
```go
{Name: "SafeNav", Pattern: `\?\\.`},      // Match first: ?.
{Name: "NullCoalesce", Pattern: `\?\?`},  // Match second: ??
{Name: "Question", Pattern: `\?`},         // Match last: ? (ambiguous)
```

### 3.5 Transpilation Strategy

**Plugin:** `pkg/plugin/builtin/ternary.go`

```dingo
// Dingo source (simple)
max := a > b ? a : b
```

```go
// Transpiled Go (statement context)
var max int
if a > b {
    max = a
} else {
    max = b
}
```

```dingo
// Dingo source (expression context)
println(user.isActive ? "active" : "inactive")
```

```go
// Transpiled Go (needs statement lifting)
var __ternary0 string
if user.isActive {
    __ternary0 = "active"
} else {
    __ternary0 = "inactive"
}
println(__ternary0)
```

**For chaining:**
```dingo
grade := score >= 90 ? "A" : score >= 80 ? "B" : "C"
```

```go
// Transpiled Go (else-if chain)
var grade string
if score >= 90 {
    grade = "A"
} else if score >= 80 {
    grade = "B"
} else {
    grade = "C"
}
```

### 3.6 Plugin Architecture

```go
type TernaryPlugin struct {
    plugin.BasePlugin
    statementLifter *StatementLifter  // Reuse from error_propagation
    tmpCounter      int
}

func (p *TernaryPlugin) Dependencies() []string {
    return nil  // No dependencies
}

func (p *TernaryPlugin) Transform(ctx *plugin.Context, node ast.Node) (ast.Node, error) {
    // 1. Identify TernaryExpr nodes
    // 2. Check context (statement vs expression)
    // 3. Generate if/else statement
    // 4. If in expression context, lift to temp variable
    // 5. Handle chained ternaries (nested else-if)
}
```

### 3.7 Type Checking

**Rules:**
1. Condition must be `bool`
2. Then and Else branches must have same type (or compatible)
3. Result type is the common type of branches

```go
func (p *TernaryPlugin) checkTypes(ctx *plugin.Context, ternary *dingoast.TernaryExpr) error {
    // 1. Verify condition is bool
    condType := ctx.TypeInfo.TypeOf(ternary.Cond)
    if !isBool(condType) {
        return fmt.Errorf("ternary condition must be bool, got %v", condType)
    }

    // 2. Check branch compatibility
    thenType := ctx.TypeInfo.TypeOf(ternary.Then)
    elseType := ctx.TypeInfo.TypeOf(ternary.Else)
    if !typesCompatible(thenType, elseType) {
        return fmt.Errorf("ternary branches have incompatible types: %v vs %v", thenType, elseType)
    }

    return nil
}
```

---

## 4. Lambda Functions

### 4.1 Overview

Concise function literals supporting multiple syntactic styles. Most complex feature due to syntax variations and closure handling.

### 4.2 Syntax Design Options

**Decision: Support all three styles** (parsed differently, transpile identically)

#### Style 1: Rust/Closure Style (Primary)
```dingo
// Basic
add := |a, b| a + b

// With types
parse := |s: string| -> int { parseInt(s) }

// No parameters
getRandom := || rand.Int()

// Block body
process := |x| {
    println("Processing ${x}")
    return x * 2
}
```

#### Style 2: Arrow Function Style (Secondary)
```dingo
// Basic
add := (a, b) => a + b

// Single parameter (no parens)
double := x => x * 2

// With types
parse := (s: string): int => parseInt(s)

// Block body
process := (x) => {
    println("Processing ${x}")
    return x * 2
}
```

#### Style 3: Trailing Lambda (Advanced, Phase 2)
```dingo
// When last parameter is function
users.filter { |u| u.age > 18 }
    .map { |u| u.name }

// With implicit 'it'
users.filter { it.age > 18 }
```

**Implementation Priority:**
1. **Phase 1:** Rust-style `|params| body` (simplest, unambiguous)
2. **Phase 2:** Arrow-style `params => body` (requires lookahead)
3. **Phase 3:** Trailing lambda `{ }` syntax (most complex)

### 4.3 AST Representation

**Existing Node:** Already defined in `pkg/ast/ast.go` as `LambdaExpr` (perfect!)

```go
type LambdaExpr struct {
    Pipe   token.Pos      // Position of '|' or '(' or '{'
    Params *ast.FieldList // Parameters (reuse go/ast!)
    Arrow  token.Pos      // Position of '=>' or '->' (if present)
    Body   ast.Expr       // Body (expression or BlockStmt)
    Rpipe  token.Pos      // Position of closing '|', ')', '}'
}
```

**Additional metadata for styles:**
```go
type LambdaStyle int

const (
    LambdaStyleRust LambdaStyle = iota  // |a, b| expr
    LambdaStyleArrow                     // (a, b) => expr
    LambdaStyleTrailing                  // { expr }
)

// Extend LambdaExpr
type LambdaExpr struct {
    // ... existing fields
    Style LambdaStyle  // Which syntax was used
}
```

### 4.4 Parser Modifications

**File:** `pkg/parser/participle.go`

**Phase 1: Rust Style Only**

```go
type PrimaryExpression struct {
    Lambda  *LambdaExpression  `parser:"  @@"`
    Match   *Match             `parser:"| @@"`
    Call    *CallExpression    `parser:"| @@"`
    Number  *int64             `parser:"| @Int"`
    // ... rest
}

type LambdaExpression struct {
    Pipe    token.Pos          `parser:"'|'"`
    Params  []*LambdaParam     `parser:"( @@ ( ',' @@ )* )?"`
    Rpipe   token.Pos          `parser:"'|'"`
    Arrow   *bool              `parser:"@('-' '>')?"`  // Optional ->
    RetType *Type              `parser:"( @@ )?"`      // Optional return type
    Body    *LambdaBody        `parser:"@@"`
}

type LambdaParam struct {
    Name string `parser:"@Ident"`
    Type *Type  `parser:"( ':' @@ )?"`  // Optional type annotation
}

type LambdaBody struct {
    Block      *Block      `parser:"  @@"`      // Block: |x| { ... }
    Expression *Expression `parser:"| @@"`      // Expression: |x| x + 1
}
```

**Phase 2: Add Arrow Style**

```go
// More complex - needs lookahead to distinguish from parenthesized expressions
// Strategy: Parse as ParenExpr first, then check for '=>' and convert
```

### 4.5 Transpilation Strategy

**Plugin:** `pkg/plugin/builtin/lambda.go`

```dingo
// Dingo source
add := |a, b| a + b
users.filter(|u| u.age > 18)
```

```go
// Transpiled Go
var add = func(a int, b int) int {
    return a + b
}

users.filter(func(u User) bool {
    return u.age > 18
})
```

**With type inference:**
```dingo
// Dingo (types inferred from context)
numbers := strings.map(|s| parseInt(s))
```

```go
// Transpiled (types resolved)
var numbers []int = strings.map(func(s string) int {
    return parseInt(s)
})
```

**Block bodies:**
```dingo
process := |x| {
    println("Processing ${x}")
    result := x * 2
    return result
}
```

```go
var process = func(x int) int {
    println(fmt.Sprintf("Processing %d", x))
    var result int = x * 2
    return result
}
```

### 4.6 Plugin Architecture

```go
type LambdaPlugin struct {
    plugin.BasePlugin
    typeInference *TypeInference
}

func (p *LambdaPlugin) Dependencies() []string {
    return nil  // Standalone feature
}

func (p *LambdaPlugin) Transform(ctx *plugin.Context, node ast.Node) (ast.Node, error) {
    // 1. Find LambdaExpr nodes
    // 2. Infer parameter types from context (if not explicit)
    // 3. Infer return type from body
    // 4. Convert to ast.FuncLit
    // 5. Handle implicit return for expression bodies
}
```

### 4.7 Type Inference

**Critical for good UX:** Lambda parameters should be inferred from context

```dingo
// Context provides types
let filter: func(User) bool = |u| u.age > 18
//                              ^ infer u: User

// From function signature
func processUsers(predicate: func(User) bool) { ... }
processUsers(|u| u.age > 18)
//            ^ infer u: User
```

**Implementation:**
```go
func (p *LambdaPlugin) inferLambdaTypes(
    ctx *plugin.Context,
    lambda *dingoast.LambdaExpr,
    expectedType types.Type,
) error {
    // 1. Check if expectedType is a function type
    funcType, ok := expectedType.(*types.Signature)
    if !ok {
        return fmt.Errorf("cannot infer lambda types: expected function type")
    }

    // 2. Match parameter count
    if funcType.Params().Len() != len(lambda.Params.List) {
        return fmt.Errorf("parameter count mismatch")
    }

    // 3. Assign inferred types to parameters
    for i, param := range lambda.Params.List {
        if param.Type == nil {  // Type not explicitly provided
            param.Type = typeToAST(funcType.Params().At(i).Type())
        }
    }

    // 4. Infer return type from body
    // ...

    return nil
}
```

### 4.8 Closure Capture

**Go handles this automatically!** Since we transpile to Go's `func()` literals, closure capture is free:

```dingo
// Dingo
func makeMultiplier(factor: int) -> func(int) int {
    return |x| x * factor  // Captures 'factor'
}
```

```go
// Transpiled Go
func makeMultiplier(factor int) func(int) int {
    return func(x int) int {
        return x * factor  // Go handles capture
    }
}
```

---

## 5. Cross-Feature Integration

### 5.1 Operator Precedence Table

**Critical: All operators must have well-defined precedence**

| Precedence | Operators | Associativity | Example |
|------------|-----------|---------------|---------|
| 14 | `()` `[]` `.` `?.` | Left | `user?.address.city` |
| 13 | `!` `-` (unary) | Right | `!user.isActive` |
| 12 | `*` `/` `%` | Left | `a * b / c` |
| 11 | `+` `-` | Left | `a + b - c` |
| 10 | `<` `>` `<=` `>=` | Left | `a < b` |
| 9 | `==` `!=` | Left | `a == b` |
| 8 | `&&` | Left | `a && b` |
| 7 | `||` | Left | `a || b` |
| **6** | `? :` (ternary) | **Right** | `a ? b : c ? d : e` |
| **5** | `??` (null coalescing) | **Right** | `a ?? b ?? c` |
| 4 | `?` (error prop) | Postfix | `expr?` |
| 3 | `=` `:=` | Right | `a = b = c` |

**Key Decisions:**
- `?.` has same precedence as `.` (both are member access)
- Ternary is lower than logical OR (matches C-family languages)
- Null coalescing is lower than ternary (can use ternary in either operand)
- Error propagation is postfix (separate from ternary `?`)

### 5.2 Combined Usage Examples

```dingo
// Example 1: All features together
result := user?.address?.city?.name ??
             config?.defaultCity ??
             "Unknown"

// Example 2: Ternary with null coalescing
status := user?.isActive ?? false ? "active" : "inactive"

// Example 3: Lambda with safe navigation
users.map(|u| u?.address?.city ?? "No city")

// Example 4: Everything combined
processUser := |u: User| -> string {
    city := u?.address?.city?.name ?? "Unknown"
    return city.length > 0 ? city : "N/A"
}

// Example 5: Error propagation with null coalescing
user := fetchUser(id)? ?? User.guest()
```

### 5.3 Plugin Execution Order

**Dependency Graph:**
```
option_type (base)
    ├── safe_navigation (depends on Option[T])
    ├── null_coalescing (depends on Option[T])
    └── result_type (parallel to Option)
        └── error_propagation (depends on Result)

ternary (independent)
lambda (independent)
```

**Execution Order (after topological sort):**
1. `option_type`
2. `result_type`
3. `safe_navigation`
4. `null_coalescing`
5. `error_propagation`
6. `ternary`
7. `lambda`

This ensures that when `safe_navigation` transforms `user?.name` to `Option_Some(...)`, the `option_type` plugin has already generated the `Option_Some` constructor.

---

## 6. Implementation Roadmap

### 6.1 Phase 1: Foundation (Week 1)

**Goal:** Get basic operators working in isolation

**Tasks:**
1. **Null Coalescing (`??`)** - Simplest
   - [ ] Add lexer token for `??`
   - [ ] Update parser grammar
   - [ ] Implement `NullCoalescingPlugin`
   - [ ] Write tests (basic, chained, with Option)
   - **Estimate:** 1-2 days

2. **Ternary (`? :`)** - Medium complexity
   - [ ] Disambiguate `?` in lexer
   - [ ] Add ternary expression to parser
   - [ ] Implement `TernaryPlugin`
   - [ ] Statement lifting for expression contexts
   - [ ] Write tests (simple, chained, nested)
   - **Estimate:** 2-3 days

### 6.2 Phase 2: Safe Navigation (Week 2)

**Goal:** Implement `?.` with full chaining support

**Tasks:**
1. **Safe Navigation (`?.`)**
   - [ ] Add `SafeNavigationExpr` AST node
   - [ ] Update lexer (distinguish `?.` from `?`)
   - [ ] Parser support for chaining
   - [ ] Implement `SafeNavigationPlugin`
   - [ ] Optimize nested conditions
   - [ ] Integration with Option[T]
   - [ ] Write tests (basic, chained, with methods)
   - **Estimate:** 3-4 days

### 6.3 Phase 3: Lambda Functions (Week 3)

**Goal:** Rust-style lambdas working

**Tasks:**
1. **Lambda (Rust style only)**
   - [ ] Update parser for `|params| body`
   - [ ] Implement `LambdaPlugin`
   - [ ] Type inference for parameters
   - [ ] Return type inference
   - [ ] Block vs expression bodies
   - [ ] Write tests (basic, in HOF, closures)
   - **Estimate:** 4-5 days

2. **Arrow style** (optional, time permitting)
   - [ ] Add arrow syntax parsing
   - [ ] Update lambda plugin
   - **Estimate:** 1-2 days

### 6.4 Phase 4: Integration & Polish (Week 4)

**Tasks:**
1. **Cross-feature testing**
   - [ ] Test all operator combinations
   - [ ] Verify precedence is correct
   - [ ] Edge cases and error handling

2. **Documentation**
   - [ ] Update feature docs
   - [ ] Add examples to README
   - [ ] Write migration guide

3. **Performance optimization**
   - [ ] Optimize generated code
   - [ ] Benchmark transpilation time

**Total Estimate:** 3-4 weeks for all four features

---

## 7. Testing Strategy

### 7.1 Unit Tests (Per Plugin)

**For each plugin:**
```go
// pkg/plugin/builtin/xxx_test.go

func TestNullCoalescing_Basic(t *testing.T) {
    // Test: a ?? b
}

func TestNullCoalescing_Chained(t *testing.T) {
    // Test: a ?? b ?? c
}

func TestNullCoalescing_WithOption(t *testing.T) {
    // Test: Option[T] ?? default
}

func TestNullCoalescing_TypeMismatch(t *testing.T) {
    // Test: Error on incompatible types
}
```

### 7.2 Golden File Tests

**Pattern:** `tests/golden/feature_NN_description.dingo`

```
tests/golden/
├── nullcoalesce_01_basic.dingo
├── nullcoalesce_01_basic.go.golden
├── nullcoalesce_02_chained.dingo
├── nullcoalesce_02_chained.go.golden
├── ternary_01_simple.dingo
├── ternary_01_simple.go.golden
├── ternary_02_nested.dingo
├── ternary_02_nested.go.golden
├── safenav_01_basic.dingo
├── safenav_01_basic.go.golden
├── safenav_02_chained.dingo
├── safenav_02_chained.go.golden
├── lambda_01_rust_style.dingo
├── lambda_01_rust_style.go.golden
├── combined_01_all_operators.dingo
└── combined_01_all_operators.go.golden
```

### 7.3 Integration Tests

**Test combined features:**
```dingo
// tests/golden/combined_01_all_operators.dingo
package main

func processUser(user: User?) -> string {
    // Safe nav + null coalescing
    city := user?.address?.city?.name ?? "Unknown"

    // Ternary
    status := user?.isActive ?? false ? "active" : "inactive"

    // Lambda
    formatter := |s: string| -> string {
        return s.length > 0 ? s : "N/A"
    }

    return formatter(city) + " - " + status
}
```

### 7.4 Precedence Tests

**Critical: Verify operator precedence**
```dingo
// Test: ternary vs null coalescing
x := a ? b : c ?? d  // Should parse as: a ? b : (c ?? d)

// Test: safe nav vs error prop
y := user?.fetch()? "error"  // Should parse as: (user?.fetch())? "error"

// Test: complex chaining
z := a?.b?.c ?? d?.e?.f ?? g
```

---

## 8. Identified Gaps & Ambiguities

### 8.1 Parser Disambiguation

**Gap:** How to distinguish three uses of `?`:
1. Ternary: `cond ? a : b`
2. Error propagation: `expr?`
3. Safe navigation: `expr?.field`

**Solution:** Lexer token ordering + lookahead
- Match `?.` first (2-char token)
- Match `??` second (2-char token)
- Match `?` last (ambiguous, context-dependent)

**Questions:**
- Should we allow `??` without left side being Option? (e.g., `nil ?? "default"`)
- Should ternary work with Option values? (e.g., `optionalBool ? "yes" : "no"`)

### 8.2 Lambda Style Choice

**Gap:** Should we support all three styles or pick one?

**Recommendation:** Start with Rust-style only, add others later
- **Rust style** `|x| expr` is unambiguous and simple
- **Arrow style** `x => expr` requires complex lookahead
- **Trailing style** `{ expr }` conflicts with blocks

**Question:** Should we use `->` or `:` for return type annotation?
```dingo
|x: int| -> string { ... }  // Rust-like
|x: int|: string { ... }    // Alternative
```

### 8.3 Safe Navigation Return Type

**Gap:** Should `?.` return `Option[T]` always, or only when chaining?

**Options:**
1. **Always Option:** `user?.name` returns `Option[string]`
   - Consistent, composable
   - More unwrapping needed

2. **Smart unwrap:** Single `?.` doesn't wrap if context expects T
   - Less consistent
   - More intuitive for simple cases

**Recommendation:** Always return Option for consistency

### 8.4 Null Coalescing with Pointers

**Gap:** Should `??` work with Go pointers directly?

```dingo
let name: string = user.Name ?? "default"  // user.Name is *string
```

**Options:**
1. Require explicit Option wrapping
2. Auto-convert pointers to Option in this context
3. Make `??` work with both Option and pointers

**Question:** Do we want pointer null-checking or Option-only?

### 8.5 Lambda Type Inference Scope

**Gap:** How aggressive should type inference be?

**Examples:**
```dingo
// Clear from context
users.filter(|u| u.age > 18)  // u inferred as User

// Less clear
fn := |x| x + 1  // What is x? int? float64?

// No context
process := |data| { ... }  // Error or default to 'any'?
```

**Questions:**
- Require type annotations when context is ambiguous?
- Default to `any` (interface{}) and let Go's type checker complain?
- Emit warning and require explicit types?

### 8.6 Operator Precedence Edge Cases

**Gap:** Some precedence interactions are ambiguous

```dingo
// How does this parse?
x := a ?? b ? c : d

// Option 1: (a ?? b) ? c : d
// Option 2: a ?? (b ? c : d)

// Recommendation: Option 1 (null coalescing before ternary)
```

**Question:** Should we require parentheses in ambiguous cases?

### 8.7 Error Propagation + Safe Navigation

**Gap:** How do these combine?

```dingo
data := user?.fetchData()? "error"
```

**Parse options:**
1. `(user?.fetchData())? "error"` - Safe nav first, then error prop
2. `user?.(fetchData()? "error")` - Error prop first, then safe nav

**Recommendation:** Option 1 (left-to-right evaluation)

---

## 9. Success Criteria

### 9.1 Feature Completeness

- [ ] All four operators implemented and tested
- [ ] Parser handles all syntax variations
- [ ] Plugins integrate with existing features
- [ ] Golden tests pass for all examples

### 9.2 Code Quality

- [ ] Generated Go code is idiomatic
- [ ] No runtime overhead (zero-cost abstraction)
- [ ] Proper error messages for type mismatches
- [ ] Source maps track all transformations

### 9.3 Developer Experience

- [ ] Clear error messages for parsing failures
- [ ] Type inference works as expected
- [ ] Documentation with examples
- [ ] Migration guide from Go patterns

### 9.4 Performance

- [ ] Transpilation time < 100ms for 1000 LOC
- [ ] Generated code compiles without warnings
- [ ] No memory leaks in parser/transformer

---

## 10. Summary

This architecture plan provides:

1. **Complete AST design** for all four features
2. **Parser modifications** with disambiguation strategy
3. **Plugin architecture** with clear dependencies
4. **Transpilation strategy** for each operator
5. **Integration plan** showing how features compose
6. **Testing strategy** with golden files and unit tests
7. **Implementation roadmap** with time estimates
8. **Identified gaps** requiring user decisions

**Next Steps:**
1. User reviews this plan and answers gap questions
2. Implementation begins with Phase 1 (simplest operators)
3. Iterative development with continuous testing
4. Integration and polish in final phase

**Estimated Timeline:** 3-4 weeks for all features
