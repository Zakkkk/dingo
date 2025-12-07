// Package typechecker provides exhaustiveness checking for match expressions.
package typechecker

import (
	"go/ast"
	goast "go/ast"
	"go/token"
	"go/types"
	"sort"
	"strings"

	dingoast "github.com/MadAppGang/dingo/pkg/ast"
)

// ExhaustivenessResult holds the result of exhaustiveness checking
type ExhaustivenessResult struct {
	IsExhaustive    bool      // true if all variants are covered
	MissingVariants []string  // Variant names that are not covered (sorted)
	EnumName        string    // The enum type being matched
	HasWildcard     bool      // True if _ or variable pattern present
	Position        token.Pos // For error reporting
}

// ExhaustivenessChecker validates match expressions for exhaustiveness
type ExhaustivenessChecker struct {
	registry *EnumRegistry // Enum type information registry
	checker  *Checker      // go/types checker for type inference
}

// NewExhaustivenessChecker creates a new checker
func NewExhaustivenessChecker(registry *EnumRegistry, checker *Checker) *ExhaustivenessChecker {
	return &ExhaustivenessChecker{
		registry: registry,
		checker:  checker,
	}
}

// Check validates exhaustiveness of a match expression
// isExpression determines if this is an expression context (true) vs statement context (false)
// Expression contexts require exhaustiveness; statement contexts do not
func (ec *ExhaustivenessChecker) Check(match *dingoast.MatchExpr, isExpression bool) *ExhaustivenessResult {
	result := &ExhaustivenessResult{
		Position: match.Match,
	}

	// 1. Infer scrutinee type
	enumName := ec.inferScrutineeType(match.Scrutinee)
	if enumName == "" {
		// Try fallback: check first constructor pattern for enum hint
		enumName = ec.inferFromPatterns(match.Arms)
	}
	if enumName == "" {
		// Cannot determine type - assume exhaustive (benefit of doubt)
		// This can happen with complex expressions or non-enum types
		result.IsExhaustive = true
		return result
	}
	result.EnumName = enumName

	// 2. Get all variants for this enum
	enumInfo := ec.registry.GetEnum(enumName)
	if enumInfo == nil {
		// Enum not found in registry - assume exhaustive
		result.IsExhaustive = true
		return result
	}

	// Build set of all variants
	allVariants := make(map[string]bool)
	for _, v := range enumInfo.Variants {
		allVariants[v.Name] = true
	}

	// 3. Extract covered variants from match arms
	coveredVariants := ec.extractCoveredVariants(match.Arms, enumName)

	// 4. Check for wildcard or variable pattern (covers all)
	if coveredVariants == nil {
		result.IsExhaustive = true
		result.HasWildcard = true
		return result
	}

	// 5. Compute missing variants
	for variant := range allVariants {
		if !coveredVariants[variant] {
			result.MissingVariants = append(result.MissingVariants, variant)
		}
	}

	// 6. Sort missing variants for deterministic output
	sort.Strings(result.MissingVariants)

	result.IsExhaustive = len(result.MissingVariants) == 0
	return result
}

// extractCoveredVariants extracts which variants are fully covered
// IMPORTANT: Guarded patterns do NOT count as covering a variant
// Returns nil to signal "wildcard/variable pattern covers all"
// Returns map[variant]bool for specific coverage
func (ec *ExhaustivenessChecker) extractCoveredVariants(arms []*dingoast.MatchArm, enumName string) map[string]bool {
	covered := make(map[string]bool)

	for _, arm := range arms {
		// Wildcard pattern covers everything
		if _, ok := arm.Pattern.(*dingoast.WildcardPattern); ok {
			return nil // Signal: all covered
		}

		// Variable pattern covers everything
		if _, ok := arm.Pattern.(*dingoast.VariablePattern); ok {
			return nil // Signal: all covered
		}

		// Constructor pattern
		if cp, ok := arm.Pattern.(*dingoast.ConstructorPattern); ok {
			// CRITICAL: Guards make pattern non-exhaustive
			// A guarded pattern does NOT fully cover the variant
			// because the guard might evaluate to false
			if arm.Guard != nil {
				// This pattern does NOT fully cover the variant
				continue
			}

			// Resolve variant name via registry
			_, variantName, ok := ec.registry.NormalizePatternName(cp.Name)
			if ok {
				covered[variantName] = true
			}
		}

		// Or-patterns: all alternatives must be unguarded to cover
		if op, ok := arm.Pattern.(*dingoast.OrPattern); ok {
			if arm.Guard != nil {
				// Guarded or-pattern doesn't cover any variant
				continue
			}

			// Recursively extract variants from or-pattern
			ec.extractFromOrPattern(op, covered)
		}
	}

	return covered
}

