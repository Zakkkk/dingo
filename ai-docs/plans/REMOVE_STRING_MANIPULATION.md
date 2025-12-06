# MANDATORY: Remove ALL String Manipulation from Dingo Transpiler

**Created**: 2025-12-06
**Status**: ACTIVE - ALL AGENTS MUST FOLLOW
**Priority**: P0 - BLOCKING

---

## THE RULE (READ THIS FIRST)

```
╔══════════════════════════════════════════════════════════════════════════════╗
║                                                                              ║
║   FORBIDDEN: Any code that searches/parses source using:                    ║
║   • bytes.Index(), strings.Index(), strings.Contains()                      ║
║   • regexp.MustCompile(), regexp.Match()                                    ║
║   • Character-by-character scanning loops                                   ║
║   • strings.Split(), strings.TrimSpace() for parsing                        ║
║   • Any function that takes []byte and searches for syntax patterns         ║
║                                                                              ║
║   REQUIRED: All parsing MUST use:                                           ║
║   • pkg/tokenizer/ - Tokenize source into tokens                            ║
║   • pkg/parser/ - Parse tokens into AST nodes using Pratt parser            ║
║   • Codegens only accept AST nodes, never []byte source                     ║
║                                                                              ║
╚══════════════════════════════════════════════════════════════════════════════╝
```

---

## WHY THIS MATTERS

We have removed "string manipulation" THREE TIMES and it kept coming back because:

1. Agents created codegens with `Find*()` functions using `bytes.Index()`
2. Agents created `Parse*()` functions using `strings.Split()`
3. The REAL parser in `pkg/parser/` (5,329 lines) was NEVER connected to codegens
4. Code LOOKED like AST (had structs like `MatchExpr`) but PARSED using strings

**This time we fix it properly.**

---

## THE ARCHITECTURE (CORRECT)

```
┌─────────────────────────────────────────────────────────────────┐
│                        SOURCE CODE                               │
│                    ([]byte or string)                            │
└─────────────────────────────┬───────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                     pkg/tokenizer/                               │
│                                                                  │
│  Tokenize(src []byte) []Token                                   │
│                                                                  │
│  THIS IS THE ONLY PLACE THAT READS RAW BYTES                    │
│  Outputs: []Token with Type, Literal, Position                  │
└─────────────────────────────┬───────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                      pkg/parser/                                 │
│                                                                  │
│  PrattParser.Parse(tokens []Token) *ast.File                    │
│                                                                  │
│  - pratt.go: Pratt expression parser                            │
│  - match.go: Match expression parsing                           │
│  - lambda.go: Lambda expression parsing                         │
│  - stmt.go: Statement parsing                                   │
│  - enum.go: Enum declaration parsing                            │
│                                                                  │
│  READS ONLY TOKENS, NEVER RAW BYTES                             │
│  Outputs: AST nodes (MatchExpr, LambdaExpr, etc.)               │
└─────────────────────────────┬───────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                      pkg/ast/codegen/                            │
│                                                                  │
│  Generate(node ast.Node) CodeGenResult                          │
│                                                                  │
│  - match_codegen.go: MatchExpr → Go switch                      │
│  - lambda_codegen.go: LambdaExpr → Go func literal              │
│  - error_prop_codegen.go: ErrorPropExpr → Go if err             │
│                                                                  │
│  READS ONLY AST NODES, NEVER RAW BYTES OR TOKENS                │
│  Outputs: Go source code + SourceMappings                       │
└─────────────────────────────┬───────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                      OUTPUT                                      │
│                  Valid Go source code                            │
└─────────────────────────────────────────────────────────────────┘
```

---

## VERIFICATION CHECKLIST (MANDATORY)

Before ANY codegen code is considered complete, verify:

### Checklist 1: No Forbidden Patterns

```bash
# Run this command - output MUST be empty
grep -rn "bytes\.Index\|strings\.Index\|strings\.Contains\|regexp\.\|bytes\.Contains" pkg/ast/*_codegen.go

# If ANY matches found → CODE IS WRONG, REWRITE IT
```

### Checklist 2: Imports Are Correct

Every `*_codegen.go` file MUST have:
```go
import (
    "github.com/MadAppGang/dingo/pkg/ast"  // AST node types
    // NO "regexp"
    // NO "bytes" (except bytes.Buffer for output)
    // NO "strings" (except strings.Builder for output)
)
```

Every `*_codegen.go` file MUST NOT have:
```go
// FORBIDDEN IMPORTS:
import "regexp"                    // ❌ NEVER
import "bytes"                     // ❌ Only bytes.Buffer for OUTPUT allowed
// bytes.Index, bytes.Contains = FORBIDDEN
```

### Checklist 3: Function Signatures Are Correct

```go
// ✅ CORRECT - Codegen takes AST node
func (g *MatchCodeGen) Generate(expr *ast.MatchExpr) CodeGenResult

// ❌ WRONG - Codegen takes raw bytes
func TransformMatchSource(src []byte) ([]byte, []SourceMapping)

// ❌ WRONG - Find function exists at all
func FindMatchExpressions(src []byte) []int
```

### Checklist 4: No Scanning Loops

