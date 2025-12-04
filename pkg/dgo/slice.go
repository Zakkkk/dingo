// Package dgo provides Dingo's core types and functional helpers
package dgo

// ============================================================================
// Utility Types
// ============================================================================

// Pair represents a pair of values of potentially different types.
// Used by functions like Zip to combine elements from two slices.
type Pair[T, U any] struct {
	First  T
	Second U
}

// ============================================================================
// Core Functions
// ============================================================================

// Map transforms each element of a slice using the provided function.
// Returns nil if the input slice is nil.
//
// Example:
//
//	numbers := []int{1, 2, 3}
//	doubled := dgo.Map(numbers, func(x int) int { return x * 2 })
//	// doubled = []int{2, 4, 6}
func Map[T, U any](slice []T, fn func(T) U) []U {
	if slice == nil {
		return nil
	}
	result := make([]U, len(slice))
	for i, v := range slice {
		result[i] = fn(v)
	}
	return result
}

// Filter returns a new slice containing only elements that satisfy the predicate.
// Returns nil if the input slice is nil.
//
// Example:
//
//	numbers := []int{1, 2, 3, 4, 5}
//	evens := dgo.Filter(numbers, func(x int) bool { return x%2 == 0 })
//	// evens = []int{2, 4}
func Filter[T any](slice []T, predicate func(T) bool) []T {
	if slice == nil {
		return nil
	}
	result := make([]T, 0, len(slice))
	for _, v := range slice {
		if predicate(v) {
			result = append(result, v)
		}
	}
	return result
}

// Reduce combines all elements into a single value using an accumulator function.
// The initial value is used as the starting accumulator.
//
// Example:
//
//	numbers := []int{1, 2, 3, 4}
//	sum := dgo.Reduce(numbers, 0, func(acc, x int) int { return acc + x })
//	// sum = 10
func Reduce[T, R any](slice []T, initial R, fn func(R, T) R) R {
	result := initial
	for _, v := range slice {
		result = fn(result, v)
	}
	return result
}

// ForEach executes a function for each element of the slice.
// Use for side effects; for transformation, use Map instead.
//
// Example:
//
//	dgo.ForEach(users, func(u User) { fmt.Println(u.Name) })
func ForEach[T any](slice []T, fn func(T)) {
	if slice == nil {
		return
	}
	for _, v := range slice {
		fn(v)
	}
}

// ============================================================================
// Index-Aware Variants
// ============================================================================

// MapWithIndex transforms each element with access to the index.
// Returns nil if the input slice is nil.
//
// Example:
//
//	words := []string{"a", "b", "c"}
//	indexed := dgo.MapWithIndex(words, func(i int, w string) string {
//	    return fmt.Sprintf("%d: %s", i, w)
//	})
//	// indexed = []string{"0: a", "1: b", "2: c"}
func MapWithIndex[T, U any](slice []T, fn func(int, T) U) []U {
	if slice == nil {
		return nil
	}
	result := make([]U, len(slice))
	for i, v := range slice {
		result[i] = fn(i, v)
	}
	return result
}

// FilterWithIndex filters elements with access to the index.
// Returns nil if the input slice is nil.
//
// Example:
//
//	numbers := []int{10, 20, 30, 40}
//	everyOther := dgo.FilterWithIndex(numbers, func(i int, x int) bool {
//	    return i%2 == 0
//	})
//	// everyOther = []int{10, 30}
func FilterWithIndex[T any](slice []T, predicate func(int, T) bool) []T {
	if slice == nil {
		return nil
	}
	result := make([]T, 0, len(slice))
	for i, v := range slice {
		if predicate(i, v) {
			result = append(result, v)
		}
	}
	return result
}

// ForEachWithIndex executes a function for each element with access to the index.
//
// Example:
//
//	dgo.ForEachWithIndex(users, func(i int, u User) {
//	    fmt.Printf("%d: %s\n", i, u.Name)
//	})
func ForEachWithIndex[T any](slice []T, fn func(int, T)) {
	if slice == nil {
		return
	}
	for i, v := range slice {
		fn(i, v)
	}
}

// ============================================================================
// Search/Predicate Functions
// ============================================================================

// Find returns the first element matching the predicate as an Option.
// Returns None if no element matches.
//
// Example:
//
//	user := dgo.Find(users, func(u User) bool { return u.ID == 42 })
//	if user.IsSome() {
//	    fmt.Println(user.Unwrap().Name)
//	}
func Find[T any](slice []T, predicate func(T) bool) Option[T] {
	for _, v := range slice {
		if predicate(v) {
			return Some(v)
		}
	}
	return None[T]()
}

