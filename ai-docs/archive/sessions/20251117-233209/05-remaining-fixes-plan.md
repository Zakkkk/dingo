# Remaining Critical Fixes Implementation Plan

**Session**: 20251117-233209
**Date**: 2025-11-18
**Phase**: Result[T,E] Type - Critical Fixes A2 & A3

## Current Status

**Completed**: 3/9 fixes (A1, A4, B1)
**Test Status**: 31/38 passing (81%)
**Remaining Critical**: 2 blockers (A2, A3)

---

## Fix A2: Constructor AST Mutation

**Priority**: CRITICAL (BLOCKING)
**Estimated Time**: 3-4 hours
**Complexity**: HIGH

### Problem Statement

Current implementation in `transformOkConstructor()` and `transformErrConstructor()` only logs transformations but never modifies the AST. Ok()/Err() calls remain unchanged in the output.

**Location**: `pkg/plugin/builtin/result_type.go:145-211`

### Implementation Strategy

#### Step 1: Add AST Mutation Infrastructure (30 min)

Create a visitor pattern that tracks parent nodes during AST traversal.

```go
// Add to ResultTypePlugin struct
type astTransformer struct {
    plugin     *ResultTypePlugin
    parents    map[ast.Node]ast.Node  // Track parent relationships
    transforms map[ast.Node]ast.Expr  // Track pending replacements
}

// Walk AST and track parent relationships
func (t *astTransformer) buildParentMap(node ast.Node) {
    ast.Inspect(node, func(n ast.Node) bool {
        // For each child, record its parent
        for _, child := range getChildren(n) {
            t.parents[child] = n
        }
        return true
    })
}
```

#### Step 2: Mark Nodes for Replacement (1 hour)

Modify `transformOkConstructor` and `transformErrConstructor` to mark CallExpr nodes for replacement instead of just logging.

```go
func (p *ResultTypePlugin) transformOkConstructor(call *ast.CallExpr) ast.Expr {
    if len(call.Args) != 1 {
        p.ctx.Logger.Warn("Ok() expects exactly one argument, found %d", len(call.Args))
        return call  // Return unchanged
    }

    valueArg := call.Args[0]
    okType := p.inferTypeFromExpr(valueArg)
    errType := "error"

    resultTypeName := fmt.Sprintf("Result_%s_%s",
        p.sanitizeTypeName(okType),
        p.sanitizeTypeName(errType))

    if !p.emittedTypes[resultTypeName] {
        p.emitResultDeclaration(okType, errType, resultTypeName)
        p.emittedTypes[resultTypeName] = true
    }

    // Create the replacement CompositeLit
    replacement := &ast.CompositeLit{
        Type: ast.NewIdent(resultTypeName),
        Elts: []ast.Expr{
            &ast.KeyValueExpr{
                Key:   ast.NewIdent("tag"),
                Value: ast.NewIdent("ResultTag_Ok"),
            },
            &ast.KeyValueExpr{
                Key: ast.NewIdent("ok_0"),
                Value: &ast.UnaryExpr{
                    Op: token.AND,
                    X:  valueArg,  // Use original argument expression
                },
            },
        },
    }

    return replacement  // Return the new node
}
```

#### Step 3: Apply Replacements (1-1.5 hours)

Implement a post-order traversal that replaces marked nodes.

```go
// Add to ResultTypePlugin
func (p *ResultTypePlugin) Transform(node ast.Node) (ast.Node, error) {
    if p.ctx == nil {
        return nil, fmt.Errorf("plugin context not initialized")
    }

    // Use ast.Inspect with modification
    transformed := astutil.Apply(node,
        func(cursor *astutil.Cursor) bool {
            n := cursor.Node()

            // Check if this is a CallExpr we need to transform
            if call, ok := n.(*ast.CallExpr); ok {
                if ident, ok := call.Fun.(*ast.Ident); ok {
                    var replacement ast.Expr
                    switch ident.Name {
                    case "Ok":
                        replacement = p.transformOkConstructor(call)
                    case "Err":
                        replacement = p.transformErrConstructor(call)
                    }

                    if replacement != nil && replacement != call {
                        cursor.Replace(replacement)
                    }
                }
            }
            return true
        },
        nil,  // Post-order not needed
    )

    return transformed, nil
}
```

#### Step 4: Update Plugin Interface (30 min)

Add Transform method to plugin interface and update call sites.

