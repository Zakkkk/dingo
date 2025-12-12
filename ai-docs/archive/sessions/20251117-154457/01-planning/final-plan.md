# Phase 3 - Result/Option Integration: Final Implementation Plan

## Executive Summary

**Current State**: Phase 2.7 Complete (Functional Utilities)
- Infrastructure exists for Result/Option types (plugins created, type inference system in place)
- Golden test files exist but are skipped (marked as "Feature not yet implemented")
- Sum types plugin transforms enums to Go structs with full generic support

**Next Milestone**: Phase 3 - Result/Option Integration
- Implement full generic enum system for Result/Option types
- Add configurable None type inference via dingo.toml
- Provide comprehensive helper methods (8-10 per type)
- Enable golden tests for result_* and option_* files

**User Decisions**:
1. **Full generic enum system** (15-20 hours complexity accepted)
2. **Configurable None inference** (both explicit and context-based via dingo.toml)
3. **Comprehensive helpers** (all 8-10 methods per type)

**Timeline**: 22-30 hours (3-4 days focused work)

---

## User Requirements Summary

### 1. Full Generic Enum System

Implement complete generic type parameters for all enums:

```dingo
// Generic enum definitions
enum Result[T, E] {
    Ok(T),
    Err(E)
}

enum Option[T] {
    Some(T),
    None
}

// Usage with type inference
x := Ok(42)               // Result[int, error]
y := Some("hello")         // Option[string]

// Explicit type parameters
let z: Result[int, MyError] = Ok(42)
```

**Implications**:
- Parser must support generic type parameter syntax `<T, E>`
- Type inference must handle parameter substitution
- Sum types plugin must generate generic Go code
- Higher complexity but provides best foundation

**Effort**: 15-20 hours

### 2. Configurable None Type Inference

Support BOTH modes via dingo.toml configuration:

**Mode 1: Explicit (Safer)**
```dingo
// Requires explicit type annotation
let x: Option[int] = None    // OK
y := None                  // ERROR: Cannot infer type
```

**Mode 2: Context-based (Ergonomic)**
```dingo
// Infer from assignment context
let x: Option[int] = None    // OK (explicit)
y := None                  // OK (infers from later usage or return type)

fn getName() Option[string] {
    return None              // OK (infers from return type)
}
```

**Configuration** (add to dingo.toml):
```toml
[features]
# None type inference mode: "explicit" | "context"
# - explicit: Requires type annotation (safer, clearer)
# - context: Infers from assignment/return context (ergonomic)
none_type_inference = "context"  # Default
```

**Implications**:
- Need to implement config loading for `none_type_inference` option
- Type inference service must support both modes
- Parser may need context tracking for inference
- Mode selection affects error messages

**Effort**: 3-4 hours (config system + inference logic)

### 3. Comprehensive Helper Methods

Implement all 8-10 helper methods per type for best developer experience:

**Result[T, E] Methods** (8 methods):
1. `Unwrap() T` - Unwrap Ok value (panic on Err)
2. `UnwrapOr(default T) T` - Unwrap or return default
3. `UnwrapErr() E` - Unwrap Err value (panic on Ok)
4. `Map(fn func(T) U) Result[U, E]` - Transform Ok value
5. `MapErr(fn func(E) F) Result[T, F]` - Transform Err value
6. `AndThen(fn func(T) Result[U, E]) Result[U, E]` - Chainable operation
7. `IsOk() bool` - Check if Ok (from sum types)
8. `IsErr() bool` - Check if Err (from sum types)

**Option[T] Methods** (10 methods):
1. `Unwrap() T` - Unwrap Some value (panic on None)
2. `UnwrapOr(default T) T` - Unwrap or return default
3. `Map(fn func(T) U) Option[U]` - Transform Some value
4. `Filter(fn func(T) bool) Option[T]` - Filter based on predicate
5. `IsSomeAnd(fn func(T) bool) bool` - Check Some with predicate
6. `And(other Option[T]) Option[T]` - Logical AND
7. `Or(other Option[T]) Option[T]` - Logical OR
8. `AndThen(fn func(T) Option[U]) Option[U]` - Chainable operation
9. `IsSome() bool` - Check if Some (from sum types)
10. `IsNone() bool` - Check if None (from sum types)

**Implications**:
- More code to write/test (~400-500 lines)
- Better developer experience (Rust-like API surface)
- Comprehensive documentation needed
- All methods must be unit tested

**Effort**: 8-10 hours (implementation + testing)

---

## Architecture Overview

### Core Strategy: Generic Enums + Helper Injection

**Key Insight**: Result and Option ARE sum types with generic parameters and additional helper methods.

