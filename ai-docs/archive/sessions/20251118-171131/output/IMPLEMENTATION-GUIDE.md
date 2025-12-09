# Context-Aware Preprocessing: Implementation Guide

## Executive Summary

**Strategy F - Hybrid Markers + AST Metadata** received **UNANIMOUS** recommendation from all 5 external models consulted (GPT-5.1-Codex, Gemini-2.5-Flash, Grok-Code-Fast-1, Qwen3-VL-235B, Polaris-Alpha).

**Why this approach?** Provides optimal balance of simplicity, power, and maintainability while preserving the proven two-stage architecture. Minimal preprocessor changes, leverages go/types for semantic analysis, clear separation of concerns.

**Timeline**: 5-6 weeks to full implementation
**Complexity**: Medium (7/10) - builds on existing infrastructure
**Performance**: 25-37ms per 1000 LOC (within <50ms target)

## Architecture Overview

### Data Flow

```
.dingo file
    ↓
┌─────────────────────────────────────────────────────────┐
│ Stage 1: Enhanced Regex Preprocessor                    │
├─────────────────────────────────────────────────────────┤
│ • Transform Dingo syntax → valid Go                     │
│ • Emit contextual markers as Go comments               │
│ • Lightweight, no semantic understanding               │
│ • Format: /* DINGO:TYPE key=value */                   │
└─────────────────────────────────────────────────────────┘
    ↓ (Valid Go with markers)
┌─────────────────────────────────────────────────────────┐
│ Stage 2: Context-Aware AST Processing                   │
├─────────────────────────────────────────────────────────┤
│ • Parse with go/parser (preserves comments)            │
│ • Extract markers → build DingoContext                 │
│ • Use go/types for semantic analysis                   │
│ • Transform AST with full context awareness            │
│ • Validate exhaustiveness, scoping, captures           │
└─────────────────────────────────────────────────────────┘
    ↓
.go file + .sourcemap + validation errors
```

### Marker System Design

#### Marker Format

Standard format across all markers:
```
/* DINGO:TYPE key1=value1 key2=value2 ... */
```

Examples:
```go
/* DINGO:MATCH:START expr=getUserById(id) type=Result[User,Error] exhaustive=true */
/* DINGO:MATCH:ARM pattern=Ok(user) binding=user type=User */
/* DINGO:BINDING var=user type=User scope=case_1 */
/* DINGO:SCOPE:START id=case_1 parent=match_1 */
/* DINGO:CLOSURE:START capture=x,y mutable=y */
```

#### Marker Types

| Marker | Purpose | Attributes |
|--------|---------|------------|
| `DINGO:MATCH:START` | Begin match expression | expr, type, exhaustive |
| `DINGO:MATCH:ARM` | Match arm/case | pattern, binding, type |
| `DINGO:MATCH:END` | End match expression | - |
| `DINGO:BINDING` | Variable binding | var, type, scope |
| `DINGO:SCOPE:START` | Scope boundary | id, parent |
| `DINGO:SCOPE:END` | End scope | id |
| `DINGO:CLOSURE:START` | Lambda/closure | capture, mutable |
| `DINGO:CLOSURE:END` | End closure | - |
| `DINGO:TYPE` | Type inference hint | expr, inferred |

#### Example Markers in Context

```go
/* DINGO:MATCH:START expr=result type=Result[User,DbError] exhaustive=true */
switch __match_0 := result.(type) {
/* DINGO:MATCH:ARM pattern=Ok(user) binding=user type=User */
case ResultOk:
    /* DINGO:SCOPE:START id=arm_1 parent=match_1 */
    /* DINGO:BINDING var=user type=User scope=arm_1 */
    user := __match_0.Value
    processUser(user)
    /* DINGO:SCOPE:END id=arm_1 */
/* DINGO:MATCH:ARM pattern=Err(e) binding=e type=DbError */
case ResultErr:
    /* DINGO:SCOPE:START id=arm_2 parent=match_1 */
    /* DINGO:BINDING var=e type=DbError scope=arm_2 */
    e := __match_0.Error
    handleError(e)
    /* DINGO:SCOPE:END id=arm_2 */
}
/* DINGO:MATCH:END */
```

