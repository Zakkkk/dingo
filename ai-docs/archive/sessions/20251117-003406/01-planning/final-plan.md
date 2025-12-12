# Functional Utilities - Final Implementation Plan
## Session: 20251117-003406

**Date:** 2025-11-17
**Author:** Claude Code (Sonnet 4.5)
**Status:** Final - Ready for Implementation

---

## Executive Summary

This plan implements functional utilities (map, filter, reduce + helpers) for Dingo slices. Based on user clarifications, we will:

1. **Coordinate with lambda implementation** - Design plugin to accept both Go function literals AND future lambda AST nodes
2. **Full initial scope** - Core (map, filter, reduce) + helpers (sum, count, all, any, find)
3. **Result/Option integration** - Create Result/Option-aware utilities (these types exist)
4. **Git worktree isolation** - Develop in separate worktree to avoid conflicts with parallel lambda work
5. **Transpile to loops** - Generate explicit Go loops (NOT stdlib function calls) per feature spec

**Critical Alignment**: The feature spec (features/functional-utilities.md) shows transpilation to explicit loops, NOT function calls. This differs from the initial plan's stdlib approach and requires a revised strategy.

---

## 1. Revised Approach

### Feature Spec Requirements

From `features/functional-utilities.md`:

```dingo
doubled := numbers.map(|x| x * 2)

// Transpiles to:
var doubled []int
for _, x := range numbers {
    doubled = append(doubled, x * 2)
}
```

**Key Insight**: The feature spec shows INLINE LOOP GENERATION, not function call wrappers.

### Architectural Decision Change

**Initial Plan:**
- Create pkg/stdlib with generic functions
- Transform `numbers.map(fn)` → `stdlib.Map(numbers, fn)`

**Revised Plan (Per Feature Spec):**
- NO stdlib package needed
- Plugin generates inline loops directly in AST
- Zero function call overhead
- More readable generated code

**Why This Is Better:**
- Aligns with feature specification
- True zero-cost abstraction
- Simpler debugging (no function boundaries)
- More idiomatic Go output
- No runtime dependencies at all

---

## 2. Package Structure

### Directory Layout

```
dingo/
├── pkg/
│   ├── plugin/
│   │   └── builtin/
│   │       ├── functional_utils.go       # NEW: Generates inline loops
│   │       └── functional_utils_test.go  # NEW: Plugin tests
│   │
│   ├── parser/
│   │   └── participle.go                 # MODIFY: Parse method calls
│   │
│   └── ast/
│       └── ast.go                        # MODIFY: If needed for AST nodes
│
├── features/
│   └── functional-utilities.md           # EXISTS: Feature spec
│
└── tests/
    └── golden/
        ├── 20_map_simple.go.golden           # NEW: Basic map
        ├── 21_filter_basic.go.golden         # NEW: Basic filter
        ├── 22_reduce_sum.go.golden           # NEW: Reduce example
        ├── 23_chaining.go.golden             # NEW: Chained operations
        ├── 24_helpers_sum.go.golden          # NEW: Sum helper
        ├── 25_helpers_any_all.go.golden      # NEW: Any/All helpers
        ├── 26_result_integration.go.golden   # NEW: With Result types
        └── 27_option_integration.go.golden   # NEW: With Option types
```

### NO pkg/stdlib Package

**Rationale**: Feature spec shows inline loop generation, not function call wrappers.

---

## 3. Implementation Strategy

### Phase 1: Core Operations (Week 1, Days 1-3)

#### 3.1 Map Operation

**Dingo Input:**
```dingo
doubled := numbers.map(|x| x * 2)
```

**Generated Go (AST):**
```go
var doubled []int
doubled = make([]int, 0, len(numbers))
for _, x := range numbers {
    doubled = append(doubled, x * 2)
}
```

**Plugin Logic:**
1. Detect `CallExpr` where `Fun` is `SelectorExpr` with `Sel.Name == "map"`
2. Extract receiver (numbers) and lambda/function argument
3. Generate:
   - Variable declaration for result slice
   - Make call with capacity hint (len of input)
   - For-range loop over input
   - Append transformed values

#### 3.2 Filter Operation

**Dingo Input:**
```dingo
evens := numbers.filter(|x| x % 2 == 0)
```

**Generated Go:**
```go
var evens []int
evens = make([]int, 0, len(numbers))
for _, x := range numbers {
    if x % 2 == 0 {
        evens = append(evens, x)
    }
}
```

**Plugin Logic:**
1. Detect `.filter()` method call
2. Generate:
   - Variable declaration
   - Make call with capacity hint
   - For-range loop
   - If statement with predicate
   - Conditional append

#### 3.3 Reduce Operation

**Dingo Input:**
```dingo
sum := numbers.reduce(0, |acc, x| acc + x)
```

**Generated Go:**
```go
var sum int
sum = 0
for _, x := range numbers {
    sum = sum + x
}
```

