# Lambda Void Function Bug - Fixed

**Date**: 2025-12-09
**Status**: ✅ Fixed
**Affected**: `pkg/codegen/lambda.go`

## Bug Description

Standalone lambda expressions generated invalid Go code: void functions with return statements.

### Before Fix

```go
// Generated from: fn1 := |x| x * 2
fn1 := func(x any) { return x * 2 }  // ❌ void function with return statement
```

**Compilation error**:
```
invalid operation: x * 2 (mismatched types any and int)
```

But more fundamentally, the function signature was `func(x any)` (void) while the body had `return x * 2`, which is syntactically invalid in Go.

### After Fix

```go
// Generated from: fn1 := |x| x * 2
fn1 := func(x any) any { return x * 2 }  // ✅ has return type
```

## Root Cause

**File**: `pkg/codegen/lambda.go`
**Lines**: 54-62 (old), 54-66 (new)

The lambda codegen had this logic:

```go
// OLD CODE (buggy)
if g.expr.ReturnType != "" {
    g.WriteByte(' ')
    g.Write(g.expr.ReturnType)
}
// No else clause - type inferrer will add return type if needed
```

This assumed the type inferrer would always run and add return types. However:

1. **Type inferencer only runs for lambdas in function call arguments** (`lambda_inference.go:68-72`)
2. **Standalone lambda assignments** are never visited by the inferencer
3. Result: void functions with return statements

## Fix

Add `any` return type for expression lambdas without explicit return types:

```go
// NEW CODE (fixed)
if g.expr.ReturnType != "" {
    g.WriteByte(' ')
    g.Write(g.expr.ReturnType)
} else if !g.expr.IsBlock {
    // Expression body - add 'any' return type so { return ... } is valid
    g.Write(" any")
}
// Block bodies have no default return type (may be void)
```

**Logic**:
- **Expression lambdas** (`|x| expr`): Always generate `func(...) any { return expr }`
- **Block lambdas** (`|x| { stmts }`): No default return type (user must add `return` if needed)
- **Explicit return type**: Use as-is
- **Type inferencer**: Replaces `any` with concrete types when in call context

## Test Updates

Updated all lambda tests in `pkg/codegen/lambda_test.go` to expect `any` return types:

```diff
- expected := "func(x any) { return x + 1 }"
+ expected := "func(x any) any { return x + 1 }"
```

All 13 lambda tests passing.

## Compilation Status

### ✅ Working Examples

```dingo
// With explicit types
add := |x: int, y: int| x + y  // → func(x int, y int) any { return x + y }

// Identity function
identity := |x| x  // → func(x any) any { return x }

// With explicit return type
multiply := |x: int| -> int { return x * 2 }  // → func(x int) int { return x * 2 }
```

All compile and run successfully.

### ⚠️ Known Limitation

Standalone lambdas with `any` parameters **cannot** use type-specific operations:

```dingo
// ❌ Will NOT compile
fn1 := |x| x * 2  // → func(x any) any { return x * 2 }
                   //   Error: invalid operation: x * 2 (mismatched types any and int)
```

**Workaround**: Add explicit parameter types:

```dingo
// ✅ Compiles
fn1 := |x: int| x * 2  // → func(x int) any { return x * 2 }
```

Or use the lambda in a context where type inference works:

```dingo
// ✅ Type inferencer can infer from Map's signature
numbers := []int{1, 2, 3}
doubled := Map(numbers, |x| x * 2)  // Type inferrer makes this work
```

## Impact on Examples

### `examples/101_combined/showcase.dingo`

The showcase example has:

```dingo
fn1 := |x| x * 2
fn2 := (x) => x * 3
fmt.Println(fn1(5), fn2(5))
```

**Status**:
- ✅ Transpilation works (no longer generates void functions with return)
- ❌ Compilation still fails due to `any * int` type mismatch
- **Fix**: Example should use explicit types: `|x: int| x * 2`

This is documented in the showcase's BUGS.md as "Bug 1: Lambda type inference fails (generates `any` instead of concrete types)".

## Conclusion

**Core bug fixed**: Expression lambdas no longer generate void functions with return statements.

**Remaining work**: Type inference for standalone lambdas from body expressions (future enhancement).

**Recommendation**: For now, users should:
1. Use explicit parameter types for standalone lambdas: `|x: int| expr`
2. Or use lambdas in function call contexts where type inference works
