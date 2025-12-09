# Dingo Parser (`pkg/goparser/`)

A token-based parser that transforms Dingo syntax to valid Go, leveraging Go's standard library for parsing.

## Architecture

```
┌─────────────────────────────────────────┐
│           .dingo Source File            │
└────────────────────┬────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────┐
│   Character-Level Passes (parser.go)    │
│   transformDingoChars():                │
│     • transformEnum()                   │
│     • transformMatch()                  │
│     • transformEnumConstructors()       │
│     • transformErrorProp()              │
│     • transformGuardLet()               │
│     • transformLambdas()                │
└────────────────────┬────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────┐
│   Token-Level Pass (TransformToGo)      │
│   Using go/scanner:                     │
│     • param: Type → param Type          │
│     • Result[T,E] → Result[T,E]         │
│     • let x = → x :=                    │
└────────────────────┬────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────┐
│   go/parser.ParseFile()                 │
│   Standard Go AST                       │
└─────────────────────────────────────────┘
```

## Package Structure

```
pkg/goparser/
├── README.md           # This file
├── parser/
│   ├── parser.go       # Core transformation logic (2000+ lines)
│   ├── parser_test.go  # Main test suite
│   ├── wildcard_test.go
│   └── debug_test.go
├── scanner/
│   └── scanner.go      # Extended scanner for Dingo tokens
└── token/
    └── token.go        # Dingo token definitions
```

## Usage

### Parse Dingo File to Go AST

```go
import "github.com/MadAppGang/dingo/pkg/goparser/parser"

fset := token.NewFileSet()
ast, err := parser.ParseFile(fset, "example.dingo", source, parser.ParseComments)
```

### Transform Dingo Source to Go Source

```go
goSource, mappings, err := parser.TransformToGo(dingoSource)
// goSource: Valid Go code
// mappings: Position mappings for source maps
```

## Features

| Feature | Dingo | Go Output |
|---------|-------|-----------|
| Type Annotations | `func(x: int)` | `func(x int)` |
| Let Declarations | `let x = 42` | `x := 42` |
| Generic Types | `Result[T, E]` | `Result[T, E]` |
| Error Propagation | `let x = expr?` | `tmp, err := expr; if err != nil { return err }; var x = tmp` |
| Guard Let | `guard let x = expr else \|err\| {...}` | `x, err := expr; if err != nil {...}` |
| Lambdas (Rust) | `\|x\| x + 1` | `func(x) { return x + 1 }` |
| Lambdas (TS) | `(x) => x + 1` | `func(x) { return x + 1 }` |
| Enums | `enum Status { Active }` | Tagged union interface pattern |
| Match | `match expr { P => r }` | IIFE with type switch |

## Key Functions

### parser.go

| Function | Purpose |
|----------|---------|
| `ParseFile()` | Main entry: transforms Dingo and returns Go AST |
| `TransformToGo()` | Token transformation, returns Go source + mappings |
| `transformDingoChars()` | Orchestrates character-level passes |
| `transformEnum()` | `enum` → interface + struct pattern |
| `transformMatch()` | `match` → IIFE type switch |
| `transformErrorProp()` | `expr?` → error handling code |
| `transformGuardLet()` | `guard let` → if err != nil |
| `transformLambdas()` | `\|x\|`, `(x) =>` → `func(x)` |

### scanner.go

Extended scanner that recognizes Dingo operators:
- `?` → QUESTION
- `??` → QUESTION_QUESTION
- `?.` → QUESTION_DOT
- `=>` → FAT_ARROW
- `->` → THIN_ARROW

### token.go

Dingo-specific tokens (100+ to avoid conflicts with go/token):
- `QUESTION`, `QUESTION_QUESTION`, `QUESTION_DOT`
- `FAT_ARROW`, `THIN_ARROW`
- `LET`, `MATCH`, `ENUM`, `GUARD`, `WHERE`

## Testing

```bash
# Run all parser tests
go test ./pkg/goparser/parser/... -v

# Run specific test
go test ./pkg/goparser/parser/... -run TestErrorPropTransformation -v
```

## Design Rationale

### Why Character-Level + Token-Level?

**Character-level** for complex syntax that Go's scanner sees as invalid:
- `enum Color { Red }` - not valid Go
- `match expr { }` - not valid Go
- `|x| x + 1` - not valid Go

**Token-level** for simple substitutions:
- `param: Type` → `param Type` (colon to space)
- `<T, E>` → `[T, E]` (angle to square brackets)

### Why Not Full Custom Parser?

1. Go's parser is battle-tested
2. We only transform syntax, not semantics
3. Leverage go/parser error messages and recovery
4. Maintain compatibility with Go tooling

## Source Map Support

`TransformToGo()` returns `[]TokenMapping`:

```go
type TokenMapping struct {
    OriginalLine   int
    OriginalCol    int
    GeneratedLine  int
    GeneratedCol   int
}
```

These mappings enable LSP features by translating positions between Dingo and Go.

## Error Handling

Parse errors include proper position information via `go/token.FileSet`. The parser transforms Dingo syntax first, so errors reference the generated Go code positions.

Future: Error position mapping back to original Dingo source using TokenMapping.
