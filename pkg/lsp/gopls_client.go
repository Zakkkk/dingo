package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// DiagnosticsHandler is called when gopls sends diagnostics
type DiagnosticsHandler func(ctx context.Context, params protocol.PublishDiagnosticsParams) error

// GoplsClient manages a gopls subprocess and forwards LSP requests
type GoplsClient struct {
	cmd                *exec.Cmd
	conn               jsonrpc2.Conn
	logger             Logger
	goplsPath          string
	restarts           int
	maxRestarts        int
	mu                 sync.Mutex
	shuttingDown       bool               // CRITICAL FIX C2: Track shutdown state
	closeMu            sync.Mutex         // CRITICAL FIX C2: Protect shutdown flag
	diagnosticsHandler DiagnosticsHandler // Callback for diagnostics

	// Version tracking for gopls sync - prevents version conflicts
	versionMu   sync.Mutex
	fileVersion map[string]int32 // URI -> current version
}

// NewGoplsClient creates and starts a gopls subprocess
func NewGoplsClient(goplsPath string, logger Logger) (*GoplsClient, error) {
	// Verify gopls exists
	if _, err := exec.LookPath(goplsPath); err != nil {
		return nil, fmt.Errorf("gopls not found at %s: %w (install: go install golang.org/x/tools/gopls@latest)", goplsPath, err)
	}

	client := &GoplsClient{
		logger:      logger,
		goplsPath:   goplsPath,
		maxRestarts: 3,
		fileVersion: make(map[string]int32),
	}

	if err := client.start(); err != nil {
		return nil, err
	}

	return client, nil
}

// SetDiagnosticsHandler sets the callback for handling diagnostics from gopls
func (c *GoplsClient) SetDiagnosticsHandler(handler DiagnosticsHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.diagnosticsHandler = handler
}

func (c *GoplsClient) start() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Start gopls subprocess with -mode=stdio
	c.cmd = exec.Command(c.goplsPath, "-mode=stdio")

	stdin, err := c.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := c.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := c.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start gopls
	if err := c.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start gopls: %w", err)
	}

	// Log stderr in background
	go c.logStderr(stderr)

	// Create JSON-RPC connection using a buffered ReadWriteCloser wrapper (GPT-5 fix)
	rwc := newReadWriteCloser(stdin, stdout)
	stream := jsonrpc2.NewStream(rwc)
	c.conn = jsonrpc2.NewConn(stream)

	// Start handler to process gopls responses and notifications
	ctx := context.Background()
	handler := jsonrpc2.ReplyHandler(func(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
		// Log gopls -> dingo-lsp notifications/requests (if any)
		c.logger.Debugf("gopls notification/request: %s", req.Method())

		// Handle server->client requests from gopls
		switch req.Method() {
		case "client/registerCapability", "client/unregisterCapability":
			// Accept capability registration (we don't need to track them)
			return reply(ctx, nil, nil)
		case "window/showMessage", "window/logMessage":
			// Log messages from gopls
			var params map[string]interface{}
			if err := json.Unmarshal(req.Params(), &params); err == nil {
				c.logger.Debugf("gopls %s: %v", req.Method(), params)
			}
			return reply(ctx, nil, nil)
		case "textDocument/publishDiagnostics":
			// Forward diagnostics to handler (for translation to .dingo positions)
			c.logger.Debugf("[gopls → dingo-lsp] Received publishDiagnostics notification")

			var params protocol.PublishDiagnosticsParams
			if err := json.Unmarshal(req.Params(), &params); err != nil {
				c.logger.Warnf("[gopls → dingo-lsp] Failed to unmarshal diagnostics: %v", err)
				return reply(ctx, nil, nil)
			}

			c.logger.Debugf("[gopls → dingo-lsp] Diagnostics for URI=%s, count=%d",
				params.URI.Filename(), len(params.Diagnostics))

			// Log each diagnostic from gopls
			for i, diag := range params.Diagnostics {
				c.logger.Debugf("[gopls → dingo-lsp]   [%d] Severity=%d, Message=%q, Range=L%d:C%d-L%d:C%d",
					i, diag.Severity, diag.Message,
					diag.Range.Start.Line, diag.Range.Start.Character,
					diag.Range.End.Line, diag.Range.End.Character)
			}

			// Call diagnostics handler if set
			c.mu.Lock()
			handler := c.diagnosticsHandler
			c.mu.Unlock()

			if handler != nil {
				c.logger.Debugf("[gopls → dingo-lsp] Calling diagnostics handler")
				if err := handler(ctx, params); err != nil {
					c.logger.Warnf("[gopls → dingo-lsp] Diagnostics handler error: %v", err)
				} else {
					c.logger.Debugf("[gopls → dingo-lsp] Diagnostics handler completed successfully")
				}
			} else {
				c.logger.Debugf("[gopls → dingo-lsp] No diagnostics handler set, discarding %d diagnostics for %s",
					len(params.Diagnostics), params.URI)
			}

			return reply(ctx, nil, nil)
		default:
			// Unknown method - reply with empty result
			c.logger.Debugf("gopls unknown method: %s", req.Method())
			return reply(ctx, nil, nil)
		}
	})
	c.conn.Go(ctx, handler)

	c.logger.Infof("gopls started (PID: %d)", c.cmd.Process.Pid)

	// CRITICAL FIX C2: Monitor process exit for crash recovery
	go func() {
		err := c.cmd.Wait()

		c.closeMu.Lock()
		shutdown := c.shuttingDown
		c.closeMu.Unlock()

		if err != nil && !shutdown {
			c.logger.Warnf("gopls process exited unexpectedly: %v", err)
			if crashErr := c.handleCrash(); crashErr != nil {
				c.logger.Errorf("Failed to restart gopls: %v", crashErr)
			}
		}
	}()

	return nil
}

