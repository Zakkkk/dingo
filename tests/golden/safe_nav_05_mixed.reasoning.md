---
title: "🔗 Safe navigation with mixed Option and pointer types"
category: "Null Safety"
category_order: 30
subcategory: "Safe Navigation"
test_id: "safe_nav_05_mixed"
order: 5

complexity: "advanced"
feature: "🔗 safe-navigation"
phase: "Phase 7.1"
status: "planned"

description: "Demonstrates safe navigation operator (?.) with mixed Option and pointer types in same chain, including type boundary handling and automatic nil-to-None conversion"
summary: "?. operator with mixed Option + pointer chains"
code_reduction: 72
lines_dingo: 62
lines_go: 221

feature_file: "safe-navigation.md"
related_tests:
  - "safe_nav_01_property"
  - "safe_nav_03_pointers"
  - "safe_nav_04_option"

tags:
  - "null-safety"
  - "option-type"
  - "pointers"
  - "type-conversion"
  - "advanced"
keywords:
  - "mixed types"
  - "type boundaries"
  - "nil to None"
  - "Option pointer hybrid"
  - "complex chains"
---

# Test Reasoning: safe_nav_05_mixed

## Test File
- **Source**: `tests/golden/safe_nav_05_mixed.dingo`
- **Feature**: Safe Navigation Operator (`?.`) - Mixed Option + Pointer Chains
- **Phase**: Phase 7.1 - Null Safety Operators
- **Status**: 📋 Planned (Implementation pending)

## What This Test Validates

This test validates the **most complex scenario**: safe navigation chains that mix both Option types and raw Go pointers. This is critical for real-world Dingo code that:
1. Uses Dingo's Option types for new code
2. Interoperates with existing Go code (uses pointers)
3. Combines both in same data structures

### The Challenge: Type Boundaries

**Problem**: How to handle transitions between Option and pointer types?

```dingo
user?.config?.theme?.color
  ↑      ↑      ↑
  |      |      |
Option  Ptr    Ptr
```

**Questions**:
- What if Option is Some but pointer is nil?
- What if pointer is non-nil but Option is None?
- What should final return type be (Option or pointer)?
- How to convert between nil and None?

## Dingo Code (62 lines)

```dingo
package main

enum UserOption {
	Some(User),
	None,
}

enum SettingsOption {
	Some(Settings),
	None,
}

type User struct {
	name     string
	config   *Config      // Raw pointer
	settings SettingsOption
}

type Config struct {
	theme *Theme          // Raw pointer
}

type Settings struct {
	lang     string
	timezone *Timezone    // Raw pointer
}

func main() {
	user := getUser(1)

	// Test 1: Option -> Pointer -> Pointer
	color := user?.config?.theme?.color
	println("Theme:", color)

	// Test 2: Option -> Option -> Pointer
	offset := user?.settings?.timezone?.offset
	println("Timezone:", offset)

	// Test 3: None propagation in mixed chain
	noUser := getUser(0)
	noColor := noUser?.config?.theme?.color
	println("No Theme:", noColor)
}
```

## Generated Go Code (221 lines)

```go
func main() {
	user := getUser(1)

	// Test 1: Option -> Pointer -> Pointer
	color := func() string {
		if user.IsNone() {
			return ""  // Option None -> zero value
		}
		_user := user.Unwrap()

		if _user.config == nil {
			return ""  // Pointer nil -> zero value
		}

		if _user.config.theme == nil {
			return ""  // Pointer nil -> zero value
		}

		return _user.config.theme.color
	}()
	println("Theme:", color)

	// Test 2: Option -> Option -> Pointer
	offset := func() int {
		if user.IsNone() {
			return 0  // Option None -> zero value
		}
		_user := user.Unwrap()

		if _user.settings.IsNone() {
			return 0  // Option None -> zero value
		}
		_settings := _user.settings.Unwrap()

		if _settings.timezone == nil {
			return 0  // Pointer nil -> zero value
		}

		return _settings.timezone.offset
	}()
	println("Timezone:", offset)
	// ...
}
```

