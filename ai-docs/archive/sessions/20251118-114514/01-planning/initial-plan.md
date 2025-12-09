# Phase 3 Implementation Plan: Fix A4, Fix A5, and Option[T] Type

**Date**: 2025-11-18
**Session**: 20251118-114514
**Architect**: golang-architect

---

## Executive Summary

Phase 3 addresses three critical areas to complete the Result[T,E]/Option[T] foundation:

1. **Fix A4**: Literal handling in constructors (`Ok(42)` generates `&42` which is invalid)
2. **Fix A5**: Enhanced type inference using `go/types` (eliminates `interface{}` fallbacks)
3. **Option[T]**: Complete implementation with full feature parity to Result[T,E]

**Complexity**: Medium-High
**Estimated Effort**: 1-2 days (8-16 hours)
**Risk Level**: Medium (requires careful AST manipulation and go/types integration)

---

## Part 1: Problem Analysis

### Fix A4: Literal Handling

**Problem**: Taking address of literals is invalid Go syntax.

```go
// CURRENT (BROKEN):
Ok(42) → Result_int_error{tag: ResultTag_Ok, ok_0: &42}  // ❌ SYNTAX ERROR

// DESIRED:
Ok(42) → func() Result_int_error {
    __tmp0 := 42
    return Result_int_error{tag: ResultTag_Ok, ok_0: &__tmp0}
}()
```

**Root Cause**:
- `/Users/jack/mag/dingo/pkg/plugin/builtin/result_type.go:195-198`
- Constructor transformation directly wraps argument in `&ast.UnaryExpr{Op: token.AND}`
- No detection of literal values vs addressable expressions

**Impact**: Medium
- Prevents Ok()/Err() constructors from working with literals
- Affects 8 deferred tests in builtin plugin suite
- Blocks Option[T] implementation (same issue)

**Detection Strategy**:
Need to identify non-addressable expressions:
- Basic literals: `42`, `"string"`, `true`, `3.14`
- Composite literals: `User{name: "Bob"}`, `[]int{1,2,3}`
- Binary expressions: `x + y`, `a * b`
- Unary expressions: `!flag`, `-value`
- Call expressions: `func()`

Addressable expressions:
- Identifiers: `x`, `user`
- Index expressions: `arr[i]`, `m[key]`
- Selector expressions: `user.Name`
- Dereference: `*ptr`

### Fix A5: Enhanced Type Inference

**Problem**: Type inference falls back to `interface{}` too often, causing type safety loss.

```go
// CURRENT:
Ok(x) where x is an identifier
  → Infers type as "interface{}" (fallback)
  → Generates Result_interface_error instead of Result_int_error

// DESIRED:
Ok(x) where x: int
  → Uses go/types to determine x is int
  → Generates Result_int_error
```

**Root Cause**:
- `/Users/jack/mag/dingo/pkg/plugin/builtin/result_type.go:252-256`
- `inferTypeFromExpr()` only handles basic literals and returns `interface{}` for identifiers
- No integration with `go/types` package for semantic analysis

**Impact**: High
- Reduces type safety (defeats purpose of Result[T,E])
- Generates incorrect Result types (Result_interface_error vs Result_int_error)
- Prevents proper type checking and autocomplete in IDEs
- Affects all constructor calls with non-literal arguments

**go/types Integration Points**:

The infrastructure partially exists:
- `TypeInferenceService` in `/Users/jack/mag/dingo/pkg/plugin/builtin/type_inference.go`
- Pipeline context support in `/Users/jack/mag/dingo/pkg/plugin/plugin.go:36-44`

**Missing pieces**:
1. Type checker initialization (parse → type-check → cache `types.Info`)
2. Propagate `types.Info` to Result/Option plugins
3. Use `types.Info.Types[expr]` instead of heuristic inference

### Option[T] Type Implementation

**Problem**: Option[T] plugin exists but is incomplete.

**Current State** (`/Users/jack/mag/dingo/pkg/plugin/builtin/option_type.go`):
- ✅ Basic structure exists (618 lines)
- ✅ Some() constructor detection
- ✅ None detection
- ❌ Literal handling (same as Fix A4)
- ❌ Type inference (same as Fix A5)
- ❌ Helper methods incomplete

**Golden Test Requirements** (`/Users/jack/mag/dingo/tests/golden/option_01_basic.{dingo,go.golden}`):
- Enum-style Option type definition
- Constructor functions: `Option_Some(value)`, `Option_None()`
- Helper methods: `IsSome()`, `IsNone()`
- Field access: `*result.some_0`

**Key Difference from Result[T,E]**:
- Single type parameter (T) vs two (T, E)
- None variant has no associated data
- Constructor syntax: `Some(value)` and `None` (constant, not function)

---

## Part 2: Proposed Solutions

