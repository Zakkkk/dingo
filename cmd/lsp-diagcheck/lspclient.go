package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// LSPClient handles JSON-RPC communication with an LSP server
// Extended from lsp-hovercheck to capture diagnostics notifications
type LSPClient struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	reader *bufio.Reader

	reqID    int64
	pending  map[int64]chan *Response
	mu       sync.Mutex
	closed   bool
	closedMu sync.RWMutex

	// Diagnostics storage
	diagnostics   map[string][]Diagnostic
	diagnosticsMu sync.RWMutex

	// For debugging
	Verbose bool

	// Stderr capture
	stderrBuf *strings.Builder
}

// Message types for LSP JSON-RPC
type Request struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int64       `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *ResponseError  `json:"error,omitempty"`
}

type ResponseError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type Notification struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// LSP types
type InitializeParams struct {
	ProcessID    int                `json:"processId"`
	RootURI      string             `json:"rootUri"`
	Capabilities ClientCapabilities `json:"capabilities"`
}

type ClientCapabilities struct {
	TextDocument TextDocumentClientCapabilities `json:"textDocument"`
}

type TextDocumentClientCapabilities struct {
	Hover              HoverClientCapabilities `json:"hover"`
	PublishDiagnostics DiagnosticsCapabilities `json:"publishDiagnostics"`
}

type HoverClientCapabilities struct {
	ContentFormat []string `json:"contentFormat"`
}

type DiagnosticsCapabilities struct {
	RelatedInformation bool `json:"relatedInformation"`
}

type TextDocumentItem struct {
	URI        string `json:"uri"`
	LanguageID string `json:"languageId"`
	Version    int    `json:"version"`
	Text       string `json:"text"`
}

type DidOpenTextDocumentParams struct {
	TextDocument TextDocumentItem `json:"textDocument"`
}

type DidSaveTextDocumentParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

type TextDocumentIdentifier struct {
	URI string `json:"uri"`
}

// Diagnostic types
type PublishDiagnosticsParams struct {
	URI         string       `json:"uri"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

type Diagnostic struct {
	Range    Range  `json:"range"`
	Severity int    `json:"severity"` // 1=Error, 2=Warning, 3=Info, 4=Hint
	Code     string `json:"code,omitempty"`
	Source   string `json:"source,omitempty"`
	Message  string `json:"message"`
}

type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

type Position struct {
	Line      int `json:"line"`      // 0-based
	Character int `json:"character"` // 0-based
}

// NewLSPClient starts an LSP server and returns a client
func NewLSPClient(lspPath string, verbose bool) (*LSPClient, error) {
	cmd := exec.Command(lspPath)

	stderrBuf := &strings.Builder{}
	cmd.Stderr = stderrBuf

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("creating stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("creating stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting LSP server: %w", err)
	}

	client := &LSPClient{
		cmd:         cmd,
		stdin:       stdin,
		stdout:      stdout,
		reader:      bufio.NewReader(stdout),
		pending:     make(map[int64]chan *Response),
		diagnostics: make(map[string][]Diagnostic),
		Verbose:     verbose,
		stderrBuf:   stderrBuf,
	}

	// Start response reader goroutine
	go client.readMessages()

	return client, nil
}

// Stderr returns captured stderr output
func (c *LSPClient) Stderr() string {
	if c.stderrBuf == nil {
		return ""
	}
	return c.stderrBuf.String()
}

// GetDiagnostics returns diagnostics for a URI
func (c *LSPClient) GetDiagnostics(uri string) []Diagnostic {
	c.diagnosticsMu.RLock()
	defer c.diagnosticsMu.RUnlock()
	return c.diagnostics[uri]
}

// readMessages continuously reads messages from the LSP server
func (c *LSPClient) readMessages() {
	for {
		c.closedMu.RLock()
		if c.closed {
			c.closedMu.RUnlock()
			return
		}
		c.closedMu.RUnlock()

		// Read headers
		var contentLength int
		for {
			line, err := c.reader.ReadString('\n')
			if err != nil {
				return
			}
			line = strings.TrimSpace(line)
			if line == "" {
				break
			}
			if strings.HasPrefix(line, "Content-Length:") {
				lenStr := strings.TrimSpace(strings.TrimPrefix(line, "Content-Length:"))
				contentLength, _ = strconv.Atoi(lenStr)
			}
		}

		if contentLength == 0 {
			continue
		}

		// Read body
		body := make([]byte, contentLength)
		_, err := io.ReadFull(c.reader, body)
		if err != nil {
			return
		}

		if c.Verbose {
			fmt.Printf("<-- %s\n", string(body))
		}

		// Try to parse as response (has ID)
		var resp Response
		if err := json.Unmarshal(body, &resp); err == nil && resp.ID != 0 {
			c.mu.Lock()
			if ch, ok := c.pending[resp.ID]; ok {
				ch <- &resp
				delete(c.pending, resp.ID)
			}
			c.mu.Unlock()
			continue
		}

		// Try to parse as notification (no ID, has Method)
		var notif Notification
		if err := json.Unmarshal(body, &notif); err == nil && notif.Method != "" {
			c.handleNotification(notif)
		}
	}
}

