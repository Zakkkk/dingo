# gopls Hover Implementation Patterns

Reference documentation extracted from `golang.org/x/tools/gopls/internal/golang/hover.go` and related packages.

## Overview

gopls hover implementation is approximately 1900 lines and handles multiple scenarios:
- Go source code hover (identifiers, expressions, types)
- Go.mod file hover
- Linkified documentation
- Package-level summaries

## Core Architecture

### Entry Point: `Hover(ctx, snapshot, fh, position)`

```go
func Hover(ctx context.Context, snapshot *cache.Snapshot, fh file.Handle, position protocol.Position) (*protocol.Hover, error) {
    // 1. Parse the file to get AST
    pkg, pgf, err := parsedGoFile(ctx, snapshot, fh)

    // 2. Convert LSP position to token.Pos
    pos, err := pgf.PositionPos(position)

    // 3. Build AST inspector and find cursor at position
    curFile := pkg.FileOfCursor(pgf.Cursor)
    curNode, ok := curFile.FindByPos(pos, pos)

    // 4. Find the hovered object(s)
    objects, _ := objectsAt(pkg.TypesInfo(), curNode)

    // 5. Generate hover content for the object
    h, err := hoverObjects(ctx, snapshot, pkg, pgf, objects)

    return &protocol.Hover{Contents: h.Contents, Range: h.Range}, nil
}
```

### Key Pattern 1: Cursor-Based Navigation

gopls uses `inspector.Cursor` for efficient AST navigation:

```go
// Finding node at position
curNode, ok := curFile.FindByPos(start, end)

// Walking enclosing nodes (from inner to outer)
for cur := range curNode.Enclosing() {
    switch node := cur.Node().(type) {
    case *ast.CallExpr:
        // Handle call expression context
    case *ast.SelectorExpr:
        // Handle selector context
    }
}

// Walking children
for child := range curNode.Children() {
    // Process each child
}
```

### Key Pattern 2: ObjectsAt Resolution

The `objectsAt` function finds what identifiers/objects are at a cursor position:

```go
func objectsAt(info *types.Info, cur inspector.Cursor) ([]types.Object, *ast.SelectorExpr) {
    // Walk children looking for identifiers
    for c := range cur.Children() {
        switch n := c.Node().(type) {
        case *ast.Ident:
            if obj := info.ObjectOf(n); obj != nil {
                return []types.Object{obj}, nil
            }
            // Handle implicit object (embedded field)
            if obj := info.Implicit(n); obj != nil {
                return []types.Object{obj}, nil
            }
        case *ast.SelectorExpr:
            // Return the selector's identifier object
            return objectsAt(info, c)
        }
    }
    return nil, nil
}
```

### Key Pattern 3: TypesInfo Usage

gopls relies heavily on `types.Info` for:

```go
// Object resolution (what does this identifier refer to?)
obj := info.ObjectOf(ident)        // Named object (var, func, type, etc.)
obj := info.Implicit(node)          // Implicit object (embedded fields)

// Type resolution (what type does this expression have?)
tv := info.Types[expr]              // TypeAndValue
typeOf := tv.Type                   // The type
isConst := tv.Value != nil          // Compile-time constant?

// Selection resolution (what field/method is being selected?)
sel := info.Selections[selectorExpr]
obj := sel.Obj()                    // The selected field/method
```

### Key Pattern 4: Declaration Finding

```go
// Find where an object is declared
func findDeclInfo(pkg Package, obj types.Object) (decl ast.Node, spec ast.Spec) {
    // Search through all files in the package
    for _, file := range pkg.Files() {
        ast.Inspect(file, func(n ast.Node) bool {
            switch d := n.(type) {
            case *ast.FuncDecl:
                if d.Name.Name == obj.Name() {
                    return d
                }
            case *ast.GenDecl:
                for _, spec := range d.Specs {
                    // Check ValueSpec, TypeSpec, etc.
                }
            }
            return true
        })
    }
}
```

### Key Pattern 5: Hover Content Generation

```go
func hoverObjects(ctx context.Context, snapshot Snapshot, pkg Package, pgf *ParsedGoFile, objects []types.Object) (*hoverResult, error) {
    obj := objects[0] // Primary object

    // 1. Get declaration for doc comment extraction
    decl, spec := findDeclInfo(pkg, obj)

    // 2. Build signature string
    signature := types.ObjectString(obj, qualifier)

    // 3. Extract doc comment
    var doc string
    if spec != nil {
        doc = spec.Doc.Text()
    }

    // 4. Format as markdown
    content := fmt.Sprintf("```go\n%s\n```\n\n%s", signature, doc)

    return &hoverResult{
        Contents: protocol.MarkupContent{Kind: "markdown", Value: content},
        Range:    nodeRange(pgf, obj),
    }
}
```