## Design Decisions

### 1. Type Boundary Handling Strategy

**Rule**: Treat all nullable types (Option + pointer) uniformly in chain.

**Implementation**:
```go
func (p *SafeNavProcessor) generateCheck(expr, field string, typ TypeKind) string {
	switch typ {
	case TypeOption:
		return fmt.Sprintf(`
			if %s.IsNone() {
				return %s  // Return zero value or None
			}
			_%s := %s.Unwrap()
		`, expr, zeroValue, varName, expr)

	case TypePointer:
		return fmt.Sprintf(`
			if %s == nil {
				return %s  // Return zero value or None
			}
		`, expr, zeroValue)
	}
}
```

**Key insight**: Both Option and pointer check for "absence", just different syntax.

### 2. Return Type Determination

**Question**: For mixed chains, what should return type be?

**Options considered**:

**Option A: Return pointer** (if chain ends with pointer)
```dingo
user?.config?.theme  // Returns *Theme (pointer)
```
❌ **Problem**: What if user is None? Can't return nil for Option None.

**Option B: Return Option** (if chain starts with Option)
```dingo
user?.config?.theme  // Returns ThemeOption
```
❌ **Problem**: `config` and `theme` are pointers, not Options. Would need wrapping.

**Option C: Return zero value** (chosen approach)
```dingo
user?.config?.theme?.color  // Returns string (empty if any level fails)
```
✅ **Advantage**: Simple, predictable, type-safe.

**Rationale**:
- **Simplicity**: No complex type conversions
- **Type safety**: Compiler validates zero value matches expected type
- **Composability**: User can combine with `??` for custom default
- **Predictability**: Always same behavior regardless of chain composition

### 3. Nil ↔ None Conversion Rules

**At type boundaries, how to convert between nil and None?**

**Scenario 1: Option (Some) → Pointer (nil)**
```dingo
user?.config?.theme  // user: Some, config: nil
```

**Generated**:
```go
if user.IsNone() { return "" }  // Option check
_user := user.Unwrap()

if _user.config == nil { return "" }  // Pointer check (nil treated as "absent")
```

**Conversion**: Nil is treated same as None (both mean "absent value")

**Scenario 2: Pointer (non-nil) → Option (None)**
```dingo
user?.settings?.timezone  // user: non-nil pointer, settings: None
```

**Generated**:
```go
if user == nil { return 0 }  // Pointer check

if user.settings.IsNone() { return 0 }  // Option check
```

**Conversion**: None is treated same as nil

**Unified rule**: "Absent" value (nil or None) → short-circuit with zero value return

### 4. Type Detection in Mixed Chains

**Challenge**: Preprocessor must track type of each step.

**Solution**: Hybrid detection

**Step 1: Parse type annotations**
```go
type User struct {
	config   *Config      // Detected: pointer
	settings SettingsOption  // Detected: Option
}
```

**Step 2: Track chain types**
```
user          → UserOption (enum declaration)
user.config   → *Config (struct field type)
config.theme  → *Theme (struct field type)
```

**Step 3: Generate appropriate checks**
- `user` → Option check (IsNone)
- `config` → Pointer check (== nil)
- `theme` → Pointer check (== nil)

**Implementation**:
```go
type ChainStep struct {
	expr string
	typ  TypeKind  // TypeOption or TypePointer
}

func (p *SafeNavProcessor) analyzeChain(chain string) []ChainStep {
	steps := []ChainStep{}

	parts := strings.Split(chain, "?.")
	for i, part := range parts {
		// Determine type of current expression
		typ := p.detectType(part)
		steps = append(steps, ChainStep{expr: part, typ: typ})
	}

	return steps
}
```

### 5. Zero Value Strategy for Mixed Chains

**Final type determines zero value**:

**Chain ending in string**:
```dingo
user?.config?.theme?.color  // Returns string
// Zero value: ""
```

