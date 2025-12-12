# Dingo Migration Completion Plan: Phases 2-7

**Date**: 2025-11-17
**Session**: 20251117-181304
**Context**: Continuation from session 20251117-154457
**Status**: Infrastructure complete (Phase 0-1), implementing features (Phase 2-7)

---

## Executive Summary

The foundation is ready. Infrastructure built in the previous session includes:
- ✅ Source map tracking (`pkg/preprocessor/sourcemap.go`)
- ✅ Preprocessor framework (`pkg/preprocessor/preprocessor.go`)
- ✅ go/parser wrapper (`pkg/parser/parser_new.go`)
- ✅ Transformer framework (`pkg/transform/transformer.go`)
- ✅ Error propagation skeleton (`pkg/preprocessor/error_prop.go`)

**Goal**: Implement all remaining features to pass 46 golden tests.

**Timeline**: 6-8 development sessions (~12-16 hours)

---

## Test Inventory by Feature

Total: 46 golden tests across 11 feature categories

| Feature | Count | Files |
|---------|-------|-------|
| Error Propagation (`?`) | 8 | error_prop_01 through error_prop_08 |
| Lambdas (`\|x\| expr`) | 4 | lambda_01 through lambda_04 |
| Sum Types (`enum`) | 5 | sum_types_01 through sum_types_05 |
| Pattern Matching (`match`) | 4 | pattern_match_01 through pattern_match_04 |
| Result Type | 5 | result_01 through result_05 |
| Option Type | 4 | option_01 through option_04 |
| Functional Utilities | 4 | func_util_01 through func_util_04 |
| Ternary Operator (`? :`) | 3 | ternary_01 through ternary_03 |
| Null Coalescing (`??`) | 3 | null_coalesce_01 through null_coalesce_03 |
| Safe Navigation (`?.`) | 3 | safe_nav_01 through safe_nav_03 |
| Tuples | 3 | tuples_01 through tuples_03 |

---

## Phase Breakdown

### Phase 2: Error Propagation (`?` operator) - 8 tests

**Estimated Time**: 2-3 hours

**Status**: Skeleton exists, needs completion

**Goal**: Transform `expr?` into proper error handling code

#### 2.1: Complete Preprocessor (1 hour)

**File**: `pkg/preprocessor/error_prop.go`

**Current State**: Basic structure, needs two-pass implementation

**Tasks**:
1. Implement expression boundary detection
   - Scan backwards from `?` to find expression start
   - Handle nested parens, method calls, array access
   - Edge cases: `fetchData(url)?`, `obj.Method()?`, `arr[i]?`

2. Generate placeholder with counter
   - Pattern: `__dingo_try_N__(expr)`
   - Increment counter for each occurrence
   - Track original position for source map

3. Source map entries
   - Record position of `?` in original
   - Record position of `__dingo_try_N__(` in preprocessed
   - Length includes full transformation

**Example Transformation**:
```dingo
// Input
data := ReadFile(path)?

// Preprocessed
data := __dingo_try_1__(ReadFile(path))
```

**Test Strategy**: Unit test with simple expressions first

#### 2.2: Complete Transformer (1.5 hours)

**File**: `pkg/transform/transformer.go` (update `transformErrorProp`)

**Current State**: Skeleton with placeholder detection

**Tasks**:
1. Detect `__dingo_try_N__` calls in AST
2. Analyze context:
   - Assignment statement: `x := expr?`
   - Return statement: `return expr?`
   - If condition: `if validate(x)? { ... }`
   - Expression: `len(data?) + 1`

3. Generate error handling code based on context:

**Assignment Context**:
```go
// Input: data := ReadFile(path)?
// Generate:
__tmp_1, __err_1 := ReadFile(path)
if __err_1 != nil {
    return __zero, __err_1  // Return zero value + error
}
data := __tmp_1
```

