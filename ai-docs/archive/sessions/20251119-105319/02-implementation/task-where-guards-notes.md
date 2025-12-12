# Where Guards Implementation - Design Notes

## Design Decisions

### 1. Keyword Support: `if` and `where`

**Decision**: Support both Rust (`if`) and Swift (`where`) syntax

**Rationale**:
- Rust ecosystem uses `if` guards
- Swift ecosystem uses `where` guards
- TypeScript uses neither (no pattern matching)
- Dingo can support both without conflict

**Implementation**: Check for `where` first, then `if` - both generate identical code

**Alternative considered**: Only support `where` (Swift-style) and deprecate `if`
- Rejected: Existing test `pattern_match_05_guards_basic.dingo` uses `if`
- Breaking change for no benefit

### 2. Guard Transformation Strategy

**Decision**: Wrap case body in `if` statement, no `else` clause

**Generated code**:
```go
case ResultTag_Ok:
    x := __match_0.ok_0
    if x > 0 {
        "positive"
    }
    // Falls through if guard fails
```

**Rationale**:
- Go switch cases DON'T auto-fallthrough (need explicit `fallthrough`)
- If guard fails, case body doesn't execute → goes to next case
- This matches Rust/Swift guard semantics

**Alternative considered**: Add `else { /* fallthrough logic */ }`
- Rejected: Go doesn't need explicit fallthrough keyword in this pattern
- Simpler code without `else`

### 3. Multiple Guards on Same Pattern

**Behavior**: Each guard creates a separate case clause

**Dingo code**:
```dingo
match result {
    Result_Ok(x) where x > 100 => "large",
    Result_Ok(x) where x > 10 => "medium",
    Result_Ok(x) => "small",
}
```

**Generated Go**:
```go
switch result.tag {
case ResultTag_Ok:
    x := result.ok_0
    if x > 100 {
        "large"
    }
case ResultTag_Ok:
    x := result.ok_0
    if x > 10 {
        "medium"
    }
case ResultTag_Ok:
    x := result.ok_0
    "small"
}
```

**Rationale**:
- Go allows duplicate case labels in different positions
- First matching guard wins (short-circuit evaluation)
- Falls through to next case if guard fails
- Matches Rust guard semantics

**Alternative considered**: Combine all guards into nested if-else chain
- Rejected: More complex, harder to debug, loses DINGO_GUARD markers

### 4. Guard Validation

**Decision**: NO compile-time validation of guard expressions

**Current behavior**: Guard condition passed through to Go as-is

**Examples**:
```dingo
// Valid guard
Result_Ok(x) where x > 0 => ...

// Invalid guard (y not in scope) - NO ERROR at Dingo compile time
Result_Ok(x) where y > 0 => ...  // Go compiler will catch this
```

**Rationale**:
- Dingo doesn't have full type checking yet
- Go compiler will validate scope and types
- Simpler implementation (no need for scope analysis)

**Future enhancement**: Add scope validation in plugin phase with go/types

### 5. Exhaustiveness Checking with Guards

**Decision**: Guards don't affect exhaustiveness

**Example**:
```dingo
// This is considered NON-exhaustive even though guards cover all cases
match option {
    Option_Some(x) where x > 0 => "positive",
    Option_Some(x) where x <= 0 => "non-positive",
    // ERROR: Missing Option_None case
}
```

**Rationale**:
- Validating guard coverage requires SMT solver (complex)
- Rust also considers guarded matches non-exhaustive
- Requires wildcard or catchall case

**Workaround**: Add catchall:
```dingo
match option {
    Option_Some(x) where x > 0 => "positive",
    Option_Some(x) => "non-positive or zero",
    Option_None => "none",
}
```

## Edge Cases Discovered

### 1. Nested Patterns with Guards

**Status**: NOT SUPPORTED

**Example**:
```dingo
Result_Ok(Option_Some(val)) where val > 0 => "positive nested"
```

**Error**: `parse error: missing ',' in argument list`

**Root cause**: Preprocessor pattern parser doesn't handle nested constructors
- Expects: `Pattern(binding)`
- Gets: `Pattern1(Pattern2(binding))`
- Fails to parse nested parentheses

**This is a SEPARATE feature** from guards - requires recursive pattern parsing

**Files affected**:
- `pattern_match_06_guards_nested.dingo` ❌
- Potentially files 07-08 (not tested)

### 2. Block Expressions with Guards

**Status**: ✅ WORKING

**Example**:
```dingo
Result_Ok(x) where x > 0 => {
    log("positive");
    return "success";
}
```

