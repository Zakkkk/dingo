# Phase 3 Final Implementation Plan
**Date**: 2025-11-18
**Session**: 20251118-114514
**Status**: APPROVED - Ready for Implementation

---

## Executive Summary

Phase 3 is a comprehensive update that completes the Result[T,E]/Option[T] foundation with full feature parity. Based on user clarifications, we will implement:

1. **Fix A5**: Enhanced type inference using go/types (4-6 hours)
2. **Fix A4**: Literal handling with IIFE pattern (2-3 hours)
3. **Option[T]**: Complete implementation including type-context-aware None constant (6-8 hours)
4. **Helper Methods**: Full suite for both Result and Option (Map, Filter, AndThen, etc.) (4-6 hours)
5. **Error Infrastructure**: Clear compile errors on type inference failure (2-3 hours)

**Total Estimated Effort**: 18-26 hours over 2-3 days
**Risk Level**: Medium-High (complex features, significant scope)
**Target**: All 39/39 builtin plugin tests passing, ~25/46 golden tests passing

---

## User Clarifications Incorporated

### 1. Type-Context-Aware None Constant (Complex)
- Implement proper constant detection that infers `Option_T` type from context
- Example: `var x Option_int = None` → infer None should be `Option_int`
- Requires context-based type inference, not just expression analysis
- More complex than simple `Option_None()` function call

### 2. Complete Helper Methods in Phase 3 (Expanded Scope)
- Implement ALL expected helper methods:
  - Result[T,E]: Map, Filter, AndThen, Unwrap, UnwrapOr, UnwrapOrElse, IsOk, IsErr
  - Option[T]: Map, Filter, AndThen, Unwrap, UnwrapOr, UnwrapOrElse, IsSome, IsNone
- This ensures 39/39 builtin test pass rate

### 3. Generate Compile Error on Type Inference Failure (Strict)
- When type inference completely fails (no go/types, no heuristics):
  - Generate invalid Go code with descriptive error comment
  - Suggest explicit type annotation
  - Fail fast rather than silent interface{} fallback
- Requires enhanced error message infrastructure

---

## Parallel Execution Strategy

### Batch 1: Foundation Infrastructure (Parallel - 4-6 hours)

These tasks are independent and can be executed simultaneously by different developers or sessions.

#### Batch 1a: Type Inference Infrastructure
**Agent**: golang-developer
**Complexity**: Medium
**Files Modified**:
- `/Users/jack/mag/dingo/pkg/plugin/builtin/type_inference.go`
- `/Users/jack/mag/dingo/pkg/generator/generator.go`

**Changes**:
1. Add `typesInfo *types.Info` field to TypeInferenceService
2. Implement `InferType(expr) types.Type` method
3. Implement `TypeToString(types.Type) string` helper
4. Implement `runTypeChecker(file) (*types.Info, error)` in generator.go
5. Integrate type checker into Generate() pipeline
6. Add unit tests for InferType() method

**Dependencies**: None
**Deliverable**: TypeInferenceService can query go/types for accurate type info

#### Batch 1b: Error Infrastructure
**Agent**: golang-developer
**Complexity**: Simple
**Files Modified**:
- `/Users/jack/mag/dingo/pkg/plugin/plugin.go`
- `/Users/jack/mag/dingo/pkg/errors/` (NEW package)

**Changes**:
1. Create error types for type inference failures
2. Add error reporting mechanism to Context
3. Implement compile error generation (invalid Go + comment)
4. Add TempVarCounter to Context (for Fix A4)

**Dependencies**: None
**Deliverable**: Plugins can report clear errors on type inference failure

#### Batch 1c: Addressability Detection (Fix A4 Foundation)
**Agent**: golang-developer
**Complexity**: Simple-Medium
**Files Modified**:
- `/Users/jack/mag/dingo/pkg/plugin/builtin/addressability.go` (NEW file)

**Changes**:
1. Create shared addressability detection module
2. Implement `isAddressable(expr) bool` with all cases
3. Implement `wrapInTemporaryVariable(expr, typeName, ctx) ast.Expr`
4. Add comprehensive unit tests

**Dependencies**: Batch 1b (needs TempVarCounter)
**Deliverable**: Reusable IIFE wrapping logic for Result and Option plugins

---

### Batch 2: Core Plugin Updates (Sequential - 6-8 hours)

These depend on Batch 1 completion. Can be split into two parallel tracks (Result vs Option).

#### Batch 2a: Result[T,E] Plugin - Fix A5 + Fix A4
**Agent**: golang-developer
**Complexity**: Medium
**Files Modified**:
- `/Users/jack/mag/dingo/pkg/plugin/builtin/result_type.go`

**Changes**:
1. Update `inferTypeFromExpr()` to use TypeInferenceService (Fix A5)
2. Add fallback error reporting on inference failure
3. Update `transformOkConstructor()` to use addressability check (Fix A4)
4. Update `transformErrConstructor()` to use addressability check (Fix A4)
5. Add logging for type inference decisions
6. Update unit tests for both fixes

**Dependencies**: Batch 1a (TypeInferenceService), Batch 1c (addressability)
**Deliverable**: Result[T,E] constructors work with literals and have accurate type inference

#### Batch 2b: Option[T] Plugin - Fix A5 + Fix A4 + None Constant
**Agent**: golang-developer
**Complexity**: High
**Files Modified**:
- `/Users/jack/mag/dingo/pkg/plugin/builtin/option_type.go`

