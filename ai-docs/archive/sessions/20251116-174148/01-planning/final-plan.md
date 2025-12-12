# Final Implementation Plan: Error Propagation with Configurable Syntax
**Session:** 20251116-174148
**Date:** 2025-11-16
**Phase:** Phase 1.5 - Error Handling Foundation (Enhanced)
**Timeline:** 3-4 weeks

---

## Executive Summary

This plan implements **configurable error propagation syntax** (`?`, `!`, `try`) with **source maps** and **real-world Go stdlib testing**. This expanded scope delivers:

1. **User Choice** - Developers select their preferred syntax via configuration
2. **Production Quality** - Source maps enable .dingo error messages from day one
3. **Proven Reliability** - Real stdlib integration validates production-readiness

**Key Additions from Initial Plan:**
- Configuration system for syntax feature switching
- Complete source map generation and integration
- Real-world testing with Go stdlib packages (http, database/sql, os)
- Updated timeline: 3-4 weeks (vs 2-3 weeks)

---

## Architecture Overview

### Three Core Components

```
Error Propagation System
├── 1. Syntax Configuration Layer
│   ├── dingo.toml config file
│   ├── Feature flag system
│   └── Syntax-specific parsers
│
├── 2. Transformation Pipeline (Syntax-Agnostic)
│   ├── ErrorPropagationPlugin (core logic)
│   ├── AST transformation (shared)
│   └── Go code generation
│
└── 3. Source Map Integration
    ├── Position tracking during parsing
    ├── Source map generation (.dingo → .go)
    ├── Error message translation
    └── IDE integration hooks
```

---

## Part 1: Configurable Syntax System

### Design Approach: Unified AST, Multiple Parsers

**Strategy:** All three syntaxes (`?`, `!`, `try`) transform to the same AST node (`ErrorPropagationExpr`). The parser layer handles syntax detection based on configuration.

### Configuration File (`dingo.toml`)

```toml
[features]
# Error propagation syntax: "question" | "bang" | "try"
error_propagation_syntax = "question"

[sourcemaps]
enabled = true
format = "inline"  # "inline" | "separate" | "both"
```

### Implementation Architecture

```go
// pkg/config/config.go
package config

type FeatureConfig struct {
    ErrorPropagationSyntax SyntaxStyle `toml:"error_propagation_syntax"`
}

type SyntaxStyle string

const (
    SyntaxQuestion SyntaxStyle = "question"  // expr?
    SyntaxBang     SyntaxStyle = "bang"      // expr!
    SyntaxTry      SyntaxStyle = "try"       // try expr
)

// pkg/parser/parser.go
type ParserConfig struct {
    SyntaxStyle config.SyntaxStyle
}

func NewParser(cfg ParserConfig) Parser {
    switch cfg.SyntaxStyle {
    case config.SyntaxQuestion:
        return NewQuestionParser()
    case config.SyntaxBang:
        return NewBangParser()
    case config.SyntaxTry:
        return NewTryParser()
    default:
        return NewQuestionParser() // Default
    }
}
```

### Syntax-Specific Parsers

All parsers output the same AST node:

```go
// pkg/ast/ast.go (already exists)
type ErrorPropagationExpr struct {
    Expr     Expr
    Syntax   SyntaxStyle  // NEW: Track which syntax was used
    Position token.Pos
}
```

#### Question Syntax Parser (`?`)

```go
// pkg/parser/question_syntax.go
type PostfixExpr struct {
    Primary ast.Expr
    Ops     []PostfixOp
}

type PostfixOp struct {
    Question *QuestionOp `@@?`
}

type QuestionOp struct {
    Pos token.Pos `parser:""`
    Op  string    `parser:"@'?'"`
}
```

#### Bang Syntax Parser (`!`)

```go
// pkg/parser/bang_syntax.go
type PostfixExpr struct {
    Primary ast.Expr
    Ops     []PostfixOp
}

type PostfixOp struct {
    Bang *BangOp `@@?`
}

type BangOp struct {
    Pos token.Pos `parser:""`
    Op  string    `parser:"@'!'"`
}
```

