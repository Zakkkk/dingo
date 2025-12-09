# Sum Types Phase 2.5 - Final Implementation Plan

**Session:** 20251116-225837
**Date:** 2025-11-16
**Status:** FINAL - Ready for Implementation
**Priority:** P0 - CRITICAL (Blocking Phase 3)

---

## Executive Summary

This plan completes Sum Types (Phase 2.5) to production-ready state by addressing all critical issues identified in the previous session. Based on user decisions, we will:

1. **Work in parallel** on position info bug + pattern destructuring
2. **Implement match expression IIFE wrapping NOW** (not deferred to Phase 3)
3. **Add configurable nil safety** with three modes (off/on/debug)
4. **Design and implement configuration system** (`dingo.toml`)
5. **Complete all 10 IMPORTANT code review issues**
6. **Verify end-to-end transpilation** with automated tests

**Key Changes from Initial Plan:**
- ✨ Added IIFE wrapping for match expressions (4-6 hours)
- ✨ Added configuration system design/implementation (3-4 hours)
- ✨ Nil safety becomes feature with 3 modes instead of binary choice
- ⚡ Parallel work on position bug + destructuring (saves time)

**Estimated Duration:** 3-4 days (vs 2-3 days initial)

---

## 1. User Decisions Incorporated

### Decision 1: Parallel Implementation
**Original Question:** Fix position bug first OR implement destructuring first?
**User Decision:** Work on both in parallel

**Impact:**
- Two independent work streams
- Position bug affects all generated declarations
- Destructuring affects only match transformation
- Minimal code conflicts
- Faster overall completion

**Implementation Strategy:**
- Fix position info in all generation functions simultaneously
- Implement destructuring in separate functions
- Merge both changes before testing
- Validate together in golden tests

### Decision 2: Match Expression IIFE Wrapping
**Original Question:** Defer IIFE wrapping to Phase 3 OR implement now?
**User Decision:** Implement now in Phase 2.5

**Impact:**
- +4-6 hours implementation time
- Enables match as expression immediately
- Removes significant limitation
- Better user experience in Phase 2

**Scope:**
- Detect expression vs statement context
- Wrap match body in IIFE when used as expression
- Handle return values correctly
- Add tests for both contexts

### Decision 3: Nil Safety Configuration
**Original Question:** Always check nil OR trust constructors?
**User Decision:** Make it configurable with 3 modes

**Three Modes:**
1. **Off** - No nil checks (trust constructors, maximum performance)
2. **On** - Always check nil with helpful panic message (default)
3. **Debug** - Checks in debug builds only, optimized out in release

**Additional Scope:**
- Design configuration system architecture
- Implement `dingo.toml` support
- Add config loading to transpiler
- Pass config to plugins
- Default to "on" for safety

**Impact:**
- +3-4 hours for config system
- Enables future configurable features
- Gives users flexibility
- Better for production use cases

---

## 2. Architecture Design

### 2.1 Configuration System

**New Component: Config Package**

```
pkg/config/
├── config.go        # Main config types and loading
├── defaults.go      # Default values
├── validation.go    # Config validation
└── config_test.go   # Tests
```

**Configuration Schema:**

```go
// pkg/config/config.go

package config

import (
    "github.com/BurntSushi/toml"
)

// Config represents the complete dingo.toml configuration
type Config struct {
    Transpiler TranspilerConfig `toml:"transpiler"`
    LSP        LSPConfig        `toml:"lsp"`
}

// TranspilerConfig controls transpilation behavior
type TranspilerConfig struct {
    // NilSafetyChecks controls nil pointer validation in pattern destructuring
    // Values: "off", "on", "debug"
    // Default: "on"
    NilSafetyChecks string `toml:"nil_safety_checks"`

    // SourceMaps enables source map generation
    // Default: true
    SourceMaps bool `toml:"source_maps"`

    // OptimizationLevel controls code generation optimizations
    // Values: "none", "basic", "aggressive"
    // Default: "basic"
    OptimizationLevel string `toml:"optimization_level"`
}

// LSPConfig controls language server behavior (future)
type LSPConfig struct {
    // Port for LSP server
    Port int `toml:"port"`
}

// NilSafetyMode represents nil safety check modes
type NilSafetyMode int

const (
    NilSafetyOff NilSafetyMode = iota
    NilSafetyOn
    NilSafetyDebug
)

// Load reads dingo.toml from the specified path
func Load(path string) (*Config, error) {
    cfg := &Config{}

    if _, err := toml.DecodeFile(path, cfg); err != nil {
        return nil, fmt.Errorf("failed to load config: %w", err)
    }

    // Apply defaults
    cfg.applyDefaults()

    // Validate
    if err := cfg.Validate(); err != nil {
        return nil, err
    }

    return cfg, nil
}

// applyDefaults sets default values for missing fields
func (c *Config) applyDefaults() {
    if c.Transpiler.NilSafetyChecks == "" {
        c.Transpiler.NilSafetyChecks = "on"
    }
    if c.Transpiler.OptimizationLevel == "" {
        c.Transpiler.OptimizationLevel = "basic"
    }
}

// GetNilSafetyMode parses the nil safety string into enum
func (c *Config) GetNilSafetyMode() (NilSafetyMode, error) {
    switch c.Transpiler.NilSafetyChecks {
    case "off":
        return NilSafetyOff, nil
    case "on":
        return NilSafetyOn, nil
    case "debug":
        return NilSafetyDebug, nil
    default:
        return 0, fmt.Errorf("invalid nil_safety_checks value: %q (must be 'off', 'on', or 'debug')", c.Transpiler.NilSafetyChecks)
    }
}
```

