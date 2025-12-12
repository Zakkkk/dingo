# User Request: Phase 2.2 - Complete Error Propagation

**Context**: Continuation of migration sessions 20251117-154457 and 20251117-181304

## Previous Accomplishments

**Session 20251117-154457 (Phase 0-1)**:
- ✅ Deleted old Participle parser
- ✅ Created infrastructure (source maps, preprocessor framework, go/parser wrapper, transformer)

**Session 20251117-181304 (Phase 2.1)**:
- ✅ Implemented error propagation preprocessor foundation
- ✅ Basic `expr?` transformation working
- ✅ Unit tests passing for simple cases

## Current Task: Phase 2.2

**Goal**: Complete error propagation feature and make all 8 golden tests pass

### Specific Requirements

1. **Polish error_prop.go Preprocessor**:
   - Handle multiple `?` in single expression: `x := foo()? + bar()?`
   - Handle complex expressions: `obj.Method(arg1, arg2)?`
   - Determine correct zero values (not hardcoded nil/0)
   - Better expression boundary detection (not just regex)

2. **Wire Up CLI Integration**:
   - Update `cmd/dingo/build.go` to use new parser
   - Connect preprocessor → go/parser → code generation pipeline
   - Ensure source maps work end-to-end

3. **Make Golden Tests Pass**:
   - Run all 8 error propagation tests: `error_prop_01` through `error_prop_08`
   - Compare generated output with `.go.golden` files
   - Fix any discrepancies

4. **Handle Edge Cases**:
   - Nested function calls with `?`
   - Method chains: `obj.Foo()?.Bar()?.Baz()?`
   - Multiple returns: `return x?, y?`
   - Error in conditional: `if validate(x)? { ... }`

## Success Criteria

1. ✅ All 8 error propagation golden tests pass
2. ✅ Generated Go code compiles
3. ✅ CLI works: `dingo build file.dingo` produces correct output
4. ✅ Source maps correctly map errors
5. ✅ Code is clean and well-tested

## Reference Files

- Previous implementation: `pkg/preprocessor/error_prop.go`
- Previous notes: `ai-docs/sessions/20251117-181304/02-implementation/implementation-notes.md`
- Architecture: `ai-docs/sessions/20251117-154457/01-planning/new-architecture.md`
- Golden tests: `tests/golden/error_prop_*.dingo`
