# Phase 4.2 Architecture Plan: Pattern Guards, Swift Syntax, Tuples, Enhanced Errors

**Session**: 20251118-173201
**Date**: 2025-11-18
**Author**: golang-architect agent
**Phase**: 4.2 Planning

---

## Executive Summary

Phase 4.2 extends the successful Phase 4.1 pattern matching infrastructure with four critical enhancements:

1. **Pattern Guards** - Runtime conditions after pattern matching (`pattern if condition`)
2. **Swift Syntax Support** - Dual syntax via config (`switch/case .Variant(let x)`)
3. **Tuple Destructuring** - Match multiple values simultaneously (`(pattern1, pattern2)`)
4. **Enhanced Error Messages** - rustc-style source snippets with suggestions

**Key Constraint**: Minimal changes to Phase 4.1 infrastructure (backward compatible)

**Timeline**: 4-6 hours following Phase 4.1's proven workflow

---

## Architecture Overview

### Design Philosophy

**Reuse Phase 4.1 Infrastructure**:
- ✅ Configuration system (add Swift syntax option)
- ✅ AST parent tracking (use for type inference)
- ✅ Exhaustiveness checking (extend for guards/tuples)
- ✅ Plugin pipeline (add new transformation phases)
- ✅ Marker-based communication (preprocessor → plugin)

**New Components**:
- `SwiftMatchProcessor` (preprocessor)
- `TuplePatternTransformer` (plugin phase)
- `GuardTransformer` (plugin phase)
- `EnhancedErrorReporter` (errors package)

### Component Architecture

```
┌────────────────────────────────────────────────────────┐
│ Phase 4.2 Two-Stage Pipeline Extension                │
└────────────────────────────────────────────────────────┘

Stage 1: Preprocessor (Text → Valid Go)
┌────────────────────────────────────────────────────────┐
│ RustMatchProcessor (Phase 4.1) ✅                      │
│ - match expr { Pattern if guard => expr }             │
│ - Parses guards, attaches as DINGO_GUARD markers       │
└────────────────────────────────────────────────────────┘
                     ↓
┌────────────────────────────────────────────────────────┐
│ SwiftMatchProcessor (NEW)                              │
│ - switch expr { case .Variant(let x): ... }           │
│ - Normalizes to same Go switch + markers as Rust       │
│ - Config-driven (match.syntax = "swift")               │
└────────────────────────────────────────────────────────┘
                     ↓
Stage 2: Plugin Pipeline (AST → Transformed AST)
┌────────────────────────────────────────────────────────┐
│ PatternMatchPlugin (Phase 4.1) ✅                      │
│ - Discovery: Find DINGO_MATCH markers                  │
│ - Exhaustiveness: Check all variants (extend for tuples)│
└────────────────────────────────────────────────────────┘
                     ↓
┌────────────────────────────────────────────────────────┐
│ TuplePatternTransformer (NEW)                          │
│ - Detect tuple scrutinee (multiple return values)      │
│ - Generate temp vars for tuple elements                │
│ - Rewrite patterns to check each element               │
└────────────────────────────────────────────────────────┘
                     ↓
┌────────────────────────────────────────────────────────┐
│ GuardTransformer (NEW)                                 │
│ - Parse DINGO_GUARD markers                            │
│ - Inject if statements after pattern match             │
│ - Handle fallthrough for failed guards                 │
└────────────────────────────────────────────────────────┘
                     ↓
┌────────────────────────────────────────────────────────┐
│ EnhancedErrorReporter (NEW)                            │
│ - Source snippet extraction (line ranges)              │
│ - Caret positioning (^^^^ under error location)        │
│ - Suggestion generation (missing patterns)             │
│ - Integration with errors.CompileError                 │
└────────────────────────────────────────────────────────┘
```

---

## Feature 1: Pattern Guards

### Syntax

```dingo
match value {
    Ok(x) if x > 0 => handlePositive(x),
    Ok(x) => handleNonPositive(x),
    Err(e) => handleError(e)
}
```

### Preprocessor Changes

**File**: `pkg/preprocessor/rust_match.go` (extend existing)

