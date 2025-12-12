# Enum Variant Naming Conventions: Impact on Pattern Matching

## Executive Summary

After analyzing Dingo's current implementation and comparing different naming conventions, the **current convention (EnumName_VariantName in PascalCase)** works best for pattern matching. It provides clear namespacing, avoids conflicts, and produces idiomatic Go code. Alternative conventions introduce readability, parsing, or compatibility issues.

## Current Implementation Analysis

Based on examination of `pkg/preprocessor/rust_match.go` and golden tests:

### Current Convention: EnumName_VariantName
- Enum variants: `Status_Pending`, `Status_Active`, `Status_Complete`
- Tag constants: `StatusTag_Pending`, `StatusTag_Active`, `StatusTag_Complete`
- Constructors: `Status_Pending()`, `Status_Active()`, `Status_Complete()`

### Pattern Matching Flow
1. Pattern `Status_Pending` in match expression
2. Translates to case `StatusTag_Pending` in Go switch
3. Clear mapping: Pattern → Tag constant (add "Tag" after enum name)

## Concrete Examples: Different Naming Conventions

### 1. Current Convention (EnumName_VariantName in PascalCase)

```dingo
enum Status {
    Pending,
    Active,
    Complete,
}

enum FileState {
    Open(handle int),
    Closed,
    Error(msg string),
}

func processStatus(s Status) string {
    match s {
        Status_Pending => "Waiting",
        Status_Active => "Running",
        Status_Complete => "Done",
    }
}

func handleFile(f FileState) int {
    match f {
        FileState_Open(h) => h,
        FileState_Closed => 0,
        FileState_Error(e) => panic(e),
    }
}
```

**Generated Go:**
```go
// Tags
const (
    StatusTag_Pending StatusTag = iota
    StatusTag_Active
    StatusTag_Complete
)

// Pattern matching
switch s.tag {
case StatusTag_Pending:
    return "Waiting"
case StatusTag_Active:
    return "Running"
case StatusTag_Complete:
    return "Done"
}

// Constructors
func Status_Pending() Status { ... }
func FileState_Open(handle int) FileState { ... }
```

### 2. Alternative: UPPER_CASE Variants

```dingo
enum Status {
    PENDING,
    ACTIVE,
    COMPLETE,
}

func processStatus(s Status) string {
    match s {
        Status_PENDING => "Waiting",
        Status_ACTIVE => "Running",
        Status_COMPLETE => "Done",
    }
}
```

**Generated Go:**
```go
// Tags become awkward
const (
    StatusTag_PENDING StatusTag = iota  // Double uppercase feels wrong
    StatusTag_ACTIVE
    StatusTag_COMPLETE
)

// Constructors violate Go naming
func Status_PENDING() Status { ... }  // Should be StatusPending in Go
```

**Problems:**
- Violates Go naming conventions (constructors should be PascalCase)
- Tag names (`StatusTag_PENDING`) feel redundant
- Harder to read in pattern matching context

### 3. Alternative: snake_case Variants

```dingo
enum Status {
    pending,
    active,
    complete,
}

func processStatus(s Status) string {
    match s {
        Status_pending => "Waiting",
        Status_active => "Running",
        Status_complete => "Done",
    }
}
```

**Generated Go:**
```go
// Tags need case transformation
const (
    StatusTag_pending StatusTag = iota  // Not idiomatic Go
    StatusTag_active
    StatusTag_complete
)

// Or transformed:
const (
    StatusTagPending StatusTag = iota   // Lost visual connection
    StatusTagActive
    StatusTagComplete
)
```

**Problems:**
- Not idiomatic for Go constants or constructors
- Requires case transformation logic
- Lost visual mapping between pattern and tag

### 4. Alternative: Bare Variants (No Prefix)

```dingo
enum Status {
    Pending,
    Active,
    Complete,
}

func processStatus(s Status) string {
    match s {
        Pending => "Waiting",    // Which Pending?
        Active => "Running",
        Complete => "Done",
    }
}
```

**Problems:**
- Namespace pollution in Go
- Ambiguity with multiple enums having same variant names
- Parser complexity to infer enum type

### 5. Alternative: Verb Prefixes (IsPending, IsActive)

```dingo
enum Status {
    IsPending,
    IsActive,
    IsComplete,
}

func processStatus(s Status) string {
    match s {
        Status_IsPending => "Waiting",
        Status_IsActive => "Running",
        Status_IsComplete => "Done",
    }
}
```

**Problems:**
- Semantically wrong (variants are states, not predicates)
- Conflicts with generated helper methods (`s.IsPending()`)
- Verbose and redundant

## Readability Comparison

### Pattern Matching Context

**Current (Best):**
```dingo
match result {
    Result_Ok(value) => value * 2,
    Result_Err(e) => panic(e),
}

match opt {
    Option_Some(x) => x,
    Option_None => 0,
}
```

