# Architectural Plan: Parser Fix & Result[T,E] Integration

## Executive Summary

**Status**: Parser already handles `:` syntax correctly via preprocessor. Main blocker is missing `enum` preprocessor for sum types. Result type plugin infrastructure exists but is not activated in pipeline.

**Key Finding**: The error "missing ',' in parameter list" does NOT occur with current code. The preprocessor's `TypeAnnotProcessor` successfully transforms `path: string` → `path string` before parsing.

**Actual Blocker**: The `enum` keyword is not recognized because there is no enum preprocessor, blocking Result type tests which use `enum Result { Ok(T), Err(E) }` syntax.

---

## Part 1: Architecture Analysis

### Current Pipeline Architecture

```
.dingo source → Preprocessor → go/parser → AST → Generator → .go output
                    ↓                           ↓
              (transforms)              (no transformation)
```

**Preprocessor Chain** (`pkg/preprocessor/preprocessor.go`):
1. `TypeAnnotProcessor` - Converts `:` → space in params ✅ WORKING
2. `ErrorPropProcessor` - Converts `expr?` → error handling ✅ WORKING
3. `KeywordProcessor` - Converts `let` → `var` ✅ WORKING
4. **Missing**: `EnumProcessor` - Would convert `enum` to Go sum types ❌ NOT IMPLEMENTED
5. **Missing**: `SumTypeProcessor` - Would handle variants ❌ NOT IMPLEMENTED

**Generator Pipeline** (`pkg/generator/generator.go`):
- Uses `plugin.Pipeline` which is currently a **NO-OP**
- `Transform()` method returns AST unchanged (line 28-29 in `pkg/plugin/plugin.go`)
- Result type plugin exists but is **never activated**

**Result Type Plugin** (`pkg/plugin/builtin/result_type.go`):
- ✅ Complete implementation (1337 lines)
- ✅ Fix A2: Constructor transformation (`Ok()` → `CompositeLit`)
- ✅ Fix A3: Type inference service
- ✅ Helper methods (`IsOk()`, `Unwrap()`, etc.)
- ❌ **Not integrated into pipeline**

---

## Part 2: Two-Phase Implementation Plan

### Phase 1: Fix Parser/Preprocessor (BLOCKER - 4-6 hours)

**Problem**: `enum` keyword causes parse errors because it's not valid Go syntax.

**Solution**: Add enum preprocessor to transform enum declarations before parsing.

#### Task 1.1: Create Enum Preprocessor (2-3 hours)

**File**: `pkg/preprocessor/enum.go`

**Responsibilities**:
- Detect `enum Name { Variant1(T), Variant2(E) }` syntax
- Transform to Go sum type representation:
  ```go
  type Name struct {
      tag NameTag
      variant1_0 *T
      variant2_0 *E
  }

  type NameTag uint8
  const (
      NameTag_Variant1 NameTag = iota
      NameTag_Variant2
  )
  ```
- Generate constructor functions: `Name_Variant1()`, `Name_Variant2()`
- Handle multiple variants and complex type parameters
- Maintain source mappings for LSP

**Key Design Decisions**:

1. **Preprocessing vs. AST Transformation**:
   - ✅ **Recommendation**: Preprocessing (text-level)
   - **Rationale**: Matches existing architecture (type annotations, error prop, keywords all use preprocessing)
   - **Trade-off**: Less precise than AST transformation, but simpler and consistent
   - **Alternative**: AST transformation (more complex, requires parser changes)

2. **Enum Representation**:
   - ✅ **Recommendation**: Tagged union with pointer fields
   - **Rationale**: Matches Result type plugin implementation
   - **Benefits**: Memory efficient, nil-safe, explicit discriminator
   - **Example**: See Result type (lines 22-27 in `result_type.go`)

3. **Syntax Support**:
   - ✅ Simple enums: `enum Color { Red, Green, Blue }`
   - ✅ Enums with data: `enum Result { Ok(T), Err(E) }`
   - ⚠️ Defer: Generic syntax `enum<T>` (Phase 3)

