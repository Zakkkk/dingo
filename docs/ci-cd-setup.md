# CI/CD Setup Documentation

**Last Updated**: 2025-11-19
**Status**: Phase V - Infrastructure & Tooling

---

## Overview

The Dingo project uses GitHub Actions for continuous integration and deployment. This document describes the enhanced CI/CD pipeline that provides:

1. **Golden Test Diff Visualization** - Visual diffs when tests fail
2. **Performance Tracking** - Benchmark trends over time
3. **Source Map Validation** - Automated validation of source map accuracy
4. **Auto-Documentation** - Feature status matrix generation

---

## Workflows

### 1. Standard CI (`.github/workflows/ci.yml`)

**Triggers**: Push to `main`/`develop`, Pull Requests

**Jobs**:
- **test**: Runs on Ubuntu and macOS with Go 1.21 and 1.22
  - Unit tests with race detection
  - Golden tests
  - Compilation tests
  - Coverage reporting to Codecov
- **lint**: golangci-lint validation
- **build-vscode-extension**: Compiles VS Code extension
- **release**: Creates releases on version tags

**Artifacts**:
- Test artifacts on failure (7 days retention)
- VS Code extension VSIX (30 days retention)
- Release binaries (permanent)

### 2. Enhanced CI (`.github/workflows/enhanced-ci.yml`)

**Triggers**: Push to `main`/`develop`, Pull Requests

**Jobs**:

#### `golden-test-visualization`
Generates visual diffs when golden tests fail.

**Steps**:
1. Runs golden tests (continues on failure)
2. Builds `diff-visualizer` tool
3. Parses test output and generates markdown diff report
4. Uploads diff report as artifact (30 days)
5. Posts diff summary as PR comment (truncated if >65KB)

**Artifacts**:
- `golden-test-diffs.md` - Markdown diff report
- `test-output.log` - Raw test output

**PR Comment Format**:
```markdown
## 🔍 Golden Test Failures

**Total Failures**: N

### Summary
| Test Name | Status | Details |
|-----------|--------|---------|
| test_name | ❌ Failed | [View Diff](#test-name) |

### Detailed Diffs
[Side-by-side comparison with unified diff]
```

#### `performance-tracking`
Tracks benchmark performance over time and detects regressions.

**Steps**:
1. Runs benchmarks (5 iterations for stability)
2. Builds `performance-tracker` tool
3. Downloads previous benchmark history (if available)
4. Compares current vs historical performance
5. Generates performance report with regression analysis
6. Uploads metrics and report as artifacts

**Artifacts**:
- `benchmark-history/metrics.json` - JSON metrics (90 days)
- `performance-report.md` - Human-readable report (30 days)
- `bench-results.txt` - Raw benchmark output (30 days)

**Regression Thresholds**:
- **Warning**: >10% slowdown in ns/op or memory
- **Critical**: >20% slowdown in ns/op or memory

**PR Comment Format**:
```markdown
## 📊 Performance Benchmark Report

**Timestamp**: 2025-11-19T15:00:00Z
**Git Commit**: abc123
**Git Branch**: feature-x

### Summary
- Total Benchmarks: N
- Average ns/op: X.XX
- Average allocs/op: X.XX
- Total Memory: X.XX MB

### ⚠️ Performance Regressions Detected (if any)
| Benchmark | Metric | Old Value | New Value | Change | Severity |
|-----------|--------|-----------|-----------|--------|----------|
| BenchmarkX | ns/op | 100.0 | 125.0 | +25% | 🔴 critical |

### Top 10 Slowest Benchmarks
[Ranked list]
```

#### `sourcemap-validation`
Validates source map accuracy using the validator from Task B.

**Steps**:
1. Runs source map validation tests (`pkg/sourcemap/validator_test.go`)
2. Uploads validation log on failure (30 days)
3. Fails workflow if validation fails

**Tests Validated**:
- Round-trip position translation (Dingo ↔ Go)
- Edge cases (multi-line, nested, UTF-8)
- All golden test source maps

#### `auto-documentation`
Generates feature status matrix (main branch only).

**Steps**:
1. Scans `tests/golden/` for test files
2. Generates feature status matrix
3. Uploads to artifacts (90 days)

**Generated Documentation**:
```markdown
# Feature Status Matrix

| Feature | Status | Test Coverage | Examples |
|---------|--------|---------------|----------|
| Result[T,E] | ✅ Working | N tests | ✅ Available |
| Option[T] | ✅ Working | N tests | ✅ Available |
| Error Propagation (?) | ✅ Working | N tests | ✅ Available |
```

