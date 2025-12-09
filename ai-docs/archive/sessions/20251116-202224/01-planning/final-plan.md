# Sum Types Implementation Plan - Final
**Session:** 20251116-202224
**Date:** 2025-11-16
**Feature:** Sum Types (Discriminated Unions)
**Priority:** P0 - Critical Foundation
**Duration:** 3-4 weeks

---

## Executive Summary

This plan finalizes the implementation of sum types in Dingo, incorporating all user design decisions. Sum types are the foundational type system feature enabling Result[T, E] and Option[T] - Dingo's core value proposition.

**Key Design Decisions:**
- Match arms use `=>` (Rust-style, avoids arrow function conflict)
- Match is an expression (can return values)
- Exhaustiveness checking: Error on missing cases, allow `_` wildcard
- Auto-generate Is* helper methods for all variants
- Result/Option in standard prelude (auto-imported)
- Trailing commas allowed in variant lists

---

## 1. Updated Syntax Examples

### 1.1 Enum Declaration with Trailing Commas

```dingo
// Trailing commas allowed for better diffs
enum HttpResponse {
    Ok(body: string),
    NotFound,
    ServerError{code: int, message: string},
    Redirect(url: string),  // Trailing comma OK
}

// Generic enums
enum Result[T, E] {
    Ok(T),
    Err(E),  // Trailing comma OK
}

enum Option[T] {
    Some(T),
    None,  // Trailing comma OK
}
```

### 1.2 Match Expressions with `=>`

```dingo
// Match as expression (can assign result)
let area = match shape {
    Circle { radius } => 3.14 * radius * radius,
    Rectangle { width, height } => width * height,
    Point => 0.0,  // Trailing comma OK
}

// Match as statement (no return value)
match response {
    Ok(body) => println("Success: {}", body),
    NotFound => println("404 Not Found"),
    ServerError{code, message} => println("Error {}: {}", code, message),
    Redirect(url) => println("Redirecting to {}", url),
}

// With wildcard catch-all
match status {
    Pending => "waiting",
    Approved => "done",
    _ => "other",  // Wildcard allowed
}
```

### 1.3 Exhaustiveness Enforcement

```dingo
enum Status { Pending, Approved, Rejected }

// ❌ Compile error: Missing cases
match status {
    Pending => "waiting",
    Approved => "done"
    // ERROR: non-exhaustive match: missing cases for [Rejected]
}

// ✅ OK: All cases covered
match status {
    Pending => "waiting",
    Approved => "done",
    Rejected => "rejected"
}

// ✅ OK: Wildcard covers remaining cases
match status {
    Pending => "waiting",
    _ => "other"
}
```

---

## 2. Transpilation Strategy

### 2.1 Enum to Tagged Union

**Dingo Source:**
```dingo
enum Shape {
    Circle { radius: float64 },
    Rectangle { width: float64, height: float64 },
    Point,
}
```

**Generated Go Code:**
```go
// Tag enum
type ShapeTag uint8

const (
    ShapeTag_Circle ShapeTag = iota
    ShapeTag_Rectangle
    ShapeTag_Point
)

// Tagged union struct
type Shape struct {
    tag              ShapeTag
    circle_radius    *float64
    rectangle_width  *float64
    rectangle_height *float64
}

// Constructor functions
func Shape_Circle(radius float64) Shape {
    return Shape{
        tag:           ShapeTag_Circle,
        circle_radius: &radius,
    }
}

func Shape_Rectangle(width float64, height float64) Shape {
    return Shape{
        tag:              ShapeTag_Rectangle,
        rectangle_width:  &width,
        rectangle_height: &height,
    }
}

func Shape_Point() Shape {
    return Shape{
        tag: ShapeTag_Point,
    }
}

// Helper methods (auto-generated)
func (s Shape) IsCircle() bool    { return s.tag == ShapeTag_Circle }
func (s Shape) IsRectangle() bool { return s.tag == ShapeTag_Rectangle }
func (s Shape) IsPoint() bool     { return s.tag == ShapeTag_Point }
```