**dingo.toml Example:**

```toml
# Dingo Transpiler Configuration
# See: https://github.com/yourusername/dingo/docs/config.md

[transpiler]
# Nil safety checks for pattern destructuring
# Options: "off" (no checks), "on" (always check), "debug" (check in debug builds only)
# Default: "on"
nil_safety_checks = "on"

# Generate source maps for debugging
# Default: true
source_maps = true

# Code generation optimization level
# Options: "none", "basic", "aggressive"
# Default: "basic"
optimization_level = "basic"

[lsp]
# Language server port (future)
port = 6060
```

**Integration with Plugin System:**

```go
// pkg/plugin/plugin.go

type Context struct {
    // ... existing fields ...

    // Config holds transpiler configuration
    Config *config.Config
}

// Update constructor
func NewContext(file *ast.File, fset *token.FileSet, cfg *config.Config, logger Logger) *Context {
    return &Context{
        File:   file,
        FileSet: fset,
        Config: cfg,
        Logger: logger,
        DingoFile: dingoFile,
    }
}
```

**Usage in Sum Types Plugin:**

```go
// pkg/plugin/builtin/sum_types.go

func (p *SumTypesPlugin) generateDestructuring(...) []ast.Stmt {
    // Get nil safety mode from config
    nilSafetyMode, _ := p.currentContext.Config.GetNilSafetyMode()

    switch nilSafetyMode {
    case config.NilSafetyOff:
        // No checks - trust constructors
        return p.generateUnsafeDestructuring(...)

    case config.NilSafetyOn:
        // Always check with runtime panic
        return p.generateSafeDestructuring(...)

    case config.NilSafetyDebug:
        // Check only in debug builds
        return p.generateDebugDestructuring(...)
    }
}
```

### 2.2 Match Expression IIFE Wrapping

**Problem:** Match expressions can be used in expression contexts:

```dingo
// Expression context - needs IIFE wrapping
let area = match shape {
    Circle{radius} => 3.14 * radius * radius,
    Rectangle{width, height} => width * height,
    Point => 0.0,
}

// Statement context - no wrapping needed
match status {
    Pending => fmt.Println("pending"),
    Active => fmt.Println("active"),
}
```

**Solution: Immediately Invoked Function Expression (IIFE)**

**Detection:**
```go
func (p *SumTypesPlugin) isExpressionContext(cursor *astutil.Cursor) bool {
    parent := cursor.Parent()

    switch parent.(type) {
    case *ast.AssignStmt:   // x := match ...
        return true
    case *ast.ReturnStmt:   // return match ...
        return true
    case *ast.BinaryExpr:   // if match ... == 5
        return true
    case *ast.CallExpr:     // fmt.Println(match ...)
        return true
    case *ast.ExprStmt:     // match ... (statement context)
        return false
    default:
        return true  // Conservative: assume expression context
    }
}
```

**IIFE Generation:**

```go
// Original Dingo code:
let area = match shape {
    Circle{radius} => 3.14 * radius * radius,
    Rectangle{width, height} => width * height,
}

// Generated Go code:
area := func() float64 {
    switch shape.tag {
    case ShapeTag_Circle:
        radius := *shape.circle_radius
        return 3.14 * radius * radius
    case ShapeTag_Rectangle:
        width := *shape.rectangle_width
        height := *shape.rectangle_height
        return width * height
    }
    panic("unreachable")
}()
```

**Implementation:**

```go
func (p *SumTypesPlugin) transformMatchExpr(
    cursor *astutil.Cursor,
    matchExpr *dingoast.MatchExpr,
) {
    // Infer type from match arms
    resultType := p.inferMatchType(matchExpr)

    // Transform to switch statement
    switchStmt := p.transformMatchToSwitch(matchExpr)

    // Check context
    if p.isExpressionContext(cursor) {
        // Wrap in IIFE
        iife := &ast.CallExpr{
            Fun: &ast.FuncLit{
                Type: &ast.FuncType{
                    Params: &ast.FieldList{},
                    Results: &ast.FieldList{
                        List: []*ast.Field{{Type: resultType}},
                    },
                },
                Body: &ast.BlockStmt{
                    List: []ast.Stmt{
                        switchStmt,
                        // Add panic for exhaustiveness safety
                        &ast.ExprStmt{
                            X: &ast.CallExpr{
                                Fun: &ast.Ident{Name: "panic"},
                                Args: []ast.Expr{
                                    &ast.BasicLit{
                                        Kind: token.STRING,
                                        Value: `"unreachable: match should be exhaustive"`,
                                    },
                                },
                            },
                        },
                    },
                },
            },
        }

        cursor.Replace(iife)
    } else {
        // Use as statement
        cursor.Replace(switchStmt)
    }
}
```

**Type Inference:**

