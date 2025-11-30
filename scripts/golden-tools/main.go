// Package main provides golden test utilities for the Dingo project.
//
// Usage:
//
//	go run ./scripts/golden-tools <command> [options]
//
// Commands:
//
//	regenerate <file.dingo>     Regenerate a golden test file
//	diff <test-output-file>     Visualize diffs from test output
//	perf <benchmark-output>     Track performance from benchmark output
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]
	args := os.Args[2:]

	switch command {
	case "regenerate":
		if err := runRegenerate(args); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "diff":
		if err := runDiff(args); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "perf":
		if err := runPerf(args); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`Golden Test Tools - Utilities for Dingo golden tests

Usage:
  go run ./scripts/golden-tools <command> [options]

Commands:
  regenerate <file.dingo>           Regenerate a golden test file from .dingo source
  diff <test-output-file>           Generate markdown diff report from test output
  perf <benchmark-output> [history] Track performance and detect regressions

Examples:
  go run ./scripts/golden-tools regenerate tests/golden/option_01_basic.dingo
  go run ./scripts/golden-tools diff test-output.txt > diff-report.md
  go run ./scripts/golden-tools perf benchmark.txt metrics.json`)
}
