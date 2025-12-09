# Dingo Enum Variant Naming Conventions Analysis

## 1. Current Dingo Approach
Dingo uses PascalCase for both enum types and variants:
```
enum PaymentStatus { Pending, Completed(orderId: int), Failed(error: string) }
enum Result { Ok(value: T), Err(error: E) }
```
Transpiles to Go structs with PascalCase constructors (e.g., `ResultOk[T,E](T) Result[T,E]`).

## 2. Language Comparisons

### Go Conventions (Target Language)
- Exported types/structs: `PascalCase`
- Exported functions/methods: `PascalCase`
- Constants: `UPPER_SNAKE_CASE`
- Unexported: `camelCase`
- Go sum type proposals (#19412): Suggest `PascalCase` variants (e.g., `StatusPending`, `StatusCompleted`)

### Rust (Primary Inspiration)
- Enums: `PascalCase` (e.g., `Result[T, E]`)
- Variants: `PascalCase` (e.g., `Ok(T)`, `Err(E)`)
- Usage: `match result { Ok(v) => ..., Err(e) => ... }`
- No distinction; consistent casing.

### Swift
- Enums: `PascalCase` (e.g., `PaymentStatus`)
- Cases: `lowercase` (e.g., `case pending`, `case completed(orderId: Int)`)
- Usage: `switch status { case .pending: ... case .completed(let id): ... }`
- Visual distinction: lowercase cases.

### Kotlin (Sealed Classes)
- Sealed classes: `PascalCase`
- Subclasses/objects: `PascalCase` (e.g., `object Pending : PaymentStatus()`)
- Usage: `when (status) { Pending -> ..., Completed(id) -> ... }`
- Consistent PascalCase.

## 3. Readability & Usability Analysis

### Benefits of PascalCase Variants (Current)
- **Consistency**: Matches Go type/constructor naming.
- **Interop**: Generated Go code uses natural `PaymentStatusCompleted(id)` constructors.
- **Pattern Matching**: `match status { PaymentStatus::Pending => ... }` reads clearly.
- **IDE Support**: Gopls autocompletes variants as types.

### Potential Issues
- **Namespace Collision**: `PaymentStatus.Pending` could look like nested type.
- **Long Names**: `ResultOk` vs `Ok` (but Go favors explicit).
- **No Visual Hierarchy**: Type and variant blend together.

### Alternatives Considered
1. **lowercase variants** (Swift-style):
   ```
   enum PaymentStatus { pending, completed(orderId: int) }
   ```
   - Pros: Clear distinction (`.pending` in match).
   - Cons: Breaks Go export conventions; awkward constructors (`paymentStatusPending`?).

2. **UPPER_SNAKE_CASE variants**:
   ```
   enum PaymentStatus { PENDING, COMPLETED(orderId: int) }
   ```
   - Pros: Stands out as constants.
   - Cons: Ugly in pattern matching; non-idiomatic for Go types.

3. **Prefixed PascalCase** (e.g., `PendingStatus` → no, variants are siblings).

## 4. Recommendations
**Primary: Retain PascalCase for variants (Status Quo).**
- Rationale: Maximizes Go interop/readability. Matches Rust (primary syntax inspo) and Go proposals.
- Transpilation: `PaymentStatus{tag: 0}` → `PaymentStatusPending(id)`
- Pattern Match: `PaymentStatus::Pending => {}, PaymentStatus::Completed(id) => {}`
- Future-Proof: Works with guards, tuples.

**Secondary Option: Scoped lowercase** (if distinction needed):
- Variants: `lowercase`
- But requires `::pending` syntax, complicating preprocessor.

**Trade-offs Table**

| Approach | Go Interop | Readability | PM Syntax | Complexity |
|----------|------------|-------------|-----------|------------|
| PascalCase | ✅ High | ✅ Good | ✅ Clean | Low |
| lowercase | ❌ Low | ✅ Distinct | ✅ Swift-like | Medium |
| UPPER_CASE | ⚠️ Medium | ❌ Poor | ❌ Verbose | Low |

## 5. Examples in Context

### Current (Recommended)
```dingo
enum HttpMethod { GET, POST, PUT, DELETE }

fn handle(method: HttpMethod) {
  match method {
    HttpMethod::GET => { ... }
    HttpMethod::POST => { ... }
  }
}
```
Generated Go:
```go
type HttpMethod struct { tag uint8 }
func HttpMethodGET() HttpMethod { ... }

switch method.tag {
case 0: // GET
}
```

### Swift-style Alternative (Not Recommended)
```dingo
enum HttpMethod { get, post }  // Awkward constructors
```

## 6. Implementation Notes
- No changes needed to preprocessor/enum processor.
- Update docs/examples if adopting alternative.
- Test golden files: Ensure `sum_types_*.go.golden` uses PascalCase constructors.

## Conclusion
PascalCase variants best balance Go conventions, readability, and simplicity. No changes recommended.