### Solution A4: Temporary Variable Generation (IIFE Pattern)

**Strategy**: Use Immediately Invoked Function Expression (IIFE) pattern for non-addressable values.

**Implementation Approach**:

1. **Detection Phase** - Classify expressions as addressable or non-addressable
2. **Conditional Wrapping** - Only wrap non-addressable in IIFE
3. **Type Preservation** - Return type matches Result type

**Code Changes**:

**File**: `/Users/jack/mag/dingo/pkg/plugin/builtin/result_type.go`

**New Function** (insert after line 200):
```go
// isAddressable determines if an expression can have its address taken
func (p *ResultTypePlugin) isAddressable(expr ast.Expr) bool {
    switch expr.(type) {
    // Addressable
    case *ast.Ident:        // x, user, name
        return true
    case *ast.IndexExpr:    // arr[i], m[key]
        return true
    case *ast.SelectorExpr: // user.Name, pkg.Var
        return true
    case *ast.StarExpr:     // *ptr
        return true

    // Non-addressable
    case *ast.BasicLit:     // 42, "string", true
        return false
    case *ast.CompositeLit: // User{}, []int{1,2}
        return false
    case *ast.BinaryExpr:   // x + y, a * b
        return false
    case *ast.UnaryExpr:    // !flag, -value
        return false
    case *ast.CallExpr:     // func()
        return false

    default:
        // Conservative: assume non-addressable
        return false
    }
}

// wrapInTemporaryVariable wraps a non-addressable expression in IIFE
// Example: 42 → func() *int { __tmp := 42; return &__tmp }()
func (p *ResultTypePlugin) wrapInTemporaryVariable(expr ast.Expr, typeName string) ast.Expr {
    // Generate unique temp variable name
    tmpVar := fmt.Sprintf("__tmp%d", p.ctx.TempVarCounter)
    p.ctx.TempVarCounter++

    // Create IIFE that assigns value to temp var and returns its address
    iife := &ast.CallExpr{
        Fun: &ast.FuncLit{
            Type: &ast.FuncType{
                Params: &ast.FieldList{},
                Results: &ast.FieldList{
                    List: []*ast.Field{{
                        Type: &ast.StarExpr{
                            X: ast.NewIdent(typeName),
                        },
                    }},
                },
            },
            Body: &ast.BlockStmt{
                List: []ast.Stmt{
                    // __tmp := expr
                    &ast.AssignStmt{
                        Lhs: []ast.Expr{ast.NewIdent(tmpVar)},
                        Tok: token.DEFINE,
                        Rhs: []ast.Expr{expr},
                    },
                    // return &__tmp
                    &ast.ReturnStmt{
                        Results: []ast.Expr{
                            &ast.UnaryExpr{
                                Op: token.AND,
                                X:  ast.NewIdent(tmpVar),
                            },
                        },
                    },
                },
            },
        },
    }

    return iife
}
```

**Modified Function** (replace lines 156-200):
```go
func (p *ResultTypePlugin) transformOkConstructor(call *ast.CallExpr) ast.Expr {
    if len(call.Args) != 1 {
        p.ctx.Logger.Warn("Ok() expects exactly one argument, found %d", len(call.Args))
        return call
    }

    valueArg := call.Args[0]
    okType := p.inferTypeFromExpr(valueArg)
    errType := "error"

    resultTypeName := fmt.Sprintf("Result_%s_%s",
        p.sanitizeTypeName(okType),
        p.sanitizeTypeName(errType))

    if !p.emittedTypes[resultTypeName] {
        p.emitResultDeclaration(okType, errType, resultTypeName)
        p.emittedTypes[resultTypeName] = true
    }

    p.ctx.Logger.Debug("Transforming Ok(%s) → %s{...}", okType, resultTypeName)

    // FIX A4: Handle non-addressable expressions
    var okValue ast.Expr
    if p.isAddressable(valueArg) {
        // Directly take address
        okValue = &ast.UnaryExpr{
            Op: token.AND,
            X:  valueArg,
        }
    } else {
        // Wrap in IIFE to create addressable temporary
        okValue = p.wrapInTemporaryVariable(valueArg, okType)
    }

    return &ast.CompositeLit{
        Type: ast.NewIdent(resultTypeName),
        Elts: []ast.Expr{
            &ast.KeyValueExpr{
                Key:   ast.NewIdent("tag"),
                Value: ast.NewIdent("ResultTag_Ok"),
            },
            &ast.KeyValueExpr{
                Key:   ast.NewIdent("ok_0"),
                Value: okValue, // Now handles both addressable and non-addressable
            },
        },
    }
}
```

**Same changes apply to**:
- `transformErrConstructor()` (lines 201-240)
- `/Users/jack/mag/dingo/pkg/plugin/builtin/option_type.go:handleSomeConstructor()`

