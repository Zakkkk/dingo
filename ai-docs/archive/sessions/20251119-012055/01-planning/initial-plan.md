# Phase 4 Priority 2 & 3 Implementation Plan

## Executive Summary

This plan addresses 4 critical type inference gaps and guard support to complete Phase 4.2 pattern matching enhancements. The implementation leverages existing infrastructure (parent tracking, go/types integration) that is already in place but not fully utilized.

**Key Insight**: All required infrastructure exists (parent map, go/types integration, context traversal). The TODO functions are stubs waiting for implementation, not architectural gaps.

---

## Architecture Overview

### Current State Analysis

**Existing Infrastructure (Already Implemented)**:
1. ✅ Parent tracking system (`pkg/plugin/plugin.go:379-381`)
2. ✅ go/types integration hooks (`TypeInferenceService.SetTypesInfo()`)
3. ✅ Context traversal logic (`InferTypeFromContext()` lines 580-639)
4. ✅ Type registry for Result/Option types
5. ✅ None constant inference framework (5/9 contexts working)

**Missing Implementations (Stubs Only)**:
1. ❌ `findFunctionReturnType()` - Line 645 (TODO)
2. ❌ `findAssignmentType()` - Line 651 (TODO)
3. ❌ `findVarDeclType()` - Line 657 (TODO)
4. ❌ `findCallArgType()` - Line 663 (TODO)
5. ❌ Pattern match scrutinee type detection - Line 498 (heuristic only)
6. ❌ Err() context inference - Line 286 (uses `interface{}`)
7. ❌ Guard validation in if-else transformation - Lines 826, 1009

### Architectural Components

```
┌──────────────────────────────────────────────────────────┐
│ TypeInferenceService (pkg/plugin/builtin/               │
│                       type_inference.go)                 │
├──────────────────────────────────────────────────────────┤
│ • InferTypeFromContext() [COMPLETE]                      │
│   - Parent chain traversal ✅                            │
│   - Context detection ✅                                 │
│   - Delegation to helpers ✅                             │
│                                                          │
│ • Helper Functions [TODO - THIS PLAN]                    │
│   - findFunctionReturnType() → Implement                │
│   - findAssignmentType() → Implement                     │
│   - findVarDeclType() → Implement                        │
│   - findCallArgType() → Implement                        │
│                                                          │
│ • Infrastructure [COMPLETE]                              │
│   - parentMap: map[ast.Node]ast.Node ✅                  │
│   - typesInfo: *types.Info ✅                            │
│   - SetParentMap() ✅                                    │
│   - SetTypesInfo() ✅                                    │
└──────────────────────────────────────────────────────────┘
         │                      │                   │
         │ Used by              │ Used by           │ Used by
         ↓                      ↓                   ↓
┌──────────────────┐  ┌───────────────────┐  ┌─────────────────┐
│ NoneContextPlugin│  │ PatternMatchPlugin│  │ ResultTypePlugin│
│ (none_context.go)│  │ (pattern_match.go)│  │ (result_type.go)│
├──────────────────┤  ├───────────────────┤  ├─────────────────┤
│ Task 1 Consumer  │  │ Task 2 Target     │  │ Task 3 Target   │
│ None inference   │  │ Scrutinee type    │  │ Err() inference │
└──────────────────┘  └───────────────────┘  └─────────────────┘
```

---

## Task 1: Implement 4 Missing Context Helper Functions

**Location**: `pkg/plugin/builtin/type_inference.go`

### Task 1.1: Implement `findFunctionReturnType()`

**Goal**: Extract return type from function declaration containing a return statement.