**Changes**:
1. Update `inferTypeFromExpr()` to use TypeInferenceService (Fix A5)
2. Update `handleSomeConstructor()` to use addressability check (Fix A4)
3. Implement type-context-aware None constant detection:
   - Detect `None` identifier in assignment/return context
   - Infer target Option_T type from left-hand side
   - Generate `Option_T{tag: OptionTag_None}` with correct type
4. Add comprehensive error reporting
5. Update unit tests

**Dependencies**: Batch 1a, Batch 1b, Batch 1c
**Deliverable**: Option[T] with working Some() and context-aware None constant

---

### Batch 3: Helper Methods (Parallel - 4-6 hours)

Can be split into two parallel tracks after Batch 2.

#### Batch 3a: Result[T,E] Helper Methods
**Agent**: golang-developer
**Complexity**: Medium
**Files Modified**:
- `/Users/jack/mag/dingo/pkg/plugin/builtin/result_type.go`

**Changes**:
1. Implement method generation in `emitResultDeclaration()`:
   - `IsOk() bool` - Check if tag == ResultTag_Ok
   - `IsErr() bool` - Check if tag == ResultTag_Err
   - `Unwrap() (T, error)` - Return ok_0 or err_0 based on tag
   - `UnwrapOr(defaultVal T) T` - Return ok_0 or defaultVal
   - `UnwrapOrElse(fn func(error) T) T` - Return ok_0 or fn(err_0)
2. Implement transformation methods (generic helpers):
   - `Map(fn func(T) U) Result[U,E]` - Transform Ok value
   - `MapErr(fn func(E) F) Result[T,F]` - Transform Err value
   - `AndThen(fn func(T) Result[U,E]) Result[U,E]` - Chain operations
   - `Filter(predicate func(T) bool, err E) Result[T,E]` - Filter Ok values
3. Add unit tests for each method
4. Create golden test: `result_05_helpers.dingo`

**Dependencies**: Batch 2a (Result plugin with fixes)
**Deliverable**: Result[T,E] has complete helper method suite

#### Batch 3b: Option[T] Helper Methods
**Agent**: golang-developer
**Complexity**: Medium
**Files Modified**:
- `/Users/jack/mag/dingo/pkg/plugin/builtin/option_type.go`

**Changes**:
1. Implement method generation in `emitOptionDeclaration()`:
   - `IsSome() bool` - Check if tag == OptionTag_Some
   - `IsNone() bool` - Check if tag == OptionTag_None
   - `Unwrap() T` - Return some_0 or panic
   - `UnwrapOr(defaultVal T) T` - Return some_0 or defaultVal
   - `UnwrapOrElse(fn func() T) T` - Return some_0 or fn()
2. Implement transformation methods:
   - `Map(fn func(T) U) Option[U]` - Transform Some value
   - `AndThen(fn func(T) Option[U]) Option[U]` - Chain operations
   - `Filter(predicate func(T) bool) Option[T]` - Filter Some values
3. Add unit tests for each method
4. Create golden test: `option_05_helpers.dingo`

**Dependencies**: Batch 2b (Option plugin with fixes)
**Deliverable**: Option[T] has complete helper method suite

---

### Batch 4: Integration & Testing (Sequential - 4-6 hours)

Final validation after all features implemented.

#### Batch 4a: Golden Test Updates
**Agent**: golang-tester
**Complexity**: Medium
**Files Modified/Created**:
- `/Users/jack/mag/dingo/tests/golden/result_03_literals.dingo` (NEW)
- `/Users/jack/mag/dingo/tests/golden/result_03_literals.go.golden` (NEW)
- `/Users/jack/mag/dingo/tests/golden/result_04_type_inference.dingo` (NEW)
- `/Users/jack/mag/dingo/tests/golden/result_04_type_inference.go.golden` (NEW)
- `/Users/jack/mag/dingo/tests/golden/result_05_helpers.dingo` (NEW)
- `/Users/jack/mag/dingo/tests/golden/result_05_helpers.go.golden` (NEW)
- `/Users/jack/mag/dingo/tests/golden/option_01_basic.dingo` (UPDATE)
- `/Users/jack/mag/dingo/tests/golden/option_01_basic.go.golden` (UPDATE)
- `/Users/jack/mag/dingo/tests/golden/option_03_literals.dingo` (NEW)
- `/Users/jack/mag/dingo/tests/golden/option_03_literals.go.golden` (NEW)
- `/Users/jack/mag/dingo/tests/golden/option_04_type_inference.dingo` (NEW)
- `/Users/jack/mag/dingo/tests/golden/option_04_type_inference.go.golden` (NEW)
- `/Users/jack/mag/dingo/tests/golden/option_05_helpers.dingo` (NEW)
- `/Users/jack/mag/dingo/tests/golden/option_05_helpers.go.golden` (NEW)

**Changes**:
1. Create comprehensive test cases for Fix A4 (literals)
2. Create test cases for Fix A5 (type inference accuracy)
3. Create test cases for all helper methods
4. Update option_01_basic to use None constant syntax
5. Write reasoning.md for each new test

**Dependencies**: Batches 2 and 3 (all features implemented)
**Deliverable**: Golden test coverage for all Phase 3 features

