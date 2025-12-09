# Sum Types Implementation - Phase 2.5 Completion Plan

**Session:** 20251116-225837
**Date:** 2025-11-16
**Objective:** Complete Sum Types implementation to production-ready state
**Priority:** P0 - CRITICAL (Blocking Phase 3)

---

## Executive Summary

This plan addresses the completion of Sum Types (Phase 2.5) by fixing remaining critical issues and verifying end-to-end functionality. The previous session (20251116-202224) successfully implemented core enum generation and achieved 100% unit test pass rate (31/31 tests), but left several blocking issues unresolved.

**Key Objectives:**
1. Fix position information bug (P1 - blocking golden tests)
2. Address 10 IMPORTANT code review issues
3. Verify end-to-end transpilation works
4. Achieve production-ready merge state

**Estimated Duration:** 2-3 days

---

## 1. Problem Analysis

### 1.1 Current State Assessment

**What's Working (Verified by Tests):**
- ✅ Enum declaration parsing (all variant types)
- ✅ Tag enum generation with iota
- ✅ Tagged union struct generation
- ✅ Constructor function generation
- ✅ Is* helper method generation
- ✅ Generic type parameter support
- ✅ Duplicate detection (enums and variants)
- ✅ Nil safety checks
- ✅ Plugin registration and integration

**Test Results:**
- Unit tests: 31/31 PASS (100%)
- Golden tests: 0/4 PASS (blocked by position info bug)
- Integration: Not tested

**What's Broken:**

**P1 - BLOCKING:**
- **Position Info Bug:** Generated AST declarations lack `TokPos` field
  - Impact: Golden file tests panic in `go/types` checker
  - Root cause: `GenDecl` nodes created without position information
  - Location: All generation functions in `sum_types.go`

**P2 - IMPORTANT (10 items from code review):**
1. Match expression IIFE wrapping for expression contexts
2. Pattern destructuring implementation
3. DingoNodes cleanup after placeholder deletion
4. Constructor parameter aliasing fix
5. Variant field access nil guards
6. Unsupported pattern form error handling
7. Match guard transformation
8. Field name collision detection
9. Memory allocation overhead documentation
10. Comprehensive match transformation tests

### 1.2 Technical Debt from Previous Session

**Known Limitations (Documented):**
- Match expressions only work as statements (not expressions)
- Pattern destructuring is placeholder-only
- No exhaustiveness checking
- Type inference uses simple heuristic (variant name lookup)

**Code Quality Issues:**
- No position tracking in generated code
- Match transformation is minimal
- Error messages could be more helpful
- Source map support missing

---

## 2. Architectural Analysis

### 2.1 Position Information Problem

**Root Cause Analysis:**

The `go/types` package requires valid position information to:
1. Calculate declaration end positions (`GenDecl.End()`)
2. Build symbol tables with location tracking
3. Provide accurate error messages

Current code creates declarations like:
```go
typeDecl := &ast.GenDecl{
    Tok:   token.TYPE,
    Specs: []ast.Spec{typeSpec},
    // MISSING: TokPos field
}
```

When `go/types` calls `GenDecl.End()`, it tries to access `Specs[0]` which panics if positions aren't set.

**Fix Strategy:**

We need to set `TokPos` on all generated declarations. However, we have a challenge: generated code doesn't correspond to any source location. We have three options:

**Option A: Use invalid position (token.NoPos)**
```go
typeDecl := &ast.GenDecl{
    TokPos: token.NoPos,  // Indicates synthetic/generated code
    Tok:    token.TYPE,
    Specs:  []ast.Spec{typeSpec},
}
```
- Pros: Simple, semantically correct (code IS generated)
- Cons: May cause issues with some tools expecting valid positions

**Option B: Use enum declaration position**
```go
typeDecl := &ast.GenDecl{
    TokPos: enumDecl.Name.Pos(),  // Reuse original enum position
    Tok:    token.TYPE,
    Specs:  []ast.Spec{typeSpec},
}
```
- Pros: Valid position, maps back to source
- Cons: Multiple declarations share same position (confusing)

**Option C: Synthesize sequential positions**
```go
// In plugin initialization
p.syntheticPos = enumDecl.Name.End() + 1

typeDecl := &ast.GenDecl{
    TokPos: p.syntheticPos,
    Tok:    token.TYPE,
    Specs:  []ast.Spec{typeSpec},
}
p.syntheticPos += token.Pos(len(enumName) + 10)  // Approximate spacing
```
- Pros: Unique positions, allows source map generation
- Cons: More complex, requires position tracking

