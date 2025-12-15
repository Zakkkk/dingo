# Dingo LSP Hover: Solution Comparison

## The Problem

When hovering on Dingo code, our current approach:
1. Translates Dingo position → Go position
2. Forwards request to gopls
3. Translates Go response → Dingo response

**This fundamentally fails** because Dingo code structure doesn't match Go code:

```dingo
// Dingo: Single line with error propagation
user := loadUserFromDB(userID) ? "database lookup failed"
```

```go
// Generated Go: Multiple lines with temp variables
tmp1, err1 := loadUserFromDB(userID)
if err1 != nil {
    return dgo.Err[User, error](fmt.Errorf("database lookup failed: %w", err1))
}
user := tmp1
```

**Hover on `user` in Dingo → maps to wrong line in Go (the `user := tmp1` line, not the function call)**

**Hover on non-existent variables → `tmp1`, `err1` don't exist in Dingo source**

## Solution 2: AST-Aware Hover Translation

### Concept
Maintain a mapping between Dingo AST nodes and generated Go AST nodes. When hovering, find the Dingo AST node, look up its corresponding Go AST node(s), and use gopls to get type info for the right Go node.

### Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                     Dingo LSP Server                             │
├─────────────────────────────────────────────────────────────────┤
│  1. Parse Dingo file → Dingo AST                                │
│  2. FindByPos(cursor) → Dingo AST Node                          │
│  3. Look up AST mapping: DingoNode → GoNode(s)                  │
│  4. For each GoNode, query gopls hover                          │
│  5. Aggregate and format response                                │
└─────────────────────────────────────────────────────────────────┘
```

### What We Already Have

| Component | Status | Location |
|-----------|--------|----------|
| Dingo Parser | ✅ Complete | `pkg/parser/` |
| Dingo AST | ✅ Complete | `pkg/ast/` |
| Expression Finder | ✅ Complete | `pkg/ast/expr_finder.go` |
| Go Parser | ✅ Complete | `go/parser` (stdlib) |
| Type Checker | ✅ Partial | `pkg/typechecker/` |
| AST Inspector | ❌ Missing | Need Dingo equivalent |
| AST Mapping | ❌ Missing | Need DingoNode → GoNode map |

### Required Work

1. **Build AST Mapping during Transpilation**
   ```go
   type ASTMapping struct {
       DingoNode  ast.Node     // Source Dingo AST node
       GoNodes    []ast.Node   // Generated Go AST node(s)
       Semantic   SemanticKind // What this represents
   }

   type SemanticKind int
   const (
       SemanticVariable SemanticKind = iota  // Variable declaration
       SemanticCall                          // Function call
       SemanticField                         // Field access
       SemanticType                          // Type expression
   )
   ```

2. **Dingo Inspector** (like go/ast/inspector)
   ```go
   type DingoInspector struct {
       events []event
   }

   // FindByPos finds Dingo AST node at position
   func (in *DingoInspector) FindByPos(pos token.Pos) DingoNode
   ```

3. **Hover Resolution Logic**
   ```go
   func (s *Server) handleHover(pos Position) *Hover {
       // 1. Find Dingo AST node
       dingoNode := s.dingoInspector.FindByPos(pos)

       // 2. Look up corresponding Go node(s)
       goNodes := s.astMapping[dingoNode]

       // 3. For expression hover, find the "primary" Go node
       primaryGoNode := selectPrimaryNode(goNodes, dingoNode)

       // 4. Query gopls for that specific Go node position
       goPos := primaryGoNode.Pos()
       hoverResult := s.gopls.Hover(goPos)

       return hoverResult
   }
   ```

### Complexity Analysis

**Pros:**
- Reuses gopls for all type information
- Accurate for 1:1 mappings (most code)
- Handles complex cases with explicit mapping

**Cons:**
- Requires maintaining complex AST mapping
- Mapping can be fragile (transform changes break mapping)
- Generated temp variables have no good hover
- ~3000 LOC estimated for proper implementation

### Edge Cases

| Dingo Construct | Go Generated | Hover Strategy |
|-----------------|--------------|----------------|
| `x?` error prop | `tmp, err := x; if err...` | Hover on function call, not tmp |
| `x?.y` safe nav | `if x != nil { x.y }` | Show type of `x.y` from conditional |
| `a ?? b` coalesce | `if a != nil { a } else { b }` | Show type of result |
| Lambda `\|x\| x*2` | Anonymous func | Map to func literal |
| Match expr | Switch statement | Map to switch |

---

## Solution 3: Dingo-Native Type Information

### Concept
Don't rely on gopls for hover. Instead, run `go/types` directly on Dingo's generated Go code and build our own hover content.

### Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                     Dingo LSP Server                             │
├─────────────────────────────────────────────────────────────────┤
│  1. Parse Dingo file → Dingo AST                                │
│  2. Transpile → Go source + Sourcemap                           │
│  3. Parse Go source → Go AST                                    │
│  4. Type-check Go AST → types.Info                              │
│  5. On hover: FindByPos → Dingo node → extract type info        │
│  6. Format hover response ourselves                              │
└─────────────────────────────────────────────────────────────────┘
```

### What We Already Have

| Component | Status | Location |
|-----------|--------|----------|
| Dingo Parser | ✅ Complete | `pkg/parser/` |
| Dingo AST | ✅ Complete | `pkg/ast/` |
| Transpiler | ✅ Complete | `pkg/transpiler/` |
| Go Parser | ✅ Complete | `go/parser` (stdlib) |
| Type Checker Wrapper | ✅ Partial | `pkg/typechecker/` |
| Expression Type Cache | ✅ Exists | `typechecker.ExprTypeCache` |

