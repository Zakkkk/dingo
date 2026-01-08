package ast

import (
	"go/token"
	"strings"
)

// OrPattern represents pattern alternatives: Ok(_) | Err(_)
// Used in match expressions for combining multiple patterns
type OrPattern struct {
	Left  Pattern   // Left pattern
	Pipe  token.Pos // Position of '|'
	Right Pattern   // Right pattern
}

func (p *OrPattern) PatternNode() {}
func (p *OrPattern) Pos() token.Pos {
	return p.Left.Pos()
}
func (p *OrPattern) End() token.Pos {
	return p.Right.End()
}
func (p *OrPattern) String() string {
	return p.Left.String() + " | " + p.Right.String()
}
func (p *OrPattern) HasBindings() bool {
	// Both sides must bind same variables (validated elsewhere)
	return p.Left.HasBindings() || p.Right.HasBindings()
}
func (p *OrPattern) GetBindings() []Binding {
	// Return bindings from left side (right must match)
	return p.Left.GetBindings()
}

// RestPattern represents rest pattern in slice/array: [a, b, ...rest]
// Matches remaining elements in sequence patterns
type RestPattern struct {
	Dots token.Pos // Position of '...'
	Name *string   // Optional variable name (nil for anonymous ...)
}

func (p *RestPattern) PatternNode() {}
func (p *RestPattern) Pos() token.Pos {
	return p.Dots
}
func (p *RestPattern) End() token.Pos {
	if p.Name != nil {
		return p.Dots + token.Pos(3+len(*p.Name))
	}
	return p.Dots + 3
}
func (p *RestPattern) String() string {
	if p.Name != nil {
		return "..." + *p.Name
	}
	return "..."
}
func (p *RestPattern) HasBindings() bool {
	return p.Name != nil
}
func (p *RestPattern) GetBindings() []Binding {
	if p.Name != nil {
		return []Binding{{Name: *p.Name, Pos: p.Dots, Path: nil}}
	}
	return nil
}

// RangePattern represents numeric range: 1..10, 'a'..'z'
// Used for matching values within a range
type RangePattern struct {
	Start     Pattern   // Start of range (literal)
	DotDot    token.Pos // Position of '..'
	EndValue  Pattern   // End of range (literal)
	Inclusive bool      // true for ..=, false for ..
}

func (p *RangePattern) PatternNode() {}
func (p *RangePattern) Pos() token.Pos {
	return p.Start.Pos()
}
func (p *RangePattern) End() token.Pos {
	return p.EndValue.End()
}
func (p *RangePattern) String() string {
	if p.Inclusive {
		return p.Start.String() + "..=" + p.EndValue.String()
	}
	return p.Start.String() + ".." + p.EndValue.String()
}
func (p *RangePattern) HasBindings() bool {
	return false // Ranges don't bind variables
}
func (p *RangePattern) GetBindings() []Binding {
	return nil
}

// SlicePattern represents slice/array pattern: [a, b, c] or [first, ...rest]
// Supports fixed and variable-length matching
type SlicePattern struct {
	LBracket token.Pos // Position of '['
	Elements []Pattern // Elements (may include RestPattern)
	RBracket token.Pos // Position of ']'
}

func (p *SlicePattern) PatternNode() {}
func (p *SlicePattern) Pos() token.Pos {
	return p.LBracket
}
func (p *SlicePattern) End() token.Pos {
	return p.RBracket + 1
}
func (p *SlicePattern) String() string {
	parts := make([]string, len(p.Elements))
	for i, elem := range p.Elements {
		parts[i] = elem.String()
	}
	return "[" + strings.Join(parts, ", ") + "]"
}
func (p *SlicePattern) HasBindings() bool {
	for _, elem := range p.Elements {
		if elem.HasBindings() {
			return true
		}
	}
	return false
}
func (p *SlicePattern) GetBindings() []Binding {
	var bindings []Binding
	for i, elem := range p.Elements {
		for _, b := range elem.GetBindings() {
			b.Path = append([]int{i}, b.Path...)
			bindings = append(bindings, b)
		}
	}
	return bindings
}

// StructFieldPattern represents struct field pattern: {x, y: newName}
// Used in struct destructuring
type StructFieldPattern struct {
	Field    string    // Field name
	FieldPos token.Pos // Position of field name
	Colon    token.Pos // Position of ':' (zero if shorthand)
	Pattern  Pattern   // Pattern for field value (nil if shorthand)
}

func (p *StructFieldPattern) PatternNode() {}
func (p *StructFieldPattern) Pos() token.Pos {
	return p.FieldPos
}
func (p *StructFieldPattern) End() token.Pos {
	if p.Pattern != nil {
		return p.Pattern.End()
	}
	return p.FieldPos + token.Pos(len(p.Field))
}
func (p *StructFieldPattern) String() string {
	if p.Pattern != nil {
		return p.Field + ": " + p.Pattern.String()
	}
	return p.Field
}
func (p *StructFieldPattern) HasBindings() bool {
	if p.Pattern != nil {
		return p.Pattern.HasBindings()
	}
	return true // Shorthand always binds
}
func (p *StructFieldPattern) GetBindings() []Binding {
	if p.Pattern != nil {
		return p.Pattern.GetBindings()
	}
	return []Binding{{Name: p.Field, Pos: p.FieldPos, Path: nil}}
}
