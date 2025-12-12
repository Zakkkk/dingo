# Architectural Plan: Next Development Stage
**Session:** 20251116-174148
**Date:** 2025-11-16
**Phase:** Phase 1 - Core Transpiler (Error Handling Foundation)

---

## Executive Summary

Based on analysis of the current codebase, the next logical development stage is **implementing the Error Propagation (`?`) operator** as the foundation for Dingo's error handling capabilities. This is the optimal starting point because:

1. **Minimal Complexity** - Simplest P0 feature (1-2 weeks vs 3-4 weeks for sum types)
2. **Immediate Value** - Provides 60% code reduction in error handling immediately
3. **Learning Foundation** - Establishes patterns for AST transformation that all other features will reuse
4. **Plugin Architecture Validation** - First real test of the newly built plugin system
5. **Strategic Sequencing** - Proves the transpiler works before tackling complex features like Result/Option types

---

## Current State Analysis

### What We Have (Completed)
- ✅ **Basic Transpiler Pipeline** - Dingo → Go compilation working
- ✅ **CLI Tools** - `dingo build`, `dingo run`, `dingo version`
- ✅ **Plugin System Architecture** - Complete modular framework with dependency resolution
- ✅ **Hybrid AST Strategy** - go/ast extensions for Dingo-specific nodes
- ✅ **Parser Interface** - Participle-based parser with clean abstractions
- ✅ **Beautiful Terminal UI** - Professional developer experience
- ✅ **Code Generator** - go/printer integration with formatting

### What We Need (Gaps)
- ❌ **No Feature Implementations** - Plugin system exists but no transformation plugins
- ❌ **AST Nodes Defined But Unused** - `ErrorPropagationExpr` exists in ast.go but not parsed
- ❌ **No Parser Extensions** - Participle parser doesn't recognize `?` operator
- ❌ **No Source Maps** - Error messages point to generated .go files, not .dingo
- ❌ **No Type Checking** - Can't verify `?` operator is used on Result types
- ❌ **No Result/Option Types** - No foundation types to propagate errors from

---

## Recommended Next Stage: Error Propagation (`?`) Operator

### Why This Feature First?

**Strategic Reasoning:**

1. **Simplest P0 Feature**
   - Only requires AST transformation: `expr?` → `if err != nil { return err }`
   - No complex type system changes needed initially
   - Can work with Go's native `(T, error)` tuples before Result type exists

2. **Plugin System Validation**
   - First real test of the newly built plugin architecture
   - Proves transformation pipeline works end-to-end
   - Establishes patterns for all future feature plugins

3. **Learning Foundation**
   - Teaches team AST manipulation patterns
   - Establishes testing methodology for transformations
   - Creates reference implementation for future features

4. **Immediate User Value**
   - Works with existing Go code immediately
   - Reduces error handling boilerplate by ~60%
   - Demonstrates Dingo's value proposition quickly

5. **Enables Incremental Development**
   - Can enhance with Result types later
   - Doesn't block other features
   - Provides early user feedback

**Alternative Rejected:** Starting with Result type (3-4 weeks, requires sum types) would delay user value and complicate the learning curve.

---

## Architecture Design

### Component Breakdown

```
Phase 1.5: Error Propagation Foundation
├── 1. Parser Extension (3-4 days)
│   ├── Extend participle grammar for ? operator
│   ├── Create ErrorPropagationExpr nodes
│   └── Add operator precedence rules
│
├── 2. Transformation Plugin (5-7 days)
│   ├── Implement ErrorPropagationPlugin
│   ├── Transform expr? → if err != nil { return err }
│   ├── Handle tuple unpacking
│   └── Generate idiomatic error checks
│
├── 3. Basic Type Analysis (3-4 days)
│   ├── Verify ? used on (T, error) returns
│   ├── Simple type inference for error position
│   └── Validation error messages
│
└── 4. Testing & Polish (2-3 days)
    ├── Golden file tests
    ├── Real-world examples
    └── Documentation
```

**Total Timeline:** 2-3 weeks for MVP implementation

---

## Detailed Implementation Plan

### Phase 1: Parser Extension (Days 1-4)

**Goal:** Teach parser to recognize `expr?` syntax

**Tasks:**

