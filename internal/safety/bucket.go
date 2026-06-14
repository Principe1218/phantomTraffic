package safety

import (
	"time"

	"github.com/Principe1218/phantomTraffic/internal/clock"
)

// tokenBucket is a clock-driven token bucket. Refill is computed lazily from
// clk.Since(last) on each take, so the injected clock is the sole time
// authority: a fake clock makes the math deterministic and a frozen clock
// (paused run) performs no refill. Not goroutine-safe; the Limiter holds the
// lock.
type tokenBucket struct {
	clk    clock.Clock
	rps    float64   // refill rate, tokens per second; <= 0 => unlimited
	burst  float64   // maximum stored tokens
	tokens float64   // current stored tokens
	last   time.Time // last refill instant (clock time)
}

func newTokenBucket(clk clock.Clock, rps, burst float64) *tokenBucket {
	return &tokenBucket{clk: clk, rps: rps, burst: burst, tokens: burst, last: clk.Now()}
}

// refill adds tokens accrued since the last call, capped at burst.
func (b *tokenBucket) refill() {
	now := b.clk.Now()
	elapsed := b.clk.Since(b.last)
	b.last = now
	if elapsed <= 0 {
		return
	}
	b.tokens += elapsed.Seconds() * b.rps
	if b.tokens > b.burst {
		b.tokens = b.burst
	}
}

// tryTake removes n tokens if available and reports success. A non-positive
// rps means the bucket is unlimited.
func (b *tokenBucket) tryTake(n float64) bool {
	if b.rps <= 0 {
		return true
	}
	b.refill()
	if b.tokens < n {
		return false
	}
	b.tokens -= n
	return true
}