### AST Plugin Enhancements

#### Context Structure

```go
// pkg/transpiler/context.go
type TransformContext struct {
    // Marker data extracted from comments
    Markers     map[ast.Node][]Marker

    // Scope management
    Scopes      map[string]*Scope
    CurrentScope *Scope

    // Type information from go/types
    TypeInfo    *types.Info
    TypeChecker *types.Checker

    // Match expression tracking
    Matches     []*MatchContext

    // Variable bindings
    Bindings    map[string]*Binding

    // Metadata storage for plugins
    Metadata    map[string]interface{}
}

type MatchContext struct {
    Expression  ast.Expr
    Type        types.Type
    Arms        []MatchArm
    Exhaustive  bool
    StartPos    token.Pos
    EndPos      token.Pos
}

type MatchArm struct {
    Pattern     string
    Bindings    map[string]*Binding
    Scope       *Scope
    CaseClause  *ast.CaseClause
}

type Binding struct {
    Name        string
    Type        types.Type
    Scope       *Scope
    Mutable     bool
    Declaration ast.Node
}

type Scope struct {
    ID          string
    Parent      *Scope
    Bindings    map[string]*Binding
    Children    []*Scope
    StartPos    token.Pos
    EndPos      token.Pos
}
```

#### Plugin API

```go
// pkg/transpiler/plugins/plugin.go
type ContextAwarePlugin interface {
    Plugin

    // Called before transform with built context
    SetContext(ctx *TransformContext)

    // Priority for execution order
    Priority() int
}

// Updated plugin execution
func (t *Transpiler) runPlugins(file *ast.File) {
    // Phase 1: Build context
    ctx := NewTransformContext()
    ctx.extractMarkers(file)
    ctx.buildTypeInfo(file)
    ctx.buildScopes()

    // Phase 2: Run plugins with context
    for _, plugin := range t.sortedPlugins() {
        if contextAware, ok := plugin.(ContextAwarePlugin); ok {
            contextAware.SetContext(ctx)
        }
        ast.Walk(plugin, file)
    }
}
```

#### go/types Integration

```go
// pkg/transpiler/typeinfo.go
func BuildTypeInfo(file *ast.File, fset *token.FileSet) (*types.Info, error) {
    conf := types.Config{
        Importer: importer.Default(),
        Error: func(err error) {
            // Log but continue for best-effort type checking
            log.Printf("Type check warning: %v", err)
        },
    }

    info := &types.Info{
        Types:      make(map[ast.Expr]types.TypeAndValue),
        Defs:       make(map[*ast.Ident]types.Object),
        Uses:       make(map[*ast.Ident]types.Object),
        Scopes:     make(map[ast.Node]*types.Scope),
        Selections: make(map[*ast.SelectorExpr]*types.Selection),
        Implicits:  make(map[ast.Node]types.Object),
    }

    _, err := conf.Check("", fset, []*ast.File{file}, info)
    return info, err
}

// Using go/types for validation
func validateExhaustiveness(ctx *MatchContext, typeInfo *types.Info) error {
    // Get the type of the match expression
    exprType := typeInfo.TypeOf(ctx.Expression)

    // For Result[T,E] or Option[T], check all variants covered
    if named, ok := exprType.(*types.Named); ok {
        variants := extractVariants(named)
        covered := make(map[string]bool)

        for _, arm := range ctx.Arms {
            covered[arm.Pattern] = true
        }

        for _, variant := range variants {
            if !covered[variant] && !hasDefaultCase(ctx) {
                return fmt.Errorf("non-exhaustive match: missing case %s", variant)
            }
        }
    }

    return nil
}
```

## Pattern Matching Implementation

### Input (Dingo)

```dingo
match getUserById(id) {
    Ok(user) => processUser(user)
    Err(NotFound) => handleNotFound()
    Err(e) => handleError(e)
}
```