**RECOMMENDATION: Option B (Use enum declaration position)**

Rationale:
1. Simple to implement (2 hours)
2. Semantically meaningful (generated code derives from enum)
3. Enables source maps (position maps to enum declaration)
4. Compatible with `go/types` checker
5. Matches approach used by other meta-languages

### 2.2 Match Expression Strategy

**Current Implementation:**
```go
// Detects expression context and errors
if _, isExprContext := cursor.Parent().(ast.Expr); isExprContext {
    p.currentContext.Logger.Error("match expressions not yet supported")
    return
}
```

**Phase 2.5 Recommendation: Document and Test**

Do NOT implement full IIFE wrapping in this phase. Instead:
1. Improve error message with example
2. Add test to verify error is raised
3. Document as Phase 3 feature
4. Create tracking issue

Rationale:
- IIFE wrapping is complex (4-6 hours)
- Not blocking core functionality
- Phase 3 will redesign match transformation
- Current error handling is correct approach

### 2.3 Pattern Destructuring Architecture

**Current State:**
```go
func (p *SumTypesPlugin) generateDestructuring(enumType string, pattern *dingoast.Pattern) []ast.Stmt {
    // TODO: Implement pattern destructuring
    return nil
}
```

**Required Implementation:**

Pattern destructuring needs to:
1. Extract variant fields from union struct
2. Dereference pointer fields
3. Bind to pattern variables
4. Handle nested patterns (future)

**Example Transformation:**

```dingo
match shape {
    Circle { radius } => 3.14 * radius * radius,
    Rectangle { width, height } => width * height,
    Point => 0.0,
}
```

Should generate:
```go
switch shape.tag {
case ShapeTag_Circle:
    radius := *shape.circle_radius  // ← Destructure
    return 3.14 * radius * radius
case ShapeTag_Rectangle:
    width := *shape.rectangle_width
    height := *shape.rectangle_height
    return width * height
case ShapeTag_Point:
    return 0.0
}
```

**Implementation Plan:**

```go
func (p *SumTypesPlugin) generateDestructuring(
    enumType string,
    variantName string,
    pattern *dingoast.Pattern,
) []ast.Stmt {
    // 1. Get variant declaration from registry
    enumDecl := p.enumRegistry[enumType]
    var variantDecl *dingoast.VariantDecl
    for _, v := range enumDecl.Variants {
        if v.Name.Name == variantName {
            variantDecl = v
            break
        }
    }

    if variantDecl == nil {
        return nil  // Error already logged
    }

    // 2. Generate assignments for each pattern binding
    stmts := []ast.Stmt{}

    switch pattern.Kind {
    case dingoast.PatternUnit:
        // No bindings needed
        return nil

    case dingoast.PatternStruct:
        // For each field pattern: fieldName := *unionVar.variantName_fieldName
        for _, fieldPat := range pattern.Fields {
            fieldName := strings.ToLower(variantName) + "_" + fieldPat.Name

            stmt := &ast.AssignStmt{
                Lhs: []ast.Expr{&ast.Ident{Name: fieldPat.Name}},
                Tok: token.DEFINE,
                Rhs: []ast.Expr{
                    &ast.StarExpr{  // Dereference pointer
                        X: &ast.SelectorExpr{
                            X:   &ast.Ident{Name: "shape"},  // TODO: Use actual match var
                            Sel: &ast.Ident{Name: fieldName},
                        },
                    },
                },
            }
            stmts = append(stmts, stmt)
        }
        return stmts

    case dingoast.PatternTuple:
        // Similar to struct, but use _0, _1, etc.
        for i, binding := range pattern.Bindings {
            fieldName := strings.ToLower(variantName) + "_" + strconv.Itoa(i)

            stmt := &ast.AssignStmt{
                Lhs: []ast.Expr{&ast.Ident{Name: binding}},
                Tok: token.DEFINE,
                Rhs: []ast.Expr{
                    &ast.StarExpr{
                        X: &ast.SelectorExpr{
                            X:   &ast.Ident{Name: "shape"},
                            Sel: &ast.Ident{Name: fieldName},
                        },
                    },
                },
            }
            stmts = append(stmts, stmt)
        }
        return stmts
    }

    return nil
}
```

