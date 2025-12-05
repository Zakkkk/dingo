package generator

import (
	"fmt"
	"strings"

	dingoast "github.com/MadAppGang/dingo/pkg/ast"
)

// MatchGenerator generates Go switch statements from MatchExpr AST nodes
type MatchGenerator struct {
	matchID      int
	tempCounter  int
	indentLevel  int
	markerID     int
}

// NewMatchGenerator creates a new match generator
func NewMatchGenerator(matchID int) *MatchGenerator {
	return &MatchGenerator{
		matchID:      matchID,
		tempCounter:  0,
		indentLevel:  0,
		markerID:     0,
	}
}

// Generate transforms a MatchExpr into Go switch statement code
func (g *MatchGenerator) Generate(match *dingoast.MatchExpr) (string, []Mapping) {
	var b strings.Builder
	var mappings []Mapping

	g.markerID++
	marker := fmt.Sprintf("// dingo:M:%d", g.markerID)

	// Extract scrutinee expression
	scrutineeExpr := match.Scrutinee.String()

	// If used as expression, wrap in IIFE
	if match.IsExpr {
		// func() ReturnType {
		b.WriteString(g.indent())
		b.WriteString("func() ")
		// Infer return type from match expression context
		returnType := g.inferReturnType(match)
		b.WriteString(returnType)
		b.WriteString(" {\n")
		g.indentLevel++
	}

	// Generate: scrutinee := expr
	tempVar := g.nextTemp()
	b.WriteString(g.indent())
	b.WriteString(tempVar)
	b.WriteString(" := ")
	b.WriteString(scrutineeExpr)
	b.WriteString("\n")

	// Generate: switch tempVar.Tag {
	b.WriteString(g.indent())
	b.WriteString("switch ")
	b.WriteString(tempVar)
	b.WriteString(".Tag {\n")

	g.indentLevel++

	// Group arms by constructor for case generation
	for _, arm := range match.Arms {
		caseCode, armMappings := g.generateArm(arm, tempVar, marker)
		b.WriteString(caseCode)
		mappings = append(mappings, armMappings...)
	}

	g.indentLevel--

	// Closing brace
	b.WriteString(g.indent())
	b.WriteString("}\n")

	// If IIFE, add panic for non-exhaustive matches and close
	if match.IsExpr {
		// Add panic statement (should never be reached in exhaustive match)
		b.WriteString(g.indent())
		b.WriteString("panic(\"non-exhaustive match\")\n")

		g.indentLevel--
		b.WriteString(g.indent())
		b.WriteString("}()")
	}

	return b.String(), mappings
}

// generateArm generates code for a single match arm
func (g *MatchGenerator) generateArm(arm *dingoast.MatchArm, scrutineeVar, marker string) (string, []Mapping) {
	var b strings.Builder
	var mappings []Mapping

	// Handle different pattern types
	switch p := arm.Pattern.(type) {
	case *dingoast.WildcardPattern:
		// default case
		b.WriteString(g.indent())
		b.WriteString("default:\n")
		g.indentLevel++
		bodyCode := g.generateBody(arm, marker)
		b.WriteString(bodyCode)
		g.indentLevel--

	case *dingoast.ConstructorPattern:
		caseCode, caseMappings := g.generateConstructorCase(p, arm, scrutineeVar, marker)
		b.WriteString(caseCode)
		mappings = append(mappings, caseMappings...)

	case *dingoast.LiteralPattern:
		// case literal:
		b.WriteString(g.indent())
		b.WriteString("case ")
		b.WriteString(p.Value)
		b.WriteString(":\n")
		g.indentLevel++
		bodyCode := g.generateBody(arm, marker)
		b.WriteString(bodyCode)
		g.indentLevel--

	case *dingoast.VariablePattern:
		// Variable binding - becomes default case with assignment
		b.WriteString(g.indent())
		b.WriteString("default:\n")
		g.indentLevel++
		b.WriteString(g.indent())
		b.WriteString(p.Name)
		b.WriteString(" := ")
		b.WriteString(scrutineeVar)
		b.WriteString("\n")
		bodyCode := g.generateBody(arm, marker)
		b.WriteString(bodyCode)
		g.indentLevel--

	case *dingoast.TuplePattern:
		// Tuple patterns not yet implemented in this phase
		b.WriteString(g.indent())
		b.WriteString("// TODO: Tuple pattern not yet implemented\n")
		b.WriteString(g.indent())
		b.WriteString("default:\n")
		g.indentLevel++
		bodyCode := g.generateBody(arm, marker)
		b.WriteString(bodyCode)
		g.indentLevel--
	}

	return b.String(), mappings
}

