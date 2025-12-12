---
title: "🔗 Safe navigation with pure Option type chains"
category: "Null Safety"
category_order: 30
subcategory: "Safe Navigation"
test_id: "safe_nav_04_option"
order: 4

complexity: "intermediate"
feature: "🔗 safe-navigation"
phase: "Phase 7.1"
status: "planned"

description: "Demonstrates safe navigation operator (?.) with complex Option type chains, including None propagation through multiple levels and type preservation"
summary: "?. operator with deep Option chains"
code_reduction: 68
lines_dingo: 60
lines_go: 188

feature_file: "safe-navigation.md"
related_tests:
  - "safe_nav_01_property"
  - "safe_nav_03_pointers"
  - "safe_nav_05_mixed"
  - "option_03_chaining"

tags:
  - "null-safety"
  - "option-type"
  - "type-safety"
  - "chaining"
keywords:
  - "Option chaining"
  - "None propagation"
  - "type preservation"
  - "nested options"
---

# Test Reasoning: safe_nav_04_option

## Test File
- **Source**: `tests/golden/safe_nav_04_option.dingo`
- **Feature**: Safe Navigation Operator (`?.`) - Pure Option Type Chains
- **Phase**: Phase 7.1 - Null Safety Operators
- **Status**: 📋 Planned (Implementation pending)

## What This Test Validates

This test validates safe navigation with **pure Option type chains** - all levels are Option types, no raw pointers. It demonstrates:
1. Deep nesting (Option → Option → Option)
2. None propagation through multiple levels
3. Type preservation (returns Option, not unwrapped value)
4. Multiple chain scenarios in same code

### Key Insight: Type Preservation

Unlike `safe_nav_01_property` which returns unwrapped values, this test shows chains that **preserve Option types**:

```dingo
user?.profile?.bio  // Returns StringOption, not string
```

**Why this matters**: Allows further chaining with `??` operator or pattern matching.

## Dingo Code (60 lines)

```dingo
package main

enum UserOption {
	Some(User),
	None,
}

enum ProfileOption {
	Some(Profile),
	None,
}

enum StringOption {
	Some(string),
	None,
}

type User struct {
	id      int
	profile ProfileOption
}

type Profile struct {
	bio   StringOption
	email string
}

func main() {
	// Test 1: Full chain (all Some)
	user := fetchUser(1)
	bio := user?.profile?.bio
	println("Bio:", bio)

	// Test 2: None at root
	noUser := fetchUser(0)
	noBio := noUser?.profile?.bio
	println("No Bio:", noBio)

	// Test 3: Intermediate None
	partialUser := UserOption_Some(User{id: 2, profile: ProfileOption_None()})
	partialBio := partialUser?.profile?.bio
	println("Partial Bio:", partialBio)
}
```

## Design Decisions

### 1. Type Preservation: Return Option, Not Value

**Generated code returns final Option type**:
```go
bio := func() StringOption {
	if user.IsNone() {
		return StringOption_None()  // Not ""
	}
	_user := user.Unwrap()

	if _user.profile.IsNone() {
		return StringOption_None()
	}
	_profile := _user.profile.Unwrap()

	return _profile.bio  // StringOption (already Option)
}()
```

**Contrast with zero value return** (pointers/basic types):
```go
// If bio was string (not StringOption)
bio := func() string {
	// ...
	return ""  // Zero value
}()
```

**Why preserve Option type?**
- **Composability**: Can chain with `??` operator
  ```dingo
  bio := user?.profile?.bio ?? StringOption_Some("No bio")
  ```
- **Pattern matching**: Can match on result
  ```dingo
  match user?.profile?.bio {
  	StringOption_Some(text) => println(text),
  	StringOption_None => println("No bio"),
  }
  ```
- **Type safety**: Caller knows value is optional, must handle explicitly

### 2. None Propagation Semantics

**Propagation rules**:
1. **Root None**: Short-circuit immediately, return None
2. **Intermediate None**: Stop at that level, return None
3. **All Some**: Continue to final value (which itself is Option)

