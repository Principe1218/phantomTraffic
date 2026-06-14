package safety

import (
	"testing"
	"time"

	"github.com/Principe1218/phantomTraffic/internal/clock"
)

// tokenBucket refills from clk.Since(last), never wall-clock, so a fake clock
// makes the rate math exact and a frozen clock means zero refill.
func TestTokenBucketRefillFromClock(t *testing.T) {
	start := time.Date(2026, 6, 13, 9, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		rps     float64
		burst   float64
		advance time.Duration
		draws   int  // tryTake calls BEFORE the advance, after draining
		wantOK  bool // result of one tryTake AFTER the advance
	}{
		{name: "full bucket allows first take", rps: 10, burst: 10, advance: 0, draws: 0, wantOK: true},
		{name: "drained bucket denies with no time passed", rps: 10, burst: 1, advance: 0, draws: 1, wantOK: false},
		{name: "refill one token after 100ms at 10rps", rps: 10, burst: 1, advance: 100 * time.Millisecond, draws: 1, wantOK: true},
		{name: "no refill before a full interval", rps: 10, burst: 1, advance: 50 * time.Millisecond, draws: 1, wantOK: false},
		{name: "refill caps at burst", rps: 10, burst: 2, advance: time.Hour, draws: 2, wantOK: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			clk := clock.NewFake(start)
			b := newTokenBucket(clk, tc.rps, tc.burst)
			for i := 0; i < tc.draws; i++ {
				if !b.tryTake(1) {
					t.Fatalf("setup draw %d unexpectedly denied", i)
				}
			}
			clk.Advance(tc.advance)
			if got := b.tryTake(1); got != tc.wantOK {
				t.Fatalf("tryTake after advance %s = %v, want %v", tc.advance, got, tc.wantOK)
			}
		})
	}
}

// A zero/negative rps bucket is treated as unlimited (never blocks).
func TestTokenBucketUnlimited(t *testing.T) {
	clk := clock.NewFake(time.Date(2026, 6, 13, 9, 0, 0, 0, time.UTC))
	b := newTokenBucket(clk, 0, 0)
	for i := 0; i < 1000; i++ {
		if !b.tryTake(1) {
			t.Fatalf("unlimited bucket denied take %d", i)
		}
	}
}