```go
// In pkg/plugin/plugin.go
type Plugin interface {
    Name() string
    Process(node ast.Node) error
    Transform(node ast.Node) (ast.Node, error)  // NEW
    GetPendingDeclarations() []ast.Decl
    ClearPendingDeclarations()
}
```

#### Step 5: Testing (30-45 min)

Update tests to verify actual AST transformation:

```go
func TestConstructor_OkTransformation(t *testing.T) {
    plugin := NewResultTypePlugin()
    plugin.ctx = createTestContext()

    input := parseCode(`
        package main
        func main() {
            x := Ok(42)
        }
    `)

    transformed, err := plugin.Transform(input)
    require.NoError(t, err)

    // Verify the transformation occurred
    var found bool
    ast.Inspect(transformed, func(n ast.Node) bool {
        if comp, ok := n.(*ast.CompositeLit); ok {
            if ident, ok := comp.Type.(*ast.Ident); ok {
                if strings.HasPrefix(ident.Name, "Result_") {
                    found = true
                }
            }
        }
        return true
    })

    assert.True(t, found, "Expected CompositeLit for Result type")
}
```

### Dependencies

- `golang.org/x/tools/go/ast/astutil` (already imported)
- No new external dependencies

### Risks & Mitigations

**Risk 1**: Losing source positions during transformation
**Mitigation**: Copy Position/Pos from original CallExpr to CompositeLit

**Risk 2**: Breaking type inference in later passes
**Mitigation**: Ensure all transformed nodes maintain correct type information

**Risk 3**: Recursive transformation infinite loops
**Mitigation**: Track visited nodes, single-pass transformation

### Verification Steps

1. All existing tests still pass
2. New transformation tests verify CompositeLit generation
3. Generated code compiles with `go build`
4. Source positions maintained (error messages point to correct lines)

---

## Fix A3: Type Inference with go/types

**Priority**: CRITICAL (BLOCKING)
**Estimated Time**: 4-6 hours
**Complexity**: HIGH

### Problem Statement

Current `inferTypeFromExpr()` returns variable names (e.g., "user") instead of actual types (e.g., "*User"). This causes generated code to reference non-existent types.

**Location**: `pkg/plugin/builtin/result_type.go:213-238`

### Implementation Strategy

#### Step 1: Integrate go/types (1 hour)

Add type checking infrastructure to plugin context.

```go
// Add to plugin.Context
type Context struct {
    FileSet    *token.FileSet
    Logger     Logger
    TypesInfo  *types.Info        // NEW: Type information
    TypesPkg   *types.Package     // NEW: Package being analyzed
}

// Initialize in plugin
func (p *ResultTypePlugin) initTypeChecker() error {
    if p.ctx.TypesInfo == nil {
        p.ctx.TypesInfo = &types.Info{
            Types: make(map[ast.Expr]types.TypeAndValue),
            Defs:  make(map[*ast.Ident]types.Object),
            Uses:  make(map[*ast.Ident]types.Object),
        }
    }
    return nil
}
```

#### Step 2: Type Checking Pass (2-3 hours)

Before AST transformation, run type checker to populate TypesInfo.

```go
func (p *ResultTypePlugin) Process(node ast.Node) error {
    if p.ctx == nil {
        return fmt.Errorf("plugin context not initialized")
    }

    // Run type checker if not already done
    if p.ctx.TypesInfo == nil {
        if err := p.runTypeChecker(node); err != nil {
            return fmt.Errorf("type checking failed: %w", err)
        }
    }

    // ... rest of Process logic
}

func (p *ResultTypePlugin) runTypeChecker(node ast.Node) error {
    conf := types.Config{
        Importer: importer.Default(),
        Error: func(err error) {
            p.ctx.Logger.Warn("Type checking: %v", err)
        },
    }

    p.ctx.TypesInfo = &types.Info{
        Types: make(map[ast.Expr]types.TypeAndValue),
        Defs:  make(map[*ast.Ident]types.Object),
        Uses:  make(map[*ast.Ident]types.Object),
    }

    // Type-check the package
    pkg, err := conf.Check("main", p.ctx.FileSet, []*ast.File{node.(*ast.File)}, p.ctx.TypesInfo)
    if err != nil {
        return err
    }

    p.ctx.TypesPkg = pkg
    return nil
}
```

#### Step 3: Implement Proper Type Inference (1-2 hours)

Replace heuristic-based inference with go/types lookup.

