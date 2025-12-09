# Phase 5 Priority Task List

**Generated**: 2025-11-19
**Session**: 20251119-103457
**Source**: Multi-model consensus (MiniMax M2, Grok Code Fast, GPT-5.1 Codex, Gemini 2.5 Flash)

---

## IMMEDIATE (Week 0-2) - BLOCKING ALL PHASE 5 WORK

### 🔴 P0: Critical Blockers

- [ ] **Fix 2 failing golden tests** (1-2 weeks)
  - File: `tests/golden/error_prop_02_multiple.dingo` (failing)
  - File: `tests/golden/option_02_literals.dingo` (failing)
  - Current: 265/267 passing (98.5%)
  - Target: 267/267 passing (100%)
  - Blocking: v1.0 quality gate

- [ ] **Source Map Validation** (1-2 weeks)
  - Create round-trip position tests (.dingo → .go → .dingo)
  - Document source map schema specification
  - Add validation suite to CI
  - Verify accuracy to >99.9%
  - Blocking: LSP, debugging, all advanced tooling

---

## TIER 1: Critical Path (Week 3-14) - MUST HAVE FOR v1.0

### 🟠 P1: Foundation (Start Week 3)

**Can run in PARALLEL** (3 developers recommended)

- [ ] **dingo-lsp Phase 1: Minimal Viable Proxy** (8-10 weeks)
  - Dependencies: Source map validation ✅
  - Milestone 1 (Week 3-6): gopls proxy scaffolding
    - Initialize LSP server
    - openTextDocument/closeTextDocument
    - Position translation layer (.dingo ↔ .go)
    - gopls subprocess lifecycle management
  - Milestone 2 (Week 7-10): Core features
    - Diagnostics (real-time errors)
    - Goto definition (across .dingo/.go boundaries)
    - Hover (type information)
    - Autocomplete (basic)
  - Target latency: <100ms for position translation
  - Architecture: templ's gopls proxy pattern

- [ ] **Deterministic Build System** (3-4 weeks)
  - Can run PARALLEL with LSP
  - Milestone 1: Hash-based caching
  - Milestone 2: Incremental compilation
  - Milestone 3: dingo.toml schema enforcement
  - Milestone 4: Multi-module workspace support
  - Milestone 5: Integration with `go build`

- [ ] **CI/CD Pipeline & Test Automation** (2-3 weeks)
  - Depends: Deterministic builds
  - GitHub Actions workflow
  - Automated golden test verification
  - Artifact diffing
  - Coverage reporting
  - Platform-specific builds (Linux, macOS, Windows)

- [ ] **VS Code Extension MVP** (3-4 weeks)
  - Dependencies: dingo-lsp Phase 1 ✅
  - Milestone 1: LSP client integration
  - Milestone 2: Syntax highlighting (Tree-sitter)
  - Milestone 3: Marketplace publishing pipeline
  - Milestone 4: CI-automated releases
  - Target: Install → autocomplete/goto-def working

**End of Tier 1**: Developers can install VS Code, get IDE features, build deterministically

---

## TIER 2: High Priority (Week 15-22) - ENABLES ADOPTION

### 🟡 P2: Developer Experience

**Can run in PARALLEL** (6 developers recommended, or 3 with extended timeline)

**Track A: Developer Tools**

- [ ] **`dingo dev` - Watch Mode** (2-3 weeks)
  - File watching (fsnotify)
  - Incremental rebuild on save
  - Target: <500ms rebuild
  - Integration with LSP for real-time updates

- [ ] **`dingo fmt` - Code Formatter** (3-4 weeks)
  - Round-trip through transpiler
  - Canonical Go output formatting
  - Dingo-specific syntax (`:`, `?`, enums)
  - Editor integration via LSP

- [ ] **`dingo lint` - Linter** (4-5 weeks)
  - AST-based rule engine
  - Dingo-specific rules:
    - Exhaustive pattern matching
    - Unused Result/Option values
    - Error handling patterns
  - Editor integration via LSP diagnostics

**Track B: Ecosystem**

- [ ] **Package Management Strategy** (4-6 weeks)
  - **DECISION REQUIRED**: Choose approach
    - Option A: Transpile-on-publish (`dingo publish` → .go package)
    - Option B: .dingo-in-go-mod (fetch .dingo, transpile locally)
  - Prototype chosen approach
  - Document strategy
  - Implement tooling support

- [ ] **Documentation Site & Pipeline** (2-4 weeks)
  - Convert internal docs to mdbook/docusaurus
  - Automated CI publishing
  - Versioning (vNext vs current)
  - API reference (CLI flags, dingo.toml)
  - Tutorial pipeline
  - Installation guide (target: <10 min to productive)

**Track C: Advanced Tooling**

