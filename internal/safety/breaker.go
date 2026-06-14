package safety

import (
	"sync"
	"time"

	"github.com/Principe1218/phantomTraffic/internal/clock"
)

// BreakerState is the lifecycle state of a per-target circuit breaker.
type BreakerState uint8

const (
	BreakerClosed BreakerState = iota
	BreakerOpen
	BreakerHalfOpen
)

// Breaker is a self-healing per-target circuit breaker over the injected
// clock: it opens after threshold consecutive failures, admits a single
// probe after a clock-driven cooldown (half-open), and closes again on a
// successful probe. The dispatcher consults Allow() to skip open targets so
// siblings keep running.
type Breaker struct {
	clk       clock.Clock
	threshold int
	cooldown  time.Duration

	mu       sync.Mutex
	state    BreakerState
	fails    int
	openedAt time.Time
}

// NewBreaker returns a closed breaker. threshold is the number of consecutive
// failures that opens it; cooldown is the wait before a half-open probe.
func NewBreaker(clk clock.Clock, threshold int, cooldown time.Duration) *Breaker {
	if threshold < 1 {
		threshold = 1
	}
	return &Breaker{clk: clk, threshold: threshold, cooldown: cooldown, state: BreakerClosed}
}

// Allow reports whether a request may proceed. Closed and HalfOpen admit;
// Open admits exactly one probe once the cooldown has elapsed, transitioning
// to HalfOpen.
func (b *Breaker) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	switch b.state {
	case BreakerClosed:
		return true
	case BreakerHalfOpen:
		// A probe is already in flight; admit only one at a time.
		return false
	case BreakerOpen:
		if b.clk.Since(b.openedAt) >= b.cooldown {
			b.state = BreakerHalfOpen
			return true
		}
		return false
	default:
		return false
	}
}

// RecordSuccess closes the breaker and clears the failure streak.
func (b *Breaker) RecordSuccess() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.state = BreakerClosed
	b.fails = 0
}

// RecordFailure increments the streak; it opens the breaker once the threshold
// is met, or immediately on a half-open probe failure (restarting cooldown).
func (b *Breaker) RecordFailure() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.state == BreakerHalfOpen {
		b.open()
		return
	}
	b.fails++
	if b.fails >= b.threshold {
		b.open()
	}
}

// ForceOpen opens the breaker regardless of failure history (TargetsDisable).
func (b *Breaker) ForceOpen() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.open()
}

// State returns the current breaker state.
func (b *Breaker) State() BreakerState {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.state
}

// open transitions to Open and (re)starts the cooldown. Caller holds b.mu.
func (b *Breaker) open() {
	b.state = BreakerOpen
	b.openedAt = b.clk.Now()
}
