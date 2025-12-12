# Final Implementation Plan: Fix Build Issues

**Session**: 20251117-204314
**Status**: Approved - Ready for Execution
**Total Estimated Time**: 5 hours

## User Clarifications Incorporated

1. **Import Detection**: Implement automatic import detection (production-ready, 2 hours)
2. **Transform Pipeline**: Check for other features (lambdas, pattern matching) before removing code
3. **File Management**: Delete `error_prop.go` completely (no archiving, git history sufficient)

## Problem Summary

The Dingo project has critical build failures preventing compilation:

1. **Duplicate Method Declaration**: `transformErrorProp` defined in both `transformer.go:104` and `error_prop.go:15`
2. **Unused Variables**: Lines 67, 71, 194 in `error_prop.go`
3. **Missing Imports**: Golden test files missing necessary standard library imports

### Root Cause

- **Architectural Confusion**: Two separate transformation systems (preprocessor and transformer) both handling error propagation
- **Incomplete Migration**: AST transformer has partial error propagation implementation that duplicates working preprocessor
- **Missing Import Tracking**: Preprocessor generates function calls without adding necessary imports

## Final Architecture

### Package Responsibilities

```
pkg/
├── preprocessor/          # Line-level transformations (PRIMARY)
│   ├── preprocessor.go    # Main orchestrator
│   └── error_prop.go      # Error propagation (? operator) - COMPLETE
│
├── transform/             # AST-level transformations (FUTURE)
│   └── transformer.go     # Lambdas, pattern matching, safe nav
│
├── parser/                # Go parser wrapper
│   └── parser.go          # Standard go/parser wrapper
│
└── generator/             # Code generation
    └── generator.go       # go/printer wrapper with plugins
```

### Pipeline Flow

```
.dingo file
    ↓
[Preprocessor] → Go source (? expanded) + imports + mappings
    ↓
[Parser] → AST
    ↓
[Transformer] → Modified AST (lambdas, pattern matching)
    ↓
[Generator] → Final .go file
```

## Implementation Plan

### Phase 1: Code Audit & Cleanup (1.5 hours)

#### Step 1.1: Audit Transform Pipeline (30 minutes)

**Action**: Examine `pkg/transform/transformer.go` for other features

**Files to Check**:
- `pkg/transform/transformer.go` - Main transformer
- `pkg/transform/*.go` - Any other feature files

**Verification Checklist**:
- [ ] Check for lambda transformation code
- [ ] Check for pattern matching code
- [ ] Check for safe navigation code
- [ ] Check for any other partial implementations
- [ ] Document findings

**Expected Outcome**: List of features to preserve vs. remove

#### Step 1.2: Remove Error Propagation from Transformer (30 minutes)

**File: `pkg/transform/error_prop.go`**
- **Action**: DELETE entire file
- **Rationale**: Error propagation fully handled by preprocessor (693 lines, production-ready)
- **Git**: No archiving needed (git history preserves)

**File: `pkg/transform/transformer.go`**
- **Action**: Remove `transformErrorProp` method (lines 103-108)
- **Action**: Remove error propagation case from `handlePlaceholderCall` (if present)
- **Preserve**: Any working lambda, pattern matching, or other feature code

**Verification**:
```bash
# Should build without errors
go build ./pkg/transform/...
```

#### Step 1.3: Update Transformer Tests (30 minutes)

**File: `pkg/transform/transformer_test.go`**
- **Action**: Remove error propagation test cases
- **Action**: Keep tests for other features
- **Action**: Add comment documenting error propagation is in preprocessor

**Verification**:
```bash
go test ./pkg/transform/...
```

### Phase 2: Implement Automatic Import Detection (2 hours)

#### Step 2.1: Design Import Tracking System (30 minutes)

**File: `pkg/preprocessor/error_prop.go`**

Add import tracking infrastructure:

```go
// ImportTracker manages automatic import detection
type ImportTracker struct {
    needed  map[string]bool  // package path → needed
    aliases map[string]string // funcName → package path
}

// Common standard library functions that need imports
var stdLibFunctions = map[string]string{
    // os package
    "ReadFile":   "os",
    "WriteFile":  "os",
    "Open":       "os",
    "Create":     "os",
    "Stat":       "os",

    // encoding/json
    "Marshal":    "encoding/json",
    "Unmarshal":  "encoding/json",

    // strconv
    "Atoi":       "strconv",
    "Itoa":       "strconv",
    "ParseInt":   "strconv",
    "ParseFloat": "strconv",
    "ParseBool":  "strconv",

    // io
    "ReadAll":    "io",

    // fmt (already tracked via needsFmt)
    "Sprintf":    "fmt",
    "Fprintf":    "fmt",
    "Printf":     "fmt",
}

func NewImportTracker() *ImportTracker {
    return &ImportTracker{
        needed:  make(map[string]bool),
        aliases: stdLibFunctions,
    }
}

func (it *ImportTracker) TrackFunctionCall(funcName string) {
    if pkg, exists := it.aliases[funcName]; exists {
        it.needed[pkg] = true
    }
}

func (it *ImportTracker) GetNeededImports() []string {
    imports := make([]string, 0, len(it.needed))
    for pkg := range it.needed {
        imports = append(imports, pkg)
    }
    sort.Strings(imports)
    return imports
}
```

#### Step 2.2: Integrate Import Tracking (45 minutes)

**File: `pkg/preprocessor/error_prop.go`**

Modify `ErrorPropProcessor` struct:

```go
type ErrorPropProcessor struct {
    tryCounter     int
    lines          []string
    currentFunc    *funcContext
    needsFmt       bool
    importTracker  *ImportTracker  // NEW
}
```

Add tracking in function call parsing:

```go
func (e *ErrorPropProcessor) parseFunctionSignature(line string, pos int) (*funcCallInfo, error) {
    // ... existing parsing logic ...

    // NEW: Track function call for imports
    if e.importTracker != nil {
        e.importTracker.TrackFunctionCall(info.funcName)
    }

    return info, nil
}
```

#### Step 2.3: Implement Import Injection (45 minutes)

**File: `pkg/preprocessor/error_prop.go`**

Add import injection method:

```go
func (e *ErrorPropProcessor) injectImports(source []byte) []byte {
    // Parse existing imports
    fset := token.NewFileSet()
    node, err := parser.ParseFile(fset, "", source, parser.ImportsOnly)
    if err != nil {
        return source // Fallback: return original
    }

    // Get needed imports
    needed := e.importTracker.GetNeededImports()
    if e.needsFmt {
        needed = append(needed, "fmt")
    }

    // Add missing imports using astutil
    for _, pkg := range needed {
        astutil.AddImport(fset, node, pkg)
    }

    // Generate updated source with imports
    var buf bytes.Buffer
    if err := printer.Fprint(&buf, fset, node); err != nil {
        return source
    }

    // Merge: new imports + original body (skip old imports)
    return e.mergeImports(buf.Bytes(), source)
}

func (e *ErrorPropProcessor) mergeImports(newImports, originalSource []byte) []byte {
    // Extract everything after import block from original
    // Combine new import block + original body
    // Implementation details...
}
```

Update `Process()` method:

```go
func (e *ErrorPropProcessor) Process(source []byte) ([]byte, []Mapping, error) {
    e.lines = strings.Split(string(source), "\n")
    e.currentFunc = nil
    e.needsFmt = false
    e.importTracker = NewImportTracker()  // NEW: Initialize tracker

    // ... existing processing logic ...

    result := e.lines

    // NEW: Inject imports before returning
    result = e.injectImports([]byte(strings.Join(result, "\n")))

    return []byte(result), e.mappings, nil
}
```

### Phase 3: Update Source Mappings (1 hour)

