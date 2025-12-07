package typechecker

import (
	"go/token"
	"testing"

	"github.com/MadAppGang/dingo/pkg/ast"
)

// Helper to create a test registry with enum variants
func createTestRegistry(name string, variants ...string) *EnumRegistry {
	registry := NewEnumRegistry()
	variantInfos := make([]VariantInfo, len(variants))
	for i, v := range variants {
		variantInfos[i] = VariantInfo{
			Name:     v,
			FullName: name + v, // PascalCase concatenation
		}
	}
	if err := registry.RegisterEnum(name, variantInfos); err != nil {
		panic(err) // Test helper - should not fail
	}
	return registry
}

// Test helpers
func makeConstructorPattern(name string) *ast.ConstructorPattern {
	return &ast.ConstructorPattern{
		NamePos: token.Pos(1),
		Name:    name,
	}
}

func makeWildcardPattern() *ast.WildcardPattern {
	return &ast.WildcardPattern{
		Pos_: token.Pos(1),
	}
}

func makeVariablePattern(name string) *ast.VariablePattern {
	return &ast.VariablePattern{
		NamePos: token.Pos(1),
		Name:    name,
	}
}

func makeRawExpr(text string) *ast.RawExpr {
	return &ast.RawExpr{
		StartPos: token.Pos(1),
		EndPos:   token.Pos(len(text) + 1),
		Text:     text,
	}
}

func TestExhaustivenessChecker_ExhaustiveMatch(t *testing.T) {
	registry := createTestRegistry("Shape", "Circle", "Rectangle", "Point")

	checker := NewExhaustivenessChecker(registry, nil)

	// All variants covered
	match := &ast.MatchExpr{
		Match:     token.Pos(1),
		Scrutinee: makeRawExpr("shape"),
		Arms: []*ast.MatchArm{
			{Pattern: makeConstructorPattern("Circle")},
			{Pattern: makeConstructorPattern("Rectangle")},
			{Pattern: makeConstructorPattern("Point")},
		},
	}

	result := checker.Check(match, true)

	if !result.IsExhaustive {
		t.Errorf("Expected exhaustive match, got non-exhaustive")
	}
	if result.EnumName != "Shape" {
		t.Errorf("Expected enum name 'Shape', got '%s'", result.EnumName)
	}
	if len(result.MissingVariants) != 0 {
		t.Errorf("Expected no missing variants, got %v", result.MissingVariants)
	}
}

func TestExhaustivenessChecker_NonExhaustive(t *testing.T) {
	registry := createTestRegistry("Shape", "Circle", "Rectangle", "Point", "Triangle")

	checker := NewExhaustivenessChecker(registry, nil)

	// Only two variants covered
	match := &ast.MatchExpr{
		Match:     token.Pos(1),
		Scrutinee: makeRawExpr("shape"),
		Arms: []*ast.MatchArm{
			{Pattern: makeConstructorPattern("Circle")},
			{Pattern: makeConstructorPattern("Point")},
		},
	}

	result := checker.Check(match, true)

	if result.IsExhaustive {
		t.Errorf("Expected non-exhaustive match, got exhaustive")
	}
	if result.EnumName != "Shape" {
		t.Errorf("Expected enum name 'Shape', got '%s'", result.EnumName)
	}
	if len(result.MissingVariants) != 2 {
		t.Errorf("Expected 2 missing variants, got %d", len(result.MissingVariants))
	}

	// Check for specific missing variants (should be sorted)
	expectedMissing := []string{"Rectangle", "Triangle"}
	for i, expected := range expectedMissing {
		if i >= len(result.MissingVariants) || result.MissingVariants[i] != expected {
			t.Errorf("Expected missing variant '%s' at position %d, got %v",
				expected, i, result.MissingVariants)
		}
	}
}

func TestExhaustivenessChecker_WildcardPattern(t *testing.T) {
	registry := createTestRegistry("Status", "Pending", "Active", "Done")

	checker := NewExhaustivenessChecker(registry, nil)

	// Wildcard covers all
	match := &ast.MatchExpr{
		Match:     token.Pos(1),
		Scrutinee: makeRawExpr("status"),
		Arms: []*ast.MatchArm{
			{Pattern: makeConstructorPattern("Pending")},
			{Pattern: makeWildcardPattern()},
		},
	}

	result := checker.Check(match, true)

	if !result.IsExhaustive {
		t.Errorf("Expected exhaustive match with wildcard, got non-exhaustive")
	}
	if !result.HasWildcard {
		t.Errorf("Expected HasWildcard to be true")
	}
	if len(result.MissingVariants) != 0 {
		t.Errorf("Expected no missing variants with wildcard, got %v", result.MissingVariants)
	}
}

