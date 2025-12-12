# Reasoning: null_coalesce_02_integration

## Purpose
Test null coalescing (`??`) operator integration with safe navigation (`?.`) operator.

## Test Coverage

### 1. user?.name ?? "unknown"
```dingo
userName := user?.name ?? "unknown"
```
**Expected:** IIFE with safe nav IIFE inside
- Safe navigation returns string (not Option)
- Null check for empty string ""
- Fall back to literal "unknown"

### 2. user?.address?.city ?? defaultCity
```dingo
city := user?.address?.city ?? defaultCity
```
**Expected:** Nested IIFE
- Outer IIFE for ??
- Inner IIFE for ?.
- Chain returns StringOption
- Unwrap if Some, else use defaultCity identifier

### 3. profile?.getEmail() ?? "no-email@example.com"
```dingo
email := profile?.getEmail() ?? "no-email@example.com"
```
**Expected:** Method call integration
- Safe navigation with method call
- Method returns StringOption
- Unwrap if Some, else literal fallback

### 4. Multiple safe nav with different fallbacks
```dingo
zip := user2?.address?.zip ?? "00000"
city2 := user2?.address?.city ?? "Unknown City"
```
**Expected:** Two separate ?? operations
- Each uses safe navigation
- Different fallback values
- Independent evaluation

## Code Generation Strategy

### Complex Case Detection
All cases are "complex" because left operand is safe navigation chain:
- Left: `user?.name` (safe nav chain)
- Right: literal or identifier
- **Result:** IIFE with intermediate variable

### Expected Pattern
```go
var result = func() T {
    __safeNav := func() T {
        // Safe navigation logic
    }()
    if __safeNav != nil/IsSome() {
        return __safeNav
    }
    return fallback
}()
```

## Integration Points
- Safe navigation processor runs FIRST
- Null coalescing processor sees safe nav result
- Type detection for return type (string vs StringOption)
- Proper unwrapping based on type

## Edge Cases Tested
- String properties (direct access)
- Option properties (StringOption)
- Method calls returning Option
- Multiple independent ?? operations
- Identifier vs literal fallbacks
