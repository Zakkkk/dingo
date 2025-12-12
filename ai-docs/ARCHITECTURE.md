# Dingo Transpiler Architecture

**Last Updated**: 2025-12-06
**Status**: MIGRATION IN PROGRESS - String manipulation being replaced with proper AST

---

## 🚨 CRITICAL: No String Manipulation 🚨

```
╔══════════════════════════════════════════════════════════════════════════════╗
║  CURRENT pkg/ast/*_codegen.go FILES USE STRING MANIPULATION - WRONG!        ║
║                                                                              ║
║  They must be DELETED and replaced with proper AST-based codegens that:     ║
║  • Use pkg/tokenizer/ for tokenization                                      ║
║  • Use pkg/parser/ for parsing (5,329 lines of Pratt parser exists!)        ║
║  • Accept AST nodes as input, NEVER raw bytes                               ║
║                                                                              ║
║  📖 FULL PLAN: ai-docs/plans/REMOVE_STRING_MANIPULATION.md                  ║
╚══════════════════════════════════════════════════════════════════════════════╝
```

---

## Overview

Dingo WILL use an **AST-based code generation** architecture:

1. **Tokenizer** - Tokenize source into tokens (pkg/tokenizer/) - ONLY place reading bytes
2. **Parser** - Parse tokens into AST nodes (pkg/parser/) - Pratt parser, 5,329 lines
3. **Codegen** - Generate Go from AST nodes (pkg/codegen/) - NEVER reads bytes
4. **Go Printer** - Output formatted Go code

