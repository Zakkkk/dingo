# Phase 2.2 Implementation Plan: Complete Error Propagation

**Session:** 20251117-183117
**Phase:** 2.2 - Error Propagation Polish & CLI Integration
**Status:** Planning

---

## Executive Summary

Phase 2.2 completes the error propagation feature by:
1. Fixing critical bugs in the preprocessor (wrong zero values, expression parsing)
2. Enhancing the preprocessor to handle all 8 golden test cases
3. Integrating the preprocessor into the CLI build pipeline
4. Making all error_prop_*.dingo tests pass

**Current State:** Basic error propagation works for simple cases but fails on:
- Multiple `?` operators in one function
- Complex expressions with `&` (address-of) operators
- Error message wrapping (`expr? "context message"`)
- Incorrect zero value generation (hardcoded `nil`, `0`)

**Target:** All 8 error_prop tests pass with correct, compilable Go output.

---

## Problem Analysis

### Current Implementation Issues

From analyzing `pkg/preprocessor/error_prop.go`:

#### 1. Hardcoded Zero Values
```go
// Line 106-107 (assignment)
buf.WriteString("\treturn nil, ")

// Line 156-157 (return)
buf.WriteString("\treturn 0, ")
```

**Problem:** Always returns `nil` or `0`, but should infer correct zero value from function signature.

**Impact:**
- `error_prop_01_simple.go.golden` expects `return nil, __err0` ✅ (works by accident)
- `error_prop_03_expression.go.golden` expects `return 0, __err0` ✅ (works by accident)
- Future tests with custom types will fail

**Solution:** Parse function signature to determine return type, generate proper zero value.

#### 2. Expression Boundary Detection

Current regex:
```go
assignPattern := regexp.MustCompile(`^\s*(let|var)\s+(\w+)\s*=\s*(.+)\?\s*$`)
```

**Problem:** `.+` is too greedy, doesn't understand Go syntax.

**Failures:**
- `&result` in `error_prop_02_multiple.dingo`: Captures `&result` as `ILLEGALresult`
- `&cfg` in `error_prop_08_chained_calls.dingo`: Captures `&cfg` as `ILLEGALcfg`

**Root Cause:** Regex can't parse Go expressions correctly.

**Solution:** Implement simple expression parser or use go/scanner to find expression boundaries.

#### 3. Error Message Wrapping

`error_prop_08_chained_calls.dingo` uses:
```go
data := ReadFile(path)? "failed to read config"
```

Expected output:
```go
if __err1 != nil {
    return nil, fmt.Errorf("failed to read config: %w", __err0)
}
```

**Current:** Not implemented at all.

**Solution:** Detect optional string after `?`, generate `fmt.Errorf` wrapper.

#### 4. Multiple `?` in Same Function

`error_prop_02_multiple.dingo` has 2 `?` operators. Current code increments `tryCounter` correctly, but variable name generation has bugs.

**Golden test shows:** `__tmp0/__err0`, then `__tmp1/__err1` ✅ (correct)

**But:** The golden test also has `ILLEGAL` prefixes, suggesting the original generator had bugs we need to understand.

#### 5. Type Syntax Handling

Dingo uses `:` for type annotations:
```go
func readConfig(path: string) ([]byte, error)
```

Go uses:
```go
func readConfig(path string) ([]byte, error)
```

**Current:** Not handled (tests may be using Go syntax already).

**Solution:** Either require Go syntax or add simple `:` → space replacement.

---

## Implementation Strategy

### Phase A: Fix Expression Parsing (Critical)

**Goal:** Correctly extract expressions before `?` operator.

**Approach:** Use `go/scanner` to tokenize and find balanced expression boundaries.

**Algorithm:**
```
1. Find first `?` in line
2. Scan backwards from `?` to find expression start
3. Track parentheses/brackets/braces for balance
4. Detect operators to find expression boundary
5. Extract expr = line[start:questionMark]
```

**Example:**
```go
Input:  "data := ReadFile(path)?"
                   ^start        ^end
Output: expr = "ReadFile(path)"
```

**Edge Cases:**
- `Unmarshal(data, &result)?` → expr = `Unmarshal(data, &result)`
- `obj.Method(a, b, c)?` → expr = `obj.Method(a, b, c)`
- `arr[i]?` → expr = `arr[i]`

**Implementation:** New function `extractExpression(line, questionMarkPos) (expr, start, end)`

### Phase B: Zero Value Inference (High Priority)