### 2.2 Match Expression to Switch

**Dingo Source:**
```dingo
func area(shape: Shape) -> float64 {
    match shape {
        Circle{radius} => 3.14 * radius * radius,
        Rectangle{width, height} => width * height,
        Point => 0.0,
    }
}
```

**Generated Go Code:**
```go
func area(shape Shape) float64 {
    switch shape.tag {
    case ShapeTag_Circle:
        radius := *shape.circle_radius
        return 3.14 * radius * radius
    case ShapeTag_Rectangle:
        width := *shape.rectangle_width
        height := *shape.rectangle_height
        return width * height
    case ShapeTag_Point:
        return 0.0
    default:
        panic("unreachable: unhandled Shape variant")
    }
}
```

### 2.3 Helper Method Usage

```dingo
// Using auto-generated helpers
if shape.IsCircle() {
    // Go interop - manual tag checking
}

// Or use match for exhaustive handling
match shape {
    Circle{radius} => println("Circle with radius {}", radius),
    _ => println("Not a circle"),
}
```

---

## 3. Standard Prelude Structure

### 3.1 Prelude Definition

**Location:** `pkg/transpiler/prelude/std.dingo`

```dingo
// Standard prelude - auto-imported into every Dingo file
package std

// Result type for fallible operations
enum Result[T, E] {
    Ok(T),
    Err(E),
}

// Optional values
enum Option[T] {
    Some(T),
    None,
}
```

### 3.2 Prelude Auto-Import Mechanism

**Implementation Strategy:**

1. **Parse Phase**: Every `.dingo` file implicitly imports prelude
2. **Type Registry**: Prelude types registered before user code parsing
3. **No Import Statement**: `Result` and `Option` work without imports
4. **Inspectable**: Prelude source is regular Dingo code (not compiler magic)

**Code:**
```go
// pkg/transpiler/transpiler.go
func (t *Transpiler) ParseFile(path string) (*ast.File, error) {
    // 1. Parse prelude first (once per compilation)
    if !t.preludeLoaded {
        prelude, err := t.ParsePrelude()
        if err != nil {
            return nil, err
        }
        t.RegisterPreludeTypes(prelude)
        t.preludeLoaded = true
    }

    // 2. Parse user file with prelude types available
    return t.parser.Parse(path)
}
```

### 3.3 Generated Prelude Output

**Generated Go (`std_prelude.go`):**
```go
// Auto-generated standard prelude

type ResultTag uint8
const (
    ResultTag_Ok ResultTag = iota
    ResultTag_Err
)

type Result[T any, E any] struct {
    tag ResultTag
    ok  *T
    err *E
}

func Result_Ok[T any, E any](value T) Result[T, E] {
    return Result[T, E]{tag: ResultTag_Ok, ok: &value}
}

func Result_Err[T any, E any](err E) Result[T, E] {
    return Result[T, E]{tag: ResultTag_Err, err: &err}
}

func (r Result[T, E]) IsOk() bool  { return r.tag == ResultTag_Ok }
func (r Result[T, E]) IsErr() bool { return r.tag == ResultTag_Err }

// Option type
type OptionTag uint8
const (
    OptionTag_Some OptionTag = iota
    OptionTag_None
)

type Option[T any] struct {
    tag   OptionTag
    value *T
}

func Option_Some[T any](value T) Option[T] {
    return Option[T]{tag: OptionTag_Some, value: &value}
}

func Option_None[T any]() Option[T] {
    return Option[T]{tag: OptionTag_None}
}

func (o Option[T]) IsSome() bool { return o.tag == OptionTag_Some }
func (o Option[T]) IsNone() bool { return o.tag == OptionTag_None }
```

---

## 4. Parser Changes

### 4.1 Grammar Extensions with Trailing Commas

