# Hybrid Parser Architecture Investigation for Dingo
## Context Update: Refined Hybrid Approach

**Previous consensus**: Keep regex preprocessor for 80% of simple transforms.

**New question**: Can we add an **optional syntax tree parser layer** on top of regex for context-aware features?

## Proposed Architecture

```
.dingo file
    ↓
┌─────────────────────────────────────────┐
│ Stage 1: Regex Preprocessor (80%)      │
├─────────────────────────────────────────┤
│ • Type annotations: param: Type → ...  │
│ • Error propagation: x? → ...          │
│ • Enums: enum Name {} → ...            │
│ • Simple transforms                     │
└─────────────────────────────────────────┘
    ↓
┌─────────────────────────────────────────┐
│ Stage 1.5: OPTIONAL Syntax Tree Parser │  ← NEW LAYER
├─────────────────────────────────────────┤
│ • Built on top of preprocessed output  │
│ • Only used when plugins need context  │
│ • Provides tree structure/scope info   │
│ • Optional dependency                   │
└─────────────────────────────────────────┘
    ↓
┌─────────────────────────────────────────┐
│ Stage 2: go/parser (AST Processing)    │
├─────────────────────────────────────────┤
│ • Parse preprocessed Go code           │
│ • Plugin pipeline transformations      │
│ • Generate .go + .sourcemap            │
└─────────────────────────────────────────┘
```

## The Key Question

**Is it feasible and beneficial to add an optional syntax tree parser layer between regex preprocessing and go/parser?**

### Use Cases for Optional Syntax Tree Parser

1. **Pattern Matching** (future feature)
   - Needs to understand match expression structure
   - Context-aware: scope, variable bindings, exhaustiveness

2. **Complex Lambda Transformations** (future)
   - Multi-line lambda bodies
   - Closure capture analysis
   - Nested lambda contexts

3. **Advanced Type Inference** (future)
   - Generic type parameter inference
   - Complex Result[T,E] chaining
   - Context-dependent type resolution

4. **Plugin System Extensions**
   - Plugins that need AST-like information
   - Custom linting/validation rules
   - Advanced code generation

### Questions to Investigate

#### 1. Technical Feasibility

**Can we build a lightweight syntax tree parser that:**
- Operates on already-preprocessed text (not raw Dingo)
- Handles only Dingo-specific constructs (not full Go)
- Provides context info (scopes, bindings, structure)
- Is optional (zero overhead when not needed)
- Remains simple to maintain

**Potential Technologies:**
- **Participle** (Go parser combinator library)
- **Tree-sitter** (incremental parsing, used by editors)
- **Custom recursive descent** (small, targeted parser)
- **PEG parser generators** (go-peg, pigeon)

#### 2. Architecture Design

**How would this layer integrate?**

Option A: **Plugin-invoked**
```go
// Plugin optionally requests syntax tree
if plugin.NeedsContext() {
    tree := syntaxParser.Parse(preprocessedCode)
    plugin.TransformWithContext(tree)
} else {
    plugin.Transform(preprocessedCode)
}
```

Option B: **Feature-gated**
```go
// Only parse tree for specific features
if hasPatternMatching(code) || hasComplexLambdas(code) {
    tree := syntaxParser.Parse(preprocessedCode)
    // Use tree for context-aware transforms
}
// Otherwise, skip tree parsing
```

Option C: **Always-available, lazily-evaluated**
```go
// Parse tree available but only computed if accessed
tree := lazy.New(func() *SyntaxTree {
    return syntaxParser.Parse(preprocessedCode)
})
// Only evaluated if plugin calls tree.Get()
```

#### 3. Cost-Benefit Analysis

**Benefits:**
- ✅ Enables complex future features (pattern matching, advanced lambdas)
- ✅ Plugin ecosystem can use context-aware transformations
- ✅ Better error messages (scope-aware diagnostics)
- ✅ Foundation for advanced type inference
- ✅ Still keeps 80% simple (regex for basic stuff)

**Costs:**
- ❌ Additional complexity layer
- ❌ More code to maintain
- ❌ Potential performance overhead (if not optional)
- ❌ Need to design Dingo-specific grammar
- ❌ Testing burden increases