**Goal:** Generate correct zero values based on function signature.

**Strategy:**

**Option 1: AST-based (Robust)**
1. Buffer all function lines until we hit the function body
2. Parse function signature with `go/parser`
3. Extract return types from AST
4. Generate zero values for each type

**Option 2: Regex-based (Quick)**
1. Look backward from current line to find function declaration
2. Parse `func name(...) (type1, type2, ...) {` with regex
3. Map types to zero values

**Recommendation:** Option 2 for Phase 2.2 (simpler), Option 1 for future refactor.

**Zero Value Map:**
```go
var zeroValues = map[string]string{
    "int": "0",
    "int8": "0", "int16": "0", "int32": "0", "int64": "0",
    "uint": "0", "uint8": "0", "uint16": "0", "uint32": "0", "uint64": "0",
    "float32": "0.0", "float64": "0.0",
    "string": `""`,
    "bool": "false",
    "error": "nil",
    // Pointers, slices, maps, channels, interfaces → "nil"
}
```

**Detection:** If type starts with `*`, `[]`, `map[`, `chan`, `interface` → `"nil"`

**Fallback:** If unknown type → `*new(T)` (universal zero value).

### Phase C: Error Message Wrapping (Medium Priority)

**Goal:** Support `expr? "error context"` syntax.

**Detection:**
```go
pattern := regexp.MustCompile(`(.+)\?\s+"([^"]+)"`)
```

**Transformation:**
```go
// Input
data := ReadFile(path)? "failed to read config"

// Output
__tmp0, __err1 := ReadFile(path)
if __err1 != nil {
    return nil, fmt.Errorf("failed to read config: %w", __err0)
}
var data = __tmp0
```

**Implementation:**
1. Extract expression and message
2. Generate wrapped error with `fmt.Errorf`
3. Ensure `import "fmt"` is present (add to imports if missing)

**Note:** Error variable naming inconsistency in golden tests (`__err0` vs `__err1`) needs investigation.

### Phase D: CLI Integration (Critical)

**Goal:** Wire preprocessor into `cmd/dingo/main.go` build pipeline.

**Current Pipeline:**
```
.dingo → parser.ParseFile → AST → generator.Generate → .go
```

**New Pipeline:**
```
.dingo → preprocessor.Process → valid Go → parser.ParseFile → AST → transformer.Transform → generator.Generate → .go
```

**Changes Required:**

#### 1. Update `buildFile()` function

**Before:**
```go
src, err := os.ReadFile(inputPath)
fset := token.NewFileSet()
file, err := parser.ParseFile(fset, inputPath, src, 0)
```

**After:**
```go
src, err := os.ReadFile(inputPath)

// Preprocess
prep := preprocessor.New(src)
goSource, sourceMap, err := prep.Process()