#### Step 3.1: Adjust Mapping Offsets (45 minutes)

**Challenge**: Adding imports shifts line numbers

**Solution**: Update mappings after import injection:

```go
func (e *ErrorPropProcessor) adjustMappingsForImports(numImportLines int) {
    // Shift all Dingo line numbers down by import count
    for i := range e.mappings {
        if e.mappings[i].DingoLine > 0 {
            e.mappings[i].GoLine += numImportLines
        }
    }
}
```

Call in `Process()` after `injectImports()`:

```go
func (e *ErrorPropProcessor) Process(source []byte) ([]byte, []Mapping, error) {
    // ... processing ...

    originalLineCount := len(e.lines)
    result := e.injectImports([]byte(strings.Join(e.lines, "\n")))
    newLineCount := len(strings.Split(string(result), "\n"))

    importLinesAdded := newLineCount - originalLineCount
    e.adjustMappingsForImports(importLinesAdded)

    return result, e.mappings, nil
}
```

#### Step 3.2: Test Mapping Accuracy (15 minutes)

**Unit Test**: Verify mappings remain accurate

```go
func TestSourceMappingWithImports(t *testing.T) {
    source := `package main

func example() {
    data := ReadFile("test.txt")?
    return data
}`

    proc := NewErrorPropProcessor()
    result, mappings, err := proc.Process([]byte(source))
    require.NoError(t, err)

    // Verify import was added
    assert.Contains(t, string(result), `import "os"`)

    // Verify mapping points to correct line (accounting for import)
    // Original line 4 should map to Go line 6 (after import block)
    found := false
    for _, m := range mappings {
        if m.DingoLine == 4 {
            assert.Equal(t, 6, m.GoLine)
            found = true
        }
    }
    assert.True(t, found, "Mapping for line 4 not found")
}
```

### Phase 4: Verification & Testing (30 minutes)

#### Step 4.1: Build Verification (10 minutes)

```bash
# Clean build
go clean ./...

# Build all packages
go build ./...

# Should succeed with zero errors
echo "Build status: $?"
```

#### Step 4.2: Unit Tests (10 minutes)

```bash
# Run all package tests
go test ./pkg/... -v

# Specific focus
go test ./pkg/preprocessor/... -v
go test ./pkg/transform/... -v
```

#### Step 4.3: Golden Test Compilation (10 minutes)

```bash
# Test that all golden .go files compile
cd tests/golden
for f in *.go.golden; do
    go build -o /tmp/test_$$ "$f" 2>&1
    if [ $? -ne 0 ]; then
        echo "FAIL: $f"
    else
        echo "PASS: $f"
    fi
done
rm -f /tmp/test_$$
```

### Phase 5: Documentation (30 minutes)

#### Step 5.1: Update CHANGELOG.md (10 minutes)

```markdown
## [Unreleased]

### Fixed
- **Build**: Resolved duplicate `transformErrorProp` method declaration
- **Build**: Fixed missing imports in golden test files
- **Build**: Removed unused variables in transform package

### Changed
- **Architecture**: Clarified preprocessor vs transformer responsibilities
  - Preprocessor: Error propagation (? operator), simple syntax sugar
  - Transformer: Complex AST features (lambdas, pattern matching)
- **Preprocessor**: Added automatic import detection for standard library functions

### Removed
- **Transform**: Removed duplicate error propagation implementation from AST transformer
- **Transform**: Deleted `pkg/transform/error_prop.go` (functionality in preprocessor)
```

#### Step 5.2: Document Architecture Decision (15 minutes)

**File: `pkg/preprocessor/README.md`** (Create)

