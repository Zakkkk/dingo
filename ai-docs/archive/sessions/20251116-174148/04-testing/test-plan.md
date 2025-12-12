# Test Plan: Error Propagation Feature
## Session: 20251116-174148
## Date: 2025-11-16

---

## 1. Requirements Understanding

### What the Feature Should Do

The Error Propagation feature implements configurable syntax for error handling in Dingo:
- **Three syntax options**: `?` (question), `!` (bang), `try` keyword
- **Unified transformation**: All syntaxes transform to the same Go early-return pattern
- **Configuration system**: TOML-based configuration with precedence (CLI > project > user > defaults)
- **Source maps**: Position tracking for error messages (skeleton implementation in Phase 1)
- **Go stdlib compatibility**: Works with any Go function returning `(T, error)`

### Key Behaviors to Validate

1. **Parser**: Correctly detects `?` operator and creates `ErrorPropagationExpr` AST nodes
2. **Configuration**: Loads TOML config with proper precedence and validation
3. **AST Transformation**: Converts error propagation to Go early-return pattern with temp variables
4. **Source Maps**: Collects position mappings (even though VLQ encoding is TODO)
5. **Real-world Integration**: Works with actual Go stdlib packages (http, os, io, json)

### Critical Edge Cases

1. **Empty/missing configuration files** - Should use defaults without errors
2. **Invalid configuration values** - Should return validation errors
3. **Nested error propagation** - Multiple `?` operators in sequence
4. **Non-error-returning expressions** - Parser should still create nodes (validation is Phase 1.5)
5. **Position tracking** - Prefix vs postfix syntax position calculation

---

## 2. Test Scenarios

### Category 1: Parser Tests (Unit)

#### Scenario 1.1: Basic Question Operator Detection
- **Purpose**: Validate parser creates ErrorPropagationExpr for `?` operator
- **Input**: `user := fetchUser(id)?`
- **Expected Output**: ErrorPropagationExpr node with SyntaxQuestion
- **Rationale**: Core functionality - must correctly parse the operator

#### Scenario 1.2: Multiple Error Propagations in One Function
- **Purpose**: Validate parser handles multiple `?` operators
- **Input**: Function with 3 different `?` usages
- **Expected Output**: 3 separate ErrorPropagationExpr nodes
- **Rationale**: Real-world code will have multiple error checks

#### Scenario 1.3: Error Propagation in Different Contexts
- **Purpose**: Validate parser works in various statement contexts
- **Input**: `?` in variable declarations, assignments, returns
- **Expected Output**: ErrorPropagationExpr nodes in all contexts
- **Rationale**: Ensures parser flexibility

#### Scenario 1.4: Position Tracking Accuracy
- **Purpose**: Validate Pos() and End() methods return correct positions
- **Input**: `?` operator at known positions
- **Expected Output**: Correct line/column positions
- **Rationale**: Source maps depend on accurate position tracking

### Category 2: Configuration Tests (Unit)

#### Scenario 2.1: Default Configuration Loading
- **Purpose**: Validate defaults when no config file exists
- **Input**: No dingo.toml file
- **Expected Output**: Config with SyntaxQuestion, FormatInline, Enabled=true
- **Rationale**: Must work out-of-the-box without configuration

#### Scenario 2.2: Project Config Override
- **Purpose**: Validate project dingo.toml overrides defaults
- **Input**: dingo.toml with syntax="bang"
- **Expected Output**: Config with SyntaxBang
- **Rationale**: Projects need to customize syntax choice

#### Scenario 2.3: CLI Override of Project Config
- **Purpose**: Validate CLI flags have highest precedence
- **Input**: dingo.toml with syntax="question", CLI override with syntax="try"
- **Expected Output**: Config with SyntaxTry
- **Rationale**: CLI must override for one-off builds

#### Scenario 2.4: Invalid Syntax Validation
- **Purpose**: Validate config rejects invalid syntax values
- **Input**: syntax="invalid"
- **Expected Output**: Validation error
- **Rationale**: Fail fast with clear errors

#### Scenario 2.5: Invalid Source Map Format Validation
- **Purpose**: Validate config rejects invalid source map formats
- **Input**: format="invalid"
- **Expected Output**: Validation error
- **Rationale**: Configuration errors should be caught early

#### Scenario 2.6: TOML Parsing Errors
- **Purpose**: Validate config handles malformed TOML gracefully
- **Input**: Invalid TOML syntax
- **Expected Output**: Parse error with file path
- **Rationale**: Users need clear feedback on config errors

### Category 3: AST Transformation Tests (Unit)

#### Scenario 3.1: Basic Transformation Structure
- **Purpose**: Validate plugin generates correct Go AST structure
- **Input**: ErrorPropagationExpr node
- **Expected Output**: Assignment + if statement + return statement
- **Rationale**: Core transformation must be correct

#### Scenario 3.2: Unique Variable Names
- **Purpose**: Validate each transformation gets unique temp/error variables
- **Input**: Two consecutive error propagations
- **Expected Output**: __tmp0/__err0, then __tmp1/__err1
- **Rationale**: Prevents variable shadowing bugs