**Nil Safety Consideration:**

Current implementation uses pointer fields for variant data. We must add nil checks:

```go
// Option 1: Panic on nil (runtime error - current behavior)
radius := *shape.circle_radius  // Panics if nil

// Option 2: Check and use zero value
var radius float64
if shape.circle_radius != nil {
    radius = *shape.circle_radius
}

// Option 3: Guarantee non-nil via constructor (RECOMMENDED)
// Constructors always initialize fields, so nil is impossible
```

**RECOMMENDATION: Option 3 (Constructor guarantees)**

Rationale:
- Constructors always set field values
- Union can only be created via constructors
- No runtime overhead
- Matches Rust/Swift semantics

However, we should add validation to detect if user manually creates union:
```go
// In destructuring code, add assertion check
if shape.circle_radius == nil {
    panic("invalid Shape: Circle variant has nil radius (union not created via constructor)")
}
```

---

## 3. Implementation Plan

### Phase 1: Fix Position Information Bug (P1 - 2-3 hours)

**Priority:** CRITICAL - Blocks all integration testing

**Tasks:**
1. Add `TokPos` field to all `GenDecl` creations
2. Use `enumDecl.Name.Pos()` as position source
3. Add helper function to reduce duplication
4. Update all generation functions

**Files to Modify:**
- `pkg/plugin/builtin/sum_types.go`
  - `generateTagEnum()` - lines 197-239
  - `generateUnionStruct()` - lines 241-290
  - `generateConstructor()` - lines 292-380
  - `generateHelperMethod()` - lines 382-460

**Implementation:**

```go
// Helper function (add to SumTypesPlugin)
func (p *SumTypesPlugin) withPosition(decl *ast.GenDecl, pos token.Pos) *ast.GenDecl {
    decl.TokPos = pos
    return decl
}

// Update generateTagEnum
func (p *SumTypesPlugin) generateTagEnum(enumDecl *dingoast.EnumDecl) []ast.Decl {
    enumPos := enumDecl.Name.Pos()  // ← Get position

    typeDecl := &ast.GenDecl{
        TokPos: enumPos,  // ← Set position
        Tok:    token.TYPE,
        Specs:  []ast.Spec{typeSpec},
    }

    constDecl := &ast.GenDecl{
        TokPos: enumPos,  // ← Set position
        Tok:    token.CONST,
        Lparen: 1,
        Specs:  constSpecs,
    }

    return []ast.Decl{typeDecl, constDecl}
}

// Similar updates for other functions...
```

**Testing:**
- Run golden tests after fix: `go test ./tests -v -run TestGoldenFiles/sum_types`
- Verify no panic occurs
- Check generated .go files match expected output

**Success Criteria:**
- ✅ All golden tests pass (4/4)
- ✅ No panics in `go/types` checker
- ✅ Generated code compiles

---

### Phase 2: Implement Pattern Destructuring (P1 - 4-6 hours)

**Priority:** CRITICAL - Core feature incomplete

**Tasks:**
1. Implement `generateDestructuring()` method
2. Handle struct patterns (named fields)
3. Handle tuple patterns (positional fields)
4. Handle unit patterns (no fields)
5. Add nil safety checks
6. Update `transformMatchArm()` to call destructuring

**Files to Modify:**
- `pkg/plugin/builtin/sum_types.go`
  - `generateDestructuring()` - lines 617-648 (replace stub)
  - `transformMatchArm()` - lines 570-615 (integrate destructuring)

**Implementation Steps:**

**Step 1: Implement field extraction**
```go
func (p *SumTypesPlugin) generateDestructuring(
    matchedExpr ast.Expr,
    enumType string,
    pattern *dingoast.Pattern,
) []ast.Stmt {
    if pattern.Kind == dingoast.PatternUnit {
        return nil  // No destructuring for unit variants
    }

    variantName := pattern.Variant.Name

    // Get variant declaration
    enumDecl := p.enumRegistry[enumType]
    var variantDecl *dingoast.VariantDecl
    for _, v := range enumDecl.Variants {
        if v.Name.Name == variantName {
            variantDecl = v
            break
        }
    }

    if variantDecl == nil {
        return nil
    }

    stmts := []ast.Stmt{}

    // Generate destructuring assignments
    switch pattern.Kind {
    case dingoast.PatternStruct:
        stmts = p.generateStructDestructuring(matchedExpr, variantName, variantDecl, pattern)
    case dingoast.PatternTuple:
        stmts = p.generateTupleDestructuring(matchedExpr, variantName, variantDecl, pattern)
    }

    return stmts
}
```

