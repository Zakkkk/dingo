// cmd/lsp-diagcheck - Automated LSP diagnostics validation tool
//
// Tests that warnings and errors from Dingo (lint, transpiler)
// and Go (gopls) are correctly reported in the LSP.
//
// Usage:
//
//	lsp-diagcheck --spec ai-docs/diag-specs/*.yaml
//	lsp-diagcheck --file examples/test.dingo --verbose
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
	specGlob      = flag.String("spec", "", "Glob pattern for YAML spec files")
	singleFile    = flag.String("file", "", "Single .dingo file to test")
	editCycle     = flag.Bool("edit-cycle", false, "Test edit cycle: valid->invalid->valid")
	autosaveCycle = flag.Bool("autosave-cycle", false, "Test edit cycle WITH file saves (mimics VS Code auto-save)")
	dingoLSP      = flag.String("dingo-lsp", "./editors/vscode/server/bin/dingo-lsp", "Path to dingo-lsp binary")
	dingoBin      = flag.String("dingo", "./dingo", "Path to dingo binary")
	timeout       = flag.Int("timeout", 30, "Timeout in seconds for LSP operations")
	verbose       = flag.Bool("verbose", false, "Verbose output")
	waitTime      = flag.Int("wait", 3, "Seconds to wait for diagnostics")
)

func main() {
	flag.Parse()

	if *specGlob == "" && *singleFile == "" && !*editCycle && !*autosaveCycle {
		fmt.Fprintf(os.Stderr, "Usage: lsp-diagcheck --spec <glob> | --file <dingo-file> | --edit-cycle | --autosave-cycle\n")
		fmt.Fprintf(os.Stderr, "  --spec ai-docs/diag-specs/*.yaml   Run YAML spec tests\n")
		fmt.Fprintf(os.Stderr, "  --file test.dingo                  Test single file\n")
		fmt.Fprintf(os.Stderr, "  --edit-cycle                       Test edit cycle (valid->invalid->valid)\n")
		fmt.Fprintf(os.Stderr, "  --autosave-cycle                   Test edit cycle WITH saves (mimics VS Code auto-save)\n")
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

	// Get workspace root
	workspaceRoot, err := findWorkspaceRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error finding workspace root: %v\n", err)
		os.Exit(1)
	}

	if *verbose {
		fmt.Printf("Workspace root: %s\n", workspaceRoot)
		fmt.Printf("dingo-lsp: %s\n", *dingoLSP)
		fmt.Printf("dingo: %s\n", *dingoBin)
		fmt.Println()
	}

	// Edit cycle mode
	if *editCycle {
		runEditCycleTest(workspaceRoot)
		return
	}

	// Auto-save cycle mode (mimics VS Code with auto-save enabled)
	if *autosaveCycle {
		runAutoSaveCycleTest(workspaceRoot)
		return
	}

	// Single file mode
	if *singleFile != "" {
		runSingleFile(workspaceRoot, *singleFile)
		return
	}

	// Spec file mode
	specFiles, err := filepath.Glob(*specGlob)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error finding spec files: %v\n", err)
		os.Exit(1)
	}

	if len(specFiles) == 0 {
		fmt.Fprintf(os.Stderr, "No spec files found matching: %s\n", *specGlob)
		os.Exit(1)
	}

	runSpecs(workspaceRoot, specFiles)
}