#### Try Syntax Parser (`try`)

```go
// pkg/parser/try_syntax.go
type TryExpr struct {
    Pos  token.Pos `parser:""`
    Try  string    `parser:"@'try'"`
    Expr ast.Expr  `parser:"@@"`
}
```

### Syntax Decision Rationale

| Syntax | Precedent | Pros | Cons |
|--------|-----------|------|------|
| `expr?` | Rust, Kotlin, Swift | Concise, postfix, proven | Visual confusion with ternary |
| `expr!` | Swift (force unwrap) | Distinguishable from `?` | Conflicts with `!` negation operator |
| `try expr` | Swift, Zig | Clear intent, keyword | More verbose, prefix style |

**Default Choice:** `?` (question) - Most widely adopted, concise, postfix consistency

**Feature Switching Benefit:** Teams can choose based on their preferences/backgrounds

---

## Part 2: Source Map Implementation

### Why Source Maps Now?

**User Benefit:** Error messages point to `.dingo` source, not generated `.go`

**Example Without Source Maps:**
```
main.go:15: undefined: fetchUser
```

**Example With Source Maps:**
```
main.dingo:8: undefined: fetchUser
    user := fetchUser(id)?
               ^^^^^^^^^
```

### Source Map Format: VLQ-Based (TypeScript/Babel Standard)

```json
{
  "version": 3,
  "file": "main.go",
  "sourceRoot": "",
  "sources": ["main.dingo"],
  "names": [],
  "mappings": "AAAA;AACA,SAASA,UAAUC,IAAIC;IACrB,IAAIC"
}
```

### Implementation Architecture

```go
// pkg/sourcemap/sourcemap.go
package sourcemap

import "github.com/go-sourcemap/sourcemap"

type Generator struct {
    source   string      // Original .dingo file
    generated string     // Generated .go file
    mappings []Mapping
}

type Mapping struct {
    SourceLine   int
    SourceColumn int
    GenLine      int
    GenColumn    int
}

func (g *Generator) AddMapping(src, gen token.Position) {
    g.mappings = append(g.mappings, Mapping{
        SourceLine:   src.Line,
        SourceColumn: src.Column,
        GenLine:      gen.Line,
        GenColumn:    gen.Column,
    })
}

func (g *Generator) Generate() ([]byte, error) {
    // Convert to VLQ format using github.com/go-sourcemap/sourcemap
}
```

### Integration Points

#### 1. Parser: Capture Original Positions

```go
// pkg/parser/participle.go
func (p *Parser) Parse(src []byte) (*ast.File, error) {
    // Participle automatically captures token.Pos
    // Store in AST nodes
}
```

#### 2. Transformer: Track Position Mappings

```go
// pkg/plugin/builtin/error_propagation.go
func (p *ErrorPropagationPlugin) Transform(
    ctx *plugin.Context,
    node ast.Node,
) (ast.Node, error) {
    if expr, ok := node.(*dingoast.ErrorPropagationExpr); ok {
        // Original position from expr.Position
        originalPos := ctx.Fset.Position(expr.Pos())

        // Generate Go code
        goCode := p.generateErrorCheck(expr)

        // Record mapping
        ctx.SourceMap.AddMapping(originalPos, goCode.Pos())

        return goCode, nil
    }
}
```

#### 3. Generator: Emit Source Map

```go
// pkg/generator/generator.go
func (g *Generator) GenerateWithSourceMap(
    file *ast.File,
    sourceMap *sourcemap.Generator,
) ([]byte, []byte, error) {
    // Generate .go file
    goCode := g.Generate(file)

    // Generate .go.map file
    mapData, err := sourceMap.Generate()

    return goCode, mapData, err
}
```

### Source Map Storage Options

**Option 1: Inline Comment (Recommended for MVP)**
```go
// Generated by Dingo v0.1.0
//go:sourcemap eyJ2ZXJzaW9uIjozLCJmaWxlIjoibWFpbi5nbyIsInNvdXJjZXMiOlsibWFpbi5kaW5nbyJd...
package main
```