**Plugin Logic:**
1. Detect `.reduce()` method call
2. Extract initial value and reducer function
3. Generate:
   - Variable declaration initialized to init value
   - For-range loop
   - Accumulator update statement

### Phase 2: Helper Utilities (Week 1, Days 4-5)

#### 3.4 Sum Helper

**Dingo Input:**
```dingo
total := numbers.sum()
```

**Generated Go:**
```go
var total int
total = 0
for _, __v := range numbers {
    total = total + __v
}
```

**Implementation**: Syntactic sugar for `reduce(0, |acc, x| acc + x)`

#### 3.5 Count Helper

**Dingo Input:**
```dingo
numAdults := users.count(|u| u.age >= 18)
```

**Generated Go:**
```go
var numAdults int
numAdults = 0
for _, u := range users {
    if u.age >= 18 {
        numAdults = numAdults + 1
    }
}
```

#### 3.6 All/Any Helpers

**Dingo Input:**
```dingo
allPositive := numbers.all(|x| x > 0)
hasNegative := numbers.any(|x| x < 0)
```

**Generated Go:**
```go
var allPositive bool
allPositive = true
for _, x := range numbers {
    if !(x > 0) {
        allPositive = false
        break
    }
}

var hasNegative bool
hasNegative = false
for _, x := range numbers {
    if x < 0 {
        hasNegative = true
        break
    }
}
```

**Optimization**: Early exit with `break` statement.

#### 3.7 Find Helper

**Dingo Input:**
```dingo
firstEven := numbers.find(|x| x % 2 == 0)
// Returns Option[int]
```

**Generated Go:**
```go
var firstEven Option[int]
firstEven = None[int]()
for _, x := range numbers {
    if x % 2 == 0 {
        firstEven = Some[int](x)
        break
    }
}
```

**Requires**: Option[T] type (already exists per clarifications)

### Phase 3: Chaining Support (Week 1, Days 3-4)

#### 3.8 Method Chaining

**Dingo Input:**
```dingo
result := users
    .filter(|u| u.age > 18)
    .map(|u| u.name)
```

**Generated Go:**
```go
// First operation: filter
var __temp0 []User
__temp0 = make([]User, 0, len(users))
for _, u := range users {
    if u.age > 18 {
        __temp0 = append(__temp0, u)
    }
}

// Second operation: map
var result []string
result = make([]string, 0, len(__temp0))
for _, u := range __temp0 {
    result = append(result, u.name)
}
```

**Plugin Logic:**
1. Detect nested method calls
2. Generate temporary variables for intermediate results
3. Chain operations sequentially
4. Use temp variables as input to next operation

**Naming Convention**: `__temp0`, `__temp1`, etc. for intermediate results

### Phase 4: Result/Option Integration (Week 2, Days 1-2)

#### 3.9 MapResult (Short-Circuiting)

**Dingo Input:**
```dingo
parsed := strings.mapResult(|s| parseInt(s))
// Returns Result[[]int, Error]
```

**Generated Go:**
```go
var parsed Result[[]int, Error]
{
    __values := make([]int, 0, len(strings))
    __hasError := false
    var __error Error

    for _, s := range strings {
        __r := parseInt(s)
        if __r.IsErr() {
            __error = __r.UnwrapErr()
            __hasError = true
            break
        }
        __values = append(__values, __r.Unwrap())
    }

    if __hasError {
        parsed = Err[[]int, Error](__error)
    } else {
        parsed = Ok[[]int, Error](__values)
    }
}
```

**Note**: Uses block scope to contain temporary variables.

#### 3.10 FilterSome (Option Filtering)

**Dingo Input:**
```dingo
validNames := maybeNames.filterSome()
// Filters out None values, extracts Some
```

**Generated Go:**
```go
var validNames []string
validNames = make([]string, 0, len(maybeNames))
for _, __opt := range maybeNames {
    if __opt.IsSome() {
        validNames = append(validNames, __opt.Unwrap())
    }
}
```

---

## 4. Lambda Integration Strategy

### Current State: Go Function Literals

**Temporary Syntax (until lambda implemented):**
```dingo
doubled := numbers.map(func(x int) int { return x * 2 })
```

**Plugin Handling:**
- Accept `ast.FuncLit` nodes as arguments to map/filter/reduce
- Extract function body for inline generation
- Works immediately without lambda syntax

### Future State: Lambda Syntax

**Target Syntax (when lambda feature ships):**
```dingo
doubled := numbers.map(|x| x * 2)
```

**Integration Plan:**

1. **Lambda Plugin Runs First**
   - Lambda plugin transforms `|x| x * 2` → `func(x int) int { return x * 2 }`
   - Produces standard Go `ast.FuncLit` node

2. **Functional Utils Plugin Runs After**
   - Receives already-transformed function literal
   - Generates inline loop as normal
   - **No changes needed to functional utils plugin!**

