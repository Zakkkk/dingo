package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// PerformanceTracker analyzes benchmark results and tracks performance trends
type PerformanceTracker struct {
	results []BenchmarkResult
	config  TrackerConfig
}

// TrackerConfig holds configuration for performance tracking
type TrackerConfig struct {
	OutputFile      string
	HistoryFile     string
	RegressionLimit float64 // Percentage threshold for regression (e.g., 10.0 = 10%)
}

// BenchmarkResult represents a single benchmark measurement
type BenchmarkResult struct {
	Name        string  `json:"name"`
	Iterations  int64   `json:"iterations"`
	NsPerOp     float64 `json:"ns_per_op"`
	BytesPerOp  int64   `json:"bytes_per_op,omitempty"`
	AllocsPerOp int64   `json:"allocs_per_op,omitempty"`
	MBPerSec    float64 `json:"mb_per_sec,omitempty"`
	Timestamp   string  `json:"timestamp"`
	GitCommit   string  `json:"git_commit,omitempty"`
	GitBranch   string  `json:"git_branch,omitempty"`
}

// PerformanceReport contains analysis of benchmark results
type PerformanceReport struct {
	Timestamp   string            `json:"timestamp"`
	GitCommit   string            `json:"git_commit"`
	GitBranch   string            `json:"git_branch"`
	Results     []BenchmarkResult `json:"results"`
	Summary     Summary           `json:"summary"`
	Regressions []Regression      `json:"regressions,omitempty"`
}

// Summary provides aggregate statistics
type Summary struct {
	TotalBenchmarks int     `json:"total_benchmarks"`
	AvgNsPerOp      float64 `json:"avg_ns_per_op"`
	AvgAllocsPerOp  float64 `json:"avg_allocs_per_op"`
	TotalMemoryMB   float64 `json:"total_memory_mb"`
}

// Regression represents a performance regression
type Regression struct {
	BenchmarkName string  `json:"benchmark_name"`
	Metric        string  `json:"metric"`
	OldValue      float64 `json:"old_value"`
	NewValue      float64 `json:"new_value"`
	ChangePercent float64 `json:"change_percent"`
	Severity      string  `json:"severity"` // "warning" or "critical"
}

var (
	// Benchmark output pattern: BenchmarkName-N    iterations    ns/op    B/op    allocs/op
	benchPattern = regexp.MustCompile(`^Benchmark(\S+)-\d+\s+(\d+)\s+([\d.]+)\s+ns/op(?:\s+([\d.]+)\s+B/op)?(?:\s+([\d.]+)\s+allocs/op)?(?:\s+([\d.]+)\s+MB/s)?`)
)

func runPerf(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: perf <benchmark-output-file> [history-file]")
	}

	config := TrackerConfig{
		OutputFile:      "metrics.json",
		HistoryFile:     "",
		RegressionLimit: 10.0, // 10% regression threshold
	}

	if len(args) >= 2 {
		config.HistoryFile = args[1]
	}

	file, err := os.Open(args[0])
	if err != nil {
		return fmt.Errorf("error opening file: %w", err)
	}
	defer file.Close()

	tracker := NewPerformanceTracker(config)
	if err := tracker.Parse(file); err != nil {
		return fmt.Errorf("error parsing benchmark output: %w", err)
	}

	report := tracker.GenerateReport()

	// Load historical data if available
	if config.HistoryFile != "" {
		if err := tracker.CompareWithHistory(config.HistoryFile, &report); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Could not compare with history: %v\n", err)
		}
	}

	// Output JSON report
	if err := writeJSON(config.OutputFile, report); err != nil {
		return fmt.Errorf("error writing JSON: %w", err)
	}

	// Print human-readable summary
	printSummary(report)

	// Exit with non-zero if regressions found
	if len(report.Regressions) > 0 {
		return fmt.Errorf("performance regressions detected")
	}

	return nil
}

// NewPerformanceTracker creates a new tracker instance
func NewPerformanceTracker(config TrackerConfig) *PerformanceTracker {
	return &PerformanceTracker{
		results: make([]BenchmarkResult, 0),
		config:  config,
	}
}