func TestExhaustivenessChecker_VariablePattern(t *testing.T) {
	registry := createTestRegistry("Result", "Ok", "Err")

	checker := NewExhaustivenessChecker(registry, nil)

	// Variable pattern covers all
	match := &ast.MatchExpr{
		Match:     token.Pos(1),
		Scrutinee: makeRawExpr("result"),
		Arms: []*ast.MatchArm{
			{Pattern: makeConstructorPattern("Ok")},
			{Pattern: makeVariablePattern("other")},
		},
	}

	result := checker.Check(match, true)

	if !result.IsExhaustive {
		t.Errorf("Expected exhaustive match with variable pattern, got non-exhaustive")
	}
	if !result.HasWildcard {
		t.Errorf("Expected HasWildcard to be true for variable pattern")
	}
}

func TestExhaustivenessChecker_GuardedPattern_NotExhaustive(t *testing.T) {
	registry := createTestRegistry("Status", "Active", "Pending")

	checker := NewExhaustivenessChecker(registry, nil)

	// Guard makes Active pattern non-exhaustive
	match := &ast.MatchExpr{
		Match:     token.Pos(1),
		Scrutinee: makeRawExpr("status"),
		Arms: []*ast.MatchArm{
			{
				Pattern: makeConstructorPattern("Active"),
				Guard:   makeRawExpr("count > 0"), // Guard present!
			},
			{Pattern: makeConstructorPattern("Pending")},
		},
	}

	result := checker.Check(match, true)

	if result.IsExhaustive {
		t.Errorf("Expected non-exhaustive match due to guard, got exhaustive")
	}
	if len(result.MissingVariants) != 1 {
		t.Errorf("Expected 1 missing variant (Active), got %d", len(result.MissingVariants))
	}
	if len(result.MissingVariants) > 0 && result.MissingVariants[0] != "Active" {
		t.Errorf("Expected missing variant 'Active', got '%s'", result.MissingVariants[0])
	}
}

func TestExhaustivenessChecker_GuardedPattern_WithFallback(t *testing.T) {
	registry := createTestRegistry("Status", "Active", "Pending")

	checker := NewExhaustivenessChecker(registry, nil)

	// Guarded pattern + unguarded fallback = exhaustive
	match := &ast.MatchExpr{
		Match:     token.Pos(1),
		Scrutinee: makeRawExpr("status"),
		Arms: []*ast.MatchArm{
			{
				Pattern: makeConstructorPattern("Active"),
				Guard:   makeRawExpr("count > 0"), // Guard present
			},
			{
				Pattern: makeConstructorPattern("Active"), // Unguarded fallback
				Guard:   nil,
			},
			{Pattern: makeConstructorPattern("Pending")},
		},
	}

	result := checker.Check(match, true)

	if !result.IsExhaustive {
		t.Errorf("Expected exhaustive match with guarded + fallback, got non-exhaustive with missing: %v",
			result.MissingVariants)
	}
}

func TestExhaustivenessChecker_OrPattern(t *testing.T) {
	registry := createTestRegistry("Color", "Red", "Green", "Blue", "Yellow")

	checker := NewExhaustivenessChecker(registry, nil)

	// Or-pattern combines multiple variants
	match := &ast.MatchExpr{
		Match:     token.Pos(1),
		Scrutinee: makeRawExpr("color"),
		Arms: []*ast.MatchArm{
			{
				Pattern: &ast.OrPattern{
					Left:  makeConstructorPattern("Red"),
					Pipe:  token.Pos(5),
					Right: makeConstructorPattern("Green"),
				},
			},
			{
				Pattern: &ast.OrPattern{
					Left:  makeConstructorPattern("Blue"),
					Pipe:  token.Pos(10),
					Right: makeConstructorPattern("Yellow"),
				},
			},
		},
	}

	result := checker.Check(match, true)

	if !result.IsExhaustive {
		t.Errorf("Expected exhaustive match with or-patterns, got non-exhaustive with missing: %v",
			result.MissingVariants)
	}
}

func TestExhaustivenessChecker_GuardedOrPattern(t *testing.T) {
	registry := createTestRegistry("Result", "Ok", "Err")

	checker := NewExhaustivenessChecker(registry, nil)

	// Guarded or-pattern doesn't cover any variant
	match := &ast.MatchExpr{
		Match:     token.Pos(1),
		Scrutinee: makeRawExpr("result"),
		Arms: []*ast.MatchArm{
			{
				Pattern: &ast.OrPattern{
					Left:  makeConstructorPattern("Ok"),
					Pipe:  token.Pos(5),
					Right: makeConstructorPattern("Err"),
				},
				Guard: makeRawExpr("shouldHandle"), // Guard makes it non-exhaustive
			},
		},
	}

	result := checker.Check(match, true)

	if result.IsExhaustive {
		t.Errorf("Expected non-exhaustive match with guarded or-pattern, got exhaustive")
	}
	if len(result.MissingVariants) != 2 {
		t.Errorf("Expected 2 missing variants (Ok, Err), got %d: %v",
			len(result.MissingVariants), result.MissingVariants)
	}
}