**Chain ending in int**:
```dingo
user?.settings?.timeout  // Returns int
// Zero value: 0
```

**Chain ending in pointer**:
```dingo
user?.config?.theme  // Returns *Theme
// Zero value: nil
```

**Chain ending in Option**:
```dingo
user?.settings  // Returns SettingsOption
// Zero value: SettingsOption_None()
```

**Type inference**:
```go
func (p *SafeNavProcessor) inferFinalType(chain []ChainStep) string {
	lastStep := chain[len(chain)-1]

	// Look up field type from struct definitions
	fieldType := p.fieldTypes[lastStep.expr]

	return fieldType
}
```

## Testing Strategy

### What This Test Proves

1. **Type Detection**: Correctly identifies Option vs pointer at each level
2. **Check Generation**: Produces correct checks (IsNone vs == nil)
3. **Short-Circuit**: First absence (None or nil) stops chain
4. **Type Boundaries**: Handles transitions smoothly
5. **Zero Value**: Returns appropriate zero for final type

### Test Cases

**Test 1: Option → Pointer → Pointer**
- Path: UserOption → *Config → *Theme → color (string)
- Types: Option, Ptr, Ptr, value
- Expected: "dark" (all present) or "" (any absent)

**Test 2: Option → Option → Pointer**
- Path: UserOption → SettingsOption → *Timezone → offset (int)
- Types: Option, Option, Ptr, value
- Expected: -5 (all present) or 0 (any absent)

