package engine

import (
	"testing"
	"time"

	"github.com/Principe1218/phantomTraffic/internal/rng"
)

// TestBackoffDelay verifies the exponential-with-full-jitter schedule:
//
//	exp     = min(max, base << attempt)
//	delay   = time.Duration(r.Float64() * float64(exp))
//
// attempt is 0-based. The fake rng scripts Float64 draws, so the jitter is exact.
func TestBackoffDelay(t *testing.T) {
	base := 100 * time.Millisecond
	max := 2 * time.Second

	tests := []struct {
		name    string
		attempt int
		float   float64
		want    time.Duration
	}{
		// attempt 0: exp = 100ms; jitter 0.0 => 0
		{name: "attempt0 zero jitter", attempt: 0, float: 0.0, want: 0},
		// attempt 0: exp = 100ms; jitter 1.0 => full 100ms
		{name: "attempt0 full jitter", attempt: 0, float: 1.0, want: 100 * time.Millisecond},
		// attempt 1: exp = 200ms; jitter 0.5 => 100ms
		{name: "attempt1 half jitter", attempt: 1, float: 0.5, want: 100 * time.Millisecond},
		// attempt 2: exp = 400ms; jitter 0.25 => 100ms
		{name: "attempt2 quarter jitter", attempt: 2, float: 0.25, want: 100 * time.Millisecond},
		// attempt 5: base<<5 = 3200ms but capped at max=2000ms; jitter 1.0 => 2000ms
		{name: "attempt5 capped full jitter", attempt: 5, float: 1.0, want: 2 * time.Second},
		// attempt 5: capped at 2000ms; jitter 0.5 => 1000ms
		{name: "attempt5 capped half jitter", attempt: 5, float: 0.5, want: 1 * time.Second},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := rng.NewFake(rng.FakeScript{Floats: []float64{tc.float}})
			got := backoffDelay(tc.attempt, base, max, r)
			if got != tc.want {
				t.Fatalf("backoffDelay(%d, %v, %v, %.2f) = %v, want %v",
					tc.attempt, base, max, tc.float, got, tc.want)
			}
		})
	}
}
