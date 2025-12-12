# Ternary Operator

**Priority:** P3 (Lower - User choice feature)
**Status:** 🔴 Not Started
**Complexity:** 🟢 Low (2-3 days implementation)
**Community Demand:** ⭐⭐ (Repeatedly requested despite Go team rejection)
**Inspiration:** C, Java, JavaScript, Python, TypeScript

---

## Overview

The ternary operator `condition ? trueValue : falseValue` provides concise conditional expressions, eliminating verbose if/else blocks for simple value selection.

## Motivation

### The Problem in Go

```go
// Verbose for simple conditional assignment
var status string
if user.IsActive {
    status = "active"
} else {
    status = "inactive"
}

// Or even more verbose
func getStatus(user User) string {
    if user.IsActive {
        return "active"
    }
    return "inactive"
}

// Painful in string formatting
fmt.Printf("You have %d friend%s\n", count, func() string {
    if count == 1 {
        return ""
    }
    return "s"
}())
```

**Go Team's Stance:**
- **Rejected multiple times** (FAQ, multiple proposals)
- "A language needs only one conditional control flow construct"
- Concern about nested ternaries reducing readability
- If/else is "unquestionably clearer"

---

## Why Dingo Should Implement It Anyway

### Meta-Language Advantage

**Key Insight:** Dingo transpiles to Go - we don't need to follow Go's philosophy if:
1. ✅ Transpilation is clean and idiomatic
2. ✅ No runtime overhead
3. ✅ Feature is optional (users choose to use it or not)

### Counter-Arguments to Go's Rejection

**Go's Concern:** "Reduces readability with nesting"
**Dingo's Response:**
- Linter can warn on nested ternaries (discourage abuse)
- Most use cases are simple, single-level conditionals
- Other languages handle this just fine with style guides
- **Users get to choose** - if you don't like it, don't use it

**Go's Concern:** "One way to do things"
**Dingo's Response:**
- Go already has multiple ways: if/else statement, if/else expression (in functions), switch
- Dingo is explicitly about **adding options** that Go doesn't provide
- Transpiled Go still follows "one way" (uses if/else)

**Go's Concern:** "Verbosity forces clarity"
**Dingo's Response:**
- Ternary is MORE clear for simple cases: `max = a > b ? a : b`
- Compare to: `var max int; if a > b { max = a } else { max = b }`
- Intent is obvious: "choose value based on condition"

### Real-World Demand