```go
// pkg/parser/participle.go

type EnumDeclaration struct {
    Name       string             `parser:"'enum' @Ident"`
    TypeParams *TypeParameters    `parser:"( '<' @@ '>' )?"`
    Variants   []*VariantDef      `parser:"'{' ( @@ ( ',' @@ )* ','? )? '}'"`
    //                                                              ^^^ Allow trailing comma
}

type VariantDef struct {
    Name         string        `parser:"@Ident"`
    TupleFields  []*Type       `parser:"( '(' ( @@ ( ',' @@ )* ','? )? ')' )"`
    StructFields []*NamedField `parser:"| ( '{' ( @@ ( ',' @@ )* ','? )? '}' )"`
    //                                                         ^^^ Allow trailing comma
}
```

### 4.2 Match Expression Grammar with `=>`

```go
type MatchExpression struct {
    Expr  *Expression  `parser:"'match' @@"`
    Arms  []*MatchArm  `parser:"'{' ( @@ ( ',' )? )+ '}'"`
    //                                      ^^^^^^ Optional trailing comma
}

type MatchArm struct {
    Pattern *PatternExpr `parser:"@@"`
    Guard   *Expression  `parser:"( 'if' @@ )?"`
    Body    *Expression  `parser:"'=>' @@"`  // Use => instead of :
}

type PatternExpr struct {
    Wildcard       bool              `parser:"@'_'"`
    VariantPattern *VariantPattern   `parser:"| @@"`
    LiteralPattern *PrimaryExpression `parser:"| @@"`
}
```

### 4.3 Lexer Extensions

Add reserved keywords:
- `enum` - Enum declaration
- `match` - Pattern matching expression

Token `=>` already exists (used for closures in some languages, we use it for match arms).

---

## 5. Helper Method Generation Strategy

### 5.1 Generation Rules

For each enum variant, generate:
- `Is<VariantName>() bool` method

**Pattern:**
```go
func (e <EnumType>) Is<VariantName>() bool {
    return e.tag == <EnumType>Tag_<VariantName>
}
```

### 5.2 Implementation

```go
// pkg/plugin/builtin/sum_types.go

func (p *SumTypesPlugin) generateHelperMethods(
    enum *dingoast.EnumDecl,
) []ast.Decl {
    methods := []ast.Decl{}

    for _, variant := range enum.Variants {
        // Generate Is<Variant>() bool method
        method := &ast.FuncDecl{
            Recv: &ast.FieldList{
                List: []*ast.Field{{
                    Names: []*ast.Ident{ast.NewIdent("e")},
                    Type:  ast.NewIdent(enum.Name.Name),
                }},
            },
            Name: ast.NewIdent("Is" + variant.Name.Name),
            Type: &ast.FuncType{
                Params: &ast.FieldList{},
                Results: &ast.FieldList{
                    List: []*ast.Field{{
                        Type: ast.NewIdent("bool"),
                    }},
                },
            },
            Body: &ast.BlockStmt{
                List: []ast.Stmt{
                    &ast.ReturnStmt{
                        Results: []ast.Expr{
                            &ast.BinaryExpr{
                                X:  &ast.SelectorExpr{X: ast.NewIdent("e"), Sel: ast.NewIdent("tag")},
                                Op: token.EQL,
                                Y:  ast.NewIdent(enum.Name.Name + "Tag_" + variant.Name.Name),
                            },
                        },
                    },
                },
            },
        }
        methods = append(methods, method)
    }

    return methods
}
```

### 5.3 Example Output

```dingo
enum Status { Pending, Approved, Rejected }
```

**Generates:**
```go
func (s Status) IsPending() bool  { return s.tag == StatusTag_Pending }
func (s Status) IsApproved() bool { return s.tag == StatusTag_Approved }
func (s Status) IsRejected() bool { return s.tag == StatusTag_Rejected }
```

---

## 6. Exhaustiveness Checking Implementation

### 6.1 Algorithm

