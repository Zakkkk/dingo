// Package transpiler provides enum variant identifier transformation.
// This transformer converts bare enum variant references to constructor calls.
package transpiler

import (
	"go/ast"
	"go/token"
)

// EnumVariantTransformer transforms bare enum variant identifiers to constructor calls.
// Example: `return Active` → `return NewStatusActive()` when Active is a variant of Status enum.
//
// This transformer ONLY operates in value contexts, NOT type contexts.
// Type contexts (declarations, type assertions, etc.) are preserved.
type EnumVariantTransformer struct {
	fset     *token.FileSet
	file     *ast.File
	registry map[string]string // Maps variant name → enum name
}

// NewEnumVariantTransformer creates a new transformer.
// The registry maps variant names (e.g., "Active") to enum names (e.g., "Status").
func NewEnumVariantTransformer(fset *token.FileSet, file *ast.File, registry map[string]string) *EnumVariantTransformer {
	return &EnumVariantTransformer{
		fset:     fset,
		file:     file,
		registry: registry,
	}
}

// Transform walks the AST and replaces bare enum variant identifiers with constructor calls.
// It handles:
// - Return statements: return Active → return NewStatusActive()
// - Call arguments: foo(Active) → foo(NewStatusActive())
// - Assignments: x = Active → x = NewStatusActive()
// - Struct literal values: Config{Status: Active} → Config{Status: NewStatusActive()}
// - Binary expressions: x == Active → x == NewStatusActive()
func (t *EnumVariantTransformer) Transform() {
	if len(t.registry) == 0 {
		return
	}

	t.transformReturns()
	t.transformCallArgs()
	t.transformAssignments()
	t.transformStructLiterals()
	t.transformBinaryExprs()
	t.transformVarInits()
}

// transformReturns handles: return Active
func (t *EnumVariantTransformer) transformReturns() {
	ast.Inspect(t.file, func(n ast.Node) bool {
		ret, ok := n.(*ast.ReturnStmt)
		if !ok {
			return true
		}

		for i, result := range ret.Results {
			if call := t.maybeTransformIdent(result); call != nil {
				ret.Results[i] = call
			}
		}
		return true
	})
}

// transformCallArgs handles: foo(Active)
func (t *EnumVariantTransformer) transformCallArgs() {
	ast.Inspect(t.file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		for i, arg := range call.Args {
			if newCall := t.maybeTransformIdent(arg); newCall != nil {
				call.Args[i] = newCall
			}
		}
		return true
	})
}

// transformAssignments handles: x = Active
func (t *EnumVariantTransformer) transformAssignments() {
	ast.Inspect(t.file, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}

		for i, rhs := range assign.Rhs {
			if call := t.maybeTransformIdent(rhs); call != nil {
				assign.Rhs[i] = call
			}
		}
		return true
	})
}

// transformStructLiterals handles: Config{Status: Active}
func (t *EnumVariantTransformer) transformStructLiterals() {
	ast.Inspect(t.file, func(n ast.Node) bool {
		compLit, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}

		for _, elt := range compLit.Elts {
			// Handle keyed elements: {Status: Active}
			if kv, ok := elt.(*ast.KeyValueExpr); ok {
				if call := t.maybeTransformIdent(kv.Value); call != nil {
					kv.Value = call
				}
			}
			// Unkeyed elements in struct literals are typically positional values
			// but those would be caught by maybeTransformIdent on the slice element
		}
		return true
	})
}

// transformBinaryExprs handles: x == Active, Active != y
func (t *EnumVariantTransformer) transformBinaryExprs() {
	ast.Inspect(t.file, func(n ast.Node) bool {
		binExpr, ok := n.(*ast.BinaryExpr)
		if !ok {
			return true
		}

		// Only comparison operators make sense for enum values
		switch binExpr.Op {
		case token.EQL, token.NEQ:
			if call := t.maybeTransformIdent(binExpr.X); call != nil {
				binExpr.X = call
			}
			if call := t.maybeTransformIdent(binExpr.Y); call != nil {
				binExpr.Y = call
			}
		}
		return true
	})
}

// transformVarInits handles: var x Status = Active
func (t *EnumVariantTransformer) transformVarInits() {
	ast.Inspect(t.file, func(n ast.Node) bool {
		genDecl, ok := n.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.VAR {
			return true
		}

		for _, spec := range genDecl.Specs {
			valueSpec, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}

			for i, val := range valueSpec.Values {
				if call := t.maybeTransformIdent(val); call != nil {
					valueSpec.Values[i] = call
				}
			}
		}
		return true
	})
}

// maybeTransformIdent checks if an expression is a bare enum variant identifier
// and returns a constructor call if so. Returns nil if no transformation needed.
func (t *EnumVariantTransformer) maybeTransformIdent(expr ast.Expr) *ast.CallExpr {
	ident, ok := expr.(*ast.Ident)
	if !ok {
		return nil
	}

	enumName, isVariant := t.registry[ident.Name]
	if !isVariant {
		return nil
	}

	// Create NewEnumVariant() call
	return &ast.CallExpr{
		Fun: &ast.Ident{
			NamePos: ident.NamePos,
			Name:    "New" + enumName + ident.Name,
		},
		Lparen: ident.End(),
		Args:   []ast.Expr{},
		Rparen: ident.End(),
	}
}
