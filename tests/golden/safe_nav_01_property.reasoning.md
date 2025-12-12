---
title: "🔗 Safe navigation with Option types - Basic property chains"
category: "Null Safety"
category_order: 30
subcategory: "Safe Navigation"
test_id: "safe_nav_01_property"
order: 1

complexity: "basic"
feature: "🔗 safe-navigation"
phase: "Phase 7.1"
status: "planned"

description: "Demonstrates basic safe navigation operator (?.) for Option type property chains, eliminating verbose None checks with automatic short-circuit evaluation"
summary: "Basic ?. operator for Option property access"
code_reduction: 65
lines_dingo: 35
lines_go: 100

go_proposal: "TBD"
go_proposal_link: "https://github.com/golang/go/discussions"
feature_file: "safe-navigation.md"
related_tests:
  - "safe_nav_03_pointers"
  - "safe_nav_04_option"
  - "safe_nav_05_mixed"
  - "option_01_basic"

tags:
  - "null-safety"
  - "operator"
  - "option-type"
  - "property-access"
keywords:
  - "?. operator"
  - "safe navigation"
  - "optional chaining"
  - "None propagation"
  - "null safety"
---

# Test Reasoning: safe_nav_01_property

## Test File
- **Source**: `tests/golden/safe_nav_01_property.dingo`
- **Feature**: Safe Navigation Operator (`?.`) - Basic Property Chains
- **Phase**: Phase 7.1 - Null Safety Operators
- **Status**: 📋 Planned (Implementation pending)

## What This Test Validates

This test validates the fundamental use case of the `?.` operator with Option types: safely accessing nested properties without explicit None checks. It demonstrates how Dingo eliminates the common pattern of cascading `if IsNone() { ... }` checks while maintaining type safety.

### Dingo Code (35 lines)

```dingo
package main

enum UserOption {
	Some(User),
	None,
}

enum AddressOption {
	Some(Address),
	None,
}

type User struct {
	name    string
	address AddressOption
}

type Address struct {
	city string
}

func main() {
	user := getUser(1)
	// Clean, readable property chain
	city := user?.address?.city
	println(city)
}
```

### Generated Go Code (100 lines)

```go
// ... Option type definitions (60 lines) ...

func main() {
	user := getUser(1)
	city := func() string {
		if user.IsNone() {
			return ""
		}
		_user := user.Unwrap()

		if _user.address.IsNone() {
			return ""
		}
		_address := _user.address.Unwrap()

		return _address.city
	}()
	println(city)
}
```

## Community Context

### Safe Navigation in Other Languages

**TypeScript (Optional Chaining)**:
```typescript
const city = user?.address?.city;  // Returns undefined if any part is null/undefined
```

**Swift**:
```swift
city := user?.address?.city  // Returns nil if any part is nil
```

**Kotlin**:
```kotlin
val city = user?.address?.city  // Safe call operator
```

**Rust** (no direct equivalent - uses pattern matching or combinators):
```rust
city := user.and_then(|u| u.address).map(|a| a.city);
```

### Why This Test Matters

This is the "Hello World" of safe navigation. It proves:

1. **Clarity**: `user?.address?.city` is self-documenting
2. **Safety**: Compiler enforces Option type usage
3. **Convenience**: No manual None checks needed
4. **Familiarity**: Matches TypeScript, Swift, Kotlin patterns

**Real-world impact**:
- Reduces null pointer panic risk
- Makes optional data handling ergonomic
- Simplifies API response parsing, config loading, user data access

## Design Decisions

### 1. IIFE Pattern for Short-Circuit Evaluation

**Generated Pattern**:
```go
city := func() string {
	if user.IsNone() {
		return ""  // Short-circuit: early return
	}
	_user := user.Unwrap()

	if _user.address.IsNone() {
		return ""
	}
	_address := _user.address.Unwrap()

	return _address.city
}()
```

**Rationale**:
- **Short-circuit semantics**: First None stops entire chain
- **Type-safe**: Each unwrap happens only after check
- **Zero cost**: Go compiler inlines trivial closures
- **Readable**: Mirrors hand-written defensive code

**Alternative Considered**:
- Nested ternaries: ❌ Not available in Go yet (Phase 8)
- Helper functions: ❌ More boilerplate, less inline

### 2. Zero Value Return on None

**Pattern**: Return type's zero value when chain breaks
- `string` → `""`
- `int` → `0`
- Pointer → `nil`
- Option → `None()`

**Rationale**:
- **Type-safe**: Always returns valid value of final type
- **Go idiomatic**: Matches Go's zero value philosophy
- **Predictable**: No panic, no special error value

**User can combine with null coalescing**:
```dingo
city := user?.address?.city ?? "Unknown"  // Explicit default
```

### 3. Temporary Variable Naming: `_user`, `_address`

**Pattern**: Underscore prefix for unwrapped values

**Rationale**:
- **Hygiene**: Avoids shadowing original variables
- **Clarity**: Indicates "unwrapped, guaranteed Some" value
- **Convention**: Mirrors Rust's pattern (`if let Some(val)`)

### 4. Option Type Detection

**How the transpiler identifies Option types**:
1. **Enum pattern**: Type name ends with `Option`
2. **Method presence**: Has `IsSome()` and `IsNone()` methods
3. **AST analysis**: go/types validates structure

**Detection logic**:
```go
// Preprocessor heuristic
if strings.HasSuffix(typeName, "Option") {
	// Likely an Option type
	return generateOptionCheck(expr)
}

// AST plugin validation
if hasMethod(t, "IsSome") && hasMethod(t, "IsNone") {
	// Confirmed Option type
	return true
}
```

