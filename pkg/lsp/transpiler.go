package lsp

import (
	"context"
	"errors"
	"fmt"
	"go/scanner"
	"go/token"
	"os"
	"sync"

	"github.com/MadAppGang/dingo/pkg/transpiler"
	"go.lsp.dev/protocol"
	lspuri "go.lsp.dev/uri"
)

// AutoTranspiler handles automatic transpilation of .dingo files
type AutoTranspiler struct {
	logger     Logger
	mapCache   *SourceMapCache
	gopls      *GoplsClient
	transpiler *transpiler.Transpiler
	server     *Server // For publishing Dingo-specific diagnostics

	// File-level locking to prevent concurrent transpilations of the same file
	// This prevents race conditions when both didSave and file watcher trigger transpilation
	fileLocksMu sync.Mutex
	fileLocks   map[string]*sync.Mutex
}

// NewAutoTranspiler creates an auto-transpiler instance
func NewAutoTranspiler(logger Logger, mapCache *SourceMapCache, gopls *GoplsClient, server *Server) *AutoTranspiler {
	// Create integrated transpiler
	t, err := transpiler.New()
	if err != nil {
		// Fall back to nil transpiler - will fail at transpile time
		logger.Warnf("Failed to create transpiler: %v", err)
	}

	return &AutoTranspiler{
		logger:     logger,
		mapCache:   mapCache,
		gopls:      gopls,
		transpiler: t,
		server:     server,
		fileLocks:  make(map[string]*sync.Mutex),
	}
}

// TranspileFile transpiles a single .dingo file
func (at *AutoTranspiler) TranspileFile(ctx context.Context, dingoPath string) error {
	at.logger.Infof("Auto-rebuild: %s", dingoPath)

	if at.transpiler == nil {
		return fmt.Errorf("transpiler not initialized")
	}

	// Use integrated transpiler library (no shell out!)
	err := at.transpiler.TranspileFile(dingoPath)
	if err != nil {
		return fmt.Errorf("transpilation failed: %w", err)
	}

	at.logger.Infof("Auto-rebuild complete: %s", dingoPath)
	return nil
}

// getFileLock returns the mutex for a specific file, creating one if needed
func (at *AutoTranspiler) getFileLock(path string) *sync.Mutex {
	at.fileLocksMu.Lock()
	defer at.fileLocksMu.Unlock()

	if lock, ok := at.fileLocks[path]; ok {
		return lock
	}

	lock := &sync.Mutex{}
	at.fileLocks[path] = lock
	return lock
}

// OnFileChange handles a .dingo file change (called by watcher and didSave)
// Uses per-file locking to prevent concurrent transpilations of the same file,
// which can cause race conditions with gopls version tracking.
func (at *AutoTranspiler) OnFileChange(ctx context.Context, dingoPath string) {
	// Acquire file-specific lock to prevent concurrent transpilations
	fileLock := at.getFileLock(dingoPath)
	fileLock.Lock()
	defer fileLock.Unlock()

	uri := protocol.DocumentURI(lspuri.File(dingoPath))

	// Transpile the file
	if err := at.TranspileFile(ctx, dingoPath); err != nil {
		at.logger.Errorf("Auto-transpile failed for %s: %v", dingoPath, err)

		// Publish Dingo-specific diagnostic for transpilation error
		// Use editor-appropriate formatter for error messages
		var formatter ErrorFormatter = &SimpleFormatter{}
		if at.server != nil {
			formatter = GetFormatterForEditor(at.server.editorType)
		}
		diagnostic := ParseTranspileError(dingoPath, err, formatter)
		if diagnostic != nil && at.server != nil {
			at.server.updateAndPublishDiagnostics(uri, "transpile", []protocol.Diagnostic{*diagnostic})
		}
		return // Don't proceed to gopls sync
	}

	// Clear transpile diagnostics on successful transpilation
	if at.server != nil {
		at.server.updateAndPublishDiagnostics(uri, "transpile", []protocol.Diagnostic{})
	}

	// Invalidate source map cache
	goPath := dingoToGoPath(dingoPath)
	at.mapCache.Invalidate(goPath)
	at.logger.Debugf("Source map cache invalidated: %s", goPath)

	// CRITICAL FIX: Synchronize gopls with new .go file content
	// This ensures gopls has the latest transpiled content in memory
	if err := at.SyncGoplsWithGoFile(ctx, goPath); err != nil {
		at.logger.Warnf("Failed to sync gopls with .go file: %v", err)
	}
}