**Step 2: Struct pattern destructuring**
```go
func (p *SumTypesPlugin) generateStructDestructuring(
    matchedExpr ast.Expr,
    variantName string,
    variantDecl *dingoast.VariantDecl,
    pattern *dingoast.Pattern,
) []ast.Stmt {
    stmts := []ast.Stmt{}

    for _, fieldPat := range pattern.Fields {
        fieldName := strings.ToLower(variantName) + "_" + fieldPat.Name

        // radius := *shape.circle_radius
        stmt := &ast.AssignStmt{
            Lhs: []ast.Expr{&ast.Ident{Name: fieldPat.Name}},
            Tok: token.DEFINE,
            Rhs: []ast.Expr{
                &ast.StarExpr{
                    X: &ast.SelectorExpr{
                        X:   matchedExpr,
                        Sel: &ast.Ident{Name: fieldName},
                    },
                },
            },
        }
        stmts = append(stmts, stmt)
    }

    return stmts
}
```

**Step 3: Update match arm transformation**
```go
func (p *SumTypesPlugin) transformMatchArm(
    enumType string,
    matchedExpr ast.Expr,
    arm *dingoast.MatchArm,
) (*ast.CaseClause, error) {
    // ... existing code ...

    // Generate destructuring assignments
    destructStmts := p.generateDestructuring(matchedExpr, enumType, arm.Pattern)

    // Build case body
    bodyStmts := []ast.Stmt{}
    bodyStmts = append(bodyStmts, destructStmts...)  // ← Add destructuring
    bodyStmts = append(bodyStmts, &ast.ExprStmt{X: arm.Body})

    return &ast.CaseClause{
        List: []ast.Expr{caseExpr},
        Body: bodyStmts,
    }, nil
}
```

**Testing:**
- Add unit test for destructuring generation
- Update golden test expected output
- Verify match with patterns compiles

**Success Criteria:**
- ✅ Struct patterns extract fields correctly
- ✅ Tuple patterns use positional bindings
- ✅ Generated code compiles
- ✅ Variables are properly scoped

---

### Phase 3: Address IMPORTANT Issues (P2 - 6-8 hours)

**Priority:** HIGH - Required before merge

**3.1 Clean Up Placeholder Nodes (Item 10) - 1 hour**

**Problem:** Deleted enum placeholders remain in DingoNodes map

**Fix:**
```go
// In transformEnumDecl, after cursor.Delete()
func (p *SumTypesPlugin) transformEnumDecl(cursor *astutil.Cursor, enumDecl *dingoast.EnumDecl) {
    // ... existing generation code ...

    // Remove the placeholder declaration
    placeholder := cursor.Node()
    cursor.Delete()

    // Clean up DingoNodes map
    if p.currentFile != nil {
        p.currentFile.RemoveDingoNode(placeholder)
    }
}
```

Requires adding method to `dingoast.File`:
```go
// pkg/ast/file.go
func (f *File) RemoveDingoNode(node ast.Node) {
    delete(f.DingoNodes, node)
}
```

**3.2 Fix Constructor Parameter Aliasing (Item 11) - 1 hour**

**Problem:** Reuses `variant.Fields.List` as constructor params (shared reference)

**Current code:**
```go
func (p *SumTypesPlugin) generateConstructor(enumDecl *dingoast.EnumDecl, variant *dingoast.VariantDecl) *ast.FuncDecl {
    params := &ast.FieldList{
        List: variant.Fields.List,  // ← SHARED REFERENCE (BUG)
    }
}
```

**Fix: Deep copy fields**
```go
func (p *SumTypesPlugin) copyFieldList(fields *ast.FieldList) *ast.FieldList {
    if fields == nil || fields.List == nil {
        return &ast.FieldList{}
    }

    copied := make([]*ast.Field, len(fields.List))
    for i, f := range fields.List {
        // Deep copy field
        copiedField := &ast.Field{
            Names: make([]*ast.Ident, len(f.Names)),
            Type:  f.Type,  // Type can be shared (immutable)
        }
        for j, name := range f.Names {
            copiedField.Names[j] = &ast.Ident{Name: name.Name}
        }
        copied[i] = copiedField
    }

    return &ast.FieldList{List: copied}
}

func (p *SumTypesPlugin) generateConstructor(enumDecl *dingoast.EnumDecl, variant *dingoast.VariantDecl) *ast.FuncDecl {
    params := p.copyFieldList(variant.Fields)  // ← Use copy
    // ...
}
```

