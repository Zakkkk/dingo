# Match-in-Assignment Bug: Alternative Solutions Analysis

## Problem Statement

**Current Bug**: `var result = __match_3 := opt` (line 62 in pattern_match_01_simple.go.golden)

**Root Cause**:
- Dingo treats `match` as an EXPRESSION (returns a value, like Rust/Swift)
- Go's `switch` is a STATEMENT (doesn't return a value)
- When match appears in assignment context (`result := match opt { ... }`), the preprocessor incorrectly generates both assignment operators

**Current Workaround**: IIFE wrapper pattern (proven, 2-3 hours, but "ugly")

```go
result := func() Option_int {
    __match_0 := opt
    switch __match_0.tag {
    case OptionTagSome:
        return Some(x * 2)  // Add 'return'
    case OptionTagNone:
        return None         // Add 'return'
    }
    panic("non-exhaustive match")
}()
```

## Why Go Switch Doesn't Return Values

Go's switch is fundamentally a **control flow statement**, not an expression:

```go
// ❌ This is INVALID Go
x := switch condition {
case A: 1
case B: 2
}

// ✅ This is VALID Go (but verbose)
var x int
switch condition {
case A: x = 1
case B: x = 2
}
```

## Alternative Solutions

### Alternative 1: Statement Sequence Transformation ⭐⭐⭐⭐⭐

**Concept**: Transform `result := match expr { ... }` into statement sequence with explicit assignments.

**Generated Code**:
```go
// Example 4: Pattern match in assignment context
func doubleIfPresent(opt Option_int) Option_int {
	var result Option_int  // Pre-declare result variable
	// DINGO_MATCH_START: opt
	switch opt.tag {
	case OptionTagSome:
		// DINGO_PATTERN: Some(x)
		x := *opt.some_0
		result = Some(x * 2)  // Assign to result
	case OptionTagNone:
		// DINGO_PATTERN: None
		result = None          // Assign to result
	}
	// DINGO_MATCH_END
	return result
}
```

**How It Works**:
1. Detect assignment context in preprocessor (already done: `isInAssignmentContext`)
2. Extract assignment target (`result`) from `result := match ...`
3. Pre-declare variable: `var result <type>`
4. Transform each arm's expression to assignment: `pattern => expr` → `result = expr`
5. No temp variable needed (`__match_N`), switch directly on scrutinee

**Implementation Steps**:
1. Parse assignment operator and variable name before `match`
2. Infer type of result (or use `interface{}` temporarily, plugin fixes later)
3. Emit `var result <type>` before match
4. Rewrite arm expressions to assignments
5. Remove temp variable logic when in assignment context

**Code Cleanliness**: ⭐⭐⭐⭐⭐ (5/5)
- Most idiomatic Go
- Zero function wrapper overhead
- Reads naturally
- Looks hand-written

**Implementation Complexity**: ⭐⭐⭐⭐ (4/5)
- Moderate: Need to parse assignment target
- Rewrite arm expressions (simple AST transformation)
- Type inference for variable declaration (can defer to plugin phase)
- Estimated: 3-4 hours

**Edge Cases**:
- ✅ Nested matches (each creates own result var with unique name)
- ✅ Multi-line expressions in arms (just wrap in assignment)
- ✅ Block expressions `{ ... }` (replace final expression with assignment)
- ⚠️ Type inference (might need `interface{}` + plugin phase type resolution)

**Example with Blocks**:
```go
var result int
switch status {
case Active:
	// DINGO_PATTERN: Active
	log.Println("active")
	result = 1  // Assignment from block
case Pending:
	// DINGO_PATTERN: Pending
	result = 0
}
```

---

### Alternative 2: Named Return Values Pattern ⭐⭐⭐

**Concept**: Use Go's named return values to simulate expression semantics.

**Generated Code**:
```go
func doubleIfPresent(opt Option_int) Option_int {
	// Helper function with named return
	result := func() (matched Option_int) {
		switch opt.tag {
		case OptionTagSome:
			x := *opt.some_0
			matched = Some(x * 2)  // Assign to named return
			return
		case OptionTagNone:
			matched = None
			return
		}
		panic("non-exhaustive")
	}()
	return result
}
```

