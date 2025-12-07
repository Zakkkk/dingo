// Package typechecker provides lambda type inference from call context.
// The LambdaTypeInferrer rewrites function literal parameter and return types
// from 'any' placeholders to actual types based on the expected function type.
package typechecker

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strings"
)

// untypedToTypedName converts untyped basic type kinds to their typed equivalents.
// Returns empty string for already-typed basic types (caller should use typ.Name()).
func untypedToTypedName(kind types.BasicKind) string {
	switch kind {
	case types.UntypedBool:
		return "bool"
	case types.UntypedInt:
		return "int"
	case types.UntypedRune:
		return "rune"
	case types.UntypedFloat:
		return "float64"
	case types.UntypedComplex:
		return "complex128"
	case types.UntypedString:
		return "string"
	case types.UntypedNil:
		return "nil"
	default:
		return "" // not an untyped kind, use typ.Name()
	}
}

// LambdaTypeInferrer rewrites function literal parameter and return types
// based on the expected function type from call context.
//
// It walks the AST looking for CallExpr nodes with FuncLit arguments,
// then uses go/types to look up the expected function signature and
// rewrites the FuncLit types accordingly.
type LambdaTypeInferrer struct {
	fset    *token.FileSet
	info    *types.Info
	file    *ast.File
	changed bool
}

// NewLambdaTypeInferrer creates a new inferrer.
func NewLambdaTypeInferrer(fset *token.FileSet, file *ast.File, info *types.Info) *LambdaTypeInferrer {
	return &LambdaTypeInferrer{
		fset: fset,
		info: info,
		file: file,
	}
}

// Infer walks the AST and rewrites lambda types from call context.
// Returns true if any changes were made.
func (inf *LambdaTypeInferrer) Infer() bool {
	inf.changed = false
	ast.Inspect(inf.file, inf.visit)
	return inf.changed
}

// visit is the AST visitor that looks for call expressions with function literal arguments.
func (inf *LambdaTypeInferrer) visit(n ast.Node) bool {
	call, ok := n.(*ast.CallExpr)
	if !ok {
		return true
	}

	// Get the function signature - try instantiated version first (for generics)
	// This uses Info.Instances which has already-resolved generic type arguments
	funcType := inf.getInstantiatedSignature(call)

	// Fallback to getting type from the function definition
	if funcType == nil {
		funcType = inf.getFunctionType(call.Fun)
	}

	if funcType == nil {
		return true
	}

	// Match each argument to expected parameter type
	params := funcType.Params()
	for i, arg := range call.Args {
		funcLit, ok := arg.(*ast.FuncLit)
		if !ok {
			continue
		}

		// Get expected function type for this parameter position
		if i >= params.Len() {
			// Variadic parameter - skip for now
			// TODO: Handle variadic parameters in future
			continue
		}

		expectedType := params.At(i).Type()
		if expectedType == nil {
			continue
		}

		underlying := expectedType.Underlying()
		if underlying == nil {
			continue
		}

		expectedSig, ok := underlying.(*types.Signature)
		if !ok {
			// Parameter is not a function type
			continue
		}


		// Rewrite the function literal's types
		if inf.rewriteFuncLit(funcLit, expectedSig) {
			inf.changed = true
		}
	}

	return true
}

// getFunctionType extracts the function signature from a call expression's Fun field.
// Handles direct function calls, method calls, and selector expressions.
func (inf *LambdaTypeInferrer) getFunctionType(fun ast.Expr) *types.Signature {
	// First, try to get the instantiated type from Types map
	// This handles both explicit instantiation Map[T, R](...) and inferred instantiation Map(...)
	if tv, ok := inf.info.Types[fun]; ok {
		if sig, ok := tv.Type.(*types.Signature); ok {
			return sig
		}
	}

	switch fn := fun.(type) {
	case *ast.Ident:
		// Direct function call: Func1(...)
		if obj := inf.info.Uses[fn]; obj != nil {
			if sig, ok := obj.Type().(*types.Signature); ok {
				return sig
			}
		}

	case *ast.SelectorExpr:
		// Method call or package-qualified function: obj.Method(...) or pkg.Func(...)
		// First try as method selection
		if sel := inf.info.Selections[fn]; sel != nil {
			if sig, ok := sel.Type().(*types.Signature); ok {
				return sig
			}
		}
		// Then try as package-qualified function
		if obj := inf.info.Uses[fn.Sel]; obj != nil {
			if sig, ok := obj.Type().(*types.Signature); ok {
				return sig
			}
		}

	case *ast.IndexExpr, *ast.IndexListExpr:
		// Generic function instantiation with explicit type params already handled above
		// This case is kept for clarity but will be caught by Types map check
		break
	}

	return nil
}

