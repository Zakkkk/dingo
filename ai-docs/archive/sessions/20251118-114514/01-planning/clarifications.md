# User Clarifications for Phase 3

## Question 1: None Constant Syntax
**Decision**: Implement in Phase 3 - Type-context-aware None constant

**Implication**: We need to implement proper constant detection that understands which `Option_T` type is being used based on context. This is more complex but provides better user experience with `None` constant syntax instead of `Option_None()` function calls.

## Question 2: Helper Methods (Map, Filter, AndThen, etc.)
**Decision**: Implement in Phase 3 - Complete all 39 builtin tests

**Implication**: Phase 3 scope expands significantly. We will implement:
- Map, Filter, AndThen for Result[T,E]
- Map, Filter, AndThen for Option[T]
- All other helper methods expected by the test suite
- Timeline extends by 8-12 hours (total: 16-28 hours)

## Question 3: Type Inference Failure Handling
**Decision**: Generate compile error

**Implication**: When type inference fails completely (no go/types, no heuristics), we will:
- Generate a clear compile error message
- Suggest user provide explicit type annotation (when available)
- This requires better error message infrastructure
- More strict but catches type errors earlier

## Remaining Questions (Deferred)
These questions can be addressed during implementation based on technical constraints:

4. **IIFE for composite literals**: Decide based on code generation complexity during A4 implementation
5. **Performance overhead budget**: Measure during A5 implementation, optimize if needed
6. **Partial type checking failures**: Use graceful degradation initially, adjust if issues arise
7. **TypeInferenceService scope**: Start with Result/Option only, generalize if needed
8. **Result[T,E] where E is not error**: Keep current assumption (E is error), revisit in Phase 4

## Updated Scope
Phase 3 is now comprehensive:
- ✅ Fix A5 (go/types integration)
- ✅ Fix A4 (IIFE pattern for literals)
- ✅ Option[T] with type-context-aware None constant
- ✅ Complete helper methods suite (Map, Filter, AndThen, etc.)
- ✅ All 39 builtin plugin tests passing
- ✅ Enhanced error messages for type inference failures

**Estimated timeline**: 16-28 hours over 2-3 days
