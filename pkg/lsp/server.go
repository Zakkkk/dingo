package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/MadAppGang/dingo/pkg/lsp/semantic"
	"github.com/MadAppGang/dingo/pkg/sourcemap/dmap"
	"github.com/MadAppGang/dingo/pkg/transpiler"
	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	lspuri "go.lsp.dev/uri"
)

// ServerConfig holds configuration for the LSP server
type ServerConfig struct {
	Logger        Logger
	GoplsPath     string
	AutoTranspile bool
}

// Server implements the LSP proxy server
type Server struct {
	config        ServerConfig
	gopls         *GoplsClient
	mapCache      *SourceMapCache
	translator    *Translator
	transpiler    *AutoTranspiler
	watcher       *FileWatcher
	docManager    *IncrementalDocumentManager // Incremental parser manager
	workspacePath string
	initialized   bool

	// CRITICAL FIX (Qwen): Protect connection and context with mutex
	connMu  sync.RWMutex
	ideConn jsonrpc2.Conn   // Store IDE connection for diagnostics
	ctx     context.Context // Store server context

	// Diagnostic cache - stores diagnostics by source to allow merging
	diagMu         sync.RWMutex
	lintDiags      map[string][]protocol.Diagnostic // URI -> lint diagnostics
	goplsDiags     map[string][]protocol.Diagnostic // URI -> gopls diagnostics
	transpileDiags map[string][]protocol.Diagnostic // URI -> transpiler diagnostics
	parseDiags     map[string][]protocol.Diagnostic // URI -> incremental parser diagnostics

	// Semantic manager for native hover (Phase 1)
	semanticManager *semantic.Manager

	// Debounced transpilation for responsive diagnostics
	transpileDebounce    map[string]*transpileDebounceState
	transpileDebounceMu  sync.Mutex
	transpileDebounceMs  int // Debounce delay in milliseconds (default: 300)
}

// transpileDebounceState tracks debounced transpilation for a single file
type transpileDebounceState struct {
	timer   *time.Timer
	content string // Latest buffer content
}

// NewServer creates a new LSP server instance
func NewServer(cfg ServerConfig) (*Server, error) {
	// Initialize gopls client
	gopls, err := NewGoplsClient(cfg.GoplsPath, cfg.Logger)
	if err != nil {
		return nil, fmt.Errorf("failed to start gopls: %w", err)
	}

	// Initialize source map cache
	mapCache, err := NewSourceMapCache(cfg.Logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create source map cache: %w", err)
	}

	// Initialize translator
	translator := NewTranslator(mapCache)

	// Initialize incremental document manager
	docManager := NewIncrementalDocumentManager(cfg.Logger)

	// Create server first (without transpiler)
	server := &Server{
		config:              cfg,
		gopls:               gopls,
		mapCache:            mapCache,
		translator:          translator,
		docManager:          docManager,
		lintDiags:           make(map[string][]protocol.Diagnostic),
		goplsDiags:          make(map[string][]protocol.Diagnostic),
		transpileDiags:      make(map[string][]protocol.Diagnostic),
		parseDiags:          make(map[string][]protocol.Diagnostic),
		transpileDebounce:   make(map[string]*transpileDebounceState),
		transpileDebounceMs: 300, // 300ms debounce for responsive typing
	}

	// Initialize auto-transpiler with server reference
	transpiler := NewAutoTranspiler(cfg.Logger, mapCache, gopls, server)
	server.transpiler = transpiler

	// Initialize semantic manager for native hover
	// Create transpile wrapper function
	transpileFunc := server.createTranspileFunc()
	semanticManager := semantic.NewManager(cfg.Logger, transpileFunc)
	server.semanticManager = semanticManager

	// Set diagnostics handler for gopls -> IDE diagnostics forwarding
	gopls.SetDiagnosticsHandler(server.handlePublishDiagnostics)

	return server, nil
}

