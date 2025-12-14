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
	"time"
)

var (
	specGlob   = flag.String("spec", "ai-docs/hover-specs/*.yaml", "Glob pattern for spec files")
	dingoLSP   = flag.String("dingo-lsp", "./editors/vscode/server/bin/dingo-lsp", "Path to dingo-lsp binary")
	dingoBin   = flag.String("dingo", "./dingo", "Path to dingo binary")
	timeout    = flag.Int("timeout", 30, "Timeout in seconds for LSP operations")
	verbose    = flag.Bool("verbose", false, "Verbose output")
	jsonOutput = flag.Bool("json", false, "Output results as JSON")
	retries    = flag.Int("retries", 10, "Number of retries waiting for LSP ready")
	debugLine  = flag.String("debug-line", "", "Debug mode: show character positions for a line (format: file:linenum)")
	probeLine  = flag.String("probe", "", "Probe mode: test hover at each position (format: file:linenum or file:linenum:start-end)")
)

func main() {
	flag.Parse()

	// Debug mode: show character positions for a line
	if *debugLine != "" {
		debugLinePositions(*debugLine)
		return
	}

	// Probe mode: test hover at each position on a line
	if *probeLine != "" {
		probeLineHovers(*probeLine, *dingoLSP, *dingoBin, *timeout, *retries)
		return
	}

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

// probeLineHovers tests hover at each position on a line to find where results change
func probeLineHovers(spec, lspPath, dingoBin string, timeoutSec, retries int) {
	// Parse file:linenum or file:linenum:start-end
	parts := strings.Split(spec, ":")
	if len(parts) < 2 || len(parts) > 3 {
		fmt.Fprintf(os.Stderr, "Invalid format. Use: file:linenum or file:linenum:start-end\n")
		os.Exit(1)
	}

	filename := parts[0]
	lineNum := 0
	fmt.Sscanf(parts[1], "%d", &lineNum)
	if lineNum < 1 {
		fmt.Fprintf(os.Stderr, "Invalid line number: %s\n", parts[1])
		os.Exit(1)
	}

	startCol, endCol := 0, -1
	if len(parts) == 3 {
		rangeParts := strings.Split(parts[2], "-")
		if len(rangeParts) == 2 {
			fmt.Sscanf(rangeParts[0], "%d", &startCol)
			fmt.Sscanf(rangeParts[1], "%d", &endCol)
		}
	}

	// Read file
	content, err := os.ReadFile(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
		os.Exit(1)
	}

	lines := strings.Split(string(content), "\n")
	if lineNum > len(lines) {
		fmt.Fprintf(os.Stderr, "Line %d out of range\n", lineNum)
		os.Exit(1)
	}

	line := lines[lineNum-1]
	// Calculate visual width of line (with tab expansion)
	// VS Code sends visual columns, not character positions!
	visualWidth := getVisualWidth(line, 4)
	if endCol < 0 || endCol > visualWidth {
		endCol = visualWidth
	}

	// Build the dingo file first
	fmt.Printf("Building %s...\n", filename)
	if err := buildDingoFile(dingoBin, filename); err != nil {
		fmt.Fprintf(os.Stderr, "Build error: %v\n", err)
		os.Exit(1)
	}

	// Find workspace root
	workspaceRoot, err := findWorkspaceRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error finding workspace: %v\n", err)
		os.Exit(1)
	}

	// Start LSP
	client, err := NewLSPClient(lspPath, false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error starting LSP: %v\n", err)
		os.Exit(1)
	}
	defer client.Close()

	rootURI := PathToURI(workspaceRoot)
	timeout := time.Duration(timeoutSec) * time.Second

	if err := client.Initialize(rootURI, timeout); err != nil {
		fmt.Fprintf(os.Stderr, "Initialize error: %v\n", err)
		os.Exit(1)
	}

	absPath := filename
	if !filepath.IsAbs(filename) {
		absPath = filepath.Join(workspaceRoot, filename)
	}
	docURI := PathToURI(absPath)

	if err := client.DidOpen(docURI, "dingo", string(content)); err != nil {
		fmt.Fprintf(os.Stderr, "DidOpen error: %v\n", err)
		os.Exit(1)
	}

	// Wait for LSP to be ready
	time.Sleep(500 * time.Millisecond)

	fmt.Printf("\nLine %d: %q\n\n", lineNum, line)
	fmt.Printf("NOTE: Columns are VISUAL columns (tabs expand to 4 spaces), matching VS Code behavior.\n\n")
	fmt.Printf("%-5s %-4s %s\n", "VCol", "Char", "Hover Result (first 60 chars)")
	fmt.Printf("%-5s %-4s %s\n", "----", "----", "------------------------------")

	lineIdx := lineNum - 1
	lastHover := ""

	// Iterate visual columns (matching VS Code behavior)
	for visualCol := startCol; visualCol <= endCol && visualCol < visualWidth; visualCol++ {
		// Get the character at this visual column position
		char := "EOF"
		r, isVirtual := getCharAtVisualColumn(line, visualCol, 4)
		if r != 0 {
			if r == '\t' {
				if isVirtual {
					char = "·" // Show dot for virtual tab positions
				} else {
					char = "→" // Show arrow for actual tab character
				}
			} else {
				char = string(r)
			}
		}

		// Send visual column to LSP (this is what VS Code sends!)
		hover, err := client.Hover(docURI, lineIdx, visualCol, timeout)
		if err != nil {
			hover = fmt.Sprintf("ERROR: %v", err)
		}

		// Normalize for display
		hover = strings.ReplaceAll(hover, "\n", " ")
		hover = strings.ReplaceAll(hover, "```go", "")
		hover = strings.ReplaceAll(hover, "```", "")
		hover = strings.TrimSpace(hover)

		// Only print if hover changed
		if hover != lastHover {
			display := hover
			if len(display) > 60 {
				display = display[:57] + "..."
			}
			if display == "" {
				display = "(empty)"
			}
			fmt.Printf("%-5d %-4s %s\n", visualCol, char, display)
			lastHover = hover
		}
	}

	fmt.Printf("\nDone. Use 'character: <visual_col>' in YAML spec to test specific visual column positions.\n")
}