func (c *GoplsClient) logStderr(stderr io.Reader) {
	// IMPORTANT FIX I4: Use bufio.Scanner for better handling
	// Handles large panic stack traces without truncation
	scanner := bufio.NewScanner(stderr)
	scanner.Buffer(make([]byte, 4096), 1024*1024) // 4KB initial, 1MB max

	for scanner.Scan() {
		line := scanner.Text()
		c.logger.Debugf("gopls stderr: %s", line)
	}

	if err := scanner.Err(); err != nil && err != io.EOF {
		c.logger.Debugf("stderr scan error: %v", err)
	}
}

// Initialize sends initialize request to gopls with timeout (GPT-5 fix)
func (c *GoplsClient) Initialize(ctx context.Context, params protocol.InitializeParams) (*protocol.InitializeResult, error) {
	// Check if gopls process is still alive
	c.mu.Lock()
	if c.cmd == nil || c.cmd.Process == nil {
		c.mu.Unlock()
		return nil, fmt.Errorf("gopls process not running")
	}
	c.mu.Unlock()

	// Add timeout to prevent hanging forever
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	c.logger.Debugf("Calling gopls initialize")
	var result protocol.InitializeResult
	_, err := c.conn.Call(ctx, "initialize", params, &result)
	if err != nil {
		c.logger.Errorf("gopls initialize call failed: %v", err)
		return nil, fmt.Errorf("gopls initialize failed: %w", err)
	}
	c.logger.Debugf("gopls initialize succeeded")
	return &result, nil
}

// Initialized sends initialized notification to gopls
func (c *GoplsClient) Initialized(ctx context.Context, params *protocol.InitializedParams) error {
	return c.conn.Notify(ctx, "initialized", params)
}

// Completion forwards completion request to gopls
func (c *GoplsClient) Completion(ctx context.Context, params protocol.CompletionParams) (*protocol.CompletionList, error) {
	var result protocol.CompletionList
	_, err := c.conn.Call(ctx, "textDocument/completion", params, &result)
	if err != nil {
		return nil, fmt.Errorf("gopls completion failed: %w", err)
	}
	return &result, nil
}

// Definition forwards definition request to gopls
func (c *GoplsClient) Definition(ctx context.Context, params protocol.DefinitionParams) ([]protocol.Location, error) {
	var result []protocol.Location
	_, err := c.conn.Call(ctx, "textDocument/definition", params, &result)
	if err != nil {
		return nil, fmt.Errorf("gopls definition failed: %w", err)
	}
	return result, nil
}

// TypeDefinition forwards type definition request to gopls
func (c *GoplsClient) TypeDefinition(ctx context.Context, params protocol.TypeDefinitionParams) ([]protocol.Location, error) {
	var result []protocol.Location
	_, err := c.conn.Call(ctx, "textDocument/typeDefinition", params, &result)
	if err != nil {
		return nil, fmt.Errorf("gopls typeDefinition failed: %w", err)
	}
	return result, nil
}