// createTranspileFunc creates a transpile function for the semantic manager
// This wraps the transpiler to match the semantic.TranspileFunc signature
func (s *Server) createTranspileFunc() semantic.TranspileFunc {
	return func(source []byte, filename string) (semantic.TranspileResult, error) {
		// Strip file:// URI prefix for //line directives in generated Go code.
		// Go's //line directive requires a filesystem path, not a URI.
		fsPath := filename
		if strings.HasPrefix(filename, "file://") {
			fsPath = strings.TrimPrefix(filename, "file://")
		}

		// Use pure pipeline directly - it handles source-based transpilation
		result, err := transpiler.PureASTTranspileWithMappings(source, fsPath, true)
		if err != nil {
			return semantic.TranspileResult{}, err
		}

		// Convert to semantic.TranspileResult format
		return semantic.TranspileResult{
			GoCode:         result.GoCode,
			LineMappings:   result.LineMappings,
			ColumnMappings: result.ColumnMappings, // For accurate hover column translation
			DingoFset:      nil,                   // Not available from this pipeline
			DingoFile:      filename,
		}, nil
	}
}

// SetConn stores the connection and context in the server (thread-safe)
func (s *Server) SetConn(conn jsonrpc2.Conn, ctx context.Context) {
	s.connMu.Lock()
	defer s.connMu.Unlock()
	s.ideConn = conn
	s.ctx = ctx
}

// GetConn returns the IDE connection (thread-safe)
func (s *Server) GetConn() (jsonrpc2.Conn, context.Context) {
	s.connMu.RLock()
	defer s.connMu.RUnlock()
	return s.ideConn, s.ctx
}

// Handler returns a jsonrpc2 handler for this server
func (s *Server) Handler() jsonrpc2.Handler {
	return jsonrpc2.ReplyHandler(s.handleRequest)
}

// handleRequest routes LSP requests to appropriate handlers
func (s *Server) handleRequest(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	s.config.Logger.Debugf("Received request: %s", req.Method())

	switch req.Method() {
	case "initialize":
		return s.handleInitialize(ctx, reply, req)
	case "initialized":
		return s.handleInitialized(ctx, reply, req)
	case "shutdown":
		return s.handleShutdown(ctx, reply, req)
	case "exit":
		return s.handleExit(ctx, reply, req)
	case "textDocument/didOpen":
		return s.handleDidOpen(ctx, reply, req)
	case "textDocument/didChange":
		return s.handleDidChange(ctx, reply, req)
	case "textDocument/didSave":
		return s.handleDidSave(ctx, reply, req)
	case "textDocument/didClose":
		return s.handleDidClose(ctx, reply, req)
	case "textDocument/completion":
		return s.handleCompletion(ctx, reply, req)
	case "textDocument/definition":
		return s.handleDefinition(ctx, reply, req)
	case "textDocument/hover":
		return s.handleHover(ctx, reply, req)
	case "textDocument/codeAction":
		return s.handleCodeAction(ctx, reply, req)
	case "textDocument/formatting":
		return s.handleFormatting(ctx, reply, req)
	default:
		// Unknown method - try forwarding to gopls
		s.config.Logger.Debugf("Forwarding unknown method to gopls: %s", req.Method())
		return s.forwardToGopls(ctx, reply, req)
	}
}