// debugLinePositions shows visual column positions for a line to help match VS Code
func debugLinePositions(spec string) {
	// Parse file:linenum
	parts := strings.Split(spec, ":")
	if len(parts) != 2 {
		fmt.Fprintf(os.Stderr, "Invalid format. Use: file:linenum (e.g., examples/01_error_propagation/http_handler.dingo:55)\n")
		os.Exit(1)
	}

	filename := parts[0]
	lineNum := 0
	fmt.Sscanf(parts[1], "%d", &lineNum)
	if lineNum < 1 {
		fmt.Fprintf(os.Stderr, "Invalid line number: %s\n", parts[1])
		os.Exit(1)
	}

	// Read file
	content, err := os.ReadFile(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
		os.Exit(1)
	}

	lines := strings.Split(string(content), "\n")
	if lineNum > len(lines) {
		fmt.Fprintf(os.Stderr, "Line %d out of range (file has %d lines)\n", lineNum, len(lines))
		os.Exit(1)
	}

	line := lines[lineNum-1]

	fmt.Printf("File: %s\n", filename)
	fmt.Printf("Line %d (0-indexed: %d):\n", lineNum, lineNum-1)
	fmt.Printf("\n")

	// Show the line with visual markers
	fmt.Printf("Content: %q\n", line)
	fmt.Printf("\n")

	// Show visual column positions (VS Code sends these!)
	fmt.Printf("Visual column positions (what VS Code sends):\n")
	fmt.Printf("%-8s %-6s %-6s %s\n", "VisCol", "Char", "Byte", "Character")
	fmt.Printf("%-8s %-6s %-6s %s\n", "------", "----", "----", "---------")

	bytePos := 0
	visualCol := 0
	tabSize := 4
	for i, r := range line {
		runeBytes := len(string(r))

		display := string(r)
		if r == '\t' {
			display = "→TAB"
		} else if r < 32 {
			display = fmt.Sprintf("\\x%02x", r)
		}

		fmt.Printf("%-8d %-6d %-6d %s\n", visualCol, i, bytePos, display)

		// Advance visual column
		if r == '\t' {
			visualCol = ((visualCol / tabSize) + 1) * tabSize
		} else {
			visualCol++
		}

		bytePos += runeBytes
	}

	fmt.Printf("\n")
	visualWidth := getVisualWidth(line, tabSize)
	fmt.Printf("Total: %d chars, %d bytes, %d visual columns (with tab size %d)\n",
		len([]rune(line)), len(line), visualWidth, tabSize)
	fmt.Printf("\n")
	fmt.Printf("To test a specific position, use:\n")
	fmt.Printf("  character: <visual_column>\n")
	fmt.Printf("in your YAML spec. VS Code sends visual columns, not character positions!\n")
}