- [ ] **Debugging Support (Delve Integration)** (6-8 weeks)
  - Dependencies: Source maps validated ✅
  - **HIGH COMPLEXITY** - Start early (can start Week 3)
  - Milestone 1: DAP (Debug Adapter Protocol) middleware
  - Milestone 2: Delve API integration (not CLI)
  - Milestone 3: Breakpoint mapping (.dingo → .go)
  - Milestone 4: Stack trace translation
  - Milestone 5: Variable inspection (Dingo syntax)
  - Milestone 6: VS Code debugging extension

**End of Tier 2**: Developers can debug, format, lint, install packages, read comprehensive docs

---

## TIER 3: Medium Priority (Week 23-30) - POLISH & SCALE

### 🟢 P3: Multi-Editor & Migration

**Can run in PARALLEL** (4-5 developers recommended)

- [ ] **Go → Dingo Migration Tool** (3-4 weeks)
  - `dingo convert --to-dingo <file.go>`
  - Convert `(T, error)` → `Result[T,E]`
  - Add type annotations (`:` syntax)
  - Add `?` operators
  - Conservative conversion (user review)

- [ ] **Compiler Version Manager** (2-3 weeks)
  - `dingo self-update` command
  - Homebrew tap (macOS)
  - GitHub releases (Linux/Windows)
  - Version pinning

- [ ] **Neovim/Vim Plugin** (3-4 weeks)
  - Dependencies: dingo-lsp stable ✅
  - Tree-sitter grammar
  - nvim-lspconfig setup
  - Vim LSP client configuration

- [ ] **JetBrains GoLand Plugin** (4-5 weeks)
  - Dependencies: dingo-lsp stable ✅
  - LSP integration
  - Syntax highlighting
  - Marketplace publishing

- [ ] **Property-Based & Fuzz Testing** (2-3 weeks)
  - Fuzz preprocessor regex patterns
  - Property tests for AST transformations
  - Integration with Go native fuzzing

- [ ] **Performance Benchmarking Suite** (2-3 weeks)
  - Transpilation speed benchmarks
  - Memory profiling
  - Comparison vs manual Go
  - CI regression detection

- [ ] **godoc Compatibility** (2-3 weeks)
  - Preserve godoc comments in transpilation
  - `dingo doc` command

**End of Tier 3**: Multi-editor support, migration tools, performance validated

---

## TIER 4: Low Priority - POST-v1.0

### ⚪ P4: Future Enhancements

- [ ] **Playground/REPL** (5-6 weeks)
  - Web-based playground
  - Interactive REPL
  - Post-v1.0: High effort, can wait

- [ ] **Emacs Plugin** (3-4 weeks)
  - Major mode + lsp-mode
  - Post-v1.0: Smaller user base

- [ ] **Telemetry & Metrics** (3-4 weeks)
  - Anonymous event pipeline
  - User opt-in (default: off)
  - Collect usage for Go proposal validation
  - Post-v1.0: Needs privacy review

- [ ] **Advanced LSP Diagnostics** (3-4 weeks)
  - Pattern match exhaustiveness hints
  - Quick fixes (auto-add variants)
  - Type inference suggestions
  - Post-v1.0: LSP MVP sufficient for v1.0

- [ ] **Vendoring Strategy** (2-3 weeks)
  - `dingo vendor` command
  - Post-v1.0: Lower priority

---

## Execution Plan

### Week 0-2: IMMEDIATE START
**Team**: 2 developers (can overlap work)

```
Week 0 ────┬─── Fix Golden Test 1 (error_prop_02_multiple)
           └─── Source Map Validation (start)

Week 1 ────┬─── Fix Golden Test 2 (option_02_literals)
           └─── Source Map Validation (continue)

Week 2 ────┴─── Source Map Validation (complete + CI integration)
```

**Deliverable**: 267/267 tests passing, source maps validated

---

### Week 3-14: TIER 1 (Foundation)
**Team**: 3 developers minimum (parallel execution)

```
Week 3-14 ─┬─── Dev 1+2: dingo-lsp Phase 1 (8-10 weeks)
           │
           ├─── Dev 3: Deterministic Build System (Week 3-6, 3-4 weeks)
           │     └──> CI/CD Pipeline (Week 7-9, 2-3 weeks)
           │          └──> VS Code Extension (Week 10-14, 3-4 weeks)
```

**Deliverable**: LSP working, VS Code extension published, CI running

---

### Week 15-22: TIER 2 (Developer Experience)
**Team**: 6 developers (or 3 with extended timeline)

