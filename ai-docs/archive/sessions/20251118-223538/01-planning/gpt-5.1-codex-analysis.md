# GPT-5.1 Codex Analysis: LSP Source Mapping Bug

**Model**: openai/gpt-5.1-codex
**Analysis Date**: 2025-11-18
**Session**: 20251118-223538

---

## Root Cause Identified

The error-prop preprocessor records **every generated line under a single `error_prop` mapping** tied to the `?` operator. When diagnostics occur on the generated line:

```go
__tmpN, __errN := ReadFile(path)
```

The system falls back to the operator range (the `?`) instead of the actual function call (`ReadFile`).

---

## Detailed Explanation

### What Happens

1. **gopls reports error** at column 0 of the generated assignment line:
   ```go
   data, err := ReadFile(path)  // Column 0 = start of line
   ```

2. **MapToOriginal() searches** for a mapping on that line
   - Finds only the `error_prop` mapping tied to `?` operator
   - No closer/more specific span exists

3. **LSP translation forwards** the operator mapping
   - Result: IDE underlines `?` instead of `ReadFile`

### Why This Happens

The preprocessor creates a **single coarse-grained mapping**:
- **Original**: `ReadFile(path)?` (entire expression)
- **Generated**: Multiple lines of expanded code
- **Mapping**: All lines → `?` operator position

When an error occurs on the **first generated line** (the function call), there's no mapping that points back to `ReadFile` specifically—only to the `?`.

---

## Fix Design (High-Level)

### Approach: Granular Mapping Segments

Emit **multiple fine-grained mappings** instead of one coarse mapping:

1. **Map the function call separately**:
   - Original: `ReadFile(path)` → position of `ReadFile`
   - Generated: `data, err := ReadFile(path)` → position of `ReadFile`

2. **Map the error check separately**:
   - Original: `?` operator → position of `?`
   - Generated: `if err != nil { return nil, err }` → position of `?`

3. **Optional: Use mapping tags/names**:
   - Tag mappings: `"function_call"`, `"error_check"`
   - Helps disambiguation during translation

### Benefits

- Diagnostics on `ReadFile(path)` line → maps to `ReadFile` in .dingo
- Diagnostics on `if err != nil` line → maps to `?` in .dingo
- More accurate, intuitive error reporting

---

## Test Strategy

### 1. Unit Tests (SourceMap)

Add test coverage for **column-level precision**:

```go
func TestMapToOriginal_ErrorPropAssignment(t *testing.T) {
    sm := &SourceMap{
        Mappings: []Mapping{
            {GeneratedLine: 4, GeneratedColumn: 15, OriginalLine: 4, OriginalColumn: 12, Length: 8, Name: "ReadFile"},
            {GeneratedLine: 5, GeneratedColumn: 2, OriginalLine: 4, OriginalColumn: 26, Length: 1, Name: "error_prop"},
        },
    }

    // Test: Error at start of assignment (column 0) should map to function
    origLine, origCol := sm.MapToOriginal(4, 15) // ReadFile position
    assert.Equal(t, 4, origLine)
    assert.Equal(t, 12, origCol) // Position of 'ReadFile', not '?'
}
```

### 2. Integration Tests (LSP)

Simulate **undefined function error** and verify diagnostic position:

```go
func TestLSP_UndefinedFunctionError(t *testing.T) {
    // Given: .dingo file with undefined function
    dingoCode := `
    func readConfig(path string) ([]byte, error) {
        data := UndefinedFunc(path)?  // UndefinedFunc doesn't exist
        return data, nil
    }
    `

    // When: gopls reports error
    // Then: Diagnostic should underline "UndefinedFunc", not "?"
    diag := getDiagnostic(dingoCode)

    assert.Equal(t, "UndefinedFunc", extractUnderlinedText(dingoCode, diag.Range))
    assert.NotEqual(t, "?", extractUnderlinedText(dingoCode, diag.Range))
}
```

### 3. Manual Testing

1. Create `.dingo` file with undefined function in `?` expression
2. Open in VS Code with dingo-lsp
3. Verify error underlines **function name**, not `?` operator
4. Test with various error types (undefined function, type mismatch, etc.)

---

## Implementation Checklist

- [ ] Update error_prop preprocessor to emit granular mappings
- [ ] Create mapping for function call specifically
- [ ] Create separate mapping for error check
- [ ] Add unit tests for SourceMap column precision
- [ ] Add LSP integration test for diagnostic position
- [ ] Manual testing in VS Code
- [ ] Update documentation on mapping strategy

---

## Summary

**Problem**: Single coarse-grained mapping per error propagation
**Cause**: All generated lines map to `?` operator position
**Solution**: Emit separate mappings for function call vs error check
**Validation**: Unit tests + LSP integration test + manual verification

---

*Analysis provided by GPT-5.1 Codex via claudish*
