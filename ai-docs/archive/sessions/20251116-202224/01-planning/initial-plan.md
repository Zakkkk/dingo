# Sum Types Implementation Plan - Dingo Project

**Session:** 20251116-202224
**Date:** 2025-11-16
**Feature:** Sum Types (Discriminated Unions)
**Priority:** P0 - Critical Foundation
**Estimated Duration:** 3-4 weeks

---

## Executive Summary

Sum types are the foundational type system feature that enables Result[T, E] and Option[T] - the core value proposition of Dingo. This document outlines a complete architecture for implementing sum types that:

1. Transpiles to zero-overhead, idiomatic Go code (tagged unions)
2. Integrates seamlessly with the existing plugin architecture
3. Provides exhaustiveness checking for pattern matching
4. Supports generic type parameters (Result[T, E], Option[T])
5. Maintains full Go interoperability

---

## 1. High-Level Architecture

### 1.1 Component Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                    Sum Types Feature Pipeline                    │
├─────────────────────────────────────────────────────────────────┤
│                                                                   │
│  Parser                Plugin System            Generator         │
│  ┌──────┐            ┌────────────────┐        ┌──────────┐     │
│  │Enum  │──AST──────>│ Sum Type       │─Go AST>│ Tagged   │     │
│  │Decl  │            │ Transform      │        │ Union    │     │
│  └──────┘            │ Plugin         │        │ Code     │     │
│      │               └────────────────┘        └──────────┘     │
│      │                       │                                   │
│      │               ┌────────────────┐                          │
│      └──────────────>│ Exhaustiveness │                          │
│                      │ Checker        │                          │
│                      └────────────────┘                          │
│                                                                   │
└─────────────────────────────────────────────────────────────────┘
```

### 1.2 Workflow

1. **Parse Phase**: Participle parser recognizes `enum Name { ... }` declarations
2. **AST Phase**: Create custom `EnumDecl` and `VariantDecl` AST nodes
3. **Transform Phase**: Sum type plugin converts enum to Go structs + constructors
4. **Validation Phase**: Exhaustiveness checker validates match expressions
5. **Generate Phase**: Standard go/printer outputs idiomatic Go code

### 1.3 Design Principles

- **Zero Runtime Overhead**: Pure compile-time feature, no runtime library
- **Type Safety**: All variants type-checked at compile time
- **Exhaustiveness**: Pattern matches must handle all variants
- **Go Idioms**: Generated code looks hand-written
- **Generics Support**: Work with Go 1.18+ generic types

---

## 2. AST Node Design

### 2.1 Core AST Nodes (pkg/ast/ast.go additions)

```go
// EnumDecl represents a sum type declaration
// Example: enum Result[T, E] { Ok(T), Err(E) }
type EnumDecl struct {
    Enum      token.Pos         // Position of 'enum' keyword
    Name      *ast.Ident        // Enum name
    TypeParams *ast.FieldList   // Generic type parameters (nil if not generic)
    Lbrace    token.Pos         // Position of '{'
    Variants  []*VariantDecl    // List of variants
    Rbrace    token.Pos         // Position of '}'
}

func (e *EnumDecl) Pos() token.Pos { return e.Enum }
func (e *EnumDecl) End() token.Pos { return e.Rbrace + 1 }

// VariantDecl represents a single variant in a sum type
// Examples:
//   Ok(T)                    - Tuple-style (unnamed field)
//   NotFound                 - Unit variant (no data)
//   ServerError{code: int}   - Struct-style (named fields)
type VariantDecl struct {
    Name   *ast.Ident        // Variant name
    Style  VariantStyle      // Tuple, Struct, or Unit
    Fields *ast.FieldList    // Fields (nil for Unit variants)

    // Position tracking
    NamePos token.Pos
    Lparen  token.Pos        // '(' or '{' position
    Rparen  token.Pos        // ')' or '}' position
}

type VariantStyle int

const (
    VariantUnit   VariantStyle = iota // NotFound
    VariantTuple                       // Ok(T)
    VariantStruct                      // Error{code: int}
)

// MatchExpr represents pattern matching
// Example: match result { Ok(x) => x, Err(e) => 0 }
type MatchExpr struct {
    Match   token.Pos         // Position of 'match' keyword
    Expr    ast.Expr          // Expression being matched
    Lbrace  token.Pos         // Position of '{'
    Arms    []*MatchArm       // Match arms
    Rbrace  token.Pos         // Position of '}'
}

