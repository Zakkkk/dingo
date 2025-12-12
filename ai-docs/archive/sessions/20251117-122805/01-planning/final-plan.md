# Dingo Priority Tasks: Final Implementation Plan

**Session:** 20251117-122805
**Date:** 2025-11-17
**Status:** APPROVED - Ready for Implementation

---

## Executive Summary

**Scope:** Complete 4-phase implementation to stabilize tests, integrate type inference, complete Result/Option types, and enhance parser.

**Timeline:** 18-24 hours total development effort

**Key Decisions:**
- Auto-wrapping: Configurable via dingo.toml (requires config system implementation)
- Performance: Flexible budget (<15% overhead), prioritize simplicity over optimization
- Scope: Full implementation across all 4 phases

---

## Phase 1: Test Stabilization (2-3 hours)

**Goal:** Achieve 100% test pass rate, establish stable baseline for further development.

### 1.1 Fix Error Propagation Tests (30 min)

**Files:**
- `/Users/jack/mag/dingo/pkg/plugin/builtin/error_propagation_test.go`

**Issue:** Field name mismatches between generated code and test expectations.

**Root Cause:** Inconsistent naming convention with sum_types plugin (lowercase vs camelCase).

**Implementation Steps:**
1. Run failing tests and capture actual vs expected output
2. Review sum_types plugin field naming convention (should be lowercase: ok_0, err_0, some_0)
3. Update test assertions to match actual generated field names
4. Verify consistency with ResultTag_*, OptionTag_* enum variants

**Validation:**
- All error propagation tests pass
- Generated code matches sum_types conventions
- No regression in sum_types tests

**Risk:** Low - Test-only changes, no logic modifications

---

### 1.2 Fix Lambda Tests (45 min)

**Files:**
- `/Users/jack/mag/dingo/pkg/plugin/builtin/lambda_test.go`
- `/Users/jack/mag/dingo/pkg/plugin/builtin/functional_utils.go` (if adding contains())

**Issue:** Tests reference missing `contains()` utility function.

**Recommended Solution:** Add `contains()` to functional_utils plugin

**Rationale:**
- Consistent with existing functional utilities (filter, map, reduce, all, any, forEach)
- Zero-overhead IIFE generation pattern already established
- More elegant than rewriting tests with verbose loops

**Implementation Steps:**
1. Add `contains()` function signature detection in functional_utils.go:
   ```go
   // Detect: items.contains(func(x T) bool { return x == target })
   // Transform to: early-exit loop with boolean result
   ```
2. Generate IIFE with loop:
   ```go
   func() bool {
       for _, item := range items {
           if (func(x T) bool { return x == target })(item) {
               return true
           }
       }
       return false
   }()
   ```
3. Add unit tests for contains() transformation
4. Update lambda tests to use new utility

**Alternative (if above blocked):** Rewrite tests using standard Go range loops

**Validation:**
- Lambda tests pass
- contains() generates efficient early-exit loop
- Pattern consistent with other functional utilities

**Risk:** Low - Following established pattern

---

### 1.3 Fix Generator Marker Tests (30 min)

**Files:**
- `/Users/jack/mag/dingo/pkg/generator/markers_test.go`

**Issue:** 2/4 marker tests failing - marker injection not working as expected.

**Investigation Required:**
1. Check if marker generation is properly integrated into generator pipeline
2. Verify marker format matches test expectations (comment format, content)
3. Ensure config flag correctly enables/disables markers
4. Check if markers are preserved through AST transformations

**Expected Outcome:**
- Identify specific failure mode (missing markers, wrong format, or wrong location)
- Fix marker injection logic based on findings

**Implementation Steps:**
1. Run failing tests with verbose output
2. Debug marker injection in generator.go
3. Fix identified issues (likely config handling or comment attachment)
4. Verify all marker tests pass

**Validation:**
- 4/4 marker tests passing
- Markers correctly appear in generated Go code when enabled
- No markers when disabled

**Risk:** Low - Isolated feature, no impact on core transpilation

---

### 1.4 Fix Parser Feature Tests (45 min)

**Files:**
- `/Users/jack/mag/dingo/pkg/parser/new_features_test.go`
- `/Users/jack/mag/dingo/pkg/parser/participle.go`

**Issue:** Ternary operator tests failing with parse errors ("unexpected ':'").

**Root Cause Analysis Required:**
1. Check ternary operator grammar definition
2. Verify precedence relative to conditional operator (`?`)
3. Ensure colon (`:`) is recognized in ternary context

**Implementation Steps:**
1. Review Expression grammar in participle.go
2. Locate ternary operator rule (should be `condition ? true_expr : false_expr`)
3. Check operator precedence - ternary should be lower than most operators
4. Fix grammar if ternary is missing or incorrectly defined
5. Add comprehensive ternary test cases:
   - Simple: `a ? b : c`
   - Nested: `a ? b ? c : d : e`
   - With operators: `a > 0 ? x + 1 : y - 1`

**Expected Fix:**
- Ensure ternary is defined in Expression grammar
- Adjust precedence if conflicting with other operators
- May need to add TernaryExpr AST node if missing

**Validation:**
- All ternary parser tests pass
- Nested ternaries parse correctly
- No regression in other expression tests

**Risk:** Low-Medium - Parser change but isolated to ternary operator

---

### Phase 1 Deliverables

- [ ] 97/97 tests passing (or specific deferrals documented)
- [ ] CI/CD pipeline green
- [ ] Clean git status, no broken tests
- [ ] Baseline established for Phase 2 changes

**Success Criteria:** Zero failing tests, all known issues documented with clear next steps.

---

## Phase 2: Type Inference System Integration (6-8 hours)

**Goal:** Create centralized TypeInferenceService accessible to all plugins, enabling type-aware transformations.

### 2.1 Architecture Overview

**Current State:**
- TypeInference exists in `/Users/jack/mag/dingo/pkg/plugin/builtin/type_inference.go` (261 lines)
- Used only by ErrorPropagationPlugin
- Created per-transformation, not shared across plugins
- No inter-plugin communication about types

**Target State:**
```
Plugin Pipeline Context
    └─> TypeInferenceService (Shared)
        ├─> go/types type checker
        ├─> Type information cache
        ├─> Result/Option type detection
        ├─> Zero value generator
        └─> Synthetic type registry

Each Plugin Can:
    - ctx.TypeInference.InferType(expr)
    - ctx.TypeInference.IsResultType(typ)
    - ctx.TypeInference.IsOptionType(typ)
    - ctx.TypeInference.IsGoErrorTuple(sig)
    - ctx.TypeInference.GenerateZeroValue(typ)
```

**Design Principles:**
1. Single shared service instance per pipeline execution
2. Lifecycle managed by pipeline (create, inject, refresh, close)
3. Lazy type checking - only run when plugins request type info
4. Cache results to avoid repeated go/types queries
5. Support synthetic types registered by plugins (Result, Option enums)

---

### 2.2 Step-by-Step Implementation

#### Step 2.2.1: Enhance Plugin Context (1 hour)

**Files:**
- `/Users/jack/mag/dingo/pkg/plugin/plugin.go` (modify Context struct)
- `/Users/jack/mag/dingo/pkg/plugin/pipeline.go` (modify Pipeline.Execute)

