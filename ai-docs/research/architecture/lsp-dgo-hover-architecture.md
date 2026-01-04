# LSP Hover Architecture for dgo.Result and dgo.Option Types

## Problem Statement

When hovering over `Result` in Dingo code like:
```go
func GetUser(id string) Result[User, DBError]
```

Users currently see the raw Go struct:
```
type Result struct{Tag dgo.ResultTag; Ok *T; Err *E}
```

This is unhelpful. Users should see:
```
Result[User, DBError]

Success OR failure container.

Methods:
- .IsOk() bool - check if success
- .IsErr() bool - check if error
- .MustOk() User - extract value (panics if Err)
- .MustErr() DBError - extract error (panics if Ok)
- .OkOr(default User) User - extract with default
```

## Current Architecture

### Hover Pipeline

```
User hovers at position (line, col)
    |
    v
Server.handleHover()
    |
    v (for .dingo files)
Server.nativeHover()
    |
    v
SemanticManager.Get() -> Document with SemanticMap
    |
    v
SemanticMap.FindAt(line, col) -> SemanticEntity
    |
    v
FormatHover(entity, pkg) -> protocol.Hover
```

### Key Files

| File | Role |
|------|------|
| `pkg/lsp/hover_native.go` | Entry point for Dingo hover |
| `pkg/lsp/semantic/document.go` | Manages semantic documents |
| `pkg/lsp/semantic/builder.go` | Builds SemanticMap from transpiled Go |
| `pkg/lsp/semantic/format.go` | Formats hover content (target for changes) |
| `pkg/lsp/semantic/types.go` | SemanticEntity, DingoContext types |

### How go/types Represents Generic Types

When `go/types` encounters `dgo.Result[User, DBError]`, it creates:

```go
*types.Named {
    Obj: *types.TypeName {
        Name: "Result"
        Pkg: &types.Package{Path: "github.com/MadAppGang/dingo/pkg/dgo"}
    }
    TypeArgs: *types.TypeList [
        *types.Named{Obj.Name: "User"},      // T
        *types.Named{Obj.Name: "DBError"}    // E
    ]
    Underlying: *types.Struct { ... }  // The raw struct
}
```

Key insight: `named.TypeArgs()` gives us the instantiated type parameters.

## Design

### Approach: Intercept in `formatSignature` / `formatType`

The cleanest place to intercept is in `format.go` because:

1. All hover formatting flows through `formatSignature` and `formatType`
2. We can detect `dgo.Result[T,E]` / `dgo.Option[T]` using `go/types`
3. We can generate friendly documentation without changing the semantic map

### Detection Logic

```go
// isDgoType checks if a type is dgo.Result or dgo.Option
func isDgoType(t types.Type) (typeName string, typeArgs []types.Type, ok bool) {
    named, ok := t.(*types.Named)
    if !ok {
        return "", nil, false
    }

    typeName = named.Obj().Name()
    if typeName != "Result" && typeName != "Option" {
        return "", nil, false
    }

    pkg := named.Obj().Pkg()
    if pkg == nil {
        return "", nil, false
    }

    // Check for dgo package (handles various import paths)
    if !strings.Contains(pkg.Path(), "dgo") && pkg.Name() != "dgo" {
        return "", nil, false
    }

    // Extract type arguments
    typeArgs = make([]types.Type, 0)
    if args := named.TypeArgs(); args != nil {
        for i := 0; i < args.Len(); i++ {
            typeArgs = append(typeArgs, args.At(i))
        }
    }

    return typeName, typeArgs, true
}
```

### Hover Content Design

**For `Result[T, E]`:**
```markdown
```go
Result[User, DBError]
```

**Success OR failure container**

| Method | Returns | Description |
|--------|---------|-------------|
| `.IsOk()` | `bool` | Check if success |
| `.IsErr()` | `bool` | Check if error |
| `.MustOk()` | `User` | Extract value (panics if Err) |
| `.MustErr()` | `DBError` | Extract error (panics if Ok) |
| `.OkOr(default)` | `User` | Extract with default |

See also: `Ok[T,E](value)`, `Err[T,E](err)`
```

**For `Option[T]`:**
```markdown
```go
Option[User]
```

**Optional value container** (Some or None)

| Method | Returns | Description |
|--------|---------|-------------|
| `.IsSome()` | `bool` | Check if value present |
| `.IsNone()` | `bool` | Check if empty |
| `.MustSome()` | `User` | Extract value (panics if None) |
| `.SomeOr(default)` | `User` | Extract with default |

See also: `Some[T](value)`, `None[T]()`
```