**Key Metrics:**
- Implementation complexity (1-10 scale)
- Maintenance burden (hours/month estimated)
- Performance impact (milliseconds added)
- Feature enablement (which features does this unlock?)

#### 4. Precedents & Best Practices

**Do other transpilers use this pattern?**
- **TypeScript**: Does it parse before or after preprocessing?
- **Babel**: Plugin architecture - how do plugins get context?
- **Scala.js**: Multi-stage compilation - lessons learned?
- **Rust macros**: Procedural macros have access to token trees - relevant?

**Parser combinator experiences:**
- **Participle** in Go projects - how well does it scale?
- **Tree-sitter** for custom languages - integration complexity?
- **PEG parsers** - are they overkill for Dingo's limited syntax?

#### 5. Specific Implementation Questions

**If we proceed, what parser technology?**

Evaluate:

**A. Participle** (Go parser combinator)
- Pros: Pure Go, composable, readable grammars
- Cons: Learning curve, performance?
- Use case fit: Good for structured Dingo constructs

**B. Tree-sitter**
- Pros: Incremental parsing, editor integration potential
- Cons: C library dependency, complex integration
- Use case fit: Overkill unless we need editor support

**C. Custom recursive descent**
- Pros: Full control, minimal dependencies, simple
- Cons: More code to write, manual maintenance
- Use case fit: Good if grammar stays small

**D. PEG generator (pigeon, etc.)**
- Pros: Declarative grammar, well-tested
- Cons: Generated code, build step complexity
- Use case fit: Solid middle ground

**Which is best for Dingo's context?**

#### 6. Migration & Rollout Strategy

**If we add this layer:**

**Phase 1**: Foundation (1-2 months)
- Design Dingo-specific grammar (pattern matching, lambdas)
- Implement optional parser layer (feature-gated)
- Zero impact on existing features

**Phase 2**: First use case (1 month)
- Implement pattern matching using syntax tree
- Prove the concept works
- Measure overhead

**Phase 3**: Plugin API (1 month)
- Expose syntax tree to plugin system
- Document plugin context API
- Create examples

**Phase 4**: Optimization (ongoing)
- Lazy evaluation
- Caching
- Performance tuning

**Total timeline**: 3-4 months for full implementation

#### 7. Alternatives to Consider

**Alternative A**: Skip syntax tree, extend regex capabilities
- Use more sophisticated regex with backreferences
- Risk: Regex hell, unmaintainable

**Alternative B**: Jump straight to full parser, abandon regex
- Parse everything with proper parser
- Risk: Complexity explosion, over-engineering

**Alternative C**: Use go/parser earlier in pipeline
- Parse Dingo code with modified go/parser
- Risk: Go version tracking, parser fork maintenance

**Alternative D**: Delay decision until actually needed
- Implement pattern matching with regex first
- Only add syntax tree if proven necessary
- Risk: Technical debt, harder to refactor later

## Your Mission

Provide **comprehensive analysis** covering:

1. ✅ **Recommendation**: Should we add optional syntax tree parser layer?
2. 🔧 **If YES**:
   - Which parser technology? (Participle, Tree-sitter, custom, PEG)
   - How to integrate? (plugin-invoked, feature-gated, lazy)
   - Implementation roadmap and timeline
   - Risk mitigation strategies
3. ❌ **If NO**:
   - Why not? What are the blockers?
   - What alternatives are better?
   - When should we revisit this decision?
4. 📊 **Cost-Benefit**: Concrete numbers
   - Implementation effort (person-weeks)
   - Performance impact (benchmark estimates)
   - Features enabled (what couldn't we do without this?)

## Constraints

- Must remain **optional** (zero overhead when unused)
- Timeline: 3-4 months acceptable if high value
- Must integrate with existing plugin pipeline
- Should enable pattern matching and advanced lambdas
- Cannot break existing regex preprocessor features

## Deliverables

**Provide detailed analysis with**:
- Clear YES/NO recommendation
- Technology choice (if YES)
- Architecture design (with code examples)
- Migration/rollout plan
- Concrete cost-benefit numbers
- Precedents from other transpilers
- Risk assessment and mitigation

**Be thorough.** This is a critical architecture decision that affects Dingo's future capabilities.