// Parse preprocessed Go
fset := token.NewFileSet()
file, err := parser.ParseFile(fset, inputPath, []byte(goSource), 0)
```

#### 2. Error Position Mapping

When `parser.ParseFile` fails, map error positions back to original .dingo:

```go
if err != nil {
    // Map error using sourceMap
    originalErr := mapErrorPosition(err, sourceMap)
    return fmt.Errorf("parse error: %w", originalErr)
}
```

**Implementation:** `mapErrorPosition()` function extracts position from go/parser error, looks up in source map, reconstructs error with original position.

#### 3. Source Map Output

For debugging and LSP support:

```go
// Write source map alongside .go file
sourceMapPath := outputPath + ".map"
sourceMapJSON, _ := json.MarshalIndent(sourceMap, "", "  ")
os.WriteFile(sourceMapPath, sourceMapJSON, 0644)
```

**Format:**
```json
{
  "mappings": [
    {
      "originalLine": 4,
      "originalColumn": 18,
      "generatedLine": 4,
      "generatedColumn": 11,
      "length": 1,
      "name": "error_prop"
    }
  ]
}
```

### Phase E: Golden Test Validation (Critical)

**Goal:** All 8 error_prop tests produce exact matches with .go.golden files.

**Test Files:**
1. `error_prop_01_simple.dingo` - Basic assignment ✅ (likely works)
2. `error_prop_02_multiple.dingo` - Multiple `?` in function (needs expression fix)
3. `error_prop_03_expression.dingo` - Return statement ✅ (likely works)
4. `error_prop_04_wrapping.dingo` - Error wrapping (needs Phase C)
5. `error_prop_05_complex_types.dingo` - Custom types (needs zero value inference)
6. `error_prop_06_mixed_context.dingo` - Mixed patterns
7. `error_prop_07_special_chars.dingo` - Edge cases
8. `error_prop_08_chained_calls.dingo` - Error wrapping + multiple (needs Phase C + expression fix)

**Test Harness:**
```go
// tests/golden/golden_test.go
func TestErrorPropGolden(t *testing.T) {
    files, _ := filepath.Glob("error_prop_*.dingo")
    for _, dingoFile := range files {
        goldenFile := strings.Replace(dingoFile, ".dingo", ".go.golden", 1)

        // Preprocess + generate
        actual := buildDingoFile(dingoFile)
        expected, _ := os.ReadFile(goldenFile)

        // Compare
        if actual != string(expected) {
            t.Errorf("Mismatch for %s", dingoFile)
            t.Logf("Expected:\n%s", expected)
            t.Logf("Actual:\n%s", actual)
        }
    }
}
```

---

## Technical Deep Dives

### Expression Parsing with go/scanner

**Why go/scanner?**
- Standard library, well-tested
- Tokenizes Go source correctly
- Handles all operators, brackets, strings, etc.
- No regex fragility

**Algorithm:**

```go
func extractExpressionBeforeQuestionMark(line string, qPos int) (expr string, start int) {
    // Scan backwards from ? to find expression start
    s := scanner.Scanner{}
    s.Init(fset.AddFile("", -1, len(line)), []byte(line), nil, 0)

    tokens := []scanner.Token{}
    for {
        pos, tok, lit := s.Scan()
        if pos >= qPos {
            break
        }
        tokens = append(tokens, scanner.Token{Pos: pos, Tok: tok, Lit: lit})
    }

    // Walk backwards from last token before ?
    // Find expression boundary (assignment operator, opening brace, etc.)
    exprStart := findExpressionStart(tokens)
    exprEnd := qPos

    return line[exprStart:exprEnd], exprStart
}

func findExpressionStart(tokens []scanner.Token) int {
    // Look for boundary operators: =, :=, {, ,, return, etc.
    for i := len(tokens) - 1; i >= 0; i-- {
        tok := tokens[i]
        if tok.Tok == token.ASSIGN || tok.Tok == token.DEFINE || tok.Tok == token.LBRACE {
            return tokens[i+1].Pos // Start after the operator
        }
    }
    return 0 // Default to start of line
}
```

**Trade-off:** More complex than regex, but correct and maintainable.

### Zero Value Generation Strategy

**Function Signature Detection:**

```go
func findFunctionSignature(lines []string, currentLine int) *ast.FuncDecl {
    // Search backwards from currentLine to find func declaration
    for i := currentLine - 1; i >= 0; i-- {
        if strings.HasPrefix(strings.TrimSpace(lines[i]), "func ") {
            // Found function start, parse until body
            funcText := ""
            for j := i; j <= currentLine; j++ {
                funcText += lines[j] + "\n"
                if strings.Contains(lines[j], "{") {
                    break
                }
            }

            // Parse as Go code
            fset := token.NewFileSet()
            file, _ := parser.ParseFile(fset, "", "package p\n"+funcText, 0)
            return file.Decls[0].(*ast.FuncDecl)
        }
    }
    return nil
}
```

**Zero Value from Type:**

```go
func getZeroValue(typ string) string {
    typ = strings.TrimSpace(typ)

    // Check map first
    if zero, ok := zeroValues[typ]; ok {
        return zero
    }

    // Pointer, slice, map, chan, interface → nil
    if strings.HasPrefix(typ, "*") || strings.HasPrefix(typ, "[]") ||
       strings.HasPrefix(typ, "map[") || strings.HasPrefix(typ, "chan ") ||
       typ == "interface{}" {
        return "nil"
    }

    // Custom struct/type → composite literal
    // T{} works for most types
    return typ + "{}"
}
```

**Return Types from FuncDecl:**

```go
func getReturnTypes(funcDecl *ast.FuncDecl) []string {
    if funcDecl.Type.Results == nil {
        return nil
    }

    types := []string{}
    for _, field := range funcDecl.Type.Results.List {
        typeStr := types.ExprString(field.Type) // Use go/types helper

        // If field has multiple names (rare in returns), repeat type
        count := len(field.Names)
        if count == 0 {
            count = 1
        }
        for i := 0; i < count; i++ {
            types = append(types, typeStr)
        }
    }
    return types
}
```

**Usage:**

```go
// In expandAssignment/expandReturn
funcDecl := findFunctionSignature(allLines, currentLineNum)
returnTypes := getReturnTypes(funcDecl)