**CURRENT STATE**: pkg/ast/*_codegen.go files incorrectly use string manipulation.
**TARGET STATE**: Proper pipeline using tokenizer → parser → codegen.

---

## The AST-Based Pipeline

```
┌─────────────────────────────────────────────────────────────┐
│                     .dingo Source File                       │
│  enum Color { Red, Green, Blue }                            │
│  func process(x: int) Result[int, error] { ... }            │
└────────────────────┬────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────┐
│            UNIFIED AST PIPELINE (pkg/ast/transform.go)       │
│                     ast.TransformSource()                    │
├─────────────────────────────────────────────────────────────┤
│  Each transform parses → generates Go → returns mappings:   │
│                                                              │
│    TransformEnumSource()        enum → Go interface          │
│    TransformLambdaSource()      |x| expr → func(x any) any  │
│    TransformMatchSource()       match → inline type switch   │
│    TransformErrorPropSource()   expr? → inline error check   │
│    TransformTernarySource()     a ? b : c → inline if        │
│    TransformNullCoalesceSource() a ?? b → inline nil check   │
│    TransformSafeNavSource()     x?.y → inline nil chain      │
│    TransformTupleSource()       (a, b) → struct literal      │
├─────────────────────────────────────────────────────────────┤
│  Returns: Go source + []SourceMapping for LSP integration   │
└────────────────────┬────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────┐
│              GO PARSER & PRINTER                             │
├─────────────────────────────────────────────────────────────┤
│  • go/parser.ParseFile()  →  Validate & build AST           │
│  • go/printer.Fprint()    →  Output formatted Go code       │
└────────────────────┬────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────┐
│                Output: .go file (compiles with go build)     │
└─────────────────────────────────────────────────────────────┘
```

---

## Why AST-Based Code Generation?

### The Problem We're Solving

Dingo needs to support syntax that **isn't valid Go**:
- `enum Color { Red, Green, Blue }` - enums don't exist in Go
- `param: Type` - Go uses `param Type` (space, not colon)
- `x?` - error propagation operator doesn't exist in Go
- `|x| x + 1` - Rust-style lambdas aren't valid Go

### The Solution: Parse → Generate → Map

Each Dingo feature follows the pattern:
1. **Parse**: Find and parse feature syntax into AST nodes
2. **Generate**: Generate equivalent Go code from AST nodes
3. **Map**: Track source mappings for LSP integration

### Advantages Over Old Approaches

**Old Regex Preprocessing** (`pkg/preprocessor/` - DELETED):
- Fragile (edge cases broke easily)
- Hard to debug (complex patterns)
- Error-prone (position tracking issues)

**Old Token-Level Transform** (`TransformToGo` - REMOVED):
- No source mapping support
- Hard to extend with new features
- Tight coupling between parsing and generation

**New AST-Based Pipeline** (`pkg/ast/` - CURRENT):
- **Modular**: Each feature is a separate parser + codegen
- **Source Maps**: Each transform returns position mappings
- **Testable**: Parsers and generators can be tested independently
- **Extensible**: Easy to add new features following the pattern

---

## Package Structure

```
pkg/
├── ast/                        # AST CODE GENERATION (NEW - December 2025)
│   ├── transform.go            # TransformSource() - unified pipeline entry
│   ├── sourcemap.go            # CodeGenResult, SourceMapping types
│   ├── helpers.go              # Shared helpers (isIdentChar, skipWhitespace)
│   │
│   ├── enum.go                 # Enum AST nodes
│   ├── enum_parser.go          # Parse enum declarations
│   ├── enum_codegen.go         # Generate Go interface pattern
│   │
│   ├── lambda_codegen.go       # |x| expr → func(x any) any { return }
│   ├── match_codegen.go        # match → inline type switch
│   ├── error_prop_codegen.go   # expr? → inline error handling
│   ├── ternary_codegen.go      # a ? b : c → inline if
│   ├── null_coalesce_codegen.go # a ?? b → inline nil check
│   ├── safe_nav_codegen.go     # x?.y → inline nil chain
│   └── tuple_codegen.go        # (a, b) → struct literal
│
├── parser/                     # DINGO PARSER (Pratt-based)
│   ├── file.go                 # File-level parsing
│   ├── pratt.go                # Pratt expression parser
│   ├── stmt.go                 # Statement parsing
│   └── simple.go               # Simple Dingo parser
│
├── goparser/                   # GO PARSER WRAPPER
│   ├── parser/
│   │   └── parser.go           # ParseFile() - Go AST from Dingo
│   ├── scanner/
│   │   └── scanner.go          # Extended scanner for Dingo tokens
│   └── token/
│       └── token.go            # Dingo token definitions (?, ??, ?.)
│
├── transpiler/                 # CLI ENTRY POINT
│   ├── transpiler.go           # TranspileFile with config
│   ├── pure_pipeline.go        # PureASTTranspile - uses ast.TransformSource()
│   └── ast_pipeline.go         # ASTTranspile - returns metadata
│
└── feature/                    # PLUGGABLE FEATURE SYSTEM
    ├── plugin.go               # Plugin interface
    ├── engine.go               # Feature engine
    └── builtin/                # Built-in plugins
        └── plugins.go          # Plugin implementations
```

---

## Pluggable Feature System

The pluggable feature system (`pkg/feature/`) allows language features to be:
- **Enabled/disabled** via `dingo.toml` configuration
- **Extended** with 3rd-party plugins (future: RPC-based)
- **Validated** for dependencies and conflicts

### Plugin Interface

```go
// pkg/feature/plugin.go
type Plugin interface {
    // Metadata
    Name() string        // e.g., "enum", "match", "lambdas"
    Version() string     // e.g., "1.0.0"
    Type() PluginType    // CharacterLevel or TokenLevel
    Priority() int       // Lower = runs earlier (10, 20, 30...)

    // Dependencies
    Dependencies() []string  // Must run after these (e.g., match → enum)
    Conflicts() []string     // Cannot run with these

    // Syntax detection (for disabled feature errors)
    Detect(src []byte) []SyntaxLocation

    // Transformation
    Transform(src []byte, ctx *Context) ([]byte, error)
}
```

### Built-in Plugins

| Plugin | Priority | Type | Description |
|--------|----------|------|-------------|
| `enum` | 10 | Character | `enum Name {...}` → Go interface pattern |
| `match` | 20 | Character | `match expr {...}` → IIFE type switch |
| `enum_constructors` | 30 | Character | `Variant()` → `NewVariant()` |
| `error_prop` | 40 | Character | `expr?` → error handling |
| `tuples` | 50 | Character | `(a, b) := fn()` → tuple destructuring |
| `safe_nav_statements` | 55 | Character | Statement-level `?.` |
| `safe_nav` | 60 | Character | Expression-level `?.` |
| `null_coalesce` | 70 | Character | `a ?? b` → nil checks |
| `lambdas` | 80 | Character | `\|x\| expr` and `x => expr` |
| `type_annotations` | 100 | Token | `param: Type` → `param Type` |
| `generics` | 110 | Token | `Result[T,E]` → `Result[T,E]` |

### Feature Configuration (dingo.toml)

```toml
[feature_matrix]
# All features enabled by default
# Only need to specify features you want to disable

# Character-level features
enum = true             # enum declarations
match = true            # match expressions
enum_constructors = true
error_prop = true       # ? operator
tuples = true           # tuple destructuring
safe_nav_statements = true
safe_nav = true         # ?. operator (set to false to disable)
null_coalesce = true    # ?? operator (set to false to disable)
lambdas = true          # |x| and => syntax

# Token-level features
type_annotations = true # x: Type syntax
generics = true         # <T> syntax
```

### Disabled Feature Detection

When a feature is disabled but its syntax is used, the transpiler reports a clear error:

```
error: feature 'lambdas' is disabled in configuration
  --> src/main.dingo:10:5
   |
10 |     add := |x, y| x + y
   |            ^^^^^^^^^^^^
   |
   = help: enable 'lambdas' in dingo.toml [feature_matrix] section
```

### Feature Engine

The engine (`pkg/feature/engine.go`) orchestrates plugin execution:

```go
// Create engine with feature configuration
engine, err := feature.NewEngine(enabledFeatures)
if err != nil {
    // Dependency error (e.g., match enabled but enum disabled)
    // Conflict error (if any plugins conflict)
    return err
}

// Transform source - runs all enabled plugins in priority order
src, err = engine.TransformCharacterLevel(src, filename)
if err != nil {
    // DisabledFeatureError if disabled syntax detected
    // TransformError if transformation fails
    return err
}
```

### Adding New Features (Future)

To add a new language feature:

1. **Create plugin** implementing `feature.Plugin` interface
2. **Register** via `feature.Register(&MyPlugin{})` in init()
3. **Add detection** logic in `Detect()` method
4. **Implement transformation** in `Transform()` method
5. **Add config field** in `FeatureMatrix` struct

For 3rd-party plugins (v1.1+), use RPC-based loading via HashiCorp's go-plugin.

---

## Features Implemented

### Fully Transformed to Go

| Feature | Dingo Syntax | Go Output |
|---------|--------------|-----------|
| **Type Annotations** | `func(x: int)` | `func(x int)` |
| **Generic Types** | `Result[T, E]` | `Result[T, E]` |
| **Error Propagation** | `x := expr?` | `tmp, err := expr; if err != nil { return err }; x := tmp` |
| **Tuples** | `(a, b) := fn()` | Multi-value assignment with destructuring |
| **Lambdas (Rust)** | `\|x\| x + 1` | `func(x) { return x + 1 }` |
| **Lambdas (TS)** | `(x) => x + 1` | `func(x) { return x + 1 }` |
| **Typed Lambdas** | `\|x: int\| x + 1` | `func(x int) { return x + 1 }` |
| **Enums** | `enum Status { Active, Inactive }` | Tagged union interface pattern |
| **Match** | `match expr { Pattern => result }` | IIFE with type switch |
| **Match Guards** | `Pattern if cond => result` | Nested if-else in case block |

### Partially Implemented (Markers Only)

| Feature | Current State |
|---------|---------------|
| **Null Coalescing** `??` | Leaves `/*DINGO_NULL_COAL*/` marker |
| **Safe Navigation** `?.` | Leaves `/*DINGO_SAFE_NAV*/` marker |

These markers are placeholders for future AST-level expansion.

---

## Transformation Examples

### Error Propagation (`?`)

**Input (Dingo):**
```go
func GetUser(id: int) (User, error) {
    data := fetchFromDB(id)?
    user := parseUser(data)?
    return user, nil
}
```

**Output (Go):**
```go
func GetUser(id int) (User, error) {
    tmp, err := fetchFromDB(id)
    if err != nil {
        return User{}, err
    }
    data := tmp

    tmp1, err1 := parseUser(data)
    if err1 != nil {
        return User{}, err1
    }
    user := tmp1

    return user, nil
}
```

### Match Expression

**Input (Dingo):**
```go
func describe(event: Event) string {
    return match event {
        UserCreated(id, email) => fmt.Sprintf("User %d: %s", id, email),
        UserDeleted(id) => fmt.Sprintf("Deleted user %d", id),
        _ => "Unknown event",
    }
}
```

**Output (Go):**
```go
func describe(event Event) string {
    return func() string {
        switch __matchVal := (event).(type) {
        case EventUserCreated:
            id := __matchVal.id
            email := __matchVal.email
            return fmt.Sprintf("User %d: %s", id, email)
        case EventUserDeleted:
            id := __matchVal.id
            return fmt.Sprintf("Deleted user %d", id)
        default:
            return "Unknown event"
        }
        panic("exhaustive match failed")
    }()
}
```

### Enum Declaration

**Input (Dingo):**
```go
enum Status {
    Active
    Inactive { reason: string }
}
```

**Output (Go):**
```go
type Status interface{ isStatus() }