#### `integration-check`
Final job that depends on all enhanced CI jobs passing.

---

## Tools

### `scripts/diff-visualizer.go`

**Purpose**: Generate markdown diffs for failed golden tests

**Usage**:
```bash
go run scripts/diff-visualizer.go <test-output-file>
```

**Input**: Test output log from `go test -v`

**Output**: Markdown file with:
- Summary table of failures
- Side-by-side expected vs actual
- Unified diffs with line highlighting
- Diff statistics (lines added/removed/changed)

**Features**:
- Extracts test failure information using regex
- Reads `.go.golden` (expected) and `.go` (actual) files
- Generates GitHub-flavored markdown
- Syntax highlighting for Go code
- Hyperlinked table of contents

**Example Output**:
```markdown
### pattern_match_01_basic

**Diff Statistics:**
- Lines Added: 5
- Lines Removed: 3
- Lines Changed: 8

**Expected (.go.golden):**
```go
[code]
```

**Actual (transpiled output):**
```go
[code]
```

**Unified Diff:**
```diff
-old line
+new line
```
```

### `scripts/performance-tracker.go`

**Purpose**: Track performance benchmarks and detect regressions

**Usage**:
```bash
# Without history (first run)
go run scripts/performance-tracker.go <bench-results.txt>

# With history comparison
go run scripts/performance-tracker.go <bench-results.txt> <history.json>
```

**Input**: Raw `go test -bench` output

**Output**:
- `metrics.json` - Structured benchmark data
- Markdown report to stdout

**Features**:
- Parses `go test -bench` output
- Extracts metrics: ns/op, B/op, allocs/op, MB/s
- Compares with historical data
- Detects regressions (>10% = warning, >20% = critical)
- Generates trend analysis
- Includes Git commit/branch metadata

**JSON Schema**:
```json
{
  "timestamp": "2025-11-19T15:00:00Z",
  "git_commit": "abc123",
  "git_branch": "main",
  "results": [
    {
      "name": "TranspileFile",
      "iterations": 10000,
      "ns_per_op": 125.5,
      "bytes_per_op": 1024,
      "allocs_per_op": 5,
      "mb_per_sec": 8.0
    }
  ],
  "summary": {
    "total_benchmarks": 10,
    "avg_ns_per_op": 150.2,
    "avg_allocs_per_op": 6.5,
    "total_memory_mb": 10.5
  },
  "regressions": [
    {
      "benchmark_name": "TranspileFile",
      "metric": "ns/op",
      "old_value": 100.0,
      "new_value": 125.5,
      "change_percent": 25.5,
      "severity": "critical"
    }
  ]
}
```

**Environment Variables**:
- `GITHUB_SHA` - Git commit hash
- `GITHUB_REF` - Git branch reference

---

## Local Development

### Running Tools Locally

**Diff Visualizer**:
```bash
# Run golden tests and capture output
go test -v ./tests -run TestGoldenFiles 2>&1 | tee test-output.log

# Generate diff visualization
go run scripts/diff-visualizer.go test-output.log > diffs.md

# View in browser or editor
open diffs.md
```

**Performance Tracker**:
```bash
# Run benchmarks
go test -bench=. -benchmem -count=5 ./pkg/... > bench-results.txt

# Generate report (first run, no history)
go run scripts/performance-tracker.go bench-results.txt > performance.md

# Generate report with history comparison
go run scripts/performance-tracker.go bench-results.txt metrics.json > performance.md

# View report
cat performance.md
```

**Source Map Validation**:
```bash
# Run validation tests
go test -v ./pkg/sourcemap -run TestValidator

# With verbose output
go test -v ./pkg/sourcemap -run TestValidator 2>&1 | tee validation.log
```

### Testing Workflows Locally

