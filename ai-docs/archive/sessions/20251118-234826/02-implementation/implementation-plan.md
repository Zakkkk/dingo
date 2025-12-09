# Implementation Plan: Variable Hoisting Pattern for Match-in-Assignment

## Consensus Recommendation

All 4 models (Internal, MiniMax M2, GPT-5.1 Codex, Grok Code Fast) independently recommended the **Variable Hoisting Pattern** as the cleanest solution.

## Target Transformation

### Current (BROKEN)
```go
var result = __match_3 := opt  // ❌ Invalid syntax
```

### Target (Variable Hoisting)
```go
var result Option_int          // ✅ Declare variable with proper type
__match_0 := opt
switch __match_0.tag {
case OptionTagSome:
    x := *__match_0.some_0
    result = Some(x * 2)       // ✅ Assign in each branch
case OptionTagNone:
    result = None
}
```

## Key Implementation Requirements

### 1. Type Inference for Variable Declaration
**Challenge**: Need to determine the type for `var result Type`

**Solution** (from all models):
- Parse the match arms to infer the return type
- Look at the first arm's expression (e.g., `Some(x * 2)`)
- Extract the type (e.g., `Option_int` for Option types)
- Use existing type inference from `pkg/plugin/builtin/type_inference.go`

### 2. Transform Match Arms
**From**: Expression arms (`Some(x * 2)`)
**To**: Assignment statements (`result = Some(x * 2)`)

**Implementation**:
- Detect when match is in assignment context
- Generate variable declaration before the switch
- Modify each case to assign instead of returning

### 3. Preserve Exhaustiveness Checking
**Must maintain**: Compile-time errors for non-exhaustive matches

**Implementation**:
- Keep the `panic("non-exhaustive match")` at the end of switch
- This ensures the variable is always assigned

## File to Modify

**Primary**: `pkg/preprocessor/rust_match.go`

### Specific Changes Needed

#### A. Detect Assignment Context (Already Exists)
```go
// Current detection (around line 200-250)
if strings.Contains(line, "let") && strings.Contains(line, "=") && strings.Contains(line, "match") {
    inAssignmentContext = true
    // Extract variable name
}
```

#### B. Generate Variable Declaration (NEW)
```go
// Add type inference
resultType := inferMatchResultType(arms)  // Use existing type inference

// Generate declaration
output := fmt.Sprintf("var %s %s\n", varName, resultType)
```

#### C. Transform Arms to Assignments (MODIFY)
```go
// Current: generates expression
armCode := fmt.Sprintf("return %s", armExpr)

// New: generate assignment when in assignment context
if inAssignmentContext {
    armCode := fmt.Sprintf("%s = %s", varName, armExpr)
} else {
    armCode := fmt.Sprintf("return %s", armExpr)
}
```

## Testing Strategy

### Primary Test
**File**: `tests/golden/pattern_match_03_assignment.dingo`

**Input**:
```dingo
fn map_option(opt: Option[int]) -> Option[int] {
    let result = match opt {
        Some(x) => Some(x * 2),
        None => None,
    }
    result
}
```

**Expected Output**:
```go
func map_option(opt Option_int) Option_int {
    var result Option_int
    __match_0 := opt
    switch __match_0.tag {
    case OptionTagSome:
        x := *__match_0.some_0
        result = Some(x * 2)
    case OptionTagNone:
        result = None
    default:
        panic("non-exhaustive match")
    }
    return result
}
```

### Regression Tests
Must preserve all 12 currently passing tests:
- `pattern_match_01_simple.dingo` ✅
- `pattern_match_02_result.dingo` ✅
- `pattern_match_04_guards.dingo` ✅
- And 9 others...

## Implementation Steps

1. **Add type inference helper** (~30 min)
   - Function to infer result type from match arms
   - Use existing `type_inference.go` infrastructure

2. **Modify assignment context handling** (~1-2 hours)
   - Generate `var result Type` declaration
   - Transform arms to assignments instead of returns
   - Handle the variable name extraction

3. **Update tests** (~30 min)
   - Run golden tests
   - Update `.go.golden` file for `pattern_match_03_assignment.dingo`
   - Verify no regressions

4. **Edge case handling** (~1 hour)
   - Nested matches in assignment context
   - Multiple assignments in same function
   - Complex type inference scenarios

## Estimated Effort

- **Optimistic**: 1-2 hours (MiniMax M2's estimate)
- **Realistic**: 3-4 hours (accounting for testing and edge cases)
- **Conservative**: 4-6 hours (Grok's estimate, includes thorough testing)

## Success Criteria

✅ **All 13 pattern matching tests pass** (including `pattern_match_03_assignment`)
✅ **Generated Go code compiles** without errors
✅ **Code cleanliness**: More idiomatic than IIFE wrapper
✅ **No regressions**: 12 existing tests still pass

## Key Insights from Models

**MiniMax M2**: "Minimal changes to existing preprocessor, focus on assignment transformation"

**GPT-5.1 Codex**: "Variable hoisting is the most idiomatic Go pattern, no closures needed"

**Grok Code Fast**: "8-9/10 cleanliness score, expect 13/13 tests to pass"

**Internal**: "Multi-statement preprocessor approach aligns with existing architecture"

## Risks & Mitigation

**Risk 1**: Type inference fails for complex types
- **Mitigation**: Start with simple Option/Result types, extend later

**Risk 2**: Breaks exhaustiveness checking
- **Mitigation**: Keep `panic()` at end of switch, test thoroughly

**Risk 3**: Nested matches complicate variable naming
- **Mitigation**: Use existing `__match_N` counter for result variables too

## References

- Model analyses: `ai-docs/sessions/20251118-234826/output/*-analysis.md`
- Current implementation: `pkg/preprocessor/rust_match.go`
- Type inference: `pkg/plugin/builtin/type_inference.go`
- Failing test: `tests/golden/pattern_match_03_assignment.dingo`
