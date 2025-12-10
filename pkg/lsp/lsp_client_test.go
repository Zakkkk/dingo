package lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// LSPTestClient provides a programmatic interface to test LSP servers
type LSPTestClient struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader
	stderr io.ReadCloser
	nextID int
	mu     sync.Mutex
	t      *testing.T
}

// LSPRequest represents a JSON-RPC request
type LSPRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id,omitempty"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
}

// LSPResponse represents a JSON-RPC response
type LSPResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *LSPError       `json:"error,omitempty"`
}

// LSPError represents an LSP error
type LSPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// HoverResult represents hover response content
type HoverResult struct {
	Contents struct {
		Kind  string `json:"kind"`
		Value string `json:"value"`
	} `json:"contents"`
	Range *Range `json:"range,omitempty"`
}

// Range represents an LSP range
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// Position represents an LSP position (0-based)
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// CompletionList represents completion response
type CompletionList struct {
	IsIncomplete bool             `json:"isIncomplete"`
	Items        []CompletionItem `json:"items"`
}

// CompletionItem represents a single completion suggestion
type CompletionItem struct {
	Label         string `json:"label"`
	Kind          int    `json:"kind"`
	Detail        string `json:"detail,omitempty"`
	Documentation string `json:"documentation,omitempty"`
}

// NewLSPTestClient creates a new test client that starts the LSP server
func NewLSPTestClient(t *testing.T, serverPath string, args ...string) *LSPTestClient {
	t.Helper()

	cmd := exec.Command(serverPath, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("Failed to create stdin pipe: %v", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("Failed to create stdout pipe: %v", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		t.Fatalf("Failed to create stderr pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start LSP server: %v", err)
	}

	client := &LSPTestClient{
		cmd:    cmd,
		stdin:  stdin,
		stdout: bufio.NewReader(stdout),
		stderr: stderr,
		nextID: 1,
		t:      t,
	}

	return client
}

// Initialize sends the initialize request to the server
func (c *LSPTestClient) Initialize(rootURI string) error {
	c.mu.Lock()
	id := c.nextID
	c.nextID++
	c.mu.Unlock()

	params := map[string]interface{}{
		"processId": os.Getpid(),
		"rootUri":   rootURI,
		"capabilities": map[string]interface{}{
			"textDocument": map[string]interface{}{
				"hover": map[string]interface{}{
					"contentFormat": []string{"markdown", "plaintext"},
				},
				"completion": map[string]interface{}{
					"completionItem": map[string]interface{}{
						"snippetSupport": true,
					},
				},
			},
		},
	}

	resp, err := c.sendRequest(id, "initialize", params)
	if err != nil {
		return fmt.Errorf("initialize request failed: %w", err)
	}

	if resp.Error != nil {
		return fmt.Errorf("initialize error: %s", resp.Error.Message)
	}

	// Send initialized notification
	c.sendNotification("initialized", map[string]interface{}{})
	time.Sleep(500 * time.Millisecond) // Wait for server to process

	return nil
}

// Hover sends a hover request and returns the result
func (c *LSPTestClient) Hover(uri string, line, character int) (*HoverResult, error) {
	c.mu.Lock()
	id := c.nextID
	c.nextID++
	c.mu.Unlock()

	params := map[string]interface{}{
		"textDocument": map[string]string{
			"uri": uri,
		},
		"position": map[string]int{
			"line":      line,
			"character": character,
		},
	}

	resp, err := c.sendRequest(id, "textDocument/hover", params)
	if err != nil {
		return nil, fmt.Errorf("hover request failed: %w", err)
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("hover error: %s", resp.Error.Message)
	}

	if resp.Result == nil || string(resp.Result) == "null" {
		return nil, nil // No hover info available
	}

	var result HoverResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to parse hover result: %w", err)
	}

	return &result, nil
}

// Completion sends a completion request and returns the result
func (c *LSPTestClient) Completion(uri string, line, character int) (*CompletionList, error) {
	c.mu.Lock()
	id := c.nextID
	c.nextID++
	c.mu.Unlock()

	params := map[string]interface{}{
		"textDocument": map[string]string{
			"uri": uri,
		},
		"position": map[string]int{
			"line":      line,
			"character": character,
		},
	}

	resp, err := c.sendRequest(id, "textDocument/completion", params)
	if err != nil {
		return nil, fmt.Errorf("completion request failed: %w", err)
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("completion error: %s", resp.Error.Message)
	}

	if resp.Result == nil || string(resp.Result) == "null" {
		return nil, nil
	}

	var result CompletionList
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		// Try parsing as array (some servers return array directly)
		var items []CompletionItem
		if err2 := json.Unmarshal(resp.Result, &items); err2 == nil {
			return &CompletionList{Items: items}, nil
		}
		return nil, fmt.Errorf("failed to parse completion result: %w", err)
	}

	return &result, nil
}

