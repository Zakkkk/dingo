# Test Compilation Fix - Architectural Plan

## Problem Summary

The previous session (7675185) successfully implemented Fix A2 (Constructor AST Mutation) and Fix A3 (Type Inference) for the Result[T,E] type plugin. However, these refactoring changes altered several APIs, causing compilation errors in the test suite:

**Test Files with Errors:**
- `tests/error_propagation_test.go` - 4 undefined symbol errors
- `tests/golden_test.go` - 3 undefined symbol/API errors

**Root Cause:**
The plugin refactoring removed old factory functions and changed the plugin architecture, but the test files still reference the obsolete APIs.

## API Changes Analysis

### 1. Removed Factory Functions

**OLD APIs (no longer exist):**
- `builtin.NewErrorPropagationPlugin()` - REMOVED
- `builtin.NewSumTypesPlugin()` - REMOVED
- `builtin.NewTypeInference()` - REMOVED (renamed)
- `builtin.StatementLifter` - Type/package no longer exists
- `builtin.NewStatementLifter()` - REMOVED

**NEW APIs (current implementation):**
- `builtin.NewResultTypePlugin()` - Implements Result[T,E] type with Ok/Err constructors
- `builtin.NewOptionTypePlugin()` - Implements Option[T] type with Some/None
- `builtin.NewTypeInferenceService(fset, file, logger)` - Type inference for Result/Option types
- Plugin components are now internal to the plugins (no exposed StatementLifter, ErrorWrapper, etc.)

### 2. Registry API Changes

**OLD API:**
```go
registry.Register(plugin)  // Method existed
```

**NEW API:**
```go
registry := plugin.NewRegistry()  // Registry is now a stub
// No Register() method - registry is passive
```

The `Registry` type in `pkg/plugin/plugin.go` is now a minimal stub with no registration logic. Plugins are instantiated directly and not registered.

### 3. Logger Interface Changes

**OLD Logger Interface (expected by tests):**
```go
type Logger interface {
    Info(format string, args ...interface{})
    Warn(format string, args ...interface{})
    Error(format string, args ...interface{})
    Debug(format string, args ...interface{})
}
```

**NEW Logger Interface (actual):**
```go
type Logger interface {
    Info(msg string)                          // NO format/args
    Error(msg string)                         // NO format/args
    Debug(format string, args ...interface{}) // Format/args
    Warn(format string, args ...interface{})  // Format/args
}
```

**Mismatch:**
The test's `testLogger` implements methods with `(format string, args ...interface{})` for Info/Error, but the interface expects `(msg string)` only.

### 4. Plugin Architecture Changes

**OLD Architecture (implied by tests):**
- Separate "ErrorPropagationPlugin" for `?` operator
- Separate "SumTypesPlugin" for sum types
- Separate "TypeInference" service
- Exposed components: StatementLifter, ErrorWrapper

**NEW Architecture (actual implementation):**
- `ResultTypePlugin` - Handles Result[T,E] type generation, Ok/Err constructors
- `OptionTypePlugin` - Handles Option[T] type generation, Some/None
- `TypeInferenceService` - Type inference for both Result and Option
- Internal components - No exposed StatementLifter or ErrorWrapper

**Key Insight:**
The "ErrorPropagationPlugin" concept no longer exists as a separate plugin. Error propagation (`?` operator) will likely be a separate concern from Result type generation.

## Recommended Approach

### Strategy: Minimal Test Updates

Given that:
1. Core implementation is complete and reviewed (75% approval rate)
2. Tests are checking obsolete APIs
3. No backward compatibility is required (pre-release)

**Approach:**
1. Update test files to use the new plugin APIs
2. Remove tests for components that are now internal/private
3. Keep tests focused on the actual public API surface
4. Preserve test intent (smoke tests, type inference validation, etc.)

### Phase 1: Fix `tests/error_propagation_test.go`

**Changes Required:**

