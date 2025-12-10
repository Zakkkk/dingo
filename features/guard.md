# Guard Statement

Swift-inspired guard statement for Result/Option unwrapping with explicit error binding.

## Status
- **Phase**: 10
- **Status**: Complete
- **Date**: 2025-12-04
- **Breaking Change**: v0.12.0 - Removed `guard let` syntax, use `guard :=` instead

## Migration Note (v0.12.0)

⚠️ **Breaking Change**: The `guard let` syntax has been removed. Use Go-style `:=` and `=` operators instead:

```diff
- guard let x = expr else { ... }
+ guard x := expr else { ... }
```

This aligns guard syntax with standard Go declaration patterns.

## Syntax

```dingo
// Declare new variable with := (most common)
guard <variable> := <result_or_option_expr> else |err| {
    // 'err' is explicitly bound from the Result error value
    <early_return_or_action>
}

// Assign to existing variable with =
guard <variable> = <result_or_option_expr> else |err| {
    // Updates existing variable instead of declaring new one
    <early_return_or_action>
}

// No error binding when error not used
guard <variable> := <expr> else {
    // No error binding needed
    <early_return_or_action>
}

// Option type - no binding allowed
guard <variable> := <option_expr> else {
    // Option types have no error to bind
    <early_return_or_action>
}

// <variable> is available here with unwrapped value
```

### Declaration vs Assignment

