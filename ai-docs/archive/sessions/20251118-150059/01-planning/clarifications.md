# User Clarifications - Phase 4

## Pattern Matching Syntax Decisions

### Q1: Syntax Style
**Decision:** Support BOTH Rust-like and Swift-like syntax, configurable via `dingo.toml`

**Configuration:**
```toml
[match]
syntax = "rust"  # or "swift"
```

**Rust-like (default):**
```go
match result {
    Ok(x) => x * 2
    Err(e) => 0
}
```

**Swift-like:**
```go
switch result {
    case .ok(let x): return x * 2
    case .err(let e): return 0
}
```

**Implementation implications:**
- Need two preprocessor patterns (RustMatchProcessor and SwiftMatchProcessor)
- Both transpile to same Go switch statement
- dingo.toml parser and config system
- Default: Rust-like (for consistency with Result/Option)

---

### Q2: Exhaustiveness Checking
**Decision:** Compile ERROR (strict) - non-exhaustive matches are rejected

**Behavior:**
```go
// ERROR: non-exhaustive match
match result {
    Ok(x) => x
}

// Fix: add explicit wildcard or all cases
match result {
    Ok(x) => x
    _ => 0  // Required
}
```

**Implementation:** PatternMatchPlugin validates exhaustiveness during Transform phase

---

### Q3: Match Type (Expression vs Statement)
**Decision:** Infer from context (smart detection)

**Rules:**
1. If match result is assigned → Expression mode (type-check all arms)
2. If match result is returned → Expression mode
3. If match is used in expression position → Expression mode
4. Otherwise → Statement mode (no return, no type checking)

**Examples:**
```go
// Expression mode (assigned)
let x = match result {
    Ok(v) => v * 2  // Must return int
    Err(_) => 0     // Must return int
}

// Expression mode (returned)
fn getValue() -> int {
    return match result {
        Ok(v) => v
        Err(_) => 0
    }
}

// Statement mode (standalone)
match result {
    Ok(v) => println(v)   // No return type required
    Err(e) => println(e)
}
```

**Implementation:**
- AST parent tracking to detect usage context
- go/types integration for type checking in expression mode
- Different transpilation strategies for expression vs statement

---

## Additional Decisions

### Q4: None Constant Context Inference
**Decision:** Error and require explicit type (conservative, safe)

**Behavior:**
```go
let x = None  // ERROR: cannot infer type for None
// Fix: let x: Option[int] = None

// Valid contexts (auto-inferred):
return None  // OK - from function signature
processAge(None)  // OK - from parameter type
let user = User{ age: None }  // OK - from field type
match { _ => None }  // OK - from other match arms
```

**See:** `ai-docs/sessions/20251118-150059/01-planning/none-inference-examples.md` for comprehensive examples

---

### Q5: Tuple Destructuring
**Decision:** Yes - include in Phase 4.2 (Advanced Patterns)

**Timeline:** 4 weeks total (MVP in 2 weeks, tuples in week 3-4)

**Example:**
```go
match (x, y) {
    (0, 0) => "origin"
    (0, _) => "on y-axis"
    (_, 0) => "on x-axis"
    _ => "other"
}
```

**Requires:** Tuple syntax support (multiple return destructuring)

---

### Q6: AST Parent Map Construction
**Decision:** Build unconditionally (simpler, predictable cost)

**Performance:** +5-10ms per file (acceptable overhead)

**Rationale:** Simplifies plugin API, required for context-aware inference

---

### Q7: Error Messages with Source Context
**Decision:** Yes - show source snippets (rustc-style)

**Example:**
```
error: non-exhaustive match
  --> example.dingo:23:5
   |
23 | match result {
24 |     Ok(x) => processX(x)
   |     ^^^^^^^^^^^^^^^^^^^ missing Err case
   |
help: add a wildcard arm: `_ => ...`
```

**Requires:** Source file reading during error formatting, line/column tracking