// FindIndex returns the index of the first element matching the predicate as an Option.
// Returns None if no element matches.
//
// Example:
//
//	numbers := []int{1, 2, 3, 4, 5}
//	idx := dgo.FindIndex(numbers, func(x int) bool { return x > 3 })
//	// idx = Some(3)
//	idx.UnwrapOr(-1) // 3
func FindIndex[T any](slice []T, predicate func(T) bool) Option[int] {
	for i, v := range slice {
		if predicate(v) {
			return Some(i)
		}
	}
	return None[int]()
}

// Any returns true if any element satisfies the predicate.
// Returns false for empty slices.
//
// Example:
//
//	hasNegative := dgo.Any(numbers, func(x int) bool { return x < 0 })
func Any[T any](slice []T, predicate func(T) bool) bool {
	for _, v := range slice {
		if predicate(v) {
			return true
		}
	}
	return false
}

// All returns true if all elements satisfy the predicate.
// Returns true for empty slices (vacuous truth).
//
// Example:
//
//	allPositive := dgo.All(numbers, func(x int) bool { return x > 0 })
func All[T any](slice []T, predicate func(T) bool) bool {
	for _, v := range slice {
		if !predicate(v) {
			return false
		}
	}
	return true
}

// NoneMatch returns true if no elements satisfy the predicate.
// Returns true for empty slices.
// Note: This is equivalent to !Any(slice, predicate).
//
// Example:
//
//	noNegatives := dgo.NoneMatch(numbers, func(x int) bool { return x < 0 })
func NoneMatch[T any](slice []T, predicate func(T) bool) bool {
	for _, v := range slice {
		if predicate(v) {
			return false
		}
	}
	return true
}

// Contains returns true if the slice contains the given value.
// Uses == for comparison, so T must be comparable.
//
// Example:
//
//	hasValue := dgo.Contains(numbers, 42)
func Contains[T comparable](slice []T, value T) bool {
	for _, v := range slice {
		if v == value {
			return true
		}
	}
	return false
}

// Count returns the number of elements that satisfy the predicate.
//
// Example:
//
//	evenCount := dgo.Count(numbers, func(x int) bool { return x%2 == 0 })
func Count[T any](slice []T, predicate func(T) bool) int {
	count := 0
	for _, v := range slice {
		if predicate(v) {
			count++
		}
	}
	return count
}

// ============================================================================
// Advanced Functions
// ============================================================================

// FlatMap maps each element to a slice and flattens the results.
// Returns nil if the input slice is nil.
//
// Example:
//
//	words := []string{"hello", "world"}
//	chars := dgo.FlatMap(words, func(s string) []rune { return []rune(s) })
func FlatMap[T, U any](slice []T, fn func(T) []U) []U {
	if slice == nil {
		return nil
	}
	result := make([]U, 0, len(slice))
	for _, v := range slice {
		result = append(result, fn(v)...)
	}
	return result
}

// Flatten flattens a slice of slices into a single slice.
// Returns nil if the input slice is nil.
//
// Example:
//
//	nested := [][]int{{1, 2}, {3, 4}, {5}}
//	flat := dgo.Flatten(nested)
//	// flat = []int{1, 2, 3, 4, 5}
func Flatten[T any](slices [][]T) []T {
	if slices == nil {
		return nil
	}
	result := make([]T, 0)
	for _, s := range slices {
		result = append(result, s...)
	}
	return result
}

// Partition splits a slice into two based on a predicate.
// First return value contains matching elements, second contains non-matching.
// Returns (nil, nil) if the input slice is nil.
//
// Example:
//
//	evens, odds := dgo.Partition(numbers, func(x int) bool { return x%2 == 0 })
func Partition[T any](slice []T, predicate func(T) bool) ([]T, []T) {
	if slice == nil {
		return nil, nil
	}
	matching := make([]T, 0, len(slice))
	notMatching := make([]T, 0, len(slice))
	for _, v := range slice {
		if predicate(v) {
			matching = append(matching, v)
		} else {
			notMatching = append(notMatching, v)
		}
	}
	return matching, notMatching
}

// GroupBy groups elements by a key function.
// Returns nil if the input slice is nil.
//
// Example:
//
//	users := []User{{Name: "Alice", Age: 30}, {Name: "Bob", Age: 30}}
//	byAge := dgo.GroupBy(users, func(u User) int { return u.Age })
//	// byAge[30] = [{Alice 30}, {Bob 30}]
func GroupBy[T any, K comparable](slice []T, keyFn func(T) K) map[K][]T {
	if slice == nil {
		return nil
	}
	result := make(map[K][]T)
	for _, v := range slice {
		key := keyFn(v)
		result[key] = append(result[key], v)
	}
	return result
}

