# Functional Utilities Architecture Plan
## Session: 20251117-003406

**Date:** 2025-11-17
**Author:** Claude Code (Sonnet 4.5)
**Status:** Draft

---

## Executive Summary

This plan outlines the architecture for implementing functional utilities (map, filter, reduce) in Dingo's standard library. The design leverages Go 1.18+ generics to provide type-safe, zero-cost abstractions that transpile to idiomatic Go code. The implementation will be developed in a git worktree to avoid conflicts with parallel development sessions.

**Key Decisions:**
- Use Go generics (not code generation) for type-safe functional utilities
- Create a new `stdlib` package for Dingo standard library utilities
- Integrate deeply with Dingo's Result[T, E] and Option[T] types
- Support method-style chaining syntax via slice wrappers
- Generate clean, readable Go code with explicit loops (zero runtime overhead)

---

## 1. Problem Summary

### User Requirements
Implement three core functional utilities:
1. **Map** - Transform each element in a collection
2. **Filter** - Select elements matching a predicate
3. **Reduce** - Aggregate collection into a single value

### Constraints
- Must work in parallel with ongoing development (use git worktree)
- No backward compatibility needed (clean slate)
- Must generate idiomatic Go code
- Zero runtime overhead
- Full integration with Dingo's type system (Result, Option, lambdas)

### Context
- Dingo currently has: sum types, pattern matching, error propagation (?)
- Lambda syntax is NOT yet implemented (planned P1 feature)
- Plugin architecture is established and working
- Go 1.25.4 with generics available

---

## 2. Recommended Approach

### High-Level Strategy

**Phase 1: Standard Library Package Structure**
Create a new `stdlib` package that contains functional utilities implemented as Go generic functions. These will be transpiled from Dingo's method-style syntax.

**Phase 2: Parser Extensions**
Extend the participle parser to recognize method-style calls on slices (`.map()`, `.filter()`, `.reduce()`).

**Phase 3: Plugin Implementation**
Create a new `functional_utilities` plugin that transforms method-style calls into stdlib function calls.

**Phase 4: Lambda Integration (Future)**
Once lambda syntax is implemented, seamlessly integrate with functional utilities.

### Why Go Generics Over Code Generation?

**Pros of Go Generics:**
- Type-safe at compile time
- Clean, readable generated code
- No code duplication
- Works with go tooling (gopls, go vet, etc.)
- Standard Go convention since 1.18

**Pros of Code Generation:**
- Could optimize specific types
- More control over generated code

**Decision: Use Go Generics**
- Go 1.25.4 is available (excellent generics support)
- Aligns with Go ecosystem conventions
- Simpler implementation
- Better developer experience in generated code

### Integration with Dingo Type System

```dingo
// Map with Result types
let users: []User = getUsers()
let results: []Result[Profile, Error] = users.map(|u| fetchProfile(u.id))

// Filter with Option types
let names: []Option[string] = users.map(|u| u.name)
let validNames: []string = names.filterSome()  // Filter out None values

// Reduce with error propagation
let total: Result[int, Error] = numbers.mapTry(|n| parseNumber(n)).sum()
```

---

## 3. Package Structure

### Directory Layout

```
dingo/
├── pkg/
│   ├── stdlib/                    # NEW: Dingo standard library
│   │   ├── slice.go               # Generic slice utilities
│   │   ├── slice_result.go        # Result-specific utilities
│   │   ├── slice_option.go        # Option-specific utilities
│   │   ├── slice_test.go          # Comprehensive tests
│   │   └── doc.go                 # Package documentation
│   │
│   ├── plugin/
│   │   └── builtin/
│   │       ├── functional_utils.go  # NEW: Functional utilities plugin
│   │       └── functional_utils_test.go
│   │
│   ├── parser/
│   │   └── participle.go          # MODIFY: Add method call syntax
│   │
│   └── ast/
│       └── ast.go                 # MODIFY: Add MethodCallExpr if needed
│
├── features/
│   └── functional-utilities.md    # EXISTS: Feature documentation
│
└── tests/
    └── golden/
        ├── 20_map_simple.go.golden        # NEW: Golden tests
        ├── 21_filter_chaining.go.golden
        ├── 22_reduce_result.go.golden
        └── 23_complex_pipeline.go.golden
```

### Package Responsibilities

#### `pkg/stdlib/` - Dingo Standard Library

**Purpose:** Contain reusable Go generic functions for functional operations.

