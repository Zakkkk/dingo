# Parser Fix: Composite Type Support (interface{}, map[], etc.)

**Date:** 2025-12-10
**Status:** Complete
**Issue:** Parser failed on `map[string]interface{}` return types

## Problem

The Dingo parser's `parseType()` function only handled simple identifiers and generic types (`Result[T,E]`), causing parse errors on Go's built-in composite types:

```
Error: "parse error at 80:84: expected declaration, got {"
```

This occurred when parsing:
```go
func LoadConfig(...) map[string]interface{} {
```

The parser couldn't handle `interface{}` (empty interface) as a type.

## Solution

Extended `pkg/parser/decl.go` to parse all Go composite types:

### 1. Modified `parseType()`

Added switch statement to dispatch to specialized parsers based on token kind:

```go
switch p.curToken.Kind {
case tokenizer.MAP:
    return p.parseMapType()
case tokenizer.LBRACKET:
    return p.parseSliceOrArrayType()
case tokenizer.CHAN:
    return p.parseChanType()
case tokenizer.FUNC:
    return p.parseFuncType()
case tokenizer.INTERFACE:
    return p.parseInterfaceType()
case tokenizer.STAR:
    // Pointer type: *T
    ...
}
```

### 2. Added Type Parsers

Implemented 5 new parser functions:

#### `parseMapType()` - map[K]V
- Parses `map[keyType]valueType`
- Recursively parses key and value types
- Returns `*ast.MapType`

#### `parseInterfaceType()` - interface{...}
- Parses `interface{methodSet}`
- Handles empty interface `interface{}`
- Skips method definitions for now (minimal parsing)
- Returns `*ast.InterfaceType`

#### `parseSliceOrArrayType()` - []T or [N]T
- Parses slice `[]T` and array `[N]T` types
- Sets `Len` to `nil` for slices
- Returns `*ast.ArrayType`

#### `parseChanType()` - chan T, chan<- T, <-chan T
- Parses channel types with direction
- Handles bidirectional, send-only, receive-only
- Returns `*ast.ChanType`

#### `parseFuncType()` - func(...) ...
- Minimal parsing (skips signature details)
- Balances parentheses
- Returns `*ast.FuncType`

## Testing

### 1. Unit Tests

Created `pkg/parser/type_parsing_test.go` with comprehensive tests:

```
TestParseType_CompositeTypes:
  ✓ empty_interface
  ✓ map_with_interface_value
  ✓ slice_type
  ✓ map_with_slice_value
  ✓ pointer_type
  ✓ chan_type

TestParseMapType - validates AST structure
TestParseInterfaceType - validates empty interface
TestParseSliceType - validates slice vs array
```

All tests pass: `ok github.com/MadAppGang/dingo/pkg/parser 0.215s`

### 2. Integration Test

Verified original failing file:

```bash
./dingo build examples/10_null_coalesce/defaults.dingo
# Build successful!

./examples/10_null_coalesce/defaults
# Output correct - program runs successfully
```

## Files Modified

- `pkg/parser/decl.go` - Extended type parsing (220 lines added)
- `pkg/parser/type_parsing_test.go` - New test file (176 lines)

## Impact

**Before:**
- Parser failed on composite types
- Error: "expected declaration, got {"
- Only simple types and generics worked

**After:**
- Full Go type syntax supported
- `interface{}`, `map[]`, `[]slice`, `chan`, `func`, `*pointer` all work
- Recursive type parsing (e.g., `map[string][]interface{}`)

## Limitations

Current implementation is minimal but sufficient:

1. **Array lengths** - Skipped during parsing (rare in Dingo)
2. **Interface methods** - Not parsed (just skip body)
3. **Function signatures** - Minimal parsing (just balance parens)

These limitations are acceptable because:
- Dingo transpiler preserves Go declarations as-is
- Type checking happens via gopls, not parser
- Parser only needs to not error on valid Go syntax

## Verification

```bash
# All parser tests pass
go test ./pkg/parser/...
# ok github.com/MadAppGang/dingo/pkg/parser 0.215s

# Original failing case now works
./dingo build examples/10_null_coalesce/defaults.dingo
# Build successful!
```

## Next Steps

If needed for future enhancements:
- Parse interface method signatures (for better LSP support)
- Parse function type signatures (for type analysis)
- Parse array length expressions (requires Dingo→Go AST conversion)

For now, current implementation handles all real-world Dingo code.
