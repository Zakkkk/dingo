# Phase 3 - Result/Option Integration: Architectural Plan

## Executive Summary

**Current State**: Phase 2.7 Complete (Functional Utilities)
- Infrastructure exists for Result/Option types (plugins created, type inference system in place)
- Golden test files exist but are skipped (marked as "Feature not yet implemented")
- Plugins transform `Ok()` and `Some()` calls but don't handle the full `enum Result/Option` syntax

**Next Milestone**: Phase 3 - Result/Option Integration
- Connect existing infrastructure to make Result/Option types fully functional
- Transform `enum Result { Ok(T), Err(E) }` declarations into sum types
- Enable golden tests for result_* and option_* files
- Integrate with existing error propagation (`?` operator)

**Scope**: This is NOT about creating new infrastructure. It's about connecting existing pieces.

---

## Problem Summary

The project has a chicken-and-egg problem:

1. **Result/Option plugins exist** but don't transform enum declarations
2. **Sum types plugin exists** and transforms enum declarations into Go structs
3. **Golden tests exist** expecting `enum Result { Ok(T), Err(E) }` syntax
4. **Current plugins** only handle constructor calls (`Ok()`, `Err()`, `Some()`), not declarations

**The Gap**: Result/Option need special handling because they're:
- **Built-in types** (not user-defined enums)
- **Generic** (Result[T, E], Option[T])
- **Used by other features** (error propagation, pattern matching)
- **Expected to work with enum syntax** in Dingo source files

---

## Recommended Approach

### Strategy: Alias Result/Option as Special Sum Types

**Core Insight**: Result and Option ARE sum types. Don't duplicate transformation logic.

**Architecture**:
1. **Parser Enhancement**: Recognize `enum Result` and `enum Option` as special cases
2. **Sum Types Plugin**: Handle Result/Option as it already handles user enums
3. **Result/Option Plugins**: Focus ONLY on helper method generation (IsOk, IsErr, Unwrap, etc.)
4. **Type Inference**: Already integrated, just needs Result/Option enum registration

**Benefits**:
- Reuses battle-tested sum types transformation (926 lines, well-tested)
- No duplicate AST generation logic
- Consistent behavior between user enums and built-in types
- Minimal code changes (<200 lines total)

**Trade-offs**:
- Result/Option lose special status (become "blessed enums")
- Type names become `Result_T_E` instead of special handling
- Acceptable because: Zero runtime overhead, idiomatic Go output

---

## Detailed Architecture

### 1. Parser Enhancement (Minimal Changes)

**Current**: Parser treats `enum Result { Ok(T), Err(E) }` as regular enum

**Needed**: Tag enum declarations as Result/Option types

```go
// pkg/ast/ast.go
type EnumDeclaration struct {
    // ... existing fields ...
    IsResultType bool  // Set to true for "enum Result"
    IsOptionType bool  // Set to true for "enum Option"
}
```

**Parser Change** (`pkg/parser/participle.go`):
```go
// After parsing enum declaration:
if enumDecl.Name.Name == "Result" {
    enumDecl.IsResultType = true
}
if enumDecl.Name.Name == "Option" {
    enumDecl.IsOptionType = true
}
```

**Validation**: Enforce Result/Option structure
- Result must have exactly 2 variants: Ok(T), Err(E)
- Option must have exactly 2 variants: Some(T), None
- Emit clear error if structure is wrong

### 2. Sum Types Plugin Enhancement

**Current**: Transforms all enums uniformly

**Needed**: Special handling for Result/Option

**Changes** (`pkg/plugin/builtin/sum_types.go`):

```go
func (p *SumTypesPlugin) transformEnumDeclaration(enumDecl *dingoast.EnumDeclaration) []ast.Decl {
    // Existing enum transformation logic...

    // NEW: Check if this is Result or Option
    if enumDecl.IsResultType || enumDecl.IsOptionType {
        // Register with type inference system
        if typeInf, ok := p.Ctx.GetTypeInference(); ok {
            typeInf.RegisterSyntheticType(typeName, "enum")
        }

        // Generate standard sum type (same as user enums)
        // But mark it for Result/Option plugin to add helpers
        p.markForHelperGeneration(enumDecl)
    }

    // Return standard sum type declarations
    return decls
}
```

**Key Point**: Sum types plugin does the heavy lifting (struct generation, tag enum, constructors). Result/Option plugins add helper methods.

