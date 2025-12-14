// Package main provides a Go-based LSP test harness for dingo-lsp
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// Request represents a JSON-RPC request
type Request struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id,omitempty"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// Response represents a JSON-RPC response
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// LSPClient wraps an LSP process
type LSPClient struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader
}

// NewLSPClient starts the LSP server
func NewLSPClient(lspPath string) (*LSPClient, error) {
	cmd := exec.Command(lspPath, "--sqlite-logging", "--sqlite-log-path=/tmp/dingo-lsp-test.db")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	return &LSPClient{
		cmd:    cmd,
		stdin:  stdin,
		stdout: bufio.NewReader(stdout),
	}, nil
}

// Send sends a JSON-RPC request
func (c *LSPClient) Send(req Request) error {
	data, err := json.Marshal(req)
	if err != nil {
		return err
	}
	msg := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(data), data)
	_, err = c.stdin.Write([]byte(msg))
	return err
}

// ReceiveMessage reads a raw LSP message
func (c *LSPClient) ReceiveMessage() ([]byte, error) {
	// Read headers
	var contentLength int
	for {
		line, err := c.stdout.ReadString('\n')
		if err != nil {
			return nil, err
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

	// Read body
	body := make([]byte, contentLength)
	_, err := io.ReadFull(c.stdout, body)
	return body, err
}

// ReceiveResponse reads responses until we get one matching expectedID
// This skips notifications (messages without ID)
func (c *LSPClient) ReceiveResponse(expectedID int) (*Response, error) {
	for {
		body, err := c.ReceiveMessage()
		if err != nil {
			return nil, err
		}

		var resp Response
		if err := json.Unmarshal(body, &resp); err != nil {
			return nil, err
		}

		// Check if this is a response (has ID) vs notification (no ID)
		if resp.ID == nil {
			// This is a notification - skip and continue
			continue
		}

		// Check if ID matches
		var respID int
		switch v := resp.ID.(type) {
		case float64:
			respID = int(v)
		case int:
			respID = v
		}

		if respID == expectedID {
			return &resp, nil
		}
		// Wrong ID - skip (shouldn't happen often)
	}
}

// Close terminates the LSP server
func (c *LSPClient) Close() {
	c.stdin.Close()
	c.cmd.Wait()
}

func main() {
	fmt.Println("=== Dingo LSP Feature Test ===")

	// Start LSP - use locally built binary
	client, err := NewLSPClient("./dingo-lsp")
	if err != nil {
		fmt.Printf("Failed to start LSP: %v\n", err)
		return
	}
	defer client.Close()

	// Test file path - use real example file
	testFile := "/Users/jack/mag/dingo/examples/01_error_propagation/http_handler.dingo"
	fileURI := "file://" + testFile

	// Test 1: Initialize
	fmt.Println("\n--- Test 1: Initialize ---")
	initParams := map[string]interface{}{
		"processId": os.Getpid(),
		"capabilities": map[string]interface{}{
			"textDocument": map[string]interface{}{
				"hover":      map[string]interface{}{"contentFormat": []string{"markdown", "plaintext"}},
				"completion": map[string]interface{}{"completionItem": map[string]interface{}{"snippetSupport": true}},
			},
		},
		"rootUri": "file:///Users/jack/mag/dingo",
	}
	if err := client.Send(Request{JSONRPC: "2.0", ID: 1, Method: "initialize", Params: initParams}); err != nil {
		fmt.Printf("Send error: %v\n", err)
		return
	}

	resp, err := client.ReceiveResponse(1)
	if err != nil {
		fmt.Printf("Receive error: %v\n", err)
		return
	}
	if resp.Error != nil {
		fmt.Printf("❌ Initialize FAILED: %s\n", resp.Error.Message)
		return
	}
	fmt.Println("✓ Initialize OK")

	// Send initialized notification
	client.Send(Request{JSONRPC: "2.0", Method: "initialized", Params: map[string]interface{}{}})

	// Test 2: didOpen
	fmt.Println("\n--- Test 2: textDocument/didOpen ---")
	content, _ := os.ReadFile(testFile)
	didOpenParams := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri":        fileURI,
			"languageId": "dingo",
			"version":    1,
			"text":       string(content),
		},
	}
	if err := client.Send(Request{JSONRPC: "2.0", Method: "textDocument/didOpen", Params: didOpenParams}); err != nil {
		fmt.Printf("Send error: %v\n", err)
		return
	}
	fmt.Println("✓ didOpen sent")

	// Wait for diagnostics to be published
	time.Sleep(2 * time.Second)

	// Test 3: Hover
	fmt.Println("\n--- Test 3: textDocument/hover ---")
	// Hover on "GetUserHandler" function definition (line 52, 0-indexed 51)
	hoverParams := map[string]interface{}{
		"textDocument": map[string]interface{}{"uri": fileURI},
		"position":     map[string]interface{}{"line": 51, "character": 5}, // "GetUserHandler" function
	}
	if err := client.Send(Request{JSONRPC: "2.0", ID: 2, Method: "textDocument/hover", Params: hoverParams}); err != nil {
		fmt.Printf("Send error: %v\n", err)
		return
	}

	hoverResp, err := client.ReceiveResponse(2)
	if err != nil {
		fmt.Printf("Receive error: %v\n", err)
		return
	}
	if hoverResp.Error != nil {
		fmt.Printf("❌ Hover FAILED: %s\n", hoverResp.Error.Message)
	} else {
		// Truncate output for display
		result := string(hoverResp.Result)
		if len(result) > 100 {
			result = result[:100] + "..."
		}
		fmt.Printf("✓ Hover OK: %s\n", result)
	}

	// Test 4: Completion
	fmt.Println("\n--- Test 4: textDocument/completion ---")
	// Request completion inside main() function (line 140, 0-indexed 139)
	completionParams := map[string]interface{}{
		"textDocument": map[string]interface{}{"uri": fileURI},
		"position":     map[string]interface{}{"line": 139, "character": 2}, // Inside main()
	}
	if err := client.Send(Request{JSONRPC: "2.0", ID: 3, Method: "textDocument/completion", Params: completionParams}); err != nil {
		fmt.Printf("Send error: %v\n", err)
		return
	}

	compResp, err := client.ReceiveResponse(3)
	if err != nil {
		fmt.Printf("Receive error: %v\n", err)
		return
	}
	if compResp.Error != nil {
		fmt.Printf("❌ Completion FAILED: %s\n", compResp.Error.Message)
	} else {
		result := string(compResp.Result)
		if len(result) > 100 {
			result = result[:100] + "..."
		}
		fmt.Printf("✓ Completion OK: %s\n", result)
	}

	// Test 5: Definition
	fmt.Println("\n--- Test 5: textDocument/definition ---")
	// Go to definition of "extractUserID" call (line 55, 0-indexed 54)
	defParams := map[string]interface{}{
		"textDocument": map[string]interface{}{"uri": fileURI},
		"position":     map[string]interface{}{"line": 54, "character": 12}, // extractUserID call
	}
	if err := client.Send(Request{JSONRPC: "2.0", ID: 4, Method: "textDocument/definition", Params: defParams}); err != nil {
		fmt.Printf("Send error: %v\n", err)
		return
	}

	defResp, err := client.ReceiveResponse(4)
	if err != nil {
		fmt.Printf("Receive error: %v\n", err)
		return
	}
	if defResp.Error != nil {
		fmt.Printf("❌ Definition FAILED: %s\n", defResp.Error.Message)
	} else {
		result := string(defResp.Result)
		if len(result) > 100 {
			result = result[:100] + "..."
		}
		fmt.Printf("✓ Definition OK: %s\n", result)
	}

	// Test 6: Shutdown
	fmt.Println("\n--- Test 6: Shutdown ---")
	fmt.Println("✓ Shutting down...")

	fmt.Println("\n=== Test Complete ===")
}