**Architecture**:
1. **Parser**: Recognize generic enum syntax `enum Result[T, E] { ... }`
2. **Sum Types Plugin**: Transform generic enums to Go structs with type parameters
3. **Result/Option Plugins**: Inject helper methods onto generated types
4. **Type Inference**: Handle generic parameter substitution and None inference
5. **Configuration**: Load `none_type_inference` from dingo.toml

**Benefits**:
- Reuses sum types transformation (926 lines, battle-tested)
- Consistent behavior between user enums and built-in types
- Generic system works for ALL enums (not just Result/Option)
- Configuration allows user preference

---

## Detailed Implementation Plan

### Phase 3.1: Generic Enum Parser (5-7 hours)

**Goal**: Support generic type parameter syntax in enum declarations

**Changes**:

1. **AST Enhancement** (`pkg/ast/ast.go`):
```go
type EnumDeclaration struct {
    Name         *ast.Ident
    TypeParams   []*TypeParameter  // NEW: Generic type parameters
    Variants     []*EnumVariant
    IsResultType bool              // NEW: Tagged as Result
    IsOptionType bool              // NEW: Tagged as Option
}

type TypeParameter struct {
    Name        string             // "T", "E", etc.
    Constraints []ast.Expr         // Type constraints (optional)
}
```

2. **Parser Enhancement** (`pkg/parser/participle.go`):
```go
// Parse: enum Result[T, E] { Ok(T), Err(E) }
type EnumSyntax struct {
    Name       string              `@Ident`
    TypeParams *TypeParamList      `[ "<" @@ ">" ]`
    Variants   []VariantSyntax     `"{" @@ { "," @@ } "}"`
}

type TypeParamList struct {
    Params []string               `@Ident { "," @Ident }`
}

// After parsing:
if enumDecl.Name.Name == "Result" {
    enumDecl.IsResultType = true
    validateResultStructure(enumDecl)  // Exactly 2 variants: Ok(T), Err(E)
}
if enumDecl.Name.Name == "Option" {
    enumDecl.IsOptionType = true
    validateOptionStructure(enumDecl)  // Exactly 2 variants: Some(T), None
}
```

3. **Validation**:
- Result: Must have exactly 2 variants named Ok and Err
- Result: Must have exactly 2 type parameters (T, E)
- Option: Must have exactly 2 variants named Some and None
- Option: Must have exactly 1 type parameter (T)
- Clear error messages with source positions

**Deliverables**:
- [ ] AST supports generic type parameters
- [ ] Parser recognizes `<T, E>` syntax
- [ ] Result/Option enums tagged and validated
- [ ] Unit tests for parser validation
- [ ] Error messages tested

**Success Criteria**: Parser correctly parses `enum Result[T, E] { Ok(T), Err(E) }`

---

### Phase 3.2: Generic Sum Types Transform (6-8 hours)

**Goal**: Transform generic enums to Go structs with type parameters

**Current Limitation**: Sum types plugin generates concrete types only

**Enhancement Needed**:

1. **Type Parameter Tracking** (`pkg/plugin/builtin/sum_types.go`):
```go
func (p *SumTypesPlugin) transformEnumDeclaration(enumDecl *dingoast.EnumDeclaration) []ast.Decl {
    // Extract type parameters
    typeParams := extractTypeParams(enumDecl.TypeParams)

    // For Result[T, E] with usage Result[int, error]:
    // Generate: Result_int_error struct
    typeName := generateTypeName(enumDecl.Name.Name, typeParams, concreteTypes)

    // Register with type inference
    if enumDecl.IsResultType || enumDecl.IsOptionType {
        if typeInf, ok := p.Ctx.GetTypeInference(); ok {
            typeInf.RegisterGenericEnum(
                enumDecl.Name.Name,  // "Result"
                typeParams,           // ["T", "E"]
                typeName,             // "Result_int_error"
            )
        }
    }

    // Generate struct with concrete types substituted
    return generateEnumStruct(enumDecl, typeParams, concreteTypes)
}
```

2. **Type Parameter Substitution**:
```go
// Input: enum Result[T, E] { Ok(T), Err(E) }
// Usage: Ok(42) where T=int, E=error
// Output:
type Result_int_error struct {
    tag Result_int_error_Tag
    ok_0 *int      // Substituted T -> int
    err_0 *error   // Substituted E -> error
}
```

3. **Generic Constructor Generation**:
```go
// Generate: Result_Ok(arg0 int) Result_int_error
// Generate: Result_Err(arg0 error) Result_int_error
// Generate: Option_Some(arg0 T) Option_T
// Generate: Option_None() Option_T  // Zero-arg constructor
```

**Deliverables**:
- [ ] Sum types handles generic type parameters
- [ ] Type parameter substitution working
- [ ] Generic constructors generated
- [ ] Result/Option registered with type inference
- [ ] Unit tests for generic transformation