**Implementation Strategy**:
```go
func (s *TypeInferenceService) findFunctionReturnType(retStmt *ast.ReturnStmt) types.Type {
    // Walk up parent chain to find *ast.FuncDecl or *ast.FuncLit
    current := ast.Node(retStmt)
    for current != nil {
        parent := s.parentMap[current]
        if parent == nil {
            break
        }

        // Case 1: Named function
        if funcDecl, ok := parent.(*ast.FuncDecl); ok {
            return s.extractReturnTypeFromFuncType(funcDecl.Type, retStmt)
        }

        // Case 2: Anonymous function (lambda)
        if funcLit, ok := parent.(*ast.FuncLit); ok {
            return s.extractReturnTypeFromFuncType(funcLit.Type, retStmt)
        }

        current = parent
    }
    return nil
}

// Helper: Extract nth return type matching the return statement position
func (s *TypeInferenceService) extractReturnTypeFromFuncType(
    funcType *ast.FuncType,
    retStmt *ast.ReturnStmt,
) types.Type {
    if funcType.Results == nil || len(funcType.Results.List) == 0 {
        return nil
    }

    // Use go/types if available
    if s.typesInfo != nil {
        if typeAndValue := s.typesInfo.Types[funcType.Results.List[0].Type]; typeAndValue.Type != nil {
            return typeAndValue.Type
        }
    }

    // Fallback: Parse type expression manually
    return s.parseTypeExpr(funcType.Results.List[0].Type)
}
```

**Edge Cases**:
- Multiple return values (need position matching)
- Named vs unnamed returns
- Lambda/closure returns
- Defer/recover contexts

**Test Coverage**: Add tests in `type_inference_test.go`:
- Single return value
- Multiple return values (2nd position)
- Lambda return
- Named returns

---

### Task 1.2: Implement `findAssignmentType()`

**Goal**: Determine the type of the variable being assigned to.

**Implementation Strategy**:
```go
func (s *TypeInferenceService) findAssignmentType(assign *ast.AssignStmt, targetNode ast.Node) types.Type {
    // Find which RHS expression matches targetNode
    rhsIndex := -1
    for i, rhs := range assign.Rhs {
        if containsNode(rhs, targetNode) {
            rhsIndex = i
            break
        }
    }

    if rhsIndex == -1 {
        return nil
    }

    // Handle multiple assignment cases
    // Case 1: x = None (simple assignment)
    // Case 2: x, y = None, None (parallel assignment)
    // Case 3: x, y = returnsTwo() (unpacking, ignore for MVP)

    if rhsIndex >= len(assign.Lhs) {
        // Unpacking scenario (len(Rhs) < len(Lhs))
        // Not supported in MVP
        return nil
    }

    lhs := assign.Lhs[rhsIndex]

    // Use go/types to get LHS type
    if s.typesInfo != nil {
        if typeAndValue := s.typesInfo.Types[lhs]; typeAndValue.Type != nil {
            return typeAndValue.Type
        }
    }

    // Fallback: If LHS is identifier, look up in scope
    if ident, ok := lhs.(*ast.Ident); ok {
        if ident.Obj != nil && ident.Obj.Decl != nil {
            // Extract type from declaration
            if valueSpec, ok := ident.Obj.Decl.(*ast.ValueSpec); ok {
                if valueSpec.Type != nil {
                    return s.parseTypeExpr(valueSpec.Type)
                }
            }
        }
    }

    return nil
}

// Helper: Check if AST subtree contains target node
func containsNode(root, target ast.Node) bool {
    found := false
    ast.Inspect(root, func(n ast.Node) bool {
        if n == target {
            found = true
            return false
        }
        return true
    })
    return found
}
```

**Edge Cases**:
- Simple assignment: `x = None`
- Declaration assignment: `var x = None` (handled by Task 1.3)
- Parallel assignment: `x, y = None, Some(5)`
- Assignment to struct field: `s.field = None`
- Pointer assignment: `*ptr = None`

**Test Coverage**:
- Simple assignment (existing var)
- Parallel assignment
- Struct field assignment
- Pointer dereference assignment

---

### Task 1.3: Implement `findVarDeclType()`

**Goal**: Extract explicit type from variable declaration.