Use [act](https://github.com/nektos/act) to test GitHub Actions locally:

```bash
# Install act
brew install act

# Run enhanced CI workflow
act -W .github/workflows/enhanced-ci.yml

# Run specific job
act -j golden-test-visualization

# Run with secrets
act -s GITHUB_TOKEN=<token>
```

---

## Artifact Management

### Retention Policies

| Artifact | Retention | Purpose |
|----------|-----------|---------|
| `golden-test-diffs` | 30 days | Debugging test failures |
| `benchmark-history` | 90 days | Long-term performance trends |
| `performance-report` | 30 days | Recent performance analysis |
| `sourcemap-validation-failures` | 30 days | Debugging validation failures |
| `auto-generated-docs` | 90 days | Feature status tracking |
| `test-artifacts` | 7 days | Debugging standard CI failures |
| `vscode-extension` | 30 days | Extension builds for testing |

### Downloading Artifacts

**GitHub UI**: Actions tab → Workflow run → Artifacts section

**GitHub CLI**:
```bash
# List artifacts
gh run list --workflow=enhanced-ci.yml

# Download specific artifact
gh run download <run-id> -n golden-test-diffs

# Download all artifacts from latest run
gh run download
```

**API**:
```bash
# List artifacts
curl -H "Authorization: token $GITHUB_TOKEN" \
  https://api.github.com/repos/OWNER/REPO/actions/artifacts

# Download artifact
curl -L -H "Authorization: token $GITHUB_TOKEN" \
  -o artifact.zip \
  https://api.github.com/repos/OWNER/REPO/actions/artifacts/ARTIFACT_ID/zip
```

---

## PR Comment Integration

### Golden Test Diffs

When golden tests fail, a PR comment is automatically posted:

**Trigger**: Test failure in `golden-test-visualization` job

**Content**:
- Summary table of failed tests
- Detailed diffs (truncated if >65KB)
- Link to full report in artifacts

**Permissions Required**:
- `contents: read`
- `pull-requests: write`

### Performance Reports

When benchmarks run on a PR, a performance comment is posted:

**Trigger**: Every PR (via `performance-tracking` job)

**Content**:
- Summary statistics
- Regression warnings (if any)
- Top 10 slowest benchmarks
- Link to full report in artifacts

**Threshold for Alert**:
- >10% regression: ⚠️ Warning
- >20% regression: 🔴 Critical (blocks merge)

---

## Troubleshooting

### Diff Visualizer Issues

**Problem**: "Could not extract file paths from test output"
**Solution**: Ensure test output matches expected format:
```
FAIL: TestGoldenFiles/test_name
Expected: tests/golden/test.go.golden
Actual: tests/golden/test.go
```

**Problem**: Empty diff report
**Solution**: Check test output log exists and contains failure information.

### Performance Tracker Issues

**Problem**: "Benchmark results not parsed"
**Solution**: Ensure benchmark output format:
```
BenchmarkName-8    1000    1234.5 ns/op    512 B/op    5 allocs/op
```

**Problem**: "Could not compare with history"
**Solution**: First run has no history (expected). Subsequent runs will compare.

### Source Map Validation Issues

**Problem**: Validation tests fail
**Solution**:
1. Check `pkg/sourcemap/validator.go` exists (from Task B)
2. Verify source maps are generated for golden tests
3. Review validation log for specific failures

### Workflow Failures

**Problem**: `enhanced-ci.yml` workflow fails to start
**Solution**:
1. Check workflow syntax: `act -W .github/workflows/enhanced-ci.yml --dryrun`
2. Verify required secrets are set
3. Ensure dependencies are available

**Problem**: PR comments not posted
**Solution**:
1. Check repository settings → Actions → Workflow permissions
2. Ensure "Read and write permissions" enabled
3. Verify `GITHUB_TOKEN` has `pull-requests: write` scope

---

## Future Enhancements

### Planned (Phase V+)

- **Trend Visualization**: Charts for performance metrics over time
- **Regression Alerts**: Slack/Discord notifications for critical regressions
- **Coverage Trends**: Track code coverage changes
- **Benchmark Comparison**: Side-by-side comparison in PR comments
- **Source Map Debugging**: Interactive source map explorer

### Deferred

- **Custom Diff Algorithm**: LCS-based diffs instead of line-based
- **Binary Artifact Publishing**: Automatic binary releases on tags
- **Documentation Deployment**: GitHub Pages for auto-generated docs
- **Performance Budgets**: Enforce maximum performance thresholds

---

## Related Documentation

- **Source Map Validator**: `pkg/sourcemap/validator.go` (Task B implementation)
- **Golden Test Guidelines**: `tests/golden/GOLDEN_TEST_GUIDELINES.md`
- **Standard CI**: `.github/workflows/ci.yml`
- **Phase V Plan**: `ai-docs/sessions/20251119-150114/01-planning/final-plan.md`

---

## Contact & Support

**Issues**: Report CI/CD issues on GitHub Issues with `ci/cd` label
**Improvements**: Submit PRs to enhance workflows or tools
**Questions**: See `CLAUDE.md` for development guidelines
