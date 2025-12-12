# Internal Analysis: Match-in-Assignment Solutions

## Problem Diagnosis

**Core Issue**: Dingo's `match` is an expression (returns value), Go's `switch` is a statement (returns nothing).

**Current Broken Output**:
```go
var result = __match_3 := opt  // Invalid: can't use := in expression context
```

**User Feedback**: IIFE wrapper works but "looks ugly" - need cleaner alternative.

## Alternative Solutions Analysis

### Alternative 1: Variable Hoisting (Statement Sequence)

**Concept**: Declare result variable first, then use switch to assign it.

**Generated Code**:
```go
func map_option(opt Option_int) Option_int {
    var result Option_int  // Declare with zero value
    __match_0 := opt
    switch __match_0.tag {
    case OptionTagSome:
        x := *__match_0.some_0
        result = Some(x * 2)  // Assignment, not return
    case OptionTagNone:
        result = None
    }
    return result
}
```

**Pros**:
- ✅ Clean, idiomatic Go (variable-then-switch is common pattern)
- ✅ No function wrapper overhead
- ✅ Easy to read and understand
- ✅ Familiar to Go developers (looks like normal Go code)
- ✅ Works with any return type (no interface{} needed)

**Cons**:
- ⚠️ Requires type inference to determine `result` type
- ⚠️ Requires preprocessing to split `x := match` into two statements
- ⚠️ Must handle exhaustiveness (zero value if non-exhaustive?)
- ⚠️ Slightly more complex source mapping (two locations: declaration + assignment)

**Implementation Complexity**: Medium
- Detect assignment context: `name := match`
- Extract variable name (`result`)
- Infer type from match arms (all arms must return same type)
- Generate: `var name Type\n` before match
- Transform arms: `expr` → `name = expr`
- Estimated: 4-5 hours

**Cleanliness Score**: 9/10 (very clean, no wrapper, idiomatic)

---

### Alternative 2: Inline Block with Goto (Zero-Overhead Expression)

**Concept**: Use labeled blocks and goto to simulate expression behavior without function call.

**Generated Code**:
```go
func map_option(opt Option_int) Option_int {
    var result Option_int
    {  // Anonymous block
        __match_0 := opt
        switch __match_0.tag {
        case OptionTagSome:
            x := *__match_0.some_0
            result = Some(x * 2)
            goto __match_0_end
        case OptionTagNone:
            result = None
            goto __match_0_end
        }
        panic("non-exhaustive match")
    }
__match_0_end:
    return result
}
```

