# Dingo Features Index

This document provides a comprehensive overview of all planned features for the Dingo language, organized by priority, complexity, and implementation status.

**Last Updated:** 2025-12-06
**Phase:** Phase 10 - Token-Based Parser with Pluggable Features
**Status Source of Truth:** See CLAUDE.md for current implementation phase
**Architecture:** Token-based parser (`pkg/goparser/`) + Pluggable Feature System (`pkg/feature/`)
**Configuration:** Features can be enabled/disabled via `dingo.toml` `[feature_matrix]` section
**Philosophy:** As a meta-language, Dingo can implement features Go rejected, as long as they transpile cleanly

---

## Priority Legend

- **P0** - Critical (Core language features, must-have for MVP)
- **P1** - High (Essential for production use, high community demand)
- **P2** - Medium (Important quality-of-life improvements)
- **P3** - Lower (Nice-to-have, user choice features)
- **P4** - Lowest (Advanced/specialized, specific use cases)
- **P5** - Future (Experimental, post-1.0 consideration)

## Complexity Legend

- **🟢 Low** - Simple syntax transformation, 1-2 weeks, minimal type system impact
- **🟡 Medium** - Moderate compiler logic, 2-3 weeks, standard patterns
- **🟠 High** - Complex type system changes, 3-4 weeks, advanced algorithms
- **🔴 Very High** - Fundamental changes, 4+ weeks, research-level features

## Status Legend

- 🔴 **Not Started** - No implementation yet
- 🟡 **In Design** - Active design/proposal phase
- 🟢 **In Development** - Implementation in progress
- ✅ **Designed** - Architecture/design complete, ready for implementation
- ✅ **Implemented** - Feature complete
- ⏸️ **On Hold** - Postponed pending other features

---

## Pluggable Feature System

**All language features are implemented as plugins** that can be enabled/disabled via configuration.

### Configuration (dingo.toml)

```toml
[feature_matrix]
# All features enabled by default
# Only specify features you want to disable:

# Character-level features
enum = true              # enum declarations
match = true             # match expressions
enum_constructors = true # Variant() → NewVariant()
error_prop = true        # ? operator
guard_let = true         # guard let expressions
safe_nav_statements = true
safe_nav = false         # Disable ?. operator
null_coalesce = false    # Disable ?? operator
lambdas = true           # |x| and => syntax

# Token-level features
type_annotations = true  # x: Type syntax
generics = true          # <T> syntax
let_binding = true       # let keyword
```

### Plugin Priority Order

Features execute in a fixed priority order to ensure correct transformation:

| Priority | Plugin | Type | Depends On |
|----------|--------|------|------------|
| 10 | `enum` | Character | - |
| 20 | `match` | Character | `enum` |
| 30 | `enum_constructors` | Character | `enum` |
| 40 | `error_prop` | Character | - |
| 50 | `guard_let` | Character | `error_prop` |
| 55 | `safe_nav_statements` | Character | - |
| 60 | `safe_nav` | Character | - |
| 70 | `null_coalesce` | Character | `safe_nav` |
| 80 | `lambdas` | Character | - |
| 100 | `type_annotations` | Token | - |
| 110 | `generics` | Token | - |
| 120 | `let_binding` | Token | - |

### Disabled Feature Errors

When a disabled feature's syntax is used, the transpiler reports:

```
error: feature 'lambdas' is disabled in configuration
  --> src/main.dingo:10:5
   |
10 |     let add = |x, y| x + y
   |               ^^^^^^^^^^^^
   |
   = help: enable 'lambdas' in dingo.toml [feature_matrix] section
```

### Future: 3rd-Party Plugins (v1.1+)

The architecture supports 3rd-party plugins via RPC (HashiCorp go-plugin):

```toml
# Future dingo.toml syntax
[[plugins]]
name = "pipe_operator"
path = "~/.dingo/plugins/dingo-pipe"
enabled = true
```

---

## Feature Matrix

### Infrastructure & Architecture

