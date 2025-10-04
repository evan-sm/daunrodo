// Package ptr provides utility functions for working with pointers.
package ptr

// Deref returns the value pointed to by the given pointer.
func Deref[T any](ptr *T) T {
	if ptr == nil {
		var zero T

		return zero
	}

	return *ptr
}

// Of returns a pointer to the given value.
func Of[T any](s T) *T { return &s }