```go
func (p *SumTypesPlugin) inferMatchType(matchExpr *dingoast.MatchExpr) ast.Expr {
    // Simple heuristic: use first arm's expression type
    if len(matchExpr.Arms) == 0 {
        return &ast.Ident{Name: "interface{}"}
    }

    firstArm := matchExpr.Arms[0]

    // Try to infer from expression
    switch expr := firstArm.Body.(type) {
    case *ast.BasicLit:
        return p.inferFromLiteral(expr)
    case *ast.CallExpr:
        // Complex - default to interface{}
        return &ast.Ident{Name: "interface{}"}
    default:
        return &ast.Ident{Name: "interface{}"}
    }
}

func (p *SumTypesPlugin) inferFromLiteral(lit *ast.BasicLit) ast.Expr {
    switch lit.Kind {
    case token.INT:
        return &ast.Ident{Name: "int"}
    case token.FLOAT:
        return &ast.Ident{Name: "float64"}
    case token.STRING:
        return &ast.Ident{Name: "string"}
    default:
        return &ast.Ident{Name: "interface{}"}
    }
}
```

### 2.3 Nil Safety Implementation

**Three Modes Implementation:**

**Mode 1: Off (No Checks)**
```go
// Generated code - trust constructors
func destructureCircle(shape Shape) float64 {
    radius := *shape.circle_radius  // Direct dereference
    return 3.14 * radius * radius
}
```

**Mode 2: On (Always Check)**
```go
// Generated code - runtime nil check
func destructureCircle(shape Shape) float64 {
    if shape.circle_radius == nil {
        panic("dingo: invalid Shape.Circle - nil radius field (union not created via constructor?)")
    }
    radius := *shape.circle_radius
    return 3.14 * radius * radius
}
```

**Mode 3: Debug (Conditional Check)**
```go
// Generated code - check only in debug builds
func destructureCircle(shape Shape) float64 {
    // Use build tag or runtime flag
    if dingoDebug && shape.circle_radius == nil {
        panic("dingo: invalid Shape.Circle - nil radius field (union not created via constructor?)")
    }
    radius := *shape.circle_radius
    return 3.14 * radius * radius
}

// Generated helper at package level
var dingoDebug = os.Getenv("DINGO_DEBUG") != ""
```

**Code Generator:**

```go
func (p *SumTypesPlugin) generateNilCheck(
    fieldAccess *ast.SelectorExpr,
    variantName string,
    fieldName string,
) ast.Stmt {
    nilSafetyMode, _ := p.currentContext.Config.GetNilSafetyMode()

    switch nilSafetyMode {
    case config.NilSafetyOff:
        return nil  // No check

    case config.NilSafetyOn:
        return &ast.IfStmt{
            Cond: &ast.BinaryExpr{
                X:  fieldAccess,
                Op: token.EQL,
                Y:  &ast.Ident{Name: "nil"},
            },
            Body: &ast.BlockStmt{
                List: []ast.Stmt{
                    &ast.ExprStmt{
                        X: &ast.CallExpr{
                            Fun: &ast.Ident{Name: "panic"},
                            Args: []ast.Expr{
                                &ast.BasicLit{
                                    Kind:  token.STRING,
                                    Value: fmt.Sprintf(`"dingo: invalid %s - nil %s field (union not created via constructor?)"`, variantName, fieldName),
                                },
                            },
                        },
                    },
                },
            },
        }

    case config.NilSafetyDebug:
        return &ast.IfStmt{
            Cond: &ast.BinaryExpr{
                X: &ast.BinaryExpr{
                    X:  &ast.Ident{Name: "dingoDebug"},
                    Op: token.LAND,
                    Y: &ast.BinaryExpr{
                        X:  fieldAccess,
                        Op: token.EQL,
                        Y:  &ast.Ident{Name: "nil"},
                    },
                },
            },
            Body: &ast.BlockStmt{
                List: []ast.Stmt{
                    &ast.ExprStmt{
                        X: &ast.CallExpr{
                            Fun: &ast.Ident{Name: "panic"},
                            Args: []ast.Expr{
                                &ast.BasicLit{
                                    Kind:  token.STRING,
                                    Value: fmt.Sprintf(`"dingo: invalid %s - nil %s field (union not created via constructor?)"`, variantName, fieldName),
                                },
                            },
                        },
                    },
                },
            },
        }
    }

    return nil
}
```

---

## 3. Implementation Plan

### Phase 1: Configuration System (3-4 hours) - NEW

**Priority:** P0 - Required for nil safety feature

**Tasks:**
1. Create `pkg/config/` package
2. Implement config loading (TOML)
3. Add validation logic
4. Update plugin context to include config
5. Add tests for config loading and validation
6. Create example `dingo.toml.example`

**Files to Create:**
- `pkg/config/config.go`
- `pkg/config/defaults.go`
- `pkg/config/validation.go`
- `pkg/config/config_test.go`
- `dingo.toml.example` (root)

**Files to Modify:**
- `pkg/plugin/plugin.go` (add Config to Context)
- `cmd/dingo/main.go` (load config before transpiling)

**Implementation Steps:**

**Step 1: Create config package**
```bash
mkdir -p pkg/config
```

**Step 2: Implement config types and loading** (see 2.1 Architecture)

**Step 3: Add dependency**
```bash
go get github.com/BurntSushi/toml
```

**Step 4: Update plugin context**
```go
// pkg/plugin/plugin.go
type Context struct {
    File      *ast.File
    FileSet   *token.FileSet
    Config    *config.Config  // ← Add this
    Logger    Logger
    DingoFile *dingoast.File
}
```