**Test scenarios**:

**Scenario A: All Some**
```dingo
user: Some(User)
user.profile: Some(Profile)
user.profile.bio: Some("Software Engineer")

Result: Some("Software Engineer")
```

**Scenario B: Root None**
```dingo
user: None

Result: None (never accesses profile or bio)
```

**Scenario C: Intermediate None**
```dingo
user: Some(User)
user.profile: None

Result: None (never accesses bio)
```

### 3. Multiple Option Types in Same Chain

**Type evolution through chain**:
```
UserOption → User (unwrapped)
  ↓
User.profile → ProfileOption
  ↓
Profile (unwrapped) → Profile.bio → StringOption
```

**Each step**:
1. Check if current Option is None
2. Unwrap to get concrete type
3. Access next field (which is also Option)
4. Repeat

**Final return**: Last type in chain (StringOption)

### 4. Comparison: Property Access vs Field Type

**Important distinction**:

**Case 1: Field is Option, return as-is**
```dingo
profile?.bio  // bio is StringOption field
// Returns: StringOption (no unwrap of bio itself)
```

**Case 2: Field is value, wrap in Option**
```dingo
profile?.email  // email is string field
// Returns: StringOption (wraps string in Option)
```

**In this test**: All fields are already Option types, so no wrapping needed.

## Testing Strategy

### What This Test Proves

1. **Deep Nesting**: Handles 3-level Option chains
2. **Type Tracking**: Correctly identifies each level's Option type
3. **None Propagation**: Stops at first None, returns correct type
4. **Type Preservation**: Returns Option, not unwrapped value
5. **Multiple Scenarios**: Same pattern works in different contexts

### Test Cases

**Test 1: Full chain (all Some)**
- Path: `fetchUser(1)` → Some(User) → Some(Profile) → Some(bio)
- Expected: Returns `StringOption_Some("Software Engineer")`

**Test 2: Root None**
- Path: `fetchUser(0)` → None
- Expected: Returns `StringOption_None()` immediately

**Test 3: Intermediate None**
- Path: `UserOption_Some(...)` with `ProfileOption_None()`
- Expected: Returns `StringOption_None()` at profile check

### Edge Cases Covered

- ✅ All levels are Option types
- ✅ None at different levels (root, intermediate, final)
- ✅ Option type return (not unwrapped)
- ✅ Multiple chains in same function

## Real-World Use Cases

### 1. User Profile Data

```dingo
// Social media platform
type User {
	profile ProfileOption
}

type Profile {
	avatar   AvatarOption
	coverImg ImageOption
	bio      StringOption
}

// Safe access to deeply nested optional fields
avatarUrl := user?.profile?.avatar?.url ?? "default-avatar.png"
bio := user?.profile?.bio ?? StringOption_Some("No bio")
```

### 2. API Response Handling

```dingo
// GitHub API response
type Repository {
	owner  OwnerOption
	readme ReadmeOption
}

type Owner {
	login StringOption
	email StringOption
}

// Graceful handling of incomplete API responses
ownerEmail := repo?.owner?.email
// Returns EmailOption - can check if present or use default
```

### 3. Configuration Cascading

```dingo
// Layered config (system -> user -> project)
type ProjectConfig {
	userConfig UserConfigOption
}

type UserConfig {
	theme ThemeOption
}

// Try project -> user -> default
theme := projectConfig?.userConfig?.theme ?? systemDefaults.theme
```

## Performance Considerations

**Generated code for 3-level chain**:
```go
bio := func() StringOption {
	if user.IsNone() { return StringOption_None() }  // Method call
	_user := user.Unwrap()                            // Method call

	if _user.profile.IsNone() { return StringOption_None() }  // Method call
	_profile := _user.profile.Unwrap()                         // Method call

	return _profile.bio  // Field access
}()
```

**Cost breakdown**:
- 2 `IsNone()` calls: ~4-8 CPU cycles each
- 2 `Unwrap()` calls: ~4-8 CPU cycles each
- 1 field access: ~1 CPU cycle
- 1 IIFE: Inlined by compiler (zero cost)

