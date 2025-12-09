# Consolidated Analysis: Pattern Match Code Generation Bug

## External Model Consultation Status

**Models Attempted**:
- x-ai/grok-code-fast-1 (TIMEOUT after 4+ minutes)
- minimax/minimax-m2 (FAILED - returned empty response)

**Fallback**: Direct analysis by golang-architect agent

## Root Cause Analysis

### The Bug

**Location**: `pkg/preprocessor/rust_match.go`, lines 400-416 (generateSwitch function)

**Issue**: The assignment context logic has a fundamental flaw in Example 4:

```dingo
let result = match opt {
    Some(x) => Some(x * 2),
    None => None
}
```

**Current (BROKEN) Output**:
```go
var result = __match_3 := opt  // INVALID SYNTAX
// DINGO_MATCH_START: opt
switch __match_3.tag {
    ...
}
```

**Root Problem**:
1. When `isInAssignment = true`, the code skips emitting the temp variable declaration (line 402-416)
2. But it still tries to USE the temp variable `__match_3` in the switch statement (line 419)
3. This creates `var result = __match_3 := opt` which is syntactically invalid

### Why This Happened

The fix attempt tried to solve: `var result = __match_3 := opt`

By skipping the temp var declaration when in assignment context. But it forgot that:
1. The match expression ITSELF needs to become the RHS of the assignment
2. The switch statement can't be the RHS (switch is a statement, not an expression in Go)
3. The entire approach is wrong for Go's statement-based syntax

## The Real Problem: Match as Expression

**Critical Insight**: Dingo treats `match` as an EXPRESSION (returns a value), but Go's `switch` is a STATEMENT (doesn't return a value).

**In Rust/Dingo**:
```rust
let result = match opt {  // match returns a value
    Some(x) => x * 2,
    None => 0
}
```

**In Go**: You can't do this:
```go
result := switch opt {  // INVALID - switch doesn't return a value
    ...
}
```

### What Works vs What Doesn't

**Examples 1-3 (Work)**: Match NOT in assignment context
```go
func processResult(result Result_int_error) int {
    __match_0 := result           // ✅ Temp var declaration
    // DINGO_MATCH_START: result
    switch __match_0.tag {        // ✅ Switch on temp var
        case ResultTagOk:
            value := *__match_0.ok_0
            value * 2             // ⚠️ Expression statement (not returned)
        ...
    }
}
```

**Example 4 (BROKEN)**: Match IN assignment context
```go
func doubleIfPresent(opt Option_int) Option_int {
    var result = __match_3 := opt  // ❌ SYNTAX ERROR
    // DINGO_MATCH_START: opt
    switch __match_3.tag {
        ...
    }
    return result  // ❌ 'result' never gets assigned
}
```

## Recommended Fix Strategy

### Option A: IIFE Pattern (Like Result/Option constructors)

Transform match-in-assignment to an immediately-invoked function expression:

**Dingo Input**:
```dingo
let result = match opt {
    Some(x) => Some(x * 2),
    None => None
}
```

**Go Output**:
```go
result := func() Option_int {
    // DINGO_MATCH_START: opt
    __match_3 := opt
    switch __match_3.tag {
    case OptionTagSome:
        x := *__match_3.some_0
        return Some(x * 2)  // Add 'return' before expression
    case OptionTagNone:
        return None         // Add 'return' before expression
    }
    panic("non-exhaustive match")
}()
```

**Pros**:
- ✅ Reuses existing IIFE pattern from Result/Option
- ✅ Match truly returns a value (via function return)
- ✅ Clean, idiomatic Go
- ✅ Handles all edge cases (nested matches, complex expressions)

**Cons**:
- ⚠️ Slightly more complex codegen
- ⚠️ Minimal performance overhead (function call - but inlined by compiler)

### Option B: Multiple Assignment Statements

Convert each arm to a separate assignment:

**Go Output**:
```go
var result Option_int
__match_3 := opt
// DINGO_MATCH_START: opt
switch __match_3.tag {
case OptionTagSome:
    x := *__match_3.some_0
    result = Some(x * 2)  // Assign to pre-declared var
case OptionTagNone:
    result = None         // Assign to pre-declared var
}
```

**Pros**:
- ✅ Simpler to implement
- ✅ No function call overhead

