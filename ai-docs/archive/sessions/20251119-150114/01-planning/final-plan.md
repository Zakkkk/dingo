# Phase V Final Implementation Plan (Infrastructure & Tooling)

**Date**: 2025-11-19
**Session**: 20251119-150114
**Agent**: golang-architect
**Status**: Final Plan (Incorporating User Clarifications)

---

## Executive Summary

Phase V focuses **exclusively on infrastructure, tooling, and developer experience** around already working Dingo features. This plan strictly avoids:
- ❌ Test fixes (handled by separate agent)
- ❌ Transpiler/parser/AST changes (engine is frozen for now)
- ❌ Golden file modifications

**Focus Areas:**
1. Package management documentation and workflow
2. CI/CD visualization and automation
3. Source map validation (read-only checks)
4. Developer experience improvements
5. Workspace build infrastructure (no engine changes)

**Timeline**: 3-4 weeks for critical infrastructure

---

## Critical Constraints (MUST FOLLOW)

### ❌ DO NOT TOUCH

1. **No Dingo Engine Changes**
   - No modifications to `pkg/preprocessor/`
   - No modifications to `pkg/plugin/`
   - No modifications to `pkg/generator/` core logic
   - No AST transformation changes

2. **No Test Fixes**
   - Do not modify `tests/golden/` .dingo files
   - Do not regenerate `.go.golden` files
   - Do not fix failing tests (another agent handles this)

3. **No Feature Implementation**
   - No new language features
   - No syntax changes
   - No type system modifications

### ✅ SAFE TO MODIFY

1. **Infrastructure Code**
   - CI/CD workflows (`.github/workflows/`)
   - Build scripts and tooling
   - Documentation generation

2. **Tooling & CLI**
   - `cmd/dingo/` command-line interface enhancements
   - Package management commands
   - Developer utilities

3. **Documentation**
   - All documentation files
   - Guides, tutorials, examples
   - Architecture documentation

4. **Validation & Reporting**
   - Read-only validation tools
   - Metrics collection
   - Reporting dashboards

---

## Phase V Scope (Infrastructure Only)

### Priority 1: Package Management Documentation (Week 1)

**Goal**: Document and demonstrate hybrid package management workflow

#### Task 1.1: Package Management Strategy Document
**Location**: `docs/package-management.md`

**Content**:
1. **Hybrid Approach Explanation**
   - Libraries: Transpile-on-publish (`.go` files in go.mod)
   - Applications: Direct `.dingo` usage
   - Rationale for each approach

2. **Library Publishing Workflow**
   ```bash
   # Example workflow
   dingo build ./...           # Transpile all .dingo files
   go test ./...               # Verify generated Go compiles
   git add *.go                # Commit transpiled .go files
   git tag v1.0.0              # Tag release
   git push --tags             # Publish
   ```

3. **Application Development Workflow**
   ```bash
   # Example workflow
   dingo build main.dingo      # Transpile entry point
   go run main.go              # Run application
   # OR with LSP: Auto-transpile on save
   ```

4. **Interoperability Patterns**
   - Consuming pure Go packages from Dingo
   - Consuming Dingo libraries from Go projects
   - Mixed codebases (both .dingo and .go)

**Deliverables**:
- ✅ `docs/package-management.md` - Comprehensive guide
- ✅ `examples/library-package/` - Example Dingo library
- ✅ `examples/app-package/` - Example Dingo application
- ✅ `examples/go-consumer/` - Pure Go project consuming Dingo library

**Success Criteria**:
- Clear decision tree for library vs application approach
- Working examples for both patterns
- Migration guide from experimental to production

**Estimated Effort**: 3-4 days

#### Task 1.2: Package Publishing Guide
**Location**: `docs/publishing-packages.md`

**Content**:
1. **Pre-Publish Checklist**
   - Transpile all .dingo files
   - Run `go build` to verify compilation
   - Run tests (both Dingo and Go)
   - Update README with Go import path
   - Version with semantic versioning

2. **Publishing Steps**
   - Tag versions correctly
   - Commit transpiled .go files
   - Push to module repository
   - Optional: Include .dingo sources as documentation

3. **Consumption Guide**
   - How Go projects import Dingo libraries
   - No special tooling required for consumers
   - Type compatibility guarantees

