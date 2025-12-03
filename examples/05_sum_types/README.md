# Sum Types (`enum`)

## Scenario
API response types and payment status tracking. Sum types are ideal for modeling "exactly one of these possibilities" - a fundamental pattern in APIs and state machines.

## The Problem
Go lacks built-in sum types, leading to:
1. Using interfaces + type assertions (verbose, error-prone)
2. Struct with multiple optional fields (wasted memory, confusing)
3. String constants (no type safety)

```go
// Approach 1: Interface + assertions
type APIResponse interface{}
// No way to know all possible types at compile time

// Approach 2: Big struct with optionals
type APIResponse struct {
    Type string
    TransactionID *string  // Only set for success
    ErrorMessage *string   // Only set for errors
    RetryAfter *int        // Only set for rate limit
    // Which fields are valid? Who knows!
}
```

## Dingo Solution
Sum types with `enum` keyword:

```dingo
enum APIResponse {
    Success { transactionID: string, amount: float64 }
    ValidationError { field: string, message: string }
    RateLimited { retryAfter: int }
}
```

## Comparison

| Aspect | Go interface | Go struct | Dingo enum |
|--------|--------------|-----------|------------|
| Type safety | Partial | None | Full |
| Exhaustive | No | No | Yes |
| Memory | Optimal | Wasted | Optimal |
| Documentation | Poor | Poor | Self-documenting |

## Key Points

### Defining Sum Types
```dingo
enum Name {
    Variant1                            // Unit variant (no data)
    Variant2 { field: Type }            // Struct variant
    Variant3 { a: int, b: string }      // Multiple fields
}
```

### Creating Values
```dingo
APIResponse.Success{transactionID: "TX-1", amount: 99.99}
PaymentStatus.Pending{}
PaymentStatus.Failed{reason: "declined", canRetry: true}
```

### Pattern Matching
Sum types pair perfectly with `match`:
```dingo
match response {
    Success { transactionID, amount } => ...,
    ValidationError { field, message } => ...,
    // Compiler error if cases missing
}
```

### When to Use
- API responses (different shapes per status)
- State machines (each state has different data)
- AST nodes (different node types)
- Command/message types (each with own payload)

## Pattern: State Machines
Sum types make state machines explicit:
```dingo
enum PaymentStatus {
    Pending
    Processing { processorID: string }
    Completed { transactionID: string }
    Failed { reason: string, canRetry: bool }
}
```
- Valid transitions become obvious
- Impossible states are unrepresentable
- Each state carries exactly the data it needs

## Generated Code
The transpiler generates:
- Interface type with private marker method
- Struct for each variant
- Marker method implementations
- No runtime overhead beyond interface dispatch