#### Batch 4b: Documentation & Cleanup
**Agent**: golang-architect
**Complexity**: Simple
**Files Modified**:
- `/Users/jack/mag/dingo/CHANGELOG.md`
- `/Users/jack/mag/dingo/ai-docs/ARCHITECTURE.md`
- `/Users/jack/mag/dingo/ai-docs/sessions/20251118-114514/completion-report.md` (NEW)

**Changes**:
1. Update CHANGELOG.md with Phase 3 changes
2. Document go/types integration in ARCHITECTURE.md
3. Add comments explaining Fix A4/A5 in code
4. Create completion report with metrics
5. Clean up debug logging
6. Format all code: `go fmt ./...`

**Dependencies**: Batch 4a (all tests passing)
**Deliverable**: Documentation complete, ready for commit

---

## Detailed Implementation Roadmap

### Milestone 1: Foundation Complete (Batch 1 - Day 1)
**Duration**: 4-6 hours
**Goal**: Infrastructure ready for plugin updates

**Success Criteria**:
- [ ] TypeInferenceService can query go/types
- [ ] runTypeChecker() integrates into generator pipeline
- [ ] Error reporting infrastructure functional
- [ ] Addressability detection module complete with tests
- [ ] TempVarCounter added to Context
- [ ] Unit tests: ~50 new tests, all passing

**Deliverables**:
- `pkg/plugin/builtin/type_inference.go` with InferType() method
- `pkg/generator/generator.go` with runTypeChecker()
- `pkg/plugin/builtin/addressability.go` with IIFE wrapping
- `pkg/errors/` package with type inference errors

### Milestone 2: Core Plugins Updated (Batch 2 - Day 1-2)
**Duration**: 6-8 hours
**Goal**: Result and Option plugins have Fix A4/A5 applied

**Success Criteria**:
- [ ] Result[T,E] constructors work with literals (Fix A4)
- [ ] Result[T,E] has accurate type inference (Fix A5)
- [ ] Option[T] constructors work with literals (Fix A4)
- [ ] Option[T] has accurate type inference (Fix A5)
- [ ] Option[T] None constant is type-context-aware
- [ ] Unit tests: Result plugin 20/20, Option plugin 15/15

**Deliverables**:
- `pkg/plugin/builtin/result_type.go` with both fixes
- `pkg/plugin/builtin/option_type.go` with both fixes + None handling
- Unit test coverage >90%

### Milestone 3: Helper Methods Complete (Batch 3 - Day 2)
**Duration**: 4-6 hours
**Goal**: All expected helper methods implemented

**Success Criteria**:
- [ ] Result[T,E] has 8 helper methods (IsOk, IsErr, Unwrap, UnwrapOr, UnwrapOrElse, Map, AndThen, Filter)
- [ ] Option[T] has 8 helper methods (IsSome, IsNone, Unwrap, UnwrapOr, UnwrapOrElse, Map, AndThen, Filter)
- [ ] All methods generate idiomatic Go code
- [ ] Unit tests: All 39/39 builtin plugin tests passing
- [ ] Golden tests: result_05_helpers and option_05_helpers pass

**Deliverables**:
- `pkg/plugin/builtin/result_type.go` with complete method suite
- `pkg/plugin/builtin/option_type.go` with complete method suite
- Golden test files demonstrating all helpers

### Milestone 4: Integration Complete (Batch 4 - Day 3)
**Duration**: 4-6 hours
**Goal**: All tests passing, documentation complete

**Success Criteria**:
- [ ] All 39/39 builtin plugin tests passing (up from 31/39)
- [ ] Golden tests: ~25/46 passing (up from ~15/46)
- [ ] No regressions in existing tests
- [ ] End-to-end: .dingo → .go → compile → run (all succeed)
- [ ] CHANGELOG.md and ARCHITECTURE.md updated
- [ ] Code formatted and cleaned

**Deliverables**:
- 10+ new golden tests covering Phase 3 features
- Updated documentation
- Completion report with metrics
- Git commit ready for push

---

## Technical Implementation Details

### Fix A5: go/types Integration

**Architecture**:
```
Generator.Generate(file)
    ↓
1. Parse .dingo → AST (preprocessor + go/parser)
    ↓
2. NEW: Run go/types type checker
    - types.Config.Check(pkg, fset, []*ast.File{file}, typesInfo)
    - Populate types.Info.Types, Types.Defs, Types.Uses
    ↓
3. Create TypeInferenceService with types.Info
    ↓
4. Inject into plugin Context
    ↓
5. Plugin Pipeline Transform
    - Plugins call ctx.TypeInference.InferType(expr)
    - Returns types.Type (accurate) instead of string heuristic
    ↓
6. Generate Go code with correct types
```

**Key Methods**:

```go
// pkg/plugin/builtin/type_inference.go

// InferType returns the type of an expression using go/types
func (s *TypeInferenceService) InferType(expr ast.Expr) types.Type {
    if s.typesInfo == nil || s.typesInfo.Types == nil {
        return nil
    }
    if tv, ok := s.typesInfo.Types[expr]; ok && tv.Type != nil {
        return tv.Type
    }
    return nil
}

// TypeToString converts types.Type to string representation
func (s *TypeInferenceService) TypeToString(t types.Type) string {
    if t == nil {
        return ""
    }
    return types.TypeString(t, nil) // Use types package formatter
}

// InferTypeWithFallback tries go/types, falls back to heuristics, reports error if both fail
func (s *TypeInferenceService) InferTypeWithFallback(expr ast.Expr) (string, error) {
    // 1. Try go/types
    if t := s.InferType(expr); t != nil {
        return s.TypeToString(t), nil
    }

    // 2. Try heuristics
    if typeName := s.heuristicInfer(expr); typeName != "" {
        return typeName, nil
    }

    // 3. Both failed - return error
    return "", fmt.Errorf("type inference failed for expression: %s", formatExpr(expr))
}
```

