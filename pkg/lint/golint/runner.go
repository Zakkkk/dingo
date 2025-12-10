package golint

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Runner executes golangci-lint as a subprocess
type Runner struct {
	configPath string
	timeout    time.Duration
	workingDir string
}

// NewRunner creates a new golangci-lint runner
func NewRunner(configPath string, timeout time.Duration) *Runner {
	if timeout == 0 {
		timeout = 5 * time.Minute
	}
	return &Runner{
		configPath: configPath,
		timeout:    timeout,
	}
}

// NewRunnerWithDefaults creates a runner with default configuration
// It writes a temporary config file with sensible defaults
func NewRunnerWithDefaults(workingDir string) (*Runner, error) {
	cfg := DefaultConfig()

	// Create temporary config file
	tmpDir := filepath.Join(workingDir, ".dingo")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}

	configPath := filepath.Join(tmpDir, "golangci.yml")
	if err := cfg.WriteToFile(configPath); err != nil {
		return nil, fmt.Errorf("write default config: %w", err)
	}

	runner := NewRunner(configPath, 5*time.Minute)
	runner.workingDir = workingDir
	return runner, nil
}

// RunResult contains the output from golangci-lint
type RunResult struct {
	JSON     []byte
	ExitCode int
	Stderr   string
}

// Run executes golangci-lint on the specified Go files
// It returns the JSON output and any errors
// Note: Exit code 1 means issues were found (not an error)
func (r *Runner) Run(goFiles []string) (*RunResult, error) {
	if len(goFiles) == 0 {
		return &RunResult{JSON: []byte("{}"), ExitCode: 0}, nil
	}

	// Build command arguments
	args := []string{
		"run",
		"--out-format=json",
	}

	// Add config if specified
	if r.configPath != "" {
		args = append(args, "--config", r.configPath)
	}

	// Validate and sanitize file paths before passing to exec
	for _, path := range goFiles {
		// Prevent flag injection: ensure path doesn't start with -
		if strings.HasPrefix(filepath.Base(path), "-") {
			return nil, fmt.Errorf("invalid file path (starts with -): %s", path)
		}
		// Ensure path is a .go file
		if !strings.HasSuffix(path, ".go") {
			return nil, fmt.Errorf("not a Go file: %s", path)
		}
	}

	// Add files to lint
	args = append(args, goFiles...)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()

	// Create command with context
	cmd := exec.CommandContext(ctx, "golangci-lint", args...)
	if r.workingDir != "" {
		cmd.Dir = r.workingDir
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run command
	err := cmd.Run()

	result := &RunResult{
		JSON:   stdout.Bytes(),
		Stderr: stderr.String(),
	}

	if err != nil {
		// Check if timeout occurred
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("golangci-lint timed out after %s", r.timeout)
		}

		// Check exit code
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()

			// Exit code 1 means issues were found, not an error
			if result.ExitCode == 1 {
				return result, nil
			}

			// Other exit codes are actual errors
			return result, fmt.Errorf("golangci-lint failed with exit code %d: %s", result.ExitCode, stderr.String())
		}
		return result, fmt.Errorf("golangci-lint execution failed: %w", err)
	}

	return result, nil
}

// IsInstalled checks if golangci-lint is available in PATH
func IsInstalled() bool {
	_, err := exec.LookPath("golangci-lint")
	return err == nil
}

// Version returns the golangci-lint version string
func Version() (string, error) {
	cmd := exec.Command("golangci-lint", "--version")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("get golangci-lint version: %w", err)
	}
	return string(bytes.TrimSpace(output)), nil
}
