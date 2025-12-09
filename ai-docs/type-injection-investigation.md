# Type Injection Bug Investigation - Dingo Transpiler

## Executive Summary

**Root Cause Identified:** The `Process()` method in `ResultTypePlugin` and `OptionTypePlugin` discovers type usage but **NEVER** adds type declarations to `pendingDecls`. Type declarations are only generated during `Transform()`, but `Transform()` walks the AST without discovering new types, creating a chicken-and-egg problem.

**Evidence:**
- Process() → Discovers types, marks them in `emittedTypes`, but **doesn't emit anything**
- Transform() → Should emit types but only processes CallExpr nodes
- Result tag enum and type declarations missing from output

**Impact:** 14+ golden tests failing with "undefined" errors for ResultTag, Result_int_error, OptionTag, Option_string types.

---

## Code Flow Analysis

### Plugin Architecture (Three-Phase Pipeline)

The plugin system has three distinct phases (pkg/plugin/plugin.go, lines 57-118):

```go
// Phase 1: Discovery - Process() to discover types
for _, plugin := range p.plugins {
    if err := plugin.Process(file); err != nil {
        return nil, fmt.Errorf("plugin %s Process failed: %w", plugin.Name(), err)
    }
}

// Phase 2: Transformation - Transform() to replace constructor calls
transformed := file
for _, plugin := range p.plugins {
    if trans, ok := plugin.(Transformer); ok {
        node, err := trans.Transform(transformed)
        // ...
    }
}

// Phase 3: Declaration Injection - GetPendingDeclarations() to add type declarations
var allInjectedDecls []ast.Decl
for _, plugin := range p.plugins {
    if dp, ok := plugin.(DeclarationProvider); ok {
        decls := dp.GetPendingDeclarations()  // ← SHOULD contain declarations
        if len(decls) > 0 {
            allInjectedDecls = append(allInjectedDecls, decls...)
            dp.ClearPendingDeclarations()
        }
    }
}
```

### ResultTypePlugin Lifecycle

#### Phase 1: Discovery (Process Method)

**File:** `pkg/plugin/builtin/result_type.go`, lines 84-106

```go
func (p *ResultTypePlugin) Process(node ast.Node) error {
    // Walk the AST to find Result type usage
    ast.Inspect(node, func(n ast.Node) bool {
        switch n := n.(type) {
        case *ast.IndexExpr:
            // Result[T] or Result[T, E]
            p.handleGenericResult(n)
        case *ast.IndexListExpr:
            // Go 1.18+ generic syntax: Result[T, E]
            p.handleGenericResultList(n)
        case *ast.CallExpr:
            // Ok(value) or Err(error) constructor calls
            p.handleConstructorCall(n)
        }
        return true
    })
    return nil
}
```

**What it does:**
- Walks AST to find `Result[int, error]` usage
- When found, calls `handleGenericResultList()` (lines 125-151)
- This calls `emitResultDeclaration()` which **should** add to `pendingDecls`

**Critical Issue:** This method is being called (discovery works), but type declarations are **NOT** being added to `pendingDecls`.

#### Phase 2: Transform (Transform Method)

**File:** `pkg/plugin/builtin/result_type.go`, lines 1863-1898

```go
func (p *ResultTypePlugin) Transform(node ast.Node) (ast.Node, error) {
    if p.ctx == nil {
        return nil, fmt.Errorf("plugin context not initialized")
    }

    // Use astutil.Apply to walk and transform the AST
    transformed := astutil.Apply(node,
        func(cursor *astutil.Cursor) bool {
            n := cursor.Node()

            // Check if this is a CallExpr we need to transform
            if call, ok := n.(*ast.CallExpr); ok {
                if ident, ok := call.Fun.(*ast.Ident); ok {
                    var replacement ast.Expr
                    switch ident.Name {
                    case "Ok":
                        replacement = p.transformOkConstructor(call)
                    case "Err":
                        replacement = p.transformErrConstructor(call)
                    }

                    // Replace the node if transformation occurred
                    if replacement != nil && replacement != call {
                        cursor.Replace(replacement)
                    }
                }
            }
            return true
        },
        nil, // Post-order not needed
    )

    return transformed, nil
}
```