**Error Handling Strategy**:
- Type checker errors are logged but don't fail pipeline
- Plugins use `InferTypeWithFallback()` which tries both methods
- Only fail if both go/types AND heuristics fail
- Generate compile error with helpful message

### Fix A4: IIFE Pattern for Literals

**Addressability Detection**:

```go
// pkg/plugin/builtin/addressability.go

// isAddressable determines if an expression can have its address taken
func isAddressable(expr ast.Expr) bool {
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
    case *ast.ParenExpr:    // (x) - recurse
        return isAddressable(expr.(*ast.ParenExpr).X)

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
    case *ast.TypeAssertExpr: // x.(Type)
        return false

    default:
        // Conservative: assume non-addressable
        return false
    }
}
```

**IIFE Generation**:

```go
// wrapInTemporaryVariable wraps a non-addressable expression in IIFE
// Example: 42 → func() *int { __tmp0 := 42; return &__tmp0 }()
func wrapInTemporaryVariable(expr ast.Expr, typeExpr ast.Expr, ctx *plugin.Context) ast.Expr {
    // Generate unique temp variable name
    tmpVar := fmt.Sprintf("__tmp%d", ctx.TempVarCounter)
    ctx.TempVarCounter++

    // Create IIFE that assigns value to temp var and returns its address
    return &ast.CallExpr{
        Fun: &ast.FuncLit{
            Type: &ast.FuncType{
                Params: &ast.FieldList{},
                Results: &ast.FieldList{
                    List: []*ast.Field{{
                        Type: &ast.StarExpr{X: typeExpr},
                    }},
                },
            },
            Body: &ast.BlockStmt{
                List: []ast.Stmt{
                    // __tmpN := expr
                    &ast.AssignStmt{
                        Lhs: []ast.Expr{ast.NewIdent(tmpVar)},
                        Tok: token.DEFINE,
                        Rhs: []ast.Expr{expr},
                    },
                    // return &__tmpN
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
}
```

**Usage in Constructors**:

```go
// pkg/plugin/builtin/result_type.go

func (p *ResultTypePlugin) transformOkConstructor(call *ast.CallExpr) ast.Expr {
    valueArg := call.Args[0]

    // Infer type with fallback
    okType, err := p.typeInference.InferTypeWithFallback(valueArg)
    if err != nil {
        // Generate compile error
        return p.generateCompileError(call, err)
    }

    // Create type expression for IIFE return type
    typeExpr := ast.NewIdent(okType)

    // Handle addressability
    var okValue ast.Expr
    if isAddressable(valueArg) {
        // Direct address
        okValue = &ast.UnaryExpr{Op: token.AND, X: valueArg}
    } else {
        // Wrap in IIFE
        okValue = wrapInTemporaryVariable(valueArg, typeExpr, p.ctx)
    }

    // Create Result struct literal
    return &ast.CompositeLit{
        Type: ast.NewIdent(resultTypeName),
        Elts: []ast.Expr{
            &ast.KeyValueExpr{Key: ast.NewIdent("tag"), Value: ast.NewIdent("ResultTag_Ok")},
            &ast.KeyValueExpr{Key: ast.NewIdent("ok_0"), Value: okValue},
        },
    }
}
```

### Option[T] None Constant (Type-Context-Aware)

**Challenge**: `None` is an identifier with no inherent type information. We need to infer which `Option_T` type to use from the assignment/return context.

**Strategy**: Analyze parent AST nodes to determine expected type.

```go
// pkg/plugin/builtin/option_type.go

// detectNoneConstant identifies None identifier and infers target Option type
func (p *OptionTypePlugin) detectNoneConstant(ident *ast.Ident, parent ast.Node) (targetType string, isNone bool) {
    if ident.Name != "None" {
        return "", false
    }

    // Look at parent context to determine expected type
    switch parent := parent.(type) {
    case *ast.AssignStmt:
        // var x Option_int = None
        // x := None (need to look at previous declaration)
        if len(parent.Lhs) == 1 {
            if lhsIdent, ok := parent.Lhs[0].(*ast.Ident); ok {
                // Query type inference for LHS type
                if t := p.typeInference.InferType(lhsIdent); t != nil {
                    return extractOptionType(t), true
                }
            }
        }

    case *ast.ReturnStmt:
        // return None (need to look at function signature)
        if funcType := p.findEnclosingFunctionType(parent); funcType != nil {
            if len(funcType.Results.List) == 1 {
                resultType := funcType.Results.List[0].Type
                return extractOptionTypeFromAST(resultType), true
            }
        }

    case *ast.CallExpr:
        // someFunc(None) - need parameter type
        // Look at function signature to determine parameter type
        if paramType := p.inferParameterType(parent, ident); paramType != "" {
            return paramType, true
        }
    }

    return "", false
}

// extractOptionType extracts "T" from types.Type representing Option_T
func extractOptionType(t types.Type) string {
    // Handle types.Named for user-defined types
    if named, ok := t.(*types.Named); ok {
        name := named.Obj().Name()
        if strings.HasPrefix(name, "Option_") {
            return strings.TrimPrefix(name, "Option_")
        }
    }
    return ""
}

// Transform None identifier to Option_T{tag: OptionTag_None}
func (p *OptionTypePlugin) transformNoneConstant(ident *ast.Ident, targetType string) ast.Expr {
    optionTypeName := fmt.Sprintf("Option_%s", p.sanitizeTypeName(targetType))

    // Emit Option type declaration if not already done
    if !p.emittedTypes[optionTypeName] {
        p.emitOptionDeclaration(targetType, optionTypeName)
        p.emittedTypes[optionTypeName] = true
    }

    // Generate: Option_T{tag: OptionTag_None}
    return &ast.CompositeLit{
        Type: ast.NewIdent(optionTypeName),
        Elts: []ast.Expr{
            &ast.KeyValueExpr{
                Key:   ast.NewIdent("tag"),
                Value: ast.NewIdent("OptionTag_None"),
            },
            // No some_0 field for None variant
        },
    }
}
```