**Changes to plugin.go:**

```go
// Add to Context struct
type Context struct {
    // ... existing fields ...

    // TypeInference provides shared type information across all plugins
    TypeInference *TypeInferenceService
}
```

**Changes to pipeline.go:**

```go
func (p *Pipeline) Execute(file *ast.File) error {
    // Create shared type inference service
    typeService, err := NewTypeInferenceService(p.fset, file, p.Ctx.Logger)
    if err != nil {
        p.Ctx.Logger.Warn("Type inference initialization failed: %v (continuing without types)", err)
        // Continue without type inference - plugins should degrade gracefully
    } else {
        p.Ctx.TypeInference = typeService
        defer typeService.Close()
    }

    // Execute plugin transformations
    for _, plugin := range p.sortedPlugins {
        if err := plugin.Transform(file, p.Ctx); err != nil {
            return fmt.Errorf("plugin %s failed: %w", plugin.Name(), err)
        }

        // Refresh type information after transformations
        // This allows later plugins to see types of generated code
        if typeService != nil {
            if err := typeService.Refresh(file); err != nil {
                p.Ctx.Logger.Warn("Type refresh after %s failed: %v", plugin.Name(), err)
                // Non-fatal - type info may be stale but continue
            }
        }
    }

    return nil
}
```

**Testing:**
- Verify pipeline creates TypeInferenceService
- Verify service is injected into context
- Verify service is closed after pipeline completes
- Verify pipeline continues if type inference fails (degraded mode)

**Risk Mitigation:**
- Make type inference optional - plugins must handle nil TypeInference
- Log warnings on type inference failures but don't halt builds
- Ensure backward compatibility with plugins that don't use types

---

#### Step 2.2.2: Refactor TypeInference to Service (2-3 hours)

**Files:**
- `/Users/jack/mag/dingo/pkg/plugin/builtin/type_inference.go` (major refactor)

**Current Structure:**
```go
type TypeInference struct {
    fset   *token.FileSet
    info   *types.Info
    pkg    *types.Package
    config *types.Config
}
```

**New Structure:**

```go
// TypeInferenceService provides centralized type information for all plugins
type TypeInferenceService struct {
    fset   *token.FileSet
    logger Logger

    // Type checker state
    info   *types.Info
    pkg    *types.Package
    config *types.Config

    // Performance cache
    typeCache     map[ast.Expr]types.Type
    cacheEnabled  bool

    // Synthetic type registry (for Result, Option, etc)
    syntheticTypes map[string]*SyntheticTypeInfo

    // Statistics (for performance monitoring)
    typeChecks int
    cacheHits  int
}

type SyntheticTypeInfo struct {
    TypeName   string           // e.g., "Result_int_error"
    Underlying *types.Named     // Type information
    GenDecl    *ast.GenDecl     // Generated AST declaration
}
```

**New Methods to Add:**

```go
// Constructor
func NewTypeInferenceService(fset *token.FileSet, file *ast.File, logger Logger) (*TypeInferenceService, error)

// Lifecycle
func (s *TypeInferenceService) Refresh(file *ast.File) error
func (s *TypeInferenceService) Close() error

// Core inference (existing, keep as-is)
func (s *TypeInferenceService) InferType(expr ast.Expr) (types.Type, error)
func (s *TypeInferenceService) InferFunctionReturnType(fn *ast.FuncDecl) (types.Type, error)
func (s *TypeInferenceService) GenerateZeroValue(typ types.Type) ast.Expr

// NEW: Type detection helpers
func (s *TypeInferenceService) IsResultType(typ types.Type) (T, E types.Type, ok bool)
func (s *TypeInferenceService) IsOptionType(typ types.Type) (T types.Type, ok bool)
func (s *TypeInferenceService) IsPointerType(typ types.Type) bool
func (s *TypeInferenceService) IsErrorType(typ types.Type) bool

// NEW: Go interop detection
func (s *TypeInferenceService) IsGoErrorTuple(sig *types.Signature) (valueType types.Type, ok bool)
func (s *TypeInferenceService) ShouldWrapAsResult(callExpr *ast.CallExpr) bool

// NEW: Synthetic type registry
func (s *TypeInferenceService) RegisterSyntheticType(name string, info *SyntheticTypeInfo)
func (s *TypeInferenceService) GetSyntheticType(name string) (*SyntheticTypeInfo, bool)
func (s *TypeInferenceService) IsSyntheticType(name string) bool

// NEW: Performance monitoring
func (s *TypeInferenceService) Stats() TypeInferenceStats

type TypeInferenceStats struct {
    TypeChecks int
    CacheHits  int
    CacheSize  int
}
```

**Implementation Details:**

**IsResultType() implementation:**
```go
func (s *TypeInferenceService) IsResultType(typ types.Type) (T, E types.Type, ok bool) {
    // Check if type is a named type matching Result_* pattern
    named, ok := typ.(*types.Named)
    if !ok {
        return nil, nil, false
    }

    name := named.Obj().Name()

    // Check for Result_T_E naming pattern
    if !strings.HasPrefix(name, "Result_") {
        return nil, nil, false
    }

    // Check if it's in synthetic type registry (more reliable than name parsing)
    if info, found := s.GetSyntheticType(name); found {
        // Extract T and E from underlying struct
        // Result[T, E] has fields: tag ResultTag, ok_0 T, err_0 E
        structType, ok := info.Underlying.Underlying().(*types.Struct)
        if !ok {
            return nil, nil, false
        }

        if structType.NumFields() != 3 {
            return nil, nil, false
        }

        // Fields: tag (0), ok_0 (1), err_0 (2)
        T = structType.Field(1).Type()
        E = structType.Field(2).Type()
        return T, E, true
    }

    // Fallback: Name parsing (less reliable but works for user-defined)
    // Pattern: Result_T_E where T and E are type names
    // This is fragile but handles edge cases
    return nil, nil, false
}
```

**IsOptionType() implementation:**
```go
func (s *TypeInferenceService) IsOptionType(typ types.Type) (T types.Type, ok bool) {
    named, ok := typ.(*types.Named)
    if !ok {
        return nil, false
    }

    name := named.Obj().Name()
    if !strings.HasPrefix(name, "Option_") {
        return nil, false
    }

    // Check synthetic registry
    if info, found := s.GetSyntheticType(name); found {
        // Option[T] has fields: tag OptionTag, some_0 T
        structType, ok := info.Underlying.Underlying().(*types.Struct)
        if !ok {
            return nil, false
        }

        if structType.NumFields() != 2 {
            return nil, false
        }

        T = structType.Field(1).Type()
        return T, true
    }

    return nil, false
}
```

**IsGoErrorTuple() implementation:**
```go
func (s *TypeInferenceService) IsGoErrorTuple(sig *types.Signature) (valueType types.Type, ok bool) {
    results := sig.Results()
    if results.Len() != 2 {
        return nil, false
    }

    // Second return must be error type
    secondType := results.At(1).Type()
    if !s.IsErrorType(secondType) {
        return nil, false
    }

    // First return is the value type
    return results.At(0).Type(), true
}

func (s *TypeInferenceService) IsErrorType(typ types.Type) bool {
    // Check if type is built-in error interface
    if typ.String() == "error" {
        return true
    }

    // Check if implements error interface
    iface, ok := typ.Underlying().(*types.Interface)
    if !ok {
        return false
    }

    // error interface has one method: Error() string
    if iface.NumMethods() == 1 {
        method := iface.Method(0)
        if method.Name() == "Error" {
            sig, ok := method.Type().(*types.Signature)
            if ok && sig.Params().Len() == 0 && sig.Results().Len() == 1 {
                return sig.Results().At(0).Type().String() == "string"
            }
        }
    }

    return false
}
```

