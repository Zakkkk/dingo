# Phase 1.6: Error Propagation Operator Pipeline Integration

## Executive Summary

This document provides a detailed architectural plan for completing Phase 1.6 of the Dingo project: full pipeline integration of the error propagation operator (`?`). The current implementation has parser detection and a standalone plugin, but lacks end-to-end integration in the transformation pipeline.

**Goal:** Achieve fully functional `?` operator transpilation from `.dingo` source to idiomatic Go code with proper source mapping.

---

## Current State Analysis

### What's Already Implemented

#### 1. Parser Support (✅ Complete)
- **Location:** `/Users/jack/mag/dingo/pkg/parser/participle.go`
- **Status:** The parser successfully detects the `?` operator
- **Implementation:**
  - `PostfixExpression` struct captures `ErrorPropagate *bool` field (line 129-132)
  - `convertPostfix()` creates `ErrorPropagationExpr` nodes (line 410-435)
  - Dingo nodes are tracked in `currentFile.DingoNodes` map
  - Uses placeholder pattern: stores primary expression in AST, maps to Dingo node

**Example:**
```dingo
user := fetchUser(id)?
```
Parser creates:
- Standard `ast.CallExpr` for `fetchUser(id)`
- `dingoast.ErrorPropagationExpr` wrapping it
- Maps the call expression → error propagation node in file's DingoNodes map

#### 2. AST Representation (✅ Complete)
- **Location:** `/Users/jack/mag/dingo/pkg/ast/ast.go`
- **Status:** Well-designed AST node for error propagation
- **Implementation:**
  ```go
  type ErrorPropagationExpr struct {
      X      ast.Expr    // The expression (e.g., fetchUser(id))
      OpPos  token.Pos   // Position of '?'
      Syntax SyntaxStyle // question/bang/try
  }
  ```
- Implements `ast.Expr` interface
- Supports three syntaxes (currently only `?` is parsed)
- Has proper `Pos()` and `End()` methods for position tracking

#### 3. Plugin Foundation (⚠️ Partial)
- **Location:** `/Users/jack/mag/dingo/pkg/plugin/builtin/error_propagation.go`
- **Status:** Core transformation logic exists but not integrated
- **Implementation:**
  - `ErrorPropagationPlugin` transforms `ErrorPropagationExpr` to Go early-return pattern
  - Generates unique variable names (`__tmp0`, `__err0`)
  - Creates assignment + error check + return statements
  - Returns `temporaryStmtWrapper` with statements

**Current Transformation:**
```go
// Input: fetchUser(id)?
// Output:
__tmp0, __err0 := fetchUser(id)
if __err0 != nil {
    return nil, __err0  // TODO: Proper zero value
}
// user := __tmp0 (handled separately)
```

**Limitations:**
- Returns statements wrapped in `temporaryStmtWrapper` (not standard `ast.Node`)
- No integration with actual AST tree mutation
- Zero value hardcoded as `nil` (needs type inference)
- No statement lifting for expression contexts

#### 4. Plugin System (✅ Complete)
- **Location:** `/Users/jack/mag/dingo/pkg/plugin/`
- **Status:** Full plugin architecture with pipeline
- **Components:**
  - `Registry` - plugin registration and dependency management
  - `Pipeline` - executes plugins in order via `ast.Inspect`
  - `Context` - shared state (FileSet, TypeInfo, Logger)
  - `BasePlugin` - implementation helper

**Issue:** Current pipeline uses `ast.Inspect` which only visits nodes, doesn't support mutation/replacement of parent references.

#### 5. Generator (⚠️ Integration Gap)
- **Location:** `/Users/jack/mag/dingo/pkg/generator/generator.go`
- **Status:** Has plugin support but doesn't handle Dingo nodes
- **Flow:**
  1. Parse `.dingo` → `dingoast.File`
  2. Transform via pipeline (if configured)
  3. Print `ast.File` to Go code
  4. Format output

**Gap:** The pipeline transforms nodes but doesn't properly handle:
- Dingo node lookup from `file.DingoNodes` map
- Statement context vs expression context
- Replacing nodes in parent structures
- Unpacking `temporaryStmtWrapper` into actual statements

