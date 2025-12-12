# Task E: None Context Inference - Context Detection Strategy

## Overview

The None Context Inference Plugin implements a **precedence-based, conservative** strategy for inferring the type of None constants from surrounding context.

## Context Detection Algorithm

### 1. Parent Chain Walking

Uses `ctx.WalkParents(noneIdent, visitor)` to traverse up the AST tree until a valid context is found.

**Traversal order** (bottom-up):
```
None identifier
    ↓
Parent 1 (e.g., KeyValueExpr in struct literal)
    ↓
Parent 2 (e.g., CompositeLit)
    ↓
Parent 3 (e.g., AssignStmt)
    ↓
...
    ↓
Root (File)
```

**Early termination**: Visitor returns `false` when valid context found.

### 2. Context Types (Precedence Order)

#### Priority 1: Explicit Type Annotation
```go
var x: Option[int] = None
// or
let x: Option[int] = None
```

**Detection**: `ast.ValueSpec` with non-nil `Type` field

**Inference**: Directly extract from `ValueSpec.Type`

**Why first**: User explicitly stated intent - highest confidence

---

#### Priority 2: Return Statement
```go
func getAge() -> Option[int] {
    return None  // ← infer from function signature
}
```

**Detection**: `ast.ReturnStmt` parent

**Inference**:
1. Walk up to find enclosing `*ast.FuncDecl`
2. Extract return type from `FuncDecl.Type.Results`
3. Verify it's an Option[T] type

**Implementation**:
```go
func findReturnType(noneIdent) {
    // Walk up to FuncDecl
    var funcDecl *ast.FuncDecl
    ctx.WalkParents(noneIdent, func(p) {
        if fn, ok := p.(*ast.FuncDecl); ok {
            funcDecl = fn
            return false  // Found it
        }
        return true
    })

    // Extract return type
    returnType := funcDecl.Type.Results.List[0].Type
    return extractOptionType(returnType)
}
```

---

#### Priority 3: Assignment Target
```go
let age: Option[int]
age = None  // ← infer from LHS variable type
```

**Detection**: `ast.AssignStmt` parent

**Inference**:
1. Find which RHS position None is in
2. Get corresponding LHS identifier
3. Use go/types to lookup variable type
4. Verify it's Option[T]

**Requires**: go/types `types.Info.Uses` map

**Implementation**:
```go
func findAssignmentType(noneIdent, assignStmt) {
    // Find RHS index
    rhsIndex := findNonePosition(assignStmt.Rhs, noneIdent)

    // Get LHS identifier
    lhsIdent := assignStmt.Lhs[rhsIndex].(*ast.Ident)

    // Use go/types
    obj := typesInfo.Uses[lhsIdent]
    typ := obj.Type()  // Get actual type

    return extractOptionType(typ)
}
```

---

#### Priority 4: Function Call Parameter
```go
processAge(None)  // ← infer from processAge parameter type
```

**Detection**: `ast.CallExpr` parent

**Inference**:
1. Find which argument position None is in
2. Use go/types to get function signature
3. Extract parameter type at that position
4. Verify it's Option[T]

**Requires**: go/types `types.Info.Types` map for CallExpr.Fun

**Implementation**:
```go
func findParameterType(noneIdent, callExpr) {
    // Find argument index
    argIndex := findNonePosition(callExpr.Args, noneIdent)

    // Get function type via go/types
    tv := typesInfo.Types[callExpr.Fun]
    funcType := tv.Type.(*types.Signature)

    // Get parameter type
    paramType := funcType.Params().At(argIndex).Type()

    return extractOptionType(paramType)
}
```

---

#### Priority 5: Struct Field
```go
User{
    name: "Alice",
    age: None,  // ← infer from User.age field type
}
```

**Detection**: `ast.CompositeLit` parent

**Inference**:
1. Find which field None is assigned to (from KeyValueExpr)
2. Use go/types to get struct type
3. Lookup field by name
4. Extract field type
5. Verify it's Option[T]

**Requires**: go/types `types.Info.Types` map for CompositeLit.Type

**Implementation**:
```go
func findFieldType(noneIdent, compLit) {
    // Find field name from KeyValueExpr
    fieldName := findFieldName(compLit.Elts, noneIdent)

    // Get struct type via go/types
    tv := typesInfo.Types[compLit.Type]
    structType := tv.Type.Underlying().(*types.Struct)

    // Find field by name
    for i := 0; i < structType.NumFields(); i++ {
        field := structType.Field(i)
        if field.Name() == fieldName {
            return extractOptionType(field.Type())
        }
    }
}
```