type MatchArm struct {
    Pattern  *Pattern          // Pattern to match
    Guard    ast.Expr          // Optional guard (if clause)
    Arrow    token.Pos         // Position of '=>'
    Body     ast.Expr          // Expression or block
}

// Pattern represents a match pattern
type Pattern struct {
    Variant  *ast.Ident       // Variant name (Ok, Err, etc.)
    Fields   []*FieldPattern  // Destructured fields
    Wildcard bool             // true for '_' pattern
}

type FieldPattern struct {
    Name  *ast.Ident          // Field name (for struct variants)
    Bind  *ast.Ident          // Binding name (variable to bind to)
    Pos   token.Pos
}
```

### 2.2 Type Representation

```go
// EnumType represents a sum type in the type system
// This is used during type checking, not in the AST
type EnumType struct {
    Name      string
    Variants  []VariantInfo
    TypeParams []*TypeParam
    Package   string
}

type VariantInfo struct {
    Name   string
    Fields []FieldInfo
    Style  VariantStyle
}

type FieldInfo struct {
    Name string         // Empty for tuple-style
    Type types.Type     // Go type
}
```

---

## 3. Parser Changes

### 3.1 Grammar Extensions (pkg/parser/participle.go)

```go
// Add to DingoFile
type DingoFile struct {
    Package string         `parser:"'package' @Ident"`
    Imports []*Import      `parser:"@@*"`
    Decls   []*Declaration `parser:"@@*"`
}

// Extend Declaration to include enums
type Declaration struct {
    Func *Function `parser:"@@"`
    Var  *Variable `parser:"| @@"`
    Enum *EnumDeclaration `parser:"| @@"`  // NEW
}

// EnumDeclaration grammar
type EnumDeclaration struct {
    Name       string             `parser:"'enum' @Ident"`
    TypeParams *TypeParameters    `parser:"( '<' @@ '>' )?"`  // Generic params
    Variants   []*VariantDef      `parser:"'{' ( @@ ( ',' @@ )* ','? )? '}'"`
}

type TypeParameters struct {
    Params []string `parser:"@Ident ( ',' @Ident )*"`
}

type VariantDef struct {
    Name        string        `parser:"@Ident"`
    TupleFields []*Type       `parser:"( '(' ( @@ ( ',' @@ )* )? ')' )"`       // Tuple style
    StructFields []*NamedField `parser:"| ( '{' ( @@ ( ',' @@ )* )? '}' )"`  // Struct style
    // If neither, it's a unit variant
}

type NamedField struct {
    Name string `parser:"@Ident"`
    Type *Type  `parser:"':' @@"`
}
```

### 3.2 Match Expression Grammar

```go
// Add to Expression alternatives
type Expression struct {
    Match      *MatchExpression      `parser:"@@"`
    Comparison *ComparisonExpression `parser:"| @@"`
    // ... existing alternatives
}

type MatchExpression struct {
    Expr  *Expression  `parser:"'match' @@"`
    Arms  []*MatchArm  `parser:"'{' ( @@ )+ '}'"`
}

type MatchArm struct {
    Pattern *PatternExpr `parser:"@@"`
    Guard   *Expression  `parser:"( 'if' @@ )?"`
    Body    *Expression  `parser:"'=>' @@"`
}

type PatternExpr struct {
    Wildcard       bool              `parser:"@'_'"`
    VariantPattern *VariantPattern   `parser:"| @@"`
    LiteralPattern *PrimaryExpression `parser:"| @@"`
}

type VariantPattern struct {
    Variant      string            `parser:"@Ident"`
    TupleBindings []*string         `parser:"( '(' ( @Ident ( ',' @Ident )* )? ')' )"`
    StructBindings []*FieldBinding   `parser:"| ( '{' ( @@ ( ',' @@ )* )? '}' )"`
}

type FieldBinding struct {
    Field string  `parser:"@Ident"`
    Bind  *string `parser:"( ':' @Ident )?"`  // Optional rename
}
```

### 3.3 Lexer Extensions

No new tokens needed! All syntax uses existing tokens:
- `enum` - keyword (add to reserved words)
- `match` - keyword (add to reserved words)
- `{`, `}`, `(`, `)`, `,`, `=>` - already supported

---

## 4. Transpilation Strategy

### 4.1 Memory Layout Design

**Goal**: Minimal overhead, type-safe access, zero-cost abstraction

**Strategy**: Tagged union with discriminated access

```dingo
// Dingo source
enum Shape {
    Circle { radius: float64 },
    Rectangle { width: float64, height: float64 },
    Point
}
```

**Transpiles to:**

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
    tag             ShapeTag
    circle_radius   *float64
    rectangle_width *float64
    rectangle_height *float64
    // Point has no fields
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
```