// Parse extracts benchmark results from test output
func (pt *PerformanceTracker) Parse(reader io.Reader) error {
	timestamp := time.Now().Format(time.RFC3339)
	gitCommit := getGitCommit()
	gitBranch := getGitBranch()

	scanner := NewLineScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()

		if matches := benchPattern.FindStringSubmatch(line); matches != nil {
			result := BenchmarkResult{
				Name:      matches[1],
				Timestamp: timestamp,
				GitCommit: gitCommit,
				GitBranch: gitBranch,
			}

			// Parse iterations
			if iterations, err := strconv.ParseInt(matches[2], 10, 64); err == nil {
				result.Iterations = iterations
			}

			// Parse ns/op
			if nsPerOp, err := strconv.ParseFloat(matches[3], 64); err == nil {
				result.NsPerOp = nsPerOp
			}

			// Parse B/op (optional)
			if len(matches) > 4 && matches[4] != "" {
				if bytesPerOp, err := strconv.ParseFloat(matches[4], 64); err == nil {
					result.BytesPerOp = int64(bytesPerOp)
				}
			}

			// Parse allocs/op (optional)
			if len(matches) > 5 && matches[5] != "" {
				if allocsPerOp, err := strconv.ParseFloat(matches[5], 64); err == nil {
					result.AllocsPerOp = int64(allocsPerOp)
				}
			}

			// Parse MB/s (optional)
			if len(matches) > 6 && matches[6] != "" {
				if mbPerSec, err := strconv.ParseFloat(matches[6], 64); err == nil {
					result.MBPerSec = mbPerSec
				}
			}

			pt.results = append(pt.results, result)
		}
	}

	return scanner.Err()
}

// GenerateReport creates a performance report from parsed results
func (pt *PerformanceTracker) GenerateReport() PerformanceReport {
	report := PerformanceReport{
		Timestamp: time.Now().Format(time.RFC3339),
		GitCommit: getGitCommit(),
		GitBranch: getGitBranch(),
		Results:   pt.results,
		Summary:   pt.calculateSummary(),
	}

	return report
}

// calculateSummary computes aggregate statistics
func (pt *PerformanceTracker) calculateSummary() Summary {
	if len(pt.results) == 0 {
		return Summary{}
	}

	var totalNs, totalAllocs, totalMemoryBytes float64
	for _, result := range pt.results {
		totalNs += result.NsPerOp
		totalAllocs += float64(result.AllocsPerOp)
		totalMemoryBytes += float64(result.BytesPerOp)
	}

	count := float64(len(pt.results))
	return Summary{
		TotalBenchmarks: len(pt.results),
		AvgNsPerOp:      totalNs / count,
		AvgAllocsPerOp:  totalAllocs / count,
		TotalMemoryMB:   totalMemoryBytes / (1024 * 1024),
	}
}

// CompareWithHistory compares current results with historical data
func (pt *PerformanceTracker) CompareWithHistory(historyFile string, report *PerformanceReport) error {
	data, err := os.ReadFile(historyFile)
	if err != nil {
		return err
	}

	var historical PerformanceReport
	if err := json.Unmarshal(data, &historical); err != nil {
		return err
	}

	// Build map of historical results
	histMap := make(map[string]BenchmarkResult)
	for _, result := range historical.Results {
		histMap[result.Name] = result
	}

	// Compare each current result with historical
	for _, current := range pt.results {
		if hist, exists := histMap[current.Name]; exists {
			// Check for regressions in ns/op
			if current.NsPerOp > hist.NsPerOp {
				changePercent := ((current.NsPerOp - hist.NsPerOp) / hist.NsPerOp) * 100
				if changePercent > pt.config.RegressionLimit {
					severity := "warning"
					if changePercent > pt.config.RegressionLimit*2 {
						severity = "critical"
					}

					report.Regressions = append(report.Regressions, Regression{
						BenchmarkName: current.Name,
						Metric:        "ns/op",
						OldValue:      hist.NsPerOp,
						NewValue:      current.NsPerOp,
						ChangePercent: changePercent,
						Severity:      severity,
					})
				}
			}

			// Check for memory regressions
			if current.BytesPerOp > hist.BytesPerOp && hist.BytesPerOp > 0 {
				changePercent := ((float64(current.BytesPerOp) - float64(hist.BytesPerOp)) / float64(hist.BytesPerOp)) * 100
				if changePercent > pt.config.RegressionLimit {
					severity := "warning"
					if changePercent > pt.config.RegressionLimit*2 {
						severity = "critical"
					}

					report.Regressions = append(report.Regressions, Regression{
						BenchmarkName: current.Name,
						Metric:        "B/op",
						OldValue:      float64(hist.BytesPerOp),
						NewValue:      float64(current.BytesPerOp),
						ChangePercent: changePercent,
						Severity:      severity,
					})
				}
			}
		}
	}

	return nil
}