**Context Enhancement**:
Add `TempVarCounter` to `/Users/jack/mag/dingo/pkg/plugin/plugin.go:Context`:
```go
type Context struct {
    FileSet         *token.FileSet
    Logger          Logger
    TypeInference   interface{} // TypeInferenceService
    TempVarCounter  int         // NEW: Counter for unique temp variable names
}
```

### Solution A5: go/types Integration

**Strategy**: Run go/types type checker before plugin pipeline, cache results in Context.

**Architecture**:

```
Generator.Generate(file)
    ↓
1. Parse .dingo → AST (preprocessor + go/parser)
    ↓
2. NEW: Run go/types type checker
    - types.Config.Check(pkg, fset, []*ast.File{file}, typesInfo)
    - Populate types.Info with type information
    ↓
3. Create TypeInferenceService with types.Info
    ↓
4. Inject into plugin Context
    ↓
5. Plugin Pipeline Transform
    - Plugins access ctx.TypeInference.InferType(expr)
    - Returns accurate types.Type instead of string heuristics
    ↓
6. Generate Go code
```

**Implementation**:

**File**: `/Users/jack/mag/dingo/pkg/generator/generator.go`

**New Function** (insert before `Generate`):
```go
// runTypeChecker runs go/types type checker on the AST
// Returns populated types.Info or error
func (g *Generator) runTypeChecker(file *ast.File) (*types.Info, error) {
    info := &types.Info{
        Types:      make(map[ast.Expr]types.TypeAndValue),
        Defs:       make(map[*ast.Ident]types.Object),
        Uses:       make(map[*ast.Ident]types.Object),
        Implicits:  make(map[ast.Node]types.Object),
        Selections: make(map[*ast.SelectorExpr]*types.Selection),
        Scopes:     make(map[ast.Node]*types.Scope),
    }

    conf := types.Config{
        Importer: importer.Default(), // Use default Go importer
        Error: func(err error) {
            // Collect errors but don't fail
            // Some Dingo syntax may not type-check yet
            g.logger.Debug("Type checker warning: %v", err)
        },
    }

    // Type-check the file
    pkg := types.NewPackage(file.Name.Name, "")
    _, err := conf.Check(pkg.Path(), g.fset, []*ast.File{file}, info)

    // Don't fail on type errors - we still want to proceed
    // Type errors might be due to Dingo-specific syntax
    if err != nil {
        g.logger.Warn("Type checking completed with errors: %v", err)
    }

    return info, nil
}
```

**Modified Function** (update `Generate` around line 80-120):
```go
func (g *Generator) Generate(dingoFile string) (string, error) {
    // ... existing parsing code ...

    // NEW: Run type checker
    typesInfo, err := g.runTypeChecker(file)
    if err != nil {
        g.logger.Warn("Type inference may be limited: %v", err)
    }

    // Create TypeInferenceService with type information
    typeInference, err := builtin.NewTypeInferenceService(g.fset, file, typesInfo, g.logger)
    if err != nil {
        return "", fmt.Errorf("type inference service creation failed: %w", err)
    }

    // Update pipeline context
    g.pipeline.Ctx.TypeInference = typeInference

    // ... rest of generation ...
}
```

**File**: `/Users/jack/mag/dingo/pkg/plugin/builtin/type_inference.go`

**Modified Function** (update `NewTypeInferenceService` signature):
```go
func NewTypeInferenceService(
    fset *token.FileSet,
    file *ast.File,
    typesInfo *types.Info, // NEW parameter
    logger plugin.Logger,
) (*TypeInferenceService, error) {
    if logger == nil {
        logger = plugin.NewNoOpLogger()
    }

    return &TypeInferenceService{
        fset:            fset,
        file:            file,
        typesInfo:       typesInfo, // Store it
        logger:          logger,
        resultTypeCache: make(map[string]*ResultTypeInfo),
        optionTypeCache: make(map[string]*OptionTypeInfo),
        registry:        NewTypeRegistry(),
    }, nil
}

// Add field to struct:
type TypeInferenceService struct {
    fset      *token.FileSet
    file      *ast.File
    typesInfo *types.Info // NEW: Type information from go/types
    logger    plugin.Logger
    // ... rest unchanged
}
```

**New Method** (add to TypeInferenceService):
```go
// InferType infers the type of an expression using go/types
// Returns types.Type or nil if inference fails
func (s *TypeInferenceService) InferType(expr ast.Expr) types.Type {
    if s.typesInfo == nil || s.typesInfo.Types == nil {
        s.logger.Debug("Type inference unavailable (types.Info not initialized)")
        return nil
    }

    if tv, ok := s.typesInfo.Types[expr]; ok && tv.Type != nil {
        return tv.Type
    }

    s.logger.Debug("Type inference failed for expression: %T", expr)
    return nil
}

// TypeToString converts types.Type to string representation
// Handles pointers, slices, maps, etc.
func (s *TypeInferenceService) TypeToString(t types.Type) string {
    if t == nil {
        return "interface{}"
    }
    return t.String()
}
```

