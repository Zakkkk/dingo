package codegen

import (
	"bytes"
	"fmt"

	"github.com/MadAppGang/dingo/pkg/ast"
	"github.com/MadAppGang/dingo/pkg/typechecker"
)

// MatchCodeGen generates Go code for match expressions.
// Transforms Dingo match expressions to Go switch statements with type assertions.
//
// Example transformation:
//   Dingo: match result { Ok(value) => value, Err(e) => 0 }
//   Go:    func() int { switch v := result.(type) { case Ok: value := v.Value; return value; case Err: e := v.Value; return 0 } }()
//
// With return context (human-like output - no IIFE):
//   Dingo: return match s { Status_Pending => "waiting", Status_Active => "active" }
//   Go:    switch s.(type) {
//              case StatusPending: return "waiting"
//              case StatusActive: return "active"
//          }
//          panic("unreachable: exhaustive match")
//
// Pattern types supported:
//   - ConstructorPattern: Ok(x), Err(e), Some(v), None
//   - VariablePattern: x (matches anything, binds to variable)
//   - WildcardPattern: _ (matches anything, no binding)
//   - LiteralPattern: 1, "hello", true (matches specific value)
//   - TuplePattern: (a, b) (matches tuple, binds elements)
type MatchCodeGen struct {
	*BaseGenerator
	Match            *ast.MatchExpr
	Context          *GenContext // Optional context for human-like code generation
	scrutineeTempVar string      // Temp var name for scrutinee in type switch (e.g., "v")
}

// SharedTempVar generates unique temp var names using shared counter if available.
// Counter starts at len(locations) and decrements, so first expression in file gets no suffix.
// Falls back to local counter if no shared counter is provided.
func (g *MatchCodeGen) SharedTempVar(base string) string {
	if g.Context != nil && g.Context.TempCounter != nil {
		*g.Context.TempCounter--
		counter := *g.Context.TempCounter
		if counter == 0 {
			return base
		}
		return fmt.Sprintf("%s%d", base, counter)
	}
	return g.TempVar(base)
}

// Constructor is in expr.go to avoid import cycles

