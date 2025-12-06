// Package typechecker provides AST rewriting for type inference.
// The TypeRewriter replaces interface{} placeholders with actual types
// after go/types has analyzed the code.
package typechecker

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"
)

// TypeRewriter rewrites interface{} placeholders with actual types.
// It uses the type information from go/types to infer the correct types.
type TypeRewriter struct {
	fset    *token.FileSet
	info    *types.Info
	file    *ast.File
	changed bool
}

// NewTypeRewriter creates a new TypeRewriter.
func NewTypeRewriter(fset *token.FileSet, file *ast.File, info *types.Info) *TypeRewriter {
	return &TypeRewriter{
		fset: fset,
		info: info,
		file: file,
	}
}

// Rewrite rewrites the AST to replace interface{} with actual types.
// Returns true if any changes were made.
func (r *TypeRewriter) Rewrite() bool {
	r.changed = false
	ast.Inspect(r.file, r.visit)
	return r.changed
}

// visit is the AST visitor that looks for interface{} patterns to rewrite.
func (r *TypeRewriter) visit(n ast.Node) bool {
	if n == nil {
		return false
	}

	switch node := n.(type) {
	case *ast.CallExpr:
		// Look for IIFE patterns: func() interface{} { ... }()
		if funcLit, ok := node.Fun.(*ast.FuncLit); ok {
			r.rewriteFuncLit(funcLit, node)
		}
	}

	return true
}

// rewriteFuncLit attempts to rewrite an IIFE's return type from interface{} to the actual type.
func (r *TypeRewriter) rewriteFuncLit(fn *ast.FuncLit, call *ast.CallExpr) {
	// Check if return type is interface{}
	if fn.Type.Results == nil || len(fn.Type.Results.List) != 1 {
		return
	}

	result := fn.Type.Results.List[0]
	if !r.isInterfaceType(result.Type) {
		return
	}

	// Look for the return statement to infer the actual type
	var returnType types.Type
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if ret, ok := n.(*ast.ReturnStmt); ok {
			if len(ret.Results) == 1 {
				// Check if returning nil
				if ident, ok := ret.Results[0].(*ast.Ident); ok && ident.Name == "nil" {
					return true // Skip nil returns
				}

				// Check if returning &tmp (pointer to temp)
				if unary, ok := ret.Results[0].(*ast.UnaryExpr); ok {
					if unary.Op == token.AND {
						// This is &tmp, get the type of tmp
						if tv, ok := r.info.Types[unary.X]; ok {
							returnType = types.NewPointer(tv.Type)
						}
					}
				} else {
					// Direct return
					if tv, ok := r.info.Types[ret.Results[0]]; ok {
						returnType = tv.Type
					}
				}
			}
		}
		return true
	})

	if returnType == nil {
		return
	}

	// Replace interface{} with the actual type
	newType := r.typeToExpr(returnType)
	if newType != nil {
		result.Type = newType
		r.changed = true
	}
}

// isInterfaceType checks if an expression represents interface{} (or any).
func (r *TypeRewriter) isInterfaceType(expr ast.Expr) bool {
	switch t := expr.(type) {
	case *ast.InterfaceType:
		// interface{}
		return t.Methods == nil || len(t.Methods.List) == 0
	case *ast.Ident:
		// "any" is an alias for interface{} in Go 1.18+
		return t.Name == "any"
	}
	return false
}

// typeToExpr converts a types.Type to an ast.Expr.
func (r *TypeRewriter) typeToExpr(t types.Type) ast.Expr {
	switch typ := t.(type) {
	case *types.Basic:
		return &ast.Ident{Name: typ.Name()}

	case *types.Pointer:
		elem := r.typeToExpr(typ.Elem())
		if elem != nil {
			return &ast.StarExpr{X: elem}
		}

	case *types.Named:
		// For named types, use the type name
		obj := typ.Obj()
		if obj != nil {
			name := obj.Name()
			pkg := obj.Pkg()
			if pkg != nil && pkg.Name() != "" && pkg.Name() != "main" {
				// Qualified type: pkg.Name
				return &ast.SelectorExpr{
					X:   &ast.Ident{Name: pkg.Name()},
					Sel: &ast.Ident{Name: name},
				}
			}
			return &ast.Ident{Name: name}
		}

	case *types.Slice:
		elem := r.typeToExpr(typ.Elem())
		if elem != nil {
			return &ast.ArrayType{Elt: elem}
		}

	case *types.Map:
		key := r.typeToExpr(typ.Key())
		val := r.typeToExpr(typ.Elem())
		if key != nil && val != nil {
			return &ast.MapType{Key: key, Value: val}
		}

	case *types.Chan:
		elem := r.typeToExpr(typ.Elem())
		if elem != nil {
			dir := ast.SEND | ast.RECV
			switch typ.Dir() {
			case types.SendOnly:
				dir = ast.SEND
			case types.RecvOnly:
				dir = ast.RECV
			}
			return &ast.ChanType{Dir: dir, Value: elem}
		}

	case *types.Interface:
		// Keep interface{} as is
		return &ast.InterfaceType{Methods: &ast.FieldList{}}
	}

	return nil
}