```
Week 15-22 ─┬─── Dev 1+2: Debugging Support (Week 15-22, 6-8 weeks)
            │              [HIGH COMPLEXITY - can start Week 3]
            │
            ├─── Dev 3: Watch Mode (Week 15-17, 2-3 weeks)
            │     └──> Code Formatter (Week 18-21, 3-4 weeks)
            │
            ├─── Dev 4: Linter (Week 15-19, 4-5 weeks)
            │
            ├─── Dev 5: Package Management (Week 15-20, 4-6 weeks)
            │
            └─── Dev 6: Documentation Site (Week 15-18, 2-4 weeks)
```

**Deliverable**: Debugging, fmt, lint, package mgmt, docs all working

---

### Week 23-30: TIER 3 (Polish & Scale)
**Team**: 4-5 developers (all parallel)

```
Week 23-30 ─┬─── Dev 1: Migration Tool (Week 23-26, 3-4 weeks)
            │
            ├─── Dev 2: Neovim Plugin (Week 23-26, 3-4 weeks)
            │     └──> Fuzz Testing (Week 27-29, 2-3 weeks)
            │
            ├─── Dev 3: GoLand Plugin (Week 23-27, 4-5 weeks)
            │
            ├─── Dev 4: Version Manager (Week 23-25, 2-3 weeks)
            │     └──> godoc Compatibility (Week 26-28, 2-3 weeks)
            │
            └─── Dev 5: Benchmarking (Week 23-25, 2-3 weeks)
```

**Deliverable**: Multi-editor, migration, benchmarks complete

---

### Post-Week 30: TIER 4 (Post-v1.0)
**Team**: Variable (as needed)

- Playground/REPL
- Emacs plugin
- Telemetry
- Advanced LSP features
- Vendoring

---

## Resource Requirements

### Minimum Viable Team
**2-3 developers** → Timeline extends to **12-15 months**

### Optimal Team (7-8 month timeline)
- **Week 0-14**: 3 developers
- **Week 15-22**: 6 developers (or 3 with 16-week timeline)
- **Week 23-30**: 4-5 developers

### Skills Required
- **Go expertise**: All developers
- **LSP experience**: 1-2 developers (for dingo-lsp)
- **Frontend/TypeScript**: 1 developer (for VS Code extension)
- **Debugging/DAP**: 1-2 developers (for Delve integration)

---

## Success Metrics

### v1.0 Definition of Done

**Quality Gate**:
- ✅ 267/267 golden tests passing (100%)
- ✅ Source maps >99.9% accurate
- ✅ CI/CD passing on every PR

**Core Experience**:
- ✅ dingo-lsp working (autocomplete, goto-def, hover, diagnostics)
- ✅ VS Code extension on marketplace
- ✅ Deterministic builds with caching
- ✅ "Time to productive" < 10 minutes

**Developer Workflows**:
- ✅ `dingo dev` watch mode
- ✅ `dingo fmt` formatting
- ✅ `dingo lint` catching issues
- ✅ Debugging with Delve + source maps
- ✅ Package management strategy implemented

**Documentation**:
- ✅ Public docs site live
- ✅ Installation guide
- ✅ Migration guide (Go → Dingo)
- ✅ API reference

---

## Next Steps

### This Week (Immediate)

1. **Review this roadmap** with team
2. **Make package management decision** (Option A vs B)
3. **Allocate developers** to Tier 0 tasks:
   - Developer 1: Fix golden test `error_prop_02_multiple`
   - Developer 2: Fix golden test `option_02_literals`
   - Developer 1+2: Source map validation (overlap after tests fixed)

### Next Week (Week 1)

1. **Complete golden test fixes**
2. **Complete source map validation**
3. **Prepare for Tier 1**:
   - Finalize LSP architecture (review templ's approach)
   - Set up development environment
   - Create GitHub project board for Phase 5

### Week 3 (Start Tier 1)

1. **Kick off LSP development** (2 developers)
2. **Start build system** (1 developer)
3. **Weekly progress reviews**

---

## Key Recommendations (Consensus from 4 Models)

### Top 5 Actions

1. ✅ **Fix tests NOW** - Blocking everything
2. ✅ **Validate source maps** - LSP depends on this
3. ✅ **Start LSP immediately** - Longest pole, highest priority
4. ✅ **Parallelize aggressively** - 3x speedup with proper team
5. ✅ **Make package mgmt decision** - Enables ecosystem planning

### Strategic Priorities

- **"Time to productive: <10 minutes"** - Success metric (Grok)
- **"Every week delayed is adoption lost"** - Urgency (Grok)
- **"Sourcemaps are the lynchpin"** - Critical dependency (Gemini)
- **"Start debugging NOW"** - Highest complexity (MiniMax)
- **"Sequence for visible wins"** - LSP → VS Code → Adoption (GPT-5.1)

---

**Full Roadmap**: `ai-docs/sessions/20251119-103457/PHASE5-ROADMAP.md`
**External Model Analyses**: `ai-docs/sessions/20251119-103457/output/`
