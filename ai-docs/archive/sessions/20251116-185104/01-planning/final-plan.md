# Phase 1.6: Error Propagation Operator - Final Implementation Plan

## Executive Summary

This document provides the comprehensive, final implementation plan for Phase 1.6 of the Dingo project: complete error propagation operator (`?`) integration with full feature set. Based on user clarifications, this plan encompasses all major features in a single phase while maintaining the existing plugin architecture.

**Scope:** ALL features implemented in Phase 1.6
- Statement context (`x := expr?`)
- Expression context (`return expr?`) with statement lifting
- Error wrapping syntax (`expr? "message"`)
- Full go/types integration for accurate type inference
- VLQ source map encoding for IDE integration

**Architecture:** Plugin-based enhancement
- Enhance existing `ErrorPropagationPlugin` in `pkg/plugin/builtin/`
- Add comprehensive go/types integration
- Implement multi-pass transformation within plugin
- Full source map integration

**Timeline:** 6-8 working days (estimated)

---

## User Decisions Incorporated

1. **Plugin Architecture** ✅ - Keep and enhance existing plugin in `pkg/plugin/builtin/error_propagation.go`
2. **Type Inference** ✅ - Implement full `go/types` integration for robust zero value generation
3. **Feature Scope** ✅ - All features in Phase 1.6 (statement, expression, wrapping, source maps)
4. **No Deprecation** ✅ - Plugin system is the correct architecture for core features

---

## Current State Analysis

### Already Implemented

#### Parser Support (✅ Complete)
- **Location:** `/Users/jack/mag/dingo/pkg/parser/participle.go`
- `PostfixExpression` captures `ErrorPropagate *bool`
- `convertPostfix()` creates `ErrorPropagationExpr` nodes
- Dingo nodes tracked in `currentFile.DingoNodes` map

#### AST Representation (✅ Complete)
- **Location:** `/Users/jack/mag/dingo/pkg/ast/ast.go`
- `ErrorPropagationExpr` struct with X, OpPos, Syntax fields
- Implements `ast.Expr` interface properly
- Position tracking for source maps

#### Plugin Foundation (⚠️ Needs Enhancement)
- **Location:** `/Users/jack/mag/dingo/pkg/plugin/builtin/error_propagation.go`
- Basic transformation exists but limited
- Returns `temporaryStmtWrapper` hack
- No type inference (hardcoded `nil` zero values)
- No expression context handling
- No error wrapping support

#### Plugin System (✅ Complete)
- Registry, Pipeline, Context all functional
- Generator integration exists
- Ready for enhanced plugin implementation

---

## Architecture Design

### Core Strategy: Enhanced Plugin with Multi-Pass Transformation

The enhanced `ErrorPropagationPlugin` will implement sophisticated multi-pass transformation while remaining within the plugin framework:

**Pass 1: Discovery & Context Analysis**
- Walk AST to find all `ErrorPropagationExpr` nodes
- Determine statement vs expression context for each
- Build transformation plan with context metadata

**Pass 2: Type Resolution**
- Use `go/types` to infer function return types
- Generate accurate zero values for all types
- Validate error type compatibility

**Pass 3: Transformation & Injection**
- Transform based on context (statement vs expression)
- Inject statements for expression contexts (lifting)
- Apply error wrapping if message provided
- Record source map entries

### Key Challenge: Statement Context vs Expression Context

```dingo
// STATEMENT CONTEXT - Simple replacement
user := fetchUser(id)?
// Becomes multiple statements in place

// EXPRESSION CONTEXT - Requires lifting
return processUser(fetchUser(id)?)
// Must inject statements BEFORE the return
```

**Solution:** Context-aware transformation with statement hoisting

---

## Detailed Component Design

### Component 1: Enhanced Error Propagation Plugin

**Location:** `/Users/jack/mag/dingo/pkg/plugin/builtin/error_propagation.go`

**New Architecture:**

```go
type ErrorPropagationPlugin struct {
    plugin.BasePlugin

    // Multi-pass state
    discoveryPass *DiscoveryPass
    typePass     *TypePass
    transformPass *TransformPass

    // Type inference
    typesInfo    *types.Info
    typesConfig  *types.Config

    // Source mapping
    sourceMap    *sourcemap.Generator

    // Transformation state
    counter      int

    // Logger
    logger       plugin.Logger
}

// DiscoveryPass tracks all error propagation expressions
type DiscoveryPass struct {
    expressions  []*ErrorPropContext
}

// ErrorPropContext holds context for each ? operator
type ErrorPropContext struct {
    Expr          *dingoast.ErrorPropagationExpr
    Context       ContextType  // Statement or Expression
    EnclosingFunc *ast.FuncDecl
    EnclosingStmt ast.Stmt
    EnclosingBlock *ast.BlockStmt
    StmtIndex     int  // Index in block (for injection)
}

type ContextType int
const (
    ContextStatement ContextType = iota  // x := expr?
    ContextExpression                     // return expr?, fn(expr?)
)

// TypePass resolves types using go/types
type TypePass struct {
    info      *types.Info
    pkg       *types.Package
    config    *types.Config
}

// TransformPass performs actual AST transformation
type TransformPass struct {
    counter      int
    sourceMap    *sourcemap.Generator
}
```

**Transform Method (Entry Point):**