### Implementation Plan

#### Step 1: Add dgo type detection to `format.go`

```go
// pkg/lsp/semantic/format.go

// dgoTypeInfo holds information about a dgo type
type dgoTypeInfo struct {
    TypeName string       // "Result" or "Option"
    TypeArgs []types.Type // [T, E] for Result, [T] for Option
}

// detectDgoType checks if a type is dgo.Result or dgo.Option
func detectDgoType(t types.Type) *dgoTypeInfo {
    named, ok := t.(*types.Named)
    if !ok {
        return nil
    }

    typeName := named.Obj().Name()
    if typeName != "Result" && typeName != "Option" {
        return nil
    }

    pkg := named.Obj().Pkg()
    if pkg == nil {
        return nil
    }

    // Check for dgo package
    if !strings.Contains(pkg.Path(), "dgo") && pkg.Name() != "dgo" {
        return nil
    }

    // Extract type arguments
    var typeArgs []types.Type
    if args := named.TypeArgs(); args != nil {
        for i := 0; i < args.Len(); i++ {
            typeArgs = append(typeArgs, args.At(i))
        }
    }

    return &dgoTypeInfo{
        TypeName: typeName,
        TypeArgs: typeArgs,
    }
}
```

#### Step 2: Add hover formatters for dgo types

```go
// formatDgoResultHover formats hover for Result[T, E]
func formatDgoResultHover(info *dgoTypeInfo, pkg *types.Package) string {
    var b strings.Builder

    // Type signature
    b.WriteString("```go\n")
    b.WriteString("Result[")
    if len(info.TypeArgs) >= 1 {
        b.WriteString(formatType(info.TypeArgs[0], pkg))
    }
    b.WriteString(", ")
    if len(info.TypeArgs) >= 2 {
        b.WriteString(formatType(info.TypeArgs[1], pkg))
    }
    b.WriteString("]\n```\n\n")

    // Description
    b.WriteString("**Success OR failure container**\n\n")

    // Methods table
    tStr := "T"
    eStr := "E"
    if len(info.TypeArgs) >= 1 {
        tStr = formatType(info.TypeArgs[0], pkg)
    }
    if len(info.TypeArgs) >= 2 {
        eStr = formatType(info.TypeArgs[1], pkg)
    }

    b.WriteString("| Method | Returns | Description |\n")
    b.WriteString("|--------|---------|-------------|\n")
    b.WriteString(fmt.Sprintf("| `.IsOk()` | `bool` | Check if success |\n"))
    b.WriteString(fmt.Sprintf("| `.IsErr()` | `bool` | Check if error |\n"))
    b.WriteString(fmt.Sprintf("| `.MustOk()` | `%s` | Extract value (panics if Err) |\n", tStr))
    b.WriteString(fmt.Sprintf("| `.MustErr()` | `%s` | Extract error (panics if Ok) |\n", eStr))
    b.WriteString(fmt.Sprintf("| `.OkOr(default)` | `%s` | Extract with default |\n", tStr))

    // Constructors
    b.WriteString(fmt.Sprintf("\n*Constructors:* `Ok[%s, %s](value)`, `Err[%s, %s](err)`",
        tStr, eStr, tStr, eStr))

    return b.String()
}