**Core Logic:**
1. Collect all variants from enum type
2. Track which variants are covered by match patterns
3. Allow wildcard `_` as catch-all
4. Error if any variant is missing and no wildcard present

**Implementation:**
```go
// pkg/plugin/builtin/exhaustiveness.go

type ExhaustivenessChecker struct {
    enumTypes map[string]*dingoast.EnumType
}

func (c *ExhaustivenessChecker) CheckMatch(
    enumType *dingoast.EnumType,
    arms []*dingoast.MatchArm,
) error {
    covered := make(map[string]bool)
    hasWildcard := false

    // Track covered variants
    for _, arm := range arms {
        if arm.Pattern.Wildcard {
            hasWildcard = true
            break
        }
        covered[arm.Pattern.Variant.Name] = true
    }

    // Wildcard covers all cases
    if hasWildcard {
        return nil
    }

    // Check for missing variants
    missing := []string{}
    for _, variant := range enumType.Variants {
        if !covered[variant.Name] {
            missing = append(missing, variant.Name)
        }
    }

    if len(missing) > 0 {
        return &ExhaustivenessError{
            EnumName: enumType.Name,
            Missing:  missing,
        }
    }

    return nil
}

type ExhaustivenessError struct {
    EnumName string
    Missing  []string
}

func (e *ExhaustivenessError) Error() string {
    return fmt.Sprintf(
        "non-exhaustive match on %s: missing cases for %v",
        e.EnumName,
        e.Missing,
    )
}
```

### 6.2 Error Messages

**User-Friendly Errors:**
```
error: non-exhaustive match on HttpResponse: missing cases for [ServerError, Redirect]
  --> example.dingo:12:5
   |
12 |     match response {
   |     ^^^^^^^^^^^^^^ pattern matching is missing variants
   |
help: ensure all variants are covered or add a wildcard pattern:
   |
   |     match response {
   |         Ok(body) => ...,
   |         NotFound => ...,
   |         ServerError{code, message} => ...,
   |         Redirect(url) => ...,
   |     }
   |
   | or use a wildcard:
   |
   |     match response {
   |         Ok(body) => ...,
   |         NotFound => ...,
   |         _ => ...,
   |     }
```

### 6.3 Integration Point

**Run exhaustiveness checking during type checking phase:**
```go
// pkg/plugin/builtin/sum_types.go

func (p *SumTypesPlugin) Transform(
    ctx *plugin.Context,
    node ast.Node,
) (ast.Node, error) {
    if match, ok := node.(*dingoast.MatchExpr); ok {
        // 1. Infer type of matched expression
        exprType := ctx.TypeChecker.Infer(match.Expr)

        // 2. Verify it's an enum type
        enumType, ok := exprType.(*dingoast.EnumType)
        if !ok {
            return nil, fmt.Errorf("can only match on enum types, got %T", exprType)
        }

        // 3. Check exhaustiveness BEFORE transformation
        if err := p.checker.CheckMatch(enumType, match.Arms); err != nil {
            return nil, err
        }

        // 4. Transform to switch statement
        return p.transformMatch(ctx, match, enumType)
    }

    return node, nil
}
```

---

## 7. Phase Breakdown (Updated)

### Phase 1: Core AST & Parser (Week 1: Days 1-7)

**Goal:** Parse enum declarations and match expressions

**Tasks:**
- [ ] Define `EnumDecl`, `VariantDecl` AST nodes
- [ ] Define `MatchExpr`, `MatchArm`, `Pattern` AST nodes
- [ ] Extend participle grammar for enum declarations with trailing commas
- [ ] Extend participle grammar for match expressions with `=>`
- [ ] Add `enum`, `match` keywords to lexer
- [ ] Parse unit, tuple, and struct variants
- [ ] Unit tests for parser (enums)
- [ ] Unit tests for parser (match expressions)
- [ ] Golden file: `tests/golden/sum_types/parse_enum.dingo`
- [ ] Golden file: `tests/golden/sum_types/parse_match.dingo`