**Implementation Strategy**:
```go
func (s *TypeInferenceService) findVarDeclType(decl *ast.GenDecl, targetNode ast.Node) types.Type {
    // Iterate through specs to find the one containing targetNode
    for _, spec := range decl.Specs {
        valueSpec, ok := spec.(*ast.ValueSpec)
        if !ok {
            continue
        }

        // Check if targetNode is in this ValueSpec's values
        for i, value := range valueSpec.Values {
            if !containsNode(value, targetNode) {
                continue
            }

            // Found the spec containing our node
            // Case 1: Explicit type annotation
            if valueSpec.Type != nil {
                if s.typesInfo != nil {
                    if tv := s.typesInfo.Types[valueSpec.Type]; tv.Type != nil {
                        return tv.Type
                    }
                }
                return s.parseTypeExpr(valueSpec.Type)
            }

            // Case 2: Type inference from other values
            // var x, y = Some(5), None  → infer None type from Some(5)
            if len(valueSpec.Names) > 1 && len(valueSpec.Values) > 1 {
                // Try to infer from sibling values
                for j, siblingValue := range valueSpec.Values {
                    if j != i {
                        if siblingType := s.inferTypeFromExpr(siblingValue); siblingType != nil {
                            return siblingType
                        }
                    }
                }
            }

            return nil
        }
    }

    return nil
}
```

**Edge Cases**:
- Single var: `var x Option_int = None`
- Multi var: `var x, y = None, None`
- Short declaration: `x := None` (handled by assignment)
- Const declaration: `const x = None` (not supported)

**Test Coverage**:
- Single var with explicit type
- Multi var with shared type
- Var without type (should fail)

---

### Task 1.4: Implement `findCallArgType()`

**Goal**: Determine parameter type for a function call argument.

**Implementation Strategy**:
```go
func (s *TypeInferenceService) findCallArgType(call *ast.CallExpr, targetNode ast.Node) types.Type {
    // Find which argument position contains targetNode
    argIndex := -1
    for i, arg := range call.Args {
        if containsNode(arg, targetNode) {
            argIndex = i
            break
        }
    }

    if argIndex == -1 {
        return nil
    }

    // Use go/types to resolve function signature
    if s.typesInfo != nil {
        // Get the type of the function being called
        if tv := s.typesInfo.Types[call.Fun]; tv.Type != nil {
            // Extract signature
            if sig, ok := tv.Type.(*types.Signature); ok {
                // Handle variadic functions
                if sig.Variadic() && argIndex >= sig.Params().Len()-1 {
                    // Last parameter is variadic
                    lastParam := sig.Params().At(sig.Params().Len() - 1)
                    if slice, ok := lastParam.Type().(*types.Slice); ok {
                        return slice.Elem()
                    }
                    return lastParam.Type()
                }

                // Regular parameter
                if argIndex < sig.Params().Len() {
                    return sig.Params().At(argIndex).Type()
                }
            }
        }
    }

    // Fallback: Try to extract from function declaration
    // This is harder without go/types and only works for locally defined functions
    return nil
}
```

**Edge Cases**:
- Regular call: `processAge(None)`
- Variadic call: `fmt.Printf("%v %v", None, Some(5))`
- Method call: `obj.method(None)`
- Function value call: `fn(None)`
- Built-in function: `append(slice, None)`

**Test Coverage**:
- Regular function call
- Variadic function call
- Method call on struct
- Function passed as variable

---

### Task 1.5: Add `parseTypeExpr()` Helper

**Purpose**: Fallback for when go/types is unavailable.

**Implementation**:
```go
// parseTypeExpr attempts to convert ast.Expr to types.Type
// This is a best-effort fallback when go/types.Info is unavailable
func (s *TypeInferenceService) parseTypeExpr(expr ast.Expr) types.Type {
    switch e := expr.(type) {
    case *ast.Ident:
        // Basic types: int, string, bool, etc.
        return types.Typ[basicTypeMap[e.Name]]

    case *ast.SelectorExpr:
        // Qualified types: pkg.Type
        // Limited support without full type info
        return types.NewNamed(
            types.NewTypeName(0, nil, e.Sel.Name, nil),
            nil, nil,
        )

    case *ast.StarExpr:
        // Pointer types
        elem := s.parseTypeExpr(e.X)
        if elem != nil {
            return types.NewPointer(elem)
        }

    case *ast.ArrayType:
        // Slice or array
        elem := s.parseTypeExpr(e.Elt)
        if elem != nil {
            if e.Len == nil {
                return types.NewSlice(elem)
            }
            // Array length requires constant evaluation
        }

    case *ast.IndexExpr:
        // Generic types: Option[int], Result[string, error]
        // This is where we detect Option_T and Result_T_E
        if ident, ok := e.X.(*ast.Ident); ok {
            if strings.HasPrefix(ident.Name, "Option_") {
                return s.parseOptionType(ident.Name)
            }
            if strings.HasPrefix(ident.Name, "Result_") {
                return s.parseResultType(ident.Name)
            }
        }
    }

    return nil
}
```