**Success Criteria**: `enum Result[T, E] { Ok(T), Err(E) }` transforms to valid Go struct

---

### Phase 3.3: Configuration System (3-4 hours)

**Goal**: Add `none_type_inference` option to dingo.toml and config loading

**Implementation**:

1. **Config Structure** (`pkg/config/config.go` - create if needed):
```go
package config

type DingoConfig struct {
    Features   FeaturesConfig   `toml:"features"`
    SourceMaps SourceMapsConfig `toml:"sourcemaps"`
}

type FeaturesConfig struct {
    ErrorPropagationSyntax  string `toml:"error_propagation_syntax"`
    LambdaSyntax            string `toml:"lambda_syntax"`
    SafeNavigationUnwrap    string `toml:"safe_navigation_unwrap"`
    NullCoalescingPointers  bool   `toml:"null_coalescing_pointers"`
    OperatorPrecedence      string `toml:"operator_precedence"`

    // NEW: None type inference mode
    NoneTypeInference       string `toml:"none_type_inference"`  // "explicit" | "context"
}

// Validation
func (f *FeaturesConfig) Validate() error {
    validModes := []string{"explicit", "context"}
    if !contains(validModes, f.NoneTypeInference) {
        return fmt.Errorf("invalid none_type_inference: %s (expected: explicit or context)", f.NoneTypeInference)
    }
    return nil
}
```

2. **Config Loading** (`pkg/config/loader.go`):
```go
package config

import "github.com/BurntSushi/toml"

func LoadConfig(path string) (*DingoConfig, error) {
    var config DingoConfig

    // Set defaults
    config.Features.NoneTypeInference = "context"  // Default: ergonomic

    // Load from file
    if _, err := toml.DecodeFile(path, &config); err != nil {
        return nil, err
    }

    // Validate
    if err := config.Features.Validate(); err != nil {
        return nil, err
    }

    return &config, nil
}
```

3. **Plugin Context Integration** (`pkg/plugin/context.go`):
```go
type Context struct {
    // ... existing fields ...
    Config *config.DingoConfig  // NEW: Configuration
}

// Access in plugins
func (p *OptionTypePlugin) shouldInferNone() bool {
    return p.Ctx.Config.Features.NoneTypeInference == "context"
}
```

4. **Update dingo.toml**:
```toml
[features]
# ... existing options ...

# None type inference mode: "explicit" | "context"
# - explicit: Requires type annotation: let x: Option[int] = None
# - context: Infers from assignment/return context (default)
none_type_inference = "context"
```

**Deliverables**:
- [ ] Config package created
- [ ] `none_type_inference` option added to dingo.toml
- [ ] Config loading and validation
- [ ] Plugin context integration
- [ ] Unit tests for config loading

**Success Criteria**: Plugins can read `none_type_inference` from config

---

### Phase 3.4: Type Inference Enhancement (4-5 hours)

**Goal**: Handle generic parameter substitution and configurable None inference

**Enhancements**:

1. **Generic Type Registry** (`pkg/plugin/builtin/type_inference.go`):
```go
type TypeInferenceService struct {
    // ... existing fields ...

    // NEW: Generic enum definitions
    genericEnums map[string]*GenericEnumInfo  // "Result" -> GenericEnumInfo

    // NEW: Concrete instantiations
    concreteTypes map[string]*ConcreteTypeInfo  // "Result_int_error" -> ConcreteTypeInfo
}

type GenericEnumInfo struct {
    Name       string            // "Result", "Option"
    TypeParams []string          // ["T", "E"] or ["T"]
    Variants   []VariantInfo     // Ok(T), Err(E) or Some(T), None
}

type ConcreteTypeInfo struct {
    GenericName string           // "Result"
    ConcreteName string          // "Result_int_error"
    Bindings map[string]ast.Expr // "T" -> int, "E" -> error
}
```

2. **Type Parameter Substitution**:
```go
// When encountering: Ok(42)
// 1. Look up "Ok" -> belongs to "Result"
// 2. Infer argument types: T=int
// 3. Check if E can be inferred (assignment context, return type)
// 4. Generate concrete type: Result_int_error
// 5. Transform to: Result_Ok(42)

func (s *TypeInferenceService) InferConstructorType(
    variantName string,    // "Ok"
    argTypes []ast.Expr,   // [int]
    context *InferenceContext,
) (string, error) {
    // Find generic enum
    generic := s.findGenericForVariant(variantName)
    if generic == nil {
        return "", fmt.Errorf("unknown variant: %s", variantName)
    }

    // Infer type parameters
    bindings, err := s.inferTypeParams(generic, argTypes, context)
    if err != nil {
        return "", err
    }

    // Generate concrete type name
    return generateConcreteTypeName(generic.Name, bindings), nil
}
```

