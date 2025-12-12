# Phase 4 Implementation Plan - Pattern Matching & Enhanced Type Inference

**Status**: Architecture Plan
**Date**: 2025-11-18
**Phase**: 4 of 6 (Implementation Roadmap)

---

## Executive Summary

Phase 4 focuses on implementing pattern matching (`match` expressions) and completing the type inference infrastructure with full go/types integration. This phase builds on Phase 3's Result/Option types and enables exhaustive, type-safe destructuring of sum types.

**Primary Goals**:
1. Pattern matching with exhaustiveness checking
2. Full go/types context integration (AST parent tracking)
3. Context-aware None constant inference
4. Enhanced error messages with actionable suggestions

**Estimated Timeline**: 3-4 weeks

---

## Architecture Overview

### High-Level Design

Phase 4 introduces two major systems:

1. **Pattern Matching Subsystem**
   - Preprocessor: `match` syntax → Go switch statements
   - Plugin: Exhaustiveness checker + pattern compiler
   - Integration: Works with existing Result/Option/Enum types

2. **Enhanced Type Inference Subsystem**
   - Full go/types integration with AST parent tracking
   - Context propagation through expression trees
   - None constant type resolution via context analysis
   - Improved error reporting with fix suggestions

### Component Relationships

```
┌─────────────────────────────────────────────────────────────┐
│                    .dingo Source File                        │
│  match result {                                              │
│    Ok(user) => processUser(user),                           │
│    Err(e) => return None                                    │
│  }                                                           │
└────────────────────┬─────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────┐
│         STAGE 1: Preprocessor (Pattern Matching)            │
├─────────────────────────────────────────────────────────────┤
│  PatternMatchProcessor:                                      │
│    - Parse match expression (capture scrutinee + arms)      │
│    - Validate pattern syntax (destructuring, guards)        │
│    - Generate switch statement skeleton                     │
│    - Preserve pattern bindings for AST stage                │
│    - Track exhaustiveness requirements                      │
├─────────────────────────────────────────────────────────────┤
│  Output: Valid Go switch with marker comments               │
└────────────────────┬─────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────┐
│          Valid Go Code (with pattern markers)                │
│  switch { /* DINGO_MATCH: Result[User,error] */            │
│    case __result.tag == ResultTag_Ok: /* Ok(user) */       │
│      user := *__result.ok_0                                 │
│      processUser(user)                                       │
│    case __result.tag == ResultTag_Err: /* Err(e) */        │
│      e := __result.err_0                                     │
│      return None /* DINGO_NONE_CONTEXT: return */           │
│  }                                                           │
└────────────────────┬─────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────┐
│          STAGE 2: AST Processing (Enhanced)                  │
├─────────────────────────────────────────────────────────────┤
│  go/parser: Parse preprocessed Go → AST                     │
│                                                              │
│  Enhanced Context System:                                    │
│    - Build AST parent map (node → parent)                   │
│    - Run go/types checker with full context                 │
│    - Store types.Info in pipeline context                   │
│                                                              │
│  PatternMatchPlugin (NEW):                                   │
│    Phase 1 - Discovery:                                      │
│      - Find DINGO_MATCH markers                             │
│      - Extract scrutinee type from go/types                 │
│      - Collect pattern arms                                 │
│    Phase 2 - Exhaustiveness Check:                          │
│      - Determine all possible variants (Result, Option, Enum)│
│      - Verify all variants are covered                      │
│      - Report missing cases with helpful errors             │
│    Phase 3 - Pattern Compilation:                           │
│      - Generate case conditions (tag checks)                │
│      - Extract pattern bindings                             │
│      - Handle guards (if conditions)                        │
│                                                              │
│  NoneContextPlugin (NEW):                                    │
│    Phase 1 - Discovery:                                      │
│      - Find DINGO_NONE_CONTEXT markers                      │
│      - Walk parent chain to find type context               │
│    Phase 2 - Type Inference:                                │
│      - Extract expected type from context:                  │
│        * Return statement → function signature              │
│        * Assignment → variable type                         │
│        * Function call → parameter type                     │
│      - Resolve None to Option[T]                            │
│    Phase 3 - Transform:                                      │
│      - Replace None with Option_T{isSet: false}             │
│                                                              │
│  Existing Plugins (Enhanced):                                │
│    - ResultTypePlugin: Uses go/types for inference          │
│    - OptionTypePlugin: Uses go/types for inference          │
└────────────────────┬─────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────┐
│             Code Generation (go/printer)                     │
│  .go file + .sourcemap + .diagnostics                        │
└─────────────────────────────────────────────────────────────┘
```

