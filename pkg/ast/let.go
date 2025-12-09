package ast

import (
	"strings"

	"go/token"
)

// LetDecl represents a let/var declaration in Dingo
// Example: let x: int = 5
type LetDecl struct {
	LetPos    token.Pos // Position of "let" keyword
	Names     []string  // Variable names (supports multiple: let a, b = ...)
	TypeAnnot string    // Type annotation as string (e.g., ": int", ": Option[string]")
	// Empty string means type inference
	// IMPORTANT: Include the colon during parsing! ToGo() will remove it
	Value   string // Value expression as string (unparsed)
	HasInit bool   // true if has initialization (= expr)
}

// Node implements DingoNode marker interface
func (d *LetDecl) Node() {}

// ToGo converts LetDecl to Go code
// Outputs:
//   - With type annotation (TypeAnnot non-empty):
//     let x: int = 5 → var x int = 5 (colon removed for valid Go)
//   - Without type, with init:
//     let x = 5 → x := 5 (short declaration)
//   - Multiple names with init:
//     let a, b = getValues() → a, b := getValues()
//   - Declaration without init:
//     let x: int → var x int
func (d *LetDecl) ToGo() string {
	var result string

	// With type annotation: use var declaration
	if d.TypeAnnot != "" {
		result = "var "
		// Single or multiple names
		for i, name := range d.Names {
			if i > 0 {
				result += ", "
			}
			result += name
		}
		// Add type annotation (remove leading colon if present and ensure spacing)
		cleanType := strings.TrimSpace(d.TypeAnnot)
		cleanType = strings.TrimPrefix(cleanType, ":")
		cleanType = strings.TrimSpace(cleanType) // Trim again in case of ":  int"
		if len(cleanType) > 0 {
			result += " " + cleanType
		}
		// Add initialization if present
		if d.HasInit {
			result += " = " + d.Value
		}
		return result
	}

	// Without type annotation: use short declaration
	if d.HasInit {
		// Single or multiple names
		for i, name := range d.Names {
			if i > 0 {
				result += ", "
			}
			result += name
		}
		result += " := " + d.Value
		return result
	}

	// Declaration without init and without type - invalid, but generate var anyway
	result = "var "
	for i, name := range d.Names {
		if i > 0 {
			result += ", "
		}
		result += name
	}
	return result
}

// Pos returns the position of the let keyword
func (d *LetDecl) Pos() token.Pos {
	return d.LetPos
}

// End returns the end position (approximation based on length)
func (d *LetDecl) End() token.Pos {
	// Approximate end position based on generated Go code
	// In a full AST, this would track the actual end position
	return d.LetPos + token.Pos(len(d.ToGo()))
}
