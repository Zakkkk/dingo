# Final Implementation Plan: Code Review Bug Fixes + Enhancements

**Session**: 20251117-224213
**Phase**: 01-planning
**Date**: 2025-11-17

---

## Executive Summary

This plan addresses 4 issues from the code review, plus adds 2 new enhancements:

### Issues (from code review):
1. **CRITICAL-2**: Source map offset incorrectly shifts all mappings - **NEEDS FIX**
2. **CRITICAL-2**: Multi-value returns dropped in error propagation - **ALREADY FIXED**, add tests
3. **IMPORTANT-1**: Stdlib import collision with user functions - **ALREADY FIXED**, add tests
4. **IMPORTANT**: Missing negative tests for edge cases - **ADD COMPREHENSIVE TESTS**

### Enhancements (from clarifications):
5. **NEW**: Add compiler flag for multi-value return mode (configurable behavior)
6. **NEW**: Document preprocessor ordering policy (imports always last)

**Total Estimated Time**: 5-6 hours
**Risk Level**: LOW (only Issue #1 requires code change)

---

## Part 1: Implementation Tasks

### Task 1.1: Fix Source Map Offset Bug (Issue #1) ✅ NEEDS CODE CHANGE

**File**: `pkg/preprocessor/preprocessor.go`
**Location**: Lines 183-192 (function `adjustMappingsForImports`)

**Root Cause**:
The condition `>= importInsertionLine` incorrectly shifts mappings AT the insertion line. These mappings represent package-level declarations BEFORE the imports and should NOT be shifted.

**Fix**:
Change `>=` to `>` in line 188:

```go
// BEFORE (line 188):
if sourceMap.Mappings[i].GeneratedLine >= importInsertionLine {

// AFTER:
if sourceMap.Mappings[i].GeneratedLine > importInsertionLine {
```

**Detailed Implementation**:
```go
func adjustMappingsForImports(sourceMap *SourceMap, numImportLines int, importInsertionLine int) {
	for i := range sourceMap.Mappings {
		// CRITICAL-2 FIX: Only shift mappings for lines AFTER import insertion
		//
		// importInsertionLine is the line number (1-based) where imports are inserted
		// (typically line 2 or 3, right after the package declaration).
		//
		// We use > (not >=) to exclude the insertion line itself. Mappings AT the
		// insertion line are for package-level declarations BEFORE the imports, and
		// should NOT be shifted.
		//
		// Example:
		//   Line 1: package main
		//   Line 2: [IMPORTS INSERTED HERE] ← importInsertionLine = 2
		//   Line 3: func foo() { ... } (shifts to line 5 if 2 imports added)
		//
		// Mappings with GeneratedLine=1 or 2 stay as-is.
		// Mappings with GeneratedLine=3+ are shifted by numImportLines.
		if sourceMap.Mappings[i].GeneratedLine > importInsertionLine {
			sourceMap.Mappings[i].GeneratedLine += numImportLines
		}
	}
}
```

**Verification**:
- Run existing tests: `go test ./pkg/preprocessor/...`
- Add negative test (see Task 2.2)

**Risk**: LOW (trivial one-line change)

---

### Task 1.2: Add Compiler Flag for Multi-Value Return Mode ✅ NEW FEATURE

**Files**:
- `cmd/dingo/main.go` (add flag)
- `pkg/preprocessor/config.go` (new file for config)
- `pkg/preprocessor/error_prop.go` (read config)

**Feature Description**:
Add a compiler flag `--multi-value-return` to control whether `return expr?` supports multi-value propagation.

**Modes**:
- `full` (default) - Support multi-value returns like `(A, B, error)`
- `single` - Restrict to single value + error like `(T, error)`

**Implementation**:

**1. Create config package** (`pkg/preprocessor/config.go`):
```go
package preprocessor

// Config holds preprocessor configuration options
type Config struct {
	// MultiValueReturnMode controls error propagation behavior for multi-value returns
	// - "full": Support (A, B, C, error) propagation (default)
	// - "single": Restrict to (T, error) only
	MultiValueReturnMode string
}

// DefaultConfig returns the default preprocessor configuration
func DefaultConfig() *Config {
	return &Config{
		MultiValueReturnMode: "full",
	}
}

// ValidateMultiValueReturnMode checks if the mode is valid
func (c *Config) ValidateMultiValueReturnMode() error {
	switch c.MultiValueReturnMode {
	case "full", "single":
		return nil
	default:
		return fmt.Errorf("invalid multi-value return mode: %q (must be 'full' or 'single')", c.MultiValueReturnMode)
	}
}
```

**2. Update Preprocessor struct** (`pkg/preprocessor/preprocessor.go`):
```go
type Preprocessor struct {
	source        []byte
	processors    []Processor
	importTracker *ImportTracker
	config        *Config  // NEW: Add config field
}

// NewWithConfig creates a preprocessor with custom configuration
func NewWithConfig(source []byte, config *Config) *Preprocessor {
	if config == nil {
		config = DefaultConfig()
	}

	p := &Preprocessor{
		source:        source,
		processors:    make([]Processor, 0),
		importTracker: NewImportTracker(),
		config:        config,
	}

	// Pass config to processors that need it
	errorProp := NewErrorPropProcessor(p.importTracker, config)
	p.processors = append(p.processors, errorProp)

	return p
}

// Existing New() function wraps NewWithConfig with defaults
func New(source []byte) *Preprocessor {
	return NewWithConfig(source, DefaultConfig())
}
```

**3. Update ErrorPropProcessor** (`pkg/preprocessor/error_prop.go`):
```go
type ErrorPropProcessor struct {
	importTracker *ImportTracker
	config        *Config  // NEW: Add config field
	// ... existing fields ...
}

func NewErrorPropProcessor(tracker *ImportTracker, config *Config) *ErrorPropProcessor {
	if config == nil {
		config = DefaultConfig()
	}
	return &ErrorPropProcessor{
		importTracker: tracker,
		config:        config,
		// ... initialize other fields ...
	}
}

// In expandReturn function (around line 416):
func (e *ErrorPropProcessor) expandReturn(...) {
	// ... existing code ...

	// Check config before allowing multi-value returns
	numNonErrorReturns := 1
	if e.currentFunc != nil && len(e.currentFunc.returnTypes) > 1 {
		// Multi-value return detected
		if e.config.MultiValueReturnMode == "single" {
			// Restrict to single value mode
			return "", nil, fmt.Errorf(
				"multi-value error propagation not allowed in 'single' mode (use --multi-value-return=full): function returns %d values plus error",
				len(e.currentFunc.returnTypes)-1,
			)
		}
		// Full mode: allow multi-value returns
		numNonErrorReturns = len(e.currentFunc.returnTypes) - 1
	}

	// ... rest of existing code ...
}
```

**4. Add CLI flag** (`cmd/dingo/main.go`):
```go
var (
	multiValueReturnMode = flag.String(
		"multi-value-return",
		"full",
		"Multi-value return propagation mode: 'full' (default, supports (A,B,error)) or 'single' (restricts to (T,error))",
	)
)

func main() {
	flag.Parse()

	// ... read input file ...

	// Create config from flags
	config := &preprocessor.Config{
		MultiValueReturnMode: *multiValueReturnMode,
	}

	// Validate config
	if err := config.ValidateMultiValueReturnMode(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Create preprocessor with config
	p := preprocessor.NewWithConfig(source, config)

	// ... rest of existing code ...
}
```

**Testing**:
- Test with `--multi-value-return=full` (default)
- Test with `--multi-value-return=single` (should error on multi-value returns)
- Test invalid mode `--multi-value-return=invalid` (should error)

**Risk**: LOW-MEDIUM (new feature, well-isolated)

---

### Task 1.3: Document Preprocessor Ordering Policy ✅ NEW DOCUMENTATION

**File**: `pkg/preprocessor/README.md` (create if doesn't exist)

**Content**:
```markdown
# Dingo Preprocessor Architecture

## Overview

The preprocessor transforms Dingo source code to valid Go code through a pipeline of processors. Each processor handles a specific language feature (error propagation, pattern matching, etc.).

## Processing Pipeline

### Stage 1: Feature Processors (Ordered)

Processors run in sequence, each receiving the output of the previous processor:

1. **Error Propagation** (`error_prop.go`)
   - Expands `?` operator to error checking code
   - Tracks function calls for automatic import detection
   - Generates source mappings for error propagation sites

2. **Pattern Matching** (future: `pattern_match.go`)
   - Expands `match` expressions to switch statements

3. **Result/Option Types** (future: `result_option.go`)
   - Transforms Result[T,E] and Option[T] to Go structs

### Stage 2: Import Injection (FINAL STEP)

After ALL processors complete:

1. Collect all needed imports from `ImportTracker`
2. Parse transformed Go source to AST
3. Inject imports using `astutil.AddImport`
4. Adjust source map offsets for injected lines
5. Format and return final Go source

**CRITICAL POLICY**: Import injection is ALWAYS the final step. No processor may run after imports are injected.

## Source Mapping Rules

### Mapping Creation

Each processor creates mappings as it transforms code:

```go
mapping := Mapping{
	OriginalLine:    originalLineInDingoSource,
	OriginalColumn:  originalColumnInDingoSource,
	GeneratedLine:   currentLineInTransformedGoCode,
	GeneratedColumn: currentColumnInTransformedGoCode,
	Length:          lengthOfTransformedToken,
	Name:            "feature_name",  // e.g., "error_prop"
}
```

### Offset Adjustment

When imports are injected, mappings are adjusted:

1. Calculate import insertion line (after package declaration)
2. Count number of import lines added
3. Shift ALL mappings with `GeneratedLine > importInsertionLine` by the number of added lines
4. Mappings AT or BEFORE the insertion line remain unchanged

**Example**:
```
BEFORE import injection:
  Line 1: package main
  Line 2:
  Line 3: func foo() { ... }  ← mapping: Generated=3

AFTER injecting 2 imports at line 2:
  Line 1: package main
  Line 2:
  Line 3: import "os"
  Line 4:
  Line 5: func foo() { ... }  ← mapping adjusted: Generated=5

Adjustment logic:
  - importInsertionLine = 2
  - numImportLines = 2
  - Original mapping: GeneratedLine=3
  - 3 > 2 → shift by 2 → new GeneratedLine=5
```

### Critical Fix (CRITICAL-2)

The offset adjustment uses `>` (not `>=`) to exclude the insertion line itself:

```go
if sourceMap.Mappings[i].GeneratedLine > importInsertionLine {
	sourceMap.Mappings[i].GeneratedLine += numImportLines
}
```

This ensures mappings AT the insertion line (package-level declarations before imports) are NOT shifted.

## Adding New Processors

### Step 1: Implement Processor Interface

```go
type MyFeatureProcessor struct {
	importTracker *ImportTracker  // For tracking needed imports
	config        *Config          // For feature flags
}

func (p *MyFeatureProcessor) Process(source []byte, sourceMap *SourceMap) ([]byte, error) {
	// Transform source
	// Create mappings
	// Track imports
	return transformedSource, nil
}
```

### Step 2: Register in Pipeline

Add to `Preprocessor.processors` in `New()` or `NewWithConfig()`:

```go
func New(source []byte) *Preprocessor {
	p := &Preprocessor{
		source:        source,
		importTracker: NewImportTracker(),
	}

	// Add processors IN ORDER
	p.processors = append(p.processors, NewErrorPropProcessor(p.importTracker))
	p.processors = append(p.processors, NewMyFeatureProcessor(p.importTracker))  // NEW

	return p
}
```

### Step 3: Verify Import Policy

Ensure your processor:
- ✅ Runs BEFORE import injection (automatic if added to processors list)
- ✅ Creates mappings with correct line numbers (relative to current transformed source)
- ✅ Tracks imports via `ImportTracker` (don't inject imports directly)
- ✅ Returns error for invalid input (don't panic)

## Testing Guidelines

### Unit Tests

Each processor should have comprehensive tests:

```go
func TestMyFeatureProcessor_BasicCase(t *testing.T) {
	input := `package main

func example() {
	// Feature-specific Dingo syntax
}
`

	tracker := NewImportTracker()
	processor := NewMyFeatureProcessor(tracker)

	result, err := processor.Process([]byte(input), NewSourceMap())
	if err != nil {
		t.Fatalf("processing failed: %v", err)
	}

	// Verify transformed output
	// Verify source mappings
	// Verify tracked imports
}
```

### Integration Tests

Test full pipeline with `Preprocessor.Process()`:

```go
func TestPreprocessor_MyFeature(t *testing.T) {
	input := `package main

func example() {
	// Feature-specific Dingo syntax
}
`

	p := New([]byte(input))
	result, sourceMap, err := p.Process()

	// Verify final Go output compiles
	// Verify source map correctness
	// Verify imports were injected
}
```

### Negative Tests

Always test edge cases:

- Invalid syntax → error
- User-defined functions shadowing stdlib → no import injection
- Empty input → valid output
- Large input → performance

## Architecture Decisions

### Q: Why is import injection the final step?

**A**: To simplify source map offset calculations. If imports were injected mid-pipeline:
- Later processors would see shifted line numbers
- Each processor would need to track offsets from previous processors
- Source maps would need multi-stage offset tracking

By injecting imports LAST, all processors work with unshifted line numbers, and we apply offsets ONCE at the end.

### Q: Why not inject imports per-processor?

**A**: Multiple reasons:
1. Duplicate imports would need deduplication
2. Each injection would require re-parsing the AST
3. Source map offsets would compound (processor 1 shifts, processor 2 shifts again)
4. Performance: parsing AST is expensive

Collecting imports via `ImportTracker` and injecting once is more efficient and simpler.

### Q: What if a processor needs imports mid-transformation?

**A**: Use `ImportTracker` to register needed imports, but don't inject them. The processor can ASSUME the import will exist in the final output and generate code accordingly:

```go
// In processor:
e.importTracker.TrackFunctionCall("os.ReadFile")  // Register need for "os"

// Generate code as if import exists:
buf.WriteString(`data, err := os.ReadFile(path)`)  // Valid after import injection
```

### Q: How do I debug source map issues?

**A**: Enable verbose logging:

```go
// In preprocessor.go:
func (p *Preprocessor) Process() (string, *SourceMap, error) {
	// ... processing ...

	if os.Getenv("DINGO_DEBUG_SOURCEMAP") == "1" {
		fmt.Fprintf(os.Stderr, "=== Source Map Debug ===\n")
		fmt.Fprintf(os.Stderr, "Import insertion line: %d\n", importInsertLine)
		fmt.Fprintf(os.Stderr, "Import lines added: %d\n", linesAdded)
		for i, m := range sourceMap.Mappings {
			fmt.Fprintf(os.Stderr, "Mapping %d: Gen=%d Orig=%d\n", i, m.GeneratedLine, m.OriginalLine)
		}
	}

	// ... rest of code ...
}
```

Run with: `DINGO_DEBUG_SOURCEMAP=1 dingo build file.dingo`

## Future Enhancements

### Parallel Processing

Currently processors run sequentially. Future optimization: run independent processors in parallel.

**Requirements**:
- Processors must not depend on each other's output
- Source maps would need merge logic
- Import tracking would need concurrency safety

**Benefit**: Faster compilation for large files

### Lazy Import Injection

Currently we inject ALL tracked imports. Future optimization: only inject imports actually used in the final code.

**Requirements**:
- Dead code elimination to identify unused imports
- Analysis pass after all processors complete
- Maintain import registration for error messages

**Benefit**: Cleaner generated code, faster compilation
```

**Risk**: NONE (documentation only)

---

## Part 2: Test Strategy

### Task 2.1: Verify Already-Fixed Issues (Issues #2 & #3) ✅ VERIFICATION ONLY

**Issue #2: Multi-Value Returns**

**Test**: Run existing test `error_prop_09_multi_value`
```bash
cd tests/golden
go test -v -run TestGolden/error_prop_09_multi_value
```

**Expected**: PASS (fix is already implemented)

**Manual Verification**:
```bash
# Create test file
cat > /tmp/test_multi.dingo <<EOF
package main

func extractUserFields(input string) (string, string, int, error) {
	return "name", "email", 42, nil
}

func processUser(input string) (string, string, int, error) {
	return extractUserFields(input)?
}
EOF

# Compile
dingo build /tmp/test_multi.dingo

# Verify output has: return __tmp0, __tmp1, __tmp2, nil
cat /tmp/test_multi.go | grep "return __tmp"
```

**Issue #3: Import Collision**

**Test**: Create negative test (see Task 2.3)

**Manual Verification**:
```bash
# Create test file with user-defined ReadFile
cat > /tmp/test_user_func.dingo <<EOF
package main

func ReadFile(path string) ([]byte, error) {
	return []byte("mock"), nil
}

func loadConfig(path string) ([]byte, error) {
	let data = ReadFile(path)?
	return data, nil
}
EOF

# Compile
dingo build /tmp/test_user_func.dingo

# Verify NO import "os" in output
cat /tmp/test_user_func.go | grep 'import "os"'
# Expected: empty (no match)
```

**Risk**: NONE (verification only)

---

### Task 2.2: Add Negative Test for Issue #1 ✅ NEW TEST

**File**: `pkg/preprocessor/preprocessor_test.go`

**Test Name**: `TestCRITICAL2_SourceMapOffset_PackageLevelNotShifted`

**Implementation**:
```go
// TestCRITICAL2_SourceMapOffset_PackageLevelNotShifted verifies that source map
// offsets for package-level declarations BEFORE imports are NOT shifted when
// imports are injected. This prevents CRITICAL-2 regression.
func TestCRITICAL2_SourceMapOffset_PackageLevelNotShifted(t *testing.T) {
	input := `package main

var config = "default"

func loadFile(path string) ([]byte, error) {
	let data = os.ReadFile(path)?
	return data, nil
}
`

	p := New([]byte(input))
	result, sourceMap, err := p.Process()
	if err != nil {
		t.Fatalf("preprocessing failed: %v", err)
	}

	// Verify imports were injected
	if !strings.Contains(result, `import "os"`) {
		t.Fatalf("expected os import, got:\n%s", result)
	}

	// Parse output to find line numbers
	lines := strings.Split(result, "\n")

	// Find package declaration line (should be line 1)
	packageLine := -1
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "package ") {
			packageLine = i + 1 // Convert to 1-based
			break
		}
	}
	if packageLine != 1 {
		t.Fatalf("expected package declaration on line 1, found on line %d", packageLine)
	}

	// Find import block
	importStartLine := -1
	importEndLine := -1
	for i, line := range lines {
		if strings.Contains(line, "import ") {
			if importStartLine == -1 {
				importStartLine = i + 1
			}
			importEndLine = i + 1
		}
	}
	if importStartLine == -1 {
		t.Fatalf("no import block found in output")
	}

	// Find error propagation line (should have __tmp0)
	errorPropLine := -1
	for i, line := range lines {
		if strings.Contains(line, "__tmp0") {
			errorPropLine = i + 1
			break
		}
	}
	if errorPropLine == -1 {
		t.Fatalf("no error propagation found in output")
	}

	// Verify source map offsets
	// 1. Package-level declarations (line 1-2) should NOT have shifted mappings
	// 2. Error propagation (after imports) SHOULD have shifted mappings

	for _, mapping := range sourceMap.Mappings {
		// Mappings for lines BEFORE imports should NOT be shifted beyond import block
		if mapping.OriginalLine <= 2 { // package and var declaration
			if mapping.GeneratedLine > importStartLine {
				t.Errorf(
					"REGRESSION: Mapping for original line %d was shifted to generated line %d (after import block at line %d)\n"+
					"This indicates the offset adjustment incorrectly shifted package-level mappings.",
					mapping.OriginalLine, mapping.GeneratedLine, importStartLine,
				)
			}
		}

		// Mappings for error propagation (original line 6) SHOULD be shifted
		if mapping.Name == "error_prop" {
			if mapping.GeneratedLine <= importEndLine {
				t.Errorf(
					"ERROR: Error propagation mapping was NOT shifted past import block\n"+
					"Generated line: %d, import block ends at: %d",
					mapping.GeneratedLine, importEndLine,
				)
			}
		}
	}

	// Additional verification: compare line counts
	inputLines := len(strings.Split(input, "\n"))
	outputLines := len(lines)
	expectedIncrease := importEndLine - importStartLine + 1 // number of import lines

	t.Logf("Input lines: %d", inputLines)
	t.Logf("Output lines: %d", outputLines)
	t.Logf("Import block: lines %d-%d (%d lines)", importStartLine, importEndLine, expectedIncrease)
	t.Logf("Error propagation: line %d", errorPropLine)
}
```

**Expected Result**: PASS after applying Issue #1 fix

**Risk**: NONE (test only)

---

### Task 2.3: Add Comprehensive Negative Tests (Issue #4) ✅ NEW TESTS

**File**: `pkg/preprocessor/preprocessor_test.go`

#### Test Group 1: User Function Shadowing (Issue #3 verification)

**Test 1.1**: User-defined `ReadFile`
```go
func TestNegative_UserDefinedReadFile(t *testing.T) {
	input := `package main

// User-defined helper with same name as os.ReadFile
func ReadFile(path string) ([]byte, error) {
	return []byte("mock data"), nil
}

func loadConfig(path string) ([]byte, error) {
	let data = ReadFile(path)?
	return data, nil
}
`

	p := New([]byte(input))
	result, _, err := p.Process()
	if err != nil {
		t.Fatalf("preprocessing failed: %v", err)
	}

	// Verify NO import "os" was injected
	if strings.Contains(result, `import "os"`) {
		t.Errorf("REGRESSION: User-defined ReadFile triggered os import injection!\n%s", result)
	}

	// Verify the function call was NOT modified
	if !strings.Contains(result, `ReadFile(path)`) {
		t.Errorf("User-defined ReadFile call was incorrectly modified\n%s", result)
	}
}
```

**Test 1.2**: User-defined `Atoi`
```go
func TestNegative_UserDefinedAtoi(t *testing.T) {
	input := `package main

// Custom string to int converter
func Atoi(s string) (int, error) {
	return 42, nil
}

func parseValue(s string) (int, error) {
	let value = Atoi(s)?
	return value, nil
}
`

	p := New([]byte(input))
	result, _, err := p.Process()
	if err != nil {
		t.Fatalf("preprocessing failed: %v", err)
	}

	// Verify NO import "strconv" was injected
	if strings.Contains(result, `import "strconv"`) {
		t.Errorf("REGRESSION: User-defined Atoi triggered strconv import injection!\n%s", result)
	}
}
```

**Test 1.3**: User-defined `Marshal`
```go
func TestNegative_UserDefinedMarshal(t *testing.T) {
	input := `package main

// Custom marshaler
func Marshal(v interface{}) ([]byte, error) {
	return []byte("{}"), nil
}

func serialize(data map[string]string) ([]byte, error) {
	let bytes = Marshal(data)?
	return bytes, nil
}
`

	p := New([]byte(input))
	result, _, err := p.Process()
	if err != nil {
		t.Fatalf("preprocessing failed: %v", err)
	}

	// Verify NO import "encoding/json" was injected
	if strings.Contains(result, `import "encoding/json"`) {
		t.Errorf("REGRESSION: User-defined Marshal triggered json import injection!\n%s", result)
	}
}
```

#### Test Group 2: Multi-Value Return Edge Cases (Issue #2 verification)

**Test 2.1**: Single error return only
```go
func TestNegative_SingleErrorReturn(t *testing.T) {
	input := `package main

func validate(input string) error {
	return nil
}

func process(input string) error {
	// This should compile but may need special handling
	// Error-only returns don't use ? operator typically
	return validate(input)
}
`

	p := New([]byte(input))
	_, _, err := p.Process()
	if err != nil {
		t.Fatalf("preprocessing failed: %v", err)
	}

	// Note: Error-only returns shouldn't use ? operator
	// This test ensures we don't crash on error-only signatures
}
```

**Test 2.2**: Four non-error values
```go
func TestNegative_FourValueReturn(t *testing.T) {
	input := `package main

func parseRecord(line string) (string, int, float64, bool, error) {
	return "name", 42, 3.14, true, nil
}

func processRecord(line string) (string, int, float64, bool, error) {
	return parseRecord(line)?
}
`

	p := New([]byte(input))
	result, _, err := p.Process()
	if err != nil {
		t.Fatalf("preprocessing failed: %v", err)
	}

	// Verify all 4 temporary variables are returned
	if !strings.Contains(result, "return __tmp0, __tmp1, __tmp2, __tmp3, nil") {
		t.Errorf("Expected 4 temporary variables in return, got:\n%s", result)
	}

	// Verify all 4 zero values in error path
	if !strings.Contains(result, `return "", 0, 0.0, false,`) {
		t.Errorf("Expected 4 zero values in error return, got:\n%s", result)
	}
}
```

**Test 2.3**: Nested multi-value returns
```go
func TestNegative_NestedMultiValueReturn(t *testing.T) {
	input := `package main

func inner() (int, string, error) {
	return 1, "a", nil
}

func middle() (int, string, error) {
	let a, b = inner()?
	return a, b, nil
}

func outer() (int, string, error) {
	return middle()?
}
`

	p := New([]byte(input))
	result, _, err := p.Process()
	if err != nil {
		t.Fatalf("preprocessing failed: %v", err)
	}

	// Verify nested error propagation works
	// Count __tmp variables (should have multiple sets)
	tmpCount := strings.Count(result, "__tmp")
	if tmpCount < 2 {
		t.Errorf("Expected multiple __tmp variables for nested calls, got %d occurrences", tmpCount)
	}
}
```

#### Test Group 3: Import Injection Edge Cases

**Test 3.1**: Multiple qualified calls
```go
func TestNegative_MultipleQualifiedCalls(t *testing.T) {
	input := `package main

func loadAndParse(path string) (map[string]interface{}, error) {
	let data = os.ReadFile(path)?
	let parsed = json.Unmarshal(data)?
	return parsed, nil
}
`

	p := New([]byte(input))
	result, _, err := p.Process()
	if err != nil {
		t.Fatalf("preprocessing failed: %v", err)
	}

	// Verify BOTH imports were injected
	if !strings.Contains(result, `"os"`) {
		t.Errorf("Expected os import, got:\n%s", result)
	}
	if !strings.Contains(result, `"encoding/json"`) {
		t.Errorf("Expected encoding/json import, got:\n%s", result)
	}
}
```

**Test 3.2**: Mix of qualified and user-defined
```go
func TestNegative_MixedQualifiedAndUserCalls(t *testing.T) {
	input := `package main

func ReadFile(path string) ([]byte, error) {
	return []byte("mock"), nil
}

func loadConfig(path string) ([]byte, error) {
	// User-defined ReadFile
	let userdata = ReadFile("user.txt")?

	// Stdlib os.ReadFile
	let sysdata = os.ReadFile("system.txt")?

	return append(userdata, sysdata...), nil
}
`

	p := New([]byte(input))
	result, _, err := p.Process()
	if err != nil {
		t.Fatalf("preprocessing failed: %v", err)
	}

	// Verify ONLY qualified call triggered import
	if !strings.Contains(result, `import "os"`) {
		t.Errorf("Expected os import for os.ReadFile, got:\n%s", result)
	}

	// Verify BOTH calls are present
	if !strings.Contains(result, `ReadFile("user.txt")`) {
		t.Errorf("User-defined ReadFile call missing\n%s", result)
	}
	if !strings.Contains(result, `os.ReadFile("system.txt")`) {
		t.Errorf("Qualified os.ReadFile call missing\n%s", result)
	}
}
```

**Test 3.3**: No error propagation (no imports needed)
```go
func TestNegative_NoErrorPropagation(t *testing.T) {
	input := `package main

func calculate(a, b int) int {
	return a + b
}

func main() {
	result := calculate(1, 2)
	println(result)
}
`

	p := New([]byte(input))
	result, _, err := p.Process()
	if err != nil {
		t.Fatalf("preprocessing failed: %v", err)
	}

	// Verify NO imports were injected
	if strings.Contains(result, "import ") {
		t.Errorf("Unexpected imports injected for code without error propagation:\n%s", result)
	}

	// Verify source is unchanged (except formatting)
	if !strings.Contains(result, "calculate(a, b int)") {
		t.Errorf("Source code was modified when it shouldn't be:\n%s", result)
	}
}
```

#### Test Group 4: Multi-Value Return Mode Flag (NEW Feature)

**Test 4.1**: Full mode allows multi-value
```go
func TestConfigFlag_FullModeAllowsMultiValue(t *testing.T) {
	input := `package main

func parse(s string) (string, int, error) {
	return "name", 42, nil
}

func process(s string) (string, int, error) {
	return parse(s)?
}
`

	config := &Config{MultiValueReturnMode: "full"}
	p := NewWithConfig([]byte(input), config)
	result, _, err := p.Process()

	if err != nil {
		t.Fatalf("full mode should allow multi-value returns, got error: %v", err)
	}

	// Verify multi-value expansion happened
	if !strings.Contains(result, "__tmp0") && !strings.Contains(result, "__tmp1") {
		t.Errorf("Expected multi-value expansion in full mode, got:\n%s", result)
	}
}
```

**Test 4.2**: Single mode rejects multi-value
```go
func TestConfigFlag_SingleModeRejectsMultiValue(t *testing.T) {
	input := `package main

func parse(s string) (string, int, error) {
	return "name", 42, nil
}

func process(s string) (string, int, error) {
	return parse(s)?  // Should error in single mode
}
`

	config := &Config{MultiValueReturnMode: "single"}
	p := NewWithConfig([]byte(input), config)
	_, _, err := p.Process()

	if err == nil {
		t.Fatalf("single mode should reject multi-value returns, but got no error")
	}

	// Verify error message mentions mode restriction
	if !strings.Contains(err.Error(), "multi-value") || !strings.Contains(err.Error(), "single") {
		t.Errorf("Error should mention multi-value restriction, got: %v", err)
	}
}
```

**Test 4.3**: Single mode allows (T, error)
```go
func TestConfigFlag_SingleModeAllowsSingleValue(t *testing.T) {
	input := `package main

func read(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func load(path string) ([]byte, error) {
	return read(path)?  // Should work in single mode
}
`

	config := &Config{MultiValueReturnMode: "single"}
	p := NewWithConfig([]byte(input), config)
	result, _, err := p.Process()

	if err != nil {
		t.Fatalf("single mode should allow (T, error) returns, got error: %v", err)
	}

	// Verify single-value expansion happened
	if !strings.Contains(result, "__tmp0") {
		t.Errorf("Expected single-value expansion in single mode, got:\n%s", result)
	}

	// Verify NO second tmp variable
	if strings.Contains(result, "__tmp1") {
		t.Errorf("Should only have one tmp variable in single mode, got:\n%s", result)
	}
}
```

**Test 4.4**: Invalid mode errors
```go
func TestConfigFlag_InvalidModeErrors(t *testing.T) {
	config := &Config{MultiValueReturnMode: "invalid"}

	err := config.ValidateMultiValueReturnMode()
	if err == nil {
		t.Fatalf("invalid mode should error, got nil")
	}

	if !strings.Contains(err.Error(), "invalid") {
		t.Errorf("Error should mention 'invalid', got: %v", err)
	}
}
```

**Risk**: LOW (tests only, no production code changes)

---

## Part 3: Documentation Updates

### Task 3.1: Update CHANGELOG.md ✅ REQUIRED

**File**: `/Users/jack/mag/dingo/CHANGELOG.md`

**Add Section**:
```markdown
## [Unreleased]

### Fixed
- **CRITICAL-2**: Fixed source map offset bug where mappings before import block were incorrectly shifted
  - Changed condition from `>=` to `>` in `adjustMappingsForImports`
  - Prevents IDE navigation issues for package-level declarations
  - Added comprehensive negative test to prevent regression

### Added
- **NEW**: Compiler flag `--multi-value-return` to control error propagation behavior
  - `full` mode (default): Supports multi-value returns like `(A, B, C, error)`
  - `single` mode: Restricts to single value returns like `(T, error)`
  - Configurable via `Config.MultiValueReturnMode` in preprocessor API
  - See `pkg/preprocessor/README.md` for usage

### Documented
- **NEW**: Preprocessor architecture and ordering policy in `pkg/preprocessor/README.md`
  - All processors must run before import injection
  - Import injection is always the final step
  - Source map offset adjustment rules
  - Guidelines for adding new processors

### Verified
- **CRITICAL-2**: Multi-value return support in error propagation (already fixed)
  - Confirmed by test `error_prop_09_multi_value`
  - Added edge case tests for 4+ value returns
- **IMPORTANT-1**: Stdlib import collision prevention (already fixed)
  - Only qualified calls like `os.ReadFile()` trigger imports
  - User-defined `ReadFile()` does NOT trigger import injection
  - Added negative tests to prevent regression

### Tests Added
- Negative test for source map offset before imports (`TestCRITICAL2_SourceMapOffset_PackageLevelNotShifted`)
- Negative tests for user-defined functions shadowing stdlib (`TestNegative_UserDefined*`)
- Edge case tests for multi-value returns (`TestNegative_*ValueReturn`)
- Tests for new multi-value return mode flag (`TestConfigFlag_*`)
- Tests for import injection edge cases (`TestNegative_Multiple*`)
```

**Risk**: NONE (documentation only)

---

### Task 3.2: Update cmd/dingo README (if exists) ✅ OPTIONAL

**File**: `/Users/jack/mag/dingo/cmd/dingo/README.md` (create if doesn't exist)

**Add Section**:
```markdown
## Command-Line Flags

### --multi-value-return

Controls error propagation behavior for multi-value returns.

**Modes**:
- `full` (default): Supports multi-value error propagation
  ```dingo
  func parse(s string) (string, int, error) { ... }

  func process(s string) (string, int, error) {
      return parse(s)?  // ✓ Expands to return all values
  }
  ```

- `single`: Restricts error propagation to single-value returns only
  ```dingo
  func read(path string) ([]byte, error) { ... }

  func load(path string) ([]byte, error) {
      return read(path)?  // ✓ Allowed
  }

  func parse(s string) (string, int, error) { ... }

  func process(s string) (string, int, error) {
      return parse(s)?  // ✗ Error: multi-value not allowed
  }
  ```

**Usage**:
```bash
# Default (full mode)
dingo build file.dingo

# Explicit full mode
dingo build --multi-value-return=full file.dingo

# Single value mode
dingo build --multi-value-return=single file.dingo
```

**Rationale**: Some teams prefer restricting error propagation to the common `(T, error)` pattern for consistency with Go stdlib conventions. Others want the power of multi-value propagation for complex data pipelines.
```

**Risk**: NONE (documentation only)

---

## Part 4: Success Criteria

### Build & Test Validation ✅

**Criteria**:
1. All existing tests pass: `go test ./...`
2. All new negative tests pass
3. Golden tests pass: `cd tests/golden && go test`
4. Build succeeds: `go build ./cmd/dingo`
5. Manual verification tests pass (see Task 2.1)

**Commands**:
```bash
# Run all tests
go test ./... -v

# Run only new tests
go test ./pkg/preprocessor -v -run TestNegative
go test ./pkg/preprocessor -v -run TestCRITICAL2
go test ./pkg/preprocessor -v -run TestConfigFlag

# Build CLI
go build ./cmd/dingo

# Test CLI flag
./dingo build --multi-value-return=single tests/golden/error_prop_09_multi_value.dingo
# Expected: error (multi-value not allowed in single mode)

./dingo build --multi-value-return=full tests/golden/error_prop_09_multi_value.dingo
# Expected: success
```

### Code Review Response ✅

**Criteria**:
1. Issue #1 (source map offset) - FIXED with test
2. Issue #2 (multi-value returns) - VERIFIED with additional tests
3. Issue #3 (import collision) - VERIFIED with negative test
4. Issue #4 (negative tests) - COMPREHENSIVE SUITE ADDED

**Response Format**:
```markdown
# Code Review Response

## CRITICAL-2 Issues

### Issue #1: Source Map Offset Bug ✅ FIXED
**Status**: Fixed in commit [COMMIT_HASH]
**Fix**: Changed condition from `>=` to `>` in `adjustMappingsForImports`
**Test**: Added `TestCRITICAL2_SourceMapOffset_PackageLevelNotShifted`
**Verification**: All source map tests pass

### Issue #2: Multi-Value Returns ✅ ALREADY FIXED
**Status**: Fix already present in codebase (lines 416-431, 524-530 of error_prop.go)
**Verification**: Test `error_prop_09_multi_value` passes
**Additional Tests**: Added edge cases for 4+ values and nested returns

## IMPORTANT Issues

### Issue #3: Import Collision ✅ ALREADY FIXED
**Status**: Fix already present in codebase (lines 859-866 of error_prop.go)
**Verification**: Only qualified calls trigger imports
**Test**: Added `TestNegative_UserDefined*` suite

### Issue #4: Negative Tests ✅ COMPREHENSIVE SUITE ADDED
**Status**: Added 12+ negative tests covering all edge cases
**Tests**:
- User function shadowing (3 tests)
- Multi-value edge cases (3 tests)
- Import injection edge cases (3 tests)
- Config flag validation (4 tests)

## Additional Enhancements

### NEW Feature: Multi-Value Return Mode Flag
**Status**: Implemented in commit [COMMIT_HASH]
**Usage**: `dingo build --multi-value-return={full|single} file.dingo`
**Rationale**: Provides flexibility for teams preferring Go stdlib conventions

### NEW Documentation: Preprocessor Architecture
**Status**: Documented in `pkg/preprocessor/README.md`
**Content**:
- Processing pipeline order
- Import injection policy (always final step)
- Source map offset rules
- Guidelines for new processors
```

---

## Part 5: Parallelization Opportunities

The following tasks can be executed independently in parallel:

### Parallel Group 1: Code Changes (2 tasks)
- Task 1.1: Fix source map offset bug (30 min)
- Task 1.2: Add compiler flag for multi-value mode (2 hours)

**Dependency**: None (independent code paths)

### Parallel Group 2: Verification (2 tasks)
- Task 2.1: Verify Issue #2 (multi-value) - (15 min)
- Task 2.1: Verify Issue #3 (import collision) - (15 min)

**Dependency**: None (independent features)

### Parallel Group 3: Test Writing (3 groups)
- Task 2.2: Source map negative test (30 min)
- Task 2.3 Group 1: User shadowing tests (30 min)
- Task 2.3 Group 2: Multi-value edge tests (30 min)
- Task 2.3 Group 3: Import edge tests (30 min)
- Task 2.3 Group 4: Config flag tests (45 min)

**Dependency**: Task 1.2 must complete before Task 2.3 Group 4 (config tests need config implementation)

### Parallel Group 4: Documentation (2 tasks)
- Task 1.3: Preprocessor README (1 hour)
- Task 3.2: CLI flag documentation (15 min)

**Dependency**: Task 1.2 must complete before Task 3.2 (document implemented flag)

### Sequential Dependencies:
```
START
  ├─ [Parallel] Task 1.1 (fix) + Task 2.1 (verify) + Task 1.3 (doc) → 1 hour
  │
  ├─ Task 1.2 (config flag) → 2 hours
  │    └─ [Parallel] Task 3.2 (CLI doc) + Task 2.3 Group 4 (config tests) → 45 min
  │
  ├─ [Parallel] Task 2.2 + Task 2.3 Groups 1-3 (tests) → 1 hour
  │
  └─ Task 3.1 (CHANGELOG) → 15 min
END

Total Time (with parallelization): ~4 hours
Total Time (sequential): ~6.5 hours
Time Saved: 2.5 hours (38%)
```

---

## Part 6: Risk Assessment

| Component | Risk Level | Impact | Mitigation |
|-----------|------------|--------|------------|
| Task 1.1 (source map fix) | LOW | CRITICAL | Trivial change, covered by test |
| Task 1.2 (config flag) | LOW-MEDIUM | MEDIUM | Well-isolated feature, comprehensive tests |
| Task 1.3 (documentation) | NONE | LOW | Documentation only |
| Task 2.1 (verification) | NONE | HIGH | Confirms existing fixes work |
| Task 2.2 (source map test) | NONE | HIGH | Prevents future regression |
| Task 2.3 (negative tests) | NONE | HIGH | Locks in correct behavior |
| Task 3.1 (CHANGELOG) | NONE | LOW | Documentation only |
| Task 3.2 (CLI doc) | NONE | LOW | Documentation only |

**Overall Risk**: LOW

**Rollback Plan**:
- Task 1.1: Revert single line if issues arise (unlikely)
- Task 1.2: Feature is opt-in (default behavior unchanged), can disable flag in future release
- All tests: Can be removed without affecting production code

---

## Part 7: Implementation Order Recommendation

### Recommended Sequence:

**Phase 1: Quick Wins (1 hour)**
1. Task 1.1: Fix source map offset (30 min)
2. Task 2.1: Verify already-fixed issues (15 min)
3. Task 2.2: Add source map negative test (15 min)

**Phase 2: Config Flag Feature (2.5 hours)**
4. Task 1.2: Implement multi-value return mode flag (2 hours)
5. Task 2.3 Group 4: Add config flag tests (30 min)

**Phase 3: Comprehensive Testing (1.5 hours)**
6. Task 2.3 Groups 1-3: Add all negative tests (1.5 hours)

**Phase 4: Documentation (1.5 hours)**
7. Task 1.3: Write preprocessor README (1 hour)
8. Task 3.2: Document CLI flag (15 min)
9. Task 3.1: Update CHANGELOG (15 min)

**Total Time**: 6.5 hours (sequential) or 4 hours (with parallelization)

---

## Appendix A: Code Snippets Reference

### Snippet A1: Source Map Fix (Task 1.1)

**File**: `pkg/preprocessor/preprocessor.go:188`

**Change**:
```diff
- if sourceMap.Mappings[i].GeneratedLine >= importInsertionLine {
+ if sourceMap.Mappings[i].GeneratedLine > importInsertionLine {
```

### Snippet A2: Config Validation (Task 1.2)

**File**: `pkg/preprocessor/config.go` (new file)

```go
package preprocessor

import "fmt"

type Config struct {
	MultiValueReturnMode string // "full" or "single"
}

func DefaultConfig() *Config {
	return &Config{
		MultiValueReturnMode: "full",
	}
}

func (c *Config) ValidateMultiValueReturnMode() error {
	switch c.MultiValueReturnMode {
	case "full", "single":
		return nil
	default:
		return fmt.Errorf(
			"invalid multi-value return mode: %q (must be 'full' or 'single')",
			c.MultiValueReturnMode,
		)
	}
}
```

### Snippet A3: CLI Flag (Task 1.2)

**File**: `cmd/dingo/main.go`

```go
var (
	multiValueReturnMode = flag.String(
		"multi-value-return",
		"full",
		"Multi-value return propagation mode: 'full' (default) or 'single'",
	)
)

func main() {
	flag.Parse()

	config := &preprocessor.Config{
		MultiValueReturnMode: *multiValueReturnMode,
	}

	if err := config.ValidateMultiValueReturnMode(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	p := preprocessor.NewWithConfig(source, config)
	// ... rest of code
}
```

---

## Appendix B: Test Examples Reference

See Task 2.2 and Task 2.3 sections above for complete test implementations.

---

**END OF FINAL PLAN**