**Parse Guards**:
```go
// Extend parsePatternArm to recognize "if" clause
// Pattern: `Ok(x) if x > 0 => expr`
type patternArm struct {
    pattern    string // "Ok(x)"
    guard      string // "x > 0" (optional)
    expression string // "handlePositive(x)"
}

// Transform to Go with markers:
case __scrutinee.Tag == "Ok": // DINGO_GUARD: x > 0
    x := __scrutinee.Value.(int)
    if !(x > 0) { goto __match_fallthrough_1 }
    return handlePositive(x)
__match_fallthrough_1:
```

**Key Design**:
- Guards become runtime `if` checks after pattern binding
- Failed guards jump to next case via `goto` labels
- Exhaustiveness checking **ignores guards** (treats as non-exhaustive pattern)

### Plugin Changes

**File**: `pkg/plugin/builtin/pattern_match.go` (extend existing)

**GuardTransformer Phase** (new):
```go
type guardInfo struct {
    caseClause  *ast.CaseClause
    condition   string // Parsed from DINGO_GUARD marker
    labelName   string // __match_fallthrough_N
}

func (p *PatternMatchPlugin) transformGuards(match *matchExpression) error {
    for i, caseClause := range match.caseStmts {
        guard := p.findGuardMarker(caseClause)
        if guard == "" {
            continue
        }

        // Parse condition
        condExpr, err := parser.ParseExpr(guard)
        if err != nil {
            return fmt.Errorf("invalid guard condition: %w", err)
        }

        // Generate fallthrough label
        labelName := fmt.Sprintf("__match_fallthrough_%d", i)

        // Inject: if !(condition) { goto label }
        p.injectGuardCheck(caseClause, condExpr, labelName)

        // Inject label before next case
        p.injectFallthroughLabel(match.switchStmt, i+1, labelName)
    }
}
```

**Exhaustiveness Impact**:
- Guards do NOT affect exhaustiveness (runtime checks, not compile-time)
- Example: `Ok(x) if x > 0` + `Ok(x)` = both patterns for Ok (valid)
- Example: `Ok(x) if x > 0` alone = non-exhaustive (missing non-positive case)

### Testing

**Golden Tests**:
- `pattern_match_05_guards_simple.dingo` - Basic guard usage
- `pattern_match_06_guards_multiple.dingo` - Multiple guards on same variant
- `pattern_match_07_guards_complex.dingo` - Nested guards with expressions

**Unit Tests**:
- `TestRustMatchGuardParsing` - Verify guard extraction
- `TestGuardTransformation` - Verify goto/label generation
- `TestGuardExhaustiveness` - Verify guards don't satisfy exhaustiveness

---

## Feature 2: Swift Pattern Syntax Support

### Syntax

```dingo
// Swift-style
switch value {
case .Ok(let x):
    handleOk(x)
case .Err(let e):
    handleError(e)
}
```

### Configuration Extension

**File**: `pkg/config/config.go` (already has `match.syntax`)

```toml
[match]
syntax = "swift"  # Options: "rust" (default), "swift"
```

**No changes needed** - already implemented in Phase 4.1!

### Preprocessor Implementation

**File**: `pkg/preprocessor/swift_match.go` (NEW)

**Architecture**:
```go
type SwiftMatchProcessor struct {
    matchCounter int
    mappings     []Mapping
}

func (s *SwiftMatchProcessor) Process(source []byte) ([]byte, []Mapping, error) {
    // Similar to RustMatchProcessor, but different regex patterns

    // Pattern: switch expr { case .Variant(let x): ... }
    // Transform to: switch __scrutinee.Tag { case "Variant": x := ... }

    // Normalize to SAME markers as Rust:
    // - DINGO_MATCH_START
    // - DINGO_PATTERN_ARM
    // - DINGO_MATCH_END

    // Plugin sees IDENTICAL AST regardless of syntax!
}
```

**Key Differences from Rust**:

| Aspect | Rust Syntax | Swift Syntax |
|--------|-------------|--------------|
| Keyword | `match expr {` | `switch expr {` |
| Case | `Pattern =>` | `case .Pattern:` |
| Binding | `Ok(x)` | `.Ok(let x)` |
| Dot prefix | No | Yes (`.Variant`) |
| Separator | `,` | `:` |

