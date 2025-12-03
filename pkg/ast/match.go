package ast

import (
	"go/token"
	"strings"
)

// MatchExpr represents a Rust-style pattern match expression
// Examples:
//   - match result { Ok(x) => x, Err(e) => 0 }
//   - match status { Status_Pending => "waiting", Status_Active => "running", _ => "other" }
//   - match val { 1 => "one", 2 => "two", _ => "other" }
//   - match (a, b) { (Ok, Ok) => ..., (Err, _) => ..., _ => ... }
type MatchExpr struct {
	MatchPos   token.Pos   // Position of 'match' keyword
	Scrutinee  string      // Expression being matched (e.g., "result", "status")
	Arms       []MatchArm  // Match arms
	IsExpr     bool        // true if used as expression (return/assign), false if statement
	MatchID    int         // Unique ID for this match (for temp variable naming)
}

// MatchArm represents one arm of a match expression
// Example: Ok(x) if x > 0 => x * 2
type MatchArm struct {
	Pattern Pattern // Pattern to match against
	Guard   string  // Guard condition (optional, e.g., "x > 0")
	Body    string  // Body expression or block
	IsBlock bool    // true if body is { ... }, false if expression
}

// Pattern represents a pattern in a match arm
type Pattern interface {
	PatternNode() // Marker interface
	String() string
}

// LiteralPattern represents a literal pattern: 1, "hello", true
type LiteralPattern struct {
	Value string // Literal value as string
}

func (p *LiteralPattern) PatternNode() {}
func (p *LiteralPattern) String() string {
	return p.Value
}

// ConstructorPattern represents a constructor pattern: Ok(x), Err(e), Some(v), None
type ConstructorPattern struct {
	Name   string   // Constructor name (Ok, Err, Some, None, or enum variant)
	Params []string // Parameter names for binding (empty for None, etc.)
}

func (p *ConstructorPattern) PatternNode() {}
func (p *ConstructorPattern) String() string {
	if len(p.Params) == 0 {
		return p.Name
	}
	return p.Name + "(" + strings.Join(p.Params, ", ") + ")"
}

// TuplePattern represents a tuple pattern: (Ok, Err), (a, b)
type TuplePattern struct {
	Elements []Pattern // Patterns for each tuple element
}

func (p *TuplePattern) PatternNode() {}
func (p *TuplePattern) String() string {
	var parts []string
	for _, elem := range p.Elements {
		parts = append(parts, elem.String())
	}
	return "(" + strings.Join(parts, ", ") + ")"
}

// WildcardPattern represents the wildcard pattern: _
type WildcardPattern struct{}

func (p *WildcardPattern) PatternNode() {}
func (p *WildcardPattern) String() string {
	return "_"
}

// VariablePattern represents a variable binding pattern: x, y
type VariablePattern struct {
	Name string // Variable name
}

func (p *VariablePattern) PatternNode() {}
func (p *VariablePattern) String() string {
	return p.Name
}

// Node implements DingoNode marker interface
func (m *MatchExpr) Node() {}

// Pos returns the position of the match expression
func (m *MatchExpr) Pos() token.Pos {
	return m.MatchPos
}

// End returns the end position (approximation)
func (m *MatchExpr) End() token.Pos {
	// Approximate end based on generated Go code
	return m.MatchPos + token.Pos(len(m.ToGo()))
}

// ToGo converts MatchExpr to Go code with proper switch statement
// Outputs:
//   - match result { Ok(x) => x, Err(e) => 0 } → switch with case Ok/Err
//   - Preserves source map markers for LSP integration
func (m *MatchExpr) ToGo() string {
	var result strings.Builder

	// Determine result variable name (only for expression context)
	var resultVar string
	if m.IsExpr {
		resultVar = formatMatchResultVar(m.MatchID)
		result.WriteString("var ")
		result.WriteString(resultVar)
		result.WriteString(" __TYPE_INFERENCE_NEEDED\n")
	}

	// DINGO_MATCH_START marker
	result.WriteString("// DINGO_MATCH_START: ")
	result.WriteString(m.Scrutinee)
	result.WriteString("\n")

	// Create scrutinee variable
	result.WriteString("scrutinee := ")
	result.WriteString(m.Scrutinee)
	result.WriteString("\n")

	// Determine switch tag based on pattern type
	switchTag := m.determineSwitchTag()
	result.WriteString("switch ")
	result.WriteString(switchTag)
	result.WriteString(" {\n")

	// Generate case clauses
	for _, arm := range m.Arms {
		m.generateCaseClause(&result, arm, resultVar)
	}

	result.WriteString("}")

	// For expression context, return the result variable
	if m.IsExpr {
		result.WriteString("\nreturn ")
		result.WriteString(resultVar)
	}

	result.WriteString("\n// DINGO_MATCH_END\n")

	return result.String()
}

