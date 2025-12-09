# Phase 4 Final Implementation Plan - Pattern Matching & Enhanced Type Inference

**Status**: Final Architecture Plan (Approved User Decisions)
**Date**: 2025-11-18
**Phase**: 4 of 6 (Implementation Roadmap)

---

## Executive Summary

Phase 4 implements pattern matching (`match` expressions) with dual syntax support (Rust + Swift), exhaustiveness checking, and enhanced type inference with full go/types integration. This phase builds on Phase 3's Result/Option types and enables type-safe, exhaustive destructuring of sum types.

**Primary Goals**:
1. Pattern matching with configurable syntax (Rust-like default, Swift-like optional)
2. Strict exhaustiveness checking (compile errors for non-exhaustive matches)
3. Context-aware match type inference (expression vs statement)
4. Conservative None constant inference (error on ambiguity)
5. Dual syntax support via dingo.toml configuration
6. Enhanced error messages with source snippets (rustc-style)
7. Full go/types integration with AST parent tracking
8. Tuple destructuring support (Phase 4.2)

**Timeline**: 4 weeks total
- Phase 4.1 (weeks 1-2): MVP - Basic pattern matching for Result/Option/Enum
- Phase 4.2 (weeks 3-4): Advanced - Guards, nested patterns, tuples, struct destructuring

---

## User Decisions (Final)

### ✅ Decision 1: Dual Syntax Support
**Configuration**: `dingo.toml`
```toml
[match]
syntax = "rust"  # or "swift", default: "rust"
```

**Rust-like (default)**:
```dingo
match result {
    Ok(x) => x * 2,
    Err(e) => 0
}
```

**Swift-like**:
```dingo
switch result {
    case .ok(let x): return x * 2
    case .err(let e): return 0
}
```

**Implementation**: Two preprocessor processors (RustMatchProcessor, SwiftMatchProcessor), both generate same AST markers.

---

### ✅ Decision 2: Strict Exhaustiveness Checking
**Policy**: Compile ERROR (non-exhaustive matches rejected)

**Example**:
```dingo
// ERROR: non-exhaustive match
match result {
    Ok(x) => x
}

// Fix: add wildcard or all cases
match result {
    Ok(x) => x,
    _ => 0
}
```

**Implementation**: PatternMatchPlugin validates exhaustiveness during Transform phase, emits compile errors for missing variants.

---

### ✅ Decision 3: Context-Aware Match Type Inference
**Policy**: Expression if assigned/returned, statement otherwise

**Expression mode** (type-checked):
```dingo
let x = match result {
    Ok(v) => v * 2,  // Must return int
    Err(_) => 0      // Must return int
}
```

**Statement mode** (no type check):
```dingo
match result {
    Ok(v) => println(v),
    Err(e) => println(e)
}
```

**Implementation**: AST parent tracking detects usage context, go/types enforces type compatibility in expression mode.

---

### ✅ Decision 4: Conservative None Inference
**Policy**: Error and require explicit type (safe default)

**Valid contexts** (auto-inferred):
```dingo
return None  // OK - from function signature
processAge(None)  // OK - from parameter type
User{ age: None }  // OK - from field type
match { _ => None }  // OK - from other match arms
```

**Invalid** (error):
```dingo
let x = None  // ERROR: cannot infer type for None
// Fix: let x: Option[int] = None
```

**Implementation**: NoneContextPlugin walks AST parent chain, infers from return/assignment/call/field context. Errors on ambiguity.

---

### ✅ Decision 5: Tuple Destructuring
**Policy**: Include in Phase 4.2 (weeks 3-4)

**Example**:
```dingo
match (x, y) {
    (0, 0) => "origin",
    (0, _) => "on y-axis",
    (_, 0) => "on x-axis",
    _ => "other"
}
```

**Timeline**: Requires tuple syntax support, implemented in Phase 4.2.

---

### ✅ Decision 6: AST Parent Map
**Policy**: Build unconditionally (simple, predictable)

**Cost**: +5-10ms per file (acceptable overhead)
**Rationale**: Required for context-aware inference, simplifies plugin API.

---

### ✅ Decision 7: Enhanced Error Messages
**Policy**: Show source snippets (rustc-style)

**Example**:
```
error: non-exhaustive match
  --> example.dingo:23:5
   |
23 | match result {
24 |     Ok(x) => processX(x)
   |     ^^^^^^^^^^^^^^^^^^^ missing Err case
   |
help: add a wildcard arm: `_ => ...`
```

**Requires**: Source file reading, line/column tracking, error suggestion system.

---

## Architecture Overview

