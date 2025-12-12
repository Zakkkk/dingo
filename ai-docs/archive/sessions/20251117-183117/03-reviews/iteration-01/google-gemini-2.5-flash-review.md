# Code Review: Phase 2.2 Error Propagation Implementation
**Reviewer:** Google Gemini 2.5 Flash
**Date:** 2025-11-17
**Session:** 20251117-183117
**Status:** CHANGES_NEEDED

## Executive Summary

This review covers the Phase 2.2 implementation which adds complete error propagation functionality to the Dingo transpiler. The implementation successfully achieves the stated goals (all 8 error_prop tests passing), but contains several critical issues that should be addressed before merging. The code demonstrates good architectural design with the preprocessor pipeline, but has bugs in edge case handling, potential performance issues with regex compilation, and some non-idiomatic Go patterns.

**Overall Assessment:** The implementation is functionally correct for the tested cases but needs refinement in error handling, performance optimization, and edge case coverage.

---

## CRITICAL Issues (3)

### CRITICAL-1: Multi-line Function Signature Parsing Bug
**File:** `pkg/preprocessor/error_prop.go`
**Lines:** 278-287

**Issue:**
The `parseFunctionSignature()` method collects lines until it finds an opening brace `{`, but this logic fails for:
1. Functions with no body (interface methods, function signatures in type declarations)
2. Functions where the opening brace is on the same line as parameters
3. Edge case: what if `{` appears in a comment or string literal within the signature?

```go
for i := startLine; i < len(e.lines); i++ {
    funcText.WriteString(e.lines[i])
    funcText.WriteString("\n")
    if strings.Contains(e.lines[i], "{") {  // ❌ Too simplistic
        break
    }
}
```

**Impact:**
- Function signature parsing will fail for interface methods
- Infinite loop if no `{` is found (exhausts e.lines array)
- Could incorrectly parse signatures with `{` in comments

**Recommended Fix:**
```go
func (e *ErrorPropProcessor) parseFunctionSignature(startLine int) *funcContext {
    var funcText strings.Builder
    foundBrace := false

    for i := startLine; i < len(e.lines) && i < startLine+20; i++ {  // Add safety limit
        funcText.WriteString(e.lines[i])
        funcText.WriteString("\n")

        // More robust brace detection - check it's not in a comment
        trimmed := strings.TrimSpace(e.lines[i])
        if idx := strings.Index(trimmed, "{"); idx != -1 {
            // Quick check: not inside a comment
            if !strings.HasPrefix(trimmed, "//") {
                foundBrace = true
                break
            }
        }
    }

    if !foundBrace {
        // Fallback for signatures without bodies
        return &funcContext{
            returnTypes: []string{},
            zeroValues:  []string{"nil"},
        }
    }
    // ... rest of parsing
}
```

---

### CRITICAL-2: Regex Compilation Performance Issue
**File:** `pkg/preprocessor/error_prop.go`
**Lines:** 95, 106, 126; `pkg/preprocessor/type_annot.go` Line 71; `pkg/preprocessor/keywords.go` Line 24

**Issue:**
Regular expressions are compiled inside methods that are called for every line of source code. This creates significant performance overhead:

```go
func (e *ErrorPropProcessor) processLine(line string, lineNum int) (string, *Mapping) {
    assignPattern := regexp.MustCompile(`^\s*(let|var)\s+(\w+)\s*=\s*(.+)$`)  // ❌ Compiled on every call
    returnPattern := regexp.MustCompile(`^\s*return\s+(.+)$`)                 // ❌ Compiled on every call
    msgPattern := regexp.MustCompile(`^(.*\?)\s*"((?:[^"\\]|\\.)*)"`)        // ❌ Compiled on every call
}
```

**Impact:**
- O(n) regex compilations where n = lines of code
- For a 1000-line file, compiles the same regex 1000 times
- Measurable performance degradation on large codebases
- Violates Go best practice: compile regexes once, reuse many times

**Recommended Fix:**
```go
// At package level
var (
    assignPattern = regexp.MustCompile(`^\s*(let|var)\s+(\w+)\s*=\s*(.+)$`)
    returnPattern = regexp.MustCompile(`^\s*return\s+(.+)$`)
    msgPattern    = regexp.MustCompile(`^(.*\?)\s*"((?:[^"\\]|\\.)*)"`)
)

