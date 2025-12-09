# User Clarifications for Phase 3 Plan

## Question 1: Go Interoperability (Auto-Wrapping)
**Decision:** Implement BOTH approaches as configuration options in dingo.toml

The user wants flexibility - both opt-in and default-enabled modes should be available as configuration options, similar to the existing dingo.toml system.

**Implementation:**
```toml
[features.result_type]
enabled = true
go_interop = "opt-in"  # Options: "opt-in", "auto", "disabled"

[features.option_type]
enabled = true
go_interop = "opt-in"  # Options: "opt-in", "auto", "disabled"
```

**Modes:**
- `"opt-in"` (default) - Requires explicit `Result.FromGo()` or `Option.FromPtr()` wrapping
- `"auto"` - Automatically wraps `(T, error)` → `Result[T, E]` and `*T` → `Option[T]`
- `"disabled"` - No Go interop, pure Dingo types only

## Question 2: None Type Inference
**Decision:** Option A - Compilation error (safer)

When `None` appears without clear type context, the compiler should fail with a helpful error message:
```
Error: Cannot infer type for None
Help: Add explicit type annotation: let x: Option[YourType] = None
```

This forces users to be explicit and preserves type safety.

## Question 3: Helper Methods Generation
**Decision:** Generate all methods (simple approach)

Generate complete API (IsOk, IsErr, Unwrap, UnwrapOr, Map, Filter, etc.) for every Result/Option instance. Simpler implementation, complete API surface. Binary size optimization can be added in Phase 4+ if needed.

## Question 4: Lambda Integration Priority
**Decision:** Keep in Phase 4 (focused approach)

Phase 3 will focus exclusively on Result/Option types. Lambda functions remain in Phase 4. This maintains timeline and focus, while still delivering complete Result/Option functionality.

---

**Summary:**
- ✅ Both go_interop modes (opt-in default, auto optional) via dingo.toml config
- ✅ None inference: Compilation error (explicit types required)
- ✅ Helper methods: Generate all (complete API)
- ✅ Lambdas: Phase 4 (separate from Result/Option work)
