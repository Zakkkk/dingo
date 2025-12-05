package ast

import (
	"go/ast"
	"go/token"
	"testing"
)

func TestNewBadExpr(t *testing.T) {
	bad := NewBadExpr(10, 20)

	if bad == nil {
		t.Fatal("NewBadExpr returned nil")
	}

	if bad.From != 10 {
		t.Errorf("From = %d, want 10", bad.From)
	}
	if bad.To != 20 {
		t.Errorf("To = %d, want 20", bad.To)
	}

	// Verify it implements ast.Expr
	var _ ast.Expr = bad
}

func TestNewBadStmt(t *testing.T) {
	bad := NewBadStmt(30, 40)

	if bad == nil {
		t.Fatal("NewBadStmt returned nil")
	}

	if bad.From != 30 {
		t.Errorf("From = %d, want 30", bad.From)
	}
	if bad.To != 40 {
		t.Errorf("To = %d, want 40", bad.To)
	}

	// Verify it implements ast.Stmt
	var _ ast.Stmt = bad
}

func TestNewBadDecl(t *testing.T) {
	bad := NewBadDecl(50, 60)

	if bad == nil {
		t.Fatal("NewBadDecl returned nil")
	}

	if bad.From != 50 {
		t.Errorf("From = %d, want 50", bad.From)
	}
	if bad.To != 60 {
		t.Errorf("To = %d, want 60", bad.To)
	}

	// Verify it implements ast.Decl
	var _ ast.Decl = bad
}

func TestIsBadNode(t *testing.T) {
	tests := []struct {
		name string
		node ast.Node
		want bool
	}{
		{
			name: "nil node",
			node: nil,
			want: false,
		},
		{
			name: "BadExpr",
			node: &ast.BadExpr{From: 0, To: 10},
			want: true,
		},
		{
			name: "BadStmt",
			node: &ast.BadStmt{From: 0, To: 10},
			want: true,
		},
		{
			name: "BadDecl",
			node: &ast.BadDecl{From: 0, To: 10},
			want: true,
		},
		{
			name: "Ident (not bad)",
			node: &ast.Ident{Name: "x"},
			want: false,
		},
		{
			name: "ExprStmt (not bad)",
			node: &ast.ExprStmt{X: &ast.Ident{Name: "x"}},
			want: false,
		},
		{
			name: "FuncDecl (not bad)",
			node: &ast.FuncDecl{Name: &ast.Ident{Name: "f"}},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsBadNode(tt.node); got != tt.want {
				t.Errorf("IsBadNode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHasBadNodes(t *testing.T) {
	tests := []struct {
		name string
		node ast.Node
		want bool
	}{
		{
			name: "nil node",
			node: nil,
			want: false,
		},
		{
			name: "clean expression",
			node: &ast.BinaryExpr{
				X:  &ast.Ident{Name: "x"},
				Op: token.ADD,
				Y:  &ast.Ident{Name: "y"},
			},
			want: false,
		},
		{
			name: "expression with BadExpr",
			node: &ast.BinaryExpr{
				X:  &ast.BadExpr{From: 0, To: 5},
				Op: token.ADD,
				Y:  &ast.Ident{Name: "y"},
			},
			want: true,
		},
		{
			name: "block with BadStmt",
			node: &ast.BlockStmt{
				List: []ast.Stmt{
					&ast.ExprStmt{X: &ast.Ident{Name: "x"}},
					&ast.BadStmt{From: 10, To: 20},
					&ast.ExprStmt{X: &ast.Ident{Name: "y"}},
				},
			},
			want: true,
		},
		{
			name: "file with BadDecl",
			node: &ast.File{
				Name: &ast.Ident{Name: "test"},
				Decls: []ast.Decl{
					&ast.BadDecl{From: 30, To: 40},
				},
			},
			want: true,
		},
		{
			name: "clean file",
			node: &ast.File{
				Name: &ast.Ident{Name: "test"},
				Decls: []ast.Decl{
					&ast.FuncDecl{
						Name: &ast.Ident{Name: "main"},
						Type: &ast.FuncType{Params: &ast.FieldList{}},
						Body: &ast.BlockStmt{},
					},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasBadNodes(tt.node); got != tt.want {
				t.Errorf("HasBadNodes() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCollectBadNodes(t *testing.T) {
	tests := []struct {
		name      string
		node      ast.Node
		wantCount int
	}{
		{
			name:      "nil node",
			node:      nil,
			wantCount: 0,
		},
		{
			name: "no bad nodes",
			node: &ast.BinaryExpr{
				X:  &ast.Ident{Name: "x"},
				Op: token.ADD,
				Y:  &ast.Ident{Name: "y"},
			},
			wantCount: 0,
		},
		{
			name: "one BadExpr",
			node: &ast.BinaryExpr{
				X:  &ast.BadExpr{From: 0, To: 5},
				Op: token.ADD,
				Y:  &ast.Ident{Name: "y"},
			},
			wantCount: 1,
		},
		{
			name: "multiple bad nodes",
			node: &ast.BlockStmt{
				List: []ast.Stmt{
					&ast.BadStmt{From: 0, To: 5},
					&ast.ExprStmt{X: &ast.BadExpr{From: 10, To: 15}},
					&ast.BadStmt{From: 20, To: 25},
				},
			},
			wantCount: 3,
		},
		{
			name: "nested bad nodes",
			node: &ast.File{
				Name: &ast.Ident{Name: "test"},
				Decls: []ast.Decl{
					&ast.FuncDecl{
						Name: &ast.Ident{Name: "f"},
						Type: &ast.FuncType{Params: &ast.FieldList{}},
						Body: &ast.BlockStmt{
							List: []ast.Stmt{
								&ast.BadStmt{From: 0, To: 5},
								&ast.ExprStmt{X: &ast.BadExpr{From: 10, To: 15}},
							},
						},
					},
					&ast.BadDecl{From: 30, To: 35},
				},
			},
			wantCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CollectBadNodes(tt.node)
			if len(got) != tt.wantCount {
				t.Errorf("CollectBadNodes() count = %d, want %d", len(got), tt.wantCount)
			}

			// Verify all collected nodes are actually bad nodes
			for i, node := range got {
				if !IsBadNode(node) {
					t.Errorf("CollectBadNodes()[%d] is not a bad node: %T", i, node)
				}
			}
		})
	}
}

func TestBadNodePositions(t *testing.T) {
	// Test that Bad nodes track positions correctly
	badExpr := NewBadExpr(100, 200)
	if badExpr.Pos() != 100 {
		t.Errorf("BadExpr.Pos() = %d, want 100", badExpr.Pos())
	}
	if badExpr.End() != 200 {
		t.Errorf("BadExpr.End() = %d, want 200", badExpr.End())
	}

	badStmt := NewBadStmt(300, 400)
	if badStmt.Pos() != 300 {
		t.Errorf("BadStmt.Pos() = %d, want 300", badStmt.Pos())
	}
	if badStmt.End() != 400 {
		t.Errorf("BadStmt.End() = %d, want 400", badStmt.End())
	}

	badDecl := NewBadDecl(500, 600)
	if badDecl.Pos() != 500 {
		t.Errorf("BadDecl.Pos() = %d, want 500", badDecl.Pos())
	}
	if badDecl.End() != 600 {
		t.Errorf("BadDecl.End() = %d, want 600", badDecl.End())
	}
}