### 3. Result Type Plugin Refactor

**Current**: Attempts full transformation (508 lines, complex)

**New Strategy**: Helper method injection only

**Responsibilities**:
1. Detect Result enum declarations (via `IsResultType` flag)
2. Add helper methods: `Unwrap()`, `UnwrapOr(default)`, `Map(fn)`, `AndThen(fn)`
3. Register Result type with pattern matching system

**Simplified Architecture**:
```go
type ResultTypePlugin struct {
    plugin.BasePlugin
    // Track which Result types need helpers injected
    resultTypes map[string]*EnumInfo  // "Result_int_error" -> EnumInfo
}

func (p *ResultTypePlugin) Transform(ctx *plugin.Context, node ast.Node) (ast.Node, error) {
    // Phase 1: Collect Result enum declarations (walk AST)
    // Phase 2: For each Result type, generate helper methods
    // Phase 3: Inject helpers after type declaration
}
```

**Helper Method Example**:
```go
// For: enum Result { Ok(int), Err(error) }
// Generate:

// Unwrap returns the Ok value or panics if Err
func (r Result_int_error) Unwrap() int {
    if r.tag != ResultTag_Ok {
        panic("called Unwrap on Err value")
    }
    return *r.ok_0
}

// UnwrapOr returns the Ok value or the provided default
func (r Result_int_error) UnwrapOr(defaultVal int) int {
    if r.tag == ResultTag_Ok {
        return *r.ok_0
    }
    return defaultVal
}
```

**Reduced Scope**: ~150 lines (down from 508)

### 4. Option Type Plugin Refactor

**Current**: Similar to Result plugin (455 lines)

**New Strategy**: Same as Result - helpers only

**Responsibilities**:
1. Detect Option enum declarations (via `IsOptionType` flag)
2. Add helper methods: `Unwrap()`, `UnwrapOr(default)`, `Map(fn)`, `Filter(fn)`, `IsSomeAnd(predicate)`
3. Handle `None` special case (zero-argument constructor already handled by sum types)

**Simplified Architecture**: Mirror Result plugin pattern

**Reduced Scope**: ~130 lines (down from 455)

### 5. Type Inference Integration

**Current**: Type inference system exists (Phase 2.8)

**Needed**: Register Result/Option as synthetic types

**Already Implemented** (Phase 2.9):
```go
// In Result plugin (line 87 of current result_type.go)
if typeInf, ok := p.Ctx.GetTypeInference(); ok {
    typeInf.RegisterSyntheticType(typeName, "result")
}
```

**Action**: Verify this integration works end-to-end

### 6. Constructor Transformation

**Current**: Result/Option plugins transform `Ok()`, `Err()`, `Some()` calls

**Needed**: Remove this logic (sum types plugin already does it)

**Change**: Delete `transformOkLiteral`, `transformErrLiteral`, `transformSomeLiteral` methods from Result/Option plugins. Sum types plugin generates constructors for ALL enum variants.

**Simplification**: ~300 lines removed across both plugins

---

## Package Structure

```
pkg/
├── ast/
│   └── ast.go                    # Add IsResultType, IsOptionType fields
├── parser/
│   └── participle.go             # Tag Result/Option enums during parsing
└── plugin/
    └── builtin/
        ├── sum_types.go          # Enhanced: Register Result/Option with type inference
        ├── result_type.go        # Refactored: Helper methods only (~150 lines)
        ├── option_type.go        # Refactored: Helper methods only (~130 lines)
        └── type_inference.go     # Unchanged: Already supports synthetic types
```

**Total Changes**: ~400 lines modified, ~500 lines removed, ~200 lines added

---

## Key Interfaces/Types

### 1. EnumDeclaration Enhancement

```go
// pkg/ast/ast.go
type EnumDeclaration struct {
    Name      *ast.Ident
    Variants  []*EnumVariant

    // NEW: Special type markers
    IsResultType bool  // True for "enum Result"
    IsOptionType bool  // True for "enum Option"
}
```

### 2. Helper Method Generator Interface

```go
// pkg/plugin/builtin/result_type.go
type HelperMethodGenerator interface {
    // Generate helper methods for a specific Result instantiation
    GenerateHelpers(typeName string, okType, errType ast.Expr) []ast.Decl
}

// Methods to generate:
// - Unwrap() T
// - UnwrapOr(T) T
// - UnwrapErr() E
// - Map(fn func(T) U) Result[U, E]
// - MapErr(fn func(E) F) Result[T, F]
// - AndThen(fn func(T) Result[U, E]) Result[U, E]
// - IsOk() bool (already exists from sum types)
// - IsErr() bool (already exists from sum types)
```

