# ENHANCED Investigation: "No Pattern Arms Found" Bug
## Round 2 - Full Code Context Provided

---

## Problem Statement

**Error**: `parse error: rust_match preprocessing failed: line XX: parsing pattern arms: no pattern arms found`

**Affected**: 6 pattern matching tests failing
**Current Status**: 95/103 tests passing (92.2%)
**Goal**: Fix bug to reach 98%+ passing

---

## Previous Round Issues (Why We're Re-Running)

**Round 1 Problems**:
1. External models had insufficient code context
2. Grok Code Fast hallucinated test results (claimed tests passing when they weren't)
3. Output was too brief (9-57 lines) for complex bug analysis
4. Models couldn't validate hypotheses against actual code

**This Round**: Full code provided for thorough analysis

---

## FULL CODE CONTEXT

### 1. Preprocessor Code (rust_match.go)

```go
package preprocessor

import (
	"bytes"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// RustMatchProcessor handles Rust-like pattern matching syntax
// Transforms: match expr { Pattern => expression, ... } → Go switch statement with markers
type RustMatchProcessor struct {
	matchCounter int
	mappings     []Mapping
}

// Pattern-matching regex for Rust-like match expressions
var (
	// Match the entire match expression: match expr { ... }
	matchExprPattern = regexp.MustCompile(`(?s)match\s+([^{]+)\s*\{(.+)\}`)
)

// NewRustMatchProcessor creates a new Rust-like match preprocessor
func NewRustMatchProcessor() *RustMatchProcessor {
	return &RustMatchProcessor{
		matchCounter: 0,
		mappings:     []Mapping{},
	}
}

// Name returns the processor name
func (r *RustMatchProcessor) Name() string {
	return "rust_match"
}

// Process transforms Rust-like match expressions
func (r *RustMatchProcessor) Process(source []byte) ([]byte, []Mapping, error) {
	r.mappings = []Mapping{}
	r.matchCounter = 0

	input := string(source)
	lines := strings.Split(input, "\n")

	var output bytes.Buffer
	inputLineNum := 0
	outputLineNum := 1

	for inputLineNum < len(lines) {
		line := lines[inputLineNum]

		// Detect match expressions
		// Look for "match " keyword (with space to avoid matching "matchmaker" etc.)
		if strings.Contains(line, "match ") {
			// Collect the complete match expression (might span multiple lines)
			matchExpr, linesConsumed := r.collectMatchExpression(lines, inputLineNum)
			if matchExpr != "" {
				// Transform the match expression
				transformed, newMappings, err := r.transformMatch(matchExpr, inputLineNum+1, outputLineNum)
				if err != nil {
					return nil, nil, fmt.Errorf("line %d: %w", inputLineNum+1, err)
				}

				output.WriteString(transformed)
				r.mappings = append(r.mappings, newMappings...)

				// Update line counters
				inputLineNum += linesConsumed
				outputLineNum += strings.Count(transformed, "\n")

				// Add newline if not at end
				if inputLineNum < len(lines) {
					output.WriteByte('\n')
					outputLineNum++
				}
				continue
			}
		}

		// Not a match expression, pass through as-is
		output.WriteString(line)
		if inputLineNum < len(lines)-1 {
			output.WriteByte('\n')
		}
		inputLineNum++
		outputLineNum++
	}

	return output.Bytes(), r.mappings, nil
}

// collectMatchExpression collects a complete match expression across multiple lines
// Returns: (matchExpression, linesConsumed)
func (r *RustMatchProcessor) collectMatchExpression(lines []string, startLine int) (string, int) {
	var buf bytes.Buffer
	braceDepth := 0
	linesConsumed := 0
	foundMatch := false

	for i := startLine; i < len(lines); i++ {
		line := lines[i]
		buf.WriteString(line)
		linesConsumed++

		// Track brace depth
		for _, ch := range line {
			if ch == '{' {
				braceDepth++
				foundMatch = true
			} else if ch == '}' {
				braceDepth--
				if braceDepth == 0 && foundMatch {
					// Complete match expression
					return buf.String(), linesConsumed
				}
			}
		}

		// Add newline if more lines to come (C7 FIX: Preserve newlines for proper formatting)
		if i < len(lines)-1 {
			buf.WriteByte('\n')
		}
	}

	return "", 0
}

// transformMatch transforms a Rust-like match expression to Go switch
func (r *RustMatchProcessor) transformMatch(matchExpr string, originalLine int, outputLine int) (string, []Mapping, error) {
	// Extract scrutinee and arms
	matches := matchExprPattern.FindStringSubmatch(matchExpr)
	if len(matches) < 3 {
		return "", nil, fmt.Errorf("invalid match expression syntax")
	}

	scrutinee := strings.TrimSpace(matches[1])
	armsText := matches[2]

	// Check if match expression is in assignment context and extract variable name
	// If the entire match expression starts with "x := match" or "var x = match",
	// we need to handle it specially using Variable Hoisting pattern
	isInAssignment, assignmentVar := r.extractAssignmentVar(matchExpr)

	// Check if scrutinee is a tuple expression
	isTuple, tupleElements, err := r.detectTuple(scrutinee)
	if err != nil {
		return "", nil, err
	}

	if isTuple {
		// Parse tuple pattern arms
		tupleArms, err := r.parseTupleArms(armsText)
		if err != nil {
			return "", nil, fmt.Errorf("parsing tuple pattern arms: %w", err)
		}

		// Generate tuple match (elements extraction + pattern info)
		result, mappings := r.generateTupleMatch(tupleElements, tupleArms, originalLine, outputLine)
		return result, mappings, nil
	}

	// Parse pattern arms (non-tuple)
	arms, err := r.parseArms(armsText)
	if err != nil {
		return "", nil, fmt.Errorf("parsing pattern arms: %w", err)
	}

	// Generate Go switch statement
	result, mappings := r.generateSwitch(scrutinee, arms, originalLine, outputLine, isInAssignment, assignmentVar)
	return result, mappings, nil
}

// parseArms parses pattern arms from the match body
// Handles both simple and block expressions:
//   Ok(x) => x * 2,
//   Err(e) => { log(e); return 0 }
func (r *RustMatchProcessor) parseArms(armsText string) ([]patternArm, error) {
	arms := []patternArm{}
	text := strings.TrimSpace(armsText)

	// Parse arms manually to handle nested braces
	i := 0
	for i < len(text) {
		// Skip whitespace
		for i < len(text) && (text[i] == ' ' || text[i] == '\t' || text[i] == '\n' || text[i] == '\r') {
			i++
		}
		if i >= len(text) {
			break
		}

		// Extract pattern + optional guard (everything before =>)
		arrowPos := strings.Index(text[i:], "=>")
		if arrowPos == -1 {
			break // No more arms
		}

		patternAndGuard := strings.TrimSpace(text[i : i+arrowPos])
		i += arrowPos + 2 // Skip =>

		// Split pattern from guard if present
		// Guard syntax: Pattern if condition => expr
		//           or: Pattern where condition => expr
		pattern, guard := r.splitPatternAndGuard(patternAndGuard)

		// Skip whitespace after =>
		for i < len(text) && (text[i] == ' ' || text[i] == '\t' || text[i] == '\n' || text[i] == '\r') {
			i++
		}
		if i >= len(text) {
			return nil, fmt.Errorf("unexpected end after =>")
		}

		// Extract expression (until comma or end)
		var expr string
		if text[i] == '{' {
			// Block expression - find matching }
			braceCount := 1
			start := i
			i++
			for i < len(text) && braceCount > 0 {
				if text[i] == '{' {
					braceCount++
				} else if text[i] == '}' {
					braceCount--
				}
				i++
			}
			expr = strings.TrimSpace(text[start:i])
			// PRIORITY 3 FIX: Skip trailing comma after block expression
			if i < len(text) && text[i] == ',' {
				i++
			}
		} else {
			// Simple expression - find comma or end (respecting strings and nesting)
			start := i
			if start >= len(text) {
				return nil, fmt.Errorf("unexpected end of text after =>")
			}
			exprEnd := r.findExpressionEnd(text, start)
			if exprEnd > start {
				expr = strings.TrimSpace(text[start:exprEnd])
				i = exprEnd
				// Skip comma if present
				if i < len(text) && text[i] == ',' {
					i++
				}
			} else {
				return nil, fmt.Errorf("invalid expression end at position %d", start)
			}
		}

		// Extract binding from pattern (if present)
		// FIX: Use proper paren matching to handle nested patterns like Result_Ok(Value_Int(n))
		binding := ""
		patternName := pattern
		if strings.Contains(pattern, "(") {
			start := strings.Index(pattern, "(")
			// Find MATCHING closing paren (not just first one)
			end := r.findMatchingCloseParen(pattern, start)
			if end > start {
				binding = strings.TrimSpace(pattern[start+1 : end])
				patternName = pattern[:start]
			}
		}

		arms = append(arms, patternArm{
			pattern:    patternName,
			binding:    binding,
			guard:      guard,
			expression: expr,
		})
	}

	if len(arms) == 0 {
		return nil, fmt.Errorf("no pattern arms found")
	}

	return arms, nil
}
```

---

### 2. FAILING Test Example (pattern_match_01_simple.dingo)

```dingo
package main

import "fmt"

// Simple pattern matching examples for Result[T,E] and Option[T]

// Example 1: Pattern match on Result[T,E]
func processResult(result: Result[int, error]) -> int {
	match result {
		Ok(value) => value * 2,
		Err(e) => 0
	}
}

// Example 2: Pattern match on Option[T]
func processOption(opt: Option[string]) -> string {
	match opt {
		Some(s) => s,
		None => "default"
	}
}

// Example 3: Pattern match with wildcard
func handleStatus(status: Status) -> string {
	match status {
		Active => "running",
		Pending => "waiting",
		_ => "unknown"
	}
}

// Example 4: Pattern match in assignment context
func doubleIfPresent(opt: Option[int]) -> Option[int] {
	result := match opt {
		Some(x) => Some(x * 2),
		None => Option_int_None()
	}
	return result
}

// Example 5: Nested pattern matching
func processNested(result: Result[Option[int], error]) -> int {
	match result {
		Ok(inner) => {
			match inner {
				Some(val) => val,
				None => 0
			}
		},
		Err(e) => -1
	}
}

// Helper enum for example 3
type Status int

const (
	Active Status = iota
	Pending
	Completed
)
```

**Error occurs around lines 9-13, 17-21, 25-29** (all the match expressions)

---

### 3. PASSING Test Example (pattern_match_01_basic.dingo)

```dingo
package main

// Test: Basic pattern matching with Result[T,E] and Option[T]
// Feature: Match expressions without guards
// Complexity: basic

// Example 1: Basic Result pattern match
func getStatusMessage(s Status) string {
	return match s {
		Status_Pending => "Waiting to start",
		Status_Active => "Currently running",
		Status_Complete => "Finished",
	}
}

// Example 2: Option pattern match with Some/None
func getMessage(opt Option_string) string {
	return match opt {
		Option_string_Some(msg) => msg,
		Option_string_None => "No message",
	}
}

// Example 3: Result with binding and block expression
func processValue(r Result_int_error) int {
	return match r {
		Result_int_error_Ok(val) => {
			// Block expression allows multiple statements
			doubled := val * 2
			doubled + 10
		},
		Result_int_error_Err(_) => 0,
	}
}

// Example 4: Wildcard pattern
func classifyStatus(s Status) string {
	return match s {
		Status_Pending => "not started",
		_ => "other",
	}
}

// Example 5: Match in variable assignment
func transform(opt Option_int) Option_int {
	result := match opt {
		Option_int_Some(x) => Option_int_Some(x * 3),
		Option_int_None => Option_int_None,
	}
	return result
}

// Type definitions
type Status int
const (
	Status_Pending Status = iota
	Status_Active
	Status_Complete
)

func main() {
	println(getStatusMessage(Status_Pending))
	println(getMessage(Option_string_Some("hello")))
	println(processValue(Result_int_error_Ok(5)))
	println(classifyStatus(Status_Active))
}
```

**All match expressions work correctly**

---

## KEY DIFFERENCES TO INVESTIGATE

Comparing FAILING vs PASSING tests:

### Syntax Patterns

**FAILING (pattern_match_01_simple.dingo)**:
- Uses `: Type` syntax: `result: Result[int, error]`
- Uses `-> Type` return syntax: `func processResult(...) -> int`
- Uses **unqualified** patterns: `Ok(value)`, `Some(s)`, `None`
- Implicit return (match as expression): `match status { ... }`

**PASSING (pattern_match_01_basic.dingo)**:
- Uses Go-style type syntax: `s Status`, `opt Option_string`
- Uses Go-style return: `func getStatusMessage(...) string`
- Uses **qualified** patterns: `Status_Pending`, `Option_string_Some(msg)`
- Explicit return: `return match s { ... }`

### Specific Examples

**FAILING**:
```dingo
func processOption(opt: Option[string]) -> string {
	match opt {
		Some(s) => s,
		None => "default"
	}
}
```

**PASSING**:
```dingo
func getMessage(opt Option_string) string {
	return match opt {
		Option_string_Some(msg) => msg,
		Option_string_None => "No message",
	}
}
```

---

## INVESTIGATION TASKS

### 1. Trace Execution

Walk through what happens when `processOption` is processed:

1. **Line detection**: Does `match opt {` trigger collection?
2. **collectMatchExpression**: What text is collected? (lines 113-146)
3. **Regex matching**: Does `matchExprPattern` match? What does it capture?
4. **parseArms**: What is `armsText`? Why does it return "no pattern arms found"?

### 2. Hypothesis Testing

Test these theories:

**Theory A**: `: Type` syntax corrupts the match line
- Does TypeAnnotProcessor run before RustMatchProcessor?
- Does it transform `: Option[string]` into something that breaks match detection?

**Theory B**: `-> Type` return syntax causes issues
- Is the arrow `->` interfering with pattern arm arrow `=>`?
- Does this confuse the parser?

**Theory C**: Unqualified patterns cause issues
- Do patterns like `Some(s)` vs `Option_string_Some(msg)` matter?
- Does UnqualifiedImportProcessor interfere? (Check pipeline order!)

**Theory D**: Implicit return breaks collection
- Does absence of `return` keyword change what's collected?
- Compare collected text for `return match` vs bare `match`

**Theory E**: Brace counting bug
- Do interior braces (in patterns or expressions) confuse `collectMatchExpression`?
- Example: If we have `{` in a pattern, does `braceDepth` get confused?

**Theory F**: Regex greedy matching
- Does `(.+)` in `matchExprPattern` capture too much or too little?
- What exactly does `armsText` contain when it fails?

### 3. Preprocessor Pipeline Order

Check `preprocessor.go` to verify pipeline order:
```go
processors := []FeatureProcessor{
    NewGenericSyntaxProcessor(),      // 0
    NewTypeAnnotProcessor(),          // 1 ← Transforms `: Type` syntax
    NewErrorPropProcessor(),          // 2
    NewEnumProcessor(),               // 3
    NewRustMatchProcessor(),          // 4 ← We are here
    NewKeywordProcessor(),            // 5
    NewUnqualifiedImportProcessor(),  // 6 ← Transforms unqualified patterns
}
```

**Key insight**: TypeAnnotProcessor runs BEFORE RustMatchProcessor!
- This means `: Option[string]` may already be transformed
- RustMatchProcessor sees POST-TypeAnnot code, not original

---

## REQUIRED OUTPUT

Please provide:

### 1. Root Cause Analysis (3-5 paragraphs)
- Identify the EXACT line of code causing the bug
- Explain WHY it fails for failing tests but works for passing tests
- Provide evidence from code tracing

### 2. Execution Trace
Walk through step-by-step:
```
Input: "func processOption(opt: Option[string]) -> string {\n    match opt {\n..."

Step 1: Line detection
- Line 1: "func processOption(opt: Option[string]) -> string {"
- Line 2: "    match opt {"
- Detection: "match " found on line 2 ✓

Step 2: collectMatchExpression (lines 113-146)
- Collected text: [WHAT EXACTLY?]
- braceDepth tracking: [SHOW EACH STEP]
- Returned: [FULL TEXT]

Step 3: Regex matching (line 150)
- Input to regex: [EXACT STRING]
- Regex: (?s)match\s+([^{]+)\s*\{(.+)\}
- Match result: [YES/NO]
- If YES, captures: scrutinee=[?], armsText=[?]

Step 4: parseArms (line 183)
- Input armsText: [EXACT STRING]
- Parsing: [STEP BY STEP]
- Result: [ARMS FOUND OR ERROR]
```

### 3. Proposed Fix
- Specific code change (file, line number, old code, new code)
- Diff format if possible
- Explanation of why this fixes it

### 4. Validation Strategy
- How to test the fix
- Expected before/after test results
- Any edge cases to watch for

### 5. Confidence Level
- High/Medium/Low confidence in this analysis
- What assumptions were made
- What would increase confidence

---

## SUCCESS CRITERIA

Your analysis should:
1. ✅ Identify exact failure point (line number in rust_match.go)
2. ✅ Explain difference between failing and passing tests
3. ✅ Provide testable fix that can be implemented immediately
4. ✅ Include execution trace showing bug manifestation
5. ✅ Achieve 98%+ test passing when implemented

---

## IMPORTANT NOTES

- Do NOT claim tests are passing unless you actually ran them
- Do NOT implement the fix yourself (just provide analysis + patch)
- Do verify your hypothesis against the actual code provided
- Do provide specific line numbers and code snippets
- Do consider interaction between preprocessors in the pipeline

---

**This is Round 2** - We need DETAILED, ACCURATE analysis to fix this bug.