// getInstantiatedSignature extracts the instantiated signature for a generic function call.
// This is the primary method for resolving generic function types, as it uses the
// already-instantiated signature from go/types' type inference.
func (inf *LambdaTypeInferrer) getInstantiatedSignature(call *ast.CallExpr) *types.Signature {
	// Extract the function identifier from various call forms
	var id *ast.Ident
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		// Direct call: Filter(...)
		id = fn
	case *ast.SelectorExpr:
		// Package-qualified or method call: pkg.Filter(...) or obj.Method(...)
		id = fn.Sel
	case *ast.IndexExpr:
		// Explicit single type arg: Filter[User](...)
		switch x := fn.X.(type) {
		case *ast.Ident:
			id = x
		case *ast.SelectorExpr:
			id = x.Sel
		}
	case *ast.IndexListExpr:
		// Explicit multiple type args: Map[User, string](...)
		switch x := fn.X.(type) {
		case *ast.Ident:
			id = x
		case *ast.SelectorExpr:
			id = x.Sel
		}
	}

	if id == nil {
		return nil
	}

	// Check Info.Instances for instantiated signature
	// This map is populated by go/types when it infers generic type arguments
	if instance, ok := inf.info.Instances[id]; ok {
		if sig, ok := instance.Type.(*types.Signature); ok {
			return sig
		}
	}

	// Fallback: try Info.Types[call.Fun]
	// This works for some cases where Instances doesn't have an entry
	// BUT: If the signature still has type parameters, we need Phase 2 resolution
	if tv, ok := inf.info.Types[call.Fun]; ok {
		if sig, ok := tv.Type.(*types.Signature); ok {
			// Only return if it's already instantiated (no type parameters)
			if sig.TypeParams() == nil || sig.TypeParams().Len() == 0 {
				return sig
			}
			// Signature has type parameters - fall through to Phase 2
		}
	}

	// Phase 2: Manual type parameter resolution
	// If Info.Instances is empty (common for transpiled code), resolve manually
	genericSig := inf.getGenericSignature(call.Fun)
	if genericSig != nil && genericSig.TypeParams() != nil && genericSig.TypeParams().Len() > 0 {
		// Resolve type parameters from concrete argument types
		typeArgs := inf.resolveTypeParamsFromArgs(call, genericSig)
		if len(typeArgs) > 0 {
			// Instantiate the generic signature with resolved type arguments
			instantiated := inf.instantiateSignature(genericSig, typeArgs)
			return instantiated
		}
	}

	return nil
}

// rewriteFuncLit updates a function literal's parameter and return types
// to match the expected signature.
// Returns true if any changes were made.
func (inf *LambdaTypeInferrer) rewriteFuncLit(fn *ast.FuncLit, expected *types.Signature) bool {
	changed := false

	// Rewrite parameters
	if inf.rewriteParams(fn, expected) {
		changed = true
	}

	// Rewrite return type
	if inf.rewriteResults(fn, expected) {
		changed = true
	}

	return changed
}

// rewriteParams updates function literal parameter types.
func (inf *LambdaTypeInferrer) rewriteParams(fn *ast.FuncLit, expected *types.Signature) bool {
	if fn.Type.Params == nil || len(fn.Type.Params.List) == 0 {
		return false
	}

	changed := false
	expectedParams := expected.Params()
	if expectedParams == nil {
		return false
	}

	paramIdx := 0

	for _, field := range fn.Type.Params.List {
		// Handle multiple names in single field: (a, b int)
		numNames := len(field.Names)
		if numNames == 0 {
			numNames = 1
		}

		// Only rewrite if current type is 'any' (placeholder)
		if inf.isAnyType(field.Type) {
			// All names in this field share the same type
			if paramIdx < expectedParams.Len() {
				expectedType := expectedParams.At(paramIdx).Type()
				newTypeExpr := inf.typeToExpr(expectedType)
				if newTypeExpr != nil {
					field.Type = newTypeExpr
					changed = true
				}
			}
		}

		paramIdx += numNames
	}

	return changed
}

