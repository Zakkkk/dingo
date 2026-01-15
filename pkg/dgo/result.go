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
//
// The Tag field is the source of truth for determining Ok vs Err state.
// Zero values are valid (e.g., Result[int, error] with Ok=0 is valid if Tag=ResultTagOk).
type Result[T, E any] struct {
	Tag ResultTag // Exported for pattern matching - source of truth
	Ok  T         // Exported for pattern matching (value, not pointer)
	Err E         // Exported for pattern matching (value, not pointer)
}

// Ok creates a successful Result containing the given value
func Ok[T, E any](value T) Result[T, E] {
	return Result[T, E]{Tag: ResultTagOk, Ok: value}
}

// Err creates a failed Result containing the given error
func Err[T, E any](err E) Result[T, E] {
	return Result[T, E]{Tag: ResultTagErr, Err: err}
}

// IsOk returns true if the Result is Ok
func (r Result[T, E]) IsOk() bool {
	return r.Tag == ResultTagOk
}

// IsErr returns true if the Result is Err
func (r Result[T, E]) IsErr() bool {
	return r.Tag == ResultTagErr
}

// Unwrap returns the Ok value, panics if Result is Err
// Deprecated: Use MustOk() for Go-style naming
func (r Result[T, E]) Unwrap() T {
	return r.MustOk()
}

// MustOk returns the Ok value, panics if Result is Err
// This follows Go's Must* convention for functions that panic on error
func (r Result[T, E]) MustOk() T {
	if r.Tag == ResultTagErr {
		panic("called MustOk on an Err value")
	}
	return r.Ok
}

// OkOr returns the Ok value or the provided default
// This follows Go's variant-based naming: "ok or default"
func (r Result[T, E]) OkOr(defaultValue T) T {
	if r.Tag == ResultTagOk {
		return r.Ok
	}
	return defaultValue
}

// UnwrapOr returns the Ok value or the provided default
// Deprecated: Use OkOr() for Go-style naming
func (r Result[T, E]) UnwrapOr(defaultValue T) T {
	return r.OkOr(defaultValue)
}

// UnwrapErr returns the Err value, panics if Result is Ok
// Deprecated: Use MustErr() for Go-style naming
func (r Result[T, E]) UnwrapErr() E {
	return r.MustErr()
}

// MustErr returns the Err value, panics if Result is Ok
// This follows Go's Must* convention for functions that panic on error
func (r Result[T, E]) MustErr() E {
	if r.Tag == ResultTagOk {
		panic("called MustErr on an Ok value")
	}
	return r.Err
}

// OkOrElse returns the Ok value or computes it from the error
// This follows Go's variant-based naming: "ok or else compute"
func (r Result[T, E]) OkOrElse(fn func(E) T) T {
	if r.Tag == ResultTagOk {
		return r.Ok
	}
	return fn(r.Err)
}

// UnwrapOrElse returns the Ok value or computes it from the error
// Deprecated: Use OkOrElse() for Go-style naming
func (r Result[T, E]) UnwrapOrElse(fn func(E) T) T {
	return r.OkOrElse(fn)
}

// Map transforms the Ok value using the provided function
func (r Result[T, E]) Map(fn func(T) T) Result[T, E] {
	if r.Tag == ResultTagOk {
		return Result[T, E]{Tag: ResultTagOk, Ok: fn(r.Ok)}
	}
	return r
}

// MapErr transforms the Err value using the provided function
func (r Result[T, E]) MapErr(fn func(E) E) Result[T, E] {
	if r.Tag == ResultTagErr {
		return Result[T, E]{Tag: ResultTagErr, Err: fn(r.Err)}
	}
	return r
}

// Filter returns the Result unchanged if Ok and predicate returns true, otherwise returns the original Result
func (r Result[T, E]) Filter(predicate func(T) bool) Result[T, E] {
	if r.Tag == ResultTagOk && predicate(r.Ok) {
		return r
	}
	return r
}

// AndThen chains operations that return a Result (flatMap)
func (r Result[T, E]) AndThen(fn func(T) Result[T, E]) Result[T, E] {
	if r.Tag == ResultTagOk {
		return fn(r.Ok)
	}
	return r
}

// OrElse chains error recovery operations
func (r Result[T, E]) OrElse(fn func(E) Result[T, E]) Result[T, E] {
	if r.Tag == ResultTagErr {
		return fn(r.Err)
	}
	return r
}

// And returns other if this Result is Ok, otherwise returns this Err
func (r Result[T, E]) And(other Result[T, E]) Result[T, E] {
	if r.Tag == ResultTagOk {
		return other
	}
	return r
}

// Or returns this Result if Ok, otherwise returns other
func (r Result[T, E]) Or(other Result[T, E]) Result[T, E] {
	if r.Tag == ResultTagOk {
		return r
	}
	return other
}

// Expect returns the Ok value or panics with the given message
func (r Result[T, E]) Expect(msg string) T {
	if r.Tag == ResultTagErr {
		panic(msg)
	}
	return r.Ok
}

// ExpectErr returns the Err value or panics with the given message
func (r Result[T, E]) ExpectErr(msg string) E {
	if r.Tag == ResultTagOk {
		panic(msg)
	}
	return r.Err
}

// OkPtr returns the Ok value as a pointer (nil if Err)
// Useful for optional-style access without panic
func (r Result[T, E]) OkPtr() *T {
	if r.Tag == ResultTagOk {
		return &r.Ok
	}
	return nil
}

// ErrPtr returns the Err value as a pointer (nil if Ok)
// Useful for optional-style access without panic
func (r Result[T, E]) ErrPtr() *E {
	if r.Tag == ResultTagErr {
		return &r.Err
	}
	return nil
}