**Implementation Strategy**:

```go
// Pattern matching approach
type EnumProcessor struct {
    enumPattern *regexp.Regexp  // Match: enum Name { ... }
    variantPattern *regexp.Regexp  // Match: Variant(Type)
}

func (e *EnumProcessor) Process(source []byte) ([]byte, []Mapping, error) {
    // 1. Find all enum declarations
    // 2. Parse variant list
    // 3. Generate struct + tag + constructors
    // 4. Replace enum block with generated code
    // 5. Track source mappings
}
```

**Integration Point**: Add to `preprocessor.New()` at line 64 (before keywords):
```go
processors: []FeatureProcessor{
    NewTypeAnnotProcessor(),
    NewErrorPropProcessorWithConfig(config),
    NewEnumProcessor(),  // ← ADD HERE
    NewKeywordProcessor(),
}
```

#### Task 1.2: Add Preprocessor Configuration (1 hour)

**File**: `pkg/preprocessor/config.go`

**Add Enum Config Options**:
```go
type Config struct {
    ErrorPropagationMode string

    // New: Enum options
    EnumEnabled bool  // Default: true
    EnumTagType string  // "uint8" (default), "int", "string"
    EnumUsePointers bool  // true (default) for nil safety
}
```

**Rationale**: Allow disabling enum processing for debugging, and configure representation strategy.

#### Task 1.3: Test Enum Preprocessor (1-2 hours)

**Files**:
- `pkg/preprocessor/enum_test.go`
- Use existing golden tests for validation

**Test Cases**:
1. Simple enum: `enum Status { Pending, Active, Done }`
2. Enum with single type: `enum Option { Some(T), None }`
3. Enum with multiple types: `enum Result { Ok(T), Err(E) }`
4. Nested types: `enum Response { Data([]User), Error(string) }`
5. Edge cases: Empty enum, single variant, long names

**Validation**:
- Preprocessed output must be valid Go (passes `go/parser`)
- Source mappings must be accurate
- Generated constructors must compile

---

### Phase 2: Complete Result[T,E] Integration (END-TO-END - 6-8 hours)

**Goal**: Activate Result type plugin and enable end-to-end workflow.

#### Task 2.1: Implement Plugin Pipeline (3-4 hours)

**File**: `pkg/plugin/plugin.go`

**Current State**: Pipeline is a stub (Transform() is no-op at line 28-29).

**Implementation**:

```go
type Pipeline struct {
    Ctx     *Context
    plugins []Plugin  // ← Add plugin list
}

func NewPipeline(registry *Registry, ctx *Context) (*Pipeline, error) {
    // Register builtin plugins
    plugins := []Plugin{
        builtin.NewResultTypePlugin(),
        // Future: Option, PatternMatch, etc.
    }

    // Initialize plugins with context
    for _, p := range plugins {
        if initializer, ok := p.(ContextAware); ok {
            initializer.SetContext(ctx)
        }
    }

    return &Pipeline{
        Ctx:     ctx,
        plugins: plugins,
    }, nil
}

func (p *Pipeline) Transform(file *ast.File) (*ast.File, error) {
    // Phase 1: Discovery pass (find Result types, Ok/Err calls)
    for _, plugin := range p.plugins {
        if err := plugin.Process(file); err != nil {
            return nil, err
        }
    }

    // Phase 2: Transformation pass (replace Ok/Err calls)
    transformed := file
    for _, plugin := range p.plugins {
        if transformer, ok := plugin.(Transformer); ok {
            var err error
            transformed, err = transformer.Transform(transformed)
            if err != nil {
                return nil, err
            }
        }
    }

    // Phase 3: Declaration injection pass (add Result type definitions)
    for _, plugin := range p.plugins {
        if declProvider, ok := plugin.(DeclarationProvider); ok {
            decls := declProvider.GetPendingDeclarations()
            transformed.Decls = append(decls, transformed.Decls...)
            declProvider.ClearPendingDeclarations()
        }
    }

    return transformed, nil
}
```

