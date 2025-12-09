# Context-Aware Preprocessing: Implementation Strategies

## Decision Context

**Previous findings**:
- Consensus: Keep current regex preprocessor + go/parser architecture
- Decision: NO to adding separate syntax tree parser layer
- Rationale: Current architecture handles 97.8% tests, avoid complexity

**New challenge**: How do we implement context-aware features (like pattern matching) within this architecture?

## Current Architecture (Proven & Working)

```
.dingo file
    ↓
┌─────────────────────────────────────────┐
│ Stage 1: Regex Preprocessor             │
├─────────────────────────────────────────┤
│ • Type annotations: param: Type → ...  │
│ • Error propagation: x? → ...          │
│ • Enums: enum Name {} → ...            │
│ • Simple text transforms               │
└─────────────────────────────────────────┘
    ↓ (Valid Go syntax)
┌─────────────────────────────────────────┐
│ Stage 2: go/parser + AST Processing    │
├─────────────────────────────────────────┤
│ • Parse with native go/parser          │
│ • Plugin pipeline (Discovery/Transform)│
│ • AST metadata & context tracking      │
│ • Generate .go + .sourcemap            │
└─────────────────────────────────────────┘
```

## The Question

**How can we add context-aware capabilities to this architecture WITHOUT adding a separate parser layer?**

Context-aware = Understanding scope, variable bindings, type information, nesting levels, etc.

## Use Cases Requiring Context

### 1. Pattern Matching

```dingo
match result {
    Ok(value) => processValue(value),    // Need to know 'value' scope
    Err(e) => handleError(e)             // Need to know 'e' scope
}
```