## Inspector Package Patterns

### Pre-computed Traversal

The `inspector.Inspector` pre-computes traversal events for efficiency:

```go
type Inspector struct {
    events []event  // Push/pop events
}

type event struct {
    node   ast.Node
    typ    uint64   // Bitmask for type filtering
    index  int32    // Index of corresponding push/pop
    parent int32    // Parent's push index
}
```

### Efficient Type Filtering

```go
// Type filtering uses bitmasks for O(1) type checking
func (in *Inspector) Preorder(types []ast.Node, f func(ast.Node)) {
    mask := maskOf(types)  // Compute bitmask once

    for i := 0; i < len(in.events); i++ {
        ev := in.events[i]
        if ev.typ&mask != 0 {
            f(ev.node)
        }
        // Skip subtrees that don't contain matching types
        if in.events[ev.index].typ&mask == 0 {
            i = ev.index  // Jump to pop event
        }
    }
}
```

### Cursor API

```go
// Navigate to parent
parent := cur.Parent()

// Find by position (innermost node containing range)
cur, ok := curFile.FindByPos(startPos, endPos)

// Iterate enclosing nodes (inner to outer)
for c := range cur.Enclosing((*ast.FuncDecl)(nil)) {
    // c is a Cursor to an *ast.FuncDecl
}

// Check parent edge kind
if astutil.IsChildOf(cur, edge.FuncDecl_Body) {
    // Inside function body
}
```

## AstUtil Package Patterns

### Range Utilities

```go
// Range represents a Pos interval
type Range struct{ Start, EndPos token.Pos }

// Get range of any AST node (handles ast.File specially)
func NodeRange(n ast.Node) Range {
    if file, ok := n.(*ast.File); ok {
        return Range{file.FileStart, file.FileEnd}
    }
    return Range{n.Pos(), n.End()}
}

// Check if range contains position
func (r Range) ContainsPos(pos token.Pos) bool
func (r Range) Contains(rng Range) bool
```

### Selection API

```go
// Select returns enclosing node + first/last selected nodes
func Select(curFile Cursor, start, end token.Pos) (enclosing, first, last Cursor, error) {
    // 1. Find innermost node containing range
    enclosing, _ := curFile.FindByPos(start, end)

    // 2. Find first/last nodes wholly within range
    for cur := range enclosing.Preorder() {
        if rng.Contains(NodeRange(cur.Node())) {
            // Track first (smallest Pos) and last (largest End)
        }
    }

    return enclosing, first, last, nil
}
```

## Type System Integration

### go/types Key Structures

```go
// types.Info - the central type checking result
type Info struct {
    Types      map[ast.Expr]TypeAndValue  // Types of expressions
    Defs       map[*ast.Ident]Object      // Definitions
    Uses       map[*ast.Ident]Object      // Uses of identifiers
    Selections map[*ast.SelectorExpr]*Selection
    Scopes     map[ast.Node]*Scope
    Implicit   map[ast.Node]Object        // Implicit objects
}

// types.Object - anything with a name
interface Object {
    Name() string
    Type() Type
    Pos() token.Pos      // Declaration position
    Pkg() *Package
}

// Object implementations:
// - *Var, *Const, *Func, *TypeName, *Label, *PkgName, *Builtin, *Nil
```

### Qualifier Function

```go
// Qualifier controls how types are printed
func qualifier(pkg *types.Package) string {
    if pkg == currentPkg {
        return ""  // Don't qualify types from current package
    }
    return pkg.Name()  // Use package name for imports
}

// Usage
signature := types.ObjectString(obj, qualifier)
// "func (s *Server) HandleRequest(ctx context.Context) error"
```

## Key Insights for Dingo LSP

1. **Position → AST is fundamental**: gopls converts LSP position to `token.Pos`, then finds AST node via `FindByPos`. This is reliable because there's a 1:1 mapping.

2. **Type info comes from `types.Info`**: All type resolution uses the pre-computed type checker results, not AST inspection.

3. **Cursor enables navigation**: The Cursor API allows finding enclosing nodes, checking parent relationships, and traversing efficiently.

4. **Inspector optimizes repeated traversal**: For multiple queries on same files, the pre-computed event list is 2.5x faster than `ast.Inspect`.

5. **Selection handles user's imprecise ranges**: The `Select` function tolerates whitespace and returns the "meaningful" selection.