**Key Interfaces** (add to `plugin.go`):

```go
// ContextAware plugins need initialization with context
type ContextAware interface {
    SetContext(*Context)
}

// Transformer plugins can modify AST
type Transformer interface {
    Plugin
    Transform(ast.Node) (ast.Node, error)
}

// DeclarationProvider plugins inject top-level declarations
type DeclarationProvider interface {
    GetPendingDeclarations() []ast.Decl
    ClearPendingDeclarations()
}
```

**Integration**: Result type plugin already implements these interfaces (see lines 1301-1336).

#### Task 2.2: Update Result Type Plugin Integration (1 hour)

**File**: `pkg/plugin/builtin/result_type.go`

**Required Changes**:

1. **Add ContextAware implementation**:
```go
func (p *ResultTypePlugin) SetContext(ctx *plugin.Context) {
    p.ctx = ctx
    // Extract type information if available
    if ctx.TypeInfo != nil {
        p.typesInfo, _ = ctx.TypeInfo.(*types.Info)
    }
}
```

2. **Ensure DeclarationProvider is exported**:
   - Already implemented (lines 1291-1299) ✅

3. **Verify Transform() method**:
   - Already implemented (lines 1301-1336) ✅

**No major changes needed** - plugin is already compliant with required interfaces.

#### Task 2.3: Update Generator to Use Pipeline (1 hour)

**File**: `pkg/generator/generator.go`

**Current Issue**: Pipeline is created but plugins are not registered (line 51).

**Fix** (at line 51-52):
```go
pipeline, err := plugin.NewPipeline(registry, ctx)
if err != nil {
    return nil, fmt.Errorf("failed to create plugin pipeline: %w", err)
}

// ← ADD: Register builtin plugins
registry.Register(builtin.NewResultTypePlugin())
// Future: registry.Register(builtin.NewOptionTypePlugin())
```

**Alternative**: Register in `NewPipeline()` as shown in Task 2.1.

#### Task 2.4: Add Registry.Register() Method (30 minutes)

**File**: `pkg/plugin/plugin.go`

**Current State**: Registry is empty struct.

**Implementation**:
```go
type Registry struct {
    plugins map[string]Plugin
}

func NewRegistry() *Registry {
    return &Registry{
        plugins: make(map[string]Plugin),
    }
}

func (r *Registry) Register(p Plugin) {
    r.plugins[p.Name()] = p
}

func (r *Registry) Get(name string) (Plugin, bool) {
    p, ok := r.plugins[name]
    return p, ok
}

func (r *Registry) All() []Plugin {
    result := make([]Plugin, 0, len(r.plugins))
    for _, p := range r.plugins {
        result = append(result, p)
    }
    return result
}
```

#### Task 2.5: End-to-End Testing (1-2 hours)

**Test Workflow**:

1. **Create test case** (`tests/integration/result_e2e_test.go`):
```go
func TestResultTypeE2E(t *testing.T) {
    source := `
package main

enum Result {
    Ok(int),
    Err(error),
}

func divide(a, b int) Result {
    if b == 0 {
        return Err(errors.New("division by zero"))
    }
    return Ok(a / b)
}
`

    // 1. Preprocess
    prep := preprocessor.New([]byte(source))
    goSource, _, err := prep.Process()
    require.NoError(t, err)

    // 2. Parse
    fset := token.NewFileSet()
    p := parser.NewParser(0)
    file, err := p.ParseFile(fset, "test.dingo", []byte(goSource))
    require.NoError(t, err)

    // 3. Generate with plugins
    registry := plugin.NewRegistry()
    logger := plugin.NewNoOpLogger()
    gen, err := generator.NewWithPlugins(fset, registry, logger)
    require.NoError(t, err)

    goCode, err := gen.Generate(file)
    require.NoError(t, err)

    // 4. Verify output contains Result type definition
    assert.Contains(t, string(goCode), "type Result_int_error struct")
    assert.Contains(t, string(goCode), "type ResultTag uint8")
    assert.Contains(t, string(goCode), "func (r Result_int_error) IsOk()")

    // 5. Compile check (go build)
    tmpFile := filepath.Join(t.TempDir(), "result.go")
    os.WriteFile(tmpFile, goCode, 0644)

    cmd := exec.Command("go", "build", tmpFile)
    output, err := cmd.CombinedOutput()
    require.NoError(t, err, "Generated code should compile: %s", output)
}
```

