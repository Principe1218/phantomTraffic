package engine

import (
	"context"
	"sync"
	"time"

	"github.com/Principe1218/phantomTraffic/internal/clock"
)

// gateSource identifies an independent pause source. The run is open only when
// EVERY source is open; it is paused while ANY source is paused (design §6.4).
type gateSource uint8

const (
	gateOperator gateSource = iota // manual Pause()/Resume(); accrues pausedTotal for timer shift
	gateSchedule                   // off-window scheduler edge
)

const numGateSources = 2

// gate is the engine's dual-source pause/resume primitive. All time is read from
// the injected clock so pause accounting is deterministic under a fake clock.
type gate struct {
	clk clock.Clock

	mu     sync.Mutex
	cond   *sync.Cond
	paused [numGateSources]bool

	// closeNotify is closed when the gate transitions open→closed and is replaced
	// by a fresh channel on the next fully-open transition. runBlockDuration
	// selects on it to implement gate-aware block-duration timer shift (design §6.4).
	closeNotify chan struct{}

	// operator-pause accounting for the ramp/schedule/duration timer shift.
	opPausedAt    time.Time     // zero unless gateOperator is currently paused
	opPausedTotal time.Duration // accumulated wall time spent operator-paused
}

func newGate(clk clock.Clock) *gate {
	g := &gate{
		clk:         clk,
		closeNotify: make(chan struct{}),
	}
	g.cond = sync.NewCond(&g.mu)
	return g
}

// pause closes src. Repeated pauses of the same source are idempotent.
// If this call transitions the gate from fully open to closed, the closeNotify
// channel is closed so runBlockDuration can stop its active-time segment.
func (g *gate) pause(src gateSource) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.paused[src] {
		return
	}
	wasOpen := g.openLocked()
	g.paused[src] = true
	if src == gateOperator {
		g.opPausedAt = g.clk.Now()
	}
	if wasOpen && !g.openLocked() {
		close(g.closeNotify)
	}
}

// resume reopens src. Resuming an already-open source is a no-op. The last
// source to reopen wakes any goroutines blocked in wait and replaces closeNotify
// with a fresh channel so the next active segment can be monitored.
func (g *gate) resume(src gateSource) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.paused[src] {
		return
	}
	g.paused[src] = false
	if src == gateOperator && !g.opPausedAt.IsZero() {
		g.opPausedTotal += g.clk.Since(g.opPausedAt)
		g.opPausedAt = time.Time{}
	}
	if g.openLocked() {
		g.closeNotify = make(chan struct{})
	}
	g.cond.Broadcast()
}

// isOpen reports whether every source is open.
func (g *gate) isOpen() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.openLocked()
}

func (g *gate) openLocked() bool {
	for _, p := range g.paused {
		if p {
			return false
		}
	}
	return true
}

// wait blocks while ANY source is paused, returning nil once every source is
// open. It is context-cancelable: if ctx is canceled while blocked, wait returns
// ctx.Err() promptly. A watcher goroutine broadcasts on ctx.Done() so the
// underlying sync.Cond wakes and re-checks (Cond cannot select on a context).
func (g *gate) wait(ctx context.Context) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.openLocked() {
		return ctx.Err()
	}

	stop := make(chan struct{})
	defer close(stop)
	go func() {
		select {
		case <-ctx.Done():
			g.cond.Broadcast()
		case <-stop:
		}
	}()

	for !g.openLocked() {
		if err := ctx.Err(); err != nil {
			return err
		}
		g.cond.Wait()
	}
	return ctx.Err()
}

// pausedTotal returns the cumulative wall time the run has spent paused by the
// operator (gateOperator), used to shift the ramp/schedule/duration timers
// forward (design §6.4). Schedule pauses do not contribute. If an operator pause
// is currently in progress, its elapsed interval so far is included.
func (g *gate) pausedTotal() time.Duration {
	g.mu.Lock()
	defer g.mu.Unlock()
	total := g.opPausedTotal
	if !g.opPausedAt.IsZero() {
		total += g.clk.Since(g.opPausedAt)
	}
	return total
}

// waitOpenAndGetCloseCh blocks until every gate source is open, then returns
// the current closeNotify channel while still holding g.mu. The returned channel
// is closed when the gate next transitions open→closed and replaced by a fresh
// channel on the following open transition. runBlockDuration uses this to sleep
// until the gate closes WITHOUT polling (design §6.4).
func (g *gate) waitOpenAndGetCloseCh(ctx context.Context) (<-chan struct{}, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	stop := make(chan struct{})
	defer close(stop)
	go func() {
		select {
		case <-ctx.Done():
			g.cond.Broadcast()
		case <-stop:
		}
	}()

	for !g.openLocked() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		g.cond.Wait()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return g.closeNotify, nil
}