### Stage 1: Preprocessor Output

```go
/* DINGO:MATCH:START expr=getUserById(id) type=Result[User,Error] */
switch __discriminant_0 := getUserById(id).(type) {
    /* DINGO:MATCH:ARM pattern=Ok(user) binding=user */
    case ResultOk:
        /* DINGO:BINDING var=user type=User scope=arm_1 */
        user := __discriminant_0.Value
        processUser(user)
    /* DINGO:MATCH:ARM pattern=Err(NotFound) const=true */
    case ResultErr:
        if __discriminant_0.Error == NotFound {
            handleNotFound()
        }
    /* DINGO:MATCH:ARM pattern=Err(e) binding=e */
    case ResultErr:
        /* DINGO:BINDING var=e type=Error scope=arm_3 */
        e := __discriminant_0.Error
        handleError(e)
}
/* DINGO:MATCH:END */
```

### Stage 2: AST Plugin Processing

```go
// pkg/transpiler/plugins/pattern_match_plugin.go
type PatternMatchPlugin struct {
    BasePlugin
    ctx *TransformContext
}

func (p *PatternMatchPlugin) Transform(node ast.Node) ast.Node {
    switch n := node.(type) {
    case *ast.SwitchStmt:
        // Check for match markers
        markers := p.ctx.Markers[n]
        if !hasMatchMarkers(markers) {
            return node
        }

        // Build match context from markers
        matchCtx := p.buildMatchContext(n, markers)

        // Validate exhaustiveness using go/types
        if err := validateExhaustiveness(matchCtx, p.ctx.TypeInfo); err != nil {
            p.reportError(err)
        }

        // Transform to optimized Go switch
        return p.transformMatch(n, matchCtx)
    }
    return node
}

func (p *PatternMatchPlugin) transformMatch(stmt *ast.SwitchStmt, ctx *MatchContext) ast.Node {
    // Optimize based on pattern analysis
    if p.canOptimizeToTypeSwitch(ctx) {
        return p.generateTypeSwitch(stmt, ctx)
    }

    if p.canOptimizeToIfElse(ctx) {
        return p.generateIfElseChain(stmt, ctx)
    }

    // Default: keep as switch with validation
    return p.generateValidatedSwitch(stmt, ctx)
}
```

### Final Output

```go
// Clean, optimized Go code
switch result := getUserById(id).(type) {
case ResultOk:
    processUser(result.Value)
case ResultErr:
    switch err := result.Error; err {
    case NotFound:
        handleNotFound()
    default:
        handleError(err)
    }
}
```

## Implementation Phases

### Phase 1: Foundation (Week 1-2)

**Goal**: Basic marker system + plugin infrastructure

#### Tasks

- [ ] Define marker format specification (`pkg/generator/preprocessor/markers.go`)
  ```go
  type MarkerType string
  type Marker struct {
      Type       MarkerType
      Attributes map[string]string
      Position   token.Pos
  }
  ```

- [ ] Implement marker emission in preprocessor
  ```go
  func (p *PatternProcessor) emitMarker(markerType MarkerType, attrs map[string]string) string {
      parts := []string{string(markerType)}
      for k, v := range attrs {
          parts = append(parts, fmt.Sprintf("%s=%s", k, v))
      }
      return fmt.Sprintf("/* DINGO:%s */", strings.Join(parts, " "))
  }
  ```

- [ ] Create AST plugin marker parser
  ```go
  func extractMarkers(comments []*ast.CommentGroup) []Marker {
      var markers []Marker
      for _, group := range comments {
          for _, comment := range group.List {
              if marker := parseMarker(comment.Text); marker != nil {
                  markers = append(markers, *marker)
              }
          }
      }
      return markers
  }
  ```

- [ ] Add TransformContext structure
- [ ] Write basic integration tests

**Deliverables**:
- Marker spec document (`docs/marker-specification.md`)
- Preprocessor marker emitter (`pkg/generator/preprocessor/marker_emitter.go`)
- Plugin marker reader (`pkg/transpiler/plugins/marker_reader.go`)
- 10+ unit tests