3. **Configurable None Inference**:
```go
func (s *TypeInferenceService) InferNoneType(context *InferenceContext) (ast.Expr, error) {
    mode := s.Ctx.Config.Features.NoneTypeInference

    switch mode {
    case "explicit":
        // Require type annotation in context
        if context.ExpectedType == nil {
            return nil, fmt.Errorf("None requires explicit type annotation (none_type_inference = explicit)")
        }
        return context.ExpectedType, nil

    case "context":
        // Try to infer from context
        if typ, ok := s.inferFromContext(context); ok {
            return typ, nil
        }
        // Fallback: require annotation
        return nil, fmt.Errorf("Cannot infer type for None (add type annotation or set none_type_inference = explicit)")

    default:
        return nil, fmt.Errorf("invalid none_type_inference mode: %s", mode)
    }
}
```

4. **Inference Context**:
```go
type InferenceContext struct {
    ExpectedType *ast.Expr      // From assignment or return type
    Scope        *Scope          // Variable bindings
    FunctionSig  *ast.FuncType   // Current function signature
}

// Example usage:
// let x: Option[int] = None
//   -> ExpectedType = Option[int] (from variable declaration)
//
// fn getName() Option[string] { return None }
//   -> ExpectedType = Option[string] (from return type)
```

**Deliverables**:
- [ ] Generic enum registry
- [ ] Type parameter substitution
- [ ] Configurable None inference
- [ ] Inference context tracking
- [ ] Unit tests for both modes

**Success Criteria**:
- `Ok(42)` infers to `Result[int, error]` (if E can be inferred)
- None inference works in both explicit and context modes

---

### Phase 3.5: Result Helper Methods (4-5 hours)

**Goal**: Implement comprehensive Result[T, E] helper methods

**Refactor Strategy**: Remove constructor transformation, focus on helper injection

**Implementation**:

1. **Plugin Structure** (`pkg/plugin/builtin/result_type.go`):
```go
type ResultTypePlugin struct {
    plugin.BasePlugin
    resultTypes map[string]*ResultTypeInfo  // Track Result types
}

type ResultTypeInfo struct {
    TypeName   string       // "Result_int_error"
    OkType     ast.Expr     // int
    ErrType    ast.Expr     // error
    Position   token.Pos    // Source position
}

func (p *ResultTypePlugin) Transform(ctx *plugin.Context, node ast.Node) (ast.Node, error) {
    // Phase 1: Collect Result type declarations (from sum types output)
    ast.Inspect(node, func(n ast.Node) bool {
        if typeSpec, ok := n.(*ast.TypeSpec); ok {
            if isResultType(typeSpec.Name.Name) {
                p.trackResultType(typeSpec)
            }
        }
        return true
    })

    // Phase 2: Generate helper methods for each Result type
    for typeName, info := range p.resultTypes {
        helpers := p.generateHelpers(typeName, info)
        // Inject after type declaration
        ctx.InjectDeclarations(info.Position, helpers)
    }

    return node, nil
}
```

2. **Helper Method Generation**:
```go
// Unwrap() T - Unwrap Ok value (panic on Err)
func (p *ResultTypePlugin) generateUnwrap(typeName string, okType ast.Expr) *ast.FuncDecl {
    return &ast.FuncDecl{
        Recv: &ast.FieldList{
            List: []*ast.Field{{
                Names: []*ast.Ident{ast.NewIdent("r")},
                Type:  ast.NewIdent(typeName),
            }},
        },
        Name: ast.NewIdent("Unwrap"),
        Type: &ast.FuncType{
            Results: &ast.FieldList{
                List: []*ast.Field{{Type: okType}},
            },
        },
        Body: &ast.BlockStmt{
            List: []ast.Stmt{
                // if r.tag != ResultTag_Ok { panic("...") }
                &ast.IfStmt{
                    Cond: &ast.BinaryExpr{
                        X:  &ast.SelectorExpr{X: ast.NewIdent("r"), Sel: ast.NewIdent("tag")},
                        Op: token.NEQ,
                        Y:  ast.NewIdent(typeName + "_Tag_Ok"),
                    },
                    Body: &ast.BlockStmt{
                        List: []ast.Stmt{
                            &ast.ExprStmt{
                                X: &ast.CallExpr{
                                    Fun: ast.NewIdent("panic"),
                                    Args: []ast.Expr{
                                        &ast.BasicLit{
                                            Kind:  token.STRING,
                                            Value: `"called Unwrap on Err value"`,
                                        },
                                    },
                                },
                            },
                        },
                    },
                },
                // return *r.ok_0
                &ast.ReturnStmt{
                    Results: []ast.Expr{
                        &ast.StarExpr{
                            X: &ast.SelectorExpr{
                                X:   ast.NewIdent("r"),
                                Sel: ast.NewIdent("ok_0"),
                            },
                        },
                    },
                },
            },
        },
    }
}

// UnwrapOr(default T) T - Unwrap or return default
func (p *ResultTypePlugin) generateUnwrapOr(typeName string, okType ast.Expr) *ast.FuncDecl {
    // Similar structure, but return default on Err
}

// Map(fn func(T) U) Result[U, E] - Transform Ok value
func (p *ResultTypePlugin) generateMap(typeName string, okType, errType ast.Expr) *ast.FuncDecl {
    // Apply fn to Ok value, preserve Err
    // Requires: Creating new Result type Result[U, E]
}

// AndThen(fn func(T) Result[U, E]) Result[U, E] - Chainable operation
func (p *ResultTypePlugin) generateAndThen(typeName string, okType, errType ast.Expr) *ast.FuncDecl {
    // Call fn on Ok value, flatten Result[Result<U, E], E> -> Result[U, E]
}

// UnwrapErr() E - Unwrap Err value (panic on Ok)
// MapErr(fn func(E) F) Result[T, F] - Transform Err value
// (IsOk, IsErr already generated by sum types plugin)
```