**File**: `/Users/jack/mag/dingo/pkg/plugin/builtin/result_type.go`

**Modified Function** (update `inferTypeFromExpr` lines 241-290):
```go
func (p *ResultTypePlugin) inferTypeFromExpr(expr ast.Expr) string {
    // FIX A5: Try go/types inference first
    if typeInf, ok := p.ctx.GetTypeInference(); ok {
        if t := typeInf.InferType(expr); t != nil {
            return typeInf.TypeToString(t)
        }
    }

    // Fallback to heuristic inference
    switch e := expr.(type) {
    case *ast.BasicLit:
        switch e.Kind {
        case token.INT:
            return "int"
        case token.STRING:
            return "string"
        case token.FLOAT:
            return "float64"
        // ... rest unchanged
        }
    case *ast.Ident:
        // Before: returned "interface{}" immediately
        // After: Already tried go/types above, so this is a fallback
        p.ctx.Logger.Warn("Type inference fallback for identifier '%s'", e.Name)
        return "interface{}"
    // ... rest unchanged
    }

    return "interface{}"
}
```

**Same changes apply to**: `/Users/jack/mag/dingo/pkg/plugin/builtin/option_type.go`

### Solution Option[T]: Complete Implementation

**Strategy**: Mirror Result[T,E] implementation with Option[T]-specific adaptations.

**Key Differences**:
1. Single type parameter: `Option[T]` vs `Result[T,E]`
2. None variant: No data field, singleton-like behavior
3. Constructor: `None` constant vs `None()` function call

**Implementation Tasks**:

**Task 1**: Apply Fix A4 to Option plugin
- Add `isAddressable()` and `wrapInTemporaryVariable()` methods
- Modify `handleSomeConstructor()` to use conditional wrapping

**Task 2**: Apply Fix A5 to Option plugin
- Modify `inferTypeFromExpr()` to use TypeInferenceService
- Same integration pattern as Result plugin

**Task 3**: Complete helper methods
- IsSome(), IsNone() - Already exist in golden test expectations
- Unwrap(), UnwrapOr(default) - Deferred (Phase 4)
- Map(), Filter() - Deferred (Phase 4, functional utilities)

**Task 4**: Golden tests
- Fix `option_01_basic.dingo` to pass with new implementation
- Create `option_03_literals.dingo` for Fix A4 verification
- Create `option_04_type_inference.dingo` for Fix A5 verification

**File Changes**:

**File**: `/Users/jack/mag/dingo/pkg/plugin/builtin/option_type.go`

Apply same patterns as Result plugin:
1. Copy `isAddressable()` method
2. Copy `wrapInTemporaryVariable()` method (adjust for Option type)
3. Update `handleSomeConstructor()` (lines 115-180)
4. Update `inferTypeFromExpr()` to use TypeInferenceService
5. Ensure `emitOptionDeclaration()` generates correct struct/methods

**None Handling**: Special case
```go
// None is a singleton constant, not a function call
// Current: Detect *ast.Ident with Name == "None"
// Challenge: Need type context to know which Option_T_None() to call

// Solution: Defer None handling to Phase 4
// For now: Require explicit Option_None() function call syntax
```

---

## Part 3: Implementation Strategy

### Parallel vs Sequential

**Parallelizable Tasks** (can be done simultaneously):
1. Fix A4 implementation (addressability detection + IIFE wrapping)
2. Fix A5 implementation (go/types integration + TypeInferenceService)
3. Option[T] basic structure (assuming Fixes A4/A5 exist as library functions)

**Sequential Dependencies**:
1. Fix A4 → Option[T] Some() constructor
2. Fix A5 → Option[T] type inference
3. Both fixes → Option[T] golden tests

**Recommended Order**:
1. **Session 1** (4-6 hours): Fix A5 (go/types integration)
   - Highest impact (affects all type inference)
   - Enables accurate testing of other features
2. **Session 2** (2-3 hours): Fix A4 (literal handling)
   - Unblocks constructor usage
   - Required for Option[T] constructors
3. **Session 3** (2-4 hours): Option[T] implementation
   - Apply both fixes to Option plugin
   - Write golden tests
   - Integration testing

**Why this order?**
- Fix A5 provides infrastructure (TypeInferenceService) used everywhere
- Fix A4 is localized to constructor transformation
- Option[T] consumes both fixes

### File-by-File Changes

**High Priority (Core Fixes)**:

| File | Changes | Lines | Risk |
|------|---------|-------|------|
| `pkg/plugin/builtin/type_inference.go` | Add typesInfo field, InferType(), TypeToString() | +50 | Low |
| `pkg/generator/generator.go` | Add runTypeChecker(), integrate into pipeline | +60 | Medium |
| `pkg/plugin/plugin.go` | Add TempVarCounter to Context, GetTypeInference() | +15 | Low |
| `pkg/plugin/builtin/result_type.go` | Add isAddressable(), wrapInTemporaryVariable(), update inferTypeFromExpr() | +120 | Medium |
| `pkg/plugin/builtin/result_type.go` | Modify transformOkConstructor(), transformErrConstructor() | ~40 | Medium |

**Medium Priority (Option[T])**:

| File | Changes | Lines | Risk |
|------|---------|-------|------|
| `pkg/plugin/builtin/option_type.go` | Apply Fix A4 (addressability) | +100 | Medium |
| `pkg/plugin/builtin/option_type.go` | Apply Fix A5 (type inference) | +30 | Low |
| `pkg/plugin/builtin/option_type.go` | Complete helper methods (IsSome, IsNone) | +40 | Low |

**Testing**:

| File | Changes | Lines | Risk |
|------|---------|-------|------|
| `pkg/plugin/builtin/result_type_test.go` | Update tests for Fix A4/A5 | +150 | Low |
| `pkg/plugin/builtin/option_type_test.go` | Add comprehensive test coverage | +200 | Low |
| `tests/golden/result_03_literals.dingo` | NEW: Test literal handling | +30 | Low |
| `tests/golden/option_03_literals.dingo` | NEW: Test Option literal handling | +30 | Low |

**Total Estimated Changes**: ~850 lines (400 production, 450 tests)

---

## Part 4: Testing Strategy

### Unit Tests

**Fix A4 Tests** (`pkg/plugin/builtin/result_type_test.go`):
```go
func TestConstructor_LiteralHandling(t *testing.T) {
    tests := []struct {
        name     string
        code     string
        expected string // Check for IIFE pattern
    }{
        {
            name: "integer literal",
            code: `Ok(42)`,
            expected: `func() *int { __tmp0 := 42; return &__tmp0 }()`,
        },
        {
            name: "string literal",
            code: `Ok("hello")`,
            expected: `func() *string { __tmp1 := "hello"; return &__tmp1 }()`,
        },
        {
            name: "composite literal",
            code: `Ok(User{name: "Bob"})`,
            expected: `func() *User { __tmp2 := User{name: "Bob"}; return &__tmp2 }()`,
        },
        {
            name: "addressable identifier",
            code: `Ok(x)`, // x is a variable
            expected: `&x`, // Direct address, no IIFE
        },
    }
    // Test implementation...
}
```

**Fix A5 Tests** (`pkg/plugin/builtin/type_inference_test.go`):
```go
func TestTypeInference_GoTypes(t *testing.T) {
    src := `
    package main
    func test() {
        x := 42
        result := Ok(x) // Should infer Result_int_error, not Result_interface_error
    }
    `

    // Parse → Type-check → Verify Result type name
    // Assert: resultTypeName == "Result_int_error"
}
```

**Option[T] Tests** (`pkg/plugin/builtin/option_type_test.go`):
- Mirror Result tests
- Test Some(literal) with IIFE pattern
- Test type inference with identifiers
- Test None handling (deferred or basic)

### Integration Tests

**Golden Test Coverage**:

| Test File | Feature | Pass Criteria |
|-----------|---------|---------------|
| `result_03_literals.dingo` | Fix A4 for Result | Compiles, runs, correct output |
| `option_01_basic.dingo` | Basic Option with fixes | Compiles, runs, correct output |
| `option_03_literals.dingo` | Fix A4 for Option | Compiles, runs, correct output |
| `option_04_type_inference.dingo` | Fix A5 for Option | Correct Option_T types generated |

**End-to-End Verification**:
```bash
# Test complete pipeline
go run ./cmd/dingo build tests/golden/result_03_literals.dingo
go build -o /tmp/test_result tests/golden/result_03_literals.go
/tmp/test_result
# Verify output matches expected

# Test Option[T]
go run ./cmd/dingo build tests/golden/option_01_basic.dingo
go build -o /tmp/test_option tests/golden/option_01_basic.go
/tmp/test_option
# Verify output matches expected
```

### Regression Tests

**Must Pass**:
- All 48/48 preprocessor tests
- All 31/39 builtin plugin tests (should become 39/39)
- All existing golden tests (no regressions)

**Success Criteria**:
- ✅ All 39 builtin plugin tests passing (currently 31/39)
- ✅ All golden tests passing (option_01, option_02, result_03)
- ✅ No regressions in existing tests
- ✅ End-to-end: .dingo → .go → compile → run (all succeed)