| Priority | Feature | Complexity | Timeline | Community Demand | Status | File |
|----------|---------|------------|----------|------------------|--------|------|
| **ARCH** | Type Annotations (`param: Type`) | 🟢 Low | 1 week | ⭐⭐⭐⭐⭐ | ✅ Implemented | Token-level plugin |
| **ARCH** | Generic Syntax (`<T>` → `[T]`) | 🟢 Low | 1 week | ⭐⭐⭐⭐⭐ | ✅ Implemented | Token-level plugin |
| **ARCH** | Keywords (`let`, `use`) | 🟢 Low | 1 week | ⭐⭐⭐⭐⭐ | ✅ Implemented | Token-level plugin |
| **ARCH** | Source Maps | 🟡 Medium | 2 weeks | ⭐⭐⭐⭐⭐ | ✅ Implemented | TokenMapping |
| **ARCH** | Workspace Builds | 🟡 Medium | 2 weeks | ⭐⭐⭐⭐⭐ | ✅ Implemented | Multi-package |
| **ARCH** | Token-Based Parser | 🟠 High | 5-6 weeks | ⭐⭐⭐⭐⭐ | ✅ Implemented (Phase 10) | `pkg/goparser/` |
| **ARCH** | **Pluggable Features** | 🟡 Medium | 1 week | ⭐⭐⭐⭐⭐ | ✅ Implemented | `pkg/feature/` |
| **ARCH** | File Organization | 🟡 Medium | 4 weeks | ⭐⭐⭐⭐⭐ | ✅ Designed | [file-organization.md](./file-organization.md) |

### Language Features

