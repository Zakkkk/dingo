package ast

import (
	"go/token"
	"testing"
)

func TestEnumDecl_String(t *testing.T) {
	tests := []struct {
		name     string
		enumDecl *EnumDecl
		want     string
	}{
		{
			name: "simple unit variants",
			enumDecl: &EnumDecl{
				Name: &Ident{Name: "Color"},
				Variants: []*EnumVariant{
					{Name: &Ident{Name: "Red"}, Kind: UnitVariant},
					{Name: &Ident{Name: "Green"}, Kind: UnitVariant},
					{Name: &Ident{Name: "Blue"}, Kind: UnitVariant},
				},
			},
			want: "enum Color { Red, Green, Blue }",
		},
		{
			name: "generic with tuple variants",
			enumDecl: &EnumDecl{
				Name: &Ident{Name: "Result"},
				TypeParams: &TypeParamList{
					Params: []*Ident{
						{Name: "T"},
						{Name: "E"},
					},
				},
				Variants: []*EnumVariant{
					{
						Name: &Ident{Name: "Ok"},
						Kind: TupleVariant,
						Fields: []*EnumField{
							{Type: &TypeExpr{Text: "T"}},
						},
					},
					{
						Name: &Ident{Name: "Err"},
						Kind: TupleVariant,
						Fields: []*EnumField{
							{Type: &TypeExpr{Text: "E"}},
						},
					},
				},
			},
			want: "enum Result[T, E] { Ok(T), Err(E) }",
		},
		{
			name: "struct variant",
			enumDecl: &EnumDecl{
				Name: &Ident{Name: "Color"},
				Variants: []*EnumVariant{
					{Name: &Ident{Name: "Red"}, Kind: UnitVariant},
					{
						Name: &Ident{Name: "RGB"},
						Kind: StructVariant,
						Fields: []*EnumField{
							{Name: &Ident{Name: "r"}, Type: &TypeExpr{Text: "int"}},
							{Name: &Ident{Name: "g"}, Type: &TypeExpr{Text: "int"}},
							{Name: &Ident{Name: "b"}, Type: &TypeExpr{Text: "int"}},
						},
					},
				},
			},
			want: "enum Color { Red, RGB { r: int, g: int, b: int } }",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.enumDecl.String()
			if got != tt.want {
				t.Errorf("EnumDecl.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEnumDecl_Positions(t *testing.T) {
	enumDecl := &EnumDecl{
		Enum:   token.Pos(1),
		Name:   &Ident{NamePos: token.Pos(6), Name: "Color"},
		LBrace: token.Pos(12),
		RBrace: token.Pos(50),
	}

	if got := enumDecl.Pos(); got != token.Pos(1) {
		t.Errorf("EnumDecl.Pos() = %v, want %v", got, token.Pos(1))
	}

	if got := enumDecl.End(); got != token.Pos(51) {
		t.Errorf("EnumDecl.End() = %v, want %v", got, token.Pos(51))
	}
}

func TestEnumVariant_String(t *testing.T) {
	tests := []struct {
		name    string
		variant *EnumVariant
		want    string
	}{
		{
			name:    "unit variant",
			variant: &EnumVariant{Name: &Ident{Name: "Red"}, Kind: UnitVariant},
			want:    "Red",
		},
		{
			name: "tuple variant",
			variant: &EnumVariant{
				Name: &Ident{Name: "Ok"},
				Kind: TupleVariant,
				Fields: []*EnumField{
					{Type: &TypeExpr{Text: "T"}},
				},
			},
			want: "Ok(T)",
		},
		{
			name: "struct variant",
			variant: &EnumVariant{
				Name: &Ident{Name: "Point"},
				Kind: StructVariant,
				Fields: []*EnumField{
					{Name: &Ident{Name: "x"}, Type: &TypeExpr{Text: "int"}},
					{Name: &Ident{Name: "y"}, Type: &TypeExpr{Text: "int"}},
				},
			},
			want: "Point { x: int, y: int }",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.variant.String()
			if got != tt.want {
				t.Errorf("EnumVariant.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEnumFieldKind_String(t *testing.T) {
	tests := []struct {
		kind EnumFieldKind
		want string
	}{
		{UnitVariant, "unit"},
		{TupleVariant, "tuple"},
		{StructVariant, "struct"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := tt.kind.String()
			if got != tt.want {
				t.Errorf("EnumFieldKind.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEnumDecl_ImplementsDecl(t *testing.T) {
	// Compile-time check that EnumDecl implements Decl interface
	var _ Decl = (*EnumDecl)(nil)
}
