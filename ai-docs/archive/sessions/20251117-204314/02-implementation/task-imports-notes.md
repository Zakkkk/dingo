# Task: Imports - Implementation Notes

## Critical Design Decision

### The "Invalid Syntax" Problem
Initially attempted to inject imports directly in `ErrorPropProcessor.Process()`, but this failed because:

1. ErrorPropProcessor outputs contain `let` keyword (Dingo syntax)
2. `let` is not valid Go syntax
3. `go/parser.ParseFile()` cannot parse invalid Go syntax
4. Import injection requires parsing to use `astutil.AddImport()`
5. Result: Parser fails, import injection silently returns original source

### Solution: Delayed Import Injection
Moved import injection to the main `Preprocessor.Process()` method:

1. **Phase 1:** ErrorPropProcessor tracks needed imports but doesn't inject
2. **Phase 2:** KeywordProcessor converts `let` → `var` (now valid Go!)
3. **Phase 3:** Main preprocessor collects imports from all processors
4. **Phase 4:** Inject imports (parser succeeds because syntax is valid)
5. **Phase 5:** Adjust all source mappings for added import lines

## ImportProvider Interface Pattern

Used **optional interface** pattern instead of modifying `FeatureProcessor`:

```go
// Existing interface - unchanged
type FeatureProcessor interface {
    Name() string
    Process(source []byte) ([]byte, []Mapping, error)
}

// Optional interface - processors implement if they need imports
type ImportProvider interface {
    GetNeededImports() []string
}

// Usage
if importProvider, ok := proc.(ImportProvider); ok {
    imports := importProvider.GetNeededImports()
}
```

**Benefits:**
- No breaking changes to existing processors
- Future processors can opt-in to import tracking
- Follows Go idiom (like io.Closer, fmt.Stringer)

## Source Mapping Adjustment

Source mappings must be shifted DOWN by the number of import lines added:

**Example:**
```
Input (line 4):
    data := ReadFile(path)?

Output BEFORE imports:
    Line 1: package main
    Line 2:
    Line 3: func example() {
    Line 4: __tmp0, __err0 := ReadFile(path)  // Maps to input line 4
    ...

Output AFTER imports:
    Line 1: package main
    Line 2:
    Line 3: import "os"
    Line 4:
    Line 5: func example() {
    Line 6: __tmp0, __err0 := ReadFile(path)  // STILL maps to input line 4
    ...
```

Offset calculation: `importLinesAdded = newLineCount - originalLineCount`

For single import: typically +3 lines (blank + import "pkg" + blank)
For multiple imports: +4-5 lines (blank + import ( ... ) + blank)

## Function Call Extraction Logic

Simple pattern matching to extract function names:

```go
"ReadFile(path)"       → "ReadFile"  → os
"os.ReadFile(path)"    → "ReadFile"  → os (still maps correctly!)
"pkg.Subpkg.Func()"    → "Func"      → lookup in stdLibFunctions
```

**Strategy:** Split on '.', take last part before '('

**Limitation:** Only tracks bare function calls, not method calls on objects.
This is acceptable because standard library functions are typically called as functions, not methods.

## Standard Library Coverage

Focused on **high-impact** functions commonly used in error-prone operations:

- **File I/O**: os.ReadFile, os.WriteFile, os.Open
- **Parsing**: strconv.Atoi, strconv.ParseInt, strconv.ParseFloat
- **JSON**: encoding/json.Marshal, encoding/json.Unmarshal
- **Error wrapping**: fmt.Errorf (already tracked via needsFmt flag)

**Rationale:** These are the functions most likely to appear with the `?` operator for error propagation.

## Test Strategy Updates

Updated tests to account for import injection:

1. **Basic tests**: Added expected import declarations to expected outputs
2. **Source mapping tests**: Adjusted expected line numbers by import offset
3. **Import detection tests**: NEW - verify imports are actually added
4. **Mapping adjustment tests**: NEW - verify mappings shift correctly

## Deviations from Plan

### Planned (from final-plan.md):
- Step 2.2: "Track function calls in parseFunctionSignature method"

### Actual:
- Tracked function calls in `expandAssignment()` and `expandReturn()` instead
- Reason: These methods already extract the expression, making tracking more accurate
- `parseFunctionSignature()` parses function declarations, not function calls

### Added (not in plan):
- Created `trackFunctionCallInExpr()` helper method
- Simplified extraction logic using string operations instead of AST parsing

## Performance Considerations

1. **Lazy Import Injection**: Only parse and inject imports if actually needed
2. **Deduplication**: Use map to eliminate duplicate imports before injection
3. **Sorted Imports**: Maintain consistent output order (important for tests)
4. **Single Pass**: All processors run once, imports collected, injected once

## Future Enhancements

Potential improvements (not implemented in this task):

1. **User-defined function mapping**: Allow users to specify custom function→package mappings
2. **Package qualification detection**: Track `os.ReadFile` vs `ReadFile` usage
3. **Import aliasing support**: Handle `import foo "github.com/bar/foo"`
4. **Optimization**: Only track functions in expressions with `?` operator
5. **Context-aware tracking**: Different import needs for different features

## Edge Cases Handled

1. **No imports needed**: Return original source unchanged (no blank import block)
2. **Existing imports**: `astutil.AddImport` handles duplicates automatically
3. **Invalid Go syntax**: Delayed injection ensures parser always succeeds
4. **Multiple processors**: Collect imports from all, deduplicate, inject once
5. **fmt.Errorf special case**: Tracked separately via needsFmt flag, merged correctly
