package ast

import (
	"go/token"
	"strconv"
	"strings"
)

// TupleLiteral represents a tuple literal expression in Dingo
// Examples:
//   - (10, 20) - Simple integer tuple
//   - ("hello", 42) - Mixed type tuple
//   - ((1, 2), 3) - Nested tuple
//   - (user.Name, user.Age, user.Email) - Expression elements
type TupleLiteral struct {
	Lparen   token.Pos // Position of opening '('
	Elements []Element // Tuple elements (can be expressions or nested tuples)
	Rparen   token.Pos // Position of closing ')'
}

// Element represents a single element in a tuple literal
// Can be either a regular expression or a nested tuple
type Element struct {
	Expr   string        // Expression as string (e.g., "10", "user.Name", "x + y")
	Nested *TupleLiteral // If nested tuple, this is set (e.g., (a, b) in ((a, b), c))
}

// Node implements DingoNode marker interface
func (t *TupleLiteral) Node() {}

// Pos returns the position of the tuple literal
func (t *TupleLiteral) Pos() token.Pos {
	return t.Lparen
}

// End returns the end position of the tuple literal
func (t *TupleLiteral) End() token.Pos {
	return t.Rparen + 1
}

// IsNested returns true if this tuple contains any nested tuples
func (t *TupleLiteral) IsNested() bool {
	for _, elem := range t.Elements {
		if elem.Nested != nil {
			return true
		}
	}
	return false
}

// TupleDestructure represents a tuple destructuring assignment
// Examples:
//   - let (a, b) = expr - Simple destructuring
//   - let (x, y, z) = getTuple() - Multi-element destructuring
//   - let ((a, b), c) = expr - Nested destructuring
type TupleDestructure struct {
	LetPos  token.Pos            // Position of 'let' keyword
	Pattern []DestructureElement // Destructuring pattern (supports nesting)
	Assign  token.Pos            // Position of '=' operator
	Value   string               // RHS expression being destructured
}

// DestructureElement represents an element in a destructuring pattern
// Can be either a simple identifier or a nested pattern
type DestructureElement struct {
	Name   string               // Identifier name (e.g., "a", "x")
	Nested []DestructureElement // Nested pattern elements (for patterns like ((a, b), c))
}

// Node implements DingoNode marker interface
func (t *TupleDestructure) Node() {}

// Pos returns the position of the destructuring statement
func (t *TupleDestructure) Pos() token.Pos {
	return t.LetPos
}

// End returns the end position (approximation based on value length)
func (t *TupleDestructure) End() token.Pos {
	// Approximate end: let + pattern + = + value
	return t.Assign + token.Pos(len(t.Value))
}

// IsNested returns true if this destructuring pattern contains nested patterns
func (t *TupleDestructure) IsNested() bool {
	for _, elem := range t.Pattern {
		if elem.IsNested() {
			return true
		}
	}
	return false
}

// IsNested returns true if this element is a nested pattern
func (e *DestructureElement) IsNested() bool {
	return len(e.Nested) > 0
}

// ToGo converts TupleLiteral to Go code with marker function call
// The actual transformation to struct literal is done by TuplePlugin
// Output: __TUPLE_N__LITERAL__<hash>(elem1, elem2, ...)
func (t *TupleLiteral) ToGo(markerName string) string {
	var result strings.Builder

	// Marker function call
	result.WriteString(markerName)
	result.WriteString("(")

	// Elements
	for i, elem := range t.Elements {
		if i > 0 {
			result.WriteString(", ")
		}

		if elem.Nested != nil {
			// Recursive call for nested tuple
			nestedMarker := generateNestedMarker(i)
			result.WriteString(elem.Nested.ToGo(nestedMarker))
		} else {
			result.WriteString(elem.Expr)
		}
	}

	result.WriteString(")")

	return result.String()
}

// ToGo converts TupleDestructure to Go code
// Output:
//   Simple: tmp := expr; a, b := tmp._0, tmp._1
//   Nested: tmp := expr; tmp1 := tmp._0; a, b := tmp1._0, tmp1._1; c := tmp._1
func (t *TupleDestructure) ToGo() string {
	var result strings.Builder

	// Generate temporary variable for RHS
	result.WriteString("tmp := ")
	result.WriteString(t.Value)
	result.WriteString("\n")

	// Generate destructuring assignments
	t.generateDestructuring(&result, t.Pattern, "tmp", 1)

	return strings.TrimRight(result.String(), "\n")
}

// generateDestructuring recursively generates destructuring assignments
// tmpVar is the current temporary variable being destructured
// tmpCounter is used to generate unique tmp variable names for nested patterns
func (t *TupleDestructure) generateDestructuring(result *strings.Builder, pattern []DestructureElement, tmpVar string, tmpCounter int) int {
	var simpleNames []string
	var simpleFields []string

	for i, elem := range pattern {
		if elem.IsNested() {
			// Nested pattern: create intermediate tmp variable
			nestedTmp := formatTmpVar(tmpCounter)
			tmpCounter++

			result.WriteString(nestedTmp)
			result.WriteString(" := ")
			result.WriteString(tmpVar)
			result.WriteString("._")
			result.WriteString(formatFieldIndex(i))
			result.WriteString("\n")

			// Recursively destructure the nested pattern
			tmpCounter = t.generateDestructuring(result, elem.Nested, nestedTmp, tmpCounter)
		} else {
			// Simple identifier: accumulate for batch assignment
			simpleNames = append(simpleNames, elem.Name)
			simpleFields = append(simpleFields, tmpVar+"._"+formatFieldIndex(i))
		}
	}

	// Generate batch assignment for simple identifiers
	if len(simpleNames) > 0 {
		result.WriteString(strings.Join(simpleNames, ", "))
		result.WriteString(" := ")
		result.WriteString(strings.Join(simpleFields, ", "))
		result.WriteString("\n")
	}

	return tmpCounter
}

// formatTmpVar formats temporary variable name following CLAUDE.md naming convention
// First tmp is unnumbered, subsequent are tmp1, tmp2, etc.
func formatTmpVar(counter int) string {
	if counter == 1 {
		return "tmp"
	}
	return "tmp" + formatNumber(counter)
}

// formatFieldIndex formats tuple field index (_0, _1, _2, etc.)
func formatFieldIndex(index int) string {
	return formatNumber(index)
}

// formatNumber converts integer to string (0 -> "0", 1 -> "1", etc.)
// Uses standard library for performance and correctness (handles all edge cases)
func formatNumber(n int) string {
	return strconv.Itoa(n)
}

// generateNestedMarker generates a unique marker name for nested tuples
func generateNestedMarker(index int) string {
	return "__NESTED_TUPLE_" + formatNumber(index) + "__"
}