3. **Plugin Execution Order**
```
Input: numbers.map(|x| x * 2)

↓ [Lambda Plugin]
numbers.map(func(x int) int { return x * 2 })

↓ [Functional Utils Plugin]
var __result []int
for _, x := range numbers {
    __result = append(__result, x * 2)
}
```

**Critical Design Decision**:
- Functional utils plugin is **agnostic to lambda vs function literal**
- Only requires `ast.FuncLit` as input
- Future-proof without modification

---

## 5. Key Implementation Details

### 5.1 AST Node Structure

**No New AST Nodes Needed**

Go's standard AST already represents method calls:
- `numbers.map(fn)` = `CallExpr{Fun: SelectorExpr{X: Ident("numbers"), Sel: Ident("map")}, Args: [...]}`

**Plugin Detection Pattern:**
```go
func (p *FunctionalUtilsPlugin) Transform(ctx *Context, node ast.Node) (ast.Node, error) {
    call, ok := node.(*ast.CallExpr)
    if !ok {
        return node, nil
    }

    sel, ok := call.Fun.(*ast.SelectorExpr)
    if !ok {
        return node, nil
    }

    switch sel.Sel.Name {
    case "map":
        return p.generateMapLoop(sel.X, call.Args)
    case "filter":
        return p.generateFilterLoop(sel.X, call.Args)
    case "reduce":
        return p.generateReduceLoop(sel.X, call.Args)
    // ... helpers
    }
}
```

### 5.2 Loop Generation Helpers

**Core Function: Generate For-Range Loop**

```go
func createForRangeLoop(
    iterVar string,      // Loop variable name (e.g., "x")
    sliceExpr ast.Expr,  // Input slice expression
    body []ast.Stmt,     // Loop body statements
) *ast.RangeStmt

func createMakeCall(
    elemType ast.Expr,   // Element type
    length ast.Expr,     // Capacity hint
) *ast.CallExpr
```

**Template Generation Approach:**

```go
// Instead of building AST nodes manually, use go/parser to parse templates

func generateMapLoop(receiver ast.Expr, fn *ast.FuncLit) *ast.BlockStmt {
    // Extract function parameter name
    paramName := fn.Type.Params.List[0].Names[0].Name

    // Extract function body expression
    bodyExpr := extractFunctionBody(fn)

    // Parse template
    template := fmt.Sprintf(`{
        var __result []T
        __result = make([]T, 0, len(%s))
        for _, %s := range %s {
            __result = append(__result, %s)
        }
    }`,
        astToString(receiver),
        paramName,
        astToString(receiver),
        astToString(bodyExpr),
    )

    // Parse and return
    return parseTemplate(template)
}
```

**Note**: Template approach is simpler than manual AST construction.

### 5.3 Type Inference

**Challenge**: Dingo needs to infer result slice element type.

**Approach 1: Use Go Type Checker**
```go
import "golang.org/x/tools/go/types"

func inferMapResultType(
    sliceType types.Type,
    fnType types.Type,
) types.Type {
    // Extract function return type
    sig := fnType.(*types.Signature)
    return types.NewSlice(sig.Results().At(0).Type())
}
```

**Approach 2: Syntactic Only (Simpler)**
- Don't infer at transpilation time
- Let Go compiler infer types from usage
- Use `var result []T` where T is extracted from function signature

**Decision**: Start with Approach 2 (simpler), add Approach 1 if needed.

### 5.4 Nil Slice Handling

**Question**: What should `nil.map(fn)` produce?

**Options:**
1. Panic (Go-like behavior)
2. Return nil (functional language behavior)
3. Return empty slice

**Decision**: Return nil (matches Go semantics)

```go
// Generated code includes nil check
var doubled []int
if numbers != nil {
    doubled = make([]int, 0, len(numbers))
    for _, x := range numbers {
        doubled = append(doubled, x * 2)
    }
}
```

**Optimization**: Only add nil check if needed (static analysis).

### 5.5 Capacity Optimization

**Always Pre-allocate with Capacity Hint:**

```go
// Map: output length = input length
result = make([]T, 0, len(input))

// Filter: unknown length, use input length as upper bound
result = make([]T, 0, len(input))

// Reduce: no slice allocation
```

**Benefit**: Reduces allocations, improves performance.

---

## 6. Parser Extensions

### 6.1 Method Call Syntax

**Current Parser State**: Already handles method calls via standard Go syntax.

**Verification Needed**: Confirm participle parser supports:
- `numbers.map(fn)`
- Chaining: `numbers.filter(p).map(fn)`

**If Not Supported**: Extend grammar.

**Grammar Extension (if needed):**

```go
type CallExpression struct {
    Operand    *Operand           `@@`
    Calls      []*Call            `@@*`
    MethodCall []*MethodCall      `@@*`  // NEW
}

type MethodCall struct {
    Dot    string               `@"."`
    Method string               `@Ident`
    Lparen string               `@"("`
    Args   []*Expression        `(@@ ("," @@)*)?`
    Rparen string               `@")"`
}
```

