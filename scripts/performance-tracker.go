//go:build ignore

// performance-tracker generates markdown performance reports from benchmark results
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

type BenchmarkResult struct {
	Name       string  `json:"name"`
	NsPerOp    float64 `json:"ns_per_op"`
	AllocsPerOp int64   `json:"allocs_per_op"`
	BytesPerOp int64   `json:"bytes_per_op"`
}

type Metrics struct {
	Benchmarks []BenchmarkResult `json:"benchmarks"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("# Performance Benchmark Report")
		fmt.Println("\nNo benchmark file provided.")
		return
	}

	benchFile := os.Args[1]
	var previousMetrics *Metrics

	if len(os.Args) >= 3 {
		data, err := os.ReadFile(os.Args[2])
		if err == nil {
			previousMetrics = &Metrics{}
			json.Unmarshal(data, previousMetrics)
		}
	}

	results, err := parseBenchmarks(benchFile)
	if err != nil {
		fmt.Printf("# Performance Benchmark Report\n\nError: %v\n", err)
		return
	}

	// Save current metrics
	metrics := Metrics{Benchmarks: results}
	data, _ := json.MarshalIndent(metrics, "", "  ")
	os.WriteFile("metrics.json", data, 0644)

	// Generate report
	fmt.Println("# Performance Benchmark Report")
	fmt.Println()
	fmt.Printf("Analyzed %d benchmark(s)\n\n", len(results))

	if len(results) == 0 {
		fmt.Println("No benchmarks found in output.")
		return
	}

	fmt.Println("| Benchmark | ns/op | allocs/op | bytes/op |")
	fmt.Println("|-----------|-------|-----------|----------|")

	for _, r := range results {
		fmt.Printf("| %s | %.2f | %d | %d |\n", r.Name, r.NsPerOp, r.AllocsPerOp, r.BytesPerOp)
	}

	if previousMetrics != nil && len(previousMetrics.Benchmarks) > 0 {
		fmt.Println("\n## Comparison with Previous Run")
		fmt.Println("\n_Previous metrics available for comparison._")
	}
}

func parseBenchmarks(filename string) ([]BenchmarkResult, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var results []BenchmarkResult
	// Match: BenchmarkName-N    	    1234	   5678 ns/op	    123 B/op	      4 allocs/op
	benchRe := regexp.MustCompile(`^(Benchmark\S+)\s+\d+\s+([\d.]+)\s+ns/op(?:\s+(\d+)\s+B/op)?(?:\s+(\d+)\s+allocs/op)?`)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		matches := benchRe.FindStringSubmatch(line)
		if len(matches) >= 3 {
			nsPerOp, _ := strconv.ParseFloat(matches[2], 64)
			var bytesPerOp, allocsPerOp int64
			if len(matches) >= 4 && matches[3] != "" {
				bytesPerOp, _ = strconv.ParseInt(matches[3], 10, 64)
			}
			if len(matches) >= 5 && matches[4] != "" {
				allocsPerOp, _ = strconv.ParseInt(matches[4], 10, 64)
			}

			// Clean up benchmark name (remove -N suffix)
			name := matches[1]
			if idx := strings.LastIndex(name, "-"); idx > 0 {
				name = name[:idx]
			}

			results = append(results, BenchmarkResult{
				Name:        name,
				NsPerOp:     nsPerOp,
				BytesPerOp:  bytesPerOp,
				AllocsPerOp: allocsPerOp,
			})
		}
	}

	return results, scanner.Err()
}