---

## Feature Specifications

### 1. Pattern Matching (`match` Expression)

#### Syntax

**Basic Match**:
```dingo
match value {
    Pattern1 => expression1,
    Pattern2 => expression2
}
```

**With Block Bodies**:
```dingo
match value {
    Pattern1 => {
        // multiple statements
    },
    Pattern2 => {
        // multiple statements
    }
}
```

**With Guards**:
```dingo
match value {
    Pattern1 if condition => expr,
    Pattern2 => expr
}
```

#### Supported Patterns

**Phase 4.1 (MVP - 2 weeks)**:
- ✅ Enum variant matching: `Color.Red`, `Color.Green`
- ✅ Result destructuring: `Ok(value)`, `Err(error)`
- ✅ Option destructuring: `Some(value)`, `None`
- ✅ Wildcard: `_` (catch-all)
- ✅ Variable binding: `Ok(x)` binds `x`
- ✅ Guards: `Some(x) if x > 10`

**Phase 4.2 (Advanced - Future)**:
- ⏳ Struct destructuring: `User{name, age}`
- ⏳ Tuple destructuring: `(x, y)`
- ⏳ Nested patterns: `Ok(Some(User{name}))`
- ⏳ Literal matching: `42`, `"string"`
- ⏳ Range patterns: `1..10`

#### Exhaustiveness Checking

**Algorithm**:
1. Determine scrutinee type from go/types
2. Extract all possible variants:
   - Enum: All declared variants
   - Result[T,E]: Ok, Err (2 variants)
   - Option[T]: Some, None (2 variants)
3. For each pattern arm, track covered variants
4. Compute uncovered set = All variants - Covered variants
5. If uncovered set is non-empty: compile error

**Error Messages**:
```
error_prop_06_pattern_match.dingo:15:5: Pattern Match Error: non-exhaustive match
  Missing cases: Err
  Hint: Add missing pattern arms or use _ wildcard

error_prop_06_pattern_match.dingo:20:9: Pattern Match Warning: unreachable pattern
  Pattern 'Ok(x)' is shadowed by earlier '_' wildcard
  Hint: Remove unreachable pattern or reorder arms
```

#### Transpilation Strategy

**Input (Dingo)**:
```dingo
func handleResult(result: Result[User, Error]) -> string {
    match result {
        Ok(user) => "Found: ${user.name}",
        Err(e) => "Error: ${e.message}"
    }
}
```

**Preprocessor Output** (Valid Go with markers):
```go
func handleResult(result Result_User_Error) string {
    switch { /* DINGO_MATCH: Result_User_Error, exhaustive=[Ok,Err] */
    case __dingo_match_guard_0(result): /* Ok(user) */
        user := __dingo_extract_ok_0(result)
        return "Found: " + user.name
    case __dingo_match_guard_1(result): /* Err(e) */
        e := __dingo_extract_err_0(result)
        return "Error: " + e.message
    }
}
```

**Plugin Output** (After AST transformation):
```go
func handleResult(result Result_User_Error) string {
    switch {
    case result.tag == ResultTag_Ok:
        user := *result.ok_0
        return "Found: " + user.name
    case result.tag == ResultTag_Err:
        e := result.err_0
        return "Error: " + e.message
    default:
        panic("unreachable: match expression is exhaustive")
    }
}
```

**Key Design Decisions**:
- Use Go's `switch` (not `switch value`) for flexibility
- Tag-based dispatch for sum types
- Inject `default: panic()` for exhaustive matches (safety net)
- Marker comments preserve pattern info through preprocessing