**Step 5: Update transpiler to load config**
```go
// cmd/dingo/main.go or pkg/transpiler/transpiler.go

func Transpile(inputPath string) error {
    // Load config
    cfg, err := config.Load("dingo.toml")
    if err != nil {
        // Use defaults if config not found
        cfg = config.Default()
    }

    // ... parse file ...

    // Create context with config
    ctx := plugin.NewContext(file, fset, cfg, logger)

    // ... run plugins ...
}
```

**Step 6: Create example config**
```toml
# dingo.toml.example
# Copy to dingo.toml and customize for your project

[transpiler]
nil_safety_checks = "on"      # "off" | "on" | "debug"
source_maps = true
optimization_level = "basic"   # "none" | "basic" | "aggressive"

[lsp]
port = 6060
```

**Testing:**
```bash
go test ./pkg/config -v
```

**Success Criteria:**
- ✅ Config loads from dingo.toml
- ✅ Defaults applied when file missing
- ✅ Validation catches invalid values
- ✅ Config accessible in plugin context

---

### Phase 2: Fix Position Information (2-3 hours) - PARALLEL STREAM A

**Priority:** P1 - Blocks golden tests

**Tasks:**
1. Add `TokPos` field to all `GenDecl` creations
2. Use `enumDecl.Name.Pos()` as position source
3. Add helper function to reduce duplication
4. Update all generation functions

**Files to Modify:**
- `pkg/plugin/builtin/sum_types.go`

**Implementation:**

```go
// Helper function
func (p *SumTypesPlugin) makeGenDecl(tok token.Token, pos token.Pos, specs []ast.Spec) *ast.GenDecl {
    return &ast.GenDecl{
        TokPos: pos,
        Tok:    tok,
        Specs:  specs,
    }
}

func (p *SumTypesPlugin) makeConstDecl(pos token.Pos, specs []ast.Spec) *ast.GenDecl {
    return &ast.GenDecl{
        TokPos: pos,
        Tok:    token.CONST,
        Lparen: 1,  // Parenthesized const block
        Specs:  specs,
    }
}
```

Update all generation functions:
- `generateTagEnum()` - 2 declarations
- `generateUnionStruct()` - 1 declaration
- `generateConstructor()` - 1 function (uses FuncDecl, not GenDecl)
- `generateHelperMethod()` - 1 function

**Testing:**
```bash
go test ./tests -v -run TestGoldenFiles/sum_types
```

**Success Criteria:**
- ✅ All golden tests pass (4/4)
- ✅ No panics in `go/types` checker
- ✅ Generated code compiles

---

### Phase 3: Pattern Destructuring (4-6 hours) - PARALLEL STREAM B

**Priority:** P1 - Core feature incomplete

**Tasks:**
1. Implement struct pattern destructuring
2. Implement tuple pattern destructuring
3. Handle unit patterns (no-op)
4. Integrate nil safety checks based on config
5. Update `transformMatchArm()` to use destructuring
6. Add unit tests

**Files to Modify:**
- `pkg/plugin/builtin/sum_types.go`

**Implementation:**

**Step 1: Main destructuring function**
```go
func (p *SumTypesPlugin) generateDestructuring(
    matchedExpr ast.Expr,
    enumType string,
    pattern *dingoast.Pattern,
) []ast.Stmt {
    if pattern.Kind == dingoast.PatternUnit {
        return nil  // No destructuring for unit variants
    }

    variantName := pattern.Variant.Name

    // Get variant declaration from registry
    enumDecl := p.enumRegistry[enumType]
    var variantDecl *dingoast.VariantDecl
    for _, v := range enumDecl.Variants {
        if v.Name.Name == variantName {
            variantDecl = v
            break
        }
    }

    if variantDecl == nil {
        p.currentContext.Logger.Error("variant not found: %s", variantName)
        return nil
    }

    stmts := []ast.Stmt{}

    // Generate destructuring based on pattern type
    switch pattern.Kind {
    case dingoast.PatternStruct:
        stmts = p.generateStructDestructuring(matchedExpr, variantName, pattern)
    case dingoast.PatternTuple:
        stmts = p.generateTupleDestructuring(matchedExpr, variantName, pattern)
    }

    return stmts
}
```

**Step 2: Struct pattern destructuring**
```go
func (p *SumTypesPlugin) generateStructDestructuring(
    matchedExpr ast.Expr,
    variantName string,
    pattern *dingoast.Pattern,
) []ast.Stmt {
    stmts := []ast.Stmt{}

    for _, fieldPat := range pattern.Fields {
        // Generate field name: circle_radius
        fieldName := strings.ToLower(variantName) + "_" + fieldPat.Name

        // Create selector: shape.circle_radius
        fieldAccess := &ast.SelectorExpr{
            X:   matchedExpr,
            Sel: &ast.Ident{Name: fieldName},
        }

        // Add nil check if enabled
        nilCheck := p.generateNilCheck(fieldAccess, variantName, fieldPat.Name)
        if nilCheck != nil {
            stmts = append(stmts, nilCheck)
        }

        // Generate assignment: radius := *shape.circle_radius
        stmt := &ast.AssignStmt{
            Lhs: []ast.Expr{&ast.Ident{Name: fieldPat.Name}},
            Tok: token.DEFINE,
            Rhs: []ast.Expr{
                &ast.StarExpr{X: fieldAccess},  // Dereference pointer
            },
        }
        stmts = append(stmts, stmt)
    }

    return stmts
}
```

