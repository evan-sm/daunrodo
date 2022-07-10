package maths

import (
	"math"
)

func RoundFloat64ToInt(v float64) int {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0

	}
	return int(math.Round(v))
}