// In processLine, just use the pre-compiled patterns
func (e *ErrorPropProcessor) processLine(line string, lineNum int) (string, *Mapping) {
    if matches := assignPattern.FindStringSubmatch(line); matches != nil {
        // ...
    }
}
```

Apply same fix to:
- `type_annot.go` line 71: `pattern := regexp.MustCompile(...)`
- `keywords.go` line 24: `letPattern := regexp.MustCompile(...)`

---

### CRITICAL-3: Import Detection Bug in ensureFmtImport
**File:** `pkg/preprocessor/error_prop.go`
**Lines:** 417-447

**Issue:**
The import detection logic has false positives and incorrect insertion logic:

```go
// Line 421: This will match "fmt" anywhere in the file, including:
// - Variable names: var fmtString = "hello"
// - Comments: // Use fmt package
// - String literals: msg := "use fmt.Println"
if strings.Contains(sourceStr, `import "fmt"`) || strings.Contains(sourceStr, `"fmt"`) {
    return source
}
```

Additionally, the import insertion logic (lines 434-442) doesn't properly handle existing import blocks:

```go
if i+1 < len(lines) && strings.Contains(lines[i+1], "import") {
    // Insert into existing import block
    continue  // ❌ Does nothing! Should actually insert into the block
} else {
    // Add standalone import
    result.WriteString("\nimport \"fmt\"\n")
}
```

**Impact:**
- Will skip adding import if "fmt" appears in a comment or string
- Fails to insert into existing import blocks correctly
- May create malformed import blocks or duplicate imports
- Generated code may not compile if fmt.Errorf is used but import is skipped

**Recommended Fix:**
```go
func (e *ErrorPropProcessor) ensureFmtImport(source []byte) []byte {
    // Use go/parser to properly detect imports
    fset := token.NewFileSet()
    file, err := parser.ParseFile(fset, "", source, parser.ImportsOnly)
    if err != nil {
        // Fallback: simple string check (safe enough for this case)
        if bytes.Contains(source, []byte(`"fmt"`)) {
            return source
        }
        return e.addFmtImportSimple(source)
    }

    // Check if fmt is already imported
    for _, imp := range file.Imports {
        if imp.Path.Value == `"fmt"` {
            return source  // Already imported
        }
    }

    // Use golang.org/x/tools/go/ast/astutil.AddImport for robust insertion
    // Or implement simple version:
    return e.addFmtImportSimple(source)
}

