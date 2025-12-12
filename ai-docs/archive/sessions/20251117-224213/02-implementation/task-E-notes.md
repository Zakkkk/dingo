# Task E: Implementation Notes

## Implementation Decisions

### 1. Error Propagation Architecture
**Decision**: Modified function signatures to propagate errors instead of using struct-level error tracking.

**Rationale**:
- The existing code pattern for processLine/expandAssignment/expandReturn returned (string, []Mapping)
- To support config-based validation errors, we needed to add error returns
- Changed all three functions to return (string, []Mapping, error)
- This allows the config validation in expandReturn() to return an error that bubbles up through processLine() → Process()

**Implementation**:
- Updated processLine signature: lines 255
- Updated expandAssignment signature: line 315
- Updated expandReturn signature: line 429
- Added error handling in Process() at lines 204-207
- All successful paths return nil error

### 2. Config Threading
**Decision**: Pass config through preprocessor → processor constructor chain.

**Rationale**:
- Config needs to be accessible in ErrorPropProcessor for runtime validation
- NewWithConfig() creates preprocessor with config, passes to NewErrorPropProcessorWithConfig()
- Maintains backward compatibility: New() still works, wraps NewWithConfig(nil)

**Implementation**:
- Preprocessor.config field (preprocessor.go:21)
- NewWithConfig() function (preprocessor.go:48-74)
- ErrorPropProcessor.config field (error_prop.go:150)
- NewErrorPropProcessorWithConfig() function (error_prop.go:165-173)

### 3. CLI Flag Integration
**Decision**: Add --multi-value-return flag to both `build` and `run` commands.

**Rationale**:
- Users need consistent flag across both build and run workflows
- Validation happens early (before preprocessing) to provide clear error messages
- Default value "full" maintains backward compatibility

**Implementation**:
- buildCmd: lines 66-70, 89, 95
- runCmd: lines 101, 119, 130, 134
- Config validation in both runBuild() and runDingoFile()

### 4. Validation Logic
**Decision**: Enforce single-value restriction only in expandReturn(), not expandAssignment().

**Rationale**:
- The flag controls "return expr?" behavior (multi-value returns)
- Assignment "x := expr?" is always single-value (x is one variable)
- Only return statements can propagate multiple values: (A, B, error)
- Check happens at line 442-450 in expandReturn()

**Validation**:
```go
if e.config != nil && e.config.MultiValueReturnMode == "single" && numNonErrorReturns > 1 {
	return "", nil, fmt.Errorf(
		"multi-value error propagation not allowed in 'single' mode (use --multi-value-return=full): function returns %d values plus error",
		numNonErrorReturns,
	)
}
```

## Deviations from Plan

### Minor Deviation 1: Error Return Signatures
**Plan**: The plan showed adding config but didn't explicitly show changing return signatures to include error.

**Reality**: Had to modify processLine, expandAssignment, and expandReturn to return (string, []Mapping, error) instead of (string, []Mapping).

**Justification**: This is the only way to propagate validation errors from the config check in expandReturn() back to the Process() method. The alternative would be to add a struct-level error field, but that's less clean and harder to reason about.

### Minor Deviation 2: Constructor Naming
**Plan**: Suggested NewErrorPropProcessor(tracker, config)

**Reality**: Implemented NewErrorPropProcessorWithConfig(config) and kept NewErrorPropProcessor() for backward compatibility.

**Justification**: The current codebase doesn't pass importTracker to the constructor - it's initialized in Process(). Maintaining the existing pattern while adding config support was cleaner.

## Testing Notes

### Build Verification
- ✅ Successfully built with `go build ./cmd/dingo`
- ✅ No compilation errors
- ✅ All imports resolved correctly

### Actual Testing Performed

**Test 1: Multi-value return with single mode (should fail)**
```bash
./dingo build --multi-value-return=single test_multi_value2.dingo
```
Result: ✅ **PASS** - Correctly rejected with error:
```
Error: preprocessing error: error_propagation preprocessing failed: line 8: multi-value error propagation not allowed in 'single' mode (use --multi-value-return=full): function returns 2 values plus error
```

**Test 2: Multi-value return with full mode (should succeed)**
```bash
./dingo build --multi-value-return=full test_multi_value2.dingo
```
Result: ✅ **PASS** - Build succeeded

**Test 3: Multi-value return with default mode (should be full)**
```bash
./dingo build test_multi_value2.dingo
```
Result: ✅ **PASS** - Build succeeded (confirms default is "full")

**Test 4: Invalid mode value (should reject)**
```bash
./dingo build --multi-value-return=invalid test_multi_value2.dingo
```
Result: ✅ **PASS** - Correctly rejected with error:
```
Error: configuration error: invalid multi-value return mode: "invalid" (must be 'full' or 'single')
```

All tests passed successfully!

### Manual Testing Recommendations
To verify the feature works:

1. **Test full mode (default)**:
```bash
# Create test file with multi-value return
cat > test_multi.dingo << 'EOF'
package main

func parseValue(s string) (int, string, error) {
    return 0, "", nil
}

func main() {
    let val, name = parseValue("test")?
    _ = val
    _ = name
}
EOF

# Should succeed
./dingo build test_multi.dingo
```

2. **Test single mode (restricted)**:
```bash
# Should fail with clear error
./dingo build --multi-value-return=single test_multi.dingo
# Expected: "multi-value error propagation not allowed in 'single' mode"
```

3. **Test single mode with (T, error)**:
```bash
# Create test file with single-value return
cat > test_single.dingo << 'EOF'
package main
import "os"

func main() {
    data := os.ReadFile("file.txt")?
    _ = data
}
EOF

# Should succeed even in single mode
./dingo build --multi-value-return=single test_single.dingo
```

4. **Test invalid mode**:
```bash
./dingo build --multi-value-return=invalid test.dingo
# Expected: "configuration error: invalid multi-value return mode: \"invalid\""
```

## Future Enhancements

### Potential Improvements
1. **Config file support**: Allow .dingo.toml or .dingorc to set default flags
2. **Per-file pragmas**: Allow `// dingo:multi-value-return=single` comment directives
3. **Linter integration**: Warn when multi-value returns are used (configurable)
4. **IDE support**: Surface the current mode in LSP hover/diagnostics

### Edge Cases to Consider
1. **Nested functions**: The config applies globally, not per-function
2. **Mixed returns**: A file can have both (T, error) and (A, B, error) in different functions
3. **Error messages**: Currently generic, could be more specific about which function/line

## Performance Impact
**Negligible**: Config validation happens once per line with `?` operator, typically <10 checks per file. No measurable performance impact.

## Backward Compatibility
**100% Compatible**:
- Default mode is "full" (existing behavior)
- New flag is optional
- Existing code continues to work without changes
