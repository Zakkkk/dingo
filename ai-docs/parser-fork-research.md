# Parser Enhancement Research: Forking Go's Parser for Dingo

**Date:** January 2026
**Status:** Research Complete

## Executive Summary

**Conclusion: Forking go/parser is NOT recommended.** The research reveals that Dingo's current architecture (custom parser → transpiler → gopls validation) is the proven pattern used by TypeScript, Babel, and Sucrase.

**Recommended Path:** Phased enhancement of the current Pratt parser with:
1. Go-style error recovery patterns (panic mode + sync sets)
2. Span-based error messages with hints
3. Refactored disambiguation logic
4. Consider Participle for complex features (pure Go, declarative grammar)

---

## Requirements
- Better error messages (human-readable with hints)
- Better performance
- Better maintainability
- Pure Go only (no C dependencies - Tree-sitter excluded)

---

## Research Findings

### 1. Go's Parser Architecture

Go uses a **hand-written recursive descent parser** (~2,900 lines) with:
- Pratt parsing for operator precedence (binary expressions)
- 1-token lookahead (no backtracking)
- Panic mode error recovery with token synchronization
- Position tracking via `token.FileSet` (survives reformatting)
- BSD-licensed (permissive)

**Key Files:**
```
go/token/token.go    → 57 token types
go/scanner/scanner.go → Tokenizer
go/parser/parser.go  → Main parser (~75KB)
go/ast/ast.go        → AST node definitions
```

### 2. Dingo's Current Parser

The Pratt parser (~3,500 lines across files) handles 8+ Dingo-specific features:

| Feature | Complexity | Current Implementation |
|---------|------------|------------------------|
| Error propagation (`?`) | High | 120+ lines, lookahead/backtrack for disambiguation |
| Pattern matching (`match`) | High | 350+ lines recursive descent |
| Lambdas (2 styles) | High | 150+ lines lookahead to distinguish from grouped expr |
| Safe navigation (`?.`) | Medium | 65 lines, Phase 3 limitations |
| Null coalesce (`??`) | Low | 33 lines |
| Ternary (`? :`) | High | Integrated with error prop disambiguation |
| Enums | Medium | 470 lines |
| Tuples | Medium | 90 lines disambiguation |

**Known Limitations:**
- Float literal parsing returns nil (TODO)
- Method chaining after safe nav deferred to Phase 3
- Heavy lookahead/backtracking for `?` disambiguation

### 3. Why Forking go/parser is NOT Recommended

| Concern | Impact |
|---------|--------|
| **Maintenance burden** | Each Go release requires merge/update work |
| **Deep coupling** | Must also fork go/token, go/ast, go/scanner |
| **AST extension** | Adding Dingo AST nodes means maintaining parallel AST package |
| **Error recovery tuning** | Go's recovery is tuned for Go semantics |
| **No precedent** | Research found NO successful go/parser forks |

**The fork trap:** Even if initially working, every Go 1.x release could break compatibility.

---

## Alternative Approaches

### Option A: Enhance Current Parser (Recommended)

Keep Dingo's current architecture but improve:
1. **Better error messages** - Add context-aware hints
2. **Reduce backtracking** - Refactor `?` disambiguation with cleaner lookahead
3. **Fix known TODOs** - Float literals, Phase 3 safe nav methods

**Effort:** Low-Medium
**Risk:** Low
**Benefit:** Incremental improvement, no architectural change

### Option B: Parser Generator (Pigeon/Participle)

Replace hand-written parser with generated one:
- **Pigeon (PEG)**: Define grammar in PEG syntax → generates Go code
- **Participle**: Struct-tag based grammar, familiar to Go devs

**Advantages:**
- Grammar specification is declarative
- Automatic error recovery
- Easier maintenance

**Disadvantages:**
- PEG: Potential exponential time in pathological cases
- Participle: No left recursion support
- Learning curve for team

**Effort:** High (rewrite parser)
**Risk:** Medium
**Benefit:** Cleaner grammar, easier to extend

### Option C: Tree-sitter (Excluded - C Dependency)

Fork tree-sitter-go grammar and extend with Dingo syntax:
- Grammar in JavaScript DSL
- Incremental parsing (great for LSP)
- Go bindings available

**Excluded because:** Adds C dependency

### Option D: Hybrid Approach

Use different parsers for different purposes:
- **Transpilation**: Keep Pratt parser (or Participle rewrite)
- **LSP**: Use Tree-sitter for incremental parsing

**Effort:** Very High
**Risk:** High (maintaining two parsers)
**Benefit:** Best of both worlds

