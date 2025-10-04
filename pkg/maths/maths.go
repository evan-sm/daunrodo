// Package maths provides utility functions for mathematical operations.
package maths

import (
	"math"
)

// RoundFloat64ToInt rounds the given float64 to the nearest integer.
func RoundFloat64ToInt(v float64) int {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}

	return int(math.Round(v))
}