**Normalization Strategy**:
- Swift preprocessor emits **same markers** as Rust preprocessor
- Plugin sees identical AST structure
- Only syntax differs (preprocessor handles conversion)

### Integration

**File**: `pkg/generator/generator.go`

```go
func (g *Generator) selectMatchProcessor(cfg *config.Config) Processor {
    switch cfg.Match.Syntax {
    case "rust":
        return preprocessor.NewRustMatchProcessor()
    case "swift":
        return preprocessor.NewSwiftMatchProcessor()
    default:
        return preprocessor.NewRustMatchProcessor() // Default
    }
}
```

### Testing

**Golden Tests**:
- `pattern_match_08_swift_simple.dingo` - Basic Swift syntax
- `pattern_match_09_swift_bindings.dingo` - Swift let bindings
- `pattern_match_10_swift_guards.dingo` - Swift with guards

**Config Tests**:
- Test both `rust` and `swift` configurations produce same behavior
- Test syntax validation (error on invalid syntax value)

---

## Feature 3: Tuple Destructuring

### Syntax

```dingo
match getTuple() {
    (Ok(x), Ok(y)) => handleBoth(x, y),
    (Ok(x), Err(e)) => handlePartial(x, e),
    (Err(e), _) => handleFirstError(e)
}
```

### Preprocessor Changes

**File**: `pkg/preprocessor/rust_match.go` (extend existing)

**Tuple Detection**:
```go
// Detect tuple scrutinee: (expr1, expr2, ...)
// Transform to:
//   __tuple := getTuple()
//   __elem0, __elem1 := __tuple._0, __tuple._1
//   switch __elem0.Tag {
//   case "Ok": // DINGO_TUPLE_PATTERN: (Ok, Ok) | (Ok, Err)
//       ...
//   }

func (r *RustMatchProcessor) detectTuple(scrutinee string) (bool, []string) {
    // Parse: (expr1, expr2, ...)
    // Return: isTuple, []elementNames
}

func (r *RustMatchProcessor) parseTuplePatterns(arms []patternArm) []tuplePattern {
    // Parse: (Pattern1, Pattern2)
    // Return: [][]string{{"Ok", "Ok"}, {"Ok", "Err"}, ...}
}
```

**Marker Format**:
```go
// DINGO_TUPLE_PATTERN: (Ok, Ok) | (Ok, Err) | (Err, _)
// Each | separates a pattern arm
// Each , separates tuple elements
```

### Plugin Implementation

**File**: `pkg/plugin/builtin/pattern_match.go` (extend existing)

**TuplePatternTransformer Phase** (new):
```go
func (p *PatternMatchPlugin) detectTupleMatch(match *matchExpression) bool {
    // Look for DINGO_TUPLE_PATTERN marker
    return p.findComment(match.switchStmt, "DINGO_TUPLE_PATTERN") != nil
}

func (p *PatternMatchPlugin) transformTupleMatch(match *matchExpression) error {
    // 1. Parse tuple pattern marker: (Ok, Ok) | (Ok, Err) | ...
    // 2. Generate switch on first element tag
    // 3. For each case, generate nested switch on second element tag
    // 4. Extract bindings for all matched elements

    // Example transformation:
    //   match (r1, r2) {
    //       (Ok(x), Ok(y)) => x + y,
    //       (Ok(x), Err(e)) => x,
    //       (Err(e), _) => 0
    //   }
    //
    // Becomes:
    //   __tuple0, __tuple1 := r1, r2
    //   switch __tuple0.Tag {
    //   case "Ok":
    //       x := __tuple0.Value.(int)
    //       switch __tuple1.Tag {
    //       case "Ok":
    //           y := __tuple1.Value.(int)
    //           return x + y
    //       case "Err":
    //           e := __tuple1.Value.(error)
    //           return x
    //       }
    //   case "Err":
    //       e := __tuple0.Value.(error)
    //       return 0
    //   }
}
```

### Exhaustiveness for Tuples