---

## Error Message Enhancement Strategy

Regardless of parser choice, better error messages require:

### 1. Context-Aware Error Detection
```go
// Current: "unexpected token"
// Better: "unexpected '?' - did you mean error propagation (expr?) or ternary (cond ? a : b)?"
```

### 2. Span-Based Errors
```go
type ParseError struct {
    Pos     token.Pos
    EndPos  token.Pos  // NEW: highlight range
    Message string
    Hint    string     // NEW: suggestion
    Code    string     // NEW: error code for docs
}
```

### 3. Recovery Hints
```go
// Current: stops at error
// Better: "near 'match x {', expected pattern => expression"
```

---

## Recommended Approach: Phased Enhancement

Given requirements (all three goals + pure Go), here's the recommended path:

### Phase 1: Error Infrastructure

**Goal:** Better error messages without architectural change

**Files to modify:**
- `pkg/parser/pratt.go` - Add span-based errors
- `pkg/parser/recovery.go` - Enhanced recovery
- New: `pkg/parser/errors.go` - Centralized error types

**Implementation:**
```go
// Enhanced error type with spans and hints
type ParseError struct {
    Pos      token.Pos
    EndPos   token.Pos  // Highlight range
    Message  string
    Hint     string     // "did you mean..."
    Code     string     // E001 for docs
    Context  string     // "in match expression"
}

// Sync sets for recovery (borrowed from Go)
var stmtStart = map[TokenType]bool{
    FUNC: true, VAR: true, CONST: true, TYPE: true,
    IF: true, FOR: true, MATCH: true, RETURN: true,
}
```

**Error message improvements:**
| Current | Improved |
|---------|----------|
| "unexpected token" | "unexpected '?' at line 5 - did you mean error propagation (expr?) or ternary (cond ? a : b)?" |
| "expected expression" | "expected expression after '=>' in match arm (line 10, col 15)" |

### Phase 2: Disambiguation Refactor

**Goal:** Cleaner code for the complex `?` operator handling

**Current problem:** `parseQuestionOperator()` is 120+ lines with multiple backtrack attempts

**Solution:** Structured lookahead with decision table
```go
// Cleaner disambiguation
func (p *Parser) classifyQuestionOperator() questionKind {
    // Look ahead without consuming
    switch p.peek() {
    case STRING, PIPE, LPAREN:
        if p.isLambdaStart() { return qkErrorTransform }
        return qkErrorContext
    case IDENT, NUMBER, ...:
        if p.hasTernaryColon() { return qkTernary }
        return qkErrorProp
    default:
        return qkErrorProp
    }
}
```

**Files to modify:**
- `pkg/parser/pratt.go` - Refactor `parseQuestionOperator()`
- New: `pkg/parser/lookahead.go` - Structured lookahead utilities

### Phase 3: Participle Evaluation

**Goal:** Evaluate Participle for complex sub-parsers

**Why Participle:**
- Pure Go (no C)
- Declarative struct-tag grammar
- Built-in error recovery
- Proven in production (Kong, Chroma, many others)

**Candidate features for Participle rewrite:**
- Pattern matching grammar (currently 350+ lines)
- Enum declarations (currently 470 lines)
- Lambda syntax (currently 537 lines)

**Example Participle grammar:**
```go
type MatchExpr struct {
    Pos   lexer.Position
    Value *Expression `"match" @@`
    Arms  []*MatchArm `"{" @@* "}"`
}

type MatchArm struct {
    Pattern *Pattern    `@@`
    Guard   *Expression `("if" @@)?`
    Body    *Expression `"=>" @@`
}
```

**Decision point:** After Phase 1-2, evaluate if Participle's benefits outweigh migration cost.

### Phase 4: Performance Optimization (Optional)

**Goal:** Profile and optimize hot paths

**Potential optimizations:**
- Pre-allocated token buffers
- Reduced allocations in lookahead
- Smarter synchronization (less backtracking)

**Metrics to track:**
- Parse time for large files (benchmark suite)
- Memory allocations per parse
- LSP response latency

---

## Files Overview

| File | Current Lines | Role |
|------|---------------|------|
| `pkg/parser/pratt.go` | ~1,100 | Core expression parser |
| `pkg/parser/lambda.go` | ~540 | Lambda parsing |
| `pkg/parser/match.go` | ~660 | Pattern matching |
| `pkg/parser/enum.go` | ~470 | Enum declarations |
| `pkg/parser/recovery.go` | ~100 | Error recovery |
| `pkg/tokenizer/tokens.go` | ~300 | Token definitions |

