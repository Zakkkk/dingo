package typechecker

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/MadAppGang/dingo/pkg/config"
)

// GoplsClient manages a gopls subprocess for type inference fallback.
// Layer 4 of the lambda inference system - only used when configured.
type GoplsClient struct {
	config  *config.TypeInferenceConfig
	timeout time.Duration
	// TODO: Add subprocess management fields
	// cmd     *exec.Cmd
	// stdin   io.WriteCloser
	// stdout  io.ReadCloser
}

// NewGoplsClient creates a new gopls client.
// Starts the gopls subprocess and initializes LSP communication.
func NewGoplsClient(cfg *config.TypeInferenceConfig) (*GoplsClient, error) {
	if cfg == nil {
		return nil, fmt.Errorf("gopls client requires non-nil config")
	}

	// Parse timeout
	timeout, err := time.ParseDuration(cfg.GoplsTimeout)
	if err != nil {
		return nil, fmt.Errorf("invalid gopls_timeout: %w", err)
	}

	client := &GoplsClient{
		config:  cfg,
		timeout: timeout,
	}

	// Start gopls subprocess
	if err := client.start(); err != nil {
		return nil, fmt.Errorf("failed to start gopls: %w", err)
	}

	return client, nil
}

// start launches the gopls subprocess.
func (c *GoplsClient) start() error {
	// Determine gopls path
	goplsPath := c.config.GoplsPath
	if goplsPath == "" {
		// Use gopls from PATH
		path, err := exec.LookPath("gopls")
		if err != nil {
			return fmt.Errorf("gopls not found in PATH: %w", err)
		}
		goplsPath = path
	}

	// TODO: Start gopls with LSP mode
	// cmd := exec.Command(goplsPath, "serve")
	// Set up stdin/stdout pipes for LSP communication
	// Store cmd, stdin, stdout in client

	_ = goplsPath // Suppress unused warning for now

	return nil
}

// QueryType queries gopls for type information at a specific position.
// Returns the type information as a string (e.g., "func(User) string").
func (c *GoplsClient) QueryType(filename string, line, column int) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	// TODO: Send LSP textDocument/hover request
	// Format: {"jsonrpc":"2.0","method":"textDocument/hover","params":{"textDocument":{"uri":"file://..."},"position":{"line":...,"character":...}}}
	// Parse response and extract type information

	_ = ctx // Suppress unused warning for now

	// Placeholder implementation
	return "", fmt.Errorf("gopls client not fully implemented")
}

// Close shuts down the gopls subprocess.
func (c *GoplsClient) Close() error {
	// TODO: Send shutdown LSP request
	// Wait for process to exit
	// Close stdin/stdout pipes

	return nil
}