---

## Error Handling

### No Context Found
```go
x := None  // ❌ ERROR
```

**Error message**:
```
cannot infer type for None constant: no valid type context found.
Add explicit type annotation: let x: Option[T] = None
```

**Rationale**: Conservative approach prevents subtle bugs

---

### Non-Option Type Context
```go
func getInt() -> int {
    return None  // ❌ ERROR: return type is not Option[T]
}
```

**Error message**:
```
cannot infer type for None constant: expected Option[T] type, got int.
Add explicit type annotation: let x: Option[T] = None
```

---

## Type Extraction

### From AST Type Expression
```go
func extractOptionType(typeExpr ast.Expr) string {
    switch t := typeExpr.(type) {
    case *ast.Ident:
        // Already transformed: Option_int
        if strings.HasPrefix(t.Name, "Option_") {
            return t.Name
        }

    case *ast.IndexExpr:
        // Generic syntax: Option[int]
        if base, ok := t.X.(*ast.Ident); ok && base.Name == "Option" {
            innerType := getTypeName(t.Index)
            return "Option_" + innerType
        }
    }
    return ""
}
```

### From go/types Type
```go
func extractOptionTypeFromGoTypes(typ types.Type) string {
    if named, ok := typ.(*types.Named); ok {
        typeName := named.Obj().Name()
        if strings.HasPrefix(typeName, "Option_") {
            return typeName
        }
    }
    return ""
}
```

---

## Edge Cases Handled

### 1. Selector Expressions
```go
tag := OptionTag_None  // ← NOT a None constant, it's a tag value
```

**Handling**: Skip if parent is `*ast.SelectorExpr`

### 2. Multiple Return Values
```go
func multi() -> (int, Option[string]) {
    return 42, None  // ← infer from second return type
}
```

**Handling**: Match return position to result type

### 3. Nested Structures
```go
User{
    profile: Profile{
        age: None,  // ← walks through nested CompositeLit
    }
}
```

**Handling**: Parent walk finds innermost CompositeLit first

---

## go/types Integration Points

### Required Information

1. **types.Info.Uses** - For variable lookups (assignment context)
2. **types.Info.Types** - For expression types (call, struct contexts)
3. **types.Info.Defs** - For function definitions (return context)

### Fallback Behavior

If go/types not available:
- Explicit annotations still work (pure AST)
- Return types work (pure AST from FuncDecl)
- Assignment/call/field contexts **fail gracefully** → error

**Rationale**: Rather fail than guess wrong type

---

## Performance Characteristics

### Time Complexity
- **Discovery**: O(N) where N = AST nodes
- **Inference per None**: O(D) where D = AST depth (typically 10-20)
- **Total**: O(N + M*D) where M = number of None constants

**Typical case**: 10-100 None constants per file → <1ms overhead

### Space Complexity
- **noneNodes slice**: O(M) pointers
- **Parent map**: O(N) built once in Task B
- **Total**: O(N + M) - minimal

---

## Future Optimizations

### Phase 5+ Enhancements

1. **Match arm inference**
   ```go
   match status {
       Active(id) => Some(id),
       Inactive => None,  // ← infer from Some(id) arm
   }
   ```

2. **Array element inference**
   ```go
   [Some(1), None, Some(3)]  // ← infer from other elements
   ```

3. **Bidirectional inference**
   ```go
   x := None  // Defer inference
   processAge(x)  // Now infer from call site
   ```

4. **Smart suggestions**
   ```go
   x := None  // ERROR
   // Suggestion: Did you mean Option[int]? (based on later usage)
   ```

---

## Summary

**Strategy**: Conservative, precedence-based, go/types-powered

**Strengths**:
- ✅ Accurate type inference in common cases
- ✅ Clear error messages for ambiguity
- ✅ Leverages existing go/types infrastructure
- ✅ Fast (parent map already built)

**Limitations**:
- ❌ No forward inference (requires complex flow analysis)
- ❌ Struct field inference requires full type checking
- ❌ Match arm inference not yet implemented (Phase 4.2)

**Overall**: Solid foundation for Phase 4.1 MVP, extensible for future enhancements.
