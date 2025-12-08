# Dingo Feature Showcase - Known Bugs

This document tracks bugs found when combining all Dingo features in `showcase.dingo`.

## Bug 1: Lambda Type Inference Fails

**Feature**: 6. Lambdas

**Code**:
```dingo
fn1 := |x| x * 2
```

**Generated Go**:
```go
fn1 := func(x any) any { return x * 2 }
```

**Error**:
```
invalid operation: x * 2 (mismatched types any and int)
```

**Root Cause**: Lambda type inference generates `any` instead of inferring `int` from the `* 2` operation.

---

## Bug 2: Ternary Doesn't Declare Variable

**Feature**: 9. Ternary

**Code**:
```dingo
let greeting = user.ID > 0 ? "Welcome" : "Hello"
```

**Generated Go**:
```go
if user.ID > 0 {
    greeting = "Welcome"  // Assignment without declaration!
} else {
    greeting = "Hello"
}
```

**Error**:
```
undefined: greeting
```

**Root Cause**: Ternary codegen generates assignment statements without variable declaration when used with `let`.

---

## Bug 3: Tuple Destructuring Mishandles Go Multiple Returns

**Feature**: 7. Tuples

**Code**:
```dingo
let (x, y) = getPoint()  // getPoint() returns (int, int)
```

**Generated Go**:
```go
tmp := getPoint()
x := tmp.First   // Wrong! Go multiple returns aren't tuples
y := tmp.Second
```

**Error**:
```
assignment mismatch: 1 variable but getPoint returns 2 values
```

**Root Cause**: Tuple destructuring treats Go's multiple return values as Dingo tuple structs with `.First`/`.Second` fields, but they're just multiple values.

---

## Features Working Correctly

The following features work correctly in the showcase:

1. ✅ **Enum** - Sum types with variants
2. ✅ **Match** - Pattern matching on enums
3. ✅ **Error Propagation** - All 3 patterns (`?`, `? "msg"`, `? |e| f(e)`)
4. ✅ **Result** - `Result<T, E>` with `Ok[T]()`/`Err[T]()`
5. ✅ **Option** - `Option<T>` with `Some[T]()`/`None[T]()`
8. ✅ **Safe Navigation** - `obj?.field`
10. ✅ **Null Coalesce** - `a ?? b`
11. ✅ **Guard Let** - `guard let x = expr else { ... }`
12. ✅ **Let Binding** - `let x = expr`

## Features With Bugs

6. ❌ **Lambdas** - Type inference fails (Bug 1)
7. ❌ **Tuples** - Destructuring mishandles Go multiple returns (Bug 3)
9. ❌ **Ternary** - Variable declaration missing (Bug 2)