// SyncGoplsWithGoFile sends the new .go file content to gopls via didChange
// This ensures gopls has the latest transpiled content in memory
func (at *AutoTranspiler) SyncGoplsWithGoFile(ctx context.Context, goPath string) error {
	at.logger.Debugf("Synchronizing gopls with updated .go file: %s", goPath)
	return at.gopls.SyncFileContent(ctx, goPath)
}

// ParseTranspileError converts a transpiler error into an LSP diagnostic.
// Handles multiple error types:
// - *transpiler.TranspileError: Structured errors from Dingo transpiler
// - scanner.ErrorList: Errors from Go parser (wrapped by pure_pipeline.go)
// - Generic errors: Fall back to line 0
//
// The formatter parameter formats error messages for the specific editor.
// Use GetFormatterForEditor() to get the appropriate formatter.
func ParseTranspileError(dingoPath string, err error, formatter ErrorFormatter) *protocol.Diagnostic {
	if err == nil {
		return nil
	}

	// Try to unwrap to structured TranspileError
	var transpileErr *transpiler.TranspileError
	if errors.As(err, &transpileErr) {
		// Structured error with position information
		startChar, endChar := 0, 1
		if transpileErr.Line > 0 {
			startChar, endChar = getLineRange(dingoPath, transpileErr.Line)
		}

		// Use formatter to create editor-appropriate message
		message := transpileErr.Message
		if formatter != nil {
			message = formatter.Format(transpileErr)
		}

		return &protocol.Diagnostic{
			Range: protocol.Range{
				Start: protocol.Position{
					Line:      uint32(max(0, transpileErr.Line-1)), // 0-based
					Character: uint32(startChar),
				},
				End: protocol.Position{
					Line:      uint32(max(0, transpileErr.Line-1)),
					Character: uint32(endChar),
				},
			},
			Severity: protocol.DiagnosticSeverityError,
			Source:   "dingo",
			Message:  message,
		}
	}

	// Try to unwrap to scanner.ErrorList (from Go parser)
	// This handles errors wrapped by fmt.Errorf in pure_pipeline.go
	var scannerErrors scanner.ErrorList
	if errors.As(err, &scannerErrors) && len(scannerErrors) > 0 {
		// Take the first error (most relevant)
		firstErr := scannerErrors[0]
		line := firstErr.Pos.Line
		col := firstErr.Pos.Column

		// Get the range for highlighting the error location
		startChar, endChar := 0, 1
		if line > 0 {
			_, endChar = getLineRange(dingoPath, line)
		}

		// Use the column from the error if available (1-based to 0-based)
		if col > 0 {
			startChar = col - 1
		}

		return &protocol.Diagnostic{
			Range: protocol.Range{
				Start: protocol.Position{
					Line:      uint32(max(0, line-1)), // 0-based
					Character: uint32(startChar),
				},
				End: protocol.Position{
					Line:      uint32(max(0, line-1)),
					Character: uint32(endChar),
				},
			},
			Severity: protocol.DiagnosticSeverityError,
			Source:   "dingo",
			Message:  firstErr.Msg,
		}
	}

	// Fallback: generic error at top of file
	return &protocol.Diagnostic{
		Range: protocol.Range{
			Start: protocol.Position{Line: 0, Character: 0},
			End:   protocol.Position{Line: 0, Character: 0},
		},
		Severity: protocol.DiagnosticSeverityError,
		Source:   "dingo",
		Message:  err.Error(),
	}
}

// getLineRange returns the start and end character positions for a line.
// Uses token.FileSet for line extraction per CLAUDE.md guidelines.
// Returns (startChar, endChar) where startChar is the first non-whitespace character
// and endChar is the end of the line (for highlighting the meaningful content).
func getLineRange(filePath string, lineNum int) (int, int) {
	src, err := os.ReadFile(filePath)
	if err != nil {
		return 0, 1 // Fallback to first character
	}

	// Use token.FileSet to get line boundaries
	fset := token.NewFileSet()
	file := fset.AddFile(filePath, fset.Base(), len(src))
	file.SetLinesForContent(src)

	if lineNum < 1 || lineNum > file.LineCount() {
		return 0, 1
	}

	// Get line content using FileSet
	lineStart := file.Offset(file.LineStart(lineNum))
	lineEnd := len(src)
	if lineNum < file.LineCount() {
		lineEnd = file.Offset(file.LineStart(lineNum + 1))
	}

	lineContent := src[lineStart:lineEnd]

	// Remove trailing newline from length
	lineLen := len(lineContent)
	if lineLen > 0 && lineContent[lineLen-1] == '\n' {
		lineLen--
	}

	// Find first non-whitespace character
	startChar := 0
	for i := 0; i < lineLen; i++ {
		if lineContent[i] != ' ' && lineContent[i] != '\t' {
			startChar = i
			break
		}
	}

	return startChar, lineLen
}
