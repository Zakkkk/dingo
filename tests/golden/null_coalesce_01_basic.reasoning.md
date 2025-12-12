# Reasoning: null_coalesce_01_basic

## Purpose
Test basic null coalescing (`??`) operator with simple cases that trigger inline optimization.

## Test Coverage

### 1. Simple Option ?? Literal
```dingo
displayName := name ?? "Guest"
```
**Expected:** Inline IIFE (simple operands, no nesting)
- Left: single identifier (`name`)
- Right: string literal (`"Guest"`)
- Generate: if-else check with IsSome/Unwrap

### 2. Simple Option ?? Option
```dingo
result := name ?? fallback
```
**Expected:** Inline IIFE
- Left: single identifier
- Right: single identifier
- Both are Option types, unwrap both

### 3. Option<int> ?? Literal
```dingo
finalCount := count ?? 0
```
**Expected:** Inline IIFE with int type
- Demonstrates type inference works for numeric literals
- Same pattern as string, different type

### 4. Multiple separate ?? operations
```dingo
firstDisplay := firstName ?? "Anonymous"
ageDisplay := age ?? 18
```
**Expected:** Independent IIFE for each
- Each evaluated separately
- Different types (string, int)

## Code Generation Strategy

### Inline Optimization
All cases in this test are "simple" according to complexity heuristics:
- Single identifier on left
- Literal or identifier on right
- No nested `??`
- No function calls

**Expected:** Direct IIFE without intermediate variables

### Type Handling
- **Option types:** Use `IsSome()` and direct access to `.some`
- Generated code uses `*option.some` pattern

## Edge Cases Tested
- String types
- Integer types
- Option fallback to Option (both operands Option)
- Multiple independent operations

## Integration Points
- Type detection (StringOption vs IntOption)
- Inline complexity classification
- Type inference from right operand

**Last Updated**: 2025-11-20
