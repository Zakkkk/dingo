package typechecker

import (
	"go/token"
	"go/types"
	"sync"
)

// DgoSignatureRegistry provides pre-built signatures for dgo functions.
// Signatures are created lazily and cached.
type DgoSignatureRegistry struct {
	once  sync.Once
	cache map[string]*types.Signature
}

var dgoRegistry = &DgoSignatureRegistry{}

// GetDgoSignature returns a synthetic signature for a dgo function.
// Returns nil if the function is not in the registry.
func GetDgoSignature(funcName string) *types.Signature {
	dgoRegistry.once.Do(dgoRegistry.init)
	return dgoRegistry.cache[funcName]
}

// init builds all dgo signatures using go/types API.
func (r *DgoSignatureRegistry) init() {
	r.cache = make(map[string]*types.Signature)

	// Create synthetic package for type parameters
	pkg := types.NewPackage("", "")

	// Core functions
	r.cache["Map"] = r.buildMapSignature(pkg)
	r.cache["Filter"] = r.buildFilterSignature(pkg)
	r.cache["Reduce"] = r.buildReduceSignature(pkg)
	r.cache["ForEach"] = r.buildForEachSignature(pkg)

	// Index-aware variants
	r.cache["MapWithIndex"] = r.buildMapWithIndexSignature(pkg)
	r.cache["FilterWithIndex"] = r.buildFilterWithIndexSignature(pkg)
	r.cache["ForEachWithIndex"] = r.buildForEachWithIndexSignature(pkg)

	// Search/predicate functions
	r.cache["Find"] = r.buildFindSignature(pkg)
	r.cache["FindIndex"] = r.buildFindIndexSignature(pkg)
	r.cache["Any"] = r.buildAnySignature(pkg)
	r.cache["All"] = r.buildAllSignature(pkg)
	r.cache["NoneMatch"] = r.buildNoneMatchSignature(pkg)
	r.cache["Contains"] = r.buildContainsSignature(pkg)
	r.cache["Count"] = r.buildCountSignature(pkg)

	// Advanced functions
	r.cache["FlatMap"] = r.buildFlatMapSignature(pkg)
	r.cache["Flatten"] = r.buildFlattenSignature(pkg)
	r.cache["Partition"] = r.buildPartitionSignature(pkg)
	r.cache["GroupBy"] = r.buildGroupBySignature(pkg)
	r.cache["Unique"] = r.buildUniqueSignature(pkg)
	r.cache["Reverse"] = r.buildReverseSignature(pkg)

	// Slice manipulation
	r.cache["Take"] = r.buildTakeSignature(pkg)
	r.cache["Drop"] = r.buildDropSignature(pkg)
	r.cache["TakeWhile"] = r.buildTakeWhileSignature(pkg)
	r.cache["DropWhile"] = r.buildDropWhileSignature(pkg)
	r.cache["Chunk"] = r.buildChunkSignature(pkg)
	r.cache["ZipSlices"] = r.buildZipSlicesSignature(pkg)
}

// buildMapSignature creates: func Map[T, U any](slice []T, fn func(T) U) []U
func (r *DgoSignatureRegistry) buildMapSignature(pkg *types.Package) *types.Signature {
	// Create type parameters T and U
	T := r.newTypeParam(pkg, "T")
	U := r.newTypeParam(pkg, "U")

	// Build func(T) U
	fnParam := types.NewVar(token.NoPos, pkg, "", T)
	fnResult := types.NewVar(token.NoPos, pkg, "", U)
	fnSig := types.NewSignatureType(
		nil, nil, nil,
		types.NewTuple(fnParam),
		types.NewTuple(fnResult),
		false,
	)

	// Build parameters: (slice []T, fn func(T) U)
	sliceParam := types.NewVar(token.NoPos, pkg, "slice", types.NewSlice(T))
	fnVar := types.NewVar(token.NoPos, pkg, "fn", fnSig)
	params := types.NewTuple(sliceParam, fnVar)

	// Build result: []U
	result := types.NewVar(token.NoPos, pkg, "", types.NewSlice(U))
	results := types.NewTuple(result)

	// Build signature with type parameters
	tparams := []*types.TypeParam{T.(*types.TypeParam), U.(*types.TypeParam)}
	return types.NewSignatureType(nil, nil, tparams, params, results, false)
}