// writeJSON writes a report to a JSON file
func writeJSON(filename string, report PerformanceReport) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filename, data, 0644)
}

// printSummary prints a human-readable summary to stdout
func printSummary(report PerformanceReport) {
	fmt.Println("# Performance Benchmark Report")
	fmt.Println()
	fmt.Printf("**Timestamp**: %s\n", report.Timestamp)
	fmt.Printf("**Git Commit**: %s\n", report.GitCommit)
	fmt.Printf("**Git Branch**: %s\n", report.GitBranch)
	fmt.Println()

	fmt.Println("## Summary")
	fmt.Println()
	fmt.Printf("- **Total Benchmarks**: %d\n", report.Summary.TotalBenchmarks)
	fmt.Printf("- **Average ns/op**: %.2f\n", report.Summary.AvgNsPerOp)
	fmt.Printf("- **Average allocs/op**: %.2f\n", report.Summary.AvgAllocsPerOp)
	fmt.Printf("- **Total Memory**: %.2f MB\n", report.Summary.TotalMemoryMB)
	fmt.Println()

	if len(report.Regressions) > 0 {
		fmt.Println("## Performance Regressions Detected")
		fmt.Println()
		fmt.Println("| Benchmark | Metric | Old Value | New Value | Change | Severity |")
		fmt.Println("|-----------|--------|-----------|-----------|--------|----------|")

		for _, reg := range report.Regressions {
			icon := "WARNING"
			if reg.Severity == "critical" {
				icon = "CRITICAL"
			}

			fmt.Printf("| `%s` | %s | %.2f | %.2f | +%.1f%% | %s %s |\n",
				reg.BenchmarkName,
				reg.Metric,
				reg.OldValue,
				reg.NewValue,
				reg.ChangePercent,
				icon,
				reg.Severity,
			)
		}
		fmt.Println()
	} else {
		fmt.Println("## No Performance Regressions")
		fmt.Println()
	}

	// Top 10 slowest benchmarks
	if len(report.Results) > 0 {
		fmt.Println("## Top 10 Slowest Benchmarks")
		fmt.Println()
		fmt.Println("| Rank | Benchmark | ns/op | B/op | allocs/op |")
		fmt.Println("|------|-----------|-------|------|-----------|")

		// Sort by ns/op (descending)
		results := make([]BenchmarkResult, len(report.Results))
		copy(results, report.Results)
		sortByNsPerOp(results)

		limit := 10
		if len(results) < limit {
			limit = len(results)
		}

		for i := 0; i < limit; i++ {
			r := results[i]
			fmt.Printf("| %d | `%s` | %.2f | %d | %d |\n",
				i+1,
				r.Name,
				r.NsPerOp,
				r.BytesPerOp,
				r.AllocsPerOp,
			)
		}
		fmt.Println()
	}
}

// sortByNsPerOp sorts results by ns/op in descending order (bubble sort, sufficient for small datasets)
func sortByNsPerOp(results []BenchmarkResult) {
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[i].NsPerOp < results[j].NsPerOp {
				results[i], results[j] = results[j], results[i]
			}
		}
	}
}

// getGitCommit returns the current Git commit hash
func getGitCommit() string {
	// Would use exec.Command("git", "rev-parse", "HEAD") in production
	// Simplified for build environment
	return os.Getenv("GITHUB_SHA")
}

// getGitBranch returns the current Git branch
func getGitBranch() string {
	// Would use exec.Command("git", "branch", "--show-current") in production
	// Simplified for build environment
	branch := os.Getenv("GITHUB_REF")
	if strings.HasPrefix(branch, "refs/heads/") {
		return strings.TrimPrefix(branch, "refs/heads/")
	}
	return branch
}

// LineScanner wraps a bufio.Scanner for line-by-line reading
type LineScanner struct {
	scanner *strings.Reader
	lines   []string
	index   int
}

// NewLineScanner creates a new line scanner
func NewLineScanner(r io.Reader) *LineScanner {
	data, _ := io.ReadAll(r)
	lines := strings.Split(string(data), "\n")
	return &LineScanner{
		lines: lines,
		index: -1,
	}
}

// Scan advances to the next line
func (ls *LineScanner) Scan() bool {
	ls.index++
	return ls.index < len(ls.lines)
}

// Text returns the current line
func (ls *LineScanner) Text() string {
	if ls.index < 0 || ls.index >= len(ls.lines) {
		return ""
	}
	return ls.lines[ls.index]
}

// Err returns any error encountered
func (ls *LineScanner) Err() error {
	return nil
}