```markdown
# Preprocessor Package

## Purpose

The preprocessor handles text-based transformations of Dingo syntax into valid Go syntax.

## Responsibilities

- **Error Propagation (`?` operator)**: Primary implementation
- **Automatic Import Detection**: Tracks function calls and adds necessary imports
- **Source Mapping**: Maintains bidirectional mappings between Dingo and Go

## Why Preprocessor vs Transformer?

Error propagation is implemented in the preprocessor because:
1. **Simplicity**: Regex-based line transformations are simpler than AST manipulation
2. **Source Mapping**: Easier to maintain accurate mappings with line-level transforms
3. **Performance**: Text processing faster than AST parsing/printing
4. **Proven**: 693 lines of battle-tested production code

Complex features requiring type information (lambdas, pattern matching) belong in the AST transformer.
```

**File: `pkg/transform/README.md`** (Create)

```markdown
# Transform Package

## Purpose

The transformer handles AST-based transformations for complex Dingo features.

## Responsibilities

- **Lambda Functions**: Transform lambda syntax to anonymous functions
- **Pattern Matching**: Transform match expressions to type switches
- **Safe Navigation**: Transform `?.` operator to nil checks

## What This Does NOT Handle

- **Error Propagation (`?` operator)**: Implemented in `pkg/preprocessor`
- **Simple Syntax Sugar**: Preprocessor is better suited

## Pipeline Position

```
Preprocessor → Parser → [Transformer] → Generator
```

The transformer receives an AST of valid Go code (with ? already expanded) and transforms complex features.
```

#### Step 5.3: Update Session Log (5 minutes)

**File: `ai-docs/sessions/20251117-204314/implementation-log.md`** (Create during implementation)

## Quality Checklist

- [ ] `go build ./...` succeeds with zero errors
- [ ] `go test ./pkg/...` all tests pass
- [ ] All 46 golden `.go.golden` files compile successfully
- [ ] No duplicate method declarations
- [ ] No unused variable warnings
- [ ] Source maps accurate after import additions
- [ ] Import detection handles all standard library functions in golden tests
- [ ] Documentation updated (CHANGELOG.md, package READMEs)

## Success Criteria

1. **Build Success**: Clean build of entire project
2. **Test Success**: All unit tests pass
3. **Golden Tests**: All test files compile without import errors
4. **Code Quality**: Zero warnings, zero duplication
5. **Architecture**: Clear separation of concerns documented
6. **Maintainability**: Future contributors understand preprocessor vs transformer split

## Risk Mitigation

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Import detection misses edge cases | Medium | Low | Comprehensive unit tests, graceful fallback |
| Source map offsets incorrect | Low | Medium | Dedicated mapping tests, visual verification |
| Other features broken during cleanup | Low | High | **Audit transform pipeline first** |
| Import conflicts with user code | Low | Low | `astutil.AddImport` handles duplicates |

## Timeline

| Phase | Duration | Dependencies |
|-------|----------|--------------|
| 1. Code Audit & Cleanup | 1.5 hours | None |
| 2. Import Detection | 2 hours | Phase 1 |
| 3. Source Mapping | 1 hour | Phase 2 |
| 4. Verification | 30 min | Phase 3 |
| 5. Documentation | 30 min | Phase 4 |
| **Total** | **5.5 hours** | Sequential |

## Implementation Order

1. **Start with Audit**: Must identify what to preserve before deleting
2. **Remove Duplicates**: Clean slate for import detection
3. **Implement Imports**: Core feature
4. **Fix Mappings**: Ensure accuracy
5. **Verify**: Confirm everything works
6. **Document**: Record decisions

## Next Steps After Completion

Once build is successful, continue with:

1. **Phase 2.8**: Lambda transformations in `pkg/transform`
2. **Phase 2.9**: Pattern matching in `pkg/transform`
3. **Phase 3**: Result/Option type integration
4. **Phase 4**: Language server development

The cleaned architecture provides a solid foundation for these features.

## Notes

- **Git History**: Deleted `error_prop.go` preserved in git history if needed
- **Import Library**: Using `golang.org/x/tools/go/ast/astutil` for safe import management
- **Testing**: Focus on golden test compilation as primary success metric
- **Documentation**: Architecture decision recorded for future contributors