**Option 2: Separate File**
```
main.go      # Generated Go code
main.go.map  # Source map
```

**Option 3: Both (Configurable)**
- Development: Inline (convenience)
- Production: Separate (clean generated code)

### Error Message Translation

```go
// pkg/errors/translator.go
package errors

func TranslateGoError(goErr error, sourceMap *sourcemap.Consumer) error {
    // Parse Go compiler error
    // Example: "main.go:15:10: undefined: fetchUser"
    goFile, goLine, goCol := parseGoError(goErr)

    // Lookup in source map
    dingoPos, err := sourceMap.Source(goLine, goCol)
    if err != nil {
        return goErr // Fallback to original
    }

    // Rewrite error message
    return fmt.Errorf("%s:%d:%d: %s",
        dingoPos.Source,
        dingoPos.Line,
        dingoPos.Column,
        extractErrorMessage(goErr),
    )
}
```

### Dependencies

```go
// go.mod additions
require (
    github.com/go-sourcemap/sourcemap v2.1.3+incompatible
)
```

---

## Part 3: Real-World Go Stdlib Testing

### Testing Philosophy

**Goal:** Prove Dingo works with real Go packages, not just synthetic examples.

**Approach:** Test actual error-returning functions from Go stdlib:
- `net/http`: Client requests, server handlers
- `database/sql`: Query operations, transactions
- `os`: File operations, environment access
- `encoding/json`: Marshaling, unmarshaling
- `io`: Reader/Writer operations

### Test Categories

#### Category 1: HTTP Client Operations

```dingo
// tests/stdlib/http_client.dingo
import "net/http"
import "io"

func fetchURL(url: string) (string, error) {
    resp := http.Get(url)?              // Question syntax
    defer resp.Body.Close()

    body := io.ReadAll(resp.Body)?      // Chained error propagation
    return string(body), nil
}

// Should transpile to:
func fetchURL(url string) (string, error) {
    __tmp0, __err0 := http.Get(url)
    if __err0 != nil {
        return "", __err0
    }
    resp := __tmp0
    defer resp.Body.Close()

    __tmp1, __err1 := io.ReadAll(resp.Body)
    if __err1 != nil {
        return "", __err1
    }
    body := __tmp1

    return string(body), nil
}
```

#### Category 2: Database Operations

```dingo
// tests/stdlib/database.dingo
import "database/sql"

func queryUser(db: *sql.DB, id: int) (*User, error) {
    row := db.QueryRow("SELECT name, email FROM users WHERE id = ?", id)

    var user User
    err := row.Scan(&user.Name, &user.Email)?
    return &user, nil
}
```

#### Category 3: File Operations

```dingo
// tests/stdlib/file_ops.dingo
import "os"
import "encoding/json"

func loadConfig(path: string) (*Config, error) {
    data := os.ReadFile(path)?

    var config Config
    err := json.Unmarshal(data, &config)?
    return &config, nil
}
```

#### Category 4: Complex Real-World Scenario

```dingo
// tests/stdlib/real_world_handler.dingo
import (
    "net/http"
    "encoding/json"
    "database/sql"
)

func handleCreateUser(
    w: http.ResponseWriter,
    r: *http.Request,
    db: *sql.DB,
) {
    // Parse request
    body := io.ReadAll(r.Body)?
    defer r.Body.Close()

    var req CreateUserRequest
    err := json.Unmarshal(body, &req)?

    // Validate
    validated := validateUser(&req)?

    // Database transaction
    tx := db.Begin()?
    defer tx.Rollback()

    result := tx.Exec(
        "INSERT INTO users (name, email) VALUES (?, ?)",
        validated.Name,
        validated.Email,
    )?

    err := tx.Commit()?

    // Response
    id := result.LastInsertId()?
    json.NewEncoder(w).Encode(map[string]int64{"id": id})
}
```

### Test Validation Strategy

**Step 1: Transpilation**
```bash
dingo build tests/stdlib/*.dingo
```

