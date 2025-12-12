# Golden Test Reasoning: safe_nav_08_combined

## Purpose

Demonstrate complex, real-world integration of safe navigation (`?.`) and null coalescing (`??`) operators in scenarios developers actually encounter:
- API response handling with nested optional fields
- Configuration parsing with sensible defaults
- Mixed property access and method calls with fallbacks
- Expression-level usage in comparisons and data structures

## Test Scenarios

### Scenario 1: API Response Handling (Lines 84-96)

**Real-world use case**: Extracting data from JSON API responses where fields may be missing.

```dingo
userBio := apiUser?.profile?.bio ?? "No bio available"
userWebsite := apiUser?.profile?.website ?? "https://example.com"
reputation := apiUser?.profile?.reputation ?? 0
userCity := apiUser?.address?.city ?? "Unknown City"
```

**Why this is important**:
- APIs often return partial data (404s, incomplete responses)
- Nested optionality is common (user exists but profile doesn't, profile exists but bio is empty)
- Need different types of defaults (strings, numbers)
- Shows 3-level deep chaining (`apiUser?.profile?.bio`)

**Expected generated code**:
- IIFE for each safe nav chain
- Option checks: `IsNone()` → return `None()`
- Unwrap at each level
- Final coalesce: `IsSome()` check with fallback

### Scenario 2: Config Parsing with Fallbacks (Lines 98-120)

**Real-world use case**: Loading configuration files with partial/missing values, providing sensible defaults.

```dingo
dbHost := config?.database?.host ?? "localhost"
dbPort := config?.database?.port ?? 5432
rateLimitRPS := config?.api?.rateLimiter?.requestsPerSec ?? 100
```

**Why this is important**:
- Demonstrates pointer types (Go stdlib style: `*string`, `*int`)
- Shows 3-level deep nesting (`config?.api?.rateLimiter?.enabled`)
- Tests mixed pointer types (bool, int, string)
- Real pattern: missing config → use defaults

**Expected generated code**:
- Nil checks for pointers: `if config == nil { return nil }`
- Different from Option mode (no `Unwrap()`)
- Final result is dereferenced value with fallback

### Scenario 3: Mixed Property and Method Calls (Lines 122-134)

**Real-world use case**: Chaining object access and method calls with safety.

```dingo
displayName := user2?.getDisplayName() ?? "Anonymous"
profileURL := user2?.profile?.formatURL() ?? "/default-profile"
formattedAddress := user2?.formatAddress("short")?.city ?? "N/A"
```

**Why this is important**:
- Tests method calls returning Option types
- Shows property-then-method chaining
- Demonstrates method-with-args followed by property
- Common pattern in OOP-style Go code

**Expected generated code**:
- Method call preservation: `user.getDisplayName()` kept in generated code
- Arguments passed through: `user.formatAddress("short")`
- Method return type handled (assumes returns Option)
- Chaining continues after method calls

### Scenario 4: Expression-level Fallbacks (Lines 136-147)

**Real-world use case**: Function calls as fallback values, chained fallbacks.

```dingo
city3 := user3?.address?.city ?? getDefaultCity()
country3 := user3?.address?.country ?? detectCountryFromIP()
primaryEmail := user3?.profile?.getEmail() ?? user3?.getBackupEmail() ?? "support@example.com"
```

**Why this is important**:
- Fallback can be function call, not just literal
- Tests chained coalescing (`a ?? b ?? c`)
- Shows smart defaults (call function to compute fallback)
- Common in production code (fallback chains)

**Expected generated code**:
- Right operand is expression, not literal
- Chained `??` becomes nested checks
- Function calls preserved in generated code
- Single IIFE for entire `a ?? b ?? c` chain

### Scenario 5: Nested Safe Nav in Expressions (Lines 149-164)

**Real-world use case**: Using safe navigation results in comparisons, data structures.

```dingo
areSameCity := (user4?.address?.city ?? "") == (user5?.address?.city ?? "")
jsonData := map[string]interface{}{
    "name": user4?.name ?? "unknown",
    "city": user4?.address?.city ?? "unknown",
}
```

**Why this is important**:
- Safe nav not always standalone statement
- Used in comparisons, map literals, function args
- Tests parenthesization and operator precedence
- Real pattern: building JSON responses with optional data

**Expected generated code**:
- Each safe nav chain in its own IIFE
- Results used in larger expression
- Proper parenthesization maintained
- Map literal construction works correctly

## Code Generation Expectations

### For Option<T> Types

**Input**:
```dingo
userBio := apiUser?.profile?.bio ?? "No bio available"
```

**Expected Output**:
```go
userBio := func() string {
    __safeNav := func() StringOption {
        if apiUser.IsNone() {
            return StringOption_None()
        }
        __apiUser0 := apiUser.Unwrap()
        if __apiUser0.profile.IsNone() {
            return ProfileOption_None()
        }
        __apiUser1 := __apiUser0.profile.Unwrap()
        return __apiUser1.bio
    }()

    if __safeNav.IsSome() {
        return __safeNav.Unwrap()
    }
    return "No bio available"
}()
```

### For Pointer Types

**Input**:
```dingo
dbHost := config?.database?.host ?? "localhost"
```

**Expected Output**:
```go
dbHost := func() string {
    __safeNav := func() *string {
        if config == nil {
            return nil
        }
        if config.database == nil {
            return nil
        }
        return config.database.host
    }()

    if __safeNav != nil {
        return *__safeNav
    }
    return "localhost"
}()
```

### For Chained Coalescing

**Input**:
```dingo
primaryEmail := user3?.profile?.getEmail() ?? user3?.getBackupEmail() ?? "support@example.com"
```

**Expected Output**:
```go
primaryEmail := func() string {
    // First option
    __opt1 := func() StringOption {
        if user3.IsNone() {
            return StringOption_None()
        }
        __user30 := user3.Unwrap()
        if __user30.profile.IsNone() {
            return ProfileOption_None()
        }
        __user31 := __user30.profile.Unwrap()
        return __user31.getEmail()
    }()

    if __opt1.IsSome() {
        return __opt1.Unwrap()
    }

    // Second option
    __opt2 := func() StringOption {
        if user3.IsNone() {
            return StringOption_None()
        }
        __user32 := user3.Unwrap()
        return __user32.getBackupEmail()
    }()

    if __opt2.IsSome() {
        return __opt2.Unwrap()
    }

    // Final fallback
    return "support@example.com"
}()
```

## Edge Cases Covered

1. **Deep nesting**: 3-level chains (`a?.b?.c`)
2. **Mixed types**: Option and pointer in same test
3. **Method calls**: With and without arguments
4. **Chained coalescing**: `a ?? b ?? c`
5. **Function fallbacks**: `value ?? getDefault()`
6. **Expression context**: Safe nav in comparisons, maps
7. **Type variety**: string, int, bool with different defaults

## Success Criteria

1. ✅ **Compiles**: Generated .go file compiles without errors
2. ✅ **Type safety**: All type transitions handled correctly
3. ✅ **Correct behavior**: Runs and produces expected output
4. ✅ **Idiomatic Go**: Generated code looks hand-written
5. ✅ **Source maps**: Position mappings accurate for debugging
6. ✅ **Performance**: No unnecessary allocations or copies

## Test Maintenance Notes

**When to update this test**:
- Adding new safe navigation features (array indexing, etc.)
- Adding new coalescing optimizations
- Changing code generation patterns
- Adding new Option/pointer types

**Related tests**:
- `safe_nav_01_basic.dingo` - Simple property access
- `safe_nav_02_methods.dingo` - Method call basics
- `null_coalesce_02_integration.dingo` - Basic integration
- `showcase_01_api_server.dingo` - Full application example

## Real-World Value

This test demonstrates patterns developers will actually use:

1. **API clients**: Handling partial responses from REST APIs
2. **Config loaders**: Parsing YAML/JSON with defaults
3. **Database queries**: Handling nullable columns
4. **User input**: Processing optional form fields
5. **Service integration**: Dealing with unreliable external services

The 67% reduction in error handling boilerplate claimed in Dingo's value proposition is demonstrated here - compare the Dingo code to equivalent Go with manual nil checks.