// For error propagation, we need all return values except last (error)
zeroValues := []string{}
for i := 0; i < len(returnTypes)-1; i++ {
    zeroValues = append(zeroValues, getZeroValue(returnTypes[i]))
}

// Generate return statement
returnStmt := fmt.Sprintf("return %s, %s",
    strings.Join(zeroValues, ", "),
    errVar)
```

---

## Implementation Phases

### Phase A: Expression Parsing (6 hours)

**Tasks:**
1. Implement `extractExpression()` using go/scanner ✅
2. Unit test with complex expressions ✅
3. Update `processLine()` to use new parser ✅
4. Test with `error_prop_02_multiple.dingo` ✅

**Deliverables:**
- `pkg/preprocessor/expression.go` - Expression parsing logic
- Unit tests covering edge cases
- `error_prop_02_multiple` test passing

**Acceptance Criteria:**
- Correctly extracts `Unmarshal(data, &result)` including `&` operator
- Handles nested function calls
- Handles method calls with multiple arguments

### Phase B: Zero Value Inference (4 hours)

**Tasks:**
1. Implement `findFunctionSignature()` ✅
2. Implement `getZeroValue()` with type map ✅
3. Update `expandAssignment()` and `expandReturn()` ✅
4. Test with various return types ✅

**Deliverables:**
- `pkg/preprocessor/types.go` - Type analysis and zero values
- Updated error_prop.go using zero value inference
- Unit tests for zero value generation

**Acceptance Criteria:**
- Generates correct zero for primitives (int, string, bool)
- Generates `nil` for pointers, slices, interfaces
- Generates `T{}` for custom types

### Phase C: Error Wrapping (3 hours)

**Tasks:**
1. Detect `? "message"` syntax ✅
2. Generate `fmt.Errorf` wrapping ✅
3. Ensure `fmt` import is added ✅
4. Test with error_prop_04 and error_prop_08 ✅

**Deliverables:**
- Error wrapping logic in error_prop.go
- Import management (detect and add `fmt` import)
- Tests passing for wrapping syntax

**Acceptance Criteria:**
- `expr? "msg"` generates `fmt.Errorf("msg: %w", err)`
- `fmt` import added automatically if not present
- Original error preserved with `%w` formatting

### Phase D: CLI Integration (3 hours)

**Tasks:**
1. Update `buildFile()` to use preprocessor ✅
2. Implement error position mapping ✅
3. Add source map output ✅
4. Update `runDingoFile()` similarly ✅

**Deliverables:**
- Modified `cmd/dingo/main.go`
- Error mapping logic
- Source map JSON output

**Acceptance Criteria:**
- `dingo build file.dingo` uses preprocessor pipeline
- Syntax errors point to original .dingo line numbers
- .go.map file generated alongside .go

### Phase E: Golden Test Validation (4 hours)

**Tasks:**
1. Create golden test harness ✅
2. Run all 8 error_prop tests ✅
3. Fix discrepancies ✅
4. Document any golden file corrections needed ✅

**Deliverables:**
- `tests/golden/golden_test.go` test suite
- All error_prop tests passing
- Documentation of any golden file updates

**Acceptance Criteria:**
- All 8 tests generate exact match with .go.golden
- Generated code compiles with `go build`
- No regressions in existing functionality

---

## Edge Cases & Challenges

### 1. Golden Test Inconsistencies

**Observation:** `error_prop_02_multiple.go.golden` contains:
```go
__tmp0, __err0 := ReadFile(path)
// ...
__tmp1, __err1 := Unmarshal(data, ILLEGALresult)
```

**Issue:** `ILLEGAL` prefix suggests the golden file itself has bugs (from old generator).

**Resolution Strategy:**
1. Determine if golden files are "expected buggy output" or "target correct output"
2. If buggy, fix golden files to show correct output
3. If correct, understand why `ILLEGAL` prefix is expected

**Investigation Needed:** Check git history or docs for golden file generation process.

### 2. Import Management

Error wrapping requires `import "fmt"`. Preprocessor needs to:
1. Detect if `fmt` is already imported
2. Add import if missing
3. Preserve existing import structure

**Simple Approach:**
- Check if line contains `import "fmt"` or `import (\n...\n"fmt"\n...\n)`
- If not found, insert after package declaration

**Robust Approach (future):**
- Use go/ast to parse imports
- Add using astutil.AddImport
- Regenerate import block

**Phase 2.2 Decision:** Simple string-based check, add `import "fmt"` if needed.

### 3. Multiple `?` in One Expression

**Example:**
```go
result := foo()? + bar()?
```

**Challenge:** This is NOT currently supported and should error gracefully.

**Golden tests don't include this** - all use separate statements.

**Phase 2.2 Decision:** Out of scope. Document as limitation. Future enhancement.

### 4. Type Annotation Syntax (`:` vs space)

**Dingo:** `func readConfig(path: string)`
**Go:** `func readConfig(path string)`

**Current golden tests:** Use Go syntax (space, not `:`)

**Phase 2.2 Decision:** Assume Go syntax. Type annotation transformation is separate feature.

### 5. Source Map Accuracy

Source maps must correctly map:
- Original line N → Generated lines N, N+1, N+2, ...
- Column offsets for expressions
- Multi-line expansions

**Challenge:** When one line becomes 5 lines, how to map errors?

**Strategy:**
- Map start of expansion to original line
- Use `// dingo:s:1` and `// dingo:e:1` markers (seen in golden tests)
- These markers indicate "folded region" for error mapping