**Performance Considerations:**
- Cache type inference results per expression
- Only invalidate cache on Refresh()
- Monitor cache hit rate, log if <50% (indicates ineffective caching)
- Flexible performance budget (<15% overhead) allows simple implementation first

**Testing Strategy:**
- Unit tests for each new method
- Test Result/Option type detection with various scenarios
- Test synthetic type registry
- Test cache effectiveness
- Performance benchmark vs baseline (should be <15% overhead)

---

#### Step 2.2.3: Update ErrorPropagationPlugin (1 hour)

**Files:**
- `/Users/jack/mag/dingo/pkg/plugin/builtin/error_propagation.go`

**Current Code:**
```go
func (p *ErrorPropagationPlugin) Transform(file *ast.File, ctx *plugin.Context) error {
    // Creates its own TypeInference
    typeInf := NewTypeInference(ctx.FileSet, file)
    // ...
}
```

**New Code:**
```go
func (p *ErrorPropagationPlugin) Transform(file *ast.File, ctx *plugin.Context) error {
    // Use shared TypeInferenceService from context
    if ctx.TypeInference == nil {
        ctx.Logger.Warn("ErrorPropagation: No type inference available, using conservative zero values")
        // Continue without type inference - use conservative defaults
    }

    // Rest of transformation uses ctx.TypeInference instead of local typeInf
    // ...
}
```

**Changes Required:**
1. Remove local TypeInference creation
2. Update all `typeInf.*` calls to `ctx.TypeInference.*`
3. Add nil checks before using ctx.TypeInference
4. Ensure graceful degradation if type inference unavailable

**Testing:**
- Verify error propagation still works with shared service
- Verify graceful degradation when TypeInference is nil
- No regression in error propagation tests

---

#### Step 2.2.4: Update SumTypesPlugin (1 hour)

**Files:**
- `/Users/jack/mag/dingo/pkg/plugin/builtin/sum_types.go`

**New Feature:** Register synthetic Result/Option types with TypeInferenceService

**Implementation:**

```go
func (p *SumTypesPlugin) Transform(file *ast.File, ctx *plugin.Context) error {
    // ... existing enum transformation logic ...

    // After generating Result[T, E] or Option[T] enum
    if ctx.TypeInference != nil {
        // Register synthetic type for other plugins to detect
        ctx.TypeInference.RegisterSyntheticType(enumName, &SyntheticTypeInfo{
            TypeName:   enumName,
            Underlying: namedType,  // *types.Named for the generated enum
            GenDecl:    genDecl,    // *ast.GenDecl for the enum declaration
        })

        ctx.Logger.Debug("Registered synthetic type: %s", enumName)
    }

    return nil
}
```

**Specific Registration Points:**
1. When generating `Result_T_E` enum → register as synthetic
2. When generating `Option_T` enum → register as synthetic
3. Store enough type information for later plugins to extract T, E types

**Testing:**
- Verify synthetic types are registered
- Verify other plugins can detect registered types
- Test cross-plugin type detection (sum_types generates, result_type detects)

---

#### Step 2.2.5: Integration Testing (1-2 hours)

**Test Categories:**

**1. Unit Tests for TypeInferenceService:**
- `/Users/jack/mag/dingo/pkg/plugin/builtin/type_inference_test.go`
- Test IsResultType() with various type structures
- Test IsOptionType() detection
- Test IsGoErrorTuple() with different signatures
- Test synthetic type registration and retrieval
- Test cache hit rate on repeated queries

**2. Integration Tests:**
- `/Users/jack/mag/dingo/tests/integration/type_inference_test.go` (new file)
- Test cross-plugin type detection:
  ```dingo
  // SumTypesPlugin generates Result[int, error]
  enum Result[int, error] { Ok(int), Err(error) }

  // ResultTypePlugin should detect this type
  x := Ok(42)  // Should recognize Result[int, error]
  ```
- Test pipeline with and without type inference
- Test degraded mode (plugins work when TypeInference is nil)

**3. Performance Benchmarks:**
- Measure build time impact on various file sizes
- Ensure <15% overhead (flexible budget from user clarifications)
- Log cache hit rates and type check counts

**Performance Test Cases:**
- Small file (< 100 LOC): Should have minimal overhead
- Medium file (100-500 LOC): Should stay under 10% overhead
- Large file (> 500 LOC): Acceptable up to 15% overhead

**Validation Criteria:**
- All existing tests still pass (backward compatibility)
- New type detection tests pass
- Performance within budget
- No memory leaks (verify with Go profiler)

---

### Phase 2 Deliverables

- [ ] TypeInferenceService integrated into plugin pipeline
- [ ] All plugins can access shared type information via ctx.TypeInference
- [ ] Result/Option type detection methods implemented and tested
- [ ] ErrorPropagationPlugin migrated to shared service
- [ ] SumTypesPlugin registers synthetic types
- [ ] Build time increase < 15% on benchmark suite
- [ ] 100% backward compatibility (all existing tests pass)
- [ ] Graceful degradation when type inference unavailable

**Success Criteria:** Plugins can reliably detect Result/Option types, enabling Phase 3 implementation.

---

## Phase 3: Result/Option Type Completion (6-8 hours)

**Goal:** Transform Result/Option from foundation-only to fully functional types with automatic Go interop.

### 3.1 Current State

**What Exists:**
- `/Users/jack/mag/dingo/pkg/plugin/builtin/result_type.go` - Foundation plugin with placeholder Transform()
- `/Users/jack/mag/dingo/pkg/plugin/builtin/option_type.go` - Foundation plugin with placeholder Transform()
- Enum structure defined via SumTypesPlugin (Ok/Err for Result, Some/None for Option)
- Helper methods designed (IsOk, Unwrap, UnwrapOr - not yet implemented)

**What's Missing:**
- Actual type detection in user code
- Transformation of Result/Option literals
- Automatic wrapping of Go (T, error) returns
- Integration with error propagation operator
- Integration with null coalescing
- Configuration system for auto-wrapping behavior

---

### 3.2 Configuration System Implementation (1 hour)

**Prerequisite:** dingo.toml support may not exist yet. Need to implement or extend.

**Files:**
- `/Users/jack/mag/dingo/pkg/config/config.go` (new or extend existing)
- `/Users/jack/mag/dingo/dingo.toml.example` (new)

**Configuration Schema:**

```toml
[transpiler]
# Auto-wrap Go functions returning (T, error) in Result[T, E]
auto_wrap_go_errors = true  # default: true

# Auto-wrap nil-able types in Option[T]
auto_wrap_go_nils = false   # default: false (less invasive)

[transpiler.performance]
# Enable type inference cache
enable_type_cache = true    # default: true

# Log type inference statistics
log_type_stats = false      # default: false
```

