# Guard Let Statement

Swift-inspired guard let statement for Result/Option unwrapping with explicit error binding.

## Status
- **Phase**: 10
- **Status**: Complete
- **Date**: 2025-12-04
- **Breaking Change**: v0.11.0 - Pipe binding syntax

## Syntax

```dingo
// Result type - explicit pipe binding REQUIRED when using error
guard let <variable> = <result_expr> else |err| {
    // 'err' is explicitly bound from the Result error value
    <early_return_or_action>
}

// Result type - no binding when error not used
guard let <variable> = <result_expr> else {
    // No error binding needed
    <early_return_or_action>
}

// Option type - no binding allowed
guard let <variable> = <option_expr> else {
    // Option types have no error to bind
    <early_return_or_action>
}

// <variable> is available here with unwrapped value
```

## Pipe Binding Rules

### Result Types

1. **Explicit binding REQUIRED when using error**:
   ```dingo
   guard let user = FindUser(id) else |err| { return err }  // ✅ Valid
   ```

2. **Custom binding names supported**:
   ```dingo
   guard let user = FindUser(id) else |e| { return e }      // ✅ Valid
   ```

3. **No binding when error not used**:
   ```dingo
   guard let user = FindUser(id) else { return defaults }   // ✅ Valid
   ```

4. **Implicit error usage is COMPILE ERROR**:
   ```dingo
   guard let user = FindUser(id) else { return err }        // ❌ COMPILE ERROR
   ```

### Option Types

1. **No binding allowed** (Option has no error):
   ```dingo
   guard let user = maybeUser else { return nil }           // ✅ Valid
   ```

2. **Pipe binding is COMPILE ERROR**:
   ```dingo
   guard let user = maybeUser else |val| { return nil }     // ❌ COMPILE ERROR
   ```

## Examples

### Result Type - Explicit Binding

```dingo
func GetUserOrder(userID int) Result {
    guard let user = FindUser(userID) else |err| { return err }
    guard let order = GetOrder(user.ID) else |err| { return err }

    return ResultOk(fmt.Sprintf("%s: %s", user.Name, order.ID))
}
```

### Result Type - Custom Error Name

```dingo
func GetUserOrder(userID int) Result {
    guard let user = FindUser(userID) else |e| {
        log.Error("user lookup failed", e)
        return e
    }
    guard let order = GetOrder(user.ID) else |e| {
        log.Error("order lookup failed", e)
        return e
    }

    return ResultOk(fmt.Sprintf("%s: %s", user.Name, order.ID))
}
```

### Result Type - No Error Binding

```dingo
func GetUserOrDefault(userID int) User {
    // Error not needed, using default value
    guard let user = FindUser(userID) else { return defaultUser }
    return user
}
```

### Option Type

```dingo
func ProcessUser(maybeUser Option) string {
    guard let user = maybeUser else {
        return "no user"  // No pipe binding for Option
    }
    return user.Name
}
```

### Inline Syntax

```dingo
guard let from = FindUser(fromID) else |err| { return err }
guard let to = FindUser(toID) else |err| { return err }
```

### Custom Error Handling with Binding

```dingo
guard let user = FindUser(id) else |err| {
    log.Error("Failed to find user", err)
    return ServiceError{Code: "NOT_FOUND", Message: err.Error()}
}
```

## Generated Code

### Result Type with Explicit Binding - Input
```dingo
guard let user = userResult else |err| {
    return err
}
fmt.Println(user.Name)
```

### Result Type with Explicit Binding - Output
```go
tmp := userResult
// dingo:g:0
if tmp.IsErr() {
    err := *tmp.err
    return ResultErr(err)
}
user := *tmp.ok
fmt.Println(user.Name)
```

### Result Type with Custom Binding - Input
```dingo
guard let user = FindUser(id) else |e| {
    log.Error("failed", e)
    return e
}
```

### Result Type with Custom Binding - Output
```go
tmp := FindUser(id)
// dingo:g:0
if tmp.IsErr() {
    e := *tmp.err
    log.Error("failed", e)
    return ResultErr(e)
}
user := *tmp.ok
```

### Result Type without Binding - Input
```dingo
guard let config = LoadConfig() else {
    return defaults
}
```

### Result Type without Binding - Output
```go
tmp := LoadConfig()
// dingo:g:0
if tmp.IsErr() {
    return defaults
}
config := *tmp.ok
```