### High-Level Design

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
│         STAGE 1: Preprocessor (Configurable Syntax)         │
├─────────────────────────────────────────────────────────────┤
│  ConfigLoader:                                               │
│    - Read dingo.toml ([match] section)                      │
│    - Determine syntax: "rust" (default) or "swift"          │
│                                                              │
│  RustMatchProcessor OR SwiftMatchProcessor:                  │
│    - Parse match/switch expression                          │
│    - Validate pattern syntax (destructuring, guards)        │
│    - Generate switch statement skeleton                     │
│    - Preserve pattern bindings as marker comments           │
│    - Track exhaustiveness requirements                      │
├─────────────────────────────────────────────────────────────┤
│  Output: Valid Go switch with DINGO_MATCH markers           │
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
│  Context Enhancement:                                        │
│    - BuildParentMap() - O(N) parent tracking                │
│    - runTypeChecker() - Full go/types integration           │
│    - Store types.Info in pipeline context                   │
│                                                              │
│  PatternMatchPlugin (NEW):                                   │
│    Phase 1 - Discovery:                                      │
│      - Find DINGO_MATCH markers in AST                      │
│      - Extract scrutinee type from go/types                 │
│      - Collect pattern arms from markers                    │
│      - Detect expression vs statement mode via parent       │
│    Phase 2 - Exhaustiveness Check:                          │
│      - Determine all variants (Result, Option, Enum)        │
│      - Track covered variants per arm                       │
│      - Compute uncovered = All - Covered                    │
│      - ERROR if uncovered set is non-empty                  │
│    Phase 3 - Pattern Compilation:                           │
│      - Generate case conditions (tag checks)                │
│      - Extract pattern bindings                             │
│      - Handle guards (if conditions)                        │
│      - Add default: panic() for exhaustive matches          │
│      - Type-check arms in expression mode                   │
│                                                              │
│  NoneContextPlugin (NEW):                                    │
│    Phase 1 - Discovery:                                      │
│      - Find DINGO_NONE_CONTEXT markers                      │
│      - Walk parent chain to find type context               │
│    Phase 2 - Type Inference:                                │
│      - Check contexts (return > assign > call > field)      │
│      - Extract expected type from go/types                  │
│      - ERROR if no valid context found                      │
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

### 1. Pattern Matching (`match`/`switch` Expression)

#### Dual Syntax Support

**Rust-like Syntax** (default, consistent with Result/Option):
```dingo
fn processResult(result: Result[int, string]) -> int {
    match result {
        Ok(value) => {
            println("Success:", value)
            return value * 2
        },
        Err(error) => {
            println("Error:", error)
            return 0
        }
    }
}
```

**Swift-like Syntax** (familiar to Swift/Go devs):
```dingo
fn processResult(result: Result[int, string]) -> int {
    switch result {
        case .ok(let value):
            println("Success:", value)
            return value * 2
        case .err(let error):
            println("Error:", error)
            return 0
    }
}
```

**Configuration** (`dingo.toml`):
```toml
[match]
syntax = "rust"  # Options: "rust", "swift" (default: "rust")
```

**Implementation**:
1. ConfigLoader reads `dingo.toml` before preprocessing
2. Dispatch to RustMatchProcessor or SwiftMatchProcessor
3. Both generate same AST markers: `/* DINGO_MATCH: Type, patterns=[...] */`
4. PatternMatchPlugin is syntax-agnostic (processes markers)

---

#### Supported Patterns (Phase 4.1 MVP)

**Enum variant matching**:
```dingo
enum Color { Red, Green, Blue }

match color {
    Red => "stop",
    Green => "go",
    Blue => "caution"
}
```

**Result destructuring**:
```dingo
match readFile(path) {
    Ok(data) => processData(data),
    Err(e) => handleError(e)
}
```

**Option destructuring**:
```dingo
match findUser(id) {
    Some(user) => user.name,
    None => "Unknown"
}
```

**Wildcard (catch-all)**:
```dingo
match status {
    Active(id) => handleActive(id),
    _ => handleOther()  // Catches Pending, Completed
}
```

**Guards** (conditionals):
```dingo
match value {
    Some(x) if x > 10 => "large",
    Some(x) if x > 0 => "small",
    Some(_) => "non-positive",
    None => "none"
}
```

---

#### Exhaustiveness Checking Algorithm

**Step 1**: Determine scrutinee type from go/types
```go
scrutineeType := ctx.TypesInfo.TypeOf(scrutineeExpr)
```

**Step 2**: Extract all possible variants
- **Enum**: All declared variants from enum definition
- **Result[T,E]**: `Ok`, `Err` (2 variants)
- **Option[T]**: `Some`, `None` (2 variants)

**Step 3**: Track covered variants per pattern arm
```go
coveredVariants := make(map[string]bool)
for _, arm := range patternArms {
    if arm.Pattern == "_" {
        // Wildcard covers all remaining
        coveredVariants = allVariants
        break
    }
    coveredVariants[arm.Pattern] = true
}
```

**Step 4**: Compute uncovered set
```go
uncovered := []string{}
for _, variant := range allVariants {
    if !coveredVariants[variant] {
        uncovered = append(uncovered, variant)
    }
}
```

**Step 5**: Error if uncovered set is non-empty
```go
if len(uncovered) > 0 {
    return NonExhaustiveMatchError(matchPos, scrutineeType, uncovered)
}
```

**Error Message**:
```
error: non-exhaustive match
  --> example.dingo:23:5
   |
23 | match result {
24 |     Ok(x) => processX(x)
   | ^^^^^^^^^^^^^^^^^^^^^^^^ missing Err case
   |
help: add missing pattern arm:
    Err(_) => defaultValue

help: or add a wildcard arm:
    _ => defaultValue
```

---

#### Expression vs Statement Mode

**Detection** (via AST parent tracking):
```go
func (p *PatternMatchPlugin) isExpressionMode(matchNode ast.Node) bool {
    parent := p.ctx.GetParent(matchNode)
    switch parent.(type) {
    case *ast.AssignStmt:  // let x = match { ... }
        return true
    case *ast.ReturnStmt:  // return match { ... }
        return true
    case *ast.CallExpr:    // foo(match { ... })
        return true
    default:
        return false
    }
}
```

**Expression Mode**:
- All pattern arms must return same type
- Type-checked via go/types
- Transpiles to Go expression (IIFE pattern if needed)

**Statement Mode**:
- No return type required
- Pattern arms can be any statements
- Transpiles to Go switch statement

---

#### Transpilation Strategy

