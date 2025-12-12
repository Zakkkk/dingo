# User Clarifications for Phase 1.6

## Architecture Decision
**Keep plugin-based approach** - Error propagation remains as a plugin in `pkg/plugin/builtin/`. Plugins can contain core features, not just optional ones.

## Type Inference Level
**Full go/types integration** - Implement comprehensive type inference using Go's standard `go/types` package for accurate zero value generation across all types.

## Phase 1.6 Scope (All Features Included)
1. ✅ **Statement context** (`x := expr?`)
2. ✅ **Expression context** (`return expr?`) - includes statement lifting
3. ✅ **Error wrapping** (`expr? "message"`) - string parsing and fmt.Errorf generation
4. ✅ **VLQ source map encoding** - proper IDE integration with go-sourcemap library

## Implementation Notes
- This is an ambitious scope that includes all major features
- Full go/types integration is the most robust approach
- Timeline will be longer due to comprehensive feature set
- Plugin architecture provides flexibility for future extensions