1. **Update Participle Grammar** (`pkg/parser/participle.go`)
   ```go
   type PostfixExpr struct {
       Primary ast.Expr
       Ops     []PostfixOp
   }

   type PostfixOp struct {
       Question *QuestionOp `@@?`
       Call     *CallOp     `| @@?`
       // ... other postfix ops
   }
   ```

2. **AST Node Creation**
   - `ErrorPropagationExpr` already defined in `pkg/ast/ast.go`
   - Ensure parser creates these nodes correctly
   - Add position tracking for error messages

3. **Operator Precedence**
   - `?` has highest precedence (postfix)
   - Binds tighter than binary operators
   - Examples: `foo()?.bar` vs `(foo()?).bar`

4. **Parser Tests**
   ```go
   // Test cases
   - "fetchUser(id)?"
   - "db.Query(sql)?.Scan(&user)?"
   - "complex.chain()?.method()?.field?"
   - Invalid: "5?" (not a function call)
   ```

**Deliverables:**
- Updated parser that recognizes `?` operator
- AST contains `ErrorPropagationExpr` nodes
- Comprehensive parser tests (>90% coverage)

---

### Phase 2: Transformation Plugin (Days 5-11)

**Goal:** Convert `ErrorPropagationExpr` to Go error handling code

**Architecture:**

```go
// pkg/plugin/builtin/error_propagation.go
package builtin

type ErrorPropagationPlugin struct {
    plugin.BasePlugin
    tempVarCounter int
}

func (p *ErrorPropagationPlugin) Transform(ctx *plugin.Context, node ast.Node) (ast.Node, error) {
    // Match ErrorPropagationExpr nodes
    if expr, ok := node.(*dingoast.ErrorPropagationExpr); ok {
        return p.transformErrorPropagation(ctx, expr)
    }
    return node, nil
}

func (p *ErrorPropagationPlugin) transformErrorPropagation(
    ctx *plugin.Context,
    expr *dingoast.ErrorPropagationExpr,
) (ast.Node, error) {
    // Transform: fetchUser(id)?
    // To:       __tmp0, __err0 := fetchUser(id)
    //           if __err0 != nil { return nil, __err0 }
    //           user := __tmp0
}
```

**Transformation Strategy:**

**Input Dingo:**
```dingo
func processUser(id: string) (User, error) {
    user := fetchUser(id)?
    validated := validateUser(user)?
    return save(validated)
}
```

**Output Go:**
```go
func processUser(id string) (User, error) {
    __tmp0, __err0 := fetchUser(id)
    if __err0 != nil {
        return User{}, __err0  // Zero value + error
    }
    user := __tmp0

    __tmp1, __err1 := validateUser(user)
    if __err1 != nil {
        return User{}, __err1
    }
    validated := __tmp1

    return save(validated)
}
```

**Key Implementation Details:**

1. **Tuple Unpacking**
   - Detect return type is `(T, error)`
   - Generate temp variables: `__tmp{N}`, `__err{N}`
   - Maintain counter for unique names

2. **Error Return Generation**
   - Analyze function signature for return types
   - Generate zero values for non-error returns
   - Preserve error wrapping if present

3. **Statement vs Expression Context**
   - In assignment: `x := f()?` → temp vars + check + assign
   - In return: `return f()?` → call + immediate check
   - In nested: `g(f()?)` → extract to statement first

4. **Position Preservation**
   - Track original `.dingo` positions
   - Map to generated `.go` positions
   - Foundation for source maps (Phase 2)

**Deliverables:**
- Working ErrorPropagationPlugin
- Transformation tests with golden files
- Handles all context variations (assign, return, nested)

---

### Phase 3: Basic Type Analysis (Days 12-15)

**Goal:** Validate `?` operator used correctly

**Type Checking Strategy:**

```go
// pkg/typechecker/error_propagation.go
package typechecker

func ValidateErrorPropagation(expr *dingoast.ErrorPropagationExpr) error {
    // 1. Check that X returns a tuple
    typ := inferType(expr.X)

    // 2. Verify it's (T, error) shape
    tuple, ok := typ.(*types.Tuple)
    if !ok || tuple.Len() != 2 {
        return fmt.Errorf("? operator requires (T, error) return")
    }

    // 3. Verify second element implements error interface
    errorType := tuple.At(1).Type()
    if !implements(errorType, errorInterface) {
        return fmt.Errorf("second return value must be error type")
    }

    return nil
}
```