---

## Task 2: Pattern Match Scrutinee go/types Integration

**Location**: `pkg/plugin/builtin/pattern_match.go:498`

**Current Problem**: Heuristic-based type detection (checks for Ok/Err/Some/None patterns).

**Solution**: Use go/types to get actual scrutinee type.

### Implementation

```go
// Replace getAllVariants() heuristic with go/types lookup
func (p *PatternMatchPlugin) getAllVariants(match *matchExpression) []string {
    // New approach: Use go/types to get scrutinee type
    if p.ctx != nil && p.ctx.TypeInfo != nil {
        if typesInfo, ok := p.ctx.TypeInfo.(*types.Info); ok {
            // Get type of scrutinee expression
            if tv, exists := typesInfo.Types[match.scrutinee]; exists && tv.Type != nil {
                return p.extractVariantsFromType(tv.Type)
            }
        }
    }

    // Fallback to heuristic (existing code)
    return p.getAllVariantsFromPatterns(match)
}

// New function: Extract variants from types.Type
func (p *PatternMatchPlugin) extractVariantsFromType(t types.Type) []string {
    // Strip pointer/named wrappers
    underlying := t.Underlying()

    // Check if it's a struct with a "tag" field
    if structType, ok := underlying.(*types.Struct); ok {
        for i := 0; i < structType.NumFields(); i++ {
            field := structType.Field(i)
            if field.Name() == "tag" {
                // This is likely Result or Option
                // Determine which by checking for "ok_0" or "some_0" fields
                typeName := t.String()

                if strings.Contains(typeName, "Result_") {
                    return []string{"Ok", "Err"}
                }
                if strings.Contains(typeName, "Option_") {
                    return []string{"Some", "None"}
                }
            }
        }
    }

    return []string{}
}
```

**Benefits**:
- Accurate type detection even for complex expressions
- Works with type aliases and named types
- Handles cases where heuristic fails (e.g., all arms use wildcards)

**Test Coverage**:
- Pattern match on variable (simple case)
- Pattern match on function call result
- Pattern match on struct field access
- Pattern match on type alias

---

## Task 3: Err() Context-Based Type Inference

**Location**: `pkg/plugin/builtin/result_type.go:286`

**Current Problem**: Uses `interface{}` as Ok type placeholder.

**Solution**: Leverage Task 1 infrastructure to infer Ok type from context.

### Implementation

```go
// In ResultTypePlugin.transformErrCall()
func (p *ResultTypePlugin) transformErrCall(call *ast.CallExpr) (ast.Expr, error) {
    // Extract error argument
    errorArg := call.Args[0]

    // Infer error type (existing code)
    errType := "error"
    if p.typeInference != nil {
        if inferredType, ok := p.typeInference.InferType(errorArg); ok {
            errType = inferredType
        }
    }

    // NEW: Infer Ok type from context using TypeInferenceService
    okType := "interface{}"
    if p.typeInference != nil {
        if contextType, ok := p.typeInference.InferTypeFromContext(call); ok {
            // contextType is Result_T_E, extract T
            if resultInfo := p.typeInference.ParseResultType(contextType.String()); resultInfo != nil {
                okType = resultInfo.OkTypeString
            }
        }
    }

    // Rest of function uses okType instead of hardcoded "interface{}"
    resultTypeName := fmt.Sprintf("Result_%s_%s",
        p.sanitizeTypeName(okType),
        p.sanitizeTypeName(errType))

    // ... continue as before
}
```

**Integration Points**:
1. Add `typeInference *TypeInferenceService` field to `ResultTypePlugin`
2. Initialize in `SetContext()` method
3. Call `InferTypeFromContext()` before generating Result type

