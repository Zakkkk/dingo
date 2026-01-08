// Package transpiler provides result/option return statement wrapping.
// This transformer automatically wraps bare return values with dgo constructors
// for functions returning Result[T,E] or Option[T] types.
package transpiler

import (
	"go/ast"
	"go/token"

	"github.com/MadAppGang/dingo/pkg/typechecker"
)

// ResultWrapperTransformer transforms return statements in Result/Option-returning functions.
// It wraps bare return values with appropriate dgo.Ok/Err/Some/None constructors.
type ResultWrapperTransformer struct {
	fset     *token.FileSet
	file     *ast.File
	analyzer *ReturnAnalyzer
}

// NewResultWrapperTransformer creates a transformer with type checking support.
func NewResultWrapperTransformer(fset *token.FileSet, file *ast.File, checker *typechecker.Checker) *ResultWrapperTransformer {
	return &ResultWrapperTransformer{
		fset:     fset,
		file:     file,
		analyzer: NewReturnAnalyzer(checker),
	}
}

// Transform walks the AST and wraps return statements in Result/Option-returning functions.
// For Result[T,E]:
//   - return value (where value is type T) → return dgo.Ok[T,E](value)
//   - return err (where err is type E) → return dgo.Err[T,E](err)
//
// For Option[T]:
//   - return value (where value is type T) → return dgo.Some[T](value)
//   - return nil → return dgo.None[T]()
//
// Already wrapped returns (with dgo.Ok/Err/Some/None) are left unchanged.
func (t *ResultWrapperTransformer) Transform() {
	ast.Inspect(t.file, func(n ast.Node) bool {
		funcDecl, ok := n.(*ast.FuncDecl)
		if !ok || funcDecl.Body == nil {
			return true
		}

		// Analyze function return type
		returnInfo := t.analyzer.AnalyzeReturnType(funcDecl)
		if returnInfo == nil {
			return true // Not a Result/Option-returning function
		}

		// Transform all return statements in this function
		t.transformFunctionReturns(funcDecl.Body, returnInfo)

		return true
	})
}

// transformFunctionReturns finds and wraps return statements in a function body.
// It does NOT traverse into nested function literals, which have their own return types.
func (t *ResultWrapperTransformer) transformFunctionReturns(body *ast.BlockStmt, returnInfo *ReturnTypeInfo) {
	ast.Inspect(body, func(n ast.Node) bool {
		// Don't traverse into nested function literals
		if _, ok := n.(*ast.FuncLit); ok {
			return false // Skip nested functions
		}

		ret, ok := n.(*ast.ReturnStmt)
		if !ok || len(ret.Results) == 0 {
			return true
		}

		// Only handle single return value (Result/Option are single-value types)
		if len(ret.Results) != 1 {
			return true
		}

		t.wrapReturnStatement(ret, returnInfo)
		return true
	})
}

// wrapReturnStatement wraps a single return expression with the appropriate constructor.
func (t *ResultWrapperTransformer) wrapReturnStatement(ret *ast.ReturnStmt, returnInfo *ReturnTypeInfo) {
	expr := ret.Results[0]

	// Determine which wrapper to use (or skip if already wrapped)
	wrapper := t.analyzer.DetermineWrapper(expr, returnInfo)
	if wrapper == WrapperSkip {
		return // Already wrapped or no wrapping needed
	}

	// Create the wrapper call
	var wrappedExpr ast.Expr
	switch returnInfo.Kind {
	case "result":
		wrappedExpr = t.createResultWrapper(expr, wrapper, returnInfo)
	case "option":
		wrappedExpr = t.createOptionWrapper(expr, wrapper, returnInfo)
	default:
		return // Unknown kind, shouldn't happen
	}

	// Replace the return expression
	ret.Results[0] = wrappedExpr
}

// createResultWrapper creates dgo.Ok[T, E](value) or dgo.Err[T](err).
// Type argument requirements differ:
//   - Ok[T, E](value T): Go infers T from value, but E is second param so Ok[X] sets T=X
//     Therefore Ok NEEDS both type arguments
//   - Err[T, E](err E): Go infers E from err, and T is first param so Err[X] sets T=X
//     Therefore Err only needs T (single type argument)
func (t *ResultWrapperTransformer) createResultWrapper(expr ast.Expr, wrapper WrapperType, returnInfo *ReturnTypeInfo) ast.Expr {
	constructorName := string(wrapper) // "Ok" or "Err"

	if wrapper == WrapperOk {
		// Ok needs both [T, E] - Go can't infer E
		return &ast.CallExpr{
			Fun: &ast.IndexListExpr{
				X: &ast.SelectorExpr{
					X:   ast.NewIdent("dgo"),
					Sel: ast.NewIdent(constructorName),
				},
				Indices: []ast.Expr{
					cloneExpr(returnInfo.TAstExpr),
					cloneExpr(returnInfo.EAstExpr),
				},
			},
			Args: []ast.Expr{expr},
		}
	}

	// Err only needs [T] - Go infers E from argument
	return &ast.CallExpr{
		Fun: &ast.IndexExpr{
			X: &ast.SelectorExpr{
				X:   ast.NewIdent("dgo"),
				Sel: ast.NewIdent(constructorName),
			},
			Index: cloneExpr(returnInfo.TAstExpr),
		},
		Args: []ast.Expr{expr},
	}
}