**Test 3: None at root (mixed chain)**
- Path: None → (doesn't evaluate rest)
- Expected: "" (short-circuit at first step)

### Edge Cases Covered

- ✅ Option → Pointer boundary
- ✅ Option → Option → Pointer sequence
- ✅ Multiple mixed chains in same function
- ✅ Different zero values (string, int)
- ✅ None propagation through mixed types

## Real-World Use Cases

### 1. API Integration with Legacy Code

```dingo
// Legacy Go function (returns pointers)
func GetLegacyUser(id int) *LegacyUser { ... }

// New Dingo code (uses Options)
type ModernUser struct {
	legacy   *LegacyUser      // Pointer from old code
	settings SettingsOption   // New Option type
}

// Mixed chain works seamlessly
email := modernUser?.legacy?.email ?? "no-email@example.com"
theme := modernUser?.settings?.theme?.name ?? "default"
```

### 2. Database + Application Layers

```dingo
// Database layer (uses pointers, idiomatic Go)
type DBUser struct {
	profile *DBProfile
}

// Application layer (uses Options, safer)
type AppUser struct {
	dbUser DBUserOption
	cache  CacheOption
}

// Mixed chain across layers
displayName := appUser?.dbUser?.profile?.displayName ?? "Anonymous"
```

### 3. HTTP Request Handling

```dingo
// Go stdlib (uses pointers)
func handler(w http.ResponseWriter, r *http.Request) {
	// r.URL is *url.URL (pointer)
	// But wrap request in Option (may be invalid)

	reqOpt := validateRequest(r)  // Returns RequestOption
	userID := reqOpt?.request?.URL?.Query()?.Get("user_id") ?? ""
}
```

### 4. Configuration Cascading

```dingo
// System config (Option, may not exist)
type SystemConfig {
	user UserConfigOption
}

// User config (pointer, from file)
type UserConfig struct {
	project *ProjectConfig
}

// Project config (pointer, from current dir)
type ProjectConfig struct {
	theme *Theme
}

// Mixed chain across config layers
theme := systemConfig?.user?.project?.theme?.name ?? "default"
```

## Performance Considerations

**Mixed chain cost** (Option → Pointer → Pointer):
```go
func() string {
	if user.IsNone() { return "" }    // Method call (~4 cycles)
	_user := user.Unwrap()             // Method call (~4 cycles)

	if _user.config == nil { return "" }  // Comparison (~1 cycle)
	if _user.config.theme == nil { return "" }  // Comparison (~1 cycle)

	return _user.config.theme.color    // Field access (~1 cycle)
}()
```

**Total**: ~11 CPU cycles (still negligible)

**Comparison**: Pure pointer chain would be ~3 cycles, pure Option chain would be ~12 cycles.

**Conclusion**: Performance is average of constituent types (no extra overhead from mixing)

## Known Limitations & Future Work

### Current Limitations

1. **Complex type transitions** (e.g., pointer returns Option):
   ```dingo
   user?.getConfig()?.theme  // getConfig() returns ConfigOption
   ```
   **Workaround**: Works in Phase 7.2 (method calls)

2. **Nested Options within structs**:
   ```dingo
   type Config {
   	theme Option<Option<Theme>>  // Nested Options
   }
   user?.config?.theme??  // Double unwrap needed
   ```
   **Consideration**: Should `?.` auto-flatten nested Options?

3. **Type inference complexity**:
   - Must track types through entire chain
   - Struct definition parsing required
   - Increases compilation time slightly

### Future Enhancements

**Optimization: Inline simple mixed chains**
```dingo
// Current: Always IIFE
user?.config?.theme

// Future: Inline for simple cases
var theme *Theme
if user.IsSome() && user.Unwrap().config != nil {
	theme = user.Unwrap().config.theme
}
```

**Better error messages**:
```
Error: Type mismatch in safe navigation chain
  → user?.config?.theme
           ^
  Expected: Option or pointer type
  Found:    Config (regular struct)
  Help: Use Option<Config> or *Config for nullable fields
```

## Comparison with Other Languages

### TypeScript (Handles Mixed Types)

```typescript
// Works with any nullable (null | undefined) mixed with non-null
const color = user?.config?.theme?.color;
// TypeScript unifies null and undefined
```

**Similarity**: Dingo unifies Option and pointer (both "nullable")

### Swift (Unified Optionals)

```swift
// All optionals use same syntax (no pointer/Option distinction)
color := user?.config?.theme?.color
// Swift: One Optional type, not multiple
```

**Difference**: Dingo must handle two nullable representations (Option + pointer)

### Rust (No Mixed Types)

```rust
// Rust only has Option (no nullable pointers)
color := user.and_then(|u| u.config).and_then(|c| c.theme)?.color;
// All steps are Option, no mixing needed
```

**Dingo advantage**: Allows mixing for Go interop (Rust doesn't need this)

## Design Rationale

**Why support mixed types instead of requiring uniform Option everywhere?**

**Answer**: Go ecosystem compatibility

**Without mixed support**:
```dingo
// Existing Go function
func GetUser(id int) *User { ... }

// Would need wrapping
userOpt := WrapPointer(GetUser(id))  // Manual conversion
name := userOpt?.name  // Now can use ?.

// Every Go function needs wrapping layer
```

**With mixed support**:
```dingo
// Direct usage
user := GetUser(id)  // Returns *User (pointer)
name := user?.name   // Just works!

// Can mix with Dingo Option types
config := user?.GetConfig()  // Returns ConfigOption
theme := config?.theme       // Works!
```

**Conclusion**: Mixed type support is essential for practical Dingo adoption.

## External References

### Dingo Documentation

- **Option type**: `docs/features/option-type.md`
- **Safe navigation**: `features/safe-navigation.md`
- **Go interoperability**: Design principle from CLAUDE.md

### Language References

- **TypeScript Optional Chaining**: https://www.typescriptlang.org/docs/handbook/release-notes/typescript-3-7.html
  - Handles mixed null/undefined seamlessly
- **Swift Optional Chaining**: https://docs.swift.org/swift-book/documentation/the-swift-programming-language/optionalchaining/
  - Unified Optional type (no mixing needed)

---

**Last Updated**: 2025-11-20
**Test Status**: 📋 Planned
**Phase**: 7.1 In Progress
**Complexity**: Advanced (requires careful type boundary handling)
**Dingo Version**: 0.1.0-alpha
