// Package dgo provides Dingo's core types: Result and Option
package dgo

// OptionTag represents the state of an Option value
type OptionTag uint8

const (
	// OptionTagSome indicates the Option contains a value
	OptionTagSome OptionTag = iota
	// OptionTagNone indicates the Option is empty
	OptionTagNone
)

// Option represents an optional value that may or may not be present
// It is a type-safe alternative to nil pointers
//
// Usage:
//
//	func findUser(id int) Option[User] {
//	    if user, ok := users[id]; ok {
//	        return Some(user)
//	    }
//	    return None[User]()
//	}
type Option[T any] struct {
	Tag   OptionTag  // Exported for pattern matching
	Some  *T         // Exported for pattern matching
}

// Some creates an Option containing the given value
func Some[T any](value T) Option[T] {
	return Option[T]{Tag: OptionTagSome, Some: &value}
}

// None creates an empty Option
func None[T any]() Option[T] {
	return Option[T]{Tag: OptionTagNone}
}

// IsSome returns true if the Option contains a value
func (o Option[T]) IsSome() bool {
	return o.Tag == OptionTagSome
}

// IsNone returns true if the Option is empty
func (o Option[T]) IsNone() bool {
	return o.Tag == OptionTagNone
}

// Unwrap returns the contained value, panics if None
// Deprecated: Use MustOk() for Go-style naming
func (o Option[T]) Unwrap() T {
	return o.MustOk()
}

// MustOk returns the contained value, panics if None
// This follows Go's Must* convention for functions that panic on error
func (o Option[T]) MustOk() T {
	if o.Tag == OptionTagNone {
		panic("called MustOk on a None value")
	}
	return *o.Some
}

// UnwrapOr returns the contained value or the provided default
func (o Option[T]) UnwrapOr(defaultValue T) T {
	if o.Tag == OptionTagSome && o.Some != nil {
		return *o.Some
	}
	return defaultValue
}

// UnwrapOrElse returns the contained value or computes it from the function
func (o Option[T]) UnwrapOrElse(fn func() T) T {
	if o.Tag == OptionTagSome && o.Some != nil {
		return *o.Some
	}
	return fn()
}

// Map transforms the contained value using the provided function
func (o Option[T]) Map(fn func(T) T) Option[T] {
	if o.Tag == OptionTagSome && o.Some != nil {
		newVal := fn(*o.Some)
		return Option[T]{Tag: OptionTagSome, Some: &newVal}
	}
	return o
}

// Filter returns the Option unchanged if Some and predicate returns true, otherwise returns None
func (o Option[T]) Filter(predicate func(T) bool) Option[T] {
	if o.Tag == OptionTagSome && o.Some != nil && predicate(*o.Some) {
		return o
	}
	return None[T]()
}

// AndThen chains operations that return an Option (flatMap)
func (o Option[T]) AndThen(fn func(T) Option[T]) Option[T] {
	if o.Tag == OptionTagSome && o.Some != nil {
		return fn(*o.Some)
	}
	return o
}

// OrElse returns this Option if Some, otherwise calls fn
func (o Option[T]) OrElse(fn func() Option[T]) Option[T] {
	if o.Tag == OptionTagSome {
		return o
	}
	return fn()
}

// And returns other if this Option is Some, otherwise returns None
func (o Option[T]) And(other Option[T]) Option[T] {
	if o.Tag == OptionTagSome {
		return other
	}
	return o
}

// Or returns this Option if Some, otherwise returns other
func (o Option[T]) Or(other Option[T]) Option[T] {
	if o.Tag == OptionTagSome {
		return o
	}
	return other
}

// Expect returns the contained value or panics with the given message
func (o Option[T]) Expect(msg string) T {
	if o.Tag == OptionTagNone {
		panic(msg)
	}
	return *o.Some
}

// Take takes the value out of the Option, leaving None in its place
// Note: Due to Go's value semantics, this returns the value and a new None Option
func (o Option[T]) Take() (T, Option[T]) {
	if o.Tag == OptionTagSome && o.Some != nil {
		return *o.Some, None[T]()
	}
	var zero T
	return zero, None[T]()
}

// Replace replaces the actual value with the given one, returning the old value
func (o Option[T]) Replace(value T) (Option[T], T) {
	if o.Tag == OptionTagSome && o.Some != nil {
		old := *o.Some
		return Some(value), old
	}
	var zero T
	return Some(value), zero
}

// Zip combines two Options into an Option of a pair
// Returns None if either Option is None
func Zip[T, U any](a Option[T], b Option[U]) Option[struct{ First T; Second U }] {
	if a.Tag == OptionTagSome && b.Tag == OptionTagSome && a.Some != nil && b.Some != nil {
		return Some(struct{ First T; Second U }{*a.Some, *b.Some})
	}
	return None[struct{ First T; Second U }]()
}

// OkOr converts an Option to a Result, using the provided error if None
func (o Option[T]) OkOr(err error) Result[T, error] {
	if o.Tag == OptionTagSome && o.Some != nil {
		return Ok[T, error](*o.Some)
	}
	return Err[T, error](err)
}

// OkOrElse converts an Option to a Result, computing the error if None
func (o Option[T]) OkOrElse(fn func() error) Result[T, error] {
	if o.Tag == OptionTagSome && o.Some != nil {
		return Ok[T, error](*o.Some)
	}
	return Err[T, error](fn())
}