// buildFilterSignature creates: func Filter[T any](slice []T, predicate func(T) bool) []T
func (r *DgoSignatureRegistry) buildFilterSignature(pkg *types.Package) *types.Signature {
	T := r.newTypeParam(pkg, "T")

	// Build func(T) bool
	fnParam := types.NewVar(token.NoPos, pkg, "", T)
	fnResult := types.NewVar(token.NoPos, pkg, "", types.Typ[types.Bool])
	fnSig := types.NewSignatureType(
		nil, nil, nil,
		types.NewTuple(fnParam),
		types.NewTuple(fnResult),
		false,
	)

	// Parameters: (slice []T, predicate func(T) bool)
	sliceParam := types.NewVar(token.NoPos, pkg, "slice", types.NewSlice(T))
	fnVar := types.NewVar(token.NoPos, pkg, "predicate", fnSig)
	params := types.NewTuple(sliceParam, fnVar)

	// Result: []T
	result := types.NewVar(token.NoPos, pkg, "", types.NewSlice(T))
	results := types.NewTuple(result)

	tparams := []*types.TypeParam{T.(*types.TypeParam)}
	return types.NewSignatureType(nil, nil, tparams, params, results, false)
}

// buildReduceSignature creates: func Reduce[T, R any](slice []T, initial R, fn func(R, T) R) R
func (r *DgoSignatureRegistry) buildReduceSignature(pkg *types.Package) *types.Signature {
	T := r.newTypeParam(pkg, "T")
	R := r.newTypeParam(pkg, "R")

	// Build func(R, T) R
	fnParams := types.NewTuple(
		types.NewVar(token.NoPos, pkg, "", R),
		types.NewVar(token.NoPos, pkg, "", T),
	)
	fnResult := types.NewVar(token.NoPos, pkg, "", R)
	fnSig := types.NewSignatureType(
		nil, nil, nil, fnParams, types.NewTuple(fnResult), false,
	)

	// Parameters: (slice []T, initial R, fn func(R, T) R)
	params := types.NewTuple(
		types.NewVar(token.NoPos, pkg, "slice", types.NewSlice(T)),
		types.NewVar(token.NoPos, pkg, "initial", R),
		types.NewVar(token.NoPos, pkg, "fn", fnSig),
	)

	// Result: R
	results := types.NewTuple(types.NewVar(token.NoPos, pkg, "", R))

	tparams := []*types.TypeParam{T.(*types.TypeParam), R.(*types.TypeParam)}
	return types.NewSignatureType(nil, nil, tparams, params, results, false)
}

// buildForEachSignature creates: func ForEach[T any](slice []T, fn func(T))
func (r *DgoSignatureRegistry) buildForEachSignature(pkg *types.Package) *types.Signature {
	T := r.newTypeParam(pkg, "T")

	// Build func(T)
	fnParam := types.NewVar(token.NoPos, pkg, "", T)
	fnSig := types.NewSignatureType(
		nil, nil, nil,
		types.NewTuple(fnParam),
		nil, // no return
		false,
	)

	// Parameters: (slice []T, fn func(T))
	sliceParam := types.NewVar(token.NoPos, pkg, "slice", types.NewSlice(T))
	fnVar := types.NewVar(token.NoPos, pkg, "fn", fnSig)
	params := types.NewTuple(sliceParam, fnVar)

	tparams := []*types.TypeParam{T.(*types.TypeParam)}
	return types.NewSignatureType(nil, nil, tparams, params, nil, false)
}