// Definition sends a go-to-definition request
func (c *LSPTestClient) Definition(uri string, line, character int) ([]Location, error) {
	c.mu.Lock()
	id := c.nextID
	c.nextID++
	c.mu.Unlock()

	params := map[string]interface{}{
		"textDocument": map[string]string{
			"uri": uri,
		},
		"position": map[string]int{
			"line":      line,
			"character": character,
		},
	}

	resp, err := c.sendRequest(id, "textDocument/definition", params)
	if err != nil {
		return nil, fmt.Errorf("definition request failed: %w", err)
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("definition error: %s", resp.Error.Message)
	}

	if resp.Result == nil || string(resp.Result) == "null" {
		return nil, nil
	}

	// Can be single location or array
	var locations []Location
	if err := json.Unmarshal(resp.Result, &locations); err != nil {
		var single Location
		if err2 := json.Unmarshal(resp.Result, &single); err2 == nil {
			return []Location{single}, nil
		}
		return nil, fmt.Errorf("failed to parse definition result: %w", err)
	}

	return locations, nil
}

// Location represents an LSP location
type Location struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

// Shutdown gracefully shuts down the server
func (c *LSPTestClient) Shutdown() error {
	c.mu.Lock()
	id := c.nextID
	c.nextID++
	c.mu.Unlock()

	_, err := c.sendRequest(id, "shutdown", nil)
	if err != nil {
		// Try to kill anyway
		c.stdin.Close()
		c.cmd.Process.Kill()
		c.cmd.Wait()
		return err
	}

	// Send exit notification
	c.sendNotification("exit", nil)

	c.stdin.Close()
	return c.cmd.Wait()
}

func (c *LSPTestClient) sendRequest(id int, method string, params interface{}) (*LSPResponse, error) {
	req := LSPRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body))
	if _, err := c.stdin.Write([]byte(header)); err != nil {
		return nil, err
	}
	if _, err := c.stdin.Write(body); err != nil {
		return nil, err
	}

	return c.readResponse()
}

func (c *LSPTestClient) sendNotification(method string, params interface{}) error {
	req := LSPRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return err
	}

	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body))
	if _, err := c.stdin.Write([]byte(header)); err != nil {
		return err
	}
	_, err = c.stdin.Write(body)
	return err
}

func (c *LSPTestClient) readResponse() (*LSPResponse, error) {
	var contentLen int

	// Read headers
	for {
		line, err := c.stdout.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("failed to read header: %w", err)
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}
		if strings.HasPrefix(line, "Content-Length:") {
			lenStr := strings.TrimSpace(strings.TrimPrefix(line, "Content-Length:"))
			contentLen, _ = strconv.Atoi(lenStr)
		}
	}

	if contentLen == 0 {
		return nil, fmt.Errorf("no Content-Length header")
	}

	// Read body
	body := make([]byte, contentLen)
	if _, err := io.ReadFull(c.stdout, body); err != nil {
		return nil, fmt.Errorf("failed to read body: %w", err)
	}

	var resp LSPResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w (body: %s)", err, string(body))
	}

	return &resp, nil
}

// HoverTest defines a hover test case
type HoverTest struct {
	Name        string
	File        string // Relative to workspace root
	Line        int    // 0-based
	Character   int    // 0-based
	WantContain string // Expected substring in hover content
	WantEmpty   bool   // Expect no hover result
}

// RunHoverTests runs a batch of hover tests
func (c *LSPTestClient) RunHoverTests(t *testing.T, workspaceURI string, tests []HoverTest) {
	t.Helper()

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			uri := workspaceURI + "/" + tc.File

			result, err := c.Hover(uri, tc.Line, tc.Character)
			if err != nil {
				t.Fatalf("Hover failed: %v", err)
			}

			if tc.WantEmpty {
				if result != nil && result.Contents.Value != "" {
					t.Errorf("Expected empty hover, got: %q", result.Contents.Value)
				}
				return
			}

			if result == nil {
				t.Fatalf("Expected hover result, got nil")
			}

			if tc.WantContain != "" && !strings.Contains(result.Contents.Value, tc.WantContain) {
				t.Errorf("Hover content %q does not contain %q", result.Contents.Value, tc.WantContain)
			}
		})
	}
}
