# AST-Based Transpilation Pipeline

**Status**: Under Development (Phase: Parser Implementation - Task W Complete)
**File**: `pkg/transpiler/ast_pipeline.go`
**Tests**: `pkg/transpiler/ast_pipeline_test.go`

## Overview

The AST-based transpilation pipeline is the **future architecture** for Dingo transpilation, replacing the current regex-based preprocessor approach with a proper AST-based transformation system.

## Pipeline Stages

```
┌────────────────────────────────────────────────────────────┐
│ Stage 1: TOKENIZATION (pkg/tokenizer)                     │
├────────────────────────────────────────────────────────────┤
│ Input:  .dingo source code ([]byte)                       │
│ Output: Token stream ([]tokenizer.Token)                  │
│ Tech:   Custom scanner with Dingo operators               │
│ Status: ✅ Complete (Phase: Tokenizer)                    │
└────────────────────────────────────────────────────────────┘
                          ↓
┌────────────────────────────────────────────────────────────┐
│ Stage 2: PARSING (pkg/parser/pratt.go)                    │
├────────────────────────────────────────────────────────────┤
│ Input:  Token stream                                       │
│ Output: Dingo AST (pkg/ast)                               │
│ Tech:   Pratt parser (operator precedence)                │
│ Status: 🚧 In Progress (Expressions done, file parsing TODO)│
└────────────────────────────────────────────────────────────┘
                          ↓
┌────────────────────────────────────────────────────────────┐
│ Stage 3: TRANSFORMATION (pkg/transformer)                 │
├────────────────────────────────────────────────────────────┤
│ Input:  Dingo AST (ast.File)                              │
│ Output: Go AST (go/ast.File)                              │
│ Tech:   Node transformer registry, visitor pattern        │
│ Status: 🚧 Partial (Framework done, transformers TODO)    │
└────────────────────────────────────────────────────────────┘
                          ↓
┌────────────────────────────────────────────────────────────┐
│ Stage 4: CODE GENERATION (go/printer)                     │
├────────────────────────────────────────────────────────────┤
│ Input:  Go AST                                             │
│ Output: .go source code ([]byte)                          │
│ Tech:   Standard go/printer                               │
│ Status: ✅ Complete (Standard library)                    │
└────────────────────────────────────────────────────────────┘
```

## API

### Main Entry Point

```go
func ASTTranspile(source []byte, filename string, fset *token.FileSet) (*TranspileResult, error)
```

**Parameters**:
- `source` - Dingo source code
- `filename` - Original filename (for error messages and source maps)
- `fset` - Go token.FileSet for position tracking

**Returns**:
- `TranspileResult` - Contains:
  - `GoCode` - Generated Go source
  - `Errors` - All parse/transform errors
  - `GoAST` - Final Go AST (for further processing)
  - `Metadata` - Transformation metadata (token count, transform count)

### LSP/Incremental Mode

```go
func ASTTranspileIncremental(source []byte, filename string, fset *token.FileSet) *TranspileResult
```

**Use case**: LSP server needs partial results even when errors occur.

**Behavior**: Always returns `TranspileResult`, errors stored in `result.Errors`.

## Current Status

### ✅ Completed

1. **Pipeline framework** (`ast_pipeline.go`)
   - 4-stage pipeline architecture
   - Error aggregation across stages
   - Metadata collection
   - Incremental mode support

2. **Test suite** (`ast_pipeline_test.go`)
   - Basic transpilation tests
   - Metadata validation
   - Pipeline stage verification
   - Incremental mode tests

3. **Integration points**
   - Tokenizer integration (Stage 1)
   - Transformer integration (Stage 3)
   - go/printer integration (Stage 4)

### 🚧 In Progress

1. **Parser integration** (Stage 2)
   - ✅ Expression parsing (Pratt parser)
   - ⏳ File parsing (`parser/decl.go` - TODO)
   - ⏳ Declaration parsing

2. **Transformer completeness**
   - ✅ Framework (visitor pattern, node registry)
   - ⏳ Individual transformers (error prop, lambdas, match, etc.)

### ⏳ TODO

1. **Full file parsing**
   - Implement `parser/decl.go` for top-level declarations
   - Parse package clause, imports, functions, types
   - Build complete Dingo AST file structure

2. **Complete transformers**
   - Error propagation: `expr?` → `if err != nil { return ... }`
   - Lambdas: `(x) => x + 1` → `func(x int) int { return x + 1 }`
   - Match expressions: `match x { ... }` → `switch x.tag { ... }`
   - Null coalescing: `a ?? b` → IIFE pattern
   - Safe navigation: `x?.field` → IIFE pattern