**Deliverables**:
- [ ] Result plugin refactored (helper injection only)
- [ ] All 8 helper methods implemented
- [ ] Unit tests for each method
- [ ] Enable result_01_basic.dingo golden test

**Success Criteria**: result_01_basic.dingo transpiles and compiles

---

### Phase 3.6: Option Helper Methods (4-5 hours)

**Goal**: Implement comprehensive Option[T] helper methods

**Implementation**: Mirror Result plugin approach

**Helper Methods**:
```go
// Unwrap() T - Unwrap Some value (panic on None)
// UnwrapOr(default T) T - Unwrap or return default
// Map(fn func(T) U) Option[U] - Transform Some value
// Filter(fn func(T) bool) Option[T] - Filter based on predicate
// IsSomeAnd(fn func(T) bool) bool - Check Some with predicate
// And(other Option[T]) Option[T] - Logical AND
// Or(other Option[T]) Option[T] - Logical OR
// AndThen(fn func(T) Option[U]) Option[U] - Chainable operation
// (IsSome, IsNone already from sum types)
```

**Special Case: None Constructor**:
```go
// Generated by sum types: Option_None() Option_T
// Type inference challenge: What is T?

// Solution 1 (explicit mode):
let x: Option[int] = None  // Type from annotation

// Solution 2 (context mode):
fn getName() Option[string] {
    return None  // Type from return type
}

let y: Option[int] = Some(42).Or(None)  // Type from Or argument
```

**Deliverables**:
- [ ] Option plugin refactored (helper injection only)
- [ ] All 10 helper methods implemented
- [ ] None type inference (both modes)
- [ ] Unit tests for each method
- [ ] Enable option_01_basic.dingo golden test

**Success Criteria**: option_01_basic.dingo transpiles and compiles

---

### Phase 3.7: Integration & Testing (5-7 hours)

**Goal**: Enable all golden tests and verify end-to-end functionality

**Tasks**:

1. **Enable Result Golden Tests** (5 files):
   - `result_01_basic.dingo` - Basic Result usage
   - `result_02_propagation.dingo` - Integration with `?` operator
   - `result_03_pattern_match.dingo` - Integration with match expressions
   - `result_04_chaining.dingo` - Map/AndThen chaining
   - `result_05_go_interop.dingo` - Wrapping Go (T, error) functions

2. **Enable Option Golden Tests** (4 files):
   - `option_01_basic.dingo` - Basic Option usage
   - `option_02_pattern_match.dingo` - Integration with match
   - `option_03_chaining.dingo` - Map/Filter chaining
   - `option_04_go_interop.dingo` - Wrapping Go pointer returns

3. **Integration Tests**:
```dingo
// Test: error_prop_result.dingo
enum Result[T, E] { Ok(T), Err(E) }

fn divide(a int, b int) Result[int, error] {
    if b == 0 {
        return Err(errors.New("division by zero"))
    }
    return Ok(a / b)
}

fn calculate() (int, error) {
    result := divide(10, 2)?  // Unwraps Result -> (int, error)
    return result, nil
}
```

```dingo
// Test: pattern_match_option.dingo
enum Option[T] { Some(T), None }

fn getUserName(id int) Option[string] {
    if id == 1 {
        return Some("Alice")
    }
    return None
}

fn greet(id int) {
    match getUserName(id) {
        Some(name) => println("Hello", name),
        None => println("User not found"),
    }
}
```