// createOptionWrapper creates dgo.Some(value) or dgo.None[T]().
// Type argument requirements differ:
//   - Some(value T): Go infers T from value, so no type argument needed
//   - None[T](): No argument to infer from, so T must be explicit
func (t *ResultWrapperTransformer) createOptionWrapper(expr ast.Expr, wrapper WrapperType, returnInfo *ReturnTypeInfo) ast.Expr {
	constructorName := string(wrapper) // "Some" or "None"

	if wrapper == WrapperSome {
		// Some doesn't need type argument - Go infers T from value
		return &ast.CallExpr{
			Fun: &ast.SelectorExpr{
				X:   ast.NewIdent("dgo"),
				Sel: ast.NewIdent(constructorName),
			},
			Args: []ast.Expr{expr},
		}
	}

	// None needs [T] - no argument to infer from
	return &ast.CallExpr{
		Fun: &ast.IndexExpr{
			X: &ast.SelectorExpr{
				X:   ast.NewIdent("dgo"),
				Sel: ast.NewIdent(constructorName),
			},
			Index: cloneExpr(returnInfo.TAstExpr),
		},
		Args: []ast.Expr{}, // Empty args
	}
}

// cloneExpr creates a shallow copy of an AST expression for reuse.
// This is needed because AST nodes cannot be reused directly (each node
// should appear only once in the tree).
func cloneExpr(expr ast.Expr) ast.Expr {
	if expr == nil {
		return nil
	}

	switch e := expr.(type) {
	case *ast.Ident:
		return &ast.Ident{
			NamePos: e.NamePos,
			Name:    e.Name,
			Obj:     e.Obj,
		}
	case *ast.SelectorExpr:
		return &ast.SelectorExpr{
			X:   cloneExpr(e.X),
			Sel: &ast.Ident{Name: e.Sel.Name},
		}
	case *ast.StarExpr:
		return &ast.StarExpr{
			Star: e.Star,
			X:    cloneExpr(e.X),
		}
	case *ast.ArrayType:
		return &ast.ArrayType{
			Lbrack: e.Lbrack,
			Len:    cloneExpr(e.Len),
			Elt:    cloneExpr(e.Elt),
		}
	case *ast.MapType:
		return &ast.MapType{
			Map:   e.Map,
			Key:   cloneExpr(e.Key),
			Value: cloneExpr(e.Value),
		}
	case *ast.InterfaceType:
		// Interface types can be cloned as-is (they're typically empty interfaces)
		return &ast.InterfaceType{
			Interface:  e.Interface,
			Methods:    e.Methods, // Shallow copy of FieldList
			Incomplete: e.Incomplete,
		}
	case *ast.IndexExpr:
		// Generic type with one parameter: Type[T]
		return &ast.IndexExpr{
			X:      cloneExpr(e.X),
			Lbrack: e.Lbrack,
			Index:  cloneExpr(e.Index),
			Rbrack: e.Rbrack,
		}
	case *ast.IndexListExpr:
		// Generic type with multiple parameters: Type[T, E]
		indices := make([]ast.Expr, len(e.Indices))
		for i, idx := range e.Indices {
			indices[i] = cloneExpr(idx)
		}
		return &ast.IndexListExpr{
			X:       cloneExpr(e.X),
			Lbrack:  e.Lbrack,
			Indices: indices,
			Rbrack:  e.Rbrack,
		}
	default:
		// WARN: Complex types not cloned (struct, func, chan, ~T constraint types).
		// This works because type expressions are typically part of function signatures
		// and aren't modified during AST transformation.
		// If AST modification errors occur with these types, extend cloning logic:
		//   - *ast.StructType: clone Fields
		//   - *ast.FuncType: clone Params and Results
		//   - *ast.ChanType: clone Value and Dir
		//   - *ast.UnaryExpr with ~: clone X (for generic constraints)
		return expr
	}
}
