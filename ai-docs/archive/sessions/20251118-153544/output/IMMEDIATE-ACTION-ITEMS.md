# Immediate Action Items - Parser Architecture

## Quick Win Improvements (This Week)

### 1. Implement Marker Comment System
**Priority: HIGH**
**Effort: 2-3 hours**

Add support for marker comments in preprocessor for complex transformations:

```go
// Example: Error propagation marker
result := doSomething() /*DINGO:ERROR_PROP*/

// Example: Pattern match marker
/*DINGO:MATCH_START*/
switch x {
case Some(v): ...
}
/*DINGO:MATCH_END*/
```

**Files to modify:**
- `pkg/generator/preprocessor/base_preprocessor.go` - Add marker support
- `pkg/generator/plugins/transformer.go` - Detect and expand markers

### 2. Improve Preprocessor Tests
**Priority: HIGH**
**Effort: 3-4 hours**

Add edge case tests for all preprocessors:
- Nested structures
- Multi-line patterns
- String literals with Dingo syntax
- Comments containing Dingo keywords

**Files to create:**
- `tests/preprocessor/edge_cases_test.go`
- `tests/preprocessor/testdata/` - Edge case examples

### 3. Document Regex Patterns
**Priority: MEDIUM**
**Effort: 1-2 hours**

Create comprehensive documentation for each preprocessor pattern:

**File to create:** `docs/preprocessor-patterns.md`

```markdown
# Preprocessor Pattern Documentation

## TypeAnnotProcessor
Pattern: `(\w+)\s*:\s*([A-Z]\w*(?:<[^>]+>)?)`
Purpose: Transform param: Type to param Type
Examples:
- Input: `name: string` → Output: `name string`
- Input: `data: Result[T, E]` → Output: `data Result[T, E]`
Edge cases: ...
```

### 4. Create Preprocessor Debugging Tool
**Priority: MEDIUM**
**Effort: 2-3 hours**

Add debug flag to show preprocessor transformations:

```bash
dingo build --debug-preprocessor file.dingo
# Shows each transformation step with before/after
```

**Files to modify:**
- `cmd/dingo/main.go` - Add debug flag
- `pkg/generator/generator.go` - Add debug output

## Pattern Matching Preparation (Next Sprint)

### 5. Design Pattern Matching Syntax
**Priority: HIGH**
**Effort: 1 day**

Research and design pattern matching syntax that works with preprocessor:

**Options to evaluate:**
1. Marker-based approach
2. Simplified syntax that maps to switch
3. Two-phase: Preprocessor for syntax, AST for exhaustiveness

**Deliverable:** `features/pattern-matching-design.md`

### 6. Prototype Pattern Matching Preprocessor
**Priority: HIGH**
**Effort: 2-3 days**

Implement basic pattern matching transformation:

```dingo
// Input
match result {
    Ok(value) => println(value),
    Err(e) => return e,
}

// After preprocessor (with markers)
/*DINGO:MATCH_START type="Result"*/
switch result.IsOk() {
case true:
    value := result.Unwrap()
    println(value)
case false:
    e := result.UnwrapErr()
    return e
}
/*DINGO:MATCH_END*/
```

## Monitoring Setup (Ongoing)

### 7. Add Preprocessor Metrics
**Priority: LOW**
**Effort: 1-2 hours**

Track preprocessor performance and complexity:
- Transformation time per file
- Number of patterns applied
- Pattern match failures
- File size impact

**File to create:** `pkg/generator/metrics/preprocessor_metrics.go`

### 8. Create Complexity Dashboard
**Priority: LOW**
**Effort: 2-3 hours**

Simple tool to monitor preprocessor health:

```bash
dingo stats
# Output:
# Preprocessor Complexity Report:
# - Total patterns: 15
# - Average pattern length: 45 chars
# - Most complex: ErrorPropProcessor (3 patterns)
# - Test coverage: 95%
# - Edge cases covered: 23/25
```

## Documentation Updates

### 9. Update CLAUDE.md
**Priority: HIGH**
**Effort: 30 mins**

Add architecture decision summary:
- Decision to maintain current approach
- Marker comment system
- Pattern matching strategy

### 10. Create Migration Guide
**Priority: LOW**
**Effort: 1 hour**

Document migration path if we ever need to switch parsers:

**File to create:** `ai-docs/parser-migration-guide.md`
- Current architecture dependencies
- Abstraction points for parser swap
- Test strategy for migration
- Risk assessment

## Success Criteria

✅ All improvements can be completed within 1 week
✅ No breaking changes to existing functionality
✅ Pattern matching design validated against golden tests
✅ Debug tooling helps identify issues quickly
✅ Documentation prevents knowledge loss

## Timeline

**Week 1 (Immediate)**:
- Days 1-2: Marker system + tests (#1, #2)
- Days 3-4: Documentation + debugging (#3, #4)
- Day 5: Review and integration

**Week 2 (Pattern Matching)**:
- Days 1-2: Pattern matching design (#5)
- Days 3-5: Prototype implementation (#6)

**Ongoing**:
- Metrics and monitoring (#7, #8)
- Documentation updates (#9, #10)

---

These action items maintain our current successful architecture while preparing for future complexity. The focus is on enhancing what works rather than replacing it.