// generateConstructorCase generates code for constructor patterns (Ok, Err, Some, None, etc.)
func (g *MatchGenerator) generateConstructorCase(p *dingoast.ConstructorPattern, arm *dingoast.MatchArm, scrutineeVar, marker string) (string, []Mapping) {
	var b strings.Builder
	var mappings []Mapping

	// Determine tag name from constructor
	tagName := g.constructorToTag(p.Name)

	// case TagName:
	b.WriteString(g.indent())
	b.WriteString("case ")
	b.WriteString(tagName)
	b.WriteString(":\n")

	g.indentLevel++

	// Extract bindings from nested patterns
	if len(p.Params) > 0 {
		bindingsCode := g.generateBindings(p, scrutineeVar, 0)
		b.WriteString(bindingsCode)
	}

	// Generate guard if present
	if arm.Guard != nil {
		// Wrap body in guard condition (non-negated)
		b.WriteString(g.indent())
		b.WriteString("if ")
		b.WriteString(arm.Guard.String())
		b.WriteString(" {\n")
		g.indentLevel++

		// Generate body inside guard
		bodyCode := g.generateBody(arm, marker)
		b.WriteString(bodyCode)

		g.indentLevel--
		b.WriteString(g.indent())
		b.WriteString("}\n")
	} else {
		// Non-guard body generation
		bodyCode := g.generateBody(arm, marker)
		b.WriteString(bodyCode)
	}

	g.indentLevel--

	return b.String(), mappings
}

// generateBindings generates binding extraction code for nested patterns
// This recursively handles patterns like Ok(Some(x)) by generating nested switches
func (g *MatchGenerator) generateBindings(p *dingoast.ConstructorPattern, parentVar string, depth int) string {
	var b strings.Builder

	for i, param := range p.Params {
		fieldName := g.constructorToField(p.Name, i)

		switch paramPattern := param.(type) {
		case *dingoast.VariablePattern:
			// Simple variable binding: x := *parentVar.Field (dereference pointer)
			b.WriteString(g.indent())
			b.WriteString(paramPattern.Name)
			b.WriteString(" := *")
			b.WriteString(parentVar)
			b.WriteString(".")
			b.WriteString(fieldName)
			b.WriteString("\n")

		case *dingoast.ConstructorPattern:
			// Nested constructor: need nested switch
			tempVar := g.nextTemp()

			// Extract the field to temp variable (dereference pointer)
			b.WriteString(g.indent())
			b.WriteString(tempVar)
			b.WriteString(" := *")
			b.WriteString(parentVar)
			b.WriteString(".")
			b.WriteString(fieldName)
			b.WriteString("\n")

			// Generate nested switch
			nestedTagName := g.constructorToTag(paramPattern.Name)
			b.WriteString(g.indent())
			b.WriteString("switch ")
			b.WriteString(tempVar)
			b.WriteString(".Tag {\n")

			g.indentLevel++
			b.WriteString(g.indent())
			b.WriteString("case ")
			b.WriteString(nestedTagName)
			b.WriteString(":\n")

			g.indentLevel++

			// Recursively extract nested bindings
			if len(paramPattern.Params) > 0 {
				nestedBindings := g.generateBindings(paramPattern, tempVar, depth+1)
				b.WriteString(nestedBindings)
			}

			g.indentLevel--

			// Add default to break out if no match
			b.WriteString(g.indent())
			b.WriteString("default:\n")
			g.indentLevel++
			b.WriteString(g.indent())
			b.WriteString("break // Inner pattern didn't match\n")
			g.indentLevel--

			g.indentLevel--

			b.WriteString(g.indent())
			b.WriteString("}\n")

		case *dingoast.WildcardPattern:
			// Wildcard - no binding needed
			continue

		case *dingoast.LiteralPattern:
			// Literal patterns in constructor params - comparison check
			// For now, skip (would need equality check in guard)
			continue
		}
	}

	return b.String()
}