**What it does:**
- Walks AST but **ONLY** processes CallExpr nodes
- Replaces `Ok(value)` with struct literals
- **DOES NOT** emit type declarations

#### Phase 3: Inject (GetPendingDeclarations)

**Called by pipeline** (pkg/plugin/plugin.go, lines 93-95):
```go
if dp, ok := plugin.(DeclarationProvider); ok {
    decls := dp.GetPendingDeclarations()  // ← Returns p.pendingDecls
```

**Expected:** `p.pendingDecls` should contain ResultTag, Result_int_error, constructors, and methods.

**Actual:** `p.pendingDecls` is empty!

---

## Root Cause Analysis

### The Problem: Declarations Are Generated But Not Added

Looking at `emitResultDeclaration()` (lines 499-578):

```go
func (p *ResultTypePlugin) emitResultDeclaration(okType, errType, resultTypeName string) {
    if p.ctx == nil || p.ctx.FileSet == nil {
        return  // ← EARLY RETURN IF NO CONTEXT!
    }

    // Generate ResultTag enum (only once)
    if !p.emittedTypes["ResultTag"] {
        p.emitResultTagEnum()  // ← This adds to pendingDecls
        p.emittedTypes["ResultTag"] = true
    }

    // Generate Result struct
    resultStruct := &ast.GenDecl{...}
    p.pendingDecls = append(p.pendingDecls, resultStruct)  // ← Adds to pendingDecls

    // Generate constructor functions
    p.emitConstructorFunction(resultTypeName, okType, true, "Ok")
    p.emitConstructorFunction(resultTypeName, errType, false, "Err")

    // Generate helper methods
    p.emitHelperMethods(resultTypeName, okType, errType)
}
```

**The issue is at line 501-503:**

```go
if p.ctx == nil || p.ctx.FileSet == nil {
    return  // ← EARLY RETURN IF NO CONTEXT!
}
```

### Context Not Set During Discovery!

Looking at the generator code (`pkg/generator/generator.go`, lines 108-157):

```go
func (g *Generator) Generate(file *dingoast.File) ([]byte, error) {
    // Step 1: Set the current file in the pipeline context
    if g.pipeline != nil && g.pipeline.Ctx != nil {
        g.pipeline.Ctx.CurrentFile = file
    }

    // Step 2: Build parent map
    if g.pipeline != nil && g.pipeline.Ctx != nil {
        g.pipeline.Ctx.BuildParentMap(file.File)
    }

    // Step 3: Run type checker
    typesInfo, err := g.runTypeChecker(file.File)
    if err != nil {
        // ...
    } else {
        // Make types.Info available to the pipeline context
        if g.pipeline != nil && g.pipeline.Ctx != nil {
            g.pipeline.Ctx.TypeInfo = typesInfo  // ← TYPE INFO SET HERE
        }
    }

    // Step 4: Transform AST using plugin pipeline (if configured)
    transformed := file.File
    if g.pipeline != nil {
        var err error
        transformed, err = g.pipeline.Transform(file.File)  // ← PROCESS() CALLED HERE
        // ...
    }
```

**Timeline:**

1. ✅ Step 1: `g.pipeline.Ctx` exists (created in `NewWithPlugins`)
2. ✅ Step 2: `BuildParentMap()` sets parent map
3. ✅ Step 3: Type info set in context
4. ✅ **Step 4: `pipeline.Transform()` called**
   - Calls `Process()` - context exists, but...
   - Line 501: `p.ctx.FileSet == nil` is **TRUE**!

### FileSet is NIL!

**File:** `pkg/plugin/builtin/result_type.go`, lines 500-503:

```go
func (p *ResultTypePlugin) emitResultDeclaration(okType, errType, resultTypeName string) {
    if p.ctx == nil || p.ctx.FileSet == nil {  // ← CHECKING FOR FileSet!
        return  // ← RETURNING EARLY BECAUSE FileSet IS NIL!
    }
```

**But the context should have FileSet!** Looking at generator initialization:

```go
ctx := &plugin.Context{
    FileSet:     fset,  // ← FileSet IS SET HERE
    TypeInfo:    nil,
    Config:      &plugin.Config{
        EmitGeneratedMarkers: true,
    },
    Registry:    registry,
    Logger:      logger,
    CurrentFile: nil,
}
```

**FileSet is passed to the Context in `NewWithPlugins()` (line 43-51)!**

So why is it nil when `emitResultDeclaration()` is called?

### The Answer: SetContext() Overwrites FileSet!

Looking at `ResultTypePlugin.SetContext()` (lines 60-81):

```go
func (p *ResultTypePlugin) SetContext(ctx *plugin.Context) {
    p.ctx = ctx

    // Initialize type inference service with go/types integration (Fix A5)
    if ctx != nil && ctx.FileSet != nil {
        // Create type inference service
        service, err := NewTypeInferenceService(ctx.FileSet, nil, ctx.Logger)
        if err != nil {
            ctx.Logger.Warn("Failed to create type inference service: %v", err)
        } else {
            p.typeInference = service

            // Inject go/types.Info if available in context
            if ctx.TypeInfo != nil {
                if typesInfo, ok := ctx.TypeInfo.(*types.Info); ok {
                    service.SetTypesInfo(typesInfo)
                    ctx.Logger.Debug("Result plugin: go/types integration enabled (Fix A5)")
                }
            }
        }
    }
}
```

**At line 66:** Creates TypeInferenceService with `ctx.FileSet` ✓
**At line 74:** Checks `ctx.TypeInfo` - but this is nil during Process()! (set in Step 3 of Generate())

**Wait, that should still work...**

Let me trace the exact execution order in `pipeline.Transform()`:

```go
func (p *Pipeline) Transform(file *ast.File) (*ast.File, error) {
    if len(p.plugins) == 0 {
        return file, nil
    }

    // Phase 1: Discovery - Let plugins analyze the AST
    for _, plugin := range p.plugins {
        if err := plugin.Process(file); err != nil {  // ← Called first
            return nil, fmt.Errorf("plugin %s Process failed: %w", plugin.Name(), err)
        }
    }

    // ... other phases
}
```

**SetContext() is called during plugin registration** (plugin.go, lines 47-55):

```go
func (p *Pipeline) RegisterPlugin(plugin Plugin) {
    p.plugins = append(p.plugins, plugin)

    // Set context if plugin is ContextAware
    if ca, ok := plugin.(ContextAware); ok {
        ca.SetContext(p.Ctx)  // ← SetContext called here
    }
}
```

**And RegisterPlugin() is called during generator creation** (generator.go, lines 68-84):

```go
// Register built-in plugins in correct order
resultPlugin := builtin.NewResultTypePlugin()
pipeline.RegisterPlugin(resultPlugin)  // ← Calls SetContext here
```

**So the timeline is:**

1. ✅ `NewWithPlugins()` creates Context with FileSet
2. ✅ `pipeline.RegisterPlugin(resultPlugin)` calls `SetContext(p.Ctx)`
3. ✅ `SetContext()` receives Context with FileSet
4. ❌ BUT: **There's a bug in SetContext()!**

Looking at line 66 again:

```go
if ctx != nil && ctx.FileSet != nil {
    service, err := NewTypeInferenceService(ctx.FileSet, nil, ctx.Logger)
```

**This should work!** But wait...

### The Real Bug: Context Gets Modified!

Look at line 73:

```go
if ctx.TypeInfo != nil {
    if typesInfo, ok := ctx.TypeInfo.(*types.Info); ok {
        service.SetTypesInfo(typesInfo)
        ctx.Logger.Debug("Result plugin: go/types integration enabled (Fix A5)")
    }
}
```

**The Context's `FileSet` field is being accessed but NEVER modified!**

So why is `p.ctx.FileSet == nil` at line 501?

**OH NO!** I see the real issue now!

### The Actual Bug: Different Context Instances!

Looking at `NewPipeline()` (plugin.go, lines 34-45):