**Implementation Approach:**

Since we don't have full type inference yet, use **simplified validation**:

1. **Heuristic Checking**
   - Verify `?` is used on function call or method call
   - Warn if used on non-call expressions
   - Defer full type checking to later phase

2. **Runtime Assumption**
   - Trust developer for now
   - Generate code that will fail at Go compile time if wrong
   - Go compiler becomes final validator

3. **Error Messages**
   - Clear, actionable errors pointing to `.dingo` source
   - Examples of correct usage
   - Link to documentation

**Deliverables:**
- Basic validation (expression type checking)
- Clear error messages for misuse
- Foundation for future full type checker

---

### Phase 4: Testing & Polish (Days 16-18)

**Goal:** Production-ready feature with excellent UX

**Testing Strategy:**

1. **Golden File Tests**
   ```
   tests/error_propagation/
   ├── simple.dingo          → simple.go
   ├── nested.dingo          → nested.go
   ├── chained.dingo         → chained.go
   ├── real_world_http.dingo → real_world_http.go
   └── real_world_db.dingo   → real_world_db.go
   ```

2. **Unit Tests**
   - Parser recognizes all `?` variations
   - Plugin generates correct Go code
   - Type checker catches errors
   - Position tracking works

3. **Integration Tests**
   - Full pipeline: parse → transform → generate
   - Verify generated Go compiles
   - Verify generated Go runs correctly
   - Performance benchmarks

4. **Real-World Examples**
   ```dingo
   // examples/error_propagation/
   // - http_client.dingo
   // - database.dingo
   // - file_operations.dingo
   ```

**Documentation:**

1. **User Guide** (`docs/features/error-propagation.md`)
   - What is `?` operator
   - How to use it
   - Common patterns
   - Migration from Go

2. **Implementation Guide** (`ai-docs/error-propagation-impl.md`)
   - Architecture decisions
   - Transformation algorithm
   - Testing strategy
   - Future enhancements

**Deliverables:**
- Comprehensive test suite (>90% coverage)
- Real-world examples that compile and run
- User and developer documentation
- Performance benchmarks

---

## Package Structure

```
dingo/
├── cmd/dingo/              # CLI (no changes needed)
│
├── pkg/
│   ├── ast/
│   │   └── ast.go          # ErrorPropagationExpr already defined
│   │
│   ├── parser/
│   │   ├── parser.go       # Interface (no changes)
│   │   └── participle.go   # ✨ ADD: ? operator parsing
│   │
│   ├── plugin/
│   │   ├── plugin.go       # Registry (exists)
│   │   ├── pipeline.go     # Pipeline (exists)
│   │   └── builtin/        # ✨ NEW: Built-in plugins
│   │       └── error_propagation.go
│   │
│   ├── typechecker/        # ✨ NEW: Type validation
│   │   └── validator.go
│   │
│   └── generator/
│       └── generator.go    # Already has plugin support
│
├── tests/
│   └── error_propagation/  # ✨ NEW: Golden file tests
│
└── examples/
    └── error_propagation/  # ✨ NEW: Real-world examples
```

---

## Dependencies & Prerequisites

### Technical Dependencies

1. **Existing Systems (Ready)**
   - ✅ Plugin architecture (just built)
   - ✅ AST definitions (ErrorPropagationExpr exists)
   - ✅ Parser interface (extensible)
   - ✅ Generator with plugin support

2. **New Dependencies (Minimal)**
   - Basic type inference (simplified, not full type checker)
   - Go's `go/types` package for validation
   - Testing framework for golden files

### Knowledge Dependencies

1. **Go AST Manipulation**
   - Creating if statements programmatically
   - Generating variable declarations
   - Position tracking with token.Pos

2. **Parser Extension**
   - Participle grammar syntax
   - Operator precedence rules
   - AST node creation

3. **Testing Strategies**
   - Golden file testing patterns
   - AST comparison techniques
   - Integration test setup

---

## Risk Analysis

### Technical Risks

