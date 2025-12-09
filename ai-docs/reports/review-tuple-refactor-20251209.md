# Tuple Refactoring Code Review

**Date**: 2025-12-09
**Reviewer**: code-reviewer (internal)
**Scope**: Tuple AST types, parser extensions, code generation, pipeline integration
**Files Reviewed**:
- `pkg/ast/tuple.go`
- `pkg/ast/tuple_types.go` (new)
- `pkg/parser/pratt.go`
- `pkg/parser/stmt.go`
- `pkg/codegen/tuple.go`
- `pkg/codegen/tuple_test.go`
- `pkg/transpiler/tuple_transform.go`

---

## ✅ Strengths

1. **Clean AST Design** (`pkg/ast/tuple.go`):
   - Well-documented AST node types with clear examples
   - Proper implementation of Node/Expr/Stmt interfaces
   - Position tracking (Pos/End) correctly implemented
   - Good separation of concerns (TupleLiteral, TupleDestructure, TupleTypeExpr)

2. **Parser Integration** (`pkg/parser/pratt.go`, `pkg/parser/stmt.go`):
   - Callback mechanism (`OnTupleLiteral`) for collecting tuple nodes is elegant
   - Disambiguation logic in `parseGroupedOrTuple()` is clear
   - Recursive handling of nested tuples is correct
   - Parser stores tuple nodes for pipeline integration (good architecture)

3. **Code Generation** (`pkg/codegen/tuple.go`):
   - Marker-based approach allows type resolution in Pass 2
   - Source mapping generation is present
   - Recursive generation handles nested tuples correctly
   - Nil checks prevent panics

4. **Comprehensive Testing** (`pkg/codegen/tuple_test.go`):
   - Good coverage of simple, nested, and complex cases
   - Tests include source mapping verification
   - Real-world examples (Point2D, BoundingBox)
   - Benchmark tests included

5. **Variable Naming Convention**:
   - Follows CLAUDE.md convention: `tmp`, `tmp1`, `tmp2` (no underscores, camelCase)
   - Helper functions `formatTmpVar()` and `formatFieldIndex()` correctly implemented

---

## ⚠️ Concerns

### CRITICAL Issues (Must Fix)

#### C1. CLAUDE.md Violation - Byte Splicing in Pipeline
**File**: `pkg/transpiler/tuple_transform.go:126-135`
**Category**: Architecture
**Issue**: The code uses byte splicing to transform tuples, which violates the mandated architecture:

```go
// TECHNICAL DEBT: This byte splicing approach works but violates the pure AST
// pipeline philosophy documented in CLAUDE.md. In a future refactor, we should
// build the complete AST first, then generate all code in one pass.
newResult := make([]byte, 0, len(result)-(replaceEnd-replaceStart)+len(marker))
newResult = append(newResult, result[:replaceStart]...)
newResult = append(newResult, marker...)
newResult = append(newResult, result[replaceEnd:]...)
```

**Impact**:
- Violates the core architecture principle: `tokenizer → parser → AST → codegen`
- Makes code harder to maintain and debug
- Position tracking becomes fragile with multiple transformations
- Self-acknowledged technical debt with no issue tracker reference

**Recommendation**:
1. **Short-term**: Create a GitHub issue tracking this technical debt (line 129 references `issues/XXX`)
2. **Medium-term**: Refactor to build complete transformed AST, then generate code in one pass
3. The comment acknowledges this is "pragmatic" - ensure this doesn't become permanent
4. Add unit tests that specifically validate position tracking survives multiple transformations

**Example of better approach**:
```go
// Instead of splicing bytes, build transformed AST:
func transformTupleAST(file *goast.File, tuples []tupleNodeWithPos) *goast.File {
    // Walk AST, replace tuple nodes with marker call expressions
    // Return transformed AST
    // Then: generate code ONCE from complete AST
}
```

---