#### 6. Source Maps (🚧 Skeleton Only)
- **Location:** `/Users/jack/mag/dingo/pkg/sourcemap/generator.go`
- **Status:** Structure exists, VLQ encoding TODO
- **Current:**
  - `Generator` collects `Mapping` entries
  - Can call `AddMapping(src, gen token.Position)`
  - `Generate()` returns JSON skeleton with empty `mappings` field
- **Missing:**
  - VLQ encoding implementation (lines encode as Base64 VLQ)
  - Integration with transformation pipeline
  - Actual mapping calls during code generation

---

## Architecture Design

### Core Challenge: Statement vs Expression Context

The `?` operator appears in expressions but generates statements:

```dingo
// Expression context
user := fetchUser(id)?

// Must become statements
__tmp0, __err0 := fetchUser(id)
if __err0 != nil {
    return nil, __err0
}
user := __tmp0
```

**Problem:** Go's AST requires statements in statement lists, expressions in expression contexts. We can't just replace an expression with multiple statements.

**Solution:** Two-pass transformation

### Proposed Architecture: Two-Pass Transformation

#### Pass 1: Discovery & Lifting
Walk the AST and identify all error propagation expressions. For each:
1. Determine if it's in statement or expression context
2. Find the enclosing statement or block
3. Mark for transformation

#### Pass 2: Transformation & Injection
For each marked error propagation:
1. Generate the error handling statements
2. If statement context: replace the statement with multiple statements
3. If expression context: inject statements before the current statement, replace expression with temp variable reference
4. Update source map with position mappings

### Detailed Component Design

#### Component 1: AST Transformer (NEW)

**Location:** `/Users/jack/mag/dingo/pkg/transformer/transformer.go`

**Purpose:** Orchestrate transformation of Dingo nodes to pure Go AST

**Key Types:**
```go
// Transformer handles AST transformations
type Transformer struct {
    fset      *token.FileSet
    sourceMap *sourcemap.Generator
    file      *dingoast.File

    // Track transformations
    stmtReplacements map[ast.Stmt][]ast.Stmt
    exprReplacements map[ast.Expr]ast.Expr
}

// Transform performs two-pass transformation
func (t *Transformer) Transform() error {
    // Pass 1: Discover Dingo nodes
    t.discover()

    // Pass 2: Transform and replace
    t.transform()

    return nil
}
```

**Responsibilities:**
- Walk the AST to find Dingo nodes
- Determine context (statement vs expression)
- Coordinate with specialized transformers (error propagation, etc.)
- Handle parent reference updates
- Collect source map entries

#### Component 2: Error Propagation Transformer (REFACTOR)

**Location:** `/Users/jack/mag/dingo/pkg/transformer/error_propagation.go`

**Purpose:** Transform `ErrorPropagationExpr` to Go early-return pattern

**Key Functions:**
```go
// TransformErrorPropagation handles ? operator transformation
func TransformErrorPropagation(
    expr *dingoast.ErrorPropagationExpr,
    ctx *Context,
) (*TransformResult, error)

// TransformResult contains generated statements and replacement expression
type TransformResult struct {
    // Statements to inject before current statement
    Statements []ast.Stmt

    // Expression to replace original with (the temp var)
    Replacement ast.Expr

    // Source map entries
    Mappings []sourcemap.Mapping
}
```

**Algorithm:**
```
Input: ErrorPropagationExpr{X: fetchUser(id), OpPos: ..., Syntax: "?"}
Context: Enclosing function returns (T, error)

1. Generate unique vars: __tmp0, __err0
2. Create assignment: __tmp0, __err0 := X
3. Create error check: if __err0 != nil
4. Infer return type T from function signature
5. Generate zero value for T
6. Create return: return zeroValue(T), __err0
7. Create if statement with return
8. Return:
   - Statements: [assignment, if-check]
   - Replacement: __tmp0
   - Mappings: [expr→assignment, ?→if-check]
```

#### Component 3: Statement Injector (NEW)

**Location:** `/Users/jack/mag/dingo/pkg/transformer/injector.go`

**Purpose:** Handle parent reference updates when injecting statements

**Key Function:**
```go
// InjectStatements inserts new statements before a target statement
// Updates parent BlockStmt.List to include new statements
func InjectStatements(
    block *ast.BlockStmt,
    targetStmt ast.Stmt,
    newStmts []ast.Stmt,
) error
```