**Limitations**:
- Requires clear type context (assignment, return, parameter)
- Cannot infer in ambiguous contexts: `fmt.Println(None)` (what type?)
- Fallback: Require explicit `Option_None()` or type annotation

### Helper Methods Generation

**Example: Result[T,E].Map() method**

```go
// Generated in emitResultDeclaration()

// Map transforms the Ok value using the provided function
// If Result is Err, returns the same Err
func (r Result_int_error) Map(fn func(int) string) Result_string_error {
    if r.tag == ResultTag_Ok {
        return Result_string_error{
            tag:  ResultTag_Ok,
            ok_0: func() *string {
                mapped := fn(*r.ok_0)
                return &mapped
            }(),
        }
    }
    return Result_string_error{
        tag:   ResultTag_Err,
        err_0: r.err_0, // Preserve error
    }
}
```

**Challenge**: Generic helper methods require code generation per Result/Option type instance.

**Solution**: Generate methods as part of type declaration in `emitResultDeclaration()` and `emitOptionDeclaration()`.

**Implementation Approach**:
1. Detect all Result_T_E types used in file (Discovery phase)
2. For each type, generate struct + all helper methods (Inject phase)
3. Use template-based code generation for method bodies
4. Handle generic parameters via string substitution (T → int, U → string, etc.)

---

## File Modification Summary

### New Files Created

| File Path | Purpose | Lines | Complexity |
|-----------|---------|-------|------------|
| `/Users/jack/mag/dingo/pkg/plugin/builtin/addressability.go` | Addressability detection + IIFE wrapping | ~150 | Simple |
| `/Users/jack/mag/dingo/pkg/errors/type_inference.go` | Type inference error types | ~50 | Simple |
| `/Users/jack/mag/dingo/tests/golden/result_03_literals.dingo` | Test Fix A4 for Result | ~40 | Simple |
| `/Users/jack/mag/dingo/tests/golden/result_03_literals.go.golden` | Expected output | ~80 | Simple |
| `/Users/jack/mag/dingo/tests/golden/result_04_type_inference.dingo` | Test Fix A5 for Result | ~50 | Simple |
| `/Users/jack/mag/dingo/tests/golden/result_04_type_inference.go.golden` | Expected output | ~100 | Simple |
| `/Users/jack/mag/dingo/tests/golden/result_05_helpers.dingo` | Test helper methods for Result | ~80 | Medium |
| `/Users/jack/mag/dingo/tests/golden/result_05_helpers.go.golden` | Expected output | ~200 | Medium |
| `/Users/jack/mag/dingo/tests/golden/option_03_literals.dingo` | Test Fix A4 for Option | ~40 | Simple |
| `/Users/jack/mag/dingo/tests/golden/option_03_literals.go.golden` | Expected output | ~80 | Simple |
| `/Users/jack/mag/dingo/tests/golden/option_04_type_inference.dingo` | Test Fix A5 for Option | ~50 | Simple |
| `/Users/jack/mag/dingo/tests/golden/option_04_type_inference.go.golden` | Expected output | ~100 | Simple |
| `/Users/jack/mag/dingo/tests/golden/option_05_helpers.dingo` | Test helper methods for Option | ~80 | Medium |
| `/Users/jack/mag/dingo/tests/golden/option_05_helpers.go.golden` | Expected output | ~200 | Medium |
| `/Users/jack/mag/dingo/ai-docs/sessions/20251118-114514/completion-report.md` | Final metrics and summary | ~100 | Simple |

**Total New Files**: 15 files, ~1400 lines

### Existing Files Modified