**3.3 Add Unsupported Pattern Errors (Item 13) - 30 minutes**

**Problem:** Silently treats all patterns as variant patterns

**Fix:**
```go
func (p *SumTypesPlugin) transformMatchArm(
    enumType string,
    matchedExpr ast.Expr,
    arm *dingoast.MatchArm,
) (*ast.CaseClause, error) {
    // Validate pattern type
    if arm.Pattern.Variant == nil {
        if arm.Pattern.Kind == dingoast.PatternWildcard {
            // Wildcard is supported
            return &ast.CaseClause{
                List: nil,  // nil = default case
                Body: []ast.Stmt{&ast.ExprStmt{X: arm.Body}},
            }, nil
        }

        // Unsupported pattern
        return nil, fmt.Errorf(
            "unsupported pattern type at %v: only variant and wildcard patterns supported in Phase 2 (literal patterns coming in Phase 3)",
            arm.Pattern.Pos(),
        )
    }

    // ... rest of variant pattern handling ...
}
```

**3.4 Handle Match Guards (Item 14) - 2 hours**

**Problem:** Guards parsed but silently discarded

**Strategy:** Error for now, implement in Phase 3

```go
func (p *SumTypesPlugin) transformMatchArm(
    enumType string,
    matchedExpr ast.Expr,
    arm *dingoast.MatchArm,
) (*ast.CaseClause, error) {
    // Check for guard
    if arm.Guard != nil {
        return nil, fmt.Errorf(
            "match guards not yet supported at %v (use if statement inside match arm body instead)",
            arm.Guard.Pos(),
        )
    }

    // ... rest of transformation ...
}
```

**3.5 Add Field Name Collision Detection (Item 15) - 2 hours**

**Problem:** `variantName_fieldName` can collide across variants

**Example:**
```dingo
enum Collision {
    Foo { bar_baz: int },
    FooBar { baz: int },  // ← Generates foobar_baz (collision!)
}
```

**Fix:**
```go
func (p *SumTypesPlugin) detectFieldCollisions(enumDecl *dingoast.EnumDecl) error {
    fieldNames := make(map[string]string)  // fieldName -> variantName

    for _, variant := range enumDecl.Variants {
        if variant.Fields == nil || variant.Fields.List == nil {
            continue
        }

        for _, field := range variant.Fields.List {
            for _, name := range field.Names {
                fieldName := strings.ToLower(variant.Name.Name) + "_" + name.Name

                if existingVariant, exists := fieldNames[fieldName]; exists {
                    return fmt.Errorf(
                        "field name collision in enum %s: variants %s and %s both generate field '%s'",
                        enumDecl.Name.Name,
                        existingVariant,
                        variant.Name.Name,
                        fieldName,
                    )
                }

                fieldNames[fieldName] = variant.Name.Name
            }
        }
    }

    return nil
}

// Call in collectEnums
func (p *SumTypesPlugin) collectEnums(file *ast.File) error {
    // ... existing code ...

    // Check for field collisions
    if err := p.detectFieldCollisions(enumDecl); err != nil {
        return err
    }

    // ...
}
```

**3.6 Document Memory Overhead (Item 16) - 15 minutes**

**Task:** Add TODO comments about optimization

```go
// generateUnionStruct creates the tagged union struct with pointer fields
//
// MEMORY LAYOUT NOTE (Phase 2):
// Current implementation uses pointer fields for all variant data to allow nil
// for unused variants. This means:
//   - Each variant field adds 8 bytes (pointer size)
//   - Actual data is heap-allocated
//   - Memory overhead = num_fields * 8 + tag (1 byte) + padding
//
// Example: enum with 3 variants * 2 fields each = 48 bytes + data
//
// OPTIMIZATION OPPORTUNITIES (Phase 6):
// - Small value optimization: inline values <= 16 bytes
// - Union of structs instead of pointer fields (unsafe)
// - Compress tag into padding bits
//
// See: Rust enum layout, Swift enum layout for reference
func (p *SumTypesPlugin) generateUnionStruct(enumDecl *dingoast.EnumDecl) *ast.GenDecl {
    // ... existing code ...
}
```