**Step 3: Tuple pattern destructuring**
```go
func (p *SumTypesPlugin) generateTupleDestructuring(
    matchedExpr ast.Expr,
    variantName string,
    pattern *dingoast.Pattern,
) []ast.Stmt {
    stmts := []ast.Stmt{}

    for i, binding := range pattern.Bindings {
        // Generate field name: ok_0, ok_1
        fieldName := strings.ToLower(variantName) + "_" + strconv.Itoa(i)

        fieldAccess := &ast.SelectorExpr{
            X:   matchedExpr,
            Sel: &ast.Ident{Name: fieldName},
        }

        // Add nil check
        nilCheck := p.generateNilCheck(fieldAccess, variantName, fmt.Sprintf("field_%d", i))
        if nilCheck != nil {
            stmts = append(stmts, nilCheck)
        }

        // Generate assignment
        stmt := &ast.AssignStmt{
            Lhs: []ast.Expr{&ast.Ident{Name: binding}},
            Tok: token.DEFINE,
            Rhs: []ast.Expr{
                &ast.StarExpr{X: fieldAccess},
            },
        }
        stmts = append(stmts, stmt)
    }

    return stmts
}
```

**Step 4: Update match arm transformation**
```go
func (p *SumTypesPlugin) transformMatchArm(
    enumType string,
    matchedExpr ast.Expr,
    arm *dingoast.MatchArm,
) (*ast.CaseClause, error) {
    // ... existing validation ...

    // Generate destructuring statements
    destructStmts := p.generateDestructuring(matchedExpr, enumType, arm.Pattern)

    // Build case body
    bodyStmts := []ast.Stmt{}
    bodyStmts = append(bodyStmts, destructStmts...)  // Add destructuring first

    // Add arm body (wrap in return if expression context)
    bodyStmts = append(bodyStmts, &ast.ExprStmt{X: arm.Body})

    return &ast.CaseClause{
        List: []ast.Expr{caseExpr},
        Body: bodyStmts,
    }, nil
}
```

**Testing:**
```bash
go test ./pkg/plugin/builtin -v -run TestGenerateDestructuring
go test ./pkg/plugin/builtin -v -run TestTransformMatch
```

**Success Criteria:**
- ✅ Struct patterns extract named fields
- ✅ Tuple patterns use positional bindings
- ✅ Nil checks inserted based on config mode
- ✅ Variables properly scoped to case body
- ✅ Generated code compiles

---

### Phase 4: Match Expression IIFE Wrapping (4-6 hours) - NEW

**Priority:** P1 - User-requested feature

**Tasks:**
1. Implement context detection (expression vs statement)
2. Implement type inference for match result
3. Implement IIFE generation
4. Add return statements to match arms
5. Add unreachable panic after switch
6. Add tests for both contexts

**Files to Modify:**
- `pkg/plugin/builtin/sum_types.go`

**Implementation:**

**Step 1: Context detection**
```go
func (p *SumTypesPlugin) isExpressionContext(cursor *astutil.Cursor) bool {
    parent := cursor.Parent()

    switch parent.(type) {
    case *ast.AssignStmt:
        return true
    case *ast.ReturnStmt:
        return true
    case *ast.BinaryExpr:
        return true
    case *ast.CallExpr:
        return true
    case *ast.CompositeLit:
        return true
    case *ast.ExprStmt:
        return false  // Statement context
    default:
        // Conservative: assume expression
        return true
    }
}
```

**Step 2: Type inference** (see 2.2 Architecture)

**Step 3: IIFE wrapping** (see 2.2 Architecture)

**Step 4: Update match transformation**
```go
func (p *SumTypesPlugin) transformMatchExpr(
    cursor *astutil.Cursor,
    matchExpr *dingoast.MatchExpr,
) {
    // Build switch statement
    switchStmt := p.buildSwitchStatement(matchExpr)

    // Check if expression context
    if p.isExpressionContext(cursor) {
        // Infer return type
        resultType := p.inferMatchType(matchExpr)

        // Wrap in IIFE
        iife := &ast.CallExpr{
            Fun: &ast.FuncLit{
                Type: &ast.FuncType{
                    Params: &ast.FieldList{},
                    Results: &ast.FieldList{
                        List: []*ast.Field{{Type: resultType}},
                    },
                },
                Body: &ast.BlockStmt{
                    List: []ast.Stmt{
                        switchStmt,
                        // Add panic for safety
                        &ast.ExprStmt{
                            X: &ast.CallExpr{
                                Fun:  &ast.Ident{Name: "panic"},
                                Args: []ast.Expr{
                                    &ast.BasicLit{
                                        Kind:  token.STRING,
                                        Value: `"unreachable: match should be exhaustive"`,
                                    },
                                },
                            },
                        },
                    },
                },
            },
        }

        cursor.Replace(iife)
    } else {
        // Statement context - use switch directly
        cursor.Replace(switchStmt)
    }
}
```

**Step 5: Add return statements to arms**
```go
func (p *SumTypesPlugin) transformMatchArm(
    enumType string,
    matchedExpr ast.Expr,
    arm *dingoast.MatchArm,
    isExprContext bool,  // ← New parameter
) (*ast.CaseClause, error) {
    // ... destructuring ...

    // Build body
    bodyStmts := []ast.Stmt{}
    bodyStmts = append(bodyStmts, destructStmts...)

    // Add arm body
    if isExprContext {
        // Wrap in return statement
        bodyStmts = append(bodyStmts, &ast.ReturnStmt{
            Results: []ast.Expr{arm.Body},
        })
    } else {
        // Use as expression statement
        bodyStmts = append(bodyStmts, &ast.ExprStmt{X: arm.Body})
    }

    return &ast.CaseClause{
        List: []ast.Expr{caseExpr},
        Body: bodyStmts,
    }, nil
}
```