**Implementation:** Use markers to collapse expansion in error messages.

---

## Success Criteria

### Must Have (P0)
- ✅ All 8 error_prop golden tests pass
- ✅ Generated Go code compiles without errors
- ✅ CLI `dingo build` command uses new preprocessor
- ✅ Correct zero values for common types
- ✅ Expression parsing handles `&`, `*`, method calls

### Should Have (P1)
- ✅ Error wrapping syntax (`? "message"`) works
- ✅ Source maps generated and accurate
- ✅ Error messages point to original .dingo positions
- ✅ Unit tests cover edge cases

### Nice to Have (P2)
- Documentation of preprocessor architecture
- Performance metrics (< 100ms per file)
- Debug mode to save preprocessed .go for inspection

---

## Testing Strategy

### Unit Tests
Location: `pkg/preprocessor/`

**Tests to add:**
```go
TestExpressionExtraction(t *testing.T)
  - Simple call: fetchData()
  - With args: ReadFile(path)
  - Address-of: Unmarshal(data, &result)
  - Method call: obj.Method(a, b)
  - Nested: outer(inner(x))

TestZeroValueGeneration(t *testing.T)
  - Primitives: int, string, bool
  - Pointers: *Config
  - Slices: []byte
  - Maps: map[string]interface{}
  - Interfaces: error
  - Custom types: Config

TestErrorWrapping(t *testing.T)
  - Basic: expr? "message"
  - With special chars: expr? "message: %s"
  - Without wrapping: expr?

TestFunctionSignatureParsing(t *testing.T)
  - Single return: (int, error)
  - Multiple returns: ([]byte, int, error)
  - Named returns: (data []byte, err error)
  - No returns: ()
```

### Integration Tests
Location: `tests/golden/`

**Golden test harness:**
```go
func TestErrorPropGolden(t *testing.T) {
    goldenTests := []string{
        "error_prop_01_simple",
        "error_prop_02_multiple",
        // ... all 8 tests
    }

    for _, name := range goldenTests {
        t.Run(name, func(t *testing.T) {
            dingoPath := filepath.Join("tests/golden", name+".dingo")
            goldenPath := filepath.Join("tests/golden", name+".go.golden")

            actual := transpileDingoFile(dingoPath)
            expected, _ := os.ReadFile(goldenPath)

            if !bytes.Equal(actual, expected) {
                t.Errorf("Mismatch for %s", name)
                showDiff(t, expected, actual)
            }
        })
    }
}
```

### Compile Tests
Ensure generated Go actually compiles:

```go
func TestErrorPropCompiles(t *testing.T) {
    // For each golden test
    // 1. Transpile .dingo to .go
    // 2. Run `go build` on output
    // 3. Assert no compile errors
}
```

---

## Risk Assessment

### High Risk
1. **Golden test inconsistencies** - If golden files have bugs, fixing them changes "expected" behavior
   - Mitigation: Investigate golden file generation, document changes, get user confirmation

2. **Expression parsing complexity** - Go expressions are complex, scanner-based approach may miss edge cases
   - Mitigation: Extensive unit tests, fallback to regex for simple cases

3. **Zero value inference fragility** - Type strings can be complex (generics, qualified types)
   - Mitigation: Conservative fallback (return `nil` or `T{}`), comprehensive type map