### Required Work

1. **Enhanced Sourcemap with Semantic Info**
   ```go
   type SemanticMapping struct {
       DingoPos     token.Pos    // Position in Dingo
       DingoEnd     token.Pos    // End position
       Kind         EntityKind   // What is this?
       GoExprPath   string       // Path to find in Go AST
       TypeString   string       // Cached type string
   }

   type EntityKind int
   const (
       EntityIdent EntityKind = iota
       EntityCall
       EntityField
       EntityType
   )
   ```

2. **Type Info Collection Pass**
   ```go
   func CollectTypeInfo(goFile *ast.File, info *types.Info) map[token.Pos]TypeInfo {
       result := make(map[token.Pos]TypeInfo)

       ast.Inspect(goFile, func(n ast.Node) bool {
           switch node := n.(type) {
           case *ast.Ident:
               if obj := info.ObjectOf(node); obj != nil {
                   result[node.Pos()] = TypeInfo{
                       Object: obj,
                       Type:   obj.Type(),
                   }
               }
           case *ast.CallExpr:
               if tv, ok := info.Types[node]; ok {
                   result[node.Pos()] = TypeInfo{Type: tv.Type}
               }
           }
           return true
       })

       return result
   }
   ```

3. **Hover Generation**
   ```go
   func (s *Server) handleHover(pos Position) *Hover {
       // 1. Find Dingo entity at position
       entity := s.findDingoEntity(pos)

       // 2. Look up type info from our collected data
       typeInfo := s.typeInfoMap[entity.ID]

       // 3. Format hover content (like gopls does)
       signature := types.ObjectString(typeInfo.Object, s.qualifier)
       doc := s.findDocComment(entity)

       return formatHover(signature, doc)
   }
   ```

### Complexity Analysis

**Pros:**
- Full control over hover content
- Can show Dingo-specific information (e.g., "Result[T, E] with error propagation")
- No complex AST mapping required
- Easier to handle synthetic variables (just don't show them)
- ~1500 LOC estimated

**Cons:**
- Must implement hover formatting ourselves
- Need to handle all hover cases (types, funcs, vars, fields, etc.)
- Documentation extraction requires extra work
- Potential divergence from gopls behavior

### Edge Cases

| Dingo Construct | Hover Strategy |
|-----------------|----------------|
| `x?` error prop | Show type of `x` (the call), note "error propagation" |
| `x?.y` safe nav | Show type of `y` field, note "nil-safe" |
| `a ?? b` coalesce | Show result type, note "nil coalesce" |
| Lambda | Show lambda signature with param types |
| Match expr | Show match expression type |

---

## Recommendation: Hybrid Approach

After analyzing both solutions and the gopls implementation, I recommend a **hybrid approach**:

### Phase 1: Basic Native Type Info (Solution 3 foundation)

1. Run `go/types` on generated Go code during transpilation
2. Build a simple semantic map: `DingoPos → TypeInfo`
3. Handle hover for identifiers, calls, and fields using our type info
4. Format hover output similar to gopls

**Deliverable:** Hover works for most common cases without gopls dependency

### Phase 2: gopls Delegation for Complex Cases

1. Keep gopls running as backend
2. For cases our simple system can't handle:
   - Documentation lookup (go doc integration)
   - Cross-package type resolution
   - Package-level hover

**Deliverable:** Full hover parity with gopls for edge cases

### Phase 3: Dingo-Enhanced Hover

1. Add Dingo-specific hover information:
   - "This Result type uses error propagation (?)"
   - "Safe navigation chain: x?.y?.z"
   - "Match expression with 3 arms"
2. Better error messages for transformed code

**Deliverable:** Hover that's better than gopls for Dingo code

---

## Implementation Priority

### Must Have (Phase 1)
- [ ] Type info collection during transpile
- [ ] Basic hover for identifiers
- [ ] Signature formatting for functions
- [ ] Type display for variables

### Should Have (Phase 2)
- [ ] Documentation from source
- [ ] gopls fallback for edge cases
- [ ] Package-level hover

### Nice to Have (Phase 3)
- [ ] Dingo-specific annotations
- [ ] Match expression hover
- [ ] Lambda signature inference

---

## Files to Create/Modify

```
pkg/lsp/
├── hover.go           # NEW: Dingo hover implementation
├── typeinfo.go        # NEW: Type info collection
├── semantic_map.go    # NEW: DingoPos → TypeInfo mapping
├── format.go          # NEW: Hover content formatting
├── handlers.go        # MODIFY: Use new hover system
└── translator.go      # MODIFY: Simplify (less position translation)

pkg/transpiler/
├── pure_pipeline.go   # MODIFY: Return type info with output
└── semantic_info.go   # NEW: Collect semantic info during transpile
```

## LOC Estimates

| Component | Phase 1 | Phase 2 | Phase 3 |
|-----------|---------|---------|---------|
| Type collection | 300 | +100 | +50 |
| Semantic mapping | 400 | +150 | +100 |
| Hover formatting | 300 | +200 | +150 |
| gopls integration | - | 400 | - |
| Tests | 300 | +200 | +100 |
| **Total** | **1300** | **+1050** | **+400** |

## Success Criteria

1. **Accuracy**: Hover shows correct type for 95%+ of identifiers
2. **Performance**: Hover response < 100ms
3. **Parity**: No worse than current gopls pass-through for basic cases
4. **Dingo-aware**: Shows useful info for Dingo-specific syntax