// buildMapWithIndexSignature creates: func MapWithIndex[T, U any](slice []T, fn func(int, T) U) []U
func (r *DgoSignatureRegistry) buildMapWithIndexSignature(pkg *types.Package) *types.Signature {
	T := r.newTypeParam(pkg, "T")
	U := r.newTypeParam(pkg, "U")

	// Build func(int, T) U
	fnParams := types.NewTuple(
		types.NewVar(token.NoPos, pkg, "", types.Typ[types.Int]),
		types.NewVar(token.NoPos, pkg, "", T),
	)
	fnResult := types.NewVar(token.NoPos, pkg, "", U)
	fnSig := types.NewSignatureType(
		nil, nil, nil, fnParams, types.NewTuple(fnResult), false,
	)

	// Parameters: (slice []T, fn func(int, T) U)
	sliceParam := types.NewVar(token.NoPos, pkg, "slice", types.NewSlice(T))
	fnVar := types.NewVar(token.NoPos, pkg, "fn", fnSig)
	params := types.NewTuple(sliceParam, fnVar)

	// Result: []U
	result := types.NewVar(token.NoPos, pkg, "", types.NewSlice(U))
	results := types.NewTuple(result)

	tparams := []*types.TypeParam{T.(*types.TypeParam), U.(*types.TypeParam)}
	return types.NewSignatureType(nil, nil, tparams, params, results, false)
}

// buildFilterWithIndexSignature creates: func FilterWithIndex[T any](slice []T, predicate func(int, T) bool) []T
func (r *DgoSignatureRegistry) buildFilterWithIndexSignature(pkg *types.Package) *types.Signature {
	T := r.newTypeParam(pkg, "T")

	// Build func(int, T) bool
	fnParams := types.NewTuple(
		types.NewVar(token.NoPos, pkg, "", types.Typ[types.Int]),
		types.NewVar(token.NoPos, pkg, "", T),
	)
	fnResult := types.NewVar(token.NoPos, pkg, "", types.Typ[types.Bool])
	fnSig := types.NewSignatureType(
		nil, nil, nil, fnParams, types.NewTuple(fnResult), false,
	)

	// Parameters: (slice []T, predicate func(int, T) bool)
	sliceParam := types.NewVar(token.NoPos, pkg, "slice", types.NewSlice(T))
	fnVar := types.NewVar(token.NoPos, pkg, "predicate", fnSig)
	params := types.NewTuple(sliceParam, fnVar)

	// Result: []T
	result := types.NewVar(token.NoPos, pkg, "", types.NewSlice(T))
	results := types.NewTuple(result)

	tparams := []*types.TypeParam{T.(*types.TypeParam)}
	return types.NewSignatureType(nil, nil, tparams, params, results, false)
}

// buildForEachWithIndexSignature creates: func ForEachWithIndex[T any](slice []T, fn func(int, T))
func (r *DgoSignatureRegistry) buildForEachWithIndexSignature(pkg *types.Package) *types.Signature {
	T := r.newTypeParam(pkg, "T")

	// Build func(int, T)
	fnParams := types.NewTuple(
		types.NewVar(token.NoPos, pkg, "", types.Typ[types.Int]),
		types.NewVar(token.NoPos, pkg, "", T),
	)
	fnSig := types.NewSignatureType(
		nil, nil, nil, fnParams, nil, false,
	)

	// Parameters: (slice []T, fn func(int, T))
	sliceParam := types.NewVar(token.NoPos, pkg, "slice", types.NewSlice(T))
	fnVar := types.NewVar(token.NoPos, pkg, "fn", fnSig)
	params := types.NewTuple(sliceParam, fnVar)

	tparams := []*types.TypeParam{T.(*types.TypeParam)}
	return types.NewSignatureType(nil, nil, tparams, params, nil, false)
}