// generateBody generates the body code for a match arm
func (g *MatchGenerator) generateBody(arm *dingoast.MatchArm, marker string) string {
	var b strings.Builder

	bodyExpr := arm.Body.String()

	if arm.IsBlock {
		// Block body: { ... }
		// Just insert the block content
		b.WriteString(g.indent())
		b.WriteString(bodyExpr)
		b.WriteString(" ")
		b.WriteString(marker)
		b.WriteString("\n")
	} else {
		// Expression body: always use return (for IIFE compatibility)
		b.WriteString(g.indent())
		b.WriteString("return ")
		b.WriteString(bodyExpr)
		b.WriteString(" ")
		b.WriteString(marker)
		b.WriteString("\n")
	}

	// Add trailing comment if present
	if arm.Comment != nil {
		// Comment was already included in body expression
	}

	return b.String()
}

// constructorToTag converts constructor name to tag constant
// Ok -> dgo.ResultTagOk, Err -> dgo.ResultTagErr, Some -> dgo.OptionTagSome, None -> dgo.OptionTagNone
func (g *MatchGenerator) constructorToTag(name string) string {
	switch name {
	case "Ok":
		return "dgo.ResultTagOk"
	case "Err":
		return "dgo.ResultTagErr"
	case "Some":
		return "dgo.OptionTagSome"
	case "None":
		return "dgo.OptionTagNone"
	default:
		// Enum variants: EnumName_Variant -> EnumNameTagVariant
		if strings.Contains(name, "_") {
			parts := strings.Split(name, "_")
			if len(parts) == 2 {
				return parts[0] + "Tag" + parts[1]
			}
		}
		// Fallback: just append Tag
		return name + "Tag"
	}
}

// constructorToField converts constructor name to field name
// For Result: Ok -> Ok, Err -> Err (exported fields for pattern matching)
// For Option: Some -> Some, None -> (no field)
func (g *MatchGenerator) constructorToField(constructorName string, paramIndex int) string {
	switch constructorName {
	case "Ok":
		return "Ok"
	case "Err":
		return "Err"
	case "Some":
		return "Some"
	default:
		// Enum variants or custom types
		// For now, use lowercase constructor name
		return strings.ToLower(constructorName)
	}
}

// nextTemp generates the next temporary variable name
// Follows Dingo naming convention: tmp, tmp1, tmp2, ... (no underscore prefix)
func (g *MatchGenerator) nextTemp() string {
	if g.tempCounter == 0 {
		g.tempCounter++
		return "tmp"
	}
	name := fmt.Sprintf("tmp%d", g.tempCounter)
	g.tempCounter++
	return name
}

// indent returns the current indentation string
func (g *MatchGenerator) indent() string {
	return strings.Repeat("\t", g.indentLevel)
}

// inferReturnType attempts to infer the return type from match expression context
// For now, returns the type from the first arm's body
// TODO: Full type inference from enclosing function signature
func (g *MatchGenerator) inferReturnType(match *dingoast.MatchExpr) string {
	// Strategy: Look at the first arm's body to infer type
	// This is a heuristic - ideally would use proper type inference
	if len(match.Arms) == 0 {
		return "interface{}" // No arms, can't infer
	}

	// Check if we can infer from first arm
	firstArm := match.Arms[0]
	bodyStr := firstArm.Body.String()

	// Simple heuristics based on body content
	// Look for common patterns in the first arm
	if strings.Contains(bodyStr, "fmt.Sprintf") || strings.Contains(bodyStr, `"`) {
		return "string"
	}
	if strings.Contains(bodyStr, "value") || strings.Contains(bodyStr, "x") {
		// Likely returning an int or other numeric value from Result/Option
		// Default to int for now
		return "int"
	}

	// Fallback: use interface{} for now
	// TODO: Implement proper type inference from function return type
	return "interface{}"
}

// Mapping represents a source map entry (placeholder for now)
type Mapping struct {
	OriginalLine   int
	OriginalColumn int
	GeneratedLine  int
	GeneratedColumn int
}