**3.7 Add Comprehensive Match Tests (Item 17) - 2 hours**

**Add tests:**
```go
// pkg/plugin/builtin/sum_types_test.go

func TestTransformMatch_ExpressionContext(t *testing.T) {
    // Verify error when match used as expression
}

func TestTransformMatch_WithWildcard(t *testing.T) {
    // Verify wildcard generates default case
}

func TestTransformMatch_WithGuard(t *testing.T) {
    // Verify guard causes error (not yet supported)
}

func TestTransformMatch_StructPatternDestructuring(t *testing.T) {
    // Verify struct patterns extract fields
}

func TestTransformMatch_TuplePatternDestructuring(t *testing.T) {
    // Verify tuple patterns use positions
}

func TestGenerateDestructuring_NilSafety(t *testing.T) {
    // Verify nil pointer handling
}
```

---

### Phase 4: End-to-End Verification (P1 - 4-6 hours)

**Priority:** CRITICAL - Prove implementation works

**4.1 Create Example Files (1 hour)**

Create comprehensive example `.dingo` files:

**`tests/e2e/sum_types_simple.dingo`**
```dingo
package main

import "fmt"

enum Status {
    Pending,
    Active,
    Complete,
}

func main() {
    status := Status_Pending()

    if status.IsPending() {
        fmt.Println("Status is pending")
    }

    match status {
        Pending => fmt.Println("Waiting"),
        Active => fmt.Println("Running"),
        Complete => fmt.Println("Done"),
    }
}
```

**`tests/e2e/sum_types_struct_variants.dingo`**
```dingo
package main

import "fmt"

enum Shape {
    Point,
    Circle { radius: float64 },
    Rectangle { width: float64, height: float64 },
}

func area(shape: Shape) -> float64 {
    match shape {
        Circle{radius} => 3.14159 * radius * radius,
        Rectangle{width, height} => width * height,
        Point => 0.0,
    }
}

func main() {
    circle := Shape_Circle(5.0)
    rect := Shape_Rectangle(10.0, 20.0)
    point := Shape_Point()

    fmt.Printf("Circle area: %f\n", area(circle))
    fmt.Printf("Rectangle area: %f\n", area(rect))
    fmt.Printf("Point area: %f\n", area(point))
}
```

**`tests/e2e/sum_types_generic.dingo`**
```dingo
package main

import "fmt"

enum Result[T, E] {
    Ok(T),
    Err(E),
}

func divide(a: float64, b: float64) -> Result[float64, string] {
    if b == 0.0 {
        return Result_Err("division by zero")
    }
    return Result_Ok(a / b)
}

func main() {
    result1 := divide(10.0, 2.0)
    match result1 {
        Ok(value) => fmt.Printf("Result: %f\n", value),
        Err(msg) => fmt.Printf("Error: %s\n", msg),
    }

    result2 := divide(10.0, 0.0)
    match result2 {
        Ok(value) => fmt.Printf("Result: %f\n", value),
        Err(msg) => fmt.Printf("Error: %s\n", msg),
    }
}
```

**4.2 Run Full Pipeline (2 hours)**

Test each file through complete pipeline:

```bash
# Build transpiler
go build ./cmd/dingo

# Transpile examples
./dingo build tests/e2e/sum_types_simple.dingo
./dingo build tests/e2e/sum_types_struct_variants.dingo
./dingo build tests/e2e/sum_types_generic.dingo

# Verify generated .go files exist and are valid
go fmt tests/e2e/sum_types_simple.go
go fmt tests/e2e/sum_types_struct_variants.go
go fmt tests/e2e/sum_types_generic.go

# Compile generated code
go build -o tests/e2e/simple tests/e2e/sum_types_simple.go
go build -o tests/e2e/struct tests/e2e/sum_types_struct_variants.go
go build -o tests/e2e/generic tests/e2e/sum_types_generic.go

# Run executables
./tests/e2e/simple
./tests/e2e/struct
./tests/e2e/generic

# Verify output matches expectations
```

**4.3 Create Automated E2E Test (2 hours)**

```go
// tests/e2e_test.go

func TestEndToEnd_SumTypes(t *testing.T) {
    tests := []struct {
        name           string
        dingoFile      string
        expectedOutput string
    }{
        {
            name:      "simple enum",
            dingoFile: "sum_types_simple.dingo",
            expectedOutput: `Status is pending