**Algorithm:**
```
1. Find targetStmt index in block.List
2. Create new slice with capacity for all statements
3. Copy statements before target
4. Insert new statements
5. Adjust target statement if needed (e.g., replace assignment LHS with temp var)
6. Copy remaining statements
7. Update block.List
```

#### Component 4: Type Inference Engine (NEW)

**Location:** `/Users/jack/mag/dingo/pkg/types/inference.go`

**Purpose:** Determine types of expressions and generate zero values

**Key Functions:**
```go
// InferFunctionReturnType determines what a function returns
func InferFunctionReturnType(fn *ast.FuncDecl) (types.Type, error)

// GenerateZeroValue creates zero value expression for a type
func GenerateZeroValue(typ types.Type) ast.Expr
```

**Implementation:**
- Use `go/types` for type checking
- For simple cases: map type name → zero value
  - `int` → `0`
  - `string` → `""`
  - `*T` → `nil`
  - `bool` → `false`
- For complex types: call `new(T)` or use composite literal

#### Component 5: Source Map Integration (ENHANCE)

**Location:** `/Users/jack/mag/dingo/pkg/sourcemap/generator.go`

**Enhancements Needed:**
1. Implement VLQ encoding (use existing `github.com/go-sourcemap/sourcemap` library)
2. Add helper for common mapping patterns:
   ```go
   // MapExpansion records when one Dingo node becomes multiple Go nodes
   func (g *Generator) MapExpansion(
       dingoExpr token.Position,
       goStmts []token.Position,
   )
   ```

**VLQ Encoding:**
- Use `github.com/go-sourcemap/sourcemap` Consumer/Producer APIs
- Reference implementation: already imported in project
- Encode mappings as semicolon-separated lines, comma-separated segments
- Each segment: [genColumn, sourceIndex, sourceLine, sourceColumn, nameIndex]

---

## Implementation Plan

### Step 1: Type Inference Foundation (2-3 hours)

**Files to Create:**
- `/Users/jack/mag/dingo/pkg/types/inference.go`
- `/Users/jack/mag/dingo/pkg/types/inference_test.go`

**Tasks:**
1. Create basic type inference for function return types
2. Implement zero value generation for common types
3. Write tests for all Go basic types
4. Handle pointer types, slices, maps

**Deliverables:**
```go
// Example usage
funcDecl := ... // *ast.FuncDecl from parsed code
returnType, err := types.InferFunctionReturnType(funcDecl)
zeroVal := types.GenerateZeroValue(returnType)
// zeroVal is ast.Expr: &ast.BasicLit{Value: "0"} for int
```

### Step 2: Statement Injector (2-3 hours)

**Files to Create:**
- `/Users/jack/mag/dingo/pkg/transformer/injector.go`
- `/Users/jack/mag/dingo/pkg/transformer/injector_test.go`

**Tasks:**
1. Implement statement injection logic
2. Handle edge cases (empty blocks, single statement)
3. Test with various block structures
4. Ensure position preservation

**Deliverables:**
```go
// Example usage
block := funcBody.(*ast.BlockStmt)
targetStmt := block.List[2]
newStmts := []ast.Stmt{assign, ifCheck}
injector.InjectStatements(block, targetStmt, newStmts)
// block.List now has newStmts before targetStmt
```

### Step 3: Core Transformer (4-5 hours)

**Files to Create:**
- `/Users/jack/mag/dingo/pkg/transformer/transformer.go`
- `/Users/jack/mag/dingo/pkg/transformer/context.go`
- `/Users/jack/mag/dingo/pkg/transformer/transformer_test.go`

**Tasks:**
1. Implement two-pass discovery/transform algorithm
2. Create context tracking (current function, block, etc.)
3. Handle Dingo node lookup from `file.DingoNodes`
4. Coordinate with specialized transformers
5. Update parent references correctly

**Algorithm:**
```
Pass 1 - Discovery:
  Walk AST with ast.Inspect
  For each node:
    If in file.DingoNodes:
      Determine context (statement vs expression)
      Store in transformationPlan

Pass 2 - Transform:
  For each entry in transformationPlan:
    Call appropriate transformer (e.g., TransformErrorPropagation)
    Get TransformResult (statements, replacement, mappings)
    Inject statements using injector
    Replace expression using ast.Inspect + mutation
    Record source mappings
```