- ✅ Requested since Go 1.0 (2009)
- ✅ Common in **every** mainstream language (C, Java, JS, Python, Ruby, PHP, C#)
- ✅ Developers coming from other languages expect it
- ✅ Specific use cases where it shines (string templates, default values)

---

## Proposed Syntax

### Basic Ternary

```dingo
// Simple conditional assignment
max := a > b ? a : b
status := isActive ? "active" : "inactive"
message := hasError ? "Error occurred" : "Success"

// In expressions
println("You have ${count} friend${count == 1 ? "" : "s"}")
println("Status: ${user.isAdmin ? "Admin" : "User"}")

// Return values
func getLevel(score: int) -> string {
    return score >= 90 ? "A" :
           score >= 80 ? "B" :
           score >= 70 ? "C" : "F"
}
```

### Type Checking

```dingo
// ✅ Both branches must have same type
value := condition ? 42 : 0  // OK: both int

// ❌ Type mismatch
value := condition ? 42 : "string"  // ERROR: incompatible types

// ✅ Type inference
x := true ? "yes" : "no"  // Inferred as string

// ✅ Works with Option types
user := findUser(id) ? user : User.guest()
```

### Style Guidelines (Recommended)

```dingo
// ✅ Good: Simple, readable
max := a > b ? a : b

// ✅ OK: Reasonable chaining
grade := score >= 90 ? "A" : score >= 80 ? "B" : "C"

// ⚠️ Discouraged: Deep nesting (linter warning)
x := a ? b ? c ? d : e : f : g

// ❌ Forbidden: Side effects in ternary (linter error)
x := condition ? doThing1() : doThing2()  // Use if/else instead
```

---

## Transpilation Strategy

### Simple Case

```dingo
// Dingo source
max := a > b ? a : b
```

```go
// Transpiled Go
var max int
if a > b {
    max = a
} else {
    max = b
}
```

### In Expressions

```dingo
// Dingo source
println("Status: ${user.isActive ? "active" : "inactive"}")
```

```go
// Transpiled Go
var __ternary0 string
if user.isActive {
    __ternary0 = "active"
} else {
    __ternary0 = "inactive"
}
fmt.Println(fmt.Sprintf("Status: %s", __ternary0))
```

### Chained Ternaries

```dingo
// Dingo source
grade := score >= 90 ? "A" : score >= 80 ? "B" : "C"
```

```go
// Transpiled Go
var grade string
if score >= 90 {
    grade = "A"
} else if score >= 80 {
    grade = "B"
} else {
    grade = "C"
}
```

### Optimization

```go
// For simple cases, can use helper function (inlined)
func __ternary_int(cond bool, ifTrue, ifFalse int) int {
    if cond {
        return ifTrue
    }
    return ifFalse
}

// Usage (when both branches are pure values)
max := __ternary_int(a > b, a, b)
```

---

## Implementation Details

### Parsing

```ebnf
TernaryExpr = OrExpr ( "?" Expr ":" TernaryExpr )?
```

**Precedence:** Lower than logical OR, higher than assignment
- `a || b ? c : d` parses as `(a || b) ? c : d`
- `a ? b : c = d` is syntax error (use parens)

### Type Checking

```
1. Evaluate condition type (must be bool)
2. Evaluate true branch type: T1
3. Evaluate false branch type: T2
4. Verify T1 == T2 (or one converts to other)
5. Result type is T1 (or common supertype)
```

### AST Representation

```go
type TernaryExpr struct {
    Condition Expr
    TrueExpr  Expr
    FalseExpr Expr
    Type      Type  // Resolved type
    Pos       token.Pos
}
```

### Linter Rules

```
WARNING: Ternary depth > 2 levels
  Suggest: Refactor to if/else or match expression

WARNING: Side effects in ternary branches
  Suggest: Use if/else statement instead

WARNING: Complex expression as condition
  Suggest: Extract to variable for clarity
```

---

## Complexity Analysis

**Implementation Complexity:** 🟢 Low

### Parsing (1 day)
- Add ternary to expression grammar
- Handle precedence correctly
- Parse tests

### Type Checking (1 day)
- Verify condition is bool
- Check branch type compatibility
- Infer result type
- Type checker tests

### Transpilation (1 day)
- Generate if/else statement
- Handle nested ternaries (else-if chain)
- Optimize pure value cases
- Integration tests

**Total: 2-3 days** for complete implementation

---

## Benefits vs Tradeoffs

### Advantages
- ✅ **Massive readability win** for simple cases
- ✅ **Zero runtime cost** (transpiles to if/else)
- ✅ **Familiar** to 90%+ of developers
- ✅ **Optional** - don't like it? Don't use it
- ✅ **Trivial complexity** - 3 days to implement
- ✅ **String template friendliness** - `"${x ? "yes" : "no"}"`

### Potential Concerns
- ❓ **Nested abuse** (people write unreadable code)
  - *Mitigation:* Linter warnings, style guide
- ❓ **"Not Go-like"** (violates Go philosophy)
  - *Response:* Dingo is explicitly NOT Go - we add what Go won't
- ❓ **Beginners confused** (what does `?:` mean?)
  - *Response:* Universally understood, good documentation

### Trade-off Decision

**Implement:** The benefits massively outweigh concerns
- Low complexity (3 days)
- High demand (every other language has it)
- Clean transpilation
- User opt-in (not forced)

---

## Examples

### Example 1: Min/Max

```dingo
min := a < b ? a : b
max := a > b ? a : b

// Go equivalent: 5 lines each
var min int
if a < b {
    min = a
} else {
    min = b
}
```

### Example 2: String Formatting

```dingo
println("You have ${count} item${count == 1 ? "" : "s"}")
println("Status: ${isOnline ? "🟢 Online" : "🔴 Offline"}")

// Go equivalent: Painful
var suffix string
if count == 1 {
    suffix = ""
} else {
    suffix = "s"
}
fmt.Printf("You have %d item%s\n", count, suffix)
```

### Example 3: Default Values

```dingo
port := env.get("PORT") ?? "8080"
host := config.host ? config.host : "localhost"

// Or with null coalescing (better)
host := config.host ?? "localhost"
```

### Example 4: Grade Calculation

```dingo
func getGrade(score: int) -> string {
    return score >= 90 ? "A" :
           score >= 80 ? "B" :
           score >= 70 ? "C" :
           score >= 60 ? "D" : "F"
}

// Go equivalent: 10+ lines of if/else
```

### Example 5: React-Style Rendering (if Dingo had UI)

```dingo
statusIcon := user.isActive ? "✓" : "✗"
className := isSelected ? "selected" : "normal"
display := isVisible ? "block" : "none"
```

---

## Alternative Syntax Considered

### Python-Style (Rejected)

```python
# Python: value_if_true if condition else value_if_false
max = a if a > b else b
```

**Why Not:**
- Order is confusing (condition in middle)
- Less familiar to C-family developers
- Harder to chain

### Postfix `?` (Rejected)

```
max = (a > b).then(a).else(b)
```

**Why Not:**
- Too verbose (defeats the purpose)
- Unfamiliar syntax
- Not worth inventing new pattern

### `if` Expression (Considered)

```dingo
max := if a > b { a } else { b }
```

**Why Not:**
- More verbose than `?:`
- Could still implement this **in addition** to ternary
- Different use case (multi-line conditionals)

**Decision:** Implement both ternary AND if-expressions

---

## Comparison with Other Languages

| Language | Syntax | Notes |
|----------|--------|-------|
| C/C++ | `a ? b : c` | Original, proven |
| Java | `a ? b : c` | Same as C |
| JavaScript | `a ? b : c` | Same, very common |
| TypeScript | `a ? b : c` | Same |
| Python | `b if a else c` | Different order |
| Ruby | `a ? b : c` | Same as C |
| Rust | `if a { b } else { c }` | Expression-based if |
| Swift | `a ? b : c` | Same as C |
| **Dingo** | `a ? b : c` | **Follow majority** |

**Consensus:** C-style `?:` is the de facto standard

---

## Success Criteria

- [ ] Ternary operator works in all expression contexts
- [ ] Type checking catches mismatched branch types
- [ ] Transpiled code is readable if/else
- [ ] Linter warns on nested/complex usage
- [ ] Zero performance overhead vs manual if/else
- [ ] Positive feedback from developers used to other languages

---

## References

- Go FAQ: "Why no ternary operator?"
- C Operator Precedence: https://en.cppreference.com/w/c/language/operator_precedence
- JavaScript Conditional Operator: https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Operators/Conditional_Operator
- Python Ternary: https://docs.python.org/3/reference/expressions.html#conditional-expressions

---

## Conclusion: Pragmatism Over Philosophy

**Go's Decision:** Reject ternary to maintain simplicity
**Dingo's Decision:** Implement ternary because:

1. ✅ Transpilation is trivial (3 days)
2. ✅ Every other language has it (developer expectation)
3. ✅ Clear wins for simple cases (string templates, min/max)
4. ✅ Optional feature (style choice)
5. ✅ Linter prevents abuse

**Result:** Dingo developers get concise syntax without sacrificing readability

---

## Next Steps

1. Implement parser support for `?:` operator
2. Add type checking for branch compatibility
3. Generate if/else transpilation
4. Create linter rules for nested usage
5. Document best practices
6. Gather community feedback on style guidelines