#### C2. Missing Error Handling - Nil Dereference Risk
**File**: `pkg/transpiler/tuple_transform.go:114`
**Category**: Error Handling
**Issue**: Potential nil dereference when accessing `node.destructure.Value.End()`:

```go
case ast.TupleKindDestructure:
    // Generate code for tuple destructuring using AST node
    genResult = gen.GenerateDestructure(node.destructure)
    replaceStart = int(node.destructure.LetPos)
    replaceEnd = int(node.destructure.Value.End())  // ← What if Value is nil?
```

**Impact**:
- Panic if parser produces TupleDestructure with nil Value
- No error message, just crash

**Recommendation**:
```go
case ast.TupleKindDestructure:
    if node.destructure.Value == nil {
        return nil, nil, fmt.Errorf("tuple destructure at pos %d has nil value", node.destructure.LetPos)
    }
    genResult = gen.GenerateDestructure(node.destructure)
    replaceStart = int(node.destructure.LetPos)
    replaceEnd = int(node.destructure.Value.End())
```

---

#### C3. Parser Loop - Infinite Loop Risk
**File**: `pkg/transpiler/tuple_transform.go:45-63`
**Category**: Robustness
**Issue**: The parser loop relies on `continue` after `stmt == nil`, but if `ParseStatement()` keeps returning `nil` without advancing the tokenizer, this could loop forever:

```go
for {
    if stmtParser.IsAtEnd() {
        break
    }
    stmt, err := stmtParser.ParseStatement()
    if err != nil {
        break
    }
    if stmt == nil {
        // Not an error - parser returns nil for most Go statements
        continue  // ← Does this advance the parser?
    }
}
```

**Impact**:
- If `ParseStatement()` returns `(nil, nil)` repeatedly without consuming tokens, infinite loop
- No timeout or maximum iteration guard

**Recommendation**:
```go
const maxParseIterations = 100000 // Safety guard
iterations := 0

for {
    if stmtParser.IsAtEnd() {
        break
    }
    iterations++
    if iterations > maxParseIterations {
        return nil, nil, fmt.Errorf("parser exceeded max iterations at position %d", stmtParser.curToken.Pos)
    }

    // ... existing code ...
}
```

**Better approach**: Ensure `ParseStatement()` always advances tokenizer on `nil` return.

---

### IMPORTANT Issues (Should Fix)

#### I1. Missing Validation - Empty Elements
**File**: `pkg/ast/tuple.go:46-60`
**Category**: Robustness
**Issue**: `TupleLiteral.String()` doesn't validate that elements have either `Nested` or `Expr`:

```go
func (t *TupleLiteral) String() string {
    var result strings.Builder
    result.WriteString("(")
    for i, elem := range t.Elements {
        if i > 0 {
            result.WriteString(", ")
        }
        if elem.Nested != nil {
            result.WriteString(elem.Nested.String())
        } else if elem.Expr != nil {
            result.WriteString(elem.Expr.String())
        }
        // ← What if both are nil? Silent skip, produces invalid output like "(, , )"
    }
    result.WriteString(")")
    return result.String()
}
```

**Impact**:
- Produces malformed output if Element has both fields nil
- No validation at construction time

**Recommendation**:
```go
// Add validation method:
func (e *Element) Validate() error {
    if e.Nested == nil && e.Expr == nil {
        return fmt.Errorf("element must have either Nested or Expr set")
    }
    if e.Nested != nil && e.Expr != nil {
        return fmt.Errorf("element cannot have both Nested and Expr set")
    }
    return nil
}

// Call in String():
func (t *TupleLiteral) String() string {
    var result strings.Builder
    result.WriteString("(")
    for i, elem := range t.Elements {
        if err := elem.Validate(); err != nil {
            // Return error marker or panic in debug mode
            return "(INVALID_ELEMENT)"
        }
        // ... rest of code
    }
    return result.String()
}
```

---

#### I2. Parser Recovery - Unclear Error Handling
**File**: `pkg/parser/stmt.go:48-64`
**Category**: Error Handling
**Issue**: Parser uses recovery helper but the error handling is unclear:

```go
result, recovered := p.recovery.TryParse(func() (interface{}, error) {
    return p.parseStmt(), nil  // ← Always returns nil error?
}, RecoverToStatement)

if recovered {
    // Return BadStmt on recovery
    return &ast.BadStmt{From: p.curToken.Pos, To: p.curToken.Pos}, nil
}

if result == nil {
    return nil, nil
}
```

**Issues**:
- `parseStmt()` is wrapped in a function that always returns `nil` error
- Recovery mechanism seems unused since no errors are propagated
- When `recovered == true`, returns `BadStmt` but with no error message

**Impact**:
- Silent failures - user doesn't know what went wrong
- Recovery mechanism may not be functioning as intended

**Recommendation**:
1. Review if recovery mechanism is actually needed
2. If needed, propagate actual errors from `parseStmt()`
3. Add error message to BadStmt:
```go
if recovered {
    return &ast.BadStmt{
        From: p.curToken.Pos,
        To: p.curToken.Pos,
    }, fmt.Errorf("failed to parse statement at %d:%d", p.curToken.Line, p.curToken.Column)
}
```

---

#### I3. Inconsistent Pattern Validation
**File**: `pkg/parser/stmt.go:558-591`
**Category**: Error Handling
**Issue**: `parseDestructurePattern()` doesn't validate minimum pattern size:

```go
func (p *StmtParser) parseDestructurePattern() []dingoast.DestructureElement {
    p.nextToken() // consume '('

    var pattern []dingoast.DestructureElement

    for !p.curTokenIs(tokenizer.RPAREN) && !p.curTokenIs(tokenizer.EOF) {
        // ... parse elements ...
    }

    // ← Missing: check len(pattern) > 0
    // A pattern like "let () = x" would be invalid but not caught here

    return pattern
}
```

**Impact**:
- Allows empty destructuring patterns: `let () = x`
- Single-element patterns like `let (x) = y` should be errors (not tuples)
- Later codegen may produce invalid markers

**Recommendation**:
```go
if len(pattern) == 0 {
    p.addError("tuple destructure pattern cannot be empty")
    return []dingoast.DestructureElement{}
}

if len(pattern) == 1 {
    p.addError("single-element tuple is not allowed, remove parentheses")
    // Or: allow single-element for consistency with literal syntax
}
```

---

#### I4. Code Generation - Marker Collision Risk
**File**: `pkg/codegen/tuple.go:52`
**Category**: Maintainability
**Issue**: Marker names are generated based only on element count:

```go
markerName := fmt.Sprintf("__tuple%d__", elemCount)
```

**Problem**:
- `__tuple2__` could represent `(int, int)` or `(string, bool)` or any 2-element tuple
- If same source has multiple 2-element tuples with different types, how does Pass 2 distinguish them?
- No hash or unique identifier per tuple instance

**Impact**:
- Pass 2 type resolution may have ambiguity issues
- Multiple tuples of same arity but different types could collide

**Recommendation**:
1. Review Pass 2 implementation to understand type resolution strategy
2. Consider adding position-based hash to marker name:
```go
// Include position to guarantee uniqueness
markerName := fmt.Sprintf("__tuple%d_%d__", elemCount, lit.Lparen)
```
3. Or: Pass 2 should use position-based lookup, not just marker name matching

**Question**: How does `transformTuplePass2()` (line 172 in tuple_transform.go) resolve markers? This needs clarification.

---

#### I5. Missing Tests - Tuple Type Edge Cases
**File**: `pkg/codegen/tuple_test.go`
**Category**: Testing
**Issue**: Test coverage is good but missing edge cases:

**Missing test cases**:
1. **Nested tuple with nil elements**: `((nil, x), y)`
2. **Tuple with only wildcards**: `let (_, _) = x`
3. **Deeply nested tuples**: `(((a, b), (c, d)), ((e, f), (g, h)))`
4. **Tuple in error propagation**: `getTuple()?.field`
5. **Tuple in null coalesce**: `maybeTuple() ?? defaultTuple`
6. **Mixed nesting**: `(a, (b, c), d, (e, f))`

**Recommendation**: Add comprehensive edge case tests to catch corner cases.

---

### MINOR Issues (Nice to Have)

#### M1. Documentation - ToGo() Method Inconsistency
**File**: `pkg/ast/tuple.go:166-214`
**Category**: Readability
**Issue**: `TupleLiteral.ToGo()` and `TupleDestructure.ToGo()` exist in AST file but are not used by codegen:

- Line 169: `TupleLiteral.ToGo(markerName string)` - takes marker name parameter
- Line 196: `TupleDestructure.ToGo()` - no parameters
- But `pkg/codegen/tuple.go` has separate `GenerateLiteral()` and `GenerateDestructure()` methods

**Issues**:
- Duplication of logic between AST and codegen
- Unclear which method is the source of truth
- `ToGo()` in AST layer violates separation of concerns (AST should be data, not code generation)

**Impact**:
- Maintainability - two places to update when generation logic changes
- Confusion - which method should be used?

**Recommendation**:
1. **Remove `ToGo()` methods from AST layer** - codegen should be in `pkg/codegen` only
2. Or: Clearly document that `ToGo()` is deprecated/unused
3. Or: Use AST's `ToGo()` methods and remove codegen duplication

**Better architecture**:
```go
// AST layer: pure data structures, no code generation
type TupleLiteral struct {
    Lparen   token.Pos
    Elements []Element
    Rparen   token.Pos
}
// No ToGo() method here

// Codegen layer: all generation logic
func (g *TupleCodeGen) GenerateLiteral(lit *ast.TupleLiteral) ast.CodeGenResult {
    // All code generation here
}
```

---

#### M2. Naming - formatTmpVar() Logic Inconsistency
**File**: `pkg/ast/tuple.go:258-263`
**Category**: Readability
**Issue**: `formatTmpVar()` has off-by-one confusion:

```go
func formatTmpVar(counter int) string {
    if counter == 1 {
        return "tmp"
    }
    return "tmp" + formatNumber(counter)  // ← counter=2 produces "tmp2", not "tmp1"
}
```

vs. in `pkg/codegen/tuple.go:270`:
```go
func formatTmpVar(counter int) string {
    if counter == 1 {
        return "tmp"
    }
    return "tmp" + strconv.Itoa(counter-1)  // ← counter=2 produces "tmp1"
}
```

**Issue**: Two different implementations with different behaviors!

**Impact**:
- Inconsistent tmp variable naming between AST and codegen
- Counter=2 produces "tmp2" in AST but "tmp1" in codegen
- This could cause variable name conflicts

**Recommendation**:
1. **Choose one implementation** and use it everywhere
2. Based on CLAUDE.md convention (tmp, tmp1, tmp2), the codegen version is correct
3. Fix AST version:
```go
func formatTmpVar(counter int) string {
    if counter == 1 {
        return "tmp"
    }
    return "tmp" + formatNumber(counter-1)  // Add the -1
}
```

---

#### M3. Code Organization - Deleted Files Not Referenced
**File**: Git status shows deleted files
**Category**: Maintainability
**Issue**: Files `tuple_finder.go` and `tuple_finder_test.go` were deleted but:
- No migration guide or comment explaining why
- No verification that all usages were removed

**Recommendation**:
```bash
# Verify no references remain:
git grep -n "TupleFinder"
git grep -n "tuple_finder"
```

If these were replaced by parser callbacks, document this in commit message or CHANGELOG.

---

#### M4. Comments - generateNestedMarker() Unused
**File**: `pkg/ast/tuple.go:277-279`
**Category**: Code Cleanliness
**Issue**: `generateNestedMarker()` function is defined but doesn't appear to be used:

```go
// generateNestedMarker generates a unique marker name for nested tuples
func generateNestedMarker(index int) string {
    return "__NESTED_TUPLE_" + formatNumber(index) + "__"
}
```

Called at line 184 but the parameter is:
```go
nestedMarker := generateNestedMarker(i)
result.WriteString(elem.Nested.ToGo(nestedMarker))
```

But in `pkg/codegen/tuple.go:64-66`, nested tuples use recursive generation without special markers:
```go
if elem.Nested != nil {
    // Nested tuple - recursive generation
    nestedGen := NewTupleCodeGen()
    nestedResult := nestedGen.GenerateLiteral(elem.Nested)
    g.Write(string(nestedResult.Output))
}
```

**Impact**: Dead code in AST layer, increases maintenance burden

**Recommendation**: Remove `generateNestedMarker()` if unused, or document why it exists.

---

#### M5. Test - formatTmpVar Test Bug
**File**: `pkg/codegen/tuple_test.go:367-384`
**Category**: Testing
**Issue**: Test for `formatTmpVar()` has incorrect expectation:

```go
func TestFormatTmpVar(t *testing.T) {
    tests := []struct {
        counter  int
        expected string
    }{
        {1, "tmp"},
        {2, "tmp1"},
        {3, "tmp2"},
        {10, "tmp9"},  // ← This is correct: counter-1 = 9
    }
    // ...
}
```

But based on the comment in M2, the AST version produces different output. This test is testing the **codegen** version (which is correct), but the **AST** version (also called `formatTmpVar`) would fail this test.

**Recommendation**:
1. Consolidate the two `formatTmpVar()` implementations
2. Test the consolidated version
3. Ensure both AST and codegen use the same function

---

#### M6. Position Tracking - End() Method Off-By-One
**File**: `pkg/ast/tuple.go:37-39`
**Category**: Correctness
**Issue**: `TupleLiteral.End()` returns `Rparen + 1`:

```go
func (t *TupleLiteral) End() token.Pos {
    return t.Rparen + 1
}
```

**Question**: Is this consistent with Go's AST conventions?
- In `go/ast`, most nodes' `End()` returns the position **after** the last character
- `Rparen` is the position **of** the `)` character
- `Rparen + 1` would be the position after `)`

This seems correct, but should verify against Go AST convention. Similarly for `TupleDestructure.End()` (line 100-105).

**Recommendation**: Add comment explaining the +1 offset:
```go
// End returns the position after the closing paren (Go AST convention)
func (t *TupleLiteral) End() token.Pos {
    return t.Rparen + 1
}
```

---

#### M7. exprToGoCode - Marker Generation for Other Features
**File**: `pkg/codegen/tuple.go:228-256`
**Category**: Architecture
**Issue**: `exprToGoCode()` generates markers for other features (error_prop, safe_nav, null_coalesce):

```go
case *ast.ErrorPropExpr:
    // Error propagation: expr?
    // Generate marker that will be processed by error_prop feature
    operandCode := g.exprToGoCode(e.Operand)
    return fmt.Sprintf("__errorProp__(%s)", operandCode)
```

**Problem**: This creates feature coupling:
- Tuple codegen now knows about error_prop, safe_nav, null_coalesce markers
- Changes to those feature's marker format require changes here
- Violates single responsibility principle

**Impact**:
- Maintenance burden - changes ripple across features
- Testing complexity - must test tuple + error_prop interactions

**Recommendation**:
1. **Short-term**: Document this coupling clearly
2. **Long-term**: Create shared marker generation registry:
```go
// In pkg/codegen/markers.go:
type MarkerGenerator interface {
    GenerateMarker(expr ast.Expr) string
}

var markerRegistry = map[reflect.Type]MarkerGenerator{
    reflect.TypeOf(&ast.ErrorPropExpr{}): &ErrorPropMarkerGen{},
    reflect.TypeOf(&ast.SafeNavExpr{}): &SafeNavMarkerGen{},
    // ...
}

func exprToMarker(expr ast.Expr) string {
    if gen, ok := markerRegistry[reflect.TypeOf(expr)]; ok {
        return gen.GenerateMarker(expr)
    }
    return expr.String()
}
```