### Step 4: Error Propagation Refactor (3-4 hours)

**Files to Modify:**
- `/Users/jack/mag/dingo/pkg/transformer/error_propagation.go` (move from plugin)
- `/Users/jack/mag/dingo/pkg/transformer/error_propagation_test.go`

**Tasks:**
1. Move logic from plugin to transformer
2. Remove `temporaryStmtWrapper` hack
3. Integrate with type inference for proper zero values
4. Add source map recording
5. Handle both statement and expression contexts

**New API:**
```go
// Old (plugin):
func (p *ErrorPropagationPlugin) Transform(ctx *plugin.Context, node ast.Node) (ast.Node, error)

// New (transformer):
func TransformErrorPropagation(
    expr *dingoast.ErrorPropagationExpr,
    ctx *TransformContext,
) (*TransformResult, error)
```

### Step 5: Generator Integration (2-3 hours)

**Files to Modify:**
- `/Users/jack/mag/dingo/pkg/generator/generator.go`

**Changes:**
1. Add transformer before printing:
   ```go
   // After pipeline (if configured)
   if file.HasDingoNodes() {
       transformer := transformer.New(g.fset, file, sourceMapGen)
       err := transformer.Transform()
       if err != nil {
           return nil, fmt.Errorf("transformation failed: %w", err)
       }
   }
   ```
2. Pass source map generator to transformer
3. Write source map file after generation

### Step 6: Source Map VLQ Encoding (3-4 hours)

**Files to Modify:**
- `/Users/jack/mag/dingo/pkg/sourcemap/generator.go`
- `/Users/jack/mag/dingo/pkg/sourcemap/sourcemap_test.go`

**Tasks:**
1. Implement VLQ encoding using `go-sourcemap` library
2. Sort mappings by generated position (required for VLQ)
3. Generate properly formatted mappings string
4. Test with real JavaScript source map consumers

**Reference Implementation:**
```go
import "github.com/go-sourcemap/sourcemap"

func (g *Generator) Generate() ([]byte, error) {
    // Use sourcemap.Producer to encode mappings
    producer := sourcemap.NewProducer()

    for _, m := range g.mappings {
        producer.AddMapping(&sourcemap.Mapping{
            GeneratedLine: m.GenLine,
            GeneratedColumn: m.GenColumn,
            SourceLine: m.SourceLine,
            SourceColumn: m.SourceColumn,
            Source: g.sourceFile,
            Name: m.Name,
        })
    }

    return producer.ToJSON()
}
```

### Step 7: End-to-End Testing (2-3 hours)

**Files to Create:**
- `/Users/jack/mag/dingo/tests/error_propagation_integration_test.go`
- `/Users/jack/mag/dingo/examples/error_propagation/basic.dingo`
- `/Users/jack/mag/dingo/examples/error_propagation/chained.dingo`
- `/Users/jack/mag/dingo/examples/error_propagation/expression_context.dingo`

**Test Cases:**
1. Simple statement context: `user := fetchUser(id)?`
2. Chained operations: `result := fetch()?.validate()?.save()?`
3. Expression context: `return processUser(fetchUser(id)?)`
4. Error wrapping: `user := fetchUser(id) ? "failed to fetch user"` (future)
5. Different return types: `int`, `string`, `*User`, `[]Item`

**Validation:**
1. Generated Go code compiles
2. Behavior matches manual error handling
3. Source maps point to correct positions
4. All positions are tracked correctly

### Step 8: Documentation & Examples (1-2 hours)

**Files to Update:**
- `/Users/jack/mag/dingo/CHANGELOG.md` - Add Phase 1.6 completion
- `/Users/jack/mag/dingo/features/error-propagation.md` - Update status to ✅

**Files to Create:**
- `/Users/jack/mag/dingo/docs/error-propagation-guide.md` - User-facing guide

---

## Testing Strategy

### Unit Tests

**Type Inference:**
- Test all Go basic types
- Test pointer, slice, array, map types
- Test struct types
- Test interface types
- Test error cases (invalid types)

**Statement Injector:**
- Inject at beginning of block
- Inject in middle of block
- Inject at end of block
- Edge cases: empty block, single statement
- Verify positions are preserved