Waiting`,
        },
        {
            name:      "struct variants",
            dingoFile: "sum_types_struct_variants.dingo",
            expectedOutput: `Circle area: 78.539750
Rectangle area: 200.000000
Point area: 0.000000`,
        },
        {
            name:      "generic enum",
            dingoFile: "sum_types_generic.dingo",
            expectedOutput: `Result: 5.000000
Error: division by zero`,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // 1. Transpile .dingo → .go
            goFile := transpile(t, tt.dingoFile)

            // 2. Compile .go → binary
            binary := compile(t, goFile)

            // 3. Run binary and capture output
            output := run(t, binary)

            // 4. Verify output
            if output != tt.expectedOutput {
                t.Errorf("Output mismatch\nExpected:\n%s\nGot:\n%s", tt.expectedOutput, output)
            }
        })
    }
}
```

**4.4 Verify Code Quality (1 hour)**

Run all quality checks:
```bash
# Format code
go fmt ./...

# Lint
golangci-lint run

# Vet
go vet ./...

# Test coverage
go test -cover ./pkg/plugin/builtin
go test -cover ./tests

# Race detector
go test -race ./...

# Build
go build ./cmd/dingo
```

**Success Criteria:**
- ✅ All e2e examples transpile without errors
- ✅ Generated Go code compiles
- ✅ Executables run and produce expected output
- ✅ Automated tests pass
- ✅ No linter warnings
- ✅ No race conditions

---

## 4. Testing Strategy

### 4.1 Test Coverage Requirements

**Unit Tests (pkg/plugin/builtin/sum_types_test.go):**
- ✅ Existing: 31 tests (100% pass)
- ➕ New: 8 additional tests
  - Position info validation
  - Destructuring generation
  - Error handling (guards, unsupported patterns)
  - Field collision detection
  - Nil safety
  - Match expression context detection
  - Wildcard handling
  - DingoNodes cleanup

**Golden Tests (tests/golden/sum_types_*.dingo):**
- ✅ Existing: 4 tests (currently blocked)
- ➕ After position fix: 4/4 should pass
- ➕ New: 2 additional golden tests
  - Match with destructuring
  - Complex multi-variant enums

**Integration Tests (tests/e2e_test.go):**
- ➕ New: 3 end-to-end tests
  - Simple enums
  - Struct variants with match
  - Generic enums

**Target Coverage:**
- Unit test coverage: 85%+
- All golden tests passing
- All e2e tests passing

### 4.2 Test Execution Plan

**Step 1: Fix position bug**
```bash
go test ./pkg/plugin/builtin -v -run TestGenerateTagEnum
go test ./pkg/plugin/builtin -v -run TestGenerateUnionStruct
go test ./tests -v -run TestGoldenFiles/sum_types_01
```

**Step 2: Add destructuring**
```bash
go test ./pkg/plugin/builtin -v -run TestGenerateDestructuring
go test ./pkg/plugin/builtin -v -run TestTransformMatch
go test ./tests -v -run TestGoldenFiles/sum_types_02
```

**Step 3: Full test suite**
```bash
go test ./... -v
```

**Step 4: E2E verification**
```bash
go test ./tests -v -run TestEndToEnd
```

---

## 5. Risk Analysis

### 5.1 High Risk Areas

**Position Information:**
- Risk: Wrong approach could break source maps
- Mitigation: Use enum position (semantically correct)
- Fallback: Use token.NoPos if issues arise

**Pattern Destructuring:**
- Risk: Complex edge cases (nested patterns, aliases)
- Mitigation: Limit to simple patterns in Phase 2
- Fallback: Error on complex patterns, defer to Phase 3

**Nil Safety:**
- Risk: Runtime panics from nil pointer dereference
- Mitigation: Constructors guarantee non-nil
- Fallback: Add nil checks in generated code

### 5.2 Medium Risk Areas

**Field Name Collisions:**
- Risk: Complex detection algorithm
- Mitigation: Simple string comparison
- Fallback: Document limitation, fix in Phase 3

**Match Expression IIFE:**
- Risk: Scope/variable capture issues
- Mitigation: Don't implement in Phase 2
- Fallback: Clear error message

### 5.3 Low Risk Areas

**DingoNodes Cleanup:**
- Risk: Memory leak
- Mitigation: Simple map deletion
- Impact: Low (temporary during compilation)