// extractFromOrPattern recursively extracts variants from or-pattern
func (ec *ExhaustivenessChecker) extractFromOrPattern(op *dingoast.OrPattern, covered map[string]bool) {
	// Check left pattern
	if cp, ok := op.Left.(*dingoast.ConstructorPattern); ok {
		_, variantName, ok := ec.registry.NormalizePatternName(cp.Name)
		if ok {
			covered[variantName] = true
		}
	} else if nested, ok := op.Left.(*dingoast.OrPattern); ok {
		ec.extractFromOrPattern(nested, covered)
	}

	// Check right pattern
	if cp, ok := op.Right.(*dingoast.ConstructorPattern); ok {
		_, variantName, ok := ec.registry.NormalizePatternName(cp.Name)
		if ok {
			covered[variantName] = true
		}
	} else if nested, ok := op.Right.(*dingoast.OrPattern); ok {
		ec.extractFromOrPattern(nested, covered)
	}
}

// inferFromPatterns infers enum type from constructor patterns in match arms
// This is a fallback when scrutinee type cannot be determined directly
func (ec *ExhaustivenessChecker) inferFromPatterns(arms []*dingoast.MatchArm) string {
	for _, arm := range arms {
		if cp, ok := arm.Pattern.(*dingoast.ConstructorPattern); ok {
			enumName, _, ok := ec.registry.NormalizePatternName(cp.Name)
			if ok {
				return enumName
			}
		}
		// Check or-patterns
		if op, ok := arm.Pattern.(*dingoast.OrPattern); ok {
			if enumName := ec.inferFromOrPattern(op); enumName != "" {
				return enumName
			}
		}
	}
	return ""
}

// inferFromOrPattern recursively searches or-patterns for enum type
func (ec *ExhaustivenessChecker) inferFromOrPattern(op *dingoast.OrPattern) string {
	if cp, ok := op.Left.(*dingoast.ConstructorPattern); ok {
		enumName, _, ok := ec.registry.NormalizePatternName(cp.Name)
		if ok {
			return enumName
		}
	}
	if nested, ok := op.Left.(*dingoast.OrPattern); ok {
		if enumName := ec.inferFromOrPattern(nested); enumName != "" {
			return enumName
		}
	}
	if cp, ok := op.Right.(*dingoast.ConstructorPattern); ok {
		enumName, _, ok := ec.registry.NormalizePatternName(cp.Name)
		if ok {
			return enumName
		}
	}
	if nested, ok := op.Right.(*dingoast.OrPattern); ok {
		return ec.inferFromOrPattern(nested)
	}
	return ""
}

// inferScrutineeType infers the enum type of a match scrutinee
// Uses go/types when available, falls back to registry-based heuristics
func (ec *ExhaustivenessChecker) inferScrutineeType(expr dingoast.Expr) string {
	// Strategy 1: Use go/types if available
	if ec.checker != nil {
		// Try to extract Go AST expression from Dingo AST
		// This is best-effort - if we can't convert, fall through to other strategies
		if goExpr := ec.tryConvertToGoExpr(expr); goExpr != nil {
			if t := ec.checker.TypeOf(goExpr); t != nil {
				typeName := types.TypeString(t, nil)
				// Remove package qualifier if present
				if idx := strings.LastIndex(typeName, "."); idx != -1 {
					typeName = typeName[idx+1:]
				}
				if ec.registry.GetEnum(typeName) != nil {
					return typeName
				}
			}
		}
	}

	// Strategy 2: String-based heuristics from Dingo AST
	exprStr := expr.String()
	exprStr = strings.TrimSpace(exprStr)

	// Try identifier lookup
	if !strings.ContainsAny(exprStr, "()[]{}.,;:") {
		// Simple identifier - try variant or full name lookup
		if info := ec.registry.GetEnumForVariant(exprStr); info != nil {
			return info.Name
		}
		if info := ec.registry.GetEnumForFullName(exprStr); info != nil {
			return info.Name
		}
	}

	// Try constructor call: NewShapePoint() -> Shape
	if strings.HasPrefix(exprStr, "New") && strings.Contains(exprStr, "(") {
		// Extract constructor name
		parenIdx := strings.Index(exprStr, "(")
		constructorName := exprStr[3:parenIdx] // Remove "New"
		if info := ec.registry.GetEnumForFullName(constructorName); info != nil {
			return info.Name
		}
	}

	// Try qualified variant: ShapePoint -> Shape
	if info := ec.registry.GetEnumForFullName(exprStr); info != nil {
		return info.Name
	}

	return "" // Cannot determine
}

// tryConvertToGoExpr attempts to convert a Dingo expression to a Go AST expression
// This is a best-effort conversion for go/types integration
func (ec *ExhaustivenessChecker) tryConvertToGoExpr(expr dingoast.Expr) ast.Expr {
	// If expr is RawExpr, we can't easily convert without parsing
	if raw, ok := expr.(*dingoast.RawExpr); ok {
		// Could parse raw.Text, but that's complex
		// For now, just try identifier case
		if !strings.ContainsAny(raw.Text, "()[]{}.,;:") {
			return &goast.Ident{
				NamePos: raw.StartPos,
				Name:    strings.TrimSpace(raw.Text),
			}
		}
		return nil
	}

	// If we had other Dingo AST types (CallExpr, SelectorExpr, etc.),
	// we could convert them here. For now, return nil.
	return nil
}
