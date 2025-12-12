# AST Bug Investigation Prompt

## Problem Statement

The Pattern Match Plugin in `pkg/plugin/builtin/pattern_match.go` has a statement positioning bug:

- **What works:** Plugin correctly builds an if-else chain from pattern match markers
- **What's broken:** The if-else chain is inserted at the wrong location in the AST
- **Impact:** All 13 pattern match golden tests fail (but code compiles successfully)

## Specific Issue

When transforming this pattern match:
```dingo
result := fetchData()
match result {
    Ok(x) => process(x),
    Err(e) => handleError(e)
}
```

The preprocessor generates markers and a switch statement:
```go
result := fetchData()
__match_0 := result  // This init statement gets lost!
switch __match_0.Tag {
case \"Ok\":
    // ...
}
```

The plugin builds this if-else chain (CORRECT):
```go
if __match_0.Tag == \"Ok\" {
    x := __match_0.Value.(int)
    process(x)
} else if __match_0.Tag == \"Err\" {
    e := __match_0.Value.(error)
    handleError(e)
} else {
    panic(\"non-exhaustive match\")
}
```

But the replacement logic doesn't preserve `__match_0 := result` and inserts the if-else chain at the wrong position.

## Code Context

**File:** `pkg/plugin/builtin/pattern_match.go`

**Relevant functions:**
- `transformPatternMatch()` - Main transformation logic
- AST manipulation code that replaces switch statements

**Current approach:**
1. Find switch statement with pattern match marker
2. Build if-else chain from pattern arms
3. Replace switch statement → **BUG IS HERE**

## Questions for Investigation

1. **Root Cause:** Why is the switch init statement (`__match_0 := result`) being lost?

2. **AST Structure:** What's the correct way to replace a switch statement while preserving its init statement in Go's AST?

3. **Fix Strategy:** Should we:
   - a) Preserve the switch init and insert it before the if-else chain?
   - b) Convert the switch (with init) to a block statement containing assignment + if-else?
   - c) Different approach?

4. **Go AST Pattern:** What's the idiomatic Go AST pattern for this transformation?
   ```
   // From:
   switch init; tag {
   case \"A\": ...
   case \"B\": ...
   }

   // To:
   init
   if tag == \"A\" {
       ...
   } else if tag == \"B\" {
       ...
   }
   ```

5. **Testing:** How can we verify the fix works without breaking other functionality?

## Expected Response

Please provide:
1. **Root cause analysis** - Why is the bug happening?
2. **Proposed solution** - Concrete fix with code examples
3. **Implementation steps** - How to apply the fix
4. **Risks** - What could go wrong?
5. **Testing approach** - How to verify it works

## Files to Reference

- `pkg/plugin/builtin/pattern_match.go` - Plugin implementation
- `pkg/preprocessor/rust_match.go` - Preprocessor that generates markers
- `tests/golden/pattern_match_01_basic.dingo` - Example test case
- `tests/golden/pattern_match_01_basic.go.golden` - Expected output

## Constraints

- Must preserve Go's AST structure validity
- Must maintain exhaustiveness checking
- Should not break guards or tuple destructuring
- Performance: &lt;1ms overhead per transformation