### Phase 2: Pattern Matching (Week 3-4)

**Goal**: Full pattern matching support with exhaustiveness

#### Tasks

- [ ] Implement MATCH marker emission
- [ ] Create pattern matching AST plugin
- [ ] Add exhaustiveness checking with go/types
- [ ] Integrate type validation
- [ ] Add 20+ golden tests for pattern matching

**Deliverables**:
- Pattern matching preprocessor (`pkg/generator/preprocessor/pattern_processor.go`)
- Pattern matching plugin (`pkg/transpiler/plugins/pattern_match_plugin.go`)
- Exhaustiveness validator (`pkg/transpiler/validators/exhaustiveness.go`)
- Golden tests (`tests/golden/pattern_match_*.dingo`)
- Documentation (`docs/pattern-matching.md`)

### Phase 3: Advanced Features (Week 5-6)

**Goal**: Closures, type inference, nested contexts

#### Tasks

- [ ] CLOSURE markers for lambda capture
- [ ] TYPE markers for inference hints
- [ ] SCOPE markers for nested contexts
- [ ] Performance optimization
- [ ] Comprehensive error messages

**Deliverables**:
- Complete marker system
- All context-aware features working
- Performance benchmarks (`tests/benchmarks/`)
- Complete documentation

## Code Examples

### Preprocessor: Marker Emission

```go
// pkg/generator/preprocessor/pattern_processor.go
type PatternProcessor struct {
    BaseProcessor
    matchCounter int
    scopeCounter int
}

func (p *PatternProcessor) Process(input string) string {
    // Pattern: match expr { arms }
    matchRegex := regexp.MustCompile(`match\s+(.+?)\s*\{([^}]+)\}`)

    return matchRegex.ReplaceAllStringFunc(input, func(match string) string {
        p.matchCounter++
        matchID := fmt.Sprintf("match_%d", p.matchCounter)

        // Extract expression
        expr := extractExpression(match)

        // Start marker
        result := fmt.Sprintf("/* DINGO:MATCH:START expr=%s id=%s */\n", expr, matchID)

        // Generate switch statement
        result += fmt.Sprintf("switch __discriminant_%d := %s.(type) {\n", p.matchCounter, expr)

        // Process each arm
        arms := extractArms(match)
        for i, arm := range arms {
            armMarker := p.processArm(arm, i, matchID)
            result += armMarker
        }

        result += "}\n"
        result += fmt.Sprintf("/* DINGO:MATCH:END id=%s */", matchID)

        return result
    })
}

func (p *PatternProcessor) processArm(arm string, index int, matchID string) string {
    // Pattern: Constructor(binding) => body
    armRegex := regexp.MustCompile(`(\w+)\(([^)]*)\)\s*=>\s*(.+)`)
    matches := armRegex.FindStringSubmatch(arm)

    if len(matches) != 4 {
        return arm // Fallback for unmatched patterns
    }

    constructor := matches[1]
    binding := matches[2]
    body := matches[3]

    p.scopeCounter++
    scopeID := fmt.Sprintf("scope_%d", p.scopeCounter)

    result := fmt.Sprintf("/* DINGO:MATCH:ARM pattern=%s(%s) */\n", constructor, binding)
    result += fmt.Sprintf("case Result%s:\n", constructor)

    if binding != "" {
        result += fmt.Sprintf("    /* DINGO:BINDING var=%s scope=%s */\n", binding, scopeID)
        result += fmt.Sprintf("    %s := __discriminant_%d.Value\n", binding, p.matchCounter)
    }

    result += fmt.Sprintf("    %s\n", body)

    return result
}
```

### Plugin: Marker Reading