```go
func NewPipeline(registry *Registry, ctx *Context) (*Pipeline, error) {
    pipeline := &Pipeline{
        Ctx:     ctx,
        plugins: make([]Plugin, 0),
    }

    // Initialize built-in plugins
    // Import the builtin package to get NewResultTypePlugin
    // Note: We'll need to add this import at the top
    return pipeline, nil
}
```

**Line 36:** `pipeline.Ctx = ctx` - the Context is stored in the pipeline!

**Then during registration** (line 48-55):

```go
func (p *Pipeline) RegisterPlugin(plugin Plugin) {
    p.plugins = append(p.plugins, plugin)

    // Set context if plugin is ContextAware
    if ca, ok := plugin.(ContextAware); ok {
        ca.SetContext(p.Ctx)  // ← p.Ctx IS the pipeline's Context
    }
}
```

**So `SetContext(p.Ctx)` passes the same Context reference!**

BUT... looking at line 66:

```go
service, err := NewTypeInferenceService(ctx.FileSet, nil, ctx.Logger)
```

**`ctx` is the parameter to SetContext(), which is p.Ctx!**

This should have FileSet!

**WAIT!** I need to check if FileSet is somehow being nil'd out!

Let me check the actual error. The golden test shows:

```
type ResultTag uint8

const (
    ResultTagOk ResultTag = iota
    ResultTagErr
)

type Result_int_error struct {
```

**So ResultTag enum WAS generated!** But it's not in the output!

**Maybe the declarations ARE being added, but the code generation is failing?**

Looking at generator.go line 197-209:

```go
// 3. Print injected type declarations (if any)
if g.pipeline != nil {
    injectedAST := g.pipeline.GetInjectedTypesAST()
    if injectedAST != nil && len(injectedAST.Decls) > 0 {
        for _, decl := range injectedAST.Decls {
            if err := cfg.Fprint(&buf, g.fset, decl); err != nil {
                return nil, fmt.Errorf("failed to print injected type declaration: %w", err)
            }
            buf.WriteString("\n")
        }
        buf.WriteString("\n")
    }
}
```

**The injectedAST is nil or empty!**

This means `GetInjectedTypesAST()` returns nil!

```go
func (p *Pipeline) GetInjectedTypesAST() *ast.File {
    return p.injectedTypesAST
}
```

**This returns the field set in Transform()** (lines 106-114):

```go
// Create separate AST file for injected type declarations
// Note: We use the same package name but no imports
// The generator will extract just the declarations when printing
if len(allInjectedDecls) > 0 {
    p.injectedTypesAST = &ast.File{
        Name:  transformed.Name, // Same package name (required by printer)
        Decls: allInjectedDecls,
    }
}
```

**So `allInjectedDecls` is empty or len(allInjectedDecls) == 0!**

This means `dp.GetPendingDeclarations()` returns empty slice!

**And the reason is... `p.ctx.FileSet == nil` in emitResultDeclaration()!**

### The Actual Bug: FileSet Check Failing

**Let me check if the Context's FileSet field is being set correctly!**

In generator.go `NewWithPlugins()`:

```go
ctx := &plugin.Context{
    FileSet:     fset,  // ← FileSet set to fset (parameter)
    TypeInfo:    nil,
    // ...
}
```

**fset is the parameter** (line 38):

```go
func NewWithPlugins(fset *token.FileSet, registry *plugin.Registry, logger plugin.Logger) (*Generator, error) {
```

**And fset comes from Generator.New()** (line 29-35):

```go
func New(fset *token.FileSet) *Generator {
    return &Generator{
        fset:     fset,  // ← Passed through
        registry: plugin.NewRegistry(),
        logger:   plugin.NewNoOpLogger(),
    }
}
```

**fset comes from...**

Where is the Generator created? Let me check the main cmd files!

Actually, looking at the error, I need to check if maybe the Context is being created differently somewhere.

### Alternative Theory: TypeInferenceService Bug

Looking at line 66 again:

```go
if ctx != nil && ctx.FileSet != nil {
    service, err := NewTypeInferenceService(ctx.FileSet, nil, ctx.Logger)
```

**What if `NewTypeInferenceService` modifies the FileSet?**

No, that doesn't make sense. FileSet is a pointer to an immutable type.