**Challenge**: Exponential pattern space
- 2 elements × 2 variants = 4 patterns: (Ok,Ok), (Ok,Err), (Err,Ok), (Err,Err)
- 3 elements × 2 variants = 8 patterns
- N elements × M variants = M^N patterns

**Strategy**: Partial exhaustiveness checking
```go
func (p *PatternMatchPlugin) checkTupleExhaustiveness(patterns []tuplePattern) error {
    // Build decision tree for each position
    // Check if all paths lead to a pattern (or wildcard)

    // Algorithm: Matrix coverage
    // Example for Result[T,E] tuple (2 elements):
    //   Covered: (Ok, Ok), (Ok, Err), (Err, _)
    //   Missing: None (Err, _) covers (Err, Ok) and (Err, Err)

    // Use wildcard propagation:
    //   (_, Ok) covers (Ok, Ok) and (Err, Ok)
    //   (Err, _) covers (Err, Ok) and (Err, Err)
}
```

### Testing

**Golden Tests**:
- `pattern_match_11_tuple_simple.dingo` - 2-element tuple, all Result
- `pattern_match_12_tuple_wildcard.dingo` - Using _ in tuple positions
- `pattern_match_13_tuple_mixed.dingo` - Result + Option tuple

**Unit Tests**:
- `TestTuplePatternParsing` - Parse tuple patterns
- `TestTupleTransformation` - Verify nested switch generation
- `TestTupleExhaustiveness` - Verify wildcard coverage

---

## Feature 4: Enhanced Error Messages

### Goal

Transform basic errors:
```
Error: non-exhaustive match at line 42
```

Into rustc-style errors:
```
Error: Non-exhaustive match in file.dingo:42:5

  40 |     let result = fetchData()
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

### Architecture

**File**: `pkg/errors/enhanced.go` (NEW)

```go
package errors

import (
    "fmt"
    "go/token"
    "strings"
)

// EnhancedError extends CompileError with source snippets
type EnhancedError struct {
    *CompileError

    // Source context
    SourceLines []string // Lines to display (e.g., lines 40-43)
    ErrorLine   int      // Which line has the error (1-indexed in SourceLines)
    ErrorCol    int      // Column position (for caret ^^^^^)

    // Suggestions
    Suggestion  string   // Multi-line suggestion text
    MissingPatterns []string // For exhaustiveness errors
}

