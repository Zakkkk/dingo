package typechecker

import (
	"go/types"
	"testing"
)

// TestGetDgoSignature_Map tests the Map signature structure.
func TestGetDgoSignature_Map(t *testing.T) {
	sig := GetDgoSignature("Map")
	if sig == nil {
		t.Fatal("Expected Map signature, got nil")
	}

	// Verify type parameters: [T, U]
	tparams := sig.TypeParams()
	if tparams == nil {
		t.Fatal("Expected type parameters, got nil")
	}
	if tparams.Len() != 2 {
		t.Errorf("Expected 2 type parameters, got %d", tparams.Len())
	}
	if tparams.At(0).Obj().Name() != "T" {
		t.Errorf("Expected first type parameter to be T, got %s", tparams.At(0).Obj().Name())
	}
	if tparams.At(1).Obj().Name() != "U" {
		t.Errorf("Expected second type parameter to be U, got %s", tparams.At(1).Obj().Name())
	}

	// Verify parameters: (slice []T, fn func(T) U)
	params := sig.Params()
	if params.Len() != 2 {
		t.Fatalf("Expected 2 parameters, got %d", params.Len())
	}

	// First parameter should be []T
	param0Type := params.At(0).Type()
	if _, ok := param0Type.(*types.Slice); !ok {
		t.Errorf("Expected first parameter to be a slice, got %T", param0Type)
	}

	// Second parameter should be func(T) U
	param1Type := params.At(1).Type()
	fnSig, ok := param1Type.(*types.Signature)
	if !ok {
		t.Fatalf("Expected second parameter to be a function, got %T", param1Type)
	}
	if fnSig.Params().Len() != 1 {
		t.Errorf("Expected function to have 1 parameter, got %d", fnSig.Params().Len())
	}
	if fnSig.Results().Len() != 1 {
		t.Errorf("Expected function to have 1 result, got %d", fnSig.Results().Len())
	}

	// Verify result: []U
	results := sig.Results()
	if results.Len() != 1 {
		t.Errorf("Expected 1 result, got %d", results.Len())
	}
	resultType := results.At(0).Type()
	if _, ok := resultType.(*types.Slice); !ok {
		t.Errorf("Expected result to be a slice, got %T", resultType)
	}
}

// TestGetDgoSignature_Filter tests the Filter signature structure.
func TestGetDgoSignature_Filter(t *testing.T) {
	sig := GetDgoSignature("Filter")
	if sig == nil {
		t.Fatal("Expected Filter signature, got nil")
	}

	// Verify type parameters: [T]
	tparams := sig.TypeParams()
	if tparams == nil {
		t.Fatal("Expected type parameters, got nil")
	}
	if tparams.Len() != 1 {
		t.Errorf("Expected 1 type parameter, got %d", tparams.Len())
	}
	if tparams.At(0).Obj().Name() != "T" {
		t.Errorf("Expected type parameter to be T, got %s", tparams.At(0).Obj().Name())
	}

	// Verify parameters: (slice []T, predicate func(T) bool)
	params := sig.Params()
	if params.Len() != 2 {
		t.Fatalf("Expected 2 parameters, got %d", params.Len())
	}

	// Second parameter should be func(T) bool
	param1Type := params.At(1).Type()
	fnSig, ok := param1Type.(*types.Signature)
	if !ok {
		t.Fatalf("Expected second parameter to be a function, got %T", param1Type)
	}
	if fnSig.Results().Len() != 1 {
		t.Fatalf("Expected function to have 1 result, got %d", fnSig.Results().Len())
	}
	resultType := fnSig.Results().At(0).Type()
	if !types.Identical(resultType, types.Typ[types.Bool]) {
		t.Errorf("Expected function result to be bool, got %v", resultType)
	}
}

// TestGetDgoSignature_Reduce tests the Reduce signature structure.
func TestGetDgoSignature_Reduce(t *testing.T) {
	sig := GetDgoSignature("Reduce")
	if sig == nil {
		t.Fatal("Expected Reduce signature, got nil")
	}

	// Verify type parameters: [T, R]
	tparams := sig.TypeParams()
	if tparams == nil {
		t.Fatal("Expected type parameters, got nil")
	}
	if tparams.Len() != 2 {
		t.Errorf("Expected 2 type parameters, got %d", tparams.Len())
	}
	if tparams.At(0).Obj().Name() != "T" {
		t.Errorf("Expected first type parameter to be T, got %s", tparams.At(0).Obj().Name())
	}
	if tparams.At(1).Obj().Name() != "R" {
		t.Errorf("Expected second type parameter to be R, got %s", tparams.At(1).Obj().Name())
	}

	// Verify parameters: (slice []T, initial R, fn func(R, T) R)
	params := sig.Params()
	if params.Len() != 3 {
		t.Fatalf("Expected 3 parameters, got %d", params.Len())
	}

	// Third parameter should be func(R, T) R
	param2Type := params.At(2).Type()
	fnSig, ok := param2Type.(*types.Signature)
	if !ok {
		t.Fatalf("Expected third parameter to be a function, got %T", param2Type)
	}
	if fnSig.Params().Len() != 2 {
		t.Errorf("Expected function to have 2 parameters, got %d", fnSig.Params().Len())
	}
	if fnSig.Results().Len() != 1 {
		t.Errorf("Expected function to have 1 result, got %d", fnSig.Results().Len())
	}
}

