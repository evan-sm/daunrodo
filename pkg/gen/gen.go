// Package gen provides utility functions for generating values.
package gen

import (
	"fmt"

	"github.com/google/uuid"
)

const sep = "|"

// Key generates a key based on the provided strings a and b.
func Key(a, b string) string {
	return fmt.Sprintf("%s%s%s", a, sep, b)
}

// UUIDv5 generates a UUIDv5 based on the provided strings a and b.
func UUIDv5(a, b string) string {
	key := Key(a, b)

	return uuid.NewSHA1(uuid.NameSpaceURL, []byte(key)).String()
}