// handleInitialize processes the initialize request
func (s *Server) handleInitialize(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	s.config.Logger.Debugf("handleInitialize: Starting")

	var params protocol.InitializeParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		s.config.Logger.Errorf("handleInitialize: Failed to unmarshal params: %v", err)
		return reply(ctx, nil, fmt.Errorf("invalid initialize params: %w", err))
	}
	s.config.Logger.Debugf("handleInitialize: Params unmarshaled")

	// Extract workspace path
	if params.RootURI != "" {
		s.workspacePath = params.RootURI.Filename()
		s.config.Logger.Infof("Workspace path: %s", s.workspacePath)

		// Start file watcher if auto-transpile enabled
		if s.config.AutoTranspile {
			watcher, err := NewFileWatcher(s.workspacePath, s.config.Logger, s.handleDingoFileChange)
			if err != nil {
				s.config.Logger.Warnf("Failed to start file watcher: %v (auto-transpile disabled)", err)
			} else {
				s.watcher = watcher
			}
		}
	}

	// Forward initialize to gopls
	s.config.Logger.Debugf("handleInitialize: Forwarding to gopls")
	goplsResult, err := s.gopls.Initialize(ctx, params)
	if err != nil {
		s.config.Logger.Errorf("handleInitialize: gopls failed: %v", err)
		return reply(ctx, nil, fmt.Errorf("gopls initialize failed: %w", err))
	}
	s.config.Logger.Debugf("handleInitialize: gopls responded")

	// Return modified capabilities (Dingo-specific)
	result := protocol.InitializeResult{
		Capabilities: protocol.ServerCapabilities{
			TextDocumentSync: protocol.TextDocumentSyncOptions{
				OpenClose: true,
				Change:    protocol.TextDocumentSyncKindFull,
				Save: &protocol.SaveOptions{
					IncludeText: false,
				},
			},
			CompletionProvider: &protocol.CompletionOptions{
				TriggerCharacters: []string{".", ":", " "},
			},
			HoverProvider:              goplsResult.Capabilities.HoverProvider,
			DefinitionProvider:         goplsResult.Capabilities.DefinitionProvider,
			DocumentFormattingProvider: true,
			CodeActionProvider: &protocol.CodeActionOptions{
				CodeActionKinds: []protocol.CodeActionKind{
					protocol.QuickFix,
					protocol.Refactor,
					protocol.RefactorRewrite,
				},
			},
		},
		ServerInfo: &protocol.ServerInfo{
			Name:    "dingo-lsp",
			Version: "0.1.0",
		},
	}

	s.initialized = true
	s.config.Logger.Debugf("Sending initialize response to client")
	s.config.Logger.Infof("Server initialized (auto-transpile: %v)", s.config.AutoTranspile)

	return reply(ctx, result, nil)
}

// handleInitialized processes the initialized notification
func (s *Server) handleInitialized(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.InitializedParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, fmt.Errorf("invalid initialized params: %w", err))
	}

	// Forward to gopls
	if err := s.gopls.Initialized(ctx, &params); err != nil {
		s.config.Logger.Warnf("gopls initialized notification failed: %v", err)
	}

	return reply(ctx, nil, nil)
}

// handleShutdown processes the shutdown request
func (s *Server) handleShutdown(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	s.config.Logger.Infof("Shutdown requested")

	// Stop file watcher
	if s.watcher != nil {
		if err := s.watcher.Close(); err != nil {
			s.config.Logger.Warnf("File watcher close failed: %v", err)
		}
	}

	// Shutdown gopls
	if err := s.gopls.Shutdown(ctx); err != nil {
		s.config.Logger.Warnf("gopls shutdown failed: %v", err)
	}

	s.initialized = false
	return reply(ctx, nil, nil)
}

// handleExit processes the exit notification
func (s *Server) handleExit(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	s.config.Logger.Infof("Exit requested")
	return reply(ctx, nil, nil)
}