2. **Validate golden tests**:
   - Run: `dingo build tests/golden/result_01_basic.dingo`
   - Expected: Successful compilation
   - Verify: Generated `.go` file compiles with `go build`

3. **Verify transformations**:
   - `Ok(42)` → `Result_int_error{tag: ResultTag_Ok, ok_0: &42}`
   - `Err(err)` → `Result_int_error{tag: ResultTag_Err, err_0: &err}`

#### Task 2.6: Update Golden Tests (1 hour)

**Files**: `tests/golden/result_*.go.golden`

**Action**: Regenerate expected output with proper transformations.

**Process**:
1. Run: `dingo build tests/golden/result_01_basic.dingo`
2. Verify: Output compiles and looks idiomatic
3. Copy: `result_01_basic.go` → `result_01_basic.go.golden`
4. Repeat for all 5 Result tests

---

## Part 3: Critical Dependencies & Assumptions

### Dependencies

1. **Enum Preprocessor** (Phase 1):
   - ✅ No external dependencies
   - ✅ Uses standard `regexp` and `bytes` packages
   - ✅ Follows existing preprocessor patterns

2. **Plugin Pipeline** (Phase 2):
   - ✅ Result type plugin already complete
   - ✅ Type inference service exists (lines 56-63 in `generator.go`)
   - ⚠️ Requires enum preprocessor (from Phase 1)

3. **Type Information**:
   - ⚠️ Current: Uses structural heuristics (lines 263-358 in `result_type.go`)
   - ✅ Sufficient for basic types (int, string, error)
   - ⚠️ Limited for complex types (requires full `go/types` integration)
   - 📌 **Defer full type checking to Phase 3**

### Assumptions

1. **Syntax Design**:
   - Enum syntax: `enum Name { Variant(Type) }`
   - No generic enums yet: `enum<T>` deferred to Phase 3
   - Constructor calls: `Ok(value)`, `Err(error)` (implicit, no prefix)

2. **Generated Code Style**:
   - Tagged unions with pointer fields (memory efficient)
   - Constructor functions: `Result_T_E_Ok()`, `Result_T_E_Err()`
   - Helper methods on receiver: `(r Result_T_E) IsOk()`

3. **Compilation Model**:
   - No runtime library (zero-cost abstraction)
   - Generated code is pure Go (no special imports)
   - Full interop with existing Go code

---

## Part 4: Testing Strategy

### Unit Tests

**Enum Preprocessor** (`pkg/preprocessor/enum_test.go`):
- Simple enum parsing
- Variant with types
- Multiple variants
- Edge cases (empty, single variant, nested types)
- Source mapping accuracy

**Plugin Pipeline** (`pkg/plugin/plugin_test.go`):
- Plugin registration
- Discovery phase (finding Result types)
- Transformation phase (replacing constructors)
- Declaration injection (adding type definitions)
- Multi-plugin execution order

**Result Type Plugin** (`pkg/plugin/builtin/result_type_test.go`):
- Constructor detection (`Ok()`, `Err()`)
- Type inference (basic types, complex types)
- AST transformation correctness
- Declaration generation

### Integration Tests

**End-to-End** (`tests/integration/result_e2e_test.go`):
- Full pipeline: `.dingo` → preprocessor → parser → generator → `.go`
- Compilation check: Generated code passes `go build`
- Behavior check: Generated code runs correctly
- Interop check: Can call from pure Go

### Golden Tests