**Deliverables**:
- ✅ `docs/publishing-packages.md` - Step-by-step guide
- ✅ `scripts/publish-package.sh` - Automation script (optional)

**Success Criteria**:
- Publishable example package works with `go get`
- Pure Go projects can consume without Dingo tooling
- Clear versioning and release process

**Estimated Effort**: 2 days

#### Task 1.3: Example Projects
**Location**: `examples/`

**Projects to Create**:

1. **`examples/library-dingo-utils/`** - Dingo Library
   ```
   library-dingo-utils/
   ├── go.mod
   ├── result.dingo          # Source
   ├── result.go             # Transpiled (committed)
   ├── option.dingo          # Source
   ├── option.go             # Transpiled (committed)
   └── README.md             # Usage guide
   ```

2. **`examples/app-todo-cli/`** - Dingo Application
   ```
   app-todo-cli/
   ├── go.mod
   ├── main.dingo            # Entry point (source)
   ├── main.go               # Transpiled (gitignored)
   ├── tasks.dingo           # Business logic
   ├── tasks.go              # Transpiled (gitignored)
   └── README.md
   ```

3. **`examples/go-consumer/`** - Pure Go consuming Dingo library
   ```
   go-consumer/
   ├── go.mod                # Imports library-dingo-utils
   ├── main.go               # Pure Go, no Dingo
   └── README.md             # Shows interop
   ```

**Deliverables**:
- ✅ 3 working example projects
- ✅ Each with comprehensive README
- ✅ Demonstrates best practices

**Success Criteria**:
- `go-consumer` compiles without Dingo tooling installed
- `app-todo-cli` runs with `dingo build && go run`
- `library-dingo-utils` is importable as normal Go package

**Estimated Effort**: 3 days

---

### Priority 2: CI/CD Enhancements (Week 2)

**Goal**: Add visualization and automation (no test fixes)

#### Task 2.1: Golden Test Diff Visualization
**Location**: `.github/workflows/ci.yml`, `scripts/visualize-diffs.sh`

**Implementation**:
1. **Diff Artifact Generation** (when tests fail)
   ```yaml
   - name: Generate test diffs
     if: failure()
     run: |
       go test ./tests -v 2>&1 | tee test-output.log
       ./scripts/visualize-diffs.sh test-output.log > diffs.md

   - name: Upload diff artifacts
     if: failure()
     uses: actions/upload-artifact@v3
     with:
       name: golden-test-diffs
       path: |
         diffs.md
         test-output.log
   ```

2. **Diff Visualization Script** (`scripts/visualize-diffs.sh`)
   - Parse test output
   - Extract expected vs actual differences
   - Format as markdown with syntax highlighting
   - Generate summary table

3. **PR Comment Integration** (optional)
   - Post diff summary as PR comment
   - Link to full diff artifacts
   - Show pass/fail counts by category

**Deliverables**:
- ✅ `scripts/visualize-diffs.sh` - Diff extraction and formatting
- ✅ Updated CI workflow with diff generation
- ✅ Diff artifacts uploaded on failure

**Success Criteria**:
- Failing tests generate readable markdown diffs
- Diffs show exact line-by-line differences
- Artifacts available for 30 days

**Estimated Effort**: 2 days

#### Task 2.2: Performance Tracking Dashboard
**Location**: `.github/workflows/benchmarks.yml`, `scripts/benchmark-tracking/`

**Implementation**:
1. **Benchmark Execution**
   ```yaml
   - name: Run benchmarks
     run: |
       go test ./pkg/... -bench=. -benchmem -count=5 > bench-results.txt
       ./scripts/parse-benchmarks.sh bench-results.txt > metrics.json
   ```

2. **Metrics Collection**
   - Transpilation speed (ns/op, MB/s)
   - LSP position translation latency
   - Memory allocations
   - Cache hit rates

3. **Trend Tracking**
   - Store metrics in git (separate branch or artifact)
   - Generate trend graphs (optional: GitHub Pages)
   - Detect regressions (>10% slowdown = warning)

4. **Reporting**
   - Summary in PR comments (if available)
   - Historical trend visualization
   - Comparison against main branch

**Deliverables**:
- ✅ `.github/workflows/benchmarks.yml` - Separate benchmark workflow
- ✅ `scripts/parse-benchmarks.sh` - Extract metrics from go test output
- ✅ `scripts/detect-regressions.sh` - Compare against baseline
- ✅ Metrics stored as JSON artifacts