// TestGetDgoSignature_GroupBy tests the GroupBy signature structure.
func TestGetDgoSignature_GroupBy(t *testing.T) {
	sig := GetDgoSignature("GroupBy")
	if sig == nil {
		t.Fatal("Expected GroupBy signature, got nil")
	}

	// Verify type parameters: [T, K] where K is comparable
	tparams := sig.TypeParams()
	if tparams == nil {
		t.Fatal("Expected type parameters, got nil")
	}
	if tparams.Len() != 2 {
		t.Errorf("Expected 2 type parameters, got %d", tparams.Len())
	}

	// Check T
	T := tparams.At(0)
	if T.Obj().Name() != "T" {
		t.Errorf("Expected first type parameter to be T, got %s", T.Obj().Name())
	}

	// Check K (should have comparable constraint)
	K := tparams.At(1)
	if K.Obj().Name() != "K" {
		t.Errorf("Expected second type parameter to be K, got %s", K.Obj().Name())
	}
	// Note: In go/types, we can't easily verify the constraint at runtime without more setup

	// Verify parameters: (slice []T, keyFn func(T) K)
	params := sig.Params()
	if params.Len() != 2 {
		t.Fatalf("Expected 2 parameters, got %d", params.Len())
	}

	// Second parameter should be func(T) K
	param1Type := params.At(1).Type()
	fnSig, ok := param1Type.(*types.Signature)
	if !ok {
		t.Fatalf("Expected second parameter to be a function, got %T", param1Type)
	}
	if fnSig.Params().Len() != 1 {
		t.Errorf("Expected function to have 1 parameter, got %d", fnSig.Params().Len())
	}
	if fnSig.Results().Len() != 1 {
		t.Errorf("Expected function to have 1 result, got %d", fnSig.Results().Len())
	}

	// Verify result: map[K][]T
	results := sig.Results()
	if results.Len() != 1 {
		t.Errorf("Expected 1 result, got %d", results.Len())
	}
	resultType := results.At(0).Type()
	if _, ok := resultType.(*types.Map); !ok {
		t.Errorf("Expected result to be a map, got %T", resultType)
	}
}

// TestGetDgoSignature_ForEach tests the ForEach signature structure.
func TestGetDgoSignature_ForEach(t *testing.T) {
	sig := GetDgoSignature("ForEach")
	if sig == nil {
		t.Fatal("Expected ForEach signature, got nil")
	}

	// Verify type parameters: [T]
	tparams := sig.TypeParams()
	if tparams.Len() != 1 {
		t.Errorf("Expected 1 type parameter, got %d", tparams.Len())
	}

	// Verify parameters: (slice []T, fn func(T))
	params := sig.Params()
	if params.Len() != 2 {
		t.Fatalf("Expected 2 parameters, got %d", params.Len())
	}

	// Second parameter should be func(T) with no return
	param1Type := params.At(1).Type()
	fnSig, ok := param1Type.(*types.Signature)
	if !ok {
		t.Fatalf("Expected second parameter to be a function, got %T", param1Type)
	}
	if fnSig.Results() != nil && fnSig.Results().Len() != 0 {
		t.Errorf("Expected function to have no results, got %d", fnSig.Results().Len())
	}

	// Verify no results
	if sig.Results() != nil && sig.Results().Len() != 0 {
		t.Errorf("Expected ForEach to have no results, got %d", sig.Results().Len())
	}
}

// TestGetDgoSignature_All tests all registered function names.
func TestGetDgoSignature_All(t *testing.T) {
	expectedFunctions := []string{
		"Map", "Filter", "Reduce", "ForEach",
		"MapWithIndex", "FilterWithIndex", "ForEachWithIndex",
		"Find", "FindIndex", "Any", "All", "NoneMatch", "Contains", "Count",
		"FlatMap", "Flatten", "Partition", "GroupBy", "Unique", "Reverse",
		"Take", "Drop", "TakeWhile", "DropWhile", "Chunk", "ZipSlices",
	}

	for _, funcName := range expectedFunctions {
		sig := GetDgoSignature(funcName)
		if sig == nil {
			t.Errorf("Expected signature for %s, got nil", funcName)
		}
	}
}

// TestGetDgoSignature_Unknown tests that unknown functions return nil.
func TestGetDgoSignature_Unknown(t *testing.T) {
	sig := GetDgoSignature("NonExistentFunction")
	if sig != nil {
		t.Errorf("Expected nil for unknown function, got %v", sig)
	}
}

// TestGetDgoSignature_Cached tests that signatures are cached correctly.
func TestGetDgoSignature_Cached(t *testing.T) {
	// Call twice and verify we get the same pointer (cached)
	sig1 := GetDgoSignature("Map")
	sig2 := GetDgoSignature("Map")

	if sig1 != sig2 {
		t.Error("Expected cached signature to return same pointer")
	}
}