### 4.2 Optimization Strategies

**Small Value Optimization** (future enhancement):

For variants with only value types fitting in 16 bytes:

```go
type SmallEnum struct {
    tag   uint8
    data  [16]byte  // Instead of pointers
}
```

**Pointer Optimization** (for single-pointer variants):

```go
// For enum Option[T] { Some(T), None }
type Option_string struct {
    value *string  // nil = None, non-nil = Some
}
```

### 4.3 Generic Sum Types

```dingo
enum Result[T, E] {
    Ok(T),
    Err(E)
}
```

**Transpiles to** (using Go generics):

```go
type ResultTag uint8

const (
    ResultTag_Ok ResultTag = iota
    ResultTag_Err
)

type Result[T any, E any] struct {
    tag Tag
    ok  *T
    err *E
}

func Result_Ok[T any, E any](value T) Result[T, E] {
    return Result[T, E]{
        tag: ResultTag_Ok,
        ok:  &value,
    }
}

func Result_Err[T any, E any](err E) Result[T, E] {
    return Result[T, E]{
        tag: ResultTag_Err,
        err: &err,
    }
}
```

---

## 5. Pattern Matching Integration

### 5.1 Match Expression Transpilation

```dingo
// Dingo source
func area(shape: Shape) -> float64 {
    match shape {
        Circle{radius} => 3.14 * radius * radius,
        Rectangle{width, height} => width * height,
        Point => 0.0
    }
}
```

**Transpiles to:**

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

### 5.2 Exhaustiveness Checking Algorithm

**Core Algorithm**: Static analysis during type checking

```go
// Exhaustiveness checking logic
type ExhaustivenessChecker struct {
    enumVariants map[string][]string  // enum name -> variant names
}

func (c *ExhaustivenessChecker) CheckMatch(
    enumType *EnumType,
    patterns []*Pattern,
) error {
    covered := make(map[string]bool)
    hasWildcard := false

    for _, pattern := range patterns {
        if pattern.Wildcard {
            hasWildcard = true
            break
        }
        covered[pattern.Variant.Name] = true
    }

    if hasWildcard {
        return nil  // Wildcard covers all cases
    }

    // Check if all variants are covered
    missing := []string{}
    for _, variant := range enumType.Variants {
        if !covered[variant.Name] {
            missing = append(missing, variant.Name)
        }
    }

    if len(missing) > 0 {
        return fmt.Errorf(
            "non-exhaustive match: missing cases for %v",
            missing,
        )
    }

    return nil
}
```

**Integration Point**: Run during type checking phase, before code generation

---

## 6. Plugin Architecture Integration

### 6.1 SumTypesPlugin Structure

```go
// pkg/plugin/builtin/sum_types.go
package builtin

import (
    "go/ast"
    dingoast "github.com/MadAppGang/dingo/pkg/ast"
    "github.com/MadAppGang/dingo/pkg/plugin"
)

type SumTypesPlugin struct {
    plugin.BasePlugin

    // Type registry for exhaustiveness checking
    enumTypes map[string]*dingoast.EnumType

    // Code generation state
    generatedTypes map[string]bool
}

func NewSumTypesPlugin() *SumTypesPlugin {
    return &SumTypesPlugin{
        BasePlugin: *plugin.NewBasePlugin(
            "sum_types",
            "Sum types (enum) transformation",
            nil,  // No dependencies
        ),
        enumTypes:      make(map[string]*dingoast.EnumType),
        generatedTypes: make(map[string]bool),
    }
}

func (p *SumTypesPlugin) Transform(
    ctx *plugin.Context,
    node ast.Node,
) (ast.Node, error) {
    // Multi-pass transformation:
    // Pass 1: Collect enum declarations
    // Pass 2: Transform enum decls to structs + constructors
    // Pass 3: Transform match expressions to switches
    // Pass 4: Validate exhaustiveness

    switch n := node.(type) {
    case *ast.File:
        return p.transformFile(ctx, n)
    case *dingoast.EnumDecl:
        return p.transformEnum(ctx, n)
    case *dingoast.MatchExpr:
        return p.transformMatch(ctx, n)
    }

    return node, nil
}
```

### 6.2 Multi-Pass Strategy