```go
// pkg/transpiler/plugins/context_builder.go
type ContextBuilder struct {
    BasePlugin
    markers  map[ast.Node][]Marker
    contexts map[ast.Node]*MatchContext
}

func (cb *ContextBuilder) Visit(node ast.Node) ast.Visitor {
    // Extract markers from preceding comments
    if cb.hasMarkers(node) {
        markers := cb.extractNodeMarkers(node)
        cb.markers[node] = markers

        // Build context for match expressions
        if switchStmt, ok := node.(*ast.SwitchStmt); ok {
            if isMatchExpression(markers) {
                ctx := cb.buildMatchContext(switchStmt, markers)
                cb.contexts[switchStmt] = ctx
            }
        }
    }

    return cb
}

func (cb *ContextBuilder) extractNodeMarkers(node ast.Node) []Marker {
    var markers []Marker

    // Get comments associated with this node
    comments := cb.getNodeComments(node)

    for _, comment := range comments {
        text := comment.Text
        if strings.HasPrefix(text, "/* DINGO:") && strings.HasSuffix(text, " */") {
            marker := cb.parseMarker(text)
            if marker != nil {
                markers = append(markers, *marker)
            }
        }
    }

    return markers
}

func (cb *ContextBuilder) parseMarker(text string) *Marker {
    // Remove comment delimiters
    text = strings.TrimPrefix(text, "/* DINGO:")
    text = strings.TrimSuffix(text, " */")

    parts := strings.Fields(text)
    if len(parts) == 0 {
        return nil
    }

    marker := &Marker{
        Type:       MarkerType(parts[0]),
        Attributes: make(map[string]string),
    }

    // Parse key=value pairs
    for i := 1; i < len(parts); i++ {
        if kv := strings.SplitN(parts[i], "=", 2); len(kv) == 2 {
            marker.Attributes[kv[0]] = kv[1]
        }
    }

    return marker
}
```

### Context Building

```go
// pkg/transpiler/context/builder.go
func BuildContext(file *ast.File, fset *token.FileSet) (*TransformContext, error) {
    ctx := &TransformContext{
        Markers:  make(map[ast.Node][]Marker),
        Scopes:   make(map[string]*Scope),
        Matches:  []*MatchContext{},
        Bindings: make(map[string]*Binding),
        Metadata: make(map[string]interface{}),
    }

    // Step 1: Extract all markers
    markerExtractor := &MarkerExtractor{ctx: ctx}
    ast.Walk(markerExtractor, file)

    // Step 2: Build type information
    typeInfo, err := BuildTypeInfo(file, fset)
    if err != nil {
        // Continue with best-effort type info
        log.Printf("Type checking incomplete: %v", err)
    }
    ctx.TypeInfo = typeInfo

    // Step 3: Build scope tree
    scopeBuilder := &ScopeBuilder{ctx: ctx}
    ast.Walk(scopeBuilder, file)

    // Step 4: Resolve bindings
    bindingResolver := &BindingResolver{ctx: ctx}
    ast.Walk(bindingResolver, file)

    // Step 5: Build match contexts
    matchBuilder := &MatchContextBuilder{ctx: ctx}
    ast.Walk(matchBuilder, file)

    return ctx, nil
}

// Example: Building match context
func (mb *MatchContextBuilder) buildMatchContext(stmt *ast.SwitchStmt) *MatchContext {
    markers := mb.ctx.Markers[stmt]

    // Find MATCH:START marker
    var startMarker *Marker
    for _, m := range markers {
        if m.Type == "DINGO:MATCH:START" {
            startMarker = &m
            break
        }
    }

    if startMarker == nil {
        return nil
    }

    ctx := &MatchContext{
        Expression: stmt.Tag,
        Arms:      []MatchArm{},
        StartPos:  stmt.Pos(),
        EndPos:    stmt.End(),
    }

    // Parse exhaustive attribute
    if exhaustive := startMarker.Attributes["exhaustive"]; exhaustive == "true" {
        ctx.Exhaustive = true
    }

    // Get expression type from go/types
    if mb.ctx.TypeInfo != nil {
        ctx.Type = mb.ctx.TypeInfo.TypeOf(stmt.Tag)
    }

    // Process each case clause
    for _, clause := range stmt.Body.List {
        if caseClause, ok := clause.(*ast.CaseClause); ok {
            arm := mb.buildMatchArm(caseClause)
            ctx.Arms = append(ctx.Arms, arm)
        }
    }

    return ctx
}
```