// Unique returns a slice with duplicate values removed.
// Preserves the order of first occurrence.
// Returns nil if the input slice is nil.
//
// Example:
//
//	unique := dgo.Unique([]int{1, 2, 2, 3, 1, 4})
//	// unique = []int{1, 2, 3, 4}
func Unique[T comparable](slice []T) []T {
	if slice == nil {
		return nil
	}
	seen := make(map[T]struct{}, len(slice))
	result := make([]T, 0, len(slice))
	for _, v := range slice {
		if _, ok := seen[v]; !ok {
			seen[v] = struct{}{}
			result = append(result, v)
		}
	}
	return result
}

// Reverse returns a reversed copy of the slice.
// Returns nil if the input slice is nil.
//
// Example:
//
//	reversed := dgo.Reverse([]int{1, 2, 3})
//	// reversed = []int{3, 2, 1}
func Reverse[T any](slice []T) []T {
	if slice == nil {
		return nil
	}
	result := make([]T, len(slice))
	for i, v := range slice {
		result[len(slice)-1-i] = v
	}
	return result
}

// ============================================================================
// Slice Manipulation
// ============================================================================

// Take returns the first n elements.
// If n > len(slice), returns the entire slice.
// Returns nil if the input slice is nil.
//
// Example:
//
//	first3 := dgo.Take(numbers, 3)
func Take[T any](slice []T, n int) []T {
	if slice == nil {
		return nil
	}
	if n <= 0 {
		return []T{}
	}
	if n > len(slice) {
		n = len(slice)
	}
	return slice[:n:n]
}

// Drop returns all but the first n elements.
// If n >= len(slice), returns an empty slice.
// Returns nil if the input slice is nil.
//
// Example:
//
//	rest := dgo.Drop(numbers, 2)
func Drop[T any](slice []T, n int) []T {
	if slice == nil {
		return nil
	}
	if n <= 0 {
		return slice[:]
	}
	if n >= len(slice) {
		return []T{}
	}
	return slice[n:]
}

// TakeWhile returns elements from the start while predicate is true.
// Returns nil if the input slice is nil.
//
// Example:
//
//	positives := dgo.TakeWhile(numbers, func(x int) bool { return x > 0 })
func TakeWhile[T any](slice []T, predicate func(T) bool) []T {
	if slice == nil {
		return nil
	}
	result := make([]T, 0, len(slice))
	for _, v := range slice {
		if !predicate(v) {
			break
		}
		result = append(result, v)
	}
	return result
}

// DropWhile drops elements from the start while predicate is true.
// Returns nil if the input slice is nil.
//
// Example:
//
//	afterNegatives := dgo.DropWhile(numbers, func(x int) bool { return x < 0 })
func DropWhile[T any](slice []T, predicate func(T) bool) []T {
	if slice == nil {
		return nil
	}
	i := 0
	for ; i < len(slice); i++ {
		if !predicate(slice[i]) {
			break
		}
	}
	result := make([]T, len(slice)-i)
	copy(result, slice[i:])
	return result
}

// Chunk splits a slice into chunks of specified size.
// The last chunk may have fewer elements.
// Returns nil if the input slice is nil.
// Panics if size <= 0.
//
// Example:
//
//	chunks := dgo.Chunk(numbers, 3)
//	// [[1,2,3], [4,5,6], [7]]
func Chunk[T any](slice []T, size int) [][]T {
	if slice == nil {
		return nil
	}
	if size <= 0 {
		panic("dgo.Chunk: size must be positive")
	}
	numChunks := (len(slice) + size - 1) / size
	result := make([][]T, 0, numChunks)
	for i := 0; i < len(slice); i += size {
		end := i + size
		if end > len(slice) {
			end = len(slice)
		}
		chunk := make([]T, end-i)
		copy(chunk, slice[i:end])
		result = append(result, chunk)
	}
	return result
}

// ZipSlices combines two slices into a slice of Pair values.
// The result length is the minimum of the two input lengths.
// Longer slice elements beyond the shorter length are ignored.
// Returns nil if either input slice is nil.
//
// Note: Renamed from Zip to avoid conflict with Option.Zip.
//
// Example:
//
//	a := []int{1, 2, 3}
//	b := []string{"a", "b"}
//	pairs := dgo.ZipSlices(a, b)
//	// pairs[0].First = 1, pairs[0].Second = "a"
//	// pairs[1].First = 2, pairs[1].Second = "b"
func ZipSlices[T, U any](a []T, b []U) []Pair[T, U] {
	if a == nil || b == nil {
		return nil
	}
	length := len(a)
	if len(b) < length {
		length = len(b)
	}
	result := make([]Pair[T, U], length)
	for i := 0; i < length; i++ {
		result[i] = Pair[T, U]{a[i], b[i]}
	}
	return result
}