**Pass 1: Discovery**
- Collect all `EnumDecl` nodes
- Build type registry
- Validate no duplicate names

**Pass 2: Enum Transformation**
- Generate tag enum
- Generate tagged union struct
- Generate constructor functions
- Replace EnumDecl with Go declarations

**Pass 3: Match Transformation**
- Convert match expressions to switch statements
- Destructure variant fields
- Insert nil checks and dereferencing

**Pass 4: Exhaustiveness Validation**
- Check all match expressions
- Report missing cases as compile errors

---

## 7. Type System Integration

### 7.1 Type Checker Requirements

Need to integrate with Go's type system:

```go
// Type inference for enum variants
func (tc *TypeChecker) inferEnumVariant(
    enum *EnumType,
    variant string,
) (types.Type, error) {
    for _, v := range enum.Variants {
        if v.Name == variant {
            return tc.constructVariantType(enum, v)
        }
    }
    return nil, fmt.Errorf("unknown variant: %s", variant)
}

// Match expression type checking
func (tc *TypeChecker) checkMatch(
    expr *MatchExpr,
) (types.Type, error) {
    // 1. Infer type of matched expression
    exprType := tc.infer(expr.Expr)

    // 2. Verify it's an enum type
    enumType, ok := exprType.(*EnumType)
    if !ok {
        return nil, fmt.Errorf("can only match on enum types")
    }

    // 3. Check exhaustiveness
    if err := tc.checkExhaustiveness(enumType, expr.Arms); err != nil {
        return nil, err
    }

    // 4. Verify all arms return compatible types
    armTypes := []types.Type{}
    for _, arm := range expr.Arms {
        armType := tc.infer(arm.Body)
        armTypes = append(armTypes, armType)
    }

    return tc.unifyTypes(armTypes)
}
```

### 7.2 Go Interoperability

Sum types can implement Go interfaces:

```dingo
enum Status {
    Pending,
    Active { since: time.Time },
    Completed { result: string }
}

impl Status: fmt.Stringer {
    func String() -> string {
        match self {
            Pending => "pending",
            Active{since} => "active since ${since}",
            Completed{result} => "completed: ${result}"
        }
    }
}
```

**Transpiles to:**

```go
func (s Status) String() string {
    switch s.tag {
    case StatusTag_Pending:
        return "pending"
    case StatusTag_Active:
        since := *s.active_since
        return fmt.Sprintf("active since %v", since)
    case StatusTag_Completed:
        result := *s.completed_result
        return fmt.Sprintf("completed: %s", result)
    default:
        panic("unreachable")
    }
}
```

---

## 8. Testing Strategy

### 8.1 Unit Tests

**Parser Tests** (`pkg/parser/parser_test.go`):
- Parse simple enums
- Parse generic enums
- Parse all variant styles (unit, tuple, struct)
- Parse match expressions
- Error cases (invalid syntax)

**Transform Tests** (`pkg/plugin/builtin/sum_types_test.go`):
- Transform enum to struct + constructors
- Transform match to switch
- Generic enum instantiation
- Nested enums

**Exhaustiveness Tests**:
- All cases covered → pass
- Missing cases → error
- Wildcard pattern → pass
- Unreachable cases → warning

### 8.2 Integration Tests

**Golden File Tests** (`tests/golden/sum_types/`):
```
sum_types/
  simple_enum.dingo          → simple_enum.go
  generic_enum.dingo         → generic_enum.go
  nested_match.dingo         → nested_match.go
  result_type.dingo          → result_type.go
  option_type.dingo          → option_type.go
```

**End-to-End Tests**:
```go
// tests/integration_test.go
func TestSumTypes_ResultType(t *testing.T) {
    source := `
        package main

        enum Result[T, E] {
            Ok(T),
            Err(E)
        }

        func divide(a: int, b: int) -> Result[int, string] {
            if b == 0 {
                return Err("division by zero")
            }
            return Ok(a / b)
        }

        func main() {
            let result = divide(10, 2)
            match result {
                Ok(value) => println("Result: ${value}"),
                Err(msg) => println("Error: ${msg}")
            }
        }
    `

    // Compile and run
    output := compileAndRun(t, source)
    require.Contains(t, output, "Result: 5")
}
```

### 8.3 Benchmark Tests

```go
// Memory overhead benchmarks
func BenchmarkSumType_Construction(b *testing.B) {
    // Measure allocation overhead vs hand-written tagged union
}

func BenchmarkSumType_PatternMatch(b *testing.B) {
    // Measure switch performance vs interface type assertion
}
```