### 3. Type Registration

```go
// pkg/plugin/builtin/type_inference.go (already exists)
type TypeInferenceService struct {
    syntheticTypes map[string]string  // "Result_int_error" -> "result"
}

func (s *TypeInferenceService) RegisterSyntheticType(name, kind string)
func (s *TypeInferenceService) IsResultType(name string) bool
func (s *TypeInferenceService) IsOptionType(name string) bool
```

---

## Dependency Map

```
┌─────────────────────────────────────────────┐
│         Parser (participle.go)              │
│  Tags: IsResultType, IsOptionType           │
└────────────────┬────────────────────────────┘
                 │ Emits: EnumDeclaration
                 │
                 ▼
┌─────────────────────────────────────────────┐
│      Sum Types Plugin (sum_types.go)        │
│  • Transforms ALL enums to Go structs       │
│  • Detects Result/Option via flags          │
│  • Registers with TypeInferenceService      │
│  • Generates: struct, tag enum, constrs     │
└────────────────┬────────────────────────────┘
                 │ Generates: ast.Decl nodes
                 │ Registers: Synthetic types
                 │
        ┌────────┴──────────┐
        ▼                   ▼
┌───────────────┐  ┌────────────────┐
│ Result Plugin │  │ Option Plugin  │
│ (150 lines)   │  │ (130 lines)    │
│               │  │                │
│ Adds helpers: │  │ Adds helpers:  │
│ • Unwrap()    │  │ • Unwrap()     │
│ • UnwrapOr()  │  │ • UnwrapOr()   │
│ • Map()       │  │ • Map()        │
│ • AndThen()   │  │ • Filter()     │
└───────────────┘  └────────────────┘
```

**External Dependencies**:
- `go/ast`: Standard library (AST nodes)
- `go/token`: Standard library (positions)
- `golang.org/x/tools/go/ast/astutil`: AST manipulation (already used)

**Internal Dependencies**:
- Result/Option plugins → Sum Types plugin (must run after)
- Sum Types plugin → Type Inference Service (registration)
- Error Propagation → Result plugin (uses Result types)

**Plugin Order**:
1. Sum Types Plugin (transforms enums, registers types)
2. Result/Option Plugins (add helper methods)
3. Error Propagation Plugin (uses Result types)

---

## Implementation Notes

### Critical Details

1. **Type Name Generation**:
   - Use existing `sanitizeTypeName()` from `type_utils.go`
   - Result[int, error] → `Result_int_error`
   - Option[*User] → `Option_ptr_User`

2. **Generic Type Handling**:
   - Dingo syntax: `enum Result { Ok(T), Err(E) }`
   - Parser sees: `T` and `E` as type parameters
   - Sum types plugin: Generates generic struct with type params
   - **CRITICAL**: Need to handle type parameter substitution

3. **Constructor Naming**:
   - Sum types plugin generates: `Result_Ok(arg0 T)`, `Result_Err(arg0 E)`
   - Golden tests expect: `Result_Ok()`, `Result_Err()`
   - **Already works**: Sum types generates correct names

4. **Field Access**:
   - Sum types generates: `ok_0`, `err_0`, `some_0` (lowercase variant + `_0`)
   - Helper methods must use: `*r.ok_0` (pointer dereference)
   - **Already works**: Pattern established in Phase 2.5

5. **Nil Safety**:
   - Unwrap() must check tag before dereferencing
   - UnwrapOr() provides safe fallback
   - Configuration: Respect `nil_safety_checks` from dingo.toml

6. **Position Tracking**:
   - All generated nodes need valid `Pos()` and `End()`
   - Use source enum declaration position as base
   - Critical for error messages and source maps

### Gotchas

1. **Plugin Ordering**: Result/Option MUST run AFTER Sum Types
   - Sum Types creates the type declaration
   - Result/Option add methods to existing type
   - Violating this = panic or wrong output

2. **Type Parameter Inference**:
   - `Ok(42)` → Need to infer T=int
   - Type inference service required
   - Already implemented (Phase 2.8)

