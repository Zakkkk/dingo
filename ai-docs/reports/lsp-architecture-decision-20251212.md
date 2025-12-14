# LSP Architecture Decision Analysis

**Date:** 2025-12-12
**Status:** RECOMMENDATION: Improve Source Mapping (Don't Fork gopls)

## Executive Summary

After consulting multiple AI models (Gemini, GPT-4.1) and researching how TypeScript and gopls handle this problem, the **unanimous recommendation is to NOT fork gopls**. Instead, invest in improving the source mapping architecture.

## Sources Consulted

### 1. Web Research Findings

**TypeScript Approach:**
- TypeScript's LSP (tsserver) operates **directly on TypeScript source** - it doesn't need source maps for LSP operations!
- Source maps are only used for debugging compiled JavaScript, not for language server features
- Key insight: TypeScript owns its entire toolchain

**gopls Extensibility:**
- Official gopls documentation states: "the only way to extend gopls is to modify gopls"
- gopls has language-specific packages (mod, work, template, golang)
- No plugin architecture exists - you must fork to extend

### 2. Gemini-2.5-Flash Analysis

**Recommendation:** Hybrid Approach - Robust Source Mapping

Key strategies:
1. **Embed position info in AST nodes** during Dingo's AST transformations
2. **Generate line mappings during AST transformation** (before go/printer)
3. **Leverage dmap for post-processing** reconciliation

### 3. GPT-4.1 Analysis

**Strong recommendation:** Hybrid Approach

Key assessments:
- **Forking gopls is NOT feasible** for a small team (200k+ lines, requires gopls specialists)
- **Native LSP is only feasible** for large, well-resourced projects (10+ full-time engineers)
- **Hybrid can reach "good enough" in weeks** - not years
- Accept that perfect mapping is impossible; "good enough for most code" is pragmatic

| Approach       | Time to Working | Maintenance | Recommendation |
|---------------|-----------------|-------------|----------------|
| Better Mapping | Weeks           | Low/Medium  | ✅ Yes         |
| Fork gopls     | Years           | High        | 🚫 No          |
| Native LSP     | Years           | Very High   | 🚫 No          |

## Root Cause of Current Issues

The current TransformTracker has a **fundamental flaw**: it tries to compute Go line numbers from byte positions AFTER go/printer has reformatted the code. The byte position tracking assumes 1:1 mapping that doesn't exist.

**Real example from this session:**
- Go line 46: `Email stringss` (the error)
- Dingo line 47: `Email stringss` (correct target)
- Current algorithm returns: Dingo line 53 (way off!)

## Recommended Architecture Fix

### Phase 1: Fix the Fundamental Issue (Immediate)

Instead of computing Go positions after go/printer, **store Dingo line numbers directly on AST nodes** before transformation:

```go
// During AST transformation, preserve Dingo position on the Go AST node
type DingoPosition struct {
    DingoLine   int
    DingoColumn int
    Kind        string  // "error_prop", "match", etc.
}

// Attach to go/ast nodes via comment or custom field
```

### Phase 2: Generate Mappings at AST Level (Short-term)

1. During each transform, record: `{DingoLine, GoASTNodePos}`
2. After go/printer, use `go/token.FileSet` to convert GoASTNodePos to final Go line
3. This works because go/printer preserves AST node positions!

```go
// Pseudo-code
func (t *TransformTracker) RecordTransform(dingoLine int, goNode ast.Node) {
    t.transforms = append(t.transforms, Transform{
        DingoLine: dingoLine,
        GoNodePos: goNode.Pos(),  // ast.Pos survives go/printer!
    })
}

func (t *TransformTracker) Finalize(fset *token.FileSet) []LineMappingEntry {
    for _, tr := range t.transforms {
        goPos := fset.Position(tr.GoNodePos)  // Convert to line:col
        // Now we have accurate GoLine from the AST position
    }
}
```

### Phase 3: Accept Limitations (Long-term)

For unmapped lines (code that wasn't transformed):
- Use identity mapping as fallback (Go line N → Dingo line N)
- Accept small inaccuracies (±1-2 lines) for edge cases
- Document limitations clearly

## Why Not Fork gopls?

1. **Size:** 200k+ lines of Go code tightly coupled to go/parser, go/ast, go/types
2. **Upstream tracking:** Go releases changes frequently; merge conflicts are constant
3. **Expertise required:** Need deep go/types knowledge
4. **Precedent:** Even ReasonML, Elm, Svelte avoided forking their respective ecosystem tools
5. **Team size:** Requires multiple dedicated gopls specialists

## Why Not Native LSP?

1. **Scope:** Would need to reimplement type checking, symbol resolution, refactoring
2. **Timeline:** Multi-year effort (Elm, Zig teams report years of investment)
3. **Maintenance:** Lose automatic Go ecosystem improvements
4. **Resources:** Typically requires 10+ full-time engineers

## Action Items

1. **Immediate:** Fix `TransformTracker.Finalize()` to use AST node positions
2. **Short-term:** Refactor to store DingoLine on AST nodes during transformation
3. **Medium-term:** Build comprehensive test suite for position mapping
4. **Long-term:** Document known limitations and edge cases

## Conclusion

The current approach (gopls proxy + source mapping) is correct in principle. The issue is the **implementation of position tracking**, not the architecture. Forking gopls would be a multi-year detour that diverts resources from what makes Dingo unique.

**Focus on fixing TransformTracker, not replacing the architecture.**
