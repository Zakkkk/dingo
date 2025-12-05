package transformer

import (
	"fmt"
	"go/ast"
	"go/token"

	dingoast "github.com/MadAppGang/dingo/pkg/ast"
)

// NullOpsTransformer transforms null coalescing and safe navigation operators to Go code
type NullOpsTransformer struct {
	fset     *token.FileSet
	tmpCount int
}

// NewNullOpsTransformer creates a new null operations transformer
func NewNullOpsTransformer(fset *token.FileSet) *NullOpsTransformer {
	return &NullOpsTransformer{
		fset:     fset,
		tmpCount: 1,
	}
}

// TransformNullCoalesce transforms a NullCoalesceExpr to Go AST
// Implements: a ?? b → var result T; if a != nil { result = a } else { result = b }
// For Option types: if a.IsSome() { result = a.Unwrap() } else { result = b }
func (t *NullOpsTransformer) TransformNullCoalesce(expr *dingoast.NullCoalesceExpr) []ast.Stmt {
	// Generate temporary variable name (no-number-first pattern)
	var resultVar string
	if t.tmpCount == 1 {
		resultVar = "coalesce"
	} else {
		resultVar = fmt.Sprintf("coalesce%d", t.tmpCount-1)
	}
	t.tmpCount++

	// Strategy: Default-first pattern
	// 1. result := defaultValue (right operand)
	// 2. if condition { result = actualValue (left operand) }

	var stmts []ast.Stmt

	// Initialize with default value (right operand)
	// var result = <right>
	assignStmt := &ast.AssignStmt{
		Lhs: []ast.Expr{
			&ast.Ident{Name: resultVar},
		},
		Tok: token.DEFINE,
		Rhs: []ast.Expr{
			expr.Right,
		},
	}
	stmts = append(stmts, assignStmt)

	// Build the condition check based on left operand type
	// For now, generate a generic check - type-specific optimization happens in later phases
	// Pattern: if val := <left>; val.IsSome() { result = val.Unwrap() } (for Option)
	// Pattern: if val := <left>; val != nil { result = *val } (for pointers)

	// Use a short variable declaration in the if condition
	valIdent := &ast.Ident{Name: "val"}

	// Build if statement
	// if val := <left>; <condition> { result = <unwrapped-val> }
	ifStmt := &ast.IfStmt{
		Init: &ast.AssignStmt{
			Lhs: []ast.Expr{valIdent},
			Tok: token.DEFINE,
			Rhs: []ast.Expr{expr.Left},
		},
		// Condition: val.IsSome() for Option types, val != nil for pointers
		// We generate a placeholder that will be resolved by type checker
		Cond: &ast.CallExpr{
			Fun: &ast.SelectorExpr{
				X:   valIdent,
				Sel: &ast.Ident{Name: "IsSome"},
			},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.AssignStmt{
					Lhs: []ast.Expr{
						&ast.Ident{Name: resultVar},
					},
					Tok: token.ASSIGN,
					// Unwrap the value
					Rhs: []ast.Expr{
						&ast.CallExpr{
							Fun: &ast.SelectorExpr{
								X:   valIdent,
								Sel: &ast.Ident{Name: "Unwrap"},
							},
						},
					},
				},
			},
		},
	}
	stmts = append(stmts, ifStmt)

	return stmts
}