// Format produces rustc-style error message
func (e *EnhancedError) Format() string {
    var buf strings.Builder

    // Header: Error: <message> in <file>:<line>:<col>
    fmt.Fprintf(&buf, "Error: %s in %s:%d:%d\n\n",
        e.Message, e.Filename, e.Line, e.Column)

    // Source snippet with line numbers
    startLine := e.Line - 2
    for i, line := range e.SourceLines {
        lineNum := startLine + i
        if lineNum == e.Line {
            // Error line - add caret
            fmt.Fprintf(&buf, "  %d | %s\n", lineNum, line)
            fmt.Fprintf(&buf, "     | %s%s %s\n",
                strings.Repeat(" ", e.ErrorCol),
                strings.Repeat("^", len(e.highlightSpan())),
                e.errorAnnotation())
        } else {
            // Context line
            fmt.Fprintf(&buf, "  %d | %s\n", lineNum, line)
        }
    }

    // Suggestion section
    if e.Suggestion != "" {
        fmt.Fprintf(&buf, "\nSuggestion: %s\n", e.Suggestion)
    }

    return buf.String()
}
```

### Integration Points

**Pattern Match Plugin**:
```go
func (p *PatternMatchPlugin) checkExhaustiveness(match *matchExpression) error {
    missing := p.computeMissingPatterns(match)
    if len(missing) == 0 {
        return nil // Exhaustive
    }

    // Build enhanced error
    err := errors.NewEnhancedError(
        p.ctx.Fset,
        match.startPos,
        fmt.Sprintf("Non-exhaustive match"),
    )

    // Extract source lines
    err.SourceLines = p.extractSourceLines(match.startPos, 5)
    err.ErrorLine = 2 // Relative to SourceLines
    err.ErrorCol = p.getColumn(match.startPos)

    // Generate suggestion
    err.Suggestion = p.generateExhaustivenessSuggestion(match, missing)
    err.MissingPatterns = missing

    return err
}
```

**Source Line Extraction**:
```go
func (p *PatternMatchPlugin) extractSourceLines(pos token.Pos, contextLines int) []string {
    // Use token.FileSet to get file
    file := p.ctx.Fset.File(pos)

    // Read source file
    source, err := os.ReadFile(file.Name())
    if err != nil {
        return nil
    }

    // Get line number
    lineNum := file.Line(pos)

    // Extract lines [lineNum - contextLines, lineNum + contextLines]
    lines := strings.Split(string(source), "\n")
    start := max(0, lineNum - contextLines - 1)
    end := min(len(lines), lineNum + contextLines)

    return lines[start:end]
}
```

### Suggestion Generation

**Exhaustiveness Suggestions**:
```go
func (p *PatternMatchPlugin) generateExhaustivenessSuggestion(
    match *matchExpression,
    missing []string,
) string {
    var buf strings.Builder

    fmt.Fprintf(&buf, "Add pattern to handle all cases:\n")
    fmt.Fprintf(&buf, "    match %s {\n", match.scrutinee)

    // Show existing patterns
    for _, pattern := range match.patterns {
        fmt.Fprintf(&buf, "        %s => ...,\n", pattern)
    }

    // Show missing patterns with comment
    for _, pattern := range missing {
        fmt.Fprintf(&buf, "        %s => ...  // Add this\n", pattern)
    }

    fmt.Fprintf(&buf, "    }")

    return buf.String()
}
```

**Type Error Suggestions**:
```go
func (p *PatternMatchPlugin) suggestTypeCorrection(
    expected string,
    actual string,
) string {
    // Example: Expected Result[int, error], got Option[int]
    return fmt.Sprintf("Did you mean to use '%s' instead of '%s'?",
        expected, actual)
}
```

### Testing

**Golden Tests**:
- `pattern_match_14_error_exhaustiveness.dingo` - Test exhaustiveness error message
- `pattern_match_15_error_type_mismatch.dingo` - Test type error message
- `pattern_match_16_error_guard.dingo` - Test guard-related errors

**Unit Tests**:
- `TestEnhancedErrorFormat` - Verify error formatting
- `TestSourceLineExtraction` - Verify line extraction logic
- `TestSuggestionGeneration` - Verify suggestion quality

---

## Implementation Sequence

### Phase 1: Pattern Guards (1-1.5 hours)

**Tasks**:
1. Extend `RustMatchProcessor` to parse guards
2. Add `DINGO_GUARD` marker support
3. Implement `GuardTransformer` in plugin
4. Add goto/label generation
5. Write 3 golden tests
6. Write 5 unit tests

**Deliverables**:
- `pkg/preprocessor/rust_match.go` (modified)
- `pkg/plugin/builtin/pattern_match.go` (modified)
- `tests/golden/pattern_match_05_guards_*.dingo` (3 files)
- `pkg/preprocessor/rust_match_test.go` (extended)

### Phase 2: Swift Syntax (1-1.5 hours)

**Tasks**:
1. Create `SwiftMatchProcessor`
2. Implement Swift → marker normalization
3. Add processor selection in generator
4. Write 3 golden tests (Swift syntax)
5. Write 5 unit tests

**Deliverables**:
- `pkg/preprocessor/swift_match.go` (NEW)
- `pkg/preprocessor/swift_match_test.go` (NEW)
- `pkg/generator/generator.go` (modified)
- `tests/golden/pattern_match_08_swift_*.dingo` (3 files)

### Phase 3: Tuple Destructuring (1.5-2 hours)

**Tasks**:
1. Extend preprocessors to detect tuples
2. Add `DINGO_TUPLE_PATTERN` markers
3. Implement `TuplePatternTransformer`
4. Implement tuple exhaustiveness checking
5. Write 3 golden tests
6. Write 7 unit tests

**Deliverables**:
- `pkg/preprocessor/rust_match.go` (modified)
- `pkg/preprocessor/swift_match.go` (modified)
- `pkg/plugin/builtin/pattern_match.go` (modified)
- `tests/golden/pattern_match_11_tuple_*.dingo` (3 files)

### Phase 4: Enhanced Errors (1-1.5 hours)

**Tasks**:
1. Create `errors/enhanced.go`
2. Implement source line extraction
3. Implement caret positioning
4. Implement suggestion generation
5. Integrate with PatternMatchPlugin
6. Write 3 golden tests
7. Write 4 unit tests

**Deliverables**:
- `pkg/errors/enhanced.go` (NEW)
- `pkg/errors/enhanced_test.go` (NEW)
- `pkg/plugin/builtin/pattern_match.go` (modified)
- `tests/golden/pattern_match_14_error_*.dingo` (3 files)

### Phase 5: Integration & Testing (0.5-1 hour)

**Tasks**:
1. Run full test suite (Phase 4.1 + 4.2)
2. Fix any integration issues
3. Update documentation
4. Code review preparation

**Target**: 100% test pass rate (57 Phase 4.1 + 19 Phase 4.2 = 76 tests)

---

## Integration with Phase 4.1

### Reused Components

| Component | Phase 4.1 Status | Phase 4.2 Usage |
|-----------|------------------|-----------------|
| `config.Config` | ✅ Implemented | Extend with Swift syntax |
| `ParentMap` | ✅ Implemented | Use for type inference |
| `PatternMatchPlugin` | ✅ Implemented | Extend with guards/tuples |
| `RustMatchProcessor` | ✅ Implemented | Extend with guards/tuples |
| `Exhaustiveness checker` | ✅ Implemented | Extend for tuple patterns |
| `errors.CompileError` | ✅ Implemented | Wrap with EnhancedError |

### New Components

| Component | Purpose | Lines of Code |
|-----------|---------|---------------|
| `SwiftMatchProcessor` | Swift syntax preprocessor | ~200 lines |
| `TuplePatternTransformer` | Tuple destructuring logic | ~150 lines |
| `GuardTransformer` | Guard condition injection | ~100 lines |
| `EnhancedError` | Source snippet errors | ~200 lines |

**Total New Code**: ~650 lines

### Backward Compatibility

**Guaranteed**:
- Phase 4.1 tests MUST continue passing (57/57)
- No changes to existing marker formats
- Config defaults unchanged (rust syntax, no guards)
- Plugin ordering preserved

**Migration Path**:
- Users can opt-in to Swift syntax via `dingo.toml`
- Guards are additive (existing match expressions work)
- Tuples are additive (existing single-value matches work)
- Enhanced errors are automatic (no config needed)

---

## Risk Analysis

### High Risk: Tuple Exhaustiveness Complexity

**Problem**: Exponential pattern space (M^N for M variants, N elements)

**Mitigation**:
1. Use decision tree algorithm (not brute force)
2. Leverage wildcard propagation (_, _) covers all
3. Set reasonable limits (e.g., max 4 tuple elements)
4. Provide clear error when limit exceeded

**Fallback**: Conservative checking (allow non-exhaustive with warning)

### Medium Risk: Guard Goto/Label Generation

**Problem**: Label name collisions, complex control flow

**Mitigation**:
1. Use unique counter: `__match_fallthrough_{matchId}_{armId}`
2. Test edge cases (nested matches, guards in loops)
3. Validate generated Go code compiles

**Fallback**: Transform guards to nested `if` (slower but safer)

### Medium Risk: Swift Syntax Parser Complexity

**Problem**: Different syntax rules, binding patterns

**Mitigation**:
1. Normalize to Rust markers early (preprocessor only)
2. Reuse Phase 4.1 plugin logic (identical markers)
3. Test cross-syntax compatibility

**Fallback**: Start with Swift as experimental feature

### Low Risk: Enhanced Error Source Extraction

**Problem**: File I/O overhead, encoding issues

**Mitigation**:
1. Cache file contents in Context
2. Handle UTF-8 properly
3. Fallback to basic error if extraction fails

**Fallback**: Show basic error without snippets

---

## Performance Targets

| Operation | Phase 4.1 Target | Phase 4.2 Target | Rationale |
|-----------|------------------|------------------|-----------|
| Preprocessor | N/A | <5ms per file | Regex-based, fast |
| Guard transform | N/A | <0.5ms per match | Simple AST injection |
| Tuple transform | N/A | <1ms per match | Nested switch generation |
| Enhanced error | N/A | <10ms per error | File I/O for snippets |
| Total overhead | ~15ms/file | ~20ms/file | +5ms acceptable |

**Measurement**: Use existing benchmark infrastructure from Phase 4.1

---

## Testing Strategy

### Golden Tests (12 new tests)

**Pattern Guards** (3 tests):
- `pattern_match_05_guards_simple.dingo` - Basic if condition
- `pattern_match_06_guards_multiple.dingo` - Multiple guards per variant
- `pattern_match_07_guards_complex.dingo` - Guards with complex expressions

**Swift Syntax** (3 tests):
- `pattern_match_08_swift_simple.dingo` - Basic switch/case
- `pattern_match_09_swift_bindings.dingo` - let/var bindings
- `pattern_match_10_swift_guards.dingo` - Swift with guards

**Tuple Destructuring** (3 tests):
- `pattern_match_11_tuple_simple.dingo` - 2-element Result tuple
- `pattern_match_12_tuple_wildcard.dingo` - Wildcards in tuples
- `pattern_match_13_tuple_mixed.dingo` - Result + Option tuple

**Enhanced Errors** (3 tests):
- `pattern_match_14_error_exhaustiveness.dingo` - Exhaustiveness error
- `pattern_match_15_error_type_mismatch.dingo` - Type error
- `pattern_match_16_error_guard.dingo` - Guard error

### Unit Tests (21 new tests)

**RustMatchProcessor**:
- `TestRustMatchGuardParsing` - Parse guards from match arms
- `TestRustMatchTupleParsing` - Parse tuple patterns
- `TestRustMatchGuardMarkers` - Verify DINGO_GUARD markers
- `TestRustMatchTupleMarkers` - Verify DINGO_TUPLE_PATTERN markers

**SwiftMatchProcessor**:
- `TestSwiftMatchBasicParsing` - Parse Swift syntax
- `TestSwiftMatchBindings` - Parse let/var bindings
- `TestSwiftMatchNormalization` - Verify Rust marker equivalence

**PatternMatchPlugin**:
- `TestGuardTransformation` - Verify goto/label injection
- `TestGuardExhaustiveness` - Verify guards don't satisfy exhaustiveness
- `TestTupleTransformation` - Verify nested switch generation
- `TestTupleExhaustiveness` - Verify wildcard coverage
- `TestTupleArity` - Verify arity mismatch errors

**EnhancedError**:
- `TestEnhancedErrorFormat` - Verify rustc-style formatting
- `TestSourceLineExtraction` - Verify line extraction
- `TestCaretPositioning` - Verify ^^^ placement
- `TestSuggestionGeneration` - Verify suggestion quality
- `TestEnhancedErrorFallback` - Verify graceful degradation

**Integration**:
- `TestPhase41Compatibility` - Verify all Phase 4.1 tests pass
- `TestGuardsWithTuples` - Combined feature test
- `TestSwiftWithGuards` - Combined feature test
- `TestEnhancedErrorsAllCases` - All error types

### Test Coverage Target

- **Preprocessors**: >90% coverage
- **Plugins**: >85% coverage
- **Errors**: >80% coverage
- **Overall**: Maintain Phase 4.1's >80% average

---

## Documentation Updates

### Files to Create

1. **`docs/pattern-guards.md`** - Pattern guard usage guide
2. **`docs/swift-syntax.md`** - Swift syntax reference
3. **`docs/tuple-destructuring.md`** - Tuple pattern guide
4. **`docs/error-messages.md`** - Error message examples

### Files to Update

1. **`tests/golden/README.md`** - Add Phase 4.2 test catalog
2. **`CHANGELOG.md`** - Phase 4.2 entry
3. **`ai-docs/sessions/20251118-173201/session-summary.md`** - Session results

---

## Success Criteria

### Functionality

- ✅ Pattern guards working (if conditions)
- ✅ Swift syntax fully supported (switch/case)
- ✅ Tuple destructuring working (2-4 elements)
- ✅ Enhanced errors with source snippets

### Quality

- ✅ 100% test pass rate (76 total tests: 57 Phase 4.1 + 19 Phase 4.2)
- ✅ Performance: <20ms total overhead per file
- ✅ Clean, idiomatic Go output
- ✅ Code review approved by multiple reviewers

### Backward Compatibility

- ✅ All Phase 4.1 tests still passing
- ✅ No breaking changes to existing configs
- ✅ No changes to existing golden test outputs

### Documentation

- ✅ 4 new feature guides written
- ✅ Updated test catalog
- ✅ Session summary complete

---

## Open Questions (See gaps.json)

1. **Tuple element limit**: Max 4 elements? Or higher?
2. **Guard fallthrough strategy**: goto labels vs nested if?
3. **Swift case bodies**: Require braces `{ ... }` or allow bare statements?
4. **Error verbosity level**: Configurable via CLI flag?
5. **Tuple wildcard semantics**: `(_, _)` covers all or requires explicit patterns?

---

## Appendix: Example Transformations

### Example 1: Pattern Guard

**Input (Dingo)**:
```dingo
match result {
    Ok(x) if x > 0 => x * 2,
    Ok(x) => 0,
    Err(e) => -1
}
```

**Output (Go)**:
```go
switch __result.Tag {
case "Ok": // DINGO_GUARD: x > 0
    x := __result.Value.(int)
    if !(x > 0) {
        goto __match_fallthrough_0
    }
    return x * 2
__match_fallthrough_0:
case "Ok":
    x := __result.Value.(int)
    return 0
case "Err":
    e := __result.Value.(error)
    return -1
default:
    panic("unreachable")
}
```

### Example 2: Swift Syntax

**Input (Dingo - Swift style)**:
```dingo
switch value {
case .Ok(let x):
    handleOk(x)
case .Err(let e):
    handleError(e)
}
```

**Preprocessor Output (Normalized)**:
```go
// Same as Rust: DINGO_MATCH_START markers
switch __value.Tag { // DINGO_MATCH_START: value
case "Ok": // DINGO_PATTERN_ARM: Ok
    x := __value.Value.(int)
    handleOk(x)
case "Err": // DINGO_PATTERN_ARM: Err
    e := __value.Value.(error)
    handleError(e)
} // DINGO_MATCH_END
```

### Example 3: Tuple Destructuring

**Input (Dingo)**:
```dingo
match (result1, result2) {
    (Ok(x), Ok(y)) => x + y,
    (Ok(x), Err(e)) => x,
    (Err(e), _) => 0
}
```

**Output (Go)**:
```go
__tuple0, __tuple1 := result1, result2
switch __tuple0.Tag {
case "Ok":
    x := __tuple0.Value.(int)
    switch __tuple1.Tag {
    case "Ok":
        y := __tuple1.Value.(int)
        return x + y
    case "Err":
        e := __tuple1.Value.(error)
        return x
    default:
        panic("unreachable")
    }
case "Err":
    e := __tuple0.Value.(error)
    return 0
default:
    panic("unreachable")
}
```

### Example 4: Enhanced Error

**Input (Non-exhaustive match)**:
```dingo
match result {
    Ok(x) => x * 2
}
```

**Error Output**:
```
Error: Non-exhaustive match in example.dingo:42:5

  40 |     let result = fetchData()
  41 |     match result {
  42 |         Ok(x) => x * 2
     |         ^^^^^^^^^^^^^^ Missing pattern: Err(_)
  43 |     }

Suggestion: Add pattern to handle all cases:
    match result {
        Ok(x) => x * 2,
        Err(e) => ...  // Add this
    }
```

---

## Conclusion

Phase 4.2 builds on Phase 4.1's solid foundation with four high-value enhancements:
- Guards enable runtime filtering (Rust/Swift feature parity)
- Swift syntax expands user choice (dual-syntax support)
- Tuples enable multi-value matching (common use case)
- Enhanced errors improve DX (rustc-quality feedback)

**Minimal Risk**: Reuses 90% of Phase 4.1 infrastructure
**High Value**: Completes pattern matching MVP
**Timeline**: 4-6 hours (proven workflow)

**Next**: Gather user decisions from gaps.json, proceed to implementation.