**Testing:**
```bash
go test ./pkg/plugin/builtin -v -run TestMatchExpression
go test ./pkg/plugin/builtin -v -run TestMatchStatement
go test ./pkg/plugin/builtin -v -run TestMatchIIFE
```

**Success Criteria:**
- ✅ Detects expression vs statement context correctly
- ✅ IIFE wrapping works for assignments
- ✅ IIFE wrapping works for return statements
- ✅ Type inference is reasonable (may default to interface{})
- ✅ Generated code compiles and executes correctly

---

### Phase 5: Address Remaining IMPORTANT Issues (6-8 hours)

**Priority:** P2 - Required before merge

**5.1 Clean Up Placeholder Nodes (1 hour)**

Add `RemoveDingoNode()` method and call after deletion:

```go
// pkg/ast/file.go
func (f *File) RemoveDingoNode(node ast.Node) {
    delete(f.DingoNodes, node)
}

// pkg/plugin/builtin/sum_types.go
func (p *SumTypesPlugin) transformEnumDecl(...) {
    // ... generation ...

    placeholder := cursor.Node()
    cursor.Delete()

    // Clean up map
    if p.currentFile != nil {
        p.currentFile.RemoveDingoNode(placeholder)
    }
}
```

**5.2 Fix Constructor Parameter Aliasing (1 hour)**

Implement deep copy for field lists (see initial plan 3.2)

**5.3 Unsupported Pattern Errors (30 minutes)**

Add validation for pattern types (see initial plan 3.3)

**5.4 Match Guard Errors (30 minutes)**

Detect and error on guards (see initial plan 3.4)

**5.5 Field Name Collision Detection (2 hours)**

Implement collision checker (see initial plan 3.5)

**5.6 Document Memory Overhead (15 minutes)**

Add comments explaining layout (see initial plan 3.6)

**5.7 Comprehensive Match Tests (2 hours)**

Add missing test coverage (see initial plan 3.7)

---

### Phase 6: End-to-End Verification (4-6 hours)

**Priority:** P1 - Prove implementation works

**Tasks:**
1. Create example .dingo files with all features
2. Run full transpilation pipeline
3. Compile and execute generated code
4. Create automated e2e tests
5. Verify all code quality checks pass

**See initial plan Phase 4 for detailed steps**

**Example Files to Create:**
- `tests/e2e/sum_types_simple.dingo`
- `tests/e2e/sum_types_struct_variants.dingo`
- `tests/e2e/sum_types_generic.dingo`
- `tests/e2e/sum_types_match_expr.dingo` ← NEW
- `tests/e2e/sum_types_nil_safety.dingo` ← NEW

**New Example: Match Expression Context**
```dingo
package main

import "fmt"

enum Shape {
    Circle { radius: float64 },
    Rectangle { width: float64, height: float64 },
    Point,
}

func main() {
    shape := Shape_Circle(5.0)

    // Match as expression (requires IIFE)
    area := match shape {
        Circle{radius} => 3.14159 * radius * radius,
        Rectangle{width, height} => width * height,
        Point => 0.0,
    }

    fmt.Printf("Area: %f\n", area)
}
```

**New Example: Nil Safety Modes**
```dingo
package main

import "fmt"

enum Result[T, E] {
    Ok(T),
    Err(E),
}

func unsafeExtract(r: Result[int, string]) -> int {
    // With nil_safety_checks = "off", this generates direct dereference
    match r {
        Ok(value) => value,
        Err(_) => 0,
    }
}

func safeExtract(r: Result[int, string]) -> int {
    // With nil_safety_checks = "on", this adds runtime nil checks
    match r {
        Ok(value) => value,
        Err(_) => 0,
    }
}

func main() {
    result := Result_Ok(42)
    fmt.Printf("Value: %d\n", safeExtract(result))
}
```

**Success Criteria:**
- ✅ All examples transpile without errors
- ✅ Generated Go code compiles
- ✅ Executables produce expected output
- ✅ Automated e2e tests pass
- ✅ No linter warnings
- ✅ No race conditions

---

## 4. Testing Strategy

### 4.1 Test Coverage Matrix

| Component | Unit Tests | Golden Tests | E2E Tests |
|-----------|-----------|--------------|-----------|
| Config loading | ✅ New | N/A | N/A |
| Position info | ✅ Update | ✅ Existing | N/A |
| Destructuring | ✅ New | ✅ Update | ✅ New |
| IIFE wrapping | ✅ New | ✅ New | ✅ New |
| Nil safety modes | ✅ New | ✅ New | ✅ New |

### 4.2 Test Priorities

**P0 - Must Pass Before Merge:**
- All existing unit tests (31/31)
- All new unit tests (8+ new tests)
- All golden tests (4 existing + 2 new)
- All e2e tests (5 new tests)

**P1 - Should Pass:**
- Code quality checks (fmt, vet, lint)
- Race detector
- Coverage > 85%

**P2 - Nice to Have:**
- Performance benchmarks
- Memory profiling

---