**Transformer:**
- Discover Dingo nodes correctly
- Handle nested expressions
- Multiple `?` in same function
- Statement vs expression context
- Parent reference updates

**Error Propagation:**
- Generate correct temp variables
- Generate correct error check
- Generate correct zero values
- Generate correct return statement
- Handle different return types

### Integration Tests

**End-to-End:**
1. Parse → Transform → Generate for simple case
2. Verify generated code matches expected output
3. Compile generated code with `go build`
4. Run and verify behavior
5. Check source map correctness

**Golden Files:**
Create test fixtures with:
- Input: `test.dingo`
- Expected: `test.go.golden`
- Source map: `test.go.map.golden`

Compare generated output to golden files.

### Manual Testing

1. Build example files in `/examples/error_propagation/`
2. Run with `dingo build` and verify output
3. Use `dingo run` to execute
4. Test with real Go projects (import generated code)

---

## Source Mapping Strategy

### Mapping Types

**1. Expression Preservation**
```
Dingo:  user := fetchUser(id)?
        ^          ^           ^
        |          |           |
Go:     __tmp0, __err0 := fetchUser(id)
        ^                 ^
```

Map:
- `fetchUser(id)` in Dingo → `fetchUser(id)` in Go (preserved)
- `?` operator → error check statements

**2. Statement Expansion**
```
Dingo line 10: user := fetchUser(id)?
                          ^
Go lines 10-14:
  10: __tmp0, __err0 := fetchUser(id)
  11: if __err0 != nil {
  12:     return nil, __err0
  13: }
  14: user := __tmp0
```

Map:
- Dingo line 10 → Go lines 10-14 (one-to-many)
- IDE jumping to line 10 in Go shows all expanded code
- Debugger breakpoint on Dingo line 10 → set on Go line 10

**3. Position Granularity**

Map at expression level, not statement level:
- Call expression: exact position preserved
- Operator `?`: maps to if statement condition
- Variable name: maps to temp variable

### Source Map Format

```json
{
  "version": 3,
  "file": "output.go",
  "sourceRoot": "",
  "sources": ["input.dingo"],
  "names": ["user", "fetchUser", "id"],
  "mappings": "AAAA;AACA,QAAA,KAAA,OAAA;AACA,EAAA,GAAA;AACA,IAAA,MAAA,GAAA;AACA,GAAA;AACA,KAAA,IAAA,MAAA"
}
```

Mappings encode:
- Semicolons separate lines
- Commas separate segments in a line
- VLQ-encoded: [genCol, srcFile, srcLine, srcCol, name]

---

## File Structure

```
pkg/
├── transformer/           # NEW package
│   ├── transformer.go     # Main orchestrator
│   ├── context.go         # Transform context
│   ├── error_propagation.go  # ? operator transformation
│   ├── injector.go        # Statement injection
│   ├── transformer_test.go
│   ├── error_propagation_test.go
│   └── injector_test.go
│
├── types/                 # NEW package
│   ├── inference.go       # Type inference
│   ├── zero_values.go     # Zero value generation
│   └── inference_test.go
│
├── generator/
│   └── generator.go       # MODIFY: Add transformer integration
│
├── sourcemap/
│   ├── generator.go       # MODIFY: Add VLQ encoding
│   └── sourcemap_test.go  # MODIFY: Test VLQ
│
└── plugin/
    └── builtin/
        └── error_propagation.go  # DEPRECATE or keep for plugin API demo

tests/
├── error_propagation_integration_test.go  # NEW
└── golden/                                 # NEW
    ├── simple.dingo
    ├── simple.go.golden
    ├── chained.dingo
    └── chained.go.golden

examples/error_propagation/
├── basic.dingo             # NEW
├── chained.dingo           # NEW
├── file_ops.dingo          # EXISTS
└── http_client.dingo       # EXISTS
```

---

## Dependencies

### Existing (Already in go.mod)
- `github.com/alecthomas/participle/v2` - Parser
- `github.com/go-sourcemap/sourcemap` - Source map encoding/decoding
- `github.com/spf13/cobra` - CLI
- Standard library: `go/ast`, `go/token`, `go/printer`, `go/format`, `go/types`

### New (Need to Add)
None! All required dependencies are already available.

---

## Risk Assessment