- **`guard x := expr`**: Declares new variable `x` (like Go's `:=`)
- **`guard x = expr`**: Assigns to existing variable `x` (like Go's `=`)

## Pipe Binding Rules

### Result Types

1. **Explicit binding REQUIRED when using error**:
   ```dingo
   guard user := FindUser(id) else |err| { return err }  // ✅ Valid
   ```

2. **Custom binding names supported**:
   ```dingo
   guard user := FindUser(id) else |e| { return e }      // ✅ Valid
   ```

3. **No binding when error not used**:
   ```dingo
   guard user := FindUser(id) else { return defaults }   // ✅ Valid
   ```

4. **Implicit error usage is COMPILE ERROR**:
   ```dingo
   guard user := FindUser(id) else { return err }        // ❌ COMPILE ERROR
   ```

### Option Types

1. **No binding allowed** (Option has no error):
   ```dingo
   guard user := maybeUser else { return nil }           // ✅ Valid
   ```

2. **Pipe binding is COMPILE ERROR**:
   ```dingo
   guard user := maybeUser else |val| { return nil }     // ❌ COMPILE ERROR
   ```

## Examples

### Result Type - Explicit Binding (Declaration)

```dingo
func GetUserOrder(userID int) Result {
    guard user := FindUser(userID) else |err| { return err }
    guard order := GetOrder(user.ID) else |err| { return err }

    return ResultOk(fmt.Sprintf("%s: %s", user.Name, order.ID))
}
```

### Result Type - Assignment to Existing Variable

```dingo
func RefreshUser(user User) Result {
    // Reassign to existing 'user' variable
    guard user = FindUser(user.ID) else |err| { return err }
    return ResultOk(user)
}
```

### Result Type - Custom Error Name

```dingo
func GetUserOrder(userID int) Result {
    guard user := FindUser(userID) else |e| {
        log.Error("user lookup failed", e)
        return e
    }
    guard order := GetOrder(user.ID) else |e| {
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
    guard user := FindUser(userID) else { return defaultUser }
    return user
}
```

### Option Type

```dingo
func ProcessUser(maybeUser Option) string {
    guard user := maybeUser else {
        return "no user"  // No pipe binding for Option
    }
    return user.Name
}
```

### Inline Syntax

```dingo
guard from := FindUser(fromID) else |err| { return err }
guard to := FindUser(toID) else |err| { return err }
```

### Custom Error Handling with Binding

```dingo
guard user := FindUser(id) else |err| {
    log.Error("Failed to find user", err)
    return ServiceError{Code: "NOT_FOUND", Message: err.Error()}
}
```

## Generated Code

### Result Type with Declaration (:=) - Input
```dingo
guard user := userResult else |err| {
    return err
}
fmt.Println(user.Name)
```

### Result Type with Declaration (:=) - Output
```go
tmp := userResult
// dingo:g:0
if tmp.IsErr() {
    err := *tmp.err
    return ResultErr(err)
}
user := *tmp.ok  // Declaration with :=
fmt.Println(user.Name)
```

### Result Type with Assignment (=) - Input
```dingo
var user User
guard user = FindUser(id) else |err| {
    return err
}
```

### Result Type with Assignment (=) - Output
```go
var user User
tmp := FindUser(id)
// dingo:g:0
if tmp.IsErr() {
    err := *tmp.err
    return ResultErr(err)
}
user = *tmp.ok  // Assignment with =
```

### Result Type with Custom Binding - Input
```dingo
guard user := FindUser(id) else |e| {
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
guard config := LoadConfig() else {
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
guard user := maybeUser else {
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

Guard determines the expression type using:

1. **Function signature scanning** - Parses `func Name() ReturnType` declarations
2. **TypeRegistry lookup** - Queries registered variables and functions
3. **Naming convention fallback** - Checks for "Result" or "Option" in expression

## Compile Errors

### Error: Implicit Error Without Binding

When you use `err` in the else block without explicit pipe binding:

```dingo
guard user := FindUser(id) else { return err }  // ❌ COMPILE ERROR
```

**Error Message**:
```
guard.dingo:5:1: error: implicit 'err' not allowed: use explicit binding |err| or |e|
    guard user := FindUser(id) else { return err }
                                      ^^^^^^^^^
    hint: Change: else { return err } -> else |err| { return err }
```

**Fix**:
```dingo
guard user := FindUser(id) else |err| { return err }  // ✅ Fixed
```

### Error: Pipe Binding on Option Type

When you try to use pipe binding with Option types:

```dingo
guard first := items.First() else |val| { return nil }  // ❌ COMPILE ERROR
```

**Error Message**:
```
guard.dingo:10:1: error: pipe binding not allowed on Option types (no error to bind)
    guard first := items.First() else |val| { return nil }
                                      ^^^^^
    hint: Option types only have Some/None, not an error value
```

**Fix**:
```dingo
guard first := items.First() else { return nil }  // ✅ Fixed
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

### Go-Style Operators (:= and =)

**Why remove `let` keyword?**

1. **Consistency with Go**: Dingo uses Go's operators everywhere else
2. **Familiarity**: Go developers already know `:=` (declare) vs `=` (assign)
3. **Simplicity**: Less new syntax to learn
4. **Alignment**: Matches Go's philosophy of minimal new keywords

**Benefits**:
- `guard x := expr` - Clear it's a declaration (like Go's `:=`)
- `guard x = expr` - Clear it's an assignment (like Go's `=`)
- No new keyword to remember

### Custom Error Names with Auto-Wrapping

When you use a custom error name like `|e|`, the return transformation applies:

```dingo
guard user := FindUser(id) else |e| { return e }
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
    guard user := FindUser(db, userID) else |err| { return err }
    guard orders := FindOrdersByUser(db, user.ID) else |err| { return err }

    var total float64
    for _, order := range orders {
        total += order.Total
    }
    return total
}
```

## Migration Guide (Breaking Change in v0.12.0)

### Overview

Version 0.12.0 removes the `guard let` syntax in favor of Go-style `:=` and `=` operators. This makes guard syntax consistent with standard Go.

### What Changed

**Before (v0.11.x and earlier)** - `let` keyword:
```dingo
guard let user = FindUser(id) else |err| { return err }  // ❌ No longer works
```

**After (v0.12.0+)** - Go-style operators:
```dingo
guard user := FindUser(id) else |err| { return err }  // ✅ Declaration
guard user = FindUser(id) else |err| { return err }   // ✅ Assignment
```

### Migration Steps

#### Step 1: Identify Affected Code

Search your codebase for all guard let statements:

```bash
grep -n "guard let" **/*.dingo
```

#### Step 2: Replace `let` with `:=`

For most cases, simply replace `let` with `:=`:

**Pattern to find**:
```dingo
guard let x = expr else { ... }
guard let x = expr else |err| { ... }
```

**Pattern to replace with**:
```dingo
guard x := expr else { ... }
guard x := expr else |err| { ... }
```

#### Step 3: Use `=` for Assignment (Rare)

If you need to assign to an existing variable, use `=` instead:

```dingo
// Declare variable first
var user User

// Assign to existing variable
guard user = FindUser(id) else |err| { return err }
```

#### Step 4: Verify Compilation

After migration, verify your code compiles:

```bash
dingo build ./...
```

### Automated Migration

A simple sed command can migrate most cases:

```bash
# Replace 'guard let' with 'guard' and ':='
sed -i 's/guard let \([a-zA-Z_][a-zA-Z0-9_]*\) =/guard \1 :=/g' **/*.dingo

# For more complex cases, use find + sed
find . -name "*.dingo" -exec sed -i 's/guard let \([a-zA-Z_][a-zA-Z0-9_]*\) =/guard \1 :=/g' {} +
```

**Warning**: This handles most common cases. Complex patterns may require manual review.

### Common Migration Patterns

#### Pattern 1: Simple Declaration

```diff
- guard let user = FindUser(id) else |err| { return err }
+ guard user := FindUser(id) else |err| { return err }
```

#### Pattern 2: Multiple Guards

```diff
- guard let from = FindUser(fromID) else |err| { return err }
- guard let to = FindUser(toID) else |err| { return err }
+ guard from := FindUser(fromID) else |err| { return err }
+ guard to := FindUser(toID) else |err| { return err }
```

#### Pattern 3: No Error Binding

```diff
- guard let user = FindUser(id) else { return defaultUser }
+ guard user := FindUser(id) else { return defaultUser }
```

#### Pattern 4: Option Types

```diff
- guard let first = items.First() else { return nil }
+ guard first := items.First() else { return nil }
```

#### Pattern 5: Assignment (Rare)

```diff
+ var user User  // Declare first
- guard let user = FindUser(id) else |err| { return err }
+ guard user = FindUser(id) else |err| { return err }  // Note: = not :=
```

### Validation

After migration, run your Dingo compiler to catch any remaining issues:

```bash
dingo build ./...
```

The compiler will report any places where:
1. `guard let` syntax is still used
2. Invalid operator usage (`:=` vs `=`)

### Why This Change?

1. **Consistency with Go**: Uses familiar `:=` and `=` operators
2. **Less syntax to learn**: No new `let` keyword needed
3. **Clearer intent**: Declaration vs assignment is explicit
4. **Go philosophy**: Minimal new keywords, maximum familiarity

### Need Help?

If you encounter migration issues:
1. Use the automated sed script above for bulk migration
2. Check the compile error messages (they include fix hints)
3. Review the examples in this document

## Implementation

- **Parser**: `pkg/ast/guard_finder.go` - Token-based guard statement parser
- **Codegen**: `pkg/codegen/guard.go` - AST-to-Go code generator
- **Pipeline Position**: Pass 1 (Structural), integrated via `pkg/transpiler/ast_transformer.go`
- **Tests**: `pkg/ast/guard_finder_test.go`, `pkg/codegen/guard_test.go`
- **Golden Tests**: `tests/golden/guard_*.dingo`
- **Breaking Change**: v0.11.0

## Golden Tests

- `tests/golden/guard_let_result.dingo` - Result type scenarios with pipe binding
- `tests/golden/guard_let_option.dingo` - Option type scenarios (no binding)
- `tests/golden/guard_let_inline.dingo` - Inline syntax variations
- `tests/golden/guard_let_tuple.dingo` - Multiple guard lets
- `tests/golden/guard_let_pipe.dingo` - Pipe binding specific tests
- `tests/golden/guard_let_errors.dingo` - Compile error test cases
