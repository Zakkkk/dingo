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

## Features (11 plugins in pkg/feature/builtin/)

| Feature | Priority | Syntax |
|---------|----------|--------|
| enum | 10 | `enum Name { Variant }` |
| match | 20 | `match expr { Pat => val }` |
| error_prop | 40 | `expr?` |
| safe_nav | 60 | `x?.y` |
| null_coalesce | 70 | `a ?? b` |
| lambdas | 80 | `\|x\| expr` or `x => expr` |
| generics | 110 | `<T>` → `[T]` |
| let_binding | 120 | `let x =` → `x :=` |

## Code Generation Standards

Variable naming:
- ✅ `tmp`, `tmp1`, `tmp2` (camelCase, no leading number)
- ❌ `__tmp0`, `_err_0` (no underscores, no zero-based)

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

## Running Dingo in Claude Code

Always use `--no-mascot` flag when running dingo build in Claude Code terminal:
```bash
./dingo build --no-mascot examples/03_option/user_settings.dingo
```
This disables animation which doesn't render properly in Claude Code.

## References

- Research: `ai-docs/claude-research.md`, `ai-docs/gemini_research.md`
- Architecture: `ai-docs/dingo-vs-borgo.md`

---
**Last Updated**: 2025-12-08