**Testing**: Add parser tests before implementing plugin.

### 6.2 Lambda Syntax (Deferred)

**Not In Scope**: Lambda syntax parsing is handled by parallel session.

**Our Responsibility**: Accept `ast.FuncLit` nodes from lambda plugin.

---

## 7. Testing Strategy

### 7.1 Golden Tests

**Test Cases:**

1. **20_map_simple.dingo** - Basic map operation
2. **21_filter_basic.dingo** - Basic filter operation
3. **22_reduce_sum.dingo** - Reduce for sum
4. **23_chaining.dingo** - Chained filter + map
5. **24_helpers_sum.dingo** - Sum helper
6. **25_helpers_any_all.dingo** - Any/All helpers
7. **26_result_integration.dingo** - MapResult with error handling
8. **27_option_integration.dingo** - FilterSome with Option types
9. **28_complex_pipeline.dingo** - Real-world data transformation
10. **29_nil_handling.dingo** - Nil slice edge cases

**Golden Test Format:**

```dingo
// tests/golden/20_map_simple.dingo
package main

func main() {
    numbers := []int{1, 2, 3, 4, 5}
    doubled := numbers.map(func(x int) int { return x * 2 })
    println(doubled)
}
```

**Expected Output:**

```go
// tests/golden/20_map_simple.go.golden
package main

func main() {
    numbers := []int{1, 2, 3, 4, 5}
    var doubled []int
    doubled = make([]int, 0, len(numbers))
    for _, x := range numbers {
        doubled = append(doubled, x*2)
    }
    println(doubled)
}
```

### 7.2 Unit Tests

**Plugin Tests:**

```go
// pkg/plugin/builtin/functional_utils_test.go

func TestTransformMap(t *testing.T) {
    tests := []struct {
        name  string
        input string
        want  string
    }{
        {
            name: "simple map",
            input: "numbers.map(func(x int) int { return x * 2 })",
            want: "...",  // Expected AST structure
        },
    }
    // Test AST transformation
}

func TestGenerateMapLoop(t *testing.T) {
    // Test loop generation logic
}

func TestChaining(t *testing.T) {
    // Test chained operations
}
```

### 7.3 Integration Tests

**End-to-End Tests:**

1. Write `.dingo` file with functional utilities
2. Transpile to `.go`
3. Compile with `go build`
4. Run and verify output
5. Compare generated code with golden file

**Test Script:**

```bash
#!/bin/bash
# tests/run_functional_tests.sh

for test in tests/golden/*_functional_*.dingo; do
    echo "Testing $test..."

    # Transpile
    dingo build "$test" -o "${test%.dingo}.go"

    # Compare with golden
    diff "${test%.dingo}.go" "${test}.golden"

    # Compile and run
    go run "${test%.dingo}.go"
done
```

---

## 8. Git Worktree Strategy

### 8.1 Setup

**Create Worktree:**

```bash
cd /Users/jack/mag/dingo

# Create new branch and worktree
git worktree add ../dingo-functional-utils feature/functional-utilities

# Switch to worktree
cd ../dingo-functional-utils
```

**Verify Isolation:**

```bash
# Check git status
git status
# Should show: On branch feature/functional-utilities

# Check for conflicts
git log --oneline --graph main..HEAD
```

### 8.2 Development Workflow

**Daily Workflow:**

```bash
# 1. Work in worktree
cd /Users/jack/mag/dingo-functional-utils

# 2. Make changes
# ... implement features ...

# 3. Test frequently
go test ./pkg/plugin/builtin/...
go test ./tests/golden_test.go

# 4. Commit small, focused changes
git add pkg/plugin/builtin/functional_utils.go
git commit -m "feat(functional): Implement map operation with inline loops"

# 5. Keep branch updated with main (if needed)
git fetch origin
git rebase origin/main
```

### 8.3 Integration

**Merge Back to Main:**

```bash
# 1. Return to main repo
cd /Users/jack/mag/dingo

# 2. Fetch latest changes
git fetch origin

# 3. Merge feature branch
git checkout main
git merge feature/functional-utilities

# 4. Resolve any conflicts (unlikely with worktree isolation)

# 5. Test merged code
go test ./...

# 6. Clean up worktree
git worktree remove ../dingo-functional-utils

# 7. Delete remote branch (if applicable)
git push origin --delete feature/functional-utilities
git branch -d feature/functional-utilities
```

---

## 9. Plugin Architecture

### 9.1 Plugin Interface Implementation

**File**: `pkg/plugin/builtin/functional_utils.go`

