package calc

import (
	"testing"
	"time"
)

func TestProgress(t *testing.T) {
	tests := []struct {
		name              string
		downloaded, total int
		want              int
	}{
		{"total_zero", 10, 0, 0},               // total == 0 -> 0
		{"zero_downloaded", 0, 100, 0},         // nothing downloaded
		{"half", 50, 100, 50},                  // exact half
		{"one_third", 1, 3, 33},                // 33.333 -> 33
		{"two_thirds", 2, 3, 67},               // 66.666 -> 67
		{"one_sixth", 1, 6, 17},                // 16.666 -> 17
		{"exact_100", 100, 100, 100},           // 100%
		{"over_100", 150, 100, 150},            // >100% not clamped
		{"negative_downloaded", -50, 100, -50}, // negative handled as-is
	}

	for _, tc := range tests {
		tc := tc // capture range variable
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := Progress(tc.downloaded, tc.total)
			if got != tc.want {
				t.Fatalf("Progress(%d, %d) = %d; want %d", tc.downloaded, tc.total, got, tc.want)
			}
		})
	}
}

func approxEqual(a, b, tol time.Duration) bool {
	if a < b {
		return b-a <= tol
	}
	return a-b <= tol
}

func TestETA(t *testing.T) {
	tests := []struct {
		name              string
		downloaded, total int
		elapsed           time.Duration // how long ago started was
	}{
		{"total_zero", 10, 0, 1 * time.Second},      // total == 0 -> expect 0
		{"half", 50, 100, 2 * time.Second},          // elapsed 2s, ratio 100/50=2, eta = 2s*(2-1)=2s
		{"quarter", 25, 100, 4 * time.Second},       // elapsed 4s, ratio 4, eta = 4s*(4-1)=12s
		{"over_100", 150, 100, 2 * time.Second},     // downloaded>total -> negative ETA expected
		{"small_download", 1, 100, 1 * time.Second}, // produces a large ETA, check magnitude
	}

	const tolerance = 50 * time.Millisecond

	for _, tc := range tests {
		tc := tc // capture
		t.Run(tc.name, func(t *testing.T) {
			// Set started to a fixed elapsed duration in the past.
			started := time.Now().Add(-tc.elapsed)

			got := ETA(tc.downloaded, tc.total, started)

			if tc.total == 0 {
				if got != 0 {
					t.Fatalf("expected 0 when total==0, got %v", got)
				}
				return
			}

			// Compute expected using the same formula but with our known 'elapsed' value.
			expected := time.Duration(float64(tc.elapsed) * (float64(tc.total)/float64(tc.downloaded) - 1))

			if !approxEqual(got, expected, tolerance) {
				t.Fatalf("ETA(downloaded=%d, total=%d, started=%v) = %v; want approx %v (tol %v)",
					tc.downloaded, tc.total, tc.elapsed, got, expected, tolerance)
			}
		})
	}
}
