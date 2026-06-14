package safety

import (
	"sync"
	"sync/atomic"
)

// Tripwire is the run-level, highest-precedence safety latch. It has no
// auto-reset: once tripped it stays tripped until an operator re-arms by
// building a fresh Tripwire. It fires on total-request-budget exhaustion
// (CountRequest) or on an explicit Trip (panic-storm, sustained failure).
// Goroutine-safe: count/tripped are atomics; reason is mutex-guarded.
type Tripwire struct {
	budget  int64 // <= 0 => unlimited
	count   atomic.Int64
	tripped atomic.Bool

	mu     sync.Mutex
	reason string
}

// NewTripwire builds a tripwire that trips when the request count exceeds
// totalRequestBudget. A non-positive budget means unlimited.
func NewTripwire(totalRequestBudget int64) *Tripwire {
	return &Tripwire{budget: totalRequestBudget}
}

// CountRequest increments the request counter, trips when the count exceeds
// the budget, and reports the latched state.
func (t *Tripwire) CountRequest() bool {
	n := t.count.Add(1)
	if t.budget > 0 && n > t.budget {
		t.Trip("total request budget exhausted")
	}
	return t.Tripped()
}

// Trip latches the tripwire with the given reason. The first reason wins.
func (t *Tripwire) Trip(reason string) {
	if t.tripped.CompareAndSwap(false, true) {
		t.mu.Lock()
		t.reason = reason
		t.mu.Unlock()
	}
}

// Tripped reports whether the tripwire is latched.
func (t *Tripwire) Tripped() bool { return t.tripped.Load() }

// Reason returns the latched reason, or "" if not tripped.
func (t *Tripwire) Reason() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.reason
}
