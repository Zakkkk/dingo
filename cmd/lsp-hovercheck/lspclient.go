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

	// For debugging
	Verbose bool

	// Stderr capture for error diagnosis
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
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
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
	Hover HoverClientCapabilities `json:"hover"`
}

type HoverClientCapabilities struct {
	ContentFormat []string `json:"contentFormat"`
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

type TextDocumentIdentifier struct {
	URI string `json:"uri"`
}

type Position struct {
	Line      int `json:"line"`      // 0-based
	Character int `json:"character"` // 0-based
}

type HoverParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

type Hover struct {
	Contents MarkupContent `json:"contents"`
}

type MarkupContent struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

// NewLSPClient starts an LSP server and returns a client
func NewLSPClient(lspPath string, verbose bool) (*LSPClient, error) {
	cmd := exec.Command(lspPath)

	// Capture stderr for debugging instead of suppressing
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
		cmd:       cmd,
		stdin:     stdin,
		stdout:    stdout,
		reader:    bufio.NewReader(stdout),
		pending:   make(map[int64]chan *Response),
		Verbose:   verbose,
		stderrBuf: stderrBuf,
	}

	// Start response reader goroutine
	go client.readResponses()

	return client, nil
}

// Stderr returns captured stderr output (for debugging)
func (c *LSPClient) Stderr() string {
	if c.stderrBuf == nil {
		return ""
	}
	return c.stderrBuf.String()
}

// readResponses continuously reads responses from the LSP server
func (c *LSPClient) readResponses() {
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

		// Try to parse as response
		var resp Response
		if err := json.Unmarshal(body, &resp); err == nil && resp.ID != 0 {
			c.mu.Lock()
			if ch, ok := c.pending[resp.ID]; ok {
				// Send the full response (including any error)
				ch <- &resp
				delete(c.pending, resp.ID)
			}
			c.mu.Unlock()
		}
		// Ignore notifications for now
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

	// Create response channel
	respCh := make(chan *Response, 1)
	c.mu.Lock()
	c.pending[id] = respCh
	c.mu.Unlock()

	// Send request
	if err := c.send(req); err != nil {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, err
	}

	// Wait for response
	select {
	case resp := <-respCh:
		// Check for JSON-RPC error response
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
	notif := Notification{
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

	// Send shutdown request (best effort)
	c.Call("shutdown", nil, time.Second)
	c.Notify("exit", nil)

	c.stdin.Close()
	c.stdout.Close()

	// Wait briefly, then kill
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
			},
		},
	}

	_, err := c.Call("initialize", params, timeout)
	if err != nil {
		return fmt.Errorf("initialize failed: %w", err)
	}

	// Send initialized notification
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

// Hover requests hover information at a position
func (c *LSPClient) Hover(uri string, line, character int, timeout time.Duration) (string, error) {
	params := HoverParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: line, Character: character},
	}

	result, err := c.Call("textDocument/hover", params, timeout)
	if err != nil {
		return "", err
	}

	if result == nil || string(result) == "null" {
		return "", nil
	}

	// Parse hover result - can be various formats
	var hover struct {
		Contents interface{} `json:"contents"`
	}
	if err := json.Unmarshal(result, &hover); err != nil {
		return "", fmt.Errorf("parsing hover result: %w", err)
	}

	return extractHoverText(hover.Contents), nil
}

// extractHoverText extracts text from various hover content formats
func extractHoverText(contents interface{}) string {
	if contents == nil {
		return ""
	}

	switch c := contents.(type) {
	case string:
		return c
	case map[string]interface{}:
		// MarkupContent or MarkedString
		if value, ok := c["value"].(string); ok {
			return value
		}
		if kind, ok := c["kind"].(string); ok && kind != "" {
			if value, ok := c["value"].(string); ok {
				return value
			}
		}
	case []interface{}:
		// Array of MarkedStrings
		var parts []string
		for _, item := range c {
			if s, ok := item.(string); ok {
				parts = append(parts, s)
			} else if m, ok := item.(map[string]interface{}); ok {
				if value, ok := m["value"].(string); ok {
					parts = append(parts, value)
				}
			}
		}
		return strings.Join(parts, "\n")
	}

	// Fallback: try to marshal back to string
	data, _ := json.Marshal(contents)
	return string(data)
}