**Step 2: Go Compilation**
```bash
go build tests/stdlib/*.go
# Must compile without errors
```

**Step 3: Runtime Testing**
```bash
go test tests/stdlib/*.go
# Real HTTP requests, actual file I/O
```

**Step 4: Source Map Validation**
```bash
# Intentionally introduce error in .dingo
# Verify error points to .dingo file, not .go
dingo build tests/stdlib/http_client.dingo
# Expected: http_client.dingo:5: undefined: httpGet
# Not: http_client.go:12: undefined: httpGet
```

### Test Organization

```
tests/
├── stdlib/
│   ├── http_client_test.dingo
│   ├── http_server_test.dingo
│   ├── database_query_test.dingo
│   ├── database_transaction_test.dingo
│   ├── file_ops_test.dingo
│   ├── json_ops_test.dingo
│   ├── io_operations_test.dingo
│   └── real_world_handler_test.dingo
│
├── golden/
│   ├── http_client.go        # Expected output
│   ├── http_client.go.map    # Expected source map
│   └── ...
│
└── integration/
    └── stdlib_integration_test.go  # Go test harness
```

### Integration Test Harness

```go
// tests/integration/stdlib_integration_test.go
package integration

import (
    "testing"
    "net/http/httptest"
    "database/sql"
    _ "github.com/mattn/go-sqlite3"
)

func TestHTTPClientErrorPropagation(t *testing.T) {
    // Mock HTTP server
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("test response"))
    }))
    defer server.Close()

    // Call transpiled Dingo function
    result, err := fetchURL(server.URL)
    if err != nil {
        t.Fatalf("Expected no error, got: %v", err)
    }

    if result != "test response" {
        t.Errorf("Expected 'test response', got '%s'", result)
    }
}

func TestDatabaseErrorPropagation(t *testing.T) {
    // In-memory SQLite
    db, err := sql.Open("sqlite3", ":memory:")
    if err != nil {
        t.Fatal(err)
    }
    defer db.Close()

    // Setup schema
    _, err = db.Exec("CREATE TABLE users (id INTEGER, name TEXT, email TEXT)")
    if err != nil {
        t.Fatal(err)
    }

    // Call transpiled Dingo function
    user, err := queryUser(db, 1)
    // Test error propagation behavior
}
```

---

## Implementation Timeline (3-4 Weeks)

### Week 1: Foundation + Configuration System

**Days 1-2: Configuration Infrastructure**
- [ ] Implement `dingo.toml` parsing
- [ ] Create `pkg/config` package
- [ ] Add syntax style enum
- [ ] CLI flag for config path
- [ ] Default configuration

**Days 3-5: Parser Architecture**
- [ ] Refactor parser to support multiple syntaxes
- [ ] Implement question syntax parser (`?`)
- [ ] Implement bang syntax parser (`!`)
- [ ] Implement try syntax parser (`try`)
- [ ] Parser factory based on config
- [ ] Unit tests for all syntaxes

**Days 6-7: AST Enhancements**
- [ ] Add `Syntax` field to `ErrorPropagationExpr`
- [ ] Position tracking improvements
- [ ] AST node tests
- [ ] Documentation

**Deliverable:** All three syntaxes parse correctly to unified AST

---

### Week 2: Transformation Plugin + Source Maps

**Days 8-10: Error Propagation Plugin**
- [ ] Implement `ErrorPropagationPlugin`
- [ ] Syntax-agnostic transformation logic
- [ ] Tuple unpacking (T, error) → temp vars
- [ ] Error return generation
- [ ] Statement vs expression context handling
- [ ] Plugin tests

**Days 11-13: Source Map Generation**
- [ ] Integrate `github.com/go-sourcemap/sourcemap`
- [ ] Position tracking in parser
- [ ] Mapping collection during transformation
- [ ] VLQ encoding generation
- [ ] Inline vs separate file output
- [ ] Source map tests

**Day 14: Integration**
- [ ] Wire source map into transpiler pipeline
- [ ] Generator emits .go + .go.map files
- [ ] CLI options for source map format
- [ ] End-to-end pipeline test