```go
package builtin

import (
    "go/ast"
    "go/token"
)

type FunctionalUtilitiesPlugin struct {
    name string
}

func NewFunctionalUtilitiesPlugin() *FunctionalUtilitiesPlugin {
    return &FunctionalUtilitiesPlugin{
        name: "functional_utilities",
    }
}

func (p *FunctionalUtilitiesPlugin) Name() string {
    return p.name
}

func (p *FunctionalUtilitiesPlugin) Transform(
    ctx *Context,
    node ast.Node,
) (ast.Node, error) {
    // Main transformation logic
    call, ok := node.(*ast.CallExpr)
    if !ok {
        return node, nil
    }

    sel, ok := call.Fun.(*ast.SelectorExpr)
    if !ok {
        return node, nil
    }

    switch sel.Sel.Name {
    case "map":
        return p.transformMap(ctx, sel.X, call.Args)
    case "filter":
        return p.transformFilter(ctx, sel.X, call.Args)
    case "reduce":
        return p.transformReduce(ctx, sel.X, call.Args)
    case "sum":
        return p.transformSum(ctx, sel.X)
    case "count":
        return p.transformCount(ctx, sel.X, call.Args)
    case "all":
        return p.transformAll(ctx, sel.X, call.Args)
    case "any":
        return p.transformAny(ctx, sel.X, call.Args)
    case "find":
        return p.transformFind(ctx, sel.X, call.Args)
    case "mapResult":
        return p.transformMapResult(ctx, sel.X, call.Args)
    case "filterSome":
        return p.transformFilterSome(ctx, sel.X)
    default:
        return node, nil
    }
}

// ... individual transformation methods
```

### 9.2 Core Transformation Methods

**Map Transformation:**

```go
func (p *FunctionalUtilitiesPlugin) transformMap(
    ctx *Context,
    receiver ast.Expr,
    args []ast.Expr,
) (ast.Node, error) {
    if len(args) != 1 {
        return nil, fmt.Errorf("map expects 1 argument, got %d", len(args))
    }

    fn, ok := args[0].(*ast.FuncLit)
    if !ok {
        return nil, fmt.Errorf("map expects function literal")
    }

    // Extract parameter name and body
    paramName := fn.Type.Params.List[0].Names[0].Name
    bodyExpr := extractReturnExpr(fn.Body)

    // Generate unique temp variable name
    resultVar := ctx.NewTempVar()

    // Build loop AST
    return &ast.BlockStmt{
        List: []ast.Stmt{
            // var resultVar []T
            createVarDecl(resultVar),

            // resultVar = make([]T, 0, len(receiver))
            createMakeAssign(resultVar, receiver),

            // for _, paramName := range receiver { ... }
            createForRange(paramName, receiver, []ast.Stmt{
                // resultVar = append(resultVar, bodyExpr)
                createAppendStmt(resultVar, bodyExpr),
            }),
        },
    }, nil
}
```

**Helper: Extract Return Expression:**

```go
func extractReturnExpr(body *ast.BlockStmt) ast.Expr {
    // Handle single expression: func(x) int { return x * 2 }
    if len(body.List) == 1 {
        if ret, ok := body.List[0].(*ast.ReturnStmt); ok {
            return ret.Results[0]
        }
    }

    // Handle expression statement: func(x) int { x * 2 } (future lambda syntax)
    if len(body.List) == 1 {
        if expr, ok := body.List[0].(*ast.ExprStmt); ok {
            return expr.X
        }
    }

    // Complex body: keep as-is
    return nil
}
```

### 9.3 AST Construction Helpers

**Create Variable Declaration:**

```go
func createVarDecl(name string, typ ast.Expr) *ast.DeclStmt {
    return &ast.DeclStmt{
        Decl: &ast.GenDecl{
            Tok: token.VAR,
            Specs: []ast.Spec{
                &ast.ValueSpec{
                    Names: []*ast.Ident{ast.NewIdent(name)},
                    Type:  typ,
                },
            },
        },
    }
}
```

**Create Make Assignment:**

```go
func createMakeAssign(varName string, slice ast.Expr) *ast.AssignStmt {
    return &ast.AssignStmt{
        Lhs: []ast.Expr{ast.NewIdent(varName)},
        Tok: token.ASSIGN,
        Rhs: []ast.Expr{
            &ast.CallExpr{
                Fun: ast.NewIdent("make"),
                Args: []ast.Expr{
                    // Type: []T (TODO: infer from slice)
                    ast.NewIdent("[]T"),
                    // Len: 0
                    &ast.BasicLit{Kind: token.INT, Value: "0"},
                    // Cap: len(slice)
                    &ast.CallExpr{
                        Fun:  ast.NewIdent("len"),
                        Args: []ast.Expr{slice},
                    },
                },
            },
        },
    }
}
```

**Create For-Range Statement:**

```go
func createForRange(
    varName string,
    slice ast.Expr,
    body []ast.Stmt,
) *ast.RangeStmt {
    return &ast.RangeStmt{
        Key:   ast.NewIdent("_"),
        Value: ast.NewIdent(varName),
        Tok:   token.DEFINE,
        X:     slice,
        Body: &ast.BlockStmt{
            List: body,
        },
    }
}
```