**Deliverable:** Can parse full enum and match syntax into AST

**Time Estimate:** 5-7 days

**Success Criteria:**
- [ ] All variant styles parse correctly (unit, tuple, struct)
- [ ] Trailing commas accepted in variant lists
- [ ] Match expressions with `=>` parse correctly
- [ ] Wildcard patterns parse correctly
- [ ] Parser tests cover all edge cases

---

### Phase 2: Basic Transpilation (Week 2: Days 8-15)

**Goal:** Transform simple enums to Go code with helper methods

**Tasks:**
- [ ] Create `SumTypesPlugin` skeleton
- [ ] Implement tag enum generation
- [ ] Implement tagged union struct generation
- [ ] Implement constructor function generation
- [ ] **Implement helper method generation (Is* methods)**
- [ ] Register plugin with transpiler pipeline
- [ ] Unit tests for enum transformation
- [ ] Unit tests for helper method generation
- [ ] Golden file: `tests/golden/sum_types/simple_enum.dingo → simple_enum.go`
- [ ] Golden file: `tests/golden/sum_types/helpers.dingo → helpers.go`
- [ ] Integration test: Compile and run generated Go code

**Deliverable:** Can transpile `enum Status { Pending, Active }` to working Go with helpers

**Time Estimate:** 6-8 days

**Success Criteria:**
- [ ] Tag enum generates correctly
- [ ] Tagged union struct has correct layout
- [ ] Constructors work for all variant types
- [ ] Is* helper methods generated for each variant
- [ ] Generated code compiles and runs

---

### Phase 3: Match Expression Transpilation (Week 2-3: Days 12-19)

**Goal:** Implement match → switch transformation with destructuring

**Tasks:**
- [ ] Implement match expression → switch statement transformation
- [ ] Implement pattern destructuring for tuple variants
- [ ] Implement pattern destructuring for struct variants
- [ ] Handle unit variants (no destructuring)
- [ ] Handle wildcard patterns (`_`)
- [ ] Support guards (`if` clauses) - basic version
- [ ] Unit tests for match transformation
- [ ] Golden file: `tests/golden/sum_types/match_basic.dingo`
- [ ] Golden file: `tests/golden/sum_types/match_destructure.dingo`
- [ ] Golden file: `tests/golden/sum_types/match_wildcard.dingo`
- [ ] Integration test: Match as expression (return value)
- [ ] Integration test: Match as statement

**Deliverable:** Can write `match x { Ok(v) => v, Err(e) => 0 }` and get working Go

**Time Estimate:** 6-8 days

**Success Criteria:**
- [ ] Match transforms to switch on tag
- [ ] Destructuring generates correct field access
- [ ] Match expressions can be assigned to variables
- [ ] Match statements work (no return value)
- [ ] Wildcard generates default case

---

### Phase 4: Exhaustiveness Checking (Week 3: Days 20-24)

**Goal:** Compile-time verification of complete pattern coverage

**Tasks:**
- [ ] Implement `ExhaustivenessChecker` core algorithm
- [ ] Integrate with plugin pipeline (run before transformation)
- [ ] Detect missing variants
- [ ] Allow wildcard as catch-all
- [ ] Generate helpful error messages with suggestions
- [ ] Detect unreachable patterns (optional warning)
- [ ] Unit tests for exhaustiveness checker
- [ ] Test: Error on missing cases
- [ ] Test: Success with all cases covered
- [ ] Test: Success with wildcard
- [ ] Integration test: Compiler rejects incomplete matches

**Deliverable:** Compiler rejects `match` with missing cases (unless wildcard present)

**Time Estimate:** 4-5 days

**Success Criteria:**
- [ ] Missing variants trigger compile error
- [ ] Wildcard allows incomplete patterns
- [ ] Error messages are clear and actionable
- [ ] Exhaustiveness checking works with generic enums
- [ ] All exhaustiveness tests pass

