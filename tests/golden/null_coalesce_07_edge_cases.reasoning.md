# Reasoning: null_coalesce_07_edge_cases

## Purpose
Test null coalescing (`??`) edge cases and special scenarios.

## Test Coverage

### 1. Empty string as fallback
```dingo
let name: StringOption = StringOption_Some("")
display := name ?? "Default"
```
**Expected:** Some("") should return ""
- CRITICAL: Empty string is valid Some value
- Should NOT fall through to "Default"
- Tests that ?? checks IsSome(), not truthiness

### 2. Zero value as fallback
```dingo
let zero: BoolOption = BoolOption_Some(false)
result := zero ?? true
```
**Expected:** Some(false) should return false
- CRITICAL: false is valid Some value
- Should NOT fall through to true
- Tests boolean handling

### 3. Nested parentheses
```dingo
combined := (opt1 ?? opt2) ?? "Fallback"
```
**Expected:** Nested ?? evaluation
- Inner: opt1 ?? opt2 → "Found"
- Outer: "Found" ?? "Fallback" → "Found"
- Tests precedence and grouping

### 4. Boolean context
```dingo
enabled := flag ?? false
```
**Expected:** BoolOption handling
- None -> false
- Some(true) -> true
- Some(false) -> false

### 5. Multiple ?? in same expression
```dingo
a := first ?? "A"
b := second ?? "B"
concat := a + b
```
**Expected:** Independent evaluation
- Each ?? evaluates separately
- Results combined in expression
- No interference between operations

### 6. ?? with method chain
```dingo
upper := text?.toUpper() ?? "DEFAULT"
```
**Expected:** Safe nav + method + ??
- text is Some("hello")
- toUpper() returns Some("hello")
- ?? should not activate

## Code Generation Strategy

### Zero Value Handling
CRITICAL: Must distinguish:
- None (no value) -> use fallback
- Some(zero) (has zero value) -> use zero value

**Implementation:**
```go
if option.IsSome() {
    return *option.some  // Even if zero!
} else {
    return fallback
}
```

### Nested Evaluation
Parentheses force evaluation order:
```go
__inner := func() T {
    if opt1.IsSome() { return opt1.Unwrap() }
    if opt2.IsSome() { return opt2.Unwrap() }
    return ""
}()
if __inner != "" {
    return __inner
} else {
    return "Fallback"
}
```

### Boolean Special Case
Bool requires same logic as other types:
```go
if flag.IsSome() {
    return *flag.some  // Could be false!
} else {
    return false  // Fallback
}
```

## Edge Cases Tested
- Empty string (valid Some)
- False boolean (valid Some)
- Nested parentheses
- Boolean type handling
- Multiple independent operations
- Method chains with ??

## Common Pitfalls Avoided
❌ **WRONG:** Check if value is "truthy"
```go
if *option.some {  // BAD: false Some becomes None
    return *option.some
}
```

✅ **CORRECT:** Check if Option is Some
```go
if option.IsSome() {  // GOOD: checks tag
    return *option.some  // Could be false!
}
```

## Integration Points
- IsSome() tag checking (not value checking)
- Parentheses precedence
- Safe navigation integration
- Method calls

**Last Updated**: 2025-11-20