// handleDidOpen processes didOpen notifications
func (s *Server) handleDidOpen(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.DidOpenTextDocumentParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}

	// CRITICAL FIX D1: When .dingo file opens, ensure .go file exists and open with gopls
	// This is necessary for gopls to analyze the file and send diagnostics
	if isDingoFile(params.TextDocument.URI) {
		dingoPath := params.TextDocument.URI.Filename()
		s.config.Logger.Infof("[didOpen] Opened .dingo file: %s", dingoPath)

		// Run linter on open (provides immediate feedback)
		go s.runLintOnOpen(ctx, params.TextDocument.URI)

		// Initialize incremental parser for this document
		if err := s.docManager.OpenDocument(string(params.TextDocument.URI), params.TextDocument.Text); err != nil {
			s.config.Logger.Warnf("[didOpen] Failed to initialize incremental parser: %v", err)
		} else {
			s.config.Logger.Debugf("[didOpen] Incremental parser initialized")

			// Publish initial diagnostics from parser
			diagnostics := s.docManager.GetDiagnostics(string(params.TextDocument.URI))
			s.updateAndPublishDiagnostics(params.TextDocument.URI, "parse", diagnostics)
		}

		// Check if .go file exists, if not auto-transpile
		goPath := dingoToGoPath(dingoPath)
		if _, err := os.Stat(goPath); os.IsNotExist(err) {
			s.config.Logger.Infof("[didOpen] .go file missing, auto-transpiling: %s", dingoPath)

			// Auto-transpile to generate .go file
			if s.transpiler != nil {
				s.transpiler.OnFileChange(ctx, dingoPath)
				s.config.Logger.Infof("[didOpen] Auto-transpile completed (check for errors above)")
			} else {
				s.config.Logger.Warnf("[didOpen] Transpiler not available!")
			}
		} else {
			s.config.Logger.Infof("[didOpen] .go file already exists: %s", goPath)
		}

		// Open corresponding .go file with gopls (if transpilation succeeded)
		if err := s.openGoFileWithGopls(ctx, dingoPath); err != nil {
			s.config.Logger.Warnf("[didOpen] Failed to open .go file with gopls: %v", err)
		} else {
			s.config.Logger.Infof("[didOpen] Successfully opened .go file with gopls")
		}

		return reply(ctx, nil, nil)
	}

	// Forward non-dingo files to gopls
	if err := s.gopls.DidOpen(ctx, params); err != nil {
		s.config.Logger.Warnf("gopls didOpen failed: %v", err)
	}

	return reply(ctx, nil, nil)
}

// handleDidChange processes didChange notifications
func (s *Server) handleDidChange(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.DidChangeTextDocumentParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}

	// IMPORTANT: Don't forward .dingo file changes to gopls
	// gopls reads .go files from disk (updated by auto-transpiler on save)
	// We translate positions during queries instead
	if isDingoFile(params.TextDocument.URI) {
		s.config.Logger.Debugf("Changed .dingo file (not forwarding to gopls): %s", params.TextDocument.URI)

		// Update incremental parser
		if err := s.docManager.UpdateDocument(string(params.TextDocument.URI), params.ContentChanges); err != nil {
			s.config.Logger.Warnf("[didChange] Incremental parse failed: %v", err)
		} else {
			s.config.Logger.Debugf("[didChange] Incremental parse succeeded")

			// Get parse diagnostics
			diagnostics := s.docManager.GetDiagnostics(string(params.TextDocument.URI))

			// Publish parse diagnostics immediately (these are fast)
			s.updateAndPublishDiagnostics(params.TextDocument.URI, "parse", diagnostics)

			// Schedule debounced transpilation for responsive gopls diagnostics.
			// This provides type errors, unused variable warnings, etc. while typing
			// with a 300ms delay after the user stops typing.
			if s.config.AutoTranspile {
				content := s.docManager.GetContent(string(params.TextDocument.URI))
				if content != "" {
					s.scheduleTranspileFromBuffer(params.TextDocument.URI, content)
				}
			}
		}

		return reply(ctx, nil, nil)
	}

	// Forward non-dingo files to gopls
	if err := s.gopls.DidChange(ctx, params); err != nil {
		s.config.Logger.Warnf("gopls didChange failed: %v", err)
	}

	return reply(ctx, nil, nil)
}

// handleDidSave processes didSave notifications
func (s *Server) handleDidSave(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.DidSaveTextDocumentParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}

	// Handle .dingo file save
	if isDingoFile(params.TextDocument.URI) {
		dingoPath := params.TextDocument.URI.Filename()

		// Run linter on save (always, regardless of auto-transpile setting)
		go s.runLintOnSave(ctx, params.TextDocument.URI)

		// Auto-transpile if enabled
		// Note: The file watcher also triggers transpilation with 500ms debounce.
		// We trigger here too for immediate feedback, but the transpiler has internal
		// locking to prevent concurrent transpilations of the same file.
		if s.config.AutoTranspile {
			s.config.Logger.Debugf("Auto-transpile on save: %s", dingoPath)
			go s.transpiler.OnFileChange(ctx, dingoPath)
		}

		// Don't forward to gopls - transpiler handles it after successful transpilation
		return reply(ctx, nil, nil)
	}

	// Forward non-dingo files to gopls
	if err := s.gopls.DidSave(ctx, params); err != nil {
		s.config.Logger.Warnf("gopls didSave failed: %v", err)
	}

	return reply(ctx, nil, nil)
}

