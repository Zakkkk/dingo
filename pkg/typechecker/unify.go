package typechecker

import (
	"go/ast"
	"go/token"
	"go/types"
)

// TypeUnifier performs type parameter inference and signature instantiation.
// This is a standalone implementation separate from lambda_inference.go's
// matchTypeToParam to avoid risk of breaking existing behavior.
type TypeUnifier struct {
	fset *token.FileSet
	info *types.Info
}

// NewTypeUnifier creates a new unifier with type information context.
func NewTypeUnifier(fset *token.FileSet, info *types.Info) *TypeUnifier {
	return &TypeUnifier{fset: fset, info: info}
}

// InferTypeParams extracts type parameter bindings from a generic call.
// It examines non-lambda arguments to determine concrete types for type parameters.
//
// Example: dgo.Map(users, |u| u.Name) where users: []User
//   - Matches []User against []T -> T=User
//   - Returns {"T": User}
func (u *TypeUnifier) InferTypeParams(call *ast.CallExpr, genericSig *types.Signature) map[string]types.Type {
	if genericSig == nil || genericSig.TypeParams() == nil {
		return nil
	}

	bindings := make(map[string]types.Type)
	params := genericSig.Params()
	typeParams := genericSig.TypeParams()

	for i, arg := range call.Args {
		if i >= params.Len() {
			break
		}

		// Skip function literal arguments (we're inferring those)
		if _, ok := arg.(*ast.FuncLit); ok {
			continue
		}

		// Get concrete type of argument
		argType := u.getExprType(arg)
		if argType == nil {
			continue
		}

		// Get expected parameter type (contains type params)
		paramType := params.At(i).Type()

		// Unify to extract bindings
		u.unify(paramType, argType, typeParams, bindings)
	}

	return bindings
}

// unify matches a parameterized type against a concrete type,
// extracting type parameter bindings.
func (u *TypeUnifier) unify(
	paramType types.Type,
	concreteType types.Type,
	typeParams *types.TypeParamList,
	bindings map[string]types.Type,
) {
	// Base case: paramType is a type parameter
	if tp, ok := paramType.(*types.TypeParam); ok {
		bindings[tp.Obj().Name()] = concreteType
		return
	}

	// Get underlying type for concreteType in case it's a named type
	concrete := concreteType
	if named, ok := concreteType.(*types.Named); ok {
		concrete = named.Underlying()
	}

	// Recursive cases - use concrete (underlying if named)
	switch pt := paramType.(type) {
	case *types.Slice:
		if ct, ok := concrete.(*types.Slice); ok {
			u.unify(pt.Elem(), ct.Elem(), typeParams, bindings)
		}

	case *types.Array:
		if ct, ok := concrete.(*types.Array); ok {
			u.unify(pt.Elem(), ct.Elem(), typeParams, bindings)
		}

	case *types.Map:
		if ct, ok := concrete.(*types.Map); ok {
			u.unify(pt.Key(), ct.Key(), typeParams, bindings)
			u.unify(pt.Elem(), ct.Elem(), typeParams, bindings)
		}

	case *types.Pointer:
		if ct, ok := concrete.(*types.Pointer); ok {
			u.unify(pt.Elem(), ct.Elem(), typeParams, bindings)
		}

	case *types.Chan:
		if ct, ok := concrete.(*types.Chan); ok {
			u.unify(pt.Elem(), ct.Elem(), typeParams, bindings)
		}

	case *types.Named:
		if ct, ok := concrete.(*types.Named); ok {
			// Match type arguments of generic instantiations
			if pt.TypeArgs() != nil && ct.TypeArgs() != nil {
				n := min(pt.TypeArgs().Len(), ct.TypeArgs().Len())
				for i := 0; i < n; i++ {
					u.unify(pt.TypeArgs().At(i), ct.TypeArgs().At(i), typeParams, bindings)
				}
			}
		}

	case *types.Signature:
		if ct, ok := concreteType.(*types.Signature); ok {
			// Unify parameter types
			if pt.Params() != nil && ct.Params() != nil {
				n := min(pt.Params().Len(), ct.Params().Len())
				for i := 0; i < n; i++ {
					u.unify(pt.Params().At(i).Type(), ct.Params().At(i).Type(), typeParams, bindings)
				}
			}
			// Unify result types
			if pt.Results() != nil && ct.Results() != nil {
				n := min(pt.Results().Len(), ct.Results().Len())
				for i := 0; i < n; i++ {
					u.unify(pt.Results().At(i).Type(), ct.Results().At(i).Type(), typeParams, bindings)
				}
			}
		}
	}
}