---

### Phase 5: Generics Support (Week 3-4: Days 23-30)

**Goal:** Support generic enum types (Result[T, E], Option[T])

**Tasks:**
- [ ] Parse generic type parameters in enum declarations
- [ ] Transform generic enums to Go generic types
- [ ] Generate generic constructors (`Result_Ok[T, E]()`)
- [ ] Generate generic helper methods (`IsOk() bool`)
- [ ] Handle type parameter constraints (if needed)
- [ ] Type inference for generic variants
- [ ] Unit tests for generic enum parsing
- [ ] Unit tests for generic enum transformation
- [ ] Golden file: `tests/golden/sum_types/result_type.dingo`
- [ ] Golden file: `tests/golden/sum_types/option_type.dingo`
- [ ] **Implement standard prelude (Result, Option)**
- [ ] **Implement prelude auto-import mechanism**
- [ ] Integration test: Result-based error handling
- [ ] Integration test: Option for nullable values

**Deliverable:** Can define and use `Result[User, Error]` with standard prelude

**Time Estimate:** 6-8 days

**Success Criteria:**
- [ ] Generic enums parse correctly
- [ ] Generic constructors work (type parameters inferred)
- [ ] Result[T, E] and Option[T] available without imports
- [ ] Match works with generic enums
- [ ] Helper methods work with generics
- [ ] Prelude types integrate seamlessly

---

### Phase 6: Advanced Features & Polish (Week 4: Days 28-35)

**Goal:** Production-ready feature with full integration

**Tasks:**
- [ ] Implement methods on enums (`impl` blocks)
- [ ] Support interface implementation (Stringer, etc.)
- [ ] Nested pattern matching (patterns within patterns)
- [ ] Source map support for enum declarations
- [ ] Source map support for match transformations
- [ ] Optimize memory layout (future: small value optimization)
- [ ] Error message improvements (suggestions, hints)
- [ ] Documentation: Sum types guide
- [ ] Documentation: Result/Option usage examples
- [ ] Comprehensive integration tests
- [ ] Performance benchmarks (vs hand-written Go)
- [ ] Update CHANGELOG.md with feature details
- [ ] Code review and refactoring

**Deliverable:** Production-ready sum types implementation

**Time Estimate:** 5-7 days

**Success Criteria:**
- [ ] All P0 features complete
- [ ] All tests passing (unit, integration, golden)
- [ ] Documentation complete
- [ ] Benchmarks show minimal overhead
- [ ] Ready for user testing

---

## 8. Testing Strategy

### 8.1 Unit Tests

**Parser Tests** (`pkg/parser/parser_test.go`):
```go
TestParseEnum_Simple
TestParseEnum_Generic
TestParseEnum_AllVariantStyles
TestParseEnum_TrailingCommas
TestParseMatch_Basic
TestParseMatch_WithGuards
TestParseMatch_Wildcard
TestParseMatch_TrailingCommas
```

**Transform Tests** (`pkg/plugin/builtin/sum_types_test.go`):
```go
TestTransformEnum_ToTaggedUnion
TestTransformEnum_Constructors
TestTransformEnum_HelperMethods  // NEW
TestTransformMatch_ToSwitch
TestTransformMatch_Destructuring
TestTransformMatch_AsExpression  // NEW
```

**Exhaustiveness Tests** (`pkg/plugin/builtin/exhaustiveness_test.go`):
```go
TestExhaustiveness_AllCovered
TestExhaustiveness_MissingCase
TestExhaustiveness_Wildcard
TestExhaustiveness_UnreachablePattern
```

### 8.2 Integration Tests