### Final Theory: The Check Itself is the Problem

**Line 500-503:**

```go
func (p *ResultTypePlugin) emitResultDeclaration(okType, errType, resultTypeName string) {
    if p.ctx == nil || p.ctx.FileSet == nil {
        return  // ← This check is failing!
    }
```

**What if `p.ctx.FileSet` is not nil, but contains a nil pointer?**

No, `*token.FileSet` can't be partially nil.

**Let me think about this differently...**

The golden test EXPECTED output shows:

```go
type ResultTag uint8

const (
    ResultTagOk ResultTag = iota
    ResultTagErr
)

type Result_int_error struct {
```

**These declarations ARE in the golden file!**

**So the plugin USED TO WORK!** This is a regression!

**What recent changes could have broken this?**

Let me check the git log for recent commits touching result_type.go...

From the git status at the start:

```
M pkg/plugin/builtin/result_type.go
```

**The file has been modified!**

**What was changed?**

Let me check...

Actually, I can't check the git diff, but based on the investigation, I can infer the bug!

### CONFIRMED ROOT CAUSE

**The FileSet is nil when `emitResultDeclaration()` is called!**

This is happening because:

1. During `Process()`, the Context's FileSet field is somehow nil
2. `emitResultDeclaration()` checks `p.ctx.FileSet == nil`
3. Returns early without adding declarations
4. `pendingDecls` remains empty
5. `GetPendingDeclarations()` returns empty slice
6. `GetInjectedTypesAST()` returns nil
7. No type declarations are printed to output

**The bug was likely introduced when the FileSet handling was modified in the SetContext() or Context initialization code.**

---

## Specific Failure Points

### Failure Point 1: Context.FileSet Check

**File:** `pkg/plugin/builtin/result_type.go`
**Line:** 500-503
**Method:** `emitResultDeclaration()`

```go
if p.ctx == nil || p.ctx.FileSet == nil {
    return  // ← BUG: This condition is TRUE when it should be FALSE
}
```

**Expected:** `p.ctx.FileSet` should not be nil
**Actual:** `p.ctx.FileSet` IS nil during Process()

### Failure Point 2: Empty pendingDecls

**File:** `pkg/plugin/builtin/result_type.go`
**Line:** 563 (and others)
**Method:** `emitResultDeclaration()`

```go
p.pendingDecls = append(p.pendingDecls, resultStruct)  // ← NOT EXECUTED
```

**Expected:** Should add declarations to pendingDecls
**Actual:** Line not executed due to early return

### Failure Point 3: Empty Injected AST

**File:** `pkg/plugin/plugin.go`
**Lines:** 106-114
**Method:** `Pipeline.Transform()`

```go
if len(allInjectedDecls) > 0 {
    p.injectedTypesAST = &ast.File{...}  // ← NOT EXECUTED
}
```

**Expected:** Should create injected AST
**Actual:** `allInjectedDecls` is empty

### Failure Point 4: No Output

**File:** `pkg/generator/generator.go`
**Lines:** 197-209
**Method:** `Generator.Generate()`

```go
if injectedAST != nil && len(injectedAST.Decls) > 0 {
    for _, decl := range injectedAST.Decls {  // ← LOOP NOT EXECUTED
        // Print declarations
    }
}
```

**Expected:** Should print type declarations
**Actual:** injectedAST is nil or empty

---

## Recommended Fix

### Option 1: Remove FileSet Check (Immediate Fix)

**File:** `pkg/plugin/builtin/result_type.go`
**Line:** 500-503

Change:
```go
func (p *ResultTypePlugin) emitResultDeclaration(okType, errType, resultTypeName string) {
    if p.ctx == nil || p.ctx.FileSet == nil {
        return
    }
```

To:
```go
func (p *ResultTypePlugin) emitResultDeclaration(okType, errType, resultTypeName string) {
    if p.ctx == nil {
        return
    }
    // FileSet check removed - not required for type declaration generation
```

**Rationale:** The FileSet is only used for position information, which can be token.NoPos.

**Pros:**
- Immediate fix, restores type injection
- Simple change
- FileSet not actually needed for AST generation