type StatusActive struct{}
func (StatusActive) isStatus() {}
func NewStatusActive() Status { return StatusActive{} }

type StatusInactive struct{ reason string }
func (StatusInactive) isStatus() {}
func NewStatusInactive(reason string) Status {
    return StatusInactive{reason: reason}
}
```

---

## Entry Points

### For CLI (`dingo build`)

```go
// pkg/transpiler/pure_pipeline.go
func PureASTTranspile(source []byte, filename string) ([]byte, error)
```

Pipeline:
1. `ast.TransformSource()` → transforms Dingo to Go with source mappings
2. `go/parser.ParseFile()` → validates output
3. `go/printer` → formats output

### For Direct Transformation

```go
// pkg/ast/transform.go

// Transform Dingo source to Go with source mappings
func TransformSource(src []byte) ([]byte, []SourceMapping, error)

// SourceMapping tracks position mappings for LSP
type SourceMapping struct {
    DingoStart, DingoEnd int
    GoStart, GoEnd       int
    Kind                 string
}
```

### For Go AST Access

```go
// pkg/goparser/parser/parser.go

// Get Go AST (for LSP/further manipulation)
func ParseFile(fset *token.FileSet, filename string, src []byte, mode Mode) (*ast.File, error)
```

---

## Testing

### Unit Tests

Located in `pkg/goparser/parser/parser_test.go`:

| Test Suite | Coverage |
|------------|----------|
| `TestTransformTypeAnnotations` | Function params, methods, receivers |
| `TestParseFile` | Full file parsing, package names |
| `TestLambdaTransformation` | Rust-style, TS-style, block bodies |
| `TestEnumTransformation` | Simple enums, enums with fields |
| `TestMatchTransformation` | Default patterns, field extraction |
| `TestQuestionMarkInStrings` | SQL queries, string literals |
| `TestWildcardBindings` | `_` binding skip extraction |

### Running Tests

```bash
# Parser tests
go test ./pkg/goparser/parser/... -v