4. **Configuration Testing**:
```dingo
// Test with none_type_inference = "explicit"
let x: Option[int] = None  // OK
y := None               // ERROR

// Test with none_type_inference = "context"
let x: Option[int] = None  // OK
fn test() Option[int] {
    return None            // OK (infers from return type)
}
```

5. **Bug Fixes**:
- Fix any golden test mismatches
- Address type inference edge cases
- Resolve plugin ordering issues
- Handle error messages

**Deliverables**:
- [ ] All 9 golden tests passing (5 result + 4 option)
- [ ] Integration tests passing
- [ ] Config modes tested
- [ ] Bugs fixed
- [ ] Test coverage >90%

**Success Criteria**: All result_* and option_* tests green

---

### Phase 3.8: Documentation (2-3 hours)

**Goal**: Complete documentation for Result/Option features

**Tasks**:

1. **Reasoning Documents**:
```markdown
// result_01_basic.reasoning.md
# Result[T, E] Type - Implementation Reasoning

## Community Context
- Go Proposal #48916: Result type (500+ 👍)
- Common pain point: (T, error) verbosity
- Proven in Rust, Scala, Haskell

## Design Decisions
1. Generic enum approach (not built-in keyword)
2. Reuses sum types infrastructure
3. Configurable None inference

## Implementation Highlights
- Zero runtime overhead (inline helpers)
- Idiomatic Go output
- 65% code reduction vs manual error handling

## Code Reduction Metrics
Before (Go): 15 lines
After (Dingo): 5 lines
Reduction: 67%
```

2. **Update Golden Test README**:
```markdown
## Result/Option Tests (9 tests)

### Result Type (5 tests)
- result_01_basic - Basic Result[T, E] usage
- result_02_propagation - Integration with ? operator
- result_03_pattern_match - Pattern matching
- result_04_chaining - Map/AndThen chaining
- result_05_go_interop - Wrapping Go functions

### Option Type (4 tests)
- option_01_basic - Basic Option[T] usage
- option_02_pattern_match - Pattern matching
- option_03_chaining - Map/Filter chaining
- option_04_go_interop - Pointer wrapping
```

3. **Update Main README**:
```markdown
## Features

### Result[T, E] Type ✅
Replace `(value, error)` with `Result[T, E]`:
```dingo
enum Result[T, E] { Ok(T), Err(E) }

fn divide(a int, b int) Result[int, error] {
    if b == 0 {
        return Err(errors.New("division by zero"))
    }
    return Ok(a / b)
}

result := divide(10, 2)
    .Map(|x| x * 2)
    .UnwrapOr(0)
```

### Configuration
Configure None type inference in dingo.toml:
```toml
[features]
none_type_inference = "context"  # or "explicit"
```
```

4. **Update CHANGELOG.md**:
```markdown
## [Unreleased]

### Added
- Full generic enum system for Result[T, E] and Option[T]
- Configurable None type inference (explicit vs context)
- Comprehensive helper methods (8-10 per type)
- 9 golden tests for Result/Option (all passing)

### Changed
- Result/Option now use generic enum syntax
- Type inference supports generic parameter substitution

### Implementation
- 22-30 hours total effort
- 400-500 lines added (helpers + config)
- 500+ lines removed (refactoring)
- Net code reduction: ~100 lines
```

**Deliverables**:
- [ ] result_01_basic.reasoning.md written
- [ ] option_01_basic.reasoning.md written
- [ ] Golden test README updated
- [ ] Main README updated with examples
- [ ] CHANGELOG.md updated

**Success Criteria**: Complete documentation for all features

---

## Package Structure

```
dingo/
├── dingo.toml                      # Config: none_type_inference option
├── pkg/
│   ├── ast/
│   │   └── ast.go                  # MODIFIED: Add TypeParams, IsResultType, IsOptionType
│   ├── parser/
│   │   └── participle.go           # MODIFIED: Parse generic enum syntax
│   ├── config/                     # NEW: Configuration system
│   │   ├── config.go               # Config structures
│   │   └── loader.go               # TOML loading
│   └── plugin/
│       ├── context.go              # MODIFIED: Add Config field
│       └── builtin/
│           ├── sum_types.go        # MODIFIED: Handle generic type params
│           ├── type_inference.go   # MODIFIED: Generic param substitution
│           ├── result_type.go      # REFACTORED: Helper methods only (~200 lines)
│           └── option_type.go      # REFACTORED: Helper methods only (~180 lines)
└── tests/
    └── golden/
        ├── result_01_basic.dingo   # ENABLED: Basic Result
        ├── result_01_basic.go.golden
        ├── option_01_basic.dingo   # ENABLED: Basic Option
        └── option_01_basic.go.golden
```