---

## 9. Phase Breakdown

### Phase 1: Core AST & Parser (Week 1)

**Goal**: Parse enum declarations and represent them in AST

**Tasks**:
- [ ] Define `EnumDecl`, `VariantDecl` AST nodes
- [ ] Extend participle grammar for enum declarations
- [ ] Add enum keyword to lexer
- [ ] Parse unit, tuple, and struct variants
- [ ] Unit tests for parser
- [ ] Golden file: simple enum parsing

**Deliverable**: Can parse `enum Shape { Circle, Square }` into AST

**Time Estimate**: 5-7 days

---

### Phase 2: Basic Transpilation (Week 2)

**Goal**: Transform simple enums to Go code (no generics, no match yet)

**Tasks**:
- [ ] Create `SumTypesPlugin` skeleton
- [ ] Implement tag enum generation
- [ ] Implement tagged union struct generation
- [ ] Implement constructor function generation
- [ ] Register plugin with generator
- [ ] Unit tests for transformation
- [ ] Golden file: simple enum → Go code
- [ ] Test: compile and run generated Go

**Deliverable**: Can transpile `enum Status { Pending, Active }` to working Go

**Time Estimate**: 6-8 days

---

### Phase 3: Pattern Matching (Week 2-3)

**Goal**: Implement match expressions with destructuring

**Tasks**:
- [ ] Define `MatchExpr`, `MatchArm`, `Pattern` AST nodes
- [ ] Extend parser for match expressions
- [ ] Add match keyword to lexer
- [ ] Implement match → switch transformation
- [ ] Implement pattern destructuring
- [ ] Handle guards (`if` clauses)
- [ ] Unit tests for match transformation
- [ ] Golden file: match expressions
- [ ] Test: exhaustive vs non-exhaustive matches

**Deliverable**: Can write `match x { Ok(v) => v, Err(e) => 0 }`

**Time Estimate**: 6-8 days

---

### Phase 4: Exhaustiveness Checking (Week 3)

**Goal**: Compile-time verification of complete pattern coverage

**Tasks**:
- [ ] Implement `ExhaustivenessChecker`
- [ ] Integrate with plugin pipeline
- [ ] Detect missing cases
- [ ] Detect unreachable cases (optional warning)
- [ ] Error messages with suggestions
- [ ] Unit tests for exhaustiveness
- [ ] Test: compile errors for incomplete matches

**Deliverable**: Compiler rejects `match` with missing cases

**Time Estimate**: 4-5 days

---

### Phase 5: Generics Support (Week 3-4)

**Goal**: Support generic enum types (Result[T, E], Option[T])

**Tasks**:
- [ ] Parse generic type parameters in enum declarations
- [ ] Transform generic enums to Go generic types
- [ ] Handle type parameter constraints
- [ ] Instantiate generic constructors
- [ ] Type inference for generic variants
- [ ] Unit tests for generic enums
- [ ] Golden file: Result[T, E] and Option[T]
- [ ] Integration test: Result-based error handling

**Deliverable**: Can define and use `Result[User, Error]`

**Time Estimate**: 6-8 days

---

### Phase 6: Advanced Features & Polish (Week 4)

**Goal**: Production-ready feature with full integration

**Tasks**:
- [ ] Implement `impl` blocks for enums (methods)
- [ ] Interface implementation (Stringer, etc.)
- [ ] Nested pattern matching
- [ ] Optimize memory layout (small value optimization)
- [ ] Source map support for enum declarations
- [ ] Error message improvements
- [ ] Documentation and examples
- [ ] Comprehensive integration tests
- [ ] Performance benchmarks
- [ ] Update CHANGELOG.md

**Deliverable**: Production-ready sum types implementation

**Time Estimate**: 5-7 days

---

## 10. Gaps and Open Questions

See `gaps.json` for detailed questions requiring user input.

**Key Technical Decisions**:

1. **Memory Layout**: Always use pointers, or optimize for small values?
2. **Variant Naming**: Prefix with enum name (`ShapeTag_Circle`) or not?
3. **Exhaustiveness**: Error or warning for missing cases?
4. **Wildcard**: Allow `_` to catch all, or require explicit default?
5. **Generics**: Use Go 1.18+ generics, or custom monomorphization?

**Syntax Ambiguities**:

1. **Trailing Commas**: Allow `enum X { A, B, }` or enforce `enum X { A, B }`?
2. **Match Syntax**: `=>` vs `:` vs `->` for arm separator?
3. **Pattern Guards**: `if` or `when` keyword?
4. **Destructuring**: `Some(x)` vs `Some{0: x}` for tuple variants?

**Integration Concerns**:

1. How to handle Result/Option stdlib (define in Dingo prelude)?
2. Should we generate `Is*()` helper methods (`result.IsOk()`)?
3. How to handle recursive enum types (e.g., AST nodes)?
4. Source map generation for multi-statement transformations?

---

## 11. Success Criteria

**Must Have (P0)**:
- [ ] Parse enum declarations with all variant styles
- [ ] Transform enums to idiomatic Go code
- [ ] Pattern matching with destructuring works
- [ ] Exhaustiveness checking catches missing cases
- [ ] Generic enums (Result[T,E], Option[T]) work
- [ ] Zero runtime overhead (no reflection, no allocations)
- [ ] All tests pass (unit, integration, golden)

**Should Have (P1)**:
- [ ] Small value optimization for <= 16 byte variants
- [ ] Helpful error messages with suggestions
- [ ] Source map support for debugging
- [ ] Interface implementation on enums
- [ ] Methods on enum types (impl blocks)

**Could Have (P2)**:
- [ ] Unreachable case detection (warnings)
- [ ] Pattern guard optimization
- [ ] Derive common traits (Debug, Eq, Hash)
- [ ] Prettier generated code (comments, formatting)

---

## 12. Risk Mitigation

**High Risk Areas**:

1. **Exhaustiveness Algorithm**: Complex, many edge cases
   - **Mitigation**: Study Rust/Swift implementations, extensive test suite

2. **Generic Type Inference**: Type parameters in patterns
   - **Mitigation**: Leverage Go's existing generic type system

3. **Memory Safety**: Pointer dereferencing in generated code
   - **Mitigation**: Always check tag before dereferencing, panic on mismatch

4. **Performance**: Overhead from pointers and tag checks
   - **Mitigation**: Benchmark against hand-written code, optimize layout

**Medium Risk Areas**:

1. **Parser Complexity**: Nested patterns, multiple syntaxes
   - **Mitigation**: Incremental parser development, test each variant style

2. **Plugin Interaction**: Dependencies with pattern matching plugin
   - **Mitigation**: Well-defined plugin interface, topological ordering

3. **Go Interop**: Exposing enums to Go code
   - **Mitigation**: Generate public constructors, document usage patterns

---

## 13. Future Enhancements (Post-MVP)

**Phase 2 Improvements**:
- Derive macros (auto-implement common interfaces)
- Pattern matching on other types (slices, maps, literals)
- Match expression optimization (compile to jump tables)
- Better error recovery (suggest closest variant name)

**Advanced Features**:
- Associated constants on variants
- Variant visibility (public/private variants)
- Sealed trait pattern (enum implementing interface)
- Zero-size optimization for unit variants

---

## 14. References

**External Inspiration**:
- Rust Enums: https://doc.rust-lang.org/book/ch06-00-enums.html
- Swift Enums: https://docs.swift.org/swift-book/documentation/the-swift-programming-language/enumerations/
- TypeScript Discriminated Unions: https://www.typescriptlang.org/docs/handbook/unions-and-intersections.html
- Go Proposal #19412: Sum types (996+ upvotes)

**Internal References**:
- Feature Spec: `/Users/jack/mag/dingo/features/sum-types.md`
- Pattern Matching Spec: `/Users/jack/mag/dingo/features/pattern-matching.md`
- Plugin System: `/Users/jack/mag/dingo/PLUGIN_SYSTEM_DESIGN.md`
- Error Propagation Plugin: `/Users/jack/mag/dingo/pkg/plugin/builtin/error_propagation.go`

---

## 15. Next Steps

**Immediate Actions**:

1. Review this plan with user, get feedback on gaps
2. Finalize syntax decisions (see gaps.json)
3. Set up feature branch: `feature/sum-types`
4. Begin Phase 1: Parser implementation
5. Create tracking issue with task checklist

**First Pull Request** (Week 1):
- Parser + AST nodes
- Unit tests
- Documentation
- ~500-700 lines of code

---

**Total Estimated Duration**: 3-4 weeks (20-28 days)
**Total Estimated LOC**: ~2,500-3,500 production code
**Risk Level**: High (foundational feature)
**Impact**: Critical (enables Result, Option, pattern matching)
