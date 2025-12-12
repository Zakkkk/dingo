---
title: "TypeScript Arrow Syntax with Multi-line Bodies"
category: "Lambda Functions"
category_order: 70
subcategory: "TypeScript Style"
test_id: "lambda_02_typescript_multiline"
order: 2

complexity: "intermediate"
feature: "lambda-functions"
phase: "Phase 6.1"
status: "implemented"

description: "Demonstrates TypeScript arrow syntax with multi-line lambda bodies using braces, complex logic with if/else statements, lambdas returning structs, and nested function calls within lambda bodies"
summary: "Multi-line TypeScript arrow lambdas"
code_reduction: 0
lines_dingo: 64
lines_go: 64

go_proposal: "N/A"
go_proposal_link: ""
feature_file: "lambdas.md"
related_tests:
  - "lambda_01_typescript_basic"
  - "lambda_04_rust_types"

tags:
  - "lambda-functions"
  - "typescript-syntax"
  - "multi-line"
  - "block-body"
keywords:
  - "lambda"
  - "arrow"
  - "block"
  - "multi-line"
  - "complex logic"
---

# Test Reasoning: lambda_02_typescript_multiline

## What This Test Validates

This test validates TypeScript arrow syntax with **multi-line lambda bodies** (block syntax with braces). This extends basic expression lambdas to handle complex logic, control flow, and multiple statements.

### Key Patterns Tested

1. **Multi-line with braces**: `(x: int) => { ... }`
2. **Multiple statements**: `(n: int) => { if ... ; return ... }`
3. **Lambda returning struct**: `(x: int, y: int) => { return Point{...} }`
4. **Complex logic (if/else)**: `(input: string) => { if ...; if ...; return ... }`
5. **Nested function calls**: `(val: int) => { helper := func(...); return ... }`

### Dingo Code

```dingo
// Multi-line lambda with braces
complexCalc := (x: int) => {
	temp := x * 2
	return temp + 5
}

// Lambda with if/else logic
processNumber := (n: int) => {
	if n < 0 {
		return -n
	}
	return n * n
}

// Lambda returning struct
makePoint := (x: int, y: int) => {
	return Point{X: x, Y: y}
}
```

### Expected Go Output

```go
// Multi-line lambda with braces
complexCalc := func(x int) int {
	temp := x * 2
	return temp + 5
}

// Lambda with if/else logic
processNumber := func(n int) int {
	if n < 0 {
		return -n
	}
	return n * n
}

// Lambda returning struct
makePoint := func(x int, y int) Point {
	return Point{X: x, Y: y}
}
```

## Transformation Rules

### Pattern: Block Body vs Expression Body

**Expression Body** (from `lambda_01_typescript_basic`):
```dingo
x => x * 2
→
func(x) { return x * 2 }
```

**Block Body** (this test):
```dingo
x => { return x * 2 }
→
func(x) { return x * 2 }
```

**Detection**:
- If body starts with `{`, it's already a block
- Preprocessor **passes through braces as-is**
- No implicit `return` wrapper added

### Type Annotation with Block Body

```dingo
(x: int) => { temp := x * 2; return temp }
→
func(x int) int { temp := x * 2; return temp }
```

**Return Type Inference**:
- Explicit: `(x: int): int => { ... }`
- Implicit: Go compiler infers from `return` statements

**Note**: Current implementation does NOT enforce return type annotation for block bodies (Go compiler will validate).

## Complex Logic Patterns

### Pattern 1: If/Else in Lambda

```dingo
processNumber := (n: int) => {
	if n < 0 {
		return -n
	}
	return n * n
}
```

**Transpiles to**:
```go
processNumber := func(n int) int {
	if n < 0 {
		return -n
	}
	return n * n
}
```

**Challenge**: Multiple return statements, control flow.

**Solution**: Block body passes through verbatim, Go handles control flow.

### Pattern 2: Multiple Statements

```dingo
validate := (input: string) => {
	if len(input) == 0 {
		return "empty"
	}
	if len(input) > 10 {
		return "too long"
	}
	return "valid"
}
```

**Transpiles to**:
```go
validate := func(input string) string {
	if len(input) == 0 {
		return "empty"
	}
	if len(input) > 10 {
		return "too long"
	}
	return "valid"
}
```

**Challenge**: Sequential if statements, early returns.

**Solution**: Block body semantics match Go exactly.

### Pattern 3: Lambda Returning Struct

```dingo
makePoint := (x: int, y: int) => {
	return Point{X: x, Y: y}
}
```

**Transpiles to**:
```go
makePoint := func(x int, y int) Point {
	return Point{X: x, Y: y}
}
```

**Return Type**: Inferred from struct literal `Point{...}`.

**Note**: Could also use explicit return type:
```dingo
makePoint := (x: int, y: int): Point => {
	return Point{X: x, Y: y}
}
```

### Pattern 4: Nested Function Call in Lambda

