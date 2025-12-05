package ast

// TransformSource applies all AST-based Dingo → Go transformations.
// This is the unified entry point for the new AST-based transpiler.
//
// Transformation order (critical for correctness):
// 1. Enums - Creates types referenced by match patterns
// 2. Let - Establishes variable declarations
// 3. Lambdas - May appear in match bodies or other expressions
// 4. Match - Generates expressions that might have ? or other operators
// 5. Error Propagation (?) - Has precedence over ?? and ?.
// 6. Ternary - Simpler operator, before null ops
// 7. Null Coalescing (??) - Related to safe navigation
// 8. Safe Navigation (?.) - Related to null coalescing
// 9. Tuples - May contain any expression type, so transform last
//
// Returns:
//   - Transformed Go source code
//   - Source mappings for LSP integration
//   - Error if transformation fails
func TransformSource(src []byte) ([]byte, []SourceMapping, error) {
	var allMappings []SourceMapping

	// Phase 1: Declaration-level transforms
	// Enums create types that other features reference
	enumRegistry := make(map[string]string)
	src, enumRegistry = TransformEnumSource(src)
	// Note: Enum transform currently doesn't return mappings, only registry

	// Let declarations establish variables
	src, letMappings := TransformLetSource(src)
	allMappings = append(allMappings, letMappings...)

	// Phase 2: Expression-level transforms (order matters!)

	// Lambdas may appear in match bodies or other expressions
	src, lambdaMappings := TransformLambdaSource(src)
	allMappings = append(allMappings, lambdaMappings...)

	// Match expressions generate complex code that may contain other operators
	src, matchMappings := TransformMatchSource(src, enumRegistry)
	allMappings = append(allMappings, matchMappings...)

	// Error propagation (?) has high precedence
	src, errorPropMappings := TransformErrorPropSource(src)
	allMappings = append(allMappings, errorPropMappings...)

	// Ternary operator (simpler, before null ops)
	src, ternaryMappings := TransformTernarySource(src)
	allMappings = append(allMappings, ternaryMappings...)

	// Null coalescing (??) - null safety feature
	src, coalesceMappings := TransformNullCoalesceSource(src)
	allMappings = append(allMappings, coalesceMappings...)

	// Safe navigation (?.) - null safety feature
	src, safeNavMappings := TransformSafeNavSource(src)
	allMappings = append(allMappings, safeNavMappings...)

	// Tuples last - may contain any expression type
	src, tupleMappings := TransformTupleSource(src)
	allMappings = append(allMappings, tupleMappings...)

	return src, allMappings, nil
}