func (e *ErrorPropProcessor) addFmtImportSimple(source []byte) []byte {
    lines := bytes.Split(source, []byte("\n"))
    var result bytes.Buffer

    importAdded := false
    for i, line := range lines {
        result.Write(line)
        if i < len(lines)-1 {
            result.WriteByte('\n')
        }

        // After package declaration
        if bytes.HasPrefix(bytes.TrimSpace(line), []byte("package ")) && !importAdded {
            result.WriteString("\nimport \"fmt\"\n")
            importAdded = true
        }
    }

    return result.Bytes()
}
```

---

## IMPORTANT Issues (3)

### IMPORTANT-1: Missing Error Wrapping Context
**File:** `pkg/preprocessor/error_prop.go`
**Line:** 261

**Issue:**
When generating wrapped errors with `fmt.Errorf`, the code uses `%%w` which is correct, but the format string doesn't follow Go error wrapping best practices. The current implementation:

```go
errPart = fmt.Sprintf(`fmt.Errorf("%s: %%w", %s)`, errMsg, errVar)
// Generates: fmt.Errorf("failed to read: %w", __err0)
```

This is correct but could be improved. However, the more important issue is that error messages aren't escaped. If `errMsg` contains `%` characters, it will break the format string:

```dingo
data := ReadFile(path)? "failed: 50% complete"
// Generates: fmt.Errorf("failed: 50% complete: %w", __err0)
// Runtime error: fmt.Errorf: missing argument
```

**Impact:**
- Error messages with `%` will cause runtime panics
- Error messages with quotes or backslashes may not be escaped properly
- Medium severity: affects specific error messages but is easy to hit

**Recommended Fix:**
```go
func (e *ErrorPropProcessor) generateReturnStatement(errVar string, errMsg string) string {
    // ... existing zero value logic ...

    var errPart string
    if errMsg != "" {
        // Escape % characters in the error message
        escapedMsg := strings.ReplaceAll(errMsg, "%", "%%")
        e.needsFmt = true
        errPart = fmt.Sprintf(`fmt.Errorf("%s: %%w", %s)`, escapedMsg, errVar)
    } else {
        errPart = errVar
    }

    return fmt.Sprintf("return %s, %s", strings.Join(zeroVals, ", "), errPart)
}
```

---

### IMPORTANT-2: Type Annotation Regex Too Restrictive
**File:** `pkg/preprocessor/type_annot.go`
**Line:** 71

**Issue:**
The regex pattern for type annotations is overly restrictive and won't match many valid Go types:

```go
pattern := regexp.MustCompile(`(\w+)\s*:\s*(\w+|[\[\]\*\{\}]+[\w\.\[\]\*\{\}]*)`)
```

This pattern fails for:
- Qualified types: `x: pkg.Type` (dot in middle of type)
- Generic types: `x: Container[T]` (brackets with type parameter)
- Complex types: `x: []map[string]*pkg.Type`
- Function types: `x: func(int) error`

**Impact:**
- Type annotations for complex types won't be converted
- Will cause parse errors when Dingo code uses these types
- Limits expressiveness of Dingo language
- Medium severity: affects specific type patterns

**Recommended Fix:**
```go
func (t *TypeAnnotProcessor) replaceColonInParams(params []byte) []byte {
    // Use a more robust approach: scan for identifier:type patterns
    // and extract the type portion using balanced bracket matching

    result := bytes.Buffer{}
    i := 0

    for i < len(params) {
        // Find next colon
        colonPos := bytes.IndexByte(params[i:], ':')
        if colonPos == -1 {
            result.Write(params[i:])
            break
        }
        colonPos += i

        // Find identifier before colon (scan backwards)
        identStart := colonPos - 1
        for identStart >= 0 && (isIdentChar(params[identStart]) || params[identStart] == ' ') {
            identStart--
        }
        identStart++

        // Extract type after colon (scan forward with bracket balancing)
        typeStart := colonPos + 1
        typeEnd := t.findTypeEnd(params, typeStart)

        // Write: identifier<space>type (without colon)
        result.Write(params[i:identStart])
        result.Write(bytes.TrimSpace(params[identStart:colonPos]))
        result.WriteByte(' ')
        result.Write(bytes.TrimSpace(params[typeStart:typeEnd]))

        i = typeEnd
    }

    return result.Bytes()
}