**Implementation:**

```go
// pkg/config/config.go
package config

type TranspilerConfig struct {
    AutoWrapGoErrors bool `toml:"auto_wrap_go_errors"`
    AutoWrapGoNils   bool `toml:"auto_wrap_go_nils"`
    Performance      PerformanceConfig `toml:"performance"`
}

type PerformanceConfig struct {
    EnableTypeCache bool `toml:"enable_type_cache"`
    LogTypeStats    bool `toml:"log_type_stats"`
}

func LoadConfig(path string) (*TranspilerConfig, error) {
    // Load dingo.toml if exists, otherwise use defaults
    // Use github.com/BurntSushi/toml for parsing
}

func DefaultConfig() *TranspilerConfig {
    return &TranspilerConfig{
        AutoWrapGoErrors: true,
        AutoWrapGoNils:   false,
        Performance: PerformanceConfig{
            EnableTypeCache: true,
            LogTypeStats:    false,
        },
    }
}
```

**Integration:**
- Add Config to plugin.Context
- Load config in main transpiler entry point
- Pass to plugin pipeline

**Testing:**
- Test config loading from file
- Test default config when no file exists
- Test config validation

---

### 3.3 Result Type Implementation (3-4 hours)

**Files:**
- `/Users/jack/mag/dingo/pkg/plugin/builtin/result_type.go` (implement Transform())
- `/Users/jack/mag/dingo/pkg/plugin/builtin/result_type_test.go` (comprehensive tests)

---

#### Feature 3.3.1: Result Literal Transformation (1 hour)

**Syntax:**
```dingo
result := Ok(42)              // Infers Result[int, error]
failure := Err(errors.New())  // Infers Result[T, error]
```

**Transform to:**
```go
result := Result_int_error{tag: ResultTag_Ok, ok_0: 42}
failure := Result_T_error{tag: ResultTag_Err, err_0: errors.New()}
```

**Detection Logic:**

```go
func (p *ResultTypePlugin) Transform(file *ast.File, ctx *plugin.Context) error {
    ast.Inspect(file, func(n ast.Node) bool {
        callExpr, ok := n.(*ast.CallExpr)
        if !ok {
            return true
        }

        // Detect Ok() or Err() calls
        ident, ok := callExpr.Fun.(*ast.Ident)
        if !ok {
            return true
        }

        if ident.Name == "Ok" {
            p.transformOkLiteral(callExpr, ctx)
        } else if ident.Name == "Err" {
            p.transformErrLiteral(callExpr, ctx)
        }

        return true
    })

    return nil
}
```

**Type Inference:**
- For `Ok(value)`: Infer T from value type, E defaults to `error`
- For `Err(errValue)`: Infer E from errValue type, T requires context (assignment LHS or function return type)
- Use ctx.TypeInference.InferType() to get value types

**Transformation:**

```go
func (p *ResultTypePlugin) transformOkLiteral(callExpr *ast.CallExpr, ctx *plugin.Context) {
    if len(callExpr.Args) != 1 {
        ctx.Logger.Error("Ok() expects exactly 1 argument")
        return
    }

    // Infer T from argument
    valueType, err := ctx.TypeInference.InferType(callExpr.Args[0])
    if err != nil {
        ctx.Logger.Warn("Cannot infer Ok value type: %v", err)
        return
    }

    // E defaults to error (could be configurable)
    errorType := types.Universe.Lookup("error").Type()

    // Generate Result_T_E type name
    resultTypeName := fmt.Sprintf("Result_%s_error", typeToString(valueType))

    // Ensure Result_T_E enum exists (ask SumTypesPlugin to generate if not)
    p.ensureResultType(resultTypeName, valueType, errorType, ctx)

    // Replace callExpr with struct literal
    *callExpr = ast.CompositeLit{
        Type: &ast.Ident{Name: resultTypeName},
        Elts: []ast.Expr{
            // tag: ResultTag_Ok
            &ast.KeyValueExpr{
                Key:   &ast.Ident{Name: "tag"},
                Value: &ast.Ident{Name: "ResultTag_Ok"},
            },
            // ok_0: <value>
            &ast.KeyValueExpr{
                Key:   &ast.Ident{Name: "ok_0"},
                Value: callExpr.Args[0],
            },
        },
    }
}
```

**Testing:**
- Test Ok() with various value types (int, string, struct, pointer)
- Test Err() with various error types
- Test type inference in different contexts
- Test Result_T_E generation on demand

---

#### Feature 3.3.2: Go Interop - Auto-wrapping (2 hours)

**Goal:** Automatically wrap Go functions returning (T, error) in Result[T, E]

**User Control:** Configurable via `auto_wrap_go_errors` in dingo.toml

**Syntax:**
```dingo
data := readFile("config.json")  // readFile returns ([]byte, error)
// With auto_wrap_go_errors = true, wraps to Result[[]byte, error]
```

**Transform to:**
```go
data := func() Result_bytes_error {
    __tmp0, __tmp1 := readFile("config.json")
    if __tmp1 != nil {
        return Result_bytes_error{tag: ResultTag_Err, err_0: __tmp1}
    }
    return Result_bytes_error{tag: ResultTag_Ok, ok_0: __tmp0}
}()
```

**Detection Logic:**

```go
func (p *ResultTypePlugin) shouldWrapGoCall(assignStmt *ast.AssignStmt, ctx *plugin.Context) bool {
    // Only wrap if auto_wrap_go_errors is enabled
    if !ctx.Config.AutoWrapGoErrors {
        return false
    }

    // Check if RHS is a call expression
    if len(assignStmt.Rhs) != 1 {
        return false
    }

    callExpr, ok := assignStmt.Rhs[0].(*ast.CallExpr)
    if !ok {
        return false
    }

    // Check if call returns (T, error)
    funcType, err := ctx.TypeInference.InferType(callExpr.Fun)
    if err != nil {
        return false
    }

    sig, ok := funcType.(*types.Signature)
    if !ok {
        return false
    }

    valueType, isErrorTuple := ctx.TypeInference.IsGoErrorTuple(sig)
    if !isErrorTuple {
        return false
    }

    // Check if LHS expects Result type (explicit type annotation or inference)
    // If LHS is untyped, don't wrap (breaking change)
    // If LHS has explicit Result type, wrap

    if len(assignStmt.Lhs) == 1 {
        // Check if variable is typed as Result
        // This requires looking at variable declaration or assignment context
        // For now, conservative: only wrap if explicitly requested

        // TODO: More sophisticated detection
        // For MVP, require explicit Result type annotation:
        // var data Result[[]byte, error] = readFile("config.json")
        return p.lhsExpectsResult(assignStmt.Lhs[0], ctx)
    }

    return false
}
```

**Transformation:**

