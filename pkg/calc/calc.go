package calc

import (
	"math"
	"time"
)

// Progress calculates the percentage for a given pair of numbers.
func Progress(downloaded, total int) int {
	if total > 0 {
		return int(math.Round(float64(downloaded) / float64(total) * 100))
	}
	return 0
}

// ETA calculates the estimated time of arrival.
func ETA(downloaded, total int, started time.Time) time.Duration {
	if total > 0 {
		downloaded := float64(downloaded)
		total := float64(total)
		elapsed := time.Since(started)
		eta := time.Duration(float64(elapsed) * (total/downloaded - 1))
		return eta
	}
	return 0
}