func (t *TypeAnnotProcessor) findTypeEnd(params []byte, start int) int {
    // Scan forward, tracking [], {}, () balance
    depth := 0
    for i := start; i < len(params); i++ {
        switch params[i] {
        case '[', '{', '(':
            depth++
        case ']', '}', ')':
            depth--
        case ',':
            if depth == 0 {
                return i  // End of this parameter
            }
        }
    }
    return len(params)
}
```

---

### IMPORTANT-3: Missing Bounds Check in parseFunctionSignature
**File:** `pkg/preprocessor/error_prop.go`
**Lines:** 280-287

**Issue:**
The loop that collects function signature lines has no bounds check:

```go
for i := startLine; i < len(e.lines); i++ {
    funcText.WriteString(e.lines[i])
    funcText.WriteString("\n")
    if strings.Contains(e.lines[i], "{") {
        break
    }
}
// If no { is found, loops through entire file
```

**Impact:**
- If a function declaration has no body (interface methods), this scans the entire rest of the file
- Performance degradation on large files
- Unnecessary work that slows down preprocessing

**Recommended Fix:**
Add a reasonable limit (e.g., 20 lines for function signature):

```go
for i := startLine; i < len(e.lines) && i < startLine+20; i++ {
    funcText.WriteString(e.lines[i])
    funcText.WriteString("\n")
    if strings.Contains(e.lines[i], "{") {
        break
    }
}
```

---

## MINOR Issues (7)

### MINOR-1: Inconsistent Error Handling Pattern
**File:** `pkg/preprocessor/error_prop.go`
**Lines:** 293-298

**Issue:**
The function returns a fallback `funcContext` when parsing fails, but doesn't log the error. This silently swallows parsing errors which could indicate bugs in the source code.

```go
file, err := parser.ParseFile(fset, "", src, 0)
if err != nil {
    // Failed to parse, use nil fallback
    return &funcContext{
        returnTypes: []string{},
        zeroValues:  []string{"nil"},
    }
}
```

**Recommended Fix:**
```go
if err != nil {
    // Log the error for debugging (if logger is available)
    // Or at least comment why we're silently failing
    // For now, fallback is safe - use nil for unknown types
    return &funcContext{
        returnTypes: []string{},
        zeroValues:  []string{"nil"},
    }
}
```

---

### MINOR-2: Unused ScanContext and Buffer Structs
**File:** `pkg/preprocessor/preprocessor.go`
**Lines:** 85-217

**Issue:**
The `ScanContext` and `Buffer` helper structs are defined but never used in the codebase. This adds ~130 lines of dead code.

**Impact:**
- Code bloat
- Maintenance burden for unused code
- May confuse future developers

**Recommended Fix:**
Either use these utilities in the preprocessor implementations, or remove them until needed. If they're for future use, add a comment:

```go
// ScanContext provides utilities for scanning and transforming source code
// NOTE: Not currently used but reserved for future processor implementations
type ScanContext struct {
    // ...
}
```

---

### MINOR-3: Function Name Mismatch
**File:** `pkg/parser/simple.go`
**Line:** 14

**Issue:**
The function is named `newParticipleParser` but it doesn't use participle - it uses `go/parser`:

```go
func newParticipleParser(mode Mode) Parser {
    return &simpleParser{mode: mode}
}
```

**Impact:**
- Confusing naming that doesn't match implementation
- May mislead developers about the parsing approach

**Recommended Fix:**
```go
func newSimpleParser(mode Mode) Parser {
    return &simpleParser{mode: mode}
}
```

Or update the call site that references this function.

---

### MINOR-4: Incomplete ParseExpr Implementation
**File:** `pkg/parser/simple.go`
**Lines:** 36-39

**Issue:**
`ParseExpr` returns `nil, nil` which violates Go idioms (should return error if not implemented):

```go
func (p *simpleParser) ParseExpr(fset *token.FileSet, expr string) (dingoast.DingoNode, error) {
    // Not implemented for now
    return nil, nil
}
```

**Recommended Fix:**
```go
func (p *simpleParser) ParseExpr(fset *token.FileSet, expr string) (dingoast.DingoNode, error) {
    return nil, fmt.Errorf("ParseExpr not yet implemented")
}
```

---

### MINOR-5: Missing Documentation
**Files:** Multiple

**Issue:**
Several exported types and functions lack godoc comments:
- `pkg/ast/file.go` Line 11: `DingoNode` interface has no doc comment
- `pkg/plugin/plugin.go` Line 10: `Registry` struct has no doc comment
- `pkg/plugin/plugin.go` Line 41: `Stats` struct has no doc comment

**Recommended Fix:**
Add godoc comments for all exported types:

```go
// DingoNode is a marker interface for Dingo-specific AST nodes.
// It allows distinguishing Dingo extensions from standard Go AST nodes.
type DingoNode interface {
    Node()
}
```

---

### MINOR-6: Hard-coded String in Multiple Places
**File:** `pkg/preprocessor/error_prop.go`
**Lines:** 155, 172, 209, 226

**Issue:**
The marker comments `// dingo:s:1` and `// dingo:e:1` are hard-coded in multiple places. Should be constants:

```go
buf.WriteString("// dingo:s:1\n")
// ...
buf.WriteString("// dingo:e:1\n")
```

**Recommended Fix:**
```go
const (
    markerStart = "// dingo:s:1"
    markerEnd   = "// dingo:e:1"
)

// Then use:
buf.WriteString(indent)
buf.WriteString(markerStart)
buf.WriteString("\n")
```

---

### MINOR-7: Potential Panic in Type Annotation Processor
**File:** `pkg/preprocessor/type_annot.go`
**Lines:** 41-43

**Issue:**
The code doesn't check if `closeParen` is actually greater than `openParen` before slicing:

```go
if openParen != -1 && closeParen != -1 && closeParen > openParen {
    // This check is good
    before := line[:openParen+1]
    params := line[openParen+1:closeParen]  // Safe due to check
    after := line[closeParen:]
}
```

Actually, this is correct! The check `closeParen > openParen` prevents the panic. However, there's an edge case: what if there are multiple opening/closing parens on the same line? `bytes.LastIndexByte` will find the LAST `)`, which might not match the first `(`.

**Example:**
```go
func foo(x: int) (string, error)
//       ^openParen    ^closeParen (LastIndexByte finds this one)
```

This would process `) (string, error)` as parameters, which is wrong.

**Recommended Fix:**
```go
// Find matching closing paren (not just last paren)
if openParen != -1 {
    closeParen := findMatchingParen(line, openParen)
    if closeParen != -1 {
        // Process parameters
    }
}

func findMatchingParen(line []byte, openPos int) int {
    depth := 1
    for i := openPos + 1; i < len(line); i++ {
        if line[i] == '(' {
            depth++
        } else if line[i] == ')' {
            depth--
            if depth == 0 {
                return i
            }
        }
    }
    return -1
}
```

---

## Positive Observations

1. **Good Architecture**: The preprocessor pipeline design is clean and extensible
2. **Proper Use of go/ast**: Using `go/parser` and `go/types` for type inference is the right approach
3. **Source Maps**: Good forward-thinking to generate source maps for LSP support
4. **Test Coverage**: All 8 error_prop tests passing indicates good functional correctness
5. **Zero Value Inference**: Smart use of AST parsing to generate correct zero values
6. **Marker Comments**: The `// dingo:s:1` and `// dingo:e:1` markers are a clever solution for source map folding

---

## Recommendations Summary

### High Priority (Fix Before Merge)
1. Fix regex compilation performance (CRITICAL-2) - Compile at package level
2. Fix import detection bug (CRITICAL-3) - Use proper AST-based detection
3. Fix multi-line function parsing (CRITICAL-1) - Add bounds checking and robust brace detection

### Medium Priority (Fix Soon)
4. Fix error message escaping (IMPORTANT-1) - Escape `%` in error messages
5. Fix type annotation regex (IMPORTANT-2) - Support complex types
6. Add bounds check to function parsing (IMPORTANT-3) - Prevent infinite loops

### Low Priority (Nice to Have)
7. Remove unused code or document future use (MINOR-2)
8. Fix function naming inconsistency (MINOR-3)
9. Add proper error returns (MINOR-4)
10. Add godoc comments (MINOR-5)
11. Use constants for marker strings (MINOR-6)
12. Fix paren matching edge case (MINOR-7)

---

## Conclusion

The implementation successfully achieves its goals and demonstrates solid software engineering practices. The preprocessor architecture is well-designed and the use of Go's standard library (`go/ast`, `go/parser`, `go/types`) is appropriate.

However, the critical issues around regex compilation performance, import detection, and function parsing need to be addressed before this code is production-ready. These are not just theoretical concerns - they will cause actual bugs and performance problems in real-world usage.

With the recommended fixes applied, this implementation will be robust, performant, and maintainable.

---

**STATUS:** CHANGES_NEEDED
**CRITICAL_COUNT:** 3
**IMPORTANT_COUNT:** 3
**MINOR_COUNT:** 7