3. **Type checking integration**
   - Run go/types on parsed AST
   - Pass type info to transformer
   - Enable type-aware transformations

4. **Source map generation**
   - Track position mappings during transformation
   - Generate accurate source maps
   - Integrate with LSP

## Design Principles

### 1. Separation of Concerns

Each stage has a single, well-defined responsibility:
- Tokenizer: Lexical analysis only
- Parser: Syntax analysis only
- Transformer: Semantic transformation only
- Printer: Code generation only

### 2. Incremental Compilation

- Stages can be run independently (for testing)
- Partial results support (for LSP)
- Error recovery at each stage
- Reusable across tools (CLI, LSP, build system)

### 3. AST-Based Transformation

**No regex** after tokenization:
- All transformations operate on AST nodes
- Precise position tracking
- Compositional transformations
- Safe refactoring

### 4. Zero Runtime Overhead

Generated code contains **only standard Go**:
- No runtime library dependencies
- No wrapper types (compile-time only)
- Inline transformations where possible
- IIFE pattern for complex expressions

## Migration Path

### Current: Preprocessor-Based

```
.dingo → Preprocessor (regex) → go/parser → Plugins → .go
```

**Issues**:
- Fragile regex patterns
- Position drift in source maps
- Hard to extend
- Complex error recovery

### Future: AST-Based

```
.dingo → Tokenizer → Parser → Transformer → .go
```

**Benefits**:
- Precise AST transformations
- Accurate position tracking
- Easy to extend (add transformer)
- Robust error recovery

### Transition Strategy

1. **Phase 1**: Implement AST pipeline (✅ Task W Complete)
2. **Phase 2**: Complete file parsing
3. **Phase 3**: Implement all transformers
4. **Phase 4**: Validate equivalence with preprocessor
5. **Phase 5**: Switch default to AST pipeline
6. **Phase 6**: Remove preprocessor (deprecate)

## Testing Strategy

### Unit Tests

Each stage tested independently:
- Tokenizer: Token stream correctness
- Parser: AST structure correctness
- Transformer: Go AST correctness
- Pipeline: End-to-end correctness

### Integration Tests

- Golden tests: `.dingo` → `.go` comparison
- LSP tests: Incremental transpilation
- Error tests: Recovery and reporting

### Regression Tests

- All preprocessor golden tests must pass
- Ensure no behavior changes during migration

## Usage Examples

### Basic Transpilation

```go
source := []byte(`
    func readConfig() Result<Config, error> {
        data := readFile("config.json")?
        Ok(parseConfig(data))
    }
`)

fset := token.NewFileSet()
result, err := ASTTranspile(source, "config.dingo", fset)

if err != nil {
    // Handle fatal errors
    log.Fatal(err)
}

// Write generated code
os.WriteFile("config.go", result.GoCode, 0644)
```

### LSP Usage

```go
// LSP needs partial results even with errors
result := ASTTranspileIncremental(changedSource, filename, fset)

if result.HasErrors() {
    // Send diagnostics to editor
    for _, err := range result.Errors {
        sendDiagnostic(err)
    }
}

// Use partial AST for autocomplete/hover even if errors exist
if result.GoAST != nil {
    ast.Inspect(result.GoAST, func(n ast.Node) bool {
        // LSP operations...
    })
}
```

## Performance Considerations

### Memory

- Tokenizer: O(n) - one pass over source
- Parser: O(n) - builds AST once
- Transformer: O(n) - single traversal
- Printer: O(n) - one pass

**Total**: O(n) memory overhead

### Time

- Tokenizer: O(n) - linear scan
- Parser: O(n) - Pratt parsing is linear
- Transformer: O(n) - AST visitor
- Printer: O(n) - linear output

**Total**: O(n) time complexity

### Comparison to Preprocessor

AST pipeline should be **faster** because:
- No repeated regex matching
- Single-pass transformation
- No position tracking overhead
- Better cache locality (AST traversal)

## Next Steps (Task X)

1. Implement full file parsing in `parser/decl.go`
2. Parse package clause, imports, declarations
3. Update `ast_pipeline.go` to use full file parser
4. Add comprehensive golden tests

## References

- Tokenizer: `pkg/tokenizer/tokenizer.go`
- Parser: `pkg/parser/pratt.go`
- Transformer: `pkg/transformer/transformer.go`
- Dingo AST: `pkg/ast/`
- Tests: `pkg/transpiler/ast_pipeline_test.go`
