package codegen

import (
	"fmt"

	"github.com/MadAppGang/dingo/pkg/ast"
)

// MatchCodeGen generates Go code for match expressions.
// Transforms Dingo match expressions to Go switch statements with type assertions.
//
// Example transformation:
//   Dingo: match result { Ok(value) => value, Err(e) => 0 }
//   Go:    func() int { switch v := result.(type) { case Ok: value := v.Value; return value; case Err: e := v.Value; return 0 } }()
//
// Pattern types supported:
//   - ConstructorPattern: Ok(x), Err(e), Some(v), None
//   - VariablePattern: x (matches anything, binds to variable)
//   - WildcardPattern: _ (matches anything, no binding)
//   - LiteralPattern: 1, "hello", true (matches specific value)
//   - TuplePattern: (a, b) (matches tuple, binds elements)
type MatchCodeGen struct {
	*BaseGenerator
	Match *ast.MatchExpr
}

// Constructor is in expr.go to avoid import cycles

// Generate generates Go code for the match expression.
// Returns CodeGenResult with generated code and source mappings.
func (g *MatchCodeGen) Generate() ast.CodeGenResult {
	if g.Match == nil {
		return ast.CodeGenResult{}
	}

	// If match is used as expression, wrap in IIFE
	if g.Match.IsExpr {
		g.generateMatchIIFE()
	} else {
		g.generateMatchSwitch()
	}

	return g.Result()
}

// generateMatchIIFE generates IIFE wrapper for match expressions.
// Example: func() TYPE { switch ... }()
func (g *MatchCodeGen) generateMatchIIFE() {
	// Track mapping for opening func()
	matchPos := int(g.Match.Pos())
	iifePart := "func() interface{} {\n"
	g.MB.Add(matchPos, matchPos+5, len(iifePart), "match")
	g.Write(iifePart)

	g.generateMatchSwitch()

	// Track mapping for closing
	matchEnd := int(g.Match.End())
	closingPart := "\n}()"
	g.MB.Add(matchEnd-1, matchEnd, len(closingPart), "match")
	g.Write(closingPart)
}

// generateMatchSwitch generates switch statement for match expression.
// Example: switch v := scrutinee.(type) { case Pattern: ... }
func (g *MatchCodeGen) generateMatchSwitch() {
	// Generate scrutinee
	scrutineeResult := GenerateExpr(g.Match.Scrutinee)
	scrutineeCode := string(scrutineeResult.Output)

	// Track mapping for scrutinee
	scrutineeStart := int(g.Match.Scrutinee.Pos())
	scrutineeEnd := int(g.Match.Scrutinee.End())

	// Check if we need type assertion (for constructor patterns)
	needsTypeSwitch := g.hasConstructorPatterns()

	if needsTypeSwitch {
		// Generate type switch: switch v := scrutinee.(type)
		tempVar := g.TempVar("v")
		switchPrefix := "switch " + tempVar + " := "
		g.MB.Add(scrutineeStart, scrutineeEnd, len(switchPrefix)+len(scrutineeCode)+len(".(type) {\n"), "match")
		g.Write(switchPrefix)
		g.Write(scrutineeCode)
		g.Write(".(type) {\n")
	} else {
		// Generate value switch: switch scrutinee {
		switchPrefix := "switch "
		g.MB.Add(scrutineeStart, scrutineeEnd, len(switchPrefix)+len(scrutineeCode)+len(" {\n"), "match")
		g.Write(switchPrefix)
		g.Write(scrutineeCode)
		g.Write(" {\n")
	}

	// Generate cases for each arm
	for _, arm := range g.Match.Arms {
		g.generateMatchArm(arm, needsTypeSwitch)
	}

	g.WriteByte('}')
}

// hasConstructorPatterns checks if any arm has constructor patterns.
// This determines whether we need a type switch or value switch.
func (g *MatchCodeGen) hasConstructorPatterns() bool {
	for _, arm := range g.Match.Arms {
		if _, ok := arm.Pattern.(*ast.ConstructorPattern); ok {
			return true
		}
	}
	return false
}

// generateMatchArm generates a case clause for a match arm.
// Handles pattern matching, guards, and body generation.
func (g *MatchCodeGen) generateMatchArm(arm *ast.MatchArm, typeSwitch bool) {
	// Track mapping for pattern
	patternStart := int(arm.PatternPos)
	patternEnd := int(arm.Pattern.End())

	// Generate case clause based on pattern type
	switch pattern := arm.Pattern.(type) {
	case *ast.ConstructorPattern:
		g.generateConstructorCase(pattern, arm, typeSwitch, patternStart, patternEnd)
	case *ast.LiteralPattern:
		g.generateLiteralCase(pattern, arm, patternStart, patternEnd)
	case *ast.WildcardPattern:
		g.generateWildcardCase(arm, patternStart, patternEnd)
	case *ast.VariablePattern:
		g.generateVariableCase(pattern, arm, patternStart, patternEnd)
	case *ast.TuplePattern:
		g.generateTupleCase(pattern, arm, patternStart, patternEnd)
	default:
		// Unknown pattern type - generate default case
		defaultCase := "default:\n"
		g.MB.Add(patternStart, patternEnd, len(defaultCase), "match")
		g.Write(defaultCase)
		g.generateArmBody(arm, nil)
	}
}