**Cons**:
- ❌ Requires declaring result variable before match
- ❌ Type inference harder (need to determine result type)
- ❌ Doesn't handle return/break in match arms well

### Option C: Revert Assignment Context Detection

Remove the assignment context logic entirely, go back to simpler approach:

**Go Output**:
```go
var result Option_int
__match_3 := opt
switch __match_3.tag {
    ...
    result = Some(x * 2)
    ...
}
```

But this requires knowing the result type beforehand, which is hard in preprocessing stage.

## Recommended Implementation: Option A (IIFE)

### Changes Needed

**File**: `pkg/preprocessor/rust_match.go`

**Function**: `generateSwitch` (lines 376-450)

**Modification**:

```go
func (r *RustMatchProcessor) generateSwitch(scrutinee string, arms []patternArm, originalLine int, outputLine int, isInAssignment bool) (string, []Mapping) {
    var buf bytes.Buffer
    mappings := []Mapping{}

    matchID := r.matchCounter
    r.matchCounter++

    scrutineeVar := fmt.Sprintf("__match_%d", matchID)

    if isInAssignment {
        // IIFE pattern for assignment context
        buf.WriteString("func() ")
        // TODO: Infer return type from arms (requires type analysis)
        // For now, use interface{} or detect from first arm
        buf.WriteString("interface{} {\n")
        outputLine++

        // DINGO_MATCH_START marker
        buf.WriteString(fmt.Sprintf("    // DINGO_MATCH_START: %s\n", scrutinee))
        outputLine++

        // Temp variable inside IIFE
        buf.WriteString(fmt.Sprintf("    %s := %s\n", scrutineeVar, scrutinee))
        outputLine++

        // Switch statement
        buf.WriteString(fmt.Sprintf("    switch %s.tag {\n", scrutineeVar))
        outputLine++

        // Generate cases (with 'return' before expressions)
        for _, arm := range arms {
            caseLines, caseMappings := r.generateCaseWithReturn(scrutineeVar, arm, originalLine, outputLine, 2) // indent=2
            buf.WriteString(caseLines)
            mappings = append(mappings, caseMappings...)
            outputLine += strings.Count(caseLines, "\n")
        }

        buf.WriteString("    }\n")
        buf.WriteString("    panic(\"non-exhaustive match\")\n")
        buf.WriteString("}()")
        outputLine += 2

    } else {
        // Original non-assignment logic (Examples 1-3)
        buf.WriteString(fmt.Sprintf("// DINGO_MATCH_START: %s\n", scrutinee))
        outputLine++

        buf.WriteString(fmt.Sprintf("%s := %s\n", scrutineeVar, scrutinee))
        outputLine++

        buf.WriteString(fmt.Sprintf("switch %s.tag {\n", scrutineeVar))
        outputLine++

        for _, arm := range arms {
            caseLines, caseMappings := r.generateCase(scrutineeVar, arm, originalLine, outputLine)
            buf.WriteString(caseLines)
            mappings = append(mappings, caseMappings...)
            outputLine += strings.Count(caseLines, "\n")
        }

        buf.WriteString("}\n")
        outputLine++
    }

    return buf.String(), mappings
}
```

**New Helper**:
```go
// generateCaseWithReturn generates a case with 'return' before expression
func (r *RustMatchProcessor) generateCaseWithReturn(scrutineeVar string, arm patternArm, originalLine int, outputLine int, indent int) (string, []Mapping) {
    // Similar to generateCase but adds 'return' before arm.expression
    // ...
}
```

### Type Inference Challenge

**Problem**: IIFE needs explicit return type, but preprocessor runs before type checking.

**Solutions**:
1. **Use `interface{}`** - Simple but loses type safety (caller must type-assert)
2. **Detect from first arm** - Parse expression, infer type (fragile)
3. **Defer to AST phase** - Add marker, replace in plugin phase (complex)
4. **Use generics placeholder** - Replace in second pass (requires two-stage)

**Recommended**: Start with `interface{}`, add proper type inference in Phase 5 when adding full go/types integration.

## Testing Strategy

### Step 1: Minimal Reproduction

Create `pattern_match_14_assignment_iife.dingo`:
```dingo
func test(opt: Option[int]) -> Option[int] {
    let result = match opt {
        Some(x) => Some(x * 2),
        None => None
    }
    return result
}
```

