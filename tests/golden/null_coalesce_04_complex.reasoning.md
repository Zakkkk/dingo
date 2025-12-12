# Reasoning: null_coalesce_04_complex

## Purpose
Test complex null coalescing expressions with nested operations, function calls, and arithmetic.

## Test Coverage

### 1. Function call on right side
```dingo
display := name ?? getDefaultName()
```
**Expected:** IIFE with function call fallback
- Left: identifier (simple)
- Right: function call (complex)
- **Result:** IIFE pattern

### 2. Nested ?? in expressions
```dingo
finalPort := (port ?? 8080) + 1000
```
**Expected:** ?? result used in arithmetic
- Evaluate `port ?? 8080` first
- Add 1000 to result
- Type: int

### 3. Multiple ?? in single expression
```dingo
host := config?.host ?? getEnv("HOST") ?? "localhost"
```
**Expected:** Triple chain
- Safe nav: config?.host
- Function call: getEnv("HOST")
- Literal: "localhost"
- Try each in sequence

### 4. ?? with method calls
```dingo
username := user?.toUpper() ?? getGuestName() ?? "GUEST"
```
**Expected:** Method on left, function on right
- Safe nav with method call
- Function call fallback
- Literal final fallback

### 5. Nested function calls
```dingo
finalTimeout := timeout ?? parseEnv("TIMEOUT") ?? getDefaultTimeout()
```
**Expected:** All function calls
- Each operand evaluated lazily
- First Some() wins
- getDefaultTimeout() returns int directly

### 6. Mixed with arithmetic
```dingo
total := (count ?? 10) * 2
```
**Expected:** ?? in arithmetic context
- Parentheses enforce order
- Result multiplied by 2

## Code Generation Strategy

### Complexity Classification
All cases are "complex" because:
- Right operand has function calls
- Nested in expressions
- Chained with other operators

**Expected:** IIFE with intermediate variables

### Lazy Evaluation
Key requirement: Right operand only evaluated if left is None
```go
var display = func() string {
    if name.IsSome() {
        return *name.some
    }
    return getDefaultName()  // Only called if name is None
}()
```

### Expression Context
When ?? is part of larger expression:
```go
var finalPort = func() int {
    if port.IsSome() {
        return *port.some
    }
    return 8080
}() + 1000  // IIFE result used in addition
```

## Performance Implications
- Function calls only happen when needed (lazy)
- IIFE overhead for complex cases
- Arithmetic happens after ?? evaluation
- Compiler can inline simple functions

## Edge Cases Tested
- Function call fallbacks
- Arithmetic with ?? results
- Triple chaining
- Method calls in chains
- Mixed Option and direct returns
- Expressions as operands

## Integration Points
- Safe navigation (?.)
- Method calls
- Function calls
- Arithmetic operators
- Parentheses for precedence