### Option Type Input
```dingo
guard let user = maybeUser else {
    return "no user"
}
```

### Option Type Output
```go
tmp := maybeUser
// dingo:g:0
if tmp.IsNone() {
    return "no user"
}
user := *tmp.value
```

## Type Detection

Guard let determines the expression type using:

1. **Function signature scanning** - Parses `func Name() ReturnType` declarations
2. **TypeRegistry lookup** - Queries registered variables and functions
3. **Naming convention fallback** - Checks for "Result" or "Option" in expression

## Compile Errors

### Error: Implicit Error Without Binding

When you use `err` in the else block without explicit pipe binding:

```dingo
guard let user = FindUser(id) else { return err }  // ❌ COMPILE ERROR
```

**Error Message**:
```
guard_let.dingo:5:1: error: implicit 'err' not allowed: use explicit binding |err| or |e|
    guard let user = FindUser(id) else { return err }
                                         ^^^^^^^^^
    hint: Change: else { return err } -> else |err| { return err }
```

**Fix**:
```dingo
guard let user = FindUser(id) else |err| { return err }  // ✅ Fixed
```

### Error: Pipe Binding on Option Type

When you try to use pipe binding with Option types:

```dingo
guard let first = items.First() else |val| { return nil }  // ❌ COMPILE ERROR
```

**Error Message**:
```
guard_let.dingo:10:1: error: pipe binding not allowed on Option types (no error to bind)
    guard let first = items.First() else |val| { return nil }
                                         ^^^^^
    hint: Option types only have Some/None, not an error value
```

**Fix**:
```dingo
guard let first = items.First() else { return nil }  // ✅ Fixed
```

## Design Decisions

### Explicit Pipe Binding (Breaking Change)

**Why mandatory `|err|` syntax?**

1. **Explicit is better than implicit**: No "magic" `err` variable appearing without declaration
2. **Consistency**: Matches Dingo's lambda syntax pattern (`|x| { ... }`)
3. **Clarity**: Reader immediately sees where error variable comes from
4. **Type safety**: Option types clearly have no error to bind (compile error prevents confusion)

**Why compile error instead of warning?**

1. **Clean break**: Pre-release project, no backward compatibility burden
2. **No ambiguity**: Code either compiles or it doesn't
3. **Forces conscious choice**: Users must adopt new explicit syntax

### Custom Error Names with Auto-Wrapping

When you use a custom error name like `|e|`, the return transformation applies:

```dingo
guard let user = FindUser(id) else |e| { return e }
```

Generates:
```go
if tmp.IsErr() {
    e := *tmp.err
    return ResultErr(e)  // Auto-wrapped!
}
```

**Benefits**:
1. **Consistency**: Whether `|err|` or `|e|`, behavior is identical
2. **Convenience**: `return e` is more natural than `return ResultErr(e)`
3. **Error-prone reduction**: Can't forget to wrap the error

### Error Variable Name: Standard `err`
Uses standard Go convention when binding is specified. User handles shadowing themselves.

### Temp Variable Strategy: Smart Detection
- Simple identifiers (e.g., `userResult`) → use directly
- Function calls (e.g., `FindUser(id)`) → use temp variable to avoid multiple evaluations

### Return Transformation
`return <binding>` in Result else blocks with pipe binding automatically transforms to `return ResultErr(<binding>)`.

## Comparison: Before and After

### Before (16 lines)
```dingo
func GetUserOrderTotal(db *sql.DB, userID int) Result {
    userResult := FindUser(db, userID)
    if userResult.IsErr() {
        return userResult.UnwrapErr()
    }
    user := userResult.Unwrap()

    ordersResult := FindOrdersByUser(db, user.ID)
    if ordersResult.IsErr() {
        return ordersResult.UnwrapErr()
    }
    orders := ordersResult.Unwrap()

    var total float64
    for _, order := range orders {
        total += order.Total
    }
    return total
}
```

### After (8 lines, 50% reduction)
```dingo
func GetUserOrderTotal(db *sql.DB, userID int) Result {
    guard let user = FindUser(db, userID) else { return err }
    guard let orders = FindOrdersByUser(db, user.ID) else { return err }

    var total float64
    for _, order := range orders {
        total += order.Total
    }
    return total
}
```

## Migration Guide (Breaking Change in v0.11.0)

### Overview

Version 0.11.0 introduces **mandatory pipe binding syntax** for guard let statements with Result types. This is a breaking change that removes implicit error binding.