**Existing Tests** (`tests/golden/result_*.dingo`):
- `result_01_basic.dingo` - Simple Result type with Ok/Err
- `result_02_propagation.dingo` - Error propagation with `?`
- `result_03_pattern_match.dingo` - Pattern matching (Phase 3)
- `result_04_chaining.dingo` - Method chaining (IsOk, Unwrap)
- `result_05_go_interop.dingo` - Calling Go functions

**Test Execution**:
1. Run: `dingo build tests/golden/result_*.dingo`
2. Compare: Generated `.go` vs `.go.golden` (exact match)
3. Compile: `go build result_*.go` (must succeed)
4. Execute: `go run result_*.go` (behavioral correctness)

---

## Part 5: Success Criteria

### Phase 1 Success Criteria

- ✅ Enum preprocessor transforms `enum` → Go sum type
- ✅ Generated code passes `go/parser` (valid Go syntax)
- ✅ Source mappings are accurate (for LSP)
- ✅ All enum unit tests pass
- ✅ Golden tests parse successfully (no "expected declaration, found enum" error)

### Phase 2 Success Criteria

- ✅ Plugin pipeline executes Result type plugin
- ✅ `Ok(value)` transforms to `CompositeLit` (Fix A2 working)
- ✅ Type inference determines correct Result type (Fix A3 working)
- ✅ Result type declarations injected at package level
- ✅ Generated code compiles with `go build`
- ✅ Golden tests pass (output matches `.go.golden`)
- ✅ End-to-end test: Write `.dingo`, transpile, compile, run

### Overall Success Criteria

- ✅ All 46 golden tests parse without errors
- ✅ Result type tests (5 files) compile and run
- ✅ Generated code is idiomatic Go (readable, maintainable)
- ✅ Zero runtime overhead (no runtime library dependencies)
- ✅ Full interop with Go (can call Go functions, return Results)

---

## Part 6: Risk Assessment

### High Risk

1. **Enum Syntax Ambiguity**:
   - **Risk**: Regex-based parsing may misidentify enum blocks
   - **Mitigation**: Conservative pattern matching, use AST when available
   - **Fallback**: Add preprocessing escape hatch (e.g., `//dingo:skip-enum`)

2. **Type Inference Limitations**:
   - **Risk**: Cannot infer complex types without full `go/types`
   - **Impact**: `Err()` calls require explicit type annotations
   - **Mitigation**: Document limitation, provide workarounds (explicit constructors)
   - **Timeline**: Full type checking in Phase 3

3. **Source Mapping Accuracy**:
   - **Risk**: Enum transformation may break line mappings
   - **Impact**: LSP features (go-to-definition) may be inaccurate
   - **Mitigation**: Test source maps extensively, adjust algorithm as needed

### Medium Risk

1. **Plugin Execution Order**:
   - **Risk**: Plugin interdependencies (e.g., Result depends on enum)
   - **Mitigation**: Document execution phases, enforce ordering in pipeline

2. **Generated Code Quality**:
   - **Risk**: Output may not be idiomatic Go (linter errors)
   - **Mitigation**: Run `gofmt`, `golangci-lint` on generated code, adjust templates

3. **Performance**:
   - **Risk**: Regex-based preprocessing may be slow on large files
   - **Mitigation**: Profile preprocessor, optimize patterns, consider caching

### Low Risk

1. **Backward Compatibility**:
   - **Risk**: Changes may break existing tests
   - **Mitigation**: No backward compatibility needed (pre-release)

2. **Edge Cases**:
   - **Risk**: Unusual enum syntax may not be handled
   - **Mitigation**: Document supported syntax, add tests for edge cases

---

## Part 7: Timeline Estimate

### Phase 1: Fix Parser/Preprocessor (4-6 hours)
- Task 1.1: Create Enum Preprocessor (2-3 hours)
- Task 1.2: Add Preprocessor Configuration (1 hour)
- Task 1.3: Test Enum Preprocessor (1-2 hours)

