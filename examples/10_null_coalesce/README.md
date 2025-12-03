# Null Coalescing (`??`)

## Scenario
Configuration loading with fallback defaults from multiple sources (config file, environment variables, hardcoded defaults). This pattern appears in virtually every application.

## The Problem
Go requires verbose nil checks for every fallback:
```go
func GetHost(config *AppConfig) string {
    if config != nil && config.Host != nil {
        return *config.Host
    }
    return "localhost"
}
```

With multiple fallback sources, this becomes unwieldy:
```go
// Check config, then env, then default
if config != nil && config.Host != nil {
    return *config.Host
}
if env := os.Getenv("HOST"); env != "" {
    return env
}
return "localhost"
```

## Dingo Solution
The `??` operator provides fallback values:

```dingo
func GetHost(config: *AppConfig) string {
    return config?.Host ?? "localhost"
}

// With multiple fallbacks
func GetLogLevel(config: *AppConfig) string {
    return config?.LogLevel ?? os.Getenv("LOG_LEVEL") ?? "info"
}
```

## Comparison

| Fallback Depth | Go | Dingo |
|----------------|-----|-------|
| 1 level | 5 lines | `a ?? b` |
| 2 levels | 9 lines | `a ?? b ?? c` |
| 3 levels | 13 lines | `a ?? b ?? c ?? d` |

## Key Points

### Syntax
```dingo
value ?? fallback
```

### Evaluation
- Returns `value` if non-nil/non-zero
- Returns `fallback` if `value` is nil/zero
- Short-circuits: `fallback` only evaluated if needed

### Chaining
Multiple fallbacks left to right:
```dingo
primary ?? secondary ?? tertiary ?? default
```

### Combining with `?.`
Powerful pattern for nested access:
```dingo
config?.Database?.Host ?? os.Getenv("DB_HOST") ?? "localhost"
```

### What Triggers Fallback
- Nil pointers: `*string`, `*int`, etc.
- Empty strings (when coalescing with strings)
- Zero values depend on context

### When to Use
- Configuration with defaults
- Environment variable fallbacks
- Optional function parameters
- Cache misses with fallback

### When NOT to Use
- When zero is a valid value (use Option instead)
- When you need to distinguish nil from zero
- Complex fallback logic (use explicit if)

## Common Patterns

### Config Priority Chain
```dingo
// CLI flag > env var > config file > default
value := flagValue ?? os.Getenv("KEY") ?? configValue ?? "default"
```

### Optional Parameters
```dingo
func Connect(host: *string, port: *int) {
    actualHost := host ?? "localhost"
    actualPort := port ?? 5432
    // ...
}
```

### Cache with Fallback
```dingo
func GetUser(id: int) User {
    return cache.Get(id) ?? db.Load(id) ?? User{}
}
```

## Generated Code
The transpiler generates:
- Nil checks for each level
- Short-circuit evaluation
- Intermediate variables for complex chains
- No runtime overhead beyond nil checks
