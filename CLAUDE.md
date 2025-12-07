# Claude AI Agent Memory & Instructions

This file contains instructions and context for Claude AI agents working on the Dingo project.

## 🚨🚨🚨 ABSOLUTE RULE: NO STRING MANIPULATION FOR PARSING 🚨🚨🚨

```
╔══════════════════════════════════════════════════════════════════════════════╗
║                        STOP AND READ THIS FIRST                              ║
╠══════════════════════════════════════════════════════════════════════════════╣
║                                                                              ║
║  We have FAILED to remove string manipulation THREE TIMES because agents    ║
║  kept reimplementing it. THIS STOPS NOW.                                    ║
║                                                                              ║
║  ❌ FORBIDDEN (will be rejected in code review):                            ║
║  • bytes.Index(), strings.Index(), strings.Contains()                       ║
║  • regexp.MustCompile(), regexp.Match(), regexp.Find*()                     ║
║  • Character scanning: for i := 0; i < len(src); i++ { if src[i] == '?' }  ║
║  • strings.Split(), strings.TrimSpace() for PARSING                         ║
║  • Any Find*() function that scans source bytes                             ║
║  • Any Transform*Source(src []byte) function pattern                        ║
║                                                                              ║
║  ✅ REQUIRED (the correct approach):                                        ║
║  • pkg/tokenizer/ tokenizes source → []Token                                ║
║  • pkg/parser/ parses tokens → AST nodes (MatchExpr, LambdaExpr, etc.)     ║
║  • pkg/codegen/ generates Go from AST nodes (NEVER from bytes)              ║
║                                                                              ║
║  THE PIPELINE:                                                               ║
║  []byte → tokenizer.Tokenize() → []Token → parser.Parse() → AST → codegen  ║
║           ↑                                                        ↑        ║
║     ONLY place that                                    ONLY accepts         ║
║     reads raw bytes                                    AST nodes            ║
║                                                                              ║
║  📖 FULL DETAILS: ai-docs/plans/REMOVE_STRING_MANIPULATION.md               ║
║                                                                              ║
╚══════════════════════════════════════════════════════════════════════════════╝
```

### Quick Verification Command

Before ANY codegen code is complete, run:
```bash
# This MUST return nothing - if it finds matches, the code is WRONG
grep -rn "bytes\.Index\|strings\.Index\|regexp\.\|func Find" pkg/codegen/ pkg/ast/*_codegen.go
```

### Why This Rule Exists

| Attempt | What Was Removed | What Replaced It |
|---------|------------------|------------------|
| 1st | `pkg/preprocessor/` | String manipulation in new files |
| 2nd | `pkg/transform/` | Nothing (was dead code) |
| 3rd | `TransformToGo()` | String manipulation in `pkg/ast/` |

**The REAL parser exists**: `pkg/parser/` has 5,329 lines of proper Pratt parsing.
**It was NEVER used**: Codegens reimplemented string manipulation instead.

---

## ⚠️ CRITICAL: Token Budget Enforcement (READ FIRST)

**EVERY action must pass this pre-check:**

### Token Budget Limits (HARD LIMITS)

| Operation | Limit | Violation Remedy |
|-----------|-------|------------------|
| File reads per message | 2 files OR 200 lines total | Delegate to agent |
| Bash output | 50 lines | Use `head -50` OR delegate |
| Grep results | 20 matches | Use `head_limit: 20` OR delegate |
| Agent response summary | 5 sentences max | Agent MUST compress |

**IF ANY LIMIT EXCEEDED → MUST delegate to agent instead**

### Pre-Check Decision Tree

```
┌─────────────────────────────────────────┐
│ Before EVERY action, ask:               │
└─────────────────────────────────────────┘
                   ↓
    [User wants multiple model perspectives?]
         ↓ YES
    [Create session folder]
         ↓
    [Write investigation prompt to file]
         ↓
    [Launch specialized agents in PARALLEL]
    (golang-architect for Go, etc.)
         ↓
    [Each agent invokes ONE external model via claudish]
         ↓
    [Results → files, Summaries → main chat (< 5 sentences)]
         ↓
    [Optional: Consolidation agent synthesizes]
                   ↓ NO
    [Will this exceed token budget?]
         /
       YES         NO
        │           │
        │           ↓
        │    [Is it multi-step task?]
        │         /
        │       YES         NO
        │        │           │
        │        │           ↓
        │        │    Execute directly
        │        │    (simple query/file op)
        │        │
        ↓        ↓
   Use Task tool (delegate to agent)
        │
        ↓
   Read ONLY summary (< 100 lines)
```

### Forbidden Patterns in Main Chat

**❌ NEVER DO THESE:**

1. **Reading Multiple Code Files**
   - ❌ Read 3+ files in one conversation turn
   - ✅ Delegate to agent → Read summary only

2. **Implementing Code**
   - ❌ Edit multiple files directly
   - ✅ Delegate to golang-developer → Read summary

3. **Running Tests**
   - ❌ Show full test output (>50 lines)
   - ✅ Delegate to golang-tester → Read summary

4. **Searching Codebase**
   - ❌ Multiple Grep calls, reading results
   - ✅ Delegate to Explore agent → Read summary

### Mandatory Pattern: Session Folders

For ANY multi-step task:

```bash
# Create session immediately
SESSION=$(date +%Y%m%d-%H%M%S)
mkdir -p ai-docs/sessions/$SESSION/{input,output}

# Write user request
echo "Request: ..." > ai-docs/sessions/$SESSION/input/request.md

# Delegate with file paths
Task → agent:
  Input: ai-docs/sessions/$SESSION/input/request.md
  Output: ai-docs/sessions/$SESSION/output/summary.txt

# Main chat reads ONLY summary
```

**Main chat NEVER reads detail files (unless presenting to user).**

## ⚠️ CRITICAL: AST-Based Code Generation Pipeline

**ALL Dingo syntax transformations use the AST-based code generators in `pkg/ast/`.**

### The New Architecture (December 2025)

The old approaches have been **replaced** by AST-based code generation:

| Old (DELETED) | New (CURRENT) |
|---------------|---------------|
| `pkg/preprocessor/*.go` | `pkg/ast/*_codegen.go` |
| `pkg/transform/`, `pkg/transformer/` | `pkg/ast/transform.go` |
| `TransformToGo()` function | `ast.TransformSource()` |
| Regex/token transforms | AST parse → generate pattern |
| No source mapping | Source mappings for LSP |

### AST Code Generation Architecture

```
pkg/
├── ast/                        # AST CODE GENERATION
│   ├── transform.go            # TransformSource() - unified pipeline
│   ├── sourcemap.go            # CodeGenResult, SourceMapping types
│   ├── helpers.go              # Shared helper functions
│   │
│   ├── enum_codegen.go         # enum → Go interface
│   ├── let_codegen.go          # let x = → x :=
│   ├── lambda_codegen.go       # |x| → func(x any) any
│   ├── match_codegen.go        # match → inline type switch
│   ├── error_prop_codegen.go   # expr? → inline error check
│   ├── ternary_codegen.go      # a ? b : c → inline if
│   ├── null_coalesce_codegen.go # a ?? b → inline nil check
│   ├── safe_nav_codegen.go     # x?.y → inline nil chain
│   └── tuple_codegen.go        # (a, b) → struct literal
│
├── parser/                     # DINGO PARSER (Pratt-based)
│   ├── file.go                 # File-level parsing
│   ├── pratt.go                # Pratt expression parser
│   └── simple.go               # Simple Dingo parser
│
├── goparser/                   # GO PARSER WRAPPER
│   └── parser/parser.go        # ParseFile() - Go AST from Dingo
│
└── transpiler/                 # CLI ENTRY POINT
    └── pure_pipeline.go        # PureASTTranspile()
```

### Pluggable Features

All language features are implemented as plugins with **priority ordering**:

| Plugin | Priority | Status | Description |
|--------|----------|--------|-------------|
| `enum` | 10 | ✅ | `enum Name {...}` → Go interface |
| `match` | 20 | ✅ | `match expr {...}` → type switch |
| `enum_constructors` | 30 | ✅ | `Variant()` → `NewVariant()` |
| `error_prop` | 40 | ✅ | `expr?` → error handling |
| `guard_let` | 50 | ✅ | `guard let x = expr else {...}` |
| `safe_nav_statements` | 55 | ✅ | Statement-level `?.` |
| `safe_nav` | 60 | ✅ | Expression `?.` (with type inference) |
| `null_coalesce` | 70 | ✅ | `??` operator |
| `lambdas` | 80 | ✅ | `\|x\|` and `=>` syntax |
| `type_annotations` | 100 | ✅ | `param: Type` → `param Type` |
| `generics` | 110 | ✅ | `<T>` → `[T]` |
| `let_binding` | 120 | ✅ | `let x =` → `x :=` |

### Feature Configuration (dingo.toml)

Features can be enabled/disabled in `dingo.toml`:

```toml
[feature_matrix]
# All features enabled by default
# Only specify features to disable:
safe_nav = false        # Disable ?. operator
null_coalesce = false   # Disable ?? operator
lambdas = true          # Keep lambdas enabled
```

When disabled syntax is used, the transpiler reports an error with line/column.

### Working with Features

**Adding/fixing a feature:**
1. **Edit the plugin** in `pkg/feature/builtin/plugins.go`
2. **Edit the transformer** in `pkg/goparser/parser/parser.go`
3. **Add tests** in `parser_test.go` and `feature_integration_test.go`
4. **Run tests**: `go test ./pkg/goparser/... ./pkg/feature/...`

**Adding a new feature:**
1. Create plugin implementing `feature.Plugin` interface
2. Register via `feature.Register()` in init()
3. Add config field to `FeatureMatrix` in `pkg/config/config.go`
4. Wire transform function in `pkg/goparser/parser/feature_integration.go`

## Project Structure Rules

### Root Directory (Minimal)
The root folder should **ONLY** contain:
- `README.md` - Main project documentation (user-facing)
- `CLAUDE.md` - This file: AI agent memory and instructions
- Standard project files: `go.mod`, `go.sum`, `.gitignore`, `LICENSE`, etc.
- Source code directories: `cmd/`, `internal/`, `pkg/`, etc.

**DO NOT create additional documentation files in the root!**

### AI Documentation (`ai-docs/`)
All AI-related research, context, and working documents go here:
- `claude-research.md` - Comprehensive implementation guide
- `gemini_research.md` - Technical blueprint and analysis
- Any future AI-generated research, design docs, or context files

**Purpose**: These files help AI agents understand the project context, architecture decisions, and current stage. They are NOT user-facing documentation.

### Other Documentation
- User-facing documentation goes in `docs/` (when created)
- API documentation, tutorials, examples go in appropriate subdirectories
- Keep root clean and minimal

## Project Context

### What is Dingo?
A meta-language for Go (like TypeScript for JavaScript) that:
- Transpiles `.dingo` files to idiomatic `.go` files
- Provides Result/Option types, pattern matching, and error propagation
- Maintains 100% Go ecosystem compatibility
- Offers full IDE support via gopls-wrapping language server

**Official Website**: https://dingolang.com (landing page domain)

### Critical Value Proposition: Dual Benefit (Personal + Collective)

**THE MOST IMPORTANT THING ABOUT DINGO:**

Dingo delivers **TWO revolutionary benefits simultaneously**:

**1. Immediate Personal Value (Why developers actually use it):**
- 67% less error handling boilerplate with `?` operator
- 78% code reduction with sum types/enums
- Zero nil pointer panics with Option types
- Same performance (transpiles to clean Go)
- Better code TODAY, zero waiting for proposals

**2. Collective Future Value (Automatic side effect):**
- Your usage generates real-world metrics
- Your bugs find edge cases theoretical debates miss
- Your production code validates ideas
- Go team gets evidence-based data for decisions