---

### 2. Full go/types Integration

#### AST Parent Tracking

**Problem**: Current type inference lacks context (parent AST nodes not tracked)

**Solution**: Build parent map during AST construction

**Implementation**:

```go
// pkg/plugin/context.go (Enhanced)
type Context struct {
    FileSet   *token.FileSet
    File      *ast.File
    Logger    Logger

    // NEW: go/types integration
    TypesInfo *types.Info
    TypesConf *types.Config

    // NEW: AST parent tracking
    ParentMap map[ast.Node]ast.Node

    // Existing: Error handling
    Errors    []*errors.CompileError
}

// BuildParentMap constructs node → parent mapping
func (c *Context) BuildParentMap() {
    c.ParentMap = make(map[ast.Node]ast.Node)

    ast.Inspect(c.File, func(n ast.Node) bool {
        if n == nil {
            return false
        }

        // For each child of n, set n as parent
        for _, child := range childNodes(n) {
            c.ParentMap[child] = n
        }
        return true
    })
}

// GetParent returns the parent node
func (c *Context) GetParent(node ast.Node) ast.Node {
    return c.ParentMap[node]
}

// WalkParents walks up the parent chain
func (c *Context) WalkParents(node ast.Node, visitor func(ast.Node) bool) {
    current := node
    for current != nil {
        if !visitor(current) {
            break
        }
        current = c.ParentMap[current]
    }
}
```

**Integration Points**:
1. **Generator** calls `BuildParentMap()` after parsing
2. **Plugins** access parent via `ctx.GetParent(node)`
3. **Type inference** uses `WalkParents()` for context propagation

#### go/types Type Checker Integration

**Current State** (Phase 3):
- go/types invoked but types.Info not fully utilized
- Type inference service has SetTypesInfo() but limited usage

**Phase 4 Enhancement**:

```go
// pkg/generator/generator.go (Enhanced)
func (g *Generator) runTypeChecker(file *ast.File) (*types.Info, error) {
    info := &types.Info{
        Types:      make(map[ast.Expr]types.TypeAndValue),
        Defs:       make(map[*ast.Ident]types.Object),
        Uses:       make(map[*ast.Ident]types.Object),
        Implicits:  make(map[ast.Node]types.Object),
        Selections: make(map[*ast.SelectorExpr]*types.Selection),
        Scopes:     make(map[ast.Node]*types.Scope),
    }

    conf := types.Config{
        Importer: importer.Default(),
        Error: func(err error) {
            // Collect type errors but don't fail
            // Some Dingo constructs may not type-check before transformation
            g.logger.Debug("Type checker: %v", err)
        },
    }

    // Type-check the file
    pkg, err := conf.Check("main", g.fset, []*ast.File{file}, info)
    if err != nil {
        // Non-fatal: we got partial info even if check failed
        g.logger.Warn("Type checking incomplete: %v", err)
    }

    // Store in pipeline context
    g.pipeline.Ctx.TypesInfo = info
    g.pipeline.Ctx.TypesConf = &conf

    return info, nil
}
```

**Usage Pattern**:

```go
// In plugins
func (p *ResultTypePlugin) Transform(node ast.Node) (ast.Node, error) {
    return astutil.Apply(node, func(cursor *astutil.Cursor) bool {
        if call, ok := cursor.Node().(*ast.CallExpr); ok {
            // Use go/types to get actual type
            if tv, ok := p.ctx.TypesInfo.Types[call.Args[0]]; ok {
                okType := tv.Type
                // Now we know the ACTUAL type, not inferred!
            }
        }
        return true
    }, nil)
}
```

---

### 3. None Constant Context Inference

#### Problem Statement

**Currently**: `None` without type context fails
```dingo
x := None  // ERROR: Cannot infer type
```

**Goal**: Infer type from surrounding context
```dingo
func getUser(): Option[User] {
    return None  // ✅ Infers Option[User] from return type
}

let x: Option[int] = None  // ✅ Infers from variable type
process(None)  // ✅ Infers from parameter type (if process(opt: Option[T]))
```

