# Implementation Patterns for Dingo Hover

Code patterns extracted from gopls that can be directly adapted for Dingo.

## Pattern 1: Position to AST Node

### gopls Version
```go
// From hover.go
func Hover(ctx context.Context, snapshot Snapshot, fh file.Handle, position protocol.Position) {
    // Get parsed file
    pkg, pgf, err := parsedGoFile(ctx, snapshot, fh)

    // Convert LSP position (0-based line/char) to token.Pos
    pos, err := pgf.PositionPos(position)

    // Find AST node at position using Inspector
    curFile := pkg.FileOfCursor(pgf.Cursor)
    curNode, ok := curFile.FindByPos(pos, pos)
}
```

### Dingo Adaptation
```go
// For Dingo LSP
func (s *Server) findNodeAtPosition(uri string, pos protocol.Position) (dingoast.Node, error) {
    // Get parsed Dingo file
    file, fset, err := s.parseDingoFile(uri)
    if err != nil {
        return nil, err
    }

    // Convert LSP position to token.Pos
    tokFile := fset.File(file.Pos())
    line := int(pos.Line) + 1    // LSP is 0-indexed
    col := int(pos.Character) + 1
    offset := tokFile.LineStart(line) + token.Pos(col-1)
    tokenPos := tokFile.Pos(int(offset))

    // Find node at position
    return s.dingoInspector.FindByPos(tokenPos)
}
```

## Pattern 2: Object Resolution

### gopls Version
```go
// From hover.go - objectsAt function
func objectsAt(info *types.Info, cur inspector.Cursor) ([]types.Object, *ast.SelectorExpr) {
    for c := range cur.Children() {
        switch n := c.Node().(type) {
        case *ast.Ident:
            if obj := info.ObjectOf(n); obj != nil {
                return []types.Object{obj}, nil
            }
        case *ast.SelectorExpr:
            return objectsAt(info, c)
        }
    }
    return nil, nil
}
```

### Dingo Adaptation
```go
// For Dingo - resolve object from type info
func (s *Server) resolveObject(info *types.Info, node ast.Node) types.Object {
    switch n := node.(type) {
    case *ast.Ident:
        return info.ObjectOf(n)
    case *ast.SelectorExpr:
        // Recurse into selector
        return info.ObjectOf(n.Sel)
    case *ast.CallExpr:
        // Get the function being called
        return s.resolveObject(info, n.Fun)
    }
    return nil
}
```

## Pattern 3: Type String Formatting

### gopls Version
```go
// Qualifier function controls package name display
func (h *hoverHandler) qualifier(pkg *types.Package) string {
    if pkg == h.currentPkg {
        return ""
    }
    return pkg.Name()
}

// Format object signature
signature := types.ObjectString(obj, h.qualifier)
```

### Dingo Adaptation
```go
// pkg/lsp/format.go
func formatSignature(obj types.Object, currentPkg *types.Package) string {
    qualifier := func(pkg *types.Package) string {
        if pkg == currentPkg {
            return ""
        }
        return pkg.Name()
    }
    return types.ObjectString(obj, qualifier)
}

// Format type for display
func formatType(t types.Type, currentPkg *types.Package) string {
    qualifier := func(pkg *types.Package) string {
        if pkg == currentPkg {
            return ""
        }
        return pkg.Name()
    }
    return types.TypeString(t, qualifier)
}
```

## Pattern 4: Hover Content Generation

### gopls Version
```go
func (h *hoverHandler) format(obj types.Object, decl ast.Node) *protocol.Hover {
    // Build signature
    signature := types.ObjectString(obj, h.qualifier)

    // Get documentation
    doc := extractDoc(decl)

    // Format as markdown
    content := fmt.Sprintf("```go\n%s\n```\n\n%s", signature, doc)

    return &protocol.Hover{
        Contents: protocol.MarkupContent{
            Kind:  protocol.Markdown,
            Value: content,
        },
        Range: h.nodeRange(obj),
    }
}
```

### Dingo Adaptation
```go
// pkg/lsp/hover.go
func formatHover(obj types.Object, typeStr string, doc string, dingoInfo *DingoContext) *protocol.Hover {
    var content strings.Builder

    // Signature block
    content.WriteString("```go\n")
    content.WriteString(typeStr)
    content.WriteString("\n```\n\n")

    // Dingo-specific info (e.g., error propagation)
    if dingoInfo != nil && dingoInfo.Kind != "" {
        content.WriteString(fmt.Sprintf("*%s*\n\n", dingoInfo.Description))
    }

    // Documentation
    if doc != "" {
        content.WriteString(doc)
    }

    return &protocol.Hover{
        Contents: protocol.MarkupContent{
            Kind:  protocol.Markdown,
            Value: content.String(),
        },
    }
}