3. **None Constructor**:
   - `None` has no arguments (unlike `Some(value)`)
   - Sum types generates: `Option_None() Option_T`
   - **Question**: How does `None` know what T is? Need type context.

4. **Duplicate Type Declarations**:
   - Multiple `Result[int, error]` usages → Only emit type once
   - Use `emittedTypes map[string]bool` (already in current plugins)

5. **Error Handling**:
   - Invalid Result structure (e.g., 3 variants) → Clear error
   - Type mismatch (e.g., Ok(string), Err(int)) → Type checker catches it
   - Graceful degradation if type inference unavailable

### Best Practices

1. **Reuse Existing Code**:
   - Sum types plugin: 926 lines, tested, works
   - Type inference: 313 lines, tested, cached
   - Type utils: sanitizeTypeName, typeToString
   - **Don't reinvent**: Build on what works

2. **Minimal AST Generation**:
   - Only generate what's necessary (helper methods)
   - Reuse Go's go/ast node types
   - Keep position info accurate

3. **Comprehensive Testing**:
   - Unit tests: Each helper method (Map, Unwrap, etc.)
   - Golden tests: result_01_basic.dingo → result_01_basic.go.golden
   - Integration tests: Result + error propagation (`?`)

4. **Clear Error Messages**:
   - "enum Result must have exactly 2 variants: Ok(T) and Err(E)"
   - "Invalid Option declaration: expected Some(T) and None variants"
   - Include source position in all errors

5. **Documentation**:
   - Update CHANGELOG.md (not progress files)
   - Add reasoning docs: `result_01_basic.reasoning.md`
   - Update feature status in README.md

---

## Testing Strategy

### Phase 1: Unit Tests

**Result Plugin Tests** (`pkg/plugin/builtin/result_type_test.go`):
- ✅ Already exists (10 tests, 17 test cases)
- **Refactor needed**: Update for new helper-only approach
- New tests:
  - `TestUnwrapMethod` - Unwrap() happy path and panic
  - `TestUnwrapOrMethod` - UnwrapOr() with Ok and Err
  - `TestMapMethod` - Map() transformation
  - `TestAndThenMethod` - AndThen() chaining

**Option Plugin Tests** (`pkg/plugin/builtin/option_type_test.go`):
- ✅ Already exists (9 tests, 16 test cases)
- **Refactor needed**: Update for new helper-only approach
- New tests:
  - `TestIsSomeAndMethod` - IsSomeAnd() predicate
  - `TestFilterMethod` - Filter() transformation

**Sum Types Tests** (`pkg/plugin/builtin/sum_types_test.go`):
- ✅ Already exists (52 tests, 100% passing)
- **New tests**:
  - `TestResultEnumTransformation` - Verify Result gets tagged
  - `TestOptionEnumTransformation` - Verify Option gets tagged
  - `TestTypeRegistration` - Verify synthetic type registration

### Phase 2: Golden File Tests

**Enable Result Tests**:
- `result_01_basic.dingo` → `result_01_basic.go.golden`
- `result_02_propagation.dingo` → Integration with `?` operator
- `result_03_pattern_match.dingo` → Integration with match expressions
- `result_04_chaining.dingo` → Map/AndThen chaining
- `result_05_go_interop.dingo` → Wrapping Go (T, error) functions

**Enable Option Tests**:
- `option_01_basic.dingo` → `option_01_basic.go.golden`
- `option_02_pattern_match.dingo` → Integration with match expressions
- `option_03_chaining.dingo` → Map/Filter chaining
- `option_04_go_interop.dingo` → Wrapping Go pointer returns

**Success Criteria**: All result_* and option_* golden tests passing

### Phase 3: Integration Tests

**Error Propagation + Result**:
```dingo
// Test: error_prop_09_result_type.dingo
func divide(a: int, b: int) Result {
    if b == 0 {
        return Err(errors.New("division by zero"))
    }
    return Ok(a / b)
}

func calculate() (int, error) {
    result := divide(10, 2)?  // Unwraps Result to (int, error)
    return result, nil
}
```

**Pattern Matching + Option**:
```dingo
// Test: pattern_match_05_option.dingo
func getUserName(id: int) Option {
    // ...
}

func greet(id: int) {
    match getUserName(id) {
        Some(name) => println("Hello", name),
        None => println("User not found"),
    }
}
```

**Success Criteria**:
- All integration tests compile
- All integration tests produce expected output
- No panics or crashes

