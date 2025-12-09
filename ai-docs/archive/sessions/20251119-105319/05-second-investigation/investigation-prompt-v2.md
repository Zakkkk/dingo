# Dingo Transpiler: Type Injection Bug Investigation (Round 2)

## Context: What We've Learned

**Initial Problem**: 14+ test failures, assumed to be missing golden files
**First Investigation**: 4 external models (MiniMax M2, Grok, GPT-5.1, Gemini) diagnosed test infrastructure issues
**Actions Taken**: Implemented where guards, fixed tuple parser, fixed naming conventions
**Current Status**: Tests still failing - discovered the **real root cause**

## The Real Problem: Type Injection Failure

### Discovery
After implementing features and fixing bugs, we found that **Result and Option type declarations are missing from generated Go code**.

### Evidence

**Example 1: pattern_match_01_simple.dingo**
```dingo
fn process_result(r: Result[int, string]) -> int {
    match r {
        Ok(x) => x,
        Err(e) => 0
    }
}
```

**Generated Go Code** (BROKEN):
```go
func process_result(r Result_int_string) int {  // ← Result_int_string undefined!
    var __result int
    switch r.__tag {
    case ResultTag_Ok:  // ← ResultTag_Ok undefined!
        x := r.Result_Ok_0
        __result = x
    case ResultTag_Err:  // ← ResultTag_Err undefined!
        e := r.Result_Err_0
        __result = 0
    }
    return __result
}
```

**Type Checker Errors**:
```
undefined: Result_int_string
undefined: ResultTag_Ok
undefined: ResultTag_Err
undefined: Option
undefined: OptionTag_Some
undefined: OptionTag_None
```

### What's Missing

The generated `.go` file should include type declarations like:
```go
// Result type definition
type ResultTag uint8
const ResultTag_Ok ResultTag = 0
const ResultTag_Err ResultTag = 1

type Result_int_string struct {
    __tag ResultTag
    Result_Ok_0 int        // T value
    Result_Err_0 string    // E error
}

// Option type definition  
type OptionTag uint8
const OptionTag_Some OptionTag = 0
const OptionTag_None OptionTag = 1

type Option_int struct {
    __tag OptionTag
    Option_Some_0 int
}

// ... etc
```

**But these are NOT being generated!**

## Architecture Context

### Two-Stage Transpilation
```
.dingo file
    ↓
Stage 1: Preprocessor (text transformations)
    ↓
Stage 2: AST Processing (go/parser + plugins)
    ↓ 
    - Discovery Phase: Find Ok/Err/Some/None calls
    - Transform Phase: Rewrite AST nodes
    - Inject Phase: Add type declarations ← THIS IS BROKEN
    ↓
.go file (missing type declarations)
```

### Plugin System

**File**: `pkg/plugin/builtin/result_type.go` (727 lines)
- Contains `ResultTypePlugin` with Inject() method
- Should generate Result[T,E] type definitions
- Has logic for creating type structs, constants, helper methods

**File**: `pkg/plugin/builtin/option_type.go`
- Contains `OptionTypePlugin` with Inject() method
- Should generate Option[T] type definitions

**File**: `pkg/generator/codegen.go`
- Orchestrates plugin pipeline
- Calls Discovery → Transform → Inject phases

### Recent Changes

**What we fixed**:
1. ✅ Result naming: `ResultTagOk` → `ResultTag_Ok` (consistent underscores)
2. ✅ Where guards: Parsing and transformation working
3. ✅ Tuple patterns: Parser fixed, nested switches generated
4. ✅ Variable hoisting: Tuple bindings now accessible

**What's still broken**:
❌ Type injection: Result/Option declarations not appearing in output

## Your Investigation Task

### Primary Question
**Why are Result and Option type declarations not being injected into the generated Go code?**

### Specific Areas to Investigate

1. **Plugin Invocation**
   - Is `ResultTypePlugin.Inject()` being called?
   - Is `OptionTypePlugin.Inject()` being called?
   - Are plugins registered in the pipeline correctly?

