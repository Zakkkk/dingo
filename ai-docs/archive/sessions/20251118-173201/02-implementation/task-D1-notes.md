# Task D1: Pattern Guards Golden Tests - Design Notes

## Test Design Philosophy

### Goal: Comprehensive Coverage Without Redundancy

Each test demonstrates unique aspects of guard behavior:
- **Test 05**: Foundation - basic guard syntax and fallthrough
- **Test 06**: Keyword variety - 'where' keyword and nested patterns
- **Test 07**: Real-world complexity - HTTP routing, method calls, mixed keywords
- **Test 08**: Boundary testing - edge cases and stress tests

### Why These Specific Tests?

**Test 05 (Basic Guards):**
- **Purpose**: First exposure to guard syntax
- **Why**: Users need simple examples to understand the concept
- **Key Learning**: Guards are runtime checks, multiple guards on same variant allowed
- **Real-world scenario**: Input validation (age checks, number classification)

**Test 06 (Nested with 'where'):**
- **Purpose**: Swift-style syntax and nested pattern interaction
- **Why**: Swift developers will look for 'where' keyword support
- **Key Learning**: Both keywords work identically, guards work with nested patterns
- **Real-world scenario**: Processing nested Result[Option[T]] data structures
- **Design choice**: FizzBuzz example shows guard precedence clearly

**Test 07 (Complex Guards):**
- **Purpose**: Production-level guard usage patterns
- **Why**: Developers need to see guards with real Go idioms (strings package, etc.)
- **Key Learning**: Guards can use any Go expression (method calls, complex boolean logic)
- **Real-world scenario**: HTTP request routing, path validation, multi-field matching
- **Design choice**: Mixed if/where demonstrates both keywords in same match

**Test 08 (Edge Cases):**
- **Purpose**: Stress test and boundary conditions
- **Why**: Ensure implementation handles unusual but valid patterns
- **Key Learning**: Guards have no artificial limits, wildcard can be guarded
- **Real-world scenario**: Fine-grained classification (many specific cases)
- **Design choice**: 11 guards shows scalability, side-effect guard shows not recommended but valid

## Design Decisions

### Enum Types Used

**Result[T,E] (Tests 05, 06):**
- Most common pattern in Go (T, error) mappings
- Guards naturally apply to Ok/Err distinction
- Users familiar with Result from Rust will understand immediately

**Option[T] (Test 06):**
- Demonstrates guards on nullable types
- Shows Some/None pattern guards
- Nested with Result shows real-world complexity

**Custom Enums (Tests 07, 08):**
- HTTP Request enum shows multi-field variants
- Status enum shows guards on different variant shapes
- Value enum (Small/Medium/Large) shows many guards on single type

### Guard Expression Complexity Progression

**Simple (Test 05):**
```dingo
x > 0           // Single comparison
x < 0           // Another simple comparison
isEven(n)       // Simple function call
```

**Intermediate (Test 06):**
```dingo
x > 100                        // Threshold checks
x%3 == 0 && x%5 == 0          // Multiple conditions
val > 0                        // Guards on nested patterns
```

**Advanced (Test 07):**
```dingo
strings.HasPrefix(path, "/api/")               // Method calls
len(body) > 100 && strings.Contains(path, "upload")  // Multiple methods
count > 1000 || count < 0                      // Logical OR
```

**Edge Cases (Test 08):**
```dingo
true                           // Constant condition
1 > 0                          // Constant expression
increment(&count) && x > 0     // Side effects
x == 1, x == 2, x == 3, ...   // Many specific cases
```

### Keyword Choice Strategy

**'if' keyword (Rust-style):**
- Used in Tests 05, 07, 08
- Primary keyword (matches Rust, more common in systems languages)
- Chosen for basic examples to establish as default

**'where' keyword (Swift-style):**
- Used in Tests 06, 07
- Secondary keyword (matches Swift, more readable in some contexts)
- Test 06 uses exclusively to show parity
- Test 07 mixes both to show interoperability

**Design rationale:**
- Test 05 establishes 'if' as default
- Test 06 shows 'where' is equivalent
- Test 07 shows both can be mixed freely
- Test 08 uses 'if' to reinforce it as primary

### Nested If Strategy Validation

All golden files demonstrate the **nested if strategy** (user decision):

```go
case ResultTag_Ok:
    x := *__match_0.ok_0
    if x > 0 {
        return "positive"
    }
    // Guard failed - fallthrough to next case
```

**Why nested if (not goto labels):**
1. **Safer**: No label collision in nested matches
2. **Debuggable**: Standard if statements, easy to step through
3. **Simpler AST**: No label nodes, just if nodes
4. **Go compiler optimizes**: Generates same machine code as goto