| Priority | Feature | Complexity | Timeline | Community Demand | Status | File |
|----------|---------|------------|----------|------------------|--------|------|
| **P0** | Result Type | 🟡 Medium | 2-3 weeks | ⭐⭐⭐⭐⭐ (#1 issue) | ✅ Implemented | [result-type.md](./result-type.md) |
| **P0** | Error Propagation (`?`) | 🟢 Low | 1-2 weeks | ⭐⭐⭐⭐⭐ | ✅ Implemented (Phase 10) | [error-propagation.md](./error-propagation.md) |
| **P0** | Option Type | 🟡 Medium | 2-3 weeks | ⭐⭐⭐⭐⭐ | ✅ Implemented | [option-type.md](./option-type.md) |
| **P0** | Pattern Matching | 🟠 High | 3-4 weeks | ⭐⭐⭐⭐⭐ | ✅ Implemented (Phase 10) | [pattern-matching.md](./pattern-matching.md) |
| **P0** | Sum Types | 🟠 High | 3-4 weeks | ⭐⭐⭐⭐⭐ (996+ 👍) | ✅ Implemented (Phase 10) | [sum-types.md](./sum-types.md) |
| **P1** | Type-Safe Enums | 🟡 Medium | 1-2 weeks | ⭐⭐⭐⭐⭐ (900+ 👍) | ✅ Implemented (Phase 10) | [enums.md](./enums.md) |
| **P1** | Lambda/Arrow Functions | 🟡 Medium | 2-3 weeks | ⭐⭐⭐⭐ (750+ 👍) | ✅ Implemented (Phase 10) | [lambdas.md](./lambdas.md) |
| **P1** | Null Safety (`?.`) | 🟡 Medium | 2 weeks | ⭐⭐⭐⭐ | ⚠️ Partial (marker-based) | [null-safety.md](./null-safety.md) |
| **P2** | Functional Utilities | 🟢 Low | 1 week | ⭐⭐⭐ | ✅ Implemented | [functional-utilities.md](./functional-utilities.md) |
| **P2** | Tuples | 🟡 Medium | 1-2 weeks | ⭐⭐⭐ | ✅ Implemented | [tuples.md](./tuples.md) |
| **P2** | Null Coalescing (`??`) | 🟢 Low | 2-3 days | ⭐⭐⭐ | ⚠️ Partial (marker-based) | [null-coalescing.md](./null-coalescing.md) |
| **P2** | Immutability | 🔴 Very High | 4+ weeks | ⭐⭐⭐ | 🔴 Not Started | [immutability.md](./immutability.md) |
| **P3** | Ternary Operator | 🟢 Low | 2-3 days | ⭐⭐ | ✅ Implemented | [ternary-operator.md](./ternary-operator.md) |
| **P3** | Guard Let | 🟡 Medium | 1 week | ⭐⭐⭐ | ✅ Implemented (Phase 10) | [guard_let.md](./guard_let.md) |
| **P3** | Default Parameters | 🟡 Medium | 2 weeks | ⭐⭐ | 🔴 Not Started | [default-parameters.md](./default-parameters.md) |
| **P4** | Function Overloading | 🟠 High | 3 weeks | ⭐⭐ | 🔴 Not Started | [function-overloading.md](./function-overloading.md) |
| **P4** | Operator Overloading | 🟡 Medium | 2 weeks | ⭐⭐ | 🔴 Not Started | [operator-overloading.md](./operator-overloading.md) |

---

## Detailed Complexity Analysis

### 🟢 Low Complexity Features (1-2 weeks each)

**Error Propagation (`?`)** - 1-2 weeks
- Simple AST transformation: `expr?` → `if err != nil { return err }`
- Requires: Basic type checking (verify Result type)
- Transpilation: Straightforward code generation
- Risk: Low - proven pattern from Rust

**Null Coalescing (`??`)** - 2-3 days
- Pure syntax sugar: `a ?? b` → `a.unwrapOr(b)`
- No type system changes needed
- Transpilation: Trivial rewrite
- Risk: Very low - simple operator

**Ternary Operator (`? :`)** - 2-3 days
- Expression form of if/else: `cond ? a : b` → `if cond { a } else { b }`
- Type checking: Both branches must have same type
- Transpilation: Direct translation
- Risk: Very low - well-understood feature

**Functional Utilities** - 1 week
- Library functions that transpile to loops
- `slice.map(f)` → `for i, v := range slice { result = append(result, f(v)) }`
- No language changes, just standard library
- Risk: Low - straightforward implementation

### 🟡 Medium Complexity Features (2-3 weeks each)

**Result Type** - 2-3 weeks
- Define generic enum: `enum Result<T, E> { Ok(T), Err(E) }`
- Transpiles to struct with tag + union
- Requires: Pattern matching integration, methods (map, unwrap, etc.)
- Risk: Medium - depends on sum types being solid

**Option Type** - 2-3 weeks
- Similar to Result: `enum Option<T> { Some(T), None }`
- Transpiles to `*T` with validation
- Requires: Pattern matching, nil coalescing support
- Risk: Medium - similar to Result

**Enums** - 1-2 weeks
- Simpler than sum types (no associated values for basic enums)
- Transpiles to Go's iota pattern + validation
- Add exhaustiveness checking in match
- Risk: Low-Medium - well-understood pattern

**Lambdas** - 2-3 weeks
- Parse: `|a, b| expr` or `{ it.field }`
- Type inference from context
- Transpiles to `func(a T, b U) R { return expr }`
- Risk: Medium - closure capture, type inference edge cases

**Null Safety (`?.`)** - 2 weeks
- Chain nil checks: `a?.b?.c` → nested if checks
- Returns Option<T>
- Requires: Option type, type inference
- Risk: Medium - complex chaining edge cases

**Tuples** - 1-2 weeks
- Anonymous structs: `(int, string)` → `struct { f0 int; f1 string }`
- Destructuring support
- Transpiles cleanly to Go
- Risk: Low-Medium - straightforward

**Default Parameters** - 2 weeks
- Two strategies: (1) Generate multiple function variants, or (2) Use options struct
- Type checking for default value compatibility
- Transpilation: Generate all variants
- Risk: Medium - interaction with overloading if both exist

**Operator Overloading** - 2 weeks
- Parse operator as method: `a + b` → `a.Add(b)`
- Define trait/interface for each operator
- Transpiles to method calls
- Risk: Medium - precedence, associativity rules

### 🟠 High Complexity Features (3-4 weeks each)

**Sum Types** - 3-4 weeks
- Foundational type system feature
- Memory layout optimization (tag + union)
- Interaction with interfaces, generics
- Exhaustiveness tracking in type checker
- Risk: High - impacts entire type system

**Pattern Matching** - 3-4 weeks
- Exhaustiveness checking algorithm (compute case coverage)
- Destructuring patterns (nested, guards)
- Type narrowing in each branch
- Generate efficient switch code
- Risk: High - complex algorithm, many edge cases

**Function Overloading** - 3 weeks
- Name resolution: Pick best function based on argument types
- Name mangling for Go output: `func_int_string`
- Interaction with generics, default params
- Type inference complications
- Risk: High - complex type resolution, potential ambiguity

### 🔴 Very High Complexity Features (4+ weeks)

**Immutability** - 4+ weeks
- Flow analysis to track const propagation
- "Const poisoning" - immutability spreads through call graph
- Interaction with generics, methods
- Verify no mutable operations on const values
- Risk: Very high - research-level problem, affects entire codebase

---

## Reconsideration: Why "Rejected" Features Are Worth Building

### Dingo's Meta-Language Advantage

**Key Insight:** Go team rejected features for Go's philosophy. Dingo is a transpiler - we can add features that compile to clean Go without changing Go itself.

### Previously "Rejected" Features - Reconsidered

#### Ternary Operator (Now P3)

**Go's Reasoning:** "Language needs only one conditional construct"
**Dingo's Counter-Argument:**
- ✅ Transpiles trivially to if/else expression
- ✅ Users who want concise code get it
- ✅ Users who prefer explicit if/else can avoid it
- ✅ Extremely common in other languages (C, Java, JS, Python)
- ✅ Zero runtime cost

**Decision:** P3 - Let developers choose their style

#### Default Parameters (Now P3)

**Go's Reasoning:** "Leads to API bloat and confusion"
**Dingo's Counter-Argument:**
- ✅ Can transpile to multiple function variants with name suffixes
- ✅ Or transpile to options struct pattern
- ✅ Very common in Swift, Kotlin, Python - developers expect it
- ✅ Reduces boilerplate for common parameter patterns
- ✅ Type-safe (defaults must match parameter type)

**Decision:** P3 - Useful for API design, transpiles cleanly

#### Function Overloading (Now P4)

**Go's Reasoning:** "Adds complexity to name resolution"
**Dingo's Counter-Argument:**
- ✅ Transpile with name mangling: `Print(int)` → `Print_int`
- ✅ Generics don't cover all use cases (different behavior per type)
- ✅ Common in Java, C++, Kotlin - developers expect it
- ✅ Type-safe resolution (no ambiguity with strict rules)
- ✅ Can be powerful with generics: `func<T> process(T)` + `func process(string)` special case

**Decision:** P4 - Advanced feature, but transpilation is feasible

#### Operator Overloading (Now P4)

**Go's Reasoning:** "Magic, reduces readability"
**Dingo's Counter-Argument:**
- ✅ Transpiles cleanly to method calls: `a + b` → `a.Add(b)`
- ✅ Essential for DSLs, matrix math, BigDecimal, scientific computing
- ✅ Common in Rust, C++, Swift - developers in those domains expect it
- ✅ Can be restricted (e.g., only for math types, not IO)
- ✅ Generated Go code is explicit method calls (readable)

**Decision:** P4 - Useful for specific domains (math/science), transpiles cleanly

---

## Implementation Roadmap (Updated)

### Phase 1: Core Error Handling (MVP) - 8-10 weeks

**Critical Path:**
1. Sum Types (3-4 weeks) - Foundation for Result/Option
2. Result Type (2-3 weeks) - Depends on sum types
3. Option Type (2-3 weeks) - Depends on sum types
4. Pattern Matching (3-4 weeks) - Needed for ergonomic Result/Option usage
5. Error Propagation (1-2 weeks) - Sugar on top of Result

**Parallel Work:**
- Enums (1-2 weeks) - Can start immediately
- Null Coalescing (2-3 days) - Simple, can do anytime

**Target:** First usable Dingo transpiler that solves Go's #1 pain point

### Phase 2: Type Safety & Ergonomics - 6-8 weeks

**Goals:**
1. Null Safety operators (2 weeks)
2. Lambdas (2-3 weeks)
3. Functional Utilities (1 week)
4. Tuples (1-2 weeks)
5. Ternary Operator (2-3 days)

**Target:** Production-ready with modern language ergonomics

### Phase 3: Advanced Type System - 4-6 weeks

**Goals:**
1. Immutability (4+ weeks) - Most complex feature
2. Default Parameters (2 weeks)

**Target:** Feature parity with Swift/Kotlin for safety

### Phase 4: Power User Features - 5-6 weeks

**Goals:**
1. Function Overloading (3 weeks)
2. Operator Overloading (2 weeks)

**Target:** Support specialized domains (math, DSLs, etc.)

### Phase 5: Future Exploration

**Ideas to explore:**
- Async/await (Go has goroutines, but sugar could help)
- Macros/metaprogramming
- Algebraic effects
- Refinement types
- Dependent types (very advanced)

---

## Complexity vs Impact Analysis

### High Impact, Low Complexity (DO FIRST) ⭐⭐⭐⭐⭐

- **Error Propagation (`?`)** - Huge developer impact, trivial to implement
- **Null Coalescing (`??`)** - Common need, 3 days to build
- **Functional Utilities** - Popular request, straightforward
- **Ternary Operator** - Widely wanted, trivial complexity

### High Impact, Medium Complexity (CORE FEATURES) ⭐⭐⭐⭐

- **Result Type** - Solves #1 Go pain point
- **Option Type** - Eliminates nil pointer panics
- **Enums** - 900+ community upvotes
- **Lambdas** - 750+ upvotes, big ergonomic win
- **Null Safety** - Prevents common bugs

### High Impact, High Complexity (INVEST HERE) ⭐⭐⭐⭐

- **Sum Types** - 996+ upvotes, foundational
- **Pattern Matching** - Essential for sum types, huge win

### Medium Impact, Medium Complexity (NICE TO HAVE) ⭐⭐⭐

- **Tuples** - Convenient for small data
- **Default Parameters** - Reduces function variant boilerplate
- **Operator Overloading** - Great for math/science users

### Medium Impact, High Complexity (CONSIDER CAREFULLY) ⭐⭐

- **Function Overloading** - Useful but adds complexity
- **Immutability** - Powerful but very hard

### Lower Impact, Low Complexity (EASY WINS) ⭐

- **Ternary Operator** - User preference feature

---

## Risk Assessment

### Low Risk Features (Safe to implement immediately)
- Error Propagation, Null Coalescing, Ternary, Functional Utilities
- **Risk:** Minimal - simple transformations, well-understood

### Medium Risk Features (Require careful design)
- Result, Option, Enums, Lambdas, Null Safety, Tuples
- **Risk:** Moderate - standard patterns, need good testing

### High Risk Features (Require prototyping)
- Sum Types, Pattern Matching, Function Overloading
- **Risk:** Significant - complex algorithms, edge cases

### Very High Risk Features (Research needed)
- Immutability
- **Risk:** Very high - may hit fundamental limitations

---

## Success Metrics per Feature

### P0 Features (Must achieve 90%+ of goals)
- [ ] Result type works in 100% of Go error cases
- [ ] `?` operator reduces error handling by 60%+
- [ ] Pattern matching has 0 false positives in exhaustiveness
- [ ] Sum types have ≤5% memory overhead vs hand-written Go

### P1 Features (Must achieve 80%+ of goals)
- [ ] Enums prevent 100% of invalid values at compile time
- [ ] Lambdas reduce callback code by 50%+
- [ ] Null safety prevents 95%+ of nil panics at compile time

### P2-P4 Features (Must achieve 70%+ of goals)
- [ ] Each feature has clear use cases where it shines
- [ ] Transpiled code remains readable
- [ ] No performance regression vs hand-written Go

---

## Community Engagement Strategy

### Get Feedback On:
1. **Ternary Operator** - Do Dingo users want this?
2. **Default Parameters** - Which transpilation strategy is better?
3. **Function/Operator Overloading** - Are these worth the complexity?
4. **Immutability** - Is 4+ weeks of work justified?

### Decision Framework:
- Prototype controversial features
- Measure transpiled code quality
- Survey potential users
- Make data-driven decisions

---

## References

### Research Documents
- [ai-docs/research/golang_missing/chatgpt.md](../ai-docs/research/golang_missing/chatgpt.md) - 200+ proposals analyzed
- [ai-docs/research/golang_missing/claud.md](../ai-docs/research/golang_missing/claud.md) - Comprehensive analysis
- [ai-docs/research/golang_missing/gemini.md](../ai-docs/research/golang_missing/gemini.md) - Philosophy conflicts
- [ai-docs/research/golang_missing/grok.md](../ai-docs/research/golang_missing/grok.md) - Top 10 features
- [ai-docs/research/golang_missing/kimi.md](../ai-docs/research/golang_missing/kimi.md) - Community survey data

### External References
- Go Proposals: 996+ upvotes on #19412 (sum types)
- Rust Book: Zero-cost abstractions, ownership
- Swift Guide: Optional chaining, enums with associated values
- Kotlin Docs: When expressions, sealed classes, null safety
- TypeScript Handbook: Discriminated unions, conditional types

---

## Conclusion: Embrace Dingo's Meta-Language Advantage

**Core Philosophy Shift:**

Dingo is NOT bound by Go's philosophy. We can:
- ✅ Add syntax that Go rejected (ternary, default params)
- ✅ Implement features Go won't (sum types, operator overloading)
- ✅ Provide options Go doesn't (immutability, overloading)

**As long as:**
- ✅ Transpiled Go code is clean and idiomatic
- ✅ No runtime overhead (zero-cost abstractions)
- ✅ Full compatibility with Go ecosystem
- ✅ Features are opt-in (don't force users to use them)

**Result:** Dingo becomes "Go with all the features you wanted but couldn't have"

---

**Next Steps:**
1. Prioritize P0 features for Phase 1 implementation
2. Prototype controversial P3-P4 features to validate transpilation
3. Create detailed RFCs for each feature
4. Build MVP transpiler with Result/Option/`?`/match
5. Gather community feedback on priorities