// handleDidClose processes didClose notifications
func (s *Server) handleDidClose(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.DidCloseTextDocumentParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}

	// CRITICAL FIX D1: When .dingo file closes, close corresponding .go file with gopls
	if isDingoFile(params.TextDocument.URI) {
		s.config.Logger.Debugf("Closed .dingo file: %s", params.TextDocument.URI)

		// Close incremental parser for this document
		s.docManager.CloseDocument(string(params.TextDocument.URI))

		// Close corresponding .go file with gopls
		if err := s.closeGoFileWithGopls(ctx, params.TextDocument.URI.Filename()); err != nil {
			s.config.Logger.Warnf("Failed to close .go file with gopls: %v", err)
		}

		return reply(ctx, nil, nil)
	}

	// Forward non-dingo files to gopls
	if err := s.gopls.DidClose(ctx, params); err != nil {
		s.config.Logger.Warnf("gopls didClose failed: %v", err)
	}

	return reply(ctx, nil, nil)
}

// handleCompletion processes completion requests with position translation
func (s *Server) handleCompletion(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	// Use enhanced handler with full response translation
	return s.handleCompletionWithTranslation(ctx, reply, req)
}

// handleDefinition processes definition requests with position translation
func (s *Server) handleDefinition(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	// Use enhanced handler with full response translation
	return s.handleDefinitionWithTranslation(ctx, reply, req)
}

// handleHover processes hover requests with position translation
func (s *Server) handleHover(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	// Use enhanced handler with full response translation
	return s.handleHoverWithTranslation(ctx, reply, req)
}

// handleDingoFileChange handles file changes detected by the watcher
func (s *Server) handleDingoFileChange(dingoPath string) {
	// IMPORTANT FIX I3: Use server context instead of background
	s.transpiler.OnFileChange(s.ctx, dingoPath)
}

// openGoFileWithGopls opens the corresponding .go file with gopls
// CRITICAL FIX D1: This enables gopls to analyze the file and send diagnostics
func (s *Server) openGoFileWithGopls(ctx context.Context, dingoPath string) error {
	// Convert .dingo path to .go path
	goPath := dingoToGoPath(dingoPath)

	s.config.Logger.Debugf("[Diagnostic Fix] Opening .go file with gopls: %s", goPath)

	// Read .go file contents
	contents, err := os.ReadFile(goPath)
	if err != nil {
		return fmt.Errorf("failed to read .go file: %w", err)
	}

	// Create didOpen params for gopls
	params := protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:        protocol.DocumentURI(lspuri.File(goPath)),
			LanguageID: "go",
			Version:    1,
			Text:       string(contents),
		},
	}

	// Open with gopls
	if err := s.gopls.DidOpen(ctx, params); err != nil {
		return fmt.Errorf("gopls didOpen failed: %w", err)
	}

	// Initialize version counter for this file to 1 (matching didOpen version)
	// This ensures SyncFileContent uses version 2, 3, etc. (not version 1 which conflicts with didOpen)
	s.gopls.InitFileVersion(string(params.TextDocument.URI), 1)

	s.config.Logger.Debugf("[Diagnostic Fix] Successfully opened .go file with gopls: %s", goPath)
	return nil
}