| File Path | Changes | Lines Modified | Complexity | Batch |
|-----------|---------|----------------|------------|-------|
| `/Users/jack/mag/dingo/pkg/plugin/plugin.go` | Add TempVarCounter, error reporting | +30 | Low | 1b |
| `/Users/jack/mag/dingo/pkg/plugin/builtin/type_inference.go` | Add typesInfo, InferType(), TypeToString() | +80 | Medium | 1a |
| `/Users/jack/mag/dingo/pkg/generator/generator.go` | Add runTypeChecker(), integrate into pipeline | +70 | Medium | 1a |
| `/Users/jack/mag/dingo/pkg/plugin/builtin/result_type.go` | Fix A4 + Fix A5 + helper methods | +300 | High | 2a, 3a |
| `/Users/jack/mag/dingo/pkg/plugin/builtin/option_type.go` | Fix A4 + Fix A5 + None constant + helpers | +350 | High | 2b, 3b |
| `/Users/jack/mag/dingo/tests/golden/option_01_basic.dingo` | Update to use None constant | ~10 | Low | 4a |
| `/Users/jack/mag/dingo/tests/golden/option_01_basic.go.golden` | Update expected output | ~15 | Low | 4a |
| `/Users/jack/mag/dingo/pkg/plugin/builtin/result_type_test.go` | Add tests for Fix A4/A5 | +200 | Medium | 2a |
| `/Users/jack/mag/dingo/pkg/plugin/builtin/option_type_test.go` | Add tests for Fix A4/A5/None | +250 | Medium | 2b |
| `/Users/jack/mag/dingo/pkg/plugin/builtin/type_inference_test.go` | Add tests for InferType() | +100 | Medium | 1a |
| `/Users/jack/mag/dingo/CHANGELOG.md` | Document Phase 3 changes | +30 | Low | 4b |
| `/Users/jack/mag/dingo/ai-docs/ARCHITECTURE.md` | Document go/types integration | +50 | Low | 4b |

**Total Modified Files**: 12 files, ~1485 lines modified

**Grand Total**: ~2900 lines (1400 new, 1500 modified)

---

## Testing Strategy

### Unit Test Coverage

**Batch 1a Tests** (Type Inference):
```go
// pkg/plugin/builtin/type_inference_test.go

TestInferType_BasicLiterals       // 42 → int, "str" → string
TestInferType_Identifiers         // x where x: int → int
TestInferType_ComplexExpressions  // x + y, func()
TestInferType_NilAndInterfaces    // Edge cases
TestTypeToString_AllTypes         // int, *int, []int, map[string]int, etc.
TestInferTypeWithFallback_GoTypes // Uses types.Info
TestInferTypeWithFallback_Heuristics // Falls back when go/types unavailable
TestInferTypeWithFallback_Error   // Both fail, returns error
```

**Batch 1c Tests** (Addressability):
```go
// pkg/plugin/builtin/addressability_test.go

TestIsAddressable_Identifiers     // x, user, name
TestIsAddressable_Selectors       // user.Name, pkg.Var
TestIsAddressable_Index           // arr[i], m[key]
TestIsAddressable_Literals        // 42, "string" (not addressable)
TestIsAddressable_Composites      // User{}, []int{} (not addressable)
TestIsAddressable_BinaryExpr      // x + y (not addressable)
TestWrapInTemporaryVariable       // Generates correct IIFE
TestWrapInTemporaryVariable_Types // Preserves type information
```

**Batch 2a Tests** (Result Plugin):
```go
// pkg/plugin/builtin/result_type_test.go

TestTransformOkConstructor_Literal        // Ok(42) uses IIFE
TestTransformOkConstructor_Identifier     // Ok(x) uses &x
TestTransformOkConstructor_TypeInference  // Ok(x) → Result_int_error
TestTransformErrConstructor_Literal       // Err(errors.New(...)) uses IIFE
TestInferTypeFromExpr_GoTypes             // Uses TypeInferenceService
TestInferTypeFromExpr_Fallback            // Uses heuristics
TestInferTypeFromExpr_Error               // Reports error on failure
```

**Batch 2b Tests** (Option Plugin):
```go
// pkg/plugin/builtin/option_type_test.go

TestHandleSomeConstructor_Literal         // Some(42) uses IIFE
TestHandleSomeConstructor_Identifier      // Some(x) uses &x
TestDetectNoneConstant_Assignment         // var x Option_int = None
TestDetectNoneConstant_Return             // return None in func() Option_int
TestDetectNoneConstant_Parameter          // someFunc(None) infers from param type
TestTransformNoneConstant                 // Generates Option_T{tag: OptionTag_None}
TestNoneConstant_AmbiguousContext         // Error or fallback to Option_None()
```

**Batch 3a/3b Tests** (Helper Methods):
```go
// pkg/plugin/builtin/result_type_test.go
TestHelperMethod_IsOk
TestHelperMethod_IsErr
TestHelperMethod_Unwrap
TestHelperMethod_UnwrapOr
TestHelperMethod_UnwrapOrElse
TestHelperMethod_Map
TestHelperMethod_AndThen
TestHelperMethod_Filter

// pkg/plugin/builtin/option_type_test.go
TestHelperMethod_IsSome
TestHelperMethod_IsNone
TestHelperMethod_Unwrap
TestHelperMethod_UnwrapOr
TestHelperMethod_UnwrapOrElse
TestHelperMethod_Map
TestHelperMethod_AndThen
TestHelperMethod_Filter
```

**Target**: All 39/39 builtin plugin tests passing

### Integration Test Coverage (Golden Tests)

**Result[T,E] Tests**:
- `result_01_basic.dingo` - Basic Ok/Err usage (EXISTING, should still pass)
- `result_02_chaining.dingo` - Error propagation (EXISTING, should still pass)
- `result_03_literals.dingo` - NEW: Ok(42), Err(errors.New("..."))
- `result_04_type_inference.dingo` - NEW: Ok(x) where x: int
- `result_05_helpers.dingo` - NEW: Map, Filter, AndThen, Unwrap, etc.