```go
func (p *ErrorPropagationPlugin) Transform(ctx *plugin.Context, node ast.Node) (ast.Node, error) {
    file, ok := node.(*ast.File)
    if !ok {
        return node, nil
    }

    // Get Dingo file for DingoNodes map
    dingoFile := ctx.CurrentFile  // Need to add to Context
    if dingoFile == nil {
        return node, nil
    }

    // Pass 1: Discovery
    if err := p.discover(file, dingoFile); err != nil {
        return nil, err
    }

    // Pass 2: Type Resolution
    if err := p.resolveTypes(file, ctx); err != nil {
        return nil, err
    }

    // Pass 3: Transformation
    transformed, err := p.transformAll(file)
    if err != nil {
        return nil, err
    }

    return transformed, nil
}
```

### Component 2: Type Inference Engine (go/types Integration)

**Location:** `/Users/jack/mag/dingo/pkg/plugin/builtin/type_inference.go`

**Purpose:** Use Go's standard `go/types` for comprehensive type analysis

```go
// TypeInference handles type resolution using go/types
type TypeInference struct {
    fset    *token.FileSet
    info    *types.Info
    pkg     *types.Package
    config  *types.Config
}

// NewTypeInference creates type inference engine
func NewTypeInference(fset *token.FileSet, file *ast.File) (*TypeInference, error) {
    info := &types.Info{
        Types:      make(map[ast.Expr]types.TypeAndValue),
        Defs:       make(map[*ast.Ident]types.Object),
        Uses:       make(map[*ast.Ident]types.Object),
        Implicits:  make(map[ast.Node]types.Object),
        Selections: make(map[*ast.SelectorExpr]*types.Selection),
        Scopes:     make(map[ast.Node]*types.Scope),
    }

    config := &types.Config{
        Importer: importer.Default(),
        Error: func(err error) {
            // Collect errors but don't fail
        },
    }

    pkg, err := config.Check("main", fset, []*ast.File{file}, info)
    if err != nil {
        // Continue even with errors - we can still infer some types
    }

    return &TypeInference{
        fset:   fset,
        info:   info,
        pkg:    pkg,
        config: config,
    }, nil
}

// InferFunctionReturnType gets return type of enclosing function
func (ti *TypeInference) InferFunctionReturnType(fn *ast.FuncDecl) (types.Type, error) {
    if fn.Type.Results == nil || len(fn.Type.Results.List) == 0 {
        return nil, fmt.Errorf("function has no return values")
    }

    // Get type of first return value (the success value, not error)
    firstReturn := fn.Type.Results.List[0]
    typ := ti.info.TypeOf(firstReturn.Type)
    if typ == nil {
        return nil, fmt.Errorf("could not determine return type")
    }

    return typ, nil
}

// GenerateZeroValue creates ast.Expr for zero value of type
func (ti *TypeInference) GenerateZeroValue(typ types.Type) ast.Expr {
    switch t := typ.(type) {
    case *types.Basic:
        return ti.basicZeroValue(t)
    case *types.Pointer:
        return &ast.Ident{Name: "nil"}
    case *types.Slice:
        return &ast.Ident{Name: "nil"}
    case *types.Map:
        return &ast.Ident{Name: "nil"}
    case *types.Chan:
        return &ast.Ident{Name: "nil"}
    case *types.Interface:
        return &ast.Ident{Name: "nil"}
    case *types.Struct:
        // Generate composite literal: TypeName{}
        return &ast.CompositeLit{
            Type: ti.typeToAST(t),
        }
    case *types.Array:
        // Generate composite literal: [N]Type{}
        return &ast.CompositeLit{
            Type: ti.typeToAST(t),
        }
    default:
        // Fallback: use zero value initialization
        return &ast.CallExpr{
            Fun: &ast.Ident{Name: "new"},
            Args: []ast.Expr{ti.typeToAST(typ)},
        }
    }
}

// basicZeroValue handles basic Go types
func (ti *TypeInference) basicZeroValue(basic *types.Basic) ast.Expr {
    switch basic.Kind() {
    case types.Bool:
        return &ast.Ident{Name: "false"}
    case types.Int, types.Int8, types.Int16, types.Int32, types.Int64,
         types.Uint, types.Uint8, types.Uint16, types.Uint32, types.Uint64,
         types.Uintptr:
        return &ast.BasicLit{Kind: token.INT, Value: "0"}
    case types.Float32, types.Float64:
        return &ast.BasicLit{Kind: token.FLOAT, Value: "0.0"}
    case types.Complex64, types.Complex128:
        return &ast.BasicLit{Kind: token.IMAG, Value: "0i"}
    case types.String:
        return &ast.BasicLit{Kind: token.STRING, Value: `""`}
    default:
        // UntypedNil, etc
        return &ast.Ident{Name: "nil"}
    }
}

// typeToAST converts types.Type to ast.Expr (for type expressions)
func (ti *TypeInference) typeToAST(typ types.Type) ast.Expr {
    // Implementation: convert types.Type back to AST representation
    // This is needed for composite literals and type assertions
    return ti.typeToASTImpl(typ)
}
```

### Component 3: Statement Lifting (Expression Context)

**Location:** `/Users/jack/mag/dingo/pkg/plugin/builtin/statement_lifter.go`

**Purpose:** Handle expression contexts by lifting statements