**Total**: ~20-35 CPU cycles (negligible for typical use cases)

**Comparison with manual checks**:
```go
// Manual None checking
var bio StringOption
if user.IsSome() {
	u := user.Unwrap()
	if u.profile.IsSome() {
		p := u.profile.Unwrap()
		bio = p.bio
	}
}
// Cost: Same ~20-35 CPU cycles
```

**Conclusion**: No performance overhead vs manual code

## Integration with Other Features

### Combining with Null Coalescing

```dingo
// Provide default if any level is None
bio := user?.profile?.bio ?? StringOption_Some("No bio provided")

// Or unwrap and provide default value
bioText := user?.profile?.bio?.Unwrap() ?? "No bio"
```

### Combining with Pattern Matching

```dingo
match user?.profile?.bio {
	StringOption_Some(text) => println("Bio:", text),
	StringOption_None => println("No bio available"),
}
```

### Combining with Error Propagation

```dingo
// If bio is Result<string, Error> instead of Option
bioResult := user?.profile?.bio?  // Propagate error
```

## Known Limitations & Future Work

### Current Limitations

1. **No automatic unwrapping** at chain end:
   ```dingo
   bio := user?.profile?.bio  // Returns StringOption
   text := bio.Unwrap()       // Must manually unwrap
   ```
   **Consideration**: Should `?.` at end unwrap final Option?
   **Tradeoff**: Explicit unwrap is safer (forces None handling)

2. **Deep nesting verbosity**:
   ```dingo
   value := user?.a?.b?.c?.d?.e  // Still verbose if many levels
   ```
   **Alternative**: Suggest flatter data structures

3. **Type inference complexity**:
   - Preprocessor must track Option types through entire chain
   - AST plugin validates but adds compilation overhead
   **Mitigation**: Cache type information, optimize analysis pass

### Future Enhancements

**Option 1: Terminal unwrap operator**
```dingo
bio := user?.profile?.bio!  // ! unwraps final Option (panics on None)
```

**Option 2: Safe unwrap with default**
```dingo
bio := user?.profile?.bio ? "default"  // ? is both safe nav and coalesce
```

**Option 3: Monadic operations**
```dingo
bio := user?.profile?.bio.map(|s| s.toUpper())
```

## Comparison with Other Languages

### Rust (Explicit Option handling)

```rust
// Rust requires explicit combinators
bio := user
	.and_then(|u| u.profile)
	.and_then(|p| p.bio);
// Returns Option<String>
```

**Dingo advantage**: `?.` is more concise than `and_then` chains

### Swift (Optional Chaining)

```swift
bio := user?.profile?.bio
// Returns String? (optional string)
```

**Similarity**: Identical syntax and semantics!

### TypeScript (Optional Chaining)

```typescript
const bio = user?.profile?.bio;
// Returns string | undefined
```

**Difference**: TS uses `undefined`, Dingo uses explicit None variant

### Kotlin (Safe Calls)

```kotlin
val bio = user?.profile?.bio
// Returns String? (nullable string)
```

**Similarity**: Same operator, same behavior

**Dingo uniqueness**: Works with both Option types AND raw pointers (see `safe_nav_03_pointers`)

## External References

### Dingo Documentation

- **Option type**: `docs/features/option-type.md`
- **Safe navigation**: `features/safe-navigation.md`
- **Pattern matching**: `docs/features/pattern-matching.md`

### Language References

- **Swift Optional Chaining**: https://docs.swift.org/swift-book/documentation/the-swift-programming-language/optionalchaining/
- **Rust Option**: https://doc.rust-lang.org/std/option/enum.Option.html
- **TypeScript Optional Chaining**: https://www.typescriptlang.org/docs/handbook/release-notes/typescript-3-7.html

---

**Last Updated**: 2025-11-20
**Test Status**: 📋 Planned
**Phase**: 7.1 In Progress
**Next**: `safe_nav_05_mixed` - Mixed Option + pointer chains
**Dingo Version**: 0.1.0-alpha