```go
func (p *ResultTypePlugin) inferTypeFromExpr(expr ast.Expr) string {
    // Try go/types first
    if p.ctx.TypesInfo != nil {
        if tv, ok := p.ctx.TypesInfo.Types[expr]; ok {
            return tv.Type.String()
        }
    }

    // Fallback to heuristics for simple cases
    switch e := expr.(type) {
    case *ast.BasicLit:
        switch e.Kind {
        case token.INT:
            return "int"
        case token.FLOAT:
            return "float64"
        case token.STRING:
            return "string"
        case token.CHAR:
            return "rune"
        }
    case *ast.Ident:
        // Look up in types.Info
        if p.ctx.TypesInfo != nil {
            if obj, ok := p.ctx.TypesInfo.Uses[e]; ok {
                if obj.Type() != nil {
                    return obj.Type().String()
                }
            }
        }
        // Fallback: return identifier as-is (may need refinement)
        return "interface{}"
    case *ast.CallExpr:
        // Look up function return type
        if p.ctx.TypesInfo != nil {
            if tv, ok := p.ctx.TypesInfo.Types[e]; ok {
                return tv.Type.String()
            }
        }
        return "interface{}"
    }

    return "interface{}"
}
```

#### Step 4: Handle Type String Parsing (1 hour)

go/types returns strings like `*main.User` or `[]byte`. Need to parse these correctly.

```go
func (p *ResultTypePlugin) parseTypeString(typeStr string) (pkgName, typeName string, isPointer bool, isSlice bool) {
    // Handle pointer prefix
    if strings.HasPrefix(typeStr, "*") {
        isPointer = true
        typeStr = strings.TrimPrefix(typeStr, "*")
    }

    // Handle slice prefix
    if strings.HasPrefix(typeStr, "[]") {
        isSlice = true
        typeStr = strings.TrimPrefix(typeStr, "[]")
    }

    // Split package and type
    parts := strings.Split(typeStr, ".")
    if len(parts) == 2 {
        return parts[0], parts[1], isPointer, isSlice
    }

    return "", typeStr, isPointer, isSlice
}
```

#### Step 5: Testing (1 hour)

```go
func TestTypeInference_ActualTypes(t *testing.T) {
    tests := []struct {
        name     string
        code     string
        expected string
    }{
        {
            name: "variable with struct type",
            code: `
                package main
                type User struct { Name string }
                func main() {
                    user := User{Name: "Alice"}
                    result := Ok(user)
                }
            `,
            expected: "main.User",
        },
        {
            name: "variable with pointer type",
            code: `
                package main
                type User struct { Name string }
                func main() {
                    user := &User{Name: "Alice"}
                    result := Ok(user)
                }
            `,
            expected: "*main.User",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Parse and type-check
            fset := token.NewFileSet()
            file, err := parser.ParseFile(fset, "test.go", tt.code, 0)
            require.NoError(t, err)

            plugin := NewResultTypePlugin()
            plugin.ctx = &plugin.Context{
                FileSet: fset,
                Logger:  testLogger{},
            }

            err = plugin.Process(file)
            require.NoError(t, err)

            // Verify type was correctly inferred
            // ... assertion logic
        })
    }
}
```

### Dependencies

- `go/types` (standard library)
- `go/importer` (standard library)

### Risks & Mitigations

**Risk 1**: Type checking failures on partial code
**Mitigation**: Graceful fallback to heuristics, log warnings

**Risk 2**: Import resolution issues
**Mitigation**: Use `importer.Default()` with proper GOPATH setup

**Risk 3**: Performance overhead
**Mitigation**: Cache types.Info, run type checker once per file

### Verification Steps

1. Type inference returns actual type names, not variable names
2. Complex types (pointers, slices, structs) handled correctly
3. Cross-package types resolved properly
4. Generated Result types use correct type names

---

## Implementation Order

1. **Fix A2** (3-4 hours) - Implement first, simpler conceptually
2. **Fix A3** (4-6 hours) - Depends on A2 being complete for proper testing
3. **Integration Testing** (1 hour) - End-to-end tests with both fixes
4. **Code Review** (1 hour) - Run all 3 review agents again

**Total Estimated Time**: 9-12 hours

---

## Success Criteria

- All 38 tests passing (including the 7 currently failing for advanced methods)
- Generated Go code compiles without errors
- Ok()/Err() calls correctly transformed to struct literals
- Type inference uses actual types from go/types
- All 3 code review agents approve the changes

---

## Next Steps

1. Create new session or continue in current session
2. Implement Fix A2 following the plan above
3. Test thoroughly
4. Implement Fix A3
5. Run full test suite
6. Run code reviews
7. Commit changes

**Last Updated**: 2025-11-18T14:00:00Z