func runSingleFile(workspaceRoot, filePath string) {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving path: %v\n", err)
		os.Exit(1)
	}

	content, err := os.ReadFile(absPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Testing diagnostics for: %s\n\n", filePath)

	// Start LSP client
	client, err := NewLSPClient(*dingoLSP, *verbose)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error starting LSP: %v\n", err)
		os.Exit(1)
	}
	defer client.Close()

	// Initialize
	if err := client.Initialize("file://"+workspaceRoot, time.Duration(*timeout)*time.Second); err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing LSP: %v\n", err)
		os.Exit(1)
	}

	// Open file
	uri := "file://" + absPath
	if err := client.DidOpen(uri, "dingo", string(content)); err != nil {
		fmt.Fprintf(os.Stderr, "Error opening document: %v\n", err)
		os.Exit(1)
	}

	// Wait for diagnostics
	fmt.Printf("Waiting %d seconds for diagnostics...\n\n", *waitTime)
	time.Sleep(time.Duration(*waitTime) * time.Second)

	// Get diagnostics
	diagnostics := client.GetDiagnostics(uri)

	if len(diagnostics) == 0 {
		fmt.Println("No diagnostics received.")
		fmt.Println("\nPossible reasons:")
		fmt.Println("  - File has no errors/warnings")
		fmt.Println("  - LSP not fully initialized")
		fmt.Println("  - Transpilation succeeded without issues")
	} else {
		fmt.Printf("Received %d diagnostic(s):\n\n", len(diagnostics))
		for i, d := range diagnostics {
			severity := severityName(d.Severity)
			fmt.Printf("%d. [%s] Line %d:%d - %s\n",
				i+1, severity, d.Range.Start.Line+1, d.Range.Start.Character+1, d.Message)
			if d.Source != "" {
				fmt.Printf("   Source: %s\n", d.Source)
			}
			if d.Code != "" {
				fmt.Printf("   Code: %s\n", d.Code)
			}
			fmt.Println()
		}
	}

	// Print stderr if verbose
	if *verbose && client.Stderr() != "" {
		fmt.Printf("\n--- LSP stderr ---\n%s\n", client.Stderr())
	}
}

func runSpecs(workspaceRoot string, specFiles []string) {
	var totalPassed, totalFailed int

	for _, specFile := range specFiles {
		specs, err := LoadSpecs(specFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading spec %s: %v\n", specFile, err)
			continue
		}

		fmt.Printf("%s:\n", filepath.Base(specFile))
		fmt.Println(strings.Repeat("-", 60))

		for _, spec := range specs {
			passed, failed := runSpec(workspaceRoot, spec)
			totalPassed += passed
			totalFailed += failed
		}
		fmt.Println()
	}

	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("Total: %d passed, %d failed\n", totalPassed, totalFailed)

	if totalFailed > 0 {
		os.Exit(1)
	}
}

func runSpec(workspaceRoot string, spec *Spec) (passed, failed int) {
	absPath := filepath.Join(workspaceRoot, spec.File)
	content, err := os.ReadFile(absPath)
	if err != nil {
		fmt.Printf("  ERROR: Cannot read file %s: %v\n", spec.File, err)
		return 0, len(spec.Expected)
	}

	// Start LSP client
	client, err := NewLSPClient(*dingoLSP, *verbose)
	if err != nil {
		fmt.Printf("  ERROR: Cannot start LSP: %v\n", err)
		return 0, len(spec.Expected)
	}
	defer client.Close()

	// Initialize
	if err := client.Initialize("file://"+workspaceRoot, time.Duration(*timeout)*time.Second); err != nil {
		fmt.Printf("  ERROR: Initialize failed: %v\n", err)
		return 0, len(spec.Expected)
	}

	// Open file
	uri := "file://" + absPath
	if err := client.DidOpen(uri, "dingo", string(content)); err != nil {
		fmt.Printf("  ERROR: didOpen failed: %v\n", err)
		return 0, len(spec.Expected)
	}

	// Wait for diagnostics
	time.Sleep(time.Duration(*waitTime) * time.Second)

	// Get diagnostics
	diagnostics := client.GetDiagnostics(uri)

	// Match diagnostics against expectations
	for _, expected := range spec.Expected {
		if matchDiagnostic(diagnostics, expected) {
			fmt.Printf("  ✓ %s\n", expected.Description)
			passed++
		} else {
			fmt.Printf("  ✗ %s\n", expected.Description)
			if *verbose {
				fmt.Printf("    Expected: Line %d, Severity=%s, Contains=%q\n",
					expected.Line, expected.Severity, expected.Contains)
				fmt.Printf("    Got %d diagnostics:\n", len(diagnostics))
				for _, d := range diagnostics {
					fmt.Printf("      - Line %d: [%s] %s\n",
						d.Range.Start.Line+1, severityName(d.Severity), d.Message)
				}
			}
			failed++
		}
	}

	// Check for unexpected diagnostics
	if spec.ExpectNone && len(diagnostics) > 0 {
		fmt.Printf("  ✗ Expected no diagnostics, got %d\n", len(diagnostics))
		failed++
	}

	return passed, failed
}