### Validation

```go
// pkg/transpiler/validators/exhaustiveness.go
type ExhaustivenessValidator struct {
    ctx *TransformContext
}

func (v *ExhaustivenessValidator) Validate(match *MatchContext) error {
    if !match.Exhaustive {
        return nil // Not required to be exhaustive
    }

    // Get type of match expression
    exprType := match.Type
    if exprType == nil {
        return fmt.Errorf("cannot determine type for exhaustiveness check")
    }

    // For Result[T,E] type
    if isResultType(exprType) {
        return v.validateResultExhaustiveness(match, exprType)
    }

    // For Option[T] type
    if isOptionType(exprType) {
        return v.validateOptionExhaustiveness(match, exprType)
    }

    // For enum types
    if isEnumType(exprType) {
        return v.validateEnumExhaustiveness(match, exprType)
    }

    return nil
}

func (v *ExhaustivenessValidator) validateResultExhaustiveness(match *MatchContext, typ types.Type) error {
    hasOk := false
    hasErr := false
    hasDefault := false

    for _, arm := range match.Arms {
        pattern := arm.Pattern
        if strings.HasPrefix(pattern, "Ok") {
            hasOk = true
        } else if strings.HasPrefix(pattern, "Err") {
            hasErr = true
        } else if pattern == "_" {
            hasDefault = true
        }
    }

    if !hasDefault && (!hasOk || !hasErr) {
        missing := []string{}
        if !hasOk {
            missing = append(missing, "Ok")
        }
        if !hasErr {
            missing = append(missing, "Err")
        }
        return fmt.Errorf("non-exhaustive match: missing cases %v", missing)
    }

    return nil
}
```

## Performance Analysis

### Estimates (from external model analyses)

| Component | Overhead | Details |
|-----------|----------|---------|
| Marker emission | 2-5ms per 1000 LOC | Regex pattern matching + string building |
| Marker parsing | 3-7ms per 1000 LOC | Comment extraction + parsing |
| Context building | 5-10ms per 1000 LOC | Scope tree + binding resolution |
| go/types | 5-8ms per 1000 LOC | Type checking (can be cached) |
| **Total added** | 15-30ms per 1000 LOC | Linear scaling maintained |

**Performance Targets**:
- Baseline: ~15ms per 1000 LOC (current)
- With context: ~25-37ms per 1000 LOC (projected)
- Target: <50ms per 1000 LOC ✅ **ACHIEVABLE**

### Optimization Strategies

1. **Lazy Evaluation**: Only build context for nodes that need it
2. **Caching**: Cache go/types results across files
3. **Parallel Processing**: Process independent files concurrently
4. **Marker Compression**: Use short marker names in production

## go/types Integration Details

### Type Inference

```go
func inferType(expr ast.Expr, info *types.Info, markers []Marker) types.Type {
    // First try go/types
    if info != nil {
        if tv, ok := info.Types[expr]; ok {
            return tv.Type
        }
    }

    // Fallback to marker hints
    for _, marker := range markers {
        if marker.Type == "DINGO:TYPE" {
            if typeStr := marker.Attributes["inferred"]; typeStr != "" {
                return parseTypeString(typeStr)
            }
        }
    }

    // Last resort: manual inference
    return inferFromAST(expr)
}
```

### Scope Tracking

```go
func trackScopes(file *ast.File, info *types.Info) map[ast.Node]*types.Scope {
    scopes := make(map[ast.Node]*types.Scope)

    // Use go/types scopes
    if info != nil && info.Scopes != nil {
        for node, scope := range info.Scopes {
            scopes[node] = scope
        }
    }

    // Augment with marker-based scopes
    ast.Inspect(file, func(n ast.Node) bool {
        if markers := extractMarkers(n); len(markers) > 0 {
            for _, m := range markers {
                if m.Type == "DINGO:SCOPE:START" {
                    // Create synthetic scope
                    scopes[n] = createScope(m.Attributes["id"])
                }
            }
        }
        return true
    })

    return scopes
}
```