**Deliverable:** Working transformation with source maps

---

### Week 3: Real-World Testing + Type Validation

**Days 15-17: Go Stdlib Test Suite**
- [ ] HTTP client tests (`net/http`)
- [ ] Database tests (`database/sql`)
- [ ] File operation tests (`os`)
- [ ] JSON operation tests (`encoding/json`)
- [ ] IO operation tests (`io`)
- [ ] Real-world handler scenario
- [ ] All tests transpile and compile
- [ ] All tests execute successfully

**Days 18-20: Type Analysis & Validation**
- [ ] Basic type inference (heuristic)
- [ ] Validate `?`/`!`/`try` on (T, error) returns
- [ ] Error messages for invalid usage
- [ ] Integration with Go's `go/types`
- [ ] Validation tests

**Day 21: Error Message Translation**
- [ ] Implement source map consumer
- [ ] Go error → Dingo error translation
- [ ] CLI integration for error display
- [ ] Test error message accuracy

**Deliverable:** Production-ready error propagation with stdlib validation

---

### Week 4: Polish, Documentation, Edge Cases

**Days 22-24: Edge Cases & Refinement**
- [ ] Nested error propagation (`f()?.g()?.h()?`)
- [ ] Error propagation in return statements
- [ ] Error propagation in complex expressions
- [ ] Multi-syntax project support (detection)
- [ ] Performance optimization
- [ ] Golden file tests for all scenarios

**Days 25-26: Documentation**
- [ ] User guide: `docs/features/error-propagation.md`
- [ ] Configuration guide: `docs/configuration.md`
- [ ] Syntax comparison guide
- [ ] Migration from Go guide
- [ ] API documentation
- [ ] Implementation notes: `ai-docs/error-propagation-impl.md`

**Days 27-28: Final Testing & Release Prep**
- [ ] Comprehensive test run
- [ ] Performance benchmarks
- [ ] Memory profiling
- [ ] Example projects
- [ ] Changelog update
- [ ] Release notes

**Deliverable:** Production-ready feature with complete documentation

---

## Updated Package Structure

```
dingo/
├── cmd/dingo/
│   └── main.go             # CLI with config loading
│
├── pkg/
│   ├── config/             # ✨ NEW: Configuration system
│   │   ├── config.go       # Config structs
│   │   ├── loader.go       # TOML parsing
│   │   └── defaults.go     # Default values
│   │
│   ├── ast/
│   │   └── ast.go          # ErrorPropagationExpr (enhanced)
│   │
│   ├── parser/
│   │   ├── parser.go       # Parser interface
│   │   ├── factory.go      # ✨ NEW: Syntax-based parser factory
│   │   ├── question.go     # ✨ NEW: ? syntax
│   │   ├── bang.go         # ✨ NEW: ! syntax
│   │   └── try.go          # ✨ NEW: try syntax
│   │
│   ├── sourcemap/          # ✨ NEW: Source map generation
│   │   ├── generator.go    # Mapping collection
│   │   ├── consumer.go     # Mapping lookup
│   │   └── vlq.go          # VLQ encoding
│   │
│   ├── plugin/
│   │   └── builtin/
│   │       └── error_propagation.go  # Transformation logic
│   │
│   ├── typechecker/        # ✨ NEW: Type validation
│   │   └── validator.go    # Basic type checking
│   │
│   └── errors/             # ✨ NEW: Error translation
│       └── translator.go   # Go error → Dingo error
│
├── tests/
│   ├── stdlib/             # ✨ NEW: Real stdlib tests
│   │   ├── http_*.dingo
│   │   ├── database_*.dingo
│   │   └── file_*.dingo
│   │
│   ├── golden/             # Expected outputs
│   │   ├── *.go
│   │   └── *.go.map
│   │
│   └── integration/        # Go test harness
│       └── stdlib_test.go
│
├── examples/
│   └── error_propagation/
│       ├── question_syntax/
│       ├── bang_syntax/
│       └── try_syntax/
│
└── dingo.toml              # ✨ NEW: Project configuration
```