**Key Interfaces:**
```go
// Core slice operations
func Map[T, U any](slice []T, fn func(T) U) []U
func Filter[T any](slice []T, predicate func(T) bool) []T
func Reduce[T, U any](slice []T, init U, reducer func(U, T) U) U

// Result-aware operations
func MapResult[T, U, E any](slice []T, fn func(T) Result[U, E]) Result[[]U, E]
func FilterOk[T, E any](slice []Result[T, E]) []T

// Option-aware operations
func MapOption[T, U any](slice []T, fn func(T) Option[U]) []Option[U]
func FilterSome[T any](slice []Option[T]) []T

// Chainable wrapper (future enhancement)
type Slice[T any] struct { inner []T }
func (s Slice[T]) Map[U any](fn func(T) U) Slice[U]
func (s Slice[T]) Filter(predicate func(T) bool) Slice[T]
```

**Why in `pkg/` not `internal/`?**
- Will be imported by generated Go code
- Needs to be public for Go compilation
- Part of Dingo's runtime contract (even though we claim "zero runtime")

#### `pkg/plugin/builtin/functional_utils.go` - Transformation Plugin

**Purpose:** Transform Dingo method-style syntax to stdlib function calls.

**Transformation Strategy:**
```dingo
// INPUT (Dingo syntax)
let doubled = numbers.map(|x| x * 2)

// INTERMEDIATE (AST representation)
MethodCallExpr{
  Receiver: Ident("numbers"),
  Method: "map",
  Args: [LambdaExpr{...}]
}

// OUTPUT (Generated Go)
var doubled []int
doubled = stdlib.Map(numbers, func(x int) int {
    return x * 2
})
```

**Plugin Dependencies:**
- None initially (lambdas not yet implemented)
- Future: Will depend on `lambda` plugin when available

---

## 4. Key Interfaces and Types

### Stdlib Core API

```go
// pkg/stdlib/slice.go

package stdlib

// Map transforms each element in a slice using the provided function.
// Returns a new slice with transformed elements.
//
// Example:
//   numbers := []int{1, 2, 3}
//   doubled := Map(numbers, func(x int) int { return x * 2 })
//   // doubled = []int{2, 4, 6}
func Map[T, U any](slice []T, fn func(T) U) []U {
    if slice == nil {
        return nil
    }
    result := make([]U, len(slice))
    for i, v := range slice {
        result[i] = fn(v)
    }
    return result
}

// Filter returns a new slice containing only elements that satisfy the predicate.
//
// Example:
//   numbers := []int{1, 2, 3, 4}
//   evens := Filter(numbers, func(x int) bool { return x % 2 == 0 })
//   // evens = []int{2, 4}
func Filter[T any](slice []T, predicate func(T) bool) []T {
    if slice == nil {
        return nil
    }
    result := make([]T, 0, len(slice))
    for _, v := range slice {
        if predicate(v) {
            result = append(result, v)
        }
    }
    return result
}

// Reduce aggregates a slice into a single value using the reducer function.
//
// Example:
//   numbers := []int{1, 2, 3, 4}
//   sum := Reduce(numbers, 0, func(acc, x int) int { return acc + x })
//   // sum = 10
func Reduce[T, U any](slice []T, init U, reducer func(U, T) U) U {
    result := init
    for _, v := range slice {
        result = reducer(result, v)
    }
    return result
}
```

### Result/Option Integration

```go
// pkg/stdlib/slice_result.go

// MapResult applies a function that returns Result to each element.
// Returns Ok with all values if all succeeded, or first Err encountered.
//
// This is similar to Rust's Iterator::collect() for Result types.
//
// Example:
//   numbers := []string{"1", "2", "invalid"}
//   result := MapResult(numbers, parseNumber)
//   // result = Err("parse error")
func MapResult[T, U, E any](slice []T, fn func(T) Result[U, E]) Result[[]U, E] {
    if slice == nil {
        return Ok[[]U, E](nil)
    }

    results := make([]U, 0, len(slice))
    for _, v := range slice {
        r := fn(v)
        if r.IsErr() {
            return Err[[]U, E](r.UnwrapErr())
        }
        results = append(results, r.Unwrap())
    }
    return Ok[[]U, E](results)
}

// FilterOk extracts successful values from a slice of Results.
// Discards all Err values.
//
// Example:
//   results := []Result[int, Error]{Ok(1), Err(e), Ok(3)}
//   values := FilterOk(results)
//   // values = []int{1, 3}
func FilterOk[T, E any](slice []Result[T, E]) []T {
    if slice == nil {
        return nil
    }

    results := make([]T, 0, len(slice))
    for _, r := range slice {
        if r.IsOk() {
            results = append(results, r.Unwrap())
        }
    }
    return results
}
```