type DingoContext struct {
    Kind        string // "error_propagation", "safe_nav", "null_coalesce", etc.
    Description string // Human-readable description
}
```

## Pattern 5: Type Checking Integration

### gopls Version
```go
// gopls maintains types.Info per package
type Package struct {
    pkg       *types.Package
    typesInfo *types.Info
    files     []*ParsedGoFile
}

func (p *Package) TypesInfo() *types.Info { return p.typesInfo }
```

### Dingo Adaptation
```go
// pkg/lsp/typeinfo.go
type TypedFile struct {
    DingoFile   *dingoast.File
    GoFile      *ast.File
    DingoFset   *token.FileSet
    GoFset      *token.FileSet
    TypesInfo   *types.Info
    TypesPkg    *types.Package
    SemanticMap map[token.Pos]SemanticInfo
}

type SemanticInfo struct {
    Kind     SemanticKind
    Object   types.Object    // For named entities
    Type     types.Type      // For expressions
    DingoPos token.Pos       // Position in Dingo source
    GoPos    token.Pos       // Position in Go source
}

type SemanticKind int
const (
    SemanticIdent SemanticKind = iota
    SemanticCall
    SemanticField
    SemanticType
    SemanticErrorProp  // Dingo-specific
    SemanticSafeNav    // Dingo-specific
    SemanticNullCoal   // Dingo-specific
)

func BuildTypedFile(dingoPath string, dingoSrc []byte) (*TypedFile, error) {
    // 1. Parse Dingo
    dingoFset := token.NewFileSet()
    dingoFile, err := parser.ParseFile(dingoFset, dingoPath, dingoSrc, 0)

    // 2. Transpile to Go
    goSrc, sourcemap, err := transpiler.Transpile(dingoFset, dingoFile)

    // 3. Parse Go
    goFset := token.NewFileSet()
    goFile, err := goparser.ParseFile(goFset, "gen.go", goSrc, goparser.ParseComments)

    // 4. Type check Go
    conf := types.Config{Importer: importer.Default()}
    info := &types.Info{
        Types:      make(map[ast.Expr]types.TypeAndValue),
        Defs:       make(map[*ast.Ident]types.Object),
        Uses:       make(map[*ast.Ident]types.Object),
        Selections: make(map[*ast.SelectorExpr]*types.Selection),
    }
    pkg, err := conf.Check("main", goFset, []*ast.File{goFile}, info)

    // 5. Build semantic map
    semanticMap := buildSemanticMap(sourcemap, goFile, info)

    return &TypedFile{
        DingoFile:   dingoFile,
        GoFile:      goFile,
        DingoFset:   dingoFset,
        GoFset:      goFset,
        TypesInfo:   info,
        TypesPkg:    pkg,
        SemanticMap: semanticMap,
    }, nil
}
```

## Pattern 6: Semantic Map Construction

```go
// pkg/lsp/semantic_map.go
func buildSemanticMap(sm *sourcemap.Map, goFile *ast.File, info *types.Info) map[token.Pos]SemanticInfo {
    result := make(map[token.Pos]SemanticInfo)

    // Walk Go AST and map back to Dingo positions
    ast.Inspect(goFile, func(n ast.Node) bool {
        if n == nil {
            return true
        }

        // Get Dingo position from sourcemap
        dingoPos, ok := sm.GotoDingo(n.Pos())
        if !ok {
            return true // Generated code without Dingo mapping
        }

        switch node := n.(type) {
        case *ast.Ident:
            if obj := info.ObjectOf(node); obj != nil {
                result[dingoPos] = SemanticInfo{
                    Kind:     SemanticIdent,
                    Object:   obj,
                    Type:     obj.Type(),
                    DingoPos: dingoPos,
                    GoPos:    node.Pos(),
                }
            }

        case *ast.CallExpr:
            if tv, ok := info.Types[node]; ok {
                result[dingoPos] = SemanticInfo{
                    Kind:     SemanticCall,
                    Type:     tv.Type,
                    DingoPos: dingoPos,
                    GoPos:    node.Pos(),
                }
            }

        case *ast.SelectorExpr:
            if sel := info.Selections[node]; sel != nil {
                result[dingoPos] = SemanticInfo{
                    Kind:     SemanticField,
                    Object:   sel.Obj(),
                    Type:     sel.Type(),
                    DingoPos: dingoPos,
                    GoPos:    node.Pos(),
                }
            }
        }

        return true
    })

    return result
}
```

## Pattern 7: Hover Request Handler

```go
// pkg/lsp/hover.go
func (s *Server) Hover(ctx context.Context, params *protocol.HoverParams) (*protocol.Hover, error) {
    uri := params.TextDocument.URI
    pos := params.Position

    // 1. Get typed file (cached)
    tf, err := s.getTypedFile(uri)
    if err != nil {
        return nil, err
    }

    // 2. Convert LSP position to Dingo token.Pos
    dingoPos := s.lspToTokenPos(tf.DingoFset, uri, pos)

    // 3. Look up semantic info
    sem, ok := tf.SemanticMap[dingoPos]
    if !ok {
        // Try nearby positions (for token center vs start)
        sem, ok = s.findNearestSemantic(tf.SemanticMap, dingoPos, 3)
        if !ok {
            return nil, nil // No hover info
        }
    }

    // 4. Build hover content
    var typeStr string
    var doc string

    if sem.Object != nil {
        typeStr = formatSignature(sem.Object, tf.TypesPkg)
        doc = s.getDocumentation(sem.Object)
    } else if sem.Type != nil {
        typeStr = formatType(sem.Type, tf.TypesPkg)
    }

    // 5. Add Dingo context
    dingoCtx := s.getDingoContext(tf, dingoPos)

    return formatHover(sem.Object, typeStr, doc, dingoCtx), nil
}

