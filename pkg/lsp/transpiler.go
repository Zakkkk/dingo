package lsp

import (
	"context"
	"errors"
	"fmt"
	"go/scanner"
	"go/token"
	"os"
	"sync"
	"time"

	"github.com/MadAppGang/dingo/pkg/config"
	dingoerrors "github.com/MadAppGang/dingo/pkg/errors"
	"github.com/MadAppGang/dingo/pkg/transpiler"
	"github.com/MadAppGang/dingo/pkg/typeloader"
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

// ReinitializeWithConfig creates a new transpiler with the given config.
// This is called after the workspace is known to ensure output paths are correct.
func (at *AutoTranspiler) ReinitializeWithConfig(cfg *config.Config) {
	at.transpiler = transpiler.NewWithConfig(cfg)
	at.logger.Infof("Transpiler reinitialized with workspace config (outdir=%q, shadow=%v)",
		cfg.Build.OutDir, cfg.Build.Shadow)
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

// OnBatchFileChange handles multiple .dingo file changes efficiently.
// Pre-loads all imports into a shared type cache before transpiling.
// This is MUCH faster than OnFileChange for multiple files (~3s for 42 files vs ~1-2s per file).
func (at *AutoTranspiler) OnBatchFileChange(ctx context.Context, dingoPaths []string) {
	if len(dingoPaths) == 0 {
		return
	}

	// Single file: use regular method
	if len(dingoPaths) == 1 {
		at.OnFileChange(ctx, dingoPaths[0])
		return
	}

	at.logger.Infof("Batch auto-rebuild: %d files", len(dingoPaths))
	startTime := time.Now()

	// Step 1: Pre-load all imports into a shared type cache
	// This is fast (~150ms) and enables fast type lookup during transpilation
	typeCache := at.preloadTypesForFiles(dingoPaths)

	// Step 2: Transpile all files in parallel with shared type cache
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 4) // Limit parallelism to 4

	for _, dingoPath := range dingoPaths {
		wg.Add(1)
		go func(path string) {
			defer wg.Done()
			semaphore <- struct{}{}        // Acquire
			defer func() { <-semaphore }() // Release

			at.transpileWithCache(ctx, path, typeCache)
		}(dingoPath)
	}

	wg.Wait()
	at.logger.Infof("Batch auto-rebuild complete: %d files in %v", len(dingoPaths), time.Since(startTime))
}

// preloadTypesForFiles collects imports from all files and loads them into a type cache.
func (at *AutoTranspiler) preloadTypesForFiles(dingoPaths []string) *typeloader.BuildCache {
	collector := transpiler.ImportCollector{}
	importSet := make(map[string]bool)

	for _, path := range dingoPaths {
		src, err := os.ReadFile(path)
		if err != nil {
			at.logger.Warnf("Failed to read %s for import collection: %v", path, err)
			continue
		}

		imports, err := collector.CollectImports(src)
		if err != nil {
			at.logger.Warnf("Failed to collect imports from %s: %v", path, err)
			continue
		}

		for _, imp := range imports {
			importSet[imp] = true
		}
	}

	if len(importSet) == 0 {
		return nil
	}

	// Convert to slice
	allImports := make([]string, 0, len(importSet))
	for imp := range importSet {
		allImports = append(allImports, imp)
	}

	// Load all imports into cache
	cache := typeloader.NewBuildCache(typeloader.LoaderConfig{
		WorkingDir: at.server.workspacePath,
		FailFast:   false,
	})

	startTime := time.Now()
	_, err := cache.LoadImports(allImports)
	if err != nil {
		at.logger.Warnf("Failed to pre-load types: %v", err)
		return nil
	}
	at.logger.Debugf("Pre-loaded %d packages in %v", len(allImports), time.Since(startTime))

	return cache
}

// transpileWithCache transpiles a single file using a pre-loaded type cache.
func (at *AutoTranspiler) transpileWithCache(ctx context.Context, dingoPath string, typeCache *typeloader.BuildCache) {
	// Acquire file-specific lock
	fileLock := at.getFileLock(dingoPath)
	fileLock.Lock()
	defer fileLock.Unlock()

	uri := protocol.DocumentURI(lspuri.File(dingoPath))

	// Read source
	src, err := os.ReadFile(dingoPath)
	if err != nil {
		at.logger.Errorf("Failed to read %s: %v", dingoPath, err)
		return
	}

	// Transpile with type cache
	opts := transpiler.TranspileOptions{
		InferTypes: true,
		TypeCache:  typeCache,
	}

	result, err := transpiler.PureASTTranspileWithMappingsOpts(src, dingoPath, opts)
	if err != nil {
		at.logger.Errorf("Auto-transpile failed for %s: %v", dingoPath, err)
		// Publish diagnostic
		var formatter ErrorFormatter = &SimpleFormatter{}
		if at.server != nil {
			formatter = GetFormatterForEditor(at.server.editorType)
		}
		diagnostic := ParseTranspileError(dingoPath, err, formatter)
		if diagnostic != nil && at.server != nil {
			at.server.updateAndPublishDiagnostics(uri, "gopls", []protocol.Diagnostic{})
			at.server.updateAndPublishDiagnostics(uri, "transpile", []protocol.Diagnostic{*diagnostic})
		}
		return
	}

	// Write output using server's path calculation
	goPath := at.server.dingoToGoPath(dingoPath)
	if err := os.WriteFile(goPath, result.GoCode, 0644); err != nil {
		at.logger.Errorf("Failed to write %s: %v", goPath, err)
		return
	}

	at.logger.Debugf("Transpiled: %s", dingoPath)

	// Clear transpile diagnostics
	if at.server != nil {
		at.server.updateAndPublishDiagnostics(uri, "transpile", []protocol.Diagnostic{})
	}

	// Invalidate cache and sync gopls
	at.mapCache.Invalidate(goPath)
	if err := at.SyncGoplsWithGoFile(ctx, goPath); err != nil {
		at.logger.Warnf("Failed to sync gopls with .go file: %v", err)
	}
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
			// Clear gopls diagnostics when transpilation fails - the .go file is stale
			// and gopls errors would point to wrong lines
			at.server.updateAndPublishDiagnostics(uri, "gopls", []protocol.Diagnostic{})
			at.server.updateAndPublishDiagnostics(uri, "transpile", []protocol.Diagnostic{*diagnostic})
		}
		return // Don't proceed to gopls sync
	}

	// Clear transpile diagnostics on successful transpilation
	if at.server != nil {
		at.server.updateAndPublishDiagnostics(uri, "transpile", []protocol.Diagnostic{})
	}

	// Invalidate source map cache using config-aware path calculation
	goPath := at.server.dingoToGoPath(dingoPath)
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

	// Try to unwrap to EnhancedError (from validator)
	var enhancedErr *dingoerrors.EnhancedError
	if errors.As(err, &enhancedErr) {
		// EnhancedError has Line and Column fields (1-indexed)
		line := enhancedErr.Line
		col := enhancedErr.Column

		// Get the range for highlighting
		startChar := 0
		endChar := 1
		if line > 0 {
			_, endChar = getLineRange(dingoPath, line)
			if col > 0 {
				startChar = col - 1 // Convert to 0-indexed
			}
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
			Message:  enhancedErr.Message,
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