Expected output:
```go
func test(opt Option_int) Option_int {
    result := func() interface{} {
        // DINGO_MATCH_START: opt
        __match_0 := opt
        switch __match_0.tag {
        case OptionTagSome:
            x := *__match_0.some_0
            return Some(x * 2)
        case OptionTagNone:
            return None
        }
        panic("non-exhaustive match")
    }()
    return result
}
```

### Step 2: Fix Example 4

Modify `pattern_match_01_simple.dingo` Example 4 to use IIFE pattern.

### Step 3: Regression Testing

Run all 13 pattern match tests:
```bash
go test ./tests -run TestGoldenFiles/pattern_match -v
```

Ensure 12/13 working tests still pass.

### Step 4: Compilation Test

Attempt to compile generated Go code:
```bash
dingo build tests/golden/pattern_match_01_simple.dingo
go build tests/golden/pattern_match_01_simple.go
```

Should compile without errors.

## Risk Assessment

### High Risk

**Breaking 12 working tests**: LOW
- Changes only affect `isInAssignment = true` path
- Examples 1-3 use `isInAssignment = false` path (unchanged)

### Medium Risk

**Type safety loss**: MEDIUM (with interface{} approach)
- Using `interface{}` loses compile-time type checking
- Mitigated by: Adding proper type inference in Phase 5

**Performance overhead**: LOW
- IIFE adds function call, but Go compiler inlines simple closures
- Benchmark: Expected <5% overhead vs direct assignment

### Low Risk

**Code size increase**: LOW
- IIFE adds ~5 lines vs current approach
- Acceptable for correctness

## Alternative Considered: Assignment Detection Removal

Could remove `isInAssignmentContext()` entirely and always use IIFE for match expressions:

**Pros**:
- Simpler logic (one code path)
- Consistent output

**Cons**:
- Examples 1-3 would have unnecessary IIFE wrapper
- Slightly less readable generated code

**Verdict**: Keep assignment detection, use IIFE only when needed.

## Implementation Plan

### Phase 1: IIFE Pattern (2-3 hours)
1. Implement `generateSwitchIIFE()` function
2. Implement `generateCaseWithReturn()` helper
3. Modify `generateSwitch()` to call IIFE path when `isInAssignment = true`
4. Use `interface{}` as return type (temporary)

### Phase 2: Testing (1 hour)
1. Create minimal reproduction test
2. Update Example 4 in `pattern_match_01_simple`
3. Run full test suite
4. Fix any regressions

### Phase 3: Type Inference (Future - Phase 5)
1. Add go/types integration
2. Infer actual return type from arms
3. Replace `interface{}` with proper type
4. Add type mismatch validation

## Success Criteria

✅ Example 4 compiles successfully
✅ Generated code is valid Go syntax
✅ 13/13 pattern match tests pass
✅ No regression in Examples 1-3, 5
✅ Performance overhead <10%

## Confidence Level

**Root Cause Identification**: HIGH (100%)
- Clear syntax error in generated code
- Assignment context logic is provably wrong

**Recommended Fix**: HIGH (90%)
- IIFE pattern proven in Result/Option implementation
- Addresses fundamental expression vs statement issue

**Implementation Risk**: MEDIUM (60%)
- Type inference challenge (mitigated by interface{} fallback)
- Need to handle edge cases (nested matches, complex arms)

**Testing Coverage**: HIGH (85%)
- Have good test suite (13 tests)
- Can validate against working examples

## External Model Failure Analysis

**Why models failed**:
1. **Grok timeout**: API pre-flight checks taking 4+ minutes (likely API availability issue)
2. **Minimax empty response**: Returned only proxy startup/shutdown messages (API error or rate limit)

**Recommendation**:
- Retry with different models (gpt-5.1-codex, gemini-pro-code)
- Check claudish configuration and API keys
- For urgent bugs, fallback to direct analysis (as done here)

## Next Actions

1. **Implement Fix**: Use IIFE pattern for assignment context (Option A)
2. **Test**: Run full test suite, ensure 13/13 pass
3. **Document**: Update reasoning file with IIFE approach
4. **Future**: Add proper type inference in Phase 5 with go/types