```go
func (p *ResultTypePlugin) wrapGoCall(assignStmt *ast.AssignStmt, ctx *plugin.Context) {
    callExpr := assignStmt.Rhs[0].(*ast.CallExpr)

    // Get value and error types
    funcType, _ := ctx.TypeInference.InferType(callExpr.Fun)
    sig := funcType.(*types.Signature)
    valueType, _ := ctx.TypeInference.IsGoErrorTuple(sig)
    errorType := sig.Results().At(1).Type()

    // Generate Result_T_E type name
    resultTypeName := fmt.Sprintf("Result_%s_%s",
        typeToString(valueType),
        typeToString(errorType))

    // Ensure Result type exists
    p.ensureResultType(resultTypeName, valueType, errorType, ctx)

    // Generate IIFE wrapper
    iife := &ast.CallExpr{
        Fun: &ast.FuncLit{
            Type: &ast.FuncType{
                Results: &ast.FieldList{
                    List: []*ast.Field{
                        {Type: &ast.Ident{Name: resultTypeName}},
                    },
                },
            },
            Body: &ast.BlockStmt{
                List: []ast.Stmt{
                    // __tmp0, __tmp1 := callExpr
                    &ast.AssignStmt{
                        Lhs: []ast.Expr{
                            &ast.Ident{Name: "__tmp0"},
                            &ast.Ident{Name: "__tmp1"},
                        },
                        Tok: token.DEFINE,
                        Rhs: []ast.Expr{callExpr},
                    },
                    // if __tmp1 != nil { return Err(...) }
                    &ast.IfStmt{
                        Cond: &ast.BinaryExpr{
                            X:  &ast.Ident{Name: "__tmp1"},
                            Op: token.NEQ,
                            Y:  &ast.Ident{Name: "nil"},
                        },
                        Body: &ast.BlockStmt{
                            List: []ast.Stmt{
                                &ast.ReturnStmt{
                                    Results: []ast.Expr{
                                        p.createResultLiteral(resultTypeName, false, &ast.Ident{Name: "__tmp1"}),
                                    },
                                },
                            },
                        },
                    },
                    // return Ok(__tmp0)
                    &ast.ReturnStmt{
                        Results: []ast.Expr{
                            p.createResultLiteral(resultTypeName, true, &ast.Ident{Name: "__tmp0"}),
                        },
                    },
                },
            },
        },
    }

    // Replace RHS with IIFE
    assignStmt.Rhs[0] = iife
}
```

**Key Design Decision:** Conservative wrapping
- Only wrap when LHS explicitly expects Result type
- Avoid breaking existing code that expects (value, error) tuples
- User can opt-out via `auto_wrap_go_errors = false`

**Testing:**
- Test auto-wrapping with various Go functions
- Test with config enabled and disabled
- Test explicit Result type annotation
- Test that unwrapped calls still work

---

#### Feature 3.3.3: Error Propagation Integration (1 hour)

**Goal:** Make `?` operator work with Result types, not just Go error tuples

**Syntax:**
```dingo
func processData() Result[Data, error] {
    config := readConfig()?  // readConfig returns Result[Config, error]
    validated := validate(config)?
    return Ok(validated)
}
```

**Transform to:**
```go
func processData() Result_Data_error {
    config := func() Config {
        __result := readConfig()
        if __result.tag == ResultTag_Err {
            return Result_Data_error{tag: ResultTag_Err, err_0: __result.err_0}
        }
        return __result.ok_0
    }()

    validated := func() Data {
        __result := validate(config)
        if __result.tag == ResultTag_Err {
            return Result_Data_error{tag: ResultTag_Err, err_0: __result.err_0}
        }
        return __result.ok_0
    }()

    return Result_Data_error{tag: ResultTag_Ok, ok_0: validated}
}
```

**Implementation:**

**Files:**
- `/Users/jack/mag/dingo/pkg/plugin/builtin/error_propagation.go` (modify)

**Changes:**

```go
func (p *ErrorPropagationPlugin) Transform(file *ast.File, ctx *plugin.Context) error {
    // Existing logic for detecting ? operator

    // NEW: Check if expression is Result type instead of (T, error) tuple
    exprType, err := ctx.TypeInference.InferType(expr)
    if err == nil {
        T, E, isResult := ctx.TypeInference.IsResultType(exprType)
        if isResult {
            // Generate pattern match unwrapping instead of error check
            return p.unwrapResultType(expr, T, E, ctx)
        }
    }

    // Fallback to existing (T, error) handling
    return p.unwrapErrorTuple(expr, ctx)
}

func (p *ErrorPropagationPlugin) unwrapResultType(
    expr ast.Expr,
    T, E types.Type,
    ctx *plugin.Context,
) ast.Expr {
    // Generate IIFE that pattern matches on Result
    resultVarName := p.generateUniqueVar("result")

    return &ast.CallExpr{
        Fun: &ast.FuncLit{
            Type: &ast.FuncType{
                Results: &ast.FieldList{
                    List: []*ast.Field{
                        {Type: typeToAST(T)},
                    },
                },
            },
            Body: &ast.BlockStmt{
                List: []ast.Stmt{
                    // __result := expr
                    &ast.AssignStmt{
                        Lhs: []ast.Expr{&ast.Ident{Name: resultVarName}},
                        Tok: token.DEFINE,
                        Rhs: []ast.Expr{expr},
                    },
                    // if __result.tag == ResultTag_Err { return Err(...) }
                    &ast.IfStmt{
                        Cond: &ast.BinaryExpr{
                            X: &ast.SelectorExpr{
                                X:   &ast.Ident{Name: resultVarName},
                                Sel: &ast.Ident{Name: "tag"},
                            },
                            Op: token.EQL,
                            Y:  &ast.Ident{Name: "ResultTag_Err"},
                        },
                        Body: &ast.BlockStmt{
                            List: []ast.Stmt{
                                &ast.ReturnStmt{
                                    Results: []ast.Expr{
                                        // Return the error wrapped in current function's Result type
                                        p.propagateError(resultVarName, ctx),
                                    },
                                },
                            },
                        },
                    },
                    // return __result.ok_0
                    &ast.ReturnStmt{
                        Results: []ast.Expr{
                            &ast.SelectorExpr{
                                X:   &ast.Ident{Name: resultVarName},
                                Sel: &ast.Ident{Name: "ok_0"},
                            },
                        },
                    },
                },
            },
        },
    }
}
```

**Testing:**
- Test `?` with Result[T, E] types
- Test `?` with Go (T, error) tuples (existing behavior)
- Test mixed usage in same function
- Test error type propagation

---

### 3.4 Option Type Implementation (2-3 hours)

**Files:**
- `/Users/jack/mag/dingo/pkg/plugin/builtin/option_type.go` (implement Transform())
- `/Users/jack/mag/dingo/pkg/plugin/builtin/option_type_test.go`

---

#### Feature 3.4.1: Option Literal Transformation (1 hour)

**Syntax:**
```dingo
value := Some(42)        // Option[int]
empty := None            // Option[T] (requires type context)
```

**Transform to:**
```go
value := Option_int{tag: OptionTag_Some, some_0: 42}
empty := Option_T{tag: OptionTag_None}
```

**Implementation:** Similar to Result literals, simpler (only one type parameter)

