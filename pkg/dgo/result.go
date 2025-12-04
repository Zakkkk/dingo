// Package dgo provides Dingo's core types: Result and Option
// These types use Go 1.18+ generics for type-safe error handling and optional values
package dgo

// ResultTag represents the state of a Result value
type ResultTag uint8

const (
	// ResultTagOk indicates the Result contains a success value
	ResultTagOk ResultTag = iota
	// ResultTagErr indicates the Result contains an error value
	ResultTagErr
)

// Result represents a value that can either be successful (Ok) or contain an error (Err)
// It is a type-safe alternative to Go's (T, error) pattern
//
// Usage:
//
//	func divide(a, b int) Result[int, string] {
//	    if b == 0 {
//	        return Err[int, string]("division by zero")
//	    }
//	    return Ok[int, string](a / b)
//	}
type Result[T, E any] struct {
	tag ResultTag
	ok  *T
	err *E
}

// Ok creates a successful Result containing the given value
func Ok[T, E any](value T) Result[T, E] {
	return Result[T, E]{tag: ResultTagOk, ok: &value}
}

// Err creates a failed Result containing the given error
func Err[T, E any](err E) Result[T, E] {
	return Result[T, E]{tag: ResultTagErr, err: &err}
}

// IsOk returns true if the Result is Ok
func (r Result[T, E]) IsOk() bool {
	return r.tag == ResultTagOk
}

// IsErr returns true if the Result is Err
func (r Result[T, E]) IsErr() bool {
	return r.tag == ResultTagErr
}

// Unwrap returns the Ok value, panics if Result is Err
func (r Result[T, E]) Unwrap() T {
	if r.tag == ResultTagErr {
		panic("called Unwrap on an Err value")
	}
	return *r.ok
}

// UnwrapOr returns the Ok value or the provided default
func (r Result[T, E]) UnwrapOr(defaultValue T) T {
	if r.tag == ResultTagOk && r.ok != nil {
		return *r.ok
	}
	return defaultValue
}

// UnwrapErr returns the Err value, panics if Result is Ok
func (r Result[T, E]) UnwrapErr() E {
	if r.tag == ResultTagOk {
		panic("called UnwrapErr on an Ok value")
	}
	return *r.err
}

// UnwrapOrElse returns the Ok value or computes it from the error
func (r Result[T, E]) UnwrapOrElse(fn func(E) T) T {
	if r.tag == ResultTagOk && r.ok != nil {
		return *r.ok
	}
	return fn(*r.err)
}

// Map transforms the Ok value using the provided function
func (r Result[T, E]) Map(fn func(T) T) Result[T, E] {
	if r.tag == ResultTagOk && r.ok != nil {
		newVal := fn(*r.ok)
		return Result[T, E]{tag: ResultTagOk, ok: &newVal}
	}
	return r
}

// MapErr transforms the Err value using the provided function
func (r Result[T, E]) MapErr(fn func(E) E) Result[T, E] {
	if r.tag == ResultTagErr && r.err != nil {
		newErr := fn(*r.err)
		return Result[T, E]{tag: ResultTagErr, err: &newErr}
	}
	return r
}

// Filter returns the Result unchanged if Ok and predicate returns true, otherwise returns the original error
func (r Result[T, E]) Filter(predicate func(T) bool) Result[T, E] {
	if r.tag == ResultTagOk && r.ok != nil && predicate(*r.ok) {
		return r
	}
	return r
}

// AndThen chains operations that return a Result (flatMap)
func (r Result[T, E]) AndThen(fn func(T) Result[T, E]) Result[T, E] {
	if r.tag == ResultTagOk && r.ok != nil {
		return fn(*r.ok)
	}
	return r
}

// OrElse chains error recovery operations
func (r Result[T, E]) OrElse(fn func(E) Result[T, E]) Result[T, E] {
	if r.tag == ResultTagErr && r.err != nil {
		return fn(*r.err)
	}
	return r
}

// And returns other if this Result is Ok, otherwise returns this Err
func (r Result[T, E]) And(other Result[T, E]) Result[T, E] {
	if r.tag == ResultTagOk {
		return other
	}
	return r
}

// Or returns this Result if Ok, otherwise returns other
func (r Result[T, E]) Or(other Result[T, E]) Result[T, E] {
	if r.tag == ResultTagOk {
		return r
	}
	return other
}

// Expect returns the Ok value or panics with the given message
func (r Result[T, E]) Expect(msg string) T {
	if r.tag == ResultTagErr {
		panic(msg)
	}
	return *r.ok
}

// ExpectErr returns the Err value or panics with the given message
func (r Result[T, E]) ExpectErr(msg string) E {
	if r.tag == ResultTagOk {
		panic(msg)
	}
	return *r.err
}

// OkOr returns the Ok value as an Option
func (r Result[T, E]) OkOr() *T {
	if r.tag == ResultTagOk {
		return r.ok
	}
	return nil
}

// ErrOr returns the Err value as an Option
func (r Result[T, E]) ErrOr() *E {
	if r.tag == ResultTagErr {
		return r.err
	}
	return nil
}
