# Context-Aware Preprocessing Architecture

## Recommended Strategy: F - Hybrid Markers + AST Metadata

After analyzing all strategies, **Strategy F (Hybrid Markers + AST Metadata)** is the optimal choice for implementing context-aware features within the existing regex + go/parser architecture.

## Architecture Design

### Data Flow

```
.dingo file
    ↓
┌─────────────────────────────────────────────────────┐
│ Stage 1: Enhanced Regex Preprocessor                │
├─────────────────────────────────────────────────────┤
│ • Pattern matching: match expr {} → switch with    │
│   markers: /*DINGO:MATCH:START:id=m1*/            │
│ • Lambda closures: || expr → func() with markers  │
│ • Track construct IDs for correlation              │
│ • Emit valid Go + lightweight metadata comments    │
└─────────────────────────────────────────────────────┘
    ↓ (Valid Go with marker comments)
┌─────────────────────────────────────────────────────┐
│ Stage 2: Context-Aware AST Processing              │
├─────────────────────────────────────────────────────┤
│ • Parse with go/parser → AST                       │
│ • Extract markers from comment nodes               │
│ • Build context map using go/types                 │
│ • Enhanced plugin pipeline with context:           │
│   - PatternMatchPlugin (reads markers, validates)  │
│   - LambdaClosurePlugin (capture analysis)         │
│   - TypeInferencePlugin (go/types integration)     │
│ • Generate final .go + enhanced .sourcemap         │
└─────────────────────────────────────────────────────┘
```

### Pattern Matching Implementation

#### Input (Dingo)
```dingo
match getUserById(id) {
    Ok(user) => {
        println("Found: {}", user.name)
        return user
    }
    Err(NotFound) => {
        println("User not found")
        return defaultUser()
    }
    Err(e) => {
        println("Error: {}", e)
        return defaultUser()
    }
}
```

#### Stage 1: Preprocessor Output
```go
/*DINGO:MATCH:START:id=m1:var=_match_result:exhaustive=pending*/
_match_result := getUserById(id)
switch _match_discriminant := _match_result.(type) {
/*DINGO:MATCH:ARM:id=m1:pattern=Ok(user):binding=user:type=auto*/
case ResultOk:
    user := _match_discriminant.Value /*DINGO:BIND:user:from=Value*/
    {
        println(fmt.Sprintf("Found: %v", user.name))
        return user
    }
/*DINGO:MATCH:ARM:id=m1:pattern=Err(NotFound):const=true*/
case ResultErr:
    if _match_discriminant.Error == NotFound {
        println("User not found")
        return defaultUser()
    }
/*DINGO:MATCH:ARM:id=m1:pattern=Err(e):binding=e:type=auto*/
case ResultErr:
    e := _match_discriminant.Error /*DINGO:BIND:e:from=Error*/
    {
        println(fmt.Sprintf("Error: %v", e))
        return defaultUser()
    }
}
/*DINGO:MATCH:END:id=m1*/
```

#### Stage 2: AST Plugin Processing

```go
// PatternMatchPlugin
type PatternMatchPlugin struct {
    markers    map[string]*MatchMarker
    typeInfo   *types.Info
    scopes     map[ast.Node]*types.Scope
}

func (p *PatternMatchPlugin) Transform(node ast.Node, ctx *TransformContext) ast.Node {
    switch n := node.(type) {
    case *ast.Comment:
        if marker := parseMatchMarker(n.Text); marker != nil {
            p.markers[marker.ID] = marker
            // Store for exhaustiveness checking
            ctx.Metadata["match_"+marker.ID] = marker
        }

    case *ast.SwitchStmt:
        if marker, ok := p.findMarkerForNode(n); ok {
            // 1. Validate exhaustiveness
            matchType := p.inferMatchType(marker.Var)
            if !p.checkExhaustiveness(n, matchType) {
                ctx.AddError("non-exhaustive match at line %d", ctx.Line(n))
            }

            // 2. Validate bindings
            for _, arm := range p.extractArms(n) {
                if arm.Binding != "" {
                    p.validateBinding(arm, matchType)
                }
            }

            // 3. Transform if needed (e.g., add default case)
            if marker.Exhaustive == "pending" {
                n = p.addDefaultCase(n)
            }
        }
    }
    return node
}
```

#### Final Generated Go
```go
_match_result := getUserById(id)
switch _match_discriminant := _match_result.(type) {
case ResultOk:
    user := _match_discriminant.Value
    {
        println(fmt.Sprintf("Found: %v", user.name))
        return user
    }
case ResultErr:
    if _match_discriminant.Error == NotFound {
        println("User not found")
        return defaultUser()
    } else {
        e := _match_discriminant.Error
        println(fmt.Sprintf("Error: %v", e))
        return defaultUser()
    }
default:
    panic("unreachable: exhaustive match")
}
```

