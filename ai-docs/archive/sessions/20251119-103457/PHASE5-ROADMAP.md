# Phase 5 Tooling Readmap - Multi-Model Consensus Analysis

**Date**: 2025-11-19
**Session**: 20251119-103457
**Models Consulted**: MiniMax M2, Grok Code Fast, GPT-5.1 Codex, Gemini 2.5 Flash

---

## Executive Summary

All four models agree that **Dingo's language core is remarkably mature** (Phase 4.1 complete with pattern matching, Result/Option types, error propagation), but the **tooling ecosystem is critically underdeveloped** and represents the primary blocker to v1.0 adoption.

### Unanimous Consensus

**#1 Priority**: **Language Server (dingo-lsp)** - All models identified LSP as the single most critical item
- **Effort**: 4-15 person-weeks (consensus: ~8-10 weeks)
- **Approach**: gopls proxy architecture (templ precedent)
- **Rationale**: "Without LSP, Dingo code is just text" - developers expect TypeScript-grade IDE support

**Critical Dependencies**: Source maps are the "lynchpin" (Gemini's term) - must be validated and robust before LSP work begins.

**Success Metric**: "Time to productive: <10 minutes" (Grok) - Install → autocomplete/goto-def/debugging working immediately.

---

## Phase 5 Roadmap by Priority

### TIER 0: Pre-Phase 5 Blockers (Start Immediately)

#### 1. **Fix 2 Failing Golden Tests**
- **Status**: BLOCKING v1.0 quality gate
- **Current**: 265/267 passing (98.5%)
- **Failing Tests**: `error_prop_02_multiple`, `option_02_literals`
- **Effort**: 1-2 person-weeks
- **Complexity**: Low
- **Dependencies**: None
- **Rationale**: (MiniMax M2) Quality gate must be 100% before v1.0
- **Action**: Fix immediately, unblocks all downstream work

#### 2. **Source Map Validation & Schema**
- **Effort**: 1-2 person-weeks
- **Complexity**: Medium
- **Dependencies**: Existing source map generation
- **Risks**: Inaccurate mapping breaks LSP + debugging
- **Approach**:
  - Build round-trip tests (.dingo position → .go position → .dingo position)
  - Document schema specification
  - Add validation suite to CI
- **Rationale**: (GPT-5.1 Codex) "Sourcemap fidelity underpins LSP, debugging, editor integrations"
- **Why First**: LSP cannot start without validated source maps

---

### TIER 1: Critical Path (Blocks GA Adoption)

**Timeline**: Months 1-3 (12 weeks)
**Parallel Execution**: Items 3-5 can run concurrently after items 1-2 complete

#### 3. **Language Server (dingo-lsp) - Phase 1**
- **Priority**: CRITICAL (unanimous #1)
- **Effort**: 8-10 person-weeks
- **Complexity**: High
- **Dependencies**: Source map validation complete
- **Risks**:
  - Latency inflation from synchronous translation
  - gopls subprocess lifecycle management
  - Error fan-out when transpiler fails mid-request
- **Approach** (Consensus):
  - Adopt templ's gopls proxy architecture
  - Stage 1: Minimal viable proxy (initialize, openTextDocument, diagnostics)
  - Stage 2: Core features (autocomplete, goto-definition, hover)
  - Stage 3: Advanced features (refactoring, quick fixes)
- **Success Metrics**:
  - Autocomplete working in .dingo files
  - Goto-definition across .dingo/.go boundaries
  - Real-time diagnostics
  - <100ms latency for position translation
- **Rationale**: (Grok) "Every week delayed is adoption lost"

#### 4. **Deterministic Build System**
- **Priority**: CRITICAL
- **Effort**: 3-4 person-weeks
- **Complexity**: Medium
- **Dependencies**: None (can start immediately)
- **Current State**: `dingo build` works for single files, lacks workspace support
- **Gaps**:
  - No incremental compilation
  - No build caching (.dingocache)
  - Non-deterministic output (timestamp issues)
  - Missing workspace/multi-module support
- **Approach**:
  - Hash-based build cache
  - Project manifest (dingo.toml schema enforcement)
  - Incremental build graph
  - Integration with `go build` workflows
- **Rationale**: (GPT-5.1 Codex) "Nondeterministic builds blocking GA adoption"

#### 5. **CI/CD Pipeline & Test Automation**
- **Priority**: CRITICAL
- **Effort**: 2-3 person-weeks
- **Complexity**: Medium
- **Dependencies**: Deterministic builds
- **Current State**: Manual test execution only
- **Gaps**:
  - No GitHub Actions workflow
  - No automated golden test verification
  - No coverage reporting
  - No performance benchmarking
- **Approach**:
  - Add `go test ./tests` workflow to GitHub Actions
  - Automated golden file diffing
  - Regression prevention
  - Platform-specific build verification
- **Rationale**: (GPT-5.1 Codex) "Prevents regressions as contributors grow"

#### 6. **VS Code Extension (MVP)**
- **Priority**: CRITICAL
- **Effort**: 3-4 person-weeks
- **Complexity**: Medium
- **Dependencies**: dingo-lsp Phase 1 complete
- **Current State**: Informal syntax highlighting work (session 20251116-194954)
- **Approach**:
  - Package LSP client + syntax highlighting
  - Tree-sitter grammar integration
  - Marketplace publishing pipeline
  - CI-automated releases
- **Rationale**: (Grok) "VS Code accounts for majority of Go devs" + (All) "First-mile experience critical"

---

### TIER 2: High Priority (Enables Early Adopter Comfort)

**Timeline**: Months 3-5 (8 weeks)
**Parallel Execution**: All items can run concurrently

#### 7. **Debugging Support (Delve Integration)**
- **Priority**: HIGH
- **Effort**: 6-8 person-weeks
- **Complexity**: High
- **Dependencies**: Source maps validated, LSP stable
- **Current State**: 0% - developers must debug transpiled Go
- **Risks**:
  - Delve has no native source map support
  - 100% accurate position translation required
  - DAP (Debug Adapter Protocol) complexity
- **Approach**:
  - Build DAP middleware (Delve ↔ Dingo translation layer)
  - Integrate via Delve API (not CLI)
  - Breakpoint mapping (.dingo line → .go line)
  - Stack trace translation
  - Variable inspection in Dingo syntax
- **Rationale**: (MiniMax M2) "Without debugging, developers won't adopt v1.0"

#### 8. **Package Management Strategy**
- **Priority**: HIGH
- **Effort**: 4-6 person-weeks
- **Complexity**: Medium
- **Dependencies**: None (can start immediately)
- **Current State**: 0% - no defined strategy
- **Decision Required**: Transpile-on-publish vs .dingo-as-dependency
- **Approach Options**:
  - **Option A** (MiniMax M2 recommendation):
    - `dingo publish` → transpiles to .go package
    - Consumers use `go get` → downloads .go only
    - Simple, Go-native, no tooling changes
  - **Option B** (Gemini recommendation):
    - `go get` fetches .dingo files as part of module
    - `dingo build` transpiles locally
    - Allows Dingo source inspection, more complex
- **Rationale**: (MiniMax M2) "Undefined package strategy blocks ecosystem growth"

#### 9. **Watch Mode (`dingo dev`)**
- **Priority**: HIGH
- **Effort**: 2-3 person-weeks
- **Complexity**: Medium
- **Dependencies**: Deterministic build system
- **Approach**:
  - File watching via fsnotify
  - Incremental compilation on save
  - Fast feedback loop (<500ms rebuild)
  - Integration with LSP for real-time updates
- **Rationale**: (Grok) "Keeps editing loop tight, competes with Go tooling"

#### 10. **Code Formatter (`dingo fmt`)**
- **Priority**: HIGH
- **Effort**: 3-4 person-weeks
- **Complexity**: Medium
- **Dependencies**: None
- **Approach**:
  - Round-trip through transpiler
  - Canonical Go output formatting
  - Style guide (align with `go fmt`)
  - Editor integration via LSP
- **Rationale**: (All) "Required for IDE integration and professional use"

#### 11. **Linter (`dingo lint`)**
- **Priority**: HIGH
- **Effort**: 4-5 person-weeks
- **Complexity**: Medium
- **Dependencies**: None
- **Approach**:
  - AST-based rule engine (golangci-lint patterns)
  - Dingo-specific rules (exhaustive pattern matching, unused Result values)
  - Editor integration via LSP diagnostics
- **Rationale**: (MiniMax M2) "v1.0 quality standard"

#### 12. **Documentation Site & Pipeline**
- **Priority**: HIGH
- **Effort**: 2-4 person-weeks
- **Complexity**: Low
- **Dependencies**: None
- **Current State**: Extensive internal docs (ai-docs/*), no public site
- **Approach**:
  - Convert to mdbook or docusaurus
  - Automated publishing via CI
  - Versioning (vNext vs current)
  - API reference for CLI flags/config
  - Tutorial pipeline
- **Rationale**: (GPT-5.1 Codex) "Communicates value, reduces onboarding time"

---

### TIER 3: Medium Priority (Polish & Scale)

**Timeline**: Months 5-7 (8 weeks)

#### 13. **Go → Dingo Migration Tool**
- **Priority**: MEDIUM
- **Effort**: 3-4 person-weeks
- **Complexity**: Medium
- **Approach**:
  - `dingo convert --to-dingo <file.go>`
  - Convert `(T, error)` → `Result[T,E]`
  - Add type annotations (`:` syntax)
  - Add `?` operators for error propagation
  - Conservative conversion (user reviews changes)
- **Rationale**: (MiniMax M2) "Lower adoption barrier by automating migration"

#### 14. **Compiler Version Manager**
- **Priority**: MEDIUM
- **Effort**: 2-3 person-weeks
- **Complexity**: Medium
- **Approach**:
  - `dingo self-update` command
  - Homebrew tap for macOS
  - GitHub releases for Linux/Windows
  - Version pinning support
- **Rationale**: (GPT-5.1 Codex) "Simplifies upgrades, ensures consistent versions"

#### 15. **Neovim/Vim Plugin**
- **Priority**: MEDIUM
- **Effort**: 3-4 person-weeks
- **Complexity**: Medium
- **Dependencies**: dingo-lsp stable
- **Approach**:
  - Tree-sitter grammar integration
  - nvim-lspconfig setup
  - Vim LSP client configuration
  - Syntax files
- **Rationale**: (MiniMax M2) "Broader developer reach"

#### 16. **JetBrains GoLand Plugin**
- **Priority**: MEDIUM
- **Effort**: 4-5 person-weeks
- **Complexity**: Medium
- **Dependencies**: dingo-lsp stable
- **Approach**:
  - LSP integration (GoLand supports LSP)
  - Syntax highlighting
  - Marketplace publishing
- **Rationale**: (Gemini) "Significant portion of Go developers"

#### 17. **Property-Based & Fuzz Testing**
- **Priority**: MEDIUM
- **Effort**: 2-3 person-weeks
- **Complexity**: Medium
- **Approach**:
  - Fuzz preprocessor regex patterns
  - Property tests for AST transformations
  - Integration with Go's native fuzzing
- **Rationale**: (GPT-5.1 Codex) "Catch edge cases (regex regressions)"

#### 18. **Performance Benchmarking Suite**
- **Priority**: MEDIUM
- **Effort**: 2-3 person-weeks
- **Complexity**: Medium
- **Approach**:
  - Transpilation speed benchmarks
  - Memory usage profiling
  - Comparison vs manual Go (prove zero overhead)
  - CI integration for regression detection
- **Rationale**: (MiniMax M2) "v1.0 quality standard"

#### 19. **godoc Compatibility**
- **Priority**: MEDIUM
- **Effort**: 2-3 person-weeks
- **Complexity**: Low
- **Approach**:
  - Preserve godoc comments in transpilation
  - `dingo doc` command (godoc from .dingo sources)
- **Rationale**: (MiniMax M2) "Documentation integration"

---

### TIER 4: Low Priority (Post-v1.0)

**Timeline**: Post-v1.0 (Future)

#### 20. **Playground/REPL**
- **Priority**: LOW
- **Effort**: 5-6 person-weeks
- **Approach**:
  - Web-based playground (like Go playground)
  - REPL for interactive development
- **Rationale**: (MiniMax M2) "Critical for adoption (like TypeScript playground)"
- **Why Post-v1.0**: High effort, can wait for stable v1.0

#### 21. **Emacs Plugin**
- **Priority**: LOW
- **Effort**: 3-4 person-weeks
- **Approach**: Major mode + lsp-mode integration
- **Why Post-v1.0**: Smaller user base, can wait

#### 22. **Telemetry & Metrics Opt-in**
- **Priority**: LOW
- **Effort**: 3-4 person-weeks
- **Complexity**: High
- **Risks**: Privacy concerns
- **Approach**:
  - Anonymous event pipeline
  - User opt-in (default: off)
  - Collect usage metrics for Go proposal validation
- **Rationale**: (GPT-5.1 Codex) "Validates Dingo impact for Go proposals"
- **Why Post-v1.0**: Needs privacy review, not blocking v1.0

#### 23. **Advanced LSP Diagnostics**
- **Priority**: LOW
- **Effort**: 3-4 person-weeks
- **Complexity**: High
- **Dependencies**: LSP Phase 1 + 2 complete
- **Approach**:
  - Pattern match exhaustiveness hints
  - Quick fixes (auto-add missing variants)
  - Type inference suggestions
- **Rationale**: (GPT-5.1 Codex) "Raises developer confidence"
- **Why Post-v1.0**: Advanced feature, LSP MVP sufficient for v1.0

#### 24. **Vendoring Strategy**
- **Priority**: LOW
- **Effort**: 2-3 person-weeks
- **Approach**: `dingo vendor` command
- **Why Post-v1.0**: Lower priority than core package management

---

## Implementation Timeline

### Pre-Phase 5 (Week 0-2) - IMMEDIATE START
**Goal**: Unblock Phase 5 work

1. **Fix 2 failing golden tests** (1-2 weeks) ✅ BLOCKING
2. **Source map validation** (1-2 weeks) ✅ BLOCKING

**Parallel Execution**: Both can run concurrently

---

### Phase 5.1: Foundation (Week 3-14) - 12 WEEKS
**Goal**: Core tooling infrastructure

**Month 1-2 (Weeks 3-10):**
- **dingo-lsp Phase 1** (8-10 weeks) - START IMMEDIATELY after source maps validated
- **Deterministic build system** (3-4 weeks) - PARALLEL with LSP
- **CI/CD pipeline** (2-3 weeks) - PARALLEL, depends on builds

**Month 3 (Weeks 11-14):**
- **VS Code extension MVP** (3-4 weeks) - Depends on LSP Phase 1

**Parallelization Opportunities**:
- LSP + Build System + CI can all run in parallel
- Team of 3 developers: 1 on LSP, 1 on build system, 1 on CI

**Milestone**: Developers can install VS Code extension, get autocomplete/goto-def, build deterministically

---

### Phase 5.2: Developer Experience (Week 15-22) - 8 WEEKS
**Goal**: Production-ready developer workflows

**Parallel Track A** (Developer Tools):
- `dingo dev` watch mode (2-3 weeks)
- `dingo fmt` (3-4 weeks)
- `dingo lint` (4-5 weeks)

**Parallel Track B** (Ecosystem):
- Package management strategy (4-6 weeks)
- Documentation site (2-4 weeks)

**Parallel Track C** (Advanced Tooling):
- Debugging support (6-8 weeks) - HIGH COMPLEXITY, start early

**Parallelization Opportunities**:
- All tracks independent, can run simultaneously
- Team of 6 developers: 2 on debugging, 2 on dev tools, 2 on ecosystem

**Milestone**: Developers can format, lint, debug, install packages, read docs

---

### Phase 5.3: Scale & Polish (Week 23-30) - 8 WEEKS
**Goal**: Multi-editor support, migration tools, benchmarks

**Parallel Execution**:
- Migration tool (3-4 weeks)
- Version manager (2-3 weeks)
- Neovim plugin (3-4 weeks)
- GoLand plugin (4-5 weeks)
- Fuzz testing (2-3 weeks)
- Benchmarking (2-3 weeks)
- godoc compatibility (2-3 weeks)

**Team of 4-5 developers**: All items can run concurrently

**Milestone**: Multi-editor support, automated migration, performance validated

---

### Post-v1.0: Future Enhancements
- Playground/REPL
- Emacs plugin
- Telemetry
- Advanced LSP features
- Vendoring

---

## Critical Path Summary

**Unblockable Sequential Dependencies**:
```
Fix Golden Tests (Week 0-2)
    ↓
Source Map Validation (Week 1-2) [can overlap with tests]
    ↓
dingo-lsp Phase 1 (Week 3-10)
    ↓
VS Code Extension (Week 11-14)
```

**Everything Else Can Run in Parallel**:
- Build system, CI, watch mode, formatter, linter, docs (no dependencies)
- Package management (no dependencies)
- Debugging (depends on source maps, can start Week 3)
- Multi-editor plugins (depend on LSP, can start Week 11)

**Total Timeline to v1.0**: ~30 weeks (7-8 months) with adequate team size

---

## Resource Recommendations

### Team Structure (Optimal)

**Phase 5.1** (Weeks 0-14):
- **2 developers**: LSP implementation
- **1 developer**: Build system + CI
- **Total**: 3 developers

**Phase 5.2** (Weeks 15-22):
- **2 developers**: Debugging support (high complexity)
- **2 developers**: Dev tools (fmt, lint, watch)
- **2 developers**: Ecosystem (package mgmt, docs)
- **Total**: 6 developers (or 3 with extended timeline)

**Phase 5.3** (Weeks 23-30):
- **4-5 developers**: Parallel polish work (plugins, migration, benchmarks)

**Minimum Viable Team**: 2-3 developers (extends timeline to 12-15 months)

---

## Key Recommendations (Multi-Model Consensus)

### Top 5 Immediate Actions

1. ✅ **Fix 2 failing golden tests NOW** - 1-2 weeks, blocks everything
2. ✅ **Validate source maps** - 1-2 weeks, LSP cannot start without this
3. ✅ **Start LSP immediately after** - 8-10 weeks, highest priority, longest pole
4. ✅ **Build system + CI in parallel** - 3-4 weeks, unblocks deterministic workflows
5. ✅ **Define package management strategy** - 4-6 weeks, enables ecosystem

### Strategic Insights

**From MiniMax M2**:
- "Dingo is more mature than expected" - language core is solid
- Start debugging support early (highest complexity, can't parallelize easily)
- Quality gate: 100% golden tests before v1.0

**From Grok Code Fast**:
- "Time to productive: <10 minutes" - success metric for tooling
- "Every week delayed is adoption lost" - urgency on LSP
- Focus on "first-mile experience" - install must be frictionless

**From GPT-5.1 Codex**:
- "Sourcemap fidelity underpins everything" - validate first
- Launch dedicated Phase 5 squad (don't block compiler work)
- Sequence work for visible wins quickly

**From Gemini 2.5 Flash**:
- "Source maps are the lynchpin" - critical dependency
- "Seamless Go ecosystem integration is non-negotiable"
- Iterative approach: core features first, gather feedback, iterate

### Precedents to Follow

- **TypeScript**: tsserver + VS Code → dingo-lsp + VS Code extension
- **templ**: gopls proxy architecture → adopt for dingo-lsp
- **Borgo**: Rust-to-Go transpiler → patterns for migration tool
- **Elm**: Great error messages → quality bar for Dingo

---

## Risk Assessment

### Top Risks

1. **Source map inaccuracy** → Breaks LSP + debugging
   - **Mitigation**: Validate with round-trip tests before LSP work

2. **LSP latency** → Poor developer experience
   - **Mitigation**: Asynchronous translation, caching, <100ms target

3. **Package management complexity** → Ecosystem fragmentation
   - **Mitigation**: Decision required (Option A vs B), prototype early

4. **Debugging integration** → Delve has no source map support
   - **Mitigation**: Custom DAP middleware, dedicated team

5. **Team capacity** → Underestimating effort
   - **Mitigation**: Parallel execution plan, prioritize ruthlessly

### Risk Mitigation Strategy

- **Freeze transpiler interfaces** before Phase 5 sprint (avoid churn)
- **Automate early** (CI/CD prevents regressions)
- **Reuse patterns** (templ, TypeScript - don't reinvent)
- **Version everything** (LSP, plugins, docs - alongside compiler)

---

## Success Metrics

### v1.0 Definition of Done

**Core Experience**:
- ✅ 267/267 golden tests passing (100%)
- ✅ Source maps validated with round-trip tests
- ✅ dingo-lsp working (autocomplete, goto-def, hover, diagnostics)
- ✅ VS Code extension published to marketplace
- ✅ Deterministic builds with caching
- ✅ CI/CD pipeline running on every PR

**Developer Workflows**:
- ✅ `dingo dev` watch mode working
- ✅ `dingo fmt` formatting code
- ✅ `dingo lint` catching issues
- ✅ Debugging with Delve + source maps
- ✅ Package management strategy defined and documented

**Documentation**:
- ✅ Public documentation site live
- ✅ Installation guide (< 10 minutes to productive)
- ✅ Migration guide (Go → Dingo)
- ✅ API reference complete

**Adoption Readiness**:
- ✅ "Time to productive" < 10 minutes from fresh install
- ✅ IDE experience on par with TypeScript
- ✅ All critical workflows automated
- ✅ Community can contribute (good docs, CI catches regressions)

---

## Model-Specific Insights

### MiniMax M2 (Score: 91/100) - Fast Root Cause
**Strength**: Pinpoint accuracy, simplest solution focus
**Key Insight**: Dingo is more mature than expected - focus on debugging + package mgmt
**Recommendation**: Start debugging NOW (highest complexity, unblockable)

### Grok Code Fast (Score: 83/100) - Debugging Methodology
**Strength**: Practical validation, step-by-step traces
**Key Insight**: LSP is critical blocker, "time to productive <10 min" metric
**Recommendation**: Start LSP immediately, draw from templ architecture

### GPT-5.1 Codex (Score: 80/100) - Architecture Vision
**Strength**: Long-term refactoring plans, architectural redesign
**Key Insight**: Sourcemap fidelity underpins everything, launch dedicated squad
**Recommendation**: Sequence for visible wins (sourcemaps → LSP → VS Code)

### Gemini 2.5 Flash (Score: 73/100) - Exhaustive Analysis
**Strength**: Comprehensive hypothesis exploration, thoroughness
**Key Insight**: Source maps = lynchpin, seamless Go integration non-negotiable
**Recommendation**: 3-phase approach over 4-7 months (LSP → DX tools → docs)

---

## Conclusion

Dingo's **language implementation is remarkably complete** (Phase 4.1 done), but **tooling is the critical bottleneck to v1.0 adoption**.

**The Path Forward**:
1. Fix tests + validate source maps (Week 0-2)
2. LSP + build system + CI in parallel (Week 3-14)
3. VS Code extension (Week 11-14)
4. DX tools + debugging + package mgmt (Week 15-22)
5. Multi-editor + polish (Week 23-30)

**With 3-6 developers and 7-8 months**, Dingo can achieve v1.0 with production-ready tooling that rivals TypeScript's developer experience.

**The consensus is clear**: Start LSP work immediately after source maps are validated. Everything else follows from there.

---

**Session Files**:
- Input Prompt: `ai-docs/sessions/20251119-103457/input/phase5-readiness-assessment.md`
- MiniMax M2 Analysis: `ai-docs/sessions/20251119-103457/output/minimax-m2-analysis.md`
- Grok Code Fast Analysis: `ai-docs/sessions/20251119-103457/output/grok-code-fast-analysis.md`
- GPT-5.1 Codex Analysis: `ai-docs/sessions/20251119-103457/output/gpt-5.1-codex-analysis.md`
- Gemini 2.5 Flash Analysis: `ai-docs/sessions/20251119-103457/output/gemini-2.5-flash-analysis.md`
- This Roadmap: `ai-docs/sessions/20251119-103457/PHASE5-ROADMAP.md`