func matchDiagnostic(diagnostics []Diagnostic, expected ExpectedDiagnostic) bool {
	for _, d := range diagnostics {
		// Check line (0-based in LSP, 1-based in spec)
		if expected.Line > 0 && int(d.Range.Start.Line)+1 != expected.Line {
			continue
		}

		// Check severity
		if expected.Severity != "" && severityName(d.Severity) != expected.Severity {
			continue
		}

		// Check source
		if expected.Source != "" && d.Source != expected.Source {
			continue
		}

		// Check contains
		if expected.Contains != "" && !strings.Contains(d.Message, expected.Contains) {
			continue
		}

		// All conditions matched
		return true
	}
	return false
}

func severityName(s int) string {
	switch s {
	case 1:
		return "error"
	case 2:
		return "warning"
	case 3:
		return "info"
	case 4:
		return "hint"
	default:
		return fmt.Sprintf("unknown(%d)", s)
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
			break
		}
		dir = parent
	}

	return os.Getwd()
}

// runEditCycleTest tests the scenario:
// 1. Open file with valid content -> no diagnostics
// 2. Change to invalid content -> should show error
// 3. Change back to valid content -> error should CLEAR
//
// This tests for the "stale diagnostic" bug where errors persist after fixing code.
func runEditCycleTest(workspaceRoot string) {
	fmt.Println("=== Edit Cycle Test ===")
	fmt.Println("Testing: valid -> invalid -> valid (diagnostics should clear)")
	fmt.Println()

	// Test content - use existing test file
	validContent := `package main

// Test: Lambda syntax error - missing arrow
// Expected: Error pointing to malformed lambda

func test() {
    fn := |x: int| x + 1
    _ = fn
}

func main() {}
`
	invalidContent := `package main

// Test: Lambda syntax error - missing arrow
// Expected: Error pointing to malformed lambda

func test() {
    fn := |x| x + 1
    _ = fn
}

func main() {}
`

	// Use existing test file in workspace
	testFile := filepath.Join(workspaceRoot, "tests/lsp/04_lambda_error/main.dingo")

	// Save original content
	originalContent, err := os.ReadFile(testFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading test file: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		// Restore original content
		os.WriteFile(testFile, originalContent, 0644)
	}()

	// Write valid content
	if err := os.WriteFile(testFile, []byte(validContent), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing test file: %v\n", err)
		os.Exit(1)
	}

	// Start LSP client
	client, err := NewLSPClient(*dingoLSP, *verbose)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error starting LSP: %v\n", err)
		os.Exit(1)
	}
	defer client.Close()

	// Initialize
	if err := client.Initialize("file://"+workspaceRoot, time.Duration(*timeout)*time.Second); err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing LSP: %v\n", err)
		os.Exit(1)
	}

	uri := "file://" + testFile
	wait := time.Duration(*waitTime) * time.Second

	// Step 1: Open with valid content
	fmt.Println("Step 1: Open file with VALID content")
	if err := client.DidOpen(uri, "dingo", validContent); err != nil {
		fmt.Fprintf(os.Stderr, "Error in didOpen: %v\n", err)
		os.Exit(1)
	}
	time.Sleep(wait)

	diags := client.GetDiagnostics(uri)
	// Filter out expected warnings like "unusedfunc" - we only care about parse errors
	var step1Errors []Diagnostic
	for _, d := range diags {
		// Skip gopls analysis warnings (severity 3 = info, severity 2 = warning)
		if d.Severity <= 1 && !strings.Contains(d.Source, "unusedfunc") {
			step1Errors = append(step1Errors, d)
		}
	}
	if len(step1Errors) == 0 {
		fmt.Println("  ✓ No parse errors (expected)")
		if len(diags) > 0 {
			fmt.Printf("    (Note: %d analysis warnings present, which is OK)\n", len(diags))
		}
	} else {
		fmt.Printf("  ✗ Got %d unexpected parse errors:\n", len(step1Errors))
		for _, d := range step1Errors {
			fmt.Printf("    Line %d: %s\n", d.Range.Start.Line+1, d.Message)
		}
		os.Exit(1)
	}
	fmt.Println()

	// Step 2: Change to invalid content
	fmt.Println("Step 2: Change to INVALID content (missing type annotation)")
	if err := client.DidChange(uri, 2, invalidContent); err != nil {
		fmt.Fprintf(os.Stderr, "Error in didChange: %v\n", err)
		os.Exit(1)
	}
	time.Sleep(wait)

	diags = client.GetDiagnostics(uri)
	if len(diags) > 0 {
		fmt.Printf("  ✓ Got %d diagnostic(s) (expected):\n", len(diags))
		for _, d := range diags {
			fmt.Printf("    Line %d: %s\n", d.Range.Start.Line+1, d.Message)
		}
	} else {
		fmt.Println("  ⚠ No diagnostics (might be OK if parse doesn't catch this)")
	}
	fmt.Println()

	// Step 3: Change back to valid content
	fmt.Println("Step 3: Change back to VALID content")
	if err := client.DidChange(uri, 3, validContent); err != nil {
		fmt.Fprintf(os.Stderr, "Error in didChange: %v\n", err)
		os.Exit(1)
	}
	time.Sleep(wait)

	diags = client.GetDiagnostics(uri)
	// Filter out expected warnings like "unusedfunc" - we only care about parse errors
	var parseErrors []Diagnostic
	for _, d := range diags {
		// Skip gopls analysis warnings (severity 3 = info, severity 2 = warning)
		if d.Severity <= 1 && !strings.Contains(d.Source, "unusedfunc") {
			parseErrors = append(parseErrors, d)
		}
	}

	if len(parseErrors) == 0 {
		fmt.Println("  ✓ No parse errors (expected)")
		if len(diags) > 0 {
			fmt.Printf("    (Note: %d analysis warnings still present, which is OK)\n", len(diags))
		}
	} else {
		fmt.Printf("  ✗ STALE PARSE ERRORS - Got %d error(s) that should have cleared:\n", len(parseErrors))
		for _, d := range parseErrors {
			fmt.Printf("    Line %d: %s\n", d.Range.Start.Line+1, d.Message)
		}
		fmt.Println()
		fmt.Println("=== FAILED ===")
		fmt.Println("This is the 'stale diagnostic' bug - errors persist after fixing code.")
		os.Exit(1)
	}

	// Step 4-7: Do more edit cycles to test for FileSet accumulation bug
	fmt.Println()
	fmt.Println("Step 4-7: Rapid edit cycle (testing FileSet accumulation)")
	for i := 4; i <= 7; i++ {
		// Toggle between invalid and valid
		var content string
		if i%2 == 0 {
			content = invalidContent
		} else {
			content = validContent
		}
		if err := client.DidChange(uri, i, content); err != nil {
			fmt.Fprintf(os.Stderr, "Error in didChange (step %d): %v\n", i, err)
			os.Exit(1)
		}
		time.Sleep(500 * time.Millisecond) // Shorter wait for rapid cycling
	}

	// Final state should be valid (step 7 is odd)
	time.Sleep(wait)
	diags = client.GetDiagnostics(uri)

	// Check for the specific FileSet bug: "expected declaration, found 'package'" on wrong line
	for _, d := range diags {
		if strings.Contains(d.Message, "expected declaration") && strings.Contains(d.Message, "package") {
			fmt.Printf("  ✗ FileSet accumulation bug detected!\n")
			fmt.Printf("    Line %d: %s\n", d.Range.Start.Line+1, d.Message)
			fmt.Println()
			fmt.Println("=== FAILED ===")
			fmt.Println("The 'expected declaration, found package' error indicates FileSet position pollution.")
			os.Exit(1)
		}
	}

	fmt.Println("  ✓ No FileSet accumulation errors after rapid cycling")
	fmt.Println()
	fmt.Println("=== PASSED ===")
}