#### Context Inference Algorithm

**Step 1: Find None usage**
```go
func (p *NoneContextPlugin) Process(file *ast.File) error {
    ast.Inspect(file, func(n ast.Node) bool {
        if ident, ok := n.(*ast.Ident); ok && ident.Name == "None" {
            p.noneNodes = append(p.noneNodes, ident)
        }
        return true
    })
    return nil
}
```

**Step 2: Walk parent chain to find context**
```go
func (p *NoneContextPlugin) inferNoneType(noneIdent *ast.Ident) (types.Type, error) {
    var expectedType types.Type

    p.ctx.WalkParents(noneIdent, func(parent ast.Node) bool {
        switch p := parent.(type) {
        case *ast.ReturnStmt:
            // Context: return statement
            // Find enclosing function and get return type
            expectedType = p.findReturnType(noneIdent)
            return false  // Stop walking

        case *ast.AssignStmt:
            // Context: assignment
            // Get variable type from LHS
            expectedType = p.findAssignmentType(noneIdent, p)
            return false

        case *ast.CallExpr:
            // Context: function call argument
            // Get parameter type from function signature
            expectedType = p.findParameterType(noneIdent, p)
            return false
        }
        return true  // Keep walking
    })

    if expectedType != nil {
        return expectedType, nil
    }

    return nil, fmt.Errorf("cannot infer type for None")
}
```

**Step 3: Extract Option[T] parameter**
```go
func (p *NoneContextPlugin) extractOptionType(typ types.Type) (types.Type, bool) {
    // Check if type is Option_T
    if named, ok := typ.(*types.Named); ok {
        typeName := named.Obj().Name()
        if strings.HasPrefix(typeName, "Option_") {
            // Extract T from Option_T
            return p.typeInference.GetOptionTypeParam(typeName)
        }
    }
    return nil, false
}
```

**Step 4: Transform None to typed value**
```go
func (p *NoneContextPlugin) Transform(node ast.Node) (ast.Node, error) {
    return astutil.Apply(node, func(cursor *astutil.Cursor) bool {
        if ident, ok := cursor.Node().(*ast.Ident); ok && ident.Name == "None" {
            // Infer type from context
            optionType, err := p.inferNoneType(ident)
            if err != nil {
                p.ctx.AddError(errors.NewTypeInferenceError(
                    fmt.Sprintf("cannot infer type for None: %v", err),
                    ident.Pos(),
                    "Add explicit type annotation: let x: Option[YourType] = None",
                ))
                return true
            }

            // Replace with typed Option value
            typeName := p.typeInference.TypeToString(optionType)
            replacement := &ast.CompositeLit{
                Type: &ast.Ident{Name: typeName},
                Elts: []ast.Expr{
                    &ast.KeyValueExpr{
                        Key:   &ast.Ident{Name: "isSet"},
                        Value: &ast.Ident{Name: "false"},
                    },
                },
            }
            cursor.Replace(replacement)
        }
        return true
    }, nil)
}
```

---

### 4. Enhanced Error Messages

#### Error Infrastructure Enhancements

**Goal**: Move from generic errors to actionable diagnostics

**Current** (Phase 3):
```
Type Inference Error: cannot infer type for expression: None
  Hint: Try providing an explicit type annotation
```

**Enhanced** (Phase 4):
```
error_prop_06_pattern_match.dingo:23:12: Type Inference Error: cannot infer type for None
  Context: None appears in return statement
  Expected: Option[T] where T can be inferred from function signature
  Found: No enclosing function or function has no return type

  Suggestion 1: Add return type to function:
    func processUser() -> Option[User] {
                      ^^^^^^^^^^^^^^^^^^^

  Suggestion 2: Use explicit type annotation:
    let result: Option[User] = None
               ^^^^^^^^^^^^^^

  See: https://dingolang.com/docs/errors#type-inference-none
```

#### Implementation