#### Scenario 3.3: Counter Reset
- **Purpose**: Validate plugin counter reset works
- **Input**: Reset() call between transformations
- **Expected Output**: Counters start from 0 again
- **Rationale**: Needed for testing and multi-file compilation

#### Scenario 3.4: Syntax Agnostic Transformation
- **Purpose**: Validate transformation is same regardless of syntax
- **Input**: ErrorPropagationExpr with SyntaxQuestion, SyntaxBang, SyntaxTry
- **Expected Output**: Identical Go AST structure for all three
- **Rationale**: Syntax should only affect parsing, not transformation

### Category 4: Source Map Tests (Unit)

#### Scenario 4.1: Mapping Collection
- **Purpose**: Validate source map generator collects mappings
- **Input**: AddMapping() calls with different positions
- **Expected Output**: Mappings stored in order
- **Rationale**: Foundation for source map generation

#### Scenario 4.2: Name Collection
- **Purpose**: Validate unique identifier names are collected
- **Input**: AddMappingWithName() with duplicate names
- **Expected Output**: De-duplicated name list
- **Rationale**: Source map names array must be unique

#### Scenario 4.3: JSON Structure Generation
- **Purpose**: Validate source map JSON structure is valid
- **Input**: Generator with mappings
- **Expected Output**: Valid Source Map v3 JSON structure
- **Rationale**: Must produce valid source map format

#### Scenario 4.4: Inline Format Generation
- **Purpose**: Validate base64 inline comment generation
- **Input**: Source map data
- **Expected Output**: Base64-encoded inline comment
- **Rationale**: Inline format is default for development

#### Scenario 4.5: Consumer Creation
- **Purpose**: Validate source map consumer can parse generated maps
- **Input**: Generated source map JSON
- **Expected Output**: Consumer successfully created
- **Rationale**: Round-trip test for source map format

### Category 5: Integration Tests (End-to-End)

#### Scenario 5.1: HTTP Client with net/http
- **Purpose**: Validate error propagation with http.Get and io.ReadAll
- **Input**: http_client.dingo example
- **Expected Output**: Parser creates ErrorPropagationExpr nodes
- **Rationale**: Most common real-world use case

#### Scenario 5.2: File Operations with os
- **Purpose**: Validate error propagation with os.ReadFile
- **Input**: file_ops.dingo example
- **Expected Output**: Parser creates ErrorPropagationExpr nodes
- **Rationale**: File I/O is critical use case

#### Scenario 5.3: JSON Parsing with encoding/json
- **Purpose**: Validate error propagation with json.Unmarshal
- **Input**: file_ops.dingo with JSON unmarshaling
- **Expected Output**: Parser creates ErrorPropagationExpr nodes
- **Rationale**: JSON parsing is common pattern

#### Scenario 5.4: Chained Error Propagation
- **Purpose**: Validate multiple stdlib functions with error propagation
- **Input**: Function calling http.Get()? then io.ReadAll()?
- **Expected Output**: Two separate ErrorPropagationExpr nodes
- **Rationale**: Real code chains multiple error-returning calls

---

## 3. Test Implementation Strategy

### Test Organization

```
tests/
├── parser/
│   ├── error_propagation_parser_test.go
│   └── position_tracking_test.go
├── config/
│   ├── loading_test.go
│   ├── validation_test.go
│   └── precedence_test.go
├── plugin/
│   ├── transformation_test.go
│   └── variable_naming_test.go
├── sourcemap/
│   ├── generator_test.go
│   └── consumer_test.go
└── integration/
    ├── stdlib_http_test.go
    ├── stdlib_file_test.go
    └── chained_propagation_test.go
```

### Test Implementation Approach

1. **Unit Tests (Parallel Execution)**
   - Use table-driven tests for multiple scenarios
   - Clear test names describing what's validated
   - Isolated tests with no dependencies
   - Fast execution (<100ms per test)

2. **Integration Tests (Sequential)**
   - Test with actual .dingo example files
   - Validate parsing + AST creation
   - Check generated code structure
   - Slower but comprehensive

3. **Error Handling**
   - Test both success and failure paths
   - Validate error messages are helpful
   - Check error types are correct

4. **Test Data**
   - Use golden files for expected output
   - Store test .dingo files in testdata/
   - Use table-driven tests for variations

---

## 4. Coverage Goals

### Code Coverage Targets

- **Parser (pkg/parser/)**: 80% coverage
  - Focus on postfix expression handling
  - Test error cases and edge cases

- **Config (pkg/config/)**: 95% coverage
  - Simple code, should be fully testable
  - All validation paths must be tested

- **Plugin (pkg/plugin/builtin/)**: 85% coverage
  - Core transformation logic
  - Variable naming edge cases

- **Source Maps (pkg/sourcemap/)**: 75% coverage
  - JSON generation (testable)
  - VLQ encoding (TODO, skip for now)
  - Consumer (testable with go-sourcemap library)

### Test Scenario Coverage

