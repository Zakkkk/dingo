# Phase 3 Implementation Notes

## Critical Discovery: Parser is Incomplete

During implementation attempt, I discovered that the Dingo parser is **missing fundamental Go language features** required for Result/Option types to work. This is a blocking issue that must be resolved first.

## Parser Gaps Identified

### 1. Selector Expressions (CRITICAL)
**Missing**: Field access and method calls using `.` operator
**Examples that fail**:
- `result.IsOk()` - method call
- `result.ok_0` - field access
- `user.name.length` - chained selectors

**Why it matters**: Sum types generate structs with methods like `IsOk()` and fields like `ok_0`. Without selector support, these types are unusable.

**Implementation needed**: ~6-8 hours
- Add SelectorExpr to PostfixExpression
- Handle method calls vs field access
- Support chaining

### 2. Assignment Statements (CRITICAL)
**Missing**: Reassignment of existing variables
**Examples that fail**:
- `v = result.value` - simple assignment
- `x = x + 1` - update

**Why it matters**: Cannot modify variables after declaration.

**Implementation needed**: ~2-3 hours
- Add AssignStmt to Statement
- Parse `=` operator in statement context
- Convert to ast.AssignStmt

### 3. Short Variable Declaration (HIGH)
**Missing**: `:=` operator
**Examples that fail**:
- `result := divide(10, 2)`
- `x, err := readFile(path)`

**Workaround**: Use `result := divide(10, 2)`

**Implementation needed**: ~1-2 hours (low priority - has workaround)

### 4. Complex Expressions (MEDIUM)
**Missing**: Dereferencing in complex contexts
**Examples that may fail**:
- `*result.ok_0` - dereference after selector
- `&user.address` - address-of after selector

**Implementation needed**: ~2-3 hours

## Why This Blocks Phase 3

The original Phase 3 plan assumed the parser could handle Go syntax. The reality:

**Sum Types Plugin** (already working):
```go
// Generates this Go code:
type Result struct {
    tag   ResultTag
    ok_0  *float64
    err_0 *error
}

func (r Result) IsOk() bool { /* ... */ }
func Result_Ok(v float64) Result { /* ... */ }
```

**Dingo Code** (what users write):
```dingo
result := divide(10.0, 2.0)
if result.IsOk() {              // ❌ Parser error: unexpected '.'
    v := *result.ok_0         // ❌ Parser error: unexpected '.'
}
```

**The problem**: Parser can't parse the Dingo code needed to USE the generated Go types!

## Revised Implementation Plan

### NEW Phase 3.0: Parser Enhancement (15-20 hours)
**Must complete before original Phase 3**

**Sub-tasks**:
1. Add selector expression parsing (6-8h)
   - Field access: `obj.field`
   - Method calls: `obj.method(args)`
   - Chaining: `obj.field.method()`

2. Add assignment statement parsing (2-3h)
   - Simple assignment: `x = value`
   - Tuple assignment: `x, y = a, b`

3. Add short variable declaration (1-2h)
   - Single: `x := value`
   - Multiple: `x, y := a, b`

4. Add complex expression support (2-3h)
   - Dereference after selector: `*obj.field`
   - Address-of after selector: `&obj.field`

5. Testing (2-3h)
   - Test each feature independently
   - Update existing golden tests
   - Ensure error propagation tests still work

**Priority**: Highest - blocks all enum-based features

### THEN Phase 3.1-3.8: Result/Option Integration
**Can proceed after Phase 3.0**

Original plan remains valid (33-44 hours), just needs parser foundation first.

## Architectural Insights

### Parser Design Philosophy Mismatch

**Current design**: Parser handles Dingo-specific syntax only
- Error propagation `?`
- Lambdas `|x| expr`
- Match expressions
- Custom operators `??`, `?.`

**Assumption**: Go syntax already supported

**Reality**: Parser only supports subset of Go
- Basic expressions ✅
- Basic statements (let, return, if) ✅
- Field access ❌
- Method calls ❌
- Assignments ❌

### Why This Happened

Looking at the parser code, it's clear the focus was on:
1. Getting Dingo-specific features working
2. Minimal Go syntax support for demos
3. "Good enough" for Phase 1-2 features

This was pragmatic for early phases but hits a wall with enums.

### Strategic Decision Required

Two paths forward:

**Path A: Enhance Participle Parser** (recommended short-term)
- Add missing Go features incrementally
- Keep existing Dingo syntax support
- Estimated: 15-20 hours
- Pro: Faster, evolutionary
- Con: Accumulating technical debt

**Path B: Migrate to Tree-sitter** (recommended long-term)
- Use tree-sitter-go for full Go grammar
- Focus Dingo effort on transformations only
- Estimated: 40-50 hours (full migration)
- Pro: Long-term maintainability, full Go support
- Con: Significant upfront cost

**Recommendation**: Path A now, plan Path B for Phase 5-6

## Testing Strategy

### What's Working
- Error propagation tests ✅
- Basic sum types ✅
- Pattern matching (basic) ✅
- Lambda tests ✅

### What's Broken
- Result tests ❌ (all 5 files)
- Option tests ❌ (all 4 files)
- Any test using method calls ❌

### How to Verify Fix

After Phase 3.0, these should work:
```dingo
enum Result {
    Ok(float64),
    Err(error),
}

func test() {
    r := Result_Ok(42.0)
    if r.IsOk() {               // Test selector
        v := *r.ok_0         // Test selector + dereference
        v = v + 1.0             // Test assignment
    }
}
```

If all three features parse, Phase 3.1-3.8 can proceed.

## Time Investment Summary

**Originally estimated**: 33-44 hours
**Actually required**: 48-64 hours
- Phase 3.0 (Parser): 15-20 hours
- Phase 3.1-3.8 (Original plan): 33-44 hours

**Why the increase**: Discovered prerequisite work not in original plan.

## Lessons Learned

1. **Validate parser capabilities before planning features**
   - Should have run enum tests FIRST
   - Would have caught this in Phase 1

2. **Parser is the foundation**
   - Cannot build language features on incomplete parser
   - Syntax support is prerequisite, not parallel work

3. **Test-driven discovery works**
   - Attempting to run tests revealed exact gaps
   - Better than theoretical planning

4. **Incremental migration path exists**
   - Can fix parser incrementally
   - Don't need full rewrite to make progress

## Next Steps (User Decision Required)

1. **Implement Phase 3.0 (Parser Enhancement) first?**
   - 15-20 hours to add selector/assignment/dereference
   - Unblocks Result/Option
   - Enables future enum features

2. **Or defer Result/Option to Phase 4+?**
   - Focus Phase 3 entirely on parser
   - More ambitious parser improvements
   - Result/Option becomes Phase 4

3. **Or start tree-sitter migration now?**
   - 40-50 hour investment
   - Future-proof solution
   - Delays Result/Option further

My recommendation: **Option 1** (Phase 3.0 then original Phase 3)
- Fastest path to working Result/Option
- Incremental, manageable
- Proves out the design before bigger refactor