### AST Extensions (if needed)

```go
// pkg/ast/ast.go

// MethodCallExpr represents a method-style call on a value
// Example: numbers.map(|x| x * 2)
//
// Note: Go doesn't have method calls on primitives/slices, so this
// is Dingo-specific syntax that transpiles to function calls.
type MethodCallExpr struct {
    X      ast.Expr      // Receiver (e.g., numbers)
    Sel    *ast.Ident    // Method name (e.g., "map")
    Lparen token.Pos     // Position of '('
    Args   []ast.Expr    // Arguments
    Rparen token.Pos     // Position of ')'
}

func (m *MethodCallExpr) Pos() token.Pos { return m.X.Pos() }
func (m *MethodCallExpr) End() token.Pos { return m.Rparen + 1 }
func (*MethodCallExpr) exprNode() {}
```

**Decision: Reuse ast.SelectorExpr + ast.CallExpr**

Actually, we can represent `numbers.map(fn)` as:
- `CallExpr` with `Fun = SelectorExpr{X: Ident("numbers"), Sel: Ident("map")}`
- Standard Go AST already handles this!

**No new AST node needed** - we'll just transform existing CallExpr patterns.

---

## 5. Dependency Map

### Build Dependencies

```
functional_utilities (plugin)
    ├── pkg/stdlib (generated code imports this)
    ├── pkg/ast (for traversal)
    └── go/ast (standard library)

pkg/stdlib
    ├── (future) pkg/types/result.go  # Result[T, E] implementation
    └── (future) pkg/types/option.go  # Option[T] implementation
```

### Plugin Execution Order

```
1. sum_types           (foundation - enum declarations)
2. pattern_matching    (uses sum types)
3. error_propagation   (standalone)
4. statement_lifter    (utility)
5. functional_utilities  (NEW - no dependencies initially)
```

When lambda syntax is added:
```
5. lambda              (syntax transformation)
6. functional_utilities  (uses lambda transformed nodes)
```

### External Dependencies

No new external dependencies required:
- Go 1.25.4 standard library (generics support)
- Existing: `github.com/alecthomas/participle/v2`
- Existing: `go/ast`, `go/token`, `go/printer`

---

## 6. Implementation Notes

### Phase 1: Foundation (Week 1)

**Tasks:**
1. Set up git worktree for parallel development
2. Create `pkg/stdlib/` package structure
3. Implement basic Map, Filter, Reduce with generics
4. Write comprehensive unit tests for stdlib
5. Add golden test cases

**Worktree Setup:**
```bash
# Create worktree for functional utilities feature
git worktree add ../dingo-functional-utils -b feature/functional-utilities

# Work in isolation
cd ../dingo-functional-utils

# When done, merge back
git checkout main
git merge feature/functional-utilities
git worktree remove ../dingo-functional-utils
```

**Stdlib Implementation Priority:**
1. Core: Map, Filter, Reduce
2. Helpers: Sum, Count, All, Any, Find
3. Result: MapResult, FilterOk
4. Option: MapOption, FilterSome

### Phase 2: Parser Extension (Week 1)

**Tasks:**
1. Extend participle grammar to parse method calls on expressions
2. Handle chaining: `numbers.filter(p1).map(fn).reduce(init, r)`
3. Add test cases for parser

**Grammar Extension:**
```go
// Existing: PrimaryExpression
type PostfixExpression struct {
    Primary        *PrimaryExpression
    ErrorPropagate *bool
    MethodCalls    []*MethodCall  // NEW: Support chaining
}

type MethodCall struct {
    Dot    bool            // '.'
    Method string          // Method name
    Args   []*Expression   // Arguments
}
```

**Decision: Start Simple**
- Phase 1: Support single method call `numbers.map(fn)`
- Phase 2: Support chaining `numbers.filter(p).map(fn)`
- Keep parser changes minimal

### Phase 3: Plugin Implementation (Week 1)

**Tasks:**
1. Create `functional_utils.go` plugin
2. Implement Transform() to detect method calls
3. Generate stdlib function calls
4. Handle edge cases (nil slices, empty, type inference)

