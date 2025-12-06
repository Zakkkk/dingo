# LSP Incremental Parser Integration

**Date**: 2025-12-05
**Status**: Complete
**Author**: golang-developer agent

## Summary

Integrated the incremental parser from `pkg/parser/` into the LSP server to enable fast, real-time diagnostics for open `.dingo` documents.

## Implementation

### 1. Created `pkg/lsp/incremental.go`

**IncrementalDocumentManager** - Manages incremental parsing state for all open documents:

```go
type IncrementalDocumentManager struct {
    documents map[string]*parser.DocumentState
    mu        sync.RWMutex
    fset      *token.FileSet
    logger    Logger
}
```

**Key Methods**:
- `OpenDocument(uri, content)` - Initialize incremental parser on didOpen
- `UpdateDocument(uri, changes)` - Apply incremental edits on didChange
- `CloseDocument(uri)` - Cleanup on didClose
- `GetDiagnostics(uri)` - Convert parser errors to LSP diagnostics
- `GetCompletionContext(uri, pos)` - Get completion context from incremental AST
- `GetHoverInfo(uri, pos)` - Get hover info from incremental AST

### 2. Updated `pkg/lsp/server.go`

**Server struct** - Added document manager field:
```go
type Server struct {
    // ... existing fields
    docManager *IncrementalDocumentManager
}
```

**Integration points**:

1. **handleDidOpen** - Initialize incremental parser and publish initial diagnostics
2. **handleDidChange** - Apply incremental changes and publish updated diagnostics
3. **handleDidClose** - Clean up incremental parser state

### 3. Protocol Conversion

**TextDocumentContentChangeEvent** conversion:
- LSP protocol uses `protocol.TextDocumentContentChangeEvent` with struct Range
- Parser uses `parser.TextDocumentContentChangeEvent` with pointer Range
- Full document sync detected by zero Range values

**Diagnostic conversion**:
- Parser `ParseError` → LSP `protocol.Diagnostic`
- Uses existing `Line` and `Column` fields from `ParseError`
- Converts severity levels appropriately

## Architecture

```
LSP Server
    ↓ didOpen
IncrementalDocumentManager.OpenDocument()
    ↓
parser.NewDocumentState() → parser.NewIncrementalParser()
    ↓
tokenizer.NewIncremental() → Initial parse
    ↓
Diagnostics published to IDE

LSP Server
    ↓ didChange
IncrementalDocumentManager.UpdateDocument()
    ↓
DocumentState.ApplyChange()
    ↓
IncrementalParser.ApplyEdit() → Incremental reparse
    ↓
Updated diagnostics published to IDE
```

## Benefits

1. **Real-time feedback** - Syntax errors shown immediately as user types
2. **Fast updates** - Incremental reparsing only affected regions
3. **Better IDE integration** - Preparation for completion/hover using incremental AST
4. **Graceful degradation** - Falls back to full reparse on complex edits

## Files Modified

- `pkg/lsp/incremental.go` (created) - 200 lines
- `pkg/lsp/server.go` (modified) - Added docManager field and integrated into handlers
- `pkg/parser/lsp.go` (modified) - Fixed diagnostic conversion to use Line/Column fields

## Testing Needed

1. Open `.dingo` file → Verify parser initialized
2. Type syntax error → Verify diagnostic appears immediately
3. Fix syntax error → Verify diagnostic clears
4. Large file edit → Verify incremental vs. full reparse behavior
5. Close file → Verify cleanup

## Next Steps

1. Use `GetCompletionContext()` in completion handler for context-aware suggestions
2. Use `GetHoverInfo()` in hover handler for AST-based information
3. Add performance metrics (parse time, reparse frequency)
4. Test with large files (>10K lines)
5. Add unit tests for document manager