---

## Success Criteria

1. **Error messages:** Users can understand and fix errors without docs
2. **Performance:** No regression (measure with benchmarks first)
3. **Maintainability:** New syntax features take <50 lines to add
4. **Pure Go:** No C dependencies introduced

---

---

## Participle Deep-Dive Evaluation

**Date:** January 2026
**Conclusion:** NOT recommended for Dingo

### Why Participle Doesn't Fit

| Feature | Participle Approach | Dingo's Need | Verdict |
|---------|---------------------|--------------|---------|
| Operator precedence | Grammar restructuring | 11 levels, mixed associativity | ❌ Pratt better |
| Left recursion | Not supported | Expression parsing | ❌ Pratt handles |
| Token disambiguation | Stateful lexer | `?` vs `?.` vs `??` | ❌ Already solved |
| Lookahead | LL(k), configurable | 10+ patterns for lambdas | ❌ Complex |
| Error messages | Backtracking limits | Human-readable hints | ⚠️ Needs work |

### The `?` Operator Problem

Dingo's `?` has **5 different interpretations**:

```go
// 1. Error propagation (postfix)
result := fetchData()?

// 2. Error with context string
result := fetchData() ? "failed to fetch"

// 3. Error with Rust-style lambda transform
result := fetchData() ? |e| fmt.Errorf("wrap: %w", e)

// 4. Error with TypeScript-style lambda transform
result := fetchData() ? (e) => fmt.Errorf("wrap: %w", e)

// 5. Ternary conditional
value := condition ? trueVal : falseVal
```

**Disambiguation requires:**
- Check if next token is terminator (`;`, `)`, `}`) → postfix error prop
- Check if next is STRING without following `:` → error with context
- Check if next is `|` → Rust lambda transform
- Check if next starts TypeScript lambda → TS lambda transform
- Otherwise try ternary, backtrack if no `:` found

This is **120+ lines of explicit lookahead logic** in Dingo's current parser. Participle would require:
- Custom lexer with modal states
- Grammar restructuring to encode precedence
- Significant lookahead configuration

### TypeScript Lambda Detection

The `isTypeScriptLambda()` function must distinguish:

```go
(x)           // grouped expression
(x) => expr   // single-param lambda
(x, y) => expr // multi-param lambda
(x: int) => expr // typed param lambda
(): int => expr  // empty params with return type
```

This requires **10 different lookahead patterns** scanning until `=>` is found or ruled out.

### What Dingo Already Has

1. **Custom tokenizer** (`pkg/tokenizer/`)
   - Distinguishes `?`, `?.`, `??` at lexing time
   - Position tracking via `token.FileSet`
   - Multi-character operator handling

2. **Pratt parser** (`pkg/parser/pratt.go`)
   - Native operator precedence (11 levels)
   - Prefix/infix function maps
   - Clean precedence climbing

3. **Explicit disambiguation** (`parseQuestionOperator()`)
   - State save/restore for backtracking
   - Clear decision tree
   - Handles all 5 `?` interpretations

### Participle's Strengths (Not Applicable Here)

Participle excels at:
- Config files (INI, TOML, HCL)
- Protocol definitions (Protobuf, Thrift)
- Simple DSLs without complex operator precedence
- Declarative grammars where struct-tags are readable

Dingo is an **expression-heavy language** with:
- Complex operator interactions
- Context-sensitive disambiguation
- Multiple syntax styles (Rust + TypeScript lambdas)

### Recommendation

**Keep current architecture.** Enhance error messages within Pratt parser rather than switching to Participle.

If future refactoring is needed:
- Consider Participle only for **isolated sub-grammars** (enum declarations, pattern syntax)
- Keep Pratt for all expression parsing
- Never use Participle for the `?` operator disambiguation

---

## References

- [Go parser source](https://go.dev/src/go/parser/parser.go)
- [Participle - Parser library for Go](https://github.com/alecthomas/participle)
- [Participle tutorial](https://github.com/alecthomas/participle/blob/master/TUTORIAL.md)
- [Pigeon - PEG parser generator](https://github.com/PigeonsLLC/pigeon)
- [Tree-sitter](https://tree-sitter.github.io/) (excluded due to C dependency)
- [Sucrase - Fast TypeScript/JSX transpiler](https://github.com/alangpierce/sucrase)
- [Pratt Parsers: Expression Parsing Made Easy](https://matklad.github.io/2020/04/13/simple-but-powerful-pratt-parsing.html)
