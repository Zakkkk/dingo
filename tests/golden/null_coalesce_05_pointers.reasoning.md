# Reasoning: null_coalesce_05_pointers

## Purpose
Test null coalescing (`??`) operator with raw Go pointer types (dual type support).

## Test Coverage

### 1. Pointer with safe nav ?? literal
```dingo
name := user?.name ?? "Unknown"
```
**Expected:** Dual operation
- Safe nav on pointer: `if user == nil`
- Null coalesce: check result, use "Unknown"
- Type: string (from pointer field access)

### 2. Pointer ?? pointer
```dingo
result := primary ?? fallback
```
**Expected:** Pointer chain
- Both operands are *string
- Check primary != nil
- Check fallback != nil
- Dereference with *

### 3. Nested pointer access
```dingo
age := userPtr?.age ?? 0
```
**Expected:** Safe nav + ??
- Pointer nil check
- Field access
- Integer fallback

### 4. Explicit nil handling
```dingo
let nilUser: *User = nil
nilName := nilUser?.name ?? "Default"
```
**Expected:** Nil propagates correctly
- nilUser == nil
- Safe nav returns ""
- ?? provides "Default"

## Code Generation Strategy

### Pointer Detection
Transpiler must distinguish:
- Option<T> types: Use IsSome()/IsNone()
- Pointer types: Use == nil / != nil

### Pointer Dereferencing
When pointer is Some:
```go
if primary != nil {
    return *primary  // Dereference
}
```

### Zero Value for Empty Check
Pointers don't have IsSome(), so use:
- String: `!= ""`
- Int: `!= 0`
- Pointer: `!= nil`

## Edge Cases Tested
- Pointer with safe navigation
- Pointer ?? pointer (both operands pointers)
- Explicit nil assignment
- Field access on pointers
- Integer fallbacks

## Integration Points
- Safe navigation with pointers
- Type detection (pointer vs Option)
- Dereference operator *
- Nil checks

## Dual Type Support
This test validates that ?? works on:
- Option<T> types (other tests)
- Raw Go pointers (*T) (this test)

**Last Updated**: 2025-11-20