// rewriteResults updates function literal return types.
func (inf *LambdaTypeInferrer) rewriteResults(fn *ast.FuncLit, expected *types.Signature) bool {
	expectedResults := expected.Results()
	if expectedResults == nil || expectedResults.Len() == 0 {
		// Expected function has no return value
		if fn.Type.Results != nil && len(fn.Type.Results.List) > 0 {
			// Lambda has return type but shouldn't - don't change
			// This would be a type error caught by go/types
		}
		return false
	}

	changed := false

	if fn.Type.Results == nil || len(fn.Type.Results.List) == 0 {
		// Lambda has no return type annotation, but expected type does
		// Add return type from expected signature
		// Note: Only add if we can successfully convert the type
		typeExprs := make([]ast.Expr, 0, expectedResults.Len())
		for i := 0; i < expectedResults.Len(); i++ {
			expectedType := expectedResults.At(i).Type()
			expr := inf.typeToExpr(expectedType)
			if expr != nil {
				typeExprs = append(typeExprs, expr)
			}
		}

		// Only add results if we successfully converted all types
		if len(typeExprs) == expectedResults.Len() {
			results := &ast.FieldList{
				List: make([]*ast.Field, len(typeExprs)),
			}
			for i, expr := range typeExprs {
				results.List[i] = &ast.Field{
					Type: expr,
				}
			}
			fn.Type.Results = results
			return true
		}
		return false
	}

	// Rewrite existing return types if they're 'any'
	for i, field := range fn.Type.Results.List {
		if i >= expectedResults.Len() {
			break
		}
		if inf.isAnyType(field.Type) {
			expectedType := expectedResults.At(i).Type()
			newTypeExpr := inf.typeToExpr(expectedType)
			if newTypeExpr != nil {
				field.Type = newTypeExpr
				changed = true
			}
		}
	}

	return changed
}

// isAnyType checks if an expression is 'any' (interface{}).
func (inf *LambdaTypeInferrer) isAnyType(expr ast.Expr) bool {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name == "any"
	case *ast.InterfaceType:
		return t.Methods == nil || len(t.Methods.List) == 0
	}
	return false
}