**Pros**:
- ✅ Zero runtime overhead (no function call)
- ✅ No interface{} conversion needed
- ✅ Explicit control flow (clear what's happening)
- ✅ Works with exhaustiveness checking (panic at end)

**Cons**:
- ❌ Uses `goto` (controversial in Go community)
- ❌ More verbose than variable hoisting
- ❌ Labeled blocks add visual clutter
- ❌ Not idiomatic Go (rarely see goto in modern Go)

**Implementation Complexity**: Medium-High
- Generate unique labels per match
- Insert goto after each assignment
- Track label names for source mapping
- Estimated: 5-6 hours

**Cleanliness Score**: 5/10 (works but goto is ugly)

---

### Alternative 3: Deferred Assignment Pattern

**Concept**: Use defer to assign result after switch completes.

**Generated Code**:
```go
func map_option(opt Option_int) Option_int {
    var result Option_int
    func() {
        defer func() { /* result already assigned */ }()
        __match_0 := opt
        switch __match_0.tag {
        case OptionTagSome:
            x := *__match_0.some_0
            result = Some(x * 2)
        case OptionTagNone:
            result = None
        }
    }()
    return result
}
```

**Pros**:
- ✅ Closure captures result variable
- ✅ Could handle panics gracefully

**Cons**:
- ❌ Still uses function wrapper (defeats the purpose)
- ❌ Defer adds runtime overhead
- ❌ More complex than IIFE
- ❌ No real benefit over IIFE

**Implementation Complexity**: High
**Cleanliness Score**: 3/10 (worse than IIFE)

**Verdict**: Rejected - no advantage over IIFE.

---

### Alternative 4: Multi-Statement Let Binding (Preprocessor Magic)

**Concept**: Transform `x := match {}` into sequence of statements during preprocessing.

**Dingo Input**:
```dingo
result := match opt {
    Some(x) => Some(x * 2),
    None => None,
}
```

**Preprocessed Output** (still Dingo-like, but multi-statement):
```go
var result Option_int
__match_0 := opt
match __match_0 {
    Some(x) => { result = Some(x * 2) },
    None => { result = None },
}
```

**Then pattern matcher transforms to**:
```go
var result Option_int
__match_0 := opt
switch __match_0.tag {
case OptionTagSome:
    x := *__match_0.some_0
    result = Some(x * 2)
case OptionTagNone:
    result = None
}
```

**Pros**:
- ✅ Clean final output (same as Alternative 1)
- ✅ Separates concerns: preprocessor handles let binding, pattern matcher handles switch
- ✅ Reuses existing pattern matching logic
- ✅ More maintainable (each phase does one thing)

**Cons**:
- ⚠️ Requires coordination between preprocessor phases
- ⚠️ Type inference needed for var declaration
- ⚠️ Must track which match expressions need this treatment

**Implementation Complexity**: Medium
- Add preprocessor pass before RustMatchProcessor
- Detect `name := match` pattern
- Extract variable name and transform to multi-statement
- Estimated: 4-5 hours

**Cleanliness Score**: 9/10 (same output as Alternative 1, cleaner architecture)

---

### Alternative 5: Type-Parametric Helper Function (Generic Wrapper)

**Concept**: Generate a generic helper that makes IIFE less ugly.

**Generated Code**:
```go
func map_option(opt Option_int) Option_int {
    result := matchExpr(func() Option_int {
        __match_0 := opt
        switch __match_0.tag {
        case OptionTagSome:
            x := *__match_0.some_0
            return Some(x * 2)
        case OptionTagNone:
            return None
        }
        panic("non-exhaustive match")
    })
    return result
}

// Generated helper (once per file):
func matchExpr[T any](fn func() T) T {
    return fn()
}
```

**Pros**:
- ✅ Slightly cleaner than raw IIFE
- ✅ Self-documenting (matchExpr makes intent clear)
- ✅ Reuses existing IIFE logic

**Cons**:
- ❌ Still a function wrapper (same overhead)
- ❌ Requires Go 1.18+ (generics)
- ❌ Adds helper function to every file
- ❌ Not actually cleaner in practice

**Implementation Complexity**: Low
**Cleanliness Score**: 6/10 (lipstick on IIFE pig)

**Verdict**: Minor improvement, not worth it.

---

### Alternative 6: What Other Transpilers Do

**TypeScript** (ternary/conditional):
```typescript
// TypeScript expression:
result := condition ? value1 : value2;

// Compiles to JavaScript (still expression):
result := condition ? value1 : value2;
```
**Not applicable** - JavaScript supports expression-based conditionals natively.

---

**Rust-to-C transpilers** (e.g., mrustc):
```rust
// Rust:
result := match opt {
    Some(x) => x * 2,
    None => 0,
};

// C (variable hoisting):
int result;
switch (opt.tag) {
    case Some:
        result = opt.value * 2;
        break;
    case None:
        result = 0;
        break;
}
```
**Lesson**: Variable hoisting is standard approach for expression→statement transpilation.

---

**Scala** (when targeting Java):
```scala
// Scala:
val result = x match {
    case Some(v) => v * 2
    case None => 0
}

// Java (variable hoisting):
int result;
if (x instanceof Some) {
    result = ((Some)x).value * 2;
} else {
    result = 0;
}
```
**Lesson**: Again, variable hoisting. Industry standard.

---

**Zig** (block expressions):
```zig
// Zig has native block expressions:
const result = blk: {
    break :blk if (x) 42 else 0;
};
```
**Not applicable** - Go doesn't support block expressions (no break-with-value).

---

**Kotlin** (when expression to Java):
```kotlin
// Kotlin:
val result = when (x) {
    is Some -> x.value * 2
    is None -> 0
}

// Java bytecode equivalent (conceptually):
int result;
if (x instanceof Some) {
    result = ((Some)x).value * 2;
} else {
    result = 0;
}
```
**Lesson**: Variable hoisting strikes again. Universal pattern.

---

## Comparison Matrix

| Alternative | Cleanliness | Complexity | Overhead | Idiomatic | Score |
|-------------|-------------|------------|----------|-----------|-------|
| **1. Variable Hoisting** | 9/10 | Medium | Zero | Yes | ⭐⭐⭐⭐⭐ |
| **2. Goto Labels** | 5/10 | Medium-High | Zero | No | ⭐⭐ |
| **3. Defer Pattern** | 3/10 | High | High | No | ⭐ |
| **4. Multi-Statement Preprocessor** | 9/10 | Medium | Zero | Yes | ⭐⭐⭐⭐⭐ |
| **5. Generic Helper** | 6/10 | Low | Function call | Meh | ⭐⭐⭐ |
| **IIFE (current working)** | 4/10 | Low | Function call | No | ⭐⭐⭐ |

---

## Deep Dive: Variable Hoisting vs Multi-Statement Preprocessor

These are the two best candidates. Let's compare implementation approaches:

### Variable Hoisting (Direct Approach)

**Changes needed** in `rust_match.go`:

1. **Detect assignment context**:
```go
if isAssignmentContext(line) {
    varName, matchExpr := extractAssignment(line)  // "result", "match opt { ... }"

    // Infer type from match arms
    armTypes := inferArmTypes(matchExpr)
    if !allSameType(armTypes) {
        return "", fmt.Errorf("match arms must return same type")
    }
    resultType := armTypes[0]

    // Generate variable declaration
    output += fmt.Sprintf("var %s %s\n", varName, resultType)

    // Generate match (will transform arms to assignments)
    output += transformMatch(matchExpr, varName)  // Pass varName for assignment
    return output, nil
}
```

2. **Transform arms to assignments**:
```go
func transformArm(pattern, expr, varName string) string {
    if varName != "" {
        // Assignment context
        return fmt.Sprintf("%s = %s", varName, expr)
    } else {
        // Return context
        return fmt.Sprintf("return %s", expr)
    }
}
```

**Pros**:
- Single-pass transformation
- All logic in one place (rust_match.go)
- Direct control over output

**Cons**:
- Type inference is complex (need to parse arm expressions)
- Mixing concerns (match transformation + assignment handling)

---

### Multi-Statement Preprocessor (Pipeline Approach)

**Changes needed**:

1. **New preprocessor phase** (before RustMatchProcessor):

```go
// LetBindingProcessor transforms let-match assignments
type LetBindingProcessor struct{}

func (p *LetBindingProcessor) Process(input string) (string, error) {
    lines := strings.Split(input, "\n")
    output := []string{}

    for i, line := range lines {
        if isLetMatchAssignment(line) {
            // "result := match opt { ... }"
            varName := extractVarName(line)      // "result"
            matchExpr := extractMatchExpr(line)   // "match opt { ... }"

            // Transform to multi-statement:
            // var result T  (type will be inferred later)
            // __match_N := opt
            // result = match opt { ... }  (pseudo-syntax)

            output = append(output, fmt.Sprintf("var %s __MATCH_RESULT__", varName))
            output = append(output, transformMatchToAssignment(matchExpr, varName))
        } else {
            output = append(output, line)
        }
    }

    return strings.Join(output, "\n"), nil
}
```

2. **Modify RustMatchProcessor**:
```go
func (p *RustMatchProcessor) transformArm(arm, assignTarget string) string {
    if assignTarget != "" {
        // Assignment mode (from LetBindingProcessor)
        return fmt.Sprintf("%s = %s", assignTarget, expr)
    } else {
        // Return mode (standalone match)
        return fmt.Sprintf("return %s", expr)
    }
}
```

**Pros**:
- Separation of concerns (each processor does one thing)
- Easier to test in isolation
- More maintainable long-term
- Follows Dingo's preprocessor pipeline philosophy

**Cons**:
- More moving parts
- Need to coordinate between processors
- Type inference still needed (but can be separate concern)

---

## Type Inference Challenge

Both approaches need to determine the type of `var result ???`.

**Options**:

### Option A: Infer from match arms
```go
// Parse each arm's return expression:
Some(x * 2)  → Option_int
None         → Option_int
// All same? Good. Use that type.
```

**Complexity**: High - need to parse Go expressions, resolve types.

### Option B: Placeholder type (defer to go compiler)
```go
var result interface{}  // Generic placeholder
__match_0 := opt
switch __match_0.tag {
case OptionTagSome:
    result = Some(x * 2)  // Assigns Option_int
case OptionTagNone:
    result = None         // Assigns Option_int
}
return result.(Option_int)  // Type assertion at end
```

**Pros**: Simple, no type inference needed
**Cons**: Runtime overhead (type assertion), not zero-cost

### Option C: Use Go's type inference
```go
var result = Option_int{}  // Zero value of inferred type
// OR:
result := Option_int{}     // Short declaration

// Then assign in switch
```

Wait - we need the type name. Back to Option A.

### Option D: Pattern-specific inference

For Dingo's current types (Result, Option), we can infer from the match subject:

```go
// Input:
result := match opt {  // opt is Option[int]
    Some(x) => Some(x * 2),
    None => None,
}

// We know:
// - Matching on Option[int]
// - Arms must return Option[int] (same type as subject)
// - Therefore: var result Option_int
```

**Inference logic**:
```go
func inferMatchResultType(subject string) (string, error) {
    // Get type of subject variable
    subjectType := lookupVariableType(subject)  // "Option_int"

    // For pattern matches on Result/Option, result type = subject type
    // (This is true for Dingo's current pattern matching semantics)

    return subjectType, nil
}
```

**Pros**:
- ✅ Works for 100% of current use cases (matching on Result/Option)
- ✅ Simple, no complex type parsing
- ✅ Can be extended later for other types

**Cons**:
- ⚠️ Assumes match result type = subject type (true now, may not be forever)
- ⚠️ Requires variable type tracking

**This is the pragmatic solution** - solve today's problem with simple inference, extend later if needed.

---

## Implementation Recommendation

### Recommended: Alternative 4 (Multi-Statement Preprocessor) + Pattern-Specific Type Inference

**Why this is the best solution**:

1. **Cleanliness**: 9/10 - generates idiomatic Go (variable hoisting pattern)
2. **Architecture**: Fits Dingo's pipeline philosophy (preprocessor stages)
3. **Type Inference**: Solvable with simple pattern-specific logic
4. **Maintainability**: Separation of concerns, easy to test
5. **Extensibility**: Can add more sophisticated inference later

**Generated Code Example**:
```go
func map_option(opt Option_int) Option_int {
    var result Option_int  // Clean variable declaration
    __match_0 := opt
    switch __match_0.tag {
    case OptionTagSome:
        x := *__match_0.some_0
        result = Some(x * 2)  // Simple assignment
    case OptionTagNone:
        result = None
    }
    return result
}
```

**vs IIFE**:
```go
func map_option(opt Option_int) Option_int {
    result := func() interface{} {  // Function wrapper noise
        __match_0 := opt
        switch __match_0.tag {
        case OptionTagSome:
            x := *__match_0.some_0
            return Some(x * 2)  // Return instead of assign
        case OptionTagNone:
            return None
        }
        panic("non-exhaustive match")
    }()  // Immediate invocation noise
    return result
}
```

**Improvement**: No function wrapper, cleaner assignments, more idiomatic.

---

## Implementation Plan

### Phase 1: New Preprocessor (2-3 hours)

Create `LetMatchProcessor` in `pkg/preprocessor/let_match.go`:

```go
type LetMatchProcessor struct {
    varTypes map[string]string  // Track variable types
}

func (p *LetMatchProcessor) Process(input string) (string, error) {
    // 1. Find "name := match expr" patterns
    // 2. Infer result type from match subject
    // 3. Transform to: "var name Type\n<match-to-assignment>"
    // 4. Mark match for assignment mode
}
```

### Phase 2: Modify RustMatchProcessor (1-2 hours)

```go
func (p *RustMatchProcessor) Process(input string) (string, error) {
    // Check if match is in assignment mode (set by LetMatchProcessor)
    // If assignment mode:
    //   - Transform arms to "varName = expr"
    // If return mode (current behavior):
    //   - Keep IIFE wrapper for backward compatibility
}
```

### Phase 3: Type Inference Helper (1-2 hours)

```go
func inferMatchResultType(subject string, ctx *Context) (string, error) {
    // Look up subject variable type
    // For Result/Option: return type = subject type
    // For others: return error (unsupported for now)
}
```

### Phase 4: Testing (1-2 hours)

- Run existing 12 passing tests (ensure no regression)
- Fix `pattern_match_09_match_in_assignment.dingo` test
- Add additional test cases for edge cases

**Total Estimated Effort**: 5-9 hours (fits within <1 day constraint)

---

## Risk Assessment

### Low Risk:
- ✅ Preprocessor pipeline is well-established pattern in Dingo
- ✅ Variable hoisting is battle-tested in other transpilers
- ✅ Type inference is simple for current use cases

### Medium Risk:
- ⚠️ Need to ensure LetMatchProcessor runs before RustMatchProcessor
- ⚠️ Source mapping needs to account for new variable declaration
- ⚠️ Edge cases: nested matches, multiple matches in one function

### Mitigation:
- Run comprehensive test suite (12 existing + 1 new + edge cases)
- Add integration tests for nested matches
- Document processor ordering clearly

---

## Alternative Fallback: If Type Inference Fails

If pattern-specific type inference proves too complex, **Option B (interface{} placeholder)** is viable fallback:

```go
var result interface{}  // No type inference needed!
__match_0 := opt
switch __match_0.tag {
case OptionTagSome:
    x := *__match_0.some_0
    result = Some(x * 2)
case OptionTagNone:
    result = None
}
return result.(Option_int)  // Type assertion (small overhead)
```

**Trade-off**: Small runtime cost (type assertion) for simpler implementation.

**User preference**: Cleanliness over micro-optimization → This may be acceptable if type inference proves hard.

---

## Conclusion

**Best Solution**: Multi-Statement Preprocessor (Alternative 4)

**Key Benefits**:
1. Clean, idiomatic Go output (9/10 cleanliness)
2. Zero runtime overhead (variable hoisting, no function calls)
3. Fits Dingo's architecture (preprocessor pipeline)
4. Implementable in <1 day (5-9 hours estimated)
5. Extensible for future features

**vs IIFE**: Removes function wrapper, cleaner assignments, more readable.

**Next Steps**: Implement LetMatchProcessor → Modify RustMatchProcessor → Test comprehensive suite.