---

#### M8. Documentation - Pass 2 Implementation Missing
**File**: `pkg/transpiler/tuple_transform.go:172-192`
**Category**: Completeness
**Issue**: `transformTuplePass2()` calls `codegen.NewTupleTypeResolver(src)` but this type is not in the reviewed files:

```go
// Create a type resolver from the marker-infused source
resolver, err := codegen.NewTupleTypeResolver(src)
if err != nil {
    return nil, fmt.Errorf("create tuple resolver: %w", err)
}
```

**Question**: Where is `TupleTypeResolver` implemented? It's not in:
- `pkg/codegen/tuple.go`
- `pkg/ast/tuple_types.go` (only defines TupleKind enum)

**Impact**: Cannot verify correctness of Pass 2 type resolution logic

**Recommendation**:
1. If `TupleTypeResolver` is in a different file, include it in review scope
2. Document the Pass 2 architecture more clearly
3. Add tests for Pass 2 transformation

---

## 🔍 Questions

### Q1. Type Resolution Strategy
**File**: `pkg/transpiler/tuple_transform.go:172-192`
How does Pass 2 resolve tuple types from markers? Specifically:
- How does `__tuple2__(a, b)` become `Tuple2IntString{_0: a, _1: b}`?
- Where are the struct definitions generated?
- How are type names cached/deduplicated?
- What happens with complex types like `__tuple2__([]int, map[string]*User)`?

### Q2. Parser Error Recovery
**File**: `pkg/parser/stmt.go:48-64`
What is the expected behavior of `RecoveryHelper.TryParse()`?
- Does it catch panics?
- Does it advance the tokenizer on error?
- When should it return `recovered == true`?

### Q3. Deleted Files Migration
**Git status**: `tuple_finder.go` and `tuple_finder_test.go` deleted
- What was the old architecture?
- How was tuple finding done before?
- Are there any performance implications of the new parser-based approach?

### Q4. Source Mapping Strategy
**File**: `pkg/codegen/tuple.go:78-84`
How are source mappings used by LSP/gopls integration?
- Are they merged across multiple transformations?
- How do they survive byte splicing in tuple_transform.go?
- What happens when positions shift due to earlier transformations?

### Q5. Tuple Type Definitions
Where are the actual `Tuple2`, `Tuple3`, etc. struct definitions?
- Are they generated on-demand?
- Are they in a runtime package?
- How do different `.dingo` files share the same tuple types?

---

## 📊 Summary

**Overall Assessment**: **CHANGES_NEEDED**

The tuple refactoring is architecturally sound with good separation between AST, parser, and codegen layers. The code is well-tested and follows most conventions. However, there are critical issues that must be addressed:

1. **Architecture violation** (byte splicing) is self-acknowledged but needs tracking
2. **Missing nil checks** could cause panics in production
3. **Parser loop** risks infinite loops without safeguards
4. **Duplicate logic** between AST and codegen creates maintenance burden
5. **Feature coupling** in marker generation needs better abstraction

### Testability Score: **MEDIUM**

**Strengths**:
- Unit tests exist for codegen with good coverage
- Test helpers make tests readable
- Benchmark tests included

**Weaknesses**:
- Missing parser-level tuple tests
- No integration tests for full pipeline (Parse → Pass1 → Pass2)
- Edge cases not covered (nil elements, deeply nested, empty patterns)
- Pass 2 transformation not tested
- No error path testing (what happens when generation fails?)

**Recommendations for Improved Testability**:
1. Add parser tests: `TestParseTupleLiteral`, `TestParseTupleDestructure`
2. Add integration test: parse → transform → verify output compiles
3. Add negative tests: invalid syntax, malformed AST nodes
4. Mock `TupleTypeResolver` for Pass 2 testing
5. Add position tracking tests (verify mappings survive transformations)