// typeToExpr converts a types.Type to an ast.Expr.
// Reuses logic from existing TypeRewriter.
func (inf *LambdaTypeInferrer) typeToExpr(t types.Type) ast.Expr {
	if t == nil {
		return nil
	}

	switch typ := t.(type) {
	case *types.Basic:
		// Handle untyped constants by converting to their typed equivalents
		// using go/types Kind constants (no string manipulation)
		name := untypedToTypedName(typ.Kind())
		if name == "" {
			name = typ.Name() // fallback for already-typed basics
		}
		return &ast.Ident{Name: name}

	case *types.Pointer:
		elem := inf.typeToExpr(typ.Elem())
		if elem != nil {
			return &ast.StarExpr{X: elem}
		}

	case *types.Named:
		// For named types, use the type name
		obj := typ.Obj()
		if obj != nil {
			name := obj.Name()
			pkg := obj.Pkg()

			// Handle generic instantiation
			if typ.TypeArgs() != nil && typ.TypeArgs().Len() > 0 {
				// Generic type instantiation: Type[T1, T2, ...]
				typeArgs := make([]ast.Expr, typ.TypeArgs().Len())
				for i := 0; i < typ.TypeArgs().Len(); i++ {
					arg := inf.typeToExpr(typ.TypeArgs().At(i))
					if arg == nil {
						// Can't convert type arg - fall back to uninstantiated name
						goto simpleNamed
					}
					typeArgs[i] = arg
				}

				var baseExpr ast.Expr
				alias := inf.getPackageAlias(pkg)
				if alias != "" && alias != "main" {
					baseExpr = &ast.SelectorExpr{
						X:   &ast.Ident{Name: alias},
						Sel: &ast.Ident{Name: name},
					}
				} else {
					baseExpr = &ast.Ident{Name: name}
				}

				// Use IndexListExpr for multiple type args, IndexExpr for single
				if len(typeArgs) == 1 {
					return &ast.IndexExpr{
						X:     baseExpr,
						Index: typeArgs[0],
					}
				} else {
					return &ast.IndexListExpr{
						X:       baseExpr,
						Indices: typeArgs,
					}
				}
			}

		simpleNamed:
			alias := inf.getPackageAlias(pkg)
			if alias != "" && alias != "main" {
				// Qualified type: pkg.Name
				return &ast.SelectorExpr{
					X:   &ast.Ident{Name: alias},
					Sel: &ast.Ident{Name: name},
				}
			}
			return &ast.Ident{Name: name}
		}

	case *types.Slice:
		elem := inf.typeToExpr(typ.Elem())
		if elem != nil {
			return &ast.ArrayType{Elt: elem}
		}

	case *types.Array:
		elem := inf.typeToExpr(typ.Elem())
		if elem != nil {
			return &ast.ArrayType{
				Len: &ast.BasicLit{
					Kind:  token.INT,
					Value: fmt.Sprintf("%d", typ.Len()),
				},
				Elt: elem,
			}
		}

	case *types.Map:
		key := inf.typeToExpr(typ.Key())
		val := inf.typeToExpr(typ.Elem())
		if key != nil && val != nil {
			return &ast.MapType{Key: key, Value: val}
		}

	case *types.Chan:
		elem := inf.typeToExpr(typ.Elem())
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

	case *types.Signature:
		// Function type
		return inf.signatureToFuncType(typ)

	case *types.Interface:
		// Build interface with method set
		if typ.NumMethods() == 0 {
			// Empty interface{} or any
			return &ast.InterfaceType{Methods: &ast.FieldList{}}
		}

		methods := &ast.FieldList{
			List: make([]*ast.Field, 0, typ.NumMethods()),
		}

		for i := 0; i < typ.NumMethods(); i++ {
			method := typ.Method(i)
			sig, ok := method.Type().(*types.Signature)
			if !ok {
				continue
			}

			methods.List = append(methods.List, &ast.Field{
				Names: []*ast.Ident{{Name: method.Name()}},
				Type:  inf.signatureToFuncType(sig),
			})
		}

		return &ast.InterfaceType{Methods: methods}

	case *types.Struct:
		// Anonymous struct - convert to struct type
		return inf.structToStructType(typ)

	case *types.TypeParam:
		// Generic type parameter (e.g., T in func[T any])
		// If we reach this case, it means the type parameter wasn't resolved
		// Fall back to 'any' for unresolved type parameters
		return &ast.Ident{Name: "any"}
	}

	return nil
}

// signatureToFuncType converts a function signature to a func type AST node.
func (inf *LambdaTypeInferrer) signatureToFuncType(sig *types.Signature) *ast.FuncType {
	funcType := &ast.FuncType{}

	// Parameters
	if sig.Params() != nil && sig.Params().Len() > 0 {
		params := &ast.FieldList{
			List: make([]*ast.Field, sig.Params().Len()),
		}
		for i := 0; i < sig.Params().Len(); i++ {
			param := sig.Params().At(i)
			params.List[i] = &ast.Field{
				Type: inf.typeToExpr(param.Type()),
			}
		}
		funcType.Params = params
	} else {
		funcType.Params = &ast.FieldList{}
	}

	// Results
	if sig.Results() != nil && sig.Results().Len() > 0 {
		results := &ast.FieldList{
			List: make([]*ast.Field, sig.Results().Len()),
		}
		for i := 0; i < sig.Results().Len(); i++ {
			result := sig.Results().At(i)
			results.List[i] = &ast.Field{
				Type: inf.typeToExpr(result.Type()),
			}
		}
		funcType.Results = results
	}

	return funcType
}

// structToStructType converts an anonymous struct type to an AST struct type.
func (inf *LambdaTypeInferrer) structToStructType(st *types.Struct) *ast.StructType {
	fields := &ast.FieldList{
		List: make([]*ast.Field, st.NumFields()),
	}

	for i := 0; i < st.NumFields(); i++ {
		field := st.Field(i)
		fields.List[i] = &ast.Field{
			Names: []*ast.Ident{{Name: field.Name()}},
			Type:  inf.typeToExpr(field.Type()),
		}
	}

	return &ast.StructType{Fields: fields}
}