**Enhanced CompileError**:
```go
// pkg/errors/error.go (Enhanced)
type CompileError struct {
    Message     string
    Location    token.Pos
    Category    ErrorCategory

    // NEW: Context and suggestions
    Context     string              // What was the code trying to do?
    Expected    string              // What type was expected?
    Found       string              // What did we get instead?
    Suggestions []ErrorSuggestion   // Actionable fixes
    DocsURL     string              // Link to documentation
}

type ErrorSuggestion struct {
    Description string              // Human-readable description
    Code        string              // Example code fix
    Location    token.Pos           // Where to apply fix (optional)
}

func (e *CompileError) FormatWithPosition(fset *token.FileSet) string {
    var buf strings.Builder

    // Header: file:line:col: category: message
    pos := fset.Position(e.Location)
    fmt.Fprintf(&buf, "%s:%d:%d: %s: %s\n",
        pos.Filename, pos.Line, pos.Column,
        e.categoryString(), e.Message)

    // Context
    if e.Context != "" {
        fmt.Fprintf(&buf, "  Context: %s\n", e.Context)
    }

    // Expected vs Found
    if e.Expected != "" || e.Found != "" {
        if e.Expected != "" {
            fmt.Fprintf(&buf, "  Expected: %s\n", e.Expected)
        }
        if e.Found != "" {
            fmt.Fprintf(&buf, "  Found: %s\n", e.Found)
        }
        buf.WriteString("\n")
    }

    // Suggestions
    for i, sugg := range e.Suggestions {
        fmt.Fprintf(&buf, "  Suggestion %d: %s\n", i+1, sugg.Description)
        if sugg.Code != "" {
            // Indent code
            for _, line := range strings.Split(sugg.Code, "\n") {
                fmt.Fprintf(&buf, "    %s\n", line)
            }
        }
        buf.WriteString("\n")
    }

    // Documentation link
    if e.DocsURL != "" {
        fmt.Fprintf(&buf, "  See: %s\n", e.DocsURL)
    }

    return buf.String()
}
```

**Error Helper Functions**:
```go
// pkg/errors/pattern_match.go (NEW)

func NonExhaustiveMatch(matchPos token.Pos, scrutineeType string, missingCases []string) *CompileError {
    return &CompileError{
        Message:  "non-exhaustive match expression",
        Location: matchPos,
        Category: ErrorCategoryPatternMatch,
        Context:  fmt.Sprintf("Matching on type %s", scrutineeType),
        Expected: "All variants must be covered",
        Found:    fmt.Sprintf("Missing cases: %s", strings.Join(missingCases, ", ")),
        Suggestions: []ErrorSuggestion{
            {
                Description: "Add missing pattern arms",
                Code: strings.Join(missingCases, " => ...,\n    "),
            },
            {
                Description: "Use wildcard to handle remaining cases",
                Code: "_ => defaultValue",
            },
        },
        DocsURL: "https://dingolang.com/docs/errors#non-exhaustive-match",
    }
}

func UnreachablePattern(patternPos token.Pos, pattern string, shadowedBy string) *CompileError {
    return &CompileError{
        Message:  "unreachable pattern",
        Location: patternPos,
        Category: ErrorCategoryPatternMatch,
        Context:  fmt.Sprintf("Pattern '%s' can never match", pattern),
        Expected: "Patterns should be reachable",
        Found:    fmt.Sprintf("Pattern shadowed by earlier '%s'", shadowedBy),
        Suggestions: []ErrorSuggestion{
            {
                Description: "Remove unreachable pattern",
            },
            {
                Description: "Reorder patterns (specific before general)",
            },
        },
        DocsURL: "https://dingolang.com/docs/errors#unreachable-pattern",
    }
}
```

---

## Implementation Strategy

### Phase 4.1: MVP (Weeks 1-2)

**Goal**: Basic pattern matching + improved type inference

#### Week 1: Pattern Match Preprocessor + Plugin Foundation

**Tasks**:
1. Implement `PatternMatchProcessor` (preprocessor)
   - Parse `match expr { arms }` syntax
   - Generate switch skeleton with markers
   - Preserve pattern structure in comments