**Input (Dingo)**:
```dingo
fn handleResult(result: Result[User, Error]) -> string {
    match result {
        Ok(user) => "Found: ${user.name}",
        Err(e) => "Error: ${e.message}"
    }
}
```

**Preprocessor Output** (Valid Go with markers):
```go
func handleResult(result Result_User_Error) string {
    switch { /* DINGO_MATCH: Result_User_Error, mode=expression, exhaustive=[Ok,Err] */
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

**Key Design**:
- Tag-based dispatch for sum types
- Inject `default: panic()` for exhaustive matches (safety net)
- Marker comments preserve pattern info through preprocessing
- Expression mode ensures all arms return same type

---

### 2. Configuration System (dingo.toml)

**New Component**: `pkg/config/` package

**File Structure**:
```toml
# dingo.toml (project root)

[match]
syntax = "rust"  # Options: "rust", "swift"

[compiler]
strict_exhaustiveness = true  # Future: allow warnings instead

[codegen]
source_maps = true
debug_mode = false
```

**Implementation**:
```go
// pkg/config/config.go

package config

import (
    "github.com/BurntSushi/toml"
)

type Config struct {
    Match    MatchConfig    `toml:"match"`
    Compiler CompilerConfig `toml:"compiler"`
    Codegen  CodegenConfig  `toml:"codegen"`
}

type MatchConfig struct {
    Syntax string `toml:"syntax"`  // "rust" or "swift"
}

type CompilerConfig struct {
    StrictExhaustiveness bool `toml:"strict_exhaustiveness"`
}

type CodegenConfig struct {
    SourceMaps bool `toml:"source_maps"`
    DebugMode  bool `toml:"debug_mode"`
}

// Load reads dingo.toml from project root
func Load(projectRoot string) (*Config, error) {
    path := filepath.Join(projectRoot, "dingo.toml")

    // Default config
    cfg := &Config{
        Match: MatchConfig{
            Syntax: "rust",  // Default
        },
        Compiler: CompilerConfig{
            StrictExhaustiveness: true,
        },
        Codegen: CodegenConfig{
            SourceMaps: true,
            DebugMode:  false,
        },
    }

    // Load if exists
    if _, err := os.Stat(path); err == nil {
        if _, err := toml.DecodeFile(path, cfg); err != nil {
            return nil, fmt.Errorf("parsing dingo.toml: %w", err)
        }
    }

    // Validate
    if err := cfg.Validate(); err != nil {
        return nil, err
    }

    return cfg, nil
}

func (c *Config) Validate() error {
    if c.Match.Syntax != "rust" && c.Match.Syntax != "swift" {
        return fmt.Errorf("invalid match syntax: %q (must be 'rust' or 'swift')", c.Match.Syntax)
    }
    return nil
}
```

**Integration**:
```go
// pkg/generator/generator.go

func (g *Generator) Generate(dingoCode string) (string, error) {
    // Load config early
    cfg, err := config.Load(g.projectRoot)
    if err != nil {
        return "", err
    }

    // Pass to preprocessor chain
    preprocessed, err := g.runPreprocessors(dingoCode, cfg)
    // ...
}

// pkg/preprocessor/preprocessor.go

func RunPreprocessors(code string, cfg *config.Config) (string, error) {
    // Dispatch based on syntax
    if cfg.Match.Syntax == "rust" {
        code = RustMatchProcessor.Process(code)
    } else {
        code = SwiftMatchProcessor.Process(code)
    }

    // Other preprocessors...
    return code, nil
}
```

---

### 3. Full go/types Integration

#### AST Parent Tracking

**Problem**: Current type inference lacks context (can't walk up AST tree)

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
        for _, child := range getChildNodes(n) {
            c.ParentMap[child] = n
        }
        return true
    })
}

// GetParent returns the immediate parent node
func (c *Context) GetParent(node ast.Node) ast.Node {
    return c.ParentMap[node]
}

// WalkParents walks up the parent chain until visitor returns false
func (c *Context) WalkParents(node ast.Node, visitor func(ast.Node) bool) {
    current := node
    for current != nil {
        if !visitor(current) {
            break
        }
        current = c.ParentMap[current]
    }
}

// getChildNodes extracts direct children of a node
func getChildNodes(n ast.Node) []ast.Node {
    var children []ast.Node

    ast.Inspect(n, func(child ast.Node) bool {
        if child == n {
            return true  // Skip self
        }
        if child != nil {
            children = append(children, child)
            return false  // Don't descend further
        }
        return false
    })

    return children
}
```

**Integration Points**:
1. Generator calls `BuildParentMap()` after parsing
2. Plugins access parent via `ctx.GetParent(node)`
3. Type inference uses `WalkParents()` for context propagation

---

#### Enhanced go/types Type Checker

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

func (g *Generator) Generate(dingoCode string) (string, error) {
    // ... preprocessing ...

    file, err := parser.ParseFile(g.fset, filename, preprocessed, 0)
    if err != nil {
        return "", err
    }

    // Build parent map
    g.pipeline.Ctx.BuildParentMap()

    // Run type checker
    typesInfo, err := g.runTypeChecker(file)
    if err != nil {
        g.logger.Warn("Type checking failed: %v", err)
    }

    // Run plugin pipeline (now has parent map + types info)
    transformedFile, err := g.pipeline.Run(file)
    // ...
}
```

**Usage Pattern in Plugins**:
```go
// In PatternMatchPlugin
func (p *PatternMatchPlugin) Transform(node ast.Node) (ast.Node, error) {
    return astutil.Apply(node, func(cursor *astutil.Cursor) bool {
        if switchStmt, ok := cursor.Node().(*ast.SwitchStmt); ok {
            // Use go/types to get scrutinee type
            if tv, ok := p.ctx.TypesInfo.Types[scrutineeExpr]; ok {
                scrutineeType := tv.Type
                // Now we know the ACTUAL type, can extract variants!
                variants := p.extractVariants(scrutineeType)
                // Check exhaustiveness...
            }
        }
        return true
    }, nil)
}
```

---

### 4. None Constant Context Inference

**Goal**: Infer None type from surrounding context, error on ambiguity

**Valid Contexts**:
```dingo
// Return statement
fn getAge() -> Option[int] {
    return None  // ✅ Infers Option[int] from return type
}