// buildFindSignature creates: func Find[T any](slice []T, predicate func(T) bool) Option[T]
func (r *DgoSignatureRegistry) buildFindSignature(pkg *types.Package) *types.Signature {
	T := r.newTypeParam(pkg, "T")

	// Build func(T) bool
	fnParam := types.NewVar(token.NoPos, pkg, "", T)
	fnResult := types.NewVar(token.NoPos, pkg, "", types.Typ[types.Bool])
	fnSig := types.NewSignatureType(
		nil, nil, nil,
		types.NewTuple(fnParam),
		types.NewTuple(fnResult),
		false,
	)

	// Parameters: (slice []T, predicate func(T) bool)
	sliceParam := types.NewVar(token.NoPos, pkg, "slice", types.NewSlice(T))
	fnVar := types.NewVar(token.NoPos, pkg, "predicate", fnSig)
	params := types.NewTuple(sliceParam, fnVar)

	// Result: Option[T] - we'll represent this as a named type
	// Since we can't easily create the actual Option type, we use a placeholder
	// The unifier will match against the actual Option[T] type from the code
	optionType := types.NewNamed(
		types.NewTypeName(token.NoPos, pkg, "Option", nil),
		nil,
		nil,
	)
	result := types.NewVar(token.NoPos, pkg, "", optionType)
	results := types.NewTuple(result)

	tparams := []*types.TypeParam{T.(*types.TypeParam)}
	return types.NewSignatureType(nil, nil, tparams, params, results, false)
}

// buildFindIndexSignature creates: func FindIndex[T any](slice []T, predicate func(T) bool) Option[int]
func (r *DgoSignatureRegistry) buildFindIndexSignature(pkg *types.Package) *types.Signature {
	T := r.newTypeParam(pkg, "T")

	// Build func(T) bool
	fnParam := types.NewVar(token.NoPos, pkg, "", T)
	fnResult := types.NewVar(token.NoPos, pkg, "", types.Typ[types.Bool])
	fnSig := types.NewSignatureType(
		nil, nil, nil,
		types.NewTuple(fnParam),
		types.NewTuple(fnResult),
		false,
	)

	// Parameters: (slice []T, predicate func(T) bool)
	sliceParam := types.NewVar(token.NoPos, pkg, "slice", types.NewSlice(T))
	fnVar := types.NewVar(token.NoPos, pkg, "predicate", fnSig)
	params := types.NewTuple(sliceParam, fnVar)

	// Result: Option[int]
	optionType := types.NewNamed(
		types.NewTypeName(token.NoPos, pkg, "Option", nil),
		nil,
		nil,
	)
	result := types.NewVar(token.NoPos, pkg, "", optionType)
	results := types.NewTuple(result)

	tparams := []*types.TypeParam{T.(*types.TypeParam)}
	return types.NewSignatureType(nil, nil, tparams, params, results, false)
}

// buildAnySignature creates: func Any[T any](slice []T, predicate func(T) bool) bool
func (r *DgoSignatureRegistry) buildAnySignature(pkg *types.Package) *types.Signature {
	T := r.newTypeParam(pkg, "T")

	// Build func(T) bool
	fnParam := types.NewVar(token.NoPos, pkg, "", T)
	fnResult := types.NewVar(token.NoPos, pkg, "", types.Typ[types.Bool])
	fnSig := types.NewSignatureType(
		nil, nil, nil,
		types.NewTuple(fnParam),
		types.NewTuple(fnResult),
		false,
	)

	// Parameters: (slice []T, predicate func(T) bool)
	sliceParam := types.NewVar(token.NoPos, pkg, "slice", types.NewSlice(T))
	fnVar := types.NewVar(token.NoPos, pkg, "predicate", fnSig)
	params := types.NewTuple(sliceParam, fnVar)

	// Result: bool
	result := types.NewVar(token.NoPos, pkg, "", types.Typ[types.Bool])
	results := types.NewTuple(result)

	tparams := []*types.TypeParam{T.(*types.TypeParam)}
	return types.NewSignatureType(nil, nil, tparams, params, results, false)
}

// buildAllSignature creates: func All[T any](slice []T, predicate func(T) bool) bool
func (r *DgoSignatureRegistry) buildAllSignature(pkg *types.Package) *types.Signature {
	// Same structure as Any
	return r.buildAnySignature(pkg)
}

// buildNoneMatchSignature creates: func NoneMatch[T any](slice []T, predicate func(T) bool) bool
func (r *DgoSignatureRegistry) buildNoneMatchSignature(pkg *types.Package) *types.Signature {
	// Same structure as Any
	return r.buildAnySignature(pkg)
}