---

## Part 5: Risk Analysis

### Technical Risks

**Risk 1: go/types Integration Complexity**
- **Severity**: Medium
- **Probability**: Medium
- **Impact**: Type checking may fail on Dingo-specific syntax
- **Mitigation**:
  - Use graceful fallback (ignore type errors, proceed anyway)
  - Only use types.Info where it's successfully populated
  - Log warnings but don't fail pipeline
  - Phase approach: Basic integration first, refine later

**Risk 2: IIFE Pattern Performance**
- **Severity**: Low
- **Probability**: Low
- **Impact**: IIFE adds function call overhead
- **Mitigation**:
  - Only use IIFE for non-addressable expressions
  - Addressable expressions use direct `&x` (zero overhead)
  - Go compiler likely inlines IIFE anyway
  - Benchmark if concerns arise

**Risk 3: Type Inference Accuracy**
- **Severity**: Medium
- **Probability**: Medium
- **Impact**: Wrong Result types generated (Result_interface_error)
- **Mitigation**:
  - go/types provides ground truth
  - Fallback to heuristics only when go/types unavailable
  - Extensive unit tests for edge cases
  - Clear error messages when inference fails

**Risk 4: AST Manipulation Bugs**
- **Severity**: High
- **Probability**: Low
- **Impact**: Invalid Go code generated, compilation fails
- **Mitigation**:
  - Extensive golden tests
  - Each change tested in isolation
  - Use go/ast/astutil for safe manipulation
  - Validate generated code compiles

### Integration Risks

**Risk 5: Plugin Pipeline Ordering**
- **Severity**: Medium
- **Probability**: Low
- **Impact**: Plugins run in wrong order, transformations conflict
- **Current State**: Phase 2.6.1 fixed plugin ordering crash
- **Mitigation**:
  - Explicit dependencies already implemented
  - Result/Option plugins have no dependencies on other plugins
  - Run type checker BEFORE plugin pipeline

**Risk 6: Preprocessor + Plugin Interaction**
- **Severity**: Medium
- **Probability**: Medium
- **Impact**: Preprocessor changes AST, breaks plugin expectations
- **Mitigation**:
  - Preprocessor runs first (Stage 1)
  - Plugins operate on go/parser AST (Stage 2)
  - Clear separation of concerns
  - Test preprocessor + plugin integration

### Timeline Risks

**Risk 7: Underestimation of Effort**
- **Severity**: Low
- **Probability**: Medium
- **Impact**: Takes 2-3 days instead of 1-2
- **Mitigation**:
  - Phased approach (can ship Fix A5 independently)
  - Clear stopping points (Fix A4, Fix A5, Option[T])
  - Defer non-critical features (None constant, advanced helpers)

---

## Part 6: Alternative Approaches Considered

### Alternative A4-1: Copy Values Instead of Pointers

**Approach**: Store values directly in Result struct instead of pointers.

```go
type Result_int_error struct {
    tag    ResultTag
    ok_0   int    // Value, not pointer
    err_0  error  // error is already a pointer type
}
```

**Pros**:
- No need for addressability checks
- Simpler code generation
- Slightly better performance (no pointer indirection)

**Cons**:
- REJECTED: Zero-value problem
  - How to distinguish None/Err case from zero value of T?
  - If ok_0 == 0, is it Ok(0) or Err(error)?
  - Pointers provide explicit nil check
- Breaks existing Phase 2 architecture
- All Phase 2 tests expect pointer semantics

**Decision**: REJECTED - Pointers are essential for zero-value safety.

### Alternative A4-2: Inline Assignment Pattern

**Approach**: Generate assignment statement instead of IIFE.

```go
// Instead of IIFE:
__tmp0 := 42
result := Result_int_error{tag: ResultTag_Ok, ok_0: &__tmp0}
```

**Pros**:
- Simpler than IIFE
- More readable generated code
- No function call overhead

**Cons**:
- REJECTED: Doesn't work in expression context
  - `return Ok(42)` requires expression, not statements
  - Would need statement lifting (same complexity as IIFE)
- Pollutes namespace with temp variables
- IIFE is self-contained

**Decision**: REJECTED - IIFE is more robust and handles all contexts.

### Alternative A5-1: Heuristic Inference Only

**Approach**: Improve heuristics instead of using go/types.

```go
// Better heuristics:
// - Look at variable declarations: var x int = 42 → infer x is int
// - Look at function signatures: func() int → infer return is int
// - Pattern matching on AST structure
```

**Pros**:
- No go/types dependency
- Simpler implementation
- Faster (no type checking)