```go
func (p *OptionTypePlugin) transformSomeLiteral(callExpr *ast.CallExpr, ctx *plugin.Context) {
    if len(callExpr.Args) != 1 {
        ctx.Logger.Error("Some() expects exactly 1 argument")
        return
    }

    // Infer T from argument
    valueType, err := ctx.TypeInference.InferType(callExpr.Args[0])
    if err != nil {
        ctx.Logger.Warn("Cannot infer Some value type: %v", err)
        return
    }

    // Generate Option_T type name
    optionTypeName := fmt.Sprintf("Option_%s", typeToString(valueType))

    // Ensure Option_T enum exists
    p.ensureOptionType(optionTypeName, valueType, ctx)

    // Replace with struct literal
    *callExpr = ast.CompositeLit{
        Type: &ast.Ident{Name: optionTypeName},
        Elts: []ast.Expr{
            &ast.KeyValueExpr{
                Key:   &ast.Ident{Name: "tag"},
                Value: &ast.Ident{Name: "OptionTag_Some"},
            },
            &ast.KeyValueExpr{
                Key:   &ast.Ident{Name: "some_0"},
                Value: callExpr.Args[0],
            },
        },
    }
}

func (p *OptionTypePlugin) transformNoneLiteral(ident *ast.Ident, ctx *plugin.Context) {
    // None requires type context from assignment or return statement
    // This is trickier - need to infer expected type from LHS

    // For MVP, may require explicit type annotation:
    // var x Option[int] = None

    // TODO: Implement type context inference
}
```

**Testing:**
- Test Some() with various value types
- Test None with explicit type annotation
- Test type inference from context

---

#### Feature 3.4.2: Null Coalescing Integration (1 hour)

**Goal:** Make `??` operator work with Option types, not just pointers

**Syntax:**
```dingo
value := maybeValue ?? 0  // maybeValue is Option[int]
```

**Transform to:**
```go
value := func() int {
    if maybeValue.tag == OptionTag_Some {
        return maybeValue.some_0
    }
    return 0
}()
```

**Files:**
- `/Users/jack/mag/dingo/pkg/plugin/builtin/null_coalescing.go` (modify)

**Changes:**

```go
func (p *NullCoalescingPlugin) Transform(file *ast.File, ctx *plugin.Context) error {
    // Existing logic for detecting ?? operator

    // NEW: Check if LHS is Option type
    lhsType, err := ctx.TypeInference.InferType(lhs)
    if err == nil {
        T, isOption := ctx.TypeInference.IsOptionType(lhsType)
        if isOption {
            // Generate pattern match unwrapping
            return p.unwrapOptionType(lhs, rhs, T, ctx)
        }
    }

    // Fallback to existing pointer nil check
    return p.unwrapPointer(lhs, rhs, ctx)
}

func (p *NullCoalescingPlugin) unwrapOptionType(
    optionExpr, defaultExpr ast.Expr,
    T types.Type,
    ctx *plugin.Context,
) ast.Expr {
    return &ast.CallExpr{
        Fun: &ast.FuncLit{
            Type: &ast.FuncType{
                Results: &ast.FieldList{
                    List: []*ast.Field{
                        {Type: typeToAST(T)},
                    },
                },
            },
            Body: &ast.BlockStmt{
                List: []ast.Stmt{
                    &ast.IfStmt{
                        Cond: &ast.BinaryExpr{
                            X: &ast.SelectorExpr{
                                X:   optionExpr,
                                Sel: &ast.Ident{Name: "tag"},
                            },
                            Op: token.EQL,
                            Y:  &ast.Ident{Name: "OptionTag_Some"},
                        },
                        Body: &ast.BlockStmt{
                            List: []ast.Stmt{
                                &ast.ReturnStmt{
                                    Results: []ast.Expr{
                                        &ast.SelectorExpr{
                                            X:   optionExpr,
                                            Sel: &ast.Ident{Name: "some_0"},
                                        },
                                    },
                                },
                            },
                        },
                    },
                    &ast.ReturnStmt{
                        Results: []ast.Expr{defaultExpr},
                    },
                },
            },
        },
    }
}
```

**Testing:**
- Test ?? with Option types
- Test ?? with pointers (existing behavior)
- Test mixed usage

---

#### Feature 3.4.3: Safe Navigation Integration (1 hour)

**Goal:** Fix chaining bug by detecting Option returns from safe navigation

**Current Bug:** `user?.address?.city` doesn't propagate Option through chain

**Syntax:**
```dingo
name := user?.address?.city  // Each step returns Option
```

**Expected Transform:**
```go
name := func() Option_string {
    if user == nil {
        return Option_string{tag: OptionTag_None}
    }
    __tmp0 := user.address
    if __tmp0 == nil {
        return Option_string{tag: OptionTag_None}
    }
    __tmp1 := __tmp0.city
    return Option_string{tag: OptionTag_Some, some_0: __tmp1}
}()
```

**Files:**
- `/Users/jack/mag/dingo/pkg/plugin/builtin/safe_navigation.go` (fix)

**Changes:**

```go
func (p *SafeNavigationPlugin) Transform(file *ast.File, ctx *plugin.Context) error {
    // When building chain, track return type of each step
    // If previous step returned Option, unwrap before next access

    for _, navStep := range navigationChain {
        // NEW: Use type inference to determine step return type
        stepType, err := ctx.TypeInference.InferType(navStep)
        if err == nil {
            if T, isOption := ctx.TypeInference.IsOptionType(stepType); isOption {
                // This step returns Option[T], need to unwrap for next step
                navStep = p.unwrapOptionForChain(navStep, T, ctx)
            }
        }
    }

    // Final result wraps entire chain in Option
}
```

**Testing:**
- Test single ?. (existing behavior)
- Test chained ?. with Option returns
- Test mixed chaining

---

### Phase 3 Deliverables