**Transformation Logic:**
```go
func (p *FunctionalUtilitiesPlugin) Transform(ctx *Context, node ast.Node) (ast.Node, error) {
    callExpr, ok := node.(*ast.CallExpr)
    if !ok {
        return node, nil
    }

    // Check if this is a method-style call: receiver.method(args)
    selExpr, ok := callExpr.Fun.(*ast.SelectorExpr)
    if !ok {
        return node, nil
    }

    methodName := selExpr.Sel.Name
    receiver := selExpr.X

    switch methodName {
    case "map":
        return p.transformMap(receiver, callExpr.Args)
    case "filter":
        return p.transformFilter(receiver, callExpr.Args)
    case "reduce":
        return p.transformReduce(receiver, callExpr.Args)
    default:
        return node, nil
    }
}
```

### Phase 4: Lambda Placeholder (Week 1)

Since lambda syntax is not yet implemented, we need a strategy:

**Option A: Use Go Function Literals**
```dingo
// Dingo source (temporary syntax)
let doubled = numbers.map(func(x int) int { return x * 2 })
```
- Pros: Works immediately, no parser changes
- Cons: Verbose, defeats the purpose

**Option B: String Placeholder**
```dingo
// NOT RECOMMENDED - just documenting
let doubled = numbers.map("x => x * 2")
```
- Pros: Placeholder for syntax
- Cons: Not type-safe, terrible DX

**Option C: Wait for Lambda Implementation**
```dingo
// Target syntax (requires lambda feature)
let doubled = numbers.map(|x| x * 2)
```
- Pros: Proper syntax, type-safe
- Cons: Blocks functional utilities

**Decision: Option A Initially, Design for Option C**

Implement functional utilities with Go function literal support first:
```go
// Generated Go code works fine
doubled := stdlib.Map(numbers, func(x int) int {
    return x * 2
})
```

When lambda syntax is added, it will transpile to the same Go code:
```dingo
// Dingo with lambda (future)
let doubled = numbers.map(|x| x * 2)

// Transpiles to same Go code
doubled := stdlib.Map(numbers, func(x int) int {
    return x * 2
})
```

No breaking changes needed!

### Phase 5: Result/Option Integration (Week 2)

**Tasks:**
1. Wait for Result[T, E] and Option[T] implementations
2. Add Result-aware utilities (MapResult, FilterOk)
3. Add Option-aware utilities (MapOption, FilterSome)
4. Add golden tests for integrated scenarios

**Coordination:**
- Check if Result/Option types exist in codebase
- If not, implement basic versions in `pkg/types/`
- Or coordinate with other development session

---

## 7. Testing Strategy

### Unit Tests

**stdlib Package Tests:**
```go
// pkg/stdlib/slice_test.go

func TestMap(t *testing.T) {
    tests := []struct{
        name   string
        input  []int
        fn     func(int) int
        want   []int
    }{
        {"double", []int{1,2,3}, func(x int) int { return x*2 }, []int{2,4,6}},
        {"nil slice", nil, func(x int) int { return x }, nil},
        {"empty", []int{}, func(x int) int { return x }, []int{}},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := Map(tt.input, tt.fn)
            assert.Equal(t, tt.want, got)
        })
    }
}
```

**Plugin Tests:**
```go
// pkg/plugin/builtin/functional_utils_test.go

func TestTransformMap(t *testing.T) {
    // Test AST transformation
}
```

### Golden Tests

**Test Cases:**
```dingo
// tests/golden/20_map_simple.dingo
package main

func main() {
    let numbers = []int{1, 2, 3, 4, 5}
    let doubled = numbers.map(func(x int) int { return x * 2 })
    println(doubled)
}
```

**Expected Output:**
```go
// tests/golden/20_map_simple.go.golden
package main

import "github.com/MadAppGang/dingo/pkg/stdlib"

func main() {
    numbers := []int{1, 2, 3, 4, 5}
    var doubled []int
    doubled = stdlib.Map(numbers, func(x int) int {
        return x * 2
    })
    println(doubled)
}
```

### Integration Tests

**Real-World Scenarios:**
1. Data pipeline: fetch → filter → transform → aggregate
2. Error handling: map with Result types
3. Optional chaining: map with Option types
4. Complex chaining: multiple operations

---

## 8. Alternatives Considered