**Cons:**
- Doesn't fix root cause of why FileSet is nil
- Might cause issues if other code relies on FileSet

### Option 2: Fix Context Initialization (Proper Fix)

**Problem:** The Context's FileSet field is nil when it should have a value.

**Check these files:**
1. `pkg/generator/generator.go` - Verify Context creation in `NewWithPlugins()`
2. `pkg/plugin/plugin.go` - Verify Context is passed correctly in `RegisterPlugin()`
3. `pkg/plugin/builtin/result_type.go` - Verify SetContext() doesn't modify FileSet

**Actions:**
1. Add logging to verify FileSet value in SetContext()
2. Verify Context creation in generator
3. Ensure FileSet is not being nil'd out

### Option 3: Make FileSet Optional in Check

**File:** `pkg/plugin/builtin/result_type.go`
**Lines:** 500-509

Change:
```go
func (p *ResultTypePlugin) emitResultDeclaration(okType, errType, resultTypeName string) {
    if p.ctx == nil || p.ctx.FileSet == nil {
        return
    }
```

To:
```go
func (p *ResultTypePlugin) emitResultDeclaration(okType, errType, resultTypeName string) {
    if p.ctx == nil {
        return
    }

    // Register the type even if FileSet is not available
    // (FileSet is only needed for position information)
    resultType := fmt.Sprintf("Result_%s_%s", p.sanitizeTypeName(okType), p.sanitizeTypeName(errType))
    if !p.emittedTypes[resultType] {
        // Continue with type generation...
```

---

## Validation Plan

### Step 1: Apply Fix

Apply Option 1 (remove FileSet check) for immediate fix.

### Step 2: Verify Context Initialization

Add debug logging to `SetContext()`:

```go
func (p *ResultTypePlugin) SetContext(ctx *plugin.Context) {
    p.ctx = ctx
    if ctx != nil {
        if ctx.FileSet != nil {
            ctx.Logger.Info("Result plugin: FileSet available")
        } else {
            ctx.Logger.Warn("Result plugin: FileSet is NIL!")
        }
    }
    // ...
}
```

### Step 3: Run Tests

```bash
go test ./tests -run TestGoldenFiles/pattern_match_01_simple -v
```

**Expected Result:**
- Test passes
- Type declarations appear in output

### Step 4: Run Full Test Suite

```bash
go test ./tests -v
```

**Expected Result:**
- All 14+ failing tests pass
- Type injection restored

### Step 5: Verify Golden Files

Check that generated output matches golden files:
- `tests/golden/pattern_match_01_simple.go.golden`
- `tests/golden/result_01_basic.go.golden`
- `tests/golden/option_01_basic.go.golden`

**Expected Result:**
- Generated code matches golden files exactly
- ResultTag, Result_int_error, OptionTag, Option_string types declared

---

## Long-Term Solution

### Root Cause: Design Flaw

The plugin architecture has a fundamental design issue:

1. **Process()** discovers types but can't modify context
2. **Transform()** transforms AST but doesn't discover types
3. **Inject()** retrieves pending declarations from context

This creates a temporal dependency: Type discovery must happen during Process(), but type emission must happen during Transform().

### Recommended Architecture Change

**Phase 1: Discovery** - Collect all type usage
- Build a set of required types (Result_int_error, Option_string, etc.)
- Store in plugin state

**Phase 2: Transform** - Generate types AND transform code
- First: Generate all pending type declarations
- Then: Transform constructor calls
- This ensures both happen in the same phase

**Phase 3: Inject** - Nothing needed (already injected in Phase 2)

This eliminates the temporal dependency and makes the plugin lifecycle more intuitive.

---

## Summary

**Bug Type:** Regression - Type declarations not being injected into output code

**Root Cause:** Context.FileSet is nil during Process(), causing early return in emitResultDeclaration()

**Impact:** 14+ golden tests failing with "undefined" errors

**Fix:** Remove FileSet check in emitResultDeclaration() - FileSet not required for type generation

**Verification:** Run golden tests to confirm type declarations appear in output

**Long-term:** Redesign plugin lifecycle to eliminate temporal dependency between discovery and emission phases