```go
// StatementLifter handles expression contexts
type StatementLifter struct {
    counter int
}

// LiftExpression extracts error propagation from expression context
func (sl *StatementLifter) LiftExpression(
    ctx *ErrorPropContext,
    typeInf *TypeInference,
) (*LiftResult, error) {

    // Generate unique temp var
    tmpVar := fmt.Sprintf("__tmp%d", sl.counter)
    errVar := fmt.Sprintf("__err%d", sl.counter)
    sl.counter++

    // Create assignment: tmpVar, errVar := expr
    assignment := &ast.AssignStmt{
        Lhs: []ast.Expr{
            &ast.Ident{Name: tmpVar},
            &ast.Ident{Name: errVar},
        },
        Tok: token.DEFINE,
        Rhs: []ast.Expr{ctx.Expr.X},
    }

    // Create error check: if errVar != nil { return zero, errVar }
    zeroValue, err := sl.getZeroValue(ctx, typeInf)
    if err != nil {
        return nil, err
    }

    errorCheck := &ast.IfStmt{
        Cond: &ast.BinaryExpr{
            X:  &ast.Ident{Name: errVar},
            Op: token.NEQ,
            Y:  &ast.Ident{Name: "nil"},
        },
        Body: &ast.BlockStmt{
            List: []ast.Stmt{
                &ast.ReturnStmt{
                    Results: []ast.Expr{
                        zeroValue,
                        &ast.Ident{Name: errVar},
                    },
                },
            },
        },
    }

    return &LiftResult{
        Statements:     []ast.Stmt{assignment, errorCheck},
        Replacement:    &ast.Ident{Name: tmpVar},
        TempVarName:    tmpVar,
        ErrorVarName:   errVar,
    }, nil
}

type LiftResult struct {
    Statements    []ast.Stmt  // Statements to inject before current
    Replacement   ast.Expr    // Expression to replace original with
    TempVarName   string
    ErrorVarName  string
}

// InjectStatements inserts statements into block before target statement
func (sl *StatementLifter) InjectStatements(
    block *ast.BlockStmt,
    targetIndex int,
    newStmts []ast.Stmt,
) error {

    if targetIndex < 0 || targetIndex > len(block.List) {
        return fmt.Errorf("invalid target index: %d", targetIndex)
    }

    // Build new statement list
    newList := make([]ast.Stmt, 0, len(block.List)+len(newStmts))

    // Copy statements before injection point
    newList = append(newList, block.List[:targetIndex]...)

    // Insert new statements
    newList = append(newList, newStmts...)

    // Copy remaining statements
    newList = append(newList, block.List[targetIndex:]...)

    // Update block
    block.List = newList

    return nil
}
```

### Component 4: Error Wrapping Support

**Location:** `/Users/jack/mag/dingo/pkg/plugin/builtin/error_wrapper.go`

**Purpose:** Handle `expr? "message"` syntax

**Parsing Enhancement:**

```go
// In parser - already exists, needs message capture
type ErrorPropagationExpr struct {
    X      ast.Expr    // The expression
    OpPos  token.Pos   // Position of '?'
    Syntax SyntaxStyle // question/bang/try

    // NEW: Error wrapping message
    Message string      // Optional message for error wrapping
    MessagePos token.Pos
}
```

**Transformation:**

```go
// ErrorWrapper handles error message wrapping
type ErrorWrapper struct{}

// WrapError generates fmt.Errorf wrapping
func (ew *ErrorWrapper) WrapError(
    errVar string,
    message string,
) ast.Expr {

    // Generate: fmt.Errorf("message: %w", errVar)
    return &ast.CallExpr{
        Fun: &ast.SelectorExpr{
            X:   &ast.Ident{Name: "fmt"},
            Sel: &ast.Ident{Name: "Errorf"},
        },
        Args: []ast.Expr{
            &ast.BasicLit{
                Kind:  token.STRING,
                Value: fmt.Sprintf(`"%s: %%w"`, message),
            },
            &ast.Ident{Name: errVar},
        },
    }
}

// NeedsImport checks if fmt import is needed
func (ew *ErrorWrapper) NeedsImport() bool {
    return true
}
```

### Component 5: Source Map Integration

**Location:** `/Users/jack/mag/dingo/pkg/sourcemap/generator.go`

**Enhancements:**

```go
// Enhance existing Generator
type Generator struct {
    sourceFile string
    genFile    string
    mappings   []Mapping

    // NEW: VLQ encoder
    producer   *sourcemap.Producer  // from go-sourcemap library
}

// AddExpansionMapping records one-to-many source mapping
func (g *Generator) AddExpansionMapping(
    dingoPos token.Position,
    goPositions []token.Position,
) {
    // Map single Dingo line to multiple Go lines
    for _, goPos := range goPositions {
        g.AddMapping(dingoPos, goPos)
    }
}

// Generate produces source map with VLQ encoding
func (g *Generator) Generate() ([]byte, error) {
    // Create producer
    g.producer = sourcemap.NewProducer()

    // Sort mappings by generated position (required for VLQ)
    sort.Slice(g.mappings, func(i, j int) bool {
        if g.mappings[i].GenLine != g.mappings[j].GenLine {
            return g.mappings[i].GenLine < g.mappings[j].GenLine
        }
        return g.mappings[i].GenColumn < g.mappings[j].GenColumn
    })

    // Add mappings to producer
    for _, m := range g.mappings {
        g.producer.AddMapping(&sourcemap.Mapping{
            GeneratedLine:   uint32(m.GenLine),
            GeneratedColumn: uint32(m.GenColumn),
            SourceLine:      uint32(m.SourceLine),
            SourceColumn:    uint32(m.SourceColumn),
            Source:          g.sourceFile,
        })
    }

    // Generate JSON with VLQ-encoded mappings
    return g.producer.ToJSON()
}
```