// Function call
processAge(None)  // ✅ Infers from parameter type

// Struct field
let user = User{ age: None }  // ✅ Infers from field type

// Match arm (expression mode)
match status {
    Active(id) => Some(id),
    Inactive => None  // ✅ Infers Option[int] from Some(id) arm
}

// Assignment to typed variable
let x: Option[int]
x = None  // ✅ Type already known
```

**Invalid Contexts** (error):
```dingo
let x = None  // ❌ ERROR: cannot infer type for None
// Fix: let x: Option[int] = None
```

**Implementation**:
```go
// pkg/plugin/builtin/none_context.go (NEW)

type NoneContextPlugin struct {
    ctx           *plugin.Context
    typeInference *TypeInferenceService
    noneNodes     []*ast.Ident
}

// Discovery phase
func (p *NoneContextPlugin) Process(file *ast.File) error {
    ast.Inspect(file, func(n ast.Node) bool {
        if ident, ok := n.(*ast.Ident); ok && ident.Name == "None" {
            p.noneNodes = append(p.noneNodes, ident)
        }
        return true
    })
    return nil
}

// Transform phase
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

// inferNoneType walks parent chain to find context
func (p *NoneContextPlugin) inferNoneType(noneIdent *ast.Ident) (types.Type, error) {
    var expectedType types.Type

    p.ctx.WalkParents(noneIdent, func(parent ast.Node) bool {
        switch p := parent.(type) {
        case *ast.ReturnStmt:
            // Context: return statement
            expectedType = p.findReturnType(noneIdent)
            return false  // Stop walking

        case *ast.AssignStmt:
            // Context: assignment
            expectedType = p.findAssignmentType(noneIdent, p)
            return false

        case *ast.CallExpr:
            // Context: function call argument
            expectedType = p.findParameterType(noneIdent, p)
            return false

        case *ast.CompositeLit:
            // Context: struct field
            expectedType = p.findFieldType(noneIdent, p)
            return false
        }
        return true  // Keep walking
    })

    if expectedType != nil {
        // Extract Option[T] → T
        if tParam, ok := p.extractOptionType(expectedType); ok {
            return expectedType, nil
        }
        return nil, fmt.Errorf("expected Option[T], got %v", expectedType)
    }

    return nil, fmt.Errorf("no valid type context found")
}

// findReturnType gets expected type from function return signature
func (p *NoneContextPlugin) findReturnType(noneIdent *ast.Ident) types.Type {
    // Walk up to find enclosing function
    var funcDecl *ast.FuncDecl
    p.ctx.WalkParents(noneIdent, func(parent ast.Node) bool {
        if fn, ok := parent.(*ast.FuncDecl); ok {
            funcDecl = fn
            return false
        }
        return true
    })

    if funcDecl == nil || funcDecl.Type.Results == nil {
        return nil
    }

    // Get return type from go/types
    funcObj := p.ctx.TypesInfo.Defs[funcDecl.Name]
    if funcObj == nil {
        return nil
    }

    funcType, ok := funcObj.Type().(*types.Signature)
    if !ok || funcType.Results().Len() == 0 {
        return nil
    }

    return funcType.Results().At(0).Type()
}

// findParameterType gets expected type from function signature
func (p *NoneContextPlugin) findParameterType(noneIdent *ast.Ident, callExpr *ast.CallExpr) types.Type {
    // Get function type from go/types
    funcExpr := callExpr.Fun
    tv, ok := p.ctx.TypesInfo.Types[funcExpr]
    if !ok {
        return nil
    }

    funcType, ok := tv.Type.(*types.Signature)
    if !ok {
        return nil
    }

    // Find argument index
    argIndex := -1
    for i, arg := range callExpr.Args {
        if arg == noneIdent {
            argIndex = i
            break
        }
    }

    if argIndex < 0 || argIndex >= funcType.Params().Len() {
        return nil
    }

    return funcType.Params().At(argIndex).Type()
}

// extractOptionType checks if type is Option_T and extracts T
func (p *NoneContextPlugin) extractOptionType(typ types.Type) (types.Type, bool) {
    if named, ok := typ.(*types.Named); ok {
        typeName := named.Obj().Name()
        if strings.HasPrefix(typeName, "Option_") {
            return typ, true  // Return full Option_T type
        }
    }
    return nil, false
}
```

**Error Message**:
```
error: cannot infer type for None constant
  --> example.dingo:42:12
   |
42 |     let x = None
   |             ^^^^ no type context available
   |
help: add explicit type annotation:
    let x: Option[YourType] = None
```

---

### 5. Enhanced Error Messages with Source Context

**Goal**: rustc-style error messages with source snippets, actionable suggestions

**Error Infrastructure**:
```go
// pkg/errors/error.go (Enhanced)

