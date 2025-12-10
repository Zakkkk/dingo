package typechecker

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"testing"
)

// TestInferTypeParams_SliceTypes tests type parameter inference with slice types.
func TestInferTypeParams_SliceTypes(t *testing.T) {
	fset := token.NewFileSet()
	src := `
package main

type User struct {
	Name string
}

func main() {
	users := []User{{Name: "Alice"}}
	_ = users
}
`

	// Parse and type-check the source
	f, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	conf := types.Config{}
	info := &types.Info{
		Types: make(map[ast.Expr]types.TypeAndValue),
		Defs:  make(map[*ast.Ident]types.Object),
		Uses:  make(map[*ast.Ident]types.Object),
	}
	pkg, err := conf.Check("main", fset, []*ast.File{f}, info)
	if err != nil {
		t.Fatalf("Failed to type-check: %v", err)
	}

	// Get the User type from the package scope
	userObj := pkg.Scope().Lookup("User")
	if userObj == nil {
		t.Fatal("User type not found")
	}
	userType := userObj.Type()

	// Create a synthetic generic signature: func Map[T, U](slice []T, fn func(T) U) []U
	tObj := types.NewTypeName(token.NoPos, pkg, "T", nil)
	uObj := types.NewTypeName(token.NoPos, pkg, "U", nil)
	anyConstraint := types.Universe.Lookup("any").Type()
	T := types.NewTypeParam(tObj, anyConstraint)
	U := types.NewTypeParam(uObj, anyConstraint)

	fnParam := types.NewVar(token.NoPos, pkg, "", T)
	fnResult := types.NewVar(token.NoPos, pkg, "", U)
	fnSig := types.NewSignatureType(nil, nil, nil,
		types.NewTuple(fnParam),
		types.NewTuple(fnResult),
		false,
	)

	sliceParam := types.NewVar(token.NoPos, pkg, "slice", types.NewSlice(T))
	fnVar := types.NewVar(token.NoPos, pkg, "fn", fnSig)
	params := types.NewTuple(sliceParam, fnVar)

	result := types.NewVar(token.NoPos, pkg, "", types.NewSlice(U))
	results := types.NewTuple(result)

	tparams := []*types.TypeParam{T, U}
	genericSig := types.NewSignatureType(nil, nil, tparams, params, results, false)

	// Find the users identifier in the actual AST (from the assignment)
	var usersIdent *ast.Ident
	ast.Inspect(f, func(n ast.Node) bool {
		if ident, ok := n.(*ast.Ident); ok && ident.Name == "users" {
			// Get the first one we find (which will be the definition)
			if usersIdent == nil {
				usersIdent = ident
			}
		}
		return true
	})

	if usersIdent == nil {
		t.Fatal("Could not find users identifier in AST")
	}

	// Create a call expression: Map(users, |u| u.Name)
	lambdaExpr := &ast.FuncLit{} // Placeholder for lambda
	callExpr := &ast.CallExpr{
		Fun:  &ast.Ident{Name: "Map"},
		Args: []ast.Expr{usersIdent, lambdaExpr},
	}

	// Create unifier and infer type parameters
	unifier := NewTypeUnifier(fset, info)
	bindings := unifier.InferTypeParams(callExpr, genericSig)

	// Verify that T is bound to User
	if bindings["T"] == nil {
		t.Error("Expected T to be bound, got nil")
	} else if !types.Identical(bindings["T"], userType) {
		t.Errorf("Expected T to be bound to User, got %v", bindings["T"])
	}
}

