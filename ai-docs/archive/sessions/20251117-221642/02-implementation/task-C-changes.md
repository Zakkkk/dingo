# Task C: Import Detection False Positives - Changes Summary

## Files Modified

### 1. `/Users/jack/mag/dingo/pkg/preprocessor/error_prop.go`

**Lines 29-80: Updated `stdLibFunctions` map**
- Removed ALL bare function name entries (ReadFile, Marshal, Atoi, etc.)
- Kept ONLY package-qualified entries (os.ReadFile, json.Marshal, strconv.Atoi, etc.)
- Updated documentation comment to clarify this prevents false positives
- Total reduction: 40 entries removed, 40 qualified entries retained

**Lines 834-867: Updated `trackFunctionCallInExpr` function**
- Removed logic that tracked bare function names
- Now ONLY tracks qualified calls (pkg.Function pattern)
- Updated documentation to reflect new behavior
- Simplified implementation by removing fallback tracking

### 2. `/Users/jack/mag/dingo/pkg/preprocessor/preprocessor_test.go`

Updated 10 test cases to use qualified function names:
- `TestErrorPropagationBasic` (2 tests)
- `TestIMPORTANT1_ErrorMessageEscaping` (1 test)
- `TestGeminiCodeReviewFixes` (1 test, added net/http import check)
- `TestSourceMapGeneration` (1 test)
- `TestSourceMapMultipleExpansions` (1 test)
- `TestAutomaticImportDetection` (4 tests)
- `TestSourceMappingWithImports` (1 test)
- `TestCRITICAL2_MultiValueReturnHandling` (1 test)

Changed patterns:
- `ReadFile(path)` → `os.ReadFile(path)`
- `Atoi(s)` → `strconv.Atoi(s)`
- `HttpGet(url)` → `http.Get(url)`

## Impact

**Before:**
- `ReadFile(path)?` → Incorrectly injected `import "os"` even for user-defined ReadFile functions
- Caused "unused import" compile errors

**After:**
- `ReadFile(path)?` → No import injection (user-defined function)
- `os.ReadFile(path)?` → Correctly injects `import "os"` (stdlib function)

## Test Results

All tests pass:
```
PASS
ok  	github.com/MadAppGang/dingo/pkg/preprocessor	0.474s
```

## Breaking Change Notice

Users must now use fully-qualified stdlib function names in Dingo code:
- WRONG: `data := ReadFile(path)?`
- CORRECT: `data := os.ReadFile(path)?`

This is the expected behavior and prevents false positive imports.