### Alternative 1: Code Generation Per Type

**Approach:** Generate specialized Map/Filter/Reduce for each type used.

```go
// Generated for []int
func mapInt(slice []int, fn func(int) int) []int { ... }

// Generated for []string
func mapString(slice []string, fn func(string) string) []string { ... }
```

**Pros:**
- Potentially faster (no interface overhead, though generics are quite fast)
- Could optimize for specific types

**Cons:**
- Code bloat in generated output
- Harder to maintain
- Defeats "readable generated code" principle
- Go generics are already very efficient

**Rejected:** Go generics provide same performance without code duplication.

### Alternative 2: Method Syntax with Runtime Library

**Approach:** Create wrapper types with methods.

```go
// pkg/stdlib/slice.go
type Slice[T any] struct {
    inner []T
}

func (s Slice[T]) Map[U any](fn func(T) U) Slice[U] {
    return Slice[U]{inner: Map(s.inner, fn)}
}

// Usage in generated Go
doubled := stdlib.NewSlice(numbers).Map(func(x int) int { return x*2 }).Unwrap()
```

**Pros:**
- True method chaining in Go
- Closer to Dingo syntax

**Cons:**
- Allocations for wrapper structs
- Not "zero runtime overhead"
- Less idiomatic Go
- Harder to debug

**Rejected:** Violates zero-cost abstraction principle.

### Alternative 3: AST Inlining (Zero Function Calls)

**Approach:** Inline map/filter/reduce as direct loops.

```dingo
let doubled = numbers.map(|x| x * 2)

// Transpiles to fully inlined loop
var doubled []int
for _, x := range numbers {
    doubled = append(doubled, x * 2)
}
```

**Pros:**
- Absolute zero overhead
- Very readable generated code
- No stdlib dependency

**Cons:**
- Complex plugin logic
- Hard to maintain consistency
- Debugging shows different code than source
- Loses function call abstraction

**Decision: Hybrid Approach**
- Default: Use stdlib functions (readable, maintainable)
- Future: Add `-O2` optimization flag for inlining
- Best of both worlds

---

## 9. Git Worktree Strategy

### Rationale

From user request: "There is a parallel development session running"

**Why Worktree?**
- Isolate functional utilities work from concurrent sessions
- Avoid merge conflicts during development
- Test independently before integration
- Easy to abandon if design changes

### Setup Commands

```bash
# 1. Create worktree
cd /Users/jack/mag/dingo
git worktree add ../dingo-functional-utils feature/functional-utilities

# 2. Develop in worktree
cd ../dingo-functional-utils

# 3. Make changes
# ... implement features ...

# 4. Test
go test ./...
dingo build tests/golden/20_map_simple.dingo

# 5. Commit
git add .
git commit -m "feat: Add functional utilities (map, filter, reduce)"

# 6. Merge back to main
cd /Users/jack/mag/dingo
git merge feature/functional-utilities

# 7. Clean up worktree
git worktree remove ../dingo-functional-utils
```

### Branch Naming

```
feature/functional-utilities
```

**Conventions:**
- `feature/` prefix for new features
- Descriptive kebab-case name
- Aligns with existing branch strategy

---

## 10. Timeline and Milestones

### Week 1: Core Implementation

**Day 1-2: Stdlib Foundation**
- [ ] Create `pkg/stdlib/` package
- [ ] Implement Map, Filter, Reduce with generics
- [ ] Write unit tests (>90% coverage)
- [ ] Document with examples

**Day 3-4: Plugin Development**
- [ ] Create `functional_utils.go` plugin
- [ ] Implement AST transformation
- [ ] Register plugin in builtin registry
- [ ] Test with Go function literals

**Day 5: Integration**
- [ ] Add golden tests
- [ ] Test with existing transpiler
- [ ] Verify generated Go compiles and runs
- [ ] Update CHANGELOG.md

### Week 2: Extensions and Polish

**Day 1-2: Helper Functions**
- [ ] Add Sum, Count, All, Any, Find
- [ ] Add ForEach, Partition
- [ ] Comprehensive tests

**Day 3-4: Result/Option Integration**
- [ ] Coordinate with Result/Option implementation
- [ ] Implement MapResult, FilterOk
- [ ] Implement MapOption, FilterSome
- [ ] Golden tests for integration

**Day 5: Documentation**
- [ ] Update `features/functional-utilities.md`
- [ ] Add examples to README
- [ ] Update CHANGELOG.md
- [ ] Prepare for PR/merge

