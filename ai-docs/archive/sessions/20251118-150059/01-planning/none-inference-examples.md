# None Constant Type Inference - Detailed Examples

## The Problem

In Dingo, `None` represents the absence of a value in `Option[T]`. But what type is `T`?

```go
let x = None  // What is the type of x? Option[int]? Option[string]? Option[??]?
```

This is ambiguous without context. We need rules to infer the type.

---

## Scenario 1: Clear Context (Easy Cases)

### Case 1.1: Explicit Type Annotation
```go
let x: Option[int] = None  // ✅ Clear: Option[int]
```
**Type:** Explicitly declared as `Option[int]`

### Case 1.2: Return Type Context
```go
fn getAge() -> Option[int] {
    return None  // ✅ Clear from function signature: Option[int]
}
```
**Type:** Inferred from function return type

### Case 1.3: Assignment to Typed Variable
```go
let x: Option[int]
x = None  // ✅ Clear from variable type: Option[int]
```
**Type:** Inferred from variable declaration

### Case 1.4: Function Call Parameter
```go
fn processAge(age: Option[int]) { ... }

processAge(None)  // ✅ Clear from function signature: Option[int]
```
**Type:** Inferred from function parameter type

---

## Scenario 2: Ambiguous Context (The Tricky Cases)

### Case 2.1: No Immediate Context
```go
let x = None  // ❓ What type is x?
println(x)    // Used later, but println accepts any type
```

**Options:**
1. **Error (require explicit type)** ← RECOMMENDED
   ```go
   let x = None  // ERROR: cannot infer type for None
   // Fix: let x: Option[int] = None
   ```

2. **Use later usage context**
   ```go
   let x = None
   processAge(x)  // Infer x as Option[int] from call site
   ```
   **Problem:** Requires complex forward type inference, hard to implement

3. **Default to Option[interface{}]** (risky)
   ```go
   let x = None  // Inferred as Option[interface{}]
   ```
   **Problem:** Loses type safety, needs runtime type assertions

---

### Case 2.2: Multiple Possible Contexts
```go
fn handleAge(age: Option[int]) { ... }
fn handleName(name: Option[string]) { ... }

let x = None
if condition {
    handleAge(x)  // Wants Option[int]
} else {
    handleName(x)  // Wants Option[string]
}
```

**What type is x?**

**Options:**
1. **Error (conflicting contexts)**
   ```go
   let x = None  // ERROR: conflicting type contexts for None
   // Fix: use None directly in calls
   if condition {
       handleAge(None)  // Option[int] from parameter
   } else {
       handleName(None)  // Option[string] from parameter
   }
   ```

2. **Union type** (complex, not planned for Phase 4)
   ```go
   let x: Option[int | string] = None
   ```

---

### Case 2.3: Deferred Usage (Variable Declared, Used Later)
```go
let result: Option[int]

if condition {
    result = Some(42)
} else {
    result = None  // ✅ Type known from result declaration: Option[int]
}
```
**Type:** Inferred from variable declaration (Case 1.3)

**But what if no type annotation?**
```go
let result  // ❓ What type?

if condition {
    result = Some(42)  // Wants Option[int]
} else {
    result = None  // Wants to infer from first assignment?
}
```

**Options:**
1. **Error (require type annotation for uninitialized variables)**
   ```go
   let result  // ERROR: type annotation required for uninitialized variable
   // Fix: let result: Option[int]
   ```

2. **Infer from first assignment**
   ```go
   let result = Some(42)  // Inferred as Option[int]
   // Later:
   result = None  // OK, already known as Option[int]
   ```
   **But:** What if first assignment is `None`? Back to square one.

---

### Case 2.4: Match Expression Context
```go
enum Status {
    Active(userId: int)
    Inactive
}

let age = match status {
    Active(id) => Some(getUserAge(id))  // Returns Option[int]
    Inactive => None  // ❓ What type? Should match Some branch: Option[int]
}
```

**Type:** Inferred from other match arms (expression mode requires type compatibility)

**This works!** Match expression mode requires all arms to return the same type:
- `Some(getUserAge(id))` returns `Option[int]`
- `None` must also be `Option[int]` to match
- Infer `None` as `Option[int]` ✅

---

## Scenario 3: Nested/Complex Cases