**Usage in Plugin:**

```go
// Record source mapping during transformation
func (p *ErrorPropagationPlugin) recordMapping(
    dingoExpr *dingoast.ErrorPropagationExpr,
    generatedStmts []ast.Stmt,
) {

    dingoPos := p.fset.Position(dingoExpr.OpPos)

    for _, stmt := range generatedStmts {
        genPos := p.fset.Position(stmt.Pos())
        p.sourceMap.AddMapping(dingoPos, genPos)
    }
}
```

---

## Implementation Steps

### Step 1: Parser Enhancement for Error Messages (1 day)

**Files to Modify:**
- `/Users/jack/mag/dingo/pkg/parser/participle.go`
- `/Users/jack/mag/dingo/pkg/ast/ast.go`

**Tasks:**
1. Add optional string literal parsing after `?` operator
2. Update `ErrorPropagationExpr` with `Message` and `MessagePos` fields
3. Parser tests for `expr?`, `expr? "msg"` variants
4. Verify AST node creation

**Test Cases:**
```dingo
x := fetch()?              // No message
y := fetch()? "failed"     // With message
z := fetch()?
  "multi-line message"        // Message on next line
```

**Deliverables:**
- ✅ Parser recognizes error message syntax
- ✅ AST captures message string
- ✅ 10+ parser test cases

### Step 2: Type Inference Foundation (2 days)

**Files to Create:**
- `/Users/jack/mag/dingo/pkg/plugin/builtin/type_inference.go`
- `/Users/jack/mag/dingo/pkg/plugin/builtin/type_inference_test.go`

**Tasks:**
1. Implement `TypeInference` struct with go/types integration
2. `InferFunctionReturnType()` - get function return types
3. `GenerateZeroValue()` - create zero values for all Go types
4. `typeToAST()` - convert types.Type back to AST expressions
5. Comprehensive tests for all type categories

**Type Categories to Support:**
- Basic types: int, string, bool, float, complex
- Pointer types: *T
- Composite types: []T, [N]T, map[K]V
- Struct types: struct{...}
- Interface types: interface{...}
- Chan types: chan T
- Named types: User, MyStruct

**Test Cases:**
```go
// Test each type category
func TestZeroValues(t *testing.T) {
    tests := []struct{
        typ      string
        expected string
    }{
        {"int", "0"},
        {"string", `""`},
        {"bool", "false"},
        {"*User", "nil"},
        {"[]int", "nil"},
        {"map[string]int", "nil"},
        {"User", "User{}"},  // struct
    }
    // ... test each case
}
```

**Deliverables:**
- ✅ Full go/types integration
- ✅ Zero value generation for all types
- ✅ 50+ type inference tests
- ✅ Handle edge cases (unnamed types, type aliases)

### Step 3: Statement Lifter (1.5 days)

**Files to Create:**
- `/Users/jack/mag/dingo/pkg/plugin/builtin/statement_lifter.go`
- `/Users/jack/mag/dingo/pkg/plugin/builtin/statement_lifter_test.go`

**Tasks:**
1. Implement `LiftExpression()` - extract statements from expressions
2. Implement `InjectStatements()` - insert into block statements
3. Handle position preservation for source maps
4. Test with various expression contexts

**Scenarios to Handle:**
```dingo
// Nested in return
return processUser(fetchUser(id)?)

// Nested in function call
result := validate(fetch()?, transform()?)

// Nested in binary expression
if fetch()? == expected { ... }

// In composite literal
user := User{
    Name: fetchName()?,
    Age: fetchAge()?,
}
```

**Algorithm:**
```
For expression context:
  1. Walk up AST to find enclosing statement and block
  2. Generate temp variables and error check statements
  3. Find statement index in block
  4. Inject new statements before current statement
  5. Replace expression with temp variable reference
  6. Update parent references
```

**Deliverables:**
- ✅ Expression context handled correctly
- ✅ Statement injection works for all block types
- ✅ 20+ test cases for various expression contexts
- ✅ Position tracking correct

### Step 4: Error Wrapper (1 day)

**Files to Create:**
- `/Users/jack/mag/dingo/pkg/plugin/builtin/error_wrapper.go`
- `/Users/jack/mag/dingo/pkg/plugin/builtin/error_wrapper_test.go`

**Tasks:**
1. Implement `WrapError()` - generate fmt.Errorf calls
2. Handle import injection (add "fmt" if needed)
3. Escape message strings properly
4. Test message formatting

**Transformation:**
```dingo
// Input
user := fetchUser(id)? "failed to fetch user"

// Generated
__tmp0, __err0 := fetchUser(id)
if __err0 != nil {
    return nil, fmt.Errorf("failed to fetch user: %w", __err0)
}
user := __tmp0
```

**Edge Cases:**
- Messages with quotes: `"user \"admin\" not found"`
- Messages with %: `"failed %d times"`
- Multi-line messages
- Empty messages (ignore wrapping)

**Deliverables:**
- ✅ Error wrapping generates correct fmt.Errorf calls
- ✅ Import injection works
- ✅ 15+ test cases including edge cases
- ✅ Handles string escaping

### Step 5: Discovery Pass (1 day)

**Files to Modify:**
- `/Users/jack/mag/dingo/pkg/plugin/builtin/error_propagation.go`

**Tasks:**
1. Implement `discover()` method
2. Walk AST to find all ErrorPropagationExpr nodes
3. Determine context (statement vs expression) for each
4. Find enclosing function, statement, block
5. Build transformation plan