---

## Configuration System Architecture

### Configuration Precedence

1. **CLI flags** (highest priority)
   ```bash
   dingo build --syntax=bang main.dingo
   ```

2. **Project `dingo.toml`**
   ```toml
   [features]
   error_propagation_syntax = "question"
   ```

3. **User config** (`~/.dingo/config.toml`)
   ```toml
   [defaults]
   error_propagation_syntax = "question"
   ```

4. **Built-in defaults** (lowest priority)
   - Default: `question` syntax

### Implementation

```go
// pkg/config/loader.go
package config

import "github.com/BurntSushi/toml"

type Config struct {
    Features  FeatureConfig  `toml:"features"`
    SourceMap SourceMapConfig `toml:"sourcemaps"`
}

type FeatureConfig struct {
    ErrorPropagationSyntax SyntaxStyle `toml:"error_propagation_syntax"`
}

type SourceMapConfig struct {
    Enabled bool   `toml:"enabled"`
    Format  string `toml:"format"` // "inline" | "separate" | "both"
}

func Load(paths ...string) (*Config, error) {
    // Load from multiple sources with precedence
    cfg := DefaultConfig()

    for _, path := range paths {
        if _, err := toml.DecodeFile(path, &cfg); err != nil {
            // Ignore missing files, return parse errors
            if !os.IsNotExist(err) {
                return nil, err
            }
        }
    }

    return cfg, nil
}
```

### CLI Integration

```go
// cmd/dingo/build.go
var buildCmd = &cobra.Command{
    Use: "build [files...]",
    Run: func(cmd *cobra.Command, args []string) {
        // Load config
        cfg, err := config.Load(
            filepath.Join(os.Getenv("HOME"), ".dingo", "config.toml"),
            "dingo.toml",
        )

        // Override with CLI flags
        if syntaxFlag := cmd.Flag("syntax").Value.String(); syntaxFlag != "" {
            cfg.Features.ErrorPropagationSyntax = config.SyntaxStyle(syntaxFlag)
        }

        // Create parser with config
        parser := parser.NewParser(parser.Config{
            SyntaxStyle: cfg.Features.ErrorPropagationSyntax,
        })

        // Build...
    },
}

func init() {
    buildCmd.Flags().String("syntax", "", "Error propagation syntax (question|bang|try)")
}
```

---

## Risk Analysis & Mitigation

### New Risks from Expanded Scope

| Risk | Probability | Impact | Mitigation |
|------|------------|--------|------------|
| **Configuration complexity** | Medium | Medium | Use proven library (BurntSushi/toml), simple schema |
| **Parser factory bugs** | Medium | High | Comprehensive unit tests for each syntax, shared test cases |
| **Source map generation errors** | High | High | Use battle-tested library (go-sourcemap), extensive testing |
| **Source map consumer integration** | Medium | High | Incremental implementation, fallback to original errors |
| **Multi-syntax confusion** | Low | Medium | Clear docs, error messages indicate syntax in use |
| **Stdlib test failures** | High | Medium | Start with simple cases, iterate, accept Go compiler as validator |
| **Timeline overrun** | Medium | Low | Built-in buffer (3-4 weeks), features can be staged |

### Critical Path Items

1. **Source Map Library Evaluation** (Day 1)
   - Validate `github.com/go-sourcemap/sourcemap` works for our use case
   - Spike: Generate simple source map and consume it
   - Fallback: Implement minimal VLQ encoder ourselves

2. **Configuration System** (Days 1-2)
   - Prove config loading works across CLI/project/user levels
   - Test precedence logic
   - Ensure parser factory integrates smoothly

3. **First Stdlib Test** (Day 15)
   - Prove we can transpile real Go stdlib usage
   - Validate error propagation works end-to-end
   - If blocked, adjust approach

---

## Success Metrics

### Technical Success Criteria

