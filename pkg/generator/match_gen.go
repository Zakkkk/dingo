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
	enumTypeName string // The enum type being matched (e.g., "Event")
	isReturn     bool   // True if match is in "return match" context (no IIFE needed)
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

// SetReturnContext marks this match as being in a "return match" context
// When true, no IIFE wrapping is needed - just direct switch with returns
func (g *MatchGenerator) SetReturnContext(isReturn bool) {
	g.isReturn = isReturn
}

// SetInitialIndent sets the starting indentation level
// This should match the indentation at the insertion point in the source
func (g *MatchGenerator) SetInitialIndent(level int) {
	g.indentLevel = level
}

// Generate transforms a MatchExpr into Go switch statement code
func (g *MatchGenerator) Generate(match *dingoast.MatchExpr) (string, []Mapping) {
	var b strings.Builder
	var mappings []Mapping

	g.markerID++
	marker := fmt.Sprintf("// dingo:M:%d", g.markerID)

	// Extract scrutinee expression
	scrutineeExpr := match.Scrutinee.String()

	// Try to infer enum type name from scrutinee (e.g., "event" -> "Event")
	// This is used for generating correct tag constants
	g.enumTypeName = g.inferEnumTypeName(scrutineeExpr)

	// Determine if we need IIFE wrapping
	// For "return match" context, no IIFE needed - just use switch with returns
	// For assignment "x := match", we need IIFE
	needsIIFE := match.IsExpr && !g.isReturn

	if needsIIFE {
		// func() ReturnType {
		// Note: No indent on first line - IIFE starts inline after ":=" or "("
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

	// Generate: switch tempVar.tag {
	b.WriteString(g.indent())
	b.WriteString("switch ")
	b.WriteString(tempVar)
	b.WriteString(".tag {\n")

	g.indentLevel++

	// Group arms by pattern to avoid duplicate case labels
	// Arms with same constructor pattern are grouped into one case with if-else chain
	groupedArms := g.groupArmsByPattern(match.Arms)
	for _, group := range groupedArms {
		caseCode, armMappings := g.generateArmGroup(group, tempVar, marker)
		b.WriteString(caseCode)
		mappings = append(mappings, armMappings...)
	}

	g.indentLevel--

	// Closing brace
	b.WriteString(g.indent())
	b.WriteString("}\n")

	// Check if match has a default/wildcard arm (exhaustive)
	hasDefault := g.hasDefaultArm(match.Arms)

	// Check if any arm has guards (which may cause non-returning paths)
	hasGuards := g.hasGuardedArms(match.Arms)

	// If IIFE, close the function and add panic only if needed
	if needsIIFE {
		// Only add panic if not exhaustive (no default arm, or has guards that might not return)
		if !hasDefault || hasGuards {
			b.WriteString(g.indent())
			b.WriteString("panic(\"non-exhaustive match\")\n")
		}

		g.indentLevel--
		b.WriteString(g.indent())
		b.WriteString("}()")
	} else if g.isReturn && (!hasDefault || hasGuards) {
		// For "return match" context:
		// - Without default arm: add panic for safety
		// - With guards: guard failures may not have a return path
		// This ensures the function has a valid return path
		b.WriteString(g.indent())
		b.WriteString("panic(\"non-exhaustive match\")\n")
	}

	return b.String(), mappings
}

// hasDefaultArm checks if the match has a wildcard/default arm
func (g *MatchGenerator) hasDefaultArm(arms []*dingoast.MatchArm) bool {
	for _, arm := range arms {
		if _, ok := arm.Pattern.(*dingoast.WildcardPattern); ok {
			return true
		}
	}
	return false
}

// hasGuardedArms checks if any arm has a guard condition
// Guards can cause non-returning paths if the guard fails
func (g *MatchGenerator) hasGuardedArms(arms []*dingoast.MatchArm) bool {
	for _, arm := range arms {
		if arm.Guard != nil {
			return true
		}
	}
	return false
}

// armGroup represents a group of arms with the same pattern
type armGroup struct {
	patternKey string          // Unique key for grouping (e.g., "Constructor:OrderPlaced")
	arms       []*dingoast.MatchArm
}

// groupArmsByPattern groups arms by their pattern type and name
// This allows generating single case with if-else chain for guarded arms
func (g *MatchGenerator) groupArmsByPattern(arms []*dingoast.MatchArm) []armGroup {
	var groups []armGroup
	keyToIndex := make(map[string]int)

	for _, arm := range arms {
		key := g.patternKey(arm.Pattern)
		if idx, exists := keyToIndex[key]; exists {
			groups[idx].arms = append(groups[idx].arms, arm)
		} else {
			keyToIndex[key] = len(groups)
			groups = append(groups, armGroup{
				patternKey: key,
				arms:       []*dingoast.MatchArm{arm},
			})
		}
	}

	return groups
}

// patternKey returns a unique key for pattern grouping
// This includes nested patterns to distinguish Ok(Some(x)) from Ok(None)
func (g *MatchGenerator) patternKey(p dingoast.Pattern) string {
	switch pat := p.(type) {
	case *dingoast.ConstructorPattern:
		key := "Constructor:" + pat.Name
		// Include nested patterns in key to distinguish Ok(Some(x)) from Ok(None)
		if len(pat.Params) > 0 {
			for _, nested := range pat.Params {
				// Only include constructor patterns in key (variable/wildcard don't change case label)
				if cp, ok := nested.(*dingoast.ConstructorPattern); ok {
					key += "." + g.patternKey(cp)
				}
			}
		}
		return key
	case *dingoast.WildcardPattern:
		return "Wildcard"
	case *dingoast.LiteralPattern:
		return "Literal:" + pat.Value
	case *dingoast.VariablePattern:
		return "Variable:" + pat.Name
	case *dingoast.TuplePattern:
		return "Tuple"
	default:
		return "Unknown"
	}
}

// generateArmGroup generates code for a group of arms with the same pattern
func (g *MatchGenerator) generateArmGroup(group armGroup, scrutineeVar, marker string) (string, []Mapping) {
	if len(group.arms) == 1 {
		// Single arm - use simple generation
		return g.generateArm(group.arms[0], scrutineeVar, marker)
	}

	// Multiple arms with same pattern - need if-else chain
	return g.generateMultiArmCase(group.arms, scrutineeVar, marker)
}

// generateMultiArmCase generates code for multiple arms with the same pattern
// Used when same constructor appears multiple times (with different guards)
// Generates: case Tag: if guard1 { body1 } else if guard2 { body2 } else { body3 }
func (g *MatchGenerator) generateMultiArmCase(arms []*dingoast.MatchArm, scrutineeVar, marker string) (string, []Mapping) {
	var b strings.Builder
	var mappings []Mapping

	if len(arms) == 0 {
		return "", nil
	}

	// All arms have the same pattern, use the first one for case generation
	firstArm := arms[0]

	// Get the constructor pattern
	p, ok := firstArm.Pattern.(*dingoast.ConstructorPattern)
	if !ok {
		// Fallback: generate each arm separately (shouldn't happen for grouped arms)
		for _, arm := range arms {
			code, armMappings := g.generateArm(arm, scrutineeVar, marker)
			b.WriteString(code)
			mappings = append(mappings, armMappings...)
		}
		return b.String(), mappings
	}

	// Generate case header
	tagName := g.constructorToTag(p.Name)
	b.WriteString(g.indent())
	b.WriteString("case ")
	b.WriteString(tagName)
	b.WriteString(":\n")

	g.indentLevel++

	// Extract bindings (same for all arms since pattern is the same)
	if len(p.Params) > 0 {
		bindingsCode := g.generateBindings(p, scrutineeVar, 0)
		b.WriteString(bindingsCode)
	}

	// Generate if-else chain for guards
	// Find arms with guards and the final fallback (no guard)
	var guardedArms []*dingoast.MatchArm
	var fallbackArm *dingoast.MatchArm

	for _, arm := range arms {
		if arm.Guard != nil {
			guardedArms = append(guardedArms, arm)
		} else {
			fallbackArm = arm
		}
	}

	// Generate if-else chain
	for i, arm := range guardedArms {
		b.WriteString(g.indent())
		if i == 0 {
			b.WriteString("if ")
		} else {
			b.WriteString("} else if ")
		}
		b.WriteString(arm.Guard.String())
		b.WriteString(" {\n")

		g.indentLevel++
		bodyCode := g.generateBody(arm, marker)
		b.WriteString(bodyCode)
		g.indentLevel--
	}

	// Generate fallback (else clause or just the body if no guards)
	if len(guardedArms) > 0 {
		if fallbackArm != nil {
			b.WriteString(g.indent())
			b.WriteString("} else {\n")
			g.indentLevel++
			bodyCode := g.generateBody(fallbackArm, marker)
			b.WriteString(bodyCode)
			g.indentLevel--
		}
		b.WriteString(g.indent())
		b.WriteString("}\n")
	} else if fallbackArm != nil {
		// No guards at all - just generate the body
		bodyCode := g.generateBody(fallbackArm, marker)
		b.WriteString(bodyCode)
	}

	g.indentLevel--

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

	// First, collect all param names for field name generation
	paramNames := g.extractParamNames(p)

	for i, param := range p.Params {
		fieldName := g.constructorToField(p.Name, i, paramNames)

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
			b.WriteString(".tag {\n")

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

// extractParamNames extracts variable names from pattern params
// Used for generating correct field names for enum variants
func (g *MatchGenerator) extractParamNames(p *dingoast.ConstructorPattern) []string {
	names := make([]string, len(p.Params))
	for i, param := range p.Params {
		switch pp := param.(type) {
		case *dingoast.VariablePattern:
			names[i] = pp.Name
		case *dingoast.WildcardPattern:
			names[i] = "_"
		default:
			names[i] = fmt.Sprintf("param%d", i)
		}
	}
	return names
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
// For enums: UserCreated -> EventTagUserCreated (when enumName is set)
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
		// For plain variant names, we need the enum type name
		// This will be set from context (scrutinee type)
		// For now, use the pattern: TypeTagVariant where Type is from scrutinee
		if g.enumTypeName != "" {
			return g.enumTypeName + "Tag" + name
		}
		// Fallback: just append Tag
		return name + "Tag"
	}
}

// constructorToField converts constructor name and param index to field name
// For Result: Ok -> Ok, Err -> Err (exported fields for pattern matching)
// For Option: Some -> Some, None -> (no field)
// For Enums: UserCreated with params [userID, email] -> usercreated_userID, usercreated_email
func (g *MatchGenerator) constructorToField(constructorName string, paramIndex int, paramNames []string) string {
	switch constructorName {
	case "Ok":
		return "Ok"
	case "Err":
		return "Err"
	case "Some":
		return "Some"
	default:
		// Enum variants: field name is lowercase(variant)_paramName
		// e.g., UserCreated.userID -> usercreated_userID
		if len(paramNames) > paramIndex {
			return strings.ToLower(constructorName) + "_" + paramNames[paramIndex]
		}
		// Fallback: just lowercase constructor name
		return strings.ToLower(constructorName)
	}
}

// inferEnumTypeName tries to infer the enum type from the scrutinee expression
// This is a heuristic based on variable naming conventions
func (g *MatchGenerator) inferEnumTypeName(scrutinee string) string {
	// Common pattern: variable name matches type name in lowercase
	// e.g., "event" -> "Event", "status" -> "Status"
	scrutinee = strings.TrimSpace(scrutinee)

	// Handle method calls like x.GetEvent() - extract just the variable/expression
	if idx := strings.LastIndex(scrutinee, "."); idx >= 0 {
		// Could be a method call, check for (
		if strings.Contains(scrutinee[idx:], "(") {
			// Method call - can't easily infer type
			return ""
		}
	}

	// Simple variable name - capitalize first letter
	if len(scrutinee) > 0 && scrutinee[0] >= 'a' && scrutinee[0] <= 'z' {
		return strings.ToUpper(scrutinee[:1]) + scrutinee[1:]
	}

	return ""
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

	// Check for boolean literals first (most specific)
	if bodyStr == "true" || bodyStr == "false" {
		return "bool"
	}

	// Check for string patterns
	if strings.Contains(bodyStr, "fmt.Sprintf") || strings.Contains(bodyStr, `"`) {
		return "string"
	}

	// Check for numeric literals
	if len(bodyStr) > 0 && (bodyStr[0] >= '0' && bodyStr[0] <= '9') {
		return "int"
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