### Case 3.1: None in Data Structures
```go
let users = [Some(42), None, Some(99)]  // ❓ Type of None?
```

**Options:**
1. **Infer from array element type**
   ```go
   let users: []Option[int] = [Some(42), None, Some(99)]  // None is Option[int]
   ```

2. **Infer from other elements**
   ```go
   let users = [Some(42), None, Some(99)]
   // First element is Option[int], so None must be Option[int]
   ```

---

### Case 3.2: None in Struct Fields
```go
type User struct {
    name: string
    age: Option[int]
}

let user = User{
    name: "Alice",
    age: None  // ✅ Inferred from struct field type: Option[int]
}
```
**Type:** Inferred from struct field declaration

---

## Recommended Strategy (Conservative Approach)

### Rule 1: Require Context
**None can only be used where type is inferrable from immediate context:**

✅ **Allowed:**
- Explicit type annotation: `let x: Option[int] = None`
- Return statement: `return None` (infer from function signature)
- Function call: `processAge(None)` (infer from parameter)
- Assignment to typed variable: `x = None` (x already typed)
- Struct field: `User{ age: None }` (infer from field type)
- Match arm: `match { _ => None }` (infer from other arms in expression mode)

❌ **Error:**
- Bare initialization: `let x = None` → ERROR: "cannot infer type for None, use explicit type annotation"
- Ambiguous contexts: conflicting types in different branches

---

### Rule 2: Go-Style Type Inference Precedence

When multiple contexts exist, use **closest/most specific context**:

1. **Explicit annotation** (highest priority)
2. **Assignment target type** (variable already typed)
3. **Return type** (function signature)
4. **Function parameter** (call site)
5. **Match arm compatibility** (expression mode)
6. **Struct field type**
7. **Array element type** (if homogeneous)

If no context matches, **error** and require explicit annotation.

---

### Rule 3: Forward Inference (Future Enhancement)

For Phase 4, we **DO NOT** do forward inference:

```go
let x = None  // ERROR now, no forward inference
processAge(x)  // Can't look ahead to infer x's type
```

**Future (Phase 5+):** Implement bidirectional type inference (like TypeScript):
```go
let x = None  // Deferred type inference
processAge(x)  // Infer x as Option[int] from call
```

This requires more sophisticated type checking (go/types integration with constraint solving).

---

## Summary: Three Approaches

| Approach | Pros | Cons | Recommendation |
|----------|------|------|----------------|
| **Error (require explicit type)** | Simple, safe, clear errors | More verbose, user must annotate | ✅ **RECOMMENDED for Phase 4** |
| **Closest context precedence** | Smart, less verbose | Complex inference, potential surprises | Consider for Phase 5 |
| **Option[interface{}]** | Always works | Loses type safety, defeats purpose | ❌ **Never do this** |

---

## Examples with Recommended Approach

### ✅ Valid Code
```go
// Explicit annotation
let x: Option[int] = None

// Return type context
fn getAge() -> Option[int] {
    return None  // OK
}

// Function parameter context
processAge(None)  // OK, inferred from parameter

// Struct field context
let user = User{ age: None }  // OK, inferred from field type

// Match expression context
let result = match status {
    Active(id) => Some(id)
    Inactive => None  // OK, inferred from Some(id) arm
}
```

### ❌ Invalid Code (Errors)
```go
// No context
let x = None  // ERROR: cannot infer type for None
              // Fix: let x: Option[int] = None

// Ambiguous context
let x = None
if cond {
    handleAge(x)     // Wants Option[int]
} else {
    handleName(x)    // Wants Option[string] - conflict!
}
// Fix: use None directly in calls

// Untyped initialization
let x
x = None  // ERROR: x has no type yet
// Fix: let x: Option[int]
```

---

## Implementation Notes

**For Phase 4:**
1. **NoneContextPlugin** uses AST parent tracking to find context
2. Check contexts in precedence order (annotation > return > parameter > field > match arm)
3. If no valid context found, emit error: "cannot infer type for None constant, use explicit type annotation"
4. Error message should suggest fix: `let x: Option[T] = None` or use in typed context

**go/types Integration:**
- Use `types.Info.Types` to get expected type at None position
- Check if expected type is `Option[T]` for some T
- If yes, rewrite None as `Option[T]{}` (zero value)
- If no expected type, error