**Context Detection Algorithm:**
```
For each ErrorPropagationExpr:
  1. Walk up parent chain
  2. If parent is AssignStmt → STATEMENT context
  3. If parent is ReturnStmt, CallExpr, etc → EXPRESSION context
  4. Record enclosing function for return type
  5. Record enclosing block for statement injection
  6. Store in transformation plan
```

**Data Structure:**
```go
type DiscoveryPass struct {
    expressions []*ErrorPropContext
}

type ErrorPropContext struct {
    Expr          *dingoast.ErrorPropagationExpr
    Context       ContextType
    EnclosingFunc *ast.FuncDecl
    EnclosingBlock *ast.BlockStmt
    TargetStmt    ast.Stmt
    StmtIndex     int
}
```

**Deliverables:**
- ✅ Discovery pass finds all ? operators
- ✅ Context detection accurate
- ✅ Parent tracking works
- ✅ 10+ test cases

### Step 6: Transformation Pass (2 days)

**Files to Modify:**
- `/Users/jack/mag/dingo/pkg/plugin/builtin/error_propagation.go`
- `/Users/jack/mag/dingo/pkg/plugin/builtin/error_propagation_test.go`

**Tasks:**
1. Implement `transformAll()` method
2. Handle statement context transformation
3. Handle expression context transformation (use StatementLifter)
4. Apply error wrapping (use ErrorWrapper)
5. Generate source map entries
6. Update parent references using astutil.Apply

**Transformation Algorithm:**
```
For each ErrorPropContext:
  if context == STATEMENT:
    // Simple replacement in place
    Generate: tmpVar, errVar := expr
    Generate: if errVar != nil { return zero, errVar }
    Generate: originalVar := tmpVar
    Replace statement with these three

  else if context == EXPRESSION:
    // Lift statements
    Generate: tmpVar, errVar := expr
    Generate: if errVar != nil { return zero, errVar }
    Inject before enclosing statement
    Replace expression with tmpVar reference

  if message != empty:
    // Apply error wrapping
    Replace errVar with fmt.Errorf(message, errVar)
    Ensure fmt import exists

  Record source mappings
```

**Using astutil.Apply:**
```go
import "golang.org/x/tools/go/ast/astutil"

transformed := astutil.Apply(file,
    func(cursor *astutil.Cursor) bool {
        node := cursor.Node()

        // Find nodes to replace
        if shouldReplace(node) {
            cursor.Replace(newNode)
        }

        return true
    },
    nil,
)
```

**Deliverables:**
- ✅ Statement context works correctly
- ✅ Expression context works correctly
- ✅ Error wrapping integrated
- ✅ Source maps recorded
- ✅ 30+ transformation tests

### Step 7: Source Map VLQ Encoding (1.5 days)

**Files to Modify:**
- `/Users/jack/mag/dingo/pkg/sourcemap/generator.go`
- `/Users/jack/mag/dingo/pkg/sourcemap/generator_test.go`

**Tasks:**
1. Integrate `github.com/go-sourcemap/sourcemap` library
2. Implement VLQ encoding in `Generate()`
3. Sort mappings by generated position
4. Test with real source map consumers (Chrome DevTools)
5. Validate format compliance

**Implementation:**
```go
import "github.com/go-sourcemap/sourcemap"

func (g *Generator) Generate() ([]byte, error) {
    producer := sourcemap.NewProducer()

    // Sort mappings
    sort.Slice(g.mappings, func(i, j int) bool {
        if g.mappings[i].GenLine != g.mappings[j].GenLine {
            return g.mappings[i].GenLine < g.mappings[j].GenLine
        }
        return g.mappings[i].GenColumn < g.mappings[j].GenColumn
    })

    // Add to producer
    for _, m := range g.mappings {
        producer.AddMapping(&sourcemap.Mapping{
            GeneratedLine:   uint32(m.GenLine),
            GeneratedColumn: uint32(m.GenColumn),
            SourceLine:      uint32(m.SourceLine),
            SourceColumn:    uint32(m.SourceColumn),
            Source:          g.sourceFile,
        })
    }

    return producer.ToJSON()
}
```

**Testing:**
1. Generate source map for test file
2. Load in Chrome DevTools
3. Verify position mapping correctness
4. Test with multi-line expansions

**Deliverables:**
- ✅ VLQ encoding works
- ✅ Source maps load in Chrome
- ✅ Positions map correctly
- ✅ 10+ source map tests

### Step 8: Generator Integration (1 day)

**Files to Modify:**
- `/Users/jack/mag/dingo/pkg/generator/generator.go`
- `/Users/jack/mag/dingo/pkg/plugin/plugin.go` (add Context fields)

**Tasks:**
1. Pass `dingoast.File` to plugin context
2. Create and pass source map generator
3. Write source map file after generation
4. Update plugin Context struct

**Context Enhancement:**
```go
// In pkg/plugin/plugin.go
type Context struct {
    FileSet  *token.FileSet
    TypeInfo *types.Info
    Config   *Config
    Registry *Registry
    Logger   Logger

    // NEW additions
    CurrentFile *dingoast.File      // Access to DingoNodes map
    SourceMap   *sourcemap.Generator // For recording mappings
}
```