// Hover forwards hover request to gopls
func (c *GoplsClient) Hover(ctx context.Context, params protocol.HoverParams) (*protocol.Hover, error) {
	var result protocol.Hover
	_, err := c.conn.Call(ctx, "textDocument/hover", params, &result)
	if err != nil {
		return nil, fmt.Errorf("gopls hover failed: %w", err)
	}
	return &result, nil
}

// References forwards references request to gopls
func (c *GoplsClient) References(ctx context.Context, params protocol.ReferenceParams) ([]protocol.Location, error) {
	var result []protocol.Location
	_, err := c.conn.Call(ctx, "textDocument/references", params, &result)
	if err != nil {
		return nil, fmt.Errorf("gopls references failed: %w", err)
	}
	return result, nil
}

// Implementation forwards implementation request to gopls
func (c *GoplsClient) Implementation(ctx context.Context, params protocol.ImplementationParams) ([]protocol.Location, error) {
	var result []protocol.Location
	_, err := c.conn.Call(ctx, "textDocument/implementation", params, &result)
	if err != nil {
		return nil, fmt.Errorf("gopls implementation failed: %w", err)
	}
	return result, nil
}

// DocumentSymbol forwards document symbol request to gopls.
// Returns []any because gopls may return either []DocumentSymbol or []SymbolInformation
// depending on client capabilities.
func (c *GoplsClient) DocumentSymbol(ctx context.Context, params protocol.DocumentSymbolParams) ([]any, error) {
	var result []any
	_, err := c.conn.Call(ctx, "textDocument/documentSymbol", params, &result)
	if err != nil {
		return nil, fmt.Errorf("gopls documentSymbol failed: %w", err)
	}
	return result, nil
}

// WorkspaceSymbol forwards workspace symbol request to gopls
func (c *GoplsClient) WorkspaceSymbol(ctx context.Context, params protocol.WorkspaceSymbolParams) ([]protocol.SymbolInformation, error) {
	var result []protocol.SymbolInformation
	_, err := c.conn.Call(ctx, "workspace/symbol", params, &result)
	if err != nil {
		return nil, fmt.Errorf("gopls workspaceSymbol failed: %w", err)
	}
	return result, nil
}

// PrepareCallHierarchy forwards call hierarchy prepare request to gopls
func (c *GoplsClient) PrepareCallHierarchy(ctx context.Context, params protocol.CallHierarchyPrepareParams) ([]protocol.CallHierarchyItem, error) {
	var result []protocol.CallHierarchyItem
	_, err := c.conn.Call(ctx, "textDocument/prepareCallHierarchy", params, &result)
	if err != nil {
		return nil, fmt.Errorf("gopls prepareCallHierarchy failed: %w", err)
	}
	return result, nil
}

// IncomingCalls forwards incoming calls request to gopls
func (c *GoplsClient) IncomingCalls(ctx context.Context, params protocol.CallHierarchyIncomingCallsParams) ([]protocol.CallHierarchyIncomingCall, error) {
	var result []protocol.CallHierarchyIncomingCall
	_, err := c.conn.Call(ctx, "callHierarchy/incomingCalls", params, &result)
	if err != nil {
		return nil, fmt.Errorf("gopls incomingCalls failed: %w", err)
	}
	return result, nil
}

// OutgoingCalls forwards outgoing calls request to gopls
func (c *GoplsClient) OutgoingCalls(ctx context.Context, params protocol.CallHierarchyOutgoingCallsParams) ([]protocol.CallHierarchyOutgoingCall, error) {
	var result []protocol.CallHierarchyOutgoingCall
	_, err := c.conn.Call(ctx, "callHierarchy/outgoingCalls", params, &result)
	if err != nil {
		return nil, fmt.Errorf("gopls outgoingCalls failed: %w", err)
	}
	return result, nil
}

// DidOpen notifies gopls of opened file
func (c *GoplsClient) DidOpen(ctx context.Context, params protocol.DidOpenTextDocumentParams) error {
	return c.conn.Notify(ctx, "textDocument/didOpen", params)
}

// DidChange notifies gopls of file changes
func (c *GoplsClient) DidChange(ctx context.Context, params protocol.DidChangeTextDocumentParams) error {
	return c.conn.Notify(ctx, "textDocument/didChange", params)
}