// TestInferTypeParams_NestedSlices tests inference with nested slice types.
func TestInferTypeParams_NestedSlices(t *testing.T) {
	fset := token.NewFileSet()
	src := `
package main

func main() {
	matrix := [][]int{{1, 2}, {3, 4}}
	_ = matrix
}
`

	f, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	conf := types.Config{}
	info := &types.Info{
		Types: make(map[ast.Expr]types.TypeAndValue),
		Defs:  make(map[*ast.Ident]types.Object),
		Uses:  make(map[*ast.Ident]types.Object),
	}
	pkg, err := conf.Check("main", fset, []*ast.File{f}, info)
	if err != nil {
		t.Fatalf("Failed to type-check: %v", err)
	}

	// Create a generic signature: func Flatten[T](slices [][]T) []T
	tObj := types.NewTypeName(token.NoPos, pkg, "T", nil)
	anyConstraint := types.Universe.Lookup("any").Type()
	T := types.NewTypeParam(tObj, anyConstraint)

	sliceOfSlices := types.NewSlice(types.NewSlice(T))
	sliceParam := types.NewVar(token.NoPos, pkg, "slices", sliceOfSlices)
	params := types.NewTuple(sliceParam)

	result := types.NewVar(token.NoPos, pkg, "", types.NewSlice(T))
	results := types.NewTuple(result)

	tparams := []*types.TypeParam{T}
	genericSig := types.NewSignatureType(nil, nil, tparams, params, results, false)

	// Find matrix identifier
	var matrixIdent *ast.Ident
	ast.Inspect(f, func(n ast.Node) bool {
		if ident, ok := n.(*ast.Ident); ok && ident.Name == "matrix" {
			matrixIdent = ident
			return false
		}
		return true
	})

	callExpr := &ast.CallExpr{
		Fun:  &ast.Ident{Name: "Flatten"},
		Args: []ast.Expr{matrixIdent},
	}

	unifier := NewTypeUnifier(fset, info)
	bindings := unifier.InferTypeParams(callExpr, genericSig)

	// Verify that T is bound to int
	if bindings["T"] == nil {
		t.Error("Expected T to be bound, got nil")
	} else if !types.Identical(bindings["T"], types.Typ[types.Int]) {
		t.Errorf("Expected T to be bound to int, got %v", bindings["T"])
	}
}

// TestInstantiateSignature tests signature instantiation with type bindings.
func TestInstantiateSignature(t *testing.T) {
	fset := token.NewFileSet()
	pkg := types.NewPackage("test", "test")

	// Create generic signature: func Map[T, U](slice []T, fn func(T) U) []U
	tObj := types.NewTypeName(token.NoPos, pkg, "T", nil)
	uObj := types.NewTypeName(token.NoPos, pkg, "U", nil)
	anyConstraint := types.Universe.Lookup("any").Type()
	T := types.NewTypeParam(tObj, anyConstraint)
	U := types.NewTypeParam(uObj, anyConstraint)

	fnParam := types.NewVar(token.NoPos, pkg, "", T)
	fnResult := types.NewVar(token.NoPos, pkg, "", U)
	fnSig := types.NewSignatureType(nil, nil, nil,
		types.NewTuple(fnParam),
		types.NewTuple(fnResult),
		false,
	)

	sliceParam := types.NewVar(token.NoPos, pkg, "slice", types.NewSlice(T))
	fnVar := types.NewVar(token.NoPos, pkg, "fn", fnSig)
	params := types.NewTuple(sliceParam, fnVar)

	result := types.NewVar(token.NoPos, pkg, "", types.NewSlice(U))
	results := types.NewTuple(result)

	tparams := []*types.TypeParam{T, U}
	genericSig := types.NewSignatureType(nil, nil, tparams, params, results, false)

	// Create bindings: T -> int, U -> string
	bindings := map[string]types.Type{
		"T": types.Typ[types.Int],
		"U": types.Typ[types.String],
	}

	// Instantiate signature
	info := &types.Info{
		Types: make(map[ast.Expr]types.TypeAndValue),
	}
	unifier := NewTypeUnifier(fset, info)
	instantiated := unifier.InstantiateSignature(genericSig, bindings)

	if instantiated == nil {
		t.Fatal("Expected instantiated signature, got nil")
	}

	// Verify parameters: (slice []int, fn func(int) string)
	if instantiated.Params().Len() != 2 {
		t.Errorf("Expected 2 parameters, got %d", instantiated.Params().Len())
	}

	// Check first parameter: []int
	param0Type := instantiated.Params().At(0).Type()
	expectedSliceType := types.NewSlice(types.Typ[types.Int])
	if !types.Identical(param0Type, expectedSliceType) {
		t.Errorf("Expected first parameter to be []int, got %v", param0Type)
	}

	// Check second parameter: func(int) string
	param1Type := instantiated.Params().At(1).Type()
	if sig, ok := param1Type.(*types.Signature); ok {
		if sig.Params().Len() != 1 || !types.Identical(sig.Params().At(0).Type(), types.Typ[types.Int]) {
			t.Errorf("Expected func(int) string parameter, got %v", sig)
		}
		if sig.Results().Len() != 1 || !types.Identical(sig.Results().At(0).Type(), types.Typ[types.String]) {
			t.Errorf("Expected func(int) string result, got %v", sig)
		}
	} else {
		t.Errorf("Expected second parameter to be a function, got %v", param1Type)
	}

	// Check result: []string
	resultType := instantiated.Results().At(0).Type()
	expectedResultType := types.NewSlice(types.Typ[types.String])
	if !types.Identical(resultType, expectedResultType) {
		t.Errorf("Expected result to be []string, got %v", resultType)
	}
}