**Cons**:
- REJECTED: Fundamentally limited
  - Cannot handle complex cases (type aliases, imports, etc.)
  - Cannot handle: `x := someFunc()` (need to know someFunc's return type)
  - Maintenance burden (re-implementing type system)
- go/types is standard library, well-tested
- Heuristics will always have edge cases

**Decision**: REJECTED - go/types is the proper solution.

### Alternative A5-2: Two-Pass Plugin Pipeline

**Approach**: Run plugins twice - first pass for type inference, second for transformation.

**Pros**:
- Plugins can register types in first pass
- Type information available for second pass

**Cons**:
- REJECTED: Overcomplicated
  - go/types already provides this (it IS a type-checking pass)
  - Duplicates work go/types does better
  - Increases pipeline complexity
- Already have Discovery → Transform → Inject phases

**Decision**: REJECTED - go/types integration is cleaner.

---

## Part 7: Implementation Checklist

### Phase 3.1: Fix A5 (go/types Integration) - 4-6 hours

**File**: `pkg/plugin/builtin/type_inference.go`
- [ ] Add `typesInfo *types.Info` field to TypeInferenceService
- [ ] Update NewTypeInferenceService signature to accept typesInfo
- [ ] Implement InferType(expr) method using typesInfo.Types[expr]
- [ ] Implement TypeToString(types.Type) helper
- [ ] Add unit tests for InferType() with various expression types

**File**: `pkg/generator/generator.go`
- [ ] Import `go/types` and `go/importer` packages
- [ ] Implement runTypeChecker(file) function
- [ ] Update Generate() to call runTypeChecker before plugin pipeline
- [ ] Pass typesInfo to NewTypeInferenceService
- [ ] Add error handling and logging for type checker failures

**File**: `pkg/plugin/builtin/result_type.go`
- [ ] Update inferTypeFromExpr() to try TypeInferenceService first
- [ ] Keep fallback heuristics for cases where go/types unavailable
- [ ] Update transformOkConstructor() to use improved inference
- [ ] Update transformErrConstructor() to use improved inference
- [ ] Add logging for type inference decisions

**File**: `pkg/plugin/builtin/option_type.go`
- [ ] Same changes as result_type.go for type inference

**Testing**:
- [ ] Unit test: TypeInferenceService.InferType() with basic literals
- [ ] Unit test: TypeInferenceService.InferType() with identifiers
- [ ] Unit test: InferType() with complex expressions (binary, call)
- [ ] Integration test: Verify Result_int_error vs Result_interface_error
- [ ] Regression test: All existing tests still pass

### Phase 3.2: Fix A4 (Literal Handling) - 2-3 hours

**File**: `pkg/plugin/plugin.go`
- [ ] Add TempVarCounter field to Context struct
- [ ] Initialize TempVarCounter to 0 in NewPipeline

**File**: `pkg/plugin/builtin/result_type.go`
- [ ] Implement isAddressable(expr) method with all cases
- [ ] Implement wrapInTemporaryVariable(expr, typeName) method
- [ ] Update transformOkConstructor() to use conditional wrapping
- [ ] Update transformErrConstructor() to use conditional wrapping
- [ ] Add position information to generated IIFE nodes

**File**: `pkg/plugin/builtin/option_type.go`
- [ ] Copy isAddressable() method (same implementation)
- [ ] Implement wrapInTemporaryVariable() for Option type
- [ ] Update handleSomeConstructor() to use conditional wrapping

**Testing**:
- [ ] Unit test: isAddressable() for all expression types
- [ ] Unit test: wrapInTemporaryVariable() generates correct IIFE
- [ ] Unit test: Ok(42) generates IIFE, Ok(x) uses direct address
- [ ] Golden test: result_03_literals.dingo compiles and runs
- [ ] Golden test: option_03_literals.dingo compiles and runs
- [ ] Regression test: Existing Result/Option tests still pass

### Phase 3.3: Option[T] Implementation - 2-4 hours

**File**: `pkg/plugin/builtin/option_type.go`
- [ ] Verify Fix A4 applied (addressability + IIFE)
- [ ] Verify Fix A5 applied (type inference)
- [ ] Ensure emitOptionDeclaration() generates correct structure
- [ ] Ensure helper methods (IsSome, IsNone) are generated
- [ ] Test Some() constructor with literals and identifiers
- [ ] Document None handling limitation (defer to Phase 4)

**File**: `tests/golden/option_01_basic.dingo`
- [ ] Update test to use explicit Option_None() syntax (if needed)
- [ ] Verify test passes with new implementation

**File**: `tests/golden/option_03_literals.dingo` (NEW)
- [ ] Create test with Some(42), Some("string"), Some(User{})
- [ ] Write corresponding .go.golden file
- [ ] Verify test passes

**File**: `tests/golden/option_04_type_inference.dingo` (NEW)
- [ ] Create test with Some(variable) where type inferred from context
- [ ] Verify Option_int vs Option_interface naming

**Testing**:
- [ ] Unit test: Option plugin with all expression types
- [ ] Integration test: option_01_basic compiles and runs
- [ ] Integration test: option_03_literals compiles and runs
- [ ] Integration test: option_04_type_inference generates correct types
- [ ] Full test suite: go test ./... (all pass)

### Phase 3.4: Final Integration & Documentation - 1-2 hours

**Testing**:
- [ ] Run full test suite: `go test ./...`
- [ ] Verify all 39/39 builtin plugin tests pass (up from 31/39)
- [ ] Run all golden tests: `go test ./tests/`
- [ ] End-to-end test: Build and run 5+ golden test files
- [ ] Performance check: Ensure no significant slowdown

**Documentation**:
- [ ] Update CHANGELOG.md with Phase 3 changes
- [ ] Update ai-docs/ARCHITECTURE.md with go/types integration
- [ ] Add comments to new code explaining Fix A4/A5
- [ ] Create reasoning.md for new golden tests

**Cleanup**:
- [ ] Remove debug logging (or reduce to appropriate level)
- [ ] Remove commented-out code
- [ ] Format all code: `go fmt ./...`
- [ ] Run linter: `golangci-lint run` (if configured)

---

## Part 8: Success Metrics

### Quantitative Metrics

| Metric | Before | After | Target |
|--------|--------|-------|--------|
| Builtin plugin tests passing | 31/39 (79%) | ? | 39/39 (100%) |
| Golden tests passing | ~15/46 | ? | ~20/46 (+5) |
| Type inference accuracy | ~40% (many interface{}) | ? | >90% |
| Literal constructor support | 0% (all fail) | ? | 100% |

### Qualitative Metrics

**Code Quality**:
- [ ] Generated Go code is idiomatic and readable
- [ ] No compiler warnings in generated code
- [ ] Type safety improved (fewer interface{} fallbacks)
- [ ] Clear error messages when inference fails

**Developer Experience**:
- [ ] Ok(42) works as expected (no manual temp variables)
- [ ] Type inference is accurate and predictable
- [ ] Option[T] has feature parity with Result[T,E]
- [ ] Clear documentation of limitations (None constant)

**Completeness**:
- [ ] All Fix A4 requirements met
- [ ] All Fix A5 requirements met
- [ ] Option[T] basic functionality complete
- [ ] Foundation ready for Phase 4 (pattern matching, helpers)

---

## Part 9: Dependencies and Imports

### New Imports Required

**File**: `pkg/generator/generator.go`
```go
import (
    "go/types"
    "go/importer"
    // ... existing imports
)
```

**File**: `pkg/plugin/builtin/type_inference.go`
```go
import (
    "go/types"
    // ... existing imports
)
```

### External Dependencies

**go/types**: Standard library, no new dependency
**go/importer**: Standard library, no new dependency
**go/ast**: Already imported
**go/token**: Already imported

**No new external dependencies required** ✅

---

## Part 10: Rollback Plan

### If Fix A5 Fails

**Symptoms**:
- Type checker crashes on Dingo syntax
- Performance degradation (>2x slower)
- Type inference worse than before

**Rollback**:
1. Revert generator.go changes
2. Keep heuristic-only inference
3. Defer go/types to Phase 4
4. Still proceed with Fix A4 and Option[T]

**Impact**: Type inference limited, but constructors still work

### If Fix A4 Fails

**Symptoms**:
- IIFE generation breaks AST
- Generated code doesn't compile
- Test failures cascade

**Rollback**:
1. Revert isAddressable() and wrapInTemporaryVariable()
2. Document limitation: "Use variables, not literals"
3. Update golden tests to use variables
4. Still proceed with Fix A5 and Option[T] (partial)

**Impact**: Constructors require manual temp variables

### If Option[T] Fails

**Symptoms**:
- Option plugin conflicts with Result plugin
- Test failures in Option suite

**Rollback**:
1. Disable Option plugin in default registry
2. Mark Option[T] as experimental
3. Keep Fix A4 and Fix A5 (benefit Result[T,E])
4. Defer Option[T] to Phase 4

**Impact**: Only Result[T,E] available, Option[T] delayed

---

## Conclusion

Phase 3 addresses critical foundation issues (Fix A4, Fix A5) and completes Option[T] implementation. The phased approach allows independent delivery of each fix, with clear rollback plans if issues arise.

**Next Steps**:
1. Review this plan with team
2. Address questions in gaps.json
3. Proceed with implementation in order: A5 → A4 → Option[T]
4. Continuous testing and validation

**Estimated Timeline**: 1-2 days (8-16 hours)
**Confidence Level**: High (clear requirements, proven patterns, fallback options)