**Documentation:**
- Risk: None
- Mitigation: N/A

---

## 6. Success Criteria

### 6.1 Must Have (P0)

- ✅ All golden file integration tests pass (4/4)
- ✅ Position information bug fixed
- ✅ Pattern destructuring implemented (struct and tuple)
- ✅ All IMPORTANT issues addressed (10 items)
- ✅ End-to-end transpilation works
- ✅ Generated Go code compiles and executes
- ✅ Test coverage at 85%+
- ✅ Implementation ready to merge

### 6.2 Should Have (P1)

- ✅ E2E automated tests
- ✅ Code quality checks pass (fmt, vet, lint)
- ✅ Error messages are helpful
- ✅ Documentation complete

### 6.3 Could Have (P2)

- Performance benchmarks
- Additional golden tests
- Optimization documentation
- Memory profiling

---

## 7. Timeline and Milestones

### Day 1 (8 hours)
- **Morning (4h):** Fix position information bug
  - Implement position tracking
  - Update all generation functions
  - Run golden tests
  - Fix any issues

- **Afternoon (4h):** Implement pattern destructuring
  - Write struct pattern destructuring
  - Write tuple pattern destructuring
  - Add unit tests
  - Verify match transformation works

**Milestone:** Golden tests passing, basic match works

### Day 2 (8 hours)
- **Morning (4h):** Address IMPORTANT issues
  - DingoNodes cleanup
  - Constructor parameter aliasing
  - Unsupported pattern errors
  - Match guard errors

- **Afternoon (4h):** Continue IMPORTANT issues
  - Field collision detection
  - Memory overhead documentation
  - Add comprehensive tests
  - Code review and refactoring

**Milestone:** All critical issues resolved

### Day 3 (8 hours)
- **Morning (4h):** End-to-end verification
  - Create example files
  - Run full pipeline
  - Create automated e2e tests
  - Fix any integration issues

- **Afternoon (4h):** Polish and merge prep
  - Code quality checks
  - Documentation
  - Final testing
  - Create merge request

**Milestone:** Ready to merge

---

## 8. Deliverables

### Code Deliverables

1. **Modified Files:**
   - `pkg/plugin/builtin/sum_types.go` (position info, destructuring, fixes)
   - `pkg/ast/file.go` (RemoveDingoNode method)
   - `pkg/plugin/builtin/sum_types_test.go` (additional tests)

2. **New Files:**
   - `tests/e2e/sum_types_simple.dingo`
   - `tests/e2e/sum_types_struct_variants.dingo`
   - `tests/e2e/sum_types_generic.dingo`
   - `tests/e2e_test.go` (automated e2e tests)

3. **Updated Files:**
   - `tests/golden/sum_types_*.go.golden` (expected output with positions)

### Documentation Deliverables

1. **Session Documentation:**
   - This plan (initial-plan.md)
   - Implementation notes
   - Test results
   - Session summary

2. **Code Documentation:**
   - Updated godoc comments
   - Architecture decision notes
   - Memory layout documentation
   - Known limitations

3. **User Documentation:**
   - CHANGELOG.md update
   - Feature status (what works, what doesn't)

---

## 9. Next Steps After Completion

### Immediate (Post-Merge)

1. Update CHANGELOG.md with Phase 2.5 completion
2. Close related issues
3. Create Phase 3 tracking issue (match expressions as expressions)
4. Update project roadmap

### Short-Term (Next Sprint)

1. **Phase 3: Full Match Expression Support**
   - IIFE wrapping for expression contexts
   - Type inference integration
   - Complex pattern support

2. **Phase 4: Exhaustiveness Checking**
   - Missing case detection
   - Unreachable pattern warnings
   - Helpful error messages

### Long-Term (Future Phases)

1. **Phase 5: Optimization**
   - Small value optimization
   - Memory layout improvements
   - Performance benchmarks

2. **Phase 6: Advanced Features**
   - Nested patterns
   - Pattern guards
   - Derive traits

---

## 10. Questions and Ambiguities

See `gaps.json` for detailed questions requiring user input.

---

**Plan Status:** COMPLETE
**Confidence Level:** HIGH (90%)
**Ready for Implementation:** YES

This plan provides a clear path to completing the Sum Types implementation and achieving production-ready merge state. The phased approach minimizes risk while ensuring all critical issues are addressed.