2. **Inject Phase Logic**
   - File: `pkg/plugin/builtin/result_type.go`, `Inject()` method (around line 95-200)
   - Does it properly detect Result usage in code?
   - Does it generate AST nodes for type declarations?
   - Does it add declarations to the file's AST?

3. **Discovery Phase**
   - Does Discovery phase properly track Result/Option usage?
   - Is usage metadata passed to Inject phase?
   - File: `pkg/plugin/builtin/result_type.go`, `Discover()` method

4. **AST Manipulation**
   - Are type declarations being added to `ast.File.Decls`?
   - Is the order correct (declarations before usage)?
   - Are imports being added (if needed)?

5. **Code Generation**
   - File: `pkg/generator/codegen.go`
   - Does it call plugin Inject() methods?
   - Does it preserve injected declarations?
   - Does printer output include all AST nodes?

### Test Cases to Analyze

**Simplest failing test**: `tests/golden/pattern_match_01_simple.dingo`
- Uses Result[int, string]
- Uses Option[int]
- Only 110 lines, easy to trace

**What to check**:
1. Run transpiler with debug logging
2. Verify Discovery phase finds Result/Option usage
3. Verify Inject phase is called
4. Check if AST nodes are created
5. Check if AST nodes are added to file
6. Check if printer outputs the nodes

### Comparison: What Works

**Working features** (to compare against):
- Pattern matching transformation (works correctly)
- Variable hoisting (fixed, works)
- Where guards (implemented, works)
- Tuple parsing (fixed, works)

**Question**: Why do these work but type injection doesn't?

## Output Format

Please structure your analysis as:

### Executive Summary
[2-3 paragraphs: root cause, severity, recommended fix approach]

### Root Cause Analysis
**Problem**: [Specific reason type injection fails]
**Evidence**: [Code locations, logic flow]
**Why it happens**: [Technical explanation]

### Code Investigation

#### Plugin Registration
[Are plugins registered? Where? Evidence?]

#### Discovery Phase
[Does it find Result/Option usage? Proof?]

#### Inject Phase  
[Is Inject() called? What does it do? Where does it fail?]

#### AST Manipulation
[Are declarations created? Added to AST? Evidence?]

### Recommended Fix

**Approach**: [High-level strategy]

**Steps**:
1. [Specific action with file and function]
2. [Specific action with file and function]
3. [etc.]

**Files to Modify**:
- `path/to/file.go` - [What to change]

**Estimated Effort**: [Hours]

### Testing Strategy
[How to verify the fix works]

## Additional Context

### Key Files (For Reference)

**Plugins**:
- `pkg/plugin/builtin/result_type.go` (727 lines) - Result type generation
- `pkg/plugin/builtin/option_type.go` - Option type generation
- `pkg/plugin/interface.go` - Plugin interface definitions

**Code Generation**:
- `pkg/generator/codegen.go` - Main orchestration
- `pkg/generator/generator.go` - Transpiler entry point

**Pattern Matching** (Working - for comparison):
- `pkg/generator/preprocessor/rust_match.go` - Pattern match preprocessing
- `pkg/generator/pattern_match.go` - Pattern match transformation

**Tests**:
- `tests/golden_test.go` - Test runner
- `tests/golden/pattern_match_01_simple.dingo` - Minimal failing test

### Success Criteria

Your analysis should answer:
1. ✅ **WHY** type injection isn't working (root cause)
2. ✅ **WHERE** the bug is (specific file, function, line if possible)
3. ✅ **HOW** to fix it (concrete steps)
4. ✅ **VALIDATION** strategy (how to test the fix)

### Important Notes

- We've already fixed parser/transformation bugs
- The pipeline architecture is sound
- Type injection **used to work** (some tests have passing golden files)
- This suggests a recent regression or incomplete implementation

## Your Analysis Begins Here

Please provide your comprehensive investigation focusing on the type injection failure.