type CompileError struct {
    Message     string
    Location    token.Pos
    Category    ErrorCategory

    // NEW: Context and suggestions
    Context     string              // What was the code trying to do?
    Expected    string              // What type/pattern was expected?
    Found       string              // What did we actually get?
    Suggestions []ErrorSuggestion   // Actionable fixes
    DocsURL     string              // Link to documentation

    // NEW: Source context
    SourceLine  string              // The line containing the error
    ColumnStart int                 // Start column (for underlining)
    ColumnEnd   int                 // End column
}

type ErrorSuggestion struct {
    Description string              // Human-readable description
    Code        string              // Example code fix
    Location    token.Pos           // Where to apply fix (optional)
}

func (e *CompileError) FormatWithSource(fset *token.FileSet) string {
    var buf strings.Builder

    pos := fset.Position(e.Location)

    // Header: error: message
    fmt.Fprintf(&buf, "error: %s\n", e.Message)

    // Location: --> file:line:col
    fmt.Fprintf(&buf, "  --> %s:%d:%d\n", pos.Filename, pos.Line, pos.Column)

    // Source snippet with line number
    fmt.Fprintf(&buf, "   |\n")
    fmt.Fprintf(&buf, "%2d | %s\n", pos.Line, e.SourceLine)

    // Underline error location
    if e.ColumnStart > 0 && e.ColumnEnd > 0 {
        fmt.Fprintf(&buf, "   | %s%s\n",
            strings.Repeat(" ", e.ColumnStart-1),
            strings.Repeat("^", e.ColumnEnd-e.ColumnStart))
    }
    fmt.Fprintf(&buf, "   |\n")

    // Context
    if e.Context != "" {
        fmt.Fprintf(&buf, "   = %s\n", e.Context)
    }

    // Suggestions
    for _, sugg := range e.Suggestions {
        fmt.Fprintf(&buf, "help: %s\n", sugg.Description)
        if sugg.Code != "" {
            for _, line := range strings.Split(sugg.Code, "\n") {
                fmt.Fprintf(&buf, "    %s\n", line)
            }
        }
    }

    return buf.String()
}
```

**Pattern Match Error Helpers**:
```go
// pkg/errors/pattern_match.go (NEW)

func NonExhaustiveMatch(
    matchPos token.Pos,
    scrutineeType string,
    missingCases []string,
    fset *token.FileSet,
) *CompileError {
    return &CompileError{
        Message:  "non-exhaustive match",
        Location: matchPos,
        Category: ErrorCategoryPatternMatch,
        Context:  fmt.Sprintf("matching on type %s", scrutineeType),
        Expected: "all variants must be covered",
        Found:    fmt.Sprintf("missing cases: %s", strings.Join(missingCases, ", ")),
        Suggestions: []ErrorSuggestion{
            {
                Description: "add missing pattern arms",
                Code: fmt.Sprintf("%s => ...,", strings.Join(missingCases, " => ...,\n    ")),
            },
            {
                Description: "add a wildcard arm",
                Code: "_ => defaultValue",
            },
        },
        DocsURL: "https://dingolang.com/docs/errors#non-exhaustive-match",
        SourceLine: extractSourceLine(fset, matchPos),
        ColumnStart: extractColumnStart(fset, matchPos),
        ColumnEnd: extractColumnEnd(fset, matchPos),
    }
}
```

**Example Error Output**:
```
error: non-exhaustive match
  --> example.dingo:23:5
   |