// RewriteSource takes source code and rewrites interface{} placeholders with actual types.
// Returns the modified source and whether any changes were made.
func RewriteSource(fset *token.FileSet, file *ast.File) (bool, error) {
	// First, run the type checker
	checker, err := New(fset, file, "main")
	if err != nil {
		return false, err
	}

	// Then rewrite the AST
	rewriter := NewTypeRewriter(fset, file, checker.Info())
	changed := rewriter.Rewrite()

	return changed, nil
}

// InferSafeNavType infers the type for a safe navigation expression.
// Given "user?.address?.city", it returns the type of city (or *city if nullable).
func InferSafeNavType(checker *Checker, baseExpr ast.Expr, fields []string) types.Type {
	if checker == nil || baseExpr == nil || len(fields) == 0 {
		return nil
	}

	// Get the type of the base expression
	baseType := checker.TypeOf(baseExpr)
	if baseType == nil {
		return nil
	}

	// Navigate through the fields
	currentType := baseType
	for _, field := range fields {
		fieldType := FieldType(currentType, field)
		if fieldType == nil {
			// Try as method
			if sig := MethodType(currentType, field); sig != nil {
				fieldType = ResultType(sig)
			}
		}
		if fieldType == nil {
			return nil
		}
		currentType = fieldType
	}

	return currentType
}

// InferNullCoalesceType infers the type for a null coalescing expression.
// Given "a ?? b", it returns the type based on the right operand (the default value).
func InferNullCoalesceType(checker *Checker, rightExpr ast.Expr) types.Type {
	if checker == nil || rightExpr == nil {
		return nil
	}

	return checker.TypeOf(rightExpr)
}

// TypeToString returns a Go source representation of a type.
func TypeToString(t types.Type) string {
	if t == nil {
		return "interface{}"
	}
	return types.TypeString(t, func(pkg *types.Package) string {
		if pkg == nil {
			return ""
		}
		return pkg.Name()
	})
}

// IsSafeNavIIFE checks if an expression is a safe navigation IIFE pattern.
// Pattern: func() interface{} { if x != nil { ... } }()
func IsSafeNavIIFE(expr ast.Expr) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok || len(call.Args) != 0 {
		return false
	}

	fn, ok := call.Fun.(*ast.FuncLit)
	if !ok || fn.Body == nil {
		return false
	}

	// Check for interface{} return type
	if fn.Type.Results == nil || len(fn.Type.Results.List) != 1 {
		return false
	}

	// Check body starts with "if x != nil"
	if len(fn.Body.List) < 1 {
		return false
	}

	ifStmt, ok := fn.Body.List[0].(*ast.IfStmt)
	if !ok {
		return false
	}

	// Check condition is "x != nil"
	binExpr, ok := ifStmt.Cond.(*ast.BinaryExpr)
	if !ok || binExpr.Op != token.NEQ {
		return false
	}

	nilIdent, ok := binExpr.Y.(*ast.Ident)
	return ok && nilIdent.Name == "nil"
}

// IsNullCoalesceIIFE checks if an expression is a null coalescing IIFE pattern.
// Pattern: func() interface{} { if a != nil { return a } return b }()
func IsNullCoalesceIIFE(expr ast.Expr) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok || len(call.Args) != 0 {
		return false
	}

	fn, ok := call.Fun.(*ast.FuncLit)
	if !ok || fn.Body == nil {
		return false
	}

	// Check for interface{} return type
	if fn.Type.Results == nil || len(fn.Type.Results.List) != 1 {
		return false
	}

	// Should have if statement followed by return
	if len(fn.Body.List) < 2 {
		return false
	}

	// First statement is if
	if _, ok := fn.Body.List[0].(*ast.IfStmt); !ok {
		return false
	}

	// Last statement is return (the default value)
	if _, ok := fn.Body.List[len(fn.Body.List)-1].(*ast.ReturnStmt); !ok {
		return false
	}

	return true
}

// ExtractSafeNavChain extracts the base variable and field chain from a safe nav IIFE.
func ExtractSafeNavChain(expr ast.Expr) (base string, fields []string) {
	if !IsSafeNavIIFE(expr) {
		return "", nil
	}

	call := expr.(*ast.CallExpr)
	fn := call.Fun.(*ast.FuncLit)
	ifStmt := fn.Body.List[0].(*ast.IfStmt)
	binExpr := ifStmt.Cond.(*ast.BinaryExpr)

	// Get base variable name
	if ident, ok := binExpr.X.(*ast.Ident); ok {
		base = ident.Name
	}

	// Extract field chain from the if body
	// This is a simplified extraction - full implementation would walk the AST
	ast.Inspect(ifStmt.Body, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok {
			fields = append(fields, sel.Sel.Name)
		}
		return true
	})

	// Remove duplicates and reverse (since we found them inside-out)
	seen := make(map[string]bool)
	var uniqueFields []string
	for i := len(fields) - 1; i >= 0; i-- {
		f := fields[i]
		// Skip "tmp" variables
		if strings.HasPrefix(f, "tmp") {
			continue
		}
		if !seen[f] {
			seen[f] = true
			uniqueFields = append(uniqueFields, f)
		}
	}

	return base, uniqueFields
}