2. Implement `PatternMatchPlugin` (plugin)
   - Discovery: Find DINGO_MATCH markers
   - Parse pattern arms from markers
   - Extract scrutinee type from go/types

3. Build AST parent tracking
   - Implement `Context.BuildParentMap()`
   - Integrate into generator pipeline
   - Test with simple AST traversals

**Deliverables**:
- `pkg/preprocessor/pattern_match.go`
- `pkg/plugin/builtin/pattern_match.go`
- `pkg/plugin/context.go` (enhanced)
- Unit tests for preprocessor
- Golden test: `pattern_match_01_simple.dingo`

#### Week 2: Exhaustiveness Checking + None Context Inference

**Tasks**:
1. Implement exhaustiveness algorithm
   - Variant extraction (Result, Option, Enum)
   - Coverage tracking
   - Missing case detection

2. Implement `NoneContextPlugin`
   - Discovery: Find None identifiers
   - Context walking (return, assignment, call)
   - Type inference from context

3. Enhance error messages
   - Implement ErrorSuggestion system
   - Add pattern match error helpers
   - Add type inference error helpers

**Deliverables**:
- Exhaustiveness checking in PatternMatchPlugin
- `pkg/plugin/builtin/none_context.go`
- `pkg/errors/pattern_match.go` (new)
- Enhanced `pkg/errors/type_inference.go`
- Golden tests: `pattern_match_02_exhaustive.dingo`, `option_06_none_inference.dingo`

### Phase 4.2: Advanced Patterns (Week 3)

**Goal**: Guards, nested patterns, struct destructuring

**Tasks**:
1. Guard support in patterns
   - Parse `Pattern if condition` syntax
   - Generate conditional checks
   - Exhaustiveness with guards

2. Nested pattern matching
   - `Ok(Some(x))` patterns
   - Recursive pattern compilation
   - Type checking for nested patterns

3. Struct destructuring (basic)
   - `User{name, age}` patterns
   - Field extraction
   - Partial matching with `..`

**Deliverables**:
- Guard support in preprocessor + plugin
- Nested pattern compilation
- `pattern_match_03_guards.dingo`, `pattern_match_04_nested.dingo`

### Phase 4.3: Polish & Integration (Week 4)

**Goal**: Documentation, testing, integration

**Tasks**:
1. Comprehensive test suite
   - Golden tests for all pattern types
   - Edge cases (empty match, single arm)
   - Error message validation

2. Documentation
   - Update `features/pattern-matching.md`
   - Add `docs/errors.md` (error catalog)
   - Update `ARCHITECTURE.md`

3. Performance optimization
   - Benchmark pattern compilation
   - Optimize parent map construction
   - Cache type inference results

**Deliverables**:
- 10+ golden tests for pattern matching
- Complete error catalog documentation
- Performance benchmarks
- Updated project documentation

---

## File/Package Structure

### New Files

```
pkg/
├── preprocessor/
│   └── pattern_match.go          # NEW: match → switch preprocessor
│
├── plugin/
│   ├── context.go                 # ENHANCED: Add ParentMap, WalkParents
│   └── builtin/
│       ├── pattern_match.go       # NEW: Exhaustiveness checker
│       └── none_context.go        # NEW: None type inference
│
└── errors/
    ├── pattern_match.go           # NEW: Pattern match errors
    └── type_inference.go          # ENHANCED: Better error messages

tests/golden/
├── pattern_match_01_simple.dingo
├── pattern_match_02_exhaustive.dingo
├── pattern_match_03_guards.dingo
├── pattern_match_04_nested.dingo
├── option_06_none_inference.dingo
└── ...
```

### Modified Files

```
pkg/
├── generator/
│   └── generator.go               # ENHANCED: Call BuildParentMap, runTypeChecker
│
└── plugin/
    └── builtin/
        ├── result_type.go          # ENHANCED: Use go/types more extensively
        ├── option_type.go          # ENHANCED: Use go/types more extensively
        └── type_inference.go       # ENHANCED: Add parent-aware inference
```

---

## Integration Points with Existing Code

### With Result[T,E] and Option[T]