**How It Works**:
1. Create IIFE with named return value `matched`
2. Each arm assigns to `matched`, then returns
3. Cleaner than explicit returns (no repeating type)

**Code Cleanliness**: ⭐⭐⭐ (3/5)
- Still uses IIFE wrapper (user's concern)
- Slightly cleaner than current IIFE (no repeated types)
- Not idiomatic Go

**Implementation Complexity**: ⭐⭐⭐⭐⭐ (5/5)
- Easy: Small modification to current IIFE approach
- Just add named return and implicit returns
- Estimated: 1 hour

**Edge Cases**:
- ✅ All same as IIFE approach
- ✅ Named returns well-understood in Go

**Trade-off**: Easier to implement but doesn't address user's "ugly wrapper" concern.

---

### Alternative 3: Comma-Ok Pattern Adaptation ⭐⭐

**Concept**: Generate a helper function that returns (value, ok) tuple.

**Generated Code**:
```go
func doubleIfPresent(opt Option_int) Option_int {
	var result Option_int
	var matched bool
	result, matched = func() (Option_int, bool) {
		switch opt.tag {
		case OptionTagSome:
			x := *opt.some_0
			return Some(x * 2), true
		case OptionTagNone:
			return None, true
		}
		return Option_int{}, false  // Exhaustive failure
	}()
	if !matched {
		panic("non-exhaustive match")
	}
	return result
}
```

**Code Cleanliness**: ⭐⭐ (2/5)
- Even more verbose than IIFE
- Extra boolean variable
- Not solving the core issue

**Implementation Complexity**: ⭐⭐⭐ (3/5)
- Moderate: Need to handle tuple returns
- Extra exhaustiveness check
- Estimated: 3 hours

**Edge Cases**:
- ⚠️ Adds runtime overhead (bool check)
- ⚠️ More verbose, not cleaner

**Verdict**: Rejected - worse than IIFE in all dimensions.

---

### Alternative 4: Variable Hoisting with Deferred Assignment ⭐⭐⭐⭐

**Concept**: Similar to Alt 1, but use closure-based initialization.

**Generated Code**:
```go
func doubleIfPresent(opt Option_int) Option_int {
	result := func(opt Option_int) Option_int {
		switch opt.tag {
		case OptionTagSome:
			x := *opt.some_0
			return Some(x * 2)
		case OptionTagNone:
			return None
		}
		panic("non-exhaustive")
	}(opt)
	return result
}
```

**How It Works**:
1. Wrap switch in IIFE that TAKES scrutinee as parameter
2. Use explicit returns (like current IIFE)
3. Call immediately with scrutinee

**Code Cleanliness**: ⭐⭐⭐ (3/5)
- Still IIFE wrapper (user's concern)
- At least reads like a transformation function
- Not idiomatic

**Implementation Complexity**: ⭐⭐⭐⭐⭐ (5/5)
- Easy: Just add parameter to current IIFE
- Estimated: 30 minutes

**Trade-off**: Doesn't solve "ugly wrapper" problem.

---

### Alternative 5: Two-Phase Transformation (Preprocessor + Plugin) ⭐⭐⭐⭐⭐

**Concept**: Use BOTH preprocessor AND AST plugin for cleaner separation.

**Phase 1 (Preprocessor)**: Emit marker + simple switch
```go
// DINGO_MATCH_ASSIGN: result
switch opt.tag {
case OptionTagSome:
	x := *opt.some_0
	Some(x * 2)  // Leave expression as-is
case OptionTagNone:
	None
}
// DINGO_MATCH_END
```

**Phase 2 (AST Plugin)**:
- Detect `DINGO_MATCH_ASSIGN` marker
- Insert `var result <type>` before switch
- Transform arm expressions to assignments: `expr` → `result = expr`

**Generated Code** (Final):
```go
var result Option_int  // Plugin inserts this
// DINGO_MATCH_ASSIGN: result
switch opt.tag {
case OptionTagSome:
	x := *opt.some_0
	result = Some(x * 2)  // Plugin transforms this
case OptionTagNone:
	result = None          // Plugin transforms this
}
// DINGO_MATCH_END
```

**Code Cleanliness**: ⭐⭐⭐⭐⭐ (5/5)
- Identical to Alternative 1
- Clean, idiomatic Go

**Implementation Complexity**: ⭐⭐⭐ (3/5)
- More complex: Two phases
- BUT each phase is simpler (separation of concerns)
- Preprocessor: Just emit marker + switch
- Plugin: AST transformation (already have go/types context)
- Estimated: 4-5 hours

**Edge Cases**:
- ✅ Same as Alternative 1
- ✅ Better type inference (AST plugin has go/types)
- ✅ Cleaner separation (preprocessor doesn't need type info)

**Trade-off**: More implementation work, but better architecture.

---

### Alternative 6: Preprocessor-Only with Interface{} ⭐⭐⭐⭐

**Concept**: Alternative 1 but use `interface{}` for unknown types.

**Generated Code**:
```go
var result interface{}  // Generic type, plugin can refine later
switch opt.tag {
case OptionTagSome:
	x := *opt.some_0
	result = Some(x * 2)
case OptionTagNone:
	result = None
}
// Plugin can later refine to: var result Option_int
```

**Code Cleanliness**: ⭐⭐⭐⭐ (4/5)
- Clean final output (after plugin refines type)
- Temporary `interface{}` acceptable

**Implementation Complexity**: ⭐⭐⭐⭐⭐ (5/5)
- Easier than Alt 5: Single phase
- Preprocessor emits `var result interface{}`
- Plugin refines type later (optional optimization)
- Estimated: 2-3 hours

**Edge Cases**:
- ✅ Works immediately with interface{}
- ✅ Can be refined later for type safety
- ✅ Go's type inference handles assignment

**Trade-off**: Slightly less type-safe until plugin phase, but simpler.

---

## Comparison Matrix

| Alternative | Cleanliness | Complexity | Time Est. | User Concern |
|-------------|-------------|------------|-----------|--------------|
| 1. Statement Sequence | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐ | 3-4h | ✅ Solves |
| 2. Named Returns | ⭐⭐⭐ | ⭐⭐⭐⭐⭐ | 1h | ❌ Still IIFE |
| 3. Comma-Ok | ⭐⭐ | ⭐⭐⭐ | 3h | ❌ Worse |
| 4. Hoisting + Closure | ⭐⭐⭐ | ⭐⭐⭐⭐⭐ | 0.5h | ❌ Still IIFE |
| 5. Two-Phase | ⭐⭐⭐⭐⭐ | ⭐⭐⭐ | 4-5h | ✅ Solves |
| 6. Interface{} | ⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ | 2-3h | ✅ Solves |

---

## What Other Transpilers Do

### TypeScript (JS Target)
Uses **statement sequences**:
```typescript
// TypeScript
result := match(x) {
  case A: 1,
  case B: 2
}

// Compiles to JavaScript
let result;
switch (x) {
  case A: result = 1; break;
  case B: result = 2; break;
}
```

### Rust (Native)
Has true expression-based match (not applicable).

### Borgo (Go Transpiler)
Uses **IIFE pattern** (same as current approach):
```borgo
x := match opt {
  Some(v) => v,
  None => 0
}

// Compiles to Go
x := func() int {
  switch opt.tag {
  case Some: return opt.value
  case None: return 0
  }
}()
```

### Kotlin (JVM Target)
Uses **statement sequences** (JVM switch is statement):
```kotlin
val result = when(x) {
  A -> 1
  B -> 2
}

// Compiles to JVM bytecode (conceptually)
int result;
switch (x) {
  case A: result = 1; break;
  case B: result = 2; break;
}
```

**Trend**: Most transpilers targeting statement-based languages (JS, JVM) use **statement sequences**, not IIFE wrappers.

---

## Recommendation Teaser

Based on this analysis, **Alternative 6 (Preprocessor-Only with Interface{})** or **Alternative 1 (Statement Sequence)** are the clear winners:
- Solve user's "ugly wrapper" concern
- Generate idiomatic Go code
- Reasonable implementation complexity
- Match industry patterns (TypeScript, Kotlin)

See `recommendation.md` for detailed implementation approach.