---

## 10. Type Handling

### 10.1 Type Inference Requirements

**Challenge**: Need to infer result slice type for `make([]T, ...)`.

**Example:**
```dingo
doubled := numbers.map(func(x int) int { return x * 2 })
//                                    ^^^--- Need to extract this
```

**Approach**: Extract from function literal's return type.

```go
func inferResultType(fn *ast.FuncLit) ast.Expr {
    // Get function signature
    funcType := fn.Type

    // Extract return type
    if funcType.Results != nil && len(funcType.Results.List) > 0 {
        return funcType.Results.List[0].Type
    }

    return nil
}

func createSliceType(elemType ast.Expr) *ast.ArrayType {
    return &ast.ArrayType{
        Elt: elemType,
    }
}
```

**Usage in Map:**
```go
resultElemType := inferResultType(fn)
resultSliceType := createSliceType(resultElemType)

// Now use in make call
makeCall := createMakeCall(resultSliceType, receiver)
```

### 10.2 Generic Type Support

**All Go Types Supported:**

```dingo
// Primitives
doubled := []int{1,2,3}.map(func(x int) int { return x * 2 })

// Structs
names := users.map(func(u User) string { return u.name })

// Pointers
ptrs := values.map(func(v int) *int { return &v })

// Interfaces
strs := items.map(func(i fmt.Stringer) string { return i.String() })
```

**No Special Handling Needed**: Go's type system handles this.

---

## 11. Result/Option Integration

### 11.1 Result Type Integration

**Assumption**: Result[T, E] is implemented as:

```go
type Result[T, E any] struct {
    value T
    err   E
    isOk  bool
}

func Ok[T, E any](value T) Result[T, E]
func Err[T, E any](err E) Result[T, E]
func (r Result[T, E]) IsOk() bool
func (r Result[T, E]) IsErr() bool
func (r Result[T, E]) Unwrap() T
func (r Result[T, E]) UnwrapErr() E
```

**MapResult Implementation:**

```dingo
parsed := strings.mapResult(func(s string) Result[int, Error] {
    return parseInt(s)
})
// Type: Result[[]int, Error]
```

**Generated Go:**

```go
var parsed Result[[]int, Error]
{
    __values := make([]int, 0, len(strings))
    __hasError := false
    var __error Error

    for _, s := range strings {
        __r := parseInt(s)
        if __r.IsErr() {
            __error = __r.UnwrapErr()
            __hasError = true
            break
        }
        __values = append(__values, __r.Unwrap())
    }

    if __hasError {
        parsed = Err[[]int, Error](__error)
    } else {
        parsed = Ok[[]int, Error](__values)
    }
}
```

**Early Exit Optimization**: Break on first error (short-circuit evaluation).

### 11.2 Option Type Integration

**Assumption**: Option[T] is implemented as:

```go
type Option[T any] struct {
    value  T
    isSome bool
}

func Some[T any](value T) Option[T]
func None[T any]() Option[T]
func (o Option[T]) IsSome() bool
func (o Option[T]) IsNone() bool
func (o Option[T]) Unwrap() T
```

**FilterSome Implementation:**

```dingo
validNames := maybeNames.filterSome()
// Input: []Option[string]
// Output: []string (only Some values)
```

**Generated Go:**

```go
var validNames []string
validNames = make([]string, 0, len(maybeNames))
for _, __opt := range maybeNames {
    if __opt.IsSome() {
        validNames = append(validNames, __opt.Unwrap())
    }
}
```

---

## 12. Implementation Timeline

### Week 1: Core Implementation

#### Day 1: Setup and Map
- [ ] Create git worktree (`feature/functional-utilities`)
- [ ] Verify parser supports method call syntax
- [ ] Create `pkg/plugin/builtin/functional_utils.go`
- [ ] Implement map transformation with inline loop generation
- [ ] Add unit tests for map
- [ ] Create golden test: `20_map_simple.go.golden`

#### Day 2: Filter and Reduce
- [ ] Implement filter transformation
- [ ] Implement reduce transformation
- [ ] Add unit tests for filter and reduce
- [ ] Create golden tests: `21_filter_basic.go.golden`, `22_reduce_sum.go.golden`
- [ ] Test all three core operations end-to-end

#### Day 3: Chaining Support
- [ ] Implement method chaining logic
- [ ] Handle temporary variable generation
- [ ] Add tests for chaining
- [ ] Create golden test: `23_chaining.go.golden`

#### Day 4: Helper Utilities (Part 1)
- [ ] Implement `sum()` helper
- [ ] Implement `count()` helper
- [ ] Add tests for helpers
- [ ] Create golden test: `24_helpers_sum.go.golden`