- [ ] All three syntaxes (`?`, `!`, `try`) parse and transform correctly
- [ ] Configuration system works: CLI > project > user > defaults
- [ ] Source maps generated for all transpiled files
- [ ] Error messages correctly point to `.dingo` files (90%+ accuracy)
- [ ] 10+ real Go stdlib packages successfully tested
- [ ] All stdlib integration tests pass (transpile → compile → run)
- [ ] 90%+ test coverage on parser, plugin, source map modules
- [ ] Zero performance regression vs manual error handling
- [ ] Generated Go code is readable and idiomatic

### User Experience Success Criteria

- [ ] Syntax choice is clear and documented
- [ ] Configuration is intuitive (sensible defaults)
- [ ] Error messages are actionable and point to source
- [ ] Works with existing Go projects (no friction)
- [ ] Examples demonstrate real-world value
- [ ] Can explain feature in <5 minutes
- [ ] Migration path from Go is straightforward

### Validation Metrics

- [ ] Reduces error handling code by 50-60% in examples
- [ ] Source map accuracy >90% for error translation
- [ ] All stdlib tests compile without transpilation errors
- [ ] Runtime behavior matches manual Go error handling
- [ ] Documentation completeness (user + implementation guides)

---

## Dependencies

### External Go Packages

```go
// go.mod additions
require (
    github.com/alecthomas/participle/v2 v2.1.1  // Parser (existing)
    github.com/BurntSushi/toml v1.3.2           // Config (new)
    github.com/go-sourcemap/sourcemap v2.1.3+incompatible  // Source maps (new)
    golang.org/x/tools v0.17.0                  // go/types (existing)
)
```

### Internal Dependencies

- Existing plugin system (built in previous phase)
- Existing AST definitions (`ErrorPropagationExpr`)
- Existing generator with plugin support
- Existing CLI infrastructure

### Knowledge Dependencies

1. **Source Map Specification** - Study VLQ encoding format
2. **Participle Grammar Extension** - Multiple syntax support
3. **Go Type System** - Basic type inference for validation
4. **Go Stdlib API Surface** - Understand common error-returning patterns

---

## Testing Strategy

### Test Pyramid

```
         /\
        /E2\        End-to-End (5%)
       /────\       - Full pipeline tests
      /Integration\ Integration (15%)
     /────────────\ - Stdlib tests
    /   Unit Tests \ Unit (80%)
   /────────────────\ - Parser, plugin, source map
```

### Unit Tests (80% of tests)

```go
// pkg/parser/question_test.go
func TestQuestionSyntaxParser(t *testing.T) {
    tests := []struct {
        input    string
        expected *ast.ErrorPropagationExpr
    }{
        {
            input: "fetchUser(id)?",
            expected: &ast.ErrorPropagationExpr{
                Expr: &ast.CallExpr{...},
                Syntax: config.SyntaxQuestion,
            },
        },
        // ... more cases
    }
}

// pkg/sourcemap/generator_test.go
func TestSourceMapGeneration(t *testing.T) {
    // Test mapping accuracy
}

// pkg/plugin/builtin/error_propagation_test.go
func TestErrorPropagationTransform(t *testing.T) {
    // Test AST transformation
}
```

### Integration Tests (15% of tests)

```go
// tests/integration/stdlib_test.go
func TestRealWorldHTTPClient(t *testing.T) {
    // Transpile → Compile → Run
}
```

### End-to-End Tests (5% of tests)

```go
// tests/e2e/full_pipeline_test.go
func TestFullPipeline(t *testing.T) {
    // .dingo → .go → compile → execute → validate output
}
```

---

## Documentation Plan

### User-Facing Documentation

#### 1. Feature Guide (`docs/features/error-propagation.md`)
- What is error propagation?
- Why use it?
- Syntax options comparison
- Usage examples
- Common patterns
- Migration from Go

#### 2. Configuration Guide (`docs/configuration.md`)
- `dingo.toml` schema
- Syntax selection
- Source map options
- Precedence rules
- Examples