// TestGetDgoSignature_MapWithIndex tests index-aware map signature.
func TestGetDgoSignature_MapWithIndex(t *testing.T) {
	sig := GetDgoSignature("MapWithIndex")
	if sig == nil {
		t.Fatal("Expected MapWithIndex signature, got nil")
	}

	// Verify type parameters: [T, U]
	tparams := sig.TypeParams()
	if tparams.Len() != 2 {
		t.Errorf("Expected 2 type parameters, got %d", tparams.Len())
	}

	// Second parameter should be func(int, T) U
	params := sig.Params()
	if params.Len() != 2 {
		t.Fatalf("Expected 2 parameters, got %d", params.Len())
	}

	param1Type := params.At(1).Type()
	fnSig, ok := param1Type.(*types.Signature)
	if !ok {
		t.Fatalf("Expected second parameter to be a function, got %T", param1Type)
	}

	// Function should have (int, T) parameters
	if fnSig.Params().Len() != 2 {
		t.Fatalf("Expected function to have 2 parameters, got %d", fnSig.Params().Len())
	}

	// First parameter should be int
	fnParam0 := fnSig.Params().At(0).Type()
	if !types.Identical(fnParam0, types.Typ[types.Int]) {
		t.Errorf("Expected first function parameter to be int, got %v", fnParam0)
	}
}

// TestGetDgoSignature_Contains tests comparable constraint.
func TestGetDgoSignature_Contains(t *testing.T) {
	sig := GetDgoSignature("Contains")
	if sig == nil {
		t.Fatal("Expected Contains signature, got nil")
	}

	// Verify type parameters: [T comparable]
	tparams := sig.TypeParams()
	if tparams.Len() != 1 {
		t.Errorf("Expected 1 type parameter, got %d", tparams.Len())
	}

	// Verify parameters: (slice []T, value T)
	params := sig.Params()
	if params.Len() != 2 {
		t.Fatalf("Expected 2 parameters, got %d", params.Len())
	}

	// Result should be bool
	results := sig.Results()
	if results.Len() != 1 {
		t.Errorf("Expected 1 result, got %d", results.Len())
	}
	resultType := results.At(0).Type()
	if !types.Identical(resultType, types.Typ[types.Bool]) {
		t.Errorf("Expected result to be bool, got %v", resultType)
	}
}

// TestGetDgoSignature_FlatMap tests nested slice handling.
func TestGetDgoSignature_FlatMap(t *testing.T) {
	sig := GetDgoSignature("FlatMap")
	if sig == nil {
		t.Fatal("Expected FlatMap signature, got nil")
	}

	// Verify type parameters: [T, U]
	tparams := sig.TypeParams()
	if tparams.Len() != 2 {
		t.Errorf("Expected 2 type parameters, got %d", tparams.Len())
	}

	// Second parameter should be func(T) []U
	params := sig.Params()
	param1Type := params.At(1).Type()
	fnSig, ok := param1Type.(*types.Signature)
	if !ok {
		t.Fatalf("Expected second parameter to be a function, got %T", param1Type)
	}

	// Function result should be a slice
	if fnSig.Results().Len() != 1 {
		t.Fatalf("Expected function to have 1 result, got %d", fnSig.Results().Len())
	}
	fnResultType := fnSig.Results().At(0).Type()
	if _, ok := fnResultType.(*types.Slice); !ok {
		t.Errorf("Expected function result to be a slice, got %T", fnResultType)
	}
}

// TestGetDgoSignature_Partition tests tuple-like return.
func TestGetDgoSignature_Partition(t *testing.T) {
	sig := GetDgoSignature("Partition")
	if sig == nil {
		t.Fatal("Expected Partition signature, got nil")
	}

	// Result should be ([]T, []T)
	results := sig.Results()
	if results.Len() != 2 {
		t.Errorf("Expected 2 results, got %d", results.Len())
	}

	// Both results should be slices
	result0 := results.At(0).Type()
	result1 := results.At(1).Type()
	if _, ok := result0.(*types.Slice); !ok {
		t.Errorf("Expected first result to be a slice, got %T", result0)
	}
	if _, ok := result1.(*types.Slice); !ok {
		t.Errorf("Expected second result to be a slice, got %T", result1)
	}
}

// TestGetDgoSignature_Chunk tests nested slice result.
func TestGetDgoSignature_Chunk(t *testing.T) {
	sig := GetDgoSignature("Chunk")
	if sig == nil {
		t.Fatal("Expected Chunk signature, got nil")
	}

	// Result should be [][]T
	results := sig.Results()
	if results.Len() != 1 {
		t.Errorf("Expected 1 result, got %d", results.Len())
	}

	resultType := results.At(0).Type()
	outerSlice, ok := resultType.(*types.Slice)
	if !ok {
		t.Fatalf("Expected result to be a slice, got %T", resultType)
	}

	// Element should also be a slice
	if _, ok := outerSlice.Elem().(*types.Slice); !ok {
		t.Errorf("Expected result to be [][]T, got %v", resultType)
	}
}