```go
// ❌ WRONG - Character scanning
for i := 0; i < len(src); i++ {
    if src[i] == '?' { ... }
}

// ❌ WRONG - Keyword searching
idx := bytes.Index(src, []byte("match "))

// ✅ CORRECT - Work with tokens/AST only
for _, token := range tokens {
    if token.Type == tokenizer.MATCH { ... }
}
```

---

## FILES TO DELETE

These files contain string manipulation and must be DELETED, not refactored:

```
pkg/ast/error_prop_codegen.go   - Has FindErrorPropExpressions() with byte scanning
pkg/ast/lambda_codegen.go       - Has regexp.MustCompile()
pkg/ast/let_codegen.go          - Has FindLetDeclarations() with byte scanning
pkg/ast/match_codegen.go        - Has FindMatchExpressions() with bytes.Index
pkg/ast/null_coalesce_codegen.go - Has byte scanning
pkg/ast/safe_nav_codegen.go     - Has byte scanning
pkg/ast/ternary_codegen.go      - Has byte scanning
pkg/ast/tuple_codegen.go        - Has byte scanning
pkg/ast/transform.go            - Orchestrates the wrong approach
```

**DO NOT REFACTOR THESE FILES. DELETE THEM AND START FRESH.**

---

## FILES TO USE (ALREADY CORRECT)

These files contain REAL parsing and should be the foundation:

```
pkg/tokenizer/tokenizer.go      - Real tokenizer (USE THIS)
pkg/tokenizer/tokens.go         - Token definitions (USE THIS)
pkg/parser/pratt.go             - Pratt expression parser (USE THIS)
pkg/parser/match.go             - Match parsing (USE THIS)
pkg/parser/lambda.go            - Lambda parsing (USE THIS)
pkg/parser/stmt.go              - Statement parsing (USE THIS)
pkg/parser/enum.go              - Enum parsing (USE THIS)
pkg/ast/enum_codegen.go         - ONLY codegen that's correct (KEEP THIS)
```

---

## IMPLEMENTATION PLAN

### Phase 1: Delete Wrong Code

```bash
# Delete all string-manipulation codegens
rm pkg/ast/error_prop_codegen.go
rm pkg/ast/lambda_codegen.go
rm pkg/ast/let_codegen.go
rm pkg/ast/match_codegen.go
rm pkg/ast/null_coalesce_codegen.go
rm pkg/ast/safe_nav_codegen.go
rm pkg/ast/ternary_codegen.go
rm pkg/ast/tuple_codegen.go
rm pkg/ast/transform.go
rm pkg/ast/helpers.go
```

### Phase 2: Create Correct Pipeline

Create `pkg/transpiler/pipeline.go`:

```go
package transpiler

import (
    "github.com/MadAppGang/dingo/pkg/tokenizer"
    "github.com/MadAppGang/dingo/pkg/parser"
    "github.com/MadAppGang/dingo/pkg/codegen"
)

// Transpile converts Dingo source to Go source.
// This is the ONLY entry point for transpilation.
func Transpile(src []byte) ([]byte, []SourceMapping, error) {
    // Step 1: Tokenize (ONLY place that reads raw bytes)
    tokens, err := tokenizer.Tokenize(src)
    if err != nil {
        return nil, nil, err
    }

    // Step 2: Parse tokens into AST (reads tokens, not bytes)
    ast, err := parser.Parse(tokens)
    if err != nil {
        return nil, nil, err
    }

    // Step 3: Generate Go code from AST (reads AST, not bytes or tokens)
    result := codegen.Generate(ast)

    return result.Code, result.Mappings, nil
}
```

### Phase 3: Create Correct Codegens

Create `pkg/codegen/` directory with proper codegens:

```go
// pkg/codegen/match.go
package codegen

import "github.com/MadAppGang/dingo/pkg/ast"

// MatchCodeGen generates Go code from MatchExpr AST nodes.
// IT DOES NOT READ SOURCE BYTES. IT ONLY READS AST NODES.
type MatchCodeGen struct {
    buf bytes.Buffer  // For output only
}

// Generate produces Go code for a match expression.
// Input: AST node (NOT bytes, NOT tokens)
// Output: Go source code
func (g *MatchCodeGen) Generate(expr *ast.MatchExpr) []byte {
    // Access expr.Scrutinee, expr.Arms, etc.
    // These are AST nodes, not strings

    g.buf.WriteString("switch __v := (")
    g.buf.WriteString(g.generateExpr(expr.Scrutinee))
    g.buf.WriteString(").(type) {\n")

    for _, arm := range expr.Arms {
        g.generateArm(arm)
    }

    g.buf.WriteString("}\n")
    return g.buf.Bytes()
}

// generateExpr generates Go for any expression AST node
func (g *MatchCodeGen) generateExpr(expr ast.Expr) string {
    switch e := expr.(type) {
    case *ast.Ident:
        return e.Name
    case *ast.CallExpr:
        // ... generate from AST, not from string parsing
    }
}
```

### Phase 4: Wire Parser to Codegen

The parser (`pkg/parser/`) already produces AST nodes.
The codegen (`pkg/codegen/`) consumes AST nodes.
Connect them in `pkg/transpiler/pipeline.go`.