// TransformSafeNav transforms a SafeNavExpr to Go AST
// Implements: a?.field → var result T; if a != nil { result = a.field }
// For Option types: var result Option[T]; if a.IsSome() { result = Some(a.Unwrap().field) } else { result = None() }
func (t *NullOpsTransformer) TransformSafeNav(expr *dingoast.SafeNavExpr) []ast.Stmt {
	// Generate temporary variable name
	var resultVar string
	if t.tmpCount == 1 {
		resultVar = "safeNav"
	} else {
		resultVar = fmt.Sprintf("safeNav%d", t.tmpCount-1)
	}
	t.tmpCount++

	var stmts []ast.Stmt

	// For Option types: return None if base is None
	// For pointer types: return zero value if base is nil

	// Pattern for Option types:
	// var result Option[T]
	// if <base>.IsSome() {
	//     result = Some(<base>.Unwrap().<field>)
	// } else {
	//     result = None()
	// }

	// Variable declaration (type will be inferred)
	varDecl := &ast.DeclStmt{
		Decl: &ast.GenDecl{
			Tok: token.VAR,
			Specs: []ast.Spec{
				&ast.ValueSpec{
					Names: []*ast.Ident{
						{Name: resultVar},
					},
					// Type will be inferred from assignment
				},
			},
		},
	}
	stmts = append(stmts, varDecl)

	// Build if statement
	// if <base>.IsSome() { result = Some(<base>.Unwrap().<field>) } else { result = None() }
	ifStmt := &ast.IfStmt{
		Cond: &ast.CallExpr{
			Fun: &ast.SelectorExpr{
				X:   expr.X,
				Sel: &ast.Ident{Name: "IsSome"},
			},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.AssignStmt{
					Lhs: []ast.Expr{
						&ast.Ident{Name: resultVar},
					},
					Tok: token.ASSIGN,
					Rhs: []ast.Expr{
						// Some(<base>.Unwrap().<field>)
						&ast.CallExpr{
							Fun: &ast.Ident{Name: "Some"},
							Args: []ast.Expr{
								&ast.SelectorExpr{
									X: &ast.CallExpr{
										Fun: &ast.SelectorExpr{
											X:   expr.X,
											Sel: &ast.Ident{Name: "Unwrap"},
										},
									},
									Sel: expr.Sel,
								},
							},
						},
					},
				},
			},
		},
		Else: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.AssignStmt{
					Lhs: []ast.Expr{
						&ast.Ident{Name: resultVar},
					},
					Tok: token.ASSIGN,
					Rhs: []ast.Expr{
						&ast.CallExpr{
							Fun: &ast.Ident{Name: "None"},
						},
					},
				},
			},
		},
	}
	stmts = append(stmts, ifStmt)

	return stmts
}

// TransformSafeNavCall transforms a SafeNavCallExpr to Go AST
// Implements: a?.method(args) → similar to SafeNavExpr but with method call
func (t *NullOpsTransformer) TransformSafeNavCall(expr *dingoast.SafeNavCallExpr) []ast.Stmt {
	// Generate temporary variable name
	var resultVar string
	if t.tmpCount == 1 {
		resultVar = "safeNav"
	} else {
		resultVar = fmt.Sprintf("safeNav%d", t.tmpCount-1)
	}
	t.tmpCount++

	var stmts []ast.Stmt

	// Pattern for Option types:
	// var result Option[T]
	// if <base>.IsSome() {
	//     result = Some(<base>.Unwrap().<method>(<args>))
	// } else {
	//     result = None()
	// }

	// Variable declaration
	varDecl := &ast.DeclStmt{
		Decl: &ast.GenDecl{
			Tok: token.VAR,
			Specs: []ast.Spec{
				&ast.ValueSpec{
					Names: []*ast.Ident{
						{Name: resultVar},
					},
				},
			},
		},
	}
	stmts = append(stmts, varDecl)

	// Build if statement
	ifStmt := &ast.IfStmt{
		Cond: &ast.CallExpr{
			Fun: &ast.SelectorExpr{
				X:   expr.X,
				Sel: &ast.Ident{Name: "IsSome"},
			},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.AssignStmt{
					Lhs: []ast.Expr{
						&ast.Ident{Name: resultVar},
					},
					Tok: token.ASSIGN,
					Rhs: []ast.Expr{
						// Some(<base>.Unwrap().<method>(<args>))
						&ast.CallExpr{
							Fun: &ast.Ident{Name: "Some"},
							Args: []ast.Expr{
								&ast.CallExpr{
									Fun: &ast.SelectorExpr{
										X: &ast.CallExpr{
											Fun: &ast.SelectorExpr{
												X:   expr.X,
												Sel: &ast.Ident{Name: "Unwrap"},
											},
										},
										Sel: expr.Fun,
									},
									Args: expr.Args,
								},
							},
						},
					},
				},
			},
		},
		Else: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.AssignStmt{
					Lhs: []ast.Expr{
						&ast.Ident{Name: resultVar},
					},
					Tok: token.ASSIGN,
					Rhs: []ast.Expr{
						&ast.CallExpr{
							Fun: &ast.Ident{Name: "None"},
						},
					},
				},
			},
		},
	}
	stmts = append(stmts, ifStmt)

	return stmts
}

// Helper functions for specific type transformations