**File Changes Summary**:
- **Modified**: 6 files (ast.go, participle.go, context.go, sum_types.go, type_inference.go, result_type.go, option_type.go)
- **Created**: 3 files (config.go, loader.go, option_type_refactored.go)
- **Tests**: 9 golden tests enabled, 20+ unit tests added

---

## Timeline & Effort

### Detailed Breakdown

| Phase | Task | Hours | Cumulative |
|-------|------|-------|------------|
| 3.1 | Generic Enum Parser | 5-7 | 5-7 |
| 3.2 | Generic Sum Types Transform | 6-8 | 11-15 |
| 3.3 | Configuration System | 3-4 | 14-19 |
| 3.4 | Type Inference Enhancement | 4-5 | 18-24 |
| 3.5 | Result Helper Methods | 4-5 | 22-29 |
| 3.6 | Option Helper Methods | 4-5 | 26-34 |
| 3.7 | Integration & Testing | 5-7 | 31-41 |
| 3.8 | Documentation | 2-3 | 33-44 |

**Total**: 33-44 hours (realistic with complexity)

### Adjusted Estimate
- **Implementation**: 26-34 hours
- **Testing**: Included in phases
- **Documentation**: 2-3 hours
- **Buffer**: 5-7 hours (generics complexity, edge cases)

**Realistic Total**: 33-44 hours (4-6 days focused work)

### Risk Factors
- Generic type system is complex (may take longer)
- Type inference edge cases may surface
- None inference context tracking non-trivial
- Integration testing may reveal unexpected issues

**Confidence**: Medium-High (complex features, but clear architecture)

---

## Success Metrics

### Quantitative
- [ ] 9 golden tests passing (5 result + 4 option)
- [ ] 30+ unit tests passing (helpers + config + inference)
- [ ] <800 lines of code added
- [ ] >200 lines of code removed (refactoring)
- [ ] 100% test coverage on helper methods
- [ ] 0 panics or crashes

### Qualitative
- [ ] Generated Go code is idiomatic
- [ ] Error messages are clear (type inference failures)
- [ ] Config system is intuitive
- [ ] Code is maintainable (reuses infrastructure)
- [ ] Documentation is comprehensive
- [ ] Feature works end-to-end (write → transpile → compile → run)

### Configuration
- [ ] Both none_type_inference modes work
- [ ] Config validation catches invalid options
- [ ] Clear error messages for config issues

---

## Risk Assessment

### High Risk

1. **Generic Type Parameter Complexity**:
   - **Risk**: Type substitution and inference more complex than expected
   - **Mitigation**: Start with simple cases (Result[int, error]), add complexity incrementally
   - **Fallback**: Support concrete types first, add full generics in Phase 3.9

2. **None Inference Context Tracking**:
   - **Risk**: Context-based inference may be incomplete (missing cases)
   - **Mitigation**: Default to "explicit" mode, make "context" opt-in
   - **Fallback**: Require type annotations for None (explicit mode only)

### Medium Risk

1. **Plugin Ordering Dependencies**:
   - **Risk**: Wrong order causes crashes or incorrect output
   - **Mitigation**: Explicit dependencies in plugin registration, comprehensive testing
   - **Test**: Verify order in unit tests

2. **Configuration System Integration**:
   - **Risk**: Config loading may fail in edge cases
   - **Mitigation**: Default values, validation, clear error messages
   - **Test**: Config validation unit tests

### Low Risk

1. **Golden Test Mismatches**:
   - **Risk**: Generated code doesn't match .go.golden files
   - **Mitigation**: Regenerate golden files, document differences

2. **Helper Method Completeness**:
   - **Risk**: Missing helper methods or incorrect signatures
   - **Mitigation**: Cross-reference Rust Option/Result APIs, comprehensive tests

---

## Open Questions

### Resolved by User
- [x] Generic type parameters? **ANSWER: Full generic system**
- [x] None type inference mode? **ANSWER: Both modes, configurable**
- [x] How many helper methods? **ANSWER: Comprehensive (8-10 per type)**

### To Be Resolved During Implementation

1. **Type Parameter Constraints**:
   - Should we support `enum Result[T, E: Error]` (E must implement error)?
   - Decision: Defer to Phase 4+ (not needed for MVP)

2. **Multiple Generic Instantiations**:
   - How to handle `Result[int, error]` and `Result[string, error]` in same file?
   - Decision: Generate both types (Result_int_error, Result_string_error)

3. **None Constructor Type Hint**:
   - Should we allow `None::<int>()` for explicit type?
   - Decision: Allow if user prefers, but context inference is default

4. **Helper Method Inlining**:
   - Should helpers be marked for inlining (`//go:inline`)?
   - Decision: Yes, for zero-cost abstractions

---

## Future Enhancements (Not in Phase 3)

### Phase 4+ Candidates