### Phase 5: Verify No String Manipulation

```bash
# This command MUST return nothing
grep -rn "bytes\.Index\|strings\.Index\|regexp\." pkg/codegen/
grep -rn "func Find" pkg/codegen/
grep -rn "for.*range.*src\[" pkg/codegen/
```

---

## ANTI-PATTERNS (NEVER DO THESE)

### Anti-Pattern 1: Find Functions

```go
// ❌ NEVER write functions like this:
func FindMatchExpressions(src []byte) []int {
    // This is STRING MANIPULATION
}

// ✅ The parser already finds expressions during parsing
// There is NO NEED for Find functions
```

### Anti-Pattern 2: Transform*Source Functions

```go
// ❌ NEVER write functions like this:
func TransformMatchSource(src []byte) ([]byte, []SourceMapping) {
    positions := FindMatchExpressions(src)  // STRING MANIPULATION
    // ...
}

// ✅ The correct approach:
func Generate(ast *ast.File) CodeGenResult {
    // Walk AST, generate code
}
```

### Anti-Pattern 3: Byte Scanning

```go
// ❌ NEVER do this:
for i := 0; i < len(src); i++ {
    if src[i] == '?' {
        // ...
    }
}

// ✅ The tokenizer already handles this
// Tokens have Type field: tokenizer.QUESTION, tokenizer.MATCH, etc.
```

### Anti-Pattern 4: Regex for Parsing

```go
// ❌ NEVER do this:
pattern := regexp.MustCompile(`\|[\w\s,:]+\|`)
matches := pattern.FindAllStringIndex(string(src), -1)

// ✅ The tokenizer + parser handle this
// Lambda tokens: PIPE, IDENT, PIPE, ARROW, etc.
```

### Anti-Pattern 5: String Parsing

```go
// ❌ NEVER do this:
paramStr := strings.TrimSpace(src[:arrowIdx])
params := strings.Split(paramStr, ",")

// ✅ The parser returns structured data:
// expr.Params is []*ast.Param with Name, Type fields
```

---

## REQUIRED HEADER IN ALL CODEGEN FILES

Every file in `pkg/codegen/` MUST start with:

```go
// Package codegen generates Go code from Dingo AST nodes.
//
// ╔═══════════════════════════════════════════════════════════════════╗
// ║  THIS FILE MUST NOT CONTAIN STRING MANIPULATION                   ║
// ║                                                                   ║
// ║  FORBIDDEN:                                                       ║
// ║  • bytes.Index, strings.Index, bytes.Contains, strings.Contains  ║
// ║  • regexp.MustCompile, regexp.Match                              ║
// ║  • Character scanning loops (for i := 0; i < len(src); i++)      ║
// ║  • Any function taking []byte source and searching for patterns  ║
// ║                                                                   ║
// ║  REQUIRED:                                                        ║
// ║  • Input: AST nodes only (ast.MatchExpr, ast.LambdaExpr, etc.)   ║
// ║  • Output: Go source code via bytes.Buffer                       ║
// ║                                                                   ║
// ║  If you need to find syntax → THAT'S THE PARSER'S JOB            ║
// ║  If you need to parse tokens → THAT'S THE PARSER'S JOB           ║
// ║  Codegen ONLY transforms AST → Go code                           ║
// ╚═══════════════════════════════════════════════════════════════════╝
package codegen
```

---

## VERIFICATION BEFORE MERGE

Before any PR is merged, run:

```bash
#!/bin/bash
# verify_no_string_manipulation.sh

echo "Checking for forbidden patterns..."

# Check for string searching
if grep -rn "bytes\.Index\|bytes\.Contains\|strings\.Index\|strings\.Contains" pkg/codegen/ pkg/ast/*_codegen.go 2>/dev/null; then
    echo "❌ FAILED: Found bytes/strings searching"
    exit 1
fi

# Check for regex
if grep -rn "regexp\." pkg/codegen/ pkg/ast/*_codegen.go 2>/dev/null; then
    echo "❌ FAILED: Found regexp usage"
    exit 1
fi

# Check for Find functions
if grep -rn "func Find" pkg/codegen/ pkg/ast/*_codegen.go 2>/dev/null; then
    echo "❌ FAILED: Found Find functions (string manipulation)"
    exit 1
fi

# Check for Transform*Source functions
if grep -rn "func Transform.*Source" pkg/codegen/ 2>/dev/null; then
    echo "❌ FAILED: Found Transform*Source functions (wrong pattern)"
    exit 1
fi

echo "✅ PASSED: No string manipulation detected"
```

---

## SUMMARY

1. **DELETE** all `pkg/ast/*_codegen.go` files (except enum_codegen.go)
2. **CREATE** `pkg/codegen/` with proper AST-to-Go generators
3. **WIRE** `pkg/tokenizer/` → `pkg/parser/` → `pkg/codegen/`
4. **VERIFY** with grep commands that no string manipulation exists
5. **EMBED** the warning header in every codegen file

**The tokenizer reads bytes. The parser reads tokens. The codegen reads AST. NOTHING ELSE.**
