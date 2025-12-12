# Dingo LSP Source Mapping Bug Investigation

## Problem Statement

The Dingo language server is underlining the WRONG part of code when there's an error.

**Expected behavior:**
- Should underline `ReadFile` (the function call that can fail)

**Actual behavior:**
- Underlines `e(path)?` instead (specifically the `?` operator at position col 15)

**Test case:** `tests/golden/error_prop_01_simple.dingo`

## Source Code Context

### Original Dingo Code (error_prop_01_simple.dingo)
```dingo
package main

func readConfig(path string) ([]byte, error) {
	data := ReadFile(path)?
	return data, nil
}
```

**Key positions:**
- Line 4, column 13: Start of `ReadFile`
- Line 4, column 15: The `?` operator (error propagation)
- Line 4, column 26: End of `ReadFile(path)`

### Generated Go Code (error_prop_01_simple.go)
```go
package main

func readConfig(path string) ([]byte, error) {
	__tmp0, __err0 := ReadFile(path)
	// dingo:s:1
	if __err0 != nil {
		return nil, __err0
	}
	// dingo:e:1
	var data = __tmp0
	return data, nil
}
```

**Key positions:**
- Line 4, column 1: Start of `__tmp0, __err0 :=`
- Line 4, column 20: Start of `ReadFile(path)`
- Lines 5-9: Error handling block (generated from `?` operator)
- Line 10: Variable declaration

### Source Map (error_prop_01_simple.go.map)
```json
{
  "version": 1,
  "mappings": [
    {
      "generated_line": 4,
      "generated_column": 20,
      "original_line": 4,
      "original_column": 13,
      "length": 14,
      "name": "expr_mapping"
    },
    {
      "generated_line": 4,
      "generated_column": 1,
      "original_line": 4,
      "original_column": 15,
      "length": 1,
      "name": "error_prop"
    },
    {
      "generated_line": 5,
      "generated_column": 1,
      "original_line": 4,
      "original_column": 15,
      "length": 1,
      "name": "error_prop"
    },
    {
      "generated_line": 6,
      "generated_column": 1,
      "original_line": 4,
      "original_column": 15,
      "length": 1,
      "name": "error_prop"
    },
    {
      "generated_line": 7,
      "generated_column": 1,
      "original_line": 4,
      "original_column": 15,
      "length": 1,
      "name": "error_prop"
    },
    {
      "generated_line": 8,
      "generated_column": 1,
      "original_line": 4,
      "original_column": 15,
      "length": 1,
      "name": "error_prop"
    },
    {
      "generated_line": 9,
      "generated_column": 1,
      "original_line": 4,
      "original_column": 15,
      "length": 1,
      "name": "error_prop"
    },
    {
      "generated_line": 10,
      "generated_column": 1,
      "original_line": 4,
      "original_column": 15,
      "length": 1,
      "name": "error_prop"
    }
  ]
}
```

## Root Cause Analysis Questions

When gopls reports an error on `ReadFile(path)` at line 4, column 20 in the .go file:

1. **What position does the source map return?**
   - The mapping shows: generated_line=4, generated_column=20 → original_line=4, original_column=13
   - This SHOULD correctly map to `ReadFile` start position

2. **But there are conflicting mappings:**
   - Mapping 1: gen_col=20 → orig_col=13 (length 14) - "expr_mapping" for `ReadFile(path)`
   - Mapping 2: gen_col=1 → orig_col=15 (length 1) - "error_prop" for `?` operator

3. **When gopls reports error at column 20:**
   - Should match Mapping 1 (exact range match: col 20-34)
   - But algorithm might choose Mapping 2 as "bestMatch" first?

## Your Task

Please analyze this source mapping bug and provide:

1. **Root cause identification:**
   - Why is the LSP underlining column 15 (`?`) instead of column 13 (`ReadFile`)?
   - Is the bug in source map generation or position translation?
   - Is the bug in the algorithm logic or mapping data structure?

2. **Step-by-step trace:**
   - When gopls reports error at (line=4, col=20)
   - What does MapToOriginal return?
   - Which mapping does it choose and why?

3. **Fix design:**
   - What changes are needed to sourcemap.go?
   - What changes are needed to source map generation?
   - Should mappings be ordered differently?
   - Should the algorithm prioritize exact range matches over "closest"?

4. **Test case validation:**
   - How to verify the fix works correctly?
   - What edge cases should be considered?

Please provide a detailed technical analysis with specific code recommendations.