// DidSave notifies gopls of file save
func (c *GoplsClient) DidSave(ctx context.Context, params protocol.DidSaveTextDocumentParams) error {
	return c.conn.Notify(ctx, "textDocument/didSave", params)
}

// DidClose notifies gopls of closed file
func (c *GoplsClient) DidClose(ctx context.Context, params protocol.DidCloseTextDocumentParams) error {
	return c.conn.Notify(ctx, "textDocument/didClose", params)
}

// InitFileVersion initializes the version counter for a file.
// Call this after didOpen to ensure subsequent didChange uses version 2+
func (c *GoplsClient) InitFileVersion(uri string, version int32) {
	c.versionMu.Lock()
	defer c.versionMu.Unlock()
	c.fileVersion[uri] = version
}

// fullDocumentChange represents a full-document content change for LSP didChange.
// We use a custom struct because go.lsp.dev/protocol's TextDocumentContentChangeEvent
// always marshals the Range field (no omitempty), causing gopls to interpret it as
// "insert at 0:0" instead of "replace all".
type fullDocumentChange struct {
	Text string `json:"text"`
}

// didChangeParams is a custom params struct for textDocument/didChange
// that properly omits Range for full-document sync.
type didChangeParams struct {
	TextDocument   versionedTextDocumentIdentifier `json:"textDocument"`
	ContentChanges []fullDocumentChange            `json:"contentChanges"`
}

// versionedTextDocumentIdentifier for didChange notifications
type versionedTextDocumentIdentifier struct {
	URI     protocol.DocumentURI `json:"uri"`
	Version int32                `json:"version"`
}

// SyncFileContent synchronizes gopls with the current content of a .go file
// This is critical for keeping gopls in sync after transpilation
//
// PERFORMANCE FIX: Uses didChange instead of didClose+didOpen for incremental analysis.
// The old didClose+didOpen pattern forced gopls to discard all cached analysis and
// re-type-check the entire package from scratch, causing 5-15 second delays.
// With proper didChange, gopls can use incremental analysis for ~0.5-2s response.
//
// Strategy:
// - If file is already open (version > 0): use didChange (fast incremental path)
// - If file is not open (version == 0): use didOpen (first time opening)
//
// We use custom structs instead of go.lsp.dev/protocol types because the library's
// TextDocumentContentChangeEvent has Range without omitempty, causing JSON to always
// include {"range": {...}}, which gopls interprets as "insert at 0:0" not "replace all".
func (c *GoplsClient) SyncFileContent(ctx context.Context, goPath string) error {
	c.logger.Debugf("Syncing .go file with gopls: %s", goPath)

	// Read current .go file content from disk
	content, err := os.ReadFile(goPath)
	if err != nil {
		return fmt.Errorf("failed to read .go file: %w", err)
	}

	fileURI := protocol.DocumentURI(uri.File(goPath))
	uriStr := string(fileURI)

	// Check if file is already open with gopls
	c.versionMu.Lock()
	currentVersion := c.fileVersion[uriStr]
	c.versionMu.Unlock()

	if currentVersion == 0 {
		// File not opened yet - use didOpen
		c.logger.Debugf("File not opened yet, using didOpen for %s", goPath)
		return c.syncViaDidOpen(ctx, fileURI, uriStr, content)
	}

	// File already open - use didChange for incremental update
	return c.syncViaDidChange(ctx, fileURI, uriStr, content)
}

// syncViaDidOpen opens a file with gopls using didOpen notification
func (c *GoplsClient) syncViaDidOpen(ctx context.Context, fileURI protocol.DocumentURI, uriStr string, content []byte) error {
	// Set version to 1 for fresh open
	c.versionMu.Lock()
	c.fileVersion[uriStr] = 1
	c.versionMu.Unlock()

	openParams := protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:        fileURI,
			LanguageID: "go",
			Version:    1,
			Text:       string(content),
		},
	}

	c.logger.Debugf("Sending didOpen to gopls with %d bytes for %s", len(content), fileURI.Filename())
	if err := c.conn.Notify(ctx, "textDocument/didOpen", openParams); err != nil {
		return fmt.Errorf("failed to send didOpen to gopls: %w", err)
	}

	c.logger.Debugf("gopls synchronized with .go file: %s (via didOpen)", fileURI.Filename())
	return nil
}