**Success Criteria**:
- Benchmarks run on every PR
- Regressions >10% are flagged
- Trend data persisted across runs

**Estimated Effort**: 3 days

#### Task 2.3: Documentation Generation Automation
**Location**: `.github/workflows/docs.yml`, `scripts/generate-docs.sh`

**Implementation**:
1. **Auto-Generate Documentation**
   - Extract feature status from code/tests
   - Generate feature compatibility matrix
   - Create API reference from code comments
   - Build documentation site (mdbook or simple static HTML)

2. **Feature Status Matrix**
   ```markdown
   | Feature           | Status    | Tests     | Examples  |
   |-------------------|-----------|-----------|-----------|
   | Result[T,E]       | ✅ Working | 5/5 pass  | ✅ Available |
   | Option[T]         | ✅ Working | 4/6 pass  | ✅ Available |
   | Error Prop (?)    | ✅ Working | 8/9 pass  | ✅ Available |
   | Pattern Matching  | 🟡 Partial | 5/12 pass | ✅ Available |
   ```

3. **Deployment**
   - Generate on every push to main
   - Deploy to GitHub Pages (or artifact)
   - Versioned documentation (per release tag)

**Deliverables**:
- ✅ `scripts/generate-docs.sh` - Documentation generation
- ✅ `.github/workflows/docs.yml` - CI workflow
- ✅ Auto-generated feature matrix
- ✅ Published documentation (GitHub Pages or artifact)

**Success Criteria**:
- Documentation auto-updates on every merge
- Feature status reflects actual test results
- Examples linked from documentation

**Estimated Effort**: 3 days

---

### Priority 3: Source Map Validation Suite (Week 2-3)

**Goal**: Validation-only (no changes to source map generation)

#### Task 3.1: Round-Trip Validation Tests
**Location**: `tests/sourcemap_validation_test.go`

**Implementation**:
1. **Position Round-Trip Tests**
   ```go
   func TestSourceMapRoundTrip(t *testing.T) {
       // Test Dingo → Go → Dingo
       dingoPos := Position{Line: 10, Column: 5}
       goPos := sm.DingoToGo(dingoPos)
       backToDingo := sm.GoToDingo(goPos)

       if dingoPos != backToDingo {
           t.Errorf("Round-trip failed: %v != %v", dingoPos, backToDingo)
       }

       // Test Go → Dingo → Go
       goPos2 := Position{Line: 15, Column: 8}
       dingoPos2 := sm.GoToDingo(goPos2)
       backToGo := sm.DingoToGo(dingoPos2)

       if goPos2 != backToGo {
           t.Errorf("Round-trip failed: %v != %v", goPos2, backToGo)
       }
   }
   ```

2. **Edge Case Coverage**
   - Multi-line expressions
   - Nested structures
   - Comments and whitespace
   - Preprocessor transformations (enum, ?, etc.)
   - UTF-8 characters

3. **Validation Against Golden Files**
   ```go
   func TestSourceMapAccuracyGolden(t *testing.T) {
       // For each golden test
       for _, test := range goldenTests {
           sourcemap := loadSourceMap(test.name + ".go.golden.map")

           // Validate key positions (function decls, variable refs, etc.)
           validateKeyPositions(t, test, sourcemap)
       }
   }
   ```

**Deliverables**:
- ✅ `tests/sourcemap_validation_test.go` - Comprehensive validation suite
- ✅ Round-trip tests for all position types
- ✅ Edge case coverage
- ✅ Validation report generation

**Success Criteria**:
- 100% of existing source maps pass validation
- Round-trip accuracy >99.9%
- All golden test source maps validated
- CI includes source map validation

**Estimated Effort**: 3 days

#### Task 3.2: Source Map Schema Documentation
**Location**: `docs/sourcemap-format.md`, `pkg/generator/SOURCEMAP.md`

**Content**:
1. **JSON Schema Definition**
   ```json
   {
     "version": 1,
     "mappings": [
       {
         "dingoLine": 10,
         "dingoColumn": 5,
         "goLine": 15,
         "goColumn": 8,
         "length": 10
       }
     ],
     "metadata": {
       "dingoFile": "main.dingo",
       "goFile": "main.go",
       "generatedAt": "2025-11-19T10:30:00Z"
     }
   }
   ```