// Generate generates Go code for the match expression.
// Returns CodeGenResult with generated code and source mappings.
func (g *MatchCodeGen) Generate() ast.CodeGenResult {
	if g.Match == nil {
		return ast.CodeGenResult{}
	}

	// Check exhaustiveness for expression matches
	if g.Match.IsExpr {
		if errResult := g.checkExhaustiveness(); len(errResult.Output) > 0 {
			// Error result has non-empty Output containing error code
			return errResult
		}
	}

	// Check for assignment context - can generate temp var pattern
	if g.Match.IsExpr && g.Context != nil && g.Context.Context == ast.ContextAssignment {
		return g.generateHumanLikeAssignment()
	}

	// Check for argument context - can hoist switch before function call
	if g.Match.IsExpr && g.Context != nil && g.Context.Context == ast.ContextArgument {
		return g.generateHoistedMatch()
	}

	// Check for return context - can unwrap IIFE for human-like output
	if g.Match.IsExpr && g.Context != nil && g.Context.Context == ast.ContextReturn {
		// Return context: unwrap IIFE, generate switch with direct returns
		// This produces more idiomatic Go code
		g.generateMatchSwitch()

		// Move output to StatementOutput for statement-level replacement
		result := g.Result()
		result.StatementOutput = result.Output
		result.Output = nil
		return result
	} else if g.Match.IsExpr {
		// Other expression contexts (no context or unknown): wrap in IIFE
		g.generateMatchIIFE()
	} else {
		// Statement context: no IIFE needed
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

	// CRITICAL FIX: Always bind match value to temp var to prevent double evaluation
	// This prevents side-effect functions from executing multiple times
	scrutineeTempVar := g.SharedTempVar("val")
	g.Write(fmt.Sprintf("%s := %s\n", scrutineeTempVar, scrutineeCode))

	// Check if we need type assertion (for constructor patterns)
	needsTypeSwitch := g.hasConstructorPatterns()

	// Check if we have any bindings (to decide on temp var usage)
	hasBindings := g.hasBindings()

	if needsTypeSwitch {
		// Generate type switch: switch v := scrutinee.(type) or switch scrutinee.(type)
		if hasBindings {
			// Need temp var for bindings
			tempVar := g.SharedTempVar("v")
			g.scrutineeTempVar = tempVar // Store for generateArmBody
			switchPrefix := "switch " + tempVar + " := "
			g.MB.Add(scrutineeStart, scrutineeEnd, len(switchPrefix)+len(scrutineeTempVar)+len(".(type) {\n"), "match")
			g.Write(switchPrefix)
			g.Write(scrutineeTempVar)
			g.Write(".(type) {\n")
		} else {
			// No bindings - just use scrutinee.(type) without variable
			switchPrefix := "switch "
			g.MB.Add(scrutineeStart, scrutineeEnd, len(switchPrefix)+len(scrutineeTempVar)+len(".(type) {\n"), "match")
			g.Write(switchPrefix)
			g.Write(scrutineeTempVar)
			g.Write(".(type) {\n")
		}
	} else {
		// Generate value switch: switch scrutinee {
		switchPrefix := "switch "
		g.MB.Add(scrutineeStart, scrutineeEnd, len(switchPrefix)+len(scrutineeTempVar)+len(" {\n"), "match")
		g.Write(switchPrefix)
		g.Write(scrutineeTempVar)
		g.Write(" {\n")
	}

	// Generate cases for each arm
	for _, arm := range g.Match.Arms {
		g.generateMatchArm(arm, needsTypeSwitch)
	}

	g.WriteByte('}')

	// CRITICAL: For expression matches in return context, add unreachable panic
	// This is NOT for runtime safety (exhaustiveness checker ensures all cases covered)
	// This is for Go's control flow analysis - it requires a return after the switch
	// The Go compiler will optimize this away as dead code
	if g.Match.IsExpr && g.Context != nil && g.Context.Context == ast.ContextReturn {
		g.Write("\npanic(\"unreachable: exhaustive match\")")
	}

	// NO PANIC for statement matches: silent fall-through per user requirement
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

// hasBindings checks if any arm has patterns that need variable bindings.
// This determines whether we need a temp var in the type switch.
func (g *MatchCodeGen) hasBindings() bool {
	for _, arm := range g.Match.Arms {
		if g.patternHasBindings(arm.Pattern) {
			return true
		}
	}
	return false
}

// patternHasBindings recursively checks if a pattern has any variable bindings.
func (g *MatchCodeGen) patternHasBindings(pattern ast.Pattern) bool {
	switch p := pattern.(type) {
	case *ast.ConstructorPattern:
		// Constructor with parameters has bindings
		return len(p.Params) > 0
	case *ast.VariablePattern:
		// Variable pattern is a binding
		return true
	case *ast.TuplePattern:
		// Tuple pattern has bindings if any element does
		for _, elem := range p.Elements {
			if g.patternHasBindings(elem) {
				return true
			}
		}
		return false
	default:
		// Wildcard, literal - no bindings
		return false
	}
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
	// Convert constructor name to full type name
	// - Result/Option: Ok → ResultOk, Some → OptionSome
	// - Enum: Status_Pending → StatusPending (strip underscore prefix)
	typeName := g.constructorToTypeName(pattern.Name)

	caseClause := "case " + typeName + ":\n"
	g.MB.Add(patternStart, patternEnd, len(caseClause), "match")
	g.Write(caseClause)

	// Extract bindings from constructor parameters
	var bindings []Binding
	numParams := len(pattern.Params)
	isTupleVariant := g.isTupleVariant(typeName)
	for i, param := range pattern.Params {
		bindings = append(bindings, g.extractBindings(param, i, numParams, isTupleVariant)...)
	}

	g.generateArmBody(arm, bindings)
}

// Binding represents a variable binding extracted from a pattern.
type Binding struct {
	Name      string
	FieldPath string // e.g., "Value", "First", "Second"
}

// extractBindings recursively extracts variable bindings from patterns.
// numParams indicates the total number of parameters in the constructor pattern.
// isTupleVariant indicates whether the current variant uses tuple (Value) or struct (named) fields.
func (g *MatchCodeGen) extractBindings(pattern ast.Pattern, index int, numParams int, isTupleVariant bool) []Binding {
	switch p := pattern.(type) {
	case *ast.VariablePattern:
		// Variable binding: x
		// For tuple variants: use "Value" (single field) or "ValueN" (multiple fields)
		// For struct variants: use the binding name as field name (e.g., userID)
		fieldPath := p.Name // Default: assume struct variant

		// Tuple variants use positional fields: Value, Value0/Value1/...
		if isTupleVariant {
			if numParams == 1 {
				fieldPath = "Value"
			} else {
				fieldPath = fmt.Sprintf("Value%d", index)
			}
		}

		return []Binding{{
			Name:      p.Name,
			FieldPath: fieldPath,
		}}
	case *ast.TuplePattern:
		// Tuple pattern: (a, b)
		var bindings []Binding
		tupleSize := len(p.Elements)
		for i, elem := range p.Elements {
			elemBindings := g.extractBindings(elem, i, tupleSize, isTupleVariant)
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
		nestedNumParams := len(p.Params)
		// TODO: Determine if nested constructor is tuple variant
		nestedIsTuple := false // Conservative: assume struct for nested
		for i, param := range p.Params {
			bindings = append(bindings, g.extractBindings(param, i, nestedNumParams, nestedIsTuple)...)
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
			// Use scrutineeTempVar (e.g., "v") set in generateMatchSwitch
			tempVar := g.scrutineeTempVar
			if tempVar == "" {
				tempVar = "v" // Fallback if not set
			}
			bindingCode += tempVar + "." + binding.FieldPath
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
		// NO PANIC: Guarded patterns are handled by exhaustiveness checking
		// Expression matches with guards must have unguarded fallback (verified by checker)
		// Statement matches with guards simply fall through if guard fails
	}
}

// constructorToTypeName converts a bare constructor name to the full type name.
// This handles both built-in Result/Option types and user-defined enums.
//
// NOTE: This is SEMANTIC TRANSFORMATION, not source parsing.
// The constructorName comes from ConstructorPattern.Name which was already
// properly parsed by pkg/parser. This function transforms naming conventions
// from Dingo pattern syntax to Go type names.
//
// Examples:
//   - Ok → ResultOk
//   - Err → ResultErr
//   - Some → OptionSome
//   - None → OptionNone
//   - Status_Pending → StatusPending (strip underscore)
//   - Status_Active → StatusActive (strip underscore)
//   - UserCreated (with registry) → EventUserCreated (prefix from enum registry)
func (g *MatchCodeGen) constructorToTypeName(constructorName string) string {
	// Check for built-in Result/Option constructors
	switch constructorName {
	case "Ok":
		return "ResultOk"
	case "Err":
		return "ResultErr"
	case "Some":
		return "OptionSome"
	case "None":
		return "OptionNone"
	}

	// For user-defined enums: Pattern has TypeName_Variant format
	// Example: "Status_Pending" → "StatusPending" (remove underscore)
	// This matches the generated enum variant type names
	//
	// NOTE: This string operation is on PARSED AST DATA (ConstructorPattern.Name),
	// not on source bytes. The underscore convention is Dingo syntax for qualified
	// enum variant names, parsed by pkg/parser/match.go.
	for i := 0; i < len(constructorName); i++ {
		if constructorName[i] == '_' && i > 0 {
			enumTypeName := constructorName[:i]
			variantName := constructorName[i+1:]
			return enumTypeName + variantName
		}
	}

	// Check enum registry for unqualified variant names
	// Example: "UserCreated" with registry["UserCreated"] = "Event" → "EventUserCreated"
	if g.Context != nil && g.Context.EnumRegistry != nil {
		if enumName, ok := g.Context.EnumRegistry[constructorName]; ok {
			return enumName + constructorName
		}
	}

	// Fallback: return as-is
	// This handles cases where the pattern already has the correct format
	return constructorName
}

// isTupleVariant determines if a type name represents a tuple variant.
// Tuple variants use positional fields (Value, Value0, Value1, ...).
// Struct variants use named fields matching the struct definition.
//
// Currently handles:
//   - Result variants: ResultOk, ResultErr (tuple)
//   - Option variants: OptionSome, OptionNone (tuple)
//   - Custom enums: assumed to be struct variants
//
// TODO: This should lookup variant metadata from a type registry
// instead of hard-coding known types.
func (g *MatchCodeGen) isTupleVariant(typeName string) bool {
	// Result and Option built-in types use tuple variants
	switch typeName {
	case "ResultOk", "ResultErr", "OptionSome", "OptionNone":
		return true
	}

	// Custom enums currently assumed to be struct variants
	// This will be fixed when we have a proper type registry (see action items #5)
	return false
}

// generateHumanLikeAssignment generates code for assignment context (x := match ...).
// Produces: var x TYPE; switch { case ...: x = value }
// Falls back to IIFE if type cannot be inferred.
func (g *MatchCodeGen) generateHumanLikeAssignment() ast.CodeGenResult {
	varName := g.Context.VarName
	varType := g.Context.VarType

	if varType == "" {
		// IIFE fallback when type cannot be inferred
		g.generateMatchIIFE()
		return g.Result()
	}

	// Track mapping for var declaration
	matchPos := int(g.Match.Pos())
	varDecl := fmt.Sprintf("var %s %s\n", varName, varType)
	g.MB.Add(matchPos, matchPos+5, len(varDecl), "match")
	g.Write(varDecl)

	// Generate switch with assignments
	g.generateMatchSwitchWithAssignment(varName)

	// Get result and move output to StatementOutput
	result := g.Result()
	result.StatementOutput = result.Output // Move to statement output
	result.Output = nil                      // Clear expression output

	return result
}

// generateHoistedMatch generates code for argument context (fn(match ...)).
// Produces: var tmp TYPE; switch { case ...: tmp = value }; fn(tmp)
// Returns temp var name as expression replacement.
// Falls back to IIFE if type cannot be inferred.
func (g *MatchCodeGen) generateHoistedMatch() ast.CodeGenResult {
	tmpVar := g.TempVar("result")
	varType := g.Context.VarType

	if varType == "" {
		// IIFE fallback when type cannot be inferred
		g.generateMatchIIFE()
		return g.Result()
	}

	// Generate hoisted code: var tmp TYPE; switch { case ...: tmp = value }
	hoistedBuf := &bytes.Buffer{}
	hoistedBuf.WriteString(fmt.Sprintf("var %s %s\n", tmpVar, varType))

	// Save current buffer and switch to hoistedBuf
	// CRITICAL FIX: Keep using original MappingBuilder so mappings accumulate
	origBuf := g.Buf
	g.Buf = *hoistedBuf
	// DON'T create new MB - keep using g.MB so LSP mappings are preserved

	// Generate switch with assignments to tmpVar
	g.generateMatchSwitchWithAssignment(tmpVar)

	// Get the hoisted code from the buffer
	hoistedCode := g.Buf.Bytes()

	// Restore original buffer (mappings already accumulated in g.MB)
	g.Buf = origBuf

	// Build result
	result := g.Result()
	result.HoistedCode = hoistedCode // Hoisted code goes BEFORE the statement
	result.Output = []byte(tmpVar)   // Replace match expression with tmpVar

	return result
}

// generateMatchSwitchWithAssignment generates switch statement that assigns to varName.
// Example: switch x.(type) { case A: varName = "a"; case B: varName = "b" }
// Includes default panic with scrutinee value.
func (g *MatchCodeGen) generateMatchSwitchWithAssignment(varName string) {
	// Generate scrutinee
	scrutineeResult := GenerateExpr(g.Match.Scrutinee)
	scrutineeCode := string(scrutineeResult.Output)

	// Track mapping for scrutinee
	scrutineeStart := int(g.Match.Scrutinee.Pos())
	scrutineeEnd := int(g.Match.Scrutinee.End())

	// CRITICAL FIX: Always bind match value to temp var to prevent double evaluation
	// This prevents side-effect functions (e.g., popQueue()) from executing twice:
	// once in switch condition and once in panic clause
	scrutineeTempVar := g.SharedTempVar("val")
	g.Write(fmt.Sprintf("%s := %s\n", scrutineeTempVar, scrutineeCode))

	// Check if we need type assertion (for constructor patterns)
	needsTypeSwitch := g.hasConstructorPatterns()

	// Check if we have any bindings (to decide on temp var usage)
	hasBindings := g.hasBindings()

	if needsTypeSwitch {
		// Generate type switch: switch v := scrutinee.(type) or switch scrutinee.(type)
		if hasBindings {
			// Need temp var for bindings
			tempVar := g.SharedTempVar("v")
			g.scrutineeTempVar = tempVar // Store for generateArmBodyWithAssignment
			switchPrefix := "switch " + tempVar + " := "
			g.MB.Add(scrutineeStart, scrutineeEnd, len(switchPrefix)+len(scrutineeTempVar)+len(".(type) {\n"), "match")
			g.Write(switchPrefix)
			g.Write(scrutineeTempVar)
			g.Write(".(type) {\n")
		} else {
			// No bindings - just use scrutinee.(type) without variable
			switchPrefix := "switch "
			g.MB.Add(scrutineeStart, scrutineeEnd, len(switchPrefix)+len(scrutineeTempVar)+len(".(type) {\n"), "match")
			g.Write(switchPrefix)
			g.Write(scrutineeTempVar)
			g.Write(".(type) {\n")
		}
	} else {
		// Generate value switch: switch scrutinee {
		switchPrefix := "switch "
		g.MB.Add(scrutineeStart, scrutineeEnd, len(switchPrefix)+len(scrutineeTempVar)+len(" {\n"), "match")
		g.Write(switchPrefix)
		g.Write(scrutineeTempVar)
		g.Write(" {\n")
	}

	// Generate cases for each arm with assignments
	for _, arm := range g.Match.Arms {
		g.generateMatchArmWithAssignment(arm, varName, needsTypeSwitch)
	}

	// NO DEFAULT PANIC:
	// - Expression matches: verified exhaustive by checkExhaustiveness()
	// - Statement matches: silent fall-through per user requirement

	g.WriteByte('}')
}

// generateMatchArmWithAssignment generates a case clause with assignment to varName.
// Handles pattern matching, guards, and assignment generation.
func (g *MatchCodeGen) generateMatchArmWithAssignment(arm *ast.MatchArm, varName string, typeSwitch bool) {
	// Track mapping for pattern
	patternStart := int(arm.PatternPos)
	patternEnd := int(arm.Pattern.End())

	// Generate case clause based on pattern type
	switch pattern := arm.Pattern.(type) {
	case *ast.ConstructorPattern:
		g.generateConstructorCaseWithAssignment(pattern, arm, varName, typeSwitch, patternStart, patternEnd)
	case *ast.LiteralPattern:
		g.generateLiteralCaseWithAssignment(pattern, arm, varName, patternStart, patternEnd)
	case *ast.WildcardPattern:
		g.generateWildcardCaseWithAssignment(arm, varName, patternStart, patternEnd)
	case *ast.VariablePattern:
		g.generateVariableCaseWithAssignment(pattern, arm, varName, patternStart, patternEnd)
	case *ast.TuplePattern:
		g.generateTupleCaseWithAssignment(pattern, arm, varName, patternStart, patternEnd)
	default:
		// Unknown pattern type - generate default case
		defaultCase := "default:\n"
		g.MB.Add(patternStart, patternEnd, len(defaultCase), "match")
		g.Write(defaultCase)
		g.generateArmBodyWithAssignment(arm, varName, nil)
	}
}

// generateConstructorCaseWithAssignment generates case for constructor patterns with assignment.
func (g *MatchCodeGen) generateConstructorCaseWithAssignment(pattern *ast.ConstructorPattern, arm *ast.MatchArm, varName string, typeSwitch bool, patternStart, patternEnd int) {
	// Convert constructor name to full type name
	typeName := g.constructorToTypeName(pattern.Name)

	caseClause := "case " + typeName + ":\n"
	g.MB.Add(patternStart, patternEnd, len(caseClause), "match")
	g.Write(caseClause)

	// Extract bindings from constructor parameters
	var bindings []Binding
	numParams := len(pattern.Params)
	isTupleVariant := g.isTupleVariant(typeName)
	for i, param := range pattern.Params {
		bindings = append(bindings, g.extractBindings(param, i, numParams, isTupleVariant)...)
	}

	g.generateArmBodyWithAssignment(arm, varName, bindings)
}

// generateLiteralCaseWithAssignment generates case for literal patterns with assignment.
func (g *MatchCodeGen) generateLiteralCaseWithAssignment(pattern *ast.LiteralPattern, arm *ast.MatchArm, varName string, patternStart, patternEnd int) {
	caseClause := "case " + pattern.Value + ":\n"
	g.MB.Add(patternStart, patternEnd, len(caseClause), "match")
	g.Write(caseClause)
	g.generateArmBodyWithAssignment(arm, varName, nil)
}

// generateWildcardCaseWithAssignment generates default case for wildcard pattern with assignment.
func (g *MatchCodeGen) generateWildcardCaseWithAssignment(arm *ast.MatchArm, varName string, patternStart, patternEnd int) {
	defaultCase := "default:\n"
	g.MB.Add(patternStart, patternEnd, len(defaultCase), "match")
	g.Write(defaultCase)
	g.generateArmBodyWithAssignment(arm, varName, nil)
}

// generateVariableCaseWithAssignment generates default case with binding for variable pattern with assignment.
func (g *MatchCodeGen) generateVariableCaseWithAssignment(pattern *ast.VariablePattern, arm *ast.MatchArm, varName string, patternStart, patternEnd int) {
	defaultCase := "default:\n"
	g.MB.Add(patternStart, patternEnd, len(defaultCase), "match")
	g.Write(defaultCase)

	// Bind variable to scrutinee value
	bindings := []Binding{{
		Name:      pattern.Name,
		FieldPath: "", // Bind to scrutinee itself
	}}
	g.generateArmBodyWithAssignment(arm, varName, bindings)
}

// generateTupleCaseWithAssignment generates case for tuple patterns with assignment.
func (g *MatchCodeGen) generateTupleCaseWithAssignment(pattern *ast.TuplePattern, arm *ast.MatchArm, varName string, patternStart, patternEnd int) {
	// Tuple patterns need special handling - not implemented yet
	// For now, generate default case
	defaultCase := "default:\n"
	g.MB.Add(patternStart, patternEnd, len(defaultCase), "match")
	g.Write(defaultCase)
	g.generateArmBodyWithAssignment(arm, varName, nil)
}

// generateArmBodyWithAssignment generates the body of a match arm with assignment to varName.
// Handles variable bindings, guards, and assignment generation.
func (g *MatchCodeGen) generateArmBodyWithAssignment(arm *ast.MatchArm, varName string, bindings []Binding) {
	// Generate variable bindings
	for _, binding := range bindings {
		bindingCode := binding.Name + " := "

		// If binding has a field path, use type assertion variable
		if binding.FieldPath != "" {
			// Use scrutineeTempVar (e.g., "v") set in generateMatchSwitchWithAssignment
			tempVar := g.scrutineeTempVar
			if tempVar == "" {
				tempVar = "v" // Fallback if not set
			}
			bindingCode += tempVar + "." + binding.FieldPath
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

	// Generate body with assignment
	bodyStart := int(arm.Body.Pos())
	bodyEnd := int(arm.Body.End())

	bodyResult := GenerateExpr(arm.Body)
	assignCode := fmt.Sprintf("\t%s = %s\n", varName, string(bodyResult.Output))
	g.MB.Add(bodyStart, bodyEnd, len(assignCode), "match")
	g.Write(assignCode)

	// Close guard if present
	if arm.Guard != nil {
		g.Write("}\n")
		// NO PANIC: Guarded patterns are handled by exhaustiveness checking
		// The guard failure simply means this case doesn't match, so try next case
	}
}

// checkExhaustiveness validates exhaustiveness for expression matches.
// Returns error result if non-exhaustive; otherwise returns empty result.
func (g *MatchCodeGen) checkExhaustiveness() ast.CodeGenResult {
	// Only check if we have a registry (might be nil in tests or simple cases)
	if g.Context == nil || g.Context.EnumRegistry == nil {
		return ast.CodeGenResult{}
	}

	// Create exhaustiveness checker with registry
	registry := typechecker.NewEnumRegistry()

	// Populate registry from context
	// GenContext.EnumRegistry is map[string]string (variant -> enum)
	// We need to convert to typechecker.EnumRegistry format
	enumVariants := make(map[string][]typechecker.VariantInfo)
	for variantName, enumName := range g.Context.EnumRegistry {
		vi := typechecker.VariantInfo{
			Name:     variantName,
			FullName: enumName + variantName,
			// Note: Fields/FieldTypes would need metadata from enum declaration
			// For exhaustiveness checking, we only need variant names
		}
		enumVariants[enumName] = append(enumVariants[enumName], vi)
	}

	// Register all enums in the registry
	for enumName, variants := range enumVariants {
		if err := registry.RegisterEnum(enumName, variants); err != nil {
			// Variant name collision detected
			return ast.CodeGenResult{
				Error: &ast.CodeGenError{
					Position: int(g.Match.Match),
					Message:  err.Error(),
					Hint:     "rename variants to avoid conflicts",
				},
			}
		}
	}

	// Pass type checker from context (if available)
	checker := typechecker.NewExhaustivenessChecker(registry, nil)

	// Check exhaustiveness
	result := checker.Check(g.Match, g.Match.IsExpr)

	// If non-exhaustive expression match, return error
	if !result.IsExhaustive && g.Match.IsExpr {
		return g.generateExhaustivenessError(result)
	}

	return ast.CodeGenResult{}
}

// generateExhaustivenessError creates a compile-time error for non-exhaustive match
func (g *MatchCodeGen) generateExhaustivenessError(result *typechecker.ExhaustivenessResult) ast.CodeGenResult {
	var msg string
	if len(result.MissingVariants) > 0 {
		msg = fmt.Sprintf(
			"non-exhaustive match on %s: missing variants: %s",
			result.EnumName,
			formatVariantList(result.MissingVariants),
		)
	} else {
		msg = "non-exhaustive match expression"
	}

	hint := "add missing variants or use a wildcard pattern (_)"

	// Return structured error that halts transpilation during dingo build
	// This produces a clear error at .dingo file location, not cryptic Go compiler error
	return ast.CodeGenResult{
		Error: &ast.CodeGenError{
			Position: int(result.Position),
			Message:  msg,
			Hint:     hint,
		},
	}
}

// formatVariantList formats variant names for error messages
func formatVariantList(variants []string) string {
	if len(variants) == 0 {
		return ""
	}
	if len(variants) == 1 {
		return variants[0]
	}
	if len(variants) == 2 {
		return variants[0] + ", " + variants[1]
	}
	// More than 2: show first 2 and count
	return fmt.Sprintf("%s, %s (and %d more)", variants[0], variants[1], len(variants)-2)
}