### Generic Parameter Resolution

```go
func resolveGenericParams(typ types.Type, markers []Marker) map[string]types.Type {
    params := make(map[string]types.Type)

    // For Result[T,E] or Option[T]
    if named, ok := typ.(*types.Named); ok {
        if typeParams := named.TypeParams(); typeParams != nil {
            for i := 0; i < typeParams.Len(); i++ {
                param := typeParams.At(i)
                // Resolve from markers or type args
                params[param.Obj().Name()] = resolveTypeParam(param, markers)
            }
        }
    }

    return params
}
```

## Testing Strategy

### Unit Tests

```go
// tests/marker_test.go
func TestMarkerEmission(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected []Marker
    }{
        {
            name:  "match expression",
            input: "match result { Ok(x) => x, Err(e) => panic(e) }",
            expected: []Marker{
                {Type: "DINGO:MATCH:START", Attributes: map[string]string{"expr": "result"}},
                {Type: "DINGO:MATCH:ARM", Attributes: map[string]string{"pattern": "Ok(x)"}},
                {Type: "DINGO:MATCH:ARM", Attributes: map[string]string{"pattern": "Err(e)"}},
                {Type: "DINGO:MATCH:END"},
            },
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            processor := NewPatternProcessor()
            output := processor.Process(tt.input)
            markers := extractMarkersFromOutput(output)
            assert.Equal(t, tt.expected, markers)
        })
    }
}
```

### Golden Tests

```dingo
// tests/golden/pattern_match_01_result.dingo
fn processUser(id: u32) -> Result[User, Error] {
    match getUserById(id) {
        Ok(user) => {
            println("Processing user: {}", user.name)
            Ok(user)
        }
        Err(NotFound) => {
            println("User not found")
            Err(NotFound)
        }
        Err(e) => {
            println("Database error: {}", e)
            Err(e)
        }
    }
}
```

### Performance Tests

```go
// tests/benchmarks/context_bench.go
func BenchmarkContextAwareProcessing(b *testing.B) {
    sizes := []int{100, 1000, 10000}

    for _, size := range sizes {
        b.Run(fmt.Sprintf("%d_LOC", size), func(b *testing.B) {
            code := generateTestCode(size)
            b.ResetTimer()

            for i := 0; i < b.N; i++ {
                // Full pipeline
                preprocessed := preprocessor.Process(code)
                file, _ := parser.ParseFile(fset, "", preprocessed, parser.ParseComments)
                ctx, _ := BuildContext(file, fset)

                plugin := &PatternMatchPlugin{ctx: ctx}
                ast.Walk(plugin, file)

                generator.Generate(file)
            }
        })
    }
}
```

### Edge Cases

```go
// tests/edge_cases_test.go
func TestNestedMatches(t *testing.T) {
    input := `
    match outer {
        Some(inner) => match inner {
            Ok(value) => value,
            Err(e) => panic(e)
        },
        None => defaultValue()
    }
    `

    // Should handle nested match expressions correctly
    output := processWithContext(input)
    assert.Contains(t, output, "DINGO:MATCH:START")
    assert.Equal(t, 2, countMatches(output))
}
```

## Risks and Mitigations

| Risk | Impact | Likelihood | Mitigation |
|------|--------|------------|------------|
| Marker parsing overhead | Medium | Low | Optimize regex, lazy evaluation |
| go/types failures | High | Medium | Graceful degradation, marker fallbacks |
| Complex nested patterns | Medium | Medium | Incremental support, clear limitations |
| Debugging complexity | Low | High | Clear marker trail, debug mode |
| Breaking changes | High | Low | Versioned markers, backward compat |

### Detailed Mitigation Strategies

1. **Performance Risks**
   - Add benchmarks from day 1
   - Profile regularly
   - Have optimization buffer (target 50ms, achieve 25-37ms)