**Generator Changes:**
```go
func (g *Generator) Generate(file *dingoast.File) ([]byte, error) {
    // Create source map generator
    sourceMapGen := sourcemap.NewGenerator(
        file.Name,
        file.Name + ".go",
    )

    // Update context
    if g.pipeline != nil {
        g.pipeline.ctx.CurrentFile = file
        g.pipeline.ctx.SourceMap = sourceMapGen
    }

    // Transform
    transformed := file.File
    if g.pipeline != nil {
        var err error
        transformed, err = g.pipeline.Transform(file.File)
        if err != nil {
            return nil, fmt.Errorf("transformation failed: %w", err)
        }
    }

    // Generate code
    var buf bytes.Buffer
    cfg := printer.Config{...}
    if err := cfg.Fprint(&buf, g.fset, transformed); err != nil {
        return nil, err
    }

    // Format
    formatted, err := format.Source(buf.Bytes())
    if err != nil {
        formatted = buf.Bytes()
    }

    // Write source map
    sourceMapData, err := sourceMapGen.Generate()
    if err != nil {
        return nil, fmt.Errorf("source map generation failed: %w", err)
    }

    sourceMapPath := file.Name + ".go.map"
    if err := os.WriteFile(sourceMapPath, sourceMapData, 0644); err != nil {
        return nil, fmt.Errorf("failed to write source map: %w", err)
    }

    return formatted, nil
}
```

**Deliverables:**
- ✅ Plugin receives DingoNodes
- ✅ Source map written to disk
- ✅ End-to-end flow works
- ✅ Integration tests pass

### Step 9: End-to-End Testing (1.5 days)

**Files to Create:**
- `/Users/jack/mag/dingo/tests/error_propagation_e2e_test.go`
- `/Users/jack/mag/dingo/tests/golden/error_prop_*.dingo`
- `/Users/jack/mag/dingo/tests/golden/error_prop_*.go.golden`

**Test Scenarios:**

**1. Simple Statement Context**
```dingo
// Input: simple.dingo
func fetchUser(id: int) (User, error) {
    data := fetch(id)?
    return User{data}, nil
}
```

**2. Expression Context**
```dingo
// Input: expression.dingo
func process(id: int) (User, error) {
    return transform(fetch(id)?)
}
```

**3. Error Wrapping**
```dingo
// Input: wrapping.dingo
func getUser(id: int) (User, error) {
    user := fetchUser(id)? "failed to fetch user"
    validated := validateUser(user)? "validation failed"
    return validated, nil
}
```

**4. Chained Operations**
```dingo
// Input: chained.dingo
func pipeline(id: int) (Result, error) {
    a := stepA(id)?
    b := stepB(a)?
    c := stepC(b)?
    return c, nil
}
```

**5. Complex Types**
```dingo
// Input: complex_types.dingo
func process() (*User, error) {
    ptr := fetchPtr()?
    slice := fetchSlice()?
    mapping := fetchMap()?
    return combine(ptr, slice, mapping)
}
```

**6. Multiple in Function**
```dingo
// Input: multiple.dingo
func multiStep() (Order, error) {
    user := fetchUser()?
    product := fetchProduct()?
    payment := processPayment(user, product)?
    order := createOrder(payment)?
    return order, nil
}
```

**Testing Strategy:**
1. Golden file comparison
2. Compile generated Go code
3. Run and verify behavior
4. Check source map correctness
5. Validate all types compile

**Validation:**
```go
func TestEndToEnd(t *testing.T) {
    tests := []struct{
        name     string
        dingoFile string
        goldenFile string
    }{
        {"simple", "simple.dingo", "simple.go.golden"},
        // ... more cases
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Parse
            file := parser.Parse(tt.dingoFile)

            // Generate
            gen := generator.New(fset)
            output, err := gen.Generate(file)
            require.NoError(t, err)

            // Compare with golden
            golden := readFile(tt.goldenFile)
            assert.Equal(t, golden, string(output))

            // Compile test
            tmpFile := writeTempFile(output)
            cmd := exec.Command("go", "build", tmpFile)
            err = cmd.Run()
            assert.NoError(t, err, "Generated code should compile")

            // Source map test
            sourceMap := readFile(tt.dingoFile + ".go.map")
            validateSourceMap(t, sourceMap)
        })
    }
}
```

**Deliverables:**
- ✅ 6+ golden file test cases
- ✅ All generated code compiles
- ✅ Source maps validated
- ✅ Behavior tests pass

### Step 10: Documentation (1 day)

**Files to Update:**
- `/Users/jack/mag/dingo/CHANGELOG.md`
- `/Users/jack/mag/dingo/features/error-propagation.md`

**Files to Create:**
- `/Users/jack/mag/dingo/examples/error_propagation/README.md`
- Example files showcasing all features

**Documentation Tasks:**
1. Update CHANGELOG with Phase 1.6 completion
2. Update feature status to ✅ Complete
3. Create comprehensive examples
4. Document type inference behavior
5. Document source map usage

**Example README Structure:**
```markdown
# Error Propagation Examples

## Basic Usage
[Code example]

## Error Wrapping
[Code example]

## Expression Context
[Code example]

## Type Inference
[Explanation of zero values]

## Source Maps
[How to use in IDE]
```

**Deliverables:**
- ✅ CHANGELOG updated
- ✅ Feature doc updated
- ✅ 5+ documented examples
- ✅ User-facing guide written

---

## Testing Strategy

### Unit Tests (Per Step)

**Type Inference Tests (50+ cases):**
- All basic types
- All pointer types
- All composite types
- Named types
- Type aliases
- Edge cases

**Statement Lifter Tests (20+ cases):**
- Different expression contexts
- Different block types
- Position tracking
- Edge cases

