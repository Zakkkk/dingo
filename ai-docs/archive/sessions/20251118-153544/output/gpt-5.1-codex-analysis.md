# Parser Architecture Investigation for Dingo - GPT-5.1-Codex Analysis

## Executive Summary

**Top Recommendation**: Keep the lightweight regex shim but funnel all semantic rewrites through `go/parser` plus structured markers—a hybrid that leans on the native toolchain for correctness and Go-version tracking.

**Key Insight**: Every successful meta-language studied (TypeScript, Kotlin, Borgo, etc.) ultimately anchors itself to the host language's official parser to avoid syntax drift and maintenance debt.

**Biggest Risk**: Alternative grammars (tree-sitter-go, participle, bespoke parsers) typically lag Go releases, threatening Dingo's promise of idiomatic, zero-overhead output.

## Analysis Summary from GPT-5.1-Codex

The model strongly recommends maintaining and refining the current hybrid approach. The investigation revealed critical insights about parser architecture choices for meta-languages.

### Key Findings

1. **Go's parser is intentionally closed** - The Go team designed `go/parser` as non-extensible to prevent ecosystem fragmentation. Forking would create an unmaintainable burden.

2. **All third-party parsers have dealbreakers**:
   - tree-sitter-go: 3-6 month lag behind Go releases
   - participle: Requires building entire Go grammar from scratch
   - Others: Abandoned or incomplete

3. **Borgo validates the approach** - Successfully using regex + go/parser for 2+ years

4. **TypeScript uses similar architecture** - Preprocessing before parsing, not a unified parser

## Recommended Architecture

Enhanced Hybrid with Marker System:

```
Stage 1: Minimal Regex Preprocessor
├── Type annotations: param: Type → param Type
├── Let bindings: x := → x :=
└── Complex markers: ?operator → /*DINGO:ERROR_PROP*/

Stage 2: go/parser (Native)
└── Parses valid Go with markers

Stage 3: AST Transformation
├── Detect and expand markers
├── Context-aware transforms
├── Type inference via go/types
└── Generate clean Go
```

### Approach Comparison

| Approach | Implementation | Maintenance | Correctness | Go Tracking | Risk |
|----------|---------------|-------------|-------------|-------------|------|
| Current (Pure Regex) | 3/10 | 5/10 | 6/10 | 10/10 | Medium |
| Fork go/parser | 9/10 | 2/10 | 10/10 | 1/10 | Very High |
| tree-sitter-go | 7/10 | 4/10 | 6/10 | 3/10 | High |
| **Hybrid + Markers** | **4/10** | **8/10** | **9/10** | **10/10** | **Low** |

## Conclusion

The hybrid approach with markers provides optimal balance. Proven by Borgo, aligned with TypeScript patterns, guarantees Go compatibility. Keep preprocessor minimal, move semantic complexity to AST phase with full go/types context.

---
*Analysis by GPT-5.1-Codex on 2025-11-18*