| Risk | Probability | Impact | Mitigation |
|------|------------|--------|------------|
| **Parser complexity with operator precedence** | Medium | Medium | Start with simple postfix parsing, add precedence incrementally |
| **Statement vs expression context handling** | Medium | High | Build comprehensive test cases upfront, handle each context separately |
| **Type inference without full type checker** | High | Medium | Use heuristic validation, rely on Go compiler as final check |
| **Generated code quality** | Low | High | Extensive golden file tests, manual review of output |
| **Plugin system bugs** | Medium | Medium | This is first real plugin - expect iteration, good testing |

### Schedule Risks

| Risk | Probability | Impact | Mitigation |
|------|------------|--------|------------|
| **Underestimated parser complexity** | Medium | Low | Parser is simple for postfix `?`, have buffer in timeline |
| **Transformation edge cases** | High | Medium | Budget extra time for edge cases, start with common cases |
| **Testing reveals fundamental issues** | Low | High | Early spike/prototype to validate approach |

### Mitigation Strategies

1. **Early Prototype (Day 1-2)**
   - Build minimal working version
   - Validate transformation approach
   - Identify blockers early

2. **Incremental Development**
   - Start with simplest case: `x := f()?`
   - Add complexity gradually
   - Test each increment

3. **Fallback Plan**
   - If full implementation blocked, ship simplified version
   - Example: Only support assignment context initially
   - Iterate based on user feedback

---

## Success Metrics

### Phase 1.5 Success Criteria

- [ ] Parser recognizes `expr?` syntax in all valid contexts
- [ ] Plugin transforms `?` to correct Go error handling code
- [ ] Generated Go code compiles and runs correctly
- [ ] 90%+ test coverage on parser and plugin
- [ ] 10+ golden file test cases covering edge cases
- [ ] 3+ real-world examples demonstrate value
- [ ] Documentation explains feature clearly
- [ ] Error messages are actionable and helpful
- [ ] Transpiled code is readable (looks hand-written)
- [ ] Zero performance regression vs manual error handling

### User Value Metrics

- [ ] Reduces error handling code by 50-60% in examples
- [ ] Works with existing Go standard library
- [ ] No runtime overhead vs manual `if err != nil`
- [ ] Positive feedback from 3-5 early testers
- [ ] Can demonstrate value in 5-minute demo

---

## Future Enhancements (Post-Phase 1.5)

### Phase 2: Result Type Integration

Once `?` operator works with Go tuples, enhance for Result types:

```dingo
func fetchUser(id: string) -> Result[User, Error] {
    // Implementation
}

func processUser(id: string) -> Result[User, Error] {
    user := fetchUser(id)?  // Now unwraps Result type
    return Ok(user)
}
```

**Changes Required:**
- Type checker recognizes Result[T, E]
- Plugin handles Result unwrapping
- Different code generation strategy

### Phase 3: Source Maps

Add bidirectional position mapping:

```json
{
  "version": 3,
  "file": "main.go",
  "sourceRoot": "",
  "sources": ["main.dingo"],
  "mappings": "AAAA,SAAS..."
}
```

### Phase 4: Error Context

Automatic error wrapping:

```dingo
user := fetchUser(id)? wrap "failed to fetch user ${id}"
```

Generates:
```go
if __err0 != nil {
    return User{}, fmt.Errorf("failed to fetch user %s: %w", id, __err0)
}
```

---

## Alternative Approaches Considered

### Alternative 1: Start with Result Type

**Pros:**
- More complete error handling solution
- Aligns with final vision

**Cons:**
- 3-4 weeks vs 2-3 weeks
- Requires sum types foundation
- More complex, higher risk of delays
- Delays user value

**Decision:** Rejected - too much complexity upfront

### Alternative 2: Implement All P0 Features Together

**Pros:**
- Complete error handling story
- No incremental integrations

**Cons:**
- 8-12 weeks before any user value
- High risk of scope creep
- Difficult to validate each piece
- No early feedback

**Decision:** Rejected - violates incremental development principle

### Alternative 3: Build Full Type Checker First

**Pros:**
- Perfect type safety
- Catches all errors at compile time

**Cons:**
- Weeks of work before any features
- Over-engineering for current needs
- Go compiler provides fallback validation