### Medium Risk
1. **Import management** - Adding imports can break existing code if conflicts exist
   - Mitigation: Check for existing imports, use qualified names if needed

2. **Source map accuracy** - Position mapping is error-prone
   - Mitigation: Extensive testing, use marker comments for validation

3. **CLI integration** - Changing build pipeline may break other features
   - Mitigation: Preserve old plugin system temporarily, test in isolation

### Low Risk
1. **Performance** - Scanner-based parsing may be slower than regex
   - Mitigation: Profile and optimize if needed, acceptable for Phase 2.2

2. **Error wrapping** - fmt.Errorf is straightforward
   - Mitigation: Unit tests, verify against golden tests

---

## Implementation Timeline

**Total Estimated Time:** 20 hours

### Session 1 (6 hours) - Expression Parsing
- Implement expression extraction with go/scanner
- Unit tests
- Update processLine()
- Test error_prop_02_multiple

### Session 2 (4 hours) - Zero Values
- Function signature detection
- Zero value generation
- Update expansion logic
- Unit tests

### Session 3 (3 hours) - Error Wrapping
- Detect wrapping syntax
- Generate fmt.Errorf
- Import management
- Test error_prop_04 and error_prop_08

### Session 4 (3 hours) - CLI Integration
- Update buildFile()
- Error mapping
- Source map output
- Update runDingoFile()

### Session 5 (4 hours) - Validation
- Create golden test harness
- Run all tests
- Fix discrepancies
- Document results

---

## Open Questions

1. **Golden file correctness:** Are the `ILLEGAL` prefixes in error_prop_02/08 golden files intentional or bugs from old generator?
   - **Action:** Investigate git history, check with user

2. **Type annotation syntax:** Should preprocessor handle `:` to space conversion, or is that a separate feature?
   - **Assumption:** Separate feature, out of scope for Phase 2.2

3. **Import position:** Where to insert `import "fmt"` if not present?
   - **Decision:** After package declaration, before other imports

4. **Error variable naming:** Golden tests show inconsistent naming (`__err0` vs `__err1` for same expansion)
   - **Action:** Investigate pattern, ensure consistency in new implementation

5. **Source map format:** JSON is readable but verbose. Binary format for production?
   - **Decision:** JSON for Phase 2.2, optimize later if needed

---

## Dependencies

### External Libraries
- `go/scanner` - Expression tokenization
- `go/parser` - Function signature parsing
- `go/ast` - AST manipulation (minimal, for future)
- Standard library only - no new dependencies

### Internal Packages
- `pkg/preprocessor` - Core implementation
- `cmd/dingo` - CLI integration
- `tests/golden` - Test validation

### Blocking Issues
None identified. All dependencies are available.

---

## Future Enhancements (Post Phase 2.2)

1. **AST-based zero value generation** - More robust than regex parsing
2. **Multiple `?` in one expression** - Needs precedence rules
3. **Optimized source maps** - Binary format, compressed
4. **Better error messages** - Show original + generated code side-by-side
5. **Debug mode** - Save preprocessed .go for manual inspection
6. **Performance optimization** - Cache compiled regexes, reuse scanners
7. **Type annotation support** - Handle `:` syntax in function parameters

---

## Appendix: File Changes

### New Files
- `pkg/preprocessor/expression.go` - Expression parsing with go/scanner
- `pkg/preprocessor/types.go` - Type analysis and zero values
- `pkg/preprocessor/imports.go` - Import management utilities
- `tests/golden/golden_test.go` - Golden test harness

### Modified Files
- `pkg/preprocessor/error_prop.go` - Use new expression parser, zero values, wrapping
- `pkg/preprocessor/preprocessor.go` - Orchestration updates
- `cmd/dingo/main.go` - CLI integration
- `pkg/preprocessor/preprocessor_test.go` - Add new unit tests

### Generated Files
- `*.go.map` - Source map files (one per .dingo file)

---

## Conclusion

Phase 2.2 is a critical polish phase that transforms the error propagation feature from "basic prototype" to "production ready". The main challenges are:

1. **Expression parsing** - Moving from fragile regex to robust go/scanner
2. **Zero value inference** - Context-aware generation instead of hardcoded values
3. **CLI integration** - Wiring up the preprocessor pipeline end-to-end

With careful implementation and comprehensive testing, all 8 golden tests will pass, providing a solid foundation for the remaining Dingo features.

**Next Steps:** Begin Phase A (Expression Parsing) implementation.