**What we're NOT doing (rejected goto strategy):**
```go
case ResultTag_Ok:
    x := *__match_0.ok_0
    if !(x > 0) { goto __match_fallthrough_0 }
    return "positive"
__match_fallthrough_0:
```

### Exhaustiveness Handling

All tests demonstrate: **Guards do NOT satisfy exhaustiveness**.

**Examples in tests:**

**Test 05 - classifyNumber():**
```dingo
Result_Ok(x) if x > 0 => "positive",    // Guard
Result_Ok(x) if x < 0 => "negative",    // Guard
Result_Ok(_) => "zero",                 // Catchall (REQUIRED)
Result_Err(e) => "error",               // Complete coverage
```

Without `Result_Ok(_)`, match would be non-exhaustive even with guards.

**Test 08 - classify():**
```dingo
Value_Small(x) if x > 10 => ...,    // Guard
Value_Medium(x) if x > 100 => ...,  // Guard
Value_Large(x) if x > 1000 => ...,  // Guard
_ => "normal range",                 // Wildcard REQUIRED
```

All patterns guarded → wildcard is mandatory.

**Rationale:**
- Guards are runtime checks (condition might always fail)
- Exhaustiveness is compile-time guarantee
- Only pattern structure matters for exhaustiveness, not guard conditions

## Real-World Scenarios

### Test 05: Input Validation

```dingo
func validateAge(result Result) string {
    return match result {
        Result_Ok(age) if age >= 18 && age < 65 => "adult",
        Result_Ok(age) if age >= 65 => "senior",
        Result_Ok(_) => "minor",
        Result_Err(_) => "invalid",
    }
}
```

**Use case**: Form validation, user registration, age restrictions
**Why guards**: Different age ranges need different handling
**Alternative without guards**: Verbose if-else chains

### Test 06: FizzBuzz

```dingo
func categorize(opt Option) string {
    return match opt {
        Option_Some(x) where x%3 == 0 && x%5 == 0 => "fizzbuzz",
        Option_Some(x) where x%3 == 0 => "fizz",
        Option_Some(x) where x%5 == 0 => "buzz",
        Option_Some(x) => "number",
        Option_None => "empty",
    }
}
```

**Use case**: Classic programming problem, rule-based systems
**Why guards**: Demonstrates precedence (first match wins)
**Alternative without guards**: Nested if-else inside match

### Test 07: HTTP Routing

```dingo
func routeRequest(req Request) string {
    return match req {
        Request_Get(path) if strings.HasPrefix(path, "/api/") => "API endpoint",
        Request_Get(path) where len(path) > 0 => "static resource",
        Request_Post(path, body) if len(body) > 100 && strings.Contains(path, "upload") => "large upload",
        Request_Post(_, body) where len(body) > 0 => "post request",
        Request_Delete(path) if isProtected(path) => "forbidden",
        Request_Delete(_) => "delete request",
        _ => "unknown",
    }
}
```

**Use case**: Web servers, API routing, middleware
**Why guards**: Different paths/methods need different handling
**Alternative without guards**: Massive if-else tree, less readable

### Test 08: Fine-Grained Classification

```dingo
func granularClassify(val Value) string {
    return match val {
        Value_Small(x) if x == 1 => "one",
        Value_Small(x) if x == 2 => "two",
        Value_Small(x) if x == 3 => "three",
        // ... 8 more guards ...
        Value_Small(_) => "small other",
        Value_Medium(_) => "medium",
        Value_Large(_) => "large",
    }
}
```

**Use case**: State machines, protocol parsing, fine-grained categorization
**Why guards**: Many specific cases, guards more readable than if-else
**Performance**: 11 guards compile efficiently (not exponential)

## Golden File Quality

### Idiomatic Go Output

All golden files generate **compilable, idiomatic Go**:
- ✅ gofmt formatted
- ✅ No runtime library dependencies
- ✅ Standard Go patterns (switch/case, if statements)
- ✅ Proper error handling (panic on unreachable)

### Marker Consistency

**Format**: `// DINGO_PATTERN: Pattern | DINGO_GUARD: condition`

**Examples from tests:**
```go
// DINGO_PATTERN: Ok(x) | DINGO_GUARD: x > 0
// DINGO_PATTERN: Some(x) | DINGO_GUARD: x%3 == 0 && x%5 == 0
// DINGO_PATTERN: Get(path) | DINGO_GUARD: strings.HasPrefix(path, "/api/")
```

**Why this format:**
- Clear separation between pattern and guard
- Easy to parse (split on " | DINGO_GUARD: ")
- Human-readable for debugging
- Consistent with other DINGO markers

### Compilation Verification

All golden files **must compile**:
- TestGoldenFilesCompilation runs `go/parser` on each golden file
- Ensures generated code is syntactically valid Go
- Catches issues like missing imports, syntax errors
- All 4 new tests pass: ✅

