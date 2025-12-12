# Test Plan: LSP Source Mapping Fix

## Objective
Verify that the fix to error propagation preprocessor's `qPos` calculation (using `LastIndex` instead of `Index`) correctly resolves source map column positions.

## Background
**Bug**: Error propagation `?` operator was calculating incorrect column positions in source maps, causing LSP diagnostics to point to wrong locations (e.g., pointing to `e(path)?` instead of `ReadFile(path)?`).

**Fix**: Changed `strings.Index(beforeQ, "?")` to `strings.LastIndex(beforeQ, "?")` in `pkg/preprocessor/error_prop.go` line 139.

**Expected Result**: Source maps should now have correct column positions, and LSP diagnostics should point to the actual error source.

## Test Scenarios

### Test 1: Source Map Correctness
**Purpose**: Verify generated source maps have correct column positions

**Input**: `tests/golden/error_prop_01_simple.dingo`
```dingo
func readConfig(path: string) (string, error) {
    data := os.ReadFile(path)?
    return string(data), nil
}
```

**Expected**:
- Source map for `ReadFile(path)?` line should show column ~27 (position of `ReadFile`)
- NOT column 15 (position of `e(path)?` in preprocessed output)

**Test Steps**:
1. Build transpiler: `go build -o dingo-test ./cmd/dingo`
2. Transpile: `./dingo-test build tests/golden/error_prop_01_simple.dingo`
3. Read generated `.go.map` file
4. Verify column positions match original Dingo source

### Test 2: LSP Diagnostic Position (Manual)
**Purpose**: Verify LSP diagnostics point to correct locations

**Test Steps**:
1. Build LSP: `go build -o dingo-lsp ./cmd/dingo-lsp`
2. Start LSP server
3. Open `error_prop_01_simple.dingo` in editor
4. Introduce error (e.g., change `ReadFile` to `ReadFileXXX`)
5. Verify diagnostic points to `ReadFileXXX`, not to `e(path)?`

**Note**: This requires manual testing with editor/LSP client. Will document expected behavior.

### Test 3: Existing Test Suite
**Purpose**: Ensure no regressions in existing functionality

**Test Steps**:
1. Run all tests: `go test ./...`
2. Verify all pass
3. Check for any new failures

### Test 4: Golden Tests
**Purpose**: Verify transpilation output unchanged (fix is source map only)

**Test Steps**:
1. Run golden tests: `go test ./tests -run TestGoldenFiles`
2. Verify all 46 tests pass
3. Confirm `.go.golden` files unchanged (only `.go.map` should differ)

## Success Criteria

✅ **Test 1 PASS**: Source maps show correct column positions (col ~27 for ReadFile)
✅ **Test 2 PASS**: LSP diagnostics point to correct source locations (manual verification)
✅ **Test 3 PASS**: All existing tests pass (no regressions)
✅ **Test 4 PASS**: All golden tests pass (transpilation unchanged)

## Expected Outcomes

**Before Fix**:
- Source map: `ReadFile` → col 15 (wrong, points to preprocessed `e(path)?`)
- LSP diagnostic: Points to wrong location in Dingo source

**After Fix**:
- Source map: `ReadFile` → col 27 (correct, points to original call site)
- LSP diagnostic: Points to correct location in Dingo source