- [ ] Configuration system implemented (dingo.toml support)
- [ ] Result[T, E] literals working (Ok/Err)
- [ ] Option[T] literals working (Some/None)
- [ ] Go (T, error) auto-wrapping functional (configurable)
- [ ] Error propagation (?) works with Result types
- [ ] Null coalescing (??) works with Option types
- [ ] Safe navigation (?.) chaining bug fixed
- [ ] 30+ new tests passing (Result/Option coverage)
- [ ] Golden file tests for Result/Option scenarios
- [ ] Documentation in /features/*.md updated

**Success Criteria:** Result and Option are production-ready, auto-wrapping works seamlessly with configuration control.

---

## Phase 4: Parser Enhancements (2-3 hours)

**Goal:** Fix advanced syntax parsing to unlock remaining golden file tests.

### 4.1 Current State

**Golden Tests:** 4/20 passing (20% pass rate)

**Root Cause:** Parser doesn't support advanced syntax patterns, blocking tests even when transformation logic is correct.

**Known Gaps:**
1. Ternary operator parsing fails ("unexpected ':'")
2. Pattern destructuring not supported (struct fields, tuple unpacking)
3. Map type syntax not recognized
4. Type declaration statements not parsed
5. String escape sequences may have issues

---

### 4.2 Priority Fixes

#### Fix 4.2.1: Ternary Operator Precedence (30 min)

**Files:**
- `/Users/jack/mag/dingo/pkg/parser/participle.go`

**Issue:** Parser fails on `condition ? value : other` with "unexpected ':'"

**Diagnosis Steps:**
1. Locate ternary operator grammar rule
2. Check if colon is recognized in ternary context
3. Verify operator precedence (ternary should be low precedence)

**Expected Fix:**

```go
// Current (likely missing or incorrect)
type Expression struct {
    // ... other operators ...
}

// Fixed
type Expression struct {
    Ternary *TernaryExpr `parser:"@@"`
    // ... other operators ...
}

type TernaryExpr struct {
    Condition  *LogicalOr `parser:"@@"`
    TrueExpr   *LogicalOr `parser:"( '?' @@ "`
    FalseExpr  *LogicalOr `parser:"  ':' @@ )?"`
}
```

**Precedence Order (lowest to highest):**
1. Ternary (? :)
2. Logical OR (||)
3. Logical AND (&&)
4. Equality (==, !=)
5. Comparison (<, >, <=, >=)
6. Additive (+, -)
7. Multiplicative (*, /, %)
8. Unary (!, -, &, *)
9. Primary (literals, identifiers, calls)

**Testing:**
- Simple ternary: `a ? b : c`
- Nested ternary: `a ? b ? c : d : e`
- With operators: `x > 0 ? y + 1 : z - 1`
- In assignment: `v := cond ? 10 : 20`

---

#### Fix 4.2.2: Pattern Destructuring (1 hour)

**Files:**
- `/Users/jack/mag/dingo/pkg/parser/participle.go`
- `/Users/jack/mag/dingo/pkg/ast/ast.go` (may need new nodes)

**Issue:** Match patterns like `Circle{radius}` not parsed

**Current Pattern Grammar (estimated):**
```go
type Pattern struct {
    Identifier *string `parser:"@Ident"`
    Wildcard   bool    `parser:"| @'_'"`
}
```

**Enhanced Pattern Grammar:**

```go
type Pattern struct {
    // Simple identifier binding
    Identifier *string `parser:"@Ident"`

    // Wildcard
    Wildcard bool `parser:"| @'_'"`

    // Struct destructuring: Point{x, y} or Point{x: newX, y: newY}
    StructPattern *StructPattern `parser:"| @@"`

    // Tuple destructuring: Point(x, y)
    TuplePattern *TuplePattern `parser:"| @@"`

    // Literal matching: 42, "string", true
    Literal *Literal `parser:"| @@"`
}

type StructPattern struct {
    TypeName string         `parser:"@Ident '{'"`
    Fields   []FieldPattern `parser:"@@? ( ',' @@ )* '}'"`
}

type FieldPattern struct {
    // Field name
    Field string `parser:"@Ident"`

    // Optional binding: x or x: newName
    Binding *string `parser:"( ':' @Ident )?"`
}

type TuplePattern struct {
    TypeName string    `parser:"@Ident '('"`
    Patterns []Pattern `parser:"@@? ( ',' @@ )* ')'"`
}
```

**Testing:**
- Struct pattern: `Circle{radius}`
- Struct with rename: `Point{x: px, y: py}`
- Tuple pattern: `Some(value)`
- Nested patterns: `Result{ok_0: Some(x)}`

---

#### Fix 4.2.3: Map Type Syntax (30 min)

**Files:**
- `/Users/jack/mag/dingo/pkg/parser/participle.go`

**Issue:** `map[string]int` not recognized

**Current Type Grammar (estimated):**
```go
type Type struct {
    Name    *string `parser:"@Ident"`
    Pointer bool    `parser:"@'*'?"`
    // ...
}
```

**Enhanced Type Grammar:**

```go
type Type struct {
    // Map type: map[K]V
    MapType *MapType `parser:"@@"`

    // Array/Slice: []T or [N]T
    ArrayType *ArrayType `parser:"| @@"`

    // Pointer: *T
    PointerType *PointerType `parser:"| @@"`

    // Named type: int, string, MyStruct
    NamedType *string `parser:"| @Ident"`

    // Generic type: Result[T, E]
    GenericType *GenericType `parser:"| @@"`
}

type MapType struct {
    KeyType   *Type `parser:"'map' '[' @@ ']'"`
    ValueType *Type `parser:"@@"`
}
```

**Testing:**
- Simple map: `map[string]int`
- Nested map: `map[string]map[int]bool`
- Map with struct value: `map[string]User`

---

#### Fix 4.2.4: Type Declarations (30 min)

**Files:**
- `/Users/jack/mag/dingo/pkg/parser/participle.go`

**Issue:** `type User struct { ... }` statements not parsed

**Current Declaration Grammar (estimated):**
```go
type Declaration struct {
    Function *FuncDecl `parser:"@@"`
    Variable *VarDecl  `parser:"| @@"`
    // Missing: Type declaration
}
```

**Enhanced Declaration Grammar:**

```go
type Declaration struct {
    Function *FuncDecl `parser:"@@"`
    Variable *VarDecl  `parser:"| @@"`
    Type     *TypeDecl `parser:"| @@"`  // NEW
    Enum     *EnumDecl `parser:"| @@"`  // Already exists for sum types
}

type TypeDecl struct {
    Name string `parser:"'type' @Ident"`
    Type *Type  `parser:"@@"`
}
```

**Note:** This supports type aliases. Type definitions (new type) may need:
```go
type TypeDecl struct {
    Name       string `parser:"'type' @Ident"`
    IsNewType  bool   `parser:"@'='?"`  // = for alias, no = for new type
    Type       *Type  `parser:"@@"`
}
```

**Testing:**
- Type alias: `type UserID = string`
- Struct type: `type User struct { Name string }`
- New type: `type UserID string`

---

### 4.3 Implementation Strategy

**Incremental Approach:**
1. Fix one syntax feature at a time
2. Run parser tests after each fix
3. Run golden file tests to see how many unlock
4. Document remaining failures

**Testing Workflow:**
```bash
# After each fix
go test ./pkg/parser/... -v
go test ./tests/golden/... -v | grep PASS | wc -l  # Count passing tests
```

**Expected Progression:**
- After ternary fix: 6-8/20 golden tests passing
- After pattern destructuring: 10-12/20 passing
- After map types: 13-15/20 passing
- After type declarations: 15-18/20 passing

---

### Phase 4 Deliverables

- [ ] Ternary operator fully parsed
- [ ] Pattern destructuring supported (struct and tuple)
- [ ] Map types parsed
- [ ] Type declarations parsed
- [ ] 15+/20 golden tests passing (75%+ pass rate)
- [ ] Remaining failures documented with clear TODOs
- [ ] Parser grammar documented

**Success Criteria:** Parser no longer blocks feature development, 75%+ golden tests pass.

---

## Cross-Phase Integration Points

### Phase 1 → Phase 2
- Clean test baseline enables confident Phase 2 changes
- No false failures masking new issues

### Phase 2 → Phase 3
- TypeInferenceService is prerequisite for Result/Option
- IsResultType(), IsOptionType() enable transformations
- Synthetic type registry enables cross-plugin detection

### Phase 3 → Phase 4
- Parser enhancements unlock advanced Result/Option syntax
- Pattern destructuring enables elegant match expressions
- Type declarations enable explicit Result/Option types

### Phase 4 → Future
- Parser foundation supports future syntax additions
- Clean golden test suite validates future features

---

## Risk Management

### High Risk: Type Inference Performance

**Risk:** go/types checking could slow builds beyond acceptable threshold (>15%)

**Mitigation:**
1. Cache type information per expression
2. Only refresh after transformations (lazy checking)
3. Monitor performance with benchmarks
4. Make type inference optional if needed

**Contingency:**
- Implement aggressive caching
- Consider incremental type checking
- Fallback: Type inference opt-in only

**Probability:** Low (go/types is well-optimized)
**Impact:** Medium (performance degradation)

---

### Medium Risk: Auto-wrapping Surprises

**Risk:** Automatic Result/Option wrapping triggers unexpectedly, confusing users

**Mitigation:**
1. Conservative wrapping (only when LHS explicitly expects Result/Option)
2. Configuration control (auto_wrap_go_errors in dingo.toml)
3. Clear documentation and examples
4. Warning logs when wrapping occurs

**Contingency:**
- Disable auto-wrapping by default (require opt-in)
- Add explicit wrapper syntax (Result.from(goFunc()))

**Probability:** Medium
**Impact:** Medium (user confusion)

---

### Medium Risk: Parser Grammar Conflicts

**Risk:** New syntax conflicts with existing grammar, causing parse errors

**Mitigation:**
1. Careful precedence management
2. Incremental testing after each change
3. Review existing tests for regressions
4. Use parser debugger if available

**Contingency:**
- Deferred syntax features don't block core functionality
- Alternative syntax if conflicts unresolvable

**Probability:** Low-Medium
**Impact:** Low (syntax features can be deferred)

---

### Low Risk: Test Coverage Gaps

**Risk:** New features not adequately tested, bugs slip through

**Mitigation:**
1. Comprehensive unit tests for each feature
2. Integration tests for cross-plugin scenarios
3. Golden file tests for end-to-end validation
4. Code coverage monitoring (target 95%+)

**Contingency:**
- Add tests retroactively when bugs discovered
- Require tests for bug fixes

**Probability:** Low
**Impact:** Low (tests can be added incrementally)

---

## Success Metrics

### Phase 1 Success
- [ ] 97/97 plugin tests passing
- [ ] 0 failing CI/CD tests
- [ ] All test failures triaged and fixed

### Phase 2 Success
- [ ] TypeInferenceService integrated
- [ ] All plugins access shared type info
- [ ] Build time increase < 15%
- [ ] 100% backward compatibility

### Phase 3 Success
- [ ] Result[T, E] fully functional
- [ ] Option[T] fully functional
- [ ] Auto-wrapping configurable and working
- [ ] 30+ new tests passing
- [ ] Golden tests for Result/Option

### Phase 4 Success
- [ ] 15+/20 golden tests passing
- [ ] Ternary, patterns, maps, types parsed
- [ ] Parser documented

### Overall Success
- [ ] All 4 phases complete
- [ ] Performance within budget
- [ ] 95%+ test coverage
- [ ] CHANGELOG.md updated
- [ ] Ready for next phase

---

## Timeline Estimate

**Total:** 18-24 hours development time

### Week 1 (Days 1-5)
- **Day 1 (3h):** Phase 1 complete
- **Day 2 (4h):** Phase 2 Steps 2.2.1-2.2.2
- **Day 3 (4h):** Phase 2 Steps 2.2.3-2.2.5
- **Day 4 (4h):** Phase 3 Config + Result Type
- **Day 5 (4h):** Phase 3 Option Type

### Week 2 (Days 6-7)
- **Day 6 (3h):** Phase 4 Parser Fixes
- **Day 7 (2h):** Polish, Documentation, CHANGELOG

**Buffer:** 2-4 hours for unexpected issues

---

## Implementation Order Summary

1. **Phase 1: Test Stabilization** (2-3h)
   - Fix error propagation tests (field names)
   - Fix lambda tests (add contains() or rewrite)
   - Fix marker tests (debug injection)
   - Fix parser tests (ternary grammar)

2. **Phase 2: Type Inference System** (6-8h)
   - Enhance plugin.Context with TypeInference field
   - Refactor TypeInference → TypeInferenceService
   - Add Result/Option detection methods
   - Update ErrorPropagationPlugin to use shared service
   - Update SumTypesPlugin to register synthetic types
   - Comprehensive testing

3. **Phase 3: Result/Option Completion** (6-8h)
   - Implement configuration system (dingo.toml)
   - Result type: literals, auto-wrapping, error propagation
   - Option type: literals, null coalescing, safe navigation
   - Integration testing

4. **Phase 4: Parser Enhancements** (2-3h)
   - Fix ternary operator
   - Fix pattern destructuring
   - Fix map types
   - Fix type declarations
   - Golden test validation

---

## Next Steps

1. **User Approval:** Review and approve this final plan
2. **Setup:** Ensure development environment ready
3. **Phase 1 Kickoff:** Start with test stabilization (quick wins)
4. **Daily Progress:** Ship each phase independently, validate before moving on
5. **Documentation:** Update CHANGELOG.md after each phase

---

## Appendix: File Modification Summary

### Phase 1 Files (4 files, ~50 lines)
- `/Users/jack/mag/dingo/pkg/plugin/builtin/error_propagation_test.go`
- `/Users/jack/mag/dingo/pkg/plugin/builtin/lambda_test.go`
- `/Users/jack/mag/dingo/pkg/generator/markers_test.go`
- `/Users/jack/mag/dingo/pkg/parser/new_features_test.go`

### Phase 2 Files (7 files, ~400 lines)
- `/Users/jack/mag/dingo/pkg/plugin/plugin.go`
- `/Users/jack/mag/dingo/pkg/plugin/pipeline.go`
- `/Users/jack/mag/dingo/pkg/plugin/builtin/type_inference.go` (major)
- `/Users/jack/mag/dingo/pkg/plugin/builtin/error_propagation.go`
- `/Users/jack/mag/dingo/pkg/plugin/builtin/sum_types.go`
- `/Users/jack/mag/dingo/pkg/plugin/builtin/null_coalescing.go`
- `/Users/jack/mag/dingo/pkg/plugin/builtin/safe_navigation.go`

### Phase 3 Files (6 files, ~800 lines)
- `/Users/jack/mag/dingo/pkg/config/config.go` (new)
- `/Users/jack/mag/dingo/pkg/plugin/builtin/result_type.go` (major)
- `/Users/jack/mag/dingo/pkg/plugin/builtin/option_type.go` (major)
- `/Users/jack/mag/dingo/pkg/plugin/builtin/error_propagation.go`
- `/Users/jack/mag/dingo/pkg/plugin/builtin/null_coalescing.go`
- `/Users/jack/mag/dingo/pkg/plugin/builtin/safe_navigation.go`

### Phase 4 Files (2 files, ~200 lines)
- `/Users/jack/mag/dingo/pkg/parser/participle.go` (major)
- `/Users/jack/mag/dingo/pkg/ast/ast.go`

### Test Files (6+ files, ~800 lines)
- `/Users/jack/mag/dingo/pkg/plugin/builtin/type_inference_test.go`
- `/Users/jack/mag/dingo/pkg/plugin/builtin/result_type_test.go` (new)
- `/Users/jack/mag/dingo/pkg/plugin/builtin/option_type_test.go` (new)
- `/Users/jack/mag/dingo/tests/golden/result_basic.dingo` (new)
- `/Users/jack/mag/dingo/tests/golden/option_basic.dingo` (new)
- `/Users/jack/mag/dingo/tests/integration/type_inference_test.go` (new)

**Total:** ~25 files, ~2,250 lines of code (production + tests)

---

**END OF FINAL PLAN**