**Error Wrapper Tests (15+ cases):**
- Basic wrapping
- String escaping
- Import injection
- Edge cases

**Transformation Tests (30+ cases):**
- Statement context
- Expression context
- Combined features
- Multiple ? in function

### Integration Tests (6+ scenarios)

**Golden File Testing:**
- Input: .dingo file
- Expected: .go.golden file
- Source map: .go.map.golden file

**Compilation Testing:**
- All generated code must compile
- Use `go build` in tests

**Behavior Testing:**
- Run generated code
- Verify error handling works
- Compare with manual implementation

### Source Map Testing (10+ cases)

**Format Validation:**
- VLQ encoding correct
- JSON structure valid
- Positions accurate

**Consumer Testing:**
- Load in Chrome DevTools
- Verify position mapping
- Test with IDE LSP

---

## File Structure

```
pkg/
├── plugin/
│   ├── plugin.go              # MODIFY: Add CurrentFile, SourceMap to Context
│   └── builtin/
│       ├── error_propagation.go        # ENHANCE: Multi-pass transformation
│       ├── error_propagation_test.go   # ENHANCE: Comprehensive tests
│       ├── type_inference.go           # NEW: go/types integration
│       ├── type_inference_test.go      # NEW: Type tests
│       ├── statement_lifter.go         # NEW: Expression context handling
│       ├── statement_lifter_test.go    # NEW: Lifting tests
│       ├── error_wrapper.go            # NEW: Error message wrapping
│       └── error_wrapper_test.go       # NEW: Wrapper tests
│
├── parser/
│   └── participle.go          # MODIFY: Parse error message syntax
│
├── ast/
│   └── ast.go                 # MODIFY: Add Message field to ErrorPropagationExpr
│
├── generator/
│   └── generator.go           # MODIFY: Source map integration
│
└── sourcemap/
    ├── generator.go           # MODIFY: VLQ encoding
    └── generator_test.go      # ENHANCE: VLQ tests

tests/
├── error_propagation_e2e_test.go       # NEW: End-to-end tests
└── golden/                              # NEW: Golden files
    ├── simple.dingo
    ├── simple.go.golden
    ├── expression.dingo
    ├── expression.go.golden
    ├── wrapping.dingo
    ├── wrapping.go.golden
    ├── chained.dingo
    ├── chained.go.golden
    ├── complex_types.dingo
    ├── complex_types.go.golden
    ├── multiple.dingo
    └── multiple.go.golden

examples/error_propagation/
├── README.md                   # NEW: Example documentation
├── basic.dingo                 # NEW: Basic usage
├── wrapping.dingo              # NEW: Error wrapping
├── expression.dingo            # NEW: Expression context
└── real_world.dingo            # NEW: Realistic example
```

---

## Dependencies

### Already in go.mod ✅
- `github.com/go-sourcemap/sourcemap` - VLQ encoding
- `github.com/alecthomas/participle/v2` - Parser
- Standard library: `go/ast`, `go/token`, `go/printer`, `go/types`
- `golang.org/x/tools/go/ast/astutil` - AST manipulation

### Need to Verify
- `go/importer` - For type checking (standard library)
- All dependencies already available ✅

---

## Risk Assessment & Mitigation

### High Risk Items

**1. go/types Integration Complexity**
- **Risk:** Type checking may fail for complex code
- **Mitigation:**
  - Graceful degradation (fallback to simple types)
  - Comprehensive test coverage
  - Error collection without failing entirely

**2. Expression Context Statement Lifting**
- **Risk:** Complex nesting may be hard to handle
- **Mitigation:**
  - Start with simple cases
  - Use `astutil.Apply` for safe mutations
  - Extensive testing with nested expressions

**3. Parent Reference Updates**
- **Risk:** AST mutation may break parent links
- **Mitigation:**
  - Use `golang.org/x/tools/go/ast/astutil.Apply`
  - Never manually update parent pointers
  - Validate AST after transformation

### Medium Risk Items

**1. Source Map Correctness**
- **Risk:** VLQ encoding bugs
- **Mitigation:**
  - Use battle-tested library
  - Validate with real consumers
  - Compare with hand-written maps

**2. Import Injection**
- **Risk:** Adding "fmt" import may conflict
- **Mitigation:**
  - Check existing imports first
  - Use AST utilities for import manipulation
  - Test import conflict scenarios

### Low Risk Items

**1. Performance**
- **Risk:** Multi-pass may be slow
- **Mitigation:**
  - Profile if needed
  - Optimize hot paths
  - Likely not an issue for current scale

**2. Error Message Parsing**
- **Risk:** String escaping issues
- **Mitigation:**
  - Use standard Go string parsing
  - Test edge cases
  - Proper escaping in generation

---

## Timeline Estimate

### Conservative Estimate (8 working days)

| Step | Task | Hours | Days |
|------|------|-------|------|
| 1 | Parser Enhancement | 6 | 0.75 |
| 2 | Type Inference | 14 | 1.75 |
| 3 | Statement Lifter | 10 | 1.25 |
| 4 | Error Wrapper | 6 | 0.75 |
| 5 | Discovery Pass | 6 | 0.75 |
| 6 | Transformation Pass | 14 | 1.75 |
| 7 | Source Map VLQ | 10 | 1.25 |
| 8 | Generator Integration | 6 | 0.75 |
| 9 | End-to-End Testing | 10 | 1.25 |
| 10 | Documentation | 6 | 0.75 |
| **Total** | | **88 hours** | **11 days** |