---

## Priority Ranking

### Must Fix (Before Merge)
1. **C2**: Add nil checks in `transformTuplePass1()` (line 114)
2. **C3**: Add infinite loop guard in parser loop (line 45-63)
3. **I3**: Validate destructure pattern size (empty/single-element)
4. **M2**: Fix `formatTmpVar()` inconsistency between AST and codegen

### Should Fix (Before v1.0)
1. **C1**: Create GitHub issue for byte splicing technical debt
2. **I1**: Add Element validation in `TupleLiteral.String()`
3. **I2**: Clarify parser recovery error handling
4. **I4**: Review marker collision risk with Pass 2 maintainer
5. **M1**: Remove `ToGo()` methods from AST layer or document deprecation

### Nice to Have (Future)
1. **I5**: Add comprehensive edge case tests
2. **M3**: Document deleted files migration
3. **M4**: Remove unused `generateNestedMarker()`
4. **M7**: Create shared marker generation registry
5. **M8**: Document Pass 2 architecture

---

## Code Examples

### Example Fix for C2 (Nil Check):
```go
case ast.TupleKindDestructure:
    // Validate destructure node
    if node.destructure == nil {
        continue // Skip nil nodes
    }
    if node.destructure.Value == nil {
        return nil, nil, fmt.Errorf(
            "tuple destructure at %d has nil value expression",
            node.destructure.LetPos,
        )
    }

    // Generate code
    genResult = gen.GenerateDestructure(node.destructure)
    replaceStart = int(node.destructure.LetPos)
    replaceEnd = int(node.destructure.Value.End())

    // Make it a valid Go statement
    genResult.Output = append([]byte("_ = "), genResult.Output...)
```

### Example Fix for I3 (Pattern Validation):
```go
func (p *StmtParser) parseDestructurePattern() []dingoast.DestructureElement {
    p.nextToken() // consume '('

    var pattern []dingoast.DestructureElement

    for !p.curTokenIs(tokenizer.RPAREN) && !p.curTokenIs(tokenizer.EOF) {
        // ... existing parsing logic ...
    }

    // Validate pattern
    if len(pattern) == 0 {
        p.addError("tuple destructure pattern cannot be empty")
        return []dingoast.DestructureElement{}
    }

    if len(pattern) == 1 {
        p.addError("single-element tuple not allowed (remove parentheses or add comma)")
        // Note: Some languages allow (x,) for single-element tuple
    }

    return pattern
}
```

### Example Test for Edge Case (Missing):
```go
func TestTupleCodeGen_DeepNesting(t *testing.T) {
    // Test: (((a, b), (c, d)), ((e, f), (g, h)))
    // Ensures recursive generation handles arbitrary depth

    inner1 := &ast.TupleLiteral{
        Elements: []ast.Element{
            {Expr: rawExpr("a")},
            {Expr: rawExpr("b")},
        },
    }

    inner2 := &ast.TupleLiteral{
        Elements: []ast.Element{
            {Expr: rawExpr("c")},
            {Expr: rawExpr("d")},
        },
    }

    middle1 := &ast.TupleLiteral{
        Elements: []ast.Element{
            {Nested: inner1},
            {Nested: inner2},
        },
    }

    // ... similar for middle2 ...

    outer := &ast.TupleLiteral{
        Elements: []ast.Element{
            {Nested: middle1},
            {Nested: middle2},
        },
    }

    gen := NewTupleCodeGen()
    result := gen.GenerateLiteral(outer)

    expected := "__tuple2__(__tuple2__(__tuple2__(a, b), __tuple2__(c, d)), __tuple2__(__tuple2__(e, f), __tuple2__(g, h)))"

    if string(result.Output) != expected {
        t.Errorf("Expected:\n%s\nGot:\n%s", expected, string(result.Output))
    }
}
```

---

**End of Review**