**Option[T] Tests**:
- `option_01_basic.dingo` - UPDATED: Use None constant instead of Option_None()
- `option_02_chaining.dingo` - EXISTING: Should still pass
- `option_03_literals.dingo` - NEW: Some(42), Some("hello")
- `option_04_type_inference.dingo` - NEW: Some(x) where x: int
- `option_05_helpers.dingo` - NEW: Map, Filter, AndThen, Unwrap, etc.

**Target**: ~25/46 golden tests passing (up from ~15/46)

### End-to-End Validation

```bash
# Full test suite
go test ./... -v

# Specific package tests
go test ./pkg/plugin/builtin/... -v
go test ./pkg/generator/... -v

# Golden tests
go test ./tests/golden/... -v

# Build and run golden test examples
for file in tests/golden/result_*.dingo tests/golden/option_*.dingo; do
    echo "Testing $file..."
    go run ./cmd/dingo build "$file" || exit 1
    go_file="${file%.dingo}.go"
    output_file="/tmp/dingo_test_$(basename $file .dingo)"
    go build -o "$output_file" "$go_file" || exit 1
    "$output_file" || exit 1
done
```

**Success Criteria**:
- [ ] All unit tests pass (100%)
- [ ] All 39/39 builtin plugin tests pass
- [ ] All golden tests compile
- [ ] All golden tests produce expected output
- [ ] No regressions in existing tests

---

## Risk Analysis & Mitigation

### High-Risk Areas

**Risk 1: Type-Context-Aware None Constant**
- **Severity**: High
- **Probability**: Medium
- **Impact**: None constant may fail in complex contexts
- **Mitigation**:
  - Start with simple contexts (assignment, return)
  - Fallback to explicit `Option_None()` for ambiguous cases
  - Provide clear error messages
  - Defer complex contexts (ternary, function args) to Phase 4 if needed
- **Rollback Plan**: Require `Option_None()` syntax, defer None constant to Phase 4