# All tests
go test ./...
```

### End-to-End Testing

```bash
# Build and compile example
go run ./cmd/dingo build examples/01_error_propagation/http_handler.dingo
go build examples/01_error_propagation/http_handler.go
```

---

## Key Design Decisions

### Why Character-Level Passes?

Some features (enum, match, lambda) require understanding context that spans multiple tokens or includes syntax Go's scanner sees as invalid.

Character-level passes:
- Handle `{...}` brace balancing manually
- Track string literals to avoid false transformations
- Generate complete replacement code before tokenization

### Why Token-Level for Simple Features?

Type annotations (`param: Type`) and generics (`Result[T,E]`) are simple token-to-token transformations. Using Go's scanner ensures:
- Correct handling of string literals
- Proper position tracking
- No false positives in comments

### Why Not Full Custom Parser?

- Go's parser is battle-tested
- Grammar maintenance is complex
- We only need to transform **before** parsing, not replace parsing
- go/parser handles edge cases we'd have to reimplement

---

## Future Work

### Null Coalescing (`??`)

Currently leaves `/*DINGO_NULL_COAL*/` marker. Full transformation:

```go
// Input
x := getValue() ?? defaultValue

// Output
x := func() T {
    tmp := getValue()
    if tmp != nil {
        return tmp
    }
    return defaultValue
}()
```

### Safe Navigation (`?.`)

Currently leaves `/*DINGO_SAFE_NAV*/` marker. Full transformation:

```go
// Input
name := user?.profile?.name ?? "Anonymous"