**Return Context**:
```go
// Input: return process(x)?
// Generate:
__tmp_1, __err_1 := process(x)
if __err_1 != nil {
    return __err_1
}
return __tmp_1
```

**Expression Context**:
```go
// Input: count := len(data?) + 1
// Generate:
__tmp_1, __err_1 := data
if __err_1 != nil {
    return 0, __err_1
}
count := len(__tmp_1) + 1
```

4. Determine function return type
   - Walk up AST to find enclosing function
   - Check if function returns `error` as last parameter
   - If not, emit compile error (can't use ? in non-error-returning function)

5. Generate zero values for return types
   - Use type information from go/types
   - Generate proper zero value for each return type

**Test Strategy**: Test each context independently, then combined

#### 2.3: Golden Test Validation (0.5 hours)

**Tests to Pass** (in order of complexity):
1. `error_prop_01_simple.dingo` - Basic assignment
2. `error_prop_02_multiple.dingo` - Multiple ? in one function
3. `error_prop_03_expression.dingo` - ? in expressions
4. `error_prop_04_wrapping.dingo` - Nested function calls
5. `error_prop_05_complex_types.dingo` - Complex return types
6. `error_prop_06_mixed_context.dingo` - Different contexts
7. `error_prop_07_special_chars.dingo` - Edge cases
8. `error_prop_08_chained_calls.dingo` - Method chaining

**Success Criteria**: All 8 tests produce matching .go.golden output

---

### Phase 3: Lambdas (`|x| expr`) - 4 tests

**Estimated Time**: 3-4 hours

**Goal**: Transform lambda syntax to Go function literals with type inference

#### 3.1: Implement Lambda Preprocessor (1.5 hours)

**File**: Create `pkg/preprocessor/lambdas.go`

**Tasks**:
1. Scan for `|` tokens
2. Parse parameter list: `|x|`, `|x, y|`, `||` (no params)
3. Find lambda body (expression or block)
   - Single expression: `|x| x * 2`
   - Block: `|x| { return x * 2 }` (if supported)
4. Generate placeholder:
   ```go
   __dingo_lambda_N__([]string{"x", "y"}, func() interface{} { return <body> })
   ```

**Edge Cases**:
- Empty params: `|| doSomething()`
- Single param: `|x| expr`
- Multiple params: `|x, y, z| expr`
- Body with commas: `|x| foo(a, b, c)`

**Example**:
```dingo
// Input
add := |a, b| a + b

// Preprocessed
add := __dingo_lambda_1__([]string{"a", "b"}, func() interface{} { return a + b })
```

#### 3.2: Implement Lambda Transformer with Type Inference (1.5 hours)

**File**: `pkg/transform/transformer.go` (add `transformLambda`)

**Critical Challenge**: Type inference

**Strategy**:
1. Find `__dingo_lambda_N__` call
2. Walk up AST to find context (assignment, function call, etc.)
3. Use go/types to determine expected function type
4. Extract parameter types and return type
5. Rebuild function literal with proper signature

**Type Inference Examples**:

```go
// Context: Assignment with explicit type
var adder func(int, int) int = |a, b| a + b
// Infer: a, b are int, returns int

// Context: Function parameter
numbers.Map(|x| x * 2)
// Look up Map signature: func Map(f func(T) T)
// Infer: x is T (int if numbers is []int)

// Context: Return statement
func makeAdder() func(int, int) int {
    return |a, b| a + b
}
// Infer from return type
```

**Implementation Steps**:
1. Extract expected type from context:
   ```go
   func (t *Transformer) inferLambdaType(cursor *astutil.Cursor) *types.Signature {
       // Walk up to find assignment/call
       // Use typeInfo.TypeOf() on context
       // Extract function signature
   }
   ```

2. Build typed function literal:
   ```go
   func (t *Transformer) rebuildLambda(params []string, body ast.Expr, sig *types.Signature) *ast.FuncLit {
       // Create ast.FuncType with proper parameter/return types
       // Rebuild body with parameter identifiers
   }
   ```

**Fallback**: If type inference fails, require explicit types or emit error

#### 3.3: Golden Test Validation (1 hour)

**Tests** (ordered by complexity):
1. `lambda_01_basic.dingo` - Simple lambdas
2. `lambda_02_multiline.dingo` - Multi-line bodies (if supported)
3. `lambda_03_closure.dingo` - Capturing variables
4. `lambda_04_higher_order.dingo` - Nested lambdas

**Success Criteria**: All 4 lambda tests pass + 4 func_util tests (use lambdas)

---

### Phase 4: Sum Types (`enum`) - 5 tests

**Estimated Time**: 3-4 hours

**Goal**: Transform enum declarations to Go tagged unions

#### 4.1: Implement Sum Types Preprocessor (1.5 hours)

**File**: Create `pkg/preprocessor/sum_types.go`

**Tasks**:
1. Detect `enum` keyword
2. Parse enum name and variants:
   ```dingo
   enum Status {
       Pending,
       Active,
       Complete,
   }

   enum Result {
       Ok(float64),
       Err(error),
   }
   ```

3. Generate placeholder type definition:
   ```go
   type __dingo_enum_Status__ struct {
       __dingo_tag__ string
       __dingo_data__ interface{}
   }

   var __dingo_enum_def_Status__ = map[string][]string{
       "Pending": nil,
       "Active": nil,
       "Complete": nil,
   }
   ```

4. For variants with data:
   ```go
   var __dingo_enum_def_Result__ = map[string][]string{
       "Ok": {"float64"},
       "Err": {"error"},
   }
   ```

#### 4.2: Implement Sum Types Transformer (1.5 hours)

**File**: `pkg/transform/transformer.go` (add `transformSumType`)

**Tasks**:
1. Detect `__dingo_enum_` type definitions
2. Parse variant metadata from `__dingo_enum_def_`
3. Generate full tagged union implementation:

**For simple enum (no data)**:
```go
type Status int

const (
    Status_Pending Status = iota
    Status_Active
    Status_Complete
)

func (s Status) IsPending() bool { return s == Status_Pending }
func (s Status) IsActive() bool { return s == Status_Active }
func (s Status) IsComplete() bool { return s == Status_Complete }
```

**For enum with data (tagged union)**:
```go
type Result struct {
    tag    string
    ok_0   *float64
    err_0  *error
}

func Result_Ok(v float64) Result {
    return Result{tag: "Ok", ok_0: &v}
}

func Result_Err(v error) Result {
    return Result{tag: "Err", err_0: &v}
}

func (r Result) IsOk() bool { return r.tag == "Ok" }
func (r Result) IsErr() bool { return r.tag == "Err" }
```

**Reference**: Can reuse logic from old `plugins/sum_types.go` (926 lines)

#### 4.3: Golden Test Validation (1 hour)

**Tests**:
1. `sum_types_01_simple.dingo` - Simple enum (no data)
2. `sum_types_02_struct_variant.dingo` - Variants with data
3. `sum_types_03_generic.dingo` - Generic sum types (if supported)
4. `sum_types_04_multiple.dingo` - Multiple enums
5. `sum_types_05_nested.dingo` - Nested variants

**Success Criteria**: All 5 sum type tests pass

---

### Phase 5: Pattern Matching (`match`) - 4 tests

**Estimated Time**: 3-4 hours

**Goal**: Transform match expressions to switch statements

#### 5.1: Implement Pattern Match Preprocessor (1.5 hours)

**File**: Create `pkg/preprocessor/pattern_match.go`

**Tasks**:
1. Detect `match` keyword
2. Parse scrutinee (value being matched)
3. Parse patterns and handlers:
   ```dingo
   match result {
       Ok(v) => println("Success:", v),
       Err(e) => println("Error:", e),
   }
   ```

4. Generate placeholder:
   ```go
   __dingo_match_1__(result, map[string]func(...interface{}) interface{}{
       "Ok": func(__binding_v interface{}) interface{} { return println("Success:", __binding_v) },
       "Err": func(__binding_e interface{}) interface{} { return println("Error:", __binding_e) },
   })
   ```

**Pattern Types**:
- Variant matching: `Ok(v)`
- Literal matching: `42`, `"hello"`
- Wildcard: `_`
- Guards: `x if x > 10` (optional)

#### 5.2: Implement Pattern Match Transformer (1.5 hours)

**File**: `pkg/transform/transformer.go` (add `transformPatternMatch`)

**Tasks**:
1. Detect `__dingo_match_N__` calls
2. Determine match type (sum type vs value match)
3. Generate appropriate switch:

**For sum types (type switch)**:
```go
switch result.tag {
case "Ok":
    v := *result.ok_0
    println("Success:", v)
case "Err":
    e := *result.err_0
    println("Error:", e)
}
```

**For value match (regular switch)**:
```go
switch value {
case 42:
    println("The answer")
case 100:
    println("Century")
default:
    println("Other")
}
```

4. Handle exhaustiveness checking (optional but recommended)

**Reference**: Can reuse logic from `plugins/pattern_match.go`

#### 5.3: Golden Test Validation (1 hour)

**Tests**:
1. `pattern_match_01_basic.dingo` - Basic patterns
2. `pattern_match_02_guards.dingo` - Pattern guards (if supported)
3. `pattern_match_03_nested.dingo` - Nested patterns
4. `pattern_match_04_exhaustive.dingo` - Exhaustiveness checking

**Success Criteria**: All 4 pattern matching tests pass

---

### Phase 6: Operators (Ternary, ??, ?.) - 9 tests

**Estimated Time**: 2-3 hours

**Goal**: Implement simple operator transformations

#### 6.1: Ternary Operator (`? :`) - 3 tests

**File**: Create `pkg/preprocessor/operators.go`

**Strategy**: Full expansion in preprocessor (no transformer needed)

**Input**:
```dingo
status := age >= 18 ? "adult" : "minor"
```

**Preprocessed Output**:
```go
status := func() string {
    if age >= 18 {
        return "adult"
    } else {
        return "minor"
    }
}()
```

**Challenge**: Type inference for return type
- Scan both branches
- Use go/types to determine common type
- Or require both branches to have same type

**Tests**:
1. `ternary_01_basic.dingo`
2. `ternary_02_nested.dingo`
3. `ternary_03_complex.dingo`

#### 6.2: Null Coalescing (`??`) - 3 tests

**Input**:
```dingo
value := maybeNil ?? defaultValue
```

**Preprocessed Output**:
```go
value := func() T {
    if __tmp := maybeNil; __tmp != nil {
        return __tmp
    } else {
        return defaultValue
    }
}()
```

**Tests**:
1. `null_coalesce_01_basic.dingo`
2. `null_coalesce_02_chained.dingo`
3. `null_coalesce_03_with_option.dingo`

#### 6.3: Safe Navigation (`?.`) - 3 tests

**Input**:
```dingo
name := user?.profile?.name
```

**Preprocessed Output**:
```go
name := __dingo_safe_nav_1__(user, "profile", "name")
```

**Transformer**: Generate nil checks
```go
var name *string
if user != nil {
    if user.profile != nil {
        name = &user.profile.name
    }
}
```

**Tests**:
1. `safe_nav_01_basic.dingo`
2. `safe_nav_02_chained.dingo`
3. `safe_nav_03_with_methods.dingo`

**Time Estimate**: 1 hour per operator group

---

### Phase 7: Result/Option Integration - 9 tests

**Estimated Time**: 2-3 hours

**Goal**: Implement Result and Option as special sum types

**Note**: Result and Option are just sum types with special names. Can reuse Phase 4 infrastructure.

#### 7.1: Result Type - 5 tests

**Definition** (built-in):
```dingo
enum Result {
    Ok(T),
    Err(error),
}
```

**Generated** (same as sum types):
```go
type Result_T_error struct {
    tag   string
    ok_0  *T
    err_0 *error
}
// + constructors and methods
```

**Tests**:
1. `result_01_basic.dingo`
2. `result_02_propagation.dingo` - With `?` operator
3. `result_03_pattern_match.dingo` - With pattern matching
4. `result_04_chaining.dingo` - Method chaining
5. `result_05_go_interop.dingo` - Convert from (T, error)

#### 7.2: Option Type - 4 tests

**Definition** (built-in):
```dingo
enum Option {
    Some(T),
    None,
}
```

**Tests**:
1. `option_01_basic.dingo`
2. `option_02_pattern_match.dingo`
3. `option_03_chaining.dingo`
4. `option_04_go_interop.dingo`

**Strategy**:
- Treat as special case of sum types
- May add convenience methods (`.unwrap()`, `.unwrapOr()`, etc.)

---

### Phase 8: Tuples - 3 tests

**Estimated Time**: 1-2 hours

**Goal**: Transform tuple syntax to Go structs

**Note**: May defer to later if complex

**Input**:
```dingo
pair := (42, "hello")
let (x, y) = pair
```

**Strategy**: Generate anonymous struct
```go
pair := struct {
    _0 int
    _1 string
}{42, "hello"}
x, y := pair._0, pair._1
```

**Tests**:
1. `tuples_01_basic.dingo`
2. `tuples_02_destructure.dingo`
3. `tuples_03_nested.dingo`

**Priority**: Low (can defer)

---

### Phase 9: CLI Integration & Testing

**Estimated Time**: 2-3 hours

**Goal**: Wire everything together, ensure end-to-end workflow

#### 9.1: Update CLI (1 hour)

**File**: `cmd/dingo/build.go`

**Tasks**:
1. Update to use new parser
   ```go
   import (
       "github.com/yourusername/dingo/pkg/parser"
       "github.com/yourusername/dingo/pkg/transform"
   )

   func buildFile(path string) error {
       source, _ := os.ReadFile(path)

       // Parse with new architecture
       p := parser.NewGoParser()
       result, err := p.ParseFile(path, source)
       if err != nil {
           return err // Errors already mapped to original positions
       }

       // Transform
       transformer := transform.New(result.FileSet, result.SourceMap)
       if err := transformer.Transform(result.AST); err != nil {
           return err
       }

       // Generate
       outputPath := strings.Replace(path, ".dingo", ".go", 1)
       if err := generator.Generate(result.AST, outputPath); err != nil {
           return err
       }

       return nil
   }
   ```

2. Remove old parser imports
3. Test with `dingo build tests/golden/error_prop_01_simple.dingo`

#### 9.2: Run Full Golden Test Suite (1 hour)

**Script**: Create `tests/golden/run_all.sh`
```bash
#!/bin/bash
set -e

for file in tests/golden/*.dingo; do
    echo "Testing: $file"
    dingo build "$file"

    # Compare output
    golden="${file%.dingo}.go.golden"
    generated="${file%.dingo}.go"

    if diff -u "$golden" "$generated"; then
        echo "✓ PASS: $file"
    else
        echo "✗ FAIL: $file"
        exit 1
    fi
done

echo "All 46 tests passed!"
```

**Iterate**: Fix failures one by one

#### 9.3: Compile Generated Code (0.5 hours)

**Ensure all generated .go files compile**:
```bash
for file in tests/golden/*.go; do
    go build -o /dev/null "$file" || echo "FAIL: $file"
done
```

#### 9.4: Error Message Quality (0.5 hours)

**Test error reporting**:
1. Introduce syntax error in .dingo file
2. Verify error points to correct line/column in .dingo (not preprocessed)
3. Ensure error message is helpful

**Example**:
```
error_prop_01_simple.dingo:4:18: cannot use ? operator in function that doesn't return error
```

---

### Phase 10: Documentation & Polish

**Estimated Time**: 1-2 hours

#### 10.1: Update Documentation

**Files to update**:
1. `CLAUDE.md`:
   - Update "Current Phase" to "Phase 3: Complete"
   - Document new architecture
   - Remove old Participle references

2. `CHANGELOG.md`:
   ```markdown
   ## [Unreleased]

   ### Changed
   - **BREAKING**: Complete rewrite of transpiler using go/parser + preprocessor architecture
   - Replaced Participle parser with standard go/parser
   - Implemented preprocessor layer for Dingo syntax transformation
   - All 46 golden tests passing with new architecture

   ### Added
   - Source map infrastructure for error position mapping
   - Modular preprocessor framework for feature composition
   - AST transformation layer with type inference support

   ### Removed
   - Participle parser dependency (~11,494 lines)
   - Custom AST types (now use go/ast)
   - Old plugin system (replaced by preprocessor + transformer)
   ```

3. `README.md`:
   - Update "How It Works" section
   - Mention preprocessor approach
   - Update architecture diagram

#### 10.2: Code Cleanup

1. Remove commented-out code
2. Remove debug print statements
3. Add package documentation comments
4. Ensure all exported functions have doc comments
5. Run `gofmt` and `goimports`

#### 10.3: Performance Benchmarks

**Create**: `pkg/preprocessor/preprocessor_bench_test.go`

Test preprocessing performance:
```go
func BenchmarkPreprocessor(b *testing.B) {
    source := readTestFile("error_prop_08_chained_calls.dingo")

    for i := 0; i < b.N; i++ {
        p := preprocessor.New(source)
        _, _, _ = p.Process()
    }
}
```

**Target**: < 1ms for typical file

---

## Implementation Order & Dependencies

### Critical Path

```
Phase 2 (Error Prop) → REQUIRED for basic functionality
    ↓
Phase 3 (Lambdas) → REQUIRED for func_util tests
    ↓
Phase 4 (Sum Types) → REQUIRED for Result/Option
    ↓
Phase 7 (Result/Option) → Depends on sum types
    ↓
Phase 5 (Pattern Match) → Enhanced by sum types
    ↓
Phase 6 (Operators) → Independent, can be parallel
    ↓
Phase 9 (CLI Integration) → After all features work
    ↓
Phase 10 (Documentation) → Final polish
```

### Parallel Opportunities

Can work in parallel:
- Phase 6 (Operators) can be done anytime after Phase 2
- Phase 8 (Tuples) is independent (low priority)

### Sequential Dependencies

Must be sequential:
- Phase 4 before Phase 7 (Result/Option need sum types)
- Phase 3 before func_util tests work
- Phase 9 after all features (integration requires completeness)

---

## Risk Assessment & Mitigation

### High Risk Items

1. **Lambda Type Inference Complexity**
   - Risk: go/types integration may be tricky
   - Mitigation: Start with simple cases, add explicit type syntax fallback
   - Fallback: Require type annotations if inference fails

2. **Pattern Matching Edge Cases**
   - Risk: Nested patterns, guards, exhaustiveness
   - Mitigation: Implement basic patterns first, defer advanced features
   - Fallback: Simplify pattern syntax if needed (pre-release, can change)

3. **Source Map Accuracy**
   - Risk: Errors may point to wrong positions
   - Mitigation: Extensive testing, manual verification
   - Fallback: Add debug mode showing preprocessed code

### Medium Risk Items

4. **Expression Boundary Detection (? operator)**
   - Risk: Edge cases in complex expressions
   - Mitigation: Comprehensive unit tests, fuzzing
   - Fallback: Require parentheses for ambiguous cases

5. **Sum Type Generic Support**
   - Risk: Generics may complicate code generation
   - Mitigation: Start with non-generic, add generics incrementally
   - Fallback: Defer generics to v1.1

### Low Risk Items

6. **Operator Precedence**
   - Risk: Ternary/null coalescing precedence issues
   - Mitigation: Use parentheses liberally in generated code
   - Fallback: Document required parentheses in user code

---

## Testing Strategy

### Unit Tests (per feature)

Create test files alongside each component:
- `pkg/preprocessor/error_prop_test.go`
- `pkg/preprocessor/lambdas_test.go`
- `pkg/transform/transformer_test.go`

**Test cases**:
- Simple input/output pairs
- Edge cases
- Error cases
- Source map accuracy

### Integration Tests (golden tests)

Use existing 46 golden tests as integration validation:
- Each feature must pass its golden tests before moving to next phase
- Run incrementally (don't wait for all features)
- Compare both:
  - Generated Go code (diff with .go.golden)
  - Compiled output (go build succeeds)

### Regression Tests

After all tests pass:
- Save current outputs as baseline
- Any future changes must not regress tests
- Use git diff to track output changes

---

## Success Criteria

### Per-Phase Criteria

Each phase complete when:
- [ ] Preprocessor generates valid Go code
- [ ] Transformer produces correct AST
- [ ] All golden tests for that feature pass
- [ ] Generated code compiles
- [ ] Unit tests pass (if written)

### Overall Success Criteria

Migration complete when:
- [ ] All 46 golden tests pass
- [ ] Generated .go files compile
- [ ] `dingo build` CLI works end-to-end
- [ ] Error messages point to .dingo files (not preprocessed)
- [ ] Documentation updated (CLAUDE.md, CHANGELOG.md, README.md)
- [ ] No Participle dependencies in go.mod
- [ ] Code is clean (no TODOs, no debug prints)
- [ ] Performance is acceptable (< 100ms per file)

---

## Timeline Estimate

### Optimistic (16 hours)

- Phase 2: 2 hours
- Phase 3: 3 hours
- Phase 4: 3 hours
- Phase 5: 3 hours
- Phase 6: 2 hours
- Phase 7: 1 hour
- Phase 8: 1 hour (deferred)
- Phase 9: 2 hours
- Phase 10: 1 hour

**Total**: ~16 hours (4 sessions @ 4 hours each)

### Realistic (24 hours)

Account for:
- Debugging edge cases (+25%)
- Type inference challenges (+25%)
- Testing iterations (+25%)
- Documentation (+25%)

**Total**: ~24 hours (6 sessions @ 4 hours each)

### Conservative (32 hours)

Account for:
- Unexpected architectural issues
- Refactoring
- Learning go/types deeply
- Extensive testing

**Total**: ~32 hours (8 sessions @ 4 hours each)

**Recommendation**: Plan for realistic (24 hours), hope for optimistic (16 hours)

---

## Session Planning

### Session 1: Error Propagation (Phase 2)
- Complete error_prop.go preprocessor
- Complete transformErrorProp in transformer
- Pass all 8 error propagation tests
- **Deliverable**: Basic transpilation works

### Session 2: Lambdas (Phase 3)
- Implement lambdas.go preprocessor
- Implement type inference transformer
- Pass 4 lambda tests + 4 func_util tests
- **Deliverable**: Advanced type inference works

### Session 3: Sum Types (Phase 4)
- Implement sum_types.go preprocessor
- Implement sum type transformer
- Pass 5 sum type tests
- **Deliverable**: Complex code generation works

### Session 4: Pattern Matching (Phase 5)
- Implement pattern_match.go preprocessor
- Implement match transformer
- Pass 4 pattern matching tests
- **Deliverable**: Control flow transformation works

### Session 5: Operators + Result/Option (Phases 6-7)
- Implement operators.go (ternary, ??, ?.)
- Validate Result/Option work (use sum types)
- Pass 9 operator tests + 9 Result/Option tests
- **Deliverable**: All syntax features complete

### Session 6: Integration & Polish (Phases 9-10)
- Update CLI
- Run full test suite (46 tests)
- Fix any failures
- Update documentation
- **Deliverable**: Migration complete

**Buffer**: Sessions 7-8 for unexpected issues

---

## Next Immediate Steps

### Start with Phase 2 (Error Propagation)

**Session Plan**:
1. Read error_prop.go skeleton (already exists)
2. Implement two-pass transformation
3. Add unit tests for expression boundary detection
4. Implement transformErrorProp in transformer
5. Test with error_prop_01_simple.dingo
6. Iterate through remaining 7 tests
7. Commit and document progress

**Estimated Time**: 2-3 hours

**Blocking Issues**: None (infrastructure ready)

**Resources Needed**:
- go/ast documentation
- go/types documentation (for return type analysis)
- Golden test files as specification

---

## Open Questions & Decisions Needed

### 1. Lambda Syntax for Multi-line Bodies
**Question**: Support block syntax `|x| { ... }` or only expressions `|x| expr`?
**Recommendation**: Start with expressions only, add blocks if needed
**Impact**: Low (most lambdas are single expression)

### 2. Pattern Matching Guards
**Question**: Support guards like `x if x > 10`?
**Recommendation**: Defer to Phase 5.2 implementation, skip if complex
**Impact**: Medium (nice-to-have, not critical)

### 3. Generic Sum Types
**Question**: Support `enum Result[T, E]`?
**Recommendation**: Test with sum_types_03_generic.dingo, implement if not too complex
**Impact**: High (Result/Option need generics)

### 4. Tuple Priority
**Question**: Implement tuples or defer?
**Recommendation**: Defer to Phase 8, low priority
**Impact**: Low (3 tests, not blocking other features)

### 5. Operator Precedence Rules
**Question**: What's the precedence of ?, ??, ?. vs other operators?
**Recommendation**: Document and enforce with parentheses if ambiguous
**Impact**: Low (can clarify in docs)

---

## Appendix: File Structure Reference

### Current State (After Phase 1)

```
pkg/
├── preprocessor/
│   ├── sourcemap.go       ✅ (107 lines)
│   ├── preprocessor.go    ✅ (212 lines)
│   └── error_prop.go      🚧 (95 lines, skeleton)
├── parser/
│   └── parser_new.go      ✅ (59 lines)
└── transform/
    └── transformer.go     ✅ (177 lines)
```

### Target State (After Phase 10)

```
pkg/
├── preprocessor/
│   ├── sourcemap.go           ✅ (existing)
│   ├── preprocessor.go        ✅ (existing)
│   ├── error_prop.go          🎯 (complete implementation)
│   ├── lambdas.go             🎯 (new)
│   ├── sum_types.go           🎯 (new)
│   ├── pattern_match.go       🎯 (new)
│   ├── operators.go           🎯 (new, handles ternary/??/?.)
│   └── preprocessor_test.go  🎯 (unit tests)
├── parser/
│   ├── parser_new.go          ✅ (existing, minor updates)
│   └── errors.go              🎯 (error mapping utilities)
├── transform/
│   ├── transformer.go         ✅ (existing, expand with feature transforms)
│   ├── type_inference.go      🎯 (new, shared type utilities)
│   ├── context.go             🎯 (new, context analysis)
│   └── transform_test.go      🎯 (unit tests)
└── generator/
    └── generator.go           🎯 (refactor existing)
```

**Total New Files**: ~8 files
**Total Lines Estimate**: ~2000-3000 lines (vs 11,494 deleted)

---

## Summary

The migration is well-scoped and achievable. Infrastructure is complete, features are modular and can be implemented incrementally. Each phase has clear success criteria (golden tests), and we can validate progress continuously.

**Confidence Level**: High
**Risk Level**: Medium (type inference is the main unknown)
**Timeline**: 16-32 hours (realistic: 24 hours)

**Recommendation**: Proceed with Phase 2 (Error Propagation) immediately.