1. **Try Blocks** (Rust-style):
```dingo
result := try {
    a := operation1()?;
    b := operation2(a)?;
    b * 2
}  // Returns Result[int, error]
```
**Effort**: 8-10 hours

2. **Auto-wrapping Go Functions**:
```dingo
// Go: func ReadFile(path string) ([]byte, error)
// Auto-wrap: ReadFile("file.txt") -> Result[[]byte, error]
content := ReadFile("file.txt")?  // No manual wrapping
```
**Effort**: 15-20 hours (requires go/types integration)

3. **Railway-Oriented Programming Helpers**:
- `Tap(fn)`: Side effects without transformation
- `Recover(fn)`: Convert Err to Ok with recovery
- `Transpose()`: Result[Option[T], E] → Option[Result[T, E]]
**Effort**: 6-8 hours

4. **Result/Option Literals**:
```dingo
let x: Result[int, error] = .Ok(42)    // Leading dot syntax
let y: Option[string] = .Some("hello")
```
**Effort**: 4-6 hours

---

## Implementation Checklist

### Pre-Implementation
- [ ] Review plan with user
- [ ] Confirm timeline (33-44 hours)
- [ ] Set up development environment
- [ ] Create feature branch

### Phase 3.1: Generic Enum Parser (5-7 hours)
- [ ] Add TypeParams to EnumDeclaration AST
- [ ] Add IsResultType, IsOptionType flags
- [ ] Update parser to recognize `<T, E>` syntax
- [ ] Implement Result/Option validation
- [ ] Write unit tests for parser
- [ ] Test error messages

### Phase 3.2: Generic Sum Types (6-8 hours)
- [ ] Enhance sum types for generic type params
- [ ] Implement type parameter substitution
- [ ] Generate generic constructors
- [ ] Register with type inference
- [ ] Write unit tests for transformation
- [ ] Verify Result/Option registration

### Phase 3.3: Configuration System (3-4 hours)
- [ ] Create pkg/config package
- [ ] Define DingoConfig structure
- [ ] Implement TOML loading
- [ ] Add validation
- [ ] Integrate with plugin context
- [ ] Update dingo.toml
- [ ] Write unit tests

### Phase 3.4: Type Inference (4-5 hours)
- [ ] Create generic enum registry
- [ ] Implement type parameter substitution
- [ ] Add configurable None inference
- [ ] Implement inference context tracking
- [ ] Write unit tests for both modes
- [ ] Test edge cases

### Phase 3.5: Result Helpers (4-5 hours)
- [ ] Refactor Result plugin
- [ ] Implement Unwrap method
- [ ] Implement UnwrapOr method
- [ ] Implement Map method
- [ ] Implement AndThen method
- [ ] Implement UnwrapErr, MapErr
- [ ] Write unit tests
- [ ] Enable result_01_basic golden test

### Phase 3.6: Option Helpers (4-5 hours)
- [ ] Refactor Option plugin
- [ ] Implement Unwrap method
- [ ] Implement UnwrapOr method
- [ ] Implement Map method
- [ ] Implement Filter method
- [ ] Implement IsSomeAnd, And, Or, AndThen
- [ ] Handle None inference
- [ ] Write unit tests
- [ ] Enable option_01_basic golden test

### Phase 3.7: Integration & Testing (5-7 hours)
- [ ] Enable all 9 golden tests
- [ ] Run integration tests
- [ ] Test config modes
- [ ] Fix discovered bugs
- [ ] Achieve 90%+ coverage
- [ ] Verify all tests green

### Phase 3.8: Documentation (2-3 hours)
- [ ] Write result_01_basic.reasoning.md
- [ ] Write option_01_basic.reasoning.md
- [ ] Update golden test README
- [ ] Update main README with examples
- [ ] Update CHANGELOG.md
- [ ] Review documentation

### Post-Implementation
- [ ] Code review
- [ ] Performance testing
- [ ] Update project status
- [ ] Plan Phase 4

---

## Conclusion

Phase 3 implements a comprehensive generic enum system for Result/Option types with:
- Full generic type parameters for all enums
- Configurable None type inference (explicit vs context)
- Comprehensive helper methods (8-10 per type)
- Strong foundation for future enhancements

**Key Architectural Decisions**:
1. Reuse sum types infrastructure for transformation
2. Add generic type parameter support in parser/transformer
3. Provide configurable type inference for ergonomics
4. Inject helper methods onto generated types

**Expected Outcomes**:
- 9 golden tests enabled and passing
- Result/Option types fully functional with generics
- Configuration system for user preferences
- Foundation for Pattern Matching (Phase 4)
- Clean, maintainable codebase

**Timeline**: 33-44 hours (4-6 days focused work)
**Confidence**: Medium-High (complex but well-architected)
**Risk**: Managed through incremental implementation and comprehensive testing