## Test Maintenance Guidelines

### When to Update These Tests

**If guard syntax changes:**
- Update Dingo source to match new syntax
- Regenerate golden files with transpiler
- Verify all 4 tests still pass

**If nested if strategy changes:**
- Update golden files to match new code generation
- Example: If switching to goto labels, replace if statements with gotos
- Marker format might change

**If exhaustiveness rules change:**
- Update test comments to explain new rules
- May need additional tests if guards can satisfy exhaustiveness

### Adding New Guard Tests

**When to add:**
- New guard syntax introduced (e.g., 'unless' keyword)
- New guard interaction discovered (e.g., guards with pattern aliases)
- Performance regression found (need benchmark test)

**Naming:**
- pattern_match_09_guards_{description}.dingo
- Continue sequential numbering
- Keep descriptive names (guards_negation, guards_aliases, etc.)

## Known Limitations (Documented)

### 1. Guard Side Effects

**Test 08 demonstrates but not recommended:**
```dingo
Value_Small(x) if increment(&count) && x > 0 => ...
```

**Why not recommended:**
- Guards may be evaluated multiple times during fallthrough
- Order of evaluation not guaranteed
- Makes code harder to reason about

**Better approach:**
- Evaluate side effects before match
- Use guard only for pure boolean conditions

### 2. Constant Guards

**Test 08 demonstrates optimization opportunity:**
```dingo
Value_Small(_) where true => "small"
```

**Current behavior:** Guard evaluates at runtime (if true)
**Optimization opportunity:** Compiler could detect constant guards and eliminate check
**Not implemented yet:** Marked as future optimization

### 3. Complex Nested Guards

**Test 06 shows nested Result[Option[T]] with guards:**
```dingo
Result_Ok(Option_Some(val)) where val > 0 => ...
```

**Current limitation:** Guard variable scoping might be tricky
**Implementation note:** Plugin must ensure 'val' is in scope for guard

## Cross-Reference with Implementation

### Preprocessor (Task A1)

**What preprocessor must do:**
1. Parse both 'if' and 'where' keywords ✅ (Tests 05, 06, 07)
2. Extract guard condition text ✅ (All tests)
3. Generate DINGO_GUARD marker ✅ (All tests)
4. Handle complex expressions ✅ (Tests 06, 07, 08)

**Test verification:**
- Test 05: `splitPatternAndGuard()` must extract "x > 0"
- Test 06: Must parse "x%3 == 0 && x%5 == 0" correctly
- Test 07: Must handle "strings.HasPrefix(path, "/api/")"

### Plugin (Task B2)

**What plugin must do:**
1. Find DINGO_GUARD markers ✅ (All tests)
2. Parse condition as Go expression ✅ (All tests)
3. Inject nested if statement ✅ (All tests)
4. No else clause (fallthrough) ✅ (All tests)
5. Ignore guards for exhaustiveness ✅ (Test 08 classify() function)

**Test verification:**
- Test 05: Nested if wraps return statement
- Test 06: Multiple guards on same variant generate multiple cases
- Test 07: Complex guard conditions parse correctly
- Test 08: All-guards-fail case hits wildcard

## Success Metrics

**Coverage:**
- ✅ 2 guard keywords (if, where)
- ✅ 4 complexity levels (simple, intermediate, advanced, edge)
- ✅ 5 pattern types (simple, nested, multi-field, wildcard, guarded wildcard)
- ✅ 3 real-world scenarios (validation, routing, classification)

**Quality:**
- ✅ All golden files compile
- ✅ Progressive complexity (05 → 06 → 07 → 08)
- ✅ Realistic examples (not contrived)
- ✅ Clear documentation (comments, test descriptions)

**Alignment:**
- ✅ Follows golden test guidelines
- ✅ Matches final-plan.md design (nested if strategy)
- ✅ Matches user decisions (both keywords, ignore guards for exhaustiveness)
- ✅ Integration-ready (preprocessor + plugin tested)

## Future Enhancements

**Potential additional tests:**

1. **Guards with pattern aliases** (if implemented):
   ```dingo
   Result_Ok(x @ y) if x > 0 => ...
   ```

2. **Guards with or-patterns** (if implemented):
   ```dingo
   (Ok(x) | Some(x)) if x > 0 => ...
   ```

3. **Guards with negative conditions** (syntax sugar):
   ```dingo
   Ok(x) unless x <= 0 => ...  // Same as: if x > 0
   ```

4. **Guard benchmarks** (performance tests):
   - Compare 11-guard match vs if-else chain
   - Measure constant guard elimination optimization

**Not needed now** - current 4 tests provide comprehensive coverage for Phase 4.2.