### Future: Lambda Integration (Separate Session)

When lambda syntax is implemented:
- [ ] Update parser to support `|x| expr` syntax
- [ ] Test functional utilities with lambda syntax
- [ ] Update golden tests
- [ ] Document combined usage

**No breaking changes needed** - function literals and lambdas transpile identically.

---

## 11. Open Questions and Risks

### Critical Questions (Need User Input)

See `gaps.json` for structured questions.

1. **Result/Option Types Status?**
   - Are Result[T, E] and Option[T] already implemented?
   - If not, should we implement basic versions?
   - What's the API contract (methods, constructors)?

2. **Lambda Syntax Priority?**
   - Is lambda implementation planned soon?
   - Should we wait, or proceed with function literals?
   - Any coordination needed?

3. **Method Chaining Scope?**
   - How many operations to support initially?
   - Should we support custom user methods?
   - Any performance constraints?

4. **Package Naming?**
   - Confirm `pkg/stdlib/` is acceptable
   - Alternative: `pkg/functional/`, `pkg/iter/`?
   - Consider future expansion (I/O, concurrency, etc.)

### Technical Risks

**Risk 1: Generic Type Inference**
- Go's type inference can fail in complex scenarios
- Mitigation: Provide explicit type parameters when needed
- Example: `stdlib.Map[int, string](slice, fn)`

**Risk 2: Performance of Generics**
- Generic functions have slight overhead vs specialized code
- Mitigation: Benchmarks show negligible difference for slice operations
- Future: Add optimization flag for inlining

**Risk 3: Nil Slice Handling**
- Go allows nil slices, need consistent behavior
- Mitigation: Defined semantics (nil in = nil out, or empty slice)
- Document and test thoroughly

**Risk 4: Chaining Complexity**
- Long chains might generate verbose Go code
- Mitigation: Each operation is clear and explicit
- Not actually a problem - readability > brevity in generated code

---

## 12. Success Criteria

### Functional Requirements

- [ ] Map transforms slice elements correctly
- [ ] Filter selects elements based on predicate
- [ ] Reduce aggregates slice to single value
- [ ] Nil and empty slices handled gracefully
- [ ] Works with all Go types (primitives, structs, pointers)

### Non-Functional Requirements

- [ ] Generated Go code is idiomatic and readable
- [ ] No runtime dependencies beyond stdlib package
- [ ] Compile-time type safety (no runtime panics)
- [ ] Performance equivalent to hand-written loops
- [ ] Golden tests pass for all scenarios

### Integration Requirements

- [ ] Works with existing Dingo features (sum types, ?)
- [ ] Ready for lambda syntax integration
- [ ] No conflicts with parallel development
- [ ] Clean merge from worktree to main

---

## 13. Future Enhancements

### Phase 2: Extended Utilities

```go
// Lazy evaluation (like Rust Iterator)
type Iterator[T any] interface {
    Next() Option[T]
}

// Parallel operations (using goroutines)
func ParallelMap[T, U any](slice []T, fn func(T) U) []U

// Windowing operations
func Windows[T any](slice []T, size int) [][]T
func Chunks[T any](slice []T, size int) [][]T
```

### Phase 3: Custom Collections

Support beyond slices:
- Maps: `MapKeys`, `MapValues`, `FilterMap`
- Channels: `ChanMap`, `ChanFilter`
- Custom types: Implement `Iterable` interface

### Phase 4: Optimization Modes

```bash
# Development: readable code
dingo build main.dingo

# Production: inline and optimize
dingo build -O2 main.dingo
```

---

## 14. References

### Go Ecosystem

- **Go Generics Proposal:** https://go.dev/blog/intro-generics
- **Go Slices Package:** golang.org/x/exp/slices
- **Go Maps Package:** golang.org/x/exp/maps

### Language Precedents

- **Rust Iterators:** https://doc.rust-lang.org/book/ch13-02-iterators.html
  - Lazy evaluation, chaining, zero-cost abstractions
- **Kotlin Collections:** https://kotlinlang.org/docs/collections-overview.html
  - Method chaining, extension functions
- **Swift Sequence:** https://developer.apple.com/documentation/swift/sequence
  - Functional operations on sequences

### Dingo Codebase

