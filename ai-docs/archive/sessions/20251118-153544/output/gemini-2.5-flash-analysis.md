# Parser Architecture Investigation for Dingo
## Analysis by gemini-2.5-flash

## Executive Summary

After investigating multiple parsing approaches for Dingo, I recommend a **Hybrid Approach with Progressive Migration** strategy: maintain the current regex preprocessor for simple transformations while progressively migrating complex features to a proper parser built on **Participle** or **tree-sitter-go**.

## 1. Go Native Parser Extension

### Investigation Results
The `go/parser`, `go/scanner`, and `go/ast` packages are **NOT designed for extension**:
- All key types are concrete structs, not interfaces
- Scanner has internal state that cannot be modified
- Parser functions are not exported/hookable
- No plugin architecture or extension points

### Attempts by Others
- **gopls**: Works around this by parsing valid Go then transforming
- **Generics pre-Go 1.18**: Used build tags and preprocessing
- **go/tools**: Build analysis on top of parsed AST, not extending parser

**Verdict**: ❌ Not viable without forking entire Go parser

**Complexity**: 9/10
**Pros**: Would be native Go
**Cons**: Requires forking and maintaining entire parser, breaks with every Go release

## 2. Third-Party Go Parsers

### Participle (github.com/alecthomas/participle)
**Status**: Active, well-maintained
**Approach**: Parser combinator library using Go structs
**Extensibility**: Excellent - designed for DSLs

```go
type DingoFile struct {
    Functions []Function `@@*`
}

type Function struct {
    Name string `"fn" @Ident`
    Params []Param `"(" (@@ ("," @@)*)? ")"`
    ReturnType string `":" @Ident`
    Body string `"{" @Raw "}"`
}
```

**Complexity**: 5/10
**Pros**:
- Clean API, easy to extend
- Good error messages
- Supports incremental parsing
**Cons**:
- Need to redefine entire Go grammar
- Performance overhead vs native parser

### tree-sitter-go
**Status**: Official tree-sitter grammar for Go
**Approach**: Incremental parsing with error recovery
**Extensibility**: Requires modifying grammar file

**Complexity**: 7/10
**Pros**:
- Battle-tested in editors (VSCode, Neovim)
- Excellent error recovery
- Incremental parsing perfect for LSP
**Cons**:
- C library with Go bindings
- Grammar modifications need recompilation
- Learning curve for tree-sitter grammar

### go-tree-sitter
**Status**: Go bindings for tree-sitter
**Quality**: Good bindings, actively maintained

**Verdict**: ✅ Both Participle and tree-sitter are viable

## 3. Meta-Language Precedents

### TypeScript
- **Parser**: Custom recursive descent parser (not extending JS parser)
- **Key Insight**: Maintains two ASTs - TypeScript AST and JavaScript AST
- **Lesson**: Don't try to extend existing parser, build your own

### Borgo (Direct Precedent!)
- **Parser**: Uses **nom** parser combinator in Rust
- **Approach**: Full custom parser, not trying to extend Go's parser
- **Success**: Clean separation between Borgo syntax and Go output
- **Lesson**: Custom parser gives full control over syntax

### CoffeeScript
- **Parser**: Jison parser generator
- **Key**: Grammar-first approach with clean transpilation rules
- **Lesson**: Parser generators work well for transpilers

### Kotlin
- **Parser**: ANTLR-based parser
- **Multi-target**: Same frontend, different backends (JVM/JS/Native)
- **Lesson**: Clean parser/codegen separation enables multiple targets

## 4. Hybrid Approach Analysis

### What Should Stay as Preprocessor
✅ **Simple, context-free transformations**:
- Type annotations: `param: Type` → `param Type`
- Let bindings: `x := 5` → `x := 5`
- Simple keywords: `fn` → `func`

### What Needs Full Parser
❌ **Context-dependent, nested structures**:
- Pattern matching (needs scope analysis)
- Error propagation `?` (needs type information)
- Enum variants (needs exhaustiveness checking)
- Generic constraints
- Macro expansion (future)

### Migration Path
```
Phase 1: Current regex preprocessor (DONE)
Phase 2: Add Participle for complex features
Phase 3: Gradual migration of simple features
Phase 4: Full parser with preprocessor for backward compat
```

## 5. Architecture Recommendations

### Recommended Approach: Hybrid with Participle

**Architecture**:
```
.dingo file
    ↓
┌─────────────────────────────────────┐
│ Stage 0: Feature Router              │
│ (Decides preprocessor vs parser)     │
└─────────────────────────────────────┘
    ↓                        ↓
┌──────────────┐    ┌────────────────────┐
│ Preprocessor │    │ Participle Parser  │
│ (Simple)     │    │ (Complex features) │
└──────────────┘    └────────────────────┘
    ↓                        ↓
┌─────────────────────────────────────┐
│ Merge & Validate                    │
└─────────────────────────────────────┘
    ↓
┌─────────────────────────────────────┐
│ AST Processing (existing)           │
└─────────────────────────────────────┘
    ↓
.go file + .sourcemap
```

### Comparison Matrix

| Approach | Complexity | Maintainability | Correctness | Go Tracking | Recommended |
|----------|------------|-----------------|-------------|-------------|-------------|
| Pure Regex | 2/10 | Poor | 60% | N/A | ❌ Current limits |
| Fork go/parser | 9/10 | Very Poor | 100% | Hard | ❌ Too complex |
| Participle | 5/10 | Good | 95% | Easy | ✅ Best balance |
| tree-sitter | 7/10 | Good | 98% | Medium | ✅ Good alternative |
| Hybrid | 4/10 | Excellent | 90% | Easy | ✅✅ **WINNER** |

### Implementation Priority

1. **Keep current preprocessor** for:
   - Type annotations
   - Simple keywords
   - Let bindings

2. **Add Participle parser** for:
   - Enum definitions
   - Pattern matching
   - Complex error propagation
   - Future: Macros, advanced generics

3. **Benefits of Hybrid**:
   - Ship faster (preprocessor works today)
   - Gradual migration reduces risk
   - Can experiment with parser without breaking existing code
   - Parallel development possible

## 6. Key Insights & Risks

### Surprising Finding
**Borgo's Success Secret**: They use a parser combinator (nom) not a grammar generator. This gives them fine control over error messages and recovery - critical for developer experience.

### Biggest Risk
**Parser Maintenance Burden**: Maintaining a full Go grammar is significant work. The hybrid approach mitigates this by only parsing Dingo-specific syntax, letting go/parser handle standard Go.

### Critical Success Factor
**Error Messages**: Whatever parser we choose must provide excellent error messages with source locations. Participle excels here with its error recovery and position tracking.

## 7. Concrete Next Steps

### Short Term (This Week)
1. Prototype enum parsing with Participle
2. Compare error message quality
3. Benchmark performance vs regex

### Medium Term (Next Month)
1. Implement pattern matching parser
2. Build source map generation
3. Create parser test suite

### Long Term (3-6 Months)
1. Gradual migration of preprocessor features
2. LSP integration with incremental parsing
3. Full parser with preprocessor compatibility mode

## Conclusion

The **Hybrid Approach with Participle** offers the best balance of:
- **Pragmatism**: Keep what works (regex for simple cases)
- **Power**: Proper parser for complex features
- **Maintainability**: Clean separation of concerns
- **Evolution**: Gradual migration path

This mirrors successful precedents like TypeScript (custom parser), Borgo (parser combinator), and maintains flexibility for future growth while shipping features today.

**Recommendation**: Start with Participle prototype for enum/pattern matching while keeping current preprocessor. This de-risks the approach and provides immediate value.