// generateConstructorCase generates case for constructor patterns (Ok(x), Err(e), Some(v), None).
func (g *MatchCodeGen) generateConstructorCase(pattern *ast.ConstructorPattern, arm *ast.MatchArm, typeSwitch bool, patternStart, patternEnd int) {
	caseClause := "case " + pattern.Name + ":\n"
	g.MB.Add(patternStart, patternEnd, len(caseClause), "match")
	g.Write(caseClause)

	// Extract bindings from constructor parameters
	var bindings []Binding
	for i, param := range pattern.Params {
		bindings = append(bindings, g.extractBindings(param, i)...)
	}

	g.generateArmBody(arm, bindings)
}

// Binding represents a variable binding extracted from a pattern.
type Binding struct {
	Name      string
	FieldPath string // e.g., "Value", "First", "Second"
}

// extractBindings recursively extracts variable bindings from patterns.
func (g *MatchCodeGen) extractBindings(pattern ast.Pattern, index int) []Binding {
	switch p := pattern.(type) {
	case *ast.VariablePattern:
		// Single variable binding: x
		// For constructor params, bind to .Value field
		return []Binding{{
			Name:      p.Name,
			FieldPath: "Value",
		}}
	case *ast.TuplePattern:
		// Tuple pattern: (a, b)
		var bindings []Binding
		for i, elem := range p.Elements {
			elemBindings := g.extractBindings(elem, i)
			for j := range elemBindings {
				// Adjust field path for tuple element
				elemBindings[j].FieldPath = fmt.Sprintf("Field%d", i)
			}
			bindings = append(bindings, elemBindings...)
		}
		return bindings
	case *ast.ConstructorPattern:
		// Nested constructor: Ok(Some(x))
		var bindings []Binding
		for i, param := range p.Params {
			bindings = append(bindings, g.extractBindings(param, i)...)
		}
		return bindings
	default:
		// Wildcard or literal - no bindings
		return nil
	}
}

// generateLiteralCase generates case for literal patterns (1, "hello", true).
func (g *MatchCodeGen) generateLiteralCase(pattern *ast.LiteralPattern, arm *ast.MatchArm, patternStart, patternEnd int) {
	caseClause := "case " + pattern.Value + ":\n"
	g.MB.Add(patternStart, patternEnd, len(caseClause), "match")
	g.Write(caseClause)
	g.generateArmBody(arm, nil)
}

// generateWildcardCase generates default case for wildcard pattern (_).
func (g *MatchCodeGen) generateWildcardCase(arm *ast.MatchArm, patternStart, patternEnd int) {
	defaultCase := "default:\n"
	g.MB.Add(patternStart, patternEnd, len(defaultCase), "match")
	g.Write(defaultCase)
	g.generateArmBody(arm, nil)
}

// generateVariableCase generates default case with binding for variable pattern (x).
func (g *MatchCodeGen) generateVariableCase(pattern *ast.VariablePattern, arm *ast.MatchArm, patternStart, patternEnd int) {
	defaultCase := "default:\n"
	g.MB.Add(patternStart, patternEnd, len(defaultCase), "match")
	g.Write(defaultCase)

	// Bind variable to scrutinee value
	bindings := []Binding{{
		Name:      pattern.Name,
		FieldPath: "", // Bind to scrutinee itself
	}}
	g.generateArmBody(arm, bindings)
}

// generateTupleCase generates case for tuple patterns ((a, b)).
func (g *MatchCodeGen) generateTupleCase(pattern *ast.TuplePattern, arm *ast.MatchArm, patternStart, patternEnd int) {
	// Tuple patterns need special handling - not implemented yet
	// For now, generate default case
	defaultCase := "default:\n"
	g.MB.Add(patternStart, patternEnd, len(defaultCase), "match")
	g.Write(defaultCase)
	g.generateArmBody(arm, nil)
}

// generateArmBody generates the body of a match arm.
// Handles variable bindings, guards, and body expression.
func (g *MatchCodeGen) generateArmBody(arm *ast.MatchArm, bindings []Binding) {
	// Generate variable bindings
	for _, binding := range bindings {
		bindingCode := binding.Name + " := "

		// If binding has a field path, use type assertion variable
		if binding.FieldPath != "" {
			// Use "v" as the temp var (matches TempVar("v") from generateMatchSwitch)
			bindingCode += "v." + binding.FieldPath
		} else {
			// Bind to scrutinee itself (for variable patterns in default case)
			scrutineeResult := GenerateExpr(g.Match.Scrutinee)
			bindingCode += string(scrutineeResult.Output)
		}

		bindingCode += "\n"
		g.Write(bindingCode)
	}

	// Generate guard check if present
	if arm.Guard != nil {
		guardResult := GenerateExpr(arm.Guard)
		guardStart := int(arm.GuardPos)
		guardEnd := int(arm.Guard.End())
		guardCode := "if " + string(guardResult.Output) + " {\n"
		g.MB.Add(guardStart, guardEnd, len(guardCode), "match")
		g.Write(guardCode)
	}

	// Generate body
	bodyStart := int(arm.Body.Pos())
	bodyEnd := int(arm.Body.End())

	// If match is expression, use return statement
	if g.Match.IsExpr {
		bodyResult := GenerateExpr(arm.Body)
		bodyCode := "return " + string(bodyResult.Output) + "\n"
		g.MB.Add(bodyStart, bodyEnd, len(bodyCode), "match")
		g.Write(bodyCode)
	} else {
		bodyResult := GenerateExpr(arm.Body)
		g.MB.Add(bodyStart, bodyEnd, len(bodyResult.Output), "match")
		g.Buf.Write(bodyResult.Output)
		g.WriteByte('\n')
	}

	// Close guard if present
	if arm.Guard != nil {
		g.Write("}\n")
	}
}