1. **TestErrorPropagationSmokeTests** (lines 15-174)
   - **Issue:** Line 139 uses `builtin.NewErrorPropagationPlugin()`
   - **Fix:** Replace with `builtin.NewResultTypePlugin()`
   - **Rationale:** Result type plugin is the new home for constructor transformations

2. **testLogger** (lines 389-408)
   - **Issue:** Lines 394-396 implement `Info(format, args...)` but interface expects `Info(msg)`
   - **Fix:** Change signatures to match interface:
     ```go
     func (l *testLogger) Info(msg string) {
         l.t.Log("[INFO] " + msg)
     }
     func (l *testLogger) Error(msg string) {
         l.t.Log("[ERROR] " + msg)
     }
     ```

3. **TestTypeInference** (lines 176-270)
   - **Issue:** Line 231 uses `builtin.NewTypeInference(fset, file)`
   - **Fix:** Replace with `builtin.NewTypeInferenceService(fset, file, logger)`
   - **Note:** New API requires logger parameter

4. **TestStatementLifter** (lines 272-313)
   - **Issue:** Lines 276, 281 reference `builtin.StatementLifter` and `builtin.NewStatementLifter()`
   - **Fix:** **DELETE THIS TEST**
   - **Rationale:** StatementLifter is now internal to ResultTypePlugin, not exposed API

### Phase 2: Fix `tests/golden_test.go`

**Changes Required:**

1. **TestGoldenFiles** (lines 19-109)
   - **Issue:** Line 75 uses `builtin.NewErrorPropagationPlugin()`
   - **Fix:** Replace with `builtin.NewResultTypePlugin()`

   - **Issue:** Line 76 calls `registry.Register(errPropPlugin)`
   - **Fix:** **REMOVE** - Registry no longer has Register method

   - **Issue:** Line 79 uses `builtin.NewSumTypesPlugin()`
   - **Fix:** **DECISION NEEDED** - Does this plugin exist? Check if it's OptionTypePlugin
   - **Likely Fix:** Replace with `builtin.NewOptionTypePlugin()` OR remove if not implemented

2. **Generator Integration**
   - **Issue:** Line 84 creates generator with plugins: `generator.NewWithPlugins(fset, registry, logger)`
   - **Fix:** Need to verify if generator API changed
   - **Investigation Needed:** Check `pkg/generator/generator.go` to see if it still accepts plugins this way

### Phase 3: Verify Generator API

**Action Required:**
Examine `pkg/generator/generator.go` to understand:
1. Does `NewWithPlugins()` still exist?
2. How should plugins be passed to the generator?
3. Is the Registry still used, or are plugins passed directly?

**Possible Outcomes:**
- **A) Generator uses plugins directly** → Pass plugin instances as variadic args
- **B) Generator uses registry** → Registry needs Register() method restored
- **C) Generator doesn't use plugins yet** → Remove plugin passing from tests

## Package Structure

### Current Plugin Files

Located in `/Users/jack/mag/dingo/pkg/plugin/builtin/`:
1. `result_type.go` - ResultTypePlugin (1337 lines) - **PRIMARY**
2. `option_type.go` - OptionTypePlugin (619 lines) - **SECONDARY**
3. `type_inference.go` - TypeInferenceService (405 lines) - **SUPPORT**
4. `builtin.go` - Factory functions (27 lines) - **DEPRECATED STUBS**
5. `result_type_test.go` - Unit tests for ResultTypePlugin

### Plugin Interface

From `pkg/plugin/plugin.go`:
```go
type Plugin interface {
    Name() string
    Process(node ast.Node) error
}
```

### Context Structure

```go
type Context struct {
    FileSet     *token.FileSet
    TypeInfo    interface{}
    Config      *Config
    Registry    *Registry
    Logger      Logger
    CurrentFile interface{}
}
```

## Key Interfaces/Types

### 1. ResultTypePlugin