**This is EXACTLY what TypeScript did for JavaScript:**
- Developers adopted TypeScript selfishly (better codebases)
- Millions used features like async/await, optional chaining
- JavaScript saw proof it worked and adopted features
- Timeline: TypeScript feature → 1-2 years usage → JavaScript adoption

**Examples:**
- Async/await: TS 2015 → Millions used it → JS ES2017 (2 years)
- Optional chaining: TS 2019 → Widespread adoption → JS ES2020 (1 year)
- Nullish coalescing: TS 2019 → Standard in TS → JS ES2020 (1 year)

**Dingo enables the same for Go:**
- You use Dingo to make YOUR code better (selfish reason)
- 50,000 other developers do the same
- Go team sees 2 years of production validation
- Go proposals now have concrete evidence

**Perfect incentive alignment:**
- Developers: Better code today, zero waiting
- Go team: Real data for decisions, reduced risk
- Ecosystem: Faster evolution, battle-tested features

**When working on Dingo, remember:**
- Primary goal: Make developers' code better IMMEDIATELY
- Secondary effect: Generate data that could reshape Go's future
- Every feature should provide measurable value (track metrics!)
- We're not competing with Go—we're accelerating its evolution through real-world experimentation
- Emphasize: "Use Dingo selfishly. Help Go evolve as a bonus."

### Architecture (Two Components)

1. **Transpiler** (`dingo build`) - Two-Stage Approach
   - **Stage 1: Preprocessor** - Text-based transformation (Dingo syntax → valid Go)
     - TypeAnnotProcessor: `param: Type` → `param Type`
     - ErrorPropProcessor: `x?` → error handling code
     - EnumProcessor: `enum Name {}` → Go tagged unions
     - KeywordProcessor: Other Dingo keywords
   - **Stage 2: AST Processing** - Parse and transform
     - Uses native `go/parser` to parse preprocessed Go code
     - Plugin pipeline transforms AST (Result types, etc.)
     - Generates `.go` + `.sourcemap` files
   - Tools: Regex-based preprocessors, `go/parser`, `go/ast`, `go/printer`

2. **Language Server** (`dingo-lsp`)
   - Wraps gopls as proxy
   - Translates LSP requests using source maps
   - Provides IDE features (autocomplete, navigation, diagnostics)
   - Tools: `go.lsp.dev/protocol`, gopls subprocess

### Current Stage

**Phase 9: Ternary Operator** ✅ Complete (2025-11-20)

**Status: v1.0-BETA READY (Phase 9 shipped)**

Dingo has completed Phase 9 with full ternary operator support (`condition ? trueValue : falseValue`). Implementation features concrete type inference, IIFE pattern for zero overhead, and robust expression parsing. All tests passing (42/42 unit + 3/3 golden), 3/3 code reviewers approved.

**Latest Features (Phase 9):**
- Ternary operator with concrete type inference (string, int, bool - not interface{})
- IIFE pattern for zero runtime overhead (compiler inlines)
- Max 3-level nesting enforcement for readability
- Complete source mapping for IDE integration
- Raw string literal support and robust expression boundaries

**Previously Completed (Phase VI):**
- Two-stage transpilation (preprocessor + go/parser)
- Result<T,E> and Option<T> types with full helper methods (Map, AndThen)
- Error propagation (`?` operator) - 100% test coverage
- Lambda expressions (TypeScript & Rust syntax) - 100% test coverage
- Pattern matching with guards and tuple patterns - 92% test coverage
- Sum types/enums with exhaustiveness checking
- Null coalescing (`??`) - implementation complete, parser refinement needed
- Tuples with literals and destructuring (Phase 8)
- Multi-package workspace builds
- Comprehensive developer documentation

**Quality Metrics:**
- 3/4 external model approval for v1.0 (Grok 4 Fast, Gemini 3 Pro, GPT-5, Claude Opus 4)
- Average scores: 8.9/10 Quality, 8.9/10 Completeness, 8.1/10 Production Readiness
- **92.5% test passing rate (124/134 tests)** ⬆️ up from 51%
- **100% compilation rate** - all generated Go code compiles
- **5/6 P0 features at 90%+** - Error prop, Lambdas, Option, Result, Pattern matching

**Recent Session (2025-11-20):**
- Fixed 6 critical P0 bugs (tuple naming, null coalesce comments, helper methods, IIFE indentation, etc.)
- Regenerated 17 golden files with new helper methods
- Improved pass rate by 41.5 percentage points (51% → 92.5%)
- Commit: `9cf49e3` - feat(p0): Complete P0 feature implementation sprint

See `ai-docs/sessions/20251120-p0-final/` for detailed session report and `CHANGELOG.md` for complete project history.

### Key Research Findings

See `ai-docs/claude-research.md` and `ai-docs/gemini_research.md` for details:

- **Proven precedents**: Borgo (Go transpiler), templ (gopls proxy), TypeScript (architecture)
- **Critical technology**: Source maps for bidirectional position mapping
- **Actual Implementation** (as of Phase 2.16):
  - **Preprocessor**: Regex-based text transformations (Dingo → valid Go)
  - **Parser**: Native `go/parser` (standard library)
  - **AST**: `go/ast`, `golang.org/x/tools/go/ast/astutil`
  - **Plugins**: Interface-based AST transformation pipeline
  - **LSP**: `go.lsp.dev/protocol` (future)
- **Timeline**: 12-15 months to v1.0

### Design Principles

1. **Zero Runtime Overhead**: Generate clean Go code, no runtime library
2. **Full Compatibility**: Interoperate with all Go packages and tools
3. **IDE-First**: Maintain gopls feature parity
4. **Simplicity**: Only add features that solve real pain points
5. **Readable Output**: Generated Go should look hand-written

### Code Generation Standards

**CRITICAL: Variable Naming Convention (Enforced 2025-11-20)**

All code generators MUST follow these naming rules:

1. **No Underscores - Use camelCase**
   - ✅ Correct: `tmp`, `tmp1`, `err`, `err1`, `coalesce`
   - ❌ Wrong: `__tmp0`, `__err0`, `__coalesce0`