### High Risk
1. **Parent Reference Updates** - Replacing nodes in AST requires finding and updating parent
   - Mitigation: Use `astutil.Apply` from `golang.org/x/tools/go/ast/astutil`
   - Alternative: Rebuild AST instead of mutating

2. **Type Inference Complexity** - Full Go type inference is complex
   - Mitigation: Phase 1.6 only handles simple cases (basic types, pointers)
   - Future: Integrate full `go/types` package

### Medium Risk
1. **Source Map Correctness** - VLQ encoding is tricky
   - Mitigation: Use battle-tested `go-sourcemap` library
   - Validation: Test with Chrome DevTools

2. **Statement Context Detection** - Determining context is error-prone
   - Mitigation: Comprehensive test coverage
   - Use `ast.Inspect` carefully

### Low Risk
1. **Performance** - Two-pass transformation might be slow
   - Mitigation: Profile and optimize if needed
   - Likely not an issue for Phase 1 scale

2. **Testing Coverage** - Missing edge cases
   - Mitigation: Use golden file testing, fuzz testing

---

## Success Criteria

### Must Have (Phase 1.6 Complete)
- ✅ Parse `?` operator (already done)
- ✅ Transform to Go early-return pattern
- ✅ Generate compilable Go code
- ✅ Handle statement context (assignment)
- ✅ Generate proper zero values for return types
- ✅ Source maps with VLQ encoding
- ✅ End-to-end test passing
- ✅ Example programs working

### Should Have
- ✅ Handle expression context (return statements, function args)
- ✅ Multiple `?` in same function
- ✅ Chained `?` operators
- ✅ All basic Go types supported

### Nice to Have (Future Phases)
- ⏭️ Error wrapping syntax: `expr? "context message"`
- ⏭️ Type inference for complex types (generics, interfaces)
- ⏭️ Optimization: eliminate unnecessary temp variables
- ⏭️ `!` and `try` syntax variants

---

## Migration Path

### Deprecating Plugin Approach

Current code has error propagation as a plugin. Phase 1.6 moves it to dedicated transformer:

**Reason:** Plugins are for optional features. Error propagation is core syntax, not optional.

**Plan:**
1. Keep `pkg/plugin/builtin/error_propagation.go` as reference
2. Create `pkg/transformer/error_propagation.go` with better implementation
3. Update generator to use transformer instead of plugin
4. Document plugin approach was for exploration
5. Keep plugin system for future truly optional features (e.g., linters, custom transforms)

---

## Timeline Estimate

**Total: 19-27 hours (2.5-3.5 days of focused work)**

- Step 1: Type Inference - 2-3 hours
- Step 2: Statement Injector - 2-3 hours
- Step 3: Core Transformer - 4-5 hours
- Step 4: Error Propagation Refactor - 3-4 hours
- Step 5: Generator Integration - 2-3 hours
- Step 6: Source Map VLQ - 3-4 hours
- Step 7: End-to-End Testing - 2-3 hours
- Step 8: Documentation - 1-2 hours

**Buffer:** +20% for unexpected issues = 23-33 hours total

**Realistic:** 4-5 working days for complete, polished implementation

---

## Next Steps

1. Review this plan with stakeholders
2. Address questions in `gaps.json`
3. Set up task tracking
4. Begin implementation with Step 1 (Type Inference)
5. Commit after each step with working tests
6. Update CHANGELOG.md when complete

---

## References

### Internal Documentation
- `/Users/jack/mag/dingo/CLAUDE.md` - Project instructions
- `/Users/jack/mag/dingo/features/error-propagation.md` - Feature specification
- `/Users/jack/mag/dingo/CHANGELOG.md` - Current status

### External References
- Go AST Manipulation: https://pkg.go.dev/go/ast
- AST Utilities: https://pkg.go.dev/golang.org/x/tools/go/ast/astutil
- Go Types: https://pkg.go.dev/go/types
- Source Map v3 Spec: https://sourcemaps.info/spec.html
- VLQ Encoding: https://www.html5rocks.com/en/tutorials/developertools/sourcemaps/#toc-base64vlq

### Similar Projects
- Borgo transpiler: https://github.com/borgo-lang/borgo
- TypeScript compiler: https://github.com/microsoft/TypeScript
- Rust Error Propagation RFC: https://rust-lang.github.io/rfcs/0243-trait-based-exception-handling.html