func (s *Server) getDingoContext(tf *TypedFile, pos token.Pos) *DingoContext {
    // Check if position is within a Dingo-specific construct
    // This uses the Dingo AST, not the Go AST

    // Walk Dingo AST to find enclosing node
    var ctx *DingoContext

    // ... walk Dingo AST looking for ErrorProp, SafeNav, etc.

    return ctx
}
```

## Pattern 8: Inspector for Dingo AST

```go
// pkg/ast/inspector.go
type DingoInspector struct {
    events []dingoEvent
}

type dingoEvent struct {
    node   dingoast.Node
    index  int32  // Index of pop event (if push) or push event (if pop)
    parent int32
}

func NewDingoInspector(file *dingoast.File) *DingoInspector {
    in := &DingoInspector{}
    in.traverse(file)
    return in
}

func (in *DingoInspector) FindByPos(pos token.Pos) (dingoast.Node, bool) {
    // Find innermost node containing pos
    var result dingoast.Node

    for i := 0; i < len(in.events); i++ {
        ev := in.events[i]
        if ev.index > int32(i) { // push event
            node := ev.node
            if node.Pos() <= pos && pos < node.End() {
                result = node // Keep narrowing down
            } else if node.Pos() > pos {
                // Past the position, stop
                break
            }
        }
    }

    return result, result != nil
}

func (in *DingoInspector) Enclosing(pos token.Pos) []dingoast.Node {
    var stack []dingoast.Node

    for i := 0; i < len(in.events); i++ {
        ev := in.events[i]
        if ev.index > int32(i) { // push
            if ev.node.Pos() <= pos && pos < ev.node.End() {
                stack = append(stack, ev.node)
            }
        }
    }

    // Return from innermost to outermost
    for i, j := 0, len(stack)-1; i < j; i, j = i+1, j-1 {
        stack[i], stack[j] = stack[j], stack[i]
    }

    return stack
}
```

## Key Differences from gopls

| Aspect | gopls | Dingo Hover |
|--------|-------|-------------|
| AST Source | Go source directly | Dingo source + generated Go |
| Type Info Source | types.Info from Go | types.Info from generated Go |
| Position System | Direct token.Pos | Dingo pos → sourcemap → Go pos |
| Inspector | Pre-built for Go AST | Need for Dingo AST |
| Hover Content | Go-focused | Dingo-aware (show ? syntax, etc.) |

## Testing Strategy

```go
// pkg/lsp/hover_test.go
func TestHover(t *testing.T) {
    tests := []struct {
        name     string
        dingo    string
        line     int
        char     int
        wantType string
    }{
        {
            name:     "simple variable",
            dingo:    `package main\nvar x int = 42`,
            line:     1,
            char:     4,  // 'x'
            wantType: "var x int",
        },
        {
            name:     "error propagation result",
            dingo:    `package main\nfunc f() Result[int, error] { x := getInt()?; return Ok(x) }`,
            line:     1,
            char:     38, // 'getInt'
            wantType: "func getInt() Result[int, error]",
        },
        {
            name:     "safe navigation",
            dingo:    `package main\nfunc f(u *User) string { return u?.Name }`,
            line:     1,
            char:     45, // 'Name'
            wantType: "field Name string",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Setup server with test file
            // Call Hover
            // Assert result contains tt.wantType
        })
    }
}
```