// closeGoFileWithGopls closes the corresponding .go file with gopls
// CRITICAL FIX D1: Clean up when .dingo file is closed
func (s *Server) closeGoFileWithGopls(ctx context.Context, dingoPath string) error {
	// Convert .dingo path to .go path
	goPath := dingoToGoPath(dingoPath)

	s.config.Logger.Debugf("[Diagnostic Fix] Closing .go file with gopls: %s", goPath)

	// Create didClose params for gopls
	params := protocol.DidCloseTextDocumentParams{
		TextDocument: protocol.TextDocumentIdentifier{
			URI: protocol.DocumentURI(lspuri.File(goPath)),
		},
	}

	// Close with gopls
	if err := s.gopls.DidClose(ctx, params); err != nil {
		return fmt.Errorf("gopls didClose failed: %w", err)
	}

	s.config.Logger.Debugf("[Diagnostic Fix] Successfully closed .go file with gopls: %s", goPath)
	return nil
}

// publishDingoDiagnostics publishes Dingo-specific diagnostics (e.g., transpilation errors)
// This is separate from gopls diagnostics (which are translated and forwarded)
func (s *Server) publishDingoDiagnostics(uri protocol.DocumentURI, diagnostics []protocol.Diagnostic) {
	// Get IDE connection (thread-safe)
	ideConn, serverCtx := s.GetConn()
	if ideConn == nil {
		s.config.Logger.Warnf("[Dingo Diagnostics] No IDE connection available, cannot publish diagnostics")
		return
	}

	// Prepare params
	params := protocol.PublishDiagnosticsParams{
		URI:         uri,
		Diagnostics: diagnostics,
	}

	// Use server context if available, otherwise background
	publishCtx := serverCtx
	if publishCtx == nil {
		publishCtx = context.Background()
	}

	// Publish to IDE
	err := ideConn.Notify(publishCtx, "textDocument/publishDiagnostics", params)
	if err != nil {
		s.config.Logger.Errorf("[Dingo Diagnostics] Failed to publish diagnostics: %v", err)
		return
	}

	if len(diagnostics) > 0 {
		s.config.Logger.Debugf("[Dingo Diagnostics] Published %d Dingo-specific diagnostic(s) for %s", len(diagnostics), uri)
	} else {
		s.config.Logger.Debugf("[Dingo Diagnostics] Cleared diagnostics for %s", uri)
	}
}

// updateAndPublishDiagnostics updates cached diagnostics for a source and publishes merged result
// source can be "lint", "gopls", "transpile", or "parse"
func (s *Server) updateAndPublishDiagnostics(uri protocol.DocumentURI, source string, diagnostics []protocol.Diagnostic) {
	s.diagMu.Lock()
	uriStr := string(uri)

	// Update the appropriate cache
	switch source {
	case "lint":
		s.lintDiags[uriStr] = diagnostics
	case "gopls":
		s.goplsDiags[uriStr] = diagnostics
	case "transpile":
		s.transpileDiags[uriStr] = diagnostics
	case "parse":
		s.parseDiags[uriStr] = diagnostics
	}

	// Merge all diagnostics for this URI
	var merged []protocol.Diagnostic
	merged = append(merged, s.lintDiags[uriStr]...)
	merged = append(merged, s.goplsDiags[uriStr]...)
	merged = append(merged, s.transpileDiags[uriStr]...)
	merged = append(merged, s.parseDiags[uriStr]...)

	s.diagMu.Unlock()

	// Get IDE connection (thread-safe)
	ideConn, serverCtx := s.GetConn()
	if ideConn == nil {
		s.config.Logger.Warnf("[Diagnostics] No IDE connection, cannot publish")
		return
	}

	// Prepare params
	params := protocol.PublishDiagnosticsParams{
		URI:         uri,
		Diagnostics: merged,
	}

	// Use server context if available
	publishCtx := serverCtx
	if publishCtx == nil {
		publishCtx = context.Background()
	}

	// Publish merged diagnostics to IDE
	err := ideConn.Notify(publishCtx, "textDocument/publishDiagnostics", params)
	if err != nil {
		s.config.Logger.Errorf("[Diagnostics] Failed to publish: %v", err)
		return
	}

	s.config.Logger.Debugf("[Diagnostics] Published %d merged diagnostics for %s (lint=%d, gopls=%d, transpile=%d, parse=%d)",
		len(merged), uri,
		len(s.lintDiags[uriStr]), len(s.goplsDiags[uriStr]), len(s.transpileDiags[uriStr]), len(s.parseDiags[uriStr]))
}

