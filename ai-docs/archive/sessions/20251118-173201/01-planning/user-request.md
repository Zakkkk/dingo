# Phase 4.2 Development Request

## Context
Continuing from successful Phase 4.1 completion (session: 20251118-150059).

Phase 4.1 delivered:
- ✅ Configuration system (dingo.toml)
- ✅ AST parent tracking
- ✅ Rust pattern matching syntax
- ✅ Strict exhaustiveness checking
- ✅ Pattern transformation to idiomatic Go
- ✅ None context inference (5 contexts)
- ✅ 57/57 tests passing (100%)

## Phase 4.2 Objectives

Implement the following enhancements to pattern matching:

### 1. Pattern Guards (if conditions)
```dingo
match value {
    Ok(x) if x > 0 => handlePositive(x),
    Ok(x) => handleNonPositive(x),
    Err(e) => handleError(e)
}
```

**Requirements:**
- Support `pattern if condition` syntax
- Guards evaluated at runtime after pattern matches
- Maintain exhaustiveness checking (guards don't affect it)
- Clean Go code generation

### 2. Swift Pattern Syntax Support
```dingo
// Swift-style
switch value {
case .Ok(let x):
    handleOk(x)
case .Err(let e):
    handleError(e)
}
```

**Requirements:**
- Support Swift's `switch/case` keywords
- Support `.Variant(let x)` binding syntax
- Configurable via dingo.toml (rust_match vs swift_match)
- Same exhaustiveness checking as Rust syntax

### 3. Tuple Destructuring in Patterns
```dingo
match getTuple() {
    (Ok(x), Ok(y)) => handleBoth(x, y),
    (Ok(x), Err(e)) => handlePartial(x, e),
    (Err(e), _) => handleFirstError(e)
}
```

**Requirements:**
- Support tuple patterns: `(pattern1, pattern2, ...)`
- Support wildcard `_` in tuple positions
- Nested pattern support
- Type checking for tuple arity

### 4. Enhanced Error Messages
**Requirements:**
- Source code snippets in error messages
- Highlight exact pattern location
- Suggest missing patterns for exhaustiveness
- Show example of correct usage

**Example output:**
```
Error: Non-exhaustive match in file.dingo:42:5

  40 |     result := fetchData()
  41 |     match result {
  42 |         Ok(x) => process(x)
     |         ^^^^^^^^^^^^^^^^^^^ Missing pattern: Err(_)
  43 |     }

Suggestion: Add pattern to handle all cases:
    match result {
        Ok(x) => process(x),
        Err(e) => handleError(e)  // Add this
    }
```

## Success Criteria

1. **Functionality:**
   - All 4 features implemented and working
   - Backward compatible with Phase 4.1 tests
   - New golden tests for each feature (4+ tests)

2. **Quality:**
   - Clean, idiomatic Go output
   - Performance: <1ms overhead per match
   - Code review approved by multiple reviewers

3. **Testing:**
   - 100% test pass rate (Phase 4.1 + Phase 4.2 tests)
   - Edge cases covered (nested guards, complex tuples, etc.)

4. **Documentation:**
   - Updated reasoning docs for new features
   - Configuration guide for syntax selection
   - Migration guide (if breaking changes)

## Timeline
Estimated: 4-6 hours (following Phase 4.1 workflow efficiency)

## Priority
Medium-High - These features complete the pattern matching MVP and bring Dingo to feature parity with Rust/Swift pattern matching capabilities.