## 5. Timeline and Milestones

### Day 1 (8 hours)

**Morning (4h):**
- ✅ Implement configuration system (3h)
  - Create pkg/config package
  - Implement loading and validation
  - Add tests
  - Create dingo.toml.example
- ✅ Update plugin context integration (1h)

**Afternoon (4h):**
- ✅ Fix position information bug (2h)
- ✅ Begin pattern destructuring (2h)
  - Implement struct patterns
  - Begin tuple patterns

**Milestone:** Config system works, position bug fixed

### Day 2 (8 hours)

**Morning (4h):**
- ✅ Complete pattern destructuring (2h)
  - Finish tuple patterns
  - Integrate nil safety checks
  - Add tests
- ✅ Begin IIFE wrapping (2h)
  - Implement context detection
  - Start type inference

**Afternoon (4h):**
- ✅ Complete IIFE wrapping (4h)
  - Finish type inference
  - Implement IIFE generation
  - Add return statements
  - Add tests

**Milestone:** Destructuring + IIFE both working

### Day 3 (8 hours)

**Morning (4h):**
- ✅ Address IMPORTANT issues (4h)
  - DingoNodes cleanup
  - Parameter aliasing fix
  - Unsupported pattern errors
  - Match guard errors

**Afternoon (4h):**
- ✅ Continue IMPORTANT issues (4h)
  - Field collision detection
  - Memory overhead docs
  - Comprehensive tests
  - Code review and refactoring

**Milestone:** All critical issues resolved

### Day 4 (8 hours)

**Morning (4h):**
- ✅ End-to-end verification (4h)
  - Create example files
  - Run full pipeline
  - Create automated e2e tests

**Afternoon (4h):**
- ✅ Polish and merge prep (4h)
  - Code quality checks
  - Documentation updates
  - Final testing
  - Create PR

**Milestone:** Ready to merge

---

## 6. Success Criteria

### Must Have (P0)

- ✅ Configuration system implemented and tested
- ✅ Config loaded and passed to plugins
- ✅ Position information bug fixed
- ✅ All golden tests pass (6/6)
- ✅ Pattern destructuring implemented (struct + tuple)
- ✅ Nil safety checks work in all 3 modes (off/on/debug)
- ✅ Match expression IIFE wrapping works
- ✅ Match works in both expression and statement contexts
- ✅ All 10 IMPORTANT issues addressed
- ✅ End-to-end transpilation works
- ✅ Generated Go code compiles and executes
- ✅ Test coverage at 85%+
- ✅ Ready to merge to main

### Should Have (P1)

- ✅ E2E automated tests
- ✅ Code quality checks pass
- ✅ Helpful error messages
- ✅ Documentation complete (code comments + CHANGELOG)
- ✅ Example dingo.toml provided

### Could Have (P2)

- Performance benchmarks
- Memory profiling
- Additional golden tests
- Optimization documentation

---

## 7. Deliverables

### New Code

**New Packages:**
- `pkg/config/` - Configuration system (4 files)

**New Test Files:**
- `tests/e2e/sum_types_match_expr.dingo`
- `tests/e2e/sum_types_nil_safety.dingo`
- `tests/e2e_test.go` (5 tests)

**New Config:**
- `dingo.toml.example`

### Modified Code

**Core Implementation:**
- `pkg/plugin/builtin/sum_types.go` - All new features
- `pkg/plugin/plugin.go` - Config integration
- `cmd/dingo/main.go` - Config loading
- `pkg/ast/file.go` - RemoveDingoNode method

**Tests:**
- `pkg/plugin/builtin/sum_types_test.go` - +8 tests
- `pkg/config/config_test.go` - New package tests
- `tests/golden/sum_types_*.go.golden` - Updated expected output

### Documentation

**Session Documentation:**
- `ai-docs/sessions/20251116-225837/01-planning/final-plan.md` (this file)
- `ai-docs/sessions/20251116-225837/01-planning/plan-summary.txt`

**User Documentation:**
- Update CHANGELOG.md with Phase 2.5 completion
- Add config documentation to README or docs/

---

## 8. Risk Analysis

### High Risk Areas

**Configuration System (NEW):**
- Risk: Breaking change if config format changes
- Mitigation: Use example file, validate strictly, version config
- Fallback: Defaults work without config file

**IIFE Type Inference (NEW):**
- Risk: Wrong type inference breaks compilation
- Mitigation: Default to interface{} for safety
- Fallback: User can add type annotations (Phase 3)

**Nil Safety Modes (NEW):**
- Risk: "off" mode causes runtime panics
- Mitigation: Default to "on", document risks
- Fallback: Users can change mode per-project

### Medium Risk Areas

**IIFE Variable Capture:**
- Risk: Closure captures wrong variables
- Mitigation: Generate unique IIFE per match, no captures needed
- Fallback: Test thoroughly, add validation

**Pattern Destructuring Scope:**
- Risk: Variables leak outside case scope
- Mitigation: Use case-local declarations
- Fallback: Test scope carefully

### Low Risk Areas

**Position Information:**
- Risk: Low (proven approach)

**DingoNodes Cleanup:**
- Risk: Low (simple map deletion)

---

## 9. Key Differences from Initial Plan

### Added Scope

1. **Configuration System** (+3-4 hours)
   - Full config package implementation
   - TOML loading and validation
   - Plugin integration
   - Example config file