// getPackageAlias resolves the correct package identifier for a package,
// respecting import aliases in the file.
func (inf *LambdaTypeInferrer) getPackageAlias(pkg *types.Package) string {
	if pkg == nil {
		return ""
	}

	// Search file imports for this package
	pkgPath := pkg.Path()
	for _, imp := range inf.file.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		if path == pkgPath {
			if imp.Name != nil {
				// Explicit alias (including "." for dot imports)
				return imp.Name.Name
			}
			// Default name
			return pkg.Name()
		}
	}

	// Not imported (shouldn't happen if go/types succeeded)
	// Fall back to package name
	return pkg.Name()
}

// getGenericSignature extracts the generic (uninstantiated) signature from a function expression.
// Returns nil if the function is not generic or if it cannot be resolved.
func (inf *LambdaTypeInferrer) getGenericSignature(fun ast.Expr) *types.Signature {
	// Unwrap indexing expressions to get to the base function
	var baseExpr ast.Expr = fun
	switch fn := fun.(type) {
	case *ast.IndexExpr:
		baseExpr = fn.X
	case *ast.IndexListExpr:
		baseExpr = fn.X
	}

	// Look up the function object
	var obj types.Object
	switch base := baseExpr.(type) {
	case *ast.Ident:
		obj = inf.info.Uses[base]
	case *ast.SelectorExpr:
		// Try as method selection first
		if sel := inf.info.Selections[base]; sel != nil {
			obj = sel.Obj()
		} else {
			// Try as package-qualified identifier
			obj = inf.info.Uses[base.Sel]
		}
	}

	if obj == nil {
		return nil
	}

	// Extract signature
	sig, ok := obj.Type().(*types.Signature)
	if !ok {
		return nil
	}

	// Check if it has type parameters
	if sig.TypeParams() == nil || sig.TypeParams().Len() == 0 {
		return nil
	}

	return sig
}

// resolveTypeParamsFromArgs resolves type parameters by analyzing concrete argument types.
// For Filter[T](items []T, predicate func(T) bool), if we see Filter(users, ...) where
// users is []User, we resolve T=User.
func (inf *LambdaTypeInferrer) resolveTypeParamsFromArgs(call *ast.CallExpr, genericSig *types.Signature) []types.Type {
	if genericSig.TypeParams() == nil || genericSig.TypeParams().Len() == 0 {
		return nil
	}

	// Build substitution map: TypeParam -> ConcreteType
	subst := make(map[*types.TypeParam]types.Type)

	params := genericSig.Params()
	for i, arg := range call.Args {
		// Skip lambda arguments - they're what we're trying to infer!
		if _, ok := arg.(*ast.FuncLit); ok {
			continue
		}

		// Get parameter index (handle variadic later)
		if i >= params.Len() {
			continue
		}

		// Get expected parameter type (with type parameters)
		paramType := params.At(i).Type()

		// Get concrete argument type
		tv, ok := inf.info.Types[arg]
		if !ok {
			continue
		}
		argType := tv.Type

		// Match argument type to parameter type to extract type parameter bindings
		inf.matchTypeToParam(argType, paramType, subst)
	}

	// Convert substitution map to ordered type arguments
	// The order must match genericSig.TypeParams()
	typeParams := genericSig.TypeParams()
	typeArgs := make([]types.Type, typeParams.Len())

	for i := 0; i < typeParams.Len(); i++ {
		tp := typeParams.At(i)
		if concreteType, ok := subst[tp]; ok {
			typeArgs[i] = concreteType
		} else {
			// Use 'any' for unresolved type parameters (common when lambda body uses the type param)
			// Phase 3 will improve this by analyzing lambda bodies
			typeArgs[i] = types.Universe.Lookup("any").Type()
		}
	}

	// Return even if some type args are 'any' - partial resolution is better than nothing
	return typeArgs
}