### Phase 2: Complete Result[T,E] Integration (6-8 hours)
- Task 2.1: Implement Plugin Pipeline (3-4 hours)
- Task 2.2: Update Result Type Plugin Integration (1 hour)
- Task 2.3: Update Generator to Use Pipeline (1 hour)
- Task 2.4: Add Registry.Register() Method (30 minutes)
- Task 2.5: End-to-End Testing (1-2 hours)
- Task 2.6: Update Golden Tests (1 hour)

### Total: 10-14 hours

**Critical Path**: Enum preprocessor → Plugin pipeline → E2E testing

---

## Part 8: Alternative Approaches Considered

### Alternative 1: AST-Based Enum Transformation

**Approach**: Parse `.dingo` with extended grammar, transform at AST level.

**Pros**:
- More precise than regex
- Easier to handle complex nested types
- Better error messages

**Cons**:
- Requires custom parser (not `go/parser`)
- More complex implementation
- Breaks existing architecture (all features use preprocessing)

**Decision**: ❌ Rejected - Inconsistent with existing architecture.

### Alternative 2: Runtime Library for Result Type

**Approach**: Ship `dingo/runtime` package with generic `Result[T,E]`.

**Pros**:
- Simpler code generation (just use library types)
- Consistent behavior across projects
- Easier to maintain

**Cons**:
- Adds dependency (violates zero-runtime goal)
- Cannot customize representation per-project
- Requires Go 1.18+ generics
- Breaks interop with pure Go

**Decision**: ❌ Rejected - Violates core design principle (zero runtime overhead).

### Alternative 3: Macro System (Like Rust)

**Approach**: Implement macro expansion system for enums.

**Pros**:
- More flexible (users can define custom macros)
- Powerful metaprogramming
- Future-proof for other features

**Cons**:
- Extremely complex (months of work)
- Overkill for current needs
- Hard to debug for users
- Not idiomatic for Go

**Decision**: ❌ Rejected - Scope creep, too complex for Phase 1.

---

## Part 9: Future Enhancements (Phase 3+)

### Type Checking Integration

**Goal**: Full `go/types` integration for accurate type inference.

**Benefits**:
- Infer types for `Err()` calls without context
- Validate Result type usage at compile time
- Better error messages

**Effort**: 2-3 weeks

### Generic Enum Support

**Syntax**: `enum Option[T] { Some(T), None }`

**Benefits**:
- Reusable enum definitions
- Type-safe generic programming

**Challenges**:
- Requires Go 1.18+ generic syntax
- Complex type parameter handling

**Effort**: 1-2 weeks

### Pattern Matching

**Syntax**:
```dingo
match result {
    Ok(v) => println(v),
    Err(e) => println(e),
}
```

**Benefits**:
- Ergonomic enum consumption
- Exhaustiveness checking

**Effort**: 2-3 weeks

### LSP Integration

**Goal**: Source map-aware gopls proxy.

**Benefits**:
- Accurate go-to-definition
- Inline error messages
- Full IDE support

**Effort**: 4-6 weeks

---

## Part 10: Implementation Order

### Recommended Sequence

1. ✅ **Enum Preprocessor** (Task 1.1-1.3) - BLOCKER
2. ✅ **Plugin Pipeline** (Task 2.1, 2.4) - Core infrastructure
3. ✅ **Result Plugin Integration** (Task 2.2, 2.3) - Activate existing code
4. ✅ **End-to-End Testing** (Task 2.5) - Validate integration
5. ✅ **Golden Test Updates** (Task 2.6) - Documentation and validation

### Milestone Checkpoints

**Checkpoint 1** (After Task 1.3):
- ✅ Enum tests parse without errors
- ✅ `dingo build result_01_basic.dingo` succeeds

**Checkpoint 2** (After Task 2.4):
- ✅ Plugin pipeline executes
- ✅ `Ok()` calls transform to `CompositeLit`

**Checkpoint 3** (After Task 2.5):
- ✅ Generated code compiles
- ✅ E2E test passes

**Final Checkpoint** (After Task 2.6):
- ✅ All golden tests pass
- ✅ Ready for Phase 3

---