**Pattern matching integrates seamlessly**:
```dingo
// Existing: Result type works
result := Ok(42)

// NEW: Pattern match on Result
match result {
    Ok(x) => println("Got ${x}"),
    Err(e) => println("Error: ${e}")
}
```

**Plugin coordination**:
1. ResultTypePlugin runs first (injects Result types)
2. PatternMatchPlugin runs second (uses injected types)
3. Type inference shared via Context.TypesInfo

### With Enum Preprocessor

**Enums are first-class match targets**:
```dingo
enum Status { Pending, Approved, Rejected }

match status {
    Pending => "waiting",
    Approved => "done",
    Rejected => "failed"
}
```

**Preprocessor coordination**:
1. EnumProcessor runs before PatternMatchProcessor
2. Enum types available when pattern match processes
3. Tag-based dispatch works uniformly

### With Error Propagation (`?`)

**Pattern matching complements `?` operator**:
```dingo
func process(): Result[int, Error] {
    data := readFile()?  // ? operator

    match parseData(data) {  // Pattern match
        Ok(value) => Ok(value * 2),
        Err(e) => Err(e)
    }
}
```

No conflicts - different concerns:
- `?` = early return on error
- `match` = explicit case handling

---

## Testing Strategy

### Unit Tests

**Preprocessor Tests** (`pkg/preprocessor/pattern_match_test.go`):
- Basic match parsing
- Guard extraction
- Pattern binding preservation
- Marker comment generation
- Edge cases (empty match, single arm)

**Plugin Tests** (`pkg/plugin/builtin/pattern_match_test.go`):
- Exhaustiveness checking algorithm
- Variant extraction (Result, Option, Enum)
- Coverage tracking
- Error generation for missing cases

**Context Tests** (`pkg/plugin/context_test.go`):
- Parent map construction
- WalkParents traversal
- Context propagation

**None Inference Tests** (`pkg/plugin/builtin/none_context_test.go`):
- Return statement context
- Assignment context
- Function call context
- Nested context (e.g., `return Ok(None)`)

### Integration Tests

**Golden Tests** (End-to-end):
1. `pattern_match_01_simple.dingo` - Basic Result/Option matching
2. `pattern_match_02_exhaustive.dingo` - Exhaustiveness errors
3. `pattern_match_03_guards.dingo` - Guard conditions
4. `pattern_match_04_nested.dingo` - Nested patterns
5. `pattern_match_05_enum.dingo` - Enum matching
6. `option_06_none_inference.dingo` - None context inference
7. `error_prop_07_match_combo.dingo` - Combined `?` + `match`

**Test Coverage Goals**:
- Preprocessor: >90% coverage
- Plugins: >85% coverage
- Error paths: 100% coverage (all error types tested)

### Error Message Tests

**Validation**:
- Snapshot testing for error messages
- Verify suggestion quality
- Check documentation links
- Validate error categories

---

## Performance Considerations

### AST Parent Map Construction

**Cost**: O(N) where N = number of AST nodes

**Optimization**:
- Build once, reuse across plugins
- Lazy construction (only if plugins need it)
- Memory: ~8 bytes per node (pointer map)

**Benchmark Target**: <10ms for 10K node AST

### Exhaustiveness Checking

**Cost**: O(V * A) where V = variants, A = pattern arms

**Optimization**:
- Early exit on wildcard
- Bitset for variant coverage (not string set)
- Cache variant lists per type

**Benchmark Target**: <1ms for typical match (3-5 arms)

### go/types Integration

**Cost**: Type checking is expensive (10-100ms)

