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
	tag   OptionTag
	value *T
}

// Some creates an Option containing the given value
func Some[T any](value T) Option[T] {
	return Option[T]{tag: OptionTagSome, value: &value}
}

// None creates an empty Option
func None[T any]() Option[T] {
	return Option[T]{tag: OptionTagNone}
}

// IsSome returns true if the Option contains a value
func (o Option[T]) IsSome() bool {
	return o.tag == OptionTagSome
}

// IsNone returns true if the Option is empty
func (o Option[T]) IsNone() bool {
	return o.tag == OptionTagNone
}

// Unwrap returns the contained value, panics if None
func (o Option[T]) Unwrap() T {
	if o.tag == OptionTagNone {
		panic("called Unwrap on a None value")
	}
	return *o.value
}

// UnwrapOr returns the contained value or the provided default
func (o Option[T]) UnwrapOr(defaultValue T) T {
	if o.tag == OptionTagSome && o.value != nil {
		return *o.value
	}
	return defaultValue
}

// UnwrapOrElse returns the contained value or computes it from the function
func (o Option[T]) UnwrapOrElse(fn func() T) T {
	if o.tag == OptionTagSome && o.value != nil {
		return *o.value
	}
	return fn()
}

// Map transforms the contained value using the provided function
func (o Option[T]) Map(fn func(T) T) Option[T] {
	if o.tag == OptionTagSome && o.value != nil {
		newVal := fn(*o.value)
		return Option[T]{tag: OptionTagSome, value: &newVal}
	}
	return o
}

// Filter returns the Option unchanged if Some and predicate returns true, otherwise returns None
func (o Option[T]) Filter(predicate func(T) bool) Option[T] {
	if o.tag == OptionTagSome && o.value != nil && predicate(*o.value) {
		return o
	}
	return None[T]()
}

// AndThen chains operations that return an Option (flatMap)
func (o Option[T]) AndThen(fn func(T) Option[T]) Option[T] {
	if o.tag == OptionTagSome && o.value != nil {
		return fn(*o.value)
	}
	return o
}

// OrElse returns this Option if Some, otherwise calls fn
func (o Option[T]) OrElse(fn func() Option[T]) Option[T] {
	if o.tag == OptionTagSome {
		return o
	}
	return fn()
}

// And returns other if this Option is Some, otherwise returns None
func (o Option[T]) And(other Option[T]) Option[T] {
	if o.tag == OptionTagSome {
		return other
	}
	return o
}

// Or returns this Option if Some, otherwise returns other
func (o Option[T]) Or(other Option[T]) Option[T] {
	if o.tag == OptionTagSome {
		return o
	}
	return other
}

// Expect returns the contained value or panics with the given message
func (o Option[T]) Expect(msg string) T {
	if o.tag == OptionTagNone {
		panic(msg)
	}
	return *o.value
}

// Take takes the value out of the Option, leaving None in its place
// Note: Due to Go's value semantics, this returns the value and a new None Option
func (o Option[T]) Take() (T, Option[T]) {
	if o.tag == OptionTagSome && o.value != nil {
		return *o.value, None[T]()
	}
	var zero T
	return zero, None[T]()
}

// Replace replaces the actual value with the given one, returning the old value
func (o Option[T]) Replace(value T) (Option[T], T) {
	if o.tag == OptionTagSome && o.value != nil {
		old := *o.value
		return Some(value), old
	}
	var zero T
	return Some(value), zero
}

// Zip combines two Options into an Option of a pair
// Returns None if either Option is None
func Zip[T, U any](a Option[T], b Option[U]) Option[struct{ First T; Second U }] {
	if a.tag == OptionTagSome && b.tag == OptionTagSome && a.value != nil && b.value != nil {
		return Some(struct{ First T; Second U }{*a.value, *b.value})
	}
	return None[struct{ First T; Second U }]()
}

// OkOr converts an Option to a Result, using the provided error if None
func (o Option[T]) OkOr(err error) Result[T, error] {
	if o.tag == OptionTagSome && o.value != nil {
		return Ok[T, error](*o.value)
	}
	return Err[T, error](err)
}

// OkOrElse converts an Option to a Result, computing the error if None
func (o Option[T]) OkOrElse(fn func() error) Result[T, error] {
	if o.tag == OptionTagSome && o.value != nil {
		return Ok[T, error](*o.value)
	}
	return Err[T, error](fn())
}