// buildContainsSignature creates: func Contains[T comparable](slice []T, value T) bool
func (r *DgoSignatureRegistry) buildContainsSignature(pkg *types.Package) *types.Signature {
	// Create type parameter T with comparable constraint
	obj := types.NewTypeName(token.NoPos, pkg, "T", nil)
	constraint := types.Universe.Lookup("comparable").Type()
	T := types.NewTypeParam(obj, constraint)

	// Parameters: (slice []T, value T)
	params := types.NewTuple(
		types.NewVar(token.NoPos, pkg, "slice", types.NewSlice(T)),
		types.NewVar(token.NoPos, pkg, "value", T),
	)

	// Result: bool
	result := types.NewVar(token.NoPos, pkg, "", types.Typ[types.Bool])
	results := types.NewTuple(result)

	tparams := []*types.TypeParam{T}
	return types.NewSignatureType(nil, nil, tparams, params, results, false)
}

// buildCountSignature creates: func Count[T any](slice []T, predicate func(T) bool) int
func (r *DgoSignatureRegistry) buildCountSignature(pkg *types.Package) *types.Signature {
	T := r.newTypeParam(pkg, "T")

	// Build func(T) bool
	fnParam := types.NewVar(token.NoPos, pkg, "", T)
	fnResult := types.NewVar(token.NoPos, pkg, "", types.Typ[types.Bool])
	fnSig := types.NewSignatureType(
		nil, nil, nil,
		types.NewTuple(fnParam),
		types.NewTuple(fnResult),
		false,
	)

	// Parameters: (slice []T, predicate func(T) bool)
	sliceParam := types.NewVar(token.NoPos, pkg, "slice", types.NewSlice(T))
	fnVar := types.NewVar(token.NoPos, pkg, "predicate", fnSig)
	params := types.NewTuple(sliceParam, fnVar)

	// Result: int
	result := types.NewVar(token.NoPos, pkg, "", types.Typ[types.Int])
	results := types.NewTuple(result)

	tparams := []*types.TypeParam{T.(*types.TypeParam)}
	return types.NewSignatureType(nil, nil, tparams, params, results, false)
}

// buildFlatMapSignature creates: func FlatMap[T, U any](slice []T, fn func(T) []U) []U
func (r *DgoSignatureRegistry) buildFlatMapSignature(pkg *types.Package) *types.Signature {
	T := r.newTypeParam(pkg, "T")
	U := r.newTypeParam(pkg, "U")

	// Build func(T) []U
	fnParam := types.NewVar(token.NoPos, pkg, "", T)
	fnResult := types.NewVar(token.NoPos, pkg, "", types.NewSlice(U))
	fnSig := types.NewSignatureType(
		nil, nil, nil,
		types.NewTuple(fnParam),
		types.NewTuple(fnResult),
		false,
	)

	// Parameters: (slice []T, fn func(T) []U)
	sliceParam := types.NewVar(token.NoPos, pkg, "slice", types.NewSlice(T))
	fnVar := types.NewVar(token.NoPos, pkg, "fn", fnSig)
	params := types.NewTuple(sliceParam, fnVar)

	// Result: []U
	result := types.NewVar(token.NoPos, pkg, "", types.NewSlice(U))
	results := types.NewTuple(result)

	tparams := []*types.TypeParam{T.(*types.TypeParam), U.(*types.TypeParam)}
	return types.NewSignatureType(nil, nil, tparams, params, results, false)
}

// buildFlattenSignature creates: func Flatten[T any](slices [][]T) []T
func (r *DgoSignatureRegistry) buildFlattenSignature(pkg *types.Package) *types.Signature {
	T := r.newTypeParam(pkg, "T")

	// Parameters: (slices [][]T)
	sliceOfSlices := types.NewSlice(types.NewSlice(T))
	params := types.NewTuple(
		types.NewVar(token.NoPos, pkg, "slices", sliceOfSlices),
	)

	// Result: []T
	result := types.NewVar(token.NoPos, pkg, "", types.NewSlice(T))
	results := types.NewTuple(result)

	tparams := []*types.TypeParam{T.(*types.TypeParam)}
	return types.NewSignatureType(nil, nil, tparams, params, results, false)
}