**Generated**:
```go
case ResultTag_Ok:
    x := result.ok_0
    if x > 0 {
        log("positive")
        return "success"
    }
```

**Handling**: Block braces removed, statements indented +1 level inside guard

### 3. Function Calls in Guards

**Status**: ✅ WORKING

**Example**:
```dingo
Result_Ok(n) where isEven(n) => "even"
```

**Generated**:
```go
case ResultTag_Ok:
    n := result.ok_0
    if isEven(n) {
        "even"
    }
```

**Note**: No validation that `isEven` exists - Go compiler checks this

### 4. Complex Boolean Expressions

**Status**: ✅ WORKING

**Example**:
```dingo
Result_Ok(age) where age >= 18 && age < 65 => "adult"
```

**Generated**:
```go
case ResultTag_Ok:
    age := result.ok_0
    if age >= 18 && age < 65 {
        "adult"
    }
```

**Parsing**: Guard condition extracted as-is, no AST parsing needed

## Performance Considerations

### Generated Code Efficiency

**With guards** (multiple cases):
```go
switch result.tag {
case ResultTag_Ok:
    x := result.ok_0
    if x > 100 { return "large" }
case ResultTag_Ok:
    x := result.ok_0
    if x > 10 { return "medium" }
case ResultTag_Ok:
    x := result.ok_0
    return "small"
}
```

**Concerns**:
- Multiple binding extractions (`x := result.ok_0` repeated)
- Multiple case clauses instead of nested if-else

**Mitigation**:
- Go compiler likely optimizes this (switch case merging)
- Clear, debuggable code more valuable than micro-optimization
- Real-world guards rarely have >3 guards per variant

**Alternative considered**: Merge into single case with nested if-else
- Rejected: Loses DINGO_GUARD markers, harder to debug

## Integration with Existing Features

### Variable Hoisting Pattern

Guards work correctly with Variable Hoisting (assignment context):

```dingo
status := match result {
    Result_Ok(x) where x > 0 => "positive",
    Result_Ok(x) => "non-positive",
    Result_Err(_) => "error",
}
```

**Generated**:
```go
var status string
// DINGO_MATCH_START
__match_0 := result
switch __match_0.tag {
case ResultTag_Ok:
    x := *__match_0.ok_0
    if x > 0 {
        status = "positive"
    }
case ResultTag_Ok:
    x := *__match_0.ok_0
    status = "non-positive"
// ...
}
```

### DINGO_GUARD Markers

**Preserved**: Guard markers remain in generated code for debugging

**Format**:
```go
// DINGO_PATTERN: Ok(x) | DINGO_GUARD: x > 0
```

**Used by**: Pattern match plugin for exhaustiveness checking (future)

## Testing Recommendations

### Test Coverage Needed

1. ✅ **Basic guards** (`if` keyword) - TESTED
2. ✅ **Swift guards** (`where` keyword) - TESTED
3. ✅ **Multiple guards per variant** - TESTED
4. ✅ **Complex boolean guards** - TESTED
5. ✅ **Function call guards** - TESTED
6. ❌ **Nested patterns with guards** - NOT SUPPORTED YET
7. ⚠️  **Tuple patterns with guards** - UNKNOWN
8. ⚠️  **Guards in expression vs statement context** - PARTIALLY TESTED

### Regression Tests

**After this change, verify**:
1. Non-guard patterns still work
2. Exhaustiveness checking still works
3. DINGO_GUARD markers present in output
4. Generated code compiles
5. Runtime behavior matches expected fallthrough

## Future Enhancements

### 1. Guard Scope Validation

**Feature**: Validate guard only references bindings in scope

**Implementation**: In plugin phase, use go/types to check variable scope

**Example error**:
```dingo
Result_Ok(x) where y > 0 => ...
//                 ^ error: y not in scope (binding is x)
```

### 2. Nested Pattern Support

**Feature**: Allow nested constructors with guards

**Example**:
```dingo
Result_Ok(Option_Some(val)) where val > 0 => "positive nested"
```

**Requirements**:
- Recursive pattern parser in preprocessor
- Multi-level binding extraction
- Guard scope analysis (val from nested pattern)

**Estimated complexity**: 4-6 hours

### 3. Smarter Guard Merging

**Feature**: Merge multiple guards into single case with if-else chain

**Generated**:
```go
case ResultTag_Ok:
    x := result.ok_0
    if x > 100 {
        result = "large"
    } else if x > 10 {
        result = "medium"
    } else {
        result = "small"
    }
```

**Benefits**: Single binding extraction, clearer structure

**Tradeoffs**: Loses separate DINGO_GUARD markers

**Estimated complexity**: 2-3 hours
