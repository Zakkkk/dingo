# Golden Files Created

## Successfully Generated (3/7)

### 1. pattern_match_09_tuple_pairs.go.golden
- **Size**: 1502 bytes
- **Source**: tests/golden/pattern_match_09_tuple_pairs.dingo
- **Status**: ✅ Transpiled and compiled successfully
- **Location**: tests/golden/pattern_match_09_tuple_pairs.go.golden

### 2. pattern_match_10_tuple_triples.go.golden
- **Size**: 1743 bytes
- **Source**: tests/golden/pattern_match_10_tuple_triples.dingo
- **Status**: ✅ Transpiled and compiled successfully
- **Location**: tests/golden/pattern_match_10_tuple_triples.go.golden

### 3. pattern_match_11_tuple_wildcards.go.golden
- **Size**: 1828 bytes
- **Source**: tests/golden/pattern_match_11_tuple_wildcards.dingo
- **Status**: ✅ Transpiled and compiled successfully
- **Location**: tests/golden/pattern_match_11_tuple_wildcards.go.golden

## Failed Transpilation (4/7)

### 4. pattern_match_01_simple.dingo
**Error**: Parse error at line 103:6 - expected operand, found ','

**Analysis**:
- Source file is only 62 lines
- Preprocessor generates output that fails at line 103
- Indicates preprocessor is generating too many lines or malformed code
- Error suggests comma placement issue in generated switch statements

**Test Content**:
- Simple pattern matching on Result[T,E]
- Pattern matching on Option[T]
- Wildcard patterns
- Assignment context patterns
- Nested pattern matching

### 5. pattern_match_03_nested.dingo
**Error**: Parse error at line 62:14 - missing ',' in argument list (and 5 more errors)

**Analysis**:
- Multiple parse errors in generated code
- Missing commas suggests function call or struct literal malformation
- Likely preprocessor bug in nested match expression transformation

### 6. pattern_match_06_guards_nested.dingo
**Error**: Parse error at line 93:18 - missing ',' in argument list (and 10 more errors)

**Analysis**:
- 10+ parse errors indicates severe preprocessor issue
- Guards with nested matching likely exposing edge case
- Most complex of the failing tests

### 7. pattern_match_12_tuple_exhaustiveness.dingo
**Error**: rust_match preprocessing failed: line 149: parsing pattern arms: no pattern arms found

**Analysis**:
- Preprocessor fails before even generating Go code
- Cannot parse pattern arms from match expression
- Suggests regex or parsing logic issue in rust_match preprocessor

## Summary

**Success Rate**: 3/7 (42.9%)

**Tuple tests**: 3/3 succeeded ✅
**Non-tuple tests**: 0/4 succeeded ❌

**Pattern Observed**: All tuple destructuring tests pass, all non-tuple tests fail.
This suggests the preprocessor handles tuple patterns correctly but has bugs in:
- Result/Option enum pattern matching
- Guard conditions
- Nested match expressions
- Basic pattern arm parsing

## Recommendation

These failures indicate **preprocessor bugs, not test file issues**. The `.dingo` source files are valid Dingo syntax, but the rust_match preprocessor is generating invalid Go code.

**Required Fix**: Debug and fix pkg/preprocessor/rust_match.go transformation logic before golden files can be generated.
