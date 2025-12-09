# User Request: Migrate Enum Naming to CamelCase

## Context

Based on unanimous recommendation from 6 expert analyses (MiniMax M2, Grok Code Fast, GPT-5.1 Codex, Gemini 3 Pro, Sherlock Think Alpha, and internal Sonnet 4.5), we need to migrate from underscore-based naming to pure CamelCase for enum variants.

## Current Naming (Underscore - Option A)

```go
// Constructors
Value_Int(42)
Result_Ok(value)
Option_Some(data)

// Tags
ValueTag_Int
ResultTag_Ok
OptionTag_Some

// Fields
int_0, string_0
ok_0, err_0
some_0
```

## Target Naming (CamelCase - Option B)

```go
// Constructors
ValueInt(42)
ResultOk(value)
OptionSome(data)

// Tags
ValueTagInt
ResultTagOk
OptionTagSome

// Fields
int0, string0
ok0, err0
some0
```

## Rationale

All 6 models unanimously agreed:
1. **Go Idiomaticity**: Matches standard library patterns (`http.MethodGet`, `ast.BadDecl`, `io.EOF`)
2. **Developer Experience**: Generated code looks hand-written, no "foreign language" visual marker
3. **Tool Support**: Better IDE autocompletion, gopls integration
4. **Code Quality**: Passes `golint` and `go vet` without warnings

## Scope

### Files to Modify

**Primary Codegen**:
- `pkg/preprocessor/rust_match.go` - Main enum/match codegen logic
- `pkg/plugin/builtin/result_type.go` - Result[T,E] type generation
- `pkg/plugin/builtin/option_type.go` - Option[T] type generation

**Test Files**:
- All `tests/golden/*.go.golden` files (46+ files need regeneration)

### Changes Required

1. **Constructor naming**: `{Type}_{Variant}` → `{Type}{Variant}`
2. **Tag constant naming**: `{Type}Tag_{Variant}` → `{Type}Tag{Variant}`
3. **Field naming**: `{variant_lowercase}_{index}` → `{variant_lowercase}{index}`

## Success Criteria

1. All golden tests regenerated with new naming
2. All tests passing (including compilation tests)
3. No breaking changes to Dingo syntax (only generated Go code changes)
4. Pattern guards feature remains working
5. Documentation updated

## Estimated Effort

3-4 hours total (per expert consensus)

## Reference Documents

- Full analysis: `ai-docs/sessions/20251119-134800/FINAL-RECOMMENDATION.md`
- Individual model analyses: `ai-docs/sessions/20251119-134800/output/`