// formatDgoOptionHover formats hover for Option[T]
func formatDgoOptionHover(info *dgoTypeInfo, pkg *types.Package) string {
    var b strings.Builder

    // Type signature
    b.WriteString("```go\n")
    b.WriteString("Option[")
    if len(info.TypeArgs) >= 1 {
        b.WriteString(formatType(info.TypeArgs[0], pkg))
    }
    b.WriteString("]\n```\n\n")

    // Description
    b.WriteString("**Optional value container** (Some or None)\n\n")

    // Methods table
    tStr := "T"
    if len(info.TypeArgs) >= 1 {
        tStr = formatType(info.TypeArgs[0], pkg)
    }

    b.WriteString("| Method | Returns | Description |\n")
    b.WriteString("|--------|---------|-------------|\n")
    b.WriteString(fmt.Sprintf("| `.IsSome()` | `bool` | Check if value present |\n"))
    b.WriteString(fmt.Sprintf("| `.IsNone()` | `bool` | Check if empty |\n"))
    b.WriteString(fmt.Sprintf("| `.MustSome()` | `%s` | Extract value (panics if None) |\n", tStr))
    b.WriteString(fmt.Sprintf("| `.SomeOr(default)` | `%s` | Extract with default |\n", tStr))

    // Constructors
    b.WriteString(fmt.Sprintf("\n*Constructors:* `Some[%s](value)`, `None[%s]()`", tStr, tStr))

    return b.String()
}
```

#### Step 3: Integrate into formatSignature/formatTypeHover

```go
// formatSignature formats the signature of an object
func formatSignature(obj types.Object, pkg *types.Package) string {
    if obj == nil {
        return ""
    }

    switch obj := obj.(type) {
    case *types.TypeName:
        // Check for dgo types FIRST
        if dgoInfo := detectDgoType(obj.Type()); dgoInfo != nil {
            switch dgoInfo.TypeName {
            case "Result":
                return formatDgoResultHover(dgoInfo, pkg)
            case "Option":
                return formatDgoOptionHover(dgoInfo, pkg)
            }
        }
        // Fall through to default formatting
        return fmt.Sprintf("type %s %s", obj.Name(), formatType(obj.Type().Underlying(), pkg))

    // ... rest of switch cases unchanged
    }
}

// formatTypeHover formats hover for expressions without objects
func formatTypeHover(entity *SemanticEntity, pkg *types.Package) string {
    // Check for dgo types FIRST
    if dgoInfo := detectDgoType(entity.Type); dgoInfo != nil {
        switch dgoInfo.TypeName {
        case "Result":
            return formatDgoResultHover(dgoInfo, pkg)
        case "Option":
            return formatDgoOptionHover(dgoInfo, pkg)
        }
    }

    // ... existing implementation
}
```

### File Changes Summary

| File | Changes |
|------|---------|
| `pkg/lsp/semantic/format.go` | Add `detectDgoType()`, `formatDgoResultHover()`, `formatDgoOptionHover()`, modify `formatSignature()` and `formatTypeHover()` |

No changes needed to:
- `builder.go` - detection happens at format time, not build time
- `types.go` - no new types needed
- `document.go` - no changes to document management

### Testing Strategy

1. **Unit test for detection:**
   ```go
   func TestDetectDgoType(t *testing.T) {
       // Create mock types.Named for Result[int, error]
       // Verify detection returns correct TypeName and TypeArgs
   }
   ```

2. **Integration test with LSP hover:**
   ```yaml
   # ai-docs/hover-specs/dgo_types.yaml
   tests:
     - name: "Result type hover"
       file: "examples/02_result/repository.go"
       position:
         line: 15
         character: 20
       expect_contains:
         - "Result[User, DBError]"
         - "Success OR failure"
         - ".IsOk()"
         - ".MustOk()"
   ```

3. **Manual VS Code verification:**
   - Open a .dingo file using Result types
   - Hover over return type
   - Verify rich documentation appears

### Edge Cases

1. **Nested dgo types:** `Result[Option[User], error]`
   - Inner Option should show Option hover
   - Outer Result shows Result hover with `Option[User]` as T

2. **Aliased imports:** `import r "github.com/MadAppGang/dingo/pkg/dgo"`
   - Detection uses package path, not alias

3. **Variables with dgo types:**
   ```go
   var result Result[int, error]
   ```
   - Hover on `result` should show variable info
   - Hover on `Result` should show dgo documentation

4. **Method receivers:**
   ```go
   func (r Result[T, E]) Custom() {}
   ```
   - Hover on method name shows method signature
   - Hover on `Result` in receiver shows dgo documentation

### Alternatives Considered

**Alternative 1: Modify SemanticMap Builder**
- Add dgo type detection during semantic map construction
- Store dgo info in DingoContext
- Rejected: Adds complexity to builder, format.go is simpler

**Alternative 2: Separate dgo hover handler**
- New file `format_dgo.go` with all dgo formatting
- Rejected: Over-engineering for two types, inline is cleaner

**Alternative 3: External documentation lookup**
- Load documentation from godoc or comments
- Rejected: dgo types are fixed, inline docs are simpler

## Complexity Assessment

**Simple/Medium/Complex:** Medium

- Detection logic is straightforward (type name + package check)
- Format functions are self-contained
- Integration requires modifying two existing functions
- Main risk: ensuring detection works for all import paths