2. **No-Number-First Pattern**
   - ✅ Correct: First `tmp`, then `tmp1`, `tmp2`
   - ✅ Correct: First `err`, then `err1`, `err2`
   - ❌ Wrong: `tmp1`, `tmp2`, `tmp3` (all numbered)
   - ❌ Wrong: `tmp0`, `tmp1`, `tmp2` (zero-based)

3. **Counter Initialization**
   - ✅ Correct: `counter = 1` or `counter := 1`
   - ❌ Wrong: `counter = 0` or `counter := 0`

**Affected Components:**
- `pkg/preprocessor/error_prop.go` - Error propagation (`tmp`, `err` → `tmp1`, `err1`)
- `pkg/preprocessor/null_coalesce.go` - Null coalescing (`coalesce` → `coalesce1`)
- `pkg/preprocessor/safe_nav.go` - Safe navigation (`user` → `user1`, `__user_tmp` → `__user_tmp1`)
- `pkg/plugin/plugin.go` - Plugin temp vars (`tmp` → `tmp1`)

**Rationale:**
- Go convention: camelCase for local variables
- Readability: No visual clutter from underscores
- Consistency: All generators follow same pattern
- Human-like: Generated code looks hand-written

### Planned Features (Priority Order)

1. `Result<T, E>` type (replaces `(T, error)`)
2. `?` operator for error propagation
3. `Option<T>` type (replaces nil checks)
4. Pattern matching (`match` expression)
5. Sum types (`enum` keyword)
6. Automatic Go interop (wrap `(T, error)` → `Result<T, E>`)

## Instructions for AI Agents

### When Adding Context/Research
- Save to `ai-docs/` directory
- Use descriptive filenames: `ai-docs/architecture-decisions.md`, `ai-docs/parser-research.md`
- Update this CLAUDE.md if adding important context

### When Creating Documentation
- **User-facing docs**: → `docs/` directory (when it exists)
- **AI context/research**: → `ai-docs/` directory
- **Root files**: Only README.md and CLAUDE.md
- **Never** create standalone docs in root

### When Implementing Code
- Follow the research recommendations in `claude-research.md` and `gemini_research.md`
- Start with minimal viable features (Result, ?, basic transpilation)
- Prioritize end-to-end functionality over completeness
- Generate idiomatic, readable Go code

### Testing Best Practices & Regression Prevention

**CRITICAL RULE**: If manual testing fails but automated tests pass, the tests are likely wrong or incomplete.

#### The Test Validation Problem

**Scenario**: You implement a feature, write tests, all tests pass ✅, but manual testing shows it's broken ❌.

**Root Causes**:
1. **Tests validate buggy behavior as "correct"**
   - Example: Test expects line 9 (wrong) instead of line 8 (correct)
   - Test passes because it's checking for the bug!
   - Manual testing reveals the actual bug

2. **Test infrastructure has bugs**
   - Example: Tests use stale AST instead of written file
   - Tests compare against wrong baseline
   - Tests can't detect the real issue

3. **Tests don't simulate real usage**
   - Example: LSP hover test doesn't check if symbol exists at position
   - Test checks data structure but not actual behavior
   - Manual testing reveals missing functionality

#### Required Actions When This Happens

**IMMEDIATELY when manual testing contradicts passing tests:**

1. **Stop and Review Test Implementation**
   - Don't assume tests are correct just because they pass
   - Question test expectations: "Why do we expect line 9? Is that actually correct?"
   - Check test infrastructure: "Are we testing the right thing?"

2. **Create Regression Tests**
   - Write a test that captures the manual testing scenario
   - Test should FAIL with the bug, PASS with the fix
   - Include negative tests (verify what should NOT happen)

3. **Verify Test Quality**
   - Would this test catch the bug if we broke the code?
   - Does the test check the actual user-facing behavior?
   - Are test expectations based on correct understanding?

#### Example: Source Map Position Bug (2025-11-22)

**Bug**: LSP hover showed nothing when hovering on `ReadFile`

**Tests**: All passing ✅ (but tests were wrong!)

**Root Cause Investigation**:
```go
// TEST WAS WRONG - Expected buggy behavior as "correct"
expectedGoLine: 9,  // Marker comment line ❌
expectedSymbol: "dingo:e:0",  // Marker text ❌

// SHOULD HAVE BEEN
expectedGoLine: 8,  // Actual code line ✅
expectedSymbol: "ReadFile",  // Actual function ✅
```

**Infrastructure Bug**:
```go
// WRONG - Used preprocessor AST (stale line numbers)
mapGen := NewPostASTGenerator(..., preprocessorAST, ...)

// CORRECT - Re-parse written file (accurate line numbers)
sourceMap := GenerateFromFiles(dingoPath, goPath, metadata)
```

**Regression Tests Added**:
1. `TestSymbolAtTranslatedPosition` - Verifies symbols exist at translated positions
2. `TestNoMappingsToComments` - Ensures mappings never point to comment lines
3. Updated `TestPositionTranslationAccuracy` - Fixed expected values

**Lesson**: Manual testing revealed the bug; automated tests were validating buggy behavior as correct.

#### Test Coverage Blindspots: The Identity Mapping Example (2025-11-22)

**Bug**: LSP Go-to-Definition jumped to wrong line (blank line 7 instead of line 3 function definition)

