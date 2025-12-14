package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Runner executes hover test specs
type Runner struct {
	DingoLSP      string
	WorkspaceRoot string
	Timeout       int
	Retries       int
	Verbose       bool
}

// RunSpec runs all test cases in a spec
func (r *Runner) RunSpec(spec *Spec) ([]CaseResult, error) {
	// Read the dingo file
	dingoPath := filepath.Join(r.WorkspaceRoot, spec.File)
	content, err := os.ReadFile(dingoPath)
	if err != nil {
		return nil, fmt.Errorf("reading dingo file: %w", err)
	}

	lines := strings.Split(string(content), "\n")

	// Start LSP client
	client, err := NewLSPClient(r.DingoLSP, r.Verbose)
	if err != nil {
		return nil, fmt.Errorf("starting LSP client: %w", err)
	}
	defer client.Close()

	// Initialize with proper file:// URI (3 slashes on Unix)
	rootURI := PathToURI(r.WorkspaceRoot)
	timeout := time.Duration(r.Timeout) * time.Second

	if err := client.Initialize(rootURI, timeout); err != nil {
		return nil, fmt.Errorf("initializing LSP: %w", err)
	}

	// Open the document with proper URI
	docURI := PathToURI(dingoPath)
	if err := client.DidOpen(docURI, "dingo", string(content)); err != nil {
		return nil, fmt.Errorf("opening document: %w", err)
	}

	// Wait for LSP to be ready (retry first hover until it works)
	if err := r.waitForReady(client, spec, lines, docURI, timeout); err != nil {
		return nil, fmt.Errorf("waiting for LSP ready: %w", err)
	}

	// Run each test case
	var results []CaseResult
	for _, tc := range spec.Cases {
		result := r.runCase(client, tc, lines, docURI, timeout)
		results = append(results, result)
	}

	// Print LSP stderr only if there were failures (for debugging)
	// In normal operation, suppress verbose stderr
	hasFailures := false
	for _, result := range results {
		if !result.Passed {
			hasFailures = true
			break
		}
	}
	if r.Verbose && hasFailures {
		if stderr := client.Stderr(); stderr != "" {
			fmt.Println("\n=== LSP Server Stderr (last 50 lines) ===")
			lines := strings.Split(stderr, "\n")
			start := 0
			if len(lines) > 50 {
				start = len(lines) - 50
			}
			for _, line := range lines[start:] {
				fmt.Println(line)
			}
			fmt.Println("=== End LSP Stderr ===\n")
		}
	}

	return results, nil
}

// waitForReady waits until the LSP server is ready to respond to hover requests
func (r *Runner) waitForReady(client *LSPClient, spec *Spec, lines []string, docURI string, timeout time.Duration) error {
	if len(spec.Cases) == 0 {
		return nil
	}

	// Find a case that expects real content (not allowAny) for better readiness detection
	var probeCase *TestCase
	for i := range spec.Cases {
		tc := &spec.Cases[i]
		// Prefer cases that expect actual content
		if tc.Expect.Contains != "" || len(tc.Expect.ContainsAny) > 0 {
			probeCase = tc
			break
		}
	}

	// Fall back to first case if no content-expecting case found
	if probeCase == nil {
		probeCase = &spec.Cases[0]
	}

	for i := 0; i < r.Retries; i++ {
		if r.Verbose {
			fmt.Printf("Waiting for LSP ready (attempt %d/%d, probing case %d: %s)...\n",
				i+1, r.Retries, probeCase.ID, probeCase.Token)
		}

		// Try hover on probe case
		lineIdx := probeCase.Line - 1
		if lineIdx < 0 || lineIdx >= len(lines) {
			return fmt.Errorf("line %d out of range", probeCase.Line)
		}

		col, err := FindTokenColumn(lines[lineIdx], probeCase.Token, probeCase.Occurrence)
		if err != nil {
			return fmt.Errorf("finding token: %w", err)
		}

		hoverText, err := client.Hover(docURI, lineIdx, col, timeout)
		if err == nil && hoverText != "" {
			if r.Verbose {
				fmt.Printf("LSP ready! Got hover response.\n")
			}
			return nil
		}

		if r.Verbose && err != nil {
			fmt.Printf("  Hover error: %v\n", err)
		}

		time.Sleep(200 * time.Millisecond)
	}

	// Don't fail - some hovers may legitimately be empty
	if r.Verbose {
		fmt.Printf("LSP may not be fully ready after %d retries, proceeding anyway...\n", r.Retries)
		if stderr := client.Stderr(); stderr != "" {
			fmt.Printf("LSP stderr:\n%s\n", stderr)
		}
	}
	return nil
}

// runCase runs a single test case and returns the result
func (r *Runner) runCase(client *LSPClient, tc TestCase, lines []string, docURI string, timeout time.Duration) CaseResult {
	result := CaseResult{
		ID:          tc.ID,
		Line:        tc.Line,
		Token:       tc.Token,
		Description: tc.Description,
	}

	// Validate line number
	lineIdx := tc.Line - 1
	if lineIdx < 0 || lineIdx >= len(lines) {
		result.Error = fmt.Sprintf("line %d out of range (file has %d lines)", tc.Line, len(lines))
		return result
	}

	lineText := lines[lineIdx]

	// Determine column: use explicit Character if set, otherwise find token
	var col int
	if tc.Character > 0 {
		// Use exact character position (already 0-based)
		col = tc.Character
	} else if tc.Token != "" {
		// Find token column
		var err error
		col, err = FindTokenColumn(lineText, tc.Token, tc.Occurrence)
		if err != nil {
			result.Error = err.Error()
			return result
		}
	} else {
		result.Error = "test case must specify either 'token' or 'character'"
		return result
	}
	result.Column = col

	// Request hover
	hoverText, err := client.Hover(docURI, lineIdx, col, timeout)
	if err != nil {
		result.Error = fmt.Sprintf("hover request failed: %v", err)
		return result
	}

	result.Got = hoverText

	// Check expectation
	passed, expected := tc.Expect.CheckExpectation(hoverText)
	result.Passed = passed
	result.Expected = expected

	return result
}

// buildDingoFile runs dingo build on a file
func buildDingoFile(dingoBin, dingoFile string) error {
	cmd := exec.Command(dingoBin, "build", "--no-mascot", dingoFile)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("dingo build failed: %v\n%s", err, string(output))
	}
	return nil
}
