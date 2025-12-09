# Sum Types Implementation - Test Plan

**Session:** 20251116-202224
**Date:** 2025-11-16
**Feature:** Sum Types (Phase 1-2)
**Testing Phase:** Comprehensive Unit + Integration Tests

---

## 1. Requirements Understanding

### What the Feature Should Do

**Core Capability:** Transform Dingo enum declarations into idiomatic Go tagged unions with full type safety.

**Key Behaviors to Validate:**

1. **Enum Declaration Parsing**
   - Parse unit variants (e.g., `Pending`)
   - Parse tuple variants (e.g., `Circle(radius: float64)`)
   - Parse struct variants (e.g., `Rectangle { width: float64, height: float64 }`)
   - Handle generic type parameters (e.g., `Result[T, E]`)
   - Accept trailing commas in variant lists

2. **Enum to Go Transformation**
   - Generate tag enum with iota constants
   - Generate tagged union struct with pointer fields
   - Generate constructor functions for each variant
   - Generate Is* helper methods for variant checking
   - Preserve type parameters for generic enums

3. **Match Expression Parsing**
   - Parse match expressions with `=>` separator
   - Parse wildcard patterns (`_`)
   - Parse variant patterns with destructuring
   - Handle trailing commas in match arms

4. **Error Handling**
   - Detect duplicate enum names
   - Detect duplicate variant names within an enum
   - Validate nil safety (no nil pointer dereferences)
   - Provide helpful error messages

### Critical Edge Cases

1. **Empty/Minimal Cases:**
   - Single variant enum
   - Enum with no data (all unit variants)
   - Match with only wildcard pattern

2. **Boundary Conditions:**
   - Large number of variants (stress test)
   - Deeply nested struct types in variants
   - Multiple type parameters

3. **Error Conditions:**
   - Duplicate enum names in same file
   - Duplicate variant names in same enum
   - Nil fields in variant declarations
   - Invalid pattern types

4. **Integration Points:**
   - Enums registered in plugin registry
   - Generated code compiles with Go compiler
   - DINGO:GENERATED markers present
   - Source positions preserved

---

## 2. Test Scenarios

### Unit Tests - Enum Registry (`pkg/plugin/builtin/sum_types_test.go`)

#### Scenario: Add and Retrieve Enums
- **Purpose:** Validate enum registration mechanism
- **Input:** Multiple enum declarations in AST
- **Expected:** All enums stored in registry, retrievable by name
- **Rationale:** Foundation for all enum transformations

#### Scenario: Detect Duplicate Enum Names
- **Purpose:** Prevent conflicting type definitions
- **Input:** Two enums with same name in one file
- **Expected:** Error with descriptive message and position
- **Rationale:** Type safety - Go compiler would reject duplicate types

#### Scenario: Detect Duplicate Variant Names
- **Purpose:** Prevent conflicting constructors/tags
- **Input:** Enum with two variants named "Circle"
- **Expected:** Error with variant and enum name
- **Rationale:** Each variant must have unique constructor

#### Scenario: Nil Safety in Variant Fields
- **Purpose:** Prevent nil pointer dereferences
- **Input:** Variant with nil Fields pointer
- **Expected:** No crash, treated as unit variant
- **Rationale:** Parser may produce nil for unit variants

### Unit Tests - Tag Enum Generation

#### Scenario: Simple Tag Enum (Unit Variants)
- **Purpose:** Validate basic tag generation
- **Input:** `enum Status { Pending, Approved, Rejected }`
- **Expected Output:**
  ```go
  type StatusTag uint8
  const (
      StatusTag_Pending StatusTag = iota
      StatusTag_Approved
      StatusTag_Rejected
  )
  ```
- **Rationale:** Foundation for all variant checking