### Context Tracking Mechanism

#### Marker Format
```
/*DINGO:<CONSTRUCT>:<ACTION>:<key=value>:...*/

Constructs: MATCH, LAMBDA, GENERIC, SCOPE
Actions: START, END, ARM, BIND, CAPTURE
Keys: id, var, type, pattern, binding, exhaustive
```

#### Context Map Structure
```go
type TransformContext struct {
    // From go/types
    TypeInfo    *types.Info
    Scopes      map[ast.Node]*types.Scope

    // Dingo-specific
    Markers     map[string]Marker
    Bindings    map[string]*Binding
    Constructs  map[string]*DingoConstruct

    // Metadata storage
    Metadata    map[string]interface{}
}

type DingoConstruct struct {
    Type        string // "match", "lambda", etc.
    ID          string
    Node        ast.Node
    Parent      *DingoConstruct
    Children    []*DingoConstruct
    Scope       *types.Scope
    Bindings    []*Binding
}
```

### go/types Integration

```go
func (p *PatternMatchPlugin) inferMatchType(varName string) types.Type {
    // 1. Find the variable in AST
    var targetNode ast.Expr
    ast.Inspect(p.root, func(n ast.Node) bool {
        if ident, ok := n.(*ast.Ident); ok && ident.Name == varName {
            targetNode = ident
            return false
        }
        return true
    })

    // 2. Use go/types to get type
    if obj := p.typeInfo.ObjectOf(targetNode.(*ast.Ident)); obj != nil {
        return obj.Type()
    }

    // 3. Fallback: check if it's a known Dingo type
    if t := p.typeInfo.TypeOf(targetNode); t != nil {
        if named, ok := t.(*types.Named); ok {
            // Check for Result[T,E] or Option[T]
            if strings.HasPrefix(named.Obj().Name(), "Result") {
                return p.extractResultType(named)
            }
        }
    }

    return nil
}
```

### Performance Estimates

```
Current Baseline:
  - Regex preprocessing: ~5ms per 1000 LOC
  - AST processing: ~10ms per 1000 LOC
  - Total: ~15ms per 1000 LOC

With Context-Aware Enhancements:
  - Regex + markers: ~7ms per 1000 LOC (+2ms for marker generation)
  - AST parsing: ~10ms (unchanged, go/parser is fast)
  - Context building: ~8ms (marker extraction + go/types setup)
  - Plugin transforms: ~12ms (was 10ms, +2ms for context lookups)
  - Total: ~37ms per 1000 LOC

Performance Impact: 2.5x baseline (acceptable, under 50ms target)
```

### Migration Plan

#### Phase 1: Foundation (Week 1-2)
- [ ] Implement marker comment system in preprocessor
- [ ] Create MarkerParser for extracting markers from AST
- [ ] Build TransformContext structure
- [ ] Update plugin interface to accept context

#### Phase 2: Pattern Matching (Week 3-4)
- [ ] Implement match preprocessor with markers
- [ ] Create PatternMatchPlugin
- [ ] Add exhaustiveness checking
- [ ] Implement binding validation
- [ ] Test with golden tests

#### Phase 3: Advanced Features (Week 5-6)
- [ ] Lambda closure capture analysis
- [ ] Generic type inference enhancements
- [ ] Nested construct support
- [ ] Performance optimization
- [ ] Comprehensive test coverage

### Key Technical Advantages

1. **Minimal Changes**: Works within existing architecture, just adds markers
2. **Separation of Concerns**: Preprocessor stays simple (regex), plugins handle complexity
3. **Leverages go/types**: Full type information without custom type system
4. **Incremental**: Can be added feature by feature
5. **Debuggable**: Markers visible in intermediate output

### Risks and Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Marker parsing fragility | High | Use structured format, comprehensive tests |
| go/types limitations with Dingo types | Medium | Maintain type registry for Result/Option |
| Performance regression | Medium | Profile and optimize hot paths, cache context |
| Complex nested constructs | Low | Hierarchical marker IDs, parent tracking |

### Success Metrics

1. **Correctness**: 100% of pattern matching tests pass
2. **Performance**: <50ms per 1000 LOC (3x baseline max)
3. **Exhaustiveness**: Catches all non-exhaustive matches at compile time
4. **Type Safety**: All bindings have correct types
5. **Maintainability**: <500 LOC added to preprocessor, <1000 LOC for plugins

## Conclusion

Strategy F (Hybrid Markers + AST Metadata) provides the best balance of:
- **Feasibility**: Works with current architecture
- **Simplicity**: Markers are lightweight, plugins do heavy lifting
- **Power**: Full context awareness via go/types
- **Performance**: Acceptable overhead (2.5x baseline)
- **Extensibility**: Easy to add new context-aware features

This approach allows us to implement pattern matching and other context-aware features without fundamentally changing the successful regex + go/parser architecture.