// TestSubstituteTypeParams_Pointer tests substitution with pointer types.
func TestSubstituteTypeParams_Pointer(t *testing.T) {
	fset := token.NewFileSet()
	pkg := types.NewPackage("test", "test")

	// Create type parameter T
	tObj := types.NewTypeName(token.NoPos, pkg, "T", nil)
	anyConstraint := types.Universe.Lookup("any").Type()
	T := types.NewTypeParam(tObj, anyConstraint)

	// Create type: *T
	pointerType := types.NewPointer(T)

	// Create bindings: T -> int
	bindings := map[string]types.Type{
		"T": types.Typ[types.Int],
	}

	// Substitute
	info := &types.Info{}
	unifier := NewTypeUnifier(fset, info)
	substituted := unifier.substituteTypeParams(pointerType, bindings)

	// Verify result: *int
	expectedType := types.NewPointer(types.Typ[types.Int])
	if !types.Identical(substituted, expectedType) {
		t.Errorf("Expected *int, got %v", substituted)
	}
}

// TestSubstituteTypeParams_Map tests substitution with map types.
func TestSubstituteTypeParams_Map(t *testing.T) {
	fset := token.NewFileSet()
	pkg := types.NewPackage("test", "test")

	// Create type parameters K and V
	kObj := types.NewTypeName(token.NoPos, pkg, "K", nil)
	vObj := types.NewTypeName(token.NoPos, pkg, "V", nil)
	comparableConstraint := types.Universe.Lookup("comparable").Type()
	anyConstraint := types.Universe.Lookup("any").Type()
	K := types.NewTypeParam(kObj, comparableConstraint)
	V := types.NewTypeParam(vObj, anyConstraint)

	// Create type: map[K]V
	mapType := types.NewMap(K, V)

	// Create bindings: K -> string, V -> int
	bindings := map[string]types.Type{
		"K": types.Typ[types.String],
		"V": types.Typ[types.Int],
	}

	// Substitute
	info := &types.Info{}
	unifier := NewTypeUnifier(fset, info)
	substituted := unifier.substituteTypeParams(mapType, bindings)

	// Verify result: map[string]int
	expectedType := types.NewMap(types.Typ[types.String], types.Typ[types.Int])
	if !types.Identical(substituted, expectedType) {
		t.Errorf("Expected map[string]int, got %v", substituted)
	}
}