// buildPartitionSignature creates: func Partition[T any](slice []T, predicate func(T) bool) ([]T, []T)
func (r *DgoSignatureRegistry) buildPartitionSignature(pkg *types.Package) *types.Signature {
	T := r.newTypeParam(pkg, "T")

	// Build func(T) bool
	fnParam := types.NewVar(token.NoPos, pkg, "", T)
	fnResult := types.NewVar(token.NoPos, pkg, "", types.Typ[types.Bool])
	fnSig := types.NewSignatureType(
		nil, nil, nil,
		types.NewTuple(fnParam),
		types.NewTuple(fnResult),
		false,
	)

	// Parameters: (slice []T, predicate func(T) bool)
	sliceParam := types.NewVar(token.NoPos, pkg, "slice", types.NewSlice(T))
	fnVar := types.NewVar(token.NoPos, pkg, "predicate", fnSig)
	params := types.NewTuple(sliceParam, fnVar)

	// Result: ([]T, []T)
	sliceType := types.NewSlice(T)
	results := types.NewTuple(
		types.NewVar(token.NoPos, pkg, "", sliceType),
		types.NewVar(token.NoPos, pkg, "", sliceType),
	)

	tparams := []*types.TypeParam{T.(*types.TypeParam)}
	return types.NewSignatureType(nil, nil, tparams, params, results, false)
}

// buildGroupBySignature creates: func GroupBy[T any, K comparable](slice []T, keyFn func(T) K) map[K][]T
func (r *DgoSignatureRegistry) buildGroupBySignature(pkg *types.Package) *types.Signature {
	T := r.newTypeParam(pkg, "T")
	// Create K with comparable constraint
	kObj := types.NewTypeName(token.NoPos, pkg, "K", nil)
	kConstraint := types.Universe.Lookup("comparable").Type()
	K := types.NewTypeParam(kObj, kConstraint)

	// Build func(T) K
	fnParam := types.NewVar(token.NoPos, pkg, "", T)
	fnResult := types.NewVar(token.NoPos, pkg, "", K)
	fnSig := types.NewSignatureType(
		nil, nil, nil,
		types.NewTuple(fnParam),
		types.NewTuple(fnResult),
		false,
	)

	// Parameters: (slice []T, keyFn func(T) K)
	sliceParam := types.NewVar(token.NoPos, pkg, "slice", types.NewSlice(T))
	fnVar := types.NewVar(token.NoPos, pkg, "keyFn", fnSig)
	params := types.NewTuple(sliceParam, fnVar)

	// Result: map[K][]T
	mapType := types.NewMap(K, types.NewSlice(T))
	result := types.NewVar(token.NoPos, pkg, "", mapType)
	results := types.NewTuple(result)

	tparams := []*types.TypeParam{T.(*types.TypeParam), K}
	return types.NewSignatureType(nil, nil, tparams, params, results, false)
}

// buildUniqueSignature creates: func Unique[T comparable](slice []T) []T
func (r *DgoSignatureRegistry) buildUniqueSignature(pkg *types.Package) *types.Signature {
	// Create type parameter T with comparable constraint
	obj := types.NewTypeName(token.NoPos, pkg, "T", nil)
	constraint := types.Universe.Lookup("comparable").Type()
	T := types.NewTypeParam(obj, constraint)

	// Parameters: (slice []T)
	params := types.NewTuple(
		types.NewVar(token.NoPos, pkg, "slice", types.NewSlice(T)),
	)

	// Result: []T
	result := types.NewVar(token.NoPos, pkg, "", types.NewSlice(T))
	results := types.NewTuple(result)

	tparams := []*types.TypeParam{T}
	return types.NewSignatureType(nil, nil, tparams, params, results, false)
}

// buildReverseSignature creates: func Reverse[T any](slice []T) []T
func (r *DgoSignatureRegistry) buildReverseSignature(pkg *types.Package) *types.Signature {
	T := r.newTypeParam(pkg, "T")

	// Parameters: (slice []T)
	params := types.NewTuple(
		types.NewVar(token.NoPos, pkg, "slice", types.NewSlice(T)),
	)

	// Result: []T
	result := types.NewVar(token.NoPos, pkg, "", types.NewSlice(T))
	results := types.NewTuple(result)

	tparams := []*types.TypeParam{T.(*types.TypeParam)}
	return types.NewSignatureType(nil, nil, tparams, params, results, false)
}