**Public Methods:**
- `NewResultTypePlugin() *ResultTypePlugin` - Constructor
- `Name() string` - Returns "result_type"
- `Process(node ast.Node) error` - Walks AST to find Result types
- `Transform(node ast.Node) (ast.Node, error)` - Transforms Ok/Err calls
- `GetPendingDeclarations() []ast.Decl` - Returns generated type declarations
- `ClearPendingDeclarations()`

**Internal Components (not exposed):**
- Constructor transformation (Ok/Err → struct literals)
- Type name sanitization
- AST generation for Result struct and methods

### 2. OptionTypePlugin

**Public Methods:**
- `NewOptionTypePlugin() *OptionTypePlugin`
- `Name() string` - Returns "option_type"
- `Process(node ast.Node) error`
- `SetTypeInference(*TypeInferenceService)`
- `GetPendingDeclarations() []ast.Decl`
- `ClearPendingDeclarations()`

### 3. TypeInferenceService

**Public Methods:**
- `NewTypeInferenceService(fset *token.FileSet, file *ast.File, logger Logger) (*TypeInferenceService, error)`
- `IsResultType(typeName string) bool`
- `IsOptionType(typeName string) bool`
- `GetResultTypeParams(typeName string) (T, E types.Type, ok bool)`
- `GetOptionTypeParam(typeName string) (T types.Type, ok bool)`
- `RegisterResultType(typeName string, okType, errType types.Type)`
- `RegisterOptionType(typeName string, valueType types.Type)`

### 4. Logger Interface (CRITICAL)

**Actual Interface:**
```go
type Logger interface {
    Info(msg string)                          // NO variadic args
    Error(msg string)                         // NO variadic args
    Debug(format string, args ...interface{}) // Has variadic args
    Warn(format string, args ...interface{})  // Has variadic args
}
```

**NoOpLogger Implementation:**
```go
func (n *NoOpLogger) Info(msg string)                            {}
func (n *NoOpLogger) Error(msg string)                           {}
func (n *NoOpLogger) Debug(format string, args ...interface{})   {}
func (n *NoOpLogger) Warn(format string, args ...interface{})    {}
```

## Implementation Notes

### Critical Details

1. **No StatementLifter Exposure**
   - Statement lifting is internal to ResultTypePlugin
   - Test must be removed, not updated

2. **No ErrorWrapper Exposure**
   - Error wrapping is internal (if it exists at all)
   - Test can be kept as it uses internal `NewErrorWrapper()` which still exists

3. **Plugin Context Initialization**
   - Plugins need `ctx *plugin.Context` to be set before use
   - Tests call `Transform(ctx, file)` which implies plugins expose this method
   - **VERIFY:** Does ResultTypePlugin have a `Transform(ctx, file)` method? YES (line 1301-1336)

4. **Transform Method Signature**
   - `ResultTypePlugin.Transform(node ast.Node) (ast.Node, error)`
   - Tests call `p.Transform(ctx, file)` - **MISMATCH**
   - Tests expect: `Transform(*Context, *ast.File) (*ast.File, error)`
   - Actual: `Transform(ast.Node) (ast.Node, error)`

### Gotchas