**Existing Test**: `TestRoundTripTranslation` - PASSED ✅ (but shouldn't have!)

**Why test didn't catch it**:
- Test only checked TRANSFORMED lines (lines with `?` operators)
- Bug was in IDENTITY mappings (untransformed lines like function definitions)
- Test had coverage blindspot - didn't test what it assumed was "simple"

**The Assumption**: "If transformed lines work, untransformed lines must be fine"

**The Reality**: Identity mappings had different bugs:
1. Line offset calculation errors
2. Duplicate mappings for same generated line
3. Wrong mapping selection in reverse lookup

**Lesson**: Test both the complex cases AND the "simple" cases
- ✅ Transformed lines (complex, obvious to test)
- ✅ Untransformed lines (simple, easy to forget)
- ✅ Edge cases (blank lines, comments, package declarations)
- ✅ Reverse operations (not just forward)
- ✅ Real user scenarios (LSP operations)

**Fix Applied**:
1. Expanded `TestRoundTripTranslation` to include untransformed lines:
   - Package declaration (line 1)
   - Function definitions (lines 3, 9) ← **CRITICAL for Go-to-Definition**
   - Return statements (line 5)
   - Regular code (line 11)
2. Added `TestIdentityMappingReverse` specifically for identity mapping reverse lookup
3. Tests now verify both forward AND reverse translation for all line types

**Before**:
```go
testLines: []int{4, 10}, // Two ? operators only
```

**After**:
```go
testLines: []int{
    1,  // package main (identity - CRITICAL)
    3,  // func readConfig (identity - CRITICAL for Go to Definition)
    4,  // ? operator (transformation)
    5,  // return statement (identity)
    9,  // func test (identity)
    10, // ? operator (transformation)
    11, // println (identity)
},
```

**Result**: Tests now expose TWO real bugs:
1. Duplicate mappings for same generated line (e.g., go line 7 maps to both dingo 3 and 7)
2. Wrong mapping selection in reverse lookup (picks duplicate instead of correct mapping)

**Checklist for avoiding coverage blindspots**:
- ✅ Test the complex transformations
- ✅ Test the "simple" pass-through cases
- ✅ Test edge cases (blank lines, comments)
- ✅ Test reverse operations (not just forward)
- ✅ Test real user scenarios (LSP operations)
- ✅ Never assume "simple" code doesn't need tests

#### Test Design Checklist

When writing tests, always verify:

✅ **Correct Expectations**
- Are expected values based on correct understanding?
- Did you verify expectations against actual working behavior?
- Are you testing what SHOULD happen, not what DOES happen?

✅ **Real Behavior Testing**
- Does test simulate actual user workflow?
- For LSP: Does test verify symbols exist at translated positions?
- For transpiler: Does test verify generated code compiles and runs?

✅ **Negative Cases**
- Test what should NOT happen (e.g., no mappings to comments)
- Test error conditions and edge cases
- Verify invalid inputs are rejected

✅ **Test Infrastructure**
- Are you testing against the right artifacts? (written files vs in-memory)
- Does test data match production data?
- Are mocks/fixtures realistic?

✅ **Regression Prevention**
- Would test FAIL if we introduced the bug?
- Can you break the code and see test fail?
- Does test catch the specific bug scenario?

#### When to Distrust Passing Tests

**Red flags that tests might be wrong:**

🚩 Manual testing consistently contradicts test results
🚩 Tests pass but feature doesn't work in real usage
🚩 Test expectations were copied from buggy output
🚩 Tests haven't been updated after major refactoring
🚩 Tests use mocks/fixtures that don't match reality
🚩 Tests check data structures but not actual behavior

**Action**: Review and rewrite tests, don't just add more tests on broken foundation.

#### Manual Testing Remains Critical

**Automated tests are necessary but not sufficient:**

- LSP features: Test in real editor (VSCode, Neovim, etc.)
- Code generation: Inspect actual generated Go code
- Error messages: Verify they're helpful to actual users
- Performance: Measure with realistic workloads

**Best Practice**: After tests pass, always do quick manual smoke test before claiming "done".

### Agent Usage Guidelines

**CRITICAL**: This project has TWO separate development areas with different agents:

#### 1. **Dingo Transpiler/Language** (This Directory)
**Working Directory**: Project root
**Code**: `cmd/`, `pkg/`, `internal/`, `tests/golden/`
**Language**: Go

**Use these agents**:
- ✅ `golang-developer` - Implementation (transpiler, parser, AST, language features)
- ✅ `golang-architect` - Architecture and design
- ✅ `golang-tester` - Testing and golden tests
- ✅ `code-reviewer` - Code review

**Slash commands**:
- ✅ `/dev` - Development orchestrator for Dingo language

#### 2. **Landing Page** (Separate Directory)
**Working Directory**: `langingpage/`
**Code**: `src/`, Astro components, React components
**Language**: TypeScript, Astro, React

**Use these agents**:
- ✅ `astro-developer` - Implementation (landing page, components, styling)
- ✅ `astro-reviewer` - Code review and visual validation
- ⚠️ `code-reviewer` - Can review, but astro-reviewer preferred

**Slash commands**:
- ✅ `/astro-dev` - Development orchestrator for landing page
- ✅ `/astro-fix` - Visual fix orchestrator for landing page

#### ❌ **NEVER Mix Agents**

**WRONG Examples** (DO NOT DO THIS):
- ❌ Using `golang-developer` for Astro/landing page work
- ❌ Using `astro-developer` for transpiler/Go work
- ❌ Using `/dev` in `langingpage/` directory
- ❌ Using `/astro-dev` in root directory

**Correct Examples**:
- ✅ Root directory → Go work → `golang-developer`, `golang-architect`, `golang-tester`, `/dev`
- ✅ `langingpage/` directory → Astro work → `astro-developer`, `astro-reviewer`, `/astro-dev`, `/astro-fix`

#### Quick Decision Guide

**If working on**:
- Parser, AST, transpiler, language features → Use golang-* agents
- Landing page, components, styling, UI → Use astro-* agents
- In doubt? Check your working directory:
  - Root (project root) → golang-* agents
  - Langingpage (`langingpage/`) → astro-* agents

### Common Delegation Patterns (Skills)

For complex delegation workflows, use these **skills** (detailed instructions loaded only when invoked):

**1. Multi-Model Consultation** → Use skill `multi-model-consult`
- **When**: Need perspectives from multiple LLMs (gpt-5, gemini, grok, etc.)
- **Triggers**: "run multiple models", "get perspectives from different models"
- **How**: Skill orchestrates parallel external model consultation via claudish
- **Result**: 2-3x faster, 10x less context, diverse expert opinions

**2. Deep Investigation** → Use skill `investigate`
- **When**: Need to understand how codebase works
- **Triggers**: "how does X work?", "find all usages of Y"
- **How**: Skill delegates to appropriate agent (Explore, golang-developer, etc.)
- **Result**: 10-20x less context, file paths with line numbers

**3. Feature Implementation** → Use skill `implement`
- **When**: Multi-file feature implementation needed
- **Triggers**: "implement feature X", "add support for Y"
- **How**: Skill orchestrates planning → implementation → testing
- **Result**: Structured workflow, parallel execution, tracked progress

**4. Testing** → Use skill `test`
- **When**: Run tests, create tests, fix failing tests
- **Triggers**: "run tests", "create golden tests", "fix failing tests"
- **How**: Skill delegates to golang-tester with appropriate scope
- **Result**: Pass/fail summary, detailed results in files

**Why Skills?**
- **Context Economy**: Detailed patterns loaded ONLY when needed
- **Consistency**: Standardized execution across all delegation tasks
- **Maintainability**: Update patterns in one place, all uses benefit

## 🚨 MANDATORY DELEGATION POLICY

**CRITICAL RULE: Main chat is STRICTLY PROHIBITED from doing detailed work. ALL multi-step tasks, code analysis, implementation, and testing MUST be delegated to specialized agents.**

### What Main Chat CAN Do (Orchestration Only)

✅ **ALLOWED** - High-level orchestration:
- User interaction (questions, approvals, presenting summaries)
- Single git status check
- Single file read for user presentation (NOT for analysis)
- Launching agents via Task tool or Skills
- Coordinating workflow and deciding next steps

❌ **FORBIDDEN** - Any detailed work:
- Reading multiple files (>2 files OR >200 lines total)
- Implementing code or editing files
- Running tests or analyzing output
- Searching codebase (multiple Grep calls)
- Deep analysis or investigation
- Writing detailed documentation

### Mandatory Delegation Triggers

**IF any of these conditions are true → MUST delegate immediately:**

| Condition | Delegate To |
|-----------|-------------|
| Reading 3+ files | Explore or golang-developer agent |
| Implementing any code | golang-developer agent |
| Running tests | golang-tester agent |
| Analyzing architecture | golang-architect agent |
| Code review | code-reviewer agent |
| Multi-step task (>3 steps) | Appropriate specialized agent |
| Codebase investigation | Explore agent (via Skill or Task) |

### Quick Reference: Agent Selection

- **Investigation/Search** → Explore agent (fast, optimized for codebase exploration)
- **Implementation** → golang-developer agent
- **Testing** → golang-tester agent
- **Architecture/Design** → golang-architect agent
- **Code Review** → code-reviewer agent
- **Multi-model consultation** → Use `claudish-usage` skill

### Response Format: Agents Return Summaries Only

Agents MUST return **2-5 sentence summaries** in this format:

```
# [Task Name] Complete

Status: [Success/Partial/Failed]
Key Finding: [One-liner]
Changed: [N] files
Details: [file-path]
```

**Detailed work ALWAYS goes to files. Main chat reads ONLY summaries.**

### Parallel Execution

When tasks are independent, launch agents in **parallel** (single message with multiple Task tool calls):

```
✅ CORRECT: Single message with 3 Task tool calls
❌ WRONG: 3 separate messages with 1 Task tool call each
```

**Benefits**: 2-3x faster execution, all summaries return together.

### Architecture: Three-Layer Pattern

```
Main Chat → Orchestrates, delegates, presents to user
    ↓
Agents → Investigate, implement, analyze (write to files)
    ↓
Files → Detailed results, code, analysis (persistent storage)
```

**Main chat never stores detailed data in conversation context.**

### Full Detailed Guide

For complete delegation patterns, templates, examples, and anti-recursion rules, see:

📖 **`ai-docs/delegation-strategy.md`**

**Key sections in detailed guide:**
- Communication protocols
- Agent self-awareness rules (anti-recursion)
- File-based communication patterns
- Session folder structure
- Complete workflow examples
- Context savings metrics (23x reduction)

## Parallel Multi-Model Review Protocol

When user requests multiple code reviewers (internal + external models):

### Execution Pattern (One-Shot)

**User Request**: "Run internal and external reviewers (grok, minimax, codex, gemini)"

**Required Pattern**:

```
Message 1 (Preparation):
  - Create directories ONLY (Bash mkdir)
  - NO other operations

Message 2 (Parallel Execution):
  - Launch ALL reviewers in SINGLE message
  - ONLY Task tool calls (no Bash, no TodoWrite)
  - Each Task call is independent

Message 3 (Automatic Consolidation):
  - DO NOT wait for user to request consolidation
  - Automatically launch consolidation agent
  - Pass all review file paths

Message 4 (Results):
  - Present consolidated review to user
```

### State Machine

```
PREP → PARALLEL_REVIEW → AUTO_CONSOLIDATE → PRESENT
         ↑ Single message      ↑ Automatic (no user prompt)
```

### Critical Rules

**DO**:
- Separate directory creation from parallel execution
- Use only Task tool in parallel execution message
- Auto-consolidate after N reviews (N ≥ 2)
- Present consolidated results

**DON'T**:
- Mix Bash and Task in same message
- Wait for user to request consolidation
- Launch reviewers sequentially
- Include TodoWrite in parallel execution message

### Example: Correct One-Shot Execution

```
User: "Run internal and 4 external reviewers in parallel"

Assistant Message 1:
  [Bash] mkdir -p ai-docs/sessions/XXX/reviews

Assistant Message 2:
  [Task] Internal review → summary
  [Task] Grok review → summary
  [Task] MiniMax review → summary
  [Task] Codex review → summary
  [Task] Gemini review → summary

Assistant Message 3 (AUTOMATIC - no user prompt):
  [Task] Consolidate reviews → summary

Assistant Message 4:
  "Consolidated review complete: 5 reviewers analyzed..."
```

### Proxy Mode for External Models

When code-reviewer agent uses external models via claudish:

**Required**: Blocking execution
```bash
# CORRECT (blocking):
REVIEW=$(claudish --model openai/gpt-5.1-codex <<'EOF'
Review prompt...
EOF
)

# Write to file
echo "$REVIEW" > review.md

# Return summary (2-5 sentences)
```

**NEVER**: Background execution
```bash
# WRONG (returns too early):
claudish --model ... &
```

---

### Implementation Architecture (Current - December 2025)

**AST-Based Code Generation Pipeline**:

```
.dingo file
    ↓
┌─────────────────────────────────────┐
│ AST Pipeline                         │  pkg/ast/transform.go
│         ast.TransformSource()        │
├─────────────────────────────────────┤
│ Each transform: Parse → Generate     │
│                                      │
│ TransformEnumSource()        enum → Go interface
│ TransformLetSource()         let x = → x :=
│ TransformLambdaSource()      |x| → func(x any) any
│ TransformMatchSource()       match → inline type switch
│ TransformErrorPropSource()   expr? → inline error check
│ TransformTernarySource()     a ? b : c → inline if
│ TransformNullCoalesceSource() a ?? b → inline nil check
│ TransformSafeNavSource()     x?.y → inline nil chain
│ TransformTupleSource()       (a, b) → struct literal
├─────────────────────────────────────┤
│ Returns: Go source + []SourceMapping │
└─────────────────────────────────────┘
    ↓
┌─────────────────────────────────────┐
│ Go Parser & Printer                  │  go/parser + go/printer
├─────────────────────────────────────┤
│ • go/parser.ParseFile()              │  Validate & build AST
│ • go/printer                         │  Output formatted Go code
└─────────────────────────────────────┘
    ↓
.go file (compiles with go build)
```

**Why This Approach?**
- **Modular**: Each feature is a separate parser + codegen
- **Source Maps**: Each transform returns position mappings for LSP
- **Testable**: Parsers and generators can be tested independently
- **Extensible**: Easy to add new features following the pattern
- **Go-native**: Falls through to standard go/parser for final parsing

**Entry Points**:
- `ast.TransformSource()` → Transform Dingo to Go with source mappings
- `parser.ParseFile()` → Get Go AST from Dingo source
- `transpiler.PureASTTranspile()` → Full pipeline for CLI

## Important References

### Research Documents
- `ai-docs/claude-research.md` - Comprehensive guide: tooling, architecture, TypeScript lessons
- `ai-docs/gemini_research.md` - Technical blueprint: transpiler, LSP proxy, implementation roadmap

### Key External Projects
- **Borgo** (github.com/borgo-lang/borgo) - Rust-like → Go transpiler, built own type checker (different goals than Dingo - see "Dingo vs Borgo" section)
- **templ** (github.com/a-h/templ) - gopls proxy architecture reference (Dingo follows this pattern)
- **TypeScript** - Meta-language architecture gold standard

### Essential Go Tools (Actually Used)
- `go/parser` - Native Go parser for transformed code
- `go/scanner` - Token-level transformation in pkg/goparser
- `go/ast`, `go/printer` - Standard library AST manipulation
- `go/token` - FileSet for position tracking
- `go.lsp.dev/protocol` - LSP implementation (future)

## Current Status (December 2025)

### AST-Based Code Generation Complete ✅

The new `pkg/ast/` code generators implement all core Dingo features:

| Feature | Status | Codegen File |
|---------|--------|--------------|
| Enum | ✅ Complete | `enum_codegen.go` |
| Let Declarations | ✅ Complete | `let_codegen.go` |
| Lambdas | ✅ Complete | `lambda_codegen.go` |
| Match | ✅ Complete | `match_codegen.go` |
| Error Propagation | ✅ Complete | `error_prop_codegen.go` |
| Ternary | ✅ Complete | `ternary_codegen.go` |
| Null Coalescing | ✅ Complete | `null_coalesce_codegen.go` |
| Safe Navigation | ✅ Complete | `safe_nav_codegen.go` |
| Tuples | ✅ Complete | `tuple_codegen.go` |

### Pluggable Feature System Complete ✅

All features are now implemented as plugins via `pkg/feature/`:

| Plugin | Priority | Type | Status |
|--------|----------|------|--------|
| enum | 10 | Character | ✅ Complete |
| match | 20 | Character | ✅ Complete |
| enum_constructors | 30 | Character | ✅ Complete |
| error_prop | 40 | Character | ✅ Complete |
| guard_let | 50 | Character | ✅ Complete |
| safe_nav_statements | 55 | Character | ✅ Complete |
| safe_nav | 60 | Character | ✅ Complete |
| null_coalesce | 70 | Character | ✅ Complete |
| lambdas | 80 | Character | ✅ Complete |
| type_annotations | 100 | Token | ✅ Complete |
| generics | 110 | Token | ✅ Complete |
| let_binding | 120 | Token | ✅ Complete |

**Benefits**:
- Enable/disable features via `dingo.toml` `[feature_matrix]`
- Clear error messages when disabled syntax is used
- Extensible for future 3rd-party plugins (v1.1+)

### Test Results

- Parser tests: 16/16 passing
- Feature tests: 27/27 passing
- Examples compile: `examples/01_error_propagation/`, `examples/04_pattern_matching/`

🎯 **Next**:
1. Source map generation from TokenMapping
2. LSP integration
3. Additional type inference for null coalescing assignments

## Architecture Decisions (Resolved)

✅ **Parser Approach**: Token-based transformation + go/parser
  - Character-level passes for complex syntax (enum, match, lambda)
  - Token-level pass for simple syntax (type annotations, let, generics)
  - go/parser for final parsing to AST
  - **Replaces old regex-based preprocessor**

✅ **Pluggable Features**: Static registry with enable/disable config
  - All 12 features implemented as plugins (`pkg/feature/builtin/`)
  - Priority ordering (10-120) ensures correct execution order
  - Dependencies validated (match→enum, guard_let→error_prop, etc.)
  - Configuration via `dingo.toml` `[feature_matrix]` section
  - Future: RPC-based 3rd-party plugins (v1.1+)

✅ **Syntax Style**: Rust-like with Go compatibility
  - `enum Name { Variant }` for sum types
  - `Result<T,E>`, `Option<T>` generic types
  - `?` operator for error propagation
  - `match expr { Pattern => result }` for pattern matching

✅ **Semantic Analysis**: gopls proxy (NOT custom type checker)
  - Dingo parses its own syntax, transforms to Go
  - gopls analyzes the generated Go code
  - LSP proxy translates positions via source maps
  - See "Dingo vs Borgo" section below for rationale

---

## 🎯 Dingo vs Borgo: Critical Architectural Difference

### Why This Matters

Borgo and Dingo are both Go transpilers, but they have **fundamentally different goals** that require **different architectures**.

### The Core Difference

| Aspect | Borgo | Dingo |
|--------|-------|-------|
| **Goal** | New language that compiles to Go | Syntax sugar for Go |
| **Type System** | Rust-like (traits, Hindley-Milner) | Go's type system unchanged |
| **Interop** | Limited - Borgo types ≠ Go types | 100% - Dingo IS Go |
| **Output** | Go code (Borgo-flavored) | Idiomatic Go code |

### Why Borgo Built Its Own Type Checker

Borgo adds **fundamentally new type concepts** that don't exist in Go:

```borgo
// Borgo: Traits (don't exist in Go)
impl Display for User {
    fn display(self) -> string { ... }
}

// Borgo: Hindley-Milner type inference
let x = Some(1)  // Borgo infers Option<int> differently than Go would

// Borgo: Algebraic data types as first-class
enum Result<T, E> { Ok(T), Err(E) }
// This is a REAL sum type in Borgo, not an interface pattern
```

gopls **cannot** type-check Borgo code because Go doesn't have these concepts.

### Why Dingo Does NOT Need Its Own Type Checker

Dingo doesn't add new type concepts - it adds **syntax** for existing Go patterns:

```dingo
// Dingo: Just syntax sugar for Go generics
func fetch() -> Result<User, error> { ... }

// Transforms to REAL Go:
func fetch() Result[User, error] { ... }

// Dingo's Result IS a Go generic type:
type Result[T, E any] struct { ... }  // Standard Go!
```

```dingo
// Dingo enum:
enum Status { Pending, Active, Done }

// Transforms to Go interface pattern:
type Status interface { isStatus() }
type StatusPending struct{}
func (StatusPending) isStatus() {}
// gopls can type-check this perfectly!
```

### The Critical Insight

| | Borgo | Dingo |
|-|-------|-------|
| After transformation | Still needs Borgo semantics | Pure Go - gopls works |
| `Result<T,E>` | Borgo's own type | Go's `Result[T,E]` generic |
| Pattern matching | Borgo's exhaustiveness rules | Transforms to Go switch |
| Type inference | Borgo's rules (different from Go) | Go's rules (unchanged) |

### Architecture Comparison

```
BORGO (must build own type checker):
  .borgo → Borgo Parser → Borgo AST → Borgo Type Checker → Go Code
                                          ↑
                               REQUIRED (Go can't understand Borgo types)

DINGO (use gopls):
  .dingo → Dingo Parser → Transform → .go file → gopls
                                          ↑
                          Just valid Go! gopls works fine
```

### Cost/Benefit Analysis

| Factor | Build Own Type Checker | Use gopls Proxy |
|--------|------------------------|-----------------|
| Engineering effort | 50,000+ LOC, 18-24 months | 5,000-10,000 LOC |
| Maintenance | 1-2 FTE ongoing | Minimal |
| Go compatibility | Risk of drift | Automatic |
| IDE features | Must build everything | Full gopls parity |

### What Dingo Builds (Minimal Semantic Analysis)

Only things gopls **cannot** do:

| Check | Why Dingo Must Do It |
|-------|---------------------|
| Pattern exhaustiveness | Go switch doesn't have this |
| Enum variant validation | Dingo-specific construct |
| `?` in non-Result function | Syntax-level check |
| Error message translation | Make gopls errors Dingo-native |

### What Dingo Delegates to gopls

Everything Go-related:
- Type checking
- Symbol resolution
- Import resolution
- Interface satisfaction
- Generic inference
- Autocomplete, go-to-def, hover, rename, etc.

### Summary

**Borgo** = "I want Rust's type system but compile to Go" → Must build type checker

**Dingo** = "I want nicer syntax for writing Go" → Use gopls, focus on syntax/DX

Dingo's value proposition is **syntax and ergonomics**, not a new type system. Building a Go type checker would be reimplementing `go/types` (30K+ LOC) plus gopls (100K+ LOC). That's not where Dingo's value is.

---

**Last Updated**: 2025-12-07 (Type Inference for Safe Navigation)
**Recent Changes**:
- 2025-12-07: Added `pkg/typechecker/` for go/types integration - infers expression types
- 2025-12-07: Safe navigation now generates human-like code with type inference
- 2025-12-07: All 12 features marked ✅ Complete (safe_nav, null_coalesce updated)
- 2025-12-06: Pluggable feature system complete (`pkg/feature/`) - 12 built-in plugins
- 2025-12-06: FeatureMatrix config integration - enable/disable features via `dingo.toml`
- 2025-12-05: Added "Dingo vs Borgo" architectural comparison and gopls strategy
- 2025-12-05: New token-based parser (pkg/goparser/) replacing regex preprocessor
**Latest Session**: 20251207 (Type Inference)
**Previous Session**: 20251206 (Pluggable Feature System)

### Additional Project Information

- All feature proposals are located in `features/` folder (split per file, e.g., `features/lambdas.md`)
- No backward compatibility needed (pre-release), keep everything simple and clean
- Do not write progress files - update `CHANGELOG.md` instead
- Official domain: **dingolang.com** (landing page)

### Golden Test Guidelines

**IMPORTANT**: When writing or modifying golden tests in `tests/golden/`, follow the comprehensive guidelines in `tests/golden/GOLDEN_TEST_GUIDELINES.md` and `tests/golden/README.md`.

The showcase example `tests/golden/showcase_01_api_server.dingo` is the flagship demo that must be updated whenever new features are implemented.