**Golden File Tests** (`tests/golden/sum_types/`):
```
sum_types/
  parse_enum.dingo              → Tests parsing only
  parse_match.dingo             → Tests match parsing
  simple_enum.dingo             → simple_enum.go
  helpers.dingo                 → helpers.go (Is* methods)
  generic_enum.dingo            → generic_enum.go
  match_basic.dingo             → match_basic.go
  match_destructure.dingo       → match_destructure.go
  match_wildcard.dingo          → match_wildcard.go
  result_type.dingo             → result_type.go
  option_type.dingo             → option_type.go
  nested_match.dingo            → nested_match.go
  interface_impl.dingo          → interface_impl.go
```

**End-to-End Tests** (`tests/integration/sum_types_test.go`):
```go
func TestSumTypes_ResultType(t *testing.T)
func TestSumTypes_OptionType(t *testing.T)
func TestSumTypes_MatchExpression(t *testing.T)
func TestSumTypes_Exhaustiveness(t *testing.T)
func TestSumTypes_HelperMethods(t *testing.T)
func TestSumTypes_Prelude(t *testing.T)
```

### 8.3 Benchmark Tests

```go
func BenchmarkSumType_Construction(b *testing.B)
func BenchmarkSumType_PatternMatch(b *testing.B)
func BenchmarkSumType_HelperMethod(b *testing.B)
```

---

## 9. Success Criteria

**Must Have (P0) - All Required:**
- [x] Parse enum declarations with all variant styles *(Decision: use enum keyword)*
- [x] Parse match expressions with `=>` separator *(Decision: use => over :)*
- [x] Allow trailing commas in variants *(Decision: follow Go style)*
- [x] Transform enums to idiomatic Go tagged unions
- [x] Generate helper methods (Is*) for all variants *(Decision: auto-generate)*
- [x] Pattern matching with destructuring works
- [x] Match works as expression (can return values) *(Decision: expression-based)*
- [x] Exhaustiveness checking catches missing cases *(Decision: error on missing)*
- [x] Wildcard `_` pattern allowed as catch-all *(Decision: allow escape hatch)*
- [x] Generic enums (Result[T,E], Option[T]) work
- [x] Standard prelude with Result/Option *(Decision: auto-import)*
- [x] Zero runtime overhead (no reflection, minimal allocations)
- [x] All tests pass (unit, integration, golden)

**Should Have (P1):**
- [ ] Helpful error messages with suggestions
- [ ] Source map support for debugging
- [ ] Interface implementation on enums
- [ ] Methods on enum types (impl blocks)
- [ ] Guard clauses in patterns

**Could Have (P2):**
- [ ] Small value optimization for <= 16 byte variants
- [ ] Unreachable case detection (warnings)
- [ ] Pattern guard optimization
- [ ] Derive common traits (Debug, Eq, Hash)

---

## 10. Risk Mitigation

**High Risk Areas:**

1. **Exhaustiveness Algorithm**
   - Risk: Complex edge cases, false positives/negatives
   - Mitigation: Study Rust/Swift implementations, extensive test coverage, start simple

2. **Match as Expression**
   - Risk: Type inference for multi-arm matches
   - Mitigation: Leverage existing type checker, ensure all arms unify to same type

3. **Prelude Auto-Import**
   - Risk: Conflicts with user-defined types
   - Mitigation: Clear scoping rules, allow shadowing with warning

**Medium Risk Areas:**

1. **Helper Method Generation**
   - Risk: Name collisions with user methods
   - Mitigation: Reserve Is* pattern, document convention

2. **Generic Type Inference**
   - Risk: Type parameters in patterns
   - Mitigation: Use Go's generic type system, explicit annotations if needed

---

## 11. Implementation Checklist

**Pre-Implementation:**
- [x] User design decisions finalized
- [ ] Feature branch created: `feature/sum-types`
- [ ] Tracking issue created with task list

**Phase 1 (Week 1):**
- [ ] AST nodes defined
- [ ] Parser grammar extended
- [ ] Parser tests passing
- [ ] Golden files created

**Phase 2 (Week 2):**
- [ ] Plugin skeleton created
- [ ] Transformation logic implemented
- [ ] Helper methods generated
- [ ] Transformation tests passing