// InstantiateSignature creates a concrete signature by substituting type parameters.
func (u *TypeUnifier) InstantiateSignature(
	genericSig *types.Signature,
	bindings map[string]types.Type,
) *types.Signature {
	if genericSig == nil || len(bindings) == 0 {
		return nil
	}

	// Substitute type parameters in each parameter
	params := genericSig.Params()
	newParams := make([]*types.Var, params.Len())
	for i := 0; i < params.Len(); i++ {
		p := params.At(i)
		newType := u.substituteTypeParams(p.Type(), bindings)
		newParams[i] = types.NewVar(p.Pos(), p.Pkg(), p.Name(), newType)
	}

	// Substitute type parameters in results
	results := genericSig.Results()
	newResults := make([]*types.Var, results.Len())
	for i := 0; i < results.Len(); i++ {
		r := results.At(i)
		newType := u.substituteTypeParams(r.Type(), bindings)
		newResults[i] = types.NewVar(r.Pos(), r.Pkg(), r.Name(), newType)
	}

	return types.NewSignatureType(
		nil, nil, nil, // No receiver, no recv type params, no type params (instantiated)
		types.NewTuple(newParams...),
		types.NewTuple(newResults...),
		genericSig.Variadic(),
	)
}

// substituteTypeParams replaces type parameters with their bound values.
func (u *TypeUnifier) substituteTypeParams(t types.Type, bindings map[string]types.Type) types.Type {
	switch typ := t.(type) {
	case *types.TypeParam:
		if bound, ok := bindings[typ.Obj().Name()]; ok {
			return bound
		}
		return t

	case *types.Slice:
		elem := u.substituteTypeParams(typ.Elem(), bindings)
		return types.NewSlice(elem)

	case *types.Array:
		elem := u.substituteTypeParams(typ.Elem(), bindings)
		return types.NewArray(elem, typ.Len())

	case *types.Map:
		key := u.substituteTypeParams(typ.Key(), bindings)
		val := u.substituteTypeParams(typ.Elem(), bindings)
		return types.NewMap(key, val)

	case *types.Pointer:
		elem := u.substituteTypeParams(typ.Elem(), bindings)
		return types.NewPointer(elem)

	case *types.Signature:
		// Substitute in function type
		params := typ.Params()
		newParams := make([]*types.Var, params.Len())
		for i := 0; i < params.Len(); i++ {
			p := params.At(i)
			newType := u.substituteTypeParams(p.Type(), bindings)
			newParams[i] = types.NewVar(p.Pos(), p.Pkg(), p.Name(), newType)
		}

		results := typ.Results()
		newResults := make([]*types.Var, results.Len())
		for i := 0; i < results.Len(); i++ {
			r := results.At(i)
			newType := u.substituteTypeParams(r.Type(), bindings)
			newResults[i] = types.NewVar(r.Pos(), r.Pkg(), r.Name(), newType)
		}

		return types.NewSignatureType(
			nil, nil, nil,
			types.NewTuple(newParams...),
			types.NewTuple(newResults...),
			typ.Variadic(),
		)

	default:
		return t
	}
}

// getExprType retrieves the type of an expression from type info.
func (u *TypeUnifier) getExprType(expr ast.Expr) types.Type {
	if tv, ok := u.info.Types[expr]; ok {
		return tv.Type
	}

	if ident, ok := expr.(*ast.Ident); ok {
		if obj := u.info.Uses[ident]; obj != nil {
			return obj.Type()
		}
		if obj := u.info.Defs[ident]; obj != nil {
			return obj.Type()
		}
	}

	return nil
}