## Part 11: Code Quality Standards

### Generated Code Quality

**Requirements**:
1. ✅ Passes `gofmt` (correct formatting)
2. ✅ Passes `go build` (compiles without errors)
3. ✅ Passes `golangci-lint` (no linter warnings)
4. ✅ Readable by humans (idiomatic Go)
5. ✅ No external dependencies (stdlib only)

**Example**:
```go
// Good: Idiomatic generated code
type Result_int_error struct {
    tag    ResultTag
    ok_0   *int
    err_0  *error
}

func (r Result_int_error) IsOk() bool {
    return r.tag == ResultTag_Ok
}

// Bad: Non-idiomatic generated code
type Result_int_error struct{tag ResultTag;ok_0 *int;err_0 *error}
func(r Result_int_error)IsOk()bool{return r.tag==ResultTag_Ok}
```

### Source Code Quality

**Requirements**:
1. ✅ Go idioms (error handling, naming, composition)
2. ✅ Unit tests for all public functions
3. ✅ Documentation comments (godoc)
4. ✅ Error messages include context
5. ✅ Logging for debugging (via `plugin.Logger`)

**Example**:
```go
// Good: Clear error with context
func (e *EnumProcessor) Process(source []byte) ([]byte, []Mapping, error) {
    enums, err := e.findEnums(source)
    if err != nil {
        return nil, nil, fmt.Errorf("failed to find enum declarations: %w", err)
    }
    // ...
}

// Bad: Generic error
func (e *EnumProcessor) Process(source []byte) ([]byte, []Mapping, error) {
    enums, err := e.findEnums(source)
    if err != nil {
        return nil, nil, err
    }
    // ...
}
```

---

## Part 12: Documentation Requirements

### Code Documentation

**Required**:
1. Package-level doc comment explaining purpose
2. Public function doc comments (what, not how)
3. Complex algorithms explained with inline comments
4. Examples in doc comments (for godoc)

### User Documentation

**Update These Files**:
1. `README.md` - Add Result type example
2. `tests/golden/README.md` - Update test catalog
3. `CHANGELOG.md` - Document new features

**Create New Files**:
1. `docs/result-type.md` - Comprehensive Result type guide
2. `docs/enum-syntax.md` - Enum syntax reference

### Developer Documentation

**Update These Files**:
1. `ai-docs/wip/GO_IMPLEMENTATION.md` - Add enum preprocessor section
2. `CLAUDE.md` - Update current phase to "Phase 3: Result/Option Integration"

---

## Part 13: Rollback Plan

If critical issues arise:

### Rollback Triggers

1. **Generated code doesn't compile** (after 2 hours of debugging)
2. **Source mappings are broken** (LSP completely broken)
3. **Performance regression** (>2x slower compilation)
4. **Incompatible with existing tests** (>50% golden tests fail)

### Rollback Steps

1. ✅ Revert commits: `git revert <commit-range>`
2. ✅ Disable enum preprocessor in `preprocessor.New()`
3. ✅ Disable Result plugin in pipeline
4. ✅ Verify core tests still pass
5. ✅ Document issues in `ai-docs/sessions/*/rollback.md`

### Recovery Strategy

1. **Isolate Issue**: Use feature flags to narrow down problem
2. **Simplify Scope**: Remove advanced features, keep basic working
3. **Alternative Approach**: Consider AST-based transformation if preprocessing fails
4. **Timeline**: Maximum 4 hours debugging before rollback

---

## Conclusion

**Recommended Approach**: Two-phase implementation (Enum preprocessor → Plugin pipeline).

**Key Insight**: Parser is already working correctly. The real blocker is missing enum support.

**Critical Success Factor**: Enum preprocessor must be robust, as it's a dependency for all sum type features.

**Timeline**: 10-14 hours total, with 4-6 hours for the critical path (enum preprocessor).

**Next Steps**:
1. Review this plan with user
2. Clarify any questions (see `gaps.json`)
3. Begin implementation with Task 1.1 (Enum Preprocessor)