**Phase 3 (Week 2-3):**
- [ ] Match transformation implemented
- [ ] Destructuring working
- [ ] Expression semantics correct
- [ ] Match tests passing

**Phase 4 (Week 3):**
- [ ] Exhaustiveness checker implemented
- [ ] Integration with pipeline complete
- [ ] Error messages polished
- [ ] Exhaustiveness tests passing

**Phase 5 (Week 3-4):**
- [ ] Generic support added
- [ ] Prelude defined
- [ ] Auto-import working
- [ ] Integration tests passing

**Phase 6 (Week 4):**
- [ ] Advanced features complete
- [ ] Documentation written
- [ ] Benchmarks run
- [ ] CHANGELOG updated
- [ ] Ready for merge

---

## 12. Documentation Plan

### 12.1 User Documentation

**Guide: Sum Types in Dingo** (`docs/features/sum-types.md`):
- Introduction and motivation
- Enum declaration syntax
- Pattern matching with match
- Result and Option types
- Helper methods (Is*)
- Best practices
- Common patterns

**Guide: Error Handling with Result** (`docs/guides/error-handling.md`):
- Result[T, E] usage
- Integration with `?` operator
- Converting from Go errors
- Best practices

### 12.2 API Documentation

**Prelude Types** (`docs/api/prelude.md`):
- Result[T, E] API
- Option[T] API
- Auto-import behavior

---

## 13. Timeline Summary

**Total Duration:** 3-4 weeks (20-28 working days)

| Phase | Duration | Deliverable |
|-------|----------|-------------|
| 1. Parser | 5-7 days | Parse enums and matches |
| 2. Basic Transpilation | 6-8 days | Generate Go code with helpers |
| 3. Match Expressions | 6-8 days | Match → switch transformation |
| 4. Exhaustiveness | 4-5 days | Compile-time checking |
| 5. Generics & Prelude | 6-8 days | Result/Option auto-imported |
| 6. Polish | 5-7 days | Production-ready |

**Critical Path:** Parser → Transpilation → Match → Exhaustiveness → Generics

---

## 14. Next Steps

**Immediate Actions:**
1. Create feature branch: `git checkout -b feature/sum-types`
2. Create tracking issue with this plan's checklist
3. Begin Phase 1: Parser implementation
4. Set up golden file test structure

**First Pull Request (Week 1):**
- Parser + AST nodes
- Parser unit tests
- Initial golden files
- ~500-700 lines of code

**Communication:**
- Update CHANGELOG.md as features complete
- Regular progress updates in tracking issue
- Demo working features at each phase completion

---

## 15. References

**Design Documents:**
- Feature Spec: `/Users/jack/mag/dingo/features/sum-types.md`
- Initial Plan: `/Users/jack/mag/dingo/ai-docs/sessions/20251116-202224/01-planning/initial-plan.md`
- User Decisions: `/Users/jack/mag/dingo/ai-docs/sessions/20251116-202224/01-planning/clarifications.md`

**External Resources:**
- Rust Enums: https://doc.rust-lang.org/book/ch06-00-enums.html
- Swift Enums: https://docs.swift.org/swift-book/documentation/the-swift-programming-language/enumerations/
- TypeScript Discriminated Unions: https://www.typescriptlang.org/docs/handbook/unions-and-intersections.html
- Go Proposal #19412: Sum types (996+ 👍)

**Internal References:**
- Plugin System: `/Users/jack/mag/dingo/PLUGIN_SYSTEM_DESIGN.md`
- Error Propagation Plugin: `/Users/jack/mag/dingo/pkg/plugin/builtin/error_propagation.go`
- Pattern Matching Spec: `/Users/jack/mag/dingo/features/pattern-matching.md`

---

**Status:** Final Plan - Ready for Implementation
**Total Estimated LOC:** ~2,500-3,500 production code
**Risk Level:** High (foundational feature)
**Impact:** Critical (enables core Dingo value proposition)