**Test Cases**:
```go
// Test 1: Return context
func getResult() Result_int_error {
    return Err(fmt.Errorf("failed"))  // Should infer Ok=int
}

// Test 2: Assignment context
var result Result_string_error
result = Err(errors.New("error"))  // Should infer Ok=string

// Test 3: Function argument
func handleResult(r Result_bool_error) {}
handleResult(Err(myErr))  // Should infer Ok=bool

// Test 4: Struct field
type Response struct {
    data Result_User_error
}
r := Response{data: Err(err)}  // Should infer Ok=User
```

---

## Task 4: Guard Validation in If-Else Chains

**Locations**:
- `pkg/plugin/builtin/pattern_match_test.go:826`
- `pkg/plugin/builtin/pattern_match_test.go:1009`

**Current State**: Guards are parsed in preprocessor phase but ignored in AST transformation.

**Goal**: Validate and preserve guards when transforming switch to if-else chains.

### Analysis

**Guard Flow**:
1. Preprocessor inserts comments: `// DINGO_GUARD: x > 0`
2. Plugin parses comments in `extractMatchInfo()`
3. Transformation ignores guards (doesn't check or validate)

**Required Changes**:

### 4.1: Parse Guards in Transform Phase

```go
// In transformSwitchToIfElse()
func (p *PatternMatchPlugin) transformSwitchToIfElse(
    match *matchExpression,
    switchStmt *ast.SwitchStmt,
) (*ast.IfStmt, error) {

    // Existing code creates if-else chain
    // ...

    // NEW: Add guard conditions to if statements
    for i, caseClause := range switchStmt.Body.List {
        clause := caseClause.(*ast.CaseClause)

        // Get pattern info (includes guard if present)
        patternInfo := p.extractPatternInfoFromComment(clause)

        if patternInfo.guard != "" {
            // Parse guard expression
            guardExpr, err := p.parseGuardExpr(patternInfo.guard)
            if err != nil {
                return nil, fmt.Errorf("invalid guard: %v", err)
            }

            // Combine pattern check with guard
            // if tag == ResultTagOk && x > 0 { ... }
            existingCond := ifStmt.Cond
            combinedCond := &ast.BinaryExpr{
                X:  existingCond,
                Op: token.LAND,
                Y:  guardExpr,
            }
            ifStmt.Cond = combinedCond
        }
    }
}

// Helper: Parse guard expression string to AST
func (p *PatternMatchPlugin) parseGuardExpr(guardStr string) (ast.Expr, error) {
    // Use go/parser to parse guard as expression
    expr, err := parser.ParseExpr(guardStr)
    if err != nil {
        return nil, fmt.Errorf("failed to parse guard '%s': %v", guardStr, err)
    }
    return expr, nil
}
```

### 4.2: Validate Guard Semantics

**Rules**:
1. Guard must be boolean expression
2. Guard can only reference pattern-bound variables
3. Multiple guards on same pattern → separate if branches

**Validation Logic**:
```go
func (p *PatternMatchPlugin) validateGuard(
    guardExpr ast.Expr,
    boundVars []string,
) error {
    // Check 1: Guard references only bound variables
    usedVars := extractIdentifiers(guardExpr)
    for _, v := range usedVars {
        if !contains(boundVars, v) {
            return fmt.Errorf("guard references unbound variable: %s", v)
        }
    }

    // Check 2: Guard is boolean (requires go/types)
    if p.ctx != nil && p.ctx.TypeInfo != nil {
        if typesInfo, ok := p.ctx.TypeInfo.(*types.Info); ok {
            if tv := typesInfo.Types[guardExpr]; tv.Type != nil {
                if !types.Identical(tv.Type, types.Typ[types.Bool]) {
                    return fmt.Errorf("guard must be boolean, got %s", tv.Type)
                }
            }
        }
    }

    return nil
}
```

### 4.3: Update Tests

Remove TODOs from tests (lines 826, 1009) and add assertions:

```go
// TestPatternMatchPlugin_TransformWithGuard (line 826)
// Add verification that guard is preserved
func TestPatternMatchPlugin_TransformWithGuard(t *testing.T) {
    // ... existing code ...

    // NEW: Verify guard is in generated code
    var foundGuard bool
    ast.Inspect(file, func(n ast.Node) bool {
        if binExpr, ok := n.(*ast.BinaryExpr); ok {
            // Check if this is our guard: x > 0
            if binExpr.Op == token.GTR {
                foundGuard = true
            }
        }
        return true
    })

    if !foundGuard {
        t.Errorf("expected guard (x > 0) in transformed code")
    }
}
```

---

## Implementation Order & Dependencies

### Phase 1: Foundation (Task 1)
**Duration**: 2-3 days

1. **Day 1**: Implement helper functions
   - `findFunctionReturnType()` (1.1)
   - `findAssignmentType()` (1.2)
   - Add `containsNode()` helper

2. **Day 2**: Complete remaining helpers
   - `findVarDeclType()` (1.3)
   - `findCallArgType()` (1.4)
   - `parseTypeExpr()` (1.5)

3. **Day 3**: Comprehensive testing
   - Unit tests for each helper
   - Integration tests with None inference
   - Golden tests for all 4 contexts

**Dependencies**: None (all infrastructure exists)

**Validation**: None inference success rate 50% → 90%+

---

### Phase 2: Enhanced Type Detection (Task 2)
**Duration**: 1 day

**Depends on**: Task 1 (shares go/types integration knowledge)

1. Implement `extractVariantsFromType()`
2. Update `getAllVariants()` to use go/types first
3. Add fallback to existing heuristic
4. Test with complex scrutinee expressions

**Validation**: Pattern match works with type aliases, function returns

---

### Phase 3: Err() Inference (Task 3)
**Duration**: 1-2 days

**Depends on**: Task 1 (uses same context inference)

1. Add TypeInferenceService to ResultTypePlugin
2. Update `transformErrCall()` to use context inference
3. Parse Result type to extract Ok type
4. Update tests to verify correct types

**Validation**: Err() generates correct Result_T_E types

---

### Phase 4: Guard Support (Task 4)
**Duration**: 1-2 days

**Depends on**: None (independent feature)

1. Parse guards in `transformSwitchToIfElse()`
2. Combine guard with pattern condition
3. Add guard validation
4. Update tests (remove TODOs)

**Validation**: 2 guard tests pass without TODO markers

---

## File Modifications Summary

### Core Changes (Required)

| File | Lines Changed | Type | Task |
|------|---------------|------|------|
| `pkg/plugin/builtin/type_inference.go` | ~200 | Implement | 1.1-1.5 |
| `pkg/plugin/builtin/pattern_match.go` | ~100 | Implement | 2, 4.1-4.2 |
| `pkg/plugin/builtin/result_type.go` | ~50 | Implement | 3 |
| `pkg/plugin/builtin/pattern_match_test.go` | ~50 | Update | 4.3 |

### New Test Files

| File | Purpose | Task |
|------|---------|------|
| `pkg/plugin/builtin/type_inference_context_test.go` | Test 4 new helpers | 1 |
| `pkg/plugin/builtin/pattern_match_scrutinee_test.go` | Test go/types scrutinee | 2 |
| `pkg/plugin/builtin/result_err_context_test.go` | Test Err() inference | 3 |

### Golden Tests (Integration)

| File | Purpose | Task |
|------|---------|------|
| `tests/golden/none_inference_comprehensive.dingo` | All 9 contexts | 1 |
| `tests/golden/pattern_match_scrutinee_complex.dingo` | Type aliases | 2 |
| `tests/golden/result_err_contexts.dingo` | Err() in 4 contexts | 3 |
| `tests/golden/pattern_guards_complete.dingo` | Guards in if-else | 4 |

---

## Testing Strategy

### Unit Tests (Per Task)

**Task 1**: `type_inference_context_test.go`
- Test each helper function independently
- Mock parent map and go/types.Info
- Cover edge cases (variadic, pointers, etc.)
- Target: 95%+ coverage on new code

**Task 2**: `pattern_match_scrutinee_test.go`
- Test `extractVariantsFromType()` with various types
- Verify fallback to heuristic works
- Test with type aliases, pointers, selectors
- Target: 20 test cases

**Task 3**: `result_err_context_test.go`
- Test Err() in 4 contexts (return, assign, call, struct)
- Verify correct Result_T_E type generation
- Test fallback when context unavailable
- Target: 15 test cases

**Task 4**: `pattern_match_guard_test.go`
- Test guard parsing
- Test guard validation (bound vars, boolean type)
- Test guard combination with pattern check
- Target: 10 test cases

### Integration Tests (Golden Files)

**Comprehensive None Inference** (`none_inference_comprehensive.dingo`):
```dingo
package test

func testAllContexts() {
    // Context 1: Return
    func getAge() Option_int {
        return None  // Should infer Option_int
    }

    // Context 2: Assignment
    var age Option_int
    age = None  // Should infer Option_int

    // Context 3: Var decl
    var name Option_string = None  // Explicit type

    // Context 4: Call arg
    func processAge(age Option_int) {}
    processAge(None)  // Should infer Option_int

    // Context 5: Struct field
    type User struct {
        age Option_int
    }
    u := User{age: None}  // Should infer Option_int

    // Context 6: Composite literal (array)
    ages := []Option_int{None, Some(25)}

    // Context 7: Map literal
    m := map[string]Option_int{"age": None}

    // Context 8: Match arm
    match age {
        Some(x) => x,
        _ => None  // Infer from match type
    }

    // Context 9: Ternary (future)
    // result := condition ? None : Some(5)
}
```

**Expected Output**: All None constants transformed to correct `Option_T{tag: OptionTagNone}`.

---

## Risk Analysis

### High Risk (Address Immediately)

**Risk 1: go/types.Info Unavailable**
- **Scenario**: Type checker not run before plugin
- **Impact**: All context inference fails
- **Mitigation**:
  - Graceful fallback to `parseTypeExpr()`
  - Warn user if go/types unavailable
  - Document requirement in plugin README

**Risk 2: Parent Map Incomplete**
- **Scenario**: Some nodes missing from parent map
- **Impact**: Context traversal stops early
- **Mitigation**:
  - Validate parent map has all nodes
  - Add debug logging for missing parents
  - Implement `ast.Inspect` fallback

**Risk 3: Complex Guard Expressions**
- **Scenario**: Guard uses function calls, method calls
- **Impact**: Validation may fail or be incorrect
- **Mitigation**:
  - Start with simple guards (comparisons, boolean ops)
  - Document guard limitations
  - Add comprehensive error messages

### Medium Risk (Monitor)

**Risk 4: Performance Degradation**
- **Scenario**: Context traversal adds >15ms per file
- **Impact**: Build times increase
- **Mitigation**:
  - Cache type lookups aggressively
  - Limit parent chain depth (max 20 levels)
  - Profile before/after

**Risk 5: Type Alias Resolution**
- **Scenario**: User defines `type MyOption = Option_int`
- **Impact**: Type detection may fail
- **Mitigation**:
  - Use `types.Type.Underlying()` consistently
  - Test with type aliases explicitly
  - Document known limitations

### Low Risk (Acceptable)

**Risk 6: Fallback Quality**
- **Scenario**: `parseTypeExpr()` can't handle complex types
- **Impact**: Some edge cases fail without go/types
- **Mitigation**: Acceptable, go/types should always be available

---

## Success Metrics

### Quantitative

1. **None Inference Coverage**: 50% → 90%+
   - 4 out of 9 contexts → 9 out of 9 contexts

2. **Pattern Match Accuracy**: 85% → 95%+
   - Heuristic failures reduced by go/types integration

3. **Err() Type Correctness**: 0% → 80%+
   - Currently uses `interface{}` → Correct concrete types

4. **Guard Test Pass Rate**: 0% → 100%
   - 2 TODO tests → 2 passing tests

5. **Performance**: <15ms overhead maintained
   - Context traversal optimized with caching

### Qualitative

1. **Developer Experience**: Error messages improved
   - "Cannot infer type" → "Expected Option[T], inferred from return type"

2. **Code Clarity**: Generated Go more readable
   - `Result_interface_error` → `Result_User_error`

3. **Robustness**: Fewer edge case failures
   - Type aliases, pointers, complex expressions handled

---

## Future Enhancements (Post-Phase 4.2)

### Short Term (Phase 5)

1. **Composite Literal Inference**
   - Arrays: `[]Option_int{None, Some(5)}`
   - Maps: `map[string]Option_int{"key": None}`
   - Slices: `append(slice, None)`

2. **Match Arm Inference**
   - Infer type from other arms
   - Validate type consistency

3. **Enhanced Guard Support**
   - Guards in match expressions (not just if-else)
   - Multi-condition guards
   - Guard exhaustiveness analysis

### Long Term (Phase 6+)

1. **Cross-Function Inference**
   - Infer types across function boundaries
   - Generic function specialization

2. **Bidirectional Type Inference**
   - Flow types both up and down AST
   - HM-style unification

3. **Type Hole Support**
   - Explicit `_` placeholders for inference
   - Interactive type suggestions

---

## Open Questions & Gaps

See `gaps.json` for detailed questions requiring user input before implementation.

---

## Appendix A: Code Examples

### Example A1: None Inference in All Contexts

**Input (Dingo)**:
```dingo
func processUser(user User) Result_int_error {
    age := user.GetAge()

    if age == None {
        return Err(errors.New("no age"))
    }

    return Ok(*age)
}
```

**After Task 1 Implementation**:
- `age == None` → Correctly inferred as `Option_int`
- `return Err(...)` → Correctly inferred as `Result_int_error` (Task 3)

### Example A2: Pattern Match with Type Alias

**Input (Dingo)**:
```dingo
type UserResult = Result_User_error

func getUser() UserResult { ... }

func main() {
    result := getUser()
    match result {
        Ok(user) => println(user.name),
        Err(e) => println(e),
    }
}
```

**After Task 2 Implementation**:
- Scrutinee type: `UserResult` → Resolved via go/types → `Result_User_error`
- Variants detected: `["Ok", "Err"]`
- Exhaustiveness check passes

### Example A3: Guards in If-Else

**Input (Preprocessed)**:
```go
switch result.tag {
case ResultTagOk:
    // DINGO_PATTERN: Ok(x) | DINGO_GUARD: x > 0
    x := *result.ok_0
    return x * 2
case ResultTagOk:
    // DINGO_PATTERN: Ok(x) | DINGO_GUARD: x == 0
    x := *result.ok_0
    return 0
}
```

**After Task 4 Implementation**:
```go
if result.tag == ResultTagOk && *result.ok_0 > 0 {
    x := *result.ok_0
    return x * 2
} else if result.tag == ResultTagOk && *result.ok_0 == 0 {
    x := *result.ok_0
    return 0
}
```

---

## Appendix B: Performance Benchmarks

**Target**: <15ms overhead per file

### Benchmark Plan

```go
func BenchmarkContextInference(b *testing.B) {
    // Setup: File with 100 None constants
    src := generateTestFile(100)

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        // Run type inference
        service.InferTypeFromContext(noneNode)
    }
}

// Expected: <150μs per call (100 calls = 15ms total)
```

### Optimization Strategies

1. **Cache Type Lookups**: Store go/types results in map
2. **Limit Parent Depth**: Stop after 20 levels
3. **Early Exit**: Return on first match, don't check all contexts
4. **Lazy Initialization**: Create parent map only when needed

---

## Summary

This plan completes the final 30% of Phase 4 type inference and pattern matching:

1. **Task 1** (Critical): Implement 4 missing context helpers → 90%+ None inference
2. **Task 2** (Important): go/types scrutinee detection → Accurate pattern matching
3. **Task 3** (Important): Err() context inference → Correct Result types
4. **Task 4** (Nice-to-have): Guard validation → Complete guard support

**Total Effort**: 5-8 days for all tasks
**Risk Level**: Low (infrastructure exists, implementation straightforward)
**Impact**: High (completes Phase 4.2 objectives)

All implementation leverages existing systems. No architectural changes required.