// TransformNullCoalescePointer generates null coalesce for pointer types
func (t *NullOpsTransformer) TransformNullCoalescePointer(left, right ast.Expr) []ast.Stmt {
	var resultVar string
	if t.tmpCount == 1 {
		resultVar = "coalesce"
	} else {
		resultVar = fmt.Sprintf("coalesce%d", t.tmpCount-1)
	}
	t.tmpCount++

	var stmts []ast.Stmt

	// result := <right>
	assignStmt := &ast.AssignStmt{
		Lhs: []ast.Expr{&ast.Ident{Name: resultVar}},
		Tok: token.DEFINE,
		Rhs: []ast.Expr{right},
	}
	stmts = append(stmts, assignStmt)

	// if val := <left>; val != nil { result = *val }
	ifStmt := &ast.IfStmt{
		Init: &ast.AssignStmt{
			Lhs: []ast.Expr{&ast.Ident{Name: "val"}},
			Tok: token.DEFINE,
			Rhs: []ast.Expr{left},
		},
		Cond: &ast.BinaryExpr{
			X:  &ast.Ident{Name: "val"},
			Op: token.NEQ,
			Y:  &ast.Ident{Name: "nil"},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.AssignStmt{
					Lhs: []ast.Expr{&ast.Ident{Name: resultVar}},
					Tok: token.ASSIGN,
					Rhs: []ast.Expr{
						&ast.StarExpr{X: &ast.Ident{Name: "val"}},
					},
				},
			},
		},
	}
	stmts = append(stmts, ifStmt)

	return stmts
}

// TransformNullCoalesceOption generates null coalesce for Option types
func (t *NullOpsTransformer) TransformNullCoalesceOption(left, right ast.Expr) []ast.Stmt {
	var resultVar string
	if t.tmpCount == 1 {
		resultVar = "coalesce"
	} else {
		resultVar = fmt.Sprintf("coalesce%d", t.tmpCount-1)
	}
	t.tmpCount++

	var stmts []ast.Stmt

	// result := <right>
	assignStmt := &ast.AssignStmt{
		Lhs: []ast.Expr{&ast.Ident{Name: resultVar}},
		Tok: token.DEFINE,
		Rhs: []ast.Expr{right},
	}
	stmts = append(stmts, assignStmt)

	// if val := <left>; val.IsSome() { result = val.Unwrap() }
	ifStmt := &ast.IfStmt{
		Init: &ast.AssignStmt{
			Lhs: []ast.Expr{&ast.Ident{Name: "val"}},
			Tok: token.DEFINE,
			Rhs: []ast.Expr{left},
		},
		Cond: &ast.CallExpr{
			Fun: &ast.SelectorExpr{
				X:   &ast.Ident{Name: "val"},
				Sel: &ast.Ident{Name: "IsSome"},
			},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.AssignStmt{
					Lhs: []ast.Expr{&ast.Ident{Name: resultVar}},
					Tok: token.ASSIGN,
					Rhs: []ast.Expr{
						&ast.CallExpr{
							Fun: &ast.SelectorExpr{
								X:   &ast.Ident{Name: "val"},
								Sel: &ast.Ident{Name: "Unwrap"},
							},
						},
					},
				},
			},
		},
	}
	stmts = append(stmts, ifStmt)

	return stmts
}

// TransformSafeNavPointer generates safe navigation for pointer types
func (t *NullOpsTransformer) TransformSafeNavPointer(base ast.Expr, field *ast.Ident) []ast.Stmt {
	var resultVar string
	if t.tmpCount == 1 {
		resultVar = "safeNav"
	} else {
		resultVar = fmt.Sprintf("safeNav%d", t.tmpCount-1)
	}
	t.tmpCount++

	var stmts []ast.Stmt

	// var result T (zero value)
	varDecl := &ast.DeclStmt{
		Decl: &ast.GenDecl{
			Tok: token.VAR,
			Specs: []ast.Spec{
				&ast.ValueSpec{
					Names: []*ast.Ident{{Name: resultVar}},
				},
			},
		},
	}
	stmts = append(stmts, varDecl)

	// if <base> != nil { result = <base>.<field> }
	ifStmt := &ast.IfStmt{
		Cond: &ast.BinaryExpr{
			X:  base,
			Op: token.NEQ,
			Y:  &ast.Ident{Name: "nil"},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.AssignStmt{
					Lhs: []ast.Expr{&ast.Ident{Name: resultVar}},
					Tok: token.ASSIGN,
					Rhs: []ast.Expr{
						&ast.SelectorExpr{
							X:   base,
							Sel: field,
						},
					},
				},
			},
		},
	}
	stmts = append(stmts, ifStmt)

	return stmts
}