1. **Context Setup**
   - Tests create context manually, then call plugin methods
   - Plugins expect `p.ctx` to be set (it's a field on the plugin)
   - **Missing:** How does ctx get set on the plugin?
   - **Likely:** Need to add `SetContext(*plugin.Context)` method or pass it to Transform

2. **Registry is a Stub**
   - Current Registry has no methods beyond `NewRegistry()`
   - Tests assume `Register()` exists - it doesn't
   - **Decision:** Either restore Register() or remove registry usage from tests

3. **Sum Types vs Options**
   - Tests reference "SumTypesPlugin"
   - Actual code has "OptionTypePlugin"
   - **Assumption:** SumTypesPlugin was renamed to OptionTypePlugin
   - **Verify:** Are these the same thing?

## Testing Strategy

### Test Coverage Goals

**KEEP (update to new APIs):**
1. ✅ ResultTypePlugin smoke tests (basic AST processing)
2. ✅ TypeInferenceService tests (type parsing and inference)
3. ✅ Golden file tests (end-to-end transpilation)
4. ✅ ErrorWrapper tests (if still relevant)

**REMOVE (internal implementation):**
1. ❌ StatementLifter tests (now internal)

**ADD (new coverage needed):**
1. 🆕 Ok() constructor transformation tests
2. 🆕 Err() constructor transformation tests
3. 🆕 Result type declaration generation tests

## Alternatives Considered

### Option A: Restore Old APIs (Rejected)

**Pros:**
- Tests pass without modification
- No need to understand new architecture

**Cons:**
- Defeats purpose of refactoring
- Maintains obsolete API surface
- Backward compatibility not a requirement

**Verdict:** ❌ Rejected

### Option B: Delete All Tests (Rejected)

**Pros:**
- Fast
- No compatibility concerns

**Cons:**
- Loses test coverage
- Loses documentation value of tests
- Reckless for critical infrastructure

**Verdict:** ❌ Rejected

### Option C: Minimal Surgical Updates (SELECTED)

**Pros:**
- Preserves test intent
- Minimal changes
- Tests document new API
- Fast to implement

**Cons:**
- Requires understanding both old and new APIs

**Verdict:** ✅ Selected

## Dependencies

### External Packages
- `go/ast` - AST manipulation
- `go/parser` - Parsing Go code
- `go/printer` - Printing AST to Go code
- `go/token` - Token positions
- `go/types` - Type checking
- `golang.org/x/tools/go/ast/astutil` - AST utilities
- `github.com/stretchr/testify` - Test assertions

### Internal Packages
- `github.com/MadAppGang/dingo/pkg/plugin` - Plugin interface
- `github.com/MadAppGang/dingo/pkg/plugin/builtin` - Builtin plugins
- `github.com/MadAppGang/dingo/pkg/generator` - Code generator
- `github.com/MadAppGang/dingo/pkg/parser` - Dingo parser

## Quality Checks

Before finalizing implementation:

- [ ] Do tests use only public APIs?
- [ ] Are all plugin factory functions correct?
- [ ] Does testLogger match Logger interface exactly?
- [ ] Are tests focused on behavior, not implementation?
- [ ] Do golden tests still make sense with new plugin architecture?
- [ ] Is Context properly initialized for plugin tests?
- [ ] Does generator integration work with new plugin system?

## Risk Assessment

### Low Risk
- ✅ Updating factory function calls (straightforward rename)
- ✅ Fixing Logger interface mismatch (simple signature change)
- ✅ Removing StatementLifter test (internal component)

### Medium Risk
- ⚠️ Generator integration (need to verify API)
- ⚠️ Plugin context initialization (unclear how ctx is set)
- ⚠️ Registry usage (stub implementation may need enhancement)

### High Risk
- 🔴 Transform method signature mismatch (tests vs actual)
- 🔴 Missing plugin lifecycle (Context setup)

## Next Steps

1. **Investigate Generator API** - Check `pkg/generator/generator.go` for plugin integration
2. **Verify Plugin Lifecycle** - How does Context get set on plugins?
3. **Clarify Transform Signature** - Reconcile test expectations with actual API
4. **Implement Fixes** - Update tests with correct APIs
5. **Run Tests** - Verify fixes resolve compilation errors
6. **Validate Golden Tests** - Ensure end-to-end transpilation still works

## Success Criteria

✅ All tests compile without errors
✅ Tests use only public APIs from new plugin system
✅ Test intent is preserved (smoke tests, type inference, golden files)
✅ No obsolete APIs referenced
✅ Logger interface matches exactly
✅ Generator integration works correctly