#### 3. Syntax Comparison Guide (`docs/syntax-guide.md`)
- Side-by-side comparison of `?`, `!`, `try`
- When to choose each
- Team conventions
- Examples in each syntax

#### 4. Stdlib Integration Guide (`docs/stdlib-usage.md`)
- Using with `net/http`
- Using with `database/sql`
- Using with `os` and `io`
- Best practices
- Gotchas

### AI/Developer Documentation

#### 1. Implementation Guide (`ai-docs/error-propagation-impl.md`)
- Architecture decisions
- Transformation algorithm
- Source map implementation
- Parser factory pattern
- Testing strategy
- Future enhancements

#### 2. Source Map Specification (`ai-docs/sourcemap-spec.md`)
- VLQ encoding details
- Mapping format
- Consumer implementation
- Error translation algorithm

---

## Future Enhancements (Post-Phase 1.5)

### Phase 2: Result Type Integration

Enhance error propagation to work with `Result[T, E]` types:

```dingo
func fetchUser(id: string) -> Result[User, Error] {
    // ...
}

func processUser(id: string) -> Result[User, Error] {
    user := fetchUser(id)?  // Unwraps Result
    return Ok(user)
}
```

### Phase 3: Error Context & Wrapping

Automatic error context injection:

```dingo
user := fetchUser(id)? wrap "failed to fetch user {id}"

// Generates:
if err != nil {
    return fmt.Errorf("failed to fetch user %s: %w", id, err)
}
```

### Phase 4: Custom Error Handlers

Per-project error handling strategies:

```toml
[error_handling]
strategy = "wrap_all"
context_template = "{function}: {error}"
```

### Phase 5: LSP Integration

Source map integration with language server:
- IDE errors point to `.dingo` files
- Go definitions map back to Dingo source
- Diagnostics use source maps

---

## Open Questions & Decisions

### Critical Decisions (Need Input)

1. **Default Syntax**
   - Recommendation: `question` (`?`)
   - Alternatives: `bang` or `try`
   - **Decision Needed:** Confirm default

2. **Source Map Format Default**
   - Recommendation: `inline` for development
   - Alternative: `separate` for production
   - **Decision Needed:** Confirm default

3. **Multi-Syntax Project Support**
   - Should a project allow mixing syntaxes?
   - Recommendation: No (one syntax per project)
   - **Decision Needed:** Confirm restriction

### Non-Critical Decisions (Can Defer)

4. **Temp Variable Naming**
   - `__tmp0, __err0` vs `_v0, _e0`
   - Decision: During implementation

5. **Error Return Strategy for Non-Pointer Types**
   - Zero value vs nil (for pointers)
   - Decision: Zero value (safer)

6. **Source Map Compression**
   - Compress VLQ mappings?
   - Decision: No for MVP, consider later

---

## Conclusion & Next Steps

### Summary

This enhanced plan delivers:
1. **Configurable syntax** - Users choose `?`, `!`, or `try`
2. **Source maps** - Errors point to `.dingo` files
3. **Real-world validation** - Tested with Go stdlib packages
4. **Production quality** - Complete testing, documentation, error handling

**Timeline:** 3-4 weeks (accounting for expanded scope)

**Risk Level:** Medium (expanded scope balanced by proven libraries)

**User Value:** High (choice + quality + proven reliability)

### Implementation Sequence

**Week 1:** Configuration + Parsers
**Week 2:** Transformation + Source Maps
**Week 3:** Stdlib Testing + Validation
**Week 4:** Polish + Documentation

### Immediate Next Steps

1. **Review this plan** - Confirm scope and timeline
2. **Resolve critical decisions** - Syntax default, source map format
3. **Spike source map library** - Validate `go-sourcemap` works (1-2 hours)
4. **Create task breakdown** - Convert weeks → daily tasks
5. **Begin Day 1** - Configuration system implementation

---

**Document Version:** 2.0 (Final)
**Author:** Claude (Dingo AI Architect)
**Status:** Ready for Implementation
**Timeline:** 3-4 weeks
**Scope:** Configurable Error Propagation + Source Maps + Stdlib Testing