```dingo
transform := (val: int) => {
	helper := func(x int) int {
		return x * 3
	}
	return helper(val) + 1
}
```

**Transpiles to**:
```go
transform := func(val int) int {
	helper := func(x int) int {
		return x * 3
	}
	return helper(val) + 1
}
```

**Challenge**: Lambda contains nested function literal.

**Solution**: Preprocessor doesn't recursively transform inside block bodies (Go syntax is valid).

## Edge Cases Covered

### 1. Empty Block

```dingo
noop := () => {}
```

**Transpiles to**:
```go
noop := func() {}
```

**Valid Go**: Empty function body is legal.

### 2. Single Statement Block

```dingo
x => { return x }
```

**Transpiles to**:
```go
func(x) { return x }
```

**Equivalent to**: `x => x` (expression body), but explicit.

### 3. Block with Variable Declaration

```dingo
(x: int) => {
	temp := x * 2
	return temp + 5
}
```

**Transpiles to**:
```go
func(x int) int {
	temp := x * 2
	return temp + 5
}
```

**Challenge**: Intermediate variable in block.

**Solution**: Go allows local variables in function bodies.

### 4. Block with Multiple Returns

```dingo
(n: int) => {
	if n < 0 {
		return -n
	}
	return n * n
}
```

**Transpiles to**:
```go
func(n int) int {
	if n < 0 {
		return -n
	}
	return n * n
}
```

**Challenge**: Multiple return paths.

**Solution**: Go compiler validates all paths return correct type.

## Expression Body vs Block Body

### When to Use Expression Body

```dingo
double := x => x * 2  // Single expression, implicit return
```

**Advantages**:
- More concise
- Implicit return
- Functional style

### When to Use Block Body

```dingo
validate := (x: int) => {
	if x < 0 {
		return "negative"
	}
	return "positive"
}
```

**Advantages**:
- Multiple statements
- Control flow (if/else, for, switch)
- Intermediate variables

### Formatting

**gofmt always runs** after transpilation, so:
```dingo
// User writes (compact)
x => {return x*2}

// After transpilation + gofmt
func(x) {
	return x * 2
}
```

**Benefit**: Idiomatic Go formatting guaranteed.

## Type Inference (Future Enhancement)

### Current Behavior (Phase 6.1)

```dingo
// Must specify types explicitly
process := (n: int) => {
	return n * 2
}
```

### Future (Phase 6.3)

```dingo
// Infer from assignment type
process: func(int) int = n => {
	return n * 2
}
// n inferred as int, return inferred as int
```

**Challenge**: Inferring types for block bodies requires analyzing all return statements.

**Solution**: go/types can infer return type from `return` expressions.

## Success Metrics

### Code Clarity

Multi-line lambdas maintain **readability** compared to Go:
```dingo
// Dingo
validate := (input: string) => {
	if len(input) == 0 { return "empty" }
	if len(input) > 10 { return "too long" }
	return "valid"
}

// Go
validate := func(input string) string {
	if len(input) == 0 { return "empty" }
	if len(input) > 10 { return "too long" }
	return "valid"
}
```

**Difference**: Arrow syntax (`=>`) vs `func` keyword.

### Zero Runtime Overhead

Generated Go is **identical** to hand-written:
- No wrapper functions
- No runtime dispatch
- Direct function literals

## Limitations (v1.0)

### No Expression Body for Multi-Statement

```dingo
// NOT VALID (multiple statements require braces)
x => y := x * 2; return y  // Error: Need { ... }
```

**Solution**: Use block body:
```dingo
x => { y := x * 2; return y }
```

### No Automatic Return Type

```dingo
// Current: Return type inferred by Go compiler
makePoint := (x: int, y: int) => {
	return Point{X: x, Y: y}
}

// Future: Could require explicit return type for block bodies
makePoint := (x: int, y: int): Point => {
	return Point{X: x, Y: y}
}
```

**Rationale**: Go already infers return types, no need to duplicate.

## Related Tests

### `lambda_01_typescript_basic.dingo`

**Difference**: Expression bodies only (`x => x * 2`).

**This test**: Block bodies with braces (`x => { return x * 2 }`).

### `lambda_04_rust_types.dingo`

**Difference**: Rust pipe syntax (`|x: int| -> bool { ... }`).

**This test**: TypeScript arrow syntax (`(x: int): bool => { ... }`).

## Future Enhancements

### Async Lambdas (Post-v1.0)

```dingo
// Future: Async lambda bodies
fetchData := async () => {
	data := await fetch("https://api.example.com")
	return data
}
```

**Requires**: Dingo async/await support (Phase 8+).

### Generator Lambdas (Post-v1.0)

```dingo
// Future: Generator lambdas
range := (n: int) => {
	for i := 0; i < n; i++ {
		yield i
	}
}
```

**Requires**: Go generics + iterator pattern.

---

**Last Updated**: 2025-11-20
**Test Status**: Implemented (Phase 6.1)
**Dingo Version**: 0.1.0-alpha (Phase 6.1)