- `pkg/plugin/plugin.go` - Plugin architecture
- `pkg/plugin/builtin/error_propagation.go` - Example plugin
- `pkg/ast/ast.go` - AST extensions
- `features/functional-utilities.md` - Feature spec

---

## Appendix A: Complete Stdlib API

```go
// pkg/stdlib/slice.go

package stdlib

// === Core Operations ===

func Map[T, U any](slice []T, fn func(T) U) []U
func Filter[T any](slice []T, predicate func(T) bool) []T
func Reduce[T, U any](slice []T, init U, reducer func(U, T) U) U

// === Aggregation ===

func Sum[T Number](slice []T) T
func Count[T any](slice []T) int
func All[T any](slice []T, predicate func(T) bool) bool
func Any[T any](slice []T, predicate func(T) bool) bool

// === Search ===

func Find[T any](slice []T, predicate func(T) bool) Option[T]
func FindIndex[T any](slice []T, predicate func(T) bool) Option[int]
func Contains[T comparable](slice []T, value T) bool

// === Transformation ===

func FlatMap[T, U any](slice []T, fn func(T) []U) []U
func Partition[T any](slice []T, predicate func(T) bool) ([]T, []T)
func Unique[T comparable](slice []T) []T
func Reverse[T any](slice []T) []T

// === Utilities ===

func ForEach[T any](slice []T, fn func(T))
func Take[T any](slice []T, n int) []T
func Drop[T any](slice []T, n int) []T
func Zip[T, U any](a []T, b []U) []Pair[T, U]

// === Result Integration ===

func MapResult[T, U, E any](slice []T, fn func(T) Result[U, E]) Result[[]U, E]
func FilterOk[T, E any](slice []Result[T, E]) []T
func FilterErr[T, E any](slice []Result[T, E]) []E

// === Option Integration ===

func MapOption[T, U any](slice []T, fn func(T) Option[U]) []Option[U]
func FilterSome[T any](slice []Option[T]) []T
func FilterNone[T any](slice []Option[T]) []Option[T]
```

---

## Appendix B: Sample Transpilation

### Example 1: Simple Map

**Dingo Input:**
```dingo
package main

func main() {
    let numbers = []int{1, 2, 3, 4, 5}
    let doubled = numbers.map(func(x int) int { return x * 2 })
    println(doubled)
}
```

**Generated Go:**
```go
package main

import "github.com/MadAppGang/dingo/pkg/stdlib"

func main() {
    numbers := []int{1, 2, 3, 4, 5}
    var doubled []int
    doubled = stdlib.Map(numbers, func(x int) int {
        return x * 2
    })
    println(doubled)
}
```

### Example 2: Chained Operations

**Dingo Input:**
```dingo
package main

func processUsers(users []User) []string {
    return users
        .filter(func(u User) bool { return u.age > 18 })
        .map(func(u User) string { return u.name })
}
```

**Generated Go:**
```go
package main

import "github.com/MadAppGang/dingo/pkg/stdlib"

func processUsers(users []User) []string {
    __temp0 := stdlib.Filter(users, func(u User) bool {
        return u.age > 18
    })
    __temp1 := stdlib.Map(__temp0, func(u User) string {
        return u.name
    })
    return __temp1
}
```

### Example 3: With Result Types (Future)

**Dingo Input:**
```dingo
package main

func parseNumbers(strs []string) Result[[]int, Error] {
    return strs.mapResult(func(s string) Result[int, Error] {
        return parseInt(s)
    })
}
```

**Generated Go:**
```go
package main

import "github.com/MadAppGang/dingo/pkg/stdlib"

func parseNumbers(strs []string) Result[[]int, Error] {
    __result := stdlib.MapResult(strs, func(s string) Result[int, Error] {
        return parseInt(s)
    })
    return __result
}
```

---

## Sign-off

This plan provides a comprehensive architecture for implementing functional utilities in Dingo. The design prioritizes:

1. **Simplicity** - Use Go generics, reuse existing patterns
2. **Zero-cost** - Generate explicit, readable Go code
3. **Integration** - Deep integration with Result/Option types
4. **Extensibility** - Plugin architecture for future enhancements
5. **Safety** - Git worktree for parallel development

**Recommended Next Steps:**
1. Review and approve this plan
2. Answer critical questions in gaps.json
3. Set up git worktree
4. Begin Week 1 implementation

**Estimated Effort:** 2 weeks (1 core + 1 extensions/polish)

**Dependencies:** None critical, but coordination on Result/Option types beneficial.
