# Context-Aware Preprocessing Implementation Strategy

## Executive Summary

**Recommended Strategy: F - Hybrid Markers + AST Metadata**

This approach combines lightweight regex-based markers in Stage 1 with rich AST context building in Stage 2, providing the optimal balance of simplicity, performance, and extensibility.

## Detailed Analysis

### Why Strategy F (Hybrid) Wins

**Key Advantages:**
1. **Separation of Concerns**: Regex handles syntax transformation, AST handles semantic analysis
2. **Minimal Complexity**: No multi-pass preprocessing or complex state management
3. **Leverages go/types**: Full type information without reinventing the wheel
4. **Extensible**: Easy to add new context-aware features
5. **Performance**: Single pass preprocessing + efficient AST walk

### Implementation Architecture

```
┌─────────────────────────────────────────┐
│ Stage 1: Enhanced Regex Preprocessor    │
├─────────────────────────────────────────┤
│ • Transform Dingo syntax to Go          │
│ • Emit lightweight markers in comments  │
│ • Preserve position mapping              │
│ • Output: Valid Go with metadata        │
└─────────────────────────────────────────┘
                    ↓
┌─────────────────────────────────────────┐
│ Stage 2: Context-Aware AST Processing   │
├─────────────────────────────────────────┤
│ • Parse with go/parser                  │
│ • Extract markers from AST comments     │
│ • Build context via go/types            │
│ • Validate + transform with full context│
│ • Generate final .go + .sourcemap       │
└─────────────────────────────────────────┘
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

#### Stage 1 Output (Preprocessed Go with Markers)
```go
/* @dingo:match:start expr="getUserById(id)" type="Result[User,Error]" */
switch __dingo_match_0 := getUserById(id).(type) {
    /* @dingo:match:arm pattern="Ok(user)" bind="user:User" */
    case ResultOk:
        user := __dingo_match_0.Value
        /* @dingo:scope:start vars="user" */
        {
            fmt.Printf("Found: %s", user.name)
            return user
        }
        /* @dingo:scope:end */

    /* @dingo:match:arm pattern="Err(NotFound)" */
    case ResultErr:
        if __dingo_match_0.Error == NotFound {
            /* @dingo:scope:start */
            {
                fmt.Println("User not found")
                return defaultUser()
            }
            /* @dingo:scope:end */
        }

    /* @dingo:match:arm pattern="Err(e)" bind="e:error" */
    case ResultErr:
        e := __dingo_match_0.Error
        /* @dingo:scope:start vars="e" */
        {
            fmt.Printf("Error: %v", e)
            return defaultUser()
        }
        /* @dingo:scope:end */
}
/* @dingo:match:end exhaustive="true" */
```

#### Stage 2 AST Plugin Processing

```go
type MatchContextPlugin struct {
    typeInfo   *types.Info
    markers    map[ast.Node]*MatchMarker
    scopes     *ScopeTracker
}

func (p *MatchContextPlugin) Transform(file *ast.File) {
    // 1. Extract markers from comments
    ast.Inspect(file, func(n ast.Node) bool {
        if comment := extractMarker(n); comment != nil {
            p.markers[n] = parseMatchMarker(comment)
        }
        return true
    })

    // 2. Build type context with go/types
    conf := types.Config{Importer: importer.Default()}
    pkg, _ := conf.Check("", fset, []*ast.File{file}, p.typeInfo)

    // 3. Validate exhaustiveness
    for node, marker := range p.markers {
        if marker.Type == "match:start" {
            validateExhaustiveness(node, marker, p.typeInfo)
        }
    }

    // 4. Transform AST with context
    astutil.Apply(file, p.preTransform, p.postTransform)
}
```

### Context Tracking Mechanism

**Recommended: AST Comments with Structured Markers**

Format: `/* @dingo:category:action key="value" key2="value2" */`

Benefits:
- Survives go/parser unchanged
- Easy to extract via AST walk
- No external files to manage
- Self-documenting in generated code

### Performance Estimates

```
Current Baseline:
  - Regex preprocessing: ~5ms per 1000 LOC
  - AST processing: ~10ms per 1000 LOC
  - Total: ~15ms per 1000 LOC

With Context-Aware Enhancements:
  - Regex + markers: ~7ms per 1000 LOC (+2ms)
  - AST + marker extraction: ~12ms per 1000 LOC (+2ms)
  - Context validation: ~3ms per 1000 LOC (new)
  - Total: ~22ms per 1000 LOC

Performance Impact: 47% increase (well under 3x threshold)
```

### Migration Plan

**Week 1: Foundation**
- Implement marker emission in preprocessors
- Create marker extraction utilities
- Basic context data structures

**Week 2: Pattern Matching MVP**
- Simple match expression support
- Variable binding in match arms
- Basic exhaustiveness checking

**Week 3: Type Integration**
- Full go/types integration
- Generic type parameter inference
- Nested pattern support

**Week 4: Polish & Optimization**
- Performance tuning
- Error message improvements
- Comprehensive test suite

### Risk Mitigation

**Risk 1: Marker Syntax Conflicts**
- Mitigation: Use unique prefix (@dingo:) unlikely to conflict
- Fallback: Switch to different comment syntax if needed

**Risk 2: go/types Limitations**
- Mitigation: Build incremental type info as needed
- Fallback: Manual type tracking for Dingo-specific constructs

**Risk 3: Performance Degradation**
- Mitigation: Profile and optimize hot paths
- Fallback: Optional context features for large files

## Key Implementation Insights

### 1. Marker Design Principles
- Keep markers minimal - just enough info for Stage 2
- Use structured format for easy parsing
- Include source position for error reporting

### 2. Leverage go/types Fully
- Let go/types handle all type inference
- Only track Dingo-specific metadata
- Use types.Info for variable resolution

### 3. Progressive Enhancement
- Start with basic pattern matching
- Add advanced features incrementally
- Maintain backward compatibility

## Success Metrics

✅ **Functional Success:**
- Pattern matching with exhaustiveness checking works
- Variable bindings correctly scoped
- Type inference accurate > 95%

✅ **Performance Success:**
- Transpilation < 50ms per 1000 LOC
- Memory usage < 2x baseline
- Incremental compilation possible

✅ **Maintainability Success:**
- Clear separation between stages
- Easy to add new context-aware features
- Comprehensive test coverage > 90%

## Conclusion

Strategy F (Hybrid Markers + AST Metadata) provides the optimal path forward for implementing context-aware preprocessing within the current architecture. It maintains the simplicity of regex-based preprocessing while enabling sophisticated semantic analysis through AST processing and go/types integration.

The implementation is achievable in 4 weeks with low risk and provides a solid foundation for all planned context-aware features including pattern matching, closures, and advanced type inference.