23 | match result {
   |     ^^^^^^^^^^^^ missing Err case
   |
   = matching on type Result[User, Error]
help: add missing pattern arms
    Err(_) => ...,
help: add a wildcard arm
    _ => defaultValue
```

---

## Implementation Roadmap

### Phase 4.1: MVP (Weeks 1-2)

**Week 1: Foundation**

**Day 1-2: Configuration System**
- [ ] Implement `pkg/config/config.go` (Config, MatchConfig)
- [ ] Implement `pkg/config/loader.go` (Load, Validate)
- [ ] Add dingo.toml parsing (BurntSushi/toml)
- [ ] Unit tests for config loading/validation
- [ ] Integration: Generator reads config

**Day 3-4: AST Parent Tracking**
- [ ] Implement `Context.BuildParentMap()` in `pkg/plugin/context.go`
- [ ] Implement `Context.GetParent()` and `Context.WalkParents()`
- [ ] Unit tests for parent map construction
- [ ] Integration: Generator calls BuildParentMap() after parsing

**Day 5-7: Rust Pattern Match Preprocessor**
- [ ] Implement `pkg/preprocessor/rust_match.go` (RustMatchProcessor)
- [ ] Parse `match expr { arms }` syntax
- [ ] Generate switch skeleton with markers
- [ ] Preserve pattern structure in comments
- [ ] Unit tests for preprocessor
- [ ] Golden test: `pattern_match_01_simple.dingo` (Rust syntax)

**Week 2: Pattern Match Plugin + None Inference**

**Day 8-10: Pattern Match Plugin (Discovery + Exhaustiveness)**
- [ ] Implement `pkg/plugin/builtin/pattern_match.go` (PatternMatchPlugin)
- [ ] Discovery: Find DINGO_MATCH markers
- [ ] Parse pattern arms from markers
- [ ] Extract scrutinee type from go/types
- [ ] Implement exhaustiveness algorithm (variant extraction, coverage)
- [ ] Unit tests for exhaustiveness checker
- [ ] Golden test: `pattern_match_02_exhaustive.dingo` (error cases)

**Day 11-12: Pattern Match Plugin (Transformation)**
- [ ] Implement pattern compilation (tag-based dispatch)
- [ ] Extract pattern bindings
- [ ] Add default: panic() for exhaustive matches
- [ ] Unit tests for pattern transformation
- [ ] Golden test: `pattern_match_03_result_option.dingo`

**Day 13-14: None Context Inference**
- [ ] Implement `pkg/plugin/builtin/none_context.go` (NoneContextPlugin)
- [ ] Discovery: Find None identifiers
- [ ] Context walking (return, assignment, call, field)
- [ ] Type inference from context
- [ ] Error on ambiguity
- [ ] Unit tests for None inference
- [ ] Golden test: `option_06_none_inference.dingo`

**Deliverables (Phase 4.1)**:
- Configuration system (dingo.toml support)
- AST parent tracking
- Rust-like pattern match syntax (preprocessor + plugin)
- Exhaustiveness checking for Result/Option/Enum
- None constant context inference
- 5+ golden tests passing
- Basic error messages

---

### Phase 4.2: Advanced Patterns (Weeks 3-4)

**Week 3: Guards + Swift Syntax + Tuples**

**Day 15-16: Guard Support**
- [ ] Extend RustMatchProcessor for `Pattern if condition` syntax
- [ ] Generate conditional checks in switch cases
- [ ] Exhaustiveness with guards (conservative: require wildcard)
- [ ] Unit tests for guards
- [ ] Golden test: `pattern_match_04_guards.dingo`

**Day 17-18: Swift-like Syntax**
- [ ] Implement `pkg/preprocessor/swift_match.go` (SwiftMatchProcessor)
- [ ] Parse `switch expr { case .variant: }` syntax
- [ ] Generate same markers as RustMatchProcessor
- [ ] Unit tests for Swift preprocessor
- [ ] Golden test: `pattern_match_05_swift_syntax.dingo`

**Day 19-21: Tuple Destructuring**
- [ ] Implement tuple syntax support (multiple return destructuring)
- [ ] Extend pattern match for tuple patterns
- [ ] Type checking for tuple arms
- [ ] Unit tests for tuple patterns
- [ ] Golden test: `pattern_match_06_tuples.dingo`

**Week 4: Polish + Enhanced Errors + Integration**

**Day 22-23: Enhanced Error Messages**
- [ ] Implement source line extraction in CompileError
- [ ] Add column tracking for underlining
- [ ] Implement suggestion system
- [ ] Update all error helpers (NonExhaustiveMatch, etc.)
- [ ] Test error message formatting

**Day 24-25: Expression Mode Type Checking**
- [ ] Detect expression vs statement mode via parent
- [ ] Type-check all arms in expression mode (via go/types)
- [ ] Error if arm types mismatch
- [ ] Unit tests for expression mode
- [ ] Golden test: `pattern_match_07_expression_mode.dingo`

**Day 26-28: Integration + Documentation**
- [ ] Comprehensive golden test suite (10+ tests)
- [ ] Update `features/pattern-matching.md`
- [ ] Create `docs/errors.md` (error catalog)
- [ ] Update `ARCHITECTURE.md`
- [ ] Performance benchmarks (parent map, exhaustiveness)

**Deliverables (Phase 4.2)**:
- Guard support in patterns
- Swift-like syntax support (configurable)
- Tuple destructuring
- Enhanced error messages with source snippets
- Expression mode type checking
- 10+ golden tests total
- Complete documentation

---

## File/Package Structure

### New Files

```
pkg/
├── config/                          # NEW: Configuration system
│   ├── config.go                    # Config structs (MatchConfig, etc.)
│   └── loader.go                    # Load and validate dingo.toml
│
├── preprocessor/
│   ├── rust_match.go                # NEW: Rust-like match syntax preprocessor
│   └── swift_match.go               # NEW: Swift-like switch syntax preprocessor
│
├── plugin/
│   ├── context.go                   # ENHANCED: Add ParentMap, WalkParents
│   └── builtin/
│       ├── pattern_match.go         # NEW: Exhaustiveness checker, pattern compiler
│       └── none_context.go          # NEW: None type inference from context
│
└── errors/
    ├── pattern_match.go             # NEW: Pattern match error helpers
    └── type_inference.go            # ENHANCED: Better error messages with suggestions

tests/golden/
├── pattern_match_01_simple.dingo
├── pattern_match_02_exhaustive.dingo
├── pattern_match_03_result_option.dingo
├── pattern_match_04_guards.dingo
├── pattern_match_05_swift_syntax.dingo
├── pattern_match_06_tuples.dingo
├── pattern_match_07_expression_mode.dingo
├── option_06_none_inference.dingo
└── ...
```

### Modified Files

```
pkg/
├── generator/
│   └── generator.go                 # ENHANCED: Load config, call BuildParentMap, runTypeChecker
│
└── plugin/
    └── builtin/
        ├── result_type.go            # ENHANCED: Use go/types more extensively
        ├── option_type.go            # ENHANCED: Use go/types more extensively
        └── type_inference.go         # ENHANCED: Add parent-aware inference

go.mod                                # ADD: github.com/BurntSushi/toml dependency
```

---

## Testing Strategy

### Unit Tests

**Configuration Tests** (`pkg/config/config_test.go`):
- Loading valid dingo.toml
- Default config when file missing
- Validation errors (invalid syntax value)
- Multiple config sections

**Parent Map Tests** (`pkg/plugin/context_test.go`):
- Parent map construction correctness
- WalkParents traversal order
- Edge cases (nil nodes, root node)

**Rust Match Preprocessor Tests** (`pkg/preprocessor/rust_match_test.go`):
- Basic match parsing
- Multi-arm matches
- Guard extraction
- Pattern binding preservation
- Marker comment generation

**Swift Match Preprocessor Tests** (`pkg/preprocessor/swift_match_test.go`):
- Switch case parsing
- .variant(let x) extraction
- Marker compatibility with Rust

**Pattern Match Plugin Tests** (`pkg/plugin/builtin/pattern_match_test.go`):
- Exhaustiveness checking algorithm
- Variant extraction (Result, Option, Enum)
- Coverage tracking
- Error generation for missing cases
- Expression vs statement mode detection

**None Inference Tests** (`pkg/plugin/builtin/none_context_test.go`):
- Return statement context
- Assignment context
- Function call context
- Struct field context
- Match arm context
- Error on no context

### Integration Tests (Golden Tests)

**Phase 4.1 (MVP)**:
1. `pattern_match_01_simple.dingo` - Basic Result/Option matching (Rust syntax)
2. `pattern_match_02_exhaustive.dingo` - Exhaustiveness errors
3. `pattern_match_03_result_option.dingo` - Combined Result + Option patterns
4. `option_06_none_inference.dingo` - None context inference (valid + error cases)
5. `error_prop_07_match_combo.dingo` - Combined `?` operator + `match`

**Phase 4.2 (Advanced)**:
6. `pattern_match_04_guards.dingo` - Guard conditions
7. `pattern_match_05_swift_syntax.dingo` - Swift-like switch syntax
8. `pattern_match_06_tuples.dingo` - Tuple destructuring
9. `pattern_match_07_expression_mode.dingo` - Expression mode type checking
10. `pattern_match_08_enum.dingo` - Complex enum with struct variants
11. `pattern_match_09_nested.dingo` - Nested patterns (Ok(Some(x)))

**Test Coverage Goals**:
- Preprocessors: >90% coverage
- Plugins: >85% coverage
- Error paths: 100% coverage (all error types tested)

### Error Message Tests

**Validation** (snapshot testing):
- Verify error message formatting
- Check suggestion quality
- Validate source line extraction
- Test column underlining accuracy

---

## Performance Considerations

### AST Parent Map Construction

**Cost**: O(N) where N = number of AST nodes

**Benchmark Target**: <10ms for 10K node AST

**Optimization**:
- Build once, reuse across all plugins
- Clear after transformation (free memory)
- Use sync.Pool if memory becomes issue

### Exhaustiveness Checking

**Cost**: O(V * A) where V = variants, A = pattern arms

**Typical**: V = 2-5 (Result/Option/small enums), A = 2-10 arms

**Benchmark Target**: <1ms for typical match (3-5 arms)

**Optimization**:
- Early exit on wildcard pattern
- Bitset for variant coverage (not string set)
- Cache variant lists per type

### go/types Integration

**Cost**: Type checking is expensive (10-100ms for large files)

**Mitigation**:
- Already running in Phase 3 (no new cost)
- Store types.Info for reuse across plugins
- Partial type checking (don't fail on Dingo constructs)

**Benchmark Target**: No regression vs Phase 3 baseline

---

## Risks and Mitigations

### Risk 1: Dual Syntax Maintenance Burden

**Risk**: Supporting both Rust and Swift syntax doubles testing surface, potential for divergence

**Mitigation**:
- Both preprocessors generate **identical** AST markers
- PatternMatchPlugin is syntax-agnostic (processes markers only)
- Shared test suite validates both syntaxes
- Golden tests have variants for both syntaxes

**Fallback**: Deprecate Swift syntax in Phase 5 if adoption is <5%

### Risk 2: Expression Mode Type Checking Complexity

**Risk**: Type-checking match arms in expression mode may fail for complex types

**Mitigation**:
- Start with simple types (int, string, struct)
- Use go/types for inference (proven robust)
- Clear error messages if type mismatch detected

**Fallback**: Allow statement mode only in Phase 4.1, defer expression mode to 4.2

### Risk 3: None Inference Ambiguity

**Risk**: Multiple contexts (e.g., assignment then call) may conflict

**Mitigation**:
- Conservative approach: Error on ambiguity
- Document precedence rules clearly
- Require explicit type annotation if uncertain

**Fallback**: Disable None inference, require explicit types always

### Risk 4: Configuration File Parsing Errors

**Risk**: Invalid dingo.toml crashes transpiler

**Mitigation**:
- Comprehensive validation in `Config.Validate()`
- Clear error messages for invalid values
- Graceful fallback to defaults if file missing

**Fallback**: Ignore dingo.toml and use hardcoded defaults

---

## Success Criteria

### Functional Requirements

- ✅ Pattern matching compiles for Result, Option, Enum (Rust + Swift syntax)
- ✅ Exhaustiveness checking catches missing cases (compile error)
- ✅ Guards work correctly (`Pattern if condition`)
- ✅ Tuple destructuring works (Phase 4.2)
- ✅ None infers type from return/assignment/call/field context
- ✅ Expression mode type-checks all arms for consistency
- ✅ Error messages include source snippets and suggestions
- ✅ Configuration (dingo.toml) loads and validates correctly
- ✅ All golden tests pass (10+ tests)

### Non-Functional Requirements

- ✅ Compilation time <10% slower than Phase 3 (acceptable: +10-20ms)
- ✅ Generated code is idiomatic Go (readable switch statements)
- ✅ No false positives in exhaustiveness checking (<1% error rate)
- ✅ Error messages are beginner-friendly (tested with external reviewers)
- ✅ Documentation is complete and accurate (features/, docs/)
- ✅ Dual syntax support has <5% code duplication

### Quality Gates

**Before Phase 4.1 → 4.2**:
- 5+ pattern match golden tests passing (Rust syntax)
- Exhaustiveness checker validated (no false positives in tests)
- None inference works for all basic contexts (return, assign, call)
- Error messages reviewed by 2+ external developers

**Before Phase 4.2 → Complete**:
- Swift syntax golden tests passing (feature parity with Rust)
- Guards working in all test cases
- Tuple destructuring implemented and tested
- Performance benchmarks meet targets (<10ms parent map, <1ms exhaustiveness)

**Before Phase 4 → Phase 5**:
- All success criteria met
- 10+ comprehensive golden tests
- Documentation reviewed and published
- Community feedback on pattern matching collected (GitHub discussions)

---

## Dependencies and Ordering

### External Dependencies

**New**:
- `github.com/BurntSushi/toml` - TOML parsing for dingo.toml

**Existing** (no change):
- `go/parser`, `go/ast`, `go/printer` - Standard library
- `go/types` - Type checking
- `golang.org/x/tools/go/ast/astutil` - AST utilities

### Internal Dependencies (Execution Order)

**Build-time Ordering**:
1. Config loading (dingo.toml)
2. Rust/Swift preprocessor (dispatch based on config)
3. go/parser (parse preprocessed code)
4. BuildParentMap (construct parent map)
5. runTypeChecker (go/types integration)
6. Plugin pipeline:
   - ResultTypePlugin (inject Result types)
   - OptionTypePlugin (inject Option types)
   - PatternMatchPlugin (exhaustiveness + compilation)
   - NoneContextPlugin (infer None types)

**Plugin Dependencies**:
- NoneContextPlugin depends on: parent map, types.Info
- PatternMatchPlugin depends on: types.Info (for scrutinee type), parent map (for expression mode detection)
- Both depend on: Result/Option types already injected

---

## Open Questions (Resolved)

All questions resolved via user decisions (see "User Decisions" section).

**Resolved**:
1. ✅ Syntax preference → Dual support (Rust default, Swift optional)
2. ✅ Exhaustiveness strictness → Compile error (strict)
3. ✅ Match type → Infer from context (expression if assigned/returned)
4. ✅ None inference → Error on ambiguity (conservative)
5. ✅ Tuple destructuring → Yes (Phase 4.2)
6. ✅ Parent map → Build unconditionally
7. ✅ Error messages → Source snippets (rustc-style)

---

## Next Steps

### Immediate (Week 1, Day 1)

1. Create Phase 4.1 task breakdown (week 1 schedule)
2. Set up golden test directory structure
3. Add `github.com/BurntSushi/toml` dependency (go get)
4. Implement config package skeleton (`pkg/config/config.go`)

### After Week 1

1. Review Rust match preprocessor implementation
2. Validate exhaustiveness algorithm accuracy
3. Test AST parent map performance

### After Phase 4.1 Complete

1. User feedback on basic pattern matching
2. Evaluate exhaustiveness checking false positive rate
3. Prioritize Phase 4.2 features (guards, Swift, tuples)
4. Decide on nested pattern support (complexity vs benefit)

### After Phase 4.2 Complete

1. Performance optimization pass
2. Documentation review (external reviewers)
3. Showcase example update (add pattern matching to `showcase_01_api_server.dingo`)
4. Plan Phase 5 (Language Server implementation)

---

## References

### External Resources

**Pattern Matching**:
- Rust Pattern Matching: https://doc.rust-lang.org/book/ch18-00-patterns.html
- Swift Pattern Matching: https://docs.swift.org/swift-book/documentation/the-swift-programming-language/patterns/
- Kotlin Sealed Classes: https://kotlinlang.org/docs/sealed-classes.html
- OCaml Pattern Matching: http://ocaml.org/manual/patterns.html

**Type Inference**:
- TypeScript Handbook - Type Inference: https://www.typescriptlang.org/docs/handbook/type-inference.html
- Hindley-Milner Type System: https://en.wikipedia.org/wiki/Hindley%E2%80%93Milner_type_system

**Error Messages**:
- Rust Compiler Error Index: https://doc.rust-lang.org/error-index.html
- Elm Compiler Error Messages: https://elm-lang.org/news/compiler-errors-for-humans

### Internal Documentation

- `features/pattern-matching.md` - Feature specification
- `ARCHITECTURE.md` - Current architecture
- `ai-docs/CRITICAL-2-FIX-SUMMARY.md` - Phase 3 issues (A4, A5)
- `tests/golden/README.md` - Golden test catalog

### Go Proposals

- Proposal #45346 - Pattern matching for sum types
- Proposal #19412 - Sum types (996+ 👍, highest-voted)

---

## Appendix: Configuration Examples

### Example dingo.toml

```toml
# dingo.toml - Project configuration

[match]
# Syntax style: "rust" (default) or "swift"
syntax = "rust"

[compiler]
# Exhaustiveness checking: true = error, false = warning (future)
strict_exhaustiveness = true

[codegen]
# Generate source maps for LSP integration
source_maps = true

# Debug mode (extra comments in generated code)
debug_mode = false
```

### Using Rust Syntax (Default)

```dingo
// dingo.toml: syntax = "rust"

fn handleResult(result: Result[int, Error]) -> int {
    match result {
        Ok(x) => x * 2,
        Err(_) => 0
    }
}
```

### Using Swift Syntax (Opt-in)

```dingo
// dingo.toml: syntax = "swift"

fn handleResult(result: Result[int, Error]) -> int {
    switch result {
        case .ok(let x):
            return x * 2
        case .err(_):
            return 0
    }
}
```

---

**END OF FINAL PLAN**