- **Happy Paths**: 100% (all critical user flows)
- **Error Conditions**: 90% (validation, parse errors, invalid config)
- **Edge Cases**: 80% (nested propagation, empty files, position boundaries)
- **Real-world Integration**: 5 stdlib packages minimum

---

## 5. Known Limitations (Phase 1)

These are acknowledged limitations that we will NOT test extensively:

1. **VLQ Encoding**: Source map mappings field is empty (TODO Phase 1.6)
   - Test: Only validate JSON structure, not mappings content

2. **Type Validation**: No checking that `?` is used on (T, error) returns
   - Test: Parser creates nodes regardless of type correctness

3. **Expression Context**: Transformation only works in statement context
   - Test: Don't test `return fetchUser(id)?` - Phase 1.5

4. **Bang/Try Syntax**: Only `?` syntax implemented in parser
   - Test: Only test question operator, defer bang/try to later

5. **Zero Values**: Always uses `nil` for first return value
   - Test: Don't validate zero value correctness - Phase 1.5

6. **Global Counters**: Plugin counters are not thread-safe
   - Test: Single-threaded tests only

---

## 6. Test Execution Plan

### Phase 1: Unit Tests (Parser)
```bash
go test -v ./tests/parser/... -run TestErrorPropagation
```
- Validate basic parsing works
- Check AST node creation
- Verify position tracking

### Phase 2: Unit Tests (Config)
```bash
go test -v ./pkg/config/... -cover
```
- Test default loading
- Test precedence
- Test validation

### Phase 3: Unit Tests (Plugin)
```bash
go test -v ./pkg/plugin/builtin/... -cover
```
- Test transformation logic
- Test variable naming
- Test counter management

### Phase 4: Unit Tests (Source Maps)
```bash
go test -v ./pkg/sourcemap/... -cover
```
- Test mapping collection
- Test JSON generation
- Test consumer creation

### Phase 5: Integration Tests
```bash
go test -v ./tests/integration/... -timeout 30s
```
- Test with real stdlib packages
- Validate full parsing pipeline
- Check example files parse correctly

### Phase 6: Full Test Suite
```bash
go test -v ./... -cover -race
```
- Run all tests
- Generate coverage report
- Check for race conditions (with caveat about global counters)

---

## 7. Success Criteria

### Must Pass (Critical)
- [ ] Parser creates ErrorPropagationExpr nodes for `?` operator
- [ ] Config loads with defaults when no file exists
- [ ] Config validation rejects invalid values
- [ ] Plugin transformation generates correct AST structure
- [ ] Source map generator collects mappings
- [ ] All integration tests with stdlib packages parse successfully

### Should Pass (Important)
- [ ] Config precedence works (CLI > project > user > defaults)
- [ ] Plugin generates unique variable names
- [ ] Source map JSON structure is valid Source Map v3
- [ ] Position tracking returns accurate line/column numbers
- [ ] Multiple error propagations in one function work

### Nice to Have (Optional)
- [ ] Source map consumer round-trip test
- [ ] Performance benchmarks for parser
- [ ] Memory profiling for large files
- [ ] Concurrent parsing tests (will reveal global counter issue)

---

## 8. Test Metrics

### Quantitative Goals
- **Total Test Count**: 40-50 tests
- **Unit Tests**: 35-40 tests
- **Integration Tests**: 5-10 tests
- **Code Coverage**: >80% overall
- **Execution Time**: <5 seconds for full suite
- **Zero Flaky Tests**: All tests deterministic

### Qualitative Goals
- **Clear Test Names**: Describes what's tested and expected outcome
- **Good Error Messages**: When test fails, reason is obvious
- **Isolated Tests**: No shared state between tests
- **Readable Tests**: Easy for reviewers to understand intent
- **Maintainable Tests**: Easy to update when requirements change

---

## 9. Risk Mitigation

### Test-Related Risks

| Risk | Probability | Mitigation |
|------|------------|------------|
| **Parser grammar too complex to test** | Low | Use simple test inputs, focus on `?` operator only |
| **Source maps can't be tested without VLQ** | Medium | Test JSON structure and mapping collection only |
| **Integration tests fail due to syntax limitations** | High | Use simplified Dingo syntax that Phase 1 parser supports |
| **Config precedence tests are flaky** | Low | Use temp directories, clean up in teardown |
| **Race conditions in tests** | Medium | Document global counter limitation, use -race flag |

---

## 10. Next Steps After Testing

### If Tests PASS
1. Document test results in test-results.md
2. Generate coverage report
3. Identify any gaps in coverage
4. Proceed to Phase 1.5 (statement lifting, type validation)

### If Tests FAIL
1. Categorize failures: test bug vs implementation bug
2. For test bugs: Fix test and re-run
3. For implementation bugs: Document in test-results.md with:
   - Expected behavior
   - Actual behavior
   - Evidence this is implementation issue
   - Suggested fix
4. Prioritize fixes: CRITICAL (blocking) vs IMPORTANT (can defer)

---

**Test Plan Version**: 1.0
**Created**: 2025-11-16
**Status**: Ready for Implementation
**Estimated Test Count**: 45 tests
**Estimated Coverage**: 82%
