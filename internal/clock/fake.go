package clock

import (
	"context"
	"sort"
	"sync"
	"time"
)

// FakeClock is a deterministic Clock for tests. Time only moves when Advance or
// Set is called; timers fire and Sleep unblocks exactly when logical time passes
// their deadline. It is safe for concurrent use (a Sleep goroutine vs. an Advance
// caller). This is the determinism keystone of the test suite (design §1).
type FakeClock struct {
	mu      sync.Mutex
	now     time.Time
	waiters []*fakeWaiter
}

// fakeWaiter is a registered timer, AfterFunc, or Sleep. Exactly one of fn
// (called synchronously in deadline order, without holding mu) or a channel
// send (ch, for timers and Sleep) happens.
type fakeWaiter struct {
	deadline time.Time
	ch       chan time.Time
	fn       func()
	fired    bool
	stopped  bool
}

// NewFake returns a FakeClock whose current time is t.
func NewFake(t time.Time) *FakeClock { return &FakeClock{now: t} }

func (c *FakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *FakeClock) Since(t time.Time) time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now.Sub(t)
}

// addWaiter registers a waiter; caller MUST hold c.mu. A non-positive duration
// fires immediately so a zero-delay timer/AfterFunc behaves like the real clock.
// Returns the fn to call (if any) after the caller releases the lock; returns
// nil when no post-lock call is needed (channel-based waiters send under the lock).
func (c *FakeClock) addWaiter(d time.Duration, fn func()) (*fakeWaiter, func()) {
	w := &fakeWaiter{deadline: c.now.Add(d), ch: make(chan time.Time, 1), fn: fn}
	if d <= 0 {
		c.fire(w) // marks fired; sends on channel if fn == nil
		if fn != nil {
			return w, fn // caller must invoke fn() after releasing mu
		}
		return w, nil
	}
	c.waiters = append(c.waiters, w)
	return w, nil
}

// fire delivers the waiter exactly once; caller MUST hold c.mu.
// Channel-based waiters (Timer/Sleep) receive on their buffered channel.
// AfterFunc waiters are NOT called here — fireDue collects them and
// calls fn() after releasing the lock to avoid re-entrance deadlocks.
func (c *FakeClock) fire(w *fakeWaiter) {
	if w.fired || w.stopped {
		return
	}
	w.fired = true
	if w.fn == nil {
		w.ch <- w.deadline
	}
	// fn != nil: caller (fireDue) is responsible for invoking fn().
}

// fakeTimer adapts a waiter to the Timer interface.
type fakeTimer struct {
	c *FakeClock
	w *fakeWaiter
}

func (t *fakeTimer) C() <-chan time.Time { return t.w.ch }

func (t *fakeTimer) Stop() bool {
	t.c.mu.Lock()
	defer t.c.mu.Unlock()
	if t.w.fired || t.w.stopped {
		return false
	}
	t.w.stopped = true
	return true
}

func (c *FakeClock) NewTimer(d time.Duration) Timer {
	c.mu.Lock()
	defer c.mu.Unlock()
	w, _ := c.addWaiter(d, nil)
	return &fakeTimer{c: c, w: w}
}

// AfterFunc schedules f to run after duration d. A callback registered via
// AfterFunc may call back into the clock (e.g. schedule a new timer); the new
// timer fires on a SUBSEQUENT Advance, not within the current one (snapshot
// semantics).
func (c *FakeClock) AfterFunc(d time.Duration, f func()) Timer {
	c.mu.Lock()
	w, immediate := c.addWaiter(d, f)
	c.mu.Unlock()
	if immediate != nil {
		immediate() // zero-duration: call synchronously without holding mu
	}
	return &fakeTimer{c: c, w: w}
}

// Sleep blocks until logical time reaches now+d (via Advance/Set) or ctx is
// canceled, returning ctx.Err() on cancel (design §2). A cancel marks the
// waiter stopped so a later Advance does not deliver to an abandoned channel.
func (c *FakeClock) Sleep(ctx context.Context, d time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if d <= 0 {
		return nil
	}
	c.mu.Lock()
	w, _ := c.addWaiter(d, nil)
	c.mu.Unlock()
	select {
	case <-w.ch:
		return nil
	case <-ctx.Done():
		c.mu.Lock()
		w.stopped = true
		c.mu.Unlock()
		return ctx.Err()
	}
}

// Advance moves logical time forward by d and fires every waiter whose deadline
// is now at or before the new time, in ascending deadline order.
// AfterFunc callbacks are invoked synchronously after the lock is released.
// A callback registered via AfterFunc may call back into the clock (e.g. schedule
// a new timer); the new timer fires on a SUBSEQUENT Advance, not within the
// current one (snapshot semantics).
func (c *FakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
	c.fireDue() // releases and re-acquires mu around AfterFunc callbacks
}

// Set moves logical time to t. If t is in the future it fires due waiters in
// order; if t is in the past it only rewinds Now (no waiter fires on a rewind).
// AfterFunc callbacks are invoked synchronously after the lock is released.
func (c *FakeClock) Set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = t
	c.fireDue() // releases and re-acquires mu around AfterFunc callbacks
}

// fireDue fires all unstopped waiters at/before c.now in deadline order;
// caller must hold c.mu on entry; this releases and re-acquires c.mu around
// AfterFunc callbacks and returns with c.mu held.
//
// Channel-based waiters (Timer/Sleep) are fired under the lock (buffered send,
// non-blocking). AfterFunc waiters are marked fired under the lock, then their
// fn() is called synchronously in deadline order AFTER releasing the lock. This
// prevents re-entrance deadlocks if a callback calls back into the clock (e.g.
// schedules another timer). Newly scheduled timers fire on a subsequent Advance
// (snapshot semantics — we do not loop to catch them in the same pass).
func (c *FakeClock) fireDue() {
	var due, rest []*fakeWaiter
	for _, w := range c.waiters {
		if w.stopped {
			continue
		}
		if !w.deadline.After(c.now) {
			due = append(due, w)
		} else {
			rest = append(rest, w)
		}
	}
	c.waiters = rest
	sort.SliceStable(due, func(i, j int) bool { return due[i].deadline.Before(due[j].deadline) })

	// Collect AfterFunc callbacks to invoke after releasing the lock.
	var callbacks []func()
	for _, w := range due {
		c.fire(w) // marks w.fired; sends on channel for channel-based waiters
		if w.fn != nil {
			callbacks = append(callbacks, w.fn)
		}
	}

	if len(callbacks) == 0 {
		return
	}

	// Release the lock before calling any AfterFunc fn so that a callback that
	// re-enters the clock (e.g. schedules a new timer) does not deadlock on mu.
	// defer guarantees the relock happens on ALL exit paths, including panic.
	c.mu.Unlock()
	defer c.mu.Lock()
	for _, fn := range callbacks {
		fn() // synchronous, in deadline order — deterministic, not racy
	}
}

// compile-time assertions.
var (
	_ Clock = (*FakeClock)(nil)
	_ Timer = (*fakeTimer)(nil)
)
