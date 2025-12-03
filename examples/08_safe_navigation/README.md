# Safe Navigation (`?.`)

## Scenario
Accessing deeply nested configuration where any level might be nil. This is extremely common when parsing JSON configs, working with optional relationships, or handling partial data.

## The Problem
Deep nil checks are verbose and error-prone:
```go
func GetSSLCert(config *ServerConfig) string {
    if config != nil {
        if config.Database != nil {
            if config.Database.SSL != nil {
                return config.Database.SSL.CertPath
            }
        }
    }
    return "/etc/ssl/cert.pem"
}
```

Each level of nesting adds another if statement. Real configs often have 4-5 levels.

## Dingo Solution
The `?.` operator short-circuits on nil:

```dingo
func GetSSLCert(config: *ServerConfig) string {
    return config?.Database?.SSL?.CertPath ?? "/etc/ssl/cert.pem"
}
```

## Comparison

| Depth | Go | Dingo |
|-------|-----|-------|
| 1 level | 3 lines | `a?.B` |
| 2 levels | 6 lines | `a?.B?.C` |
| 3 levels | 9 lines | `a?.B?.C?.D` |
| 4 levels | 12 lines | `a?.B?.C?.D?.E` |

## Key Points

### How It Works
- `obj?.field` returns `obj.field` if obj is non-nil
- `obj?.field` returns zero value if obj is nil
- Chains short-circuit at first nil

### Combining with `??`
Provide defaults for the entire chain:
```dingo
config?.Database?.Host ?? "localhost"
```

### With Methods
Safe navigation works with method calls:
```dingo
user?.GetProfile()?.Avatar ?? "default.png"
```

### With Arrays
Access array elements safely:
```dingo
config?.Servers?.[0]?.Host
```

### When to Use
- Nested optional data (configs, JSON)
- API responses with optional fields
- Database records with nullable relations
- Any chain where any part might be nil

### When NOT to Use
- When nil is a bug (let it panic for debugging)
- Performance-critical hot paths (marginal overhead)
- When you need to distinguish "nil" from "zero value"

## Pattern: Config Access
```dingo
// Define sensible defaults for each config value
dbHost := config?.Database?.Host ?? "localhost"
dbPort := config?.Database?.Port ?? 5432
sslEnabled := config?.Database?.SSL?.Enabled ?? false
```

## Generated Code
The transpiler generates:
- Nested if statements for nil checks
- Intermediate variables for each step
- Short-circuit evaluation (stops at first nil)
- No runtime overhead beyond nil checks