// matchTypeToParam matches a concrete type to a (possibly generic) parameter type,
// extracting type parameter bindings.
// For example: matchTypeToParam([]User, []T, subst) sets subst[T] = User
func (inf *LambdaTypeInferrer) matchTypeToParam(concreteType, paramType types.Type, subst map[*types.TypeParam]types.Type) {
	// Handle type parameter directly
	if tp, ok := paramType.(*types.TypeParam); ok {
		if _, exists := subst[tp]; !exists {
			subst[tp] = concreteType
		}
		return
	}

	// Handle slice: []T
	if concreteSlice, ok := concreteType.(*types.Slice); ok {
		if paramSlice, ok := paramType.(*types.Slice); ok {
			inf.matchTypeToParam(concreteSlice.Elem(), paramSlice.Elem(), subst)
			return
		}
	}

	// Handle array: [N]T
	if concreteArray, ok := concreteType.(*types.Array); ok {
		if paramArray, ok := paramType.(*types.Array); ok {
			inf.matchTypeToParam(concreteArray.Elem(), paramArray.Elem(), subst)
			return
		}
	}

	// Handle map: map[K]V
	if concreteMap, ok := concreteType.(*types.Map); ok {
		if paramMap, ok := paramType.(*types.Map); ok {
			inf.matchTypeToParam(concreteMap.Key(), paramMap.Key(), subst)
			inf.matchTypeToParam(concreteMap.Elem(), paramMap.Elem(), subst)
			return
		}
	}

	// Handle pointer: *T
	if concretePtr, ok := concreteType.(*types.Pointer); ok {
		if paramPtr, ok := paramType.(*types.Pointer); ok {
			inf.matchTypeToParam(concretePtr.Elem(), paramPtr.Elem(), subst)
			return
		}
	}

	// Handle channel: chan T, <-chan T, chan<- T
	if concreteChan, ok := concreteType.(*types.Chan); ok {
		if paramChan, ok := paramType.(*types.Chan); ok {
			inf.matchTypeToParam(concreteChan.Elem(), paramChan.Elem(), subst)
			return
		}
	}

	// Handle named types with type arguments: Type[T1, T2]
	if concreteNamed, ok := concreteType.(*types.Named); ok {
		if paramNamed, ok := paramType.(*types.Named); ok {
			// Check if both are instantiations of the same generic type
			if concreteNamed.Obj() == paramNamed.Obj() {
				// Match type arguments
				concreteArgs := concreteNamed.TypeArgs()
				paramArgs := paramNamed.TypeArgs()
				if concreteArgs != nil && paramArgs != nil {
					for i := 0; i < concreteArgs.Len() && i < paramArgs.Len(); i++ {
						inf.matchTypeToParam(concreteArgs.At(i), paramArgs.At(i), subst)
					}
				}
			}
			return
		}
	}

	// Handle function types: func(T) R
	if concreteSig, ok := concreteType.(*types.Signature); ok {
		if paramSig, ok := paramType.(*types.Signature); ok {
			// Match parameters
			if concreteSig.Params() != nil && paramSig.Params() != nil {
				for i := 0; i < concreteSig.Params().Len() && i < paramSig.Params().Len(); i++ {
					inf.matchTypeToParam(
						concreteSig.Params().At(i).Type(),
						paramSig.Params().At(i).Type(),
						subst,
					)
				}
			}
			// Match results
			if concreteSig.Results() != nil && paramSig.Results() != nil {
				for i := 0; i < concreteSig.Results().Len() && i < paramSig.Results().Len(); i++ {
					inf.matchTypeToParam(
						concreteSig.Results().At(i).Type(),
						paramSig.Results().At(i).Type(),
						subst,
					)
				}
			}
			return
		}
	}

	// No structural match - types don't align or no type parameters present
}

// instantiateSignature creates a concrete signature by substituting type arguments
// into a generic signature's type parameters.
func (inf *LambdaTypeInferrer) instantiateSignature(genericSig *types.Signature, typeArgs []types.Type) *types.Signature {
	if len(typeArgs) == 0 {
		return nil
	}

	typeParams := genericSig.TypeParams()
	if typeParams == nil || typeParams.Len() != len(typeArgs) {
		return nil
	}

	// Use types.Instantiate to perform the substitution
	// This handles all the complex cases (nested types, constraints, etc.)
	// NOTE: Create a fresh context for each instantiation
	ctx := types.NewContext()
	instantiated, err := types.Instantiate(ctx, genericSig, typeArgs, false)
	if err != nil {
		return nil
	}

	sig, ok := instantiated.(*types.Signature)
	if !ok {
		return nil
	}

	return sig
}