**Context needs**:
- Match expression structure and nesting
- Variable binding scopes (value, e)
- Exhaustiveness checking (all cases covered?)
- Type information (what's being matched?)

### 2. Advanced Lambda Closures

```dingo
fn makeCounter() -> fn() -> int {
    let count = 0
    return || {                          // Closure captures 'count'
        count += 1
        return count
    }
}
```

**Context needs**:
- Variable capture analysis (what's captured?)
- Closure scope boundaries
- Mutable vs immutable captures

### 3. Generic Type Inference

```dingo
let result = fetchData()                 // Type inferred from return
result.map(|x| x * 2)?                   // Generic chain, needs context
```

**Context needs**:
- Type flow through method chains
- Generic parameter inference
- Error propagation context

## Potential Strategies

### Strategy A: Enhanced Regex with Markers

**Idea**: Regex preprocessor emits marker comments that Stage 2 uses for context.

```go
// Preprocessor output:
/* DINGO_MATCH_START scope=result type=Result[T,E] */
switch __match_discriminant_0 := result.(type) {
    /* DINGO_MATCH_ARM pattern=Ok(value) binding=value */
    case ResultOk:
        value := __match_discriminant_0.Value
        /* DINGO_SCOPE_START var=value */
        processValue(value)
        /* DINGO_SCOPE_END */
    /* DINGO_MATCH_ARM pattern=Err(e) binding=e */
    case ResultErr:
        e := __match_discriminant_0.Error
        /* DINGO_SCOPE_START var=e */
        handleError(e)
        /* DINGO_SCOPE_END */
}
/* DINGO_MATCH_END */
```

**Stage 2 plugin** reads markers, builds context map, validates exhaustiveness.

**Questions**:
- Can markers provide enough context?
- How to handle nested constructs?
- Performance impact?

### Strategy B: Multi-pass Preprocessing

**Idea**: Preprocessor makes multiple passes, building context incrementally.

**Pass 1**: Identify all Dingo constructs, mark positions
**Pass 2**: Build scope tree and binding map
**Pass 3**: Transform with context awareness
**Pass 4**: Emit valid Go with metadata comments

**Questions**:
- How many passes needed?
- State management between passes?
- Complexity vs benefit?

### Strategy C: Leverage go/parser AST in Preprocessor

**Idea**: After regex transform, parse with go/parser, walk AST for context, then finish transform.

```go
// Preprocessor flow:
1. Initial regex transform (Dingo → Go-like)
2. Parse with go/parser → AST
3. Walk AST, build context map (scopes, types via go/types)
4. Use context to refine transform
5. Emit final Go code
```

**Questions**:
- Can preprocessor use go/parser before final output?
- Does this blur Stage 1/Stage 2 boundary?
- Performance implications?

### Strategy D: Metadata Sidecar Files

**Idea**: Preprocessor emits .dingo.meta file alongside preprocessed Go.

```json
// example.dingo.meta
{
  "matchExpressions": [
    {
      "line": 42,
      "variable": "result",
      "type": "Result[User, DbError]",
      "arms": [
        {"pattern": "Ok(user)", "bindings": ["user"]},
        {"pattern": "Err(e)", "bindings": ["e"]}
      ]
    }
  ],
  "scopes": [ /* ... */ ]
}
```

**Stage 2 plugin** reads .meta file for context-aware validations.

**Questions**:
- File management complexity?
- How to keep .meta synchronized with code?
- Worth the indirection?

### Strategy E: Enhanced AST Plugin Capabilities

**Idea**: Stage 2 plugins get richer context from go/parser + go/types.

```go
// Plugin API enhancement:
type ContextAwarePlugin interface {
    Transform(node ast.Node, ctx *TransformContext) ast.Node
}

type TransformContext struct {
    TypeInfo    *types.Info        // Type information
    ScopeStack  []*types.Scope     // Scope hierarchy
    Bindings    map[string]Type    // Variable bindings
    Metadata    map[string]any     // Custom metadata
}
```

**Preprocessor emits minimal markers**, Stage 2 does heavy lifting via AST.

**Questions**:
- Can go/types provide what we need?
- How to correlate Dingo constructs with Go AST?
- Is this sufficient for all use cases?

### Strategy F: Hybrid Markers + AST Metadata

**Idea**: Combine lightweight markers (Strategy A) with rich AST plugins (Strategy E).

**Preprocessor**:
- Emits minimal markers for complex constructs
- Keeps transforms simple (regex-based)

**Stage 2 AST Plugin**:
- Reads markers via AST comments
- Uses go/types for type information
- Builds context map on-the-fly
- Validates + transforms with full context

**Questions**:
- Best of both worlds?
- Implementation complexity?
- Clear separation of concerns?

## Your Mission

Investigate and recommend **how to implement context-aware preprocessing** within the current architecture.

### Questions to Answer

#### 1. Which Strategy is Best?

Evaluate each strategy (A-F) and any others you can think of:
- **Feasibility**: Can it actually work? Technical blockers?
- **Complexity**: Implementation effort (person-weeks)
- **Performance**: Overhead added (milliseconds)
- **Maintainability**: Long-term maintenance burden
- **Extensibility**: Can it handle future complex features?

#### 2. Concrete Implementation Plan

For your recommended strategy, provide:

**Architecture**:
- Detailed data flow (Dingo → markers/metadata → AST → Go)
- What happens in Stage 1 (regex preprocessor)?
- What happens in Stage 2 (go/parser + plugins)?
- How does context information flow?

**Code Examples**:
```go
// Show actual Go code examples:
// 1. Preprocessor marker emission
// 2. AST plugin reading context
// 3. Context validation logic
```

**Edge Cases**:
- Nested match expressions
- Closures capturing match-bound variables
- Generic type inference across match arms
- Error handling in complex contexts

#### 3. Pattern Matching Proof of Concept

**Provide a concrete implementation sketch** for pattern matching:

**Input (Dingo)**:
```dingo
match getUserById(id) {
    Ok(user) => {
        println("Found: {}", user.name)
        return user
    }
    Err(NotFound) => {
        println("User not found")
        return defaultUser()
    }
    Err(e) => {
        println("Error: {}", e)
        return defaultUser()
    }
}
```

**Show**:
1. What regex preprocessor emits (with markers)
2. What go/parser sees (valid Go AST)
3. What plugin does (context extraction + validation)
4. Final generated Go code

#### 4. Context Tracking Mechanism

**How do we track context** (scopes, bindings, types)?

Options:
- **AST comments** (/* DINGO_* */ markers)
- **Metadata files** (.dingo.meta sidecar)
- **In-memory structures** (built during AST walk)
- **AST node metadata** (custom ast.Node fields)
- **Combination** of above

**Recommend one** and justify.

#### 5. go/types Integration

**Can we leverage go/types** for context?

```go
// Example: Type inference for pattern matching
import "go/types"

func inferMatchType(expr ast.Expr, typeInfo *types.Info) types.Type {
    // Can go/types tell us the type of 'result' in match expression?
    // Can it infer generic parameters?
    // Limitations?
}
```

**Investigate**:
- What context can go/types provide?
- What can't it provide (Dingo-specific constructs)?
- How to integrate with preprocessor output?

#### 6. Performance Analysis

**Estimate performance impact**:

```
Baseline (current):
  - Regex preprocessing: ~5ms per 1000 LOC
  - AST processing: ~10ms per 1000 LOC
  - Total: ~15ms per 1000 LOC

With context-aware enhancements:
  - Regex + markers: ~? ms
  - AST + context building: ~? ms
  - Context validation: ~? ms
  - Total: ~? ms per 1000 LOC

Acceptable if: <50ms per 1000 LOC (3x baseline)
```

#### 7. Migration Path

**How to roll this out**?

**Phase 1** (Week 1-2):
- [ ] Implement marker system
- [ ] Basic context tracking
- [ ] Simple pattern matching support

**Phase 2** (Week 3-4):
- [ ] Full go/types integration
- [ ] Nested construct support
- [ ] Exhaustiveness checking

**Phase 3** (Week 5-6):
- [ ] Advanced features (closures, generics)
- [ ] Performance optimization
- [ ] Comprehensive testing

#### 8. Comparison with Precedents

**How do other transpilers handle this?**

- **TypeScript**: Context-aware how? AST metadata?
- **Babel**: Plugin context mechanism?
- **Rust macros**: Procedural macros with TokenStream context?
- **Scala macros**: Compile-time context access?

**Learn from their approaches.**

## Constraints

- Must work within current regex + go/parser architecture
- No separate syntax tree parser layer
- Performance: <50ms per 1000 LOC acceptable
- Timeline: 3-6 weeks implementation acceptable
- Must handle pattern matching, closures, type inference

## Deliverables

Provide **comprehensive implementation guide**:

1. ✅ **Recommended Strategy** (A-F or hybrid)
2. 🏗️ **Architecture Design** with data flow diagrams
3. 💻 **Code Examples** (preprocessor, markers, AST plugins)
4. 🔬 **Pattern Matching POC** (complete example)
5. 📊 **Performance Estimates** (concrete numbers)
6. 🗺️ **Migration Plan** (phased rollout)
7. ⚠️ **Risks and Mitigations**
8. 🎯 **Success Metrics** (how to validate it works)

**Be thorough. Provide actionable, concrete guidance we can implement immediately.**