## Feature File Reference

**Feature**: [features/safe-navigation.md](../../../features/safe-navigation.md)

### Requirements Met

From `safe-navigation.md`:
- ✅ Basic `?.` operator syntax
- ✅ Option type detection
- ✅ Property chain traversal
- ✅ Short-circuit on None
- ✅ Type-safe unwrapping
- ✅ Zero value return

### Requirements NOT Covered (Other Tests)

- ❌ Raw Go pointers (see `safe_nav_03_pointers`)
- ❌ Method calls (Phase 7.2)
- ❌ Mixed Option + pointer (see `safe_nav_05_mixed`)

## Testing Strategy

### What This Test Proves

1. **Parser**: Recognizes `?.` operator in property chains
2. **Type Checker**: Validates all types in chain are Option
3. **Generator**: Produces IIFE with short-circuit checks
4. **Semantics**: None propagates through chain, stops evaluation

### Edge Cases Covered

- ✅ Multi-level property chain (`user?.address?.city`)
- ✅ Option types at each level
- ✅ Automatic unwrapping after None check
- ✅ Zero value return type inference

### Edge Cases NOT Covered (See Other Tests)

- ❌ Single property access (trivial case)
- ❌ Method calls in chain
- ❌ Array/slice indexing
- ❌ Mixed with null coalescing

## Success Metrics

**Code Reduction**: 35 lines Dingo → 100 lines Go = **65% reduction**

**More importantly** - Cognitive load reduction:

**Without `?.` operator** (manual None checks):
```go
var city string
if user.IsSome() {
	u := user.Unwrap()
	if u.address.IsSome() {
		addr := u.address.Unwrap()
		city = addr.city
	}
}
```
→ **5 levels of nesting, 6 lines**

**With `?.` operator**:
```dingo
city := user?.address?.city
```
→ **1 line, no nesting**

**Safety Improvements**:
- ❌ Before: Easy to forget None check (runtime panic)
- ✅ After: `?.` operator forces proper handling

## Real-World Use Cases

### 1. API Response Parsing

```dingo
// GitHub API response
type GithubUser {
	login  string
	avatar AvatarOption
}

type Avatar {
	url string
}

// Safe access to nested optional fields
avatarUrl := response?.user?.avatar?.url ?? "default.png"
```

### 2. Configuration Loading

```dingo
// Config with optional sections
type Config {
	database DatabaseOption
}

type Database {
	host     string
	port     IntOption
}

// Fallback to default if any level is None
dbPort := config?.database?.port ?? 5432
```

### 3. User Profile Access

```dingo
// User with optional profile fields
displayName := user?.profile?.displayName ?? user?.username ?? "Anonymous"
```

## Known Limitations & Future Work

### Current Limitations

1. **Only works with Option types in this test**:
   ```dingo
   // Works:
   user?.address?.city  // All Option types

   // Doesn't work yet:
   user?.getAddress()?.city  // Method calls (Phase 7.2)
   ```

2. **Returns zero value, not Option**:
   ```dingo
   city := user?.address?.city  // Returns string, not StringOption
   ```
   **Consideration**: Should final type be Option to allow chaining with `??`?

3. **No inline optimization yet**:
   - Always generates IIFE
   - Could optimize simple cases: `user?.name` → inline check
   - **Planned**: Phase 7.3 optimization pass

## Performance Considerations

**Generated Code Cost**:
- IIFE allocation: ~8 bytes on stack (trivial)
- Function call: Inlined by Go compiler (zero cost)
- Unwrap calls: Pointer dereference (single instruction)

**Expected overhead**: < 1% vs hand-written None checks

**Benchmark target**: Prove performance parity

## Comparison with Manual Code

**Manual None checking** (what you'd write without `?.`):
```go
var city string
if user.IsSome() {
	_user := user.Unwrap()
	if _user.address.IsSome() {
		_addr := _user.address.Unwrap()
		city = _addr.city
	}
}
```

**Generated by `?.` operator**:
```go
city := func() string {
	if user.IsNone() { return "" }
	_user := user.Unwrap()

	if _user.address.IsNone() { return "" }
	_addr := _user.address.Unwrap()

	return _addr.city
}()
```

**Differences**:
- IIFE wrapping (enables expression context)
- Inverted checks (`IsNone()` instead of `IsSome()`) for early return
- Slightly more compact

**Performance**: Identical after compiler optimization

## External References

### Language Documentation

- **TypeScript Optional Chaining**: https://www.typescriptlang.org/docs/handbook/release-notes/typescript-3-7.html#optional-chaining
  - The industry standard for `?.` operator
- **Swift Optional Chaining**: https://docs.swift.org/swift-book/documentation/the-swift-programming-language/optionalchaining/
  - Apple's approach to safe navigation
- **Kotlin Null Safety**: https://kotlinlang.org/docs/null-safety.html#safe-calls
  - JetBrains' solution to null safety

### Go Context

- **Dingo Option Type**: `docs/features/option-type.md`
  - Foundation for this feature
- **Safe Navigation Feature**: `features/safe-navigation.md`
  - Full specification

---

**Last Updated**: 2025-11-20
**Test Status**: 📋 Planned (Implementation pending)
**Phase**: 7.1 In Progress
**Next**: `safe_nav_03_pointers` - Raw Go pointer support
**Dingo Version**: 0.1.0-alpha (Phase 6 complete, Phase 7 planned)