// TestSubstituteTypeParams_Array tests substitution with array types.
func TestSubstituteTypeParams_Array(t *testing.T) {
	fset := token.NewFileSet()
	pkg := types.NewPackage("test", "test")

	// Create type parameter T
	tObj := types.NewTypeName(token.NoPos, pkg, "T", nil)
	anyConstraint := types.Universe.Lookup("any").Type()
	T := types.NewTypeParam(tObj, anyConstraint)

	// Create type: [5]T
	arrayType := types.NewArray(T, 5)

	// Create bindings: T -> float64
	bindings := map[string]types.Type{
		"T": types.Typ[types.Float64],
	}

	// Substitute
	info := &types.Info{}
	unifier := NewTypeUnifier(fset, info)
	substituted := unifier.substituteTypeParams(arrayType, bindings)

	// Verify result: [5]float64
	expectedType := types.NewArray(types.Typ[types.Float64], 5)
	if !types.Identical(substituted, expectedType) {
		t.Errorf("Expected [5]float64, got %v", substituted)
	}
}

// TestUnify_MapType tests unification of map types.
func TestUnify_MapType(t *testing.T) {
	fset := token.NewFileSet()
	pkg := types.NewPackage("test", "test")

	// Create type parameters K and V
	kObj := types.NewTypeName(token.NoPos, pkg, "K", nil)
	vObj := types.NewTypeName(token.NoPos, pkg, "V", nil)
	comparableConstraint := types.Universe.Lookup("comparable").Type()
	anyConstraint := types.Universe.Lookup("any").Type()
	K := types.NewTypeParam(kObj, comparableConstraint)
	V := types.NewTypeParam(vObj, anyConstraint)

	// Create parameterized type: map[K][]V
	paramType := types.NewMap(K, types.NewSlice(V))

	// Create concrete type: map[string][]int
	concreteType := types.NewMap(types.Typ[types.String], types.NewSlice(types.Typ[types.Int]))

	// Unify
	bindings := make(map[string]types.Type)
	// Create a dummy signature to hold the type params
	dummySig := types.NewSignatureType(nil, nil, []*types.TypeParam{K, V}, nil, nil, false)
	typeParams := dummySig.TypeParams()
	info := &types.Info{}
	unifier := NewTypeUnifier(fset, info)
	unifier.unify(paramType, concreteType, typeParams, bindings)

	// Verify bindings
	if !types.Identical(bindings["K"], types.Typ[types.String]) {
		t.Errorf("Expected K to be bound to string, got %v", bindings["K"])
	}
	if !types.Identical(bindings["V"], types.Typ[types.Int]) {
		t.Errorf("Expected V to be bound to int, got %v", bindings["V"])
	}
}

// TestUnify_NilBindings tests that unify handles nil bindings gracefully.
func TestUnify_NilSignature(t *testing.T) {
	fset := token.NewFileSet()
	info := &types.Info{}
	unifier := NewTypeUnifier(fset, info)

	callExpr := &ast.CallExpr{Fun: &ast.Ident{Name: "test"}}
	bindings := unifier.InferTypeParams(callExpr, nil)

	if bindings != nil {
		t.Errorf("Expected nil bindings for nil signature, got %v", bindings)
	}
}

// TestInstantiateSignature_EmptyBindings tests instantiation with empty bindings.
func TestInstantiateSignature_EmptyBindings(t *testing.T) {
	fset := token.NewFileSet()
	pkg := types.NewPackage("test", "test")

	// Create a simple non-generic signature
	param := types.NewVar(token.NoPos, pkg, "x", types.Typ[types.Int])
	params := types.NewTuple(param)
	sig := types.NewSignatureType(nil, nil, nil, params, nil, false)

	info := &types.Info{}
	unifier := NewTypeUnifier(fset, info)

	// Try to instantiate with empty bindings
	instantiated := unifier.InstantiateSignature(sig, map[string]types.Type{})

	if instantiated != nil {
		t.Errorf("Expected nil for empty bindings, got %v", instantiated)
	}
}