// Output
name := func() string {
    if user == nil { return "Anonymous" }
    if user.profile == nil { return "Anonymous" }
    return user.profile.name
}()
```

### Source Maps

`TokenMapping` is generated but not yet serialized:

```go
type TokenMapping struct {
    OriginalLine int
    OriginalCol  int
    GeneratedLine int
    GeneratedCol  int
}
```

Future: Generate `.sourcemap` files for LSP integration.

### LSP Integration

Using templ's gopls proxy pattern:

```go
type Server struct {
    gopls      *subprocess
    sourceMaps map[string]*SourceMap
}

func (s *Server) Handle(req *protocol.Request) (*protocol.Response, error) {
    translated := s.translateRequest(req)
    resp := s.gopls.Handle(translated)
    return s.translateResponse(resp)
}
```

---

## Comparison to Previous Architectures

### Old: Regex-Based Preprocessor (`pkg/preprocessor/`)

```
❌ Fragile pattern matching
❌ Position tracking errors accumulate
❌ Hard to debug complex patterns
❌ Order-dependent pass system
❌ No AST awareness
```

**Status**: DELETED (November 2025)

### Old: Token-Based Transform (`TransformToGo()`)

```
❌ No source mapping support
❌ Tight coupling between parsing and generation
❌ Hard to extend with new features
```

**Status**: REMOVED (December 2025)

### Current: AST-Based Code Generation (`pkg/ast/`)

```
✅ Each feature is a separate parser + codegen
✅ Source mappings returned by each transform
✅ Modular and testable components
✅ Easy to add new features
✅ Unified pipeline with transform.go
✅ Falls through to standard go/parser
```

**Status**: CURRENT (December 2025)

---

## Semantic Analysis Strategy: gopls Proxy

### The Decision

**Dingo delegates semantic analysis to gopls** instead of building a custom type checker.

This is a deliberate architectural choice based on:
1. Cost/benefit analysis
2. Comparison with similar projects (Borgo, templ, TypeScript)
3. Dingo's goals (syntax sugar, not a new type system)

### Dingo vs Borgo: Why Different Approaches

| Aspect | Borgo | Dingo |
|--------|-------|-------|
| **Goal** | New language with Rust-like type system | Syntax sugar for Go |
| **Type System** | Traits, Hindley-Milner inference | Go's type system unchanged |
| **After Transform** | Still needs Borgo semantics | Pure Go - gopls works |
| **Type Checker** | Must build own (50K+ LOC) | Use gopls |

**Borgo** adds concepts Go doesn't have (traits, algebraic types as first-class). gopls can't understand Borgo's types, so Borgo must build its own type checker.

**Dingo** transforms to idiomatic Go. After transformation:
- `Result[T,E]` becomes `Result[T,E]` (Go generic)
- `enum Status {...}` becomes interface pattern
- `x?` becomes `if err != nil { return err }`

gopls can analyze all of this perfectly.

### The Hybrid Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     DINGO ARCHITECTURE                       │
├─────────────────────────────────────────────────────────────┤
│  .dingo file                                                │
│      │                                                       │
│      ▼                                                       │
│  ┌──────────────────────────────────────┐                   │
│  │  Dingo Parser (pkg/goparser/)        │  ← DINGO OWNS     │
│  │  - Custom tokenizer (?, ??, enum)    │                   │
│  │  - Pratt expression parser           │                   │
│  │  - Character-level transformations   │                   │
│  └──────────────────────────────────────┘                   │
│      │                                                       │
│      ▼                                                       │
│  ┌──────────────────────────────────────┐                   │
│  │  Dingo-Only Semantic Checks          │  ← DINGO OWNS     │
│  │  - Pattern exhaustiveness            │    (minimal)      │
│  │  - Enum variant validation           │                   │
│  │  - ? operator context check          │                   │
│  └──────────────────────────────────────┘                   │
│      │                                                       │
│      ▼                                                       │
│  ┌──────────────────────────────────────┐                   │
│  │  Transform → .go + .sourcemap        │  ← DINGO OWNS     │
│  └──────────────────────────────────────┘                   │
│      │                                                       │
│      ▼                                                       │
│  ┌──────────────────────────────────────┐                   │
│  │  gopls (via LSP proxy)               │  ← DELEGATE       │
│  │  - Full Go type checking             │                   │
│  │  - Symbol resolution                 │                   │
│  │  - All IDE features                  │                   │
│  └──────────────────────────────────────┘                   │
│      │                                                       │
│      ▼                                                       │
│  ┌──────────────────────────────────────┐                   │
│  │  LSP Proxy (dingo-lsp)               │  ← DINGO OWNS     │
│  │  - Translate positions via sourcemap │                   │
│  │  - Translate error messages          │                   │
│  │  - Merge Dingo-specific diagnostics  │                   │
│  └──────────────────────────────────────┘                   │
└─────────────────────────────────────────────────────────────┘
```