---

## Alternatives Considered

### Alternative 1: Keep Result/Option as Separate Types

**Approach**: Don't use enum syntax, treat Result/Option as first-class types

**Pros**:
- More control over type generation
- Can optimize Result/Option specifically
- Clearer separation of concerns

**Cons**:
- Duplicate transformation logic (500+ lines)
- Two ways to define sum types (confusing)
- More code to maintain
- Golden tests expect `enum Result` syntax

**Rejected**: Too much duplication, violates DRY principle

### Alternative 2: Result/Option as Language Keywords

**Approach**: Make Result and Option keywords, not enums

**Pros**:
- Most explicit
- Could enable special optimizations
- Clear that these are built-in types

**Cons**:
- Requires parser changes (keywords)
- Violates "library over language" principle
- Makes language larger, not smaller
- Harder to extend with other monads (Either, Maybe, etc.)

**Rejected**: Goes against Dingo philosophy of minimal syntax

### Alternative 3: Generic Enum Base Type

**Approach**: `enum Result[T, E] { Ok(T), Err(E) }` as generic definition

**Pros**:
- Most Rust-like syntax
- Clear type parameters
- Reusable across files

**Cons**:
- Requires generic type system in parser
- Complex type parameter substitution
- Not needed for phase 3 (premature)
- Can add later if needed

**Deferred**: Good idea, but too complex for Phase 3. Add in Phase 4+.

---

## Quality Checks

Before finalizing implementation, verify:

- [x] Does this solve the actual problem? **YES**: Connects infrastructure to enable Result/Option
- [x] Are we reusing existing Go packages? **YES**: go/ast, astutil, existing plugins
- [x] Can this be broken into simpler pieces? **YES**: 3 independent changes (parser, sum types, helpers)
- [x] Is each component independently testable? **YES**: Unit tests per plugin, golden tests end-to-end
- [x] Will this code be maintainable in 2 years? **YES**: Reuses sum types, clear separation
- [x] Are interfaces minimal and focused? **YES**: HelperMethodGenerator single responsibility
- [x] Does this follow Go idioms? **YES**: Generates idiomatic Go code

---

## Implementation Phases

### Phase 3.1: Parser Enhancement (2-3 hours)

**Deliverables**:
1. Add `IsResultType`, `IsOptionType` to `EnumDeclaration`
2. Tag enums in parser
3. Validation: Enforce Result/Option structure
4. Unit tests for parser validation

**Success**: Parser recognizes Result/Option and tags them correctly

### Phase 3.2: Sum Types Integration (3-4 hours)

**Deliverables**:
1. Detect Result/Option in sum types plugin
2. Register with type inference service
3. Mark for helper generation (metadata)
4. Unit tests for type registration

**Success**: Sum types transforms Result/Option and registers them

### Phase 3.3: Result Helper Methods (4-5 hours)

**Deliverables**:
1. Refactor Result plugin to helper-only mode
2. Implement: Unwrap, UnwrapOr, Map, MapErr, AndThen
3. Unit tests for each helper method
4. Enable result_01_basic golden test

**Success**: result_01_basic.dingo transpiles and compiles correctly

### Phase 3.4: Option Helper Methods (3-4 hours)

**Deliverables**:
1. Refactor Option plugin to helper-only mode
2. Implement: Unwrap, UnwrapOr, Map, Filter, IsSomeAnd
3. Handle None special case
4. Unit tests for each helper method
5. Enable option_01_basic golden test

**Success**: option_01_basic.dingo transpiles and compiles correctly

### Phase 3.5: Integration & Testing (4-6 hours)

**Deliverables**:
1. Enable all result_* golden tests
2. Enable all option_* golden tests
3. Integration tests (Result + error propagation, Option + pattern matching)
4. Fix any discovered bugs
5. Update CHANGELOG.md
6. Update README.md feature status

**Success**: All result_* and option_* golden tests passing (100%)

### Phase 3.6: Documentation (2-3 hours)

**Deliverables**:
1. Write `result_01_basic.reasoning.md`
2. Write `option_01_basic.reasoning.md`
3. Update golden test README with Result/Option sections
4. Add code samples to main README

**Success**: Complete documentation for Result/Option features

---

## Total Effort Estimate

**Implementation**: 16-22 hours
**Testing**: 4-6 hours (included in phases)
**Documentation**: 2-3 hours
**Buffer**: 3-5 hours (bug fixes, edge cases)

