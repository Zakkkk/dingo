# Pattern Matching Preprocessor Bug Investigation

## Context: Where We Are

**Good News**: Type injection works perfectly (11/11 Result/Option tests passing ✅)
**Current Problem**: 12 pattern matching tests failing with preprocessor errors
**Goal**: Fix pattern matching parser to achieve 98%+ test passing rate

## The Problem: "No Pattern Arms Found" Errors

### Failure Evidence

**Test**: `pattern_match_01_simple.dingo`
**Error**: Parser error: "no pattern arms found"

**Test**: `pattern_match_02_guards.dingo`  
**Error**: Similar parsing failures

**Test**: All pattern_match_* tests (01-12)**
**Error**: Preprocessor fails to parse match expressions

### Example Failing Code

**File**: `tests/golden/pattern_match_01_simple.dingo`
```dingo
fn process_result(r: Result[int, string]) -> int {
    match r {
        Ok(x) => x,
        Err(e) => 0
    }
}

fn process_option(o: Option[int]) -> int {
    match o {
        Some(x) => x,
        None => 0
    }
}
```

**Expected Behavior**: Should transpile to Go switch statements
**Actual Behavior**: Parser error: "no pattern arms found"

### What Works vs What Fails

**✅ Working (from previous implementation)**:
- Tuple pattern matching (tests 09-11) - transpiles successfully
- Where guards (tests 06-08) - feature implemented
- Type injection (Result/Option) - 100% working

**❌ Failing**:
- Basic pattern matching (test 01) - "no pattern arms found"
- Guard patterns (test 02) - parse error
- Nested patterns (test 03) - parse error
- All remaining pattern match tests (04-12) - similar errors

## Architecture Context

### Pattern Matching Pipeline

```
.dingo file with match expression
    ↓
Stage 1: Preprocessor - rust_match.go
    ↓ parseMatchExpression()
    ↓ parseArms()
    ↓ findMatchBoundary()
    ↓
Generates: Go switch statement
    ↓
Stage 2: AST Processing
    ↓
.go file
```

### Key Files

**Preprocessor**:
- `pkg/generator/preprocessor/rust_match.go` (~500+ lines after recent changes)
- Function: `preprocessRustMatch()` - Entry point
- Function: `parseMatchExpression()` - Parse match keyword
- Function: `parseArms()` - Parse pattern arms ← **LIKELY BUG HERE**
- Function: `findMatchBoundary()` - Find closing brace

**Recent Changes**:
- ✅ Where guards implemented (working)
- ✅ Tuple pattern parsing fixed (working)
- ❌ Something broke basic pattern arm parsing

## Investigation Focus

### Primary Question
**Why does the parser report "no pattern arms found" for valid match expressions?**

### Specific Areas to Investigate

1. **Arm Parser Logic**
   - File: `pkg/generator/preprocessor/rust_match.go`
   - Function: `parseArms()` or similar
   - Question: Why doesn't it detect `Ok(x) => x` as valid arm?

2. **Pattern Recognition**
   - How does parser identify pattern arms?
   - Regex patterns used?
   - Boundary detection logic?

3. **Recent Regression**
   - Where guards were added - did they break basic parsing?
   - Tuple pattern fixes - did they interfere with simple patterns?
   - Check if recent changes added restrictive conditions

4. **Edge Cases**
   - Whitespace handling
   - Newline detection
   - Bracket balancing

### Test Case Analysis

**Simplest failing case**: `pattern_match_01_simple.dingo`

Example from file:
```dingo
match r {
    Ok(x) => x,
    Err(e) => 0
}
```

**What parser should find**:
- Arm 1: Pattern `Ok(x)`, Expression `x`
- Arm 2: Pattern `Err(e)`, Expression `0`

**What parser reports**: "no pattern arms found"

**Why?** This is what we need to discover.

### Debug Strategy

1. **Add logging to parseArms()**
   - Log input string
   - Log regex matches
   - Log detected boundaries
   - See where it fails

2. **Test minimal input**
   ```go
   input := `match x {
       Ok(y) => y
   }`
   // Does this parse?
   ```

3. **Compare with working code**
   - Tuple patterns work: `(Ok(x), Ok(y)) => x + y`
   - Basic patterns fail: `Ok(x) => x`
   - What's the difference in parsing logic?

## Code Investigation Areas

### Area 1: Pattern Arm Regex

**Likely location**: `pkg/generator/preprocessor/rust_match.go`

**Look for**:
```go
// Pattern arm regex (example)
armPattern := regexp.MustCompile(`(\w+)\(([^)]*)\)\s*=>\s*([^,}]+)`)
```

**Questions**:
- Is regex too restrictive?
- Does it handle simple patterns like `Ok(x)`?
- Does it require specific syntax that test files don't have?

### Area 2: Arm Boundary Detection

**Look for**:
```go
func parseArms(input string) ([]PatternArm, error) {
    // How does it split arms?
    // Comma-separated?
    // Newline-separated?
    // Something else?
}
```

**Questions**:
- How are arms delimited?
- Does it expect commas? (our examples have commas)
- Does it expect specific formatting?

### Area 3: Where Guard Interaction

**Recent change**: Where guards added

**Hypothesis**: Guard parsing might have broken non-guard patterns

**Check**:
```go
// Does this code check for guards BEFORE checking basic patterns?
if hasWhereClause {
    // Parse with guard
} else {
    // Parse basic pattern ← Is this branch unreachable?
}
```

### Area 4: Tuple vs Simple Pattern Logic

**Recent change**: Tuple pattern parser fixed

**Hypothesis**: Tuple logic might have overridden simple pattern logic

**Check**:
```go
// Does tuple parser run first?
if isTuplePattern {
    // Parse tuple
} else {
    // Parse simple ← Does this ever execute?
}
```

## Output Format

Please structure your analysis as:

### Executive Summary
[1-2 paragraphs: root cause, why it fails, quick fix approach]

### Root Cause Analysis

**Problem**: [Specific reason "no pattern arms found"]
**Evidence**: [Code location, logic flow]
**Why it happens**: [Technical explanation]

### Code Investigation

#### Pattern Arm Parser
[What does parseArms() do? Where does it fail?]

#### Regex Patterns
[What patterns are used? Are they correct?]

#### Recent Changes Impact
[Did where guards or tuple parsing break basic patterns?]

### Recommended Fix

**Approach**: [High-level strategy]

**Steps**:
1. [Specific code change with file and line]
2. [Specific code change with file and line]
3. [etc.]

**Files to Modify**:
- `path/to/file.go` - [What to change]

**Code Examples**:
```go
// BEFORE (broken)
...

// AFTER (fixed)
...
```

### Testing Strategy
[How to verify the fix works]

## Expected Results

After fix:
- `pattern_match_01_simple.dingo` should transpile successfully
- Generated .go should compile
- 12 pattern matching tests should pass
- Overall test suite: ~273/278 passing (98%)

## Important Notes

**What Already Works**:
- Type injection ✅
- Tuple patterns ✅
- Where guards ✅

**What Fails**:
- Basic variant patterns: `Ok(x) => x` ❌
- Simple patterns: `Some(x) => x` ❌
- All pattern_match_* golden tests ❌

**This Suggests**: Recent changes broke a core feature that used to work.

## Your Analysis Begins Here

Please provide comprehensive investigation focusing on why basic pattern arms aren't being detected by the parser.
