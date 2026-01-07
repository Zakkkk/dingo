# Pratt Parser Enhancement Code Review

**Date**: 2026-01-06
**Reviewer**: Internal (code-reviewer agent)
**Files Reviewed**:
- `pkg/parser/errors.go` (new)
- `pkg/parser/lookahead.go` (new)
- `pkg/parser/pratt.go` (modified)
- `pkg/parser/lookahead_test.go` (new)
- `pkg/parser/errors_test.go` (new)
- `pkg/parser/pratt_bench_test.go` (new)
- `pkg/parser/recovery_test.go` (existing)

---

## Strengths

### 1. Clean Error Infrastructure (errors.go)

The `SpanError` design is well-suited for LSP integration:

- **Full span information**: `Pos`, `EndPos`, `Line`, `Column`, `EndCol` - all the fields needed for LSP diagnostic ranges
- **Error codes**: Structured codes like `ErrUnexpectedToken` (E001) enable documentation linking and tooling
- **Hints**: The `Hint` field allows context-aware suggestions without cluttering the main message
- **Context field**: Useful for nested parsing contexts ("in match expression", "in lambda body")

The fluent builder pattern (`NewErrorBuilder().EndPos().Code().Build()`) is idiomatic and reduces error construction boilerplate.

### 2. Decision Table Design (lookahead.go)

The `classifyQuestionOperator()` function is a significant improvement over the previous 106-line implementation:

```go
// Decision table based on token after ?:
//   Token after ?   | Check                      | Classification
//   ----------------|----------------------------|---------------
//   terminator      | -                          | qkErrorPropPostfix
//   ?               | -                          | qkErrorPropPostfix (chained)
//   STRING          | has colon after?           | context/ternary
//   PIPE            | -                          | qkErrorWithRustLambda
//   LPAREN          | isTypeScriptLambda()?      | lambda/ternary
//   IDENT + ARROW   | -                          | qkErrorWithTSLambda
//   other           | try parse, has colon?      | ternary/postfix
```

This is much more maintainable than nested if-else chains. Each pattern is a clear entry point.

### 3. State Management (pratt.go)

The `saveState()`/`restoreState()` pattern correctly preserves:
- `curToken`
- `peekToken`
- Tokenizer position (via `SavePos()`)

The use of `defer p.restoreState(state)` in lookahead functions ensures proper cleanup.

### 4. Comprehensive Benchmarks (pratt_bench_test.go)

The benchmark suite covers all critical paths:
- Single expression parsing (all major patterns)
- Lookahead operations (positive/negative cases)
- Parser creation overhead
- State save/restore operations
- Complex nested expressions
- Large file simulation

### 5. Position Tracking Compliance

The implementation correctly uses `token.Pos` throughout, avoiding forbidden byte arithmetic patterns. Positions flow from tokenizer tokens, not from string scanning.

---

## Concerns

### CRITICAL (Must Fix)

#### 1. sync.Pool Not Actually Used (pratt.go:185-209)

**File**: `pkg/parser/pratt.go`
**Lines**: 185-209

The `saveStatePooled()`, `restoreStatePooled()`, and `releaseState()` functions are defined but **never called anywhere** in the codebase:

```bash
$ grep -r "saveStatePooled\|restoreStatePooled\|releaseState" pkg/parser/
# Only shows the function definitions, no actual usage
```

**Impact**: Dead code that suggests incomplete refactoring. The `sync.Pool` is initialized at package level, consuming memory for no benefit.

**Recommendation**: Either:
1. Remove the pooled variants if benchmarks show the non-pooled version is adequate
2. Use the pooled variants in hot paths (`classifyQuestionOperator`, `hasTernaryColon`, `isTypeScriptLambda`)

#### 2. Race Condition Risk in sync.Pool Usage (pratt.go:188-208)

**File**: `pkg/parser/pratt.go`
**Lines**: 188-208

If the pooled functions were used, there's a subtle correctness issue:

```go
func (p *PrattParser) releaseState(state *parserState) {
    parserStatePool.Put(state)
}
```

The `releaseState()` function returns the state to the pool **without clearing it**. If another goroutine gets this state from the pool, it would contain stale token data.

**Recommendation**: Clear state fields before returning to pool:
```go
func (p *PrattParser) releaseState(state *parserState) {
    state.curToken = tokenizer.Token{}
    state.peekToken = tokenizer.Token{}
    state.tokPos = 0
    parserStatePool.Put(state)
}
```

### IMPORTANT (Should Fix)

#### 3. Missing Test: Error Propagation with Chained Question Operators (lookahead_test.go)

**File**: `pkg/parser/lookahead_test.go`

The test for chained `?` operators only tests the classification:
```go
{"chained_question", "?", qkErrorPropPostfix},
```

But there's no integration test for parsing `getData()??` (which should be error prop followed by another error prop, not null coalescing `??`).

**Recommendation**: Add a test case in `parser_test.go`:
```go
{"chained_error_prop", "getData()??", wantAST: ...},
```

#### 4. hasColonAfterToken() Doesn't Handle All Newline Cases (lookahead.go:107-114)

**File**: `pkg/parser/lookahead.go`
**Lines**: 107-114

```go
func (p *PrattParser) hasColonAfterToken() bool {
    state := p.saveState()
    defer p.restoreState(state)

    p.nextToken() // move past current token
    p.consumeNewlinesAndComments()
    return p.curTokenIs(tokenizer.COLON)
}
```

If there are **multiple** newlines/comments between the token and colon, this should still work because `consumeNewlinesAndComments()` loops. However, the test only covers single newline:

```go
{"newline_then_colon", "\"text\"\n:", true},
```

**Recommendation**: Add test case for multiple newlines:
```go
{"multi_newline_then_colon", "\"text\"\n\n\n:", true},
```

#### 5. hasTernaryColon() maxLookahead Limit May Be Insufficient (lookahead.go:123)

**File**: `pkg/parser/lookahead.go`
**Line**: 123

```go
maxLookahead := 20 // Prevent runaway lookahead
```

20 tokens is quite restrictive. A ternary with a function call in the true branch could easily exceed this:
```
condition ? someFunction(arg1, arg2, arg3) : defaultValue
//          ^--- could be 10+ tokens just for the call
```

**Impact**: Complex ternary expressions might be misclassified as error propagation.

**Recommendation**: Either:
1. Increase limit to 50 (with benchmarking to verify acceptable performance)
2. Add a test case that demonstrates the limit is sufficient for realistic expressions

#### 6. parseBinaryExpr Returns nil Without Error Recording (pratt.go:1064-1067)

**File**: `pkg/parser/pratt.go`
**Lines**: 1064-1067

```go
// Handle nil right operand (parse error occurred)
if right == nil {
    return nil
}
```

When the right operand fails to parse, the function returns `nil` without adding its own error. This relies on the inner parse call having recorded an error, which may not always happen (e.g., if the expression just ends unexpectedly).

**Recommendation**: Add a fallback error:
```go
if right == nil {
    p.addErrorWithCode(ErrMissingOperand,
        fmt.Sprintf("missing right operand for '%s'", opToken.Lit),
        "binary operators require operands on both sides")
    return nil
}
```

### MINOR (Nice to Have)

#### 7. SpanError.Error() Format Could Include Position (errors.go:40-46)

**File**: `pkg/parser/errors.go`
**Lines**: 40-46

```go
func (e SpanError) Error() string {
    msg := e.Message
    if e.Hint != "" {
        msg += "\n  Hint: " + e.Hint
    }
    return msg
}
```

The `Error()` method doesn't include line:column information, which would be useful for CLI output.

**Recommendation**: Include position when available:
```go
func (e SpanError) Error() string {
    msg := e.Message
    if e.Line > 0 {
        msg = fmt.Sprintf("%d:%d: %s", e.Line, e.Column, msg)
    }
    if e.Hint != "" {
        msg += "\n  Hint: " + e.Hint
    }
    return msg
}
```

#### 8. questionKind and lambdaClassification Could Be Exported (lookahead.go)

**File**: `pkg/parser/lookahead.go`

The `questionKind` and `lambdaClassification` types are unexported but have `String()` methods, suggesting they're useful for debugging. Consider exporting them for test assertions and debugging output.

#### 9. Incomplete Float Literal Parsing (pratt.go:301-304)

**File**: `pkg/parser/pratt.go`
**Lines**: 301-304

```go
func (p *PrattParser) parseFloatLiteral() ast.Expr {
    // TODO: Return proper ast.BasicLit node
    return nil
}
```

This TODO has been here and returns `nil`, which will cause issues if float literals are used.

**Recommendation**: Implement similarly to `parseIntegerLiteral()`:
```go
func (p *PrattParser) parseFloatLiteral() ast.Expr {
    return &ast.RawExpr{
        StartPos: p.curToken.Pos,
        EndPos:   p.curToken.End,
        Text:     p.curToken.Lit,
    }
}
```

#### 10. Duplicate Error Code Constants (errors.go:12-23)

**File**: `pkg/parser/errors.go`

The error codes E001-E010 are well-defined, but `TestErrorCodes` in `errors_test.go` manually lists them instead of using reflection. If a new code is added and forgotten in the test, the uniqueness check won't catch it.

---

## Questions

1. **Performance Measurement**: Have you benchmarked the decision table approach vs. the previous 106-line implementation? The decision table is cleaner but involves more function calls.

2. **LSP Integration Plan**: Will `SpanError` be the type returned to the LSP server, or will it be converted to `protocol.Diagnostic`? The current design seems ready for direct mapping.

3. **Error Recovery Strategy**: The `SyncSet` definitions exist but `synchronize()` isn't used in the main parsing paths. Is this intentional, or planned for a future error recovery pass?

4. **classifyTSLambda vs isTypeScriptLambda**: Both functions exist with overlapping functionality. Should `classifyTSLambda` replace `isTypeScriptLambda` for consistency?

---

## Summary

**Overall Assessment**: CHANGES_NEEDED

The implementation represents a solid architectural improvement over the previous approach. The decision table pattern for question operator disambiguation is maintainable and well-documented. The error infrastructure is LSP-ready with proper span tracking.

However, there are two critical issues:
1. The sync.Pool optimization code is dead (never called)
2. If used, the pool has a potential race condition due to state not being cleared

These should be addressed before merging.

**Priority Ranking**:
1. CRITICAL: Remove or use the pooled state functions
2. CRITICAL: Fix race condition in releaseState if pool is kept
3. IMPORTANT: Add chained question operator integration test
4. IMPORTANT: Consider increasing maxLookahead limit
5. IMPORTANT: Add fallback error in parseBinaryExpr
6. MINOR: Implement parseFloatLiteral
7. MINOR: Include position in SpanError.Error()

**Testability Score**: Medium-High

- Unit tests for classification logic are comprehensive
- Benchmarks cover all critical paths
- Missing: integration tests for edge cases (chained operators, large ternaries)
- Missing: concurrent parsing tests (relevant if pool is used)

---

**Files Modified by Review**: None (review only)

**Review Duration**: Full analysis