#### Scenario: Generic Tag Enum
- **Purpose:** Ensure type parameters don't affect tag generation
- **Input:** `enum Result[T, E] { Ok(T), Err(E) }`
- **Expected:** Non-generic tag enum (tags don't depend on T or E)
- **Rationale:** Tag is discriminator, not type-parameterized

### Unit Tests - Union Struct Generation

#### Scenario: Struct with All Variant Types
- **Purpose:** Validate field generation for mixed variants
- **Input:**
  ```dingo
  enum Shape {
      Point,
      Circle { radius: float64 },
      Rectangle { width: float64, height: float64 }
  }
  ```
- **Expected:**
  ```go
  type Shape struct {
      tag              ShapeTag
      circle_radius    *float64
      rectangle_width  *float64
      rectangle_height *float64
  }
  ```
- **Rationale:** Covers all variant kinds in one test

#### Scenario: Generic Union Struct
- **Purpose:** Validate generic type parameter handling
- **Input:** `enum Option[T] { Some(T), None }`
- **Expected:**
  ```go
  type Option[T any] struct {
      tag   OptionTag
      some  *T
  }
  ```
- **Rationale:** Tests Go 1.18+ generics integration

### Unit Tests - Constructor Generation

#### Scenario: Unit Variant Constructor
- **Purpose:** Validate zero-parameter constructor
- **Input:** `enum Status { Pending }`
- **Expected:**
  ```go
  func Status_Pending() Status {
      return Status{tag: StatusTag_Pending}
  }
  ```
- **Rationale:** Common case - no data to store

#### Scenario: Tuple Variant Constructor
- **Purpose:** Validate positional parameter constructor
- **Input:** `Circle(radius: float64)`
- **Expected:**
  ```go
  func Shape_Circle(radius float64) Shape {
      return Shape{
          tag:           ShapeTag_Circle,
          circle_radius: &radius,
      }
  }
  ```
- **Rationale:** Tuple variants are common for single values

#### Scenario: Struct Variant Constructor
- **Purpose:** Validate named parameter constructor
- **Input:** `Rectangle { width: float64, height: float64 }`
- **Expected:**
  ```go
  func Shape_Rectangle(width float64, height float64) Shape {
      return Shape{
          tag:              ShapeTag_Rectangle,
          rectangle_width:  &width,
          rectangle_height: &height,
      }
  }
  ```
- **Rationale:** Struct variants used for multiple related fields

#### Scenario: Generic Constructor
- **Purpose:** Validate type parameter propagation
- **Input:** `enum Result[T, E] { Ok(T) }`
- **Expected:**
  ```go
  func Result_Ok[T any, E any](value T) Result[T, E] {
      return Result[T, E]{tag: ResultTag_Ok, ok: &value}
  }
  ```
- **Rationale:** Generic enums are core to Result/Option

### Unit Tests - Helper Method Generation

#### Scenario: Is* Methods for All Variants
- **Purpose:** Validate helper generation for each variant
- **Input:** `enum Status { Pending, Approved, Rejected }`
- **Expected:**
  ```go
  func (s Status) IsPending() bool  { return s.tag == StatusTag_Pending }
  func (s Status) IsApproved() bool { return s.tag == StatusTag_Approved }
  func (s Status) IsRejected() bool { return s.tag == StatusTag_Rejected }
  ```
- **Rationale:** Enables Go-idiomatic variant checking

#### Scenario: Generic Is* Methods
- **Purpose:** Validate receiver type for generics
- **Input:** `enum Option[T] { Some(T), None }`
- **Expected:**
  ```go
  func (o Option[T]) IsSome() bool { return o.tag == OptionTag_Some }
  func (o Option[T]) IsNone() bool { return o.tag == OptionTag_None }
  ```
- **Rationale:** Generic receivers must use type parameter

### Parser Tests (`pkg/parser/`)

#### Scenario: Parse Simple Enum
- **Purpose:** Validate basic enum parsing
- **Input:** `enum Status { Pending, Active }`
- **Expected:** EnumDecl with 2 VariantDecl nodes (both Unit kind)
- **Rationale:** Foundation for all enum features

#### Scenario: Parse Enum with Generic Parameters
- **Purpose:** Validate type parameter parsing
- **Input:** `enum Result[T, E] { Ok(T), Err(E) }`
- **Expected:** EnumDecl with TypeParams containing T and E
- **Rationale:** Generic enums are P0 requirement

#### Scenario: Parse Trailing Commas
- **Purpose:** Validate Go-style trailing comma support
- **Input:**
  ```dingo
  enum Shape {
      Circle { r: float64 },
      Point,
  }
  ```
- **Expected:** Parses successfully, trailing comma ignored
- **Rationale:** Improves diffs in version control

#### Scenario: Parse Match Expression
- **Purpose:** Validate match syntax parsing
- **Input:**
  ```dingo
  match shape {
      Circle{r} => r,
      Point => 0.0,
  }
  ```
- **Expected:** MatchExpr with 2 MatchArm nodes
- **Rationale:** Match is expression-based in design

#### Scenario: Parse Wildcard Pattern
- **Purpose:** Validate wildcard syntax
- **Input:** `match x { Ok(v) => v, _ => 0 }`
- **Expected:** Second arm has Pattern.Wildcard = true
- **Rationale:** Wildcard enables escape hatch for exhaustiveness

#### Scenario: Parse Destructuring Patterns
- **Purpose:** Validate pattern field extraction
- **Input:** `Circle{radius}` in match arm
- **Expected:** Pattern with FieldPattern for "radius"
- **Rationale:** Destructuring is core to pattern matching ergonomics

### Integration/Golden Tests (`tests/golden/sum_types_*.dingo`)

#### Golden Test 1: Simple Unit Enum
- **File:** `sum_types_01_simple_enum.dingo`
- **Purpose:** End-to-end validation of minimal enum
- **Input:**
  ```dingo
  package main

  enum Status {
      Pending,
      Active,
      Complete,
  }

  func main() {
      s := Status.Pending()
      if s.IsPending() {
          println("pending")
      }
  }
  ```
- **Expected:** Compiles, runs, prints "pending"
- **Rationale:** Smoke test for basic enum functionality

#### Golden Test 2: Tuple Variant Enum
- **File:** `sum_types_02_tuple_variant.dingo`
- **Purpose:** Validate single-field variant
- **Input:**
  ```dingo
  package main

  enum Shape {
      Circle { radius: float64 },
      Point,
  }

  func main() {
      c := Shape.Circle(5.0)
      if c.IsCircle() {
          println("circle")
      }
  }
  ```
- **Expected:** Compiles, runs, prints "circle"
- **Rationale:** Common pattern for wrapping values

#### Golden Test 3: Struct Variant Enum
- **File:** `sum_types_03_struct_variant.dingo`
- **Purpose:** Validate multi-field variants
- **Input:**
  ```dingo
  package main

  enum Shape {
      Rectangle { width: float64, height: float64 },
      Circle { radius: float64 },
      Point,
  }

  func area(s: Shape) -> float64 {
      if s.IsCircle() {
          return 3.14 * (*s.circle_radius) * (*s.circle_radius)
      } else if s.IsRectangle() {
          return (*s.rectangle_width) * (*s.rectangle_height)
      }
      return 0.0
  }

  func main() {
      r := Shape.Rectangle(4.0, 5.0)
      println(area(r)) // 20.0
  }
  ```
- **Expected:** Compiles, runs, prints "20"
- **Rationale:** Real-world use case with multiple fields

#### Golden Test 4: Generic Enum (Phase 2)
- **File:** `sum_types_04_generic_enum.dingo`
- **Purpose:** Validate generic type support
- **Input:**
  ```dingo
  package main

  enum Option[T] {
      Some(T),
      None,
  }

  func unwrapOr<T>(opt: Option[T], def: T) -> T {
      if opt.IsSome() {
          return *opt.some
      }
      return def
  }

  func main() {
      opt := Option.Some(42)
      println(unwrapOr(opt, 0)) // 42
  }
  ```
- **Expected:** Compiles, runs, prints "42"
- **Rationale:** Generic enums are foundation for Result/Option

#### Golden Test 5: Multiple Enums in File
- **File:** `sum_types_05_multiple_enums.dingo`
- **Purpose:** Validate enum registry handles multiple enums
- **Input:**
  ```dingo
  package main

  enum Status { Pending, Active }
  enum Priority { Low, High }

  func main() {
      s := Status.Pending()
      p := Priority.High()
      if s.IsPending() && p.IsHigh() {
          println("high priority pending")
      }
  }
  ```
- **Expected:** Compiles, runs, prints "high priority pending"
- **Rationale:** Files often have multiple related enums

#### Golden Test 6: DINGO:GENERATED Markers
- **File:** `sum_types_06_markers.dingo`
- **Purpose:** Verify VSCode highlighting integration
- **Input:** Any enum declaration
- **Expected:** Generated Go has `// DINGO:GENERATED` comments
- **Rationale:** Integration with VSCode extension

#### Golden Test 7: Edge Case - Single Variant
- **File:** `sum_types_07_single_variant.dingo`
- **Purpose:** Validate degenerate case
- **Input:** `enum Singleton { Only }`
- **Expected:** Compiles (even if not useful)
- **Rationale:** Should not crash on unusual inputs

### Error Case Tests

#### Test: Duplicate Enum Names
- **Input:** Two `enum Status { ... }` in same file
- **Expected:** Compilation error: "duplicate enum Status"
- **Rationale:** Prevents type confusion

#### Test: Duplicate Variant Names
- **Input:** `enum Shape { Circle{r: float64}, Circle{d: float64} }`
- **Expected:** Error: "duplicate variant Circle in enum Shape"
- **Rationale:** Constructor name would conflict

#### Test: Nil Variant Fields
- **Input:** Programmatically constructed VariantDecl with nil Fields
- **Expected:** No crash, treated as unit variant
- **Rationale:** Defensive coding against malformed AST

---

## 3. Test Implementation Strategy

### Unit Test Structure

```go
// pkg/plugin/builtin/sum_types_test.go

package builtin

import (
    "go/ast"
    "go/token"
    "testing"

    dingoast "github.com/MadAppGang/dingo/pkg/ast"
    "github.com/MadAppGang/dingo/pkg/plugin"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

// Test enum registry
func TestSumTypesPlugin_EnumRegistry(t *testing.T) { ... }
func TestSumTypesPlugin_DuplicateEnumDetection(t *testing.T) { ... }
func TestSumTypesPlugin_DuplicateVariantDetection(t *testing.T) { ... }

// Test tag enum generation
func TestGenerateTagEnum_SimpleEnum(t *testing.T) { ... }
func TestGenerateTagEnum_GenericEnum(t *testing.T) { ... }

// Test union struct generation
func TestGenerateUnionStruct_AllVariantKinds(t *testing.T) { ... }
func TestGenerateUnionStruct_GenericStruct(t *testing.T) { ... }

// Test constructor generation
func TestGenerateConstructor_UnitVariant(t *testing.T) { ... }
func TestGenerateConstructor_TupleVariant(t *testing.T) { ... }
func TestGenerateConstructor_StructVariant(t *testing.T) { ... }
func TestGenerateConstructor_GenericVariant(t *testing.T) { ... }

// Test helper method generation
func TestGenerateHelperMethods_AllVariants(t *testing.T) { ... }
func TestGenerateHelperMethods_GenericEnum(t *testing.T) { ... }

// Test nil safety
func TestTransform_NilVariantFields(t *testing.T) { ... }
```

### Parser Test Structure

```go
// pkg/parser/parser_test.go (add to existing file)

func TestParseEnum_Simple(t *testing.T) { ... }
func TestParseEnum_GenericParams(t *testing.T) { ... }
func TestParseEnum_AllVariantKinds(t *testing.T) { ... }
func TestParseEnum_TrailingCommas(t *testing.T) { ... }

func TestParseMatch_Basic(t *testing.T) { ... }
func TestParseMatch_Wildcard(t *testing.T) { ... }
func TestParseMatch_Destructuring(t *testing.T) { ... }
func TestParseMatch_TrailingCommas(t *testing.T) { ... }
```

### Golden File Structure

```
tests/golden/
  sum_types_01_simple_enum.dingo
  sum_types_01_simple_enum.go.golden
  sum_types_02_tuple_variant.dingo
  sum_types_02_tuple_variant.go.golden
  sum_types_03_struct_variant.dingo
  sum_types_03_struct_variant.go.golden
  sum_types_04_generic_enum.dingo
  sum_types_04_generic_enum.go.golden
  sum_types_05_multiple_enums.dingo
  sum_types_05_multiple_enums.go.golden
  sum_types_06_markers.dingo
  sum_types_06_markers.go.golden
  sum_types_07_single_variant.dingo
  sum_types_07_single_variant.go.golden
```

### Integration Test Strategy

1. **Transpile Each Golden File:** Run through full pipeline
2. **Compile Generated Code:** Use `go build` or `go/types.Check`
3. **Verify DINGO:GENERATED:** Check markers present
4. **Runtime Tests (where applicable):** Execute and check output

---

## 4. Expected Test Results

### Success Criteria

**All tests should PASS** given the current implementation state (Phase 1-2 complete).

**Why?**
- Enum declaration parsing: ✅ Implemented in participle.go
- Enum transformation: ✅ Implemented in sum_types.go
- Tag enum generation: ✅ Implemented
- Union struct generation: ✅ Implemented
- Constructor generation: ✅ Implemented
- Helper method generation: ✅ Implemented
- Error validation: ✅ Implemented (duplicate detection, nil checks)
- Generic support: ✅ Implemented (uses ast.IndexExpr/IndexListExpr)

**Known Limitations (NOT tested in this phase):**
- ❌ Match expression transformation (placeholder only) - Phase 3
- ❌ Pattern destructuring - Phase 3
- ❌ Exhaustiveness checking - Phase 4
- ❌ Type inference for match subjects - Phase 3

### Test Coverage Targets

- **Unit Tests:** 80%+ code coverage for sum_types.go
- **Parser Tests:** 100% coverage of enum/match grammar rules
- **Golden Tests:** 7+ realistic scenarios
- **Integration Tests:** End-to-end transpile + compile

### Expected Failures (Phase 3 features)

If we test Phase 3 features, we expect:
- Match expressions in expression context → Error (not yet implemented)
- Pattern destructuring → Placeholder code (won't work at runtime)
- Match type inference → Incorrect tag names

**These are expected and documented as Phase 3 work.**

---

## 5. Test Execution Plan

### Step 1: Unit Tests
1. Create `pkg/plugin/builtin/sum_types_test.go`
2. Implement all unit test scenarios above
3. Run: `go test ./pkg/plugin/builtin -v -run TestSumTypes`
4. Target: All PASS

### Step 2: Parser Tests
1. Extend `pkg/parser/parser_test.go`
2. Add enum and match parsing tests
3. Run: `go test ./pkg/parser -v -run TestParseEnum -run TestParseMatch`
4. Target: All PASS

### Step 3: Golden File Tests
1. Create 7 golden file pairs in `tests/golden/`
2. Update `tests/golden_test.go` to include sum_types tests
3. Run: `go test ./tests -v -run TestGoldenFiles`
4. Target: All PASS

### Step 4: Integration Tests
1. Extend `tests/integration_test.go`
2. Add sum_types end-to-end tests
3. Run: `go test ./tests -v -run TestIntegration`
4. Target: Generated code compiles

### Step 5: Full Test Suite
1. Run: `go test ./... -v`
2. Capture all output
3. Analyze failures (should be none for Phase 1-2)

---

## 6. Coverage Summary

### What's Well-Covered

✅ **Enum Declaration:**
- All variant kinds (unit, tuple, struct)
- Generic type parameters
- Trailing commas
- Parser → AST conversion

✅ **Code Generation:**
- Tag enum with iota
- Tagged union struct
- Constructor functions
- Is* helper methods
- Generic type handling

✅ **Error Handling:**
- Duplicate enum detection
- Duplicate variant detection
- Nil safety
- Validation errors

✅ **Integration:**
- End-to-end transpilation
- Go compilation
- DINGO:GENERATED markers

### Gaps (Deferred to Later Phases)

⏳ **Phase 3 (Match Expressions):**
- Full pattern destructuring
- Expression context handling
- Type inference for match subjects

⏳ **Phase 4 (Exhaustiveness):**
- Missing case detection
- Wildcard coverage tracking

⏳ **Phase 5 (Generics & Prelude):**
- Standard Result/Option in prelude
- Auto-import mechanism
- Generic match type inference

⏳ **Phase 6 (Polish):**
- Source map integration
- Performance benchmarks
- Advanced error messages

---

## 7. Test Quality Assurance

### Verification Checklist

Before reporting results:
- [ ] All test code compiles
- [ ] Test names are descriptive
- [ ] Each test validates one clear behavior
- [ ] Error messages are helpful
- [ ] Golden files have realistic examples
- [ ] No flaky tests (all deterministic)
- [ ] No test-only bugs (tests verify real requirements)

### Failure Analysis Process

When a test fails:
1. ✅ **Verify test is correct** - Check test logic against spec
2. ✅ **Reproduce manually** - Can you trigger the failure outside tests?
3. ✅ **Check test assumptions** - Are we testing Phase 1-2 only?
4. ✅ **Identify root cause** - Implementation bug or test bug?
5. ✅ **Provide evidence** - Show actual vs expected behavior
6. ✅ **Suggest fix** - Where in implementation is the issue?

---

## 8. Confidence Level

**Overall Confidence:** HIGH (85%)

**Why?**
- Implementation is complete for Phase 1-2 scope
- Code review passed with all critical issues fixed
- Architecture follows proven patterns (error_propagation plugin)
- Test scenarios are realistic and comprehensive

**Risk Areas:**
- Generic type handling (IndexExpr vs IndexListExpr edge cases)
- Field naming consistency (tuple variants)
- Parser edge cases (malformed syntax)

**Mitigation:**
- Comprehensive unit tests catch edge cases early
- Golden tests ensure end-to-end correctness
- Go compiler validates generated code structure

---

**Status:** Test Plan Complete - Ready for Implementation
**Next Step:** Implement unit tests and golden tests