2. **Format Specification**
   - Field descriptions
   - Position calculation rules
   - Version compatibility
   - Extension points

3. **Usage Examples**
   - How LSP uses source maps
   - How to debug source maps
   - Common pitfalls

**Deliverables**:
- ✅ `docs/sourcemap-format.md` - Public documentation
- ✅ `pkg/generator/SOURCEMAP.md` - Internal specification
- ✅ JSON schema file (for validation tools)

**Success Criteria**:
- Format is fully documented
- Examples cover all use cases
- Future extensions considered

**Estimated Effort**: 2 days

#### Task 3.3: CI Integration for Source Map Validation
**Location**: `.github/workflows/ci.yml`

**Implementation**:
```yaml
- name: Validate source maps
  run: go test -v ./tests -run TestSourceMapValidation

- name: Generate source map report
  if: failure()
  run: |
    ./scripts/sourcemap-report.sh > sourcemap-report.md

- name: Upload source map artifacts
  if: failure()
  uses: actions/upload-artifact@v3
  with:
    name: sourcemap-validation-failures
    path: sourcemap-report.md
```

**Deliverables**:
- ✅ CI workflow includes source map validation
- ✅ Failures generate detailed reports
- ✅ Artifacts uploaded for debugging

**Success Criteria**:
- Source map validation runs on every PR
- Failures are clearly reported
- CI blocks on validation failures

**Estimated Effort**: 1 day

---

### Priority 4: Developer Experience Improvements (Week 3)

**Goal**: Enhance developer-facing documentation and tooling

#### Task 4.1: Getting Started Guide
**Location**: `docs/getting-started.md`

**Content**:
1. **Installation**
   - Building from source
   - Installing VS Code extension
   - Verifying installation

2. **First Dingo Project**
   - Creating main.dingo
   - Using Result[T,E] and Option[T]
   - Error propagation with ?
   - Transpiling and running

3. **IDE Setup**
   - Configuring VS Code
   - LSP features overview
   - Auto-transpile on save

4. **Next Steps**
   - Link to feature documentation
   - Link to examples
   - Community resources

**Deliverables**:
- ✅ `docs/getting-started.md` - Comprehensive guide
- ✅ Step-by-step with screenshots (if feasible)
- ✅ Working example code

**Success Criteria**:
- Complete beginner can follow guide
- Guide takes <15 minutes
- All links work

**Estimated Effort**: 2 days

#### Task 4.2: Feature Documentation Index
**Location**: `docs/features/`

**Content**:
Organize existing feature documentation into discoverable structure:
```
docs/features/
├── README.md                 # Feature index and status
├── result-type.md            # Result[T,E] documentation
├── option-type.md            # Option[T] documentation
├── error-propagation.md      # ? operator
├── pattern-matching.md       # match expressions
├── enums.md                  # Sum types/enums
└── type-annotations.md       # : syntax
```

For each feature:
- Syntax explanation
- Examples (basic → advanced)
- Common patterns
- Gotchas and limitations
- Related features

**Deliverables**:
- ✅ `docs/features/README.md` - Feature index
- ✅ Individual feature docs (7+ files)
- ✅ Cross-linking between related features

**Success Criteria**:
- All working features documented
- Examples compile and run
- Clear status indicators (stable, experimental, planned)

**Estimated Effort**: 3 days

#### Task 4.3: Migration Guide (Go → Dingo)
**Location**: `docs/migration-guide.md`

**Content**:
1. **When to Migrate**
   - Use cases where Dingo shines
   - When to stick with pure Go
   - Gradual migration strategy

2. **Common Patterns**
   - Error handling: `if err != nil` → `?`
   - Nullable values: `*T` → `Option[T]`
   - Tagged unions: interfaces → enums
   - Pattern matching: type switches → match

3. **Before/After Examples**
   - Real-world code snippets
   - Metrics (lines saved, clarity improvements)

4. **Interoperability**
   - Calling Go from Dingo
   - Calling Dingo from Go
   - Mixed codebases

**Deliverables**:
- ✅ `docs/migration-guide.md` - Complete guide
- ✅ 5+ before/after examples
- ✅ Decision framework for migration

**Success Criteria**:
- Guide helps developers decide if/when to migrate
- Clear ROI explanation
- Practical migration strategies

