//go:build ignore

// diff-visualizer generates markdown visualization of golden test failures
package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("# Golden Test Diff Report")
		fmt.Println("\nNo test output file provided.")
		return
	}

	file, err := os.Open(os.Args[1])
	if err != nil {
		fmt.Printf("# Golden Test Diff Report\n\nError: %v\n", err)
		return
	}
	defer file.Close()

	fmt.Println("# Golden Test Diff Report")
	fmt.Println()

	scanner := bufio.NewScanner(file)
	var failures []string
	var currentFailure strings.Builder
	inFailure := false

	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "--- FAIL:") {
			if inFailure && currentFailure.Len() > 0 {
				failures = append(failures, currentFailure.String())
				currentFailure.Reset()
			}
			inFailure = true
			currentFailure.WriteString("## " + line + "\n\n```\n")
		} else if inFailure {
			if strings.HasPrefix(line, "FAIL") || strings.HasPrefix(line, "ok ") {
				currentFailure.WriteString("```\n")
				failures = append(failures, currentFailure.String())
				currentFailure.Reset()
				inFailure = false
			} else {
				currentFailure.WriteString(line + "\n")
			}
		}
	}

	if currentFailure.Len() > 0 {
		currentFailure.WriteString("```\n")
		failures = append(failures, currentFailure.String())
	}

	if len(failures) == 0 {
		fmt.Println("No test failures found in output.")
		return
	}

	fmt.Printf("Found %d test failure(s):\n\n", len(failures))
	for _, f := range failures {
		fmt.Println(f)
	}
}