// handleNotification processes incoming notifications
func (c *LSPClient) handleNotification(notif Notification) {
	switch notif.Method {
	case "textDocument/publishDiagnostics":
		var params PublishDiagnosticsParams
		if err := json.Unmarshal(notif.Params, &params); err != nil {
			if c.Verbose {
				fmt.Printf("Error parsing diagnostics: %v\n", err)
			}
			return
		}

		c.diagnosticsMu.Lock()
		c.diagnostics[params.URI] = params.Diagnostics
		c.diagnosticsMu.Unlock()

		if c.Verbose {
			fmt.Printf("[DIAG] Received %d diagnostics for %s\n", len(params.Diagnostics), params.URI)
			for _, d := range params.Diagnostics {
				fmt.Printf("  Line %d: [%d] %s\n", d.Range.Start.Line+1, d.Severity, d.Message)
			}
		}

	default:
		if c.Verbose {
			fmt.Printf("[NOTIF] %s\n", notif.Method)
		}
	}
}

// Call sends a request and waits for response
func (c *LSPClient) Call(method string, params interface{}, timeout time.Duration) (json.RawMessage, error) {
	id := atomic.AddInt64(&c.reqID, 1)

	req := Request{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	respCh := make(chan *Response, 1)
	c.mu.Lock()
	c.pending[id] = respCh
	c.mu.Unlock()

	if err := c.send(req); err != nil {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, err
	}

	select {
	case resp := <-respCh:
		if resp.Error != nil {
			return nil, fmt.Errorf("LSP error %d: %s", resp.Error.Code, resp.Error.Message)
		}
		return resp.Result, nil
	case <-time.After(timeout):
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, fmt.Errorf("timeout waiting for response to %s", method)
	}
}

// Notify sends a notification (no response expected)
func (c *LSPClient) Notify(method string, params interface{}) error {
	notif := struct {
		JSONRPC string      `json:"jsonrpc"`
		Method  string      `json:"method"`
		Params  interface{} `json:"params,omitempty"`
	}{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}
	return c.send(notif)
}

func (c *LSPClient) send(msg interface{}) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshaling message: %w", err)
	}

	if c.Verbose {
		fmt.Printf("--> %s\n", string(body))
	}

	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body))
	_, err = c.stdin.Write([]byte(header))
	if err != nil {
		return fmt.Errorf("writing header: %w", err)
	}

	_, err = c.stdin.Write(body)
	if err != nil {
		return fmt.Errorf("writing body: %w", err)
	}

	return nil
}

// Close shuts down the LSP server
func (c *LSPClient) Close() error {
	c.closedMu.Lock()
	c.closed = true
	c.closedMu.Unlock()

	c.Call("shutdown", nil, time.Second)
	c.Notify("exit", nil)

	c.stdin.Close()
	c.stdout.Close()

	done := make(chan error, 1)
	go func() {
		done <- c.cmd.Wait()
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		c.cmd.Process.Kill()
	}

	return nil
}

// Initialize sends the initialize request
func (c *LSPClient) Initialize(rootURI string, timeout time.Duration) error {
	params := InitializeParams{
		ProcessID: 0,
		RootURI:   rootURI,
		Capabilities: ClientCapabilities{
			TextDocument: TextDocumentClientCapabilities{
				Hover: HoverClientCapabilities{
					ContentFormat: []string{"markdown", "plaintext"},
				},
				PublishDiagnostics: DiagnosticsCapabilities{
					RelatedInformation: true,
				},
			},
		},
	}

	_, err := c.Call("initialize", params, timeout)
	if err != nil {
		return fmt.Errorf("initialize failed: %w", err)
	}

	return c.Notify("initialized", struct{}{})
}

// DidOpen notifies the server that a document was opened
func (c *LSPClient) DidOpen(uri, languageID, text string) error {
	params := DidOpenTextDocumentParams{
		TextDocument: TextDocumentItem{
			URI:        uri,
			LanguageID: languageID,
			Version:    1,
			Text:       text,
		},
	}
	return c.Notify("textDocument/didOpen", params)
}

// DidSave notifies the server that a document was saved
func (c *LSPClient) DidSave(uri string) error {
	params := DidSaveTextDocumentParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
	}
	return c.Notify("textDocument/didSave", params)
}

// DidChangeTextDocumentParams for didChange notification
type DidChangeTextDocumentParams struct {
	TextDocument   VersionedTextDocumentIdentifier  `json:"textDocument"`
	ContentChanges []TextDocumentContentChangeEvent `json:"contentChanges"`
}

type VersionedTextDocumentIdentifier struct {
	URI     string `json:"uri"`
	Version int    `json:"version"`
}

type TextDocumentContentChangeEvent struct {
	Text string `json:"text"` // Full document sync
}

// DidChange notifies the server that a document was changed (full sync)
func (c *LSPClient) DidChange(uri string, version int, newContent string) error {
	params := DidChangeTextDocumentParams{
		TextDocument: VersionedTextDocumentIdentifier{
			URI:     uri,
			Version: version,
		},
		ContentChanges: []TextDocumentContentChangeEvent{
			{Text: newContent},
		},
	}
	return c.Notify("textDocument/didChange", params)
}