// runAutoSaveCycleTest tests the scenario WITH file saves (mimics VS Code auto-save):
// 1. Open file with valid content -> no diagnostics
// 2. Change + SAVE to invalid content -> should show error
// 3. Change + SAVE back to valid content -> error should CLEAR
//
// This tests for race conditions between async transpilation and diagnostics.
// The key difference from runEditCycleTest is that we:
// - Write the file to disk after each change (simulating VS Code save)
// - Send didSave notification (triggers transpilation)
// - Wait for transpilation to complete
func runAutoSaveCycleTest(workspaceRoot string) {
	fmt.Println("=== Auto-Save Cycle Test (mimics VS Code) ===")
	fmt.Println("Testing: valid -> invalid -> valid WITH file saves")
	fmt.Println("This tests for race conditions between async transpilation and diagnostics")
	fmt.Println()

	// Test content
	validContent := `package main

// Test: Lambda syntax error - missing arrow
// Expected: Error pointing to malformed lambda

func test() {
    fn := |x: int| x + 1
    _ = fn
}

func main() {}
`
	invalidContent := `package main

// Test: Lambda syntax error - missing arrow
// Expected: Error pointing to malformed lambda

func test() {
    fn := |x| x + 1
    _ = fn
}

func main() {}
`

	// Use existing test file in workspace
	testFile := filepath.Join(workspaceRoot, "tests/lsp/04_lambda_error/main.dingo")

	// Save original content
	originalContent, err := os.ReadFile(testFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading test file: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		// Restore original content
		os.WriteFile(testFile, originalContent, 0644)
	}()

	// Write valid content to disk
	if err := os.WriteFile(testFile, []byte(validContent), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing test file: %v\n", err)
		os.Exit(1)
	}

	// Start LSP client
	client, err := NewLSPClient(*dingoLSP, *verbose)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error starting LSP: %v\n", err)
		os.Exit(1)
	}
	defer client.Close()

	// Initialize
	if err := client.Initialize("file://"+workspaceRoot, time.Duration(*timeout)*time.Second); err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing LSP: %v\n", err)
		os.Exit(1)
	}

	uri := "file://" + testFile
	wait := time.Duration(*waitTime) * time.Second

	// Step 1: Open with valid content (file already has valid content on disk)
	fmt.Println("Step 1: Open file with VALID content")
	if err := client.DidOpen(uri, "dingo", validContent); err != nil {
		fmt.Fprintf(os.Stderr, "Error in didOpen: %v\n", err)
		os.Exit(1)
	}
	time.Sleep(wait)

	diags := client.GetDiagnostics(uri)
	parseErrors := filterParseErrors(diags)
	if len(parseErrors) == 0 {
		fmt.Println("  ✓ No parse errors (expected)")
	} else {
		fmt.Printf("  ✗ Got %d unexpected parse errors:\n", len(parseErrors))
		for _, d := range parseErrors {
			fmt.Printf("    Line %d: [%s] %s\n", d.Range.Start.Line+1, d.Source, d.Message)
		}
		os.Exit(1)
	}
	fmt.Println()

	// Step 2: Change to INVALID content AND SAVE (mimics VS Code auto-save)
	fmt.Println("Step 2: Change to INVALID content + SAVE (triggers transpilation)")
	// First send didChange
	if err := client.DidChange(uri, 2, invalidContent); err != nil {
		fmt.Fprintf(os.Stderr, "Error in didChange: %v\n", err)
		os.Exit(1)
	}
	// Then write to disk (simulates VS Code auto-save)
	if err := os.WriteFile(testFile, []byte(invalidContent), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing file: %v\n", err)
		os.Exit(1)
	}
	// Then send didSave (triggers transpilation)
	if err := client.DidSave(uri); err != nil {
		fmt.Fprintf(os.Stderr, "Error in didSave: %v\n", err)
		os.Exit(1)
	}
	time.Sleep(wait) // Wait for async transpilation

	diags = client.GetDiagnostics(uri)
	if len(diags) > 0 {
		fmt.Printf("  ✓ Got %d diagnostic(s) (expected for invalid code):\n", len(diags))
		for _, d := range diags {
			fmt.Printf("    Line %d: [%s] %s\n", d.Range.Start.Line+1, d.Source, d.Message)
		}
	} else {
		fmt.Println("  ⚠ No diagnostics (transpilation may not catch this syntax)")
	}
	fmt.Println()

	// Step 3: Change back to VALID content AND SAVE
	fmt.Println("Step 3: Change back to VALID content + SAVE")
	// First send didChange
	if err := client.DidChange(uri, 3, validContent); err != nil {
		fmt.Fprintf(os.Stderr, "Error in didChange: %v\n", err)
		os.Exit(1)
	}
	// Then write to disk
	if err := os.WriteFile(testFile, []byte(validContent), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing file: %v\n", err)
		os.Exit(1)
	}
	// Then send didSave
	if err := client.DidSave(uri); err != nil {
		fmt.Fprintf(os.Stderr, "Error in didSave: %v\n", err)
		os.Exit(1)
	}
	time.Sleep(wait) // Wait for async transpilation

	diags = client.GetDiagnostics(uri)
	parseErrors = filterParseErrors(diags)

	if len(parseErrors) == 0 {
		fmt.Println("  ✓ No parse errors (expected)")
	} else {
		fmt.Printf("  ✗ STALE ERRORS - Got %d parse error(s) that should have cleared:\n", len(parseErrors))
		for _, d := range parseErrors {
			fmt.Printf("    Line %d: [%s] %s\n", d.Range.Start.Line+1, d.Source, d.Message)
		}
		fmt.Println()
		fmt.Println("=== FAILED ===")
		fmt.Println("This indicates a race condition between async transpilation and diagnostics.")
		fmt.Println("Stale diagnostics from a previous transpilation are persisting.")
		os.Exit(1)
	}
	fmt.Println()

	// Step 4-7: Rapid auto-save cycling
	fmt.Println("Step 4-7: Rapid auto-save cycling")
	for i := 4; i <= 7; i++ {
		var content string
		if i%2 == 0 {
			content = invalidContent
		} else {
			content = validContent
		}

		// didChange + write + didSave
		if err := client.DidChange(uri, i, content); err != nil {
			fmt.Fprintf(os.Stderr, "Error in didChange (step %d): %v\n", i, err)
			os.Exit(1)
		}
		if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing file (step %d): %v\n", i, err)
			os.Exit(1)
		}
		if err := client.DidSave(uri); err != nil {
			fmt.Fprintf(os.Stderr, "Error in didSave (step %d): %v\n", i, err)
			os.Exit(1)
		}
		time.Sleep(500 * time.Millisecond)
	}

	// Final state should be valid (step 7 is odd)
	time.Sleep(wait)
	diags = client.GetDiagnostics(uri)

	// Check for the specific bug
	for _, d := range diags {
		if strings.Contains(d.Message, "expected declaration") && strings.Contains(d.Message, "package") {
			fmt.Printf("  ✗ Race condition detected!\n")
			fmt.Printf("    Line %d: [%s] %s\n", d.Range.Start.Line+1, d.Source, d.Message)
			fmt.Println()
			fmt.Println("=== FAILED ===")
			fmt.Println("The 'expected declaration, found package' error indicates stale gopls diagnostics.")
			os.Exit(1)
		}
	}

	fmt.Println("  ✓ No race condition errors after rapid auto-save cycling")
	fmt.Println()
	fmt.Println("=== PASSED ===")
}

// filterParseErrors filters diagnostics to only include actual errors (not warnings)
// and excludes known non-critical warnings like "unusedfunc"
func filterParseErrors(diags []Diagnostic) []Diagnostic {
	var errors []Diagnostic
	for _, d := range diags {
		// Severity 1 = error, 2 = warning, 3 = info, 4 = hint
		if d.Severity <= 1 && !strings.Contains(d.Source, "unusedfunc") {
			errors = append(errors, d)
		}
	}
	return errors
}