**Mitigation**:
- Already running in Phase 3 (no new cost)
- Store types.Info for reuse
- Partial type checking (don't fail on Dingo constructs)

**Benchmark Target**: No regression vs Phase 3

---

## Risks and Mitigations

### Risk 1: Exhaustiveness Algorithm Complexity

**Risk**: Complex type hierarchies (nested Result[Option[T], E]) may be hard to analyze

**Mitigation**:
- Start with flat types (Result, Option, Enum only)
- Defer nested exhaustiveness to Phase 5
- Use go/types to get concrete variant lists

**Fallback**: Require `_` wildcard for complex types in Phase 4

### Risk 2: None Inference Ambiguity

**Risk**: Multiple valid contexts (e.g., `x := None; return x` - is it return or assignment?)

**Mitigation**:
- Prefer closest context (assignment over return)
- Document precedence rules clearly
- Error if inference is ambiguous (require explicit type)

**Fallback**: Be conservative - error on ambiguity, force explicit types

### Risk 3: Pattern Match Syntax Conflicts

**Risk**: Dingo syntax `match value { }` might conflict with future Go syntax

**Mitigation**:
- Use `match` keyword (not used in Go)
- Preprocessor transforms before Go parser sees it
- Monitor Go proposals for conflicts

**Fallback**: Add configuration flag to disable pattern matching if conflict arises

### Risk 4: AST Parent Map Memory Usage

**Risk**: Large files (100K+ nodes) could use excessive memory

**Mitigation**:
- Use weak references if Go adds them (future)
- Clear parent map after transformation
- Make parent tracking opt-in per plugin

**Fallback**: Disable parent tracking, fall back to tree walking

---

## Success Criteria

### Functional Requirements

- ✅ Pattern matching compiles for Result, Option, Enum
- ✅ Exhaustiveness checking catches missing cases
- ✅ Guards work correctly (`Pattern if condition`)
- ✅ None infers type from return/assignment/call context
- ✅ Error messages include actionable suggestions
- ✅ All golden tests pass

### Non-Functional Requirements

- ✅ Compilation time <5% slower than Phase 3
- ✅ Generated code is idiomatic Go
- ✅ No false positives in exhaustiveness checking
- ✅ Error messages are beginner-friendly
- ✅ Documentation is complete and accurate

### Quality Gates

**Before Phase 4.1 → 4.2**:
- 10+ pattern match golden tests passing
- Exhaustiveness checker has <5% false positive rate
- Error messages validated by external reviewers

**Before Phase 4.2 → 4.3**:
- Guards working in all test cases
- Nested patterns compile correctly
- Performance benchmarks meet targets

**Before Phase 4 → Phase 5**:
- All success criteria met
- Documentation reviewed and approved
- Community feedback on pattern matching collected

---

## Open Questions (see gaps.json)

See `gaps.json` for detailed questions requiring user input.

Key questions:
1. Pattern syntax preference (Rust-like vs Kotlin-like)
2. Exhaustiveness error strictness (error or warning?)
3. None inference fallback behavior
4. Integration with future Go generics
5. Performance vs completeness tradeoffs

---

## Next Steps

**Immediate** (After plan approval):
1. Create task breakdown for Week 1
2. Implement PatternMatchProcessor skeleton
3. Set up golden test framework for pattern matching
4. Begin AST parent map implementation

**After Phase 4.1 Complete**:
1. Gather feedback on basic pattern matching
2. Evaluate exhaustiveness checking accuracy
3. Decide on advanced pattern priorities (guards vs nested vs struct)

**After Phase 4 Complete**:
1. Plan Phase 5 (Language Server implementation)
2. Document lessons learned
3. Publish Phase 4 blog post / docs

---

## References

### External Resources
- Rust Pattern Matching: https://doc.rust-lang.org/book/ch18-00-patterns.html
- Kotlin Sealed Classes: https://kotlinlang.org/docs/sealed-classes.html
- Swift Pattern Matching: https://docs.swift.org/swift-book/documentation/the-swift-programming-language/patterns/
- OCaml Pattern Matching (theory): http://ocaml.org/manual/patterns.html

### Internal Documentation
- `features/pattern-matching.md` - Feature specification
- `ARCHITECTURE.md` - Current architecture
- `ai-docs/CRITICAL-2-FIX-SUMMARY.md` - Phase 3 issues (A4, A5)

### Go Proposals
- Proposal #45346 - Pattern matching for sum types
- Proposal #19412 - Sum types (996+ 👍)

---

**END OF PLAN**