**UPPER_CASE:**
```dingo
match result {
    Result_OK(value) => value * 2,    // OK looks like shouting
    Result_ERR(e) => panic(e),
}
```

**snake_case:**
```dingo
match result {
    Result_ok(value) => value * 2,    // Inconsistent with constructor
    Result_err(e) => panic(e),
}
```

### Constructor Usage

**Current (Best):**
```dingo
r := Result_Ok(42)          // Clear it's a constructor
e := Result_Err("failed")
s := Status_Active()
```

**Alternatives have issues:**
```dingo
r := Result_OK(42)          // SHOUTING constructor
r := Result_ok(42)          // Not Go-like
r := Ok(42)                 // Ambiguous without context
```

## Go Code Translation Quality

### Current Convention Produces Best Go Code

```go
// Clean tag constants
const (
    ResultTagOk ResultTag = iota
    ResultTagErr
)

// Idiomatic constructors
func Result_Ok(value interface{}) Result {
    return Result{tag: ResultTagOk, ok_0: &value}
}

// Clear switch cases
switch r.tag {
case ResultTagOk:
    value := *r.ok_0
    // ...
}
```

### Alternative Conventions Produce Worse Go

**UPPER_CASE:**
```go
func Result_OK(value interface{}) Result {  // Violates Go conventions
    return Result{tag: ResultTag_OK, ...}   // Awkward constant
}
```

**snake_case:**
```go
func Result_ok(value interface{}) Result {  // Not idiomatic
    return Result{tag: ResultTag_ok, ...}   // Wrong for constants
}
```

## Implementation Complexity

### Current: Simple and Direct

The `getTagName` function in `rust_match.go` shows the simplicity:
```go
func (r *RustMatchProcessor) getTagName(pattern string) string {
    switch pattern {
    case "Ok": return "ResultTagOk"
    case "Err": return "ResultTagErr"
    case "Some": return "OptionTagSome"
    case "None": return "OptionTagNone"
    default:
        // Custom enum: Status_Pending → StatusTag_Pending
        if idx := strings.Index(pattern, "_"); idx > 0 {
            enumName := pattern[:idx]
            variantName := pattern[idx:]
            return enumName + "Tag" + variantName
        }
        return pattern + "Tag"
    }
}
```

### Alternatives Require More Complex Logic

- **UPPER_CASE**: Need case detection and transformation
- **snake_case**: Need case conversion for Go constants
- **Bare variants**: Need context tracking to resolve enum type

## Edge Cases and Special Considerations

### 1. Single-Letter Variants
```dingo
enum Grade { A, B, C, D, F }

// Current convention handles well:
match grade {
    Grade_A => 4.0,
    Grade_B => 3.0,
    // ...
}
```

### 2. Multi-Word Variants
```dingo
enum HttpStatus {
    NotFound,
    InternalServerError,
    BadRequest,
}

// Current convention:
HttpStatus_NotFound         // Clear and readable
HttpStatus_InternalServerError  // Long but unambiguous

// vs UPPER_CASE:
HttpStatus_NOT_FOUND        // Too many underscores
HttpStatus_INTERNAL_SERVER_ERROR  // Hard to read
```

### 3. Acronyms
```dingo
enum Protocol {
    HTTP,
    HTTPS,
    FTP,
}

// Current convention works:
Protocol_HTTP    // Clear
Protocol_HTTPS   // Maintains acronym

// Built-in types keep simple names:
Result_Ok        // Not Result_OK
Option_Some      // Not Option_SOME
```

## Recommendations

### Primary Recommendation: Keep Current Convention

**EnumName_VariantName** with PascalCase for all parts:
- Custom enums: `Status_Pending`, `FileState_Open`
- Built-in types: `Result_Ok`, `Option_Some` (special-cased)

### Naming Rules

1. **Enum names**: Always PascalCase (`Status`, `FileState`)
2. **Variant names**: Always PascalCase (`Pending`, `Active`)
3. **Pattern matching**: Full name `EnumName_VariantName`
4. **Constructors**: Same as patterns (act as namespace)
5. **Tag constants**: Insert "Tag" after enum name

### Special Cases

1. **Built-in types** (Result, Option): Use simple variant names in patterns
   - Pattern: `Ok(x)` not `Result_Ok(x)` (parser can infer)
   - Constructor: `Result_Ok(42)` (explicit for clarity)

2. **Import handling**: When unqualified imports are used
   ```dingo
   use std::result::Result::{Ok, Err}

   match r {
       Ok(x) => x,      // Bare variant OK with import
       Err(e) => 0,
   }
   ```

## Conclusion

The current naming convention (EnumName_VariantName in PascalCase) provides the best balance of:
- **Clarity**: Clear namespace, no ambiguity
- **Go compatibility**: Produces idiomatic Go code
- **Readability**: Natural in pattern matching context
- **Simplicity**: Straightforward transformation rules
- **Consistency**: Works for all enum types uniformly

Alternative conventions introduce various problems without significant benefits. The current approach should be maintained and documented as the standard for Dingo.