func TestExhaustivenessChecker_UnknownType(t *testing.T) {
	registry := createTestRegistry("Shape", "Circle", "Point")

	checker := NewExhaustivenessChecker(registry, nil)

	// Unknown type - assume exhaustive
	match := &ast.MatchExpr{
		Match:     token.Pos(1),
		Scrutinee: makeRawExpr("unknownVariable"),
		Arms: []*ast.MatchArm{
			{Pattern: makeConstructorPattern("SomePattern")},
		},
	}

	result := checker.Check(match, true)

	if !result.IsExhaustive {
		t.Errorf("Expected exhaustive for unknown type (benefit of doubt), got non-exhaustive")
	}
	if result.EnumName != "" {
		t.Errorf("Expected empty enum name for unknown type, got '%s'", result.EnumName)
	}
}

func TestExhaustivenessChecker_MissingVariants_Sorted(t *testing.T) {
	registry := createTestRegistry("Letter", "D", "A", "C", "B")

	checker := NewExhaustivenessChecker(registry, nil)

	// Cover only D
	match := &ast.MatchExpr{
		Match:     token.Pos(1),
		Scrutinee: makeRawExpr("letter"),
		Arms: []*ast.MatchArm{
			{Pattern: makeConstructorPattern("D")},
		},
	}

	result := checker.Check(match, true)

	// Missing variants should be sorted alphabetically
	expectedMissing := []string{"A", "B", "C"}
	if len(result.MissingVariants) != len(expectedMissing) {
		t.Fatalf("Expected %d missing variants, got %d: %v",
			len(expectedMissing), len(result.MissingVariants), result.MissingVariants)
	}
	for i, expected := range expectedMissing {
		if result.MissingVariants[i] != expected {
			t.Errorf("Expected missing variant '%s' at position %d, got '%s'",
				expected, i, result.MissingVariants[i])
		}
	}
}

func TestExhaustivenessChecker_FullNamePattern(t *testing.T) {
	registry := createTestRegistry("Shape", "Circle", "Point")

	checker := NewExhaustivenessChecker(registry, nil)

	// Use full PascalCase names
	match := &ast.MatchExpr{
		Match:     token.Pos(1),
		Scrutinee: makeRawExpr("shape"),
		Arms: []*ast.MatchArm{
			{Pattern: makeConstructorPattern("ShapeCircle")}, // Full name
			{Pattern: makeConstructorPattern("ShapePoint")},  // Full name
		},
	}

	result := checker.Check(match, true)

	if !result.IsExhaustive {
		t.Errorf("Expected exhaustive match with full names, got non-exhaustive with missing: %v",
			result.MissingVariants)
	}
}

func TestExhaustivenessChecker_NestedOrPattern(t *testing.T) {
	registry := createTestRegistry("Number", "One", "Two", "Three", "Four")

	checker := NewExhaustivenessChecker(registry, nil)

	// Nested or-patterns: (One | Two) | (Three | Four)
	match := &ast.MatchExpr{
		Match:     token.Pos(1),
		Scrutinee: makeRawExpr("num"),
		Arms: []*ast.MatchArm{
			{
				Pattern: &ast.OrPattern{
					Left: &ast.OrPattern{
						Left:  makeConstructorPattern("One"),
						Pipe:  token.Pos(5),
						Right: makeConstructorPattern("Two"),
					},
					Pipe: token.Pos(10),
					Right: &ast.OrPattern{
						Left:  makeConstructorPattern("Three"),
						Pipe:  token.Pos(15),
						Right: makeConstructorPattern("Four"),
					},
				},
			},
		},
	}

	result := checker.Check(match, true)

	if !result.IsExhaustive {
		t.Errorf("Expected exhaustive match with nested or-patterns, got non-exhaustive with missing: %v",
			result.MissingVariants)
	}
}

func TestExhaustivenessChecker_InferScrutineeType_Variant(t *testing.T) {
	registry := createTestRegistry("Result", "Ok", "Err")

	checker := NewExhaustivenessChecker(registry, nil)

	// Scrutinee is a variant name
	match := &ast.MatchExpr{
		Match:     token.Pos(1),
		Scrutinee: makeRawExpr("Ok"),
		Arms: []*ast.MatchArm{
			{Pattern: makeConstructorPattern("Ok")},
			{Pattern: makeConstructorPattern("Err")},
		},
	}

	result := checker.Check(match, true)

	if result.EnumName != "Result" {
		t.Errorf("Expected enum name 'Result' inferred from variant, got '%s'", result.EnumName)
	}
}

func TestExhaustivenessChecker_InferScrutineeType_Constructor(t *testing.T) {
	registry := createTestRegistry("Shape", "Circle", "Point")

	checker := NewExhaustivenessChecker(registry, nil)

	// Scrutinee is a constructor call
	match := &ast.MatchExpr{
		Match:     token.Pos(1),
		Scrutinee: makeRawExpr("NewShapeCircle(5.0)"),
		Arms: []*ast.MatchArm{
			{Pattern: makeConstructorPattern("Circle")},
			{Pattern: makeConstructorPattern("Point")},
		},
	}

	result := checker.Check(match, true)

	if result.EnumName != "Shape" {
		t.Errorf("Expected enum name 'Shape' inferred from constructor, got '%s'", result.EnumName)
	}
}