2. **Complexity Risks**
   - Keep strict boundaries between stages
   - Document marker format extensively
   - Provide debugging tools

3. **Compatibility Risks**
   - Version markers (`DINGO:v1:MATCH`)
   - Keep old marker parsers
   - Gradual migration paths

## Success Metrics

### Functionality
- ✅ Pattern matching with exhaustiveness checking works 100%
- ✅ Variable bindings with proper scoping
- ✅ Type inference accuracy >95%
- ✅ Closure capture analysis working
- ✅ Nested patterns supported

### Performance
- ✅ <50ms per 1000 LOC processing time
- ✅ Linear scaling maintained (O(n))
- ✅ Memory usage <20MB for 10k LOC files

### Quality
- ✅ 100% backward compatibility
- ✅ Zero regression in existing golden tests
- ✅ Clear error messages with source locations
- ✅ Generated Go code remains idiomatic

## Next Steps

### Immediate (This Week)

1. **Create marker format specification**
   ```bash
   touch docs/marker-specification.md
   # Document all marker types, attributes, examples
   ```

2. **Prototype marker emission**
   ```go
   // In existing ErrorPropProcessor
   func (p *ErrorPropProcessor) Process(input string) string {
       // Add marker before transformation
       marker := "/* DINGO:ERROR_PROP expr=" + expr + " */"
       // ... existing transformation
   }
   ```

3. **Prototype marker reading**
   ```go
   // Simple plugin to verify markers work
   type MarkerDebugPlugin struct {
       BasePlugin
   }

   func (p *MarkerDebugPlugin) Visit(node ast.Node) ast.Visitor {
       if markers := extractMarkers(node); len(markers) > 0 {
           log.Printf("Found markers at %v: %+v", node.Pos(), markers)
       }
       return p
   }
   ```

4. **Write integration test**
   ```go
   func TestMarkerRoundTrip(t *testing.T) {
       input := "x?"
       preprocessed := preprocessor.Process(input)
       assert.Contains(t, preprocessed, "DINGO:ERROR_PROP")

       file, _ := parser.ParseFile(fset, "", preprocessed, parser.ParseComments)
       markers := extractAllMarkers(file)
       assert.Len(t, markers, 1)
   }
   ```

### Next Week

1. Implement pattern matching markers
2. Create pattern matching plugin
3. Add exhaustiveness checking
4. Write 10 golden tests

### Following Weeks

- Week 3-4: Complete pattern matching
- Week 5: Advanced features (closures, nested)
- Week 6: Performance optimization and polish

## References

### External Model Analyses
- `gpt-5.1-codex-analysis.md` - Detailed hybrid approach design
- `gemini-flash-analysis.md` - Phased rollout strategy
- `grok-code-fast-1-analysis.md` - Performance benchmarks
- `qwen3-vl-235b-analysis.md` - Context structure design
- `final-consensus-analysis.md` - Unanimous recommendation summary

### Go Documentation
- [go/types package](https://pkg.go.dev/go/types) - Type checking
- [go/parser package](https://pkg.go.dev/go/parser) - AST parsing
- [go/ast package](https://pkg.go.dev/go/ast) - AST manipulation

### Pattern Matching Proposals
- [Go Proposal #57644](https://github.com/golang/go/issues/57644) - Pattern matching
- [Sum Types Discussion](https://github.com/golang/go/discussions/19412) - Algebraic types

## Conclusion

The Hybrid Markers + AST Metadata approach (Strategy F) provides the optimal path forward for implementing context-aware preprocessing in Dingo. With unanimous agreement from 5 external models and clear implementation guidance, this approach:

1. **Preserves** the successful two-stage architecture
2. **Minimizes** preprocessor complexity
3. **Leverages** go/types for semantic power
4. **Enables** advanced features like pattern matching
5. **Maintains** performance within targets

The implementation is immediately actionable with concrete code examples, clear phases, and measurable success criteria. Begin with marker emission prototype this week to validate the approach.