2. **IIFE Wrapping** (+4-6 hours)
   - Expression context detection
   - Type inference
   - IIFE generation
   - Return statement handling

3. **Nil Safety Modes** (complexity increase)
   - 3 modes instead of binary
   - Debug mode with runtime flag
   - Configurable per-project

### Changed Approach

1. **Parallel Implementation**
   - Position bug + destructuring in parallel (saves 1-2 hours)
   - Reduces overall timeline despite added scope

2. **Default Safety**
   - Nil safety "on" by default (vs "off" in initial plan)
   - More conservative approach for Phase 2

### Timeline Impact

**Initial Plan:** 2-3 days
**Final Plan:** 3-4 days

**Added:** +1 day for config system + IIFE wrapping
**Saved:** -0.5 days from parallel work
**Net:** +0.5-1 day

---

## 10. Next Steps After Completion

### Immediate (Post-Merge)

1. Update CHANGELOG.md with Phase 2.5 features:
   - Match expressions in expression contexts (IIFE)
   - Pattern destructuring (struct and tuple)
   - Configurable nil safety (3 modes)
   - Configuration system (dingo.toml)

2. Create GitHub issues:
   - Phase 3: Type inference improvements
   - Phase 4: Exhaustiveness checking
   - Phase 5: Complex pattern support

3. Update documentation:
   - Add config reference to README
   - Document nil safety modes
   - Add match expression examples

### Short-Term (Next Sprint)

**Phase 3: Pattern Matching Enhancements**
- Exhaustiveness checking
- Better type inference
- Nested patterns
- Pattern guards implementation (not just errors)

**Phase 4: Sum Types Polish**
- Memory layout optimization
- Derive traits (Display, Debug)
- Generic constraints
- Better error messages

### Long-Term

**Phase 5: Stdlib Sum Types**
- Builtin Option[T]
- Builtin Result[T, E]
- Interop with Go (T, error)

**Phase 6: LSP Integration**
- Match completion
- Exhaustiveness warnings in editor
- Type hints for inferred types

---

## Appendix A: Example Generated Code

### Example 1: Match Expression with IIFE

**Input (Dingo):**
```dingo
enum Shape {
    Circle { radius: float64 },
    Rectangle { width: float64, height: float64 },
}

let area = match shape {
    Circle{radius} => 3.14 * radius * radius,
    Rectangle{width, height} => width * height,
}
```

**Output (Go):**
```go
type ShapeTag int

const (
    ShapeTag_Circle ShapeTag = iota
    ShapeTag_Rectangle
)

type Shape struct {
    tag              ShapeTag
    circle_radius    *float64
    rectangle_width  *float64
    rectangle_height *float64
}

func Shape_Circle(radius float64) Shape {
    return Shape{
        tag:           ShapeTag_Circle,
        circle_radius: &radius,
    }
}

func Shape_Rectangle(width float64, height float64) Shape {
    return Shape{
        tag:              ShapeTag_Rectangle,
        rectangle_width:  &width,
        rectangle_height: &height,
    }
}

// Match expression as IIFE
area := func() float64 {
    switch shape.tag {
    case ShapeTag_Circle:
        // Nil check (when nil_safety_checks = "on")
        if shape.circle_radius == nil {
            panic("dingo: invalid Shape.Circle - nil radius field (union not created via constructor?)")
        }
        radius := *shape.circle_radius
        return 3.14 * radius * radius

    case ShapeTag_Rectangle:
        if shape.rectangle_width == nil {
            panic("dingo: invalid Shape.Rectangle - nil width field (union not created via constructor?)")
        }
        if shape.rectangle_height == nil {
            panic("dingo: invalid Shape.Rectangle - nil height field (union not created via constructor?)")
        }
        width := *shape.rectangle_width
        height := *shape.rectangle_height
        return width * height
    }
    panic("unreachable: match should be exhaustive")
}()
```

### Example 2: Nil Safety Off Mode

**Config:**
```toml
[transpiler]
nil_safety_checks = "off"
```

**Output (Go):**
```go
area := func() float64 {
    switch shape.tag {
    case ShapeTag_Circle:
        // No nil check - trust constructors
        radius := *shape.circle_radius
        return 3.14 * radius * radius

    case ShapeTag_Rectangle:
        width := *shape.rectangle_width
        height := *shape.rectangle_height
        return width * height
    }
    panic("unreachable: match should be exhaustive")
}()
```

### Example 3: Debug Mode

**Config:**
```toml
[transpiler]
nil_safety_checks = "debug"
```

**Output (Go):**
```go
// Generated at package level
var dingoDebug = os.Getenv("DINGO_DEBUG") != ""

// Match expression
area := func() float64 {
    switch shape.tag {
    case ShapeTag_Circle:
        // Check only when DINGO_DEBUG env var is set
        if dingoDebug && shape.circle_radius == nil {
            panic("dingo: invalid Shape.Circle - nil radius field (union not created via constructor?)")
        }
        radius := *shape.circle_radius
        return 3.14 * radius * radius

    // ...
    }
    panic("unreachable: match should be exhaustive")
}()
```

---

**Plan Status:** FINAL - Ready for Implementation
**Confidence Level:** HIGH (85%)
**Estimated Total Time:** 28-32 hours (3.5-4 days)

This plan incorporates all user decisions and provides a clear roadmap to complete Phase 2.5 with production-ready sum types implementation, configurable nil safety, and full match expression support.