**Risk 2: go/types Integration Stability**
- **Severity**: Medium
- **Probability**: Medium
- **Impact**: Type checker may fail on Dingo-specific syntax
- **Mitigation**:
  - Graceful error handling (log warnings, don't fail pipeline)
  - Robust fallback to heuristics
  - Only use types.Info where successfully populated
  - Test with various Dingo syntax patterns
- **Rollback Plan**: Disable go/types, use heuristics only

**Risk 3: Helper Method Code Generation**
- **Severity**: Medium
- **Probability**: Low
- **Impact**: Generated methods may have bugs or type mismatches
- **Mitigation**:
  - Template-based generation with extensive testing
  - Generate one method at a time, test incrementally
  - Use go/ast/astutil for safe AST manipulation
  - Compile and run generated code in tests
- **Rollback Plan**: Disable helper methods, mark as experimental

**Risk 4: IIFE Performance Overhead**
- **Severity**: Low
- **Probability**: Low
- **Impact**: IIFE may add runtime overhead
- **Mitigation**:
  - Go compiler likely inlines IIFE
  - Only use for non-addressable expressions
  - Benchmark if concerns arise
  - Optimize later if needed
- **Rollback Plan**: Document limitation, require manual temp variables

### Medium-Risk Areas

**Risk 5: Plugin Pipeline Complexity**
- **Severity**: Medium
- **Probability**: Low
- **Impact**: Plugins may conflict or run in wrong order
- **Mitigation**:
  - Explicit dependencies already implemented (Phase 2.6.1)
  - Run type checker BEFORE plugin pipeline
  - Clear separation between Discovery/Transform/Inject phases
  - Test plugin ordering
- **Rollback Plan**: Disable conflicting plugins

**Risk 6: Test Suite Expansion**
- **Severity**: Low
- **Probability**: Medium
- **Impact**: Test suite becomes too large, slow to run
- **Mitigation**:
  - Use table-driven tests for similar cases
  - Parallelize test execution
  - Use build tags for slow/integration tests
  - Keep unit tests fast (<1s each)
- **Rollback Plan**: None needed (more tests is good)

### Timeline Risks

**Risk 7: Scope Creep**
- **Severity**: Medium
- **Probability**: High
- **Impact**: Implementation takes longer than estimated
- **Mitigation**:
  - Strict scope definition (no new features mid-implementation)
  - Phased delivery (can ship Fix A5 independently)
  - Clear stopping points after each batch
  - Defer non-critical features (complex None contexts)
- **Rollback Plan**: Ship incrementally (Fix A5 → Fix A4 → Option → Helpers)

---

## Success Metrics

### Quantitative Targets

| Metric | Before Phase 3 | Target After Phase 3 | Stretch Goal |
|--------|----------------|---------------------|--------------|
| Builtin plugin tests passing | 31/39 (79%) | 39/39 (100%) | 39/39 (100%) |
| Golden tests passing | ~15/46 (33%) | ~25/46 (54%) | ~30/46 (65%) |
| Type inference accuracy | ~40% (many interface{}) | >90% | >95% |
| Literal constructor support | 0% (all fail) | 100% | 100% |
| Helper methods implemented | 0 | 16 (8 per type) | 16 |
| Lines of code (production) | ~8000 | ~9500 | ~10000 |
| Lines of code (tests) | ~3000 | ~4500 | ~5000 |
| Test coverage | ~70% | >80% | >85% |

### Qualitative Goals

**Code Quality**:
- [ ] Generated Go code is idiomatic (passes `golangci-lint`)
- [ ] No compiler warnings in generated code
- [ ] Type safety improved (fewer `interface{}` fallbacks)
- [ ] Clear, actionable error messages on failure
- [ ] Code is maintainable (clear comments, logical structure)

**Developer Experience**:
- [ ] `Ok(42)` works intuitively (no manual temp variables)
- [ ] Type inference is accurate and predictable
- [ ] `None` constant works in common contexts
- [ ] Helper methods (Map, Filter, etc.) are ergonomic
- [ ] Error messages guide users to solutions

**Completeness**:
- [ ] All Fix A4 requirements met
- [ ] All Fix A5 requirements met
- [ ] Option[T] has feature parity with Result[T,E]
- [ ] Foundation ready for Phase 4 (pattern matching)
- [ ] No major known bugs or limitations (or documented)

---

## Dependencies & Prerequisites

### External Dependencies
- **go/types**: Standard library (no new dependency)
- **go/importer**: Standard library (no new dependency)
- **go/ast**: Already in use
- **go/token**: Already in use
- **go/parser**: Already in use

**No new external dependencies required** ✅

### Internal Dependencies
- Phase 2.16 complete (Result[T,E] foundation, plugin pipeline)
- All 48/48 preprocessor tests passing
- Plugin ordering fix (Phase 2.6.1) stable

### Environment Requirements
- Go 1.21+ (for go/types features)
- golangci-lint (for code quality checks)
- git (for version control)

---

## Rollback Strategy

### Per-Component Rollback

**If Fix A5 (go/types) Fails**:
1. Revert `pkg/generator/generator.go` changes
2. Revert `pkg/plugin/builtin/type_inference.go` changes
3. Keep heuristic-only inference
4. Document limitation: "Type inference limited in complex cases"
5. Proceed with Fix A4 and Option[T] (still valuable)

**If Fix A4 (IIFE) Fails**:
1. Revert addressability module
2. Revert constructor changes in Result/Option plugins
3. Document limitation: "Use variables for constructors, not literals"
4. Update golden tests to use variables
5. Proceed with Fix A5 and Option[T] (partial)

**If None Constant Fails**:
1. Revert None detection logic
2. Document: "Use `Option_None()` syntax instead of `None`"
3. Update option_01_basic.dingo to use function syntax
4. Keep Fix A4, Fix A5, and helper methods (still valuable)

**If Helper Methods Fail**:
1. Disable helper method generation
2. Mark as experimental / Phase 4 feature
3. Keep Fix A4 and Fix A5 (core functionality)
4. User can manually implement helper methods if needed

### Full Rollback

If Phase 3 must be completely reverted:
1. `git revert <phase-3-commits>`
2. Return to Phase 2.16 baseline
3. All 48/48 preprocessor tests, 31/39 builtin tests still passing
4. No data loss, clean state

---

## Post-Implementation Checklist

### Code Quality
- [ ] All code formatted: `go fmt ./...`
- [ ] No linter errors: `golangci-lint run`
- [ ] No TODO/FIXME comments (or tracked in issues)
- [ ] Comprehensive comments on complex code
- [ ] All exported functions have godoc comments

### Testing
- [ ] All unit tests pass: `go test ./...`
- [ ] All 39/39 builtin plugin tests pass
- [ ] All golden tests compile and run
- [ ] End-to-end smoke test successful
- [ ] No regressions in existing functionality

### Documentation
- [ ] CHANGELOG.md updated with Phase 3 summary
- [ ] ARCHITECTURE.md updated with go/types integration
- [ ] Golden test README.md updated with new tests
- [ ] Reasoning files created for new golden tests
- [ ] Inline code comments explain complex logic

### Version Control
- [ ] All changes committed with clear messages
- [ ] Commit messages follow convention: "feat(phase-3): ..."
- [ ] Git history is clean (no WIP commits)
- [ ] Ready to push to origin/main

### Communication
- [ ] Completion report created with metrics
- [ ] Known limitations documented
- [ ] Next steps identified (Phase 4 planning)
- [ ] Stakeholders notified of completion

---

## Next Phase Preview: Phase 4

After Phase 3 completes, we'll have a solid foundation for:

**Phase 4 Candidates** (in priority order):
1. Pattern matching (`match` expression for Result/Option)
2. Error propagation operator (`?`) for Result[T,E]
3. Advanced Option[T] features (None in all contexts, chaining)
4. Automatic Go interop (wrap `(T, error)` → `Result[T,E]`)
5. Performance optimizations (inline IIFEs, reduce allocations)
6. Language server integration (gopls proxy for .dingo files)

**Estimated Timeline**: Phase 4 planning begins after Phase 3 completion report.

---

## Conclusion

Phase 3 is the most comprehensive update yet, completing the Result[T,E]/Option[T] foundation with:
- Accurate type inference via go/types integration
- Literal support via IIFE pattern
- Type-context-aware None constant
- Complete helper method suite
- Strict error reporting on failures

This provides a robust, production-ready foundation for pattern matching and error propagation in Phase 4.

**Estimated Effort**: 18-26 hours over 2-3 days
**Confidence Level**: Medium-High (complex features, proven patterns, clear rollback plans)
**Risk Level**: Medium (significant scope, multiple moving parts, good mitigation strategies)

---

**Plan Status**: APPROVED - Ready for Implementation
**Next Step**: Begin Batch 1 (Foundation Infrastructure)
**Success Criteria**: All 39/39 builtin tests passing, ~25/46 golden tests passing, no regressions