**Total**: 22-33 hours (~3-4 days of focused work)

---

## Success Metrics

**Quantitative**:
- [ ] 9 golden tests passing (5 result_*, 4 option_*)
- [ ] 20+ unit tests passing (helper methods)
- [ ] <500 lines of code added
- [ ] >500 lines of code removed (refactoring)
- [ ] 100% test coverage on helper methods
- [ ] 0 panics or crashes

**Qualitative**:
- [ ] Generated Go code is idiomatic
- [ ] Error messages are clear and helpful
- [ ] Code is maintainable (reuses existing infrastructure)
- [ ] Documentation is comprehensive
- [ ] Feature works end-to-end (write Dingo → compile → run)

---

## Future Enhancements

**Phase 4+** (Deferred, not in this plan):

1. **Generic Result/Option Declarations**:
   ```dingo
   enum Result[T, E] { Ok(T), Err(E) }
   enum Option[T] { Some(T), None }
   ```
   - Requires generic type system
   - Type parameter substitution
   - 10-15 hours of work

2. **Auto-wrapping Go Functions**:
   ```dingo
   // Go: func ReadFile(path string) ([]byte, error)
   // Dingo: Automatically wraps to Result[[]byte, error]
   content := ReadFile("file.txt")?  // No manual wrapping
   ```
   - Requires go/types integration
   - Function call interception
   - 15-20 hours of work

3. **Try Blocks** (Rust-style):
   ```dingo
   result := try {
       a := operation1()?;
       b := operation2(a)?;
       b * 2
   }  // Returns Result[int, error]
   ```
   - Syntactic sugar for error handling
   - 8-10 hours of work

4. **Railway-Oriented Programming Helpers**:
   - `Tap(fn)`: Side effects without transformation
   - `Recover(fn)`: Convert Err to Ok with recovery function
   - `Transpose()`: Result[Option[T], E] → Option[Result[T, E]]
   - 6-8 hours of work

**Note**: These are outside Phase 3 scope. Focus on foundational integration first.

---

## Risk Assessment

### High Risk

1. **Generic Type Parameter Handling**:
   - **Risk**: `enum Result { Ok(T), Err(E) }` - How to handle T and E?
   - **Mitigation**: Use type inference for constructor calls, require type context for instantiation
   - **Fallback**: Start with non-generic (e.g., `Result_int_error` hardcoded in test)

2. **None Constructor Type Inference**:
   - **Risk**: `None` has no value - how to infer Option[T]?
   - **Mitigation**: Require assignment context: `let x: Option = None`
   - **Fallback**: Require explicit type: `Option_None::<string>()`

### Medium Risk

1. **Plugin Ordering Dependencies**:
   - **Risk**: Wrong order causes crashes or incorrect output
   - **Mitigation**: Explicit dependencies in plugin registration, documented order
   - **Testing**: Verify order in unit tests

2. **Helper Method Name Collisions**:
   - **Risk**: User defines `Unwrap()` method on Result type
   - **Mitigation**: Document that IsOk, IsErr, Unwrap, etc. are reserved
   - **Future**: Namespace helpers under `result::` module

### Low Risk

1. **Golden Test Mismatches**:
   - **Risk**: Generated code doesn't match .go.golden files
   - **Mitigation**: Regenerate golden files if needed, document changes

2. **Performance Regression**:
   - **Risk**: Helper methods add overhead
   - **Mitigation**: Inline methods, zero-cost abstractions
   - **Measurement**: Benchmark before/after

---

## Conclusion

Phase 3 is about **integration, not invention**. All the pieces exist:
- Sum types plugin: Transforms enums → Go structs ✅
- Type inference: Detects types, caches results ✅
- Error propagation: Uses `?` operator ✅
- Golden tests: Define expected behavior ✅

**This plan connects the dots.**

By treating Result/Option as "blessed enums" and reusing sum types infrastructure, we avoid duplicating 500+ lines of code and leverage battle-tested transformation logic. The only new code is helper method generation (~300 lines total) and parser tagging (~50 lines).

**Expected outcome**:
- 9 golden tests enabled and passing
- Result/Option types fully functional
- Foundation for Pattern Matching (Phase 4)
- Clean, maintainable codebase

**Timeline**: 3-4 days of focused implementation

**Confidence**: High (reusing proven components, clear architecture, comprehensive test coverage)
