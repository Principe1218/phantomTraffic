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

// fakeWaiter is a registered timer, AfterFunc, or Sleep. Exactly one of fn (run
// in a goroutine on fire) or a channel send (ch, for timers and Sleep) happens.
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
func (c *FakeClock) addWaiter(d time.Duration, fn func()) *fakeWaiter {
	w := &fakeWaiter{deadline: c.now.Add(d), ch: make(chan time.Time, 1), fn: fn}
	if d <= 0 {
		c.fire(w)
		return w
	}
	c.waiters = append(c.waiters, w)
	return w
}

// fire delivers the waiter exactly once; caller MUST hold c.mu. Callbacks run in
// their own goroutine so a callback that re-enters the clock cannot deadlock on mu.
func (c *FakeClock) fire(w *fakeWaiter) {
	if w.fired || w.stopped {
		return
	}
	w.fired = true
	if w.fn != nil {
		go w.fn()
		return
	}
	w.ch <- w.deadline
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
	return &fakeTimer{c: c, w: c.addWaiter(d, nil)}
}

func (c *FakeClock) AfterFunc(d time.Duration, f func()) Timer {
	c.mu.Lock()
	defer c.mu.Unlock()
	return &fakeTimer{c: c, w: c.addWaiter(d, f)}
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
	w := c.addWaiter(d, nil)
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
func (c *FakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
	c.fireDueLocked()
}

// Set moves logical time to t. If t is in the future it fires due waiters in
// order; if t is in the past it only rewinds Now (no waiter fires on a rewind).
func (c *FakeClock) Set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = t
	c.fireDueLocked()
}

// fireDueLocked fires all unstopped waiters at/before c.now in deadline order;
// caller MUST hold c.mu.
func (c *FakeClock) fireDueLocked() {
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
	for _, w := range due {
		c.fire(w)
	}
}

// compile-time assertions.
var (
	_ Clock = (*FakeClock)(nil)
	_ Timer = (*fakeTimer)(nil)
)