**Estimated Effort**: 2 days

---

### Priority 5: Workspace Build Infrastructure (Week 3-4)

**Goal**: Multi-package support (no engine changes)

#### Task 5.1: Workspace Detection
**Location**: `pkg/workspace/scanner.go`

**Implementation**:
```go
type Workspace struct {
    Root     string
    Packages []Package
}

type Package struct {
    Path    string   // Relative to workspace root
    Dingo   []string // .dingo files
    Go      []string // .go files (existing)
}

func ScanWorkspace(root string) (*Workspace, error) {
    // Walk directory tree
    // Find all .dingo files
    // Group by package (directory)
    // Detect go.mod boundaries
}
```

**Features**:
- Detect all .dingo files in workspace
- Group by package (directory)
- Respect .dingoignore (like .gitignore)
- Detect module boundaries (go.mod)

**Deliverables**:
- ✅ `pkg/workspace/scanner.go` - Workspace scanner
- ✅ `pkg/workspace/scanner_test.go` - Tests
- ✅ Support for .dingoignore files

**Success Criteria**:
- Correctly scans nested packages
- Respects ignore patterns
- Fast (scans 1000 files in <100ms)

**Estimated Effort**: 2 days

#### Task 5.2: Build Orchestration
**Location**: `pkg/workspace/builder.go`

**Implementation**:
```go
type Builder struct {
    Workspace *Workspace
    Options   BuildOptions
}

type BuildOptions struct {
    Parallel    bool  // Build packages in parallel
    Incremental bool  // Only rebuild changed files
    Verbose     bool  // Logging
}

func (b *Builder) BuildAll() error {
    // For each package in dependency order
    for _, pkg := range b.Workspace.Packages {
        if err := b.buildPackage(pkg); err != nil {
            return err
        }
    }
    return nil
}

func (b *Builder) buildPackage(pkg Package) error {
    // Transpile all .dingo files in package
    // Use existing transpiler (no changes)
    // Generate .go files + source maps
}
```

**Features**:
- Build all packages in workspace
- Parallel builds (independent packages)
- Incremental builds (track modification times)
- Progress reporting

**Deliverables**:
- ✅ `pkg/workspace/builder.go` - Build orchestration
- ✅ `pkg/workspace/builder_test.go` - Tests
- ✅ Progress reporting with logging

**Success Criteria**:
- Builds multi-package workspaces correctly
- Parallel builds 2-4x faster
- Incremental builds skip unchanged files

**Estimated Effort**: 3 days

#### Task 5.3: CLI Integration (`dingo build ./...`)
**Location**: `cmd/dingo/build.go`

**Implementation**:
```go
// Existing: dingo build file.dingo (single file)
// New:      dingo build ./...       (workspace-wide)
// New:      dingo build ./pkg/...   (package pattern)

func buildCommand(args []string) error {
    if strings.HasSuffix(args[0], "...") {
        // Workspace build
        ws, err := workspace.ScanWorkspace(".")
        if err != nil {
            return err
        }

        builder := workspace.NewBuilder(ws, options)
        return builder.BuildAll()
    } else {
        // Single file build (existing code)
        return buildFile(args[0])
    }
}
```

**Features**:
- `dingo build ./...` - Build all packages
- `dingo build ./pkg/...` - Build package pattern
- `dingo build file.dingo` - Single file (existing)
- Flags: `-parallel`, `-incremental`, `-v`

**Deliverables**:
- ✅ Updated `cmd/dingo/build.go` with workspace support
- ✅ Pattern matching for packages
- ✅ Backward compatibility with single-file builds

**Success Criteria**:
- `dingo build ./...` works in multi-package projects
- Existing single-file builds still work
- Clear error messages for failures

**Estimated Effort**: 2 days

---

## Timeline Summary

### Week 1: Package Management Documentation
- **Days 1-2**: Package management strategy document
- **Day 3**: Publishing guide
- **Days 4-5**: Example projects (library, app, Go consumer)

**Deliverables**:
- ✅ 3 documentation files
- ✅ 3 example projects
- ✅ Publishing workflow

### Week 2: CI/CD & Source Maps
- **Days 1-2**: Golden test diff visualization
- **Days 3-4**: Performance tracking dashboard
- **Day 5**: Documentation generation automation
- **Days 6-7**: Source map validation tests
- **Day 8**: Source map schema documentation