#### Day 5: Helper Utilities (Part 2)
- [ ] Implement `all()` helper
- [ ] Implement `any()` helper
- [ ] Implement `find()` helper (returns Option[T])
- [ ] Add tests for all/any/find
- [ ] Create golden test: `25_helpers_any_all.go.golden`

### Week 2: Integration and Polish

#### Day 1: Result Integration
- [ ] Verify Result[T, E] type exists in codebase
- [ ] Implement `mapResult()` transformation
- [ ] Add early exit optimization
- [ ] Add tests for Result integration
- [ ] Create golden test: `26_result_integration.go.golden`

#### Day 2: Option Integration
- [ ] Verify Option[T] type exists
- [ ] Implement `filterSome()` transformation
- [ ] Add tests for Option integration
- [ ] Create golden test: `27_option_integration.go.golden`

#### Day 3: Edge Cases and Optimization
- [ ] Implement nil slice handling
- [ ] Add capacity optimization for make calls
- [ ] Test edge cases (empty slices, single element, etc.)
- [ ] Create golden test: `29_nil_handling.go.golden`

#### Day 4: Complex Integration Test
- [ ] Create real-world data pipeline example
- [ ] Test combination of all features
- [ ] Create golden test: `28_complex_pipeline.go.golden`
- [ ] Performance testing

#### Day 5: Documentation and Merge
- [ ] Update CHANGELOG.md
- [ ] Add code comments and documentation
- [ ] Run full test suite
- [ ] Merge feature branch to main
- [ ] Remove worktree

---

## 13. Success Criteria

### Functional Requirements

- [x] Map transforms elements with inline loop
- [x] Filter selects elements with inline loop
- [x] Reduce aggregates with inline loop
- [x] Sum/Count/All/Any/Find helpers work correctly
- [x] Method chaining generates sequential operations
- [x] MapResult short-circuits on first error
- [x] FilterSome extracts Some values

### Code Quality

- [x] Generated Go code is readable and idiomatic
- [x] No stdlib package dependency (pure inline loops)
- [x] Capacity optimization for all allocations
- [x] Early exit optimization for all/any/find/mapResult
- [x] Nil slice handling is safe and consistent

### Testing

- [x] All golden tests pass
- [x] Unit tests have >90% coverage
- [x] Integration tests run successfully
- [x] No regressions in existing tests

### Integration

- [x] Works with Go function literals (temporary)
- [x] Compatible with future lambda syntax (plugin agnostic)
- [x] Integrates with Result[T, E] and Option[T]
- [x] No conflicts with parallel development (worktree isolation)

---

## 14. Risk Mitigation

### Risk 1: AST Complexity

**Risk**: Manual AST construction is error-prone and hard to maintain.

**Mitigation**:
- Use helper functions to encapsulate common patterns
- Write unit tests for each AST generation function
- Use go/printer to verify generated code compiles
- Add integration tests to catch AST errors early

### Risk 2: Type Inference Failures

**Risk**: Cannot infer result type in complex scenarios.

**Mitigation**:
- Extract types from function literal signatures (simple)
- Fallback: Use type parameters if inference fails
- Document limitations in feature spec
- Add comprehensive type tests

### Risk 3: Lambda Integration

**Risk**: Lambda plugin changes might break functional utils.

**Mitigation**:
- Design functional utils to accept `ast.FuncLit` only
- Lambda plugin's responsibility to produce `ast.FuncLit`
- Add integration test with both plugins
- Coordinate with lambda implementation session

### Risk 4: Performance Regression

**Risk**: Generated code might be slower than hand-written loops.

**Mitigation**:
- Always use capacity hints in make calls
- Add early exit optimizations
- Benchmark against equivalent Go code
- Profile generated code if needed

---

## 15. Open Questions

### Question 1: Nil Slice Behavior

**Options**:
1. Return nil (Go-like)
2. Return empty slice (functional-like)
3. Panic (error-like)

**Recommendation**: Return nil (matches Go semantics)

**Needs Decision**: User preference?

### Question 2: Sorted() Helper

**Feature spec mentions**: `.sorted()` in chaining example.

**Questions**:
- Should we implement `sorted()` in this session?
- What sorting semantics (stable? custom comparator?)

**Recommendation**: Defer to separate feature (sorting is complex)

### Question 3: Reverse() Helper

**Should we include**: `reverse()` helper?

**Recommendation**: Yes, simple and useful
- Generates in-place reversal or new slice copy

---

## 16. Future Enhancements

### Phase 3: Advanced Utilities

After initial release, consider:

- `flatMap()` - Flatten nested slices
- `partition()` - Split into two slices based on predicate
- `unique()` - Remove duplicates
- `zip()` - Combine two slices element-wise
- `take(n)` / `drop(n)` - Slice operations
- `groupBy()` - Group elements by key

### Phase 4: Parallel Operations

For large datasets:

```dingo
results := bigSlice.parallelMap(|x| expensiveOperation(x))
```