// buildTakeSignature creates: func Take[T any](slice []T, n int) []T
func (r *DgoSignatureRegistry) buildTakeSignature(pkg *types.Package) *types.Signature {
	T := r.newTypeParam(pkg, "T")

	// Parameters: (slice []T, n int)
	params := types.NewTuple(
		types.NewVar(token.NoPos, pkg, "slice", types.NewSlice(T)),
		types.NewVar(token.NoPos, pkg, "n", types.Typ[types.Int]),
	)

	// Result: []T
	result := types.NewVar(token.NoPos, pkg, "", types.NewSlice(T))
	results := types.NewTuple(result)

	tparams := []*types.TypeParam{T.(*types.TypeParam)}
	return types.NewSignatureType(nil, nil, tparams, params, results, false)
}

// buildDropSignature creates: func Drop[T any](slice []T, n int) []T
func (r *DgoSignatureRegistry) buildDropSignature(pkg *types.Package) *types.Signature {
	// Same structure as Take
	return r.buildTakeSignature(pkg)
}

// buildTakeWhileSignature creates: func TakeWhile[T any](slice []T, predicate func(T) bool) []T
func (r *DgoSignatureRegistry) buildTakeWhileSignature(pkg *types.Package) *types.Signature {
	// Same structure as Filter
	return r.buildFilterSignature(pkg)
}

// buildDropWhileSignature creates: func DropWhile[T any](slice []T, predicate func(T) bool) []T
func (r *DgoSignatureRegistry) buildDropWhileSignature(pkg *types.Package) *types.Signature {
	// Same structure as Filter
	return r.buildFilterSignature(pkg)
}

// buildChunkSignature creates: func Chunk[T any](slice []T, size int) [][]T
func (r *DgoSignatureRegistry) buildChunkSignature(pkg *types.Package) *types.Signature {
	T := r.newTypeParam(pkg, "T")

	// Parameters: (slice []T, size int)
	params := types.NewTuple(
		types.NewVar(token.NoPos, pkg, "slice", types.NewSlice(T)),
		types.NewVar(token.NoPos, pkg, "size", types.Typ[types.Int]),
	)

	// Result: [][]T
	result := types.NewVar(token.NoPos, pkg, "", types.NewSlice(types.NewSlice(T)))
	results := types.NewTuple(result)

	tparams := []*types.TypeParam{T.(*types.TypeParam)}
	return types.NewSignatureType(nil, nil, tparams, params, results, false)
}

// buildZipSlicesSignature creates: func ZipSlices[T, U any](a []T, b []U) []Pair[T, U]
func (r *DgoSignatureRegistry) buildZipSlicesSignature(pkg *types.Package) *types.Signature {
	T := r.newTypeParam(pkg, "T")
	U := r.newTypeParam(pkg, "U")

	// Parameters: (a []T, b []U)
	params := types.NewTuple(
		types.NewVar(token.NoPos, pkg, "a", types.NewSlice(T)),
		types.NewVar(token.NoPos, pkg, "b", types.NewSlice(U)),
	)

	// Result: []Pair[T, U] - we'll represent Pair as a named type
	pairType := types.NewNamed(
		types.NewTypeName(token.NoPos, pkg, "Pair", nil),
		nil,
		nil,
	)
	result := types.NewVar(token.NoPos, pkg, "", types.NewSlice(pairType))
	results := types.NewTuple(result)

	tparams := []*types.TypeParam{T.(*types.TypeParam), U.(*types.TypeParam)}
	return types.NewSignatureType(nil, nil, tparams, params, results, false)
}

// newTypeParam creates a fresh type parameter with the given name.
func (r *DgoSignatureRegistry) newTypeParam(pkg *types.Package, name string) types.Type {
	// Create type parameter with 'any' constraint
	obj := types.NewTypeName(token.NoPos, pkg, name, nil)
	constraint := types.Universe.Lookup("any").Type()
	return types.NewTypeParam(obj, constraint)
}
