# Claude AI Agent Instructions - Dingo Project

## 🚨🚨🚨 STOP: READ THIS BEFORE ANY IMPLEMENTATION 🚨🚨🚨

**We have FAILED to follow this rule THREE TIMES. This is non-negotiable.**

### ❌ FORBIDDEN (will be rejected in code review):
- `bytes.Index()`, `strings.Index()`, `strings.Contains()`
- `regexp.MustCompile()`, `regexp.Match()`, `regexp.Find*()`
- Character scanning: `for i := 0; i < len(src); i++ { if src[i] == '?' }`
- Heuristics like "find the first `{` after `match`"
- Any `Transform*Source(src []byte)` pattern that manipulates bytes

### ✅ REQUIRED (the ONLY correct approach):
```
Source → pkg/tokenizer/ → []Token → pkg/parser/ → AST → pkg/ast/*_codegen.go
         ↑                                                 ↑
   ONLY place that                                   ONLY accepts
   reads raw bytes                                   AST nodes
```

### Before implementing ANY feature:
1. Check if parser already handles it: `pkg/parser/`
2. Check if codegen exists: `pkg/ast/*_codegen.go`
3. If adding new syntax: extend parser FIRST, then codegen from AST

### Verification (run before ANY PR):
```bash
grep -rn "bytes\.Index\|strings\.Index\|regexp\.\|strings\.Contains" pkg/codegen/ pkg/ast/*_codegen.go
# Must return NOTHING
```

---

## What is Dingo?
A meta-language for Go (like TypeScript for JavaScript):
- Transpiles `.dingo` files to idiomatic `.go` files
- Provides Result/Option types, pattern matching, error propagation (`?`)
- 100% Go ecosystem compatibility via gopls proxy for IDE support

## Project Structure
```
cmd/dingo/          # CLI entry point
pkg/
├── ast/            # Code generators (*_codegen.go) - FROM AST ONLY
├── parser/         # Dingo parser (Pratt-based) - PRODUCES AST
├── tokenizer/      # Tokenizer - ONLY place reading raw bytes
├── goparser/       # Go parser wrapper + transforms
├── feature/        # Pluggable feature system
├── transpiler/     # Main pipeline
└── typechecker/    # go/types integration
tests/golden/       # Golden test files
examples/           # Example .dingo files
```

## Architecture

```
.dingo → tokenizer → parser → AST → *_codegen.go → .go file → gopls
```

**Key insight**: Dingo is syntax sugar, NOT a new type system. We use gopls for all type checking.

## Features (10 plugins in pkg/feature/builtin/)

| Feature | Priority | Syntax |
|---------|----------|--------|
| enum | 10 | `enum Name { Variant }` |
| match | 20 | `match expr { Pat => val }` |
| error_prop | 40 | `expr?` |
| tuples | 50 | `(a, b) := func()` |
| safe_nav | 60 | `x?.y` |
| null_coalesce | 70 | `a ?? b` |
| lambdas | 80 | `\|x\| expr` or `x => expr` |
| generics | 110 | Uses Go's native `[T]` syntax directly |

## Option/Result API (dgo package)

**Option[T]** methods:
- `.IsSome()` / `.IsNone()` - check state
- `.MustSome()` - extract value (panics if None)
- `.SomeOr(defaultVal)` - extract with default
- `.SomeOrElse(func() T)` - extract with lazy default

**Result[T, E]** methods:
- `.IsOk()` / `.IsErr()` - check state
- `.MustOk()` - extract value (panics if Err)
- `.MustErr()` - extract error (panics if Ok)
- `.OkOr(defaultVal)` - extract with default

**Constructors:**
- `Some(val)`, `None[T]()` - for Option
- `Ok[T, E](val)`, `Err[T, E](err)` - for Result

## Two Enum Patterns

1. **Generic types (dgo package):** `Option[T]`, `Result[T, E]`
   - Methods: `.IsSome()`, `.MustSome()`, `.IsOk()`, `.MustOk()`
   - Constructors: `Some(x)`, `None[T]()`, `Ok[T,E](x)`, `Err[T,E](e)`

2. **Interface-based enums:** `enum Option { Some(T), None }`
   - Generates Go interfaces + struct variants
   - Constructors: `NewOptionSome(x)`, `NewOptionNone()`
   - Use type switches: `switch v := opt.(type) { case OptionSome: ... }`

**Don't mix these patterns** - they have different APIs.

## Code Generation Standards

Variable naming:
- ✅ `tmp`, `tmp1`, `tmp2` (camelCase, no leading number)
- ❌ `__tmp0`, `_err_0` (no underscores, no zero-based)

## Test Policy

**NEVER exclude tests to hide bugs.** Fix the underlying issue instead.

- If a test is failing, fix the bug - don't exclude the test
- If an example doesn't compile, fix the transpiler - don't skip the example
- CI exclusions should only be temporary during active development
- Document any temporary exclusions with specific bug tracking
- **Features in `features/` directory have NO limitations** - all documented features must work

## Agent Selection

| Task | Agent |
|------|-------|
| Implementation | golang-developer |
| Architecture | golang-architect |
| Testing | golang-tester |
| Code review | code-reviewer |
| Codebase search | Explore |

**Landing page** (`landingpage/` dir): Use astro-* agents instead.

## Key Files

- Entry: `pkg/transpiler/pure_pipeline.go` → `PureASTTranspile()`
- Transform: `pkg/ast/transform.go` → `TransformSource()`
- Parser: `pkg/parser/` → Pratt-based expression parsing
- Features: `pkg/feature/builtin/plugins.go`

## Testing

- Golden tests: `tests/golden/` - see `GOLDEN_TEST_GUIDELINES.md`
- Run: `go test ./...`

## CLI Commands

Dingo CLI mirrors Go's compiler structure:

| Command | Description | Go Equivalent |
|---------|-------------|---------------|
| `dingo build` | Transpile + compile to binary | `go build` |
| `dingo run` | Transpile + run immediately | `go run` |
| `dingo go` | Transpile to .go files only | N/A |

All `go build/run` flags are passed through (e.g., `-o`, `-race`, `-ldflags`).

**Dingo-specific flags:**
- `--verbose` - Show the go build/run command
- `--no-mascot` - Disable mascot animation (silent output)

Note: `dingo run` always runs in silent mode (no mascot) to give the running program full CLI access.

## Running Dingo in Claude Code

Always use `--no-mascot` flag when running dingo build in Claude Code terminal:
```bash
./dingo build --no-mascot examples/03_option/user_settings.dingo
```
This disables animation which doesn't render properly in Claude Code.

For `dingo run`, the mascot is automatically disabled (no flag needed):
```bash
./dingo run examples/03_option/user_settings.dingo
```

## References

- Research: `ai-docs/claude-research.md`, `ai-docs/gemini_research.md`
- Architecture: `ai-docs/dingo-vs-borgo.md`

---
**Last Updated**: 2025-12-10