// determineSwitchTag returns the appropriate switch tag based on pattern types
func (m *MatchExpr) determineSwitchTag() string {
	if len(m.Arms) == 0 {
		return "scrutinee"
	}

	// Check first pattern to determine type
	firstPattern := m.Arms[0].Pattern

	switch p := firstPattern.(type) {
	case *ConstructorPattern:
		// Constructor patterns (Result, Option, Enum) switch on .tag
		// Special cases: Check if it's a known enum variant
		if isEnumVariant(p.Name) {
			return "scrutinee.tag"
		}
		// Result/Option also use .tag
		if p.Name == "Ok" || p.Name == "Err" || p.Name == "Some" || p.Name == "None" {
			return "scrutinee.tag"
		}
		return "scrutinee.tag"
	case *TuplePattern:
		// Tuple patterns need special handling - no switch tag
		return ""
	case *LiteralPattern:
		// Literal patterns switch on value directly
		return "scrutinee"
	case *WildcardPattern:
		// Wildcard - switch on scrutinee
		return "scrutinee"
	case *VariablePattern:
		// Variable binding - switch on scrutinee
		return "scrutinee"
	default:
		return "scrutinee"
	}
}

// generateCaseClause generates a single case clause for a match arm
func (m *MatchExpr) generateCaseClause(result *strings.Builder, arm MatchArm, resultVar string) {
	// Generate case pattern
	result.WriteString("case ")
	m.generateCasePattern(result, arm.Pattern)
	result.WriteString(":\n")

	// DINGO_PATTERN marker
	result.WriteString("// DINGO_PATTERN: ")
	result.WriteString(arm.Pattern.String())
	result.WriteString("\n")

	// Guard condition (if present)
	if arm.Guard != "" {
		result.WriteString("if !(")
		result.WriteString(arm.Guard)
		result.WriteString(") {\n")
		result.WriteString("break // Guard failed\n")
		result.WriteString("}\n")
	}

	// Destructure constructor parameters if needed
	if ctor, ok := arm.Pattern.(*ConstructorPattern); ok && len(ctor.Params) > 0 {
		for _, param := range ctor.Params {
			result.WriteString(param)
			result.WriteString(" := scrutinee.")
			result.WriteString(capitalize(param))
			result.WriteString("\n")
		}
	}

	// Body
	if m.IsExpr {
		// Expression context - assign to result variable
		result.WriteString(resultVar)
		result.WriteString(" = ")
		if arm.IsBlock {
			result.WriteString("func() __TYPE_INFERENCE_NEEDED ")
			result.WriteString(arm.Body)
			result.WriteString("()")
		} else {
			result.WriteString(arm.Body)
		}
		result.WriteString("\n")
	} else {
		// Statement context - just execute body
		if arm.IsBlock {
			result.WriteString(arm.Body)
			result.WriteString("\n")
		} else {
			result.WriteString(arm.Body)
			result.WriteString("\n")
		}
	}
}

// generateCasePattern generates the case pattern for a switch
func (m *MatchExpr) generateCasePattern(result *strings.Builder, pattern Pattern) {
	switch p := pattern.(type) {
	case *LiteralPattern:
		result.WriteString(p.Value)
	case *ConstructorPattern:
		// Map constructor name to tag constant
		tagName := m.constructorToTag(p.Name)
		result.WriteString(tagName)
	case *WildcardPattern:
		result.WriteString("default")
	case *VariablePattern:
		result.WriteString("default")
	case *TuplePattern:
		// Tuple patterns require nested switches - handle specially
		// For now, emit default (this should be handled in preprocessing)
		result.WriteString("default")
	}
}

// constructorToTag maps a constructor name to its tag constant
func (m *MatchExpr) constructorToTag(name string) string {
	// Standard Result/Option constructors
	switch name {
	case "Ok":
		return "ResultTagOk"
	case "Err":
		return "ResultTagErr"
	case "Some":
		return "OptionTagSome"
	case "None":
		return "OptionTagNone"
	}

	// Enum variants - check if it contains underscore (qualified name)
	if strings.Contains(name, "_") {
		// Status_Pending → StatusTagPending
		parts := strings.Split(name, "_")
		if len(parts) >= 2 {
			enumName := parts[0]
			variantName := strings.Join(parts[1:], "_")
			return enumName + "Tag" + variantName
		}
	}

	// Bare variant name - assume it's already been resolved
	// (preprocessor should have qualified it)
	return name + "Tag"
}

// formatMatchResultVar returns the result variable name for a match ID
// Follows CLAUDE.md naming convention: camelCase, first unnumbered, then numbered from 1
func formatMatchResultVar(id int) string {
	if id == 0 {
		return "matchResult"
	}
	return "matchResult" + string(rune('0'+id))
}

// isEnumVariant checks if a name looks like an enum variant
func isEnumVariant(name string) bool {
	// Check for qualified name pattern: EnumName_VariantName
	return strings.Contains(name, "_") ||
		// Or known constructors
		name == "Ok" || name == "Err" || name == "Some" || name == "None"
}

// capitalize returns the string with first letter capitalized
func capitalize(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
