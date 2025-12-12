# Reasoning: null_coalesce_03_chaining

## Purpose
Test chained null coalescing (`a ?? b ?? c`) with single IIFE optimization.

## Test Coverage

### 1. firstName ?? middleName ?? lastName ?? "Unknown"
```dingo
name := firstName ?? middleName ?? lastName ?? "Unknown"
```
**Expected:** Single IIFE with sequential checks
- Try firstName (None)
- Try middleName (None)
- Try lastName (Some("Smith"))
- Return lastName.Unwrap()

### 2. primary ?? fallback ?? "default"
```dingo
config := primary ?? fallback ?? "default"
```
**Expected:** Function call results
- primary = getConfig("primary") returns None
- fallback = getConfig("fallback") returns Some("backup-value")
- Return fallback.Unwrap()

### 3. opt1 ?? opt2 ?? opt3 ?? opt4 ?? "Not Found"
```dingo
result := opt1 ?? opt2 ?? opt3 ?? opt4 ?? "Not Found"
```
**Expected:** Long chain (5 operands)
- Try each in sequence
- opt4 is Some("Found")
- Return opt4.Unwrap()

## Code Generation Strategy

### Chaining Optimization
Instead of nested IIFEs:
```go
// BAD: Nested IIFEs (inefficient)
func() T {
    if a.IsSome() { return a.Unwrap() }
    return func() T {
        if b.IsSome() { return b.Unwrap() }
        return c
    }()
}()
```

Use single IIFE with if-else chain:
```go
// GOOD: Single IIFE (optimized)
func() T {
    if a.IsSome() {
        return a.Unwrap()
    }
    if b.IsSome() {
        return b.Unwrap()
    }
    return c
}()
```

### Actual Implementation
Based on golden file, uses if-else-if pattern:
```go
var name string
if firstName.IsSome() {
    name = *firstName.some
} else if middleName.IsSome() {
    name = *middleName.some
} else if lastName.IsSome() {
    name = *lastName.some
} else {
    name = "Unknown"
}
```

**This is even better:** No IIFE wrapper, direct variable assignment.

## Performance Benefits
- Single scope (no nested closures)
- Early return when value found
- Compiler can optimize if-else chain
- Minimal stack depth

## Edge Cases Tested
- All None until last Option
- Function call results
- Long chains (5+ operands)
- Mixed Option and literal fallback