**Buffer (20%):** +18 hours = **106 hours total**

**Realistic:** **13-14 working days** with testing and polish

### Optimistic Estimate (6 working days)

If development proceeds smoothly without major blockers: **6-7 days**

### Recommendation

**Target: 8 working days** (2 weeks calendar time with interruptions)

---

## Success Criteria

### Must Have (Phase 1.6 Complete)

- ✅ Parse `?` operator in all contexts
- ✅ Parse `expr? "message"` syntax
- ✅ Full go/types integration for type inference
- ✅ Accurate zero values for all Go types
- ✅ Statement context transformation works
- ✅ Expression context with statement lifting works
- ✅ Error wrapping generates fmt.Errorf calls
- ✅ VLQ-encoded source maps
- ✅ All generated code compiles
- ✅ 100+ tests pass
- ✅ 6+ golden file tests
- ✅ Documentation complete

### Quality Metrics

- ✅ Test coverage > 80% for new code
- ✅ All edge cases documented and tested
- ✅ Generated code is readable and idiomatic
- ✅ Source maps work in real IDEs
- ✅ No runtime overhead vs manual error handling

### Nice to Have (Future)

- ⏭️ Optimization: eliminate unnecessary temp vars
- ⏭️ Alternative syntaxes (`!`, `try`)
- ⏭️ Error type conversion support
- ⏭️ Integration with Result type (when implemented)

---

## Implementation Order Rationale

### Why This Order?

**Foundation First:**
1. Parser enhancement - enables all other features
2. Type inference - required for all transformations
3. Statement lifter - needed for expression context

**Core Features:**
4. Error wrapper - independent feature
5. Discovery pass - orchestration layer
6. Transformation - brings it all together

**Integration:**
7. Source maps - polish
8. Generator integration - wire it up
9. Testing - validate everything
10. Documentation - make it usable

### Parallel Opportunities

**Can be done in parallel:**
- Error wrapper (Step 4) during Type inference (Step 2)
- Documentation (Step 10) throughout development

**Must be sequential:**
- Steps 1-3 (foundation)
- Steps 5-6 (transformation)
- Steps 7-9 (integration)

---

## Appendix: Code Examples

### Example 1: Full Transformation (Statement Context)

**Input (Dingo):**
```dingo
func fetchUser(id: int) (User, error) {
    data := fetch(id)? "failed to fetch"
    return User{data}, nil
}
```

**Output (Go):**
```go
func fetchUser(id int) (User, error) {
    __tmp0, __err0 := fetch(id)
    if __err0 != nil {
        return User{}, fmt.Errorf("failed to fetch: %w", __err0)
    }
    data := __tmp0
    return User{data}, nil
}
```

**Source Map:**
```json
{
  "version": 3,
  "file": "output.go",
  "sources": ["input.dingo"],
  "mappings": "AAAA;AACA,QAAA,KAAA;AACA,EAAA;AACA,IAAA,MAAA;AACA,GAAA;AACA"
}
```

### Example 2: Expression Context Lifting

**Input (Dingo):**
```dingo
func process(id: int) (User, error) {
    return transform(fetch(id)?)
}
```

**Output (Go):**
```go
func process(id int) (User, error) {
    __tmp0, __err0 := fetch(id)
    if __err0 != nil {
        return User{}, __err0
    }
    return transform(__tmp0)
}
```

### Example 3: Multiple Error Propagations

**Input (Dingo):**
```dingo
func pipeline(id: int) (Result, error) {
    a := stepA(id)? "step A failed"
    b := stepB(a)? "step B failed"
    c := stepC(b)? "step C failed"
    return c, nil
}
```

**Output (Go):**
```go
func pipeline(id int) (Result, error) {
    __tmp0, __err0 := stepA(id)
    if __err0 != nil {
        return Result{}, fmt.Errorf("step A failed: %w", __err0)
    }
    a := __tmp0

    __tmp1, __err1 := stepB(a)
    if __err1 != nil {
        return Result{}, fmt.Errorf("step B failed: %w", __err1)
    }
    b := __tmp1

    __tmp2, __err2 := stepC(b)
    if __err2 != nil {
        return Result{}, fmt.Errorf("step C failed: %w", __err2)
    }
    c := __tmp2

    return c, nil
}
```

---

## Next Steps

1. ✅ Review this final plan
2. ✅ Get user approval
3. → Begin Step 1: Parser Enhancement
4. → Commit after each step
5. → Update progress in CHANGELOG
6. → Celebrate Phase 1.6 completion!

---

## References

### Internal
- `/Users/jack/mag/dingo/CLAUDE.md` - Project instructions
- `/Users/jack/mag/dingo/features/error-propagation.md` - Feature spec
- `/Users/jack/mag/dingo/pkg/plugin/builtin/error_propagation.go` - Current implementation

### External
- Go AST: https://pkg.go.dev/go/ast
- Go Types: https://pkg.go.dev/go/types
- AST Utilities: https://pkg.go.dev/golang.org/x/tools/go/ast/astutil
- Source Map Spec: https://sourcemaps.info/spec.html
- VLQ Encoding: https://www.html5rocks.com/en/tutorials/developertools/sourcemaps/
- go-sourcemap: https://github.com/go-sourcemap/sourcemap

### Similar Projects
- Rust Error Propagation: https://rust-lang.github.io/rfcs/0243-trait-based-exception-handling.html
- TypeScript Compiler: https://github.com/microsoft/TypeScript
- Borgo Transpiler: https://github.com/borgo-lang/borgo