**Decision:** Rejected - YAGNI, build what we need when we need it

---

## Open Questions & Decisions Needed

### Critical Decisions (Block Implementation)

1. **Operator Syntax**
   - Option A: `expr?` (Rust-style, postfix)
   - Option B: `expr!` (Swift-style try!)
   - Option C: `try expr` (prefix keyword)
   - **Recommendation:** Option A (`?`) - proven in Rust, concise, unambiguous
   - **User Input Needed:** Confirm syntax preference

2. **Error Return Strategy**
   - Option A: Return zero value + error: `return T{}, err`
   - Option B: Return nil + error: `return nil, err` (only for pointers)
   - Option C: Configurable via pragma
   - **Recommendation:** Option A - safest, works for all types
   - **User Input Needed:** Confirm approach

3. **Nested Expression Handling**
   - Option A: Extract all `?` to statements (safest, verbose)
   - Option B: Allow inline for simple cases (complex, risky)
   - **Recommendation:** Option A for MVP, Option B later
   - **User Input Needed:** Confirm strategy

### Non-Blocking Questions (Can Decide During Implementation)

4. **Temp Variable Naming**
   - `__tmp0, __err0` vs `_v0, _e0` vs `__dingo_tmp_0`
   - Decision: Choose during implementation, non-critical

5. **Error Message Format**
   - Level of detail in validation errors
   - Decision: Iterate based on user feedback

6. **Performance Optimizations**
   - Inline vs function call for simple cases
   - Decision: Measure first, optimize later

---

## Conclusion

**Recommendation: Implement Error Propagation (`?`) Operator as Phase 1.5**

This approach:
- ✅ Validates plugin architecture with real feature
- ✅ Delivers immediate user value in 2-3 weeks
- ✅ Establishes patterns for all future features
- ✅ Proves Dingo's value proposition quickly
- ✅ Enables incremental development of Result types
- ✅ Minimizes risk through simplicity
- ✅ Provides early user feedback

**Next Steps:**
1. Review this plan with user
2. Get confirmation on critical decisions
3. Create detailed task breakdown
4. Begin Day 1 implementation (parser spike)

**Estimated Timeline:** 2-3 weeks to production-ready feature

---

## Appendix A: Example Transformations

### Example 1: Simple Assignment

**Input:**
```dingo
user := fetchUser(id)?
```

**Output:**
```go
__tmp0, __err0 := fetchUser(id)
if __err0 != nil {
    return User{}, __err0
}
user := __tmp0
```

### Example 2: Chained Calls

**Input:**
```dingo
result := step1()?.step2()?.step3()?
```

**Output:**
```go
__tmp0, __err0 := step1()
if __err0 != nil {
    return Result{}, __err0
}
__tmp1 := __tmp0

__tmp2, __err1 := __tmp1.step2()
if __err1 != nil {
    return Result{}, __err1
}
__tmp3 := __tmp2

__tmp4, __err2 := __tmp3.step3()
if __err2 != nil {
    return Result{}, __err2
}
result := __tmp4
```

### Example 3: Real-World HTTP Handler

**Input:**
```dingo
func handleUser(w: http.ResponseWriter, r: http.Request) {
    id := r.URL.Query().Get("id")
    user := fetchUser(id)?
    validated := validateUser(user)?
    saved := saveUser(validated)?

    json.NewEncoder(w).Encode(saved)
}
```

**Output:**
```go
func handleUser(w http.ResponseWriter, r http.Request) {
    id := r.URL.Query().Get("id")

    __tmp0, __err0 := fetchUser(id)
    if __err0 != nil {
        http.Error(w, __err0.Error(), 500)
        return
    }
    user := __tmp0

    __tmp1, __err1 := validateUser(user)
    if __err1 != nil {
        http.Error(w, __err1.Error(), 400)
        return
    }
    validated := __tmp1

    __tmp2, __err2 := saveUser(validated)
    if __err2 != nil {
        http.Error(w, __err2.Error(), 500)
        return
    }
    saved := __tmp2

    json.NewEncoder(w).Encode(saved)
}
```

---

**Document Version:** 1.0
**Author:** Claude (Dingo AI Architect)
**Status:** Pending Review