// forwardToGopls forwards unknown requests directly to gopls
func (s *Server) forwardToGopls(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	// This is a simplified forwarding - full implementation would use gopls connection directly
	s.config.Logger.Debugf("Method %s not implemented, returning error", req.Method())
	return reply(ctx, nil, fmt.Errorf("method not implemented: %s", req.Method()))
}

// scheduleTranspileFromBuffer schedules a debounced transpilation from in-memory buffer content.
// This provides responsive diagnostics while typing without overwhelming the system.
func (s *Server) scheduleTranspileFromBuffer(uri protocol.DocumentURI, content string) {
	uriStr := string(uri)

	s.transpileDebounceMu.Lock()
	defer s.transpileDebounceMu.Unlock()

	// Cancel existing timer if any
	if state, exists := s.transpileDebounce[uriStr]; exists {
		state.timer.Stop()
		state.content = content
	} else {
		s.transpileDebounce[uriStr] = &transpileDebounceState{content: content}
	}

	state := s.transpileDebounce[uriStr]

	// Start new timer
	state.timer = time.AfterFunc(time.Duration(s.transpileDebounceMs)*time.Millisecond, func() {
		s.executeTranspileFromBuffer(uri, state.content)
	})

	s.config.Logger.Debugf("[Debounce] Scheduled transpile for %s in %dms", uriStr, s.transpileDebounceMs)
}

// executeTranspileFromBuffer performs the actual transpilation from buffer content.
// Called after debounce timer fires.
func (s *Server) executeTranspileFromBuffer(uri protocol.DocumentURI, content string) {
	dingoPath := uri.Filename()

	s.config.Logger.Debugf("[Debounce] Executing transpile for %s", dingoPath)

	// Use PureASTTranspileWithMappings for in-memory transpilation
	result, err := transpiler.PureASTTranspileWithMappings([]byte(content), dingoPath, false)
	if err != nil {
		// Transpilation failed - publish error diagnostic
		s.config.Logger.Debugf("[Debounce] Transpile failed: %v", err)

		diagnostic := ParseTranspileError(dingoPath, err)
		if diagnostic != nil {
			s.updateAndPublishDiagnostics(uri, "transpile", []protocol.Diagnostic{*diagnostic})
		}
		return
	}

	// Transpilation succeeded - clear transpile diagnostics
	s.updateAndPublishDiagnostics(uri, "transpile", []protocol.Diagnostic{})

	// Write transpiled Go code to disk for gopls
	goPath := dingoToGoPath(dingoPath)
	if err := os.WriteFile(goPath, result.GoCode, 0644); err != nil {
		s.config.Logger.Warnf("[Debounce] Failed to write Go file: %v", err)
		return
	}

	// Write dmap file for position mapping
	dmapPath := goPath + ".dmap"
	writer := dmap.NewWriter(result.DingoSource, result.GoCode)
	if err := writer.WriteFile(dmapPath, result.LineMappings, result.ColumnMappings); err != nil {
		s.config.Logger.Warnf("[Debounce] Failed to write dmap: %v", err)
	}

	// Invalidate source map cache
	s.mapCache.Invalidate(goPath)

	// Sync gopls with new Go file content
	ctx := s.getServerContext()
	if ctx == nil {
		ctx = context.Background()
	}

	if err := s.transpiler.SyncGoplsWithGoFile(ctx, goPath); err != nil {
		s.config.Logger.Warnf("[Debounce] Failed to sync gopls: %v", err)
	}

	s.config.Logger.Debugf("[Debounce] Transpile complete for %s", dingoPath)
}

// getServerContext safely retrieves the server context
func (s *Server) getServerContext() context.Context {
	s.connMu.RLock()
	defer s.connMu.RUnlock()
	return s.ctx
}