**Deliverables**:
- ✅ Enhanced CI workflows (3 additions)
- ✅ Source map validation suite
- ✅ Schema documentation

### Week 3: Developer Experience
- **Days 1-2**: Getting started guide
- **Days 3-4**: Feature documentation index
- **Day 5**: Migration guide
- **Days 6-7**: Source map CI integration

**Deliverables**:
- ✅ Developer-facing documentation (3 guides)
- ✅ Feature documentation structure

### Week 4: Workspace Builds (Optional)
- **Days 1-2**: Workspace scanner
- **Days 3-5**: Build orchestration
- **Days 6-7**: CLI integration

**Deliverables**:
- ✅ Multi-package build support
- ✅ `dingo build ./...` command

---

## Success Criteria

### Quality Gates

**Documentation**:
- ✅ Package management strategy documented
- ✅ All working features documented
- ✅ Getting started guide complete
- ✅ Migration guide complete

**CI/CD**:
- ✅ Diff visualization working
- ✅ Performance tracking active
- ✅ Documentation auto-generated

**Validation**:
- ✅ Source map validation suite passing
- ✅ Round-trip accuracy >99.9%
- ✅ Schema fully documented

**Tooling**:
- ✅ Workspace builds working (if implemented)
- ✅ CLI supports package patterns

### Metrics

**Documentation Coverage**:
- 100% of working features documented
- Getting started guide <15 minutes to complete
- 3+ example projects available

**CI/CD Coverage**:
- Diff visualization on all test failures
- Performance benchmarks on every PR
- Documentation updates on every merge

**Validation Coverage**:
- 100% of existing source maps validated
- All golden test source maps checked
- CI blocks on validation failures

---

## Risk Mitigation

### Low Risk Items

1. **Documentation** - Standard markdown writing, low technical risk
   - Mitigation: Review with external readers for clarity

2. **CI Enhancements** - Well-understood GitHub Actions patterns
   - Mitigation: Test workflows in feature branches

3. **Source Map Validation** - Read-only validation, no generation changes
   - Mitigation: Comprehensive test coverage

### Medium Risk Items

4. **Workspace Builds** - New infrastructure, potential complexity
   - Mitigation: Start with simple scanner, iterate incrementally
   - Fallback: Defer to future phase if complex

5. **Performance Tracking** - Benchmark variability
   - Mitigation: Use statistical analysis, multiple runs
   - Fallback: Manual benchmark review if automation unreliable

---

## Dependencies

### External Dependencies
- ✅ GitHub Actions (already in use)
- ✅ Codecov (already integrated)
- ✅ Go 1.21+ (required)

### Internal Dependencies
- ✅ Existing transpiler (no changes needed)
- ✅ Existing LSP (no changes needed)
- ✅ Source map generation (already working)

### Blocks Future Work
- Package management docs → Enable library ecosystem
- Workspace builds → Multi-package projects
- Documentation → User onboarding and adoption

---

## Out of Scope (Explicitly NOT Included)

❌ **Test Fixes**
- No modifications to failing tests
- No golden file regeneration
- No test expectations updates

❌ **Engine Changes**
- No preprocessor changes
- No parser modifications
- No AST transformation updates
- No type inference changes

❌ **New Features**
- No new language syntax
- No new type system features
- No new transformations

❌ **Debugging Support**
- No DAP implementation
- No Delve integration
- Deferred to future phase

❌ **Advanced Tooling**
- No formatter (`dingo fmt`)
- No linter (`dingo lint`)
- No watch mode (`dingo dev`)
- Deferred to future phase

---

## Conclusion

This Phase V plan focuses on **infrastructure, tooling, and documentation** that can be delivered **without touching the Dingo engine or test suite**. The work is low-risk, high-value, and complements the ongoing test fixes being handled by another agent.

**Recommended Execution Order**:
1. **Week 1**: Package management docs (enables ecosystem)
2. **Week 2**: CI/CD + source maps (improves quality visibility)
3. **Week 3**: Developer docs (improves onboarding)
4. **Week 4**: Workspace builds (optional, enables scale)

**Quality Gate**: Completion of Weeks 1-3 provides substantial infrastructure improvements without any engine modifications.

**Coordination**: This plan assumes another agent is fixing failing tests in parallel. No conflicts expected as we're operating on completely separate file sets.