**Implementation**: Use goroutines and channels

### Phase 5: Lazy Evaluation

Iterator-like lazy evaluation:

```dingo
result := hugeSlice
    .iter()
    .filter(predicate)
    .map(transform)
    .take(10)    // Only process first 10 after filtering
    .collect()
```

**Benefit**: Avoid intermediate allocations

---

## 17. Coordination Points

### With Lambda Implementation

**Coordination Needed**:
- Lambda plugin execution order (before functional utils)
- Lambda AST output format (must be `ast.FuncLit`)
- Integration testing

**Action**: Schedule sync meeting with lambda session

### With Result/Option Implementation

**Verification Needed**:
- Confirm Result[T, E] API contract
- Confirm Option[T] API contract
- Test compatibility

**Action**: Review existing implementation before Result integration (Day 1, Week 2)

---

## 18. Documentation Requirements

### Code Documentation

**Each transformation method needs**:
- Function doc comment
- Parameter descriptions
- Example Dingo input
- Example generated Go output

**Example**:
```go
// transformMap generates an inline loop for map operations.
//
// Dingo:
//   doubled := numbers.map(|x| x * 2)
//
// Generated Go:
//   var doubled []int
//   doubled = make([]int, 0, len(numbers))
//   for _, x := range numbers {
//       doubled = append(doubled, x * 2)
//   }
func (p *FunctionalUtilitiesPlugin) transformMap(...) { ... }
```

### CHANGELOG Update

**Add to CHANGELOG.md**:

```markdown
## [Unreleased]

### Added
- Functional utilities for slices: `map()`, `filter()`, `reduce()`
- Helper methods: `sum()`, `count()`, `all()`, `any()`, `find()`
- Method chaining support: `slice.filter(...).map(...)`
- Result integration: `mapResult()` for error-aware mapping
- Option integration: `filterSome()` to extract Some values

### Implementation
- Transpiles to inline Go loops (zero function call overhead)
- Compatible with Go function literals and future lambda syntax
- Capacity optimization for all slice allocations
- Early exit optimization for short-circuit operations
```

### Feature Spec Update

**Update features/functional-utilities.md**:
- Change status to "✅ Implemented"
- Add detailed examples
- Document generated code patterns
- List all available operations

---

## 19. Final Checklist

### Pre-Implementation

- [ ] Review this plan with user
- [ ] Confirm Result/Option API contracts
- [ ] Verify parser supports method call syntax
- [ ] Set up git worktree

### Week 1 Deliverables

- [ ] Map, filter, reduce implemented
- [ ] Chaining support working
- [ ] Sum, count, all, any, find helpers complete
- [ ] 5+ golden tests passing
- [ ] Unit tests >90% coverage

### Week 2 Deliverables

- [ ] Result/Option integration complete
- [ ] Edge cases handled (nil, empty slices)
- [ ] All optimizations implemented
- [ ] Complex integration test passing
- [ ] Documentation complete

### Integration

- [ ] All tests pass (unit + golden + integration)
- [ ] No conflicts with main branch
- [ ] CHANGELOG updated
- [ ] Feature spec updated
- [ ] Code reviewed and merged
- [ ] Worktree removed

---

## 20. Appendix: Complete Operation Reference

### Core Operations

| Operation | Signature | Generates |
|-----------|-----------|-----------|
| `map` | `.map(fn: T → U)` | For-range with append |
| `filter` | `.filter(pred: T → bool)` | For-range with conditional append |
| `reduce` | `.reduce(init: U, fn: (U, T) → U)` | For-range with accumulator |

### Helper Operations

| Operation | Signature | Generates |
|-----------|-----------|-----------|
| `sum` | `.sum()` | For-range with addition |
| `count` | `.count(pred: T → bool)` | For-range with counter |
| `all` | `.all(pred: T → bool)` | For-range with break |
| `any` | `.any(pred: T → bool)` | For-range with break |
| `find` | `.find(pred: T → bool)` | For-range with Option return |

### Integration Operations

| Operation | Signature | Generates |
|-----------|-----------|-----------|
| `mapResult` | `.mapResult(fn: T → Result[U, E])` | For-range with error handling |
| `filterSome` | `.filterSome()` | For-range filtering Some values |

---

## Sign-Off

This final implementation plan:

1. **Aligns with feature spec** - Generates inline loops, not function calls
2. **Coordinates with lambda** - Plugin agnostic to lambda vs function literal
3. **Full initial scope** - Core + helpers + Result/Option integration
4. **Isolated development** - Git worktree prevents conflicts
5. **Clear timeline** - 2-week implementation with daily milestones

**Next Steps**:
1. User review and approval
2. Set up git worktree
3. Begin Day 1 implementation

**Estimated Effort**: 10 days (2 weeks)
**Risk Level**: Low (well-defined scope, proven patterns)
**Dependencies**: None blocking (Result/Option exist, lambda coordination async)
