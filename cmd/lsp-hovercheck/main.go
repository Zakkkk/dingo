// cmd/lsp-hovercheck - Automated LSP hover validation tool
//
// Replaces manual VS Code hover checks with headless testing.
// Reads YAML spec files, starts dingo-lsp, sends hover requests,
// and compares results against expectations.
//
// Usage:
//
//	lsp-hovercheck --spec ai-docs/hover-specs/*.yaml
//	lsp-hovercheck --spec ai-docs/hover-specs/http_handler.yaml --verbose
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var (
	specGlob   = flag.String("spec", "ai-docs/hover-specs/*.yaml", "Glob pattern for spec files")
	dingoLSP   = flag.String("dingo-lsp", "./editors/vscode/server/bin/dingo-lsp", "Path to dingo-lsp binary")
	dingoBin   = flag.String("dingo", "./dingo", "Path to dingo binary")
	timeout    = flag.Int("timeout", 30, "Timeout in seconds for LSP operations")
	verbose    = flag.Bool("verbose", false, "Verbose output")
	jsonOutput = flag.Bool("json", false, "Output results as JSON")
	retries    = flag.Int("retries", 10, "Number of retries waiting for LSP ready")
)

func main() {
	flag.Parse()

	// Find spec files
	specFiles, err := filepath.Glob(*specGlob)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error finding spec files: %v\n", err)
		os.Exit(1)
	}

	if len(specFiles) == 0 {
		fmt.Fprintf(os.Stderr, "No spec files found matching: %s\n", *specGlob)
		os.Exit(1)
	}

	// Verify binaries exist
	if _, err := os.Stat(*dingoLSP); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "dingo-lsp not found at: %s\n", *dingoLSP)
		fmt.Fprintf(os.Stderr, "Build it with: go build -o %s ./cmd/dingo-lsp\n", *dingoLSP)
		os.Exit(1)
	}

	if _, err := os.Stat(*dingoBin); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "dingo not found at: %s\n", *dingoBin)
		fmt.Fprintf(os.Stderr, "Build it with: go build -o %s ./cmd/dingo\n", *dingoBin)
		os.Exit(1)
	}

	// Get workspace root (directory containing go.mod)
	workspaceRoot, err := findWorkspaceRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error finding workspace root: %v\n", err)
		os.Exit(1)
	}

	if *verbose {
		fmt.Printf("Workspace root: %s\n", workspaceRoot)
		fmt.Printf("Spec files: %v\n", specFiles)
		fmt.Printf("dingo-lsp: %s\n", *dingoLSP)
		fmt.Printf("dingo: %s\n", *dingoBin)
		fmt.Println()
	}

	// Process each spec file
	var allResults []CaseResult
	var totalPassed, totalFailed int

	for _, specFile := range specFiles {
		if *verbose {
			fmt.Printf("=== Processing %s ===\n", specFile)
		}

		spec, err := LoadSpec(specFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading spec %s: %v\n", specFile, err)
			continue
		}

		// Build the dingo file
		dingoFile := filepath.Join(workspaceRoot, spec.File)
		if *verbose {
			fmt.Printf("Building: %s\n", dingoFile)
		}

		if err := buildDingoFile(*dingoBin, dingoFile); err != nil {
			fmt.Fprintf(os.Stderr, "Error building %s: %v\n", dingoFile, err)
			continue
		}

		// Run hover checks
		runner := &Runner{
			DingoLSP:      *dingoLSP,
			WorkspaceRoot: workspaceRoot,
			Timeout:       *timeout,
			Retries:       *retries,
			Verbose:       *verbose,
		}

		results, err := runner.RunSpec(spec)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error running spec %s: %v\n", specFile, err)
			continue
		}

		allResults = append(allResults, results...)

		// Print results for this spec
		if !*jsonOutput {
			fmt.Printf("\n%s:\n", filepath.Base(specFile))
			fmt.Println(strings.Repeat("-", 60))
		}

		for _, r := range results {
			if r.Passed {
				totalPassed++
				if !*jsonOutput {
					fmt.Printf("%d: works\n", r.ID)
				}
			} else {
				totalFailed++
				if !*jsonOutput {
					if r.Error != "" {
						fmt.Printf("%d: error - %s\n", r.ID, r.Error)
					} else {
						fmt.Printf("%d: expected %q, got %q\n", r.ID, r.Expected, truncate(r.Got, 60))
					}
				}
			}

			if *verbose && !*jsonOutput {
				fmt.Printf("    Line %d, token %q, col %d\n", r.Line, r.Token, r.Column)
				if r.Got != "" {
					fmt.Printf("    Hover: %s\n", truncate(r.Got, 100))
				}
			}
		}
	}

	// Print summary
	if !*jsonOutput {
		fmt.Println()
		fmt.Println(strings.Repeat("=", 60))
		fmt.Printf("Total: %d passed, %d failed\n", totalPassed, totalFailed)

		if totalFailed > 0 {
			os.Exit(1)
		}
	} else {
		// JSON output
		printJSONResults(allResults)
	}
}

func findWorkspaceRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found in any parent directory")
		}
		dir = parent
	}
}

func truncate(s string, maxLen int) string {
	// Remove newlines for display
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")

	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func printJSONResults(results []CaseResult) {
	fmt.Println("[")
	for i, r := range results {
		comma := ","
		if i == len(results)-1 {
			comma = ""
		}
		fmt.Printf(`  {"id": %d, "passed": %v, "got": %q, "expected": %q}%s`+"\n",
			r.ID, r.Passed, r.Got, r.Expected, comma)
	}
	fmt.Println("]")
}