### What Dingo Builds vs Delegates

**Dingo builds** (minimal semantic analysis):

| Check | Why Dingo Must Do It |
|-------|---------------------|
| Pattern exhaustiveness | Go switch doesn't enforce this |
| Enum variant validation | Dingo-specific construct |
| `?` operator context | Must be in Result-returning function |
| Error message translation | Make gopls errors Dingo-native |

**Dingo delegates to gopls**:

| Feature | Delegated To |
|---------|-------------|
| Type checking | gopls |
| Symbol resolution | gopls |
| Import resolution | gopls |
| Interface satisfaction | gopls |
| Generic inference | gopls |
| Autocomplete | gopls |
| Go-to-definition | gopls |
| Find references | gopls |
| Rename refactoring | gopls |
| Hover information | gopls |

### Cost/Benefit Analysis

| Factor | Build Own Type Checker | Use gopls Proxy |
|--------|------------------------|-----------------|
| Engineering effort | 50,000+ LOC, 18-24 months | 5,000-10,000 LOC |
| Maintenance | 1-2 FTE to track Go evolution | Minimal |
| Go compatibility | Risk of drift | Automatic |
| IDE feature parity | Years to match gopls | Immediate |
| Error messages | Full control | Requires translation layer |

### Similar Projects

**templ** (Go templating) uses the same approach:
- Transforms `.templ` → `.go` with source maps
- LSP proxy wraps gopls
- Users get full IDE features via position translation

Quote from Go Time podcast:
> "The LSP can piggyback on top of gopls and provide all the features that gopls provides by just remapping the source code locations."

### Summary

Dingo's value is **syntax and ergonomics**, not a new type system. Building a Go type checker would mean reimplementing:
- `go/types`: 30,000+ LOC
- gopls analysis: 100,000+ LOC

That's not where Dingo should invest. Focus on:
1. Excellent Dingo parser ✅ (done)
2. Accurate source maps 🔄 (in progress)
3. Smooth gopls proxy 🔄 (in progress)
4. Minimal Dingo-specific checks 📋 (planned)

---

## References

### External Projects

- **TypeScript** - Transpiler architecture reference
- **Borgo** (github.com/borgo-lang/borgo) - Rust-like → Go transpiler (built own type checker)
- **templ** (github.com/a-h/templ) - gopls proxy pattern (Dingo follows this)

### Go Standard Library

- `go/scanner` - Tokenization
- `go/parser` - Parsing
- `go/token` - Position tracking
- `go/ast` - AST manipulation
- `go/printer` - Code output

### Research

- [Go Time Podcast #291](https://changelog.com/gotime/291) - templ's gopls proxy architecture
- [Borgo GitHub](https://github.com/borgo-lang/borgo) - Why Borgo needed its own type checker

---

**Questions or concerns about the architecture?**
Open an issue: https://github.com/MadAppGang/dingo/issues