// syncViaDidChange updates an already-open file with gopls using didChange notification
func (c *GoplsClient) syncViaDidChange(ctx context.Context, fileURI protocol.DocumentURI, uriStr string, content []byte) error {
	// Increment version for this file
	c.versionMu.Lock()
	c.fileVersion[uriStr]++
	version := c.fileVersion[uriStr]
	c.versionMu.Unlock()

	// Use custom struct to properly omit Range for full-document replacement
	params := didChangeParams{
		TextDocument: versionedTextDocumentIdentifier{
			URI:     fileURI,
			Version: version,
		},
		ContentChanges: []fullDocumentChange{
			{Text: string(content)},
		},
	}

	c.logger.Debugf("Sending didChange to gopls with %d bytes, version %d for %s", len(content), version, fileURI.Filename())
	if err := c.conn.Notify(ctx, "textDocument/didChange", params); err != nil {
		return fmt.Errorf("failed to send didChange to gopls: %w", err)
	}

	c.logger.Debugf("gopls synchronized with .go file: %s (via didChange, version %d)", fileURI.Filename(), version)
	return nil
}

// NotifyFileChange notifies gopls that a .go file changed (for auto-transpile)
// Deprecated: Use SyncFileContent instead for reliable synchronization
func (c *GoplsClient) NotifyFileChange(ctx context.Context, goPath string) error {
	fileURI := uri.File(goPath)
	fileEvent := protocol.FileEvent{
		URI:  fileURI,
		Type: protocol.FileChangeTypeChanged,
	}
	params := protocol.DidChangeWatchedFilesParams{
		Changes: []*protocol.FileEvent{&fileEvent},
	}
	return c.conn.Notify(ctx, "workspace/didChangeWatchedFiles", params)
}

// Shutdown gracefully shuts down gopls
func (c *GoplsClient) Shutdown(ctx context.Context) error {
	// CRITICAL FIX C2: Set shutdown flag to prevent crash recovery
	c.closeMu.Lock()
	c.shuttingDown = true
	c.closeMu.Unlock()

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return nil
	}

	// Send shutdown request
	if _, err := c.conn.Call(ctx, "shutdown", nil, nil); err != nil {
		c.logger.Warnf("gopls shutdown request failed: %v", err)
	}

	// Send exit notification
	if err := c.conn.Notify(ctx, "exit", nil); err != nil {
		c.logger.Warnf("gopls exit notification failed: %v", err)
	}

	// Close connection
	if err := c.conn.Close(); err != nil {
		c.logger.Debugf("gopls connection close error: %v", err)
	}

	// Wait for process to exit
	if c.cmd != nil && c.cmd.Process != nil {
		if err := c.cmd.Wait(); err != nil {
			c.logger.Debugf("gopls process wait error: %v", err)
		}
		c.logger.Infof("gopls stopped (PID: %d)", c.cmd.Process.Pid)
	}

	return nil
}

// handleCrash attempts to restart gopls after a crash
func (c *GoplsClient) handleCrash() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.restarts >= c.maxRestarts {
		return fmt.Errorf("gopls crashed %d times, giving up", c.restarts)
	}

	c.logger.Warnf("gopls crashed, restarting (attempt %d/%d)", c.restarts+1, c.maxRestarts)
	c.restarts++

	return c.start()
}

// readWriteCloser combines separate Read and Write closers with buffering (GPT-5 fix)
type readWriteCloser struct {
	stdin  io.WriteCloser
	stdout io.ReadCloser
	reader *bufio.Reader
	writer *bufio.Writer
}

func newReadWriteCloser(stdin io.WriteCloser, stdout io.ReadCloser) *readWriteCloser {
	return &readWriteCloser{
		stdin:  stdin,
		stdout: stdout,
		reader: bufio.NewReaderSize(stdout, 32*1024), // 32KB buffer
		writer: bufio.NewWriterSize(stdin, 32*1024),  // 32KB buffer
	}
}

func (rwc *readWriteCloser) Read(p []byte) (n int, err error) {
	return rwc.reader.Read(p)
}

func (rwc *readWriteCloser) Write(p []byte) (n int, err error) {
	n, err = rwc.writer.Write(p)
	if err != nil {
		return n, err
	}
	// Flush after each write to ensure messages are sent immediately
	return n, rwc.writer.Flush()
}

func (rwc *readWriteCloser) Close() error {
	// Flush any remaining data
	_ = rwc.writer.Flush()
	err1 := rwc.stdin.Close()
	err2 := rwc.stdout.Close()
	if err1 != nil {
		return err1
	}
	return err2
}
