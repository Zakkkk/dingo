package ast

import (
	"go/token"
	"strings"
)

// EnumDecl represents a Dingo enum/sum type declaration
// Examples:
//   - enum Result[T, E] { Ok(T), Err(E) }
//   - enum Color { Red, Green, Blue, RGB { r: int, g: int, b: int } }
type EnumDecl struct {
	Enum       token.Pos        // Position of 'enum' keyword
	Name       *Ident           // Enum name
	TypeParams *TypeParamList   // Generic type parameters (optional)
	LBrace     token.Pos        // Position of '{'
	Variants   []*EnumVariant   // Enum variants
	RBrace     token.Pos        // Position of '}'
}

// Ident represents an identifier
type Ident struct {
	NamePos token.Pos
	Name    string
}

func (i *Ident) Pos() token.Pos { return i.NamePos }
func (i *Ident) End() token.Pos { return i.NamePos + token.Pos(len(i.Name)) }
func (i *Ident) String() string { return i.Name }

// TypeParamList represents generic type parameters: <T, E>
type TypeParamList struct {
	Opening token.Pos   // Position of '<'
	Params  []*Ident    // Type parameter names
	Closing token.Pos   // Position of '>'
}

func (t *TypeParamList) Pos() token.Pos { return t.Opening }
func (t *TypeParamList) End() token.Pos { return t.Closing + 1 }
func (t *TypeParamList) String() string {
	if len(t.Params) == 0 {
		return ""
	}
	names := make([]string, len(t.Params))
	for i, p := range t.Params {
		names[i] = p.Name
	}
	return "[" + strings.Join(names, ", ") + "]"
}

// EnumVariant represents one variant of an enum
// Examples:
//   - Red (unit variant)
//   - Ok(T) (tuple variant)
//   - RGB { r: int, g: int, b: int } (struct variant)
type EnumVariant struct {
	Name    *Ident          // Variant name
	Kind    EnumFieldKind   // Variant kind (unit/tuple/struct)
	LDelim  token.Pos       // Position of '(' or '{' (zero if unit)
	Fields  []*EnumField    // Fields (empty for unit variants)
	RDelim  token.Pos       // Position of ')' or '}' (zero if unit)
	Comma   token.Pos       // Position of trailing comma (if present)
}

func (v *EnumVariant) Pos() token.Pos { return v.Name.Pos() }
func (v *EnumVariant) End() token.Pos {
	if v.RDelim.IsValid() {
		return v.RDelim + 1
	}
	return v.Name.End()
}
func (v *EnumVariant) String() string {
	s := v.Name.Name
	if len(v.Fields) == 0 {
		return s
	}

	fields := make([]string, len(v.Fields))
	for i, f := range v.Fields {
		fields[i] = f.String()
	}

	switch v.Kind {
	case TupleVariant:
		return s + "(" + strings.Join(fields, ", ") + ")"
	case StructVariant:
		return s + " { " + strings.Join(fields, ", ") + " }"
	default:
		return s
	}
}

// EnumFieldKind represents the kind of enum variant
type EnumFieldKind int

const (
	UnitVariant   EnumFieldKind = iota // Red (no data)
	TupleVariant                       // Ok(T) (positional fields)
	StructVariant                      // RGB { r: int, g: int, b: int } (named fields)
)

func (k EnumFieldKind) String() string {
	switch k {
	case UnitVariant:
		return "unit"
	case TupleVariant:
		return "tuple"
	case StructVariant:
		return "struct"
	default:
		return "unknown"
	}
}

// EnumField represents a field in a tuple or struct variant
// Examples:
//   - T (tuple field, no name)
//   - r: int (struct field, with name)
type EnumField struct {
	Name     *Ident    // Field name (nil for tuple variants)
	Colon    token.Pos // Position of ':' (zero for tuple variants)
	Type     *TypeExpr // Field type
}

func (f *EnumField) Pos() token.Pos {
	if f.Name != nil {
		return f.Name.Pos()
	}
	return f.Type.Pos()
}
func (f *EnumField) End() token.Pos {
	return f.Type.End()
}
func (f *EnumField) String() string {
	if f.Name != nil {
		return f.Name.Name + ": " + f.Type.String()
	}
	return f.Type.String()
}

// TypeExpr represents a type expression
// Examples: int, string, T, Option[T], []int
type TypeExpr struct {
	StartPos token.Pos
	EndPos   token.Pos
	Text     string // Type as string (e.g., "int", "T", "Option[T]")
}

func (t *TypeExpr) Pos() token.Pos { return t.StartPos }
func (t *TypeExpr) End() token.Pos { return t.EndPos }
func (t *TypeExpr) String() string { return t.Text }

// Implement Decl interface for EnumDecl
func (e *EnumDecl) Node()          {}
func (e *EnumDecl) declNode()      {}
func (e *EnumDecl) Pos() token.Pos { return e.Enum }
func (e *EnumDecl) End() token.Pos {
	if e.RBrace.IsValid() {
		return e.RBrace + 1
	}
	return e.Enum
}
func (e *EnumDecl) String() string {
	s := "enum " + e.Name.Name
	if e.TypeParams != nil {
		s += e.TypeParams.String()
	}
	s += " {"
	if len(e.Variants) > 0 {
		variants := make([]string, len(e.Variants))
		for i, v := range e.Variants {
			variants[i] = v.String()
		}
		s += " " + strings.Join(variants, ", ") + " "
	}
	s += "}"
	return s
}