### What Changed

**Before (v0.10.x and earlier)** - Implicit `err` binding:
```dingo
guard let user = FindUser(id) else { return err }  // ❌ No longer works
```

**After (v0.11.0+)** - Explicit pipe binding required:
```dingo
guard let user = FindUser(id) else |err| { return err }  // ✅ Required
```

### Migration Steps

#### Step 1: Identify Affected Code

Search your codebase for guard let statements that use `err` in the else block:

```bash
grep -n "guard let.*else.*{.*err" **/*.dingo
```

#### Step 2: Add Pipe Binding

For each occurrence, add `|err|` after `else`:

**Pattern to find**:
```dingo
else { return err }
else { log.Error(err); return err }
```

**Pattern to replace with**:
```dingo
else |err| { return err }
else |err| { log.Error(err); return err }
```

#### Step 3: Optional - Use Custom Names

You can use any valid identifier instead of `err`:

```dingo
// Before
guard let user = FindUser(id) else { return err }

// After - with custom name
guard let user = FindUser(id) else |e| { return e }
```

#### Step 4: Handle Cases Without Error Usage

If your else block doesn't use the error value, you can omit the pipe binding:

```dingo
// Before
guard let user = FindUser(id) else { return defaultUser }

// After - no change needed!
guard let user = FindUser(id) else { return defaultUser }
```

### Automated Migration

A simple sed command can migrate most cases:

```bash
# Migrate simple return err cases
sed -i 's/else { return err }/else |err| { return err }/g' **/*.dingo

# For more complex cases, use find + sed
find . -name "*.dingo" -exec sed -i 's/else { return err }/else |err| { return err }/g' {} +
```

**Warning**: This only handles simple cases. Complex else blocks with error usage require manual review.

### Common Migration Patterns

#### Pattern 1: Simple Error Return

```diff
- guard let user = FindUser(id) else { return err }
+ guard let user = FindUser(id) else |err| { return err }
```

#### Pattern 2: Error Logging

```diff
- guard let user = FindUser(id) else {
-     log.Error("lookup failed", err)
-     return err
- }
+ guard let user = FindUser(id) else |err| {
+     log.Error("lookup failed", err)
+     return err
+ }
```

#### Pattern 3: Error Wrapping

```diff
- guard let user = FindUser(id) else {
-     return fmt.Errorf("user not found: %w", err)
- }
+ guard let user = FindUser(id) else |err| {
+     return fmt.Errorf("user not found: %w", err)
+ }
```

#### Pattern 4: Default Values (No Change)

```diff
  guard let user = FindUser(id) else { return defaultUser }
  // No change needed - no error usage
```

#### Pattern 5: Option Types (No Change)

```diff
  guard let first = items.First() else { return nil }
  // No change needed - Option types never had error binding
```

### Validation

After migration, run your Dingo compiler to catch any remaining issues:

```bash
dingo build ./...
```

The compiler will report any places where:
1. `err` is used without explicit `|err|` binding
2. Pipe binding is incorrectly used on Option types

### Why This Change?

1. **Explicit over implicit**: The `err` variable's origin is now clear
2. **Consistency**: Matches Dingo's lambda syntax (`|x| { ... }`)
3. **Type safety**: Prevents confusion between Result and Option types
4. **Better IDE support**: Explicit binding enables better autocomplete and type inference

### Need Help?

If you encounter migration issues:
1. Check the compile error messages (they include fix hints)
2. Review the examples above
3. See the full documentation in `features/guard_let.md`

## Implementation

- **Processor**: `pkg/preprocessor/guard_let_ast.go` (enhanced with pipe binding parser)
- **Pipeline Position**: Pass 1 (Structural), position 1 (after DingoPreParser)
- **Type Info**: Uses TypeRegistry from `pkg/registry/`
- **Breaking Change**: v0.11.0

## Golden Tests

- `tests/golden/guard_let_result.dingo` - Result type scenarios with pipe binding
- `tests/golden/guard_let_option.dingo` - Option type scenarios (no binding)
- `tests/golden/guard_let_inline.dingo` - Inline syntax variations
- `tests/golden/guard_let_tuple.dingo` - Multiple guard lets
- `tests/golden/guard_let_pipe.dingo` - Pipe binding specific tests
- `tests/golden/guard_let_errors.dingo` - Compile error test cases
